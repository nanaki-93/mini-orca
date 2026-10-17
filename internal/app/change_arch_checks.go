package app

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const maxFunctionComplexity = 15

type architectureImportRule struct {
	sourcePrefix string
	forbidden    []string
}

var defaultArchitectureImportRules = []architectureImportRule{
	{
		sourcePrefix: "internal/api",
		forbidden:    []string{"internal/storage", "internal/llm"},
	},
	{
		sourcePrefix: "internal/project",
		forbidden:    []string{"internal/app", "internal/api"},
	},
}

var backtickSpanPattern = regexp.MustCompile("`([^`]+)`")

func changeArchitectureAndComplexityChecks(root string, edits []ChangeEdit) []CheckResult {
	if len(edits) == 0 {
		return nil
	}
	rules := loadArchitectureImportRules(root)
	checks := []CheckResult{checkArchitectureBoundaries(edits, rules)}
	if changeHasGo(edits) {
		checks = append(checks, checkGoFunctionComplexity(edits))
	}
	return checks
}

func loadArchitectureImportRules(root string) []architectureImportRule {
	rules := append([]architectureImportRule(nil), defaultArchitectureImportRules...)
	if strings.TrimSpace(root) == "" {
		return rules
	}
	data, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		return rules
	}
	for _, raw := range strings.Split(string(data), "\n") {
		if parsed, ok := parseAgentsImportRuleLine(raw); ok {
			rules = append(rules, parsed)
		}
	}
	return rules
}

func parseAgentsImportRuleLine(line string) (architectureImportRule, bool) {
	lower := strings.ToLower(line)
	idx := strings.Index(lower, "must never import")
	if idx < 0 {
		return architectureImportRule{}, false
	}
	before := extractBacktickPaths(line[:idx])
	after := extractBacktickPaths(line[idx+len("must never import"):])
	if len(before) == 0 || len(after) == 0 {
		return architectureImportRule{}, false
	}
	return architectureImportRule{
		sourcePrefix: normalizeRulePathPrefix(before[0]),
		forbidden:    normalizeRulePaths(after),
	}, true
}

func extractBacktickPaths(segment string) []string {
	matches := backtickSpanPattern.FindAllStringSubmatch(segment, -1)
	var out []string
	for _, m := range matches {
		candidate := strings.TrimSpace(m[1])
		if strings.Contains(candidate, "/") || strings.Contains(candidate, ".") {
			out = append(out, candidate)
		}
	}
	return out
}

func normalizeRulePathPrefix(raw string) string {
	return strings.Trim(filepath.ToSlash(strings.TrimSpace(raw)), "/")
}

func normalizeRulePaths(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if norm := normalizeRulePathPrefix(item); norm != "" {
			out = append(out, norm)
		}
	}
	return out
}

func checkArchitectureBoundaries(edits []ChangeEdit, rules []architectureImportRule) CheckResult {
	var violations []string
	for _, edit := range edits {
		violations = append(violations, editArchitectureViolations(edit, rules)...)
	}
	if len(violations) > 0 {
		return CheckResult{
			Name:     "architecture boundaries",
			Required: true,
			State:    CheckFailed,
			Output:   strings.Join(violations, "\n"),
		}
	}
	return CheckResult{
		Name:     "architecture boundaries",
		Required: true,
		State:    CheckPassed,
		Output:   "All proposed imports respect architecture boundaries.",
	}
}

func editArchitectureViolations(edit ChangeEdit, rules []architectureImportRule) []string {
	slashPath := strings.TrimPrefix(filepath.ToSlash(edit.Path), "./")
	imports := extractEditImports(edit.Path, edit.Content)
	if len(imports) == 0 {
		return nil
	}
	var violations []string
	seen := map[string]bool{}
	for _, rule := range rules {
		if !pathMatchesPrefix(slashPath, rule.sourcePrefix) {
			continue
		}
		for _, imp := range imports {
			for _, forbidden := range rule.forbidden {
				if importMatchesForbidden(imp, forbidden) {
					msg := fmt.Sprintf("%s imports %q: violates architecture boundary (%s must never import %s)", slashPath, imp, rule.sourcePrefix, forbidden)
					if !seen[msg] {
						seen[msg] = true
						violations = append(violations, msg)
					}
				}
			}
		}
	}
	return violations
}

func pathMatchesPrefix(slashPath, prefix string) bool {
	if prefix == "" {
		return false
	}
	return slashPath == prefix || strings.HasPrefix(slashPath, prefix+"/")
}

func importMatchesForbidden(importPath, forbidden string) bool {
	normImp := strings.Trim(filepath.ToSlash(importPath), "/")
	if normImp == forbidden || strings.HasSuffix(normImp, "/"+forbidden) {
		return true
	}
	return strings.Contains(normImp, "/"+forbidden+"/")
}

func extractEditImports(path, content string) []string {
	if filepath.Ext(path) != ".go" {
		return nil
	}
	file, err := parser.ParseFile(token.NewFileSet(), path, content, parser.ImportsOnly)
	if err != nil || file == nil {
		return nil
	}
	imports := make([]string, 0, len(file.Imports))
	for _, spec := range file.Imports {
		if spec != nil && spec.Path != nil {
			imports = append(imports, strings.Trim(spec.Path.Value, "`\""))
		}
	}
	return imports
}

func checkGoFunctionComplexity(edits []ChangeEdit) CheckResult {
	var violations []string
	for _, edit := range edits {
		if filepath.Ext(edit.Path) != ".go" {
			continue
		}
		violations = append(violations, goEditComplexityViolations(edit.Path, edit.Content)...)
	}
	if len(violations) > 0 {
		return CheckResult{
			Name:     "complexity gate",
			Required: true,
			State:    CheckFailed,
			Output:   strings.Join(violations, "\n"),
		}
	}
	return CheckResult{
		Name:     "complexity gate",
		Required: true,
		State:    CheckPassed,
		Output:   fmt.Sprintf("All proposed Go functions are within the cyclomatic complexity budget (<= %d).", maxFunctionComplexity),
	}
}

func goEditComplexityViolations(path, content string) []string {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, content, 0)
	if err != nil || file == nil {
		return nil
	}
	var violations []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		complexity := astFunctionComplexity(fn.Body)
		if complexity > maxFunctionComplexity {
			line := fset.Position(fn.Pos()).Line
			violations = append(violations, fmt.Sprintf("%s:%d: function %s has cyclomatic complexity %d (max %d); split into smaller helpers", filepath.ToSlash(path), line, fn.Name.Name, complexity, maxFunctionComplexity))
		}
	}
	return violations
}

func astFunctionComplexity(body *ast.BlockStmt) int {
	complexity := 1
	ast.Inspect(body, func(node ast.Node) bool {
		if isComplexityBranchNode(node) {
			complexity++
		}
		return true
	})
	return complexity
}

func isComplexityBranchNode(node ast.Node) bool {
	switch n := node.(type) {
	case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt:
		return true
	case *ast.CaseClause:
		return len(n.List) > 0
	case *ast.CommClause:
		return n.Comm != nil
	case *ast.BinaryExpr:
		return n.Op == token.LAND || n.Op == token.LOR
	default:
		return false
	}
}
