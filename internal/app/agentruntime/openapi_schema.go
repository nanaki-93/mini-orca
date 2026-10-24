package agentruntime

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
)

func apiParameters(inherited json.RawMessage, local []APIParameter, path string) ([]APIParameter, error) {
	var common []APIParameter
	if len(inherited) > 0 {
		if err := apiDecode(inherited, &common); err != nil {
			return nil, err
		}
	}
	merged := map[string]APIParameter{}
	for _, group := range [][]APIParameter{common, local} {
		seen := map[string]bool{}
		for _, parameter := range group {
			key := parameter.In + "/" + parameter.Name
			if seen[key] {
				return nil, fmt.Errorf("duplicate OpenAPI parameter")
			}
			seen[key] = true
			if err := validateAPIParameter(parameter); err != nil {
				return nil, err
			}
			merged[key] = parameter
		}
	}
	if len(merged) > 16 {
		return nil, fmt.Errorf("at most 16 parameters are supported")
	}
	result := make([]APIParameter, 0, len(merged))
	for _, key := range slices.Sorted(maps.Keys(merged)) {
		result = append(result, merged[key])
	}
	return result, validateAPIPathParameters(path, result)
}

func validateAPIParameter(parameter APIParameter) error {
	if len(parameter.Name) == 0 || len(parameter.Name) > 64 || strings.ContainsAny(parameter.Name, "{}&=?/#\\\x00\r\n") {
		return fmt.Errorf("invalid OpenAPI parameter name")
	}
	if parameter.In != "path" && parameter.In != "query" {
		return fmt.Errorf("only scalar path and query parameters are supported")
	}
	style := "form"
	if parameter.In == "path" {
		style = "simple"
	}
	if parameter.Style != "" && parameter.Style != style || parameter.In == "path" && !parameter.Required {
		return fmt.Errorf("unsupported parameter serialization or optional path parameter")
	}
	schema, err := apiSchema(parameter.Schema, false)
	if err != nil {
		return err
	}
	var kind struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(schema, &kind); err != nil {
		return err
	}
	if !slices.Contains([]string{"string", "integer", "number", "boolean"}, kind.Type) {
		return fmt.Errorf("path/query parameters must be scalar")
	}
	return nil
}

func validateAPIPathParameters(path string, parameters []APIParameter) error {
	seen := map[string]bool{}
	for _, segment := range strings.Split(path, "/") {
		if !strings.ContainsAny(segment, "{}") {
			continue
		}
		if !strings.HasPrefix(segment, "{") || !strings.HasSuffix(segment, "}") || strings.Count(segment, "{") != 1 || strings.Count(segment, "}") != 1 {
			return fmt.Errorf("path placeholders must occupy complete segments")
		}
		seen[segment[1:len(segment)-1]] = true
	}
	for _, parameter := range parameters {
		if parameter.In == "path" {
			if !seen[parameter.Name] {
				return fmt.Errorf("path parameter has no placeholder")
			}
			delete(seen, parameter.Name)
		}
	}
	if len(seen) != 0 {
		return fmt.Errorf("path placeholder has no parameter")
	}
	return nil
}

func apiInputSchema(parameters []APIParameter, body *apiRequestBody, method string) (json.RawMessage, error) {
	properties := map[string]any{}
	required := []string{}
	for _, location := range []string{"path", "query"} {
		fields, names := map[string]any{}, []string{}
		for _, parameter := range parameters {
			if parameter.In != location {
				continue
			}
			schema, err := apiSchema(parameter.Schema, false)
			if err != nil {
				return nil, err
			}
			fields[parameter.Name] = schema
			if parameter.Required {
				names = append(names, parameter.Name)
			}
		}
		if len(fields) > 0 {
			properties[location] = map[string]any{"type": "object", "properties": fields, "required": names, "additionalProperties": false}
			if len(names) > 0 {
				required = append(required, location)
			}
		}
	}
	if body != nil {
		if method == "get" {
			return nil, fmt.Errorf("GET request bodies are unsupported")
		}
		schema, err := apiJSONMedia(body.Content)
		if err != nil {
			return nil, err
		}
		properties["body"] = schema
		if body.Required {
			required = append(required, "body")
		}
	}
	data, err := json.Marshal(map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false})
	if err != nil {
		return nil, err
	}
	_, err = resolveToolSchema(data)
	return data, err
}

