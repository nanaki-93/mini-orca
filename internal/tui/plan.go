package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
)

type PlanDraft struct {
	Source    ProjectIdentity
	Seed      ChangeSeed
	Saved     *ChangeSession
	Submitted bool
	PreviewID string
}

func (s *State) openTaskPlan(seed *ChangeSeed) {
	if seed == nil {
		return
	}
	copy := *seed
	if seed.Plan != nil {
		input := *seed.Plan
		input.NonGoals = append([]string(nil), input.NonGoals...)
		input.ReadPaths = append([]string(nil), input.ReadPaths...)
		copy.Plan = &input
	}
	copy.Paths = domain.EnsureChangeTestPaths(seed.Paths, nil)
	copy.AcceptanceCriteria = append([]string(nil), seed.AcceptanceCriteria...)
	if copy.Plan == nil {
		copy.Plan = &ChangePlanInput{Problem: copy.Title, Approach: copy.Message}
	}
	s.PlanDraft = &PlanDraft{Source: ProjectIdentity{ProjectID: s.projectID(), ProjectRevision: s.projectRevision()}, Seed: copy}
	s.Navigate(PagePlan)
	s.CmdOutput = "Review the approach, scope and criteria. [e] Edit approach · [f] Files · [t] Criteria · [g] Approve & generate · [s] Save draft"
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
	d := s.PlanDraft
	if d == nil || d.Submitted {
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
	seed := d.Seed
	seed.Message = seed.Plan.Problem + "\nApproved approach: " + seed.Plan.Approach
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
	s.enqueueAction(ActionRequest{Kind: kind, PreviewID: previewID, Seed: &seed, Source: d.Source, ExistingChange: d.Saved})
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
	if len(d.Seed.AcceptanceCriteria) == 0 {
		return fmt.Errorf("describe success with :criteria expected behavior; boundary case")
	}
	return nil
}

func (s *State) execPlanCommand(cmd, arg string) bool {
	switch cmd {
	case "plan":
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
	if s.PlanDraft == nil {
		s.handleGlobalNavigationKey(ev)
		return
	}
	switch ev.Key {
	case "g":
		s.submitTaskPlan(true)
	case "s":
		s.submitTaskPlan(false)
	case "e":
		s.beginPlanEdit("approach", s.PlanDraft.Seed.Plan.Approach)
	case "f":
		s.beginPlanEdit("scope", strings.Join(s.PlanDraft.Seed.Paths, "; "))
	case "t":
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
	s.CmdActive, s.CmdInput = true, field+" "+value
}

func (s *State) renderPlanPage(th ansiTheme, width int) []string {
	lines := renderPageHeader(th, "TASK PLAN · REVIEW BEFORE GENERATION", "[e] Approach · [f] Files · [t] Criteria · [s] Save · [g] Approve & generate")
	if s.PlanDraft == nil {
		return append(lines, "  Select an issue or feature, or use :agent <request> to prepare a plan.")
	}
	d := s.PlanDraft
	p := d.Seed.Plan
	contentW := issuePanelContentWidth(adaptivePanelWidth(width))
	rows := renderDiagnosticRow(th, th.bold+th.cyan, "Task:", th.fg, d.Seed.Title, contentW)
	rows = append(rows, renderDiagnosticRow(th, th.bold+th.amber, "Problem / goal:", th.fg, p.Problem, contentW)...)
	rows = append(rows, renderDiagnosticRow(th, th.bold+th.cyan, "Evidence:", th.fg, client.FirstNonEmpty(p.Evidence, "No reproduction evidence recorded; inspect the source and verify the hypothesis."), contentW)...)
	rows = append(rows, renderDiagnosticRow(th, th.bold+th.green, "Approach:", th.fg, p.Approach, contentW)...)
	rows = append(rows, renderDiagnosticBulletRows(th, th.cyan, "Write scope:", th.cyan, th.fg, d.Seed.Paths, contentW)...)
	rows = append(rows, renderDiagnosticBulletRows(th, th.cyan, "Read context:", th.cyan, th.fg, p.ReadPaths, contentW)...)
	rows = append(rows, renderDiagnosticBulletRows(th, th.purple, "Acceptance:", th.purple, th.fg, d.Seed.AcceptanceCriteria, contentW)...)
	rows = append(rows, renderDiagnosticBulletRows(th, th.purple, "Non-goals:", th.purple, th.fg, p.NonGoals, contentW)...)
	repairs := "Off — inspect failures before retrying"
	if d.Seed.AutoRepair {
		repairs = "On — up to 2 repairs within the approved scope"
	}
	rows = append(rows, renderDiagnosticRow(th, th.amber, "Auto repair:", th.fg, repairs, contentW)...)
	if s.Change != nil && len(s.Change.ScopeSuggestions) > 0 {
		rows = append(rows, renderDiagnosticBulletRows(th, th.amber, "Suggested scope (review and add explicitly):", th.amber, th.fg, s.Change.ScopeSuggestions, contentW)...)
	}
	rows = append(rows, "Related sibling/import declarations and applicable AGENTS.md guides may also be read under the project context policy.", "Edit: :problem · :approach · :scope · :context · :criteria · :boundaries · :repairs on|off", "Separate paths, criteria and boundaries with semicolons. [PgUp/PgDn] Scroll")
	return append(lines, th.renderPanelBox("EDITABLE PLAN", "NO CODE GENERATED", adaptivePanelWidth(width), rows)...)
}
