package domain

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

type remediationRule struct {
	keywords []string
	template string
}

var remediationRules = []remediationRule{
	{
		keywords: []string{"unchecked", "ignore", "error return", "err != nil", "check error", "unhandled error", "swallow"},
		template: "Check and propagate the error for %q %s with contextual wrapping, and cover the failure path%s.",
	},
	{
		keywords: []string{"nil", "panic", "dereference", "out of range", "bounds", "empty slice", "index"},
		template: "Add an explicit nil/bounds guard for %q %s before access and return a safe error or fallback%s.",
	},
	{
		keywords: []string{"race", "mutex", "lock", "goroutine", "concurrent", "deadlock", "channel", "context", "cancel", "timeout"},
		template: "Guard shared state and propagate context cancellation/deadlines for %q %s, then verify with -race%s.",
	},
	{
		keywords: []string{"leak", "defer", "close", "descriptor", "handle", "body", "reader", "writer", "stream"},
		template: "Ensure deterministic resource cleanup (deferred Close and error check) for %q %s on all return paths%s.",
	},
	{
		keywords: []string{"inject", "traversal", "symlink", "exec", "command", "shell", "sql", "xss", "ssrf", "unsanitized", "untrusted"},
		template: "Validate and canonicalize untrusted input for %q %s before the sensitive operation, rejecting invalid inputs%s.",
	},
	{
		keywords: []string{"auth", "token", "jwt", "secret", "password", "credential", "crypto", "nonce", "permission", "privilege", "tls", "hash"},
		template: "Enforce strict authentication/cryptographic validation for %q %s and reject unauthorized or malformed inputs%s.",
	},
	{
		keywords: []string{"alloc", "loop", "iteration", "prealloc", "capacity", "concat", "builder", "regex", "compile", "buffer", "unbounded", "memory"},
		template: "Eliminate redundant per-iteration work or preallocate capacity for %q %s to bound resource usage%s.",
	},
}

var actionablePrefixes = []string{
	"use ", "ensure ", "validate ", "check ", "return ", "replace ", "guard ",
	"wrap ", "close ", "avoid ", "reject ", "sanitize ", "lock ", "propagate ",
	"handle ", "bound ", "pass ", "add ", "verify ", "enforce ", "preallocate ",
}

// MatchIndexedSymbolForFinding resolves the best-matching symbol and signature from indexed file symbols
// using explicit symbol name, line span, or identifier mention in the finding title/cause.
func MatchIndexedSymbolForFinding(symbols []client.SymbolInfo, targetName string, line int, title, cause string) (string, string) {
	for _, sym := range symbols {
		if targetName != "" && sym.Name == targetName {
			return sym.Name, sym.Signature
		}
	}
	if line > 0 {
		for _, sym := range symbols {
			if sym.StartLine <= line && (sym.EndLine >= line || sym.StartLine == line) {
				return sym.Name, sym.Signature
			}
		}
	}
	return matchSymbolMentionInText(symbols, title+" "+cause)
}

func matchSymbolMentionInText(symbols []client.SymbolInfo, text string) (string, string) {
	if strings.TrimSpace(text) == "" || len(symbols) == 0 {
		return "", ""
	}
	var best client.SymbolInfo
	for _, sym := range symbols {
		short := shortSymbolIdentifier(sym.Name)
		if len(short) < 3 {
			continue
		}
		if containsIdentifierToken(text, sym.Name) || containsIdentifierToken(text, short) {
			if len(sym.Name) > len(best.Name) {
				best = sym
			}
		}
	}
	return best.Name, best.Signature
}

func shortSymbolIdentifier(name string) string {
	if idx := strings.LastIndexByte(name, '.'); idx >= 0 {
		return name[idx+1:]
	}
	return name
}

func containsIdentifierToken(text, ident string) bool {
	if ident == "" {
		return false
	}
	idx := strings.Index(text, ident)
	if idx < 0 {
		return false
	}
	beforeOK := idx == 0 || !isIdentByte(text[idx-1])
	end := idx + len(ident)
	afterOK := end >= len(text) || !isIdentByte(text[end])
	return beforeOK && afterOK
}

func isIdentByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

// DistinctInsightEvidence returns non-redundant evidence from explicit finding evidence or model EngineeringInsight.
func DistinctInsightEvidence(evidence, cause string, insight *client.EngineeringInsight) string {
	trimmedCause := strings.TrimSpace(cause)
	if trimmed := strings.TrimSpace(evidence); trimmed != "" && trimmed != trimmedCause {
		return trimmed
	}
	if insight == nil {
		return ""
	}
	for _, candidate := range []string{insight.WhyItMattersHere, insight.TradeoffOrFailureMode, insight.Mechanism} {
		if c := strings.TrimSpace(candidate); c != "" && c != trimmedCause {
			return c
		}
	}
	return ""
}

// SynthesizeIssueRemediation produces a specific, finding-grounded remediation plan when TaskSpec is absent.
func SynthesizeIssueRemediation(title, cause, category, rule, source, path, symbol string, line int, insight *client.EngineeringInsight) string {
	if fromInsight := remediationFromInsight(insight, cause); fromInsight != "" {
		return fromInsight
	}
	subject := conciseSubject(title, cause)
	target := formatRemediationTarget(path, symbol, line)
	testSuffix := formatCompanionTestSuffix(path)
	if fromTool := remediationForToolSource(source, rule, subject, target); fromTool != "" {
		return fromTool
	}
	if fromClause := extractActionableSentence(cause); fromClause != "" {
		return fmt.Sprintf("%s (%s)%s.", strings.TrimRight(fromClause, ". "), target, testSuffix)
	}
	combined := strings.ToLower(title + " " + cause + " " + rule)
	for _, r := range remediationRules {
		if matchesAnyKeyword(combined, r.keywords) {
			return fmt.Sprintf(r.template, subject, target, testSuffix)
		}
	}
	return fallbackCategoryRemediation(category, subject, cause, formatRemediationNounTarget(path, symbol, line), testSuffix)
}

