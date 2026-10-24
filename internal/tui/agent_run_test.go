package tui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

func executionState() (*State, AgentRunDetail) {
	s, detail := progressState()
	detail.Run.CanCancel = true
	child := detail.Run
	child.ID = "model-call"
	child.ParentID = detail.Run.ID
	child.Kind = "completion"
	detail.Invocations = []AgentRun{detail.Run, child}
	s.applyAgentProgress(&agentProgressView{Request: *s.agentProgressRequest(), Detail: &detail, Status: "current"})
	s.Navigate(PageAgents)
	s.AgentDetailOpen = true
	return s, detail
}

func pendingInputFixture(s *State) {
	s.Change.Hash = "proposal"
	s.Change.Revision = 3
	s.Change.Workflow.Status = "awaiting_human_review"
	s.Change.Workflow.Input = &client.ChangeInput{ProjectID: "checkout", ProjectRevision: "rev", Revision: 3, Hash: "proposal", WorkflowID: "workflow", RequestID: "request", RunID: "confirmation", Status: "waiting", Action: "review_proposal", ExpiresAt: "2026-10-25T01:00:00Z", Scope: []client.ChangeInputScope{{Path: "main.go", SourceHash: "before", ProposalHash: "after"}}}
}

func TestExecutionSectionsAndCommandBrowsingArePassive(t *testing.T) {
	s, _ := executionState()
	pendingInputFixture(s)
	for i, name := range agentRunSections {
		if i > 0 {
			s.HandleKey(KeyEvent{Key: "ArrowRight"})
		}
		if s.AgentRunSection != i || s.Page != PageAgents {
			t.Fatalf("section %s changed workspace", name)
		}
		if i == 4 {
			body := strings.Join(s.renderAgentRunPage(themeForPalette(s.Palette), 120), "\n")
			for _, text := range []string{"Destination: /workspace/task", "main.go", "before", "after", ":run-input approve"} {
				if !strings.Contains(body, text) {
					t.Fatalf("missing exact approval scope %q", text)
				}
			}
		}
	}
	s.ExecuteCommand("pause")
	s.ExecuteCommand("resume")
	if len(s.PendingActions) != 0 || !strings.Contains(s.CmdOutput, "unavailable") {
		t.Fatal("navigation or unavailable control dispatched work")
	}
	s.openCommandInput("run-input")
	_ = s.Render(80, 24)
	if len(s.PendingActions) != 0 {
		t.Fatal("browsing submitted a decision")
	}
}

func TestExecutionInputUsesExactGuardAndNeverApplies(t *testing.T) {
	for _, decision := range []string{"approve", "reject", "cancel"} {
		t.Run(decision, func(t *testing.T) {
			s, _ := executionState()
			pendingInputFixture(s)
			snapshot := cloneChangeSessionSnapshot(s.Change)
			requests := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Method != "POST" || r.URL.Path != "/api/projects/current/agent-runs/confirmation/input" || r.URL.Query().Get("worktree_path") != "/workspace/task" {
					t.Errorf("wrong action: %s %s", r.Method, r.URL)
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body["hash"] != "proposal" || body["revision"] != float64(3) || body["workflow_id"] != "workflow" || body["request_id"] != "request" || body["decision"] != decision || body["project_id"] != "checkout" || body["project_revision"] != "rev" {
					t.Errorf("lost input guard: %v", body)
				}
				changed := cloneChangeSessionSnapshot(&snapshot)
				input := *changed.Workflow.Input
				input.Status = map[string]string{"approve": "approved", "reject": "rejected", "cancel": "canceled"}[decision]
				changed.Workflow.Input = &input
				_ = json.NewEncoder(w).Encode(changed)
			}))
			defer srv.Close()
			cl, err := NewClient(srv.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer cl.Close()
			s.ExecuteCommand("run-input " + decision)
			s.ExecuteCommand("run-input " + decision)
			if len(s.PendingActions) != 1 {
				t.Fatal("duplicate input was queued")
			}
			s.FlushActions(t.Context(), cl)
			if requests != 1 || s.Error != "" || s.Change.State == "applied" || s.AgentRunPending != "" || !strings.Contains(s.CmdOutput, "Apply remains") {
				t.Fatalf("decision result: requests=%d error=%s change=%+v", requests, s.Error, s.Change)
			}
		})
	}
}

