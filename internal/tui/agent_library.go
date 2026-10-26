package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

type libraryRow struct {
	Key          libraryKey
	Title, State string
}

func (s *State) libraryRows() []libraryRow {
	if !s.libraryCurrent() {
		return nil
	}
	v := s.AgentLibrary
	var rows []libraryRow
	if s.EffectivePage() == PageArtifacts && v.Artifacts != nil {
		for _, item := range v.Artifacts.Artifacts {
			state := item.RevisionStatus
			if item.Protected || item.InUse {
				state += " · retained"
			}
			rows = append(rows, libraryRow{Key: libraryKey{libraryArtifacts, item.ID}, Title: fmt.Sprintf("%s · v%d", client.FirstNonEmpty(item.DisplayName, item.Name), item.Version), State: state})
		}
	}
	if s.EffectivePage() == PageMemory {
		for _, item := range v.Memories {
			rows = append(rows, libraryRow{Key: libraryKey{libraryMemory, item.ID}, Title: item.Title, State: libraryMemoryState(item)})
		}
	}
	return rows
}

func libraryMemoryState(item client.AgentMemory) string {
	state := item.State
	if item.Stale {
		state += " · stale"
	}
	if item.Expired {
		state += " · expired"
	}
	return state
}

func (s *State) selectedLibraryKey() libraryKey {
	rows := s.libraryRows()
	if len(rows) == 0 {
		return libraryKey{Kind: s.libraryPageKind()}
	}
	return rows[s.ClampedIdx()].Key
}

func (s *State) restoreLibrarySelection(key libraryKey) {
	for i, row := range s.libraryRows() {
		if row.Key == key {
			s.SelectedIdx = i
			return
		}
	}
	if s.libraryPageKind() != "" {
		s.SelectedIdx = s.ClampedIdx()
	}
}

func (s *State) libraryPageKind() string {
	switch s.EffectivePage() {
	case PageArtifacts:
		return libraryArtifacts
	case PageMemory:
		return libraryMemory
	default:
		return ""
	}
}

func (s *State) libraryArtifact(id string) *client.AgentArtifact {
	if !s.libraryCurrent() || s.AgentLibrary.Artifacts == nil {
		return nil
	}
	for i := range s.AgentLibrary.Artifacts.Artifacts {
		if item := &s.AgentLibrary.Artifacts.Artifacts[i]; item.ID == id {
			return item
		}
	}
	return nil
}

func (s *State) libraryMemory(id string) *client.AgentMemory {
	if !s.libraryCurrent() {
		return nil
	}
	for i := range s.AgentLibrary.Memories {
		if item := &s.AgentLibrary.Memories[i]; item.ID == id {
			return item
		}
	}
	return nil
}

func (s *State) renderLibrary(th ansiTheme, width int) []string {
	kind := s.libraryPageKind()
	title, subtitle := "Artifacts & evidence", "[Enter] Preview · [e] Export · [d] Delete · [n] Next page · [r] Refresh"
	if kind == libraryMemory {
		title, subtitle = "Memory", "[Enter] Inspect · [n] Create · [e] Edit · [a] Accept · [d] Reject · [f] Forget"
	}
	lines := renderPageHeader(th, title, subtitle)
	if !s.libraryCurrent() {
		return append(lines, "  Library unavailable for this checkout.", "  Use :library-scope main or :library-scope task.")
	}
	v := s.AgentLibrary
	label := "MAIN CHECKOUT"
	if v.Scope.WorktreePath != "" {
		label = "TASK WORKTREE"
	}
	lines = append(lines, "  "+th.cyan+theme.TruncatePlain(label+": "+externalSafeText(v.Path), max(1, width-4))+th.reset)
	if v.Pending != "" {
		lines = append(lines, "  Request pending · [x] Cancel local request")
	}
	if v.Failures[kind] != "" {
		lines = append(lines, "  Unavailable: "+externalSafeText(v.Failures[kind]))
	} else if !v.Loaded[kind] {
		lines = append(lines, "  Not loaded · [r] Read local library")
	}
	return s.renderLibraryRows(th, width, lines)
}

func (s *State) renderLibraryRows(th ansiTheme, width int, lines []string) []string {
	rows, selected := s.libraryRows(), s.ClampedIdx()
	if len(rows) == 0 {
		if !s.AgentLibrary.Loaded[s.libraryPageKind()] {
			return append(lines, "  Records unavailable for this view.")
		}
		return append(lines, "  No retained records in this view.")
	}
	start, end := windowBounds(len(rows), selected, s.adaptiveWindowRows(8, 4))
	for i := start; i < end; i++ {
		row := rows[i]
		text := row.Title + " · " + row.State
		lines = append(lines, "  "+th.renderCursorPrefix(i == selected)+rowTitleStyle(th, i == selected)+theme.TruncatePlain(theme.SanitizeCodeText(text), max(1, width-6))+th.reset)
	}
	lines = appendWindowFooter(th, lines, start, end, len(rows))
	if s.libraryPageKind() == libraryArtifacts && s.AgentLibrary.Artifacts.NextCursor != "" {
		lines = append(lines, "  More versions · [n] Next page · :artifact-first restarts")
	}
	panelW := adaptivePanelWidth(width)
	fields := s.libraryDetails(rows[selected].Key)
	var body []string
	for _, field := range fields {
		body = append(body, renderDiagnosticRow(th, th.cyan, field[0], th.fg, externalSafeText(field[1]), issuePanelContentWidth(panelW))...)
	}
	return append(lines, th.renderPanelBox("SELECTED RECORD", "PgUp/PgDn: details", panelW, body)...)
}

