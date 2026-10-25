package insighteval

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

	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

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
		manifest.Receipt.Attempts = append(manifest.Receipt.Attempts, buildUnknownReservedAttempt(item.Expected, cfg.CandidateID))
		manifest.Receipt.Consumption.Requests++
		if err := storeRunnerJSON(manifestPath, manifest); err != nil {
			return err
		}
		if !dispatch {
			continue
		}

		attempt, response, diagnostics, err := executeRunnerAttempt(ctx, cfg, item)
		permanentRejection := errors.Is(err, llm.ErrStructuredRequestRejected)
		if err != nil && !permanentRejection {
			attempt = buildFailedRunnerAttempt(item.Expected, cfg.CandidateID, err)
		}
		if response != "" {
			// Publish the source-free diagnostic before the raw private response.
			// A failure between either immutable write and the receipt update leaves
			// the reserved attempt unknown, and resume intentionally never replays it.
			if err := storePrivateOptionalDiagnostics(directory, engineeringInsightOptionalDiagnostics{RunID: cfg.RunID, AttemptID: buildExpectedAttemptID(item.Expected), ResponseDigest: attempt.ResponseDigest, Insights: diagnostics.Insights, TaskSpecDegradedRiskIndices: diagnostics.TaskSpecDegradedRiskIndices}); err != nil {
				return err
			}
			if err := storePrivateResponse(handoff.privateDir, buildExpectedAttemptID(item.Expected), response); err != nil {
				return err
			}
		}
		manifest.Receipt.Attempts[index] = attempt
		manifest.Receipt.Consumption.OutputTokens += attempt.OutputTokens
		if permanentRejection {
			manifest.Finished = true
			manifest.TerminalReason = permanentRequestRejectionReason
		}
		if err := storeRunnerJSON(manifestPath, manifest); err != nil {
			return err
		}
		if response != "" {
			handoff.Responses[buildExpectedAttemptID(item.Expected)] = response
		}
		if permanentRejection {
			return ErrEngineeringInsightRunTerminated
		}
	}
	return nil
}

func hasRecoveryV2IdentityMarkers(cfg EngineeringInsightRunnerConfig) bool {
	return cfg.CandidateID == recoveryV2CandidateID || cfg.Model == recoveryV2Model || cfg.Profile.Model == recoveryV2Model || cfg.CorpusID == recoveryV2CorpusID || cfg.RunID == recoveryV2DevelopmentRunID || cfg.RunID == recoveryV2QualificationRunID
}

func isRecoveryV2RunnerConfig(cfg EngineeringInsightRunnerConfig) bool {
	if !hasSameRecoveryV2RunnerIdentity(cfg) || !hasSameRecoveryV2RunnerProfile(cfg) || !isValidRecoveryV2Corpus(cfg) {
		return false
	}
	if cfg.Mode == EngineeringInsightDevelopmentRunMode {
		return cfg.RunID == recoveryV2DevelopmentRunID && cfg.CorpusDigest == recoveryV2DevelopmentDigest
	}
	return cfg.Mode == EngineeringInsightQualificationRunMode && cfg.RunID == recoveryV2QualificationRunID && cfg.CorpusDigest == recoveryV2QualificationDigest
}

func isValidRecoveryV2Corpus(cfg EngineeringInsightRunnerConfig) bool {
	if !hasMatchingRecoveryV2CorpusDigest(cfg) || ValidateStrictJSONDocument(cfg.CorpusJSON) != nil {
		return false
	}
	corpus, ok := decodeRecoveryV2Corpus(cfg.CorpusJSON)
	return ok && hasMatchingRecoveryV2CorpusCases(corpus, cfg.Cases)
}

