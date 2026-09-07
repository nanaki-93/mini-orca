package app

import (
	"fmt"
	"strings"
)

const minimumRetainedEngineeringInsightScore = 6

// EngineeringInsightEvaluationReceipt records a human-scored, opt-in provider
// sample. It intentionally stores no credential, endpoint, or source text.
type EngineeringInsightEvaluationReceipt struct {
	Provider        string                          `json:"provider"`
	Model           string                          `json:"model"`
	PromptVersion   string                          `json:"prompt_version"`
	SampleCount     int                             `json:"sample_count"`
	MaxRequests     int                             `json:"max_requests"`
	MaxOutputTokens int                             `json:"max_output_tokens"`
	Samples         []EngineeringInsightSampleScore `json:"samples"`
}

// EngineeringInsightSampleScore separates deterministic contract checks from
// an evaluator's judgment about whether the returned prose teaches something useful.
type EngineeringInsightSampleScore struct {
	CaseName           string `json:"case_name"`
	Correctness        int    `json:"correctness"`
	LocalRelevance     int    `json:"local_relevance"`
	TradeoffClarity    int    `json:"tradeoff_clarity"`
	UsefulVerification int    `json:"useful_verification"`
	CriticalFalseClaim bool   `json:"critical_false_claim"`
	RetainExample      bool   `json:"retain_example"`
}

func (score EngineeringInsightSampleScore) Total() int {
	return score.Correctness + score.LocalRelevance + score.TradeoffClarity + score.UsefulVerification
}

// ValidateEngineeringInsightEvaluationReceipt requires an explicit bounded
// sampling plan and prevents retaining an example with a critical false claim
// or a score below the acceptance threshold.
func ValidateEngineeringInsightEvaluationReceipt(receipt EngineeringInsightEvaluationReceipt) error {
	if err := validateEngineeringInsightEvaluationMetadata(receipt); err != nil {
		return err
	}
	if err := validateEngineeringInsightEvaluationBudget(receipt); err != nil {
		return err
	}
	return validateEngineeringInsightEvaluationSamples(receipt.Samples)
}

func validateEngineeringInsightEvaluationMetadata(receipt EngineeringInsightEvaluationReceipt) error {
	for _, field := range []struct {
		name  string
		value string
	}{
		{"provider", receipt.Provider},
		{"model", receipt.Model},
		{"prompt_version", receipt.PromptVersion},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("engineering insight evaluation %s is required", field.name)
		}
	}
	return nil
}

func validateEngineeringInsightEvaluationBudget(receipt EngineeringInsightEvaluationReceipt) error {
	if receipt.SampleCount <= 0 || receipt.MaxRequests <= 0 || receipt.SampleCount > receipt.MaxRequests {
		return fmt.Errorf("engineering insight evaluation sample_count must be between 1 and max_requests")
	}
	if receipt.MaxOutputTokens <= 0 {
		return fmt.Errorf("engineering insight evaluation max_output_tokens is required")
	}
	if len(receipt.Samples) != receipt.SampleCount {
		return fmt.Errorf("engineering insight evaluation has %d samples, want %d", len(receipt.Samples), receipt.SampleCount)
	}
	return nil
}

func validateEngineeringInsightEvaluationSamples(samples []EngineeringInsightSampleScore) error {
	seen := make(map[string]struct{}, len(samples))
	for _, sample := range samples {
		if err := validateEngineeringInsightEvaluationSampleName(sample, seen); err != nil {
			return err
		}
		if err := validateEngineeringInsightEvaluationScores(sample); err != nil {
			return err
		}
		if err := validateEngineeringInsightEvaluationRetention(sample); err != nil {
			return err
		}
	}
	return nil
}

func validateEngineeringInsightEvaluationSampleName(sample EngineeringInsightSampleScore, seen map[string]struct{}) error {
	if strings.TrimSpace(sample.CaseName) == "" {
		return fmt.Errorf("engineering insight evaluation sample case_name is required")
	}
	if _, duplicate := seen[sample.CaseName]; duplicate {
		return fmt.Errorf("engineering insight evaluation repeats sample %q", sample.CaseName)
	}
	seen[sample.CaseName] = struct{}{}
	return nil
}

func validateEngineeringInsightEvaluationScores(sample EngineeringInsightSampleScore) error {
	for _, score := range []int{sample.Correctness, sample.LocalRelevance, sample.TradeoffClarity, sample.UsefulVerification} {
		if score < 0 || score > 2 {
			return fmt.Errorf("engineering insight evaluation score for %q must be between 0 and 2", sample.CaseName)
		}
	}
	return nil
}

func validateEngineeringInsightEvaluationRetention(sample EngineeringInsightSampleScore) error {
	if sample.RetainExample && (sample.CriticalFalseClaim || sample.Total() < minimumRetainedEngineeringInsightScore) {
		return fmt.Errorf("engineering insight evaluation cannot retain %q with a critical false claim or score below %d", sample.CaseName, minimumRetainedEngineeringInsightScore)
	}
	return nil
}
