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
	recoveryV2DevelopmentDigest              = "2370397f665af4aefa0ab8db0dab7e0a0e961930b63594e0fc894fabed6a2431"
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
	Fingerprint    string                              `json:"fingerprint"`
	Finished       bool                                `json:"finished"`
	TerminalReason string                              `json:"terminal_reason,omitempty"`
	Receipt        EngineeringInsightEvaluationReceipt `json:"receipt"`
}

// engineeringInsightOptionalDiagnostics is private, source-free evidence for
// one emitted response. It deliberately remains outside the public receipt.
type engineeringInsightOptionalDiagnostics struct {
	RunID                       string                                 `json:"run_id"`
	AttemptID                   string                                 `json:"attempt_id"`
	ResponseDigest              string                                 `json:"response_digest"`
	Insights                    []engineeringInsightOptionalDiagnostic `json:"insights"`
	SymbolExplanationsDegraded  bool                                   `json:"symbol_explanations_degraded"`
	TaskSpecDegradedRiskIndices []int                                  `json:"task_spec_degraded_risk_indices"`
}

type engineeringInsightOptionalDiagnostic struct {
	Location              string                                            `json:"location"`
	Index                 *int                                              `json:"index,omitempty"`
	Reason                project.OptionalEngineeringInsightReason          `json:"reason"`
	Presence              project.OptionalEngineeringInsightPresence        `json:"presence"`
	Mechanism             project.OptionalEngineeringInsightFieldDiagnostic `json:"mechanism"`
	WhyItMattersHere      project.OptionalEngineeringInsightFieldDiagnostic `json:"why_it_matters_here"`
	TradeoffOrFailureMode project.OptionalEngineeringInsightFieldDiagnostic `json:"tradeoff_or_failure_mode"`
	TransferableLesson    project.OptionalEngineeringInsightFieldDiagnostic `json:"transferable_lesson"`
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

// AuthorizeEngineeringInsightRecoveryV2 records the only reviewed successor
// period. It has no caller-controlled candidate, cap, corpus, or identity.
func AuthorizeEngineeringInsightRecoveryV2(root string) error {
	canonicalRoot, err := canonicalEvaluationRoot(root)
	if err != nil {
		return fmt.Errorf("recovery evaluation root is invalid")
	}
	baseRevision, err := cleanCanonicalRecoveryBase(canonicalRoot)
	if err != nil {
		return fmt.Errorf("recovery evaluation base is not a clean canonical checkout")
	}
	if err := validateRecoveryV2Predecessors(canonicalRoot); err != nil {
		return err
	}
	if err := validateRecoveryCoordinatorBoundary(canonicalRoot); err != nil {
		return err
	}
	schemaIdentity, err := FileAnalysisResponseSchema().Identity()
	if err != nil {
		return fmt.Errorf("recovery response schema is invalid")
	}
	directory := filepath.Join(canonicalRoot, runnerRelativeDirectory)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return fmt.Errorf("create evaluation storage")
	}
	lock, err := lockRunnerFile(filepath.Join(directory, "campaign.lock"))
	if err != nil {
		return err
	}
	defer lock.Close()
	path := recoveryV2PeriodPath(directory)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("recovery successor period is already authorized")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read recovery successor period")
	}
	period := engineeringInsightRecoveryV2Period{
		Version: "v2", PeriodID: recoveryV2PeriodID, CanonicalRoot: canonicalRoot,
		CandidateID: recoveryV2CandidateID, Provider: recoveryV2Provider, Endpoint: recoveryV2Endpoint, Model: recoveryV2Model, PromptVersion: recoveryV2PromptVersion,
		CorpusID: recoveryV2CorpusID, DevelopmentCorpusDigest: recoveryV2DevelopmentDigest,
		QualificationCorpusDigest: recoveryV2QualificationDigest, BaseRevision: baseRevision,
		SchemaIdentity: schemaIdentity, MaxOutputTokens: qualificationOutputTokenCap, InputContextTokens: 16384, RuntimeContextTokens: recoveryV2RuntimeContextTokens,
		AttemptTimeoutSeconds: qualificationAttemptTimeoutSeconds, ReasoningEffort: mediumDevelopmentReasoningEffort,
		Temperature: 1, TopP: .95, TopK: 20, Lanes: 1,
		DevelopmentLimit: recoveryV2DevelopmentLimit, QualificationLimit: recoveryV2QualificationLimit,
		CumulativeDevelopmentRequests: 48, CumulativeQualificationRequests: 24,
		PredecessorFiles: cloneRecoveryV2PredecessorFiles(), DevelopmentSchedule: []string{}, QualificationSchedule: []string{}, Reservations: make(map[string]bool),
	}
	return writeRunnerJSON(path, period)
}

func cleanCanonicalRecoveryBase(root string) (string, error) {
	worktree, err := exec.Command("git", "-C", root, "rev-parse", "--show-toplevel").Output()
	if err != nil || strings.TrimSpace(string(worktree)) != root {
		return "", fmt.Errorf("not canonical")
	}
	gitDir, err := exec.Command("git", "-C", root, "rev-parse", "--absolute-git-dir").Output()
	if err != nil || strings.TrimSpace(string(gitDir)) != filepath.Join(root, ".git") {
		return "", fmt.Errorf("linked worktree")
	}
	branch, err := exec.Command("git", "-C", root, "symbolic-ref", "--quiet", "--short", "HEAD").Output()
	if err != nil || strings.TrimSpace(string(branch)) != "codex/autopilot" {
		return "", fmt.Errorf("wrong branch")
	}
	status, err := exec.Command("git", "-C", root, "status", "--porcelain=v1", "--untracked-files=all").Output()
	if err != nil || strings.TrimSpace(string(status)) != "" {
		return "", fmt.Errorf("unclean checkout")
	}
	base, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil || !validRecoveryV2BaseRevision(strings.TrimSpace(string(base))) {
		return "", fmt.Errorf("invalid head")
	}
	return strings.TrimSpace(string(base)), nil
}

func validateRecoveryCoordinatorBoundary(root string) error {
	data, err := os.ReadFile(filepath.Join(root, ".mini-orca", "autopilot", "coordinator", "RCV-recovery.json"))
	if err != nil || ValidateStrictJSONDocument(data) != nil {
		return fmt.Errorf("recovery coordinator boundary is invalid")
	}
	var boundary struct {
		Phase           string `json:"phase"`
		SchedulerStatus string `json:"scheduler_status"`
		Tasks           map[string]struct {
			Phase string `json:"phase"`
		} `json:"tasks"`
	}
	if json.Unmarshal(data, &boundary) != nil || boundary.Phase != "RCV-07" || boundary.SchedulerStatus != "PAUSED" {
		return fmt.Errorf("recovery coordinator boundary is inactive")
	}
	for _, task := range []string{"RCV-01", "RCV-02", "RCV-03", "RCV-04", "RCV-05", "RCV-06"} {
		if boundary.Tasks[task].Phase != "complete" {
			return fmt.Errorf("recovery coordinator boundary is incomplete")
		}
	}
	return nil
}

func validateRecoveryV2Predecessors(root string) error {
	directory := filepath.Join(root, runnerRelativeDirectory)
	for name, want := range recoveryV2PredecessorFiles {
		data, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			return fmt.Errorf("recovery predecessor evidence is missing")
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != want {
			return fmt.Errorf("recovery predecessor evidence changed")
		}
	}
	return nil
}

func validRecoveryV2BaseRevision(value string) bool {
	if len(value) != 40 {
		return false
	}
	for _, character := range value {
		if !(character >= '0' && character <= '9' || character >= 'a' && character <= 'f') {
			return false
		}
	}
	return true
}

func recoveryV2PeriodPath(directory string) string {
	return filepath.Join(directory, recoveryV2PeriodID+".json")
}

