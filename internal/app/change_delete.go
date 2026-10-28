package app

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

// ErrChangeInUse reports a task that cannot be deleted yet.
var ErrChangeInUse = errors.New("task is in use")

// DeleteChange removes a task's history from the checkout. The request names
// the task version the user saw (its revision and hash); a stale task can be
// deleted because the project revision is not part of that identity. A task
// with a running workflow, or whose latest Apply can still be undone, is kept:
// Undo restores the task from its recovery journal.
func (s *Service) DeleteChange(ctx context.Context, id string, request ChangeIdentity) error {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	root := s.manager.Root()
	session, err := loadChangeSession(root, id)
	if err != nil {
		return err
	}
	if request.ProjectID != session.ProjectID || request.Revision != session.Revision || request.Hash != session.Hash {
		return project.ErrRevisionConflict
	}
	if isChangePipelineActive(session.Workflow) {
		return fmt.Errorf("%w: stop its agents before deleting it", ErrChangeInUse)
	}
	journal, err := readChangeJournal(root)
	if err != nil {
		return err
	}
	if journal != nil && journal.Session.ID == id && journal.State != "undone" {
		return fmt.Errorf("%w: its latest Apply can still be undone; undo it or apply another task first", ErrChangeInUse)
	}
	path, err := changeMetadataPath(root, id+".json")
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	delete(s.changeAuthority, session.ProjectID+"/"+id)
	s.publishEvent(Event{Kind: "workflow", SessionID: id, Title: session.Title, Status: "deleted"})
	return nil
}
