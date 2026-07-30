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

func TestStore_Plan(t *testing.T) {
	s := NewStore(nil)
	sess := &Session{ID: "sess-1"}
	s.SaveSession(sess)

	plan := &Plan{
		ID:        "plan-1",
		SessionID: "sess-1",
		Status:    PlanStatusDraft,
		Units: []PlanUnit{
			{ID: "unit-1", Status: UnitStatusPending},
		},
	}

	t.Run("Save and Get Plan", func(t *testing.T) {
		err := s.SavePlan(plan)
		if err != nil {
			t.Fatalf("SavePlan failed: %v", err)
		}

		got, err := s.GetPlan("plan-1")
		if err != nil {
			t.Fatalf("GetPlan failed: %v", err)
		}
		if got.ID != plan.ID {
			t.Errorf("expected ID %s, got %s", plan.ID, got.ID)
		}

		// Verify session plan reference was updated
		sessGot, _ := s.GetSession("sess-1")
		if sessGot.Plan == nil || sessGot.Plan.ID != "plan-1" {
			t.Error("session plan reference not updated")
		}
	})

	t.Run("Update Unit Status", func(t *testing.T) {
		err := s.UpdateUnitStatus("unit-1", UnitStatusCoding)
		if err != nil {
			t.Fatalf("UpdateUnitStatus failed: %v", err)
		}

		got, _ := s.GetPlan("plan-1")
		if got.Units[0].Status != UnitStatusCoding {
			t.Errorf("expected StatusCoding, got %s", got.Units[0].Status)
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
	plan := &Plan{ID: "p1", SessionID: "s1", Units: []PlanUnit{{ID: "u1", Status: UnitStatusPending}}}
	s.SavePlan(plan)

	t.Run("GetPendingUnits", func(t *testing.T) {
		units, err := s.GetPendingUnits("p1")
		if err != nil || len(units) != 1 {
			t.Errorf("expected 1 pending unit, got %d, err: %v", len(units), err)
		}
	})

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

	t.Run("SaveUnitCode", func(t *testing.T) {
		err := s.SaveUnitCode("s1", "u1", "println('hello')")
		if err != nil {
			t.Errorf("SaveUnitCode failed: %v", err)
		}
		planGot, _ := s.GetPlan("p1")
		if planGot.Units[0].GeneratedCode != "println('hello')" {
			t.Errorf("expected code, got %s", planGot.Units[0].GeneratedCode)
		}
	})

	t.Run("GetPlanBySession", func(t *testing.T) {
		p, err := s.GetPlanBySession("s1")
		if err != nil || p == nil {
			t.Errorf("expected plan, got %v, err: %v", p, err)
		}
	})
}
