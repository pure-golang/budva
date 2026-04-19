package handler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/handler/mocks"
	"github.com/pure-golang/budva-claude/internal/repo/queue"
)

func TestOnNewMessage_NoRuleSet(t *testing.T) {
	t.Parallel()

	// Arrange
	taskQueue := queue.New()
	h := New(
		mocks.NewTelegramRepo(t),
		mocks.NewStateRepo(t),
		mocks.NewMessageService(t),
		mocks.NewFilterService(t),
		mocks.NewTransformService(t),
		mocks.NewAlbumService(t),
		taskQueue,
		mocks.NewRateLimiter(t),
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) },
	)

	// Act + Assert
	h.OnNewMessage(context.Background(), &client.Message{ChatId: 100, Id: 1})
	taskQueue.ProcessAll()
}

func TestOnNewMessage_UnknownSource(t *testing.T) {
	t.Parallel()

	// Arrange
	taskQueue := queue.New()
	h := New(
		mocks.NewTelegramRepo(t),
		mocks.NewStateRepo(t),
		mocks.NewMessageService(t),
		mocks.NewFilterService(t),
		mocks.NewTransformService(t),
		mocks.NewAlbumService(t),
		taskQueue,
		mocks.NewRateLimiter(t),
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) },
	)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{"r1": rule},
		UniqueSources:       map[int64]struct{}{100: {}},
		UniqueDestinations:  map[int64]struct{}{200: {}},
		OrderedForwardRules: []string{"r1"},
	}
	h.SetRuleSet(rs)

	// Act + Assert
	h.OnNewMessage(context.Background(), &client.Message{ChatId: 999, Id: 1})
	taskQueue.ProcessAll()
}

func TestOnNewMessage_SystemMessage_DeleteEnabled(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	messageService := mocks.NewMessageService(t)
	taskQueue := queue.New()
	h := New(
		telegramRepo,
		mocks.NewStateRepo(t),
		messageService,
		mocks.NewFilterService(t),
		mocks.NewTransformService(t),
		mocks.NewAlbumService(t),
		taskQueue,
		mocks.NewRateLimiter(t),
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) },
	)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}}
	rs := &domain.RuleSet{
		Sources:             map[int64]*domain.Source{100: {ChatID: 100, DeleteSystemMessages: true}},
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{"r1": rule},
		UniqueSources:       map[int64]struct{}{100: {}},
		UniqueDestinations:  map[int64]struct{}{200: {}},
		OrderedForwardRules: []string{"r1"},
	}
	h.SetRuleSet(rs)

	msg := &client.Message{ChatId: 100, Id: 1, Content: &client.MessageChatJoinByLink{}}
	messageService.EXPECT().IsSystemMessage(msg).Return(true)
	telegramRepo.EXPECT().DeleteMessages(mock.MatchedBy(func(req *client.DeleteMessagesRequest) bool {
		return req.ChatId == 100 && req.Revoke && len(req.MessageIds) == 1 && req.MessageIds[0] == 1
	})).Return(&client.Ok{}, nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

func TestOnNewMessage_ForwardWithoutCopy(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	tracker := mocks.NewDedupTracker(t)
	taskQueue := queue.New()
	h := New(
		telegramRepo,
		stateRepo,
		messageService,
		filterService,
		mocks.NewTransformService(t),
		mocks.NewAlbumService(t),
		taskQueue,
		rateLimiter,
		func(_ []int64) DedupTracker { return tracker },
	)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: false}
	rs := &domain.RuleSet{
		Sources:             map[int64]*domain.Source{100: {ChatID: 100}},
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{"r1": rule},
		UniqueSources:       map[int64]struct{}{100: {}},
		UniqueDestinations:  map[int64]struct{}{200: {}},
		OrderedForwardRules: []string{"r1"},
	}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         1,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "hello"}},
	}
	text := &client.FormattedText{Text: "hello"}
	messageService.EXPECT().IsSystemMessage(msg).Return(false)
	messageService.EXPECT().GetFormattedText(msg).Return(text)
	filterService.EXPECT().Evaluate("hello", rule).Return(domain.FiltersOK)
	tracker.EXPECT().TryMark(int64(200)).Return(true)
	rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(200))
	telegramRepo.EXPECT().ForwardMessages(mock.MatchedBy(func(req *client.ForwardMessagesRequest) bool {
		return req.ChatId == 200 && req.FromChatId == 100 && len(req.MessageIds) == 1 && req.MessageIds[0] == 1
	})).Return(&client.Messages{}, nil)
	// Stats
	stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.AnythingOfType("string")).Return(uint64(1), nil)
	stateRepo.EXPECT().IncrementForwardedMessages(int64(200), mock.AnythingOfType("string")).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

