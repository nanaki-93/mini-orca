package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
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

func TestCleanupMergedHTTPGuardsIdentityAndReturnsRemovalReceipt(t *testing.T) {
	fixture := newChangeHandlerFixture(t, "http://127.0.0.1:1")
	root := fixture.manager.Root()
	git := func(args ...string) {
		t.Helper()
		flags := []string{"-C", root, "-c", "core.hooksPath=" + os.DevNull, "-c", "commit.gpgSign=false", "-c", "user.name=Test", "-c", "user.email=test@example.com"}
		if out, err := exec.Command("git", append(flags, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git: %v %s", err, out)
		}
	}
	git("init", "-b", "main")
	git("add", "main.go")
	git("commit", "-m", "Initial")
	index, _ := fixture.manager.Index()
	repo, err := fixture.service.GitRepository(context.Background(), index.ProjectID, index.ProjectRevision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.TrustProjectExecution(context.Background(), index.ProjectRevision, true); err != nil {
		t.Fatal(err)
	}
	request := app.GitMutationRequest{GitIdentity: repo.GitIdentity, ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Confirm: true, ConfirmExecution: true, TargetBranch: "fix", Path: filepath.Join(root, ".worktrees", "fix")}
	created, err := fixture.service.CreateGitWorktree(context.Background(), request)
	if err != nil || created.State != "created" {
		t.Fatalf("create: %+v %v", created, err)
	}
	request.SourceHead, request.SourceBranch = "stale", "fix"
	h := NewGitHandler(fixture.service)
	response := workflowResponse(t, h.CleanupMergedWorktree, "POST", "/git/cleanup-merged", "", request)
	if response.Code != http.StatusConflict {
		t.Fatalf("stale cleanup: %d %s", response.Code, response.Body)
	}
	request.SourceHead = repo.Head
	response = workflowResponse(t, h.CleanupMergedWorktree, "POST", "/git/cleanup-merged", "", request)
	var result app.GitMutationResult
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &result) != nil || result.RemovedWorktree != created.Path || result.RemovedBranch != "fix" {
		t.Fatalf("cleanup: %d %s", response.Code, response.Body)
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
	for _, action := range []http.HandlerFunc{handler.CreateWorktree, handler.Switch, handler.Merge, handler.AbortMerge, handler.DiscardWorktree, handler.CleanupMergedWorktree} {
		for _, body := range []string{`{}`, `{"confirm":true,"shell":"rm"}`, `{} {}`} {
			response := httptest.NewRecorder()
			action(response, httptest.NewRequest("POST", "/git", strings.NewReader(body)))
			if response.Code < 400 {
				t.Fatalf("accepted unsupported mutation: %s", body)
			}
		}
	}
}

func TestChangeOpenHTTPCreatesTheTaskInItsOwnWorktree(t *testing.T) {
	fixture := newChangeHandlerFixture(t, "http://127.0.0.1:1")
	root := fixture.manager.Root()
	git := func(args ...string) {
		t.Helper()
		flags := []string{"-C", root, "-c", "core.hooksPath=" + os.DevNull, "-c", "commit.gpgSign=false", "-c", "user.name=Test", "-c", "user.email=test@example.com"}
		if out, err := exec.Command("git", append(flags, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git: %v %s", err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".mini-orca/\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git("init", "-b", "main")
	git("add", "main.go", ".gitignore")
	git("commit", "-m", "Initial")
	if _, err := fixture.service.Reindex(); err != nil {
		t.Fatal(err)
	}
	index, _ := fixture.manager.Index()
	if _, err := fixture.service.TrustProjectExecution(context.Background(), index.ProjectRevision, true); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"project_id": index.ProjectID, "project_revision": index.ProjectRevision, "kind": "fix", "title": "Fix Run", "paths": []string{"main.go"}, "worktree": true, "confirm_execution": true}
	response := workflowResponse(t, fixture.Open, "POST", "/api/projects/current/changes", "", body)
	var opened struct {
		app.ChangeSession
		WorktreePath string   `json:"worktree_path"`
		Branch       string   `json:"branch"`
		Warnings     []string `json:"warnings"`
	}
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &opened) != nil || opened.ID == "" || opened.Title != "Fix Run" || !strings.HasPrefix(opened.Branch, "orca/fix-run-") || !strings.HasPrefix(opened.WorktreePath, filepath.Join(root, ".worktrees")) {
		t.Fatalf("open in a worktree = %d %s", response.Code, response.Body)
	}
	delete(body, "worktree")
	if response := workflowResponse(t, fixture.Open, "POST", "/api/projects/current/changes", "", body); response.Code != http.StatusOK || strings.Contains(response.Body.String(), "worktree_path") {
		t.Fatalf("open in this checkout = %d %s", response.Code, response.Body)
	}
}
