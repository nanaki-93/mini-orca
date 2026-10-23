package domain

import (
	"cmp"
	"slices"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

type StageState string

const (
	StageUnknown     StageState = "unknown"
	StagePending     StageState = "pending"
	StageRunning     StageState = "running"
	StageCompleted   StageState = "completed"
	StageFailed      StageState = "failed"
	StageWaiting     StageState = "waiting"
	StageBlocked     StageState = "blocked"
	StageCanceled    StageState = "canceled"
	StageInterrupted StageState = "interrupted"
	StageStale       StageState = "stale"
	StageSkipped     StageState = "skipped"
)

type AgentStage struct {
	ID, Label                                   string
	State                                       StageState
	Parallel                                    bool
	Model                                       *client.Model
	StartedAt, FinishedAt, Reason, InvocationID string
}

func CanonicalStageID(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "create", "architect & code", "architect":
		return "create"
	case "test", "verify & test":
		return "generate_tests"
	case "review", "critic review", "critic":
		return "review"
	case "human_review", "human review":
		return "human_review"
	default:
		return name
	}
}

func StageLabel(id string) string {
	switch id {
	case "create":
		return "Create"
	case "generate_tests":
		return "Generate tests"
	case "verify":
		return "Verify"
	case "review":
		return "Review"
	case "human_review":
		return "Human review"
	case "review_correctness":
		return "Correctness"
	case "review_architecture":
		return "Architecture"
	case "review_test_quality":
		return "Test quality"
	case "review_security":
		return "Security"
	case "review_performance":
		return "Performance"
	default:
		return strings.ReplaceAll(id, "_", " ")
	}
}

func stageState(value string) StageState {
	switch value {
	case "pending", "queued":
		return StagePending
	case "running", "canceling":
		return StageRunning
	case "completed", "passed", "approved", "applied":
		return StageCompleted
	case "failed", "rejected", "changes_requested":
		return StageFailed
	case "waiting", "waiting_input", "awaiting_human_review", "ready", "checked":
		return StageWaiting
	case "blocked":
		return StageBlocked
	case "canceled":
		return StageCanceled
	case "interrupted":
		return StageInterrupted
	case "stale", "outdated", "expired":
		return StageStale
	case "skipped":
		return StageSkipped
	default:
		return StageUnknown
	}
}

// WorkflowStages uses recorded identities, never a positional four-stage model.
// Legacy workflow summaries remain readable when native progress is unavailable.
func WorkflowStages(change *client.ChangeSession, detail *client.AgentRunDetail) []AgentStage {
	if change == nil {
		return nil
	}
	stages := recordedWorkflowStages(change)
	if NativeProgressMatches(change, detail) {
		stages = mergeNativeStages(stages, detail)
	}
	return applyHumanStage(stages, change)
}

func NativeProgressMatches(change *client.ChangeSession, detail *client.AgentRunDetail) bool {
	if change == nil || change.Workflow == nil || detail == nil {
		return false
	}
	run := detail.Run
	return run.ID != "" && run.RootID == run.ID && run.ProjectID == change.ProjectID && run.ProjectRevision == change.ProjectRevision && run.ChangeSessionID == change.ID && run.WorkflowID == change.Workflow.ID
}

func recordedWorkflowStages(change *client.ChangeSession) []AgentStage {
	var stages []AgentStage
	if change.Workflow != nil {
		for _, recorded := range change.Workflow.Stages {
			id := CanonicalStageID(recorded.Name)
			if id == "" {
				continue
			}
			stage := AgentStage{ID: id, Label: StageLabel(id), State: stageState(recorded.Status), Model: recorded.Model, StartedAt: recorded.StartedAt, FinishedAt: recorded.FinishedAt, Reason: recorded.Reason}
			if id == "generate_tests" && recorded.Name != id {
				stage.Label = "Testing"
				if len(change.Checks) > 0 {
					stage.State = StageUnknown
					stage.Reason = strings.TrimSpace(stage.Reason + " Combined testing summary; inspect verification for recorded check results.")
				}
			}
			stages = insertStage(stages, stage)
		}
		stages = addSpecialistStages(stages, change.Workflow.Specialists)
	}
	if len(stages) == 0 {
		state := StagePending
		if len(change.Changes) > 0 {
			state = StageCompleted
		}
		stages = append(stages, AgentStage{ID: "create", Label: "Proposal", State: state})
		stages = append(stages, AgentStage{ID: "human_review", Label: "Human review", State: StageBlocked})
	}
	if len(change.Checks) > 0 {
		stages = insertStage(stages, AgentStage{ID: "verify", Label: "Verify", State: checkStageState(change.Checks)}, "generate_tests", "create")
	}
	return stages
}

func checkStageState(checks []client.Check) StageState {
	state := StageSkipped
	for _, check := range checks {
		if check.State != "passed" && check.State != "skipped" {
			return stageState(check.State)
		}
		if check.State == "passed" {
			state = StageCompleted
		}
	}
	return state
}

