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

// Stack содержит собранный стек для BDD/integration тестов.
type Stack struct {
	Telegram  *FakeTelegram
	Handler   *handler.Handler
	State     *state.Repo
	Queue     *queue.Repo
	SourceID  domain.ChatID
	TargetIDs []domain.ChatID
	tmpDir    string
}

// NewStack собирает полный стек: handler + real services + fake telegram + real BadgerDB.
func NewStack() (*Stack, error) {
	tmpDir, err := os.MkdirTemp("", "budva-bdd-*")
	if err != nil {
		return nil, err
	}

	telegram := NewFakeTelegram()

	stateRepo := state.New(config.StorageConfig{
		DatabaseDirectory: tmpDir,
	})
	if err := stateRepo.Start(context.Background()); err != nil {
		os.RemoveAll(tmpDir) //nolint:errcheck // Best-effort cleanup при ошибке start
		return nil, err
	}

	queueRepo := queue.New()

	messageService := message.New()
	transformService := transform.New(telegram, stateRepo)
	filterService := filters.New()
	albumService := album.New()
	limiterService := limiter.New()

	h := handler.New(
		telegram,
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

	return &Stack{
		Telegram:  telegram,
		Handler:   h,
		State:     stateRepo,
		Queue:     queueRepo,
		SourceID:  -1001000,
		TargetIDs: []domain.ChatID{-1002000, -1003000},
		tmpDir:    tmpDir,
	}, nil
}

// Close освобождает ресурсы.
func (e *Stack) Close() error {
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
func (e *Stack) MakeRuleSet(sendCopy bool, src *domain.Source) *domain.RuleSet {
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
func (e *Stack) DrainQueue() {
	e.Queue.ProcessAll()
}
