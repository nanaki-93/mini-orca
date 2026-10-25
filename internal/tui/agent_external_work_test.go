package tui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

const externalTestID = "abcdefghijklmnopqrstuvwx12"

func externalTestState() *State {
	s := connectionsTestState()
	model := s.Models.Scopes["code"]
	model.Model, model.ProviderOrigin, model.RemoteProvider = "research-model", "https://provider.example", true
	model.Capabilities.Grounding = true
	s.Models.Scopes["code"] = model
	remote := &s.AgentConnections.Records[client.ConnectionRemote][0]
	remote.Inventory.Skills = []client.ConnectionSkill{{ID: "review", Name: "Review"}}
	remote.Inventory.Streaming = true
	v := newExternalView(s.Project.ProjectIdentity, s.AgentConnections.Scope, s.Project.Path)
	for _, kind := range []string{externalAPI, externalResearch, externalRemote} {
		v.Loaded[kind] = true
	}
	request := json.RawMessage(`{"operation_id":"create","method":"POST","url":"https://api.example/items","body":{"title":"reviewed"}}`)
	hash := sha256.Sum256(request)
	v.Actions = []client.APIAction{{ProjectIdentity: s.Project.ProjectIdentity, ID: externalTestID, IntegrationID: "catalog", Version: 1, Request: request, RequestHash: hex.EncodeToString(hash[:]), State: "candidate", CanApprove: true, ExpiresAt: *remote.ExpiresAt}}
	v.Research = &client.ResearchOverview{State: "available", Model: model.Model, Reports: []client.ResearchReport{{ProjectIdentity: s.Project.ProjectIdentity, ID: externalTestID, Title: "Retained report", State: "grounded", Model: model.Model}}}
	endpoint := sha256.Sum256([]byte(remote.Config.Endpoint))
	v.Delegations = []client.Delegation{{ProjectIdentity: s.Project.ProjectIdentity, ID: externalTestID, Version: 1, RemoteID: remote.Config.ID, EndpointHash: hex.EncodeToString(endpoint[:]), CardFingerprint: remote.Inventory.Fingerprint, Skill: "review", Kind: "review", State: "unknown", Advisory: true, Progress: client.RemoteProgress{TaskID: "task", ContextID: "context", State: "unknown", Text: "Advisory result", Usage: client.RemoteUsage{Source: "unknown"}}}}
	detail := v.Delegations[0]
	v.Detail = &detail
	s.AgentExternal = v
	s.Index = &ProjectIndex{ProjectIdentity: s.Project.ProjectIdentity, Files: []IndexedFile{{Path: "main.go", ContentHash: "captured-source"}}}
	s.openTaskPlan(&ChangeSeed{Title: "Review", Paths: []string{"main.go"}, Plan: &ChangePlanInput{Problem: "Coverage", Approach: "Inspect"}})
	s.Navigate(PageExternalWork)
	return s
}

func externalPending(t *testing.T, ctx context.Context, s *State, cl *Client) (*BubbleModel, tea.Cmd) {
	t.Helper()
	if len(s.PendingActions) != 1 {
		t.Fatalf("expected one external action: %d %s", len(s.PendingActions), s.Error)
	}
	model := NewBubbleModel(ctx, s, cl, 80, 24)
	cmd, ok := model.prepareAsyncActionCmd("external-test", s.PendingActions[0])
	s.PendingActions = nil
	if !ok || cmd == nil {
		t.Fatalf("external action not prepared: %s", s.Error)
	}
	return model, cmd
}

func externalTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	cl, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cl.Close() })
	return cl
}

