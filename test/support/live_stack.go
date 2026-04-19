package support

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	aenv "github.com/pure-golang/adapters/env"
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/config"
	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/handler"
	"github.com/pure-golang/budva-claude/internal/repo/queue"
	"github.com/pure-golang/budva-claude/internal/repo/state"
	"github.com/pure-golang/budva-claude/internal/repo/telegram"
	"github.com/pure-golang/budva-claude/internal/service/album"
	"github.com/pure-golang/budva-claude/internal/service/dedup"
	"github.com/pure-golang/budva-claude/internal/service/filters"
	"github.com/pure-golang/budva-claude/internal/service/limiter"
	"github.com/pure-golang/budva-claude/internal/service/message"
	"github.com/pure-golang/budva-claude/internal/service/transform"
)

// putMinInterval — минимальный интервал между отправками test-side в один чат.
// Telegram рейтит ~1 msg/sec в чат; 1.2 сек даёт комфортный запас против
// FLOOD_WAIT, не раздувая общее время BDD-прогона.
const putMinInterval = 1200 * time.Millisecond

// LiveStack содержит собранный стек для BDD-тестов с реальным TDLib.
type LiveStack struct {
	Telegram      *telegram.Repo
	Handler       *handler.Handler
	State         *state.Repo
	Queue         *queue.Repo
	Fixtures      *Fixtures
	SourceID      int64
	TargetIDs     []int64
	fixturesPath  string
	tmpDir        string
	cancelUpdates context.CancelFunc
	mu            sync.RWMutex // защищает State и Handler от race между processUpdates и ResetState

	// putMu и putLastSend реализуют per-chat throttle для PutMessage/PutAlbum.
	// Handler внутри уже использует limiter.Service для форвардов; это
	// отдельный throttle для прямых test-side отправок.
	putMu       sync.Mutex
	putLastSend map[int64]time.Time
}

// NewLiveStack создаёт экземпляр стека для BDD-тестов.
func NewLiveStack(fixturesPath string) *LiveStack {
	return &LiveStack{fixturesPath: fixturesPath}
}

// Start инициализирует TDLib, загружает фикстуры, ждёт авторизации, собирает handler pipeline.
func (s *LiveStack) Start() error {
	var cfg config.TelegramConfig
	if err := aenv.InitConfig(&cfg); err != nil {
		return fmt.Errorf("config: %w", err)
	}

	fixtures, err := LoadFixtures(s.fixturesPath)
	if err != nil {
		return fmt.Errorf("load fixtures: %w", err)
	}
	s.Fixtures = fixtures

	ctx, cancel := context.WithCancel(context.Background())
	s.cancelUpdates = cancel

	telegramRepo := telegram.New(cfg)
	if err := telegramRepo.Start(ctx); err != nil {
		cancel()
		return fmt.Errorf("telegram start: %w", err)
	}

	for {
		select {
		case <-telegramRepo.ClientDone():
			goto authorized
		case ev := <-telegramRepo.AuthStates():
			switch ev.State {
			case domain.AuthStateReady:
				goto authorized
			case domain.AuthStateWaitPhone, domain.AuthStateWaitCode, domain.AuthStateWaitPassword:
				cancel()
				telegramRepo.Close() //nolint:errcheck // Best-effort cleanup
				return fmt.Errorf("TDLib requires interactive auth (%s): run cmd/stand --up first", ev.State)
			case domain.AuthStateClosed:
				cancel()
				return fmt.Errorf("TDLib session closed unexpectedly")
			}
		case <-time.After(30 * time.Second):
			cancel()
			telegramRepo.Close() //nolint:errcheck // Best-effort cleanup
			return fmt.Errorf("authorization timeout: ensure .env is configured and session is cached")
		}
	}
authorized:

	// Прогреваем кеш чатов.
	_, _ = telegramRepo.LoadChats(&client.LoadChatsRequest{Limit: 100}) //nolint:errcheck // TDLib возвращает ошибку когда чатов меньше limit

	tmpDir, err := os.MkdirTemp("", "budva-bdd-*")
	if err != nil {
		cancel()
		telegramRepo.Close() //nolint:errcheck // Best-effort cleanup
		return err
	}

	stateRepo := state.New(config.StorageConfig{DatabaseDirectory: tmpDir}) //nolint:exhaustruct // Только путь нужен для temp DB
	if err := stateRepo.Start(context.Background()); err != nil {
		cancel()
		os.RemoveAll(tmpDir) //nolint:errcheck // Best-effort cleanup
		telegramRepo.Close() //nolint:errcheck // Best-effort cleanup
		return err
	}

	s.Telegram = telegramRepo
	s.State = stateRepo
	s.Queue = queue.New()
	s.tmpDir = tmpDir
	s.putLastSend = make(map[int64]time.Time)
	s.Handler = handler.New(
		telegramRepo,
		stateRepo,
		message.New(),
		filters.New(),
		transform.New(telegramRepo, stateRepo),
		album.New(),
		s.Queue,
		limiter.New(),
		func(dsts []int64) handler.DedupTracker {
			return dedup.NewTracker(dsts)
		},
	)

	for _, chat := range s.Fixtures.Chats {
		if strings.HasPrefix(chat.Name, "целевой") || strings.HasPrefix(chat.Name, "целевая") {
			s.TargetIDs = append(s.TargetIDs, chat.ChatID)
		} else if (strings.HasPrefix(chat.Name, "исходный") || strings.HasPrefix(chat.Name, "исходная")) && s.SourceID == 0 {
			s.SourceID = chat.ChatID
		}
	}

	go s.processUpdates(ctx)

	return nil
}

