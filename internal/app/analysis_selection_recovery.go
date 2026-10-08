package app

import (
	"fmt"
	"strings"
)

// AnalysisRecoveryState says whether recover_incomplete can finish saved-selection work.
type AnalysisRecoveryState string

const (
	AnalysisRecoveryRunning      AnalysisRecoveryState = "running"
	AnalysisRecoveryContinuation AnalysisRecoveryState = "continuation"
	AnalysisRecoveryNotStarted   AnalysisRecoveryState = "not_started"
	AnalysisRecoveryBlocked      AnalysisRecoveryState = "blocked"
	AnalysisRecoveryAvailable    AnalysisRecoveryState = "available"
	AnalysisRecoveryComplete     AnalysisRecoveryState = "complete"
)

// AnalysisRecoverySummary is read-only eligibility; it never dispatches or admits work.
// Counts cover selected eligible files and stages that recovery would reattempt under
// the evidence run's models, whatever the state.
type AnalysisRecoverySummary struct {
	State      AnalysisRecoveryState `json:"state"`
	FileCount  int                   `json:"file_count"`
	StageCount int                   `json:"stage_count"`
	Reason     string                `json:"reason,omitempty"`
}

// analysisFileRecovery is one eligible file's contribution to the summary.
type analysisFileRecovery struct {
	recoverable             int
	sourceBlocked           bool
	codeModelMissing        bool
	performanceModelMissing bool
	securityModelMissing    bool
}

type analysisRecoveryEvidence struct {
	projectID string
	models    *AnalysisModels
	files     map[string]analysisFileRecovery
}

// Changed or unreadable sources block the whole file: retrying cannot use them.
func (s *Service) classifyAnalysisFileRecovery(stages []AnalysisFileStageStatus, sourceCurrent bool, models *AnalysisModels) analysisFileRecovery {
	if !sourceCurrent {
		return analysisFileRecovery{sourceBlocked: true}
	}
	var result analysisFileRecovery
	for _, stage := range stages {
		switch analysisStageRecoveryFor(stage.Status, s.analysisStageConfigured(stage.Stage, models)) {
		case analysisStageRecoverable:
			result.recoverable++
		case analysisStageNeedsModel:
			if stage.Stage == AnalysisStageSemantic {
				result.codeModelMissing = true
			} else if stage.Stage == AnalysisStagePerformance {
				result.performanceModelMissing = true
			} else {
				result.securityModelMissing = true
			}
		}
	}
	return result
}

// Active and continuable runs take precedence, matching start admission. Saved
// selection exclusions are not eligible work.
func (s *Service) analysisRecoverySummaryLocked(evidence analysisRecoveryEvidence, excluded []string) AnalysisRecoverySummary {
	summary, total := summarizeAnalysisRecoveryFiles(evidence.files, excluded)
	c := s.analysisRun
	var status AnalysisRunStatus
	if c.run != nil && c.run.Identity.ProjectID == evidence.projectID {
		status = c.run.Status
	}
	switch {
	case isAnalysisRunRunning(c.done, status):
		summary.State, summary.Reason = AnalysisRecoveryRunning, "An analysis run is in progress. Wait for it, or pause or cancel it, before repairing analysis."
	case isAnalysisRunPaused(c.fault, status):
		summary.State, summary.Reason = AnalysisRecoveryContinuation, "The last run is paused or interrupted. Continue or cancel it from its run page."
	case status == "":
		summary.State = AnalysisRecoveryNotStarted
	case total.sourceBlocked:
		summary.State, summary.Reason = AnalysisRecoveryBlocked, "A selected file changed or could not be read since indexing. Refresh project files, then repair analysis."
	case summary.StageCount > 0:
		summary.State = AnalysisRecoveryAvailable
	case total.codeModelMissing || total.performanceModelMissing || total.securityModelMissing:
		summary.State, summary.Reason = AnalysisRecoveryBlocked, analysisRecoveryModelReason(total, evidence.models)
	default:
		summary.State = AnalysisRecoveryComplete
	}
	return summary
}

func isAnalysisRunRunning(done chan struct{}, status AnalysisRunStatus) bool {
	return done != nil || status == AnalysisRunRunning || status == AnalysisRunQueued || status == AnalysisRunPausing || status == AnalysisRunCanceling
}

func isAnalysisRunPaused(fault error, status AnalysisRunStatus) bool {
	return fault != nil || status == AnalysisRunPaused || status == AnalysisRunInterrupted
}

func summarizeAnalysisRecoveryFiles(files map[string]analysisFileRecovery, excluded []string) (AnalysisRecoverySummary, analysisFileRecovery) {
	ignored := make(map[string]bool, len(excluded))
	for _, path := range excluded {
		ignored[path] = true
	}
	var summary AnalysisRecoverySummary
	var total analysisFileRecovery
	for path, file := range files {
		if ignored[path] {
			continue
		}
		if file.recoverable > 0 {
			summary.FileCount++
			summary.StageCount += file.recoverable
		}
		total.sourceBlocked = total.sourceBlocked || file.sourceBlocked
		total.codeModelMissing = total.codeModelMissing || file.codeModelMissing
		total.performanceModelMissing = total.performanceModelMissing || file.performanceModelMissing
		total.securityModelMissing = total.securityModelMissing || file.securityModelMissing
	}
	return summary, total
}

func analysisRecoveryModelReason(missing analysisFileRecovery, models *AnalysisModels) string {
	needs := []string{}
	if missing.codeModelMissing {
		needs = append(needs, fmt.Sprintf("the %s model profile used for Code analysis", analysisStageProfile(AnalysisStageSemantic, models)))
	}
	if missing.performanceModelMissing {
		needs = append(needs, fmt.Sprintf("the %s model profile used for Performance", analysisStageProfile(AnalysisStagePerformance, models)))
	}
	if missing.securityModelMissing {
		needs = append(needs, fmt.Sprintf("the %s model profile used for Security", analysisStageProfile(AnalysisStageSecurityAI, models)))
	}
	return "Configure " + strings.Join(needs, " and ") + " to complete the remaining analysis, or prepare a new analysis with configured models."
}
