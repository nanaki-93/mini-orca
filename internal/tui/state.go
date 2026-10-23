package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

// Page identifies an active hub view or sub-view in the TUI.
type Page string

const (
	PageSummary        Page = "summary"
	PageRepo           Page = "repo"
	PageProject        Page = "project"
	PageAgents         Page = "agents"
	PageBugs           Page = "bugs"
	PageIssueDetail    Page = "issue-detail"
	PageFeatures       Page = "features"
	PageFeatureDetail  Page = "feature-detail"
	PagePlan           Page = "plan"
	PageAnalysisRun    Page = "analysis-run"
	PageAnalysisFiles  Page = "analysis-files"
	PageEditor         Page = "editor"
	PageGit            Page = "git"
	PageWorktreeDetail Page = "worktree-detail"
	PageHistory        Page = "history"
	PageDocs           Page = "docs"
	PageDocsComponents Page = "docs-components"
	PageDocsDecisions  Page = "docs-decisions"
	PageDiagrams       Page = "diagrams"
	PageInstructions   Page = "instructions"
	PageModels         Page = "models"
	PageBootstrap      Page = "bootstrap"
	PageLessons        Page = "lessons"
)

// Palette identifies one of the four terminal color themes.
type Palette = theme.Palette

const (
	PaletteTokyo = theme.PaletteTokyo
	PaletteCRT   = theme.PaletteCRT
	PaletteAmber = theme.PaletteAmber
	PalettePaper = theme.PalettePaper
)

// HubSubPage pairs a sub-view Page with its human-readable tab label.
type HubSubPage struct {
	Page  Page
	Label string
}

// HubTab defines one of the 5 top-level TUI hubs.
type HubTab struct {
	Key      string
	Page     Page
	Label    string
	SubPages []HubSubPage
}

// Confirmation represents a modal confirmation prompt requiring explicit user consent.
type Confirmation struct {
	Title       string
	Message     string
	AcceptLabel string
	Destructive bool
	OnConfirm   func(*State)
}

// ActionRequest represents an asynchronous daemon operation queued by a user action.
type ActionRequest struct {
	Worktree       *WorktreeIntent
	Source         ProjectIdentity
	ExistingChange *ChangeSession
	ResumeFrom     string
	Kind           string
	Target         string
	Message        string
	PreviewID      string
	Seed           *ChangeSeed
	Issue          *FindingTarget
}

// State holds the complete deterministic state of the Mini-Orca Terminal UI.
type State struct {
	DetailSection       int
	PlanDraft           *PlanDraft
	DetailScroll        int
	DiffOffset          int
	DiffSinceReview     bool
	Page                Page
	Palette             Palette
	Connected           bool
	Version             string
	Busy                string
	Error               string
	Notice              string
	SelectedIdx         int
	IssueView           string
	IssueCategory       string
	SelectedIssueIDs    []string
	DocsSection         string
	RulesSection        string
	ShowInlineWhy       bool
	DiffMode            string
	HunkIdx             int
	BrowseFiles         bool
	AgentDetailOpen     bool
	AgentStage          int
	ShowKeybindsModal   bool
	KeybindsScroll      int
	Confirmation        *Confirmation
	CmdActive           bool
	CmdInput            string
	CmdHistory          []string
	CmdOutput           string
	GraduatedLessons    []string
	InstructionDraft    string
	ExcludedPaths       []string
	ActivePresetID      string
	CustomPresets       []ModelPreset
	ScopeModels         map[string]string
	AnalysisConfig      AnalysisRunConfig
	PendingActions      []ActionRequest
	Quitting            bool
	viewport            viewportMemory
	viewportBodyRows    int
	nextPreviewSeq      int
	commandCursor       int
	commandSelection    int
	commandHistoryIndex int
	commandHistoryDraft string

	Project           *Project
	Index             *ProjectIndex
	Overview          *Overview
	Findings          []Finding
	CompletedFindings []FindingCompletion
	DismissedTargets  []FindingTarget
	Results           map[string]SectionResults
	SecurityReport    *SecurityReport
	Run               *AnalysisRun
	Selection         *Selection
	File              *FileInfo
	IssueFile         *FileInfo
	Symbols           []SymbolInfo
	Change            *ChangeSession
	ActiveChanges     []*ChangeSession
	ChangeReceipt     *ChangeMutation
	ChangeHistory     []ChangeHistoryEntry
	Repository        *GitRepository
	GitWorkspace      *GitWorkspace
	Features          *FeatureReport
	Instructions      *InstructionPreview
	Scan              *Scan
	WorkspaceTrust    bool
	Models            *ModelCatalog
	AvailableModels   *AvailableModels
}

