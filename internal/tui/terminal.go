package tui

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
)

func isCharDevice(in io.Reader) (*os.File, bool) {
	f, ok := in.(*os.File)
	if !ok {
		return nil, false
	}
	info, err := f.Stat()
	if err != nil || (info.Mode()&os.ModeCharDevice) == 0 {
		return nil, false
	}
	return f, true
}

// DetectTerminalSize inspects the active controlling terminal or environment for full-screen dimensions.
func DetectTerminalSize(fallbackWidth, fallbackHeight int) (int, int) {
	if fallbackWidth > 0 && fallbackHeight > 0 {
		return fallbackWidth, fallbackHeight
	}
	cols, rows := querySttySize(os.Stdin)
	if cols <= 0 || rows <= 0 {
		if tty, err := os.Open("/dev/tty"); err == nil {
			cols, rows = querySttySize(tty)
			_ = tty.Close()
		}
	}
	if cols <= 0 {
		cols, _ = strconv.Atoi(os.Getenv("COLUMNS"))
	}
	if rows <= 0 {
		rows, _ = strconv.Atoi(os.Getenv("LINES"))
	}
	if fallbackWidth > 0 {
		cols = fallbackWidth
	} else if cols <= 0 {
		cols = 120
	}
	if fallbackHeight > 0 {
		rows = fallbackHeight
	} else if rows <= 0 {
		rows = 36
	}
	return cols, rows
}

func querySttySize(f *os.File) (int, int) {
	if f == nil {
		return 0, 0
	}
	cmd := exec.Command("stty", "size")
	cmd.Stdin = f
	out, err := cmd.Output()
	if err != nil {
		return 0, 0
	}
	fields := strings.Fields(string(out))
	if len(fields) != 2 {
		return 0, 0
	}
	rows, err1 := strconv.Atoi(fields[0])
	cols, err2 := strconv.Atoi(fields[1])
	if err1 != nil || err2 != nil {
		return 0, 0
	}
	return cols, rows
}

// DecodeInputBytes parses raw terminal input bytes into discrete KeyEvents.
func DecodeInputBytes(data []byte) []KeyEvent {
	var events []KeyEvent
	for i := 0; i < len(data); {
		if ev, consumed := decodeEscapeSequence(data[i:]); consumed > 0 {
			events = append(events, ev)
			i += consumed
			continue
		}
		ev, consumed := decodeSingleKey(data[i:])
		events = append(events, ev)
		i += consumed
	}
	return events
}

func decodeEscapeSequence(buf []byte) (KeyEvent, int) {
	if len(buf) == 0 || buf[0] != 0x1b {
		return KeyEvent{}, 0
	}
	if len(buf) >= 3 && buf[1] == '[' {
		switch buf[2] {
		case 'A':
			return KeyEvent{Key: "ArrowUp"}, 3
		case 'B':
			return KeyEvent{Key: "ArrowDown"}, 3
		case 'C':
			return KeyEvent{Key: "ArrowRight"}, 3
		case 'D':
			return KeyEvent{Key: "ArrowLeft"}, 3
		case 'Z':
			return KeyEvent{Key: "Tab", Shift: true}, 3
		}
	}
	return KeyEvent{Key: "Escape"}, 1
}

func decodeSingleKey(buf []byte) (KeyEvent, int) {
	switch buf[0] {
	case 0x03:
		return KeyEvent{Key: "c", Ctrl: true}, 1
	case '\r', '\n':
		return KeyEvent{Key: "Enter"}, 1
	case '\t':
		return KeyEvent{Key: "Tab"}, 1
	case 0x7f, 0x08:
		return KeyEvent{Key: "Backspace"}, 1
	}
	r, size := utf8.DecodeRune(buf)
	if r == utf8.RuneError && size <= 1 {
		return KeyEvent{Key: string(buf[0])}, 1
	}
	return KeyEvent{Key: string(r)}, size
}

// FlushActions dispatches queued PendingActions to the daemon and clears the queue.
func (s *State) FlushActions(ctx context.Context, client *Client) {
	if client == nil || len(s.PendingActions) == 0 {
		s.PendingActions = nil
		return
	}
	actions := s.PendingActions
	s.PendingActions = nil
	for idx, act := range actions {
		reqID := fmt.Sprintf("act-%d-%s", idx, act.Kind)
		if err := s.dispatchAction(ctx, client, reqID, act); err != nil {
			s.Error = err.Error()
		}
	}
}

