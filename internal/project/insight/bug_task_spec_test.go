package insight

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSanitizeFindingTextKeepsWholeCharacters(t *testing.T) {
	if text := SanitizeFindingText(strings.Repeat("é", 100), 52); !utf8.ValidString(text) || !strings.HasSuffix(text, "[truncated]") {
		t.Fatalf("truncated finding text = %q", text)
	}
}
