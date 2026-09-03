package project

import (
	"strings"
	"testing"
)

func TestComposeGoDeclarationReplacesExactFunctionMethodTypeAndVariable(t *testing.T) {
	original := `package fixture

import "fmt"

// Run keeps the original behavior.
func Run() { fmt.Println("old") }

type Worker struct{}

func (Worker) Work() string { return "old" }

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "old",
}

func Other() {}
`
	for _, test := range []struct {
		name        string
		target      string
		declaration string
		want        string
	}{
		{name: "function", target: "Run", declaration: "// Run has new behavior.\nfunc Run() { fmt.Println(\"new\") }", want: "fmt.Println(\"new\")"},
		{name: "method", target: "Worker.Work", declaration: "func (Worker) Work() string { return \"new\" }", want: "return \"new\""},
		{name: "type", target: "Worker", declaration: "type Worker struct { Name string }", want: "type Worker struct{ Name string }"},
		{name: "variable", target: "diffCmd", declaration: "// diffCmd has new behavior.\nvar diffCmd = &cobra.Command{Use: \"diff\", Short: \"new\"}", want: "Short: \"new\""},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := ComposeGoDeclaration("fixture.go", original, GoDeclarationEdit{
				Mode: DeclarationEditReplaceSymbol, TargetSymbol: test.target, Declaration: test.declaration,
			})
			if !result.Validation.Applicable {
				t.Fatalf("composition diagnostics = %#v", result.Validation.Diagnostics)
			}
			if result.CompositionHash == "" || !strings.Contains(result.Source, test.want) {
				t.Fatalf("composition = %q, hash = %q", result.Source, result.CompositionHash)
			}
			if !strings.Contains(result.Source, "func Other() {}") || !strings.Contains(result.Source, `import "fmt"`) {
				t.Fatalf("unrelated source changed: %q", result.Source)
			}
			if result.NormalizedDeclaration == "" {
				t.Fatal("normalized declaration is empty")
			}
			if test.target == "Run" && (!strings.Contains(result.Source, "// Run has new behavior.") || strings.Contains(result.Source, "// Run keeps the original behavior.")) {
				t.Fatalf("target comment was not replaced: %q", result.Source)
			}
		})
	}
}

func TestComposeGoDeclarationCreatesAtEndAndAddsOnlyRequestedImports(t *testing.T) {
	original := `package fixture

import "fmt"

// Existing remains before the new declaration.
func Existing() { fmt.Println("existing") }
`
	result := ComposeGoDeclaration("fixture.go", original, GoDeclarationEdit{
		Mode:         DeclarationEditCreateSymbol,
		TargetSymbol: "NewWorker",
		Declaration:  "// NewWorker is deliberately appended.\ntype NewWorker struct { Name string }",
		Imports:      []string{"strings", `alias "example.com/alias"`},
	})
	if !result.Validation.Applicable {
		t.Fatalf("composition diagnostics = %#v", result.Validation.Diagnostics)
	}
	if !strings.Contains(result.Source, `import "fmt"`) || !strings.Contains(result.Source, `"strings"`) || !strings.Contains(result.Source, `alias "example.com/alias"`) {
		t.Fatalf("imports = %q", result.Source)
	}
	if strings.Index(result.Source, "func Existing") > strings.Index(result.Source, "type NewWorker") {
		t.Fatalf("new declaration was not inserted after existing declarations: %q", result.Source)
	}
	if added := diffLines(result.Validation.Diff); len(added) == 0 || !containsDiffText(added, "type NewWorker struct{ Name string }") {
		t.Fatalf("diff = %#v", result.Validation.Diff.Lines)
	}
}

func TestComposeGoDeclarationReplacesTypeInsideGroupedDeclaration(t *testing.T) {
	original := `package fixture

type (
	// Worker is the original target.
	Worker struct{}
	Other  struct{}
)

func Run() {}
`
	result := ComposeGoDeclaration("fixture.go", original, GoDeclarationEdit{
		Mode:         DeclarationEditReplaceSymbol,
		TargetSymbol: "Worker",
		Declaration:  "// Worker has the replacement shape.\ntype Worker struct { Name string }",
	})
	if !result.Validation.Applicable {
		t.Fatalf("composition diagnostics = %#v", result.Validation.Diagnostics)
	}
	if !strings.Contains(result.Source, "// Worker has the replacement shape.\n\tWorker struct{ Name string }") {
		t.Fatalf("replacement type spec = %q", result.Source)
	}
	if strings.Contains(result.Source, "// Worker is the original target.") || !strings.Contains(result.Source, "Other") || !strings.Contains(result.Source, "func Run() {}") {
		t.Fatalf("unrelated grouped declarations changed: %q", result.Source)
	}
}

