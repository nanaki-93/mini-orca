package app

import (
	"context"
	"testing"
	"time"
)

func TestWorkflowChargesItsStagesAndTaskUsageAddsUp(t *testing.T) {
	s, _, initial := workflowService(t, [3]func() string{func() string { return workflowImplementation }, func() string { return workflowTests }, func() string { return workflowApproval }})
	if _, err := s.StartChangeWorkflow(t.Context(), initial.ID, workflowRequest(initial)); err != nil {
		t.Fatal(err)
	}
	finishedWorkflow(t, s, initial.ID)
	read, err := s.ChangeSession(t.Context(), initial.ID)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	for _, stage := range read.Workflow.Stages[:3] {
		if stage.Usage == nil || stage.Usage.ModelCalls == 0 || stage.Cost == nil {
			t.Fatalf("stage %s has no usage or cost: %+v", stage.Name, stage)
		}
		calls += stage.Usage.ModelCalls
	}
	report, err := s.ChangeUsage(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Tasks) != 1 || report.Tasks[0].SessionID != initial.ID || report.Tasks[0].Title != initial.Title || report.Tasks[0].Usage.ModelCalls != calls || report.Tasks[0].Runs == 0 {
		t.Fatalf("task usage disagrees with its stages (%d calls): %+v", calls, report.Tasks)
	}
	if report.Total.Usage.ModelCalls < calls || report.Tasks[0].Cost == nil || report.Tasks[0].Cost.Status != "unknown" {
		t.Fatalf("totals or cost: %+v", report)
	}
}

func TestManualRevisionsCountTowardTheirTask(t *testing.T) {
	service, session, _ := commentedProposal(t)
	report, err := service.ChangeUsage(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Tasks) != 1 || report.Tasks[0].SessionID != session.ID || report.Tasks[0].Usage.ModelCalls != 1 {
		t.Fatalf("revision usage not attributed: %+v", report)
	}
}

func TestWorkflowEventsNameTheTaskStageAndUsage(t *testing.T) {
	s, _, initial := workflowService(t, [3]func() string{func() string { return workflowImplementation }, func() string { return workflowTests }, func() string { return workflowApproval }})
	events, unsubscribe := s.SubscribeEvents()
	defer unsubscribe()
	if _, err := s.StartChangeWorkflow(t.Context(), initial.ID, workflowRequest(initial)); err != nil {
		t.Fatal(err)
	}
	stages := map[string]bool{}
	deadline := time.After(2 * time.Minute)
	for {
		select {
		case event := <-events:
			if event.Kind != "workflow" {
				continue
			}
			if event.SessionID != initial.ID || event.WorkflowID == "" || event.Title != initial.Title {
				t.Fatalf("workflow event lacks its task: %+v", event)
			}
			stages[event.Stage] = true
			if event.Stage == "" {
				if event.Status != "awaiting_human_review" || event.Usage == nil || event.Usage.ModelCalls == 0 || event.Cost == nil {
					t.Fatalf("final event: %+v", event)
				}
				for _, stage := range []string{"create", "test", "review"} {
					if !stages[stage] {
						t.Fatalf("no %s event among %v", stage, stages)
					}
				}
				return
			}
		case <-deadline:
			t.Fatal("workflow did not finish")
		}
	}
}
