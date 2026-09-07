package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type engineeringInsightFixture struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	Anchor struct {
		StartLine int    `json:"start_line"`
		EndLine   int    `json:"end_line"`
		Evidence  string `json:"evidence"`
	} `json:"anchor"`
	ExpectInsight   bool   `json:"expect_insight"`
	SanitizedOutput string `json:"sanitized_output"`
}

func TestEngineeringInsightEvaluationFixturesAreAnchoredBoundedAndOmitLowValueOutput(t *testing.T) {
	fixtures := loadEngineeringInsightFixtures(t)
	seen := make(map[string]bool, len(fixtures))
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			if fixture.Name == "" || seen[fixture.Name] {
				t.Fatalf("fixture name is empty or repeated: %q", fixture.Name)
			}
			seen[fixture.Name] = true
			assertFixtureSourceAnchor(t, fixture)

			var wire map[string]json.RawMessage
			if err := json.Unmarshal([]byte(fixture.SanitizedOutput), &wire); err != nil {
				t.Fatalf("sanitized output is not JSON: %v", err)
			}
			parsed, err := parseSemanticAnalysis(fixture.SanitizedOutput, project.IndexFile{Path: "fixture.go", Language: "Go"}, fixture.Source)
			if err != nil {
				t.Fatalf("sanitized output rejected its valid parent: %v", err)
			}
			if !fixture.ExpectInsight {
				if parsed.EngineeringInsight != nil {
					t.Fatalf("trivial, no-finding, or malformed insight was retained: %+v", parsed.EngineeringInsight)
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
		})
	}
	for _, name := range []string{"cancellation-locking", "allocation", "n-plus-one-io", "idempotency", "authorization", "trivial-edit", "no-finding", "malformed-overconfident"} {
		if !seen[name] {
			t.Fatalf("missing representative insight fixture %q", name)
		}
	}
}

func TestEngineeringInsightEvaluationReceiptRequiresASelectedBoundedSampleAndSafeRetention(t *testing.T) {
	accepted := EngineeringInsightEvaluationReceipt{
		Provider: "chosen-provider", Model: "chosen-model", PromptVersion: semanticAnalysisPromptVersion,
		SampleCount: 1, MaxRequests: 1, MaxOutputTokens: 800,
		Samples: []EngineeringInsightSampleScore{{CaseName: "allocation", Correctness: 2, LocalRelevance: 2, TradeoffClarity: 1, UsefulVerification: 1, RetainExample: true}},
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
