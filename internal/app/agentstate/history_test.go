package agentstate

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestProgressIsAtomicPrivateAndBoundedAcrossReopen(t *testing.T) {
	s, key, admission := inputFixture(t, false)
	before, err := s.History(t.Context())
	if err != nil || before.Sequence != 2 || before.Epoch == "" {
		t.Fatalf("initial history: %+v %v", before, err)
	}
	event := Event{ID: "node-event", Time: time.Now(), Data: json.RawMessage(`{"prompt":"PRIVATE_SOURCE","response":"PRIVATE_RESULT"}`), Progress: &NodeProgress{NodeID: "human_review", State: "waiting_input"}}
	if _, err := s.AppendEvent(t.Context(), key, event, nil); err != nil {
		t.Fatal(err)
	}
	write := s.write
	s.write = func(string, []byte, os.FileMode, func() error) error { return errors.New("disk unavailable") }
	if _, err := s.ReceiveInput(t.Context(), admission, "approve"); !errors.Is(err, ErrPersistence) {
		t.Fatalf("receipt write: %v", err)
	}
	s.write = write
	if err := s.mutate(t.Context(), func(db *database) error {
		for range MaxProgressEvents {
			if err := appendProgress(db, db.Runs[admission.RunID], "node", event.Progress); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(s.root)
	if err != nil {
		t.Fatal(err)
	}
	after, err := reopened.History(t.Context())
	if err != nil || after.Epoch != before.Epoch || after.Sequence != 3+MaxProgressEvents || len(after.Events) != MaxProgressEvents || after.Events[0].Sequence != 4 || after.Runs[0].Input.Decision != "" {
		t.Fatalf("history after eviction: %+v %v", after, err)
	}
	data, err := json.Marshal(after)
	if err != nil || strings.Contains(string(data), "PRIVATE_") {
		t.Fatal("private native content leaked into progress", err)
	}
	if _, err := reopened.ReceiveInput(t.Context(), admission, "approve"); err != nil {
		t.Fatal(err)
	}
	if err := reopened.FinishInput(t.Context(), admission); err != nil {
		t.Fatal(err)
	}
	done, err := reopened.History(t.Context())
	if err != nil || done.Events[len(done.Events)-1].Kind != "input_finished" || done.Events[len(done.Events)-1].State != "completed" {
		t.Fatalf("lost terminal input: %v", err)
	}
	if _, err := reopened.ReceiveInput(t.Context(), admission, "approve"); err != nil {
		t.Fatal(err)
	}
	again, err := reopened.History(t.Context())
	if err != nil || again.Sequence != done.Sequence {
		t.Fatal("duplicate receipt appended progress", err)
	}
}

func TestProgressParentIdentityAndSessionRetention(t *testing.T) {
	s, key, admission := inputFixture(t, false)
	if _, err := s.ReceiveInput(t.Context(), admission, "cancel"); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishInput(t.Context(), admission); err != nil {
		t.Fatal(err)
	}
	history, err := s.History(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	parent := history.Runs[0].Run
	parent.ID, parent.Status, parent.Input, parent.SchemaVersion = "parent", "running", nil, "change-graph-v1"
	if _, err := s.BeginRun(t.Context(), parent, nil); err != nil {
		t.Fatal(err)
	}
	child := parent
	child.ID, child.ParentID, child.Session.ID = "child", parent.ID, "child-session"
	if _, err := s.CreateSession(t.Context(), child.Session, State{}); err != nil {
		t.Fatal(err)
	}
	bad := child
	bad.ParentID = "missing"
	if _, err := s.BeginRun(t.Context(), bad, nil); err == nil {
		t.Fatal("accepted missing parent")
	}
	if _, err := s.BeginRun(t.Context(), child, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishRun(t.Context(), child.ID, "completed", nil); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishRun(t.Context(), parent.ID, "completed", nil); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSession(t.Context(), key); err == nil {
		t.Fatal("deleted retained child's parent")
	}
	if err := s.DeleteSession(t.Context(), child.Session); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSession(t.Context(), key); err != nil {
		t.Fatal(err)
	}
	retained, err := s.History(t.Context())
	if err != nil || len(retained.Runs) != 0 || retained.Events[len(retained.Events)-2].RootID != parent.ID {
		t.Fatal("deleted session erased progress or lost ancestry", err)
	}
}

func TestProgressLegacyAndCorruptSequences(t *testing.T) {
	s, _, _ := inputFixture(t, false)
	db, err := s.load()
	if err != nil {
		t.Fatal(err)
	}
	for _, corrupt := range []func(*database){
		func(db *database) { db.Progress[1].Sequence++ },
		func(db *database) { db.Progress[1].State = "success-with-source" },
		func(db *database) { db.ProgressEpoch = "" },
	} {
		copy := *db
		copy.Progress = append([]ProgressEvent(nil), db.Progress...)
		corrupt(&copy)
		if validateProgress(&copy) == nil {
			t.Fatal("corrupt progress accepted")
		}
	}
	db.ProgressEpoch, db.ProgressSequence, db.Progress = "", 0, nil
	if err := validateProgress(db); err != nil {
		t.Fatalf("legacy journal rejected: %v", err)
	}
}
