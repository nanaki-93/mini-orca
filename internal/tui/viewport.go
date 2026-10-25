package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

type viewportMemory struct {
	width, height   int
	followSelection bool
	manualScroll    bool
	dialogScroll    int
	dialogKey       string
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

func (s *State) scrollDetails(th ansiTheme, body []string, rows int) []string {
	if s.Page == PageEditor && s.isViewingDiff() {
		return body
	}
	if s.viewport.followSelection {
		s.viewport.followSelection = false
		s.viewport.manualScroll = false
		marker := "  " + th.renderCursorPrefix(true)
		for i, line := range body {
			if strings.HasPrefix(line, marker) {
				if i < s.DetailScroll || i >= s.DetailScroll+max(1, rows-1) {
					s.DetailScroll = max(0, i-max(1, rows-1)/2)
				}
				break
			}
		}
	}
	return scrollWindow(th, body, rows, &s.DetailScroll)
}

func scrollWindow(th ansiTheme, body []string, rows int, scroll *int) []string {
	if len(body) <= rows {
		*scroll = 0
		return body
	}
	visible := max(1, rows-1)
	*scroll = min(max(0, *scroll), max(0, len(body)-visible))
	end := min(len(body), *scroll+visible)
	out := append([]string(nil), body[*scroll:end]...)
	return append(out, fmt.Sprintf("  %sPgUp/PgDn · lines %d–%d of %d%s", th.muted, *scroll+1, end, len(body), th.reset))
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

func (s *State) renderDialog(th ansiTheme, width, rows int) []string {
	key := s.Error
	var body []string
	if s.Confirmation != nil {
		key = s.Confirmation.Title + "\x00" + s.Confirmation.Message
		body = s.renderConfirmationModal(th, width)
	} else {
		body = th.renderErrorBanner(s.Error, width-4)
		body = append(body, "", "  [Esc] Dismiss · [:] Commands")
		if !s.Connected {
			body = append(body, "  Start the daemon, then use :reconnect.")
		}
	}
	if s.viewport.dialogKey != key {
		s.viewport.dialogKey, s.viewport.dialogScroll = key, 0
	}
	return scrollWindow(th, body, rows, &s.viewport.dialogScroll)
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
	case "1", "2", "3", "4", "5":
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
