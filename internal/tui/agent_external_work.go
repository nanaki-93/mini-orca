package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

type externalRow struct {
	Key          externalKey
	Title, State string
	Stale        bool
}

func (s *State) externalRows() []externalRow {
	if !s.externalCurrent() {
		return nil
	}
	v := s.AgentExternal
	var rows []externalRow
	for _, item := range v.Actions {
		rows = append(rows, externalRow{Key: externalKey{externalAPI, item.ID}, Title: item.IntegrationID, State: item.State, Stale: item.Stale})
	}
	if len(v.Actions) == 0 {
		rows = append(rows, externalRow{Key: externalKey{Kind: externalAPI}, State: "no receipts"})
	}
	if v.Research != nil {
		for _, item := range v.Research.Reports {
			rows = append(rows, externalRow{Key: externalKey{externalResearch, item.ID}, Title: item.Title, State: item.State, Stale: item.Stale})
		}
	}
	if v.Research == nil || len(v.Research.Reports) == 0 {
		state := "no reports"
		if v.Research != nil {
			state = v.Research.State + " · no reports"
		}
		rows = append(rows, externalRow{Key: externalKey{Kind: externalResearch}, State: state})
	}
	for _, item := range v.Delegations {
		rows = append(rows, externalRow{Key: externalKey{externalRemote, item.ID}, Title: item.RemoteID + " · " + item.Kind, State: item.State, Stale: item.Stale})
	}
	if len(v.Delegations) == 0 {
		rows = append(rows, externalRow{Key: externalKey{Kind: externalRemote}, State: "no receipts"})
	}
	return rows
}

func (s *State) selectedExternalKey() externalKey {
	if s.EffectivePage() != PageExternalWork {
		return externalKey{}
	}
	rows := s.externalRows()
	if len(rows) == 0 {
		return externalKey{}
	}
	return rows[s.ClampedIdx()].Key
}

func (s *State) restoreExternalSelection(key externalKey) {
	if s.EffectivePage() != PageExternalWork {
		return
	}
	for i, row := range s.externalRows() {
		if row.Key == key {
			s.SelectedIdx = i
			return
		}
	}
	s.SelectedIdx = s.ClampedIdx()
}

func externalGroupLabel(kind string) string {
	switch kind {
	case externalAPI:
		return "API action"
	case externalResearch:
		return "Research"
	case externalRemote:
		return "Remote advisory"
	default:
		return "Unavailable"
	}
}

func (s *State) externalRowState(row externalRow) string {
	v := s.AgentExternal
	if v.Failures[row.Key.Kind] != "" {
		return "unavailable"
	}
	if !v.Loaded[row.Key.Kind] {
		return "not loaded"
	}
	state := row.State
	if row.Stale {
		state += " · stale"
	}
	return state
}

func (s *State) renderExternalWork(th ansiTheme, width int) []string {
	lines := renderPageHeader(th, "EXTERNAL WORK", "[Enter] Inspect / open report · [r] Local refresh · [n] Research · [d] Delegate · [x] Stop request")
	if !s.externalCurrent() {
		return append(lines, "  History unavailable for this checkout.", "  Use :external-scope main or :external-scope task.")
	}
	v := s.AgentExternal
	label := "MAIN CHECKOUT"
	if v.Scope.WorktreePath != "" {
		label = "TASK WORKTREE"
	}
	lines = append(lines, "  "+th.cyan+theme.TruncatePlain(label+": "+settingsText(v.Path), max(1, width-4))+th.reset)
	if v.Pending != "" {
		lines = append(lines, "  Request pending. [x] stops local transport; refresh for its outcome.")
	}
	rows, selected := s.externalRows(), s.ClampedIdx()
	start, end := windowBounds(len(rows), selected, s.adaptiveWindowRows(10, 4))
	for i := start; i < end; i++ {
		row := rows[i]
		text := externalGroupLabel(row.Key.Kind) + " · " + s.externalRowState(row)
		if row.Title != "" {
			text += " · " + row.Title
		}
		lines = append(lines, "  "+th.renderCursorPrefix(i == selected)+rowTitleStyle(th, i == selected)+theme.TruncatePlain(theme.SanitizeCodeText(text), max(1, width-6))+th.reset)
	}
	if len(rows) > 0 {
		lines = append(lines, s.renderExternalDetails(th, width, rows[selected])...)
	}
	return lines
}

func externalJSONText(data json.RawMessage) string {
	if len(data) == 0 {
		return "none"
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, data, "", "  "); err != nil {
		return "Invalid retained JSON; refresh before acting."
	}
	return externalSafeText(pretty.String())
}

func externalSafeText(text string) string {
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = theme.SanitizeCodeText(lines[i])
	}
	return strings.Join(lines, "\n")
}

func (s *State) renderExternalDetails(th ansiTheme, width int, row externalRow) []string {
	panelW := adaptivePanelWidth(width)
	fields := [][2]string{{"Checkout:", s.AgentExternal.Path}, {"State:", s.externalRowState(row)}}
	if failure := s.AgentExternal.Failures[row.Key.Kind]; failure != "" {
		fields = append(fields, [2]string{"History:", failure})
	}
	if row.Key.ID != "" {
		fields = append(fields, [2]string{"Receipt:", row.Key.ID})
	}
	fields = append(fields, s.externalReceiptDetails(row.Key)...)
	var body []string
	for _, field := range fields {
		body = append(body, renderDiagnosticRow(th, th.cyan, field[0], th.fg, externalSafeText(field[1]), issuePanelContentWidth(panelW))...)
	}
	return append([]string{""}, th.renderPanelBox("SELECTED EXTERNAL WORK", "PgUp/PgDn: details", panelW, body)...)
}

