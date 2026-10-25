package tui

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

// The local settings read retains retry intent, never provider consent.
type workflowRetrySettings struct {
	Action      ActionRequest
	Fingerprint string
}

func (s *State) prepareWorkflowRetrySettings(action ActionRequest) bool {
	v := s.AgentSettings
	if v == nil || s.settingsScopeCurrent(v) && v.Scope.WorktreePath == s.Change.WorktreePath && v.Data != nil {
		return false
	}
	if v.Pending != "" {
		if v.Operation != "read" || s.settingsScopeCurrent(v) {
			s.Error = "Finish or cancel the pending settings request before retrying this task."
			return true
		}
		s.cancelAgentSettings()
	}
	view, err := s.workflowRetrySettingsView()
	if err != nil {
		s.Error = err.Error()
		return true
	}
	fingerprint, err := s.workflowRetryFingerprint()
	if err != nil {
		s.Error = err.Error()
		return true
	}
	s.AgentSettings = view
	s.queueSettingsAction(agentSettingsAction{Operation: "read", Retry: &workflowRetrySettings{Action: action, Fingerprint: fingerprint}})
	s.CmdOutput = "Loading settings for this task's checkout before retrying. Provider consent and checks remain required."
	return true
}

func (s *State) workflowRetrySettingsView() (*agentSettingsView, error) {
	if s.Project == nil || s.Project.ProjectID == "" || s.Project.ProjectRevision == "" {
		return nil, fmt.Errorf("refresh the project before retrying this task")
	}
	v := &agentSettingsView{Parent: s.Project.ProjectIdentity, Scope: client.AgentSettingsScope{ProjectIdentity: s.Project.ProjectIdentity}, Path: s.Project.Path}
	if s.Change.WorktreePath != "" {
		if s.Change.ProjectID == "" || s.Change.ProjectRevision == "" {
			return nil, fmt.Errorf("refresh the task's checkout identity before retrying")
		}
		v.Scope = client.AgentSettingsScope{ProjectIdentity: s.Change.ProjectIdentity, WorktreePath: s.Change.WorktreePath}
		v.Path = s.Change.WorktreePath
	}
	return v, nil
}

func (s *State) workflowRetryFingerprint() (string, error) {
	data, err := json.Marshal(struct {
		Parent     *Project
		Page       Page
		Change     *ChangeSession
		Models     *ModelCatalog
		Available  *AvailableModels
		Selections map[string]string
	}{s.Project, s.Page, s.Change, s.Models, s.AvailableModels, s.ScopeModels})
	if err != nil {
		return "", fmt.Errorf("capture task before loading checkout settings: %w", err)
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

func (s *State) resumeWorkflowRetrySettings(retry *workflowRetrySettings) {
	fingerprint, err := s.workflowRetryFingerprint()
	if err != nil {
		s.Error = err.Error()
		return
	}
	if s.Quitting || s.CmdActive || s.Confirmation != nil || fingerprint != retry.Fingerprint {
		s.CmdOutput = "Task, view or model selection changed while loading settings. Retry the current task again."
		return
	}
	s.queueWorkflowRetry(retry.Action, nil)
}
