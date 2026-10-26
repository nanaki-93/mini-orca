package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

func (s *State) diffChanges() []FileChange {
	if s.Change == nil {
		return nil
	}
	if s.DiffSinceReview {
		return s.Change.ReviewDiff
	}
	return domain.EffectiveSessionChanges(s.Change)
}

func (s *State) handleDiffReviewKey(ev KeyEvent) bool {
	if s.Page != PageEditor || !s.isViewingDiff() {
		return false
	}
	switch ev.Key {
	case "h", "ArrowLeft":
		s.DiffOffset = max(0, s.DiffOffset-12)
	case "l", "ArrowRight":
		s.DiffOffset += 12
	case "0":
		s.DiffOffset = 0
	case "w":
		s.reviewCurrentDiffFile()
	case "W":
		s.stepNextUnreviewedDiffFile()
	case "z":
		if s.Change == nil || s.Change.LastReviewedHash == "" {
			s.CmdOutput = "No complete file review recorded yet. Mark each file with [w]."
			return true
		}
		s.DiffSinceReview = !s.DiffSinceReview
		s.HunkIdx, s.SelectedIdx, s.DiffOffset = 0, 0, 0
	case "P":
		s.openCurrentPlan()
	default:
		return false
	}
	return true
}

func shiftedCode(text string, offset int) string {
	runes := []rune(theme.SanitizeCodeText(text))
	return string(runes[min(offset, len(runes)):])
}

func (s *State) stepNextUnreviewedDiffFile() {
	active, allHunks, ok := s.activeDiffHunk()
	if !ok || s.Change == nil {
		return
	}
	changes := domain.EffectiveSessionChanges(s.Change)
	if len(changes) == 0 {
		return
	}
	for step := 1; step <= len(changes); step++ {
		idx := (active.FileIndex + step) % len(changes)
		path := changes[idx].Path
		if s.Change.FileReviewHash == s.Change.Hash && domain.ContainsString(s.Change.ReviewedFiles, path) {
			continue
		}
		for _, fh := range allHunks {
			if fh.FileIndex == idx {
				s.syncDiffSelectionToHunk(fh.GlobalIndex)
				s.CmdOutput = fmt.Sprintf("Next unreviewed file (%d/%d): %s", idx+1, len(changes), path)
				return
			}
		}
	}
	s.CmdOutput = fmt.Sprintf("All %d changed files are marked reviewed.", len(changes))
}

func (s *State) activeProposalReviewCount() (reviewed, total int) {
	if s.Change == nil {
		return 0, 0
	}
	changes := domain.EffectiveSessionChanges(s.Change)
	total = len(changes)
	if total == 0 || s.Change.FileReviewHash != s.Change.Hash {
		return 0, total
	}
	for _, file := range changes {
		if domain.ContainsString(s.Change.ReviewedFiles, file.Path) {
			reviewed++
		}
	}
	return reviewed, total
}

func (s *State) activeProposalChecksSummary() string {
	if s.Change == nil || len(s.Change.Checks) == 0 {
		return ""
	}
	passed := 0
	for _, ch := range s.Change.Checks {
		if ch.State == "passed" {
			passed++
		}
	}
	return fmt.Sprintf("Checks %d/%d passed", passed, len(s.Change.Checks))
}

func (s *State) diffReviewOverview(th ansiTheme, active FileDiffHunk, width int) []string {
	if s.Change == nil {
		return nil
	}
	var labels []string
	for _, file := range domain.EffectiveSessionChanges(s.Change) {
		mark := "○"
		if s.Change.FileReviewHash == s.Change.Hash && domain.ContainsString(s.Change.ReviewedFiles, file.Path) {
			mark = "✓"
		}
		labels = append(labels, mark+" "+file.Path)
	}
	contentW := issuePanelContentWidth(adaptivePanelWidth(width))
	reviewed := 0
	if s.Change.FileReviewHash == s.Change.Hash {
		reviewed = len(s.Change.ReviewedFiles)
	}
	rows := []string{fmt.Sprintf("%d/%d files reviewed · [w] Mark reviewed · [e] Details", reviewed, len(labels))}
	if s.ShowInlineWhy {
		rows = append(rows, renderDiagnosticRow(th, th.cyan, "Files:", th.fg, strings.Join(labels, " · "), contentW)...)
	}
	mode := "Complete proposal"
	if s.DiffSinceReview {
		mode = "Changes since last complete review"
	}
	rows = append(rows, fmt.Sprintf("%s · column %d · [h/l] Scroll · [z] Compare", mode, s.DiffOffset))
	for _, file := range s.diffChanges() {
		if file.Path != active.Path {
			continue
		}
		var criteria []string
		for _, n := range file.Criteria {
			if n > 0 && n <= len(s.Change.AcceptanceCriteria) {
				criteria = append(criteria, fmt.Sprintf("%d. %s", n, s.Change.AcceptanceCriteria[n-1]))
			}
		}
		if s.ShowInlineWhy && len(criteria) > 0 {
			rows = append(rows, renderDiagnosticBulletRows(th, th.purple, "Model says this meets:", th.purple, th.fg, criteria, contentW)...)
		}
	}
	return append(rows, s.diffApplyDestination())
}

func shiftedDiffLine(line DiffLine, offset int) DiffLine {
	line.Text = shiftedCode(line.Text, offset)
	return line
}

func (s *State) diffApplyDestination() string {
	_, branch := s.mainGitIdentity()
	if branch == "" {
		branch = "current checkout"
	}
	if s.Change.WorktreePath != "" {
		return "Apply: commit in " + s.Change.WorktreePath + "; review and merge separately in Worktrees"
	}
	return "Apply: write and commit reviewed files in " + branch
}

func (s *State) diffMetadataWindow(rows []string, panelW int) []string {
	var wrapped []string
	for _, row := range rows {
		wrapped = append(wrapped, theme.WrapVisibleLine(row, issuePanelContentWidth(panelW))...)
	}
	budget := s.diffMetadataRows()
	if len(wrapped) <= budget {
		s.DetailScroll = 0
		return wrapped
	}
	s.DetailScroll = min(s.DetailScroll, len(wrapped)-budget)
	first := s.DetailScroll
	out := append([]string(nil), wrapped[first:first+budget]...)
	return append(out, fmt.Sprintf("[PgUp/PgDn] Details %d–%d/%d · [e] Why", first+1, first+budget, len(wrapped)))
}

func (s *State) diffMetadataRows() int {
	if s.viewportBodyRows <= 0 {
		return 12
	}
	if s.viewportBodyRows < 10 {
		return 1
	}
	return max(2, s.viewportBodyRows/3-2)
}
