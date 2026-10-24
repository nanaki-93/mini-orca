package tui

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

func missionResource(scope client.AgentSettingsScope, resource string) string {
	query := url.Values{"project_id": {scope.ProjectID}, "project_revision": {scope.ProjectRevision}}
	if scope.WorktreePath != "" {
		query.Set("worktree_path", scope.WorktreePath)
	}
	return "/api/projects/current" + resource + "?" + query.Encode()
}

func executeMissionExecutionAction(ctx context.Context, cl *Client, action missionAction) missionResult {
	r := missionResult{Action: action}
	switch action.Operation {
	case "execution-read":
		r.Err = readMissionExecution(ctx, cl, &r)
	case "proposal", "candidate-read", "candidate-history":
		r.Err = readMissionCandidate(ctx, cl, &r)
	case "execution-control", "mission-trust":
		r.Err = validateMissionExecutionDispatch(ctx, cl, action)
		if r.Err == nil && action.Operation == "mission-trust" {
			r.Err = grantMissionTrust(ctx, cl, &r)
		} else if r.Err == nil {
			r.Execution, r.Err = cl.ControlMissionExecution(ctx, action.ID, action.Scope, action.MissionID, action.Execution)
		}
	default:
		r.Err = fmt.Errorf("unknown mission operation")
	}
	return r
}

func (s *State) applyMissionExecutionResult(result missionResult) bool {
	v := s.Mission
	switch result.Action.Operation {
	case "execution-read":
		if err := s.applyMissionExecutionRead(result); err != nil {
			v.Failure, s.Error = err.Error(), err.Error()
		}
	case "proposal", "candidate-read", "candidate-history":
		v.Candidate, v.CandidateGit = result.Candidate, result.CandidateGit
		if result.Action.Operation == "proposal" && result.Candidate != nil {
			s.Change = result.Candidate
			s.upsertActiveChange(s.Change)
			s.File, s.BrowseFiles = nil, false
			s.DiffSinceReview, s.DiffOffset, s.DetailScroll = false, 0, 0
			s.Navigate(PageEditor)
			s.syncDiffSelectionToHunk(0)
			s.CmdOutput = "Mission candidate · review each changed file, then explicitly Apply/commit. :mission-execution returns to the owner. Historical candidates cannot replace the selected generation."
		} else if result.Action.Operation == "candidate-history" && result.Candidate != nil {
			s.Change = result.Candidate
			s.upsertActiveChange(s.Change)
			s.Navigate(PageAgents)
			s.syncSelectedIdxToCurrentChange()
			s.AgentDetailOpen = true
			s.enqueueAgentInspection()
			s.CmdOutput = "Historical model execution for the original candidate. This history grants no current check, review or Apply authority."
		} else {
			s.CmdOutput = "Committed candidate inspected. :mission-integrate confirms its approved destination and order."
		}
	case "mission-trust":
		v.Trust = result.Trust
		s.CmdOutput = "Check commands trusted for the displayed mission checkout. Start or revalidation remains a separate action."
	case "execution-control":
		if result.Execution == nil {
			v.Failure = "Execution receipt unavailable; refresh before retrying."
			return true
		}
		if err := s.applyMissionGitReceipt(result); err != nil {
			v.Failure, s.Error = err.Error(), err.Error()
			return true
		}
		v.Execution, v.ProgressFailure = result.Execution, ""
		s.invalidateMissionCandidate()
		s.CmdOutput = "Mission " + result.Execution.Execution.State + ". Inspect children and recovery before the next action."
		if git := result.Execution.Git; git != nil && (git.Error != "" || len(git.Warnings) > 0) {
			s.CmdOutput = externalSafeText(strings.Join(append([]string{git.State, git.Error}, git.Warnings...), " · "))
		}
	default:
		return false
	}
	return true
}

