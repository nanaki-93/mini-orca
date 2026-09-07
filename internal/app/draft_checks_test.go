package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func TestDraftChecksUseIsolatedWorkspace(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	originalPath := filepath.Join(root, "main.go")
	original, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatal(err)
	}
	source := "package main\n\nimport \"fmt\"\n\nfunc Run() { fmt.Println(\"changed\") }\n"
	report, err := runFixtureDraftChecks(service, context.Background(), source, DraftCheckOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Applicable || len(report.Checks) != 4 || report.Checks[0].State != CheckPassed || report.Checks[1].State != CheckPassed {
		t.Fatalf("report = %+v", report)
	}
	after, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Fatal("draft checks modified the imported project")
	}
}

func TestDraftChecksReportFormatterFailure(t *testing.T) {
	service, _ := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	source := "package main\nimport \"fmt\"\nfunc Run(){fmt.Println(\"changed\")}\n"
	report, err := runFixtureDraftChecks(service, context.Background(), source, DraftCheckOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Applicable || report.Checks[1].State != CheckFailed {
		t.Fatalf("formatting failure report = %+v", report)
	}
}

func TestRunCheckCommandHonorsCancellation(t *testing.T) {
	t.Setenv("MINI_ORCA_CHECK_HELPER", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := runCheckCommand(ctx, t.TempDir(), checkHelperCommand("cancel-with-output"))
	if err == nil || !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("command error = %v, context = %v", err, ctx.Err())
	}
}

func TestRunCheckCommandBoundsOutputAndPreservesExitCode(t *testing.T) {
	t.Setenv("MINI_ORCA_CHECK_HELPER", "1")
	result, err := runCheckCommand(context.Background(), t.TempDir(), checkHelperCommand("huge-output"))
	if err == nil || result.exitCode != 17 {
		t.Fatalf("command result = %+v, err = %v", result, err)
	}
	if !result.truncated || len(result.output) != checkLimits.maxOutputBytes {
		t.Fatalf("unbounded command result = %+v", result)
	}
	output := sanitizeCheckOutput(result.output, result.truncated, "/workspace", "/project")
	if !strings.Contains(output, "[output truncated]") {
		t.Fatalf("sanitized output = %q", output)
	}
}

func TestRunCheckCommandCancellationDoesNotWaitForInheritedOutputPipe(t *testing.T) {
	t.Setenv("MINI_ORCA_CHECK_HELPER", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := runCheckCommand(ctx, t.TempDir(), checkHelperCommand("cancel-with-output"))
	if err == nil || !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("command error = %v, context = %v", err, ctx.Err())
	}
	if elapsed := time.Since(started); elapsed > 2*checkLimits.pipeDrainWait {
		t.Fatalf("canceled command waited %s for inherited output pipe", elapsed)
	}
}

func TestRunCheckCommandUsesAllowlistedChildEnvironment(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "openai-must-not-reach-child")
	t.Setenv("ANTHROPIC_API_KEY", "anthropic-must-not-reach-child")
	t.Setenv("PATH", os.Getenv("PATH"))
	result, err := runCheckCommand(context.Background(), t.TempDir(), checkHelperCommand("environment"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.output, "must-not-reach-child") {
		t.Fatalf("child inherited sentinel: %q", result.output)
	}
	if !strings.Contains(result.output, "path-present") {
		t.Fatalf("child lost toolchain path: %q", result.output)
	}
}

func TestSourceOnlyCheckCommandDoesNotRequireDescendantOwnership(t *testing.T) {
	result, err := runCheckCommand(context.Background(), t.TempDir(), checkHelperCommand("environment"))
	if err != nil || !strings.Contains(result.output, "path-present") {
		t.Fatalf("source-only command = %+v, %v", result, err)
	}
}

func TestRunCheckCommandCancellationStopsDescendants(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "descendant-marker")
	directory := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := runCheckCommandWithStart(ctx, directory, checkHelperCommand("descendant-parent", marker), true, nil)
		done <- err
	}()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.ReadFile(marker); err == nil {
			break
		} else if time.Now().After(deadline) {
			cancel()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("descendant command did not stop after cancellation")
			}
			t.Fatalf("descendant did not start: %v", err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	var err error
	select {
	case err = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("descendant command did not stop after cancellation")
	}
	if err == nil || !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("command error = %v, context = %v", err, ctx.Err())
	}
	before, readErr := os.ReadFile(marker)
	if readErr != nil {
		t.Fatal(readErr)
	}
	time.Sleep(40 * time.Millisecond)
	after, readErr := os.ReadFile(marker)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(after) != string(before) {
		t.Fatalf("descendant survived cancellation: before=%q after=%q", before, after)
	}
}

