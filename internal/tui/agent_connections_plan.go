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

// Investigation connections are ephemeral draft choices. Saving a plan or
// generating a proposal does not persist or inherit these disclosure grants.
type investigationConnection struct {
	Key    connectionKey
	Parent ProjectIdentity
	Scope  client.AgentSettingsScope
	Data   client.AgentConnection
}

func (s *State) connectionInPlan(key connectionKey) bool {
	d := s.PlanDraft
	return d != nil && s.connectionsCurrent() && slices.ContainsFunc(d.Connections, func(item investigationConnection) bool {
		return item.Key == key && item.Parent == d.Source && item.Scope == s.AgentConnections.Scope
	})
}

func (s *State) togglePlanConnection() {
	row, err := s.selectedConnection()
	if err != nil {
		s.Error = err.Error()
		return
	}
	d, v := s.PlanDraft, s.AgentConnections
	if d == nil || d.Submitted || d.InvestigationState == "running" {
		s.Error = "Open an editable task plan and cancel its active investigation first."
		return
	}
	if d.Source != v.Parent || d.Seed.WorktreePath != v.Scope.WorktreePath {
		s.Error = "Load connections for the same checkout as this plan."
		return
	}
	if s.connectionInPlan(row.Key) {
		d.Connections = slices.DeleteFunc(d.Connections, func(item investigationConnection) bool { return item.Key == row.Key })
		s.CmdOutput = "Connection removed from this plan's next investigation."
		return
	}
	if err := usableInvestigationConnection(*row, len(d.Connections)); err != nil {
		s.Error = err.Error()
		return
	}
	d.Connections = append(d.Connections, investigationConnection{Key: row.Key, Parent: v.Parent, Scope: v.Scope, Data: row.Data})
	s.CmdOutput = "Selected " + row.Key.ID + " for this plan. Investigation will separately confirm destination and source sharing."
}

func usableInvestigationConnection(row agentConnectionRow, count int) error {
	if row.Key.Kind != client.ConnectionMCP && row.Key.Kind != client.ConnectionAPI {
		return fmt.Errorf("only MCP and OpenAPI tools can be selected for plan investigation")
	}
	if count >= 2 {
		return fmt.Errorf("select at most two connections per investigation")
	}
	if row.State != "ready" || row.Data.Inventory == nil || row.Data.Inventory.Fingerprint == "" {
		return fmt.Errorf("discover and review a current inventory and grant before selecting this connection")
	}
	return nil
}

func (s *State) planConnectionUses(d *PlanDraft) ([]client.IntegrationUse, *client.AgentSettingsScope, error) {
	if len(d.Connections) == 0 {
		return nil, nil, nil
	}
	if !s.connectionsCurrent() || len(d.Connections) > 2 || s.AgentConnections.Pending != "" {
		return nil, nil, fmt.Errorf("refresh the plan's checkout connections before investigation")
	}
	v := s.AgentConnections
	if d.Source != v.Parent || d.Seed.WorktreePath != v.Scope.WorktreePath {
		return nil, nil, fmt.Errorf("plan and connection checkout scopes differ")
	}
	var uses []client.IntegrationUse
	for _, selected := range d.Connections {
		entry, err := s.currentPlanConnection(selected)
		if err != nil {
			return nil, nil, err
		}
		uses = append(uses, client.IntegrationUse{ID: entry.Config.ID, Grant: entry.Grant, SchemaFingerprint: entry.Inventory.Fingerprint, ConfirmSourceSharing: true})
	}
	scope := v.Scope
	return uses, &scope, nil
}

func (s *State) currentPlanConnection(selected investigationConnection) (*client.AgentConnection, error) {
	v := s.AgentConnections
	if selected.Parent != v.Parent || selected.Scope != v.Scope || v.Failures[selected.Key.Kind].State != "" || selected.Data.Inventory == nil {
		return nil, fmt.Errorf("selected connection scope is stale or unavailable; refresh and select it again")
	}
	for _, row := range s.connectionRows() {
		if row.Key != selected.Key {
			continue
		}
		if err := usableInvestigationConnection(row, 0); err != nil {
			return nil, err
		}
		if row.Data.Grant != selected.Data.Grant || row.Data.Configuration != selected.Data.Configuration || row.Data.Inventory.Fingerprint != selected.Data.Inventory.Fingerprint {
			return nil, fmt.Errorf("connection grant or inventory changed; remove and select the current connection again")
		}
		return &row.Data, nil
	}
	return nil, fmt.Errorf("selected connection is no longer configured")
}

