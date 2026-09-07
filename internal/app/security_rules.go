package app

import (
	"context"
	"fmt"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type securityRulesSnapshot struct {
	root          string
	analysis      project.Analysis
	file          project.IndexFile
	source        string
	policyVersion string
}

// ScanSecurityFile applies the deterministic source-only Go rules to one
// current, policy-eligible file. It never calls a provider or runs a process.
func (s *Service) ScanSecurityFile(ctx context.Context, path, revision string) (*project.SecurityFileReport, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	snapshot, err := s.prepareSecurityRulesScan(path, revision)
	if err != nil {
		return nil, err
	}
	input := project.SecurityReportInput{
		ProjectID: snapshot.analysis.ProjectID, ProjectRevision: snapshot.analysis.ProjectRevision,
		Path: snapshot.file.Path, ContentHash: snapshot.file.ContentHash,
		Source: project.SecuritySourceDeterministic, RuleSetVersion: project.SecurityGoRuleSetVersion,
		ContextPolicyVersion: snapshot.policyVersion,
	}
	cached, err := s.loadSecurityFileReport(snapshot.root, input)
	if err != nil {
		return nil, err
	}
	if cached != nil && (cached.Status == project.SecurityStatusCompleted || cached.Status == project.SecurityStatusCompletedEmpty || cached.Status == project.SecurityStatusPartial) {
		if err := s.validateSecurityRulesSnapshot(ctx, snapshot); err != nil {
			return nil, err
		}
		return cached, nil
	}
	scan, err := project.ScanGoSecurityRules(snapshot.file, snapshot.source)
	if err != nil {
		return nil, err
	}
	report := project.SecurityFileReport{
		SchemaVersion: "1", ProjectID: input.ProjectID, ProjectRevision: input.ProjectRevision,
		Path: input.Path, ContentHash: input.ContentHash, Source: input.Source, RuleSetVersion: input.RuleSetVersion,
		Findings: scan.Findings, ContextPolicyVersion: input.ContextPolicyVersion, GeneratedAt: time.Now().UTC(),
	}
	if scan.Truncated {
		report.Status = project.SecurityStatusPartial
		report.Reason = "Only the first five source rule matches are shown; review the file for additional matches."
	} else if len(scan.Findings) == 0 {
		report.Status = project.SecurityStatusCompletedEmpty
	} else {
		report.Status = project.SecurityStatusCompleted
	}
	if err := s.validateSecurityRulesSnapshot(ctx, snapshot); err != nil {
		return nil, err
	}
	if err := project.StoreSecurityFileReport(snapshot.root, report, snapshot.file); err != nil {
		return nil, err
	}
	return &report, nil
}

func (s *Service) prepareSecurityRulesScan(path, revision string) (securityRulesSnapshot, error) {
	analysis, err := s.manager.Analysis()
	if err != nil {
		return securityRulesSnapshot{}, err
	}
	if revision == "" || revision != analysis.ProjectRevision {
		return securityRulesSnapshot{}, project.ErrRevisionConflict
	}
	file, err := s.manager.IndexedFile(path)
	if err != nil {
		return securityRulesSnapshot{}, err
	}
	if file.Language != "Go" || file.Binary {
		return securityRulesSnapshot{}, project.ErrSecurityRulesUnavailable
	}
	root := s.manager.Root()
	policy, err := project.NewContextPolicy(root)
	if err != nil {
		return securityRulesSnapshot{}, err
	}
	if decision := policy.Decide(file.Path); !decision.Include {
		return securityRulesSnapshot{}, fmt.Errorf("%w: %s", project.ErrExcludedFile, decision.Reason)
	}
	info, err := project.GetFileInfo(root, file.Path)
	if err != nil {
		return securityRulesSnapshot{}, err
	}
	if info.Binary || info.ContentHash != file.ContentHash {
		return securityRulesSnapshot{}, project.ErrRevisionConflict
	}
	return securityRulesSnapshot{root: root, analysis: *analysis, file: *file, source: info.Content, policyVersion: policy.Version()}, nil
}

func (s *Service) validateSecurityRulesSnapshot(ctx context.Context, snapshot securityRulesSnapshot) error {
	return s.validateSourceFileSnapshot(ctx, snapshot.root, snapshot.analysis, snapshot.file, snapshot.policyVersion, true)
}
