package transform

// TDLib считает entity offset и length в code units UTF-16, поэтому все
// строковые манипуляции, затрагивающие entities, должны происходить
// через конвертацию в []uint16 и обратно.

// encodeUTF16 конвертирует строку в UTF-16 массив.
//
//nolint:gosec // Конвертация rune → uint16 безопасна для BMP и surrogate pairs.
func encodeUTF16(s string) []uint16 {
	var result []uint16
	for _, r := range s {
		if r >= 0x10000 {
			r -= 0x10000
			result = append(result, uint16(0xD800+(r>>10)), uint16(0xDC00+(r&0x3FF)))
		} else {
			result = append(result, uint16(r))
		}
	}
	return result
}

// decodeUTF16 конвертирует UTF-16 массив обратно в строку.
func decodeUTF16(u []uint16) string {
	var runes []rune
	for i := 0; i < len(u); i++ {
		if u[i] >= 0xD800 && u[i] <= 0xDBFF && i+1 < len(u) && u[i+1] >= 0xDC00 && u[i+1] <= 0xDFFF {
			r := rune((int(u[i])-0xD800)<<10 + int(u[i+1]) - 0xDC00 + 0x10000)
			runes = append(runes, r)
			i++
		} else {
			runes = append(runes, rune(u[i]))
		}
	}
	return string(runes)
}
