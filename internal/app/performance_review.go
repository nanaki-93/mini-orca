package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

const maxPerformanceSourceBytes = 64 * 1024

// CachedPerformanceFileReview reads a prior performance report without model work.
func (s *Service) CachedPerformanceFileReview(path string) (*project.PerformanceFileReport, error) {
	file, err := s.manager.IndexedFile(path)
	if err != nil {
		return nil, err
	}
	return project.LoadPerformanceFileReport(s.manager.Root(), file.Path, file.ContentHash)
}

// ReviewPerformanceFile explicitly reviews one eligible file's supplied source.
// It never executes project code and has no route until the bounded job API ships.
func (s *Service) ReviewPerformanceFile(ctx context.Context, path string, confirmRemoteProvider bool) (*project.PerformanceFileReport, error) {
	return s.reviewPerformanceFile(ctx, path, confirmRemoteProvider, nil)
}

func (s *Service) reviewPerformanceFile(ctx context.Context, path string, confirmRemoteProvider bool, authorizePublication func() error) (*project.PerformanceFileReport, error) {
	if err := s.RequireRemoteConfirmation(config.AnalyzeModelScope, confirmRemoteProvider); err != nil {
		return nil, err
	}
	file, err := s.manager.IndexedFile(path)
	if err != nil {
		return nil, err
	}
	policy, err := project.NewContextPolicy(s.manager.Root())
	if err != nil {
		return nil, err
	}
	if decision := policy.Decide(file.Path); !decision.Include {
		return nil, fmt.Errorf("performance review path is not eligible: %s", decision.Reason)
	}
	info, err := project.GetFileInfo(s.manager.Root(), file.Path)
	if err != nil {
		return nil, err
	}
	if info.ContentHash != file.ContentHash {
		return nil, project.ErrRevisionConflict
	}
	if len(info.Content) > maxPerformanceSourceBytes {
		return nil, fmt.Errorf("performance review skipped: source exceeds 64 KiB")
	}
	analysis, err := s.manager.Analysis()
	if err != nil {
		return nil, err
	}
	runtime := s.runtimes.analyze
	prompt, err := performancePrompt(info.Content, *analysis, *file)
	if err != nil {
		return nil, err
	}
	timed, cancel := context.WithTimeout(ctx, s.analysisTimeout)
	defer cancel()
	result, err := s.retry(timed, runtime, []llm.ChatMessage{{Role: "user", Content: prompt}})
	if timed.Err() != nil {
		return nil, timed.Err()
	}
	if err != nil {
		return nil, err
	}
	findings, warning, err := project.ParsePerformanceFindings(result.Content, file.Path, info.Content, file.Symbols)
	if err != nil {
		return nil, err
	}
	current, err := project.GetFileInfo(s.manager.Root(), file.Path)
	if err != nil {
		return nil, err
	}
	if current.ContentHash != file.ContentHash {
		return nil, project.ErrRevisionConflict
	}
	report := project.PerformanceFileReport{SchemaVersion: "1", ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, Path: file.Path, ContentHash: file.ContentHash, Status: "completed", Findings: findings, Warning: warning, Model: runtime.profile.Model, Profile: runtime.effective.Profile, Scope: runtime.effective.Scope, ProviderOrigin: runtime.effective.ProviderOrigin, ReasoningEffort: runtime.effective.ReasoningEffort, PromptVersion: project.PerformancePromptVersion, ContextPolicyVersion: policy.Version(), GeneratedAt: time.Now().UTC()}
	if result.Model != "" {
		report.Model = result.Model
	}
	if authorizePublication != nil {
		if err := authorizePublication(); err != nil {
			return nil, err
		}
	}
	if err := project.StorePerformanceFileReport(s.manager.Root(), report); err != nil {
		return nil, err
	}
	return &report, nil
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
	return "Review exactly one supplied source file for plausible performance opportunities. This is source-based review, not measurement: never claim a bottleneck, speedup, metric, hot path, or that the project is fast. Return one JSON object only: findings array (maximum five), with category (cpu,memory,io,concurrency,caching,ui), potential_impact (high,medium,low,unknown), confidence (high,medium,low), title, observed_pattern, workload_conditions, recommendation, tradeoff, verification_plan, start_line, end_line, symbol?, engineering_insight?. Require concrete source evidence and conditions; return an empty array if none. The path, source anchors, and symbols must only refer to FILE_FACTS.\n\nFILE_FACTS:\n" + string(facts) + "\n\nSOURCE:\n```\n" + strings.TrimSpace(source) + "\n```", nil
}
