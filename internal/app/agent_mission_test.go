package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func missionDraftFixture(t *testing.T, s *Service) MissionMutation {
	t.Helper()
	query := settingsQuery(t, s)
	file, err := s.manager.IndexedFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	childLimit := agentstate.MissionLimits{ModelCalls: 2, ToolCalls: 4, InputTokens: 2048, OutputTokens: 1024, ElapsedSeconds: 60, ConcurrentCalls: 1}
	budget := childLimit
	budget.ModelCalls, budget.ToolCalls, budget.InputTokens, budget.OutputTokens = 4, 8, 4096, 2048
	return MissionMutation{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Action: "create", Plan: &agentstate.MissionPlan{Goal: "Create and use a helper", Limits: budget,
		Sources: []agentstate.MissionSource{{Path: "new.go"}, {Path: "main.go", Exists: true, Hash: file.ContentHash}},
		Children: []agentstate.MissionChild{
			{ID: "helper", Title: "Helper", Approach: "Add a pure helper", WritePaths: []string{"new.go"}, ReadPaths: []string{}, Dependencies: []string{}, AcceptanceCriteria: []string{"Helper returns one"}, Limits: childLimit},
			{ID: "caller", Title: "Caller", Approach: "Call the helper", WritePaths: []string{"main.go"}, ReadPaths: []string{"new.go"}, Dependencies: []string{"helper"}, AcceptanceCriteria: []string{"Caller uses helper"}, Limits: childLimit},
		}, IntegrationOrder: []string{"helper", "caller"}}}
}

func missionReviewRequest(value *AgentMission, child string) MissionMutation {
	r := MissionMutation{ProjectID: value.Mission.ProjectID, ProjectRevision: value.Mission.ProjectRevision, CatalogRevision: value.CatalogRevision, PlanHash: value.Mission.PlanHash, Action: "review", ChildID: child, Confirm: true}
	if child == "" {
		r.Action = "approve"
	}
	return r
}

func TestMissionLifecycleIsPassiveScopedAndDoesNotAuthorizeExecution(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer server.Close()
	s, root := newSemanticAnalysisService(t, server.URL, 0)
	query := settingsQuery(t, s)
	list, err := s.ListMissions(t.Context(), query)
	if err != nil || list.CatalogRevision != 0 || len(list.Missions) != 0 {
		t.Fatal(list, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".mini-orca", "agents")); !os.IsNotExist(err) {
		t.Fatal("passive list created storage", err)
	}
	before, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	request := missionDraftFixture(t, s)
	saved, err := s.MutateMission(t.Context(), "", request)
	if err != nil || saved.Stale || saved.SourceStatus != "current" {
		t.Fatal(saved, err)
	}
	list, err = s.ListMissions(t.Context(), query)
	if err != nil || len(list.Missions) != 1 || list.Missions[0].SourceStatus != "unchecked" {
		t.Fatal(list, err)
	}
	detail, err := s.MissionDetail(t.Context(), query, saved.Mission.ID)
	if err != nil || detail.Mission.PlanHash != saved.Mission.PlanHash || len(detail.Mission.Instructions) != 2 {
		t.Fatal(detail, err)
	}
	if _, err := s.MutateMission(t.Context(), saved.Mission.ID, missionReviewRequest(saved, "")); err == nil {
		t.Fatal("premature approval")
	}
	for _, child := range []string{"helper", "caller", ""} {
		saved, err = s.MutateMission(t.Context(), saved.Mission.ID, missionReviewRequest(saved, child))
		if err != nil {
			t.Fatal(err)
		}
	}
	if !saved.Mission.Approved || len(saved.Mission.Reviewed) != 2 {
		t.Fatal("review lost", saved)
	}
	request.Action, request.CatalogRevision, request.PlanHash = "revise", saved.CatalogRevision, saved.Mission.PlanHash
	request.Plan.Goal = "Revise helper behavior"
	revised, err := s.MutateMission(t.Context(), saved.Mission.ID, request)
	if err != nil || revised.Mission.Approved || len(revised.Mission.Reviewed) != 0 || revised.Mission.PlanHash == saved.Mission.PlanHash {
		t.Fatal(revised, err)
	}
	if _, err := s.MutateMission(t.Context(), saved.Mission.ID, missionReviewRequest(saved, "helper")); !errors.Is(err, ErrMissionConflict) {
		t.Fatal("stale review admitted", err)
	}
	history, err := s.ListAgentRuns(t.Context(), query)
	if err != nil || len(history.Runs) != 0 {
		t.Fatal("planning created agent runs", history, err)
	}
	after, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil || string(after) != string(before) || calls.Load() != 0 || len(s.changeAuthority) != 0 {
		t.Fatal("planning changed execution authority or source", err, calls.Load())
	}
	if _, err := os.Stat(filepath.Join(root, "new.go")); !os.IsNotExist(err) {
		t.Fatal("plan created source", err)
	}
}

