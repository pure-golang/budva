package handler

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/domain"
)

// telegramRepo — частично применяемый интерфейс к infra/telegram.
// Обёртки clientAdapter: raw go-tdlib сигнатуры, без ctx и без domain-типов.
type telegramRepo interface {
	ClientDone() <-chan struct{}
	Updates() <-chan client.Type
	DeleteMessages(*client.DeleteMessagesRequest) (*client.Ok, error)
	GetMessage(*client.GetMessageRequest) (*client.Message, error)
	EditMessageText(*client.EditMessageTextRequest) (*client.Message, error)
	EditMessageCaption(*client.EditMessageCaptionRequest) (*client.Message, error)
	SendMessage(*client.SendMessageRequest) (*client.Message, error)
	SendMessageAlbum(*client.SendMessageAlbumRequest) (*client.Messages, error)
	ForwardMessages(*client.ForwardMessagesRequest) (*client.Messages, error)
	GetMessageLink(*client.GetMessageLinkRequest) (*client.MessageLink, error)
}

type stateRepo interface {
	SetCopiedMessageID(chatID int64, messageID int64, toChatMessageID string) error
	GetCopiedMessageIDs(chatID int64, messageID int64) []string
	DeleteCopiedMessageIDs(chatID int64, messageID int64) error
	SetNewMessageID(chatID int64, tmpMessageID, newMessageID int64) error
	GetNewMessageID(chatID int64, tmpMessageID int64) int64
	DeleteNewMessageID(chatID int64, tmpMessageID int64) error
	SetTmpMessageID(chatID int64, newMessageID, tmpMessageID int64) error
	DeleteTmpMessageID(chatID int64, newMessageID int64) error
	SetAnswerMessageID(dstChatID int64, tmpMessageID int64, srcChatID int64, srcMessageID int64) error
	GetAnswerMessageID(dstChatID int64, tmpMessageID int64) string
	DeleteAnswerMessageID(dstChatID int64, tmpMessageID int64) error
	IncrementViewedMessages(toChatID int64, date string) (uint64, error)
	IncrementForwardedMessages(toChatID int64, date string) (uint64, error)
}

type messageService interface {
	GetFormattedText(msg *client.Message) *client.FormattedText
	IsSystemMessage(msg *client.Message) bool
	GetReplyMarkupData(msg *client.Message) []byte
	BuildInputContent(msg *client.Message, text *client.FormattedText) client.InputMessageContent
}

type filterService interface {
	Evaluate(text string, rule *domain.ForwardRule) domain.FiltersMode
}

type transformService interface {
	Transform(ctx context.Context, params domain.TransformParams) (*client.FormattedText, error)
	AddNextLink(ctx context.Context, text *client.FormattedText, src *domain.Source, dstChatID int64, nextMessageID int64) *client.FormattedText
}

// DedupTracker отслеживает дедупликацию доставки.
type DedupTracker interface {
	TryMark(chatID int64) bool
}

// DedupFactory создаёт новый трекер дедупликации для набора целевых чатов.
type DedupFactory = func(destinations []int64) DedupTracker

type albumService interface {
	AddMessage(key domain.MediaAlbumKey, msg *client.Message) bool
	LastReceivedAge(key domain.MediaAlbumKey) time.Duration
	PopMessages(key domain.MediaAlbumKey) []*client.Message
}

type taskQueue interface {
	Add(fn func())
}

type rateLimiter interface {
	WaitForForward(ctx context.Context, dstChatID int64)
}

// Service обрабатывает обновления Telegram.
type Service struct {
	logger           *slog.Logger
	telegramRepo     telegramRepo
	stateRepo        stateRepo
	messageService   messageService
	filterService    filterService
	transformService transformService
	albumService     albumService
	taskQueue        taskQueue
	rateLimiter      rateLimiter
	newTracker       DedupFactory
	ruleset          atomic.Pointer[domain.RuleSet]
}

