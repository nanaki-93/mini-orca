package theme

import (
	"strings"
	"testing"
)

func TestSanitizeAndTruncateVisibleWidth(t *testing.T) {
	if got := ClampWidth(40); got != 68 {
		t.Fatalf("expected ClampWidth(40)=68, got %d", got)
	}
	if got := ClampHeight(10); got != 20 {
		t.Fatalf("expected ClampHeight(10)=20, got %d", got)
	}
	code := SanitizeCodeText("a\tb\x1b[2Jc")
	if strings.Contains(code, "\x1b") || !strings.Contains(code, "\\x1b") {
		t.Fatalf("expected literal ESC to be escaped in code text, got %q", code)
	}
	padded := PadVisibleRight("\x1b[1mhello\x1b[0m", 10)
	if VisibleRuneCount(padded) != 10 {
		t.Fatalf("expected visible width 10, got %d", VisibleRuneCount(padded))
	}
}
