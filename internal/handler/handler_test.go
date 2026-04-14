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

func newTestHandler() (*Handler, *testDeps) {
	d := &testDeps{
		telegram:  &mocks.TelegramGateway{},
		state:     &mocks.StateStore{},
		messages:  &mocks.MessageService{},
		filters:   &mocks.FilterService{},
		transform: &mocks.TransformService{},
		albums:    &mocks.AlbumService{},
		queue:     &syncQueue{},
	}
	tracker := &mocks.DedupTracker{}
	tracker.On("TryMark", mock.Anything).Return(true)

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
	h, _ := newTestHandler()

	h.OnNewMessage(context.Background(), &domain.Message{ChatID: 100, ID: 1})
	// Не паникует, ничего не делает
}

func TestOnNewMessage_UnknownSource(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()

	rs := makeRuleSet(&domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}})
	h.SetRuleSet(rs)

	h.OnNewMessage(context.Background(), &domain.Message{ChatID: 999, ID: 1})
	// Нет совпадения — ничего не делает
}

func TestOnNewMessage_SystemMessage_DeleteEnabled(t *testing.T) {
	t.Parallel()
	h, d := newTestHandler()

	rs := makeRuleSet(&domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}})
	rs.Sources[100] = &domain.Source{ChatID: 100, DeleteSystemMessages: true}
	h.SetRuleSet(rs)

	msg := &domain.Message{
		ChatID:  100,
		ID:      1,
		Content: domain.MessageContent{Type: domain.ContentSystem},
	}

	d.messages.On("IsSystemMessage", msg).Return(true)
	d.telegram.On("DeleteMessages", mock.Anything, int64(100), []int64{int64(1)}, true).Return(nil)

	h.OnNewMessage(context.Background(), msg)
	d.queue.drain()

	d.telegram.AssertCalled(t, "DeleteMessages", mock.Anything, int64(100), []int64{int64(1)}, true)
}

func TestOnNewMessage_SystemMessage_DeleteDisabled(t *testing.T) {
	t.Parallel()
	h, d := newTestHandler()

	rs := makeRuleSet(&domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}})
	rs.Sources[100] = &domain.Source{ChatID: 100, DeleteSystemMessages: false}
	h.SetRuleSet(rs)

	msg := &domain.Message{
		ChatID:  100,
		ID:      1,
		Content: domain.MessageContent{Type: domain.ContentSystem},
	}

	d.messages.On("IsSystemMessage", msg).Return(true)

	h.OnNewMessage(context.Background(), msg)

	d.telegram.AssertNotCalled(t, "DeleteMessages", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestOnNewMessage_NilFormattedText(t *testing.T) {
	t.Parallel()
	h, d := newTestHandler()

	rs := makeRuleSet(&domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}})
	rs.Sources[100] = &domain.Source{ChatID: 100}
	h.SetRuleSet(rs)

	msg := &domain.Message{ChatID: 100, ID: 1}
	d.messages.On("IsSystemMessage", msg).Return(false)
	d.messages.On("GetFormattedText", msg).Return(nil)

	h.OnNewMessage(context.Background(), msg)
	// Возвращается раньше — нет forwardMessage
}

func TestOnNewMessage_ForwardWithoutCopy(t *testing.T) {
	t.Parallel()
	h, d := newTestHandler()

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: false}
	rs := makeRuleSet(rule)
	rs.Sources[100] = &domain.Source{ChatID: 100}
	h.SetRuleSet(rs)

	msg := &domain.Message{ChatID: 100, ID: 1, CanBeSaved: true}
	text := &domain.FormattedText{Text: "hello"}

	d.messages.On("IsSystemMessage", msg).Return(false)
	d.messages.On("GetFormattedText", msg).Return(text)
	d.filters.On("Evaluate", "hello", rule).Return(domain.FiltersOK)
	d.telegram.On("ForwardMessages", mock.Anything, int64(100), int64(200), []int64{int64(1)}).Return([]int64{int64(300)}, nil)

	h.OnNewMessage(context.Background(), msg)
	d.queue.drain()

	d.telegram.AssertCalled(t, "ForwardMessages", mock.Anything, int64(100), int64(200), []int64{int64(1)})
}

