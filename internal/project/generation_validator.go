package project

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/workflow"
)

// GenerationValidation describes whether a candidate remains safely applicable.
type GenerationValidation struct {
	Applicable  bool                `json:"applicable"`
	ScopeMode   workflow.ScopeMode  `json:"scope_mode"`
	Diagnostics []GenerationFinding `json:"diagnostics"`
	Diff        UnifiedDiff         `json:"diff"`
}

type GenerationFinding struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type UnifiedDiff struct {
	OldPath string     `json:"old_path"`
	NewPath string     `json:"new_path"`
	Lines   []DiffLine `json:"lines"`
}

type DiffLine struct {
	Kind    string `json:"kind"`
	OldLine int    `json:"old_line,omitempty"`
	NewLine int    `json:"new_line,omitempty"`
	Text    string `json:"text"`
}

// ValidateGeneration proves Go candidates leave all non-target declarations
// unchanged. Other languages are accepted only when unchanged because their
// fallback symbols are approximate rather than parser-exact.
func ValidateGeneration(path, original, candidate string, target SymbolInfo, scope workflow.ScopeMode) GenerationValidation {
	validation := GenerationValidation{ScopeMode: scope, Diff: buildUnifiedDiff(path, original, candidate)}
	if target.Confidence != "exact" || !target.AtomicTarget || !strings.EqualFold(detectLanguage(path), "Go") {
		if original == candidate {
			validation.Applicable = true
			return validation
		}
		return invalidGeneration(validation, "ambiguous_scope", "This language does not have an exact parser-backed target validator.")
	}
	if scope != workflow.ScopeStrictSymbol && scope != workflow.ScopeSymbolPlusImports {
		return invalidGeneration(validation, "invalid_scope", "The requested generation scope is not supported.")
	}
	fset := token.NewFileSet()
	before, err := parser.ParseFile(fset, path, original, parser.ParseComments)
	if err != nil {
		return invalidGeneration(validation, "original_syntax", "The original Go file cannot be parsed for scope validation.")
	}
	after, err := parser.ParseFile(fset, path, candidate, parser.ParseComments)
	if err != nil {
		return invalidGeneration(validation, "candidate_syntax", "The candidate Go file has invalid syntax.")
	}
	if before.Name.Name != after.Name.Name {
		return invalidGeneration(validation, "package_changed", "The candidate changes the Go package.")
	}
	beforeDecls, err := goDeclarations(fset, before)
	if err != nil {
		return invalidGeneration(validation, "original_declarations", err.Error())
	}
	afterDecls, err := goDeclarations(fset, after)
	if err != nil {
		return invalidGeneration(validation, "candidate_declarations", err.Error())
	}
	if beforeDecls.count(target.Name) != 1 || afterDecls.count(target.Name) != 1 {
		return invalidGeneration(validation, "target_identity", "The selected symbol is missing, renamed, or duplicated.")
	}
	if !sameNonTargetDeclarations(beforeDecls, afterDecls, target.Name) {
		return invalidGeneration(validation, "out_of_scope_declaration", "The candidate changes a declaration outside the selected symbol.")
	}
	if scope == workflow.ScopeStrictSymbol && !sameImports(fset, before, after) {
		return invalidGeneration(validation, "out_of_scope_import", "Strict symbol scope does not allow import changes.")
	}
	validation.Applicable = true
	return validation
}

func invalidGeneration(validation GenerationValidation, code, message string) GenerationValidation {
	validation.Diagnostics = []GenerationFinding{{Code: code, Message: message}}
	return validation
}

type declarationSet map[string][]string

func (set declarationSet) count(name string) int { return len(set[name]) }

func goDeclarations(fset *token.FileSet, file *ast.File) (declarationSet, error) {
	set := declarationSet{}
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			name := declaration.Name.Name
			if declaration.Recv != nil && len(declaration.Recv.List) > 0 {
				name = receiverName(declaration.Recv.List[0].Type) + "." + name
			}
			set[name] = append(set[name], renderGoNode(fset, declaration))
		case *ast.GenDecl:
			if declaration.Tok != token.TYPE && declaration.Tok != token.VAR && declaration.Tok != token.CONST {
				continue
			}
			for _, spec := range declaration.Specs {
				for _, name := range declarationNames(spec) {
					set[name] = append(set[name], renderGoNode(fset, spec))
				}
			}
		}
	}
	return set, nil
}

func declarationNames(spec ast.Spec) []string {
	switch spec := spec.(type) {
	case *ast.TypeSpec:
		return []string{spec.Name.Name}
	case *ast.ValueSpec:
		names := make([]string, 0, len(spec.Names))
		for _, name := range spec.Names {
			names = append(names, name.Name)
		}
		return names
	default:
		return nil
	}
}

func renderGoNode(fset *token.FileSet, node any) string {
	var output bytes.Buffer
	_ = printer.Fprint(&output, fset, node)
	return output.String()
}

func sameNonTargetDeclarations(before, after declarationSet, target string) bool {
	for name, beforeValues := range before {
		if name == target || len(beforeValues) != len(after[name]) {
			if name == target {
				continue
			}
			return false
		}
		for i := range beforeValues {
			if beforeValues[i] != after[name][i] {
				return false
			}
		}
	}
	for name := range after {
		if name != target {
			if _, ok := before[name]; !ok {
				return false
			}
		}
	}
	return true
}

func sameImports(fset *token.FileSet, before, after *ast.File) bool {
	render := func(file *ast.File) []string {
		imports := make([]string, 0, len(file.Imports))
		for _, spec := range file.Imports {
			imports = append(imports, renderGoNode(fset, spec))
		}
		return imports
	}
	left, right := render(before), render(after)
	return strings.Join(left, "\x00") == strings.Join(right, "\x00")
}

func buildUnifiedDiff(path, original, candidate string) UnifiedDiff {
	before, after := strings.Split(original, "\n"), strings.Split(candidate, "\n")
	lines := make([]DiffLine, 0)
	for i := 0; i < len(before) || i < len(after); i++ {
		switch {
		case i < len(before) && i < len(after) && before[i] == after[i]:
			lines = append(lines, DiffLine{Kind: "context", OldLine: i + 1, NewLine: i + 1, Text: before[i]})
		case i < len(before) && i < len(after):
			lines = append(lines, DiffLine{Kind: "removed", OldLine: i + 1, Text: before[i]}, DiffLine{Kind: "added", NewLine: i + 1, Text: after[i]})
		case i < len(before):
			lines = append(lines, DiffLine{Kind: "removed", OldLine: i + 1, Text: before[i]})
		default:
			lines = append(lines, DiffLine{Kind: "added", NewLine: i + 1, Text: after[i]})
		}
	}
	return UnifiedDiff{OldPath: path, NewPath: path, Lines: lines}
}
