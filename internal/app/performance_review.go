package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type performanceReviewSnapshot struct {
	runtime       modelRuntime
	root          string
	analysis      project.Analysis
	file          project.IndexFile
	source        string
	policyVersion string
}

func (s *Service) preparePerformanceReview(path string) (performanceReviewSnapshot, error) {
	file, err := s.manager.IndexedFile(path)
	if err != nil {
		return performanceReviewSnapshot{}, err
	}
	root := s.manager.Root()
	policy, err := project.NewContextPolicy(root)
	if err != nil {
		return performanceReviewSnapshot{}, err
	}
	if decision := policy.Decide(file.Path); !decision.Include {
		return performanceReviewSnapshot{}, fmt.Errorf("performance review path is not eligible: %s", decision.Reason)
	}
	info, err := project.GetFileInfo(root, file.Path)
	if err != nil {
		return performanceReviewSnapshot{}, err
	}
	if info.ContentHash != file.ContentHash {
		return performanceReviewSnapshot{}, project.ErrRevisionConflict
	}
	if len(info.Content) > project.PerformanceMaxSourceBytes {
		return performanceReviewSnapshot{}, fmt.Errorf("performance review skipped: source exceeds 64 KiB")
	}
	analysis, err := s.manager.Analysis()
	if err != nil {
		return performanceReviewSnapshot{}, err
	}
	return performanceReviewSnapshot{runtime: s.runtimes.analyze, root: root, analysis: *analysis, file: *file, source: info.Content, policyVersion: policy.Version()}, nil
}

func (s *Service) requestPerformanceReview(ctx context.Context, snapshot performanceReviewSnapshot, dispatch *analysisModelDispatch) (modelOutput, error) {
	runtime := snapshot.runtime
	prompt, err := performancePrompt(snapshot.source, snapshot.analysis, snapshot.file)
	if err != nil {
		return modelOutput{}, err
	}
	timed, cancel := context.WithTimeout(ctx, s.analysisTimeout)
	defer cancel()
	result, err := s.requestAnalysisModel(timed, runtime, []llm.ChatMessage{{Role: "user", Content: prompt}}, nil, dispatch)
	if timed.Err() != nil {
		return modelOutput{}, timed.Err()
	}
	if err != nil {
		return modelOutput{}, err
	}
	return result, nil
}

func (s *Service) publishPerformanceReview(ctx context.Context, snapshot performanceReviewSnapshot, result modelOutput, findings []project.PerformanceFinding, warning string, authorizePublication func(func() error) error) (*project.PerformanceFileReport, error) {
	if err := s.validatePerformanceReviewSnapshot(ctx, snapshot); err != nil {
		return nil, err
	}
	runtime := snapshot.runtime
	report := project.PerformanceFileReport{SchemaVersion: "1", ProjectID: snapshot.analysis.ProjectID, ProjectRevision: snapshot.analysis.ProjectRevision, Path: snapshot.file.Path, ContentHash: snapshot.file.ContentHash, Status: "completed", Findings: findings, Warning: warning, Model: runtime.profile.Model, Profile: runtime.effective.Profile, Scope: runtime.effective.Scope, ProviderOrigin: runtime.effective.ProviderOrigin, ReasoningEffort: runtime.effective.ReasoningEffort, PromptVersion: project.PerformancePromptVersion, ContextPolicyVersion: snapshot.policyVersion, GeneratedAt: time.Now().UTC()}
	if result.Model != "" {
		report.Model = result.Model
	}
	store := func() error {
		if err := s.validatePerformanceReviewSnapshot(ctx, snapshot); err != nil {
			return err
		}
		return project.StorePerformanceFileReport(snapshot.root, report)
	}
	if err := publishAnalysisReport(store, authorizePublication); err != nil {
		return nil, err
	}
	return &report, nil
}

func (s *Service) validatePerformanceReviewSnapshot(ctx context.Context, snapshot performanceReviewSnapshot) error {
	if s.runtimes.analyze.effective != snapshot.runtime.effective {
		return project.ErrRevisionConflict
	}
	return s.validateSourceFileSnapshot(ctx, snapshot.root, snapshot.analysis, snapshot.file, snapshot.policyVersion, false)
}

func performancePrompt(source string, analysis project.Analysis, file project.IndexFile) (string, error) {
	facts, err := json.Marshal(struct {
		Name     string               `json:"name"`
		Type     string               `json:"type"`
		Path     string               `json:"path"`
		Language string               `json:"language"`
		Symbols  []project.SymbolInfo `json:"symbols"`
	}{analysis.Name, analysis.Type, file.Path, file.Language, file.Symbols})
	if err != nil {
		return "", err
	}
	return "Review exactly one supplied source file for plausible performance opportunities. This is source-based review, not measurement: never claim a bottleneck, speedup, metric, hot path, or that the project is fast. Return one JSON object only: findings array (maximum five), with category (cpu,memory,io,concurrency,caching,ui), potential_impact (high,medium,low,unknown), confidence (high,medium,low), title, observed_pattern, workload_conditions, recommendation, tradeoff, verification_plan, start_line, end_line, symbol?, engineering_insight?. Require concrete source evidence and conditions; return an empty array if none. " + project.EngineeringInsightPromptInstructions + "The path, source anchors, and symbols must only refer to FILE_FACTS.\n\nFILE_FACTS:\n" + string(facts) + "\n\nSOURCE:\n```\n" + strings.TrimSpace(source) + "\n```", nil
}