func TestOnNewMessage_SendCopy(t *testing.T) {
	t.Parallel()
	h, d := newTestHandler()

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

	d.messages.On("IsSystemMessage", msg).Return(false)
	d.messages.On("GetFormattedText", msg).Return(text)
	d.messages.On("GetReplyMarkupData", msg).Return([]byte(nil))
	d.messages.On("BuildInputContent", msg, transformed).Return(inputContent)
	d.filters.On("Evaluate", "hello", rule).Return(domain.FiltersOK)
	d.transform.On("Transform", mock.Anything, mock.AnythingOfType("domain.TransformParams")).Return(transformed, nil)
	d.telegram.On("SendMessage", mock.Anything, int64(200), inputContent).Return(int64(500), nil)
	d.state.On("SetCopiedMessageID", int64(100), int64(1), "r1:200:500").Return(nil)

	h.OnNewMessage(context.Background(), msg)
	d.queue.drain()

	d.telegram.AssertCalled(t, "SendMessage", mock.Anything, int64(200), inputContent)
	d.state.AssertCalled(t, "SetCopiedMessageID", int64(100), int64(1), "r1:200:500")
}

func TestOnNewMessage_FiltersCheck(t *testing.T) {
	t.Parallel()
	h, d := newTestHandler()

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true, Check: 300}
	rs := makeRuleSet(rule)
	rs.Sources[100] = &domain.Source{ChatID: 100}
	h.SetRuleSet(rs)

	msg := &domain.Message{ChatID: 100, ID: 1, CanBeSaved: true}
	text := &domain.FormattedText{Text: "suspicious"}

	d.messages.On("IsSystemMessage", msg).Return(false)
	d.messages.On("GetFormattedText", msg).Return(text)
	d.filters.On("Evaluate", "suspicious", rule).Return(domain.FiltersCheck)
	d.telegram.On("ForwardMessages", mock.Anything, int64(100), int64(300), []int64{int64(1)}).Return([]int64{int64(400)}, nil)

	h.OnNewMessage(context.Background(), msg)
	d.queue.drain()

	d.telegram.AssertCalled(t, "ForwardMessages", mock.Anything, int64(100), int64(300), []int64{int64(1)})
}

func TestOnNewMessage_FiltersOther(t *testing.T) {
	t.Parallel()
	h, d := newTestHandler()

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true, Other: 400}
	rs := makeRuleSet(rule)
	rs.Sources[100] = &domain.Source{ChatID: 100}
	h.SetRuleSet(rs)

	msg := &domain.Message{ChatID: 100, ID: 1, CanBeSaved: true}
	text := &domain.FormattedText{Text: "filtered out"}

	d.messages.On("IsSystemMessage", msg).Return(false)
	d.messages.On("GetFormattedText", msg).Return(text)
	d.filters.On("Evaluate", "filtered out", rule).Return(domain.FiltersOther)
	d.telegram.On("ForwardMessages", mock.Anything, int64(100), int64(400), []int64{int64(1)}).Return([]int64{int64(500)}, nil)

	h.OnNewMessage(context.Background(), msg)
	d.queue.drain()

	d.telegram.AssertCalled(t, "ForwardMessages", mock.Anything, int64(100), int64(400), []int64{int64(1)})
}

