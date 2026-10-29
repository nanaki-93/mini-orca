package app

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
)

func TestChangeWorkflowNamesTheFileWrittenOutsideTheScope(t *testing.T) {
	var later atomic.Int32
	outside := `{"explanation":"Move the helpers.","changes":[{"path":"new.go","content":"package main\n\nfunc Helper() int { return 2 }\n"},{"path":"util.go","content":"package main\n"}]}`
	s, _, initial := workflowService(t, [3]func() string{
		func() string { return outside },
		func() string { later.Add(1); return workflowTests },
		func() string { later.Add(1); return workflowApproval },
	})
	if _, err := s.StartChangeWorkflow(context.Background(), initial.ID, workflowRequest(initial)); err != nil {
		t.Fatal(err)
	}
	result := finishedWorkflow(t, s, initial.ID)
	want := `(change node create: proposal wrote "util.go", outside the task's files (a new file must be in the task's scope))`
	if result.Workflow.Status != "failed" || !strings.HasSuffix(result.Workflow.Reason, want) {
		t.Fatalf("workflow reason does not name the rejected file: %+v", result.Workflow)
	}
	if result.Workflow.Stages[0].Reason != result.Workflow.Reason || len(result.Changes) != 0 || later.Load() != 0 {
		t.Fatalf("rejected proposal reached later stages: %+v, changes=%d, later calls=%d", result.Workflow.Stages, len(result.Changes), later.Load())
	}
}
