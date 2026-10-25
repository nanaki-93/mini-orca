package handlers

import (
	_ "embed"
	"net/http"
)

// WebUICSP restricts the embedded local Web UI to same-origin API requests and inline styles/scripts.
const WebUICSP = "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; img-src 'self' data:; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

//go:embed webui.html
var webUIHTML []byte

// WebUIHandler serves the self-contained local Web UI and diff viewer.
type WebUIHandler struct{}

// NewWebUIHandler constructs the handler for GET /ui.
func NewWebUIHandler() *WebUIHandler {
	return &WebUIHandler{}
}

// Serve writes the embedded single-page Web UI with strict local security headers.
func (h *WebUIHandler) Serve(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", WebUICSP)
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(webUIHTML)
}
