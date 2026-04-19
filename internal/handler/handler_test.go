package handler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/zelenin/go-tdlib/client"

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
		func(_ []int64) DedupTracker { return tracker },
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
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        make(map[string]*domain.ForwardRule),
		UniqueSources:       make(map[int64]struct{}),
		UniqueDestinations:  make(map[int64]struct{}),
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

// textMsg — helper: короткий конструктор *client.Message с MessageText.
func textMsg(chatID, msgID int64, text string) *client.Message {
	return &client.Message{
		ChatId:     chatID,
		Id:         msgID,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: text}},
	}
}

// photoMsg — helper для MessagePhoto с caption.
func photoMsg(chatID, msgID int64, caption string) *client.Message {
	return &client.Message{
		ChatId:  chatID,
		Id:      msgID,
		Content: &client.MessagePhoto{Caption: &client.FormattedText{Text: caption}},
	}
}

func TestOnNewMessage_NoRuleSet(t *testing.T) {
	t.Parallel()

	// Arrange
	h, _ := newTestHandler(t)

	// Act + Assert
	h.OnNewMessage(context.Background(), &client.Message{ChatId: 100, Id: 1})
}

func TestOnNewMessage_UnknownSource(t *testing.T) {
	t.Parallel()

	// Arrange
	h, _ := newTestHandler(t)
	rs := makeRuleSet(&domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}})
	h.SetRuleSet(rs)

	// Act + Assert
	h.OnNewMessage(context.Background(), &client.Message{ChatId: 999, Id: 1})
}

func TestOnNewMessage_SystemMessage_DeleteEnabled(t *testing.T) {
	t.Parallel()

	// Arrange
	h, d := newTestHandler(t)
	rs := makeRuleSet(&domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}})
	rs.Sources[100] = &domain.Source{ChatID: 100, DeleteSystemMessages: true}
	h.SetRuleSet(rs)

	msg := &client.Message{ChatId: 100, Id: 1, Content: &client.MessageChatJoinByLink{}}
	d.messageService.EXPECT().IsSystemMessage(msg).Return(true)
	d.telegramRepo.EXPECT().DeleteMessages(mock.MatchedBy(func(req *client.DeleteMessagesRequest) bool {
		return req.ChatId == 100 && req.Revoke && len(req.MessageIds) == 1 && req.MessageIds[0] == 1
	})).Return(&client.Ok{}, nil)

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

	msg := textMsg(100, 1, "hello")
	text := &client.FormattedText{Text: "hello"}
	d.messageService.EXPECT().IsSystemMessage(msg).Return(false)
	d.messageService.EXPECT().GetFormattedText(msg).Return(text)
	d.filterService.EXPECT().Evaluate("hello", rule).Return(domain.FiltersOK)
	d.tracker.EXPECT().TryMark(int64(200)).Return(true)
	d.rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(200))
	d.telegramRepo.EXPECT().ForwardMessages(mock.MatchedBy(func(req *client.ForwardMessagesRequest) bool {
		return req.ChatId == 200 && req.FromChatId == 100 && len(req.MessageIds) == 1 && req.MessageIds[0] == 1
	})).Return(&client.Messages{}, nil)
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

	msg := textMsg(100, 1, "hello")
	text := &client.FormattedText{Text: "hello"}
	transformed := &client.FormattedText{Text: "transformed"}
	inputContent := &client.InputMessageText{Text: transformed}

	d.messageService.EXPECT().IsSystemMessage(msg).Return(false)
	d.messageService.EXPECT().GetFormattedText(msg).Return(text)
	d.messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte(nil))
	d.messageService.EXPECT().BuildInputContent(msg, transformed).Return(inputContent)
	d.filterService.EXPECT().Evaluate("hello", rule).Return(domain.FiltersOK)
	d.tracker.EXPECT().TryMark(int64(200)).Return(true)
	d.rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(200))
	d.transformService.EXPECT().Transform(mock.Anything, mock.AnythingOfType("domain.TransformParams")).Return(transformed, nil)
	d.telegramRepo.EXPECT().SendMessage(mock.MatchedBy(func(req *client.SendMessageRequest) bool {
		return req.ChatId == 200
	})).Return(&client.Message{Id: 500}, nil)
	d.stateRepo.EXPECT().SetCopiedMessageID(int64(100), int64(1), "r1:200:500").Return(nil)
	// Stats
	d.stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.AnythingOfType("string")).Return(uint64(1), nil)
	d.stateRepo.EXPECT().IncrementForwardedMessages(int64(200), mock.AnythingOfType("string")).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	d.taskQueue.drain()

	// suppress unused import
	_ = domain.TransformParams{}
}

