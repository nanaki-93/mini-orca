package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strings"
)

const (
	minimumRetainedEngineeringInsightScore = 6
	minimumUsableQualificationAttempts     = 23
	minimumCompleteQualificationAttempts   = 22
	minimumUsefulSubstantiveAttempts       = 13
	qualificationControlAttemptCount       = 8
	qualificationRequestCap                = 24
	qualificationOutputTokenCap            = 4096
	qualificationAttemptTimeoutSeconds     = 300
)

var (
	receiptJSONFields     = []string{"mode", "run_id", "candidate_id", "provider", "model", "prompt_version", "corpus_id", "corpus_digest", "base_revision", "max_requests", "max_output_tokens", "attempt_timeout_seconds", "consumption", "attempts"}
	consumptionJSONFields = []string{"requests", "output_tokens"}
	attemptJSONFields     = []string{"case_name", "partition", "intent", "repetition", "attempt", "candidate_id", "outcome", "usable_summary", "complete_summary", "optional_insight", "optional_section_degraded", "emitted_response", "response_digest", "output_tokens", "invalid_provider_metadata", "finish_reason", "elapsed_milliseconds", "score"}
	scoreJSONFields       = []string{"response_digest", "correctness", "local_relevance", "tradeoff_clarity", "useful_verification", "critical_false_claim", "retain_example"}
)

type EngineeringInsightEvaluationMode string

const (
	EngineeringInsightCollectionMode    EngineeringInsightEvaluationMode = "collection"
	EngineeringInsightQualificationMode EngineeringInsightEvaluationMode = "qualification"
)

type EngineeringInsightEvaluationOutcome string

const (
	EngineeringInsightCollectionOutcome EngineeringInsightEvaluationOutcome = "collection"
	EngineeringInsightIncompleteOutcome EngineeringInsightEvaluationOutcome = "incomplete"
	EngineeringInsightFailedOutcome     EngineeringInsightEvaluationOutcome = "failed"
	EngineeringInsightPassedOutcome     EngineeringInsightEvaluationOutcome = "passed"
)

// EngineeringInsightEvaluationReceipt stores source-free, reviewer-scored evidence.
type EngineeringInsightEvaluationReceipt struct {
	ProtocolVersion       string                                  `json:"protocol_version,omitempty"`
	Mode                  EngineeringInsightEvaluationMode        `json:"mode"`
	RunID                 string                                  `json:"run_id"`
	CandidateID           string                                  `json:"candidate_id"`
	Provider              string                                  `json:"provider"`
	Model                 string                                  `json:"model"`
	PromptVersion         string                                  `json:"prompt_version"`
	CorpusID              string                                  `json:"corpus_id"`
	CorpusDigest          string                                  `json:"corpus_digest"`
	BaseRevision          string                                  `json:"base_revision"`
	MaxRequests           int                                     `json:"max_requests"`
	MaxOutputTokens       int                                     `json:"max_output_tokens"`
	AttemptTimeoutSeconds int                                     `json:"attempt_timeout_seconds"`
	Consumption           EngineeringInsightEvaluationConsumption `json:"consumption"`
	Attempts              []EngineeringInsightEvaluationAttempt   `json:"attempts"`
}

type EngineeringInsightEvaluationConsumption struct {
	Requests     int `json:"requests"`
	OutputTokens int `json:"output_tokens"`
}

type EngineeringInsightEvaluationAttempt struct {
	CaseName                string                          `json:"case_name"`
	Partition               string                          `json:"partition"`
	Intent                  string                          `json:"intent"`
	Repetition              int                             `json:"repetition"`
	Attempt                 int                             `json:"attempt"`
	CandidateID             string                          `json:"candidate_id"`
	Outcome                 string                          `json:"outcome"`
	UsableSummary           bool                            `json:"usable_summary"`
	CompleteSummary         bool                            `json:"complete_summary"`
	OptionalInsight         string                          `json:"optional_insight"`
	OptionalSectionDegraded bool                            `json:"optional_section_degraded"`
	EmittedResponse         bool                            `json:"emitted_response"`
	ResponseDigest          string                          `json:"response_digest"`
	OutputTokens            int                             `json:"output_tokens"`
	InvalidProviderMetadata bool                            `json:"invalid_provider_metadata"`
	FinishReason            string                          `json:"finish_reason"`
	ElapsedMilliseconds     float64                         `json:"elapsed_milliseconds"`
	Score                   *EngineeringInsightAttemptScore `json:"score"`
}

