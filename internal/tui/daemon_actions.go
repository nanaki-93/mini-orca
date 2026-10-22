package tui

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
)

// FlushActions dispatches queued PendingActions to the daemon and clears the queue.
func (s *State) FlushActions(ctx context.Context, cl *Client) {
	if cl == nil || len(s.PendingActions) == 0 {
		s.PendingActions = nil
		return
	}
	actions := s.PendingActions
	s.PendingActions = nil
	s.flushActionBatch(ctx, cl, actions)
}

func (s *State) flushActionBatch(ctx context.Context, cl *Client, actions []ActionRequest) {
	var agentTasks []startAgentTask
	for _, act := range actions {
		reqID := nextTUIReqID("act-" + act.Kind)
		if act.Kind == "start-agent" {
			if task, ok := s.prepareStartAgentTask(reqID, act); ok {
				agentTasks = append(agentTasks, task)
			}
			continue
		}
		if err := s.dispatchAction(ctx, cl, reqID, act); err != nil {
			s.Error = err.Error()
		}
	}
	if len(agentTasks) == 1 {
		res := executeStartAgentTask(ctx, cl, agentTasks[0])
		_ = s.applyStartAgentResult(res)
		return
	}
	if len(agentTasks) > 1 {
		results := make([]startAgentResult, len(agentTasks))
		var wg sync.WaitGroup
		for i, task := range agentTasks {
			wg.Add(1)
			go func(idx int, t startAgentTask) {
				defer wg.Done()
				results[idx] = executeStartAgentTask(ctx, cl, t)
			}(i, task)
		}
		wg.Wait()
		for _, res := range results {
			_ = s.applyStartAgentResult(res)
		}
	}
}

func (s *State) dispatchAction(ctx context.Context, cl *Client, reqID string, act ActionRequest) error {
	switch act.Kind {
	case "reconnect":
		return s.SyncFromDaemon(ctx, cl, "")
	case "open-project":
		return s.SyncFromDaemon(ctx, cl, act.Target)
	case "reindex":
		return s.dispatchReindex(ctx, cl, reqID)
	case "scan":
		return s.dispatchScan(ctx, cl, reqID)
	case "open-file":
		return s.fetchFileInfo(ctx, cl, reqID, act.Target)
	case "run-analysis", "reanalyze", "repair-analysis", "pause-analysis", "resume-analysis":
		return s.dispatchAnalysisAction(ctx, cl, reqID, act)
	default:
		return s.dispatchMutationAction(ctx, cl, reqID, act)
	}
}

func (s *State) dispatchReindex(ctx context.Context, cl *Client, reqID string) error {
	var idx ProjectIndex
	body := map[string]any{"project_revision": s.projectRevision()}
	if err := cl.DoJSON(ctx, reqID, http.MethodPost, "/api/projects/current/reindex", body, &idx); err != nil {
		return err
	}
	if s.Project != nil && (s.Project.ProjectID == "" || idx.ProjectID == s.Project.ProjectID) && idx.ProjectRevision != "" {
		s.Project.ProjectRevision = idx.ProjectRevision
	}
	if s.AcceptProjectResponse(idx.ProjectIdentity) {
		s.Index = &idx
	}
	return nil
}

func (s *State) dispatchScan(ctx context.Context, cl *Client, reqID string) error {
	_ = s.ensureExecutionTrust(ctx, cl, reqID+"-trust")
	var scan Scan
	body := map[string]any{"project_revision": s.projectRevision()}
	if err := cl.DoJSON(ctx, reqID, http.MethodPost, "/api/projects/current/scan", body, &scan); err != nil {
		return err
	}
	if s.AcceptProjectResponse(scan.ProjectIdentity) {
		s.Scan = &scan
	}
	return nil
}

func (s *State) dispatchAnalysisAction(ctx context.Context, cl *Client, reqID string, act ActionRequest) error {
	switch act.Kind {
	case "pause-analysis":
		return s.dispatchPauseAnalysis(ctx, cl, reqID)
	case "resume-analysis":
		return s.dispatchResumeAnalysis(ctx, cl, reqID)
	default:
		return s.dispatchAnalysisRun(ctx, cl, reqID, act)
	}
}

func (s *State) analysisLimitsPayload() map[string]int {
	cfg := s.EffectiveAnalysisConfig()
	return map[string]int{
		"batch_files":            cfg.BatchFiles,
		"budget_seconds":         cfg.BudgetSeconds,
		"max_attempts_per_stage": cfg.MaxAttemptsPerStage,
	}
}

func (s *State) analysisModelsPayload() map[string]string {
	analyzeSel := s.AssignedScopeModelID("analyze")
	bugSel := s.AssignedScopeModelID("bug")
	if (analyzeSel == "" || analyzeSel == "analyze") && (bugSel == "" || bugSel == "bug") {
		return nil
	}
	codeModel := s.resolveDaemonModelSelection(client.FirstNonEmpty(bugSel, "bug"), "bug")
	analyzeModel := s.resolveDaemonModelSelection(client.FirstNonEmpty(analyzeSel, "analyze"), "analyze")
	return map[string]string{
		"code":        codeModel,
		"performance": analyzeModel,
		"security":    analyzeModel,
		"features":    analyzeModel,
	}
}

type analysisPreviewPayload struct {
	PreviewID string         `json:"preview_id"`
	Identity  map[string]any `json:"identity"`
	Limits    map[string]any `json:"limits"`
	Providers []struct {
		ID string `json:"id"`
	} `json:"providers"`
}

func (p analysisPreviewPayload) providerIDs() []string {
	ids := make([]string, 0, len(p.Providers))
	for _, prov := range p.Providers {
		if prov.ID != "" {
			ids = append(ids, prov.ID)
		}
	}
	return ids
}

func (s *State) isAnalysisPausedOrInterrupted() bool {
	return s.Run != nil && (s.Run.Status == "paused" || s.Run.Status == "interrupted")
}

func isRevisionConflictError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "project or file changed") ||
		strings.Contains(msg, "active project changed") ||
		strings.Contains(msg, "stale")
}

func (s *State) cancelPausedAnalysis(ctx context.Context, cl *Client, reqID string) {
	if s.Run == nil || s.Run.Identity.ID == "" {
		return
	}
	ctrlReq := map[string]any{"identity": s.Run.Identity, "action": "cancel"}
	var run AnalysisRun
	if err := cl.DoJSON(ctx, reqID, http.MethodPost, "/api/projects/current/analysis/run/control", ctrlReq, &run); err == nil && run.Identity.ID != "" {
		s.Run = &run
	}
}

func (s *State) dispatchAnalysisRun(ctx context.Context, cl *Client, reqID string, act ActionRequest) error {
	if s.Project == nil || s.Project.ProjectID == "" {
		return nil
	}
	cfg := s.EffectiveAnalysisConfig()
	if act.Kind == "run-analysis" && s.isAnalysisPausedOrInterrupted() && !cfg.ClearResults {
		return s.dispatchResumeAnalysis(ctx, cl, reqID)
	}
	if act.Kind == "repair-analysis" && s.isAnalysisPausedOrInterrupted() {
		s.cancelPausedAnalysis(ctx, cl, reqID+"-cancel")
	}
	refresh, clearResults, includeFeatures, retryStale, staleOnly := s.resolveAnalysisDispatchFlags(act, cfg)
	preview, err := s.requestAnalysisPreview(ctx, cl, reqID, refresh, clearResults, includeFeatures, retryStale, staleOnly, act.Target)
	if err != nil {
		return err
	}
	startReq := s.buildAnalysisStartRequest(preview, refresh, clearResults, includeFeatures, retryStale, staleOnly, act.Target)
	var run AnalysisRun
	if err := cl.DoJSON(ctx, reqID+"-start", http.MethodPost, "/api/projects/current/analysis/run", startReq, &run); err != nil {
		return err
	}
	s.Run = &run
	return nil
}

func (s *State) resolveAnalysisDispatchFlags(act ActionRequest, cfg AnalysisRunConfig) (refresh, clearResults, includeFeatures, retryStale, staleOnly bool) {
	refresh = act.Kind == "reanalyze" || cfg.Refresh
	clearResults = (act.Kind == "reanalyze" && s.isAnalysisPausedOrInterrupted()) || cfg.ClearResults
	if clearResults {
		refresh = true
	}
	retryStale = act.Kind == "repair-analysis" && act.Target == ""
	staleOnly = act.Kind == "repair-analysis" && act.Target != ""
	includeFeatures = cfg.IncludeFeatures && !staleOnly
	return
}

func (s *State) buildAnalysisStartRequest(preview analysisPreviewPayload, refresh, clearResults, includeFeatures, retryStale, staleOnly bool, stalePath string) map[string]any {
	limitsPayload := any(preview.Limits)
	if len(preview.Limits) == 0 {
		limitsPayload = s.analysisLimitsPayload()
	}
	startReq := map[string]any{
		"identity":           preview.Identity,
		"preview_id":         preview.PreviewID,
		"limits":             limitsPayload,
		"refresh":            refresh,
		"clear_results":      clearResults,
		"include_features":   includeFeatures,
		"retry_stale_failed": retryStale,
		"stale_only":         staleOnly,
		"stale_path":         stalePath,
		"confirmations": map[string]any{
			"provider_ids":    preview.providerIDs(),
			"security_review": true,
		},
	}
	if models := s.analysisModelsPayload(); len(models) > 0 {
		startReq["models"] = models
	}
	return startReq
}

func (s *State) requestAnalysisPreview(ctx context.Context, cl *Client, reqID string, refresh, clearResults, includeFeatures, retryStale, staleOnly bool, stalePath string) (analysisPreviewPayload, error) {
	buildReq := func() map[string]any {
		req := map[string]any{
			"project_id":         s.Project.ProjectID,
			"project_revision":   s.Project.ProjectRevision,
			"scope":              "project",
			"refresh":            refresh,
			"clear_results":      clearResults,
			"include_features":   includeFeatures,
			"retry_stale_failed": retryStale,
			"stale_only":         staleOnly,
			"stale_path":         stalePath,
			"limits":             s.analysisLimitsPayload(),
		}
		if models := s.analysisModelsPayload(); len(models) > 0 {
			req["models"] = models
		}
		return req
	}
	var preview analysisPreviewPayload
	err := cl.DoJSON(ctx, reqID+"-prev", http.MethodPost, "/api/projects/current/analysis/preview", buildReq(), &preview)
	if err != nil && isRevisionConflictError(err) {
		s.syncProjectRevision(ctx, cl, reqID+"-sync")
		err = cl.DoJSON(ctx, reqID+"-prev2", http.MethodPost, "/api/projects/current/analysis/preview", buildReq(), &preview)
	}
	return preview, err
}

