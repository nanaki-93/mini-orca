package handlers

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func webUIMux() *http.ServeMux {
	handler := NewWebUIHandler()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ui", handler.Serve)
	mux.HandleFunc("GET /ui/assets/{path...}", handler.ServeAsset)
	return mux
}

func assertWebUISecurityHeaders(t *testing.T, res *httptest.ResponseRecorder, contentType string) {
	t.Helper()
	for header, want := range map[string]string{
		"Content-Type":            contentType,
		"Cache-Control":           "no-store",
		"Content-Security-Policy": WebUICSP,
		"Referrer-Policy":         "no-referrer",
		"X-Content-Type-Options":  "nosniff",
	} {
		if got := res.Header().Get(header); got != want {
			t.Fatalf("%s = %q, want %q", header, got, want)
		}
	}
}

func TestWebUIHandlerServesShellWithStrictSecurityHeaders(t *testing.T) {
	res := httptest.NewRecorder()
	webUIMux().ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/ui", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}
	assertWebUISecurityHeaders(t, res, "text/html; charset=utf-8")
	body := res.Body.String()
	for _, needle := range []string{
		`<script type="module" src="/ui/assets/js/main.js"></script>`,
		`<link rel="stylesheet" href="/ui/assets/app.css">`,
		`id="activity"`, `id="side"`, `id="main"`, `id="palette"`, `id="dialog"`, `id="toasts"`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("web UI shell missing %q", needle)
		}
	}
	if strings.Contains(body, " style=") || regexp.MustCompile(`<script>`).MatchString(body) || strings.Contains(body, "<style") {
		t.Fatal("web UI shell must not contain inline scripts or styles; the CSP forbids them")
	}
	if strings.Contains(WebUICSP, "unsafe-inline") || strings.Contains(WebUICSP, "unsafe-eval") {
		t.Fatalf("CSP must not allow inline or eval code: %s", WebUICSP)
	}
}

func TestWebUIAssetsServeModulesAndStylesWithContentTypes(t *testing.T) {
	mux := webUIMux()
	for path, contentType := range map[string]string{
		"/ui/assets/app.css":              "text/css; charset=utf-8",
		"/ui/assets/js/main.js":           "text/javascript; charset=utf-8",
		"/ui/assets/js/core/readiness.js": "text/javascript; charset=utf-8",
		"/ui/assets/js/views/review.js":   "text/javascript; charset=utf-8",
		"/ui/assets/js/app/actions.js":    "text/javascript; charset=utf-8",
		"/ui/assets/js/ui/overlays.js":    "text/javascript; charset=utf-8",
	} {
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, httptest.NewRequest(http.MethodGet, path, nil))
		if res.Code != http.StatusOK {
			t.Fatalf("GET %s = %d, want 200", path, res.Code)
		}
		assertWebUISecurityHeaders(t, res, contentType)
		if res.Body.Len() == 0 {
			t.Fatalf("GET %s returned an empty body", path)
		}
	}
}

func TestWebUIAssetsRejectUnsafeOrUnknownPaths(t *testing.T) {
	mux := webUIMux()
	for _, path := range []string{
		"/ui/assets/",
		"/ui/assets/js",
		"/ui/assets/js/",
		"/ui/assets/index.html",
		"/ui/assets/missing.js",
		"/ui/assets/js/../app.css",
		"/ui/assets/js/./main.js",
		"/ui/assets/test/readiness.test.mjs",
		"/ui/assets/%2e%2e/webui_handler.go",
	} {
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, httptest.NewRequest(http.MethodGet, path, nil))
		if res.Code == http.StatusOK {
			t.Fatalf("GET %s = 200, want rejection", path)
		}
	}
}

// Every module import must resolve to an embedded file, otherwise the browser
// fails to boot the app with no server-side signal.
func TestWebUIModuleImportsResolveToEmbeddedFiles(t *testing.T) {
	files, err := fs.Sub(webUIFiles, "webui")
	if err != nil {
		t.Fatal(err)
	}
	importPattern := regexp.MustCompile(`(?m)^\s*import\s[^'"]*['"]([^'"]+)['"]`)
	count := 0
	err = fs.WalkDir(files, "js", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		if !strings.HasSuffix(name, ".js") {
			t.Fatalf("unexpected embedded file %s", name)
		}
		source, err := fs.ReadFile(files, name)
		if err != nil {
			return err
		}
		if strings.Contains(string(source), `style="`) {
			t.Fatalf("%s builds inline style attributes, which the CSP blocks", name)
		}
		for _, match := range importPattern.FindAllStringSubmatch(string(source), -1) {
			target := pathJoin(name, match[1])
			if _, err := fs.Stat(files, target); err != nil {
				t.Fatalf("%s imports %s, which is not embedded (%s)", name, match[1], target)
			}
			count++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count < 20 {
		t.Fatalf("found only %d module imports; the import scan is not reading the modules", count)
	}
}

func pathJoin(from, relative string) string {
	parts := strings.Split(from, "/")
	parts = parts[:len(parts)-1]
	for _, segment := range strings.Split(relative, "/") {
		switch segment {
		case ".", "":
		case "..":
			parts = parts[:len(parts)-1]
		default:
			parts = append(parts, segment)
		}
	}
	return strings.Join(parts, "/")
}
