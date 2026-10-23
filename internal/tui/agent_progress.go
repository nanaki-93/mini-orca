package tui

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
)

type agentProgressRequest struct {
	Parent, Project                                  ProjectIdentity
	ChangeID, WorkflowID, WorktreePath, RunID, Query string
}

type agentProgressView struct {
	Request       agentProgressRequest
	Detail        *AgentRunDetail
	Status, Error string
}

func (s *State) agentProgressRequest() *agentProgressRequest {
	chg := s.Change
	if s.Project == nil || chg == nil || chg.Workflow == nil || chg.ProjectID == "" || chg.ProjectRevision == "" || chg.Workflow.ID == "" {
		return nil
	}
	request := &agentProgressRequest{Parent: s.Project.ProjectIdentity, Project: chg.ProjectIdentity, ChangeID: chg.ID, WorkflowID: chg.Workflow.ID, WorktreePath: chg.WorktreePath, Query: s.changeResourceQuery(chg)}
	if s.AgentProgress != nil && s.AgentProgress.Detail != nil && domain.NativeProgressMatches(chg, s.AgentProgress.Detail) {
		request.RunID = s.AgentProgress.Detail.Run.ID
	}
	return request
}

func readAgentProgress(ctx context.Context, cl *Client, request agentProgressRequest) *agentProgressView {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	result := &agentProgressView{Request: request, Status: "unavailable"}
	id, err := resolveProgressRun(ctx, cl, request)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	if id == "" {
		result.Status = "not_recorded"
		return result
	}
	var detail AgentRunDetail
	if err := cl.DoJSON(ctx, nextTUIReqID("progress-detail"), http.MethodGet, "/api/projects/current/agent-runs/"+url.PathEscape(id)+request.Query, nil, &detail); err != nil {
		result.Error = err.Error()
		return result
	}
	if detail.Run.ID != id || !request.matchesRun(detail.Run) || detail.Text != nil {
		result.Error = "Run detail does not match this workflow."
		return result
	}
	result.Detail, result.Status = &detail, "current"
	return result
}

func resolveProgressRun(ctx context.Context, cl *Client, request agentProgressRequest) (string, error) {
	if request.RunID != "" {
		return request.RunID, nil
	}
	var history AgentRunList
	if err := cl.DoJSON(ctx, nextTUIReqID("progress-list"), http.MethodGet, "/api/projects/current/agent-runs"+request.Query+"&limit=100", nil, &history); err != nil {
		return "", err
	}
	if history.ProjectID != request.Project.ProjectID || history.ProjectRevision != request.Project.ProjectRevision {
		return "", fmt.Errorf("run history belongs to a different project revision")
	}
	for _, run := range history.Runs {
		if request.matchesRun(run) {
			return run.ID, nil
		}
	}
	return "", nil
}

func (r agentProgressRequest) matchesRun(run AgentRun) bool {
	return run.ID != "" && run.ID == run.RootID && run.ChangeSessionID == r.ChangeID && run.WorkflowID == r.WorkflowID && run.ProjectID == r.Project.ProjectID && run.ProjectRevision == r.Project.ProjectRevision
}

func (s *State) applyAgentProgress(result *agentProgressView) {
	if result == nil || s.Project == nil || s.Project.ProjectIdentity != result.Request.Parent || s.Change == nil || s.Change.Workflow == nil {
		return
	}
	chg, req := s.Change, result.Request
	if chg.ID != req.ChangeID || chg.Workflow.ID != req.WorkflowID || chg.ProjectIdentity != req.Project || chg.WorktreePath != req.WorktreePath {
		return
	}
	if result.Detail != nil && s.AgentProgress != nil && domain.NativeProgressMatches(chg, s.AgentProgress.Detail) && progressSequence(result.Detail) < progressSequence(s.AgentProgress.Detail) {
		return
	}
	s.AgentProgress = result
}

func progressSequence(detail *AgentRunDetail) uint64 {
	var sequence uint64
	if detail != nil {
		for _, event := range detail.Events {
			sequence = max(sequence, event.Sequence)
		}
	}
	return sequence
}

func (s *State) refreshAgentProgress(ctx context.Context, cl *Client) {
	if request := s.agentProgressRequest(); request != nil {
		s.applyAgentProgress(readAgentProgress(ctx, cl, *request))
	}
}

func (s *State) enqueueAgentInspection() {
	if s.Change != nil && s.Change.ID != "" && !isLocalPreviewChangeID(s.Change.ID) {
		s.enqueueAction(ActionRequest{Kind: "load-change", Target: s.Change.ID})
	}
}

func (s *State) activeAgentProgressRequest() *agentProgressRequest {
	if !s.AgentDetailOpen {
		return nil
	}
	return s.agentProgressRequest()
}

func (s *State) currentAgentStages() []domain.AgentStage {
	var detail *AgentRunDetail
	if s.AgentProgress != nil {
		detail = s.AgentProgress.Detail
	}
	return domain.WorkflowStages(s.Change, detail)
}

func (s *State) activeAgentStageNumber() int { return domain.ActiveStage(s.currentAgentStages()) + 1 }

func (s *State) effectiveAgentDetailStage() int {
	stages := s.currentAgentStages()
	if index := slices.IndexFunc(stages, func(stage domain.AgentStage) bool { return stage.ID == s.AgentStageID }); index >= 0 {
		return index + 1
	}
	if s.AgentStage > 0 && s.AgentStage <= len(stages) {
		return s.AgentStage
	}
	return domain.ActiveStage(stages) + 1
}

func (s *State) selectActiveAgentStage() {
	s.AgentStageID = ""
	s.selectAgentStage(s.activeAgentStageNumber())
}

func (s *State) selectAgentStage(position int) {
	stages := s.currentAgentStages()
	if len(stages) == 0 {
		s.AgentStage, s.AgentStageID = 0, ""
		return
	}
	position = min(len(stages), max(1, position))
	s.AgentStage, s.AgentStageID = position, stages[position-1].ID
}

func (s *State) agentStageSelectionLabel() string {
	stages := s.currentAgentStages()
	if len(stages) == 0 {
		return "No recorded stages"
	}
	position := s.effectiveAgentDetailStage()
	return fmt.Sprintf("Selected: %d/%d · %s", position, len(stages), stages[position-1].Label)
}
