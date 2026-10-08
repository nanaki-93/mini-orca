package bridge

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type requestTransport func(*http.Request) (*http.Response, error)

func (transport requestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestFeatureGenerationDoesNotShortenTheWorkflowDeadline(t *testing.T) {
	for _, allowance := range []time.Duration{0, time.Minute, 10 * time.Minute, 20 * time.Minute} {
		t.Run(allowance.String(), func(t *testing.T) {
			ctx := context.Background()
			if allowance > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, allowance)
				defer cancel()
			}
			client, err := NewClient("http://127.0.0.1:9090")
			if err != nil {
				t.Fatal(err)
			}
			client.http.Transport = requestTransport(func(request *http.Request) (*http.Response, error) {
				deadline, bounded := request.Context().Deadline()
				if request.URL.Path == "/api/projects/current/features/generate" {
					want, supplied := ctx.Deadline()
					if bounded != supplied || bounded && !deadline.Equal(want) {
						t.Errorf("feature transport deadline = %v (bounded %t), caller = %v (bounded %t)", deadline, bounded, want, supplied)
					}
				} else if !bounded || time.Until(deadline) > client.http.Timeout {
					t.Errorf("ordinary request lost its transport timeout: %v", deadline)
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"status":"ready"}`))}, nil
			})
			for _, target := range []string{"/api/projects/current/features/generate", "/api/projects/current/features/generate?request=1"} {
				result, err := client.Request(ctx, "feature", http.MethodPost, target, "{}")
				if err != nil || result.Status != http.StatusOK {
					t.Fatalf("feature generation = %+v, %v", result, err)
				}
			}
			if _, err := client.Request(context.Background(), "status", http.MethodGet, "/status", ""); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFeatureGenerationRemainsCancelable(t *testing.T) {
	for _, action := range []string{"request", "parent", "close"} {
		t.Run(action, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			client, err := NewClient("http://127.0.0.1:9090")
			if err != nil {
				t.Fatal(err)
			}
			entered := make(chan struct{})
			client.http.Transport = requestTransport(func(request *http.Request) (*http.Response, error) {
				close(entered)
				<-request.Context().Done()
				return nil, request.Context().Err()
			})
			done := make(chan error, 1)
			go func() {
				_, err := client.Request(ctx, "feature", http.MethodPost, "/api/projects/current/features/generate", "{}")
				done <- err
			}()
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("feature request did not reach transport")
			}
			switch action {
			case "request":
				client.Cancel("feature")
			case "parent":
				cancel()
			case "close":
				if err := client.Close(); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("feature cancellation = %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("feature request ignored cancellation")
			}
		})
	}
}

func TestNativeRequestsPreserveDaemonContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != "" || r.Header.Get("Accept") != "application/json" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("native transport changed local request headers")
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"project_revision":"r1","confirm":true}` || r.URL.Path != "/api/projects/current/execution-trust" {
			t.Error("guarded request body was changed")
		}
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"type":"conflict","user_message":"Refresh project"}`))
	}))
	defer server.Close()
	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Request(context.Background(), "1", "POST", "/api/projects/current/execution-trust", `{"project_revision":"r1","confirm":true}`)
	if err != nil || result.Status != 409 || !strings.Contains(result.Body, "Refresh project") {
		t.Fatalf("structured failure was lost: %#v, %v", result, err)
	}
}

func TestNativeModelCatalogReadsStayOnTheDaemon(t *testing.T) {
	client, err := NewClient("http://127.0.0.1:9090")
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	client.http.Transport = requestTransport(func(request *http.Request) (*http.Response, error) {
		calls++
		if request.Method != http.MethodGet || request.URL.Path != "/api/models/available" || request.Header.Get("Origin") != "" {
			t.Fatalf("catalog request changed: %s %s", request.Method, request.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"models":[],"defaults":{},"pi":{"status":"ready"}}`))}, nil
	})
	if result, err := client.Request(context.Background(), "catalog", http.MethodGet, "/api/models/available", ""); err != nil || result.Status != http.StatusOK {
		t.Fatalf("catalog response = %+v, %v", result, err)
	}
	if _, err := client.Request(context.Background(), "invalid", http.MethodPost, "/api/models/available", "{}"); err == nil || calls != 1 {
		t.Fatal("catalog endpoint permitted a write")
	}
}