func hasMatchingRecoveryV2CorpusDigest(cfg EngineeringInsightRunnerConfig) bool {
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

func hasMatchingRecoveryV2CorpusCases(corpus []recoveryV2CorpusCase, cases []EngineeringInsightRunnerCase) bool {
	selected := make(map[string]struct{ partition, intent, source string })
	for _, item := range corpus {
		if !isValidRecoveryV2CorpusCase(item) {
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
		if !exists || !isMatchingRecoveryV2SelectedCase(caseSpec, item) {
			return false
		}
	}
	return true
}

func isValidRecoveryV2CorpusCase(item recoveryV2CorpusCase) bool {
	return item.Name != "" && !strings.ContainsRune(item.Name, '\x00') && item.Source != "" && (item.Partition == "development" || item.Partition == "qualification") && (item.Intent == "substantive" || item.Intent == "control")
}

func isMatchingRecoveryV2SelectedCase(spec struct{ partition, intent, source string }, item EngineeringInsightRunnerCase) bool {
	return spec.partition == item.Expected.Partition && spec.intent == item.Expected.Intent && spec.source == item.Source && item.Expected.Repetition == 1 && item.Expected.Attempt == 1
}

func hasSameRecoveryV2RunnerIdentity(cfg EngineeringInsightRunnerConfig) bool {
	return cfg.CandidateID == recoveryV2CandidateID && cfg.Provider == recoveryV2Provider && cfg.Model == recoveryV2Model && cfg.Profile.Model == recoveryV2Model && cfg.PromptVersion == recoveryV2PromptVersion && cfg.CorpusID == recoveryV2CorpusID && isValidRecoveryV2BaseRevision(cfg.BaseRevision)
}

func hasSameRecoveryV2RunnerProfile(cfg EngineeringInsightRunnerConfig) bool {
	return cfg.Profile.APIBaseURL == recoveryV2Endpoint && cfg.Profile.ReasoningEffort == mediumDevelopmentReasoningEffort && cfg.Profile.MaxTokens == qualificationOutputTokenCap && cfg.Profile.ContextMaxTokens == 16384 && cfg.Profile.Temperature == 1 && cfg.Profile.TopP != nil && *cfg.Profile.TopP == .95 && cfg.Profile.TopK != nil && *cfg.Profile.TopK == 20 && cfg.Profile.MinP == nil && cfg.Profile.PresencePenalty == nil && cfg.Profile.RepeatPenalty == nil
}

func isValidRecoveryV2DevelopmentSchedule(cases []EngineeringInsightRunnerCase) bool {
	return isValidRecoveryV2Schedule(cases, "development", recoveryV2DevelopmentRequests, 8, 4)
}

func isValidRecoveryV2QualificationSchedule(cases []EngineeringInsightRunnerCase) bool {
	return isValidRecoveryV2Schedule(cases, "qualification", recoveryV2QualificationRequests, 16, 8)
}

func isValidRecoveryV2Schedule(cases []EngineeringInsightRunnerCase, partition string, count, substantive, controls int) bool {
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

func reserveRunnerAttempt(directory string, cfg EngineeringInsightRunnerConfig, expected EngineeringInsightExpectedAttempt) (bool, error) {
	if isRecoveryV2RunnerConfig(cfg) {
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
		cap, err := loadDevelopmentRequestCap(directory, campaign.DevelopmentRequests, cfg)
		if err != nil {
			return false, err
		}
		if campaign.DevelopmentRequests >= cap {
			return false, fmt.Errorf("development request budget is exhausted")
		}
		campaign.DevelopmentRequests++
	}
	return true, storeRunnerJSON(path, campaign)
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
	if err := storeRunnerJSON(buildRecoveryV2PeriodPath(directory), period); err != nil {
		return false, err
	}
	return true, nil
}

func validateRecoveryV2ReservationBoundary(directory string, cfg EngineeringInsightRunnerConfig, period engineeringInsightRecoveryV2Period) error {
	actualBase, err := loadCleanCanonicalRecoveryBase(period.CanonicalRoot)
	if err != nil || actualBase != period.BaseRevision || cfg.BaseRevision != period.BaseRevision {
		return fmt.Errorf("recovery evaluation base changed after authorization")
	}
	if err := validateRecoveryV2Predecessors(period.CanonicalRoot); err != nil {
		return err
	}
	if cfg.Mode == EngineeringInsightQualificationRunMode && !isRecoveryV2DevelopmentAccepted(directory, period) {
		return fmt.Errorf("recovery qualification requires accepted development evidence")
	}
	return nil
}

func bindRecoveryV2Schedule(period *engineeringInsightRecoveryV2Period, cfg EngineeringInsightRunnerConfig, expected EngineeringInsightExpectedAttempt) (string, error) {
	schedule := buildRecoveryV2ScheduleIDs(cfg.Cases)
	boundSchedule := &period.DevelopmentSchedule
	if cfg.Mode == EngineeringInsightQualificationRunMode {
		boundSchedule = &period.QualificationSchedule
	}
	if len(*boundSchedule) == 0 {
		*boundSchedule = schedule
	} else if !isSameStringSchedule(*boundSchedule, schedule) {
		return "", fmt.Errorf("recovery evaluation schedule changed after reservation")
	}
	id := buildExpectedAttemptID(expected)
	if !hasScheduledAttemptID(*boundSchedule, id) {
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
	data, err := os.ReadFile(buildRecoveryV2PeriodPath(directory))
	if err != nil {
		return engineeringInsightRecoveryV2Period{}, fmt.Errorf("recovery successor period is not authorized")
	}
	if ValidateStrictJSONDocument(data) != nil {
		return engineeringInsightRecoveryV2Period{}, fmt.Errorf("recovery successor period is invalid")
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil || !hasValidRecoveryV2PeriodFields(fields) {
		return engineeringInsightRecoveryV2Period{}, fmt.Errorf("recovery successor period is invalid")
	}
	var period engineeringInsightRecoveryV2Period
	if json.Unmarshal(data, &period) != nil || !isValidRecoveryV2Period(period, root) {
		return engineeringInsightRecoveryV2Period{}, fmt.Errorf("recovery successor period is invalid")
	}
	return period, nil
}

func hasValidRecoveryV2PeriodFields(fields map[string]json.RawMessage) bool {
	want := []string{"version", "period_id", "canonical_root", "provider", "endpoint", "candidate_id", "model", "prompt_version", "corpus_id", "development_corpus_digest", "qualification_corpus_digest", "base_revision", "schema_identity", "max_output_tokens", "input_context_tokens", "runtime_context_tokens", "attempt_timeout_seconds", "reasoning_effort", "temperature", "top_p", "top_k", "lanes", "development_requests", "qualification_requests", "cumulative_development_requests", "cumulative_qualification_requests", "development_limit", "qualification_limit", "predecessor_files_sha256", "development_schedule", "qualification_schedule", "reservations"}
	return isValidRequiredJSONObject(fields, want)
}

func isValidRecoveryV2Period(period engineeringInsightRecoveryV2Period, root string) bool {
	canonicalRoot, err := loadCanonicalEvaluationRoot(root)
	schemaIdentity, schemaErr := app.FileAnalysisResponseSchema().Identity()
	if err != nil || schemaErr != nil || !hasSameRecoveryV2PeriodIdentity(period, canonicalRoot) || !hasSameRecoveryV2PeriodProfile(period, schemaIdentity) || !isValidRecoveryV2PeriodAccounting(period) {
		return false
	}
	if !hasSameRecoveryV2Predecessors(period.PredecessorFiles) || !hasValidRecoveryV2PeriodSchedules(period) {
		return false
	}
	developmentReservations, qualificationReservations, valid := buildRecoveryV2ReservationCounts(period)
	return valid && developmentReservations == period.DevelopmentRequests && qualificationReservations == period.QualificationRequests
}

func hasSameRecoveryV2Predecessors(actual map[string]string) bool {
	for name, digest := range recoveryV2PredecessorFiles {
		if actual[name] != digest {
			return false
		}
	}
	return true
}

func hasValidRecoveryV2PeriodSchedules(period engineeringInsightRecoveryV2Period) bool {
	return isValidBoundRecoveryV2Schedule(period.DevelopmentSchedule, "development", recoveryV2DevelopmentRequests, 8, 4, period.DevelopmentRequests) && isValidBoundRecoveryV2Schedule(period.QualificationSchedule, "qualification", recoveryV2QualificationRequests, 16, 8, period.QualificationRequests)
}

func buildRecoveryV2ReservationCounts(period engineeringInsightRecoveryV2Period) (int, int, bool) {
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

func isValidBoundRecoveryV2Schedule(schedule []string, partition string, count, substantive, controls, consumed int) bool {
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

func buildRecoveryV2ScheduleIDs(cases []EngineeringInsightRunnerCase) []string {
	result := make([]string, len(cases))
	for index, item := range cases {
		result[index] = buildExpectedAttemptID(item.Expected)
	}
	return result
}

func isSameStringSchedule(left, right []string) bool {
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

func hasScheduledAttemptID(schedule []string, id string) bool {
	for _, scheduled := range schedule {
		if scheduled == id {
			return true
		}
	}
	return false
}

func hasSameRecoveryV2PeriodIdentity(period engineeringInsightRecoveryV2Period, root string) bool {
	return period.Version == "v2" && period.PeriodID == recoveryV2PeriodID && period.CanonicalRoot == root && period.Provider == recoveryV2Provider && period.Endpoint == recoveryV2Endpoint && period.CandidateID == recoveryV2CandidateID && period.Model == recoveryV2Model && period.PromptVersion == recoveryV2PromptVersion && period.CorpusID == recoveryV2CorpusID && period.DevelopmentCorpusDigest == recoveryV2DevelopmentDigest && period.QualificationCorpusDigest == recoveryV2QualificationDigest && isValidRecoveryV2BaseRevision(period.BaseRevision)
}

func hasSameRecoveryV2PeriodProfile(period engineeringInsightRecoveryV2Period, schemaIdentity string) bool {
	return period.SchemaIdentity == schemaIdentity && period.MaxOutputTokens == qualificationOutputTokenCap && period.InputContextTokens == 16384 && period.RuntimeContextTokens == recoveryV2RuntimeContextTokens && period.AttemptTimeoutSeconds == qualificationAttemptTimeoutSeconds && period.ReasoningEffort == mediumDevelopmentReasoningEffort && period.Temperature == 1 && period.TopP == .95 && period.TopK == 20 && period.Lanes == 1
}

func isValidRecoveryV2PeriodAccounting(period engineeringInsightRecoveryV2Period) bool {
	return period.DevelopmentLimit == recoveryV2DevelopmentLimit && period.QualificationLimit == recoveryV2QualificationLimit && period.DevelopmentRequests >= 0 && period.DevelopmentRequests <= recoveryV2DevelopmentRequests && period.QualificationRequests >= 0 && period.QualificationRequests <= recoveryV2QualificationRequests && period.CumulativeDevelopmentRequests == 48+period.DevelopmentRequests && period.CumulativeQualificationRequests == 24+period.QualificationRequests && len(period.PredecessorFiles) == len(recoveryV2PredecessorFiles)
}

func isRecoveryV2DevelopmentAccepted(directory string, period engineeringInsightRecoveryV2Period) bool {
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
	return storeRunnerJSON(path, engineeringInsightCampaign{})
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

func buildUnknownReservedAttempt(expected EngineeringInsightExpectedAttempt, candidate string) EngineeringInsightEvaluationAttempt {
	return EngineeringInsightEvaluationAttempt{CaseName: expected.CaseName, Partition: expected.Partition, Intent: expected.Intent, Repetition: expected.Repetition, Attempt: expected.Attempt, CandidateID: candidate, Outcome: "unknown", OptionalInsight: "not_evaluated", FinishReason: "unknown"}
}

func executeRunnerAttempt(parent context.Context, cfg EngineeringInsightRunnerConfig, item EngineeringInsightRunnerCase) (EngineeringInsightEvaluationAttempt, string, app.FileAnalysisDiagnostics, error) {
	root, prompt, target, err := buildRunnerPrompt(cfg, item)
	if err != nil {
		return EngineeringInsightEvaluationAttempt{}, "", app.FileAnalysisDiagnostics{}, err
	}
	defer os.RemoveAll(root)
	return dispatchRunnerPrompt(parent, cfg, item, prompt, target)
}

func buildRunnerPrompt(cfg EngineeringInsightRunnerConfig, item EngineeringInsightRunnerCase) (string, string, *project.IndexFile, error) {
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
	if err := validateRunnerFixtureCompilation(root); err != nil {
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
	prompt, err := app.PrepareFileAnalysisEvaluation(item.Source, analysis, index, *target, cfg.Profile)
	if err != nil {
		return fail(err)
	}
	return root, prompt, target, nil
}

func dispatchRunnerPrompt(parent context.Context, cfg EngineeringInsightRunnerConfig, item EngineeringInsightRunnerCase, prompt string, target *project.IndexFile) (EngineeringInsightEvaluationAttempt, string, app.FileAnalysisDiagnostics, error) {
	timed, cancel := context.WithTimeout(parent, qualificationAttemptTimeoutSeconds*time.Second)
	defer cancel()
	started := time.Now()
	response, err := cfg.Client.ChatWithJSONSchema(timed, []llm.ChatMessage{{Role: "user", Content: prompt}}, app.FileAnalysisResponseSchema())
	elapsed := time.Since(started).Milliseconds()
	if err != nil {
		attempt := buildFailedRunnerAttemptWithElapsed(item.Expected, cfg.CandidateID, err, elapsed)
		if errors.Is(err, llm.ErrStructuredRequestRejected) {
			return attempt, "", app.FileAnalysisDiagnostics{}, err
		}
		return attempt, "", app.FileAnalysisDiagnostics{}, nil
	}
	message := response.Choices[0].Message
	content := message.Content
	emitted := buildEvaluationResponseMaterial(message)
	attempt := EngineeringInsightEvaluationAttempt{CaseName: item.Expected.CaseName, Partition: item.Expected.Partition, Intent: item.Expected.Intent, Repetition: item.Expected.Repetition, Attempt: item.Expected.Attempt, CandidateID: cfg.CandidateID, OutputTokens: response.Usage.CompletionTokens, ElapsedMilliseconds: float64(elapsed), OptionalInsight: "not_evaluated"}
	if emitted != "" {
		digest := sha256.Sum256([]byte(emitted))
		attempt.EmittedResponse = true
		attempt.ResponseDigest = hex.EncodeToString(digest[:])
	}
	assessment := app.AssessFileAnalysisEvaluation(content, *target, item.Source)
	if attempt.OutputTokens < 0 || attempt.OutputTokens > qualificationOutputTokenCap || response.Model != "" && response.Model != cfg.Model {
		attempt.Outcome, attempt.FinishReason = "failed", "error"
		attempt.InvalidProviderMetadata = true
		return attempt, emitted, assessment.Diagnostics, nil
	}
	if response.Choices[0].FinishReason == "length" {
		attempt.Outcome, attempt.FinishReason = "truncated", "length"
		return attempt, emitted, assessment.Diagnostics, nil
	}
	if response.Choices[0].FinishReason != "" && response.Choices[0].FinishReason != "stop" {
		attempt.Outcome, attempt.FinishReason = "failed", "error"
		return attempt, emitted, assessment.Diagnostics, nil
	}
	if strings.TrimSpace(content) == "" {
		attempt.Outcome, attempt.FinishReason = "malformed", "stop"
		return attempt, emitted, app.FileAnalysisDiagnostics{}, nil
	}
	if !assessment.UsableSummary {
		attempt.Outcome, attempt.FinishReason = "malformed", "stop"
		return attempt, emitted, assessment.Diagnostics, nil
	}
	attempt.Outcome, attempt.FinishReason, attempt.UsableSummary = "completed", "stop", true
	attempt.OptionalInsight = assessment.Diagnostics.OptionalInsight()
	attempt.OptionalSectionDegraded = assessment.Diagnostics.Degraded()
	attempt.CompleteSummary = !attempt.OptionalSectionDegraded
	return attempt, emitted, assessment.Diagnostics, nil
}

// buildEvaluationResponseMaterial retains every non-empty provider response field
// for private scoring. The semantic parser receives only final content, so
// separately returned reasoning is never interpreted as a JSON prefix.
func buildEvaluationResponseMaterial(message llm.ChatMessage) string {
	parts := make([]string, 0, 3)
	for _, value := range []string{message.ReasoningContent, message.Reasoning, message.Content} {
		if strings.TrimSpace(value) != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, "\n\n")
}

func validateRunnerFixtureCompilation(root string) error {
	command := exec.Command("go", "test", "./...")
	command.Dir = root
	command.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off")
	if err := command.Run(); err != nil {
		return fmt.Errorf("compile private evaluation fixture")
	}
	return nil
}

func buildFailedRunnerAttempt(expected EngineeringInsightExpectedAttempt, candidate string, cause error) EngineeringInsightEvaluationAttempt {
	return buildFailedRunnerAttemptWithElapsed(expected, candidate, cause, 0)
}

func buildFailedRunnerAttemptWithElapsed(expected EngineeringInsightExpectedAttempt, candidate string, cause error, elapsed int64) EngineeringInsightEvaluationAttempt {
	attempt := buildUnknownReservedAttempt(expected, candidate)
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
