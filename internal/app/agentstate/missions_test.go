package agentstate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
)

func missionFixture() MissionRecord {
	limits := MissionLimits{ModelCalls: 4, ToolCalls: 8, InputTokens: 4096, OutputTokens: 2048, ElapsedSeconds: 60, ConcurrentCalls: 1, Repairs: 1}
	child := MissionChild{ID: "create", Title: "Create module", Approach: "Implement the bounded module.", AcceptanceCriteria: []string{"Returns the expected value"}, Dependencies: []string{}, WritePaths: []string{"new.go"}, ReadPaths: []string{}, Limits: limits}
	consumer := child
	consumer.ID, consumer.Title, consumer.Dependencies = "consume", "Use module", []string{"create"}
	consumer.WritePaths, consumer.ReadPaths = []string{"main.go"}, []string{"new.go"}
	budget := limits
	budget.ModelCalls, budget.ToolCalls, budget.InputTokens, budget.OutputTokens, budget.Repairs = 8, 16, 8192, 4096, 2
	hash := "sha256:" + strings.Repeat("a", 64)
	return MissionRecord{ID: "mission", ProjectID: "project", ProjectRevision: "revision", Policy: "policy", Configuration: "configuration", Instructions: map[string]string{"new.go": hash, "main.go": hash},
		Plan: MissionPlan{Goal: "Implement a composed change", Sources: []MissionSource{{Path: "new.go"}, {Path: "main.go", Hash: hash, Exists: true}}, Limits: budget, Children: []MissionChild{child, consumer}, IntegrationOrder: []string{"create", "consume"}}}
}

