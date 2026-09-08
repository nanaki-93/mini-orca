package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

const (
	EngineeringInsightCollectRunMode       = "collect"
	EngineeringInsightDevelopmentRunMode   = "development"
	EngineeringInsightQualificationRunMode = "qualification"
	runnerRelativeDirectory                = ".mini-orca/autopilot/engineering-insight-evaluation"
)

var (
	ErrEngineeringInsightRunLocked    = errors.New("engineering insight evaluation is already running")
	ErrEngineeringInsightRunFinished  = errors.New("engineering insight evaluation run is already complete")
	ErrEngineeringInsightRemoteDenied = errors.New("evaluation remote provider requires explicit confirmation")
)

// EngineeringInsightRunnerCase is deliberately source-bearing only in memory.
// It is never included in a manifest or receipt.
type EngineeringInsightRunnerCase struct {
	Expected EngineeringInsightExpectedAttempt
	Source   string
}

// EngineeringInsightRunnerClient is the single request boundary used by the
// runner. The production client executes one transport attempt per call.
type EngineeringInsightRunnerClient interface {
	Chat(context.Context, []llm.ChatMessage) (*llm.ChatResponse, error)
}

type EngineeringInsightRunnerConfig struct {
	Root                  string
	RunID                 string
	Mode                  string
	CandidateID           string
	Provider              string
	Model                 string
	PromptVersion         string
	CorpusID              string
	CorpusDigest          string
	BaseRevision          string
	Profile               config.ModelProfile
	ConfirmRemoteProvider bool
	Cases                 []EngineeringInsightRunnerCase
	Client                EngineeringInsightRunnerClient
}

// EngineeringInsightScoringHandoff keeps replies private for an independent
// reviewer. Call Discard when scoring is complete; no reply is persisted.
type EngineeringInsightScoringHandoff struct {
	RunID      string
	Responses  map[string]string
	privateDir string
}

func (handoff *EngineeringInsightScoringHandoff) Discard() {
	for key := range handoff.Responses {
		handoff.Responses[key] = ""
		delete(handoff.Responses, key)
	}
	if validRunnerID(handoff.RunID) && handoff.privateDir != "" {
		_ = os.RemoveAll(handoff.privateDir)
	}
}

type engineeringInsightRunManifest struct {
	Fingerprint string                              `json:"fingerprint"`
	Finished    bool                                `json:"finished"`
	Receipt     EngineeringInsightEvaluationReceipt `json:"receipt"`
}

type engineeringInsightCampaign struct {
	DevelopmentRequests   int `json:"development_requests"`
	QualificationRequests int `json:"qualification_requests"`
}

