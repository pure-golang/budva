package domain

import "strings"

// MaskPhoneNumber маскирует номер телефона, заменяя 5 цифр перед последними двумя.
// Например, "+7 926 111 22 33" становится "+7926*****33".
func MaskPhoneNumber(phone string) string {
	cleanPhone := strings.ReplaceAll(strings.ReplaceAll(phone, " ", ""), "-", "")

	const maskedCount = 5
	const visibleSuffixCount = 2

	if len(cleanPhone) <= maskedCount+visibleSuffixCount {
		if len(cleanPhone) <= visibleSuffixCount {
			return "**"
		}
		visibleSuffix := cleanPhone[len(cleanPhone)-visibleSuffixCount:]
		maskLength := len(cleanPhone) - visibleSuffixCount
		return strings.Repeat("*", maskLength) + visibleSuffix
	}

	prefixLength := len(cleanPhone) - maskedCount - visibleSuffixCount
	visiblePrefix := cleanPhone[:prefixLength]
	visibleSuffix := cleanPhone[len(cleanPhone)-visibleSuffixCount:]
	mask := strings.Repeat("*", maskedCount)

	return visiblePrefix + mask + visibleSuffix
}
