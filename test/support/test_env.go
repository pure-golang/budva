package support

import (
	"context"
	"errors"
	"os"

	"github.com/pure-golang/budva-claude/internal/config"
	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/handler"
	"github.com/pure-golang/budva-claude/internal/repo/queue"
	"github.com/pure-golang/budva-claude/internal/repo/state"
	"github.com/pure-golang/budva-claude/internal/service/album"
	"github.com/pure-golang/budva-claude/internal/service/dedup"
	"github.com/pure-golang/budva-claude/internal/service/filters"
	"github.com/pure-golang/budva-claude/internal/service/limiter"
	"github.com/pure-golang/budva-claude/internal/service/message"
	"github.com/pure-golang/budva-claude/internal/service/transform"
)

// TelegramGateway — интерфейс для подмены Telegram в тестах.
// FakeTelegram (in-memory) реализует его сейчас; при переходе на Variant 3
// будет заменён на реальный TDLib с Test DC без изменения TestEnv.
type TelegramGateway interface {
	SendMessage(ctx context.Context, chatID domain.ChatID, content domain.InputMessageContent) (domain.MessageID, error)
	SendMessageAlbum(ctx context.Context, chatID domain.ChatID, contents []domain.InputMessageContent) ([]domain.MessageID, error)
	ForwardMessages(ctx context.Context, fromChatID domain.ChatID, toChatID domain.ChatID, messageIDs []domain.MessageID) ([]domain.MessageID, error)
	GetMessage(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID) (*domain.Message, error)
	EditMessageText(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID, text *domain.FormattedText) error
	EditMessageCaption(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID, text *domain.FormattedText) error
	DeleteMessages(ctx context.Context, chatID domain.ChatID, messageIDs []domain.MessageID, revoke bool) error
	GetMessageLink(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID) (string, error)
	GetMessageLinkInfo(ctx context.Context, url string) (*domain.MessageLinkInfo, error)
	TranslateText(ctx context.Context, text *domain.FormattedText, lang string) (*domain.FormattedText, error)
	GetCallbackQueryAnswer(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID, data []byte) (string, error)
	GetChatType(ctx context.Context, chatID domain.ChatID) (string, error)
	ParseTextEntities(ctx context.Context, text string) (*domain.FormattedText, error)
	GetChatHistory(ctx context.Context, chatID domain.ChatID, fromMessageID domain.MessageID, offset int32, limit int32) ([]*domain.Message, error)
	ClientDone() <-chan struct{}
	GetOption(ctx context.Context, name string) (string, error)
	GetMe(ctx context.Context) (int64, error)
}

// TestEnv содержит собранный стек для BDD/integration тестов.
type TestEnv struct {
	Telegram     TelegramGateway
	TelegramFake *FakeTelegram // assertion-методы: PutMessage, MessagesInChat, HasMessageWithText
	Handler      *handler.Handler
	State        *state.Repo
	Queue        *queue.Repo
	SourceID     domain.ChatID
	TargetIDs    []domain.ChatID
	tmpDir       string
}

// NewTestEnv собирает полный стек: handler + real services + fake telegram + real BadgerDB.
func NewTestEnv() (*TestEnv, error) {
	tmpDir, err := os.MkdirTemp("", "budva-bdd-*")
	if err != nil {
		return nil, err
	}

	fakeTG := NewFakeTelegram()

	stateRepo := state.New(config.StorageConfig{
		DatabaseDirectory: tmpDir,
	})
	if err := stateRepo.Start(context.Background()); err != nil {
		os.RemoveAll(tmpDir) //nolint:errcheck // Best-effort cleanup при ошибке start
		return nil, err
	}

	queueRepo := queue.New()

	messageSvc := message.New()
	transformSvc := transform.New(fakeTG, stateRepo)
	filtersSvc := filters.New()
	albumSvc := album.New()
	limiterSvc := limiter.New()

	h := handler.New(
		fakeTG,
		stateRepo,
		messageSvc,
		filtersSvc,
		transformSvc,
		albumSvc,
		queueRepo,
		limiterSvc,
		func(dsts []domain.ChatID) handler.DedupTracker {
			return dedup.NewTracker(dsts)
		},
	)

	return &TestEnv{
		Telegram:     fakeTG,
		TelegramFake: fakeTG,
		Handler:      h,
		State:        stateRepo,
		Queue:        queueRepo,
		SourceID:     -1001000,
		TargetIDs:    []domain.ChatID{-1002000, -1003000},
		tmpDir:       tmpDir,
	}, nil
}

// Close освобождает ресурсы.
func (e *TestEnv) Close() error {
	var errs []error
	if e.State != nil {
		errs = append(errs, e.State.Close())
	}
	if e.tmpDir != "" {
		errs = append(errs, os.RemoveAll(e.tmpDir))
	}
	return errors.Join(errs...)
}

// MakeRuleSet создаёт RuleSet с одним правилом source→targets.
func (e *TestEnv) MakeRuleSet(sendCopy bool, src *domain.Source) *domain.RuleSet {
	if src == nil {
		src = &domain.Source{ChatID: e.SourceID}
	}
	src.ChatID = e.SourceID

	rule := &domain.ForwardRule{
		ID:       "test_rule",
		From:     e.SourceID,
		To:       e.TargetIDs,
		SendCopy: sendCopy,
	}

	rs := &domain.RuleSet{
		Sources:             map[domain.ChatID]*domain.Source{e.SourceID: src},
		Destinations:        make(map[domain.ChatID]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		UniqueSources:       map[domain.ChatID]struct{}{e.SourceID: {}},
		UniqueDestinations:  make(map[domain.ChatID]struct{}),
		OrderedForwardRules: []string{rule.ID},
	}
	for _, id := range e.TargetIDs {
		rs.UniqueDestinations[id] = struct{}{}
		rs.Destinations[id] = &domain.Destination{ChatID: id}
	}

	return rs
}

// DrainQueue синхронно выполняет все задачи в очереди.
func (e *TestEnv) DrainQueue() {
	e.Queue.ProcessAll()
}