// processUpdates читает Telegram updates: записывает temp→permanent ID напрямую в state
// и делегирует edit/delete handler-у. Resolve edit (GetMessage) — тоже здесь, потому
// что handler.OnEditedMessage принимает уже резольвнутый *client.Message.
func (s *LiveStack) processUpdates(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case upd, ok := <-s.Telegram.Updates():
			if !ok {
				return
			}
			s.mu.RLock()
			st := s.State
			h := s.Handler
			s.mu.RUnlock()

			switch u := upd.(type) {
			case *client.UpdateMessageSendSucceeded:
				if u.Message != nil {
					_ = st.SetNewMessageID(u.Message.ChatId, u.OldMessageId, u.Message.Id) //nolint:errcheck // Best-effort
					_ = st.SetTmpMessageID(u.Message.ChatId, u.Message.Id, u.OldMessageId) //nolint:errcheck // Best-effort
				}
			case *client.UpdateMessageEdited:
				// Resolve в отдельной горутине: GetMessage синхронный и при медленном
				// TDLib ответе блокирует receiver → другие listener-ы (SendMessageAndWait)
				// не получают UpdateMessageSendSucceeded → timeout.
				go func(chatID, msgID int64) {
					msg, err := s.Telegram.GetMessage(&client.GetMessageRequest{
						ChatId:    chatID,
						MessageId: msgID,
					})
					if err == nil {
						h.OnEditedMessage(ctx, msg)
					}
				}(u.ChatId, u.MessageId)
			case *client.UpdateDeleteMessages:
				if u.IsPermanent {
					h.OnDeletedMessages(ctx, u.ChatId, u.MessageIds, u.IsPermanent)
				}
			}
		}
	}
}

// Close освобождает ресурсы.
func (s *LiveStack) Close() error {
	if s.cancelUpdates != nil {
		s.cancelUpdates()
	}
	var errs []error
	if s.State != nil {
		errs = append(errs, s.State.Close())
	}
	if s.Telegram != nil {
		errs = append(errs, s.Telegram.Close())
	}
	if s.tmpDir != "" {
		errs = append(errs, os.RemoveAll(s.tmpDir))
	}
	return errors.Join(errs...)
}

