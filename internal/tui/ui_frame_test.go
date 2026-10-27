package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

// demoWorkspace is a project mid-flight: one task waiting for review, one
// with agents running, one failed, a branch ready to merge and a handbook.
func demoWorkspace() *State {
	s := workspaceSampleState()
	started := time.Now().Add(-6 * time.Minute).UTC().Format(time.RFC3339Nano)
	mid := time.Now().Add(-4 * time.Minute).UTC().Format(time.RFC3339Nano)
	done := time.Now().Add(-90 * time.Second).UTC().Format(time.RFC3339Nano)
	review := &ChangeSession{
		ID: "chg-24", ProjectIdentity: ProjectIdentity{ProjectID: "fixture", ProjectRevision: "rev-1"}, Kind: "fix", Revision: 3, Hash: "sha256:77ab91c2",
		Title: "Propagate request deadlines to the storage client", State: "checked", Freshness: "current",
		AcceptanceCriteria: []string{"Canceled requests stop the storage call", "A test covers the deadline"},
		Targets:            []ChangeTarget{{Path: "internal/storage/client.go"}, {Path: "internal/storage/client_test.go"}},
		Changes: []FileChange{
			{Path: "internal/storage/client.go", Summary: "Pass the request context to the storage call.", Diff: FileDiff{Lines: []DiffLine{
				{Kind: "context", OldLine: 80, NewLine: 80, Text: "func (c *Client) Get(ctx context.Context, key string) ([]byte, error) {"},
				{Kind: "removed", OldLine: 81, Text: "\treq, err := http.NewRequest(http.MethodGet, c.url(key), nil)"},
				{Kind: "added", NewLine: 81, Text: "\treq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url(key), nil)"},
				{Kind: "context", OldLine: 82, NewLine: 82, Text: "\tif err != nil {"},
				{Kind: "context", OldLine: 83, NewLine: 83, Text: "\t\treturn nil, err"},
				{Kind: "context", OldLine: 84, NewLine: 84, Text: "\t}"},
			}}},
			{Path: "internal/storage/client_test.go", Diff: FileDiff{Lines: []DiffLine{
				{Kind: "added", NewLine: 40, Text: "func TestGetStopsWhenTheRequestIsCanceled(t *testing.T) {"},
				{Kind: "added", NewLine: 41, Text: "\tctx, cancel := context.WithCancel(context.Background())"},
				{Kind: "added", NewLine: 42, Text: "\tcancel()"},
				{Kind: "added", NewLine: 43, Text: "}"},
			}}},
		},
		ReviewedFiles: []string{"internal/storage/client_test.go"}, FileReviewHash: "sha256:77ab91c2",
		Checks: []Check{{Name: "gofmt", Required: true, State: "passed"}, {Name: "go vet", Required: true, State: "passed"}, {Name: "go test", Required: true, State: "failed", ExitCode: 1, Output: "--- FAIL: TestGetStopsWhenTheRequestIsCanceled"}},
		Git:    &GitIdentity{Branch: "main", Head: "9694f66"},
		Workflow: &ChangeWorkflow{ID: "wf-1", Status: "awaiting_human_review", Attempt: 1, AutoRepair: true, StartedAt: started, UpdatedAt: done,
			Stages: []ChangeWorkflowStage{
				{Name: "create", Status: "completed", StartedAt: started, FinishedAt: mid, Model: &Model{Model: "qwen3-coder-30b"}},
				{Name: "test", Status: "completed", StartedAt: mid, FinishedAt: done, Reason: "Wrote 1 test"},
				{Name: "review", Status: "completed", StartedAt: done, FinishedAt: done},
				{Name: "human_review", Status: "waiting"},
			},
			Specialists: []client.ChangeSpecialistReview{
				{ID: "correctness", Status: "completed", Evidence: "static review", Review: &client.ChangeWorkflowReview{Verdict: "approve", Summary: "The context reaches the transport."}},
				{ID: "test_quality", Status: "completed", Evidence: "tests executed", Review: &client.ChangeWorkflowReview{Verdict: "approve", Summary: "The test cancels before the call."}},
			},
		},
	}
	running := &ChangeSession{ID: "chg-25", Title: "Validate upload paths before opening files", Kind: "security", State: "draft", Freshness: "current",
		Targets:  []ChangeTarget{{Path: "internal/api/uploads.go"}, {Path: "internal/api/uploads_test.go"}},
		Workflow: &ChangeWorkflow{Status: "running", StartedAt: mid, Stages: []ChangeWorkflowStage{{Name: "create", Status: "running", StartedAt: mid}}}}
	failed := &ChangeSession{ID: "chg-26", Title: "Avoid rebuilding the route index on every request", Kind: "performance", State: "draft", Freshness: "current",
		Workflow: &ChangeWorkflow{Status: "failed", Reason: "The testing agent could not make the new benchmark pass.", StartedAt: started, UpdatedAt: done,
			Stages: []ChangeWorkflowStage{{Name: "create", Status: "completed", StartedAt: started, FinishedAt: mid}, {Name: "test", Status: "failed", StartedAt: mid, FinishedAt: done, Reason: "benchmark failed"}}}}
	s.Change = review
	s.ActiveChanges = []*ChangeSession{review, running, failed}
	s.GitWorkspace = &GitWorkspace{
		GitRepository: GitRepository{GitIdentity: GitIdentity{Head: "9694f66", Branch: "main"}, Available: true},
		Worktrees: []GitWorktree{
			{GitIdentity: GitIdentity{Head: "9694f66", Branch: "main"}, Path: "/workspace/atlas", Current: true},
			{GitIdentity: GitIdentity{Head: "3b9e1c2", Branch: "orca/validate-requests"}, Path: "/workspace/atlas/.worktrees/validate", Mergeable: true,
				Completed: []ChangeHistoryEntry{{Title: "Validate incoming requests", State: "applied"}}},
		},
	}
	s.Index = &ProjectIndex{Files: []IndexedFile{{Path: "cmd/atlas/main.go", Language: "go", LineCount: 42}, {Path: "internal/api/uploads.go", Language: "go", LineCount: 120}, {Path: "internal/storage/client.go", Language: "go", LineCount: 210}}}
	s.Overview = &Overview{Docs: &ProjectDocsReport{Status: "fresh", UpdatedAt: done,
		Context:      ProjectDocsContext{Purpose: "Atlas stores and serves project artifacts over HTTP.", Topology: "An HTTP API in front of a storage client.", Stack: []string{"Go 1.27", "net/http"}, GoalPursuit: []string{"Keep uploads inside the workspace", "Bound every storage call"}},
		Dependencies: []ProjectDocsDependency{{Name: "golang.org/x/sync", Kind: "library", Version: "v0.10.0", Purpose: "Bounded concurrency"}},
		Components:   []ProjectDocsComponent{{Name: "API", Layer: "transport", Path: "internal/api", Summary: "HTTP handlers.", Responsibilities: []string{"Validate requests"}}, {Name: "Storage", Layer: "infrastructure", Path: "internal/storage", Summary: "Storage client."}},
		Decisions:    []ProjectDocsDecision{{Title: "Context on every call", Category: "reliability", Decision: "Every storage call takes the request context.", Tradeoff: "More parameters."}},
	}}
	s.Navigate(PageSummary)
	return s
}