func TestExternalAPIRequiresExactReviewAndNeverRetriesUnknownOutcome(t *testing.T) {
	s := externalTestState()
	s.confirmAPIAction("approve", "", "")
	if s.Confirmation != nil || len(s.PendingActions) != 0 {
		t.Fatal("API approval bypassed inspection")
	}
	s.Error = ""
	s.inspectExternalReceipt()
	s.confirmAPIAction("approve", "", "")
	if s.Confirmation == nil || !strings.Contains(s.Confirmation.Message, "https://api.example/items") || !strings.Contains(s.Confirmation.Message, s.AgentExternal.Actions[0].RequestHash) {
		t.Fatal("confirmation omitted the exact request")
	}
	s.AgentExternal.Actions[0].Version++
	s.HandleKey(KeyEvent{Key: "Enter"})
	if len(s.PendingActions) != 0 {
		t.Fatal("stale version confirmation dispatched")
	}
	s.Error = ""
	s.inspectExternalReceipt()
	s.confirmAPIAction("approve", "", "")
	s.HandleKey(KeyEvent{Key: "Enter"})
	var calls atomic.Int64
	cl := externalTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var request client.APIActionControl
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		value := s.AgentExternal.Actions[0]
		if request.RequestHash != value.RequestHash || request.Version != value.Version || !request.Confirm {
			t.Error("lost exact approval")
		}
		value.Version++
		value.State, value.CanApprove, value.CanReconcile = "unknown", false, true
		_ = json.NewEncoder(w).Encode(value)
	})
	model, cmd := externalPending(t, t.Context(), s, cl)
	model.Update(cmd())
	s.confirmAPIAction("approve", "", "")
	if calls.Load() != 1 || len(s.PendingActions) != 0 || s.Confirmation != nil || s.AgentExternal.Actions[0].State != "unknown" {
		t.Fatal("uncertain outcome was retried or reported as success")
	}
	s.Error = ""
	s.confirmAPIAction("reconcile", "not_applied", "Checked the external service; no item exists.")
	if s.Confirmation == nil || !strings.Contains(s.Confirmation.Message, "does not send or replay") {
		t.Fatal("uncertain receipt had no explicit local recovery")
	}
	s.HandleKey(KeyEvent{Key: "Enter"})
	request := s.PendingActions[0].AgentExternal.APIControl
	if request.Action != "reconcile" || request.Version != 3 || request.ReportedOutcome != "not_applied" {
		t.Fatal("reconciliation lost retained version or reported outcome")
	}
}

func TestExternalSourceCapturePrecedesResearchAndDelegationConsent(t *testing.T) {
	for _, mode := range []string{"research", "delegate"} {
		t.Run(mode, func(t *testing.T) {
			s := externalTestState()
			var reads, writes atomic.Int64
			cl := externalTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					reads.Add(1)
					if r.URL.Path != "/api/projects/current/index" {
						t.Error("unexpected capture read")
					}
					_ = json.NewEncoder(w).Encode(s.Index)
					return
				}
				writes.Add(1)
				if mode == "research" {
					var body client.ResearchRequest
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if len(body.Sources) != 1 || body.Sources[0].Hash != "captured-source" || !body.ConfirmRemoteProvider || !body.ConfirmSearch || !body.ConfirmSourceSharing {
						t.Error("research lost captured source or consent")
					}
					_ = json.NewEncoder(w).Encode(s.AgentExternal.Research.Reports[0])
				} else {
					var body client.DelegationRequest
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if body.Kind != "review" || body.Skill != "review" || len(body.Sources) != 1 || !body.Remote.ConfirmSourceSharing || !body.ConfirmDelegate || !body.Stream {
						t.Error("delegation lost intent or sharing consent")
					}
					_ = json.NewEncoder(w).Encode(s.AgentExternal.Delegations[0])
				}
			})
			d := externalDraft{Mode: mode, Question: "Inspect this API", Kind: "review", Skill: "review"}
			d.Remote.Config.ID = "reviewer"
			s.captureExternalWork(d)
			if s.Confirmation != nil || writes.Load() != 0 {
				t.Fatal("external consent preceded source capture")
			}
			model, cmd := externalPending(t, t.Context(), s, cl)
			model.Update(cmd())
			if s.Confirmation == nil || reads.Load() != 1 || writes.Load() != 0 {
				t.Fatal("capture contacted an external destination", s.Error)
			}
			for _, text := range []string{"main.go", "captured-source", "/repo", "advisory", "Not shared (no indexed source): main_test.go"} {
				if !strings.Contains(s.Confirmation.Message, text) {
					t.Fatal("confirmation omitted scope", text)
				}
			}
			s.HandleKey(KeyEvent{Key: "Enter"})
			model, cmd = externalPending(t, t.Context(), s, cl)
			model.Update(cmd())
			if writes.Load() != 1 || s.AgentExternal.Pending != "" || s.Error != "" {
				t.Fatalf("external request failed: %s", s.Error)
			}
		})
	}
}

