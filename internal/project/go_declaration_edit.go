package project

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/workflow"
)

// DeclarationEditMode identifies whether a draft replaces an exact symbol or
// creates a new top-level symbol in the open Go file.
type DeclarationEditMode string

const (
	DeclarationEditReplaceSymbol DeclarationEditMode = "replace_symbol"
	DeclarationEditCreateSymbol  DeclarationEditMode = "create_symbol"
)

// GoDeclarationEdit contains the isolated, editable part of a Go candidate.
// Imports are Go import specs: a bare path such as "fmt", a quoted path, or an
// aliased spec such as `alias "example.com/package"`.
type GoDeclarationEdit struct {
	Mode         DeclarationEditMode `json:"mode"`
	TargetSymbol string              `json:"target_symbol"`
	Declaration  string              `json:"declaration"`
	Imports      []string            `json:"imports,omitempty"`
}

// GoDeclarationComposition is an in-memory, preview-only candidate. Invalid
// edits return diagnostics and never produce a candidate or candidate hash.
type GoDeclarationComposition struct {
	NormalizedDeclaration string               `json:"normalized_declaration,omitempty"`
	CandidateContent      string               `json:"candidate_content,omitempty"`
	CandidateHash         string               `json:"candidate_hash,omitempty"`
	Validation            GenerationValidation `json:"validation"`
}

// ComposeGoDeclaration composes one isolated Go declaration into a complete
// file. It accepts exact functions, methods, types, and single-name top-level
// variables, and proves that every unrelated declaration and every pre-existing
// import remains unchanged.
func ComposeGoDeclaration(path, original string, edit GoDeclarationEdit) GoDeclarationComposition {
	result := GoDeclarationComposition{Validation: GenerationValidation{ScopeMode: workflow.ScopeSymbolPlusImports}}
	if !validDeclarationEditMode(edit.Mode) {
		return invalidDeclarationComposition(result, "invalid_mode", "The requested declaration edit mode is not supported.")
	}
	if !validDeclarationTarget(edit.TargetSymbol) {
		return invalidDeclarationComposition(result, "invalid_target", "The requested symbol name is not a supported Go declaration name.")
	}

	fset := token.NewFileSet()
	before, err := parser.ParseFile(fset, path, original, parser.ParseComments)
	if err != nil {
		return invalidDeclarationComposition(result, "original_syntax", "The original Go file cannot be parsed for declaration composition.")
	}
	declaration, normalized, err := parseIsolatedGoDeclaration(edit.Declaration, edit.Mode)
	if err != nil {
		return invalidDeclarationComposition(result, "invalid_declaration", err.Error())
	}
	if declarationName(declaration) != edit.TargetSymbol {
		return invalidDeclarationComposition(result, "target_mismatch", "The declaration name does not match the requested symbol.")
	}
	requestedImports, err := normalizeRequestedImports(edit.Imports)
	if err != nil {
		return invalidDeclarationComposition(result, "invalid_import", err.Error())
	}
	if err := validateEditTarget(before, edit); err != nil {
		return invalidDeclarationComposition(result, "target_identity", err.Error())
	}
	if err := validateReplacementKind(before, edit, declaration); err != nil {
		return invalidDeclarationComposition(result, "target_kind", err.Error())
	}

	candidateSource, err := composeDeclarationSource(original, fset, before, declaration, normalized, edit.Mode, requestedImports)
	if err != nil {
		return invalidDeclarationComposition(result, "compose_failed", err.Error())
	}
	candidate, err := format.Source(candidateSource)
	if err != nil {
		return invalidDeclarationComposition(result, "candidate_syntax", "The composed Go candidate cannot be formatted.")
	}
	result.NormalizedDeclaration = normalized
	result.CandidateContent = string(candidate)
	result.CandidateHash = declarationCandidateHash(result.CandidateContent)
	result.Validation = validateGoDeclarationComposition(path, original, result.CandidateContent, edit, requestedImports)
	if !result.Validation.Applicable {
		result.CandidateContent = ""
		result.CandidateHash = ""
	}
	return result
}

func validDeclarationEditMode(mode DeclarationEditMode) bool {
	return mode == DeclarationEditReplaceSymbol || mode == DeclarationEditCreateSymbol
}

