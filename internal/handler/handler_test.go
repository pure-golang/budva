package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/handler/mocks"
	"github.com/pure-golang/budva-claude/internal/repo/queue"
)

func TestRun_DispatchesMessageSendSucceeded(t *testing.T) {
	t.Parallel()

	// Arrange
	clientDone := make(chan struct{})
	close(clientDone)
	updates := make(chan client.Type, 1)
	updates <- &client.UpdateMessageSendSucceeded{
		Message:      &client.Message{ChatId: 200, Id: 600},
		OldMessageId: 500,
	}
	close(updates)

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

	telegramRepo.EXPECT().ClientDone().Return(clientDone)
	telegramRepo.EXPECT().Updates().Return((<-chan client.Type)(updates))
	stateRepo.EXPECT().SetNewMessageID(int64(200), int64(500), int64(600)).Return(nil)
	stateRepo.EXPECT().SetTmpMessageID(int64(200), int64(600), int64(500)).Return(nil)

	// Act
	h.Run(context.Background())
	taskQueue.ProcessAll()

	// Assert
	assert.Zero(t, taskQueue.Len())
}

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

// TestRunNextLinkWorkflow_CtxDone — отменённый ctx приводит к немедленному return в первой итерации.
func TestRunNextLinkWorkflow_CtxDone(t *testing.T) {
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
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Act — должен выйти сразу по ctx.Done.
	h.runNextLinkWorkflow(ctx, &domain.Source{ChatID: 100}, 200, 50, 51)
	taskQueue.ProcessAll()

	// Assert — без моков: ни одной операции.
}

// TestRunNextLinkWorkflow_Success — покрывает успешный путь (GetMessage + AddNextLink + EditMessageText).
// Занимает ~1s из-за захардкоженного time.After(1*time.Second) в цикле.
func TestRunNextLinkWorkflow_Success(t *testing.T) {
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
	src := &domain.Source{ChatID: 100}

	prevMsg := &client.Message{ChatId: 200, Id: 500, Content: &client.MessageText{Text: &client.FormattedText{Text: "prev"}}}
	prevText := &client.FormattedText{Text: "prev"}
	updated := &client.FormattedText{Text: "prev with next"}

	stateRepo.EXPECT().GetNewMessageID(int64(200), int64(51)).Return(int64(52))
	telegramRepo.EXPECT().GetMessage(mock.MatchedBy(func(req *client.GetMessageRequest) bool {
		return req.ChatId == 200 && req.MessageId == 500
	})).Return(prevMsg, nil)
	messageService.EXPECT().GetFormattedText(prevMsg).Return(prevText)
	transformService.EXPECT().AddNextLink(mock.Anything, prevText, src, int64(200), int64(52)).Return(updated)
	telegramRepo.EXPECT().EditMessageText(mock.MatchedBy(func(req *client.EditMessageTextRequest) bool {
		return req.ChatId == 200 && req.MessageId == 500
	})).Return(&client.Message{}, errors.New("edit fail")) // покрывает error branch тоже

	// Act
	h.runNextLinkWorkflow(context.Background(), src, 200, 500, 51)
	taskQueue.ProcessAll()
}

// TestRunNextLinkWorkflow_GetMessageFail — ошибка GetMessage останавливает loop.
func TestRunNextLinkWorkflow_GetMessageFail(t *testing.T) {
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

	stateRepo.EXPECT().GetNewMessageID(int64(200), int64(51)).Return(int64(52))
	telegramRepo.EXPECT().GetMessage(mock.Anything).Return(nil, errors.New("not found"))

	// Act
	h.runNextLinkWorkflow(context.Background(), &domain.Source{ChatID: 100}, 200, 500, 51)
	taskQueue.ProcessAll()
}

// TestRunNextLinkWorkflow_NilText — text == nil, return.
func TestRunNextLinkWorkflow_NilText(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	taskQueue := queue.New()
	h := New(
		telegramRepo,
		stateRepo,
		messageService,
		mocks.NewFilterService(t),
		mocks.NewTransformService(t),
		mocks.NewAlbumService(t),
		taskQueue,
		mocks.NewRateLimiter(t),
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) },
	)

	prevMsg := &client.Message{ChatId: 200, Id: 500}
	stateRepo.EXPECT().GetNewMessageID(int64(200), int64(51)).Return(int64(52))
	telegramRepo.EXPECT().GetMessage(mock.Anything).Return(prevMsg, nil)
	messageService.EXPECT().GetFormattedText(prevMsg).Return(nil)

	// Act
	h.runNextLinkWorkflow(context.Background(), &domain.Source{ChatID: 100}, 200, 500, 51)
	taskQueue.ProcessAll()
}

// TestParseCopyRef_Variants — табличный тест для приватной parseCopyRef.
func TestParseCopyRef_Variants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		wantOK     bool
		wantRuleID string
		wantDstID  int64
		wantTmpID  int64
	}{
		{name: "valid", input: "r1:200:500", wantOK: true, wantRuleID: "r1", wantDstID: 200, wantTmpID: 500},
		{name: "valid_with_extra_parts", input: "r1:200:500:extra", wantOK: true, wantRuleID: "r1", wantDstID: 200, wantTmpID: 500},
		{name: "too_few_parts", input: "r1:200", wantOK: false},
		{name: "single_part", input: "r1", wantOK: false},
		{name: "empty", input: "", wantOK: false},
		{name: "bad_dst_id", input: "r1:not_a_number:500", wantOK: true, wantRuleID: "r1", wantDstID: 0, wantTmpID: 500},
		{name: "bad_tmp_id", input: "r1:200:not_a_number", wantOK: true, wantRuleID: "r1", wantDstID: 200, wantTmpID: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			got, ok := parseCopyRef(tt.input)

			// Assert
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if tt.wantOK {
				if got.ruleID != tt.wantRuleID || got.dstChatID != tt.wantDstID || got.tmpMsgID != tt.wantTmpID {
					t.Errorf("got %+v, want rule=%q dst=%d tmp=%d", got, tt.wantRuleID, tt.wantDstID, tt.wantTmpID)
				}
			}
		})
	}
}

// TestParseID_Variants — ParseInt fallback возвращает 0 для невалидного входа.
func TestParseID_Variants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  int64
	}{
		{name: "valid_positive", input: "42", want: 42},
		{name: "valid_negative", input: "-100", want: -100},
		{name: "zero", input: "0", want: 0},
		{name: "empty_returns_zero", input: "", want: 0},
		{name: "invalid_returns_zero", input: "abc", want: 0},
		{name: "overflow_returns_zero", input: "99999999999999999999", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := parseID(tt.input)

			// Assert
			if got != tt.want {
				t.Errorf("parseID(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// TestEditMessages_CaptionBranch_SetAnswer — обновление caption с reply markup: SetAnswerMessageID vs DeleteAnswer.
func TestEditMessages_CaptionBranch_SetAnswer(t *testing.T) {
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
		Sources:       map[int64]*domain.Source{100: {ChatID: 100}},
		Destinations:  map[int64]*domain.Destination{200: {ChatID: 200}},
		ForwardRules:  map[string]*domain.ForwardRule{"r1": rule},
		UniqueSources: map[int64]struct{}{100: {}},
	}

	msg := &client.Message{
		ChatId:  100,
		Id:      1,
		Content: &client.MessagePhoto{Caption: &client.FormattedText{Text: "c"}},
	}
	transformed := &client.FormattedText{Text: "t"}

	stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"r1:200:500"})
	stateRepo.EXPECT().GetNewMessageID(int64(200), int64(500)).Return(int64(600))
	messageService.EXPECT().GetFormattedText(msg).Return(&client.FormattedText{Text: "c"})
	messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte("btn"))
	transformService.EXPECT().Transform(mock.Anything, mock.Anything).Return(transformed, nil)
	telegramRepo.EXPECT().EditMessageCaption(mock.Anything).Return(&client.Message{Id: 600}, nil)
	stateRepo.EXPECT().SetAnswerMessageID(int64(200), int64(500), int64(100), int64(1)).Return(errors.New("set fail"))

	// Act — прямой вызов editMessages.
	needRetry := h.editMessages(context.Background(), rs, msg)

	// Assert
	if needRetry {
		t.Fatal("editMessages returned needRetry=true unexpectedly")
	}
}

