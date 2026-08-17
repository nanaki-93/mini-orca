// Package workflow defines the focused Mini-Orca request contract.
package workflow

import (
	"fmt"
	"path/filepath"
	"strings"
)

type Action string

const (
	ActionAnalyzeFile   Action = "analyze_file"
	ActionExplainSymbol Action = "explain_symbol"
	ActionFix           Action = "fix"
	ActionRefactor      Action = "refactor"
	ActionDocument      Action = "document"
	ActionGenerateTest  Action = "generate_test"
)

type ScopeMode string

const (
	ScopeStrictSymbol      ScopeMode = "strict_symbol"
	ScopeSymbolPlusImports ScopeMode = "symbol_plus_imports"
)

// Request identifies the single unit affected or examined by an action.
type Request struct {
	Action       Action
	Scope        ScopeMode
	TargetFile   string
	TargetSymbol string
}

// Validate enforces Mini-Orca's one-project, one-file, one-symbol contract.
func (r Request) Validate() error {
	if !isKnownAction(r.Action) {
		return fmt.Errorf("unsupported action %q", r.Action)
	}
	if strings.TrimSpace(r.TargetFile) == "" {
		return fmt.Errorf("target file is required")
	}
	if r.Action != ActionAnalyzeFile && strings.TrimSpace(r.TargetSymbol) == "" {
		return fmt.Errorf("target symbol is required for %s", r.Action)
	}
	if r.Action == ActionGenerateTest && !isTestFile(r.TargetFile) {
		return fmt.Errorf("generate_test requires an already selected test file")
	}
	if r.IsCandidateProducing() && r.Scope != ScopeStrictSymbol && r.Scope != ScopeSymbolPlusImports {
		return fmt.Errorf("candidate-producing actions require strict_symbol or symbol_plus_imports scope")
	}
	return nil
}

func (r Request) IsReadOnly() bool {
	return r.Action == ActionAnalyzeFile || r.Action == ActionExplainSymbol
}

func (r Request) IsCandidateProducing() bool {
	return r.Action == ActionFix || r.Action == ActionRefactor || r.Action == ActionDocument || r.Action == ActionGenerateTest
}

func isKnownAction(action Action) bool {
	switch action {
	case ActionAnalyzeFile, ActionExplainSymbol, ActionFix, ActionRefactor, ActionDocument, ActionGenerateTest:
		return true
	default:
		return false
	}
}

func isTestFile(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	return strings.HasSuffix(name, "_test.go") ||
		strings.HasSuffix(name, "test.kt") ||
		strings.HasSuffix(name, "tests.kt") ||
		strings.HasSuffix(name, "test.java") ||
		strings.HasSuffix(name, "tests.java") ||
		strings.Contains(name, ".test.") ||
		strings.Contains(name, ".spec.") ||
		strings.HasPrefix(name, "test_") ||
		strings.HasSuffix(name, "_test.py") ||
		strings.HasSuffix(name, "_test.rs")
}
