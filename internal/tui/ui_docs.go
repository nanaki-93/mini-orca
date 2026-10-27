package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

const docsSplit = 0.24

// renderDocsWorkspace shows the generated handbook: its sections beside the
// current section, read with j/k and PgUp/PgDn.
func (s *State) renderDocsWorkspace(th ansiTheme, width, rows int) []string {
	docs := domain.ResolveProjectDocs(s.Overview, s.Project)
	page := s.EffectivePage()
	contentW := detailPaneWidth(width, docsSplit)
	s.viewportBodyRows = detailPaneRows(width, rows)
	sections := []struct {
		page  Page
		label string
		count int
	}{
		{PageDocs, "Context", len(docs.Dependencies)},
		{PageDocsComponents, "Components", len(docs.Components)},
		{PageDocsDecisions, "Scenarios & decisions", len(docs.Scenarios) + len(docs.Decisions)},
		{PageDiagrams, "Architecture", len(docs.Components)},
	}
	navW := listPaneWidth(width, docsSplit)
	nav := []string{}
	for _, sec := range sections {
		label := rowTitleStyle(th, sec.page == page) + sec.label + th.reset
		nav = append(nav, th.listRow(sec.page == page, rightAlign(label, th.muted+fmt.Sprint(sec.count)+th.reset, navW-1), navW))
	}
	nav = append(nav, "", th.field("Status", th.docsStatusTone(docs.Status)+docs.Status+th.reset, 8))
	if docs.UpdatedAt != "" {
		nav = append(nav, th.field("Updated", th.fg+humanTime(docs.UpdatedAt)+th.reset, 8))
	}
	if docs.Status != "" && docs.Status != "fresh" && docs.Status != "completed" {
		nav = append(nav, "")
		nav = append(nav, wrapText(th.muted+"Parts come from indexed facts only. Press r on Inbox to analyze the project."+th.reset, navW)...)
	}
	nav = append(nav, "", th.muted+"Tab next section"+th.reset)

	var body []string
	switch page {
	case PageDocsComponents:
		body = th.docsComponents(docs, contentW)
	case PageDocsDecisions:
		body = th.docsDecisions(docs, contentW)
	case PageDiagrams:
		body = domain.RenderArchitectureASCII(docs)
		if strings.TrimSpace(docs.Context.Topology) != "" {
			body = append(wrapText(th.muted+docs.Context.Topology+th.reset, contentW), append([]string{""}, body...)...)
		}
	default:
		body = th.docsContext(docs, contentW)
	}
	s.DetailScroll = theme.ClampScroll(s.DetailScroll, len(body), detailPaneRows(width, rows))
	left := theme.Pane{Title: "Handbook", Body: nav}
	var title string
	for _, sec := range sections {
		if sec.page == page {
			title = sec.label
		}
	}
	right := theme.Pane{Title: title, Body: body, Focused: true, Scroll: s.DetailScroll}
	return s.splitPanes(th, width, rows, docsSplit, left, right)
}

func (th ansiTheme) docsStatusTone(status string) string {
	switch status {
	case "fresh", "completed":
		return th.green
	case "":
		return th.muted
	default:
		return th.amber
	}
}