func TestCheckCommandHelperProcess(t *testing.T) {
	separator := -1
	for index, argument := range os.Args {
		if argument == "--" {
			separator = index
			break
		}
	}
	if separator < 0 || len(os.Args) <= separator+1 {
		return
	}
	mode := os.Args[separator+1]
	argument := ""
	if len(os.Args) > separator+2 {
		argument = os.Args[separator+2]
	}
	switch mode {
	case "environment":
		if os.Getenv("PATH") == "" {
			fmt.Fprint(os.Stdout, "path-missing")
		} else {
			fmt.Fprint(os.Stdout, "path-present")
		}
		fmt.Fprint(os.Stdout, os.Getenv("OPENAI_API_KEY"))
		fmt.Fprint(os.Stdout, os.Getenv("ANTHROPIC_API_KEY"))
	case "huge-output":
		chunk := strings.Repeat("x", checkLimits.maxOutputBytes)
		_, _ = fmt.Fprint(os.Stdout, chunk)
		_, _ = fmt.Fprint(os.Stderr, chunk)
		os.Exit(17)
	case "cancel-with-output":
		child := exec.Command(os.Args[0], "-test.run=^TestCheckCommandHelperProcess$", "--", "keep-output")
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		if err := child.Start(); err != nil {
			os.Exit(18)
		}
		for {
			_, _ = fmt.Fprintln(os.Stdout, "parent output")
			time.Sleep(time.Millisecond)
		}
	case "descendant-parent":
		child := exec.Command(os.Args[0], "-test.run=^TestCheckCommandHelperProcess$", "--", "descendant-child", argument)
		if err := child.Start(); err != nil {
			os.Exit(18)
		}
		for {
			time.Sleep(time.Millisecond)
		}
	case "descendant-child":
		for counter := 0; ; counter++ {
			_ = os.WriteFile(argument, []byte(fmt.Sprintf("%d", counter)), 0600)
			time.Sleep(time.Millisecond)
		}
	case "keep-output":
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			_, _ = fmt.Fprintln(os.Stdout, "child output")
			time.Sleep(time.Millisecond)
		}
	}
	os.Exit(0)
}

func checkHelperCommand(mode string, arguments ...string) []string {
	return append([]string{os.Args[0], "-test.run=^TestCheckCommandHelperProcess$", "--", mode}, arguments...)
}