func (s *State) dispatchPauseAnalysis(ctx context.Context, cl *Client, reqID string) error {
	if s.Run == nil || s.Run.Identity.ID == "" {
		return nil
	}
	ctrlReq := map[string]any{
		"identity": s.Run.Identity,
		"action":   "pause",
	}
	var run AnalysisRun
	if err := cl.DoJSON(ctx, reqID, http.MethodPost, "/api/projects/current/analysis/run/control", ctrlReq, &run); err != nil {
		return err
	}
	s.Run = &run
	return nil
}

func (s *State) dispatchResumeAnalysis(ctx context.Context, cl *Client, reqID string) error {
	if s.Project == nil || s.Project.ProjectID == "" {
		return nil
	}
	if s.Run == nil || s.Run.Identity.ID == "" {
		_, idQuery := s.resourceQueries()
		var latest AnalysisRun
		if err := cl.DoJSON(ctx, reqID+"-fetch", http.MethodGet, "/api/projects/current/analysis/run"+idQuery, nil, &latest); err == nil && latest.Identity.ID != "" {
			s.Run = &latest
		}
	}
	if s.Run == nil || s.Run.Identity.ID == "" || !s.isAnalysisPausedOrInterrupted() {
		return s.dispatchAnalysisRun(ctx, cl, reqID, ActionRequest{Kind: "run-analysis"})
	}
	limits := s.Run.Plan.Limits
	if len(limits) == 0 {
		limits = s.analysisLimitsPayload()
	}
	previewReq := map[string]any{
		"project_id":         s.Project.ProjectID,
		"project_revision":   s.Project.ProjectRevision,
		"scope":              "project",
		"refresh":            s.Run.Plan.Refresh,
		"retry_stale_failed": s.Run.Plan.RetryStaleFailed,
		"stale_only":         s.Run.Plan.StaleOnly,
		"stale_path":         s.Run.Plan.StalePath,
		"resume_run":         s.Run.Identity,
		"limits":             limits,
	}
	var preview analysisPreviewPayload
	if err := cl.DoJSON(ctx, reqID+"-prev", http.MethodPost, "/api/projects/current/analysis/preview", previewReq, &preview); err != nil {
		if isRevisionConflictError(err) {
			s.syncProjectRevision(ctx, cl, reqID+"-sync")
			s.cancelPausedAnalysis(ctx, cl, reqID+"-cancel")
			return s.dispatchAnalysisRun(ctx, cl, reqID, ActionRequest{Kind: "run-analysis"})
		}
		return err
	}
	ctrlReq := map[string]any{
		"identity":   s.Run.Identity,
		"action":     "resume",
		"preview_id": preview.PreviewID,
		"confirmations": map[string]any{
			"provider_ids":    preview.providerIDs(),
			"security_review": true,
		},
	}
	var run AnalysisRun
	if err := cl.DoJSON(ctx, reqID+"-resume", http.MethodPost, "/api/projects/current/analysis/run/control", ctrlReq, &run); err != nil {
		return err
	}
	s.Run = &run
	return nil
}

func (s *State) dispatchMutationAction(ctx context.Context, cl *Client, reqID string, act ActionRequest) error {
	switch act.Kind {
	case "start-agent":
		return s.dispatchStartAgent(ctx, cl, reqID, act)
	case "apply-commit":
		return s.dispatchApplyCommit(ctx, cl, reqID, act)
	case "dismiss-finding":
		return s.dispatchDismissFinding(ctx, cl, reqID, act)
	case "dismiss-feature":
		return s.dispatchDismissFeature(ctx, cl, reqID, act)
	case "set-trust":
		return s.dispatchSetTrust(ctx, cl, reqID)
	case "undo":
		return s.dispatchUndo(ctx, cl, reqID, act)
	default:
		return s.dispatchSecondaryMutation(ctx, cl, reqID, act)
	}
}

func (s *State) dispatchSecondaryMutation(ctx context.Context, cl *Client, reqID string, act ActionRequest) error {
	switch act.Kind {
	case "update-proposal":
		return s.dispatchUpdateProposal(ctx, cl, reqID, act)
	case "check-change":
		return s.dispatchCheckChange(ctx, cl, reqID)
	case "repair-change":
		return s.dispatchRepairChange(ctx, cl, reqID, act)
	case "merge-worktree":
		return s.dispatchMergeWorktree(ctx, cl, reqID, act.Target)
	case "load-change":
		return s.dispatchLoadChange(ctx, cl, reqID, act.Target)
	case "save-selection":
		return s.dispatchSaveSelection(ctx, cl, reqID)
	case "propose-instructions":
		return s.dispatchProposeInstructions(ctx, cl, reqID, act.Target, act.Message)
	case "bootstrap":
		return s.dispatchBootstrap(ctx, cl, reqID, act.Target)
	case "cancel-workflow":
		return s.dispatchCancelWorkflow(ctx, cl, reqID, act.Target)
	case "reject-change":
		return s.dispatchRejectChange(ctx, cl, reqID, act)
	case "refresh-models":
		_ = s.RefreshModels(ctx, cl)
		return nil
	default:
		return nil
	}
}

func (s *State) changeProjectIdentity(chg *ChangeSession) (string, string) {
	if chg != nil && chg.ProjectID != "" && chg.ProjectRevision != "" {
		return chg.ProjectID, chg.ProjectRevision
	}
	if s.Project != nil {
		return s.Project.ProjectID, s.Project.ProjectRevision
	}
	return "", ""
}

func changeWorktreeScopeQuery(chg *ChangeSession) string {
	if chg != nil && chg.WorktreePath != "" {
		return "?worktree_path=" + url.QueryEscape(chg.WorktreePath)
	}
	return ""
}

func (s *State) changeResourceQuery(chg *ChangeSession) string {
	projID, projRev := s.changeProjectIdentity(chg)
	if projID == "" || projRev == "" {
		return ""
	}
	q := fmt.Sprintf("?project_id=%s&project_revision=%s", url.QueryEscape(projID), url.QueryEscape(projRev))
	if chg != nil && chg.WorktreePath != "" {
		q += "&worktree_path=" + url.QueryEscape(chg.WorktreePath)
	}
	return q
}

func (s *State) dispatchCancelWorkflow(ctx context.Context, cl *Client, reqID, sessionID string) error {
	if s.Change == nil || s.Change.Workflow == nil || s.Project == nil {
		return nil
	}
	wtPath := s.Change.WorktreePath
	projID, projRev := s.changeProjectIdentity(s.Change)
	id := client.FirstNonEmpty(sessionID, s.Change.ID)
	req := map[string]any{
		"project_id":       projID,
		"project_revision": projRev,
		"workflow_id":      s.Change.Workflow.ID,
	}
	var updated ChangeSession
	target := "/api/projects/current/changes/" + url.PathEscape(id) + "/workflow/cancel" + changeWorktreeScopeQuery(s.Change)
	if err := cl.DoJSON(ctx, reqID, http.MethodPost, target, req, &updated); err != nil {
		return err
	}
	if updated.ID != "" {
		updated.WorktreePath = client.FirstNonEmpty(updated.WorktreePath, wtPath)
		s.Change = &updated
		s.upsertActiveChange(&updated)
	}
	return nil
}

func (s *State) workflowModelsPayload() (map[string]string, []string) {
	createSel := s.resolveDaemonModelSelection(s.AssignedScopeModelID("bug"), "bug")
	testSel := s.resolveDaemonModelSelection(s.AssignedScopeModelID("function"), "function")
	reviewSel := s.resolveDaemonModelSelection(s.AssignedScopeModelID("analyze"), "analyze")
	models := map[string]string{
		"create": createSel,
		"test":   testSel,
		"review": reviewSel,
	}
	confirmed := []string{"analyze", "bug", "function"}
	for _, sel := range []string{createSel, testSel, reviewSel} {
		if !domain.ContainsString(confirmed, sel) {
			confirmed = append(confirmed, sel)
		}
	}
	return models, confirmed
}

func (s *State) resolveDaemonModelSelection(candidate, fallbackScope string) string {
	switch candidate {
	case "analyze", "bug", "function":
		return candidate
	}
	if s.AvailableModels != nil {
		for _, ch := range s.AvailableModels.Models {
			if ch.ID == candidate {
				return candidate
			}
		}
	}
	return fallbackScope
}

type startAgentTask struct {
	reqID             string
	act               ActionRequest
	projectID         string
	projectRevision   string
	root              string
	head              string
	branch            string
	workspaceTrust    bool
	paths             []string
	modelsMap         map[string]string
	confirmedProfiles []string
	targetBranch      string
	reservedPath      string
	existingWorktrees []GitWorktree
}

type startAgentResult struct {
	task      startAgentTask
	wtPath    string
	wtHead    string
	wtBranch  string
	session   ChangeSession
	wfSession ChangeSession
	syncProj  *Project
	syncIdx   *ProjectIndex
	err       error
}

func (s *State) dispatchStartAgent(ctx context.Context, cl *Client, reqID string, act ActionRequest) error {
	task, ok := s.prepareStartAgentTask(reqID, act)
	if !ok {
		return nil
	}
	res := executeStartAgentTask(ctx, cl, task)
	return s.applyStartAgentResult(res)
}

