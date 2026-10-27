package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

func TestExecutionRecoveryIsReachableAcrossStatesAndSizes(t *testing.T) {
	for _, status := range []string{"failed", "partial", "stale", "canceled", "interrupted", "pending-input", "disconnected"} {
		for _, palette := range []Palette{PaletteTokyo, PaletteCRT, PaletteAmber, PalettePaper} {
			for _, dims := range [][2]int{{52, 16}, {120, 36}} {
				t.Run(fmt.Sprintf("%s/%s/%dx%d", status, palette, dims[0], dims[1]), func(t *testing.T) {
					s := workspaceAgentState(status)
					s.Palette = palette
					s.AgentDetailOpen = true
					s.AgentRunSection = 4
					if status == "pending-input" {
						detail := *s.currentAgentDetail()
						s.Change.WorktreePath = "/workspace/" + strings.Repeat("nested/", 40) + "DESTINATION_END"
						s.Change.Workflow.Input.Scope[0].Path = strings.Repeat("directory/", 20) + "SOURCE_END.go"
						s.applyAgentProgress(&agentProgressView{Request: *s.agentProgressRequest(), Detail: &detail, Status: "current"})
					}
					text := readAgentView(t, s, dims)
					want := status
					if status == "pending-input" {
						want = "Recording a decision does not apply files."
						for _, tail := range []string{"DESTINATION_END", "SOURCE_END.go", "proposal", "before", "after"} {
							if !strings.Contains(text, tail) {
								t.Fatalf("approval scope clipped: %s", tail)
							}
						}
					}
					if !strings.Contains(text, want) || len(s.PendingActions) != 0 || s.Confirmation != nil {
						t.Fatalf("recovery state hidden or browsing caused an effect: %s", want)
					}
				})
			}
		}
	}
}

func readAgentView(t *testing.T, s *State, dims [2]int) string {
	t.Helper()
	s.HandleKey(KeyEvent{Key: "Home"})
	var pages strings.Builder
	for range 128 {
		frame := s.Render(dims[0], dims[1])
		assertFrameSize(t, frame, dims[0], dims[1])
		pages.WriteString(ansi.Strip(frame))
		pages.WriteByte('\n')
		before := s.DetailScroll
		s.HandleKey(KeyEvent{Key: "PageDown"})
		s.Render(dims[0], dims[1])
		if s.DetailScroll == before {
			return pages.String()
		}
	}
	t.Fatal("reading never reached the end of bounded details")
	return ""
}

func TestCompactStagesRetainFreshAdmissionGuidance(t *testing.T) {
	for _, status := range []string{"interrupted", "outdated", "stale"} {
		s, _ := executionState()
		s.Change.Workflow.Status = status
		if text := readAgentView(t, s, [2]int{52, 16}); !strings.Contains(text, "fresh consent and checks") {
			t.Fatal("compact stage history hid recovery requirements", status)
		}
	}
}

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

func TestExecutionUsageSeparatesReportedEstimatedAndLegacyConsumption(t *testing.T) {
	s, detail := executionState()
	detail.Run.Usage = client.AgentUsage{ModelCalls: 2, InputTokens: 100, OutputTokens: 50, UnknownUsageCalls: 1, Accounting: &client.AgentUsageAccounting{Version: 1, Providers: []client.AgentProviderUsage{{ModelCalls: 2, FailedCalls: 1, ReportedCalls: 1, PartialUsageCalls: 1, CacheKnownCalls: 1, ReportedInputTokens: 20, ReportedOutputTokens: 10, CachedInputTokens: 5}}}}
	text := strings.Join(renderAgentUsage(&detail), "\n")
	for _, want := range []string{"Input budget charged: 100", "Provider-reported tokens: input 20 · output 10", "Conservative estimates: input 80 · output 40", "Cached input reported: 5", "Failed provider attempts: 1", "partial usage 1", "Cost: unknown"} {
		if !strings.Contains(text, want) {
			t.Fatalf("usage disclosure missing %q: %s", want, text)
		}
	}
	s.applyAgentProgress(&agentProgressView{Request: *s.agentProgressRequest(), Detail: &detail, Status: "current"})
	s.AgentRunSection = 3
	for _, palette := range []Palette{PaletteTokyo, PaletteCRT, PaletteAmber, PalettePaper} {
		s.Palette = palette
		for range 8 {
			_ = s.Render(52, 16)
			s.HandleKey(KeyEvent{Key: "PageDown"})
		}
	}
	if len(s.PendingActions) != 0 {
		t.Fatal("usage browsing dispatched work")
	}
	detail.Run.Usage.Accounting = nil
	text = strings.Join(renderAgentUsage(&detail), "\n")
	if !strings.Contains(text, "Legacy accounting") || strings.Contains(text, "Provider-reported tokens:") {
		t.Fatal("legacy charges presented as measurements")
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
				if !strings.Contains(frame, ": commands") {
					t.Fatal("command access disappeared")
				}
			}
		}
	}
}