func TestOnNewMessage_FiltersCheck(t *testing.T) {
	t.Parallel()

	// Arrange
	h, d := newTestHandler(t)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true, Check: 300}
	rs := makeRuleSet(rule)
	rs.Sources[100] = &domain.Source{ChatID: 100}
	h.SetRuleSet(rs)

	msg := textMsg(100, 1, "suspicious")
	text := &client.FormattedText{Text: "suspicious"}
	d.messageService.EXPECT().IsSystemMessage(msg).Return(false)
	d.messageService.EXPECT().GetFormattedText(msg).Return(text)
	d.filterService.EXPECT().Evaluate("suspicious", rule).Return(domain.FiltersCheck)
	d.rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(300))
	d.telegramRepo.EXPECT().ForwardMessages(mock.MatchedBy(func(req *client.ForwardMessagesRequest) bool {
		return req.ChatId == 300 && req.FromChatId == 100
	})).Return(&client.Messages{}, nil)
	// Stats (viewed only)
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

	msg := textMsg(100, 1, "hello")
	msg.CanBeSaved = false
	text := &client.FormattedText{Text: "hello"}
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
	d.telegramRepo.EXPECT().DeleteMessages(mock.MatchedBy(func(req *client.DeleteMessagesRequest) bool {
		return req.ChatId == 200 && len(req.MessageIds) == 1 && req.MessageIds[0] == 600
	})).Return(&client.Ok{}, nil)
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
			return 0
		}
		return 600
	}).Times(2)
	d.stateRepo.EXPECT().DeleteCopiedMessageIDs(int64(100), int64(1)).Return(nil)
	d.telegramRepo.EXPECT().DeleteMessages(mock.MatchedBy(func(req *client.DeleteMessagesRequest) bool {
		return req.ChatId == 200 && req.MessageIds[0] == 600
	})).Return(&client.Ok{}, nil)
	d.stateRepo.EXPECT().DeleteNewMessageID(int64(200), int64(500)).Return(nil)
	d.stateRepo.EXPECT().DeleteTmpMessageID(int64(200), int64(600)).Return(nil)
	d.stateRepo.EXPECT().DeleteAnswerMessageID(int64(200), int64(500)).Return(nil)

	// Act
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

	msg := textMsg(100, 1, "unrelated")
	text := &client.FormattedText{Text: "unrelated"}
	d.messageService.EXPECT().IsSystemMessage(msg).Return(false)
	d.messageService.EXPECT().GetFormattedText(msg).Return(text)
	d.filterService.EXPECT().Evaluate("unrelated", rule).Return(domain.FiltersOther)
	d.rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(400))
	d.telegramRepo.EXPECT().ForwardMessages(mock.MatchedBy(func(req *client.ForwardMessagesRequest) bool {
		return req.ChatId == 400
	})).Return(&client.Messages{}, nil)
	d.stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.AnythingOfType("string")).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	d.taskQueue.drain()
}

func TestOnEditedMessage_NoRuleSet(t *testing.T) {
	t.Parallel()

	// Arrange
	h, _ := newTestHandler(t)

	// Act
	h.OnEditedMessage(context.Background(), &client.Message{ChatId: 100, Id: 1})
}