func (s *State) prepareStartAgentTask(reqID string, act ActionRequest) (startAgentTask, bool) {
	if act.Seed == nil || s.Project == nil || s.Project.ProjectID == "" {
		return startAgentTask{}, false
	}
	paths := domain.EnsureChangeTestPaths(act.Seed.Paths, s.Index)
	if len(paths) == 0 {
		return startAgentTask{}, false
	}
	modelsMap, confirmedProfiles := s.workflowModelsPayload()
	root := s.Project.Path
	if root == "" && s.Repository != nil {
		root = s.Repository.Root
	}
	head, branch := s.mainGitIdentity()
	existing := s.worktreeCollisionEntries()
	var targetBranch, reservedPath string
	if act.Seed.WorktreePath == "" && root != "" && filepath.IsAbs(root) && head != "" && branch != "" {
		var relPath string
		targetBranch, relPath = domain.BuildWorktreeSpec(act.Seed, existing)
		reservedPath = filepath.Join(root, relPath)
		s.registerCreatedWorktree(reservedPath, head, targetBranch)
	}
	return startAgentTask{
		reqID:             reqID,
		act:               act,
		projectID:         s.Project.ProjectID,
		projectRevision:   s.Project.ProjectRevision,
		root:              root,
		head:              head,
		branch:            branch,
		workspaceTrust:    s.WorkspaceTrust,
		paths:             paths,
		modelsMap:         modelsMap,
		confirmedProfiles: confirmedProfiles,
		targetBranch:      targetBranch,
		reservedPath:      reservedPath,
		existingWorktrees: existing,
	}, true
}

func executeStartAgentTask(ctx context.Context, cl *Client, task startAgentTask) startAgentResult {
	res := startAgentResult{task: task}
	if task.workspaceTrust && task.projectRevision != "" {
		body := map[string]any{"project_revision": task.projectRevision, "confirm": true}
		var trust ExecutionTrust
		_ = cl.DoJSON(ctx, task.reqID+"-trust", http.MethodPost, "/api/projects/current/execution-trust", body, &trust)
	}
	wtPath, wtProjID, wtProjRev, wtHead, wtBranch := prepareAgentWorktreeTask(ctx, cl, &task)
	res.wtPath, res.wtHead, res.wtBranch = wtPath, wtHead, wtBranch
	session, syncProj, syncIdx, err := openChangeSessionTask(ctx, cl, &task, wtPath, wtProjID, wtProjRev)
	res.syncProj, res.syncIdx = syncProj, syncIdx
	if err != nil {
		if wtPath != "" && (task.act.Seed == nil || task.act.Seed.WorktreePath == "") {
			discardCreatedWorktreeOnError(ctx, cl, &task, wtPath, wtHead, wtBranch)
			res.wtPath, res.wtBranch = "", ""
		}
		res.err = err
		return res
	}
	res.session = session
	wfSession, err := startAgentWorkflowTask(ctx, cl, &task, &session)
	if err != nil {
		res.err = err
		return res
	}
	res.wfSession = wfSession
	return res
}

func discardCreatedWorktreeOnError(ctx context.Context, cl *Client, task *startAgentTask, wtPath, wtHead, wtBranch string) {
	req := map[string]any{
		"project_id":        task.projectID,
		"project_revision":  task.projectRevision,
		"head":              task.head,
		"branch":            task.branch,
		"path":              wtPath,
		"source_head":       wtHead,
		"source_branch":     wtBranch,
		"confirm":           true,
		"confirm_execution": true,
	}
	_ = cl.DoJSON(ctx, task.reqID+"-wt-discard", http.MethodPost, "/api/projects/current/git/discard-worktree", req, nil)
}

func prepareAgentWorktreeTask(ctx context.Context, cl *Client, task *startAgentTask) (string, string, string, string, string) {
	if task.act.Seed != nil && task.act.Seed.WorktreePath != "" {
		wtPath := task.act.Seed.WorktreePath
		projID, projRev := resolveWorktreeIdentityTask(ctx, cl, task.reqID+"-wt", wtPath, task.projectID, task.projectRevision, task.workspaceTrust)
		return wtPath, projID, projRev, task.head, ""
	}
	if task.reservedPath == "" || task.root == "" || !filepath.IsAbs(task.root) || task.head == "" || task.branch == "" {
		return "", task.projectID, task.projectRevision, task.head, ""
	}
	if created, ok := tryCreateAgentWorktree(ctx, cl, task.reqID+"-wt", task.projectID, task.projectRevision, task.head, task.branch, task.targetBranch, task.reservedPath); ok {
		projID, projRev := resolveWorktreeIdentityTask(ctx, cl, task.reqID+"-wt", created, task.projectID, task.projectRevision, task.workspaceTrust)
		return created, projID, projRev, task.head, task.targetBranch
	}
	return retryCreateAgentWorktree(ctx, cl, task)
}

func tryCreateAgentWorktree(ctx context.Context, cl *Client, reqID, projID, projRev, head, branch, targetBranch, absPath string) (string, bool) {
	wtReq := map[string]any{
		"project_id":        projID,
		"project_revision":  projRev,
		"head":              head,
		"branch":            branch,
		"target_branch":     targetBranch,
		"path":              absPath,
		"confirm":           true,
		"confirm_execution": true,
	}
	var wtRes struct {
		State string `json:"state"`
		Path  string `json:"path"`
		Error string `json:"error"`
	}
	if err := cl.DoJSON(ctx, reqID, http.MethodPost, "/api/projects/current/git/worktrees", wtReq, &wtRes); err != nil || wtRes.State != "created" || wtRes.Error != "" {
		return "", false
	}
	return client.FirstNonEmpty(wtRes.Path, absPath), true
}

func retryCreateAgentWorktree(ctx context.Context, cl *Client, task *startAgentTask) (string, string, string, string, string) {
	existing := append([]GitWorktree(nil), task.existingWorktrees...)
	existing = append(existing, GitWorktree{GitIdentity: GitIdentity{Branch: task.targetBranch}, Path: task.reservedPath})
	projID, projRev, head, branch := refreshWorktreeGitState(ctx, cl, task, &existing)
	if task.workspaceTrust && projRev != "" {
		body := map[string]any{"project_revision": projRev, "confirm": true}
		var trust ExecutionTrust
		_ = cl.DoJSON(ctx, task.reqID+"-wt-trust-retry", http.MethodPost, "/api/projects/current/execution-trust", body, &trust)
	}
	for attempt := 1; attempt <= 2; attempt++ {
		targetBranch, relPath := domain.BuildWorktreeSpec(task.act.Seed, existing)
		candidatePath := filepath.Join(task.root, relPath)
		reqID := fmt.Sprintf("%s-wt-retry-%d", task.reqID, attempt)
		if created, ok := tryCreateAgentWorktree(ctx, cl, reqID, projID, projRev, head, branch, targetBranch, candidatePath); ok {
			wtProjID, wtProjRev := resolveWorktreeIdentityTask(ctx, cl, reqID, created, projID, projRev, task.workspaceTrust)
			return created, wtProjID, wtProjRev, head, targetBranch
		}
		existing = append(existing, GitWorktree{GitIdentity: GitIdentity{Branch: targetBranch}, Path: candidatePath})
	}
	return "", projID, projRev, head, ""
}

func refreshWorktreeGitState(ctx context.Context, cl *Client, task *startAgentTask, existing *[]GitWorktree) (string, string, string, string) {
	projID, projRev, head, branch := task.projectID, task.projectRevision, task.head, task.branch
	var proj Project
	if err := cl.DoJSON(ctx, task.reqID+"-wt-proj", http.MethodGet, "/api/projects/current", nil, &proj); err == nil && proj.ProjectID != "" {
		projID = proj.ProjectID
		if proj.ProjectRevision != "" {
			projRev = proj.ProjectRevision
		}
	}
	idQuery := fmt.Sprintf("?project_id=%s&project_revision=%s", url.QueryEscape(projID), url.QueryEscape(projRev))
	var gitWS GitWorkspace
	if err := cl.DoJSON(ctx, task.reqID+"-wt-git", http.MethodGet, "/api/projects/current/git/workspace"+idQuery, nil, &gitWS); err == nil {
		if gitWS.Head != "" && gitWS.Branch != "" {
			head, branch = gitWS.Head, gitWS.Branch
		}
		*existing = append(*existing, gitWS.Worktrees...)
		for _, b := range gitWS.Branches {
			if strings.TrimSpace(b.Name) != "" {
				*existing = append(*existing, GitWorktree{GitIdentity: GitIdentity{Branch: b.Name}})
			}
		}
	}
	return projID, projRev, head, branch
}

func resolveWorktreeIdentityTask(ctx context.Context, cl *Client, reqID, wtPath, fallbackID, fallbackRev string, trustEnabled bool) (string, string) {
	scope := "?worktree_path=" + url.QueryEscape(wtPath)
	var wtProj Project
	if err := cl.DoJSON(ctx, reqID+"-proj", http.MethodGet, "/api/projects/current"+scope, nil, &wtProj); err == nil && wtProj.ProjectID != "" && wtProj.ProjectRevision != "" {
		if trustEnabled {
			trustBody := map[string]any{"project_revision": wtProj.ProjectRevision, "confirm": true}
			var trust ExecutionTrust
			_ = cl.DoJSON(ctx, reqID+"-trust", http.MethodPost, "/api/projects/current/execution-trust"+scope, trustBody, &trust)
		}
		return wtProj.ProjectID, wtProj.ProjectRevision
	}
	return fallbackID, fallbackRev
}

func (s *State) worktreeCollisionEntries() []GitWorktree {
	var out []GitWorktree
	if s.GitWorkspace != nil {
		out = append(out, s.GitWorkspace.Worktrees...)
		for _, b := range s.GitWorkspace.Branches {
			if strings.TrimSpace(b.Name) != "" {
				out = append(out, GitWorktree{GitIdentity: GitIdentity{Branch: b.Name}})
			}
		}
	}
	if s.Change != nil && s.Change.WorktreePath != "" {
		out = append(out, GitWorktree{Path: s.Change.WorktreePath})
	}
	for _, chg := range s.ActiveChanges {
		if chg != nil && chg.WorktreePath != "" {
			out = append(out, GitWorktree{Path: chg.WorktreePath})
		}
	}
	return out
}