// EngineeringInsightAttemptScore is joined to a response by digest only.
type EngineeringInsightAttemptScore struct {
	ResponseDigest     string `json:"response_digest"`
	Correctness        int    `json:"correctness"`
	LocalRelevance     int    `json:"local_relevance"`
	TradeoffClarity    int    `json:"tradeoff_clarity"`
	UsefulVerification int    `json:"useful_verification"`
	CriticalFalseClaim bool   `json:"critical_false_claim"`
	RetainExample      bool   `json:"retain_example"`
}

func (score EngineeringInsightAttemptScore) Total() int {
	return score.Correctness + score.LocalRelevance + score.TradeoffClarity + score.UsefulVerification
}

// DevelopmentGatePassed is deliberately stricter than historical collection
// validation. The v2 promotion gate needs an independent whole-final verdict
// for every emitted response, including intentional omission controls.
func (receipt EngineeringInsightEvaluationReceipt) DevelopmentGatePassed() bool {
	if receipt.ProtocolVersion != "v2" || receipt.Mode != EngineeringInsightCollectionMode || len(receipt.Attempts) != 12 || receipt.Consumption.Requests != 12 {
		return false
	}
	seen := make(map[string]bool, 12)
	counts := developmentGateCounts{}
	for _, attempt := range receipt.Attempts {
		if !counts.record(attempt, seen) {
			return false
		}
	}
	return counts.valid(len(seen))
}

type developmentGateCounts struct{ usable, complete, useful, omitted, substantive, controls int }

func (counts *developmentGateCounts) record(attempt EngineeringInsightEvaluationAttempt, seen map[string]bool) bool {
	id := attemptScoreID(attempt)
	if seen[id] || !validDevelopmentGateAttempt(attempt) {
		return false
	}
	seen[id] = true
	counts.usable++
	counts.complete++
	if attempt.Intent == "substantive" {
		counts.substantive++
		if attempt.OptionalInsight == "present" && attempt.Score.Total() >= minimumRetainedEngineeringInsightScore {
			counts.useful++
		}
		return true
	}
	if attempt.Intent == "control" {
		counts.controls++
		if attempt.OptionalInsight == "omitted" {
			counts.omitted++
		}
		return true
	}
	return false
}

func (counts developmentGateCounts) valid(unique int) bool {
	return unique == 12 && counts.usable == 12 && counts.complete == 12 && counts.substantive == 8 && counts.controls == 4 && counts.useful == 8 && counts.omitted == 4
}

func validDevelopmentGateAttempt(attempt EngineeringInsightEvaluationAttempt) bool {
	return attempt.Partition == "development" && attempt.Repetition == 1 && attempt.Attempt == 1 && attempt.EmittedResponse && attempt.UsableSummary && attempt.CompleteSummary && attempt.Score != nil && !attempt.Score.CriticalFalseClaim
}

// EngineeringInsightExpectedAttempt is selected outside the receipt.
type EngineeringInsightExpectedAttempt struct {
	CaseName   string
	Partition  string
	Intent     string
	Repetition int
	Attempt    int
}

type EngineeringInsightEvaluationExpectation struct {
	ProtocolVersion       string
	CandidateID           string
	Provider              string
	Model                 string
	PromptVersion         string
	CorpusID              string
	CorpusDigest          string
	BaseRevision          string
	MaxRequests           int
	MaxOutputTokens       int
	AttemptTimeoutSeconds int
	Schedule              []EngineeringInsightExpectedAttempt
}

type EngineeringInsightLatencyStatistics struct {
	SuccessfulCount           int
	MedianMilliseconds        float64
	P95Milliseconds           float64
	TimeoutOrCensoredAttempts int
}

type EngineeringInsightEvaluationReport struct {
	Outcome                   EngineeringInsightEvaluationOutcome
	ScheduledAttempts         int
	RecordedAttempts          int
	UsableAttempts            int
	CompleteAttempts          int
	UsefulSubstantiveAttempts int
	OmittedControls           int
	CriticalFalseClaims       int
	InvalidProviderMetadata   int
	OutcomeCounts             map[string]int
	Latency                   EngineeringInsightLatencyStatistics
}

// DecodeEngineeringInsightEvaluationReceipt rejects malformed evidence without
// echoing arbitrary receipt content.
func DecodeEngineeringInsightEvaluationReceipt(data []byte) (EngineeringInsightEvaluationReceipt, error) {
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return EngineeringInsightEvaluationReceipt{}, fmt.Errorf("invalid evaluation receipt")
	}
	if err := requireReceiptFields(data); err != nil {
		return EngineeringInsightEvaluationReceipt{}, fmt.Errorf("invalid evaluation receipt")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var receipt EngineeringInsightEvaluationReceipt
	if err := decoder.Decode(&receipt); err != nil || requireJSONEOF(decoder) != nil {
		return EngineeringInsightEvaluationReceipt{}, fmt.Errorf("invalid evaluation receipt")
	}
	return receipt, nil
}

