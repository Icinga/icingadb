// Package strcase implements functions to convert a camelCase UTF-8 string into various cases.
//
// New delimiters will be inserted based on the following transitions:
//   - On any change from lowercase to uppercase letter.
//   - On any change from number to uppercase letter.
package strcase

import (
	"strings"
	"unicode"
)

// Delimited converts a string to delimited.lower.case, here using `.` as delimiter.
func Delimited(s string, d rune) string {
	return convert(s, unicode.LowerCase, d)
}

// ScreamingDelimited converts a string to DELIMITED.UPPER.CASE, here using `.` as delimiter.
func ScreamingDelimited(s string, d rune) string {
	return convert(s, unicode.UpperCase, d)
}

// Snake converts a string to snake_case.
func Snake(s string) string {
	return Delimited(s, '_')
}

// ScreamingSnake converts a string to SCREAMING_SNAKE_CASE.
func ScreamingSnake(s string) string {
	return ScreamingDelimited(s, '_')
}

// convert converts a camelCase UTF-8 string into various cases.
// _case must be unicode.LowerCase or unicode.UpperCase.
func convert(s string, _case int, d rune) string {
	if len(s) == 0 {
		return s
	}

	n := strings.Builder{}
	n.Grow(len(s) + 2) // Allow adding at least 2 delimiters without another allocation.

	var prevRune rune

	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) && (unicode.IsNumber(prevRune) || unicode.IsLower(prevRune)) {
			n.WriteRune(d)
		}

		n.WriteRune(unicode.To(_case, r))

		prevRune = r
	}

	return n.String()
}
