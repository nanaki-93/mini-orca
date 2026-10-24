package app

import "testing"

func TestWorkflowSnapshotReadAcrossCompletion(t *testing.T) {
	s, _ := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	for _, tc := range []struct {
		name, root, id, failure, freshness, want string
		present, finished                        bool
	}{
		{name: "active", present: true, root: s.manager.Root(), id: "workflow", want: "running"},
		{name: "finished after snapshot load", present: true, finished: true, root: s.manager.Root(), id: "workflow", want: "running"},
		{name: "failed persistence", present: true, finished: true, root: s.manager.Root(), id: "workflow", failure: "progress could not be saved", want: "failed"},
		{name: "restored without worker", want: "interrupted"},
		{name: "different checkout", present: true, finished: true, root: "/different", id: "workflow", want: "interrupted"},
		{name: "different workflow", present: true, finished: true, root: s.manager.Root(), id: "other", want: "interrupted"},
		{name: "stale source", present: true, finished: true, root: s.manager.Root(), id: "workflow", freshness: "stale", want: "stale"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// ChangeSession loads this snapshot before consulting worker ownership.
			// A finished worker has already stored its final state atomically; the
			// older snapshot remains a running observation, not an orphaned job.
			snapshot := &ChangeSession{ID: "change", Freshness: tc.freshness, Workflow: &ChangeWorkflow{ID: "workflow", Status: "running", Stages: []ChangeWorkflowStage{{Name: "create", Status: "completed"}, {Name: "test", Status: "completed"}, {Name: "review", Status: "running"}, {Name: "human_review", Status: "blocked"}}}}
			s.changeWorkflow = changeWorkflowController{}
			if tc.present {
				s.changeWorkflow.setJob(&changeWorkflowJob{sessionID: "change", id: tc.id, root: tc.root, finished: tc.finished, failure: tc.failure})
			}
			s.restoreChangeWorkflow(snapshot)
			if snapshot.Workflow.Status != tc.want {
				t.Fatalf("read classified %s as %s, want %s", tc.name, snapshot.Workflow.Status, tc.want)
			}
			if tc.want == "running" && snapshot.Workflow.Stages[2].Status != "running" {
				t.Fatal("read fabricated interrupted stage evidence")
			}
		})
	}
}
