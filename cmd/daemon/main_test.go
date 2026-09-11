package main

import (
	"encoding/json"
	"gopkg.in/yaml.v3"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"github.com/nanaki-93/mini-orca/v2/internal/version"
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
	request := httptest.NewRequest(http.MethodGet, "/status", nil)
	request.Host = "localhost:9090"
	newHTTPMux(service, manager).ServeHTTP(response, request)
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

func TestLocalAPIPolicyRejectsBrowserAndInvalidContentBeforeRoutes(t *testing.T) {
	t.Setenv("MINI_ORCA_BIND_ADDRESS", "")
	mux := daemonTestMux(t)

	tests := []struct {
		name        string
		method      string
		path        string
		host        string
		origin      string
		contentType string
		preflight   bool
		wantStatus  int
	}{
		{name: "forged host", method: http.MethodGet, path: "/health", host: "attacker.example", wantStatus: http.StatusForbidden},
		{name: "hostile origin mutation", method: http.MethodPost, path: "/api/projects/restore", host: "localhost:9090", origin: "https://attacker.example", contentType: "application/json", wantStatus: http.StatusForbidden},
		{name: "browser preflight", method: http.MethodOptions, path: "/api/projects/restore", host: "localhost:9090", origin: "https://attacker.example", preflight: true, wantStatus: http.StatusForbidden},
		{name: "missing JSON content type", method: http.MethodPost, path: "/api/projects/restore", host: "localhost:9090", wantStatus: http.StatusUnsupportedMediaType},
		{name: "non JSON mutation", method: http.MethodPost, path: "/api/projects/restore", host: "localhost:9090", contentType: "text/plain", wantStatus: http.StatusUnsupportedMediaType},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(`{"project_path":"ignored"}`))
			request.Host = test.host
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			if test.contentType != "" {
				request.Header.Set("Content-Type", test.contentType)
			}
			if test.preflight {
				request.Header.Set("Access-Control-Request-Method", http.MethodPost)
			}

			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("response status = %d, want %d: %s", response.Code, test.wantStatus, response.Body.String())
			}
			if response.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatalf("unexpected CORS response header: %q", response.Header().Get("Access-Control-Allow-Origin"))
			}
		})
	}
}

func TestLocalAPIPolicyAllowsNativeDesktopCallsAndHealth(t *testing.T) {
	t.Setenv("MINI_ORCA_BIND_ADDRESS", "")
	mux := daemonTestMux(t)
	health := httptest.NewRecorder()
	healthRequest := httptest.NewRequest(http.MethodGet, "/health", nil)
	healthRequest.Host = "localhost:9090"
	mux.ServeHTTP(health, healthRequest)
	if health.Code != http.StatusOK {
		t.Fatalf("health response = %d: %s", health.Code, health.Body.String())
	}

	for _, test := range []struct {
		name string
		path string
		body io.Reader
	}{
		{name: "JSON request with charset", path: "/api/projects/restore", body: strings.NewReader(`{"project_path":"ignored"}`)},
		{name: "bodyless analyze-all pause", path: "/api/projects/current/analysis-job/pause?project_revision=revision"},
		{name: "bodyless analyze-all cancel", path: "/api/projects/current/analysis-job/cancel?project_revision=revision"},
		{name: "bodyless performance pause", path: "/api/projects/current/performance-job/pause?project_revision=revision&expected_job_id=job"},
		{name: "bodyless performance cancel", path: "/api/projects/current/performance-job/cancel?project_revision=revision&expected_job_id=job"},
	} {
		t.Run(test.name, func(t *testing.T) {
			handlerCalled := false
			nativeRequest := httptest.NewRequest(http.MethodPost, test.path, test.body)
			nativeRequest.Host = "127.0.0.1:9090"
			if test.body != nil {
				nativeRequest.Header.Set("Content-Type", "application/json; charset=utf-8")
			}
			response := httptest.NewRecorder()
			api.LocalOnly(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusNoContent)
			}), false).ServeHTTP(response, nativeRequest)
			if response.Code != http.StatusNoContent || !handlerCalled {
				t.Fatalf("native request status = %d, handler called = %t", response.Code, handlerCalled)
			}
		})
	}
}

func TestExplicitExternalBindAllowsExternalHost(t *testing.T) {
	for _, bindAddress := range []string{"0.0.0.0:9090", ":9090"} {
		t.Run(bindAddress, func(t *testing.T) {
			t.Setenv("MINI_ORCA_BIND_ADDRESS", bindAddress)
			mux := daemonTestMux(t)
			request := httptest.NewRequest(http.MethodGet, "/health", nil)
			request.Host = "desktop.example:9090"
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("external bind response = %d: %s", response.Code, response.Body.String())
			}
		})
	}
}

