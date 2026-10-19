package app

import (
	"github.com/nanaki-93/mini-orca/v2/internal/app/features"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

const (
	legacyFeatureSchemaVersion   = features.LegacySchemaVersion
	featureHistorySchemaVersion  = features.HistorySchemaVersion
	maxFeatureHistoryBytes       = features.MaxHistoryBytes
	maxExistingFeatureIdeasBytes = features.MaxExistingIdeasBytes
	legacyFeatureGenerationID    = features.LegacyGenerationID
)

var (
	errFeatureHistoryFull        = features.ErrHistoryFull
	errUnsupportedFeatureHistory = features.ErrUnsupportedHistory
)

type FeatureModelSummary = features.FeatureModelSummary
type FeatureGeneration = features.FeatureGeneration
type FeatureGenerationOutcome = features.FeatureGenerationOutcome

func readFeatures(root string, index *project.ProjectIndex) (*FeatureReport, error) {
	return features.Read(root, index)
}

func writeFeatures(root string, report *FeatureReport) error {
	return features.Write(root, report)
}

func clearFeatureSuggestions(root string, index *project.ProjectIndex) (*FeatureReport, error) {
	return features.Clear(root, index)
}

func existingFeatureIdeas(suggestions []FeatureSuggestion) string {
	return features.ExistingIdeas(suggestions)
}

func featureModelSummary(runtime modelRuntime) *FeatureModelSummary {
	return &FeatureModelSummary{
		Profile:        runtime.effective.Profile,
		Model:          runtime.effective.Model,
		ProviderOrigin: runtime.effective.ProviderOrigin,
		RemoteProvider: runtime.effective.RemoteProvider,
	}
}
