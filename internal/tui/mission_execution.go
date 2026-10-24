package tui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

func (s *State) execMissionExecutionCommand(command string) bool {
	switch command {
	case "mission-execution":
		s.refreshMissionExecution()
	case "mission-proposal":
		s.openMissionProposal()
	case "mission-prepare", "mission-start", "mission-pause", "mission-stop", "mission-reconcile", "mission-integrate", "mission-revalidate", "mission-recover-integration", "mission-recover-revalidation", "mission-cleanup", "mission-trust":
		action := strings.TrimPrefix(command, "mission-")
		switch action {
		case "stop":
			action = "cancel"
		case "recover-integration":
			action = "reconcile_integration"
		case "recover-revalidation":
			action = "reconcile_revalidation"
		case "cleanup":
			action = "cleanup_integration"
		}
		s.confirmMissionExecution(action)
	default:
		return false
	}
	return true
}

func (s *State) refreshMissionExecution() {
	if !s.missionCurrent() || s.Mission.Saved == nil {
		s.Error = "Open a saved mission before inspecting execution."
		return
	}
	s.Mission.ExecutionOpen, s.Mission.History = true, false
	s.PlanView = "mission"
	s.Navigate(PagePlan)
	s.queueMissionAction(missionAction{Operation: "execution-read", MissionID: s.Mission.Saved.Mission.ID})
}

func (s *State) missionExecutionReady() error {
	if err := s.missionExecutionInspectionReady(); err != nil {
		return err
	}
	v := s.Mission
	if missionDigest(v.Draft) != missionDigest(v.Saved.Mission.Plan) || !v.Saved.Mission.Approved {
		return fmt.Errorf("save and approve the exact mission plan first")
	}
	if v.Execution != nil && v.Execution.Execution.Mission.PlanHash != v.Saved.Mission.PlanHash {
		return fmt.Errorf("the stored execution belongs to another plan; create a new reviewed mission")
	}
	return nil
}

func (s *State) missionExecutionInspectionReady() error {
	if !s.missionCurrent() || s.Mission.Pending != nil || s.Mission.Failure != "" {
		return fmt.Errorf("refresh the current mission and finish its pending request first")
	}
	v := s.Mission
	if !v.ExecutionLoaded || v.Saved == nil || v.Repository == nil || v.Trust == nil || v.Settings == nil || v.Parent != s.Project.ProjectIdentity {
		return fmt.Errorf("inspect :mission-execution before controlling the retained mission")
	}
	return nil
}

func (s *State) confirmMissionExecution(operation string) {
	ready := s.missionExecutionReady
	if missionRecoveryControl(operation) {
		ready = s.missionExecutionInspectionReady
	}
	if err := ready(); err != nil {
		s.Error = err.Error()
		return
	}
	action, err := s.missionExecutionAction(operation)
	if err != nil {
		s.Error = err.Error()
		return
	}
	title, message := missionExecutionDisclosure(s.Mission, action)
	s.confirmMissionAction(title, message, action)
}

func missionRecoveryControl(operation string) bool {
	return slices.Contains([]string{"pause", "cancel", "reconcile", "reconcile_integration", "reconcile_revalidation", "cleanup_integration"}, operation)
}

func (s *State) missionExecutionAction(operation string) (missionAction, error) {
	v := s.Mission
	request := client.MissionExecutionRequest{ProjectIdentity: v.Scope.ProjectIdentity, Action: operation, PlanHash: v.Saved.Mission.PlanHash, Confirm: true}
	action := missionAction{Operation: "execution-control", MissionID: v.Saved.Mission.ID, Sources: slices.Clone(v.Saved.Mission.Plan.Sources)}
	if v.Execution == nil {
		if operation != "prepare" && operation != "trust" {
			return action, fmt.Errorf("prepare the approved mission before starting children")
		}
		if v.Saved.Stale || v.Settings.State == "stale" || !missionCleanRepository(v.Repository) {
			return action, fmt.Errorf("preparation requires current captured sources and a clean committed checkout")
		}
		base := v.Repository.GitIdentity
		request.CatalogRevision, request.Base = v.Saved.CatalogRevision, &base
	} else {
		record := v.Execution.Execution
		request.ExecutionID, request.ExecutionRevision = record.ID, record.Revision
		request.PlanHash = record.Mission.PlanHash
		action.Sources = slices.Clone(record.Current.Sources)
		if err := s.configureMissionControl(&request); err != nil {
			return action, err
		}
	}
	if operation == "trust" {
		if v.Trust.Trusted {
			return action, fmt.Errorf("the displayed checkout commands are already trusted")
		}
		action.Operation = "mission-trust"
	}
	action.Execution = request
	return action, nil
}

