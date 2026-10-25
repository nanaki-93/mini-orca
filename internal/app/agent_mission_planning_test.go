package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func missionPlanningFixture(t *testing.T, s *Service) (MissionPlanningRequest, string) {
	t.Helper()
	draft := missionDraftFixture(t, s)
	request := MissionPlanningRequest{ProjectID: draft.ProjectID, ProjectRevision: draft.ProjectRevision, Goal: draft.Plan.Goal, Sources: draft.Plan.Sources, Limits: draft.Plan.Limits,
		PlanningLimits: agentstate.MissionLimits{ModelCalls: 4, ToolCalls: 8, InputTokens: 128 << 10, OutputTokens: 128 << 10, ElapsedSeconds: 60, ConcurrentCalls: 1}}
	data, err := json.Marshal(map[string]any{"version": agentruntime.MissionPlannerVersion, "status": "planned", "reason": "", "children": draft.Plan.Children, "integration_order": draft.Plan.IntegrationOrder})
	if err != nil {
		t.Fatal(err)
	}
	return request, string(data)
}

func writeMissionPlanningAnswer(w http.ResponseWriter, answer string) {
	_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{FinishReason: "stop", Message: llm.ChatMessage{Role: "assistant", Content: answer}}}, Usage: llm.ChatUsage{PromptTokens: 100, CompletionTokens: 40, TotalTokens: 140}})
}

func TestMissionPlanningUsesNativeToolsAndPublishesOnlyAnUnapprovedDraft(t *testing.T) {
	for _, google := range []bool{false, true} {
		t.Run(map[bool]string{false: "compatible", true: "Gemini"}[google], func(t *testing.T) {
			var calls atomic.Int32
			answer := ""
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				if !strings.Contains(string(body), "read_snippet") || !strings.Contains(string(body), "mission-planner-v1") || strings.Contains(string(body), "helper secret") {
					t.Error("planner lost scope or native tool contract")
				}
				turn := calls.Add(1)
				if google {
					writeGoogleMissionTurn(t, w, string(body), answer, turn)
				} else if turn == 1 {
					repositoryToolCall(w, "read_snippet", `{"path":"main.go","start_line":1,"end_line":1}`)
				} else {
					writeMissionPlanningAnswer(w, answer)
				}
			}))
			defer server.Close()
			s, root := newSemanticAnalysisServiceWithHelper(t, server.URL, 0)
			if google {
				profile := s.runtimes.function.profile
				profile.Provider, profile.APIKey = config.GeminiProvider, "fixture-key"
				s.runtimes.function.profile, s.runtimes.function.client = profile, agentruntime.NewProviderClient(profile)
			}
			request, response := missionPlanningFixture(t, s)
			answer = response
			before, _ := os.ReadFile(filepath.Join(root, "main.go"))
			result, err := s.PlanMission(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			if calls.Load() != 2 || result.Usage.ModelCalls != 2 || result.Usage.ToolCalls != 1 || result.PromptVersion != agentruntime.MissionPlannerVersion || len(result.InputHash) != 64 || result.Mission.Approved || len(result.Mission.Reviewed) != 0 {
				t.Fatalf("planning receipt: %+v calls=%d", result, calls.Load())
			}
			if result.Mission.Plan.Goal != request.Goal || result.Mission.Plan.Limits != request.Limits || len(result.Mission.Plan.Children) != 2 || result.ContextManifest.Scope != "code" {
				t.Fatal("constraints lost", result)
			}
			detail, err := s.MissionDetail(t.Context(), settingsQuery(t, s), result.Mission.ID)
			if err != nil || detail.Mission.PlanHash != result.Mission.PlanHash {
				t.Fatal("generated draft not durable", detail, err)
			}
			after, _ := os.ReadFile(filepath.Join(root, "main.go"))
			if string(before) != string(after) || len(s.changeAuthority) != 0 {
				t.Fatal("planner changed source or Apply authority")
			}
			if _, err := os.Stat(filepath.Join(root, "new.go")); !os.IsNotExist(err) {
				t.Fatal("planner created child source", err)
			}
			history, err := s.ListAgentRuns(t.Context(), settingsQuery(t, s))
			if err != nil || len(history.Runs) != 1 || history.Runs[0].Active {
				t.Fatal("native invocation history unavailable", history, err)
			}
		})
	}
}