func (s *State) dispatchAction(ctx context.Context, client *Client, reqID string, act ActionRequest) error {
	switch act.Kind {
	case "reconnect":
		return s.SyncFromDaemon(ctx, client, "")
	case "open-project":
		return s.SyncFromDaemon(ctx, client, act.Target)
	case "reindex":
		return s.dispatchReindex(ctx, client, reqID)
	case "scan":
		return s.dispatchScan(ctx, client, reqID)
	case "open-file":
		return s.fetchFileInfo(ctx, client, reqID, act.Target)
	case "run-analysis", "reanalyze", "repair-analysis", "pause-analysis", "resume-analysis":
		return s.dispatchAnalysisAction(ctx, client, reqID, act)
	default:
		return s.dispatchMutationAction(ctx, client, reqID, act)
	}
}

func (s *State) ensureExecutionTrust(ctx context.Context, client *Client, reqID string) error {
	if !s.WorkspaceTrust || s.Project == nil || s.Project.ProjectRevision == "" {
		return nil
	}
	body := map[string]any{
		"project_revision": s.Project.ProjectRevision,
		"confirm":          true,
	}
	var trust ExecutionTrust
	if err := client.DoJSON(ctx, reqID, http.MethodPost, "/api/projects/current/execution-trust", body, &trust); err != nil {
		return err
	}
	s.WorkspaceTrust = trust.Trusted || s.WorkspaceTrust
	return nil
}

