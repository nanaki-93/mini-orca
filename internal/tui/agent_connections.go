package tui

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

type agentConnectionRow struct {
	Key   connectionKey
	Data  client.AgentConnection
	State string
}

func connectionKindLabel(kind client.ConnectionKind) string {
	switch kind {
	case client.ConnectionMCP:
		return "MCP"
	case client.ConnectionAPI:
		return "OpenAPI"
	case client.ConnectionRemote:
		return "A2A outbound"
	case client.ConnectionAccess:
		return "A2A inbound"
	default:
		return "Unavailable"
	}
}

func (s *State) connectionRows() []agentConnectionRow {
	if !s.connectionsCurrent() {
		return nil
	}
	v := s.AgentConnections
	var rows []agentConnectionRow
	for _, kind := range client.ConnectionKinds() {
		entries, loaded := v.Records[kind]
		if len(entries) == 0 {
			state := "none configured"
			if !loaded {
				state = "not loaded"
			}
			if failure := v.Failures[kind]; failure.State != "" {
				state = failure.State
			}
			rows = append(rows, agentConnectionRow{Key: connectionKey{Kind: kind}, State: state})
			continue
		}
		for _, entry := range entries {
			key := connectionKey{Kind: kind, ID: entry.Config.ID}
			rows = append(rows, agentConnectionRow{Key: key, Data: entry, State: s.connectionState(key, entry)})
		}
	}
	return rows
}

func (s *State) connectionState(key connectionKey, entry client.AgentConnection) string {
	v := s.AgentConnections
	if failure := v.Failures[key.Kind]; failure.State != "" {
		return failure.State
	}
	if entry.State == "ready" || entry.State == "authorized" {
		if entry.ExpiresAt == nil || entry.Grant == "" {
			return "unavailable"
		}
		if !entry.ExpiresAt.After(time.Now()) {
			return "expired"
		}
	}
	if receipt := v.Receipts[key]; entry.State == "configured" && receipt.Configuration == entry.Configuration && receipt.Action == "revoke" {
		return "disconnected"
	}
	return entry.State
}

func (s *State) selectedConnectionKey() connectionKey {
	if s.EffectivePage() != PageConnections {
		return connectionKey{}
	}
	rows := s.connectionRows()
	if len(rows) == 0 {
		return connectionKey{}
	}
	return rows[s.ClampedIdx()].Key
}

func (s *State) restoreConnectionSelection(key connectionKey) {
	if s.EffectivePage() != PageConnections {
		return
	}
	for i, row := range s.connectionRows() {
		if row.Key == key {
			s.SelectedIdx = i
			return
		}
	}
	s.SelectedIdx = s.ClampedIdx()
}

func (s *State) selectedConnection() (*agentConnectionRow, error) {
	if !s.connectionsCurrent() || s.EffectivePage() != PageConnections || s.AgentConnections.Pending != "" {
		return nil, fmt.Errorf("open Connections for the current checkout and finish or cancel its pending request first")
	}
	rows := s.connectionRows()
	if len(rows) == 0 || rows[s.ClampedIdx()].Key.ID == "" {
		return nil, fmt.Errorf("select a configured connection first")
	}
	row := rows[s.ClampedIdx()]
	if s.AgentConnections.Failures[row.Key.Kind].State != "" {
		return nil, fmt.Errorf("refresh local inventory to establish the current grant before acting")
	}
	return &row, nil
}

func (s *State) renderConnections(th ansiTheme, width int) []string {
	lines := renderPageHeader(th, "CONNECTIONS", "[Enter / c] Discover · [d] Revoke · [s] Use in plan · [w] External work · [r] Refresh")
	if !s.connectionsCurrent() {
		return append(lines, "  Inventory unavailable for this checkout.", "  Use :connection-scope main or :connection-scope task.")
	}
	v := s.AgentConnections
	label := "MAIN CHECKOUT"
	if v.Scope.WorktreePath != "" {
		label = "TASK WORKTREE"
	}
	lines = append(lines, "  "+th.cyan+theme.TruncatePlain(label+": "+settingsText(v.Path), max(1, width-4))+th.reset)
	if v.Pending != "" {
		lines = append(lines, "  Request pending; cancellation requires refresh to confirm outcome.")
	}
	rows, selected := s.connectionRows(), s.ClampedIdx()
	start, end := windowBounds(len(rows), selected, s.adaptiveWindowRows(12, 5))
	for i := start; i < end; i++ {
		row := rows[i]
		text := connectionKindLabel(row.Key.Kind) + " · " + client.FirstNonEmpty(row.Key.ID, "—") + " · " + row.State
		if s.connectionInPlan(row.Key) {
			text += " · in plan"
		}
		lines = append(lines, "  "+th.renderCursorPrefix(i == selected)+rowTitleStyle(th, i == selected)+theme.TruncatePlain(settingsText(text), max(1, width-6))+th.reset)
	}
	lines = append(lines, "")
	if len(rows) > 0 {
		lines = append(lines, s.renderConnectionDetails(th, adaptivePanelWidth(width), rows[selected])...)
	}
	return lines
}

