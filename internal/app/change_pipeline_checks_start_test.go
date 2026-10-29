package app

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
)

// failingWorkflowTests expects the wrong result, so the first run's checks fail.
var failingWorkflowTests = strings.ReplaceAll(workflowTests, "Helper() != 2", "Helper() != 99")

// reviseByHand replaces the proposal the way a conversation revision does:
// a new version, no checks, and the run marked outdated.
func reviseByHand(t *testing.T, s *Service, session *ChangeSession, testWant string) *ChangeSession {
	t.Helper()
	revised, err := s.SendChangeMessage(context.Background(), session.ID, ChangeMessageRequest{
		ChangeIdentity: changeIdentity(session),
		Message:        "Fix the expected value in the test.",
		Changes: []ChangeEditInput{
			{Path: "new.go", Content: "package main\nfunc Helper() int { return 2 }\n"},
			{Path: "new_test.go", Content: "package main\nimport \"testing\"\nfunc TestHelper(t *testing.T) { if Helper() != " + testWant + " { t.Fatal(\"wrong result\") } }\n"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if revised.Workflow.Status != "outdated" || len(revised.Checks) != 0 {
		t.Fatalf("revision kept the old run: %+v", revised.Workflow)
	}
	return revised
}

func TestChecksStartRunsChecksAndReviewOnARevisedProposal(t *testing.T) {
	var calls [3]atomic.Int32
	s, _, initial := workflowService(t, [3]func() string{
		func() string { calls[0].Add(1); return workflowImplementation },
		func() string { calls[1].Add(1); return failingWorkflowTests },
		func() string { calls[2].Add(1); return workflowApproval },
	})
	if _, err := s.StartChangeWorkflow(context.Background(), initial.ID, workflowRequest(initial)); err != nil {
		t.Fatal(err)
	}
	failed := finishedWorkflow(t, s, initial.ID)
	if failed.Workflow.Status != "failed" || calls[2].Load() != 0 {
		t.Fatalf("first run should fail its checks: %+v", failed.Workflow)
	}
	revised := reviseByHand(t, s, failed, "2")

	request := workflowRequest(revised)
	request.ResumeFrom = "checks"
	request.Models.Test = "code" // The test stage calls no model, so its model may change.
	if _, err := s.StartChangeWorkflow(context.Background(), revised.ID, request); err != nil {
		t.Fatal(err)
	}
	ready := finishedWorkflow(t, s, revised.ID)
	w := ready.Workflow
	if w.Status != "awaiting_human_review" || w.ResumeFrom != "checks" || !requiredChecksPassed(ready.Checks) {
		t.Fatalf("checks start did not reach human review: %+v, checks=%+v", w, ready.Checks)
	}
	if calls[0].Load() != 1 || calls[1].Load() != 1 || calls[2].Load() == 0 {
		t.Fatalf("checks start called the writing agents: calls=%d/%d/%d", calls[0].Load(), calls[1].Load(), calls[2].Load())
	}
	if ready.Hash != revised.Hash || !strings.Contains(ready.Changes[1].Content, "!= 2") {
		t.Fatal("checks start changed the revised proposal")
	}
	if w.Stages[0].Status != "completed" || w.Stages[0].Model.Profile != "bug" || w.Stages[0].StartedAt.After(w.StartedAt) {
		t.Fatalf("creation stage was not kept from the previous run: %+v", w.Stages[0])
	}
	test := w.Stages[1]
	if test.Status != "completed" || test.Model.Profile != "code" || test.StartedAt.Before(w.StartedAt) || test.FinishedAt.Before(test.StartedAt) || test.Usage != nil {
		t.Fatalf("test stage should time this run's checks without a model call: %+v", test)
	}
	if w.Review == nil || w.Review.ProposalHash != ready.Hash {
		t.Fatalf("reviewers did not review the revised proposal: %+v", w.Review)
	}
}

func TestChecksStartFailureNamesTheTestStageAndWritesNothing(t *testing.T) {
	var calls [3]atomic.Int32
	s, _, initial := workflowService(t, [3]func() string{
		func() string { calls[0].Add(1); return workflowImplementation },
		func() string { calls[1].Add(1); return workflowTests },
		func() string { calls[2].Add(1); return workflowApproval },
	})
	if _, err := s.StartChangeWorkflow(context.Background(), initial.ID, workflowRequest(initial)); err != nil {
		t.Fatal(err)
	}
	revised := reviseByHand(t, s, finishedWorkflow(t, s, initial.ID), "99")
	reviews := calls[2].Load()

	request := workflowRequest(revised)
	request.ResumeFrom = "checks"
	if _, err := s.StartChangeWorkflow(context.Background(), revised.ID, request); err != nil {
		t.Fatal(err)
	}
	failed := finishedWorkflow(t, s, revised.ID)
	w := failed.Workflow
	if w.Status != "failed" || !strings.Contains(w.Reason, "Tests or source checks failed") {
		t.Fatalf("failing checks should fail the run: %+v", w)
	}
	if w.Stages[1].Status != "failed" || w.Stages[1].Reason != w.Reason || w.Stages[2].Status != "skipped" {
		t.Fatalf("the test stage should carry the check failure: %+v", w.Stages)
	}
	if calls[0].Load() != 1 || calls[1].Load() != 1 || calls[2].Load() != reviews || failed.Hash != revised.Hash {
		t.Fatalf("a failed checks start changed the proposal or reviewed it: calls=%d/%d/%d", calls[0].Load(), calls[1].Load(), calls[2].Load())
	}
}

func TestChecksStartNeedsAProposalAndTheKeptCreationModel(t *testing.T) {
	s, _, initial := workflowService(t, [3]func() string{
		func() string { return workflowImplementation },
		func() string { return workflowTests },
		func() string { return workflowApproval },
	})
	request := workflowRequest(initial)
	request.ResumeFrom = "checks"
	if _, err := s.StartChangeWorkflow(context.Background(), initial.ID, request); err == nil || !strings.Contains(err.Error(), "cannot resume checks") {
		t.Fatalf("checks start without a proposal: %v", err)
	}
	if _, err := s.StartChangeWorkflow(context.Background(), initial.ID, workflowRequest(initial)); err != nil {
		t.Fatal(err)
	}
	ready := finishedWorkflow(t, s, initial.ID)
	request = workflowRequest(ready)
	request.ResumeFrom, request.Models.Create = "checks", "code"
	if _, err := s.StartChangeWorkflow(context.Background(), ready.ID, request); err == nil || !strings.Contains(err.Error(), "retry from create to change its model") {
		t.Fatalf("checks start replaced the kept creation model: %v", err)
	}
	request.ResumeFrom, request.Models.Create, request.IncludeComments = "checks", "bug", true
	if _, err := s.StartChangeWorkflow(context.Background(), ready.ID, request); err == nil || !strings.Contains(err.Error(), "send line comments with a run that writes") {
		t.Fatalf("checks start would mark line comments sent: %v", err)
	}
	request.ResumeFrom = "lint"
	if _, err := s.StartChangeWorkflow(context.Background(), ready.ID, request); err == nil || !strings.Contains(err.Error(), "create, test, checks or review") {
		t.Fatalf("unknown start accepted: %v", err)
	}
}
