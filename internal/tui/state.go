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
	PageSummary       Page = "summary"
	PageProject       Page = "project"
	PageAgents        Page = "agents"
	PageBugs          Page = "bugs"
	PageFeatures      Page = "features"
	PageAnalysisRun   Page = "analysis-run"
	PageAnalysisFiles Page = "analysis-files"
	PageEditor        Page = "editor"
	PageGit           Page = "git"
	PageHistory       Page = "history"
	PageDocs          Page = "docs"
	PageDiagrams      Page = "diagrams"
	PageInstructions  Page = "instructions"
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

// HubTabs returns the 5 canonical Mini-Orca TUI hubs and their sub-views.
func HubTabs() []HubTab {
	return []HubTab{
		{
			Key:   "1",
			Page:  PageSummary,
			Label: "Overview",
			SubPages: []HubSubPage{
				{Page: PageSummary, Label: "Dashboard"},
				{Page: PageProject, Label: "Project & Tests"},
			},
		},
		{
			Key:   "2",
			Page:  PageAgents,
			Label: "Agent Studio",
			SubPages: []HubSubPage{
				{Page: PageAgents, Label: "Agents"},
				{Page: PageBugs, Label: "Issues"},
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
				{Page: PageEditor, Label: "Code & Diff"},
				{Page: PageGit, Label: "Git & Worktrees"},
				{Page: PageHistory, Label: "Saved Proposals"},
			},
		},
		{
			Key:   "4",
			Page:  PageDocs,
			Label: "Handbook & Arch",
			SubPages: []HubSubPage{
				{Page: PageDocs, Label: "Handbook / Docs"},
				{Page: PageDiagrams, Label: "Architecture"},
			},
		},
		{
			Key:      "5",
			Page:     PageInstructions,
			Label:    "Governance & Rules",
			SubPages: nil,
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
		CmdOutput:      "Press [?] or type :keys for keyboard shortcuts · [1-5] switch hubs · [Tab] sub-view · [j/k] move · [Enter] select · [:q] quit",
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

// Navigate switches the active page and resets the row selection index.
func (s *State) Navigate(page Page) {
	s.Page = page
	s.SelectedIdx = 0
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
	target := FindingTarget{ProjectID: f.ProjectID, Kind: "semantic", ID: f.ID, Path: f.Location.Path}
	title := FindingTitle(f.Title, f.Message)
	name := FindingName(f.Title, f.Message)
	comp := s.findCompletion(target)
	done := f.Status == "fixed" || comp != nil
	agentPath := ""
	if !done {
		agentPath = matchAgentPath(agents, target, name, title)
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
		target := FindingTarget{ProjectID: rep.ProjectID, Kind: "performance", ID: f.ID, Path: rep.Path}
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
		id := fmt.Sprintf("security:%s:%s:%s", rep.Path, rep.Source, f.ID)
		if containsIssueID(rows, id) {
			continue
		}
		target := FindingTarget{ProjectID: rep.ProjectID, Kind: "security", ID: f.ID, Path: path, Source: rep.Source}
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

func (s *State) findCompletion(target FindingTarget) *FindingCompletion {
	for i := range s.CompletedFindings {
		c := &s.CompletedFindings[i]
		if c.Kind == target.Kind && c.ID == target.ID && c.Path == target.Path && c.Source == target.Source {
			return c
		}
	}
	return nil
}

func matchAgentPath(agents []AgentItem, target FindingTarget, name, title string) string {
	lowName := strings.ToLower(strings.TrimSpace(name))
	lowTitle := strings.ToLower(strings.TrimSpace(title))
	for _, item := range agents {
		for _, t := range item.FindingTargets {
			if t.Kind == target.Kind && t.ID == target.ID && t.Path == target.Path {
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

func matchesIssueView(status, view string) bool {
	switch view {
	case "in_progress":
		return status == "in_progress"
	case "done":
		return status == "done"
	default:
		return status != "done"
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
	case PageAnalysisFiles, PageEditor:
		return s.indexedFileCount()
	default:
		return s.secondaryListLength()
	}
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
	case PageDocs, PageDiagrams:
		return s.docsListLength()
	case PageInstructions:
		return s.instructionsListLength()
	default:
		return 0
	}
}

func (s *State) openIssues() []IssueRow {
	var out []IssueRow
	for _, item := range s.CollectAllIssues() {
		if item.Status != "done" {
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
	if s.Page == PageDiagrams || s.DocsSection == "components" {
		return len(docs.Components)
	}
	switch s.DocsSection {
	case "scenarios":
		return len(docs.Scenarios)
	case "decisions":
		return len(docs.Decisions)
	default:
		return len(docs.Dependencies)
	}
}

func (s *State) instructionsListLength() int {
	switch s.RulesSection {
	case "models":
		return len(DefaultModelPresets())
	case "bootstrap":
		return len(EvaluateBootstrapChecklist(s.Index, s.Instructions))
	case "lessons":
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
		var proj Project
		body := map[string]string{"project_path": projectPath}
		if err := client.DoJSON(ctx, "sync-restore", http.MethodPost, "/api/projects/restore", body, &proj); err == nil {
			s.Project = &proj
		}
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

func (s *State) loadProjectResources(ctx context.Context, client *Client) {
	revQuery := ""
	if s.Project.ProjectRevision != "" {
		revQuery = "?project_revision=" + url.QueryEscape(s.Project.ProjectRevision)
	}
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
	if err := client.DoJSON(ctx, "sync-git", http.MethodGet, "/api/projects/current/git/workspace", nil, &gitWS); err == nil {
		s.GitWorkspace = &gitWS
		s.Repository = &gitWS.GitRepository
	}
	var trust ExecutionTrust
	if err := client.DoJSON(ctx, "sync-trust", http.MethodGet, "/api/projects/current/execution-trust", nil, &trust); err == nil {
		s.WorkspaceTrust = trust.Trusted
	}
	if s.Project.ProjectID != "" && s.Project.ProjectRevision != "" {
		idQuery := fmt.Sprintf("?project_id=%s&project_revision=%s", url.QueryEscape(s.Project.ProjectID), url.QueryEscape(s.Project.ProjectRevision))
		var feat FeatureReport
		if err := client.DoJSON(ctx, "sync-features", http.MethodGet, "/api/projects/current/features"+idQuery, nil, &feat); err == nil {
			s.Features = &feat
		}
		var history []ChangeHistoryEntry
		if err := client.DoJSON(ctx, "sync-changes", http.MethodGet, "/api/projects/current/changes"+idQuery, nil, &history); err == nil {
			s.ChangeHistory = history
		}
	}
}