func validDeclarationTarget(target string) bool {
	parts := strings.Split(target, ".")
	if len(parts) < 1 || len(parts) > 2 {
		return false
	}
	for _, part := range parts {
		if !token.IsIdentifier(part) || token.Lookup(part).IsKeyword() {
			return false
		}
	}
	return true
}

func parseIsolatedGoDeclaration(source string, mode DeclarationEditMode) (ast.Decl, string, error) {
	if strings.TrimSpace(source) == "" {
		return nil, "", fmt.Errorf("a Go declaration is required")
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "declaration.go", "package declaration\n\n"+source, parser.ParseComments)
	if err != nil {
		return nil, "", fmt.Errorf("the declaration is not valid Go")
	}
	if len(file.Decls) != 1 {
		return nil, "", fmt.Errorf("exactly one Go declaration is required")
	}
	declaration := file.Decls[0]
	switch declaration := declaration.(type) {
	case *ast.FuncDecl:
		if declaration.Name == nil || declaration.Name.Name == "" {
			return nil, "", fmt.Errorf("the function or method declaration has no name")
		}
	case *ast.GenDecl:
		switch declaration.Tok {
		case token.TYPE:
			if len(declaration.Specs) != 1 {
				return nil, "", fmt.Errorf("only one type declaration is supported")
			}
			if _, ok := declaration.Specs[0].(*ast.TypeSpec); !ok {
				return nil, "", fmt.Errorf("only type declarations are supported")
			}
		case token.VAR:
			if mode != DeclarationEditReplaceSymbol || !isAtomicVariableDeclaration(declaration, valueSpec(declaration)) {
				return nil, "", fmt.Errorf("only one existing top-level variable declaration is supported")
			}
		default:
			return nil, "", fmt.Errorf("only function, method, type, and single top-level variable declarations are supported")
		}
	default:
		return nil, "", fmt.Errorf("only function, method, type, and single top-level variable declarations are supported")
	}
	var output bytes.Buffer
	if err := format.Node(&output, fset, declaration); err != nil {
		return nil, "", fmt.Errorf("format declaration: %w", err)
	}
	return declaration, strings.TrimSpace(output.String()), nil
}

func valueSpec(declaration *ast.GenDecl) *ast.ValueSpec {
	if len(declaration.Specs) != 1 {
		return nil
	}
	spec, _ := declaration.Specs[0].(*ast.ValueSpec)
	return spec
}

func declarationName(declaration ast.Decl) string {
	switch declaration := declaration.(type) {
	case *ast.FuncDecl:
		if declaration.Recv == nil || len(declaration.Recv.List) == 0 {
			return declaration.Name.Name
		}
		return receiverName(declaration.Recv.List[0].Type) + "." + declaration.Name.Name
	case *ast.GenDecl:
		if len(declaration.Specs) != 1 {
			return ""
		}
		if typeSpec, ok := declaration.Specs[0].(*ast.TypeSpec); declaration.Tok == token.TYPE && ok {
			return typeSpec.Name.Name
		}
		if variable := valueSpec(declaration); isAtomicVariableDeclaration(declaration, variable) {
			return variable.Names[0].Name
		}
		return ""
	default:
		return ""
	}
}

func validateEditTarget(file *ast.File, edit GoDeclarationEdit) error {
	count := countNamedDeclarations(file, edit.TargetSymbol)
	switch edit.Mode {
	case DeclarationEditReplaceSymbol:
		if count != 1 {
			return fmt.Errorf("replace mode requires exactly one existing requested symbol")
		}
	case DeclarationEditCreateSymbol:
		if count != 0 {
			return fmt.Errorf("create mode requires the requested symbol to be absent")
		}
	}
	return nil
}

func validateReplacementKind(file *ast.File, edit GoDeclarationEdit, replacement ast.Decl) error {
	if edit.Mode != DeclarationEditReplaceSymbol {
		return nil
	}
	location, found := declarationLocation(file, edit.TargetSymbol)
	if !found || replaceableDeclarationKind(location.node) == "" {
		return fmt.Errorf("replace mode requires an existing supported declaration")
	}
	if replaceableDeclarationKind(location.node) != replaceableDeclarationKind(replacement) {
		return fmt.Errorf("the replacement must preserve the requested declaration kind")
	}
	return nil
}