// New создаёт новый экземпляр обработчика обновлений.
func New(
	telegramRepo telegramRepo,
	stateRepo stateRepo,
	messageService messageService,
	filterService filterService,
	transformService transformService,
	albumService albumService,
	taskQueue taskQueue,
	rateLimiter rateLimiter,
	newTracker DedupFactory,
) *Service {
	return &Service{
		logger:           slog.Default().With("module", "app.handler"),
		telegramRepo:     telegramRepo,
		stateRepo:        stateRepo,
		messageService:   messageService,
		filterService:    filterService,
		transformService: transformService,
		albumService:     albumService,
		taskQueue:        taskQueue,
		rateLimiter:      rateLimiter,
		newTracker:       newTracker,
	}
}

// SetRuleSet обновляет текущий набор правил.
func (s *Service) SetRuleSet(rs *domain.RuleSet) {
	s.ruleset.Store(rs)
}

// Run читает поток обновлений Telegram и запускает обработчики событий.
func (s *Service) Run(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	case <-s.telegramRepo.ClientDone():
	}

	updates := s.telegramRepo.Updates()
	for {
		select {
		case <-ctx.Done():
			return
		case update, ok := <-updates:
			if !ok {
				return
			}
			s.handleUpdate(ctx, update)
		}
	}
}

func (s *Service) handleUpdate(ctx context.Context, update client.Type) {
	switch u := update.(type) {
	case *client.UpdateNewMessage:
		s.OnNewMessage(ctx, u.Message)
	case *client.UpdateMessageEdited:
		// Resolve в отдельной горутине: синхронный GetMessage внутри update loop
		// блокирует приём updates, что приводит к зависанию других listener-ов TDLib.
		go func(chatID, messageID int64) {
			msg, err := s.telegramRepo.GetMessage(&client.GetMessageRequest{
				ChatId:    chatID,
				MessageId: messageID,
			})
			if err != nil {
				s.logger.Warn("Failed to get edited message",
					slog.Int64("chat_id", chatID),
					slog.Int64("message_id", messageID),
					slog.Any("err", err),
				)
				return
			}
			s.OnEditedMessage(ctx, msg)
		}(u.ChatId, u.MessageId)
	case *client.UpdateDeleteMessages:
		s.OnDeletedMessages(ctx, u.ChatId, u.MessageIds, u.IsPermanent)
	case *client.UpdateMessageSendSucceeded:
		if u.Message == nil {
			return
		}
		s.OnMessageSendSucceeded(u.Message.ChatId, u.OldMessageId, u.Message.Id)
	}
}

// OnNewMessage обрабатывает новое сообщение.
func (s *Service) OnNewMessage(ctx context.Context, msg *client.Message) {
	rs := s.ruleset.Load()
	if rs == nil {
		return
	}
	srcChatID := msg.ChatId
	if _, ok := rs.UniqueSources[srcChatID]; !ok {
		return
	}

	// Удаление системных сообщений.
	src := rs.Sources[srcChatID]
	if s.messageService.IsSystemMessage(msg) {
		if src != nil && src.DeleteSystemMessages {
			s.taskQueue.Add(func() {
				if _, err := s.telegramRepo.DeleteMessages(&client.DeleteMessagesRequest{
					ChatId:     srcChatID,
					MessageIds: []int64{msg.Id},
					Revoke:     true,
				}); err != nil {
					s.logger.Error("Failed to delete system message", slog.Any("err", err))
				}
			})
		}
		return
	}

	formattedText := s.messageService.GetFormattedText(msg)
	if formattedText == nil {
		return
	}

	// Обработка по каждому правилу.
	for _, ruleID := range rs.OrderedForwardRules {
		rule := rs.ForwardRules[ruleID]
		if rule.From != srcChatID {
			continue
		}
		if !rule.SendCopy && !msg.CanBeSaved {
			continue
		}

		// Медиа-альбом.
		if msg.MediaAlbumId != 0 {
			albumKey := fmt.Sprintf("%s:%d", ruleID, int64(msg.MediaAlbumId))
			isFirst := s.albumService.AddMessage(albumKey, msg)
			if isFirst {
				s.processMediaAlbum(ctx, albumKey, rs, ruleID, msg, formattedText)
			}
			continue
		}

		// Одиночное сообщение.
		s.processMessage(ctx, rs, ruleID, rule, msg, formattedText)
	}
}

