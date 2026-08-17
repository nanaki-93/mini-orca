package workflow

import "testing"

func TestRequestValidate(t *testing.T) {
	tests := []struct {
		name string
		req  Request
		want bool
	}{
		{"analyze one file", Request{Action: ActionAnalyzeFile, TargetFile: "internal/app.go"}, true},
		{"explain selected symbol", Request{Action: ActionExplainSymbol, TargetFile: "internal/app.go", TargetSymbol: "Run"}, true},
		{"fix strict symbol", Request{Action: ActionFix, Scope: ScopeStrictSymbol, TargetFile: "internal/app.go", TargetSymbol: "Run"}, true},
		{"refactor with imports", Request{Action: ActionRefactor, Scope: ScopeSymbolPlusImports, TargetFile: "internal/app.go", TargetSymbol: "Run"}, true},
		{"generate test in test file", Request{Action: ActionGenerateTest, Scope: ScopeStrictSymbol, TargetFile: "internal/app_test.go", TargetSymbol: "TestRun"}, true},
		{"unknown action", Request{Action: "rewrite_project", TargetFile: "internal/app.go"}, false},
		{"candidate missing target", Request{Action: ActionFix, Scope: ScopeStrictSymbol, TargetFile: "internal/app.go"}, false},
		{"candidate invalid scope", Request{Action: ActionFix, Scope: "whole_file", TargetFile: "internal/app.go", TargetSymbol: "Run"}, false},
		{"test action targets source", Request{Action: ActionGenerateTest, Scope: ScopeStrictSymbol, TargetFile: "internal/app.go", TargetSymbol: "TestRun"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err == nil) != tt.want {
				t.Fatalf("Validate() error = %v, want valid=%v", err, tt.want)
			}
		})
	}
}

func TestRequestClassifiesActions(t *testing.T) {
	if !(Request{Action: ActionAnalyzeFile}).IsReadOnly() || !(Request{Action: ActionExplainSymbol}).IsReadOnly() {
		t.Fatal("expected analysis and symbol explanation to be read-only")
	}
	if !(Request{Action: ActionDocument}).IsCandidateProducing() || (Request{Action: ActionExplainSymbol}).IsCandidateProducing() {
		t.Fatal("unexpected candidate-producing action classification")
	}
}
