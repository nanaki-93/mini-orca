package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type fakeGoBenchmarkRunner struct {
	runs         [][]string
	environments [][]string
	sources      [][]byte
	onRun        func(int)
	output       string
	truncated    bool
	err          error
}

func (runner *fakeGoBenchmarkRunner) Run(_ context.Context, directory string, command, environment []string, _ string) (checkCommandResult, error) {
	runner.runs = append(runner.runs, append([]string(nil), command...))
	runner.environments = append(runner.environments, append([]string(nil), environment...))
	source, _ := os.ReadFile(filepath.Join(directory, "main.go"))
	runner.sources = append(runner.sources, source)
	if runner.onRun != nil {
		runner.onRun(len(runner.runs))
	}
	return checkCommandResult{output: runner.output, truncated: runner.truncated}, runner.err
}

func TestGoBenchmarkCommandUsesFixedEscapedArguments(t *testing.T) {
	got := goBenchmarkCommand("BenchmarkCache[unsafe]")
	want := []string{"go", "test", ".", "-run", "^$", "-bench", "^BenchmarkCache\\[unsafe\\]$", "-count", "5", "-benchtime", "100ms", "-benchmem", "-timeout", "15s"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("command = %#v, want %#v", got, want)
	}
}

func TestBenchmarkCatalogPreviewsScopeBeforeTrust(t *testing.T) {
	service, _, draft := benchmarkDraftFixture(t, false)
	catalog, err := service.AvailableGoBenchmarks(context.Background(), catalogRequest(draft))
	if err != nil || !catalog.Available || catalog.Trusted || len(catalog.Benchmarks) != 1 || catalog.Benchmarks[0].Scope == "" || !reflect.DeepEqual(catalog.Benchmarks[0].Command, goBenchmarkCommand("BenchmarkRun")) {
		t.Fatalf("catalog = %#v, err = %v", catalog, err)
	}
	comparison, err := service.CompareGoBenchmark(context.Background(), comparisonRequest(draft, catalog.Benchmarks[0]))
	if err != nil || comparison.Status != "unavailable" || comparison.Reason != "trusted local execution is required" || !reflect.DeepEqual(comparison.Command, catalog.Benchmarks[0].Command) {
		t.Fatalf("untrusted comparison = %#v, err = %v", comparison, err)
	}
}

func TestCompareGoBenchmarkBindsDisplayedScopeAndFixture(t *testing.T) {
	service, root, draft := benchmarkDraftFixture(t, true)
	catalog, err := service.AvailableGoBenchmarks(context.Background(), catalogRequest(draft))
	if err != nil || len(catalog.Benchmarks) != 1 {
		t.Fatal(err)
	}
	runner := &fakeGoBenchmarkRunner{output: benchmarkOutput("BenchmarkRun")}
	service.benchmarkRunner = runner
	request := comparisonRequest(draft, catalog.Benchmarks[0])
	request.ExpectedScope = "benchmark:wrong"
	unavailable, err := service.CompareGoBenchmark(context.Background(), request)
	if err != nil || unavailable.Status != "unavailable" || unavailable.Reason != "displayed benchmark scope changed" || len(runner.runs) != 0 {
		t.Fatalf("scope result = %#v, calls=%d, err=%v", unavailable, len(runner.runs), err)
	}
	comparison, err := service.CompareGoBenchmark(context.Background(), comparisonRequest(draft, catalog.Benchmarks[0]))
	if err != nil || comparison.Status != "completed" || len(runner.runs) != 2 || !reflect.DeepEqual(runner.runs[0], runner.runs[1]) || !reflect.DeepEqual(runner.environments[0], runner.environments[1]) {
		t.Fatalf("comparison=%#v calls=%#v environments=%#v err=%v", comparison, runner.runs, runner.environments, err)
	}
	original, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil || string(runner.sources[0]) != string(original) || string(runner.sources[1]) == string(original) {
		t.Fatalf("source isolation failed: %q %q %v", runner.sources[0], runner.sources[1], err)
	}
	if _, err := service.ApplyDraft(context.Background(), applyDraftRequest(draft)); err == nil {
		t.Fatal("comparison enabled Apply")
	}
}