func TestExternalSourceCaptureDoesNotInventMissingFilesOrQuestionOnlyConsent(t *testing.T) {
	index := &ProjectIndex{Files: []IndexedFile{{Path: "existing.go", ContentHash: "current"}}}
	sources, unshared, err := externalCapturedSources([]string{"existing.go", "new.go"}, index)
	if err != nil || len(sources) != 1 || sources[0].Hash != "current" || len(unshared) != 1 || unshared[0] != "new.go" {
		t.Fatal("source capture invented missing content", sources, unshared, err)
	}
	if _, _, err := externalCapturedSources([]string{"new.go"}, index); err == nil {
		t.Fatal("missing sources became implicit question-only consent")
	}
	if _, _, err := externalCapturedSources([]string{"existing.go", "existing.go"}, index); err == nil {
		t.Fatal("accepted repeated source scope")
	}
}

func TestExternalConfirmationRejectsChangedPlanProviderGrantAndCheckout(t *testing.T) {
	for _, change := range []string{"plan", "provider", "grant", "expiry", "checkout"} {
		t.Run(change, func(t *testing.T) {
			s := externalTestState()
			mode := "research"
			if change == "grant" || change == "expiry" {
				mode = "delegate"
			}
			d := externalDraft{Mode: mode, Question: "Inspect", Kind: "review", Skill: "review"}
			d.Remote.Config.ID = "reviewer"
			s.captureExternalWork(d)
			a := *s.PendingActions[0].AgentExternal
			s.PendingActions = nil
			s.applyExternalResult(agentExternalResult{Action: a, Index: s.Index})
			if s.Confirmation == nil {
				t.Fatal(s.Error)
			}
			switch change {
			case "plan":
				s.PlanDraft.Seed.Plan.Approach = "Changed"
			case "provider":
				model := s.Models.Scopes["code"]
				model.ProviderOrigin = "https://different.example"
				s.Models.Scopes["code"] = model
			case "grant":
				s.AgentConnections.Records[client.ConnectionRemote][0].Grant = "changed"
			case "expiry":
				expired := time.Now().Add(-time.Minute)
				s.AgentConnections.Records[client.ConnectionRemote][0].ExpiresAt = &expired
			case "checkout":
				s.Project.ProjectRevision = "changed"
			}
			s.HandleKey(KeyEvent{Key: "Enter"})
			if len(s.PendingActions) != 0 || s.Error == "" {
				t.Fatal("changed scope accepted old consent")
			}
		})
	}
}

func TestExternalQueuedWorkRechecksProviderAndDoesNotShareNewSources(t *testing.T) {
	s := externalTestState()
	s.captureExternalWork(externalDraft{Mode: "research", Question: "Inspect"})
	a := *s.PendingActions[0].AgentExternal
	s.PendingActions = nil
	s.applyExternalResult(agentExternalResult{Action: a, Index: s.Index})
	s.HandleKey(KeyEvent{Key: "Enter"})
	model := s.Models.Scopes["code"]
	model.Model = "changed"
	s.Models.Scopes["code"] = model
	bubble := NewBubbleModel(t.Context(), s, nil, 80, 24)
	cmd, _ := bubble.prepareAsyncActionCmd("queued", s.PendingActions[0])
	if cmd != nil || s.AgentExternal.Pending != "" || s.Error == "" {
		t.Fatal("queued research was retargeted")
	}
}

