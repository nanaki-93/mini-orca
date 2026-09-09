package project

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"unicode/utf8"
)

const maxEngineeringInsightRunes = 1000

// OptionalEngineeringInsightReason records only the validation category for
// optional insight prose. It intentionally carries no model-provided text.
type OptionalEngineeringInsightReason string

const (
	OptionalEngineeringInsightAbsent             OptionalEngineeringInsightReason = "absent"
	OptionalEngineeringInsightNull               OptionalEngineeringInsightReason = "null"
	OptionalEngineeringInsightInvalidShape       OptionalEngineeringInsightReason = "invalid_shape"
	OptionalEngineeringInsightEmptyRequiredField OptionalEngineeringInsightReason = "empty_required_field"
	OptionalEngineeringInsightOverLimit          OptionalEngineeringInsightReason = "over_limit"
	OptionalEngineeringInsightAccepted           OptionalEngineeringInsightReason = "accepted"
)

// OptionalEngineeringInsightPresence distinguishes an omitted field from an
// explicit JSON null without retaining the supplied value.
type OptionalEngineeringInsightPresence string

const (
	OptionalEngineeringInsightAbsentPresence OptionalEngineeringInsightPresence = "absent"
	OptionalEngineeringInsightNullPresence   OptionalEngineeringInsightPresence = "null"
	OptionalEngineeringInsightValuePresence  OptionalEngineeringInsightPresence = "value"
)

// OptionalEngineeringInsightFieldDiagnostic is source-free validation
// evidence for one supported field.
type OptionalEngineeringInsightFieldDiagnostic struct {
	Present        bool `json:"present"`
	RuneCountKnown bool `json:"rune_count_known"`
	Runes          int  `json:"runes"`
}

// OptionalEngineeringInsightDiagnostic contains only stable validation facts.
// It is suitable for private evidence records and must never be populated with
// model values or parser errors.
type OptionalEngineeringInsightDiagnostic struct {
	Reason                OptionalEngineeringInsightReason          `json:"reason"`
	Presence              OptionalEngineeringInsightPresence        `json:"presence"`
	Mechanism             OptionalEngineeringInsightFieldDiagnostic `json:"mechanism"`
	WhyItMattersHere      OptionalEngineeringInsightFieldDiagnostic `json:"why_it_matters_here"`
	TradeoffOrFailureMode OptionalEngineeringInsightFieldDiagnostic `json:"tradeoff_or_failure_mode"`
	TransferableLesson    OptionalEngineeringInsightFieldDiagnostic `json:"transferable_lesson"`
}

// EngineeringInsightPromptInstructions keeps every model producer aligned with
// the bounded advisory contract accepted below.
const EngineeringInsightPromptInstructions = "Engineering insights are optional, concise advisory prose. Include one only when it gives non-obvious, job-useful guidance grounded in supplied local evidence: mechanism explains the concrete mechanism; why_it_matters_here names the exact local evidence and impact; tradeoff_or_failure_mode names a real trade-off or failure condition; transferable_lesson gives a reusable lesson with a concrete verification idea. Omit trivial or generic lessons. "

// EngineeringInsight is advisory model prose attached to the result that owns
// its identity and freshness. It deliberately carries no navigation or mutation authority.
type EngineeringInsight struct {
	Mechanism             string `json:"mechanism"`
	WhyItMattersHere      string `json:"why_it_matters_here"`
	TradeoffOrFailureMode string `json:"tradeoff_or_failure_mode,omitempty"`
	TransferableLesson    string `json:"transferable_lesson,omitempty"`
}

// ParseOptionalEngineeringInsight isolates optional model prose from strict
// parent contracts. Invalid prose is omitted without rejecting valid parent data.
func ParseOptionalEngineeringInsight(raw json.RawMessage) (*EngineeringInsight, string) {
	insight, diagnostic := ParseOptionalEngineeringInsightDiagnostic(raw)
	if diagnostic.Reason == OptionalEngineeringInsightAbsent || diagnostic.Reason == OptionalEngineeringInsightNull {
		return nil, ""
	}
	if diagnostic.Reason != OptionalEngineeringInsightAccepted {
		return nil, "optional engineering insight omitted"
	}
	return insight, ""
}

