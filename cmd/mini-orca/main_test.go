package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
)

func TestProjectFolderDefaultsToTheCurrentFolder(t *testing.T) {
	folder := t.TempDir()
	t.Chdir(folder)
	canonical, _ := filepath.EvalSymlinks(folder)
	opts, err := parseOptions(nil, io.Discard)
	if err != nil || opts.project != canonical || !opts.open || opts.url != defaultDaemonURL {
		t.Fatalf("defaults: %+v %v", opts, err)
	}
	other := t.TempDir()
	if opts, err := parseOptions([]string{"--no-open", other}, io.Discard); err != nil || opts.open || filepath.Base(opts.project) != filepath.Base(other) {
		t.Fatalf("explicit folder: %+v %v", opts, err)
	}
	file := filepath.Join(folder, "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{file}, {filepath.Join(folder, "missing")}, {folder, other}} {
		if _, err := parseOptions(args, io.Discard); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestStartedDaemonPrefersTheProjectConfigThenTheInstalledOne(t *testing.T) {
	project, install := t.TempDir(), t.TempDir()
	binary := filepath.Join(install, daemonBinary)
	if got := daemonConfig(project, binary); got != "" {
		t.Fatalf("no config: %q", got)
	}
	installed := filepath.Join(install, "config.yaml")
	if err := os.WriteFile(installed, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := daemonConfig(project, binary); got != installed {
		t.Fatalf("installed config: %q", got)
	}
	local := filepath.Join(project, "config.yaml")
	if err := os.WriteFile(local, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := daemonConfig(project, binary); got != local {
		t.Fatalf("project config: %q", got)
	}
	t.Setenv("MINI_ORCA_CONFIG", "/elsewhere/config.yaml")
	if got := daemonConfig(project, binary); got != "" {
		t.Fatalf("explicit MINI_ORCA_CONFIG overridden: %q", got)
	}
}

// fakeDaemon answers like the daemon's project routes and records writes.
type fakeDaemon struct {
	mu      sync.Mutex
	current string
	writes  []string
	restore bool
}

func (f *fakeDaemon) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Method == http.MethodGet {
		_ = json.NewEncoder(w).Encode(map[string]string{"path": f.current})
		return
	}
	if r.Header.Get(api.WriteTokenHeader) != "launch-token" {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"user_message": "Reload the Web UI to get this daemon's write token."})
		return
	}
	var body struct {
		ProjectPath string `json:"project_path"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	f.writes = append(f.writes, r.URL.Path)
	if r.URL.Path == "/api/projects/restore" && !f.restore {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	f.current = body.ProjectPath
	_ = json.NewEncoder(w).Encode(map[string]string{"path": f.current})
}

func TestOpenProjectSwitchesARunningDaemonOnlyWhenNeeded(t *testing.T) {
	token := filepath.Join(t.TempDir(), "daemon-token")
	t.Setenv("MINI_ORCA_TOKEN_FILE", token)
	daemon := &fakeDaemon{current: "/projects/other"}
	server := httptest.NewServer(daemon)
	defer server.Close()
	client := newDaemonClient(server.URL + "/")
	folder := t.TempDir()
	if err := client.openProject(t.Context(), folder); err == nil || !strings.Contains(err.Error(), "write token") {
		t.Fatalf("missing token: %v", err)
	}
	if err := os.WriteFile(token, []byte("launch-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := client.openProject(t.Context(), folder); err != nil || daemon.current != folder || strings.Join(daemon.writes, ",") != "/api/projects/restore,/api/projects/import" {
		t.Fatalf("new project not imported: %v %+v", err, daemon)
	}
	daemon.writes, daemon.restore, daemon.current = nil, true, "/projects/other"
	if err := client.openProject(t.Context(), folder); err != nil || strings.Join(daemon.writes, ",") != "/api/projects/restore" {
		t.Fatalf("known project not restored: %v %+v", err, daemon.writes)
	}
	daemon.writes = nil
	if err := client.openProject(t.Context(), folder); err != nil || len(daemon.writes) != 0 {
		t.Fatalf("the current project was reopened: %v %+v", err, daemon.writes)
	}
}

func TestStartedDaemonThatExitsEarlyIsReported(t *testing.T) {
	daemon := &ownedDaemon{done: make(chan struct{}), err: os.ErrProcessDone}
	close(daemon.done)
	client := newDaemonClient("http://127.0.0.1:1")
	if err := client.waitHealthy(t.Context(), daemon); err == nil || !strings.Contains(err.Error(), "exited during startup") {
		t.Fatalf("early exit: %v", err)
	}
	daemon.stop() // An exited daemon needs no signal and must not block.
}
