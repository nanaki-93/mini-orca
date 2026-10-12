package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func TestWorktreeRoutesPreserveProjectAndRequireScopedIdentity(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Fixture\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-b", "main"}, {"add", "README.md"}, {"-c", "user.name=Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgSign=false", "-c", "core.hooksPath=" + os.DevNull, "commit", "-m", "Initial"}} {
		if output, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git: %s, %v", output, err)
		}
	}
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	root = manager.Root()
	if err := manager.Set(root, &project.Analysis{Path: root, Name: "Fixture"}); err != nil {
		t.Fatal(err)
	}
	service, err := app.New(config.Default(), manager)
	if err != nil {
		t.Fatal(err)
	}
	index, _ := manager.Index()
	repository, err := service.GitRepository(context.Background(), index.ProjectID, index.ProjectRevision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.TrustProjectExecution(index.ProjectRevision, true); err != nil {
		t.Fatal(err)
	}
	created, err := service.CreateGitWorktree(context.Background(), app.GitMutationRequest{
		GitIdentity: repository.GitIdentity, ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision,
		Confirm: true, ConfirmExecution: true, TargetBranch: "codex/fix", Path: filepath.Join(root, ".worktrees", "fix"),
	})
	if err != nil || created.State != "created" || created.Error != "" {
		t.Fatalf("create: %+v, %v", created, err)
	}
	mux := newHTTPMux(service, manager)
	call := func(method, path, body, origin string) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request.Host = "localhost:9090"
		if body != "" {
			request.Header.Set("Content-Type", "application/json")
		}
		if origin != "" {
			request.Header.Set("Origin", origin)
		}
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		return response
	}
	scope := "?worktree_path=" + url.QueryEscape(created.Path)
	response := call("GET", "/api/projects/current"+scope, "", "")
	var child project.Analysis
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &child) != nil || child.ProjectID == index.ProjectID || child.Path != created.Path {
		t.Fatalf("scoped project: %d %s", response.Code, response.Body.String())
	}
	for _, checkout := range []struct {
		name, path, id, revision string
	}{
		{"current", "", index.ProjectID, index.ProjectRevision},
		{"worktree", created.Path, child.ProjectID, child.ProjectRevision},
	} {
		for _, endpoint := range []struct {
			name   string
			status int
		}{
			{"selection", http.StatusOK},
			{"run", http.StatusNoContent},
		} {
			t.Run(checkout.name+"/"+endpoint.name, func(t *testing.T) {
				query := url.Values{"project_id": {checkout.id}, "project_revision": {checkout.revision}}
				if checkout.path != "" {
					query.Set("worktree_path", checkout.path)
				}
				target := "/api/projects/current/analysis/" + endpoint.name + "?"
				got := call("GET", target+query.Encode(), "", "")
				if got.Code != endpoint.status {
					t.Fatalf("analysis read: %d %s", got.Code, got.Body.String())
				}
				if endpoint.name == "selection" {
					var selection app.AnalysisFileSelection
					if err := json.Unmarshal(got.Body.Bytes(), &selection); err != nil {
						t.Fatal(err)
					}
					if selection.ProjectID != checkout.id || selection.ProjectRevision != checkout.revision || !slices.ContainsFunc(selection.Files, func(file app.AnalysisSelectableFile) bool { return file.Path == "README.md" }) {
						t.Fatalf("wrong selection: %+v", selection)
					}
				}
				for _, extra := range []string{"&project_id=other", "&project_revision=other", "&unexpected=value", "&path=README.md", "&invalid=%ZZ", "&invalid=a;b"} {
					if got := call("GET", target+query.Encode()+extra, "", ""); got.Code != http.StatusBadRequest {
						t.Fatalf("invalid query %q: %d %s", extra, got.Code, got.Body.String())
					}
				}
				query.Del("project_revision")
				if got := call("GET", target+query.Encode(), "", ""); got.Code != http.StatusBadRequest {
					t.Fatalf("missing identity: %d %s", got.Code, got.Body.String())
				}
				query.Set("project_revision", "stale")
				if got := call("GET", target+query.Encode(), "", ""); got.Code != http.StatusConflict {
					t.Fatalf("stale identity: %d %s", got.Code, got.Body.String())
				}
				query.Set("project_revision", checkout.revision)
				otherID := child.ProjectID
				if checkout.path != "" {
					otherID = index.ProjectID
				}
				query.Set("project_id", otherID)
				if got := call("GET", target+query.Encode(), "", ""); got.Code != http.StatusConflict {
					t.Fatalf("another checkout's identity: %d %s", got.Code, got.Body.String())
				}
			})
		}
	}
	for _, test := range []struct {
		path, origin string
		status       int
	}{
		{"/api/projects/current?worktree_path=" + url.QueryEscape(t.TempDir()), "", http.StatusBadRequest},
		{"/api/projects/current" + scope + "&worktree_path=" + url.QueryEscape(created.Path), "", http.StatusBadRequest},
		{"/api/projects/current" + scope + "&worktree_path=" + url.QueryEscape(root), "", http.StatusBadRequest},
		{"/api/projects/current?worktree_path=", "", http.StatusBadRequest},
		{"/api/projects/current?worktree_path=%ZZ", "", http.StatusBadRequest},
		{"/api/projects/current" + scope + "&invalid=%ZZ", "", http.StatusBadRequest},
		{"/api/projects/current" + scope + "&invalid=a;b", "", http.StatusBadRequest},
		{"/api/models/current" + scope, "", http.StatusBadRequest},
		{"/api/projects/current" + scope, "https://example.invalid", http.StatusForbidden},
	} {
		if got := call("GET", test.path, "", test.origin); got.Code != test.status {
			t.Fatalf("scope boundary: %d %s", got.Code, got.Body.String())
		}
	}
	for _, id := range []string{index.ProjectID, child.ProjectID} {
		body, _ := json.Marshal(app.ChangeCreateRequest{ProjectID: id, ProjectRevision: child.ProjectRevision, Kind: "feature", Title: "Document behavior", Paths: []string{"README.md"}})
		got := call("POST", "/api/projects/current/changes"+scope, string(body), "")
		if (got.Code == http.StatusOK) != (id == child.ProjectID) {
			t.Fatalf("identity boundary: %d %s", got.Code, got.Body.String())
		}
	}
	current, _ := manager.Index()
	if manager.Root() != root || current.ProjectID != index.ProjectID || current.ProjectRevision != index.ProjectRevision {
		t.Fatal("scoped requests replaced the active project")
	}
	parentHistory, err := service.ChangeHistory(context.Background())
	if err != nil || len(parentHistory) != 0 {
		t.Fatalf("child history leaked: %+v, %v", parentHistory, err)
	}
}