// HubTabs returns the 5 canonical Mini-Orca TUI hubs and their focused sub-views.
func HubTabs() []HubTab {
	return []HubTab{
		{
			Key:   "1",
			Page:  PageSummary,
			Label: "Project",
			SubPages: []HubSubPage{
				{Page: PageSummary, Label: "Dashboard"},
				{Page: PageRepo, Label: "Repository"},
				{Page: PageProject, Label: "Tests & Checks"},
			},
		},
		{
			Key:   "2",
			Page:  PageAgents,
			Label: "Work",
			SubPages: []HubSubPage{
				{Page: PageAgents, Label: "Tasks"},
				{Page: PageBugs, Label: "Issues"},
				{Page: PageIssueDetail, Label: "Issue Detail"},
				{Page: PageFeatures, Label: "Features"},
				{Page: PagePlan, Label: "Task Plan"},
				{Page: PageAnalysisRun, Label: "Analysis"},
				{Page: PageAnalysisFiles, Label: "File Scope"},
			},
		},
		{
			Key:   "3",
			Page:  PageEditor,
			Label: "Code",
			SubPages: []HubSubPage{
				{Page: PageEditor, Label: "Files & Diff"},
				{Page: PageGit, Label: "Worktrees"},
				{Page: PageHistory, Label: "Proposals"},
			},
		},
		{
			Key:   "4",
			Page:  PageDocs,
			Label: "Docs",
			SubPages: []HubSubPage{
				{Page: PageDocs, Label: "Context & Deps"},
				{Page: PageDocsComponents, Label: "Components"},
				{Page: PageDocsDecisions, Label: "Scenarios & ADRs"},
				{Page: PageDiagrams, Label: "Architecture"},
			},
		},
		{
			Key:   "5",
			Page:  PageInstructions,
			Label: "Settings",
			SubPages: []HubSubPage{
				{Page: PageInstructions, Label: "Trust"},
				{Page: PageModels, Label: "Models"},
				{Page: PageBootstrap, Label: "Bootstrap"},
				{Page: PageLessons, Label: "Lessons"},
			},
		},
	}
}

// NewState initializes a default TUI workspace state.
func NewState() *State {
	return &State{
		Page:                PageSummary,
		commandSelection:    -1,
		commandHistoryIndex: -1,
		Palette:             PaletteTokyo,
		IssueView:           "open",
		IssueCategory:       "all",
		DocsSection:         "context",
		RulesSection:        "trust",
		ShowInlineWhy:       false,
		DiffMode:            "hunk",
		ActivePresetID:      "local-default",
		ScopeModels: map[string]string{
			"analyze":  "analyze",
			"bug":      "bug",
			"function": "function",
		},
		AnalysisConfig: domain.DefaultAnalysisRunConfig(),
		WorkspaceTrust: false,
		Results:        make(map[string]SectionResults),
		CmdOutput:      "Workspace ready",
	}
}

// EffectiveAnalysisConfig returns the normalized analysis run configuration.
func (s *State) EffectiveAnalysisConfig() AnalysisRunConfig {
	return domain.NormalizeAnalysisRunConfig(s.AnalysisConfig)
}

func (s *State) adaptiveWindowRows(overhead, defaultMin int) int {
	if s.viewportBodyRows <= 0 {
		return defaultMin
	}
	return max(1, s.viewportBodyRows-overhead)
}

// AllModelPresets returns the built-in presets followed by any user-saved custom presets.
func (s *State) AllModelPresets() []ModelPreset {
	return domain.MergeModelPresets(s.CustomPresets)
}

type persistedModelSettings struct {
	ActivePresetID string            `json:"active_preset_id,omitempty"`
	ScopeModels    map[string]string `json:"scope_models,omitempty"`
	CustomPresets  []ModelPreset     `json:"custom_presets,omitempty"`
}

func (s *State) customPresetsFilePath() string {
	if s.Project == nil || strings.TrimSpace(s.Project.Path) == "" {
		return ""
	}
	info, err := os.Stat(s.Project.Path)
	if err != nil || !info.IsDir() {
		return ""
	}
	return filepath.Join(s.Project.Path, ".mini-orca", "tui-presets.json")
}

