package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func executionClientFixture() (AgentSettingsScope, AgentMissionExecution) {
	scope, _, planning := missionClientFixture()
	mission := planning.Mission
	mission.Approved = true
	mission.Instructions = map[string]string{}
	for _, source := range mission.Plan.Sources {
		mission.Instructions[source.Path] = "instruction-hash"
	}
	base := GitIdentity{Head: strings.Repeat("a", 40), Branch: "main"}
	record := MissionExecution{ID: "mission-run-abcdefghijklmnopqrstuvwx", Revision: 1, CatalogRevision: 1, Mission: mission, Base: base, Current: MissionCheckpoint{ProjectRevision: scope.ProjectRevision, Base: base, Sources: mission.Plan.Sources, Instructions: mission.Instructions}, State: "prepared"}
	for _, child := range mission.Plan.Children {
		record.Children = append(record.Children, MissionExecutionChild{ID: child.ID, State: "pending", Limits: child.Limits})
	}
	return scope, AgentMissionExecution{Execution: record, SourceStatus: "current", OwnerState: "idle", ReadyChildren: []string{record.Children[0].ID}}
}

func TestMissionExecutionClientPreservesAbsentFailedAndInvalidInspection(t *testing.T) {
	scope, value := executionClientFixture()
	for _, status := range []int{http.StatusNotFound, http.StatusInternalServerError, http.StatusOK} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Query().Get("worktree_path") != scope.WorktreePath {
					t.Error("inspection lost its passive checkout scope")
				}
				w.WriteHeader(status)
				_ = json.NewEncoder(w).Encode(value)
			}))
			defer server.Close()
			cl, err := NewClient(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer cl.Close()
			got, err := cl.MissionExecution(t.Context(), "read", scope, value.Execution.Mission.ID)
			if status == 404 && (got != nil || err != nil) {
				t.Fatal("not-prepared state lost", got, err)
			}
			if status == 500 && err == nil {
				t.Fatal("persistence failure became an unprepared mission")
			}
			if status == 200 && (err != nil || got == nil) {
				t.Fatal(got, err)
			}
		})
	}
}

func revalidatedClientFixture() (AgentSettingsScope, AgentMissionExecution) {
	scope, value := executionClientFixture()
	r := &value.Execution
	child := &r.Children[0]
	child.State, child.Generation, child.WaveID = "awaiting_review", 0, "wave-abcdefghijklmnopqrstuvwx"
	child.Directory, child.Branch, child.BaseHead = r.ID+"-"+child.ID, "codex/"+r.ID+"-"+child.ID, r.Base.Head
	child.ProjectID, child.ProjectRevision, child.SessionID, child.WorkflowID = "child", "child-revision", "session-old", "workflow-old"
	child.ProposalHash, child.ProposalRevision = strings.Repeat("c", 64), 2
	previous := *child
	child.Generation, child.WorkflowID, child.SessionID = 1, "", "session-fresh"
	child.Directory += "-r1"
	child.Branch += "-r1"
	entry := MissionRevalidation{ID: "revalidation-abcdefghijklmnopqrstuvwx", Owner: "owner", State: "awaiting_review", Previous: previous, Candidate: *child, Before: r.Current, Source: GitIdentity{Head: previous.BaseHead, Branch: previous.Branch}, AllowanceMillis: 1000}
	entry.Evidence.Key.Session = AgentSessionKey{App: "mini-orca-missions", User: "local", ID: r.ID}
	entry.Evidence.Key.Name, entry.Evidence.Hash, entry.Evidence.Version = child.ID+"-proposal.json", strings.Repeat("e", 64), 1
	r.Revalidations = []MissionRevalidation{entry}
	r.State, value.ReadyChildren = "awaiting_review", nil
	return scope, value
}

func TestMissionExecutionClientRejectsLostCheckpointCandidateAndEvidence(t *testing.T) {
	for name, mutate := range map[string]func(*AgentMissionExecution){
		"foreign owner":        func(v *AgentMissionExecution) { v.Execution.Mission.ProjectID = "other" },
		"checkpoint":           func(v *AgentMissionExecution) { v.Execution.Current.Base.Head = "unrecorded" },
		"scope":                func(v *AgentMissionExecution) { v.Execution.Current.Sources = nil },
		"budget":               func(v *AgentMissionExecution) { v.Execution.Children[0].Limits.ModelCalls++ },
		"path traversal":       func(v *AgentMissionExecution) { v.Execution.Children[0].Directory = "../foreign" },
		"workflow reuse":       func(v *AgentMissionExecution) { v.Execution.Children[0].WorkflowID = "old-model-authority" },
		"generation":           func(v *AgentMissionExecution) { v.Execution.Revalidations[0].Candidate.Generation = 4 },
		"missing lineage":      func(v *AgentMissionExecution) { v.Execution.Revalidations = nil },
		"unselected candidate": func(v *AgentMissionExecution) { v.Execution.Children[0].SessionID = "forged-session" },
		"archive":              func(v *AgentMissionExecution) { v.Execution.Revalidations[0].Evidence.Key.Session.ID = "other" },
		"owner state":          func(v *AgentMissionExecution) { v.OwnerState = "invented" },
	} {
		t.Run(name, func(t *testing.T) {
			scope, value := revalidatedClientFixture()
			if err := validateMissionExecutionReceipt(scope, value.Execution.Mission.ID, value); err != nil {
				t.Fatal("invalid fixture", err)
			}
			mutate(&value)
			if err := validateMissionExecutionReceipt(scope, value.Execution.Mission.ID, value); err == nil {
				t.Fatal("invalid authority accepted")
			}
		})
	}
}

func TestMissionExecutionClientRejectsUnrequestedGitMutation(t *testing.T) {
	scope, value := executionClientFixture()
	value.Execution.Revision++
	value.Git = &GitMutationResult{State: "merged"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _ = json.NewEncoder(w).Encode(value) }))
	defer server.Close()
	cl, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	request := MissionExecutionRequest{ProjectIdentity: scope.ProjectIdentity, Action: "start", ExecutionID: value.Execution.ID, ExecutionRevision: 1, PlanHash: value.Execution.Mission.PlanHash, Confirm: true, ConfirmExecution: true}
	if _, err := cl.ControlMissionExecution(t.Context(), "control", scope, value.Execution.Mission.ID, request); err == nil {
		t.Fatal("start accepted an unrelated Git mutation")
	}
}
