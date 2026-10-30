// Package textutil holds dependency-free text helpers shared across layers.
package textutil

import "unicode/utf8"

// Truncate returns at most the first n bytes of s, cut at a character
// boundary so the result stays valid UTF-8 when s is.
func Truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// TruncateTail returns at most the last n bytes of s, starting at a character
// boundary so the result stays valid UTF-8 when s is.
func TruncateTail(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	start := len(s) - n
	for start < len(s) && !utf8.RuneStart(s[start]) {
		start++
	}
	return s[start:]
}