func writeGoogleMissionTurn(t *testing.T, w http.ResponseWriter, body, answer string, turn int32) {
	t.Helper()
	var part any
	if turn == 1 {
		part = map[string]any{"functionCall": map[string]any{"id": "mission-read", "name": "read_snippet", "args": map[string]any{"path": "main.go", "start_line": 1, "end_line": 1}}, "thoughtSignature": "c2ln"}
	} else {
		if !strings.Contains(body, "functionResponse") || !strings.Contains(body, "c2ln") {
			t.Error("native tool round trip lost evidence or thought signature")
		}
		part = map[string]any{"text": answer}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"finishReason": "STOP", "content": map[string]any{"role": "model", "parts": []any{part}}}}, "usageMetadata": map[string]any{"promptTokenCount": 100, "candidatesTokenCount": 40, "totalTokenCount": 140}})
}

func TestMissionPlanningRejectsLateBindingAndCatalogChanges(t *testing.T) {
	for _, change := range []string{"source", "absent created", "instructions", "settings", "catalog"} {
		t.Run(change, func(t *testing.T) {
			var mutate func()
			answer := ""
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { mutate(); writeMissionPlanningAnswer(w, answer) }))
			defer server.Close()
			s, root := newSemanticAnalysisService(t, server.URL, 0)
			existing, err := s.MutateMission(t.Context(), "", missionDraftFixture(t, s))
			if err != nil {
				t.Fatal(err)
			}
			request, response := missionPlanningFixture(t, s)
			request.CatalogRevision, answer = existing.CatalogRevision, response
			control := settingsControl(t, s)
			mutate = func() {
				var err error
				switch change {
				case "source":
					err = os.WriteFile(filepath.Join(root, "main.go"), []byte("package changed\n"), 0600)
				case "absent created":
					err = os.WriteFile(filepath.Join(root, "new.go"), []byte("package main\n"), 0600)
				case "instructions":
					err = os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("New instructions\n"), 0600)
				case "settings":
					control.Values.Limits.ModelCalls--
					_, err = s.ControlAgentSettings(t.Context(), control)
				case "catalog":
					_, err = s.MutateMission(t.Context(), existing.Mission.ID, missionReviewRequest(existing, "helper"))
				}
				if err != nil {
					t.Error(err)
				}
			}
			result, err := s.PlanMission(t.Context(), request)
			if result != nil || err == nil || !errors.Is(err, project.ErrRevisionConflict) && !errors.Is(err, ErrMissionConflict) {
				t.Fatal("late plan published", result, err)
			}
			list, err := s.ListMissions(t.Context(), settingsQuery(t, s))
			if err != nil || len(list.Missions) != 1 || list.Missions[0].ID != existing.Mission.ID {
				t.Fatal("existing draft replaced", list, err)
			}
		})
	}
}

func TestMissionPlanningCancellationPreservesExistingDraft(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	s, _ := newSemanticAnalysisService(t, server.URL, 0)
	existing, err := s.MutateMission(t.Context(), "", missionDraftFixture(t, s))
	if err != nil {
		t.Fatal(err)
	}
	request, _ := missionPlanningFixture(t, s)
	request.CatalogRevision = existing.CatalogRevision
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := s.PlanMission(ctx, request); done <- err }()
	select {
	case <-started:
	case err := <-done:
		t.Fatalf("planner ended before dispatch: %v", err)
	case <-ctx.Done():
		t.Fatal("planner did not dispatch before the test deadline")
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	detail, err := s.MissionDetail(t.Context(), settingsQuery(t, s), existing.Mission.ID)
	if err != nil || detail.CatalogRevision != existing.CatalogRevision || detail.Mission.PlanHash != existing.Mission.PlanHash {
		t.Fatal("canceled planner changed existing draft", detail, err)
	}
}

