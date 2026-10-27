package tui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
)

func progressState() (*State, AgentRunDetail) {
	s := NewState()
	s.Project = &Project{ProjectIdentity: ProjectIdentity{ProjectID: "main", ProjectRevision: "main-rev"}, Name: "example"}
	s.Change = &ChangeSession{ID: "change", ProjectIdentity: ProjectIdentity{ProjectID: "checkout", ProjectRevision: "rev"}, Title: "Improve validation", WorktreePath: "/workspace/task", Workflow: &ChangeWorkflow{ID: "workflow", Status: "running", Stages: []ChangeWorkflowStage{
		{Name: "create", Status: "completed"}, {Name: "generate_tests", Status: "completed"}, {Name: "verify", Status: "running"}, {Name: "review", Status: "pending"}, {Name: "human_review", Status: "blocked"},
	}}}
	run := AgentRun{ID: "run", RootID: "run", ProjectID: "checkout", ProjectRevision: "rev", ChangeSessionID: "change", WorkflowID: "workflow", Status: "running", Active: true}
	return s, AgentRunDetail{Run: run, Events: []AgentRunEvent{{Sequence: 5, RunID: "run", InvocationID: "run", ProjectID: "checkout", ProjectRevision: "rev", Kind: "node", NodeID: "verify", State: "running"}}}
}

func TestProgressReadsExactCheckoutWithoutPrivateText(t *testing.T) {
	s, detail := progressState()
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		q := r.URL.Query()
		if r.Method != http.MethodGet || q.Get("worktree_path") != "/workspace/task" || q.Get("project_id") != "checkout" || q.Get("project_revision") != "rev" || q.Has("include_text") {
			t.Errorf("unguarded progress request: %s %s", r.Method, r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/run") {
			_ = json.NewEncoder(w).Encode(detail)
			return
		}
		foreign := detail.Run
		foreign.ID = "foreign"
		foreign.RootID = "foreign"
		foreign.WorkflowID = "old"
		_ = json.NewEncoder(w).Encode(AgentRunList{ProjectID: "checkout", ProjectRevision: "rev", Runs: []AgentRun{foreign, detail.Run}})
	}))
	defer srv.Close()
	cl, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	s.refreshAgentProgress(context.Background(), cl)
	if s.AgentProgress == nil || s.AgentProgress.Status != "current" || len(paths) != 2 {
		t.Fatalf("progress: %+v; requests %v", s.AgentProgress, paths)
	}
	s.refreshAgentProgress(context.Background(), cl)
	if len(paths) != 3 || paths[2] != "/api/projects/current/agent-runs/run" {
		t.Fatalf("cached identity was not reused: %v", paths)
	}
}

func TestProgressUnavailableAndUnrecordedRemainExplicit(t *testing.T) {
	for _, kind := range []string{"failed", "absent", "foreign", "private"} {
		t.Run(kind, func(t *testing.T) {
			s, detail := progressState()
			request := *s.agentProgressRequest()
			if kind == "foreign" || kind == "private" {
				request.RunID = "run"
			}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch kind {
				case "failed":
					http.Error(w, "journal unavailable", 500)
				case "absent":
					_ = json.NewEncoder(w).Encode(AgentRunList{ProjectID: "checkout", ProjectRevision: "rev"})
				case "foreign":
					detail.Run.ProjectRevision = "old"
					_ = json.NewEncoder(w).Encode(detail)
				case "private":
					_, _ = w.Write([]byte(`{"run":{"id":"run","root_id":"run","project_id":"checkout","project_revision":"rev","change_session_id":"change","workflow_id":"workflow"},"text":{"text":"private"}}`))
				}
			}))
			defer srv.Close()
			cl, err := NewClient(srv.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer cl.Close()
			result := readAgentProgress(context.Background(), cl, request)
			want := "unavailable"
			if kind == "absent" {
				want = "not_recorded"
			}
			if result.Status != want || result.Detail != nil {
				t.Fatalf("unexpected disclosure or false success: %+v", result)
			}
			if kind != "absent" && result.Error == "" {
				t.Fatal("failure was swallowed")
			}
		})
	}
}

