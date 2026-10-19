package tui

import "github.com/nanaki-93/mini-orca/v2/internal/tui/tuidomain"

type (
	DiffHunk              = tuidomain.DiffHunk
	IssueRow              = tuidomain.IssueRow
	IssueSummary          = tuidomain.IssueSummary
	AgentItem             = tuidomain.AgentItem
	BootstrapPillar       = tuidomain.BootstrapPillar
	AnalysisConfigRowSpec = tuidomain.AnalysisConfigRowSpec
)

// IsAddedLine reports whether a diff line kind represents an insertion.
func IsAddedLine(kind string) bool {
	return tuidomain.IsAddedLine(kind)
}

// IsRemovedLine reports whether a diff line kind represents a deletion.
func IsRemovedLine(kind string) bool {
	return tuidomain.IsRemovedLine(kind)
}

// RejectHunkFromDiff reconstructs the proposed file content with the specified hunk reverted.
func RejectHunkFromDiff(allLines []DiffLine, hunk DiffHunk) string {
	return tuidomain.RejectHunkFromDiff(allLines, hunk)
}

// RejectHunkLines returns an updated diff line slice with the specified hunk reverted to context.
func RejectHunkLines(allLines []DiffLine, hunk DiffHunk) []DiffLine {
	return tuidomain.RejectHunkLines(allLines, hunk)
}

// ClassifyDiffScope returns added, removed, and the precision scope badge for a change session.
func ClassifyDiffScope(changes []FileChange) (added int, removed int, badge string) {
	return tuidomain.ClassifyDiffScope(changes)
}

// FindingTitle normalizes a finding title, falling back to its summary when generic or empty.
func FindingTitle(title, summary string) string {
	return tuidomain.FindingTitle(title, summary)
}

// FindingName produces a concise 6-word / 60-character label for a finding.
func FindingName(title, summary string) string {
	return tuidomain.FindingName(title, summary)
}

// SortIssuesBySeverity sorts issues in-place from highest severity to lowest.
func SortIssuesBySeverity(rows []IssueRow) {
	tuidomain.SortIssuesBySeverity(rows)
}

// DedupeIssues removes duplicate findings across semantic and dedicated analysis stages.
func DedupeIssues(items []IssueRow) []IssueRow {
	return tuidomain.DedupeIssues(items)
}

// ComputeIssueSummary aggregates category and lifecycle counts from unified issue rows.
func ComputeIssueSummary(issues []IssueRow, features *FeatureReport) IssueSummary {
	return tuidomain.ComputeIssueSummary(issues, features)
}

// CollectAgentItems derives Agent Studio items from the active change session and Git worktrees.
func CollectAgentItems(change *ChangeSession, receipt *ChangeMutation, gitWS *GitWorkspace) []AgentItem {
	return tuidomain.CollectAgentItems(change, receipt, gitWS)
}

func countPassedChecks(checks []Check) (int, int) {
	return tuidomain.CountPassedChecks(checks)
}

// CanonicalStageLabel maps daemon stage indices and names (create, test, review, human_review) to display labels.
func CanonicalStageLabel(idx int, rawName string) string {
	return tuidomain.CanonicalStageLabel(idx, rawName)
}

// ContainsGuideline reports whether existing AGENTS.md text already contains a rule bullet.
func ContainsGuideline(markdown, rule string) bool {
	return tuidomain.ContainsGuideline(markdown, rule)
}

// InsertGuidelineSection appends a bullet rule under a named Markdown heading in AGENTS.md.
func InsertGuidelineSection(markdown, heading, bullet string) string {
	return tuidomain.InsertGuidelineSection(markdown, heading, bullet)
}

// EvaluateBootstrapChecklist inspects the project index and instructions to check 4 core governance pillars.
func EvaluateBootstrapChecklist(index *ProjectIndex, instructions *InstructionPreview) []BootstrapPillar {
	return tuidomain.EvaluateBootstrapChecklist(index, instructions)
}

