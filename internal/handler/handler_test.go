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
	for len(q.tasks) > 0 {
		fn := q.tasks[0]
		q.tasks = q.tasks[1:]
		fn()
	}
}

func newTestHandler(t *testing.T) (*Handler, *testDeps) {
	t.Helper()
	d := &testDeps{
		telegramRepo:     mocks.NewTelegramRepo(t),
		stateRepo:        mocks.NewStateRepo(t),
		messageService:   mocks.NewMessageService(t),
		filterService:    mocks.NewFilterService(t),
		transformService: mocks.NewTransformService(t),
		albumService:     mocks.NewAlbumService(t),
		rateLimiter:      mocks.NewRateLimiter(t),
		taskQueue:        &syncQueue{},
	}
	tracker := mocks.NewDedupTracker(t)

	h := New(
		d.telegramRepo,
		d.stateRepo,
		d.messageService,
		d.filterService,
		d.transformService,
		d.albumService,
		d.taskQueue,
		d.rateLimiter,
		func(_ []domain.ChatID) DedupTracker { return tracker },
	)
	d.tracker = tracker
	return h, d
}

type testDeps struct {
	telegramRepo     *mocks.TelegramRepo
	stateRepo        *mocks.StateRepo
	messageService   *mocks.MessageService
	filterService    *mocks.FilterService
	transformService *mocks.TransformService
	albumService     *mocks.AlbumService
	rateLimiter      *mocks.RateLimiter
	taskQueue        *syncQueue
	tracker          *mocks.DedupTracker
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
	d.messageService.EXPECT().IsSystemMessage(msg).Return(true)
	d.telegramRepo.EXPECT().DeleteMessages(mock.Anything, int64(100), []int64{int64(1)}, true).Return(nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	d.taskQueue.drain()
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
	d.messageService.EXPECT().IsSystemMessage(msg).Return(false)
	d.messageService.EXPECT().GetFormattedText(msg).Return(text)
	d.filterService.EXPECT().Evaluate("hello", rule).Return(domain.FiltersOK)
	d.tracker.EXPECT().TryMark(int64(200)).Return(true)
	d.rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(200))
	d.telegramRepo.EXPECT().ForwardMessages(mock.Anything, int64(100), int64(200), []int64{int64(1)}).Return([]int64{int64(300)}, nil)
	// Stats
	d.stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.AnythingOfType("string")).Return(uint64(1), nil)
	d.stateRepo.EXPECT().IncrementForwardedMessages(int64(200), mock.AnythingOfType("string")).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	d.taskQueue.drain()
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
	inputContent := domain.InputMessageContent{Type: domain.ContentText, Text: transformed, DisableLinkPreview: true}

	d.messageService.EXPECT().IsSystemMessage(msg).Return(false)
	d.messageService.EXPECT().GetFormattedText(msg).Return(text)
	d.messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte(nil))
	d.messageService.EXPECT().BuildInputContent(msg, transformed).Return(inputContent)
	d.filterService.EXPECT().Evaluate("hello", rule).Return(domain.FiltersOK)
	d.tracker.EXPECT().TryMark(int64(200)).Return(true)
	d.rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(200))
	d.transformService.EXPECT().Transform(mock.Anything, mock.AnythingOfType("domain.TransformParams")).Return(transformed, nil)
	d.telegramRepo.EXPECT().SendMessage(mock.Anything, int64(200), mock.AnythingOfType("domain.InputMessageContent")).Return(int64(500), nil)
	d.stateRepo.EXPECT().SetCopiedMessageID(int64(100), int64(1), "r1:200:500").Return(nil)
	// Stats
	d.stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.AnythingOfType("string")).Return(uint64(1), nil)
	d.stateRepo.EXPECT().IncrementForwardedMessages(int64(200), mock.AnythingOfType("string")).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	d.taskQueue.drain()
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
	d.messageService.EXPECT().IsSystemMessage(msg).Return(false)
	d.messageService.EXPECT().GetFormattedText(msg).Return(text)
	d.filterService.EXPECT().Evaluate("suspicious", rule).Return(domain.FiltersCheck)
	d.rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(300))
	d.telegramRepo.EXPECT().ForwardMessages(mock.Anything, int64(100), int64(300), []int64{int64(1)}).Return([]int64{int64(400)}, nil)
	// Stats (viewed only, not forwarded for FiltersCheck)
	d.stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.AnythingOfType("string")).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	d.taskQueue.drain()
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
	d.messageService.EXPECT().IsSystemMessage(msg).Return(false)
	d.messageService.EXPECT().GetFormattedText(msg).Return(text)

	// Act
	h.OnNewMessage(context.Background(), msg)
}

