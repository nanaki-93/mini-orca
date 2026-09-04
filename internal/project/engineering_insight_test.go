package project

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOptionalEngineeringInsightIsBoundedAndIndependent(t *testing.T) {
	valid := json.RawMessage(`{"mechanism":"  Cache identity prevents a late answer from replacing current evidence. ","why_it_matters_here":"  The review remains advisory unless it belongs to this file hash.  ","tradeoff_or_failure_mode":"Extra identity checks reject stale work."}`)
	insight, diagnostic := ParseOptionalEngineeringInsight(valid)
	if diagnostic != "" || insight == nil || insight.Mechanism != "Cache identity prevents a late answer from replacing current evidence." {
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
	if insight, diagnostic := ParseOptionalEngineeringInsight(nil); insight != nil || diagnostic != "" {
		t.Fatalf("absent insight = %+v, %q", insight, diagnostic)
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