func (s *State) configureMissionControl(request *client.MissionExecutionRequest) error {
	v, record := s.Mission, s.Mission.Execution.Execution
	if request.Action == "prepare" {
		return fmt.Errorf("execution is already prepared; its allocations cannot be reset")
	}
	switch request.Action {
	case "start":
		return s.configureMissionStart(request)
	case "pause":
		if record.State != "running" {
			return fmt.Errorf("only a running mission wave can be paused")
		}
	case "cancel":
		if slices.Contains([]string{"canceled", "canceling", "completed"}, record.State) {
			return fmt.Errorf("this mission has no cancelable execution")
		}
	case "reconcile":
		if record.Wave == nil || v.Execution.OwnerState != "recovery_required" {
			return fmt.Errorf("wave recovery requires an interrupted or expired owner")
		}
	case "integrate", "revalidate":
		return s.configureMissionCandidateAction(request)
	case "reconcile_integration", "cleanup_integration", "reconcile_revalidation":
		return s.configureMissionRecovery(request)
	case "trust":
		if v.Execution.Stale || v.Settings.State == "stale" {
			return fmt.Errorf("refresh the mission's current base before granting check trust")
		}
	default:
		return fmt.Errorf("unknown mission execution control")
	}
	return nil
}

func (s *State) configureMissionStart(request *client.MissionExecutionRequest) error {
	v := s.Mission
	if v.Execution.Stale || v.Settings.State == "stale" || v.Execution.OwnerState != "idle" || len(v.Execution.ReadyChildren) == 0 || !v.Trust.Trusted {
		return fmt.Errorf("start requires current scope, ready unstarted children and explicit check trust; inspect recovery or :mission-trust")
	}
	models, err := missionExecutionModels(v.Settings)
	if err != nil {
		return err
	}
	for _, model := range models {
		profile := client.FirstNonEmpty(model.Profile, model.Scope)
		if model.RemoteProvider && !slices.Contains(request.ConfirmedProfiles, profile) {
			request.ConfirmedProfiles = append(request.ConfirmedProfiles, profile)
		}
	}
	request.ConfirmExecution = true
	return nil
}

func missionExecutionModels(settings *client.AgentSettings) ([]client.Model, error) {
	var models []client.Model
	for _, profile := range []string{settings.Roles.Creation, settings.Roles.Testing, settings.Roles.Review} {
		index := slices.IndexFunc(settings.Models, func(model client.Model) bool { return client.FirstNonEmpty(model.Profile, model.Scope) == profile })
		if index < 0 || !settings.Models[index].Capabilities.Completion {
			return nil, fmt.Errorf("inspect the configured creation, testing and review models before starting")
		}
		models = append(models, settings.Models[index])
	}
	return models, nil
}

func (s *State) selectedMissionExecutionChild() *client.MissionExecutionChild {
	if s.Mission == nil || s.Mission.Execution == nil {
		return nil
	}
	for _, child := range s.Mission.Execution.Execution.Children {
		if child.ID == s.Mission.ChildID {
			return &child
		}
	}
	return nil
}

func missionCleanRepository(repository *GitRepository) bool {
	return repository != nil && repository.Available && repository.AtRoot && repository.Head != "" && repository.Branch != "" && !repository.Dirty && repository.MergeHead == "" && repository.Operation == "" && len(repository.Conflicts) == 0
}

