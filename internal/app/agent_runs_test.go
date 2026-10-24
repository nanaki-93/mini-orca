package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func agentHistoryFixture(t *testing.T) (*Service, *agentstate.Store, AgentRunQuery) {
	t.Helper()
	s, _ := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	store, _, err := s.agentJournal(s.manager.Root())
	if err != nil {
		t.Fatal(err)
	}
	index, err := s.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	return s, store, AgentRunQuery{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision}
}

func seedAgentRun(t *testing.T, s *Service, store *agentstate.Store, id, parent string, started time.Time) agentstate.Run {
	t.Helper()
	_, binding, err := s.agentJournal(s.manager.Root())
	if err != nil {
		t.Fatal(err)
	}
	key := agentstate.Key{App: "mini-orca", User: "local", ID: id}
	if _, err := store.CreateSession(t.Context(), key, agentstate.State{"private": json.RawMessage(`"PRIVATE_PROMPT"`)}); err != nil {
		t.Fatal(err)
	}
	budgetID := "budget-" + id
	if parent != "" {
		budgetID = "budget-" + parent
	}
	if parent == "" {
		if _, err := store.Checkpoint(t.Context(), agentstate.BudgetRecord{ID: budgetID, Binding: binding, Limits: s.agentRuntime.Limits, Sequence: 1}, nil); err != nil {
			t.Fatal(err)
		}
	}
	run := agentstate.Run{ID: id, ParentID: parent, Session: key, BudgetID: budgetID, RequestHash: strings.Repeat("a", 64), SchemaHash: strings.Repeat("b", 64), SchemaVersion: "completion-v1", Status: "running", Started: started, Updated: started}
	if _, err := store.BeginRun(t.Context(), run, nil); err != nil {
		t.Fatal(err)
	}
	return run
}

func TestAgentHistoryPaginationScopeInterruptionAndPrivateText(t *testing.T) {
	s, store, query := agentHistoryFixture(t)
	start := time.Now().UTC().Add(-time.Hour)
	first := seedAgentRun(t, s, store, "first", "", start)
	second := seedAgentRun(t, s, store, "second", "", start.Add(time.Second))
	seedAgentRun(t, s, store, "child", second.ID, start.Add(2*time.Second))
	if _, err := store.SaveArtifact(t.Context(), agentstate.ArtifactWrite{Key: agentstate.ArtifactKey{Session: first.Session, Name: "completion.json"}, Kind: "inline", MIME: "application/json", Data: []byte(`{"Content":"PRIVATE_RESULT"}`), Provenance: map[string]string{"run": first.ID}}); err != nil {
		t.Fatal(err)
	}
	query.Limit = 1
	page, err := s.ListAgentRuns(t.Context(), query)
	if err != nil || len(page.Runs) != 1 || page.Runs[0].ID != "second" || page.Runs[0].Status != "interrupted" || page.Runs[0].CanCancel || page.NextCursor == "" {
		t.Fatalf("first page: %+v %v", page, err)
	}
	query.Cursor = page.NextCursor
	next, err := s.ListAgentRuns(t.Context(), query)
	if err != nil || len(next.Runs) != 1 || next.Runs[0].ID != "first" || next.NextCursor != "" {
		t.Fatalf("second page: %+v %v", next, err)
	}
	query.Cursor = ""
	detail, err := s.AgentRunDetail(t.Context(), query, first.ID, false)
	if err != nil || detail.Text != nil {
		t.Fatalf("private detail default: %+v %v", detail, err)
	}
	data, _ := json.Marshal(detail)
	if strings.Contains(string(data), "PRIVATE_") {
		t.Fatal("private content in metadata")
	}
	detail, err = s.AgentRunDetail(t.Context(), query, first.ID, true)
	if err != nil || detail.Text.Text != "PRIVATE_RESULT" || detail.Text.Artifact == nil {
		t.Fatalf("explicit text: %+v %v", detail, err)
	}
	query.ProjectRevision = "foreign"
	if _, err := s.ListAgentRuns(t.Context(), query); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("foreign scope: %v", err)
	}
	if _, err := s.AgentRunDetail(t.Context(), query, first.ID, true); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("foreign private detail: %v", err)
	}
}

func TestAgentHistoryControlOnlyOwnsCurrentActiveRoots(t *testing.T) {
	s, store, query := agentHistoryFixture(t)
	root := seedAgentRun(t, s, store, "root", "", time.Now().UTC())
	child := seedAgentRun(t, s, store, "child", root.ID, time.Now().UTC())
	canceled := false
	s.agentRuns.started(root, func() { canceled = true })
	s.agentRuns.started(child, func() { t.Error("child cancellation bypassed root") })
	request := AgentRunControl{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Action: "cancel"}
	if _, err := s.ControlAgentRun(t.Context(), child.ID, request); !errors.Is(err, project.ErrRevisionConflict) || canceled {
		t.Fatalf("child cancellation: %v", err)
	}
	detail, err := s.ControlAgentRun(t.Context(), root.ID, request)
	if err != nil || !canceled || detail.Run.Status != "canceling" || detail.Run.CanCancel {
		t.Fatalf("root cancellation: %+v %v", detail, err)
	}
	s.agentRuns.ended(root.ID)
	if _, err := s.ControlAgentRun(t.Context(), root.ID, request); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("historical cancellation: %v", err)
	}
	request.Action = "retry"
	if _, err := s.ControlAgentRun(t.Context(), root.ID, request); err == nil {
		t.Fatal("history started arbitrary work")
	}
	if _, err := s.AgentRunDetail(t.Context(), query, "missing", false); !errors.Is(err, ErrAgentRunNotFound) {
		t.Fatalf("missing invocation: %v", err)
	}
}

func TestAgentHistoryInputUsesExactDomainProposal(t *testing.T) {
	s, root, ready := readyConfirmation(t)
	query := AgentRunQuery{ProjectID: ready.ProjectID, ProjectRevision: ready.ProjectRevision}
	history, err := s.ListAgentRuns(t.Context(), query)
	if err != nil || len(history.Runs) != 1 || history.Runs[0].Status != "waiting_input" {
		t.Fatalf("workflow root grouping: %+v %v", history, err)
	}
	if history.Runs[0].ChangeSessionID != ready.ID || history.Runs[0].WorkflowID != ready.Workflow.ID {
		t.Fatal("root lost domain workflow identity")
	}
	detail, err := s.AgentRunDetail(t.Context(), query, ready.Workflow.Input.RunID, false)
	if err != nil || detail.Input == nil || detail.Input.RequestID != ready.Workflow.Input.RequestID || detail.Run.RootID != history.Runs[0].ID {
		t.Fatalf("checkpoint detail: %+v %v", detail, err)
	}
	request := inputResponse(ready, "approve")
	forged := request
	forged.Hash = "foreign"
	if _, err := s.RespondAgentRunInput(t.Context(), ready.Workflow.Input.RunID, forged); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("forged proposal accepted: %v", err)
	}
	approved, err := s.RespondAgentRunInput(t.Context(), ready.Workflow.Input.RunID, request)
	if err != nil || approved.ReviewedHash != ready.Hash {
		t.Fatalf("native input review: %+v %v", approved, err)
	}
	if _, err := os.Stat(filepath.Join(root, "new.go")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("input endpoint applied source")
	}
	done, err := s.ListAgentRuns(t.Context(), query)
	if err != nil || done.Runs[0].Status != "completed" {
		t.Fatalf("terminal input not recovered: %+v %v", done, err)
	}
}
