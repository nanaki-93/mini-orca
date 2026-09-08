package app

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

const (
	developmentPartition   = "development"
	qualificationPartition = "qualification"
	substantiveIntent      = "substantive"
	controlIntent          = "control"
)

type engineeringInsightFixture struct {
	Name                       string                       `json:"name"`
	Partition                  string                       `json:"partition"`
	Intent                     string                       `json:"intent"`
	ReferenceMechanisms        []string                     `json:"reference_mechanisms"`
	PermittedUncertainty       []string                     `json:"permitted_uncertainty"`
	CriticalFalseClaimExamples []string                     `json:"critical_false_claim_examples"`
	ScoringAnchors             map[string]map[string]string `json:"scoring_anchors"`
	Source                     string                       `json:"source"`
	Anchor                     struct {
		StartLine int    `json:"start_line"`
		EndLine   int    `json:"end_line"`
		Evidence  string `json:"evidence"`
	} `json:"anchor"`
	ExpectInsight   bool   `json:"expect_insight"`
	SanitizedOutput string `json:"sanitized_output"`
}

func TestEngineeringInsightEvaluationFixturesHavePartitionedGroundedRubric(t *testing.T) {
	fixtures := loadEngineeringInsightFixtures(t)
	seen := make(map[string]bool, len(fixtures))
	counts := make(map[string]int)
	qualificationIntents := make(map[string]int)
	developmentMechanisms := make(map[string]struct{})

	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			if fixture.Name == "" || seen[fixture.Name] {
				t.Fatalf("fixture name is empty or repeated: %q", fixture.Name)
			}
			seen[fixture.Name] = true
			assertFixturePartitionAndIntent(t, fixture)
			assertFixtureRubric(t, fixture)
			assertFixtureSourceAnchor(t, fixture)
			assertFixtureSemanticOutput(t, fixture)
		})
		counts[fixture.Partition]++
		if fixture.Partition == qualificationPartition {
			qualificationIntents[fixture.Intent]++
		} else {
			for _, mechanism := range fixture.ReferenceMechanisms {
				developmentMechanisms[mechanism] = struct{}{}
			}
		}
	}

	if counts[developmentPartition] != 3 || counts[qualificationPartition] != 12 {
		t.Fatalf("case partitions = %+v, want 3 development and 12 qualification", counts)
	}
	if qualificationIntents[substantiveIntent] != 8 || qualificationIntents[controlIntent] != 4 {
		t.Fatalf("qualification intents = %+v, want 8 substantive and 4 controls", qualificationIntents)
	}
	for _, fixture := range fixtures {
		if fixture.Partition != qualificationPartition {
			continue
		}
		for _, mechanism := range fixture.ReferenceMechanisms {
			if _, reused := developmentMechanisms[mechanism]; reused {
				t.Fatalf("qualification case %q reuses a development reference mechanism %q", fixture.Name, mechanism)
			}
		}
	}
}

func TestEngineeringInsightEvaluationFixturesCompileInOfflineTemporaryModules(t *testing.T) {
	for _, fixture := range loadEngineeringInsightFixtures(t) {
		t.Run(fixture.Name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n\ngo 1.22\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "fixture.go"), []byte(fixture.Source), 0600); err != nil {
				t.Fatal(err)
			}
			command := exec.Command("go", "test", "./...")
			command.Dir = root
			command.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off")
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("offline isolated fixture compile: %v\n%s", err, output)
			}
		})
	}
}