func TestOnDeletedMessages_PermanentWithCopies(t *testing.T) {
	t.Parallel()

	// Arrange
	h, d := newTestHandler(t)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}}
	rs := makeRuleSet(rule)
	h.SetRuleSet(rs)

	d.stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"r1:200:500"})
	d.stateRepo.EXPECT().GetNewMessageID(int64(200), int64(500)).Return(int64(600))
	d.telegramRepo.EXPECT().DeleteMessages(mock.Anything, int64(200), []int64{int64(600)}, true).Return(nil)
	d.stateRepo.EXPECT().DeleteNewMessageID(int64(200), int64(500)).Return(nil)
	d.stateRepo.EXPECT().DeleteTmpMessageID(int64(200), int64(600)).Return(nil)
	d.stateRepo.EXPECT().DeleteAnswerMessageID(int64(200), int64(500)).Return(nil)
	d.stateRepo.EXPECT().DeleteCopiedMessageIDs(int64(100), int64(1)).Return(nil)

	// Act
	h.OnDeletedMessages(context.Background(), 100, []int64{1}, true)
	d.taskQueue.drain()
}

func TestOnDeletedMessages_IndelibleRule(t *testing.T) {
	t.Parallel()

	// Arrange
	h, d := newTestHandler(t)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, Indelible: true}
	rs := makeRuleSet(rule)
	h.SetRuleSet(rs)

	d.stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"r1:200:500"})
	d.stateRepo.EXPECT().DeleteCopiedMessageIDs(int64(100), int64(1)).Return(nil)

	// Act
	h.OnDeletedMessages(context.Background(), 100, []int64{1}, true)
	d.taskQueue.drain()
}

func TestOnDeletedMessages_RetryOnMissingNewID(t *testing.T) {
	t.Parallel()

	// Arrange
	h, d := newTestHandler(t)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}}
	rs := makeRuleSet(rule)
	h.SetRuleSet(rs)

	call := 0
	d.stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"r1:200:500"}).Times(2)
	d.stateRepo.EXPECT().GetNewMessageID(int64(200), int64(500)).RunAndReturn(func(_ int64, _ int64) int64 {
		call++
		if call == 1 {
			return 0 // retry
		}
		return 600 // success
	}).Times(2)
	d.stateRepo.EXPECT().DeleteCopiedMessageIDs(int64(100), int64(1)).Return(nil).Once()
	d.telegramRepo.EXPECT().DeleteMessages(mock.Anything, int64(200), []int64{int64(600)}, true).Return(nil)
	d.stateRepo.EXPECT().DeleteNewMessageID(int64(200), int64(500)).Return(nil)
	d.stateRepo.EXPECT().DeleteTmpMessageID(int64(200), int64(600)).Return(nil)
	d.stateRepo.EXPECT().DeleteAnswerMessageID(int64(200), int64(500)).Return(nil)

	// Act — drain выполняет и retry
	h.OnDeletedMessages(context.Background(), 100, []int64{1}, true)
	d.taskQueue.drain()
}

func TestOnMessageSendSucceeded(t *testing.T) {
	t.Parallel()

	// Arrange
	h, d := newTestHandler(t)
	d.stateRepo.EXPECT().SetNewMessageID(int64(200), int64(500), int64(600)).Return(nil)
	d.stateRepo.EXPECT().SetTmpMessageID(int64(200), int64(600), int64(500)).Return(nil)

	// Act
	h.OnMessageSendSucceeded(200, 500, 600)
	d.taskQueue.drain()
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
	text := &domain.FormattedText{Text: "unrelated"}
	d.messageService.EXPECT().IsSystemMessage(msg).Return(false)
	d.messageService.EXPECT().GetFormattedText(msg).Return(text)
	d.filterService.EXPECT().Evaluate("unrelated", rule).Return(domain.FiltersOther)
	d.rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(400))
	d.telegramRepo.EXPECT().ForwardMessages(mock.Anything, int64(100), int64(400), []int64{int64(1)}).Return([]int64{int64(500)}, nil)
	// Stats (viewed only, not forwarded for FiltersOther)
	d.stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.AnythingOfType("string")).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	d.taskQueue.drain()
}