func (s *State) loadCustomPresetsFromDisk() {
	p := s.customPresetsFilePath()
	if p == "" {
		return
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return
	}
	var settings persistedModelSettings
	if err := json.Unmarshal(data, &settings); err == nil &&
		(settings.ActivePresetID != "" || len(settings.ScopeModels) > 0 || len(settings.CustomPresets) > 0) {
		if len(settings.CustomPresets) > 0 {
			s.CustomPresets = settings.CustomPresets
		}
		if strings.TrimSpace(settings.ActivePresetID) != "" {
			s.ActivePresetID = strings.TrimSpace(settings.ActivePresetID)
		}
		if len(settings.ScopeModels) > 0 {
			if s.ScopeModels == nil {
				s.ScopeModels = make(map[string]string)
			}
			for _, scope := range domain.CanonicalModelScopes() {
				if val := strings.TrimSpace(settings.ScopeModels[scope]); val != "" {
					s.ScopeModels[scope] = val
				}
			}
		}
		return
	}
	var loaded []ModelPreset
	if err := json.Unmarshal(data, &loaded); err == nil && len(loaded) > 0 {
		s.CustomPresets = loaded
	}
}

func (s *State) saveCustomPresetsToDisk() {
	p := s.customPresetsFilePath()
	if p == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return
	}
	scopeModels := make(map[string]string, len(s.ScopeModels))
	for _, scope := range domain.CanonicalModelScopes() {
		scopeModels[scope] = s.AssignedScopeModelID(scope)
	}
	payload := persistedModelSettings{
		ActivePresetID: s.ActivePresetID,
		ScopeModels:    scopeModels,
		CustomPresets:  s.CustomPresets,
	}
	if data, err := json.MarshalIndent(payload, "", "  "); err == nil {
		_ = os.WriteFile(p, data, 0o600)
	}
}

func (s *State) issueHistoryFilePath() string {
	if s.Project == nil || strings.TrimSpace(s.Project.Path) == "" {
		return ""
	}
	info, err := os.Stat(s.Project.Path)
	if err != nil || !info.IsDir() {
		return ""
	}
	return filepath.Join(s.Project.Path, ".mini-orca", "tui-issue-history.json")
}

type persistedIssueHistory struct {
	Completed []FindingCompletion `json:"completed,omitempty"`
	Dismissed []FindingTarget     `json:"dismissed,omitempty"`
}

func (s *State) loadIssueHistoryFromDisk() {
	p := s.issueHistoryFilePath()
	if p == "" {
		return
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return
	}
	var hist persistedIssueHistory
	if err := json.Unmarshal(data, &hist); err == nil && (len(hist.Completed) > 0 || len(hist.Dismissed) > 0) {
		s.applyPersistedIssueHistory(hist.Completed, hist.Dismissed)
		return
	}
	var saved []FindingCompletion
	if err := json.Unmarshal(data, &saved); err == nil && len(saved) > 0 {
		s.applyPersistedIssueHistory(saved, nil)
	}
}

func (s *State) applyPersistedIssueHistory(completed []FindingCompletion, dismissed []FindingTarget) {
	projID := s.projectID()
	for _, item := range completed {
		if projID == "" || item.ProjectID == "" || item.ProjectID == projID {
			s.mergeFindingCompletion(item)
		}
	}
	for _, target := range dismissed {
		if projID == "" || target.ProjectID == "" || target.ProjectID == projID {
			s.addDismissedTarget(target)
		}
	}
}

func (s *State) saveIssueHistoryToDisk() {
	p := s.issueHistoryFilePath()
	if p == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return
	}
	payload := persistedIssueHistory{
		Completed: s.CompletedFindings,
		Dismissed: s.DismissedTargets,
	}
	if data, err := json.MarshalIndent(payload, "", "  "); err == nil {
		_ = os.WriteFile(p, data, 0o600)
	}
}

func (s *State) addDismissedTarget(target FindingTarget) {
	if target.ID == "" {
		return
	}
	for _, existing := range s.DismissedTargets {
		if sameFindingTarget(existing, target) {
			return
		}
	}
	s.DismissedTargets = append(s.DismissedTargets, target)
}

func (s *State) isFindingDismissed(target FindingTarget) bool {
	for _, existing := range s.DismissedTargets {
		if sameFindingTarget(existing, target) {
			return true
		}
	}
	return false
}

func (s *State) mergeFindingCompletion(item FindingCompletion) {
	for i := range s.CompletedFindings {
		cur := &s.CompletedFindings[i]
		if sameFindingTarget(cur.FindingTarget, item.FindingTarget) {
			cur.CommitOID = client.FirstNonEmpty(item.CommitOID, cur.CommitOID)
			cur.CommitCode = client.FirstNonEmpty(item.CommitCode, cur.CommitCode)
			cur.SessionID = client.FirstNonEmpty(item.SessionID, cur.SessionID)
			cur.Title = client.FirstNonEmpty(item.Title, cur.Title)
			return
		}
	}
	s.CompletedFindings = append(s.CompletedFindings, item)
}