func (s *State) externalReceiptDetails(key externalKey) [][2]string {
	switch key.Kind {
	case externalAPI:
		return s.externalAPIDetails(key.ID)
	case externalResearch:
		return s.externalResearchDetails(key.ID)
	case externalRemote:
		return s.externalRemoteDetails(key.ID)
	}
	return nil
}

func (s *State) externalAPIDetails(id string) [][2]string {
	item := s.apiReceipt(id)
	if item == nil {
		return [][2]string{{"Review:", "API tools stage exact requests here. Browsing never dispatches them."}}
	}
	fields := [][2]string{{"Version:", fmt.Sprint(item.Version)}, {"Request hash:", item.RequestHash}, {"Expires:", item.ExpiresAt.UTC().Format(time.RFC3339)}}
	if s.AgentExternal.Reviewed != (externalReview{ID: item.ID, Hash: item.RequestHash, Version: item.Version}) {
		return append(fields, [2]string{"Inspect:", "Press Enter to disclose the exact private request and outcome. Review may include admitted source."})
	}
	fields = append(fields, [2]string{"Exact request:", externalJSONText(item.Request)})
	if len(item.Response) > 0 {
		fields = append(fields, [2]string{"HTTP status:", fmt.Sprint(item.HTTPStatus)}, [2]string{"Response:", externalJSONText(item.Response)})
	}
	if item.ReportedOutcome != "" {
		fields = append(fields, [2]string{"User reported:", item.ReportedOutcome + " · " + item.Observation})
	}
	ready := s.externalReady(externalAPI) == nil && s.AgentExternal.Pending == ""
	if ready && item.CanApprove && !item.Stale && item.ExpiresAt.After(time.Now()) {
		fields = append(fields, [2]string{"Action:", "[a] / :api-approve · confirm one dispatch of this exact request"})
	}
	if ready && item.CanReconcile {
		fields = append(fields, [2]string{"Recovery:", "[u] / :api-reconcile applied|not_applied|other <observation> · no replay"})
	}
	if item.State == "unknown" || item.State == "pending" {
		fields = append(fields, [2]string{"Outcome:", "Delivery is uncertain. Check the external system, then reconcile when the receipt permits it. No automatic retry."})
	}
	return fields
}

func (s *State) externalResearchDetails(id string) [][2]string {
	v := s.AgentExternal.Research
	if v == nil {
		return [][2]string{{"Research:", "Refresh local history to read availability and retained reports."}}
	}
	fields := [][2]string{{"Search:", v.State + " · " + v.Model}, {"New research:", ":research [--question-only] <question> · captures current plan sources before consent"}}
	for _, report := range v.Reports {
		if report.ID != id {
			continue
		}
		fields = append(fields, [2]string{"Title:", report.Title}, [2]string{"Model:", report.Model}, [2]string{"Created:", report.CreatedAt.UTC().Format(time.RFC3339)}, [2]string{"Report:", "[Enter] / :report-open · open retained HTML on the loopback daemon in your browser. Includes citations and original search suggestions."})
		break
	}
	return append(fields, [2]string{"Evidence:", "Grounded research is advisory. Provider-internal search usage remains unknown."})
}

func (s *State) externalRemoteDetails(id string) [][2]string {
	item := s.remoteReceipt(id)
	if item == nil {
		return [][2]string{{"Inspect:", "Press Enter on a receipt to read its retained private detail. This does not contact the remote agent."}, {"New delegation:", ":delegate <remote-id> <analysis|review> <skill> [--question-only] <question>"}}
	}
	fields := [][2]string{{"Version:", fmt.Sprint(item.Version)}, {"Remote:", item.RemoteID + " · " + item.Kind + " · " + item.Skill}, {"Task:", client.FirstNonEmpty(item.Progress.TaskID, "not retained")}, {"Question:", item.Question}, {"Advisory output:", client.FirstNonEmpty(item.Progress.Text, "none retained")}}
	for _, artifact := range item.Progress.Artifacts {
		state := "partial"
		if artifact.Complete {
			state = "complete"
		}
		fields = append(fields, [2]string{"Advisory artifact:", artifact.Name + " · " + state + "\n" + artifact.Text})
	}
	usage := "Unknown; remote model and search call counts are unavailable."
	if item.Progress.Usage.Source == "remote_reported" {
		usage = fmt.Sprintf("Remote reported: input %d · output %d tokens. Not independently measured; cost unknown.", item.Progress.Usage.InputTokens, item.Progress.Usage.OutputTokens)
	}
	fields = append(fields, [2]string{"Usage:", usage})
	for _, source := range item.Sources {
		fields = append(fields, [2]string{"Captured source:", source.Path + " · " + source.Hash})
	}
	if item.Observation != "" {
		fields = append(fields, [2]string{"User reported:", item.Observation})
	}
	controls := s.availableRemoteControls(*item)
	if len(controls) > 0 {
		fields = append(fields, [2]string{"Controls:", strings.Join(controls, " · ")})
	}
	return append(fields, [2]string{"Authority:", "Remote results do not grant local tools, execution, Apply or merge."})
}

func (s *State) availableRemoteControls(item client.Delegation) []string {
	if s.externalReady(externalRemote) != nil || s.AgentExternal.Pending != "" {
		return nil
	}
	var controls []string
	remote, err := s.remoteControlDestination(item)
	for _, operation := range []string{"refresh", "reconnect", "cancel", "input", "reconcile"} {
		if !remoteControlAllowed(item, operation, time.Now()) {
			continue
		}
		if operation != "reconcile" && (err != nil || operation == "reconnect" && !remote.Inventory.Streaming) {
			continue
		}
		controls = append(controls, ":remote-"+operation)
	}
	return controls
}
