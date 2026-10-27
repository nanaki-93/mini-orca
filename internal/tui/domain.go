package tui

import "github.com/nanaki-93/mini-orca/v2/internal/tui/domain"

type (
	DiffHunk              = domain.DiffHunk
	FileDiffHunk          = domain.FileDiffHunk
	SplitDiffRow          = domain.SplitDiffRow
	UnifiedDiffRow        = domain.UnifiedDiffRow
	IssueRow              = domain.IssueRow
	IssueSummary          = domain.IssueSummary
	AgentItem             = domain.AgentItem
	BootstrapPillar       = domain.BootstrapPillar
	AnalysisConfigRowSpec = domain.AnalysisConfigRowSpec
)

// AssignedScopeModelID returns the user-assigned model/profile ID for a scope.
func (s *State) AssignedScopeModelID(scope string) string {
	return domain.AssignedScopeModelID(s.ScopeModels, scope)
}

// AvailableModelIDs returns all selectable model IDs across configured scopes and discovered CLI catalogs.
func (s *State) AvailableModelIDs() []string {
	return domain.AvailableModelIDs(s.AvailableModels)
}

// EffectiveScopeModel resolves the full runtime Model metadata for the given scope.
func (s *State) EffectiveScopeModel(scope string) Model {
	return domain.EffectiveScopeModel(s.ScopeModels, s.AvailableModels, s.Models, scope)
}

// AgentItems returns all Tasks rows across active change sessions and git worktrees.
func (s *State) AgentItems() []AgentItem {
	items := domain.CollectAllAgentItems(s.Change, s.ActiveChanges, s.ChangeReceipt, s.GitWorkspace)
	for i := range items {
		if s.Change != nil && items[i].ID == s.Change.ID {
			stages := s.currentAgentStages()
			items[i].Stage, items[i].StageID = domain.StageSummary(stages), stages[domain.ActiveStage(stages)].ID
		}
	}
	return items
}

func isLocalPreviewChangeID(id string) bool {
	return id == "agent-session" || (len(id) > 14 && id[:14] == "agent-session-")
}

// HasActiveProposal reports whether the TUI currently has an active, unapplied proposal.
func (s *State) HasActiveProposal() bool {
	return s.Change != nil && !domain.IsCompletedOrInactiveSession(s.Change)
}

func (s *State) upsertActiveChange(chg *ChangeSession) {
	if chg == nil || chg.ID == "" {
		return
	}
	if domain.IsCompletedOrInactiveSession(chg) {
		s.removeActiveChangeByID(chg.ID)
		return
	}
	cp := *chg
	for i, existing := range s.ActiveChanges {
		if existing != nil && existing.ID == chg.ID {
			if cp.WorktreePath == "" {
				cp.WorktreePath = existing.WorktreePath
			}
			if len(cp.FindingTargets) == 0 && len(existing.FindingTargets) > 0 {
				cp.FindingTargets = existing.FindingTargets
			}
			s.ActiveChanges[i] = &cp
			return
		}
	}
	s.ActiveChanges = append(s.ActiveChanges, &cp)
}

func (s *State) removeActiveChangeByID(id string) {
	s.removeActiveSession(id, "")
}

func (s *State) removeActiveSession(id, wtPath string) {
	if (id == "" && wtPath == "") || len(s.ActiveChanges) == 0 {
		return
	}
	var kept []*ChangeSession
	for _, chg := range s.ActiveChanges {
		if chg == nil || (id != "" && chg.ID == id) || (wtPath != "" && chg.WorktreePath == wtPath) {
			continue
		}
		kept = append(kept, chg)
	}
	s.ActiveChanges = kept
}

func (s *State) removeWorktreeByPath(wtPath string) {
	if wtPath == "" || s.GitWorkspace == nil || len(s.GitWorkspace.Worktrees) == 0 {
		return
	}
	var kept []GitWorktree
	for _, wt := range s.GitWorkspace.Worktrees {
		if wt.Path != wtPath {
			kept = append(kept, wt)
		}
	}
	s.GitWorkspace.Worktrees = kept
}

