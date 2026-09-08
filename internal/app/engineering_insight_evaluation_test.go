package app

import (
	"encoding/json"
	"fmt"
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

func TestEngineeringInsightEvaluationReceiptRequiresASelectedBoundedSampleAndSafeRetention(t *testing.T) {
	accepted := EngineeringInsightEvaluationReceipt{
		Provider: "chosen-provider", Model: "chosen-model", PromptVersion: semanticAnalysisPromptVersion,
		SampleCount: 1, MaxRequests: 1, MaxOutputTokens: 800,
		Samples: []EngineeringInsightSampleScore{{CaseName: "development-slice-capacity", Correctness: 2, LocalRelevance: 2, TradeoffClarity: 1, UsefulVerification: 1, RetainExample: true}},
	}
	if err := ValidateEngineeringInsightEvaluationReceipt(accepted); err != nil {
		t.Fatalf("accepted receipt = %v", err)
	}
	for name, mutate := range map[string]func(*EngineeringInsightEvaluationReceipt){
		"missing provider":     func(receipt *EngineeringInsightEvaluationReceipt) { receipt.Provider = "" },
		"unbounded sample":     func(receipt *EngineeringInsightEvaluationReceipt) { receipt.SampleCount = 2 },
		"critical false claim": func(receipt *EngineeringInsightEvaluationReceipt) { receipt.Samples[0].CriticalFalseClaim = true },
		"low retained score": func(receipt *EngineeringInsightEvaluationReceipt) {
			receipt.Samples[0].Correctness = 1
			receipt.Samples[0].LocalRelevance = 1
		},
	} {
		t.Run(name, func(t *testing.T) {
			receipt := accepted
			receipt.Samples = append([]EngineeringInsightSampleScore(nil), accepted.Samples...)
			mutate(&receipt)
			if err := ValidateEngineeringInsightEvaluationReceipt(receipt); err == nil {
				t.Fatal("invalid evaluation receipt was accepted")
			}
		})
	}
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