func (s *State) applyMissionExecutionRead(result missionResult) error {
	v := s.Mission
	if result.Owner == nil || result.Index == nil || result.Repository == nil || result.Trust == nil || result.Settings == nil || result.Detail == nil {
		return fmt.Errorf("mission inspection returned incomplete owner authority")
	}
	if v.Execution != nil && (result.Execution == nil || result.Execution.Execution.ID != v.Execution.Execution.ID || result.Execution.Execution.Revision < v.Execution.Execution.Revision) {
		return fmt.Errorf("mission execution disappeared or moved backwards; retained receipt preserved")
	}
	if err := s.applyMissionOwnerSnapshot(result.Action, result.Index, result.Repository); err != nil {
		return err
	}
	v.Saved, v.Settings, v.Trust = result.Detail, result.Settings, result.Trust
	v.Execution, v.ExecutionLoaded, v.ExecutionOpen = result.Execution, true, true
	v.Candidate, v.CandidateGit, v.ProgressFailure = nil, nil, ""
	s.updateMissionSummary(result.Detail)
	s.CmdOutput = "Execution inspected locally. Preparation, start, checks and integration each require an explicit action."
	return nil
}

func (s *State) applyMissionOwnerSnapshot(action missionAction, index *ProjectIndex, repository *GitRepository) error {
	v := s.Mission
	if s.Project == nil || s.Project.ProjectID != action.Parent.ProjectID || index.ProjectID != action.Scope.ProjectID || repository.ProjectIdentity != index.ProjectIdentity || repository.Root != action.Path {
		return fmt.Errorf("mission result belongs to a replaced checkout")
	}
	if action.Scope.WorktreePath == "" {
		if s.Project.Path != action.Path || s.Project.ProjectIdentity != action.Parent && s.Project.ProjectIdentity != index.ProjectIdentity {
			return fmt.Errorf("project advanced again before the mission receipt arrived; refresh")
		}
		s.applyGitMutationSnapshot(mergeWorktreeResponse{Index: index, Repository: repository})
	} else if s.GitWorkspace != nil {
		for i := range s.GitWorkspace.Worktrees {
			worktree := &s.GitWorkspace.Worktrees[i]
			if worktree.Path == action.Path {
				worktree.GitIdentity, worktree.Dirty = repository.GitIdentity, repository.Dirty
			}
		}
	}
	v.Parent, v.Scope.ProjectIdentity = s.Project.ProjectIdentity, index.ProjectIdentity
	v.Index, v.Repository = index, repository
	return nil
}

func (s *State) applyMissionGitReceipt(result missionResult) error {
	git := result.Execution.Git
	if git == nil {
		return nil
	}
	if err := validateMissionCleanupReceipt(result.Action, git); err != nil {
		return err
	}
	v := s.Mission
	if git.Index != nil {
		if git.Repository == nil {
			return fmt.Errorf("integration index has no checkout identity")
		}
		if err := s.applyMissionOwnerSnapshot(result.Action, git.Index, git.Repository); err != nil {
			return err
		}
		// A new base needs a fresh command/settings inspection; it grants no trust.
		v.Trust, v.ExecutionLoaded = nil, false
	}
	if git.RemovedWorktree != "" {
		s.removeWorktreeByPath(git.RemovedWorktree)
		s.forgetDiscardedWorktree(git.RemovedWorktree)
		if v.Scope.WorktreePath != "" {
			s.Change = v.OwnerChange
		}
	}
	if git.RemovedBranch != "" && s.GitWorkspace != nil {
		s.GitWorkspace.Branches = slices.DeleteFunc(s.GitWorkspace.Branches, func(branch client.GitBranch) bool { return branch.Name == git.RemovedBranch })
	}
	return nil
}

func validateMissionCleanupReceipt(action missionAction, git *client.GitMutationResult) error {
	input := action.Execution.Integration
	if input == nil || git.RemovedWorktree != "" && git.RemovedWorktree != input.Path || git.RemovedBranch != "" && git.RemovedBranch != input.Source.Branch {
		return fmt.Errorf("cleanup receipt names an unselected checkout or branch")
	}
	return nil
}

func (s *State) invalidateMissionCandidate() {
	v := s.Mission
	child := s.selectedMissionExecutionChild()
	if child == nil || !missionCandidateMatches(v.Execution.Execution, *child, v.Candidate) {
		v.Candidate, v.CandidateGit = nil, nil
	}
}

