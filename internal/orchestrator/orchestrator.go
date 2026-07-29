// Package orchestrator provides the main orchestration logic for the Mini-Orca pipeline.
package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/internal/agent"
	"github.com/nanaki-93/mini-orca/internal/agent/prompts"
	"github.com/nanaki-93/mini-orca/internal/config"
	"github.com/nanaki-93/mini-orca/internal/model"
	"github.com/nanaki-93/mini-orca/internal/state"
	"github.com/nanaki-93/mini-orca/internal/tools"
)

// Session represents a single execution session within the pipeline.
type Session struct {
	// ID is the unique identifier for this session.
	ID string
	// Phase is the current phase of the session.
	Phase Phase
	// CreatedAt is when the session was created.
	CreatedAt string
}

// Orchestrator manages the execution flow of the multi-phase development pipeline.
type Orchestrator struct {
	agents         map[string]agent.Agent
	router         *model.Router
	executor       tools.ToolExecutor
	stateStore     *state.Store
	config         *config.Config
	currentPhase   Phase
	currentSession *Session
	historyTracker *HistoryTracker
	ctx            context.Context
	cancel         context.CancelFunc
	humanGate      *HumanGate
}

// New creates a new Orchestrator instance with the given dependencies.
func New(router *model.Router, executor tools.ToolExecutor, config *config.Config) *Orchestrator {
	ctx, cancel := context.WithCancel(context.Background())

	return &Orchestrator{
		router:   router,
		executor: executor,
		config:   config,
		ctx:      ctx,
		cancel:   cancel,
		agents:   make(map[string]agent.Agent),
	}
}

// Run executes the pipeline from start to finish.
func (o *Orchestrator) Run() error {
	// TODO: implement pipeline execution
	return nil
}

// runPlanning executes the planning phase of the pipeline.
// It calls the planner agent with the goal, parses the plan output,
// saves it to the state store, transitions to PlanningReview, and requests human approval.
func (o *Orchestrator) runPlanning(goal string) error {
	if goal == "" {
		return fmt.Errorf("planning phase: goal is required")
	}

	// Create session if not already set
	if o.currentSession == nil {
		o.currentSession = &Session{
			ID:        fmt.Sprintf("session-%d", time.Now().Unix()),
			Phase:     PhasePlanning,
			CreatedAt: time.Now().Format(time.RFC3339),
		}
	}

	// Initialize state store if not set
	if o.stateStore == nil {
		session := &state.Session{
			ID:           o.currentSession.ID,
			Goal:         goal,
			ProjectPath:  "",
			ProjectType:  "",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
			CurrentPhase: state.PhasePlanning,
			Status:       state.SessionStatusRunning,
		}
		o.stateStore = state.NewStore(session)
	}

	// Initialize history tracker
	if o.historyTracker == nil {
		o.historyTracker = NewHistoryTracker(o.currentSession.ID, o.stateStore)
	}

	// Update session status to running
	if err := o.stateStore.UpdatePhaseStatus(state.PhasePlanning, state.SessionStatusRunning); err != nil {
		return fmt.Errorf("planning phase: failed to update session status: %w", err)
	}

	// Call planner agent with goal
	var err error
	var plannerResult *agent.AgentResult
	start := time.Now()
	err = WithRetry(func() error {
		res, agentErr := o.runPlannerAgent(goal)
		if agentErr != nil {
			return fmt.Errorf("planner agent failed: %w", agentErr)
		}
		plannerResult = res
		return nil
	}, o.config.Retry.MaxRetries, o.config.Retry.BackoffBase, o.config.Retry.BackoffMax)
	if err != nil {
		o.historyTracker.LogLLMCall("planner", goal, "", time.Since(start), err)
		return fmt.Errorf("planning phase: planner execution failed: %w", err)
	}
	result := plannerResult
	o.historyTracker.LogLLMCall("planner", goal, truncateString(result.Output, 500), time.Since(start), nil)

	// Parse plan output into structured plan
	plan, err := parsePlanOutput(result.Output, goal, o.currentSession.ID)
	if err != nil {
		return fmt.Errorf("planning phase: failed to parse plan: %w", err)
	}

	// Save plan to state store
	if err := o.stateStore.SavePlan(plan); err != nil {
		return fmt.Errorf("planning phase: failed to save plan: %w", err)
	}

	// Add phase history
	history := &state.PhaseHistory{
		ID:          fmt.Sprintf("history-%d", time.Now().UnixNano()),
		SessionID:   o.currentSession.ID,
		Phase:       string(PhasePlanning),
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
		Status:      state.PhaseStatusCompleted,
		Output:      result.Output,
		RetryCount:  o.config.Retry.MaxRetries,
	}
	if err := o.stateStore.SavePhaseHistory(history); err != nil {
		return fmt.Errorf("planning phase: failed to add phase history: %w", err)
	}

	// Transition to PlanningReview phase
	if err := o.transitionTo(PhasePlanningReview); err != nil {
		o.historyTracker.LogPhaseTransition(PhasePlanning, PhasePlanningReview)
		return fmt.Errorf("planning phase: failed to transition to planning review: %w", err)
	}
	o.historyTracker.LogPhaseTransition(PhasePlanning, PhasePlanningReview)

	// Request human approval
	if err := o.requestHumanApproval(plan); err != nil {
		return fmt.Errorf("planning phase: human approval failed: %w", err)
	}

	return nil
}

