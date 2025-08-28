package config

import "strings"

func ToSnakeCase(str string) string {
	out := ""
	runes := []rune(str)

	for i, c := range runes {
		ch := string(c)
		isUpper := strings.ToUpper(ch) == ch && strings.ToLower(ch) != ch

		if isUpper {
			// Add dash before uppercase letter if:
			// 1. It's not the first character AND
			// 2. Either the previous character is lowercase OR the next character is lowercase
			if i > 0 {
				prevIsLower := i > 0 && strings.ToLower(string(runes[i-1])) == string(runes[i-1])
				nextIsLower := i < len(runes)-1 && strings.ToLower(string(runes[i+1])) == string(runes[i+1])

				if prevIsLower || nextIsLower {
					out += "-"
				}
			}
			out += strings.ToLower(ch)
		} else {
			out += ch
		}
	}
	return out
}