type frameCase struct {
	name  string
	setup func(*State)
}

func frameCases() []frameCase {
	return []frameCase{
		{"inbox", func(s *State) {}},
		{"tasks", func(s *State) { s.Navigate(PageAgents) }},
		{"task-stages", func(s *State) { s.Navigate(PageAgents); s.HandleKey(KeyEvent{Key: "Enter"}) }},
		{"review", func(s *State) { s.Navigate(PageAgents); s.HandleKey(KeyEvent{Key: "d"}) }},
		{"review-split", func(s *State) { s.Navigate(PageAgents); s.HandleKey(KeyEvent{Key: "d"}); s.setDiffViewMode("split") }},
		{"explorer", func(s *State) { s.Change = nil; s.ActiveChanges = nil; s.Navigate(PageEditor) }},
		{"issues", func(s *State) { s.Navigate(PageBugs); s.IssueView = "all"; s.SelectedIdx = 1 }},
		{"docs", func(s *State) { s.Navigate(PageDocs) }},
		{"components", func(s *State) { s.Navigate(PageDocsComponents) }},
		{"branches", func(s *State) { s.Navigate(PageGit); s.SelectedIdx = 1 }},
		{"settings", func(s *State) { s.Navigate(PageModels) }},
		{"commands", func(s *State) { s.openCommandInput("re") }},
		{"help", func(s *State) { s.Navigate(PageAgents); s.ShowKeybindsModal = true }},
		{"confirm", func(s *State) { s.requestTrustChange(); s.WorkspaceTrust = false; s.requestTrustChange() }},
		{"error", func(s *State) {
			s.Error = "The daemon rejected the request: the project changed since the proposal was made."
		}},
		{"welcome", func(s *State) { *s = *NewState(); s.Connected = true }},
	}
}

