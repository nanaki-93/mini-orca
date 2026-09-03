package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

const goScanPath = ".mini-orca/scans/go.json"

type GoScanPhase struct {
	Name     string   `json:"name"`
	State    string   `json:"state"`
	Command  []string `json:"command,omitempty"`
	Output   string   `json:"output,omitempty"`
	ExitCode int      `json:"exit_code,omitempty"`
}

// GoScanReport is an explicit, source-free record of isolated verification.
type GoScanReport struct {
	ProjectID       string        `json:"project_id"`
	ProjectRevision string        `json:"project_revision"`
	Status          string        `json:"status"`
	StartedAt       time.Time     `json:"started_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	CompletedAt     time.Time     `json:"completed_at,omitempty"`
	Phases          []GoScanPhase `json:"phases"`
}

type goScanController struct {
	mu     sync.Mutex
	report *GoScanReport
	cancel context.CancelFunc
}

func newGoScanController() *goScanController { return &goScanController{} }

// ScanGoProject performs parser, vet, and test phases only after an explicit
// request for the active revision. Tool commands run in a copied workspace.
func (s *Service) ScanGoProject(ctx context.Context, revision string) (*GoScanReport, error) {
	analysis, err := s.manager.Analysis()
	if err != nil {
		return nil, err
	}
	if revision == "" || revision != analysis.ProjectRevision {
		return nil, project.ErrRevisionConflict
	}
	root := analysis.Path
	index, err := s.manager.Index()
	if err != nil {
		return nil, err
	}
	if index.ProjectID != analysis.ProjectID || index.ProjectRevision != analysis.ProjectRevision {
		return nil, project.ErrRevisionConflict
	}
	now := time.Now().UTC()
	report := &GoScanReport{ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, Status: "running", StartedAt: now, UpdatedAt: now, Phases: []GoScanPhase{}}
	workspace, err := os.MkdirTemp("", "mini-orca-scan-")
	if err != nil {
		return nil, fmt.Errorf("create scan workspace: %w", err)
	}
	defer os.RemoveAll(workspace)
	if err := copyCheckWorkspace(root, workspace); err != nil {
		return nil, err
	}
	fileHashes := make(map[string]string, len(index.Files))
	for _, file := range index.Files {
		fileHashes[file.Path] = file.ContentHash
	}
	findings := parserScanFindings(index)
	report.Phases = append(report.Phases, GoScanPhase{Name: "parse", State: scanPhaseState(findings)})
	if err := s.finishGoScan(root, report, findings, fileHashes, project.FindingSourceParser); err != nil {
		return nil, err
	}
	for _, phase := range []struct {
		name    string
		command []string
		source  string
	}{{"vet", []string{"go", "vet", "./..."}, project.FindingSourceVet}, {"tests", []string{"go", "test", "./..."}, project.FindingSourceTest}} {
		result := s.runGoScanPhase(ctx, workspace, root, phase.name, phase.command)
		report.Phases = append(report.Phases, result)
		findings = append(findings, toolScanFindings(result, phase.source, fileHashes)...)
		if err := s.finishGoScan(root, report, findings, fileHashes, phase.source); err != nil {
			return nil, err
		}
		if result.State == CheckCanceled {
			break
		}
	}
	report.Status = "completed"
	if ctx.Err() != nil {
		report.Status = "canceled"
	}
	report.CompletedAt = time.Now().UTC()
	if err := s.finishGoScan(root, report, findings, fileHashes); err != nil {
		return nil, err
	}
	return cloneGoScanReport(report), nil
}

// StartGoScan starts one explicit isolated scan for the active revision and
// returns immediately with source-free progress. Repeated calls reuse the
// active scan instead of running verification commands concurrently.
func (s *Service) StartGoScan(revision string) (*GoScanReport, error) {
	analysis, err := s.manager.Analysis()
	if err != nil {
		return nil, err
	}
	if revision == "" || revision != analysis.ProjectRevision {
		return nil, project.ErrRevisionConflict
	}
	s.goScan.mu.Lock()
	defer s.goScan.mu.Unlock()
	if s.goScan.cancel != nil && s.goScan.report != nil && s.goScan.report.ProjectID == analysis.ProjectID && s.goScan.report.ProjectRevision == revision {
		return cloneGoScanReport(s.goScan.report), nil
	}
	now := time.Now().UTC()
	report := &GoScanReport{ProjectID: analysis.ProjectID, ProjectRevision: revision, Status: "running", StartedAt: now, UpdatedAt: now, Phases: []GoScanPhase{}}
	data, err := marshalGoScanReport(report)
	if err != nil {
		return nil, err
	}
	if err := writeGoScanReport(analysis.Path, data); err != nil {
		return nil, err
	}
	worker, cancel := context.WithCancel(context.Background())
	s.goScan.report = cloneGoScanReport(report)
	s.goScan.cancel = cancel
	go s.runStartedGoScan(worker, report.ProjectID, revision)
	return cloneGoScanReport(report), nil
}

// GoScanProgress returns the report for the requested active revision. A
// missing or older report is represented by nil so clients can distinguish it
// from an active scan.
func (s *Service) GoScanProgress(revision string) (*GoScanReport, error) {
	analysis, err := s.manager.Analysis()
	if err != nil {
		return nil, err
	}
	if revision == "" || revision != analysis.ProjectRevision {
		return nil, project.ErrRevisionConflict
	}
	report, err := loadGoScanReport(analysis.Path)
	if err != nil {
		return nil, err
	}
	if report == nil || report.ProjectID != analysis.ProjectID || report.ProjectRevision != revision {
		return nil, nil
	}
	return report, nil
}

// CancelGoScan stops the active isolated scan. Completed phase results remain
// available in the persisted progress report.
func (s *Service) CancelGoScan(revision string) (*GoScanReport, error) {
	analysis, err := s.manager.Analysis()
	if err != nil {
		return nil, err
	}
	if revision == "" || revision != analysis.ProjectRevision {
		return nil, project.ErrRevisionConflict
	}
	s.goScan.mu.Lock()
	if s.goScan.cancel == nil || s.goScan.report == nil || s.goScan.report.ProjectID != analysis.ProjectID || s.goScan.report.ProjectRevision != revision {
		s.goScan.mu.Unlock()
		return nil, fmt.Errorf("verified scan is not running")
	}
	cancel := s.goScan.cancel
	s.goScan.mu.Unlock()
	cancel()
	return s.GoScanProgress(revision)
}

func (s *Service) runStartedGoScan(ctx context.Context, projectID, revision string) {
	report, err := s.ScanGoProject(ctx, revision)
	if err != nil {
		now := time.Now().UTC()
		report = &GoScanReport{ProjectID: projectID, ProjectRevision: revision, Status: "failed", StartedAt: now, UpdatedAt: now, CompletedAt: now, Phases: []GoScanPhase{}}
		if active, activeErr := s.manager.Analysis(); activeErr == nil && active.ProjectID == projectID && active.ProjectRevision == revision {
			if data, marshalErr := marshalGoScanReport(report); marshalErr == nil {
				_ = writeGoScanReport(active.Path, data)
			}
		}
	}
	s.goScan.mu.Lock()
	defer s.goScan.mu.Unlock()
	if s.goScan.report == nil || s.goScan.report.ProjectID != projectID || s.goScan.report.ProjectRevision != revision {
		return
	}
	s.goScan.report = cloneGoScanReport(report)
	s.goScan.cancel = nil
}

func (s *Service) runGoScanPhase(ctx context.Context, workspace, root, name string, command []string) GoScanPhase {
	phase := GoScanPhase{Name: name, Command: append([]string(nil), command...)}
	timed, cancel := context.WithTimeout(ctx, s.focusedCheckTimeout)
	defer cancel()
	output, exitCode, err := runCheckCommand(timed, workspace, command)
	phase.Output = sanitizeCheckOutput(output, workspace, root)
	phase.ExitCode = exitCode
	if timed.Err() != nil {
		phase.State = CheckCanceled
		phase.Output = timed.Err().Error()
	} else if err != nil {
		phase.State = CheckFailed
	} else {
		phase.State = CheckPassed
	}
	return phase
}

func (s *Service) finishGoScan(root string, report *GoScanReport, findings []project.UnifiedFinding, hashes map[string]string, sources ...string) error {
	store, err := project.NewFindingStore(root)
	if err != nil {
		return err
	}
	input := project.FindingInput{ProjectID: report.ProjectID, ProjectRevision: report.ProjectRevision, FileHashes: hashes}
	for _, source := range sources {
		matching := make([]project.UnifiedFinding, 0, len(findings))
		for _, finding := range findings {
			if finding.Source == source {
				matching = append(matching, finding)
			}
		}
		if _, err := store.ReconcileSource(input, source, matching); err != nil {
			return err
		}
	}
	report.UpdatedAt = time.Now().UTC()
	data, err := marshalGoScanReport(report)
	if err != nil {
		return err
	}
	return writeGoScanReport(root, data)
}

func parserScanFindings(index *project.ProjectIndex) []project.UnifiedFinding {
	findings := []project.UnifiedFinding{}
	for _, file := range index.Files {
		for _, diagnostic := range file.Diagnostics {
			findings = append(findings, project.UnifiedFinding{Source: project.FindingSourceParser, Confidence: project.FindingConfidenceToolReported, Severity: "high", Title: "Parser diagnostic", Message: diagnostic.Message, Rule: "parse", FileHash: file.ContentHash, Location: project.FindingLocation{Path: file.Path, StartLine: diagnostic.Line, EndLine: diagnostic.Line}, Evidence: diagnostic.Message})
		}
	}
	return findings
}

func scanPhaseState(findings []project.UnifiedFinding) string {
	if len(findings) == 0 {
		return CheckPassed
	}
	return CheckFailed
}

var scanLocation = regexp.MustCompile(`(?m)([^\s:]+\.go):(\d+)(?::\d+)?:\s*([^\n]+)`)

func toolScanFindings(phase GoScanPhase, source string, hashes map[string]string) []project.UnifiedFinding {
	if phase.State == CheckPassed || phase.State == CheckCanceled {
		return nil
	}
	message := strings.TrimSpace(phase.Output)
	location := project.FindingLocation{}
	fileHash := ""
	if match := scanLocation.FindStringSubmatch(message); len(match) == 4 {
		location.Path = filepath.ToSlash(match[1])
		fmt.Sscan(match[2], &location.StartLine)
		location.EndLine = location.StartLine
		message = match[3]
		fileHash = hashes[location.Path]
	}
	if message == "" {
		message = phase.Name + " failed"
	}
	return []project.UnifiedFinding{{Source: source, Confidence: project.FindingConfidenceToolReported, Severity: "high", Title: "Go " + phase.Name + " failure", Message: message, Rule: phase.Name, FileHash: fileHash, Location: location, Evidence: phase.Output}}
}

func cloneGoScanReport(source *GoScanReport) *GoScanReport {
	if source == nil {
		return nil
	}
	copy := *source
	copy.Phases = append([]GoScanPhase(nil), source.Phases...)
	return &copy
}

func loadGoScanReport(root string) (*GoScanReport, error) {
	data, err := os.ReadFile(filepath.Join(root, goScanPath))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read go scan report: %w", err)
	}
	var report GoScanReport
	if err := json.Unmarshal(data, &report); err != nil || report.ProjectID == "" || report.ProjectRevision == "" || report.Status == "" || report.StartedAt.IsZero() {
		return nil, fmt.Errorf("parse go scan report")
	}
	return cloneGoScanReport(&report), nil
}

func marshalGoScanReport(report *GoScanReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

func writeGoScanReport(root string, data []byte) error {
	path := filepath.Join(root, goScanPath)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create scan directory: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".go-scan-*.tmp")
	if err != nil {
		return fmt.Errorf("create scan temp file: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write scan: %w", err)
	}
	if err := temp.Chmod(0600); err != nil {
		temp.Close()
		return fmt.Errorf("set scan permissions: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close scan: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace scan: %w", err)
	}
	return nil
}
