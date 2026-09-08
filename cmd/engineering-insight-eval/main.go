// Command engineering-insight-eval validates source-free collection and
// qualification receipts. It never contacts a provider.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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
	expected, err := options.expectation()
	if err != nil {
		fail(err)
	}
	report, err := app.ValidateEngineeringInsightEvaluationReceipt(receipt, expected)
	if err != nil {
		fail(err)
	}
	fmt.Printf("%s: %d/%d usable, %d/%d complete, %d useful substantive, %d omitted controls, %d critical claims; successful latency median %.0fms p95 %.0fms (%d timeout/censored)\n", report.Outcome, report.UsableAttempts, report.ScheduledAttempts, report.CompleteAttempts, report.ScheduledAttempts, report.UsefulSubstantiveAttempts, report.OmittedControls, report.CriticalFalseClaims, report.Latency.MedianMilliseconds, report.Latency.P95Milliseconds, report.Latency.TimeoutOrCensoredAttempts)
	failWithStatus(nil, evaluationExitStatus(report.Outcome))
}

type evaluationOptions struct {
	receiptPath           string
	caseSetPath           string
	candidateID           string
	provider              string
	model                 string
	promptVersion         string
	corpusID              string
	baseRevision          string
	maxRequests           int
	maxOutputTokens       int
	attemptTimeoutSeconds int
}

func evaluationOptionsFromFlags() (evaluationOptions, error) {
	receiptPath := flag.String("receipt", "", "path to a source-free evaluation receipt")
	caseSetPath := flag.String("cases", "", "path to the frozen evaluation case set")
	candidateID := flag.String("candidate-id", "", "externally selected candidate identity")
	provider := flag.String("provider", "", "selected provider identity")
	model := flag.String("model", "", "selected model identity")
	promptVersion := flag.String("prompt-version", "", "selected prompt version")
	corpusID := flag.String("corpus-id", "", "externally selected corpus identity")
	baseRevision := flag.String("base-revision", "", "selected candidate base revision")
	maxRequests := flag.Int("max-requests", 0, "authorized request cap")
	maxOutputTokens := flag.Int("max-output-tokens", 0, "authorized output-token cap per request")
	attemptTimeoutSeconds := flag.Int("attempt-timeout-seconds", 0, "authorized timeout per attempt")
	flag.Parse()
	if *receiptPath == "" || *caseSetPath == "" || *candidateID == "" || *provider == "" || *model == "" || *promptVersion == "" || *corpusID == "" || *baseRevision == "" || *maxRequests <= 0 || *maxOutputTokens <= 0 || *attemptTimeoutSeconds <= 0 {
		return evaluationOptions{}, fmt.Errorf("receipt, cases, candidate-id, provider, model, prompt-version, corpus-id, base-revision, max-requests, max-output-tokens, and attempt-timeout-seconds are required")
	}
	return evaluationOptions{receiptPath: *receiptPath, caseSetPath: *caseSetPath, candidateID: *candidateID, provider: *provider, model: *model, promptVersion: *promptVersion, corpusID: *corpusID, baseRevision: *baseRevision, maxRequests: *maxRequests, maxOutputTokens: *maxOutputTokens, attemptTimeoutSeconds: *attemptTimeoutSeconds}, nil
}

func loadReceipt(path string) (app.EngineeringInsightEvaluationReceipt, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return app.EngineeringInsightEvaluationReceipt{}, fmt.Errorf("read evaluation receipt")
	}
	receipt, err := app.DecodeEngineeringInsightEvaluationReceipt(data)
	if err != nil {
		return app.EngineeringInsightEvaluationReceipt{}, err
	}
	return receipt, nil
}

