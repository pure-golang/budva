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

func newTestService() (*Service, *mocks.TelegramGateway, *mocks.StateStore) {
	tg := &mocks.TelegramGateway{}
	st := &mocks.StateStore{}
	return New(tg, st), tg, st
}

func TestTransform_NoTransformations(t *testing.T) {
	t.Parallel()
	svc, _, _ := newTestService()

	p := domain.TransformParams{
		Text:   &domain.FormattedText{Text: "hello"},
		Source: &domain.Source{ChatID: 100},
	}
	result, err := svc.Transform(context.Background(), p)

	require.NoError(t, err)
	assert.Equal(t, "hello", result.Text)
}

func TestTransform_Translation(t *testing.T) {
	t.Parallel()
	svc, tg, _ := newTestService()

	src := &domain.Source{
		ChatID:    100,
		Translate: &domain.Translate{Lang: "ru", For: []int64{200}},
	}
	text := &domain.FormattedText{Text: "hello"}
	translated := &domain.FormattedText{Text: "привет"}

	tg.On("TranslateText", mock.Anything, text, "ru").Return(translated, nil)

	p := domain.TransformParams{
		Text:      text,
		Source:    src,
		DstChatID: 200,
	}
	result, err := svc.Transform(context.Background(), p)

	require.NoError(t, err)
	assert.Equal(t, "привет", result.Text)
}

func TestTransform_Translation_SkippedForOtherChat(t *testing.T) {
	t.Parallel()
	svc, _, _ := newTestService()

	src := &domain.Source{
		ChatID:    100,
		Translate: &domain.Translate{Lang: "ru", For: []int64{200}},
	}

	p := domain.TransformParams{
		Text:      &domain.FormattedText{Text: "hello"},
		Source:    src,
		DstChatID: 300, // не в списке For
	}
	result, err := svc.Transform(context.Background(), p)

	require.NoError(t, err)
	assert.Equal(t, "hello", result.Text)
}

func TestTransform_ReplaceFragments(t *testing.T) {
	t.Parallel()
	svc, _, _ := newTestService()

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
	result, err := svc.Transform(context.Background(), p)

	require.NoError(t, err)
	assert.Equal(t, "bar world", result.Text)
}

func TestTransform_Sign(t *testing.T) {
	t.Parallel()
	svc, tg, _ := newTestService()

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
	tg.On("ParseTextEntities", mock.Anything, "**Source**").Return(parsedSign, nil)

	p := domain.TransformParams{
		Text:        &domain.FormattedText{Text: "hello"},
		Source:      src,
		DstChatID:   200,
		WithSources: true,
	}
	result, err := svc.Transform(context.Background(), p)

	require.NoError(t, err)
	assert.Contains(t, result.Text, "Source")
}

func TestTransform_Link(t *testing.T) {
	t.Parallel()
	svc, tg, _ := newTestService()

	src := &domain.Source{
		ChatID: 100,
		Link:   &domain.Link{Title: "Orig", For: []int64{200}},
	}

	tg.On("GetMessageLink", mock.Anything, int64(100), int64(1)).Return("https://t.me/c/100/1", nil)
	parsed := &domain.FormattedText{
		Text: "Orig",
		Entities: []domain.TextEntity{
			{Offset: 0, Length: 4, Type: domain.TextEntityTextURL, URL: "https://t.me/c/100/1"},
		},
	}
	tg.On("ParseTextEntities", mock.Anything, "[Orig](https://t.me/c/100/1)").Return(parsed, nil)

	p := domain.TransformParams{
		Text:         &domain.FormattedText{Text: "hello"},
		Source:       src,
		DstChatID:    200,
		SrcChatID:    100,
		SrcMessageID: 1,
		WithSources:  true,
	}
	result, err := svc.Transform(context.Background(), p)

	require.NoError(t, err)
	assert.Contains(t, result.Text, "Orig")
}

func TestTransform_PrevLink(t *testing.T) {
	t.Parallel()
	svc, tg, _ := newTestService()

	src := &domain.Source{
		ChatID: 100,
		Prev:   &domain.Prev{Title: "Prev", For: []int64{200}},
	}

	tg.On("GetMessageLink", mock.Anything, int64(200), int64(50)).Return("https://t.me/c/200/50", nil)
	parsed := &domain.FormattedText{
		Text: "Prev",
		Entities: []domain.TextEntity{
			{Offset: 0, Length: 4, Type: domain.TextEntityTextURL, URL: "https://t.me/c/200/50"},
		},
	}
	tg.On("ParseTextEntities", mock.Anything, "[Prev](https://t.me/c/200/50)").Return(parsed, nil)

	p := domain.TransformParams{
		Text:          &domain.FormattedText{Text: "hello"},
		Source:        src,
		DstChatID:     200,
		PrevMessageID: 50,
		WithSources:   true,
	}
	result, err := svc.Transform(context.Background(), p)

	require.NoError(t, err)
	assert.Contains(t, result.Text, "Prev")
}

func TestAddNextLink(t *testing.T) {
	t.Parallel()
	svc, tg, _ := newTestService()

	src := &domain.Source{
		ChatID: 100,
		Next:   &domain.Next{Title: "Next", For: []int64{200}},
	}

	tg.On("GetMessageLink", mock.Anything, int64(200), int64(60)).Return("https://t.me/c/200/60", nil)
	parsed := &domain.FormattedText{
		Text: "Next",
		Entities: []domain.TextEntity{
			{Offset: 0, Length: 4, Type: domain.TextEntityTextURL, URL: "https://t.me/c/200/60"},
		},
	}
	tg.On("ParseTextEntities", mock.Anything, "[Next](https://t.me/c/200/60)").Return(parsed, nil)

	text := &domain.FormattedText{Text: "original"}
	result := svc.AddNextLink(context.Background(), text, src, 200, 60)

	assert.Contains(t, result.Text, "Next")
}

func TestAddNextLink_NoNextConfig(t *testing.T) {
	t.Parallel()
	svc, _, _ := newTestService()

	src := &domain.Source{ChatID: 100}
	text := &domain.FormattedText{Text: "original"}

	result := svc.AddNextLink(context.Background(), text, src, 200, 60)
	assert.Equal(t, "original", result.Text)
}

func TestAddNextLink_ChatNotInFor(t *testing.T) {
	t.Parallel()
	svc, _, _ := newTestService()

	src := &domain.Source{
		ChatID: 100,
		Next:   &domain.Next{Title: "Next", For: []int64{300}},
	}
	text := &domain.FormattedText{Text: "original"}

	result := svc.AddNextLink(context.Background(), text, src, 200, 60)
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

	text := &domain.FormattedText{Text: "foo bar foo"}
	fragment := &domain.ReplaceFragment{From: "foo", To: "baz"}

	result := replaceFragment(text, fragment)
	assert.Equal(t, "baz bar baz", result.Text)
}

func TestReplaceFragment_NoMatch(t *testing.T) {
	t.Parallel()

	text := &domain.FormattedText{Text: "hello"}
	fragment := &domain.ReplaceFragment{From: "xyz", To: "abc"}

	result := replaceFragment(text, fragment)
	assert.Equal(t, "hello", result.Text)
}

func TestReplaceFragment_NilText(t *testing.T) {
	t.Parallel()

	result := replaceFragment(nil, &domain.ReplaceFragment{From: "a", To: "b"})
	assert.Nil(t, result)
}