// runPlannerAgent calls the planner agent with the given goal and returns the result.
func (o *Orchestrator) runPlannerAgent(goal string) (*agent.AgentResult, error) {
	orchestrator := agent.NewOrchestrator(o.router, nil, o.executor)
	return orchestrator.RunPlanner(goal)
}

// parsePlanOutput parses the raw planner output into a structured state.Plan.
func parsePlanOutput(rawOutput, goal, sessionID string) (*state.Plan, error) {
	if rawOutput == "" {
		return nil, fmt.Errorf("parse plan: empty output")
	}

	plan := &state.Plan{
		ID:          fmt.Sprintf("plan-%d", time.Now().UnixNano()),
		SessionID:   sessionID,
		GeneratedAt: time.Now(),
		GeneratedBy: "planner",
		Status:      state.PlanStatusDraft,
	}

	// Parse plan units from the raw output
	units := parsePlanUnits(rawOutput)
	plan.Units = units

	if len(units) == 0 {
		return nil, fmt.Errorf("parse plan: no units found in output")
	}

	return plan, nil
}

// parsePlanUnits parses the raw plan output into a slice of state.PlanUnit.
func parsePlanUnits(rawOutput string) []state.PlanUnit {
	var units []state.PlanUnit

	// Split output into lines
	lines := strings.Split(rawOutput, "\n")

	var currentUnit *state.PlanUnit
	var currentDescription strings.Builder
	var currentDependencies strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for numbered item (new unit)
		if isNumberedItem(trimmed) {
			// Save previous unit if exists
			if currentUnit != nil {
				currentUnit.Description = strings.TrimSpace(currentDescription.String())
				currentUnit.Dependencies = parseDependencies(currentDependencies.String())
				units = append(units, *currentUnit)
			}

			// Start new unit
			currentUnit = &state.PlanUnit{
				ID:             fmt.Sprintf("unit-%d", len(units)+1),
				Name:           extractItemTitle(trimmed),
				Description:    "",
				Dependencies:   []string{},
				File:           "",
				UnitType:       "",
				GeneratedCode:  "",
				Status:         state.UnitStatusPending,
				Tests:          []state.TestDefinition{},
				ReviewComments: []state.ReviewComment{},
				RetryCount:     0,
			}
			currentDescription.Reset()
			currentDependencies.Reset()
		} else if currentUnit != nil {
			// Accumulate description and dependencies
			if strings.HasPrefix(trimmed, "Description:") || strings.HasPrefix(trimmed, "description:") {
				currentDescription.WriteString(strings.TrimSpace(trimmed[len("Description"):]))
			} else if strings.HasPrefix(trimmed, "Dependencies:") || strings.HasPrefix(trimmed, "dependencies:") {
				currentDependencies.WriteString(strings.TrimSpace(trimmed[len("Dependencies"):]))
			} else if currentDescription.Len() > 0 || strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "*") {
				currentDescription.WriteString("\n" + trimmed)
			}
		}
	}

	// Save last unit
	if currentUnit != nil {
		currentUnit.Description = strings.TrimSpace(currentDescription.String())
		currentUnit.Dependencies = parseDependencies(currentDependencies.String())
		units = append(units, *currentUnit)
	}

	return units
}

// isNumberedItem checks if a line starts with a numbered item.
func isNumberedItem(line string) bool {
	if len(line) < 2 {
		return false
	}

	// Check for patterns like "1.", "2.", "1)" etc.
	for i, ch := range line {
		if ch >= '1' && ch <= '9' {
			// Found a digit, check if followed by "." or ")"
			if i+1 < len(line) && (line[i+1] == '.' || line[i+1] == ')') {
				return true
			}
			return false
		}
		if ch != ' ' {
			return false
		}
	}
	return false
}

// extractItemTitle extracts the title from a numbered item line.
func extractItemTitle(line string) string {
	// Remove the leading number and separator
	idx := strings.IndexAny(line, ".)")
	if idx == -1 || idx >= len(line)-1 {
		return line
	}

	title := strings.TrimSpace(line[idx+1:])

	// Remove bold formatting markers
	title = strings.TrimPrefix(title, "**")
	title = strings.TrimSuffix(title, "**")

	return title
}

// parseDependencies parses a dependencies string into a slice of strings.
func parseDependencies(deps string) []string {
	if deps == "" || strings.EqualFold(deps, "none") {
		return []string{}
	}

	var result []string
	// Split by comma and clean up
	parts := strings.Split(deps, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}

	return result
}

// requestHumanApproval requests human approval for the plan.
func (o *Orchestrator) requestHumanApproval(plan *state.Plan) error {
	if plan == nil {
		return fmt.Errorf("request approval: plan is required")
	}

	// Create human gate for planning review
	o.humanGate = NewHumanGate(o.currentSession.ID, PhasePlanningReview)

	// Format the plan for human review
	planOutput := formatPlanForReview(plan)

	// Request approval
	_, err := o.humanGate.RequestApproval(planOutput)
	if err != nil {
		return fmt.Errorf("human approval: %w", err)
	}

	return nil
}

// formatPlanForReview formats a plan into a human-readable review string.
func formatPlanForReview(plan *state.Plan) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Plan: %s\n\n", plan.ID))
	sb.WriteString(fmt.Sprintf("**Goal:** %s\n\n", plan.SessionID))
	sb.WriteString(fmt.Sprintf("**Generated At:** %s\n\n", plan.GeneratedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("**Units:** %d\n\n", len(plan.Units)))
	sb.WriteString("---\n\n")

	for i, unit := range plan.Units {
		sb.WriteString(fmt.Sprintf("### %d. %s\n\n", i+1, unit.Name))
		sb.WriteString(fmt.Sprintf("**Description:** %s\n\n", unit.Description))

		if len(unit.Dependencies) > 0 {
			sb.WriteString("**Dependencies:**\n")
			for _, dep := range unit.Dependencies {
				sb.WriteString(fmt.Sprintf("- %s\n", dep))
			}
			sb.WriteString("\n")
		}

		sb.WriteString("---\n\n")
	}

	return sb.String()
}

