package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/storage"
)

const analysisRunRelativePath = ".mini-orca/analysis/run.json"
const maxAnalysisRunMetadataBytes = 64 * 1024 * 1024

var errAnalysisRunCorrupt = errors.New("saved analysis progress is invalid; preserved for explicit recovery")

func cloneAnalysisPreview(plan AnalysisRunPreview) AnalysisRunPreview {
	copy := plan
	copy.Files = append([]AnalysisPlannedFile{}, plan.Files...)
	for i := range copy.Files {
		copy.Files[i].Stages = append([]AnalysisStagePlan{}, plan.Files[i].Stages...)
	}
	copy.Excluded = append([]AnalysisExcludedFile{}, plan.Excluded...)
	copy.Providers = append([]AnalysisProviderRequirement{}, plan.Providers...)
	for i := range copy.Providers {
		copy.Providers[i].Stages = append([]AnalysisStage{}, plan.Providers[i].Stages...)
	}
	return copy
}

func cloneAnalysisRun(run *AnalysisRun) *AnalysisRun {
	if run == nil {
		return nil
	}
	copy := *run
	copy.Plan = cloneAnalysisPreview(run.Plan)
	copy.Files = append([]AnalysisRunFile{}, run.Files...)
	for i := range copy.Files {
		copy.Files[i].Stages = append([]AnalysisStageProgress{}, run.Files[i].Stages...)
		for j := range copy.Files[i].Stages {
			if count := copy.Files[i].Stages[j].FindingCount; count != nil {
				n := *count
				copy.Files[i].Stages[j].FindingCount = &n
			}
		}
	}
	copy.Sections = append([]AnalysisSectionProgress{}, run.Sections...)
	for i := range copy.Sections {
		if count := copy.Sections[i].FindingCount; count != nil {
			n := *count
			copy.Sections[i].FindingCount = &n
		}
	}
	return &copy
}

func loadAnalysisRun(root string) (*AnalysisRun, error) {
	file, err := os.Open(filepath.Join(root, analysisRunRelativePath))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read analysis progress: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxAnalysisRunMetadataBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxAnalysisRunMetadataBytes {
		return nil, errAnalysisRunCorrupt
	}
	var run AnalysisRun
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&run); err != nil {
		return nil, errAnalysisRunCorrupt
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, errAnalysisRunCorrupt
	}
	if err := validateStoredAnalysisRun(&run); err != nil {
		return nil, errAnalysisRunCorrupt
	}
	return &run, nil
}

func (s *Service) restoreAnalysisRunLocked() error {
	c := s.analysisRun
	root := s.manager.Root()
	if root == "" {
		return nil
	}
	if c.root == root && c.run != nil {
		return c.fault
	}
	if c.cancel != nil {
		c.cancel()
	}
	c.run = nil
	c.fault = nil
	c.root = root
	c.windowStart = time.Time{}
	run, err := loadAnalysisRun(root)
	if err != nil {
		return err
	}
	c.run = run
	if run == nil {
		return nil
	}
	switch run.Status {
	case AnalysisRunQueued, AnalysisRunRunning, AnalysisRunPausing:
		run.Status = AnalysisRunInterrupted
		run.Reason = "Analysis was interrupted; preview and resume explicitly."
		interruptAnalysisStages(run, AnalysisStageInterrupted)
		return s.saveAnalysisRunLocked(false)
	case AnalysisRunCanceling:
		run.Status = AnalysisRunCanceled
		interruptAnalysisStages(run, AnalysisStageCanceled)
		return s.saveAnalysisRunLocked(false)
	}
	return nil
}

func (s *Service) saveAnalysisRunLocked(recoverFault bool) error {
	c := s.analysisRun
	if c.fault != nil && !recoverFault {
		return errAnalysisRunPersistence
	}
	s.updateAnalysisElapsedLocked()
	c.run.UpdatedAt = time.Now().UTC()
	refreshAnalysisSections(c.run)
	err := validateStoredAnalysisRun(c.run)
	var data []byte
	if err == nil {
		data, err = json.MarshalIndent(c.run, "", "  ")
	}
	if err == nil && len(data) > maxAnalysisRunMetadataBytes {
		err = fmt.Errorf("analysis progress exceeds metadata size limit")
	}
	if err == nil {
		write := s.writeAnalysisRun
		if write == nil {
			write = storage.WriteFile
		}
		err = write(filepath.Join(c.root, analysisRunRelativePath), data, 0600)
	}
	if err != nil {
		s.failAnalysisPersistenceLocked()
		return errAnalysisRunPersistence
	}
	c.fault = nil
	return nil
}

func (s *Service) failAnalysisPersistenceLocked() {
	c := s.analysisRun
	c.fault = errAnalysisRunPersistence
	if c.run.Status != AnalysisRunStale {
		c.run.Status = AnalysisRunInterrupted
	}
	c.run.Reason = "Analysis progress could not be saved; resume or cancel to recover."
	interruptAnalysisStages(c.run, AnalysisStageInterrupted)
	if c.cancel != nil {
		c.cancel()
	}
}

func analysisMetadataPath(value string) bool {
	return value != "" && len(value) <= 4096 && path.Clean(value) == value && !path.IsAbs(value) && value != ".." && !strings.HasPrefix(value, "../") && !strings.ContainsAny(value, "\\\x00\r\n")
}

