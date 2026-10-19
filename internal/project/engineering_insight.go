package project

import (
	"encoding/json"

	"github.com/nanaki-93/mini-orca/v2/internal/project/insight"
)

const MaxEngineeringInsightRunes = insight.MaxEngineeringInsightRunes

type OptionalEngineeringInsightReason = insight.OptionalEngineeringInsightReason

const (
	OptionalEngineeringInsightAbsent             = insight.OptionalEngineeringInsightAbsent
	OptionalEngineeringInsightNull               = insight.OptionalEngineeringInsightNull
	OptionalEngineeringInsightInvalidShape       = insight.OptionalEngineeringInsightInvalidShape
	OptionalEngineeringInsightEmptyRequiredField = insight.OptionalEngineeringInsightEmptyRequiredField
	OptionalEngineeringInsightOverLimit          = insight.OptionalEngineeringInsightOverLimit
	OptionalEngineeringInsightAccepted           = insight.OptionalEngineeringInsightAccepted
)

type OptionalEngineeringInsightPresence = insight.OptionalEngineeringInsightPresence

const (
	OptionalEngineeringInsightAbsentPresence = insight.OptionalEngineeringInsightAbsentPresence
	OptionalEngineeringInsightNullPresence   = insight.OptionalEngineeringInsightNullPresence
	OptionalEngineeringInsightValuePresence  = insight.OptionalEngineeringInsightValuePresence
)

type OptionalEngineeringInsightFieldDiagnostic = insight.OptionalEngineeringInsightFieldDiagnostic
type OptionalEngineeringInsightDiagnostic = insight.OptionalEngineeringInsightDiagnostic

const EngineeringInsightPromptInstructions = insight.EngineeringInsightPromptInstructions

type EngineeringInsight = insight.EngineeringInsight

// ParseOptionalEngineeringInsight isolates optional model prose from strict
// parent contracts. Invalid prose is omitted without rejecting valid parent data.
func ParseOptionalEngineeringInsight(raw json.RawMessage) (*EngineeringInsight, string) {
	return insight.ParseOptionalEngineeringInsight(raw)
}

// ParseOptionalEngineeringInsightDiagnostic shares the production parser's
// validation path while returning source-free facts needed by private
// evaluation evidence.
func ParseOptionalEngineeringInsightDiagnostic(raw json.RawMessage) (*EngineeringInsight, OptionalEngineeringInsightDiagnostic) {
	return insight.ParseOptionalEngineeringInsightDiagnostic(raw)
}

// ValidEngineeringInsight normalizes trusted presentation text after decoding.
func ValidEngineeringInsight(value *EngineeringInsight) bool {
	return insight.ValidEngineeringInsight(value)
}

func CloneEngineeringInsight(source *EngineeringInsight) *EngineeringInsight {
	return insight.CloneEngineeringInsight(source)
}

const (
	BugTaskSpecSchemaVersion = insight.BugTaskSpecSchemaVersion
	MaxBugTaskItems          = insight.MaxBugTaskItems
	MaxBugTaskItemBytes      = insight.MaxBugTaskItemBytes
	MaxBugTaskCandidateBytes = insight.MaxBugTaskCandidateBytes
)

type BugTaskSpec = insight.BugTaskSpec
type GoTestCandidateSpec = insight.GoTestCandidateSpec

// SanitizeBugTaskSpec applies the same secret redaction and text normalization
// used by persisted findings while preserving no shared mutable slices.
func SanitizeBugTaskSpec(spec *BugTaskSpec) *BugTaskSpec {
	return insight.SanitizeBugTaskSpec(spec)
}
