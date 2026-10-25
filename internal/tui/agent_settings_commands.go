package tui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

func (s *State) execAgentSettingsCommand(command, arg string) bool {
	switch command {
	case "agent-settings":
		s.selectAgentSettings(strings.TrimSpace(arg))
	case "settings-refresh":
		s.refreshAgentSettings()
	case "settings-cancel":
		s.cancelAgentSettings()
	case "settings-reset":
		s.confirmSettingsControl(nil, "reset", "Restore this checkout's configured roles, budgets and skill enablement.")
	case "agent-role":
		s.editAgentRole(strings.Fields(arg))
	case "agent-budget":
		s.editAgentBudget(strings.Fields(arg))
	case "skill-validate":
		s.validateSelectedSkill()
	case "skill-toggle":
		s.toggleSelectedSkill()
	default:
		return false
	}
	return true
}

func (s *State) editableSettings() *client.AgentSettings {
	v := s.AgentSettings
	if !s.settingsScopeCurrent(v) || v.Data == nil || v.Pending != "" {
		s.Error = "Refresh settings for the current checkout first."
		return nil
	}
	return v.Data
}

func settingsPreferences(data *client.AgentSettings) client.AgentPreferences {
	values := client.AgentPreferences{Roles: data.Roles, Limits: data.Limits, DisabledSkills: []string{}}
	for _, skill := range data.Skills {
		if !skill.Enabled {
			values.DisabledSkills = append(values.DisabledSkills, skill.Directory)
		}
	}
	return values
}

func (s *State) confirmSettingsControl(values *client.AgentPreferences, action, description string) {
	data := s.editableSettings()
	if data == nil {
		return
	}
	if data.State == "stale" && action != "reset" {
		s.Error = "Configured defaults changed. Review them and use :settings-reset before editing."
		return
	}
	v := s.AgentSettings
	scope, parent, version, configuration := v.Scope, v.Parent, data.Version, data.Configuration
	request := client.AgentSettingsControl{ProjectIdentity: data.ProjectIdentity, Version: version, Configuration: configuration, Action: action, Confirm: true, Values: values}
	s.Confirmation = &Confirmation{Title: "Save agent settings", AcceptLabel: "Save", Message: settingsText(description) + "\nCheckout: " + settingsText(v.Path) + "\nOnly future runs in this checkout are affected. Existing runs keep their captured settings. New worktrees have separate budgets and skills.", OnConfirm: func(state *State) {
		current := state.editableSettings()
		if current == nil {
			return
		}
		if state.AgentSettings.Scope != scope || state.AgentSettings.Parent != parent || current.Version != version || current.Configuration != configuration {
			state.Error = "Settings changed. Review the current checkout and values again."
			return
		}
		state.queueSettingsAction(agentSettingsAction{Operation: "control", Control: request})
	}}
}

func (s *State) editAgentRole(fields []string) {
	if len(fields) != 2 || !slices.Contains([]string{"analyze", "bug", "code"}, fields[1]) {
		s.Error = "Use :agent-role creation|testing|review analyze|bug|code."
		return
	}
	data := s.editableSettings()
	if data == nil {
		return
	}
	values := settingsPreferences(data)
	switch fields[0] {
	case "creation":
		values.Roles.Creation = fields[1]
	case "testing":
		values.Roles.Testing = fields[1]
	case "review":
		values.Roles.Review = fields[1]
	default:
		s.Error = "Choose creation, testing or review."
		return
	}
	s.confirmSettingsControl(&values, "replace", "Assign "+fields[0]+" to the "+fields[1]+" model scope.")
}