func (s *State) renderConnectionDetails(th ansiTheme, panelW int, row agentConnectionRow) []string {
	width := issuePanelContentWidth(panelW)
	fields := [][2]string{{"Kind:", connectionKindLabel(row.Key.Kind)}, {"Checkout:", s.AgentConnections.Path}, {"State:", row.State}}
	if failure := s.AgentConnections.Failures[row.Key.Kind]; failure.State != "" {
		fields = append(fields, [2]string{"Request:", failure.Message})
	}
	if row.Key.ID != "" {
		fields = append(fields, connectionConfigurationDetails(row)...)
		fields = append(fields, connectionInventoryDetails(row.Data.Inventory)...)
	}
	var body []string
	for _, field := range fields {
		body = append(body, renderDiagnosticRow(th, th.cyan, field[0], th.fg, settingsText(client.FirstNonEmpty(field[1], "not configured")), width)...)
	}
	return th.renderPanelBox("CONNECTION DETAILS", "", panelW, body)
}

func connectionDestination(row agentConnectionRow) string {
	if row.Key.Kind == client.ConnectionAccess {
		return "Incoming authenticated requests for principal " + row.Key.ID
	}
	if row.Key.Kind == client.ConnectionMCP && row.Data.Config.Transport == "stdio" {
		args, _ := json.Marshal(row.Data.Config.Args)
		return row.Data.Config.Command + " " + string(args)
	}
	return row.Data.Config.Endpoint
}

func connectionConfigurationDetails(row agentConnectionRow) [][2]string {
	config := row.Data.Config
	fields := [][2]string{{"Destination:", connectionDestination(row)}, {"Configuration:", row.Data.Configuration}, {"Credential reference:", client.FirstNonEmpty(config.BearerTokenEnv, "none")}}
	for _, name := range slices.Sorted(maps.Keys(config.Environment)) {
		fields = append(fields, [2]string{"Process credential:", name + " ← " + config.Environment[name]})
	}
	if row.Data.ExpiresAt != nil {
		fields = append(fields, [2]string{"Grant expires:", row.Data.ExpiresAt.UTC().Format(time.RFC3339)})
	}
	for _, permission := range append(slices.Clone(config.Tools), config.Operations...) {
		fields = append(fields, [2]string{"Permission:", permission.Name + " · " + permission.Effect})
	}
	switch row.Key.Kind {
	case client.ConnectionMCP:
		fields = append(fields, [2]string{"Transport:", config.Transport}, [2]string{"Use:", "Discovery may connect or launch the configured process. Source sharing requires a separate investigation confirmation."})
	case client.ConnectionAPI:
		fields = append(fields, [2]string{"Schema:", config.SpecPath}, [2]string{"Use:", "Schema discovery reads local files. Endpoint availability is untested. External writes require exact request review."})
	case client.ConnectionRemote:
		fields = append(fields, [2]string{"Agent card:", config.CardURL}, [2]string{"Allowed skills:", strings.Join(config.Skills, ", ")}, [2]string{"Use:", "Discovery retrieves the agent card. Delegation requires separate source sharing; results remain advisory."})
	case client.ConnectionAccess:
		fields = append(fields, [2]string{"Project:", config.ProjectID}, [2]string{"Capabilities:", strings.Join(config.Capabilities, ", ")}, [2]string{"Use:", "Grant permits incoming authenticated analysis or proposals for 15 minutes. Source Apply and execution trust remain separate."})
	}
	return fields
}

func connectionInventoryDetails(inventory *client.ConnectionInventory) [][2]string {
	if inventory == nil {
		return nil
	}
	fields := [][2]string{{"Inventory:", inventory.Fingerprint}}
	for _, tool := range inventory.Tools {
		fields = append(fields, [2]string{"Tool:", tool.Name + " · " + tool.Description})
	}
	for _, operation := range inventory.Operations {
		fields = append(fields, [2]string{"Operation:", operation.ID + " · " + operation.Method + " " + operation.Path + " · " + operation.Effect}, [2]string{"Description:", operation.Description})
	}
	for _, skill := range inventory.Skills {
		fields = append(fields, [2]string{"Remote skill:", skill.ID + " · " + skill.Name + " · " + skill.Description})
	}
	if inventory.Name != "" {
		fields = append(fields, [2]string{"Remote agent:", inventory.Name + " · " + inventory.Version}, [2]string{"Description:", inventory.Description}, [2]string{"Streaming:", fmt.Sprint(inventory.Streaming)})
	}
	return fields
}