func TestMissionRejectsChangedBindingsButRetainsHistoricalDetail(t *testing.T) {
	for _, change := range []string{"source", "deleted", "absent created", "instructions", "policy", "settings", "revision", "symlink"} {
		t.Run(change, func(t *testing.T) {
			s, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
			saved, err := s.MutateMission(t.Context(), "", missionDraftFixture(t, s))
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "source", "revision":
				err = os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc Changed() {}\n"), 0600)
			case "deleted":
				err = os.Remove(filepath.Join(root, "main.go"))
			case "absent created":
				err = os.WriteFile(filepath.Join(root, "new.go"), []byte("package main\n"), 0600)
			case "instructions":
				err = os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("Use a new approach.\n"), 0600)
			case "policy":
				err = os.WriteFile(filepath.Join(root, ".mini-orca", "context-policy.json"), []byte(`{"exclude":["main.go"]}`), 0600)
			case "settings":
				control := settingsControl(t, s)
				control.Values.Limits.ModelCalls--
				_, err = s.ControlAgentSettings(t.Context(), control)
			case "symlink":
				if err := os.Rename(filepath.Join(root, "main.go"), filepath.Join(root, "original.go")); err != nil {
					t.Fatal(err)
				}
				err = os.Symlink("original.go", filepath.Join(root, "main.go"))
			}
			if err != nil {
				t.Fatal(err)
			}
			if change == "revision" {
				if _, err := s.Reindex(); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := s.MutateMission(t.Context(), saved.Mission.ID, missionReviewRequest(saved, "helper")); err == nil {
				t.Fatal("changed binding accepted")
			}
			detail, err := s.MissionDetail(t.Context(), settingsQuery(t, s), saved.Mission.ID)
			if err != nil || !detail.Stale || detail.SourceStatus == "current" || detail.Mission.PlanHash != saved.Mission.PlanHash || len(detail.Mission.Reviewed) != 0 {
				t.Fatalf("historical plan lost or made current: %+v %v", detail, err)
			}
		})
	}
}

func TestMissionRejectsInvalidDraftsAndActionsBeforePublication(t *testing.T) {
	for name, alter := range map[string]func(*MissionMutation){
		"scope":          func(r *MissionMutation) { r.ProjectID = "foreign" },
		"revision":       func(r *MissionMutation) { r.ProjectRevision = "old" },
		"catalog":        func(r *MissionMutation) { r.CatalogRevision = 1 },
		"action":         func(r *MissionMutation) { r.Action = "execute" },
		"hash on create": func(r *MissionMutation) { r.PlanHash = "old" },
		"missing plan":   func(r *MissionMutation) { r.Plan = nil },
		"path":           func(r *MissionMutation) { r.Plan.Sources[0].Path = "../outside.go" },
		"excluded": func(r *MissionMutation) {
			r.Plan.Sources[0].Path = ".env"
			r.Plan.Children[0].WritePaths[0] = ".env"
			r.Plan.Children[1].ReadPaths[0] = ".env"
		},
		"wrong source hash": func(r *MissionMutation) { r.Plan.Sources[1].Hash = "sha256:" + strings.Repeat("b", 64) },
		"no dependencies":   func(r *MissionMutation) { r.Plan.Children[1].Dependencies = nil },
		"child on create":   func(r *MissionMutation) { r.ChildID = "helper" },
	} {
		t.Run(name, func(t *testing.T) {
			s, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
			r := missionDraftFixture(t, s)
			alter(&r)
			if _, err := s.MutateMission(t.Context(), "", r); err == nil {
				t.Fatal("invalid request published")
			}
			matches, err := filepath.Glob(filepath.Join(root, ".mini-orca", "agents", "*", "missions.json"))
			if err != nil || len(matches) != 0 {
				t.Fatal("invalid request persisted", err)
			}
		})
	}
}

func TestMissionScopeCancellationCorruptionAndConfiguredCeilings(t *testing.T) {
	s, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	request := missionDraftFixture(t, s)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := s.MutateMission(ctx, "", request); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	ceiling := s.agentRuntime.Limits.ModelCalls
	s.agentRuntime.Limits.ModelCalls = 3
	if _, err := s.MutateMission(t.Context(), "", request); err == nil {
		t.Fatal("configured ceiling ignored")
	}
	s.agentRuntime.Limits.ModelCalls = ceiling
	saved, err := s.MutateMission(t.Context(), "", request)
	if err != nil {
		t.Fatal(err)
	}
	control := missionReviewRequest(saved, "helper")
	control.Confirm = false
	if _, err := s.MutateMission(t.Context(), saved.Mission.ID, control); err == nil {
		t.Fatal("unconfirmed review")
	}
	query := settingsQuery(t, s)
	if _, err := s.MissionDetail(t.Context(), query, "missing"); !errors.Is(err, ErrMissionNotFound) {
		t.Fatal(err)
	}
	query.ProjectID = "foreign"
	if _, err := s.ListMissions(t.Context(), query); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join(root, ".mini-orca", "agents", "*", "missions.json"))
	if err != nil || len(paths) != 1 {
		t.Fatal(paths, err)
	}
	if err := os.WriteFile(paths[0], []byte("{corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ListMissions(t.Context(), settingsQuery(t, s)); !errors.Is(err, ErrMissionUnavailable) {
		t.Fatal(err)
	}
	if _, err := s.MutateMission(t.Context(), saved.Mission.ID, missionReviewRequest(saved, "helper")); !errors.Is(err, ErrMissionUnavailable) {
		t.Fatal(err)
	}
}
