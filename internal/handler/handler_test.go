package handler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/handler/mocks"
)

// syncQueue выполняет задачи синхронно для тестов.
type syncQueue struct {
	tasks []func()
}

func (q *syncQueue) Add(fn func()) {
	q.tasks = append(q.tasks, fn)
}

func (q *syncQueue) drain() {
	for _, fn := range q.tasks {
		fn()
	}
	q.tasks = nil
}

func newTestHandler(t *testing.T) (*Handler, *testDeps) {
	t.Helper()
	d := &testDeps{
		telegram:  mocks.NewTelegramGateway(t),
		state:     mocks.NewStateStore(t),
		messages:  mocks.NewMessageService(t),
		filters:   mocks.NewFilterService(t),
		transform: mocks.NewTransformService(t),
		albums:    mocks.NewAlbumService(t),
		queue:     &syncQueue{},
	}
	tracker := mocks.NewDedupTracker(t)

	h := New(
		d.telegram,
		d.state,
		d.messages,
		d.filters,
		d.transform,
		d.albums,
		d.queue,
		func(_ []domain.ChatID) DedupTracker { return tracker },
	)
	d.tracker = tracker
	return h, d
}

type testDeps struct {
	telegram  *mocks.TelegramGateway
	state     *mocks.StateStore
	messages  *mocks.MessageService
	filters   *mocks.FilterService
	transform *mocks.TransformService
	albums    *mocks.AlbumService
	queue     *syncQueue
	tracker   *mocks.DedupTracker
}

func makeRuleSet(rules ...*domain.ForwardRule) *domain.RuleSet {
	rs := &domain.RuleSet{
		Sources:             make(map[domain.ChatID]*domain.Source),
		Destinations:        make(map[domain.ChatID]*domain.Destination),
		ForwardRules:        make(map[string]*domain.ForwardRule),
		UniqueSources:       make(map[domain.ChatID]struct{}),
		UniqueDestinations:  make(map[domain.ChatID]struct{}),
		OrderedForwardRules: nil,
	}
	for _, rule := range rules {
		rs.ForwardRules[rule.ID] = rule
		rs.OrderedForwardRules = append(rs.OrderedForwardRules, rule.ID)
		rs.UniqueSources[rule.From] = struct{}{}
		for _, to := range rule.To {
			rs.UniqueDestinations[to] = struct{}{}
		}
	}
	return rs
}

func TestOnNewMessage_NoRuleSet(t *testing.T) {
	t.Parallel()

	// Arrange
	h, _ := newTestHandler(t)

	// Act + Assert
	h.OnNewMessage(context.Background(), &domain.Message{ChatID: 100, ID: 1})
}

func TestOnNewMessage_UnknownSource(t *testing.T) {
	t.Parallel()

	// Arrange
	h, _ := newTestHandler(t)
	rs := makeRuleSet(&domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}})
	h.SetRuleSet(rs)

	// Act + Assert
	h.OnNewMessage(context.Background(), &domain.Message{ChatID: 999, ID: 1})
}

func TestOnNewMessage_SystemMessage_DeleteEnabled(t *testing.T) {
	t.Parallel()

	// Arrange
	h, d := newTestHandler(t)
	rs := makeRuleSet(&domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}})
	rs.Sources[100] = &domain.Source{ChatID: 100, DeleteSystemMessages: true}
	h.SetRuleSet(rs)

	msg := &domain.Message{
		ChatID:  100,
		ID:      1,
		Content: domain.MessageContent{Type: domain.ContentSystem},
	}
	d.messages.EXPECT().IsSystemMessage(msg).Return(true)
	d.telegram.EXPECT().DeleteMessages(mock.Anything, int64(100), []int64{int64(1)}, true).Return(nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	d.queue.drain()
}

func TestOnNewMessage_SystemMessage_DeleteDisabled(t *testing.T) {
	t.Parallel()

	// Arrange
	h, d := newTestHandler(t)
	rs := makeRuleSet(&domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}})
	rs.Sources[100] = &domain.Source{ChatID: 100, DeleteSystemMessages: false}
	h.SetRuleSet(rs)

	msg := &domain.Message{
		ChatID:  100,
		ID:      1,
		Content: domain.MessageContent{Type: domain.ContentSystem},
	}
	d.messages.EXPECT().IsSystemMessage(msg).Return(true)

	// Act
	h.OnNewMessage(context.Background(), msg)
}

