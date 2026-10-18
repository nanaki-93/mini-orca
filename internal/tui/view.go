package tui

import (
	"fmt"
	"strings"
)

type ansiTheme struct {
	accent string
	fg     string
	muted  string
	green  string
	amber  string
	red    string
	purple string
	reset  string
}

func themeForPalette(p Palette) ansiTheme {
	switch p {
	case PaletteCRT:
		return ansiTheme{
			accent: "\033[38;5;46m",
			fg:     "\033[38;5;120m",
			muted:  "\033[38;5;28m",
			green:  "\033[38;5;46m",
			amber:  "\033[38;5;226m",
			red:    "\033[38;5;196m",
			purple: "\033[38;5;83m",
			reset:  "\033[0m",
		}
	case PaletteAmber:
		return ansiTheme{
			accent: "\033[38;5;214m",
			fg:     "\033[38;5;222m",
			muted:  "\033[38;5;136m",
			green:  "\033[38;5;142m",
			amber:  "\033[38;5;214m",
			red:    "\033[38;5;203m",
			purple: "\033[38;5;179m",
			reset:  "\033[0m",
		}
	case PalettePaper:
		return ansiTheme{
			accent: "\033[38;5;25m",
			fg:     "\033[38;5;235m",
			muted:  "\033[38;5;244m",
			green:  "\033[38;5;28m",
			amber:  "\033[38;5;130m",
			red:    "\033[38;5;160m",
			purple: "\033[38;5;91m",
			reset:  "\033[0m",
		}
	default:
		return ansiTheme{
			accent: "\033[38;5;111m",
			fg:     "\033[38;5;253m",
			muted:  "\033[38;5;245m",
			green:  "\033[38;5;114m",
			amber:  "\033[38;5;215m",
			red:    "\033[38;5;203m",
			purple: "\033[38;5;176m",
			reset:  "\033[0m",
		}
	}
}

// Render generates a complete ANSI terminal frame for the current TUI state.
func (s *State) Render(width, height int) string {
	if width <= 0 {
		width = 100
	}
	if height <= 0 {
		height = 30
	}
	th := themeForPalette(s.Palette)
	var lines []string
	lines = append(lines, s.renderStatusBar(th, width))
	lines = append(lines, s.renderHubBar(th, width))
	if sub := s.renderSubViewBar(th); sub != "" {
		lines = append(lines, sub)
	}
	if prog := s.renderProgressStrip(th); prog != "" {
		lines = append(lines, prog)
	}
	lines = append(lines, strings.Repeat("─", max(20, min(width, 120))))

	if s.Confirmation != nil {
		lines = append(lines, s.renderConfirmationModal(th)...)
	} else if s.ShowKeybindsModal {
		lines = append(lines, s.renderKeybindsModal(th)...)
	} else {
		lines = append(lines, s.renderActivePage(th)...)
	}

	lines = append(lines, strings.Repeat("─", max(20, min(width, 120))))
	lines = append(lines, s.renderCommandBar(th))
	return strings.Join(lines, "\n")
}

func (s *State) renderStatusBar(th ansiTheme, _ int) string {
	connBadge := th.green + "● ONLINE" + th.reset
	if !s.Connected {
		connBadge = th.amber + "○ OFFLINE" + th.reset
	}
	projName := "no-project"
	if s.Project != nil && s.Project.Name != "" {
		projName = s.Project.Name
	}
	branch := "main"
	if s.Repository != nil && s.Repository.Branch != "" {
		branch = s.Repository.Branch
	}
	dirty := ""
	if s.GitWorkspace != nil && s.GitWorkspace.Dirty {
		dirty = th.amber + "*dirty" + th.reset
	}
	trust := th.green + "TRUSTED" + th.reset
	if !s.WorkspaceTrust {
		trust = th.amber + "UNTRUSTED" + th.reset
	}
	return fmt.Sprintf(
		"%sMINI-ORCA // TUI%s  [%s %s]  git:%s%s  trust:%s  palette:%s%s%s  [?] Keys",
		th.accent, th.reset, connBadge, projName, branch, dirty, trust, th.purple, s.Palette, th.reset,
	)
}