func TestOnEditedMessage_NoRuleSet(t *testing.T) {
	t.Parallel()

	// Arrange
	h, _ := newTestHandler(t)

	// Act — не паникует при отсутствии ruleset
	h.OnEditedMessage(context.Background(), &domain.Message{ChatID: 100, ID: 1})
}

func TestOnEditedMessage_UnknownSource(t *testing.T) {
	t.Parallel()

	// Arrange
	h, _ := newTestHandler(t)
	rs := makeRuleSet(&domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}})
	h.SetRuleSet(rs)

	// Act — неизвестный source игнорируется
	h.OnEditedMessage(context.Background(), &domain.Message{ChatID: 999, ID: 1})
}

func TestOnEditedMessage_TextUpdate(t *testing.T) {
	t.Parallel()

	// Arrange
	h, d := newTestHandler(t)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := makeRuleSet(rule)
	rs.Sources[100] = &domain.Source{ChatID: 100}
	rs.Destinations[200] = &domain.Destination{ChatID: 200}
	h.SetRuleSet(rs)

	msg := &domain.Message{
		ChatID: 100, ID: 1,
		Content: domain.MessageContent{
			Type: domain.ContentText,
			Text: &domain.FormattedText{Text: "edited text"},
		},
	}
	text := &domain.FormattedText{Text: "edited text"}
	transformed := &domain.FormattedText{Text: "transformed edit"}

	d.stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"r1:200:500"})
	d.stateRepo.EXPECT().GetNewMessageID(int64(200), int64(500)).Return(int64(600))
	d.messageService.EXPECT().GetFormattedText(msg).Return(text)
	d.messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte(nil))
	d.transformService.EXPECT().Transform(mock.Anything, mock.AnythingOfType("domain.TransformParams")).Return(transformed, nil)
	d.telegramRepo.EXPECT().EditMessageText(mock.Anything, int64(200), int64(600), transformed).Return(nil)
	d.stateRepo.EXPECT().DeleteAnswerMessageID(int64(200), int64(500)).Return(nil)

	// Act
	h.OnEditedMessage(context.Background(), msg)
	d.taskQueue.drain()
}

func TestOnEditedMessage_CaptionUpdate(t *testing.T) {
	t.Parallel()

	// Arrange
	h, d := newTestHandler(t)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := makeRuleSet(rule)
	rs.Sources[100] = &domain.Source{ChatID: 100}
	rs.Destinations[200] = &domain.Destination{ChatID: 200}
	h.SetRuleSet(rs)

	msg := &domain.Message{
		ChatID: 100, ID: 1,
		Content: domain.MessageContent{
			Type: domain.ContentPhoto,
			Text: &domain.FormattedText{Text: "new caption"},
		},
	}
	text := &domain.FormattedText{Text: "new caption"}
	transformed := &domain.FormattedText{Text: "transformed caption"}

	d.stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"r1:200:500"})
	d.stateRepo.EXPECT().GetNewMessageID(int64(200), int64(500)).Return(int64(600))
	d.messageService.EXPECT().GetFormattedText(msg).Return(text)
	d.messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte(nil))
	d.transformService.EXPECT().Transform(mock.Anything, mock.AnythingOfType("domain.TransformParams")).Return(transformed, nil)
	d.telegramRepo.EXPECT().EditMessageCaption(mock.Anything, int64(200), int64(600), transformed).Return(nil)
	d.stateRepo.EXPECT().DeleteAnswerMessageID(int64(200), int64(500)).Return(nil)

	// Act
	h.OnEditedMessage(context.Background(), msg)
	d.taskQueue.drain()
}