func TestEngineeringInsightEvaluationMalformedOutputIsSeparateFromControls(t *testing.T) {
	target := project.IndexFile{Path: "fixture.go", Language: "Go"}
	validParent := `{"purpose":"Summarizes the fixture.","responsibilities":[],"dependencies":[],"side_effects":[],"risks":[],"suggestions":[],"symbol_explanations":{}`
	for name, output := range map[string]string{
		"unknown parent field": validParent + `,"unexpected":true}`,
		"trailing JSON":        validParent + `}{}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseSemanticAnalysis(output, target, "package fixture\n"); err == nil {
				t.Fatal("strict parent validation accepted malformed output")
			}
		})
	}

	for name, insight := range map[string]string{
		"malformed optional insight": `{"mechanism":false}`,
		"oversized optional insight": fmt.Sprintf(`{"mechanism":%q,"why_it_matters_here":"local"}`, strings.Repeat("x", 1001)),
	} {
		t.Run(name, func(t *testing.T) {
			output := validParent + `,"engineering_insight":` + insight + `}`
			parsed, err := parseSemanticAnalysis(output, target, "package fixture\n")
			if err != nil {
				t.Fatalf("valid parent with optional malformed insight = %v", err)
			}
			if parsed.EngineeringInsight != nil {
				t.Fatal("invalid optional insight was retained")
			}
		})
	}
}

func TestEngineeringInsightQualificationThresholdBoundaries(t *testing.T) {
	expected := qualificationExpectation()
	for name, mutate := range map[string]struct {
		want  EngineeringInsightEvaluationOutcome
		apply func(*EngineeringInsightEvaluationReceipt)
	}{
		"thresholds pass": {want: EngineeringInsightPassedOutcome},
		"23 usable passes": {want: EngineeringInsightPassedOutcome, apply: func(receipt *EngineeringInsightEvaluationReceipt) {
			degradeAttempt(&receipt.Attempts[0], false)
		}},
		"22 complete passes": {want: EngineeringInsightPassedOutcome, apply: func(receipt *EngineeringInsightEvaluationReceipt) {
			degradeAttempt(&receipt.Attempts[0], true)
			degradeAttempt(&receipt.Attempts[1], true)
		}},
		"13 useful passes": {want: EngineeringInsightPassedOutcome, apply: func(receipt *EngineeringInsightEvaluationReceipt) {
			for index := 0; index < 3; index++ {
				receipt.Attempts[index].Score.Correctness = 0
				receipt.Attempts[index].Score.LocalRelevance = 1
			}
		}},
		"optional degradation retains useful and omitted metrics": {want: EngineeringInsightPassedOutcome, apply: func(receipt *EngineeringInsightEvaluationReceipt) {
			receipt.Attempts[0].CompleteSummary = false
			receipt.Attempts[0].OptionalSectionDegraded = true
			for index := range receipt.Attempts {
				if receipt.Attempts[index].Intent == controlIntent {
					receipt.Attempts[index].CompleteSummary = false
					receipt.Attempts[index].OptionalSectionDegraded = true
					return
				}
			}
		}},
		"22 usable fails": {want: EngineeringInsightFailedOutcome, apply: func(receipt *EngineeringInsightEvaluationReceipt) {
			degradeAttempt(&receipt.Attempts[0], false)
			degradeAttempt(&receipt.Attempts[1], false)
		}},
		"21 complete fails": {want: EngineeringInsightFailedOutcome, apply: func(receipt *EngineeringInsightEvaluationReceipt) {
			degradeAttempt(&receipt.Attempts[0], true)
			degradeAttempt(&receipt.Attempts[1], true)
			degradeAttempt(&receipt.Attempts[2], true)
		}},
		"12 useful fails": {want: EngineeringInsightFailedOutcome, apply: func(receipt *EngineeringInsightEvaluationReceipt) {
			for index := 0; index < 4; index++ {
				receipt.Attempts[index].Score.Correctness = 0
				receipt.Attempts[index].Score.LocalRelevance = 1
			}
		}},
		"one bad control fails": {want: EngineeringInsightFailedOutcome, apply: func(receipt *EngineeringInsightEvaluationReceipt) {
			for index := range receipt.Attempts {
				if receipt.Attempts[index].Intent == controlIntent {
					receipt.Attempts[index].OptionalInsight = "present"
					return
				}
			}
		}},
		"critical claim always fails": {want: EngineeringInsightFailedOutcome, apply: func(receipt *EngineeringInsightEvaluationReceipt) {
			receipt.Attempts[0].Score.CriticalFalseClaim = true
		}},
		"invalid provider metadata always fails": {want: EngineeringInsightFailedOutcome, apply: func(receipt *EngineeringInsightEvaluationReceipt) {
			receipt.Attempts[0].InvalidProviderMetadata = true
			receipt.Attempts[0].OutputTokens = qualificationOutputTokenCap + 1
			receipt.Consumption.OutputTokens += qualificationOutputTokenCap + 1 - 100
		}},
	} {
		t.Run(name, func(t *testing.T) {
			receipt := qualificationReceipt()
			if mutate.apply != nil {
				mutate.apply(&receipt)
			}
			report, err := ValidateEngineeringInsightEvaluationReceipt(receipt, expected)
			if err != nil {
				t.Fatalf("validate receipt: %v", err)
			}
			if report.Outcome != mutate.want {
				t.Fatalf("outcome = %s, want %s", report.Outcome, mutate.want)
			}
			if name == "thresholds pass" && (report.Latency.MedianMilliseconds != 12.5 || report.Latency.P95Milliseconds != 23) {
				t.Fatalf("latency = %+v, want median 12.5 and p95 23", report.Latency)
			}
		})
	}
}

func TestEngineeringInsightEvaluationPreservesCensoredDeadlineOverhead(t *testing.T) {
	expected := qualificationExpectation()
	timeout := qualificationReceipt()
	timeout.Attempts[0].Outcome = "timeout"
	timeout.Attempts[0].FinishReason = "timeout"
	timeout.Attempts[0].EmittedResponse = false
	timeout.Attempts[0].ResponseDigest = ""
	timeout.Attempts[0].OutputTokens = 0
	timeout.Consumption.OutputTokens -= 100
	timeout.Attempts[0].Score = nil
	timeout.Attempts[0].OptionalInsight = "not_evaluated"
	timeout.Attempts[0].UsableSummary = false
	timeout.Attempts[0].CompleteSummary = false
	timeout.Attempts[0].ElapsedMilliseconds = float64(qualificationAttemptTimeoutSeconds*1000 + 1)
	if _, err := ValidateEngineeringInsightEvaluationReceipt(timeout, expected); err != nil {
		t.Fatalf("censored timeout overhead was rejected: %v", err)
	}
	completed := qualificationReceipt()
	completed.Attempts[0].ElapsedMilliseconds = float64(qualificationAttemptTimeoutSeconds*1000 + 1)
	if _, err := ValidateEngineeringInsightEvaluationReceipt(completed, expected); err == nil {
		t.Fatal("overlong completed attempt was accepted")
	}
}

func TestDecodeEngineeringInsightScoresIsStrict(t *testing.T) {
	valid := `{"case":{"response_digest":"` + strings.Repeat("a", 64) + `","correctness":2,"local_relevance":2,"tradeoff_clarity":2,"useful_verification":2,"critical_false_claim":false,"retain_example":false}}`
	if _, err := DecodeEngineeringInsightScores([]byte(valid)); err != nil {
		t.Fatal(err)
	}
	for _, data := range []string{
		`{"case":{"response_digest":"` + strings.Repeat("a", 64) + `","correctness":2,"local_relevance":2,"tradeoff_clarity":2,"useful_verification":2,"critical_false_claim":false,"retain_example":false,"extra":true}}`,
		`{"case":{"response_digest":"` + strings.Repeat("a", 64) + `","response_digest":"` + strings.Repeat("b", 64) + `","correctness":2,"local_relevance":2,"tradeoff_clarity":2,"useful_verification":2,"critical_false_claim":false,"retain_example":false}}`,
		valid + ` {}`,
	} {
		if _, err := DecodeEngineeringInsightScores([]byte(data)); err == nil {
			t.Fatal("invalid score document accepted")
		}
	}
}

func TestEngineeringInsightEvaluationRejectsCaseIntentChangedBetweenRepetitions(t *testing.T) {
	expected := qualificationExpectation()
	expected.Schedule[12].Intent = controlIntent
	if _, err := ValidateEngineeringInsightEvaluationReceipt(qualificationReceipt(), expected); err == nil {
		t.Fatal("schedule with a changed case intent was accepted")
	}
}

func TestEngineeringInsightEvaluationSeparatesCollectionIncompleteAndInvalidEvidence(t *testing.T) {
	expected := qualificationExpectation()
	collection := qualificationReceipt()
	collection.Mode = EngineeringInsightCollectionMode
	collection.Attempts = collection.Attempts[:1]
	collection.Consumption.Requests = 1
	collection.Consumption.OutputTokens = collection.Attempts[0].OutputTokens
	collection.Attempts[0].Score = nil
	report, err := ValidateEngineeringInsightEvaluationReceipt(collection, expected)
	if err != nil || report.Outcome != EngineeringInsightCollectionOutcome {
		t.Fatalf("collection = %+v, %v", report, err)
	}

	incomplete := qualificationReceipt()
	incomplete.Attempts = incomplete.Attempts[:23]
	incomplete.Consumption.Requests = len(incomplete.Attempts)
	incomplete.Consumption.OutputTokens -= 100
	report, err = ValidateEngineeringInsightEvaluationReceipt(incomplete, expected)
	if err != nil || report.Outcome != EngineeringInsightIncompleteOutcome {
		t.Fatalf("incomplete = %+v, %v", report, err)
	}

	timeout := qualificationReceipt()
	makeNonCompletedAttempt(&timeout.Attempts[0], "timeout", "timeout")
	report, err = ValidateEngineeringInsightEvaluationReceipt(timeout, expected)
	if err != nil || report.Outcome != EngineeringInsightPassedOutcome || report.Latency.SuccessfulCount != 23 || report.Latency.TimeoutOrCensoredAttempts != 1 {
		t.Fatalf("timeout evidence = %+v, %v", report, err)
	}

	for name, mutate := range map[string]func(*EngineeringInsightEvaluationReceipt){
		"unscored emitted qualification response": func(receipt *EngineeringInsightEvaluationReceipt) { receipt.Attempts[0].Score = nil },
		"mixed candidate":                         func(receipt *EngineeringInsightEvaluationReceipt) { receipt.Attempts[0].CandidateID = "other" },
		"duplicate attempt":                       func(receipt *EngineeringInsightEvaluationReceipt) { receipt.Attempts[1] = receipt.Attempts[0] },
		"out of order attempt": func(receipt *EngineeringInsightEvaluationReceipt) {
			receipt.Attempts[0], receipt.Attempts[1] = receipt.Attempts[1], receipt.Attempts[0]
		},
		"selected cap mismatch": func(receipt *EngineeringInsightEvaluationReceipt) { receipt.MaxOutputTokens++ },
		"over cap consumption":  func(receipt *EngineeringInsightEvaluationReceipt) { receipt.Consumption.Requests++ },
		"elapsed time exceeds cap": func(receipt *EngineeringInsightEvaluationReceipt) {
			receipt.Attempts[0].ElapsedMilliseconds = 300001
		},
		"nonfinite latency": func(receipt *EngineeringInsightEvaluationReceipt) {
			receipt.Attempts[0].ElapsedMilliseconds = math.Inf(1)
		},
		"completed length finish": func(receipt *EngineeringInsightEvaluationReceipt) {
			receipt.Attempts[0].FinishReason = "length"
		},
		"truncated stop finish": func(receipt *EngineeringInsightEvaluationReceipt) {
			makeNonCompletedAttempt(&receipt.Attempts[0], "truncated", "stop")
		},
		"timeout error finish": func(receipt *EngineeringInsightEvaluationReceipt) {
			makeNonCompletedAttempt(&receipt.Attempts[0], "timeout", "error")
		},
		"canceled completed finish": func(receipt *EngineeringInsightEvaluationReceipt) {
			receipt.Attempts[0].FinishReason = "canceled"
		},
	} {
		t.Run(name, func(t *testing.T) {
			receipt := qualificationReceipt()
			mutate(&receipt)
			if _, err := ValidateEngineeringInsightEvaluationReceipt(receipt, expected); err == nil {
				t.Fatal("invalid evidence was accepted")
			}
		})
	}
}

func TestDecodeEngineeringInsightEvaluationReceiptRejectsUnsafeJSON(t *testing.T) {
	for name, data := range map[string]string{
		"unknown field":   `{"unexpected":true}`,
		"trailing JSON":   `{} {}`,
		"duplicate field": `{"mode":"qualification","mode":"collection"}`,
		"missing fields":  `{"mode":"qualification"}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeEngineeringInsightEvaluationReceipt([]byte(data)); err == nil || strings.Contains(err.Error(), "unexpected") {
				t.Fatal("unsafe JSON was accepted or exposed")
			}
		})
	}
}

