package app

import (
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
	"errors"
	"fmt"
)

// Compatibility decisions follow strict JSON validation. A duplicate header
// must never hide corrupt data behind a legacy-schema exception.
func decodeAgentHistory(data []byte, value any) error {
	if !jsontext.Value(data).IsValid() {
		return fmt.Errorf("history must contain one unambiguous JSON value")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(value)
}

var errNewerTask = errors.New("task file was written by a newer Mini-Orca")

func decodeCompatibleChangeHistory(data []byte, id string) (*ChangeSession, error) {
	var session ChangeSession
	if err := decodeAgentHistory(data, &session); err != nil {
		return nil, fmt.Errorf("decode change history: %w", err)
	}
	if session.SchemaVersion > 1 {
		return nil, errNewerTask
	}
	if session.SchemaVersion != 1 || session.ID != id || len(session.Targets) == 0 || len(session.Targets) > maxChangePaths || len(session.Messages) > maxChangeMessages {
		return nil, fmt.Errorf("unsupported or corrupt change history")
	}
	if err := validateStoredChange(session); err != nil {
		return nil, err
	}
	return &session, nil
}

func decodeCompatibleAnalysisHistory(data []byte) (*AnalysisRun, error) {
	if !jsontext.Value(data).IsValid() {
		return nil, errAnalysisRunCorrupt
	}
	// Retired single-stage jobs are archived bytes, never restored authority.
	// Keep this deliberate behavior without fabricating a unified run or graph.
	var header struct {
		SchemaVersion string `json:"schema_version"`
		Plan          struct {
			RetiredStage string `json:"compatibility_stage"`
		} `json:"plan"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return nil, errAnalysisRunCorrupt
	}
	if header.SchemaVersion == AnalysisRunSchemaVersion && (header.Plan.RetiredStage == "semantic" || header.Plan.RetiredStage == "performance") {
		return nil, nil
	}
	var run AnalysisRun
	if decodeAgentHistory(data, &run) != nil || validateStoredAnalysisRun(&run) != nil {
		return nil, errAnalysisRunCorrupt
	}
	// Original schema-1 totals included ineligible stages. Validate the stored
	// aggregate before projecting today's eligible coverage; retain provenance.
	normalizeAnalysisRunEligibility(&run)
	return &run, nil
}
