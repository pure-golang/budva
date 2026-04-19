package transform_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/service/transform"
	"github.com/pure-golang/budva-claude/internal/service/transform/mocks"
)

// Transform использует статическую client.ParseTextEntities — её не мокаем.
// Тесты подают текст, проходящий через реальный парсер Markdown v2.

func TestTransform_NoTransformations(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	p := domain.TransformParams{
		Text:   &client.FormattedText{Text: "hello"},
		Source: &domain.Source{ChatID: 100},
	}

	// Act
	result, err := svc.Transform(context.Background(), p)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "hello", result.Text)
}

func TestTransform_Translation(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{
		ChatID:    100,
		Translate: &domain.Translate{Lang: "ru", For: []int64{200}},
	}
	text := &client.FormattedText{Text: "hello"}
	translated := &client.FormattedText{Text: "привет"}
	tg.EXPECT().TranslateText(mock.MatchedBy(func(req *client.TranslateTextRequest) bool {
		return req.ToLanguageCode == "ru" && req.Text != nil && req.Text.Text == "hello"
	})).Return(translated, nil)

	p := domain.TransformParams{
		Text:      text,
		Source:    src,
		DstChatID: 200,
	}

	// Act
	result, err := svc.Transform(context.Background(), p)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "привет", result.Text)
}

func TestTransform_Translation_ErrorKeepsOriginal(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{
		ChatID:    100,
		Translate: &domain.Translate{Lang: "ru", For: []int64{200}},
	}
	tg.EXPECT().TranslateText(mock.Anything).Return(nil, errors.New("boom"))
	p := domain.TransformParams{
		Text:      &client.FormattedText{Text: "hello"},
		Source:    src,
		DstChatID: 200,
	}

	// Act
	result, err := svc.Transform(context.Background(), p)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "hello", result.Text)
}

func TestTransform_Translation_SkippedForOtherChat(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{
		ChatID:    100,
		Translate: &domain.Translate{Lang: "ru", For: []int64{200}},
	}
	p := domain.TransformParams{
		Text:      &client.FormattedText{Text: "hello"},
		Source:    src,
		DstChatID: 300,
	}

	// Act
	result, err := svc.Transform(context.Background(), p)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "hello", result.Text)
}

func TestTransform_ReplaceFragments(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{ChatID: 100}
	dst := &domain.Destination{
		ChatID: 200,
		ReplaceFragments: []*domain.ReplaceFragment{
			{From: "foo", To: "bar"},
			{From: "world", To: "there"},
		},
	}
	p := domain.TransformParams{
		Text:        &client.FormattedText{Text: "foo world"},
		Source:      src,
		Destination: dst,
		DstChatID:   200,
	}

	// Act
	result, err := svc.Transform(context.Background(), p)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "bar there", result.Text)
}

func TestTransform_Sign(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{
		ChatID: 100,
		Sign:   &domain.Sign{Title: "Source", For: []int64{200}},
	}
	p := domain.TransformParams{
		Text:        &client.FormattedText{Text: "hello"},
		Source:      src,
		DstChatID:   200,
		WithSources: true,
	}

	// Act
	result, err := svc.Transform(context.Background(), p)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, result.Text, "Source")
}

func TestTransform_Sign_SkippedWithoutWithSources(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{
		ChatID: 100,
		Sign:   &domain.Sign{Title: "Source", For: []int64{200}},
	}
	p := domain.TransformParams{
		Text:      &client.FormattedText{Text: "hello"},
		Source:    src,
		DstChatID: 200,
	}

	// Act
	result, err := svc.Transform(context.Background(), p)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "hello", result.Text)
}

func TestTransform_Link(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{
		ChatID: 100,
		Link:   &domain.Link{Title: "Orig", For: []int64{200}},
	}
	tg.EXPECT().GetMessageLink(mock.MatchedBy(func(req *client.GetMessageLinkRequest) bool {
		return req.ChatId == 100 && req.MessageId == 1
	})).Return(&client.MessageLink{Link: "https://t.me/c/100/1"}, nil)
	p := domain.TransformParams{
		Text:         &client.FormattedText{Text: "hello"},
		Source:       src,
		DstChatID:    200,
		SrcChatID:    100,
		SrcMessageID: 1,
		WithSources:  true,
	}

	// Act
	result, err := svc.Transform(context.Background(), p)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, result.Text, "Orig")
}

