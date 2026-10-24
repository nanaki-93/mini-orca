package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func memoryDraftFixture(t *testing.T, service *Service, path string) MemoryMutation {
	t.Helper()
	index, err := service.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	file, err := service.manager.IndexedFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return MemoryMutation{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Action: "create", Draft: &MemoryDraft{Kind: "decision", Title: "main semantic review lesson", Text: "RETAINED_PRIVATE_ADVICE", RetentionDays: 30, Sources: []agentstate.MemorySource{{Path: path, Hash: file.ContentHash}}}}
}

func retainMemoryFixture(t *testing.T, s *Service, request MemoryMutation) *AgentMemory {
	t.Helper()
	record, err := s.MutateMemory(t.Context(), "", request)
	if err != nil {
		t.Fatal(err)
	}
	record, err = s.MutateMemory(t.Context(), record.ID, MemoryMutation{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision, Version: record.Version, Action: "accept"})
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func TestMemoryCurationGuardsVersionsAndKeepsReadsPassive(t *testing.T) {
	s, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	request := memoryDraftFixture(t, s, "main.go")
	query := AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}
	rows, err := s.ListMemories(t.Context(), query)
	if err != nil || len(rows) != 0 {
		t.Fatalf("empty read=%+v %v", rows, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".mini-orca", "agents")); !os.IsNotExist(err) {
		t.Fatalf("read created agent storage: %v", err)
	}
	record := retainMemoryFixture(t, s, request)
	rows, err = s.ListMemories(t.Context(), query)
	if err != nil || len(rows) != 1 || rows[0].Text != "" || rows[0].SourceStatus != "unchecked" {
		t.Fatalf("list leaked text or claimed source validation: %+v %v", rows, err)
	}
	detail, err := s.MemoryDetail(t.Context(), query, record.ID)
	if err != nil || detail.Text != request.Draft.Text || detail.SourceStatus != "current" {
		t.Fatalf("detail=%+v %v", detail, err)
	}
	request.Action, request.Version = "edit", record.Version
	request.Draft.Text = "REVISED_PRIVATE_ADVICE"
	revised, err := s.MutateMemory(t.Context(), record.ID, request)
	if err != nil || revised.State != "candidate" || revised.Version != 3 {
		t.Fatalf("edit=%+v %v", revised, err)
	}
	for _, action := range []string{"accept", "forget"} {
		_, err := s.MutateMemory(t.Context(), record.ID, MemoryMutation{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Version: record.Version, Action: action})
		if !errors.Is(err, ErrMemoryConflict) {
			t.Fatalf("stale %s=%v", action, err)
		}
	}
	if _, err := s.MutateMemory(t.Context(), revised.ID, MemoryMutation{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Version: revised.Version, Action: "forget"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MemoryDetail(t.Context(), query, record.ID); !errors.Is(err, ErrMemoryNotFound) {
		t.Fatal(err)
	}
	query.ProjectID = "other"
	if _, err := s.ListMemories(t.Context(), query); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatal(err)
	}
}

func TestMemoryCurationRejectsStaleSourcesAndLabelsChangedDetail(t *testing.T) {
	s, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	request := memoryDraftFixture(t, s, "main.go")
	record, err := s.MutateMemory(t.Context(), "", request)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc Changed() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = s.MutateMemory(t.Context(), record.ID, MemoryMutation{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision, Action: "accept", Version: record.Version})
	if !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("stale candidate accepted: %v", err)
	}
	detail, err := s.MemoryDetail(t.Context(), AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}, record.ID)
	if err != nil || !detail.Stale || detail.SourceStatus != "changed" {
		t.Fatalf("stale detail=%+v %v", detail, err)
	}
}