func (s *State) returnCommittedMission(chg *ChangeSession) bool {
	if chg.MissionOrigin == nil {
		return false
	}
	// Keep the actual committed session selected after ordinary history archival.
	s.Change = chg
	if s.Mission != nil && s.Mission.Execution != nil {
		for _, child := range s.Mission.Execution.Execution.Children {
			if missionCandidateMatches(s.Mission.Execution.Execution, child, chg) {
				s.Mission.ChildID, s.Mission.History, s.Mission.ExecutionOpen = child.ID, false, true
				s.PlanView = "mission"
				s.Navigate(PagePlan)
				s.queueMissionAction(missionAction{Operation: "candidate-read", MissionID: chg.MissionOrigin.MissionID, Candidate: &child})
				return true
			}
		}
	}
	s.CmdOutput = "Mission candidate committed. Open its owner in :missions, inspect :mission-execution, then integrate in its approved order."
	return true
}

func (s *State) routeMissionWorktree(path string) bool {
	if s.Change != nil && s.Change.WorktreePath == path && s.Change.MissionOrigin != nil {
		if s.Mission != nil && s.Mission.Execution != nil && s.Mission.Execution.Execution.ID == s.Change.MissionOrigin.ExecutionID {
			s.Mission.ChildID = s.Change.MissionOrigin.ChildID
			s.PlanView, s.Mission.History, s.Mission.ExecutionOpen = "mission", false, true
			s.Navigate(PagePlan)
			s.confirmMissionExecution("integrate")
		} else {
			s.CmdOutput = "Open this mission's owner and inspect :mission-execution before integration."
		}
		return true
	}
	if s.Mission == nil || s.Mission.Execution == nil {
		return s.retainedMissionCheckoutHint(path)
	}
	for _, child := range missionAllCandidates(s.Mission.Execution.Execution) {
		if child.Directory != "" && path == missionCandidatePath(s.Mission.Path, child) {
			s.Mission.ChildID, s.Mission.History, s.Mission.ExecutionOpen = child.ID, false, true
			s.PlanView = "mission"
			s.Navigate(PagePlan)
			s.CmdOutput = "Owned mission checkout. Inspect its current proposal before :mission-integrate; historical candidates remain retained."
			return true
		}
	}
	return false
}

func (s *State) retainedMissionCheckoutHint(path string) bool {
	if s.GitWorkspace == nil {
		return false
	}
	for _, worktree := range s.GitWorkspace.Worktrees {
		if worktree.Path == path && (strings.HasPrefix(worktree.Branch, "codex/mission-run-") || strings.HasPrefix(filepath.Base(path), "mission-run-")) {
			s.CmdOutput = "This checkout uses mission ownership. Inspect its task, then open the owning mission with :missions and :mission-execution before integration."
			return true
		}
	}
	return false
}

func readMissionExecution(ctx context.Context, cl *Client, r *missionResult) error {
	action := r.Action
	// Explicit recovery reads the same owner's actual revision. It never adopts
	// a new checkout, executes checks, or rewrites the retained local draft.
	query := url.Values{"project_id": {action.Scope.ProjectID}}
	if action.Scope.WorktreePath != "" {
		query.Set("worktree_path", action.Scope.WorktreePath)
	}
	var owner Project
	if err := cl.DoJSON(ctx, action.ID+"-owner", http.MethodGet, "/api/projects/current?"+query.Encode(), nil, &owner); err != nil {
		return err
	}
	if owner.ProjectID != action.Scope.ProjectID || owner.ProjectRevision == "" || owner.Path != action.Path {
		return fmt.Errorf("mission owner changed; select its checkout again")
	}
	scope := action.Scope
	scope.ProjectIdentity = owner.ProjectIdentity
	r.Owner = &owner
	var err error
	if r.Detail, err = cl.MissionDetail(ctx, action.ID+"-plan", scope, action.MissionID); err != nil {
		return err
	}
	if r.Execution, err = cl.MissionExecution(ctx, action.ID+"-execution", scope, action.MissionID); err != nil {
		return err
	}
	if r.Settings, err = cl.ReadAgentSettings(ctx, action.ID+"-settings", scope); err != nil {
		return err
	}
	if r.Index, err = cl.ExternalIndex(ctx, action.ID+"-index", scope); err != nil {
		return err
	}
	r.Repository, r.Trust, err = readMissionAuthority(ctx, cl, action.ID, scope)
	return err
}

