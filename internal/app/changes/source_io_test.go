package changes

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteSourceAndContentHash(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "sample.go")
	if err := os.WriteFile(target, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	updated := []byte("package main\n\nfunc main() {}\n")
	if err := AtomicWriteSource(target, updated); err != nil {
		t.Fatalf("AtomicWriteSource failed: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if ContentHash(got) != ContentHash(updated) {
		t.Fatalf("hash mismatch: got %s, want %s", ContentHash(got), ContentHash(updated))
	}
}

func TestResolveExpandedTargetPathsFromCheckOutputsAndImports(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/pwd-manager\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for rel, content := range map[string]string{
		"cmd/master.go":                "package cmd\nimport \"example.test/pwd-manager/internal/utils\"\nfunc save() { _ = utils.Key }\n",
		"cmd/master_test.go":           "package cmd\n",
		"cmd/add.go":                   "package cmd\n",
		"cmd/add_test.go":              "package cmd\n",
		"internal/utils/utils.go":      "package utils\nconst Key = \"k\"\n",
		"internal/utils/utils_test.go": "package utils\n",
		"internal/model/pwd.go":        "package model\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	isTest := func(p string) bool { return filepath.Ext(p) == ".go" && len(p) > 8 && p[len(p)-8:] == "_test.go" }

	// Case 1: Sibling test failure in same package (e.g. add_test.go:236) + coupled local package import (internal/utils).
	existing1 := []string{"cmd/master.go", "cmd/master_test.go"}
	contents1 := map[string]string{
		"cmd/master.go": "package cmd\nimport (\n\t\"example.test/pwd-manager/internal/model\"\n\t\"example.test/pwd-manager/internal/utils\"\n)\n",
	}
	out1 := []string{"--- FAIL: TestSavePasswordWhenMasterPasswordEmpty (0.00s)\n    add_test.go:236: expected \"Password saved\", got \"\"\nFAIL\nFAIL\texample.test/pwd-manager/cmd\t0.200s"}
	got1 := ResolveExpandedTargetPaths(root, existing1, contents1, out1, isTest)
	if len(got1) != 3 || got1[0] != "cmd/add_test.go" || got1[1] != "internal/utils/utils.go" || got1[2] != "internal/utils/utils_test.go" {
		t.Fatalf("case 1 expanded targets = %+v, want [cmd/add_test.go internal/utils/utils.go internal/utils/utils_test.go]", got1)
	}

	// Case 2: Cross-package caller failure when editing internal/utils/utils.go.
	existing2 := []string{"internal/utils/utils.go", "internal/utils/utils_test.go"}
	out2 := []string{"--- FAIL: TestExecMasterCmd (0.00s)\n    master_test.go:179: missing key\nFAIL\texample.test/pwd-manager/cmd\t0.200s"}
	got2 := ResolveExpandedTargetPaths(root, existing2, nil, out2, isTest)
	if len(got2) != 2 || got2[0] != "cmd/master.go" || got2[1] != "cmd/master_test.go" {
		t.Fatalf("case 2 expanded targets = %+v, want [cmd/master.go cmd/master_test.go]", got2)
	}
}
