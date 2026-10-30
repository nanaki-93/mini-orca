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