func (s *State) registerCreatedWorktree(wtPath, head, branch string) {
	if wtPath == "" {
		return
	}
	if s.GitWorkspace == nil {
		s.GitWorkspace = &GitWorkspace{}
	}
	if branch != "" {
		hasBranch := false
		for _, b := range s.GitWorkspace.Branches {
			if b.Name == branch {
				hasBranch = true
				break
			}
		}
		if !hasBranch {
			s.GitWorkspace.Branches = append(s.GitWorkspace.Branches, GitBranch{Name: branch})
		}
	}
	for i := range s.GitWorkspace.Worktrees {
		if s.GitWorkspace.Worktrees[i].Path == wtPath {
			if head != "" {
				s.GitWorkspace.Worktrees[i].Head = head
			}
			if branch != "" {
				s.GitWorkspace.Worktrees[i].Branch = branch
			}
			s.GitWorkspace.Worktrees[i].Mergeable = true
			return
		}
	}
	s.GitWorkspace.Worktrees = append(s.GitWorkspace.Worktrees, GitWorktree{
		GitIdentity: GitIdentity{Head: head, Branch: branch},
		Path:        wtPath,
		Mergeable:   true,
	})
}

func syncProjectRevisionTask(ctx context.Context, cl *Client, reqID, fallbackRev string, trustEnabled bool) (*Project, *ProjectIndex, string) {
	rev := fallbackRev
	var proj Project
	var projPtr *Project
	if err := cl.DoJSON(ctx, reqID+"-proj", http.MethodGet, "/api/projects/current", nil, &proj); err == nil && proj.ProjectID != "" {
		projPtr = &proj
		if proj.ProjectRevision != "" {
			rev = proj.ProjectRevision
		}
	}
	var idx ProjectIndex
	var idxPtr *ProjectIndex
	if err := cl.DoJSON(ctx, reqID+"-idx", http.MethodGet, "/api/projects/current/index", nil, &idx); err == nil && idx.ProjectID != "" {
		idxPtr = &idx
		if idx.ProjectRevision != "" {
			rev = idx.ProjectRevision
		}
	}
	if trustEnabled && rev != "" {
		body := map[string]any{"project_revision": rev, "confirm": true}
		var trust ExecutionTrust
		_ = cl.DoJSON(ctx, reqID+"-trust2", http.MethodPost, "/api/projects/current/execution-trust", body, &trust)
	}
	return projPtr, idxPtr, rev
}

func openChangeSessionTask(ctx context.Context, cl *Client, task *startAgentTask, wtPath, projID, projRev string) (ChangeSession, *Project, *ProjectIndex, error) {
	scope := ""
	if wtPath != "" {
		scope = "?worktree_path=" + url.QueryEscape(wtPath)
	}
	seed := task.act.Seed
	buildReq := func(pid, prev string) map[string]any {
		effProjID := client.FirstNonEmpty(pid, task.projectID)
		targetProjID := client.FirstNonEmpty(task.projectID, effProjID)
		return map[string]any{
			"project_id":          effProjID,
			"project_revision":    client.FirstNonEmpty(prev, task.projectRevision),
			"kind":                client.FirstNonEmpty(seed.Kind, "fix"),
			"title":               client.FirstNonEmpty(seed.Title, "Agent Mission"),
			"paths":               task.paths,
			"acceptance_criteria": seed.AcceptanceCriteria,
			"finding_targets":     domain.NormalizeFindingTargets(seed.FindingTargets, targetProjID, task.paths),
		}
	}
	var session ChangeSession
	var syncProj *Project
	var syncIdx *ProjectIndex
	err := cl.DoJSON(ctx, task.reqID+"-open", http.MethodPost, "/api/projects/current/changes"+scope, buildReq(projID, projRev), &session)
	if err != nil && isRevisionConflictError(err) {
		if wtPath != "" {
			projID, projRev = resolveWorktreeIdentityTask(ctx, cl, task.reqID+"-sync", wtPath, task.projectID, task.projectRevision, task.workspaceTrust)
		} else {
			syncProj, syncIdx, projRev = syncProjectRevisionTask(ctx, cl, task.reqID+"-sync", task.projectRevision, task.workspaceTrust)
		}
		err = cl.DoJSON(ctx, task.reqID+"-open2", http.MethodPost, "/api/projects/current/changes"+scope, buildReq(projID, projRev), &session)
	}
	if session.ProjectID == "" {
		session.ProjectID = client.FirstNonEmpty(projID, task.projectID)
	}
	if session.ProjectRevision == "" {
		session.ProjectRevision = client.FirstNonEmpty(projRev, task.projectRevision)
	}
	session.WorktreePath = wtPath
	if len(session.FindingTargets) == 0 && len(seed.FindingTargets) > 0 {
		session.FindingTargets = seed.FindingTargets
	}
	return session, syncProj, syncIdx, err
}

func startAgentWorkflowTask(ctx context.Context, cl *Client, task *startAgentTask, session *ChangeSession) (ChangeSession, error) {
	sessProjID := client.FirstNonEmpty(session.ProjectID, task.projectID)
	sessProjRev := client.FirstNonEmpty(session.ProjectRevision, task.projectRevision)
	wfReq := map[string]any{
		"project_id":         sessProjID,
		"project_revision":   sessProjRev,
		"revision":           session.Revision,
		"hash":               session.Hash,
		"message":            client.FirstNonEmpty(task.act.Seed.Message, task.act.Message, session.Title),
		"models":             task.modelsMap,
		"confirmed_profiles": task.confirmedProfiles,
		"confirm_security":   true,
		"auto_repair":        true,
	}
	target := "/api/projects/current/changes/" + url.PathEscape(session.ID) + "/workflow" + changeWorktreeScopeQuery(session)
	var wfSession ChangeSession
	if err := cl.DoJSON(ctx, task.reqID+"-wf", http.MethodPost, target, wfReq, &wfSession); err != nil {
		return ChangeSession{}, err
	}
	wfSession.WorktreePath = session.WorktreePath
	if len(wfSession.FindingTargets) == 0 && len(session.FindingTargets) > 0 {
		wfSession.FindingTargets = session.FindingTargets
	}
	return wfSession, nil
}

func (s *State) applyStartAgentResult(res startAgentResult) error {
	if res.task.reservedPath != "" && res.wtPath != res.task.reservedPath {
		s.removeWorktreeByPath(res.task.reservedPath)
	}
	if res.wtPath != "" && res.wtBranch != "" {
		s.registerCreatedWorktree(res.wtPath, res.wtHead, res.wtBranch)
	}
	if res.syncProj != nil && res.syncProj.ProjectID != "" {
		s.Project = res.syncProj
	}
	if res.syncIdx != nil && s.AcceptProjectResponse(res.syncIdx.ProjectIdentity) {
		s.Index = res.syncIdx
		if s.Project != nil && res.syncIdx.ProjectRevision != "" {
			s.Project.ProjectRevision = res.syncIdx.ProjectRevision
		}
	}
	if res.err != nil {
		if res.session.ID == "" {
			s.dropPreviewChangeOnError(res.task.act.PreviewID)
		} else {
			res.session.State = "failed"
			s.replacePreviewWithSession(res.task.act.PreviewID, &res.session)
		}
		s.Error = res.err.Error()
		return res.err
	}
	s.replacePreviewWithSession(res.task.act.PreviewID, &res.wfSession)
	if s.Page == PageAgents {
		s.syncSelectedIdxToCurrentChange()
	}
	return nil
}

func (s *State) dispatchApplyCommit(ctx context.Context, cl *Client, reqID string, act ActionRequest) error {
	if s.Change == nil || s.Project == nil {
		return nil
	}
	if s.Change.WorktreePath != "" && s.Change.Commit != nil && s.Change.Commit.State == "committed" {
		return s.mergeAlreadyCommittedChange(ctx, cl, reqID, s.Change)
	}
	sessionID := url.PathEscape(s.Change.ID)
	s.ensureChangeReviewed(ctx, cl, reqID, sessionID)
	appliedChange := s.Change
	wtPath := appliedChange.WorktreePath
	commitCode := domain.GenerateCommitCode(appliedChange.ID, appliedChange.Hash, appliedChange.FindingTargets)
	commitMsg := domain.FormatCommitMessageWithCode(client.FirstNonEmpty(act.Message, appliedChange.Title), commitCode)
	applyReq, hasGitTarget := s.buildApplyCommitRequest(appliedChange, commitMsg)
	var receipt ChangeMutation
	target := "/api/projects/current/changes/" + sessionID + "/apply" + changeWorktreeScopeQuery(appliedChange)
	if err := cl.DoJSON(ctx, reqID, http.MethodPost, target, applyReq, &receipt); err != nil {
		return err
	}
	if wtPath == "" && receipt.ProjectRevision != "" {
		s.Project.ProjectRevision = receipt.ProjectRevision
	}
	if err := s.recordApplyReceipt(appliedChange, &receipt, commitMsg, wtPath != "" || hasGitTarget); err != nil {
		return err
	}
	if err := s.autoMergeAppliedWorktree(ctx, cl, reqID, wtPath, receipt.Commit, commitCode); err != nil {
		appliedChange.State = "reviewed"
		s.upsertActiveChange(appliedChange)
		return err
	}
	s.finalizeAppliedChange(appliedChange, receipt.Commit, commitCode)
	return nil
}

func (s *State) mergeAlreadyCommittedChange(ctx context.Context, cl *Client, reqID string, appliedChange *ChangeSession) error {
	wtPath := appliedChange.WorktreePath
	commitCode := domain.ResolveCommitCode(appliedChange.ID, appliedChange.Hash, appliedChange.FindingTargets, appliedChange.Commit)
	if err := s.autoMergeAppliedWorktree(ctx, cl, reqID, wtPath, appliedChange.Commit, commitCode); err != nil {
		appliedChange.State = "reviewed"
		s.upsertActiveChange(appliedChange)
		return err
	}
	s.finalizeAppliedChange(appliedChange, appliedChange.Commit, commitCode)
	return nil
}

