package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

func TestAgentWorkflowStreamingOutputIsOptInAndPrivate(t *testing.T) {
	base, _, initial := workflowService(t, [3]func() string{func() string { return workflowImplementation }, func() string { return workflowTests }, func() string { return workflowApproval }})
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Model  string `json:"model"`
			Stream bool   `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || !request.Stream {
			t.Errorf("stream choice lost: %+v %v", request, err)
		}
		content := map[string]string{"creator": workflowImplementation, "tester": workflowTests, "reviewer": workflowApproval}[request.Model]
		w.Header().Set("Content-Type", "text/event-stream")
		data, _ := json.Marshal(map[string]any{"model": request.Model, "choices": []any{map[string]any{"index": 0, "delta": map[string]string{"content": content}}}})
		fmt.Fprintf(w, "data: %s\n\n", data)
		io.WriteString(w, "data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer provider.Close()
	cfg := scopedTestConfig(provider.URL)
	cfg.ModelScopes.Bug.Model, cfg.ModelScopes.Analyze.Model, cfg.ModelScopes.Code.Model = "creator", "tester", "reviewer"
	s, err := New(cfg, base.manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.CancelChangeWorkflows)
	if _, err := s.TrustProjectExecution(t.Context(), initial.ProjectRevision, true); err != nil {
		t.Fatal(err)
	}
	request := workflowRequest(initial)
	request.StreamOutput = true
	if _, err := s.StartChangeWorkflow(t.Context(), initial.ID, request); err != nil {
		t.Fatal(err)
	}
	ready := finishedWorkflow(t, s, initial.ID)
	if ready.Workflow.Status != "awaiting_human_review" || !ready.Workflow.StreamOutput {
		t.Fatalf("streamed workflow failed: %+v", ready.Workflow)
	}
	query := AgentRunQuery{ProjectID: ready.ProjectID, ProjectRevision: ready.ProjectRevision}
	history, store, err := s.readAgentHistory(t.Context(), query)
	if err != nil {
		t.Fatal(err)
	}
	outputs := 0
	for _, run := range history.Runs {
		if run.SchemaVersion != "completion-v1" {
			continue
		}
		outputs++
		if !s.agentRuns.privateText(run.ID).Available {
			t.Fatal("streaming did not reach private observer")
		}
		text, err := s.readAgentText(t.Context(), store, run)
		if err != nil || !text.Available || text.State != "final_model_output" || text.Text == "" {
			t.Fatalf("final output: %+v %v", text, err)
		}
	}
	if outputs != 5 {
		t.Fatalf("unexpected model invocation count: %d", outputs)
	}
	// The provider-neutral compatibility type is still used by ordinary callers.
	if !llm.ProviderCapabilities(s.conf().runtimes.bug.profile).Streaming {
		t.Fatal("streamed with unavailable capability")
	}
}

func TestAgentEventsResetReplayAndDroppedNotifications(t *testing.T) {
	s, store, query := agentHistoryFixture(t)
	run := seedAgentRun(t, s, store, "root", "", time.Now().UTC())
	initial, err := s.AgentRunEvents(t.Context(), query)
	if err != nil || !initial.Reset || initial.Snapshot == nil || len(initial.Snapshot.Runs) != 1 || len(initial.Events) != 0 {
		t.Fatalf("initial snapshot: %+v %v", initial, err)
	}
	query.Cursor = initial.Cursor
	// There is deliberately no subscriber while the durable terminal event is written.
	if err := store.FinishRun(t.Context(), run.ID, "completed", nil); err != nil {
		t.Fatal(err)
	}
	batch, err := s.AgentRunEvents(t.Context(), query)
	if err != nil || batch.Reset || len(batch.Events) != 1 || batch.Events[0].State != "completed" || batch.Cursor == initial.Cursor {
		t.Fatalf("replay lost terminal state: %+v %v", batch, err)
	}
	query.Cursor = batch.Cursor
	empty, err := s.AgentRunEvents(t.Context(), query)
	if err != nil || empty.Reset || len(empty.Events) != 0 {
		t.Fatalf("duplicate replay: %+v %v", empty, err)
	}
	for _, cursor := range []string{"foreign:1", progressCursor(initial.Epoch, 9999)} {
		query.Cursor = cursor
		reset, err := s.AgentRunEvents(t.Context(), query)
		if err != nil || !reset.Reset || reset.Snapshot.Runs[0].Status != "completed" || len(reset.Events) != 0 {
			t.Fatalf("foreign/future cursor: %+v %v", reset, err)
		}
	}
	query.Cursor = "bad\ncursor"
	if _, err := s.AgentRunEvents(t.Context(), query); err == nil {
		t.Fatal("invalid cursor accepted")
	}
}

func TestAgentPrivateTextBoundsUTF8AndOwnership(t *testing.T) {
	c := newAgentRunController()
	c.appendText("missing", "secret")
	if c.privateText("missing").Available {
		t.Fatal("unowned output retained")
	}
	for i := range 65 {
		id := fmt.Sprint(i)
		c.started(agentstate.Run{ID: id}, func() {})
		c.appendText(id, strings.Repeat("é", maxAgentPrivateText))
		c.ended(id)
	}
	if c.privateText("0").Available {
		t.Fatal("unbounded retained private runs")
	}
	value := c.privateText("64")
	if !value.Available || !value.Truncated || len(value.Text) != maxAgentPrivateText || !utf8.ValidString(value.Text) {
		t.Fatalf("private text bound: %d %+v", len(value.Text), value.Truncated)
	}
	c.appendText("64", "ignored")
	if c.privateText("64").Text != value.Text {
		t.Fatal("ended run retained late text")
	}
}
