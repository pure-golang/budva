package transform

import (
	"math"
	"unicode/utf16"
)

// TDLib считает entity offset и length в code units UTF-16, поэтому все
// строковые манипуляции, затрагивающие entities, должны происходить
// через конвертацию в []uint16 и обратно.

// encodeUTF16 конвертирует строку в UTF-16 массив.
func encodeUTF16(s string) []uint16 {
	return utf16.Encode([]rune(s))
}

// decodeUTF16 конвертирует UTF-16 массив обратно в строку.
func decodeUTF16(u []uint16) string {
	return string(utf16.Decode(u))
}

// lenUTF16 возвращает длину UTF-16 массива как int32.
// Telegram ограничивает сообщения 4096 code units, поэтому int32 достаточно.
func lenUTF16(u []uint16) int32 {
	if len(u) > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(len(u)) //nolint:gosec // G115 FP: len clamped выше; Telegram лимит 4096 делает overflow невозможным.
}
