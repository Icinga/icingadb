package utils

import (
	"strings"
	"unicode/utf8"
)

// TruncatePerfData truncates the string of performance data string to `lim` or less bytes, such that the performance data
// remains syntactically correct (till the last performance datum). It also reports whether the performance data
// was truncated.
func TruncatePerfData(str string, lim uint) (string, bool) {
	lenStr := len(str)
	limit := int(lim)

	if lenStr > limit {
		outString := str[:limit]

		tempCutLen := 255

		if lenStr < tempCutLen || limit < tempCutLen {
			tempCutLen = limit
		}
		sliceStart := limit - tempCutLen
		// obtain rightmost substring of length (limit - tempCutLen) in the truncated string
		tempLastCutString := outString[sliceStart:]
		ql := strings.LastIndex(tempLastCutString, "'")

		var ql2 int
		if ql > 0 {
			// performance data contains quoted labels
			ql2 = strings.LastIndex(tempLastCutString[:ql], "'")

			// tempLastCutString like "a=10 'b=" or "'a'=10 'b="
			if ql2 < 0 || (string(str[ql+sliceStart-1]) == " " && string(str[ql2+sliceStart+1]) == "=") {
				lSpaceIdx := strings.LastIndex(str[:ql+sliceStart], " ")
				outString = str[:lSpaceIdx]
			} else {
				// tempLastCutString like "'a'=10 'b'" or "'a'=10 'b'=" "'a=10 b'"
				lSpaceIdx := strings.LastIndex(str[:limit+1], " ")

				// for case like 'b'=" "'a=10 b'"
				if lSpaceIdx < ql+sliceStart {
					lSpaceIdx = strings.LastIndex(str[:ql2+sliceStart+1], " ")
				}
				outString = str[:lSpaceIdx]
			}
		} else {
			// unquoted performance data or tempLastCutString[0] = "'"
			lSpaceIdx := strings.LastIndex(str[:limit+1], " ")
			if lSpaceIdx < 0 {
				outString = ""
			} else {
				outString = str[:lSpaceIdx]
			}
		}
		return outString, true
	}
	return str, false
}

// TruncateText truncates the string of text data to `lim` or less bytes such that the last character remains a valid
// utf-8 character in the string. It also reports whether the text data was truncated.
func TruncateText(str string, lim uint) (string, bool) {
	truncated := false
	limit := int(lim)
	if len(str) >= limit {
		truncated = true
		str = str[:limit]
		for i := utf8.UTFMax; i > 0; i-- {
			r, _ := utf8.DecodeLastRuneInString(str)
			if r != utf8.RuneError {
				break
			}
			limit -= 1
			str = str[:limit]
		}
	}
	return str, truncated
}
