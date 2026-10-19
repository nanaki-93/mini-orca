package tuiclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClientValidatesOrigin(t *testing.T) {
	invalid := []string{
		"",
		"ftp://127.0.0.1:9090",
		"http://user:pass@127.0.0.1:9090",
		"http://127.0.0.1:9090/api",
		"http://127.0.0.1:9090?foo=bar",
		"http://127.0.0.1:9090#frag",
	}
	for _, origin := range invalid {
		if _, err := NewClient(origin); err == nil {
			t.Fatalf("expected invalid origin %q to fail", origin)
		}
	}
	if _, err := NewClient("http://127.0.0.1:9090/"); err != nil {
		t.Fatalf("expected valid origin to succeed: %v", err)
	}
}

func TestClientRejectsUnsupportedRoutesAndPayloads(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	cases := []struct {
		name   string
		id     string
		method string
		target string
		body   string
	}{
		{name: "empty id", id: "", method: http.MethodGet, target: "/status"},
		{name: "unknown route", id: "req-1", method: http.MethodGet, target: "/admin"},
		{name: "traversal", id: "req-2", method: http.MethodGet, target: "/api/projects/../status"},
		{name: "get with body", id: "req-3", method: http.MethodGet, target: "/status", body: `{}`},
		{name: "invalid json", id: "req-4", method: http.MethodPost, target: "/api/projects/restore", body: `{bad`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := client.Request(context.Background(), tc.id, tc.method, tc.target, tc.body); err == nil {
				t.Fatalf("expected %s to be rejected", tc.name)
			}
		})
	}
}

func TestClientCancellationAndTombstone(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		w.WriteHeader(http.StatusRequestTimeout)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	// Pre-cancel before request starts (tombstone path)
	client.Cancel("pre-canceled")
	if _, err := client.Request(context.Background(), "pre-canceled", http.MethodGet, "/status", ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled for tombstoned request, got %v", err)
	}

	// Cancel in-flight request
	done := make(chan error, 1)
	go func() {
		_, reqErr := client.Request(context.Background(), "in-flight", http.MethodGet, "/status", "")
		done <- reqErr
	}()
	<-started
	client.Cancel("in-flight")
	select {
	case reqErr := <-done:
		if reqErr == nil {
			t.Fatal("expected canceled in-flight request to return error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for canceled request")
	}
}

func TestClientTracksProjectRootAndDecodesJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/projects/restore":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"project_id":"p1","project_revision":"rev1","name":"demo","path":"/tmp/demo"}`))
		case "/api/projects/current":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"message":"project is stale","details":{"hint":"Reopen the project."}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	var proj Project
	err = client.DoJSON(context.Background(), "restore-1", http.MethodPost, "/api/projects/restore", map[string]string{"project_path": "/tmp/demo"}, &proj)
	if err != nil {
		t.Fatalf("DoJSON restore failed: %v", err)
	}
	if proj.Name != "demo" || client.Root() != "/tmp/demo" {
		t.Fatalf("unexpected project %+v or root %q", proj, client.Root())
	}

	err = client.DoJSON(context.Background(), "curr-1", http.MethodGet, "/api/projects/current", nil, &proj)
	if err == nil || !strings.Contains(err.Error(), "project is stale — Reopen the project.") {
		t.Fatalf("expected structured daemon error with hint, got %v", err)
	}

	gotDetail := DecodeDaemonError(http.StatusBadRequest, `{"type":"bad_request","message":"AI workflow request failed","user_message":"Review the request and try again.","detail":"include a test path in the captured scope"}`)
	if gotDetail != "AI workflow request failed — include a test path in the captured scope" {
		t.Fatalf("unexpected AppError detail decode: %q", gotDetail)
	}

	if !allowedRequest(http.MethodPost, "/api/projects/current/bootstrap/exec", `{"project_revision":"r1","confirm":true}`) {
		t.Fatal("expected bootstrap/exec route to be allowed")
	}
	if !allowedRequest(http.MethodGet, "/api/projects/current/events", "") {
		t.Fatal("expected events route to be allowed")
	}
}