func (s *State) buildApplyCommitRequest(appliedChange *ChangeSession, commitMsg string) (map[string]any, bool) {
	projID, projRev := s.changeProjectIdentity(appliedChange)
	applyReq := map[string]any{
		"project_id":       projID,
		"project_revision": projRev,
		"revision":         appliedChange.Revision,
		"hash":             appliedChange.Hash,
		"confirm":          true,
		"commit_message":   commitMsg,
	}
	head, branch := s.activeGitIdentity()
	hasGitTarget := head != "" && branch != ""
	if hasGitTarget {
		applyReq["head"] = head
		applyReq["branch"] = branch
	}
	return applyReq, hasGitTarget
}

func (s *State) finalizeAppliedChange(appliedChange *ChangeSession, commit *ChangeCommit, commitCode string) {
	if appliedChange == nil || appliedChange.State == "applied" {
		return
	}
	appliedChange.State = "applied"
	s.recordAppliedFindingCompletions(appliedChange.FindingTargets, commit)
	s.syncAppliedInstructions(appliedChange)
	_, mainBranch := s.mainGitIdentity()
	s.archiveAndCompleteChange(appliedChange)
	s.CmdOutput = fmt.Sprintf("Committed & merged proposal %q [%s] into %s", appliedChange.Title, commitCode, client.FirstNonEmpty(mainBranch, "main"))
}

func (s *State) recordApplyReceipt(appliedChange *ChangeSession, receipt *ChangeMutation, commitMsg string, requireCommit bool) error {
	if receipt.SessionID == "" {
		receipt.SessionID = appliedChange.ID
	}
	if receipt.Hash == "" {
		receipt.Hash = appliedChange.Hash
	}
	if receipt.Commit != nil && receipt.Commit.Message == "" {
		receipt.Commit.Message = commitMsg
	}
	s.ChangeReceipt = receipt
	appliedChange.Commit = receipt.Commit
	if receipt.State != "applied" {
		appliedChange.State = client.FirstNonEmpty(receipt.State, "failed")
		s.upsertActiveChange(appliedChange)
		return applyReceiptError(receipt)
	}
	if receipt.Commit != nil && (receipt.Commit.State != "committed" || receipt.Commit.Error != "") {
		appliedChange.State = "failed"
		s.upsertActiveChange(appliedChange)
		return applyReceiptError(receipt)
	}
	if requireCommit && receipt.Commit == nil {
		appliedChange.State = "failed"
		s.upsertActiveChange(appliedChange)
		return applyReceiptError(receipt)
	}
	return nil
}

func applyReceiptError(receipt *ChangeMutation) error {
	if receipt == nil {
		return fmt.Errorf("apply mutation failed")
	}
	if receipt.Commit != nil && strings.TrimSpace(receipt.Commit.Error) != "" {
		return fmt.Errorf("commit failed: %s", receipt.Commit.Error)
	}
	if len(receipt.Warnings) > 0 {
		return fmt.Errorf("%s", strings.Join(receipt.Warnings, "; "))
	}
	if receipt.State == "recovery_required" {
		return fmt.Errorf("apply failed: recovery is required")
	}
	if receipt.Commit != nil && receipt.Commit.State != "committed" {
		return fmt.Errorf("commit did not complete (state=%s)", client.FirstNonEmpty(receipt.Commit.State, "unknown"))
	}
	return fmt.Errorf("apply mutation did not complete (state=%s)", client.FirstNonEmpty(receipt.State, "unknown"))
}

func (s *State) autoMergeAppliedWorktree(ctx context.Context, cl *Client, reqID, wtPath string, commit *ChangeCommit, commitCode string) error {
	if wtPath == "" {
		return nil
	}
	if commit != nil {
		s.registerCreatedWorktree(wtPath, commit.OID, commit.Branch)
	}
	if err := s.dispatchMergeWorktree(ctx, cl, reqID+"-merge", wtPath); err != nil {
		return fmt.Errorf("committed [%s] in worktree, but merge failed: %w", commitCode, err)
	}
	return nil
}

func (s *State) syncAppliedInstructions(appliedChange *ChangeSession) {
	if s.Instructions != nil && len(appliedChange.Changes) > 0 && appliedChange.Changes[0].Path == client.FirstNonEmpty(s.Instructions.Path, "AGENTS.md") {
		s.Instructions.Exists = true
		s.Instructions.ExistingContent = appliedChange.Changes[0].Content
	}
}

func (s *State) dispatchUpdateProposal(ctx context.Context, cl *Client, reqID string, act ActionRequest) error {
	if s.Change == nil || s.Project == nil || s.Change.ID == "" {
		return nil
	}
	wtPath := s.Change.WorktreePath
	projID, projRev := s.changeProjectIdentity(s.Change)
	changes := make([]map[string]string, 0, len(s.Change.Changes))
	for _, ch := range s.Change.Changes {
		changes = append(changes, map[string]string{
			"path":    ch.Path,
			"content": ch.Content,
			"summary": ch.Summary,
		})
	}
	body := map[string]any{
		"project_id":       projID,
		"project_revision": projRev,
		"revision":         s.Change.Revision,
		"hash":             s.Change.Hash,
		"message":          client.FirstNonEmpty(act.Message, "Updated proposal hunks in diff editor"),
		"changes":          changes,
	}
	var updated ChangeSession
	sessionID := url.PathEscape(s.Change.ID)
	target := "/api/projects/current/changes/" + sessionID + "/messages" + changeWorktreeScopeQuery(s.Change)
	if err := cl.DoJSON(ctx, reqID, http.MethodPost, target, body, &updated); err != nil {
		return err
	}
	if updated.ID != "" {
		updated.WorktreePath = client.FirstNonEmpty(updated.WorktreePath, wtPath)
		s.Change = &updated
		s.upsertActiveChange(&updated)
	}
	return nil
}

func (s *State) ensureChangeReviewed(ctx context.Context, cl *Client, reqID, sessionID string) {
	if s.Change == nil || s.Change.Hash == "" {
		return
	}
	wtPath := s.Change.WorktreePath
	prevGit := s.Change.Git
	prevTargets := s.Change.FindingTargets
	preserveMeta := func(chg *ChangeSession) {
		chg.WorktreePath = client.FirstNonEmpty(chg.WorktreePath, wtPath)
		if chg.Git == nil && prevGit != nil {
			chg.Git = prevGit
		}
		if len(chg.FindingTargets) == 0 && len(prevTargets) > 0 {
			chg.FindingTargets = prevTargets
		}
	}
	scope := changeWorktreeScopeQuery(s.Change)
	reviewReq := func() map[string]any {
		projID, projRev := s.changeProjectIdentity(s.Change)
		return map[string]any{
			"project_id":       projID,
			"project_revision": projRev,
			"revision":         s.Change.Revision,
			"hash":             s.Change.Hash,
		}
	}
	var reviewed ChangeSession
	if err := cl.DoJSON(ctx, reqID+"-rev", http.MethodPost, "/api/projects/current/changes/"+sessionID+"/review"+scope, reviewReq(), &reviewed); err == nil && reviewed.ID != "" {
		preserveMeta(&reviewed)
		s.Change = &reviewed
		return
	}
	checkReq := reviewReq()
	if s.Change.Kind != "instructions" {
		checkReq["run_tests"] = true
	}
	var checked ChangeSession
	if err := cl.DoJSON(ctx, reqID+"-chk", http.MethodPost, "/api/projects/current/changes/"+sessionID+"/checks"+scope, checkReq, &checked); err == nil && checked.ID != "" {
		preserveMeta(&checked)
		s.Change = &checked
	}
	if err := cl.DoJSON(ctx, reqID+"-rev2", http.MethodPost, "/api/projects/current/changes/"+sessionID+"/review"+scope, reviewReq(), &reviewed); err == nil && reviewed.ID != "" {
		preserveMeta(&reviewed)
		s.Change = &reviewed
	}
}

func (s *State) dispatchDismissFinding(ctx context.Context, cl *Client, reqID string, act ActionRequest) error {
	if act.Issue == nil || act.Issue.ID == "" {
		return nil
	}
	if err := s.patchFindingStatusOnDaemon(ctx, cl, reqID, *act.Issue, "dismissed"); err != nil {
		return err
	}
	s.markFindingDismissed(*act.Issue)
	return nil
}

func (s *State) patchFindingStatusOnDaemon(ctx context.Context, cl *Client, reqID string, issue FindingTarget, status string) error {
	kind := client.FirstNonEmpty(issue.Kind, "semantic")
	body := map[string]string{
		"project_revision": s.projectRevision(),
		"status":           status,
		"kind":             kind,
		"path":             issue.Path,
	}
	if kind == "security" {
		body["source"] = client.FirstNonEmpty(issue.Source, "deterministic")
	}
	target := "/api/projects/current/findings/" + url.PathEscape(issue.ID)
	return cl.DoJSON(ctx, reqID, http.MethodPatch, target, body, nil)
}

func (s *State) dispatchDismissFeature(ctx context.Context, cl *Client, reqID string, act ActionRequest) error {
	if act.Target == "" {
		return nil
	}
	expectedHash := ""
	if s.Features != nil {
		expectedHash = s.Features.Hash
	}
	body := map[string]string{
		"project_id":       s.projectID(),
		"project_revision": s.projectRevision(),
		"expected_hash":    expectedHash,
		"status":           "dismissed",
	}
	var report FeatureReport
	target := "/api/projects/current/features/" + url.PathEscape(act.Target)
	if err := cl.DoJSON(ctx, reqID, http.MethodPatch, target, body, &report); err != nil {
		return err
	}
	if report.Hash != "" || len(report.Suggestions) > 0 {
		s.Features = &report
		return nil
	}
	if s.Features != nil {
		for i := range s.Features.Suggestions {
			if s.Features.Suggestions[i].ID == act.Target {
				s.Features.Suggestions[i].Status = "dismissed"
			}
		}
	}
	return nil
}

func (s *State) dispatchSetTrust(ctx context.Context, cl *Client, reqID string) error {
	body := map[string]any{
		"project_revision": s.projectRevision(),
		"confirm":          s.WorkspaceTrust,
	}
	var trust ExecutionTrust
	return cl.DoJSON(ctx, reqID, http.MethodPost, "/api/projects/current/execution-trust", body, &trust)
}