func (s *State) dispatchReindex(ctx context.Context, client *Client, reqID string) error {
	var idx ProjectIndex
	body := map[string]any{"project_revision": s.projectRevision()}
	if err := client.DoJSON(ctx, reqID, http.MethodPost, "/api/projects/current/reindex", body, &idx); err != nil {
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

func (s *State) dispatchScan(ctx context.Context, client *Client, reqID string) error {
	_ = s.ensureExecutionTrust(ctx, client, reqID+"-trust")
	var scan Scan
	body := map[string]any{"project_revision": s.projectRevision()}
	if err := client.DoJSON(ctx, reqID, http.MethodPost, "/api/projects/current/scan", body, &scan); err != nil {
		return err
	}
	if s.AcceptProjectResponse(scan.ProjectIdentity) {
		s.Scan = &scan
	}
	return nil
}

func (s *State) fetchFileInfo(ctx context.Context, client *Client, reqID, path string) error {
	var info FileInfo
	escaped := url.QueryEscape(path)
	if err := client.DoJSON(ctx, reqID, http.MethodGet, "/api/projects/current/files/info?path="+escaped, nil, &info); err != nil {
		return err
	}
	if s.AcceptFileResponse(info.Path, "") {
		s.File = &info
	}
	var symResp struct {
		Symbols []SymbolInfo `json:"symbols"`
	}
	if err := client.DoJSON(ctx, reqID+"-sym", http.MethodGet, "/api/projects/current/files/symbols?path="+escaped, nil, &symResp); err == nil {
		s.Symbols = symResp.Symbols
	}
	return nil
}

func (s *State) dispatchAnalysisAction(ctx context.Context, client *Client, reqID string, act ActionRequest) error {
	switch act.Kind {
	case "pause-analysis":
		return s.dispatchPauseAnalysis(ctx, client, reqID)
	case "resume-analysis":
		return s.dispatchResumeAnalysis(ctx, client, reqID)
	default:
		return s.dispatchAnalysisRun(ctx, client, reqID, act)
	}
}

func defaultAnalysisLimits() map[string]int {
	return map[string]int{
		"batch_files":            64,
		"budget_seconds":         300,
		"max_attempts_per_stage": 2,
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

func (s *State) syncProjectRevision(ctx context.Context, client *Client, reqID string) {
	_ = s.dispatchReindex(ctx, client, reqID+"-reidx")
	var proj Project
	if err := client.DoJSON(ctx, reqID+"-proj", http.MethodGet, "/api/projects/current", nil, &proj); err == nil && proj.ProjectID != "" {
		s.Project = &proj
	}
}

func (s *State) cancelPausedAnalysis(ctx context.Context, client *Client, reqID string) {
	if s.Run == nil || s.Run.Identity.ID == "" {
		return
	}
	ctrlReq := map[string]any{"identity": s.Run.Identity, "action": "cancel"}
	var run AnalysisRun
	if err := client.DoJSON(ctx, reqID, http.MethodPost, "/api/projects/current/analysis/run/control", ctrlReq, &run); err == nil && run.Identity.ID != "" {
		s.Run = &run
	}
}

func (s *State) dispatchAnalysisRun(ctx context.Context, client *Client, reqID string, act ActionRequest) error {
	if s.Project == nil || s.Project.ProjectID == "" {
		return nil
	}
	if act.Kind == "run-analysis" && s.isAnalysisPausedOrInterrupted() {
		return s.dispatchResumeAnalysis(ctx, client, reqID)
	}
	if act.Kind == "repair-analysis" && s.isAnalysisPausedOrInterrupted() {
		s.cancelPausedAnalysis(ctx, client, reqID+"-cancel")
	}
	refresh := act.Kind == "reanalyze"
	clearResults := refresh && s.isAnalysisPausedOrInterrupted()
	retryStale := act.Kind == "repair-analysis" && act.Target == ""
	staleOnly := act.Kind == "repair-analysis" && act.Target != ""
	includeFeatures := !staleOnly

	preview, err := s.requestAnalysisPreview(ctx, client, reqID, refresh, clearResults, includeFeatures, retryStale, staleOnly, act.Target)
	if err != nil {
		return err
	}
	startReq := map[string]any{
		"identity":           preview.Identity,
		"preview_id":         preview.PreviewID,
		"limits":             preview.Limits,
		"refresh":            refresh,
		"clear_results":      clearResults,
		"include_features":   includeFeatures,
		"retry_stale_failed": retryStale,
		"stale_only":         staleOnly,
		"stale_path":         act.Target,
		"confirmations": map[string]any{
			"provider_ids":    preview.providerIDs(),
			"security_review": true,
		},
	}
	var run AnalysisRun
	if err := client.DoJSON(ctx, reqID+"-start", http.MethodPost, "/api/projects/current/analysis/run", startReq, &run); err != nil {
		return err
	}
	s.Run = &run
	return nil
}

func (s *State) requestAnalysisPreview(ctx context.Context, client *Client, reqID string, refresh, clearResults, includeFeatures, retryStale, staleOnly bool, stalePath string) (analysisPreviewPayload, error) {
	buildReq := func() map[string]any {
		return map[string]any{
			"project_id":         s.Project.ProjectID,
			"project_revision":   s.Project.ProjectRevision,
			"scope":              "project",
			"refresh":            refresh,
			"clear_results":      clearResults,
			"include_features":   includeFeatures,
			"retry_stale_failed": retryStale,
			"stale_only":         staleOnly,
			"stale_path":         stalePath,
			"limits":             defaultAnalysisLimits(),
		}
	}
	var preview analysisPreviewPayload
	err := client.DoJSON(ctx, reqID+"-prev", http.MethodPost, "/api/projects/current/analysis/preview", buildReq(), &preview)
	if err != nil && isRevisionConflictError(err) {
		s.syncProjectRevision(ctx, client, reqID+"-sync")
		err = client.DoJSON(ctx, reqID+"-prev2", http.MethodPost, "/api/projects/current/analysis/preview", buildReq(), &preview)
	}
	return preview, err
}

func (s *State) dispatchPauseAnalysis(ctx context.Context, client *Client, reqID string) error {
	if s.Run == nil || s.Run.Identity.ID == "" {
		return nil
	}
	ctrlReq := map[string]any{
		"identity": s.Run.Identity,
		"action":   "pause",
	}
	var run AnalysisRun
	if err := client.DoJSON(ctx, reqID, http.MethodPost, "/api/projects/current/analysis/run/control", ctrlReq, &run); err != nil {
		return err
	}
	s.Run = &run
	return nil
}

func (s *State) dispatchResumeAnalysis(ctx context.Context, client *Client, reqID string) error {
	if s.Project == nil || s.Project.ProjectID == "" {
		return nil
	}
	if s.Run == nil || s.Run.Identity.ID == "" {
		_, idQuery := s.resourceQueries()
		var latest AnalysisRun
		if err := client.DoJSON(ctx, reqID+"-fetch", http.MethodGet, "/api/projects/current/analysis/run"+idQuery, nil, &latest); err == nil && latest.Identity.ID != "" {
			s.Run = &latest
		}
	}
	if s.Run == nil || s.Run.Identity.ID == "" || !s.isAnalysisPausedOrInterrupted() {
		return s.dispatchAnalysisRun(ctx, client, reqID, ActionRequest{Kind: "run-analysis"})
	}
	limits := s.Run.Plan.Limits
	if len(limits) == 0 {
		limits = defaultAnalysisLimits()
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
	if err := client.DoJSON(ctx, reqID+"-prev", http.MethodPost, "/api/projects/current/analysis/preview", previewReq, &preview); err != nil {
		if isRevisionConflictError(err) {
			s.syncProjectRevision(ctx, client, reqID+"-sync")
			s.cancelPausedAnalysis(ctx, client, reqID+"-cancel")
			return s.dispatchAnalysisRun(ctx, client, reqID, ActionRequest{Kind: "run-analysis"})
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
	if err := client.DoJSON(ctx, reqID+"-resume", http.MethodPost, "/api/projects/current/analysis/run/control", ctrlReq, &run); err != nil {
		return err
	}
	s.Run = &run
	return nil
}

func (s *State) dispatchMutationAction(ctx context.Context, client *Client, reqID string, act ActionRequest) error {
	switch act.Kind {
	case "start-agent":
		return s.dispatchStartAgent(ctx, client, reqID, act)
	case "apply-commit":
		return s.dispatchApplyCommit(ctx, client, reqID, act)
	case "dismiss-finding":
		return s.dispatchDismissFinding(ctx, client, reqID, act)
	case "dismiss-feature":
		return s.dispatchDismissFeature(ctx, client, reqID, act)
	case "set-trust":
		return s.dispatchSetTrust(ctx, client, reqID)
	case "undo":
		return s.dispatchUndo(ctx, client, reqID, act)
	default:
		return s.dispatchSecondaryMutation(ctx, client, reqID, act)
	}
}

func (s *State) dispatchSecondaryMutation(ctx context.Context, client *Client, reqID string, act ActionRequest) error {
	switch act.Kind {
	case "update-proposal":
		return s.dispatchUpdateProposal(ctx, client, reqID, act)
	case "check-change":
		return s.dispatchCheckChange(ctx, client, reqID)
	case "repair-change":
		return s.dispatchRepairChange(ctx, client, reqID, act)
	case "merge-worktree":
		return s.dispatchMergeWorktree(ctx, client, reqID, act.Target)
	case "load-change":
		return s.dispatchLoadChange(ctx, client, reqID, act.Target)
	case "save-selection":
		return s.dispatchSaveSelection(ctx, client, reqID)
	case "propose-instructions":
		return s.dispatchProposeInstructions(ctx, client, reqID, act.Target, act.Message)
	case "bootstrap":
		return s.dispatchBootstrap(ctx, client, reqID, act.Target)
	case "cancel-workflow":
		return s.dispatchCancelWorkflow(ctx, client, reqID, act.Target)
	case "refresh-models":
		s.RefreshModels(ctx, client)
		return nil
	default:
		return nil
	}
}

func (s *State) dispatchCancelWorkflow(ctx context.Context, client *Client, reqID, sessionID string) error {
	if s.Change == nil || s.Change.Workflow == nil || s.Project == nil {
		return nil
	}
	id := firstNonEmpty(sessionID, s.Change.ID)
	req := map[string]any{
		"project_id":       s.Project.ProjectID,
		"project_revision": s.Project.ProjectRevision,
		"workflow_id":      s.Change.Workflow.ID,
	}
	var updated ChangeSession
	target := "/api/projects/current/changes/" + url.PathEscape(id) + "/workflow/cancel"
	if err := client.DoJSON(ctx, reqID, http.MethodPost, target, req, &updated); err != nil {
		return err
	}
	if updated.ID != "" {
		s.Change = &updated
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
		if !containsString(confirmed, sel) {
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

func (s *State) dispatchStartAgent(ctx context.Context, client *Client, reqID string, act ActionRequest) error {
	if act.Seed == nil || s.Project == nil || s.Project.ProjectID == "" {
		return nil
	}
	_ = s.ensureExecutionTrust(ctx, client, reqID+"-trust")
	paths := EnsureChangeTestPaths(act.Seed.Paths, s.Index)
	if len(paths) == 0 {
		return nil
	}
	session, err := s.openChangeSessionWithRetry(ctx, client, reqID, act.Seed, paths)
	if err != nil {
		if s.Change != nil && s.Change.ID == "agent-session" {
			s.Change = nil
		}
		return err
	}
	s.Change = &session
	modelsMap, confirmedProfiles := s.workflowModelsPayload()
	wfReq := map[string]any{
		"project_id":         s.Project.ProjectID,
		"project_revision":   s.Project.ProjectRevision,
		"revision":           session.Revision,
		"hash":               session.Hash,
		"message":            firstNonEmpty(act.Seed.Message, act.Message, session.Title),
		"models":             modelsMap,
		"confirmed_profiles": confirmedProfiles,
		"confirm_security":   true,
		"auto_repair":        true,
	}
	target := "/api/projects/current/changes/" + url.PathEscape(session.ID) + "/workflow"
	var wfSession ChangeSession
	if err := client.DoJSON(ctx, reqID+"-wf", http.MethodPost, target, wfReq, &wfSession); err != nil {
		s.Change.State = "failed"
		return err
	}
	s.Change = &wfSession
	return nil
}

func (s *State) openChangeSessionWithRetry(ctx context.Context, client *Client, reqID string, seed *ChangeSeed, paths []string) (ChangeSession, error) {
	buildReq := func() map[string]any {
		return map[string]any{
			"project_id":          s.Project.ProjectID,
			"project_revision":    s.Project.ProjectRevision,
			"kind":                firstNonEmpty(seed.Kind, "fix"),
			"title":               firstNonEmpty(seed.Title, "Agent Mission"),
			"paths":               paths,
			"acceptance_criteria": seed.AcceptanceCriteria,
			"finding_targets":     NormalizeFindingTargets(seed.FindingTargets, s.Project.ProjectID, paths),
		}
	}
	var session ChangeSession
	err := client.DoJSON(ctx, reqID+"-open", http.MethodPost, "/api/projects/current/changes", buildReq(), &session)
	if err != nil && isRevisionConflictError(err) {
		s.syncProjectRevision(ctx, client, reqID+"-sync")
		_ = s.ensureExecutionTrust(ctx, client, reqID+"-trust2")
		err = client.DoJSON(ctx, reqID+"-open2", http.MethodPost, "/api/projects/current/changes", buildReq(), &session)
	}
	return session, err
}

func (s *State) dispatchApplyCommit(ctx context.Context, client *Client, reqID string, act ActionRequest) error {
	if s.Change == nil || s.Project == nil {
		return nil
	}
	sessionID := url.PathEscape(s.Change.ID)
	s.ensureChangeReviewed(ctx, client, reqID, sessionID)
	applyReq := map[string]any{
		"project_id":       s.Project.ProjectID,
		"project_revision": s.Project.ProjectRevision,
		"revision":         s.Change.Revision,
		"hash":             s.Change.Hash,
		"confirm":          true,
		"commit_message":   firstNonEmpty(act.Message, s.Change.Title),
	}
	if head, branch := s.activeGitIdentity(); head != "" && branch != "" {
		applyReq["head"] = head
		applyReq["branch"] = branch
	}
	var receipt ChangeMutation
	if err := client.DoJSON(ctx, reqID, http.MethodPost, "/api/projects/current/changes/"+sessionID+"/apply", applyReq, &receipt); err != nil {
		return err
	}
	s.ChangeReceipt = &receipt
	s.Change.State = firstNonEmpty(receipt.State, "applied")
	if receipt.Commit != nil {
		s.Change.Commit = receipt.Commit
	}
	s.recordAppliedFindingCompletions(s.Change.FindingTargets, receipt.Commit)
	if receipt.ProjectRevision != "" {
		s.Project.ProjectRevision = receipt.ProjectRevision
	}
	s.CmdOutput = fmt.Sprintf("Applied and committed proposal %q", s.Change.Title)
	return nil
}

func (s *State) dispatchUpdateProposal(ctx context.Context, client *Client, reqID string, act ActionRequest) error {
	if s.Change == nil || s.Project == nil || s.Change.ID == "" {
		return nil
	}
	changes := make([]map[string]string, 0, len(s.Change.Changes))
	for _, ch := range s.Change.Changes {
		changes = append(changes, map[string]string{
			"path":    ch.Path,
			"content": ch.Content,
			"summary": ch.Summary,
		})
	}
	body := map[string]any{
		"project_id":       s.Project.ProjectID,
		"project_revision": s.Project.ProjectRevision,
		"revision":         s.Change.Revision,
		"hash":             s.Change.Hash,
		"message":          firstNonEmpty(act.Message, "Updated proposal hunks in diff editor"),
		"changes":          changes,
	}
	var updated ChangeSession
	sessionID := url.PathEscape(s.Change.ID)
	if err := client.DoJSON(ctx, reqID, http.MethodPost, "/api/projects/current/changes/"+sessionID+"/messages", body, &updated); err != nil {
		return err
	}
	if updated.ID != "" {
		s.Change = &updated
	}
	return nil
}

func (s *State) ensureChangeReviewed(ctx context.Context, client *Client, reqID, sessionID string) {
	if s.Change == nil || s.Change.Hash == "" {
		return
	}
	reviewReq := func() map[string]any {
		return map[string]any{
			"project_id":       s.Project.ProjectID,
			"project_revision": s.Project.ProjectRevision,
			"revision":         s.Change.Revision,
			"hash":             s.Change.Hash,
		}
	}
	var reviewed ChangeSession
	if err := client.DoJSON(ctx, reqID+"-rev", http.MethodPost, "/api/projects/current/changes/"+sessionID+"/review", reviewReq(), &reviewed); err == nil && reviewed.ID != "" {
		s.Change = &reviewed
		return
	}
	checkReq := reviewReq()
	checkReq["run_tests"] = true
	var checked ChangeSession
	if err := client.DoJSON(ctx, reqID+"-chk", http.MethodPost, "/api/projects/current/changes/"+sessionID+"/checks", checkReq, &checked); err == nil && checked.ID != "" {
		s.Change = &checked
	}
	if err := client.DoJSON(ctx, reqID+"-rev2", http.MethodPost, "/api/projects/current/changes/"+sessionID+"/review", reviewReq(), &reviewed); err == nil && reviewed.ID != "" {
		s.Change = &reviewed
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

func (s *State) dispatchDismissFinding(ctx context.Context, client *Client, reqID string, act ActionRequest) error {
	if act.Issue == nil || act.Issue.ID == "" {
		return nil
	}
	kind := firstNonEmpty(act.Issue.Kind, "semantic")
	body := map[string]string{
		"project_revision": s.projectRevision(),
		"status":           "dismissed",
		"kind":             kind,
		"path":             act.Issue.Path,
	}
	if kind == "security" {
		body["source"] = firstNonEmpty(act.Issue.Source, "deterministic")
	}
	target := "/api/projects/current/findings/" + url.PathEscape(act.Issue.ID)
	if err := client.DoJSON(ctx, reqID, http.MethodPatch, target, body, nil); err != nil {
		return err
	}
	s.markFindingDismissed(*act.Issue)
	return nil
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

func (s *State) dispatchDismissFeature(ctx context.Context, client *Client, reqID string, act ActionRequest) error {
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
	if err := client.DoJSON(ctx, reqID, http.MethodPatch, target, body, &report); err != nil {
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

func (s *State) dispatchSetTrust(ctx context.Context, client *Client, reqID string) error {
	body := map[string]any{
		"project_revision": s.projectRevision(),
		"confirm":          s.WorkspaceTrust,
	}
	var trust ExecutionTrust
	return client.DoJSON(ctx, reqID, http.MethodPost, "/api/projects/current/execution-trust", body, &trust)
}

func (s *State) dispatchUndo(ctx context.Context, client *Client, reqID string, act ActionRequest) error {
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
	if err := client.DoJSON(ctx, reqID, http.MethodPost, target, body, &receipt); err != nil {
		return err
	}
	s.ChangeReceipt = &receipt
	if s.Change != nil && s.Change.ID == act.Target {
		s.Change.State = firstNonEmpty(receipt.State, "reverted")
		s.removeFindingCompletions(s.Change.FindingTargets)
	}
	if receipt.ProjectRevision != "" && s.Project != nil {
		s.Project.ProjectRevision = receipt.ProjectRevision
	}
	return nil
}

func (s *State) dispatchCheckChange(ctx context.Context, client *Client, reqID string) error {
	return s.postChangeSessionAction(ctx, client, reqID, "/checks", map[string]any{
		"run_tests": true,
		"run_lint":  true,
	})
}

func (s *State) dispatchRepairChange(ctx context.Context, client *Client, reqID string, act ActionRequest) error {
	return s.postChangeSessionAction(ctx, client, reqID, "/messages", map[string]any{
		"message":                 firstNonEmpty(act.Message, "Repair the current proposal to satisfy failed checks."),
		"confirm_remote_provider": true,
		"repair":                  true,
	})
}

func (s *State) postChangeSessionAction(ctx context.Context, client *Client, reqID, suffix string, extra map[string]any) error {
	if s.Change == nil || s.Project == nil {
		return nil
	}
	_ = s.ensureExecutionTrust(ctx, client, reqID+"-trust")
	req := map[string]any{
		"project_id":       s.Project.ProjectID,
		"project_revision": s.Project.ProjectRevision,
		"revision":         s.Change.Revision,
		"hash":             s.Change.Hash,
	}
	for k, v := range extra {
		req[k] = v
	}
	var updated ChangeSession
	target := "/api/projects/current/changes/" + url.PathEscape(s.Change.ID) + suffix
	if err := client.DoJSON(ctx, reqID, http.MethodPost, target, req, &updated); err != nil {
		if isRevisionConflictError(err) {
			if seed := s.buildSeedFromCurrentChange(); seed != nil {
				s.syncProjectRevision(ctx, client, reqID+"-sync")
				s.CmdOutput = fmt.Sprintf("Proposal was stale — started fresh agent mission for %q", seed.Title)
				return s.dispatchStartAgent(ctx, client, reqID+"-refresh", ActionRequest{Kind: "start-agent", Seed: seed})
			}
		}
		return err
	}
	s.Change = &updated
	return nil
}

func (s *State) dispatchMergeWorktree(ctx context.Context, client *Client, reqID, wtPath string) error {
	if s.GitWorkspace == nil || s.Project == nil || wtPath == "" {
		return nil
	}
	_ = s.ensureExecutionTrust(ctx, client, reqID+"-trust")
	var srcHead, srcBranch string
	for _, wt := range s.GitWorkspace.Worktrees {
		if wt.Path == wtPath {
			srcHead = wt.Head
			srcBranch = wt.Branch
			break
		}
	}
	req := map[string]any{
		"project_id":        s.Project.ProjectID,
		"project_revision":  s.Project.ProjectRevision,
		"head":              s.GitWorkspace.Head,
		"branch":            s.GitWorkspace.Branch,
		"path":              wtPath,
		"source_head":       srcHead,
		"source_branch":     srcBranch,
		"confirm":           true,
		"confirm_execution": true,
	}
	var result struct {
		State      string         `json:"state"`
		Error      string         `json:"error"`
		Index      *ProjectIndex  `json:"index"`
		Repository *GitRepository `json:"repository"`
	}
	if err := client.DoJSON(ctx, reqID, http.MethodPost, "/api/projects/current/git/merge", req, &result); err != nil {
		return err
	}
	if result.Error != "" {
		return fmt.Errorf("%s", result.Error)
	}
	if result.Index != nil {
		s.Index = result.Index
		if result.Index.ProjectRevision != "" {
			s.Project.ProjectRevision = result.Index.ProjectRevision
		}
	}
	if result.Repository != nil {
		s.Repository = result.Repository
	}
	return nil
}

func (s *State) dispatchLoadChange(ctx context.Context, client *Client, reqID, sessionID string) error {
	if sessionID == "" || s.Project == nil {
		return nil
	}
	_, idQuery := s.resourceQueries()
	var chg ChangeSession
	target := "/api/projects/current/changes/" + url.PathEscape(sessionID) + idQuery
	if err := client.DoJSON(ctx, reqID, http.MethodGet, target, nil, &chg); err != nil {
		return err
	}
	if chg.ID != "" {
		s.Change = &chg
	}
	return nil
}

func (s *State) dispatchSaveSelection(ctx context.Context, client *Client, reqID string) error {
	if s.Project == nil || s.Project.ProjectID == "" {
		return nil
	}
	if s.Selection == nil || s.Selection.SelectionID == "" {
		_, idQuery := s.resourceQueries()
		var sel Selection
		if err := client.DoJSON(ctx, reqID+"-get", http.MethodGet, "/api/projects/current/analysis/selection"+idQuery, nil, &sel); err == nil {
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
	if err := client.DoJSON(ctx, reqID, http.MethodPost, "/api/projects/current/analysis/selection", req, &updated); err != nil {
		return err
	}
	s.Selection = &updated
	s.ExcludedPaths = updated.ExcludedPaths
	return nil
}

func (s *State) dispatchProposeInstructions(ctx context.Context, client *Client, reqID, path, content string) error {
	if s.Project == nil || s.Project.ProjectID == "" {
		return nil
	}
	targetPath := firstNonEmpty(path, "AGENTS.md")
	draft := firstNonEmpty(content, s.InstructionDraft)
	if draft == "" {
		return nil
	}
	req := map[string]any{
		"project_id":       s.Project.ProjectID,
		"project_revision": s.Project.ProjectRevision,
		"title":            "Update " + targetPath,
		"paths":            []string{targetPath},
		"content":          draft,
	}
	var session ChangeSession
	if err := client.DoJSON(ctx, reqID, http.MethodPost, "/api/projects/current/instructions/proposal", req, &session); err != nil {
		return err
	}
	s.Change = &session
	return nil
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

type pollTickMsg time.Time

func pollTickCmd() tea.Cmd {
	return tea.Tick(750*time.Millisecond, func(t time.Time) tea.Msg {
		return pollTickMsg(t)
	})
}

// BubbleModel adapts State to the Charmbracelet Bubble Tea tea.Model architecture.
type BubbleModel struct {
	ctx    context.Context
	state  *State
	client *Client
	width  int
	height int
}

// NewBubbleModel constructs a Bubble Tea model wrapping State and Client.
func NewBubbleModel(ctx context.Context, state *State, client *Client, width, height int) *BubbleModel {
	w, h := DetectTerminalSize(width, height)
	return &BubbleModel{
		ctx:    ctx,
		state:  state,
		client: client,
		width:  w,
		height: h,
	}
}

// Init implements tea.Model.
func (m *BubbleModel) Init() tea.Cmd {
	if m.client != nil && m.state.hasActiveBackgroundJob() {
		return pollTickCmd()
	}
	return nil
}

// Update implements tea.Model, handling tea.WindowSizeMsg, tea.KeyMsg, KeyEvent, and poll ticks.
func (m *BubbleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch ev := msg.(type) {
	case tea.WindowSizeMsg:
		if ev.Width > 0 && ev.Height > 0 {
			m.width = ev.Width
			m.height = ev.Height
		}
	case tea.KeyMsg:
		m.applyKeyEvent(keyEventFromTea(ev))
	case KeyEvent:
		m.applyKeyEvent(ev)
	case pollTickMsg:
		m.state.PollActiveJobs(m.ctx, m.client)
	}
	if m.state.Quitting {
		return m, tea.Quit
	}
	if m.client != nil && m.state.hasActiveBackgroundJob() {
		return m, pollTickCmd()
	}
	return m, nil
}

func (m *BubbleModel) applyKeyEvent(ev KeyEvent) {
	m.state.HandleKey(ev)
	m.state.FlushActions(m.ctx, m.client)
}

// View implements tea.Model, rendering the full-screen Lip Gloss / Bubbles frame.
func (m *BubbleModel) View() string {
	return m.state.Render(m.width, m.height)
}

func keyEventFromTea(msg tea.KeyMsg) KeyEvent {
	switch msg.Type {
	case tea.KeyUp:
		return KeyEvent{Key: "ArrowUp"}
	case tea.KeyDown:
		return KeyEvent{Key: "ArrowDown"}
	case tea.KeyLeft:
		return KeyEvent{Key: "ArrowLeft"}
	case tea.KeyRight:
		return KeyEvent{Key: "ArrowRight"}
	case tea.KeyTab:
		return KeyEvent{Key: "Tab"}
	case tea.KeyShiftTab:
		return KeyEvent{Key: "Tab", Shift: true}
	case tea.KeyEnter:
		return KeyEvent{Key: "Enter"}
	case tea.KeyEsc:
		return KeyEvent{Key: "Escape"}
	case tea.KeyBackspace:
		return KeyEvent{Key: "Backspace"}
	case tea.KeySpace:
		return KeyEvent{Key: " "}
	case tea.KeyCtrlC:
		return KeyEvent{Key: "c", Ctrl: true}
	default:
		return KeyEvent{Key: msg.String()}
	}
}

// RunSession runs an interactive terminal session using Bubble Tea on TTYs or deterministic model updates on streams.
func (s *State) RunSession(ctx context.Context, client *Client, in io.Reader, out io.Writer, width, height int) error {
	model := NewBubbleModel(ctx, s, client, width, height)
	_ = model.Init()
	if _, isTTY := isCharDevice(in); isTTY {
		prog := tea.NewProgram(model, tea.WithInput(in), tea.WithOutput(out), tea.WithAltScreen(), tea.WithContext(ctx))
		_, err := prog.Run()
		return err
	}
	if _, err := fmt.Fprintf(out, "\033[2J\033[H%s\n", model.View()); err != nil {
		return fmt.Errorf("render initial frame: %w", err)
	}
	buf := make([]byte, 64)
	for !s.Quitting {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		n, err := in.Read(buf)
		if n > 0 {
			for _, ev := range DecodeInputBytes(buf[:n]) {
				_, _ = model.Update(ev)
				if s.Quitting {
					break
				}
			}
			w, h := DetectTerminalSize(width, height)
			_, _ = model.Update(tea.WindowSizeMsg{Width: w, Height: h})
			if _, wErr := fmt.Fprintf(out, "\033[2J\033[H%s\n", model.View()); wErr != nil {
				return fmt.Errorf("render frame: %w", wErr)
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read terminal input: %w", err)
		}
	}
	return nil
}
