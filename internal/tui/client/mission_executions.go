package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
)

func (c *Client) MissionExecution(ctx context.Context, requestID string, scope AgentSettingsScope, id string) (*AgentMissionExecution, error) {
	if id == "" {
		return nil, fmt.Errorf("select a retained mission")
	}
	var result AgentMissionExecution
	target, err := externalTarget(scope, "missions", id, "execution")
	if err != nil {
		return nil, err
	}
	response, err := c.Request(ctx, requestID, http.MethodGet, target, "")
	if err != nil {
		return nil, err
	}
	// An inspected saved mission can exist before its execution is prepared.
	if response.Status == http.StatusNotFound {
		return nil, nil
	}
	if response.Status != http.StatusOK {
		return nil, fmt.Errorf("%s", DecodeDaemonError(response.Status, response.Body))
	}
	if err := json.Unmarshal([]byte(response.Body), &result); err != nil {
		return nil, fmt.Errorf("decode mission execution: %w", err)
	}
	return &result, validateMissionExecutionReceipt(scope, id, result)
}

func (c *Client) ControlMissionExecution(ctx context.Context, requestID string, scope AgentSettingsScope, id string, request MissionExecutionRequest) (*AgentMissionExecution, error) {
	if id == "" || request.ProjectIdentity != scope.ProjectIdentity || !request.Confirm || !libraryHash.MatchString(request.PlanHash) || !slices.Contains([]string{"prepare", "start", "pause", "cancel", "reconcile", "integrate", "reconcile_integration", "cleanup_integration", "revalidate", "reconcile_revalidation"}, request.Action) {
		return nil, fmt.Errorf("mission execution action or scope changed")
	}
	var result AgentMissionExecution
	if err := c.externalJSON(ctx, requestID, scope, "missions", id, "execution", request, &result); err != nil {
		return nil, err
	}
	observed, err := missionExecutionResultScope(scope, request, result)
	if err != nil {
		return nil, err
	}
	if err := validateMissionExecutionReceipt(observed, id, result); err != nil {
		return nil, err
	}
	return &result, validateMissionControlProgress(request, result)
}

func validateMissionControlProgress(request MissionExecutionRequest, result AgentMissionExecution) error {
	partial := result.Git != nil && len(result.Git.Warnings) > 0 && result.Execution.Revision == request.ExecutionRevision
	if result.Execution.Mission.PlanHash != request.PlanHash || request.ExecutionID != "" && result.Execution.ID != request.ExecutionID || result.Execution.Revision <= request.ExecutionRevision && !partial {
		return fmt.Errorf("mission control did not retain and advance its exact execution")
	}
	return nil
}

func missionExecutionResultScope(scope AgentSettingsScope, request MissionExecutionRequest, result AgentMissionExecution) (AgentSettingsScope, error) {
	if result.Git == nil {
		return scope, nil
	}
	if request.Integration == nil || !request.ConfirmExecution {
		return scope, fmt.Errorf("an execution response performed an unrequested Git action")
	}
	git := result.Git
	if git.Index != nil {
		if git.Index.ProjectID != scope.ProjectID || git.Index.ProjectRevision == "" || git.Repository == nil || git.Repository.ProjectID != scope.ProjectID || git.Repository.ProjectRevision != git.Index.ProjectRevision {
			return scope, fmt.Errorf("integration response belongs to another checkout")
		}
		scope.ProjectRevision = git.Index.ProjectRevision
	}
	return scope, nil
}

func validateMissionExecutionReceipt(scope AgentSettingsScope, id string, value AgentMissionExecution) error {
	record := value.Execution
	if err := externalIdentity(scope, record.Mission.ProjectIdentity, record.ID); err != nil {
		return err
	}
	if err := validateMissionExecutionBounds(record, id); err != nil {
		return err
	}
	if !slices.Contains([]string{"current", "changed", "unavailable"}, value.SourceStatus) || !slices.Contains([]string{"idle", "active", "waiting_for_owner", "recovery_required"}, value.OwnerState) {
		return fmt.Errorf("invalid mission execution freshness or ownership")
	}
	if !value.Stale && (value.SourceStatus != "current" || record.Current.ProjectRevision != scope.ProjectRevision) {
		return fmt.Errorf("mission execution freshness belongs to another checkpoint")
	}
	if !slices.Contains([]string{"prepared", "running", "pausing", "canceling", "paused", "canceled", "awaiting_review", "blocked", "interrupted", "integrating", "revalidating", "ready", "completed"}, record.State) {
		return fmt.Errorf("invalid mission execution state")
	}
	return validateMissionExecutionChildren(record, value.ReadyChildren)
}