// ActiveHub returns the HubTab owning the current Page.
func (s *State) ActiveHub() HubTab {
	tabs := HubTabs()
	for _, tab := range tabs {
		if tab.Page == parentPage(s.Page) {
			return tab
		}
		for _, sp := range tab.SubPages {
			if sp.Page == parentPage(s.Page) {
				return tab
			}
		}
	}
	return tabs[0]
}

// Navigate switches the active page, synchronizes section state, and resets selection when switching lists.
func (s *State) Navigate(page Page) {
	if s.Page != page {
		s.viewport.followSelection = true
		s.DetailScroll = 0
		s.DetailSection = 0
		s.Error = ""
		preserveIssueIdx := parentPage(s.Page) == parentPage(page)
		if !preserveIssueIdx {
			s.SelectedIdx = 0
		}
	}
	if page != PageAgents {
		s.AgentDetailOpen = false
		s.AgentStage = 0
	}
	s.Page = page
	if page == PageAgents {
		s.syncSelectedIdxToCurrentChange()
	}
	switch page {
	case PageDocs:
		s.DocsSection = "context"
	case PageDocsComponents:
		s.DocsSection = "components"
	case PageDocsDecisions:
		s.DocsSection = "decisions"
	case PageInstructions:
		s.RulesSection = "trust"
	case PageModels:
		s.RulesSection = "models"
	case PageBootstrap:
		s.RulesSection = "bootstrap"
	case PageLessons:
		s.RulesSection = "lessons"
	}
}

// EffectivePage resolves the active sub-page, accounting for DocsSection and RulesSection overrides.
func (s *State) EffectivePage() Page {
	if s.Page == PageDocs {
		switch s.DocsSection {
		case "components":
			return PageDocsComponents
		case "scenarios", "decisions":
			return PageDocsDecisions
		}
	}
	if s.Page == PageInstructions {
		switch s.RulesSection {
		case "models":
			return PageModels
		case "bootstrap":
			return PageBootstrap
		case "lessons":
			return PageLessons
		}
	}
	return s.Page
}

// AcceptProjectResponse verifies that a late response still matches the active project identity.
func (s *State) AcceptProjectResponse(id ProjectIdentity) bool {
	if s.Project == nil || id.ProjectID == "" {
		return true
	}
	if s.Project.ProjectID != id.ProjectID {
		return false
	}
	if id.ProjectRevision != "" && s.Project.ProjectRevision != "" && s.Project.ProjectRevision != id.ProjectRevision {
		return false
	}
	return true
}

// AcceptFileResponse verifies that a late file response matches the currently selected file path.
func (s *State) AcceptFileResponse(path, contentHash string) bool {
	if s.File == nil || s.File.Path == "" {
		return true
	}
	if s.File.Path != path {
		return false
	}
	if contentHash != "" && s.File.ContentHash != "" && s.File.ContentHash != contentHash {
		return false
	}
	return true
}

// CollectAllIssues returns all deduplicated, severity-sorted issues across semantic, performance, and security results.
func (s *State) CollectAllIssues() []IssueRow {
	agentItems := s.AgentItems()
	var rows []IssueRow
	rows = s.appendSemanticFindings(rows, agentItems)
	rows = s.appendSectionResults(rows, agentItems)
	deduped := domain.DedupeIssues(rows)
	domain.SortIssuesBySeverity(deduped)
	return deduped
}

func (s *State) appendSemanticFindings(rows []IssueRow, agents []AgentItem) []IssueRow {
	for _, f := range s.Findings {
		cat := f.Category
		if cat == "" {
			cat = "bugs"
		}
		if _, hasSection := s.Results[cat]; !hasSection || f.Status == "in_progress" {
			rows = s.pushSemanticIssue(rows, f, cat, agents)
		}
	}
	return rows
}

func (s *State) appendSectionResults(rows []IssueRow, agents []AgentItem) []IssueRow {
	for _, cat := range []string{"bugs", "security", "performance"} {
		res, ok := s.Results[cat]
		if !ok {
			continue
		}
		for _, f := range res.Semantic {
			rows = s.pushSemanticIssue(rows, f, cat, agents)
		}
		for _, rep := range res.Performance {
			rows = s.pushPerformanceIssues(rows, rep, agents)
		}
		for _, rep := range res.Security {
			rows = s.pushSecurityIssues(rows, rep, agents)
		}
	}
	if s.SecurityReport != nil {
		rows = s.pushSecurityIssues(rows, *s.SecurityReport, agents)
	}
	return rows
}