func (s *State) renderHubBar(th ansiTheme, _ int) string {
	active := s.ActiveHub()
	var parts []string
	for _, tab := range HubTabs() {
		label := fmt.Sprintf("[%s] %s", tab.Key, tab.Label)
		if tab.Key == active.Key {
			parts = append(parts, th.accent+"▸ "+label+th.reset)
		} else {
			parts = append(parts, th.muted+label+th.reset)
		}
	}
	return strings.Join(parts, "  │  ")
}

func (s *State) renderSubViewBar(th ansiTheme) string {
	hub := s.ActiveHub()
	if len(hub.SubPages) == 0 {
		return ""
	}
	var parts []string
	for _, sp := range hub.SubPages {
		if sp.Page == s.Page {
			parts = append(parts, th.accent+"["+sp.Label+"]"+th.reset)
		} else {
			parts = append(parts, th.muted+sp.Label+th.reset)
		}
	}
	return "  Sub-views (Tab): " + strings.Join(parts, " · ")
}

func (s *State) renderProgressStrip(th ansiTheme) string {
	var parts []string
	if s.Run != nil && s.Run.Status != "" {
		parts = append(parts, fmt.Sprintf("Analysis: %s%s%s (%ds)", th.accent, strings.ToUpper(s.Run.Status), th.reset, s.Run.ElapsedSeconds))
	}
	if s.Change != nil {
		_, _, badge := ClassifyDiffScope(s.Change.Changes)
		parts = append(parts, fmt.Sprintf("Proposal: %s [%s]", s.Change.Title, badge))
	}
	if s.Busy != "" {
		parts = append(parts, th.amber+"Busy: "+s.Busy+th.reset)
	}
	return strings.Join(parts, "  │  ")
}

func (s *State) renderConfirmationModal(th ansiTheme) []string {
	c := s.Confirmation
	return []string{
		th.amber + "┌── CONFIRMATION REQUIRED ──────────────────────────────────────┐" + th.reset,
		fmt.Sprintf("│  %s", c.Title),
		fmt.Sprintf("│  %s", c.Message),
		fmt.Sprintf("│  Press [y / Enter] %s   or   [n / Esc] Cancel", firstNonEmpty(c.AcceptLabel, "Confirm")),
		th.amber + "└───────────────────────────────────────────────────────────────┘" + th.reset,
	}
}

func (s *State) renderKeybindsModal(th ansiTheme) []string {
	return []string{
		th.accent + "┌── KEYBINDINGS & COLON COMMANDS REFERENCE ([?] or Esc to close) ──┐" + th.reset,
		"│  [1-5] Switch Hub      [Tab / Shift+Tab] Sub-view   [j/k] Move Row",
		"│  [Enter] Select/Open   [t/c/f] Issue Category       [s/v] Issue Status",
		"│  [x/Space] Select/Excl [a] Dispatch to Agent        [d] Dismiss Item",
		"│  [c] Apply & Commit    [u] Guarded Undo             [m] Merge Worktree",
		"│  [r] Run/Repair        [e] Toggle Diff Why          [p] Cycle Palette",
		"│  Commands: :analyze :test :fix :agent :open :project :apply :undo :q",
		th.accent + "└──────────────────────────────────────────────────────────────────┘" + th.reset,
	}
}

func (s *State) renderCommandBar(th ansiTheme) string {
	if s.CmdActive {
		return fmt.Sprintf("%s:%s%s█   (Enter to execute, Esc to cancel)", th.green, s.CmdInput, th.reset)
	}
	if s.Error != "" {
		return fmt.Sprintf("%sERROR: %s%s", th.red, s.Error, th.reset)
	}
	return fmt.Sprintf("%s>%s %s", th.green, th.reset, s.CmdOutput)
}
