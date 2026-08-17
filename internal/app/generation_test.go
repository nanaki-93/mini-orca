package app

import (
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/workflow"
)

func TestParseGenerationResponseContract(t *testing.T) {
	valid := `{"version":"v1","target_path":"main.go","target_symbol":"Run","scope_mode":"strict_symbol","candidate_content":"package main\nfunc Run() {}","rationale":"Keeps the edit focused."}`
	response, err := ParseGenerationResponse(valid, "main.go", "Run", workflow.ScopeStrictSymbol)
	if err != nil {
		t.Fatal(err)
	}
	if response.CandidateContent != "package main\nfunc Run() {}" || response.Rationale == "" {
		t.Fatalf("parsed response = %+v", response)
	}

	for _, test := range []struct {
		name   string
		output string
	}{
		{name: "compatibility code block", output: "```go\npackage main\nfunc Run() {}\n```"},
		{name: "multiple blocks", output: "```go\npackage main\n```\n```go\nfunc Run() {}\n```"},
		{name: "extra target field", output: `{"version":"v1","target_path":"main.go","target_symbol":"Run","scope_mode":"strict_symbol","candidate_content":"package main","additional_target_path":"other.go"}`},
		{name: "missing candidate", output: `{"version":"v1","target_path":"main.go","target_symbol":"Run","scope_mode":"strict_symbol"}`},
		{name: "wrong target path", output: `{"version":"v1","target_path":"other.go","target_symbol":"Run","scope_mode":"strict_symbol","candidate_content":"package main"}`},
		{name: "wrong target symbol", output: `{"version":"v1","target_path":"main.go","target_symbol":"Other","scope_mode":"strict_symbol","candidate_content":"package main"}`},
		{name: "wrong scope", output: `{"version":"v1","target_path":"main.go","target_symbol":"Run","scope_mode":"symbol_plus_imports","candidate_content":"package main"}`},
		{name: "prose", output: "Here is the result:\n```go\npackage main\n```"},
	} {
		t.Run(test.name, func(t *testing.T) {
			response, err := ParseGenerationResponse(test.output, "main.go", "Run", workflow.ScopeStrictSymbol)
			if test.name == "compatibility code block" {
				if err != nil || !strings.Contains(response.CandidateContent, "func Run") {
					t.Fatalf("compatibility response = %+v, %v", response, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("response = %+v, want validation error", response)
			}
		})
	}
}