func TestExecutionTraceAndCostRemainPassiveReadableAndSanitized(t *testing.T) {
	for _, palette := range []Palette{PaletteTokyo, PaletteCRT, PaletteAmber, PalettePaper} {
		s, detail := executionState()
		s.Palette = palette
		detail.Trace = &client.AgentTraceSummary{Version: 1, TraceID: strings.Repeat("a", 32), Snapshot: strings.Repeat("b", 64), Truncated: true, Omitted: 3, Incomplete: true, Spans: []client.AgentTraceSpan{{ID: "0123456789abcdef", ParentID: "abcdef0123456789", Kind: "provider\x1b]52;c;PRIVATE\x07", Status: "interrupted"}}}
		detail.Cost = &client.AgentCostEstimate{Status: "unknown", PriceVersion: "fixture-v1", Currency: "USD", KnownSubtotal: 0.25, Reasons: []string{"usage_incomplete"}}
		s.applyAgentProgress(&agentProgressView{Request: *s.agentProgressRequest(), Detail: &detail, Status: "current"})
		for _, section := range []int{1, 3} {
			s.AgentRunSection = section
			text := strings.Join(s.renderAgentRunPage(themeForPalette(palette), 52), "\n")
			if strings.Contains(text, "\x1b]52") || strings.Contains(text, "\x07") {
				t.Fatal("terminal escape leaked")
			}
			for _, size := range [][2]int{{52, 16}, {80, 24}, {120, 36}} {
				for range 8 {
					assertFrameSize(t, s.Render(size[0], size[1]), size[0], size[1])
					s.HandleKey(KeyEvent{Key: "PageDown"})
				}
			}
		}
		text := strings.Join(renderAgentTimeline(&detail), "\n") + strings.Join(renderAgentUsage(&detail), "\n")
		for _, want := range []string{"3 spans omitted", "duration unfinished", "Total cost: unknown", "known subtotal 0.250000 USD", "fixture-v1"} {
			if !strings.Contains(text, want) {
				t.Fatalf("missing trace/cost disclosure %q", want)
			}
		}
		if len(s.PendingActions) != 0 {
			t.Fatal("trace/cost navigation dispatched work")
		}
	}
	total := 0.125
	text := strings.Join(renderAgentCost(&client.AgentCostEstimate{Status: "estimated", PriceVersion: "rates-v2", Currency: "USD", Total: &total}), "\n")
	if !strings.Contains(text, "Estimated token cost: 0.125000 USD") || !strings.Contains(text, "not an invoice") {
		t.Fatal("cost provenance missing", text)
	}
}

func traceExportState() *State {
	s, detail := executionState()
	detail.Trace = &client.AgentTraceSummary{Version: 1, TraceID: strings.Repeat("a", 32), Snapshot: strings.Repeat("b", 64), Incomplete: true, Truncated: true, Omitted: 3, Export: &client.AgentTraceExportPreview{Enabled: true, Destination: "https://collector.example/v1/traces", Fingerprint: strings.Repeat("c", 64), Finished: 1, Unfinished: 2}}
	s.applyAgentProgress(&agentProgressView{Request: *s.agentProgressRequest(), Detail: &detail, Status: "current"})
	return s
}

func TestTraceExportConfirmationDispatchesOnceWithExactSnapshotAndNoSourceMutation(t *testing.T) {
	for _, state := range []string{"exported", "partial", "warning", "unknown"} {
		t.Run(state, func(t *testing.T) {
			s := traceExportState()
			before := cloneChangeSessionSnapshot(s.Change)
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != http.MethodPost || r.URL.Path != "/api/projects/current/agent-runs/run/export" || r.URL.Query().Get("worktree_path") != "/workspace/task" {
					t.Error("wrong export route", r.URL)
				}
				var request client.AgentTraceExportRequest
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				if request.ProjectIdentity != s.Change.ProjectIdentity || request.Snapshot != strings.Repeat("b", 64) || request.DestinationFingerprint != strings.Repeat("c", 64) || !request.Confirm {
					t.Error("lost export admission", request)
				}
				accepted, rejected := 1, 0
				if state == "partial" {
					accepted, rejected = 0, 1
				}
				receipt := client.AgentTraceExportReceipt{ProjectIdentity: request.ProjectIdentity, RunID: "run", Snapshot: request.Snapshot, DestinationFingerprint: request.DestinationFingerprint, State: state, Sent: 1, Accepted: &accepted, Rejected: &rejected}
				if state == "unknown" {
					receipt.Accepted, receipt.Rejected = nil, nil
				}
				_ = json.NewEncoder(w).Encode(receipt)
			}))
			defer server.Close()
			cl, err := NewClient(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer cl.Close()
			s.ExecuteCommand("run-export")
			if s.Confirmation == nil || len(s.PendingActions) != 0 {
				t.Fatal("export bypassed confirmation")
			}
			for _, text := range []string{"/workspace/task", "https://collector.example/v1/traces", strings.Repeat("b", 64), strings.Repeat("c", 64), "1 finished spans", "2 unfinished spans", "3 spans", "No automatic retry"} {
				if !strings.Contains(s.Confirmation.Message, text) {
					t.Fatal("missing export disclosure", text)
				}
			}
			s.HandleKey(KeyEvent{Key: "Enter"})
			s.FlushActions(t.Context(), cl)
			if calls.Load() != 1 || s.Error != "" || s.AgentRunPending != "" || !reflect.DeepEqual(*s.Change, before) {
				t.Fatal("export changed proposal or lost receipt", s.Error)
			}
			if !strings.Contains(s.CmdOutput, state) {
				t.Fatal("delivery state missing", s.CmdOutput)
			}
		})
	}
}

