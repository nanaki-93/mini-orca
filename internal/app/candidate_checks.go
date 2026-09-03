package app

import (
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

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

const (
	CheckNotRun      = "not-run"
	CheckPassed      = "passed"
	CheckFailed      = "failed"
	CheckSkipped     = "skipped"
	CheckCanceled    = "canceled"
	CheckUnavailable = "unavailable"
)

const maxCheckOutputBytes = 8 * 1024

var checkSecret = regexp.MustCompile(`(?i)\b(?:api[_-]?key|access[_-]?token|auth[_-]?token|password|secret|credential|authorization)\b\s*[:=]\s*[^\s,;]+`)

// CandidateCheck is one parser, formatter, lint, or test result. Command is a
// display-only argv preview; it is never executed through a shell.
type CandidateCheck struct {
	Name     string   `json:"name"`
	Required bool     `json:"required"`
	State    string   `json:"state"`
	Command  []string `json:"command,omitempty"`
	Output   string   `json:"output,omitempty"`
	ExitCode int      `json:"exit_code,omitempty"`
}

// CandidateCheckReport determines whether a scope-valid candidate can be
// applied. It contains no source content and all commands run in a copy.
type CandidateCheckReport struct {
	DraftID         string           `json:"draft_id,omitempty"`
	DraftRevision   int64            `json:"draft_revision,omitempty"`
	DraftHash       string           `json:"draft_hash,omitempty"`
	CandidateHash   string           `json:"candidate_hash,omitempty"`
	ProjectID       string           `json:"project_id,omitempty"`
	ProjectRevision string           `json:"project_revision,omitempty"`
	BaseFileHash    string           `json:"base_file_hash,omitempty"`
	TargetPath      string           `json:"target_path"`
	Applicable      bool             `json:"applicable"`
	Checks          []CandidateCheck `json:"checks"`
}

// CandidateCheckOptions keeps expensive checks explicit. Parsing and formatting
// are required where supported; lint and tests run only when requested.
type CandidateCheckOptions struct {
	RunLint  bool `json:"run_lint,omitempty"`
	RunTests bool `json:"run_tests,omitempty"`
}

// RunCandidateChecks validates a candidate in an isolated copy of the active
// project. Neither the candidate nor a formatter/check process can write the
// imported project.
func (s *Service) RunCandidateChecks(ctx context.Context, targetPath, candidate string, options CandidateCheckOptions) (CandidateCheckReport, error) {
	return s.runCandidateChecks(ctx, targetPath, candidate, options, nil)
}

// RunCandidateChecksForTask adds one reviewed task test to the same isolated
// workspace used by the ordinary parser, formatter, lint, and test checks.
func (s *Service) RunCandidateChecksForTask(ctx context.Context, targetPath, candidate string, options CandidateCheckOptions, test *project.GoTestCandidateSpec) (CandidateCheckReport, error) {
	return s.runCandidateChecks(ctx, targetPath, candidate, options, test)
}

func (s *Service) runCandidateChecks(ctx context.Context, targetPath, candidate string, options CandidateCheckOptions, taskTest *project.GoTestCandidateSpec) (CandidateCheckReport, error) {
	file, err := s.manager.IndexedFile(targetPath)
	if err != nil {
		return CandidateCheckReport{}, err
	}
	workspace, err := os.MkdirTemp("", "mini-orca-check-")
	if err != nil {
		return CandidateCheckReport{}, fmt.Errorf("create candidate workspace: %w", err)
	}
	defer os.RemoveAll(workspace)
	if err := copyCandidateWorkspace(s.manager.Root(), workspace); err != nil {
		return CandidateCheckReport{}, err
	}
	var taskChecks []CandidateCheck
	var taskCommand []string
	if taskTest != nil {
		testPath, err := taskTestPath(workspace, file.Path)
		if err != nil {
			return CandidateCheckReport{}, err
		}
		if err := os.WriteFile(testPath, []byte(taskTest.Content), 0600); err != nil {
			return CandidateCheckReport{}, fmt.Errorf("write task test workspace file: %w", err)
		}
		taskCommand = []string{"go", "test", "./...", "-run", "^" + taskTest.Name + "$"}
		taskChecks = append(taskChecks, s.runTaskTestCheck(ctx, workspace, "task test baseline", taskCommand, false))
	}
	candidatePath := filepath.Join(workspace, filepath.FromSlash(file.Path))
	if err := os.MkdirAll(filepath.Dir(candidatePath), 0700); err != nil {
		return CandidateCheckReport{}, fmt.Errorf("create candidate directory: %w", err)
	}
	if err := os.WriteFile(candidatePath, []byte(candidate), 0600); err != nil {
		return CandidateCheckReport{}, fmt.Errorf("write candidate workspace file: %w", err)
	}

	report := CandidateCheckReport{TargetPath: file.Path, Checks: make([]CandidateCheck, 0, 4)}
	switch file.Language {
	case "Go":
		report.Checks = append(report.Checks, parseGoCandidate(candidatePath, candidate))
		report.Checks = append(report.Checks, s.runCheck(ctx, workspace, "format", true, []string{"gofmt", "-d", file.Path}, true))
		if options.RunLint {
			report.Checks = append(report.Checks, s.runCheck(ctx, workspace, "lint", false, []string{"go", "vet", "./..."}, false))
		} else {
			report.Checks = append(report.Checks, CandidateCheck{Name: "lint", State: CheckSkipped})
		}
		if options.RunTests {
			report.Checks = append(report.Checks, s.runCheck(ctx, workspace, "tests", false, []string{"go", "test", "./..."}, false))
		} else {
			report.Checks = append(report.Checks, CandidateCheck{Name: "tests", State: CheckSkipped})
		}
	default:
		report.Checks = append(report.Checks,
			CandidateCheck{Name: "parse", State: CheckUnavailable},
			CandidateCheck{Name: "format", Required: true, State: CheckUnavailable},
			CandidateCheck{Name: "lint", State: CheckUnavailable},
			CandidateCheck{Name: "tests", State: CheckUnavailable},
		)
	}
	if len(taskChecks) == 1 && taskChecks[0].State == CheckPassed {
		taskChecks = append(taskChecks, s.runTaskTestCheck(ctx, workspace, "task test candidate", taskCommand, true))
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

func (s *Service) runTaskTestCheck(ctx context.Context, workspace, name string, command []string, expectPass bool) CandidateCheck {
	check := CandidateCheck{Name: name, Required: true, Command: append([]string(nil), command...)}
	if err := ctx.Err(); err != nil {
		check.State = CheckCanceled
		check.Output = err.Error()
		return check
	}
	timed, cancel := context.WithTimeout(ctx, s.focusedCheckTimeout)
	defer cancel()
	output, exitCode, err := runCheckCommand(timed, workspace, command)
	check.Output = sanitizeCheckOutput(output, workspace, s.manager.Root())
	check.ExitCode = exitCode
	if timed.Err() != nil {
		check.State = CheckCanceled
		check.Output = timed.Err().Error()
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

func parseGoCandidate(path, candidate string) CandidateCheck {
	check := CandidateCheck{Name: "parse", Required: true}
	if _, err := parser.ParseFile(token.NewFileSet(), path, candidate, parser.AllErrors); err != nil {
		check.State = CheckFailed
		check.Output = err.Error()
		return check
	}
	check.State = CheckPassed
	return check
}

func (s *Service) runCheck(ctx context.Context, workspace, name string, required bool, command []string, failOnOutput bool) CandidateCheck {
	check := CandidateCheck{Name: name, Required: required, Command: append([]string(nil), command...)}
	if err := ctx.Err(); err != nil {
		check.State = CheckCanceled
		check.Output = err.Error()
		return check
	}
	timed, cancel := context.WithTimeout(ctx, s.focusedCheckTimeout)
	defer cancel()
	output, exitCode, err := runCheckCommand(timed, workspace, command)
	check.Output = sanitizeCheckOutput(output, workspace, s.manager.Root())
	check.ExitCode = exitCode
	if timed.Err() != nil {
		check.State = CheckCanceled
		check.Output = timed.Err().Error()
		return check
	}
	if err != nil || failOnOutput && check.Output != "" {
		check.State = CheckFailed
		return check
	}
	check.State = CheckPassed
	return check
}

func runCheckCommand(ctx context.Context, directory string, command []string) (string, int, error) {
	if len(command) == 0 {
		return "", 0, fmt.Errorf("empty check command")
	}
	process := exec.CommandContext(ctx, command[0], command[1:]...)
	process.Dir = directory
	output, err := process.CombinedOutput()
	exitCode := 0
	if exitError, ok := err.(*exec.ExitError); ok {
		exitCode = exitError.ExitCode()
	}
	return string(output), exitCode, err
}

func requiredChecksPassed(checks []CandidateCheck) bool {
	for _, check := range checks {
		if check.Required && check.State != CheckPassed {
			return false
		}
	}
	return true
}

func sanitizeCheckOutput(output, workspace, root string) string {
	output = strings.ReplaceAll(output, workspace, "<workspace>")
	output = strings.ReplaceAll(output, root, "<project>")
	output = checkSecret.ReplaceAllString(output, "[redacted]")
	if len(output) > maxCheckOutputBytes {
		output = output[:maxCheckOutputBytes] + "\n[output truncated]"
	}
	return strings.TrimSpace(output)
}

func copyCandidateWorkspace(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		if entry.IsDir() {
			if entry.Name() == ".mini-orca" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(destination, relative), 0700)
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return nil
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		defer input.Close()
		outputPath := filepath.Join(destination, relative)
		if err := os.MkdirAll(filepath.Dir(outputPath), 0700); err != nil {
			return err
		}
		output, err := os.OpenFile(outputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(output, input)
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}
