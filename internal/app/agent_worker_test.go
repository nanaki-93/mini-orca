package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

const workerFixtureSource = "package sample\nfunc Sum(a, b int) int { return a - b }\n"

func workerFixture(t *testing.T, endpoint string) (*config.Config, AgentWorkerOptions, AgentWorkerRequest) {
	t.Helper()
	t.Setenv("MINI_ORCA_WORKER_FIXTURE", serviceFixtureToken)
	p := config.ModelProfileConfig{Provider: config.GeminiProvider, APIBaseURL: endpoint, APIKey: "PRIVATE_KEY", Model: "fixture"}
	cfg := &config.Config{ModelScopes: config.ModelScopesConfig{Analyze: p, Bug: p, Function: p}, Integrations: config.IntegrationsConfig{A2AServer: config.AgentServer{Enabled: true, Principals: []config.AgentPrincipal{{ID: "worker", BearerTokenEnv: "MINI_ORCA_WORKER_FIXTURE", ProjectID: "project", Capabilities: []string{"analysis"}}}}}}
	o := AgentWorkerOptions{Scope: AgentWorkerScope{ProjectID: "project", ProjectRevision: "revision", Files: []ExternalSource{{Path: "sample.go", Hash: contentHash([]byte(workerFixtureSource))}}}, Endpoint: "http://127.0.0.1:9091/a2a", Lifetime: time.Minute, MaxJobs: 2, ConfirmSourceSharing: true}
	r := AgentWorkerRequest{ProjectID: "project", ProjectRevision: "revision", Question: "Inspect Sum", Files: map[string]string{"sample.go": workerFixtureSource}, ConfirmSourceSharing: true}
	return cfg, o, r
}

func workerJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func workerAnswer() AgentWorkerAnswer {
	return AgentWorkerAnswer{Summary: "Sum subtracts; check the intended arithmetic.", Evidence: []PlanEvidence{{Path: "sample.go", Hash: contentHash([]byte(workerFixtureSource)), StartLine: 2, EndLine: 2, Quote: "return a - b", Observation: "The implementation subtracts its arguments."}}}
}

func closeWorkerFixture(t *testing.T, worker *AgentWorker) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := worker.Close(ctx); err != nil {
		t.Error(err)
	}
}

func TestAgentWorkerNativeToolsReturnOnlyValidatedAdvisoryEvidence(t *testing.T) {
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		body := string(data)
		var part any
		if calls.Add(1) == 1 {
			if strings.Contains(body, workerFixtureSource) || !strings.Contains(body, "read_snippet") {
				t.Error("source bypassed tools or tools missing")
			}
			part = map[string]any{"functionCall": map[string]any{"id": "read", "name": "read_snippet", "args": map[string]any{"path": "sample.go", "start_line": 2, "end_line": 2}}}
		} else {
			if !strings.Contains(body, "functionResponse") || !strings.Contains(body, "return a - b") {
				t.Error("captured tool evidence missing")
			}
			part = map[string]any{"text": workerJSON(t, workerAnswer())}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"finishReason": "STOP", "content": map[string]any{"role": "model", "parts": []any{part}}}}, "usageMetadata": map[string]any{"promptTokenCount": 100, "candidatesTokenCount": 20, "totalTokenCount": 120}})
	}))
	defer provider.Close()
	cfg, options, request := workerFixture(t, provider.URL)
	worker, err := NewAgentWorker(t.Context(), cfg, options)
	if err != nil {
		t.Fatal(err)
	}
	defer closeWorkerFixture(t, worker)
	admission, err := worker.authenticate(t.Context(), serviceFixtureToken)
	if err != nil {
		t.Fatal(err)
	}
	text, err := admission.Execute(t.Context(), workerJSON(t, request))
	var result AgentWorkerResult
	if err != nil || json.Unmarshal([]byte(text), &result) != nil {
		t.Fatal(text, err)
	}
	if !result.Advisory || result.Checks != "not_run" || result.ProjectRevision != options.Scope.ProjectRevision || result.Usage.ModelCalls != 2 || result.Usage.ToolCalls != 1 || calls.Load() != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if _, err := worker.authenticate(t.Context(), "other-token"); err == nil {
		t.Fatal("foreign principal admitted")
	}
	t.Setenv("MINI_ORCA_WORKER_FIXTURE", strings.Repeat("z", 32))
	if err := admission.Validate(t.Context()); err == nil {
		t.Fatal("rotated credential retained access")
	}
}