// RunEngineeringInsightEvaluation executes the selected schedule sequentially.
// Each reservation is durable before Chat, so a process crash consumes that
// slot and resume never repeats an uncertain provider request.
func RunEngineeringInsightEvaluation(ctx context.Context, cfg EngineeringInsightRunnerConfig) (EngineeringInsightEvaluationReceipt, *EngineeringInsightScoringHandoff, error) {
	if err := validateRunnerConfig(cfg); err != nil {
		return EngineeringInsightEvaluationReceipt{}, nil, err
	}
	if err := validateRunnerDestination(cfg); err != nil {
		return EngineeringInsightEvaluationReceipt{}, nil, err
	}
	directory := filepath.Join(cfg.Root, runnerRelativeDirectory)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return EngineeringInsightEvaluationReceipt{}, nil, fmt.Errorf("create evaluation storage")
	}
	campaignLock, err := lockRunnerFile(filepath.Join(directory, "campaign.lock"))
	if err != nil {
		return EngineeringInsightEvaluationReceipt{}, nil, err
	}
	defer campaignLock.Close()
	if err := ensureEvaluationCampaign(directory); err != nil {
		return EngineeringInsightEvaluationReceipt{}, nil, err
	}
	lock, err := lockRunnerFile(filepath.Join(directory, cfg.RunID+".lock"))
	if err != nil {
		return EngineeringInsightEvaluationReceipt{}, nil, err
	}
	defer lock.Close()

	manifestPath := filepath.Join(directory, cfg.RunID+".json")
	fingerprint := runnerFingerprint(cfg)
	manifest, exists, err := loadRunManifest(manifestPath)
	if err != nil {
		return EngineeringInsightEvaluationReceipt{}, nil, err
	}
	if exists {
		if manifest.Fingerprint != fingerprint || validateRunnerManifest(manifest, cfg) != nil {
			return EngineeringInsightEvaluationReceipt{}, nil, fmt.Errorf("evaluation run configuration changed")
		}
		if manifest.Finished {
			return manifest.Receipt, nil, ErrEngineeringInsightRunFinished
		}
	} else {
		manifest = engineeringInsightRunManifest{Fingerprint: fingerprint, Receipt: runnerReceipt(cfg)}
		if err := writeRunnerJSON(manifestPath, manifest); err != nil {
			return EngineeringInsightEvaluationReceipt{}, nil, err
		}
	}

	handoff := &EngineeringInsightScoringHandoff{RunID: cfg.RunID, Responses: make(map[string]string), privateDir: privateResponseDirectory(directory, cfg.RunID)}
	if err := executeRemainingAttempts(ctx, cfg, directory, manifestPath, &manifest, handoff); err != nil {
		return manifest.Receipt, handoff, err
	}
	manifest.Finished = true
	if err := writeRunnerJSON(manifestPath, manifest); err != nil {
		return manifest.Receipt, handoff, err
	}
	return manifest.Receipt, handoff, nil
}

func validateRunnerDestination(cfg EngineeringInsightRunnerConfig) error {
	if !isLoopbackURL(cfg.Profile.APIBaseURL) && !cfg.ConfirmRemoteProvider {
		return ErrEngineeringInsightRemoteDenied
	}
	return nil
}

func executeRemainingAttempts(ctx context.Context, cfg EngineeringInsightRunnerConfig, directory, manifestPath string, manifest *engineeringInsightRunManifest, handoff *EngineeringInsightScoringHandoff) error {
	for index, item := range cfg.Cases {
		if index < len(manifest.Receipt.Attempts) {
			continue
		}
		if err := reserveRunnerAttempt(directory, cfg.Mode); err != nil {
			return err
		}
		// A reservation is recorded as unknown before prompt delivery. If this
		// process dies after this write, resume leaves the consumed slot intact.
		manifest.Receipt.Attempts = append(manifest.Receipt.Attempts, unknownReservedAttempt(item.Expected, cfg.CandidateID))
		manifest.Receipt.Consumption.Requests++
		if err := writeRunnerJSON(manifestPath, manifest); err != nil {
			return err
		}

		attempt, response, err := executeRunnerAttempt(ctx, cfg, item)
		if err != nil {
			attempt = failedRunnerAttempt(item.Expected, cfg.CandidateID, err)
		}
		manifest.Receipt.Attempts[index] = attempt
		manifest.Receipt.Consumption.OutputTokens += attempt.OutputTokens
		if response != "" {
			if err := writePrivateResponse(handoff.privateDir, expectedAttemptID(item.Expected), response); err != nil {
				return err
			}
		}
		if err := writeRunnerJSON(manifestPath, manifest); err != nil {
			return err
		}
		if response != "" {
			handoff.Responses[expectedAttemptID(item.Expected)] = response
		}
	}
	return nil
}

