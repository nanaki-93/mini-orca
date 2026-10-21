package tui

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
)

var tuiReqSeq atomic.Uint64

func nextTUIReqID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, tuiReqSeq.Add(1))
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
	s.loadIssueHistoryFromDisk()
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

func (s *State) loadAnalysisSectionResults(ctx context.Context, cl *Client) {
	if s.Run == nil || s.Run.Identity.ID == "" || s.Run.Identity.QueueID == "" {
		return
	}
	fetched := fetchAnalysisSectionResults(ctx, cl, s.Run.Identity)
	if len(fetched) == 0 {
		return
	}
	if s.Results == nil {
		s.Results = make(map[string]SectionResults)
	}
	for cat, sec := range fetched {
		s.Results[cat] = sec
	}
}

func fetchAnalysisSectionResults(ctx context.Context, cl *Client, id RunIdentity) map[string]SectionResults {
	if cl == nil || id.ID == "" || id.QueueID == "" {
		return nil
	}
	base := fmt.Sprintf(
		"/api/projects/current/analysis/results?project_id=%s&project_revision=%s&policy_fingerprint=%s&provider_fingerprint=%s&queue_id=%s&id=%s&generation=%s",
		url.QueryEscape(id.ProjectID), url.QueryEscape(id.ProjectRevision),
		url.QueryEscape(id.PolicyFingerprint), url.QueryEscape(id.ProviderFingerprint),
		url.QueryEscape(id.QueueID), url.QueryEscape(id.ID), url.QueryEscape(id.Generation),
	)
	cats := []string{"bugs", "security", "performance"}
	type catResult struct {
		cat string
		sec SectionResults
		ok  bool
	}
	fetched := make([]catResult, len(cats))
	var wg sync.WaitGroup
	for i, cat := range cats {
		wg.Add(1)
		go func(idx int, category string) {
			defer wg.Done()
			var sec SectionResults
			reqID := nextTUIReqID("sync-res-" + category)
			if err := cl.DoJSON(ctx, reqID, http.MethodGet, base+"&category="+category, nil, &sec); err == nil {
				fetched[idx] = catResult{cat: category, sec: sec, ok: true}
			}
		}(i, cat)
	}
	wg.Wait()
	out := make(map[string]SectionResults, len(cats))
	for _, item := range fetched {
		if item.ok {
			out[item.cat] = item.sec
		}
	}
	return out
}

type workflowPollTarget struct {
	id             string
	worktreePath   string
	query          string
	findingTargets []FindingTarget
}

type pollSnapshot struct {
	idQuery      string
	revQuery     string
	workflows    []workflowPollTarget
	pollAnalysis bool
	prevRun      *AnalysisRun
	emptyResults bool
	pollScan     bool
}

type pollResult struct {
	changes        []ChangeSession
	changeOK       []bool
	hasRun         bool
	run            AnalysisRun
	sectionResults map[string]SectionResults
	hasFindings    bool
	findingsResp   FindingsResponse
	hasScan        bool
	scan           Scan
}

// PollActiveJobs refreshes in-progress workflow, analysis, or scan states from the daemon concurrently.
func (s *State) PollActiveJobs(ctx context.Context, cl *Client) {
	snap, ok := s.capturePollSnapshot()
	if !ok || cl == nil {
		return
	}
	res := executePollSnapshot(ctx, cl, snap)
	s.applyPollResult(res)
}

func (s *State) capturePollSnapshot() (pollSnapshot, bool) {
	if s.Project == nil || s.Project.ProjectID == "" || s.Project.ProjectRevision == "" {
		return pollSnapshot{}, false
	}
	revQuery, idQuery := s.resourceQueries()
	running := s.runningWorkflowSessions()
	workflows := make([]workflowPollTarget, 0, len(running))
	for _, chg := range running {
		workflows = append(workflows, workflowPollTarget{
			id:             chg.ID,
			worktreePath:   chg.WorktreePath,
			query:          client.FirstNonEmpty(s.changeResourceQuery(chg), idQuery),
			findingTargets: chg.FindingTargets,
		})
	}
	pollAnalysis := s.hasRunningAnalysis()
	var prevRun *AnalysisRun
	if s.Run != nil {
		cp := *s.Run
		if len(s.Run.Sections) > 0 {
			cp.Sections = append([]Section(nil), s.Run.Sections...)
		}
		prevRun = &cp
	}
	pollScan := s.Scan != nil && s.Scan.Status == "running"
	if len(workflows) == 0 && !pollAnalysis && !pollScan {
		return pollSnapshot{}, false
	}
	return pollSnapshot{
		idQuery:      idQuery,
		revQuery:     revQuery,
		workflows:    workflows,
		pollAnalysis: pollAnalysis,
		prevRun:      prevRun,
		emptyResults: len(s.Results) == 0,
		pollScan:     pollScan,
	}, true
}

