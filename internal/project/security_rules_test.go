package project

import (
	"strings"
	"testing"
)

func TestScanGoSecurityRulesRejectsUnsupportedAndStaleInput(t *testing.T) {
	source := "package fixture\nfunc Run() {}\n"
	indexed := securityRuleIndexedFile("main.go", source)
	indexed.Language = "Kotlin"
	if _, err := ScanGoSecurityRules(indexed, source); err != ErrSecurityRulesUnavailable {
		t.Fatalf("unsupported language = %v", err)
	}
	indexed = securityRuleIndexedFile("main.go", source)
	if _, err := ScanGoSecurityRules(indexed, source+"// changed\n"); err != ErrRevisionConflict {
		t.Fatalf("stale source = %v", err)
	}
}

func securityRuleIndexedFile(path, source string) IndexFile {
	_, symbols, _ := extractGoFacts(path, []byte(source))
	return IndexFile{Path: path, ContentHash: contentHash([]byte(source)), Language: "Go", LineCount: strings.Count(source, "\n"), Symbols: symbols}
}