// runCoding executes the coding phase of the pipeline.
// It gets the next pending unit from the plan, calls the coder agent,
// writes the generated code to a file, formats it, updates the unit status,
// and transitions to the Testing phase.
func (o *Orchestrator) runCoding() error {
	// Get the plan from state store
	plan, err := o.stateStore.GetPlanBySession(o.currentSession.ID)
	if err != nil {
		return fmt.Errorf("coding phase: %w", err)
	}
	if plan == nil {
		return fmt.Errorf("coding phase: no plan found in state store")
	}

	// Get next pending unit from plan
	unit, err := o.getNextPendingUnit(plan)
	if err != nil {
		return fmt.Errorf("coding phase: %w", err)
	}

	// Update unit status to coding
	if err := o.updateUnitStatus(plan, unit.ID, state.UnitStatusCoding); err != nil {
		return fmt.Errorf("coding phase: failed to update unit status: %w", err)
	}

	// Call coder agent with unit details
	var coderResult *agent.AgentResult
	start := time.Now()
	err = WithRetry(func() error {
		res, agentErr := o.runCoderAgent(unit)
		if agentErr != nil {
			return fmt.Errorf("coder agent failed: %w", agentErr)
		}
		coderResult = res
		return nil
	}, o.config.Retry.MaxRetries, o.config.Retry.BackoffBase, o.config.Retry.BackoffMax)
	if err != nil {
		o.historyTracker.LogLLMCall("coder", fmt.Sprintf("Unit: %s", unit.Name), "", time.Since(start), err)
		return fmt.Errorf("coding phase: coder execution failed: %w", err)
	}
	result := coderResult
	o.historyTracker.LogLLMCall("coder", fmt.Sprintf("Unit: %s", unit.Name), truncateString(result.Output, 500), time.Since(start), nil)

	// Extract code from agent output
	code, targetFile, err := o.extractCodeFromResult(result.Output, unit)
	if err != nil {
		return fmt.Errorf("coding phase: failed to extract code: %w", err)
	}

	// Execute atomic file operation (write ONE unit)
	if err := o.writeFileAtomic(targetFile, code); err != nil {
		o.historyTracker.LogFileOperation("write", targetFile, err)
		return fmt.Errorf("coding phase: failed to write file: %w", err)
	}
	o.historyTracker.LogFileOperation("write", targetFile, nil)

	// Format code
	if err := o.formatCode(targetFile); err != nil {
		// Don't fail the phase if formatting fails, just log it
		fmt.Printf("Warning: formatting failed for %s: %v\n", targetFile, err)
	}

	// Update unit status to completed
	unit.GeneratedCode = code
	unit.File = targetFile
	if err := o.updateUnitStatus(plan, unit.ID, state.UnitStatusCompleted); err != nil {
		return fmt.Errorf("coding phase: failed to update unit status to completed: %w", err)
	}

	// Persist unit code to state store
	if err := o.stateStore.SaveUnitCode(o.currentSession.ID, unit.ID, code); err != nil {
		return fmt.Errorf("coding phase: failed to save unit code: %w", err)
	}

	// Add phase history
	history := &state.PhaseHistory{
		ID:          fmt.Sprintf("history-%d", time.Now().UnixNano()),
		SessionID:   o.currentSession.ID,
		Phase:       string(PhaseCoding),
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
		Status:      state.PhaseStatusCompleted,
		Output:      fmt.Sprintf("Unit %s completed: %s", unit.ID, unit.Name),
		RetryCount:  o.config.Retry.MaxRetries,
	}
	if err := o.stateStore.SavePhaseHistory(history); err != nil {
		return fmt.Errorf("coding phase: failed to add phase history: %w", err)
	}

	// Transition to Testing phase
	if err := o.transitionTo(PhaseTesting); err != nil {
		o.historyTracker.LogPhaseTransition(PhaseCoding, PhaseTesting)
		return fmt.Errorf("coding phase: failed to transition to testing: %w", err)
	}
	o.historyTracker.LogPhaseTransition(PhaseCoding, PhaseTesting)

	return nil
}

// getNextPendingUnit finds the first pending unit in the plan.
func (o *Orchestrator) getNextPendingUnit(plan *state.Plan) (*state.PlanUnit, error) {
	if plan == nil {
		return nil, fmt.Errorf("no plan available")
	}

	for i := range plan.Units {
		unit := &plan.Units[i]
		if unit.Status == state.UnitStatusPending {
			return unit, nil
		}
	}

	return nil, fmt.Errorf("no pending units found in plan")
}

// runCoderAgent calls the coder agent with the given unit details and returns the result.
func (o *Orchestrator) runCoderAgent(unit *state.PlanUnit) (*agent.AgentResult, error) {
	// Convert state.PlanUnit to prompts.PlanUnit for the coder agent
	promptsUnit := prompts.PlanUnit{
		Title:        unit.Name,
		Description:  unit.Description,
		Dependencies: unit.Dependencies,
	}

	orchestrator := agent.NewOrchestrator(o.router, nil, o.executor)
	return orchestrator.RunCoder(promptsUnit)
}

