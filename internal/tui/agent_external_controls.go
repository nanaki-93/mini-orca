package tui

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

func (s *State) apiReceipt(id string) *client.APIAction {
	for i := range s.AgentExternal.Actions {
		if s.AgentExternal.Actions[i].ID == id {
			return &s.AgentExternal.Actions[i]
		}
	}
	return nil
}

func (s *State) remoteReceipt(id string) *client.Delegation {
	v := s.AgentExternal
	if v.Detail != nil && v.Detail.ID == id {
		for _, item := range v.Delegations {
			if item.ID == id && item.Version == v.Detail.Version {
				return v.Detail
			}
		}
	}
	return nil
}

func (s *State) selectedExternal(kind string) (externalKey, error) {
	if err := s.externalReady(kind); err != nil {
		return externalKey{}, err
	}
	key := s.selectedExternalKey()
	if key.Kind != kind || key.ID == "" || s.AgentExternal.Pending != "" {
		return key, fmt.Errorf("select the relevant receipt in External Work and finish its pending request first")
	}
	return key, nil
}

func (s *State) confirmAPIAction(operation, outcome, observation string) {
	key, err := s.selectedExternal(externalAPI)
	if err != nil {
		s.Error = err.Error()
		return
	}
	item := s.apiReceipt(key.ID)
	if item == nil {
		s.Error = "Selected API receipt is unavailable."
		return
	}
	action := agentExternalAction{Operation: "api-control", Key: key, APIControl: client.APIActionControl{ProjectIdentity: s.AgentExternal.Scope.ProjectIdentity, Action: operation, Version: item.Version, RequestHash: item.RequestHash, Confirm: true, ReportedOutcome: outcome, Observation: observation}}
	if operation == "approve" && s.AgentExternal.Reviewed != (externalReview{ID: item.ID, Hash: item.RequestHash, Version: item.Version}) {
		s.Error = "Press Enter to inspect the exact retained request before approving it."
		return
	}
	message := fmt.Sprintf("Checkout: %s\nReceipt: %s · version %d\nRequest hash: %s\n%s", theme.SanitizeCodeText(s.AgentExternal.Path), item.ID, item.Version, item.RequestHash, externalJSONText(item.Request))
	if operation == "approve" {
		message += "\nSend this exact external request once. Cancellation cannot undo a completed external effect; uncertain delivery requires reconciliation."
	} else {
		message += "\nRecord an externally checked, user-reported outcome: " + theme.SanitizeCodeText(outcome+" · "+observation) + ". This does not send or replay the request."
	}
	s.confirmExternalControl(action, message)
}

func (s *State) confirmRemoteControl(operation, text string) {
	key, err := s.selectedExternal(externalRemote)
	if err != nil {
		s.Error = err.Error()
		return
	}
	item := s.remoteReceipt(key.ID)
	if item == nil {
		s.Error = "Press Enter to load the current local delegation detail before acting."
		return
	}
	request := client.DelegationControl{ProjectIdentity: s.AgentExternal.Scope.ProjectIdentity, Version: item.Version, Action: operation, Confirm: true}
	message := fmt.Sprintf("Checkout: %s\nRemote receipt: %s · version %d · %s\n", theme.SanitizeCodeText(s.AgentExternal.Path), item.ID, item.Version, item.State)
	if operation == "reconcile" {
		request.Observation = text
		message += "Record a user-reported reconciliation without network activity. This does not confirm remote cancellation or success.\nObservation: " + theme.SanitizeCodeText(text)
	} else {
		remote, err := s.externalRemoteConnection(item.RemoteID)
		if err != nil {
			s.Error = err.Error()
			return
		}
		request.Remote = externalRemoteUse(*remote, operation == "input")
		message += "Destination: " + settingsText(remote.Config.Endpoint) + "\n" + operation + " the retained remote task. Previous messages are not resent."
		if operation == "input" {
			request.Reply = text
			message += "\nConfirm sharing this reply once: " + theme.SanitizeCodeText(text)
		}
		if operation == "cancel" {
			message += " Remote cleanup is attempted; inspect the resulting receipt for the actual outcome."
		}
	}
	s.confirmExternalControl(agentExternalAction{Operation: "remote-control", Key: key, RemoteControl: request}, message)
}

