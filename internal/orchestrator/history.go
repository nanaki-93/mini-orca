// Package orchestrator provides the main orchestration logic for the Mini-Orca pipeline.
package orchestrator

import (
	"fmt"
	"strings"
	"time"
)

// HistoryEvent represents a single event in the session history.
type HistoryEvent struct {
	// ID is the unique identifier for the event.
	ID string
	// SessionID is the ID of the session this event belongs to.
	SessionID string
	// EventType is the type of event (phase_transition, llm_call, file_operation).
	EventType string
	// Timestamp is when the event occurred.
	Timestamp time.Time
	// Summary is a brief summary of the event.
	Summary string
	// Details contains detailed information about the event.
	Details string
	// Error contains any error that occurred during the event.
	Error string
	// Duration is how long the event took.
	Duration time.Duration
}

// HistoryTracker manages session history tracking.
type HistoryTracker struct {
	sessionID string
	history   []HistoryEvent
}

// NewHistoryTracker creates a new HistoryTracker instance.
func NewHistoryTracker(sessionID string) *HistoryTracker {
	return &HistoryTracker{
		sessionID: sessionID,
		history:   make([]HistoryEvent, 0),
	}
}

// LogPhaseTransition logs a phase transition event.
func (h *HistoryTracker) LogPhaseTransition(from, to Phase) {
	event := HistoryEvent{
		ID:        fmt.Sprintf("hist-%d", time.Now().UnixNano()),
		SessionID: h.sessionID,
		EventType: "phase_transition",
		Timestamp: time.Now(),
		Summary:   fmt.Sprintf("Transitioned from %s to %s", from, to),
		Details:   fmt.Sprintf("From: %s, To: %s", from, to),
	}

	h.addEvent(event)
}

// LogLLMCall logs an LLM call event with input/output summary.
func (h *HistoryTracker) LogLLMCall(agent string, input string, output string, duration time.Duration, err error) {
	event := HistoryEvent{
		ID:        fmt.Sprintf("hist-%d", time.Now().UnixNano()),
		SessionID: h.sessionID,
		EventType: "llm_call",
		Timestamp: time.Now(),
		Summary:   fmt.Sprintf("LLM call to %s agent", agent),
		Details:   h.formatLLMDetails(agent, input, output, err),
		Duration:  duration,
	}

	if err != nil {
		event.Error = err.Error()
	}

	h.addEvent(event)
}

// LogFileOperation logs a file operation event.
func (h *HistoryTracker) LogFileOperation(operation string, path string, err error) {
	event := HistoryEvent{
		ID:        fmt.Sprintf("hist-%d", time.Now().UnixNano()),
		SessionID: h.sessionID,
		EventType: "file_operation",
		Timestamp: time.Now(),
		Summary:   fmt.Sprintf("%s: %s", operation, path),
		Details:   fmt.Sprintf("Operation: %s, Path: %s", operation, path),
	}

	if err != nil {
		event.Error = err.Error()
	}

	h.addEvent(event)
}

// GetHistory returns all history events.
func (h *HistoryTracker) GetHistory() []HistoryEvent {
	return h.history
}

// PersistToStore persists all history events.
// Note: This method is now a no-op since session persistence has been removed.
func (h *HistoryTracker) PersistToStore() error {
	return nil
}

// addEvent adds an event to the history.
func (h *HistoryTracker) addEvent(event HistoryEvent) {
	h.history = append(h.history, event)
}

// formatLLMDetails formats LLM call details for logging.
func (h *HistoryTracker) formatLLMDetails(agent string, input string, output string, err error) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Agent: %s\n", agent))
	sb.WriteString(fmt.Sprintf("Input (first 500 chars): %s\n", truncateString(input, 500)))
	sb.WriteString(fmt.Sprintf("Output (first 500 chars): %s\n", truncateString(output, 500)))

	if err != nil {
		sb.WriteString(fmt.Sprintf("Error: %v\n", err))
	}

	return sb.String()
}

// truncateString truncates a string to the specified length.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