// OnEditedMessage обрабатывает отредактированное сообщение.
func (s *Service) OnEditedMessage(ctx context.Context, msg *client.Message) {
	rs := s.ruleset.Load()
	if rs == nil {
		return
	}
	if _, ok := rs.UniqueSources[msg.ChatId]; !ok {
		return
	}

	s.taskQueue.Add(func() {
		s.editMessagesWithRetry(ctx, rs, msg, 0)
	})
}

// OnDeletedMessages обрабатывает удаление сообщений.
func (s *Service) OnDeletedMessages(ctx context.Context, chatID int64, messageIDs []int64, isPermanent bool) {
	if !isPermanent {
		return
	}
	rs := s.ruleset.Load()
	if rs == nil {
		return
	}
	if _, ok := rs.UniqueSources[chatID]; !ok {
		return
	}

	s.taskQueue.Add(func() {
		s.deleteMessagesWithRetry(ctx, rs, chatID, messageIDs, 0)
	})
}

// OnMessageSendSucceeded обрабатывает подтверждение отправки.
func (s *Service) OnMessageSendSucceeded(chatID int64, oldMessageID, newMessageID int64) {
	s.taskQueue.Add(func() {
		if err := s.stateRepo.SetNewMessageID(chatID, oldMessageID, newMessageID); err != nil {
			s.logger.Error("Failed to set new message ID", slog.Any("err", err))
		}
		if err := s.stateRepo.SetTmpMessageID(chatID, newMessageID, oldMessageID); err != nil {
			s.logger.Error("Failed to set tmp message ID", slog.Any("err", err))
		}
	})
}

func (s *Service) processMessage(ctx context.Context, rs *domain.RuleSet, ruleID string, rule *domain.ForwardRule, msg *client.Message, text *client.FormattedText) {
	mode := s.filterService.Evaluate(text.Text, rule)
	src := rs.Sources[msg.ChatId]

	tracker := s.newTracker(rule.To)

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
			s.taskQueue.Add(func() {
				s.forwardMessage(ctx, ruleID, rule, msg, text, src, dst, dstChatID, 0, withSources)
			})
		}
	case domain.FiltersCheck:
		if rule.Check != 0 {
			s.taskQueue.Add(func() {
				s.rateLimiter.WaitForForward(ctx, rule.Check)
				if _, err := s.telegramRepo.ForwardMessages(&client.ForwardMessagesRequest{
					ChatId:     rule.Check,
					FromChatId: msg.ChatId,
					MessageIds: []int64{msg.Id},
				}); err != nil {
					s.logger.Error("Failed to forward to check chat", slog.Any("err", err))
				}
			})
		}
	case domain.FiltersOther:
		if rule.Other != 0 {
			s.taskQueue.Add(func() {
				s.rateLimiter.WaitForForward(ctx, rule.Other)
				if _, err := s.telegramRepo.ForwardMessages(&client.ForwardMessagesRequest{
					ChatId:     rule.Other,
					FromChatId: msg.ChatId,
					MessageIds: []int64{msg.Id},
				}); err != nil {
					s.logger.Error("Failed to forward to other chat", slog.Any("err", err))
				}
			})
		}
	}

	// Статистика.
	s.taskQueue.Add(func() {
		date := time.Now().Format("2006-01-02")
		for _, dstChatID := range rule.To {
			if _, err := s.stateRepo.IncrementViewedMessages(dstChatID, date); err != nil {
				s.logger.Error("Failed to increment viewed messages", slog.Any("err", err))
			}
		}
		if mode == domain.FiltersOK {
			for _, dstChatID := range rule.To {
				if _, err := s.stateRepo.IncrementForwardedMessages(dstChatID, date); err != nil {
					s.logger.Error("Failed to increment forwarded messages", slog.Any("err", err))
				}
			}
		}
	})
}