func replaceableDeclarationKind(node ast.Node) string {
	switch declaration := node.(type) {
	case *ast.FuncDecl:
		return "function"
	case *ast.TypeSpec:
		return "type"
	case *ast.GenDecl:
		if declaration.Tok == token.TYPE && len(declaration.Specs) == 1 {
			if _, ok := declaration.Specs[0].(*ast.TypeSpec); ok {
				return "type"
			}
		}
		if isAtomicVariableDeclaration(declaration, valueSpec(declaration)) {
			return "var"
		}
	}
	return ""
}

func composeDeclarationSource(original string, fset *token.FileSet, file *ast.File, replacement ast.Decl, replacementText string, mode DeclarationEditMode, imports []string) ([]byte, error) {
	source := original
	if mode == DeclarationEditReplaceSymbol {
		location, found := declarationLocation(file, declarationName(replacement))
		if !found {
			return nil, fmt.Errorf("requested declaration was not found")
		}
		text := replacementText
		if location.groupedType {
			var err error
			text, err = typeSpecText(replacementText)
			if err != nil {
				return nil, err
			}
		}
		start := fset.Position(declarationStart(location.node)).Offset
		end := fset.Position(location.node.End()).Offset
		source = source[:start] + text + source[end:]
	} else {
		source = appendDeclaration(source, replacementText)
	}
	if len(imports) == 0 {
		return []byte(source), nil
	}
	return []byte(insertRequestedImports(source, fset, file, imports)), nil
}

func declarationStart(node ast.Node) token.Pos {
	switch declaration := node.(type) {
	case *ast.FuncDecl:
		if declaration.Doc != nil {
			return declaration.Doc.Pos()
		}
	case *ast.GenDecl:
		if declaration.Doc != nil {
			return declaration.Doc.Pos()
		}
		if declaration.Tok == token.TYPE && len(declaration.Specs) == 1 {
			if spec, ok := declaration.Specs[0].(*ast.TypeSpec); ok && spec.Doc != nil {
				return spec.Doc.Pos()
			}
		}
	case *ast.TypeSpec:
		if declaration.Doc != nil {
			return declaration.Doc.Pos()
		}
	}
	return node.Pos()
}

type declarationSourceLocation struct {
	node        ast.Node
	groupedType bool
}

func declarationLocation(file *ast.File, target string) (declarationSourceLocation, bool) {
	for _, declaration := range file.Decls {
		if declarationNameOf(declaration) == target {
			return declarationSourceLocation{node: declaration}, true
		}
		group, ok := declaration.(*ast.GenDecl)
		if !ok || group.Tok != token.TYPE || len(group.Specs) == 1 {
			continue
		}
		for _, spec := range group.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if ok && typeSpec.Name.Name == target {
				return declarationSourceLocation{node: typeSpec, groupedType: true}, true
			}
		}
	}
	return declarationSourceLocation{}, false
}

func typeSpecText(normalizedDeclaration string) (string, error) {
	fset := token.NewFileSet()
	const packagePrefix = "package declaration\n\n"
	file, err := parser.ParseFile(fset, "declaration.go", packagePrefix+normalizedDeclaration, parser.ParseComments)
	if err != nil || len(file.Decls) != 1 {
		return "", fmt.Errorf("the replacement type declaration cannot be parsed")
	}
	declaration, ok := file.Decls[0].(*ast.GenDecl)
	if !ok || declaration.Tok != token.TYPE || len(declaration.Specs) != 1 {
		return "", fmt.Errorf("the replacement must be a single type declaration")
	}
	typeOffset := fset.Position(declaration.TokPos).Offset - len(packagePrefix)
	specOffset := fset.Position(declaration.Specs[0].Pos()).Offset - len(packagePrefix)
	endOffset := fset.Position(declaration.End()).Offset - len(packagePrefix)
	if typeOffset < 0 || specOffset < typeOffset || endOffset > len(normalizedDeclaration) {
		return "", fmt.Errorf("the replacement type declaration has invalid source positions")
	}
	return strings.TrimSpace(normalizedDeclaration[:typeOffset] + normalizedDeclaration[specOffset:endOffset]), nil
}

