package integration

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

var badgerContainer *support.BadgerContainer

func TestMain(m *testing.M) {
	ctx := context.Background()

	var err error
	badgerContainer, err = support.StartBadgerContainer(ctx)
	if err != nil {
		panic("failed to start badger container: " + err.Error())
	}

	code := m.Run()

	badgerContainer.Stop(ctx)
	os.Exit(code)
}

type testEnv struct {
	telegram *support.FakeTelegram
	handler  *handler.Handler
	state    *state.Repo
	queue    *queue.Repo
	sourceID domain.ChatID
	targets  []domain.ChatID
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	if testing.Short() {
		t.Skip("integration")
	}

	kv := badgerContainer.NewKVStore()
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

	return &testEnv{
		telegram: fakeTG,
		handler:  h,
		state:    stateRepo,
		queue:    queueRepo,
		sourceID: -1001000,
		targets:  []domain.ChatID{-1002000, -1003000},
	}
}

func (e *testEnv) makeRuleSet(sendCopy bool) *domain.RuleSet {
	src := &domain.Source{ChatID: e.sourceID}
	rule := &domain.ForwardRule{
		ID: "test_rule", From: e.sourceID, To: e.targets, SendCopy: sendCopy,
	}
	rs := &domain.RuleSet{
		Sources:             map[domain.ChatID]*domain.Source{e.sourceID: src},
		Destinations:        make(map[domain.ChatID]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		UniqueSources:       map[domain.ChatID]struct{}{e.sourceID: {}},
		UniqueDestinations:  make(map[domain.ChatID]struct{}),
		OrderedForwardRules: []string{rule.ID},
	}
	for _, id := range e.targets {
		rs.UniqueDestinations[id] = struct{}{}
		rs.Destinations[id] = &domain.Destination{ChatID: id}
	}
	return rs
}

func TestForwardPipeline_CopyWithTransform(t *testing.T) {
	// Arrange
	env := newTestEnv(t)
	rs := env.makeRuleSet(true)
	rs.Sources[env.sourceID].Sign = &domain.Sign{Title: "TestSign", For: env.targets}
	env.handler.SetRuleSet(rs)

	msg := &domain.Message{
		ChatID: env.sourceID, ID: 1, CanBeSaved: true,
		Content: domain.MessageContent{Type: domain.ContentText, Text: &domain.FormattedText{Text: "hello"}},
	}
	env.telegram.PutMessage(msg)

	// Act
	env.handler.OnNewMessage(context.Background(), msg)
	env.queue.ProcessAll()

	// Assert
	msgs := env.telegram.MessagesInChat(env.targets[0])
	require.NotEmpty(t, msgs)
	assert.Contains(t, msgs[0].Content.Text.Text, "TestSign")
}

func TestForwardPipeline_EditSync(t *testing.T) {
	// Arrange
	env := newTestEnv(t)
	rs := env.makeRuleSet(true)
	env.handler.SetRuleSet(rs)

	msg := &domain.Message{
		ChatID: env.sourceID, ID: 1, CanBeSaved: true,
		Content: domain.MessageContent{Type: domain.ContentText, Text: &domain.FormattedText{Text: "original"}},
	}
	env.telegram.PutMessage(msg)
	env.handler.OnNewMessage(context.Background(), msg)
	env.queue.ProcessAll()

	// Simulate send succeeded
	for _, target := range env.targets {
		for _, m := range env.telegram.MessagesInChat(target) {
			env.handler.OnMessageSendSucceeded(target, m.ID, m.ID)
		}
	}

	// Act — edit
	editMsg := &domain.Message{
		ChatID: env.sourceID, ID: 1, CanBeSaved: true,
		Content: domain.MessageContent{Type: domain.ContentText, Text: &domain.FormattedText{Text: "updated"}},
	}
	env.telegram.PutMessage(editMsg)
	env.handler.OnEditedMessage(context.Background(), editMsg)
	env.queue.ProcessAll()

	// Assert
	for _, target := range env.targets {
		msgs := env.telegram.MessagesInChat(target)
		require.NotEmpty(t, msgs)
		found := false
		for _, m := range msgs {
			if m.Content.Text != nil && m.Content.Text.Text == "updated" {
				found = true
			}
		}
		assert.True(t, found, "target %d should have updated message", target)
	}
}

func TestForwardPipeline_DeleteSync(t *testing.T) {
	// Arrange
	env := newTestEnv(t)
	rs := env.makeRuleSet(true)
	env.handler.SetRuleSet(rs)

	msg := &domain.Message{
		ChatID: env.sourceID, ID: 1, CanBeSaved: true,
		Content: domain.MessageContent{Type: domain.ContentText, Text: &domain.FormattedText{Text: "to delete"}},
	}
	env.telegram.PutMessage(msg)
	env.handler.OnNewMessage(context.Background(), msg)
	env.queue.ProcessAll()

	// Simulate send succeeded
	for _, target := range env.targets {
		for _, m := range env.telegram.MessagesInChat(target) {
			env.handler.OnMessageSendSucceeded(target, m.ID, m.ID)
		}
	}
	env.queue.ProcessAll()

	// Act — delete
	env.handler.OnDeletedMessages(context.Background(), env.sourceID, []domain.MessageID{1}, true)
	env.queue.ProcessAll()

	// Assert
	for _, target := range env.targets {
		msgs := env.telegram.MessagesInChat(target)
		assert.Empty(t, msgs, "target %d should have no messages after delete", target)
	}
}

func TestForwardPipeline_FilterExclude(t *testing.T) {
	// Arrange
	env := newTestEnv(t)
	rs := env.makeRuleSet(true)
	rs.ForwardRules["test_rule"].Exclude = "SPAM"
	env.handler.SetRuleSet(rs)

	msg := &domain.Message{
		ChatID: env.sourceID, ID: 1, CanBeSaved: true,
		Content: domain.MessageContent{Type: domain.ContentText, Text: &domain.FormattedText{Text: "contains SPAM word"}},
	}
	env.telegram.PutMessage(msg)

	// Act
	env.handler.OnNewMessage(context.Background(), msg)
	env.queue.ProcessAll()

	// Assert — no messages in targets (excluded)
	for _, target := range env.targets {
		msgs := env.telegram.MessagesInChat(target)
		assert.Empty(t, msgs, "target %d should have no messages (excluded)", target)
	}
}
