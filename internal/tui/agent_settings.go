package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

type settingsRole struct{ Name, Scope string }
type settingsBudget struct {
	Name           string
	Value, Ceiling int64
}

func settingsRoleRows(data *client.AgentSettings) []settingsRole {
	return []settingsRole{{"creation", data.Roles.Creation}, {"testing", data.Roles.Testing}, {"review", data.Roles.Review}}
}

func settingsBudgetRows(data *client.AgentSettings) []settingsBudget {
	l, c := data.Limits, data.Ceilings
	return []settingsBudget{
		{"model_calls", int64(l.ModelCalls), int64(c.ModelCalls)}, {"tool_calls", int64(l.ToolCalls), int64(c.ToolCalls)},
		{"input_tokens", l.InputTokens, c.InputTokens}, {"output_tokens", l.OutputTokens, c.OutputTokens},
		{"elapsed_seconds", int64(l.ElapsedSeconds), int64(c.ElapsedSeconds)}, {"concurrent_calls", int64(l.ConcurrentCalls), int64(c.ConcurrentCalls)},
		{"repairs", int64(l.Repairs), int64(c.Repairs)},
	}
}

func (s *State) settingsRows() []string {
	v := s.AgentSettings
	if !s.settingsScopeCurrent(v) || v.Data == nil {
		return nil
	}
	var rows []string
	if s.EffectivePage() == PageSkills {
		for _, skill := range v.Data.Skills {
			status := "disabled"
			if skill.Enabled {
				status = "enabled"
			}
			rows = append(rows, skill.Directory+" · "+status)
		}
		return rows
	}
	for _, role := range settingsRoleRows(v.Data) {
		rows = append(rows, role.Name+" → "+role.Scope)
	}
	for _, budget := range settingsBudgetRows(v.Data) {
		rows = append(rows, fmt.Sprintf("%s  %d / %d", budget.Name, budget.Value, budget.Ceiling))
	}
	return rows
}

func (s *State) selectedSettingsKey() string {
	v := s.AgentSettings
	if s.EffectivePage() == PageSkills && v != nil && v.Data != nil && s.ClampedIdx() < len(v.Data.Skills) {
		return v.Data.Skills[s.ClampedIdx()].Directory
	}
	return ""
}

func (s *State) restoreSettingsSelection(key string) {
	if key == "" || s.EffectivePage() != PageSkills || s.AgentSettings.Data == nil {
		return
	}
	for i, skill := range s.AgentSettings.Data.Skills {
		if skill.Directory == key {
			s.SelectedIdx = i
			return
		}
	}
	s.SelectedIdx = s.ClampedIdx()
}

func settingsText(text string) string {
	return theme.TruncatePlain(theme.SanitizeSingleLine(text), 1024)
}

func (s *State) renderAgentSettings(th ansiTheme, width int) []string {
	title, keys := "ROLES & BUDGETS", "[Enter] Edit · [r] Refresh · [o] Checkout · [x] Cancel"
	if s.EffectivePage() == PageSkills {
		title, keys = "SKILLS", "[Enter / v] Validate · [e] Enable/disable · [r] Refresh · [o] Checkout"
	}
	lines := renderPageHeader(th, title, keys)
	v := s.AgentSettings
	if !s.settingsScopeCurrent(v) {
		return append(lines, "  Settings unavailable for this checkout.", "  Use :agent-settings main or :agent-settings task.")
	}
	label := "Main checkout"
	if v.Scope.WorktreePath != "" {
		label = "Selected task worktree"
	}
	panelW := adaptivePanelWidth(width)
	status := v.Status
	if v.Data != nil {
		status += fmt.Sprintf(" · %s · version %d", v.Data.State, v.Data.Version)
	}
	lines = append(lines, "  "+th.cyan+theme.TruncatePlain(strings.ToUpper(label)+": "+settingsText(v.Path), max(1, width-4))+th.reset)
	lines = append(lines, "  "+theme.TruncatePlain("State: "+settingsText(status), max(1, width-4)))
	if v.Failure != "" {
		lines = append(lines, renderDiagnosticRow(th, th.amber, "Request:", th.fg, settingsText(v.Failure), max(1, width-4))...)
	}
	rows := s.settingsRows()
	selected := s.ClampedIdx()
	start, end := windowBounds(len(rows), selected, s.adaptiveWindowRows(15, 5))
	for i := start; i < end; i++ {
		lines = append(lines, "  "+th.renderCursorPrefix(i == selected)+rowTitleStyle(th, i == selected)+theme.TruncatePlain(settingsText(rows[i]), max(1, width-6))+th.reset)
	}
	if len(rows) == 0 {
		lines = append(lines, "  No configured entries available.")
	}
	lines = append(lines, "")
	return append(lines, s.renderSettingsDetails(th, panelW)...)
}

