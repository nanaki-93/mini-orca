package tui

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

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
		if s.Project.Path == "" && projectPath != "" {
			s.Project.Path = projectPath
		}
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
	s.loadCustomPresetsFromDisk()
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
	projRev := s.projectRevision()
	if s.Change != nil && !isUsableInitialChange(s.Change, projRev) {
		s.Change = nil
	}
	if s.Change == nil && len(s.ChangeHistory) > 0 {
		if chgID := pickInitialChangeID(s.ChangeHistory, projRev); chgID != "" {
			var chg ChangeSession
			target := "/api/projects/current/changes/" + url.PathEscape(chgID) + idQuery
			if err := client.DoJSON(ctx, "sync-latest-change", http.MethodGet, target, nil, &chg); err == nil && isUsableInitialChange(&chg, projRev) {
				s.Change = &chg
				s.upsertActiveChange(&chg)
			}
		}
	}
	s.syncActiveHistoryChanges(ctx, client, idQuery, projRev)
	s.loadAuxResources(ctx, client, idQuery, revQuery)
}

func (s *State) syncActiveHistoryChanges(ctx context.Context, client *Client, idQuery, projRev string) {
	for idx, entry := range s.ChangeHistory {
		if (entry.WorkflowStatus != "running" && entry.WorkflowStatus != "canceling") || !isUsableHistoryEntry(entry, projRev) {
			continue
		}
		if s.Change != nil && s.Change.ID == entry.ID {
			continue
		}
		var chg ChangeSession
		target := "/api/projects/current/changes/" + url.PathEscape(entry.ID) + idQuery
		if err := client.DoJSON(ctx, fmt.Sprintf("sync-active-change-%d", idx), http.MethodGet, target, nil, &chg); err == nil && isUsableInitialChange(&chg, projRev) {
			s.upsertActiveChange(&chg)
		}
	}
}

func pickInitialChangeID(history []ChangeHistoryEntry, projectRev string) string {
	for _, entry := range history {
		if isUsableHistoryEntry(entry, projectRev) {
			return entry.ID
		}
	}
	return ""
}

func isUsableHistoryEntry(entry ChangeHistoryEntry, projectRev string) bool {
	if entry.ID == "" || entry.Revision <= 0 || entry.Freshness == "stale" {
		return false
	}
	if isInactiveAgentStatus(entry.State) || isInactiveAgentStatus(entry.WorkflowStatus) {
		return false
	}
	if projectRev != "" && entry.ProjectRevision != "" && entry.ProjectRevision != projectRev {
		return false
	}
	return true
}

func isUsableInitialChange(chg *ChangeSession, projectRev string) bool {
	if chg == nil || chg.ID == "" || chg.Freshness == "stale" {
		return false
	}
	if isInactiveAgentStatus(chg.State) {
		return false
	}
	if chg.Workflow != nil && isInactiveAgentStatus(chg.Workflow.Status) {
		return false
	}
	if projectRev != "" && chg.ProjectRevision != "" && chg.ProjectRevision != projectRev {
		return false
	}
	return true
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
	_ = s.RefreshModels(ctx, client)
	s.loadAnalysisSectionResults(ctx, client)
}

// RefreshModels fetches the active scoped models and discovered model catalog from the daemon.
func (s *State) RefreshModels(ctx context.Context, client *Client) error {
	var models ModelCatalog
	errCur := client.DoJSON(ctx, "sync-models", http.MethodGet, "/api/models/current", nil, &models)
	if errCur == nil && len(models.Scopes) > 0 {
		s.Models = &models
	}
	var avail AvailableModels
	if err := client.DoJSON(ctx, "sync-models-avail", http.MethodGet, "/api/models/available", nil, &avail); err == nil {
		s.AvailableModels = &avail
	}
	return errCur
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
	s.pollRunningWorkflowSessions(ctx, client, idQuery)
	if s.hasRunningAnalysis() {
		s.pollRunningAnalysis(ctx, client, idQuery, revQuery)
	}
	if s.Scan != nil && s.Scan.Status == "running" {
		var scan Scan
		if err := client.DoJSON(ctx, "poll-scan", http.MethodGet, "/api/projects/current/scan"+revQuery, nil, &scan); err == nil && scan.Status != "" {
			s.Scan = &scan
		}
	}
}