func (s *State) dispatchUndo(ctx context.Context, cl *Client, reqID string, act ActionRequest) error {
	if act.Target == "" {
		return nil
	}
	undoHash := ""
	if s.ChangeReceipt != nil && s.ChangeReceipt.Hash != "" {
		undoHash = s.ChangeReceipt.Hash
	} else if s.Change != nil {
		undoHash = s.Change.Hash
	}
	body := map[string]any{
		"project_id":       s.projectID(),
		"project_revision": s.projectRevision(),
		"hash":             undoHash,
		"confirm":          true,
	}
	var receipt ChangeMutation
	target := "/api/projects/current/changes/" + url.PathEscape(act.Target) + "/undo"
	if err := cl.DoJSON(ctx, reqID, http.MethodPost, target, body, &receipt); err != nil {
		return err
	}
	s.ChangeReceipt = &receipt
	newState := client.FirstNonEmpty(receipt.State, "reverted")
	var targets []FindingTarget
	if s.Change != nil && s.Change.ID == act.Target {
		s.Change.State = newState
		targets = append(targets, s.Change.FindingTargets...)
	}
	for i := range s.ChangeHistory {
		if s.ChangeHistory[i].ID == act.Target {
			s.ChangeHistory[i].State = newState
			targets = append(targets, s.ChangeHistory[i].FindingTargets...)
		}
	}
	for _, comp := range s.CompletedFindings {
		if comp.SessionID == act.Target {
			targets = append(targets, comp.FindingTarget)
		}
	}
	s.removeFindingCompletions(targets)
	if receipt.ProjectRevision != "" && s.Project != nil {
		s.Project.ProjectRevision = receipt.ProjectRevision
	}
	return nil
}

type changeActionTask struct {
	reqID             string
	isRepair          bool
	session           ChangeSession
	prevSession       ChangeSession
	projID            string
	projRev           string
	workspaceTrust    bool
	message           string
	modelsMap         map[string]string
	confirmedProfiles []string
	fallbackStart     startAgentTask
	hasFallbackStart  bool
}

type changeActionResult struct {
	task          changeActionTask
	updated       ChangeSession
	fallbackRes   *startAgentResult
	usedFallback  bool
	staleFallback bool
	err           error
}

func (s *State) dispatchCheckChange(ctx context.Context, cl *Client, reqID string) error {
	return s.dispatchChangeSessionTask(ctx, cl, reqID, ActionRequest{Kind: "check-change"})
}

func (s *State) dispatchRepairChange(ctx context.Context, cl *Client, reqID string, act ActionRequest) error {
	act.Kind = "repair-change"
	return s.dispatchChangeSessionTask(ctx, cl, reqID, act)
}

func (s *State) dispatchChangeSessionTask(ctx context.Context, cl *Client, reqID string, act ActionRequest) error {
	task, ok := s.prepareChangeActionTask(reqID, act)
	if !ok {
		return nil
	}
	res := executeChangeActionTask(ctx, cl, task)
	return s.applyChangeActionResult(res)
}

func (s *State) prepareChangeActionTask(reqID string, act ActionRequest) (changeActionTask, bool) {
	if s.Change == nil || s.Project == nil || s.Change.ID == "" {
		return changeActionTask{}, false
	}
	prev := cloneChangeSessionSnapshot(s.Change)
	projID, projRev := s.changeProjectIdentity(s.Change)
	modelsMap, confirmedProfiles := s.workflowModelsPayload()
	seed := s.buildSeedFromCurrentChange()
	msg := resolveRepairWorkflowMessage(act.Message, seed, s.Change)
	fallback, hasFallback := s.prepareFallbackStartTask(reqID+"-refresh", s.Change.ID, seed, modelsMap, confirmedProfiles)
	isRepair := act.Kind == "repair-change"
	if isRepair {
		s.markChangeWorkflowRunningOptimistic()
	}
	return changeActionTask{
		reqID:             reqID,
		isRepair:          isRepair,
		session:           prev,
		prevSession:       prev,
		projID:            projID,
		projRev:           projRev,
		workspaceTrust:    s.WorkspaceTrust,
		message:           msg,
		modelsMap:         modelsMap,
		confirmedProfiles: confirmedProfiles,
		fallbackStart:     fallback,
		hasFallbackStart:  hasFallback,
	}, true
}

func cloneChangeSessionSnapshot(chg *ChangeSession) ChangeSession {
	if chg == nil {
		return ChangeSession{}
	}
	cp := *chg
	cp.Targets = append([]ChangeTarget(nil), chg.Targets...)
	cp.Changes = append([]FileChange(nil), chg.Changes...)
	cp.Checks = append([]Check(nil), chg.Checks...)
	cp.FindingTargets = append([]FindingTarget(nil), chg.FindingTargets...)
	if chg.Workflow != nil {
		wf := *chg.Workflow
		wf.Stages = append([]ChangeWorkflowStage(nil), chg.Workflow.Stages...)
		cp.Workflow = &wf
	}
	return cp
}

func resolveRepairWorkflowMessage(explicit string, seed *ChangeSeed, chg *ChangeSession) string {
	if msg := strings.TrimSpace(explicit); msg != "" {
		return msg
	}
	if seed != nil && strings.TrimSpace(seed.Message) != "" {
		return strings.TrimSpace(seed.Message)
	}
	fallback := "Repair the current proposal to satisfy failed checks."
	if chg != nil {
		return firstUserChangeMessage(chg, client.FirstNonEmpty(chg.Title, fallback))
	}
	return fallback
}

func (s *State) prepareFallbackStartTask(reqID, previewID string, seed *ChangeSeed, modelsMap map[string]string, confirmed []string) (startAgentTask, bool) {
	if seed == nil || s.Project == nil || s.Project.ProjectID == "" {
		return startAgentTask{}, false
	}
	paths := domain.EnsureChangeTestPaths(seed.Paths, s.Index)
	if len(paths) == 0 {
		return startAgentTask{}, false
	}
	root := s.Project.Path
	if root == "" && s.Repository != nil {
		root = s.Repository.Root
	}
	head, branch := s.mainGitIdentity()
	return startAgentTask{
		reqID:             reqID,
		act:               ActionRequest{Kind: "start-agent", PreviewID: previewID, Seed: seed},
		projectID:         s.Project.ProjectID,
		projectRevision:   s.Project.ProjectRevision,
		root:              root,
		head:              head,
		branch:            branch,
		workspaceTrust:    s.WorkspaceTrust,
		paths:             paths,
		modelsMap:         modelsMap,
		confirmedProfiles: confirmed,
		existingWorktrees: s.worktreeCollisionEntries(),
	}, true
}

func (s *State) markChangeWorkflowRunningOptimistic() {
	if s.Change == nil {
		return
	}
	if s.Change.State == "failed" || s.Change.State == "interrupted" {
		s.Change.State = "proposed"
	}
	if s.Change.Workflow == nil {
		s.Change.Workflow = &ChangeWorkflow{ID: "wf-" + s.Change.ID}
	}
	s.Change.Workflow.Status = "running"
	s.Change.Workflow.Reason = ""
	if len(s.Change.Workflow.Stages) < 4 {
		s.Change.Workflow.Stages = []ChangeWorkflowStage{
			{Name: "Architect & Code", Status: "running"},
			{Name: "Verify & Test", Status: "pending"},
			{Name: "Critic Review", Status: "pending"},
			{Name: "Human Review", Status: "blocked"},
		}
	} else {
		s.Change.Workflow.Stages[0].Status, s.Change.Workflow.Stages[0].Reason = "running", ""
		s.Change.Workflow.Stages[1].Status, s.Change.Workflow.Stages[1].Reason = "pending", ""
		s.Change.Workflow.Stages[2].Status, s.Change.Workflow.Stages[2].Reason = "pending", ""
		s.Change.Workflow.Stages[3].Status, s.Change.Workflow.Stages[3].Reason = "blocked", ""
	}
	s.upsertActiveChange(s.Change)
}

func executeChangeActionTask(ctx context.Context, cl *Client, task changeActionTask) changeActionResult {
	projID, projRev := ensureChangeTaskTrust(ctx, cl, &task)
	var updated ChangeSession
	var err error
	if task.isRepair {
		updated, err = executeRepairWorkflowRequest(ctx, cl, &task, projID, projRev)
	} else {
		updated, err = executeCheckChangeRequest(ctx, cl, &task, projID, projRev)
	}
	if err != nil && task.hasFallbackStart && shouldFallbackChangeAction(err, task.isRepair) {
		startRes := executeStartAgentTask(ctx, cl, task.fallbackStart)
		return changeActionResult{
			task:          task,
			fallbackRes:   &startRes,
			usedFallback:  true,
			staleFallback: isRevisionConflictError(err),
			err:           startRes.err,
		}
	}
	return changeActionResult{task: task, updated: updated, err: err}
}

func ensureChangeTaskTrust(ctx context.Context, cl *Client, task *changeActionTask) (string, string) {
	projID, projRev := task.projID, task.projRev
	if task.session.WorktreePath != "" {
		return resolveWorktreeIdentityTask(ctx, cl, task.reqID+"-wt", task.session.WorktreePath, projID, projRev, task.workspaceTrust)
	}
	if task.workspaceTrust && projRev != "" {
		body := map[string]any{"project_revision": projRev, "confirm": true}
		var trust ExecutionTrust
		_ = cl.DoJSON(ctx, task.reqID+"-trust", http.MethodPost, "/api/projects/current/execution-trust", body, &trust)
	}
	return projID, projRev
}

