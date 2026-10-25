package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoIndexExtractsImports(t *testing.T) {
	root := t.TempDir()
	source := `package fixture

import (
    "fmt"
    alias "strings"
)

type Box[T any] struct { Value T }
type Worker interface { Work(input string) error }
const DefaultName = "mini"
var cached int

func Run[T any](value T) T { return value }
func (b *Box[T]) Work(input string) error { fmt.Println(alias.TrimSpace(input)); return nil }
`
	if err := os.WriteFile(filepath.Join(root, "fixture.go"), []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	index, err := BuildIndex(root, "sha256:project", "sha256:revision")
	if err != nil {
		t.Fatal(err)
	}
	file := findIndexFile(index, "fixture.go")
	if file == nil {
		t.Fatal("Go file missing from index")
	}
	if len(file.Imports) != 2 || file.Imports[0] != "fmt" || file.Imports[1] != "strings" {
		t.Fatalf("imports = %#v", file.Imports)
	}
	if len(file.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", file.Diagnostics)
	}
}

func TestGoIndexKeepsMalformedFileWithDiagnostics(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "broken.go"), []byte("package fixture\nfunc Broken( {\n"), 0644); err != nil {
		t.Fatal(err)
	}
	index, err := BuildIndex(root, "sha256:project", "sha256:revision")
	if err != nil {
		t.Fatal(err)
	}
	file := findIndexFile(index, "broken.go")
	if file == nil || len(file.Diagnostics) == 0 {
		t.Fatalf("malformed file facts = %#v", file)
	}
	if file.Diagnostics[0].Line == 0 {
		t.Fatalf("diagnostic lacks source line: %#v", file.Diagnostics)
	}
}
