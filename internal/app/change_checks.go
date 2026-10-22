package app

import (
	"bytes"
	"context"
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/archgate"
	"github.com/nanaki-93/mini-orca/v2/internal/app/changes"
	"github.com/nanaki-93/mini-orca/v2/internal/app/sandbox"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func (s *Service) CheckChange(ctx context.Context, id string, request ChangeCheckRequest) (*ChangeSession, error) {
	s.changesMu.Lock()
	session, root, err := s.loadChangeForAction(ctx, id, request.ChangeIdentity)
	s.changesMu.Unlock()
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
	checks := changeSourceChecks(root, session.Changes)
	if requiredChecksPassed(checks) && (request.RunTests || request.RunLint) {
		executed, err := s.checkChangeWorkspace(ctx, root, session, request.CheckOptions)
		if err != nil {
			return nil, err
		}
		checks = append(checks, executed...)
	}
	return s.publishChangeChecks(ctx, id, request, checks)
}

func (s *Service) publishChangeChecks(ctx context.Context, id string, request ChangeCheckRequest, checks []CheckResult) (*ChangeSession, error) {
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	current, root, err := s.loadChangeForAction(ctx, id, request.ChangeIdentity)
	if err != nil {
		return nil, err
	}
	current.Checks, current.ReviewedHash, current.UpdatedAt = checks, "", time.Now().UTC()
	current.CheckOptions = request.CheckOptions
	expandTargetsFromFailedChecks(root, current)
	if current.Workflow != nil {
		reconcileWorkflowAfterChecks(current, checks, ctx.Value(changeWorkflowContextKey{}) != nil)
	}
	pinChangeRegression(current, checks)
	if err := storeChangeSession(root, current); err != nil {
		return nil, err
	}
	s.changeAuthority[current.ProjectID+"/"+id] = "checks:" + current.Hash
	return current, nil
}

func reconcileWorkflowAfterChecks(current *ChangeSession, checks []CheckResult, inWorkflowJob bool) {
	wf := current.Workflow
	if !inWorkflowJob && !reconcileManualCheckStages(wf, current.Hash, checks) {
		return
	}
	if canHumanReviewProposal(current) {
		markLeadingWorkflowStagesCompleted(wf)
		if len(wf.Stages) > 3 {
			wf.Stages[3].Status = "waiting"
		}
		return
	}
	if len(wf.Stages) > 3 {
		wf.Stages[3].Status = "blocked"
	}
}

func reconcileManualCheckStages(wf *ChangeWorkflow, hash string, checks []CheckResult) bool {
	if !requiredChecksPassed(checks) {
		wf.Status, wf.Reason = workflowFailure(errWorkflowChecksFailed, checks)
		if len(wf.Stages) > 0 && (wf.Stages[0].Status == "failed" || wf.Stages[0].Status == "interrupted") {
			wf.Stages[0].Status, wf.Stages[0].Reason = "completed", ""
		}
		if len(wf.Stages) > 1 {
			wf.Stages[1].Status, wf.Stages[1].Reason = "failed", wf.Reason
		}
		if len(wf.Stages) > 3 {
			wf.Stages[3].Status = "blocked"
		}
		return false
	}
	if len(wf.Stages) > 0 {
		wf.Stages[0].Status, wf.Stages[0].Reason = "completed", ""
	}
	if len(wf.Stages) > 1 {
		wf.Stages[1].Status, wf.Stages[1].Reason = "completed", ""
	}
	if wf.Review != nil && wf.Review.ProposalHash == hash && wf.Review.Verdict == "approve" {
		wf.Status, wf.Reason = "awaiting_human_review", ""
	}
	return true
}

func markLeadingWorkflowStagesCompleted(wf *ChangeWorkflow) {
	for i := 0; i < 3 && i < len(wf.Stages); i++ {
		wf.Stages[i].Status, wf.Stages[i].Reason = "completed", ""
	}
}

func pinChangeRegression(session *ChangeSession, checks []CheckResult) {
	if !requiredChecksPassed(checks) {
		return
	}
	for _, check := range checks {
		if check.Name == "regression baseline" && check.State == CheckPassed {
			session.PinnedTests = changeTestEdits(session.Changes)
		}
	}
}

func changeSourceChecks(root string, edits []ChangeEdit) []CheckResult {
	checks := []CheckResult{}
	for _, edit := range edits {
		if filepath.Ext(edit.Path) != ".go" {
			checks = append(checks, CheckResult{Name: "text: " + edit.Path, Required: true, State: CheckPassed})
			continue
		}
		parsed := parseGoSource(edit.Path, edit.Content)
		parsed.Name += ": " + edit.Path
		checks = append(checks, parsed)
		formatted, err := format.Source([]byte(edit.Content))
		check := CheckResult{Name: "format: " + edit.Path, Required: true, State: CheckPassed}
		if err != nil || !bytes.Equal(formatted, []byte(edit.Content)) {
			check.State, check.Output = CheckFailed, "Candidate must parse and be gofmt-formatted."
		}
		checks = append(checks, check)
	}
	checks = append(checks, archgate.ChangeArchitectureAndComplexityChecks(root, edits)...)
	if len(edits) > 0 {
		checks = append(checks, archgate.CheckChangeSecretAndHygiene(edits))
	}
	return checks
}

func (s *Service) checkChangeWorkspace(ctx context.Context, root string, session *ChangeSession, options CheckOptions) ([]CheckResult, error) {
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
	if !workspaceHasGo(workspace, session.Changes) {
		return s.checkNonGoWorkspace(ctx, workspace, session, options)
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
		if archgate.HasOptionalGitleaks(workspace) {
			checks = append(checks, s.runCheck(ctx, workspace, session.ProjectRevision, "gitleaks", true, []string{"gitleaks", "detect", "--no-git", "--source", ".", "--redact"}, false, true))
		}
	}
	if options.RunTests {
		checks = append(checks, s.runCheck(ctx, workspace, session.ProjectRevision, "tests", true, []string{"go", "test", "./..."}, false, true))
	}
	return checks, ctx.Err()
}

func (s *Service) checkNonGoWorkspace(ctx context.Context, workspace string, session *ChangeSession, options CheckOptions) ([]CheckResult, error) {
	if err := stageChangeEdits(workspace, session.Changes); err != nil {
		return nil, err
	}
	runner, checkTarget, testTarget := detectExecutableTaskRunner(workspace)
	checks := []CheckResult{}
	if options.RunLint {
		if runner != "" && checkTarget != "" {
			checks = append(checks, s.runCheck(ctx, workspace, session.ProjectRevision, "lint", true, buildTaskRunnerArgv(runner, checkTarget), false, true))
		} else {
			checks = append(checks, CheckResult{Name: "lint", Required: false, State: CheckSkipped, Output: "Go vet skipped for non-Go workspace; source checks passed."})
		}
	}
	if options.RunTests {
		if runner != "" && testTarget != "" {
			testCheck := s.runCheck(ctx, workspace, session.ProjectRevision, "tests", true, buildTaskRunnerArgv(runner, testTarget), false, true)
			if len(newChangeTestEdits(session)) > 0 {
				checks = append(checks, CheckResult{Name: "regression baseline", Required: true, State: testCheck.State, Output: testCheck.Output})
			}
			checks = append(checks, testCheck)
		} else {
			msg := "No executable test runner is configured or available for this workspace."
			if len(newChangeTestEdits(session)) > 0 {
				checks = append(checks, CheckResult{Name: "regression baseline", Required: true, State: CheckUnavailable, Output: msg})
			}
			checks = append(checks, CheckResult{Name: "tests", Required: true, State: CheckUnavailable, Output: msg})
		}
	}
	return checks, ctx.Err()
}

func buildTaskRunnerArgv(runner, target string) []string {
	if runner == "npm" && target != "test" {
		return []string{"npm", "run", target}
	}
	return []string{runner, target}
}

func detectExecutableTaskRunner(workspace string) (string, string, string) {
	runner, checkTarget, testTarget := detectDeclaredTaskRunner(workspace)
	if runner == "" {
		return "", "", ""
	}
	if _, err := exec.LookPath(runner); err != nil {
		return "", "", ""
	}
	return runner, checkTarget, testTarget
}

func detectDeclaredTaskRunner(workspace string) (string, string, string) {
	for _, entry := range []struct {
		files  []string
		runner string
	}{
		{files: []string{"justfile", "Justfile"}, runner: "just"},
		{files: []string{"Taskfile.yml", "Taskfile.yaml"}, runner: "task"},
		{files: []string{"Makefile"}, runner: "make"},
	} {
		for _, name := range entry.files {
			data, err := os.ReadFile(filepath.Join(workspace, name))
			if err != nil {
				continue
			}
			checkTarget, testTarget := parseTaskRunnerTargets(string(data))
			if checkTarget != "" || testTarget != "" {
				return entry.runner, checkTarget, testTarget
			}
		}
	}
	return detectEcosystemTaskRunner(workspace)
}

func detectEcosystemTaskRunner(workspace string) (string, string, string) {
	if data, err := os.ReadFile(filepath.Join(workspace, "package.json")); err == nil {
		if checkTarget, testTarget := parsePackageJSONTargets(string(data)); checkTarget != "" || testTarget != "" {
			return "npm", checkTarget, testTarget
		}
	}
	if _, err := os.Stat(filepath.Join(workspace, "Cargo.toml")); err == nil {
		return "cargo", "check", "test"
	}
	if data, err := os.ReadFile(filepath.Join(workspace, "pyproject.toml")); err == nil {
		text := string(data)
		if strings.Contains(text, "pytest") {
			return "pytest", "", "-q"
		}
	}
	if _, err := os.Stat(filepath.Join(workspace, "pom.xml")); err == nil {
		return "mvn", "test-compile", "test"
	}
	if _, err := os.Stat(filepath.Join(workspace, "build.gradle")); err == nil {
		return "gradle", "check", "test"
	}
	if _, err := os.Stat(filepath.Join(workspace, "build.gradle.kts")); err == nil {
		return "gradle", "check", "test"
	}
	return "", "", ""
}

func parsePackageJSONTargets(content string) (string, string) {
	if !strings.Contains(content, `"scripts"`) {
		return "", ""
	}
	var checkTarget, testTarget string
	if strings.Contains(content, `"check"`) {
		checkTarget = "check"
	} else if strings.Contains(content, `"lint"`) {
		checkTarget = "lint"
	}
	if strings.Contains(content, `"test"`) && !strings.Contains(content, "no test specified") {
		testTarget = "test"
	}
	return checkTarget, testTarget
}

func parseTaskRunnerTargets(content string) (string, string) {
	var checkTarget, testTarget string
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "#") || !strings.Contains(line, ":") {
			continue
		}
		head := strings.TrimSpace(strings.SplitN(line, ":", 2)[0])
		name := strings.Fields(head)
		if len(name) == 0 {
			continue
		}
		switch name[0] {
		case "check":
			checkTarget = "check"
		case "lint":
			if checkTarget == "" {
				checkTarget = "lint"
			}
		case "test":
			testTarget = "test"
		}
	}
	return checkTarget, testTarget
}

