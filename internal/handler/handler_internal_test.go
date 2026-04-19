package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/handler/mocks"
)

// newRunNextHandler — внутренний helper для прямого тестирования runNextLinkWorkflow.
// Живёт в internal-тесте т.к. метод приватный.
func newRunNextHandler(t *testing.T) (*Handler, *mocks.TelegramRepo, *mocks.StateRepo, *mocks.MessageService, *mocks.TransformService) {
	t.Helper()
	telegramRepo := mocks.NewTelegramRepo(t)
	stateRepo := mocks.NewStateRepo(t)
	messageService := mocks.NewMessageService(t)
	transformService := mocks.NewTransformService(t)
	h := New(
		telegramRepo,
		stateRepo,
		messageService,
		mocks.NewFilterService(t),
		transformService,
		mocks.NewAlbumService(t),
		&internalSyncQueue{},
		mocks.NewRateLimiter(t),
		func(_ []int64) DedupTracker { return mocks.NewDedupTracker(t) },
	)
	return h, telegramRepo, stateRepo, messageService, transformService
}

type internalSyncQueue struct{ tasks []func() }

func (q *internalSyncQueue) Add(fn func()) { q.tasks = append(q.tasks, fn) }

// TestRunNextLinkWorkflow_CtxDone — отменённый ctx приводит к немедленному return в первой итерации.
func TestRunNextLinkWorkflow_CtxDone(t *testing.T) {
	t.Parallel()

	// Arrange
	h, _, _, _, _ := newRunNextHandler(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Act — должен выйти сразу по ctx.Done.
	h.runNextLinkWorkflow(ctx, &domain.Source{ChatID: 100}, 200, 50, 51)

	// Assert — без моков: ни одной операции.
}

// TestRunNextLinkWorkflow_Success — покрывает успешный путь (GetMessage + AddNextLink + EditMessageText).
// Занимает ~1s из-за захардкоженного time.After(1*time.Second) в цикле.
func TestRunNextLinkWorkflow_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	h, telegramRepo, stateRepo, messageService, transformService := newRunNextHandler(t)
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
}

// TestRunNextLinkWorkflow_GetMessageFail — ошибка GetMessage останавливает loop.
func TestRunNextLinkWorkflow_GetMessageFail(t *testing.T) {
	t.Parallel()

	// Arrange
	h, telegramRepo, stateRepo, _, _ := newRunNextHandler(t)

	stateRepo.EXPECT().GetNewMessageID(int64(200), int64(51)).Return(int64(52))
	telegramRepo.EXPECT().GetMessage(mock.Anything).Return(nil, errors.New("not found"))

	// Act
	h.runNextLinkWorkflow(context.Background(), &domain.Source{ChatID: 100}, 200, 500, 51)
}

// TestRunNextLinkWorkflow_NilText — text == nil, return.
func TestRunNextLinkWorkflow_NilText(t *testing.T) {
	t.Parallel()

	// Arrange
	h, telegramRepo, stateRepo, messageService, _ := newRunNextHandler(t)

	prevMsg := &client.Message{ChatId: 200, Id: 500}
	stateRepo.EXPECT().GetNewMessageID(int64(200), int64(51)).Return(int64(52))
	telegramRepo.EXPECT().GetMessage(mock.Anything).Return(prevMsg, nil)
	messageService.EXPECT().GetFormattedText(prevMsg).Return(nil)

	// Act
	h.runNextLinkWorkflow(context.Background(), &domain.Source{ChatID: 100}, 200, 500, 51)
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
	h, telegramRepo, stateRepo, messageService, transformService := newRunNextHandler(t)
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