func TestPrivateDisclosureIsExplicitSanitizedAndScopeBound(t *testing.T) {
	s, detail := executionState()
	s.ExecuteCommand("run-output unknown")
	if s.Error == "" || len(s.PendingActions) != 0 {
		t.Fatal("unknown invocation disclosed")
	}
	s.Error = ""
	s.ExecuteCommand("run-output model-call")
	action := *s.PendingActions[0].AgentRun
	s.PendingActions = nil
	private := detail
	private.Run = detail.Invocations[1]
	private.Text = &client.AgentPrivateText{Available: true, State: "partial_model_output", Text: "hello\x1b[2J\x1b]52;c;injected\a\nworld", ToolRecords: []client.AgentToolRecord{{Name: "read\x1b[31m", Kind: "arguments", Value: "{\"path\":\"main.go\"}"}}}
	s.DetailScroll = 9
	s.AgentStageID = "review"
	s.applyAgentRunAction(agentRunActionResult{Action: action, Detail: private})
	lines := strings.Join(s.renderAgentRunPage(themeForPalette(s.Palette), 100), "\n")
	if strings.Contains(lines, "\x1b[2J") || strings.Contains(lines, "\x1b]52") || !strings.Contains(lines, "\\x1b") || !strings.Contains(lines, "main.go") {
		t.Fatal("private text was not safely disclosed")
	}
	if s.DetailScroll != 9 || s.AgentStageID != "review" {
		t.Fatal("streamed text moved selection or scroll")
	}
	s.ExecuteCommand("hide-output")
	s.applyAgentRunAction(agentRunActionResult{Action: action, Detail: private})
	if s.AgentDisclosure != nil {
		t.Fatal("late result reopened hidden output")
	}
	s.ExecuteCommand("run-output model-call")
	action = *s.PendingActions[0].AgentRun
	s.PendingActions = nil
	s.Project.ProjectRevision = "other"
	s.applyAgentRunAction(agentRunActionResult{Action: action, Detail: private})
	if s.AgentDisclosure.Detail != nil {
		t.Fatal("foreign project result published")
	}
}

func TestPrivateRequestAndNativeCancellationUseSeparateEndpoints(t *testing.T) {
	s, detail := executionState()
	requests := make(chan string, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.Method + " " + r.URL.Path
		response := detail
		if r.Method == http.MethodGet {
			if r.URL.Query().Get("include_text") != "true" {
				t.Error("missing explicit disclosure")
			}
			response.Run = detail.Invocations[1]
			response.Text = &client.AgentPrivateText{State: "unavailable"}
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer srv.Close()
	cl, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	s.ExecuteCommand("run-output model-call")
	s.FlushActions(context.Background(), cl)
	s.ExecuteCommand("run-cancel")
	s.FlushActions(context.Background(), cl)
	if s.Error != "" {
		t.Fatal(s.Error)
	}
	if <-requests != "GET /api/projects/current/agent-runs/model-call" || <-requests != "POST /api/projects/current/agent-runs/run/control" {
		t.Fatal("unexpected execution endpoint")
	}
}

func TestQueuedExecutionActionIsDroppedWhenScopeOrDisclosureChanges(t *testing.T) {
	for _, replacement := range []string{"scope", "hidden"} {
		t.Run(replacement, func(t *testing.T) {
			s, _ := executionState()
			s.ExecuteCommand("run-output model-call")
			action := s.PendingActions[0]
			if replacement == "scope" {
				s.Project.ProjectRevision = "new"
			} else {
				s.ExecuteCommand("hide-output")
			}
			m := NewBubbleModel(t.Context(), s, nil, 80, 24)
			cmd, handled := m.prepareAsyncActionCmd("request", action)
			if !handled || cmd != nil || s.AgentRunPending != "" {
				t.Fatal("obsolete queued disclosure survived or blocked future controls")
			}
		})
	}
}

func TestExecutionDetailsFitAllPalettesAndSizes(t *testing.T) {
	for _, palette := range []Palette{PaletteTokyo, PaletteCRT, PaletteAmber, PalettePaper} {
		for _, size := range [][2]int{{52, 16}, {80, 24}, {100, 20}, {160, 40}} {
			s, detail := executionState()
			s.Palette = palette
			pendingInputFixture(s)
			detail.Events[0].NodeID = strings.Repeat("long-node-", 100)
			for section := range agentRunSections {
				s.AgentRunSection = section
				s.DetailScroll = 0
				frame := s.Render(size[0], size[1])
				assertFrameSize(t, frame, size[0], size[1])
				if !strings.Contains(frame, "[:] Commands") {
					t.Fatal("command access disappeared")
				}
			}
		}
	}
}
