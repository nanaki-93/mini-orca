package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

func TestWorkspaceAgentStatesAcrossHubsAndPalettes(t *testing.T) {
	for _, state := range []string{"empty", "loading", "populated", "pending-input", "disconnected", "failed", "partial", "stale", "canceled"} {
		for _, palette := range []Palette{PaletteTokyo, PaletteCRT, PaletteAmber, PalettePaper} {
			for _, dims := range [][2]int{{52, 16}, {120, 36}} {
				t.Run(fmt.Sprintf("%s/%s/%dx%d", state, palette, dims[0], dims[1]), func(t *testing.T) {
					s := workspaceAgentState(state)
					s.Palette = palette
					for _, hub := range HubTabs() {
						for _, page := range hub.SubPages {
							s.Navigate(page.Page)
							frame := s.Render(dims[0], dims[1])
							assertFrameSize(t, frame, dims[0], dims[1])
							if !strings.Contains(frame, "[:] Commands") || !strings.Contains(frame, "[?] Help") {
								t.Fatalf("actions hidden on %s", page.Page)
							}
							s.HandleKey(KeyEvent{Key: "End"})
							assertFrameSize(t, s.Render(dims[0], dims[1]), dims[0], dims[1])
							s.HandleKey(KeyEvent{Key: "Home"})
							if len(s.PendingActions) != 0 || s.Confirmation != nil {
								t.Fatalf("browsing %s dispatched an action", page.Page)
							}
						}
					}
				})
			}
		}
	}
}

func workspaceAgentState(state string) *State {
	if state == "empty" {
		return NewState()
	}
	s, detail := executionState()
	s.Connected = state != "disconnected"
	s.Project.Name = "Agent workspace"
	if state == "loading" {
		s.Busy = "Loading recorded progress"
		s.AgentProgress = nil
		return s
	}
	if state == "pending-input" {
		pendingInputFixture(s)
		detail.Run.Status = "waiting_input"
	} else if state != "populated" && state != "disconnected" {
		s.Change.Workflow.Status = state
		detail.Run.Status = state
		detail.Run.Active, detail.Run.CanCancel = false, false
		detail.Run.Stale = state == "stale"
	}
	if state == "disconnected" {
		s.AgentStreamStatus = "disconnected"
	}
	s.applyAgentProgress(&agentProgressView{Request: *s.agentProgressRequest(), Detail: &detail, Status: "current"})
	return s
}

func TestWorkspaceFitsActualTerminalAndKeepsActiveNavigation(t *testing.T) {
	for _, dims := range [][2]int{{1, 1}, {40, 10}, {52, 16}, {60, 20}, {80, 24}, {99, 30}, {100, 16}, {100, 20}, {120, 40}, {320, 125}} {
		for _, hub := range HubTabs() {
			for _, page := range hub.SubPages {
				s := NewState()
				s.Navigate(page.Page)
				frame := s.Render(dims[0], dims[1])
				assertFrameSize(t, frame, dims[0], dims[1])
				if dims[0] >= 52 && dims[1] >= 16 && page.Page != PageIssueDetail {
					if !strings.Contains(frame, "▸ "+page.Label) || !strings.Contains(frame, "[?] Help") {
						t.Fatalf("active page or help hidden at %v on %s:\n%s", dims, page.Page, frame)
					}
				}
				if len(s.PendingActions) != 0 {
					t.Fatal("rendering must not dispatch work")
				}
			}
		}
	}
}

func assertFrameSize(t *testing.T, frame string, width, height int) {
	t.Helper()
	rows := strings.Split(frame, "\n")
	if len(rows) != height {
		t.Fatalf("expected %d rows, got %d", height, len(rows))
	}
	for i, row := range rows {
		if got := theme.VisibleRuneCount(row); got != width {
			t.Fatalf("row %d: expected width %d, got %d", i, width, got)
		}
	}
}

func TestWorkspaceStatusAndCommandRemainVisible(t *testing.T) {
	s := NewState()
	s.Project = &Project{Name: strings.Repeat("長いproject", 30)}
	s.Repository = &GitRepository{Available: true, GitIdentity: GitIdentity{Branch: "feature/" + strings.Repeat("long", 30)}}
	s.GitWorkspace = &GitWorkspace{GitRepository: GitRepository{Dirty: true}}
	s.WorkspaceTrust = true
	for _, connected := range []bool{false, true} {
		s.Connected = connected
		frame := s.Render(100, 24)
		assertFrameSize(t, frame, 100, 24)
		for _, token := range []string{"Modified", "Checks trusted", "[?] Help", "[:] Commands"} {
			if !strings.Contains(frame, token) {
				t.Fatalf("status %q disappeared with a long project name", token)
			}
		}
		if connected != strings.Contains(frame, "Connected") {
			t.Fatal("incorrect connection state")
		}
	}
	s.openCommandInput("project /" + strings.Repeat("nested/", 30) + "visible-tail")
	if frame := s.Render(80, 24); !strings.Contains(frame, "visible-tail") {
		t.Fatal("long command must keep its cursor and tail visible")
	}
}
