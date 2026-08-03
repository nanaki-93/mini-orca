package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nanaki-93/mini-orca/v2/internal/agent/prompts"
	"github.com/nanaki-93/mini-orca/v2/internal/agent/skills"
	"github.com/nanaki-93/mini-orca/v2/internal/model"
	"github.com/nanaki-93/mini-orca/v2/internal/tools"
)

// Orchestrator coordinates the execution of agents in the multi-agent system.
// It provides high-level helper functions for running each agent with the appropriate inputs.
type Orchestrator struct {
	router   *model.Router
	registry *skills.SkillsRegistry
	executor tools.ToolExecutor
}

// NewOrchestrator creates a new orchestrator with the given router, skills registry, and tool executor.
func NewOrchestrator(router *model.Router, registry *skills.SkillsRegistry, executor tools.ToolExecutor) *Orchestrator {
	return &Orchestrator{
		router:   router,
		registry: registry,
		executor: executor,
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

// RunCoder executes the coder agent with the given plan unit description.
// It returns a structured AgentResult containing the generated code.
func (o *Orchestrator) RunCoder(unit prompts.PlanUnit) (*Result, error) {
	if unit.Title == "" {
		return nil, fmt.Errorf("orchestrator: coder unit title is required")
	}
	if unit.Description == "" {
		return nil, fmt.Errorf("orchestrator: coder unit description is required")
	}

	agent := NewCoderAgent(o.router, o.registry)
	agent.SetSkills(getSkillNames(o.registry.GetForAgent("coder")))

	// Build the input from the plan unit
	input := fmt.Sprintf("Title: %s\nDescription: %s", unit.Title, unit.Description)
	if len(unit.Dependencies) > 0 {
		input += fmt.Sprintf("\nDependencies: %v", unit.Dependencies)
	}

	return agent.Execute(context.Background(), input)
}

// RunCoderFromPrompt executes the coder agent with a user prompt and project context.
// It returns a structured AgentResult containing the generated code.
func (o *Orchestrator) RunCoderFromPrompt(userPrompt string, projectContext string) (*Result, error) {
	if userPrompt == "" {
		return nil, fmt.Errorf("orchestrator: coder user prompt is required")
	}

	agent := NewCoderAgent(o.router, o.registry)
	agent.SetSkills(getSkillNames(o.registry.GetForAgent("coder")))

	// Build the input from the user prompt and project context
	input := userPrompt
	if projectContext != "" {
		input = fmt.Sprintf("%s\n\nProject Context:\n%s", userPrompt, projectContext)
	}

	return agent.Execute(context.Background(), input)
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

	agent := NewTesterAgent(o.router, o.registry)
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

// RunReviewer executes the reviewer agent with the given code and plan.
// It returns a structured AgentResult containing the review report as JSON.
func (o *Orchestrator) RunReviewer(code string, plan string) (*Result, error) {
	if code == "" {
		return nil, fmt.Errorf("orchestrator: reviewer code is required")
	}
	if plan == "" {
		return nil, fmt.Errorf("orchestrator: reviewer plan is required")
	}

	agent := NewReviewerAgent(o.router, o.registry)
	agent.SetSkills(getSkillNames(o.registry.GetForAgent("reviewer")))

	// Combine code and plan into a single input
	input := fmt.Sprintf("Code:\n%s\n\nPlan:\n%s", code, plan)

	report, err := agent.Execute(context.Background(), input)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: reviewer execution failed: %w", err)
	}

	// Convert ReviewReport to AgentResult
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
