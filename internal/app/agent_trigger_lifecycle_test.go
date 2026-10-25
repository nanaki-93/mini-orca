package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func triggerBlockingProvider(t *testing.T) (string, <-chan struct{}, <-chan struct{}, func()) {
	t.Helper()
	started, ended, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			return
		}
		close(started)
		defer close(ended)
		select {
		case <-r.Context().Done():
			return
		case <-release:
			_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Role: "assistant", Content: emptyAnalysisReply(AnalysisStageSemantic)}, FinishReason: "stop"}}})
		}
	}))
	t.Cleanup(func() { unblock(); server.Close() })
	return server.URL, started, ended, unblock
}

func awaitTriggerSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(10 * time.Second):
		t.Fatal("ambient owner did not finish its bounded lifecycle")
	}
}

func startBlockedTrigger(t *testing.T, s *Service, root string, ticks chan time.Time, started <-chan struct{}) *triggerOwner {
	t.Helper()
	triggerControl(t, s, "watch", &agentstate.TriggerOptions{Mode: "changes", Paths: []string{"main.go"}})
	triggerControl(t, s, "grant", nil)
	changeTriggerFile(t, s, root, "Changed")
	owner := triggerOwnerForTest(s)
	select {
	case ticks <- time.Now().UTC():
	case <-time.After(10 * time.Second):
		t.Fatal("owner did not accept tick")
	}
	awaitTriggerSignal(t, started)
	return owner
}

func TestAgentTriggersLifecycleCancelsAndJoinsActiveProvider(t *testing.T) {
	for _, action := range []string{"disable", "revoke", "reindex", "settings", "replace", "shutdown"} {
		t.Run(action, func(t *testing.T) {
			endpoint, started, ended, _ := triggerBlockingProvider(t)
			s, root, ticks := triggerService(t, endpoint)
			owner := startBlockedTrigger(t, s, root, ticks, started)
			before, err := owner.store.Triggers(t.Context(), owner.projectID)
			if err != nil || before.LastAttempt == nil || before.LastAttempt.State != "running" || before.Window.Charged.Runs != 1 {
				t.Fatal("provider preceded durable reservation", err)
			}
			switch action {
			case "disable", "revoke":
				triggerControl(t, s, action, nil)
			case "reindex":
				_, err = s.Reindex()
			case "settings":
				_, err = s.ControlAgentSettings(t.Context(), settingsControl(t, s))
			case "replace":
				next := t.TempDir()
				err = s.ActivateProject(next, &project.Analysis{Name: "replacement", Path: next})
			case "shutdown":
				err = s.ShutdownChangeWorkflows(t.Context())
			}
			if err != nil {
				t.Fatal(err)
			}
			awaitTriggerSignal(t, owner.done)
			awaitTriggerSignal(t, ended)
			after, err := owner.store.Triggers(t.Context(), owner.projectID)
			if err != nil || after.LastAttempt.State != "canceled" || after.Window.Charged != before.Window.Charged {
				t.Fatalf("uncertain cancellation lost receipt/charge: %+v %v", after, err)
			}
			if action == "replace" {
				if err = s.ActivateProject(root, &project.Analysis{Name: "fixture", Path: root}); err != nil {
					t.Fatal(err)
				}
				if triggerView(t, s).Owned {
					t.Fatal("project return silently restored permission")
				}
			}
		})
	}
}

func TestAgentTriggersDeferToLifecycleAndActiveProjectWork(t *testing.T) {
	server, calls := analysisResponseServer(t, emptyAnalysisReply)
	s, root, _ := triggerService(t, server.URL)
	triggerControl(t, s, "watch", &agentstate.TriggerOptions{Mode: "changes", Paths: []string{"main.go"}})
	triggerControl(t, s, "grant", nil)
	changeTriggerFile(t, s, root, "Changed")
	owner := triggerOwnerForTest(s)
	s.jobLifecycleMu.Lock()
	err := s.triggerTick(owner, time.Now().UTC())
	s.jobLifecycleMu.Unlock()
	if err != nil || triggerView(t, s).Record.State != "waiting" || calls.Load() != 0 {
		t.Fatal("ambient admission competed with lifecycle", err)
	}
	_, release, err := s.integrations.begin(t.Context(), "foreground")
	if err != nil {
		t.Fatal(err)
	}
	err = s.triggerTick(owner, time.Now().UTC())
	release()
	if err != nil || triggerView(t, s).Record.State != "waiting" || calls.Load() != 0 {
		t.Fatal("ambient admission competed with active integration", err)
	}
	if err = s.triggerTick(owner, time.Now().UTC()); err != nil || calls.Load() != 1 {
		t.Fatal("idle project did not resume eligible work", err)
	}
}