func declarationNameOf(declaration ast.Decl) string {
	switch declaration := declaration.(type) {
	case *ast.FuncDecl:
		if declaration.Recv == nil || len(declaration.Recv.List) == 0 {
			return declaration.Name.Name
		}
		return receiverName(declaration.Recv.List[0].Type) + "." + declaration.Name.Name
	case *ast.GenDecl:
		if declaration.Tok == token.TYPE && len(declaration.Specs) == 1 {
			if spec, ok := declaration.Specs[0].(*ast.TypeSpec); ok {
				return spec.Name.Name
			}
		}
		if variable := valueSpec(declaration); isAtomicVariableDeclaration(declaration, variable) {
			return variable.Names[0].Name
		}
	}
	return ""
}

func appendDeclaration(source, declaration string) string {
	return strings.TrimRight(source, "\n") + "\n\n" + declaration + "\n"
}

func insertRequestedImports(source string, fset *token.FileSet, file *ast.File, requested []string) string {
	existing := make(map[string]bool, len(file.Imports))
	for _, spec := range file.Imports {
		existing[renderGoNode(fset, spec)] = true
	}
	newImports := make([]string, 0, len(requested))
	for _, spec := range requested {
		if !existing[spec] {
			newImports = append(newImports, spec)
		}
	}
	if len(newImports) == 0 {
		return source
	}
	block := "\n\nimport (\n\t" + strings.Join(newImports, "\n\t") + "\n)"
	offset := fset.Position(file.Name.End()).Offset
	for _, declaration := range file.Decls {
		if importDeclaration, ok := declaration.(*ast.GenDecl); ok && importDeclaration.Tok == token.IMPORT {
			offset = fset.Position(importDeclaration.End()).Offset
		}
	}
	return source[:offset] + block + source[offset:]
}

func normalizeRequestedImports(imports []string) ([]string, error) {
	if len(imports) == 0 {
		return nil, nil
	}
	inputs := make([]string, 0, len(imports))
	for _, value := range imports {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return nil, fmt.Errorf("an import cannot be empty")
		}
		if !strings.Contains(trimmed, `"`) {
			if strings.ContainsAny(trimmed, " \t\r\n") {
				return nil, fmt.Errorf("import %q must be a Go import spec", value)
			}
			trimmed = strconv.Quote(trimmed)
		}
		inputs = append(inputs, trimmed)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "imports.go", "package declaration\nimport (\n"+strings.Join(inputs, "\n")+"\n)\n", 0)
	if err != nil || len(file.Decls) != 1 {
		return nil, fmt.Errorf("requested imports are not valid Go import specs")
	}
	declaration, ok := file.Decls[0].(*ast.GenDecl)
	if !ok || declaration.Tok != token.IMPORT || len(declaration.Specs) != len(inputs) {
		return nil, fmt.Errorf("requested imports are not valid Go import specs")
	}
	unique := make(map[string]bool, len(inputs))
	normalized := make([]string, 0, len(inputs))
	for _, spec := range declaration.Specs {
		value := renderGoNode(fset, spec)
		if unique[value] {
			return nil, fmt.Errorf("requested imports contain a duplicate")
		}
		unique[value] = true
		normalized = append(normalized, value)
	}
	sort.Strings(normalized)
	return normalized, nil
}

