package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFingerprintCheckWorkspaceCancelsDuringRead(t *testing.T) {
	root := t.TempDir()
	writeBenchmarkFile(t, root, "large.go", strings.Repeat("x", 2*checkCopyBufferSize))
	ctx := &cancelAfterBenchmarkContext{cancelAt: 4}
	if _, err := fingerprintCheckWorkspace(ctx, root); !errors.Is(err, context.Canceled) {
		t.Fatalf("fingerprint error = %v, want context cancellation", err)
	}
}

func TestWorkspaceFingerprintMatchesCopiedCheckWorkspace(t *testing.T) {
	root := t.TempDir()
	writeBenchmarkFile(t, root, "main.go", "package main\n")
	if err := os.MkdirAll(filepath.Join(root, ".mini-orca"), 0700); err != nil {
		t.Fatal(err)
	}
	writeBenchmarkFile(t, root, filepath.Join(".mini-orca", "state.json"), "{}")
	if err := os.MkdirAll(filepath.Join(root, "node_modules", "pkg"), 0700); err != nil {
		t.Fatal(err)
	}
	writeBenchmarkFile(t, root, filepath.Join("node_modules", "pkg", "index.js"), "export default 1;\n")

	direct, err := workspaceFingerprint(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	copiedDir := t.TempDir()
	if err := copyCheckWorkspace(context.Background(), root, copiedDir); err != nil {
		t.Fatal(err)
	}
	copied, err := fingerprintCheckWorkspace(context.Background(), copiedDir)
	if err != nil {
		t.Fatal(err)
	}
	if direct != copied {
		t.Fatalf("in-place fingerprint %s != copied fingerprint %s", direct, copied)
	}
}

type cancelAfterBenchmarkContext struct {
	checks   int
	cancelAt int
}

func (ctx *cancelAfterBenchmarkContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (ctx *cancelAfterBenchmarkContext) Done() <-chan struct{}       { return nil }
func (ctx *cancelAfterBenchmarkContext) Value(any) any               { return nil }
func (ctx *cancelAfterBenchmarkContext) Err() error {
	ctx.checks++
	if ctx.checks >= ctx.cancelAt {
		return context.Canceled
	}
	return nil
}

func writeBenchmarkFile(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}
