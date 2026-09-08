package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
)

func TestRunEvaluationModeCompletesOfflineDevelopmentLifecycle(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init")
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".mini-orca/\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", ".gitignore")
	git(t, root, "-c", "user.email=test@example.invalid", "-c", "user.name=Test", "commit", "-m", "fixture")
	base := strings.TrimSpace(git(t, root, "rev-parse", "HEAD"))
	var calls int
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls++
		_ = json.NewEncoder(writer).Encode(map[string]any{"model": "fixture-model", "choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": `{"purpose":"summary","responsibilities":[],"dependencies":[],"side_effects":[],"risks":[],"suggestions":[],"symbol_explanations":{}}`}, "finish_reason": "stop"}}, "usage": map[string]int{"completion_tokens": 7}})
	}))
	defer provider.Close()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configText := "model_scopes:\n  analyze: {api_base_url: " + provider.URL + ", model: fixture-model}\n  bug: {api_base_url: " + provider.URL + ", model: fixture-model}\n  function: {api_base_url: " + provider.URL + ", model: fixture-model}\n"
	if err := os.WriteFile(configPath, []byte(configText), 0600); err != nil {
		t.Fatal(err)
	}
	casesPath, err := filepath.Abs("../../internal/app/testdata/engineering-insight-eval/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	receiptPath := filepath.Join(t.TempDir(), "receipt.json")
	args := lifecycleArgs(root, configPath, casesPath, base, receiptPath)
	if err := runEvaluationMode(args); err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatalf("provider calls = %d, want 3", calls)
	}
	if err := os.WriteFile(receiptPath, []byte("preserve-this-receipt"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := runEvaluationMode(args); err == nil || calls != 3 {
		t.Fatal("finished collection run was accepted")
	}
	if data, err := os.ReadFile(receiptPath); err != nil || string(data) != "preserve-this-receipt" {
		t.Fatal("finished collection rewrote receipt")
	}
	if err := runEvaluationMode([]string{"-mode", "handoff", "-root", root, "-run-id", "development-one"}); err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatal("handoff made a provider call")
	}
	handoff, err := app.LoadEngineeringInsightScoringHandoff(root, "development-one")
	if err != nil {
		t.Fatal(err)
	}
	scores := map[string]app.EngineeringInsightAttemptScore{}
	for id, response := range handoff.Responses {
		digest := sha256.Sum256([]byte(response))
		scores[id] = app.EngineeringInsightAttemptScore{ResponseDigest: hex.EncodeToString(digest[:]), Correctness: 2, LocalRelevance: 2, TradeoffClarity: 2, UsefulVerification: 2}
	}
	scoresPath := filepath.Join(t.TempDir(), "scores.json")
	data, _ := json.Marshal(scores)
	if err := os.WriteFile(scoresPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := runEvaluationMode([]string{"-mode", "score", "-root", root, "-run-id", "development-one", "-scores", scoresPath, "-receipt", receiptPath}); err != nil {
		t.Fatal(err)
	}
	exportPath := filepath.Join(t.TempDir(), "export.json")
	if err := runEvaluationMode([]string{"-mode", "export", "-root", root, "-run-id", "development-one", "-receipt", exportPath}); err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatal("offline lifecycle made a generation call")
	}
	receipt, err := loadReceipt(exportPath)
	if err != nil {
		t.Fatal(err)
	}
	options := evaluationOptions{caseSetPath: casesPath, candidateID: "candidate", provider: "fixture", model: "fixture-model", promptVersion: app.EngineeringInsightPromptVersion(), corpusID: "corpus", baseRevision: base, maxRequests: 6, maxOutputTokens: 4096, attemptTimeoutSeconds: 300}
	expected, err := options.expectationFor(receipt.Mode)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.ValidateEngineeringInsightEvaluationReceipt(receipt, expected); err != nil {
		t.Fatal(err)
	}
	badBase := append([]string(nil), args...)
	badBase[len(badBase)-1] = "wrong-base"
	if err := runEvaluationMode(badBase); err == nil {
		t.Fatal("invalid base was accepted")
	}
	badPrompt := append([]string(nil), args...)
	for index := range badPrompt {
		if badPrompt[index] == "-prompt-version" {
			badPrompt[index+1] = "wrong-prompt"
		}
	}
	if err := runEvaluationMode(badPrompt); err == nil || calls != 3 {
		t.Fatal("invalid prompt dispatched a request")
	}
	badID := append([]string(nil), args...)
	for index := range badID {
		if badID[index] == "-run-id" {
			badID[index+1] = ".."
		}
	}
	if err := runEvaluationMode(badID); err == nil || calls != 3 {
		t.Fatal("path-like run ID dispatched a request")
	}
	stateDirectory := filepath.Join(root, ".mini-orca", "autopilot", "engineering-insight-evaluation")
	campaign := filepath.Join(stateDirectory, "campaign.json")
	campaignData, err := os.ReadFile(campaign)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"...json", "..json"} {
		if err := os.WriteFile(filepath.Join(stateDirectory, name), []byte(`{"finished":true}`), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, runID := range []string{".", ".."} {
		for _, modeArgs := range [][]string{
			{"-mode", "handoff", "-root", root, "-run-id", runID},
			{"-mode", "score", "-root", root, "-run-id", runID, "-scores", filepath.Join(t.TempDir(), "unread-scores.json"), "-receipt", filepath.Join(t.TempDir(), "unwritten-receipt.json")},
			{"-mode", "export", "-root", root, "-run-id", runID, "-receipt", filepath.Join(t.TempDir(), "unwritten-export.json")},
			{"-mode", "discard", "-root", root, "-run-id", runID},
		} {
			if err := runEvaluationMode(modeArgs); err == nil {
				t.Fatalf("%s accepted run ID %q", modeArgs[1], runID)
			}
		}
		if data, err := os.ReadFile(campaign); err != nil || string(data) != string(campaignData) {
			t.Fatalf("run ID %q changed campaign: %v", runID, err)
		}
	}
	for _, name := range []string{"...json", "..json"} {
		if _, err := os.Stat(filepath.Join(stateDirectory, name)); err != nil {
			t.Fatalf("hostile manifest %q changed: %v", name, err)
		}
	}
}

func lifecycleArgs(root, configPath, casesPath, base, receiptPath string) []string {
	return []string{"-mode", "collect", "-root", root, "-run-id", "development-one", "-receipt", receiptPath, "-cases", casesPath, "-config", configPath, "-candidate-id", "candidate", "-provider", "fixture", "-prompt-version", app.EngineeringInsightPromptVersion(), "-corpus-id", "corpus", "-base-revision", base}
}

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return string(output)
}

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

func TestRunnerUsesActualDevelopmentCorpusOnceWithItsControl(t *testing.T) {
	data, err := os.ReadFile("../../internal/app/testdata/engineering-insight-eval/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	cases, err := runnerCasesForMode(data, app.EngineeringInsightDevelopmentRunMode)
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != 3 {
		t.Fatalf("development cases = %d, want 3", len(cases))
	}
	controls := 0
	for _, item := range cases {
		if item.Expected.Intent == "control" {
			controls++
		}
		if item.Expected.Repetition != 1 {
			t.Fatalf("unexpected repetition: %+v", item.Expected)
		}
	}
	if controls != 1 {
		t.Fatalf("controls = %d, want 1", controls)
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
	expected, err := options.expectationFor(app.EngineeringInsightQualificationMode)
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
