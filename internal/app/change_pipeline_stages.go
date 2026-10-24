package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

const workflowTestPrompt = "Write meaningful regression and boundary tests for this proposal and its acceptance criteria. Return complete test file contents only for captured test file paths (such as _test.go, .test.ts, test_*.py, or tests/ files). Preserve existing tests and all pinned regression tests. Do not change implementation files or claim tests ran."

func stripWorkflowTestMessages(messages []ChangeMessage) []ChangeMessage {
	if len(messages) == 0 {
		return messages
	}
	cleaned := make([]ChangeMessage, 0, len(messages))
	for i := 0; i < len(messages); i++ {
		if messages[i].Role == "user" && strings.HasPrefix(messages[i].Content, "Write meaningful regression and boundary tests for this proposal") {
			if i+1 < len(messages) && messages[i+1].Role == "assistant" {
				i++
			}
			continue
		}
		cleaned = append(cleaned, messages[i])
	}
	if len(cleaned) > maxChangeMessages-4 {
		keepTail := maxChangeMessages - 6
		cleaned = append(cleaned[:2:2], cleaned[len(cleaned)-keepTail:]...)
	}
	return cleaned
}

func workflowRetryMessage(message string, workflow *ChangeWorkflow, checks []CheckResult) string {
	extra := criticReviewRetryFeedback(message, workflow) + workflowFailureRetryFeedback(message, workflow, checks) + harnessProbeRetryDirective(message, workflow, checks)
	if extra == "" {
		return message
	}
	combined := message + extra
	if len(combined) > 8192 {
		return combined[:8192]
	}
	return combined
}

func harnessProbeRetryDirective(message string, workflow *ChangeWorkflow, checks []CheckResult) string {
	if strings.Contains(message, "Harness² Propose-Probe-Compose:") {
		return ""
	}
	probe, _ := probeWorkflowFriction(workflow, checks)
	if probe == "" {
		return ""
	}
	return "\n\nHarness² Propose-Probe-Compose:\n" + probe
}

func probeWorkflowFriction(workflow *ChangeWorkflow, checks []CheckResult) (string, []string) {
	var directives []string
	var lessons []string
	for _, check := range checks {
		if check.State == CheckPassed || check.State == CheckSkipped {
			continue
		}
		dir, lesson := classifyFailedCheckFriction(check)
		if dir != "" {
			directives = append(directives, dir)
		}
		if lesson != "" {
			lessons = append(lessons, lesson)
		}
	}
	if workflow != nil && workflow.Review != nil && workflow.Review.Verdict == "changes_requested" {
		directives = append(directives, "Resolve every Critic review finding while keeping existing tests and architecture boundaries intact.")
		if len(workflow.Review.Findings) > 0 {
			lessons = append(lessons, "Critic Guardrail: "+strings.TrimSpace(workflow.Review.Findings[0]))
		}
	}
	if len(directives) == 0 {
		return "", nil
	}
	return strings.Join(directives, " "), mergeHarnessLessons(nil, lessons)
}

func classifyFailedCheckFriction(check CheckResult) (string, string) {
	name := strings.ToLower(check.Name)
	switch {
	case name == "architecture boundaries":
		return "Remove forbidden cross-layer imports and call the owning domain/service interface instead.", "Respect declared layer boundaries (`<source>` must never import `<forbidden>`); route cross-layer access through service contracts."
	case name == "complexity gate":
		return "Decompose complex or oversized functions into smaller cohesive helpers within the complexity budget.", "Keep function cyclomatic complexity <= 15 and extract focused helper functions before adding nested branches."
	case name == "secret & hygiene gate":
		return "Remove hardcoded tokens/keys and unresolved conflict markers; inject secrets from configuration or environment.", "Never embed credentials, API tokens, or merge conflict markers in source or documentation."
	case strings.HasPrefix(name, "parse:") || strings.HasPrefix(name, "format:") || name == "lint":
		return "Fix syntax, formatting, and static analysis diagnostics using sibling symbol signatures.", "Ensure all proposed edits compile cleanly and pass static/lint checks before test execution."
	case name == "tests" || name == "regression baseline":
		return "Align implementation logic with failing test assertions without weakening pinned regression tests.", "Verify edge cases and return contracts against unit test assertions before finalizing changes."
	default:
		return "", ""
	}
}