// TestOnNewMessage_SystemMessage_NoDeleteFlag — системное сообщение без DeleteSystemMessages.
// Покрывает ветку где src != nil но флаг выключен, а также ветку когда src == nil.
func TestOnNewMessage_SystemMessage_NoDeleteFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		srcSet bool
	}{
		{name: "src_present_flag_off", srcSet: true},
		{name: "src_nil", srcSet: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			telegramRepo := mocks.NewTelegramRepo(t)
			stateRepo := mocks.NewStateRepo(t)
			messageService := mocks.NewMessageService(t)
			filterService := mocks.NewFilterService(t)
			transformService := mocks.NewTransformService(t)
			albumService := mocks.NewAlbumService(t)
			rateLimiter := mocks.NewRateLimiter(t)
			taskQueue := queue.New()
			h := New(telegramRepo, stateRepo, messageService, filterService,
				transformService, albumService, taskQueue, rateLimiter,
				func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) })

			rs := &domain.RuleSet{
				Sources:             make(map[int64]*domain.Source),
				Destinations:        make(map[int64]*domain.Destination),
				ForwardRules:        map[string]*domain.ForwardRule{"r1": &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}}},
				OrderedForwardRules: []string{"r1"},
				UniqueSources:       map[int64]struct{}{100: {}},
				UniqueDestinations:  map[int64]struct{}{200: {}},
			}
			if tt.srcSet {
				rs.Sources[100] = &domain.Source{ChatID: 100, DeleteSystemMessages: false}
			}
			h.SetRuleSet(rs)

			msg := &client.Message{ChatId: 100, Id: 1, Content: &client.MessageChatJoinByLink{}}
			messageService.EXPECT().IsSystemMessage(msg).Return(true)

			// Act
			h.OnNewMessage(context.Background(), msg)
			taskQueue.ProcessAll()

			// Assert — не должен звать DeleteMessages.
		})
	}
}

// TestOnNewMessage_SystemMessage_DeleteFails — ошибка DeleteMessages логируется, не паникует.
func TestOnNewMessage_SystemMessage_DeleteFails(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) })

	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{"r1": &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}}},
		OrderedForwardRules: []string{"r1"},
		UniqueSources:       map[int64]struct{}{100: {}},
		UniqueDestinations:  map[int64]struct{}{200: {}},
	}
	rs.Sources[100] = &domain.Source{ChatID: 100, DeleteSystemMessages: true}
	h.SetRuleSet(rs)

	msg := &client.Message{ChatId: 100, Id: 1, Content: &client.MessageChatJoinByLink{}}
	messageService.EXPECT().IsSystemMessage(msg).Return(true)
	telegramRepo.EXPECT().DeleteMessages(mock.Anything).Return(nil, errors.New("tg down"))

	// Act
	h.OnNewMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

// TestOnNewMessage_NoFormattedText — GetFormattedText возвращает nil, обработка прекращается.
func TestOnNewMessage_NoFormattedText(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) })

	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{"r1": &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}}},
		OrderedForwardRules: []string{"r1"},
		UniqueSources:       map[int64]struct{}{100: {}},
		UniqueDestinations:  map[int64]struct{}{200: {}},
	}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         1,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "hi"}},
	}
	messageService.EXPECT().IsSystemMessage(msg).Return(false)
	messageService.EXPECT().GetFormattedText(msg).Return(nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

// TestOnNewMessage_RuleFromMismatch — правило с другим From пропускается.
func TestOnNewMessage_RuleFromMismatch(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	tracker := mocks.NewDedupTracker(t)
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return tracker })

	rule1 := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: false}
	rule2 := &domain.ForwardRule{ID: "r2", From: 101, To: []int64{201}, SendCopy: false}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule1.ID: rule1, rule2.ID: rule2},
		OrderedForwardRules: []string{rule1.ID, rule2.ID},
		UniqueSources:       map[int64]struct{}{rule1.From: {}, rule2.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	// важно: UniqueSources для 100 есть, но r2.From=101 mismatch'ится и пропустится
	rs.Sources[100] = &domain.Source{ChatID: 100}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         1,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "hi"}},
	}
	text := &client.FormattedText{Text: "hi"}
	messageService.EXPECT().IsSystemMessage(msg).Return(false)
	messageService.EXPECT().GetFormattedText(msg).Return(text)
	// Только r1 дойдёт до Evaluate.
	filterService.EXPECT().Evaluate("hi", rule1).Return(domain.FiltersOK)
	tracker.EXPECT().TryMark(int64(200)).Return(true)
	// r1 — forward без copy.
	rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(200))
	telegramRepo.EXPECT().ForwardMessages(mock.Anything).Return(&client.Messages{}, nil)
	stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.Anything).Return(uint64(1), nil)
	stateRepo.EXPECT().IncrementForwardedMessages(int64(200), mock.Anything).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

// TestOnNewMessage_Dedup_SkipsDuplicate — tracker.TryMark=false пропускает получателя.
func TestOnNewMessage_Dedup_SkipsDuplicate(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	tracker := mocks.NewDedupTracker(t)
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return tracker })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200, 201}, SendCopy: false}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         1,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "hi"}},
	}
	messageService.EXPECT().IsSystemMessage(msg).Return(false)
	messageService.EXPECT().GetFormattedText(msg).Return(&client.FormattedText{Text: "hi"})
	filterService.EXPECT().Evaluate("hi", rule).Return(domain.FiltersOK)
	tracker.EXPECT().TryMark(int64(200)).Return(true)
	tracker.EXPECT().TryMark(int64(201)).Return(false) // уже обработан
	rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(200))
	telegramRepo.EXPECT().ForwardMessages(mock.Anything).Return(&client.Messages{}, nil)
	stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.Anything).Return(uint64(1), nil)
	stateRepo.EXPECT().IncrementViewedMessages(int64(201), mock.Anything).Return(uint64(1), nil)
	stateRepo.EXPECT().IncrementForwardedMessages(int64(200), mock.Anything).Return(uint64(1), nil)
	stateRepo.EXPECT().IncrementForwardedMessages(int64(201), mock.Anything).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

// TestOnNewMessage_FiltersCheck_NoCheckChat — mode Check, но Check=0 пропускается.
func TestOnNewMessage_FiltersCheck_NoCheckChat(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true, Check: 0}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         1,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "bad"}},
	}
	messageService.EXPECT().IsSystemMessage(msg).Return(false)
	messageService.EXPECT().GetFormattedText(msg).Return(&client.FormattedText{Text: "bad"})
	filterService.EXPECT().Evaluate("bad", rule).Return(domain.FiltersCheck)
	stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.Anything).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

// TestOnNewMessage_FiltersOther_NoOtherChat — mode Other с Other=0.
func TestOnNewMessage_FiltersOther_NoOtherChat(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true, Other: 0}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         1,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "x"}},
	}
	messageService.EXPECT().IsSystemMessage(msg).Return(false)
	messageService.EXPECT().GetFormattedText(msg).Return(&client.FormattedText{Text: "x"})
	filterService.EXPECT().Evaluate("x", rule).Return(domain.FiltersOther)
	stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.Anything).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

// TestOnNewMessage_FiltersCheck_ForwardFails — ForwardMessages в check возвращает ошибку.
func TestOnNewMessage_FiltersCheck_ForwardFails(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true, Check: 300}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         1,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "bad"}},
	}
	messageService.EXPECT().IsSystemMessage(msg).Return(false)
	messageService.EXPECT().GetFormattedText(msg).Return(&client.FormattedText{Text: "bad"})
	filterService.EXPECT().Evaluate("bad", rule).Return(domain.FiltersCheck)
	rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(300))
	telegramRepo.EXPECT().ForwardMessages(mock.Anything).Return(nil, errors.New("fail"))
	stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.Anything).Return(uint64(0), errors.New("stats err"))

	// Act
	h.OnNewMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

// TestOnNewMessage_FiltersOther_ForwardFails — ForwardMessages в other возвращает ошибку.
func TestOnNewMessage_FiltersOther_ForwardFails(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true, Other: 400}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         1,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "x"}},
	}
	messageService.EXPECT().IsSystemMessage(msg).Return(false)
	messageService.EXPECT().GetFormattedText(msg).Return(&client.FormattedText{Text: "x"})
	filterService.EXPECT().Evaluate("x", rule).Return(domain.FiltersOther)
	rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(400))
	telegramRepo.EXPECT().ForwardMessages(mock.Anything).Return(nil, errors.New("fail"))
	stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.Anything).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

