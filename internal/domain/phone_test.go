package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaskPhoneNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		phone    string
		expected string
	}{
		{name: "standard_phone_number", phone: "+7 926 111 22 33", expected: "+7926*****33"},
		{name: "phone_number_without_spaces", phone: "+79261112233", expected: "+7926*****33"},
		{name: "phone_number_with_dashes", phone: "+7-926-111-22-33", expected: "+7926*****33"},
		{name: "short_phone_number", phone: "12345", expected: "***45"},
		{name: "very_short_number_2_chars", phone: "12", expected: "**"},
		{name: "very_short_number_1_char", phone: "1", expected: "**"},
		{name: "empty_number", phone: "", expected: "**"},
		{name: "international_format", phone: "+38 067 123 45 67", expected: "+38067*****67"},
		{name: "border_7_chars", phone: "1234567", expected: "*****67"},
		{name: "border_8_chars", phone: "12345678", expected: "1*****78"},
		{name: "minimal_masking_3_chars", phone: "123", expected: "*23"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := MaskPhoneNumber(test.phone)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestMaskPhoneNumber_PreservesLength(t *testing.T) {
	t.Parallel()

	phones := []string{"+79261234567", "12345", "1234567890"}
	for _, phone := range phones {
		cleanPhone := strings.ReplaceAll(strings.ReplaceAll(phone, " ", ""), "-", "")
		if len(cleanPhone) > 2 {
			result := MaskPhoneNumber(phone)
			assert.Equal(t, len(cleanPhone), len(result))
		}
	}
}
