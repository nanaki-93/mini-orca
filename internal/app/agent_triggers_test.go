package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

func triggerService(t *testing.T, endpoint string) (*Service, string, chan time.Time) {
	t.Helper()
	s, root := newSemanticAnalysisService(t, endpoint, 0)
	ticks := make(chan time.Time)
	s.triggers.ticks = ticks
	s.agentRuntime.Triggers.MaxRuns = 1
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.ShutdownChangeWorkflows(ctx); err != nil {
			t.Error(err)
		}
	})
	return s, root, ticks
}

func triggerView(t *testing.T, s *Service) *AgentTriggers {
	t.Helper()
	index, err := s.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	view, err := s.AgentTriggers(t.Context(), AgentRunQuery{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision})
	if err != nil {
		t.Fatal(err)
	}
	return view
}

func triggerControl(t *testing.T, s *Service, action string, options *agentstate.TriggerOptions) *AgentTriggers {
	t.Helper()
	v := triggerView(t, s)
	next, err := s.ControlAgentTriggers(t.Context(), TriggerControl{TriggerBinding: v.TriggerBinding, ControlRevision: v.Record.ControlRevision, Action: action, Confirm: true, ConfirmSourceSharing: action == "grant", ConfirmRemoteProvider: true, ScopeHash: v.ScopeHash, GrantSeconds: 3600, Options: options})
	if err != nil {
		t.Fatalf("%s: %v", action, err)
	}
	return next
}

func triggerOwnerForTest(s *Service) *triggerOwner {
	s.triggers.mu.Lock()
	defer s.triggers.mu.Unlock()
	return s.triggers.owner
}

func ageTriggerObservation(t *testing.T, owner *triggerOwner) {
	t.Helper()
	r, err := owner.store.Triggers(t.Context(), owner.projectID)
	if err != nil {
		t.Fatal(err)
	}
	r.ChangedAt, r.NextEligibleAt = time.Now().Add(-time.Minute), time.Now().Add(-time.Minute)
	if _, err := owner.store.SaveTriggers(t.Context(), r, r.Version); err != nil {
		t.Fatal(err)
	}
}