func TestOnNewMessage_NilFormattedText(t *testing.T) {
	t.Parallel()

	// Arrange
	h, d := newTestHandler(t)
	rs := makeRuleSet(&domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}})
	rs.Sources[100] = &domain.Source{ChatID: 100}
	h.SetRuleSet(rs)

	msg := &domain.Message{ChatID: 100, ID: 1}
	d.messages.EXPECT().IsSystemMessage(msg).Return(false)
	d.messages.EXPECT().GetFormattedText(msg).Return(nil)

	// Act + Assert
	h.OnNewMessage(context.Background(), msg)
}

func TestOnNewMessage_ForwardWithoutCopy(t *testing.T) {
	t.Parallel()

	// Arrange
	h, d := newTestHandler(t)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: false}
	rs := makeRuleSet(rule)
	rs.Sources[100] = &domain.Source{ChatID: 100}
	h.SetRuleSet(rs)

	msg := &domain.Message{ChatID: 100, ID: 1, CanBeSaved: true}
	text := &domain.FormattedText{Text: "hello"}
	d.messages.EXPECT().IsSystemMessage(msg).Return(false)
	d.messages.EXPECT().GetFormattedText(msg).Return(text)
	d.filters.EXPECT().Evaluate("hello", rule).Return(domain.FiltersOK)
	d.tracker.EXPECT().TryMark(int64(200)).Return(true)
	d.telegram.EXPECT().ForwardMessages(mock.Anything, int64(100), int64(200), []int64{int64(1)}).Return([]int64{int64(300)}, nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	d.queue.drain()
}

func TestOnNewMessage_SendCopy(t *testing.T) {
	t.Parallel()

	// Arrange
	h, d := newTestHandler(t)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := makeRuleSet(rule)
	rs.Sources[100] = &domain.Source{ChatID: 100}
	rs.Destinations[200] = &domain.Destination{ChatID: 200}
	h.SetRuleSet(rs)

	msg := &domain.Message{
		ChatID:     100,
		ID:         1,
		CanBeSaved: true,
		Content: domain.MessageContent{
			Type: domain.ContentText,
			Text: &domain.FormattedText{Text: "hello"},
		},
	}
	text := &domain.FormattedText{Text: "hello"}
	transformed := &domain.FormattedText{Text: "transformed"}
	inputContent := domain.InputMessageContent{Type: domain.ContentText, Text: transformed}

	d.messages.EXPECT().IsSystemMessage(msg).Return(false)
	d.messages.EXPECT().GetFormattedText(msg).Return(text)
	d.messages.EXPECT().GetReplyMarkupData(msg).Return([]byte(nil))
	d.messages.EXPECT().BuildInputContent(msg, transformed).Return(inputContent)
	d.filters.EXPECT().Evaluate("hello", rule).Return(domain.FiltersOK)
	d.tracker.EXPECT().TryMark(int64(200)).Return(true)
	d.transform.EXPECT().Transform(mock.Anything, mock.AnythingOfType("domain.TransformParams")).Return(transformed, nil)
	d.telegram.EXPECT().SendMessage(mock.Anything, int64(200), inputContent).Return(int64(500), nil)
	d.state.EXPECT().SetCopiedMessageID(int64(100), int64(1), "r1:200:500").Return(nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	d.queue.drain()
}

func TestOnNewMessage_FiltersCheck(t *testing.T) {
	t.Parallel()

	// Arrange
	h, d := newTestHandler(t)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true, Check: 300}
	rs := makeRuleSet(rule)
	rs.Sources[100] = &domain.Source{ChatID: 100}
	h.SetRuleSet(rs)

	msg := &domain.Message{ChatID: 100, ID: 1, CanBeSaved: true}
	text := &domain.FormattedText{Text: "suspicious"}
	d.messages.EXPECT().IsSystemMessage(msg).Return(false)
	d.messages.EXPECT().GetFormattedText(msg).Return(text)
	d.filters.EXPECT().Evaluate("suspicious", rule).Return(domain.FiltersCheck)
	d.telegram.EXPECT().ForwardMessages(mock.Anything, int64(100), int64(300), []int64{int64(1)}).Return([]int64{int64(400)}, nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	d.queue.drain()
}

func TestOnNewMessage_FiltersOther(t *testing.T) {
	t.Parallel()

	// Arrange
	h, d := newTestHandler(t)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true, Other: 400}
	rs := makeRuleSet(rule)
	rs.Sources[100] = &domain.Source{ChatID: 100}
	h.SetRuleSet(rs)

	msg := &domain.Message{ChatID: 100, ID: 1, CanBeSaved: true}
	text := &domain.FormattedText{Text: "filtered out"}
	d.messages.EXPECT().IsSystemMessage(msg).Return(false)
	d.messages.EXPECT().GetFormattedText(msg).Return(text)
	d.filters.EXPECT().Evaluate("filtered out", rule).Return(domain.FiltersOther)
	d.telegram.EXPECT().ForwardMessages(mock.Anything, int64(100), int64(400), []int64{int64(1)}).Return([]int64{int64(500)}, nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	d.queue.drain()
}

func TestOnNewMessage_CannotBeSaved_WithoutSendCopy(t *testing.T) {
	t.Parallel()

	// Arrange
	h, d := newTestHandler(t)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: false}
	rs := makeRuleSet(rule)
	rs.Sources[100] = &domain.Source{ChatID: 100}
	h.SetRuleSet(rs)

	msg := &domain.Message{ChatID: 100, ID: 1, CanBeSaved: false}
	text := &domain.FormattedText{Text: "hello"}
	d.messages.EXPECT().IsSystemMessage(msg).Return(false)
	d.messages.EXPECT().GetFormattedText(msg).Return(text)

	// Act
	h.OnNewMessage(context.Background(), msg)
}