func (s *Service) forwardMessage(ctx context.Context, ruleID string, rule *domain.ForwardRule, msg *client.Message, text *client.FormattedText, src *domain.Source, dst *domain.Destination, dstChatID int64, prevMessageID int64, withSources bool) {
	s.rateLimiter.WaitForForward(ctx, dstChatID)

	if !rule.SendCopy {
		if _, err := s.telegramRepo.ForwardMessages(&client.ForwardMessagesRequest{
			ChatId:     dstChatID,
			FromChatId: msg.ChatId,
			MessageIds: []int64{msg.Id},
		}); err != nil {
			s.logger.Error("Failed to forward message", slog.Any("err", err))
		}
		return
	}

	// Разворачивание origin для forwarded-from-channel.
	originMsg := s.getOriginMessage(msg)

	transformed, err := s.transformService.Transform(ctx, domain.TransformParams{
		Text:          text,
		Source:        src,
		Destination:   dst,
		DstChatID:     dstChatID,
		SrcChatID:     msg.ChatId,
		SrcMessageID:  msg.Id,
		PrevMessageID: prevMessageID,
		WithSources:   withSources,
		ReplyMarkup:   s.messageService.GetReplyMarkupData(msg),
	})
	if err != nil {
		s.logger.Error("Failed to transform message", slog.Any("err", err))
		return
	}

	// Используем origin для BuildInputContent если доступен.
	contentMsg := msg
	if originMsg != nil {
		contentMsg = originMsg
	}
	content := s.messageService.BuildInputContent(contentMsg, transformed)
	replyTo := s.resolveReplyTo(msg, dstChatID)

	sent, err := s.telegramRepo.SendMessage(&client.SendMessageRequest{
		ChatId:              dstChatID,
		ReplyTo:             replyTo,
		InputMessageContent: content,
	})
	if err != nil {
		s.logger.Error("Failed to send message", slog.Any("err", err), slog.Int64("dst_chat_id", dstChatID))
		return
	}
	tmpMsgID := sent.Id

	toChatMsgID := fmt.Sprintf("%s:%d:%d", ruleID, dstChatID, tmpMsgID)
	if err := s.stateRepo.SetCopiedMessageID(msg.ChatId, msg.Id, toChatMsgID); err != nil {
		s.logger.Error("Failed to set copied message ID", slog.Any("err", err))
	}

	// Reply markup tracking.
	replyData := s.messageService.GetReplyMarkupData(msg)
	if len(replyData) > 0 {
		if err := s.stateRepo.SetAnswerMessageID(dstChatID, tmpMsgID, msg.ChatId, msg.Id); err != nil {
			s.logger.Error("Failed to set answer message ID", slog.Any("err", err))
		}
	}

	// Next link workflow.
	if prevMessageID != 0 && rule.CopyOnce {
		go s.runNextLinkWorkflow(ctx, src, dstChatID, prevMessageID, tmpMsgID)
	}
}

// getOriginMessage разворачивает forwarded-from-channel сообщение до оригинала.
func (s *Service) getOriginMessage(msg *client.Message) *client.Message {
	if msg.ForwardInfo == nil {
		return nil
	}
	channel, ok := msg.ForwardInfo.Origin.(*client.MessageOriginChannel)
	if !ok || channel.ChatId == 0 {
		return nil
	}
	origin, err := s.telegramRepo.GetMessage(&client.GetMessageRequest{
		ChatId:    channel.ChatId,
		MessageId: channel.MessageId,
	})
	if err != nil {
		return nil
	}
	// Валидация: текст оригинала должен совпадать с forwarded.
	originText := s.messageService.GetFormattedText(origin)
	msgText := s.messageService.GetFormattedText(msg)
	if originText == nil || msgText == nil || originText.Text != msgText.Text {
		return nil
	}
	return origin
}

