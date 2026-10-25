package tui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

func missionBudgetText(limits client.AgentLimits) string {
	return fmt.Sprintf("%d model calls · %d tools · %d input / %d output tokens · %ds · %d concurrent · %d repairs", limits.ModelCalls, limits.ToolCalls, limits.InputTokens, limits.OutputTokens, limits.ElapsedSeconds, limits.ConcurrentCalls, limits.Repairs)
}

func missionScopeText(sources []client.MissionSource) string {
	var lines []string
	for _, source := range sources {
		identity := "planned path · absent at capture"
		if source.Exists {
			identity = source.Hash
		}
		lines = append(lines, source.Path+"\n"+identity)
	}
	return strings.Join(lines, "\n")
}

func missionPlanningDisclosure(v *missionView, model client.Model) string {
	return fmt.Sprintf("Checkout: %s\nProject: %s\nRevision: %s\nCatalog: %d\n\nGoal: %s\n\nCode provider: %s\nModel: %s\nRemote sharing: %t\nSettings version: %d\n\nCaptured paths:\n%s\nApplicable AGENTS.md instructions for these paths accompany the request.\n\nPlanning budget: %s\nFuture mission allocation: %s\n\nThis sends the goal and admitted context to the displayed provider and saves a new unapproved suggestion. Existing saved missions remain separate. It does not launch children, run checks or change project source.", v.Path, v.Scope.ProjectID, v.Scope.ProjectRevision, v.Catalog.CatalogRevision, v.Draft.Goal, model.ProviderOrigin, model.Model, model.RemoteProvider, v.Settings.Version, missionScopeText(v.Draft.Sources), missionBudgetText(v.PlanningLimits), missionBudgetText(v.Draft.Limits))
}

func missionChildFields(child client.MissionChild) [][2]string {
	return [][2]string{{"Child:", child.ID + " · " + child.Title}, {"Approach:", child.Approach}, {"Depends on:", client.FirstNonEmpty(strings.Join(child.Dependencies, "; "), "None")}, {"Writes:", strings.Join(child.WritePaths, "; ")}, {"Reads:", client.FirstNonEmpty(strings.Join(child.ReadPaths, "; "), "None")}, {"Acceptance:", strings.Join(child.AcceptanceCriteria, "\n")}, {"Child allocation:", missionBudgetText(child.Limits)}}
}

func missionReviewDisclosure(v *missionView, child *client.MissionChild, approve bool) (string, string, error) {
	message := fmt.Sprintf("Checkout: %s\nCurrent revision: %s\nStored revision: %s\nMission: %s\nCatalog: %d\nExact plan hash: %s\n\nGoal: %s\nIntegration order: %s\n\n", v.Path, v.Scope.ProjectRevision, v.Saved.Mission.ProjectRevision, v.Saved.Mission.ID, v.Catalog.CatalogRevision, v.Saved.Mission.PlanHash, v.Draft.Goal, strings.Join(v.Draft.IntegrationOrder, " → "))
	if approve {
		for _, item := range v.Draft.Children {
			if !slices.Contains(v.Saved.Mission.Reviewed, item.ID) {
				return "", "", fmt.Errorf("review every child before approving the whole mission")
			}
		}
		return "Approve mission plan", message + "Captured inventory:\n" + missionScopeText(v.Draft.Sources) + "\n\nMission allocation: " + missionBudgetText(v.Draft.Limits) + "\n\nApprove this exact reviewed plan. This decision does not execute children or grant Apply authority.", nil
	}
	if child == nil || slices.Contains(v.Saved.Mission.Reviewed, child.ID) {
		return "", "", fmt.Errorf("select an unreviewed child of the saved plan")
	}
	for _, field := range missionChildFields(*child) {
		message += field[0] + " " + field[1] + "\n"
	}
	return "Record child review", message + "\nRecord review of this concrete child within the exact saved plan. Final mission approval is separate.", nil
}