// ParseOptionalEngineeringInsightDiagnostic shares the production parser's
// validation path while returning source-free facts needed by private
// evaluation evidence. ParseOptionalEngineeringInsight retains its generic
// user-facing diagnostic for compatibility.
func ParseOptionalEngineeringInsightDiagnostic(raw json.RawMessage) (*EngineeringInsight, OptionalEngineeringInsightDiagnostic) {
	diagnostic := OptionalEngineeringInsightDiagnostic{Reason: OptionalEngineeringInsightInvalidShape}
	if len(raw) == 0 {
		diagnostic.Reason = OptionalEngineeringInsightAbsent
		diagnostic.Presence = OptionalEngineeringInsightAbsentPresence
		return nil, diagnostic
	}
	trimmed := bytes.TrimSpace(raw)
	if bytes.Equal(trimmed, []byte("null")) {
		diagnostic.Reason = OptionalEngineeringInsightNull
		diagnostic.Presence = OptionalEngineeringInsightNullPresence
		return nil, diagnostic
	}
	diagnostic.Presence = OptionalEngineeringInsightValuePresence
	fields := engineeringInsightDiagnosticFields(trimmed)
	diagnostic.Mechanism = fields[0]
	diagnostic.WhyItMattersHere = fields[1]
	diagnostic.TradeoffOrFailureMode = fields[2]
	diagnostic.TransferableLesson = fields[3]

	var insight EngineeringInsight
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&insight); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return nil, diagnostic
	}

	if reason := normalizeAndValidateEngineeringInsight(&insight); reason != OptionalEngineeringInsightAccepted {
		diagnostic.Reason = reason
		return nil, diagnostic
	}
	diagnostic.Reason = OptionalEngineeringInsightAccepted
	return &insight, diagnostic
}

func engineeringInsightDiagnosticFields(raw json.RawMessage) [4]OptionalEngineeringInsightFieldDiagnostic {
	var values map[string]json.RawMessage
	if json.Unmarshal(raw, &values) != nil {
		return [4]OptionalEngineeringInsightFieldDiagnostic{}
	}
	fieldNames := []string{"mechanism", "why_it_matters_here", "tradeoff_or_failure_mode", "transferable_lesson"}
	var fields [4]OptionalEngineeringInsightFieldDiagnostic
	for index, name := range fieldNames {
		for actual, value := range values {
			if strings.EqualFold(actual, name) {
				fields[index].Present = true
				var text string
				if json.Unmarshal(value, &text) == nil {
					fields[index].RuneCountKnown = true
					fields[index].Runes = utf8.RuneCountInString(normalizeEngineeringInsightText(text))
				}
				break
			}
		}
	}
	return fields
}

// ValidEngineeringInsight normalizes trusted presentation text after decoding.
func ValidEngineeringInsight(insight *EngineeringInsight) bool {
	if insight == nil {
		return false
	}
	return normalizeAndValidateEngineeringInsight(insight) == OptionalEngineeringInsightAccepted
}

func normalizeAndValidateEngineeringInsight(insight *EngineeringInsight) OptionalEngineeringInsightReason {
	insight.Mechanism = normalizeEngineeringInsightText(insight.Mechanism)
	insight.WhyItMattersHere = normalizeEngineeringInsightText(insight.WhyItMattersHere)
	insight.TradeoffOrFailureMode = normalizeEngineeringInsightText(insight.TradeoffOrFailureMode)
	insight.TransferableLesson = normalizeEngineeringInsightText(insight.TransferableLesson)
	if insight.Mechanism == "" || insight.WhyItMattersHere == "" {
		return OptionalEngineeringInsightEmptyRequiredField
	}
	if utf8.RuneCountInString(insight.Mechanism+insight.WhyItMattersHere+insight.TradeoffOrFailureMode+insight.TransferableLesson) > maxEngineeringInsightRunes {
		return OptionalEngineeringInsightOverLimit
	}
	return OptionalEngineeringInsightAccepted
}

func normalizeEngineeringInsightText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func CloneEngineeringInsight(source *EngineeringInsight) *EngineeringInsight {
	if source == nil {
		return nil
	}
	copy := *source
	return &copy
}
