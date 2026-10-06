package app

import (
	"fmt"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

// Choices reference configured profiles; credentials and endpoints remain daemon-owned.
type AnalysisModels struct {
	Code     string `json:"code"`
	Review   string `json:"review"`
	Features string `json:"features"`
}

func (models *AnalysisModels) Validate() error {
	if models == nil {
		return nil
	}
	for _, profile := range []string{models.Code, models.Review, models.Features} {
		if profile != "analyze" && profile != "bug" && profile != "function" {
			return fmt.Errorf("analysis models must reference configured analyze, bug or function profiles")
		}
	}
	return nil
}

func (models *AnalysisModels) profile(stage AnalysisStage) string {
	if models == nil {
		return ""
	}
	switch stage {
	case AnalysisStageSemantic:
		return models.Code
	case AnalysisStageFeatures:
		return models.Features
	default:
		return models.Review
	}
}

func (s *Service) analysisModelRuntime(stage AnalysisStage, models *AnalysisModels) modelRuntime {
	return s.runtimeForAnalysisScope(analysisStageScope(stage), models.profile(stage))
}

func analysisStageScope(stage AnalysisStage) config.ModelScope {
	if stage == AnalysisStageSemantic {
		return config.BugModelScope
	}
	return config.AnalyzeModelScope
}

// analysisStageProfile names the configured profile a stage uses, including defaults.
func analysisStageProfile(stage AnalysisStage, models *AnalysisModels) string {
	if profile := models.profile(stage); profile != "" {
		return profile
	}
	return string(analysisStageScope(stage))
}

func (s *Service) runtimeForAnalysisScope(scope config.ModelScope, profile string) modelRuntime {
	if profile == "" {
		profile = string(scope)
	}
	runtime, _ := s.runtimes.forScope(profile) // Request and stored-plan validation reject unknown profiles.
	runtime.profile.Scope = scope
	runtime.effective.Scope = string(scope)
	return runtime
}

func cloneAnalysisModels(models *AnalysisModels) *AnalysisModels {
	if models == nil {
		return nil
	}
	copy := *models
	return &copy
}
