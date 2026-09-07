package app

import (
	"bytes"
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

const (
	CheckNotRun         = "not-run"
	CheckPassed         = "passed"
	CheckFailed         = "failed"
	CheckSkipped        = "skipped"
	CheckCanceled       = "canceled"
	CheckUnavailable    = "unavailable"
	checkCopyBufferSize = 32 * 1024
)

type checkResourceLimits struct {
	maxOutputBytes    int
	maxWorkspaceFiles int
	maxWorkspaceBytes int64
	pipeDrainWait     time.Duration
}

// checkLimits is the single resource budget for copied check workspaces and
// their command diagnostics. A copy that exceeds this budget is not evidence
// for a successful check.
var checkLimits = checkResourceLimits{
	maxOutputBytes:    8 * 1024,
	maxWorkspaceFiles: 4 * 1024,
	maxWorkspaceBytes: 64 * 1024 * 1024,
	pipeDrainWait:     250 * time.Millisecond,
}

var (
	checkAuthorization = regexp.MustCompile(`(?i)\b(?:proxy-)?authorization\b\s*[:=]\s*[^\r\n]*`)
	checkSecret        = regexp.MustCompile(`(?i)\b(?:api[_-]?key|access[_-]?token|auth[_-]?token|password|secret|credential)\b\s*[:=]\s*[^\s,;]+`)
)

// DraftCheck is one parser, formatter, lint, or test result. Command is a
// display-only argv preview; it is never executed through a shell.
type DraftCheck struct {
	Name     string   `json:"name"`
	Required bool     `json:"required"`
	State    string   `json:"state"`
	Command  []string `json:"command,omitempty"`
	Output   string   `json:"output,omitempty"`
	ExitCode int      `json:"exit_code,omitempty"`
}

// DraftCheckReport determines whether a valid draft composition can be
// applied. It contains no source content and all commands run in a copy.
type DraftCheckReport struct {
	DraftID         string       `json:"draft_id,omitempty"`
	DraftRevision   int64        `json:"draft_revision,omitempty"`
	DraftHash       string       `json:"draft_hash,omitempty"`
	CompositionHash string       `json:"candidate_hash,omitempty"`
	ProjectID       string       `json:"project_id,omitempty"`
	ProjectRevision string       `json:"project_revision,omitempty"`
	BaseFileHash    string       `json:"base_file_hash,omitempty"`
	TargetPath      string       `json:"target_path"`
	Applicable      bool         `json:"applicable"`
	Checks          []DraftCheck `json:"checks"`
}

// DraftCheckOptions keeps expensive checks explicit. Parsing and formatting
// are required where supported; lint and tests run only when requested.
type DraftCheckOptions struct {
	RunLint  bool `json:"run_lint,omitempty"`
	RunTests bool `json:"run_tests,omitempty"`
}

type draftCheckInput struct {
	file            project.IndexFile
	source          string
	projectRevision string
	taskTest        *project.GoTestCandidateSpec
}

// runDraftChecks executes only the exact, already-validated draft in an
// temporary project copy. It is intentionally not a general source-check API.
func (s *Service) runDraftChecks(ctx context.Context, input draftCheckInput, options DraftCheckOptions) (DraftCheckReport, error) {
	file := input.file
	revision := input.projectRevision
	if revision == "" {
		analysis, err := s.manager.Analysis()
		if err != nil {
			return DraftCheckReport{}, err
		}
		revision = analysis.ProjectRevision
	}
	workspace, err := os.MkdirTemp("", "mini-orca-check-")
	if err != nil {
		return DraftCheckReport{}, fmt.Errorf("create draft check workspace: %w", err)
	}
	defer os.RemoveAll(workspace)
	if err := copyCheckWorkspace(ctx, s.manager.Root(), workspace); err != nil {
		return DraftCheckReport{}, err
	}
	var taskChecks []DraftCheck
	var taskCommand []string
	if input.taskTest != nil {
		testPath, err := taskTestPath(workspace, file.Path)
		if err != nil {
			return DraftCheckReport{}, err
		}
		if err := os.WriteFile(testPath, []byte(input.taskTest.Content), 0600); err != nil {
			return DraftCheckReport{}, fmt.Errorf("write task test workspace file: %w", err)
		}
		taskCommand = []string{"go", "test", "./...", "-run", "^" + regexp.QuoteMeta(input.taskTest.Name) + "$"}
		taskChecks = append(taskChecks, s.runTaskTestCheck(ctx, workspace, revision, "task test baseline", taskCommand, false))
	}
	draftPath := filepath.Join(workspace, filepath.FromSlash(file.Path))
	if err := os.MkdirAll(filepath.Dir(draftPath), 0700); err != nil {
		return DraftCheckReport{}, fmt.Errorf("create draft directory: %w", err)
	}
	if err := os.WriteFile(draftPath, []byte(input.source), 0600); err != nil {
		return DraftCheckReport{}, fmt.Errorf("write draft workspace file: %w", err)
	}

	report := DraftCheckReport{TargetPath: file.Path, Checks: make([]DraftCheck, 0, 4)}
	switch file.Language {
	case "Go":
		report.Checks = append(report.Checks, parseGoDraft(draftPath, input.source))
		report.Checks = append(report.Checks, s.runCheck(ctx, workspace, revision, "format", true, []string{"gofmt", "-d", file.Path}, true, false))
		if options.RunLint {
			report.Checks = append(report.Checks, s.runCheck(ctx, workspace, revision, "lint", false, []string{"go", "vet", "./..."}, false, false))
		} else {
			report.Checks = append(report.Checks, DraftCheck{Name: "lint", State: CheckSkipped})
		}
		if options.RunTests {
			report.Checks = append(report.Checks, s.runCheck(ctx, workspace, revision, "tests", false, []string{"go", "test", "./..."}, false, true))
		} else {
			report.Checks = append(report.Checks, DraftCheck{Name: "tests", State: CheckSkipped})
		}
	default:
		report.Checks = append(report.Checks,
			DraftCheck{Name: "parse", State: CheckUnavailable},
			DraftCheck{Name: "format", Required: true, State: CheckUnavailable},
			DraftCheck{Name: "lint", State: CheckUnavailable},
			DraftCheck{Name: "tests", State: CheckUnavailable},
		)
	}
	if len(taskChecks) == 1 && taskChecks[0].State == CheckPassed {
		taskChecks = append(taskChecks, s.runTaskTestCheck(ctx, workspace, revision, "task test verification", taskCommand, true))
	}
	report.Checks = append(report.Checks, taskChecks...)
	report.Applicable = requiredChecksPassed(report.Checks)
	return report, nil
}

func taskTestPath(workspace, targetPath string) (string, error) {
	directory := filepath.Join(workspace, filepath.Dir(filepath.FromSlash(targetPath)))
	for suffix := 0; suffix < 100; suffix++ {
		name := "mini_orca_task_test.go"
		if suffix > 0 {
			name = fmt.Sprintf("mini_orca_task_%d_test.go", suffix)
		}
		path := filepath.Join(directory, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("no available generated task test filename")
}

func (s *Service) runTaskTestCheck(ctx context.Context, workspace, revision, name string, command []string, expectPass bool) DraftCheck {
	check, err := s.executeDraftCheck(ctx, workspace, revision, DraftCheck{Name: name, Required: true, Command: append([]string(nil), command...)}, true)
	if check.State == CheckCanceled {
		return check
	}
	passed := err == nil
	if passed == expectPass {
		check.State = CheckPassed
	} else {
		check.State = CheckFailed
	}
	return check
}

func parseGoDraft(path, source string) DraftCheck {
	check := DraftCheck{Name: "parse", Required: true}
	if _, err := parser.ParseFile(token.NewFileSet(), path, source, parser.AllErrors); err != nil {
		check.State = CheckFailed
		check.Output = err.Error()
		return check
	}
	check.State = CheckPassed
	return check
}

func (s *Service) runCheck(ctx context.Context, workspace, revision, name string, required bool, command []string, failOnOutput, executesProjectCode bool) DraftCheck {
	check, err := s.executeDraftCheck(ctx, workspace, revision, DraftCheck{Name: name, Required: required, Command: append([]string(nil), command...)}, executesProjectCode)
	if check.State == CheckCanceled {
		return check
	}
	if err != nil || failOnOutput && check.Output != "" {
		check.State = CheckFailed
		return check
	}
	check.State = CheckPassed
	return check
}

func (s *Service) executeDraftCheck(ctx context.Context, workspace, revision string, check DraftCheck, executesProjectCode bool) (DraftCheck, error) {
	if err := ctx.Err(); err != nil {
		check.State = CheckCanceled
		check.Output = err.Error()
		return check, err
	}
	timed, cancel := context.WithTimeout(ctx, s.focusedCheckTimeout)
	defer cancel()
	result, err := s.runCheckCommand(timed, workspace, check.Command, revision, executesProjectCode)
	check.Output = sanitizeCheckOutput(result.output, result.truncated, workspace, s.manager.Root())
	check.ExitCode = result.exitCode
	if timed.Err() != nil {
		check.State = CheckCanceled
		check.Output = timed.Err().Error()
		return check, timed.Err()
	}
	return check, err
}

func (s *Service) runCheckCommand(ctx context.Context, workspace string, command []string, revision string, executesProjectCode bool) (checkCommandResult, error) {
	if !executesProjectCode {
		return runCheckCommand(ctx, workspace, command)
	}
	return runCheckCommandWithStart(ctx, workspace, command, true, func() error {
		return s.requireProjectExecutionTrust(revision)
	})
}

type checkCommandResult struct {
	output    string
	exitCode  int
	truncated bool
}

func runCheckCommand(ctx context.Context, directory string, command []string) (checkCommandResult, error) {
	return runCheckCommandWithStart(ctx, directory, command, false, nil)
}

func runCheckCommandWithStart(ctx context.Context, directory string, command []string, ownsDescendants bool, beforeStart func() error) (checkCommandResult, error) {
	if len(command) == 0 {
		return checkCommandResult{}, fmt.Errorf("empty check command")
	}
	process := exec.CommandContext(ctx, command[0], command[1:]...)
	process.Dir = directory
	process.Env = checkChildEnvironment(os.Environ())
	if ownsDescendants {
		if err := configureCheckCommand(process); err != nil {
			return checkCommandResult{}, err
		}
	}
	process.WaitDelay = checkLimits.pipeDrainWait
	output := &boundedCheckOutput{limit: checkLimits.maxOutputBytes}
	process.Stdout = output
	process.Stderr = output
	if beforeStart != nil {
		if err := beforeStart(); err != nil {
			return checkCommandResult{}, err
		}
	}
	err := process.Run()
	result := checkCommandResult{output: output.String(), truncated: output.Truncated()}
	if exitError, ok := err.(*exec.ExitError); ok {
		result.exitCode = exitError.ExitCode()
	}
	return result, err
}

var checkEnvironmentAllowlist = map[string]bool{
	"PATH": true, "HOME": true, "TMPDIR": true, "TMP": true, "TEMP": true,
	"LANG": true, "LC_ALL": true, "LC_CTYPE": true,
	"GOROOT": true, "GOPATH": true, "GOCACHE": true, "GOENV": true,
	"CGO_ENABLED": true, "CC": true, "CXX": true, "PKG_CONFIG": true,
	"SYSTEMROOT": true, "SystemRoot": true, "COMSPEC": true, "ComSpec": true,
}

// checkChildEnvironment starts from a small toolchain allowlist. In
// particular it never inherits provider credentials or arbitrary host secrets.
func checkChildEnvironment(parent []string) []string {
	child := make([]string, 0, len(checkEnvironmentAllowlist))
	for _, entry := range parent {
		name, _, found := strings.Cut(entry, "=")
		if found && checkEnvironmentAllowlist[name] {
			child = append(child, entry)
		}
	}
	return child
}

// boundedCheckOutput owns both command streams. Write always reports a full
// write so os/exec continues draining either pipe after the diagnostic budget
// is exhausted.
type boundedCheckOutput struct {
	mu        sync.Mutex
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func (output *boundedCheckOutput) Write(data []byte) (int, error) {
	output.mu.Lock()
	defer output.mu.Unlock()
	remaining := output.limit - output.buffer.Len()
	if remaining <= 0 {
		output.truncated = true
		return len(data), nil
	}
	if len(data) > remaining {
		_, _ = output.buffer.Write(data[:remaining])
		output.truncated = true
		return len(data), nil
	}
	_, _ = output.buffer.Write(data)
	return len(data), nil
}

func (output *boundedCheckOutput) String() string {
	output.mu.Lock()
	defer output.mu.Unlock()
	return output.buffer.String()
}

func (output *boundedCheckOutput) Truncated() bool {
	output.mu.Lock()
	defer output.mu.Unlock()
	return output.truncated
}

func requiredChecksPassed(checks []DraftCheck) bool {
	for _, check := range checks {
		if check.Required && check.State != CheckPassed {
			return false
		}
	}
	return true
}

func sanitizeCheckOutput(output string, truncated bool, workspace, root string) string {
	if workspace != "" {
		output = strings.ReplaceAll(output, workspace, "<workspace>")
	}
	if root != "" {
		output = strings.ReplaceAll(output, root, "<project>")
	}
	output = checkAuthorization.ReplaceAllString(output, "[redacted]")
	output = checkSecret.ReplaceAllString(output, "[redacted]")
	if len(output) > checkLimits.maxOutputBytes {
		output = output[:checkLimits.maxOutputBytes]
		truncated = true
	}
	if truncated {
		output += "\n[output truncated]"
	}
	return strings.TrimSpace(output)
}

// copyCheckWorkspace intentionally keeps its own traversal: a temporary Go
// check workspace needs every regular project file (except metadata and known
// caches), whereas project.WalkProjectFiles returns only source-policy
// candidates. destination must be a temporary workspace owned by the caller.
func copyCheckWorkspace(ctx context.Context, source, destination string) error {
	copier := checkWorkspaceCopier{
		ctx: ctx, source: source, destination: destination,
		buffer: make([]byte, checkCopyBufferSize),
	}
	if err := copier.copy(); err != nil {
		if cleanupErr := os.RemoveAll(destination); cleanupErr != nil {
			return fmt.Errorf("%w; clean failed check workspace: %v", err, cleanupErr)
		}
		return err
	}
	return nil
}

type checkWorkspaceCopier struct {
	ctx         context.Context
	source      string
	destination string
	files       int
	bytes       int64
	buffer      []byte
}

func (copier *checkWorkspaceCopier) copy() error {
	return filepath.WalkDir(copier.source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := copier.ctx.Err(); err != nil {
			return fmt.Errorf("copy check workspace: %w", err)
		}
		relative, err := filepath.Rel(copier.source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if isCheckWorkspaceMetadata(relative) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			if skipCheckWorkspaceDirectory(relative) {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(copier.destination, relative), 0700)
		}
		if !isRegularCheckWorkspaceFile(info) {
			return nil
		}
		if copier.files == checkLimits.maxWorkspaceFiles {
			return fmt.Errorf("check workspace exceeds file limit of %d files", checkLimits.maxWorkspaceFiles)
		}
		if info.Size() > checkLimits.maxWorkspaceBytes-copier.bytes {
			return fmt.Errorf("check workspace exceeds byte limit of %d bytes", checkLimits.maxWorkspaceBytes)
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		if err := validateCheckWorkspaceFile(path, info, input); err != nil {
			_ = input.Close()
			return err
		}
		outputPath := filepath.Join(copier.destination, relative)
		if err := os.MkdirAll(filepath.Dir(outputPath), 0700); err != nil {
			_ = input.Close()
			return err
		}
		output, err := os.OpenFile(outputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err != nil {
			_ = input.Close()
			return err
		}
		copied, copyErr := copyCheckWorkspaceFile(copier.ctx, output, input, copier.buffer, checkLimits.maxWorkspaceBytes-copier.bytes)
		closeErr := output.Close()
		inputErr := input.Close()
		if copyErr != nil {
			_ = os.Remove(outputPath)
			return copyErr
		}
		if closeErr != nil {
			_ = os.Remove(outputPath)
			return closeErr
		}
		if inputErr != nil {
			_ = os.Remove(outputPath)
			return inputErr
		}
		copier.files++
		copier.bytes += copied
		return nil
	})
}

func isRegularCheckWorkspaceFile(info os.FileInfo) bool {
	return info.Mode()&os.ModeSymlink == 0 && info.Mode().IsRegular()
}

func validateCheckWorkspaceFile(path string, expected os.FileInfo, input *os.File) error {
	if !isRegularCheckWorkspaceFile(expected) {
		return fmt.Errorf("check workspace source %q is not a regular file", path)
	}
	opened, err := input.Stat()
	if err != nil {
		return fmt.Errorf("stat check workspace source %q: %w", path, err)
	}
	if !isRegularCheckWorkspaceFile(opened) || !os.SameFile(expected, opened) {
		return fmt.Errorf("check workspace source %q changed while opening", path)
	}
	current, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("recheck workspace source %q: %w", path, err)
	}
	if !isRegularCheckWorkspaceFile(current) || !os.SameFile(expected, current) {
		return fmt.Errorf("check workspace source %q changed while opening", path)
	}
	return nil
}

func copyCheckWorkspaceFile(ctx context.Context, output io.Writer, input io.Reader, buffer []byte, remaining int64) (int64, error) {
	var copied int64
	for {
		if err := ctx.Err(); err != nil {
			return copied, fmt.Errorf("copy check workspace: %w", err)
		}
		read, readErr := input.Read(buffer)
		if read > 0 {
			if int64(read) > remaining-copied {
				return copied, fmt.Errorf("check workspace exceeds byte limit of %d bytes", checkLimits.maxWorkspaceBytes)
			}
			written, writeErr := output.Write(buffer[:read])
			copied += int64(written)
			if writeErr != nil {
				return copied, writeErr
			}
			if written != read {
				return copied, io.ErrShortWrite
			}
		}
		if readErr == io.EOF {
			return copied, nil
		}
		if readErr != nil {
			return copied, readErr
		}
	}
}

func skipCheckWorkspaceDirectory(relative string) bool {
	switch filepath.Base(relative) {
	case "node_modules", ".gradle", ".kotlin", "__pycache__", ".pytest_cache", ".mypy_cache", ".ruff_cache", ".tox", "build", "dist", "target":
		return !checkWorkspaceFixturePath(relative)
	default:
		return false
	}
}

func isCheckWorkspaceMetadata(relative string) bool {
	switch filepath.Base(relative) {
	case ".mini-orca", ".git", ".hg", ".svn":
		return true
	default:
		return false
	}
}

func checkWorkspaceFixturePath(relative string) bool {
	for _, part := range strings.FieldsFunc(filepath.ToSlash(relative), func(r rune) bool { return r == '/' }) {
		if part == "testdata" {
			return true
		}
	}
	return false
}
