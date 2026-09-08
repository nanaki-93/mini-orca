package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
)

func TestQualificationScheduleBuildsFrozenTwoRepetitionPlan(t *testing.T) {
	schedule, err := qualificationSchedule([]byte(testCasesJSON()))
	if err != nil {
		t.Fatalf("qualification schedule: %v", err)
	}
	if len(schedule) != 24 {
		t.Fatalf("schedule count = %d, want 24", len(schedule))
	}
	for index, attempt := range schedule {
		if attempt.Partition != "qualification" || attempt.Attempt != 1 || attempt.Repetition != index/12+1 {
			t.Fatalf("schedule attempt %d = %+v", index, attempt)
		}
	}
}

func TestQualificationScheduleRejectsIncompleteAndRepeatedCorpus(t *testing.T) {
	for name, data := range map[string]string{
		"not enough qualification cases": `[{"name":"case","partition":"qualification","intent":"substantive"}]`,
		"duplicate names":                strings.Repeat(`{"name":"same","partition":"qualification","intent":"substantive"},`, 12),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := qualificationSchedule([]byte("[" + strings.TrimSuffix(data, ",") + "]")); err == nil {
				t.Fatal("invalid corpus was accepted")
			}
		})
	}
}

func TestQualificationScheduleRejectsDuplicateAndTrailingJSON(t *testing.T) {
	for name, data := range map[string]string{
		"duplicate object field": `[{"name":"case","name":"other","partition":"qualification","intent":"substantive"}]`,
		"trailing JSON":          testCasesJSON() + ` []`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := qualificationSchedule([]byte(data)); err == nil {
				t.Fatal("unsafe corpus JSON was accepted")
			}
		})
	}
}

func TestEvaluationExitStatusSeparatesValidCollectionIncompleteFailureAndPass(t *testing.T) {
	for outcome, want := range map[app.EngineeringInsightEvaluationOutcome]int{
		app.EngineeringInsightCollectionOutcome: 0,
		app.EngineeringInsightPassedOutcome:     0,
		app.EngineeringInsightIncompleteOutcome: 2,
		app.EngineeringInsightFailedOutcome:     1,
	} {
		if got := evaluationExitStatus(outcome); got != want {
			t.Fatalf("exit status for %s = %d, want %d", outcome, got, want)
		}
	}
}

func TestEvaluationOptionsBindReceiptToExternallySelectedIdentity(t *testing.T) {
	options := evaluationOptions{candidateID: "candidate", provider: "provider", model: "model", promptVersion: "prompt", corpusID: "corpus", baseRevision: "base", maxRequests: 24, maxOutputTokens: 4096, attemptTimeoutSeconds: 300}
	path := writeCases(t, testCasesJSON())
	options.caseSetPath = path
	expected, err := options.expectation()
	if err != nil {
		t.Fatal(err)
	}
	receipt := receiptFor(expected)
	if report, err := app.ValidateEngineeringInsightEvaluationReceipt(receipt, expected); err != nil || report.Outcome != app.EngineeringInsightPassedOutcome {
		t.Fatalf("externally selected receipt = %+v, %v", report, err)
	}
	receipt.CorpusDigest = strings.Repeat("b", 64)
	if _, err := app.ValidateEngineeringInsightEvaluationReceipt(receipt, expected); err == nil {
		t.Fatal("self-asserted corpus identity was accepted")
	}
}

func testCasesJSON() string {
	cases := make([]string, 0, 15)
	for index := 0; index < 3; index++ {
		cases = append(cases, fmt.Sprintf(`{"name":"development-%d","partition":"development","intent":"substantive"}`, index))
	}
	for index := 0; index < 12; index++ {
		intent := "substantive"
		if index >= 8 {
			intent = "control"
		}
		cases = append(cases, fmt.Sprintf(`{"name":"qualification-%d","partition":"qualification","intent":"%s"}`, index, intent))
	}
	return "[" + strings.Join(cases, ",") + "]"
}

func writeCases(t *testing.T, data string) string {
	t.Helper()
	path := t.TempDir() + "/cases.json"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func receiptFor(expected app.EngineeringInsightEvaluationExpectation) app.EngineeringInsightEvaluationReceipt {
	receipt := app.EngineeringInsightEvaluationReceipt{Mode: app.EngineeringInsightQualificationMode, RunID: "run", CandidateID: expected.CandidateID, Provider: expected.Provider, Model: expected.Model, PromptVersion: expected.PromptVersion, CorpusID: expected.CorpusID, CorpusDigest: expected.CorpusDigest, BaseRevision: expected.BaseRevision, MaxRequests: expected.MaxRequests, MaxOutputTokens: expected.MaxOutputTokens, AttemptTimeoutSeconds: expected.AttemptTimeoutSeconds}
	for index, scheduled := range expected.Schedule {
		digest := fmt.Sprintf("%064x", index+1)
		optional := "present"
		if scheduled.Intent == "control" {
			optional = "omitted"
		}
		receipt.Attempts = append(receipt.Attempts, app.EngineeringInsightEvaluationAttempt{CaseName: scheduled.CaseName, Partition: scheduled.Partition, Intent: scheduled.Intent, Repetition: scheduled.Repetition, Attempt: scheduled.Attempt, CandidateID: expected.CandidateID, Outcome: "completed", UsableSummary: true, CompleteSummary: true, OptionalInsight: optional, EmittedResponse: true, ResponseDigest: digest, OutputTokens: 100, FinishReason: "stop", ElapsedMilliseconds: float64(index + 1), Score: &app.EngineeringInsightAttemptScore{ResponseDigest: digest, Correctness: 2, LocalRelevance: 2, TradeoffClarity: 2, UsefulVerification: 2}})
	}
	receipt.Consumption = app.EngineeringInsightEvaluationConsumption{Requests: len(receipt.Attempts), OutputTokens: len(receipt.Attempts) * 100}
	return receipt
}