func analysisMetadataReason(value string) bool {
	switch value {
	case "", "Excluded by source policy.", "Not a supported text source file.", "Source exceeds analyzer size limits.", "Not eligible for semantic source analysis.",
		"Passive security rules require a Go source file.", "The model for this stage is not configured.",
		"Analysis stage could not complete.", "The stage needs an additional attempt allowance.", "The file is not eligible for this source analysis.",
		"The report contains incomplete evidence; review its details.", "The model request or response failed. Other analysis results remain available.",
		"Analysis canceled by the user.", "Analysis paused by the user.", "The batch limit was reached; resume to continue pending files.",
		"The stage exhausted its total attempt allowance.", "The dispatch allowance ended; preview and resume the remaining work.",
		"Analysis stopped before the stage completed; review and resume.", "Project source, policy or provider identity changed; start a new analysis.",
		"Analysis progress could not be saved; resume or cancel to recover.", "Analysis was interrupted; preview and resume explicitly.":
		return true
	}
	return false
}

func validateStoredAnalysisRun(run *AnalysisRun) error {
	invalid := func() error { return errAnalysisRunCorrupt }
	if run == nil || run.SchemaVersion != AnalysisRunSchemaVersion || run.Identity.Validate() != nil || !run.Status.Valid() || run.Plan.SchemaVersion != AnalysisRunSchemaVersion || run.Plan.Scope != AnalysisRunScopeProject || run.Plan.Identity != run.Identity.AnalysisQueueIdentity || run.Plan.Limits.Validate() != nil || run.Plan.PreviewID == "" || !analysisMetadataReason(run.Reason) {
		return invalid()
	}
	queue, err := analysisQueueFingerprint(&run.Plan)
	if err != nil || queue != run.Identity.QueueID {
		return invalid()
	}
	if run.CreatedAt.IsZero() || run.UpdatedAt.Before(run.CreatedAt) || run.ElapsedSeconds < 0 || run.WindowElapsedSeconds < 0 || run.WindowElapsedSeconds > int64(run.Plan.Limits.BudgetSeconds) || run.ElapsedSeconds < run.WindowElapsedSeconds || run.WindowFilesCompleted < 0 || run.WindowFilesCompleted > run.Plan.Limits.BatchFiles || len(run.Files) != len(run.Plan.Files) || len(run.Sections) != 3 {
		return invalid()
	}
	totalFindings := 0
	for i, file := range run.Files {
		planned := run.Plan.Files[i]
		if file.AnalysisFileIdentity != planned.AnalysisFileIdentity || !analysisMetadataPath(file.Path) || file.ContentHash == "" || file.Language == "" || planned.SizeBytes < 0 || len(file.Stages) != len(analysisStages) || len(planned.Stages) != len(analysisStages) || i > 0 && run.Files[i-1].Path >= file.Path {
			return invalid()
		}
		for j, stage := range file.Stages {
			plan := planned.Stages[j]
			if stage.Stage != analysisStages[j] || plan.Stage != stage.Stage || stage.Attempts < 0 || stage.Attempts > run.Plan.Limits.MaxAttemptsPerStage || stage.Stage == AnalysisStageSecurityRules && stage.Attempts != 0 || !analysisMetadataReason(stage.Reason) || !analysisMetadataReason(plan.Reason) || plan.MaxModelRequests < 0 || plan.MaxModelRequests > run.Plan.Limits.MaxAttemptsPerStage {
				return invalid()
			}
			evidence := false
			switch stage.Status {
			case AnalysisStageCompleted, AnalysisStageCompletedEmpty, AnalysisStagePartial:
				evidence = true
			case AnalysisStagePending, AnalysisStageRunning, AnalysisStageFailed, AnalysisStageSkipped, AnalysisStageUnavailable, AnalysisStageCanceled, AnalysisStageInterrupted, AnalysisStageStale:
			default:
				return invalid()
			}
			if evidence != (stage.FindingCount != nil) || !evidence && (stage.Cached || stage.ReportID != "") {
				return invalid()
			}
			if evidence {
				if *stage.FindingCount < 0 || *stage.FindingCount > 32 || len(stage.ReportID) != 64 || stage.Status == AnalysisStageCompletedEmpty && *stage.FindingCount != 0 || stage.Status == AnalysisStageCompleted && *stage.FindingCount == 0 {
					return invalid()
				}
				totalFindings += *stage.FindingCount
			}
		}
	}
	for _, file := range run.Plan.Excluded {
		if !analysisMetadataPath(file.Path) || !analysisMetadataReason(file.Reason) {
			return invalid()
		}
	}
	recomputed := cloneAnalysisRun(run)
	refreshAnalysisSections(recomputed)
	if !reflect.DeepEqual(run.Sections, recomputed.Sections) {
		return invalid()
	}
	sectionFindings := 0
	for _, section := range run.Sections {
		if section.Validate() != nil {
			return invalid()
		}
		if section.FindingCount != nil {
			sectionFindings += *section.FindingCount
		}
	}
	if totalFindings != sectionFindings {
		return invalid()
	}
	switch run.Status {
	case AnalysisRunCompleted, AnalysisRunCompletedEmpty, AnalysisRunPartial, AnalysisRunFailed, AnalysisRunUnavailable:
		if analysisFinishedStatus(run) != run.Status {
			return invalid()
		}
	}
	return nil
}
