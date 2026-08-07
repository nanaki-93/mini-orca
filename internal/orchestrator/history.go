// Package orchestrator provides the main orchestration logic for the Mini-Orca pipeline.
package orchestrator

import (
	"time"
)

// PhaseLogEntry represents a single phase execution in the pipeline.
type PhaseLogEntry struct {
	// Phase is the pipeline phase that was executed.
	Phase Phase
	// Status indicates the result of the phase execution.
	Status string
	// Timestamp records when the phase was executed.
	Timestamp time.Time
	// Output contains a brief summary of the phase output.
	Output string
}

// HistoryTracker manages simple phase execution logs for a session.
type HistoryTracker struct {
	sessionID string
	entries   []PhaseLogEntry
}

// NewHistoryTracker creates a new HistoryTracker instance.
func NewHistoryTracker(sessionID string) *HistoryTracker {
	return &HistoryTracker{
		sessionID: sessionID,
		entries:   make([]PhaseLogEntry, 0),
	}
}

// LogPhase logs a phase execution with its status and output.
func (h *HistoryTracker) LogPhase(phase Phase, status string, output string) {
	entry := PhaseLogEntry{
		Phase:     phase,
		Status:    status,
		Timestamp: time.Now(),
		Output:    output,
	}
	h.entries = append(h.entries, entry)
}

// GetHistory returns all phase log entries.
func (h *HistoryTracker) GetHistory() []PhaseLogEntry {
	return h.entries
}

// GetPhaseStatus returns the status of the most recent execution of the given phase.
func (h *HistoryTracker) GetPhaseStatus(phase Phase) string {
	for i := len(h.entries) - 1; i >= 0; i-- {
		if h.entries[i].Phase == phase {
			return h.entries[i].Status
		}
	}
	return ""
}

// CountEntries returns the total number of phase log entries.
func (h *HistoryTracker) CountEntries() int {
	return len(h.entries)
}
