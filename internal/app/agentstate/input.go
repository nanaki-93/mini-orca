package agentstate

import (
	"context"
	"fmt"
	"time"
)

// InputCheckpoint is a host-owned receipt. SDK events and session state cannot
// create or change it. Waiting preserves recovery data without a live worker.
type InputCheckpoint struct {
	RequestID    string    `json:"request_id"`
	NodeID       string    `json:"node_id"`
	InvocationID string    `json:"invocation_id"`
	ExpiresAt    time.Time `json:"expires_at"`
	Decision     string    `json:"decision,omitempty"`
	ReceivedAt   time.Time `json:"received_at,omitempty"`
}

type InputAdmission struct {
	RunID, RequestID, RequestHash string
	Binding                       Binding
}

func validateInputCheckpoint(run Run) error {
	p := run.Input
	if p == nil {
		if run.Status == "waiting_input" || run.Status == "expired" {
			return ErrCorrupt
		}
		return nil
	}
	if run.SchemaVersion != "confirmation-v1" || !validID(p.RequestID) || !validID(p.InvocationID) || p.NodeID != "human_review" || p.ExpiresAt.Before(run.Started) || p.ExpiresAt.After(run.Started.Add(25*time.Hour)) {
		return ErrCorrupt
	}
	return validateInputReceipt(run)
}

func validateInputReceipt(run Run) error {
	p := run.Input
	if p.Decision == "" {
		if run.Status != "waiting_input" || !p.ReceivedAt.IsZero() {
			return ErrCorrupt
		}
		return nil
	}
	if !validInputDecision(p.Decision) || p.ReceivedAt.Before(run.Started) {
		return ErrCorrupt
	}
	if (p.Decision == "expire") != !p.ReceivedAt.Before(p.ExpiresAt) {
		return ErrCorrupt
	}
	if run.Status != "waiting_input" && run.Status != inputTerminalStatus(p.Decision) {
		return ErrCorrupt
	}
	return nil
}

func validInputDecision(decision string) bool {
	return decision == "approve" || decision == "reject" || decision == "cancel" || decision == "expire"
}

func inputTerminalStatus(decision string) string {
	switch decision {
	case "cancel":
		return "canceled"
	case "expire":
		return "expired"
	default:
		return "completed"
	}
}

func (s *Store) PauseForInput(ctx context.Context, id string, checkpoint InputCheckpoint) error {
	return s.mutate(ctx, func(db *database) error {
		run, ok := db.Runs[id]
		if !ok {
			return ErrNotFound
		}
		if run.Status != "running" || run.Input != nil || checkpoint.Decision != "" {
			return fmt.Errorf("invocation cannot create another input checkpoint")
		}
		run.Input, run.Status, run.Updated = &checkpoint, "waiting_input", time.Now().UTC()
		db.Runs[id] = run
		return nil
	})
}

func admittedInputRun(db *database, admission InputAdmission) (Run, error) {
	run, ok := db.Runs[admission.RunID]
	if !ok {
		return Run{}, ErrNotFound
	}
	if run.Input == nil || run.RequestHash != admission.RequestHash || run.Input.RequestID != admission.RequestID || db.Budgets[run.BudgetID].Binding != admission.Binding {
		return Run{}, fmt.Errorf("input checkpoint does not match current admission")
	}
	return run, nil
}

func (s *Store) InputRun(ctx context.Context, admission InputAdmission) (Run, error) {
	if err := context.Cause(ctx); err != nil {
		return Run{}, err
	}
	db, err := s.load()
	if err != nil {
		return Run{}, err
	}
	return admittedInputRun(db, admission)
}

// ReceiveInput saves the decision before any native resume event. A retry of
// the same response retains the original timestamp; conflicting replies fail.
func (s *Store) ReceiveInput(ctx context.Context, admission InputAdmission, decision string) (Run, error) {
	if !validInputDecision(decision) || decision == "expire" {
		return Run{}, fmt.Errorf("input decision must be approve, reject or cancel")
	}
	var result Run
	err := s.mutate(ctx, func(db *database) error {
		run, err := admittedInputRun(db, admission)
		if err != nil {
			return err
		}
		if err := receiveInput(&run, decision, time.Now().UTC()); err != nil {
			return err
		}
		db.Runs[run.ID], result = run, run
		return nil
	})
	return result, err
}

func receiveInput(run *Run, decision string, now time.Time) error {
	p := run.Input
	if p.Decision != "" {
		if p.Decision != decision && p.Decision != "expire" {
			return fmt.Errorf("input request already received a different decision")
		}
		return nil
	}
	if run.Status != "waiting_input" {
		return fmt.Errorf("invocation is not waiting for input")
	}
	if !now.Before(p.ExpiresAt) {
		decision = "expire"
	}
	p.Decision, p.ReceivedAt, run.Updated = decision, now, now
	return nil
}

func (s *Store) FinishInput(ctx context.Context, admission InputAdmission) error {
	return s.mutate(ctx, func(db *database) error {
		run, err := admittedInputRun(db, admission)
		if err != nil {
			return err
		}
		if run.Input.Decision == "" {
			return fmt.Errorf("input receipt is required before completing a checkpoint")
		}
		if run.Status == "waiting_input" {
			run.Status, run.Updated = inputTerminalStatus(run.Input.Decision), time.Now().UTC()
			db.Runs[run.ID] = run
		}
		return nil
	})
}

func unfinishedRun(run Run) bool { return run.Status == "running" || run.Status == "waiting_input" }