// StoreEngineeringInsightScores joins independent scores to the durable receipt
// by response digest. It never accepts or persists reviewer prose.
func StoreEngineeringInsightScores(root, runID string, scores map[string]EngineeringInsightAttemptScore) (EngineeringInsightEvaluationReceipt, error) {
	if !validRunnerID(runID) {
		return EngineeringInsightEvaluationReceipt{}, fmt.Errorf("evaluation run ID is invalid")
	}
	if _, err := LoadEngineeringInsightScoringHandoff(root, runID); err != nil {
		return EngineeringInsightEvaluationReceipt{}, err
	}
	directory := filepath.Join(root, runnerRelativeDirectory)
	lock, err := lockRunnerFile(filepath.Join(directory, runID+".lock"))
	if err != nil {
		return EngineeringInsightEvaluationReceipt{}, err
	}
	defer lock.Close()
	path := filepath.Join(directory, runID+".json")
	manifest, exists, err := loadRunManifest(path)
	if err != nil || !exists || !manifest.Finished {
		return EngineeringInsightEvaluationReceipt{}, fmt.Errorf("evaluation run is not ready for scoring")
	}
	if err := applyEngineeringInsightScores(&manifest.Receipt, scores); err != nil {
		return EngineeringInsightEvaluationReceipt{}, err
	}
	if err := writeRunnerJSON(path, manifest); err != nil {
		return EngineeringInsightEvaluationReceipt{}, err
	}
	return manifest.Receipt, nil
}

func applyEngineeringInsightScores(receipt *EngineeringInsightEvaluationReceipt, scores map[string]EngineeringInsightAttemptScore) error {
	byID := make(map[string]*EngineeringInsightEvaluationAttempt, len(receipt.Attempts))
	for index := range receipt.Attempts {
		attempt := &receipt.Attempts[index]
		byID[attemptScoreID(*attempt)] = attempt
	}
	for id, score := range scores {
		attempt, exists := byID[id]
		if !exists {
			return fmt.Errorf("evaluation score is outside the selected schedule")
		}
		if !attempt.EmittedResponse || score.ResponseDigest != attempt.ResponseDigest || !validScore(score) {
			return fmt.Errorf("evaluation score does not match a retained response")
		}
		copy := score
		attempt.Score = &copy
	}
	return nil
}

func attemptScoreID(attempt EngineeringInsightEvaluationAttempt) string {
	return expectedAttemptID(EngineeringInsightExpectedAttempt{CaseName: attempt.CaseName, Partition: attempt.Partition, Intent: attempt.Intent, Repetition: attempt.Repetition, Attempt: attempt.Attempt})
}

// LoadEngineeringInsightScoringHandoff reads private replies for a completed
// run. The caller must discard it after scoring; this never touches receipts.
func LoadEngineeringInsightScoringHandoff(root, runID string) (*EngineeringInsightScoringHandoff, error) {
	if !validRunnerID(runID) {
		return nil, fmt.Errorf("evaluation run ID is invalid")
	}
	directory := filepath.Join(root, runnerRelativeDirectory)
	manifest, exists, err := loadRunManifest(filepath.Join(directory, runID+".json"))
	if err != nil || !exists || !manifest.Finished {
		return nil, fmt.Errorf("evaluation run is not ready for scoring")
	}
	handoff := &EngineeringInsightScoringHandoff{RunID: runID, Responses: make(map[string]string), privateDir: privateResponseDirectory(directory, runID)}
	for _, attempt := range manifest.Receipt.Attempts {
		if !attempt.EmittedResponse {
			continue
		}
		id := expectedAttemptID(EngineeringInsightExpectedAttempt{CaseName: attempt.CaseName, Partition: attempt.Partition, Intent: attempt.Intent, Repetition: attempt.Repetition, Attempt: attempt.Attempt})
		response, err := readPrivateResponse(handoff.privateDir, id)
		if err != nil {
			return nil, fmt.Errorf("read private evaluation response")
		}
		digest := sha256.Sum256([]byte(response))
		if hex.EncodeToString(digest[:]) != attempt.ResponseDigest {
			return nil, fmt.Errorf("private evaluation response does not match receipt")
		}
		handoff.Responses[id] = response
	}
	return handoff, nil
}

// LoadEngineeringInsightEvaluationReceipt exports only the source-free receipt.
func LoadEngineeringInsightEvaluationReceipt(root, runID string) (EngineeringInsightEvaluationReceipt, error) {
	if !validRunnerID(runID) {
		return EngineeringInsightEvaluationReceipt{}, fmt.Errorf("evaluation run ID is invalid")
	}
	manifest, exists, err := loadRunManifest(filepath.Join(root, runnerRelativeDirectory, runID+".json"))
	if err != nil || !exists || !manifest.Finished {
		return EngineeringInsightEvaluationReceipt{}, fmt.Errorf("evaluation run is not ready for export")
	}
	return manifest.Receipt, nil
}