func TestOnNewMessage_CannotBeSaved_WithoutSendCopy(t *testing.T) {
	t.Parallel()
	h, d := newTestHandler()

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: false}
	rs := makeRuleSet(rule)
	rs.Sources[100] = &domain.Source{ChatID: 100}
	h.SetRuleSet(rs)

	msg := &domain.Message{ChatID: 100, ID: 1, CanBeSaved: false}
	text := &domain.FormattedText{Text: "hello"}

	d.messages.On("IsSystemMessage", msg).Return(false)
	d.messages.On("GetFormattedText", msg).Return(text)

	h.OnNewMessage(context.Background(), msg)

	// Правило пропущено, т.к. SendCopy=false и CanBeSaved=false
	d.telegram.AssertNotCalled(t, "ForwardMessages", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestOnEditedMessage_NoRuleSet(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()

	h.OnEditedMessage(context.Background(), &domain.Message{ChatID: 100, ID: 1})
}

func TestOnEditedMessage_UnknownSource(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()

	rs := makeRuleSet(&domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}})
	h.SetRuleSet(rs)

	h.OnEditedMessage(context.Background(), &domain.Message{ChatID: 999, ID: 1})
}

func TestOnDeletedMessages_NonPermanent(t *testing.T) {
	t.Parallel()
	h, d := newTestHandler()

	rs := makeRuleSet(&domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}})
	h.SetRuleSet(rs)

	h.OnDeletedMessages(context.Background(), 100, []int64{1}, false)

	// isPermanent=false — ничего не удаляем
	d.state.AssertNotCalled(t, "GetCopiedMessageIDs", mock.Anything, mock.Anything)
}

func TestOnDeletedMessages_PermanentWithCopies(t *testing.T) {
	t.Parallel()
	h, d := newTestHandler()

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}}
	rs := makeRuleSet(rule)
	h.SetRuleSet(rs)

	d.state.On("GetCopiedMessageIDs", int64(100), int64(1)).Return([]string{"r1:200:500"})
	d.state.On("GetNewMessageID", int64(200), int64(500)).Return(int64(600))
	d.telegram.On("DeleteMessages", mock.Anything, int64(200), []int64{int64(600)}, true).Return(nil)
	d.state.On("DeleteNewMessageID", int64(200), int64(500)).Return(nil)
	d.state.On("DeleteTmpMessageID", int64(200), int64(600)).Return(nil)
	d.state.On("DeleteAnswerMessageID", int64(200), int64(500)).Return(nil)
	d.state.On("DeleteCopiedMessageIDs", int64(100), int64(1)).Return(nil)

	h.OnDeletedMessages(context.Background(), 100, []int64{1}, true)
	d.queue.drain()

	d.telegram.AssertCalled(t, "DeleteMessages", mock.Anything, int64(200), []int64{int64(600)}, true)
	d.state.AssertCalled(t, "DeleteCopiedMessageIDs", int64(100), int64(1))
}

func TestOnDeletedMessages_IndelibleRule(t *testing.T) {
	t.Parallel()
	h, d := newTestHandler()

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, Indelible: true}
	rs := makeRuleSet(rule)
	h.SetRuleSet(rs)

	d.state.On("GetCopiedMessageIDs", int64(100), int64(1)).Return([]string{"r1:200:500"})
	d.state.On("DeleteCopiedMessageIDs", int64(100), int64(1)).Return(nil)

	h.OnDeletedMessages(context.Background(), 100, []int64{1}, true)
	d.queue.drain()

	// Indelible — не удаляем копию
	d.telegram.AssertNotCalled(t, "DeleteMessages", mock.Anything, int64(200), mock.Anything, mock.Anything)
}

func TestOnMessageSendSucceeded(t *testing.T) {
	t.Parallel()
	h, d := newTestHandler()

	d.state.On("SetNewMessageID", int64(200), int64(500), int64(600)).Return(nil)
	d.state.On("SetTmpMessageID", int64(200), int64(600), int64(500)).Return(nil)

	h.OnMessageSendSucceeded(200, 500, 600)
	d.queue.drain()

	d.state.AssertCalled(t, "SetNewMessageID", int64(200), int64(500), int64(600))
	d.state.AssertCalled(t, "SetTmpMessageID", int64(200), int64(600), int64(500))
}

func TestSetRuleSet(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()

	rs := makeRuleSet(&domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}})
	h.SetRuleSet(rs)

	loaded := h.ruleset.Load()
	assert.NotNil(t, loaded)
	assert.Contains(t, loaded.ForwardRules, "r1")
}