func TestExternalAdmissionRejectsUnavailableCapabilitiesAndUnboundedInput(t *testing.T) {
	for _, mode := range []string{"disabled", "tools-only", "too-long", "too-many-paths", "unknown-skill", "foreign-checkout"} {
		s := externalTestState()
		d := externalDraft{Mode: "research", Question: "Inspect"}
		switch mode {
		case "disabled":
			s.AgentExternal.Research.State = "disabled"
		case "tools-only":
			model := s.Models.Scopes["code"]
			model.Capabilities.Grounding = false
			model.Capabilities.Tools = true
			s.Models.Scopes["code"] = model
		case "too-long":
			d.Question = strings.Repeat("界", 700)
		case "too-many-paths":
			s.PlanDraft.Seed.Paths = make([]string, 9)
		case "unknown-skill":
			d.Mode, d.Kind, d.Skill, d.Remote.Config.ID = "delegate", "review", "not-allowed", "reviewer"
		case "foreign-checkout":
			d.Mode, d.Kind, d.Skill, d.Remote.Config.ID = "delegate", "review", "review", "reviewer"
			s.AgentConnections.Scope.WorktreePath = "/other"
		}
		s.captureExternalWork(d)
		if len(s.PendingActions) != 0 || s.Confirmation != nil || s.Error == "" {
			t.Fatal("invalid external work was admitted", mode)
		}
	}
	s := externalTestState()
	temperature := float32(0.2)
	model := s.Models.Scopes["code"]
	model.Temperature = &temperature
	s.Models.Scopes["code"] = model
	d := externalDraft{Mode: "research", Question: "Inspect"}
	if err := s.prepareExternalDraft(&d); err != nil {
		t.Fatal(err)
	}
	temperature = 0.8
	if err := s.externalDraftCurrent(&d); err == nil {
		t.Fatal("mutated model parameters changed the captured consent snapshot")
	}
}

func TestRemoteInputUsesFreshReplyConsentAndMatchingRetainedTask(t *testing.T) {
	s := externalTestState()
	s.SelectedIdx = 2
	s.AgentExternal.Detail.State = "input_required"
	s.AgentExternal.Delegations[0].State = "input_required"
	s.confirmRemoteControl("input", "Use the documented behavior.")
	if s.Confirmation == nil || !strings.Contains(s.Confirmation.Message, "Confirm sharing this reply once") {
		t.Fatal("reply bypassed source-sharing consent", s.Error)
	}
	s.HandleKey(KeyEvent{Key: "Enter"})
	var calls atomic.Int64
	cl := externalTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var request client.DelegationControl
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request.Action != "input" || request.Reply != "Use the documented behavior." || request.Version != 1 || !request.Remote.ConfirmSourceSharing || request.Remote.ID != "reviewer" {
			t.Error("reply lost its task binding")
		}
		value := *s.AgentExternal.Detail
		value.Version++
		value.State, value.Progress.State = "working", "working"
		_ = json.NewEncoder(w).Encode(value)
	})
	bubble, cmd := externalPending(t, t.Context(), s, cl)
	bubble.Update(cmd())
	if calls.Load() != 1 || s.AgentExternal.Detail.State != "working" {
		t.Fatal("reply failed", s.Error)
	}
	s.confirmRemoteControl("input", "Another reply")
	if s.Confirmation != nil || len(s.PendingActions) != 0 {
		t.Fatal("reply sent without an input-required receipt")
	}
}

func TestExternalTaskCaptureAndQuestionOnlyRemainExplicit(t *testing.T) {
	s := externalTestState()
	child := ProjectIdentity{ProjectID: "child", ProjectRevision: "child-revision"}
	path := "/tmp/task"
	s.Change = &ChangeSession{ProjectIdentity: child, WorktreePath: path}
	s.AgentExternal.Scope, s.AgentExternal.Path = client.AgentSettingsScope{ProjectIdentity: child, WorktreePath: path}, path
	s.PlanDraft.Seed.WorktreePath = path
	cl := externalTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("worktree_path") != path || r.URL.Query().Get("project_id") != child.ProjectID {
			t.Error("task scope lost")
		}
		_ = json.NewEncoder(w).Encode(ProjectIndex{ProjectIdentity: s.Project.ProjectIdentity, Files: s.Index.Files})
	})
	s.captureExternalWork(externalDraft{Mode: "research", Question: "Inspect task"})
	model, cmd := externalPending(t, t.Context(), s, cl)
	model.Update(cmd())
	if s.Confirmation != nil || s.Error == "" {
		t.Fatal("main index was accepted for task checkout")
	}
	s.Error = ""
	s.PlanDraft = nil
	s.captureExternalWork(externalDraft{Mode: "research", Question: "Inspect"})
	if len(s.PendingActions) != 0 {
		t.Fatal("missing plan became implicit question-only sharing")
	}
	s.ExecuteCommand(":research --question-only Explain a pattern")
	a := *s.PendingActions[0].AgentExternal
	s.PendingActions = nil
	s.applyExternalResult(agentExternalResult{Action: a, Index: &ProjectIndex{ProjectIdentity: child}})
	if s.Confirmation == nil || !strings.Contains(s.Confirmation.Message, "no project source files") {
		t.Fatal("explicit question-only selection failed", s.Error)
	}
}

