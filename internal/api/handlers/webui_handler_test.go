package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebUIHandlerServesEmbeddedAppWithSecurityHeaders(t *testing.T) {
	handler := NewWebUIHandler()
	req := httptest.NewRequest(http.MethodGet, "/ui", nil)
	res := httptest.NewRecorder()
	handler.Serve(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}
	if got := res.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := res.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := res.Header().Get("Content-Security-Policy"); got != WebUICSP {
		t.Fatalf("Content-Security-Policy = %q, want %q", got, WebUICSP)
	}
	if got := res.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Fatalf("Referrer-Policy = %q", got)
	}
	if got := res.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
	}
	body := res.Body.String()
	for _, needle := range []string{
		`Mini-Orca · Workspace`,
		"left-dock",
		`data-tab="understand"`,
		`data-tab="build"`,
		`data-tab="diff"`,
		"Split (Side-by-Side)",
		"X-Mini-Orca-Web-UI",
		"/api/projects/current/overview",
		"/api/projects/current/files/explanation",
		"/api/projects/current/changes",
		`data-theme="tokyo"`,
		"cmd-modal-backdrop",
		"ide-status-bar",
		`Checks`,
		"phase-summary-bar",
		"/api/projects/current/scan",
		"diff-subtab-bar",
		"diff-subtab-checks",
		"diff-subtab-spec",
		"1 · Task Intent &amp; Specification",
		"2 · Target File Scope (1–8 Files)",
		"3 · Verification &amp; Acceptance Criteria",
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("embedded webui.html missing %q", needle)
		}
	}
	for _, retired := range []string{
		"/api/projects/current/files/symbols",
		`<header class="topbar">`,
		`<nav class="nav-tabs">`,
	} {
		if strings.Contains(body, retired) {
			t.Fatalf("embedded webui.html still contains retired markup or endpoint %q", retired)
		}
	}
}
