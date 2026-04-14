package integration

import (
	"context"
	"testing"

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
	"github.com/pure-golang/budva-claude/test/support"
)

type integrationSuite struct {
	telegram *support.FakeTelegram
	handler  *handler.Handler
	state    *state.Repo
	queue    *queue.Repo
	sourceID domain.ChatID
	targets  []domain.ChatID
}

func setupSuite(tb testing.TB) *integrationSuite {
	tb.Helper()

	stateRepo := state.New(config.StorageConfig{DatabaseDirectory: tb.(*testing.T).TempDir()})
	if err := stateRepo.Start(context.Background()); err != nil {
		tb.Fatalf("failed to start state repo: %v", err)
	}

	fakeTG := support.NewFakeTelegram()
	queueRepo := queue.New()
	messageSvc := message.New()
	transformSvc := transform.New(fakeTG, stateRepo)
	filtersSvc := filters.New()
	albumSvc := album.New()
	limiterSvc := limiter.New()

	h := handler.New(
		fakeTG, stateRepo, messageSvc, filtersSvc, transformSvc,
		albumSvc, queueRepo, limiterSvc,
		func(dsts []domain.ChatID) handler.DedupTracker { return dedup.NewTracker(dsts) },
	)

	return &integrationSuite{
		telegram: fakeTG,
		handler:  h,
		state:    stateRepo,
		queue:    queueRepo,
		sourceID: -1001000,
		targets:  []domain.ChatID{-1002000, -1003000},
	}
}

func tearDownSuite(tb testing.TB, s *integrationSuite) {
	tb.Helper()
	if s.state != nil {
		s.state.Close()
	}
}

func resetState(tb testing.TB, s *integrationSuite) {
	tb.Helper()
	s.telegram.Reset()
}

func (s *integrationSuite) makeRuleSet(sendCopy bool) *domain.RuleSet {
	src := &domain.Source{ChatID: s.sourceID}
	rule := &domain.ForwardRule{
		ID: "test_rule", From: s.sourceID, To: s.targets, SendCopy: sendCopy,
	}
	rs := &domain.RuleSet{
		Sources:             map[domain.ChatID]*domain.Source{s.sourceID: src},
		Destinations:        make(map[domain.ChatID]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		UniqueSources:       map[domain.ChatID]struct{}{s.sourceID: {}},
		UniqueDestinations:  make(map[domain.ChatID]struct{}),
		OrderedForwardRules: []string{rule.ID},
	}
	for _, id := range s.targets {
		rs.UniqueDestinations[id] = struct{}{}
		rs.Destinations[id] = &domain.Destination{ChatID: id}
	}
	return rs
}
