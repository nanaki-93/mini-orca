package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestRunOnceRendersSyncedWorkspace(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"version":"2.4.0"}`))
	})
	mux.HandleFunc("/api/projects/restore", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"project_id":"p1","project_revision":"r1","name":"mini-orca","path":"/tmp/mini-orca"}`))
	})
	mux.HandleFunc("/api/projects/current/index", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"project_id":"p1","project_revision":"r1","files":[{"path":"cmd/tui/main.go","language":"Go"}]}`))
	})
	mux.HandleFunc("/api/projects/current/overview", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"project_id":"p1","project_revision":"r1","analysis":{"purpose":"TUI test"}}`))
	})
	mux.HandleFunc("/api/projects/current/findings", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"project_id":"p1","project_revision":"r1","findings":[]}`))
	})
	mux.HandleFunc("/api/projects/current/git/workspace", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"available":true,"branch":"main","worktrees":[]}`))
	})
	mux.HandleFunc("/api/projects/current/execution-trust", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"project_id":"p1","project_revision":"r1","trusted":true}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	err := run(
		context.Background(),
		[]string{"-url", server.URL, "-project", "/tmp/mini-orca", "-palette", "amber", "-cmd", ":arch", "-once"},
		strings.NewReader(""),
		&out,
	)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	rendered := out.String()
	if !strings.Contains(rendered, "MINI-ORCA") || !strings.Contains(rendered, "ARCHITECTURE DIAGRAM") {
		t.Fatalf("unexpected rendered output:\n%s", rendered)
	}
}

func TestRunRejectsInvalidDaemonURL(t *testing.T) {
	var out bytes.Buffer
	err := run(context.Background(), []string{"-url", "ftp://bad", "-once"}, strings.NewReader(""), &out)
	if err == nil {
		t.Fatal("expected invalid daemon URL to fail")
	}
}

func TestStopChildDaemonGracefulAndFallback(t *testing.T) {
	stopChildDaemon(nil, 50*time.Millisecond)

	cmd := exec.Command("sleep", "10")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start child process: %v", err)
	}
	start := time.Now()
	stopChildDaemon(cmd, 5*time.Second)
	if elapsed := time.Since(start); elapsed >= 2*time.Second {
		t.Fatalf("expected child process to stop promptly on SIGTERM, took %v", elapsed)
	}
	if cmd.ProcessState == nil {
		t.Fatal("expected ProcessState to be populated after Wait")
	}
}