// DecodeEngineeringInsightScores accepts only digest-bound reviewer scores.
// It applies the receipt's strict JSON rules before any private handoff is joined.
func DecodeEngineeringInsightScores(data []byte) (map[string]EngineeringInsightAttemptScore, error) {
	if rejectDuplicateJSONKeys(data) != nil {
		return nil, fmt.Errorf("invalid evaluation scores")
	}
	var raw map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(data))
	if decoder.Decode(&raw) != nil || requireJSONEOF(decoder) != nil || len(raw) == 0 {
		return nil, fmt.Errorf("invalid evaluation scores")
	}
	result := make(map[string]EngineeringInsightAttemptScore, len(raw))
	for id, value := range raw {
		var fields map[string]json.RawMessage
		if strings.TrimSpace(id) == "" || json.Unmarshal(value, &fields) != nil || !validRequiredJSONObject(fields, scoreJSONFields) {
			return nil, fmt.Errorf("invalid evaluation scores")
		}
		valueDecoder := json.NewDecoder(bytes.NewReader(value))
		valueDecoder.DisallowUnknownFields()
		var score EngineeringInsightAttemptScore
		if valueDecoder.Decode(&score) != nil || requireJSONEOF(valueDecoder) != nil || !validDigest(score.ResponseDigest) || !validScore(score) {
			return nil, fmt.Errorf("invalid evaluation scores")
		}
		result[id] = score
	}
	return result, nil
}

// ValidateStrictJSONDocument rejects duplicate object keys and trailing JSON.
// Callers can combine it with their own schema validation without revealing input.
func ValidateStrictJSONDocument(data []byte) error {
	return rejectDuplicateJSONKeys(data)
}

func requireReceiptFields(data []byte) error {
	var receipt map[string]json.RawMessage
	if err := json.Unmarshal(data, &receipt); err != nil || !hasJSONFields(receipt, receiptJSONFields...) || !nonNullJSONFields(receipt, receiptJSONFields...) {
		return fmt.Errorf("missing receipt field")
	}
	version, versioned := receipt["protocol_version"]
	if !versioned {
		if !onlyJSONFields(receipt, receiptJSONFields...) {
			return fmt.Errorf("invalid receipt field")
		}
	} else if string(version) != `"v2"` || !onlyJSONFields(receipt, append(receiptJSONFields, "protocol_version")...) {
		return fmt.Errorf("invalid protocol version")
	}
	if err := requireConsumptionFields(receipt["consumption"]); err != nil {
		return err
	}
	return requireAttemptFields(receipt["attempts"])
}

func requireConsumptionFields(data json.RawMessage) error {
	var consumption map[string]json.RawMessage
	if err := json.Unmarshal(data, &consumption); err != nil || !validRequiredJSONObject(consumption, consumptionJSONFields) {
		return fmt.Errorf("missing consumption field")
	}
	return nil
}

func requireAttemptFields(data json.RawMessage) error {
	var attempts []map[string]json.RawMessage
	if err := json.Unmarshal(data, &attempts); err != nil {
		return fmt.Errorf("invalid attempts")
	}
	for _, attempt := range attempts {
		if !hasJSONFields(attempt, attemptJSONFields...) || !onlyJSONFields(attempt, attemptJSONFields...) || !nonNullJSONFields(attempt, attemptJSONFields[:len(attemptJSONFields)-1]...) {
			return fmt.Errorf("missing attempt field")
		}
		if err := requireScoreFields(attempt["score"]); err != nil {
			return fmt.Errorf("missing score field")
		}
	}
	return nil
}

func requireScoreFields(data json.RawMessage) error {
	if string(data) == "null" {
		return nil
	}
	var score map[string]json.RawMessage
	if err := json.Unmarshal(data, &score); err != nil || !validRequiredJSONObject(score, scoreJSONFields) {
		return fmt.Errorf("invalid score")
	}
	return nil
}

func validRequiredJSONObject(values map[string]json.RawMessage, fields []string) bool {
	return hasJSONFields(values, fields...) && onlyJSONFields(values, fields...) && nonNullJSONFields(values, fields...)
}

