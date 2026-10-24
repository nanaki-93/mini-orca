package sandbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunCheckCommandHonorsCancellation(t *testing.T) {
	t.Setenv("MINI_ORCA_CHECK_HELPER", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := RunCommand(ctx, t.TempDir(), checkHelperCommand("cancel-with-output"))
	if err == nil || !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("command error = %v, context = %v", err, ctx.Err())
	}
}

func TestRunCheckCommandBoundsOutputAndPreservesExitCode(t *testing.T) {
	t.Setenv("MINI_ORCA_CHECK_HELPER", "1")
	result, err := RunCommand(context.Background(), t.TempDir(), checkHelperCommand("huge-output"))
	if err == nil || result.ExitCode != 17 {
		t.Fatalf("command result = %+v, err = %v", result, err)
	}
	if !result.Truncated || len(result.Output) != checkLimits.maxOutputBytes {
		t.Fatalf("unbounded command result = %+v", result)
	}
	output := SanitizeOutput(result.Output, result.Truncated, "/workspace", "/project")
	if !strings.Contains(output, "[output truncated]") {
		t.Fatalf("sanitized output = %q", output)
	}
}

func TestRunCheckCommandCancellationDoesNotWaitForInheritedOutputPipe(t *testing.T) {
	t.Setenv("MINI_ORCA_CHECK_HELPER", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := RunCommand(ctx, t.TempDir(), checkHelperCommand("cancel-with-output"))
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
	result, err := RunCommand(context.Background(), t.TempDir(), checkHelperCommand("environment"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.Output, "must-not-reach-child") {
		t.Fatalf("child inherited sentinel: %q", result.Output)
	}
	if !strings.Contains(result.Output, "path-present") {
		t.Fatalf("child lost toolchain path: %q", result.Output)
	}
}

func TestSourceOnlyCheckCommandDoesNotRequireDescendantOwnership(t *testing.T) {
	result, err := RunCommand(context.Background(), t.TempDir(), checkHelperCommand("environment"))
	if err != nil || !strings.Contains(result.Output, "path-present") {
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
		_, err := RunCommandWithStart(ctx, directory, checkHelperCommand("descendant-parent", marker), true, nil)
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

func TestRunCheckCommandNormalExitStopsDescendants(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "descendant-exit-marker")
	directory := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result, err := RunCommandWithStart(ctx, directory, checkHelperCommand("descendant-parent-exit", marker), true, nil)
	if err != nil || result.ExitCode != 0 {
		t.Fatalf("command result = %+v, err = %v", result, err)
	}
	cancel()
	if err := os.Remove(marker); err != nil {
		t.Fatalf("expected descendant to have created marker before parent exit: %v", err)
	}
	time.Sleep(40 * time.Millisecond)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("background descendant survived successful check completion and wrote marker: %v", err)
	}
}

func TestVerifyCommandGroupStoppedRequiresReapingAndAbsence(t *testing.T) {
	if err := VerifyCommandGroupStopped(nil); err == nil {
		t.Fatal("missing process accepted as stopped")
	}
	arguments := checkHelperCommand("wait-stdin")
	command := exec.CommandContext(t.Context(), arguments[0], arguments[1:]...)
	if err := ConfigureCommand(command); err != nil {
		t.Skip(err)
	}
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = StopCommandGroup(command); _ = input.Close() })
	if err := VerifyCommandGroupStopped(command); err == nil {
		t.Fatal("unreaped live process accepted as stopped")
	}
	if err := input.Close(); err != nil {
		t.Fatal(err)
	}
	if err := command.Wait(); err != nil {
		t.Fatal(err)
	}
	if err := VerifyCommandGroupStopped(command); err != nil {
		t.Fatalf("reaped empty group: %v", err)
	}

	marker := filepath.Join(t.TempDir(), "remaining-descendant")
	arguments = checkHelperCommand("descendant-parent-exit", marker)
	parent := exec.CommandContext(t.Context(), arguments[0], arguments[1:]...)
	if err := ConfigureCommand(parent); err != nil {
		t.Fatal(err)
	}
	if err := parent.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = StopCommandGroup(parent) })
	if err := parent.Wait(); err != nil {
		t.Fatal(err)
	}
	if err := VerifyCommandGroupStopped(parent); err == nil {
		t.Fatal("remaining descendant accepted as stopped after leader exit")
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
	case "wait-stdin":
		_, _ = io.Copy(io.Discard, os.Stdin)
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
	case "descendant-parent-exit":
		child := exec.Command(os.Args[0], "-test.run=^TestCheckCommandHelperProcess$", "--", "descendant-child", argument)
		if err := child.Start(); err != nil {
			os.Exit(18)
		}
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.ReadFile(argument); err == nil {
				os.Exit(0)
			}
			time.Sleep(time.Millisecond)
		}
		os.Exit(19)
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
		".worktrees/fix/main.go":                "package ignored\n",
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
	if err := CopyWorkspace(context.Background(), source, destination); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"main.go", "testdata/node_modules/fixture.txt"} {
		if _, err := os.Stat(filepath.Join(destination, path)); err != nil {
			t.Fatalf("required copied path %q: %v", path, err)
		}
	}
	for _, path := range []string{".git", ".worktrees", "node_modules", "desktop/build", ".mini-orca", "linked.go", "testdata/nested/.git", "testdata/nested/.hg", "testdata/nested/.svn", "testdata/nested/.mini-orca"} {
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
			err := CopyWorkspace(context.Background(), source, destination)
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
	err := CopyWorkspace(ctx, source, destination)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("copy error = %v", err)
	}
	if _, statErr := os.Stat(destination); !os.IsNotExist(statErr) {
		t.Fatalf("canceled copy left workspace behind: %v", statErr)
	}
}

