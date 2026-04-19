package transform

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zelenin/go-tdlib/client"
)

func TestAddText_ValidMarkdown(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := &Service{logger: slog.Default()}
	base := &client.FormattedText{Text: "hi"}

	// Act
	result := svc.addText(context.Background(), base, "*bold*")

	// Assert
	require.NotNil(t, result)
	assert.Equal(t, "hi\n\nbold", result.Text)
	// addText добавляет bold entity к результату, offset сдвинут на lenUTF16("hi\n\n") = 4.
	require.Len(t, result.Entities, 1)
	assert.Equal(t, int32(4), result.Entities[0].Offset)
	assert.Equal(t, int32(4), result.Entities[0].Length)
	_, ok := result.Entities[0].Type.(*client.TextEntityTypeBold)
	assert.True(t, ok)
}

func TestAddText_FallbackOnParseError(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := &Service{logger: slog.Default()}
	base := &client.FormattedText{Text: "hi"}
	// Markdown v2 падает на незакрытом entity, например одиночном `*`.
	broken := "*unclosed"

	// Act
	result := svc.addText(context.Background(), base, broken)

	// Assert
	require.NotNil(t, result)
	// Fallback: добавлен как plain text.
	assert.Equal(t, "hi\n\n"+broken, result.Text)
	assert.Empty(t, result.Entities)
}

func TestAddText_PreservesOriginal(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := &Service{logger: slog.Default()}
	base := &client.FormattedText{
		Text: "hi",
		Entities: []*client.TextEntity{
			{Offset: 0, Length: 2, Type: &client.TextEntityTypeBold{}},
		},
	}

	// Act
	result := svc.addText(context.Background(), base, "tail")

	// Assert
	// Оригинал не мутирован.
	assert.Equal(t, "hi", base.Text)
	assert.Len(t, base.Entities, 1)
	assert.Equal(t, "hi\n\ntail", result.Text)
	// Первая entity — из оригинала, её offset/length сохранены.
	require.Len(t, result.Entities, 1)
	assert.Equal(t, int32(0), result.Entities[0].Offset)
	assert.Equal(t, int32(2), result.Entities[0].Length)
}