func hasJSONFields(values map[string]json.RawMessage, fields ...string) bool {
	for _, field := range fields {
		if _, present := values[field]; !present {
			return false
		}
	}
	return true
}

func onlyJSONFields(values map[string]json.RawMessage, fields ...string) bool {
	allowed := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		allowed[field] = struct{}{}
	}
	for field := range values {
		if _, known := allowed[field]; !known {
			return false
		}
	}
	return true
}

func nonNullJSONFields(values map[string]json.RawMessage, fields ...string) bool {
	for _, field := range fields {
		if string(values[field]) == "null" {
			return false
		}
	}
	return true
}

// ValidateEngineeringInsightEvaluationReceipt validates a receipt against a
// caller-selected candidate, corpus, limits and schedule. Collection evidence may
// be unscored; qualification evidence with an emitted response may not.
func ValidateEngineeringInsightEvaluationReceipt(receipt EngineeringInsightEvaluationReceipt, expected EngineeringInsightEvaluationExpectation) (EngineeringInsightEvaluationReport, error) {
	if err := validateEvaluationExpectation(expected); err != nil {
		return EngineeringInsightEvaluationReport{}, err
	}
	if err := validateReceiptIdentity(receipt, expected); err != nil {
		return EngineeringInsightEvaluationReport{}, err
	}
	if err := validateReceiptConsumption(receipt); err != nil {
		return EngineeringInsightEvaluationReport{}, err
	}
	recorded, err := validateAttempts(receipt, expected)
	if err != nil {
		return EngineeringInsightEvaluationReport{}, err
	}
	report := evaluationReport(expected, recorded)
	if receipt.Mode == EngineeringInsightCollectionMode {
		report.Outcome = EngineeringInsightCollectionOutcome
		return report, nil
	}
	if len(recorded) != len(expected.Schedule) {
		report.Outcome = EngineeringInsightIncompleteOutcome
		return report, nil
	}
	if report.UsableAttempts < minimumUsableQualificationAttempts || report.CompleteAttempts < minimumCompleteQualificationAttempts || report.UsefulSubstantiveAttempts < minimumUsefulSubstantiveAttempts || report.OmittedControls != qualificationControlAttemptCount || report.CriticalFalseClaims != 0 || report.InvalidProviderMetadata != 0 {
		report.Outcome = EngineeringInsightFailedOutcome
		return report, nil
	}
	report.Outcome = EngineeringInsightPassedOutcome
	return report, nil
}

func validateEvaluationExpectation(expected EngineeringInsightEvaluationExpectation) error {
	if !completeExpectationIdentity(expected) {
		return fmt.Errorf("evaluation expectation is incomplete")
	}
	if expected.ProtocolVersion == "v2" {
		return validateV2EvaluationExpectation(expected)
	}
	if expected.ProtocolVersion != "" {
		return fmt.Errorf("evaluation expectation has an invalid protocol version")
	}
	if hasQualificationLimits(expected) {
		return validateQualificationSchedule(expected.Schedule)
	}
	if expected.MaxRequests <= 0 || expected.MaxRequests > 6 || expected.MaxOutputTokens != qualificationOutputTokenCap || expected.AttemptTimeoutSeconds != qualificationAttemptTimeoutSeconds || len(expected.Schedule) == 0 || len(expected.Schedule) > expected.MaxRequests {
		return fmt.Errorf("evaluation expectation has invalid limits")
	}
	for _, attempt := range expected.Schedule {
		if !validExpectedAttempt(attempt) {
			return fmt.Errorf("evaluation expectation has an invalid schedule")
		}
	}
	return nil
}

func validateV2EvaluationExpectation(expected EngineeringInsightEvaluationExpectation) error {
	if expected.CorpusID != recoveryV2CorpusID || expected.CandidateID != recoveryV2CandidateID || expected.Provider != recoveryV2Provider || expected.Model != recoveryV2Model || expected.PromptVersion != recoveryV2PromptVersion || !validRecoveryV2BaseRevision(expected.BaseRevision) || expected.MaxOutputTokens != qualificationOutputTokenCap || expected.AttemptTimeoutSeconds != qualificationAttemptTimeoutSeconds {
		return fmt.Errorf("evaluation expectation has an invalid v2 identity")
	}
	if expected.MaxRequests == recoveryV2DevelopmentRequests && expected.CorpusDigest == recoveryV2DevelopmentDigest {
		return validateV2Schedule(expected.Schedule, "development", 12, 8, 4)
	}
	if expected.MaxRequests == recoveryV2QualificationRequests && expected.CorpusDigest == recoveryV2QualificationDigest {
		return validateV2Schedule(expected.Schedule, "qualification", 24, 16, 8)
	}
	return fmt.Errorf("evaluation expectation has an invalid v2 schedule")
}

