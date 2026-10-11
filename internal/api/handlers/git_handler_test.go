package handlers

import (
	"encoding/json"
	"net/http/httptest"
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
