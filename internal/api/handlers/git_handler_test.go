package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
)

func TestGitRepositoryRequiresCurrentProject(t *testing.T) {
	fixture := newChangeHandlerFixture(t, "http://127.0.0.1:1")
	handler := NewGitHandler(fixture.service)
	index, _ := fixture.manager.Index()
	for _, current := range []bool{false, true} {
		path := "/api/projects/current/git/repository"
		if current {
			path += "?project_id=" + index.ProjectID + "&project_revision=" + index.ProjectRevision
		}
		response := httptest.NewRecorder()
		handler.Repository(response, httptest.NewRequest("GET", path, nil))
		if !current {
			if response.Code != 409 {
				t.Fatalf("unguarded read: %d", response.Code)
			}
			continue
		}
		var repository app.GitRepository
		if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &repository) != nil || repository.Available || repository.ProjectID != index.ProjectID {
			t.Fatalf("non-Git response: %d %s", response.Code, response.Body)
		}
	}
}

func TestProjectGitStatusEndpointHonorsCancellationAndPathPolicy(t *testing.T) {
	fixture := newChangeHandlerFixture(t, "http://127.0.0.1:1")
	handler := NewProjectHandler(fixture.manager, fixture.service)

	okResponse := httptest.NewRecorder()
	handler.GitStatus(okResponse, httptest.NewRequest(http.MethodGet, "/api/projects/current/git?path=main.go", nil))
	if okResponse.Code != http.StatusOK {
		t.Fatalf("GitStatus status = %d: %s", okResponse.Code, okResponse.Body.String())
	}

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	canceledResponse := httptest.NewRecorder()
	handler.GitStatus(canceledResponse, httptest.NewRequest(http.MethodGet, "/api/projects/current/git?path=main.go", nil).WithContext(canceledCtx))
	if canceledResponse.Code != http.StatusRequestTimeout {
		t.Fatalf("canceled GitStatus status = %d, want 408: %s", canceledResponse.Code, canceledResponse.Body.String())
	}
}

func TestGitMutationsRejectUnconfirmedAndUnknownFields(t *testing.T) {
	fixture := newChangeHandlerFixture(t, "http://127.0.0.1:1")
	handler := NewGitHandler(fixture.service)
	for _, action := range []http.HandlerFunc{handler.CreateWorktree, handler.Switch, handler.Merge, handler.AbortMerge, handler.DiscardWorktree} {
		for _, body := range []string{`{}`, `{"confirm":true,"shell":"rm"}`, `{} {}`} {
			response := httptest.NewRecorder()
			action(response, httptest.NewRequest("POST", "/git", strings.NewReader(body)))
			if response.Code < 400 {
				t.Fatalf("accepted unsupported mutation: %s", body)
			}
		}
	}
}
