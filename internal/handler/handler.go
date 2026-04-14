package handler

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/pure-golang/budva-claude/internal/domain"
)

type telegramGateway interface {
	DeleteMessages(ctx context.Context, chatID domain.ChatID, messageIDs []domain.MessageID, revoke bool) error
	GetMessage(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID) (*domain.Message, error)
	EditMessageText(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID, text *domain.FormattedText) error
	EditMessageCaption(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID, text *domain.FormattedText) error
	SendMessage(ctx context.Context, chatID domain.ChatID, content domain.InputMessageContent) (domain.MessageID, error)
	SendMessageAlbum(ctx context.Context, chatID domain.ChatID, contents []domain.InputMessageContent) ([]domain.MessageID, error)
	ForwardMessages(ctx context.Context, fromChatID domain.ChatID, toChatID domain.ChatID, messageIDs []domain.MessageID) ([]domain.MessageID, error)
	GetMessageLink(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID) (string, error)
}

type stateStore interface {
	SetCopiedMessageID(chatID domain.ChatID, messageID domain.MessageID, toChatMessageID string) error
	GetCopiedMessageIDs(chatID domain.ChatID, messageID domain.MessageID) []string
	DeleteCopiedMessageIDs(chatID domain.ChatID, messageID domain.MessageID) error
	SetNewMessageID(chatID domain.ChatID, tmpMessageID, newMessageID domain.MessageID) error
	GetNewMessageID(chatID domain.ChatID, tmpMessageID domain.MessageID) domain.MessageID
	DeleteNewMessageID(chatID domain.ChatID, tmpMessageID domain.MessageID) error
	SetTmpMessageID(chatID domain.ChatID, newMessageID, tmpMessageID domain.MessageID) error
	DeleteTmpMessageID(chatID domain.ChatID, newMessageID domain.MessageID) error
	SetAnswerMessageID(dstChatID domain.ChatID, tmpMessageID domain.MessageID, srcChatID domain.ChatID, srcMessageID domain.MessageID) error
	GetAnswerMessageID(dstChatID domain.ChatID, tmpMessageID domain.MessageID) string
	DeleteAnswerMessageID(dstChatID domain.ChatID, tmpMessageID domain.MessageID) error
	IncrementViewedMessages(toChatID domain.ChatID, date string) (uint64, error)
	IncrementForwardedMessages(toChatID domain.ChatID, date string) (uint64, error)
}

type messageService interface {
	GetFormattedText(msg *domain.Message) *domain.FormattedText
	IsSystemMessage(msg *domain.Message) bool
	GetReplyMarkupData(msg *domain.Message) []byte
	BuildInputContent(msg *domain.Message, text *domain.FormattedText) domain.InputMessageContent
}

type filterService interface {
	Evaluate(text string, rule *domain.ForwardRule) domain.FiltersMode
}

type transformService interface {
	Transform(ctx context.Context, params domain.TransformParams) (*domain.FormattedText, error)
	AddNextLink(ctx context.Context, text *domain.FormattedText, src *domain.Source, dstChatID domain.ChatID, nextMessageID domain.MessageID) *domain.FormattedText
}

// DedupTracker отслеживает дедупликацию доставки.
type DedupTracker interface {
	TryMark(chatID domain.ChatID) bool
}

// DedupFactory создаёт новый трекер дедупликации для набора целевых чатов.
type DedupFactory = func(destinations []domain.ChatID) DedupTracker

type albumService interface {
	AddMessage(key domain.MediaAlbumKey, messageID domain.MessageID) bool
	LastReceivedAge(key domain.MediaAlbumKey) time.Duration
	PopMessages(key domain.MediaAlbumKey) []domain.MessageID
}

type taskQueue interface {
	Add(fn func())
}

type rateLimiter interface {
	WaitForForward(ctx context.Context, dstChatID domain.ChatID)
}

// Handler обрабатывает обновления Telegram.
type Handler struct {
	logger     *slog.Logger
	telegram   telegramGateway
	state      stateStore
	messages   messageService
	filters    filterService
	transform  transformService
	albums     albumService
	queue      taskQueue
	limiter    rateLimiter
	newTracker DedupFactory
	ruleset    atomic.Pointer[domain.RuleSet]
}

// New создаёт новый экземпляр обработчика обновлений.
func New(
	telegram telegramGateway,
	state stateStore,
	messages messageService,
	filters filterService,
	transformSvc transformService,
	albums albumService,
	queue taskQueue,
	limiter rateLimiter,
	newTracker DedupFactory,
) *Handler {
	return &Handler{
		logger:     slog.Default().With("module", "handler"),
		telegram:   telegram,
		state:      state,
		messages:   messages,
		filters:    filters,
		transform:  transformSvc,
		albums:     albums,
		queue:      queue,
		limiter:    limiter,
		newTracker: newTracker,
	}
}