// extractCodeFromResult extracts code from the agent output and determines the target file.
func (o *Orchestrator) extractCodeFromResult(output string, unit *state.PlanUnit) (string, string, error) {
	if output == "" {
		return "", "", fmt.Errorf("empty code output")
	}

	// Extract code blocks from the output
	codeBlocks := extractCodeBlocks(output)
	if len(codeBlocks) == 0 {
		// Treat entire output as code if no code blocks found
		codeBlocks = []codeBlock{
			{Language: "go", Content: output},
		}
	}

	// Use the first code block
	code := codeBlocks[0].Content

	// Determine target file path
	targetFile := o.determineTargetFile(unit)

	return code, targetFile, nil
}

// determineTargetFile determines the file path for the generated code.
func (o *Orchestrator) determineTargetFile(unit *state.PlanUnit) string {
	// If unit already has a file specified, use it
	if unit.File != "" {
		return unit.File
	}

	// Default: use unit name as filename
	filename := sanitizeFilename(unit.Name)
	return fmt.Sprintf("%s.go", filename)
}

// writeFileAtomic writes content to a file atomically using temp file + rename.
func (o *Orchestrator) writeFileAtomic(path string, content string) error {
	// Create parent directories if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("coding phase: failed to create directory %s: %w", dir, err)
	}

	// Create temp file in the same directory (same filesystem for atomic rename)
	tempFile, err := os.CreateTemp(dir, ".tmp-"+filepath.Base(path)+"-*")
	if err != nil {
		return fmt.Errorf("coding phase: failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()

	// Clean up temp file on error
	defer func() {
		if err != nil {
			os.Remove(tempPath)
		}
	}()

	// Write content to temp file
	if _, err := tempFile.WriteString(content); err != nil {
		tempFile.Close()
		return fmt.Errorf("coding phase: failed to write to temp file: %w", err)
	}

	// Close temp file before rename
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("coding phase: failed to close temp file: %w", err)
	}

	// Atomically rename temp file to target
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("coding phase: failed to rename temp file to %s: %w", path, err)
	}

	return nil
}

// formatCode formats the code file at the given path.
func (o *Orchestrator) formatCode(path string) error {
	if o.executor == nil {
		return fmt.Errorf("coding phase: executor not configured")
	}

	return o.executor.FormatCode(path)
}

// updateUnitStatus updates the status of a unit in the plan.
func (o *Orchestrator) updateUnitStatus(plan *state.Plan, unitID string, status state.UnitStatus) error {
	if plan == nil {
		return fmt.Errorf("update unit status: no plan available")
	}

	for i := range plan.Units {
		if plan.Units[i].ID == unitID {
			plan.Units[i].Status = status
			return nil
		}
	}

	return fmt.Errorf("update unit status: unit %s not found in plan", unitID)
}

// transitionTo transitions the pipeline to the specified phase.
func (o *Orchestrator) transitionTo(phase Phase) error {
	if err := ValidateTransition(o.currentPhase, phase); err != nil {
		return fmt.Errorf("transition: %w", err)
	}

	logTransition(o.currentPhase, phase)
	o.currentPhase = phase

	// Update session phase
	if o.currentSession != nil {
		o.currentSession.Phase = phase
	}

	// Update state store
	if o.stateStore != nil {
		_ = o.stateStore.UpdatePhaseStatus(state.Phase(phase), state.SessionStatusRunning)
	}

	return nil
}

// codeBlock represents a code block extracted from agent output.
type codeBlock struct {
	// Language is the programming language of the code block.
	Language string
	// Content is the actual code content.
	Content string
}

// extractCodeBlocks extracts code blocks from the agent output.
func extractCodeBlocks(output string) []codeBlock {
	var blocks []codeBlock

	// Split by triple backticks
	parts := strings.Split(output, "```")
	for i := 0; i < len(parts)-1; i += 2 {
		// First part is the language specifier
		language := strings.TrimSpace(parts[i])
		if language == "" {
			language = "go" // Default to Go
		}

		// Second part is the content
		content := strings.TrimSpace(parts[i+1])
		if content != "" {
			blocks = append(blocks, codeBlock{
				Language: language,
				Content:  content,
			})
		}
	}

	return blocks
}

// sanitizeFilename sanitizes a string for use as a filename.
func sanitizeFilename(name string) string {
	// Replace spaces and special characters with underscores
	var sb strings.Builder
	for _, ch := range name {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' {
			sb.WriteRune(ch)
		} else {
			sb.WriteString("_")
		}
	}
	return sb.String()
}

