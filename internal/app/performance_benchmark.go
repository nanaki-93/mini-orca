package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

const (
	benchmarkSampleCount       = 5
	benchmarkDuration          = 100 * time.Millisecond
	benchmarkProcessTimeout    = 15 * time.Second
	benchmarkCommandTimeout    = 20 * time.Second
	maxBenchmarkDiscoveryFiles = 128
	maxBenchmarkSourceBytes    = 1024 * 1024
)

var benchmarkCPUSuffix = regexp.MustCompile(`^-[1-9][0-9]*$`)

type GoBenchmarkChoice struct {
	Name               string   `json:"name"`
	Command            []string `json:"command"`
	Scope              string   `json:"scope"`
	fixtureFingerprint string
}

type GoBenchmarkCatalog struct {
	DraftID            string              `json:"draft_id"`
	DraftRevision      int64               `json:"draft_revision"`
	DraftHash          string              `json:"draft_hash"`
	ProjectID          string              `json:"project_id,omitempty"`
	ProjectRevision    string              `json:"project_revision,omitempty"`
	BaseFileHash       string              `json:"base_file_hash,omitempty"`
	TargetPath         string              `json:"target_path,omitempty"`
	Available          bool                `json:"available"`
	Trusted            bool                `json:"trusted"`
	Reason             string              `json:"reason,omitempty"`
	Benchmarks         []GoBenchmarkChoice `json:"benchmarks"`
	fixtureFingerprint string
}

type GoBenchmarkCatalogRequest struct {
	ID               string
	ExpectedRevision int64
	ExpectedHash     string
}

type GoBenchmarkComparisonRequest struct {
	GoBenchmarkCatalogRequest
	Benchmark     string
	ExpectedScope string
}

type GoBenchmarkSample struct {
	Iterations  int64   `json:"iterations"`
	Nanoseconds float64 `json:"ns_per_op"`
	Bytes       *int64  `json:"bytes_per_op,omitempty"`
	Allocations *int64  `json:"allocs_per_op,omitempty"`
}

type GoBenchmarkMeasurement struct {
	Samples []GoBenchmarkSample `json:"samples"`
}

type GoBenchmarkComparison struct {
	DraftID         string                  `json:"draft_id"`
	DraftRevision   int64                   `json:"draft_revision"`
	DraftHash       string                  `json:"draft_hash"`
	ProjectID       string                  `json:"project_id,omitempty"`
	ProjectRevision string                  `json:"project_revision,omitempty"`
	BaseFileHash    string                  `json:"base_file_hash,omitempty"`
	TargetPath      string                  `json:"target_path,omitempty"`
	Benchmark       string                  `json:"benchmark,omitempty"`
	Scope           string                  `json:"scope,omitempty"`
	Status          string                  `json:"status"`
	Reason          string                  `json:"reason,omitempty"`
	Command         []string                `json:"command,omitempty"`
	Base            *GoBenchmarkMeasurement `json:"base,omitempty"`
	Candidate       *GoBenchmarkMeasurement `json:"candidate,omitempty"`
}

type goBenchmarkRunner interface {
	Run(context.Context, string, []string, []string, string) (checkCommandResult, error)
}

type checkGoBenchmarkRunner struct{ service *Service }

func (runner checkGoBenchmarkRunner) Run(ctx context.Context, workspace string, command, environment []string, revision string) (checkCommandResult, error) {
	return runCheckCommandWithEnvironment(ctx, workspace, command, true, func() error {
		return runner.service.requireProjectExecutionTrust(revision)
	}, environment)
}