func (s *State) editAgentBudget(fields []string) {
	if len(fields) != 2 {
		s.Error = "Use :agent-budget <name> <integer>; see Roles & Budgets for names and ceilings."
		return
	}
	data := s.editableSettings()
	if data == nil {
		return
	}
	value, err := strconv.ParseInt(fields[1], 10, 64)
	rows := settingsBudgetRows(data)
	index := slices.IndexFunc(rows, func(row settingsBudget) bool { return row.Name == fields[0] })
	if err != nil || index < 0 || value < 0 || value > rows[index].Ceiling || value == 0 && fields[0] != "repairs" {
		s.Error = "Use a positive budget within its configured ceiling; repairs may be zero."
		return
	}
	values := settingsPreferences(data)
	setSettingsBudget(&values.Limits, fields[0], value)
	s.confirmSettingsControl(&values, "replace", fmt.Sprintf("Set %s to %d (configured ceiling %d).", fields[0], value, rows[index].Ceiling))
}

func setSettingsBudget(limits *client.AgentLimits, name string, value int64) {
	switch name {
	case "model_calls":
		limits.ModelCalls = int(value)
	case "tool_calls":
		limits.ToolCalls = int(value)
	case "input_tokens":
		limits.InputTokens = value
	case "output_tokens":
		limits.OutputTokens = value
	case "elapsed_seconds":
		limits.ElapsedSeconds = int(value)
	case "concurrent_calls":
		limits.ConcurrentCalls = int(value)
	case "repairs":
		limits.Repairs = int(value)
	}
}

func (s *State) selectedSettingsSkill() *client.AgentSkillSetting {
	data := s.editableSettings()
	if data == nil {
		return nil
	}
	if s.EffectivePage() != PageSkills || s.ClampedIdx() >= len(data.Skills) {
		s.Error = "Select a configured skill in Settings / Skills first."
		return nil
	}
	copy := data.Skills[s.ClampedIdx()]
	return &copy
}

func (s *State) validateSelectedSkill() {
	skill := s.selectedSettingsSkill()
	if skill == nil {
		return
	}
	data := s.AgentSettings.Data
	delete(s.AgentSettings.Validations, skill.Directory)
	s.queueSettingsAction(agentSettingsAction{Operation: "validate", Validation: client.AgentSkillValidationRequest{ProjectIdentity: data.ProjectIdentity, Version: data.Version, Configuration: data.Configuration, Directory: skill.Directory}})
}

func (s *State) toggleSelectedSkill() {
	skill := s.selectedSettingsSkill()
	if skill == nil {
		return
	}
	values := settingsPreferences(s.AgentSettings.Data)
	verb := "Disable"
	if skill.Enabled {
		values.DisabledSkills = append(values.DisabledSkills, skill.Directory)
	} else {
		verb = "Enable"
		values.DisabledSkills = slices.DeleteFunc(values.DisabledSkills, func(path string) bool { return path == skill.Directory })
	}
	s.confirmSettingsControl(&values, "replace", verb+" "+skill.Directory+". Validation does not grant tools or execution permission.")
}

func (s *State) handleAgentSettingsKey(key string) bool {
	switch key {
	case "r":
		s.refreshAgentSettings()
	case "x":
		s.cancelAgentSettings()
	case "o":
		s.openCommandInput("agent-settings ")
	case "e":
		if s.EffectivePage() == PageSkills {
			s.toggleSelectedSkill()
		} else {
			s.activateAgentSettingsRow()
		}
	case "v":
		if s.EffectivePage() == PageSkills {
			s.validateSelectedSkill()
		}
	default:
		return false
	}
	return true
}

func (s *State) activateAgentSettingsRow() {
	if s.EffectivePage() == PageSkills {
		s.validateSelectedSkill()
		return
	}
	data := s.editableSettings()
	if data == nil {
		return
	}
	idx := s.ClampedIdx()
	if idx < 3 {
		roles := settingsRoleRows(data)
		s.openCommandInput("agent-role " + roles[idx].Name + " ")
	} else {
		row := settingsBudgetRows(data)[idx-3]
		s.openCommandInput(fmt.Sprintf("agent-budget %s %d", row.Name, row.Value))
	}
}
