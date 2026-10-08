package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type ModelChoice struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Provider string         `json:"provider"`
	Source   string         `json:"source"`
	Location string         `json:"location"`
	Model    EffectiveModel `json:"model"`
}

type AvailableModels struct {
	Models   []ModelChoice     `json:"models"`
	Defaults map[string]string `json:"defaults"`
	Pi       ModelDiscovery    `json:"pi"`
}

type ModelDiscovery struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// IDs describe model configurations, not the job that originally configured them.
func configuredModelID(profile config.ModelProfile) (string, error) {
	profile.Scope, profile.APIKey, profile.ContextMaxTokens = "", "", 0
	encoded, err := json.Marshal(profile)
	if err != nil {
		return "", fmt.Errorf("encode configured model identity: %w", err)
	}
	return fmt.Sprintf("configured:%x", sha256.Sum256(encoded)), nil
}

func piModelSelection(selection string) (provider, model string, ok bool) {
	if !strings.HasPrefix(selection, "pi:") {
		return "", "", false
	}
	provider, model, ok = strings.Cut(strings.TrimPrefix(selection, "pi:"), "/")
	return provider, model, ok && llm.ValidPiModelID(provider, model)
}

func validModelSelection(selection string) bool {
	if selection == "analyze" || selection == "bug" || selection == "function" {
		return true
	}
	if _, _, ok := piModelSelection(selection); ok {
		return true
	}
	if !strings.HasPrefix(selection, "configured:") {
		return false
	}
	value, err := hex.DecodeString(strings.TrimPrefix(selection, "configured:"))
	return err == nil && len(value) == sha256.Size
}

func (s *Service) piExecutable() string {
	for _, runtime := range []modelRuntime{s.runtimes.analyze, s.runtimes.bug, s.runtimes.function} {
		if runtime.profile.Provider == config.PiProvider {
			return runtime.profile.CLIPath
		}
	}
	return "pi"
}

func (s *Service) AvailableModels(ctx context.Context) (AvailableModels, error) {
	catalog := AvailableModels{Models: []ModelChoice{}, Defaults: map[string]string{}, Pi: ModelDiscovery{Status: "ready"}}
	seen := map[string]bool{}
	for _, runtime := range []modelRuntime{s.runtimes.analyze, s.runtimes.bug, s.runtimes.function} {
		id, err := configuredModelID(runtime.profile)
		if err != nil {
			return AvailableModels{}, err
		}
		catalog.Defaults[string(runtime.profile.Scope)] = id
		if seen[id] {
			continue
		}
		seen[id] = true
		model := runtime.effective
		model.Scope, model.Profile = "", id
		location := "remote"
		if runtime.profile.IsCLI() {
			location = "unknown"
		} else if !model.RemoteProvider {
			location = "local"
		}
		catalog.Models = append(catalog.Models, ModelChoice{ID: id, Name: model.Model, Source: "configured", Provider: string(runtime.profile.Provider), Location: location, Model: model})
	}
	models, err := s.discoverPiModels(ctx, s.piExecutable())
	if ctx.Err() != nil {
		return AvailableModels{}, ctx.Err()
	}
	if err != nil {
		catalog.Pi = ModelDiscovery{Status: "unavailable", Message: "Pi models could not be loaded. Check that Pi is installed, signed in, and its model configuration is valid, then refresh models."}
		return catalog, nil
	}
	for _, candidate := range models {
		id := "pi:" + candidate.Provider + "/" + candidate.ID
		runtime := s.runtimeForAnalysisScope(config.AnalyzeModelScope, id)
		model := runtime.effective
		model.Scope = ""
		name := candidate.Name
		if name == "" {
			name = candidate.ID
		}
		location := "remote"
		if candidate.BaseURL == "" {
			location = "unknown"
		} else if isLoopbackURL(candidate.BaseURL) {
			location = "local"
		}
		catalog.Models = append(catalog.Models, ModelChoice{ID: id, Name: name, Provider: candidate.Provider, Source: "pi", Location: location, Model: model})
	}
	sort.SliceStable(catalog.Models, func(i, j int) bool {
		a, b := catalog.Models[i], catalog.Models[j]
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		if a.Provider != b.Provider {
			return a.Provider < b.Provider
		}
		return a.Name < b.Name
	})
	return catalog, nil
}

func (s *Service) selectedRuntime(scope config.ModelScope, selection string) (modelRuntime, bool) {
	if runtime, ok := s.runtimes.forScope(selection); ok {
		return runtime, true
	}
	base, _ := s.runtimes.forScope(string(scope))
	for _, runtime := range []modelRuntime{s.runtimes.analyze, s.runtimes.bug, s.runtimes.function} {
		id, err := configuredModelID(runtime.profile)
		if err != nil {
			return modelRuntime{}, false
		}
		if id != selection {
			continue
		}
		// Reuse the immutable transport and sampling values; only the operation's
		// context and dispatch limits change when assigning this model.
		selected := runtime
		selected.profile.Scope, selected.profile.ContextMaxTokens = scope, base.profile.ContextMaxTokens
		selected.effective.Scope, selected.effective.Profile = string(scope), selection
		selected.effective.ContextMaxTokens = base.profile.ContextMaxTokens
		selected.effective.Timeout, selected.effective.MaxRetries = base.effective.Timeout, base.effective.MaxRetries
		return selected, true
	}
	if provider, model, ok := piModelSelection(selection); ok {
		profile := config.ModelProfile{Scope: scope, Provider: config.PiProvider, CLIPath: s.piExecutable(), Model: provider + "/" + model, ContextMaxTokens: base.profile.ContextMaxTokens}
		runtime := newModelRuntime(profile, duration(base.effective.Timeout), base.effective.MaxRetries)
		runtime.effective.Profile = selection
		return runtime, true
	}
	return modelRuntime{}, false
}

func (s *Service) validateModelSelections(ctx context.Context, selections []string) error {
	var available map[string]bool
	for _, selection := range selections {
		if _, _, pi := piModelSelection(selection); pi {
			if available == nil {
				models, err := s.discoverPiModels(ctx, s.piExecutable())
				if err != nil {
					return fmt.Errorf("pi model catalog unavailable: %w", err)
				}
				available = make(map[string]bool, len(models))
				for _, model := range models {
					available["pi:"+model.Provider+"/"+model.ID] = true
				}
			}
			if available[selection] {
				continue
			}
		} else if _, ok := s.selectedRuntime(config.AnalyzeModelScope, selection); ok {
			continue
		}
		return fmt.Errorf("%w: selected model is unavailable; refresh models and choose again", project.ErrRevisionConflict)
	}
	return nil
}

func (s *Service) validateAnalysisModelSelections(ctx context.Context, models *AnalysisModels, resume *AnalysisRunIdentity) error {
	if resume != nil {
		run, err := s.ReadAnalysisRun(ctx, resume.ProjectID, resume.ProjectRevision)
		// An explicit preview/resume can recover a progress-write fault using
		// the retained in-memory run. Other read failures still stop admission.
		if err != nil && (run == nil || !errors.Is(err, errAnalysisRunPersistence)) {
			return err
		}
		if run == nil || run.Identity != *resume {
			return project.ErrRevisionConflict
		}
		models = run.Plan.Models
	}
	if models == nil {
		return nil
	}
	return s.validateModelSelections(ctx, models.selections())
}