func validateV2Schedule(schedule []EngineeringInsightExpectedAttempt, partition string, count, substantive, controls int) error {
	if len(schedule) != count {
		return fmt.Errorf("evaluation expectation has an invalid schedule")
	}
	seen := make(map[string]bool, count)
	actualSubstantive, actualControls := 0, 0
	for _, attempt := range schedule {
		if strings.TrimSpace(attempt.CaseName) == "" || seen[attempt.CaseName] || attempt.Partition != partition || attempt.Repetition != 1 || attempt.Attempt != 1 {
			return fmt.Errorf("evaluation expectation has an invalid schedule")
		}
		seen[attempt.CaseName] = true
		if attempt.Intent == "substantive" {
			actualSubstantive++
		} else if attempt.Intent == "control" {
			actualControls++
		} else {
			return fmt.Errorf("evaluation expectation has an invalid schedule")
		}
	}
	if actualSubstantive != substantive || actualControls != controls {
		return fmt.Errorf("evaluation expectation has an invalid schedule")
	}
	return nil
}

func completeExpectationIdentity(expected EngineeringInsightEvaluationExpectation) bool {
	for _, value := range []string{expected.CandidateID, expected.Provider, expected.Model, expected.PromptVersion, expected.CorpusID, expected.CorpusDigest, expected.BaseRevision} {
		if strings.TrimSpace(value) == "" {
			return false
		}
	}
	return true
}

func hasQualificationLimits(expected EngineeringInsightEvaluationExpectation) bool {
	return expected.MaxRequests == qualificationRequestCap && expected.MaxOutputTokens == qualificationOutputTokenCap && expected.AttemptTimeoutSeconds == qualificationAttemptTimeoutSeconds && len(expected.Schedule) == qualificationRequestCap
}

type expectedCaseSchedule struct {
	intent      string
	repetitions map[int]bool
}

func validateQualificationSchedule(schedule []EngineeringInsightExpectedAttempt) error {
	cases := make(map[string]expectedCaseSchedule, 12)
	for _, scheduled := range schedule {
		if err := recordExpectedAttempt(cases, scheduled); err != nil {
			return err
		}
	}
	if len(cases) != 12 || scheduleIntentCount(cases, "substantive") != 8 || scheduleIntentCount(cases, "control") != 4 {
		return fmt.Errorf("evaluation expectation has an invalid schedule")
	}
	for _, scheduled := range cases {
		if len(scheduled.repetitions) != 2 || !scheduled.repetitions[1] || !scheduled.repetitions[2] {
			return fmt.Errorf("evaluation expectation has an invalid schedule")
		}
	}
	return nil
}

func recordExpectedAttempt(cases map[string]expectedCaseSchedule, attempt EngineeringInsightExpectedAttempt) error {
	if !validQualificationExpectedAttempt(attempt) {
		return fmt.Errorf("evaluation expectation has an invalid schedule")
	}
	scheduled, exists := cases[attempt.CaseName]
	if !exists {
		scheduled = expectedCaseSchedule{intent: attempt.Intent, repetitions: make(map[int]bool, 2)}
	}
	if scheduled.intent != attempt.Intent || scheduled.repetitions[attempt.Repetition] {
		return fmt.Errorf("evaluation expectation has an invalid schedule")
	}
	scheduled.repetitions[attempt.Repetition] = true
	cases[attempt.CaseName] = scheduled
	return nil
}

func validQualificationExpectedAttempt(attempt EngineeringInsightExpectedAttempt) bool {
	return strings.TrimSpace(attempt.CaseName) != "" && attempt.Partition == "qualification" && (attempt.Intent == "substantive" || attempt.Intent == "control") && (attempt.Repetition == 1 || attempt.Repetition == 2) && attempt.Attempt == 1
}

func scheduleIntentCount(cases map[string]expectedCaseSchedule, intent string) int {
	count := 0
	for _, scheduled := range cases {
		if scheduled.intent == intent {
			count++
		}
	}
	return count
}

