package tui

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

func (s *State) execConnectionCommand(command, arg string) bool {
	switch command {
	case "connection-scope":
		s.selectConnectionScope(strings.TrimSpace(arg))
	case "connections-refresh":
		s.refreshConnections()
	case "connection-cancel":
		s.cancelConnectionRequest()
	case "connection-discover":
		s.confirmConnectionControl("discover")
	case "connection-revoke":
		s.confirmConnectionControl("revoke")
	case "connection-use":
		s.togglePlanConnection()
	case "connections-clear":
		if s.PlanDraft == nil || s.PlanDraft.Submitted || s.PlanDraft.InvestigationState == "running" {
			s.Error = "Open an editable plan and cancel any active investigation first."
		} else {
			s.PlanDraft.Connections = nil
			s.CmdOutput = "Connections removed from this plan's next investigation. Existing grants are unchanged."
		}
	default:
		return false
	}
	return true
}

func (s *State) handleConnectionKey(key string) bool {
	switch key {
	case "w":
		s.Navigate(PageExternalWork)
	case "c":
		s.confirmConnectionControl("discover")
	case "d":
		s.confirmConnectionControl("revoke")
	case "s":
		s.togglePlanConnection()
	case "r":
		s.refreshConnections()
	case "x":
		s.cancelConnectionRequest()
	case "o":
		s.openCommandInput("connection-scope ")
	default:
		return false
	}
	return true
}

type connectionApproval struct {
	Key         connectionKey
	Request     client.ConnectionControl
	Fingerprint string
	Message     string
}

func (s *State) prepareConnectionApproval(action string) (*connectionApproval, error) {
	row, err := s.selectedConnection()
	if err != nil {
		return nil, err
	}
	if action == "revoke" && row.Data.Grant == "" {
		return nil, fmt.Errorf("this connection has no current grant; refresh local inventory to confirm its state")
	}
	if action != "revoke" && (row.State == "disabled" || row.State == "unavailable") {
		return nil, fmt.Errorf("this connection is unavailable; check operator configuration and credential references")
	}
	v := s.AgentConnections
	request := client.ConnectionControl{ProjectIdentity: v.Scope.ProjectIdentity, Action: action, Configuration: row.Data.Configuration, Grant: row.Data.Grant, ConfirmConnect: action != "revoke"}
	message := connectionControlMessage(*row, action)
	var providers []Model
	if row.Key.Kind == client.ConnectionAccess && action != "revoke" {
		providers, err = s.inboundConnectionProviders(row.Data.Config)
		if err != nil {
			return nil, err
		}
		request.Action, request.ConfirmSourceSharing = "grant", true
		for _, provider := range providers {
			message += "\nProvider: " + settingsText(provider.Scope+" · "+provider.Model+" at "+provider.ProviderOrigin)
			request.ConfirmRemoteProvider = request.ConfirmRemoteProvider || provider.RemoteProvider
		}
	}
	data, err := json.Marshal(struct {
		Parent    ProjectIdentity
		Scope     client.AgentSettingsScope
		Row       agentConnectionRow
		Request   client.ConnectionControl
		Providers []Model
	}{v.Parent, v.Scope, *row, request, providers})
	if err != nil {
		return nil, fmt.Errorf("capture connection approval: %w", err)
	}
	hash := sha256.Sum256(data)
	return &connectionApproval{Key: row.Key, Request: request, Fingerprint: hex.EncodeToString(hash[:]), Message: message + "\nCheckout: " + settingsText(v.Path)}, nil
}

func connectionControlMessage(row agentConnectionRow, action string) string {
	if action == "revoke" {
		return "Revoke the current " + connectionKindLabel(row.Key.Kind) + " grant for " + row.Key.ID + ". Active operations owned by this grant will be canceled. This does not undo completed external effects."
	}
	message := "Review " + connectionKindLabel(row.Key.Kind) + " connection " + row.Key.ID + ":\n" + settingsText(connectionDestination(row))
	switch row.Key.Kind {
	case client.ConnectionMCP:
		message += "\nConnect or launch the configured process to discover allowlisted tools. Project source is not shared by discovery; investigation requires separate consent."
	case client.ConnectionAPI:
		message += "\nInspect the local schema at " + settingsText(row.Data.Config.SpecPath) + ". This does not test endpoint availability. Tool use requires a separate investigation; writes require exact request approval."
	case client.ConnectionRemote:
		message += "\nRetrieve the agent card from " + settingsText(row.Data.Config.CardURL) + ". Delegation and source sharing require a separate action."
	case client.ConnectionAccess:
		message += "\nAllow authenticated incoming " + settingsText(strings.Join(row.Data.Config.Capabilities, ", ")) + " requests to use this project's admitted sources for 15 minutes. Allowing proposals does not grant Apply, merge or project execution permission."
	}
	return message
}

func (s *State) inboundConnectionProviders(config client.ConnectionConfig) ([]Model, error) {
	var models []Model
	for _, capability := range config.Capabilities {
		scope := "function"
		if capability == "proposal" {
			scope = "bug"
		}
		model := s.workflowBoundModel(scope)
		if model.Model == "" || model.ProviderOrigin == "" {
			return nil, fmt.Errorf("refresh configured model metadata before granting incoming project access")
		}
		model.Scope = scope
		models = append(models, model)
	}
	return models, nil
}

func (s *State) confirmConnectionControl(action string) {
	approval, err := s.prepareConnectionApproval(action)
	if err != nil {
		s.Error = err.Error()
		return
	}
	s.Confirmation = &Confirmation{Title: "Confirm connection access", Message: approval.Message, AcceptLabel: "Confirm", OnConfirm: func(state *State) {
		current, err := state.prepareConnectionApproval(action)
		if err != nil {
			state.Error = err.Error()
			return
		}
		if current.Fingerprint != approval.Fingerprint {
			state.Error = "Connection, checkout, grant or provider changed. Review the current selection again."
			return
		}
		state.queueConnectionAction(agentConnectionAction{Operation: "control", Key: approval.Key, Control: approval.Request})
	}}
}