func TestCopyCheckWorkspaceExcludesCachesAndPreservesTestFixtures(t *testing.T) {
	source := t.TempDir()
	destination := filepath.Join(t.TempDir(), "workspace")
	for path, content := range map[string]string{
		"main.go":                               "package fixture\n",
		".git":                                  "gitdir: /outside/worktree\n",
		"node_modules/dependency/index.js":      "cache",
		"desktop/build/generated.txt":           "cache",
		".mini-orca/check.json":                 "metadata",
		"testdata/node_modules/fixture.txt":     "required fixture",
		"testdata/nested/.git":                  "gitdir: /outside/fixture-worktree\n",
		"testdata/nested/.hg/store":             "metadata fixture",
		"testdata/nested/.svn/entries":          "metadata fixture",
		"testdata/nested/.mini-orca/check.json": "metadata fixture",
	} {
		full := filepath.Join(source, path)
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(source, "main.go"), filepath.Join(source, "linked.go")); err != nil {
		t.Fatal(err)
	}
	if err := copyCheckWorkspace(context.Background(), source, destination); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"main.go", "testdata/node_modules/fixture.txt"} {
		if _, err := os.Stat(filepath.Join(destination, path)); err != nil {
			t.Fatalf("required copied path %q: %v", path, err)
		}
	}
	for _, path := range []string{".git", "node_modules", "desktop/build", ".mini-orca", "linked.go", "testdata/nested/.git", "testdata/nested/.hg", "testdata/nested/.svn", "testdata/nested/.mini-orca"} {
		if _, err := os.Lstat(filepath.Join(destination, path)); !os.IsNotExist(err) {
			t.Fatalf("excluded path %q has error %v", path, err)
		}
	}
}

func TestValidateCheckWorkspaceFileRejectsNonRegularAndChangedFiles(t *testing.T) {
	directory := t.TempDir()
	regularPath := filepath.Join(directory, "regular.txt")
	otherPath := filepath.Join(directory, "other.txt")
	if err := os.WriteFile(regularPath, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(otherPath, []byte("second"), 0600); err != nil {
		t.Fatal(err)
	}
	regular, err := os.Lstat(regularPath)
	if err != nil {
		t.Fatal(err)
	}
	other, err := os.Open(otherPath)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if err := validateCheckWorkspaceFile(regularPath, regular, other); err == nil || !strings.Contains(err.Error(), "changed while opening") {
		t.Fatalf("identity mismatch error = %v", err)
	}
	directoryInfo, err := os.Lstat(directory)
	if err != nil {
		t.Fatal(err)
	}
	input, err := os.Open(regularPath)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	if err := validateCheckWorkspaceFile(directory, directoryInfo, input); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("nonregular source error = %v", err)
	}
}

func TestCopyCheckWorkspaceRejectsResourceLimitAndCleansWorkspace(t *testing.T) {
	for _, test := range []struct {
		name  string
		setup func(t *testing.T, source string)
		limit string
	}{
		{
			name: "bytes",
			setup: func(t *testing.T, source string) {
				t.Helper()
				path := filepath.Join(source, "large.bin")
				if err := os.WriteFile(path, nil, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Truncate(path, checkLimits.maxWorkspaceBytes+1); err != nil {
					t.Fatal(err)
				}
			},
			limit: "byte limit",
		},
		{
			name: "files",
			setup: func(t *testing.T, source string) {
				t.Helper()
				for index := 0; index <= checkLimits.maxWorkspaceFiles; index++ {
					path := filepath.Join(source, fmt.Sprintf("file-%05d", index))
					if err := os.WriteFile(path, nil, 0600); err != nil {
						t.Fatal(err)
					}
				}
			},
			limit: "file limit",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := t.TempDir()
			destination := filepath.Join(t.TempDir(), "workspace")
			test.setup(t, source)
			err := copyCheckWorkspace(context.Background(), source, destination)
			if err == nil || !strings.Contains(err.Error(), test.limit) {
				t.Fatalf("copy error = %v", err)
			}
			if _, statErr := os.Stat(destination); !os.IsNotExist(statErr) {
				t.Fatalf("failed copy left workspace behind: %v", statErr)
			}
		})
	}
}

func TestCopyCheckWorkspaceHonorsCancellation(t *testing.T) {
	source := t.TempDir()
	destination := filepath.Join(t.TempDir(), "workspace")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := copyCheckWorkspace(ctx, source, destination)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("copy error = %v", err)
	}
	if _, statErr := os.Stat(destination); !os.IsNotExist(statErr) {
		t.Fatalf("canceled copy left workspace behind: %v", statErr)
	}
}