func validateReceiptIdentity(receipt EngineeringInsightEvaluationReceipt, expected EngineeringInsightEvaluationExpectation) error {
	if receipt.Mode != EngineeringInsightCollectionMode && receipt.Mode != EngineeringInsightQualificationMode {
		return fmt.Errorf("evaluation receipt has invalid mode")
	}
	if receipt.ProtocolVersion != expected.ProtocolVersion || strings.TrimSpace(receipt.RunID) == "" || receipt.CandidateID != expected.CandidateID || receipt.Provider != expected.Provider || receipt.Model != expected.Model || receipt.PromptVersion != expected.PromptVersion || receipt.CorpusID != expected.CorpusID || receipt.CorpusDigest != expected.CorpusDigest || receipt.BaseRevision != expected.BaseRevision {
		return fmt.Errorf("evaluation receipt identity does not match the selected candidate")
	}
	if receipt.MaxRequests != expected.MaxRequests || receipt.MaxOutputTokens != expected.MaxOutputTokens || receipt.AttemptTimeoutSeconds != expected.AttemptTimeoutSeconds {
		return fmt.Errorf("evaluation receipt limits do not match the selected limits")
	}
	return nil
}

func validateReceiptConsumption(receipt EngineeringInsightEvaluationReceipt) error {
	if receipt.Consumption.Requests != len(receipt.Attempts) || receipt.Consumption.Requests < 0 || receipt.Consumption.Requests > receipt.MaxRequests {
		return fmt.Errorf("evaluation receipt consumption is invalid")
	}
	totalTokens := 0
	for _, attempt := range receipt.Attempts {
		if !attempt.InvalidProviderMetadata && (attempt.OutputTokens < 0 || attempt.OutputTokens > receipt.MaxOutputTokens) || attempt.ElapsedMilliseconds > float64(receipt.AttemptTimeoutSeconds)*1000 && !censoredAttempt(attempt) {
			return fmt.Errorf("evaluation attempt consumption is invalid")
		}
		totalTokens += attempt.OutputTokens
	}
	if totalTokens != receipt.Consumption.OutputTokens {
		return fmt.Errorf("evaluation receipt consumption is invalid")
	}
	return nil
}

func censoredAttempt(attempt EngineeringInsightEvaluationAttempt) bool {
	return attempt.Outcome == "timeout" || attempt.Outcome == "failed" && attempt.FinishReason == "canceled" || attempt.Outcome == "unknown"
}

func validateAttempts(receipt EngineeringInsightEvaluationReceipt, expected EngineeringInsightEvaluationExpectation) (map[string]EngineeringInsightEvaluationAttempt, error) {
	expectedByID := make(map[string]EngineeringInsightExpectedAttempt, len(expected.Schedule))
	for _, scheduled := range expected.Schedule {
		id := expectedAttemptID(scheduled)
		if _, duplicate := expectedByID[id]; duplicate || !validExpectedAttempt(scheduled) {
			return nil, fmt.Errorf("evaluation expectation has an invalid schedule")
		}
		expectedByID[id] = scheduled
	}
	recorded := make(map[string]EngineeringInsightEvaluationAttempt, len(receipt.Attempts))
	if len(receipt.Attempts) > len(expected.Schedule) {
		return nil, fmt.Errorf("evaluation receipt contains evidence outside the selected schedule")
	}
	for index, attempt := range receipt.Attempts {
		id := expectedAttemptID(EngineeringInsightExpectedAttempt{CaseName: attempt.CaseName, Partition: attempt.Partition, Intent: attempt.Intent, Repetition: attempt.Repetition, Attempt: attempt.Attempt})
		if _, known := expectedByID[id]; !known || id != expectedAttemptID(expected.Schedule[index]) || attempt.CandidateID != expected.CandidateID {
			return nil, fmt.Errorf("evaluation receipt contains evidence outside the selected schedule")
		}
		if _, duplicate := recorded[id]; duplicate {
			return nil, fmt.Errorf("evaluation receipt contains duplicate attempt evidence")
		}
		if err := validateAttempt(attempt, receipt.Mode); err != nil {
			return nil, err
		}
		recorded[id] = attempt
	}
	return recorded, nil
}

func validExpectedAttempt(attempt EngineeringInsightExpectedAttempt) bool {
	return strings.TrimSpace(attempt.CaseName) != "" && (attempt.Partition == "development" || attempt.Partition == "qualification") && (attempt.Intent == "substantive" || attempt.Intent == "control") && attempt.Repetition > 0 && attempt.Attempt > 0
}

func expectedAttemptID(attempt EngineeringInsightExpectedAttempt) string {
	return fmt.Sprintf("%s\x00%s\x00%s\x00%d\x00%d", attempt.CaseName, attempt.Partition, attempt.Intent, attempt.Repetition, attempt.Attempt)
}