func remediationFromInsight(insight *client.EngineeringInsight, cause string) string {
	if insight == nil {
		return ""
	}
	trimmedCause := strings.TrimSpace(cause)
	for _, candidate := range []string{insight.TransferableLesson, insight.TradeoffOrFailureMode} {
		if c := strings.TrimSpace(candidate); c != "" && c != trimmedCause {
			return c
		}
	}
	return ""
}

func remediationForToolSource(source, rule, subject, target string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "parser":
		return fmt.Sprintf("Fix syntax/parser error %s (%s) and verify clean AST parsing.", target, subject)
	case "vet":
		ruleLabel := client.FirstNonEmpty(rule, "static")
		return fmt.Sprintf("Resolve 'go vet' %s diagnostic %s (%s) and re-run 'go vet ./...'.", ruleLabel, target, subject)
	case "test":
		return fmt.Sprintf("Fix failing test or underlying logic %s (%s) and re-run package tests.", target, subject)
	default:
		return ""
	}
}

func extractActionableSentence(cause string) string {
	parts := strings.Split(strings.TrimSpace(cause), ". ")
	if len(parts) < 2 {
		return ""
	}
	last := strings.TrimSpace(parts[len(parts)-1])
	lower := strings.ToLower(last)
	for _, prefix := range actionablePrefixes {
		if strings.HasPrefix(lower, prefix) {
			return last
		}
	}
	return ""
}

func matchesAnyKeyword(haystack string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(haystack, kw) {
			return true
		}
	}
	return false
}

func fallbackCategoryRemediation(category, subject, cause, nounTarget, testSuffix string) string {
	detail := conciseCauseDetail(subject, cause)
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "security":
		return fmt.Sprintf("Enforce boundary validation in %s to remediate %q%s, and add a security regression test%s.", nounTarget, subject, detail, testSuffix)
	case "performance":
		return fmt.Sprintf("Optimize %s to eliminate %q%s while preserving output semantics%s.", nounTarget, subject, detail, testSuffix)
	default:
		return fmt.Sprintf("Update %s to resolve %q%s and add a targeted regression test%s.", nounTarget, subject, detail, testSuffix)
	}
}

func conciseSubject(title, cause string) string {
	subj := FindingTitle(title, cause)
	if len(subj) > 72 {
		subj = FindingName(title, cause)
	}
	return strings.TrimRight(subj, ". ")
}

func conciseCauseDetail(subject, cause string) string {
	c := strings.TrimSpace(strings.TrimRight(cause, ". "))
	s := strings.TrimSpace(strings.TrimRight(subject, ".… "))
	if c == "" || strings.EqualFold(c, subject) || c == "No cause recorded" {
		return ""
	}
	if len(s) >= 12 && strings.HasPrefix(strings.ToLower(c), strings.ToLower(s)) {
		return ""
	}
	if len(c) > 80 {
		c = c[:77] + "…"
	}
	return fmt.Sprintf(" (%s)", c)
}

func formatRemediationTarget(path, symbol string, line int) string {
	noun := formatRemediationNounTarget(path, symbol, line)
	if strings.TrimSpace(symbol) != "" || noun == "target scope" {
		return "in " + noun
	}
	return "at " + noun
}

func formatRemediationNounTarget(path, symbol string, line int) string {
	loc := strings.TrimSpace(path)
	if loc != "" && line > 0 {
		loc = fmt.Sprintf("%s:%d", loc, line)
	}
	sym := strings.TrimSpace(symbol)
	switch {
	case sym != "" && loc != "":
		return fmt.Sprintf("%s (%s)", sym, loc)
	case sym != "":
		return sym
	case loc != "":
		return loc
	default:
		return "target scope"
	}
}

func formatCompanionTestSuffix(path string) string {
	testPath := deriveCompanionTestPath(path)
	if testPath == "" {
		return ""
	}
	return fmt.Sprintf(" in %s", testPath)
}

func deriveCompanionTestPath(path string) string {
	p := filepath.ToSlash(strings.TrimSpace(path))
	if p == "" || strings.HasSuffix(p, "_test.go") {
		return p
	}
	if strings.HasSuffix(p, ".go") {
		return strings.TrimSuffix(p, ".go") + "_test.go"
	}
	return ""
}

// SynthesizeIssueVerification builds a specific verification plan for Issue Detail when AcceptanceCriteria are absent.
func SynthesizeIssueVerification(cur IssueRow, index *client.ProjectIndex) string {
	subject := conciseSubject(cur.Title, cur.Cause)
	nounTarget := formatRemediationNounTarget(cur.Path, cur.Symbol, cur.Line)
	scope := EnsureChangeTestPaths([]string{cur.Path}, index)
	testTarget := deriveCompanionTestPath(cur.Path)
	for _, sp := range scope {
		if sp != cur.Path {
			testTarget = sp
			break
		}
	}
	if testTarget != "" {
		return fmt.Sprintf("Add regression test in %s exercising %s for %q and pass verification checks", testTarget, nounTarget, subject)
	}
	return fmt.Sprintf("Add regression test exercising %s for %q and pass project verification checks", nounTarget, subject)
}
