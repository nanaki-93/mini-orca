# Task 3.1: Update Agent Orchestrator

## Goal
Update the agent orchestrator to accept user prompts directly instead of `PlanUnit` structs.

## Files to Modify
- `/Users/marcoandreose/DEV/lab/mini-orca/internal/agent/orchestrator.go`

## Detailed Steps

### Step 1: Add RunCoderFromPrompt()
```go
func (o *Orchestrator) RunCoderFromPrompt(userPrompt string, projectContext string) (*Result, error) {
	if userPrompt == "" {
		return nil, fmt.Errorf("orchestrator: coder user prompt is required")
	}
	agent := NewCoderAgent(o.router, o.registry)
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
```

### Step 2: Update RunCoder() — Mark as Deprecated
```go
// RunCoder executes the coder agent with a plan unit description.
// DEPRECATED: Use RunCoderFromPrompt instead.
func (o *Orchestrator) RunCoder(unit prompts.PlanUnit) (*Result, error) {
	prompt := fmt.Sprintf("Title: %s\nDescription: %s", unit.Title, unit.Description)
	if len(unit.Dependencies) > 0 {
		prompt += fmt.Sprintf("\nDependencies: %v", unit.Dependencies)
	}
	return o.RunCoderFromPrompt(prompt, "")
}
```

### Step 3: Update RunReviewer()
```go
func (o *Orchestrator) RunReviewer(code string, userPrompt string) (*Result, error) {
	if code == "" {
		return nil, fmt.Errorf("orchestrator: reviewer code is required")
	}
	if userPrompt == "" {
		return nil, fmt.Errorf("orchestrator: reviewer user prompt is required")
	}
	agent := NewReviewerAgent(o.router, o.registry)
	agent.SetSkills(getSkillNames(o.registry.GetForAgent("reviewer")))
	input := fmt.Sprintf("## User Request\n%s\n\n## Generated Code\n%s", userPrompt, code)
	report, err := agent.Execute(context.Background(), input)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: reviewer execution failed: %w", err)
	}
	return reviewReportToAgentResult(report), nil
}
```

### Step 4: Keep RunTester() Unchanged
The tester agent doesn't need changes.

### Step 5: Add "strings" import if not present

## Verification
- `go build ./internal/agent/` — compiles
- `RunCoderFromPrompt()` method exists
- `RunReviewer()` accepts `userPrompt` instead of `plan`
