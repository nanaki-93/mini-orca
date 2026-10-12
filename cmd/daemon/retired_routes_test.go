package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRetiredWorkflowRoutesAreUnavailable(t *testing.T) {
	mux := daemonTestMux(t)
	for _, route := range []struct{ method, path string }{
		{"POST", "chat/sessions"},
		{"POST", "chat/sessions/old/messages"},
		{"PATCH", "drafts/old"},
		{"POST", "drafts/old/checks"},
		{"GET", "drafts/old/benchmarks"},
		{"POST", "apply"},
		{"POST", "undo"},
		{"GET", "analysis-job"},
		{"POST", "analysis-job/resume"},
		{"GET", "performance"},
		{"GET", "performance/context"},
		{"POST", "performance-job"},
	} {
		t.Run(route.method+"/"+route.path, func(t *testing.T) {
			request := httptest.NewRequest(route.method, "/api/projects/current/"+route.path, strings.NewReader("{}"))
			request.Host = "localhost:9090"
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			if response.Code != http.StatusNotFound {
				t.Fatalf("retired route returned %d: %s", response.Code, response.Body)
			}
		})
	}
}