// resolveReplyTo находит копию replied-to сообщения в целевом чате.
// Возвращает InputMessageReplyToMessage для SendMessageRequest.ReplyTo или nil.
func (s *Service) resolveReplyTo(msg *client.Message, dstChatID int64) *client.InputMessageReplyToMessage {
	reply, ok := msg.ReplyTo.(*client.MessageReplyToMessage)
	if !ok {
		return nil
	}
	if reply.ChatId != msg.ChatId {
		return nil
	}
	copies := s.stateRepo.GetCopiedMessageIDs(reply.ChatId, reply.MessageId)
	for _, c := range copies {
		ref, ok := parseCopyRef(c)
		if !ok || ref.dstChatID != dstChatID {
			continue
		}
		newID := s.stateRepo.GetNewMessageID(dstChatID, ref.tmpMsgID)
		if newID != 0 {
			return &client.InputMessageReplyToMessage{MessageId: newID}
		}
	}
	return nil
}

func (s *Service) runNextLinkWorkflow(ctx context.Context, src *domain.Source, dstChatID int64, prevMessageID, tmpMessageID int64) {
	for i := 0; i < 10; i++ {
		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Second):
		}
		newID := s.stateRepo.GetNewMessageID(dstChatID, tmpMessageID)
		if newID == 0 {
			continue
		}
		prevMsg, err := s.telegramRepo.GetMessage(&client.GetMessageRequest{
			ChatId:    dstChatID,
			MessageId: prevMessageID,
		})
		if err != nil {
			return
		}
		text := s.messageService.GetFormattedText(prevMsg)
		if text == nil {
			return
		}
		updated := s.transformService.AddNextLink(ctx, text, src, dstChatID, newID)
		if _, err := s.telegramRepo.EditMessageText(&client.EditMessageTextRequest{
			ChatId:              dstChatID,
			MessageId:           prevMessageID,
			InputMessageContent: &client.InputMessageText{Text: updated},
		}); err != nil {
			s.logger.Error("Failed to edit message with next link", slog.Any("err", err))
		}
		return
	}
}

func (s *Service) processMediaAlbum(ctx context.Context, albumKey string, rs *domain.RuleSet, ruleID string, firstMsg *client.Message, formattedText *client.FormattedText) {
	_ = formattedText // оставлен для симметрии API, текст для каждого сообщения берётся по месту
	s.taskQueue.Add(func() {
		// Ожидаем пока все сообщения альбома придут (3 секунды после последнего).
		for {
			age := s.albumService.LastReceivedAge(albumKey)
			if age >= 3*time.Second {
				break
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(3*time.Second - age):
			}
		}

		messages := s.albumService.PopMessages(albumKey)
		if len(messages) == 0 {
			return
		}

		rule := rs.ForwardRules[ruleID]
		if rule == nil {
			return
		}

		src := rs.Sources[firstMsg.ChatId]
		tracker := s.newTracker(rule.To)

		for _, dstChatID := range rule.To {
			if !tracker.TryMark(dstChatID) {
				continue
			}
			s.taskQueue.Add(func() {
				s.forwardAlbum(ctx, rule, messages, src, rs.Destinations[dstChatID], dstChatID)
			})
		}

		// Статистика.
		s.taskQueue.Add(func() {
			date := time.Now().Format("2006-01-02")
			for _, dstChatID := range rule.To {
				if _, err := s.stateRepo.IncrementViewedMessages(dstChatID, date); err != nil {
					s.logger.Error("Failed to increment viewed messages", slog.Any("err", err))
				}
				if _, err := s.stateRepo.IncrementForwardedMessages(dstChatID, date); err != nil {
					s.logger.Error("Failed to increment forwarded messages", slog.Any("err", err))
				}
			}
		})
	})
}