func (th ansiTheme) docsContext(docs ProjectDocsReport, width int) []string {
	var body []string
	body = append(body, th.sectionHeader("Purpose", -1))
	body = append(body, wrapText(th.fg+theme.SanitizeSingleLine(docs.Context.Purpose)+th.reset, width)...)
	if len(docs.Context.GoalPursuit) > 0 {
		body = append(body, "", th.sectionHeader("How it gets there", -1))
		for _, goal := range docs.Context.GoalPursuit {
			body = append(body, wrapText(th.accent+"• "+th.reset+th.fg+theme.SanitizeSingleLine(goal)+th.reset, width)...)
		}
	}
	if strings.TrimSpace(docs.Context.Topology) != "" {
		body = append(body, "", th.sectionHeader("Topology", -1))
		body = append(body, wrapText(th.fg+theme.SanitizeSingleLine(docs.Context.Topology)+th.reset, width)...)
	}
	if len(docs.Context.Stack) > 0 {
		body = append(body, "", th.sectionHeader("Stack", -1))
		body = append(body, wrapText(th.fg+strings.Join(docs.Context.Stack, th.muted+" · "+th.reset+th.fg)+th.reset, width)...)
	}
	if len(docs.Dependencies) > 0 {
		body = append(body, "", th.sectionHeader("Dependencies", len(docs.Dependencies)))
		for _, dep := range docs.Dependencies {
			name := th.bold + th.fg + dep.Name + th.reset
			if dep.Version != "" {
				name += th.muted + " " + dep.Version + th.reset
			}
			body = append(body, rightAlign(name, th.muted+dep.Kind+th.reset, width))
			if dep.Purpose != "" {
				body = append(body, wrapText("  "+th.muted+theme.SanitizeSingleLine(dep.Purpose)+th.reset, width)...)
			}
		}
	}
	return body
}

func (th ansiTheme) docsComponents(docs ProjectDocsReport, width int) []string {
	if len(docs.Components) == 0 {
		return emptyPane(th, "No components recorded", "Analysis groups the source into components with owners and contracts.")
	}
	var body []string
	layer := ""
	for _, c := range docs.Components {
		if c.Layer != layer {
			if layer != "" {
				body = append(body, "")
			}
			layer = c.Layer
			body = append(body, th.sectionHeader(layer, -1))
		}
		body = append(body, rightAlign(th.bold+th.fg+c.Name+th.reset, th.cyan+theme.TruncatePlain(c.Path, max(10, width/2))+th.reset, width))
		if c.Summary != "" {
			body = append(body, wrapText("  "+th.fg+theme.SanitizeSingleLine(c.Summary)+th.reset, width)...)
		}
		for _, r := range c.Responsibilities {
			body = append(body, wrapText("  "+th.muted+"· "+theme.SanitizeSingleLine(r)+th.reset, width)...)
		}
		if len(c.Dependencies) > 0 {
			body = append(body, wrapText("  "+th.muted+"uses "+strings.Join(c.Dependencies, ", ")+th.reset, width)...)
		}
	}
	return body
}

func (th ansiTheme) docsDecisions(docs ProjectDocsReport, width int) []string {
	var body []string
	if len(docs.Scenarios) > 0 {
		body = append(body, th.sectionHeader("Scenarios", len(docs.Scenarios)))
		for _, sc := range docs.Scenarios {
			body = append(body, th.bold+th.fg+sc.Title+th.reset)
			if sc.Summary != "" {
				body = append(body, wrapText("  "+th.fg+theme.SanitizeSingleLine(sc.Summary)+th.reset, width)...)
			}
			if sc.Invariant != "" {
				body = append(body, wrapText("  "+th.accent+"invariant "+th.reset+th.fg+theme.SanitizeSingleLine(sc.Invariant)+th.reset, width)...)
			}
			for i, step := range sc.Steps {
				line := "  " + th.muted + fmt.Sprintf("%d.", i+1) + th.reset + " " + th.fg + step.Title + th.reset
				if step.Path != "" {
					line += th.muted + "  " + step.Path + th.reset
				}
				body = append(body, wrapText(line, width)...)
			}
			body = append(body, "")
		}
	}
	if len(docs.Decisions) > 0 {
		body = append(body, th.sectionHeader("Decisions", len(docs.Decisions)))
		for _, d := range docs.Decisions {
			body = append(body, rightAlign(th.bold+th.fg+d.Title+th.reset, th.muted+d.Category+th.reset, width))
			body = append(body, wrapText("  "+th.fg+theme.SanitizeSingleLine(d.Decision)+th.reset, width)...)
			if d.Tradeoff != "" {
				body = append(body, wrapText("  "+th.muted+"trade-off "+theme.SanitizeSingleLine(d.Tradeoff)+th.reset, width)...)
			}
			body = append(body, "")
		}
	}
	if len(body) == 0 {
		return emptyPane(th, "No scenarios or decisions recorded", "Analysis records runtime scenarios and architecture decisions.")
	}
	return body
}