func readMissionAuthority(ctx context.Context, cl *Client, id string, scope client.AgentSettingsScope) (*GitRepository, *ExecutionTrust, error) {
	var repository GitRepository
	if err := cl.DoJSON(ctx, id+"-git", http.MethodGet, missionResource(scope, "/git/repository"), nil, &repository); err != nil {
		return nil, nil, err
	}
	var trust ExecutionTrust
	if err := cl.DoJSON(ctx, id+"-trust", http.MethodGet, missionResource(scope, "/execution-trust"), nil, &trust); err != nil {
		return nil, nil, err
	}
	if repository.ProjectIdentity != scope.ProjectIdentity || trust.ProjectIdentity != scope.ProjectIdentity {
		return nil, nil, fmt.Errorf("mission check or Git authority belongs to another checkout")
	}
	return &repository, &trust, nil
}

func validateMissionExecutionDispatch(ctx context.Context, cl *Client, action missionAction) error {
	if missionRecoveryControl(action.Execution.Action) {
		return validateMissionExecutionSnapshot(ctx, cl, action)
	}
	settings, err := cl.ReadAgentSettings(ctx, action.ID+"-settings", action.Scope)
	if err != nil {
		return err
	}
	if missionDigest(settings) != action.SettingsHash {
		return fmt.Errorf("mission provider or settings changed; refresh and confirm again")
	}
	if err := validateMissionExecutionSnapshot(ctx, cl, action); err != nil {
		return err
	}
	repository, trust, err := readMissionAuthority(ctx, cl, action.ID, action.Scope)
	if err != nil {
		return err
	}
	if missionDigest(trust) != action.TrustHash || action.Execution.Base != nil && repository.GitIdentity != *action.Execution.Base {
		return fmt.Errorf("mission Git base or check commands changed; inspect and confirm again")
	}
	index, err := cl.ExternalIndex(ctx, action.ID+"-index", action.Scope)
	if err != nil {
		return err
	}
	for _, source := range action.Sources {
		if source != missionSource(index, source.Path) {
			return fmt.Errorf("mission source changed after confirmation")
		}
	}
	return validateMissionCandidateDispatch(ctx, cl, action)
}

func validateMissionCandidateDispatch(ctx context.Context, cl *Client, action missionAction) error {
	if action.Execution.Action != "integrate" && action.Execution.Action != "revalidate" {
		return context.Cause(ctx)
	}
	input := action.Execution.Integration
	if action.Execution.Revalidation != nil {
		input = &action.Execution.Revalidation.Previous
	}
	if action.Captured == nil || input == nil {
		return fmt.Errorf("mission candidate admission is missing")
	}
	for _, child := range action.Captured.Execution.Children {
		if child.ID != input.ChildID || child.SessionID != input.SessionID {
			continue
		}
		action.Candidate = &child
		result := missionResult{Action: action}
		if err := readMissionCandidate(ctx, cl, &result); err != nil {
			return err
		}
		if result.CandidateGit.GitIdentity != input.Source || !missionCleanRepository(result.CandidateGit) {
			return fmt.Errorf("candidate source changed after confirmation; inspect and confirm again")
		}
		return context.Cause(ctx)
	}
	return fmt.Errorf("the selected candidate is no longer owned by this execution")
}

func validateMissionExecutionSnapshot(ctx context.Context, cl *Client, action missionAction) error {
	current, err := cl.MissionExecution(ctx, action.ID+"-admission", action.Scope, action.MissionID)
	if err != nil {
		return err
	}
	if action.Captured == nil {
		if current != nil {
			return fmt.Errorf("mission execution already exists; refresh its retained allocation")
		}
		detail, err := cl.MissionDetail(ctx, action.ID+"-plan", action.Scope, action.MissionID)
		if err != nil {
			return err
		}
		if detail.Stale || !detail.Mission.Approved || detail.Mission.PlanHash != action.Execution.PlanHash || detail.CatalogRevision != action.Execution.CatalogRevision {
			return fmt.Errorf("mission approval or catalog changed after confirmation")
		}
		return nil
	}
	if current == nil || missionExecutionSnapshot(current) != missionExecutionSnapshot(action.Captured) {
		return fmt.Errorf("mission execution or ownership changed; refresh and confirm its current state")
	}
	return nil
}