func TestDecodeEngineeringInsightEvaluationReceiptRejectsNullAndCaseAliasFields(t *testing.T) {
	receipt, err := json.Marshal(qualificationReceipt())
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{
		"required scalar null": strings.Replace(string(receipt), `"provider":"provider"`, `"provider":null`, 1),
		"case alias":           strings.Replace(string(receipt), `"critical_false_claim":false`, `"critical_false_claim":true,"CRITICAL_FALSE_CLAIM":false`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeEngineeringInsightEvaluationReceipt([]byte(data)); err == nil || strings.Contains(err.Error(), "CRITICAL") {
				t.Fatal("unsafe field alias or null was accepted or exposed")
			}
		})
	}
}

func qualificationExpectation() EngineeringInsightEvaluationExpectation {
	schedule := make([]EngineeringInsightExpectedAttempt, 0, 24)
	for repetition := 1; repetition <= 2; repetition++ {
		for index := 0; index < 12; index++ {
			intent := substantiveIntent
			if index >= 8 {
				intent = controlIntent
			}
			schedule = append(schedule, EngineeringInsightExpectedAttempt{CaseName: fmt.Sprintf("qualification-%02d", index), Partition: qualificationPartition, Intent: intent, Repetition: repetition, Attempt: 1})
		}
	}
	return EngineeringInsightEvaluationExpectation{CandidateID: "candidate-v1", Provider: "provider", Model: "model", PromptVersion: semanticAnalysisPromptVersion, CorpusID: "engineering-insight-v1", CorpusDigest: strings.Repeat("a", 64), BaseRevision: "base6a14c01", MaxRequests: 24, MaxOutputTokens: 4096, AttemptTimeoutSeconds: 300, Schedule: schedule}
}

