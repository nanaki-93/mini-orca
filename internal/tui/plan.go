package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
)

type PlanDraft struct {
	Connections        []investigationConnection
	Source             ProjectIdentity
	Seed               ChangeSeed
	Saved              *ChangeSession
	Submitted          bool
	PreviewID          string
	Investigation      *PlanInvestigation
	InvestigationInput string
	InvestigationID    string
	InvestigationState string
}

func (s *State) openTaskPlan(seed *ChangeSeed) {
	if seed == nil {
		return
	}
	s.cancelPlanInvestigation()
	s.PlanView = ""
	copy := copyTaskPlanSeed(*seed)
	copy.Paths = domain.EnsureChangeTestPaths(seed.Paths, nil)
	if copy.Plan == nil {
		copy.Plan = &ChangePlanInput{Problem: copy.Title, Approach: copy.Message}
	}
	s.PlanDraft = &PlanDraft{Source: ProjectIdentity{ProjectID: s.projectID(), ProjectRevision: s.projectRevision()}, Seed: copy}
	s.Navigate(PagePlan)
	s.CmdOutput = "Review the approach, scope and criteria. [e] Edit approach · [f] Files · [t] Criteria · [g] Approve & generate · [s] Save draft"
}

func copyTaskPlanSeed(seed ChangeSeed) ChangeSeed {
	copy := seed
	if seed.Plan != nil {
		input := *seed.Plan
		input.NonGoals = append([]string(nil), input.NonGoals...)
		input.ReadPaths = append([]string(nil), input.ReadPaths...)
		input.Citations = append([]PlanEvidence(nil), input.Citations...)
		input.Assumptions = append([]string(nil), input.Assumptions...)
		copy.Plan = &input
	}
	copy.Paths = append([]string(nil), seed.Paths...)
	copy.AcceptanceCriteria = append([]string(nil), seed.AcceptanceCriteria...)
	copy.FindingTargets = append([]FindingTarget(nil), seed.FindingTargets...)
	return copy
}

func (s *State) openCurrentPlan() {
	if s.Change == nil {
		s.Navigate(PagePlan)
		return
	}
	seed := s.buildSeedFromCurrentChange()
	if seed == nil {
		return
	}
	if s.Change.Plan != nil {
		input := s.Change.Plan.ChangePlanInput
		seed.Plan = &input
	}
	if s.Change.Workflow != nil {
		seed.AutoRepair = s.Change.Workflow.AutoRepair
	}
	s.openTaskPlan(seed)
	if len(s.Change.Changes) == 0 && !s.hasRunningWorkflow() && !s.isChangeStale() {
		copy := cloneChangeSessionSnapshot(s.Change)
		s.PlanDraft.Saved = &copy
	}
}

func (s *State) submitTaskPlan(generate bool) {
	s.submitTaskPlanChoice(generate, nil)
}

func (s *State) submitTaskPlanChoice(generate bool, approved *workflowChoice) {
	d := s.PlanDraft
	if d == nil || d.Submitted || d.InvestigationState == "running" {
		return
	}
	if d.Source.ProjectID != s.projectID() || d.Source.ProjectRevision != s.projectRevision() {
		s.Error = "Project changed. Reopen the task and review its current scope before approval."
		return
	}
	if err := validatePlanDraft(d); err != nil {
		s.Error = err.Error()
		return
	}
	savedID := ""
	if d.Saved != nil {
		savedID = d.Saved.ID
	}
	if generate {
		if reason := s.overlappingTask(d.Seed.Paths, savedID); reason != "" {
			s.Error = reason
			return
		}
	}
	seed := copyTaskPlanSeed(d.Seed)
	seed.Message = seed.Plan.Problem + "\nApproved approach: " + seed.Plan.Approach
	var choice *workflowChoice
	if generate {
		var ok bool
		choice, ok = s.approveWorkflowChoice(&seed, d.Saved, approved, func(state *State, choice *workflowChoice) { state.submitTaskPlanChoice(true, choice) })
		if !ok {
			return
		}
	}
	previewID := s.startLocalAgentPreview(&seed)
	d.Submitted, d.PreviewID = true, previewID
	kind := "save-plan"
	if generate {
		kind = "start-agent"
		s.Navigate(PageAgents)
	} else {
		s.Change.Workflow = nil
		s.Change.State = "draft"
	}
	s.enqueueAction(ActionRequest{Kind: kind, PreviewID: previewID, Seed: &seed, Source: d.Source, ExistingChange: d.Saved, WorkflowChoice: choice})
	s.CmdOutput = "Saving the captured plan..."
	if generate {
		s.CmdOutput = "Approved approach; preparing code generation and verification..."
	}
}

func validatePlanDraft(d *PlanDraft) error {
	if d.Seed.Plan == nil || strings.TrimSpace(d.Seed.Plan.Approach) == "" || strings.TrimSpace(d.Seed.Plan.Problem) == "" {
		return fmt.Errorf("provide the problem and approach before approval")
	}
	if len(d.Seed.Paths) == 0 || len(d.Seed.Paths) > 8 {
		return fmt.Errorf("choose 1–8 write paths, including tests, with :scope path; path. Split larger tasks explicitly")
	}
	if len(d.Seed.Plan.ReadPaths) > 8 {
		return fmt.Errorf("choose at most 8 read-only context paths")
	}
	if len(d.Seed.AcceptanceCriteria) == 0 {
		return fmt.Errorf("describe success with :criteria expected behavior; boundary case")
	}
	return nil
}