// SetRuleSet обновляет текущий набор правил.
func (h *Handler) SetRuleSet(rs *domain.RuleSet) {
	h.ruleset.Store(rs)
}

// OnNewMessage обрабатывает новое сообщение.
func (h *Handler) OnNewMessage(ctx context.Context, msg *domain.Message) {
	rs := h.ruleset.Load()
	if rs == nil {
		return
	}
	srcChatID := msg.ChatID
	if _, ok := rs.UniqueSources[srcChatID]; !ok {
		return
	}

	// Удаление системных сообщений
	src := rs.Sources[srcChatID]
	if h.messages.IsSystemMessage(msg) {
		if src != nil && src.DeleteSystemMessages {
			h.queue.Add(func() {
				if err := h.telegram.DeleteMessages(ctx, srcChatID, []domain.MessageID{msg.ID}, true); err != nil {
					h.logger.Error("Failed to delete system message", slog.Any("err", err))
				}
			})
		}
		return
	}

	formattedText := h.messages.GetFormattedText(msg)
	if formattedText == nil {
		return
	}

	// Обработка по каждому правилу
	for _, ruleID := range rs.OrderedForwardRules {
		rule := rs.ForwardRules[ruleID]
		if rule.From != srcChatID {
			continue
		}
		if !rule.SendCopy && !msg.CanBeSaved {
			continue
		}

		// Медиа-альбом
		if msg.MediaAlbumID != 0 {
			albumKey := fmt.Sprintf("%s:%d", ruleID, msg.MediaAlbumID)
			isFirst := h.albums.AddMessage(albumKey, msg.ID)
			if isFirst {
				h.processMediaAlbum(ctx, albumKey, rs, ruleID, msg, formattedText)
			}
			continue
		}

		// Одиночное сообщение
		h.processMessage(ctx, rs, ruleID, rule, msg, formattedText)
	}
}

// OnEditedMessage обрабатывает отредактированное сообщение.
func (h *Handler) OnEditedMessage(ctx context.Context, msg *domain.Message) {
	rs := h.ruleset.Load()
	if rs == nil {
		return
	}
	if _, ok := rs.UniqueSources[msg.ChatID]; !ok {
		return
	}

	h.queue.Add(func() {
		h.editMessagesWithRetry(ctx, rs, msg, 0)
	})
}

// OnDeletedMessages обрабатывает удаление сообщений.
func (h *Handler) OnDeletedMessages(ctx context.Context, chatID domain.ChatID, messageIDs []domain.MessageID, isPermanent bool) {
	if !isPermanent {
		return
	}
	rs := h.ruleset.Load()
	if rs == nil {
		return
	}
	if _, ok := rs.UniqueSources[chatID]; !ok {
		return
	}

	h.queue.Add(func() {
		h.deleteMessagesWithRetry(ctx, rs, chatID, messageIDs, 0)
	})
}

// OnMessageSendSucceeded обрабатывает подтверждение отправки.
func (h *Handler) OnMessageSendSucceeded(chatID domain.ChatID, oldMessageID, newMessageID domain.MessageID) {
	h.queue.Add(func() {
		if err := h.state.SetNewMessageID(chatID, oldMessageID, newMessageID); err != nil {
			h.logger.Error("Failed to set new message ID", slog.Any("err", err))
		}
		if err := h.state.SetTmpMessageID(chatID, newMessageID, oldMessageID); err != nil {
			h.logger.Error("Failed to set tmp message ID", slog.Any("err", err))
		}
	})
}