// TestOnNewMessage_Forward_Fails — error path ForwardMessages для SendCopy=false.
func TestOnNewMessage_Forward_Fails(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	tracker := mocks.NewDedupTracker(t)
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return tracker })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: false}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         1,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "hi"}},
	}
	messageService.EXPECT().IsSystemMessage(msg).Return(false)
	messageService.EXPECT().GetFormattedText(msg).Return(&client.FormattedText{Text: "hi"})
	filterService.EXPECT().Evaluate("hi", rule).Return(domain.FiltersOK)
	tracker.EXPECT().TryMark(int64(200)).Return(true)
	rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(200))
	telegramRepo.EXPECT().ForwardMessages(mock.Anything).Return(nil, errors.New("forward fail"))
	stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.Anything).Return(uint64(1), nil)
	stateRepo.EXPECT().IncrementForwardedMessages(int64(200), mock.Anything).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

// TestOnNewMessage_SendCopy_TransformError — Transform возвращает ошибку, send не вызывается.
func TestOnNewMessage_SendCopy_TransformError(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	tracker := mocks.NewDedupTracker(t)
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return tracker })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	rs.Destinations[200] = &domain.Destination{ChatID: 200}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         1,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "hi"}},
	}
	messageService.EXPECT().IsSystemMessage(msg).Return(false)
	messageService.EXPECT().GetFormattedText(msg).Return(&client.FormattedText{Text: "hi"})
	messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte(nil))
	filterService.EXPECT().Evaluate("hi", rule).Return(domain.FiltersOK)
	tracker.EXPECT().TryMark(int64(200)).Return(true)
	rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(200))
	transformService.EXPECT().Transform(mock.Anything, mock.Anything).Return(nil, errors.New("transform fail"))
	stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.Anything).Return(uint64(1), nil)
	stateRepo.EXPECT().IncrementForwardedMessages(int64(200), mock.Anything).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

// TestOnNewMessage_SendCopy_SendFails — SendMessage ошибка, state не обновляется.
func TestOnNewMessage_SendCopy_SendFails(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	tracker := mocks.NewDedupTracker(t)
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return tracker })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	rs.Destinations[200] = &domain.Destination{ChatID: 200}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         1,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "hi"}},
	}
	transformed := &client.FormattedText{Text: "t"}
	messageService.EXPECT().IsSystemMessage(msg).Return(false)
	messageService.EXPECT().GetFormattedText(msg).Return(&client.FormattedText{Text: "hi"})
	messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte(nil))
	messageService.EXPECT().BuildInputContent(msg, transformed).Return(&client.InputMessageText{Text: transformed})
	filterService.EXPECT().Evaluate("hi", rule).Return(domain.FiltersOK)
	tracker.EXPECT().TryMark(int64(200)).Return(true)
	rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(200))
	transformService.EXPECT().Transform(mock.Anything, mock.Anything).Return(transformed, nil)
	telegramRepo.EXPECT().SendMessage(mock.Anything).Return(nil, errors.New("send fail"))
	stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.Anything).Return(uint64(1), nil)
	stateRepo.EXPECT().IncrementForwardedMessages(int64(200), mock.Anything).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

// TestOnNewMessage_SendCopy_ReplyMarkup — SetAnswerMessageID вызывается когда есть reply markup.
func TestOnNewMessage_SendCopy_ReplyMarkup(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	tracker := mocks.NewDedupTracker(t)
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return tracker })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	rs.Destinations[200] = &domain.Destination{ChatID: 200}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         1,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "hi"}},
	}
	transformed := &client.FormattedText{Text: "t"}
	messageService.EXPECT().IsSystemMessage(msg).Return(false)
	messageService.EXPECT().GetFormattedText(msg).Return(&client.FormattedText{Text: "hi"})
	messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte("btn"))
	messageService.EXPECT().BuildInputContent(msg, transformed).Return(&client.InputMessageText{Text: transformed})
	filterService.EXPECT().Evaluate("hi", rule).Return(domain.FiltersOK)
	tracker.EXPECT().TryMark(int64(200)).Return(true)
	rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(200))
	transformService.EXPECT().Transform(mock.Anything, mock.Anything).Return(transformed, nil)
	telegramRepo.EXPECT().SendMessage(mock.Anything).Return(&client.Message{Id: 500}, nil)
	stateRepo.EXPECT().SetCopiedMessageID(int64(100), int64(1), "r1:200:500").Return(errors.New("set fail"))
	stateRepo.EXPECT().SetAnswerMessageID(int64(200), int64(500), int64(100), int64(1)).Return(errors.New("set fail"))
	stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.Anything).Return(uint64(1), nil)
	stateRepo.EXPECT().IncrementForwardedMessages(int64(200), mock.Anything).Return(uint64(1), errors.New("fwd err"))

	// Act
	h.OnNewMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

// TestOnNewMessage_SendCopy_WithReplyTo — resolveReplyTo находит копию и возвращает InputMessageReplyToMessage.
func TestOnNewMessage_SendCopy_WithReplyTo(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	tracker := mocks.NewDedupTracker(t)
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return tracker })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	rs.Destinations[200] = &domain.Destination{ChatID: 200}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         10,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "reply"}},
	}
	msg.ReplyTo = &client.MessageReplyToMessage{ChatId: 100, MessageId: 5}
	transformed := &client.FormattedText{Text: "t"}

	messageService.EXPECT().IsSystemMessage(msg).Return(false)
	messageService.EXPECT().GetFormattedText(msg).Return(&client.FormattedText{Text: "reply"})
	messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte(nil))
	messageService.EXPECT().BuildInputContent(msg, transformed).Return(&client.InputMessageText{Text: transformed})
	filterService.EXPECT().Evaluate("reply", rule).Return(domain.FiltersOK)
	tracker.EXPECT().TryMark(int64(200)).Return(true)
	rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(200))
	transformService.EXPECT().Transform(mock.Anything, mock.Anything).Return(transformed, nil)

	// resolveReplyTo: ответ на сообщение id=5, уже есть копия в 200.
	stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(5)).Return([]string{"r1:200:400"})
	stateRepo.EXPECT().GetNewMessageID(int64(200), int64(400)).Return(int64(401))

	telegramRepo.EXPECT().SendMessage(mock.MatchedBy(func(req *client.SendMessageRequest) bool {
		r, ok := req.ReplyTo.(*client.InputMessageReplyToMessage)
		return ok && r.MessageId == 401
	})).Return(&client.Message{Id: 500}, nil)
	stateRepo.EXPECT().SetCopiedMessageID(int64(100), int64(10), "r1:200:500").Return(nil)
	stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.Anything).Return(uint64(1), nil)
	stateRepo.EXPECT().IncrementForwardedMessages(int64(200), mock.Anything).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

// TestOnNewMessage_SendCopy_ReplyTo_DifferentChat — reply на сообщение из другого чата, replyTo=nil.
func TestOnNewMessage_SendCopy_ReplyTo_DifferentChat(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	tracker := mocks.NewDedupTracker(t)
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return tracker })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	rs.Destinations[200] = &domain.Destination{ChatID: 200}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         10,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "reply"}},
	}
	msg.ReplyTo = &client.MessageReplyToMessage{ChatId: 999, MessageId: 5} // другой чат
	transformed := &client.FormattedText{Text: "t"}

	messageService.EXPECT().IsSystemMessage(msg).Return(false)
	messageService.EXPECT().GetFormattedText(msg).Return(&client.FormattedText{Text: "reply"})
	messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte(nil))
	messageService.EXPECT().BuildInputContent(msg, transformed).Return(&client.InputMessageText{Text: transformed})
	filterService.EXPECT().Evaluate("reply", rule).Return(domain.FiltersOK)
	tracker.EXPECT().TryMark(int64(200)).Return(true)
	rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(200))
	transformService.EXPECT().Transform(mock.Anything, mock.Anything).Return(transformed, nil)

	// Проверяем что ReplyTo либо nil, либо typed-nil — в обоих случаях TDLib интерпретирует как "no reply".
	telegramRepo.EXPECT().SendMessage(mock.MatchedBy(func(req *client.SendMessageRequest) bool {
		if req.ReplyTo == nil {
			return true
		}
		r, ok := req.ReplyTo.(*client.InputMessageReplyToMessage)
		return ok && r == nil
	})).Return(&client.Message{Id: 500}, nil)
	stateRepo.EXPECT().SetCopiedMessageID(int64(100), int64(10), "r1:200:500").Return(nil)
	stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.Anything).Return(uint64(1), nil)
	stateRepo.EXPECT().IncrementForwardedMessages(int64(200), mock.Anything).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