func TestMissionPlanningAdmissionBudgetAndInvalidOutput(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		writeMissionPlanningAnswer(w, `{"version":"mission-planner-v1","status":"unavailable","reason":"Insufficient context","children":[],"integration_order":[]}`)
	}))
	defer server.Close()
	s, _ := newSemanticAnalysisService(t, server.URL, 0)
	request, _ := missionPlanningFixture(t, s)
	s.runtimes.function.effective.RemoteProvider = true
	if _, err := s.PlanMission(t.Context(), request); err == nil || calls.Load() != 0 {
		t.Fatal("remote consent bypassed", err)
	}
	s.runtimes.function.effective.RemoteProvider = false
	for _, mutate := range []func(*MissionPlanningRequest){
		func(r *MissionPlanningRequest) { r.ProjectRevision = "stale" },
		func(r *MissionPlanningRequest) { r.CatalogRevision = 1 },
		func(r *MissionPlanningRequest) { r.PlanningLimits.ModelCalls = 9 },
		func(r *MissionPlanningRequest) { r.PlanningLimits.ConcurrentCalls = 2 },
		func(r *MissionPlanningRequest) { r.PlanningLimits.Repairs = 1 },
		func(r *MissionPlanningRequest) { r.Sources = append(r.Sources, r.Sources[0]) },
		func(r *MissionPlanningRequest) { r.Goal = "bad\x1bgoal" },
	} {
		copy := request
		mutate(&copy)
		if _, err := s.PlanMission(t.Context(), copy); err == nil || calls.Load() != 0 {
			t.Fatal("invalid admission dispatched", err)
		}
	}
	copy := request
	copy.PlanningLimits.InputTokens = 1
	if _, err := s.PlanMission(t.Context(), copy); !errors.Is(err, agentruntime.ErrBudgetExceeded) || calls.Load() != 0 {
		t.Fatal("planning request exceeded its budget", err, calls.Load())
	}
	if result, err := s.PlanMission(t.Context(), request); !errors.Is(err, ErrMissionPlanningIncomplete) || result != nil || calls.Load() != 1 {
		t.Fatal("incomplete output became plan", result, err, calls.Load())
	}
	list, err := s.ListMissions(t.Context(), settingsQuery(t, s))
	if err != nil || list.CatalogRevision != 0 || len(list.Missions) != 0 {
		t.Fatal("failed planner created mission", list, err)
	}
}