func TestComposeGoDeclarationRejectsInvalidOrOutOfScopeEdits(t *testing.T) {
	original := `package fixture

func Run() {}
func Other() {}
`
	for _, test := range []struct {
		name string
		edit GoDeclarationEdit
		code string
	}{
		{name: "unknown replacement target", edit: GoDeclarationEdit{Mode: DeclarationEditReplaceSymbol, TargetSymbol: "Missing", Declaration: "func Missing() {}"}, code: "target_identity"},
		{name: "duplicate create target", edit: GoDeclarationEdit{Mode: DeclarationEditCreateSymbol, TargetSymbol: "Run", Declaration: "func Run() {}"}, code: "target_identity"},
		{name: "invalid target name", edit: GoDeclarationEdit{Mode: DeclarationEditCreateSymbol, TargetSymbol: "not valid", Declaration: "func Valid() {}"}, code: "invalid_target"},
		{name: "target mismatch", edit: GoDeclarationEdit{Mode: DeclarationEditReplaceSymbol, TargetSymbol: "Run", Declaration: "func Other() {}"}, code: "target_mismatch"},
		{name: "multiple declarations", edit: GoDeclarationEdit{Mode: DeclarationEditReplaceSymbol, TargetSymbol: "Run", Declaration: "func Run() {}\nfunc Added() {}"}, code: "invalid_declaration"},
		{name: "package clause", edit: GoDeclarationEdit{Mode: DeclarationEditReplaceSymbol, TargetSymbol: "Run", Declaration: "package other\nfunc Run() {}"}, code: "invalid_declaration"},
		{name: "replacement kind changed", edit: GoDeclarationEdit{Mode: DeclarationEditReplaceSymbol, TargetSymbol: "Run", Declaration: "var Run = 1"}, code: "target_kind"},
		{name: "malformed declaration", edit: GoDeclarationEdit{Mode: DeclarationEditReplaceSymbol, TargetSymbol: "Run", Declaration: "func Run( {"}, code: "invalid_declaration"},
		{name: "duplicate requested import", edit: GoDeclarationEdit{Mode: DeclarationEditReplaceSymbol, TargetSymbol: "Run", Declaration: "func Run() {}", Imports: []string{"fmt", `"fmt"`}}, code: "invalid_import"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := ComposeGoDeclaration("fixture.go", original, test.edit)
			if result.Validation.Applicable || result.Source != "" || result.CompositionHash != "" {
				t.Fatalf("invalid composition = %#v", result)
			}
			if len(result.Validation.Diagnostics) != 1 || result.Validation.Diagnostics[0].Code != test.code {
				t.Fatalf("diagnostics = %#v, want %q", result.Validation.Diagnostics, test.code)
			}
		})
	}
}

func TestValidateGoDeclarationCompositionRejectsUnrelatedDeclarationsAndImports(t *testing.T) {
	original := `package fixture

import "fmt"

func Run() { fmt.Println("old") }
func Other() {}
`
	edit := GoDeclarationEdit{Mode: DeclarationEditReplaceSymbol, TargetSymbol: "Run", Declaration: `func Run() { fmt.Println("new") }`}
	for _, test := range []struct {
		name     string
		composed string
		code     string
	}{
		{
			name:     "unrelated declaration",
			composed: strings.Replace(strings.Replace(original, "old", "new", 1), "func Other() {}", `func Other() { fmt.Println("changed") }`, 1),
			code:     "out_of_scope_declaration",
		},
		{
			name:     "unrequested import",
			composed: strings.Replace(strings.Replace(original, "old", "new", 1), `import "fmt"`, "import (\n\t\"fmt\"\n\t\"strings\"\n)", 1),
			code:     "out_of_scope_import",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			validation := validateGoDeclarationComposition("fixture.go", original, test.composed, edit, nil)
			if validation.Applicable || len(validation.Diagnostics) != 1 || validation.Diagnostics[0].Code != test.code {
				t.Fatalf("validation = %#v, want %q", validation, test.code)
			}
		})
	}
}

func TestBuildUnifiedDiffKeepsFollowingLinesAsContextAfterInsertion(t *testing.T) {
	diff := buildUnifiedDiff("fixture.go", "one\ntwo\nthree\n", "one\ninserted\ntwo\nthree\n")
	if len(diff.Lines) != 5 {
		t.Fatalf("diff lines = %#v", diff.Lines)
	}
	if diff.Lines[1] != (DiffLine{Kind: "added", NewLine: 2, Text: "inserted"}) {
		t.Fatalf("insertion = %#v", diff.Lines[1])
	}
	if diff.Lines[2].Kind != "context" || diff.Lines[2].OldLine != 2 || diff.Lines[2].NewLine != 3 || diff.Lines[3].Kind != "context" || diff.Lines[3].OldLine != 3 || diff.Lines[3].NewLine != 4 {
		t.Fatalf("following lines were not aligned as context: %#v", diff.Lines)
	}
}

func diffLines(diff UnifiedDiff) []DiffLine {
	result := make([]DiffLine, 0, len(diff.Lines))
	for _, line := range diff.Lines {
		if line.Kind == "added" {
			result = append(result, line)
		}
	}
	return result
}

func containsDiffText(lines []DiffLine, text string) bool {
	for _, line := range lines {
		if line.Text == text {
			return true
		}
	}
	return false
}