func TestTaskTestChecksRequireBaseFailureAndCandidatePassWithoutWritingProject(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n\ngo 1.22\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc Run() bool { return false }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reindex(); err != nil {
		t.Fatal(err)
	}
	test := project.GoTestCandidateSpec{Name: "TestRun", Content: "package main\n\nimport \"testing\"\n\nfunc TestRun(t *testing.T) { if !Run() { t.Fatal(\"expected true\") } }\n"}
	report, err := runFixtureDraftChecks(service, context.Background(), "package main\n\nfunc Run() bool { return true }\n", DraftCheckOptions{}, &test)
	checks := taskChecks(report.Checks)
	if err != nil || len(checks) != 2 || checks[0].State != CheckPassed || checks[1].State != CheckPassed {
		t.Fatalf("task checks = %+v, err = %v", checks, err)
	}
	if _, err := os.Stat(filepath.Join(root, "mini_orca_task_test.go")); !os.IsNotExist(err) {
		t.Fatalf("task test escaped temporary workspace: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc Run() bool { return true }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reindex(); err != nil {
		t.Fatal(err)
	}
	report, err = runFixtureDraftChecks(service, context.Background(), "package main\n\nfunc Run() bool { return true }\n", DraftCheckOptions{}, &test)
	checks = taskChecks(report.Checks)
	if err != nil || len(checks) != 1 || checks[0].State != CheckFailed {
		t.Fatalf("unexpectedly passing baseline = %+v, err = %v", checks, err)
	}
}

func TestTaskTestChecksReportCandidateFailureAndUseNonConflictingFilename(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n\ngo 1.22\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "mini_orca_task_test.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc Run() bool { return false }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reindex(); err != nil {
		t.Fatal(err)
	}
	test := project.GoTestCandidateSpec{Name: "TestRun", Content: "package main\n\nimport \"testing\"\n\nfunc TestRun(t *testing.T) { if !Run() { t.Fatal(\"expected true\") } }\n"}
	report, err := runFixtureDraftChecks(service, context.Background(), "package main\n\nfunc Run() bool { return false }\n", DraftCheckOptions{}, &test)
	checks := taskChecks(report.Checks)
	if err != nil || len(checks) != 2 || checks[0].State != CheckPassed || checks[1].State != CheckFailed {
		t.Fatalf("draft failure checks = %+v, err = %v", checks, err)
	}
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "nested", "mini_orca_task_test.go"), []byte("package nested\n"), 0600); err != nil {
		t.Fatal(err)
	}
	path, err := taskTestPath(workspace, "nested/main.go")
	if err != nil || filepath.Base(path) != "mini_orca_task_1_test.go" {
		t.Fatalf("task filename = %q, err = %v", path, err)
	}
}

func taskChecks(checks []DraftCheck) []DraftCheck {
	result := make([]DraftCheck, 0, 2)
	for _, check := range checks {
		if strings.HasPrefix(check.Name, "task test ") {
			result = append(result, check)
		}
	}
	return result
}

func TestTaskTestCheckCancellationAndEvidenceSanitization(t *testing.T) {
	service, _ := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	analysis, err := service.manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	check := service.runTaskTestCheck(ctx, t.TempDir(), analysis.ProjectRevision, "task", []string{"go", "test", "./..."}, true)
	if check.State != CheckCanceled {
		t.Fatalf("canceled task check = %+v", check)
	}
	evidence := sanitizeCheckOutput("/tmp/work/api_key=private\npassword: hidden\n"+strings.Repeat("x", checkLimits.maxOutputBytes+1), false, "/tmp/work", "/project")
	if strings.Contains(evidence, "private") || strings.Contains(evidence, "hidden") || !strings.Contains(evidence, "[output truncated]") || !strings.Contains(evidence, "<workspace>") {
		t.Fatalf("sanitized evidence = %q", evidence)
	}
}