func validateRunnerConfig(cfg EngineeringInsightRunnerConfig) error {
	if !validRunnerIdentity(cfg) {
		return fmt.Errorf("evaluation runner configuration is incomplete")
	}
	if !validRunnerMode(cfg.Mode) {
		return fmt.Errorf("evaluation runner mode is invalid")
	}
	if cfg.PromptVersion != semanticAnalysisPromptVersion {
		return fmt.Errorf("evaluation runner prompt identity is invalid")
	}
	if cfg.Mode == EngineeringInsightQualificationRunMode && len(cfg.Cases) != qualificationRequestCap {
		return fmt.Errorf("qualification runner requires 24 attempts")
	}
	if cfg.Mode != EngineeringInsightQualificationRunMode && len(cfg.Cases) > 6 {
		return fmt.Errorf("development runner exceeds its request budget")
	}
	if !validRunnerCases(cfg.Cases) {
		return fmt.Errorf("evaluation runner schedule is invalid")
	}
	if cfg.Mode == EngineeringInsightQualificationRunMode {
		schedule := make([]EngineeringInsightExpectedAttempt, len(cfg.Cases))
		for index, item := range cfg.Cases {
			schedule[index] = item.Expected
		}
		if err := validateQualificationSchedule(schedule); err != nil {
			return fmt.Errorf("qualification runner schedule is invalid")
		}
	} else if !validDevelopmentRunnerSchedule(cfg.Cases, cfg.Mode) {
		return fmt.Errorf("development runner schedule is invalid")
	}
	return nil
}

func validRunnerIdentity(cfg EngineeringInsightRunnerConfig) bool {
	return strings.TrimSpace(cfg.Root) != "" && validRunnerID(cfg.RunID) && cfg.Client != nil && cfg.Profile.Scope == config.BugModelScope && len(cfg.Cases) != 0
}

// ValidEngineeringInsightEvaluationRunID reports whether a run identifier can be
// used as a single private-state file name.
func ValidEngineeringInsightEvaluationRunID(runID string) bool {
	return validRunnerID(runID)
}

func validRunnerID(runID string) bool {
	if len(runID) == 0 || len(runID) > 96 || !asciiLetterOrDigit(runID[0]) {
		return false
	}
	for index := 1; index < len(runID); index++ {
		if !asciiLetterOrDigit(runID[index]) && runID[index] != '-' && runID[index] != '_' {
			return false
		}
	}
	return true
}
func asciiLetterOrDigit(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}
func validRunnerMode(mode string) bool {
	return mode == EngineeringInsightCollectRunMode || mode == EngineeringInsightDevelopmentRunMode || mode == EngineeringInsightQualificationRunMode
}

func validRunnerCases(cases []EngineeringInsightRunnerCase) bool {
	seen := make(map[string]bool, len(cases))
	for _, item := range cases {
		id := expectedAttemptID(item.Expected)
		if seen[id] || !validExpectedAttempt(item.Expected) || strings.TrimSpace(item.Source) == "" {
			return false
		}
		seen[id] = true
	}
	return true
}

func validDevelopmentRunnerSchedule(cases []EngineeringInsightRunnerCase, mode string) bool {
	wantRepetitions := 1
	if len(cases) != 3*wantRepetitions {
		return false
	}
	seen := make(map[string]map[int]bool, 3)
	for _, item := range cases {
		attempt := item.Expected
		if attempt.Partition != "development" || (attempt.Intent != "substantive" && attempt.Intent != "control") || attempt.Attempt != 1 || attempt.Repetition != 1 {
			return false
		}
		if seen[attempt.CaseName] == nil {
			seen[attempt.CaseName] = make(map[int]bool, wantRepetitions)
		}
		if seen[attempt.CaseName][attempt.Repetition] {
			return false
		}
		seen[attempt.CaseName][attempt.Repetition] = true
	}
	if len(seen) != 3 {
		return false
	}
	for _, repetitions := range seen {
		if len(repetitions) != wantRepetitions {
			return false
		}
	}
	return true
}

