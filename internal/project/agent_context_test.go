package project

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentRetrievalKeepsScopeAndProvenance(t *testing.T) {
	data := []byte("package p\nimport \"fmt\"\nfunc Run() { fmt.Println(\"selected\") }\nfunc Other() { println(\"private body\") }\n")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), data, 0600); err != nil {
		t.Fatal(err)
	}
	index, err := BuildIndex(root, "p", "r")
	if err != nil {
		t.Fatal(err)
	}
	policy, err := NewContextPolicy(root)
	if err != nil {
		t.Fatal(err)
	}
	scope := AgentScope{Path: "main.go", Hash: contentHash(data), StartLine: 3, EndLine: 3}
	reads := 0
	reader, err := NewAgentContext(index, []AgentScope{scope}, func(ctx context.Context, path, hash string) ([]byte, error) {
		reads++
		return policy.ReadSource(ctx, path, hash)
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []AgentQuery{{Path: "other.go"}, {Path: "../main.go"}, {Path: "main.go", StartLine: 2, EndLine: 4}} {
		result, err := reader.Retrieve(t.Context(), "read_snippet", query)
		if err != nil || result.Status != "scope_required" || reads != 0 || len(result.Evidence) != 0 {
			t.Fatalf("scope expanded: %+v %v", result, err)
		}
	}
	for _, kind := range []string{"read_snippet", "search_repository", "inspect_symbols", "inspect_imports", "inspect_impact"} {
		result, err := reader.Retrieve(t.Context(), kind, AgentQuery{Path: "main.go", Query: "selected"})
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range result.Evidence {
			if item.Path != scope.Path || item.Hash != scope.Hash || strings.Contains(item.Text, "private body") {
				t.Fatalf("leaked or unbound evidence: %+v", item)
			}
		}
		if kind == "read_snippet" && (len(result.Evidence) != 1 || result.Evidence[0].StartLine != 3) {
			t.Fatal("missing source span")
		}
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.Retrieve(t.Context(), "read_snippet", AgentQuery{Path: "main.go"}); !errors.Is(err, ErrRevisionConflict) {
		t.Fatal("stale source accepted", err)
	}
}

func TestAgentSourceRejectsExcludedLargeBinarySymlinkAndCanceledReads(t *testing.T) {
	root := t.TempDir()
	for name, data := range map[string][]byte{"main.go": []byte("package p\n"), ".env": []byte("credential"), "large.go": []byte(strings.Repeat("x", maxFileViewBytes+1)), "binary.go": {0, 1, 2}} {
		if err := os.WriteFile(filepath.Join(root, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	policy, err := NewContextPolicy(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{".env", "large.go", "binary.go", "../main.go"} {
		if _, err := policy.ReadSource(t.Context(), path, "hash"); err == nil {
			t.Fatalf("unsafe source accepted: %s", path)
		}
	}
	if err := os.Symlink(filepath.Join(root, "main.go"), filepath.Join(root, "alias.go")); err == nil {
		if _, err := policy.ReadSource(t.Context(), "alias.go", contentHash([]byte("package p\n"))); err == nil {
			t.Fatal("symlink admitted")
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := policy.ReadSource(ctx, "main.go", contentHash([]byte("package p\n"))); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestAgentEvidenceBoundsResultsAndText(t *testing.T) {
	result := AgentEvidenceResult{Evidence: []AgentEvidence{}}
	for range 25 {
		result.add(AgentScope{Path: "main.go"}, "search_repository", strings.Repeat("é", 1000), "exact")
	}
	bytes := 0
	for _, item := range result.Evidence {
		bytes += len(item.Text)
	}
	if !result.Truncated || len(result.Evidence) > 20 || bytes > 8192 {
		t.Fatalf("unbounded evidence: %d %d", len(result.Evidence), bytes)
	}
}