func addSpecialistStages(stages []AgentStage, specialists []client.ChangeSpecialistReview) []AgentStage {
	after := "review"
	for _, report := range specialists {
		id := "review_" + report.ID
		stages = insertStage(stages, AgentStage{ID: id, Label: StageLabel(id), State: stageState(report.Status), Parallel: true, Model: report.Model, Reason: report.Reason}, after)
		after = id
	}
	return stages
}

func insertStage(stages []AgentStage, stage AgentStage, after ...string) []AgentStage {
	if slices.ContainsFunc(stages, func(s AgentStage) bool { return s.ID == stage.ID }) {
		return stages
	}
	for _, id := range after {
		if index := slices.IndexFunc(stages, func(s AgentStage) bool { return s.ID == id }); index >= 0 {
			return slices.Insert(stages, index+1, stage)
		}
	}
	return append(stages, stage)
}

func mergeNativeStages(stages []AgentStage, detail *client.AgentRunDetail) []AgentStage {
	invocations := make(map[string]client.AgentRun, len(detail.Invocations))
	invocations[detail.Run.ID] = detail.Run
	for _, run := range detail.Invocations {
		if run.RootID == detail.Run.ID && run.ProjectID == detail.Run.ProjectID && run.ProjectRevision == detail.Run.ProjectRevision {
			invocations[run.ID] = run
		}
	}
	events := slices.Clone(detail.Events)
	slices.SortStableFunc(events, func(a, b client.AgentRunEvent) int { return cmp.Compare(a.Sequence, b.Sequence) })
	seen := make(map[string]uint64)
	for _, event := range events {
		if !matchingNodeEvent(event, detail.Run) || event.Sequence <= seen[event.NodeID] {
			continue
		}
		seen[event.NodeID] = event.Sequence
		index := slices.IndexFunc(stages, func(s AgentStage) bool { return s.ID == event.NodeID })
		if index < 0 {
			stages = insertStage(stages, AgentStage{ID: event.NodeID, Label: StageLabel(event.NodeID)}, "generate_tests")
			index = slices.IndexFunc(stages, func(s AgentStage) bool { return s.ID == event.NodeID })
		}
		applyNodeEvent(&stages[index], event, invocations[event.InvocationID])
	}
	return stages
}

func matchingNodeEvent(event client.AgentRunEvent, run client.AgentRun) bool {
	return event.Kind == "node" && visibleStageNode(event.NodeID) && event.ProjectID == run.ProjectID && event.ProjectRevision == run.ProjectRevision && event.RunID == run.ID
}

func applyNodeEvent(stage *AgentStage, event client.AgentRunEvent, run client.AgentRun) {
	stage.InvocationID = event.InvocationID
	stage.Label = StageLabel(stage.ID)
	if event.State == "running" {
		stage.StartedAt, stage.FinishedAt = event.Time, ""
	}
	if event.State == "completed" {
		stage.FinishedAt = event.Time
	}
	// The host's human decision and verification evidence remain authoritative.
	if stage.ID == "human_review" {
		return
	}
	if event.State == "completed" && slices.Contains([]StageState{StageFailed, StageStale, StageSkipped}, stage.State) && stage.ID != "generate_tests" {
		return
	}
	stage.State = stageState(event.State)
	if stage.State == StageRunning {
		stage.State = retainedStageState(run)
	}
}

func visibleStageNode(id string) bool {
	return id != "" && id != "agent" && id != "completion" && id != "join_reviews" && id != "review_result" && !strings.HasPrefix(id, "attempt_")
}

func retainedStageState(run client.AgentRun) StageState {
	if run.Status == "running" && run.Active {
		return StageRunning
	}
	if run.Status == "completed" {
		return StageUnknown
	} // Missing terminal node evidence is not success.
	return stageState(run.Status)
}

func applyHumanStage(stages []AgentStage, change *client.ChangeSession) []AgentStage {
	for i := range stages {
		if stages[i].ID != "human_review" {
			continue
		}
		switch {
		case change.ReviewedHash != "" && change.ReviewedHash == change.Hash:
			stages[i].State = StageCompleted
		case change.Workflow != nil && change.Workflow.Input != nil:
			stages[i].State = stageState(change.Workflow.Input.Status)
		case change.Workflow != nil && change.Workflow.Status == "changes_requested":
			stages[i].State = StageBlocked
		case change.Workflow != nil && change.Workflow.Status == "awaiting_human_review":
			stages[i].State = StageWaiting
		case change.State == "checked" || change.State == "ready":
			stages[i].State = StageWaiting
		}
	}
	return stages
}

func ActiveStage(stages []AgentStage) int {
	for _, states := range [][]StageState{{StageRunning}, {StageFailed, StageInterrupted, StageStale, StageWaiting}, {StagePending}} {
		for i, stage := range stages {
			if slices.Contains(states, stage.State) {
				return i
			}
		}
	}
	return max(0, len(stages)-1)
}

func StageSummary(stages []AgentStage) string {
	if len(stages) == 0 {
		return "Not started"
	}
	parallel := 0
	for _, stage := range stages {
		if stage.Parallel && stage.State == StageRunning {
			parallel++
		}
	}
	if parallel > 1 {
		return "Parallel review"
	}
	return stages[ActiveStage(stages)].Label
}