func TestTransform_Link_ErrorSkipped(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{
		ChatID: 100,
		Link:   &domain.Link{Title: "Orig", For: []int64{200}},
	}
	tg.EXPECT().GetMessageLink(mock.Anything).Return(nil, errors.New("boom"))
	p := domain.TransformParams{
		Text:         &client.FormattedText{Text: "hello"},
		Source:       src,
		DstChatID:    200,
		SrcChatID:    100,
		SrcMessageID: 1,
		WithSources:  true,
	}

	// Act
	result, err := svc.Transform(context.Background(), p)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "hello", result.Text)
}

func TestTransform_Link_EmptyLinkSkipped(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{
		ChatID: 100,
		Link:   &domain.Link{Title: "Orig", For: []int64{200}},
	}
	tg.EXPECT().GetMessageLink(mock.Anything).Return(&client.MessageLink{Link: ""}, nil)
	p := domain.TransformParams{
		Text:        &client.FormattedText{Text: "hello"},
		Source:      src,
		DstChatID:   200,
		WithSources: true,
	}

	// Act
	result, err := svc.Transform(context.Background(), p)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "hello", result.Text)
}

func TestTransform_PrevLink(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{
		ChatID: 100,
		Prev:   &domain.Prev{Title: "Prev", For: []int64{200}},
	}
	tg.EXPECT().GetMessageLink(mock.MatchedBy(func(req *client.GetMessageLinkRequest) bool {
		return req.ChatId == 200 && req.MessageId == 50
	})).Return(&client.MessageLink{Link: "https://t.me/c/200/50"}, nil)
	p := domain.TransformParams{
		Text:          &client.FormattedText{Text: "hello"},
		Source:        src,
		DstChatID:     200,
		PrevMessageID: 50,
		WithSources:   true,
	}

	// Act
	result, err := svc.Transform(context.Background(), p)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, result.Text, "Prev")
}

func TestTransform_PrevLink_ErrorSkipped(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{
		ChatID: 100,
		Prev:   &domain.Prev{Title: "Prev", For: []int64{200}},
	}
	tg.EXPECT().GetMessageLink(mock.Anything).Return(nil, errors.New("boom"))
	p := domain.TransformParams{
		Text:          &client.FormattedText{Text: "hello"},
		Source:        src,
		DstChatID:     200,
		PrevMessageID: 50,
		WithSources:   true,
	}

	// Act
	result, err := svc.Transform(context.Background(), p)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "hello", result.Text)
}

func TestTransform_AutoAnswer(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{ChatID: 100, AutoAnswer: true}
	tg.EXPECT().GetCallbackQueryAnswer(mock.MatchedBy(func(req *client.GetCallbackQueryAnswerRequest) bool {
		return req.ChatId == 100 && req.MessageId == 1
	})).Return(&client.CallbackQueryAnswer{Text: "Answered!"}, nil)
	p := domain.TransformParams{
		Text:         &client.FormattedText{Text: "hello"},
		Source:       src,
		SrcChatID:    100,
		SrcMessageID: 1,
		DstChatID:    200,
		ReplyMarkup:  []byte{0x01, 0x02},
	}

	// Act
	result, err := svc.Transform(context.Background(), p)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, result.Text, "Answered!")
}

func TestTransform_AutoAnswer_NoReplyMarkupSkipped(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{ChatID: 100, AutoAnswer: true}
	p := domain.TransformParams{
		Text:      &client.FormattedText{Text: "hello"},
		Source:    src,
		DstChatID: 200,
	}

	// Act
	result, err := svc.Transform(context.Background(), p)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "hello", result.Text)
}

