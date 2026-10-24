package tui

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
)

type agentRunAction struct {
	Export                              client.AgentTraceExportRequest
	ExportCount                         int
	ID, Operation, Invocation, Decision string
	Request                             agentProgressRequest
	Change                              ChangeSession
}

type agentDisclosure struct {
	Request    agentProgressRequest
	Invocation string
	Detail     *AgentRunDetail
	Status     string
}

type agentRunActionResult struct {
	Export client.AgentTraceExportReceipt
	Action agentRunAction
	Detail AgentRunDetail
	Change ChangeSession
	Err    error
}

func (s *State) acceptsAgentRunAction(action *agentRunAction) bool {
	if action == nil || action.ID != s.AgentRunPending {
		return false
	}
	current := s.matchesAgentRequest(action.Request)
	if current && action.Operation == "export" && !s.matchesTraceExport(*action) {
		current = false
		s.Error = "Trace or destination changed. Inspect and confirm the current snapshot again."
	}
	if action.Operation == "output" && (s.AgentDisclosure == nil || s.AgentDisclosure.Invocation != action.Invocation) {
		current = false
	}
	if !current {
		s.AgentRunPending = ""
	}
	return current
}

func (s *State) execAgentRunCommand(command, arg string) bool {
	switch command {
	case "execution":
		s.Navigate(PageAgents)
		s.AgentDetailOpen = true
		s.AgentRunSection = 0
		s.enqueueAgentInspection()
	case "run-output":
		s.queueAgentRunAction("output", strings.TrimSpace(arg))
	case "hide-output":
		s.AgentDisclosure = nil
		s.CmdOutput = "Private output hidden."
	case "run-cancel":
		s.queueAgentRunAction("cancel", "")
	case "run-input":
		s.queueAgentRunAction("input", strings.TrimSpace(arg))
	case "run-export":
		s.requestTraceExport(arg)
	case "retry":
		if !s.hasRunningWorkflow() {
			return false
		}
		s.CmdOutput = "The workflow is running. Cancel it before requesting a stage retry."
	case "stream-output":
		if arg != "on" && arg != "off" {
			s.Error = "Use :stream-output on or :stream-output off."
			return true
		}
		s.StreamAgentOutput = arg == "on"
		s.CmdOutput = "Output streaming " + arg + " for new task workflows and retries; provider capability and consent checks still apply."
	case "pause", "resume":
		if s.Page != PageAgents || !s.AgentDetailOpen {
			return false
		}
		s.CmdOutput = "Task pause/resume is unavailable. Cancel active work, then use a guarded stage retry. Analysis pause/resume is available in Project."
	default:
		return false
	}
	return true
}

func (s *State) matchesAgentRequest(request agentProgressRequest) bool {
	return s.Project != nil && s.Project.ProjectIdentity == request.Parent && s.Change != nil && s.Change.Workflow != nil && s.Change.ID == request.ChangeID && s.Change.Workflow.ID == request.WorkflowID && s.Change.ProjectIdentity == request.Project && s.Change.WorktreePath == request.WorktreePath
}

func (s *State) currentAgentDetail() *AgentRunDetail {
	if s.AgentProgress == nil || s.AgentProgress.Status != "current" || !s.matchesAgentRequest(s.AgentProgress.Request) || !domain.NativeProgressMatches(s.Change, s.AgentProgress.Detail) {
		return nil
	}
	return s.AgentProgress.Detail
}

func (s *State) queueAgentRunAction(operation, arg string) {
	detail := s.currentAgentDetail()
	if detail == nil {
		s.Error = "Inspect the current task to load its native execution identity first."
		return
	}
	if s.AgentRunPending != "" {
		s.CmdOutput = "An execution request is already pending."
		return
	}
	request := *s.agentProgressRequest()
	action := agentRunAction{ID: nextTUIReqID("run-action"), Operation: operation, Request: request, Change: cloneChangeSessionSnapshot(s.Change)}
	if err := prepareAgentRunAction(&action, detail, arg); err != nil {
		s.Error = err.Error()
		return
	}
	s.AgentRunPending = action.ID
	if operation == "output" {
		s.AgentDisclosure = &agentDisclosure{Request: request, Invocation: action.Invocation, Status: "loading"}
		s.AgentRunSection = 2
	}
	s.enqueueAction(ActionRequest{Kind: "agent-run", AgentRun: &action})
}

func prepareAgentRunAction(action *agentRunAction, detail *AgentRunDetail, arg string) error {
	switch action.Operation {
	case "output":
		if arg != detail.Run.ID && !slices.ContainsFunc(detail.Invocations, func(run AgentRun) bool {
			return run.ID == arg && run.RootID == detail.Run.ID && action.Request.matchesInvocation(run)
		}) {
			return fmt.Errorf("use :run-output <invocation ID> from this task's Timeline")
		}
		action.Invocation = arg
	case "cancel":
		if !detail.Run.CanCancel {
			return fmt.Errorf("this native run cannot be canceled; inspect pending input or recovery")
		}
		action.Invocation = detail.Run.ID
	case "input":
		input := currentAgentInput(&action.Change)
		if input == nil {
			return fmt.Errorf("this proposal has no current pending input")
		}
		if !slices.Contains([]string{"approve", "reject", "cancel"}, arg) {
			return fmt.Errorf("use :run-input approve, reject or cancel")
		}
		action.Invocation, action.Decision = input.RunID, arg
	default:
		return fmt.Errorf("unsupported execution action")
	}
	return nil
}