func (s *State) pollRunningWorkflowSessions(ctx context.Context, client *Client, idQuery string) {
	for idx, running := range s.runningWorkflowSessions() {
		var chg ChangeSession
		target := "/api/projects/current/changes/" + url.PathEscape(running.ID) + idQuery
		reqID := "poll-change"
		if idx > 0 {
			reqID = fmt.Sprintf("poll-change-%d", idx)
		}
		if err := client.DoJSON(ctx, reqID, http.MethodGet, target, nil, &chg); err == nil && chg.ID != "" {
			s.upsertActiveChange(&chg)
			if s.Change != nil && s.Change.ID == chg.ID {
				s.Change = &chg
			}
			s.announceWorkflowTransition(&chg)
		}
	}
}

func (s *State) pollRunningAnalysis(ctx context.Context, client *Client, idQuery, revQuery string) {
	var run AnalysisRun
	if err := client.DoJSON(ctx, "poll-analysis", http.MethodGet, "/api/projects/current/analysis/run"+idQuery, nil, &run); err != nil || run.Identity.ID == "" {
		return
	}
	s.Run = &run
	s.loadAnalysisSectionResults(ctx, client)
	if s.hasRunningAnalysis() {
		return
	}
	s.announceAnalysisTransition(&run)
	var fResp FindingsResponse
	if err := client.DoJSON(ctx, "poll-findings", http.MethodGet, "/api/projects/current/findings"+revQuery, nil, &fResp); err == nil && s.AcceptProjectResponse(fResp.ProjectIdentity) {
		s.Findings = fResp.Findings
		s.CompletedFindings = fResp.Completed
	}
}

func (s *State) announceAnalysisTransition(run *AnalysisRun) {
	if run == nil {
		return
	}
	switch run.Status {
	case "paused", "interrupted":
		s.CmdOutput = fmt.Sprintf("Analysis %s: %s — press [r] to resume", run.Status, client.FirstNonEmpty(run.Reason, "window completed"))
	case "completed", "completed_empty":
		s.CmdOutput = "Analysis completed — press [2] Agent Studio to inspect findings"
	case "failed":
		s.CmdOutput = fmt.Sprintf("Analysis failed: %s — press [r] to retry", client.FirstNonEmpty(run.Reason, "inspect Last Run for details"))
	}
}

func (s *State) announceWorkflowTransition(chg *ChangeSession) {
	if chg == nil || chg.Workflow == nil {
		return
	}
	switch chg.Workflow.Status {
	case "awaiting_human_review", "completed":
		s.CmdOutput = fmt.Sprintf("Agent completed %q — press [d/Enter] to inspect diff or [c] to Apply & Commit", chg.Title)
	case "changes_requested", "failed":
		s.CmdOutput = fmt.Sprintf("Agent workflow %s for %q — press [r] to self-repair or [g] to debug", chg.Workflow.Status, chg.Title)
	case "canceled":
		s.CmdOutput = fmt.Sprintf("Canceled agent workflow for %q", chg.Title)
	}
}

func (s *State) hasActiveBackgroundJob() bool {
	return len(s.runningWorkflowSessions()) > 0 || s.hasRunningAnalysis() || (s.Scan != nil && s.Scan.Status == "running")
}

func isChangeSessionWorkflowRunning(chg *ChangeSession) bool {
	if chg == nil || chg.ID == "" || isLocalPreviewChangeID(chg.ID) || chg.Workflow == nil {
		return false
	}
	st := chg.Workflow.Status
	return st == "running" || st == "queued" || st == "canceling"
}

func (s *State) hasRunningWorkflow() bool {
	return isChangeSessionWorkflowRunning(s.Change)
}