func apiJSONMedia(content map[string]apiMedia) (json.RawMessage, error) {
	media, exists := content["application/json"]
	if !exists || len(content) != 1 {
		return nil, fmt.Errorf("only application/json object bodies are supported")
	}
	return apiSchema(media.Schema, true)
}

func apiResponseSchemas(responses map[string]apiResponse) (map[string]json.RawMessage, error) {
	result := map[string]json.RawMessage{}
	for code, response := range responses {
		status, err := strconv.Atoi(code)
		if err != nil || status < 200 || status > 299 {
			continue
		}
		if status == 204 && len(response.Content) == 0 {
			result[code] = nil
			continue
		}
		schema, err := apiJSONMedia(response.Content)
		if err != nil {
			return nil, err
		}
		result[code] = schema
	}
	if len(result) == 0 || len(result) > 8 {
		return nil, fmt.Errorf("declare one to eight exact successful JSON responses or empty 204")
	}
	return result, nil
}

func apiSchema(raw json.RawMessage, object bool) (json.RawMessage, error) {
	if len(raw) == 0 || len(raw) > maxToolPayload {
		return nil, fmt.Errorf("schema exceeds limits")
	}
	var value map[string]any
	if err := apiDecode(raw, &value); err != nil {
		return nil, err
	}
	if err := normalizeAPISchema(value, 0); err != nil {
		return nil, err
	}
	if object && value["type"] != "object" {
		return nil, fmt.Errorf("request and response bodies must be objects")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	// Match the schema compiler's numeric representation before accepting bounds.
	var numeric any
	if err := json.Unmarshal(data, &numeric); err != nil || !mcpSafeNumbers(numeric) {
		return nil, fmt.Errorf("schema numbers exceed the exact supported range")
	}
	if object {
		if _, err := resolveToolSchema(data); err != nil {
			return nil, err
		}
	}
	return data, nil
}

func normalizeAPISchema(value map[string]any, depth int) error {
	if depth > 12 || value == nil {
		return fmt.Errorf("schema nesting exceeds limits")
	}
	allowed := []string{"type", "title", "description", "properties", "items", "required", "additionalProperties", "enum", "minLength", "maxLength", "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum", "minItems", "maxItems", "minProperties", "maxProperties", "pattern", "multipleOf", "uniqueItems"}
	for key := range value {
		if !slices.Contains(allowed, key) {
			return fmt.Errorf("unsupported OpenAPI schema keyword")
		}
	}
	kind, ok := value["type"].(string)
	if !ok || !slices.Contains([]string{"object", "array", "string", "integer", "number", "boolean"}, kind) {
		return fmt.Errorf("an explicit supported schema type is required")
	}
	if err := normalizeAPISchemaBounds(value); err != nil {
		return err
	}
	if err := normalizeAPIProperties(value, depth); err != nil {
		return err
	}
	if child, exists := value["items"]; exists {
		items, ok := child.(map[string]any)
		if !ok {
			return fmt.Errorf("array items must be an inline schema")
		}
		return normalizeAPISchema(items, depth+1)
	}
	if value["type"] == "array" {
		return fmt.Errorf("array items schema is required")
	}
	return nil
}

func normalizeAPIProperties(value map[string]any, depth int) error {
	if additional, exists := value["additionalProperties"]; exists {
		if _, ok := additional.(bool); !ok {
			return fmt.Errorf("additionalProperties must be a boolean")
		}
	}
	if raw, exists := value["properties"]; exists {
		properties, ok := raw.(map[string]any)
		if !ok || len(properties) > 64 {
			return fmt.Errorf("object properties exceed supported limits")
		}
		for _, raw := range properties {
			child, ok := raw.(map[string]any)
			if !ok {
				return fmt.Errorf("property must be an inline schema")
			}
			if err := normalizeAPISchema(child, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func normalizeAPISchemaBounds(value map[string]any) error {
	for _, bound := range []string{"Minimum", "Maximum"} {
		key := "exclusive" + bound
		if raw, exists := value[key]; exists {
			exclusive, ok := raw.(bool)
			if !ok {
				return fmt.Errorf("OpenAPI 3.0 exclusive bounds must be boolean")
			}
			delete(value, key)
			if !exclusive {
				continue
			}
			inclusive := strings.ToLower(bound)
			number, ok := value[inclusive].(json.Number)
			if !ok {
				return fmt.Errorf("exclusive bound requires a numeric limit")
			}
			value[key] = number
			delete(value, inclusive)
		}
	}
	return nil
}
