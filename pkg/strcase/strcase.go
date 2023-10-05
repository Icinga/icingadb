package strcase

import (
	"strings"
	"unicode"
)

var delimiters = map[rune]any{' ': struct{}{}, '_': struct{}{}, '-': struct{}{}, '.': struct{}{}}

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

// convert converts a camelCase or space/underscore/hyphen/dot delimited string into various cases.
// _case must be unicode.Lower or unicode.Upper.
func convert(s string, _case int, d rune) string {
	if s == "" {
		return s
	}

	var wasLower bool
	var wasNumber bool

	s = strings.TrimSpace(s)

	n := strings.Builder{}
	n.Grow(len(s) + 2) // Allow adding at least 2 delimiters without another allocation.

	for _, r := range s {
		var isNumber bool
		delimiter, isDelimiter := delimiters[r]
		if !isDelimiter {
			isNumber = unicode.IsNumber(r)
			if wasNumber {
				if !isNumber {
					n.WriteRune(d)
				}
			} else if isNumber || wasLower && unicode.IsUpper(r) {
				n.WriteRune(d)
			}

			n.WriteRune(unicode.To(_case, r))
		} else if delimiter != d {
			n.WriteRune(d)
		}

		wasLower = unicode.IsLower(r)
		if !wasLower {
			wasNumber = isNumber
		}
	}

	return n.String()
}
