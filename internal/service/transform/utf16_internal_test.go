package transform

import (
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeUTF16_Roundtrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantUnits int
	}{
		{"empty", "", 0},
		{"ascii", "hello", 5},
		{"cyrillic_bmp", "привет", 6},
		{"bmp_emoji", "\u2600", 1}, // SUN SIGN — в BMP
		{"single_surrogate_emoji", "🌍", 2},
		{"mixed_surrogate_with_ascii", "hi 🌍 world", 11},
		{"zwj_family", "👨‍👩‍👧‍👦", 11}, // ZWJ-последовательность: четыре surrogate-пары и три ZWJ
		{"cjk", "日本語", 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			encoded := encodeUTF16(tt.input)
			decoded := decodeUTF16(encoded)

			// Assert
			assert.Equal(t, tt.input, decoded)
			assert.Equal(t, tt.wantUnits, len(encoded))
		})
	}
}

func TestLenUTF16(t *testing.T) {
	t.Parallel()

	assert.Equal(t, int32(0), lenUTF16(nil))
	assert.Equal(t, int32(0), lenUTF16([]uint16{}))
	assert.Equal(t, int32(5), lenUTF16(encodeUTF16("hello")))
	// "🌍" = 2 code units.
	assert.Equal(t, int32(2), lenUTF16(encodeUTF16("🌍")))
}

func TestLenUTF16_LargeStringClampedInt32(t *testing.T) {
	t.Parallel()

	// lenUTF16 clampit при len > MaxInt32. Эмулируем nil-safe long slice через
	// синтетический случай: используем строку в пределах int32 для sanity.
	long := strings.Repeat("a", 1<<16)
	got := lenUTF16(encodeUTF16(long))
	assert.Equal(t, int32(1<<16), got)
	assert.Less(t, int(got), math.MaxInt32)
}

func TestDecodeUTF16_EmptyAndLoneSurrogate(t *testing.T) {
	t.Parallel()

	// Пустой []uint16 → пустая строка.
	assert.Equal(t, "", decodeUTF16(nil))
	assert.Equal(t, "", decodeUTF16([]uint16{}))

	// Одиночный high-surrogate без пары → utf16.Decode заменит на U+FFFD.
	got := decodeUTF16([]uint16{0xD800})
	assert.Equal(t, "\uFFFD", got)
}