func TestTransform_AutoAnswer_ErrorKeepsText(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{ChatID: 100, AutoAnswer: true}
	tg.EXPECT().GetCallbackQueryAnswer(mock.Anything).Return(nil, errors.New("boom"))
	p := domain.TransformParams{
		Text:        &client.FormattedText{Text: "hello"},
		Source:      src,
		DstChatID:   200,
		ReplyMarkup: []byte{0x01},
	}

	// Act
	result, err := svc.Transform(context.Background(), p)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "hello", result.Text)
}

func TestTransform_AutoAnswer_EmptyAnswerKeepsText(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{ChatID: 100, AutoAnswer: true}
	tg.EXPECT().GetCallbackQueryAnswer(mock.Anything).Return(&client.CallbackQueryAnswer{Text: ""}, nil)
	p := domain.TransformParams{
		Text:        &client.FormattedText{Text: "hello"},
		Source:      src,
		DstChatID:   200,
		ReplyMarkup: []byte{0x01},
	}

	// Act
	result, err := svc.Transform(context.Background(), p)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "hello", result.Text)
}

func TestTransform_AutoAnswer_NilAnswerKeepsText(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{ChatID: 100, AutoAnswer: true}
	tg.EXPECT().GetCallbackQueryAnswer(mock.Anything).Return(nil, nil)
	p := domain.TransformParams{
		Text:        &client.FormattedText{Text: "hello"},
		Source:      src,
		DstChatID:   200,
		ReplyMarkup: []byte{0x01},
	}

	// Act
	result, err := svc.Transform(context.Background(), p)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "hello", result.Text)
}

func TestTransform_ReplaceMyselfLinks_BasicGroupSkipped(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{ChatID: 100}
	dst := &domain.Destination{
		ChatID:             200,
		ReplaceMyselfLinks: &domain.ReplaceMyselfLinks{Run: true},
	}
	tg.EXPECT().GetChat(mock.Anything).Return(&client.Chat{
		Id:   100,
		Type: &client.ChatTypeBasicGroup{},
	}, nil)
	// entity URL нужен, чтобы войти в цикл: "hi https://t.me/c/100/5".
	text := &client.FormattedText{
		Text: "hi https://t.me/c/100/5",
		Entities: []*client.TextEntity{
			{Offset: 3, Length: 20, Type: &client.TextEntityTypeUrl{}},
		},
	}
	p := domain.TransformParams{
		Text:        text,
		Source:      src,
		Destination: dst,
		SrcChatID:   100,
		DstChatID:   200,
	}

	// Act
	result, err := svc.Transform(context.Background(), p)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "hi https://t.me/c/100/5", result.Text)
}

func TestTransform_ReplaceMyselfLinks_GetChatError(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{ChatID: 100}
	dst := &domain.Destination{
		ChatID:             200,
		ReplaceMyselfLinks: &domain.ReplaceMyselfLinks{Run: true},
	}
	tg.EXPECT().GetChat(mock.Anything).Return(nil, errors.New("boom"))
	text := &client.FormattedText{
		Text: "hi https://t.me/c/100/5",
		Entities: []*client.TextEntity{
			{Offset: 3, Length: 20, Type: &client.TextEntityTypeUrl{}},
		},
	}
	p := domain.TransformParams{
		Text:        text,
		Source:      src,
		Destination: dst,
		SrcChatID:   100,
		DstChatID:   200,
	}

	// Act
	result, err := svc.Transform(context.Background(), p)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "hi https://t.me/c/100/5", result.Text)
}

func TestTransform_ReplaceMyselfLinks_NoEntities(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{ChatID: 100}
	dst := &domain.Destination{
		ChatID:             200,
		ReplaceMyselfLinks: &domain.ReplaceMyselfLinks{Run: true},
	}
	text := &client.FormattedText{Text: "hello"}
	p := domain.TransformParams{
		Text:        text,
		Source:      src,
		Destination: dst,
		SrcChatID:   100,
		DstChatID:   200,
	}

	// Act
	result, err := svc.Transform(context.Background(), p)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "hello", result.Text)
}