func TestOnNewMessage_SendCopy(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	tracker := mocks.NewDedupTracker(t)
	taskQueue := queue.New()
	h := New(
		telegramRepo,
		stateRepo,
		messageService,
		filterService,
		transformService,
		mocks.NewAlbumService(t),
		taskQueue,
		rateLimiter,
		func(_ []int64) DedupTracker { return tracker },
	)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := &domain.RuleSet{
		Sources:             map[int64]*domain.Source{100: {ChatID: 100}},
		Destinations:        map[int64]*domain.Destination{200: {ChatID: 200}},
		ForwardRules:        map[string]*domain.ForwardRule{"r1": rule},
		UniqueSources:       map[int64]struct{}{100: {}},
		UniqueDestinations:  map[int64]struct{}{200: {}},
		OrderedForwardRules: []string{"r1"},
	}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         1,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "hello"}},
	}
	text := &client.FormattedText{Text: "hello"}
	transformed := &client.FormattedText{Text: "transformed"}
	inputContent := &client.InputMessageText{Text: transformed}

	messageService.EXPECT().IsSystemMessage(msg).Return(false)
	messageService.EXPECT().GetFormattedText(msg).Return(text)
	messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte(nil))
	messageService.EXPECT().BuildInputContent(msg, transformed).Return(inputContent)
	filterService.EXPECT().Evaluate("hello", rule).Return(domain.FiltersOK)
	tracker.EXPECT().TryMark(int64(200)).Return(true)
	rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(200))
	transformService.EXPECT().Transform(mock.Anything, mock.AnythingOfType("domain.TransformParams")).Return(transformed, nil)
	telegramRepo.EXPECT().SendMessage(mock.MatchedBy(func(req *client.SendMessageRequest) bool {
		return req.ChatId == 200
	})).Return(&client.Message{Id: 500}, nil)
	stateRepo.EXPECT().SetCopiedMessageID(int64(100), int64(1), "r1:200:500").Return(nil)
	// Stats
	stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.AnythingOfType("string")).Return(uint64(1), nil)
	stateRepo.EXPECT().IncrementForwardedMessages(int64(200), mock.AnythingOfType("string")).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

func TestOnNewMessage_FiltersCheck(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	h := New(
		telegramRepo,
		stateRepo,
		messageService,
		filterService,
		mocks.NewTransformService(t),
		mocks.NewAlbumService(t),
		taskQueue,
		rateLimiter,
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) },
	)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true, Check: 300}
	rs := &domain.RuleSet{
		Sources:             map[int64]*domain.Source{100: {ChatID: 100}},
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{"r1": rule},
		UniqueSources:       map[int64]struct{}{100: {}},
		UniqueDestinations:  map[int64]struct{}{200: {}},
		OrderedForwardRules: []string{"r1"},
	}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         1,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "suspicious"}},
	}
	text := &client.FormattedText{Text: "suspicious"}
	messageService.EXPECT().IsSystemMessage(msg).Return(false)
	messageService.EXPECT().GetFormattedText(msg).Return(text)
	filterService.EXPECT().Evaluate("suspicious", rule).Return(domain.FiltersCheck)
	rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(300))
	telegramRepo.EXPECT().ForwardMessages(mock.MatchedBy(func(req *client.ForwardMessagesRequest) bool {
		return req.ChatId == 300 && req.FromChatId == 100
	})).Return(&client.Messages{}, nil)
	// Stats (viewed only)
	stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.AnythingOfType("string")).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

