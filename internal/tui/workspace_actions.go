package tui

func (s *State) pageActions() []keyDesc {
	switch s.EffectivePage() {
	case PageSummary:
		if s.Project == nil {
			return []keyDesc{{"Enter", "Open project"}}
		}
		return []keyDesc{{"Enter", "Open"}, {"n", "New task"}, {"r", "Analyze"}, {"o", "Switch project"}}
	case PageRepo:
		return []keyDesc{{"o", "Switch project"}, {"r", "Reindex"}}
	case PageProject:
		return []keyDesc{{"t", "Run checks"}, {"↑/↓", "Select"}, {"PgUp/PgDn", "Details"}}
	case PageBugs:
		return []keyDesc{{"Enter", "Plan fix"}, {"i", "Details"}, {"Space", "Select"}, {"s/t", "Filters"}}
	case PageAgents:
		if s.AgentDetailOpen {
			return []keyDesc{{"↑/↓", "Stage"}, {"d", "Diff"}, {"r", "Retry"}, {"Esc", "Back"}}
		}
		return []keyDesc{{"Enter", "Inspect"}, {"n", "New task"}, {"d", "Diff"}}
	case PageFeatures:
		return []keyDesc{{"Enter", "Details"}, {"f", "Plan"}, {"r", "Analyze"}}
	case PageEditor:
		if s.isViewingDiff() {
			return []keyDesc{{"w", "Reviewed"}, {"a", "Apply"}, {"f", "Next file"}, {"e", "Details"}}
		}
		if s.File != nil && !s.BrowseFiles {
			return []keyDesc{{"Esc", "Files"}, {"h/l", "Pan"}, {"PgUp/PgDn", "Scroll"}}
		}
		return []keyDesc{{"Enter", "Read file"}, {"d", "Diff"}}
	default:
		return lookupPageHelpSpec(s.EffectivePage()).primary
	}
}