// TestOnNewMessage_SendCopy_ReplyTo_NoMatchingCopy — reply в том же чате, но нет копии в целевом чате.
func TestOnNewMessage_SendCopy_ReplyTo_NoMatchingCopy(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	tracker := mocks.NewDedupTracker(t)
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return tracker })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	rs.Destinations[200] = &domain.Destination{ChatID: 200}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         10,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "r"}},
	}
	msg.ReplyTo = &client.MessageReplyToMessage{ChatId: 100, MessageId: 5}
	transformed := &client.FormattedText{Text: "t"}

	messageService.EXPECT().IsSystemMessage(msg).Return(false)
	messageService.EXPECT().GetFormattedText(msg).Return(&client.FormattedText{Text: "r"})
	messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte(nil))
	messageService.EXPECT().BuildInputContent(msg, transformed).Return(&client.InputMessageText{Text: transformed})
	filterService.EXPECT().Evaluate("r", rule).Return(domain.FiltersOK)
	tracker.EXPECT().TryMark(int64(200)).Return(true)
	rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(200))
	transformService.EXPECT().Transform(mock.Anything, mock.Anything).Return(transformed, nil)

	// копия в другом чате (201, не 200) — не матчится
	stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(5)).Return([]string{"r1:201:300", "bogus"})

	telegramRepo.EXPECT().SendMessage(mock.MatchedBy(func(req *client.SendMessageRequest) bool {
		if req.ReplyTo == nil {
			return true
		}
		r, ok := req.ReplyTo.(*client.InputMessageReplyToMessage)
		return ok && r == nil
	})).Return(&client.Message{Id: 500}, nil)
	stateRepo.EXPECT().SetCopiedMessageID(int64(100), int64(10), "r1:200:500").Return(nil)
	stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.Anything).Return(uint64(1), nil)
	stateRepo.EXPECT().IncrementForwardedMessages(int64(200), mock.Anything).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

// TestOnNewMessage_SendCopy_ReplyTo_NewIDZero — есть копия, но GetNewMessageID=0.
func TestOnNewMessage_SendCopy_ReplyTo_NewIDZero(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	tracker := mocks.NewDedupTracker(t)
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return tracker })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	rs.Destinations[200] = &domain.Destination{ChatID: 200}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         10,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "r"}},
	}
	msg.ReplyTo = &client.MessageReplyToMessage{ChatId: 100, MessageId: 5}
	transformed := &client.FormattedText{Text: "t"}

	messageService.EXPECT().IsSystemMessage(msg).Return(false)
	messageService.EXPECT().GetFormattedText(msg).Return(&client.FormattedText{Text: "r"})
	messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte(nil))
	messageService.EXPECT().BuildInputContent(msg, transformed).Return(&client.InputMessageText{Text: transformed})
	filterService.EXPECT().Evaluate("r", rule).Return(domain.FiltersOK)
	tracker.EXPECT().TryMark(int64(200)).Return(true)
	rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(200))
	transformService.EXPECT().Transform(mock.Anything, mock.Anything).Return(transformed, nil)

	stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(5)).Return([]string{"r1:200:400"})
	stateRepo.EXPECT().GetNewMessageID(int64(200), int64(400)).Return(int64(0))

	telegramRepo.EXPECT().SendMessage(mock.MatchedBy(func(req *client.SendMessageRequest) bool {
		if req.ReplyTo == nil {
			return true
		}
		r, ok := req.ReplyTo.(*client.InputMessageReplyToMessage)
		return ok && r == nil
	})).Return(&client.Message{Id: 500}, nil)
	stateRepo.EXPECT().SetCopiedMessageID(int64(100), int64(10), "r1:200:500").Return(nil)
	stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.Anything).Return(uint64(1), nil)
	stateRepo.EXPECT().IncrementForwardedMessages(int64(200), mock.Anything).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

// TestOnNewMessage_OriginChannel_Success — forwarded-from-channel разворачивается до оригинала.
func TestOnNewMessage_OriginChannel_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	tracker := mocks.NewDedupTracker(t)
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return tracker })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	rs.Destinations[200] = &domain.Destination{ChatID: 200}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         10,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "origin text"}},
	}
	msg.ForwardInfo = &client.MessageForwardInfo{
		Origin: &client.MessageOriginChannel{ChatId: 555, MessageId: 777},
	}
	origin := &client.Message{
		ChatId:     555,
		Id:         777,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "origin text"}},
	}
	transformed := &client.FormattedText{Text: "t"}

	messageService.EXPECT().IsSystemMessage(msg).Return(false)
	// GetFormattedText вызовется: 1) первый раз в OnNewMessage, 2) в getOriginMessage для origin, 3) в getOriginMessage для msg.
	messageService.EXPECT().GetFormattedText(msg).Return(&client.FormattedText{Text: "origin text"})
	messageService.EXPECT().GetFormattedText(origin).Return(&client.FormattedText{Text: "origin text"})
	messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte(nil))
	// BuildInputContent получает origin, не msg.
	messageService.EXPECT().BuildInputContent(origin, transformed).Return(&client.InputMessageText{Text: transformed})
	filterService.EXPECT().Evaluate("origin text", rule).Return(domain.FiltersOK)
	tracker.EXPECT().TryMark(int64(200)).Return(true)
	rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(200))
	telegramRepo.EXPECT().GetMessage(mock.MatchedBy(func(req *client.GetMessageRequest) bool {
		return req.ChatId == 555 && req.MessageId == 777
	})).Return(origin, nil)
	transformService.EXPECT().Transform(mock.Anything, mock.Anything).Return(transformed, nil)
	telegramRepo.EXPECT().SendMessage(mock.Anything).Return(&client.Message{Id: 500}, nil)
	stateRepo.EXPECT().SetCopiedMessageID(int64(100), int64(10), "r1:200:500").Return(nil)
	stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.Anything).Return(uint64(1), nil)
	stateRepo.EXPECT().IncrementForwardedMessages(int64(200), mock.Anything).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

// TestOnNewMessage_OriginChannel_TextMismatch — текст origin != текст msg, origin не используется.
func TestOnNewMessage_OriginChannel_TextMismatch(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	tracker := mocks.NewDedupTracker(t)
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return tracker })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	rs.Destinations[200] = &domain.Destination{ChatID: 200}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         10,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "fwd text"}},
	}
	msg.ForwardInfo = &client.MessageForwardInfo{
		Origin: &client.MessageOriginChannel{ChatId: 555, MessageId: 777},
	}
	origin := &client.Message{
		ChatId:     555,
		Id:         777,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "DIFFERENT"}},
	}
	transformed := &client.FormattedText{Text: "t"}

	messageService.EXPECT().IsSystemMessage(msg).Return(false)
	messageService.EXPECT().GetFormattedText(msg).Return(&client.FormattedText{Text: "fwd text"})
	messageService.EXPECT().GetFormattedText(origin).Return(&client.FormattedText{Text: "DIFFERENT"})
	messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte(nil))
	// fallback: используется msg, не origin.
	messageService.EXPECT().BuildInputContent(msg, transformed).Return(&client.InputMessageText{Text: transformed})
	filterService.EXPECT().Evaluate("fwd text", rule).Return(domain.FiltersOK)
	tracker.EXPECT().TryMark(int64(200)).Return(true)
	rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(200))
	telegramRepo.EXPECT().GetMessage(mock.Anything).Return(origin, nil)
	transformService.EXPECT().Transform(mock.Anything, mock.Anything).Return(transformed, nil)
	telegramRepo.EXPECT().SendMessage(mock.Anything).Return(&client.Message{Id: 500}, nil)
	stateRepo.EXPECT().SetCopiedMessageID(int64(100), int64(10), "r1:200:500").Return(nil)
	stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.Anything).Return(uint64(1), nil)
	stateRepo.EXPECT().IncrementForwardedMessages(int64(200), mock.Anything).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