func (s *State) investigationConsent(d *PlanDraft, model Model) (string, string, error) {
	if len(d.Connections) > 0 && !model.Capabilities.Tools {
		return "", "", fmt.Errorf("the configured Code adapter does not advertise tool support; refresh or configure a tool-capable model before using connections")
	}
	_, _, err := s.planConnectionUses(d)
	if err != nil {
		return "", "", err
	}
	data, err := json.Marshal(struct {
		Plan  string
		Model Model
	}{planDraftFingerprint(d), model})
	if err != nil {
		return "", "", fmt.Errorf("capture investigation destinations: %w", err)
	}
	hash := sha256.Sum256(data)
	message := "Investigate the captured plan, selected read/write sources and applicable AGENTS.md guides."
	if model.RemoteProvider {
		message += "\nProvider: " + settingsText(model.Model+" at "+model.ProviderOrigin)
	}
	for _, connection := range d.Connections {
		row := agentConnectionRow{Key: connection.Key, Data: connection.Data}
		message += "\n" + connectionKindLabel(connection.Key.Kind) + " " + connection.Key.ID + ": " + settingsText(connectionDestination(row))
	}
	if len(d.Connections) > 0 {
		message += "\nAllow these tools to receive admitted project context, including prompt, source, skill and memory context, during this investigation. External writes remain staged for exact request review."
	}
	paths := append(slices.Clone(d.Seed.Paths), d.Seed.Plan.ReadPaths...)
	for i, path := range paths {
		paths[i] = settingsText(path)
	}
	message += "\nCaptured paths:\n" + strings.Join(paths, "\n") + "\nResults are suggestions for review. Source remains read-only."
	return hex.EncodeToString(hash[:]), message, nil
}

func planConnectionSummary(d *PlanDraft) string {
	if d == nil || len(d.Connections) == 0 {
		return ""
	}
	var names []string
	for _, connection := range d.Connections {
		names = append(names, connectionKindLabel(connection.Key.Kind)+" "+connection.Key.ID)
	}
	return "Investigation connections: " + strings.Join(names, ", ") + ". Source sharing requires confirmation."
}

type planConnectionIdentity struct {
	Key                             connectionKey
	Parent                          ProjectIdentity
	Scope                           client.AgentSettingsScope
	Configuration, Grant, Inventory string
}

func planConnectionIdentities(d *PlanDraft) []planConnectionIdentity {
	var result []planConnectionIdentity
	for _, item := range d.Connections {
		fingerprint := ""
		if item.Data.Inventory != nil {
			fingerprint = item.Data.Inventory.Fingerprint
		}
		result = append(result, planConnectionIdentity{Key: item.Key, Parent: item.Parent, Scope: item.Scope, Configuration: item.Data.Configuration, Grant: item.Data.Grant, Inventory: fingerprint})
	}
	return result
}

func (s *State) approvePlanInvestigation(d *PlanDraft, model Model, confirmed bool) (string, bool) {
	fingerprint, message, err := s.investigationConsent(d, model)
	if err != nil {
		s.Error = err.Error()
		return "", false
	}
	if confirmed || !model.RemoteProvider && len(d.Connections) == 0 {
		return fingerprint, true
	}
	s.Confirmation = &Confirmation{Title: "Confirm investigation destinations", AcceptLabel: "Investigate", Message: message, OnConfirm: func(state *State) {
		if state.PlanDraft == nil {
			state.Error = "Plan changed. Review the current scope before investigation."
			return
		}
		currentModel, err := state.planInvestigationModel(state.PlanDraft)
		if err != nil {
			state.Error = err.Error()
			return
		}
		current, _, err := state.investigationConsent(state.PlanDraft, currentModel)
		if err != nil || current != fingerprint {
			state.Error = "Plan, provider or connection grant changed. Review and confirm the current destinations again."
			return
		}
		state.startPlanInvestigation(true)
	}}
	return "", false
}

func (s *State) investigationActionCurrent(action ActionRequest) error {
	d := s.PlanDraft
	if !s.matchesPlanSource(action.Source) || planDraftFingerprint(d) != d.InvestigationInput {
		return fmt.Errorf("plan changed before investigation dispatch")
	}
	uses, scope, err := s.planConnectionUses(d)
	if err != nil {
		return err
	}
	if !slices.Equal(uses, action.Integrations) || !sameIntegrationScope(scope, action.IntegrationScope) {
		return fmt.Errorf("investigation connection scope changed before dispatch")
	}
	model, err := s.planInvestigationModel(d)
	if err != nil {
		return err
	}
	fingerprint, _, err := s.investigationConsent(d, model)
	if err != nil {
		return err
	}
	if fingerprint != action.InvestigationConsent {
		return fmt.Errorf("investigation provider or connection destinations changed before dispatch")
	}
	return nil
}

func sameIntegrationScope(left, right *client.AgentSettingsScope) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}