func TestTransform_ReplaceMyselfLinks_TextURLEntityReplaced(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{ChatID: 100}
	dst := &domain.Destination{
		ChatID:             200,
		ReplaceMyselfLinks: &domain.ReplaceMyselfLinks{Run: true},
	}
	tg.EXPECT().GetChat(mock.Anything).Return(&client.Chat{
		Id:   100,
		Type: &client.ChatTypeSupergroup{},
	}, nil)
	tg.EXPECT().GetMessageLinkInfo(mock.MatchedBy(func(r *client.GetMessageLinkInfoRequest) bool {
		return r.Url == "https://t.me/c/100/5"
	})).Return(&client.MessageLinkInfo{
		ChatId:  100,
		Message: &client.Message{Id: 5},
	}, nil)
	// Одна копия подходящего формата "ruleID:dstChatID:tmpID".
	st.EXPECT().GetCopiedMessageIDs(int64(100), int64(5)).Return([]string{"r1:200:999"})
	st.EXPECT().GetNewMessageID(int64(200), int64(999)).Return(int64(777))
	tg.EXPECT().GetMessageLink(mock.MatchedBy(func(r *client.GetMessageLinkRequest) bool {
		return r.ChatId == 200 && r.MessageId == 777
	})).Return(&client.MessageLink{Link: "https://t.me/c/200/777"}, nil)

	text := &client.FormattedText{
		Text: "see here",
		Entities: []*client.TextEntity{
			{Offset: 4, Length: 4, Type: &client.TextEntityTypeTextUrl{Url: "https://t.me/c/100/5"}},
		},
	}
	p := domain.TransformParams{
		Text:        text,
		Source:      src,
		Destination: dst,
		SrcChatID:   100,
		DstChatID:   200,
	}

	// Act
	result, err := svc.Transform(context.Background(), p)

	// Assert
	require.NoError(t, err)
	require.Len(t, result.Entities, 1)
	textURL, ok := result.Entities[0].Type.(*client.TextEntityTypeTextUrl)
	require.True(t, ok)
	assert.Equal(t, "https://t.me/c/200/777", textURL.Url)
}

func TestTransform_ReplaceMyselfLinks_PlainURLReplaced(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{ChatID: 100}
	dst := &domain.Destination{
		ChatID:             200,
		ReplaceMyselfLinks: &domain.ReplaceMyselfLinks{Run: true},
	}
	url := "https://t.me/c/100/5"
	tg.EXPECT().GetChat(mock.Anything).Return(&client.Chat{
		Id:   100,
		Type: &client.ChatTypeSupergroup{},
	}, nil)
	tg.EXPECT().GetMessageLinkInfo(mock.Anything).Return(&client.MessageLinkInfo{
		ChatId:  100,
		Message: &client.Message{Id: 5},
	}, nil)
	st.EXPECT().GetCopiedMessageIDs(int64(100), int64(5)).Return([]string{"r1:200:999"})
	st.EXPECT().GetNewMessageID(int64(200), int64(999)).Return(int64(777))
	tg.EXPECT().GetMessageLink(mock.Anything).Return(&client.MessageLink{Link: "https://t.me/c/200/777"}, nil)

	text := &client.FormattedText{
		Text: url,
		Entities: []*client.TextEntity{
			{Offset: 0, Length: int32(len(url)), Type: &client.TextEntityTypeUrl{}}, //nolint:gosec // test-data URL фиксированной длины, overflow невозможен
		},
	}
	p := domain.TransformParams{
		Text:        text,
		Source:      src,
		Destination: dst,
		SrcChatID:   100,
		DstChatID:   200,
	}

	// Act
	result, err := svc.Transform(context.Background(), p)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "https://t.me/c/200/777", result.Text)
}