func TestOnNewMessage_CannotBeSaved_WithoutSendCopy(t *testing.T) {
	t.Parallel()

	// Arrange
	messageService := mocks.NewMessageService(t)
	taskQueue := queue.New()
	h := New(
		mocks.NewTelegramRepo(t),
		mocks.NewStateRepo(t),
		messageService,
		mocks.NewFilterService(t),
		mocks.NewTransformService(t),
		mocks.NewAlbumService(t),
		taskQueue,
		mocks.NewRateLimiter(t),
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) },
	)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: false}
	rs := &domain.RuleSet{
		Sources:             map[int64]*domain.Source{100: {ChatID: 100}},
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{"r1": rule},
		UniqueSources:       map[int64]struct{}{100: {}},
		UniqueDestinations:  map[int64]struct{}{200: {}},
		OrderedForwardRules: []string{"r1"},
	}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         1,
		CanBeSaved: false,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "hello"}},
	}
	text := &client.FormattedText{Text: "hello"}
	messageService.EXPECT().IsSystemMessage(msg).Return(false)
	messageService.EXPECT().GetFormattedText(msg).Return(text)

	// Act
	h.OnNewMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

func TestOnDeletedMessages_PermanentWithCopies(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	taskQueue := queue.New()
	h := New(
		telegramRepo,
		stateRepo,
		mocks.NewMessageService(t),
		mocks.NewFilterService(t),
		mocks.NewTransformService(t),
		mocks.NewAlbumService(t),
		taskQueue,
		mocks.NewRateLimiter(t),
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) },
	)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{"r1": rule},
		UniqueSources:       map[int64]struct{}{100: {}},
		UniqueDestinations:  map[int64]struct{}{200: {}},
		OrderedForwardRules: []string{"r1"},
	}
	h.SetRuleSet(rs)

	stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"r1:200:500"})
	stateRepo.EXPECT().GetNewMessageID(int64(200), int64(500)).Return(int64(600))
	telegramRepo.EXPECT().DeleteMessages(mock.MatchedBy(func(req *client.DeleteMessagesRequest) bool {
		return req.ChatId == 200 && len(req.MessageIds) == 1 && req.MessageIds[0] == 600
	})).Return(&client.Ok{}, nil)
	stateRepo.EXPECT().DeleteNewMessageID(int64(200), int64(500)).Return(nil)
	stateRepo.EXPECT().DeleteTmpMessageID(int64(200), int64(600)).Return(nil)
	stateRepo.EXPECT().DeleteAnswerMessageID(int64(200), int64(500)).Return(nil)
	stateRepo.EXPECT().DeleteCopiedMessageIDs(int64(100), int64(1)).Return(nil)

	// Act
	h.OnDeletedMessages(context.Background(), 100, []int64{1}, true)
	taskQueue.ProcessAll()
}

func TestOnDeletedMessages_IndelibleRule(t *testing.T) {
	t.Parallel()

	// Arrange
	stateRepo := mocks.NewStateRepo(t)
	taskQueue := queue.New()
	h := New(
		mocks.NewTelegramRepo(t),
		stateRepo,
		mocks.NewMessageService(t),
		mocks.NewFilterService(t),
		mocks.NewTransformService(t),
		mocks.NewAlbumService(t),
		taskQueue,
		mocks.NewRateLimiter(t),
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) },
	)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, Indelible: true}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{"r1": rule},
		UniqueSources:       map[int64]struct{}{100: {}},
		UniqueDestinations:  map[int64]struct{}{200: {}},
		OrderedForwardRules: []string{"r1"},
	}
	h.SetRuleSet(rs)

	stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"r1:200:500"})
	stateRepo.EXPECT().DeleteCopiedMessageIDs(int64(100), int64(1)).Return(nil)

	// Act
	h.OnDeletedMessages(context.Background(), 100, []int64{1}, true)
	taskQueue.ProcessAll()
}