// AvailableGoBenchmarks lists exact daemon-built argv previews without running
// project code. Trusted remains false until the existing revision-bound consent.
func (s *Service) AvailableGoBenchmarks(ctx context.Context, request GoBenchmarkCatalogRequest) (*GoBenchmarkCatalog, error) {
	identity, err := newDraftRevisionIdentity(request.ID, request.ExpectedRevision, request.ExpectedHash)
	if err != nil {
		return nil, err
	}
	catalog := &GoBenchmarkCatalog{DraftID: identity.id, DraftRevision: identity.revision, DraftHash: identity.hash, Benchmarks: []GoBenchmarkChoice{}}
	draft, err := s.loadValidatedDraftForChecks(identity)
	if err != nil {
		if errors.Is(err, project.ErrRevisionConflict) {
			return nil, err
		}
		catalog.Reason = "candidate is not valid"
		return catalog, nil
	}
	populateBenchmarkCatalog(catalog, draft)
	root := s.manager.Root()
	benchmarks, err := discoverGoBenchmarks(root, draft.TargetPath)
	if err != nil {
		catalog.Reason = "benchmarks are unavailable"
		return catalog, nil
	}
	fixtureFingerprint, err := benchmarkSourceFingerprint(ctx, root)
	if err != nil {
		if isBenchmarkContextError(err) {
			return nil, err
		}
		catalog.Reason = "benchmarks are unavailable"
		return catalog, nil
	}
	catalog.fixtureFingerprint = fixtureFingerprint
	for _, benchmark := range benchmarks {
		command := goBenchmarkCommand(benchmark)
		catalog.Benchmarks = append(catalog.Benchmarks, GoBenchmarkChoice{Name: benchmark, Command: command, Scope: benchmarkExecutionScope(draft, benchmark, command, fixtureFingerprint), fixtureFingerprint: fixtureFingerprint})
	}
	if len(catalog.Benchmarks) == 0 {
		catalog.Reason = "no benchmark exists in the affected package"
		return catalog, nil
	}
	trust, err := s.ExecutionTrust(draft.ProjectRevision, "")
	if err != nil {
		return nil, err
	}
	catalog.Available, catalog.Trusted = true, trust.Trusted
	return catalog, nil
}

func populateBenchmarkCatalog(catalog *GoBenchmarkCatalog, draft Draft) {
	catalog.ProjectID, catalog.ProjectRevision, catalog.BaseFileHash, catalog.TargetPath = draft.ProjectID, draft.ProjectRevision, draft.BaseFileHash, draft.TargetPath
}

func (s *Service) CompareGoBenchmark(ctx context.Context, request GoBenchmarkComparisonRequest) (*GoBenchmarkComparison, error) {
	comparison, draft, input, choice, unavailable, err := s.prepareGoBenchmarkComparison(ctx, request)
	if err != nil || unavailable {
		return comparison, err
	}
	fixture, base, candidate, cleanup, err := s.captureBenchmarkFixture(ctx, input, choice.fixtureFingerprint)
	if err != nil {
		if isBenchmarkContextError(err) {
			return canceledBenchmarkComparison(comparison), nil
		}
		return nil, err
	}
	defer cleanup()
	return s.measureGoBenchmarkCopies(ctx, comparison, draft, request, choice, fixture, base, candidate)
}

func (s *Service) prepareGoBenchmarkComparison(ctx context.Context, request GoBenchmarkComparisonRequest) (*GoBenchmarkComparison, Draft, draftCheckInput, GoBenchmarkChoice, bool, error) {
	catalog, err := s.AvailableGoBenchmarks(ctx, request.GoBenchmarkCatalogRequest)
	if err != nil {
		return nil, Draft{}, draftCheckInput{}, GoBenchmarkChoice{}, false, err
	}
	choice, found := findBenchmarkChoice(catalog.Benchmarks, request.Benchmark)
	choice.fixtureFingerprint = catalog.fixtureFingerprint
	comparison := comparisonFromCatalog(catalog, request.Benchmark, choice)
	if !catalog.Available || !found {
		comparison.Status, comparison.Reason = "unavailable", unavailableBenchmarkReason(catalog, found)
		return comparison, Draft{}, draftCheckInput{}, GoBenchmarkChoice{}, true, nil
	}
	if request.ExpectedScope == "" || request.ExpectedScope != choice.Scope {
		comparison.Status, comparison.Reason = "unavailable", "displayed benchmark scope changed"
		return comparison, Draft{}, draftCheckInput{}, GoBenchmarkChoice{}, true, nil
	}
	if !catalog.Trusted {
		comparison.Status, comparison.Reason = "unavailable", "trusted local execution is required"
		return comparison, Draft{}, draftCheckInput{}, GoBenchmarkChoice{}, true, nil
	}
	identity, err := newDraftRevisionIdentity(request.ID, request.ExpectedRevision, request.ExpectedHash)
	if err != nil {
		return nil, Draft{}, draftCheckInput{}, GoBenchmarkChoice{}, false, err
	}
	draft, err := s.loadValidatedDraftForChecks(identity)
	if err != nil {
		return nil, Draft{}, draftCheckInput{}, GoBenchmarkChoice{}, false, err
	}
	input, _, err := s.composeDraftCheckInput(draft)
	if err != nil {
		return nil, Draft{}, draftCheckInput{}, GoBenchmarkChoice{}, false, err
	}
	return comparison, draft, input, choice, false, nil
}