func qualificationReceipt() EngineeringInsightEvaluationReceipt {
	expected := qualificationExpectation()
	receipt := EngineeringInsightEvaluationReceipt{Mode: EngineeringInsightQualificationMode, RunID: "run-1", CandidateID: expected.CandidateID, Provider: expected.Provider, Model: expected.Model, PromptVersion: expected.PromptVersion, CorpusID: expected.CorpusID, CorpusDigest: expected.CorpusDigest, BaseRevision: expected.BaseRevision, MaxRequests: expected.MaxRequests, MaxOutputTokens: expected.MaxOutputTokens, AttemptTimeoutSeconds: expected.AttemptTimeoutSeconds}
	for index, scheduled := range expected.Schedule {
		digest := fmt.Sprintf("%064x", index+1)
		optional := "present"
		if scheduled.Intent == controlIntent {
			optional = "omitted"
		}
		receipt.Attempts = append(receipt.Attempts, EngineeringInsightEvaluationAttempt{CaseName: scheduled.CaseName, Partition: scheduled.Partition, Intent: scheduled.Intent, Repetition: scheduled.Repetition, Attempt: scheduled.Attempt, CandidateID: expected.CandidateID, Outcome: "completed", UsableSummary: true, CompleteSummary: true, OptionalInsight: optional, EmittedResponse: true, ResponseDigest: digest, OutputTokens: 100, FinishReason: "stop", ElapsedMilliseconds: float64(index + 1), Score: &EngineeringInsightAttemptScore{ResponseDigest: digest, Correctness: 2, LocalRelevance: 2, TradeoffClarity: 2, UsefulVerification: 2}})
	}
	receipt.Consumption = EngineeringInsightEvaluationConsumption{Requests: len(receipt.Attempts), OutputTokens: len(receipt.Attempts) * 100}
	return receipt
}