func validateAttempt(attempt EngineeringInsightEvaluationAttempt, mode EngineeringInsightEvaluationMode) error {
	if !validAttemptValues(attempt) {
		return fmt.Errorf("evaluation attempt has invalid evidence")
	}
	if !validOutcomeFinishReason(attempt.Outcome, attempt.FinishReason) {
		return fmt.Errorf("evaluation attempt has inconsistent outcome evidence")
	}
	if err := validateAttemptSummary(attempt); err != nil {
		return err
	}
	return validateAttemptScore(attempt, mode)
}

func validAttemptValues(attempt EngineeringInsightEvaluationAttempt) bool {
	return validOutcome(attempt.Outcome) && validOptionalInsight(attempt.OptionalInsight) && validFinishReason(attempt.FinishReason) && finiteNonNegative(attempt.ElapsedMilliseconds)
}

func validOutcomeFinishReason(outcome, finishReason string) bool {
	switch outcome {
	case "completed", "malformed", "abstained":
		return finishReason == "stop"
	case "truncated":
		return finishReason == "length"
	case "timeout":
		return finishReason == "timeout"
	case "failed":
		return finishReason == "error" || finishReason == "canceled"
	case "budget_exhausted":
		return finishReason == "unknown"
	case "unknown":
		return finishReason == "unknown"
	default:
		return false
	}
}

func validateAttemptSummary(attempt EngineeringInsightEvaluationAttempt) error {
	if attempt.Outcome != "completed" {
		return validateNonCompletedSummary(attempt)
	}
	return validateCompletedSummary(attempt)
}

func validateNonCompletedSummary(attempt EngineeringInsightEvaluationAttempt) error {
	if hasSummaryEvidence(attempt) {
		return fmt.Errorf("evaluation attempt has inconsistent summary evidence")
	}
	return nil
}

func validateCompletedSummary(attempt EngineeringInsightEvaluationAttempt) error {
	if !attempt.EmittedResponse {
		return fmt.Errorf("evaluation attempt has inconsistent summary evidence")
	}
	if attempt.UsableSummary {
		return validateUsableSummary(attempt)
	}
	if hasSummaryEvidence(attempt) {
		return fmt.Errorf("evaluation attempt has inconsistent summary evidence")
	}
	return nil
}

func validateUsableSummary(attempt EngineeringInsightEvaluationAttempt) error {
	if attempt.OptionalInsight == "not_evaluated" {
		return fmt.Errorf("evaluation attempt has inconsistent summary evidence")
	}
	if !attempt.CompleteSummary {
		return nil
	}
	if attempt.OptionalInsight == "rejected" || attempt.OptionalSectionDegraded {
		return fmt.Errorf("evaluation attempt has inconsistent summary evidence")
	}
	return nil
}

func hasSummaryEvidence(attempt EngineeringInsightEvaluationAttempt) bool {
	return attempt.UsableSummary || attempt.CompleteSummary || attempt.OptionalInsight != "not_evaluated" || attempt.OptionalSectionDegraded
}

func validateAttemptScore(attempt EngineeringInsightEvaluationAttempt, mode EngineeringInsightEvaluationMode) error {
	if err := validateResponseDigest(attempt); err != nil {
		return err
	}
	if attempt.Score == nil {
		if mode == EngineeringInsightQualificationMode && attempt.EmittedResponse {
			return fmt.Errorf("qualification receipt has unscored response evidence")
		}
		return nil
	}
	if !attempt.EmittedResponse || attempt.Score.ResponseDigest != attempt.ResponseDigest || !validDigest(attempt.Score.ResponseDigest) || !validScore(*attempt.Score) {
		return fmt.Errorf("evaluation attempt has invalid score evidence")
	}
	if attempt.Score.RetainExample && (attempt.Score.CriticalFalseClaim || attempt.Score.Total() < minimumRetainedEngineeringInsightScore) {
		return fmt.Errorf("evaluation attempt retains an ineligible example")
	}
	return nil
}

func validateResponseDigest(attempt EngineeringInsightEvaluationAttempt) error {
	if attempt.EmittedResponse == (attempt.ResponseDigest == "") {
		return fmt.Errorf("evaluation attempt has invalid response evidence")
	}
	if attempt.EmittedResponse && !validDigest(attempt.ResponseDigest) {
		return fmt.Errorf("evaluation attempt has invalid response evidence")
	}
	return nil
}

