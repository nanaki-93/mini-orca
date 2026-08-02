package state

import (
	"fmt"
	"testing"
	"time"
)

func TestStore_Session(t *testing.T) {
	s := NewStore(nil)
	sess := &Session{
		ID:     "sess-1",
		Status: SessionStatusPending,
	}

	t.Run("Save and Get Session", func(t *testing.T) {
		err := s.SaveSession(sess)
		if err != nil {
			t.Fatalf("SaveSession failed: %v", err)
		}

		got, err := s.GetSession("sess-1")
		if err != nil {
			t.Fatalf("GetSession failed: %v", err)
		}
		if got.ID != sess.ID {
			t.Errorf("expected ID %s, got %s", sess.ID, got.ID)
		}
		if got.Status != sess.Status {
			t.Errorf("expected status %s, got %s", sess.Status, got.Status)
		}
	})

	t.Run("Get Non-existent Session", func(t *testing.T) {
		_, err := s.GetSession("non-existent")
		if err == nil {
			t.Error("expected error for non-existent session")
		}
	})
}

func TestStore_History(t *testing.T) {
	s := NewStore(nil)
	history := PhaseHistory{
		ID:        "h1",
		SessionID: "sess-1",
		Phase:     "planning",
		Status:    PhaseStatusCompleted,
		StartedAt: time.Now(),
	}

	t.Run("Add and Get History", func(t *testing.T) {
		err := s.SavePhaseHistory(&history)
		if err != nil {
			t.Fatalf("SavePhaseHistory failed: %v", err)
		}

		got, err := s.GetSessionHistory("sess-1")
		if err != nil {
			t.Fatalf("GetSessionHistory failed: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("expected 1 history entry, got %d", len(got))
		}
		if got[0].Phase != "planning" {
			t.Errorf("expected planning, got %s", got[0].Phase)
		}
	})
}

func TestStore_TestResults(t *testing.T) {
	s := NewStore(nil)
	res := TestResult{
		ID:        "tr-1",
		SessionID: "sess-1",
		UnitID:    "unit-1",
		Passed:    true,
	}

	t.Run("Save and Get Test Results", func(t *testing.T) {
		err := s.SaveTestResult(&res)
		if err != nil {
			t.Fatalf("SaveTestResult failed: %v", err)
		}

		got, err := s.GetTestResults("sess-1")
		if err != nil {
			t.Fatalf("GetTestResults failed: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("expected 1 result, got %d", len(got))
		}
		if got[0].ID != "tr-1" {
			t.Errorf("expected tr-1, got %s", got[0].ID)
		}
	})
}

func TestStore_ReviewReports(t *testing.T) {
	s := NewStore(nil)
	rep := ReviewReportEntry{
		ID:        "rr-1",
		SessionID: "sess-1",
		UnitID:    "unit-1",
		Score:     90,
	}

	t.Run("Save and Get Review Reports", func(t *testing.T) {
		err := s.SaveReviewReport(&rep)
		if err != nil {
			t.Fatalf("SaveReviewReport failed: %v", err)
		}

		got, err := s.GetReviewReports("sess-1")
		if err != nil {
			t.Fatalf("GetReviewReports failed: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("expected 1 report, got %d", len(got))
		}
		if got[0].ID != "rr-1" {
			t.Errorf("expected rr-1, got %s", got[0].ID)
		}
	})
}

func TestStore_ListSessions(t *testing.T) {
	s := NewStore(nil)
	s.SaveSession(&Session{ID: "s1"})
	s.SaveSession(&Session{ID: "s2"})

	sessions := s.ListSessions()
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestStore_More(t *testing.T) {
	s := NewStore(nil)
	sess := &Session{ID: "s1"}
	s.SaveSession(sess)

	t.Run("UpdatePhaseStatus", func(t *testing.T) {
		err := s.UpdatePhaseStatus(PhaseCoding, SessionStatusRunning)
		if err != nil {
			t.Errorf("UpdatePhaseStatus failed: %v", err)
		}
		sessGot, _ := s.GetSession("s1")
		if sessGot.CurrentPhase != PhaseCoding {
			t.Errorf("expected phase coding, got %s", sessGot.CurrentPhase)
		}
	})

	t.Run("SetError", func(t *testing.T) {
		err := s.SetError("s1", fmt.Errorf("some error"))
		if err != nil {
			t.Errorf("SetError failed: %v", err)
		}
		sessGot, _ := s.GetSession("s1")
		if sessGot.Error != "some error" || sessGot.Status != SessionStatusFailed {
			t.Errorf("expected error and status failed, got %s, %s", sessGot.Error, sessGot.Status)
		}
	})
}