func (request agentProgressRequest) matchesInvocation(run AgentRun) bool {
	return run.ChangeSessionID == request.ChangeID && run.WorkflowID == request.WorkflowID && run.ProjectID == request.Project.ProjectID && run.ProjectRevision == request.Project.ProjectRevision
}

func currentAgentInput(change *ChangeSession) *client.ChangeInput {
	if change == nil || change.Workflow == nil || change.Workflow.Input == nil {
		return nil
	}
	input := change.Workflow.Input
	if input.Status != "waiting" || input.Hash != change.Hash || input.Revision != int64(change.Revision) || input.WorkflowID != change.Workflow.ID || input.ProjectID != change.ProjectID || input.ProjectRevision != change.ProjectRevision {
		return nil
	}
	return input
}

func executeAgentRunAction(ctx context.Context, cl *Client, action agentRunAction) agentRunActionResult {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	result := agentRunActionResult{Action: action}
	target := "/api/projects/current/agent-runs/" + url.PathEscape(action.Invocation)
	switch action.Operation {
	case "output":
		result.Err = cl.DoJSON(ctx, action.ID, http.MethodGet, target+action.Request.Query+"&include_text=true", nil, &result.Detail)
	case "export":
		result.Err = cl.DoJSON(ctx, action.ID, http.MethodPost, target+"/export"+changeWorktreeScopeQuery(&action.Change), action.Export, &result.Export)
	case "cancel":
		body := map[string]any{"project_id": action.Request.Project.ProjectID, "project_revision": action.Request.Project.ProjectRevision, "action": "cancel"}
		result.Err = cl.DoJSON(ctx, action.ID, http.MethodPost, target+"/control"+changeWorktreeScopeQuery(&action.Change), body, &result.Detail)
	case "input":
		input := currentAgentInput(&action.Change)
		if input == nil {
			result.Err = fmt.Errorf("pending input no longer matches the captured proposal")
			return result
		}
		body := map[string]any{"project_id": input.ProjectID, "project_revision": input.ProjectRevision, "revision": input.Revision, "hash": input.Hash, "workflow_id": input.WorkflowID, "request_id": input.RequestID, "decision": action.Decision}
		result.Err = cl.DoJSON(ctx, action.ID, http.MethodPost, target+"/input"+changeWorktreeScopeQuery(&action.Change), body, &result.Change)
	default:
		result.Err = fmt.Errorf("unsupported execution action")
	}
	return result
}

func (s *State) applyAgentRunAction(result agentRunActionResult) {
	action := result.Action
	if action.Operation == "export" {
		s.applyTraceExportAction(result)
		return
	}
	if s.AgentRunPending == action.ID {
		s.AgentRunPending = ""
	}
	if !s.matchesAgentRequest(action.Request) {
		return
	}
	if action.Operation == "output" && (s.AgentDisclosure == nil || s.AgentDisclosure.Invocation != action.Invocation || s.AgentDisclosure.Request != action.Request) {
		return
	}
	if result.Err != nil {
		s.Error = result.Err.Error()
		if action.Operation == "output" {
			s.AgentDisclosure = nil
		}
		return
	}
	switch action.Operation {
	case "output":
		s.applyAgentDisclosure(action.Request, action.Invocation, &result.Detail)
	case "cancel":
		if !action.Request.matchesRun(result.Detail.Run) || result.Detail.Text != nil {
			s.Error = "Cancellation receipt belongs to a different run."
			return
		}
		s.applyAgentProgress(&agentProgressView{Request: action.Request, Detail: &result.Detail, Status: "current"})
		s.CmdOutput = "Cancellation requested; the worker is finishing cleanup."
	case "input":
		s.applyAgentInputResult(result)
	}
}

func (s *State) applyAgentDisclosure(request agentProgressRequest, id string, detail *AgentRunDetail) {
	current := s.AgentDisclosure
	if current == nil || current.Invocation != id || current.Request != request || !s.matchesAgentRequest(request) {
		return
	}
	if detail.Run.ID != id || detail.Run.RootID != request.RunID || !request.matchesInvocation(detail.Run) {
		s.AgentDisclosure = nil
		s.Error = "Private output belongs to a different invocation."
		return
	}
	s.AgentDisclosure = &agentDisclosure{Request: request, Invocation: id, Detail: detail, Status: "current"}
}

func (s *State) applyAgentInputResult(result agentRunActionResult) {
	action, change := result.Action, result.Change
	if s.Change.Hash != action.Change.Hash || s.Change.Revision != action.Change.Revision {
		return
	}
	if change.ID != action.Change.ID || change.ProjectIdentity != action.Change.ProjectIdentity || change.Hash != action.Change.Hash || change.Revision != action.Change.Revision || change.Workflow == nil || change.Workflow.ID != action.Request.WorkflowID {
		s.Error = "Input receipt no longer matches the proposal."
		return
	}
	change.WorktreePath = action.Change.WorktreePath
	input := change.Workflow.Input
	want := map[string]string{"approve": "approved", "reject": "rejected", "cancel": "canceled"}[action.Decision]
	if input == nil || input.RequestID != action.Change.Workflow.Input.RequestID || input.Status != want {
		s.Error = "Input receipt does not confirm the requested decision."
		return
	}
	s.Change = &change
	s.upsertActiveChange(&change)
	s.CmdOutput = "Input decision recorded. Apply remains a separate guarded action."
}
