package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

type mediaRow struct {
	ID, Title, State string
	Artifact         *client.AgentArtifact
	Session          *client.LiveSession
}

func (s *State) mediaRows() []mediaRow {
	if !s.mediaCurrent() {
		return nil
	}
	v := s.AgentMedia
	var rows []mediaRow
	switch v.Group {
	case "attachments":
		if v.Attachments != nil {
			for i := range v.Attachments.Attachments {
				rows = append(rows, mediaArtifactRow(&v.Attachments.Attachments[i]))
			}
		}
	case "sessions":
		if v.Live != nil {
			for i := range v.Live.Sessions {
				item := &v.Live.Sessions[i]
				state := item.State
				if item.Stale {
					state += " · stale"
				}
				rows = append(rows, mediaRow{ID: item.ID, Title: item.Model + " · " + item.ID, State: state, Session: item})
			}
		}
	case "outputs":
		if v.Advice != nil {
			rows = append(rows, mediaArtifactRow(&v.Advice.Artifact))
		}
		if v.Session != nil {
			for i := range v.Session.OutputArtifacts {
				rows = append(rows, mediaArtifactRow(&v.Session.OutputArtifacts[i]))
			}
		}
	}
	return rows
}

func mediaArtifactRow(item *client.AgentArtifact) mediaRow {
	return mediaRow{ID: item.ID, Title: fmt.Sprintf("%s · v%d", client.FirstNonEmpty(item.DisplayName, item.Name), item.Version), State: item.RevisionStatus, Artifact: item}
}

func (s *State) selectedMediaRow() mediaRow {
	rows := s.mediaRows()
	if len(rows) == 0 {
		return mediaRow{}
	}
	return rows[max(0, min(s.SelectedIdx, len(rows)-1))]
}

func (s *State) selectedMediaArtifact() *client.AgentArtifact { return s.selectedMediaRow().Artifact }

func (s *State) selectedMediaSession() *client.LiveSession {
	if !s.mediaCurrent() || s.AgentMedia.Session == nil {
		return nil
	}
	if s.AgentMedia.Group == "sessions" && s.selectedMediaRow().ID != s.AgentMedia.Session.ID {
		return nil
	}
	return s.AgentMedia.Session
}

func (s *State) restoreMediaSelection(id string) {
	for i, row := range s.mediaRows() {
		if row.ID == id {
			s.SelectedIdx = i
			return
		}
	}
	s.SelectedIdx = 0
}

func (s *State) inspectMedia() {
	if err := s.mediaReady(); err != nil {
		s.Error = err.Error()
		return
	}
	row := s.selectedMediaRow()
	if row.ID == "" {
		s.Error = "Select an imported attachment, live session or saved output."
		return
	}
	operation := "inspect"
	if row.Session != nil {
		operation = "session"
	}
	s.queueMediaAction(agentMediaAction{Operation: operation, Target: row.ID})
}

func (s *State) confirmMediaExport() {
	if err := s.mediaReady(); err != nil {
		s.Error = err.Error()
		return
	}
	item := s.selectedMediaArtifact()
	if item == nil {
		s.Error = "Select a retained attachment or output to export."
		return
	}
	request := client.ArtifactControl{ProjectIdentity: s.AgentMedia.Scope.ProjectIdentity, Action: "export", Version: item.Version, Hash: item.Hash}
	message := fmt.Sprintf("Create a private local copy of %s, version %d.\nMIME: %s\nSize: %d bytes\nSHA-256: %s\nCheckout: %s\nRevision: %s\nThe receipt will show the saved path. Audio is not played automatically.", client.FirstNonEmpty(item.DisplayName, item.Name), item.Version, item.MIME, item.Size, item.Hash, s.AgentMedia.Path, request.ProjectRevision)
	s.confirmMediaAction(agentMediaAction{Operation: "export", Target: item.ID, Artifact: request}, "Export private media", message, "Export")
}