// forwardAlbum пересылает/копирует медиа-альбом целевому чату.
// src/dst сохранены в сигнатуре для симметрии с forwardMessage и на случай
// будущей интеграции transform-правил для альбомов.
func (s *Service) forwardAlbum(ctx context.Context, rule *domain.ForwardRule, messages []*client.Message, src *domain.Source, dst *domain.Destination, dstChatID int64) {
	_ = src
	_ = dst
	// Сортируем по ID — albumService может хранить в произвольном порядке,
	// а TDLib требует strictly increasing order для ForwardMessages
	// и порядок контента определяет порядок фото в альбоме для SendMessageAlbum.
	slices.SortFunc(messages, func(a, b *client.Message) int {
		return int(a.Id - b.Id)
	})

	s.rateLimiter.WaitForForward(ctx, dstChatID)

	if !rule.SendCopy {
		// Forward mode: пересылаем оригиналы с атрибуцией.
		ids := make([]int64, 0, len(messages))
		for _, m := range messages {
			ids = append(ids, m.Id)
		}
		if _, err := s.telegramRepo.ForwardMessages(&client.ForwardMessagesRequest{
			ChatId:     dstChatID,
			FromChatId: messages[0].ChatId,
			MessageIds: ids,
		}); err != nil {
			s.logger.Error("Failed to forward media album", slog.Any("err", err))
		}
		return
	}

	// Copy mode: реконструируем контент и отправляем без атрибуции.
	contents := make([]client.InputMessageContent, 0, len(messages))
	for _, msg := range messages {
		text := s.messageService.GetFormattedText(msg)
		content := s.messageService.BuildInputContent(msg, text)
		contents = append(contents, content)
	}

	sent, err := s.telegramRepo.SendMessageAlbum(&client.SendMessageAlbumRequest{
		ChatId:               dstChatID,
		InputMessageContents: contents,
	})
	if err != nil {
		s.logger.Error("Failed to send media album copy", slog.Any("err", err))
		return
	}

	// Сохраняем маппинг для edit/delete sync.
	ruleID := rule.ID
	for i, m := range sent.Messages {
		if i >= len(messages) || m == nil {
			break
		}
		toChatMsgID := fmt.Sprintf("%s:%d:%d", ruleID, dstChatID, m.Id)
		if err := s.stateRepo.SetCopiedMessageID(messages[i].ChatId, messages[i].Id, toChatMsgID); err != nil {
			s.logger.Error("Failed to set copied message ID", slog.Any("err", err))
		}
	}
}

// editMessagesWithRetry обрабатывает редактирование с retry до 3 раз.
func (s *Service) editMessagesWithRetry(ctx context.Context, rs *domain.RuleSet, msg *client.Message, attempt int) {
	needRetry := s.editMessages(ctx, rs, msg)
	if needRetry && attempt < 3 {
		s.taskQueue.Add(func() {
			s.editMessagesWithRetry(ctx, rs, msg, attempt+1)
		})
	}
}