func TestOnEditedMessage_NoRuleSet(t *testing.T) {
	t.Parallel()

	// Arrange
	h, _ := newTestHandler(t)

	// Act + Assert
	h.OnEditedMessage(context.Background(), &domain.Message{ChatID: 100, ID: 1})
}

func TestOnEditedMessage_UnknownSource(t *testing.T) {
	t.Parallel()

	// Arrange
	h, _ := newTestHandler(t)
	rs := makeRuleSet(&domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}})
	h.SetRuleSet(rs)

	// Act + Assert
	h.OnEditedMessage(context.Background(), &domain.Message{ChatID: 999, ID: 1})
}

func TestOnDeletedMessages_NonPermanent(t *testing.T) {
	t.Parallel()

	// Arrange
	h, _ := newTestHandler(t)
	rs := makeRuleSet(&domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}})
	h.SetRuleSet(rs)

	// Act
	h.OnDeletedMessages(context.Background(), 100, []int64{1}, false)
}

func TestOnDeletedMessages_PermanentWithCopies(t *testing.T) {
	t.Parallel()

	// Arrange
	h, d := newTestHandler(t)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}}
	rs := makeRuleSet(rule)
	h.SetRuleSet(rs)

	d.state.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"r1:200:500"})
	d.state.EXPECT().GetNewMessageID(int64(200), int64(500)).Return(int64(600))
	d.telegram.EXPECT().DeleteMessages(mock.Anything, int64(200), []int64{int64(600)}, true).Return(nil)
	d.state.EXPECT().DeleteNewMessageID(int64(200), int64(500)).Return(nil)
	d.state.EXPECT().DeleteTmpMessageID(int64(200), int64(600)).Return(nil)
	d.state.EXPECT().DeleteAnswerMessageID(int64(200), int64(500)).Return(nil)
	d.state.EXPECT().DeleteCopiedMessageIDs(int64(100), int64(1)).Return(nil)

	// Act
	h.OnDeletedMessages(context.Background(), 100, []int64{1}, true)
	d.queue.drain()
}

func TestOnDeletedMessages_IndelibleRule(t *testing.T) {
	t.Parallel()

	// Arrange
	h, d := newTestHandler(t)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, Indelible: true}
	rs := makeRuleSet(rule)
	h.SetRuleSet(rs)

	d.state.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"r1:200:500"})
	d.state.EXPECT().DeleteCopiedMessageIDs(int64(100), int64(1)).Return(nil)

	// Act
	h.OnDeletedMessages(context.Background(), 100, []int64{1}, true)
	d.queue.drain()
}

func TestOnMessageSendSucceeded(t *testing.T) {
	t.Parallel()

	// Arrange
	h, d := newTestHandler(t)
	d.state.EXPECT().SetNewMessageID(int64(200), int64(500), int64(600)).Return(nil)
	d.state.EXPECT().SetTmpMessageID(int64(200), int64(600), int64(500)).Return(nil)

	// Act
	h.OnMessageSendSucceeded(200, 500, 600)
	d.queue.drain()
}

func TestSetRuleSet(t *testing.T) {
	t.Parallel()

	// Arrange
	h, _ := newTestHandler(t)
	rs := makeRuleSet(&domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}})

	// Act
	h.SetRuleSet(rs)

	// Assert
	loaded := h.ruleset.Load()
	assert.NotNil(t, loaded)
	assert.Contains(t, loaded.ForwardRules, "r1")
}
