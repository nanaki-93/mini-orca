package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/agent/prompts"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/tools"
)

// Orchestrator coordinates the execution of agents in the multi-agent system.
// It provides high-level helper functions for running each agent with the appropriate inputs.
type Orchestrator struct {
	llmClient *llm.Client
	executor  tools.ToolExecutor
}

// NewOrchestrator creates a new orchestrator with the given LLM client and tool executor.
func NewOrchestrator(llmClient *llm.Client, executor tools.ToolExecutor) *Orchestrator {
	return &Orchestrator{
		llmClient: llmClient,
		executor:  executor,
	}
}

// RunCoder executes the coder agent with a plan unit description.
// DEPRECATED: Use RunCoderFromPrompt instead.
func (o *Orchestrator) RunCoder(unit prompts.PlanUnit) (*Result, error) {
	prompt := fmt.Sprintf("Title: %s\nDescription: %s", unit.Title, unit.Description)
	if len(unit.Dependencies) > 0 {
		prompt += fmt.Sprintf("\nDependencies: %v", unit.Dependencies)
	}
	return o.RunCoderFromPrompt(prompt, "")
}

// RunCoderFromPrompt executes the coder agent with a user prompt and project context.
// It returns a structured AgentResult containing the generated code.
func (o *Orchestrator) RunCoderFromPrompt(userPrompt string, projectContext string) (*Result, error) {
	if userPrompt == "" {
		return nil, fmt.Errorf("orchestrator: coder user prompt is required")
	}
	agent := NewCoderAgent(o.llmClient)
	var input strings.Builder
	input.WriteString("## Feature Request\n" + userPrompt + "\n\n")
	if projectContext != "" {
		input.WriteString("## Project Context\n" + projectContext + "\n\n")
	}
	input.WriteString("## Instructions\n")
	input.WriteString("Generate the code for this feature. Include a comment at the top:\n")
	input.WriteString("// target: path/to/target_file.go\n")
	input.WriteString("Return ONLY the code in a code block.\n")
	return agent.Execute(context.Background(), input.String())
}

// RunCoderForSymbol generates one atomic function or class in exactly one target file.
func (o *Orchestrator) RunCoderForSymbol(userPrompt, projectContext, targetFile, targetSymbol string) (*Result, error) {
	return o.RunCoderForSymbolContext(context.Background(), userPrompt, projectContext, targetFile, targetSymbol)
}

// RunCoderForSymbolContext generates one atomic candidate using the caller's context.
func (o *Orchestrator) RunCoderForSymbolContext(ctx context.Context, userPrompt, projectContext, targetFile, targetSymbol string) (*Result, error) {
	input, err := AtomicCoderInput(userPrompt, projectContext, targetFile, targetSymbol)
	if err != nil {
		return nil, err
	}
	agent := NewCoderAgent(o.llmClient)
	return agent.Execute(ctx, input)
}

// AtomicCoderInput builds the shared prompt for one-file, one-symbol generation.
func AtomicCoderInput(userPrompt, projectContext, targetFile, targetSymbol string) (string, error) {
	return AtomicCoderInputWithScope(userPrompt, projectContext, targetFile, targetSymbol, "strict_symbol")
}

// AtomicCoderInputWithScope builds a versioned machine-checkable generation request.
func AtomicCoderInputWithScope(userPrompt, projectContext, targetFile, targetSymbol, scope string) (string, error) {
	if strings.TrimSpace(userPrompt) == "" {
		return "", fmt.Errorf("orchestrator: coder user prompt is required")
	}
	if strings.TrimSpace(targetFile) == "" {
		return "", fmt.Errorf("orchestrator: target file is required")
	}
	if strings.TrimSpace(targetSymbol) == "" {
		return "", fmt.Errorf("orchestrator: target symbol is required")
	}
	var input strings.Builder
	input.WriteString("## Atomic code request\n" + userPrompt + "\n\n")
	input.WriteString("## Immutable scope\n")
	input.WriteString("Target file: " + targetFile + "\n")
	input.WriteString("Target function or class: " + targetSymbol + "\n")
	input.WriteString("Action: fix\n")
	input.WriteString("Scope mode: " + scope + "\n")
	input.WriteString("You may change only this named symbol in this one file. Do not create, rename, or modify any other file or symbol. Preserve unrelated target-file code exactly.\n\n")
	input.WriteString("## Project-wide context\n" + projectContext + "\n\n")
	input.WriteString("## Output contract\nReturn exactly one JSON object and no Markdown or prose. Its fields must be version (\"v1\"), target_path (\"" + targetFile + "\"), target_symbol (\"" + targetSymbol + "\"), scope_mode (\"" + scope + "\"), candidate_content (the complete updated content of " + targetFile + "), and optional rationale. Do not return patches, explanations outside rationale, or additional files.\n")
	return input.String(), nil
}

// RunTester executes the tester agent with the given code and test results.
// It returns a structured AgentResult containing the test report as JSON.
func (o *Orchestrator) RunTester(code string, testResults string) (*Result, error) {
	return o.RunTesterContext(context.Background(), code, testResults)
}

// RunTesterContext runs the optional focused tester with the caller's context.
func (o *Orchestrator) RunTesterContext(ctx context.Context, code string, testResults string) (*Result, error) {
	if code == "" {
		return nil, fmt.Errorf("orchestrator: tester code is required")
	}
	if testResults == "" {
		return nil, fmt.Errorf("orchestrator: tester test results are required")
	}

	agent := NewTesterAgent(o.llmClient)

	// Combine code and test results into a single input
	input := fmt.Sprintf("Code:\n%s\n\nTest Results:\n%s", code, testResults)

	report, err := agent.Execute(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: tester execution failed: %w", err)
	}

	// Convert TestReport to AgentResult
	return testReportToAgentResult(report), nil
}

// RunReviewer executes the reviewer agent with the given code and user prompt.
// It returns a structured AgentResult containing the review report as JSON.
func (o *Orchestrator) RunReviewer(code string, userPrompt string) (*Result, error) {
	return o.RunReviewerContext(context.Background(), code, userPrompt)
}

// RunReviewerContext runs the optional focused reviewer with the caller's context.
func (o *Orchestrator) RunReviewerContext(ctx context.Context, code string, userPrompt string) (*Result, error) {
	if code == "" {
		return nil, fmt.Errorf("orchestrator: reviewer code is required")
	}
	if userPrompt == "" {
		return nil, fmt.Errorf("orchestrator: reviewer user prompt is required")
	}
	agent := NewReviewerAgent(o.llmClient)
	input := fmt.Sprintf("## User Request\n%s\n\n## Generated Code\n%s", userPrompt, code)
	report, err := agent.Execute(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: reviewer execution failed: %w", err)
	}
	return reviewReportToAgentResult(report), nil
}

// testReportToAgentResult converts a TestReport to an AgentResult.
func testReportToAgentResult(report *TestReport) *Result {
	data, _ := json.Marshal(report)
	return &Result{
		Output: string(data),
		Metadata: map[string]string{
			"passed":  fmt.Sprintf("%t", report.Passed),
			"summary": report.Summary,
		},
		Phase: "testing",
	}
}

// reviewReportToAgentResult converts a ReviewReport to an AgentResult.
func reviewReportToAgentResult(report *ReviewReport) *Result {
	data, _ := json.Marshal(report)
	return &Result{
		Output: string(data),
		Metadata: map[string]string{
			"score":          fmt.Sprintf("%d", report.Score),
			"recommendation": report.Recommendation,
			"summary":        report.Summary,
		},
		Phase: "review",
	}
}