func TestRemoteControlsFollowReceiptStateAndDoNotInventRecovery(t *testing.T) {
	for _, test := range []struct {
		state, task, action            string
		active, stale, future, allowed bool
	}{
		{"unknown", "", "refresh", false, false, false, false},
		{"unknown", "", "reconcile", false, true, false, true},
		{"auth_required", "task", "reconcile", false, false, false, true},
		{"input_required", "task", "input", false, false, false, true},
		{"input_required", "task", "input", false, true, false, false},
		{"working", "task", "cancel", false, false, false, true},
		{"unknown", "task", "refresh", true, false, false, false},
		{"pending", "task", "reconcile", false, false, true, false},
		{"pending", "task", "reconcile", false, false, false, true},
		{"completed", "task", "refresh", false, false, false, false},
	} {
		item := client.Delegation{State: test.state, Active: test.active, Stale: test.stale, Progress: client.RemoteProgress{TaskID: test.task}}
		if test.future {
			item.LeaseUntil = time.Now().Add(time.Minute)
		}
		if got := remoteControlAllowed(item, test.action, time.Now()); got != test.allowed {
			t.Fatal("incorrect remote control availability", test)
		}
	}
	s := externalTestState()
	s.SelectedIdx = 2
	s.confirmRemoteControl("refresh", "")
	if s.Confirmation == nil {
		t.Fatal(s.Error)
	}
	s.AgentExternal.Detail.Version++
	s.HandleKey(KeyEvent{Key: "Enter"})
	if len(s.PendingActions) != 0 {
		t.Fatal("changed remote receipt reused consent")
	}
	s = externalTestState()
	s.SelectedIdx = 2
	s.AgentConnections.Records[client.ConnectionRemote][0].State = "expired"
	s.confirmRemoteControl("reconcile", "Checked remotely; task no longer exists.")
	if s.Confirmation == nil {
		t.Fatal("local reconciliation required a network grant", s.Error)
	}
	s.HandleKey(KeyEvent{Key: "Enter"})
	request := s.PendingActions[0].AgentExternal.RemoteControl
	if request.Remote.Grant != "" || request.Remote.ConfirmSourceSharing || request.Action != "reconcile" {
		t.Fatal("local recovery obtained remote authority")
	}
}

func TestExternalHistoryPartialRefreshPreservesSelectionAndIsPassive(t *testing.T) {
	s := externalTestState()
	s.SelectedIdx = 2
	var reads atomic.Int64
	cl := externalTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		reads.Add(1)
		if r.Method != http.MethodGet {
			t.Error("history refresh mutated external work")
		}
		switch r.URL.Path {
		case "/api/projects/current/api-actions":
			_ = json.NewEncoder(w).Encode([]client.APIAction{})
		case "/api/projects/current/research":
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
		case "/api/projects/current/delegations":
			_ = json.NewEncoder(w).Encode(s.AgentExternal.Delegations)
		default:
			t.Error("unexpected history route", r.URL.Path)
		}
	})
	s.refreshExternalWork()
	model, cmd := externalPending(t, t.Context(), s, cl)
	model.Update(cmd())
	if reads.Load() != 3 || s.selectedExternalKey() != (externalKey{externalRemote, externalTestID}) || s.AgentExternal.Failures[externalResearch] == "" || len(s.AgentExternal.Research.Reports) != 1 || s.AgentExternal.Detail != nil {
		t.Fatal("partial refresh lost selection, failure or private disclosure state")
	}
	if len(s.PendingActions) != 0 || s.Confirmation != nil {
		t.Fatal("local refresh caused external work")
	}
}