// TestTransform_ReplaceMyselfLinks_NegativeDstChatID проверяет корректный парсинг
// отрицательного dstChatID (супергруппы `-100…`) в findCopyLink: parts[1] должен
// совпадать с `-200` со знаком, минус не срезается.
func TestTransform_ReplaceMyselfLinks_NegativeDstChatID(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{ChatID: -100}
	dst := &domain.Destination{
		ChatID:             -200,
		ReplaceMyselfLinks: &domain.ReplaceMyselfLinks{Run: true},
	}
	url := "https://t.me/c/100/5"
	tg.EXPECT().GetChat(mock.Anything).Return(&client.Chat{
		Id:   -100,
		Type: &client.ChatTypeSupergroup{},
	}, nil)
	tg.EXPECT().GetMessageLinkInfo(mock.Anything).Return(&client.MessageLinkInfo{
		ChatId:  -100,
		Message: &client.Message{Id: 5},
	}, nil)
	st.EXPECT().GetCopiedMessageIDs(int64(-100), int64(5)).Return([]string{"r1:-200:999"})
	st.EXPECT().GetNewMessageID(int64(-200), int64(999)).Return(int64(777))
	tg.EXPECT().GetMessageLink(mock.MatchedBy(func(r *client.GetMessageLinkRequest) bool {
		return r.ChatId == -200 && r.MessageId == 777
	})).Return(&client.MessageLink{Link: "https://t.me/c/200/777"}, nil)

	text := &client.FormattedText{
		Text: url,
		Entities: []*client.TextEntity{
			{Offset: 0, Length: int32(len(url)), Type: &client.TextEntityTypeUrl{}}, //nolint:gosec // test-data URL фиксированной длины, overflow невозможен
		},
	}
	p := domain.TransformParams{
		Text:        text,
		Source:      src,
		Destination: dst,
		SrcChatID:   -100,
		DstChatID:   -200,
	}

	// Act
	result, err := svc.Transform(context.Background(), p)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "https://t.me/c/200/777", result.Text)
}

func TestTransform_ReplaceMyselfLinks_ExternalLinkDeleted(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{ChatID: 100}
	dst := &domain.Destination{
		ChatID: 200,
		ReplaceMyselfLinks: &domain.ReplaceMyselfLinks{
			Run:            true,
			DeleteExternal: true,
		},
	}
	url := "https://t.me/c/999/5"
	tg.EXPECT().GetChat(mock.Anything).Return(&client.Chat{
		Id:   100,
		Type: &client.ChatTypeSupergroup{},
	}, nil)
	tg.EXPECT().GetMessageLinkInfo(mock.Anything).Return(&client.MessageLinkInfo{
		ChatId:  999, // external chat
		Message: &client.Message{Id: 5},
	}, nil)

	text := &client.FormattedText{
		Text: url,
		Entities: []*client.TextEntity{
			{Offset: 0, Length: int32(len(url)), Type: &client.TextEntityTypeUrl{}}, //nolint:gosec // test-data URL фиксированной длины, overflow невозможен
		},
	}
	p := domain.TransformParams{
		Text:        text,
		Source:      src,
		Destination: dst,
		SrcChatID:   100,
		DstChatID:   200,
	}

	// Act
	result, err := svc.Transform(context.Background(), p)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, result.Text, domain.DeletedLink)
}

func TestTransform_ReplaceMyselfLinks_ExternalLinkWithoutDeleteKept(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{ChatID: 100}
	dst := &domain.Destination{
		ChatID:             200,
		ReplaceMyselfLinks: &domain.ReplaceMyselfLinks{Run: true},
	}
	url := "https://t.me/c/999/5"
	tg.EXPECT().GetChat(mock.Anything).Return(&client.Chat{
		Id:   100,
		Type: &client.ChatTypeSupergroup{},
	}, nil)
	tg.EXPECT().GetMessageLinkInfo(mock.Anything).Return(&client.MessageLinkInfo{
		ChatId: 999,
	}, nil)

	text := &client.FormattedText{
		Text: url,
		Entities: []*client.TextEntity{
			{Offset: 0, Length: int32(len(url)), Type: &client.TextEntityTypeUrl{}}, //nolint:gosec // test-data URL фиксированной длины, overflow невозможен
		},
	}
	p := domain.TransformParams{
		Text:        text,
		Source:      src,
		Destination: dst,
		SrcChatID:   100,
		DstChatID:   200,
	}

	// Act
	result, err := svc.Transform(context.Background(), p)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, url, result.Text)
}