func (s *State) missionStateText() string {
	v := s.Mission
	if v.Saved == nil {
		return "LOCAL DRAFT · unapproved"
	}
	if v.Execution != nil {
		return "FROZEN PLAN · execution " + v.Execution.Execution.State
	}
	if v.Saved.Stale {
		return "STALE · source " + v.Saved.SourceStatus
	}
	if missionDigest(v.Draft) != missionDigest(v.Saved.Mission.Plan) {
		return "EDITED LOCALLY · previous reviews do not apply"
	}
	if v.Saved.Mission.Approved {
		return "PLAN APPROVED · execution not started"
	}
	return fmt.Sprintf("SAVED DRAFT · %d/%d children reviewed", len(v.Saved.Mission.Reviewed), len(v.Draft.Children))
}

func (s *State) renderMissionPage(th ansiTheme, width int) []string {
	if !s.missionCurrent() {
		lines := renderPageHeader(th, "MISSIONS", ":mission-scope main|task")
		return append(lines, "  Select :mission-scope main or task to read local history.")
	}
	v := s.Mission
	title := "MISSIONS"
	if v.Draft != nil && !v.History {
		title = "MISSION PLAN"
		if v.ExecutionOpen {
			title = "MISSION EXECUTION"
		}
	}
	var hints []string
	for _, action := range s.missionHelpSpec().primary {
		hints = append(hints, "["+action.key+"] "+action.desc)
	}
	lines := renderPageHeader(th, title, strings.Join(hints, " · "))
	lines = append(lines, "  "+theme.TruncatePlain(externalSafeText(v.Path), max(1, width-4)))
	if v.Pending != nil {
		lines = append(lines, "  "+th.amber+"Request pending · [x] Cancel"+th.reset)
	}
	if v.Failure != "" {
		lines = append(lines, "  "+th.amber+"Unavailable: "+externalSafeText(v.Failure)+th.reset)
	}
	if v.Draft == nil || v.History {
		return s.renderMissionHistory(th, width, lines)
	}
	lines = append(lines, s.renderDetailSections(th)...)
	var rows []string
	for _, field := range s.missionDetailFields() {
		rows = append(rows, renderDiagnosticRow(th, th.cyan, field[0], th.fg, externalSafeText(field[1]), issuePanelContentWidth(adaptivePanelWidth(width)))...)
	}
	meta := "REVIEW BEFORE APPROVAL"
	if v.ExecutionOpen {
		meta = "EXECUTION RECEIPTS"
	}
	return append(lines, th.renderPanelBox(s.detailSections()[s.DetailSection], meta, adaptivePanelWidth(width), rows)...)
}

func (s *State) renderMissionHistory(th ansiTheme, width int, lines []string) []string {
	v := s.Mission
	if v.Catalog == nil {
		return append(lines, "  History not loaded · :mission-refresh")
	}
	if len(v.Catalog.Missions) == 0 {
		return append(lines, "  No saved missions in this checkout.", "  [n] Create a local draft. Model decomposition is a separate action.")
	}
	selected := min(max(0, v.Selection), len(v.Catalog.Missions)-1)
	start, end := windowBounds(len(v.Catalog.Missions), selected, s.adaptiveWindowRows(8, 4))
	for i := start; i < end; i++ {
		item := v.Catalog.Missions[i]
		state := fmt.Sprintf("%d/%d reviewed", item.ReviewedCount, item.ChildCount)
		if item.Approved {
			state = "plan approved"
		}
		if item.Stale {
			state += " · stale revision"
		}
		text := theme.TruncatePlain(theme.SanitizeSingleLine(item.Goal+" · "+state), max(1, width-6))
		lines = append(lines, "  "+th.renderCursorPrefix(i == selected)+rowTitleStyle(th, i == selected)+text+th.reset)
	}
	lines = appendWindowFooter(th, lines, start, end, len(v.Catalog.Missions))
	item := v.Catalog.Missions[selected]
	rows := renderDiagnosticRow(th, th.cyan, "Mission:", th.fg, externalSafeText(item.ID), issuePanelContentWidth(adaptivePanelWidth(width)))
	rows = append(rows, renderDiagnosticRow(th, th.cyan, "Sources:", th.fg, "Unchecked in history. Open to inspect the plan and recheck its bindings.", issuePanelContentWidth(adaptivePanelWidth(width)))...)
	return append(lines, th.renderPanelBox("SELECTED MISSION", "LOCAL HISTORY", adaptivePanelWidth(width), rows)...)
}