func (h *Handler) processMessage(ctx context.Context, rs *domain.RuleSet, ruleID string, rule *domain.ForwardRule, msg *domain.Message, text *domain.FormattedText) {
	mode := h.filters.Evaluate(text.Text, rule)
	src := rs.Sources[msg.ChatID]

	tracker := h.newTracker(rule.To)

	switch mode {
	case domain.FiltersOK:
		isFirstDst := true
		for _, dstChatID := range rule.To {
			if !tracker.TryMark(dstChatID) {
				continue
			}
			dst := rs.Destinations[dstChatID]
			withSources := isFirstDst
			isFirstDst = false
			h.queue.Add(func() {
				h.forwardMessage(ctx, ruleID, rule, msg, text, src, dst, dstChatID, 0, withSources)
			})
		}
	case domain.FiltersCheck:
		if rule.Check != 0 {
			h.queue.Add(func() {
				h.limiter.WaitForForward(ctx, rule.Check)
				if _, err := h.telegram.ForwardMessages(ctx, msg.ChatID, rule.Check, []domain.MessageID{msg.ID}); err != nil {
					h.logger.Error("Failed to forward to check chat", slog.Any("err", err))
				}
			})
		}
	case domain.FiltersOther:
		if rule.Other != 0 {
			h.queue.Add(func() {
				h.limiter.WaitForForward(ctx, rule.Other)
				if _, err := h.telegram.ForwardMessages(ctx, msg.ChatID, rule.Other, []domain.MessageID{msg.ID}); err != nil {
					h.logger.Error("Failed to forward to other chat", slog.Any("err", err))
				}
			})
		}
	}

	// Статистика
	h.queue.Add(func() {
		date := time.Now().Format("2006-01-02")
		for _, dstChatID := range rule.To {
			if _, err := h.state.IncrementViewedMessages(dstChatID, date); err != nil {
				h.logger.Error("Failed to increment viewed messages", slog.Any("err", err))
			}
		}
		if mode == domain.FiltersOK {
			for _, dstChatID := range rule.To {
				if _, err := h.state.IncrementForwardedMessages(dstChatID, date); err != nil {
					h.logger.Error("Failed to increment forwarded messages", slog.Any("err", err))
				}
			}
		}
	})
}

func (h *Handler) forwardMessage(ctx context.Context, ruleID string, rule *domain.ForwardRule, msg *domain.Message, text *domain.FormattedText, src *domain.Source, dst *domain.Destination, dstChatID domain.ChatID, prevMessageID domain.MessageID, withSources bool) {
	h.limiter.WaitForForward(ctx, dstChatID)

	if !rule.SendCopy {
		if _, err := h.telegram.ForwardMessages(ctx, msg.ChatID, dstChatID, []domain.MessageID{msg.ID}); err != nil {
			h.logger.Error("Failed to forward message", slog.Any("err", err))
		}
		return
	}

	// Разворачивание origin для forwarded-from-channel
	originMsg := h.getOriginMessage(ctx, msg)

	transformed, err := h.transform.Transform(ctx, domain.TransformParams{
		Text:          text.DeepCopy(),
		Source:        src,
		Destination:   dst,
		DstChatID:     dstChatID,
		SrcChatID:     msg.ChatID,
		SrcMessageID:  msg.ID,
		PrevMessageID: prevMessageID,
		WithSources:   withSources,
		ReplyMarkup:   h.messages.GetReplyMarkupData(msg),
	})
	if err != nil {
		h.logger.Error("Failed to transform message", slog.Any("err", err))
		return
	}

	// Используем origin для BuildInputContent если доступен
	contentMsg := msg
	if originMsg != nil {
		contentMsg = originMsg
	}
	content := h.messages.BuildInputContent(contentMsg, transformed)

	// Сохранение reply chain
	content.ReplyToMessageID = h.resolveReplyTo(msg, dstChatID)

	tmpMsgID, err := h.telegram.SendMessage(ctx, dstChatID, content)
	if err != nil {
		h.logger.Error("Failed to send message", slog.Any("err", err), slog.Int64("dst_chat_id", dstChatID))
		return
	}

	toChatMsgID := fmt.Sprintf("%s:%d:%d", ruleID, dstChatID, tmpMsgID)
	if err := h.state.SetCopiedMessageID(msg.ChatID, msg.ID, toChatMsgID); err != nil {
		h.logger.Error("Failed to set copied message ID", slog.Any("err", err))
	}

	// Reply markup tracking
	replyData := h.messages.GetReplyMarkupData(msg)
	if len(replyData) > 0 {
		if err := h.state.SetAnswerMessageID(dstChatID, tmpMsgID, msg.ChatID, msg.ID); err != nil {
			h.logger.Error("Failed to set answer message ID", slog.Any("err", err))
		}
	}

	// Next link workflow
	if prevMessageID != 0 && rule.CopyOnce {
		go h.runNextLinkWorkflow(ctx, src, dstChatID, prevMessageID, tmpMsgID)
	}
}

// getOriginMessage разворачивает forwarded-from-channel сообщение до оригинала.
func (h *Handler) getOriginMessage(ctx context.Context, msg *domain.Message) *domain.Message {
	if msg.ForwardInfo == nil || msg.ForwardInfo.OriginChatID == 0 {
		return nil
	}
	origin, err := h.telegram.GetMessage(ctx, msg.ForwardInfo.OriginChatID, msg.ForwardInfo.OriginMessageID)
	if err != nil {
		return nil
	}
	// Валидация: текст оригинала должен совпадать с forwarded
	originText := h.messages.GetFormattedText(origin)
	msgText := h.messages.GetFormattedText(msg)
	if originText == nil || msgText == nil || originText.Text != msgText.Text {
		return nil
	}
	return origin
}

