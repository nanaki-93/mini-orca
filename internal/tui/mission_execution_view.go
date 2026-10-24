package tui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

func missionExecutionDisclosure(v *missionView, action missionAction) (string, string) {
	request := action.Execution
	title := "Confirm mission " + strings.ReplaceAll(request.Action, "_", " ")
	message := fmt.Sprintf("Owner checkout: %s\nProject: %s\nRevision: %s\nMission: %s\nPlan hash: %s\n\n", v.Path, v.Scope.ProjectID, v.Scope.ProjectRevision, action.MissionID, request.PlanHash)
	if v.Execution != nil {
		record := v.Execution.Execution
		message += fmt.Sprintf("Execution: %s / revision %d\nState: %s · owner %s\nCurrent base: %s @ %s\nApproved order: %s\nReady children: %s\n\n", record.ID, record.Revision, record.State, v.Execution.OwnerState, record.Current.Base.Branch, record.Current.Base.Head, strings.Join(record.Mission.Plan.IntegrationOrder, " → "), strings.Join(v.Execution.ReadyChildren, ", "))
	}
	limits := v.Draft.Limits
	if v.Execution != nil {
		limits = v.Execution.Execution.Mission.Plan.Limits
	}
	message += "Captured source scope:\n" + missionScopeText(action.Sources) + "\n\nFixed mission allocation: " + missionBudgetText(limits) + "\n"
	if request.Action == "start" {
		models, _ := missionExecutionModels(v.Settings) // The action builder already validated these selections.
		for i, model := range models {
			message += fmt.Sprintf("%s: %s · %s · %s · remote sharing %t\n", []string{"Creation", "Testing", "Review"}[i], client.FirstNonEmpty(model.Profile, model.Scope), model.Model, model.ProviderOrigin, model.RemoteProvider)
		}
		message += "\nOnly ready children start, in owned worktrees. The displayed models receive the approved goal, paths, context and applicable instructions. Failed or interrupted allocations are not reset. Review and Apply remain explicit.\n"
	}
	if slices.Contains([]string{"start", "revalidate", "trust"}, request.Action) {
		message += "\nProject check commands:\n"
		for _, command := range v.Trust.Commands {
			message += strings.Join(command, " ") + "\n"
		}
	}
	if request.Integration != nil {
		input := request.Integration
		message += fmt.Sprintf("\nChild: %s\nProposal: %s / revision %d / %s\nSource: %s\nCommit: %s @ %s\nIntegration receipt: %s\n", input.ChildID, input.SessionID, input.ProposalRevision, input.ProposalHash, input.Path, input.Source.Branch, input.Source.Head, input.IntegrationID)
		message += "\nIntegration observes the exact committed proposal at the displayed destination. Recovery inspects an existing outcome without repeating the merge. Cleanup removes only a proven integrated checkout and branch after its protected proposal is retained."
	}
	if request.Revalidation != nil {
		input := request.Revalidation
		message += fmt.Sprintf("\nPrevious proposal: %s / revision %d / %s\nPrevious checkout: %s\nSource: %s @ %s\nRevalidation receipt: %s\n", input.Previous.SessionID, input.Previous.ProposalRevision, input.Previous.ProposalHash, input.Previous.Path, input.Previous.Source.Branch, input.Previous.Source.Head, input.RevalidationID)
		message += "\nRevalidation carries proposal content into a new owned checkout and runs fresh checks within the remaining elapsed allowance. It invokes no provider. Old checks and approval do not transfer; review every file and Apply the new candidate explicitly. Recovery records interruption without replaying checks."
	}
	switch request.Action {
	case "prepare":
		message += "\nReserve this exact approved plan and its allocations. Preparation starts no models or checks and writes no source."
	case "pause":
		message += "\nCancel the active wave and stop admitting children. Interrupted child allocations remain consumed; a later start can admit only unstarted children."
	case "cancel":
		message += "\nStop this mission permanently. Keep its proposals, branches and receipts for inspection; do not apply or delete source."
	case "reconcile":
		message += "\nMark the abandoned wave interrupted and charge its reserved elapsed allowance. Do not restart children or invoke providers."
	case "trust":
		message += "\nTrust the displayed check commands for this checkout and revision. This records consent; it does not execute them or start models."
	}
	return title, message
}

