package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstructionsResolveApplicableGuidesAndIdentity(t *testing.T) {
	root := t.TempDir()
	for path, content := range map[string]string{"AGENTS.md": "Use precise names.", "internal/AGENTS.md": "Test error paths.", "internal/worker/AGENTS.md": "Keep cancellation.", "other/AGENTS.md": "Unrelated."} {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	result, err := ResolveInstructions(root, "internal/worker/new.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 3 || result.Files[0].Path != "AGENTS.md" || result.Files[2].Scope != "internal/worker" || strings.Contains(result.Text, "Unrelated") {
		t.Fatalf("instructions: %+v", result)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("Changed guidance."), 0644); err != nil {
		t.Fatal(err)
	}
	changed, err := ResolveInstructions(root, "internal/worker/new.go")
	if err != nil || changed.Fingerprint == result.Fingerprint {
		t.Fatalf("identity did not change: %v", err)
	}
}

func TestInstructionsRespectPolicyAndRejectUnsafeInput(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("Private."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".mini-orca"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".mini-orca/context-policy.json"), []byte(`{"exclude":["AGENTS.md"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := ResolveInstructions(root, "new.go")
	if err != nil || len(result.Files) != 0 || len(result.Excluded) != 1 {
		t.Fatalf("excluded instructions: %+v, %v", result, err)
	}
	if _, err := ResolveInstructions(root, "../outside.go"); err == nil {
		t.Fatal("accepted traversal")
	}
	if err := os.Remove(filepath.Join(root, ".mini-orca/context-policy.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(strings.Repeat("a", MaxInstructionBytes+1)), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveInstructions(root, "new.go"); err == nil {
		t.Fatal("accepted oversized instructions")
	}
}

func TestInstructionsMissingAndSymlink(t *testing.T) {
	root := t.TempDir()
	result, err := ResolveInstructions(root, "new/path.go")
	if err != nil || len(result.Files) != 0 || result.Fingerprint == "" {
		t.Fatalf("missing instructions: %+v, %v", result, err)
	}
	outside := filepath.Join(t.TempDir(), "guide.md")
	if err := os.WriteFile(outside, []byte("Outside."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveInstructions(root, "new.go"); err == nil {
		t.Fatal("followed instruction symlink")
	}
}