func runnerReceipt(cfg EngineeringInsightRunnerConfig) EngineeringInsightEvaluationReceipt {
	mode := EngineeringInsightCollectionMode
	maxRequests := 6
	if cfg.Mode == EngineeringInsightQualificationRunMode {
		mode, maxRequests = EngineeringInsightQualificationMode, qualificationRequestCap
	}
	return EngineeringInsightEvaluationReceipt{Mode: mode, RunID: cfg.RunID, CandidateID: cfg.CandidateID, Provider: cfg.Provider, Model: cfg.Model, PromptVersion: cfg.PromptVersion, CorpusID: cfg.CorpusID, CorpusDigest: cfg.CorpusDigest, BaseRevision: cfg.BaseRevision, MaxRequests: maxRequests, MaxOutputTokens: qualificationOutputTokenCap, AttemptTimeoutSeconds: qualificationAttemptTimeoutSeconds}
}

func runnerFingerprint(cfg EngineeringInsightRunnerConfig) string {
	// This private digest binds endpoint, credentials, context limit and the
	// immutable public identity without ever writing those values to a receipt.
	values := []string{cfg.Mode, cfg.CandidateID, cfg.Provider, cfg.Model, cfg.PromptVersion, cfg.CorpusID, cfg.CorpusDigest, cfg.BaseRevision, string(cfg.Profile.Scope), cfg.Profile.APIBaseURL, cfg.Profile.APIKey, cfg.Profile.Model, cfg.Profile.ReasoningEffort, fmt.Sprint(cfg.Profile.Temperature), fmt.Sprint(cfg.Profile.ContextMaxTokens)}
	for _, item := range cfg.Cases {
		source := sha256.Sum256([]byte(item.Source))
		values = append(values, expectedAttemptID(item.Expected), hex.EncodeToString(source[:]))
	}
	value := strings.Join(values, "\x00")
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func reserveRunnerAttempt(directory, mode string) error {
	path := filepath.Join(directory, "campaign.json")
	campaign, err := loadEvaluationCampaign(path)
	if err != nil {
		return fmt.Errorf("read evaluation campaign")
	}
	if mode == EngineeringInsightQualificationRunMode {
		if campaign.QualificationRequests >= qualificationRequestCap {
			return fmt.Errorf("qualification request budget is exhausted")
		}
		campaign.QualificationRequests++
	} else {
		if campaign.DevelopmentRequests >= 6 {
			return fmt.Errorf("development request budget is exhausted")
		}
		campaign.DevelopmentRequests++
	}
	return writeRunnerJSON(path, campaign)
}

func ensureEvaluationCampaign(directory string) error {
	path := filepath.Join(directory, "campaign.json")
	if _, err := loadEvaluationCampaign(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read evaluation campaign")
	}
	runs, err := filepath.Glob(filepath.Join(directory, "*.json"))
	if err != nil || len(runs) != 0 {
		return fmt.Errorf("read evaluation campaign")
	}
	return writeRunnerJSON(path, engineeringInsightCampaign{})
}

func loadEvaluationCampaign(path string) (engineeringInsightCampaign, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return engineeringInsightCampaign{}, err
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil || len(fields) != 2 || fields["development_requests"] == nil || fields["qualification_requests"] == nil {
		return engineeringInsightCampaign{}, fmt.Errorf("invalid campaign")
	}
	var campaign engineeringInsightCampaign
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&campaign) != nil || campaign.DevelopmentRequests < 0 || campaign.DevelopmentRequests > 6 || campaign.QualificationRequests < 0 || campaign.QualificationRequests > qualificationRequestCap {
		return engineeringInsightCampaign{}, fmt.Errorf("invalid campaign")
	}
	return campaign, nil
}