func (s *State) missionDetailFields() [][2]string {
	v := s.Mission
	if v.ExecutionOpen {
		return s.missionExecutionFields()
	}
	fields := [][2]string{{"State:", s.missionStateText()}}
	switch s.DetailSection {
	case 1:
		fields = append(fields, [2]string{"Children:", missionChildReviewList(v)}, [2]string{"Integration order:", strings.Join(v.Draft.IntegrationOrder, " → ")}, [2]string{"Select:", "[j/k] Previous/next child · :mission-child <id>"})
		if child := s.missionChild(); child != nil {
			return append(fields, missionChildFields(*child)...)
		}
		return append(fields, [2]string{"Next:", "Use :mission-plan for suggestions, or :mission-child <id> to author locally."})
	case 2:
		return append(fields, [2]string{"Captured inventory:", missionScopeText(v.Draft.Sources)}, [2]string{"Edit:", ":mission-edit files path; path. Child read/write paths must stay inside this inventory."})
	case 3:
		return append(fields, s.missionBudgetFields()...)
	default:
		fields = append(fields, [2]string{"Goal:", v.Draft.Goal}, [2]string{"Current project:", v.Scope.ProjectID + " / " + v.Scope.ProjectRevision}, [2]string{"Integration order:", strings.Join(v.Draft.IntegrationOrder, " → ")})
		if v.Saved != nil {
			fields = append(fields, [2]string{"Stored revision:", v.Saved.Mission.ProjectRevision}, [2]string{"Source status:", v.Saved.SourceStatus}, [2]string{"Plan hash:", v.Saved.Mission.PlanHash})
		}
		return append(fields, [2]string{"Next:", "Inspect children and scope, save local edits, review each child, then approve the exact plan. Approval starts no work."})
	}
}

func (s *State) missionBudgetFields() [][2]string {
	v := s.Mission
	fields := [][2]string{{"Planning run:", missionBudgetText(v.PlanningLimits)}, {"Future mission:", missionBudgetText(v.Draft.Limits)}, {"Allocation:", "Child calls, tools, tokens and repairs sum within the mission. Elapsed time and concurrency are ceilings."}, {"Edit:", ":mission-budget mission|planning|child <field> <value>"}}
	if v.Settings != nil {
		fields = append(fields, [2]string{"Settings:", fmt.Sprintf("%s · version %d", v.Settings.State, v.Settings.Version)}, [2]string{"Effective ceiling:", missionBudgetText(v.Settings.Limits)})
	}
	fields = append(fields, [2]string{"Fields:", "model_calls · tool_calls · input_tokens · output_tokens · elapsed_seconds · concurrent_calls · repairs"})
	if child := s.missionChild(); child != nil {
		fields = append(fields, [2]string{"Selected child " + child.ID + ":", missionBudgetText(child.Limits)})
	}
	if v.PlanningReceipt != nil {
		u := v.PlanningReceipt.Usage
		fields = append(fields, [2]string{"Last planning charge:", fmt.Sprintf("%d model calls · %d tools · %d input / %d output tokens · %d calls without measured usage", u.ModelCalls, u.ToolCalls, u.InputTokens, u.OutputTokens, u.UnknownUsageCalls)})
	}
	return fields
}

