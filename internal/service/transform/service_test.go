package transform

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/service/transform/mocks"
)

func newTestService(t *testing.T) (*Service, *mocks.TelegramRepo, *mocks.StateRepo) {
	t.Helper()
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	return New(tg, st), tg, st
}

// Transform использует статическую client.ParseTextEntities — её не мокаем.
// Тесты подают текст, проходящий через реальный парсер Markdown v2.

func TestTransform_NoTransformations(t *testing.T) {
	t.Parallel()

	// Arrange
	svc, _, _ := newTestService(t)
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
	svc, tg, _ := newTestService(t)
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

func TestTransform_Translation_SkippedForOtherChat(t *testing.T) {
	t.Parallel()

	// Arrange
	svc, _, _ := newTestService(t)
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
	svc, _, _ := newTestService(t)
	src := &domain.Source{ChatID: 100}
	dst := &domain.Destination{
		ChatID: 200,
		ReplaceFragments: []*domain.ReplaceFragment{
			{From: "foo", To: "bar"},
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
	assert.Equal(t, "bar world", result.Text)
}

func TestTransform_Sign(t *testing.T) {
	t.Parallel()

	// Arrange
	svc, _, _ := newTestService(t)
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

func TestTransform_Link(t *testing.T) {
	t.Parallel()

	// Arrange
	svc, tg, _ := newTestService(t)
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

func TestTransform_PrevLink(t *testing.T) {
	t.Parallel()

	// Arrange
	svc, tg, _ := newTestService(t)
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

func TestAddNextLink(t *testing.T) {
	t.Parallel()

	// Arrange
	svc, tg, _ := newTestService(t)
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
	svc, _, _ := newTestService(t)
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
	svc, _, _ := newTestService(t)
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

func TestEncodeDecodeUTF16(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{"ascii", "hello"},
		{"cyrillic", "привет"},
		{"emoji", "hello 🌍 world"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			encoded := encodeUTF16(tt.input)
			decoded := decodeUTF16(encoded)
			assert.Equal(t, tt.input, decoded)
		})
	}
}

func TestExtractSubstring(t *testing.T) {
	t.Parallel()
	result := extractSubstring("hello world", 6, 5)
	assert.Equal(t, "world", result)
}

func TestExtractSubstring_BeyondLength(t *testing.T) {
	t.Parallel()
	result := extractSubstring("hi", 0, 100)
	assert.Equal(t, "", result)
}

func TestReplaceFragment(t *testing.T) {
	t.Parallel()
	text := &client.FormattedText{Text: "foo bar foo"}
	fragment := &domain.ReplaceFragment{From: "foo", To: "baz"}
	result := replaceFragment(text, fragment)
	assert.Equal(t, "baz bar baz", result.Text)
}

func TestReplaceFragment_NoMatch(t *testing.T) {
	t.Parallel()
	text := &client.FormattedText{Text: "hello"}
	fragment := &domain.ReplaceFragment{From: "xyz", To: "abc"}
	result := replaceFragment(text, fragment)
	assert.Equal(t, "hello", result.Text)
}

func TestReplaceFragment_NilText(t *testing.T) {
	t.Parallel()
	result := replaceFragment(nil, &domain.ReplaceFragment{From: "a", To: "b"})
	assert.Nil(t, result)
}