// Every workspace and overlay fills the frame exactly at common sizes, so
// nothing wraps or leaves stale cells in a real terminal.
func TestEveryViewFillsTheFrameExactly(t *testing.T) {
	for _, c := range frameCases() {
		for _, size := range [][2]int{{52, 16}, {60, 16}, {80, 24}, {100, 30}, {120, 36}, {160, 44}, {220, 60}} {
			s := demoWorkspace()
			c.setup(s)
			assertFrameSize(t, s.Render(size[0], size[1]), size[0], size[1])
		}
	}
}

// MINI_ORCA_TUI_FRAMES=<dir> writes every case as ANSI frames for visual
// review (palette and size in the file name).
func TestWriteFramesForVisualReview(t *testing.T) {
	dir := os.Getenv("MINI_ORCA_TUI_FRAMES")
	if dir == "" {
		t.Skip("set MINI_ORCA_TUI_FRAMES to write frames")
	}
	for _, c := range frameCases() {
		for _, palette := range []Palette{PaletteTokyo, PalettePaper} {
			s := demoWorkspace()
			s.Palette = palette
			c.setup(s)
			frame := s.Render(150, 42)
			name := filepath.Join(dir, string(palette)+"-"+c.name+".ansi")
			if err := os.WriteFile(name, []byte(frame), 0o644); err != nil {
				t.Fatal(err)
			}
			if palette == PaletteTokyo {
				narrow := filepath.Join(dir, "narrow-"+c.name+".ansi")
				if err := os.WriteFile(narrow, []byte(s.Render(80, 24)), 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

func TestFrameNamesProjectCheckoutAndWorkspaces(t *testing.T) {
	s := demoWorkspace()
	frame := ansi.Strip(s.Render(150, 40))
	rows := strings.Split(frame, "\n")
	for _, token := range []string{"mini-orca", "atlas", "⎇ main", "checks trusted", "live"} {
		if !strings.Contains(rows[0], token) {
			t.Fatalf("header missing %q: %q", token, rows[0])
		}
	}
	for _, token := range []string{"1 Inbox", "2 Tasks", "3 Code", "4 Issues", "5 Docs", "6 Branches", "7 Settings"} {
		if !strings.Contains(rows[1], token) {
			t.Fatalf("workspace tabs missing %q: %q", token, rows[1])
		}
	}
	for key, page := range map[string]Page{"1": PageSummary, "2": PageAgents, "3": PageEditor, "4": PageBugs, "5": PageDocs, "6": PageGit, "7": PageInstructions} {
		s.HandleKey(KeyEvent{Key: key})
		if s.Page != page {
			t.Fatalf("key %s opened %s, want %s", key, s.Page, page)
		}
	}
}

func TestInboxPutsTasksThatNeedYouFirstAndOpensThem(t *testing.T) {
	s := demoWorkspace()
	frame := ansi.Strip(s.Render(150, 40))
	needs := strings.Index(frame, "NEEDS YOU")
	working := strings.Index(frame, "AGENTS WORKING")
	failed := strings.Index(frame, "FAILED OR STALE")
	merge := strings.Index(frame, "READY TO MERGE")
	if needs < 0 || working < needs || failed < working || merge < failed {
		t.Fatalf("inbox groups out of order:\n%s", frame)
	}
	if !strings.Contains(frame, "READY TO APPLY?") || !strings.Contains(frame, "Files reviewed") {
		t.Fatalf("the selected task's gate must show beside the inbox:\n%s", frame)
	}
	s.HandleKey(KeyEvent{Key: "Enter"})
	if s.Page != PageAgents || s.Change == nil || s.Change.ID != "chg-24" || len(s.PendingActions) != 0 {
		t.Fatalf("Enter on a task must open it in Tasks without starting work: page=%s change=%v actions=%v", s.Page, s.Change, s.PendingActions)
	}
	s.Navigate(PageSummary)
	for range 4 {
		s.HandleKey(KeyEvent{Key: "j"})
	}
	s.HandleKey(KeyEvent{Key: "Enter"})
	if s.Page != PageBugs {
		t.Fatalf("the Issues row opens Issues, got %s", s.Page)
	}
}

func TestTaskDetailShowsGateStagesChecksAndFiles(t *testing.T) {
	s := demoWorkspace()
	s.Navigate(PageAgents)
	frame := ansi.Strip(s.Render(160, 48))
	for _, token := range []string{
		"needs review", "agents working", "failed",
		"READY TO APPLY?", "Agents finished", "Reviewers", "2 approved", "Files reviewed", "1 of 2", "Checks", "2/3 passed",
		"AGENT RUN", "Create", "waiting for you",
		"✖ go test", "CHANGED FILES", "internal/storage/client.go", "DONE WHEN", "Canceled requests stop the storage call",
	} {
		if !strings.Contains(frame, token) {
			t.Fatalf("task detail missing %q:\n%s", token, frame)
		}
	}
}

func TestReviewShowsFilesBesideTheDiff(t *testing.T) {
	s := demoWorkspace()
	s.Navigate(PageAgents)
	s.HandleKey(KeyEvent{Key: "d"})
	if !s.isViewingDiff() {
		t.Fatal("d opens the diff")
	}
	frame := ansi.Strip(s.Render(160, 40))
	for _, token := range []string{"Changed files", "1/2 viewed", "client.go", "client_test.go", "NewRequestWithContext", "hunk 1/2"} {
		if !strings.Contains(frame, token) {
			t.Fatalf("review missing %q:\n%s", token, frame)
		}
	}
	s.HandleKey(KeyEvent{Key: "w"})
	if len(s.PendingActions) != 1 {
		t.Fatalf("w marks the file reviewed through the daemon: %+v", s.PendingActions)
	}
}

func TestOverlaysFloatOverTheWorkspace(t *testing.T) {
	s := demoWorkspace()
	s.Navigate(PageAgents)
	s.openCommandInput("")
	frame := ansi.Strip(s.Render(150, 40))
	if !strings.Contains(frame, "Commands") || !strings.Contains(frame, "Tasks") {
		t.Fatalf("the palette floats over the workspace, which stays visible:\n%s", frame)
	}
	s.CmdActive = false
	s.WorkspaceTrust = false
	s.requestTrustChange()
	frame = ansi.Strip(s.Render(150, 40))
	if !strings.Contains(frame, "Trust this project?") || !strings.Contains(frame, "Trust project") {
		t.Fatalf("confirmation dialog missing:\n%s", frame)
	}
}

func TestNarrowTerminalsStackListAndDetail(t *testing.T) {
	s := demoWorkspace()
	s.Navigate(PageAgents)
	frame := ansi.Strip(s.Render(80, 40))
	list := strings.Index(frame, "Propagate request deadlines")
	gate := strings.Index(frame, "READY TO APPLY?")
	if list < 0 || gate < 0 || gate < list {
		t.Fatalf("narrow layout must stack the detail under the list:\n%s", frame)
	}
	small := s.Render(50, 12)
	if !strings.Contains(ansi.Strip(small), "Enlarge the terminal") {
		t.Fatalf("tiny terminals ask for more room:\n%s", ansi.Strip(small))
	}
}