func workspaceHasGo(workspace string, edits []ChangeEdit) bool {
	if changeHasGo(edits) {
		return true
	}
	info, err := os.Stat(filepath.Join(workspace, "go.mod"))
	return err == nil && !info.IsDir()
}

func (s *Service) changeRegressionBaseline(ctx context.Context, workspace string, session *ChangeSession, runTests bool) ([]CheckResult, error) {
	checks := []CheckResult{}
	newTests := newChangeTestEdits(session)
	if !runTests || len(newTests) == 0 {
		return checks, nil
	}
	checks = append(checks, s.runCheck(ctx, workspace, session.ProjectRevision, "existing test baseline", true, []string{"go", "test", "./..."}, false, true))
	if err := stageChangeEdits(workspace, newTests); err != nil {
		return nil, err
	}
	baseline := s.runTaskTestCheck(ctx, workspace, session.ProjectRevision, "regression baseline", []string{"go", "test", "./..."}, false)
	if baseline.State != CheckCanceled {
		baseline.State = CheckPassed
	}
	checks = append(checks, baseline)
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

func isTestFilePath(path string) bool {
	lower := strings.ToLower(filepath.ToSlash(path))
	base := filepath.Base(lower)
	if strings.HasSuffix(lower, "_test.go") || strings.HasSuffix(lower, "_test.py") || (strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py")) {
		return true
	}
	for _, suffix := range []string{
		".test.ts", ".spec.ts", ".test.tsx", ".spec.tsx",
		".test.js", ".spec.js", ".test.jsx", ".spec.jsx",
		".test.mjs", ".spec.mjs", ".test.cjs", ".spec.cjs",
		"test.kt", "tests.kt", "spec.kt", "test.java", "tests.java",
		"_test.rs", "_test.rb", "_spec.rb", "test.php", "test.cs", "tests.cs",
		"test.swift", "tests.swift", "_test.c", "_test.cpp", "_test.cc",
	} {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}
	parts := strings.Split(lower, "/")
	for _, part := range parts[:len(parts)-1] {
		if part == "test" || part == "tests" || part == "__tests__" || part == "spec" || part == "specs" {
			return true
		}
	}
	return false
}

func changeTestEdits(edits []ChangeEdit) []ChangeEdit {
	result := []ChangeEdit{}
	for _, edit := range edits {
		if isTestFilePath(edit.Path) {
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
	if !canHumanReviewProposal(session) || len(session.Checks) == 0 || !requiredChecksPassed(session.Checks) {
		return nil, fmt.Errorf("complete current proposal checks before review")
	}
	session.ReviewedHash = session.Hash
	if session.Workflow != nil {
		session.Workflow.Stages[3].Status = "completed"
	}
	if err := storeChangeSession(root, session); err != nil {
		return nil, err
	}
	s.changeAuthority[session.ProjectID+"/"+id] = "review:" + session.Hash
	return session, nil
}

func (s *Service) RepairChangeMessage(ctx context.Context, id string, request ChangeMessageRequest) (*ChangeSession, error) {
	userGuidance := strings.TrimSpace(request.Message)
	runtime, err := s.changeMessageRuntime(ctx, request)
	if err != nil {
		return nil, err
	}
	session, root, err := s.reserveChangeRepair(ctx, id, request.ChangeIdentity)
	if err != nil {
		return nil, err
	}
	prompt := "Repair the current proposal (including any test compile or assertion errors) without changing scope or acceptance criteria. Check evidence:\n" + changeCheckEvidence(session.Checks)
	if userGuidance != "" && userGuidance != "Repair the current proposal to satisfy failed checks." {
		prompt += "\n\nUser repair guidance:\n" + userGuidance
	}
	request.Message = prompt
	return s.generateChangeWithRuntime(ctx, root, session, request, runtime)
}

func (s *Service) reserveChangeRepair(ctx context.Context, id string, identity ChangeIdentity) (*ChangeSession, string, error) {
	s.changesMu.Lock()
	defer s.changesMu.Unlock()
	session, root, err := s.loadChangeForAction(ctx, id, identity)
	if err != nil {
		return nil, root, err
	}
	hasFailedChecks := len(session.Checks) > 0 && !requiredChecksPassed(session.Checks) && session.ReviewedHash == ""
	if session.RepairAttempts >= 3 || len(session.Checks) == 0 || requiredChecksPassed(session.Checks) || (s.changeAuthority[session.ProjectID+"/"+id] != "checks:"+session.Hash && !hasFailedChecks) {
		return nil, root, fmt.Errorf("repair requires current failed checks and fewer than three attempts")
	}
	session.RepairAttempts++
	expandTargetsFromFailedChecks(root, session)
	if session.Workflow != nil {
		if probeSummary, lessons := probeWorkflowFriction(session.Workflow, session.Checks); probeSummary != "" || len(lessons) > 0 {
			if probeSummary != "" {
				session.Workflow.ProbeSummary = probeSummary
			}
			session.Workflow.HarnessLessons = mergeHarnessLessons(session.Workflow.HarnessLessons, lessons)
		}
	}
	if err := storeChangeSession(root, session); err != nil {
		return nil, root, err
	}
	return session, root, nil
}

func expandTargetsFromFailedChecks(root string, session *ChangeSession) {
	if session == nil || len(session.Checks) == 0 || requiredChecksPassed(session.Checks) {
		return
	}
	existingPaths := collectSessionTargetPaths(session)
	failedOutputs := collectFailedCheckOutputs(session.Checks)
	for _, path := range changes.ResolveExpandedTargetPaths(root, existingPaths, failedOutputs, isTestFilePath) {
		captureExpandedSessionTarget(root, session, path)
	}
}

func collectSessionTargetPaths(session *ChangeSession) []string {
	paths := make([]string, 0, len(session.Targets))
	for _, t := range session.Targets {
		paths = append(paths, t.Path)
	}
	return paths
}

func collectFailedCheckOutputs(checks []CheckResult) []string {
	var out []string
	for _, check := range checks {
		if check.State != CheckPassed && check.State != CheckSkipped && strings.TrimSpace(check.Output) != "" {
			out = append(out, check.Output)
		}
	}
	return out
}

func captureExpandedSessionTarget(root string, session *ChangeSession, path string) {
	if changeTarget(session, path) != nil {
		return
	}
	target, err := captureChangeTarget(root, path)
	if err != nil {
		return
	}
	instructions, err := project.ResolveInstructions(root, path)
	if err != nil {
		return
	}
	if session.Instructions == nil {
		session.Instructions = map[string]string{}
	}
	session.Targets = append(session.Targets, target)
	session.Instructions[path] = instructions.Fingerprint
}

func changeCheckEvidence(checks []CheckResult) string {
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

type workspaceIdentity struct {
	fingerprint string
}

func workspaceFingerprint(ctx context.Context, root string) (string, error) {
	return sandbox.WorkspaceFingerprint(ctx, root)
}

func fingerprintCheckWorkspace(ctx context.Context, root string) (string, error) {
	return sandbox.FingerprintWorkspace(ctx, root)
}

func (fixture workspaceIdentity) verifyCurrent(ctx context.Context, root string) error {
	return sandbox.VerifyWorkspaceFingerprint(ctx, root, fixture.fingerprint)
}