// TestOnNewMessage_OriginChannel_GetMessageFail — ошибка GetMessage, origin отбрасывается.
func TestOnNewMessage_OriginChannel_GetMessageFail(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	tracker := mocks.NewDedupTracker(t)
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return tracker })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	rs.Destinations[200] = &domain.Destination{ChatID: 200}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         10,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "fwd text"}},
	}
	msg.ForwardInfo = &client.MessageForwardInfo{
		Origin: &client.MessageOriginChannel{ChatId: 555, MessageId: 777},
	}
	transformed := &client.FormattedText{Text: "t"}

	messageService.EXPECT().IsSystemMessage(msg).Return(false)
	messageService.EXPECT().GetFormattedText(msg).Return(&client.FormattedText{Text: "fwd text"})
	messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte(nil))
	messageService.EXPECT().BuildInputContent(msg, transformed).Return(&client.InputMessageText{Text: transformed})
	filterService.EXPECT().Evaluate("fwd text", rule).Return(domain.FiltersOK)
	tracker.EXPECT().TryMark(int64(200)).Return(true)
	rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(200))
	telegramRepo.EXPECT().GetMessage(mock.Anything).Return(nil, errors.New("not found"))
	transformService.EXPECT().Transform(mock.Anything, mock.Anything).Return(transformed, nil)
	telegramRepo.EXPECT().SendMessage(mock.Anything).Return(&client.Message{Id: 500}, nil)
	stateRepo.EXPECT().SetCopiedMessageID(int64(100), int64(10), "r1:200:500").Return(nil)
	stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.Anything).Return(uint64(1), nil)
	stateRepo.EXPECT().IncrementForwardedMessages(int64(200), mock.Anything).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

// TestOnNewMessage_OriginChannel_ZeroChatID — MessageOriginChannel.ChatId=0, origin не подставляется.
func TestOnNewMessage_OriginChannel_ZeroChatID(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	tracker := mocks.NewDedupTracker(t)
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return tracker })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	rs.Destinations[200] = &domain.Destination{ChatID: 200}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         10,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "fwd"}},
	}
	msg.ForwardInfo = &client.MessageForwardInfo{
		Origin: &client.MessageOriginChannel{ChatId: 0, MessageId: 777},
	}
	transformed := &client.FormattedText{Text: "t"}

	messageService.EXPECT().IsSystemMessage(msg).Return(false)
	messageService.EXPECT().GetFormattedText(msg).Return(&client.FormattedText{Text: "fwd"})
	messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte(nil))
	messageService.EXPECT().BuildInputContent(msg, transformed).Return(&client.InputMessageText{Text: transformed})
	filterService.EXPECT().Evaluate("fwd", rule).Return(domain.FiltersOK)
	tracker.EXPECT().TryMark(int64(200)).Return(true)
	rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(200))
	transformService.EXPECT().Transform(mock.Anything, mock.Anything).Return(transformed, nil)
	telegramRepo.EXPECT().SendMessage(mock.Anything).Return(&client.Message{Id: 500}, nil)
	stateRepo.EXPECT().SetCopiedMessageID(int64(100), int64(10), "r1:200:500").Return(nil)
	stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.Anything).Return(uint64(1), nil)
	stateRepo.EXPECT().IncrementForwardedMessages(int64(200), mock.Anything).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

// TestOnNewMessage_MediaAlbum_CopyMode — медиа-альбом с SendCopy реконструирует контент.
func TestOnNewMessage_MediaAlbum_CopyMode(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	tracker := mocks.NewDedupTracker(t)
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return tracker })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	rs.Destinations[200] = &domain.Destination{ChatID: 200}
	h.SetRuleSet(rs)

	msg1 := &client.Message{
		ChatId:     100,
		Id:         10,
		CanBeSaved: true,
		Content:    &client.MessagePhoto{Caption: &client.FormattedText{Text: "c1"}},
	}
	msg1.MediaAlbumId = client.JsonInt64(42)
	msg2 := &client.Message{
		ChatId:     100,
		Id:         11,
		CanBeSaved: true,
		Content:    &client.MessagePhoto{Caption: &client.FormattedText{Text: "c2"}},
	}
	msg2.MediaAlbumId = client.JsonInt64(42)

	txt1 := &client.FormattedText{Text: "c1"}
	txt2 := &client.FormattedText{Text: "c2"}
	content1 := &client.InputMessagePhoto{Caption: txt1}
	content2 := &client.InputMessagePhoto{Caption: txt2}

	messageService.EXPECT().IsSystemMessage(msg1).Return(false)
	messageService.EXPECT().GetFormattedText(msg1).Return(txt1) // в OnNewMessage
	albumService.EXPECT().AddMessage(mock.Anything, msg1).Return(true)

	// processMediaAlbum таск: LastReceivedAge>=3s сразу, потом Pop возвращает оба.
	albumService.EXPECT().LastReceivedAge(mock.Anything).Return(5 * time.Second)
	albumService.EXPECT().PopMessages(mock.Anything).Return([]*client.Message{msg2, msg1}) // не по порядку

	tracker.EXPECT().TryMark(int64(200)).Return(true)
	rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(200))

	// forwardAlbum copy mode — GetFormattedText + BuildInputContent на каждое сообщение.
	messageService.EXPECT().GetFormattedText(msg1).Return(txt1)
	messageService.EXPECT().GetFormattedText(msg2).Return(txt2)
	messageService.EXPECT().BuildInputContent(msg1, txt1).Return(content1)
	messageService.EXPECT().BuildInputContent(msg2, txt2).Return(content2)

	telegramRepo.EXPECT().SendMessageAlbum(mock.MatchedBy(func(req *client.SendMessageAlbumRequest) bool {
		return req.ChatId == 200 && len(req.InputMessageContents) == 2
	})).Return(&client.Messages{Messages: []*client.Message{{Id: 1000}, {Id: 1001}}}, nil)

	// SetCopiedMessageID для каждого (msg1 сортируется в начало — Id=10 < Id=11)
	stateRepo.EXPECT().SetCopiedMessageID(int64(100), int64(10), "r1:200:1000").Return(nil)
	stateRepo.EXPECT().SetCopiedMessageID(int64(100), int64(11), "r1:200:1001").Return(nil)

	stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.Anything).Return(uint64(1), nil)
	stateRepo.EXPECT().IncrementForwardedMessages(int64(200), mock.Anything).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg1)
	taskQueue.ProcessAll()
}

// TestOnNewMessage_MediaAlbum_NotFirst — второе сообщение в альбоме не запускает processMediaAlbum.
func TestOnNewMessage_MediaAlbum_NotFirst(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	tracker := mocks.NewDedupTracker(t)
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return tracker })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	rs.Destinations[200] = &domain.Destination{ChatID: 200}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         11,
		CanBeSaved: true,
		Content:    &client.MessagePhoto{Caption: &client.FormattedText{Text: "c2"}},
	}
	msg.MediaAlbumId = client.JsonInt64(42)

	messageService.EXPECT().IsSystemMessage(msg).Return(false)
	messageService.EXPECT().GetFormattedText(msg).Return(&client.FormattedText{Text: "c2"})
	albumService.EXPECT().AddMessage(mock.Anything, msg).Return(false) // not first

	// Act
	h.OnNewMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

