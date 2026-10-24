package agentruntime

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
)

// ToolRecord is a bounded, explicit private disclosure, never progress metadata.
type ToolRecord struct {
	EventID, CallID, Name, Kind, Value string
	Truncated                          bool
}

func InspectToolEvents(events []agentstate.Event) ([]ToolRecord, bool, error) {
	truncated := len(events) > 128
	events = events[max(0, len(events)-128):]
	var records []ToolRecord
	for _, stored := range events {
		var event session.Event
		if err := json.Unmarshal(stored.Data, &event); err != nil {
			return nil, false, fmt.Errorf("decode private tool event: %w", err)
		}
		if event.Content == nil {
			continue
		}
		for _, part := range event.Content.Parts {
			items, err := inspectToolPart(stored.ID, part)
			if err != nil {
				return nil, false, err
			}
			records = append(records, items...)
			if len(records) > 16 {
				records = records[len(records)-16:]
				truncated = true
			}
		}
	}
	return records, truncated, nil
}

func inspectToolPart(event string, part *genai.Part) ([]ToolRecord, error) {
	if part == nil {
		return nil, nil
	}
	var records []ToolRecord
	if call := part.FunctionCall; call != nil {
		record, err := inspectToolRecord(event, call.ID, call.Name, "arguments", call.Args)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if response := part.FunctionResponse; response != nil {
		record, err := inspectToolRecord(event, response.ID, response.Name, "result", response.Response)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

func inspectToolRecord(event, id, name, kind string, value any) (ToolRecord, error) {
	// SDK maps contain JSON values. Redaction also bounds recursive nesting.
	nestedCut := false
	data, err := json.Marshal(redactToolValue(value, 0, &nestedCut))
	if err != nil {
		return ToolRecord{}, fmt.Errorf("encode private tool record: %w", err)
	}
	text, cut := privateTextBound(string(data), 2048)
	boundedID, idCut := privateTextBound(id, 128)
	boundedName, nameCut := privateTextBound(name, 128)
	return ToolRecord{EventID: event, CallID: boundedID, Name: boundedName, Kind: kind, Value: text, Truncated: cut || idCut || nameCut || nestedCut}, nil
}

func privateTextBound(value string, limit int) (string, bool) {
	if len(value) <= limit {
		return value, false
	}
	value = value[:limit]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value, true
}

func redactToolValue(value any, depth int, truncated *bool) any {
	if depth >= 24 {
		*truncated = true
		return "[omitted: nesting limit]"
	}
	switch value := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(value))
		for key, item := range value {
			if credentialKey(key) {
				result[key] = "[redacted]"
			} else {
				result[key] = redactToolValue(item, depth+1, truncated)
			}
		}
		return result
	case []any:
		result := make([]any, len(value))
		for i, item := range value {
			result[i] = redactToolValue(item, depth+1, truncated)
		}
		return result
	default:
		return value
	}
}

func credentialKey(key string) bool {
	key = strings.NewReplacer("_", "", "-", "").Replace(strings.ToLower(key))
	switch key {
	case "authorization", "apikey", "password", "passwd", "secret", "token", "accesstoken", "refreshtoken", "privatekey", "clientsecret":
		return true
	default:
		return false
	}
}