func (s *State) missionExecutionFields() [][2]string {
	v := s.Mission
	if !v.ExecutionLoaded && v.Execution == nil {
		return [][2]string{{"State:", "INSPECTION REQUIRED"}, {"Next:", ":mission-execution reads the current owner, execution and check authority."}}
	}
	if v.Execution == nil {
		return missionUnpreparedFields(v)
	}
	r := v.Execution.Execution
	state := strings.ToUpper(r.State)
	if v.Execution.Stale {
		state += " · STALE / " + v.Execution.SourceStatus
	}
	fields := [][2]string{{"State:", state}, {"Owner:", v.Execution.OwnerState}}
	if !v.ExecutionLoaded {
		fields = append(fields, [2]string{"Next:", "[r] Refresh current commands and settings after the Git operation."})
	}
	if v.ProgressFailure != "" {
		fields = append(fields, [2]string{"Progress:", v.ProgressFailure + " · [r] Refresh"})
	}
	if r.Failure != "" {
		fields = append(fields, [2]string{"Failure:", r.Failure})
	}
	switch s.DetailSection {
	case 1:
		var rows []string
		for _, child := range r.Children {
			marker := "  "
			if child.ID == v.ChildID {
				marker = "▸ "
			}
			rows = append(rows, marker+child.ID+" · "+child.State+fmt.Sprintf(" · candidate %d", child.Generation))
		}
		fields = append(fields, [2]string{"Children:", strings.Join(rows, "\n")}, [2]string{"Approved order:", strings.Join(r.Mission.Plan.IntegrationOrder, " → ")})
		return append(fields, s.missionExecutionChildFields()...)
	case 2:
		return append(fields, [2]string{"Owner checkout:", v.Path}, [2]string{"Current base:", r.Current.Base.Branch + " @ " + r.Current.Base.Head}, [2]string{"Current sources:", missionScopeText(r.Current.Sources)}, [2]string{"Original base:", r.Base.Branch + " @ " + r.Base.Head}, [2]string{"Plan hash:", r.Mission.PlanHash})
	case 3:
		return append(fields, missionExecutionBudgetFields(r)...)
	default:
		fields = append(fields, [2]string{"Goal:", r.Mission.Plan.Goal}, [2]string{"Execution:", fmt.Sprintf("%s / revision %d", r.ID, r.Revision)}, [2]string{"Destination:", v.Path}, [2]string{"Current base:", r.Current.Base.Branch + " @ " + r.Current.Base.Head}, [2]string{"Ready:", client.FirstNonEmpty(strings.Join(v.Execution.ReadyChildren, ", "), "No unstarted child is ready")}, [2]string{"Recovery:", s.missionRecoveryText()})
		return fields
	}
}

func (s *State) missionExecutionChildFields() [][2]string {
	child, v := s.selectedMissionExecutionChild(), s.Mission
	if child == nil {
		return nil
	}
	fields := [][2]string{{"Selected child:", child.ID}, {"Child state:", child.State}, {"Reason:", client.FirstNonEmpty(child.Reason, "None recorded")}}
	if spec := s.missionChild(); spec != nil {
		fields = append(fields, missionChildFields(*spec)...)
	}
	if child.Directory != "" {
		fields = append(fields, [2]string{"Checkout:", missionCandidatePath(v.Path, *child)}, [2]string{"Checked base:", child.BaseHead}, [2]string{"Proposal:", fmt.Sprintf("%s / revision %d / %s", child.SessionID, child.ProposalRevision, child.ProposalHash)})
	}
	if child.Generation > 0 {
		fields = append(fields, [2]string{"Verification:", "Carried content with fresh checks. Original model history is historical; fresh per-file Review and Apply are required."})
	}
	for _, entry := range v.Execution.Execution.Revalidations {
		if entry.Previous.ID == child.ID {
			fields = append(fields, [2]string{fmt.Sprintf("Revalidation %d:", entry.Candidate.Generation), entry.State + " · " + entry.Failure + "\n" + missionEvidenceText(entry.Evidence)})
		}
	}
	for _, entry := range v.Execution.Execution.Integrations {
		if entry.ChildID == child.ID {
			fields = append(fields, [2]string{"Integration:", entry.ID + " · " + entry.State + " · cleanup " + entry.Cleanup + "\n" + entry.Failure + "\n" + missionEvidenceText(entry.Evidence)})
		}
	}
	return fields
}

func missionEvidenceText(evidence client.MissionProposalEvidence) string {
	return fmt.Sprintf("Protected proposal: %s / version %d / %s", evidence.Key.Name, evidence.Version, evidence.Hash)
}

func (s *State) missionRecoveryText() string {
	v, r := s.Mission, s.Mission.Execution.Execution
	if v.Execution.Stale {
		return "Current checkpoint no longer matches. Retain evidence and create a new reviewed mission for unrelated source changes."
	}
	if v.Execution.OwnerState == "waiting_for_owner" {
		return "Owner lease is active. Refresh to inspect progress; recovery cannot steal it."
	}
	if v.Execution.OwnerState == "recovery_required" {
		if r.Wave != nil {
			return ":mission-reconcile records the interrupted wave without restarting it."
		}
		for _, entry := range r.Revalidations {
			if entry.State == "pending" {
				return ":mission-recover-revalidation retains the candidate and records interruption without rerunning checks."
			}
		}
		return ":mission-recover-integration inspects the actual commit without repeating the merge."
	}
	return "Inspect the selected proposal, review each file and Apply/commit. Revalidate older bases before integration. :mission-cleanup retries cleanup of a proven integrated child. Failed allocations cannot be reset."
}

