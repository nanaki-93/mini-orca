package app

import (
	"context"
	"fmt"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func (s *Service) CancelChangeWorkflow(ctx context.Context, id string, request ChangeWorkflowControl) (*ChangeSession, error) {
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	index, err := s.manager.Index()
	if err != nil {
		return nil, err
	}
	if request.ProjectID != index.ProjectID || request.ProjectRevision != index.ProjectRevision {
		return nil, project.ErrRevisionConflict
	}
	root := s.manager.Root()
	session, err := readChangeSession(root, id)
	if err != nil {
		return nil, err
	}
	if session.ProjectID != request.ProjectID || session.Workflow == nil || session.Workflow.ID != request.WorkflowID {
		return nil, project.ErrRevisionConflict
	}
	if !changeWorkflowActive(session.Workflow) {
		return session, nil
	}
	s.changeWorkflow.mu.Lock()
	job := s.changeWorkflow.job
	active := job != nil && !job.finished && job.root == root && job.id == request.WorkflowID
	if active {
		job.cancel()
	}
	s.changeWorkflow.mu.Unlock()
	if !active {
		return nil, fmt.Errorf("restore this interrupted workflow before running again")
	}
	session.Workflow.Status = "canceling"
	session.Workflow.UpdatedAt = time.Now().UTC()
	session.UpdatedAt = session.Workflow.UpdatedAt
	return session, writeChangeSession(root, session)
}

// CancelChangeWorkflows revokes dispatch on project replacement, reindex and
// daemon shutdown. The worker owns cleanup and saves a terminal result.
func (s *Service) CancelChangeWorkflows() {
	s.changeWorkflow.mu.Lock()
	defer s.changeWorkflow.mu.Unlock()
	if job := s.changeWorkflow.job; job != nil && !job.finished {
		job.cancel()
	}
}

// ShutdownChangeWorkflows waits for process/workspace cleanup after revoking
// the worker. Lifecycle mutations use cancellation alone to avoid lock waits.
func (s *Service) ShutdownChangeWorkflows(ctx context.Context) error {
	s.changeWorkflow.mu.Lock()
	job := s.changeWorkflow.job
	if job != nil {
		job.cancel()
	}
	s.changeWorkflow.mu.Unlock()
	if job == nil {
		return nil
	}
	select {
	case <-job.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) restoreChangeWorkflow(session *ChangeSession) {
	w := session.Workflow
	if w == nil {
		return
	}
	s.changeWorkflow.mu.Lock()
	job := s.changeWorkflow.job
	active := job != nil && job.id == w.ID && job.root == s.manager.Root() && !job.finished
	failure := ""
	if job != nil && job.id == w.ID {
		failure = job.failure
	}
	s.changeWorkflow.mu.Unlock()
	if failure != "" {
		w.Status, w.Reason = "failed", failure
	} else if changeWorkflowActive(w) && !active {
		w.Status, w.Reason = "interrupted", "The workflow stopped before completion. Restore the conversation to run again."
	}
	if session.Freshness == "stale" {
		w.Status, w.Reason = "stale", "Project source or instructions changed. Start a fresh conversation."
	}
	if w.Status == "interrupted" || w.Status == "failed" || w.Status == "stale" {
		stopChangeWorkflowStages(w)
	}
}
