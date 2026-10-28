package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

const (
	defaultDaemonURL = "http://127.0.0.1:9090"
	daemonBinary     = "mini-orca-daemon"
	// Startup indexes the project before the daemon listens; large projects
	// take a while.
	startupTimeout = 2 * time.Minute
	stopGrace      = 20 * time.Second
)

// projectFolder resolves the folder to open: the argument, or the current one.
func projectFolder(argument string) (string, error) {
	folder := argument
	if folder == "" {
		current, err := os.Getwd()
		if err != nil {
			return "", err
		}
		folder = current
	}
	absolute, err := filepath.Abs(folder)
	if err != nil {
		return "", err
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", folder, err)
	}
	if info, err := os.Stat(canonical); err != nil || !info.IsDir() {
		return "", fmt.Errorf("%s is not a folder", folder)
	}
	return canonical, nil
}

type ownedDaemon struct {
	cmd *exec.Cmd
	// done closes when the process exits; err is then its exit result.
	done chan struct{}
	err  error
}

// startDaemon runs the daemon beside this command (or on PATH) in the project
// folder, so the folder is its project, with its logs in this terminal. It
// names the configuration the daemon reads, or warns that there is none.
func startDaemon(ctx context.Context, project string, out io.Writer) (*ownedDaemon, error) {
	binary, err := findDaemon()
	if err != nil {
		return nil, err
	}
	fmt.Fprintln(out, configurationNote(project, binary))
	cmd := exec.Command(binary)
	cmd.Dir = project
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", binary, err)
	}
	daemon := &ownedDaemon{cmd: cmd, done: make(chan struct{})}
	go func() {
		daemon.err = cmd.Wait()
		close(daemon.done)
	}()
	return daemon, ctx.Err()
}

// configurationNote names the configuration a daemon started from binary
// uses for project: the project's own file, the daemon's, or the defaults.
func configurationNote(project, binary string) string {
	daemon := config.Find(binary)
	if path, err := config.ProjectFile(project); err == nil {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return "Project configuration: " + path
		}
	}
	if daemon != "" {
		return "Configuration: " + daemon + "\nThe Web UI offers to set this project up with its own configuration."
	}
	return fmt.Sprintf("No configuration found, so models use the built-in local defaults (%s).\nThe Web UI's setup saves this project's configuration in %s; a daemon-wide one goes in:\n  %s", config.DefaultProviderAPIBaseURL, config.ProjectConfigPath, strings.Join(config.Candidates(binary), "\n  "))
}

// stop asks the daemon to shut down and waits for it; it has its own budget
// to drain requests and stop running workflows.
func (d *ownedDaemon) stop() {
	select {
	case <-d.done:
		return
	default:
	}
	if err := d.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		_ = d.cmd.Process.Kill()
	}
	select {
	case <-d.done:
	case <-time.After(stopGrace):
		_ = d.cmd.Process.Kill()
		<-d.done
	}
}

func findDaemon() (string, error) {
	if self, err := os.Executable(); err == nil {
		sibling := filepath.Join(filepath.Dir(self), daemonBinary)
		if info, err := os.Stat(sibling); err == nil && !info.IsDir() {
			return sibling, nil
		}
	}
	if path, err := exec.LookPath(daemonBinary); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("%s was not found beside mini-orca or on PATH; run `just build` in the Mini-Orca repository", daemonBinary)
}

type daemonClient struct {
	base string
	http *http.Client
}

func newDaemonClient(url string) *daemonClient {
	return &daemonClient{base: strings.TrimRight(url, "/"), http: &http.Client{Timeout: 30 * time.Second}}
}

func (c *daemonClient) healthy(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/health", nil)
	if err != nil {
		return false
	}
	response, err := c.http.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	return response.StatusCode == http.StatusOK
}

// waitHealthy waits until the started daemon answers, or reports why not.
func (c *daemonClient) waitHealthy(ctx context.Context, daemon *ownedDaemon) error {
	deadline := time.NewTimer(startupTimeout)
	defer deadline.Stop()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for !c.healthy(ctx) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-daemon.done:
			return fmt.Errorf("the daemon exited during startup (%v); see its log above", daemon.err)
		case <-deadline.C:
			return fmt.Errorf("the daemon did not answer within %s", startupTimeout)
		case <-tick.C:
		}
	}
	return nil
}

// openProject makes folder the daemon's project unless it already is:
// restore saved data for a known path, otherwise import it.
func (c *daemonClient) openProject(ctx context.Context, folder string) error {
	var current struct {
		Path string `json:"path"`
	}
	if err := c.call(ctx, http.MethodGet, "/api/projects/current", nil, &current); err == nil && sameFolder(current.Path, folder) {
		return nil
	}
	body := map[string]string{"project_path": folder}
	if err := c.call(ctx, http.MethodPost, "/api/projects/restore", body, nil); err == nil {
		return nil
	}
	if err := c.call(ctx, http.MethodPost, "/api/projects/import", body, nil); err != nil {
		return fmt.Errorf("open %s in the daemon: %w", folder, err)
	}
	return nil
}

// configNote says which configuration a running daemon uses; it keeps the one
// it started with until it is restarted.
func (c *daemonClient) configNote(ctx context.Context) string {
	var catalog struct {
		ConfigFile *string `json:"config_file"`
	}
	if err := c.call(ctx, http.MethodGet, "/api/models/current", nil, &catalog); err != nil || catalog.ConfigFile == nil {
		return "Restart the daemon to load another configuration."
	}
	if *catalog.ConfigFile == "" {
		return "The running daemon found no config.yaml and uses the built-in local model defaults; stop it and run mini-orca again after adding one."
	}
	return "The running daemon uses " + *catalog.ConfigFile + "; restart it to load another configuration."
}

func sameFolder(a, b string) bool {
	if a == "" {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(a); err == nil {
		a = resolved
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func (c *daemonClient) call(ctx context.Context, method, path string, body, result any) error {
	var payload io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.base+path, payload)
	if err != nil {
		return err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if method != http.MethodGet {
		token, err := readWriteToken()
		if err != nil {
			return err
		}
		request.Header.Set(api.WriteTokenHeader, token)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode >= 300 {
		return fmt.Errorf("%s %s: %s", method, path, daemonMessage(data, response.Status))
	}
	if result == nil {
		return nil
	}
	return json.Unmarshal(data, result)
}

func daemonMessage(data []byte, status string) string {
	var failure struct {
		UserMessage string `json:"user_message"`
		Message     string `json:"message"`
	}
	if json.Unmarshal(data, &failure) == nil {
		if failure.UserMessage != "" {
			return failure.UserMessage
		}
		if failure.Message != "" {
			return failure.Message
		}
	}
	return status
}

func readWriteToken() (string, error) {
	path, err := api.WriteTokenFile()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("the daemon's write token %s is missing; restart the daemon", path)
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// goos selects the browser opener; tests replace it.
var goos = runtime.GOOS

// openBrowser opens page with the system's default browser.
func openBrowser(page string) error {
	var cmd *exec.Cmd
	switch goos {
	case "darwin":
		cmd = exec.Command("open", page)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", page)
	default:
		cmd = exec.Command("xdg-open", page)
	}
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	return cmd.Start()
}
