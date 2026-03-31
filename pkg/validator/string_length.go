package validator

import (
	"unicode"
)

// CalculateMultilingualLength calculates the weighted length of a string
// where ASCII characters count as 1 and CJK characters count as 2
func CalculateMultilingualLength(s string) int {
	length := 0
	for _, r := range s {
		if isCJK(r) {
			length += 2
		} else {
			length += 1
		}
	}
	return length
}

// isCJK checks if a rune is a CJK (Chinese, Japanese, Korean) character
func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) ||
		unicode.Is(unicode.Hangul, r)
}

// ValidateStringLength validates if a string's weighted length is within the limit
func ValidateStringLength(s string, maxLength int) bool {
	return CalculateMultilingualLength(s) <= maxLength
}
