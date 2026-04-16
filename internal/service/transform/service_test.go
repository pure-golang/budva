package transform

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/service/transform/mocks"
)

func newTestService(t *testing.T) (*Service, *mocks.TelegramRepo, *mocks.StateRepo) {
	t.Helper()
	tg := mocks.NewTelegramRepo(t)
	st := mocks.NewStateRepo(t)
	return New(tg, st), tg, st
}

func TestTransform_NoTransformations(t *testing.T) {
	t.Parallel()

	// Arrange
	svc, _, _ := newTestService(t)
	p := domain.TransformParams{
		Text:   &domain.FormattedText{Text: "hello"},
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
	text := &domain.FormattedText{Text: "hello"}
	translated := &domain.FormattedText{Text: "привет"}
	tg.EXPECT().TranslateText(mock.Anything, text, "ru").Return(translated, nil)

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
		Text:      &domain.FormattedText{Text: "hello"},
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
		Text:        &domain.FormattedText{Text: "foo world"},
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
	svc, tg, _ := newTestService(t)
	src := &domain.Source{
		ChatID: 100,
		Sign:   &domain.Sign{Title: "Source", For: []int64{200}},
	}
	parsedSign := &domain.FormattedText{
		Text: "**Source**",
		Entities: []domain.TextEntity{
			{Offset: 0, Length: 10, Type: domain.TextEntityBold},
		},
	}
	tg.EXPECT().ParseTextEntities(mock.Anything, "**Source**").Return(parsedSign, nil)
	p := domain.TransformParams{
		Text:        &domain.FormattedText{Text: "hello"},
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
	tg.EXPECT().GetMessageLink(mock.Anything, int64(100), int64(1)).Return("https://t.me/c/100/1", nil)
	parsed := &domain.FormattedText{
		Text: "Orig",
		Entities: []domain.TextEntity{
			{Offset: 0, Length: 4, Type: domain.TextEntityTextURL, URL: "https://t.me/c/100/1"},
		},
	}
	tg.EXPECT().ParseTextEntities(mock.Anything, "[Orig](https://t.me/c/100/1)").Return(parsed, nil)
	p := domain.TransformParams{
		Text:         &domain.FormattedText{Text: "hello"},
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
	tg.EXPECT().GetMessageLink(mock.Anything, int64(200), int64(50)).Return("https://t.me/c/200/50", nil)
	parsed := &domain.FormattedText{
		Text: "Prev",
		Entities: []domain.TextEntity{
			{Offset: 0, Length: 4, Type: domain.TextEntityTextURL, URL: "https://t.me/c/200/50"},
		},
	}
	tg.EXPECT().ParseTextEntities(mock.Anything, "[Prev](https://t.me/c/200/50)").Return(parsed, nil)
	p := domain.TransformParams{
		Text:          &domain.FormattedText{Text: "hello"},
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
	tg.EXPECT().GetMessageLink(mock.Anything, int64(200), int64(60)).Return("https://t.me/c/200/60", nil)
	parsed := &domain.FormattedText{
		Text: "Next",
		Entities: []domain.TextEntity{
			{Offset: 0, Length: 4, Type: domain.TextEntityTextURL, URL: "https://t.me/c/200/60"},
		},
	}
	tg.EXPECT().ParseTextEntities(mock.Anything, "[Next](https://t.me/c/200/60)").Return(parsed, nil)
	text := &domain.FormattedText{Text: "original"}

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
	text := &domain.FormattedText{Text: "original"}

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
	text := &domain.FormattedText{Text: "original"}

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

			// Act
			encoded := encodeUTF16(tt.input)
			decoded := decodeUTF16(encoded)

			// Assert
			assert.Equal(t, tt.input, decoded)
		})
	}
}

func TestExtractSubstring(t *testing.T) {
	t.Parallel()

	// Act
	result := extractSubstring("hello world", 6, 5)

	// Assert
	assert.Equal(t, "world", result)
}

func TestExtractSubstring_BeyondLength(t *testing.T) {
	t.Parallel()

	// Act
	result := extractSubstring("hi", 0, 100)

	// Assert
	assert.Equal(t, "", result)
}

func TestReplaceFragment(t *testing.T) {
	t.Parallel()

	// Arrange
	text := &domain.FormattedText{Text: "foo bar foo"}
	fragment := &domain.ReplaceFragment{From: "foo", To: "baz"}

	// Act
	result := replaceFragment(text, fragment)

	// Assert
	assert.Equal(t, "baz bar baz", result.Text)
}

func TestReplaceFragment_NoMatch(t *testing.T) {
	t.Parallel()

	// Arrange
	text := &domain.FormattedText{Text: "hello"}
	fragment := &domain.ReplaceFragment{From: "xyz", To: "abc"}

	// Act
	result := replaceFragment(text, fragment)

	// Assert
	assert.Equal(t, "hello", result.Text)
}

func TestReplaceFragment_NilText(t *testing.T) {
	t.Parallel()

	// Act
	result := replaceFragment(nil, &domain.ReplaceFragment{From: "a", To: "b"})

	// Assert
	assert.Nil(t, result)
}