func changeTriggerFile(t *testing.T, s *Service, root, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc "+name+"() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	owner := triggerOwnerForTest(s)
	if err := s.triggerTick(owner, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	ageTriggerObservation(t, owner)
}

func TestAgentTriggersSeparateWatchGrantAndNativeAnalysisWithPersistentBudget(t *testing.T) {
	server, calls := analysisResponseServer(t, emptyAnalysisReply)
	s, root, _ := triggerService(t, server.URL)
	v := triggerView(t, s)
	if v.State != "disabled" || v.Owned || calls.Load() != 0 {
		t.Fatal("passive metadata started work")
	}
	v = triggerControl(t, s, "watch", &agentstate.TriggerOptions{Mode: "changes", Paths: []string{"main.go"}})
	if v.State != "watch_only" || !v.Owned || v.Record.Grant != nil {
		t.Fatal("watch implied provider permission")
	}
	changeTriggerFile(t, s, root, "First")
	if err := s.triggerTick(triggerOwnerForTest(s), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if triggerView(t, s).Record.State != "suggested" || calls.Load() != 0 {
		t.Fatal("ungranted detection dispatched")
	}
	v = triggerControl(t, s, "grant", nil)
	if v.State != "granted" || calls.Load() != 0 {
		t.Fatal("grant immediately replayed a missed change")
	}
	changeTriggerFile(t, s, root, "Second")
	owner := triggerOwnerForTest(s)
	if err := s.triggerTick(owner, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	v = triggerView(t, s)
	if calls.Load() != 1 || v.Record.State != "completed" || v.Record.LastAttempt.Usage.ModelCalls != 1 || v.Record.Window.Charged.Runs != 1 {
		t.Fatalf("native ambient attempt: calls=%d view=%+v", calls.Load(), v.Record)
	}
	assertKnowledgeHistory(t, s, root, "semantic", v.ProjectID, v.ProjectRevision)
	charged := v.Record.Window.Charged
	ageTriggerObservation(t, owner)
	if err := s.triggerTick(owner, time.Now().UTC()); err != nil || calls.Load() != 1 {
		t.Fatal("unchanged source replayed", err)
	}
	triggerControl(t, s, "revoke", nil)
	triggerControl(t, s, "grant", nil)
	changeTriggerFile(t, s, root, "Third")
	if err := s.triggerTick(triggerOwnerForTest(s), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	v = triggerView(t, s)
	if v.Record.State != "budget_exhausted" || v.Record.Window.Charged != charged || calls.Load() != 1 {
		t.Fatal("new grant replenished allowance", v.Record.State, calls.Load())
	}
	content, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil || string(content) != "package main\nfunc Third() {}\n" {
		t.Fatal("ambient analysis changed source", err)
	}
}

func TestAgentTriggersRejectStaleControlsUnsafePathsAndUnconfirmedSharing(t *testing.T) {
	server, calls := analysisResponseServer(t, emptyAnalysisReply)
	s, root, _ := triggerService(t, server.URL)
	triggerControl(t, s, "watch", &agentstate.TriggerOptions{Mode: "changes", Paths: []string{"main.go"}})
	v := triggerView(t, s)
	base := TriggerControl{TriggerBinding: v.TriggerBinding, ControlRevision: v.Record.ControlRevision, Action: "grant", Confirm: true, ConfirmSourceSharing: true, ConfirmRemoteProvider: true, ScopeHash: v.ScopeHash, GrantSeconds: 3600}
	for _, mutate := range []func(*TriggerControl){
		func(r *TriggerControl) { r.Confirm = false }, func(r *TriggerControl) { r.ConfirmSourceSharing = false },
		func(r *TriggerControl) { r.ControlRevision++ }, func(r *TriggerControl) { r.Configuration = "wrong" },
		func(r *TriggerControl) { r.Policy = "different" }, func(r *TriggerControl) { r.ProjectRevision = "stale" },
		func(r *TriggerControl) { r.ScopeHash = "wrong" }, func(r *TriggerControl) { r.GrantSeconds = 86401 },
	} {
		request := base
		mutate(&request)
		if _, err := s.ControlAgentTriggers(t.Context(), request); err == nil {
			t.Fatal("stale or unconfirmed grant admitted")
		}
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.go"), []byte("package secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "secret.go"), filepath.Join(root, "link.go")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../secret.go", "link.go", ".mini-orca/context-policy.json", "missing.go"} {
		request := base
		request.Action = "watch"
		request.Options = &agentstate.TriggerOptions{Mode: "changes", Paths: []string{path}}
		if _, err := s.ControlAgentTriggers(t.Context(), request); err == nil {
			t.Fatal("unsafe source watch admitted", path)
		}
	}
	if calls.Load() != 0 || !triggerView(t, s).Owned {
		t.Fatal("rejected control dispatched or discarded a healthy watcher")
	}
}

func TestAgentTriggersDebounceIntervalDedupAndQuietPeriod(t *testing.T) {
	now := time.Now().UTC()
	limits := triggerFixtureLimits(t)
	r := agentstate.TriggerRecord{Options: agentstate.TriggerOptions{Mode: "changes", Paths: []string{"main.go"}}, Observed: map[string]string{"main.go": "new"}, Handled: map[string]string{"main.go": "old"}, ChangedAt: now, NextEligibleAt: now.Add(time.Minute)}
	if eligibleTriggerPath(&r, now.Add(10*time.Second), limits) != "" {
		t.Fatal("quiet period ignored")
	}
	r.NextEligibleAt = now
	if eligibleTriggerPath(&r, now.Add(time.Second), limits) != "" {
		t.Fatal("debounce ignored")
	}
	if eligibleTriggerPath(&r, now.Add(4*time.Second), limits) != "main.go" {
		t.Fatal("stable change unavailable")
	}
	r.Handled["main.go"] = "new"
	if eligibleTriggerPath(&r, now.Add(time.Minute), limits) != "" {
		t.Fatal("same snapshot replayed")
	}
	r.Options.Mode, r.Options.IntervalSeconds = "interval", 120
	if eligibleTriggerPath(&r, now.Add(time.Minute), limits) != "" || !r.NextEligibleAt.Equal(now.Add(3*time.Minute)) || r.State != "unchanged" {
		t.Fatal("interval replayed unchanged snapshots")
	}
}

func triggerFixtureLimits(t *testing.T) config.AgentTriggers {
	t.Helper()
	limits, err := config.ResolveAgentTriggers(config.AgentTriggers{})
	if err != nil {
		t.Fatal(err)
	}
	return limits
}

func TestAgentTriggersRestartRetainsChargesWithoutAdoptingProviderPermission(t *testing.T) {
	server, calls := analysisResponseServer(t, emptyAnalysisReply)
	s, root, _ := triggerService(t, server.URL)
	triggerControl(t, s, "watch", &agentstate.TriggerOptions{Mode: "interval", IntervalSeconds: 60, Paths: []string{"main.go"}})
	triggerControl(t, s, "grant", nil)
	owner := triggerOwnerForTest(s)
	ageTriggerObservation(t, owner)
	if err := s.triggerTick(owner, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	before := triggerView(t, s)
	if err := s.stopTriggers(t.Context()); err != nil {
		t.Fatal(err)
	}
	restarted, err := New(s.config, s.manager)
	if err != nil {
		t.Fatal(err)
	}
	after := triggerView(t, restarted)
	if after.Owned || after.State != "interrupted" || calls.Load() != 1 || before.Record.Window.Charged != after.Record.Window.Charged {
		t.Fatal("restart adopted a grant or lost its charge")
	}
	if _, err := os.Stat(filepath.Join(root, "main.go")); err != nil {
		t.Fatal(err)
	}
}

func TestAgentTriggerSourcePolicyAndGrantExpiryStopAutomaticDispatch(t *testing.T) {
	for _, change := range []string{"policy", "expiry", "source"} {
		t.Run(change, func(t *testing.T) {
			server, calls := analysisResponseServer(t, emptyAnalysisReply)
			s, root, _ := triggerService(t, server.URL)
			triggerControl(t, s, "watch", &agentstate.TriggerOptions{Mode: "changes", Paths: []string{"main.go"}})
			triggerControl(t, s, "grant", nil)
			changeTriggerFile(t, s, root, "Changed")
			owner := triggerOwnerForTest(s)
			now := time.Now().UTC()
			switch change {
			case "policy":
				if err := os.WriteFile(filepath.Join(root, ".mini-orcaignore"), []byte("main.go\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "source":
				if err := os.Remove(filepath.Join(root, "main.go")); err != nil {
					t.Fatal(err)
				}
			case "expiry":
				r, err := owner.store.Triggers(t.Context(), owner.projectID)
				if err != nil {
					t.Fatal(err)
				}
				now = r.Grant.ExpiresAt.Add(time.Second)
				if triggerViewState(triggerView(t, s), "current", now) != "grant_expired" {
					t.Fatal("expiry was not visible")
				}
			}
			err := s.triggerTick(owner, now)
			if err != nil {
				t.Fatal(err)
			}
			if calls.Load() != 0 {
				t.Fatal("stale grant dispatched")
			}
			if change == "expiry" && triggerView(t, s).Record.State != "suggested" {
				t.Fatal("expired grant stopped recording watch-only suggestions")
			}
			if change != "expiry" && !strings.Contains(triggerView(t, s).Record.Reason, "unavailable") {
				t.Fatal("source failure became empty success")
			}
		})
	}
}
