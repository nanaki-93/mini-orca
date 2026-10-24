package tui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

type missionView struct {
	Parent          ProjectIdentity
	Scope           client.AgentSettingsScope
	Path, Epoch     string
	Catalog         *client.MissionList
	Index           *ProjectIndex
	Settings        *client.AgentSettings
	Draft           *client.MissionPlan
	Saved           *client.AgentMission
	PlanningLimits  client.AgentLimits
	PlanningReceipt *client.MissionPlanningResult
	Selection       int
	History         bool
	ChildID         string
	Pending         *missionAction
	Cancel          context.CancelFunc
	Failure         string
	Execution       *client.AgentMissionExecution
	ExecutionOpen   bool
	ExecutionLoaded bool
	Repository      *GitRepository
	Trust           *ExecutionTrust
	Candidate       *ChangeSession
	CandidateGit    *GitRepository
	ProgressFailure string
	OwnerChange     *ChangeSession
}

type missionAction struct {
	ID, Operation, MissionID, Expected, SettingsHash string
	Parent                                           ProjectIdentity
	Scope                                            client.AgentSettingsScope
	Mutation                                         client.MissionMutation
	Planning                                         client.MissionPlanningRequest
	Execution                                        client.MissionExecutionRequest
	Captured                                         *client.AgentMissionExecution
	Candidate                                        *client.MissionExecutionChild
	Path, TrustHash                                  string
	Sources                                          []client.MissionSource
}

type missionResult struct {
	Action       missionAction
	Catalog      *client.MissionList
	Index        *ProjectIndex
	Settings     *client.AgentSettings
	Detail       *client.AgentMission
	Planning     *client.MissionPlanningResult
	Execution    *client.AgentMissionExecution
	Owner        *Project
	Repository   *GitRepository
	Trust        *ExecutionTrust
	Candidate    *ChangeSession
	CandidateGit *GitRepository
	Err          error
}

func missionDigest(value any) string {
	// All callers use finite, JSON-compatible wire DTOs without custom encoders.
	data, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("invalid internal mission snapshot: %v", err))
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func cloneMissionPlan(plan client.MissionPlan) client.MissionPlan {
	plan.Sources, plan.IntegrationOrder = slices.Clone(plan.Sources), slices.Clone(plan.IntegrationOrder)
	plan.Children = slices.Clone(plan.Children)
	for i := range plan.Children {
		child := &plan.Children[i]
		child.WritePaths, child.ReadPaths = slices.Clone(child.WritePaths), slices.Clone(child.ReadPaths)
		child.Dependencies, child.AcceptanceCriteria = slices.Clone(child.Dependencies), slices.Clone(child.AcceptanceCriteria)
	}
	return plan
}

func (s *State) missionCurrent() bool {
	v := s.Mission
	return v != nil && (s.checkoutScopeCurrent(v.Parent, v.Scope, v.Path) || s.missionOwnedSelection())
}

func (s *State) missionFingerprint() string {
	v := s.Mission
	if v == nil {
		return ""
	}
	var revision int64
	if v.Catalog != nil {
		revision = v.Catalog.CatalogRevision
	}
	return missionDigest([]any{v.Parent, v.Scope, v.Path, v.Epoch, revision, v.Draft, v.Saved, v.PlanningLimits, v.Settings, s.Models, v.Execution, v.Repository, v.Trust, v.Candidate, v.CandidateGit})
}

func (s *State) missionReady() error {
	if !s.missionCurrent() || s.Mission.Catalog == nil || s.Mission.Settings == nil || s.Mission.Index == nil || s.Mission.Failure != "" {
		return fmt.Errorf("refresh missions for the selected checkout before acting")
	}
	if s.Mission.Pending != nil {
		return fmt.Errorf("finish or cancel the pending mission request first")
	}
	if s.Mission.Settings.State == "stale" {
		return fmt.Errorf("mission settings are stale; review Roles & Budgets for this checkout")
	}
	return nil
}

func (s *State) selectMissionScope(selection string) {
	if s.Project == nil || s.projectID() == "" || s.projectRevision() == "" {
		s.Error = "Open an indexed project before planning a mission."
		return
	}
	scope, path := client.AgentSettingsScope{ProjectIdentity: s.Project.ProjectIdentity}, s.Project.Path
	switch selection {
	case "main":
	case "task":
		if s.Change == nil || s.Change.WorktreePath == "" || s.Change.ProjectRevision == "" {
			s.Error = "Select an indexed task worktree first."
			return
		}
		scope, path = client.AgentSettingsScope{ProjectIdentity: s.Change.ProjectIdentity, WorktreePath: s.Change.WorktreePath}, s.Change.WorktreePath
	default:
		s.Error = "Use :mission-scope main or :mission-scope task."
		return
	}
	s.cancelMissionRequest()
	s.Mission = &missionView{Parent: s.Project.ProjectIdentity, Scope: scope, Path: path, Epoch: nextTUIReqID("mission-view"), History: true}
	if selection == "task" {
		owner := *s.Change
		s.Mission.OwnerChange = &owner
	}
	s.PlanView = "mission"
	s.Navigate(PagePlan)
	s.queueMissionAction(missionAction{Operation: "read"})
}