func TestTransform_ReplaceMyselfLinks_LinkInfoErrorSkipped(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{ChatID: 100}
	dst := &domain.Destination{
		ChatID:             200,
		ReplaceMyselfLinks: &domain.ReplaceMyselfLinks{Run: true},
	}
	url := "https://t.me/c/100/5"
	tg.EXPECT().GetChat(mock.Anything).Return(&client.Chat{
		Id:   100,
		Type: &client.ChatTypeSupergroup{},
	}, nil)
	tg.EXPECT().GetMessageLinkInfo(mock.Anything).Return(nil, errors.New("boom"))

	text := &client.FormattedText{
		Text: url,
		Entities: []*client.TextEntity{
			{Offset: 0, Length: int32(len(url)), Type: &client.TextEntityTypeUrl{}}, //nolint:gosec // test-data URL фиксированной длины, overflow невозможен
		},
	}
	p := domain.TransformParams{
		Text:        text,
		Source:      src,
		Destination: dst,
		SrcChatID:   100,
		DstChatID:   200,
	}

	// Act
	result, err := svc.Transform(context.Background(), p)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, url, result.Text)
}

func TestTransform_ReplaceMyselfLinks_NonURLEntitySkipped(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{ChatID: 100}
	dst := &domain.Destination{
		ChatID:             200,
		ReplaceMyselfLinks: &domain.ReplaceMyselfLinks{Run: true},
	}
	tg.EXPECT().GetChat(mock.Anything).Return(&client.Chat{
		Id:   100,
		Type: &client.ChatTypeSupergroup{},
	}, nil)

	text := &client.FormattedText{
		Text: "bold text",
		Entities: []*client.TextEntity{
			{Offset: 0, Length: 4, Type: &client.TextEntityTypeBold{}},
		},
	}
	p := domain.TransformParams{
		Text:        text,
		Source:      src,
		Destination: dst,
		SrcChatID:   100,
		DstChatID:   200,
	}

	// Act
	result, err := svc.Transform(context.Background(), p)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "bold text", result.Text)
}

func TestTransform_ReplaceMyselfLinks_NoCopyFound(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{ChatID: 100}
	dst := &domain.Destination{
		ChatID:             200,
		ReplaceMyselfLinks: &domain.ReplaceMyselfLinks{Run: true},
	}
	url := "https://t.me/c/100/5"
	tg.EXPECT().GetChat(mock.Anything).Return(&client.Chat{
		Id:   100,
		Type: &client.ChatTypeSupergroup{},
	}, nil)
	tg.EXPECT().GetMessageLinkInfo(mock.Anything).Return(&client.MessageLinkInfo{
		ChatId:  100,
		Message: &client.Message{Id: 5},
	}, nil)
	// Покрываем все ветки findCopyLink: несовпадающий dstChatID, слишком мало частей,
	// GetNewMessageID=0, GetMessageLink-error, всё это приводит к пустой ссылке.
	// Реальный формат записи — "ruleID:dstChatID:tmpMsgID" (см. handler.go:352).
	st.EXPECT().GetCopiedMessageIDs(int64(100), int64(5)).Return([]string{
		"r1:300:111",     // dstChatID=300 не совпадает с 200
		"r1:2001",        // частей < 3
		"r1:200:222",     // GetNewMessageID=0
		"r1:200:333",     // GetMessageLink-error
		"r1:200:bad_num", // parseMessageID вернёт 0 → GetNewMessageID=0
	})
	st.EXPECT().GetNewMessageID(int64(200), int64(222)).Return(int64(0))
	st.EXPECT().GetNewMessageID(int64(200), int64(333)).Return(int64(444))
	st.EXPECT().GetNewMessageID(int64(200), int64(0)).Return(int64(0))
	tg.EXPECT().GetMessageLink(mock.MatchedBy(func(r *client.GetMessageLinkRequest) bool {
		return r.ChatId == 200 && r.MessageId == 444
	})).Return(nil, errors.New("boom"))

	text := &client.FormattedText{
		Text: url,
		Entities: []*client.TextEntity{
			{Offset: 0, Length: int32(len(url)), Type: &client.TextEntityTypeUrl{}}, //nolint:gosec // test-data URL фиксированной длины, overflow невозможен
		},
	}
	p := domain.TransformParams{
		Text:        text,
		Source:      src,
		Destination: dst,
		SrcChatID:   100,
		DstChatID:   200,
	}

	// Act
	result, err := svc.Transform(context.Background(), p)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, url, result.Text)
}

