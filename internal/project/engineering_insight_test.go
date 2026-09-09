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

func TestOptionalEngineeringInsightDiagnosticIsSourceFreeAndKeepsParserCompatibility(t *testing.T) {
	for _, test := range []struct {
		name         string
		raw          json.RawMessage
		wantReason   OptionalEngineeringInsightReason
		wantPresence OptionalEngineeringInsightPresence
		wantInsight  bool
		mechanism    OptionalEngineeringInsightFieldDiagnostic
		why          OptionalEngineeringInsightFieldDiagnostic
	}{
		{name: "absent", wantReason: OptionalEngineeringInsightAbsent, wantPresence: OptionalEngineeringInsightAbsentPresence},
		{name: "null", raw: json.RawMessage(`null`), wantReason: OptionalEngineeringInsightNull, wantPresence: OptionalEngineeringInsightNullPresence},
		{name: "invalid type", raw: json.RawMessage(`"advice"`), wantReason: OptionalEngineeringInsightInvalidShape, wantPresence: OptionalEngineeringInsightValuePresence},
		{name: "wrong typed known field", raw: json.RawMessage(`{"mechanism":3,"why_it_matters_here":"y"}`), wantReason: OptionalEngineeringInsightInvalidShape, wantPresence: OptionalEngineeringInsightValuePresence, mechanism: OptionalEngineeringInsightFieldDiagnostic{Present: true}, why: OptionalEngineeringInsightFieldDiagnostic{Present: true, RuneCountKnown: true, Runes: 1}},
		{name: "unknown key", raw: json.RawMessage(`{"mechanism":"x","why_it_matters_here":"y","unexpected":"secret"}`), wantReason: OptionalEngineeringInsightInvalidShape, wantPresence: OptionalEngineeringInsightValuePresence, mechanism: OptionalEngineeringInsightFieldDiagnostic{Present: true, RuneCountKnown: true, Runes: 1}, why: OptionalEngineeringInsightFieldDiagnostic{Present: true, RuneCountKnown: true, Runes: 1}},
		{name: "malformed JSON", raw: json.RawMessage(`{"mechanism":"x",`), wantReason: OptionalEngineeringInsightInvalidShape, wantPresence: OptionalEngineeringInsightValuePresence},
		{name: "whitespace is invalid JSON", raw: json.RawMessage(" \t "), wantReason: OptionalEngineeringInsightInvalidShape, wantPresence: OptionalEngineeringInsightValuePresence},
		{name: "empty normalized field", raw: json.RawMessage(`{"mechanism":" \t ","why_it_matters_here":"y"}`), wantReason: OptionalEngineeringInsightEmptyRequiredField, wantPresence: OptionalEngineeringInsightValuePresence, mechanism: OptionalEngineeringInsightFieldDiagnostic{Present: true, RuneCountKnown: true}, why: OptionalEngineeringInsightFieldDiagnostic{Present: true, RuneCountKnown: true, Runes: 1}},
		{name: "over limit", raw: json.RawMessage(`{"mechanism":"` + strings.Repeat("界", maxEngineeringInsightRunes) + `","why_it_matters_here":"y"}`), wantReason: OptionalEngineeringInsightOverLimit, wantPresence: OptionalEngineeringInsightValuePresence, mechanism: OptionalEngineeringInsightFieldDiagnostic{Present: true, RuneCountKnown: true, Runes: maxEngineeringInsightRunes}, why: OptionalEngineeringInsightFieldDiagnostic{Present: true, RuneCountKnown: true, Runes: 1}},
		{name: "accepted normalized", raw: json.RawMessage(`{"mechanism":"  alpha\n beta ","why_it_matters_here":"gamma","tradeoff_or_failure_mode":null}`), wantReason: OptionalEngineeringInsightAccepted, wantPresence: OptionalEngineeringInsightValuePresence, wantInsight: true, mechanism: OptionalEngineeringInsightFieldDiagnostic{Present: true, RuneCountKnown: true, Runes: len("alpha beta")}, why: OptionalEngineeringInsightFieldDiagnostic{Present: true, RuneCountKnown: true, Runes: len("gamma")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			insight, diagnostic := ParseOptionalEngineeringInsightDiagnostic(test.raw)
			if diagnostic.Reason != test.wantReason || diagnostic.Presence != test.wantPresence || (insight != nil) != test.wantInsight || diagnostic.Mechanism != test.mechanism || diagnostic.WhyItMattersHere != test.why {
				t.Fatalf("diagnostic = %+v, insight=%+v", diagnostic, insight)
			}
			parsed, message := ParseOptionalEngineeringInsight(test.raw)
			wantMessage := ""
			if test.wantReason != OptionalEngineeringInsightAbsent && test.wantReason != OptionalEngineeringInsightNull && test.wantReason != OptionalEngineeringInsightAccepted {
				wantMessage = "optional engineering insight omitted"
			}
			if (parsed != nil) != test.wantInsight || message != wantMessage {
				t.Fatalf("public parser = %+v, %q", parsed, message)
			}
		})
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