func unknownReservedAttempt(expected EngineeringInsightExpectedAttempt, candidate string) EngineeringInsightEvaluationAttempt {
	return EngineeringInsightEvaluationAttempt{CaseName: expected.CaseName, Partition: expected.Partition, Intent: expected.Intent, Repetition: expected.Repetition, Attempt: expected.Attempt, CandidateID: candidate, Outcome: "unknown", OptionalInsight: "not_evaluated", FinishReason: "unknown"}
}

func executeRunnerAttempt(parent context.Context, cfg EngineeringInsightRunnerConfig, item EngineeringInsightRunnerCase) (EngineeringInsightEvaluationAttempt, string, error) {
	root, prompt, target, err := prepareRunnerPrompt(cfg, item)
	if err != nil {
		return EngineeringInsightEvaluationAttempt{}, "", err
	}
	defer os.RemoveAll(root)
	return dispatchRunnerPrompt(parent, cfg, item, prompt, target)
}

func prepareRunnerPrompt(cfg EngineeringInsightRunnerConfig, item EngineeringInsightRunnerCase) (string, string, *project.IndexFile, error) {
	root, err := os.MkdirTemp("", "mini-orca-insight-fixture-")
	if err != nil {
		return "", "", nil, fmt.Errorf("create private evaluation fixture")
	}
	fail := func(err error) (string, string, *project.IndexFile, error) {
		_ = os.RemoveAll(root)
		return "", "", nil, err
	}
	if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte(item.Source), 0600); err != nil {
		return fail(fmt.Errorf("write private evaluation fixture"))
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module evaluationfixture\n\ngo 1.22\n"), 0600); err != nil {
		return fail(fmt.Errorf("write private evaluation fixture"))
	}
	if err := compileRunnerFixture(root); err != nil {
		return fail(err)
	}
	manager, err := project.NewManager(root)
	if err != nil {
		return fail(err)
	}
	analysis := project.Analysis{Name: "evaluation", Type: "go", Path: root}
	if err := manager.Set(root, &analysis); err != nil {
		return fail(err)
	}
	index, err := manager.Index()
	if err != nil {
		return fail(err)
	}
	target, err := manager.IndexedFile("sample.go")
	if err != nil {
		return fail(err)
	}
	model := EffectiveModel{Scope: string(cfg.Profile.Scope), Model: cfg.Profile.Model, ProviderOrigin: providerOrigin(cfg.Profile.APIBaseURL), RemoteProvider: !isLoopbackURL(cfg.Profile.APIBaseURL), ContextMaxTokens: cfg.Profile.ContextMaxTokens}
	prompt, err := semanticPrompt(item.Source, analysis, index, *target, bindContextManifestModel(semanticManifest(*target), model))
	if err != nil {
		return fail(err)
	}
	return root, prompt, target, nil
}

func dispatchRunnerPrompt(parent context.Context, cfg EngineeringInsightRunnerConfig, item EngineeringInsightRunnerCase, prompt string, target *project.IndexFile) (EngineeringInsightEvaluationAttempt, string, error) {
	timed, cancel := context.WithTimeout(parent, qualificationAttemptTimeoutSeconds*time.Second)
	defer cancel()
	started := time.Now()
	response, err := cfg.Client.Chat(timed, []llm.ChatMessage{{Role: "user", Content: prompt}})
	elapsed := time.Since(started).Milliseconds()
	if err != nil {
		return failedRunnerAttemptWithElapsed(item.Expected, cfg.CandidateID, err, elapsed), "", nil
	}
	content := response.Choices[0].Message.Content
	digest := sha256.Sum256([]byte(content))
	attempt := EngineeringInsightEvaluationAttempt{CaseName: item.Expected.CaseName, Partition: item.Expected.Partition, Intent: item.Expected.Intent, Repetition: item.Expected.Repetition, Attempt: item.Expected.Attempt, CandidateID: cfg.CandidateID, EmittedResponse: true, ResponseDigest: hex.EncodeToString(digest[:]), OutputTokens: response.Usage.CompletionTokens, ElapsedMilliseconds: float64(elapsed), OptionalInsight: "not_evaluated"}
	if attempt.OutputTokens < 0 || attempt.OutputTokens > qualificationOutputTokenCap || response.Model != "" && response.Model != cfg.Model {
		attempt.Outcome, attempt.FinishReason = "failed", "error"
		attempt.InvalidProviderMetadata = true
		return attempt, content, nil
	}
	if response.Choices[0].FinishReason == "length" {
		attempt.Outcome, attempt.FinishReason = "truncated", "length"
		return attempt, content, nil
	}
	if response.Choices[0].FinishReason != "" && response.Choices[0].FinishReason != "stop" {
		attempt.Outcome, attempt.FinishReason = "failed", "error"
		return attempt, content, nil
	}
	parsed, parseErr := parseSemanticAnalysis(content, *target, item.Source)
	if parseErr != nil {
		attempt.Outcome, attempt.FinishReason = "malformed", "stop"
		return attempt, content, nil
	}
	attempt.Outcome, attempt.FinishReason, attempt.UsableSummary = "completed", "stop", true
	attempt.OptionalInsight, attempt.OptionalSectionDegraded = evaluationOptionalState(content, *target, item.Source, parsed)
	attempt.CompleteSummary = !attempt.OptionalSectionDegraded
	return attempt, content, nil
}