func (s *State) configureMissionCandidateAction(request *client.MissionExecutionRequest) error {
	v, child := s.Mission, s.selectedMissionExecutionChild()
	if child == nil || child.State != "awaiting_review" || v.Execution.Stale || v.Settings.State == "stale" || v.Execution.OwnerState != "idle" || !missionCandidateMatches(v.Execution.Execution, *child, v.Candidate) || !missionCleanRepository(v.CandidateGit) {
		return fmt.Errorf("open the selected current candidate with :mission-proposal before confirming its next action")
	}
	record := v.Execution.Execution
	base := record.Current.Base
	request.Base, request.ConfirmExecution = &base, true
	input := missionProposalInput(v.Path, *child, v.CandidateGit.GitIdentity)
	if request.Action == "revalidate" {
		if child.BaseHead == base.Head || !v.Trust.Trusted {
			return fmt.Errorf("revalidation requires an older child base and explicit current check trust")
		}
		request.Revalidation = &client.MissionRevalidationRequest{Previous: input}
		return nil
	}
	if child.BaseHead != base.Head {
		return fmt.Errorf("this candidate uses an older base; :mission-revalidate creates a fresh checked proposal")
	}
	if err := validateMissionCommitSelection(v, *child, input); err != nil {
		return err
	}
	request.Integration = &input
	return nil
}

func validateMissionCommitSelection(v *missionView, child client.MissionExecutionChild, input client.MissionIntegrationRequest) error {
	record := v.Execution.Execution
	for _, id := range record.Mission.Plan.IntegrationOrder {
		index := slices.IndexFunc(record.Children, func(item client.MissionExecutionChild) bool { return item.ID == id })
		if index >= 0 && record.Children[index].State != "integrated" {
			if id != child.ID {
				return fmt.Errorf("integrate child %s first, following the approved order", id)
			}
			break
		}
	}
	commit := v.Candidate.Commit
	if v.Candidate.State != "applied" || commit == nil || commit.State != "committed" || commit.OID != input.Source.Head || commit.Branch != input.Source.Branch {
		return fmt.Errorf("review every changed file, then explicitly Apply and commit this exact candidate first")
	}
	return nil
}

func missionProposalInput(root string, child client.MissionExecutionChild, source client.GitIdentity) client.MissionIntegrationRequest {
	return client.MissionIntegrationRequest{ChildID: child.ID, SessionID: child.SessionID, WorkflowID: child.WorkflowID, ProposalHash: child.ProposalHash, ProposalRevision: child.ProposalRevision, Path: missionCandidatePath(root, child), Source: source}
}

func (s *State) configureMissionRecovery(request *client.MissionExecutionRequest) error {
	v, record := s.Mission, s.Mission.Execution.Execution
	if request.Action != "cleanup_integration" && v.Execution.OwnerState != "recovery_required" {
		return fmt.Errorf("the effect is still owned; refresh after its owner stops or expires")
	}
	base := record.Current.Base
	request.Base, request.ConfirmExecution = &base, true
	if request.Action == "reconcile_revalidation" {
		return configureMissionRevalidationRecovery(v, request)
	}
	for _, entry := range slices.Backward(record.Integrations) {
		eligible := entry.State == "pending" && request.Action == "reconcile_integration"
		eligible = eligible || request.Action == "cleanup_integration" && entry.State == "merged" && entry.Cleanup != "removed" && entry.ChildID == v.ChildID
		if !eligible {
			continue
		}
		for _, child := range missionAllCandidates(record) {
			if child.ID == entry.ChildID && child.SessionID == entry.SessionID {
				input := missionProposalInput(v.Path, child, entry.Source)
				input.IntegrationID = entry.ID
				request.Integration = &input
				return nil
			}
		}
	}
	return fmt.Errorf("no selected integration receipt permits this recovery action")
}