func TestOnDeletedMessages_RetryOnMissingNewID(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	taskQueue := queue.New()
	h := New(
		telegramRepo,
		stateRepo,
		mocks.NewMessageService(t),
		mocks.NewFilterService(t),
		mocks.NewTransformService(t),
		mocks.NewAlbumService(t),
		taskQueue,
		mocks.NewRateLimiter(t),
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) },
	)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{"r1": rule},
		UniqueSources:       map[int64]struct{}{100: {}},
		UniqueDestinations:  map[int64]struct{}{200: {}},
		OrderedForwardRules: []string{"r1"},
	}
	h.SetRuleSet(rs)

	call := 0
	stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"r1:200:500"}).Times(2)
	stateRepo.EXPECT().GetNewMessageID(int64(200), int64(500)).RunAndReturn(func(_ int64, _ int64) int64 {
		call++
		if call == 1 {
			return 0
		}
		return 600
	}).Times(2)
	stateRepo.EXPECT().DeleteCopiedMessageIDs(int64(100), int64(1)).Return(nil)
	telegramRepo.EXPECT().DeleteMessages(mock.MatchedBy(func(req *client.DeleteMessagesRequest) bool {
		return req.ChatId == 200 && req.MessageIds[0] == 600
	})).Return(&client.Ok{}, nil)
	stateRepo.EXPECT().DeleteNewMessageID(int64(200), int64(500)).Return(nil)
	stateRepo.EXPECT().DeleteTmpMessageID(int64(200), int64(600)).Return(nil)
	stateRepo.EXPECT().DeleteAnswerMessageID(int64(200), int64(500)).Return(nil)

	// Act
	h.OnDeletedMessages(context.Background(), 100, []int64{1}, true)
	taskQueue.ProcessAll()
}

func TestOnMessageSendSucceeded(t *testing.T) {
	t.Parallel()

	// Arrange
	stateRepo := mocks.NewStateRepo(t)
	taskQueue := queue.New()
	h := New(
		mocks.NewTelegramRepo(t),
		stateRepo,
		mocks.NewMessageService(t),
		mocks.NewFilterService(t),
		mocks.NewTransformService(t),
		mocks.NewAlbumService(t),
		taskQueue,
		mocks.NewRateLimiter(t),
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) },
	)
	stateRepo.EXPECT().SetNewMessageID(int64(200), int64(500), int64(600)).Return(nil)
	stateRepo.EXPECT().SetTmpMessageID(int64(200), int64(600), int64(500)).Return(nil)

	// Act
	h.OnMessageSendSucceeded(200, 500, 600)
	taskQueue.ProcessAll()
}

func TestOnNewMessage_FiltersOther(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	h := New(
		telegramRepo,
		stateRepo,
		messageService,
		filterService,
		mocks.NewTransformService(t),
		mocks.NewAlbumService(t),
		taskQueue,
		rateLimiter,
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) },
	)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true, Other: 400}
	rs := &domain.RuleSet{
		Sources:             map[int64]*domain.Source{100: {ChatID: 100}},
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{"r1": rule},
		UniqueSources:       map[int64]struct{}{100: {}},
		UniqueDestinations:  map[int64]struct{}{200: {}},
		OrderedForwardRules: []string{"r1"},
	}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         1,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "unrelated"}},
	}
	text := &client.FormattedText{Text: "unrelated"}
	messageService.EXPECT().IsSystemMessage(msg).Return(false)
	messageService.EXPECT().GetFormattedText(msg).Return(text)
	filterService.EXPECT().Evaluate("unrelated", rule).Return(domain.FiltersOther)
	rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(400))
	telegramRepo.EXPECT().ForwardMessages(mock.MatchedBy(func(req *client.ForwardMessagesRequest) bool {
		return req.ChatId == 400
	})).Return(&client.Messages{}, nil)
	stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.AnythingOfType("string")).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