// MakeRuleSet создаёт RuleSet с одним правилом source→targets.
func (s *LiveStack) MakeRuleSet(sendCopy bool, src *domain.Source) *domain.RuleSet {
	if src == nil {
		src = &domain.Source{ChatID: s.SourceID}
	}
	src.ChatID = s.SourceID

	rule := &domain.ForwardRule{
		ID:       "test_rule",
		From:     s.SourceID,
		To:       s.TargetIDs,
		SendCopy: sendCopy,
	}

	rs := &domain.RuleSet{
		Sources:             map[int64]*domain.Source{s.SourceID: src},
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		UniqueSources:       map[int64]struct{}{s.SourceID: {}},
		UniqueDestinations:  make(map[int64]struct{}),
		OrderedForwardRules: []string{rule.ID},
	}
	for _, id := range s.TargetIDs {
		rs.UniqueDestinations[id] = struct{}{}
		rs.Destinations[id] = &domain.Destination{ChatID: id}
	}

	return rs
}

// ResetState сбрасывает BadgerDB, очередь и handler между сценариями.
func (s *LiveStack) ResetState() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State != nil {
		if err := s.State.Close(); err != nil {
			return err
		}
	}
	if s.tmpDir != "" {
		os.RemoveAll(s.tmpDir) //nolint:errcheck // Best-effort cleanup
	}
	tmpDir, err := os.MkdirTemp("", "budva-bdd-*")
	if err != nil {
		return err
	}
	s.tmpDir = tmpDir

	stateRepo := state.New(config.StorageConfig{DatabaseDirectory: tmpDir})
	if err := stateRepo.Start(context.Background()); err != nil {
		return err
	}
	s.State = stateRepo
	s.Queue = queue.New()

	s.Handler = handler.New(
		s.Telegram,
		s.State,
		message.New(),
		filters.New(),
		transform.New(s.Telegram, s.State),
		album.New(),
		s.Queue,
		limiter.New(),
		func(dsts []int64) handler.DedupTracker {
			return dedup.NewTracker(dsts)
		},
	)
	return nil
}

// DrainQueue синхронно выполняет все задачи в очереди.
func (s *LiveStack) DrainQueue() {
	s.Queue.ProcessAll()
}

// PrefixText добавляет prefix сценария к тексту: "{prefix}\n\n{text}".
func PrefixText(prefix, text string) string {
	return fmt.Sprintf("%s\n\n%s", prefix, text)
}

// throttlePut выдерживает putMinInterval между test-side отправками в один чат,
// чтобы не получать FLOOD_WAIT от Telegram (rate ~1 msg/sec в чат).
func (s *LiveStack) throttlePut(ctx context.Context, chatID int64) {
	s.putMu.Lock()
	last, ok := s.putLastSend[chatID]
	s.putMu.Unlock()

	if ok {
		wait := putMinInterval - time.Since(last)
		if wait > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
		}
	}

	s.putMu.Lock()
	s.putLastSend[chatID] = time.Now()
	s.putMu.Unlock()
}

// PutMessage отправляет сообщение в чат через TDLib с prefix сценария.
// Блокирует до получения permanent ID через SendMessageAndWait.
func (s *LiveStack) PutMessage(ctx context.Context, chatID int64, content client.InputMessageContent, prefix string) (*client.Message, error) {
	return s.PutMessageReply(ctx, chatID, content, 0, prefix)
}

// PutMessageReply отправляет сообщение с reply-to в чат (если replyToMessageID != 0).
// Блокирует до получения permanent ID.
func (s *LiveStack) PutMessageReply(ctx context.Context, chatID int64, content client.InputMessageContent, replyToMessageID int64, prefix string) (*client.Message, error) {
	content = applyPrefix(content, prefix)
	s.throttlePut(ctx, chatID)
	req := &client.SendMessageRequest{
		ChatId:              chatID,
		InputMessageContent: content,
	}
	if replyToMessageID != 0 {
		req.ReplyTo = &client.InputMessageReplyToMessage{MessageId: replyToMessageID}
	}
	msg, err := s.Telegram.SendMessageAndWait(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("put message: %w", err)
	}
	return msg, nil
}