func findBenchmarkChoice(choices []GoBenchmarkChoice, name string) (GoBenchmarkChoice, bool) {
	for _, choice := range choices {
		if choice.Name == name {
			return choice, true
		}
	}
	return GoBenchmarkChoice{}, false
}

func unavailableBenchmarkReason(catalog *GoBenchmarkCatalog, found bool) string {
	if !catalog.Available {
		return catalog.Reason
	}
	if !found {
		return "selected benchmark is unavailable"
	}
	return "benchmark is unavailable"
}

type benchmarkFixture struct {
	fingerprint string
	environment []string
}

func (s *Service) captureBenchmarkFixture(ctx context.Context, input draftCheckInput, expectedFingerprint string) (benchmarkFixture, string, string, func(), error) {
	before, err := benchmarkSourceFingerprint(ctx, s.manager.Root())
	if err != nil {
		return benchmarkFixture{}, "", "", nil, err
	}
	if before != expectedFingerprint {
		return benchmarkFixture{}, "", "", nil, project.ErrRevisionConflict
	}
	base, candidate, cleanup, err := newBenchmarkWorkspaces()
	if err != nil {
		return benchmarkFixture{}, "", "", nil, err
	}
	if err := copyCheckWorkspace(ctx, s.manager.Root(), base); err != nil {
		cleanup()
		return benchmarkFixture{}, "", "", nil, err
	}
	captured, err := fingerprintCheckWorkspace(ctx, base)
	if err != nil {
		cleanup()
		return benchmarkFixture{}, "", "", nil, err
	}
	if captured != before {
		cleanup()
		return benchmarkFixture{}, "", "", nil, project.ErrRevisionConflict
	}
	after, err := benchmarkSourceFingerprint(ctx, s.manager.Root())
	if err != nil {
		cleanup()
		return benchmarkFixture{}, "", "", nil, err
	}
	if after != captured {
		cleanup()
		return benchmarkFixture{}, "", "", nil, project.ErrRevisionConflict
	}
	if err := copyCheckWorkspace(ctx, base, candidate); err != nil {
		cleanup()
		return benchmarkFixture{}, "", "", nil, err
	}
	if err := os.WriteFile(filepath.Join(candidate, filepath.FromSlash(input.file.Path)), []byte(input.source), 0600); err != nil {
		cleanup()
		return benchmarkFixture{}, "", "", nil, fmt.Errorf("write candidate benchmark source: %w", err)
	}
	return benchmarkFixture{fingerprint: captured, environment: checkChildEnvironment(os.Environ())}, base, candidate, cleanup, nil
}

func newBenchmarkWorkspaces() (string, string, func(), error) {
	base, err := os.MkdirTemp("", "mini-orca-benchmark-base-")
	if err != nil {
		return "", "", nil, fmt.Errorf("create base benchmark workspace: %w", err)
	}
	candidate, err := os.MkdirTemp("", "mini-orca-benchmark-candidate-")
	if err != nil {
		_ = os.RemoveAll(base)
		return "", "", nil, fmt.Errorf("create candidate benchmark workspace: %w", err)
	}
	return base, candidate, func() { _ = os.RemoveAll(base); _ = os.RemoveAll(candidate) }, nil
}