func TestExternalCancellationStopsTransportAndRejectsLateReceipt(t *testing.T) {
	s := externalTestState()
	s.inspectExternalReceipt()
	s.confirmAPIAction("approve", "", "")
	s.HandleKey(KeyEvent{Key: "Enter"})
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	started, stopped := make(chan struct{}), make(chan struct{})
	cl := externalTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(started)
		select {
		case <-r.Context().Done():
		case <-ctx.Done():
		}
		close(stopped)
	})
	defer cancel()
	model, cmd := externalPending(t, ctx, s, cl)
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("control did not start")
	}
	s.cancelExternalRequest()
	s.FlushActions(ctx, cl)
	select {
	case <-stopped:
	case <-ctx.Done():
		t.Fatal("cancellation did not reach transport")
	}
	var message tea.Msg
	select {
	case message = <-done:
	case <-ctx.Done():
		t.Fatal("external owner did not finish")
	}
	model.Update(message)
	if s.AgentExternal.Actions[0].State != "candidate" || s.AgentExternal.Failures[externalAPI] == "" || s.AgentExternal.Pending != "" {
		t.Fatal("canceled receipt replaced retained state or became reusable")
	}
	s.confirmAPIAction("approve", "", "")
	if s.Confirmation != nil || len(s.PendingActions) != 0 {
		t.Fatal("canceled outcome dispatched before local refresh")
	}
}

func TestExternalLateResponseCannotReplaceCheckoutOrSelectedReceipt(t *testing.T) {
	s := externalTestState()
	s.inspectExternalReceipt()
	s.confirmAPIAction("approve", "", "")
	s.HandleKey(KeyEvent{Key: "Enter"})
	a := *s.PendingActions[0].AgentExternal
	s.PendingActions = nil
	value := s.AgentExternal.Actions[0]
	value.Version++
	value.State = "completed"
	s.SelectedIdx = 2
	s.applyExternalResult(agentExternalResult{Action: a, API: &value})
	if s.selectedExternalKey().Kind != externalRemote || s.AgentExternal.Actions[0].Version != 2 {
		t.Fatal("API result displaced another selection")
	}
	s = externalTestState()
	s.refreshExternalWork()
	a = *s.PendingActions[0].AgentExternal
	s.Project.ProjectRevision = "other"
	s.applyExternalResult(agentExternalResult{Action: a, Actions: []client.APIAction{}})
	if len(s.AgentExternal.Actions) != 1 {
		t.Fatal("late history replaced a different revision")
	}
}

func TestExternalViewsAreBoundedSanitizedAndPassivelyInspectable(t *testing.T) {
	for _, palette := range []Palette{PaletteTokyo, PaletteCRT, PaletteAmber, PalettePaper} {
		for _, size := range [][2]int{{52, 16}, {80, 24}, {120, 36}} {
			for selection := range 3 {
				s := externalTestState()
				s.Palette, s.SelectedIdx = palette, selection
				s.AgentExternal.Detail.Progress.Text = "remote text\x1b[2J\n" + strings.Repeat("bounded advisory text ", 75) + "end-of-advisory"
				if selection == 0 {
					s.inspectExternalReceipt()
				}
				var visited strings.Builder
				for range 30 {
					frame := s.Render(size[0], size[1])
					visited.WriteString(frame)
					for _, line := range strings.Split(frame, "\n") {
						if theme.VisibleRuneCount(line) != size[0] {
							t.Fatalf("external view overflow %v: %q", size, line)
						}
					}
					s.HandleKey(KeyEvent{Key: "PageDown"})
				}
				for _, token := range []string{"SELECTED EXTERNAL WORK", "Checkout:", "/repo"} {
					if !strings.Contains(visited.String(), token) {
						t.Fatal("external detail inaccessible", token, size, selection)
					}
				}
				if selection == 2 && !strings.Contains(visited.String(), "end-of-advisory") {
					t.Fatal("remote output was silently truncated", size)
				}
				if strings.Contains(visited.String(), "\x1b[2J") || len(s.PendingActions) != 0 || s.Confirmation != nil {
					t.Fatal("passive history dispatched or contained cursor control")
				}
			}
		}
	}
}