func (s *State) execPlanCommand(cmd, arg string) bool {
	if s.interceptMissionCommand(cmd, arg) {
		return true
	}
	if s.execPlanInvestigationCommand(cmd) {
		return true
	}
	switch cmd {
	case "replan":
		if s.PlanDraft != nil && !s.PlanDraft.Submitted {
			seed := s.PlanDraft.Seed
			seed.WorktreePath = ""
			s.openTaskPlan(&seed)
			s.clearPlanCitations()
			s.CmdOutput = "New plan based on the current project. Review the evidence, scope and approach before approval. Previous worktree retained in Worktrees."
		}
		return true
	case "plan":
		s.PlanView = ""
		if s.PlanDraft != nil {
			s.Navigate(PagePlan)
		} else {
			s.openCurrentPlan()
		}
		return true
	case "retry":
		if arg == "" && s.Page != PageAgents && s.Page != PageEditor {
			return false
		}
		s.execRetryCommand(arg)
		return true
	case "generate", "approve-plan":
		s.submitTaskPlan(true)
		return true
	case "save-plan":
		s.submitTaskPlan(false)
		return true
	case "approach", "problem", "scope", "context", "criteria", "boundaries", "repairs":
		return s.editPlanField(cmd, arg)
	}
	return false
}

func (s *State) editPlanField(field, value string) bool {
	d := s.PlanDraft
	if d == nil || d.Submitted {
		s.Error = "Open an editable task plan first."
		return true
	}
	p := d.Seed.Plan
	switch field {
	case "approach":
		p.Approach = value
	case "problem":
		p.Problem = value
	case "scope":
		d.Seed.Paths = domain.EnsureChangeTestPaths(splitPlanList(value), nil)
	case "context":
		p.ReadPaths = splitPlanList(value)
	case "criteria":
		d.Seed.AcceptanceCriteria = splitPlanList(value)
	case "boundaries":
		p.NonGoals = splitPlanList(value)
	case "repairs":
		if value != "on" && value != "off" {
			s.Error = "Use :repairs on or :repairs off."
			return true
		}
		d.Seed.AutoRepair = value == "on"
	}
	s.DetailScroll = 0
	s.CmdOutput = "Plan updated locally. [g] Approve & generate · [s] Save draft"
	if s.prunePlanCitations() {
		s.CmdOutput += " · Removed citations outside the selected scope"
	}
	return true
}

func splitPlanList(text string) []string {
	var out []string
	for _, value := range strings.Split(text, ";") {
		value = strings.TrimSpace(value)
		if value != "" && !domain.ContainsString(out, value) {
			out = append(out, value)
		}
	}
	return out
}

func (s *State) handlePlanKey(ev KeyEvent) {
	if s.PlanView == "mission" {
		s.handleMissionKey(ev)
		return
	}
	if s.PlanDraft == nil {
		s.handleGlobalNavigationKey(ev)
		return
	}
	switch ev.Key {
	case "i":
		s.startPlanInvestigation(false)
	case "a":
		s.acceptPlanInvestigation()
	case "x":
		s.cancelPlanInvestigation()
	case "g":
		s.submitTaskPlan(true)
	case "s":
		s.submitTaskPlan(false)
	case "e":
		s.DetailSection, s.DetailScroll = 0, 0
		s.beginPlanEdit("approach", s.PlanDraft.Seed.Plan.Approach)
	case "f":
		s.DetailSection, s.DetailScroll = 2, 0
		s.beginPlanEdit("scope", strings.Join(s.PlanDraft.Seed.Paths, "; "))
	case "t":
		s.DetailSection, s.DetailScroll = 3, 0
		s.beginPlanEdit("criteria", strings.Join(s.PlanDraft.Seed.AcceptanceCriteria, "; "))
	case "j", "ArrowDown":
		s.DetailScroll++
	case "k", "ArrowUp":
		s.DetailScroll = max(0, s.DetailScroll-1)
	case "Escape":
		s.Navigate(PageAgents)
	default:
		s.handleGlobalNavigationKey(ev)
	}
}

func (s *State) beginPlanEdit(field, value string) {
	if s.PlanDraft.Submitted {
		return
	}
	s.openCommandInput(field + " " + value)
}

func (s *State) renderPlanPage(th ansiTheme, width int) []string {
	if s.PlanView == "mission" {
		return s.renderMissionPage(th, width)
	}
	lines := renderPageHeader(th, "Task plan", "[i] Investigate · [s] Save · [g] Approve & generate · [Esc] Back")
	if s.PlanDraft == nil {
		return append(lines, "  Select an issue or feature, or use :agent <request> to prepare a plan.")
	}
	lines = append(lines, s.renderDetailSections(th)...)
	if s.PlanDraft.InvestigationState == "running" {
		lines = append(lines, "  Investigating admitted source… [x] Cancel")
	} else if s.currentPlanInvestigation() != nil && s.PlanDraft.InvestigationState == "ready" {
		lines = append(lines, "  Suggestions ready for review. [a] Accept into draft")
	}
	rows := renderDiagnosticRow(th, th.cyan, "Task:", th.fg, s.PlanDraft.Seed.Title, issuePanelContentWidth(adaptivePanelWidth(width)))
	rows = append(rows, s.renderPlanSection(th, width)...)
	return append(lines, th.renderPanelBox(s.detailSections()[s.DetailSection], "REVIEW BEFORE GENERATION", adaptivePanelWidth(width), rows)...)
}