func (s *State) pushSemanticIssue(rows []IssueRow, f Finding, cat string, agents []AgentItem) []IssueRow {
	if f.Status == "retired" || f.Status == "dismissed" {
		return rows
	}
	id := "semantic:" + f.ID
	if containsIssueID(rows, id) {
		return rows
	}
	target := FindingTarget{ProjectID: client.FirstNonEmpty(f.ProjectID, s.projectID()), Kind: "semantic", ID: f.ID, Path: f.Location.Path}
	if s.isFindingDismissed(target) {
		return rows
	}
	title := domain.FindingTitle(f.Title, f.Message)
	name := domain.FindingName(f.Title, f.Message)
	comp := s.findCompletion(target)
	done := f.Status == "fixed" || f.Status == "done" || comp != nil
	agentPath := s.resolveSemanticAgentPath(done, f, agents, target, name, title)
	cause := client.FirstNonEmpty(f.Message, f.Evidence, "No cause recorded.")
	evidence := domain.DistinctInsightEvidence(f.Evidence, cause, f.EngineeringInsight)
	symbol, signature, sol, criteria, nonGoals := s.extractSemanticIssueDetails(f, cat, title, cause)
	commitOID, commitCode := completionCommitMeta(comp)
	rows = append(rows, IssueRow{
		ID:                 id,
		Title:              title,
		Name:               name,
		Path:               f.Location.Path,
		Symbol:             symbol,
		Signature:          signature,
		Line:               f.Location.StartLine,
		EndLine:            f.Location.EndLine,
		Severity:           f.Severity,
		Confidence:         client.FirstNonEmpty(f.Confidence, "medium"),
		Category:           cat,
		Rule:               f.Rule,
		Source:             client.FirstNonEmpty(f.Source, "ai"),
		Freshness:          client.FirstNonEmpty(f.Freshness, "current"),
		Status:             issueStatus(done, agentPath),
		CommitOID:          commitOID,
		CommitCode:         commitCode,
		AgentPath:          agentPath,
		Cause:              cause,
		Evidence:           evidence,
		Solution:           sol,
		Target:             target,
		AcceptanceCriteria: criteria,
		NonGoals:           nonGoals,
	})
	return rows
}

func (s *State) resolveSemanticAgentPath(done bool, f Finding, agents []AgentItem, target FindingTarget, name, title string) string {
	if done {
		return ""
	}
	if p := matchAgentPath(agents, target, name, title); p != "" {
		return p
	}
	if f.Status == "in_progress" {
		return client.FirstNonEmpty(f.Location.Path, f.ID)
	}
	return ""
}

func (s *State) extractSemanticIssueDetails(f Finding, cat, title, cause string) (symbol, signature, solution string, criteria, nonGoals []string) {
	symbol = f.Location.Symbol
	if f.TaskSpec != nil {
		if symbol == "" {
			symbol = f.TaskSpec.TargetSymbol
		}
		signature = f.TaskSpec.TargetSignature
		nonGoals = f.TaskSpec.NonGoals
		if len(f.TaskSpec.AcceptanceCriteria) > 0 {
			criteria = f.TaskSpec.AcceptanceCriteria
		}
	}
	if (symbol == "" || signature == "") && s.Index != nil && f.Location.Path != "" {
		sym, sig := s.lookupIndexedSymbolForIssue(f.Location.Path, symbol, f.Location.StartLine, title, cause)
		if symbol == "" {
			symbol = sym
		}
		if signature == "" {
			signature = sig
		}
	}
	src := client.FirstNonEmpty(f.Source, "ai")
	solution = domain.SynthesizeIssueRemediation(title, cause, cat, f.Rule, src, f.Location.Path, symbol, f.Location.StartLine, f.EngineeringInsight)
	return symbol, signature, solution, criteria, nonGoals
}

func (s *State) lookupIndexedSymbolForIssue(path, targetSymbol string, line int, title, cause string) (string, string) {
	if s.Index == nil {
		return "", ""
	}
	for _, f := range s.Index.Files {
		if f.Path == path {
			return domain.MatchIndexedSymbolForFinding(f.Symbols, targetSymbol, line, title, cause)
		}
	}
	return "", ""
}

func completionCommitMeta(comp *FindingCompletion) (string, string) {
	if comp == nil {
		return "", ""
	}
	return comp.CommitOID, comp.CommitCode
}

