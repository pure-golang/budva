package support

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	aenv "github.com/pure-golang/adapters/env"

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

// LiveStack содержит собранный стек для BDD-тестов с реальным TDLib.
type LiveStack struct {
	Telegram      *telegram.Repo
	Handler       *handler.Handler
	State         *state.Repo
	Queue         *queue.Repo
	Fixtures      *Fixtures
	SourceID      domain.ChatID
	TargetIDs     []domain.ChatID
	fixturesPath  string
	tmpDir        string
	cancelUpdates context.CancelFunc
	mu            sync.RWMutex // Защищает State и Handler от race между processUpdates и ResetState
}

// NewLiveStack создаёт экземпляр стека для BDD-тестов.
func NewLiveStack(fixturesPath string) *LiveStack {
	return &LiveStack{fixturesPath: fixturesPath}
}

// Start инициализирует TDLib, загружает фикстуры, ждёт авторизации, собирает handler pipeline.
// Требует: TDLib собран, .env с реальными credentials, cmd/stand --up выполнен.
// Context создаётся внутри и живёт до Close().
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

	// Long-lived контекст: listenUpdates и processUpdates живут до Close()
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelUpdates = cancel

	telegramRepo := telegram.New(cfg)
	if err := telegramRepo.Start(ctx); err != nil {
		cancel()
		return fmt.Errorf("telegram start: %w", err)
	}

	// Ждём авторизации. Сессия должна быть закеширована после cmd/stand --up.
	// Если TDLib запрашивает phone/code/password — fail-fast: интерактивная авторизация
	// в тестах невозможна, нужен предварительный cmd/stand --up.
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

	// Прогреваем кеш чатов — после чистого старта TDLib не знает про тестовые чаты
	_ = telegramRepo.LoadChats(ctx, 100) //nolint:errcheck // TDLib возвращает ошибку когда чатов меньше limit

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
	s.Handler = handler.New(
		telegramRepo,
		stateRepo,
		message.New(),
		filters.New(),
		transform.New(telegramRepo, stateRepo),
		album.New(),
		s.Queue,
		limiter.New(),
		func(dsts []domain.ChatID) handler.DedupTracker {
			return dedup.NewTracker(dsts)
		},
	)

	// Определяем source и targets из фикстур
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

