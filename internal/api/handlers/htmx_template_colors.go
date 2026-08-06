package handlers

// ─── Phase Tracker Helpers ────────────────────────────────────────────────────

func phaseBorderClass(s string) string {
	switch s {
	case "completed":
		return "border-green-500"
	case "failed":
		return "border-red-500"
	case "in-progress":
		return "border-blue-500"
	case "waiting":
		return "border-yellow-500"
	default:
		return "border-dark-600"
	}
}

func phaseDotClass(s string) string {
	switch s {
	case "completed":
		return "bg-green-500"
	case "failed":
		return "bg-red-500"
	case "in-progress":
		return "bg-blue-500 animate-pulse"
	case "waiting":
		return "bg-yellow-500"
	default:
		return "bg-dark-600"
	}
}

func phaseProgressClass(s string) string {
	switch s {
	case "completed":
		return "bg-green-500"
	case "failed":
		return "bg-red-500"
	case "in-progress":
		return "bg-blue-500"
	case "waiting":
		return "bg-yellow-500"
	default:
		return "bg-dark-600"
	}
}

func phaseTextClass(s string) string {
	switch s {
	case "completed":
		return "text-green-500"
	case "failed":
		return "text-red-500"
	case "in-progress":
		return "text-blue-500 font-bold"
	case "waiting":
		return "text-yellow-500"
	default:
		return "text-text-secondary"
	}
}

// ─── Score and Coverage Helpers ───────────────────────────────────────────────

func scoreColor(p int) string {
	if p >= 80 {
		return "#22c55e"
	}
	if p >= 60 {
		return "#eab308"
	}
	if p >= 40 {
		return "#f97316"
	}
	return "#ef4444"
}

func coverageColor(p int) string {
	if p >= 80 {
		return "#22c55e"
	}
	if p >= 60 {
		return "#eab308"
	}
	if p >= 40 {
		return "#f97316"
	}
	return "#ef4444"
}

// ─── Issue Helpers ────────────────────────────────────────────────────────────

func totalIssues(issues []interface{}) int { return len(issues) }

func countBySeverity(issues []interface{}, severity string) int {
	count := 0
	for _, issue := range issues {
		if m, ok := issue.(map[string]interface{}); ok {
			if s, ok := m["severity"].(string); ok && s == severity {
				count++
			}
		}
	}
	return count
}

func filterBySeverity(issues []interface{}, severity string) []interface{} {
	result := []interface{}{}
	for _, issue := range issues {
		if m, ok := issue.(map[string]interface{}); ok {
			if s, ok := m["severity"].(string); ok && s == severity {
				result = append(result, issue)
			}
		}
	}
	return result
}

func isSkillAssigned(skillName string, skills []string) bool {
	for _, s := range skills {
		if s == skillName {
			return true
		}
	}
	return false
}
