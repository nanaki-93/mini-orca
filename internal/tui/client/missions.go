package client

import (
	"context"
	"fmt"
	"reflect"
	"slices"
)

func (c *Client) ReadMissions(ctx context.Context, requestID string, scope AgentSettingsScope) (*MissionList, error) {
	var result MissionList
	if err := c.externalJSON(ctx, requestID, scope, "missions", "", "", nil, &result); err != nil {
		return nil, err
	}
	if result.ProjectIdentity != scope.ProjectIdentity || result.CatalogRevision < 0 || len(result.Missions) > 128 {
		return nil, fmt.Errorf("mission catalog belongs to another checkout or is invalid")
	}
	seen := map[string]bool{}
	for _, item := range result.Missions {
		if !externalRecordID.MatchString(item.ID) || !libraryHash.MatchString(item.PlanHash) || seen[item.ID] {
			return nil, fmt.Errorf("mission catalog contains an invalid identity")
		}
		seen[item.ID] = true
	}
	return &result, nil
}

func validateMissionReceipt(scope AgentSettingsScope, id string, value *AgentMission) error {
	m := value.Mission
	if err := externalIdentity(scope, m.ProjectIdentity, m.ID); err != nil {
		return fmt.Errorf("mission identity: %w", err)
	}
	if !libraryHash.MatchString(m.PlanHash) || value.CatalogRevision < 1 || id != "" && id != m.ID {
		return fmt.Errorf("mission receipt does not match its checkout or plan identity")
	}
	if !slices.Contains([]string{"current", "changed", "unavailable"}, value.SourceStatus) || !value.Stale && (value.SourceStatus != "current" || m.ProjectIdentity != scope.ProjectIdentity) {
		return fmt.Errorf("mission receipt has inconsistent source freshness")
	}
	if len(m.Plan.Children) == 0 || len(m.Plan.Children) > 16 || len(m.Plan.Sources) == 0 || len(m.Plan.Sources) > 64 {
		return fmt.Errorf("mission receipt has an invalid bounded plan")
	}
	return nil
}

func (c *Client) MissionDetail(ctx context.Context, requestID string, scope AgentSettingsScope, id string) (*AgentMission, error) {
	if id == "" {
		return nil, fmt.Errorf("select a retained mission")
	}
	var result AgentMission
	if err := c.externalJSON(ctx, requestID, scope, "missions", id, "", nil, &result); err != nil {
		return nil, err
	}
	return &result, validateMissionReceipt(scope, id, &result)
}

func (c *Client) MutateMission(ctx context.Context, requestID string, scope AgentSettingsScope, id string, request MissionMutation) (*AgentMission, error) {
	if request.ProjectIdentity != scope.ProjectIdentity || !slices.Contains([]string{"create", "revise", "review", "approve"}, request.Action) || (request.Action == "create") != (id == "") {
		return nil, fmt.Errorf("mission mutation scope or action changed")
	}
	var result AgentMission
	if err := c.externalJSON(ctx, requestID, scope, "missions", id, "", request, &result); err != nil {
		return nil, err
	}
	return &result, validateMissionMutationReceipt(scope, id, request, result)
}

func validateMissionMutationReceipt(scope AgentSettingsScope, id string, request MissionMutation, result AgentMission) error {
	if err := validateMissionReceipt(scope, id, &result); err != nil {
		return err
	}
	if result.Stale || result.CatalogRevision != request.CatalogRevision+1 || result.Mission.ProjectIdentity != scope.ProjectIdentity {
		return fmt.Errorf("mission mutation did not advance the captured catalog")
	}
	if request.Plan != nil && (!reflect.DeepEqual(*request.Plan, result.Mission.Plan) || result.Mission.Approved || len(result.Mission.Reviewed) != 0) {
		return fmt.Errorf("mission revision did not retain the submitted unapproved plan")
	}
	return validateMissionDecision(request, result.Mission)
}

func validateMissionDecision(request MissionMutation, result MissionRecord) error {
	if request.Action == "review" && (result.PlanHash != request.PlanHash || !slices.Contains(result.Reviewed, request.ChildID) || result.Approved) {
		return fmt.Errorf("mission receipt did not record the exact child review")
	}
	if request.Action == "approve" && (result.PlanHash != request.PlanHash || !result.Approved) {
		return fmt.Errorf("mission receipt did not approve the exact plan")
	}
	return nil
}

func (c *Client) PlanMission(ctx context.Context, requestID string, scope AgentSettingsScope, request MissionPlanningRequest) (*MissionPlanningResult, error) {
	if request.ProjectIdentity != scope.ProjectIdentity {
		return nil, fmt.Errorf("mission planning checkout changed")
	}
	var result MissionPlanningResult
	if err := c.externalJSON(ctx, requestID, scope, "missions", "", "plan", request, &result); err != nil {
		return nil, err
	}
	if err := validateMissionReceipt(scope, "", &result.AgentMission); err != nil {
		return nil, err
	}
	plan := result.Mission.Plan
	if result.Stale || result.Mission.ProjectIdentity != scope.ProjectIdentity || result.Mission.Approved || len(result.Mission.Reviewed) != 0 || result.CatalogRevision != request.CatalogRevision+1 {
		return nil, fmt.Errorf("planner did not return a new unapproved draft")
	}
	if result.PromptVersion != "mission-planner-v1" || !libraryHash.MatchString(result.InputHash) || plan.Goal != request.Goal || plan.Limits != request.Limits || !slices.Equal(plan.Sources, request.Sources) || result.PlanningLimits != request.PlanningLimits {
		return nil, fmt.Errorf("planner receipt changed the admitted constraints")
	}
	return &result, nil
}