// processUpdates читает Telegram updates и обрабатывает temp→permanent ID маппинг.
// Без этой горутины канал updates (capacity 100) переполнится и заблокирует
// go-tdlib receiver, что сломает SendMessageAndWait и другие операции.
//
// Маппинг записывается напрямую в state (минуя handler task queue), чтобы
// горутины handler (runNextLinkWorkflow) видели permanent ID сразу, не дожидаясь DrainQueue.
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

			switch upd.Type {
			case domain.UpdateMessageSendSucceeded:
				if upd.Message != nil {
					_ = st.SetNewMessageID(upd.Message.ChatID, upd.OldMessageID, upd.Message.ID) //nolint:errcheck // Best-effort в фоновой горутине
					_ = st.SetTmpMessageID(upd.Message.ChatID, upd.Message.ID, upd.OldMessageID) //nolint:errcheck // Best-effort в фоновой горутине
				}
			case domain.UpdateMessageEdited:
				if upd.Message != nil {
					h.OnEditedMessage(ctx, upd.Message)
				}
			case domain.UpdateDeleteMessages:
				if upd.IsPermanent {
					h.OnDeletedMessages(ctx, upd.ChatID, upd.MessageIDs, upd.IsPermanent)
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
		Sources:             map[domain.ChatID]*domain.Source{s.SourceID: src},
		Destinations:        make(map[domain.ChatID]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		UniqueSources:       map[domain.ChatID]struct{}{s.SourceID: {}},
		UniqueDestinations:  make(map[domain.ChatID]struct{}),
		OrderedForwardRules: []string{rule.ID},
	}
	for _, id := range s.TargetIDs {
		rs.UniqueDestinations[id] = struct{}{}
		rs.Destinations[id] = &domain.Destination{ChatID: id}
	}

	return rs
}

// ResetState сбрасывает BadgerDB, очередь и handler между сценариями (TDLib не пересоздаётся).
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

	// Пересоздаём handler с новыми state и queue (TDLib repo переиспользуется)
	s.Handler = handler.New(
		s.Telegram,
		s.State,
		message.New(),
		filters.New(),
		transform.New(s.Telegram, s.State),
		album.New(),
		s.Queue,
		limiter.New(),
		func(dsts []domain.ChatID) handler.DedupTracker {
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

// PutMessage отправляет сообщение в чат через TDLib с prefix сценария.
// Блокирует до получения permanent ID через SendMessageAndWait.
func (s *LiveStack) PutMessage(ctx context.Context, chatID domain.ChatID, content domain.InputMessageContent, prefix string) (*domain.Message, error) {
	// Добавляем prefix к тексту для идентификации сообщения сценария
	if content.Text != nil && prefix != "" {
		content.Text = &domain.FormattedText{
			Text:     PrefixText(prefix, content.Text.Text),
			Entities: content.Text.Entities,
		}
	}

	msg, err := s.Telegram.SendMessageAndWait(ctx, chatID, content)
	if err != nil {
		return nil, fmt.Errorf("put message: %w", err)
	}
	return msg, nil
}

// PutAlbum отправляет медиа-альбом в чат через TDLib.
// Добавляет prefix к caption первого фото. Поллит chat history до появления
// всех сообщений альбома с matching MediaAlbumID.
func (s *LiveStack) PutAlbum(ctx context.Context, chatID domain.ChatID, contents []domain.InputMessageContent, prefix string) ([]*domain.Message, error) {
	// Добавляем prefix к caption первого фото
	if len(contents) > 0 && prefix != "" {
		caption := ""
		if contents[0].Text != nil {
			caption = contents[0].Text.Text
		}
		contents[0].Text = &domain.FormattedText{
			Text: PrefixText(prefix, caption),
		}
	}

	_, err := s.Telegram.SendMessageAlbum(ctx, chatID, contents)
	if err != nil {
		return nil, fmt.Errorf("put album: %w", err)
	}

	// Поллим chat history до появления альбома с prefix
	deadline := time.After(15 * time.Second)
	for {
		msgs, err := s.Telegram.GetChatHistory(ctx, chatID, 0, 0, 10)
		if err != nil {
			return nil, err
		}
		// Ищем сообщение с prefix → определяем MediaAlbumID → собираем альбом
		for _, m := range msgs {
			if m.Content.Text != nil && strings.HasPrefix(m.Content.Text.Text, prefix) && m.MediaAlbumID != 0 {
				aid := m.MediaAlbumID
				var result []*domain.Message
				for _, am := range msgs {
					if am.MediaAlbumID == aid {
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
// Поллит до 10 секунд, потому что TDLib SendMessage асинхронный.
func (s *LiveStack) CheckLastMessage(ctx context.Context, chatID domain.ChatID, prefix string) (*domain.Message, error) {
	deadline := time.After(10 * time.Second)
	for {
		msgs, err := s.Telegram.GetChatHistory(ctx, chatID, 0, 0, 1)
		if err != nil {
			return nil, err
		}
		if len(msgs) > 0 {
			msg := msgs[0]
			if msg.Content.Text != nil && strings.HasPrefix(msg.Content.Text.Text, prefix) {
				return msg, nil
			}
		}
		select {
		case <-deadline:
			var got string
			if len(msgs) > 0 {
				got = truncate(msgs[0].Content.Text)
			}
			return nil, fmt.Errorf("timeout: last message in chat %d has wrong prefix: want %q, got %q",
				chatID, prefix, got)
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// CheckNoMessage проверяет что последнее сообщение НЕ содержит prefix (сообщение не доставлено).
// Ждёт 3 секунды чтобы убедиться что сообщение действительно не появится.
func (s *LiveStack) CheckNoMessage(ctx context.Context, chatID domain.ChatID, prefix string) error {
	time.Sleep(3 * time.Second)
	msgs, err := s.Telegram.GetChatHistory(ctx, chatID, 0, 0, 1)
	if err != nil {
		return err
	}
	if len(msgs) == 0 {
		return nil
	}
	msg := msgs[0]
	if msg.Content.Text != nil && strings.HasPrefix(msg.Content.Text.Text, prefix) {
		return fmt.Errorf("unexpected message with prefix %q in chat %d", prefix, chatID)
	}
	return nil
}

func truncate(ft *domain.FormattedText) string {
	if ft == nil {
		return "<nil>"
	}
	if len(ft.Text) > 50 {
		return ft.Text[:50] + "..."
	}
	return ft.Text
}

// ChatByName возвращает фикстуру по имени.
func (s *LiveStack) ChatByName(name string) (ChatFixture, error) {
	return s.Fixtures.ChatByName(name)
}
