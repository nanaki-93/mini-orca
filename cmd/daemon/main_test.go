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

const routeRetained = "retained"

type routeSpec struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Disposition string `json:"disposition"`
	Consumer    string `json:"consumer,omitempty"`
}

func registeredRoutes() []routeSpec {
	return []routeSpec{
		{http.MethodGet, "/health", routeRetained, "container health check"},
		{http.MethodGet, "/status", routeRetained, "Desktop ApiClient.status"},
		{http.MethodPost, "/api/projects/current/chat/sessions", routeRetained, "Desktop ApiClient.openChatSession"},
		{http.MethodPost, "/api/projects/current/chat/sessions/{sessionID}/messages", routeRetained, "Desktop ApiClient.sendChatMessage"},
		{http.MethodGet, "/api/models/current", routeRetained, "Desktop ApiClient.modelCatalog"},
		{http.MethodGet, "/api/projects/current/context", routeRetained, "Desktop ApiClient.context"},
		{http.MethodPost, "/api/projects/import", routeRetained, "Desktop ApiClient.importProject"},
		{http.MethodPost, "/api/projects/restore", routeRetained, "Desktop ApiClient.restoreProject"},
		{http.MethodGet, "/api/projects/current/overview", routeRetained, "Desktop ApiClient.overview"},
		{http.MethodGet, "/api/projects/current/findings", routeRetained, "Desktop ApiClient.findings"},
		{http.MethodPatch, "/api/projects/current/findings/{findingID}", routeRetained, "Desktop ApiClient.updateFindingStatus"},
		{http.MethodGet, "/api/projects/current/scan", routeRetained, "Desktop ApiClient.goScan"},
		{http.MethodPost, "/api/projects/current/scan", routeRetained, "Desktop ApiClient.startGoScan"},
		{http.MethodDelete, "/api/projects/current/scan", routeRetained, "Desktop ApiClient.cancelGoScan"},
		{http.MethodGet, "/api/projects/current/index", routeRetained, "Desktop ApiClient.index"},
		{http.MethodGet, "/api/projects/current/files/info", routeRetained, "Desktop ApiClient.fileInfo"},
		{http.MethodGet, "/api/projects/current/files/symbols", routeRetained, "Desktop ApiClient.symbols"},
		{http.MethodGet, "/api/projects/current/impact", routeRetained, "Desktop ApiClient.impact"},
		{http.MethodGet, "/api/projects/current/git", routeRetained, "Desktop ApiClient.gitStatus"},
		{http.MethodGet, "/api/projects/current/files/analysis", routeRetained, "Desktop ApiClient.analysis"},
		{http.MethodPost, "/api/projects/current/files/analysis", routeRetained, "Desktop ApiClient.analyze"},
		{http.MethodGet, "/api/projects/current/analysis-job", routeRetained, "Desktop ApiClient.analyzeAllJob"},
		{http.MethodPost, "/api/projects/current/analysis-job", routeRetained, "Desktop ApiClient.startAnalyzeAll"},
		{http.MethodPost, "/api/projects/current/analysis-job/pause", routeRetained, "Desktop ApiClient.pauseAnalyzeAll"},
		{http.MethodPost, "/api/projects/current/analysis-job/resume", routeRetained, "Desktop ApiClient.resumeAnalyzeAll"},
		{http.MethodPost, "/api/projects/current/analysis-job/cancel", routeRetained, "Desktop ApiClient.cancelAnalyzeAll"},
		{http.MethodPost, "/api/projects/current/reindex", routeRetained, "Desktop ApiClient.reindex"},
		{http.MethodPatch, "/api/projects/current/drafts/{draftID}", routeRetained, "Desktop ApiClient.updateDraft"},
		{http.MethodPost, "/api/projects/current/drafts/{draftID}/validate", routeRetained, "Desktop ApiClient.validateDraft"},
		{http.MethodPost, "/api/projects/current/drafts/{draftID}/checks", routeRetained, "Desktop ApiClient.checkDraft"},
		{http.MethodPost, "/api/projects/current/apply", routeRetained, "Desktop ApiClient.applyDraft"},
		{http.MethodPost, "/api/projects/current/undo", routeRetained, "Desktop ApiClient.undo"},
	}
}