// runTesting executes the testing phase of the pipeline.
// It runs language-specific tests, captures test output, calls the tester agent
// to analyze results, and transitions to Review (if pass) or Coding (if fail).
func (o *Orchestrator) runTesting() error {
	var err error
	// Get the plan from state store
	plan, err := o.stateStore.GetPlanBySession(o.currentSession.ID)
	if err != nil {
		return fmt.Errorf("testing phase: %w", err)
	}
	if plan == nil {
		return fmt.Errorf("testing phase: no plan found in state store")
	}

	// Update session status to running
	if err := o.stateStore.UpdatePhaseStatus(state.PhaseTesting, state.SessionStatusRunning); err != nil {
		return fmt.Errorf("testing phase: failed to update session status: %w", err)
	}

	// Run language-specific tests and capture output
	var testOutput string
	err = WithRetry(func() error {
		out, testErr := o.runTests()
		if testErr != nil {
			return fmt.Errorf("test execution failed: %w", testErr)
		}
		testOutput = out
		return nil
	}, 1, o.config.Retry.BackoffBase, o.config.Retry.BackoffMax)
	if err != nil {
		// Tests may have failed, but we still want to analyze the output
		fmt.Printf("Warning: test execution returned error: %v\n", err)
	}

	// Get code from the last completed unit for analysis
	code := o.getLastCompletedCode(plan)

	// Call tester agent to analyze test results
	var testerReport *TestReport
	start := time.Now()
	err = WithRetry(func() error {
		res, agentErr := o.runTesterAgent(code, testOutput)
		if agentErr != nil {
			return fmt.Errorf("tester agent failed: %w", agentErr)
		}
		testerReport = res
		return nil
	}, o.config.Retry.MaxRetries, o.config.Retry.BackoffBase, o.config.Retry.BackoffMax)
	if err != nil {
		o.historyTracker.LogLLMCall("tester", truncateString(code, 500), "", time.Since(start), err)
		return fmt.Errorf("testing phase: tester agent execution failed: %w", err)
	}
	report := testerReport
	o.historyTracker.LogLLMCall("tester", truncateString(code, 500), truncateString(report.Summary, 500), time.Since(start), nil)

	// Persist test results to state store
	testResult := &state.TestResult{
		ID:          fmt.Sprintf("test-result-%d", time.Now().UnixNano()),
		SessionID:   o.currentSession.ID,
		UnitID:      plan.Units[len(plan.Units)-1].ID, // Associate with last unit
		TestOutput:  testOutput,
		Passed:      report.Passed,
		Failures:    report.Failures,
		Suggestions: report.Suggestions,
		Coverage:    report.Coverage,
		Summary:     report.Summary,
		ExecutedAt:  time.Now(),
	}
	if err := o.stateStore.SaveTestResult(testResult); err != nil {
		return fmt.Errorf("testing phase: failed to save test results: %w", err)
	}

	// Add phase history
	history := &state.PhaseHistory{
		ID:          fmt.Sprintf("history-%d", time.Now().UnixNano()),
		SessionID:   o.currentSession.ID,
		Phase:       string(PhaseTesting),
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
		Status:      state.PhaseStatusCompleted,
		Output:      fmt.Sprintf("Test report: passed=%t, summary=%s", report.Passed, report.Summary),
		RetryCount:  o.config.Retry.MaxRetries,
	}
	if err := o.stateStore.SavePhaseHistory(history); err != nil {
		return fmt.Errorf("testing phase: failed to add phase history: %w", err)
	}

	// Decide next phase based on test results
	if report.Passed {
		// Pass → transition to Review
		if err := o.transitionTo(PhaseReview); err != nil {
			o.historyTracker.LogPhaseTransition(PhaseTesting, PhaseReview)
			return fmt.Errorf("testing phase: failed to transition to review: %w", err)
		}
		o.historyTracker.LogPhaseTransition(PhaseTesting, PhaseReview)
	} else {
		// Fail → transition to Coding with feedback
		if err := o.transitionToCodingWithFeedback(plan, report); err != nil {
			o.historyTracker.LogPhaseTransition(PhaseTesting, PhaseCoding)
			return fmt.Errorf("testing phase: failed to transition to coding with feedback: %w", err)
		}
		o.historyTracker.LogPhaseTransition(PhaseTesting, PhaseCoding)
	}

	return nil
}

// runTests executes language-specific tests and captures the output.
// It uses the configured executor to run tests appropriate for the project type.
func (o *Orchestrator) runTests() (string, error) {
	if o.executor == nil {
		return "", fmt.Errorf("testing phase: executor not configured")
	}

	// Execute tests using the tool executor
	result, err := o.executor.Execute("go", []string{"test", "./...", "-v", "-cover"}, 120*time.Second)
	if err != nil {
		// Capture whatever output we got even on error
		return result.Stdout, err
	}

	return result.Stdout, nil
}

// runTesterAgent calls the tester agent with code and test results for analysis.
func (o *Orchestrator) runTesterAgent(code string, testResults string) (*TestReport, error) {
	orchestrator := agent.NewOrchestrator(o.router, nil, o.executor)
	agentResult, err := orchestrator.RunTester(code, testResults)
	if err != nil {
		return nil, err
	}

	// Parse the agent result as a TestReport
	return parseTestReportFromAgentResult(agentResult)
}

// parseTestReportFromAgentResult parses an AgentResult into a TestReport.
func parseTestReportFromAgentResult(result *agent.AgentResult) (*TestReport, error) {
	if result == nil {
		return nil, fmt.Errorf("parse test report: agent result is nil")
	}

	// Parse the agent result as a TestReport from JSON
	var report TestReport
	if err := json.Unmarshal([]byte(result.Output), &report); err != nil {
		// Fall back to creating a basic report from the output text
		report = TestReport{
			Summary: result.Output,
			Passed:  !strings.Contains(strings.ToLower(result.Output), "fail"),
		}
	}

	return &report, nil
}

