package integration

import (
	"context"
	"testing"
	"time"

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
	telegram  *support.FakeTelegram
	handler   *handler.Handler
	state     *state.Repo
	queue     *queue.Repo
	container *support.BadgerContainer
	sourceID  domain.ChatID
	targets   []domain.ChatID
}

func setupSuite(tb testing.TB) *integrationSuite {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container, err := support.StartBadgerContainer(ctx)
	if err != nil {
		tb.Fatalf("failed to start badger container: %v", err)
	}

	kv := container.NewKVStore()
	stateRepo := state.NewWithKV(kv)

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
		telegram:  fakeTG,
		handler:   h,
		state:     stateRepo,
		queue:     queueRepo,
		container: container,
		sourceID:  -1001000,
		targets:   []domain.ChatID{-1002000, -1003000},
	}
}

func tearDownSuite(tb testing.TB, s *integrationSuite) {
	tb.Helper()
	if s.container != nil {
		s.container.Stop(context.Background())
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
