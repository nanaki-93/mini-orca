package agentruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
)

func localAgentFixture(t *testing.T, execute func(context.Context, string) (string, error)) (*AgentServer, *httptest.Server) {
	t.Helper()
	s, err := NewAgentServer(AgentServerConfig{Endpoint: "http://127.0.0.1:9091/a2a", Version: "fixture", Timeout: 5 * time.Second, MaxConcurrent: 1},
		func(_ context.Context, token string) (ServerAdmission, error) {
			if token != "first" && token != "second" {
				return ServerAdmission{}, errors.New("denied")
			}
			return ServerAdmission{Owner: token, ExpiresAt: time.Now().Add(time.Minute), Validate: func(ctx context.Context) error { return context.Cause(ctx) }, Execute: execute}, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(s)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := s.Close(ctx); err != nil {
			t.Error(err)
		}
		httpServer.Close()
	})
	return s, httpServer
}

func localAgentRPC(t *testing.T, server *httptest.Server, token, method string, params any) (int, string) {
	t.Helper()
	data, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": "request", "method": method, "params": params})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/a2a", strings.NewReader(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, string(body)
}

func localAgentMessage(id string) *a2a.SendMessageRequest {
	return &a2a.SendMessageRequest{Message: &a2a.Message{ID: id, Role: a2a.MessageRoleUser, Parts: a2a.ContentParts{a2a.NewTextPart(`{"operation":"analysis"}`)}}}
}

func TestAgentServerWorkerPayloadDoesNotRelaxDaemonAdmission(t *testing.T) {
	var calls atomic.Int32
	admit := func(_ context.Context, token string) (ServerAdmission, error) {
		return ServerAdmission{Owner: token, ExpiresAt: time.Now().Add(time.Minute), Validate: func(ctx context.Context) error { return context.Cause(ctx) }, Execute: func(_ context.Context, text string) (string, error) { calls.Add(1); return "advisory", nil }}, nil
	}
	for _, inputLimit := range []int{0, 128 << 10} {
		cfg := AgentServerConfig{Endpoint: "http://127.0.0.1:9091/a2a", Version: "fixture", Timeout: time.Second, MaxConcurrent: 1, InputBytes: inputLimit, AnalysisOnly: inputLimit != 0}
		native, err := NewAgentServer(cfg, admit)
		if err != nil {
			t.Fatal(err)
		}
		server := httptest.NewServer(native)
		message := localAgentMessage("large")
		message.Message.Parts = a2a.ContentParts{a2a.NewTextPart(strings.Repeat("x", 9<<10))}
		before := calls.Load()
		_, body := localAgentRPC(t, server, "owner", "SendMessage", message)
		if inputLimit == 0 && calls.Load() != before {
			t.Fatal("daemon payload limit expanded", body)
		}
		if inputLimit != 0 && (calls.Load() != before+1 || !strings.Contains(body, "advisory")) {
			t.Fatal("worker payload rejected", body)
		}
		card := serverCard(cfg)
		if (inputLimit == 0 && len(card.Skills) != 2) || (inputLimit != 0 && (len(card.Skills) != 1 || card.Skills[0].ID != "analysis")) {
			t.Fatal("incorrect capabilities", card)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		if err := native.Close(ctx); err != nil {
			t.Error(err)
		}
		cancel()
		server.Close()
	}
	for _, limit := range []int{-1, 8191, (128 << 10) + 1} {
		if _, err := NewAgentServer(AgentServerConfig{Timeout: time.Second, MaxConcurrent: 1, InputBytes: limit}, admit); err == nil {
			t.Fatal("unbounded input admitted", limit)
		}
	}
}

func TestAgentServerNativeExecutionAndPrivateTaskOwnership(t *testing.T) {
	var calls atomic.Int32
	_, server := localAgentFixture(t, func(_ context.Context, _ string) (string, error) {
		calls.Add(1)
		return `{"advisory":true,"result":"private evidence"}`, nil
	})
	response, err := server.Client().Get(server.URL + "/.well-known/agent-card.json")
	if err != nil {
		t.Fatal(err)
	}
	var card a2a.AgentCard
	err = json.NewDecoder(response.Body).Decode(&card)
	response.Body.Close()
	if err != nil || len(card.Skills) != 2 || calls.Load() != 0 {
		t.Fatalf("passive card: %+v %v", card, err)
	}
	status, body := localAgentRPC(t, server, "invalid", "SendMessage", localAgentMessage("new"))
	if status != 401 || calls.Load() != 0 {
		t.Fatal("unauthenticated dispatch", status, body)
	}
	status, body = localAgentRPC(t, server, "first", "SendMessage", localAgentMessage("new"))
	var result struct {
		Result struct {
			Task *a2a.Task `json:"task"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatal(err)
	}
	if status != 200 || result.Result.Task == nil || result.Result.Task.Status.State != a2a.TaskStateCompleted || calls.Load() != 1 {
		t.Fatal("native execution", status, body)
	}
	id := result.Result.Task.ID
	for _, method := range []string{"GetTask", "CancelTask", "SubscribeToTask"} {
		_, body := localAgentRPC(t, server, "second", method, map[string]string{"id": string(id)})
		if !strings.Contains(body, "task not found") || strings.Contains(body, "private evidence") {
			t.Fatalf("%s crossed owners: %s", method, body)
		}
	}
	_, body = localAgentRPC(t, server, "first", "GetTask", map[string]string{"id": string(id)})
	if !strings.Contains(body, "private evidence") {
		t.Fatal("owner cannot read retained artifact", body)
	}
	_, body = localAgentRPC(t, server, "first", "SendMessage", localAgentMessage("new"))
	if calls.Load() != 1 || !strings.Contains(body, "not supported") {
		t.Fatal("message replayed", body)
	}
}

func TestAgentServerRejectsUnsupportedAndAmbiguousRequests(t *testing.T) {
	var calls atomic.Int32
	s, _ := localAgentFixture(t, func(context.Context, string) (string, error) { calls.Add(1); return "unused", nil })
	valid := `{"jsonrpc":"2.0","id":"request","method":"SendMessage","params":{"message":{"messageId":"m","role":"ROLE_USER","parts":[{"text":"hello"}]}}}`
	for _, body := range []string{
		valid + `{}`, strings.Replace(valid, `"id":"request"`, `"id":"request","id":"other"`, 1),
		strings.Replace(valid, `"role":"ROLE_USER"`, `"role":"ROLE_USER","contextId":"other-session"`, 1),
		strings.Replace(valid, `"text":"hello"`, `"data":{"source":"untrusted"}`, 1),
		strings.Replace(valid, `"params":{"message"`, `"params":{"metadata":{"adk_user_id":"other"},"message"`, 1),
		strings.Replace(valid, `SendMessage`, `ListTasks`, 1), strings.Repeat(" ", 16<<10) + valid,
	} {
		request := httptest.NewRequest("POST", "/a2a", strings.NewReader(body))
		request.Header.Set("Authorization", "Bearer first")
		response := httptest.NewRecorder()
		s.ServeHTTP(response, request)
		if response.Code != 400 || calls.Load() != 0 {
			t.Fatal("invalid request dispatched", response.Code, response.Body.String())
		}
	}
	for i := 0; i < 128; i++ {
		if err := s.reserveMessage(fmt.Sprint("reserved-", i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.reserveMessage("overflow"); err == nil {
		t.Fatal("unbounded task retention")
	}
}

type agentServerFixtureTransport struct{ base http.RoundTripper }

func (t agentServerFixtureTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	request = request.Clone(request.Context())
	request.Header.Set("Authorization", "Bearer first")
	return t.base.RoundTrip(request)
}

func TestAgentServerNativeClientAndSanitizedFailures(t *testing.T) {
	var output atomic.Value
	output.Store("ok")
	_, server := localAgentFixture(t, func(context.Context, string) (string, error) {
		if output.Load().(string) == "fail" {
			return "", errors.New("private upstream diagnostic with token=secret")
		}
		return output.Load().(string), nil
	})
	card := serverCard(AgentServerConfig{Endpoint: server.URL + "/a2a", Version: "fixture"})
	httpClient := &http.Client{Transport: agentServerFixtureTransport{base: server.Client().Transport}}
	native, err := a2aclient.NewFromCard(t.Context(), card, a2aclient.WithDefaultsDisabled(), a2aclient.WithJSONRPCTransport(httpClient))
	if err != nil {
		t.Fatal(err)
	}
	defer native.Destroy()
	result, err := native.SendMessage(t.Context(), localAgentMessage("native"))
	task, ok := result.(*a2a.Task)
	if err != nil || !ok || task.Status.State != a2a.TaskStateCompleted {
		t.Fatalf("native client interoperability: %+v %v", result, err)
	}
	for _, value := range []string{"fail", strings.Repeat("x", (16<<10)+1)} {
		output.Store(value)
		_, body := localAgentRPC(t, server, "first", "SendMessage", localAgentMessage(fmt.Sprint("failure-", len(value))))
		if !strings.Contains(body, "TASK_STATE_FAILED") || strings.Contains(body, "token=secret") || strings.Contains(body, strings.Repeat("x", 1024)) {
			t.Fatal("failure contract", body)
		}
	}
}

func TestAgentServerCancellationDisconnectAndShutdownJoinExecution(t *testing.T) {
	for _, action := range []string{"cancel", "disconnect", "shutdown"} {
		t.Run(action, func(t *testing.T) {
			started, stopped := make(chan struct{}), make(chan struct{})
			s, server := localAgentFixture(t, func(ctx context.Context, _ string) (string, error) {
				close(started)
				<-ctx.Done()
				close(stopped)
				return "", context.Cause(ctx)
			})
			requestCtx, cancel := context.WithCancel(t.Context())
			defer cancel()
			data, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": "stream", "method": "SendStreamingMessage", "params": localAgentMessage("stream-message")})
			request, _ := http.NewRequestWithContext(requestCtx, "POST", server.URL+"/a2a", strings.NewReader(string(data)))
			request.Header.Set("Authorization", "Bearer first")
			request.Header.Set("Content-Type", "application/json")
			response, err := server.Client().Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			select {
			case <-started:
			case <-time.After(3 * time.Second):
				t.Fatal("agent not started")
			}
			s.mu.Lock()
			var id a2a.TaskID
			for task := range s.active {
				id = task
			}
			s.mu.Unlock()
			_, limited := localAgentRPC(t, server, "first", "SendMessage", localAgentMessage("concurrent"))
			if !strings.Contains(limited, `"error"`) {
				t.Fatal("concurrency limit ignored", limited)
			}
			_, other := localAgentRPC(t, server, "second", "SubscribeToTask", map[string]string{"id": string(id)})
			if !strings.Contains(other, "task not found") {
				t.Fatal("active subscription crossed owners", other)
			}
			switch action {
			case "cancel":
				_, body := localAgentRPC(t, server, "first", "CancelTask", map[string]string{"id": string(id)})
				if !strings.Contains(body, "TASK_STATE_CANCELED") {
					t.Fatal("cancel did not acknowledge", body)
				}
			case "disconnect":
				cancel()
				response.Body.Close()
			case "shutdown":
				ctx, stop := context.WithTimeout(t.Context(), 3*time.Second)
				defer stop()
				if err := s.Close(ctx); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case <-stopped:
			case <-time.After(3 * time.Second):
				t.Fatal("provider execution leaked")
			}
		})
	}
}