func TestBridgeRejectsUnboundedAndForeignRequests(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); w.WriteHeader(204) }))
	defer server.Close()
	client, _ := NewClient(server.URL)
	for _, request := range [][3]string{
		{"GET", "https://example.com/status", ""}, {"GET", "//example.com/status", ""}, {"POST", "/run-command", "{}"},
		{"GET", "/api/projects/current/files/%2e%2e/info", ""}, {"POST", "/api/projects/current/apply", "not json"},
		{"GET", "/status", "{}"}, {"OPTIONS", "/status", ""}, {"POST", "/api/projects/import", `"` + strings.Repeat("x", 2<<20) + `"`},
	} {
		if _, err := client.Request(context.Background(), "bad", request[0], request[1], request[2]); err == nil {
			t.Errorf("accepted %s %s", request[0], request[1])
		}
	}
	if calls.Load() != 0 {
		t.Fatal("rejected requests reached the network")
	}
	for _, origin := range []string{"file:///etc/passwd", "http://user:password@localhost", "http://localhost/path", "http://localhost?query=1"} {
		if _, err := NewClient(origin); err == nil {
			t.Errorf("accepted invalid origin %s", origin)
		}
	}
}

func TestBridgeCancelsRequestAndRejectsDuplicateIdentity(t *testing.T) {
	entered := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { close(entered); <-r.Context().Done() }))
	defer server.Close()
	client, _ := NewClient(server.URL)
	done := make(chan error, 1)
	go func() { _, err := client.Request(context.Background(), "request", "GET", "/status", ""); done <- err }()
	<-entered
	if _, err := client.Request(context.Background(), "request", "GET", "/status", ""); err == nil {
		t.Fatal("duplicate request accepted")
	}
	client.Cancel("request")
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation lost: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("native cancellation did not reach transport")
	}
}

func TestBridgeDoesNotFollowRedirects(t *testing.T) {
	var reached atomic.Bool
	other := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached.Store(true) }))
	defer other.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	client, _ := NewClient(server.URL)
	for _, request := range [][3]string{
		{http.MethodGet, "/status", ""},
		{http.MethodPost, "/api/projects/current/features/generate", "{}"},
	} {
		if _, err := client.Request(context.Background(), "1", request[0], request[1], request[2]); err == nil || reached.Load() {
			t.Fatalf("redirect escaped daemon origin for %s", request[1])
		}
	}
}

func TestTerminalRequiresActivatedProject(t *testing.T) {
	client, _ := NewClient("http://127.0.0.1:9090")
	if _, err := client.OpenTerminal(t.TempDir(), 80, 24); err == nil {
		t.Fatal("terminal started without an activated project")
	}
}

func TestClosedClientRejectsNewWork(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer server.Close()
	client, _ := NewClient(server.URL)
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Request(context.Background(), "late", "GET", "/status", ""); err == nil {
		t.Fatal("request dispatched after shutdown")
	}
	if _, err := client.OpenTerminal(t.TempDir(), 80, 24); err == nil {
		t.Fatal("shell started after shutdown")
	}
	if requests.Load() != 0 {
		t.Fatal("closed client reached daemon")
	}
}

func TestCancellationBeforeDispatchDoesNotStartNetworkWork(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer server.Close()
	client, _ := NewClient(server.URL)
	client.Cancel("early")
	_, err := client.Request(context.Background(), "early", "GET", "/status", "")
	if !errors.Is(err, context.Canceled) || calls.Load() != 0 {
		t.Fatalf("early cancellation dispatched work: %v (%d calls)", err, calls.Load())
	}
}

func TestBridgeSharedChangeRoutesRemainAllowlisted(t *testing.T) {
	for _, request := range [][3]string{
		{"GET", "/api/projects/current/changes?project_id=p&project_revision=r", ""},
		{"GET", "/api/projects/current/changes/recovery", ""},
		{"GET", "/api/projects/current/changes/change-id", ""},
		{"GET", "/api/projects/current/instructions?path=AGENTS.md", ""},
		{"GET", "/api/projects/current/features", ""},
		{"POST", "/api/projects/current/changes", "{}"},
		{"POST", "/api/projects/current/changes/change-id/messages", "{}"},
		{"POST", "/api/projects/current/changes/change-id/apply", "{}"},
		{"POST", "/api/projects/current/changes/change-id/verify", "{}"},
		{"POST", "/api/projects/current/changes/change-id/workflow", "{}"},
		{"POST", "/api/projects/current/changes/change-id/workflow/cancel", "{}"},
		{"POST", "/api/projects/current/instructions/proposal", "{}"},
		{"POST", "/api/projects/current/features/generate", "{}"},
		{"PATCH", "/api/projects/current/features/idea", "{}"},
	} {
		if !allowedRequest(request[0], request[1], request[2]) {
			t.Errorf("blocked workflow request: %v", request)
		}
	}
	for _, path := range []string{"/api/projects/current/changes/id/run", "/api/projects/current/instructions/write", "/api/projects/current/changes/id/delete"} {
		if allowedRequest("POST", path, "{}") {
			t.Errorf("allowed unsupported operation: %s", path)
		}
	}
}