func benchmarkSourceFingerprint(ctx context.Context, root string) (string, error) {
	workspace, err := os.MkdirTemp("", "mini-orca-benchmark-fingerprint-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(workspace)
	if err := copyCheckWorkspace(ctx, root, workspace); err != nil {
		return "", err
	}
	return fingerprintCheckWorkspace(ctx, workspace)
}

func fingerprintCheckWorkspace(ctx context.Context, root string) (string, error) {
	hash := sha256.New()
	var bytes int64
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil || !isRegularCheckWorkspaceFile(info) {
			return fmt.Errorf("benchmark fixture contains a non-regular file")
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(hash, filepath.ToSlash(relative)+"\x00"+strconv.FormatInt(info.Size(), 10)+"\x00"); err != nil {
			return err
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		if err := validateCheckWorkspaceFile(path, info, input); err != nil {
			_ = input.Close()
			return err
		}
		copied, copyErr := copyCheckWorkspaceFile(ctx, hash, input, make([]byte, checkCopyBufferSize), checkLimits.maxWorkspaceBytes-bytes)
		postReadErr := validateCheckWorkspaceFile(path, info, input)
		closeErr := input.Close()
		if copyErr != nil {
			return copyErr
		}
		if postReadErr != nil {
			return postReadErr
		}
		if closeErr != nil {
			return closeErr
		}
		if copied != info.Size() {
			return fmt.Errorf("benchmark fixture changed while hashing")
		}
		bytes += copied
		return nil
	})
	if err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func (fixture benchmarkFixture) verifyCurrent(ctx context.Context, root string) error {
	current, err := benchmarkSourceFingerprint(ctx, root)
	if err != nil {
		return err
	}
	if current != fixture.fingerprint {
		return project.ErrRevisionConflict
	}
	return nil
}

func (s *Service) measureGoBenchmarkCopies(ctx context.Context, comparison *GoBenchmarkComparison, draft Draft, request GoBenchmarkComparisonRequest, choice GoBenchmarkChoice, fixture benchmarkFixture, baseWorkspace, candidateWorkspace string) (*GoBenchmarkComparison, error) {
	identity, err := newDraftRevisionIdentity(request.ID, request.ExpectedRevision, request.ExpectedHash)
	if err != nil {
		return nil, err
	}
	if err := s.verifyBenchmarkIdentity(ctx, identity, fixture); err != nil {
		if isBenchmarkContextError(err) {
			return canceledBenchmarkComparison(comparison), nil
		}
		return nil, err
	}
	base, state, err := s.runGoBenchmark(ctx, filepath.Join(baseWorkspace, filepath.Dir(filepath.FromSlash(draft.TargetPath))), choice, fixture.environment, draft.ProjectRevision)
	if err != nil {
		if state == "canceled" {
			return canceledBenchmarkComparison(comparison), nil
		}
		comparison.Status, comparison.Reason = state, "base benchmark did not produce the required samples"
		return comparison, nil
	}
	if err := s.verifyBenchmarkIdentity(ctx, identity, fixture); err != nil {
		if isBenchmarkContextError(err) {
			return canceledBenchmarkComparison(comparison), nil
		}
		return nil, err
	}
	candidate, state, err := s.runGoBenchmark(ctx, filepath.Join(candidateWorkspace, filepath.Dir(filepath.FromSlash(draft.TargetPath))), choice, fixture.environment, draft.ProjectRevision)
	if err != nil {
		if state == "canceled" {
			return canceledBenchmarkComparison(comparison), nil
		}
		comparison.Status, comparison.Reason = state, "candidate benchmark did not produce the required samples"
		return comparison, nil
	}
	if err := s.verifyBenchmarkIdentity(ctx, identity, fixture); err != nil {
		if isBenchmarkContextError(err) {
			return canceledBenchmarkComparison(comparison), nil
		}
		return nil, err
	}
	comparison.Base = &base
	comparison.Candidate = &candidate
	comparison.Status = "completed"
	return comparison, nil
}

func isBenchmarkContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func canceledBenchmarkComparison(comparison *GoBenchmarkComparison) *GoBenchmarkComparison {
	comparison.Status = "canceled"
	comparison.Reason = "benchmark comparison canceled"
	comparison.Base = nil
	comparison.Candidate = nil
	return comparison
}

func (s *Service) verifyBenchmarkIdentity(ctx context.Context, identity draftRevisionIdentity, fixture benchmarkFixture) error {
	if _, err := s.loadValidatedDraftForChecks(identity); err != nil {
		return err
	}
	return fixture.verifyCurrent(ctx, s.manager.Root())
}

func comparisonFromCatalog(catalog *GoBenchmarkCatalog, benchmark string, choice GoBenchmarkChoice) *GoBenchmarkComparison {
	return &GoBenchmarkComparison{DraftID: catalog.DraftID, DraftRevision: catalog.DraftRevision, DraftHash: catalog.DraftHash, ProjectID: catalog.ProjectID, ProjectRevision: catalog.ProjectRevision, BaseFileHash: catalog.BaseFileHash, TargetPath: catalog.TargetPath, Benchmark: benchmark, Scope: choice.Scope, Command: append([]string(nil), choice.Command...)}
}

func goBenchmarkCommand(benchmark string) []string {
	return []string{"go", "test", ".", "-run", "^$", "-bench", "^" + regexp.QuoteMeta(benchmark) + "$", "-count", strconv.Itoa(benchmarkSampleCount), "-benchtime", benchmarkDuration.String(), "-benchmem", "-timeout", benchmarkProcessTimeout.String()}
}

func benchmarkExecutionScope(draft Draft, benchmark string, command []string, fixtureFingerprint string) string {
	hash := sha256.Sum256([]byte(strings.Join([]string{draft.ProjectID, draft.ProjectRevision, draft.ID, strconv.FormatInt(draft.Revision, 10), draft.Hash, draft.BaseFileHash, draft.TargetPath, benchmark, strings.Join(command, "\x00"), fixtureFingerprint}, "\x00")))
	return "benchmark:" + hex.EncodeToString(hash[:16])
}

func (s *Service) runGoBenchmark(ctx context.Context, workspace string, choice GoBenchmarkChoice, environment []string, revision string) (GoBenchmarkMeasurement, string, error) {
	timed, cancel := context.WithTimeout(ctx, benchmarkCommandTimeout)
	defer cancel()
	result, err := s.benchmarkRunner.Run(timed, workspace, choice.Command, environment, revision)
	if timed.Err() != nil || ctx.Err() != nil {
		return GoBenchmarkMeasurement{}, "canceled", timed.Err()
	}
	if err != nil {
		return GoBenchmarkMeasurement{}, "failed", err
	}
	if result.truncated {
		return GoBenchmarkMeasurement{}, "failed", fmt.Errorf("benchmark output was truncated")
	}
	samples, err := parseGoBenchmarkSamples(result.output, choice.Name)
	if err != nil || len(samples) != benchmarkSampleCount {
		return GoBenchmarkMeasurement{}, "failed", err
	}
	return GoBenchmarkMeasurement{Samples: samples}, "completed", nil
}

func discoverGoBenchmarks(root, targetPath string) ([]string, error) {
	directory := filepath.Join(root, filepath.Dir(filepath.FromSlash(targetPath)))
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) > maxBenchmarkDiscoveryFiles {
		return nil, fmt.Errorf("benchmark package is unavailable")
	}
	counts := make(map[string]int)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		info, err := os.Lstat(path)
		if err != nil || !isRegularCheckWorkspaceFile(info) || info.Size() > maxBenchmarkSourceBytes {
			return nil, fmt.Errorf("benchmark source is unavailable")
		}
		source, err := readBenchmarkSource(path, info)
		if err != nil {
			return nil, err
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, source, 0)
		if err != nil {
			continue
		}
		imports := testingImports(file)
		for _, declaration := range file.Decls {
			if function, ok := declaration.(*ast.FuncDecl); ok && validGoBenchmark(function, imports) {
				counts[function.Name.Name]++
			}
		}
	}
	return uniqueSortedBenchmarks(counts), nil
}