// resolveReplyTo находит копию replied-to сообщения в целевом чате.
func (h *Handler) resolveReplyTo(msg *domain.Message, dstChatID domain.ChatID) domain.MessageID {
	if msg.ReplyTo == nil || msg.ReplyTo.ChatID != msg.ChatID {
		return 0
	}
	copies := h.state.GetCopiedMessageIDs(msg.ReplyTo.ChatID, msg.ReplyTo.MessageID)
	for _, copy := range copies {
		ref, ok := parseCopyRef(copy)
		if !ok || ref.dstChatID != dstChatID {
			continue
		}
		newID := h.state.GetNewMessageID(dstChatID, ref.tmpMsgID)
		if newID != 0 {
			return newID
		}
	}
	return 0
}

func (h *Handler) runNextLinkWorkflow(ctx context.Context, src *domain.Source, dstChatID domain.ChatID, prevMessageID, tmpMessageID domain.MessageID) {
	for i := 0; i < 10; i++ {
		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Second):
		}
		newID := h.state.GetNewMessageID(dstChatID, tmpMessageID)
		if newID == 0 {
			continue
		}
		prevMsg, err := h.telegram.GetMessage(ctx, dstChatID, prevMessageID)
		if err != nil {
			return
		}
		text := h.messages.GetFormattedText(prevMsg)
		if text == nil {
			return
		}
		updated := h.transform.AddNextLink(ctx, text, src, dstChatID, newID)
		if err := h.telegram.EditMessageText(ctx, dstChatID, prevMessageID, updated); err != nil {
			h.logger.Error("Failed to edit message with next link", slog.Any("err", err))
		}
		return
	}
}

func (h *Handler) processMediaAlbum(ctx context.Context, albumKey string, rs *domain.RuleSet, ruleID string, firstMsg *domain.Message, _ *domain.FormattedText) {
	h.queue.Add(func() {
		// Ожидаем пока все сообщения альбома придут (3 секунды после последнего)
		for {
			age := h.albums.LastReceivedAge(albumKey)
			if age >= 3*time.Second {
				break
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(3*time.Second - age):
			}
		}

		messageIDs := h.albums.PopMessages(albumKey)
		if len(messageIDs) == 0 {
			return
		}

		rule := rs.ForwardRules[ruleID]
		if rule == nil {
			return
		}

		for _, dstChatID := range rule.To {
			h.limiter.WaitForForward(ctx, dstChatID)
			if _, err := h.telegram.ForwardMessages(ctx, firstMsg.ChatID, dstChatID, messageIDs); err != nil {
				h.logger.Error("Failed to forward media album", slog.Any("err", err))
			}
		}
	})
}

// editMessagesWithRetry обрабатывает редактирование с retry до 3 раз.
func (h *Handler) editMessagesWithRetry(ctx context.Context, rs *domain.RuleSet, msg *domain.Message, attempt int) {
	needRetry := h.editMessages(ctx, rs, msg)
	if needRetry && attempt < 3 {
		h.queue.Add(func() {
			h.editMessagesWithRetry(ctx, rs, msg, attempt+1)
		})
	}
}

