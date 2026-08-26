package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
	CompletedAt     time.Time     `json:"completed_at,omitempty"`
	Phases          []GoScanPhase `json:"phases"`
}

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
	index, err := s.manager.Index()
	if err != nil {
		return nil, err
	}
	report := &GoScanReport{ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, Status: "running", StartedAt: time.Now().UTC(), Phases: []GoScanPhase{}}
	workspace, err := os.MkdirTemp("", "mini-orca-scan-")
	if err != nil {
		return nil, fmt.Errorf("create scan workspace: %w", err)
	}
	defer os.RemoveAll(workspace)
	if err := copyCandidateWorkspace(s.manager.Root(), workspace); err != nil {
		return nil, err
	}
	fileHashes := make(map[string]string, len(index.Files))
	for _, file := range index.Files {
		fileHashes[file.Path] = file.ContentHash
	}
	findings := parserScanFindings(index)
	report.Phases = append(report.Phases, GoScanPhase{Name: "parse", State: scanPhaseState(findings)})
	if err := s.finishGoScan(report, findings, fileHashes); err != nil {
		return nil, err
	}
	for _, phase := range []struct {
		name    string
		command []string
		source  string
	}{{"vet", []string{"go", "vet", "./..."}, project.FindingSourceVet}, {"tests", []string{"go", "test", "./..."}, project.FindingSourceTest}} {
		result := s.runGoScanPhase(ctx, workspace, phase.name, phase.command)
		report.Phases = append(report.Phases, result)
		findings = append(findings, toolScanFindings(result, phase.source, fileHashes)...)
		if err := s.finishGoScan(report, findings, fileHashes); err != nil {
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
	if err := s.finishGoScan(report, findings, fileHashes); err != nil {
		return nil, err
	}
	return cloneGoScanReport(report), nil
}

func (s *Service) runGoScanPhase(ctx context.Context, workspace, name string, command []string) GoScanPhase {
	phase := GoScanPhase{Name: name, Command: append([]string(nil), command...)}
	timed, cancel := context.WithTimeout(ctx, s.focusedCheckTimeout)
	defer cancel()
	output, exitCode, err := runCheckCommand(timed, workspace, command)
	phase.Output = sanitizeCheckOutput(output, workspace, s.manager.Root())
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

func (s *Service) finishGoScan(report *GoScanReport, findings []project.UnifiedFinding, hashes map[string]string) error {
	store, err := project.NewFindingStore(s.manager.Root())
	if err != nil {
		return err
	}
	if _, err := store.Reconcile(project.FindingInput{ProjectID: report.ProjectID, ProjectRevision: report.ProjectRevision, FileHashes: hashes}, findings); err != nil {
		return err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return writeGoScanReport(s.manager.Root(), data)
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
	copy := *source
	copy.Phases = append([]GoScanPhase(nil), source.Phases...)
	return &copy
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