func missionExecutionSnapshot(value *client.AgentMissionExecution) string {
	return missionDigest([]any{value.Execution, value.Stale, value.SourceStatus, value.ReadyChildren, value.OwnerState})
}

func grantMissionTrust(ctx context.Context, cl *Client, r *missionResult) error {
	request := struct {
		ProjectRevision string `json:"project_revision"`
		Confirm         bool   `json:"confirm"`
	}{r.Action.Scope.ProjectRevision, true}
	var trust ExecutionTrust
	if err := cl.DoJSON(ctx, r.Action.ID, http.MethodPost, missionResource(r.Action.Scope, "/execution-trust"), request, &trust); err != nil {
		return err
	}
	if trust.ProjectIdentity != r.Action.Scope.ProjectIdentity || !trust.Trusted {
		return fmt.Errorf("check trust was not recorded for this mission owner")
	}
	r.Trust = &trust
	return nil
}

func missionCandidatePath(root string, child client.MissionExecutionChild) string {
	return filepath.Join(root, ".worktrees", child.Directory)
}

func readMissionCandidate(ctx context.Context, cl *Client, r *missionResult) error {
	action := r.Action
	if action.Captured == nil || action.Candidate == nil || action.Candidate.SessionID == "" {
		return fmt.Errorf("select a retained child proposal first")
	}
	current, err := cl.MissionExecution(ctx, action.ID+"-execution", action.Scope, action.MissionID)
	if err != nil {
		return err
	}
	if current == nil || missionDigest(current.Execution) != missionDigest(action.Captured.Execution) {
		return fmt.Errorf("mission candidate changed; refresh before opening its proposal")
	}
	child := *action.Candidate
	scope := client.AgentSettingsScope{ProjectIdentity: ProjectIdentity{ProjectID: child.ProjectID, ProjectRevision: child.ProjectRevision}, WorktreePath: missionCandidatePath(action.Path, child)}
	var candidate ChangeSession
	if err := cl.DoJSON(ctx, action.ID+"-candidate", http.MethodGet, missionResource(scope, "/changes/"+url.PathEscape(child.SessionID)), nil, &candidate); err != nil {
		return err
	}
	if !missionCandidateMatches(current.Execution, child, &candidate) || candidate.WorktreePath != "" && candidate.WorktreePath != scope.WorktreePath {
		return fmt.Errorf("child proposal does not match the selected mission candidate")
	}
	candidate.WorktreePath = scope.WorktreePath
	var repository GitRepository
	if err := cl.DoJSON(ctx, action.ID+"-candidate-git", http.MethodGet, missionResource(scope, "/git/repository"), nil, &repository); err != nil {
		return err
	}
	if repository.ProjectIdentity != scope.ProjectIdentity || repository.Root != scope.WorktreePath || repository.Branch != child.Branch {
		return fmt.Errorf("child checkout identity changed")
	}
	r.Candidate, r.CandidateGit = &candidate, &repository
	return nil
}

func missionCandidateMatches(record client.MissionExecution, child client.MissionExecutionChild, proposal *ChangeSession) bool {
	if !missionProposalIdentityMatches(child, proposal) {
		return false
	}
	origin := client.MissionChildOrigin{MissionID: record.Mission.ID, ExecutionID: record.ID, ChildID: child.ID, PlanHash: record.Mission.PlanHash}
	if child.Generation > 0 {
		for _, entry := range record.Revalidations {
			if entry.Candidate.ID == child.ID && entry.Candidate.Generation == child.Generation {
				origin.RevalidationID = entry.ID
			}
		}
		return origin.RevalidationID != "" && *proposal.MissionOrigin == origin && proposal.Workflow == nil && child.WorkflowID == ""
	}
	return *proposal.MissionOrigin == origin && proposal.Workflow != nil && proposal.Workflow.ID == child.WorkflowID
}

func missionProposalIdentityMatches(child client.MissionExecutionChild, proposal *ChangeSession) bool {
	return proposal != nil && proposal.MissionOrigin != nil && proposal.ID == child.SessionID && proposal.ProjectID == child.ProjectID && proposal.ProjectRevision == child.ProjectRevision && proposal.Hash == child.ProposalHash && int64(proposal.Revision) == child.ProposalRevision
}
