package agentruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"
)

func missionGraphFixture() MissionGraphInput {
	return MissionGraphInput{ProjectID: "project", ProjectRevision: "revision", ExecutionID: "execution", WaveID: "wave", PlanHash: "approved", Children: []string{"first", "second", "third"}, Concurrency: 2}
}

func missionNodeFixture(input MissionGraphInput, id string) MissionNodeResult {
	return MissionNodeResult{ExecutionID: input.ExecutionID, WaveID: input.WaveID, ChildID: id, State: "awaiting_review", SessionID: "session-" + id, WorkflowID: "workflow-" + id, ProposalHash: "proposal-" + id, ProposalRevision: 3}
}

func TestMissionGraphPersistsAdmissionBoundsParallelismAndKeepsReviewOutcomes(t *testing.T) {
	ctx, budget, store := persistedContext(t, t.TempDir())
	input := missionGraphFixture()
	entered, release := make(chan string, 3), make(chan struct{}, 3)
	finished := make(chan MissionGraphResult, 1)
	failure := make(chan error, 1)
	go func() {
		result, err := RunMissionGraph(ctx, input, func(ctx context.Context, id string) (MissionNodeResult, error) {
			sessions, err := store.Sessions(ctx, "mini-orca", "local")
			if err != nil || len(sessions) != 1 || sessions[0].State["mini_orca.mission_graph"] == nil {
				return MissionNodeResult{}, fmt.Errorf("child ran before durable admission: %v", err)
			}
			found := false
			index := 0
			for i, value := range input.Children {
				if value == id {
					index = i
				}
			}
			for _, entry := range sessions[0].Events {
				var event struct{ CustomMetadata map[string]any }
				if err := json.Unmarshal(entry.Data, &event); err != nil {
					return MissionNodeResult{}, err
				}
				found = found || event.CustomMetadata["mini_orca.mission_node"] == missionNodeName(index)
			}
			if !found {
				return MissionNodeResult{}, fmt.Errorf("child start was not durable")
			}
			entered <- id
			select {
			case <-release:
			case <-ctx.Done():
				return MissionNodeResult{}, context.Cause(ctx)
			}
			result := missionNodeFixture(input, id)
			if id == "second" {
				result.State = "rejected"
			}
			return result, nil
		})
		finished <- result
		failure <- err
	}()
	for range 2 {
		<-entered
	}
	select {
	case id := <-entered:
		t.Fatalf("too many concurrent children: %s", id)
	default:
	}
	release <- struct{}{}
	<-entered
	release <- struct{}{}
	release <- struct{}{}
	result, err := <-finished, <-failure
	if err != nil || len(result.Children) != 3 || result.Children[1].State != "rejected" {
		t.Fatal(result, err)
	}
	var ids []string
	for _, result := range result.Children {
		ids = append(ids, result.ChildID)
	}
	if !reflect.DeepEqual(ids, input.Children) {
		t.Fatal("completion timing changed approved order", ids)
	}
	if usage := budget.Snapshot(); usage.ModelCalls != 0 || usage.ToolCalls != 0 {
		t.Fatal("orchestrator fabricated child usage", usage)
	}
}

func TestMissionGraphFailureAndCancellationDrainOwnedChildren(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		t.Run(fmt.Sprintf("cancel=%t", canceled), func(t *testing.T) {
			base, _ := budgetContext(t, nil)
			ctx, cancel := context.WithCancel(base)
			defer cancel()
			input := missionGraphFixture()
			input.Concurrency = 3
			entered := make(chan struct{}, 3)
			release := make(chan struct{})
			done := make(chan error, 1)
			var exits atomic.Int32
			sentinel := errors.New("owned worktree failure")
			go func() {
				result, err := RunMissionGraph(ctx, input, func(ctx context.Context, id string) (MissionNodeResult, error) {
					defer exits.Add(1)
					entered <- struct{}{}
					if id == "first" && !canceled {
						<-release
						return MissionNodeResult{}, sentinel
					}
					<-ctx.Done()
					return MissionNodeResult{}, context.Cause(ctx)
				})
				if len(result.Children) != 0 {
					err = fmt.Errorf("failed wave published completion")
				}
				done <- err
			}()
			for range 3 {
				<-entered
			}
			if canceled {
				cancel()
			} else {
				close(release)
			}
			err := <-done
			want := sentinel
			if canceled {
				want = context.Canceled
			}
			if !errors.Is(err, want) || exits.Load() != 3 {
				t.Fatal("failed wave did not drain children", err, exits.Load())
			}
		})
	}
}

func TestMissionGraphRejectsChangedIdentityAndMalformedAdmissions(t *testing.T) {
	for _, field := range []string{"execution", "wave", "child", "state", "proposal"} {
		t.Run(field, func(t *testing.T) {
			ctx, _ := budgetContext(t, nil)
			input := missionGraphFixture()
			input.Children, input.Concurrency = input.Children[:1], 1
			calls := 0
			result, err := RunMissionGraph(ctx, input, func(_ context.Context, id string) (MissionNodeResult, error) {
				calls++
				result := missionNodeFixture(input, id)
				switch field {
				case "execution":
					result.ExecutionID = "another"
				case "wave":
					result.WaveID = "another"
				case "child":
					result.ChildID = "another"
				case "state":
					result.State = "applied"
				case "proposal":
					result.ProposalHash = ""
				}
				return result, nil
			})
			if err == nil || calls != 1 || len(result.Children) != 0 {
				t.Fatal("invalid child outcome retried or trusted", result, calls, err)
			}
		})
	}
	for name, mutate := range map[string]func(*MissionGraphInput){
		"empty":            func(v *MissionGraphInput) { v.Children = nil },
		"duplicate":        func(v *MissionGraphInput) { v.Children[1] = v.Children[0] },
		"invalid ID":       func(v *MissionGraphInput) { v.Children[0] = "../child" },
		"unbounded":        func(v *MissionGraphInput) { v.Concurrency = 32 },
		"missing identity": func(v *MissionGraphInput) { v.WaveID = "" },
	} {
		t.Run(name, func(t *testing.T) {
			ctx, _ := budgetContext(t, nil)
			input := missionGraphFixture()
			mutate(&input)
			_, err := RunMissionGraph(ctx, input, func(context.Context, string) (MissionNodeResult, error) {
				t.Error("invalid graph dispatched")
				return MissionNodeResult{}, nil
			})
			if err == nil {
				t.Fatal("invalid graph admitted")
			}
		})
	}
}
