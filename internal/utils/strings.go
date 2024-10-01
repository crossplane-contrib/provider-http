package utils

import (
	"strings"
)

// NormalizeWhitespace removes extra whitespace from a string.
func NormalizeWhitespace(input string) string {
	return strings.Join(strings.Fields(input), " ")
}
