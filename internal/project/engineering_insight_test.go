package project

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOptionalEngineeringInsightIsBoundedAndIndependent(t *testing.T) {
	valid := json.RawMessage(`{"mechanism":"  Cache\nidentity prevents a late answer from replacing current evidence. ","why_it_matters_here":"  The review\tremains advisory unless it belongs to this file hash.  ","tradeoff_or_failure_mode":"Extra identity checks reject stale work."}`)
	insight, diagnostic := ParseOptionalEngineeringInsight(valid)
	if diagnostic != "" || insight == nil || insight.Mechanism != "Cache identity prevents a late answer from replacing current evidence." || insight.WhyItMattersHere != "The review remains advisory unless it belongs to this file hash." {
		t.Fatalf("valid insight = %+v, %q", insight, diagnostic)
	}
	for _, raw := range []json.RawMessage{
		json.RawMessage(`{"mechanism":"only one required field"}`),
		json.RawMessage(`{"mechanism":"x","why_it_matters_here":"y","unknown":"z"}`),
		json.RawMessage(`{"mechanism":3,"why_it_matters_here":"y"}`),
		json.RawMessage(`{"mechanism":"` + strings.Repeat("界", maxEngineeringInsightRunes) + `","why_it_matters_here":"y"}`),
	} {
		if insight, diagnostic := ParseOptionalEngineeringInsight(raw); insight != nil || diagnostic != "optional engineering insight omitted" {
			t.Fatalf("invalid insight = %+v, %q", insight, diagnostic)
		}
	}
	for _, raw := range []json.RawMessage{nil, json.RawMessage(`null`)} {
		if insight, diagnostic := ParseOptionalEngineeringInsight(raw); insight != nil || diagnostic != "" {
			t.Fatalf("absent insight = %+v, %q", insight, diagnostic)
		}
	}
}

func TestOptionalEngineeringInsightCountsUnicodeRunesAcrossAllFields(t *testing.T) {
	atLimit := json.RawMessage(`{"mechanism":"` + strings.Repeat("界", 400) + `","why_it_matters_here":"` + strings.Repeat("界", 300) + `","tradeoff_or_failure_mode":"` + strings.Repeat("界", 200) + `","transferable_lesson":"` + strings.Repeat("界", 100) + `"}`)
	insight, diagnostic := ParseOptionalEngineeringInsight(atLimit)
	if diagnostic != "" || insight == nil {
		t.Fatalf("insight at rune limit = %+v, %q", insight, diagnostic)
	}
	overLimit := json.RawMessage(`{"mechanism":"` + strings.Repeat("界", 400) + `","why_it_matters_here":"` + strings.Repeat("界", 300) + `","tradeoff_or_failure_mode":"` + strings.Repeat("界", 200) + `","transferable_lesson":"` + strings.Repeat("界", 101) + `"}`)
	if insight, diagnostic := ParseOptionalEngineeringInsight(overLimit); insight != nil || diagnostic != "optional engineering insight omitted" {
		t.Fatalf("insight over rune limit = %+v, %q", insight, diagnostic)
	}
}

func TestFindingIdentityIgnoresEngineeringInsight(t *testing.T) {
	base := UnifiedFinding{Source: FindingSourceAI, Confidence: FindingConfidenceSuggested, Severity: "medium", Title: "Suggestion", Message: "Same finding", Location: FindingLocation{Path: "main.go"}}
	withInsight := base
	withInsight.EngineeringInsight = &EngineeringInsight{Mechanism: "An extra explanation", WhyItMattersHere: "Does not change triage identity"}
	if FindingID(base) != FindingID(withInsight) {
		t.Fatal("insight wording changed finding identity")
	}
}
