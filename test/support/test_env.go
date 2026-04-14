package support

import (
	"context"
	"os"

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

	"github.com/pure-golang/budva-claude/internal/config"
)

// TestEnv содержит собранный стек для BDD/integration тестов.
type TestEnv struct {
	Telegram  *FakeTelegram
	Handler   *handler.Handler
	State     *state.Repo
	Queue     *queue.Repo
	SourceID  domain.ChatID
	TargetIDs []domain.ChatID
	tmpDir    string
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
		os.RemoveAll(tmpDir)
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
		Telegram:  fakeTG,
		Handler:   h,
		State:     stateRepo,
		Queue:     queueRepo,
		SourceID:  -1001000,
		TargetIDs: []domain.ChatID{-1002000, -1003000},
		tmpDir:    tmpDir,
	}, nil
}

// Close освобождает ресурсы.
func (e *TestEnv) Close() {
	if e.State != nil {
		e.State.Close()
	}
	if e.tmpDir != "" {
		os.RemoveAll(e.tmpDir)
	}
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
