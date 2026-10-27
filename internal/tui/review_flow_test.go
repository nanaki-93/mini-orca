package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestWideDiffShowsFileListWithReviewState(t *testing.T) {
	s := twoFileReviewState()
	s.Change.FileReviewHash, s.Change.ReviewedFiles = s.Change.Hash, []string{"internal/api/server_test.go"}
	wide := ansi.Strip(s.Render(170, 34))
	for _, token := range []string{"Changed files", "1/2 viewed", "○ server.go", "✔ server_test.go", "+1 −0", "var T = 5"} {
		if !strings.Contains(wide, token) {
			t.Fatalf("wide diff missing %q:\n%s", token, wide)
		}
	}
	assertFrameSize(t, s.Render(170, 34), 170, 34)
	narrow := ansi.Strip(s.Render(80, 34))
	files, code := strings.Index(narrow, "Changed files"), strings.Index(narrow, "var T = 5")
	if files < 0 || code < 0 || code < files {
		t.Fatalf("a narrow diff stacks the file list above the code:\n%s", narrow)
	}
}

func TestMergingAWorktreeShowsAPreviewFirst(t *testing.T) {
	s := workspaceSampleState()
	s.GitWorkspace = &GitWorkspace{
		GitRepository: GitRepository{GitIdentity: GitIdentity{Head: "base", Branch: "main"}, Available: true},
		Worktrees: []GitWorktree{
			{GitIdentity: GitIdentity{Head: "base", Branch: "main"}, Path: "/repo", Current: true},
			{
				GitIdentity: GitIdentity{Head: "c1", Branch: "orca/fix"}, Path: "/repo/.worktrees/fix", Mergeable: true,
				Integration: "behind", OverlapPaths: []string{"internal/api/server.go"},
				Completed: []ChangeHistoryEntry{{Title: "Fix", FindingTargets: []FindingTarget{{}}}},
			},
		},
	}
	s.Navigate(PageGit)
	s.SelectedIdx = 1
	s.HandleKey(KeyEvent{Key: "m"})
	if s.Confirmation == nil || len(s.PendingActions) != 0 {
		t.Fatal("merge must preview before it runs")
	}
	for _, token := range []string{"orca/fix", "main", "1 committed task", "1 linked issue", "internal/api/server.go", "conflict"} {
		if !strings.Contains(s.Confirmation.Message+s.Confirmation.Title, token) {
			t.Fatalf("merge preview missing %q: %s", token, s.Confirmation.Message)
		}
	}
	s.HandleKey(KeyEvent{Key: "n"})
	if len(s.PendingActions) != 0 {
		t.Fatal("canceling the preview must not merge")
	}
	s.HandleKey(KeyEvent{Key: "m"})
	s.HandleKey(KeyEvent{Key: "y"})
	if len(s.PendingActions) != 1 || s.PendingActions[0].Kind != "merge-worktree" || s.PendingActions[0].Target != "/repo/.worktrees/fix" {
		t.Fatalf("confirmed merge not queued: %+v", s.PendingActions)
	}
}

func TestApplyWarnsWhichRunningTasksItStops(t *testing.T) {
	s := twoFileReviewState()
	s.Change.Checks = []Check{{Name: "tests", Required: true, State: "passed"}}
	s.ActiveChanges = []*ChangeSession{{ID: "chg-other", Title: "Speed up indexing", Workflow: &ChangeWorkflow{Status: "running"}}}
	s.HandleKey(KeyEvent{Key: "a"})
	if s.Confirmation == nil || len(s.PendingActions) != 0 {
		t.Fatal("Apply must warn before it stops another running task")
	}
	if !strings.Contains(s.Confirmation.Message, "Speed up indexing") {
		t.Fatalf("the warning must name the task it stops: %s", s.Confirmation.Message)
	}
	s.HandleKey(KeyEvent{Key: "y"})
	if len(s.PendingActions) != 1 || s.PendingActions[0].Kind != "apply-commit" {
		t.Fatalf("confirmed Apply not queued: %+v", s.PendingActions)
	}

	quiet := twoFileReviewState()
	quiet.Change.Checks = s.Change.Checks
	quiet.HandleKey(KeyEvent{Key: "a"})
	if quiet.Confirmation != nil || len(quiet.PendingActions) != 1 {
		t.Fatal("with nothing else running, [a] applies directly")
	}
}
