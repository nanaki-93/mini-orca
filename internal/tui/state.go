package tui

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
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
	PageAnalysisRun    Page = "analysis-run"
	PageAnalysisFiles  Page = "analysis-files"
	PageEditor         Page = "editor"
	PageGit            Page = "git"
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
type Palette string

const (
	PaletteTokyo Palette = "tokyo"
	PaletteCRT   Palette = "crt"
	PaletteAmber Palette = "amber"
	PalettePaper Palette = "paper"
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
	Kind    string
	Target  string
	Message string
	Seed    *ChangeSeed
	Issue   *FindingTarget
}

// State holds the complete deterministic state of the Mini-Orca Terminal UI.
type State struct {
	Page              Page
	Palette           Palette
	Connected         bool
	Version           string
	Busy              string
	Error             string
	Notice            string
	SelectedIdx       int
	IssueView         string
	IssueCategory     string
	SelectedIssueIDs  []string
	DocsSection       string
	RulesSection      string
	ShowInlineWhy     bool
	HunkIdx           int
	AgentDetailOpen   bool
	ShowKeybindsModal bool
	Confirmation      *Confirmation
	CmdActive         bool
	CmdInput          string
	CmdHistory        []string
	CmdOutput         string
	GraduatedLessons  []string
	InstructionDraft  string
	ExcludedPaths     []string
	ActivePresetID    string
	PendingActions    []ActionRequest
	Quitting          bool

	Project           *Project
	Index             *ProjectIndex
	Overview          *Overview
	Findings          []Finding
	CompletedFindings []FindingCompletion
	Results           map[string]SectionResults
	SecurityReport    *SecurityReport
	Run               *AnalysisRun
	Selection         *Selection
	File              *FileInfo
	Symbols           []SymbolInfo
	Change            *ChangeSession
	ChangeReceipt     *ChangeMutation
	ChangeHistory     []ChangeHistoryEntry
	Repository        *GitRepository
	GitWorkspace      *GitWorkspace
	Features          *FeatureReport
	Instructions      *InstructionPreview
	Scan              *Scan
	WorkspaceTrust    bool
	Models            *ModelCatalog
}

// HubTabs returns the 5 canonical Mini-Orca TUI hubs and their focused sub-views.
func HubTabs() []HubTab {
	return []HubTab{
		{
			Key:   "1",
			Page:  PageSummary,
			Label: "Overview",
			SubPages: []HubSubPage{
				{Page: PageSummary, Label: "Dashboard"},
				{Page: PageRepo, Label: "Repository"},
				{Page: PageProject, Label: "Tests & Checks"},
			},
		},
		{
			Key:   "2",
			Page:  PageAgents,
			Label: "Agent Studio",
			SubPages: []HubSubPage{
				{Page: PageAgents, Label: "Pipeline"},
				{Page: PageBugs, Label: "Issues"},
				{Page: PageIssueDetail, Label: "Issue Detail"},
				{Page: PageFeatures, Label: "Features"},
				{Page: PageAnalysisRun, Label: "Last Run"},
				{Page: PageAnalysisFiles, Label: "File Scope"},
			},
		},
		{
			Key:   "3",
			Page:  PageEditor,
			Label: "Code & Diff",
			SubPages: []HubSubPage{
				{Page: PageEditor, Label: "Inspector"},
				{Page: PageGit, Label: "Worktrees"},
				{Page: PageHistory, Label: "Proposals"},
			},
		},
		{
			Key:   "4",
			Page:  PageDocs,
			Label: "Handbook & Arch",
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
			Label: "Governance & Rules",
			SubPages: []HubSubPage{
				{Page: PageInstructions, Label: "Trust Policy"},
				{Page: PageModels, Label: "Model Routing"},
				{Page: PageBootstrap, Label: "Bootstrap"},
				{Page: PageLessons, Label: "Lessons"},
			},
		},
	}
}

// NewState initializes a default TUI workspace state.
func NewState() *State {
	return &State{
		Page:           PageSummary,
		Palette:        PaletteTokyo,
		IssueView:      "open",
		IssueCategory:  "all",
		DocsSection:    "context",
		RulesSection:   "trust",
		ShowInlineWhy:  true,
		ActivePresetID: "local-default",
		WorkspaceTrust: true,
		Results:        make(map[string]SectionResults),
		CmdOutput:      "Workspace ready",
	}
}

// ActiveHub returns the HubTab owning the current Page.
func (s *State) ActiveHub() HubTab {
	tabs := HubTabs()
	for _, tab := range tabs {
		if tab.Page == s.Page {
			return tab
		}
		for _, sp := range tab.SubPages {
			if sp.Page == s.Page {
				return tab
			}
		}
	}
	return tabs[0]
}

