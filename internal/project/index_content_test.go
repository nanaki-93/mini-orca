package project

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLargeSourceIndexRetainsFullHashAndLinesWithoutParsingPartialSource(t *testing.T) {
	root := t.TempDir()
	content := "package sample\n" + strings.Repeat("// bounded memory\n", maxFileViewBytes/8) + "func BeyondLimit() {}"
	writeIndexFixture(t, root, "large.go", content)
	entry, err := buildIndexFile(root, "large.go", IndexFile{})
	if err != nil {
		t.Fatal(err)
	}
	if entry.Binary || entry.SizeBytes != int64(len(content)) || entry.ContentHash != contentHash([]byte(content)) || entry.LineCount != countLines([]byte(content)) {
		t.Fatalf("large source facts must cover all bytes: %+v", entry)
	}
	if len(entry.Symbols) != 0 || len(entry.Imports) != 0 || len(entry.Diagnostics) != 1 || !strings.Contains(entry.Diagnostics[0].Message, "symbol extraction is unavailable") {
		t.Fatalf("oversized source must not have partial parser facts: %+v", entry)
	}
	path := filepath.Join(root, "large.go")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	read, err := readIndexContent(path, info)
	if err != nil || len(read.source) > maxFileViewBytes {
		t.Fatalf("source retention exceeded the parsing budget: %d, %v", len(read.source), err)
	}
	legacy := ProjectIndex{SchemaVersion: "2", Files: []IndexFile{{Path: "large.go", ContentHash: entry.ContentHash, Symbols: []SymbolInfo{{Name: "BeyondLimit", AtomicTarget: true}}}}}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	writeIndexFixture(t, root, indexRelativePath, string(data))
	rebuilt, err := BuildIndex(root, "project", "revision")
	if err != nil || len(rebuilt.Files) != 1 || len(rebuilt.Files[0].Symbols) != 0 || len(rebuilt.Files[0].Diagnostics) != 1 {
		t.Fatalf("legacy unbounded parser facts must be rebuilt: %+v, %v", rebuilt, err)
	}
}

func TestStreamedIndexMatchesSmallTextBinaryAndEmptyFacts(t *testing.T) {
	for _, content := range []string{"", "one", "one\ntwo\n", "one\ntwo", "\x00" + strings.Repeat("x", maxFileViewBytes*2), strings.Repeat("x", 32767) + "\nend"} {
		root := t.TempDir()
		writeIndexFixture(t, root, "sample.txt", content)
		entry, err := buildIndexFile(root, "sample.txt", IndexFile{})
		if err != nil {
			t.Fatal(err)
		}
		lines := countLines([]byte(content))
		if isBinary([]byte(content)) {
			lines = 0
		}
		if entry.ContentHash != contentHash([]byte(content)) || entry.LineCount != lines || entry.Binary != isBinary([]byte(content)) {
			t.Fatalf("streamed facts differ for %d bytes: %+v", len(content), entry)
		}
	}
}

func TestVerifyIndexedSourcesReadsContentDespiteUnchangedSizeAndTimestamp(t *testing.T) {
	root := t.TempDir()
	writeIndexFixture(t, root, "main.go", "package sample\nfunc Run() { println(1) }\n")
	entry, err := buildIndexFile(root, "main.go", IndexFile{})
	if err != nil {
		t.Fatal(err)
	}
	files := []IndexFile{entry}
	if err := VerifyIndexedSources(context.Background(), root, files); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := VerifyIndexedSources(canceled, root, files); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled verification = %v", err)
	}
	writeIndexFixture(t, root, "main.go", "package sample\nfunc Run() { println(2) }\n")
	if err := os.Chtimes(filepath.Join(root, "main.go"), entry.ModifiedAt, entry.ModifiedAt); err != nil {
		t.Fatal(err)
	}
	if err := VerifyIndexedSources(context.Background(), root, files); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("same-size source edit = %v; want conflict", err)
	}
	writeIndexFixture(t, root, "main.go", strings.Repeat("x", maxFileViewBytes+1))
	if err := VerifyIndexedSources(context.Background(), root, files); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("oversized replacement = %v; want conflict", err)
	}
}

func TestVerifyIndexedSourcesKeepsBatchPathContainment(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeIndexFixture(t, outside, "other.go", "package other\n")
	if err := os.Symlink(filepath.Join(outside, "other.go"), filepath.Join(root, "linked.go")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"linked.go", "../other.go", filepath.Join(outside, "other.go")} {
		err := VerifyIndexedSources(context.Background(), root, []IndexFile{{Path: path, ContentHash: contentHash([]byte("package other\n"))}})
		if err == nil {
			t.Fatalf("outside source was accepted: %s", path)
		}
	}
}
