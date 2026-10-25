package insighteval

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/storage"
)

const (
	EngineeringInsightCollectRunMode         = "collect"
	EngineeringInsightDevelopmentRunMode     = "development"
	EngineeringInsightQualificationRunMode   = "qualification"
	runnerRelativeDirectory                  = ".mini-orca/autopilot/engineering-insight-evaluation"
	defaultDevelopmentRequestCap             = 6
	extendedDevelopmentRequestCap            = 12
	recoveryDevelopmentRequestCap            = 18
	finalDevelopmentRequestCap               = 24
	structuredDevelopmentRequestCap          = 30
	thinkingOffDevelopmentRequestCap         = 36
	thinkingSchemaDevelopmentRequestCap      = 42
	mediumDevelopmentRequestCap              = 48
	developmentGrantRequestCount             = 6
	recoveryDevelopmentAuthorizationID       = "qual05-qwen38-recovery-1"
	recoveryDevelopmentCandidateID           = "qwen38-v10-recovery-1"
	recoveryDevelopmentModel                 = "qwen/qwen3.8-27b"
	v11DevelopmentAuthorizationID            = "qual05-qwen38-schema-1"
	v11DevelopmentCandidateID                = "qwen38-v11-schema-1"
	v11DevelopmentModel                      = recoveryDevelopmentModel
	v11DevelopmentPromptVersion              = "file-analysis-v11"
	v12DevelopmentAuthorizationID            = "authqual05-qwen38-structured-1"
	v12DevelopmentCandidateID                = "qwen38-v12-structured-1"
	v12DevelopmentModel                      = recoveryDevelopmentModel
	v12DevelopmentPromptVersion              = "file-analysis-v12"
	thinkingOffDevelopmentAuthorizationID    = "qual05-qwen38-thinking-off-1"
	thinkingOffDevelopmentCandidateID        = "qwen38-v12-thinking-off-1"
	thinkingOffDevelopmentModel              = recoveryDevelopmentModel
	thinkingOffDevelopmentPromptVersion      = "file-analysis-v12"
	thinkingOffDevelopmentReasoningEffort    = "none"
	thinkingSchemaDevelopmentAuthorizationID = "qual05-qwen38-thinking-schema-1"
	thinkingSchemaDevelopmentCandidateID     = "qwen38-v12-thinking-schema-1"
	thinkingSchemaDevelopmentModel           = "./models/qwen38-v12-thinking-schema-1"
	thinkingSchemaDevelopmentPromptVersion   = "file-analysis-v12"
	thinkingSchemaDevelopmentReasoningEffort = "low"
	mediumDevelopmentAuthorizationID         = "qual05-qwen38-medium-1"
	mediumDevelopmentCandidateID             = "qwen38-v12-medium-1"
	mediumDevelopmentModel                   = "./models/qwen38-v12-medium-1"
	mediumDevelopmentPromptVersion           = "file-analysis-v12"
	mediumDevelopmentReasoningEffort         = "medium"
	recoveryV2PeriodID                       = "engineering-insight-v2-recovery-1"
	recoveryV2CandidateID                    = "qwen38-v13-recovery-1"
	recoveryV2Model                          = "./models/qwen38-v13-recovery-1"
	recoveryV2PromptVersion                  = "file-analysis-v13"
	recoveryV2CorpusID                       = "engineering-insight-v2"
	recoveryV2DevelopmentDigest              = "c85fc3c5855da261047dccd63498787efcbdec9186ab0cd9b1dfee364c6afc3f"
	recoveryV2QualificationDigest            = "7cb16ac34b337e172e97a14aa639ef00bec77fbc77085028c01e6e1fe4183229"
	recoveryV2DevelopmentRunID               = "rcv07-qwen38-v13-dev-1"
	recoveryV2QualificationRunID             = "rcv08-qwen38-v13-qual-1"
	recoveryV2DevelopmentRequests            = 12
	recoveryV2QualificationRequests          = 24
	recoveryV2DevelopmentLimit               = 60
	recoveryV2QualificationLimit             = 48
	recoveryV2RuntimeContextTokens           = 119552
	recoveryV2Provider                       = "configured-bug"
	recoveryV2Endpoint                       = "http://127.0.0.1:1235/v1"
)

