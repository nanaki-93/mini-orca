package syntax

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// Symbol is one declaration of a file's outline: a function, method, type,
// class or interface, with the first line of its declaration.
type Symbol struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	Line int    `json:"line"`
	// Signature is the declaration's first line, without its body.
	Signature string `json:"signature"`
}

const maxSignature = 160

// Outline lists the top-level and nested declarations of a source file, in
// source order. It returns nothing for a file no grammar outlines.
func Outline(path string, source []byte) []Symbol {
	if len(source) > maxSourceBytes {
		return nil
	}
	if strings.EqualFold(filepath.Ext(path), ".go") {
		return goOutline(path, source)
	}
	entry := grammars.DetectLanguage(filepath.Base(path))
	if entry == nil {
		return nil
	}
	g, err := loadGrammar(entry)
	if err != nil {
		return nil
	}
	parser := gotreesitter.NewParser(g.language)
	parser.SetTimeoutMicros(parseTimeoutMicros)
	parser.SetMemoryBudgetBytes(parseMemoryBudget)
	tree, err := parser.ParseStrict(source)
	if tree != nil {
		defer tree.Release()
	}
	if err != nil {
		return nil
	}
	symbols, _ := g.outliner.OutlineTree(tree)
	var out []Symbol
	collectSymbols(symbols, "", source, &out)
	return out
}

func collectSymbols(symbols []gotreesitter.OutlineSymbol, prefix string, source []byte, out *[]Symbol) {
	for _, symbol := range symbols {
		name := symbol.Name
		if prefix != "" {
			name = prefix + "." + name
		}
		*out = append(*out, Symbol{Kind: symbol.Kind, Name: name, Line: int(symbol.Range.StartPoint.Row) + 1,
			Signature: signature(string(source[symbol.Range.StartByte:symbol.Range.EndByte]))})
		collectSymbols(symbol.Children, name, source, out)
	}
}

// signature keeps a declaration's first line up to where its body starts.
func signature(text string) string {
	line := strings.TrimSpace(strings.SplitN(text, "\n", 2)[0])
	if i := strings.Index(line, "{"); i > 0 {
		line = strings.TrimSpace(line[:i])
	}
	if len(line) > maxSignature {
		line = strings.TrimSpace(line[:maxSignature]) + "…"
	}
	return line
}

func goOutline(path string, source []byte) []Symbol {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, source, parser.SkipObjectResolution)
	if file == nil || err != nil && len(file.Decls) == 0 {
		return nil
	}
	text := func(node ast.Node) string {
		start, end := fset.Position(node.Pos()).Offset, fset.Position(node.End()).Offset
		if start < 0 || end > len(source) || start >= end {
			return ""
		}
		return signature(string(source[start:end]))
	}
	var out []Symbol
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			kind, name := "function", d.Name.Name
			if receiver := goReceiverType(d); receiver != "" {
				kind, name = "method", receiver+"."+name
			}
			out = append(out, Symbol{Kind: kind, Name: name, Line: fset.Position(d.Pos()).Line, Signature: text(d)})
		case *ast.GenDecl:
			out = append(out, goGenSymbols(d, fset, text)...)
		}
	}
	return out
}

func goGenSymbols(decl *ast.GenDecl, fset *token.FileSet, text func(ast.Node) string) []Symbol {
	var out []Symbol
	for _, spec := range decl.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			out = append(out, Symbol{Kind: "type", Name: s.Name.Name, Line: fset.Position(s.Pos()).Line, Signature: "type " + text(s)})
		case *ast.ValueSpec:
			for _, name := range s.Names {
				if name.IsExported() {
					out = append(out, Symbol{Kind: strings.ToLower(decl.Tok.String()), Name: name.Name, Line: fset.Position(name.Pos()).Line, Signature: decl.Tok.String() + " " + text(s)})
				}
			}
		}
	}
	return out
}