func TestAgentWorkerRejectsScopeConsentAndMalformedRequestsWithoutDispatch(t *testing.T) {
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer provider.Close()
	cfg, o, base := workerFixture(t, provider.URL)
	worker, err := NewAgentWorker(t.Context(), cfg, o)
	if err != nil {
		t.Fatal(err)
	}
	defer closeWorkerFixture(t, worker)
	a, err := worker.authenticate(t.Context(), serviceFixtureToken)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"project", "revision", "consent", "hash", "path", "oversize", "question", "unknown", "duplicate", "trailing"} {
		t.Run(kind, func(t *testing.T) {
			r := base
			r.Files = map[string]string{"sample.go": workerFixtureSource}
			switch kind {
			case "project":
				r.ProjectID = "foreign"
			case "revision":
				r.ProjectRevision = "stale"
			case "consent":
				r.ConfirmSourceSharing = false
			case "hash":
				r.Files["sample.go"] += "changed"
			case "path":
				r.Files = map[string]string{"../private.env": workerFixtureSource}
			case "oversize":
				r.Files["sample.go"] = strings.Repeat("x", 33<<10)
			case "question":
				r.Question = ""
			}
			text := workerJSON(t, r)
			switch kind {
			case "unknown":
				text = strings.TrimSuffix(text, "}") + `,"unknown":true}`
			case "duplicate":
				text = strings.TrimSuffix(text, "}") + `,"project_id":"project"}`
			case "trailing":
				text += `{}`
			}
			if _, err := a.Execute(t.Context(), text); err == nil {
				t.Fatal("invalid scope admitted")
			}
		})
	}
	if calls.Load() != 0 || worker.jobs.Load() != 0 {
		t.Fatal("rejected request spent budget")
	}
	for _, kind := range []string{"remote", "source", "lifetime", "jobs", "capability", "principal", "path", "hash", "endpoint"} {
		t.Run("startup-"+kind, func(t *testing.T) {
			cfg, o, _ := workerFixture(t, provider.URL)
			switch kind {
			case "remote":
				cfg.ModelScopes.Function.APIBaseURL = "https://provider.example"
			case "source":
				o.ConfirmSourceSharing = false
			case "lifetime":
				o.Lifetime = 16 * time.Minute
			case "jobs":
				o.MaxJobs = 129
			case "capability":
				cfg.Integrations.A2AServer.Principals[0].Capabilities = []string{"proposal"}
			case "principal":
				cfg.Integrations.A2AServer.Principals[0].ProjectID = "other"
			case "path":
				o.Scope.Files[0].Path = "../private"
			case "hash":
				o.Scope.Files[0].Hash = strings.Repeat("a", 64)
			case "endpoint":
				o.Endpoint = "http://remote.example/a2a"
			}
			if w, err := NewAgentWorker(t.Context(), cfg, o); err == nil {
				closeWorkerFixture(t, w)
				t.Fatal("invalid startup admitted")
			}
		})
	}
}

func TestAgentWorkerRejectsFabricatedEvidenceAndBoundsJobs(t *testing.T) {
	for _, kind := range []string{"hash", "path", "line", "quote", "fields"} {
		answer := workerAnswer()
		switch kind {
		case "hash":
			answer.Evidence[0].Hash = "wrong"
		case "path":
			answer.Evidence[0].Path = "other.go"
		case "line":
			answer.Evidence[0].EndLine = 99
		case "quote":
			answer.Evidence[0].Quote = "invented"
		case "fields":
			answer.Summary = ""
		}
		if _, err := decodeWorkerAnswer(workerJSON(t, answer), map[string]string{"sample.go": workerFixtureSource}); err == nil {
			t.Fatal("accepted", kind)
		}
	}
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		http.Error(w, "private provider failure", http.StatusServiceUnavailable)
	}))
	defer provider.Close()
	cfg, o, request := workerFixture(t, provider.URL)
	o.MaxJobs = 1
	worker, err := NewAgentWorker(t.Context(), cfg, o)
	if err != nil {
		t.Fatal(err)
	}
	defer closeWorkerFixture(t, worker)
	a, err := worker.authenticate(t.Context(), serviceFixtureToken)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := a.Execute(t.Context(), workerJSON(t, request)); err == nil {
			t.Fatal("failed provider became success")
		}
	}
	if calls.Load() != 1 {
		t.Fatal("failed operation replayed", calls.Load())
	}
}

func TestAgentWorkerHTTPShutdownCancelsAndJoinsOwnedModelCall(t *testing.T) {
	started, canceled := make(chan struct{}), make(chan struct{})
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(started)
		<-r.Context().Done()
		close(canceled)
	}))
	defer provider.Close()
	cfg, options, request := workerFixture(t, provider.URL)
	worker, err := NewAgentWorker(t.Context(), cfg, options)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(worker)
	defer server.Close()
	defer closeWorkerFixture(t, worker)
	message := map[string]any{"jsonrpc": "2.0", "id": "job", "method": "SendMessage", "params": map[string]any{"message": map[string]any{"messageId": "message", "role": "ROLE_USER", "parts": []any{map[string]any{"text": workerJSON(t, request)}}}}}
	r, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/a2a", strings.NewReader(workerJSON(t, message)))
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Authorization", "Bearer "+serviceFixtureToken)
	r.Header.Set("Content-Type", "application/json")
	finished := make(chan error, 1)
	go func() {
		response, err := server.Client().Do(r)
		if err == nil {
			_, err = io.Copy(io.Discard, response.Body)
			response.Body.Close()
		}
		finished <- err
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("model call not started")
	}
	closeWorkerFixture(t, worker)
	select {
	case <-canceled:
	case <-time.After(3 * time.Second):
		t.Fatal("provider retained canceled call")
	}
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("HTTP request not joined")
	}
	if _, err := worker.authenticate(t.Context(), serviceFixtureToken); err == nil {
		t.Fatal("closed worker admits work")
	}
}