var (
	ErrEngineeringInsightRunLocked     = errors.New("engineering insight evaluation is already running")
	ErrEngineeringInsightRunFinished   = errors.New("engineering insight evaluation run is already complete")
	ErrEngineeringInsightRunTerminated = errors.New("engineering insight evaluation run stopped after permanent request rejection")
	ErrEngineeringInsightRemoteDenied  = errors.New("evaluation remote provider requires explicit confirmation")
)

const permanentRequestRejectionReason = "permanent_request_rejected"

// EngineeringInsightRunnerCase is deliberately source-bearing only in memory.
// It is never included in a manifest or receipt.
type EngineeringInsightRunnerCase struct {
	Expected EngineeringInsightExpectedAttempt
	Source   string
}

// EngineeringInsightRunnerClient is the single request boundary used by the
// runner. The production client executes one transport attempt per call.
type EngineeringInsightRunnerClient interface {
	ChatWithJSONSchema(context.Context, []llm.ChatMessage, llm.JSONSchema) (*llm.ChatResponse, error)
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
	CorpusJSON            []byte
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

type engineeringInsightRunManifest struct {
	Fingerprint    string                              `json:"fingerprint"`
	Finished       bool                                `json:"finished"`
	TerminalReason string                              `json:"terminal_reason,omitempty"`
	Receipt        EngineeringInsightEvaluationReceipt `json:"receipt"`
}

// engineeringInsightOptionalDiagnostics is private, source-free evidence for
// one emitted response. It deliberately remains outside the public receipt.
type engineeringInsightOptionalDiagnostics struct {
	RunID                       string                              `json:"run_id"`
	AttemptID                   string                              `json:"attempt_id"`
	ResponseDigest              string                              `json:"response_digest"`
	Insights                    []app.FileAnalysisInsightDiagnostic `json:"insights"`
	TaskSpecDegradedRiskIndices []int                               `json:"task_spec_degraded_risk_indices"`
}

type engineeringInsightCampaign struct {
	DevelopmentRequests   int `json:"development_requests"`
	QualificationRequests int `json:"qualification_requests"`
}

// engineeringInsightRecoveryV2Period is the sole successor authorization. It
// is intentionally separate from the frozen v1 campaign and its grant ledger.
// Reservations share the counter write so a crash cannot turn a charged slot
// into a new provider attempt.
type engineeringInsightRecoveryV2Period struct {
	Version                         string            `json:"version"`
	PeriodID                        string            `json:"period_id"`
	CanonicalRoot                   string            `json:"canonical_root"`
	Provider                        string            `json:"provider"`
	Endpoint                        string            `json:"endpoint"`
	CandidateID                     string            `json:"candidate_id"`
	Model                           string            `json:"model"`
	PromptVersion                   string            `json:"prompt_version"`
	CorpusID                        string            `json:"corpus_id"`
	DevelopmentCorpusDigest         string            `json:"development_corpus_digest"`
	QualificationCorpusDigest       string            `json:"qualification_corpus_digest"`
	BaseRevision                    string            `json:"base_revision"`
	SchemaIdentity                  string            `json:"schema_identity"`
	MaxOutputTokens                 int               `json:"max_output_tokens"`
	InputContextTokens              int               `json:"input_context_tokens"`
	RuntimeContextTokens            int               `json:"runtime_context_tokens"`
	AttemptTimeoutSeconds           int               `json:"attempt_timeout_seconds"`
	ReasoningEffort                 string            `json:"reasoning_effort"`
	Temperature                     float32           `json:"temperature"`
	TopP                            float32           `json:"top_p"`
	TopK                            int               `json:"top_k"`
	Lanes                           int               `json:"lanes"`
	DevelopmentRequests             int               `json:"development_requests"`
	QualificationRequests           int               `json:"qualification_requests"`
	CumulativeDevelopmentRequests   int               `json:"cumulative_development_requests"`
	CumulativeQualificationRequests int               `json:"cumulative_qualification_requests"`
	DevelopmentLimit                int               `json:"development_limit"`
	QualificationLimit              int               `json:"qualification_limit"`
	PredecessorFiles                map[string]string `json:"predecessor_files_sha256"`
	DevelopmentSchedule             []string          `json:"development_schedule"`
	QualificationSchedule           []string          `json:"qualification_schedule"`
	Reservations                    map[string]bool   `json:"reservations"`
}

var recoveryV2PredecessorFiles = map[string]string{
	"campaign.json": "d30821b8ba97a3914fbed615c01aed25ff5953a94fbec7f0de095c85df0ada2c", "development-grants.json": "e51c04994b479e2ace9e34121cf5cb2dd0e9d774ea825c6e403b1bd01d64bbd0",
	"qual05-v9-dev3-receipt.json": "f64c9a495a05f28aa2d15e6069de2293318b2ae5759dd94d48eca1fc0cacb10f", "qual05-v8-dev2-receipt.json": "32c8c6ad8654e195984747952c4f9898f1af14c664d10b2d30f830649da5346a", "qual05-v10-dev4-receipt.json": "c3a1d7c31ead7fcb329b3650767ce076baecece21ff0139cc36dc25ba6f7e0d9",
	"qual05-qwen38-thinking-off-dev1-receipt.json": "2aae836ea20ac7a8f4931a195400c5709180b18526aacfba2daa384426138b6e", "qual05-qwen38-v11-dev2-receipt.json": "13a32d8d0a10512069ea6703bdfb877b473c63660d04200e4b8a101876b60e7e", "qual05-qwen38-thinking-schema-dev1-receipt.json": "20c3354a989beb0d124f98145d47298f4ed516ee6e710dc5cf5f25555a3720c9",
	"qual05-v7-dev1-receipt.json": "f60d46fcfd3dc73c19ddfa28c1914cf1bb024ea35ed88835e508c02039d48aba", "qual05-qwen38-v12-dev2-receipt.json": "3688c2f0ba11ffa6fbf3559b31e492c9441e79b53439a1079b070afb12a8a0fc", "qual05-qwen38-dev2-receipt.json": "446f40922f5bdda1bdfab75f4137a59652d7cdf3a44162d1afd407926c8eeaff",
	"qual05-qwen38-medium-dev1-receipt.json": "6651383c177b78e7f5ea8b41bea3aea49e99ca98af5650f6452449c87205735c", "qual06-qwen38-medium-1-receipt.json": "e5f3a665d50376870369f0f577448b6df0d17fe4049a1b2289295f1816c4174e", "qual05-qwen38-dev1-receipt.json": "3088e4ab82da3cab170362db4f958233853e7c32bab3c9de2f6ecf87aaebe1c1",
	"qual05-qwen38-v12-dev1-receipt.json": "95f719680e90f8b198da1f34b5b5173a7e71a72feb863f0ddc3d2c82935b4546", "qual05-qwen38-medium-dev2-receipt.json": "260ee40baf7f76ccda776f9295afa55cae9e685becf43281368bbd971326dba2", "qual05-qwen38-thinking-off-dev2-receipt.json": "820eb7c9203a75ff6ad92428cf51c1de5b9b76970a657cc142345cdc15331d25",
	"qual05-qwen38-v11-dev1-receipt.json": "4b01d63027222a004c94b986c8c41b2f22ab04f0fda4534d7e2e78994a7d4c68", "qual05-qwen38-thinking-schema-dev2-receipt.json": "300e47e980a2cc5929a0bf5fd393061e7102ad5b919612a75c34c4790df84326",
}

type engineeringInsightDevelopmentGrantLedger struct {
	Grants []engineeringInsightDevelopmentGrant `json:"grants"`
}

type engineeringInsightDevelopmentGrant struct {
	AuthorizationID string `json:"authorization_id"`
	Requests        int    `json:"requests"`
	CandidateID     string `json:"candidate_id,omitempty"`
	Model           string `json:"model,omitempty"`
	PromptVersion   string `json:"prompt_version,omitempty"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}

type engineeringInsightDevelopmentGrantIdentity struct {
	CandidateID     string
	Model           string
	PromptVersion   string
	ReasoningEffort string
}

type engineeringInsightDevelopmentGrantWindow struct {
	grantIndex int
	minimum    int
	maximum    int
	errorText  string
}

type engineeringInsightDevelopmentGrantRule struct {
	valid     func(string, engineeringInsightDevelopmentGrantIdentity) bool
	errorText string
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
	manifest, err := loadOrCreateRunnerManifest(manifestPath, cfg)
	if err != nil {
		if manifest.Finished {
			return manifest.Receipt, nil, err
		}
		return EngineeringInsightEvaluationReceipt{}, nil, err
	}

	handoff := &EngineeringInsightScoringHandoff{RunID: cfg.RunID, Responses: make(map[string]string), privateDir: buildPrivateResponseDirectory(directory, cfg.RunID)}
	if err := executeRemainingAttempts(ctx, cfg, directory, manifestPath, &manifest, handoff); err != nil {
		return manifest.Receipt, handoff, err
	}
	manifest.Finished = true
	if err := storeRunnerJSON(manifestPath, manifest); err != nil {
		handoff.Discard()
		return manifest.Receipt, nil, err
	}
	return manifest.Receipt, handoff, nil
}

func loadOrCreateRunnerManifest(path string, cfg EngineeringInsightRunnerConfig) (engineeringInsightRunManifest, error) {
	fingerprint := buildRunnerFingerprint(cfg)
	manifest, exists, err := loadRunManifest(path)
	if err != nil {
		return engineeringInsightRunManifest{}, err
	}
	if !exists {
		manifest = engineeringInsightRunManifest{Fingerprint: fingerprint, Receipt: buildRunnerReceipt(cfg)}
		if err := storeRunnerJSON(path, manifest); err != nil {
			return engineeringInsightRunManifest{}, err
		}
		return manifest, nil
	}
	if manifest.Fingerprint != fingerprint || validateRunnerManifest(manifest, cfg) != nil {
		return engineeringInsightRunManifest{}, fmt.Errorf("evaluation run configuration changed")
	}
	if manifest.Finished {
		return manifest, buildCompletedRunnerManifestError(manifest)
	}
	return manifest, nil
}

func buildCompletedRunnerManifestError(manifest engineeringInsightRunManifest) error {
	if manifest.TerminalReason != "" {
		return ErrEngineeringInsightRunTerminated
	}
	return ErrEngineeringInsightRunFinished
}

func validateRunnerDestination(cfg EngineeringInsightRunnerConfig) error {
	if app.FileAnalysisEvaluationProvider(cfg.Profile).RemoteProvider && !cfg.ConfirmRemoteProvider {
		return ErrEngineeringInsightRemoteDenied
	}
	return nil
}

func validateRunnerConfig(cfg EngineeringInsightRunnerConfig) error {
	if hasRecoveryV2IdentityMarkers(cfg) && !isRecoveryV2RunnerConfig(cfg) {
		return fmt.Errorf("recovery evaluation identity is invalid")
	}
	if !isValidRunnerIdentity(cfg) {
		return fmt.Errorf("evaluation runner configuration is incomplete")
	}
	if !isValidRunnerMode(cfg.Mode) {
		return fmt.Errorf("evaluation runner mode is invalid")
	}
	if cfg.PromptVersion != app.EngineeringInsightPromptVersion() {
		return fmt.Errorf("evaluation runner prompt identity is invalid")
	}
	if _, err := app.FileAnalysisResponseSchema().Identity(); err != nil {
		return fmt.Errorf("evaluation runner response schema is invalid: %w", err)
	}
	if isRecoveryV2RunnerConfig(cfg) {
		return validateRecoveryV2RunnerConfig(cfg)
	}
	return validateHistoricalRunnerConfig(cfg)
}

func validateHistoricalRunnerConfig(cfg EngineeringInsightRunnerConfig) error {
	if cfg.Mode == EngineeringInsightQualificationRunMode && len(cfg.Cases) != qualificationRequestCap {
		return fmt.Errorf("qualification runner requires 24 attempts")
	}
	if cfg.Mode != EngineeringInsightQualificationRunMode && len(cfg.Cases) > 6 {
		return fmt.Errorf("development runner exceeds its request budget")
	}
	if !isValidRunnerCases(cfg.Cases) {
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
	} else if !isValidDevelopmentRunnerSchedule(cfg.Cases, cfg.Mode) {
		return fmt.Errorf("development runner schedule is invalid")
	}
	return nil
}

func validateRecoveryV2RunnerConfig(cfg EngineeringInsightRunnerConfig) error {
	if cfg.Mode == EngineeringInsightDevelopmentRunMode && !isValidRecoveryV2DevelopmentSchedule(cfg.Cases) {
		return fmt.Errorf("recovery development runner schedule is invalid")
	}
	if cfg.Mode == EngineeringInsightQualificationRunMode && !isValidRecoveryV2QualificationSchedule(cfg.Cases) {
		return fmt.Errorf("recovery qualification runner schedule is invalid")
	}
	return nil
}

func isValidRunnerIdentity(cfg EngineeringInsightRunnerConfig) bool {
	return strings.TrimSpace(cfg.Root) != "" && isValidRunnerID(cfg.RunID) && cfg.Client != nil && cfg.Profile.Scope == config.BugModelScope && len(cfg.Cases) != 0
}

// ValidEngineeringInsightEvaluationRunID reports whether a run identifier can be
// used as a single private-state file name.
func ValidEngineeringInsightEvaluationRunID(runID string) bool {
	return isValidRunnerID(runID)
}

func isValidRunnerID(runID string) bool {
	if len(runID) == 0 || len(runID) > 96 || !isASCIILetterOrDigit(runID[0]) {
		return false
	}
	for index := 1; index < len(runID); index++ {
		if !isASCIILetterOrDigit(runID[index]) && runID[index] != '-' && runID[index] != '_' {
			return false
		}
	}
	return true
}

func isASCIILetterOrDigit(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}

func isValidRunnerMode(mode string) bool {
	return mode == EngineeringInsightCollectRunMode || mode == EngineeringInsightDevelopmentRunMode || mode == EngineeringInsightQualificationRunMode
}

func isValidRunnerCases(cases []EngineeringInsightRunnerCase) bool {
	seen := make(map[string]bool, len(cases))
	for _, item := range cases {
		id := buildExpectedAttemptID(item.Expected)
		if seen[id] || !isValidExpectedAttempt(item.Expected) || strings.TrimSpace(item.Source) == "" {
			return false
		}
		seen[id] = true
	}
	return true
}

func isValidDevelopmentRunnerSchedule(cases []EngineeringInsightRunnerCase, mode string) bool {
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

func buildRunnerReceipt(cfg EngineeringInsightRunnerConfig) EngineeringInsightEvaluationReceipt {
	mode := EngineeringInsightCollectionMode
	maxRequests := defaultDevelopmentRequestCap
	if isRecoveryV2RunnerConfig(cfg) {
		if cfg.Mode == EngineeringInsightQualificationRunMode {
			mode, maxRequests = EngineeringInsightQualificationMode, recoveryV2QualificationRequests
		} else {
			maxRequests = recoveryV2DevelopmentRequests
		}
		return EngineeringInsightEvaluationReceipt{ProtocolVersion: "v2", Mode: mode, RunID: cfg.RunID, CandidateID: cfg.CandidateID, Provider: cfg.Provider, Model: cfg.Model, PromptVersion: cfg.PromptVersion, CorpusID: cfg.CorpusID, CorpusDigest: cfg.CorpusDigest, BaseRevision: cfg.BaseRevision, MaxRequests: maxRequests, MaxOutputTokens: qualificationOutputTokenCap, AttemptTimeoutSeconds: qualificationAttemptTimeoutSeconds}
	}
	if cfg.Mode == EngineeringInsightQualificationRunMode {
		mode, maxRequests = EngineeringInsightQualificationMode, qualificationRequestCap
	}
	return EngineeringInsightEvaluationReceipt{Mode: mode, RunID: cfg.RunID, CandidateID: cfg.CandidateID, Provider: cfg.Provider, Model: cfg.Model, PromptVersion: cfg.PromptVersion, CorpusID: cfg.CorpusID, CorpusDigest: cfg.CorpusDigest, BaseRevision: cfg.BaseRevision, MaxRequests: maxRequests, MaxOutputTokens: qualificationOutputTokenCap, AttemptTimeoutSeconds: qualificationAttemptTimeoutSeconds}
}

func buildRunnerFingerprint(cfg EngineeringInsightRunnerConfig) string {
	// This private digest binds endpoint, credentials, context limit and the
	// immutable public identity without ever writing those values to a receipt.
	// validateRunnerConfig verifies the embedded schema before this digest is
	// used to create or resume a manifest.
	schemaIdentity, _ := app.FileAnalysisResponseSchema().Identity()
	return buildRunnerFingerprintWithSchemaIdentity(cfg, schemaIdentity)
}

func buildRunnerFingerprintWithSchemaIdentity(cfg EngineeringInsightRunnerConfig, schemaIdentity string) string {
	values := []string{cfg.Mode, cfg.CandidateID, cfg.Provider, cfg.Model, cfg.PromptVersion, schemaIdentity, cfg.CorpusID, cfg.CorpusDigest, cfg.BaseRevision, string(cfg.Profile.Scope), cfg.Profile.APIBaseURL, cfg.Profile.APIKey, cfg.Profile.Model, cfg.Profile.ReasoningEffort, fmt.Sprint(cfg.Profile.Temperature), buildFingerprintOptionalFloat(cfg.Profile.TopP), buildFingerprintOptionalInt(cfg.Profile.TopK), buildFingerprintOptionalFloat(cfg.Profile.MinP), buildFingerprintOptionalFloat(cfg.Profile.PresencePenalty), buildFingerprintOptionalFloat(cfg.Profile.RepeatPenalty), fmt.Sprint(cfg.Profile.ContextMaxTokens)}
	for _, item := range cfg.Cases {
		source := sha256.Sum256([]byte(item.Source))
		values = append(values, buildExpectedAttemptID(item.Expected), hex.EncodeToString(source[:]))
	}
	value := strings.Join(values, "\x00")
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func buildFingerprintOptionalFloat(value *float32) string {
	if value == nil {
		return "unset"
	}
	return fmt.Sprintf("%g", *value)
}

func buildFingerprintOptionalInt(value *int) string {
	if value == nil {
		return "unset"
	}
	return fmt.Sprint(*value)
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
	want := buildRunnerReceipt(cfg)
	if !hasSameRunnerReceiptIdentity(manifest.Receipt, want) || len(manifest.Receipt.Attempts) > len(cfg.Cases) || manifest.Receipt.Consumption.Requests != len(manifest.Receipt.Attempts) {
		return fmt.Errorf("invalid evaluation manifest")
	}
	if manifest.TerminalReason != "" && (!manifest.Finished || manifest.TerminalReason != permanentRequestRejectionReason || len(manifest.Receipt.Attempts) == 0) {
		return fmt.Errorf("invalid evaluation manifest")
	}
	if !hasValidManifestAttempts(manifest.Receipt, cfg.Cases) {
		return fmt.Errorf("invalid evaluation manifest")
	}
	return nil
}

func hasSameRunnerReceiptIdentity(actual, expected EngineeringInsightEvaluationReceipt) bool {
	return actual.ProtocolVersion == expected.ProtocolVersion && actual.Mode == expected.Mode && actual.RunID == expected.RunID && actual.CandidateID == expected.CandidateID && actual.Provider == expected.Provider && actual.Model == expected.Model && actual.PromptVersion == expected.PromptVersion && actual.CorpusID == expected.CorpusID && actual.CorpusDigest == expected.CorpusDigest && actual.BaseRevision == expected.BaseRevision && actual.MaxRequests == expected.MaxRequests && actual.MaxOutputTokens == expected.MaxOutputTokens && actual.AttemptTimeoutSeconds == expected.AttemptTimeoutSeconds
}

func hasValidManifestAttempts(receipt EngineeringInsightEvaluationReceipt, cases []EngineeringInsightRunnerCase) bool {
	tokens := 0
	for index, attempt := range receipt.Attempts {
		actual := EngineeringInsightExpectedAttempt{CaseName: attempt.CaseName, Partition: attempt.Partition, Intent: attempt.Intent, Repetition: attempt.Repetition, Attempt: attempt.Attempt}
		if buildExpectedAttemptID(cases[index].Expected) != buildExpectedAttemptID(actual) || validateAttempt(attempt, EngineeringInsightCollectionMode) != nil {
			return false
		}
		tokens += attempt.OutputTokens
	}
	return tokens == receipt.Consumption.OutputTokens
}

// Keep private paths and serialization details out of CLI errors while retaining
// the underlying operation for internal error inspection.
type evaluationMetadataWriteError struct {
	cause error
}

func (e *evaluationMetadataWriteError) Error() string { return "write evaluation run" }

func (e *evaluationMetadataWriteError) Unwrap() error { return e.cause }

func storeRunnerJSON(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return &evaluationMetadataWriteError{cause: fmt.Errorf("marshal evaluation metadata: %w", err)}
	}
	// Evaluation callers establish storage before acquiring their locks. Do not
	// let the shared writer initialize a missing evaluation directory implicitly.
	directory, err := os.Stat(filepath.Dir(path))
	if err != nil {
		return &evaluationMetadataWriteError{cause: fmt.Errorf("inspect evaluation directory: %w", err)}
	}
	if !directory.IsDir() {
		return &evaluationMetadataWriteError{cause: errors.New("evaluation storage is not a directory")}
	}
	if err := storage.WriteFile(path, data, 0600); err != nil {
		return &evaluationMetadataWriteError{cause: err}
	}
	return nil
}
