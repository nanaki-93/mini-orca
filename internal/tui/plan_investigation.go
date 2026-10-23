package tui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

func planDraftFingerprint(d *PlanDraft) string {
	if d == nil {
		return ""
	}
	data, _ := json.Marshal(struct {
		Source ProjectIdentity
		Seed   ChangeSeed
	}{d.Source, d.Seed})
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func (s *State) execPlanInvestigationCommand(cmd string) bool {
	switch cmd {
	case "investigate":
		s.startPlanInvestigation(false)
	case "accept-investigation":
		s.acceptPlanInvestigation()
	case "cancel-investigation":
		s.cancelPlanInvestigation()
	default:
		return false
	}
	return true
}

func (s *State) planInvestigationModel(d *PlanDraft) (Model, error) {
	if !s.matchesPlanSource(d.Source) {
		return Model{}, fmt.Errorf("project changed; reopen the plan before investigation")
	}
	if d.Seed.Plan == nil || strings.TrimSpace(d.Seed.Plan.Problem) == "" || strings.TrimSpace(d.Seed.Plan.Approach) == "" || len(d.Seed.Paths) == 0 || len(d.Seed.Paths) > 8 {
		return Model{}, fmt.Errorf("provide a problem, initial approach and 1–8 write paths before investigating")
	}
	if s.Models == nil {
		return Model{}, fmt.Errorf("refresh project model information before investigation")
	}
	model, ok := s.Models.Scopes["function"]
	if !ok {
		return Model{}, fmt.Errorf("configure the Function model before investigation")
	}
	return model, nil
}

func (s *State) startPlanInvestigation(confirmed bool) {
	d := s.PlanDraft
	if d == nil || d.Submitted || d.InvestigationState == "running" {
		return
	}
	model, err := s.planInvestigationModel(d)
	if err != nil {
		s.Error = err.Error()
		return
	}
	fingerprint := planDraftFingerprint(d)
	if model.RemoteProvider && !confirmed {
		s.Confirmation = &Confirmation{Title: "Investigate with remote provider", AcceptLabel: "Investigate", Message: fmt.Sprintf("Send this plan, its selected read/write sources and applicable AGENTS.md guides to %s at %s. The result is a suggestion for review. Source remains read-only.", model.Model, model.ProviderOrigin), OnConfirm: func(state *State) {
			if planDraftFingerprint(state.PlanDraft) != fingerprint {
				state.Error = "Plan changed. Review the current scope before investigation."
				return
			}
			state.startPlanInvestigation(true)
		}}
		return
	}
	data, err := json.Marshal(d.Seed)
	if err != nil {
		s.Error = err.Error()
		return
	}
	var seed ChangeSeed
	if err := json.Unmarshal(data, &seed); err != nil {
		s.Error = err.Error()
		return
	}
	d.InvestigationID = nextTUIReqID("investigation")
	d.InvestigationInput = fingerprint
	d.InvestigationState = "running"
	d.Investigation = nil
	consent := "local"
	if confirmed {
		consent = "remote-confirmed"
	}
	s.enqueueAction(ActionRequest{Kind: "investigate-plan", PreviewID: d.InvestigationID, Source: d.Source, Seed: &seed, Target: consent})
	s.CmdOutput = "Investigating selected source. Draft retained; [x] cancels the request."
}

func (s *State) cancelPlanInvestigation() {
	d := s.PlanDraft
	if d == nil || d.InvestigationState != "running" {
		return
	}
	s.enqueueAction(ActionRequest{Kind: "cancel-plan-investigation", Target: d.InvestigationID})
	d.InvestigationState = "canceled"
	d.InvestigationID = ""
	s.CmdOutput = "Investigation canceled. Draft retained."
}

func (s *State) currentPlanInvestigation() *PlanInvestigation {
	d := s.PlanDraft
	if d == nil || (d.InvestigationState != "ready" && d.InvestigationState != "accepted") || !s.matchesPlanSource(d.Source) || d.InvestigationInput != planDraftFingerprint(d) {
		return nil
	}
	return d.Investigation
}

func (s *State) acceptPlanInvestigation() {
	result := s.currentPlanInvestigation()
	if result == nil || s.PlanDraft.InvestigationState != "ready" {
		s.Error = "Investigate the current plan and review the suggestions before accepting."
		return
	}
	d := s.PlanDraft
	d.Seed.Plan.Approach = result.Approach
	d.Seed.Plan.Citations = slices.Clone(result.Evidence)
	d.Seed.Plan.Assumptions = slices.Clone(result.Assumptions)
	d.Seed.Plan.Evidence = planEvidenceObservations(result.Evidence)
	d.Seed.AcceptanceCriteria = slices.Clone(result.AcceptanceCriteria)
	// Retain suggestions for inspection, with a new draft identity. Suggested
	// files are never promoted into read or write scope by acceptance.
	d.InvestigationInput = planDraftFingerprint(d)
	d.InvestigationState = "accepted"
	if d.Saved != nil && d.Saved.Plan != nil {
		d.Saved.Plan.ApprovedHash = ""
	}
	s.DetailScroll = 0
	s.CmdOutput = "Suggestions accepted into the draft. Review scope and criteria, then [g] approve generation."
}

func planEvidenceObservations(citations []PlanEvidence) string {
	var observations []string
	for _, citation := range citations {
		observations = append(observations, citation.Observation)
	}
	return strings.Join(observations, "\n")
}

func (s *State) clearPlanCitations() {
	p := s.PlanDraft.Seed.Plan
	if len(p.Citations) > 0 {
		p.Citations, p.Evidence = nil, ""
	}
}

func (s *State) prunePlanCitations() bool {
	d := s.PlanDraft
	p := d.Seed.Plan
	before := len(p.Citations)
	p.Citations = slices.DeleteFunc(slices.Clone(p.Citations), func(citation PlanEvidence) bool {
		return !slices.Contains(d.Seed.Paths, citation.Path) && !slices.Contains(p.ReadPaths, citation.Path)
	})
	if len(p.Citations) == before {
		return false
	}
	p.Evidence = planEvidenceObservations(p.Citations)
	return true
}

type planInvestigationTask struct {
	action      ActionRequest
	fingerprint string
	index       ProjectIndex
}

type planInvestigationResult struct {
	task  planInvestigationTask
	value PlanInvestigation
	err   error
}

func (s *State) preparePlanInvestigationTask(action ActionRequest) (planInvestigationTask, bool) {
	d := s.PlanDraft
	if d == nil || d.InvestigationState != "running" || d.InvestigationID != action.PreviewID || action.Seed == nil {
		return planInvestigationTask{}, false
	}
	task := planInvestigationTask{action: action, fingerprint: d.InvestigationInput}
	if action.Seed.WorktreePath == "" {
		if s.Index == nil || s.Index.ProjectIdentity != action.Source {
			d.InvestigationState = "failed"
			s.Error = "Refresh the index before investigation."
			return task, false
		}
		task.index = *s.Index
		task.index.Files = slices.Clone(s.Index.Files)
	}
	return task, true
}

func executePlanInvestigation(ctx context.Context, cl *Client, task planInvestigationTask) planInvestigationResult {
	result := planInvestigationResult{task: task}
	index := task.index
	scope := ""
	if task.action.Seed.WorktreePath != "" {
		scope = "?worktree_path=" + url.QueryEscape(task.action.Seed.WorktreePath)
		if err := cl.DoJSON(ctx, task.action.PreviewID+"-index", http.MethodGet, "/api/projects/current/index"+scope, nil, &index); err != nil {
			result.err = err
			return result
		}
	}
	seed := task.action.Seed
	request := PlanInvestigationRequest{ProjectIdentity: index.ProjectIdentity, Plan: *seed.Plan, Paths: slices.Clone(seed.Paths), AcceptanceCriteria: slices.Clone(seed.AcceptanceCriteria), ConfirmRemoteProvider: task.action.Target == "remote-confirmed"}
	for _, path := range append(slices.Clone(seed.Paths), seed.Plan.ReadPaths...) {
		source := PlanInvestigationSource{Path: path}
		for _, file := range index.Files {
			if file.Path == path {
				source.Hash, source.Exists = file.ContentHash, true
				break
			}
		}
		request.Sources = append(request.Sources, source)
	}
	result.err = cl.DoJSON(ctx, task.action.PreviewID, http.MethodPost, "/api/projects/current/plans/investigate"+scope, request, &result.value)
	if result.err == nil && (result.value.ProjectIdentity != request.ProjectIdentity || result.value.Version != "v1" || result.value.InputHash == "") {
		result.err = fmt.Errorf("planning response does not match the admitted project")
	}
	return result
}

func (s *State) applyPlanInvestigation(result planInvestigationResult) {
	d := s.PlanDraft
	if d == nil || d.InvestigationID != result.task.action.PreviewID {
		return
	}
	d.InvestigationState = "stale"
	if !s.matchesPlanSource(result.task.action.Source) || planDraftFingerprint(d) != result.task.fingerprint {
		s.CmdOutput = "Discarded investigation for an earlier plan. Current draft retained."
		return
	}
	if result.err != nil {
		d.InvestigationState = "failed"
		if errors.Is(result.err, context.Canceled) {
			d.InvestigationState = "canceled"
			s.CmdOutput = "Investigation canceled. Draft retained."
		} else {
			s.Error = "Investigation failed; draft retained: " + result.err.Error()
		}
		return
	}
	d.Investigation = &result.value
	d.InvestigationState = "ready"
	s.CmdOutput = "Investigation ready. Review suggestions in the plan sections; [a] accepts them into the draft."
}

func (s *State) renderInvestigationSuggestions(th ansiTheme, width int) []string {
	result := s.currentPlanInvestigation()
	if result == nil {
		return nil
	}
	if s.PlanDraft.InvestigationState == "accepted" && s.DetailSection != 2 {
		return nil
	}
	rows := []string{"", th.muted + "Investigation · model suggestions" + th.reset}
	switch s.DetailSection {
	case 1:
		return append(rows, renderPlanCitations(th, result.Evidence, result.Assumptions, width)...)
	case 2:
		return append(rows, renderDiagnosticBulletRows(th, th.amber, "Suggested paths (add explicitly):", th.amber, th.fg, result.SuggestedScope, width)...)
	case 3:
		return append(rows, renderDiagnosticBulletRows(th, th.cyan, "Suggested acceptance:", th.cyan, th.fg, result.AcceptanceCriteria, width)...)
	default:
		return append(rows, renderDiagnosticRow(th, th.cyan, "Suggested approach:", th.fg, result.Approach, width)...)
	}
}

func renderPlanCitations(th ansiTheme, citations []PlanEvidence, assumptions []string, width int) []string {
	var rows []string
	for _, citation := range citations {
		location := fmt.Sprintf("%s:%d–%d", citation.Path, citation.StartLine, citation.EndLine)
		rows = append(rows, renderDiagnosticRow(th, th.cyan, location, th.fg, citation.Quote, width)...)
		rows = append(rows, renderDiagnosticRow(th, th.muted, "Model observation:", th.fg, citation.Observation, width)...)
	}
	return append(rows, renderDiagnosticBulletRows(th, th.amber, "Assumptions:", th.amber, th.fg, assumptions, width)...)
}

func (s *State) dispatchPlanInvestigation(ctx context.Context, cl *Client, action ActionRequest) {
	if action.Kind == "cancel-plan-investigation" {
		cl.Cancel(action.Target)
		cl.Cancel(action.Target + "-index")
		return
	}
	if task, ok := s.preparePlanInvestigationTask(action); ok {
		s.applyPlanInvestigation(executePlanInvestigation(ctx, cl, task))
	}
}