func TestProgressPreservesSelectionAndRejectsLateResponses(t *testing.T) {
	s, detail := progressState()
	request := *s.agentProgressRequest()
	s.selectAgentStage(4)
	s.DetailScroll = 7
	s.applyAgentProgress(&agentProgressView{Request: request, Detail: &detail, Status: "current"})
	s.Change.Workflow.Stages = append([]ChangeWorkflowStage{{Name: "new_node", Status: "pending"}}, s.Change.Workflow.Stages...)
	if stages := s.currentAgentStages(); stages[s.effectiveAgentDetailStage()-1].ID != "review" || s.DetailScroll != 7 {
		t.Fatal("progress moved selection or scroll")
	}
	older := detail
	older.Events = []AgentRunEvent{{Sequence: 4}}
	s.applyAgentProgress(&agentProgressView{Request: request, Detail: &older, Status: "current"})
	if progressSequence(s.AgentProgress.Detail) != 5 {
		t.Fatal("old progress overwrote newer evidence")
	}
	for _, field := range []string{"project", "workflow", "checkout", "change"} {
		t.Run(field, func(t *testing.T) {
			s, detail := progressState()
			request := *s.agentProgressRequest()
			switch field {
			case "project":
				s.Project.ProjectRevision = "other"
			case "workflow":
				s.Change.Workflow.ID = "other"
			case "checkout":
				s.Change.WorktreePath = "/other"
			case "change":
				s.Change.ID = "other"
			}
			s.applyAgentProgress(&agentProgressView{Request: request, Detail: &detail, Status: "current"})
			if s.AgentProgress != nil {
				t.Fatal("late progress crossed identity boundary")
			}
		})
	}
}

func TestPollCannotRestoreReplacedWorkflowOrProject(t *testing.T) {
	s, _ := progressState()
	s.AgentDetailOpen = true
	snap, ok := s.capturePollSnapshot()
	if !ok || snap.progress == nil {
		t.Fatal("inspection omitted native progress")
	}
	old := *s.Change
	result := pollResult{parent: snap.parent, workflows: snap.workflows, changes: []ChangeSession{old}, changeOK: []bool{true}}
	replacement := *s.Change.Workflow
	replacement.ID = "replacement"
	s.Change.Workflow = &replacement
	s.applyPollResult(result)
	if s.Change.Workflow.ID != "replacement" {
		t.Fatal("late poll restored old workflow")
	}
	s.Project.ProjectRevision = "new-project"
	s.applyPollResult(result)
	if s.Change.Workflow.ID != "replacement" {
		t.Fatal("late poll crossed project revision")
	}
}

func TestDynamicStageViewsFitPalettesAndTerminalSizes(t *testing.T) {
	for _, palette := range []Palette{PaletteTokyo, PaletteCRT, PaletteAmber, PalettePaper} {
		for _, dims := range [][2]int{{40, 10}, {52, 16}, {80, 24}, {100, 20}, {160, 40}} {
			s, detail := progressState()
			s.Palette = palette
			s.Connected = true
			s.Repository = &GitRepository{Available: true, GitIdentity: GitIdentity{Branch: "work"}}
			s.Navigate(PageAgents)
			s.AgentDetailOpen = true
			s.selectAgentStage(3)
			s.applyAgentProgress(&agentProgressView{Request: *s.agentProgressRequest(), Detail: &detail, Status: "current"})
			frame := s.Render(dims[0], dims[1])
			assertFrameSize(t, frame, dims[0], dims[1])
			if dims[0] >= 52 && dims[1] >= 16 {
				for _, token := range []string{"Task · stages", "Verify", ": commands"} {
					if !strings.Contains(frame, token) {
						t.Fatalf("%v %s missing %q: %s", dims, palette, token, frame)
					}
				}
			}
			if len(s.PendingActions) != 0 || s.AgentStageID != "verify" || domain.StageSummary(s.currentAgentStages()) != "Verify" {
				t.Fatal("passive render changed task state")
			}
		}
	}
}
