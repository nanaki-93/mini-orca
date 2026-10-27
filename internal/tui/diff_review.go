package tui

import (
	"fmt"

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

func (s *State) diffMetadataRows() int {
	if s.viewportBodyRows <= 0 {
		return 12
	}
	if s.viewportBodyRows < 10 {
		return 1
	}
	return max(2, s.viewportBodyRows/3-2)
}