func executeRepairWorkflowRequest(ctx context.Context, cl *Client, task *changeActionTask, projID, projRev string) (ChangeSession, error) {
	wfSession, err := postRepairWorkflowAttempt(ctx, cl, task.reqID, task, projID, projRev, task.session.Revision, task.session.Hash)
	if err == nil || !isRevisionConflictError(err) {
		return wfSession, err
	}
	latest, ok := fetchLatestChangeSessionTask(ctx, cl, task.reqID+"-get", &task.session, projID, projRev)
	if !ok {
		return wfSession, err
	}
	if latest.Workflow != nil && latest.Workflow.Status == "running" {
		return latest, nil
	}
	lProjID := client.FirstNonEmpty(latest.ProjectID, projID)
	lProjRev := client.FirstNonEmpty(latest.ProjectRevision, projRev)
	return postRepairWorkflowAttempt(ctx, cl, task.reqID+"-retry", task, lProjID, lProjRev, latest.Revision, latest.Hash)
}

func postRepairWorkflowAttempt(ctx context.Context, cl *Client, reqID string, task *changeActionTask, projID, projRev string, rev int, hash string) (ChangeSession, error) {
	req := map[string]any{
		"project_id":         projID,
		"project_revision":   projRev,
		"revision":           rev,
		"hash":               hash,
		"message":            task.message,
		"models":             task.modelsMap,
		"confirmed_profiles": task.confirmedProfiles,
		"confirm_security":   true,
		"auto_repair":        true,
	}
	var updated ChangeSession
	target := "/api/projects/current/changes/" + url.PathEscape(task.session.ID) + "/workflow" + changeWorktreeScopeQuery(&task.session)
	err := cl.DoJSON(ctx, reqID, http.MethodPost, target, req, &updated)
	return updated, err
}

func fetchLatestChangeSessionTask(ctx context.Context, cl *Client, reqID string, session *ChangeSession, projID, projRev string) (ChangeSession, bool) {
	if session == nil || session.ID == "" || projID == "" || projRev == "" {
		return ChangeSession{}, false
	}
	q := fmt.Sprintf("?project_id=%s&project_revision=%s", url.QueryEscape(projID), url.QueryEscape(projRev))
	if session.WorktreePath != "" {
		q += "&worktree_path=" + url.QueryEscape(session.WorktreePath)
	}
	var latest ChangeSession
	target := "/api/projects/current/changes/" + url.PathEscape(session.ID) + q
	if err := cl.DoJSON(ctx, reqID, http.MethodGet, target, nil, &latest); err != nil || latest.ID == "" {
		return ChangeSession{}, false
	}
	return latest, true
}

func executeCheckChangeRequest(ctx context.Context, cl *Client, task *changeActionTask, projID, projRev string) (ChangeSession, error) {
	req := map[string]any{
		"project_id":       projID,
		"project_revision": projRev,
		"revision":         task.session.Revision,
		"hash":             task.session.Hash,
		"run_tests":        true,
		"run_lint":         true,
	}
	var updated ChangeSession
	target := "/api/projects/current/changes/" + url.PathEscape(task.session.ID) + "/checks" + changeWorktreeScopeQuery(&task.session)
	err := cl.DoJSON(ctx, task.reqID, http.MethodPost, target, req, &updated)
	return updated, err
}

func shouldFallbackChangeAction(err error, isRepair bool) bool {
	if isRevisionConflictError(err) {
		return true
	}
	if !isRepair || err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "include a test path") ||
		strings.Contains(msg, "start a new feature") ||
		strings.Contains(msg, "not found")
}

func (s *State) applyChangeActionResult(res changeActionResult) error {
	if res.usedFallback && res.fallbackRes != nil {
		if res.task.session.ID != "" && res.fallbackRes.session.ID != "" && res.fallbackRes.session.ID != res.task.session.ID {
			s.removeActiveChangeByID(res.task.session.ID)
		}
		if err := s.applyStartAgentResult(*res.fallbackRes); err != nil {
			return err
		}
		if res.staleFallback && res.task.fallbackStart.act.Seed != nil {
			s.CmdOutput = fmt.Sprintf("Proposal was stale — started fresh agent mission for %q", res.task.fallbackStart.act.Seed.Title)
		}
		return nil
	}
	if res.err != nil {
		if res.task.isRepair {
			prev := res.task.prevSession
			s.Change = &prev
			s.upsertActiveChange(&prev)
		}
		s.Error = res.err.Error()
		return res.err
	}
	updated := res.updated
	updated.WorktreePath = client.FirstNonEmpty(updated.WorktreePath, res.task.session.WorktreePath)
	if len(updated.FindingTargets) == 0 && len(res.task.session.FindingTargets) > 0 {
		updated.FindingTargets = res.task.session.FindingTargets
	}
	s.Change = &updated
	s.upsertActiveChange(&updated)
	if s.Page == PageAgents {
		s.syncSelectedIdxToCurrentChange()
	}
	return nil
}

type mergeWorktreeResponse struct {
	State           string         `json:"state"`
	Error           string         `json:"error"`
	RemovedWorktree string         `json:"removed_worktree"`
	Warnings        []string       `json:"warnings"`
	Index           *ProjectIndex  `json:"index"`
	Repository      *GitRepository `json:"repository"`
}

func (s *State) dispatchMergeWorktree(ctx context.Context, cl *Client, reqID, wtPath string) error {
	if s.Project == nil || wtPath == "" {
		return nil
	}
	result, err := s.postWorktreeMutation(ctx, cl, reqID, "/api/projects/current/git/merge", wtPath, "", "")
	if err != nil {
		return err
	}
	if result.State != "" && result.State != "merged" {
		return fmt.Errorf("merge did not complete (state=%s)", result.State)
	}
	s.applyMergeWorktreeResponse(wtPath, result)
	return nil
}

func (s *State) dispatchRejectChange(ctx context.Context, cl *Client, reqID string, act ActionRequest) error {
	s.cancelRejectedChangeWorkflow(ctx, cl, reqID, act)
	if act.Seed != nil && act.Seed.WorktreePath != "" && s.Project != nil {
		wtPath := act.Seed.WorktreePath
		result, err := s.postWorktreeMutation(ctx, cl, reqID+"-discard", "/api/projects/current/git/discard-worktree", wtPath, act.Seed.Message, act.Seed.Kind)
		s.applyMergeWorktreeResponse(wtPath, result)
		if err != nil {
			return err
		}
		if result.State != "" && result.State != "discarded" {
			return fmt.Errorf("discard did not complete (state=%s)", result.State)
		}
	}
	if act.Seed != nil && len(act.Seed.FindingTargets) > 0 {
		s.reopenRejectedTargets(ctx, cl, reqID, act.Seed.FindingTargets)
	}
	return nil
}

func (s *State) cancelRejectedChangeWorkflow(ctx context.Context, cl *Client, reqID string, act ActionRequest) {
	if act.PreviewID == "" || act.Target == "" || isLocalPreviewChangeID(act.Target) || s.Project == nil {
		return
	}
	scope := ""
	if act.Seed != nil && act.Seed.WorktreePath != "" {
		scope = "?worktree_path=" + url.QueryEscape(act.Seed.WorktreePath)
	}
	cancelReq := map[string]any{
		"project_id":       s.Project.ProjectID,
		"project_revision": s.Project.ProjectRevision,
		"workflow_id":      act.PreviewID,
	}
	_ = cl.DoJSON(ctx, reqID+"-cancel", http.MethodPost, "/api/projects/current/changes/"+url.PathEscape(act.Target)+"/workflow/cancel"+scope, cancelReq, nil)
}

func (s *State) reopenRejectedTargets(ctx context.Context, cl *Client, reqID string, targets []FindingTarget) {
	for idx, ft := range targets {
		if ft.ID != "" {
			_ = s.patchFindingStatusOnDaemon(ctx, cl, fmt.Sprintf("%s-reopen-%d", reqID, idx), ft, "open")
		}
	}
	s.reopenFindingTargets(targets)
}

func (s *State) postWorktreeMutation(ctx context.Context, cl *Client, reqID, endpoint, wtPath, fallbackHead, fallbackBranch string) (mergeWorktreeResponse, error) {
	_ = s.ensureExecutionTrust(ctx, cl, reqID+"-trust")
	req := s.buildWorktreeMutationRequest(ctx, cl, reqID, wtPath, fallbackHead, fallbackBranch, false)
	var result mergeWorktreeResponse
	err := cl.DoJSON(ctx, reqID, http.MethodPost, endpoint, req, &result)
	if err != nil && isRevisionConflictError(err) {
		req = s.buildWorktreeMutationRequest(ctx, cl, reqID+"-retry", wtPath, fallbackHead, fallbackBranch, true)
		_ = s.ensureExecutionTrust(ctx, cl, reqID+"-trust-retry")
		err = cl.DoJSON(ctx, reqID+"-retry", http.MethodPost, endpoint, req, &result)
	}
	if err != nil {
		return result, err
	}
	if result.Error != "" {
		return result, fmt.Errorf("%s", result.Error)
	}
	return result, nil
}

func (s *State) applyMergeWorktreeResponse(wtPath string, result mergeWorktreeResponse) {
	if result.Index != nil {
		s.Index = result.Index
		if result.Index.ProjectRevision != "" {
			s.Project.ProjectRevision = result.Index.ProjectRevision
		}
	}
	if result.Repository != nil {
		s.Repository = result.Repository
		if s.GitWorkspace != nil {
			s.GitWorkspace.GitRepository = *result.Repository
		}
		if result.Repository.ProjectRevision != "" && s.Project != nil {
			s.Project.ProjectRevision = result.Repository.ProjectRevision
		}
	}
	s.removeWorktreeByPath(wtPath)
	if chg := s.findActiveChangeByWorktree(wtPath); chg != nil && chg.Commit != nil && chg.Commit.State == "committed" {
		commitCode := domain.ResolveCommitCode(chg.ID, chg.Hash, chg.FindingTargets, chg.Commit)
		s.finalizeAppliedChange(chg, chg.Commit, commitCode)
	}
	if result.RemovedWorktree == "" && len(result.Warnings) > 0 {
		s.CmdOutput = strings.Join(result.Warnings, "; ")
	}
}

