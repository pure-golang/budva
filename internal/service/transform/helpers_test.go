package transform

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/domain"
)

func TestDeepCopyFormattedText_Nil(t *testing.T) {
	t.Parallel()

	// Act
	result := deepCopyFormattedText(nil)

	// Assert
	assert.Nil(t, result)
}

func TestDeepCopyFormattedText_CopiesEntitiesIndependently(t *testing.T) {
	t.Parallel()

	// Arrange
	original := &client.FormattedText{
		Text: "hello",
		Entities: []*client.TextEntity{
			{Offset: 0, Length: 5, Type: &client.TextEntityTypeBold{}},
		},
	}

	// Act
	copied := deepCopyFormattedText(original)
	copied.Entities[0].Offset = 999

	// Assert
	require.NotNil(t, copied)
	assert.Equal(t, int32(0), original.Entities[0].Offset, "mutation must not leak back")
	assert.Equal(t, "hello", copied.Text)
}

func TestDeepCopyFormattedText_NilEntityEntrySkipped(t *testing.T) {
	t.Parallel()

	// Arrange
	original := &client.FormattedText{
		Text:     "x",
		Entities: []*client.TextEntity{nil, {Offset: 0, Length: 1, Type: &client.TextEntityTypeBold{}}},
	}

	// Act
	copied := deepCopyFormattedText(original)

	// Assert
	require.Len(t, copied.Entities, 2)
	assert.Nil(t, copied.Entities[0])
	require.NotNil(t, copied.Entities[1])
	assert.Equal(t, int32(1), copied.Entities[1].Length)
}

func TestEntityURL_Variants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		text   string
		entity *client.TextEntity
		want   string
	}{
		{
			name:   "url_entity_substring",
			text:   "see https://example.com here",
			entity: &client.TextEntity{Offset: 4, Length: 19, Type: &client.TextEntityTypeUrl{}},
			want:   "https://example.com",
		},
		{
			name:   "text_url_entity_from_type",
			text:   "click",
			entity: &client.TextEntity{Offset: 0, Length: 5, Type: &client.TextEntityTypeTextUrl{Url: "https://hidden.example"}},
			want:   "https://hidden.example",
		},
		{
			name:   "non_url_entity_empty",
			text:   "bold",
			entity: &client.TextEntity{Offset: 0, Length: 4, Type: &client.TextEntityTypeBold{}},
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := entityURL(tt.text, tt.entity)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsURLEntity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		entity *client.TextEntity
		want   bool
	}{
		{"url", &client.TextEntity{Type: &client.TextEntityTypeUrl{}}, true},
		{"text_url", &client.TextEntity{Type: &client.TextEntityTypeTextUrl{Url: "x"}}, true},
		{"bold_false", &client.TextEntity{Type: &client.TextEntityTypeBold{}}, false},
		{"mention_false", &client.TextEntity{Type: &client.TextEntityTypeMention{}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := isURLEntity(tt.entity)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReplaceFragmentInternal(t *testing.T) {
	t.Parallel()

	// Arrange
	text := &client.FormattedText{Text: "foo bar foo"}
	fragment := &domain.ReplaceFragment{From: "foo", To: "baz"}

	// Act
	result := replaceFragment(text, fragment)

	// Assert
	assert.Equal(t, "baz bar baz", result.Text)
	assert.Equal(t, "foo bar foo", text.Text, "original must be untouched")
}

func TestReplaceFragment_NilAndNoMatch(t *testing.T) {
	t.Parallel()

	nilResult := replaceFragment(nil, &domain.ReplaceFragment{From: "a", To: "b"})
	assert.Nil(t, nilResult)

	noMatch := &client.FormattedText{Text: "hello"}
	result := replaceFragment(noMatch, &domain.ReplaceFragment{From: "xyz", To: "!"})
	assert.Same(t, noMatch, result, "no-match путь возвращает тот же объект")
}

func TestContainsChatID(t *testing.T) {
	t.Parallel()

	assert.True(t, containsChatID([]int64{1, 2, 3}, 2))
	assert.False(t, containsChatID([]int64{1, 2, 3}, 99))
	assert.False(t, containsChatID(nil, 1))
}

func TestParseMessageID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, int64(123), parseMessageID("123"))
	assert.Equal(t, int64(-5), parseMessageID("-5"))
	assert.Equal(t, int64(0), parseMessageID("not_a_number"))
	assert.Equal(t, int64(0), parseMessageID(""))
}

func TestApplyReplacement_ShiftsEntities(t *testing.T) {
	t.Parallel()

	// Arrange
	// Текст "AAA BBB CCC" → заменим "BBB" (offset 4, length 3) на "XX".
	text := &client.FormattedText{
		Text: "AAA BBB CCC",
		Entities: []*client.TextEntity{
			{Offset: 0, Length: 3, Type: &client.TextEntityTypeBold{}},   // до замены
			{Offset: 4, Length: 3, Type: &client.TextEntityTypeItalic{}}, // совпадает по offset
			{Offset: 8, Length: 3, Type: &client.TextEntityTypeCode{}},   // после замены
		},
	}

	// Act
	result := applyReplacement(text, 4, 3, "XX")

	// Assert
	assert.Equal(t, "AAA XX CCC", result.Text)
	// Entity до — не изменилась.
	assert.Equal(t, int32(0), result.Entities[0].Offset)
	assert.Equal(t, int32(3), result.Entities[0].Length)
	// Entity по совпадающему offset — длина обновилась до длины новой строки.
	assert.Equal(t, int32(4), result.Entities[1].Offset)
	assert.Equal(t, int32(2), result.Entities[1].Length)
	// Entity после — offset сдвинулся на -1 (diff = 2-3).
	assert.Equal(t, int32(7), result.Entities[2].Offset)
}

func TestApplyReplacement_InsertLonger(t *testing.T) {
	t.Parallel()

	// Arrange
	text := &client.FormattedText{
		Text: "abc",
		Entities: []*client.TextEntity{
			{Offset: 2, Length: 1, Type: &client.TextEntityTypeBold{}},
		},
	}

	// Act
	result := applyReplacement(text, 1, 1, "ZZZ")

	// Assert
	assert.Equal(t, "aZZZc", result.Text)
	// Offset 2 > 1 → сдвигается на +2.
	assert.Equal(t, int32(4), result.Entities[0].Offset)
}

func TestExtractSubstringInternal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		text   string
		offset int32
		length int32
		want   string
	}{
		{"ascii", "hello world", 6, 5, "world"},
		{"empty_length", "hello", 0, 0, ""},
		{"beyond_length", "hi", 0, 100, ""},
		{"cyrillic_bmp", "привет мир", 7, 3, "мир"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := extractSubstring(tt.text, tt.offset, tt.length)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractSubstring_SurrogatePair(t *testing.T) {
	t.Parallel()

	// Эмоджи "🌍" = 1 rune, но 2 UTF-16 code units.
	// "hi 🌍" → h(0) i(1) space(2) surrogate_hi(3) surrogate_lo(4).
	got := extractSubstring("hi 🌍", 3, 2)
	assert.Equal(t, "🌍", got)
}