func TestAgentTriggersRunDeadlineCancelsProviderAndRetainsReservation(t *testing.T) {
	endpoint, started, ended, _ := triggerBlockingProvider(t)
	s, root, ticks := triggerService(t, endpoint)
	s.agentRuntime.Triggers.RunSeconds = 1
	owner := startBlockedTrigger(t, s, root, ticks, started)
	awaitTriggerSignal(t, ended)
	// Joining waits for the final persisted receipt as well as the HTTP call.
	if err := s.stopTriggers(t.Context()); err != nil {
		t.Fatal(err)
	}
	r, err := owner.store.Triggers(t.Context(), owner.projectID)
	if err != nil || r.LastAttempt.State != "failed" || r.Window.Charged.Runs != 1 || r.Window.Charged.Seconds != 1 {
		t.Fatalf("deadline lost accounting: %+v %v", r, err)
	}
}

func TestAgentTriggersPersistenceFailureStopsOwnerWithUncertainReceipt(t *testing.T) {
	endpoint, started, _, release := triggerBlockingProvider(t)
	s, root, ticks := triggerService(t, endpoint)
	owner := startBlockedTrigger(t, s, root, ticks, started)
	paths, err := filepath.Glob(filepath.Join(root, ".mini-orca", "agents", "*", "triggers.json"))
	if err != nil || len(paths) != 1 {
		t.Fatal("missing private trigger record", paths, err)
	}
	path := paths[0]
	if err = os.Rename(path, path+".saved"); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := os.Stat(path + ".saved"); err == nil {
			_ = os.Remove(path)
			_ = os.Rename(path+".saved", path)
		}
	})
	release()
	awaitTriggerSignal(t, owner.done)
	owned, fault := s.triggerOwnerState(owner.root, owner.controlRevision)
	if owned || !fault {
		t.Fatal("failed receipt persistence kept automatic owner alive")
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(path+".saved", path); err != nil {
		t.Fatal(err)
	}
	r, err := owner.store.Triggers(t.Context(), owner.projectID)
	if err != nil || r.LastAttempt.State != "running" || r.Window.Charged.Runs != 1 {
		t.Fatal("uncertain delivery lost reservation", err)
	}
	v := triggerView(t, s)
	if v.State != "failed" || !v.Fault {
		t.Fatal("uncertain persistence appeared successful")
	}
	// An explicit restart settles the interrupted receipt, keeping its charge.
	triggerControl(t, s, "watch", &agentstate.TriggerOptions{Mode: "changes", Paths: []string{"main.go"}})
	v = triggerView(t, s)
	if v.Record.LastAttempt.State != "interrupted" || v.Record.Window.Charged != r.Window.Charged || v.Record.Grant != nil {
		t.Fatal("restart replayed an uncertain grant")
	}
}

func TestAgentTriggersActualSemanticSelectionAndRemoteConsent(t *testing.T) {
	var calls atomic.Int32
	models := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request llm.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		calls.Add(1)
		models <- request.Model
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Role: "assistant", Content: emptyAnalysisReply(AnalysisStageSemantic)}, FinishReason: "stop"}}})
	}))
	t.Cleanup(server.Close)
	s, root, _ := triggerService(t, server.URL)
	s.runtimes.function.effective.RemoteProvider = true
	identity := triggerView(t, s).ProjectID
	s.analysisRun.run = &AnalysisRun{Identity: AnalysisRunIdentity{ProjectID: identity}, Status: AnalysisRunCompleted,
		Plan: AnalysisRunPreview{Models: &AnalysisModels{Code: "code"}}}
	triggerControl(t, s, "watch", &agentstate.TriggerOptions{Mode: "changes", Paths: []string{"main.go"}})
	v := triggerView(t, s)
	if v.Model != s.runtimes.function.effective.Model || v.Destination.Profile != "code" {
		t.Fatal("explicit semantic selection was ignored")
	}
	request := TriggerControl{TriggerBinding: v.TriggerBinding, ControlRevision: v.Record.ControlRevision, Action: "grant", Confirm: true, ConfirmSourceSharing: true, ScopeHash: v.ScopeHash, GrantSeconds: 60}
	if _, err := s.ControlAgentTriggers(t.Context(), request); err == nil {
		t.Fatal("missing remote-provider consent accepted")
	}
	request.ConfirmRemoteProvider = true
	if _, err := s.ControlAgentTriggers(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	changeTriggerFile(t, s, root, "Changed")
	s.analysisRun.mu.Lock()
	s.analysisRun.run.Plan.Models.Code = "bug"
	s.analysisRun.mu.Unlock()
	if err := s.triggerTick(triggerOwnerForTest(s), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 || triggerView(t, s).State != "stale_grant" {
		t.Fatal("changed model inherited old permission")
	}
	s.analysisRun.mu.Lock()
	s.analysisRun.run.Plan.Models.Code = "code"
	s.analysisRun.mu.Unlock()
	triggerControl(t, s, "grant", nil)
	changeTriggerFile(t, s, root, "ChangedAgain")
	if err := s.triggerTick(triggerOwnerForTest(s), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 || <-models != v.Model {
		t.Fatal("actual provider model differs from grant")
	}
}
