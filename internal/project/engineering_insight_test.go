package project

import "testing"

func TestFindingIdentityIgnoresEngineeringInsight(t *testing.T) {
	base := UnifiedFinding{Source: FindingSourceAI, Confidence: FindingConfidenceSuggested, Severity: "medium", Title: "Suggestion", Message: "Same finding", Location: FindingLocation{Path: "main.go"}}
	withInsight := base
	withInsight.EngineeringInsight = &EngineeringInsight{Mechanism: "An extra explanation", WhyItMattersHere: "Does not change triage identity"}
	if FindingID(base) != FindingID(withInsight) {
		t.Fatal("insight wording changed finding identity")
	}
}
