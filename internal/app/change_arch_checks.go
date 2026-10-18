package app

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

const maxFunctionComplexity = 15

type architectureImportRule struct {
	sourcePrefix string
	forbidden    []string
}

type architectureQualityBudgets struct {
	maxComplexity    int
	maxFunctionLines int
	maxFileLines     int
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

type parsedEditFacts struct {
	lang       string
	rawImports []string
	symbols    []project.SymbolInfo
}

var (
	backtickSpanPattern    = regexp.MustCompile("`([^`]+)`")
	plainPathTokenPattern  = regexp.MustCompile(`([A-Za-z0-9_./-]+)`)
	budgetNumberPattern    = regexp.MustCompile(`(\d+)`)
	commonJSRequirePattern = regexp.MustCompile(`\brequire\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	branchKeywordPattern   = regexp.MustCompile(`\b(if|elif|for|while|case|catch)\b`)
)

func changeArchitectureAndComplexityChecks(root string, edits []ChangeEdit) []CheckResult {
	if len(edits) == 0 {
		return nil
	}
	factsByPath := extractAllEditFacts(edits)
	rules, budgets := loadArchitecturePolicy(root)
	checks := []CheckResult{checkArchitectureBoundaries(edits, rules, factsByPath)}
	if changeHasCodeSource(edits, factsByPath) || budgets.maxFileLines > 0 {
		checks = append(checks, checkFunctionComplexityAndBudgets(edits, budgets, factsByPath))
	}
	return checks
}

func extractAllEditFacts(edits []ChangeEdit) map[string]parsedEditFacts {
	facts := make(map[string]parsedEditFacts, len(edits))
	for _, edit := range edits {
		if filepath.Ext(edit.Path) == ".go" {
			continue
		}
		lang, rawImports, symbols := project.ExtractFileFacts(edit.Path, []byte(edit.Content))
		facts[edit.Path] = parsedEditFacts{
			lang:       lang,
			rawImports: rawImports,
			symbols:    symbols,
		}
	}
	return facts
}

func loadArchitecturePolicy(root string) ([]architectureImportRule, architectureQualityBudgets) {
	var rules []architectureImportRule
	for _, r := range defaultArchitectureImportRules {
		rules = appendUniqueImportRule(rules, r)
	}
	budgets := architectureQualityBudgets{maxComplexity: maxFunctionComplexity}
	if strings.TrimSpace(root) == "" {
		return rules, budgets
	}
	data, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		return rules, budgets
	}
	for _, raw := range strings.Split(string(data), "\n") {
		if parsed, ok := parseAgentsImportRuleLine(raw); ok {
			rules = appendUniqueImportRule(rules, parsed)
		}
		parseAgentsBudgetLine(raw, &budgets)
	}
	return rules, budgets
}

func appendUniqueImportRule(rules []architectureImportRule, candidate architectureImportRule) []architectureImportRule {
	if candidate.sourcePrefix == "" || len(candidate.forbidden) == 0 {
		return rules
	}
	for i := range rules {
		if rules[i].sourcePrefix == candidate.sourcePrefix {
			for _, f := range candidate.forbidden {
				if !slices.Contains(rules[i].forbidden, f) {
					rules[i].forbidden = append(rules[i].forbidden, f)
				}
			}
			return rules
		}
	}
	return append(rules, architectureImportRule{
		sourcePrefix: candidate.sourcePrefix,
		forbidden:    append([]string(nil), candidate.forbidden...),
	})
}

func parseAgentsBudgetLine(line string, budgets *architectureQualityBudgets) {
	lower := strings.ToLower(line)
	match := budgetNumberPattern.FindStringSubmatch(lower)
	if len(match) != 2 {
		return
	}
	val, err := strconv.Atoi(match[1])
	if err != nil || val <= 0 {
		return
	}
	switch {
	case strings.Contains(lower, "max function complexity"):
		budgets.maxComplexity = val
	case strings.Contains(lower, "max function lines"):
		budgets.maxFunctionLines = val
	case strings.Contains(lower, "max file lines"):
		budgets.maxFileLines = val
	}
}

func parseAgentsImportRuleLine(line string) (architectureImportRule, bool) {
	lower := strings.ToLower(line)
	idx := strings.Index(lower, "must never import")
	if idx < 0 {
		return architectureImportRule{}, false
	}
	before := extractRulePaths(line[:idx])
	after := extractRulePaths(line[idx+len("must never import"):])
	if len(before) == 0 || len(after) == 0 {
		return architectureImportRule{}, false
	}
	source := normalizeRulePathPrefix(before[0])
	forbidden := normalizeRulePaths(after)
	if source == "" || len(forbidden) == 0 {
		return architectureImportRule{}, false
	}
	return architectureImportRule{
		sourcePrefix: source,
		forbidden:    forbidden,
	}, true
}

func extractRulePaths(segment string) []string {
	var out []string
	for _, m := range backtickSpanPattern.FindAllStringSubmatch(segment, -1) {
		candidate := strings.Trim(strings.TrimSpace(m[1]), ".,;:")
		if candidate != "" && !strings.ContainsAny(candidate, " \t") {
			out = append(out, candidate)
		}
	}
	if len(out) > 0 {
		return out
	}
	for _, tokenMatch := range plainPathTokenPattern.FindAllStringSubmatch(segment, -1) {
		tok := strings.Trim(strings.TrimSpace(tokenMatch[1]), ".,;:-")
		if strings.Contains(tok, "/") || strings.Contains(tok, ".") {
			out = append(out, tok)
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
		if norm := normalizeRulePathPrefix(item); norm != "" && !slices.Contains(out, norm) {
			out = append(out, norm)
		}
	}
	return out
}

func checkArchitectureBoundaries(edits []ChangeEdit, rules []architectureImportRule, factsByPath map[string]parsedEditFacts) CheckResult {
	var violations []string
	for _, edit := range edits {
		violations = append(violations, editArchitectureViolations(edit, rules, factsByPath[edit.Path])...)
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

func editArchitectureViolations(edit ChangeEdit, rules []architectureImportRule, facts parsedEditFacts) []string {
	slashPath := strings.TrimPrefix(filepath.ToSlash(edit.Path), "./")
	imports := extractEditImports(edit.Path, edit.Content, facts.rawImports)
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
	if normImp == forbidden || strings.HasPrefix(normImp, forbidden+"/") || strings.HasSuffix(normImp, "/"+forbidden) {
		return true
	}
	return strings.Contains(normImp, "/"+forbidden+"/")
}

var rawQuotedImportSpec = regexp.MustCompile(`['"<]([^'">]+)['">]`)

func extractEditImports(path, content string, rawImports []string) []string {
	if filepath.Ext(path) == ".go" {
		return extractGoEditImports(path, content)
	}
	slashDir := filepath.Dir(strings.TrimPrefix(filepath.ToSlash(path), "./"))
	if rawImports == nil {
		_, rawImports, _ = project.ExtractFileFacts(path, []byte(content))
	}
	imports := make([]string, 0, len(rawImports)*2)
	appendNorm := func(raw string) {
		if m := rawQuotedImportSpec.FindStringSubmatch(raw); len(m) == 2 {
			spec := strings.ReplaceAll(strings.TrimSpace(m[1]), "\\", "/")
			if strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") {
				resolved := filepath.ToSlash(filepath.Clean(filepath.Join(slashDir, spec)))
				if resolved != "" && resolved != "." {
					imports = append(imports, resolved)
				}
			}
		}
		if norm := project.NormalizeGenericImportPath(raw); norm != "" {
			imports = append(imports, norm)
		}
	}
	for _, raw := range rawImports {
		appendNorm(raw)
	}
	for _, m := range commonJSRequirePattern.FindAllStringSubmatch(content, -1) {
		if len(m) == 2 {
			appendNorm(`"` + m[1] + `"`)
		}
	}
	return imports
}

func extractGoEditImports(path, content string) []string {
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

func changeHasCodeSource(edits []ChangeEdit, factsByPath map[string]parsedEditFacts) bool {
	if changeHasGo(edits) {
		return true
	}
	for _, edit := range edits {
		f := factsByPath[edit.Path]
		if f.lang != "" && f.lang != "Markdown" && f.lang != "Text" && len(f.symbols) > 0 {
			return true
		}
	}
	return false
}

func checkFunctionComplexityAndBudgets(edits []ChangeEdit, budgets architectureQualityBudgets, factsByPath map[string]parsedEditFacts) CheckResult {
	var violations []string
	for _, edit := range edits {
		violations = append(violations, fileLineBudgetViolations(edit, budgets)...)
		if filepath.Ext(edit.Path) == ".go" {
			violations = append(violations, goEditComplexityViolations(edit.Path, edit.Content, budgets)...)
		} else {
			violations = append(violations, genericEditComplexityViolations(edit.Path, edit.Content, budgets, factsByPath[edit.Path].symbols)...)
		}
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
		Output:   fmt.Sprintf("All proposed functions are within the cyclomatic complexity budget (<= %d).", budgets.maxComplexity),
	}
}

func fileLineBudgetViolations(edit ChangeEdit, budgets architectureQualityBudgets) []string {
	if budgets.maxFileLines <= 0 {
		return nil
	}
	lines := strings.Count(edit.Content, "\n") + 1
	if lines <= budgets.maxFileLines {
		return nil
	}
	return []string{fmt.Sprintf("%s: file has %d lines (max %d); split into cohesive modules", filepath.ToSlash(edit.Path), lines, budgets.maxFileLines)}
}

func goEditComplexityViolations(path, content string, budgets architectureQualityBudgets) []string {
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
		startLine := fset.Position(fn.Pos()).Line
		endLine := fset.Position(fn.End()).Line
		complexity := astFunctionComplexity(fn.Body)
		if complexity > budgets.maxComplexity {
			violations = append(violations, fmt.Sprintf("%s:%d: function %s has cyclomatic complexity %d (max %d); split into smaller helpers", filepath.ToSlash(path), startLine, fn.Name.Name, complexity, budgets.maxComplexity))
		}
		if budgets.maxFunctionLines > 0 && (endLine-startLine+1) > budgets.maxFunctionLines {
			violations = append(violations, fmt.Sprintf("%s:%d: function %s has %d lines (max %d); split into smaller helpers", filepath.ToSlash(path), startLine, fn.Name.Name, endLine-startLine+1, budgets.maxFunctionLines))
		}
	}
	return violations
}

func genericEditComplexityViolations(path, content string, budgets architectureQualityBudgets, symbols []project.SymbolInfo) []string {
	if symbols == nil {
		_, _, symbols = project.ExtractFileFacts(path, []byte(content))
	}
	if len(symbols) == 0 {
		return nil
	}
	lines := strings.Split(content, "\n")
	var violations []string
	for _, sym := range symbols {
		if !isCallableSymbolKind(sym.Kind) {
			continue
		}
		complexity := genericSpanComplexity(lines, sym.StartLine, sym.EndLine)
		if complexity > budgets.maxComplexity {
			violations = append(violations, fmt.Sprintf("%s:%d: function %s has cyclomatic complexity %d (max %d); split into smaller helpers", filepath.ToSlash(path), sym.StartLine, sym.Name, complexity, budgets.maxComplexity))
		}
		spanLines := sym.EndLine - sym.StartLine + 1
		if budgets.maxFunctionLines > 0 && spanLines > budgets.maxFunctionLines {
			violations = append(violations, fmt.Sprintf("%s:%d: function %s has %d lines (max %d); split into smaller helpers", filepath.ToSlash(path), sym.StartLine, sym.Name, spanLines, budgets.maxFunctionLines))
		}
	}
	return violations
}

func isCallableSymbolKind(kind string) bool {
	switch kind {
	case "function", "fun", "fn", "def", "defp", "method", "const":
		return true
	default:
		return false
	}
}

func genericSpanComplexity(lines []string, startLine, endLine int) int {
	if startLine < 1 || startLine > len(lines) {
		return 1
	}
	if endLine < startLine || endLine > len(lines) {
		endLine = min(len(lines), startLine+40)
	}
	complexity := 1
	for i := startLine - 1; i < endLine; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		complexity += len(branchKeywordPattern.FindAllString(trimmed, -1))
		complexity += strings.Count(trimmed, "&&") + strings.Count(trimmed, "||")
	}
	return complexity
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
