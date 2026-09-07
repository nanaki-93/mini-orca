package project

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"unicode/utf8"
)

const maxEngineeringInsightRunes = 1000

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
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, ""
	}
	var insight EngineeringInsight
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&insight); err != nil {
		return nil, "optional engineering insight omitted"
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, "optional engineering insight omitted"
	}
	if !ValidEngineeringInsight(&insight) {
		return nil, "optional engineering insight omitted"
	}
	return &insight, ""
}

// ValidEngineeringInsight normalizes trusted presentation text after decoding.
func ValidEngineeringInsight(insight *EngineeringInsight) bool {
	if insight == nil {
		return false
	}
	insight.Mechanism = normalizeEngineeringInsightText(insight.Mechanism)
	insight.WhyItMattersHere = normalizeEngineeringInsightText(insight.WhyItMattersHere)
	insight.TradeoffOrFailureMode = normalizeEngineeringInsightText(insight.TradeoffOrFailureMode)
	insight.TransferableLesson = normalizeEngineeringInsightText(insight.TransferableLesson)
	if insight.Mechanism == "" || insight.WhyItMattersHere == "" {
		return false
	}
	return utf8.RuneCountInString(insight.Mechanism+insight.WhyItMattersHere+insight.TradeoffOrFailureMode+insight.TransferableLesson) <= maxEngineeringInsightRunes
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
