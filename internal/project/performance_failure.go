package project

// PerformanceReviewFailure is safe to retain in source-free run metadata. It
// never contains provider output, prose, paths, or decoder-supplied field names.
type PerformanceReviewFailure string

const (
	PerformanceOutputTooLarge  PerformanceReviewFailure = "The Performance response exceeded the output size limit."
	PerformanceInvalidJSON     PerformanceReviewFailure = "The Performance response was not valid findings JSON."
	PerformanceInvalidShape    PerformanceReviewFailure = "The Performance response must contain a findings array with at most five entries."
	PerformanceInvalidAnchors  PerformanceReviewFailure = "Performance findings referenced invalid lines or symbols."
	PerformanceInvalidFields   PerformanceReviewFailure = "Performance findings were missing required explanations."
	PerformanceInvalidValues   PerformanceReviewFailure = "Performance findings used unsupported category, impact or confidence values."
	PerformanceInvalidFindings PerformanceReviewFailure = "No Performance findings passed the required field and source anchor checks."
)

func (failure PerformanceReviewFailure) Error() string { return string(failure) }

func (failure PerformanceReviewFailure) Valid() bool {
	switch failure {
	case PerformanceOutputTooLarge, PerformanceInvalidJSON, PerformanceInvalidShape, PerformanceInvalidAnchors, PerformanceInvalidFields, PerformanceInvalidValues, PerformanceInvalidFindings:
		return true
	}
	return false
}

func (f performanceFindingValidationFailures) failure() PerformanceReviewFailure {
	switch {
	case f.anchor && !f.fields && !f.enums:
		return PerformanceInvalidAnchors
	case f.fields && !f.anchor && !f.enums:
		return PerformanceInvalidFields
	case f.enums && !f.anchor && !f.fields:
		return PerformanceInvalidValues
	default:
		return PerformanceInvalidFindings
	}
}