// TestOnNewMessage_MediaAlbum_ForwardMode — медиа-альбом без SendCopy пересылается ForwardMessages.
func TestOnNewMessage_MediaAlbum_ForwardMode(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	tracker := mocks.NewDedupTracker(t)
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return tracker })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: false}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	h.SetRuleSet(rs)

	msg1 := &client.Message{
		ChatId:     100,
		Id:         10,
		CanBeSaved: true,
		Content:    &client.MessagePhoto{Caption: &client.FormattedText{Text: "c1"}},
	}
	msg1.MediaAlbumId = client.JsonInt64(42)
	msg2 := &client.Message{
		ChatId:     100,
		Id:         11,
		CanBeSaved: true,
		Content:    &client.MessagePhoto{Caption: &client.FormattedText{Text: "c2"}},
	}
	msg2.MediaAlbumId = client.JsonInt64(42)

	messageService.EXPECT().IsSystemMessage(msg1).Return(false)
	messageService.EXPECT().GetFormattedText(msg1).Return(&client.FormattedText{Text: "c1"})
	albumService.EXPECT().AddMessage(mock.Anything, msg1).Return(true)

	albumService.EXPECT().LastReceivedAge(mock.Anything).Return(5 * time.Second)
	albumService.EXPECT().PopMessages(mock.Anything).Return([]*client.Message{msg1, msg2})

	tracker.EXPECT().TryMark(int64(200)).Return(true)
	rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(200))
	telegramRepo.EXPECT().ForwardMessages(mock.MatchedBy(func(req *client.ForwardMessagesRequest) bool {
		return req.ChatId == 200 && req.FromChatId == 100 && len(req.MessageIds) == 2
	})).Return(&client.Messages{}, nil)

	stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.Anything).Return(uint64(1), nil)
	stateRepo.EXPECT().IncrementForwardedMessages(int64(200), mock.Anything).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg1)
	taskQueue.ProcessAll()
}

// TestOnNewMessage_MediaAlbum_ForwardMode_Fails — ошибка ForwardMessages в альбоме.
func TestOnNewMessage_MediaAlbum_ForwardMode_Fails(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	tracker := mocks.NewDedupTracker(t)
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return tracker })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: false}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	h.SetRuleSet(rs)

	msg1 := &client.Message{
		ChatId:     100,
		Id:         10,
		CanBeSaved: true,
		Content:    &client.MessagePhoto{Caption: &client.FormattedText{Text: "c1"}},
	}
	msg1.MediaAlbumId = client.JsonInt64(42)

	messageService.EXPECT().IsSystemMessage(msg1).Return(false)
	messageService.EXPECT().GetFormattedText(msg1).Return(&client.FormattedText{Text: "c1"})
	albumService.EXPECT().AddMessage(mock.Anything, msg1).Return(true)
	albumService.EXPECT().LastReceivedAge(mock.Anything).Return(5 * time.Second)
	albumService.EXPECT().PopMessages(mock.Anything).Return([]*client.Message{msg1})

	tracker.EXPECT().TryMark(int64(200)).Return(true)
	rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(200))
	telegramRepo.EXPECT().ForwardMessages(mock.Anything).Return(nil, errors.New("fail"))
	stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.Anything).Return(uint64(1), nil)
	stateRepo.EXPECT().IncrementForwardedMessages(int64(200), mock.Anything).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg1)
	taskQueue.ProcessAll()
}

// TestOnNewMessage_MediaAlbum_CopyMode_SendFails — SendMessageAlbum возвращает ошибку.
func TestOnNewMessage_MediaAlbum_CopyMode_SendFails(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	tracker := mocks.NewDedupTracker(t)
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return tracker })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	rs.Destinations[200] = &domain.Destination{ChatID: 200}
	h.SetRuleSet(rs)

	msg1 := &client.Message{
		ChatId:     100,
		Id:         10,
		CanBeSaved: true,
		Content:    &client.MessagePhoto{Caption: &client.FormattedText{Text: "c1"}},
	}
	msg1.MediaAlbumId = client.JsonInt64(42)
	txt1 := &client.FormattedText{Text: "c1"}

	messageService.EXPECT().IsSystemMessage(msg1).Return(false)
	messageService.EXPECT().GetFormattedText(msg1).Return(txt1)
	albumService.EXPECT().AddMessage(mock.Anything, msg1).Return(true)
	albumService.EXPECT().LastReceivedAge(mock.Anything).Return(5 * time.Second)
	albumService.EXPECT().PopMessages(mock.Anything).Return([]*client.Message{msg1})

	tracker.EXPECT().TryMark(int64(200)).Return(true)
	rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(200))
	messageService.EXPECT().GetFormattedText(msg1).Return(txt1)
	messageService.EXPECT().BuildInputContent(msg1, txt1).Return(&client.InputMessagePhoto{Caption: txt1})
	telegramRepo.EXPECT().SendMessageAlbum(mock.Anything).Return(nil, errors.New("album fail"))

	stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.Anything).Return(uint64(1), nil)
	stateRepo.EXPECT().IncrementForwardedMessages(int64(200), mock.Anything).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg1)
	taskQueue.ProcessAll()
}

// TestOnNewMessage_MediaAlbum_EmptyPop — PopMessages возвращает пусто, ранний выход.
func TestOnNewMessage_MediaAlbum_EmptyPop(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	h.SetRuleSet(rs)

	msg1 := &client.Message{
		ChatId:     100,
		Id:         10,
		CanBeSaved: true,
		Content:    &client.MessagePhoto{Caption: &client.FormattedText{Text: "c1"}},
	}
	msg1.MediaAlbumId = client.JsonInt64(42)

	messageService.EXPECT().IsSystemMessage(msg1).Return(false)
	messageService.EXPECT().GetFormattedText(msg1).Return(&client.FormattedText{Text: "c1"})
	albumService.EXPECT().AddMessage(mock.Anything, msg1).Return(true)
	albumService.EXPECT().LastReceivedAge(mock.Anything).Return(5 * time.Second)
	albumService.EXPECT().PopMessages(mock.Anything).Return(nil)

	// Act
	h.OnNewMessage(context.Background(), msg1)
	taskQueue.ProcessAll()
}

// TestOnNewMessage_MediaAlbum_ContextCancel — контекст отменяется во время ожидания альбома.
func TestOnNewMessage_MediaAlbum_ContextCancel(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	h.SetRuleSet(rs)

	msg1 := &client.Message{
		ChatId:     100,
		Id:         10,
		CanBeSaved: true,
		Content:    &client.MessagePhoto{Caption: &client.FormattedText{Text: "c1"}},
	}
	msg1.MediaAlbumId = client.JsonInt64(42)

	messageService.EXPECT().IsSystemMessage(msg1).Return(false)
	messageService.EXPECT().GetFormattedText(msg1).Return(&client.FormattedText{Text: "c1"})
	albumService.EXPECT().AddMessage(mock.Anything, msg1).Return(true)
	// age меньше 3s - цикл продолжится, но ctx.Done() сработает раньше таймера.
	albumService.EXPECT().LastReceivedAge(mock.Anything).Return(1 * time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // отменён до вызова

	// Act
	h.OnNewMessage(ctx, msg1)
	taskQueue.ProcessAll()
}

// TestOnDeletedMessages_NotPermanent — isPermanent=false пропускает обработку.
func TestOnDeletedMessages_NotPermanent(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) })

	// Act
	h.OnDeletedMessages(context.Background(), 100, []int64{1}, false)
	taskQueue.ProcessAll()
}

// TestOnDeletedMessages_NoRuleSet — nil ruleset, ничего не происходит.
func TestOnDeletedMessages_NoRuleSet(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) })

	// Act
	h.OnDeletedMessages(context.Background(), 100, []int64{1}, true)
	taskQueue.ProcessAll()
}

// TestOnDeletedMessages_UnknownSource — unknown chat, skip.
func TestOnDeletedMessages_UnknownSource(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) })

	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{"r1": &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}}},
		OrderedForwardRules: []string{"r1"},
		UniqueSources:       map[int64]struct{}{100: {}},
		UniqueDestinations:  map[int64]struct{}{200: {}},
	}
	h.SetRuleSet(rs)

	// Act
	h.OnDeletedMessages(context.Background(), 999, []int64{1}, true)
	taskQueue.ProcessAll()
}

// TestOnDeletedMessages_NoCopies — нет копий, пропуск.
func TestOnDeletedMessages_NoCopies(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) })

	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{"r1": &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}}},
		OrderedForwardRules: []string{"r1"},
		UniqueSources:       map[int64]struct{}{100: {}},
		UniqueDestinations:  map[int64]struct{}{200: {}},
	}
	h.SetRuleSet(rs)

	stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return(nil)

	// Act
	h.OnDeletedMessages(context.Background(), 100, []int64{1}, true)
	taskQueue.ProcessAll()
}

