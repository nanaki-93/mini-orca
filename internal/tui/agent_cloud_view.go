package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

type cloudRow struct {
	Key, Title, State string
	Target            client.CloudTarget
	Copy              *client.CloudCopy
}

func (s *State) cloudRows() []cloudRow {
	if !s.cloudCurrent() || s.AgentCloud.Data == nil {
		return nil
	}
	d := s.AgentCloud.Data
	rows := make([]cloudRow, 0, len(d.Destinations)+len(d.Copies))
	for _, destination := range d.Destinations {
		t := destination.Target
		rows = append(rows, cloudRow{Key: "destination:" + t.ID, Title: t.ID + " · " + cloudServiceLabel(t.Service), State: "configured", Target: t})
	}
	for i := range d.Copies {
		c := &d.Copies[i]
		state := c.State
		if !c.LeaseUntil.IsZero() && !c.LeaseUntil.After(time.Now()) {
			state += " · recovery available"
		}
		rows = append(rows, cloudRow{Key: c.ID, Title: c.Target.ID + " · " + c.Kind + " · " + c.CreatedAt.Local().Format("Jan 02 15:04"), State: state, Target: c.Target, Copy: c})
	}
	return rows
}

func (s *State) selectedCloudRow() cloudRow {
	rows := s.cloudRows()
	if len(rows) == 0 {
		return cloudRow{}
	}
	return rows[min(max(0, s.SelectedIdx), len(rows)-1)]
}

func cloudServiceLabel(service string) string {
	if service == "vertex_session" {
		return "Vertex session archive"
	}
	return "GCS artifact copy"
}

func cloudDestinationText(target client.CloudTarget) string {
	return fmt.Sprintf("Destination: %s · %s\nPrincipal: %s\nGoogle project: %s\nRegion: %s\nResource: %s\nCredential reference: %s\nManual cleanup: %d days after copying", target.ID, cloudServiceLabel(target.Service), target.Principal, target.Project, target.Region, target.Resource, target.CredentialFileEnv, target.RetentionDays)
}

func (s *State) renderCloudCopies(th ansiTheme, width int) []string {
	lines := renderPageHeader(th, "Cloud copies", "[e] Preview · [c] Copy · [i] Check · [d] Delete")
	if !s.cloudCurrent() {
		return append(lines, "  Main checkout only · [r] Read local receipts.", "  Nothing is uploaded by opening this view.")
	}
	v := s.AgentCloud
	label := client.FirstNonEmpty(v.Status, "not loaded")
	if v.Data != nil && v.Status != "current" {
		label += " · last known"
	}
	lines = append(lines, "  "+th.statusBadge(label)+" · [r] Refresh local receipts")
	if v.Pending != "" {
		lines = append(lines, "  Request pending · [x] Cancel local request")
	}
	for _, notice := range []string{v.Recovery, v.Failure} {
		if notice != "" {
			lines = appendWrappedText(lines, externalSafeText(notice), width-4)
		}
	}
	if v.Data == nil {
		return append(lines, "  [r] Load configured destinations and retained copies.")
	}
	rows := s.cloudRows()
	if len(rows) == 0 {
		return append(lines, "  No cloud destinations or copies configured.", "  Configure optional destinations in integrations.cloud.")
	}
	selected := min(max(0, s.SelectedIdx), len(rows)-1)
	start, end := windowBounds(len(rows), selected, s.adaptiveWindowRows(7, 5))
	for i := start; i < end; i++ {
		text := rows[i].State + " · " + rows[i].Title
		lines = append(lines, "  "+th.renderCursorPrefix(i == selected)+rowTitleStyle(th, i == selected)+theme.TruncatePlain(externalSafeText(text), max(1, width-6))+th.reset)
	}
	if v.Preview != nil {
		lines = appendWrappedText(lines, fmt.Sprintf("Preview ready: %s · %d bytes · [c] Review copying", v.Preview.Kind, v.Preview.Size), width-4)
	}
	lines = append(lines, "  [Enter] Details · PgUp/PgDn scroll")
	if v.Details {
		lines = append(lines, s.renderCloudDetails(width)...)
	}
	return lines
}

func (s *State) renderCloudDetails(width int) []string {
	v := s.AgentCloud
	row := s.selectedCloudRow()
	text := "Main checkout: " + v.Path + "\nCurrent revision: " + v.Scope.ProjectRevision
	if row.Key != "" {
		text += "\n" + cloudDestinationText(row.Target)
	}
	if c := row.Copy; c != nil {
		text += fmt.Sprintf("\nCopy: %s\nState: %s · receipt version %d\nSource: %s · %s\nCaptured checkout revision: %s\nSource revision: %s\nSource version: %d\nArchive: %d bytes\nSHA-256: %s\nCopied/attempted: %s\nLast receipt: %s\nManual cleanup deadline: %s", c.ID, c.State, c.Version, c.Kind, c.SourceID, c.ProjectRevision, c.SourceRevision, c.SourceVersion, c.Size, c.Hash, triggerTime(c.CreatedAt), triggerTime(c.UpdatedAt), triggerTime(c.CleanupAt))
		if !c.LeaseUntil.IsZero() {
			text += "\nOperation lease until: " + triggerTime(c.LeaseUntil)
		}
		if c.State != "deleted" && !c.CleanupAt.After(time.Now()) {
			text += "\nCleanup deadline reached. Inspect and explicitly delete the remote copy."
		}
	}
	if p := v.Preview; p != nil {
		text += fmt.Sprintf("\n\nSELECTED PRIVATE DATA\n%s\nSource: %s · %s\nSource revision: %s\nSource version: %d\nArchive: %d bytes\nSHA-256: %s\nConfiguration: %s", cloudDestinationText(p.Target), p.Kind, p.SourceID, p.SourceRevision, p.SourceVersion, p.Size, p.Hash, p.Configuration)
	}
	text += "\nLocal originals remain intact. Archives cannot resume workflows.\nCleanup is manual. Provider retention and backups may preserve deleted data.\nUse :cloud-preview <destination> artifact|session <source-id>. Opening :cloud from a main-checkout artifact or task detail prefills the source when available."
	lines := []string{""}
	for _, line := range strings.Split(externalSafeText(text), "\n") {
		lines = appendWrappedText(lines, line, max(1, width-4))
	}
	return lines
}