func TestCompareGoBenchmarkRejectsFixtureChangedAfterPreview(t *testing.T) {
	for path, content := range map[string]string{
		"bench_test.go": "package main\n\nimport \"testing\"\n\nfunc BenchmarkRun(b *testing.B) { b.ReportMetric(1, \"changed\") }\n",
		"go.mod":        "module fixture\n\ngo 1.22\n// changed\n",
		"dependency.go": "package main\n\nfunc Dependency() {}\n",
	} {
		t.Run(path, func(t *testing.T) {
			service, root, draft := benchmarkDraftFixture(t, true)
			choice := onlyBenchmark(t, service, draft)
			if err := os.WriteFile(filepath.Join(root, path), []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			runner := &fakeGoBenchmarkRunner{output: benchmarkOutput("BenchmarkRun")}
			service.benchmarkRunner = runner
			comparison, err := service.CompareGoBenchmark(context.Background(), comparisonRequest(draft, choice))
			if err != nil || comparison.Status != "unavailable" || comparison.Reason != "displayed benchmark scope changed" || len(runner.runs) != 0 {
				t.Fatalf("comparison=%#v runs=%d err=%v", comparison, len(runner.runs), err)
			}
		})
	}
}

func TestCaptureBenchmarkFixtureRejectsChangeAfterScopeRecheck(t *testing.T) {
	service, root, draft := benchmarkDraftFixture(t, true)
	choice := onlyBenchmark(t, service, draft)
	input, _, err := service.composeDraftCheckInput(*draft)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "dependency.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, _, _, _, err = service.captureBenchmarkFixture(context.Background(), input, choice.fixtureFingerprint)
	if !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("capture error = %v, want fixture conflict", err)
	}
}