func TestSanitizeCheckOutputRedactsAuthorizationValues(t *testing.T) {
	for _, value := range []string{
		"Authorization: Bearer secret.token,with=delimiters",
		"authorization=Basic YWxpY2U6c2VjcmV0",
		"proxy-authorization: Bearer another secret",
	} {
		evidence := sanitizeCheckOutput(value, false, "", "")
		if evidence != "[redacted]" {
			t.Fatalf("authorization evidence = %q", evidence)
		}
	}
}

func TestApplyUndoAndAuditAreConflictSafeAndSourceFree(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	draft := createDraftReadyForApply(t, service)
	applied, err := service.ApplyDraft(context.Background(), applyDraftRequest(draft))
	if err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil || string(current) != "package main\n\nimport \"fmt\"\n\nfunc Run() { println(\"draft\") }\n" {
		t.Fatalf("applied content = %q, %v", current, err)
	}
	audits, err := readAudit(root)
	if err != nil || len(audits) != 1 || stringMustContain(jsonAudit(t, audits), "changed") {
		t.Fatalf("audit = %+v, %v", audits, err)
	}
	restartedManager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := restartedManager.Set(root, &project.Analysis{Name: "fixture", Path: root}); err != nil {
		t.Fatal(err)
	}
	restarted, err := New(scopedTestConfig("http://127.0.0.1:1"), restartedManager)
	if err != nil {
		t.Fatal(err)
	}
	undone, err := restarted.UndoDraft(context.Background(), UndoRequest{ProjectID: draft.ProjectID, ProjectRevision: applied.ProjectRevision, PostApplyHash: applied.PostApplyHash, Confirm: true})
	if err != nil || undone.UndoAvailable {
		t.Fatalf("undo = %+v, %v", undone, err)
	}
	restored, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil || string(restored) == string(current) {
		t.Fatalf("restored content = %q, %v", restored, err)
	}
}

func jsonAudit(t *testing.T, audits []AuditEntry) string {
	t.Helper()
	data, err := json.Marshal(audits)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func stringMustContain(value, unwanted string) bool { return strings.Contains(value, unwanted) }

func runFixtureDraftChecks(service *Service, ctx context.Context, source string, options DraftCheckOptions, taskTest *project.GoTestCandidateSpec) (DraftCheckReport, error) {
	if taskTest != nil || options.RunTests {
		analysis, err := service.manager.Analysis()
		if err != nil {
			return DraftCheckReport{}, err
		}
		if _, err := service.TrustProjectExecution(analysis.ProjectRevision, true); err != nil {
			return DraftCheckReport{}, err
		}
	}
	file, err := service.manager.IndexedFile("main.go")
	if err != nil {
		return DraftCheckReport{}, err
	}
	return service.runDraftChecks(ctx, draftCheckInput{file: *file, source: source, taskTest: taskTest}, options)
}

func BenchmarkCopyCheckWorkspace(b *testing.B) {
	for _, files := range []int{50, 2000} {
		b.Run(fmt.Sprintf("files_%d", files), func(b *testing.B) {
			root := writeCheckWorkspaceBenchmarkFixture(b, files, 4096)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				dest, err := os.MkdirTemp("", "perf01-copy-")
				if err != nil {
					b.Fatal(err)
				}
				err = copyCheckWorkspace(context.Background(), root, dest)
				removeErr := os.RemoveAll(dest)
				if err != nil {
					b.Fatal(err)
				}
				if removeErr != nil {
					b.Fatal(removeErr)
				}
			}
		})
	}
}

func writeCheckWorkspaceBenchmarkFixture(b *testing.B, files, bytes int) string {
	b.Helper()
	root := b.TempDir()
	content := make([]byte, bytes)
	for i := range content {
		content[i] = 'x'
	}
	for i := 0; i < files; i++ {
		path := filepath.Join(root, fmt.Sprintf("pkg%03d", i/100), fmt.Sprintf("file%05d.dat", i))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0600); err != nil {
			b.Fatal(err)
		}
	}
	return root
}