func (s *State) queueMissionAction(action missionAction) {
	if !s.missionCurrent() || s.Mission.Pending != nil {
		s.Error = "Select a current checkout and finish or cancel its pending mission request."
		return
	}
	v := s.Mission
	action.ID, action.Parent, action.Scope = nextTUIReqID("mission"), v.Parent, v.Scope
	action.Path, action.TrustHash = v.Path, missionDigest(v.Trust)
	action.Captured = v.Execution
	action.Expected, action.SettingsHash = s.missionFingerprint(), missionDigest(v.Settings)
	v.Pending = &action
	s.enqueueAction(ActionRequest{Kind: "mission", Mission: &action})
}

func (s *State) acceptsMissionAction(action *missionAction) bool {
	return action != nil && s.missionCurrent() && s.Mission.Pending != nil && s.Mission.Pending.ID == action.ID && s.Mission.Scope == action.Scope && s.Mission.Parent == action.Parent && s.missionActionParentCurrent(action) && s.missionFingerprint() == action.Expected
}

func (s *State) missionActionParentCurrent(action *missionAction) bool {
	return s.Project != nil && (s.Project.ProjectIdentity == action.Parent || action.Operation == "execution-read" || action.Execution.Integration != nil)
}

func (s *State) prepareMissionAction(action *missionAction) bool {
	if s.acceptsMissionAction(action) && (action.Operation == "execution-read" || action.Parent == s.Project.ProjectIdentity) {
		return true
	}
	if action != nil && s.Mission != nil && s.Mission.Pending != nil && s.Mission.Pending.ID == action.ID {
		s.cancelMissionRequest()
		s.Error = "Mission input or checkout changed; refresh before trying again."
	}
	return false
}

func (m *BubbleModel) prepareMissionCmd(action *missionAction) tea.Cmd {
	if !m.state.prepareMissionAction(action) {
		return nil
	}
	captured := *action
	owned, cancel := context.WithCancel(m.ctx)
	m.state.Mission.Cancel = cancel
	return func() tea.Msg {
		defer cancel()
		return missionResultMsg{result: executeMissionAction(owned, m.client, captured)}
	}
}

func (s *State) cancelMissionRequest() {
	v := s.Mission
	if v == nil || v.Pending == nil {
		return
	}
	if v.Cancel != nil {
		v.Cancel()
		v.Cancel = nil
	}
	v.Pending = nil
	v.Failure = "Request canceled. Draft and receipts retained; refresh to establish whether the action completed."
	s.CmdOutput = v.Failure
}

func (s *State) reconcileMissionScope() {
	if s.retainMissionRevisionChange() {
		return
	}
	if s.Mission != nil && !s.missionCurrent() {
		s.cancelMissionRequest()
		s.Mission = nil
	}
}

func executeMissionAction(ctx context.Context, cl *Client, action missionAction) missionResult {
	r := missionResult{Action: action}
	if cl == nil {
		r.Err = fmt.Errorf("daemon is unavailable")
		return r
	}
	switch action.Operation {
	case "read":
		r.Catalog, r.Err = cl.ReadMissions(ctx, action.ID+"-catalog", action.Scope)
		if r.Err == nil {
			r.Settings, r.Err = cl.ReadAgentSettings(ctx, action.ID+"-settings", action.Scope)
		}
		if r.Err == nil {
			r.Index, r.Err = cl.ExternalIndex(ctx, action.ID+"-index", action.Scope)
		}
	case "detail":
		r.Detail, r.Err = cl.MissionDetail(ctx, action.ID, action.Scope, action.MissionID)
	case "plan":
		r.Err = validateMissionDispatch(ctx, cl, action)
		if r.Err == nil {
			r.Planning, r.Err = cl.PlanMission(ctx, action.ID, action.Scope, action.Planning)
		}
		if r.Err == nil {
			r.Detail = &r.Planning.AgentMission
		}
	case "mutate":
		r.Detail, r.Err = cl.MutateMission(ctx, action.ID, action.Scope, action.MissionID, action.Mutation)
	default:
		return executeMissionExecutionAction(ctx, cl, action)
	}
	return r
}