func missionChildReviewList(v *missionView) string {
	current := v.Saved != nil && !v.Saved.Stale && missionDigest(v.Draft) == missionDigest(v.Saved.Mission.Plan)
	var rows []string
	for _, child := range v.Draft.Children {
		state := "needs review"
		if current && slices.Contains(v.Saved.Mission.Reviewed, child.ID) {
			state = "reviewed"
		}
		rows = append(rows, child.ID+" · "+child.Title+" · "+state)
	}
	return strings.Join(rows, "\n")
}

func (s *State) handleMissionKey(ev KeyEvent) {
	if ev.Key == "x" {
		s.cancelMissionRequest()
		return
	}
	if !s.missionCurrent() {
		s.handleGlobalNavigationKey(ev)
		return
	}
	if s.Mission.Draft == nil || s.Mission.History {
		s.handleMissionHistoryKey(ev)
		return
	}
	if s.Mission.ExecutionOpen {
		s.handleMissionExecutionKey(ev)
		return
	}
	s.handleMissionDraftKey(ev)
}

func (s *State) handleMissionDraftKey(ev KeyEvent) {
	switch ev.Key {
	case "Enter":
		s.refreshMissionExecution()
	case "n":
		s.openCommandInput("mission-new " + s.Mission.Draft.Goal)
	case "g":
		s.confirmMissionPlanning()
	case "s":
		s.saveMission()
	case "r", "a":
		s.confirmMissionReview(ev.Key == "a")
	case "e":
		s.beginMissionEdit()
	case "j", "ArrowDown", "k", "ArrowUp":
		delta := 1
		if ev.Key == "k" || ev.Key == "ArrowUp" {
			delta = -1
		}
		s.stepMissionChildOrScroll(delta)
	case "Escape":
		s.Mission.History, s.DetailScroll = true, 0
	default:
		s.handleGlobalNavigationKey(ev)
	}
}

func (s *State) handleMissionHistoryKey(ev KeyEvent) {
	v := s.Mission
	switch ev.Key {
	case "j", "ArrowDown", "k", "ArrowUp":
		if v.Catalog != nil {
			delta := 1
			if ev.Key == "k" || ev.Key == "ArrowUp" {
				delta = -1
			}
			v.Selection = min(max(0, v.Selection+delta), max(0, len(v.Catalog.Missions)-1))
		}
	case "Enter":
		if v.Catalog != nil && len(v.Catalog.Missions) > 0 {
			s.openMission(v.Catalog.Missions[v.Selection].ID)
		}
	case "n":
		s.openCommandInput("mission-new ")
	case "r":
		s.queueMissionAction(missionAction{Operation: "read"})
	case "Escape":
		s.Navigate(PageAgents)
	default:
		s.handleGlobalNavigationKey(ev)
	}
}

func (s *State) stepMissionChildOrScroll(delta int) {
	children := s.Mission.Draft.Children
	if s.DetailSection != 1 || len(children) == 0 {
		s.DetailScroll = max(0, s.DetailScroll+delta)
		return
	}
	index := slices.IndexFunc(children, func(child client.MissionChild) bool { return child.ID == s.Mission.ChildID })
	s.Mission.ChildID = children[(max(0, index)+delta+len(children))%len(children)].ID
	s.DetailScroll = 0
}

func (s *State) beginMissionEdit() {
	v := s.Mission
	field, value := "goal", v.Draft.Goal
	switch s.DetailSection {
	case 1:
		if child := s.missionChild(); child != nil {
			field, value = "approach", child.Approach
		} else {
			s.openCommandInput("mission-child ")
			return
		}
	case 2:
		var paths []string
		for _, source := range v.Draft.Sources {
			paths = append(paths, source.Path)
		}
		field, value = "files", strings.Join(paths, "; ")
	case 3:
		s.openCommandInput(fmt.Sprintf("mission-budget mission model_calls %d", v.Draft.Limits.ModelCalls))
		return
	}
	s.openCommandInput("mission-edit " + field + " " + value)
}
