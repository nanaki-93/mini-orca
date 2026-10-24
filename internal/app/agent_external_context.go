package app

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type ExternalSource struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
}

type externalContext struct {
	root, policy, prompt string
	query                AgentRunQuery
	sources              []ExternalSource
}

func (s *Service) prepareExternalContext(ctx context.Context, query AgentRunQuery, question string, sources []ExternalSource, limit int) (externalContext, error) {
	captured := externalContext{root: s.manager.Root(), query: query, sources: slices.Clone(sources)}
	if err := s.integrationScope(ctx, query, captured.root); err != nil {
		return captured, err
	}
	if err := validateExternalQuestion(question, sources); err != nil {
		return captured, err
	}
	policy, err := project.NewContextPolicy(captured.root)
	if err != nil {
		return captured, err
	}
	captured.policy = policy.Version()
	var prompt strings.Builder
	prompt.WriteString(question)
	seen := map[string]bool{}
	for _, source := range captured.sources {
		if seen[source.Path] || source.Hash == "" {
			return captured, fmt.Errorf("external context requires distinct paths and captured hashes")
		}
		seen[source.Path] = true
		file, err := s.manager.IndexedFile(source.Path)
		if err != nil {
			return captured, err
		}
		if file.ContentHash != source.Hash {
			return captured, project.ErrRevisionConflict
		}
		data, err := policy.ReadSource(ctx, source.Path, source.Hash)
		if err != nil {
			return captured, err
		}
		fmt.Fprintf(&prompt, "\n\nUntrusted admitted source: %s (%s)\n%s", source.Path, source.Hash, data)
		if prompt.Len() > 64<<10 {
			return captured, fmt.Errorf("external context exceeds 64 KiB")
		}
	}
	captured.prompt = prompt.String()
	if (len(captured.prompt)+3)/4+128 > declarationContextLimit(limit) {
		return captured, fmt.Errorf("external context exceeds the configured Function context limit")
	}
	return captured, s.validateExternalContext(ctx, captured)
}

func validateExternalQuestion(question string, sources []ExternalSource) error {
	if strings.TrimSpace(question) == "" || len(question) > 2048 || !utf8.ValidString(question) || strings.ContainsRune(question, 0) || len(sources) > 8 {
		return fmt.Errorf("provide a question up to 2048 bytes and at most eight explicit source files")
	}
	for _, char := range question {
		if unicode.IsControl(char) && char != '\n' && char != '\t' {
			return fmt.Errorf("question contains unsupported control characters")
		}
	}
	return nil
}

func (s *Service) validateExternalContext(ctx context.Context, captured externalContext) error {
	if err := s.integrationScope(ctx, captured.query, captured.root); err != nil {
		return err
	}
	policy, err := project.NewContextPolicy(captured.root)
	if err != nil {
		return err
	}
	if policy.Version() != captured.policy {
		return project.ErrRevisionConflict
	}
	for _, source := range captured.sources {
		if _, err := policy.ReadSource(ctx, source.Path, source.Hash); err != nil {
			return err
		}
	}
	return nil
}
