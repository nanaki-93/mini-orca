package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
)

func (s *State) renderDocsPage(th ansiTheme, width int) []string {
	if s.DocsSection == "components" {
		return s.renderDocsComponentsPage(th, width)
	}
	if s.DocsSection == "scenarios" || s.DocsSection == "decisions" {
		return s.renderDocsDecisionsPage(th, width)
	}
	panelW := adaptivePanelWidth(width)
	docs := domain.ResolveProjectDocs(s.Overview, s.Project)
	lines := renderPageHeader(th, "CONTEXT & DEPENDENCIES", "Specification & subsystem boundaries")
	ctxLines := []string{
		fmt.Sprintf("%sPurpose:%s  %s%s%s", th.bold+th.cyan, th.reset, th.fg, docs.Context.Purpose, th.reset),
		fmt.Sprintf("%sTopology:%s %s%s%s", th.bold+th.purple, th.reset, th.fg, docs.Context.Topology, th.reset),
	}
	lines = append(lines, th.renderPanelBox("SYSTEM CONTEXT & TOPOLOGY", strings.ToUpper(docs.Status), panelW, ctxLines)...)
	lines = append(lines, "")
	lines = append(lines, th.renderTableHeader("   DEPENDENCY & SUBSYSTEM CATALOG", width)...)
	for _, dep := range docs.Dependencies {
		lines = append(lines, fmt.Sprintf("  %s•%s %s%s%s %s(%s)%s — %s",
			th.accent, th.reset, th.bold+th.fg, dep.Name, th.reset, th.muted, dep.Kind, th.reset, dep.Purpose))
	}
	return lines
}

func (s *State) renderDocsComponentsPage(th ansiTheme, width int) []string {
	docs := domain.ResolveProjectDocs(s.Overview, s.Project)
	lines := renderPageHeader(th, "COMPONENTS", "Package ownership & responsibility layers")
	lines = append(lines, th.renderTableHeader("   LAYER          COMPONENT                 PATH                        SUMMARY", width)...)
	for _, c := range docs.Components {
		lines = append(lines, fmt.Sprintf("  %s•%s %s[%s]%s %s%s%s %s(%s)%s — %s",
			th.accent, th.reset, th.purple, c.Layer, th.reset, th.bold+th.fg, c.Name, th.reset, th.muted, c.Path, th.reset, c.Summary))
	}
	return lines
}

func (s *State) renderDocsDecisionsPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	docs := domain.ResolveProjectDocs(s.Overview, s.Project)
	lines := renderPageHeader(th, "SCENARIOS & DECISIONS", "Runtime invariants & ADRs")
	var scenBody []string
	for _, sc := range docs.Scenarios {
		scenBody = append(scenBody, fmt.Sprintf("%s• %s:%s %s", th.bold+th.cyan, sc.Title, th.reset, sc.Summary))
	}
	lines = append(lines, th.renderPanelBox("RUNTIME SCENARIOS & INVARIANTS", "BEHAVIOR", panelW, scenBody)...)
	lines = append(lines, "")
	lines = append(lines, th.renderTableHeader("   CATEGORY       ARCHITECTURAL DECISION RECORD (ADR)", width)...)
	for _, d := range docs.Decisions {
		lines = append(lines, fmt.Sprintf("  %s•%s %s[%s]%s %s%s%s — %s",
			th.accent, th.reset, th.purple, d.Category, th.reset, th.bold+th.fg, d.Title, th.reset, d.Decision))
	}
	return lines
}

func (s *State) renderDiagramsPage(th ansiTheme, width int) []string {
	panelW := adaptivePanelWidth(width)
	docs := domain.ResolveProjectDocs(s.Overview, s.Project)
	lines := renderPageHeader(th, "ARCHITECTURE DIAGRAM", "Component topology blueprint")
	diagramLines := domain.RenderArchitectureASCII(docs)
	lines = append(lines, th.renderPanelBox("COMPONENT TOPOLOGY BLUEPRINT", docs.Context.Topology, panelW, diagramLines)...)
	return lines
}
