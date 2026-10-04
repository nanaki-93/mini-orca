package app

import (
	"bytes"
	"context"
	"fmt"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s *Service) CheckChange(ctx context.Context, id string, request ChangeCheckRequest) (*ChangeSession, error) {
	session, root, err := s.loadChangeForAction(ctx, id, request.ChangeIdentity)
	if err != nil {
		return nil, err
	}
	if len(session.Changes) == 0 {
		return nil, fmt.Errorf("prepare a proposal before checking")
	}
	request.RunTests = request.RunTests || session.CheckOptions.RunTests
	request.RunLint = request.RunLint || session.CheckOptions.RunLint
	if request.RunTests || request.RunLint {
		if err := s.requireProjectExecutionTrust(session.ProjectRevision); err != nil {
			return nil, err
		}
	}
	checks := changeSourceChecks(session.Changes)
	if requiredChecksPassed(checks) && (request.RunTests || request.RunLint) {
		executed, err := s.checkChangeWorkspace(ctx, root, session, request.DraftCheckOptions)
		if err != nil {
			return nil, err
		}
		checks = append(checks, executed...)
	}
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	current, _, err := s.loadChangeForAction(ctx, id, request.ChangeIdentity)
	if err != nil {
		return nil, err
	}
	current.Checks, current.ReviewedHash, current.UpdatedAt = checks, "", time.Now().UTC()
	current.CheckOptions = request.DraftCheckOptions
	for _, check := range checks {
		if check.Name == "regression baseline" && check.State == CheckPassed {
			current.PinnedTests = changeTestEdits(current.Changes)
		}
	}
	if err := writeChangeSession(root, current); err != nil {
		return nil, err
	}
	s.changeAuthority[current.ProjectID+"/"+id] = "checks:" + current.Hash
	return current, nil
}

func changeSourceChecks(edits []ChangeEdit) []DraftCheck {
	checks := []DraftCheck{}
	for _, edit := range edits {
		if filepath.Ext(edit.Path) != ".go" {
			checks = append(checks, DraftCheck{Name: "text: " + edit.Path, Required: true, State: CheckPassed})
			continue
		}
		parsed := parseGoDraft(edit.Path, edit.Content)
		parsed.Name += ": " + edit.Path
		checks = append(checks, parsed)
		formatted, err := format.Source([]byte(edit.Content))
		check := DraftCheck{Name: "format: " + edit.Path, Required: true, State: CheckPassed}
		if err != nil || !bytes.Equal(formatted, []byte(edit.Content)) {
			check.State, check.Output = CheckFailed, "Candidate must parse and be gofmt-formatted."
		}
		checks = append(checks, check)
	}
	return checks
}