func (s *State) missionExecutionKeys() []keyDesc {
	v := s.Mission
	if !v.ExecutionLoaded {
		return []keyDesc{{"r", "Inspect execution"}, {"Esc", "History"}}
	}
	if v.Execution == nil {
		return []keyDesc{{"p", "Prepare"}, {"r", "Refresh"}, {"Esc", "History"}}
	}
	if v.Execution.Execution.State == "running" {
		return []keyDesc{{"P", "Pause"}, {"X", "Stop mission"}, {"r", "Refresh"}}
	}
	if v.Execution.OwnerState == "recovery_required" {
		return []keyDesc{{"c", "Recover"}, {"r", "Refresh"}, {"Esc", "History"}}
	}
	if v.Trust != nil && !v.Trust.Trusted {
		return []keyDesc{{"t", "Trust checks"}, {"Enter", "Proposal"}, {"r", "Refresh"}}
	}
	if len(v.Execution.ReadyChildren) > 0 {
		return []keyDesc{{"g", "Start ready children"}, {"Enter", "Proposal"}, {"r", "Refresh"}}
	}
	return []keyDesc{{"Enter", "Proposal"}, {"a", "Integrate"}, {"r", "Refresh"}}
}

func (s *State) handleMissionExecutionKey(ev KeyEvent) {
	commands := map[string]string{"r": "mission-execution", "p": "mission-prepare", "g": "mission-start", "P": "mission-pause", "X": "mission-stop", "t": "mission-trust", "Enter": "mission-proposal", "a": "mission-integrate", "v": "mission-revalidate"}
	if command, ok := commands[ev.Key]; ok {
		s.execMissionExecutionCommand(command)
		return
	}
	switch ev.Key {
	case "c":
		if s.Mission.Execution == nil {
			return
		}
		command := "mission-recover-integration"
		if s.Mission.Execution.Execution.Wave != nil {
			command = "mission-reconcile"
		} else {
			for _, entry := range s.Mission.Execution.Execution.Revalidations {
				if entry.State == "pending" {
					command = "mission-recover-revalidation"
				}
			}
		}
		s.execMissionExecutionCommand(command)
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

func missionUnpreparedFields(v *missionView) [][2]string {
	state, next := "PLAN REVIEW REQUIRED", ":mission-draft returns to plan review."
	if v.Saved.Mission.Approved {
		state, next = "APPROVED · NOT PREPARED", "[p] Prepare the fixed allocation. Preparation runs no models or checks."
	}
	if v.Saved.Stale {
		state, next = "STALE PLAN", "Create a new reviewed mission with current source bindings."
	}
	return [][2]string{{"State:", state}, {"Goal:", v.Draft.Goal}, {"Destination:", v.Path}, {"Base:", v.Repository.Branch + " @ " + v.Repository.Head}, {"Next:", next}}
}

func missionExecutionBudgetFields(r client.MissionExecution) [][2]string {
	var fields [][2]string
	unstarted := client.AgentLimits{}
	for _, child := range r.Children {
		if child.State == "pending" {
			unstarted.ModelCalls += child.Limits.ModelCalls
			unstarted.ToolCalls += child.Limits.ToolCalls
			unstarted.InputTokens += child.Limits.InputTokens
			unstarted.OutputTokens += child.Limits.OutputTokens
			unstarted.Repairs += child.Limits.Repairs
		}
	}
	fields = append(fields, [2]string{"Fixed allocation:", missionBudgetText(r.Mission.Plan.Limits)}, [2]string{"Unstarted limits:", fmt.Sprintf("%d model calls · %d tools · %d input / %d output tokens · %d repairs", unstarted.ModelCalls, unstarted.ToolCalls, unstarted.InputTokens, unstarted.OutputTokens, unstarted.Repairs)}, [2]string{"Elapsed charged:", fmt.Sprintf("%.1fs of %ds", float64(r.ElapsedMillis)/1000, r.Mission.Plan.Limits.ElapsedSeconds)}, [2]string{"Usage:", "Allocations are ceilings. Inspect each original child's execution history for measured model usage. Revalidation makes no model calls."})
	if r.Wave != nil {
		fields = append(fields, [2]string{"Active elapsed reservation:", fmt.Sprintf("%.1fs · charged on completion or recovery", float64(r.Wave.AllowanceMillis)/1000)})
	}
	return fields
}
