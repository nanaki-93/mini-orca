package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/agent/prompts"
	"github.com/nanaki-93/mini-orca/v2/internal/agent/skills"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/tools"
)

// Orchestrator coordinates the execution of agents in the multi-agent system.
// It provides high-level helper functions for running each agent with the appropriate inputs.
type Orchestrator struct {
	llmClient *llm.Client
	registry  *skills.SkillsRegistry
	executor  tools.ToolExecutor
}

// NewOrchestrator creates a new orchestrator with the given LLM client, skills registry, and tool executor.
func NewOrchestrator(llmClient *llm.Client, registry *skills.SkillsRegistry, executor tools.ToolExecutor) *Orchestrator {
	return &Orchestrator{
		llmClient: llmClient,
		registry:  registry,
		executor:  executor,
	}
}

// getSkillNames extracts skill names from a slice of skills.
func getSkillNames(s []skills.Skill) []string {
	names := make([]string, len(s))
	for i, s := range s {
		names[i] = s.Name
	}
	return names
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
	agent := NewCoderAgent(o.llmClient, o.registry)
	agent.SetSkills(getSkillNames(o.registry.GetForAgent("coder")))
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

// RunTester executes the tester agent with the given code and test results.
// It returns a structured AgentResult containing the test report as JSON.
func (o *Orchestrator) RunTester(code string, testResults string) (*Result, error) {
	if code == "" {
		return nil, fmt.Errorf("orchestrator: tester code is required")
	}
	if testResults == "" {
		return nil, fmt.Errorf("orchestrator: tester test results are required")
	}

	agent := NewTesterAgent(o.llmClient, o.registry)
	agent.SetSkills(getSkillNames(o.registry.GetForAgent("tester")))

	// Combine code and test results into a single input
	input := fmt.Sprintf("Code:\n%s\n\nTest Results:\n%s", code, testResults)

	report, err := agent.Execute(context.Background(), input)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: tester execution failed: %w", err)
	}

	// Convert TestReport to AgentResult
	return testReportToAgentResult(report), nil
}

// RunReviewer executes the reviewer agent with the given code and user prompt.
// It returns a structured AgentResult containing the review report as JSON.
func (o *Orchestrator) RunReviewer(code string, userPrompt string) (*Result, error) {
	if code == "" {
		return nil, fmt.Errorf("orchestrator: reviewer code is required")
	}
	if userPrompt == "" {
		return nil, fmt.Errorf("orchestrator: reviewer user prompt is required")
	}
	agent := NewReviewerAgent(o.llmClient, o.registry)
	agent.SetSkills(getSkillNames(o.registry.GetForAgent("reviewer")))
	input := fmt.Sprintf("## User Request\n%s\n\n## Generated Code\n%s", userPrompt, code)
	report, err := agent.Execute(context.Background(), input)
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