func (s *State) pushPerformanceIssues(rows []IssueRow, rep PerformanceReport, agents []AgentItem) []IssueRow {
	for _, f := range rep.Findings {
		id := fmt.Sprintf("performance:%s:%s", rep.Path, f.ID)
		if containsIssueID(rows, id) {
			continue
		}
		target := FindingTarget{ProjectID: client.FirstNonEmpty(rep.ProjectID, s.projectID()), Kind: "performance", ID: f.ID, Path: rep.Path}
		if s.isFindingDismissed(target) {
			continue
		}
		name := domain.FindingName(f.Title, f.ObservedPattern)
		comp := s.findCompletion(target)
		done := comp != nil
		agentPath := ""
		if !done {
			agentPath = matchAgentPath(agents, target, name, f.Title)
		}
		commitOID, commitCode := completionCommitMeta(comp)
		var criteria []string
		if plan := strings.TrimSpace(f.VerificationPlan); plan != "" {
			criteria = []string{plan}
		}
		cause := client.FirstNonEmpty(f.ObservedPattern, "No cause recorded.")
		sol := strings.TrimSpace(f.Recommendation)
		if sol == "" {
			sol = domain.SynthesizeIssueRemediation(f.Title, cause, "performance", f.Category, "ai", rep.Path, f.Symbol, f.StartLine, nil)
		}
		rows = append(rows, IssueRow{
			ID:                 id,
			Title:              f.Title,
			Name:               name,
			Path:               rep.Path,
			Symbol:             f.Symbol,
			Line:               f.StartLine,
			EndLine:            f.EndLine,
			Severity:           f.PotentialImpact,
			Confidence:         client.FirstNonEmpty(f.Confidence, "medium"),
			Category:           "performance",
			Rule:               f.Category,
			Source:             "ai",
			Freshness:          client.FirstNonEmpty(rep.Status, "current"),
			Status:             issueStatus(done, agentPath),
			CommitOID:          commitOID,
			CommitCode:         commitCode,
			AgentPath:          agentPath,
			Cause:              cause,
			Solution:           sol,
			Target:             target,
			AcceptanceCriteria: criteria,
		})
	}
	return rows
}

func (s *State) pushSecurityIssues(rows []IssueRow, rep SecurityReport, agents []AgentItem) []IssueRow {
	for _, f := range rep.Findings {
		if f.Triage == "dismissed" {
			continue
		}
		path := client.FirstNonEmpty(f.SourceAnchor.Path, rep.Path)
		src := client.FirstNonEmpty(rep.Source, "deterministic")
		id := fmt.Sprintf("security:%s:%s:%s", rep.Path, src, f.ID)
		if containsIssueID(rows, id) {
			continue
		}
		target := FindingTarget{ProjectID: client.FirstNonEmpty(rep.ProjectID, s.projectID()), Kind: "security", ID: f.ID, Path: path, Source: src}
		if s.isFindingDismissed(target) {
			continue
		}
		name := domain.FindingName(f.Title, f.ObservedCondition)
		comp := s.findCompletion(target)
		done := f.Triage == "fixed" || comp != nil
		agentPath := ""
		if !done {
			agentPath = matchAgentPath(agents, target, name, f.Title)
		}
		commitOID, commitCode := completionCommitMeta(comp)
		var criteria []string
		if idea := strings.TrimSpace(f.VerificationIdea); idea != "" {
			criteria = []string{idea}
		}
		cause := client.FirstNonEmpty(f.ObservedCondition, "No cause recorded.")
		sol := strings.TrimSpace(f.Remediation)
		if sol == "" {
			sol = domain.SynthesizeIssueRemediation(f.Title, cause, "security", f.Rule, src, path, f.SourceAnchor.Symbol, f.SourceAnchor.StartLine, nil)
		}
		rows = append(rows, IssueRow{
			ID:                 id,
			Title:              f.Title,
			Name:               name,
			Path:               path,
			Symbol:             f.SourceAnchor.Symbol,
			Line:               f.SourceAnchor.StartLine,
			EndLine:            f.SourceAnchor.EndLine,
			Severity:           f.Severity,
			Confidence:         client.FirstNonEmpty(f.Confidence, "high"),
			Category:           "security",
			Rule:               f.Rule,
			Source:             src,
			Freshness:          client.FirstNonEmpty(rep.Status, "current"),
			Status:             issueStatus(done, agentPath),
			CommitOID:          commitOID,
			CommitCode:         commitCode,
			AgentPath:          agentPath,
			Cause:              cause,
			Solution:           sol,
			Target:             target,
			AcceptanceCriteria: criteria,
		})
	}
	return rows
}

func containsIssueID(rows []IssueRow, id string) bool {
	for _, r := range rows {
		if r.ID == id {
			return true
		}
	}
	return false
}

func sameFindingTarget(a, b FindingTarget) bool {
	if a.Kind != b.Kind || a.ID != b.ID || a.Path != b.Path {
		return false
	}
	return a.Source == "" || b.Source == "" || a.Source == b.Source
}