// TestReport represents the output of a testing phase analysis by the tester agent.
type TestReport struct {
	Passed      bool     `json:"passed"`
	Failures    []string `json:"failures,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
	Coverage    string   `json:"coverage,omitempty"`
	Summary     string   `json:"summary"`
}

// getLastCompletedCode returns the code from the last completed unit in the plan.
func (o *Orchestrator) getLastCompletedCode(plan *state.Plan) string {
	if plan == nil {
		return ""
	}

	// Iterate in reverse to find the most recently completed unit
	for i := len(plan.Units) - 1; i >= 0; i-- {
		unit := &plan.Units[i]
		if unit.Status == state.UnitStatusCompleted && unit.GeneratedCode != "" {
			return unit.GeneratedCode
		}
	}

	return ""
}

// transitionToCodingWithFeedback transitions to the Coding phase with feedback from the tester.
func (o *Orchestrator) transitionToCodingWithFeedback(plan *state.Plan, report *TestReport) error {
	// Log the feedback
	if len(report.Failures) > 0 {
		fmt.Printf("Testing phase: %d failures detected:\n", len(report.Failures))
		for i, failure := range report.Failures {
			fmt.Printf("  %d. %s\n", i+1, failure)
		}
	}

	if len(report.Suggestions) > 0 {
		fmt.Printf("Testing phase: %d suggestions:\n", len(report.Suggestions))
		for i, suggestion := range report.Suggestions {
			fmt.Printf("  %d. %s\n", i+1, suggestion)
		}
	}

	// Reset the last completed unit to pending so it will be reprocessed
	if err := o.resetLastCompletedUnit(plan); err != nil {
		return fmt.Errorf("testing phase: failed to reset unit status: %w", err)
	}

	// Transition to Coding phase
	if err := o.transitionTo(PhaseCoding); err != nil {
		return fmt.Errorf("testing phase: failed to transition to coding: %w", err)
	}

	return nil
}

// resetLastCompletedUnit resets the status of the last completed unit to pending.
func (o *Orchestrator) resetLastCompletedUnit(plan *state.Plan) error {
	if plan == nil {
		return fmt.Errorf("reset unit: no plan available")
	}

	// Find and reset the last completed unit
	for i := len(plan.Units) - 1; i >= 0; i-- {
		if plan.Units[i].Status == state.UnitStatusCompleted {
			plan.Units[i].Status = state.UnitStatusPending
			plan.Units[i].RetryCount++
			return nil
		}
	}

	return fmt.Errorf("reset unit: no completed units found")
}

// runReview executes the review phase of the pipeline.
// It calls the reviewer agent to analyze the code, parses the review report,
// and transitions to HumanReview (if pass) or Coding (if fail).
func (o *Orchestrator) runReview() error {
	var err error
	// Get the plan from state store
	plan, err := o.stateStore.GetPlanBySession(o.currentSession.ID)
	if err != nil {
		return fmt.Errorf("review phase: %w", err)
	}
	if plan == nil {
		return fmt.Errorf("review phase: no plan found in state store")
	}

	// Update session status to running
	if err := o.stateStore.UpdatePhaseStatus(state.PhaseReview, state.SessionStatusRunning); err != nil {
		return fmt.Errorf("review phase: failed to update session status: %w", err)
	}

	// Get code from the last completed unit for review
	code := o.getLastCompletedCode(plan)
	if code == "" {
		return fmt.Errorf("review phase: no code available for review")
	}

	// Call reviewer agent with code and plan
	var reviewerReport *ReviewReport
	start := time.Now()
	err = WithRetry(func() error {
		res, agentErr := o.runReviewerAgent(code, plan)
		if agentErr != nil {
			return fmt.Errorf("reviewer agent failed: %w", agentErr)
		}
		reviewerReport = res
		return nil
	}, o.config.Retry.MaxRetries, o.config.Retry.BackoffBase, o.config.Retry.BackoffMax)
	if err != nil {
		o.historyTracker.LogLLMCall("reviewer", truncateString(code, 500), "", time.Since(start), err)
		return fmt.Errorf("review phase: reviewer agent execution failed: %w", err)
	}
	report := reviewerReport
	o.historyTracker.LogLLMCall("reviewer", truncateString(code, 500), truncateString(report.Summary, 500), time.Since(start), nil)

	// Persist review report to state store
	reviewReport := &state.ReviewReportEntry{
		ID:             fmt.Sprintf("review-report-%d", time.Now().UnixNano()),
		SessionID:      o.currentSession.ID,
		UnitID:         plan.Units[len(plan.Units)-1].ID, // Associate with last unit
		Issues:         report.Issues,
		Suggestions:    report.Suggestions,
		Score:          report.Score,
		Recommendation: report.Recommendation,
		Summary:        report.Summary,
		ReviewedAt:     time.Now(),
	}
	if err := o.stateStore.SaveReviewReport(reviewReport); err != nil {
		return fmt.Errorf("review phase: failed to save review report: %w", err)
	}

	// Add phase history
	history := &state.PhaseHistory{
		ID:          fmt.Sprintf("history-%d", time.Now().UnixNano()),
		SessionID:   o.currentSession.ID,
		Phase:       string(PhaseReview),
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
		Status:      state.PhaseStatusCompleted,
		Output:      fmt.Sprintf("Review score: %d, recommendation: %s", report.Score, report.Recommendation),
		RetryCount:  o.config.Retry.MaxRetries,
	}
	if err := o.stateStore.SavePhaseHistory(history); err != nil {
		return fmt.Errorf("review phase: failed to add phase history: %w", err)
	}

	// Decide next phase based on review results
	if o.isReviewPassing(report) {
		// Pass → transition to HumanReview
		if err := o.transitionTo(PhaseHumanReview); err != nil {
			o.historyTracker.LogPhaseTransition(PhaseReview, PhaseHumanReview)
			return fmt.Errorf("review phase: failed to transition to human review: %w", err)
		}
		o.historyTracker.LogPhaseTransition(PhaseReview, PhaseHumanReview)
	} else {
		// Fail → transition to Coding with feedback
		if err := o.transitionToCodingWithReviewFeedback(plan, report); err != nil {
			o.historyTracker.LogPhaseTransition(PhaseReview, PhaseCoding)
			return fmt.Errorf("review phase: failed to transition to coding with feedback: %w", err)
		}
		o.historyTracker.LogPhaseTransition(PhaseReview, PhaseCoding)
	}

	return nil
}

// runReviewerAgent calls the reviewer agent with code and plan for review.
func (o *Orchestrator) runReviewerAgent(code string, plan *state.Plan) (*ReviewReport, error) {
	orchestrator := agent.NewOrchestrator(o.router, nil, o.executor)
	agentResult, err := orchestrator.RunReviewer(code, o.formatPlanForReviewInternal(plan))
	if err != nil {
		return nil, err
	}

	// Parse the agent result as a ReviewReport
	return parseReviewReportFromAgentResult(agentResult)
}

// isReviewPassing determines if the review report indicates a passing review.
func (o *Orchestrator) isReviewPassing(report *ReviewReport) bool {
	// Pass if score is above threshold and no critical issues
	if report.Score >= 70 && len(report.Issues) == 0 {
		return true
	}

	// Also pass if recommendation indicates approval
	lowerRec := strings.ToLower(report.Recommendation)
	if strings.Contains(lowerRec, "approve") || strings.Contains(lowerRec, "pass") || strings.Contains(lowerRec, "accept") {
		return true
	}

	return false
}

// transitionToCodingWithReviewFeedback transitions to the Coding phase with feedback from the reviewer.
func (o *Orchestrator) transitionToCodingWithReviewFeedback(plan *state.Plan, report *ReviewReport) error {
	// Log the feedback
	if len(report.Issues) > 0 {
		fmt.Printf("Review phase: %d issues found:\n", len(report.Issues))
		for i, issue := range report.Issues {
			fmt.Printf("  %d. %s\n", i+1, issue)
		}
	}

	if len(report.Suggestions) > 0 {
		fmt.Printf("Review phase: %d suggestions:\n", len(report.Suggestions))
		for i, suggestion := range report.Suggestions {
			fmt.Printf("  %d. %s\n", i+1, suggestion)
		}
	}

	// Reset the last completed unit to pending so it will be reprocessed
	if err := o.resetLastCompletedUnit(plan); err != nil {
		return fmt.Errorf("review phase: failed to reset unit status: %w", err)
	}

	// Transition to Coding phase
	if err := o.transitionTo(PhaseCoding); err != nil {
		return fmt.Errorf("review phase: failed to transition to coding: %w", err)
	}

	return nil
}

// formatPlanForReviewInternal formats a plan into a string for the reviewer agent.
func (o *Orchestrator) formatPlanForReviewInternal(plan *state.Plan) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Plan: %s\n\n", plan.ID))
	sb.WriteString(fmt.Sprintf("**Goal:** %s\n\n", plan.SessionID))
	sb.WriteString(fmt.Sprintf("**Generated At:** %s\n\n", plan.GeneratedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("**Units:** %d\n\n", len(plan.Units)))
	sb.WriteString("---\n\n")

	for i, unit := range plan.Units {
		sb.WriteString(fmt.Sprintf("### %d. %s\n\n", i+1, unit.Name))
		sb.WriteString(fmt.Sprintf("**Description:** %s\n\n", unit.Description))

		if len(unit.Dependencies) > 0 {
			sb.WriteString("**Dependencies:**\n")
			for _, dep := range unit.Dependencies {
				sb.WriteString(fmt.Sprintf("- %s\n", dep))
			}
			sb.WriteString("\n")
		}

		sb.WriteString("---\n\n")
	}

	return sb.String()
}

// parseReviewReportFromAgentResult parses an AgentResult into a ReviewReport.
func parseReviewReportFromAgentResult(result *agent.AgentResult) (*ReviewReport, error) {
	if result == nil {
		return nil, fmt.Errorf("parse review report: agent result is nil")
	}

	// Parse the agent result as a ReviewReport from JSON
	var report ReviewReport
	if err := json.Unmarshal([]byte(result.Output), &report); err != nil {
		// Fall back to creating a basic report from the output text
		report = ReviewReport{
			Summary: result.Output,
			Score:   50, // Default neutral score
		}
	}

	return &report, nil
}

// ReviewReport represents the output of a code review phase.
type ReviewReport struct {
	Issues         []string `json:"issues,omitempty"`
	Suggestions    []string `json:"suggestions,omitempty"`
	Score          int      `json:"score"`
	Recommendation string   `json:"recommendation"`
	Summary        string   `json:"summary"`
}

// runHumanReview executes the human review phase of the pipeline.
// It presents the final review results to the human, waits for approval or edit request,
// and transitions to Completed (if approved) or Coding (if edit requested).
func (o *Orchestrator) runHumanReview() error {
	// Get the plan from state store
	plan, err := o.stateStore.GetPlanBySession(o.currentSession.ID)
	if err != nil {
		return fmt.Errorf("human review phase: %w", err)
	}
	if plan == nil {
		return fmt.Errorf("human review phase: no plan found in state store")
	}

	// Get code from the last completed unit for review
	code := o.getLastCompletedCode(plan)
	if code == "" {
		return fmt.Errorf("human review phase: no code available for review")
	}

	// Update session status to running
	if err := o.stateStore.UpdatePhaseStatus(state.PhaseHumanReview, state.SessionStatusRunning); err != nil {
		return fmt.Errorf("human review phase: failed to update session status: %w", err)
	}

	// Create human gate for final review
	o.humanGate = NewHumanGate(o.currentSession.ID, PhaseHumanReview)

	// Format the complete review package for human presentation
	reviewOutput := o.formatHumanReviewOutput(plan, code)

	// Present review results to human and wait for response
	response, err := o.humanGate.RequestApproval(reviewOutput)
	if err != nil {
		return fmt.Errorf("human review phase: failed to get human response: %w", err)
	}

	// Add phase history
	history := &state.PhaseHistory{
		ID:          fmt.Sprintf("history-%d", time.Now().UnixNano()),
		SessionID:   o.currentSession.ID,
		Phase:       string(PhaseHumanReview),
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
		Status:      state.PhaseStatusCompleted,
		Output:      fmt.Sprintf("Human response: action=%s, feedback=%s", response.Action, response.Feedback),
	}
	if err := o.stateStore.SavePhaseHistory(history); err != nil {
		return fmt.Errorf("human review phase: failed to add phase history: %w", err)
	}

	// Decide next phase based on human response
	switch response.Action {
	case "approve":
		// Approve → transition to Completed
		if err := o.transitionToCompleted(); err != nil {
			o.historyTracker.LogPhaseTransition(PhaseHumanReview, "completed")
			return fmt.Errorf("human review phase: failed to transition to completed: %w", err)
		}
		o.historyTracker.LogPhaseTransition(PhaseHumanReview, "completed")
	case "edit":
		// Edit → transition to Coding with feedback
		if err := o.transitionToCodingWithHumanFeedback(plan, response.Feedback); err != nil {
			o.historyTracker.LogPhaseTransition(PhaseHumanReview, PhaseCoding)
			return fmt.Errorf("human review phase: failed to transition to coding with feedback: %w", err)
		}
		o.historyTracker.LogPhaseTransition(PhaseHumanReview, PhaseCoding)
	default:
		return fmt.Errorf("human review phase: unexpected action %q", response.Action)
	}

	return nil
}

// formatHumanReviewOutput formats the complete review package for human presentation.
func (o *Orchestrator) formatHumanReviewOutput(plan *state.Plan, code string) string {
	var sb strings.Builder

	sb.WriteString("## Final Review Package\n\n")
	sb.WriteString(fmt.Sprintf("**Session:** %s\n\n", o.currentSession.ID))
	sb.WriteString(fmt.Sprintf("**Goal:** %s\n\n", plan.SessionID))
	sb.WriteString(fmt.Sprintf("**Generated At:** %s\n\n", plan.GeneratedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("**Total Units:** %d\n\n", len(plan.Units)))
	sb.WriteString("---\n\n")

	sb.WriteString("## Plan\n\n")
	for i, unit := range plan.Units {
		sb.WriteString(fmt.Sprintf("### %d. %s\n\n", i+1, unit.Name))
		sb.WriteString(fmt.Sprintf("**Description:** %s\n\n", unit.Description))
		sb.WriteString(fmt.Sprintf("**Status:** %s\n\n", unit.Status))
		if len(unit.Dependencies) > 0 {
			sb.WriteString("**Dependencies:**\n")
			for _, dep := range unit.Dependencies {
				sb.WriteString(fmt.Sprintf("- %s\n", dep))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("---\n\n")
	}

	sb.WriteString("## Generated Code\n\n")
	sb.WriteString("```go\n")
	sb.WriteString(code)
	sb.WriteString("\n```\n\n")
	sb.WriteString("---\n\n")
	sb.WriteString("Please review the above and respond with:\n")
	sb.WriteString("- **approve**: to mark the project as complete\n")
	sb.WriteString("- **edit**: to request changes (include feedback)\n")

	return sb.String()
}