func canonicalEvaluationRoot(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", fmt.Errorf("empty root")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

func cloneRecoveryV2PredecessorFiles() map[string]string {
	result := make(map[string]string, len(recoveryV2PredecessorFiles))
	for name, digest := range recoveryV2PredecessorFiles {
		result[name] = digest
	}
	return result
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

// GrantEngineeringInsightDevelopmentBudget records the original append-only
// recovery authorization. It remains unbound because it predates dispatch
// identity binding.
func GrantEngineeringInsightDevelopmentBudget(root, authorizationID string, requests int) error {
	return grantEngineeringInsightDevelopmentBudget(root, authorizationID, requests, engineeringInsightDevelopmentGrantIdentity{})
}

// GrantEngineeringInsightRecoveryDevelopmentBudget records the one approved
// model-bound recovery authorization. Dispatch validates this identity again
// before any request from the grant is reserved.
func GrantEngineeringInsightRecoveryDevelopmentBudget(root, authorizationID string, requests int, candidateID, model string) error {
	return grantEngineeringInsightDevelopmentBudget(root, authorizationID, requests, engineeringInsightDevelopmentGrantIdentity{CandidateID: candidateID, Model: model})
}

// GrantEngineeringInsightV11DevelopmentBudget records the one approved v11
// model-bound extension. Its prompt identity is checked again at dispatch.
func GrantEngineeringInsightV11DevelopmentBudget(root, authorizationID string, requests int, candidateID, model, promptVersion string) error {
	return grantEngineeringInsightDevelopmentBudget(root, authorizationID, requests, engineeringInsightDevelopmentGrantIdentity{CandidateID: candidateID, Model: model, PromptVersion: promptVersion})
}

// GrantEngineeringInsightV12DevelopmentBudget records the one approved
// schema-constrained pilot extension with fixed dispatch identity.
func GrantEngineeringInsightV12DevelopmentBudget(root, authorizationID string, requests int, candidateID, model, promptVersion string) error {
	return grantEngineeringInsightDevelopmentBudget(root, authorizationID, requests, engineeringInsightDevelopmentGrantIdentity{CandidateID: candidateID, Model: model, PromptVersion: promptVersion})
}

// GrantEngineeringInsightThinkingOffDevelopmentBudget records the one approved
// thinking-disabled extension with its fixed structured dispatch identity.
func GrantEngineeringInsightThinkingOffDevelopmentBudget(root, authorizationID string, requests int, candidateID, model, promptVersion string) error {
	return grantEngineeringInsightDevelopmentBudget(root, authorizationID, requests, engineeringInsightDevelopmentGrantIdentity{CandidateID: candidateID, Model: model, PromptVersion: promptVersion, ReasoningEffort: thinkingOffDevelopmentReasoningEffort})
}

// GrantEngineeringInsightThinkingSchemaDevelopmentBudget records the one
// approved thinking-enabled structured-output extension. Its fixed dispatch
// identity is checked again before every request from the grant is reserved.
func GrantEngineeringInsightThinkingSchemaDevelopmentBudget(root, authorizationID string, requests int, candidateID, model, promptVersion string) error {
	return grantEngineeringInsightDevelopmentBudget(root, authorizationID, requests, engineeringInsightDevelopmentGrantIdentity{CandidateID: candidateID, Model: model, PromptVersion: promptVersion, ReasoningEffort: thinkingSchemaDevelopmentReasoningEffort})
}

// GrantEngineeringInsightMediumDevelopmentBudget records the one approved
// medium-reasoning recovery extension with its fixed structured dispatch identity.
func GrantEngineeringInsightMediumDevelopmentBudget(root, authorizationID string, requests int, candidateID, model, promptVersion string) error {
	return grantEngineeringInsightDevelopmentBudget(root, authorizationID, requests, engineeringInsightDevelopmentGrantIdentity{CandidateID: candidateID, Model: model, PromptVersion: promptVersion, ReasoningEffort: mediumDevelopmentReasoningEffort})
}

func grantEngineeringInsightDevelopmentBudget(root, authorizationID string, requests int, dispatchIdentity engineeringInsightDevelopmentGrantIdentity) error {
	if !validDevelopmentGrantRequest(root, authorizationID, requests) {
		return fmt.Errorf("development budget grant is invalid")
	}
	directory := filepath.Join(root, runnerRelativeDirectory)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return fmt.Errorf("create evaluation storage")
	}
	lock, err := lockRunnerFile(filepath.Join(directory, "campaign.lock"))
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := ensureEvaluationCampaign(directory); err != nil {
		return err
	}
	return appendDevelopmentGrant(directory, authorizationID, requests, dispatchIdentity)
}

func validDevelopmentGrantRequest(root, authorizationID string, requests int) bool {
	return strings.TrimSpace(root) != "" && validRunnerID(authorizationID) && requests == developmentGrantRequestCount
}

func appendDevelopmentGrant(directory, authorizationID string, requests int, dispatchIdentity engineeringInsightDevelopmentGrantIdentity) error {
	ledger, _, err := loadDevelopmentGrantLedger(directory)
	if err != nil {
		return err
	}
	grant, err := nextDevelopmentGrant(ledger, authorizationID, requests, dispatchIdentity)
	if err != nil {
		return err
	}
	campaign, err := loadEvaluationCampaign(filepath.Join(directory, "campaign.json"))
	if err != nil {
		return fmt.Errorf("read evaluation campaign")
	}
	priorCap := developmentGrantCap(ledger)
	if campaign.DevelopmentRequests != priorCap {
		return fmt.Errorf("development prior budget is not exhausted")
	}
	ledger.Grants = append(ledger.Grants, grant)
	if err := writeRunnerJSON(developmentGrantLedgerPath(directory), ledger); err != nil {
		return err
	}
	return nil
}

func nextDevelopmentGrant(ledger engineeringInsightDevelopmentGrantLedger, authorizationID string, requests int, dispatchIdentity engineeringInsightDevelopmentGrantIdentity) (engineeringInsightDevelopmentGrant, error) {
	if hasDevelopmentGrantAuthorization(ledger, authorizationID) {
		return engineeringInsightDevelopmentGrant{}, fmt.Errorf("development budget grant already exists")
	}
	if len(ledger.Grants) >= len(engineeringInsightDevelopmentGrantRules) {
		return engineeringInsightDevelopmentGrant{}, fmt.Errorf("development request budget extension is exhausted")
	}
	rule := engineeringInsightDevelopmentGrantRules[len(ledger.Grants)]
	if !rule.valid(authorizationID, dispatchIdentity) {
		return engineeringInsightDevelopmentGrant{}, fmt.Errorf("%s", rule.errorText)
	}
	return engineeringInsightDevelopmentGrant{AuthorizationID: authorizationID, Requests: requests, CandidateID: dispatchIdentity.CandidateID, Model: dispatchIdentity.Model, PromptVersion: dispatchIdentity.PromptVersion, ReasoningEffort: dispatchIdentity.ReasoningEffort}, nil
}

var engineeringInsightDevelopmentGrantRules = []engineeringInsightDevelopmentGrantRule{
	{valid: validOriginalDevelopmentGrant, errorText: "development budget grant identity is invalid"},
	{valid: validRecoveryDevelopmentGrant, errorText: "development recovery grant is invalid"},
	{valid: validV11DevelopmentGrant, errorText: "development v11 grant is invalid"},
	{valid: validV12DevelopmentGrant, errorText: "development structured-output grant is invalid"},
	{valid: validThinkingOffDevelopmentGrant, errorText: "development thinking-off grant is invalid"},
	{valid: validThinkingSchemaDevelopmentGrant, errorText: "development thinking-schema grant is invalid"},
	{valid: validMediumDevelopmentGrant, errorText: "development medium-reasoning grant is invalid"},
}

func hasDevelopmentGrantAuthorization(ledger engineeringInsightDevelopmentGrantLedger, authorizationID string) bool {
	for _, grant := range ledger.Grants {
		if grant.AuthorizationID == authorizationID {
			return true
		}
	}
	return false
}

func validOriginalDevelopmentGrant(authorizationID string, dispatchIdentity engineeringInsightDevelopmentGrantIdentity) bool {
	return authorizationID != recoveryDevelopmentAuthorizationID && authorizationID != v11DevelopmentAuthorizationID && authorizationID != v12DevelopmentAuthorizationID && authorizationID != thinkingOffDevelopmentAuthorizationID && authorizationID != thinkingSchemaDevelopmentAuthorizationID && authorizationID != mediumDevelopmentAuthorizationID && dispatchIdentity == (engineeringInsightDevelopmentGrantIdentity{})
}

func validRecoveryDevelopmentGrant(authorizationID string, dispatchIdentity engineeringInsightDevelopmentGrantIdentity) bool {
	return authorizationID == recoveryDevelopmentAuthorizationID &&
		dispatchIdentity.CandidateID == recoveryDevelopmentCandidateID &&
		dispatchIdentity.Model == recoveryDevelopmentModel &&
		dispatchIdentity.PromptVersion == "" &&
		dispatchIdentity.ReasoningEffort == ""
}

func validV11DevelopmentGrant(authorizationID string, dispatchIdentity engineeringInsightDevelopmentGrantIdentity) bool {
	return authorizationID == v11DevelopmentAuthorizationID &&
		dispatchIdentity.CandidateID == v11DevelopmentCandidateID &&
		dispatchIdentity.Model == v11DevelopmentModel &&
		dispatchIdentity.PromptVersion == v11DevelopmentPromptVersion &&
		dispatchIdentity.ReasoningEffort == ""
}

func validV12DevelopmentGrant(authorizationID string, dispatchIdentity engineeringInsightDevelopmentGrantIdentity) bool {
	return authorizationID == v12DevelopmentAuthorizationID &&
		dispatchIdentity.CandidateID == v12DevelopmentCandidateID &&
		dispatchIdentity.Model == v12DevelopmentModel &&
		dispatchIdentity.PromptVersion == v12DevelopmentPromptVersion &&
		dispatchIdentity.ReasoningEffort == ""
}

func validThinkingOffDevelopmentGrant(authorizationID string, dispatchIdentity engineeringInsightDevelopmentGrantIdentity) bool {
	return authorizationID == thinkingOffDevelopmentAuthorizationID &&
		dispatchIdentity.CandidateID == thinkingOffDevelopmentCandidateID &&
		dispatchIdentity.Model == thinkingOffDevelopmentModel &&
		dispatchIdentity.PromptVersion == thinkingOffDevelopmentPromptVersion &&
		dispatchIdentity.ReasoningEffort == thinkingOffDevelopmentReasoningEffort
}

func validThinkingSchemaDevelopmentGrant(authorizationID string, dispatchIdentity engineeringInsightDevelopmentGrantIdentity) bool {
	return authorizationID == thinkingSchemaDevelopmentAuthorizationID &&
		dispatchIdentity.CandidateID == thinkingSchemaDevelopmentCandidateID &&
		dispatchIdentity.Model == thinkingSchemaDevelopmentModel &&
		dispatchIdentity.PromptVersion == thinkingSchemaDevelopmentPromptVersion &&
		dispatchIdentity.ReasoningEffort == thinkingSchemaDevelopmentReasoningEffort
}

func validMediumDevelopmentGrant(authorizationID string, dispatchIdentity engineeringInsightDevelopmentGrantIdentity) bool {
	return authorizationID == mediumDevelopmentAuthorizationID &&
		dispatchIdentity.CandidateID == mediumDevelopmentCandidateID &&
		dispatchIdentity.Model == mediumDevelopmentModel &&
		dispatchIdentity.PromptVersion == mediumDevelopmentPromptVersion &&
		dispatchIdentity.ReasoningEffort == mediumDevelopmentReasoningEffort
}

func developmentGrantCap(ledger engineeringInsightDevelopmentGrantLedger) int {
	cap := defaultDevelopmentRequestCap
	for _, grant := range ledger.Grants {
		cap += grant.Requests
	}
	return cap
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

func loadOrCreateRunnerManifest(path string, cfg EngineeringInsightRunnerConfig) (engineeringInsightRunManifest, error) {
	fingerprint := runnerFingerprint(cfg)
	manifest, exists, err := loadRunManifest(path)
	if err != nil {
		return engineeringInsightRunManifest{}, err
	}
	if !exists {
		manifest = engineeringInsightRunManifest{Fingerprint: fingerprint, Receipt: runnerReceipt(cfg)}
		if err := writeRunnerJSON(path, manifest); err != nil {
			return engineeringInsightRunManifest{}, err
		}
		return manifest, nil
	}
	if manifest.Fingerprint != fingerprint || validateRunnerManifest(manifest, cfg) != nil {
		return engineeringInsightRunManifest{}, fmt.Errorf("evaluation run configuration changed")
	}
	if manifest.Finished {
		return manifest, completedRunnerManifestError(manifest)
	}
	return manifest, nil
}

func completedRunnerManifestError(manifest engineeringInsightRunManifest) error {
	if manifest.TerminalReason != "" {
		return ErrEngineeringInsightRunTerminated
	}
	return ErrEngineeringInsightRunFinished
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
		dispatch, err := reserveRunnerAttempt(directory, cfg, item.Expected)
		if err != nil {
			return err
		}
		// A reservation is recorded as unknown before prompt delivery. If this
		// process dies after this write, resume leaves the consumed slot intact.
		manifest.Receipt.Attempts = append(manifest.Receipt.Attempts, unknownReservedAttempt(item.Expected, cfg.CandidateID))
		manifest.Receipt.Consumption.Requests++
		if err := writeRunnerJSON(manifestPath, manifest); err != nil {
			return err
		}
		if !dispatch {
			continue
		}

		attempt, response, diagnostics, err := executeRunnerAttempt(ctx, cfg, item)
		permanentRejection := errors.Is(err, llm.ErrStructuredRequestRejected)
		if err != nil && !permanentRejection {
			attempt = failedRunnerAttempt(item.Expected, cfg.CandidateID, err)
		}
		if response != "" {
			// Publish the source-free diagnostic before the raw private response.
			// A failure between either immutable write and the receipt update leaves
			// the reserved attempt unknown, and resume intentionally never replays it.
			if err := writePrivateOptionalDiagnostics(directory, engineeringInsightOptionalDiagnostics{RunID: cfg.RunID, AttemptID: expectedAttemptID(item.Expected), ResponseDigest: attempt.ResponseDigest, Insights: diagnostics.insights, SymbolExplanationsDegraded: diagnostics.symbolExplanationsDegraded, TaskSpecDegradedRiskIndices: diagnostics.taskSpecDegradedRiskIndices}); err != nil {
				return err
			}
			if err := writePrivateResponse(handoff.privateDir, expectedAttemptID(item.Expected), response); err != nil {
				return err
			}
		}
		manifest.Receipt.Attempts[index] = attempt
		manifest.Receipt.Consumption.OutputTokens += attempt.OutputTokens
		if permanentRejection {
			manifest.Finished = true
			manifest.TerminalReason = permanentRequestRejectionReason
		}
		if err := writeRunnerJSON(manifestPath, manifest); err != nil {
			return err
		}
		if response != "" {
			handoff.Responses[expectedAttemptID(item.Expected)] = response
		}
		if permanentRejection {
			return ErrEngineeringInsightRunTerminated
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
	if looksLikeRecoveryV2(cfg) && !recoveryV2RunnerConfig(cfg) {
		return fmt.Errorf("recovery evaluation identity is invalid")
	}
	if !validRunnerIdentity(cfg) {
		return fmt.Errorf("evaluation runner configuration is incomplete")
	}
	if !validRunnerMode(cfg.Mode) {
		return fmt.Errorf("evaluation runner mode is invalid")
	}
	if cfg.PromptVersion != semanticAnalysisPromptVersion {
		return fmt.Errorf("evaluation runner prompt identity is invalid")
	}
	if _, err := FileAnalysisResponseSchema().Identity(); err != nil {
		return fmt.Errorf("evaluation runner response schema is invalid: %w", err)
	}
	if recoveryV2RunnerConfig(cfg) {
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

func validateRecoveryV2RunnerConfig(cfg EngineeringInsightRunnerConfig) error {
	if cfg.Mode == EngineeringInsightDevelopmentRunMode && !validRecoveryV2DevelopmentSchedule(cfg.Cases) {
		return fmt.Errorf("recovery development runner schedule is invalid")
	}
	if cfg.Mode == EngineeringInsightQualificationRunMode && !validRecoveryV2QualificationSchedule(cfg.Cases) {
		return fmt.Errorf("recovery qualification runner schedule is invalid")
	}
	return nil
}

func looksLikeRecoveryV2(cfg EngineeringInsightRunnerConfig) bool {
	return cfg.CandidateID == recoveryV2CandidateID || cfg.Model == recoveryV2Model || cfg.Profile.Model == recoveryV2Model || cfg.CorpusID == recoveryV2CorpusID || cfg.RunID == recoveryV2DevelopmentRunID || cfg.RunID == recoveryV2QualificationRunID
}

func recoveryV2RunnerConfig(cfg EngineeringInsightRunnerConfig) bool {
	if !sameRecoveryV2RunnerIdentity(cfg) || !sameRecoveryV2RunnerProfile(cfg) || !validRecoveryV2Corpus(cfg) {
		return false
	}
	if cfg.Mode == EngineeringInsightDevelopmentRunMode {
		return cfg.RunID == recoveryV2DevelopmentRunID && cfg.CorpusDigest == recoveryV2DevelopmentDigest
	}
	return cfg.Mode == EngineeringInsightQualificationRunMode && cfg.RunID == recoveryV2QualificationRunID && cfg.CorpusDigest == recoveryV2QualificationDigest
}

func validRecoveryV2Corpus(cfg EngineeringInsightRunnerConfig) bool {
	if !matchesRecoveryV2CorpusDigest(cfg) || ValidateStrictJSONDocument(cfg.CorpusJSON) != nil {
		return false
	}
	corpus, ok := decodeRecoveryV2Corpus(cfg.CorpusJSON)
	return ok && matchesRecoveryV2CorpusCases(corpus, cfg.Cases)
}

func matchesRecoveryV2CorpusDigest(cfg EngineeringInsightRunnerConfig) bool {
	want := recoveryV2DevelopmentDigest
	if cfg.Mode == EngineeringInsightQualificationRunMode {
		want = recoveryV2QualificationDigest
	}
	sum := sha256.Sum256(cfg.CorpusJSON)
	return cfg.CorpusDigest == want && hex.EncodeToString(sum[:]) == want
}

type recoveryV2CorpusCase struct {
	Name      string `json:"name"`
	Partition string `json:"partition"`
	Intent    string `json:"intent"`
	Source    string `json:"source"`
}

func decodeRecoveryV2Corpus(data []byte) ([]recoveryV2CorpusCase, bool) {
	var corpus []recoveryV2CorpusCase
	if json.Unmarshal(data, &corpus) != nil {
		return nil, false
	}
	return corpus, true
}

func matchesRecoveryV2CorpusCases(corpus []recoveryV2CorpusCase, cases []EngineeringInsightRunnerCase) bool {
	selected := make(map[string]struct{ partition, intent, source string })
	for _, item := range corpus {
		if !validRecoveryV2CorpusCase(item) {
			return false
		}
		if _, exists := selected[item.Name]; exists {
			return false
		}
		selected[item.Name] = struct{ partition, intent, source string }{item.Partition, item.Intent, item.Source}
	}
	if len(selected) != len(cases) {
		return false
	}
	for _, item := range cases {
		caseSpec, exists := selected[item.Expected.CaseName]
		if !exists || !matchesRecoveryV2SelectedCase(caseSpec, item) {
			return false
		}
	}
	return true
}

func validRecoveryV2CorpusCase(item recoveryV2CorpusCase) bool {
	return item.Name != "" && !strings.ContainsRune(item.Name, '\x00') && item.Source != "" && (item.Partition == "development" || item.Partition == "qualification") && (item.Intent == "substantive" || item.Intent == "control")
}

func matchesRecoveryV2SelectedCase(spec struct{ partition, intent, source string }, item EngineeringInsightRunnerCase) bool {
	return spec.partition == item.Expected.Partition && spec.intent == item.Expected.Intent && spec.source == item.Source && item.Expected.Repetition == 1 && item.Expected.Attempt == 1
}

func sameRecoveryV2RunnerIdentity(cfg EngineeringInsightRunnerConfig) bool {
	return cfg.CandidateID == recoveryV2CandidateID && cfg.Provider == recoveryV2Provider && cfg.Model == recoveryV2Model && cfg.Profile.Model == recoveryV2Model && cfg.PromptVersion == recoveryV2PromptVersion && cfg.CorpusID == recoveryV2CorpusID && validRecoveryV2BaseRevision(cfg.BaseRevision)
}

func sameRecoveryV2RunnerProfile(cfg EngineeringInsightRunnerConfig) bool {
	return cfg.Profile.APIBaseURL == recoveryV2Endpoint && cfg.Profile.ReasoningEffort == mediumDevelopmentReasoningEffort && cfg.Profile.MaxTokens == qualificationOutputTokenCap && cfg.Profile.ContextMaxTokens == 16384 && cfg.Profile.Temperature == 1 && cfg.Profile.TopP != nil && *cfg.Profile.TopP == .95 && cfg.Profile.TopK != nil && *cfg.Profile.TopK == 20 && cfg.Profile.MinP == nil && cfg.Profile.PresencePenalty == nil && cfg.Profile.RepeatPenalty == nil
}

func validRecoveryV2DevelopmentSchedule(cases []EngineeringInsightRunnerCase) bool {
	return validRecoveryV2Schedule(cases, "development", recoveryV2DevelopmentRequests, 8, 4)
}

func validRecoveryV2QualificationSchedule(cases []EngineeringInsightRunnerCase) bool {
	return validRecoveryV2Schedule(cases, "qualification", recoveryV2QualificationRequests, 16, 8)
}

func validRecoveryV2Schedule(cases []EngineeringInsightRunnerCase, partition string, count, substantive, controls int) bool {
	if len(cases) != count {
		return false
	}
	seen := make(map[string]bool, len(cases))
	actualSubstantive, actualControls := 0, 0
	for _, item := range cases {
		attempt := item.Expected
		if seen[attempt.CaseName] || attempt.Partition != partition || attempt.Repetition != 1 || attempt.Attempt != 1 {
			return false
		}
		seen[attempt.CaseName] = true
		if attempt.Intent == "substantive" {
			actualSubstantive++
		} else if attempt.Intent == "control" {
			actualControls++
		} else {
			return false
		}
	}
	return actualSubstantive == substantive && actualControls == controls
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
	maxRequests := defaultDevelopmentRequestCap
	if recoveryV2RunnerConfig(cfg) {
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

func runnerFingerprint(cfg EngineeringInsightRunnerConfig) string {
	// This private digest binds endpoint, credentials, context limit and the
	// immutable public identity without ever writing those values to a receipt.
	// validateRunnerConfig verifies the embedded schema before this digest is
	// used to create or resume a manifest.
	schemaIdentity, _ := FileAnalysisResponseSchema().Identity()
	return runnerFingerprintWithSchemaIdentity(cfg, schemaIdentity)
}

func runnerFingerprintWithSchemaIdentity(cfg EngineeringInsightRunnerConfig, schemaIdentity string) string {
	values := []string{cfg.Mode, cfg.CandidateID, cfg.Provider, cfg.Model, cfg.PromptVersion, schemaIdentity, cfg.CorpusID, cfg.CorpusDigest, cfg.BaseRevision, string(cfg.Profile.Scope), cfg.Profile.APIBaseURL, cfg.Profile.APIKey, cfg.Profile.Model, cfg.Profile.ReasoningEffort, fmt.Sprint(cfg.Profile.Temperature), fingerprintOptionalFloat(cfg.Profile.TopP), fingerprintOptionalInt(cfg.Profile.TopK), fingerprintOptionalFloat(cfg.Profile.MinP), fingerprintOptionalFloat(cfg.Profile.PresencePenalty), fingerprintOptionalFloat(cfg.Profile.RepeatPenalty), fmt.Sprint(cfg.Profile.ContextMaxTokens)}
	for _, item := range cfg.Cases {
		source := sha256.Sum256([]byte(item.Source))
		values = append(values, expectedAttemptID(item.Expected), hex.EncodeToString(source[:]))
	}
	value := strings.Join(values, "\x00")
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func fingerprintOptionalFloat(value *float32) string {
	if value == nil {
		return "unset"
	}
	return fmt.Sprintf("%g", *value)
}

func fingerprintOptionalInt(value *int) string {
	if value == nil {
		return "unset"
	}
	return fmt.Sprint(*value)
}

func reserveRunnerAttempt(directory string, cfg EngineeringInsightRunnerConfig, expected EngineeringInsightExpectedAttempt) (bool, error) {
	if recoveryV2RunnerConfig(cfg) {
		return reserveRecoveryV2Attempt(directory, cfg, expected)
	}
	path := filepath.Join(directory, "campaign.json")
	campaign, err := loadEvaluationCampaign(path)
	if err != nil {
		return false, fmt.Errorf("read evaluation campaign")
	}
	if cfg.Mode == EngineeringInsightQualificationRunMode {
		if campaign.QualificationRequests >= qualificationRequestCap {
			return false, fmt.Errorf("qualification request budget is exhausted")
		}
		campaign.QualificationRequests++
	} else {
		cap, err := developmentRequestCap(directory, campaign.DevelopmentRequests, cfg)
		if err != nil {
			return false, err
		}
		if campaign.DevelopmentRequests >= cap {
			return false, fmt.Errorf("development request budget is exhausted")
		}
		campaign.DevelopmentRequests++
	}
	return true, writeRunnerJSON(path, campaign)
}

func reserveRecoveryV2Attempt(directory string, cfg EngineeringInsightRunnerConfig, expected EngineeringInsightExpectedAttempt) (bool, error) {
	period, err := loadRecoveryV2Period(directory, cfg.Root)
	if err != nil {
		return false, err
	}
	if err := validateRecoveryV2ReservationBoundary(directory, cfg, period); err != nil {
		return false, err
	}
	id, err := bindRecoveryV2Schedule(&period, cfg, expected)
	if err != nil {
		return false, err
	}
	if period.Reservations[id] {
		return false, nil
	}
	if err := consumeRecoveryV2Request(&period, cfg.Mode); err != nil {
		return false, err
	}
	period.Reservations[id] = true
	if err := writeRunnerJSON(recoveryV2PeriodPath(directory), period); err != nil {
		return false, err
	}
	return true, nil
}

func validateRecoveryV2ReservationBoundary(directory string, cfg EngineeringInsightRunnerConfig, period engineeringInsightRecoveryV2Period) error {
	actualBase, err := cleanCanonicalRecoveryBase(period.CanonicalRoot)
	if err != nil || actualBase != period.BaseRevision || cfg.BaseRevision != period.BaseRevision {
		return fmt.Errorf("recovery evaluation base changed after authorization")
	}
	if err := validateRecoveryV2Predecessors(period.CanonicalRoot); err != nil {
		return err
	}
	if cfg.Mode == EngineeringInsightQualificationRunMode && !recoveryV2DevelopmentAccepted(directory, period) {
		return fmt.Errorf("recovery qualification requires accepted development evidence")
	}
	return nil
}

func bindRecoveryV2Schedule(period *engineeringInsightRecoveryV2Period, cfg EngineeringInsightRunnerConfig, expected EngineeringInsightExpectedAttempt) (string, error) {
	schedule := recoveryV2ScheduleIDs(cfg.Cases)
	boundSchedule := &period.DevelopmentSchedule
	if cfg.Mode == EngineeringInsightQualificationRunMode {
		boundSchedule = &period.QualificationSchedule
	}
	if len(*boundSchedule) == 0 {
		*boundSchedule = schedule
	} else if !sameStringSchedule(*boundSchedule, schedule) {
		return "", fmt.Errorf("recovery evaluation schedule changed after reservation")
	}
	id := expectedAttemptID(expected)
	if !scheduleContains(*boundSchedule, id) {
		return "", fmt.Errorf("recovery evaluation attempt is outside the authorized schedule")
	}
	return id, nil
}

func consumeRecoveryV2Request(period *engineeringInsightRecoveryV2Period, mode string) error {
	if mode == EngineeringInsightQualificationRunMode {
		if period.QualificationRequests >= recoveryV2QualificationRequests {
			return fmt.Errorf("recovery qualification request budget is exhausted")
		}
		period.QualificationRequests++
		period.CumulativeQualificationRequests++
		return nil
	}
	if period.DevelopmentRequests >= recoveryV2DevelopmentRequests {
		return fmt.Errorf("recovery development request budget is exhausted")
	}
	period.DevelopmentRequests++
	period.CumulativeDevelopmentRequests++
	return nil
}

func loadRecoveryV2Period(directory, root string) (engineeringInsightRecoveryV2Period, error) {
	data, err := os.ReadFile(recoveryV2PeriodPath(directory))
	if err != nil {
		return engineeringInsightRecoveryV2Period{}, fmt.Errorf("recovery successor period is not authorized")
	}
	if ValidateStrictJSONDocument(data) != nil {
		return engineeringInsightRecoveryV2Period{}, fmt.Errorf("recovery successor period is invalid")
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil || !validRecoveryV2PeriodFields(fields) {
		return engineeringInsightRecoveryV2Period{}, fmt.Errorf("recovery successor period is invalid")
	}
	var period engineeringInsightRecoveryV2Period
	if json.Unmarshal(data, &period) != nil || !validRecoveryV2Period(period, root) {
		return engineeringInsightRecoveryV2Period{}, fmt.Errorf("recovery successor period is invalid")
	}
	return period, nil
}

func validRecoveryV2PeriodFields(fields map[string]json.RawMessage) bool {
	want := []string{"version", "period_id", "canonical_root", "provider", "endpoint", "candidate_id", "model", "prompt_version", "corpus_id", "development_corpus_digest", "qualification_corpus_digest", "base_revision", "schema_identity", "max_output_tokens", "input_context_tokens", "runtime_context_tokens", "attempt_timeout_seconds", "reasoning_effort", "temperature", "top_p", "top_k", "lanes", "development_requests", "qualification_requests", "cumulative_development_requests", "cumulative_qualification_requests", "development_limit", "qualification_limit", "predecessor_files_sha256", "development_schedule", "qualification_schedule", "reservations"}
	return validRequiredJSONObject(fields, want)
}

func validRecoveryV2Period(period engineeringInsightRecoveryV2Period, root string) bool {
	canonicalRoot, err := canonicalEvaluationRoot(root)
	schemaIdentity, schemaErr := FileAnalysisResponseSchema().Identity()
	if err != nil || schemaErr != nil || !sameRecoveryV2PeriodIdentity(period, canonicalRoot) || !sameRecoveryV2PeriodProfile(period, schemaIdentity) || !validRecoveryV2PeriodAccounting(period) {
		return false
	}
	if !sameRecoveryV2Predecessors(period.PredecessorFiles) || !validRecoveryV2PeriodSchedules(period) {
		return false
	}
	developmentReservations, qualificationReservations, valid := recoveryV2ReservationCounts(period)
	return valid && developmentReservations == period.DevelopmentRequests && qualificationReservations == period.QualificationRequests
}

func sameRecoveryV2Predecessors(actual map[string]string) bool {
	for name, digest := range recoveryV2PredecessorFiles {
		if actual[name] != digest {
			return false
		}
	}
	return true
}

func validRecoveryV2PeriodSchedules(period engineeringInsightRecoveryV2Period) bool {
	return validBoundRecoveryV2Schedule(period.DevelopmentSchedule, "development", recoveryV2DevelopmentRequests, 8, 4, period.DevelopmentRequests) && validBoundRecoveryV2Schedule(period.QualificationSchedule, "qualification", recoveryV2QualificationRequests, 16, 8, period.QualificationRequests)
}

func recoveryV2ReservationCounts(period engineeringInsightRecoveryV2Period) (int, int, bool) {
	allowed := make(map[string]string, len(period.DevelopmentSchedule)+len(period.QualificationSchedule))
	for _, id := range period.DevelopmentSchedule {
		allowed[id] = "development"
	}
	for _, id := range period.QualificationSchedule {
		allowed[id] = "qualification"
	}
	developmentReservations, qualificationReservations := 0, 0
	for id, reserved := range period.Reservations {
		partition, authorized := allowed[id]
		if !reserved || !authorized {
			return 0, 0, false
		}
		switch partition {
		case "development":
			developmentReservations++
		case "qualification":
			qualificationReservations++
		default:
			return 0, 0, false
		}
	}
	return developmentReservations, qualificationReservations, true
}

func validBoundRecoveryV2Schedule(schedule []string, partition string, count, substantive, controls, consumed int) bool {
	if len(schedule) == 0 {
		return consumed == 0
	}
	if len(schedule) != count {
		return false
	}
	seen := make(map[string]bool, count)
	actualSubstantive, actualControls := 0, 0
	for _, id := range schedule {
		parts := strings.Split(id, "\x00")
		if len(parts) != 5 || parts[0] == "" || parts[1] != partition || parts[3] != "1" || parts[4] != "1" || seen[id] {
			return false
		}
		seen[id] = true
		switch parts[2] {
		case "substantive":
			actualSubstantive++
		case "control":
			actualControls++
		default:
			return false
		}
	}
	return actualSubstantive == substantive && actualControls == controls
}

func recoveryV2ScheduleIDs(cases []EngineeringInsightRunnerCase) []string {
	result := make([]string, len(cases))
	for index, item := range cases {
		result[index] = expectedAttemptID(item.Expected)
	}
	return result
}

func sameStringSchedule(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func scheduleContains(schedule []string, id string) bool {
	for _, scheduled := range schedule {
		if scheduled == id {
			return true
		}
	}
	return false
}

func sameRecoveryV2PeriodIdentity(period engineeringInsightRecoveryV2Period, root string) bool {
	return period.Version == "v2" && period.PeriodID == recoveryV2PeriodID && period.CanonicalRoot == root && period.Provider == recoveryV2Provider && period.Endpoint == recoveryV2Endpoint && period.CandidateID == recoveryV2CandidateID && period.Model == recoveryV2Model && period.PromptVersion == recoveryV2PromptVersion && period.CorpusID == recoveryV2CorpusID && period.DevelopmentCorpusDigest == recoveryV2DevelopmentDigest && period.QualificationCorpusDigest == recoveryV2QualificationDigest && validRecoveryV2BaseRevision(period.BaseRevision)
}

func sameRecoveryV2PeriodProfile(period engineeringInsightRecoveryV2Period, schemaIdentity string) bool {
	return period.SchemaIdentity == schemaIdentity && period.MaxOutputTokens == qualificationOutputTokenCap && period.InputContextTokens == 16384 && period.RuntimeContextTokens == recoveryV2RuntimeContextTokens && period.AttemptTimeoutSeconds == qualificationAttemptTimeoutSeconds && period.ReasoningEffort == mediumDevelopmentReasoningEffort && period.Temperature == 1 && period.TopP == .95 && period.TopK == 20 && period.Lanes == 1
}

func validRecoveryV2PeriodAccounting(period engineeringInsightRecoveryV2Period) bool {
	return period.DevelopmentLimit == recoveryV2DevelopmentLimit && period.QualificationLimit == recoveryV2QualificationLimit && period.DevelopmentRequests >= 0 && period.DevelopmentRequests <= recoveryV2DevelopmentRequests && period.QualificationRequests >= 0 && period.QualificationRequests <= recoveryV2QualificationRequests && period.CumulativeDevelopmentRequests == 48+period.DevelopmentRequests && period.CumulativeQualificationRequests == 24+period.QualificationRequests && len(period.PredecessorFiles) == len(recoveryV2PredecessorFiles)
}

func recoveryV2DevelopmentAccepted(directory string, period engineeringInsightRecoveryV2Period) bool {
	manifest, exists, err := loadRunManifest(filepath.Join(directory, recoveryV2DevelopmentRunID+".json"))
	if err != nil || !exists || !manifest.Finished || len(period.DevelopmentSchedule) != recoveryV2DevelopmentRequests || len(manifest.Receipt.Attempts) != recoveryV2DevelopmentRequests {
		return false
	}
	schedule := make([]EngineeringInsightExpectedAttempt, len(period.DevelopmentSchedule))
	for index, id := range period.DevelopmentSchedule {
		parts := strings.Split(id, "\x00")
		schedule[index] = EngineeringInsightExpectedAttempt{CaseName: parts[0], Partition: parts[1], Intent: parts[2], Repetition: 1, Attempt: 1}
	}
	expected := EngineeringInsightEvaluationExpectation{ProtocolVersion: "v2", CandidateID: period.CandidateID, Provider: period.Provider, Model: period.Model, PromptVersion: period.PromptVersion, CorpusID: period.CorpusID, CorpusDigest: period.DevelopmentCorpusDigest, BaseRevision: period.BaseRevision, MaxRequests: recoveryV2DevelopmentRequests, MaxOutputTokens: period.MaxOutputTokens, AttemptTimeoutSeconds: period.AttemptTimeoutSeconds, Schedule: schedule}
	if _, err := ValidateEngineeringInsightEvaluationReceipt(manifest.Receipt, expected); err != nil || !manifest.Receipt.DevelopmentGatePassed() {
		return false
	}
	return manifest.Receipt.CandidateID == period.CandidateID && manifest.Receipt.CorpusDigest == period.DevelopmentCorpusDigest
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
	if decoder.Decode(&campaign) != nil || campaign.DevelopmentRequests < 0 || campaign.DevelopmentRequests > mediumDevelopmentRequestCap || campaign.QualificationRequests < 0 || campaign.QualificationRequests > qualificationRequestCap {
		return engineeringInsightCampaign{}, fmt.Errorf("invalid campaign")
	}
	return campaign, nil
}

func developmentRequestCap(directory string, consumed int, cfg EngineeringInsightRunnerConfig) (int, error) {
	ledger, _, err := loadDevelopmentGrantLedger(directory)
	if err != nil {
		return 0, err
	}
	cap := developmentGrantCap(ledger)
	if cap > mediumDevelopmentRequestCap {
		return 0, fmt.Errorf("invalid development grant ledger")
	}
	if err := validateDevelopmentGrantDispatch(ledger.Grants, consumed, cfg); err != nil {
		return 0, err
	}
	return cap, nil
}

func validateDevelopmentGrantDispatch(grants []engineeringInsightDevelopmentGrant, consumed int, cfg EngineeringInsightRunnerConfig) error {
	for _, window := range []engineeringInsightDevelopmentGrantWindow{
		{grantIndex: 1, minimum: extendedDevelopmentRequestCap, maximum: recoveryDevelopmentRequestCap, errorText: "development recovery grant does not match dispatch identity"},
		{grantIndex: 2, minimum: recoveryDevelopmentRequestCap, maximum: finalDevelopmentRequestCap, errorText: "development v11 grant does not match dispatch identity"},
		{grantIndex: 3, minimum: finalDevelopmentRequestCap, maximum: structuredDevelopmentRequestCap, errorText: "development structured-output grant does not match dispatch identity"},
		{grantIndex: 4, minimum: structuredDevelopmentRequestCap, maximum: thinkingOffDevelopmentRequestCap, errorText: "development thinking-off grant does not match dispatch identity"},
		{grantIndex: 5, minimum: thinkingOffDevelopmentRequestCap, maximum: thinkingSchemaDevelopmentRequestCap, errorText: "development thinking-schema grant does not match dispatch identity"},
		{grantIndex: 6, minimum: thinkingSchemaDevelopmentRequestCap, errorText: "development medium-reasoning grant does not match dispatch identity"},
	} {
		if len(grants) > window.grantIndex && consumed >= window.minimum && (window.maximum == 0 || consumed < window.maximum) && !matchesDevelopmentGrantDispatch(cfg, grants[window.grantIndex]) {
			return fmt.Errorf("%s", window.errorText)
		}
	}
	return nil
}

func matchesDevelopmentGrantDispatch(cfg EngineeringInsightRunnerConfig, grant engineeringInsightDevelopmentGrant) bool {
	return cfg.CandidateID == grant.CandidateID &&
		cfg.Model == grant.Model &&
		cfg.Profile.Model == grant.Model &&
		(grant.PromptVersion == "" || cfg.PromptVersion == grant.PromptVersion) &&
		(grant.ReasoningEffort == "" || cfg.Profile.ReasoningEffort == grant.ReasoningEffort) &&
		isLoopbackURL(cfg.Profile.APIBaseURL)
}

func developmentGrantLedgerPath(directory string) string {
	return filepath.Join(directory, "development-grants.json")
}

func loadDevelopmentGrantLedger(directory string) (engineeringInsightDevelopmentGrantLedger, bool, error) {
	path := developmentGrantLedgerPath(directory)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return engineeringInsightDevelopmentGrantLedger{}, false, nil
	}
	if err != nil || ValidateStrictJSONDocument(data) != nil {
		return engineeringInsightDevelopmentGrantLedger{}, false, fmt.Errorf("invalid development grant ledger")
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil || len(fields) != 1 || fields["grants"] == nil {
		return engineeringInsightDevelopmentGrantLedger{}, false, fmt.Errorf("invalid development grant ledger")
	}
	var ledger engineeringInsightDevelopmentGrantLedger
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&ledger) != nil || requireJSONEOF(decoder) != nil || !validDevelopmentGrantLedger(ledger) {
		return engineeringInsightDevelopmentGrantLedger{}, false, fmt.Errorf("invalid development grant ledger")
	}
	return ledger, true, nil
}

func validDevelopmentGrantLedger(ledger engineeringInsightDevelopmentGrantLedger) bool {
	if len(ledger.Grants) < 1 || len(ledger.Grants) > 7 {
		return false
	}
	seen := make(map[string]bool, len(ledger.Grants))
	for index, grant := range ledger.Grants {
		if !validRunnerID(grant.AuthorizationID) || grant.Requests != developmentGrantRequestCount || seen[grant.AuthorizationID] {
			return false
		}
		if !validDevelopmentGrantAt(index, grant) {
			return false
		}
		seen[grant.AuthorizationID] = true
	}
	return true
}

func validDevelopmentGrantAt(index int, grant engineeringInsightDevelopmentGrant) bool {
	identity := engineeringInsightDevelopmentGrantIdentity{CandidateID: grant.CandidateID, Model: grant.Model, PromptVersion: grant.PromptVersion, ReasoningEffort: grant.ReasoningEffort}
	switch index {
	case 0:
		return validOriginalDevelopmentGrant(grant.AuthorizationID, identity)
	case 1:
		return validRecoveryDevelopmentGrant(grant.AuthorizationID, identity)
	case 2:
		return validV11DevelopmentGrant(grant.AuthorizationID, identity)
	case 3:
		return validV12DevelopmentGrant(grant.AuthorizationID, identity)
	case 4:
		return validThinkingOffDevelopmentGrant(grant.AuthorizationID, identity)
	case 5:
		return validThinkingSchemaDevelopmentGrant(grant.AuthorizationID, identity)
	case 6:
		return validMediumDevelopmentGrant(grant.AuthorizationID, identity)
	default:
		return false
	}
}

func unknownReservedAttempt(expected EngineeringInsightExpectedAttempt, candidate string) EngineeringInsightEvaluationAttempt {
	return EngineeringInsightEvaluationAttempt{CaseName: expected.CaseName, Partition: expected.Partition, Intent: expected.Intent, Repetition: expected.Repetition, Attempt: expected.Attempt, CandidateID: candidate, Outcome: "unknown", OptionalInsight: "not_evaluated", FinishReason: "unknown"}
}

func executeRunnerAttempt(parent context.Context, cfg EngineeringInsightRunnerConfig, item EngineeringInsightRunnerCase) (EngineeringInsightEvaluationAttempt, string, fileAnalysisOptionalDiagnosticsResult, error) {
	root, prompt, target, err := prepareRunnerPrompt(cfg, item)
	if err != nil {
		return EngineeringInsightEvaluationAttempt{}, "", fileAnalysisOptionalDiagnosticsResult{}, err
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

func dispatchRunnerPrompt(parent context.Context, cfg EngineeringInsightRunnerConfig, item EngineeringInsightRunnerCase, prompt string, target *project.IndexFile) (EngineeringInsightEvaluationAttempt, string, fileAnalysisOptionalDiagnosticsResult, error) {
	timed, cancel := context.WithTimeout(parent, qualificationAttemptTimeoutSeconds*time.Second)
	defer cancel()
	started := time.Now()
	response, err := cfg.Client.ChatWithJSONSchema(timed, []llm.ChatMessage{{Role: "user", Content: prompt}}, FileAnalysisResponseSchema())
	elapsed := time.Since(started).Milliseconds()
	if err != nil {
		attempt := failedRunnerAttemptWithElapsed(item.Expected, cfg.CandidateID, err, elapsed)
		if errors.Is(err, llm.ErrStructuredRequestRejected) {
			return attempt, "", fileAnalysisOptionalDiagnosticsResult{}, err
		}
		return attempt, "", fileAnalysisOptionalDiagnosticsResult{}, nil
	}
	message := response.Choices[0].Message
	content := message.Content
	emitted := evaluationResponseMaterial(message)
	attempt := EngineeringInsightEvaluationAttempt{CaseName: item.Expected.CaseName, Partition: item.Expected.Partition, Intent: item.Expected.Intent, Repetition: item.Expected.Repetition, Attempt: item.Expected.Attempt, CandidateID: cfg.CandidateID, OutputTokens: response.Usage.CompletionTokens, ElapsedMilliseconds: float64(elapsed), OptionalInsight: "not_evaluated"}
	if emitted != "" {
		digest := sha256.Sum256([]byte(emitted))
		attempt.EmittedResponse = true
		attempt.ResponseDigest = hex.EncodeToString(digest[:])
	}
	if attempt.OutputTokens < 0 || attempt.OutputTokens > qualificationOutputTokenCap || response.Model != "" && response.Model != cfg.Model {
		attempt.Outcome, attempt.FinishReason = "failed", "error"
		attempt.InvalidProviderMetadata = true
		return attempt, emitted, fileAnalysisOptionalDiagnostics(content, *target, item.Source), nil
	}
	if response.Choices[0].FinishReason == "length" {
		attempt.Outcome, attempt.FinishReason = "truncated", "length"
		return attempt, emitted, fileAnalysisOptionalDiagnostics(content, *target, item.Source), nil
	}
	if response.Choices[0].FinishReason != "" && response.Choices[0].FinishReason != "stop" {
		attempt.Outcome, attempt.FinishReason = "failed", "error"
		return attempt, emitted, fileAnalysisOptionalDiagnostics(content, *target, item.Source), nil
	}
	if strings.TrimSpace(content) == "" {
		attempt.Outcome, attempt.FinishReason = "malformed", "stop"
		return attempt, emitted, fileAnalysisOptionalDiagnosticsResult{}, nil
	}
	parsed, parseErr := parseSemanticAnalysis(content, *target, item.Source)
	if parseErr != nil {
		attempt.Outcome, attempt.FinishReason = "malformed", "stop"
		return attempt, emitted, fileAnalysisOptionalDiagnostics(content, *target, item.Source), nil
	}
	attempt.Outcome, attempt.FinishReason, attempt.UsableSummary = "completed", "stop", true
	state := evaluateOptionalState(content, *target, item.Source, parsed)
	attempt.OptionalInsight = state.insight
	attempt.OptionalSectionDegraded = state.degraded()
	attempt.CompleteSummary = !attempt.OptionalSectionDegraded
	return attempt, emitted, fileAnalysisOptionalDiagnostics(content, *target, item.Source), nil
}

// evaluationResponseMaterial retains every non-empty provider response field
// for private scoring. The semantic parser receives only final content, so
// separately returned reasoning is never interpreted as a JSON prefix.
func evaluationResponseMaterial(message llm.ChatMessage) string {
	parts := make([]string, 0, 3)
	for _, value := range []string{message.ReasoningContent, message.Reasoning, message.Content} {
		if strings.TrimSpace(value) != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, "\n\n")
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

type evaluatedOptionalState struct {
	insight                     string
	insightRejected             bool
	symbolExplanationsDegraded  bool
	taskSpecDegradedRiskIndices []int
}

func (state evaluatedOptionalState) degraded() bool {
	return state.insightRejected || state.nonInsightDegraded()
}

func (state evaluatedOptionalState) nonInsightDegraded() bool {
	return state.symbolExplanationsDegraded || len(state.taskSpecDegradedRiskIndices) > 0
}

func evaluateOptionalState(content string, target project.IndexFile, source string, parsed semanticAnalysisResponse) evaluatedOptionalState {
	var wire semanticAnalysisWireResponse
	if decodeSemanticAnalysis(content, &wire) != nil {
		return evaluatedOptionalState{insight: "rejected", insightRejected: true}
	}
	state := evaluatedOptionalState{insight: "omitted"}
	if _, err := normalizeSymbolExplanations(wire.SymbolExplanations, target.Symbols); err != nil {
		state.symbolExplanationsDegraded = true
	}
	insight, insightError := parseFileAnalysisEngineeringInsight(wire.EngineeringInsight)
	if insightError != "" {
		state.insight, state.insightRejected = "rejected", true
	} else if insight != nil || parsed.EngineeringInsight != nil {
		state.insight = "present"
	}
	for index, risk := range wire.Risks {
		next := riskOptionalState(risk, target, source)
		state.merge(next)
		if next.taskSpecDegraded {
			state.taskSpecDegradedRiskIndices = append(state.taskSpecDegradedRiskIndices, index)
		}
	}
	for _, suggestion := range wire.Suggestions {
		state.merge(suggestionOptionalState(suggestion))
	}
	return state
}

type optionalState struct{ present, rejected, taskSpecDegraded bool }

func (state *evaluatedOptionalState) merge(next optionalState) {
	if next.rejected {
		state.insight, state.insightRejected = "rejected", true
	} else if next.present && state.insight != "rejected" {
		state.insight = "present"
	}
}

func riskOptionalState(risk semanticAnalysisFinding, target project.IndexFile, source string) optionalState {
	state := optionalInsightState(risk.Insight)
	if optionalTaskSpecDegraded(risk.TaskSpec, target, source) {
		state.taskSpecDegraded = true
	}
	return state
}

func optionalTaskSpecDegraded(raw json.RawMessage, target project.IndexFile, source string) bool {
	return len(raw) > 0 && strings.TrimSpace(string(raw)) != "null" && parseOptionalBugTaskSpec(raw, target, source) == nil
}
func suggestionOptionalState(suggestion semanticAnalysisSuggestion) optionalState {
	return optionalInsightState(suggestion.Insight)
}
func optionalInsightState(raw json.RawMessage) optionalState {
	insight, reason := parseFileAnalysisEngineeringInsight(raw)
	return optionalState{present: insight != nil, rejected: reason != ""}
}

type fileAnalysisOptionalDiagnosticsResult struct {
	insights                    []engineeringInsightOptionalDiagnostic
	symbolExplanationsDegraded  bool
	taskSpecDegradedRiskIndices []int
}

func fileAnalysisOptionalDiagnostics(content string, target project.IndexFile, source string) fileAnalysisOptionalDiagnosticsResult {
	var wire semanticAnalysisWireResponse
	if decodeSemanticAnalysis(content, &wire) != nil {
		return fileAnalysisOptionalDiagnosticsResult{}
	}
	result := fileAnalysisOptionalDiagnosticsResult{insights: make([]engineeringInsightOptionalDiagnostic, 0, 1+len(wire.Risks)+len(wire.Suggestions))}
	if _, err := normalizeSymbolExplanations(wire.SymbolExplanations, target.Symbols); err != nil {
		result.symbolExplanationsDegraded = true
	}
	result.insights = append(result.insights, optionalInsightDiagnostic("top_level", nil, wire.EngineeringInsight))
	for index, risk := range wire.Risks {
		index := index
		result.insights = append(result.insights, optionalInsightDiagnostic("risk", &index, risk.Insight))
		if optionalTaskSpecDegraded(risk.TaskSpec, target, source) {
			result.taskSpecDegradedRiskIndices = append(result.taskSpecDegradedRiskIndices, index)
		}
	}
	for index, suggestion := range wire.Suggestions {
		index := index
		result.insights = append(result.insights, optionalInsightDiagnostic("suggestion", &index, suggestion.Insight))
	}
	return result
}

func optionalInsightDiagnostic(location string, index *int, raw json.RawMessage) engineeringInsightOptionalDiagnostic {
	_, diagnostic := parseFileAnalysisEngineeringInsightDiagnostic(raw)
	return engineeringInsightOptionalDiagnostic{
		Location:              location,
		Index:                 index,
		Reason:                diagnostic.Reason,
		Presence:              diagnostic.Presence,
		Mechanism:             diagnostic.Mechanism,
		WhyItMattersHere:      diagnostic.WhyItMattersHere,
		TradeoffOrFailureMode: diagnostic.TradeoffOrFailureMode,
		TransferableLesson:    diagnostic.TransferableLesson,
	}
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
	if manifest.TerminalReason != "" && (!manifest.Finished || manifest.TerminalReason != permanentRequestRejectionReason || len(manifest.Receipt.Attempts) == 0) {
		return fmt.Errorf("invalid evaluation manifest")
	}
	if !validManifestAttempts(manifest.Receipt, cfg.Cases) {
		return fmt.Errorf("invalid evaluation manifest")
	}
	return nil
}

func sameRunnerReceiptIdentity(actual, expected EngineeringInsightEvaluationReceipt) bool {
	return actual.ProtocolVersion == expected.ProtocolVersion && actual.Mode == expected.Mode && actual.RunID == expected.RunID && actual.CandidateID == expected.CandidateID && actual.Provider == expected.Provider && actual.Model == expected.Model && actual.PromptVersion == expected.PromptVersion && actual.CorpusID == expected.CorpusID && actual.CorpusDigest == expected.CorpusDigest && actual.BaseRevision == expected.BaseRevision && actual.MaxRequests == expected.MaxRequests && actual.MaxOutputTokens == expected.MaxOutputTokens && actual.AttemptTimeoutSeconds == expected.AttemptTimeoutSeconds
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

func privateOptionalDiagnosticsDirectory(directory, runID string) string {
	return filepath.Join(directory, "optional-diagnostics", runID)
}

func privateResponsePath(directory, id string) string {
	sum := sha256.Sum256([]byte(id))
	return filepath.Join(directory, hex.EncodeToString(sum[:])+".response")
}

func privateOptionalDiagnosticsPath(directory, id string) string {
	sum := sha256.Sum256([]byte(id))
	return filepath.Join(directory, hex.EncodeToString(sum[:])+".json")
}

func writePrivateResponse(directory, id, response string) error {
	if err := os.MkdirAll(directory, 0700); err != nil {
		return fmt.Errorf("write private evaluation response")
	}
	if err := writePrivateArtifact(privateResponsePath(directory, id), []byte(response)); err != nil {
		return fmt.Errorf("write private evaluation response")
	}
	return nil
}

func writePrivateOptionalDiagnostics(directory string, diagnostics engineeringInsightOptionalDiagnostics) error {
	if !validRunnerID(diagnostics.RunID) || diagnostics.AttemptID == "" || !validDigest(diagnostics.ResponseDigest) || !validPrivateOptionalDiagnostics(diagnostics) {
		return fmt.Errorf("write private optional diagnostics")
	}
	privateDirectory := privateOptionalDiagnosticsDirectory(directory, diagnostics.RunID)
	if err := os.MkdirAll(privateDirectory, 0700); err != nil {
		return fmt.Errorf("write private optional diagnostics")
	}
	data, err := json.Marshal(diagnostics)
	if err != nil {
		return fmt.Errorf("write private optional diagnostics")
	}
	if err := writePrivateArtifact(privateOptionalDiagnosticsPath(privateDirectory, diagnostics.AttemptID), data); err != nil {
		return fmt.Errorf("write private optional diagnostics")
	}
	return nil
}

func validPrivateOptionalDiagnostics(record engineeringInsightOptionalDiagnostics) bool {
	previousIndex := -1
	for _, index := range record.TaskSpecDegradedRiskIndices {
		if index < 0 || index <= previousIndex {
			return false
		}
		previousIndex = index
	}
	for _, diagnostic := range record.Insights {
		if !validPrivateOptionalDiagnosticReason(diagnostic.Reason) || !validPrivateOptionalDiagnosticPresence(diagnostic.Presence) || !validPrivateOptionalDiagnosticLocation(diagnostic.Location, diagnostic.Index) {
			return false
		}
		for _, field := range []project.OptionalEngineeringInsightFieldDiagnostic{diagnostic.Mechanism, diagnostic.WhyItMattersHere, diagnostic.TradeoffOrFailureMode, diagnostic.TransferableLesson} {
			if field.Runes < 0 {
				return false
			}
		}
	}
	return true
}

func validPrivateOptionalDiagnosticReason(reason project.OptionalEngineeringInsightReason) bool {
	switch reason {
	case project.OptionalEngineeringInsightAbsent, project.OptionalEngineeringInsightNull, project.OptionalEngineeringInsightInvalidShape, project.OptionalEngineeringInsightEmptyRequiredField, project.OptionalEngineeringInsightOverLimit, project.OptionalEngineeringInsightAccepted:
		return true
	default:
		return false
	}
}

func validPrivateOptionalDiagnosticPresence(presence project.OptionalEngineeringInsightPresence) bool {
	return presence == project.OptionalEngineeringInsightAbsentPresence || presence == project.OptionalEngineeringInsightNullPresence || presence == project.OptionalEngineeringInsightValuePresence
}

func validPrivateOptionalDiagnosticLocation(location string, index *int) bool {
	switch location {
	case "top_level":
		return index == nil
	case "risk", "suggestion":
		return index != nil && *index >= 0
	default:
		return false
	}
}

// writePrivateArtifact publishes a complete immutable file. Link creation
// fails if a resumed or uncertain attempt already owns this artifact, so a
// later process can never replace evidence from a prior dispatch.
func writePrivateArtifact(path string, data []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".private-evaluation-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Link(temporaryName, path)
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