func (s *Service) checkChangeWorkspace(ctx context.Context, root string, session *ChangeSession, options DraftCheckOptions) ([]DraftCheck, error) {
	workspace, err := os.MkdirTemp("", "mini-orca-change-check-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workspace)
	if err := copyCheckWorkspace(ctx, root, workspace); err != nil {
		return nil, err
	}
	fingerprint, err := fingerprintCheckWorkspace(ctx, workspace)
	if err != nil {
		return nil, err
	}
	if fingerprint != session.WorkspaceHash {
		return nil, project.ErrRevisionConflict
	}
	checks, err := s.changeRegressionBaseline(ctx, workspace, session, options.RunTests)
	if err != nil {
		return nil, err
	}
	if err := stageChangeEdits(workspace, session.Changes); err != nil {
		return nil, err
	}
	if options.RunLint {
		checks = append(checks, s.runCheck(ctx, workspace, session.ProjectRevision, "lint", true, []string{"go", "vet", "./..."}, false, false))
	}
	if options.RunTests {
		checks = append(checks, s.runCheck(ctx, workspace, session.ProjectRevision, "tests", true, []string{"go", "test", "./..."}, false, true))
	}
	return checks, ctx.Err()
}

func (s *Service) changeRegressionBaseline(ctx context.Context, workspace string, session *ChangeSession, runTests bool) ([]DraftCheck, error) {
	checks := []DraftCheck{}
	newTests := newChangeTestEdits(session)
	if !runTests || len(newTests) == 0 {
		return checks, nil
	}
	checks = append(checks, s.runCheck(ctx, workspace, session.ProjectRevision, "existing test baseline", true, []string{"go", "test", "./..."}, false, true))
	if err := stageChangeEdits(workspace, newTests); err != nil {
		return nil, err
	}
	checks = append(checks, s.runTaskTestCheck(ctx, workspace, session.ProjectRevision, "regression baseline", []string{"go", "test", "./..."}, false))
	return checks, nil
}

func stageChangeEdits(workspace string, edits []ChangeEdit) error {
	for _, edit := range edits {
		path, err := project.ResolveWritePath(workspace, edit.Path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(edit.Content), 0600); err != nil {
			return err
		}
	}
	return nil
}

func changeTestEdits(edits []ChangeEdit) []ChangeEdit {
	result := []ChangeEdit{}
	for _, edit := range edits {
		if strings.HasSuffix(edit.Path, "_test.go") {
			result = append(result, edit)
		}
	}
	return result
}

func newChangeTestEdits(session *ChangeSession) []ChangeEdit {
	result := []ChangeEdit{}
	for _, edit := range changeTestEdits(session.Changes) {
		if target := changeTarget(session, edit.Path); target != nil && !target.Exists {
			result = append(result, edit)
		}
	}
	return result
}

func (s *Service) ReviewChange(ctx context.Context, id string, identity ChangeIdentity) (*ChangeSession, error) {
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	session, root, err := s.loadChangeForAction(ctx, id, identity)
	if err != nil {
		return nil, err
	}
	if len(session.Checks) == 0 || !requiredChecksPassed(session.Checks) || s.changeAuthority[session.ProjectID+"/"+id] != "checks:"+session.Hash {
		return nil, fmt.Errorf("complete current proposal checks before review")
	}
	session.ReviewedHash = session.Hash
	if err := writeChangeSession(root, session); err != nil {
		return nil, err
	}
	s.changeAuthority[session.ProjectID+"/"+id] = "review:" + session.Hash
	return session, nil
}

func (s *Service) RepairChangeMessage(ctx context.Context, id string, request ChangeMessageRequest) (*ChangeSession, error) {
	if err := s.RequireRemoteConfirmation(config.FunctionModelScope, request.ConfirmRemoteProvider); err != nil {
		return nil, err
	}
	session, root, err := s.reserveChangeRepair(ctx, id, request.ChangeIdentity)
	if err != nil {
		return nil, err
	}
	request.Message = "Repair the current proposal without changing scope, acceptance criteria or pinned tests. Check evidence:\n" + changeCheckEvidence(session.Checks)
	return s.generateChangeMessage(ctx, root, session, request)
}

func (s *Service) reserveChangeRepair(ctx context.Context, id string, identity ChangeIdentity) (*ChangeSession, string, error) {
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	session, root, err := s.loadChangeForAction(ctx, id, identity)
	if err != nil {
		return nil, root, err
	}
	if session.RepairAttempts >= 3 || len(session.Checks) == 0 || requiredChecksPassed(session.Checks) || s.changeAuthority[session.ProjectID+"/"+id] != "checks:"+session.Hash {
		return nil, root, fmt.Errorf("repair requires current failed checks and fewer than three attempts")
	}
	session.RepairAttempts++
	if err := writeChangeSession(root, session); err != nil {
		return nil, root, err
	}
	return session, root, nil
}

func changeCheckEvidence(checks []DraftCheck) string {
	var text strings.Builder
	for _, check := range checks {
		if text.Len() >= 4096 {
			break
		}
		text.WriteString(check.Name + ": " + check.State + "\n" + sanitizeCheckOutput(check.Output, false, "", "") + "\n")
	}
	result := text.String()
	if len(result) > 4096 {
		result = result[:4096]
	}
	return result
}
