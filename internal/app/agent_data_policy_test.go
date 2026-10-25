package app

import (
	"errors"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func TestCloudDataPolicyRejectsActiveAndForeignHistory(t *testing.T) {
	s, store, q := agentHistoryFixture(t)
	target := cloudTestTarget()
	target.Service, target.Resource = "vertex_session", "12345"
	s.integrations.config.Cloud = []config.CloudTarget{target}
	run := seedAgentRun(t, s, store, "copy-fixture", "", time.Now().UTC().Add(-time.Minute))
	selection := CloudSelection{ProjectID: q.ProjectID, ProjectRevision: q.ProjectRevision, TargetID: target.ID, Kind: "session", SourceID: run.ID}
	if _, err := s.PreviewCloudCopy(t.Context(), selection); err == nil {
		t.Fatal("active history admitted")
	}
	if err := store.FinishRun(t.Context(), run.ID, "completed", nil); err != nil {
		t.Fatal(err)
	}
	p, err := s.PreviewCloudCopy(t.Context(), selection)
	if err != nil || p.SourceRevision != q.ProjectRevision || p.Size == 0 {
		t.Fatal(p, err)
	}
	checkpoint := run
	checkpoint.ID, checkpoint.Session.ID, checkpoint.SchemaVersion = "checkpoint", "checkpoint", "confirmation-v1"
	if _, err := store.CreateSession(t.Context(), checkpoint.Session, agentstate.State{}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginRun(t.Context(), checkpoint, nil); err != nil {
		t.Fatal(err)
	}
	input := agentstate.InputCheckpoint{RequestID: "input", NodeID: "human_review", InvocationID: "invocation", ExpiresAt: time.Now().UTC().Add(time.Hour)}
	if err := store.PauseForInput(t.Context(), checkpoint.ID, input); err != nil {
		t.Fatal(err)
	}
	selection.SourceID = checkpoint.ID
	if _, err := s.PreviewCloudCopy(t.Context(), selection); err == nil {
		t.Fatal("pending approval copied")
	}
	_, binding, err := s.agentJournal(s.manager.Root())
	if err != nil {
		t.Fatal(err)
	}
	admission := agentstate.InputAdmission{RunID: checkpoint.ID, RequestID: input.RequestID, RequestHash: checkpoint.RequestHash, Binding: binding}
	if _, err := store.ReceiveInput(t.Context(), admission, "approve"); err != nil {
		t.Fatal(err)
	}
	if err := store.FinishInput(t.Context(), admission); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PreviewCloudCopy(t.Context(), selection); err == nil {
		t.Fatal("approval authority migrated in completed history")
	}
	selection.SourceID = run.ID
	selection.ProjectRevision = "stale"
	if _, err := s.PreviewCloudCopy(t.Context(), selection); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatal("stale checkout admitted", err)
	}
	selection.ProjectRevision = q.ProjectRevision
	selection.SourceID = "foreign"
	if _, err := s.PreviewCloudCopy(t.Context(), selection); !errors.Is(err, ErrAgentRunNotFound) {
		t.Fatal("unknown run admitted", err)
	}
	selection.SourceID = run.ID
	selection.Kind = "memory"
	if _, err := s.PreviewCloudCopy(t.Context(), selection); err == nil {
		t.Fatal("unsupported memory migrated")
	}
}

func TestCloudConfigChangesAndBusyReceiptsBlockEffects(t *testing.T) {
	s, store, selection, remote := cloudAppFixture(t)
	p, err := s.PreviewCloudCopy(t.Context(), selection)
	if err != nil {
		t.Fatal(err)
	}
	s.integrations.config.Cloud[0].RetentionDays++
	if _, err := s.ControlCloudCopy(t.Context(), cloudCopyRequest(p)); !errors.Is(err, agentstate.ErrCloudConflict) || remote.copies != 0 {
		t.Fatal("changed data policy accepted", err)
	}
	p, err = s.PreviewCloudCopy(t.Context(), selection)
	if err != nil {
		t.Fatal(err)
	}
	record, _, _, err := s.claimCloudCopy(t.Context(), cloudCopyRequest(p), func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ControlCloudCopy(t.Context(), CloudControl{ProjectID: selection.ProjectID, ProjectRevision: selection.ProjectRevision, ID: record.ID, Version: record.Version, Action: "delete", Confirm: true}); !errors.Is(err, agentstate.ErrCloudConflict) || remote.deletes != 0 {
		t.Fatal("concurrent control admitted", err)
	}
	if rows, err := store.CloudCopies(t.Context(), selection.ProjectID); err != nil || len(rows) != 1 || rows[0].State != "copying" {
		t.Fatal(rows, err)
	}
}

func TestCloudCopyRequiresTheEntireWorkflowToBeComplete(t *testing.T) {
	s, store, q := agentHistoryFixture(t)
	target := cloudTestTarget()
	target.Service, target.Resource = "vertex_session", "12345"
	s.integrations.config.Cloud = []config.CloudTarget{target}
	parent := seedAgentRun(t, s, store, "parent", "", time.Now().UTC().Add(-time.Minute))
	child := seedAgentRun(t, s, store, "child", parent.ID, time.Now().UTC().Add(-30*time.Second))
	if err := store.FinishRun(t.Context(), child.ID, "completed", nil); err != nil {
		t.Fatal(err)
	}
	selection := CloudSelection{ProjectID: q.ProjectID, ProjectRevision: q.ProjectRevision, TargetID: target.ID, Kind: "session", SourceID: child.ID}
	if _, err := s.PreviewCloudCopy(t.Context(), selection); err == nil {
		t.Fatal("completed child escaped active parent admission")
	}
	if err := store.FinishRun(t.Context(), parent.ID, "completed", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PreviewCloudCopy(t.Context(), selection); err != nil {
		t.Fatal("completed workflow unavailable", err)
	}

	parent = seedAgentRun(t, s, store, "approval-parent", "", time.Now().UTC().Add(-time.Minute))
	checkpoint := parent
	checkpoint.ID, checkpoint.Session.ID, checkpoint.ParentID, checkpoint.SchemaVersion = "approval-child", "approval-child", parent.ID, "confirmation-v1"
	if _, err := store.CreateSession(t.Context(), checkpoint.Session, agentstate.State{}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginRun(t.Context(), checkpoint, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.PauseForInput(t.Context(), checkpoint.ID, agentstate.InputCheckpoint{RequestID: "input", NodeID: "human_review", InvocationID: "invocation", ExpiresAt: time.Now().UTC().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := store.FinishRun(t.Context(), parent.ID, "completed", nil); err != nil {
		t.Fatal(err)
	}
	selection.SourceID = parent.ID
	if _, err := s.PreviewCloudCopy(t.Context(), selection); err == nil {
		t.Fatal("completed root hid a waiting approval")
	}
}
