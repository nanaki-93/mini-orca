package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func approveExecutionMission(t *testing.T, service *Service) *AgentMission {
	t.Helper()
	saved, err := service.MutateMission(t.Context(), "", missionDraftFixture(t, service))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"helper", "caller", ""} {
		saved, err = service.MutateMission(t.Context(), saved.Mission.ID, missionReviewRequest(saved, id))
		if err != nil {
			t.Fatal(err)
		}
	}
	return saved
}

func prepareExecutionRequest(t *testing.T, service *Service, saved *AgentMission) MissionExecutionRequest {
	t.Helper()
	repo, err := service.GitRepository(t.Context(), saved.Mission.ProjectID, saved.Mission.ProjectRevision)
	if err != nil {
		t.Fatal(err)
	}
	return MissionExecutionRequest{ProjectID: saved.Mission.ProjectID, ProjectRevision: saved.Mission.ProjectRevision, Action: "prepare", CatalogRevision: saved.CatalogRevision, PlanHash: saved.Mission.PlanHash, Base: &agentstate.MissionBase{Head: repo.Head, Branch: repo.Branch}, Confirm: true}
}

func cancelExecutionRequest(record *AgentMissionExecution, query AgentRunQuery) MissionExecutionRequest {
	return MissionExecutionRequest{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Action: "cancel", PlanHash: record.Execution.Mission.PlanHash, ExecutionID: record.Execution.ID, ExecutionRevision: record.Execution.Revision, Confirm: true}
}

func TestMissionExecutionPreparationIsPassiveDurableAndDoesNotGrantExecution(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer server.Close()
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	initializeChangeGit(t, service, root)
	saved := approveExecutionMission(t, service)
	request := prepareExecutionRequest(t, service, saved)
	query := settingsQuery(t, service)
	if err := service.requireProjectExecutionTrust(t.Context(), query.ProjectRevision); err == nil {
		t.Fatal("fixture already trusted")
	}
	if _, err := service.MissionExecution(t.Context(), query, saved.Mission.ID); !errors.Is(err, ErrMissionNotFound) {
		t.Fatal(err)
	}
	before := testGit(t, root, "status", "--porcelain=v1")
	record, err := service.ControlMissionExecution(t.Context(), saved.Mission.ID, request)
	if err != nil || record.Stale || record.SourceStatus != "current" || record.Execution.State != "prepared" || record.Execution.Base != *request.Base {
		t.Fatal(record, err)
	}
	if !reflect.DeepEqual(record.Execution.Mission, saved.Mission) {
		t.Fatal("preparation altered the reviewed mission")
	}
	if _, err := service.ControlMissionExecution(t.Context(), saved.Mission.ID, request); !errors.Is(err, ErrMissionConflict) {
		t.Fatal("replay reset allocation", err)
	}
	if err := service.requireProjectExecutionTrust(t.Context(), query.ProjectRevision); err == nil {
		t.Fatal("preparation granted execution")
	}
	if calls.Load() != 0 || before != testGit(t, root, "status", "--porcelain=v1") || len(service.changeAuthority) != 0 {
		t.Fatal("passive preparation dispatched work")
	}
	if _, err := os.Stat(filepath.Join(root, ".worktrees")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("preparation created worktree", err)
	}
	if runs, err := service.ListAgentRuns(t.Context(), query); err != nil || len(runs.Runs) != 0 {
		t.Fatal("preparation created agent invocation", runs, err)
	}
	restarted, err := New(service.config, service.manager)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := restarted.MissionExecution(t.Context(), query, saved.Mission.ID)
	if err != nil || !reflect.DeepEqual(loaded, record) {
		t.Fatal("restart lost frozen execution", loaded, err)
	}
	if _, err := restarted.ControlMissionExecution(t.Context(), saved.Mission.ID, request); !errors.Is(err, ErrMissionConflict) {
		t.Fatal("restart reset allocation", err)
	}
}

