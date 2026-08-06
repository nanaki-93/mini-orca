package handlers

// ─── Activity Log Helpers ─────────────────────────────────────────────────────

func getStats(entries []ActivityEntry) map[string]int {
	stats := map[string]int{"success": 0, "error": 0, "running": 0}
	for _, e := range entries {
		switch e.Status {
		case "success":
			stats["success"]++
		case "error":
			stats["error"]++
		case "running":
			stats["running"]++
		}
	}
	return stats
}

func logTypeClass(t string) string {
	switch t {
	case "llm":
		return "type-llm"
	case "file":
		return "type-file"
	case "test":
		return "type-test"
	case "review":
		return "type-review"
	case "system":
		return "type-system"
	default:
		return ""
	}
}

func logStatusClass(s string) string {
	switch s {
	case "error":
		return "status-error"
	case "running":
		return "status-running"
	default:
		return ""
	}
}

func typeBadgeClass(t string) string {
	switch t {
	case "llm":
		return "badge-llm"
	case "file":
		return "badge-file"
	case "test":
		return "badge-test"
	case "review":
		return "badge-review"
	case "system":
		return "badge-system"
	default:
		return ""
	}
}

// ─── Phase State Helpers ──────────────────────────────────────────────────────

func phaseState(s string) string {
	switch s {
	case "completed":
		return "completed"
	case "failed":
		return "failed"
	case "in_progress":
		return "in-progress"
	case "pending":
		return "waiting"
	default:
		return "pending"
	}
}

func phaseClass(s string) string {
	switch s {
	case "completed":
		return "border-green-500 text-green-500"
	case "failed":
		return "border-red-500 text-red-500"
	case "in-progress":
		return "border-blue-500 text-blue-500 ring-2 ring-blue-500/30"
	case "waiting":
		return "border-yellow-500 text-yellow-500"
	default:
		return "border-dark-600 text-dark-600"
	}
}

func phaseLabelClass(s string) string {
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
		return "text-dark-600"
	}
}

func phaseConnectorClass(s string) string {
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

func phaseProgress(s string) int {
	switch s {
	case "completed":
		return 100
	case "in-progress":
		return 75
	case "waiting":
		return 25
	default:
		return 0
	}
}

func currentPhaseDotClass(s string) string {
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

// ─── Priority and Badge Helpers ───────────────────────────────────────────────

func priorityClass(p string) string {
	switch p {
	case "high":
		return "priority-high"
	case "medium":
		return "priority-medium"
	case "low":
		return "priority-low"
	default:
		return ""
	}
}

func categoryBadgeClass(cat string) string {
	switch cat {
	case "knowledge":
		return "badge-knowledge"
	case "tool":
		return "badge-tool"
	default:
		return "badge-default"
	}
}

// ─── Agent Helpers ────────────────────────────────────────────────────────────

func agentIconClass(name string) string {
	switch name {
	case "coder":
		return "bg-green-500/20 text-green-400"
	case "tester":
		return "bg-yellow-500/20 text-yellow-400"
	case "reviewer":
		return "bg-purple-500/20 text-purple-400"
	default:
		return "bg-dark-600 text-text-secondary"
	}
}

func agentIcon(name string) string {
	switch name {
	case "coder":
		return `<svg class="w-4 h-4" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M13.5 2a2 2 0 012 2v12a2 2 0 01-2 2h-7a2 2 0 01-2-2V4a2 2 0 012-2h7zm0 2h-7v12h7V4zM7 7a.5.5 0 01.5-.5h5a.5.5 0 010 1h-5A.5.5 0 017 7zm0 2.5a.5.5 0 01.5-.5h5a.5.5 0 010 1h-5a.5.5 0 01-.5-.5zm0 2.5a.5.5 0 01.5-.5h3a.5.5 0 010 1h-3a.5.5 0 01-.5-.5z" clip-rule="evenodd"/></svg>`
	case "tester":
		return `<svg class="w-4 h-4" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M15.312 11.424a5.5 5.5 0 01-9.201 2.466l-.312-.311h2.433a.5.5 0 000-1H3.999A.5.5 0 003.5 15v3a.5.5 0 00.5.5h3a.5.5 0 00.5-.5v-2.433l.311.312a7.5 7.5 0 0012.548-3.362.5.5 0 00-.049-.586l-.002-.001zM10 4a6 6 0 100 12A6 6 0 0010 4z" clip-rule="evenodd"/></svg>`
	case "reviewer":
		return `<svg class="w-4 h-4" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path d="M10 12.5a2.5 2.5 0 100-5 2.5 2.5 0 000 5z"/><path fill-rule="evenodd" d="M.664 10.59a1.651 1.651 0 010-1.186A10.004 10.004 0 0110 3c4.257 0 7.893 2.66 9.336 6.41.147.381.146.804 0 1.186A10.004 10.004 0 0110 17c-4.257 0-7.893-2.66-9.336-6.41zM14 10a4 4 0 11-8 0 4 4 0 018 0z" clip-rule="evenodd"/></svg>`
	default:
		return `<svg class="w-4 h-4" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M10 9a3 3 0 100-6 3 3 0 000 6zm-7 9a7 7 0 1114 0H3z" clip-rule="evenodd"/></svg>`
	}
}