func (s *State) renderSettingsDetails(th ansiTheme, panelW int) []string {
	v := s.AgentSettings
	if v.Data == nil {
		return nil
	}
	width := issuePanelContentWidth(panelW)
	var body []string
	if s.EffectivePage() == PageSkills {
		body = s.renderSkillDetails(th, width)
	} else {
		idx := s.ClampedIdx()
		if idx < 3 {
			role := settingsRoleRows(v.Data)[idx]
			body = renderDiagnosticRow(th, th.cyan, "Role:", th.fg, role.Name+" uses "+role.Scope+"; Models presets can override that scope's selected model for TUI workflows.", width)
			for _, model := range v.Data.Models {
				if model.Scope == role.Scope {
					body = append(body, renderDiagnosticRow(th, th.cyan, "Configured:", th.fg, settingsText(model.Model+" at "+model.ProviderOrigin), width)...)
				}
			}
		} else {
			row := settingsBudgetRows(v.Data)[idx-3]
			body = renderDiagnosticRow(th, th.cyan, "Budget:", th.fg, fmt.Sprintf("%s: %d, configured ceiling %d. Values may only be reduced. Repairs may be zero.", row.Name, row.Value, row.Ceiling), width)
		}
	}
	body = append(body, renderDiagnosticRow(th, th.cyan, "Checkout:", th.fg, settingsText(v.Path), width)...)
	body = append(body, renderDiagnosticRow(th, th.cyan, "Effect:", th.fg, "Future runs in this checkout only. Active runs retain their snapshot. New worktrees use separate settings.", width)...)
	if v.Data.State == "stale" {
		body = append(body, renderDiagnosticRow(th, th.amber, "Recovery:", th.fg, "Operator configuration changed. Review current ceilings and models, then use :settings-reset.", width)...)
	}
	return th.renderPanelBox("SELECTED SETTING", "", panelW, body)
}

func (s *State) renderSkillDetails(th ansiTheme, width int) []string {
	v := s.AgentSettings
	if len(v.Data.Skills) == 0 {
		return renderDiagnosticRow(th, th.cyan, "Skills:", th.fg, "No directories configured. The operator configures project-relative skill packages.", width)
	}
	skill := v.Data.Skills[s.ClampedIdx()]
	body := renderDiagnosticRow(th, th.cyan, "Directory:", th.fg, settingsText(skill.Directory), width)
	validation := v.Validations[skill.Directory]
	if validation == nil || validation.Inspection == nil {
		return append(body, renderDiagnosticRow(th, th.cyan, "Validation:", th.fg, "Unavailable. Press Enter to validate the current captured files without executing them.", width)...)
	}
	i := validation.Inspection
	for _, row := range [][2]string{{"Validation:", "Valid captured snapshot; each run validates again."}, {"Name:", i.Name}, {"Description:", i.Description}, {"Compatibility:", i.Compatibility}, {"License:", i.License}, {"Required tools:", strings.Join(i.RequiredTools, ", ")}, {"Manifest:", i.Manifest.Path + " · " + i.Manifest.Hash}} {
		body = append(body, renderDiagnosticRow(th, th.cyan, row[0], th.fg, settingsText(client.FirstNonEmpty(row[1], "not declared")), width)...)
	}
	for _, resource := range i.Resources {
		body = append(body, renderDiagnosticRow(th, th.cyan, "Resource:", th.fg, settingsText(fmt.Sprintf("%s · %d bytes · %s", resource.Path, resource.SizeBytes, resource.Hash)), width)...)
	}
	body = append(body, renderDiagnosticRow(th, th.cyan, "Resources:", th.fg, fmt.Sprintf("%d validated files. Required tools are declarations, not grants; availability is checked per invocation.", len(i.Resources)), width)...)
	return body
}

func modelCapabilitySummary(capabilities client.ModelCapabilities) string {
	var values []string
	for _, item := range []struct {
		name string
		on   bool
	}{{"completion", capabilities.Completion}, {"schema", capabilities.JSONSchema}, {"tools", capabilities.Tools}, {"streaming", capabilities.Streaming}, {"attachments", capabilities.Attachments}, {"grounding", capabilities.Grounding}, {"live", capabilities.LiveSessions}} {
		if item.on {
			values = append(values, item.name)
		}
	}
	if len(values) == 0 {
		return "Unavailable; no capabilities advertised."
	}
	return strings.Join(values, ", ") + ". Unlisted features are unavailable."
}

func (s *State) settingsModelDisplay(scope string) (Model, string) {
	selection := s.resolveDaemonModelSelection(s.AssignedScopeModelID(scope), scope)
	model := s.workflowBoundModel(selection)
	if model.Model == "" || model.ProviderOrigin == "" {
		return model, "UNAVAILABLE"
	}
	if model.RemoteProvider {
		return model, "REMOTE"
	}
	return model, "LOCAL"
}