// TestOnDeletedMessages_BadCopyRef — парс копии не удался, пропуск + DeleteCopiedMessageIDs.
func TestOnDeletedMessages_BadCopyRef(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) })

	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{"r1": &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}}},
		OrderedForwardRules: []string{"r1"},
		UniqueSources:       map[int64]struct{}{100: {}},
		UniqueDestinations:  map[int64]struct{}{200: {}},
	}
	h.SetRuleSet(rs)

	stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"bad"})
	stateRepo.EXPECT().DeleteCopiedMessageIDs(int64(100), int64(1)).Return(errors.New("del fail"))

	// Act
	h.OnDeletedMessages(context.Background(), 100, []int64{1}, true)
	taskQueue.ProcessAll()
}

// TestOnDeletedMessages_RuleMissing — rule удалён, copy ref валидный, пропуск.
func TestOnDeletedMessages_RuleMissing(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) })

	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{"r1": &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}}},
		OrderedForwardRules: []string{"r1"},
		UniqueSources:       map[int64]struct{}{100: {}},
		UniqueDestinations:  map[int64]struct{}{200: {}},
	}
	h.SetRuleSet(rs)

	// Ссылка на несуществующее правило - ветка rule == nil (после Indelible).
	stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"missing:200:500"})
	stateRepo.EXPECT().GetNewMessageID(int64(200), int64(500)).Return(int64(600))
	telegramRepo.EXPECT().DeleteMessages(mock.Anything).Return(&client.Ok{}, errors.New("del fail"))
	stateRepo.EXPECT().DeleteNewMessageID(int64(200), int64(500)).Return(errors.New("err"))
	stateRepo.EXPECT().DeleteTmpMessageID(int64(200), int64(600)).Return(errors.New("err"))
	stateRepo.EXPECT().DeleteAnswerMessageID(int64(200), int64(500)).Return(errors.New("err"))
	stateRepo.EXPECT().DeleteCopiedMessageIDs(int64(100), int64(1)).Return(nil)

	// Act
	h.OnDeletedMessages(context.Background(), 100, []int64{1}, true)
	taskQueue.ProcessAll()
}

// TestOnEditedMessage_NoCopies — нет копий, ранний выход без retry.
func TestOnEditedMessage_NoCopies(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) })

	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{"r1": &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}}},
		OrderedForwardRules: []string{"r1"},
		UniqueSources:       map[int64]struct{}{100: {}},
		UniqueDestinations:  map[int64]struct{}{200: {}},
	}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         1,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "edit"}},
	}
	stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return(nil)

	// Act
	h.OnEditedMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

// TestOnEditedMessage_NoText — text=nil, ранний выход.
func TestOnEditedMessage_NoText(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) })

	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{"r1": &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}},
		OrderedForwardRules: []string{"r1"},
		UniqueSources:       map[int64]struct{}{100: {}},
		UniqueDestinations:  map[int64]struct{}{200: {}},
	}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         1,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: ""}},
	}
	stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"r1:200:500"})
	messageService.EXPECT().GetFormattedText(msg).Return(nil)

	// Act
	h.OnEditedMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

// TestOnEditedMessage_BadCopyRef — parseCopyRef fail, skip ref.
func TestOnEditedMessage_BadCopyRef(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) })

	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{"r1": &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}},
		OrderedForwardRules: []string{"r1"},
		UniqueSources:       map[int64]struct{}{100: {}},
		UniqueDestinations:  map[int64]struct{}{200: {}},
	}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         1,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "e"}},
	}
	stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"bad"})
	messageService.EXPECT().GetFormattedText(msg).Return(&client.FormattedText{Text: "e"})

	// Act
	h.OnEditedMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

// TestOnEditedMessage_RuleMissing — ref.ruleID нет в ForwardRules, пропуск.
func TestOnEditedMessage_RuleMissing(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) })

	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{"r1": &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}},
		OrderedForwardRules: []string{"r1"},
		UniqueSources:       map[int64]struct{}{100: {}},
		UniqueDestinations:  map[int64]struct{}{200: {}},
	}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         1,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "e"}},
	}
	stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"missing:200:500"})
	stateRepo.EXPECT().GetNewMessageID(int64(200), int64(500)).Return(int64(600))
	messageService.EXPECT().GetFormattedText(msg).Return(&client.FormattedText{Text: "e"})

	// Act
	h.OnEditedMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

// TestOnEditedMessage_TransformError — Transform вернул ошибку, пропуск.
func TestOnEditedMessage_TransformError(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	rs.Destinations[200] = &domain.Destination{ChatID: 200}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         1,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "e"}},
	}
	stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"r1:200:500"})
	stateRepo.EXPECT().GetNewMessageID(int64(200), int64(500)).Return(int64(600))
	messageService.EXPECT().GetFormattedText(msg).Return(&client.FormattedText{Text: "e"})
	messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte(nil))
	transformService.EXPECT().Transform(mock.Anything, mock.Anything).Return(nil, errors.New("transform fail"))

	// Act
	h.OnEditedMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

// TestOnEditedMessage_EditTextFails — EditMessageText возвращает ошибку, но продолжает обновлять answer state.
func TestOnEditedMessage_EditTextFails(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	rs.Destinations[200] = &domain.Destination{ChatID: 200}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         1,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "e"}},
	}
	transformed := &client.FormattedText{Text: "t"}
	stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"r1:200:500"})
	stateRepo.EXPECT().GetNewMessageID(int64(200), int64(500)).Return(int64(600))
	messageService.EXPECT().GetFormattedText(msg).Return(&client.FormattedText{Text: "e"})
	messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte(nil))
	transformService.EXPECT().Transform(mock.Anything, mock.Anything).Return(transformed, nil)
	telegramRepo.EXPECT().EditMessageText(mock.Anything).Return(nil, errors.New("edit fail"))
	stateRepo.EXPECT().DeleteAnswerMessageID(int64(200), int64(500)).Return(errors.New("del fail"))

	// Act
	h.OnEditedMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

// TestOnEditedMessage_EditCaptionFails — EditMessageCaption ошибка.
func TestOnEditedMessage_EditCaptionFails(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	rs.Destinations[200] = &domain.Destination{ChatID: 200}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         1,
		CanBeSaved: true,
		Content:    &client.MessagePhoto{Caption: &client.FormattedText{Text: "c"}},
	}
	transformed := &client.FormattedText{Text: "t"}
	stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"r1:200:500"})
	stateRepo.EXPECT().GetNewMessageID(int64(200), int64(500)).Return(int64(600))
	messageService.EXPECT().GetFormattedText(msg).Return(&client.FormattedText{Text: "c"})
	messageService.EXPECT().GetReplyMarkupData(msg).Return([]byte(nil))
	transformService.EXPECT().Transform(mock.Anything, mock.Anything).Return(transformed, nil)
	telegramRepo.EXPECT().EditMessageCaption(mock.Anything).Return(nil, errors.New("caption fail"))
	stateRepo.EXPECT().DeleteAnswerMessageID(int64(200), int64(500)).Return(nil)

	// Act
	h.OnEditedMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

// TestOnEditedMessage_RetryExhausted — все 3 попытки retry не удались.
func TestOnEditedMessage_RetryExhausted(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	rs.Destinations[200] = &domain.Destination{ChatID: 200}
	h.SetRuleSet(rs)

	msg := &client.Message{
		ChatId:     100,
		Id:         1,
		CanBeSaved: true,
		Content:    &client.MessageText{Text: &client.FormattedText{Text: "e"}},
	}
	// Все 4 вызова (attempt=0,1,2,3) возвращают newID=0.
	stateRepo.EXPECT().GetCopiedMessageIDs(int64(100), int64(1)).Return([]string{"r1:200:500"}).Times(4)
	stateRepo.EXPECT().GetNewMessageID(int64(200), int64(500)).Return(int64(0)).Times(4)
	messageService.EXPECT().GetFormattedText(msg).Return(&client.FormattedText{Text: "e"}).Times(4)

	// Act
	h.OnEditedMessage(context.Background(), msg)
	taskQueue.ProcessAll()
}

// TestOnMessageSendSucceeded_ErrorPaths — errors в SetNewMessageID и SetTmpMessageID логируются.
func TestOnMessageSendSucceeded_ErrorPaths(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) })

	stateRepo.EXPECT().SetNewMessageID(int64(200), int64(500), int64(600)).Return(errors.New("a"))
	stateRepo.EXPECT().SetTmpMessageID(int64(200), int64(600), int64(500)).Return(errors.New("b"))

	// Act
	h.OnMessageSendSucceeded(200, 500, 600)
	taskQueue.ProcessAll()
}