func (options evaluationOptions) expectation() (app.EngineeringInsightEvaluationExpectation, error) {
	data, err := os.ReadFile(options.caseSetPath)
	if err != nil {
		return app.EngineeringInsightEvaluationExpectation{}, fmt.Errorf("read evaluation cases")
	}
	schedule, err := qualificationSchedule(data)
	if err != nil {
		return app.EngineeringInsightEvaluationExpectation{}, err
	}
	digest := sha256.Sum256(data)
	return app.EngineeringInsightEvaluationExpectation{CandidateID: options.candidateID, Provider: options.provider, Model: options.model, PromptVersion: options.promptVersion, CorpusID: options.corpusID, CorpusDigest: hex.EncodeToString(digest[:]), BaseRevision: options.baseRevision, MaxRequests: options.maxRequests, MaxOutputTokens: options.maxOutputTokens, AttemptTimeoutSeconds: options.attemptTimeoutSeconds, Schedule: schedule}, nil
}

func qualificationSchedule(data []byte) ([]app.EngineeringInsightExpectedAttempt, error) {
	if err := app.ValidateStrictJSONDocument(data); err != nil {
		return nil, fmt.Errorf("invalid evaluation cases")
	}
	cases, err := decodeEvaluationCases(data)
	if err != nil {
		return nil, err
	}
	qualification, err := selectQualificationCases(cases)
	if err != nil {
		return nil, err
	}
	return repeatQualificationCases(qualification), nil
}

type evaluationCase struct {
	Name      string `json:"name"`
	Partition string `json:"partition"`
	Intent    string `json:"intent"`
}

type qualificationCase struct {
	name   string
	intent string
}

func decodeEvaluationCases(data []byte) ([]evaluationCase, error) {
	var cases []evaluationCase
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&cases); err != nil || len(cases) == 0 {
		return nil, fmt.Errorf("invalid evaluation cases")
	}
	return cases, nil
}

func selectQualificationCases(cases []evaluationCase) ([]qualificationCase, error) {
	seen := make(map[string]bool, len(cases))
	qualification := make([]qualificationCase, 0, 12)
	for _, evaluationCase := range cases {
		if !validEvaluationCase(evaluationCase, seen) {
			return nil, fmt.Errorf("invalid evaluation cases")
		}
		seen[evaluationCase.Name] = true
		if evaluationCase.Partition == "qualification" {
			qualification = append(qualification, qualificationCase{name: evaluationCase.Name, intent: evaluationCase.Intent})
		}
	}
	if !hasQualificationCaseBalance(qualification) {
		return nil, fmt.Errorf("evaluation cases do not define the qualification schedule")
	}
	return qualification, nil
}

func validEvaluationCase(evaluationCase evaluationCase, seen map[string]bool) bool {
	return evaluationCase.Name != "" && !seen[evaluationCase.Name] && validCasePartition(evaluationCase.Partition) && validCaseIntent(evaluationCase.Intent)
}

func validCasePartition(partition string) bool {
	return partition == "development" || partition == "qualification"
}
func validCaseIntent(intent string) bool { return intent == "substantive" || intent == "control" }

func hasQualificationCaseBalance(cases []qualificationCase) bool {
	substantive := 0
	for _, evaluationCase := range cases {
		if evaluationCase.intent == "substantive" {
			substantive++
		}
	}
	return len(cases) == 12 && substantive == 8
}

func repeatQualificationCases(cases []qualificationCase) []app.EngineeringInsightExpectedAttempt {
	schedule := make([]app.EngineeringInsightExpectedAttempt, 0, 24)
	for repetition := 1; repetition <= 2; repetition++ {
		for _, evaluationCase := range cases {
			schedule = append(schedule, app.EngineeringInsightExpectedAttempt{CaseName: evaluationCase.name, Partition: "qualification", Intent: evaluationCase.intent, Repetition: repetition, Attempt: 1})
		}
	}
	return schedule
}

func evaluationExitStatus(outcome app.EngineeringInsightEvaluationOutcome) int {
	if outcome == app.EngineeringInsightCollectionOutcome || outcome == app.EngineeringInsightPassedOutcome {
		return 0
	}
	if outcome == app.EngineeringInsightIncompleteOutcome {
		return 2
	}
	return 1
}

func fail(err error) { failWithStatus(err, 1) }

func failWithStatus(err error, status int) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "engineering-insight-eval:", err)
	}
	if status != 0 {
		os.Exit(status)
	}
}
