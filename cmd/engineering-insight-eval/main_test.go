package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
)

func TestEvaluationReceiptAcceptsKnownCaseAndExactSelectedMetadata(t *testing.T) {
	options := evaluationOptions{
		provider: "chosen-provider", model: "chosen-model", promptVersion: "file-analysis-v5",
		maxRequests: 1, maxOutputTokens: 800,
	}
	receipt := evaluationReceiptFixture()
	if err := options.validateReceipt(receipt); err != nil {
		t.Fatalf("validate receipt: %v", err)
	}
	if err := validateReceiptCaseNames(receipt, map[string]bool{"allocation": true}); err != nil {
		t.Fatalf("validate known case: %v", err)
	}
}

func TestEvaluationReceiptRejectsUnknownSampleCase(t *testing.T) {
	receipt := evaluationReceiptFixture()
	receipt.Samples[0].CaseName = "unknown"
	if err := validateReceiptCaseNames(receipt, map[string]bool{"allocation": true}); err == nil {
		t.Fatal("unknown sample case was accepted")
	}
}

func TestLoadCaseNamesRejectsEmptyAndRepeatedCaseSets(t *testing.T) {
	for name, contents := range map[string]string{
		"empty":    `[]`,
		"repeated": `[{"name":"allocation"},{"name":"allocation"}]`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "cases.json")
			if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadCaseNames(path); err == nil {
				t.Fatal("invalid case set was accepted")
			}
		})
	}
}

func TestEvaluationReceiptRejectsEveryMetadataMismatch(t *testing.T) {
	options := evaluationOptions{
		provider: "chosen-provider", model: "chosen-model", promptVersion: "file-analysis-v5",
		maxRequests: 1, maxOutputTokens: 800,
	}
	for name, mutate := range map[string]func(*app.EngineeringInsightEvaluationReceipt){
		"provider":       func(receipt *app.EngineeringInsightEvaluationReceipt) { receipt.Provider = "other-provider" },
		"model":          func(receipt *app.EngineeringInsightEvaluationReceipt) { receipt.Model = "other-model" },
		"prompt version": func(receipt *app.EngineeringInsightEvaluationReceipt) { receipt.PromptVersion = "file-analysis-v4" },
		"request budget": func(receipt *app.EngineeringInsightEvaluationReceipt) { receipt.MaxRequests = 2 },
		"token budget":   func(receipt *app.EngineeringInsightEvaluationReceipt) { receipt.MaxOutputTokens = 801 },
	} {
		t.Run(name, func(t *testing.T) {
			receipt := evaluationReceiptFixture()
			mutate(&receipt)
			if err := options.validateReceipt(receipt); err == nil {
				t.Fatal("metadata mismatch was accepted")
			}
		})
	}
}

func evaluationReceiptFixture() app.EngineeringInsightEvaluationReceipt {
	return app.EngineeringInsightEvaluationReceipt{
		Provider: "chosen-provider", Model: "chosen-model", PromptVersion: "file-analysis-v5",
		SampleCount: 1, MaxRequests: 1, MaxOutputTokens: 800,
		Samples: []app.EngineeringInsightSampleScore{{
			CaseName: "allocation", Correctness: 2, LocalRelevance: 2,
			TradeoffClarity: 1, UsefulVerification: 1, RetainExample: true,
		}},
	}
}