func (s *State) buildWorktreeMutationRequest(ctx context.Context, cl *Client, reqID, wtPath, fallbackHead, fallbackBranch string, forceRefresh bool) map[string]any {
	head, branch, srcHead, srcBranch := s.resolveMergeWorktreeIdentities(wtPath)
	srcHead = client.FirstNonEmpty(srcHead, fallbackHead)
	srcBranch = client.FirstNonEmpty(srcBranch, fallbackBranch)
	if forceRefresh || head == "" || branch == "" || srcHead == "" || srcBranch == "" {
		s.refreshMergeWorkspaceState(ctx, cl, reqID)
		h2, b2, sh2, sb2 := s.resolveMergeWorktreeIdentities(wtPath)
		head = client.FirstNonEmpty(h2, head)
		branch = client.FirstNonEmpty(b2, branch)
		srcHead = client.FirstNonEmpty(sh2, srcHead, fallbackHead)
		srcBranch = client.FirstNonEmpty(sb2, srcBranch, fallbackBranch)
	}
	return map[string]any{
		"project_id":        s.Project.ProjectID,
		"project_revision":  s.Project.ProjectRevision,
		"head":              head,
		"branch":            branch,
		"path":              wtPath,
		"source_head":       srcHead,
		"source_branch":     srcBranch,
		"confirm":           true,
		"confirm_execution": true,
	}
}

func (s *State) refreshMergeWorkspaceState(ctx context.Context, cl *Client, reqID string) {
	var proj Project
	if err := cl.DoJSON(ctx, reqID+"-proj", http.MethodGet, "/api/projects/current", nil, &proj); err == nil && proj.ProjectID != "" {
		s.Project = &proj
	}
	_, idQuery := s.resourceQueries()
	var ws GitWorkspace
	if err := cl.DoJSON(ctx, reqID+"-ws", http.MethodGet, "/api/projects/current/git/workspace"+idQuery, nil, &ws); err == nil && ws.ProjectID != "" {
		s.GitWorkspace = &ws
		s.Repository = &ws.GitRepository
	}
}

func (s *State) resolveMergeWorktreeIdentities(wtPath string) (head, branch, srcHead, srcBranch string) {
	head, branch = s.mainGitIdentity()
	if s.GitWorkspace != nil {
		for _, wt := range s.GitWorkspace.Worktrees {
			if wt.Path == wtPath {
				srcHead = wt.Head
				srcBranch = wt.Branch
				break
			}
		}
	}
	if chg := s.findActiveChangeByWorktree(wtPath); chg != nil {
		if chg.Commit != nil && chg.Commit.State == "committed" {
			srcHead = client.FirstNonEmpty(chg.Commit.OID, srcHead)
			srcBranch = client.FirstNonEmpty(chg.Commit.Branch, srcBranch)
		}
		if chg.Git != nil {
			srcHead = client.FirstNonEmpty(srcHead, chg.Git.Head)
			srcBranch = client.FirstNonEmpty(srcBranch, chg.Git.Branch)
		}
	}
	return head, branch, srcHead, srcBranch
}

func (s *State) findActiveChangeByWorktree(wtPath string) *ChangeSession {
	if wtPath == "" {
		return nil
	}
	if s.Change != nil && s.Change.WorktreePath == wtPath {
		return s.Change
	}
	for _, chg := range s.ActiveChanges {
		if chg != nil && chg.WorktreePath == wtPath {
			return chg
		}
	}
	return nil
}

func (s *State) dispatchLoadChange(ctx context.Context, cl *Client, reqID, sessionID string) error {
	if sessionID == "" || s.Project == nil {
		return nil
	}
	prev := s.findTrackedChangeSession(sessionID)
	var wtPath string
	if prev != nil {
		wtPath = prev.WorktreePath
	}
	var chg ChangeSession
	target := "/api/projects/current/changes/" + url.PathEscape(sessionID) + s.changeResourceQuery(prev)
	if err := cl.DoJSON(ctx, reqID, http.MethodGet, target, nil, &chg); err != nil {
		return err
	}
	if chg.ID != "" {
		chg.WorktreePath = client.FirstNonEmpty(chg.WorktreePath, wtPath)
		s.Change = &chg
		s.upsertActiveChange(&chg)
	}
	return nil
}

func (s *State) findTrackedChangeSession(sessionID string) *ChangeSession {
	if s.Change != nil && s.Change.ID == sessionID {
		return s.Change
	}
	for _, chg := range s.ActiveChanges {
		if chg != nil && chg.ID == sessionID {
			return chg
		}
	}
	return nil
}

func (s *State) dispatchSaveSelection(ctx context.Context, cl *Client, reqID string) error {
	if s.Project == nil || s.Project.ProjectID == "" {
		return nil
	}
	if s.Selection == nil || s.Selection.SelectionID == "" {
		_, idQuery := s.resourceQueries()
		var sel Selection
		if err := cl.DoJSON(ctx, reqID+"-get", http.MethodGet, "/api/projects/current/analysis/selection"+idQuery, nil, &sel); err == nil {
			s.Selection = &sel
		}
	}
	if s.Selection == nil || s.Selection.SelectionID == "" {
		return nil
	}
	excluded := s.ExcludedPaths
	if excluded == nil {
		excluded = []string{}
	}
	req := map[string]any{
		"project_id":       s.Project.ProjectID,
		"project_revision": s.Project.ProjectRevision,
		"selection_id":     s.Selection.SelectionID,
		"excluded_paths":   excluded,
	}
	var updated Selection
	if err := cl.DoJSON(ctx, reqID, http.MethodPost, "/api/projects/current/analysis/selection", req, &updated); err != nil {
		return err
	}
	s.Selection = &updated
	s.ExcludedPaths = updated.ExcludedPaths
	return nil
}

func (s *State) dispatchProposeInstructions(ctx context.Context, cl *Client, reqID, path, content string) error {
	if s.Project == nil || s.Project.ProjectID == "" {
		return nil
	}
	targetPath := client.FirstNonEmpty(path, "AGENTS.md")
	content = s.syncInstructionDraft(ctx, cl, reqID, targetPath, content)
	draft := client.FirstNonEmpty(content, s.InstructionDraft)
	if draft == "" {
		return nil
	}
	if s.Instructions != nil && s.Instructions.Exists && s.Instructions.ExistingContent == draft {
		s.CmdOutput = fmt.Sprintf("%s already contains the selected guardrail", targetPath)
		return nil
	}
	req := map[string]any{
		"project_id":       s.Project.ProjectID,
		"project_revision": s.Project.ProjectRevision,
		"title":            "Update " + targetPath,
		"kind":             "instructions",
		"paths":            []string{targetPath},
		"content":          draft,
	}
	var session ChangeSession
	err := cl.DoJSON(ctx, reqID, http.MethodPost, "/api/projects/current/instructions/proposal", req, &session)
	if err != nil && isRevisionConflictError(err) {
		s.syncProjectRevision(ctx, cl, reqID+"-sync")
		req["project_revision"] = s.Project.ProjectRevision
		err = cl.DoJSON(ctx, reqID+"-retry", http.MethodPost, "/api/projects/current/instructions/proposal", req, &session)
	}
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "instructions are unchanged") {
			s.CmdOutput = fmt.Sprintf("%s already contains the selected guardrail", targetPath)
			return nil
		}
		return err
	}
	s.Change = &session
	s.prepareInstructionProposal(ctx, cl, reqID)
	s.CmdOutput = fmt.Sprintf("Prepared %s guardrail proposal — press [d] to inspect diff or [a/c] to apply & commit", targetPath)
	return nil
}

func (s *State) syncInstructionDraft(ctx context.Context, cl *Client, reqID, targetPath, content string) string {
	if s.Instructions != nil {
		return content
	}
	_, idQuery := s.resourceQueries()
	var inst InstructionPreview
	instrTarget := "/api/projects/current/instructions" + idQuery + "&path=" + url.QueryEscape(targetPath)
	if err := cl.DoJSON(ctx, reqID+"-inst", http.MethodGet, instrTarget, nil, &inst); err != nil {
		return content
	}
	s.Instructions = &inst
	if len(s.GraduatedLessons) == 0 {
		return content
	}
	merged := inst.ExistingContent
	for _, rule := range s.GraduatedLessons {
		merged = domain.InsertGuidelineSection(merged, "Harness² Lessons & Guardrails", rule)
	}
	s.InstructionDraft = merged
	return merged
}

func (s *State) prepareInstructionProposal(ctx context.Context, client *Client, reqID string) {
	if s.Change == nil || s.Change.ID == "" || s.Change.Hash == "" || s.Project == nil {
		return
	}
	req := map[string]any{
		"project_id":       s.Project.ProjectID,
		"project_revision": s.Project.ProjectRevision,
		"revision":         s.Change.Revision,
		"hash":             s.Change.Hash,
		"run_tests":        false,
	}
	target := "/api/projects/current/changes/" + url.PathEscape(s.Change.ID)
	var checked ChangeSession
	if err := client.DoJSON(ctx, reqID+"-chk", http.MethodPost, target+"/checks", req, &checked); err == nil && checked.ID != "" {
		s.Change = &checked
		req["revision"] = s.Change.Revision
		req["hash"] = s.Change.Hash
	}
	var reviewed ChangeSession
	if err := client.DoJSON(ctx, reqID+"-rev", http.MethodPost, target+"/review", req, &reviewed); err == nil && reviewed.ID != "" {
		s.Change = &reviewed
	}
}

func (s *State) dispatchBootstrap(ctx context.Context, client *Client, reqID, pillarID string) error {
	if s.Project == nil || s.Project.ProjectRevision == "" {
		return nil
	}
	_ = s.ensureExecutionTrust(ctx, client, reqID+"-trust")
	if pillarID == "agents-md" {
		draft := s.InstructionDraft
		if draft == "" && s.Instructions != nil && len(s.Instructions.Presets) > 0 {
			draft = s.Instructions.Presets[0].Content
		}
		if draft == "" {
			draft = "# Project instructions\n\n## Verification\n- Run tests before committing changes.\n"
		}
		return s.dispatchProposeInstructions(ctx, client, reqID, "AGENTS.md", draft)
	}
	body := map[string]any{
		"project_revision": s.Project.ProjectRevision,
		"confirm":          true,
	}
	return client.DoJSON(ctx, reqID, http.MethodPost, "/api/projects/current/bootstrap/exec", body, nil)
}