func compileRunnerFixture(root string) error {
	command := exec.Command("go", "test", "./...")
	command.Dir = root
	command.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off")
	if err := command.Run(); err != nil {
		return fmt.Errorf("compile private evaluation fixture")
	}
	return nil
}

func failedRunnerAttempt(expected EngineeringInsightExpectedAttempt, candidate string, cause error) EngineeringInsightEvaluationAttempt {
	return failedRunnerAttemptWithElapsed(expected, candidate, cause, 0)
}

func failedRunnerAttemptWithElapsed(expected EngineeringInsightExpectedAttempt, candidate string, cause error, elapsed int64) EngineeringInsightEvaluationAttempt {
	attempt := unknownReservedAttempt(expected, candidate)
	attempt.ElapsedMilliseconds = float64(elapsed)
	if errors.Is(cause, context.DeadlineExceeded) {
		attempt.Outcome, attempt.FinishReason = "timeout", "timeout"
	} else if errors.Is(cause, context.Canceled) {
		attempt.Outcome, attempt.FinishReason = "failed", "canceled"
	} else {
		attempt.Outcome, attempt.FinishReason = "failed", "error"
	}
	return attempt
}

func evaluationOptionalState(content string, target project.IndexFile, source string, parsed semanticAnalysisResponse) (string, bool) {
	var wire semanticAnalysisWireResponse
	if decodeSemanticAnalysis(content, &wire) != nil {
		return "rejected", true
	}
	degraded := false
	if _, err := normalizeSymbolExplanations(wire.SymbolExplanations, target.Symbols); err != nil {
		degraded = true
	}
	insight, insightError := project.ParseOptionalEngineeringInsight(wire.EngineeringInsight)
	optional := "omitted"
	if insightError != "" {
		optional, degraded = "rejected", true
	} else if insight != nil || parsed.EngineeringInsight != nil {
		optional = "present"
	}
	for _, risk := range wire.Risks {
		optional, degraded = mergeOptionalState(optional, degraded, riskOptionalState(risk, target, source))
	}
	for _, suggestion := range wire.Suggestions {
		optional, degraded = mergeOptionalState(optional, degraded, suggestionOptionalState(suggestion))
	}
	return optional, degraded
}

type optionalState struct{ present, rejected, degraded bool }

func riskOptionalState(risk semanticAnalysisFinding, target project.IndexFile, source string) optionalState {
	state := optionalInsightState(risk.Insight)
	if len(risk.TaskSpec) > 0 && strings.TrimSpace(string(risk.TaskSpec)) != "null" && parseOptionalBugTaskSpec(risk.TaskSpec, target, source) == nil {
		state.degraded = true
	}
	return state
}
func suggestionOptionalState(suggestion semanticAnalysisSuggestion) optionalState {
	return optionalInsightState(suggestion.Insight)
}
func optionalInsightState(raw json.RawMessage) optionalState {
	insight, reason := project.ParseOptionalEngineeringInsight(raw)
	return optionalState{present: insight != nil, rejected: reason != "", degraded: reason != ""}
}
func mergeOptionalState(optional string, degraded bool, state optionalState) (string, bool) {
	if state.rejected {
		optional = "rejected"
	} else if state.present && optional != "rejected" {
		optional = "present"
	}
	return optional, degraded || state.degraded
}