func daemonTestMux(t *testing.T) http.Handler {
	t.Helper()
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
	return newHTTPMux(service, manager)
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

func TestOpenAPIGoScanPhaseIncludesWorkspace(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Components struct {
			Schemas map[string]struct {
				Properties map[string]struct {
					Enum []string `yaml:"enum"`
				} `yaml:"properties"`
			} `yaml:"schemas"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(data, &spec); err != nil {
		t.Fatalf("parse OpenAPI: %v", err)
	}
	for _, name := range spec.Components.Schemas["GoScanPhase"].Properties["name"].Enum {
		if name == "workspace" {
			return
		}
	}
	t.Fatalf("GoScanPhase names = %v, want workspace", spec.Components.Schemas["GoScanPhase"].Properties["name"].Enum)
}

func TestOpenAPIGoBenchmarkSchemaMatchesRuntimeBounds(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	type schema struct {
		Properties       map[string]schema `yaml:"properties"`
		Required         []string          `yaml:"required"`
		Minimum          *float64          `yaml:"minimum"`
		ExclusiveMinimum bool              `yaml:"exclusiveMinimum"`
		MinItems         int               `yaml:"minItems"`
		MaxItems         int               `yaml:"maxItems"`
		AllOf            []schema          `yaml:"allOf"`
		OneOf            []schema          `yaml:"oneOf"`
		Not              *schema           `yaml:"not"`
	}
	var spec struct {
		Components struct {
			Schemas map[string]schema `yaml:"schemas"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}
	request := spec.Components.Schemas["GoBenchmarkComparisonRequest"]
	if request.Properties["expected_revision"].Minimum == nil || *request.Properties["expected_revision"].Minimum != 1 {
		t.Fatalf("expected_revision schema = %#v", request.Properties["expected_revision"])
	}
	sample := spec.Components.Schemas["GoBenchmarkSample"]
	if sample.Properties["iterations"].Minimum == nil || *sample.Properties["iterations"].Minimum != 1 || sample.Properties["ns_per_op"].Minimum == nil || *sample.Properties["ns_per_op"].Minimum != 0 || !sample.Properties["ns_per_op"].ExclusiveMinimum || sample.Properties["bytes_per_op"].Minimum == nil || *sample.Properties["bytes_per_op"].Minimum != 0 || sample.Properties["allocs_per_op"].Minimum == nil || *sample.Properties["allocs_per_op"].Minimum != 0 {
		t.Fatalf("sample schema = %#v", sample)
	}
	measurement := spec.Components.Schemas["GoBenchmarkMeasurement"]
	if measurement.Properties["samples"].MinItems != 5 || measurement.Properties["samples"].MaxItems != 5 {
		t.Fatalf("measurement schema = %#v", measurement.Properties["samples"])
	}
	comparison := spec.Components.Schemas["GoBenchmarkComparison"]
	if len(comparison.AllOf) != 1 || len(comparison.AllOf[0].OneOf) != 3 || !containsSchemaRequired(comparison.AllOf[0].OneOf[0].Required, "base") || !containsSchemaRequired(comparison.AllOf[0].OneOf[0].Required, "candidate") || comparison.AllOf[0].OneOf[1].Not == nil || comparison.AllOf[0].OneOf[2].Not == nil {
		t.Fatalf("comparison status schema = %#v", comparison.AllOf)
	}
}

func containsSchemaRequired(required []string, name string) bool {
	for _, item := range required {
		if item == name {
			return true
		}
	}
	return false
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
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Host = "localhost:9090"
		mux.ServeHTTP(response, request)
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

func TestUnifiedAnalysisRoutesUseLocalOriginPolicy(t *testing.T) {
	manager, err := project.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service, err := app.New(config.Default(), manager)
	if err != nil {
		t.Fatal(err)
	}
	mux := newHTTPMux(service, manager)
	for _, route := range []string{"/api/projects/current/analysis/preview", "/api/projects/current/analysis/run", "/api/projects/current/analysis/run/control"} {
		request := httptest.NewRequest(http.MethodPost, route, strings.NewReader(`{}`))
		request.Host = "localhost:9090"
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", "https://untrusted.example")
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("origin bypass %s: %d %s", route, response.Code, response.Body)
		}
	}
}