func TestTraceExportRejectsChangedConfirmationsAndQueuedSnapshots(t *testing.T) {
	for _, phase := range []string{"confirmation", "queued"} {
		for _, change := range []string{"scope", "snapshot", "destination", "run"} {
			t.Run(phase+"/"+change, func(t *testing.T) {
				s := traceExportState()
				s.ExecuteCommand("run-export")
				if phase == "queued" {
					s.HandleKey(KeyEvent{Key: "Enter"})
				}
				switch change {
				case "scope":
					s.Project.ProjectRevision = "changed"
				case "snapshot":
					s.AgentProgress.Detail.Trace.Snapshot = "changed"
				case "destination":
					s.AgentProgress.Detail.Trace.Export.Fingerprint = "changed"
				case "run":
					s.AgentProgress.Detail.Run.ID = "changed"
				}
				if phase == "confirmation" {
					s.HandleKey(KeyEvent{Key: "Enter"})
					if len(s.PendingActions) != 0 || s.Error == "" {
						t.Fatal("changed confirmation queued")
					}
				} else {
					if s.acceptsAgentRunAction(s.PendingActions[0].AgentRun) || s.AgentRunPending != "" {
						t.Fatal("changed queued export dispatched")
					}
				}
			})
		}
	}
}

func TestTraceExportLateAndMismatchedReceiptsCannotBecomeSuccess(t *testing.T) {
	for _, change := range []string{"superseded", "scope", "snapshot", "counts"} {
		s := traceExportState()
		s.ExecuteCommand("run-export")
		s.HandleKey(KeyEvent{Key: "Enter"})
		action := *s.PendingActions[0].AgentRun
		accepted, rejected := 1, 0
		receipt := client.AgentTraceExportReceipt{ProjectIdentity: action.Export.ProjectIdentity, RunID: action.Invocation, Snapshot: action.Export.Snapshot, DestinationFingerprint: action.Export.DestinationFingerprint, State: "exported", Sent: 1, Accepted: &accepted, Rejected: &rejected}
		s.CmdOutput = "unchanged"
		switch change {
		case "superseded":
			s.AgentRunPending = "new-request"
		case "scope":
			s.Project.ProjectRevision = "changed"
		case "snapshot":
			receipt.Snapshot = "foreign"
		case "counts":
			accepted = 2
		}
		s.applyAgentRunAction(agentRunActionResult{Action: action, Export: receipt})
		if s.CmdOutput != "unchanged" {
			t.Fatal("late or mismatched receipt became success", change)
		}
		if (change == "snapshot" || change == "counts") && !strings.Contains(s.Error, "delivery is unknown") {
			t.Fatal("invalid acknowledgement not disclosed")
		}
	}
}

func TestTraceExportConfirmationFitsEveryPaletteAndSmallTerminal(t *testing.T) {
	for _, palette := range []Palette{PaletteTokyo, PaletteCRT, PaletteAmber, PalettePaper} {
		for _, size := range [][2]int{{52, 16}, {80, 24}, {120, 36}} {
			s := traceExportState()
			s.Palette = palette
			s.AgentProgress.Detail.Trace.Export.Destination += "\x1b]52;c;PRIVATE\x07"
			s.ExecuteCommand("run-export")
			var visited strings.Builder
			var readable strings.Builder
			for range 30 {
				frame := s.Render(size[0], size[1])
				assertFrameSize(t, frame, size[0], size[1])
				visited.WriteString(frame)
				for _, line := range strings.Split(ansi.Strip(frame), "\n") {
					readable.WriteString(strings.Trim(line, " │┌┐└┘─"))
					readable.WriteByte(' ')
				}
				s.HandleKey(KeyEvent{Key: "PageDown"})
			}
			for _, text := range []string{"Checkout:", "Destination:", "Snapshot:", "finished spans", "automatic retry"} {
				if !strings.Contains(strings.Join(strings.Fields(readable.String()), " "), text) {
					t.Fatal("unreachable confirmation content", text, size)
				}
			}
			if strings.Contains(visited.String(), "\x1b]52") || len(s.PendingActions) != 0 {
				t.Fatal("unsafe or active confirmation rendering")
			}
			s.HandleKey(KeyEvent{Key: "Escape"})
			if s.Confirmation != nil || len(s.PendingActions) != 0 {
				t.Fatal("canceling confirmation dispatched")
			}
		}
	}
}
