package project

import (
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/scanner"
	"go/token"
	"strconv"
	"strings"
)

func extractGoFacts(filename string, source []byte) ([]string, []SymbolInfo, []Diagnostic) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, source, parser.ParseComments|parser.AllErrors)
	diagnostics := parseDiagnostics(err)
	if file == nil {
		return []string{}, []SymbolInfo{}, diagnostics
	}
	imports := make([]string, 0, len(file.Imports))
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err == nil {
			imports = append(imports, path)
		}
	}
	symbols := make([]SymbolInfo, 0)
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			name := declaration.Name.Name
			kind := "function"
			if declaration.Recv != nil && len(declaration.Recv.List) > 0 {
				kind = "method"
				name = receiverName(declaration.Recv.List[0].Type) + "." + name
			}
			symbols = append(symbols, newGoSymbol(fset, declaration.Pos(), declaration.End(), name, kind, formatNode(fset, declaration.Type), declaration.Name.Name, true))
		case *ast.GenDecl:
			for _, spec := range declaration.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					kind := "type"
					if _, ok := spec.Type.(*ast.StructType); ok {
						kind = "struct"
					}
					if _, ok := spec.Type.(*ast.InterfaceType); ok {
						kind = "interface"
					}
					symbols = append(symbols, newGoSymbol(fset, spec.Pos(), spec.End(), spec.Name.Name, kind, formatTypeSpec(fset, spec), spec.Name.Name, true))
				case *ast.ValueSpec:
					kind := "var"
					if declaration.Tok == token.CONST {
						kind = "const"
					}
					for _, name := range spec.Names {
						symbols = append(symbols, newGoSymbol(fset, name.Pos(), spec.End(), name.Name, kind, "", name.Name, false))
					}
				}
			}
		}
	}
	return imports, symbols, diagnostics
}

func newGoSymbol(fset *token.FileSet, start, end token.Pos, name, kind, signature, visibilityName string, atomicTarget bool) SymbolInfo {
	return SymbolInfo{
		Name: name, Kind: kind, Signature: signature, StartLine: fset.Position(start).Line, EndLine: fset.Position(end).Line,
		Visibility: visibility(visibilityName), Confidence: "exact", AtomicTarget: atomicTarget,
	}
}

func receiverName(expression ast.Expr) string {
	switch expression := expression.(type) {
	case *ast.Ident:
		return expression.Name
	case *ast.StarExpr:
		return receiverName(expression.X)
	case *ast.IndexExpr:
		return receiverName(expression.X)
	case *ast.IndexListExpr:
		return receiverName(expression.X)
	case *ast.ParenExpr:
		return receiverName(expression.X)
	default:
		return formatNode(token.NewFileSet(), expression)
	}
}

func formatTypeSpec(fset *token.FileSet, spec *ast.TypeSpec) string {
	var output bytes.Buffer
	if err := printer.Fprint(&output, fset, spec); err != nil {
		return ""
	}
	return output.String()
}

func formatNode(fset *token.FileSet, node ast.Node) string {
	var output bytes.Buffer
	if err := printer.Fprint(&output, fset, node); err != nil {
		return ""
	}
	return output.String()
}

func visibility(name string) string {
	if ast.IsExported(name) {
		return "exported"
	}
	return "unexported"
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
