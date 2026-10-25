package tui

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

// A choice binds consent to the selected destinations and captured task input.
// It is immutable once queued; changing a preset cannot retarget queued work.
type workflowChoice struct {
	Fingerprint  string
	Models       map[string]string
	Confirmed    []string
	Destinations []string
}

func (s *State) workflowRoles(path string) client.AgentRoles {
	roles := client.AgentRoles{Creation: "bug", Testing: "code", Review: "analyze"}
	v := s.AgentSettings
	if s.settingsScopeCurrent(v) && v.Data != nil && v.Scope.WorktreePath == path {
		roles = v.Data.Roles
	}
	return roles
}

func (s *State) workflowModelsPayload(path string) (map[string]string, []string) {
	roles := s.workflowRoles(path)
	models := map[string]string{}
	var profiles []string
	for _, role := range []struct{ stage, scope string }{{"create", roles.Creation}, {"test", roles.Testing}, {"review", roles.Review}} {
		selection := s.resolveDaemonModelSelection(s.AssignedScopeModelID(role.scope), role.scope)
		models[role.stage] = selection
		if !slices.Contains(profiles, selection) {
			profiles = append(profiles, selection)
		}
	}
	return models, profiles
}

func (s *State) workflowBoundModel(selection string) Model {
	if s.Models != nil {
		if model, ok := s.Models.Scopes[selection]; ok {
			return model
		}
	}
	if s.AvailableModels != nil {
		for _, choice := range s.AvailableModels.Models {
			if choice.ID == selection {
				return choice.Model
			}
		}
	}
	if s.settingsScopeCurrent(s.AgentSettings) && s.AgentSettings.Data != nil {
		for _, model := range s.AgentSettings.Data.Models {
			if model.Scope == selection {
				return model
			}
		}
	}
	// Unknown metadata never supplies remote consent. The daemon's provider
	// policy rejects an unconfirmed remote selection before a provider call.
	return Model{}
}

func (s *State) captureWorkflowChoice(seed *ChangeSeed, change *ChangeSession) (*workflowChoice, error) {
	path := ""
	if seed != nil {
		path = seed.WorktreePath
	} else if change != nil {
		path = change.WorktreePath
	}
	settings, err := s.workflowSettings(path)
	if err != nil {
		return nil, err
	}
	models, _ := s.workflowModelsPayload(path)
	choice := &workflowChoice{Models: models}
	bindings := map[string]Model{}
	for _, stage := range []string{"create", "test", "review"} {
		selection := models[stage]
		model := s.workflowBoundModel(selection)
		bindings[stage] = model
		if model.RemoteProvider {
			choice.Destinations = append(choice.Destinations, fmt.Sprintf("%s: %s (%s) at %s", stage, selection, settingsText(model.Model), settingsText(model.ProviderOrigin)))
			if !slices.Contains(choice.Confirmed, selection) {
				choice.Confirmed = append(choice.Confirmed, selection)
			}
		}
	}
	data, err := json.Marshal(struct {
		Parent   ProjectIdentity
		Seed     *ChangeSeed
		Change   *ChangeSession
		Settings *client.AgentSettings
		Models   map[string]string
		Bindings map[string]Model
	}{ProjectIdentity{ProjectID: s.projectID(), ProjectRevision: s.projectRevision()}, seed, change, settings, models, bindings})
	if err != nil {
		return nil, fmt.Errorf("capture workflow destinations: %w", err)
	}
	hash := sha256.Sum256(data)
	choice.Fingerprint = hex.EncodeToString(hash[:])
	return choice, nil
}

func (s *State) approveWorkflowChoice(seed *ChangeSeed, change *ChangeSession, approved *workflowChoice, resume func(*State, *workflowChoice)) (*workflowChoice, bool) {
	choice, err := s.captureWorkflowChoice(seed, change)
	if err != nil {
		s.Error = err.Error()
		return nil, false
	}
	if approved != nil {
		if approved.Fingerprint != choice.Fingerprint {
			s.Error = "Task, settings or model destinations changed. Review and confirm the current request again."
			return nil, false
		}
		return choice, true
	}
	if len(choice.Destinations) == 0 {
		return choice, true
	}
	paths := []string{}
	if seed != nil {
		paths = append(paths, seed.Paths...)
		if seed.Plan != nil {
			paths = append(paths, seed.Plan.ReadPaths...)
		}
	} else if change != nil {
		for _, target := range change.Targets {
			paths = append(paths, target.Path)
		}
		if change.Plan != nil {
			paths = append(paths, change.Plan.ReadPaths...)
		}
	}
	for i, path := range paths {
		paths[i] = settingsText(path)
	}
	message := "Send the captured plan, scoped sources, applicable AGENTS.md instructions and workflow evidence to:\n" + strings.Join(choice.Destinations, "\n") + "\nCaptured paths:\n" + strings.Join(paths, "\n") + "\nReview and Apply remain separate. A new task worktree has its own budgets and skills."
	s.Confirmation = &Confirmation{Title: "Confirm workflow providers", AcceptLabel: "Generate", Message: message, OnConfirm: func(state *State) { resume(state, choice) }}
	return nil, false
}

func (s *State) queueWorkflowRetry(action ActionRequest, approved *workflowChoice) {
	if s.Change == nil || s.Change.ID != action.Target {
		s.Error = "Task changed. Select the proposal again before retrying."
		return
	}
	if approved == nil && s.prepareWorkflowRetrySettings(action) {
		return
	}
	choice, ok := s.approveWorkflowChoice(nil, s.Change, approved, func(state *State, choice *workflowChoice) { state.queueWorkflowRetry(action, choice) })
	if !ok {
		return
	}
	action.WorkflowChoice = choice
	s.enqueueAction(action)
	s.CmdOutput = "Retrying the captured task with the selected role models."
}

func (s *State) queuedWorkflowModels(action ActionRequest, change *ChangeSession) (map[string]string, []string, error) {
	choice, err := s.captureWorkflowChoice(action.Seed, change)
	if err != nil {
		return nil, nil, err
	}
	if action.WorkflowChoice != nil {
		if action.WorkflowChoice.Fingerprint != choice.Fingerprint {
			return nil, nil, fmt.Errorf("task settings or destinations changed before dispatch; review the task again")
		}
	} else if len(choice.Destinations) > 0 {
		return nil, nil, fmt.Errorf("confirm the selected remote workflow providers before generation")
	}
	return choice.Models, choice.Confirmed, nil
}

func (s *State) workflowSettings(path string) (*client.AgentSettings, error) {
	v := s.AgentSettings
	if v == nil {
		return nil, nil
	}
	if !s.settingsScopeCurrent(v) || v.Scope.WorktreePath != path {
		return nil, fmt.Errorf("load :agent-settings main or task for this task's checkout before generation")
	}
	if v.Data == nil || v.Data.State == "stale" || v.Pending != "" {
		return nil, fmt.Errorf("refresh and resolve unavailable, stale or pending agent settings before generation")
	}
	return v.Data, nil
}

func (s *State) startWorkflowModels(action ActionRequest) (map[string]string, []string, error) {
	if action.Kind == "save-plan" {
		return nil, nil, nil
	}
	return s.queuedWorkflowModels(action, action.ExistingChange)
}
