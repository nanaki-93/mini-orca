package project

import (
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/workflow"
)

func TestValidateGenerationGoScope(t *testing.T) {
	original := "package fixture\n\nimport \"fmt\"\n\nfunc Run() { fmt.Println(\"old\") }\n\nfunc Other() {}\n"
	target := SymbolInfo{Name: "Run", Confidence: "exact", AtomicTarget: true}
	for _, test := range []struct {
		name       string
		candidate  string
		scope      workflow.ScopeMode
		applicable bool
		code       string
	}{
		{name: "target only", candidate: strings.Replace(original, "old", "new", 1), scope: workflow.ScopeStrictSymbol, applicable: true},
		{name: "second function", candidate: strings.Replace(strings.Replace(original, "old", "new", 1), "func Other() {}", "func Other() { panic(\"changed\") }", 1), scope: workflow.ScopeStrictSymbol, code: "out_of_scope_declaration"},
		{name: "renamed target", candidate: strings.Replace(original, "func Run", "func Renamed", 1), scope: workflow.ScopeStrictSymbol, code: "target_identity"},
		{name: "invalid syntax", candidate: "package fixture\nfunc Run( {", scope: workflow.ScopeStrictSymbol, code: "candidate_syntax"},
		{name: "strict import", candidate: strings.Replace(original, "import \"fmt\"", "import \"strings\"", 1), scope: workflow.ScopeStrictSymbol, code: "out_of_scope_import"},
		{name: "imports allowed", candidate: strings.Replace(strings.Replace(original, "import \"fmt\"", "import \"strings\"", 1), "fmt.Println", "strings.TrimSpace", 1), scope: workflow.ScopeSymbolPlusImports, applicable: true},
		{name: "unused import addition", candidate: strings.Replace(original, "import \"fmt\"", "import (\n\t\"fmt\"\n\t\"strings\"\n)", 1), scope: workflow.ScopeSymbolPlusImports, code: "out_of_scope_import"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := ValidateGeneration("fixture.go", original, test.candidate, target, test.scope)
			if got.Applicable != test.applicable {
				t.Fatalf("validation = %+v", got)
			}
			if test.code != "" && (len(got.Diagnostics) != 1 || got.Diagnostics[0].Code != test.code) {
				t.Fatalf("diagnostics = %+v, want %q", got.Diagnostics, test.code)
			}
			if len(got.Diff.Lines) == 0 || got.Diff.OldPath != "fixture.go" {
				t.Fatalf("diff = %+v", got.Diff)
			}
		})
	}
}

func TestValidateGenerationRefusesApproximateTargetChanges(t *testing.T) {
	got := ValidateGeneration("Example.kt", "fun run() = 1\n", "fun run() = 2\n", SymbolInfo{Name: "run", Confidence: "approximate", AtomicTarget: true}, workflow.ScopeStrictSymbol)
	if got.Applicable || len(got.Diagnostics) != 1 || got.Diagnostics[0].Code != "ambiguous_scope" {
		t.Fatalf("validation = %+v", got)
	}
}