func (s *State) findCompletion(target FindingTarget) *FindingCompletion {
	for i := range s.CompletedFindings {
		c := s.CompletedFindings[i]
		if sameFindingTarget(c.FindingTarget, target) {
			if c.CommitCode == "" {
				c.CommitCode = s.lookupHistoryCommitCode(target, c.CommitOID)
			}
			return &c
		}
	}
	if s.Change != nil && s.Change.State == "applied" {
		for _, t := range s.Change.FindingTargets {
			if sameFindingTarget(t, target) {
				oid := ""
				if s.Change.Commit != nil {
					oid = s.Change.Commit.OID
				}
				code := domain.ResolveCommitCode(s.Change.ID, s.Change.Hash, s.Change.FindingTargets, s.Change.Commit)
				return &FindingCompletion{FindingTarget: target, CommitOID: oid, CommitCode: code, SessionID: s.Change.ID, Title: s.Change.Title}
			}
		}
	}
	for _, entry := range s.ChangeHistory {
		if entry.State != "applied" {
			continue
		}
		for _, t := range entry.FindingTargets {
			if sameFindingTarget(t, target) {
				oid := ""
				if entry.Commit != nil {
					oid = entry.Commit.OID
				}
				code := domain.ResolveCommitCode(entry.ID, entry.Hash, entry.FindingTargets, entry.Commit)
				return &FindingCompletion{FindingTarget: target, CommitOID: oid, CommitCode: code, SessionID: entry.ID, Title: entry.Title}
			}
		}
	}
	return nil
}

func (s *State) lookupHistoryCommitCode(target FindingTarget, commitOID string) string {
	for _, entry := range s.ChangeHistory {
		if commitOID != "" && entry.Commit != nil && entry.Commit.OID == commitOID {
			return domain.ResolveCommitCode(entry.ID, entry.Hash, entry.FindingTargets, entry.Commit)
		}
		for _, t := range entry.FindingTargets {
			if sameFindingTarget(t, target) {
				return domain.ResolveCommitCode(entry.ID, entry.Hash, entry.FindingTargets, entry.Commit)
			}
		}
	}
	if commitOID != "" {
		return domain.GenerateCommitCode(commitOID, "", []FindingTarget{target})
	}
	return ""
}

func matchAgentPath(agents []AgentItem, target FindingTarget, name, title string) string {
	lowName := strings.ToLower(strings.TrimSpace(name))
	lowTitle := strings.ToLower(strings.TrimSpace(title))
	for _, item := range agents {
		if isInactiveAgentStatus(item.Status) {
			continue
		}
		if len(item.FindingTargets) > 0 {
			for _, t := range item.FindingTargets {
				if sameFindingTarget(t, target) {
					return client.FirstNonEmpty(item.Path, item.ID)
				}
			}
			continue
		}
		lowItem := strings.ToLower(strings.TrimSpace(item.Title))
		if lowItem != "" && (lowItem == lowName || lowItem == lowTitle) {
			return client.FirstNonEmpty(item.Path, item.ID)
		}
	}
	return ""
}

func isInactiveAgentStatus(status string) bool {
	switch status {
	case "applied", "reverted", "undone", "canceled", "stale":
		return true
	default:
		return false
	}
}

func issueStatus(done bool, agentPath string) string {
	if done {
		return "done"
	}
	if agentPath != "" {
		return "in_progress"
	}
	return "open"
}

// FilteredIssues returns the issue list matching the current IssueCategory and IssueView filters.
func (s *State) FilteredIssues() []IssueRow {
	all := s.CollectAllIssues()
	var out []IssueRow
	for _, item := range all {
		if s.IssueCategory != "all" && item.Category != s.IssueCategory {
			continue
		}
		if !matchesIssueView(item.Status, s.IssueView) {
			continue
		}
		out = append(out, item)
	}
	return out
}

// IssueStatusCounts returns the count of open, in_progress, and done findings for the active IssueCategory.
func (s *State) IssueStatusCounts() (openCount, inProgressCount, doneCount int) {
	for _, item := range s.CollectAllIssues() {
		if s.IssueCategory != "all" && item.Category != s.IssueCategory {
			continue
		}
		switch item.Status {
		case "in_progress":
			inProgressCount++
		case "done":
			doneCount++
		default:
			openCount++
		}
	}
	return openCount, inProgressCount, doneCount
}

func matchesIssueView(status, view string) bool {
	switch view {
	case "in_progress":
		return status == "in_progress"
	case "done":
		return status == "done"
	case "all":
		return true
	default:
		return status == "open"
	}
}