func TestOnEditedMessage_CopyOnce_Versioning(t *testing.T) {
	t.Parallel()

	// Arrange
	h, d := newTestHandler(t)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true, CopyOnce: true}
	rs := makeRuleSet(rule)
	rs.Sources[100] = &domain.Source{ChatID: 100}
	rs.Destinations[200] = &domain.Destination{ChatID: 200}
	h.SetRuleSet(rs)

	msg := &domain.Message{
		ChatID: 100, ID: 1, CanBeSaved: true,
		Content: domain.MessageContent{
			Type: domain.ContentText,
			Text: &domain.FormattedText{Text: "v2"},
		},
	}
	text := &domain.FormattedText{Text: "v2"}
	transformed := &domain.FormattedText{Text: "transformed v2"}
	inputContent := domain.InputMessageContent{Type: domain.ContentText, Text: transformed, DisableLinkPreview: true}

	d.stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"r1:200:500"})
	d.stateRepo.EXPECT().GetNewMessageID(int64(200), int64(500)).Return(int64(600))
	d.messageService.EXPECT().GetFormattedText(msg).Return(text)
	d.messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte(nil))
	d.transformService.EXPECT().Transform(mock.Anything, mock.AnythingOfType("domain.TransformParams")).Return(transformed, nil)
	d.messageService.EXPECT().BuildInputContent(msg, transformed).Return(inputContent)
	d.rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(200))
	d.telegramRepo.EXPECT().SendMessage(mock.Anything, int64(200), mock.AnythingOfType("domain.InputMessageContent")).Return(int64(700), nil)
	d.stateRepo.EXPECT().SetCopiedMessageID(int64(100), int64(1), "r1:200:700").Return(nil)

	// Act — cancelled context останавливает goroutine runNextLinkWorkflow
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h.OnEditedMessage(ctx, msg)
	d.taskQueue.drain()
}

func TestOnEditedMessage_RetryOnMissingNewID(t *testing.T) {
	t.Parallel()

	// Arrange
	h, d := newTestHandler(t)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := makeRuleSet(rule)
	rs.Sources[100] = &domain.Source{ChatID: 100}
	rs.Destinations[200] = &domain.Destination{ChatID: 200}
	h.SetRuleSet(rs)

	msg := &domain.Message{
		ChatID: 100, ID: 1,
		Content: domain.MessageContent{
			Type: domain.ContentText,
			Text: &domain.FormattedText{Text: "edit"},
		},
	}
	text := &domain.FormattedText{Text: "edit"}
	transformed := &domain.FormattedText{Text: "transformed"}

	call := 0
	d.stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"r1:200:500"}).Times(2)
	d.stateRepo.EXPECT().GetNewMessageID(int64(200), int64(500)).RunAndReturn(func(_ int64, _ int64) int64 {
		call++
		if call == 1 {
			return 0
		}
		return 600
	}).Times(2)
	d.messageService.EXPECT().GetFormattedText(msg).Return(text).Times(2)
	d.messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte(nil))
	d.transformService.EXPECT().Transform(mock.Anything, mock.AnythingOfType("domain.TransformParams")).Return(transformed, nil)
	d.telegramRepo.EXPECT().EditMessageText(mock.Anything, int64(200), int64(600), transformed).Return(nil)
	d.stateRepo.EXPECT().DeleteAnswerMessageID(int64(200), int64(500)).Return(nil)

	// Act
	h.OnEditedMessage(context.Background(), msg)
	d.taskQueue.drain()
}

func TestOnEditedMessage_ReplyMarkupSync(t *testing.T) {
	t.Parallel()

	// Arrange
	h, d := newTestHandler(t)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := makeRuleSet(rule)
	rs.Sources[100] = &domain.Source{ChatID: 100}
	rs.Destinations[200] = &domain.Destination{ChatID: 200}
	h.SetRuleSet(rs)

	msg := &domain.Message{
		ChatID: 100, ID: 1,
		Content: domain.MessageContent{
			Type: domain.ContentText,
			Text: &domain.FormattedText{Text: "with button"},
		},
		ReplyMarkup: &domain.ReplyMarkup{CallbackData: []byte("action")},
	}
	text := &domain.FormattedText{Text: "with button"}
	transformed := &domain.FormattedText{Text: "transformed"}

	d.stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"r1:200:500"})
	d.stateRepo.EXPECT().GetNewMessageID(int64(200), int64(500)).Return(int64(600))
	d.messageService.EXPECT().GetFormattedText(msg).Return(text)
	d.messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte("action"))
	d.transformService.EXPECT().Transform(mock.Anything, mock.AnythingOfType("domain.TransformParams")).Return(transformed, nil)
	d.telegramRepo.EXPECT().EditMessageText(mock.Anything, int64(200), int64(600), transformed).Return(nil)
	d.stateRepo.EXPECT().SetAnswerMessageID(int64(200), int64(500), int64(100), int64(1)).Return(nil)

	// Act
	h.OnEditedMessage(context.Background(), msg)
	d.taskQueue.drain()
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
