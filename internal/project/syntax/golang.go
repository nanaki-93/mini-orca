package syntax

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
)

func analyzeGo(path string, source []byte) File {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, source, parser.SkipObjectResolution)
	result := File{Grammar: "go", Measured: true}
	if err != nil {
		result.SyntaxError = goSyntaxError(err)
	}
	if file == nil {
		result.Measured = false
		result.Reason = "file does not parse"
		return result
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		start, end := fset.Position(fn.Pos()), fset.Position(fn.End())
		kind, name := "function", fn.Name.Name
		if receiver := goReceiverType(fn); receiver != "" {
			kind, name = "method", receiver+"."+name
		}
		result.Functions = append(result.Functions, Function{
			Name: name, Kind: kind, StartLine: start.Line, EndLine: end.Line,
			Complexity: goComplexity(fn.Body),
			Hash:       textHash(string(source[start.Offset:end.Offset])),
		})
	}
	return result
}

func goSyntaxError(err error) string {
	if list, ok := err.(scanner.ErrorList); ok && len(list) > 0 {
		return fmt.Sprintf("line %d: %s", list[0].Pos.Line, list[0].Msg)
	}
	return err.Error()
}

func goReceiverType(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	expr := fn.Recv.List[0].Type
	for {
		switch typed := expr.(type) {
		case *ast.StarExpr:
			expr = typed.X
		case *ast.IndexExpr:
			expr = typed.X
		case *ast.IndexListExpr:
			expr = typed.X
		case *ast.ParenExpr:
			expr = typed.X
		case *ast.Ident:
			return typed.Name
		default:
			return ""
		}
	}
}

// goComplexity counts decision points the way gocyclo does, so the gate and
// the repository's quality script agree on Go code.
func goComplexity(body *ast.BlockStmt) int {
	complexity := 1
	ast.Inspect(body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt:
			complexity++
		case *ast.CaseClause:
			if len(n.List) > 0 {
				complexity++
			}
		case *ast.CommClause:
			if n.Comm != nil {
				complexity++
			}
		case *ast.BinaryExpr:
			if n.Op == token.LAND || n.Op == token.LOR {
				complexity++
			}
		}
		return true
	})
	return complexity
}