func TestMissionExecutionRejectsStaleInputsAndRetainsChangedHistory(t *testing.T) {
	for _, change := range []string{"source", "instructions", "settings", "dirty", "head", "branch", "revision", "catalog plan"} {
		t.Run(change, func(t *testing.T) {
			service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
			initializeChangeGit(t, service, root)
			saved := approveExecutionMission(t, service)
			request := prepareExecutionRequest(t, service, saved)
			record, err := service.ControlMissionExecution(t.Context(), saved.Mission.ID, request)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "source", "revision":
				err = os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n// modified source\n"), 0600)
			case "instructions":
				err = os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("Changed instructions\n"), 0600)
			case "settings":
				control := settingsControl(t, service)
				control.Values.Limits.ModelCalls--
				_, err = service.ControlAgentSettings(t.Context(), control)
			case "dirty":
				err = os.WriteFile(filepath.Join(root, "unrelated.txt"), []byte("User work\n"), 0600)
			case "head":
				testGit(t, root, "commit", "--allow-empty", "-m", "Changed base")
			case "branch":
				testGit(t, root, "switch", "-c", "changed")
			case "catalog plan":
				update := missionDraftFixture(t, service)
				update.Action, update.CatalogRevision, update.PlanHash = "revise", saved.CatalogRevision, saved.Mission.PlanHash
				update.Plan.Goal = "A changed goal"
				_, err = service.MutateMission(t.Context(), saved.Mission.ID, update)
			}
			if err != nil {
				t.Fatal(err)
			}
			if change == "revision" {
				if _, err := service.Reindex(); err != nil {
					t.Fatal(err)
				}
			}
			query := settingsQuery(t, service)
			loaded, err := service.MissionExecution(t.Context(), query, saved.Mission.ID)
			if err != nil || !loaded.Stale || loaded.SourceStatus != "changed" || !reflect.DeepEqual(loaded.Execution, record.Execution) {
				t.Fatal("changed evidence lost its meaning", loaded, err)
			}
			if _, err := service.ControlMissionExecution(t.Context(), saved.Mission.ID, request); err == nil {
				t.Fatal("changed preparation admitted")
			}
			canceled, err := service.ControlMissionExecution(t.Context(), saved.Mission.ID, cancelExecutionRequest(record, query))
			if err != nil || canceled.Execution.State != "canceled" || canceled.Execution.Revision != 2 || !canceled.Stale || !reflect.DeepEqual(canceled.Execution.Children, record.Execution.Children) {
				t.Fatal("changed checkout blocked cancellation", canceled, err)
			}
		})
	}
}

func TestMissionExecutionPreparationRejectsUnapprovedDirtyAndChangedGitBeforePublication(t *testing.T) {
	for _, change := range []string{"approval", "head", "branch", "dirty", "source", "settings", "catalog", "no Git", "detached"} {
		t.Run(change, func(t *testing.T) {
			service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
			if change != "no Git" {
				initializeChangeGit(t, service, root)
			}
			saved := approveExecutionMission(t, service)
			request := prepareExecutionRequest(t, service, saved)
			switch change {
			case "approval":
				update := missionDraftFixture(t, service)
				update.Action, update.CatalogRevision, update.PlanHash = "revise", saved.CatalogRevision, saved.Mission.PlanHash
				var err error
				saved, err = service.MutateMission(t.Context(), saved.Mission.ID, update)
				if err != nil {
					t.Fatal(err)
				}
				request = prepareExecutionRequest(t, service, saved)
			case "head":
				request.Base.Head = strings.Repeat("a", 40)
			case "branch":
				request.Base.Branch = "not-current"
			case "dirty":
				if err := os.WriteFile(filepath.Join(root, "unrelated.txt"), []byte("uncommitted\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "source":
				if err := os.WriteFile(filepath.Join(root, "new.go"), []byte("package main\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "settings":
				control := settingsControl(t, service)
				control.Values.Limits.ModelCalls--
				if _, err := service.ControlAgentSettings(t.Context(), control); err != nil {
					t.Fatal(err)
				}
			case "catalog":
				request.CatalogRevision--
			case "detached":
				testGit(t, root, "switch", "--detach")
			}
			if _, err := service.ControlMissionExecution(t.Context(), saved.Mission.ID, request); err == nil {
				t.Fatal("invalid preparation accepted")
			}
			store, err := agentstate.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.MissionExecution(t.Context(), request.ProjectID, saved.Mission.ID); !errors.Is(err, agentstate.ErrNotFound) {
				t.Fatal("rejected preparation published record", err)
			}
		})
	}
}

func TestMissionExecutionControlRequiresCurrentScopeAndConcreteAction(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	initializeChangeGit(t, service, root)
	saved := approveExecutionMission(t, service)
	request := prepareExecutionRequest(t, service, saved)
	for name, mutate := range map[string]func(*MissionExecutionRequest){
		"confirmation":          func(r *MissionExecutionRequest) { r.Confirm = false },
		"scope":                 func(r *MissionExecutionRequest) { r.ProjectID = "foreign" },
		"revision":              func(r *MissionExecutionRequest) { r.ProjectRevision = "old" },
		"action":                func(r *MissionExecutionRequest) { r.Action = "start" },
		"base":                  func(r *MissionExecutionRequest) { r.Base = nil },
		"hash":                  func(r *MissionExecutionRequest) { r.PlanHash = "invalid" },
		"new identity":          func(r *MissionExecutionRequest) { r.ExecutionID = "forged" },
		"new version":           func(r *MissionExecutionRequest) { r.ExecutionRevision = 1 },
		"cancel lacks identity": func(r *MissionExecutionRequest) { r.Action = "cancel"; r.Base = nil; r.CatalogRevision = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			invalid := request
			mutate(&invalid)
			if _, err := service.ControlMissionExecution(t.Context(), saved.Mission.ID, invalid); err == nil {
				t.Fatal("invalid execution request accepted")
			}
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := service.ControlMissionExecution(ctx, saved.Mission.ID, request); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := service.MissionExecution(t.Context(), AgentRunQuery{ProjectID: "foreign", ProjectRevision: request.ProjectRevision}, saved.Mission.ID); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatal(err)
	}
}