// PutAlbum отправляет медиа-альбом в чат через TDLib.
// Добавляет prefix к caption первого элемента. Поллит историю чата до появления альбома.
func (s *LiveStack) PutAlbum(ctx context.Context, chatID int64, contents []client.InputMessageContent, prefix string) ([]*client.Message, error) {
	if len(contents) > 0 && prefix != "" {
		contents[0] = applyPrefix(contents[0], prefix)
	}

	s.throttlePut(ctx, chatID)
	if _, err := s.Telegram.SendMessageAlbum(&client.SendMessageAlbumRequest{
		ChatId:               chatID,
		InputMessageContents: contents,
	}); err != nil {
		return nil, fmt.Errorf("put album: %w", err)
	}

	deadline := time.After(15 * time.Second)
	for {
		msgs, err := s.Telegram.GetChatHistory(&client.GetChatHistoryRequest{
			ChatId: chatID,
			Limit:  10,
		})
		if err != nil {
			return nil, err
		}
		for _, m := range msgs.Messages {
			text := messageText(m)
			if m.MediaAlbumId != 0 && strings.HasPrefix(text, prefix) {
				aid := m.MediaAlbumId
				var result []*client.Message
				for _, am := range msgs.Messages {
					if am.MediaAlbumId == aid {
						result = append(result, am)
					}
				}
				return result, nil
			}
		}
		select {
		case <-deadline:
			return nil, fmt.Errorf("timeout: album with prefix %q not found in chat %d", prefix, chatID)
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// CheckLastMessage проверяет что последнее сообщение в чате содержит prefix сценария.
func (s *LiveStack) CheckLastMessage(_ context.Context, chatID int64, prefix string) (*client.Message, error) {
	deadline := time.After(10 * time.Second)
	for {
		msgs, err := s.Telegram.GetChatHistory(&client.GetChatHistoryRequest{
			ChatId: chatID,
			Limit:  1,
		})
		if err != nil {
			return nil, err
		}
		if len(msgs.Messages) > 0 {
			msg := msgs.Messages[0]
			if strings.HasPrefix(messageText(msg), prefix) {
				return msg, nil
			}
		}
		select {
		case <-deadline:
			var got string
			if len(msgs.Messages) > 0 {
				got = truncate(messageText(msgs.Messages[0]))
			}
			return nil, fmt.Errorf("timeout: last message in chat %d has wrong prefix: want %q, got %q",
				chatID, prefix, got)
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// CheckAlbumMessage ищет среди последних сообщений чата сообщение с указанным prefix.
func (s *LiveStack) CheckAlbumMessage(_ context.Context, chatID int64, prefix string) (*client.Message, error) {
	deadline := time.After(10 * time.Second)
	for {
		msgs, err := s.Telegram.GetChatHistory(&client.GetChatHistoryRequest{
			ChatId: chatID,
			Limit:  10,
		})
		if err != nil {
			return nil, err
		}
		for _, msg := range msgs.Messages {
			if strings.HasPrefix(messageText(msg), prefix) {
				return msg, nil
			}
		}
		select {
		case <-deadline:
			return nil, fmt.Errorf("timeout: no album message with prefix %q in chat %d", prefix, chatID)
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// CheckAlbumMessages проверяет что в чате появился альбом с prefix и возвращает его сообщения
// отсортированные по ID (возрастающий = порядок фото в альбоме).
func (s *LiveStack) CheckAlbumMessages(_ context.Context, chatID int64, prefix string, count int) ([]*client.Message, error) {
	deadline := time.After(10 * time.Second)
	for {
		msgs, err := s.Telegram.GetChatHistory(&client.GetChatHistoryRequest{
			ChatId: chatID,
			Limit:  int32(count * 2), //nolint:gosec // count всегда маленький
		})
		if err != nil {
			return nil, err
		}
		for _, m := range msgs.Messages {
			if strings.HasPrefix(messageText(m), prefix) && m.MediaAlbumId != 0 {
				aid := m.MediaAlbumId
				var result []*client.Message
				for _, am := range msgs.Messages {
					if am.MediaAlbumId == aid {
						result = append(result, am)
					}
				}
				sort.Slice(result, func(i, j int) bool {
					return result[i].Id < result[j].Id
				})
				if len(result) >= count {
					return result[:count], nil
				}
			}
		}
		select {
		case <-deadline:
			return nil, fmt.Errorf("timeout: album with prefix %q not found in chat %d", prefix, chatID)
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// CheckNoMessage проверяет что последнее сообщение НЕ содержит prefix (сообщение не доставлено).
func (s *LiveStack) CheckNoMessage(_ context.Context, chatID int64, prefix string) error {
	time.Sleep(3 * time.Second)
	msgs, err := s.Telegram.GetChatHistory(&client.GetChatHistoryRequest{
		ChatId: chatID,
		Limit:  1,
	})
	if err != nil {
		return err
	}
	if len(msgs.Messages) == 0 {
		return nil
	}
	if strings.HasPrefix(messageText(msgs.Messages[0]), prefix) {
		return fmt.Errorf("unexpected message with prefix %q in chat %d", prefix, chatID)
	}
	return nil
}

// ChatByName возвращает фикстуру по имени.
func (s *LiveStack) ChatByName(name string) (ChatFixture, error) {
	return s.Fixtures.ChatByName(name)
}

// messageText возвращает текст/подпись для prefix-проверки.
func messageText(msg *client.Message) string {
	if msg == nil || msg.Content == nil {
		return ""
	}
	switch c := msg.Content.(type) {
	case *client.MessageText:
		if c.Text != nil {
			return c.Text.Text
		}
	case *client.MessagePhoto:
		if c.Caption != nil {
			return c.Caption.Text
		}
	case *client.MessageVideo:
		if c.Caption != nil {
			return c.Caption.Text
		}
	case *client.MessageDocument:
		if c.Caption != nil {
			return c.Caption.Text
		}
	case *client.MessageAudio:
		if c.Caption != nil {
			return c.Caption.Text
		}
	case *client.MessageAnimation:
		if c.Caption != nil {
			return c.Caption.Text
		}
	}
	return ""
}

// applyPrefix заменяет Text/Caption в содержимом на prefix-версию.
func applyPrefix(content client.InputMessageContent, prefix string) client.InputMessageContent {
	if prefix == "" {
		return content
	}
	switch c := content.(type) {
	case *client.InputMessageText:
		c.Text = prefixFormatted(c.Text, prefix)
		return c
	case *client.InputMessagePhoto:
		c.Caption = prefixFormatted(c.Caption, prefix)
		return c
	case *client.InputMessageVideo:
		c.Caption = prefixFormatted(c.Caption, prefix)
		return c
	case *client.InputMessageDocument:
		c.Caption = prefixFormatted(c.Caption, prefix)
		return c
	case *client.InputMessageAudio:
		c.Caption = prefixFormatted(c.Caption, prefix)
		return c
	case *client.InputMessageAnimation:
		c.Caption = prefixFormatted(c.Caption, prefix)
		return c
	}
	return content
}

func prefixFormatted(ft *client.FormattedText, prefix string) *client.FormattedText {
	base := ""
	if ft != nil {
		base = ft.Text
	}
	return &client.FormattedText{Text: PrefixText(prefix, base)}
}

func truncate(s string) string {
	if s == "" {
		return "<nil>"
	}
	if len(s) > 50 {
		return s[:50] + "..."
	}
	return s
}
