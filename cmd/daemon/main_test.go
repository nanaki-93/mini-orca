package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"github.com/nanaki-93/mini-orca/v2/internal/version"
	"gopkg.in/yaml.v3"
)

func TestDaemonStatusReportsCanonicalVersion(t *testing.T) {
	root := t.TempDir()
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &project.Analysis{Name: "fixture", Path: root}); err != nil {
		t.Fatal(err)
	}
	service, err := app.New(config.Default(), manager)
	if err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	newHTTPMux(service, manager).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/status", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status response = %d: %s", response.Code, response.Body.String())
	}
	var status struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(response.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	if status.Version != version.Version {
		t.Fatalf("daemon version = %q, want %q", status.Version, version.Version)
	}
}

func TestReleaseDocumentationUsesCanonicalVersion(t *testing.T) {
	root := filepath.Join("..", "..")
	for path, expected := range map[string]string{
		"docs/openapi.yaml":    "version: " + version.Version,
		"docs/api-contract.md": "**Version:** " + version.Version,
		"RELEASE_NOTES.md":     "## v" + version.Version + " —",
		"DOCKER.md":            "mini-orca:" + version.Version,
	} {
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if !strings.Contains(string(data), expected) {
			t.Errorf("%s does not contain canonical version marker %q", path, expected)
		}
	}
}

type documentedRoute struct {
	method string
	path   string
}

func TestDocumentedRoutesAreHandledByDaemon(t *testing.T) {
	routes := openAPIRoutes(t)
	if got := apiContractRoutes(t); !sameRoutes(got, routes) {
		t.Fatalf("API contract routes = %v, want OpenAPI routes %v", got, routes)
	}

	root := t.TempDir()
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &project.Analysis{Name: "fixture", Path: root}); err != nil {
		t.Fatal(err)
	}
	service, err := app.New(config.Default(), manager)
	if err != nil {
		t.Fatal(err)
	}
	mux := newHTTPMux(service, manager)
	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			req := httptest.NewRequest(route.method, requestPathForDocumentedRoute(route.path), nil)
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, req)
			if response.Code == http.StatusNotFound || response.Code == http.StatusMethodNotAllowed {
				t.Fatalf("documented route is not registered: status %d", response.Code)
			}
		})
	}
}

func requestPathForDocumentedRoute(path string) string {
	switch path {
	case "/api/projects/current/files/info", "/api/projects/current/files/symbols", "/api/projects/current/files/analysis", "/api/projects/current/context":
		return path + "?path=missing.go"
	case "/api/projects/current/overview", "/api/projects/current/findings", "/api/projects/current/scan":
		return path + "?project_revision=sha256:missing"
	default:
		return path
	}
}

func TestDaemonDoesNotRegisterBrowserUIRoutes(t *testing.T) {
	root := t.TempDir()
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &project.Analysis{Name: "fixture", Path: root}); err != nil {
		t.Fatal(err)
	}
	service, err := app.New(config.Default(), manager)
	if err != nil {
		t.Fatal(err)
	}
	mux := newHTTPMux(service, manager)
	for _, path := range []string{"/", "/static/js/chat.js", "/api/render/file-tree", "/api/tree/expand", "/api/files/view"} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("browser route %s returned %d, want 404", path, response.Code)
		}
	}
}

func TestDaemonBindsToLoopbackUnlessExplicitlyOverridden(t *testing.T) {
	t.Setenv("MINI_ORCA_BIND_ADDRESS", "")
	if got := daemonAddress(); got != "127.0.0.1:9090" {
		t.Fatalf("default daemon address = %q", got)
	}
	t.Setenv("MINI_ORCA_BIND_ADDRESS", "0.0.0.0:9090")
	if got := daemonAddress(); got != "0.0.0.0:9090" {
		t.Fatalf("override daemon address = %q", got)
	}
}

func openAPIRoutes(t *testing.T) []documentedRoute {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(data, &spec); err != nil {
		t.Fatalf("parse OpenAPI: %v", err)
	}
	var routes []documentedRoute
	for path, methods := range spec.Paths {
		for method := range methods {
			if method == "parameters" {
				continue
			}
			routes = append(routes, documentedRoute{strings.ToUpper(method), path})
		}
	}
	sort.Slice(routes, func(i, j int) bool { return routes[i].method+routes[i].path < routes[j].method+routes[j].path })
	return routes
}

func sameRoutes(left, right []documentedRoute) bool {
	if len(left) != len(right) {
		return false
	}
	copyRight := append([]documentedRoute(nil), right...)
	sort.Slice(copyRight, func(i, j int) bool {
		return copyRight[i].method+copyRight[i].path < copyRight[j].method+copyRight[j].path
	})
	for i := range left {
		if left[i] != copyRight[i] {
			return false
		}
	}
	return true
}

func apiContractRoutes(t *testing.T) []documentedRoute {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "api-contract.md"))
	if err != nil {
		t.Fatal(err)
	}
	var routes []documentedRoute
	for _, line := range strings.Split(string(data), "\n") {
		columns := strings.Split(line, "|")
		if len(columns) < 4 || !strings.HasPrefix(strings.TrimSpace(columns[1]), "GET") && !strings.HasPrefix(strings.TrimSpace(columns[1]), "POST") && !strings.HasPrefix(strings.TrimSpace(columns[1]), "PATCH") && !strings.HasPrefix(strings.TrimSpace(columns[1]), "DELETE") {
			continue
		}
		routes = append(routes, documentedRoute{
			method: strings.TrimSpace(columns[1]),
			path:   strings.Trim(strings.TrimSpace(columns[2]), "`"),
		})
	}
	sort.Slice(routes, func(i, j int) bool { return routes[i].method+routes[i].path < routes[j].method+routes[j].path })
	return routes
}