func TestTransform_ReplaceMyselfLinks_EmptyURL(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{ChatID: 100}
	dst := &domain.Destination{
		ChatID:             200,
		ReplaceMyselfLinks: &domain.ReplaceMyselfLinks{Run: true},
	}
	tg.EXPECT().GetChat(mock.Anything).Return(&client.Chat{
		Id:   100,
		Type: &client.ChatTypeSupergroup{},
	}, nil)

	// entity TextEntityTypeTextUrl с пустым Url → entityURL вернёт "" → continue.
	text := &client.FormattedText{
		Text: "link",
		Entities: []*client.TextEntity{
			{Offset: 0, Length: 4, Type: &client.TextEntityTypeTextUrl{Url: ""}},
		},
	}
	p := domain.TransformParams{
		Text:        text,
		Source:      src,
		Destination: dst,
		SrcChatID:   100,
		DstChatID:   200,
	}

	// Act
	result, err := svc.Transform(context.Background(), p)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "link", result.Text)
}

func TestAddNextLink(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{
		ChatID: 100,
		Next:   &domain.Next{Title: "Next", For: []int64{200}},
	}
	tg.EXPECT().GetMessageLink(mock.MatchedBy(func(req *client.GetMessageLinkRequest) bool {
		return req.ChatId == 200 && req.MessageId == 60
	})).Return(&client.MessageLink{Link: "https://t.me/c/200/60"}, nil)
	text := &client.FormattedText{Text: "original"}

	// Act
	result := svc.AddNextLink(context.Background(), text, src, 200, 60)

	// Assert
	assert.Contains(t, result.Text, "Next")
}

func TestAddNextLink_NoNextConfig(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{ChatID: 100}
	text := &client.FormattedText{Text: "original"}

	// Act
	result := svc.AddNextLink(context.Background(), text, src, 200, 60)

	// Assert
	assert.Equal(t, "original", result.Text)
}

func TestAddNextLink_ChatNotInFor(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{
		ChatID: 100,
		Next:   &domain.Next{Title: "Next", For: []int64{300}},
	}
	text := &client.FormattedText{Text: "original"}

	// Act
	result := svc.AddNextLink(context.Background(), text, src, 200, 60)

	// Assert
	assert.Equal(t, "original", result.Text)
}

func TestAddNextLink_LinkError(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{
		ChatID: 100,
		Next:   &domain.Next{Title: "Next", For: []int64{200}},
	}
	tg.EXPECT().GetMessageLink(mock.Anything).Return(nil, errors.New("boom"))
	text := &client.FormattedText{Text: "original"}

	// Act
	result := svc.AddNextLink(context.Background(), text, src, 200, 60)

	// Assert
	assert.Equal(t, "original", result.Text)
}

func TestAddNextLink_EmptyLink(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{
		ChatID: 100,
		Next:   &domain.Next{Title: "Next", For: []int64{200}},
	}
	tg.EXPECT().GetMessageLink(mock.Anything).Return(&client.MessageLink{Link: ""}, nil)
	text := &client.FormattedText{Text: "original"}

	// Act
	result := svc.AddNextLink(context.Background(), text, src, 200, 60)

	// Assert
	assert.Equal(t, "original", result.Text)
}

func TestAddNextLink_NilLink(t *testing.T) {
	t.Parallel()

	// Arrange
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	svc := transform.New(tg, st)
	src := &domain.Source{
		ChatID: 100,
		Next:   &domain.Next{Title: "Next", For: []int64{200}},
	}
	tg.EXPECT().GetMessageLink(mock.Anything).Return(nil, nil)
	text := &client.FormattedText{Text: "original"}

	// Act
	result := svc.AddNextLink(context.Background(), text, src, 200, 60)

	// Assert
	assert.Equal(t, "original", result.Text)
}
