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

// AgentItems returns all Agent Studio rows across active change sessions and git worktrees.
func (s *State) AgentItems() []AgentItem {
	return domain.CollectAllAgentItems(s.Change, s.ActiveChanges, s.ChangeReceipt, s.GitWorkspace)
}

func isLocalPreviewChangeID(id string) bool {
	return id == "agent-session" || (len(id) > 14 && id[:14] == "agent-session-")
}

func (s *State) upsertActiveChange(chg *ChangeSession) {
	if chg == nil || chg.ID == "" || isLocalPreviewChangeID(chg.ID) {
		return
	}
	cp := *chg
	for i, existing := range s.ActiveChanges {
		if existing != nil && existing.ID == chg.ID {
			s.ActiveChanges[i] = &cp
			return
		}
	}
	s.ActiveChanges = append(s.ActiveChanges, &cp)
}

func (s *State) removeActiveChangeByID(id string) {
	if id == "" || len(s.ActiveChanges) == 0 {
		return
	}
	var kept []*ChangeSession
	for _, chg := range s.ActiveChanges {
		if chg != nil && chg.ID != id {
			kept = append(kept, chg)
		}
	}
	s.ActiveChanges = kept
}

func (s *State) replacePreviewWithSession(chg *ChangeSession) {
	if chg == nil {
		return
	}
	cp := *chg
	s.upsertActiveChange(&cp)
	s.Change = &cp
}

func (s *State) dropPreviewChangeOnError() {
	if s.Change != nil && isLocalPreviewChangeID(s.Change.ID) {
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
	if targetID == "" {
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