// ActiveListLength returns the selectable row count for the active hub page.
func (s *State) ActiveListLength() int {
	switch s.Page {
	case PageSummary:
		if s.Project == nil {
			return 1
		}
		return len(dashboardPages)
	case PageProject:
		if s.Scan != nil && len(s.Scan.Phases) > 0 {
			return len(s.Scan.Phases)
		}
		return 1
	case PageAgents:
		return len(s.AgentItems())
	case PageBugs, PageIssueDetail:
		return len(s.FilteredIssues())
	case PageFeatures, PageFeatureDetail:
		return len(s.activeFeatures())
	case PageAnalysisRun:
		return s.analysisRunListLen()
	case PageAnalysisFiles:
		return s.indexedFileCount()
	case PageEditor:
		return s.editorListLength()
	default:
		return s.secondaryListLength()
	}
}

func (s *State) isViewingDiff() bool {
	return !s.BrowseFiles && s.File == nil && s.Change != nil && len(domain.EffectiveSessionChanges(s.Change)) > 0
}

func (s *State) editorListLength() int {
	if s.isViewingDiff() {
		return s.diffViewRowCount()
	}
	if !s.BrowseFiles && s.File != nil && s.File.Content != "" {
		return max(1, len(strings.Split(strings.ReplaceAll(s.File.Content, "\r\n", "\n"), "\n")))
	}
	return s.indexedFileCount()
}

func (s *State) secondaryListLength() int {
	switch s.Page {
	case PageGit, PageWorktreeDetail:
		if s.GitWorkspace != nil {
			return len(s.GitWorkspace.Worktrees)
		}
		return 0
	case PageHistory:
		return len(s.ChangeHistory)
	case PageDocs, PageDocsComponents, PageDocsDecisions, PageDiagrams:
		return s.docsListLength()
	case PageInstructions, PageModels, PageBootstrap, PageLessons:
		return s.instructionsListLength()
	default:
		return 1
	}
}

func (s *State) openIssues() []IssueRow {
	var out []IssueRow
	for _, item := range s.CollectAllIssues() {
		if item.Status == "open" {
			out = append(out, item)
		}
	}
	return out
}

func (s *State) activeFeatures() []FeatureSuggestion {
	if s.Features == nil {
		return nil
	}
	var out []FeatureSuggestion
	for _, f := range s.Features.Suggestions {
		if f.Status != "dismissed" {
			out = append(out, f)
		}
	}
	return out
}

func (s *State) analysisRunSectionCount() int {
	if s.Run == nil {
		return 0
	}
	n := len(s.Run.Sections)
	if s.Run.Features != nil {
		n++
	}
	return n
}

func (s *State) analysisRunListLen() int {
	return s.analysisRunSectionCount() + len(domain.BuildAnalysisConfigRows(s.EffectiveAnalysisConfig()))
}

func (s *State) indexedFileCount() int {
	if s.Index == nil {
		return 0
	}
	return len(s.Index.Files)
}

func (s *State) docsListLength() int {
	docs := domain.ResolveProjectDocs(s.Overview, s.Project)
	switch s.EffectivePage() {
	case PageDiagrams, PageDocsComponents:
		return len(docs.Components)
	case PageDocsDecisions:
		return len(docs.Decisions)
	default:
		return len(docs.Dependencies)
	}
}

func (s *State) instructionsListLength() int {
	switch s.EffectivePage() {
	case PageModels:
		return len(domain.CanonicalModelScopes()) + len(s.AllModelPresets())
	case PageBootstrap:
		return len(domain.EvaluateBootstrapChecklist(s.Index, s.Instructions))
	case PageLessons:
		return len(s.AllHarnessLessons())
	default:
		return 1
	}
}

// ClampedIdx returns SelectedIdx bounded to [0, ActiveListLength()-1].
func (s *State) ClampedIdx() int {
	length := s.ActiveListLength()
	if length <= 0 {
		return 0
	}
	if s.SelectedIdx < 0 {
		return 0
	}
	if s.SelectedIdx >= length {
		return length - 1
	}
	return s.SelectedIdx
}

// AllHarnessLessons returns the default guardrail lesson plus any dynamic workflow lessons.
func (s *State) AllHarnessLessons() []string {
	const defaultRule = "Propagate context.Context cancellation and add a regression test for every check repair."
	lessons := []string{defaultRule}
	if s.Change != nil && s.Change.Workflow != nil {
		for _, l := range s.Change.Workflow.HarnessLessons {
			trimmed := strings.TrimSpace(l)
			if trimmed != "" && !strings.EqualFold(trimmed, defaultRule) {
				lessons = append(lessons, trimmed)
			}
		}
	}
	return lessons
}

func (s *State) projectID() string {
	if s.Project != nil {
		return s.Project.ProjectID
	}
	return ""
}

func (s *State) projectRevision() string {
	if s.Project != nil {
		return s.Project.ProjectRevision
	}
	return ""
}