func validateGoDeclarationComposition(path, original, candidate string, edit GoDeclarationEdit, requestedImports []string) GenerationValidation {
	validation := GenerationValidation{ScopeMode: workflow.ScopeSymbolPlusImports, Diff: buildUnifiedDiff(path, original, candidate)}
	fset := token.NewFileSet()
	before, err := parser.ParseFile(fset, path, original, parser.ParseComments)
	if err != nil {
		return invalidGeneration(validation, "original_syntax", "The original Go file cannot be parsed for declaration composition.")
	}
	after, err := parser.ParseFile(fset, path, candidate, parser.ParseComments)
	if err != nil {
		return invalidGeneration(validation, "candidate_syntax", "The composed Go candidate has invalid syntax.")
	}
	if before.Name.Name != after.Name.Name {
		return invalidGeneration(validation, "package_changed", "The candidate changes the Go package.")
	}
	beforeCount := countNamedDeclarations(before, edit.TargetSymbol)
	afterCount := countNamedDeclarations(after, edit.TargetSymbol)
	if (edit.Mode == DeclarationEditReplaceSymbol && (beforeCount != 1 || afterCount != 1)) ||
		(edit.Mode == DeclarationEditCreateSymbol && (beforeCount != 0 || afterCount != 1)) {
		return invalidGeneration(validation, "target_identity", "The candidate does not preserve the requested declaration identity.")
	}
	beforeDeclarations, err := goDeclarations(fset, before)
	if err != nil {
		return invalidGeneration(validation, "original_declarations", err.Error())
	}
	afterDeclarations, err := goDeclarations(fset, after)
	if err != nil {
		return invalidGeneration(validation, "candidate_declarations", err.Error())
	}
	if !sameNonTargetDeclarations(beforeDeclarations, afterDeclarations, edit.TargetSymbol) || !sameNonTargetDeclarationOrder(before, after, edit.TargetSymbol) {
		return invalidGeneration(validation, "out_of_scope_declaration", "The candidate changes, reorders, removes, or duplicates an unrelated declaration.")
	}
	if !importsPreservedWithRequests(fset, before, after, requestedImports) {
		return invalidGeneration(validation, "out_of_scope_import", "The candidate changes an existing import or adds an import that was not requested.")
	}
	validation.Applicable = true
	return validation
}

func countNamedDeclarations(file *ast.File, target string) int {
	count := 0
	for _, declaration := range file.Decls {
		if declarationNameOf(declaration) == target {
			count++
			continue
		}
		group, ok := declaration.(*ast.GenDecl)
		if !ok || group.Tok != token.TYPE {
			continue
		}
		for _, spec := range group.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if ok && typeSpec.Name.Name == target {
				count++
			}
		}
	}
	return count
}

func sameNonTargetDeclarationOrder(before, after *ast.File, target string) bool {
	left, right := nonTargetDeclarationNames(before, target), nonTargetDeclarationNames(after, target)
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func nonTargetDeclarationNames(file *ast.File, target string) []string {
	names := make([]string, 0, len(file.Decls))
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			name := declarationNameOf(declaration)
			if name != target {
				names = append(names, name)
			}
		case *ast.GenDecl:
			if declaration.Tok != token.TYPE && declaration.Tok != token.VAR && declaration.Tok != token.CONST {
				continue
			}
			for _, spec := range declaration.Specs {
				for _, name := range declarationNames(spec) {
					if name != target {
						names = append(names, name)
					}
				}
			}
		}
	}
	return names
}

func importsPreservedWithRequests(fset *token.FileSet, before, after *ast.File, requested []string) bool {
	beforeImports := importSpecs(fset, before)
	requestedSet := make(map[string]bool, len(requested))
	for _, spec := range requested {
		if !containsImportSpec(beforeImports, spec) {
			requestedSet[spec] = true
		}
	}
	beforeIndex := 0
	for _, spec := range importSpecs(fset, after) {
		if beforeIndex < len(beforeImports) && spec == beforeImports[beforeIndex] {
			beforeIndex++
			continue
		}
		if !requestedSet[spec] {
			return false
		}
		delete(requestedSet, spec)
	}
	return beforeIndex == len(beforeImports) && len(requestedSet) == 0
}

func importSpecs(fset *token.FileSet, file *ast.File) []string {
	specs := make([]string, 0, len(file.Imports))
	for _, spec := range file.Imports {
		specs = append(specs, renderGoNode(fset, spec))
	}
	return specs
}

func containsImportSpec(imports []string, want string) bool {
	for _, spec := range imports {
		if spec == want {
			return true
		}
	}
	return false
}

func declarationCandidateHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func invalidDeclarationComposition(result GoDeclarationComposition, code, message string) GoDeclarationComposition {
	result.Validation = invalidGeneration(result.Validation, code, message)
	return result
}