func TestOnEditedMessage_NoRuleSet(t *testing.T) {
	t.Parallel()

	// Arrange
	taskQueue := queue.New()
	h := New(
		mocks.NewTelegramRepo(t),
		mocks.NewStateRepo(t),
		mocks.NewMessageService(t),
		mocks.NewFilterService(t),
		mocks.NewTransformService(t),
		mocks.NewAlbumService(t),
		taskQueue,
		mocks.NewRateLimiter(t),
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) },
	)

	// Act
	h.OnEditedMessage(context.Background(), &client.Message{ChatId: 100, Id: 1})
	taskQueue.ProcessAll()
}

func TestOnEditedMessage_UnknownSource(t *testing.T) {
	t.Parallel()

	// Arrange
	taskQueue := queue.New()
	h := New(
		mocks.NewTelegramRepo(t),
		mocks.NewStateRepo(t),
		mocks.NewMessageService(t),
		mocks.NewFilterService(t),
		mocks.NewTransformService(t),
		mocks.NewAlbumService(t),
		taskQueue,
		mocks.NewRateLimiter(t),
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) },
	)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{"r1": rule},
		UniqueSources:       map[int64]struct{}{100: {}},
		UniqueDestinations:  map[int64]struct{}{200: {}},
		OrderedForwardRules: []string{"r1"},
	}
	h.SetRuleSet(rs)

	// Act
	h.OnEditedMessage(context.Background(), &client.Message{ChatId: 999, Id: 1})
	taskQueue.ProcessAll()
}

func TestOnEditedMessage_TextUpdate(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	transformService := mocks.NewTransformService(t)
	taskQueue := queue.New()
	h := New(
		telegramRepo,
		stateRepo,
		messageService,
		mocks.NewFilterService(t),
		transformService,
		mocks.NewAlbumService(t),
		taskQueue,
		mocks.NewRateLimiter(t),
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) },
	)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := &domain.RuleSet{
		Sources:             map[int64]*domain.Source{100: {ChatID: 100}},
		Destinations:        map[int64]*domain.Destination{200: {ChatID: 200}},
		ForwardRules:        map[string]*domain.ForwardRule{"r1": rule},
		UniqueSources:       map[int64]struct{}{100: {}},
		UniqueDestinations:  map[int64]struct{}{200: {}},
		OrderedForwardRules: []string{"r1"},
	}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         1,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "edited text"}},
	}
	text := &client.FormattedText{Text: "edited text"}
	transformed := &client.FormattedText{Text: "transformed edit"}

	stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"r1:200:500"})
	stateRepo.EXPECT().GetNewMessageID(int64(200), int64(500)).Return(int64(600))
	messageService.EXPECT().GetFormattedText(msg).Return(text)
	messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte(nil))
	transformService.EXPECT().Transform(mock.Anything, mock.AnythingOfType("domain.TransformParams")).Return(transformed, nil)
	telegramRepo.EXPECT().EditMessageText(mock.MatchedBy(func(req *client.EditMessageTextRequest) bool {
		if req.ChatId != 200 || req.MessageId != 600 {
			return false
		}
		t, ok := req.InputMessageContent.(*client.InputMessageText)
		return ok && t.Text == transformed
	})).Return(&client.Message{Id: 600}, nil)
	stateRepo.EXPECT().DeleteAnswerMessageID(int64(200), int64(500)).Return(nil)

	// Act
	h.OnEditedMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

