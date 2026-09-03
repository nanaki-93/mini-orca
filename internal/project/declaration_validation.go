package project

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
	"strings"
)

// DeclarationValidation describes whether a composed declaration remains safely applicable.
type DeclarationValidation struct {
	Applicable  bool                 `json:"applicable"`
	ScopeMode   string               `json:"scope_mode"`
	Diagnostics []DeclarationFinding `json:"diagnostics"`
	Diff        UnifiedDiff          `json:"diff"`
}

type DeclarationFinding struct {
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

func invalidDeclarationValidation(validation DeclarationValidation, code, message string) DeclarationValidation {
	validation.Diagnostics = []DeclarationFinding{{Code: code, Message: message}}
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
			set[name] = append(set[name], renderGoDeclaration(fset, declaration, declaration.Doc))
		case *ast.GenDecl:
			if declaration.Tok != token.TYPE && declaration.Tok != token.VAR && declaration.Tok != token.CONST {
				continue
			}
			for index, spec := range declaration.Specs {
				comments := declarationSpecComments(spec)
				if index == 0 {
					comments = append([]*ast.CommentGroup{declaration.Doc}, comments...)
				}
				for _, name := range declarationNames(spec) {
					set[name] = append(set[name], renderGoDeclaration(fset, spec, comments...))
				}
			}
		}
	}
	return set, nil
}

func declarationSpecComments(spec ast.Spec) []*ast.CommentGroup {
	switch spec := spec.(type) {
	case *ast.TypeSpec:
		return []*ast.CommentGroup{spec.Doc, spec.Comment}
	case *ast.ValueSpec:
		return []*ast.CommentGroup{spec.Doc, spec.Comment}
	default:
		return nil
	}
}

func renderGoDeclaration(fset *token.FileSet, node ast.Node, comments ...*ast.CommentGroup) string {
	var output strings.Builder
	for _, group := range comments {
		if group == nil {
			continue
		}
		output.WriteString(group.Text())
		output.WriteByte('\n')
	}
	output.WriteString(renderGoNode(fset, node))
	return output.String()
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

func buildUnifiedDiff(path, original, composed string) UnifiedDiff {
	before, after := strings.Split(original, "\n"), strings.Split(composed, "\n")
	common := longestCommonSubsequence(before, after)
	lines := make([]DiffLine, 0, len(before)+len(after))
	for oldLine, newLine := 0, 0; oldLine < len(before) || newLine < len(after); {
		switch {
		case oldLine < len(before) && newLine < len(after) && before[oldLine] == after[newLine]:
			lines = append(lines, DiffLine{Kind: "context", OldLine: oldLine + 1, NewLine: newLine + 1, Text: before[oldLine]})
			oldLine++
			newLine++
		case newLine < len(after) && (oldLine == len(before) || common[oldLine][newLine+1] >= common[oldLine+1][newLine]):
			lines = append(lines, DiffLine{Kind: "added", NewLine: newLine + 1, Text: after[newLine]})
			newLine++
		default:
			lines = append(lines, DiffLine{Kind: "removed", OldLine: oldLine + 1, Text: before[oldLine]})
			oldLine++
		}
	}
	return UnifiedDiff{OldPath: path, NewPath: path, Lines: lines}
}

func longestCommonSubsequence(before, after []string) [][]int {
	common := make([][]int, len(before)+1)
	for index := range common {
		common[index] = make([]int, len(after)+1)
	}
	for oldLine := len(before) - 1; oldLine >= 0; oldLine-- {
		for newLine := len(after) - 1; newLine >= 0; newLine-- {
			if before[oldLine] == after[newLine] {
				common[oldLine][newLine] = common[oldLine+1][newLine+1] + 1
			} else if common[oldLine+1][newLine] >= common[oldLine][newLine+1] {
				common[oldLine][newLine] = common[oldLine+1][newLine]
			} else {
				common[oldLine][newLine] = common[oldLine][newLine+1]
			}
		}
	}
	return common
}