// CanonicalModelScopes returns the three primary LLM routing scopes in Mini-Orca.
func CanonicalModelScopes() []string {
	return tuidomain.CanonicalModelScopes()
}

// ScopeRoleDescription returns the purpose of a model scope.
func ScopeRoleDescription(scope string) string {
	return tuidomain.ScopeRoleDescription(scope)
}

// DefaultModelPresets returns the built-in 3-scope model configuration presets.
func DefaultModelPresets() []ModelPreset {
	return tuidomain.DefaultModelPresets()
}

// AssignedScopeModelID returns the user-assigned model/profile ID for a scope.
func (s *State) AssignedScopeModelID(scope string) string {
	return tuidomain.AssignedScopeModelID(s.ScopeModels, scope)
}

// AvailableModelIDs returns all selectable model IDs across configured scopes and discovered CLI catalogs.
func (s *State) AvailableModelIDs() []string {
	return tuidomain.AvailableModelIDs(s.AvailableModels)
}

// EffectiveScopeModel resolves the full runtime Model metadata for the given scope.
func (s *State) EffectiveScopeModel(scope string) Model {
	return tuidomain.EffectiveScopeModel(s.ScopeModels, s.AvailableModels, s.Models, scope)
}

// FormatModelParams formats a concise runtime parameter string for a Model.
func FormatModelParams(m Model) string {
	return tuidomain.FormatModelParams(m)
}

// ResolveProjectDocs returns the project's structured handbook or synthesizes a fallback from Overview.
func ResolveProjectDocs(overview *Overview, project *Project) ProjectDocsReport {
	return tuidomain.ResolveProjectDocs(overview, project)
}

// RenderArchitectureASCII formats an ASCII box-and-arrow architecture diagram for terminal display.
func RenderArchitectureASCII(docs ProjectDocsReport) []string {
	return tuidomain.RenderArchitectureASCII(docs)
}

// EnsureChangeTestPaths deduplicates captured paths, caps scope at 8 files, and guarantees at least one test path.
func EnsureChangeTestPaths(paths []string, index *ProjectIndex) []string {
	return tuidomain.EnsureChangeTestPaths(paths, index)
}

// NormalizeFindingTargets validates and canonicalizes finding targets for OpenChangeSession.
func NormalizeFindingTargets(targets []FindingTarget, projectID string, allowedPaths []string) []FindingTarget {
	return tuidomain.NormalizeFindingTargets(targets, projectID, allowedPaths)
}

// DefaultAnalysisRunConfig returns the default daemon limits and flags for project analysis runs.
func DefaultAnalysisRunConfig() AnalysisRunConfig {
	return tuidomain.DefaultAnalysisRunConfig()
}

// NormalizeAnalysisRunConfig clamps limits to daemon bounds and ensures valid flag combinations.
func NormalizeAnalysisRunConfig(cfg AnalysisRunConfig) AnalysisRunConfig {
	return tuidomain.NormalizeAnalysisRunConfig(cfg)
}

// StepAnalysisBatchFiles cycles through canonical batch file sizes (16..500).
func StepAnalysisBatchFiles(cur int, forward bool) int {
	return tuidomain.StepAnalysisBatchFiles(cur, forward)
}

// StepAnalysisBudgetSeconds cycles through canonical analysis time budgets (60s..3600s).
func StepAnalysisBudgetSeconds(cur int, forward bool) int {
	return tuidomain.StepAnalysisBudgetSeconds(cur, forward)
}

// StepAnalysisMaxAttempts cycles through canonical stage retry limits (1..4).
func StepAnalysisMaxAttempts(cur int, forward bool) int {
	return tuidomain.StepAnalysisMaxAttempts(cur, forward)
}

// BuildAnalysisConfigRows builds the 5 interactive configuration rows for PageAnalysisRun.
func BuildAnalysisConfigRows(cfg AnalysisRunConfig) []AnalysisConfigRowSpec {
	return tuidomain.BuildAnalysisConfigRows(cfg)
}