func readBenchmarkSource(path string, expected os.FileInfo) ([]byte, error) {
	input, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer input.Close()
	if err := validateCheckWorkspaceFile(path, expected, input); err != nil {
		return nil, err
	}
	source, err := io.ReadAll(io.LimitReader(input, expected.Size()+1))
	if err != nil || int64(len(source)) != expected.Size() {
		return nil, fmt.Errorf("benchmark source changed while reading")
	}
	if err := validateCheckWorkspaceFile(path, expected, input); err != nil {
		return nil, err
	}
	if err := benchmarkSourceUnchanged(path, expected, input); err != nil {
		return nil, err
	}
	return source, nil
}

func benchmarkSourceUnchanged(path string, expected os.FileInfo, input *os.File) error {
	opened, err := input.Stat()
	if err != nil {
		return err
	}
	current, err := os.Lstat(path)
	if err != nil || !isRegularCheckWorkspaceFile(current) || !os.SameFile(expected, current) || opened.Size() != expected.Size() || current.Size() != expected.Size() || !opened.ModTime().Equal(expected.ModTime()) || !current.ModTime().Equal(expected.ModTime()) {
		return fmt.Errorf("benchmark source changed while reading")
	}
	return nil
}

func testingImports(file *ast.File) map[string]bool {
	imports := make(map[string]bool)
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil || path != "testing" || spec.Name != nil && (spec.Name.Name == "_" || spec.Name.Name == ".") {
			continue
		}
		name := "testing"
		if spec.Name != nil {
			name = spec.Name.Name
		}
		imports[name] = true
	}
	return imports
}