func TestOnEditedMessage_CaptionUpdate(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	transformService := mocks.NewTransformService(t)
	taskQueue := queue.New()
	h := New(
		telegramRepo,
		stateRepo,
		messageService,
		mocks.NewFilterService(t),
		transformService,
		mocks.NewAlbumService(t),
		taskQueue,
		mocks.NewRateLimiter(t),
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) },
	)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := &domain.RuleSet{
		Sources:             map[int64]*domain.Source{100: {ChatID: 100}},
		Destinations:        map[int64]*domain.Destination{200: {ChatID: 200}},
		ForwardRules:        map[string]*domain.ForwardRule{"r1": rule},
		UniqueSources:       map[int64]struct{}{100: {}},
		UniqueDestinations:  map[int64]struct{}{200: {}},
		OrderedForwardRules: []string{"r1"},
	}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:  100,
		Id:      1,
		Content: &client.MessagePhoto{Caption: &client.FormattedText{Text: "new caption"}},
	}
	text := &client.FormattedText{Text: "new caption"}
	transformed := &client.FormattedText{Text: "transformed caption"}

	stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"r1:200:500"})
	stateRepo.EXPECT().GetNewMessageID(int64(200), int64(500)).Return(int64(600))
	messageService.EXPECT().GetFormattedText(msg).Return(text)
	messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte(nil))
	transformService.EXPECT().Transform(mock.Anything, mock.AnythingOfType("domain.TransformParams")).Return(transformed, nil)
	telegramRepo.EXPECT().EditMessageCaption(mock.MatchedBy(func(req *client.EditMessageCaptionRequest) bool {
		return req.ChatId == 200 && req.MessageId == 600 && req.Caption == transformed
	})).Return(&client.Message{Id: 600}, nil)
	stateRepo.EXPECT().DeleteAnswerMessageID(int64(200), int64(500)).Return(nil)

	// Act
	h.OnEditedMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

func TestOnEditedMessage_CopyOnce_Versioning(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	transformService := mocks.NewTransformService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	h := New(
		telegramRepo,
		stateRepo,
		messageService,
		mocks.NewFilterService(t),
		transformService,
		mocks.NewAlbumService(t),
		taskQueue,
		rateLimiter,
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) },
	)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true, CopyOnce: true}
	rs := &domain.RuleSet{
		Sources:             map[int64]*domain.Source{100: {ChatID: 100}},
		Destinations:        map[int64]*domain.Destination{200: {ChatID: 200}},
		ForwardRules:        map[string]*domain.ForwardRule{"r1": rule},
		UniqueSources:       map[int64]struct{}{100: {}},
		UniqueDestinations:  map[int64]struct{}{200: {}},
		OrderedForwardRules: []string{"r1"},
	}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         1,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "v2"}},
	}
	text := &client.FormattedText{Text: "v2"}
	transformed := &client.FormattedText{Text: "transformed v2"}
	inputContent := &client.InputMessageText{Text: transformed}

	stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"r1:200:500"})
	stateRepo.EXPECT().GetNewMessageID(int64(200), int64(500)).Return(int64(600))
	messageService.EXPECT().GetFormattedText(msg).Return(text)
	messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte(nil))
	transformService.EXPECT().Transform(mock.Anything, mock.AnythingOfType("domain.TransformParams")).Return(transformed, nil)
	messageService.EXPECT().BuildInputContent(msg, transformed).Return(inputContent)
	rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(200))
	telegramRepo.EXPECT().SendMessage(mock.MatchedBy(func(req *client.SendMessageRequest) bool {
		return req.ChatId == 200
	})).Return(&client.Message{Id: 700}, nil)
	stateRepo.EXPECT().SetCopiedMessageID(int64(100), int64(1), "r1:200:700").Return(nil)

	// Act — cancelled context останавливает goroutine runNextLinkWorkflow
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h.OnEditedMessage(ctx, msg)
	taskQueue.ProcessAll()
}