func TestOpenAPIRoutesMatchRegisteredDesktopAPI(t *testing.T) {
	routes := documentedRegisteredRoutes()

	if got := openAPIRoutes(t); !sameRoutes(got, routes) {
		t.Fatalf("OpenAPI routes = %v, want %v", got, routes)
	}
	if got := apiContractRoutes(t); !sameRoutes(got, routes) {
		t.Fatalf("API contract routes = %v, want %v", got, routes)
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
			path := route.path
			if path == "/api/projects/current/files/info" || path == "/api/projects/current/files/symbols" || path == "/api/projects/current/files/analysis" || path == "/api/projects/current/context" {
				path += "?path=missing.go"
			}
			if route.path == "/api/projects/current/overview" || route.path == "/api/projects/current/findings" || route.path == "/api/projects/current/scan" {
				path += "?project_revision=sha256:missing"
			}
			req := httptest.NewRequest(route.method, path, nil)
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, req)
			if response.Code == http.StatusNotFound || response.Code == http.StatusMethodNotAllowed {
				t.Fatalf("documented route is not registered: status %d", response.Code)
			}
		})
	}
}

func TestCleanupRouteInventoryMatchesDaemonRegistration(t *testing.T) {
	inventory := cleanupRouteInventory(t)
	registered := registeredRoutes()
	if len(inventory) != len(registered) {
		t.Fatalf("cleanup inventory routes = %d, registered routes = %d", len(inventory), len(registered))
	}
	for _, route := range registered {
		if route.Disposition != routeRetained {
			t.Fatalf("%s %s has invalid disposition %q", route.Method, route.Path, route.Disposition)
		}
		if route.Consumer == "" {
			t.Fatalf("registered route %s %s has no maintained consumer", route.Method, route.Path)
		}
		if !containsRouteSpec(inventory, route) {
			t.Fatalf("registered route missing from cleanup inventory: %+v", route)
		}
	}
	for _, route := range inventory {
		if !containsRouteSpec(registered, route) {
			t.Fatalf("cleanup inventory route is not registered: %+v", route)
		}
	}
}

func TestMaintainedDesktopClientPathsHaveRegisteredRoutes(t *testing.T) {
	wantConsumers := []string{
		"Desktop ApiClient.status", "Desktop ApiClient.openChatSession", "Desktop ApiClient.sendChatMessage",
		"Desktop ApiClient.modelCatalog", "Desktop ApiClient.context", "Desktop ApiClient.importProject",
		"Desktop ApiClient.restoreProject", "Desktop ApiClient.overview", "Desktop ApiClient.findings",
		"Desktop ApiClient.updateFindingStatus", "Desktop ApiClient.goScan", "Desktop ApiClient.startGoScan",
		"Desktop ApiClient.cancelGoScan", "Desktop ApiClient.index", "Desktop ApiClient.fileInfo",
		"Desktop ApiClient.symbols", "Desktop ApiClient.impact", "Desktop ApiClient.gitStatus",
		"Desktop ApiClient.analysis", "Desktop ApiClient.analyze", "Desktop ApiClient.analyzeAllJob",
		"Desktop ApiClient.startAnalyzeAll", "Desktop ApiClient.pauseAnalyzeAll", "Desktop ApiClient.resumeAnalyzeAll",
		"Desktop ApiClient.cancelAnalyzeAll", "Desktop ApiClient.reindex", "Desktop ApiClient.updateDraft",
		"Desktop ApiClient.validateDraft", "Desktop ApiClient.checkDraft", "Desktop ApiClient.applyDraft",
		"Desktop ApiClient.undo",
	}
	for _, consumer := range wantConsumers {
		var found bool
		for _, route := range registeredRoutes() {
			if route.Consumer == consumer && route.Disposition == routeRetained {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("maintained desktop client consumer %q has no registered route", consumer)
		}
	}
}

func documentedRegisteredRoutes() []documentedRoute {
	routes := registeredRoutes()
	result := make([]documentedRoute, 0, len(routes))
	for _, route := range routes {
		result = append(result, documentedRoute{method: route.Method, path: route.Path})
	}
	return result
}

func cleanupRouteInventory(t *testing.T) []routeSpec {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "cleanup-baseline-routes.json"))
	if err != nil {
		t.Fatal(err)
	}
	var inventory []routeSpec
	if err := json.Unmarshal(data, &inventory); err != nil {
		t.Fatalf("parse cleanup route inventory: %v", err)
	}
	return inventory
}

func containsRouteSpec(routes []routeSpec, want routeSpec) bool {
	for _, route := range routes {
		if route == want {
			return true
		}
	}
	return false
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