func validateMissionExecutionBounds(record MissionExecution, id string) error {
	if record.Mission.ID != id || !libraryHash.MatchString(record.Mission.PlanHash) || record.Revision < 1 || record.CatalogRevision < 1 || len(record.Children) == 0 || len(record.Children) != len(record.Mission.Plan.Children) || len(record.Children) > 16 || len(record.Integrations) > 128 || len(record.Revalidations) > 48 {
		return fmt.Errorf("invalid mission execution identity or bounds")
	}
	return validateMissionCheckpoint(record.Current, record.Mission.Plan.Sources)
}

func validateMissionCheckpoint(checkpoint MissionCheckpoint, sources []MissionSource) error {
	if checkpoint.ProjectRevision == "" || checkpoint.Base.Head == "" || checkpoint.Base.Branch == "" || len(checkpoint.Sources) != len(sources) || len(checkpoint.Instructions) != len(sources) {
		return fmt.Errorf("mission checkpoint lost its source bindings")
	}
	for index, source := range sources {
		if checkpoint.Sources[index].Path != source.Path || checkpoint.Instructions[source.Path] == "" {
			return fmt.Errorf("mission checkpoint source scope changed")
		}
	}
	return nil
}

func validateMissionExecutionChildren(record MissionExecution, ready []string) error {
	seen := map[string]bool{}
	for index, child := range record.Children {
		if child.ID != record.Mission.Plan.Children[index].ID || child.Limits != record.Mission.Plan.Children[index].Limits || seen[child.ID] || !slices.Contains([]string{"pending", "reserved", "running", "awaiting_review", "rejected", "failed", "interrupted", "integrated"}, child.State) {
			return fmt.Errorf("mission execution child differs from its frozen allocation")
		}
		seen[child.ID] = true
		if err := validateMissionCandidate(record.ID, child); err != nil {
			return err
		}
	}
	readyIDs := map[string]bool{}
	for _, id := range ready {
		if readyIDs[id] || !slices.ContainsFunc(record.Children, func(child MissionExecutionChild) bool { return child.ID == id && child.State == "pending" }) {
			return fmt.Errorf("mission readiness contains an unowned or reserved child")
		}
		readyIDs[id] = true
	}
	if err := validateMissionIntegrationReceipts(record); err != nil {
		return err
	}
	return validateMissionCandidateLineage(record)
}

func validateMissionCandidate(executionID string, child MissionExecutionChild) error {
	if child.Generation < 0 || child.Generation > 3 {
		return fmt.Errorf("invalid mission candidate generation")
	}
	if child.State == "pending" {
		if child.Directory != "" || child.SessionID != "" || child.Generation != 0 {
			return fmt.Errorf("unstarted child contains a candidate")
		}
		return nil
	}
	directory := executionID + "-" + child.ID
	if child.Generation > 0 {
		directory += "-r" + strconv.Itoa(child.Generation)
	}
	if filepath.Base(directory) != directory || child.Directory != directory || child.Branch != "codex/"+directory || child.BaseHead == "" {
		return fmt.Errorf("mission candidate lost its owned checkout")
	}
	return validateMissionCandidateProposal(child)
}

func validateMissionCandidateProposal(child MissionExecutionChild) error {
	if child.SessionID == "" && slices.Contains([]string{"awaiting_review", "integrated"}, child.State) {
		return fmt.Errorf("reviewable child has no proposal identity")
	}
	if child.SessionID != "" && (child.ProjectID == "" || child.ProjectRevision == "" || child.ProposalHash == "" || child.ProposalRevision < 1 || (child.Generation == 0) != (child.WorkflowID != "")) {
		return fmt.Errorf("mission candidate lost its proposal or workflow identity")
	}
	return nil
}

func validateMissionCandidateLineage(record MissionExecution) error {
	seen := map[string]bool{}
	generations := map[string]int{}
	selected := map[string]MissionExecutionChild{}
	for _, entry := range record.Revalidations {
		if !externalRecordID.MatchString(entry.ID) || seen[entry.ID] || !slices.Contains([]string{"pending", "awaiting_review", "failed", "interrupted"}, entry.State) {
			return fmt.Errorf("invalid retained revalidation identity or state")
		}
		seen[entry.ID] = true
		generations[entry.Previous.ID]++
		if err := validateMissionRevalidationEntry(record, entry, generations[entry.Previous.ID]); err != nil {
			return err
		}
		previous, exists := selected[entry.Previous.ID]
		if exists && !sameMissionCandidate(previous, entry.Previous) || !exists && entry.Previous.Generation != 0 {
			return fmt.Errorf("revalidation lost the selected previous candidate")
		}
		selected[entry.Previous.ID] = entry.Previous
		if entry.State == "awaiting_review" {
			selected[entry.Previous.ID] = entry.Candidate
		}
	}
	return validateMissionSelectedCandidates(record.Children, selected)
}