func TestOnEditedMessage_RetryOnMissingNewID(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	transformService := mocks.NewTransformService(t)
	taskQueue := queue.New()
	h := New(
		telegramRepo,
		stateRepo,
		messageService,
		mocks.NewFilterService(t),
		transformService,
		mocks.NewAlbumService(t),
		taskQueue,
		mocks.NewRateLimiter(t),
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) },
	)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := &domain.RuleSet{
		Sources:             map[int64]*domain.Source{100: {ChatID: 100}},
		Destinations:        map[int64]*domain.Destination{200: {ChatID: 200}},
		ForwardRules:        map[string]*domain.ForwardRule{"r1": rule},
		UniqueSources:       map[int64]struct{}{100: {}},
		UniqueDestinations:  map[int64]struct{}{200: {}},
		OrderedForwardRules: []string{"r1"},
	}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         1,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "edit"}},
	}
	text := &client.FormattedText{Text: "edit"}
	transformed := &client.FormattedText{Text: "transformed"}

	call := 0
	stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"r1:200:500"}).Times(2)
	stateRepo.EXPECT().GetNewMessageID(int64(200), int64(500)).RunAndReturn(func(_ int64, _ int64) int64 {
		call++
		if call == 1 {
			return 0
		}
		return 600
	}).Times(2)
	messageService.EXPECT().GetFormattedText(msg).Return(text).Times(2)
	messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte(nil))
	transformService.EXPECT().Transform(mock.Anything, mock.AnythingOfType("domain.TransformParams")).Return(transformed, nil)
	telegramRepo.EXPECT().EditMessageText(mock.Anything).Return(&client.Message{Id: 600}, nil)
	stateRepo.EXPECT().DeleteAnswerMessageID(int64(200), int64(500)).Return(nil)

	// Act
	h.OnEditedMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

func TestOnEditedMessage_ReplyMarkupSync(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	transformService := mocks.NewTransformService(t)
	taskQueue := queue.New()
	h := New(
		telegramRepo,
		stateRepo,
		messageService,
		mocks.NewFilterService(t),
		transformService,
		mocks.NewAlbumService(t),
		taskQueue,
		mocks.NewRateLimiter(t),
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) },
	)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := &domain.RuleSet{
		Sources:             map[int64]*domain.Source{100: {ChatID: 100}},
		Destinations:        map[int64]*domain.Destination{200: {ChatID: 200}},
		ForwardRules:        map[string]*domain.ForwardRule{"r1": rule},
		UniqueSources:       map[int64]struct{}{100: {}},
		UniqueDestinations:  map[int64]struct{}{200: {}},
		OrderedForwardRules: []string{"r1"},
	}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         1,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "with button"}},
	}
	text := &client.FormattedText{Text: "with button"}
	transformed := &client.FormattedText{Text: "transformed"}

	stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"r1:200:500"})
	stateRepo.EXPECT().GetNewMessageID(int64(200), int64(500)).Return(int64(600))
	messageService.EXPECT().GetFormattedText(msg).Return(text)
	messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte("action"))
	transformService.EXPECT().Transform(mock.Anything, mock.AnythingOfType("domain.TransformParams")).Return(transformed, nil)
	telegramRepo.EXPECT().EditMessageText(mock.Anything).Return(&client.Message{Id: 600}, nil)
	stateRepo.EXPECT().SetAnswerMessageID(int64(200), int64(500), int64(100), int64(1)).Return(nil)

	// Act
	h.OnEditedMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

func TestSetRuleSet(t *testing.T) {
	t.Parallel()

	// Arrange
	taskQueue := queue.New()
	h := New(
		mocks.NewTelegramRepo(t),
		mocks.NewStateRepo(t),
		mocks.NewMessageService(t),
		mocks.NewFilterService(t),
		mocks.NewTransformService(t),
		mocks.NewAlbumService(t),
		taskQueue,
		mocks.NewRateLimiter(t),
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) },
	)
	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{"r1": rule},
		UniqueSources:       map[int64]struct{}{100: {}},
		UniqueDestinations:  map[int64]struct{}{200: {}},
		OrderedForwardRules: []string{"r1"},
	}

	// Act
	h.SetRuleSet(rs)

	// Assert
	loaded := h.ruleset.Load()
	assert.NotNil(t, loaded)
	assert.Contains(t, loaded.ForwardRules, "r1")
}