func degradeAttempt(attempt *EngineeringInsightEvaluationAttempt, keepUsable bool) {
	attempt.CompleteSummary = false
	attempt.UsableSummary = keepUsable
	if keepUsable {
		attempt.OptionalInsight = "rejected"
	} else {
		attempt.OptionalInsight = "not_evaluated"
	}
}

func makeNonCompletedAttempt(attempt *EngineeringInsightEvaluationAttempt, outcome, finishReason string) {
	attempt.Outcome = outcome
	attempt.FinishReason = finishReason
	attempt.UsableSummary = false
	attempt.CompleteSummary = false
	attempt.OptionalInsight = "not_evaluated"
	attempt.OptionalSectionDegraded = false
}

func assertFixturePartitionAndIntent(t *testing.T, fixture engineeringInsightFixture) {
	t.Helper()
	if fixture.Partition != developmentPartition && fixture.Partition != qualificationPartition {
		t.Fatalf("unsupported fixture partition %q", fixture.Partition)
	}
	if fixture.Intent != substantiveIntent && fixture.Intent != controlIntent {
		t.Fatalf("unsupported fixture intent %q", fixture.Intent)
	}
	if fixture.ExpectInsight != (fixture.Intent == substantiveIntent) {
		t.Fatalf("expect_insight=%t does not match %s intent", fixture.ExpectInsight, fixture.Intent)
	}
}