func TestCopyCheckWorkspaceFileStopsAfterMidCopyCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	input := cancelAfterFirstRead{data: []byte("first-second"), cancel: cancel}
	var output bytes.Buffer

	copied, err := copyCheckWorkspaceFile(ctx, &output, &input, make([]byte, len("first")), int64(len(input.data)))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("copy error = %v", err)
	}
	if copied != int64(len("first")) || output.String() != "first" {
		t.Fatalf("partial copy = %d, %q", copied, output.String())
	}
}

func TestCheckWorkspaceCopierCopyFileRemovesOutputOnCancellation(t *testing.T) {
	source := t.TempDir()
	destination := filepath.Join(t.TempDir(), "workspace")
	sourcePath := filepath.Join(source, "main.go")
	if err := os.WriteFile(sourcePath, []byte("package fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	copier := checkWorkspaceCopier{
		ctx: ctx, source: source, destination: destination,
		buffer: make([]byte, checkCopyBufferSize),
	}

	err = copier.copyFile(sourcePath, "main.go", info)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("copy error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, "main.go")); !os.IsNotExist(err) {
		t.Fatalf("canceled file copy left output behind: %v", err)
	}
}

type cancelAfterFirstRead struct {
	data   []byte
	cancel context.CancelFunc
	read   bool
}

func (reader *cancelAfterFirstRead) Read(destination []byte) (int, error) {
	if reader.read {
		return 0, io.EOF
	}
	reader.read = true
	read := copy(destination, reader.data)
	reader.cancel()
	return read, nil
}

func TestSanitizeCheckOutputRedactsAuthorizationValues(t *testing.T) {
	for _, value := range []string{
		"Authorization: Bearer secret.token,with=delimiters",
		"authorization=Basic YWxpY2U6c2VjcmV0",
		"proxy-authorization: Bearer another secret",
	} {
		evidence := SanitizeOutput(value, false, "", "")
		if evidence != "[redacted]" {
			t.Fatalf("authorization evidence = %q", evidence)
		}
	}
}

func TestFingerprintCheckWorkspaceCancelsDuringRead(t *testing.T) {
	root := t.TempDir()
	writeBenchmarkFile(t, root, "large.go", strings.Repeat("x", 2*checkCopyBufferSize))
	ctx := &cancelAfterBenchmarkContext{cancelAt: 4}
	if _, err := FingerprintWorkspace(ctx, root); !errors.Is(err, context.Canceled) {
		t.Fatalf("fingerprint error = %v, want context cancellation", err)
	}
}

func TestWorkspaceFingerprintMatchesCopiedCheckWorkspace(t *testing.T) {
	root := t.TempDir()
	writeBenchmarkFile(t, root, "main.go", "package main\n")
	if err := os.MkdirAll(filepath.Join(root, ".mini-orca"), 0700); err != nil {
		t.Fatal(err)
	}
	writeBenchmarkFile(t, root, filepath.Join(".mini-orca", "state.json"), "{}")
	if err := os.MkdirAll(filepath.Join(root, "node_modules", "pkg"), 0700); err != nil {
		t.Fatal(err)
	}
	writeBenchmarkFile(t, root, filepath.Join("node_modules", "pkg", "index.js"), "export default 1;\n")

	direct, err := WorkspaceFingerprint(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	copiedDir := t.TempDir()
	if err := CopyWorkspace(context.Background(), root, copiedDir); err != nil {
		t.Fatal(err)
	}
	copied, err := FingerprintWorkspace(context.Background(), copiedDir)
	if err != nil {
		t.Fatal(err)
	}
	if direct != copied {
		t.Fatalf("in-place fingerprint %s != copied fingerprint %s", direct, copied)
	}
}

type cancelAfterBenchmarkContext struct {
	checks   int
	cancelAt int
}

func (ctx *cancelAfterBenchmarkContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (ctx *cancelAfterBenchmarkContext) Done() <-chan struct{}       { return nil }
func (ctx *cancelAfterBenchmarkContext) Value(any) any               { return nil }
func (ctx *cancelAfterBenchmarkContext) Err() error {
	ctx.checks++
	if ctx.checks >= ctx.cancelAt {
		return context.Canceled
	}
	return nil
}

func writeBenchmarkFile(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
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
				err = CopyWorkspace(context.Background(), root, dest)
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