// editMessages возвращает true если нужен retry (permanent ID ещё не в storage).
func (h *Handler) editMessages(ctx context.Context, rs *domain.RuleSet, msg *domain.Message) bool {
	copies := h.state.GetCopiedMessageIDs(msg.ChatID, msg.ID)
	if len(copies) == 0 {
		return false
	}

	src := rs.Sources[msg.ChatID]
	text := h.messages.GetFormattedText(msg)
	if text == nil {
		return false
	}

	needRetry := false

	for _, copy := range copies {
		ref, ok := parseCopyRef(copy)
		if !ok {
			continue
		}

		newMsgID := h.state.GetNewMessageID(ref.dstChatID, ref.tmpMsgID)
		if newMsgID == 0 {
			needRetry = true
			continue
		}

		rule := rs.ForwardRules[ref.ruleID]
		if rule == nil {
			continue
		}

		dst := rs.Destinations[ref.dstChatID]

		if rule.CopyOnce {
			// Версионирование: отправляем новую копию
			h.forwardMessage(ctx, ref.ruleID, rule, msg, text, src, dst, ref.dstChatID, newMsgID, true)
		} else {
			// Обновляем существующую копию
			transformed, err := h.transform.Transform(ctx, domain.TransformParams{
				Text:         text.DeepCopy(),
				Source:       src,
				Destination:  dst,
				DstChatID:    ref.dstChatID,
				SrcChatID:    msg.ChatID,
				SrcMessageID: msg.ID,
				WithSources:  true,
				ReplyMarkup:  h.messages.GetReplyMarkupData(msg),
			})
			if err != nil {
				h.logger.Error("Failed to transform edited message", slog.Any("err", err))
				continue
			}
			if msg.Content.Type == domain.ContentText {
				if err := h.telegram.EditMessageText(ctx, ref.dstChatID, newMsgID, transformed); err != nil {
					h.logger.Error("Failed to edit message text", slog.Any("err", err))
				}
			} else {
				if err := h.telegram.EditMessageCaption(ctx, ref.dstChatID, newMsgID, transformed); err != nil {
					h.logger.Error("Failed to edit message caption", slog.Any("err", err))
				}
			}

			// Reply markup sync
			replyData := h.messages.GetReplyMarkupData(msg)
			if len(replyData) > 0 {
				if err := h.state.SetAnswerMessageID(ref.dstChatID, ref.tmpMsgID, msg.ChatID, msg.ID); err != nil {
					h.logger.Error("Failed to set answer message ID", slog.Any("err", err))
				}
			} else {
				if err := h.state.DeleteAnswerMessageID(ref.dstChatID, ref.tmpMsgID); err != nil {
					h.logger.Error("Failed to delete answer message ID", slog.Any("err", err))
				}
			}
		}
	}

	return needRetry
}

// deleteMessagesWithRetry обрабатывает удаление с retry до 3 раз.
func (h *Handler) deleteMessagesWithRetry(ctx context.Context, rs *domain.RuleSet, chatID domain.ChatID, messageIDs []domain.MessageID, attempt int) {
	needRetry := h.deleteMessages(ctx, rs, chatID, messageIDs)
	if needRetry && attempt < 3 {
		h.queue.Add(func() {
			h.deleteMessagesWithRetry(ctx, rs, chatID, messageIDs, attempt+1)
		})
	}
}

// deleteMessages возвращает true если нужен retry.
func (h *Handler) deleteMessages(ctx context.Context, rs *domain.RuleSet, chatID domain.ChatID, messageIDs []domain.MessageID) bool {
	needRetry := false

	for _, msgID := range messageIDs {
		copies := h.state.GetCopiedMessageIDs(chatID, msgID)
		if len(copies) == 0 {
			continue
		}

		for _, copy := range copies {
			ref, ok := parseCopyRef(copy)
			if !ok {
				continue
			}

			rule := rs.ForwardRules[ref.ruleID]
			if rule != nil && rule.Indelible {
				continue
			}

			newMsgID := h.state.GetNewMessageID(ref.dstChatID, ref.tmpMsgID)
			if newMsgID == 0 {
				needRetry = true
				continue
			}

			if err := h.telegram.DeleteMessages(ctx, ref.dstChatID, []domain.MessageID{newMsgID}, true); err != nil {
				h.logger.Error("Failed to delete copied message", slog.Any("err", err))
			}
			if err := h.state.DeleteNewMessageID(ref.dstChatID, ref.tmpMsgID); err != nil {
				h.logger.Error("Failed to delete new message ID", slog.Any("err", err))
			}
			if err := h.state.DeleteTmpMessageID(ref.dstChatID, newMsgID); err != nil {
				h.logger.Error("Failed to delete tmp message ID", slog.Any("err", err))
			}
			if err := h.state.DeleteAnswerMessageID(ref.dstChatID, ref.tmpMsgID); err != nil {
				h.logger.Error("Failed to delete answer message ID", slog.Any("err", err))
			}
		}

		if !needRetry {
			if err := h.state.DeleteCopiedMessageIDs(chatID, msgID); err != nil {
				h.logger.Error("Failed to delete copied message IDs", slog.Any("err", err))
			}
		}
	}

	return needRetry
}

// copyRef содержит разобранную ссылку на копию сообщения формата "ruleID:dstChatID:tmpMsgID".
type copyRef struct {
	ruleID    string
	dstChatID int64
	tmpMsgID  int64
}

// parseCopyRef разбирает строку формата "ruleID:dstChatID:tmpMsgID".
func parseCopyRef(s string) (copyRef, bool) {
	parts := strings.Split(s, ":")
	if len(parts) < 3 {
		return copyRef{}, false
	}
	dstChatID := parseID(parts[1])
	tmpMsgID := parseID(parts[2])
	return copyRef{
		ruleID:    parts[0],
		dstChatID: dstChatID,
		tmpMsgID:  tmpMsgID,
	}, true
}

func parseID(s string) int64 {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return id
}