func configureMissionRevalidationRecovery(v *missionView, request *client.MissionExecutionRequest) error {
	record := v.Execution.Execution
	for _, entry := range slices.Backward(record.Revalidations) {
		if entry.State == "pending" {
			request.Revalidation = &client.MissionRevalidationRequest{Previous: missionProposalInput(v.Path, entry.Previous, entry.Source), RevalidationID: entry.ID}
			return nil
		}
	}
	return fmt.Errorf("no pending revalidation needs recovery")
}

func missionAllCandidates(record client.MissionExecution) []client.MissionExecutionChild {
	children := slices.Clone(record.Children)
	for _, entry := range record.Revalidations {
		children = append(children, entry.Previous, entry.Candidate)
	}
	return children
}

func (s *State) openMissionProposal() {
	if err := s.missionExecutionInspectionReady(); err != nil {
		s.Error = err.Error()
		return
	}
	child := s.selectedMissionExecutionChild()
	if child == nil || child.SessionID == "" || child.State == "integrated" {
		s.Error = "Select a retained child proposal; integrated evidence remains in the receipt."
		return
	}
	s.queueMissionAction(missionAction{Operation: "proposal", MissionID: s.Mission.Saved.Mission.ID, Candidate: child})
}

func (s *State) inspectMissionCandidate(selection string, modelHistory bool) {
	if err := s.missionExecutionInspectionReady(); err != nil {
		s.Error = err.Error()
		return
	}
	child := s.selectedMissionExecutionChild()
	if child == nil {
		s.Error = "Select a child from the retained execution."
		return
	}
	if modelHistory {
		selection = "0"
	}
	generation, err := strconv.Atoi(selection)
	if err != nil || generation < 0 || generation > 3 {
		s.Error = "Use :mission-candidate 0|1|2|3; 0 is the original model proposal."
		return
	}
	for _, candidate := range missionAllCandidates(s.Mission.Execution.Execution) {
		if candidate.ID != child.ID || candidate.Generation != generation || candidate.SessionID == "" {
			continue
		}
		operation := "proposal"
		if modelHistory {
			operation = "candidate-history"
		}
		s.queueMissionAction(missionAction{Operation: operation, MissionID: s.Mission.Saved.Mission.ID, Candidate: &candidate})
		return
	}
	s.Error = "That generation has no retained session. Its receipt remains inspectable without recreating a checkout."
}

func (s *State) missionOwnedSelection() bool {
	v := s.Mission
	if v == nil || v.Execution == nil || s.Project == nil || v.Parent.ProjectID != s.Project.ProjectID {
		return false
	}
	if v.Scope.WorktreePath == "" {
		return v.Path == s.Project.Path
	}
	if s.Change == nil {
		return false
	}
	if s.Change.WorktreePath == v.Path && s.Change.ProjectID == v.Scope.ProjectID {
		return true
	}
	for _, child := range missionAllCandidates(v.Execution.Execution) {
		if s.Change.WorktreePath == missionCandidatePath(v.Path, child) && missionCandidateMatches(v.Execution.Execution, child, s.Change) {
			return true
		}
	}
	return false
}

func (s *State) retainMissionRevisionChange() bool {
	v := s.Mission
	if !s.missionOwnedSelection() || v.Parent == s.Project.ProjectIdentity {
		return false
	}
	if v.Pending != nil && (v.Pending.Operation == "execution-read" || v.Pending.Execution.Integration != nil) {
		return true
	}
	s.cancelMissionRequest()
	v.Failure = "Project revision changed. :mission-execution refreshes the retained mission without starting work."
	return true
}

func (s *State) blockMissionChildRetry(command string) bool {
	if (s.Page != PageAgents && s.Page != PageEditor) || (command != "repair" && command != "retry") || s.Change == nil || s.Change.MissionOrigin == nil {
		return false
	}
	s.CmdOutput = "Mission child allocations cannot be restarted independently. Inspect :mission-execution for current-base revalidation or recovery."
	return true
}