func (s *State) mediaActiveNotice() string {
	if !s.mediaCurrent() || s.AgentMedia.Live == nil {
		return ""
	}
	for _, item := range s.AgentMedia.Live.Sessions {
		if item.Owned && item.ExpiresAt.After(time.Now()) {
			return "Live session may still be active · :media to inspect or close"
		}
	}
	return ""
}

func (s *State) renderMedia(th ansiTheme, width int) []string {
	lines := renderPageHeader(th, "Media & live advice", "[a] Attachments · [l] Live sessions · [o] Outputs · [r] Refresh")
	if !s.mediaCurrent() {
		return append(lines, "  Select a checkout with :media-scope main or task.", "  Import and provider sharing are separate explicit actions.")
	}
	v := s.AgentMedia
	lines = append(lines, "  "+th.cyan+theme.TruncatePlain(externalSafeText(v.Path), max(1, width-4))+th.reset)
	if v.Pending != "" {
		lines = append(lines, "  "+th.cyan+"Request pending · [x] Cancel local request"+th.reset)
	}
	if failure := v.Failures[v.Group]; failure != "" {
		lines = appendWrappedText(lines, "Unavailable: "+externalSafeText(failure)+" · [r] Refresh", width-4)
		lines = append(lines, "  Any retained rows show the last known state.")
	}
	if v.Recovery != "" {
		lines = appendWrappedText(lines, externalSafeText(v.Recovery)+" Refresh before another provider or import action.", width-4)
	}
	if v.Notice != "" {
		lines = appendWrappedText(lines, externalSafeText(v.Notice), width-4)
	}
	lines = append(lines, "  "+th.bold+strings.ToUpper(v.Group)+th.reset)
	lines = append(lines, s.mediaAvailability()...)
	rows := s.mediaRows()
	if len(rows) == 0 {
		return append(lines, s.mediaEmptyState()...)
	}
	start, end := windowBounds(len(rows), s.ClampedIdx(), s.adaptiveWindowRows(7, 3))
	for i := start; i < end; i++ {
		row, mark := rows[i], ""
		if v.Group == "attachments" {
			if _, ok := v.Selected[row.ID]; ok {
				mark = "[selected] "
			}
		}
		text := mark + row.Title + " · " + row.State
		lines = append(lines, "  "+th.renderCursorPrefix(i == s.ClampedIdx())+rowTitleStyle(th, i == s.ClampedIdx())+theme.TruncatePlain(externalSafeText(text), max(1, width-6))+th.reset)
	}
	lines = appendWindowFooter(th, lines, start, end, len(rows))
	return append(lines, s.renderMediaDetail(th, width)...)
}

func (s *State) mediaAvailability() []string {
	v := s.AgentMedia
	switch v.Group {
	case "attachments":
		if v.Attachments == nil {
			return []string{"  [r] Load local attachment metadata."}
		}
		if v.Attachments.State != "available" {
			return []string{"  Advice unavailable: " + externalSafeText(v.Attachments.State), "  Local import remains available; configure a native Code model for advice."}
		}
		refs, total := s.selectedMediaRefs()
		return []string{fmt.Sprintf("  Selected: %d / 4 · %d bytes · [Space] Toggle · [n] Import · [q] Ask", len(refs), total)}
	case "sessions":
		if v.Live == nil {
			return []string{"  [r] Read live availability."}
		}
		if v.Live.State != "available" {
			return []string{"  Live: " + externalSafeText(v.Live.State), "  Configure agent_runtime.live_model and a Gemini API Code profile."}
		}
		return []string{"  :live-open · Enter inspects · :live-close joins the selected session"}
	default:
		return []string{"  Enter discloses private text · [e] Export · Esc hides text"}
	}
}

func (s *State) mediaEmptyState() []string {
	v := s.AgentMedia
	if v.Failures[v.Group] != "" || v.Group == "attachments" && v.Attachments == nil || v.Group == "sessions" && v.Live == nil {
		return []string{"  Records unavailable. Refresh to load local history."}
	}
	switch s.AgentMedia.Group {
	case "attachments":
		return []string{"  No imported attachments in this view.", "  Use :media-import <path> to review a local file before import."}
	case "sessions":
		return []string{"  No retained live sessions in this checkout."}
	default:
		return []string{"  No selected output yet. Advice is saved after completion; live", "  transcript/audio artifacts appear after the connection ends.", "  Older versions remain in Code / Artifacts."}
	}
}

