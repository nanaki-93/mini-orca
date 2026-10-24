package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

func parentPage(page Page) Page {
	switch page {
	case PageIssueDetail:
		return PageBugs
	case PageFeatureDetail:
		return PageFeatures
	case PageWorktreeDetail:
		return PageGit
	default:
		return page
	}
}

func (s *State) detailSections() []string {
	switch s.Page {
	case PagePlan:
		if s.PlanView == "mission" {
			if s.Mission != nil && s.Mission.Draft != nil && !s.Mission.History {
				return []string{"Overview", "Children", "Scope", "Budget"}
			}
			return nil
		}
		if s.PlanDraft == nil {
			return nil
		}
		return []string{"Approach", "Evidence", "Scope", "Acceptance"}
	case PageIssueDetail:
		return []string{"Problem", "Evidence", "Solution"}
	case PageFeatureDetail:
		return []string{"Goal", "Evidence", "Acceptance"}
	}
	return nil
}

func (s *State) handleFocusedPageKey(ev KeyEvent) bool {
	if s.handleSourceViewportKey(ev.Key) {
		return true
	}
	if (ev.Key == "ArrowLeft" || ev.Key == "ArrowRight" || ev.Key == "h" || ev.Key == "l") && len(s.detailSections()) > 0 {
		delta := 1
		if ev.Key == "ArrowLeft" || ev.Key == "h" {
			delta = -1
		}
		s.DetailSection = (s.DetailSection + delta + len(s.detailSections())) % len(s.detailSections())
		s.DetailScroll = 0
		return true
	}
	if parentPage(s.Page) == PageGit {
		return s.handleWorktreeKey(ev.Key)
	}
	if s.Page == PageFeatureDetail {
		switch ev.Key {
		case "Escape", "Backspace", "i":
			s.Navigate(PageFeatures)
			return true
		case "f", "a", "Enter":
			features := s.activeFeatures()
			if len(features) > 0 {
				s.dispatchFeatureIdea(features[s.ClampedIdx()])
			}
			return true
		}
	}
	return false
}

func (s *State) renderDetailSections(th ansiTheme) []string {
	labels := s.detailSections()
	var tabs []string
	for i, label := range labels {
		if i == s.DetailSection {
			tabs = append(tabs, th.bold+th.cyan+"▸ "+label+th.reset)
		} else {
			tabs = append(tabs, th.muted+label+th.reset)
		}
	}
	return []string{"  " + strings.Join(tabs, "  ·  "), "  " + th.muted + "←/→: section · Tab: menu · PgUp/PgDn: scroll" + th.reset, ""}
}

func (s *State) renderPlanSection(th ansiTheme, width int) []string {
	rows := s.renderPlanDraftSection(th, width)
	return append(rows, s.renderInvestigationSuggestions(th, issuePanelContentWidth(adaptivePanelWidth(width)))...)
}

func (s *State) renderPlanDraftSection(th ansiTheme, width int) []string {
	d := s.PlanDraft
	p := d.Seed.Plan
	contentW := issuePanelContentWidth(adaptivePanelWidth(width))
	switch s.DetailSection {
	case 1:
		rows := renderDiagnosticRow(th, th.cyan, "Evidence:", th.fg, client.FirstNonEmpty(p.Evidence, "No reproduction evidence recorded. Verify the hypothesis before applying a fix."), contentW)
		return append(rows, renderPlanCitations(th, p.Citations, p.Assumptions, contentW)...)
	case 2:
		rows := renderDiagnosticBulletRows(th, th.cyan, "Write scope:", th.cyan, th.fg, d.Seed.Paths, contentW)
		rows = append(rows, renderDiagnosticBulletRows(th, th.cyan, "Read context:", th.cyan, th.fg, p.ReadPaths, contentW)...)
		rows = append(rows, renderDiagnosticBulletRows(th, th.purple, "Non-goals:", th.purple, th.fg, p.NonGoals, contentW)...)
		if s.Change != nil {
			rows = append(rows, renderDiagnosticBulletRows(th, th.amber, "Suggested scope (add explicitly):", th.amber, th.fg, s.Change.ScopeSuggestions, contentW)...)
		}
		return append(rows, "[f] Edit write scope · :context · :boundaries", "Context policy may include related declarations and AGENTS.md guides.")
	case 3:
		rows := renderDiagnosticBulletRows(th, th.purple, "Acceptance:", th.purple, th.fg, d.Seed.AcceptanceCriteria, contentW)
		repairs := "Off — inspect failures before retrying"
		if d.Seed.AutoRepair {
			repairs = "On — up to 2 repairs within approved scope"
		}
		rows = append(rows, renderDiagnosticRow(th, th.amber, "Auto repair:", th.fg, repairs, contentW)...)
		return append(rows, "[t] Edit criteria · :repairs on|off")
	default:
		rows := renderDiagnosticRow(th, th.amber, "Problem / goal:", th.fg, p.Problem, contentW)
		rows = append(rows, renderDiagnosticRow(th, th.green, "Approach:", th.fg, p.Approach, contentW)...)
		return append(rows, "[e] Edit approach · :problem <description>")
	}
}

func (s *State) renderFeatureDetailPage(th ansiTheme, width int) []string {
	lines := renderPageHeader(th, "FEATURE", "[f] Plan implementation · [Esc] Back")
	features := s.activeFeatures()
	if len(features) == 0 {
		return append(lines, "  No feature selected.")
	}
	feature := features[s.ClampedIdx()]
	lines = append(lines, s.renderDetailSections(th)...)
	contentW := issuePanelContentWidth(adaptivePanelWidth(width))
	var rows []string
	switch s.DetailSection {
	case 1:
		rows = renderDiagnosticRow(th, th.cyan, "Evidence:", th.fg, feature.Evidence, contentW)
		rows = append(rows, renderDiagnosticBulletRows(th, th.cyan, "Suggested files:", th.cyan, th.fg, feature.Paths, contentW)...)
	case 2:
		rows = renderDiagnosticBulletRows(th, th.purple, "Acceptance:", th.purple, th.fg, feature.AcceptanceCriteria, contentW)
	default:
		rows = renderDiagnosticRow(th, th.cyan, "Goal:", th.fg, feature.Title, contentW)
		rows = append(rows, renderDiagnosticRow(th, th.green, "Benefit:", th.fg, feature.Benefit, contentW)...)
		rows = append(rows, fmt.Sprintf("Suggested effort: %s · Model suggestion", client.FirstNonEmpty(feature.Effort, "unspecified")))
	}
	return append(lines, th.renderPanelBox(s.detailSections()[s.DetailSection], "", adaptivePanelWidth(width), rows)...)
}
