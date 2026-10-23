package domain

import (
	"slices"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

func progressFixture() (*client.ChangeSession, *client.AgentRunDetail) {
	change := &client.ChangeSession{ID: "change", ProjectIdentity: client.ProjectIdentity{ProjectID: "project", ProjectRevision: "rev"}, Hash: "proposal", Workflow: &client.ChangeWorkflow{ID: "workflow", Status: "failed", Stages: []client.ChangeWorkflowStage{
		{Name: "create", Status: "completed"}, {Name: "test", Status: "failed"}, {Name: "review", Status: "pending"}, {Name: "human_review", Status: "blocked"},
	}}, Checks: []client.Check{{Name: "tests", State: "failed"}}}
	run := client.AgentRun{ID: "run", RootID: "run", ChangeSessionID: change.ID, WorkflowID: change.Workflow.ID, ProjectID: change.ProjectID, ProjectRevision: change.ProjectRevision, Status: "failed"}
	return change, &client.AgentRunDetail{Run: run}
}

func nodeEvent(detail *client.AgentRunDetail, seq uint64, node, state string) client.AgentRunEvent {
	return client.AgentRunEvent{Sequence: seq, RunID: detail.Run.ID, InvocationID: detail.Run.ID, ProjectID: detail.Run.ProjectID, ProjectRevision: detail.Run.ProjectRevision, Kind: "node", NodeID: node, State: state, Time: "2026-10-24T01:00:00Z"}
}

func stageByID(t *testing.T, stages []AgentStage, id string) AgentStage {
	t.Helper()
	index := slices.IndexFunc(stages, func(stage AgentStage) bool { return stage.ID == id })
	if index < 0 {
		t.Fatalf("missing %s in %+v", id, stages)
	}
	return stages[index]
}

func TestNativeStagesSeparateGenerationAndFailedVerification(t *testing.T) {
	change, detail := progressFixture()
	legacy := WorkflowStages(change, nil)
	if stage := stageByID(t, legacy, "generate_tests"); stage.Label != "Testing" || stage.State != StageUnknown {
		t.Fatalf("invented separate legacy evidence: %+v", stage)
	}
	detail.Events = []client.AgentRunEvent{nodeEvent(detail, 3, "verify", "running"), nodeEvent(detail, 2, "generate_tests", "completed"), nodeEvent(detail, 1, "generate_tests", "running"), nodeEvent(detail, 1, "generate_tests", "running")}
	stages := WorkflowStages(change, detail)
	if stage := stageByID(t, stages, "generate_tests"); stage.Label != "Generate tests" || stage.State != StageCompleted {
		t.Fatalf("generation: %+v", stage)
	}
	if stage := stageByID(t, stages, "verify"); stage.State != StageFailed {
		t.Fatalf("verification: %+v", stage)
	}
	if stages[ActiveStage(stages)].ID != "verify" {
		t.Fatalf("active stage: %+v", stages)
	}
	if detail.Events[0].Sequence != 3 {
		t.Fatal("projection mutated input event order")
	}
}

func TestStagesPreserveIdentityUnknownNodesAndParallelReview(t *testing.T) {
	change, detail := progressFixture()
	change.Workflow.Stages = []client.ChangeWorkflowStage{{Name: "human_review", Status: "blocked"}, {Name: "review", Status: "running"}, {Name: "create", Status: "completed"}, {Name: "custom_lint", Status: "skipped"}}
	change.Workflow.Specialists = []client.ChangeSpecialistReview{{ID: "security", Status: "running"}, {ID: "architecture", Status: "running"}}
	detail.Run.Status, detail.Run.Active = "running", true
	detail.Events = []client.AgentRunEvent{nodeEvent(detail, 1, "new_quality_node", "running")}
	stages := WorkflowStages(change, detail)
	if stages[0].ID != "human_review" || stages[0].Label != "Human review" {
		t.Fatalf("positional labels returned: %+v", stages)
	}
	if stageByID(t, stages, "custom_lint").State != StageSkipped || stageByID(t, stages, "new_quality_node").State != StageRunning {
		t.Fatal("unknown recorded stages were lost")
	}
	if StageSummary(stages) != "Parallel review" || !stageByID(t, stages, "review_security").Parallel {
		t.Fatalf("parallel review: %+v", stages)
	}
}

func TestNativeCompletionDoesNotApproveHumanReview(t *testing.T) {
	change, detail := progressFixture()
	detail.Run.Status = "completed"
	detail.Events = []client.AgentRunEvent{nodeEvent(detail, 1, "human_review", "completed")}
	if got := stageByID(t, WorkflowStages(change, detail), "human_review").State; got != StageBlocked {
		t.Fatalf("execution promoted approval: %s", got)
	}
	change.Workflow.Input = &client.ChangeInput{Status: "waiting"}
	if got := stageByID(t, WorkflowStages(change, detail), "human_review").State; got != StageWaiting {
		t.Fatalf("pending input: %s", got)
	}
	change.ReviewedHash = "different-proposal"
	if got := stageByID(t, WorkflowStages(change, detail), "human_review").State; got != StageWaiting {
		t.Fatalf("stale approval accepted: %s", got)
	}
	change.ReviewedHash = change.Hash
	if got := stageByID(t, WorkflowStages(change, detail), "human_review").State; got != StageCompleted || change.State == "applied" {
		t.Fatalf("review receipt: %s", got)
	}
}

func TestNativeStageRejectsForeignScopeAndMissingTerminalEvidence(t *testing.T) {
	for _, status := range []string{"interrupted", "canceled", "completed"} {
		t.Run(status, func(t *testing.T) {
			change, detail := progressFixture()
			detail.Run.Status = status
			detail.Events = []client.AgentRunEvent{nodeEvent(detail, 1, "new_node", "running")}
			want := stageState(status)
			if status == "completed" {
				want = StageUnknown
			}
			if got := stageByID(t, WorkflowStages(change, detail), "new_node").State; got != want {
				t.Fatalf("state %s, want %s", got, want)
			}
		})
	}
	change, detail := progressFixture()
	foreign := nodeEvent(detail, 10, "foreign_node", "completed")
	foreign.ProjectRevision = "old"
	detail.Events = []client.AgentRunEvent{foreign}
	if stages := WorkflowStages(change, detail); slices.ContainsFunc(stages, func(s AgentStage) bool { return s.ID == "foreign_node" }) {
		t.Fatal("foreign event accepted")
	}
	detail.Run.WorkflowID = "old-workflow"
	detail.Events = []client.AgentRunEvent{nodeEvent(detail, 11, "new_node", "completed")}
	if NativeProgressMatches(change, detail) {
		t.Fatal("old workflow accepted")
	}
}