func TestOnEditedMessage_UnknownSource(t *testing.T) {
	t.Parallel()

	// Arrange
	h, _ := newTestHandler(t)
	rs := makeRuleSet(&domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}})
	h.SetRuleSet(rs)

	// Act
	h.OnEditedMessage(context.Background(), &client.Message{ChatId: 999, Id: 1})
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

	msg := textMsg(100, 1, "edited text")
	text := &client.FormattedText{Text: "edited text"}
	transformed := &client.FormattedText{Text: "transformed edit"}

	d.stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"r1:200:500"})
	d.stateRepo.EXPECT().GetNewMessageID(int64(200), int64(500)).Return(int64(600))
	d.messageService.EXPECT().GetFormattedText(msg).Return(text)
	d.messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte(nil))
	d.transformService.EXPECT().Transform(mock.Anything, mock.AnythingOfType("domain.TransformParams")).Return(transformed, nil)
	d.telegramRepo.EXPECT().EditMessageText(mock.MatchedBy(func(req *client.EditMessageTextRequest) bool {
		if req.ChatId != 200 || req.MessageId != 600 {
			return false
		}
		t, ok := req.InputMessageContent.(*client.InputMessageText)
		return ok && t.Text == transformed
	})).Return(&client.Message{Id: 600}, nil)
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

	msg := photoMsg(100, 1, "new caption")
	text := &client.FormattedText{Text: "new caption"}
	transformed := &client.FormattedText{Text: "transformed caption"}

	d.stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"r1:200:500"})
	d.stateRepo.EXPECT().GetNewMessageID(int64(200), int64(500)).Return(int64(600))
	d.messageService.EXPECT().GetFormattedText(msg).Return(text)
	d.messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte(nil))
	d.transformService.EXPECT().Transform(mock.Anything, mock.AnythingOfType("domain.TransformParams")).Return(transformed, nil)
	d.telegramRepo.EXPECT().EditMessageCaption(mock.MatchedBy(func(req *client.EditMessageCaptionRequest) bool {
		return req.ChatId == 200 && req.MessageId == 600 && req.Caption == transformed
	})).Return(&client.Message{Id: 600}, nil)
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

	msg := textMsg(100, 1, "v2")
	text := &client.FormattedText{Text: "v2"}
	transformed := &client.FormattedText{Text: "transformed v2"}
	inputContent := &client.InputMessageText{Text: transformed}

	d.stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"r1:200:500"})
	d.stateRepo.EXPECT().GetNewMessageID(int64(200), int64(500)).Return(int64(600))
	d.messageService.EXPECT().GetFormattedText(msg).Return(text)
	d.messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte(nil))
	d.transformService.EXPECT().Transform(mock.Anything, mock.AnythingOfType("domain.TransformParams")).Return(transformed, nil)
	d.messageService.EXPECT().BuildInputContent(msg, transformed).Return(inputContent)
	d.rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(200))
	d.telegramRepo.EXPECT().SendMessage(mock.MatchedBy(func(req *client.SendMessageRequest) bool {
		return req.ChatId == 200
	})).Return(&client.Message{Id: 700}, nil)
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

	msg := textMsg(100, 1, "edit")
	text := &client.FormattedText{Text: "edit"}
	transformed := &client.FormattedText{Text: "transformed"}

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
	d.telegramRepo.EXPECT().EditMessageText(mock.Anything).Return(&client.Message{Id: 600}, nil)
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

	msg := textMsg(100, 1, "with button")
	text := &client.FormattedText{Text: "with button"}
	transformed := &client.FormattedText{Text: "transformed"}

	d.stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"r1:200:500"})
	d.stateRepo.EXPECT().GetNewMessageID(int64(200), int64(500)).Return(int64(600))
	d.messageService.EXPECT().GetFormattedText(msg).Return(text)
	d.messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte("action"))
	d.transformService.EXPECT().Transform(mock.Anything, mock.AnythingOfType("domain.TransformParams")).Return(transformed, nil)
	d.telegramRepo.EXPECT().EditMessageText(mock.Anything).Return(&client.Message{Id: 600}, nil)
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