// Navigate switches the active page, synchronizes section state, and resets selection when switching lists.
func (s *State) Navigate(page Page) {
	if s.Page != page && page != PageIssueDetail {
		s.SelectedIdx = 0
	}
	s.Page = page
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
	agentItems := CollectAgentItems(s.Change, s.ChangeReceipt, s.GitWorkspace)
	var rows []IssueRow
	rows = s.appendSemanticFindings(rows, agentItems)
	rows = s.appendSectionResults(rows, agentItems)
	deduped := DedupeIssues(rows)
	SortIssuesBySeverity(deduped)
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
	target := FindingTarget{ProjectID: firstNonEmpty(f.ProjectID, s.projectID()), Kind: "semantic", ID: f.ID, Path: f.Location.Path}
	title := FindingTitle(f.Title, f.Message)
	name := FindingName(f.Title, f.Message)
	comp := s.findCompletion(target)
	done := f.Status == "fixed" || f.Status == "done" || comp != nil
	agentPath := ""
	if !done {
		agentPath = matchAgentPath(agents, target, name, title)
		if agentPath == "" && f.Status == "in_progress" {
			agentPath = firstNonEmpty(f.Location.Path, f.ID)
		}
	}
	cause := firstNonEmpty(f.Message, f.Evidence, "No cause recorded.")
	sol := "Synthesize a minimal, verified fix and unit test."
	var criteria []string
	if f.TaskSpec != nil && len(f.TaskSpec.AcceptanceCriteria) > 0 {
		criteria = f.TaskSpec.AcceptanceCriteria
		sol = strings.Join(criteria, "\n")
	}
	commitOID := ""
	if comp != nil {
		commitOID = comp.CommitOID
	}
	rows = append(rows, IssueRow{
		ID:                 id,
		Title:              title,
		Name:               name,
		Path:               f.Location.Path,
		Symbol:             f.Location.Symbol,
		Line:               f.Location.StartLine,
		Severity:           f.Severity,
		Category:           cat,
		Status:             issueStatus(done, agentPath),
		CommitOID:          commitOID,
		AgentPath:          agentPath,
		Cause:              cause,
		Solution:           sol,
		Target:             target,
		AcceptanceCriteria: criteria,
	})
	return rows
}

func (s *State) pushPerformanceIssues(rows []IssueRow, rep PerformanceReport, agents []AgentItem) []IssueRow {
	for _, f := range rep.Findings {
		id := fmt.Sprintf("performance:%s:%s", rep.Path, f.ID)
		if containsIssueID(rows, id) {
			continue
		}
		target := FindingTarget{ProjectID: firstNonEmpty(rep.ProjectID, s.projectID()), Kind: "performance", ID: f.ID, Path: rep.Path}
		name := FindingName(f.Title, f.ObservedPattern)
		comp := s.findCompletion(target)
		done := comp != nil
		agentPath := ""
		if !done {
			agentPath = matchAgentPath(agents, target, name, f.Title)
		}
		commitOID := ""
		if comp != nil {
			commitOID = comp.CommitOID
		}
		rows = append(rows, IssueRow{
			ID:        id,
			Title:     f.Title,
			Name:      name,
			Path:      rep.Path,
			Symbol:    f.Symbol,
			Line:      f.StartLine,
			Severity:  f.PotentialImpact,
			Category:  "performance",
			Status:    issueStatus(done, agentPath),
			CommitOID: commitOID,
			AgentPath: agentPath,
			Cause:     firstNonEmpty(f.ObservedPattern, "No cause recorded."),
			Solution:  firstNonEmpty(f.Recommendation, "Synthesize a minimal, verified fix and unit test."),
			Target:    target,
		})
	}
	return rows
}

