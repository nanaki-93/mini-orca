package tui

import (
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

func (s *State) interceptMissionCommand(command, argument string) bool {
	if s.execMissionCommand(command, argument) {
		return true
	}
	if s.PlanView == "mission" && slices.Contains([]string{"generate", "approve-plan", "save-plan", "approach", "problem", "scope", "context", "criteria", "boundaries", "repairs", "replan", "investigate", "accept-investigation", "cancel-investigation"}, command) {
		s.Error = "Use mission commands for this draft, or :plan to return to the ordinary task plan."
		return true
	}
	return false
}

func (s *State) execMissionCommand(command, argument string) bool {
	switch command {
	case "missions":
		s.PlanView = "mission"
		s.Navigate(PagePlan)
		if s.Mission == nil {
			s.selectMissionScope("main")
		} else {
			s.Mission.History = true
		}
	case "mission-draft":
		if s.Mission != nil {
			s.Mission.History, s.Mission.ExecutionOpen = false, false
		}
		s.PlanView = "mission"
		s.Navigate(PagePlan)
	case "mission-scope":
		s.selectMissionScope(argument)
	case "mission-refresh":
		if s.Mission != nil && s.Mission.ExecutionOpen {
			s.refreshMissionExecution()
		} else {
			s.queueMissionAction(missionAction{Operation: "read"})
		}
	case "mission-open":
		s.openMission(argument)
	case "mission-cancel":
		s.cancelMissionRequest()
	case "mission-new":
		s.newMission(argument)
	case "mission-candidate":
		s.inspectMissionCandidate(argument, false)
	case "mission-model-history":
		s.inspectMissionCandidate("0", true)
	default:
		return s.execMissionDraftCommand(command, argument)
	}
	return true
}

func (s *State) execMissionDraftCommand(command, argument string) bool {
	switch command {
	case "mission-edit":
		field, value, _ := strings.Cut(argument, " ")
		s.editMissionField(field, strings.TrimSpace(value))
	case "mission-budget":
		s.editMissionBudget(strings.Fields(argument))
	case "mission-child":
		s.selectMissionChild(argument)
	case "mission-remove":
		s.removeMissionChild(argument)
	case "mission-plan":
		s.confirmMissionPlanning()
	case "mission-save":
		s.saveMission()
	case "mission-review", "mission-approve":
		s.confirmMissionReview(command == "mission-approve")
	default:
		return s.execMissionExecutionCommand(command)
	}
	return true
}

func (s *State) openMission(id string) {
	if err := s.missionReady(); err != nil {
		s.Error = err.Error()
		return
	}
	if !slices.ContainsFunc(s.Mission.Catalog.Missions, func(item client.MissionSummary) bool { return item.ID == id }) {
		s.Error = "Select a mission from the current checkout's history."
		return
	}
	s.PlanView = "mission"
	s.Mission.History = false
	s.Navigate(PagePlan)
	s.queueMissionAction(missionAction{Operation: "detail", MissionID: id})
}

func (s *State) newMission(goal string) {
	if err := s.missionReady(); err != nil {
		s.Error = err.Error()
		return
	}
	if !missionText(goal, 4096) {
		s.Error = "Use :mission-new <goal> with 1–4096 UTF-8 bytes."
		return
	}
	v := s.Mission
	v.Draft = &client.MissionPlan{Goal: goal, Limits: v.Settings.Limits, Sources: []client.MissionSource{}, Children: []client.MissionChild{}, IntegrationOrder: []string{}}
	v.Saved, v.PlanningReceipt = nil, nil
	v.Execution, v.Candidate, v.CandidateGit = nil, nil, nil
	v.ExecutionOpen, v.ExecutionLoaded, v.ProgressFailure = false, false, ""
	v.PlanningLimits = defaultMissionPlanningLimits(v.Settings.Limits)
	v.Epoch, v.ChildID = nextTUIReqID("mission-draft"), ""
	v.History = false
	s.PlanView = "mission"
	s.Navigate(PagePlan)
	s.DetailSection, s.DetailScroll = 0, 0
	s.CmdOutput = "Local draft. Choose captured paths with :mission-edit files path; path, then review budgets before :mission-plan."
}

func missionText(value string, limit int) bool {
	return strings.TrimSpace(value) != "" && len(value) <= limit && utf8.ValidString(value) && !strings.ContainsFunc(value, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\t' })
}

func (s *State) editableMission() *client.MissionPlan {
	if !s.missionCurrent() || s.Mission.Draft == nil {
		s.Error = "Open or create a mission draft first."
		return nil
	}
	if s.Mission.Saved != nil && s.Mission.Saved.Stale {
		s.Error = "Stored bindings changed. Create a new mission with current source captures."
		return nil
	}
	if s.Mission.Execution != nil {
		s.Error = "An execution freezes its plan. Create a new mission to change scope or allocations."
		return nil
	}
	return s.Mission.Draft
}

func (s *State) missionEdited() {
	s.cancelMissionRequest()
	s.Mission.Epoch = nextTUIReqID("mission-draft")
	s.Mission.PlanningReceipt = nil
	s.DetailScroll = 0
	s.CmdOutput = "Edited locally. Save the revision to clear previous reviews and approval."
}

func (s *State) editMissionField(field, value string) {
	plan := s.editableMission()
	if plan == nil {
		return
	}
	switch field {
	case "goal":
		if !missionText(value, 4096) {
			s.Error = "Provide a nonempty goal of at most 4096 UTF-8 bytes."
			return
		}
		plan.Goal = value
	case "files":
		sources, err := captureMissionFiles(s.Mission.Index, splitPlanList(value))
		if err != nil {
			s.Error = err.Error()
			return
		}
		plan.Sources = sources
	case "order":
		plan.IntegrationOrder = splitPlanList(value)
	default:
		if !s.editMissionChildField(field, value) {
			return
		}
	}
	s.missionEdited()
}

func captureMissionFiles(index *ProjectIndex, paths []string) ([]client.MissionSource, error) {
	if index == nil || len(paths) == 0 || len(paths) > 64 {
		return nil, fmt.Errorf("refresh the checkout index and choose 1–64 explicit file paths")
	}
	var sources []client.MissionSource
	for _, value := range paths {
		if !missionText(value, 512) || path.Clean(value) != value || strings.HasPrefix(value, "/") || value == "." || value == ".." || strings.HasPrefix(value, "../") || strings.ContainsAny(value, "\\\n\t") {
			return nil, fmt.Errorf("mission files must be canonical relative file paths")
		}
		sources = append(sources, missionSource(index, value))
	}
	return sources, nil
}

func (s *State) missionChild() *client.MissionChild {
	if s.Mission == nil || s.Mission.Draft == nil {
		return nil
	}
	for i := range s.Mission.Draft.Children {
		child := &s.Mission.Draft.Children[i]
		if child.ID == s.Mission.ChildID {
			return child
		}
	}
	return nil
}

func (s *State) editMissionChildField(field, value string) bool {
	child := s.missionChild()
	if child == nil {
		s.Error = "Select or create a child with :mission-child <id>."
		return false
	}
	switch field {
	case "title":
		child.Title = value
	case "approach":
		child.Approach = value
	case "criteria":
		child.AcceptanceCriteria = splitPlanList(value)
	case "writes":
		child.WritePaths = splitPlanList(value)
	case "reads":
		child.ReadPaths = splitPlanList(value)
	case "depends":
		child.Dependencies = splitPlanList(value)
	default:
		s.Error = "Edit goal, files, order, title, approach, criteria, writes, reads or depends. Lists use semicolons."
		return false
	}
	return true
}

func (s *State) selectMissionChild(id string) {
	if s.Mission != nil && s.Mission.Execution != nil {
		if slices.ContainsFunc(s.Mission.Execution.Execution.Children, func(child client.MissionExecutionChild) bool { return child.ID == id }) {
			s.Mission.ChildID = id
			s.DetailScroll = 0
		} else {
			s.Error = "Select a child from the frozen execution."
		}
		return
	}
	plan := s.editableMission()
	if plan == nil {
		return
	}
	if !validMissionChildID(id) {
		s.Error = "Use a child ID of 1–64 letters, digits, underscores or hyphens."
		return
	}
	if !slices.ContainsFunc(plan.Children, func(child client.MissionChild) bool { return child.ID == id }) {
		if len(plan.Children) == 16 {
			s.Error = "A mission permits at most 16 children."
			return
		}
		plan.Children = append(plan.Children, client.MissionChild{ID: id, Title: id, Limits: plan.Limits, Dependencies: []string{}, WritePaths: []string{}, ReadPaths: []string{}, AcceptanceCriteria: []string{}})
		plan.IntegrationOrder = append(plan.IntegrationOrder, id)
		s.missionEdited()
		s.CmdOutput = "Child added locally. Set approach, scope, criteria and its budget; total child allocations must fit the mission."
	}
	s.Mission.ChildID, s.DetailSection, s.DetailScroll = id, 1, 0
}

func validMissionChildID(value string) bool {
	return len(value) > 0 && len(value) <= 64 && !strings.ContainsFunc(value, func(r rune) bool {
		return r != '_' && r != '-' && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9')
	})
}

func (s *State) removeMissionChild(id string) {
	plan := s.editableMission()
	if plan == nil {
		return
	}
	index := slices.IndexFunc(plan.Children, func(child client.MissionChild) bool { return child.ID == id })
	if index < 0 {
		s.Error = "Choose an existing child ID."
		return
	}
	plan.Children = slices.Delete(plan.Children, index, index+1)
	plan.IntegrationOrder = slices.DeleteFunc(plan.IntegrationOrder, func(value string) bool { return value == id })
	s.Mission.ChildID = ""
	if len(plan.Children) > 0 {
		s.Mission.ChildID = plan.Children[0].ID
	}
	s.missionEdited()
	s.CmdOutput = "Child removed locally. Review remaining dependencies and integration order before saving."
}

func (s *State) editMissionBudget(fields []string) {
	if s.editableMission() == nil {
		return
	}
	if len(fields) != 3 || s.Mission.Settings == nil {
		s.Error = "Use :mission-budget mission|planning|child <field> <integer>."
		return
	}
	limits, ceiling := &s.Mission.Draft.Limits, s.Mission.Settings.Limits
	switch fields[0] {
	case "mission":
	case "planning":
		limits, ceiling = &s.Mission.PlanningLimits, defaultMissionPlanningLimits(ceiling)
	case "child":
		child := s.missionChild()
		if child == nil {
			s.Error = "Select a child before editing its budget."
			return
		}
		limits, ceiling = &child.Limits, s.Mission.Draft.Limits
	default:
		s.Error = "Choose mission, planning or child budget."
		return
	}
	rows := settingsBudgetRows(&client.AgentSettings{Limits: *limits, Ceilings: ceiling})
	index := slices.IndexFunc(rows, func(row settingsBudget) bool { return row.Name == fields[1] })
	value, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil || index < 0 || value < 0 || value > rows[index].Ceiling || value == 0 && fields[1] != "repairs" {
		s.Error = "Use a positive limit within the displayed ceiling; repairs may be zero."
		return
	}
	setSettingsBudget(limits, fields[1], value)
	s.missionEdited()
}

func (s *State) savedMissionReady() error {
	if err := s.missionReady(); err != nil {
		return err
	}
	v := s.Mission
	if v.Draft == nil || v.Saved == nil || v.Saved.Stale || v.Saved.SourceStatus != "current" || missionDigest(v.Draft) != missionDigest(v.Saved.Mission.Plan) {
		return fmt.Errorf("save the current revision and inspect its source status before review or approval")
	}
	for _, item := range v.Catalog.Missions {
		if item.ID == v.Saved.Mission.ID && item.PlanHash == v.Saved.Mission.PlanHash {
			return nil
		}
	}
	return fmt.Errorf("saved mission changed; reopen it before acting")
}

func (s *State) saveMission() {
	if err := s.missionReady(); err != nil {
		s.Error = err.Error()
		return
	}
	plan := s.editableMission()
	if plan == nil {
		return
	}
	if len(plan.Children) == 0 || len(plan.Sources) == 0 {
		s.Error = "Capture paths and define children before saving; :mission-plan can propose a decomposition."
		return
	}
	v := s.Mission
	copy := cloneMissionPlan(*plan)
	request := client.MissionMutation{ProjectIdentity: v.Scope.ProjectIdentity, CatalogRevision: v.Catalog.CatalogRevision, Action: "create", Plan: &copy}
	id := ""
	if v.Saved != nil {
		id, request.Action, request.PlanHash = v.Saved.Mission.ID, "revise", v.Saved.Mission.PlanHash
	}
	s.queueMissionAction(missionAction{Operation: "mutate", MissionID: id, Mutation: request})
}

func (s *State) confirmMissionPlanning() {
	if err := s.missionReady(); err != nil {
		s.Error = err.Error()
		return
	}
	v := s.Mission
	if v.Draft == nil || !missionText(v.Draft.Goal, 4096) || len(v.Draft.Sources) == 0 {
		s.Error = "Create a mission goal and capture explicit paths before decomposition."
		return
	}
	model, ok := missionCodeModel(v.Settings)
	if !ok {
		s.Error = "Refresh mission settings with an available Code model before planning."
		return
	}
	request := client.MissionPlanningRequest{ProjectIdentity: v.Scope.ProjectIdentity, CatalogRevision: v.Catalog.CatalogRevision, Goal: v.Draft.Goal, Sources: slices.Clone(v.Draft.Sources), Limits: v.Draft.Limits, PlanningLimits: v.PlanningLimits, ConfirmRemoteProvider: model.RemoteProvider}
	action := missionAction{Operation: "plan", Planning: request}
	s.confirmMissionAction("Decompose mission", missionPlanningDisclosure(v, model), action)
}

func missionCodeModel(settings *client.AgentSettings) (client.Model, bool) {
	for _, model := range settings.Models {
		if model.Scope == "code" && model.Model != "" && model.ProviderOrigin != "" && model.Capabilities.Completion {
			return model, true
		}
	}
	return client.Model{}, false
}

func (s *State) confirmMissionReview(approve bool) {
	if err := s.savedMissionReady(); err != nil {
		s.Error = err.Error()
		return
	}
	v := s.Mission
	if v.Saved.Mission.Approved {
		s.CmdOutput = "This exact mission is already approved for planning."
		return
	}
	request := client.MissionMutation{ProjectIdentity: v.Scope.ProjectIdentity, CatalogRevision: v.Catalog.CatalogRevision, PlanHash: v.Saved.Mission.PlanHash, Action: "review", ChildID: v.ChildID, Confirm: true}
	title, message, err := missionReviewDisclosure(v, s.missionChild(), approve)
	if err != nil {
		s.Error = err.Error()
		return
	}
	if approve {
		request.Action, request.ChildID = "approve", ""
	}
	s.confirmMissionAction(title, message, missionAction{Operation: "mutate", MissionID: v.Saved.Mission.ID, Mutation: request})
}

func (s *State) confirmMissionAction(title, message string, action missionAction) {
	fingerprint := s.missionFingerprint()
	childID := s.Mission.ChildID
	s.Confirmation = &Confirmation{Title: title, Message: externalSafeText(message), AcceptLabel: "Confirm", OnConfirm: func(state *State) {
		if !state.missionCurrent() || state.Mission.Parent != state.Project.ProjectIdentity || state.missionFingerprint() != fingerprint || state.Mission.ChildID != childID || state.Mission.Pending != nil {
			state.Error = "Mission, checkout or settings changed. Review and confirm the current selection."
			return
		}
		state.queueMissionAction(action)
	}}
}
