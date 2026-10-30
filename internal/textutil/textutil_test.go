package textutil

import (
	"testing"
	"unicode/utf8"
)

func TestTruncateKeepsWholeCharacters(t *testing.T) {
	for _, test := range []struct {
		in   string
		n    int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello", 3, "hel"},
		{"hello", 0, ""},
		{"hello", -1, ""},
		{"héllo", 2, "h"},
		{"héllo", 3, "hé"},
		{"日本語", 4, "日"},
		{"日本語", 6, "日本"},
	} {
		got := Truncate(test.in, test.n)
		if got != test.want || !utf8.ValidString(got) {
			t.Errorf("Truncate(%q, %d) = %q, want %q", test.in, test.n, got, test.want)
		}
	}
}

func TestTruncateTailKeepsWholeCharacters(t *testing.T) {
	for _, test := range []struct {
		in   string
		n    int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello", 3, "llo"},
		{"hello", 0, ""},
		{"héllo", 4, "llo"},
		{"héllo", 5, "éllo"},
		{"日本語", 4, "語"},
	} {
		got := TruncateTail(test.in, test.n)
		if got != test.want || !utf8.ValidString(got) {
			t.Errorf("TruncateTail(%q, %d) = %q, want %q", test.in, test.n, got, test.want)
		}
	}
}