func assertFixtureRubric(t *testing.T, fixture engineeringInsightFixture) {
	t.Helper()
	for label, values := range map[string][]string{
		"reference mechanism":          fixture.ReferenceMechanisms,
		"permitted uncertainty":        fixture.PermittedUncertainty,
		"critical false-claim example": fixture.CriticalFalseClaimExamples,
	} {
		if len(values) == 0 {
			t.Fatalf("fixture has no %s", label)
		}
		for _, value := range values {
			if strings.TrimSpace(value) == "" {
				t.Fatalf("fixture has blank %s", label)
			}
		}
	}
	for _, dimension := range []string{"correctness", "local_relevance", "tradeoff_clarity", "useful_verification"} {
		anchors := fixture.ScoringAnchors[dimension]
		for _, score := range []string{"0", "1", "2"} {
			if strings.TrimSpace(anchors[score]) == "" {
				t.Fatalf("fixture is missing %s score anchor %s", dimension, score)
			}
		}
	}
	if len(fixture.ScoringAnchors) != 4 {
		t.Fatalf("fixture scoring dimensions = %d, want 4", len(fixture.ScoringAnchors))
	}
}

func assertFixtureSemanticOutput(t *testing.T, fixture engineeringInsightFixture) {
	t.Helper()
	var wire map[string]json.RawMessage
	if err := json.Unmarshal([]byte(fixture.SanitizedOutput), &wire); err != nil {
		t.Fatalf("sanitized output is not JSON: %v", err)
	}
	parsed, err := parseSemanticAnalysis(fixture.SanitizedOutput, project.IndexFile{Path: "fixture.go", Language: "Go"}, fixture.Source)
	if err != nil {
		t.Fatalf("sanitized output rejected its valid parent: %v", err)
	}
	if !fixture.ExpectInsight {
		if _, present := wire["engineering_insight"]; present {
			t.Fatal("control contains an engineering insight instead of intentionally omitting it")
		}
		if parsed.EngineeringInsight != nil {
			t.Fatalf("control retained an insight: %+v", parsed.EngineeringInsight)
		}
		return
	}
	if parsed.EngineeringInsight == nil {
		t.Fatal("grounded insight was omitted")
	}
	insight := wire["engineering_insight"]
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(insight, &fields); err != nil {
		t.Fatalf("insight structure = %v", err)
	}
	for _, field := range []string{"mechanism", "why_it_matters_here", "tradeoff_or_failure_mode", "transferable_lesson"} {
		if len(fields[field]) == 0 {
			t.Fatalf("insight is missing %q: %s", field, insight)
		}
	}
	if !project.ValidEngineeringInsight(parsed.EngineeringInsight) {
		t.Fatalf("insight schema or 1,000-rune bound is invalid: %+v", parsed.EngineeringInsight)
	}
}

func loadEngineeringInsightFixtures(t *testing.T) []engineeringInsightFixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "engineering-insight-eval", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []engineeringInsightFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) == 0 {
		t.Fatal("engineering insight fixture set is empty")
	}
	return fixtures
}

func assertFixtureSourceAnchor(t *testing.T, fixture engineeringInsightFixture) {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(fixture.Source, "\n"), "\n")
	if fixture.Anchor.StartLine <= 0 || fixture.Anchor.EndLine < fixture.Anchor.StartLine || fixture.Anchor.EndLine > len(lines) {
		t.Fatalf("invalid source anchor %+v for %d source lines", fixture.Anchor, len(lines))
	}
	anchoredSource := strings.Join(lines[fixture.Anchor.StartLine-1:fixture.Anchor.EndLine], "\n")
	if fixture.Anchor.Evidence == "" || !strings.Contains(anchoredSource, fixture.Anchor.Evidence) {
		t.Fatalf("source anchor %+v does not contain its evidence in %q", fixture.Anchor, anchoredSource)
	}
}