func (s *State) archiveAndCompleteChange(chg *ChangeSession) {
	if chg != nil && chg.ID != "" && !isLocalPreviewChangeID(chg.ID) {
		entry := ChangeHistoryEntry{
			ProjectIdentity: chg.ProjectIdentity,
			WorktreePath:    chg.WorktreePath,
			ID:              chg.ID,
			Kind:            chg.Kind,
			Title:           chg.Title,
			Revision:        chg.Revision,
			Hash:            chg.Hash,
			State:           chg.State,
			Freshness:       chg.Freshness,
			FindingTargets:  chg.FindingTargets,
			Commit:          chg.Commit,
			UpdatedAt:       chg.UpdatedAt,
		}
		updated := false
		for i := range s.ChangeHistory {
			if s.ChangeHistory[i].ID == entry.ID {
				s.ChangeHistory[i] = entry
				updated = true
				break
			}
		}
		if !updated {
			s.ChangeHistory = append([]ChangeHistoryEntry{entry}, s.ChangeHistory...)
		}
		s.removeActiveChangeByID(chg.ID)
	}
	s.selectNextActiveChange()
}

func (s *State) selectNextActiveChange() {
	s.Change = nil
	for _, rem := range s.ActiveChanges {
		if rem != nil && !domain.IsCompletedOrInactiveSession(rem) {
			cp := *rem
			s.Change = &cp
			return
		}
	}
}

func (s *State) removeUnappliedHistoryByID(id string) {
	if id == "" || len(s.ChangeHistory) == 0 {
		return
	}
	var kept []ChangeHistoryEntry
	for _, entry := range s.ChangeHistory {
		if entry.ID != id || entry.State == "applied" {
			kept = append(kept, entry)
		}
	}
	s.ChangeHistory = kept
}

func (s *State) collectRejectedFindingTargets(chg *ChangeSession) []FindingTarget {
	if chg == nil {
		return nil
	}
	targets := append([]FindingTarget(nil), chg.FindingTargets...)
	if len(targets) > 0 {
		return targets
	}
	for _, row := range s.CollectAllIssues() {
		if row.Name == chg.Title || row.Title == chg.Title {
			targets = append(targets, row.Target)
		}
	}
	return targets
}

func matchesPreviewID(id, previewID string) bool {
	if previewID != "" {
		return id == previewID
	}
	return isLocalPreviewChangeID(id)
}

func (s *State) replacePreviewWithSession(previewID string, chg *ChangeSession) {
	if chg == nil {
		return
	}
	cp := *chg
	replaced := false
	for i, existing := range s.ActiveChanges {
		if existing != nil && matchesPreviewID(existing.ID, previewID) {
			if len(cp.FindingTargets) == 0 && len(existing.FindingTargets) > 0 {
				cp.FindingTargets = existing.FindingTargets
			}
			s.ActiveChanges[i] = &cp
			replaced = true
			break
		}
	}
	if !replaced {
		s.upsertActiveChange(&cp)
	}
	if s.Change == nil || s.Change.ID == chg.ID || matchesPreviewID(s.Change.ID, previewID) {
		s.Change = &cp
	}
}

func (s *State) dropPreviewChangeOnError(previewID string) {
	if previewID != "" {
		s.removeActiveChangeByID(previewID)
	} else {
		for i := len(s.ActiveChanges) - 1; i >= 0; i-- {
			if s.ActiveChanges[i] != nil && isLocalPreviewChangeID(s.ActiveChanges[i].ID) {
				s.removeActiveChangeByID(s.ActiveChanges[i].ID)
				break
			}
		}
	}
	if s.Change != nil && matchesPreviewID(s.Change.ID, previewID) {
		if len(s.ActiveChanges) > 0 {
			last := s.ActiveChanges[len(s.ActiveChanges)-1]
			cp := *last
			s.Change = &cp
		} else {
			s.Change = nil
		}
	}
}

func (s *State) syncSelectedAgentSession() {
	items := s.AgentItems()
	idx := s.ClampedIdx()
	if idx < 0 || idx >= len(items) {
		return
	}
	targetID := items[idx].ID
	if targetID == "" || (s.Change != nil && s.Change.ID == targetID) {
		return
	}
	for _, chg := range s.ActiveChanges {
		if chg != nil && chg.ID == targetID {
			cp := *chg
			s.Change = &cp
			return
		}
	}
}

func (s *State) syncSelectedIdxToCurrentChange() {
	if s.Change == nil || s.Change.ID == "" {
		return
	}
	items := s.AgentItems()
	for i, item := range items {
		if item.ID == s.Change.ID {
			s.SelectedIdx = i
			return
		}
	}
}