func uniqueSortedBenchmarks(counts map[string]int) []string {
	benchmarks := make([]string, 0, len(counts))
	for name, count := range counts {
		if count == 1 {
			benchmarks = append(benchmarks, name)
		}
	}
	sort.Strings(benchmarks)
	return benchmarks
}

func validGoBenchmark(function *ast.FuncDecl, imports map[string]bool) bool {
	return validBenchmarkFunctionName(function) && validBenchmarkParameters(function, imports)
}

func validBenchmarkFunctionName(function *ast.FuncDecl) bool {
	if function == nil || function.Recv != nil || function.Name == nil || function.Type == nil {
		return false
	}
	name := function.Name.Name
	return strings.HasPrefix(name, "Benchmark") && len(name) > len("Benchmark") && ast.IsExported(name) && !unicode.IsLower(benchmarkSuffixRune(name))
}

func benchmarkSuffixRune(name string) rune {
	runeValue, _ := utf8.DecodeRuneInString(strings.TrimPrefix(name, "Benchmark"))
	return runeValue
}

func validBenchmarkParameters(function *ast.FuncDecl, imports map[string]bool) bool {
	if function.Type.TypeParams != nil || function.Type.Results != nil || function.Type.Params == nil || len(function.Type.Params.List) != 1 {
		return false
	}
	if len(function.Type.Params.List[0].Names) > 1 {
		return false
	}
	parameter, ok := function.Type.Params.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	selector, ok := parameter.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	qualifier, ok := selector.X.(*ast.Ident)
	return ok && imports[qualifier.Name] && selector.Sel.Name == "B"
}