func executePollSnapshot(ctx context.Context, cl *Client, snap pollSnapshot) pollResult {
	res := pollResult{
		changes:  make([]ChangeSession, len(snap.workflows)),
		changeOK: make([]bool, len(snap.workflows)),
	}
	var wg sync.WaitGroup
	for idx, wf := range snap.workflows {
		wg.Add(1)
		go func(i int, target workflowPollTarget) {
			defer wg.Done()
			var chg ChangeSession
			endpoint := "/api/projects/current/changes/" + url.PathEscape(target.id) + target.query
			reqID := nextTUIReqID(fmt.Sprintf("poll-change-%d", i))
			if err := cl.DoJSON(ctx, reqID, http.MethodGet, endpoint, nil, &chg); err == nil && chg.ID != "" {
				chg.WorktreePath = client.FirstNonEmpty(chg.WorktreePath, target.worktreePath)
				if len(chg.FindingTargets) == 0 && len(target.findingTargets) > 0 {
					chg.FindingTargets = target.findingTargets
				}
				res.changes[i] = chg
				res.changeOK[i] = true
			}
		}(idx, wf)
	}
	if snap.pollAnalysis {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pollAnalysisTask(ctx, cl, snap, &res)
		}()
	}
	if snap.pollScan {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var scan Scan
			reqID := nextTUIReqID("poll-scan")
			if err := cl.DoJSON(ctx, reqID, http.MethodGet, "/api/projects/current/scan"+snap.revQuery, nil, &scan); err == nil && scan.Status != "" {
				res.hasScan = true
				res.scan = scan
			}
		}()
	}
	wg.Wait()
	return res
}

func pollAnalysisTask(ctx context.Context, cl *Client, snap pollSnapshot, res *pollResult) {
	var run AnalysisRun
	reqID := nextTUIReqID("poll-analysis")
	if err := cl.DoJSON(ctx, reqID, http.MethodGet, "/api/projects/current/analysis/run"+snap.idQuery, nil, &run); err != nil || run.Identity.ID == "" {
		return
	}
	res.hasRun = true
	res.run = run
	if analysisSectionsChanged(snap.prevRun, &run, snap.emptyResults) {
		res.sectionResults = fetchAnalysisSectionResults(ctx, cl, run.Identity)
	}
	if !isAnalysisStatusRunning(run.Status) {
		var fResp FindingsResponse
		fReqID := nextTUIReqID("poll-findings")
		if err := cl.DoJSON(ctx, fReqID, http.MethodGet, "/api/projects/current/findings"+snap.revQuery, nil, &fResp); err == nil {
			res.hasFindings = true
			res.findingsResp = fResp
		}
	}
}

func analysisSectionsChanged(prev, next *AnalysisRun, emptyResults bool) bool {
	if next == nil {
		return false
	}
	if emptyResults || prev == nil || prev.Identity.ID != next.Identity.ID || prev.Status != next.Status || len(prev.Sections) != len(next.Sections) {
		return true
	}
	for i := range next.Sections {
		a, b := prev.Sections[i], next.Sections[i]
		if a.Status != b.Status || a.Coverage != b.Coverage || a.Reason != b.Reason {
			return true
		}
	}
	return false
}

func (s *State) applyPollResult(res pollResult) {
	for i, chg := range res.changes {
		if !res.changeOK[i] {
			continue
		}
		cp := chg
		s.upsertActiveChange(&cp)
		if s.Change != nil && s.Change.ID == cp.ID {
			s.Change = &cp
		}
		s.announceWorkflowTransition(&cp)
	}
	if res.hasRun {
		run := res.run
		s.Run = &run
		if len(res.sectionResults) > 0 {
			if s.Results == nil {
				s.Results = make(map[string]SectionResults)
			}
			for cat, sec := range res.sectionResults {
				s.Results[cat] = sec
			}
		}
		if !s.hasRunningAnalysis() {
			s.announceAnalysisTransition(&run)
			if res.hasFindings && s.AcceptProjectResponse(res.findingsResp.ProjectIdentity) {
				s.Findings = res.findingsResp.Findings
				for _, comp := range res.findingsResp.Completed {
					s.mergeFindingCompletion(comp)
				}
				s.loadIssueHistoryFromDisk()
			}
		}
	}
	if res.hasScan {
		scan := res.scan
		s.Scan = &scan
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

func isAnalysisStatusRunning(status string) bool {
	switch status {
	case "running", "queued", "pausing", "canceling":
		return true
	default:
		return false
	}
}

func (s *State) hasRunningAnalysis() bool {
	if s.Run == nil {
		return false
	}
	return isAnalysisStatusRunning(s.Run.Status)
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
	if len(targets) == 0 {
		return
	}
	oid := ""
	if commit != nil {
		oid = commit.OID
	}
	sessionID, hash, title := "", "", ""
	if s.Change != nil {
		sessionID = s.Change.ID
		hash = s.Change.Hash
		title = s.Change.Title
	}
	commitCode := domain.ResolveCommitCode(sessionID, hash, targets, commit)
	for _, ft := range targets {
		if ft.ProjectID == "" {
			ft.ProjectID = s.projectID()
		}
		s.mergeFindingCompletion(FindingCompletion{
			FindingTarget: ft,
			CommitOID:     oid,
			CommitCode:    commitCode,
			SessionID:     sessionID,
			Title:         title,
		})
	}
	s.saveIssueHistoryToDisk()
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
	s.saveIssueHistoryToDisk()
}

func (s *State) activeGitIdentity() (string, string) {
	if s.Change != nil && s.Change.Git != nil && s.Change.Git.Head != "" && s.Change.Git.Branch != "" {
		return s.Change.Git.Head, s.Change.Git.Branch
	}
	return s.mainGitIdentity()
}

func (s *State) mainGitIdentity() (string, string) {
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
