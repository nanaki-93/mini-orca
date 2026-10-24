package agentruntime

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
)

func missionPlannerFixture() (MissionPlannerInput, missionDecomposition) {
	limits := agentstate.MissionLimits{ModelCalls: 4, ToolCalls: 8, InputTokens: 4096, OutputTokens: 2048, ElapsedSeconds: 60, ConcurrentCalls: 1, Repairs: 1}
	input := MissionPlannerInput{Goal: "Add a pure helper", Sources: []agentstate.MissionSource{{Path: "helper.go"}}, Limits: limits}
	output := missionDecomposition{Version: MissionPlannerVersion, Status: "planned", Reason: "", Children: []agentstate.MissionChild{{ID: "helper", Title: "Helper", Approach: "Implement a bounded pure helper", AcceptanceCriteria: []string{"Returns the expected value"}, Dependencies: []string{}, WritePaths: []string{"helper.go"}, ReadPaths: []string{}, Limits: limits}}, IntegrationOrder: []string{"helper"}}
	return input, output
}

func TestMissionPlannerSchemaAndDomainValidatorAgreeForACompletePlan(t *testing.T) {
	input, output := missionPlannerFixture()
	request, err := MissionPlannerRequest(input, "Applicable instructions", nil, 8192)
	if err != nil || request.Investigation != nil || request.Stream || request.Schema.Name != "mission_planner_v1" || len(request.Messages) != 1 {
		t.Fatal(request, err)
	}
	schema, err := resolveToolSchema(request.Schema.Schema)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(output)
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(value); err != nil {
		t.Fatal(err)
	}
	plan, err := ParseMissionPlan(Result{Content: string(data), FinishReason: "stop"}, input)
	if err != nil || plan.Goal != input.Goal || plan.Limits != input.Limits || len(plan.Children) != 1 {
		t.Fatal(plan, err)
	}
	input.Sources[0].Path = "changed.go"
	if plan.Sources[0].Path != "helper.go" {
		t.Fatal("parsed plan aliases caller constraints")
	}
	for _, tokens := range []int{0, 1} {
		if _, err := MissionPlannerRequest(input, "context", nil, tokens); err == nil {
			t.Fatal("context ceiling ignored")
		}
	}
	if _, err := MissionPlannerRequest(input, strings.Repeat("x", 128<<10), nil, 128000); err == nil {
		t.Fatal("absolute prompt bound ignored")
	}
}

func TestMissionPlannerRejectsIncompleteAndHostileDecompositions(t *testing.T) {
	for name, alter := range map[string]func(*missionDecomposition){
		"unavailable": func(o *missionDecomposition) {
			o.Status, o.Reason, o.Children, o.IntegrationOrder = "unavailable", "Insufficient context", []agentstate.MissionChild{}, []string{}
		},
		"version":         func(o *missionDecomposition) { o.Version = "future" },
		"empty":           func(o *missionDecomposition) { o.Children = nil },
		"scope":           func(o *missionDecomposition) { o.Children[0].WritePaths[0] = "outside.go" },
		"dependency":      func(o *missionDecomposition) { o.Children[0].Dependencies = []string{"missing"} },
		"self cycle":      func(o *missionDecomposition) { o.Children[0].Dependencies = []string{"helper"} },
		"budget":          func(o *missionDecomposition) { o.Children[0].Limits.ModelCalls++ },
		"criteria":        func(o *missionDecomposition) { o.Children[0].AcceptanceCriteria = nil },
		"partial order":   func(o *missionDecomposition) { o.IntegrationOrder = nil },
		"status mismatch": func(o *missionDecomposition) { o.Reason = "Partial plan" },
	} {
		t.Run(name, func(t *testing.T) {
			input, output := missionPlannerFixture()
			alter(&output)
			data, _ := json.Marshal(output)
			plan, err := ParseMissionPlan(Result{Content: string(data), FinishReason: "stop"}, input)
			if !errors.Is(err, ErrMissionPlanningIncomplete) || len(plan.Children) != 0 {
				t.Fatal("invalid plan became usable", plan, err)
			}
		})
	}
	input, output := missionPlannerFixture()
	data, _ := json.Marshal(output)
	for _, content := range []string{"null", string(data) + " {}", strings.Replace(string(data), `"version":`, `"extra":true,"version":`, 1), strings.Replace(string(data), `"version":`, `"version":"wrong","version":`, 1), strings.Replace(string(data), `"repairs":1`, `"repairs":null`, 1), strings.Repeat(" ", 128<<10) + string(data)} {
		if plan, err := ParseMissionPlan(Result{Content: content}, input); !errors.Is(err, ErrMissionPlanningIncomplete) || len(plan.Children) != 0 {
			t.Fatal("malformed JSON admitted", err)
		}
	}
	for _, reason := range []string{"length", "content_filter", "error", "tool_calls"} {
		if _, err := ParseMissionPlan(Result{Content: string(data), FinishReason: reason}, input); !errors.Is(err, ErrMissionPlanningIncomplete) {
			t.Fatal("abnormal provider completion admitted", reason, err)
		}
	}
	for _, field := range []string{`"reason":"",`, `"read_paths":[],`, `"dependencies":[],`} {
		content := strings.Replace(string(data), field, "", 1)
		if _, err := ParseMissionPlan(Result{Content: content}, input); !errors.Is(err, ErrMissionPlanningIncomplete) {
			t.Fatal("omitted required field admitted", field, err)
		}
	}
}
