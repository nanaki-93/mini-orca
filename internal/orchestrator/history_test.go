package orchestrator

import (
	"testing"
	"time"
)

func TestHistoryTracker(t *testing.T) {
	tracker := NewHistoryTracker("sess-1")

	t.Run("LogPhase", func(t *testing.T) {
		tracker.LogPhase(PhaseCoding, "success", "Code generated")
		history := tracker.GetHistory()
		if len(history) != 1 {
			t.Errorf("expected 1 entry, got %d", len(history))
		}
		if history[0].Phase != PhaseCoding {
			t.Errorf("expected phase %s, got %s", PhaseCoding, history[0].Phase)
		}
		if history[0].Status != "success" {
			t.Errorf("expected status 'success', got %s", history[0].Status)
		}
	})

	t.Run("MultiplePhases", func(t *testing.T) {
		tracker.LogPhase(PhaseTesting, "success", "Tests passed")
		tracker.LogPhase(PhaseReview, "failed", "Issues found")

		history := tracker.GetHistory()
		if len(history) != 3 {
			t.Errorf("expected 3 entries, got %d", len(history))
		}
	})

	t.Run("GetPhaseStatus", func(t *testing.T) {
		status := tracker.GetPhaseStatus(PhaseTesting)
		if status != "success" {
			t.Errorf("expected status 'success', got %s", status)
		}

		unknownStatus := tracker.GetPhaseStatus(PhaseHumanReview)
		if unknownStatus != "" {
			t.Errorf("expected empty status for unknown phase, got %s", unknownStatus)
		}
	})

	t.Run("CountEntries", func(t *testing.T) {
		count := tracker.CountEntries()
		if count != 3 {
			t.Errorf("expected 3 entries, got %d", count)
		}
	})

	t.Run("GetHistory", func(t *testing.T) {
		history := tracker.GetHistory()
		if len(history) != 3 {
			t.Errorf("expected 3 entries, got %d", len(history))
		}
	})
}

func TestPhaseLogEntry_Struct(t *testing.T) {
	entry := PhaseLogEntry{
		Phase:     PhaseCoding,
		Status:    "success",
		Timestamp: time.Now(),
		Output:    "package main",
	}

	if entry.Phase != PhaseCoding {
		t.Errorf("expected phase %s, got %s", PhaseCoding, entry.Phase)
	}
	if entry.Status != "success" {
		t.Errorf("expected status 'success', got %s", entry.Status)
	}
	if entry.Output != "package main" {
		t.Errorf("expected output 'package main', got %s", entry.Output)
	}
}
