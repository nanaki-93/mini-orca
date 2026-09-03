package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoIndexExtractsExactSymbolsAndImports(t *testing.T) {
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

var (
	grouped = 1
)
var first, second = 1, 2
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
	assertGoSymbol(t, file, "Run", "function", true, 13, 13)
	assertGoSymbol(t, file, "Box.Work", "method", true, 14, 14)
	assertGoSymbol(t, file, "Box", "struct", true, 8, 8)
	assertGoSymbol(t, file, "Worker", "interface", true, 9, 9)
	assertGoSymbol(t, file, "DefaultName", "const", false, 10, 10)
	assertGoSymbol(t, file, "cached", "var", true, 11, 11)
	assertGoSymbol(t, file, "grouped", "var", false, 17, 17)
	assertGoSymbol(t, file, "first", "var", false, 19, 19)
	assertGoSymbol(t, file, "second", "var", false, 19, 19)
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

func TestGoIndexTreatsSingleClosureVariableAsAnAtomicTarget(t *testing.T) {
	root := t.TempDir()
	source := `package fixture

var diffCmd = func() error {
	return nil
}
`
	if err := os.WriteFile(filepath.Join(root, "command.go"), []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	index, err := BuildIndex(root, "sha256:project", "sha256:revision")
	if err != nil {
		t.Fatal(err)
	}
	file := findIndexFile(index, "command.go")
	if file == nil {
		t.Fatal("Go file missing from index")
	}
	assertGoSymbol(t, file, "diffCmd", "var", true, 3, 5)
}

func assertGoSymbol(t *testing.T, file *IndexFile, name, kind string, atomic bool, start, end int) {
	t.Helper()
	for _, symbol := range file.Symbols {
		if symbol.Name != name {
			continue
		}
		if symbol.Kind != kind || symbol.AtomicTarget != atomic || symbol.StartLine != start || symbol.EndLine != end || symbol.Confidence != "exact" {
			t.Fatalf("symbol %s = %#v", name, symbol)
		}
		return
	}
	t.Fatalf("symbol %s not found in %#v", name, file.Symbols)
}