// TestSetRuleSet_Concurrent — SetRuleSet / loaded ruleset под гонкой.
func TestSetRuleSet_Concurrent(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) })

	// Act — 100 concurrent writers + 100 readers (через OnNewMessage с UnknownSource).
	done := make(chan struct{})
	for i := range 20 {
		go func(i int) {
			rs := &domain.RuleSet{
				Sources:             make(map[int64]*domain.Source),
				Destinations:        make(map[int64]*domain.Destination),
				ForwardRules:        map[string]*domain.ForwardRule{"r": &domain.ForwardRule{ID: "r", From: int64(100 + i), To: []int64{200}}},
				OrderedForwardRules: []string{"r"},
				UniqueSources:       map[int64]struct{}{int64(100 + i): {}},
				UniqueDestinations:  map[int64]struct{}{200: {}},
			}
			h.SetRuleSet(rs)
			done <- struct{}{}
		}(i)
	}
	for range 20 {
		go func() {
			h.OnNewMessage(context.Background(), &client.Message{ChatId: 999, Id: 1})
			taskQueue.ProcessAll()
			done <- struct{}{}
		}()
	}
	for range 40 {
		<-done
	}

	// Assert — не должно упасть под -race.
	assert.NotNil(t, h)
}

// TestOnNewMessage_MediaAlbum_RuleRemoved — rule вынимается из RuleSet перед pop-обработкой.
func TestOnNewMessage_MediaAlbum_RuleRemoved(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	h.SetRuleSet(rs)

	msg1 := &client.Message{
		ChatId:     100,
		Id:         10,
		CanBeSaved: true,
		Content:    &client.MessagePhoto{Caption: &client.FormattedText{Text: "c1"}},
	}
	msg1.MediaAlbumId = client.JsonInt64(42)

	messageService.EXPECT().IsSystemMessage(msg1).Return(false)
	messageService.EXPECT().GetFormattedText(msg1).Return(&client.FormattedText{Text: "c1"})
	albumService.EXPECT().AddMessage(mock.Anything, msg1).Return(true)
	albumService.EXPECT().LastReceivedAge(mock.Anything).RunAndReturn(func(_ domain.MediaAlbumKey) time.Duration {
		// удаляем правило перед Pop.
		delete(rs.ForwardRules, "r1")
		return 5 * time.Second
	})
	albumService.EXPECT().PopMessages(mock.Anything).Return([]*client.Message{msg1})

	// Act
	h.OnNewMessage(context.Background(), msg1)
	taskQueue.ProcessAll()
}

// TestOnNewMessage_MediaAlbum_DedupSkip — tracker.TryMark=false в альбоме.
func TestOnNewMessage_MediaAlbum_DedupSkip(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	tracker := mocks.NewDedupTracker(t)
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return tracker })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	h.SetRuleSet(rs)

	msg1 := &client.Message{
		ChatId:     100,
		Id:         10,
		CanBeSaved: true,
		Content:    &client.MessagePhoto{Caption: &client.FormattedText{Text: "c1"}},
	}
	msg1.MediaAlbumId = client.JsonInt64(42)

	messageService.EXPECT().IsSystemMessage(msg1).Return(false)
	messageService.EXPECT().GetFormattedText(msg1).Return(&client.FormattedText{Text: "c1"})
	albumService.EXPECT().AddMessage(mock.Anything, msg1).Return(true)
	albumService.EXPECT().LastReceivedAge(mock.Anything).Return(5 * time.Second)
	albumService.EXPECT().PopMessages(mock.Anything).Return([]*client.Message{msg1})
	tracker.EXPECT().TryMark(int64(200)).Return(false) // dedup skip

	// Статистика всё равно крутит.
	stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.Anything).Return(uint64(0), errors.New("view fail"))
	stateRepo.EXPECT().IncrementForwardedMessages(int64(200), mock.Anything).Return(uint64(0), errors.New("fwd fail"))

	// Act
	h.OnNewMessage(context.Background(), msg1)
	taskQueue.ProcessAll()
}

// TestOnNewMessage_MediaAlbum_SentMessagesMismatch — sent альбома содержит nil / больше элементов чем request.
func TestOnNewMessage_MediaAlbum_SentMessagesMismatch(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	tracker := mocks.NewDedupTracker(t)
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return tracker })

	rule := &domain.ForwardRule{ID: "r1", From: 100, To: []int64{200}, SendCopy: true}
	rs := &domain.RuleSet{
		Sources:             make(map[int64]*domain.Source),
		Destinations:        make(map[int64]*domain.Destination),
		ForwardRules:        map[string]*domain.ForwardRule{rule.ID: rule},
		OrderedForwardRules: []string{rule.ID},
		UniqueSources:       map[int64]struct{}{rule.From: {}},
		UniqueDestinations:  make(map[int64]struct{}),
	}
	rs.Sources[100] = &domain.Source{ChatID: 100}
	rs.Destinations[200] = &domain.Destination{ChatID: 200}
	h.SetRuleSet(rs)

	msg1 := &client.Message{
		ChatId:     100,
		Id:         10,
		CanBeSaved: true,
		Content:    &client.MessagePhoto{Caption: &client.FormattedText{Text: "c1"}},
	}
	msg1.MediaAlbumId = client.JsonInt64(42)
	msg2 := &client.Message{
		ChatId:     100,
		Id:         11,
		CanBeSaved: true,
		Content:    &client.MessagePhoto{Caption: &client.FormattedText{Text: "c2"}},
	}
	msg2.MediaAlbumId = client.JsonInt64(42)
	txt1 := &client.FormattedText{Text: "c1"}
	txt2 := &client.FormattedText{Text: "c2"}

	messageService.EXPECT().IsSystemMessage(msg1).Return(false)
	messageService.EXPECT().GetFormattedText(msg1).Return(txt1)
	albumService.EXPECT().AddMessage(mock.Anything, msg1).Return(true)
	albumService.EXPECT().LastReceivedAge(mock.Anything).Return(5 * time.Second)
	albumService.EXPECT().PopMessages(mock.Anything).Return([]*client.Message{msg1, msg2})

	tracker.EXPECT().TryMark(int64(200)).Return(true)
	rateLimiter.EXPECT().WaitForForward(mock.Anything, int64(200))
	messageService.EXPECT().GetFormattedText(msg1).Return(txt1)
	messageService.EXPECT().GetFormattedText(msg2).Return(txt2)
	messageService.EXPECT().BuildInputContent(msg1, txt1).Return(&client.InputMessagePhoto{Caption: txt1})
	messageService.EXPECT().BuildInputContent(msg2, txt2).Return(&client.InputMessagePhoto{Caption: txt2})

	// sent содержит nil на втором месте — ломает цикл (break при m==nil).
	telegramRepo.EXPECT().SendMessageAlbum(mock.Anything).Return(&client.Messages{
		Messages: []*client.Message{{Id: 1000}, nil},
	}, nil)
	stateRepo.EXPECT().SetCopiedMessageID(int64(100), int64(10), "r1:200:1000").Return(errors.New("fail"))

	stateRepo.EXPECT().IncrementViewedMessages(int64(200), mock.Anything).Return(uint64(1), nil)
	stateRepo.EXPECT().IncrementForwardedMessages(int64(200), mock.Anything).Return(uint64(1), nil)

	// Act
	h.OnNewMessage(context.Background(), msg1)
	taskQueue.ProcessAll()
}

// TestOnEditedMessage_RuleMissingInMap — ruleset пропадает между вызовами (паранойя).
func TestOnEditedMessage_RuleMissingInMap(t *testing.T) {
	t.Parallel()

	// Arrange
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	filterService := mocks.NewFilterService(t)
	transformService := mocks.NewTransformService(t)
	albumService := mocks.NewAlbumService(t)
	rateLimiter := mocks.NewRateLimiter(t)
	taskQueue := queue.New()
	h := New(telegramRepo, stateRepo, messageService, filterService,
		transformService, albumService, taskQueue, rateLimiter,
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) })

	// Act + Assert — nil ruleset не должен падать.
	require.NotPanics(t, func() {
		h.OnEditedMessage(context.Background(), &client.Message{
			ChatId:     100,
			Id:         1,
			CanBeSaved: true,
			Content:    &client.MessageText{Text: &client.FormattedText{Text: "e"}},
		})
	})
}