func TestMissionGraphRejectsUnsafeScopeDependenciesAndBudgets(t *testing.T) {
	for name, mutate := range map[string]func(*MissionPlan){
		"empty":                 func(p *MissionPlan) { p.Children = nil },
		"too many":              func(p *MissionPlan) { p.Children = append(p.Children, make([]MissionChild, 15)...) },
		"goal":                  func(p *MissionPlan) { p.Goal = "bad\x00text" },
		"title":                 func(p *MissionPlan) { p.Children[0].Title = "\x1b[0m" },
		"missing approach":      func(p *MissionPlan) { p.Children[0].Approach = "" },
		"criteria":              func(p *MissionPlan) { p.Children[0].AcceptanceCriteria = nil },
		"criterion bound":       func(p *MissionPlan) { p.Children[0].AcceptanceCriteria[0] = strings.Repeat("x", 513) },
		"missing child":         func(p *MissionPlan) { p.Children[1].Dependencies[0] = "unknown" },
		"self":                  func(p *MissionPlan) { p.Children[0].Dependencies = []string{"create"} },
		"cycle":                 func(p *MissionPlan) { p.Children[0].Dependencies = []string{"consume"} },
		"duplicate dependency":  func(p *MissionPlan) { p.Children[1].Dependencies = []string{"create", "create"} },
		"duplicate child":       func(p *MissionPlan) { p.Children[1].ID = "create" },
		"duplicate order":       func(p *MissionPlan) { p.IntegrationOrder[1] = "create" },
		"missing order":         func(p *MissionPlan) { p.IntegrationOrder = p.IntegrationOrder[:1] },
		"reverse order":         func(p *MissionPlan) { slices.Reverse(p.IntegrationOrder) },
		"unreachable read":      func(p *MissionPlan) { p.Children[0].WritePaths = []string{"main.go"} },
		"concurrent read-write": func(p *MissionPlan) { p.Children[1].Dependencies = nil },
		"concurrent write-write": func(p *MissionPlan) {
			p.Children[1].Dependencies = nil
			p.Children[1].ReadPaths = nil
			p.Children[1].WritePaths = []string{"new.go"}
		},
		"duplicate access":   func(p *MissionPlan) { p.Children[0].ReadPaths = []string{"new.go"} },
		"uncaptured":         func(p *MissionPlan) { p.Children[0].WritePaths = []string{"outside.go"} },
		"no writer":          func(p *MissionPlan) { p.Children[0].WritePaths = nil },
		"too many writers":   func(p *MissionPlan) { p.Children[0].WritePaths = make([]string, 9) },
		"too many readers":   func(p *MissionPlan) { p.Children[0].ReadPaths = make([]string, 9) },
		"duplicate source":   func(p *MissionPlan) { p.Sources = append(p.Sources, p.Sources[0]) },
		"parent source":      func(p *MissionPlan) { p.Sources = append(p.Sources, MissionSource{Path: "new.go/child.go"}) },
		"path escape":        func(p *MissionPlan) { p.Sources[0].Path = "../outside.go" },
		"absent hash":        func(p *MissionPlan) { p.Sources[0].Hash = p.Sources[1].Hash },
		"invalid hash":       func(p *MissionPlan) { p.Sources[1].Hash = "wrong" },
		"source limit":       func(p *MissionPlan) { p.Sources = make([]MissionSource, 65) },
		"cumulative calls":   func(p *MissionPlan) { p.Limits.ModelCalls = 7 },
		"cumulative tools":   func(p *MissionPlan) { p.Limits.ToolCalls = 15 },
		"cumulative input":   func(p *MissionPlan) { p.Limits.InputTokens = 8191 },
		"cumulative output":  func(p *MissionPlan) { p.Limits.OutputTokens = 4095 },
		"cumulative repairs": func(p *MissionPlan) { p.Limits.Repairs = 1 },
		"child time":         func(p *MissionPlan) { p.Children[0].Limits.ElapsedSeconds = 61 },
		"child concurrent":   func(p *MissionPlan) { p.Children[0].Limits.ConcurrentCalls = 2 },
	} {
		t.Run(name, func(t *testing.T) {
			plan := missionFixture().Plan
			mutate(&plan)
			if err := plan.Validate(); err == nil {
				t.Fatal("unsafe mission accepted")
			}
		})
	}
	plan := missionFixture().Plan
	if err := plan.Validate(); err != nil {
		t.Fatal(err)
	}
	// An unrelated child may be integrated earlier; integration order alone
	// does not give it dependency authority to read a future creation.
	plan.Children[1].Dependencies, plan.Children[1].ReadPaths = []string{}, []string{}
	if err := plan.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestMissionCanCaptureSixteenDisjointChildrenAndSixtyFourPaths(t *testing.T) {
	plan := missionFixture().Plan
	plan.Children, plan.IntegrationOrder, plan.Sources = nil, nil, nil
	plan.Limits.ModelCalls, plan.Limits.ToolCalls = 16, 16
	plan.Limits.InputTokens, plan.Limits.OutputTokens = 1600, 1600
	for i := range 16 {
		id := fmt.Sprintf("child-%d", i)
		paths := []string{}
		for j := range 4 {
			path := fmt.Sprintf("child%d/file%d.go", i, j)
			plan.Sources = append(plan.Sources, MissionSource{Path: path})
			paths = append(paths, path)
		}
		plan.Children = append(plan.Children, MissionChild{ID: id, Title: id, Approach: "Implement module", AcceptanceCriteria: []string{"Compiles"}, WritePaths: paths, Limits: MissionLimits{ModelCalls: 1, ToolCalls: 1, InputTokens: 100, OutputTokens: 100, ElapsedSeconds: 60, ConcurrentCalls: 1}})
		plan.IntegrationOrder = append(plan.IntegrationOrder, id)
	}
	if err := plan.Validate(); err != nil {
		t.Fatal(err)
	}
}

func saveMissionFixture(t *testing.T, store *Store) MissionSnapshot {
	t.Helper()
	result, err := store.SaveMission(t.Context(), missionFixture(), 0, "", allowMemoryWrite)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestMissionReviewIsExactDurableAndClearedOnRevision(t *testing.T) {
	store := memoryStore(t)
	saved := saveMissionFixture(t, store)
	if saved.CatalogRevision != 1 || saved.Mission.Approved || len(saved.Mission.Reviewed) != 0 {
		t.Fatalf("new plan: %+v", saved)
	}
	if _, err := store.ReviewMission(t.Context(), "project", "mission", 1, saved.Mission.PlanHash, "", allowMemoryWrite); err == nil {
		t.Fatal("approved without child review")
	}
	for _, child := range saved.Mission.Plan.IntegrationOrder {
		result, err := store.ReviewMission(t.Context(), "project", "mission", saved.CatalogRevision, saved.Mission.PlanHash, child, allowMemoryWrite)
		if err != nil {
			t.Fatal(err)
		}
		saved = result
	}
	approved, err := store.ReviewMission(t.Context(), "project", "mission", saved.CatalogRevision, saved.Mission.PlanHash, "", allowMemoryWrite)
	if err != nil || !approved.Mission.Approved || approved.CatalogRevision != 4 {
		t.Fatalf("approval: %+v %v", approved, err)
	}
	for _, test := range []struct {
		revision int64
		hash     string
	}{{3, approved.Mission.PlanHash}, {4, strings.Repeat("b", 64)}} {
		if _, err := store.ReviewMission(t.Context(), "project", "mission", test.revision, test.hash, "", allowMemoryWrite); !errors.Is(err, ErrMissionConflict) {
			t.Fatalf("stale review: %v", err)
		}
	}
	approved.Mission.Plan.Children[0].Approach = "Revised approach"
	revised, err := store.SaveMission(t.Context(), approved.Mission, 4, approved.Mission.PlanHash, allowMemoryWrite)
	if err != nil || revised.Mission.Approved || len(revised.Mission.Reviewed) != 0 || revised.Mission.PlanHash == approved.Mission.PlanHash {
		t.Fatalf("revision: %+v %v", revised, err)
	}
	reopened, err := Open(store.root)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := reopened.Missions(t.Context(), "project")
	if err != nil || catalog.Revision != 5 || len(catalog.Missions) != 1 || catalog.Missions[0].Plan.Children[0].Approach != "Revised approach" {
		t.Fatalf("persistence: %+v %v", catalog, err)
	}
	other, err := reopened.Missions(t.Context(), "foreign")
	if err != nil || len(other.Missions) != 0 {
		t.Fatal("cross-project disclosure", err)
	}
}

func TestMissionConcurrentEditorsAndOwnedInput(t *testing.T) {
	store := memoryStore(t)
	saved := saveMissionFixture(t, store)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	start := make(chan struct{})
	for _, child := range []string{"create", "consume"} {
		wg.Go(func() {
			<-start
			_, err := store.ReviewMission(t.Context(), "project", "mission", 1, saved.Mission.PlanHash, child, allowMemoryWrite)
			results <- err
		})
	}
	close(start)
	wg.Wait()
	close(results)
	counts := map[bool]int{}
	for err := range results {
		if err != nil && !errors.Is(err, ErrMissionConflict) {
			t.Fatal(err)
		}
		counts[err == nil]++
	}
	if counts[true] != 1 || counts[false] != 1 {
		t.Fatalf("concurrent edits: %v", counts)
	}
	record := missionFixture()
	record.ID = "owned"
	snapshot, err := store.SaveMission(t.Context(), record, 2, "", func() error {
		record.Plan.Children[0].Title = "mutated caller"
		record.Instructions["new.go"] = "changed"
		return nil
	})
	if err != nil || snapshot.Mission.Plan.Children[0].Title == "mutated caller" {
		t.Fatalf("caller retained publication ownership: %+v %v", snapshot, err)
	}
}

func TestMissionPublicationFailurePreservesPreviousCatalog(t *testing.T) {
	for _, failure := range []string{"write", "authority", "canceled", "canceled during authorization"} {
		t.Run(failure, func(t *testing.T) {
			store := memoryStore(t)
			saved := saveMissionFixture(t, store)
			location, _ := store.path("missions.json")
			before, err := os.ReadFile(location)
			if err != nil {
				t.Fatal(err)
			}
			original := store.write
			blocked := errors.New("publication revoked")
			denied := false
			cancelDuringAuthorization := false
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			store.write = func(path string, data []byte, mode os.FileMode, authorize func() error) error {
				switch failure {
				case "write":
					return errors.New("disk full")
				case "authority":
					denied = true
				case "canceled":
					cancel()
				case "canceled during authorization":
					cancelDuringAuthorization = true
				}
				return original(path, data, mode, authorize)
			}
			_, err = store.ReviewMission(ctx, "project", "mission", 1, saved.Mission.PlanHash, "create", func() error {
				if denied {
					return blocked
				}
				if cancelDuringAuthorization {
					cancel()
				}
				return nil
			})
			if err == nil {
				t.Fatal("failed publication reported success")
			}
			after, readErr := os.ReadFile(location)
			if readErr != nil || string(before) != string(after) {
				t.Fatal("failed publication changed catalog", readErr)
			}
		})
	}
}

func TestMissionCorruptionUnknownSchemaAndPrivacyArePreserved(t *testing.T) {
	for _, corrupt := range []string{"truncated", "schema", "unknown field", "hash", "workspace", "approval", "permissions", "symlink"} {
		t.Run(corrupt, func(t *testing.T) {
			store := memoryStore(t)
			saveMissionFixture(t, store)
			location, _ := store.path("missions.json")
			info, err := os.Stat(location)
			if err != nil || info.Mode().Perm() != 0600 {
				t.Fatal("mission is not private", err)
			}
			data, err := os.ReadFile(location)
			if err != nil {
				t.Fatal(err)
			}
			var value map[string]any
			if err := json.Unmarshal(data, &value); err != nil {
				t.Fatal(err)
			}
			switch corrupt {
			case "schema":
				value["version"] = 999
			case "unknown field":
				value["extra"] = true
			case "workspace":
				value["workspace"] = "other"
			case "hash":
				value["records"].(map[string]any)["mission"].(map[string]any)["plan_hash"] = strings.Repeat("b", 64)
			case "approval":
				value["records"].(map[string]any)["mission"].(map[string]any)["approved"] = true
			}
			data, err = json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if corrupt == "truncated" {
				data = data[:len(data)-2]
			}
			if err := os.WriteFile(location, data, 0600); err != nil {
				t.Fatal(err)
			}
			if corrupt == "permissions" {
				if err := os.Chmod(location, 0644); err != nil {
					t.Fatal(err)
				}
			}
			if corrupt == "symlink" {
				if err := os.Rename(location, location+".original"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(location+".original", location); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := store.Missions(t.Context(), "project"); err == nil {
				t.Fatal("corrupt catalog read as valid")
			}
			if _, err := store.SaveMission(t.Context(), missionFixture(), 1, "", allowMemoryWrite); err == nil {
				t.Fatal("corrupt catalog overwritten")
			}
			after, err := os.ReadFile(location)
			if err != nil || string(after) != string(data) {
				t.Fatal("corrupt evidence changed", err)
			}
		})
	}
}

func TestMissionLimitsRequireEveryExplicitValue(t *testing.T) {
	data, err := json.Marshal(missionFixture().Plan.Limits)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	for name, raw := range fields {
		for _, replacement := range []json.RawMessage{nil, json.RawMessage("null")} {
			if replacement == nil {
				delete(fields, name)
			} else {
				fields[name] = replacement
			}
			data, _ := json.Marshal(fields)
			var limits MissionLimits
			if json.Unmarshal(data, &limits) == nil {
				t.Fatalf("missing/null %s accepted", name)
			}
		}
		fields[name] = raw
	}
}
