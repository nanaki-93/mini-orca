package symbols

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"strconv"
	"strings"
)

// ExtractGoFacts parses Go source and returns imports and parser diagnostics.
func ExtractGoFacts(filename string, source []byte) ([]string, []Diagnostic) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, source, parser.ParseComments|parser.AllErrors)
	diagnostics := parseDiagnostics(err)
	if file == nil {
		return []string{}, diagnostics
	}
	return goImports(file), diagnostics
}

func goImports(file *ast.File) []string {
	imports := make([]string, 0, len(file.Imports))
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err == nil {
			imports = append(imports, path)
		}
	}
	return imports
}

func parseDiagnostics(err error) []Diagnostic {
	if err == nil {
		return []Diagnostic{}
	}
	var errorList scanner.ErrorList
	if errors.As(err, &errorList) {
		diagnostics := make([]Diagnostic, 0, len(errorList))
		for _, parseError := range errorList {
			diagnostics = append(diagnostics, Diagnostic{Message: parseError.Msg, Line: parseError.Pos.Line})
		}
		return diagnostics
	}
	messages := strings.Split(err.Error(), "\n")
	diagnostics := make([]Diagnostic, 0, len(messages))
	for _, message := range messages {
		if message == "" {
			continue
		}
		diagnostics = append(diagnostics, Diagnostic{Message: message})
	}
	return diagnostics
}