func TestMemoryVerifiedOutcomeRequiresLiveChecksAndDerivesReceipt(t *testing.T) {
	server := changeProvider(t, func() string {
		return `{"explanation":"Change output.","changes":[{"path":"main.go","content":"package main\nfunc Run() { println(\"changed\") }\n"}]}`
	})
	s, _ := newSemanticAnalysisService(t, server.URL, 0)
	session := prepareChangeFixture(t, s, "main.go")
	request := memoryDraftFixture(t, s, "main.go")
	identity := changeIdentity(session)
	request.Draft.Kind, request.Draft.SessionID, request.Draft.Identity, request.Draft.Text = "verified_outcome", session.ID, &identity, ""
	if _, err := s.MutateMemory(t.Context(), "", request); err == nil {
		t.Fatal("unchecked proposal became verified memory")
	}
	checked, err := s.CheckChange(t.Context(), session.ID, ChangeCheckRequest{ChangeIdentity: identity})
	if err != nil {
		t.Fatal(err)
	}
	identity = changeIdentity(checked)
	request.Draft.Text = "User asserted checks passed"
	if _, err := s.MutateMemory(t.Context(), "", request); err == nil {
		t.Fatal("arbitrary verification claim accepted")
	}
	request.Draft.Text = ""
	record := retainMemoryFixture(t, s, request)
	if record.Provenance.Origin != "proposal_checks" || record.Provenance.ProposalHash != checked.Hash || !strings.Contains(record.Text, "isolated proposal checks") || !strings.Contains(record.Text, "Passed:") {
		t.Fatalf("missing actual receipt: %+v", record)
	}
	delete(s.changeAuthority, checked.ProjectID+"/"+checked.ID)
	if _, err := s.MutateMemory(t.Context(), "", request); err == nil {
		t.Fatal("lost check authority accepted")
	}
}

func TestMemoryCurationRejectsInvalidDraftsWithoutCatalog(t *testing.T) {
	for _, kind := range []string{"path", "span", "retention", "unknown", "identity"} {
		t.Run(kind, func(t *testing.T) {
			s, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
			request := memoryDraftFixture(t, s, "main.go")
			switch kind {
			case "path":
				request.Draft.Sources[0].Path = "../outside"
			case "span":
				request.Draft.Sources[0].EndLine = 100
			case "retention":
				request.Draft.RetentionDays = 366
			case "unknown":
				request.Action = "ingest"
			case "identity":
				request.ProjectRevision = "other"
			}
			if _, err := s.MutateMemory(t.Context(), "", request); err == nil {
				t.Fatal("invalid draft accepted")
			}
			store, err := memoryStore(root)
			if err != nil {
				t.Fatal(err)
			}
			records, err := store.Memories(t.Context(), request.ProjectID)
			if err != nil || len(records) != 0 {
				t.Fatalf("invalid request stored: %+v %v", records, err)
			}
		})
	}
}

func TestMemoryRetainedHarnessLessonPreservesExactProvenance(t *testing.T) {
	s, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	session := openChangeFixture(t, s, "main.go")
	session.Workflow = s.prepareWorkflowRun(session, ChangeWorkflowRequest{Models: ChangeWorkflowModels{Create: "function", Test: "function", Review: "analyze"}}, 0, nil)
	session.Workflow.Status = "failed"
	session.Workflow.HarnessLessons = []string{"Run formatting before rerunning the isolated checks."}
	if err := storeChangeSession(root, session); err != nil {
		t.Fatal(err)
	}
	request := memoryDraftFixture(t, s, "main.go")
	identity, lesson := changeIdentity(session), 0
	request.Draft.Kind, request.Draft.Text = "lesson", ""
	request.Draft.SessionID, request.Draft.Identity, request.Draft.LessonIndex = session.ID, &identity, &lesson
	record := retainMemoryFixture(t, s, request)
	if record.Text != session.Workflow.HarnessLessons[0] || record.Provenance.Origin != "harness_lesson" || record.Provenance.WorkflowID != session.Workflow.ID {
		t.Fatalf("lesson provenance lost: %+v", record)
	}
	lesson = 1
	if _, err := s.MutateMemory(t.Context(), "", request); err == nil {
		t.Fatal("out-of-range lesson accepted")
	}
}