func (s *State) confirmExternalControl(action agentExternalAction, message string) {
	identity, err := s.externalControlIdentity(action)
	if err != nil {
		s.Error = err.Error()
		return
	}
	action.Expected = identity
	parent, scope := s.AgentExternal.Parent, s.AgentExternal.Scope
	s.Confirmation = &Confirmation{Title: "Confirm external receipt action", AcceptLabel: "Confirm", Message: message, OnConfirm: func(state *State) {
		if !state.externalCurrent() || state.AgentExternal.Scope != scope || state.AgentExternal.Parent != parent || state.selectedExternalKey() != action.Key {
			state.Error = "Checkout or selected receipt changed. Review the current action again."
			return
		}
		if err := state.externalAdmission(action); err != nil {
			state.Error = err.Error()
			return
		}
		state.queueExternalAction(action)
	}}
}

func (s *State) externalControlIdentity(action agentExternalAction) (string, error) {
	if err := s.externalReady(action.Key.Kind); err != nil {
		return "", err
	}
	var receipt, destination any
	if action.Operation == "api-control" {
		item := s.apiReceipt(action.Key.ID)
		if err := validateAPIControl(item, action.APIControl); err != nil {
			return "", err
		}
		receipt = item
	} else {
		item := s.remoteReceipt(action.Key.ID)
		if item == nil || !remoteControlAllowed(*item, action.RemoteControl.Action, time.Now()) {
			return "", fmt.Errorf("the current remote receipt does not permit this control; refresh local detail")
		}
		remote, err := s.validateRemoteControl(*item, action.RemoteControl)
		if err != nil {
			return "", err
		}
		receipt, destination = item, remote
	}
	return externalFingerprint(struct {
		Parent               ProjectIdentity
		Scope                client.AgentSettingsScope
		Receipt, Destination any
		API                  client.APIActionControl
		Remote               client.DelegationControl
	}{s.AgentExternal.Parent, s.AgentExternal.Scope, receipt, destination, action.APIControl, action.RemoteControl})
}

func validateAPIControl(item *client.APIAction, request client.APIActionControl) error {
	if item == nil || item.Version != request.Version || item.RequestHash != request.RequestHash {
		return fmt.Errorf("API receipt changed; review its current version and hash")
	}
	switch request.Action {
	case "approve":
		if item.Stale || !item.CanApprove || item.State != "candidate" || !item.ExpiresAt.After(time.Now()) {
			return fmt.Errorf("this API candidate is stale, expired or no longer approvable")
		}
	case "reconcile":
		if !item.CanReconcile || !slices.Contains([]string{"applied", "not_applied", "other"}, request.ReportedOutcome) {
			return fmt.Errorf("only an uncertain receipt can be reconciled with applied, not_applied or other")
		}
		return externalTextInput(request.Observation)
	default:
		return fmt.Errorf("API action must be approve or reconcile")
	}
	return nil
}

func remoteControlAllowed(item client.Delegation, operation string, now time.Time) bool {
	if item.Active || item.State == "pending" && now.Before(item.LeaseUntil) || slices.Contains([]string{"completed", "failed", "rejected", "canceled", "not_sent", "reconciled"}, item.State) {
		return false
	}
	if operation == "reconcile" {
		return item.State == "unknown" || item.State == "auth_required" || item.State == "pending"
	}
	if item.Progress.TaskID == "" {
		return false
	}
	if operation == "input" {
		return item.State == "input_required" && !item.Stale
	}
	return slices.Contains([]string{"refresh", "reconnect", "cancel"}, operation)
}

func (s *State) validateRemoteControl(item client.Delegation, request client.DelegationControl) (*client.AgentConnection, error) {
	if request.Version != item.Version {
		return nil, fmt.Errorf("remote receipt version changed")
	}
	if request.Action == "reconcile" {
		return nil, externalTextInput(request.Observation)
	}
	remote, err := s.remoteControlDestination(item)
	if err != nil {
		return nil, err
	}
	if request.Remote != externalRemoteUse(*remote, request.Action == "input") {
		return nil, fmt.Errorf("retained remote task and current card, endpoint or grant differ")
	}
	if request.Action == "reconnect" && !remote.Inventory.Streaming {
		return nil, fmt.Errorf("this remote card does not advertise streaming reconnection")
	}
	if request.Action == "input" {
		if err := externalTextInput(request.Reply); err != nil {
			return nil, err
		}
	}
	return remote, nil
}

func (s *State) remoteControlDestination(item client.Delegation) (*client.AgentConnection, error) {
	remote, err := s.externalRemoteConnection(item.RemoteID)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256([]byte(remote.Config.Endpoint))
	if item.EndpointHash != hex.EncodeToString(hash[:]) || item.CardFingerprint != remote.Inventory.Fingerprint {
		return nil, fmt.Errorf("retained remote task and current card or endpoint differ")
	}
	return remote, nil
}
