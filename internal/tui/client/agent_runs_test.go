package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const agentEventsTarget = "/api/projects/current/agent-runs/events?project_id=p&project_revision=r"

func agentFrame(batch AgentEventBatch) string {
	data, _ := json.Marshal(batch)
	return fmt.Sprintf("event: agent_progress\nid: %s\ndata: %s\n\n", batch.Cursor, data)
}

func agentBatchFixture(sequence uint64, reset bool) AgentEventBatch {
	batch := AgentEventBatch{ProjectID: "p", ProjectRevision: "r", Epoch: "epoch", Cursor: fmt.Sprintf("epoch:%d", sequence), Reset: reset, Events: []AgentRunEvent{}}
	if reset {
		batch.Snapshot = &AgentRunList{ProjectID: "p", ProjectRevision: "r", Runs: []AgentRun{}}
	} else {
		batch.Events = []AgentRunEvent{{Sequence: sequence, RunID: "root", InvocationID: "child", ProjectID: "p", ProjectRevision: "r", Kind: "finished", State: "completed", Usage: AgentUsage{InputTokens: 9007199254740993}}}
	}
	return batch
}

func TestAgentStreamReconnectionDuplicateSuppressionAndExactUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Last-Event-ID") != "epoch:1" || r.Header.Get("Accept") != "text/event-stream" {
			t.Errorf("reconnect headers: %+v", r.Header)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, agentFrame(agentBatchFixture(1, false))+agentFrame(agentBatchFixture(2, false))+agentFrame(agentBatchFixture(2, false)))
	}))
	defer server.Close()
	cl, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	calls := 0
	cursor, err := cl.StreamAgentEvents(t.Context(), "stream", agentEventsTarget, "epoch:1", func(batch AgentEventBatch) error {
		calls++
		if len(batch.Events) != 1 || batch.Events[0].Usage.InputTokens != 9007199254740993 {
			t.Errorf("usage/order lost: %+v", batch)
		}
		return nil
	})
	if !errors.Is(err, io.EOF) || cursor != "epoch:2" || calls != 1 {
		t.Fatalf("reconnect: cursor=%s calls=%d error=%v", cursor, calls, err)
	}
}

func TestAgentStreamResetValidationAndUnacknowledgedFailure(t *testing.T) {
	reset := agentFrame(agentBatchFixture(10, true))
	terminal := agentFrame(agentBatchFixture(11, false))
	cause := errors.New("view replaced")
	calls := 0
	cursor, err := readAgentEvents(strings.NewReader(reset+terminal), "p", "r", "old:5", func(batch AgentEventBatch) error {
		calls++
		if !batch.Reset {
			return cause
		}
		return nil
	})
	if !errors.Is(err, cause) || cursor != "epoch:10" || calls != 2 {
		t.Fatalf("failed batch acknowledged: %s %v %d", cursor, err, calls)
	}
	for _, input := range []string{
		strings.Replace(reset, `"project_id":"p"`, `"project_id":"foreign"`, 1),
		strings.Replace(reset, "id: epoch:10", "id: epoch:11", 1),
		strings.Replace(reset, `"reset":true`, `"reset":false`, 1),
		strings.TrimSuffix(reset, "\n\n"),
		"data: " + strings.Repeat("x", maxAgentFrame) + "\n\n",
		agentFrame(agentBatchFixture(11, false)),
	} {
		called := false
		_, err := readAgentEvents(strings.NewReader(input), "p", "r", "old:5", func(AgentEventBatch) error { called = true; return nil })
		if err == nil || called {
			t.Fatalf("invalid frame delivered: error=%v called=%v", err, called)
		}
	}
}

func TestAgentStreamCancellationClosesResponse(t *testing.T) {
	closed := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, agentFrame(agentBatchFixture(1, true)))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(closed)
	}))
	defer server.Close()
	cl, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	ready, done := make(chan struct{}), make(chan error, 1)
	go func() {
		_, err := cl.StreamAgentEvents(t.Context(), "stream", agentEventsTarget, "", func(AgentEventBatch) error { close(ready); return nil })
		done <- err
	}()
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("stream did not connect")
	}
	cl.Cancel("stream")
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancel error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("stream outlived cancellation")
	}
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("server response leaked")
	}
}

func TestAgentRoutesAndPrivateDetailContract(t *testing.T) {
	for _, path := range []string{"/agent-runs", "/agent-runs/events", "/agent-runs/run"} {
		if !allowedRequest("GET", "/api/projects/current"+path, "") {
			t.Fatal("history route rejected", path)
		}
	}
	for _, action := range []string{"control", "input"} {
		if !allowedRequest("POST", "/api/projects/current/agent-runs/run/"+action, `{}`) {
			t.Fatal("control route rejected", action)
		}
	}
	for _, path := range []string{"/agent-runs", "/agent-runs/run/retry", "/agent-runs/run/apply"} {
		if allowedRequest("POST", "/api/projects/current"+path, `{}`) {
			t.Fatal("unadmitted route allowed", path)
		}
	}
	var detail AgentRunDetail
	err := json.Unmarshal([]byte(`{"run":{"id":"run","root_id":"root","usage":{"input_tokens":9007199254740993},"input":{"request_id":"request"}},"invocations":[],"events":[],"input":{"revision":9007199254740993,"run_id":"run"},"text":{"available":true,"state":"final_model_output","text":"advisory","artifact":{"session":{"app":"mini-orca","user":"local","id":"session"},"name":"completion.json","version":1}}}`), &detail)
	if err != nil || detail.Input.Revision != 9007199254740993 || detail.Run.Usage.InputTokens != 9007199254740993 || detail.Text.Artifact.Version != 1 {
		t.Fatalf("detail contract: %+v %v", detail, err)
	}
}
