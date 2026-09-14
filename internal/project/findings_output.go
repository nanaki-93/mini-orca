package project

import (
	"encoding/json"
	"strings"
)

// Review stages may return an empty section instead of a findings object. Only
// an actually empty array or blank content is normalized; invalid findings and
// missing object fields must still pass the producer's normal validation.
func normalizeEmptyFindingsOutput(output string) string {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return `{"findings":[]}`
	}
	if strings.HasPrefix(trimmed, "[") {
		var findings []json.RawMessage
		if json.Unmarshal([]byte(trimmed), &findings) == nil && len(findings) == 0 {
			return `{"findings":[]}`
		}
	}
	return output
}
