package tui

import (
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

type viewportMemory struct {
	width, height   int
	followSelection bool
	manualScroll    bool
	dialogScroll    int
	dialogKey       string
	// lists keeps each split view's list scroll so the cursor moves within a
	// stable window instead of re-centering on every key.
	lists map[Page]int
}

// listScroll returns the scroll offset of the active page's list pane that
// keeps cursorRow visible.
func (s *State) listScroll(cursorRow, total, rows int) int {
	if s.viewport.lists == nil {
		s.viewport.lists = map[Page]int{}
	}
	page := s.EffectivePage()
	next := theme.FollowScroll(s.viewport.lists[page], cursorRow, total, rows)
	s.viewport.lists[page] = next
	return next
}

func (s *State) isReadingPage() bool {
	return isHandbookPage(s.Page) || s.Page == PageRepo
}

func (s *State) handleDetailScrollKey(ev KeyEvent) bool {
	step := max(1, s.viewportBodyRows-2)
	if s.Page == PageEditor && s.isViewingDiff() {
		step = s.diffMetadataRows()
	}
	switch ev.Key {
	case "PageDown":
		s.DetailScroll += step
	case "PageUp":
		s.DetailScroll = max(0, s.DetailScroll-step)
	case "Home":
		s.DetailScroll = 0
	case "End":
		s.DetailScroll = 1 << 30
	default:
		return false
	}
	s.viewport.followSelection = false
	s.viewport.manualScroll = true
	return true
}

func wrapWorkspaceLines(lines []string, width int) []string {
	var wrapped []string
	for _, line := range lines {
		if theme.VisibleRuneCount(line) <= width {
			wrapped = append(wrapped, line)
			continue
		}
		for _, row := range theme.WrapVisibleLine(strings.TrimLeft(line, " "), width-4) {
			wrapped = append(wrapped, "  "+row)
		}
	}
	return wrapped
}

func (s *State) handleDialogScrollKey(ev KeyEvent) bool {
	if s.Confirmation == nil && s.Error == "" {
		return false
	}
	switch ev.Key {
	case "PageDown":
		s.viewport.dialogScroll += max(1, s.viewportBodyRows-2)
	case "PageUp":
		s.viewport.dialogScroll = max(0, s.viewport.dialogScroll-max(1, s.viewportBodyRows-2))
	case "Home":
		s.viewport.dialogScroll = 0
	case "End":
		s.viewport.dialogScroll = 1 << 30
	default:
		return false
	}
	return true
}

func (s *State) handleErrorOverlayKey(ev KeyEvent) {
	switch ev.Key {
	case "Escape":
		s.Error = ""
		s.CmdOutput = "Dismissed error."
	case ":", "/":
		s.openCommandInput("")
	case "?":
		s.ShowKeybindsModal, s.KeybindsScroll = true, 0
	case "1", "2", "3", "4", "5", "6", "7":
		s.switchHubByKey(ev.Key)
	}
}

func (s *State) handleSourceViewportKey(key string) bool {
	if s.Page != PageEditor || s.BrowseFiles || s.File == nil {
		return false
	}
	switch key {
	case "h", "ArrowLeft":
		s.DiffOffset = max(0, s.DiffOffset-12)
	case "l", "ArrowRight":
		s.DiffOffset += 12
	case "0":
		s.DiffOffset = 0
	case "PageDown":
		s.moveSelection(s.adaptiveWindowRows(10, 14))
	case "PageUp":
		s.moveSelection(-s.adaptiveWindowRows(10, 14))
	case "Home":
		s.SelectedIdx = 0
	case "End":
		s.SelectedIdx = max(0, s.ActiveListLength()-1)
	default:
		return false
	}
	return true
}
