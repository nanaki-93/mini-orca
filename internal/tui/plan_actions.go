package tui

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
)

func planHash(session *ChangeSession) string {
	if session.Plan == nil {
		return ""
	}
	return session.Plan.Hash
}

func updateSavedPlanTask(ctx context.Context, cl *Client, task *startAgentTask, scope string) (ChangeSession, error) {
	old := task.act.ExistingChange
	body := map[string]any{"project_id": old.ProjectID, "project_revision": old.ProjectRevision, "revision": old.Revision, "hash": old.Hash,
		"paths": task.paths, "acceptance_criteria": task.act.Seed.AcceptanceCriteria, "plan": task.act.Seed.Plan}
	var updated ChangeSession
	err := cl.DoJSON(ctx, task.reqID+"-plan", http.MethodPost, "/api/projects/current/changes/"+url.PathEscape(old.ID)+"/plan"+scope, body, &updated)
	updated.WorktreePath = old.WorktreePath
	return updated, err
}

func resumableWorkflowStage(session *ChangeSession) string {
	if session.Workflow == nil || len(session.Workflow.Stages) < 3 {
		return "create"
	}
	w := session.Workflow
	passed, total := domain.CountPassedChecks(session.Checks)
	if w.Stages[1].Status == "completed" && total > 0 && passed == total && w.Review == nil {
		return "review"
	}
	if w.Stages[0].Status == "completed" && len(session.Changes) > 0 && len(session.Checks) == 0 {
		return "test"
	}
	return "create"
}

func allProposalFilesReviewed(session *ChangeSession) bool {
	if session.FileReviewHash != session.Hash || len(session.Changes) == 0 {
		return false
	}
	for _, file := range domain.EffectiveSessionChanges(session) {
		if !domain.ContainsString(session.ReviewedFiles, file.Path) {
			return false
		}
	}
	return true
}

func (s *State) reviewCurrentDiffFile() {
	active, _, ok := s.activeDiffHunk()
	if !ok || s.Change == nil {
		return
	}
	if s.DiffSinceReview {
		s.CmdOutput = "Switch to the complete proposal with [z] before marking a file reviewed."
		return
	}
	snapshot := cloneChangeSessionSnapshot(s.Change)
	s.enqueueAction(ActionRequest{Kind: "review-file", Target: active.Path, ExistingChange: &snapshot})
}

func (s *State) dispatchReviewFile(ctx context.Context, cl *Client, reqID string, act ActionRequest) error {
	old := act.ExistingChange
	if old == nil {
		return fmt.Errorf("no proposal selected")
	}
	body := map[string]any{"project_id": old.ProjectID, "project_revision": old.ProjectRevision, "revision": old.Revision, "hash": old.Hash, "path": act.Target}
	var result ChangeSession
	err := cl.DoJSON(ctx, reqID, http.MethodPost, "/api/projects/current/changes/"+url.PathEscape(old.ID)+"/review-file"+changeWorktreeScopeQuery(old), body, &result)
	if err != nil {
		return err
	}
	if s.Change == nil || s.Change.ID != old.ID || s.Change.Hash != old.Hash || s.Change.ProjectIdentity != old.ProjectIdentity || s.Change.Revision != old.Revision {
		return nil
	}
	result.WorktreePath = old.WorktreePath
	s.Change = &result
	s.upsertActiveChange(&result)
	s.CmdOutput = fmt.Sprintf("Reviewed %s · %d/%d files. [f] Next file · [a] Apply when all files are reviewed", act.Target, len(result.ReviewedFiles), len(domain.EffectiveSessionChanges(&result)))
	return nil
}

func (s *State) finishPlanSubmission(res startAgentResult) {
	if s.PlanDraft != nil && s.PlanDraft.PreviewID == res.task.act.PreviewID {
		s.PlanDraft.Submitted = false
		if s.acceptPlanResult(res) && res.err == nil && res.task.act.Kind == "save-plan" {
			saved := res.wfSession
			s.PlanDraft.Saved = &saved
			s.PlanDraft.Seed.WorktreePath = saved.WorktreePath
			s.CmdOutput = "Draft saved. Edit locally or [g] approve and generate."
		}
	}
}

func (s *State) matchesPlanSource(source ProjectIdentity) bool {
	return source.ProjectID == "" || source.ProjectID == s.projectID() && source.ProjectRevision == s.projectRevision()
}

func (s *State) acceptPlanResult(res startAgentResult) bool {
	if s.projectID() != res.task.projectID {
		return false
	}
	if res.task.act.Seed == nil || res.task.act.Seed.Plan == nil {
		return true
	}
	return s.projectRevision() == res.task.projectRevision
}

func (s *State) rejectPlanSubmission(action ActionRequest, reason string) {
	if s.PlanDraft != nil && s.PlanDraft.PreviewID == action.PreviewID {
		s.PlanDraft.Submitted = false
	}
	s.dropPreviewChangeOnError(action.PreviewID)
	s.Error = reason
}