func loadRunManifest(path string) (engineeringInsightRunManifest, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return engineeringInsightRunManifest{}, false, nil
	}
	if err != nil || json.Unmarshal(data, &engineeringInsightRunManifest{}) != nil {
		return engineeringInsightRunManifest{}, false, fmt.Errorf("read evaluation run")
	}
	var manifest engineeringInsightRunManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return engineeringInsightRunManifest{}, false, fmt.Errorf("read evaluation run")
	}
	return manifest, true, nil
}

func validateRunnerManifest(manifest engineeringInsightRunManifest, cfg EngineeringInsightRunnerConfig) error {
	want := runnerReceipt(cfg)
	if !sameRunnerReceiptIdentity(manifest.Receipt, want) || len(manifest.Receipt.Attempts) > len(cfg.Cases) || manifest.Receipt.Consumption.Requests != len(manifest.Receipt.Attempts) {
		return fmt.Errorf("invalid evaluation manifest")
	}
	if !validManifestAttempts(manifest.Receipt, cfg.Cases) {
		return fmt.Errorf("invalid evaluation manifest")
	}
	return nil
}

func sameRunnerReceiptIdentity(actual, expected EngineeringInsightEvaluationReceipt) bool {
	return actual.Mode == expected.Mode && actual.RunID == expected.RunID && actual.CandidateID == expected.CandidateID && actual.Provider == expected.Provider && actual.Model == expected.Model && actual.PromptVersion == expected.PromptVersion && actual.CorpusID == expected.CorpusID && actual.CorpusDigest == expected.CorpusDigest && actual.BaseRevision == expected.BaseRevision && actual.MaxRequests == expected.MaxRequests && actual.MaxOutputTokens == expected.MaxOutputTokens && actual.AttemptTimeoutSeconds == expected.AttemptTimeoutSeconds
}

func validManifestAttempts(receipt EngineeringInsightEvaluationReceipt, cases []EngineeringInsightRunnerCase) bool {
	tokens := 0
	for index, attempt := range receipt.Attempts {
		actual := EngineeringInsightExpectedAttempt{CaseName: attempt.CaseName, Partition: attempt.Partition, Intent: attempt.Intent, Repetition: attempt.Repetition, Attempt: attempt.Attempt}
		if expectedAttemptID(cases[index].Expected) != expectedAttemptID(actual) || validateAttempt(attempt, EngineeringInsightCollectionMode) != nil {
			return false
		}
		tokens += attempt.OutputTokens
	}
	return tokens == receipt.Consumption.OutputTokens
}

func privateResponseDirectory(directory, runID string) string {
	return filepath.Join(directory, "responses", runID)
}

func privateResponsePath(directory, id string) string {
	sum := sha256.Sum256([]byte(id))
	return filepath.Join(directory, hex.EncodeToString(sum[:])+".response")
}

func writePrivateResponse(directory, id, response string) error {
	if err := os.MkdirAll(directory, 0700); err != nil {
		return fmt.Errorf("write private evaluation response")
	}
	if err := os.WriteFile(privateResponsePath(directory, id), []byte(response), 0600); err != nil {
		return fmt.Errorf("write private evaluation response")
	}
	return nil
}

func readPrivateResponse(directory, id string) (string, error) {
	data, err := os.ReadFile(privateResponsePath(directory, id))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func writeRunnerJSON(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("write evaluation run")
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".evaluation-*")
	if err != nil {
		return fmt.Errorf("write evaluation run")
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write evaluation run")
	}
	if _, err := temporary.Write(data); err != nil || temporary.Sync() != nil || temporary.Close() != nil {
		return fmt.Errorf("write evaluation run")
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return fmt.Errorf("write evaluation run")
	}
	return nil
}
