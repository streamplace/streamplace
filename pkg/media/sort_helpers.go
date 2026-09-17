package media

import (
	"sort"
	"strconv"
)

// sortNumericStrings sorts decimal-numeric strings in numeric order (0, 1, 2,
// 10) rather than lexical order (0, 1, 10, 2). Non-numeric strings fall back to
// lexical comparison. Used for track IDs and video pad names where lexical
// order breaks past single digits.
func sortNumericStrings(ss []string) {
	sort.Slice(ss, func(i, j int) bool {
		a, aErr := strconv.Atoi(ss[i])
		b, bErr := strconv.Atoi(ss[j])
		if aErr != nil || bErr != nil {
			return ss[i] < ss[j]
		}
		return a < b
	})
}

// numericLess compares two strings as integers when both parse, falling back to
// lexical order. For use with sort.Slice on strings that carry a numeric
// prefix (e.g. "video_0", "video_10") after the caller has stripped the prefix.
func numericLess(a, b string) bool {
	ai, aErr := strconv.Atoi(a)
	bi, bErr := strconv.Atoi(b)
	if aErr != nil || bErr != nil {
		return a < b
	}
	return ai < bi
}
