package support

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
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
	Telegram  *telegram.Repo
	Handler   *handler.Handler
	State     *state.Repo
	Queue     *queue.Repo
	Fixtures  *Fixtures
	SourceID  domain.ChatID
	TargetIDs []domain.ChatID
	tmpDir    string
}

// NewLiveStack собирает полный стек с реальным TDLib и тестовыми чатами из фикстур.
// Требует: TDLib собран, .env с реальными credentials, cmd/stand --up выполнен.
func NewLiveStack(ctx context.Context, fixturesPath string) (*LiveStack, error) {
	var cfg config.TelegramConfig
	if err := aenv.InitConfig(&cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	fixtures, err := LoadFixtures(fixturesPath)
	if err != nil {
		return nil, fmt.Errorf("load fixtures: %w", err)
	}

	telegramRepo := telegram.New(cfg)
	if err := telegramRepo.Start(ctx); err != nil {
		return nil, fmt.Errorf("telegram start: %w", err)
	}

	// Ждём авторизации (сессия должна быть закеширована после cmd/stand --up)
	select {
	case <-ctx.Done():
		telegramRepo.Close() //nolint:errcheck // Best-effort cleanup при ошибке инициализации
		return nil, ctx.Err()
	case <-telegramRepo.ClientDone():
	case <-time.After(30 * time.Second):
		telegramRepo.Close() //nolint:errcheck // Best-effort cleanup при ошибке инициализации
		return nil, fmt.Errorf("authorization timeout: ensure .env is configured and session is cached")
	}

	tmpDir, err := os.MkdirTemp("", "budva-bdd-*")
	if err != nil {
		telegramRepo.Close() //nolint:errcheck // Best-effort cleanup при ошибке инициализации
		return nil, err
	}

	stateRepo := state.New(config.StorageConfig{DatabaseDirectory: tmpDir}) //nolint:exhaustruct // Только путь нужен для temp DB
	if err := stateRepo.Start(context.Background()); err != nil {
		os.RemoveAll(tmpDir) //nolint:errcheck // Best-effort cleanup при ошибке инициализации
		telegramRepo.Close() //nolint:errcheck // Best-effort cleanup при ошибке инициализации
		return nil, err
	}

	queueRepo := queue.New()
	messageService := message.New()
	transformService := transform.New(telegramRepo, stateRepo)
	filterService := filters.New()
	albumService := album.New()
	limiterService := limiter.New()

	h := handler.New(
		telegramRepo,
		stateRepo,
		messageService,
		filterService,
		transformService,
		albumService,
		queueRepo,
		limiterService,
		func(dsts []domain.ChatID) handler.DedupTracker {
			return dedup.NewTracker(dsts)
		},
	)

	// Определяем source и targets из фикстур: "исходный*" → sources, "целевой/целевая*" → targets
	var sourceID domain.ChatID
	var targetIDs []domain.ChatID
	for _, chat := range fixtures.Chats {
		if strings.HasPrefix(chat.Name, "целевой") || strings.HasPrefix(chat.Name, "целевая") {
			targetIDs = append(targetIDs, chat.ChatID)
		} else if strings.HasPrefix(chat.Name, "исходный") || strings.HasPrefix(chat.Name, "исходная") {
			if sourceID == 0 {
				sourceID = chat.ChatID
			}
		}
	}

	return &LiveStack{
		Telegram:  telegramRepo,
		Handler:   h,
		State:     stateRepo,
		Queue:     queueRepo,
		Fixtures:  fixtures,
		SourceID:  sourceID,
		TargetIDs: targetIDs,
		tmpDir:    tmpDir,
	}, nil
}

// Close освобождает ресурсы.
func (s *LiveStack) Close() error {
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
// SendMessage возвращает temporary ID; permanent ID приходит через UpdateMessageSendSucceeded.
func (s *LiveStack) PutMessage(ctx context.Context, chatID domain.ChatID, content domain.InputMessageContent, prefix string) (*domain.Message, error) {
	// Добавляем prefix к тексту для идентификации сообщения сценария
	if content.Text != nil && prefix != "" {
		content.Text = &domain.FormattedText{
			Text:     PrefixText(prefix, content.Text.Text),
			Entities: content.Text.Entities,
		}
	}

	msgID, err := s.Telegram.SendMessage(ctx, chatID, content)
	if err != nil {
		return nil, fmt.Errorf("put message: %w", err)
	}
	return &domain.Message{
		ChatID:     chatID,
		ID:         msgID,
		CanBeSaved: true,
		Content: domain.MessageContent{
			Type: content.Type,
			Text: content.Text,
		},
	}, nil
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