func validOutcome(value string) bool {
	return value == "completed" || value == "timeout" || value == "malformed" || value == "truncated" || value == "abstained" || value == "failed" || value == "budget_exhausted" || value == "unknown"
}
func validOptionalInsight(value string) bool {
	return value == "present" || value == "omitted" || value == "rejected" || value == "not_evaluated"
}
func validFinishReason(value string) bool {
	return value == "stop" || value == "length" || value == "error" || value == "timeout" || value == "canceled" || value == "unknown"
}
func validScore(score EngineeringInsightAttemptScore) bool {
	for _, value := range []int{score.Correctness, score.LocalRelevance, score.TradeoffClarity, score.UsefulVerification} {
		if value < 0 || value > 2 {
			return false
		}
	}
	return true
}

var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func validDigest(value string) bool { return digestPattern.MatchString(value) }
func finiteNonNegative(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0
}

func evaluationReport(expected EngineeringInsightEvaluationExpectation, recorded map[string]EngineeringInsightEvaluationAttempt) EngineeringInsightEvaluationReport {
	report := newEvaluationReport(expected, recorded)
	var successful []float64
	for _, scheduled := range expected.Schedule {
		attempt, present := recorded[expectedAttemptID(scheduled)]
		if present {
			successful = recordEvaluationAttempt(&report, scheduled, attempt, successful)
		}
	}
	report.Latency.SuccessfulCount = len(successful)
	report.Latency.MedianMilliseconds = median(successful)
	report.Latency.P95Milliseconds = nearestRank(successful, 0.95)
	return report
}

func newEvaluationReport(expected EngineeringInsightEvaluationExpectation, recorded map[string]EngineeringInsightEvaluationAttempt) EngineeringInsightEvaluationReport {
	return EngineeringInsightEvaluationReport{ScheduledAttempts: len(expected.Schedule), RecordedAttempts: len(recorded), OutcomeCounts: map[string]int{"completed": 0, "timeout": 0, "malformed": 0, "truncated": 0, "abstained": 0, "failed": 0, "budget_exhausted": 0, "unknown": 0}}
}

func recordEvaluationAttempt(report *EngineeringInsightEvaluationReport, scheduled EngineeringInsightExpectedAttempt, attempt EngineeringInsightEvaluationAttempt, successful []float64) []float64 {
	report.OutcomeCounts[attempt.Outcome]++
	if attempt.InvalidProviderMetadata {
		report.InvalidProviderMetadata++
	}
	recordSummaryCounts(report, attempt)
	recordScoreCounts(report, scheduled, attempt)
	if attempt.Outcome == "completed" && attempt.UsableSummary {
		return append(successful, attempt.ElapsedMilliseconds)
	}
	report.Latency.TimeoutOrCensoredAttempts++
	return successful
}

func recordSummaryCounts(report *EngineeringInsightEvaluationReport, attempt EngineeringInsightEvaluationAttempt) {
	if attempt.UsableSummary {
		report.UsableAttempts++
	}
	if attempt.CompleteSummary {
		report.CompleteAttempts++
	}
}

func recordScoreCounts(report *EngineeringInsightEvaluationReport, scheduled EngineeringInsightExpectedAttempt, attempt EngineeringInsightEvaluationAttempt) {
	if attempt.Score != nil && attempt.Score.CriticalFalseClaim {
		report.CriticalFalseClaims++
	}
	if scheduled.Intent == "substantive" && hasUsefulSubstantiveInsight(attempt) {
		report.UsefulSubstantiveAttempts++
	}
	if scheduled.Intent == "control" && attempt.UsableSummary && attempt.OptionalInsight == "omitted" {
		report.OmittedControls++
	}
}

func hasUsefulSubstantiveInsight(attempt EngineeringInsightEvaluationAttempt) bool {
	return attempt.UsableSummary && attempt.OptionalInsight == "present" && attempt.Score != nil && attempt.Score.Total() >= minimumRetainedEngineeringInsightScore
}

func nearestRank(values []float64, percentile float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	return sorted[int(math.Ceil(percentile*float64(len(sorted))))-1]
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	middle := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[middle-1] + sorted[middle]) / 2
	}
	return sorted[middle]
}

func rejectDuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := consumeJSONValue(decoder); err != nil {
		return err
	}
	return requireJSONEOF(decoder)
}

func consumeJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, compound := token.(json.Delim)
	if !compound {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]struct{}{}
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok {
				return fmt.Errorf("invalid object")
			}
			if _, duplicate := seen[name]; duplicate {
				return fmt.Errorf("duplicate key")
			}
			seen[name] = struct{}{}
			if err := consumeJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	case '[':
		for decoder.More() {
			if err := consumeJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	default:
		return fmt.Errorf("invalid delimiter")
	}
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("trailing data")
	}
	return nil
}