func validateMissionSelectedCandidates(children []MissionExecutionChild, selected map[string]MissionExecutionChild) error {
	for _, child := range children {
		chosen, exists := selected[child.ID]
		if exists && !sameMissionCandidate(chosen, child) || !exists && child.Generation > 0 {
			return fmt.Errorf("selected candidate has no successful revalidation receipt")
		}
	}
	return nil
}

func sameMissionCandidate(a, b MissionExecutionChild) bool {
	a.State, a.Reason, a.UpdatedAt = "", "", ""
	b.State, b.Reason, b.UpdatedAt = "", "", ""
	return a == b
}

func validateMissionRevalidationEntry(record MissionExecution, entry MissionRevalidation, generation int) error {
	if !slices.ContainsFunc(record.Mission.Plan.Children, func(child MissionChild) bool {
		return child.ID == entry.Previous.ID && child.Limits == entry.Previous.Limits
	}) {
		return fmt.Errorf("revalidation lost its approved child allocation")
	}
	if entry.Candidate.ID != entry.Previous.ID || entry.Candidate.Generation != generation || entry.Candidate.Generation <= entry.Previous.Generation || entry.Candidate.Limits != entry.Previous.Limits || entry.Candidate.BaseHead != entry.Before.Base.Head || entry.Candidate.WorkflowID != "" || entry.AllowanceMillis < 1 {
		return fmt.Errorf("revalidation changed candidate lineage or its allocation")
	}
	for _, child := range []MissionExecutionChild{entry.Previous, entry.Candidate} {
		if err := validateMissionCandidate(record.ID, child); err != nil {
			return err
		}
	}
	if err := validateMissionProposalEvidence(record.ID, entry.Previous.ID, entry.Evidence); err != nil {
		return err
	}
	if err := validateMissionCheckpoint(entry.Before, record.Mission.Plan.Sources); err != nil {
		return err
	}
	return nil
}

func validateMissionIntegrationReceipts(record MissionExecution) error {
	checkpoint := MissionCheckpoint{ProjectRevision: record.Mission.ProjectRevision, Base: record.Base, Sources: record.Mission.Plan.Sources, Instructions: record.Mission.Instructions}
	seen := map[string]bool{}
	for _, entry := range record.Integrations {
		if !externalRecordID.MatchString(entry.ID) || seen[entry.ID] || !reflect.DeepEqual(entry.Before, checkpoint) || !slices.ContainsFunc(record.Children, func(child MissionExecutionChild) bool { return child.ID == entry.ChildID }) {
			return fmt.Errorf("integration receipt lost its child or checkpoint identity")
		}
		seen[entry.ID] = true
		if err := validateMissionIntegrationEvidence(record, entry); err != nil {
			return err
		}
		if entry.State == "merged" {
			checkpoint = *entry.After
		}
	}
	if !reflect.DeepEqual(checkpoint, record.Current) {
		return fmt.Errorf("current checkpoint has no verified integration receipt")
	}
	return nil
}

func validateMissionIntegrationEvidence(record MissionExecution, entry MissionIntegration) error {
	if !missionIntegrationCandidateMatches(record, entry) {
		return fmt.Errorf("integration lost its exact retained candidate")
	}
	if !slices.Contains([]string{"pending", "merged", "failed", "conflicts", "unavailable"}, entry.State) || !slices.Contains([]string{"retained", "pending", "worktree_removed", "removed", "failed"}, entry.Cleanup) {
		return fmt.Errorf("invalid integration or cleanup state")
	}
	if err := validateMissionProposalEvidence(record.ID, entry.ChildID, entry.Evidence); err != nil {
		return err
	}
	if (entry.State == "merged") != (entry.After != nil) {
		return fmt.Errorf("integration result does not match its checkpoint")
	}
	if entry.After != nil {
		return validateMissionCheckpoint(*entry.After, record.Mission.Plan.Sources)
	}
	return nil
}

func missionIntegrationCandidateMatches(record MissionExecution, entry MissionIntegration) bool {
	children := slices.Clone(record.Children)
	for _, attempt := range record.Revalidations {
		children = append(children, attempt.Previous)
	}
	for _, child := range children {
		if child.ID == entry.ChildID && child.SessionID == entry.SessionID && child.WorkflowID == entry.WorkflowID && child.ProposalHash == entry.ProposalHash && child.ProposalRevision == entry.ProposalRevision && child.Branch == entry.Source.Branch && child.BaseHead == entry.Before.Base.Head {
			return true
		}
	}
	return false
}

func validateMissionProposalEvidence(executionID, childID string, evidence MissionProposalEvidence) error {
	if evidence.Key.Session != (AgentSessionKey{App: "mini-orca-missions", User: "local", ID: executionID}) || evidence.Key.Name != childID+"-proposal.json" || evidence.Version < 1 || !libraryHash.MatchString(evidence.Hash) {
		return fmt.Errorf("mission receipt lost its protected proposal reference")
	}
	return nil
}