func (s *State) renderMediaDetail(th ansiTheme, width int) []string {
	row := s.selectedMediaRow()
	fields := [][2]string{}
	if row.Artifact != nil {
		item := row.Artifact
		fields = append(fields, [2]string{"Type:", fmt.Sprintf("%s · %d bytes", item.MIME, item.Size)}, [2]string{"SHA-256:", item.Hash}, [2]string{"Revision:", item.ProjectRevision + " · " + item.RevisionStatus})
		fields = append(fields, s.mediaArtifactDisclosure(item)...)
	} else if row.Session != nil {
		item := row.Session
		if s.AgentMedia.Session != nil && s.AgentMedia.Session.ID == item.ID {
			item = s.AgentMedia.Session
		}
		fields = s.mediaSessionFields(item)
	}
	panelW := adaptivePanelWidth(width)
	var body []string
	for _, field := range fields {
		body = append(body, renderDiagnosticRow(th, th.cyan, field[0], th.fg, externalSafeText(field[1]), issuePanelContentWidth(panelW))...)
	}
	return th.renderPanelBox("SELECTED MEDIA", "PgUp/PgDn: details", panelW, body)
}

func (s *State) mediaArtifactDisclosure(item *client.AgentArtifact) [][2]string {
	detail := s.AgentMedia.Private
	if detail == nil || detail.ID != item.ID || detail.Version != item.Version || detail.Hash != item.Hash {
		return [][2]string{{"Preview:", "Enter explicitly discloses retained text. Binary data remains private."}}
	}
	if detail.PreviewKind != "text" {
		return [][2]string{{"Preview:", "Binary media · export to inspect locally. No automatic playback."}}
	}
	label := "Private advice:"
	if detail.Truncated {
		label += " (truncated)"
	}
	return [][2]string{{label, detail.Text}}
}

func (s *State) mediaSessionFields(item *client.LiveSession) [][2]string {
	usage := item.Usage
	fields := [][2]string{{"Session:", item.ID}, {"State:", fmt.Sprintf("%s · owned %t · stale %t", item.State, item.Owned, item.Stale)}, {"Destination:", item.Model + " · " + item.ProviderOrigin}, {"Progress:", fmt.Sprintf("%d/2 connections · %d input controls · %d completed turns", item.Connections, item.Turns, item.CompletedTurns)}, {"Budget charge:", fmt.Sprintf("%d input / %d output · %d unknown calls", usage.InputTokens, usage.OutputTokens, usage.UnknownUsageCalls)}, {"Reserved:", fmt.Sprintf("%d input / %d output", usage.ReservedInputTokens, usage.ReservedOutputTokens)}, {"Expires:", item.ExpiresAt.Local().Format("15:04:05 MST")}}
	if item.Reported {
		fields = append(fields, [2]string{"Reported usage:", fmt.Sprintf("%d input / %d output · partial, not final billing", item.ReportedInput, item.ReportedOutput)})
	}
	if item.RetentionFailed {
		fields = append(fields, [2]string{"Retention:", "Failed. Inspect local history; saved outputs may be missing."})
	}
	if item.MissingArtifacts > 0 {
		fields = append(fields, [2]string{"Missing outputs:", fmt.Sprint(item.MissingArtifacts)})
	}
	if item.CanReconnect {
		fields = append(fields, [2]string{"Recovery:", ":live-reconnect reviews a fresh connection; no input replay."})
	}
	if item.Owned {
		fields = append(fields, [2]string{"Controls:", ":live-send · :live-send-media · :live-interrupt · :live-close"})
	}
	return append(fields, [2]string{"Output:", "Advisory only. Inspect this session, then press o for retained transcript/audio."})
}