// editMessages возвращает true если нужен retry (permanent ID ещё не в storage).
func (s *Service) editMessages(ctx context.Context, rs *domain.RuleSet, msg *client.Message) bool {
	copies := s.stateRepo.GetCopiedMessageIDs(msg.ChatId, msg.Id)
	if len(copies) == 0 {
		return false
	}

	src := rs.Sources[msg.ChatId]
	text := s.messageService.GetFormattedText(msg)
	if text == nil {
		return false
	}

	_, isText := msg.Content.(*client.MessageText)

	needRetry := false

	for _, c := range copies {
		ref, ok := parseCopyRef(c)
		if !ok {
			continue
		}

		newMsgID := s.stateRepo.GetNewMessageID(ref.dstChatID, ref.tmpMsgID)
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
			// Версионирование: отправляем новую копию.
			s.forwardMessage(ctx, ref.ruleID, rule, msg, text, src, dst, ref.dstChatID, newMsgID, true)
			continue
		}

		// Обновляем существующую копию.
		transformed, err := s.transformService.Transform(ctx, domain.TransformParams{
			Text:         text,
			Source:       src,
			Destination:  dst,
			DstChatID:    ref.dstChatID,
			SrcChatID:    msg.ChatId,
			SrcMessageID: msg.Id,
			WithSources:  true,
			ReplyMarkup:  s.messageService.GetReplyMarkupData(msg),
		})
		if err != nil {
			s.logger.Error("Failed to transform edited message", slog.Any("err", err))
			continue
		}
		if isText {
			if _, err := s.telegramRepo.EditMessageText(&client.EditMessageTextRequest{
				ChatId:              ref.dstChatID,
				MessageId:           newMsgID,
				InputMessageContent: &client.InputMessageText{Text: transformed},
			}); err != nil {
				s.logger.Error("Failed to edit message text", slog.Any("err", err))
			}
		} else {
			if _, err := s.telegramRepo.EditMessageCaption(&client.EditMessageCaptionRequest{
				ChatId:    ref.dstChatID,
				MessageId: newMsgID,
				Caption:   transformed,
			}); err != nil {
				s.logger.Error("Failed to edit message caption", slog.Any("err", err))
			}
		}

		// Reply markup sync.
		replyData := s.messageService.GetReplyMarkupData(msg)
		if len(replyData) > 0 {
			if err := s.stateRepo.SetAnswerMessageID(ref.dstChatID, ref.tmpMsgID, msg.ChatId, msg.Id); err != nil {
				s.logger.Error("Failed to set answer message ID", slog.Any("err", err))
			}
		} else {
			if err := s.stateRepo.DeleteAnswerMessageID(ref.dstChatID, ref.tmpMsgID); err != nil {
				s.logger.Error("Failed to delete answer message ID", slog.Any("err", err))
			}
		}
	}

	return needRetry
}

// deleteMessagesWithRetry обрабатывает удаление с retry до 3 раз.
func (s *Service) deleteMessagesWithRetry(ctx context.Context, rs *domain.RuleSet, chatID int64, messageIDs []int64, attempt int) {
	needRetry := s.deleteMessages(ctx, rs, chatID, messageIDs)
	if needRetry && attempt < 3 {
		s.taskQueue.Add(func() {
			s.deleteMessagesWithRetry(ctx, rs, chatID, messageIDs, attempt+1)
		})
	}
}

// deleteMessages возвращает true если нужен retry.
func (s *Service) deleteMessages(_ context.Context, rs *domain.RuleSet, chatID int64, messageIDs []int64) bool {
	needRetry := false

	for _, msgID := range messageIDs {
		copies := s.stateRepo.GetCopiedMessageIDs(chatID, msgID)
		if len(copies) == 0 {
			continue
		}

		for _, c := range copies {
			ref, ok := parseCopyRef(c)
			if !ok {
				continue
			}

			rule := rs.ForwardRules[ref.ruleID]
			if rule != nil && rule.Indelible {
				continue
			}

			newMsgID := s.stateRepo.GetNewMessageID(ref.dstChatID, ref.tmpMsgID)
			if newMsgID == 0 {
				needRetry = true
				continue
			}

			if _, err := s.telegramRepo.DeleteMessages(&client.DeleteMessagesRequest{
				ChatId:     ref.dstChatID,
				MessageIds: []int64{newMsgID},
				Revoke:     true,
			}); err != nil {
				s.logger.Error("Failed to delete copied message", slog.Any("err", err))
			}
			if err := s.stateRepo.DeleteNewMessageID(ref.dstChatID, ref.tmpMsgID); err != nil {
				s.logger.Error("Failed to delete new message ID", slog.Any("err", err))
			}
			if err := s.stateRepo.DeleteTmpMessageID(ref.dstChatID, newMsgID); err != nil {
				s.logger.Error("Failed to delete tmp message ID", slog.Any("err", err))
			}
			if err := s.stateRepo.DeleteAnswerMessageID(ref.dstChatID, ref.tmpMsgID); err != nil {
				s.logger.Error("Failed to delete answer message ID", slog.Any("err", err))
			}
		}

		if !needRetry {
			if err := s.stateRepo.DeleteCopiedMessageIDs(chatID, msgID); err != nil {
				s.logger.Error("Failed to delete copied message IDs", slog.Any("err", err))
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