func (s *State) libraryDetails(key libraryKey) [][2]string {
	fields := [][2]string{{"Checkout:", s.AgentLibrary.Path}, {"Record:", key.ID}}
	if key.Kind == libraryArtifacts {
		return append(fields, s.libraryArtifactDetails(key.ID)...)
	}
	return append(fields, s.libraryMemoryDetails(key.ID)...)
}

func (s *State) libraryArtifactDetails(id string) [][2]string {
	item := s.libraryArtifact(id)
	if item == nil {
		return nil
	}
	fields := [][2]string{{"Name:", item.Name}, {"Version:", fmt.Sprint(item.Version)}, {"Hash:", item.Hash}, {"Media:", fmt.Sprintf("%s · %d bytes", item.MIME, item.Size)}, {"Owner:", item.Owner.App + " / " + item.Owner.User + " / " + item.Owner.ID}, {"Revision:", item.ProjectRevision + " · " + item.RevisionStatus}, {"Sources:", "Unchecked; revision equality does not revalidate source."}, {"Retention:", fmt.Sprintf("Protected evidence: %t · unfinished run reference: %t", item.Protected, item.InUse)}, {"Created:", item.Created}, {"URI:", item.URI}}
	for _, key := range []string{"run", "proposal", "policy", "configuration", "workspace"} {
		if value := item.Provenance[key]; value != "" {
			fields = append(fields, [2]string{key + ":", value})
		}
	}
	if receipt := s.AgentLibrary.Export; receipt != nil && receipt.ID == id {
		fields = append(fields, [2]string{"Last confirmed copy:", receipt.Path}, [2]string{"Copy hash:", receipt.Hash})
	}
	detail := s.AgentLibrary.Artifact
	if detail == nil || detail.ID != id || detail.Version != item.Version || detail.Hash != item.Hash || s.AgentLibrary.Failures[libraryArtifacts] != "" {
		return append(fields, [2]string{"Preview:", "Press Enter to disclose retained text. Binary media remains metadata only."})
	}
	switch detail.PreviewKind {
	case "research_report":
		return append(fields, [2]string{"Report:", "Use :artifact-report to open the cited local research report in your browser."})
	case "binary":
		return append(fields, [2]string{"Preview:", "Binary media; explicit export preserves the original bytes."})
	default:
		fields = append(fields, [2]string{"Preview:", "Retained text · read-only"})
		if detail.Truncated {
			fields = append(fields, [2]string{"Limit:", "Preview truncated at 64 KiB. Export explicitly for the full artifact."})
		}
		return append(fields, [2]string{"Text:", detail.Text})
	}
}

func (s *State) libraryMemoryDetails(id string) [][2]string {
	item := s.libraryMemory(id)
	if item == nil {
		return nil
	}
	fields := [][2]string{{"Title:", item.Title}, {"Kind:", item.Kind + " · advisory memory"}, {"State:", libraryMemoryState(*item)}, {"Version:", fmt.Sprint(item.Version)}, {"Revision:", item.ProjectRevision}, {"Retention:", "Expires " + item.ExpiresAt}, {"Origin:", item.Provenance.Origin}, {"Updated:", item.UpdatedAt}, {"Policy:", item.Policy}}
	if item.Provenance.SessionID != "" {
		fields = append(fields, [2]string{"Proposal:", item.Provenance.SessionID + " / " + item.Provenance.ProposalHash}, [2]string{"Workflow:", item.Provenance.WorkflowID})
	}
	for i, source := range item.Sources {
		span := "whole file"
		if source.StartLine > 0 {
			span = fmt.Sprintf("lines %d–%d", source.StartLine, source.EndLine)
		}
		fields = append(fields, [2]string{fmt.Sprintf("Source %d:", i+1), source.Path + " · " + span + "\n" + source.Hash})
	}
	fields = append(fields, [2]string{"Source link:", ":memory-source <number> explicitly opens a matching read-only source."})
	detail := s.AgentLibrary.Memory
	if detail == nil || detail.ID != id || detail.Version != item.Version || s.AgentLibrary.Failures[libraryMemory] != "" {
		return append(fields, [2]string{"Text:", "Press Enter to disclose retained advice and check source freshness."})
	}
	fields = append(fields, [2]string{"Source status:", detail.SourceStatus}, [2]string{"Text:", detail.Text})
	if source := s.AgentLibrary.Source; source != nil && s.AgentLibrary.SourceFor == id {
		text := "Binary source; metadata only."
		if !source.Binary {
			text = strings.ReplaceAll(source.Content, "\r\n", "\n")
		}
		if source.Truncated {
			fields = append(fields, [2]string{"Source limit:", "Source preview truncated at 64 KiB."})
		}
		fields = append(fields, [2]string{"Source preview:", source.Path + " · " + source.ContentHash + "\n" + text})
	}
	return fields
}