func (s *State) runningWorkflowSessions() []*ChangeSession {
	seen := make(map[string]bool)
	var out []*ChangeSession
	if isChangeSessionWorkflowRunning(s.Change) {
		seen[s.Change.ID] = true
		out = append(out, s.Change)
	}
	for _, chg := range s.ActiveChanges {
		if isChangeSessionWorkflowRunning(chg) && !seen[chg.ID] {
			seen[chg.ID] = true
			out = append(out, chg)
		}
	}
	return out
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

func (s *State) ensureExecutionTrust(ctx context.Context, cl *Client, reqID string) error {
	if !s.WorkspaceTrust || s.Project == nil || s.Project.ProjectRevision == "" {
		return nil
	}
	body := map[string]any{
		"project_revision": s.Project.ProjectRevision,
		"confirm":          true,
	}
	var trust ExecutionTrust
	if err := cl.DoJSON(ctx, reqID, http.MethodPost, "/api/projects/current/execution-trust", body, &trust); err != nil {
		return err
	}
	s.WorkspaceTrust = trust.Trusted || s.WorkspaceTrust
	return nil
}

func (s *State) fetchFileInfo(ctx context.Context, cl *Client, reqID, path string) error {
	var info FileInfo
	escaped := url.QueryEscape(path)
	if err := cl.DoJSON(ctx, reqID, http.MethodGet, "/api/projects/current/files/info?path="+escaped, nil, &info); err != nil {
		return err
	}
	if s.AcceptFileResponse(info.Path, "") {
		s.File = &info
	}
	var symResp struct {
		Symbols []SymbolInfo `json:"symbols"`
	}
	if err := cl.DoJSON(ctx, reqID+"-sym", http.MethodGet, "/api/projects/current/files/symbols?path="+escaped, nil, &symResp); err == nil {
		s.Symbols = symResp.Symbols
	}
	return nil
}

func (s *State) syncProjectRevision(ctx context.Context, cl *Client, reqID string) {
	oldRev := s.projectRevision()
	var proj Project
	if err := cl.DoJSON(ctx, reqID+"-proj", http.MethodGet, "/api/projects/current", nil, &proj); err == nil && proj.ProjectID != "" {
		s.Project = &proj
	}
	var idx ProjectIndex
	if err := cl.DoJSON(ctx, reqID+"-idx", http.MethodGet, "/api/projects/current/index", nil, &idx); err == nil && s.AcceptProjectResponse(idx.ProjectIdentity) {
		s.Index = &idx
		if s.Project != nil && idx.ProjectRevision != "" {
			s.Project.ProjectRevision = idx.ProjectRevision
		}
	}
	if s.projectRevision() == oldRev && len(s.runningWorkflowSessions()) == 0 {
		_ = s.dispatchReindex(ctx, cl, reqID+"-reidx")
	}
}

func (s *State) recordAppliedFindingCompletions(targets []FindingTarget, commit *ChangeCommit) {
	oid := ""
	if commit != nil {
		oid = commit.OID
	}
	for _, ft := range targets {
		if s.findCompletion(ft) == nil {
			s.CompletedFindings = append(s.CompletedFindings, FindingCompletion{
				FindingTarget: ft,
				CommitOID:     oid,
			})
		}
	}
}

func (s *State) removeFindingCompletions(targets []FindingTarget) {
	if len(targets) == 0 || len(s.CompletedFindings) == 0 {
		return
	}
	var kept []FindingCompletion
	for _, c := range s.CompletedFindings {
		matched := false
		for _, t := range targets {
			if sameFindingTarget(c.FindingTarget, t) {
				matched = true
				break
			}
		}
		if !matched {
			kept = append(kept, c)
		}
	}
	s.CompletedFindings = kept
}

func (s *State) activeGitIdentity() (string, string) {
	if s.Change != nil && s.Change.Git != nil && s.Change.Git.Head != "" && s.Change.Git.Branch != "" {
		return s.Change.Git.Head, s.Change.Git.Branch
	}
	if s.Repository != nil && s.Repository.Head != "" && s.Repository.Branch != "" {
		return s.Repository.Head, s.Repository.Branch
	}
	if s.GitWorkspace != nil && s.GitWorkspace.Head != "" && s.GitWorkspace.Branch != "" {
		return s.GitWorkspace.Head, s.GitWorkspace.Branch
	}
	return "", ""
}

func (s *State) markFindingDismissed(target FindingTarget) {
	for i := range s.Findings {
		if s.Findings[i].ID == target.ID {
			s.Findings[i].Status = "dismissed"
		}
	}
	if s.SecurityReport != nil {
		for i := range s.SecurityReport.Findings {
			if s.SecurityReport.Findings[i].ID == target.ID {
				s.SecurityReport.Findings[i].Triage = "dismissed"
			}
		}
	}
}