func mergeHarnessLessons(existing, added []string) []string {
	out := make([]string, 0, len(existing)+len(added))
	seen := map[string]bool{}
	for _, item := range append(append([]string(nil), existing...), added...) {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" || seen[trimmed] || len(out) >= 6 {
			continue
		}
		if len(trimmed) > 360 {
			trimmed = trimmed[:360]
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	return out
}

func criticReviewRetryFeedback(message string, workflow *ChangeWorkflow) string {
	if workflow == nil || workflow.Review == nil || workflow.Review.Verdict != "changes_requested" || strings.Contains(message, "Critic review feedback:") {
		return ""
	}
	var out strings.Builder
	out.WriteString("\n\nCritic review feedback:\n" + strings.TrimSpace(workflow.Review.Summary))
	for _, finding := range workflow.Review.Findings {
		out.WriteString("\n- " + strings.TrimSpace(finding))
	}
	return out.String()
}

func workflowFailureRetryFeedback(message string, workflow *ChangeWorkflow, checks []CheckResult) string {
	if strings.Contains(message, "Previous check failures to resolve:") || strings.Contains(message, "Previous workflow failure to resolve:") {
		return ""
	}
	if evidence := failedWorkflowCheckEvidence(checks); evidence != "" {
		return "\n\nPrevious check failures to resolve:\n" + evidence
	}
	if workflow != nil && workflow.Status == "failed" && strings.TrimSpace(workflow.Reason) != "" && !strings.Contains(message, workflow.Reason) {
		return "\n\nPrevious workflow failure to resolve:\n" + strings.TrimSpace(workflow.Reason)
	}
	return ""
}

func failedWorkflowCheckEvidence(checks []CheckResult) string {
	var text strings.Builder
	for _, check := range checks {
		if check.State == CheckPassed || check.State == CheckSkipped {
			continue
		}
		if text.Len() >= 2400 {
			break
		}
		output := strings.TrimSpace(sanitizeCheckOutput(check.Output, false, "", ""))
		if output == "" {
			output = check.State
		}
		if len(output) > 1500 {
			output = output[len(output)-1500:]
		}
		text.WriteString(check.Name + " (" + check.State + "):\n" + output + "\n")
	}
	return strings.TrimSpace(text.String())
}

func workflowTestStageMessage(stageMessage string) string {
	for _, marker := range []string{
		"\n\nCritic review feedback:",
		"\n\nPrevious check failures to resolve:",
		"\n\nPrevious workflow failure to resolve:",
		"\n\nHarness² Propose-Probe-Compose:",
	} {
		if idx := strings.Index(stageMessage, marker); idx >= 0 {
			combined := workflowTestPrompt + stageMessage[idx:]
			if len(combined) > 8192 {
				return combined[:8192]
			}
			return combined
		}
	}
	return workflowTestPrompt
}

func (s *Service) runWorkflowModelStage(ctx context.Context, job *changeWorkflowJob, session *ChangeSession, runtime modelRuntime, stage int, message string) (*ChangeSession, error) {
	limit := duration(runtime.effective.Timeout)
	if stage == 2 {
		limit *= time.Duration(s.workflowReviewWaves(ctx, len(job.specialists)))
	}
	timed, cancel := context.WithTimeoutCause(ctx, limit, &workflowTimeoutError{stage: []string{"Creation", "Testing", "Review"}[stage], limit: limit})
	defer cancel()
	var err error
	switch stage {
	case 0:
		isRepair := session.Workflow.AutoRepairAttempts > 0 || strings.Contains(message, "Previous check failures to resolve:")
		session, err = s.generateChangeWithRuntime(timed, job.root, session, ChangeMessageRequest{ChangeIdentity: identityForChange(session), Message: message, Repair: isRepair}, runtime)
	case 1:
		session, err = s.generateWorkflowTests(timed, job.root, session, runtime, workflowTestStageMessage(message))
	case 2:
		err = s.reviewWorkflowProposal(timed, job, session, runtime)
	}
	if timed.Err() != nil {
		return nil, context.Cause(timed)
	}
	return session, err
}

func (s *Service) generateWorkflowTests(ctx context.Context, root string, session *ChangeSession, runtime modelRuntime, message string) (*ChangeSession, error) {
	text, _, err := composeChangePromptContext(root, session)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(message) == "" {
		message = workflowTestPrompt
	}
	messages, err := changeMessages(session, message, text)
	if err != nil {
		return nil, err
	}
	messages[0].Content += " You are the testing agent. Only captured test files may appear in changes; implementation is fixed."
	schema := changeResponseSchema()
	result, err := s.requestWorkflowModel(ctx, root, session, runtime, messages, schema)
	if err != nil {
		return nil, err
	}
	explanation, edits, err := parseChangeResponse(result.Content, session)
	if err != nil {
		return nil, err
	}
	testEdits := make([]ChangeEdit, 0, len(edits))
	for _, edit := range edits {
		if isTestFilePath(edit.Path) {
			testEdits = append(testEdits, edit)
		}
	}
	if len(testEdits) == 0 {
		return nil, fmt.Errorf("testing agent attempted to change implementation")
	}
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	current, _, err := s.loadChangeForAction(ctx, session.ID, identityForChange(session))
	if err != nil {
		return nil, err
	}
	current.Changes = mergeWorkflowTests(current.Changes, testEdits)
	current.Hash, current.Revision = changeProposalHash(current.Changes), current.Revision+1
	current.Checks, current.ReviewedHash = []CheckResult{}, ""
	now := time.Now().UTC()
	current.Messages = append(current.Messages, ChangeMessage{Role: "user", Content: message, CreatedAt: now}, ChangeMessage{Role: "assistant", Content: explanation, CreatedAt: now})
	current.UpdatedAt = now
	return current, storeChangeSession(root, current)
}

func mergeWorkflowTests(changes, tests []ChangeEdit) []ChangeEdit {
	for _, test := range tests {
		found := false
		for i := range changes {
			if changes[i].Path == test.Path {
				changes[i], found = test, true
				break
			}
		}
		if !found {
			changes = append(changes, test)
		}
	}
	return changes
}

func (s *Service) requestWorkflowModel(ctx context.Context, root string, session *ChangeSession, runtime modelRuntime, messages []llm.ChatMessage, schema llm.JSONSchema) (modelOutput, error) {
	if limit := changeContextByteLimit(runtime); limit > 0 && len(messages[0].Content)+len(messages[1].Content) > limit {
		return modelOutput{}, fmt.Errorf("selected context exceeds configured %s token limit", runtime.effective.Profile)
	}
	timed, cancel := context.WithTimeout(ctx, duration(runtime.effective.Timeout))
	defer cancel()
	authorize := func(ctx context.Context) error {
		if err := requireMissionProviderAdmission(ctx, session); err != nil {
			return err
		}
		s.changesMu.Lock()
		defer s.changesMu.Unlock()
		current, currentRoot, err := s.loadChangeForAction(ctx, session.ID, identityForChange(session))
		if err != nil {
			return err
		}
		if currentRoot != root || current.Workflow == nil || session.Workflow == nil || current.Workflow.ID != session.Workflow.ID {
			return fmt.Errorf("workflow identity changed before model dispatch")
		}
		return s.requireProjectExecutionTrust(ctx, session.ProjectRevision)
	}
	timed, err := withProjectMemory(timed, root, session.ProjectID, session.ProjectRevision, "proposal tests review lessons", changeMemorySources(session, false), authorize)
	if err != nil {
		return modelOutput{}, err
	}
	result, err := s.retryRequestAuthorized(timed, runtime, messages, &schema, authorize)
	if err == nil {
		err = timed.Err()
	}
	return result, err
}

func parseWorkflowReview(output string) (*ChangeWorkflowReview, error) {
	if len(output) > 32*1024 {
		return nil, fmt.Errorf("workflow review exceeds its limit")
	}
	output = llm.UnwrapMarkdownJSON(output)
	// The provider may not choose the proposal identity attached by the daemon.
	var wire struct {
		Verdict  string   `json:"verdict"`
		Summary  string   `json:"summary"`
		Findings []string `json:"findings"`
	}
	decoder := json.NewDecoder(strings.NewReader(output))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return nil, err
	}
	if decoder.Decode(new(any)) != io.EOF || wire.Findings == nil || len(wire.Findings) > 16 {
		return nil, fmt.Errorf("invalid workflow review response")
	}
	review := &ChangeWorkflowReview{Verdict: wire.Verdict, Summary: wire.Summary, Findings: wire.Findings}
	return review, validateWorkflowReview(review)
}

func validateWorkflowReview(review *ChangeWorkflowReview) error {
	if review.Verdict != "approve" && review.Verdict != "changes_requested" {
		return fmt.Errorf("invalid workflow review verdict")
	}
	if !boundedWorkflowText(review.Summary, 4096) || len(review.Findings) > 80 {
		return fmt.Errorf("invalid workflow review summary")
	}
	if (review.Verdict == "approve") != (len(review.Findings) == 0) {
		return fmt.Errorf("workflow verdict and findings disagree")
	}
	for _, finding := range review.Findings {
		if !boundedWorkflowText(finding, 1024) {
			return fmt.Errorf("invalid workflow review finding")
		}
	}
	return nil
}

func boundedWorkflowText(text string, limit int) bool {
	return strings.TrimSpace(text) != "" && len(text) <= limit && utf8.ValidString(text) && !strings.ContainsRune(text, 0)
}
