package project

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestWalkProjectFilesPreservesEligibilityOrderingAndSymlinkPolicy(t *testing.T) {
	root := t.TempDir()
	writeSourceWalkFixture(t, root, "z.txt", "text")
	writeSourceWalkFixture(t, root, "src/main.go", "package main\n")
	writeSourceWalkFixture(t, root, "src/generated.tmp", "temporary")
	writeSourceWalkFixture(t, root, "vendor/dependency.go", "package dependency\n")
	writeSourceWalkFixture(t, root, ".git/ignored.go", "package ignored\n")
	writeSourceWalkFixture(t, root, ".gitignore", "src/generated.tmp\n")
	outside := filepath.Join(t.TempDir(), "outside.go")
	if err := os.WriteFile(outside, []byte("package outside\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked.go")); err != nil {
		t.Fatal(err)
	}
	policy, err := NewContextPolicy(root)
	if err != nil {
		t.Fatal(err)
	}

	eligible, err := WalkProjectFiles(context.Background(), ProjectWalkOptions{
		Root: root, Policy: policy, IgnoredDirectories: ignoredProjectDirectories, MaxFiles: maxProjectFiles,
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{".gitignore", "src/main.go", "z.txt"}; !reflect.DeepEqual(eligible, want) {
		t.Fatalf("eligible files = %v, want %v", eligible, want)
	}

	all, err := WalkProjectFiles(context.Background(), ProjectWalkOptions{
		Root: root, IgnoredDirectories: ignoredProjectDirectories, IncludeSymlinkFiles: true, MaxFiles: maxProjectFiles,
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{".gitignore", "linked.go", "src/generated.tmp", "src/main.go", "z.txt"}; !reflect.DeepEqual(all, want) {
		t.Fatalf("all files = %v, want %v", all, want)
	}
}

func TestWalkProjectFilesHonorsExtensionsLimitsAndCancellation(t *testing.T) {
	root := t.TempDir()
	writeSourceWalkFixture(t, root, "a.go", "package a\n")
	writeSourceWalkFixture(t, root, "b.txt", "text")

	goOnly, err := WalkProjectFiles(context.Background(), ProjectWalkOptions{
		Root: root, IgnoredDirectories: ignoredProjectDirectories, AllowedExtensions: map[string]bool{".go": true}, MaxFiles: maxProjectFiles,
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a.go"}; !reflect.DeepEqual(goOnly, want) {
		t.Fatalf("Go files = %v, want %v", goOnly, want)
	}

	if _, err := WalkProjectFiles(context.Background(), ProjectWalkOptions{
		Root: root, IgnoredDirectories: ignoredProjectDirectories, MaxFiles: 1,
	}); err == nil {
		t.Fatal("expected maximum file error")
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := WalkProjectFiles(canceled, ProjectWalkOptions{
		Root: root, IgnoredDirectories: ignoredProjectDirectories, MaxFiles: maxProjectFiles,
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled walk error = %v", err)
	}
}

func TestWalkProjectFilesSkipsUnreadableDescendants(t *testing.T) {
	root := t.TempDir()
	writeSourceWalkFixture(t, root, "public.go", "package public\n")
	writeSourceWalkFixture(t, root, "restricted/secret.go", "package restricted\n")
	restricted := filepath.Join(root, "restricted")
	if err := os.Chmod(restricted, 0); err != nil {
		t.Skipf("cannot create unreadable fixture: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(restricted, 0o755) })
	if _, err := os.ReadDir(restricted); err == nil {
		t.Skip("test process can read a mode-000 directory")
	}

	files, err := WalkProjectFiles(context.Background(), ProjectWalkOptions{
		Root: root, IgnoredDirectories: ignoredProjectDirectories, MaxFiles: maxProjectFiles,
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"public.go"}; !reflect.DeepEqual(files, want) {
		t.Fatalf("files = %v, want %v", files, want)
	}
}

func TestProjectSurfacesUseTheSamePolicyEligibleFiles(t *testing.T) {
	root := t.TempDir()
	writeSourceWalkFixture(t, root, "main.go", "package main\nfunc Run() {}\n")
	writeSourceWalkFixture(t, root, "notes.txt", "notes")
	writeSourceWalkFixture(t, root, ".env", "TOKEN=hidden")
	writeSourceWalkFixture(t, root, "ignored.go", "package ignored\n")
	writeSourceWalkFixture(t, root, ".gitignore", "ignored.go\n")
	policy, err := NewContextPolicy(root)
	if err != nil {
		t.Fatal(err)
	}
	want, err := WalkProjectFiles(context.Background(), ProjectWalkOptions{
		Root: root, Policy: policy, IgnoredDirectories: ignoredProjectDirectories, MaxFiles: maxProjectFiles,
	})
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := scanWithPolicy(context.Background(), root, policy)
	if err != nil {
		t.Fatal(err)
	}
	index, err := BuildIndex(root, "project", "revision")
	if err != nil {
		t.Fatal(err)
	}
	_, manifest, err := NewContextBuilder().BuildWithManifest(root, "main.go")
	if err != nil {
		t.Fatal(err)
	}
	if got := analysis.Files; !reflect.DeepEqual(got, want) {
		t.Fatalf("analysis files = %v, want %v", got, want)
	}
	if got := indexPaths(index); !reflect.DeepEqual(got, want) {
		t.Fatalf("index files = %v, want %v", got, want)
	}
	if got := manifestPaths(manifest); !reflect.DeepEqual(got, want) {
		t.Fatalf("context files = %v, want %v", got, want)
	}
}

func writeSourceWalkFixture(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func indexPaths(index *ProjectIndex) []string {
	paths := make([]string, 0, len(index.Files))
	for _, file := range index.Files {
		paths = append(paths, file.Path)
	}
	sort.Strings(paths)
	return paths
}

func manifestPaths(manifest ContextManifest) []string {
	paths := make([]string, 0, len(manifest.Included))
	for _, file := range manifest.Included {
		paths = append(paths, file.Path)
	}
	sort.Strings(paths)
	return paths
}