func (s *State) pushSecurityIssues(rows []IssueRow, rep SecurityReport, agents []AgentItem) []IssueRow {
	for _, f := range rep.Findings {
		if f.Triage == "dismissed" {
			continue
		}
		path := firstNonEmpty(f.SourceAnchor.Path, rep.Path)
		src := firstNonEmpty(rep.Source, "deterministic")
		id := fmt.Sprintf("security:%s:%s:%s", rep.Path, src, f.ID)
		if containsIssueID(rows, id) {
			continue
		}
		target := FindingTarget{ProjectID: firstNonEmpty(rep.ProjectID, s.projectID()), Kind: "security", ID: f.ID, Path: path, Source: src}
		name := FindingName(f.Title, f.ObservedCondition)
		comp := s.findCompletion(target)
		done := f.Triage == "fixed" || comp != nil
		agentPath := ""
		if !done {
			agentPath = matchAgentPath(agents, target, name, f.Title)
		}
		commitOID := ""
		if comp != nil {
			commitOID = comp.CommitOID
		}
		rows = append(rows, IssueRow{
			ID:        id,
			Title:     f.Title,
			Name:      name,
			Path:      path,
			Symbol:    f.SourceAnchor.Symbol,
			Line:      f.SourceAnchor.StartLine,
			Severity:  f.Severity,
			Category:  "security",
			Status:    issueStatus(done, agentPath),
			CommitOID: commitOID,
			AgentPath: agentPath,
			Cause:     firstNonEmpty(f.ObservedCondition, "No cause recorded."),
			Solution:  firstNonEmpty(f.Remediation, "Synthesize a minimal, verified fix and unit test."),
			Target:    target,
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
		c := &s.CompletedFindings[i]
		if sameFindingTarget(c.FindingTarget, target) {
			return c
		}
	}
	if s.Change != nil && s.Change.State == "applied" {
		for _, t := range s.Change.FindingTargets {
			if sameFindingTarget(t, target) {
				oid := ""
				if s.Change.Commit != nil {
					oid = s.Change.Commit.OID
				}
				return &FindingCompletion{FindingTarget: target, CommitOID: oid}
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
				return &FindingCompletion{FindingTarget: target, CommitOID: oid}
			}
		}
	}
	return nil
}

func matchAgentPath(agents []AgentItem, target FindingTarget, name, title string) string {
	lowName := strings.ToLower(strings.TrimSpace(name))
	lowTitle := strings.ToLower(strings.TrimSpace(title))
	for _, item := range agents {
		if item.Status == "applied" || item.Status == "reverted" || item.Status == "canceled" {
			continue
		}
		for _, t := range item.FindingTargets {
			if sameFindingTarget(t, target) {
				return firstNonEmpty(item.Path, item.ID)
			}
		}
		lowItem := strings.ToLower(strings.TrimSpace(item.Title))
		if lowItem != "" && (lowItem == lowName || lowItem == lowTitle) {
			return firstNonEmpty(item.Path, item.ID)
		}
	}
	return ""
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

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
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
		open := s.openIssues()
		return min(8, len(open))
	case PageProject:
		if s.Scan != nil && len(s.Scan.Phases) > 0 {
			return len(s.Scan.Phases)
		}
		return 1
	case PageAgents:
		return len(CollectAgentItems(s.Change, s.ChangeReceipt, s.GitWorkspace))
	case PageBugs:
		return len(s.FilteredIssues())
	case PageFeatures:
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

func (s *State) editorListLength() int {
	if s.Change != nil && len(s.Change.Changes) > 0 {
		return max(1, len(s.Change.Changes[0].Diff.Lines))
	}
	if s.File != nil && s.File.Content != "" {
		return max(1, len(strings.Split(s.File.Content, "\n")))
	}
	return s.indexedFileCount()
}

func (s *State) secondaryListLength() int {
	switch s.Page {
	case PageGit:
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

func (s *State) analysisRunListLen() int {
	if s.Run == nil {
		return 0
	}
	n := len(s.Run.Sections)
	if s.Run.Features != nil {
		n++
	}
	return n
}

func (s *State) indexedFileCount() int {
	if s.Index == nil {
		return 0
	}
	return len(s.Index.Files)
}

func (s *State) docsListLength() int {
	docs := ResolveProjectDocs(s.Overview, s.Project)
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
		return len(DefaultModelPresets())
	case PageBootstrap:
		return len(EvaluateBootstrapChecklist(s.Index, s.Instructions))
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

// SyncFromDaemon loads the initial workspace snapshot from the local daemon via Client.
func (s *State) SyncFromDaemon(ctx context.Context, client *Client, projectPath string) error {
	var status struct {
		Version string `json:"version"`
	}
	if err := client.DoJSON(ctx, "sync-status", http.MethodGet, "/status", nil, &status); err != nil {
		s.Connected = false
		s.Error = err.Error()
		return err
	}
	s.Connected = true
	s.Version = status.Version
	s.Error = ""

	if projectPath != "" {
		s.openOrRestoreProject(ctx, client, projectPath)
	}
	if s.Project == nil {
		var proj Project
		if err := client.DoJSON(ctx, "sync-project", http.MethodGet, "/api/projects/current", nil, &proj); err == nil {
			s.Project = &proj
		}
	}
	if s.Project != nil {
		s.loadProjectResources(ctx, client)
	}
	return nil
}

func (s *State) openOrRestoreProject(ctx context.Context, client *Client, projectPath string) {
	var proj Project
	body := map[string]string{"project_path": projectPath}
	if err := client.DoJSON(ctx, "sync-restore", http.MethodPost, "/api/projects/restore", body, &proj); err == nil {
		s.Project = &proj
		return
	}
	importBody := map[string]any{"project_path": projectPath, "confirm_remote_provider": true}
	if err := client.DoJSON(ctx, "sync-import", http.MethodPost, "/api/projects/import", importBody, &proj); err == nil {
		s.Project = &proj
	}
}

func (s *State) loadProjectResources(ctx context.Context, client *Client) {
	revQuery, idQuery := s.resourceQueries()
	var idx ProjectIndex
	if err := client.DoJSON(ctx, "sync-index", http.MethodGet, "/api/projects/current/index", nil, &idx); err == nil && s.AcceptProjectResponse(idx.ProjectIdentity) {
		s.Index = &idx
	}
	var ov Overview
	if err := client.DoJSON(ctx, "sync-overview", http.MethodGet, "/api/projects/current/overview"+revQuery, nil, &ov); err == nil && s.AcceptProjectResponse(ov.ProjectIdentity) {
		s.Overview = &ov
		if ov.AnalysisRun != nil {
			s.Run = ov.AnalysisRun
		}
	}
	var fResp FindingsResponse
	if err := client.DoJSON(ctx, "sync-findings", http.MethodGet, "/api/projects/current/findings"+revQuery, nil, &fResp); err == nil && s.AcceptProjectResponse(fResp.ProjectIdentity) {
		s.Findings = fResp.Findings
		s.CompletedFindings = fResp.Completed
	}
	var gitWS GitWorkspace
	if err := client.DoJSON(ctx, "sync-git", http.MethodGet, "/api/projects/current/git/workspace"+idQuery, nil, &gitWS); err == nil {
		s.GitWorkspace = &gitWS
		s.Repository = &gitWS.GitRepository
	}
	s.syncExecutionTrust(ctx, client, revQuery)
	if idQuery != "" {
		s.loadGuardedProjectResources(ctx, client, idQuery, revQuery)
	}
}

func (s *State) resourceQueries() (string, string) {
	revQuery := ""
	if s.Project.ProjectRevision != "" {
		revQuery = "?project_revision=" + url.QueryEscape(s.Project.ProjectRevision)
	}
	idQuery := ""
	if s.Project.ProjectID != "" && s.Project.ProjectRevision != "" {
		idQuery = fmt.Sprintf("?project_id=%s&project_revision=%s", url.QueryEscape(s.Project.ProjectID), url.QueryEscape(s.Project.ProjectRevision))
	}
	return revQuery, idQuery
}

func (s *State) syncExecutionTrust(ctx context.Context, client *Client, revQuery string) {
	var trust ExecutionTrust
	if err := client.DoJSON(ctx, "sync-trust", http.MethodGet, "/api/projects/current/execution-trust"+revQuery, nil, &trust); err == nil {
		if trust.Trusted {
			s.WorkspaceTrust = true
			return
		}
		if s.WorkspaceTrust && s.Project.ProjectRevision != "" {
			_ = s.ensureExecutionTrust(ctx, client, "sync-trust-grant")
			return
		}
		s.WorkspaceTrust = trust.Trusted
	}
}

func (s *State) loadGuardedProjectResources(ctx context.Context, client *Client, idQuery, revQuery string) {
	var feat FeatureReport
	if err := client.DoJSON(ctx, "sync-features", http.MethodGet, "/api/projects/current/features"+idQuery, nil, &feat); err == nil {
		s.Features = &feat
	}
	var history []ChangeHistoryEntry
	if err := client.DoJSON(ctx, "sync-changes", http.MethodGet, "/api/projects/current/changes"+idQuery, nil, &history); err == nil {
		s.ChangeHistory = history
	}
	var rec ChangeMutation
	if err := client.DoJSON(ctx, "sync-recovery", http.MethodGet, "/api/projects/current/changes/recovery"+idQuery, nil, &rec); err == nil && rec.SessionID != "" {
		s.ChangeReceipt = &rec
	}
	if s.Change == nil && len(s.ChangeHistory) > 0 && s.ChangeHistory[0].ID != "" {
		var chg ChangeSession
		target := "/api/projects/current/changes/" + url.PathEscape(s.ChangeHistory[0].ID) + idQuery
		if err := client.DoJSON(ctx, "sync-latest-change", http.MethodGet, target, nil, &chg); err == nil && chg.ID != "" {
			s.Change = &chg
		}
	}
	s.loadAuxResources(ctx, client, idQuery, revQuery)
}

func (s *State) loadAuxResources(ctx context.Context, client *Client, idQuery, revQuery string) {
	var instr InstructionPreview
	instrTarget := "/api/projects/current/instructions" + idQuery + "&path=AGENTS.md"
	if err := client.DoJSON(ctx, "sync-instructions", http.MethodGet, instrTarget, nil, &instr); err == nil && instr.Path != "" {
		s.Instructions = &instr
	}
	var sel Selection
	if err := client.DoJSON(ctx, "sync-selection", http.MethodGet, "/api/projects/current/analysis/selection"+idQuery, nil, &sel); err == nil && sel.SelectionID != "" {
		s.Selection = &sel
		s.ExcludedPaths = sel.ExcludedPaths
	}
	var scan Scan
	if err := client.DoJSON(ctx, "sync-scan", http.MethodGet, "/api/projects/current/scan"+revQuery, nil, &scan); err == nil && (scan.Status != "" || len(scan.Phases) > 0) {
		s.Scan = &scan
	}
	var models ModelCatalog
	if err := client.DoJSON(ctx, "sync-models", http.MethodGet, "/api/models/current", nil, &models); err == nil && len(models.Scopes) > 0 {
		s.Models = &models
	}
	s.loadAnalysisSectionResults(ctx, client)
}

func (s *State) loadAnalysisSectionResults(ctx context.Context, client *Client) {
	if s.Run == nil || s.Run.Identity.ID == "" || s.Run.Identity.QueueID == "" {
		return
	}
	id := s.Run.Identity
	base := fmt.Sprintf(
		"/api/projects/current/analysis/results?project_id=%s&project_revision=%s&policy_fingerprint=%s&provider_fingerprint=%s&queue_id=%s&id=%s&generation=%s",
		url.QueryEscape(id.ProjectID), url.QueryEscape(id.ProjectRevision),
		url.QueryEscape(id.PolicyFingerprint), url.QueryEscape(id.ProviderFingerprint),
		url.QueryEscape(id.QueueID), url.QueryEscape(id.ID), url.QueryEscape(id.Generation),
	)
	for _, cat := range []string{"bugs", "security", "performance"} {
		var sec SectionResults
		if err := client.DoJSON(ctx, "sync-res-"+cat, http.MethodGet, base+"&category="+cat, nil, &sec); err == nil {
			if s.Results == nil {
				s.Results = make(map[string]SectionResults)
			}
			s.Results[cat] = sec
		}
	}
}

// PollActiveJobs refreshes in-progress workflow, analysis, or scan states from the daemon.
func (s *State) PollActiveJobs(ctx context.Context, client *Client) {
	if client == nil || s.Project == nil || s.Project.ProjectID == "" || s.Project.ProjectRevision == "" {
		return
	}
	revQuery, idQuery := s.resourceQueries()
	if s.hasRunningWorkflow() {
		var chg ChangeSession
		target := "/api/projects/current/changes/" + url.PathEscape(s.Change.ID) + idQuery
		if err := client.DoJSON(ctx, "poll-change", http.MethodGet, target, nil, &chg); err == nil && chg.ID != "" {
			s.Change = &chg
		}
	}
	if s.hasRunningAnalysis() {
		var run AnalysisRun
		if err := client.DoJSON(ctx, "poll-analysis", http.MethodGet, "/api/projects/current/analysis/run"+idQuery, nil, &run); err == nil && run.Identity.ID != "" {
			s.Run = &run
			s.loadAnalysisSectionResults(ctx, client)
		}
	}
	if s.Scan != nil && s.Scan.Status == "running" {
		var scan Scan
		if err := client.DoJSON(ctx, "poll-scan", http.MethodGet, "/api/projects/current/scan"+revQuery, nil, &scan); err == nil && scan.Status != "" {
			s.Scan = &scan
		}
	}
}

func (s *State) hasActiveBackgroundJob() bool {
	return s.hasRunningWorkflow() || s.hasRunningAnalysis() || (s.Scan != nil && s.Scan.Status == "running")
}

func (s *State) hasRunningWorkflow() bool {
	if s.Change == nil || s.Change.ID == "" || s.Change.ID == "agent-session" || s.Change.Workflow == nil {
		return false
	}
	st := s.Change.Workflow.Status
	return st == "running" || st == "queued"
}

func (s *State) hasRunningAnalysis() bool {
	if s.Run == nil {
		return false
	}
	switch s.Run.Status {
	case "running", "queued", "pausing", "canceling":
		return true
	default:
		return false
	}
}
