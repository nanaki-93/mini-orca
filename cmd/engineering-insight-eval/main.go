// Command engineering-insight-eval validates source-free collection and
// qualification receipts. It never contacts a provider.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

func main() {
	if evaluationRunModeRequested(os.Args[1:]) {
		if err := runEvaluationMode(os.Args[1:]); err != nil {
			fail(err)
		}
		return
	}
	options, err := evaluationOptionsFromFlags()
	if err != nil {
		failWithStatus(err, 2)
	}
	receipt, err := loadReceipt(options.receiptPath)
	if err != nil {
		fail(err)
	}
	expected, err := options.expectationFor(receipt.Mode)
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

func evaluationRunModeRequested(args []string) bool {
	for index, arg := range args {
		if strings.HasPrefix(arg, "-mode=") || (arg == "-mode" && index+1 < len(args)) {
			return true
		}
	}
	return false
}

func runEvaluationMode(args []string) error {
	flags := flag.NewFlagSet("engineering-insight-eval", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	mode := flags.String("mode", "", "collect, development, qualification, grant-development, handoff, score, export, or discard")
	root := flags.String("root", ".", "project root containing private evaluation state")
	runID := flags.String("run-id", "", "new or resumable evaluation run identity")
	receiptPath := flags.String("receipt", "", "source-free receipt destination")
	casesPath := flags.String("cases", "", "evaluation case set")
	configPath := flags.String("config", "config.yaml", "configured local daemon model profile")
	candidateID := flags.String("candidate-id", "", "selected candidate identity")
	provider := flags.String("provider", "", "selected provider label without endpoint or credential")
	model := flags.String("model", "", "selected model identity for a development grant")
	promptVersion := flags.String("prompt-version", "", "selected prompt identity")
	corpusID := flags.String("corpus-id", "", "selected corpus identity")
	baseRevision := flags.String("base-revision", "", "selected base revision")
	confirmRemote := flags.Bool("confirm-remote-provider", false, "confirm this exact configured remote destination")
	scoresPath := flags.String("scores", "", "digest-bound score JSON for score mode")
	authorizationID := flags.String("authorization-id", "", "unique explicit development-budget authorization")
	requests := flags.Int("requests", 0, "explicit development-budget grant request count")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("invalid evaluation command")
	}
	if *mode == "grant-development" {
		switch {
		case *authorizationID == "qual05-qwen38-thinking-off-1":
			return app.GrantEngineeringInsightThinkingOffDevelopmentBudget(*root, *authorizationID, *requests, *candidateID, *model, *promptVersion)
		case *authorizationID == "authqual05-qwen38-structured-1":
			return app.GrantEngineeringInsightV12DevelopmentBudget(*root, *authorizationID, *requests, *candidateID, *model, *promptVersion)
		case *authorizationID == "qual05-qwen38-schema-1":
			return app.GrantEngineeringInsightV11DevelopmentBudget(*root, *authorizationID, *requests, *candidateID, *model, *promptVersion)
		case *authorizationID == "qual05-qwen38-recovery-1":
			return app.GrantEngineeringInsightRecoveryDevelopmentBudget(*root, *authorizationID, *requests, *candidateID, *model)
		case *candidateID == "" && *model == "" && *promptVersion == "":
			return app.GrantEngineeringInsightDevelopmentBudget(*root, *authorizationID, *requests)
		default:
			return fmt.Errorf("development budget grant identity is invalid")
		}
	}
	if !app.ValidEngineeringInsightEvaluationRunID(*runID) {
		return fmt.Errorf("evaluation run ID is invalid")
	}
	if handled, err := runAuxiliaryEvaluationMode(*mode, *root, *runID, *receiptPath, *scoresPath); handled {
		return err
	}
	return runProviderEvaluation(providerEvaluationOptions{mode: *mode, root: *root, runID: *runID, receiptPath: *receiptPath, casesPath: *casesPath, configPath: *configPath, candidateID: *candidateID, provider: *provider, promptVersion: *promptVersion, corpusID: *corpusID, baseRevision: *baseRevision, confirmRemote: *confirmRemote})
}

type providerEvaluationOptions struct {
	mode, root, runID, receiptPath, casesPath, configPath, candidateID, provider, promptVersion, corpusID, baseRevision string
	confirmRemote                                                                                                       bool
}

func runProviderEvaluation(options providerEvaluationOptions) error {
	if !validProviderEvaluationOptions(options) {
		return fmt.Errorf("mode, root, run-id, receipt, cases, config, candidate-id, provider, prompt-version, corpus-id, and base-revision are required")
	}
	profile, err := evaluationBugProfile(options.configPath)
	if err != nil {
		return err
	}
	actualBase, err := evaluationBaseRevision(options.root)
	if err != nil || actualBase != options.baseRevision {
		return fmt.Errorf("evaluation base revision is not the clean current candidate")
	}
	casesData, err := os.ReadFile(options.casesPath)
	if err != nil {
		return fmt.Errorf("read evaluation cases")
	}
	runnerCases, err := runnerCasesForMode(casesData, options.mode)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(casesData)
	receipt, _, err := app.RunEngineeringInsightEvaluation(context.Background(), app.EngineeringInsightRunnerConfig{Root: options.root, RunID: options.runID, Mode: options.mode, CandidateID: options.candidateID, Provider: options.provider, Model: profile.Model, PromptVersion: options.promptVersion, CorpusID: options.corpusID, CorpusDigest: hex.EncodeToString(digest[:]), BaseRevision: options.baseRevision, Profile: profile, ConfirmRemoteProvider: options.confirmRemote, Cases: runnerCases, Client: llm.NewEvaluationClient(profile)})
	if err != nil {
		return err
	}
	return writeEvaluationReceipt(options.receiptPath, receipt)
}

func validProviderEvaluationOptions(options providerEvaluationOptions) bool {
	if !validRunnerModeForCommand(options.mode) || options.promptVersion != app.EngineeringInsightPromptVersion() {
		return false
	}
	for _, value := range []string{options.root, options.runID, options.receiptPath, options.casesPath, options.configPath, options.candidateID, options.provider, options.corpusID, options.baseRevision} {
		if value == "" {
			return false
		}
	}
	return true
}
func validRunnerModeForCommand(mode string) bool {
	return mode == app.EngineeringInsightCollectRunMode || mode == app.EngineeringInsightDevelopmentRunMode || mode == app.EngineeringInsightQualificationRunMode
}

func runAuxiliaryEvaluationMode(mode, root, runID, receiptPath, scoresPath string) (bool, error) {
	if mode == "score" {
		return true, scoreEvaluationRun(root, runID, receiptPath, scoresPath)
	}
	if mode == "export" {
		return true, exportEvaluationRun(root, runID, receiptPath)
	}
	if mode != "handoff" && mode != "discard" {
		return false, nil
	}
	if runID == "" {
		return true, fmt.Errorf("run-id is required")
	}
	handoff, err := app.LoadEngineeringInsightScoringHandoff(root, runID)
	if err != nil {
		return true, err
	}
	if mode == "discard" {
		handoff.Discard()
		return true, nil
	}
	for id := range handoff.Responses {
		fmt.Println(id)
	}
	return true, nil
}

func scoreEvaluationRun(root, runID, receiptPath, scoresPath string) error {
	if runID == "" || scoresPath == "" || receiptPath == "" {
		return fmt.Errorf("run-id, scores, and receipt are required")
	}
	data, err := os.ReadFile(scoresPath)
	if err != nil {
		return fmt.Errorf("read evaluation scores")
	}
	scores, err := app.DecodeEngineeringInsightScores(data)
	if err != nil {
		return err
	}
	receipt, err := app.StoreEngineeringInsightScores(root, runID, scores)
	if err != nil {
		return err
	}
	return writeEvaluationReceipt(receiptPath, receipt)
}

func exportEvaluationRun(root, runID, receiptPath string) error {
	if runID == "" || receiptPath == "" {
		return fmt.Errorf("run-id and receipt are required")
	}
	receipt, err := app.LoadEngineeringInsightEvaluationReceipt(root, runID)
	if err != nil {
		return err
	}
	return writeEvaluationReceipt(receiptPath, receipt)
}

func evaluationBaseRevision(root string) (string, error) {
	head := exec.Command("git", "-C", root, "rev-parse", "HEAD")
	output, err := head.Output()
	if err != nil {
		return "", err
	}
	status := exec.Command("git", "-C", root, "status", "--porcelain")
	dirty, err := status.Output()
	if err != nil || !permittedEvaluationDirtyState(string(dirty)) {
		return "", fmt.Errorf("candidate is dirty")
	}
	return strings.TrimSpace(string(output)), nil
}

func permittedEvaluationDirtyState(status string) bool {
	status = strings.TrimSpace(status)
	return status == "" || status == "M PLAN.md" || status == " M PLAN.md"
}

func writeEvaluationReceipt(path string, receipt app.EngineeringInsightEvaluationReceipt) error {
	data, err := json.Marshal(receipt)
	if err != nil {
		return fmt.Errorf("write evaluation receipt")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("write evaluation receipt")
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write evaluation receipt")
	}
	return nil
}

func evaluationBugProfile(path string) (config.ModelProfile, error) {
	cfg, err := config.LoadFromYAML(path)
	if err != nil {
		return config.ModelProfile{}, fmt.Errorf("load evaluation configuration")
	}
	profiles, err := config.ResolveModelProfiles(cfg)
	if err != nil {
		return config.ModelProfile{}, fmt.Errorf("load evaluation configuration")
	}
	profile := profiles.Bug
	profile.MaxTokens = 4096
	return profile, nil
}

type runnerCaseDocument struct {
	Name      string `json:"name"`
	Partition string `json:"partition"`
	Intent    string `json:"intent"`
	Source    string `json:"source"`
}

func runnerCasesForMode(data []byte, mode string) ([]app.EngineeringInsightRunnerCase, error) {
	if err := app.ValidateStrictJSONDocument(data); err != nil {
		return nil, fmt.Errorf("invalid evaluation cases")
	}
	var document []runnerCaseDocument
	if json.Unmarshal(data, &document) != nil {
		return nil, fmt.Errorf("invalid evaluation cases")
	}
	selected := make([]runnerCaseDocument, 0, 24)
	for _, item := range document {
		if !validRunnerCaseDocument(item) {
			return nil, fmt.Errorf("invalid evaluation cases")
		}
		if selectsRunnerCase(mode, item.Partition) {
			selected = append(selected, item)
		}
	}
	if mode == app.EngineeringInsightQualificationRunMode {
		if len(selected) != 12 {
			return nil, fmt.Errorf("evaluation cases do not define qualification")
		}
		return repeatedRunnerCases(selected, 2), nil
	}
	if len(selected) != 3 {
		return nil, fmt.Errorf("evaluation cases do not define development")
	}
	return repeatedRunnerCases(selected, 1), nil
}

func validRunnerCaseDocument(item runnerCaseDocument) bool {
	return item.Name != "" && item.Source != "" && validCasePartition(item.Partition) && validCaseIntent(item.Intent)
}
func selectsRunnerCase(mode, partition string) bool {
	if mode == app.EngineeringInsightQualificationRunMode {
		return partition == "qualification"
	}
	return partition == "development"
}

func repeatedRunnerCases(cases []runnerCaseDocument, repetitions int) []app.EngineeringInsightRunnerCase {
	result := make([]app.EngineeringInsightRunnerCase, 0, len(cases)*repetitions)
	for repetition := 1; repetition <= repetitions; repetition++ {
		for _, item := range cases {
			result = append(result, app.EngineeringInsightRunnerCase{Expected: app.EngineeringInsightExpectedAttempt{CaseName: item.Name, Partition: item.Partition, Intent: item.Intent, Repetition: repetition, Attempt: 1}, Source: item.Source})
		}
	}
	return result
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

func (options evaluationOptions) expectationFor(mode app.EngineeringInsightEvaluationMode) (app.EngineeringInsightEvaluationExpectation, error) {
	data, err := os.ReadFile(options.caseSetPath)
	if err != nil {
		return app.EngineeringInsightEvaluationExpectation{}, fmt.Errorf("read evaluation cases")
	}
	schedule, err := qualificationSchedule(data)
	if mode == app.EngineeringInsightCollectionMode {
		schedule, err = developmentSchedule(data)
	}
	if err != nil {
		return app.EngineeringInsightEvaluationExpectation{}, err
	}
	digest := sha256.Sum256(data)
	return app.EngineeringInsightEvaluationExpectation{CandidateID: options.candidateID, Provider: options.provider, Model: options.model, PromptVersion: options.promptVersion, CorpusID: options.corpusID, CorpusDigest: hex.EncodeToString(digest[:]), BaseRevision: options.baseRevision, MaxRequests: options.maxRequests, MaxOutputTokens: options.maxOutputTokens, AttemptTimeoutSeconds: options.attemptTimeoutSeconds, Schedule: schedule}, nil
}

func developmentSchedule(data []byte) ([]app.EngineeringInsightExpectedAttempt, error) {
	if err := app.ValidateStrictJSONDocument(data); err != nil {
		return nil, fmt.Errorf("invalid evaluation cases")
	}
	cases, err := decodeEvaluationCases(data)
	if err != nil {
		return nil, err
	}
	result := make([]app.EngineeringInsightExpectedAttempt, 0, 3)
	seen := map[string]bool{}
	for _, item := range cases {
		if !validEvaluationCase(item, seen) {
			return nil, fmt.Errorf("invalid evaluation cases")
		}
		seen[item.Name] = true
		if item.Partition == "development" {
			result = append(result, app.EngineeringInsightExpectedAttempt{CaseName: item.Name, Partition: item.Partition, Intent: item.Intent, Repetition: 1, Attempt: 1})
		}
	}
	if len(result) != 3 {
		return nil, fmt.Errorf("evaluation cases do not define development schedule")
	}
	return result, nil
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