func TestCompareGoBenchmarkInvalidatesBenchmarkAndModuleChanges(t *testing.T) {
	for _, changed := range []string{"bench_test.go", "go.mod"} {
		t.Run(changed, func(t *testing.T) {
			service, root, draft := benchmarkDraftFixture(t, true)
			choice := onlyBenchmark(t, service, draft)
			runner := &fakeGoBenchmarkRunner{output: benchmarkOutput("BenchmarkRun")}
			runner.onRun = func(run int) {
				if run == 1 {
					if err := os.WriteFile(filepath.Join(root, changed), []byte("changed\n"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			service.benchmarkRunner = runner
			_, err := service.CompareGoBenchmark(context.Background(), comparisonRequest(draft, choice))
			if !errors.Is(err, project.ErrRevisionConflict) || len(runner.runs) != 1 {
				t.Fatalf("mutation error=%v calls=%d", err, len(runner.runs))
			}
		})
	}
}

func TestRunGoBenchmarkRejectsTruncatedOutput(t *testing.T) {
	service, _, draft := benchmarkDraftFixture(t, true)
	choice := onlyBenchmark(t, service, draft)
	service.benchmarkRunner = &fakeGoBenchmarkRunner{output: benchmarkOutput("BenchmarkRun"), truncated: true}
	measurement, state, err := service.runGoBenchmark(context.Background(), t.TempDir(), choice, []string{"PATH=/bin"}, draft.ProjectRevision)
	if err == nil || state != "failed" || len(measurement.Samples) != 0 {
		t.Fatalf("truncated result=%#v state=%q err=%v", measurement, state, err)
	}
}

func TestCompareGoBenchmarkRunsDisposableRealBenchmark(t *testing.T) {
	service, root, draft := benchmarkDraftFixture(t, true)
	original, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	comparison, err := service.CompareGoBenchmark(context.Background(), comparisonRequest(draft, onlyBenchmark(t, service, draft)))
	if err != nil || comparison.Status != "completed" || len(comparison.Base.Samples) != benchmarkSampleCount || len(comparison.Candidate.Samples) != benchmarkSampleCount {
		t.Fatalf("comparison=%#v err=%v", comparison, err)
	}
	after, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil || string(after) != string(original) {
		t.Fatalf("real benchmark changed source: %q %v", after, err)
	}
}

func TestDiscoverGoBenchmarksVerifiesTestingImportAndFiles(t *testing.T) {
	root := t.TempDir()
	writeBenchmarkFile(t, root, "valid_test.go", "package fixture\nimport t \"testing\"\nfunc BenchmarkAlias(b *t.B) {}\n")
	writeBenchmarkFile(t, root, "lookalike_test.go", "package fixture\nimport x \"example.invalid/testing\"\nfunc BenchmarkLookalike(b *x.B) {}\n")
	benchmarks, err := discoverGoBenchmarks(root, "main.go")
	if err != nil || !reflect.DeepEqual(benchmarks, []string{"BenchmarkAlias"}) {
		t.Fatalf("benchmarks=%#v err=%v", benchmarks, err)
	}
	if err := os.Symlink(filepath.Join(root, "valid_test.go"), filepath.Join(root, "link_test.go")); err != nil {
		t.Fatal(err)
	}
	if _, err := discoverGoBenchmarks(root, "main.go"); err == nil {
		t.Fatal("symlink benchmark source was accepted")
	}
}

func TestDiscoverGoBenchmarksRequiresExactNonGenericSignature(t *testing.T) {
	root := t.TempDir()
	writeBenchmarkFile(t, root, "signature_test.go", `package fixture
import t "testing"
func BenchmarkUnnamed(*t.B) {}
func BenchmarkNamed(b *t.B) {}
func BenchmarkTwoNames(first, second *t.B) {}
func BenchmarkGeneric[T any](b *t.B) {}
`)
	benchmarks, err := discoverGoBenchmarks(root, "main.go")
	if err != nil || !reflect.DeepEqual(benchmarks, []string{"BenchmarkNamed", "BenchmarkUnnamed"}) {
		t.Fatalf("benchmarks=%#v err=%v", benchmarks, err)
	}
}

func TestReadBenchmarkSourceRejectsReplacementAndGrowth(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bench_test.go")
	if err := os.WriteFile(path, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("first growth"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readBenchmarkSource(path, info); err == nil {
		t.Fatal("growth was accepted")
	}
	if err := os.WriteFile(path, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err = os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	replacement := path + ".replacement"
	if err := os.WriteFile(replacement, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	if _, err := readBenchmarkSource(path, info); err == nil {
		t.Fatal("replacement was accepted")
	}
}

func TestParseGoBenchmarkSamplesRejectsInvalidSelectedRows(t *testing.T) {
	valid := benchmarkOutput("BenchmarkRun")
	for name, output := range map[string]string{
		"zero iterations":      strings.Replace(valid, "  100  ", "  0  ", 1),
		"negative ns":          strings.Replace(valid, "  12.5 ns/op", "  -1 ns/op", 1),
		"non finite":           strings.Replace(valid, "  12.5 ns/op", "  NaN ns/op", 1),
		"negative memory":      strings.Replace(valid, "  8 B/op", "  -1 B/op", 1),
		"malformed cpu suffix": strings.Replace(valid, "BenchmarkRun-8", "BenchmarkRun-invalid", 1),
		"zero cpu suffix":      strings.Replace(valid, "BenchmarkRun-8", "BenchmarkRun-0", 1),
		"padded cpu suffix":    strings.Replace(valid, "BenchmarkRun-8", "BenchmarkRun-08", 1),
		"overflow cpu suffix":  strings.Replace(valid, "BenchmarkRun-8", "BenchmarkRun-"+strings.Repeat("9", 100), 1),
		"malformed":            valid + "\nBenchmarkRun-8 broken",
		"extra":                valid + "\nBenchmarkRun-8  100  12.5 ns/op  8 B/op  1 allocs/op",
		"subbenchmark":         valid + "\nBenchmarkRun/child-8  100  12.5 ns/op  8 B/op  1 allocs/op",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseGoBenchmarkSamples(output, "BenchmarkRun"); err == nil {
				t.Fatal("invalid selected row was accepted")
			}
		})
	}
	if _, err := parseGoBenchmarkSamples(strings.Join(strings.Split(valid, "\n")[:benchmarkSampleCount-1], "\n"), "BenchmarkRun"); err == nil {
		t.Fatal("missing selected sample was accepted")
	}
}

func TestParseGoBenchmarkSamplesAcceptsStandardAndCustomMetrics(t *testing.T) {
	output := benchmarkOutputWithMetrics("BenchmarkRun", "  1.25 MB/s  42 custom-metric")
	samples, err := parseGoBenchmarkSamples(output, "BenchmarkRun")
	if err != nil || len(samples) != benchmarkSampleCount || samples[0].Nanoseconds != 12.5 || *samples[0].Bytes != 8 || *samples[0].Allocations != 1 {
		t.Fatalf("samples=%#v err=%v", samples, err)
	}
	withoutCPUSuffix := strings.ReplaceAll(output, "BenchmarkRun-8", "BenchmarkRun")
	if samples, err := parseGoBenchmarkSamples(withoutCPUSuffix, "BenchmarkRun"); err != nil || len(samples) != benchmarkSampleCount {
		t.Fatalf("single-CPU samples=%#v err=%v", samples, err)
	}
	valid := benchmarkOutput("BenchmarkRun")
	for name, output := range map[string]string{
		"duplicate metric":        benchmarkOutputWithMetrics("BenchmarkRun", "  1 MB/s  2 MB/s"),
		"duplicate required unit": strings.Replace(valid, "  1 allocs/op", "  1 allocs/op  2 ns/op", 1),
		"noninteger bytes":        strings.Replace(valid, "  8 B/op", "  1.5 B/op", 1),
		"nonfinite custom":        benchmarkOutputWithMetrics("BenchmarkRun", "  NaN custom"),
		"negative custom":         benchmarkOutputWithMetrics("BenchmarkRun", "  -1 custom"),
		"negative zero custom":    benchmarkOutputWithMetrics("BenchmarkRun", "  -0 custom"),
		"malformed pair":          benchmarkOutputWithMetrics("BenchmarkRun", "  1 MB/s  dangling"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseGoBenchmarkSamples(output, "BenchmarkRun"); err == nil {
				t.Fatal("invalid metrics were accepted")
			}
		})
	}
}

func TestFingerprintCheckWorkspaceCancelsDuringRead(t *testing.T) {
	root := t.TempDir()
	writeBenchmarkFile(t, root, "large.go", strings.Repeat("x", 2*checkCopyBufferSize))
	ctx := &cancelAfterBenchmarkContext{cancelAt: 4}
	if _, err := fingerprintCheckWorkspace(ctx, root); !errors.Is(err, context.Canceled) {
		t.Fatalf("fingerprint error = %v, want context cancellation", err)
	}
}

func TestCompareGoBenchmarkClearsPartialResultOnCancellation(t *testing.T) {
	service, _, draft := benchmarkDraftFixture(t, true)
	choice := onlyBenchmark(t, service, draft)
	ctx, cancel := context.WithCancel(context.Background())
	runner := &fakeGoBenchmarkRunner{output: benchmarkOutput("BenchmarkRun")}
	runner.onRun = func(run int) {
		if run == 2 {
			cancel()
		}
	}
	service.benchmarkRunner = runner
	comparison, err := service.CompareGoBenchmark(ctx, comparisonRequest(draft, choice))
	if err != nil || comparison.Status != "canceled" || comparison.Base != nil || comparison.Candidate != nil {
		t.Fatalf("comparison=%#v err=%v", comparison, err)
	}
}

func TestCompareGoBenchmarkClearsBaseSamplesWhenCandidateFails(t *testing.T) {
	for name, failCandidate := range map[string]func(*fakeGoBenchmarkRunner){
		"command failure":  func(runner *fakeGoBenchmarkRunner) { runner.err = errors.New("candidate failed") },
		"malformed output": func(runner *fakeGoBenchmarkRunner) { runner.output = "BenchmarkRun-8 malformed" },
		"truncated output": func(runner *fakeGoBenchmarkRunner) { runner.truncated = true },
	} {
		t.Run(name, func(t *testing.T) {
			service, _, draft := benchmarkDraftFixture(t, true)
			choice := onlyBenchmark(t, service, draft)
			runner := &fakeGoBenchmarkRunner{output: benchmarkOutput("BenchmarkRun")}
			runner.onRun = func(run int) {
				if run == 2 {
					failCandidate(runner)
				}
			}
			service.benchmarkRunner = runner
			comparison, err := service.CompareGoBenchmark(context.Background(), comparisonRequest(draft, choice))
			if err != nil || comparison.Status != "failed" || comparison.Base != nil || comparison.Candidate != nil {
				t.Fatalf("comparison=%#v err=%v", comparison, err)
			}
		})
	}
}

func benchmarkDraftFixture(t *testing.T, trust bool) (*Service, string, *Draft) {
	t.Helper()
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	for path, content := range map[string]string{"go.mod": "module fixture\n\ngo 1.22\n", "main.go": "package main\n\nfunc Run() int { return 1 }\n", "bench_test.go": "package main\n\nimport \"testing\"\n\nfunc BenchmarkRun(b *testing.B) { for i := 0; i < b.N; i++ { _ = Run() } }\n"} {
		writeBenchmarkFile(t, root, path, content)
	}
	index, err := service.manager.Reindex()
	if err != nil {
		t.Fatal(err)
	}
	file, err := service.manager.IndexedFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	draft, err := service.CreateDraft(DraftCreateRequest{ID: "benchmark-draft", ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, BaseFileHash: file.ContentHash, TargetPath: "main.go", Mode: project.DeclarationEditReplaceSymbol, TargetSymbol: "Run", Declaration: "func Run() int { return 2 }"})
	if err != nil {
		t.Fatal(err)
	}
	draft, err = service.ValidateDraft(draft.ID, draft.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if trust {
		if _, err := service.TrustProjectExecution(draft.ProjectRevision, true); err != nil {
			t.Fatal(err)
		}
	}
	return service, root, draft
}

func catalogRequest(draft *Draft) GoBenchmarkCatalogRequest {
	return GoBenchmarkCatalogRequest{ID: draft.ID, ExpectedRevision: draft.Revision, ExpectedHash: draft.Hash}
}

func onlyBenchmark(t *testing.T, service *Service, draft *Draft) GoBenchmarkChoice {
	t.Helper()
	catalog, err := service.AvailableGoBenchmarks(context.Background(), catalogRequest(draft))
	if err != nil || len(catalog.Benchmarks) != 1 {
		t.Fatalf("catalog=%#v err=%v", catalog, err)
	}
	return catalog.Benchmarks[0]
}

func comparisonRequest(draft *Draft, choice GoBenchmarkChoice) GoBenchmarkComparisonRequest {
	return GoBenchmarkComparisonRequest{GoBenchmarkCatalogRequest: catalogRequest(draft), Benchmark: choice.Name, ExpectedScope: choice.Scope}
}

func benchmarkOutput(name string) string {
	return benchmarkOutputWithMetrics(name, "")
}

func benchmarkOutputWithMetrics(name, metrics string) string {
	lines := make([]string, benchmarkSampleCount)
	for i := range lines {
		lines[i] = name + "-8  100  12.5 ns/op  8 B/op  1 allocs/op" + metrics
	}
	return strings.Join(lines, "\n")
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
