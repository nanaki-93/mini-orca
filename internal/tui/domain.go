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