func validateMissionDispatch(ctx context.Context, cl *Client, action missionAction) error {
	settings, err := cl.ReadAgentSettings(ctx, action.ID+"-settings", action.Scope)
	if err != nil {
		return err
	}
	if missionDigest(settings) != action.SettingsHash {
		return fmt.Errorf("mission settings or provider changed after confirmation; refresh and confirm again")
	}
	index, err := cl.ExternalIndex(ctx, action.ID+"-index", action.Scope)
	if err != nil {
		return err
	}
	for _, captured := range action.Planning.Sources {
		current := missionSource(index, captured.Path)
		if current != captured {
			return fmt.Errorf("mission source index changed after confirmation")
		}
	}
	return context.Cause(ctx)
}

func (s *State) applyMissionResult(result missionResult) {
	if !s.acceptsMissionAction(&result.Action) {
		if s.Mission != nil && s.Mission.Pending != nil && s.Mission.Pending.ID == result.Action.ID {
			s.cancelMissionRequest()
		}
		return
	}
	v := s.Mission
	if v.Cancel != nil {
		v.Cancel()
		v.Cancel = nil
	}
	v.Pending = nil
	if result.Err != nil {
		v.Failure = externalSafeText(result.Err.Error()) + " · Draft retained. Refresh local history before retrying."
		s.Error = v.Failure
		return
	}
	v.Failure = ""
	if s.applyMissionExecutionResult(result) {
		return
	}
	if result.Action.Operation == "read" {
		s.applyMissionCatalog(result)
		return
	}
	if result.Detail == nil {
		v.Failure = "Mission response is unavailable; refresh before another action."
		return
	}
	v.Saved, v.PlanningReceipt = result.Detail, result.Planning
	v.Execution, v.Candidate, v.CandidateGit = nil, nil, nil
	v.ExecutionOpen, v.ExecutionLoaded = false, false
	plan := cloneMissionPlan(result.Detail.Mission.Plan)
	v.Draft = &plan
	v.Epoch = nextTUIReqID("mission-draft")
	if !slices.ContainsFunc(plan.Children, func(child client.MissionChild) bool { return child.ID == v.ChildID }) {
		v.ChildID = plan.Children[0].ID
	}
	if v.PlanningLimits.ModelCalls == 0 && v.Settings != nil {
		v.PlanningLimits = defaultMissionPlanningLimits(v.Settings.Limits)
	}
	s.updateMissionSummary(result.Detail)
	s.DetailScroll = 0
	s.CmdOutput = "Mission retained. Inspect each child, record its review, then separately approve the exact plan."
}

func (s *State) applyMissionCatalog(result missionResult) {
	v := s.Mission
	selected := ""
	if v.Catalog != nil && v.Selection >= 0 && v.Selection < len(v.Catalog.Missions) {
		selected = v.Catalog.Missions[v.Selection].ID
	}
	v.Catalog, v.Settings, v.Index = result.Catalog, result.Settings, result.Index
	v.Selection = min(v.Selection, max(0, len(v.Catalog.Missions)-1))
	if index := slices.IndexFunc(v.Catalog.Missions, func(item client.MissionSummary) bool { return item.ID == selected }); index >= 0 {
		v.Selection = index
	}
	s.CmdOutput = "Loaded local mission history, settings and source identities. No model work started."
}

func (s *State) updateMissionSummary(detail *client.AgentMission) {
	v, m := s.Mission, detail.Mission
	if v.Catalog == nil {
		return
	}
	v.Catalog.CatalogRevision = detail.CatalogRevision
	item := client.MissionSummary{ID: m.ID, ProjectRevision: m.ProjectRevision, Goal: m.Plan.Goal, PlanHash: m.PlanHash, ChildCount: len(m.Plan.Children), ReviewedCount: len(m.Reviewed), Approved: m.Approved, Stale: detail.Stale, SourceStatus: "unchecked", UpdatedAt: m.UpdatedAt}
	for i, previous := range v.Catalog.Missions {
		if previous.ID == m.ID {
			v.Catalog.Missions[i] = item
			return
		}
	}
	v.Catalog.Missions = append(v.Catalog.Missions, item)
}

func missionSource(index *ProjectIndex, path string) client.MissionSource {
	for _, file := range index.Files {
		if file.Path == path {
			return client.MissionSource{Path: path, Hash: file.ContentHash, Exists: true}
		}
	}
	return client.MissionSource{Path: path}
}

func defaultMissionPlanningLimits(ceiling client.AgentLimits) client.AgentLimits {
	return client.AgentLimits{ModelCalls: min(8, ceiling.ModelCalls), ToolCalls: min(24, ceiling.ToolCalls), InputTokens: min(1<<20, ceiling.InputTokens), OutputTokens: min(8<<20, ceiling.OutputTokens), ElapsedSeconds: min(300, ceiling.ElapsedSeconds), ConcurrentCalls: 1}
}