func parseGoBenchmarkSamples(output, benchmark string) ([]GoBenchmarkSample, error) {
	samples := make([]GoBenchmarkSample, 0, benchmarkSampleCount)
	for _, line := range strings.Split(output, "\n") {
		selected, invalidName := selectedBenchmarkLine(line, benchmark)
		if !selected {
			continue
		}
		if invalidName {
			return nil, fmt.Errorf("unsupported selected benchmark result name")
		}
		sample, err := parseGoBenchmarkSample(line)
		if err != nil {
			return nil, err
		}
		samples = append(samples, sample)
		if len(samples) > benchmarkSampleCount {
			return nil, fmt.Errorf("benchmark returned extra samples")
		}
	}
	if len(samples) != benchmarkSampleCount {
		return nil, fmt.Errorf("benchmark returned %d samples, want %d", len(samples), benchmarkSampleCount)
	}
	return samples, nil
}

func selectedBenchmarkLine(line, benchmark string) (bool, bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 || !strings.HasPrefix(fields[0], benchmark) {
		return false, false
	}
	suffix := strings.TrimPrefix(fields[0], benchmark)
	if suffix == "" || validBenchmarkCPUSuffix(suffix) {
		return true, false
	}
	if strings.HasPrefix(suffix, "/") || strings.HasPrefix(suffix, "-") {
		return true, true
	}
	return false, false
}

func validBenchmarkCPUSuffix(suffix string) bool {
	if !benchmarkCPUSuffix.MatchString(suffix) {
		return false
	}
	value, err := strconv.Atoi(strings.TrimPrefix(suffix, "-"))
	return err == nil && value > 0
}

func parseGoBenchmarkSample(line string) (GoBenchmarkSample, error) {
	fields := strings.Fields(line)
	if len(fields) < 8 || len(fields)%2 != 0 {
		return GoBenchmarkSample{}, fmt.Errorf("malformed benchmark result")
	}
	iterations, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil || iterations <= 0 {
		return GoBenchmarkSample{}, fmt.Errorf("invalid benchmark iterations")
	}
	nanoseconds, bytes, allocations, err := parseGoBenchmarkMetrics(fields[2:])
	if err != nil {
		return GoBenchmarkSample{}, err
	}
	return GoBenchmarkSample{Iterations: iterations, Nanoseconds: nanoseconds, Bytes: &bytes, Allocations: &allocations}, nil
}

func parseGoBenchmarkMetrics(fields []string) (float64, int64, int64, error) {
	var nanoseconds float64
	var bytes, allocations int64
	seen := make(map[string]bool, len(fields)/2)
	for index := 0; index < len(fields); index += 2 {
		value, unit := fields[index], fields[index+1]
		if seen[unit] {
			return 0, 0, 0, fmt.Errorf("duplicate benchmark metric %q", unit)
		}
		seen[unit] = true
		metric, err := parseFiniteBenchmarkMetric(value, unit)
		if err != nil {
			return 0, 0, 0, err
		}
		switch unit {
		case "ns/op":
			if metric <= 0 {
				return 0, 0, 0, fmt.Errorf("invalid benchmark ns/op")
			}
			nanoseconds = metric
		case "B/op":
			parsed, err := parseNonnegativeBenchmarkInteger(value)
			if err != nil {
				return 0, 0, 0, fmt.Errorf("invalid benchmark bytes/op")
			}
			bytes = parsed
		case "allocs/op":
			parsed, err := parseNonnegativeBenchmarkInteger(value)
			if err != nil {
				return 0, 0, 0, fmt.Errorf("invalid benchmark allocs/op")
			}
			allocations = parsed
		}
	}
	if !seen["ns/op"] || !seen["B/op"] || !seen["allocs/op"] {
		return 0, 0, 0, fmt.Errorf("benchmark result is missing required metrics")
	}
	return nanoseconds, bytes, allocations, nil
}

func parseFiniteBenchmarkMetric(value, unit string) (float64, error) {
	metric, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(metric) || math.IsInf(metric, 0) || math.Signbit(metric) {
		return 0, fmt.Errorf("invalid benchmark metric %q", unit)
	}
	return metric, nil
}

func parseNonnegativeBenchmarkInteger(value string) (int64, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("invalid nonnegative integer")
	}
	return parsed, nil
}