// transitionToCompleted transitions the pipeline to the completed state.
func (o *Orchestrator) transitionToCompleted() error {
	// Validate the transition to completed
	if err := ValidateTransition(o.currentPhase, "completed"); err != nil {
		return fmt.Errorf("transition: %w", err)
	}

	logTransition(o.currentPhase, "completed")

	// Update session phase
	if o.currentSession != nil {
		o.currentSession.Phase = "completed"
	}

	// Update state store
	if o.stateStore != nil {
		if err := o.stateStore.UpdatePhaseStatus(state.PhaseHumanReview, state.SessionStatusCompleted); err != nil {
			return fmt.Errorf("human review phase: failed to update session status to completed: %w", err)
		}
	}

	// Mark session as completed
	o.currentPhase = "completed"

	// Persist history
	if o.historyTracker != nil {
		if err := o.historyTracker.PersistToStore(); err != nil {
			return fmt.Errorf("human review phase: failed to persist history: %w", err)
		}
	}

	return nil
}

// transitionToCodingWithHumanFeedback transitions to the Coding phase with human feedback.
func (o *Orchestrator) transitionToCodingWithHumanFeedback(plan *state.Plan, feedback string) error {
	// Log the feedback
	fmt.Printf("Human review phase: edit requested with feedback: %s\n", feedback)

	// Reset the last completed unit to pending so it will be reprocessed
	if err := o.resetLastCompletedUnit(plan); err != nil {
		return fmt.Errorf("human review phase: failed to reset unit status: %w", err)
	}

	// Transition to Coding phase
	if err := o.transitionTo(PhaseCoding); err != nil {
		return fmt.Errorf("human review phase: failed to transition to coding: %w", err)
	}

	return nil
}
