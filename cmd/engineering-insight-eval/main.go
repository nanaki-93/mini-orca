// Command engineering-insight-eval validates a human-scored live insight sample.
// It makes no provider request; a reviewer runs the selected bounded sample through
// Mini-Orca first, then records the resulting scores in the receipt passed here.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
)

func main() {
	options, err := evaluationOptionsFromFlags()
	if err != nil {
		failWithStatus(err, 2)
	}
	receipt, err := loadReceipt(options.receiptPath)
	if err != nil {
		fail(err)
	}
	if err := options.validateReceipt(receipt); err != nil {
		fail(err)
	}
	caseNames, err := loadCaseNames(options.caseSetPath)
	if err != nil {
		fail(err)
	}
	if err := validateReceiptCaseNames(receipt, caseNames); err != nil {
		fail(err)
	}
	fmt.Printf("accepted %d scored samples for %s / %s / %s within %d requests and %d output tokens per request\n", receipt.SampleCount, receipt.Provider, receipt.Model, receipt.PromptVersion, receipt.MaxRequests, receipt.MaxOutputTokens)
}

type evaluationOptions struct {
	receiptPath     string
	caseSetPath     string
	provider        string
	model           string
	promptVersion   string
	maxRequests     int
	maxOutputTokens int
}

func evaluationOptionsFromFlags() (evaluationOptions, error) {
	receiptPath := flag.String("receipt", "", "path to a human-scored evaluation receipt")
	caseSetPath := flag.String("cases", "", "path to the selected evaluation case set")
	provider := flag.String("provider", "", "provider selected for the live sample")
	model := flag.String("model", "", "model selected for the live sample")
	promptVersion := flag.String("prompt-version", "", "prompt version selected for the live sample")
	maxRequests := flag.Int("max-requests", 0, "maximum provider requests authorized for the sample")
	maxOutputTokens := flag.Int("max-output-tokens", 0, "maximum output tokens authorized per request")
	flag.Parse()
	if *receiptPath == "" || *caseSetPath == "" || *provider == "" || *model == "" || *promptVersion == "" || *maxRequests <= 0 || *maxOutputTokens <= 0 {
		return evaluationOptions{}, fmt.Errorf("receipt, cases, provider, model, prompt-version, max-requests, and max-output-tokens are required")
	}
	return evaluationOptions{receiptPath: *receiptPath, caseSetPath: *caseSetPath, provider: *provider, model: *model, promptVersion: *promptVersion, maxRequests: *maxRequests, maxOutputTokens: *maxOutputTokens}, nil
}

func loadReceipt(path string) (app.EngineeringInsightEvaluationReceipt, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return app.EngineeringInsightEvaluationReceipt{}, err
	}
	var receipt app.EngineeringInsightEvaluationReceipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		return app.EngineeringInsightEvaluationReceipt{}, fmt.Errorf("decode evaluation receipt: %w", err)
	}
	return receipt, nil
}

func (options evaluationOptions) validateReceipt(receipt app.EngineeringInsightEvaluationReceipt) error {
	if receipt.Provider != options.provider || receipt.Model != options.model || receipt.PromptVersion != options.promptVersion || receipt.MaxRequests != options.maxRequests || receipt.MaxOutputTokens != options.maxOutputTokens {
		return fmt.Errorf("receipt metadata must match the explicitly selected provider, model, prompt version, and budget")
	}
	if err := app.ValidateEngineeringInsightEvaluationReceipt(receipt); err != nil {
		return err
	}
	return nil
}

func validateReceiptCaseNames(receipt app.EngineeringInsightEvaluationReceipt, caseNames map[string]bool) error {
	for _, sample := range receipt.Samples {
		if !caseNames[sample.CaseName] {
			return fmt.Errorf("receipt names unknown evaluation case %q", sample.CaseName)
		}
	}
	return nil
}

func loadCaseNames(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cases []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		return nil, fmt.Errorf("decode evaluation cases: %w", err)
	}
	names := make(map[string]bool, len(cases))
	for _, evaluationCase := range cases {
		if evaluationCase.Name == "" || names[evaluationCase.Name] {
			return nil, fmt.Errorf("evaluation cases contain an empty or repeated name %q", evaluationCase.Name)
		}
		names[evaluationCase.Name] = true
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("evaluation cases are empty")
	}
	return names, nil
}

func fail(err error) {
	failWithStatus(err, 1)
}

func failWithStatus(err error, status int) {
	fmt.Fprintln(os.Stderr, "engineering-insight-eval:", err)
	os.Exit(status)
}