func TestMissionPlanningFreezesInventoryAndBoundsCompletionOnlyContext(t *testing.T) {
	s, _ := newSemanticAnalysisServiceWithHelper(t, "http://127.0.0.1:1", 0)
	request, _ := missionPlanningFixture(t, s)
	p, err := s.prepareMissionPlanning(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.Sources[0].Path = "changed.go"
	if p.request.Sources[0].Path != "new.go" {
		t.Fatal("caller changed captured inventory")
	}
	runtime := s.runtimes.function
	runtime.profile.Provider = config.PiProvider
	input, err := s.missionPlanningInput(t.Context(), p, runtime, func(ctx context.Context) error { return s.validateMissionPlanning(ctx, p) })
	if err != nil || input.Investigation != nil || !strings.Contains(input.Messages[0].Content, "package main") || strings.Contains(input.Messages[0].Content, "helper secret") {
		t.Fatal("completion fallback lost bounded source", input, err)
	}
	runtime.effective.ContextMaxTokens = 1
	if _, err := s.missionPlanningInput(t.Context(), p, runtime, func(ctx context.Context) error { return s.validateMissionPlanning(ctx, p) }); err == nil {
		t.Fatal("profile context ceiling ignored")
	}
}

func TestMissionPlanningSharesBudgetAcrossRetriesAndToolTurns(t *testing.T) {
	for _, mode := range []string{"retry exhausted", "retry succeeds", "tool exhausted"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			answer := ""
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				turn := calls.Add(1)
				if mode == "tool exhausted" {
					fmt.Fprintf(w, `{"choices":[{"index":0,"finish_reason":"tool_calls","message":{"role":"assistant","tool_calls":[{"id":"read-%d","type":"function","function":{"name":"read_snippet","arguments":"{\"path\":\"main.go\",\"start_line\":1,\"end_line\":1}"}}]}}]}`, turn)
				} else if turn == 1 {
					http.Error(w, "temporary failure", 500)
				} else {
					writeMissionPlanningAnswer(w, answer)
				}
			}))
			defer server.Close()
			s, _ := newSemanticAnalysisService(t, server.URL, 0)
			s.retryBase = time.Millisecond
			request, response := missionPlanningFixture(t, s)
			answer = response
			if mode == "retry exhausted" {
				request.PlanningLimits.ModelCalls = 1
			}
			if mode == "tool exhausted" {
				request.PlanningLimits.ToolCalls = 1
			}
			result, err := s.PlanMission(t.Context(), request)
			if mode == "retry succeeds" {
				if err != nil || result.Usage.ModelCalls != 2 || result.Usage.UnknownUsageCalls != 1 || calls.Load() != 2 {
					t.Fatal("retry accounting lost", result, err, calls.Load())
				}
				return
			}
			want := int32(1)
			if mode == "tool exhausted" {
				want = 2
			}
			if result != nil || !errors.Is(err, agentruntime.ErrBudgetExceeded) || calls.Load() != want {
				t.Fatal("budget replenished", result, err, calls.Load())
			}
			list, err := s.ListMissions(t.Context(), settingsQuery(t, s))
			if err != nil || len(list.Missions) != 0 {
				t.Fatal("exhausted planning produced a mission", list, err)
			}
		})
	}
}

func TestMissionPlanningCompletionOnlyProfileRunsThroughBoundedAdapter(t *testing.T) {
	s, _ := newSemanticAnalysisServiceWithHelper(t, "http://127.0.0.1:1", 0)
	request, answer := missionPlanningFixture(t, s)
	directory := t.TempDir()
	capture := filepath.Join(directory, "captured-input.json")
	message, err := json.Marshal(map[string]any{"type": "message_end", "message": map[string]any{"role": "assistant", "model": "fixture-model", "stopReason": "stop", "content": []any{map[string]string{"type": "text", "text": answer}}, "usage": map[string]int{"input": 10, "output": 20, "totalTokens": 30}}})
	if err != nil {
		t.Fatal(err)
	}
	// This fixed fake provider records input and emits fixture data; it never
	// interprets the prompt or launches project code.
	quotedCapture := "'" + strings.ReplaceAll(capture, "'", "'\"'\"'") + "'"
	script := "#!/bin/sh\numask 077\ncat > " + quotedCapture + "\ncat <<'MISSION_FIXTURE_END'\n{\"type\":\"agent_start\"}\n" + string(message) + "\n{\"type\":\"agent_end\",\"willRetry\":false}\nMISSION_FIXTURE_END\n"
	executable := filepath.Join(directory, "fake-pi")
	if err := os.WriteFile(executable, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	profile := config.ModelProfile{Provider: config.PiProvider, CLIPath: executable, Model: "fixture-model"}
	s.runtimes.function.profile, s.runtimes.function.client = profile, agentruntime.NewProviderClient(profile)
	s.runtimes.function.effective.RemoteProvider = true
	request.ConfirmRemoteProvider = true
	request.PlanningLimits.OutputTokens = 4 << 20 // CLI responses need this conservative reservation.
	result, err := s.PlanMission(t.Context(), request)
	if err != nil || result.Usage.ModelCalls != 1 || result.Usage.ToolCalls != 0 || result.Mission.Approved {
		t.Fatal(result, err)
	}
	data, err := os.ReadFile(capture)
	if err != nil || !strings.Contains(string(data), "package main") || strings.Contains(string(data), "helper secret") {
		t.Fatal("CLI context was not the admitted capture", err)
	}
}
