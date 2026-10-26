package handlers

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// WebUICSP restricts the embedded local Web UI to same-origin scripts, styles
// and API requests. No inline script or style is permitted.
const WebUICSP = "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' data:; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

// The Web UI is a no-build set of ES modules. Tests and development notes in
// webui/test are deliberately not embedded.
//
//go:embed webui/index.html webui/app.css webui/js
var webUIFiles embed.FS

var webUIContentTypes = map[string]string{
	".css": "text/css; charset=utf-8",
	".js":  "text/javascript; charset=utf-8",
}

// WebUIHandler serves the embedded local Web UI and its static assets.
type WebUIHandler struct {
	files fs.FS
}

// NewWebUIHandler constructs the handler for GET /ui and GET /ui/assets/{path...}.
func NewWebUIHandler() *WebUIHandler {
	files, err := fs.Sub(webUIFiles, "webui")
	if err != nil {
		panic("embedded web UI is missing: " + err.Error())
	}
	return &WebUIHandler{files: files}
}

// Serve writes the Web UI document with strict local security headers.
func (h *WebUIHandler) Serve(w http.ResponseWriter, _ *http.Request) {
	data, err := fs.ReadFile(h.files, "index.html")
	if err != nil {
		http.Error(w, "web UI unavailable", http.StatusInternalServerError)
		return
	}
	writeWebUIHeaders(w, "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// ServeAsset writes one embedded stylesheet or module. Only clean, relative
// .css and .js paths are served; directories and other files are not found.
func (h *WebUIHandler) ServeAsset(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("path")
	contentType, ok := webUIContentTypes[path.Ext(name)]
	if !ok || name == "" || path.Clean(name) != name || strings.HasPrefix(name, "/") || strings.Contains(name, "..") || !fs.ValidPath(name) {
		http.NotFound(w, r)
		return
	}
	data, err := fs.ReadFile(h.files, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	writeWebUIHeaders(w, contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func writeWebUIHeaders(w http.ResponseWriter, contentType string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", WebUICSP)
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}
