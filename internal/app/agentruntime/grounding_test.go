package agentruntime

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

const groundedFixtureResponse = `{"modelVersion":"gemini-2.5-flash","candidates":[{"finishReason":"STOP","content":{"role":"model","parts":[{"text":"Café fact."}]},"groundingMetadata":{"webSearchQueries":["fixture question"],"searchEntryPoint":{"renderedContent":"<a href=\"https://www.google.com/search?q=fixture\">Search</a>"},"groundingChunks":[{"web":{"uri":"https://source.example/report","title":"Source"}}],"groundingSupports":[{"segment":{"startIndex":0,"endIndex":5,"text":"Café"},"groundingChunkIndices":[0]}]}}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":3,"totalTokenCount":8}}`

func TestGroundingUsesNativeSearchOnlyAndRetainsAttribution(t *testing.T) {
	var calls atomic.Int32
	client := googleFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"googleSearch":{}`) || strings.Contains(string(body), "functionDeclarations") || strings.Contains(string(body), "responseJsonSchema") || strings.Contains(r.URL.Path, "cachedContent") {
			t.Errorf("wrong grounded request: %s %s", r.URL.Path, body)
		}
		io.WriteString(w, groundedFixtureResponse)
	})
	client.profile.Model = "gemini-2.5-flash"
	ctx, budget := budgetContext(t, nil)
	result, err := GroundedResearch(ctx, client, googleRequest().Messages, func(ctx context.Context) error { return context.Cause(ctx) })
	if err != nil || result.Content != "Café fact." || result.Grounding == nil {
		t.Fatalf("grounded result: %+v %v", result, err)
	}
	metadata := result.Grounding
	if metadata.State != "grounded" || len(metadata.Sources) != 1 || len(metadata.Supports) != 1 || metadata.Supports[0].End != 5 || metadata.SearchUsageKnown || !strings.Contains(metadata.SuggestionsHTML, "google.com") || calls.Load() != 1 || budget.Snapshot().ModelCalls != 1 || budget.Snapshot().ToolCalls != 1 {
		t.Fatalf("attribution or budget lost: %+v %+v", metadata, budget.Snapshot())
	}
}

func TestGroundingRejectsMissingAuthorityUnsupportedProfilesAndLateChanges(t *testing.T) {
	var calls atomic.Int32
	client := googleFixture(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); io.WriteString(w, groundedFixtureResponse) })
	ctx, _ := budgetContext(t, nil)
	authorize := func(ctx context.Context) error { return context.Cause(ctx) }
	if _, err := GroundedResearch(ctx, client, googleRequest().Messages, authorize); !errors.Is(err, ErrPolicyDenied) || calls.Load() != 0 {
		t.Fatal("unsupported profile searched")
	}
	client.profile.Model = "gemini-2.5-flash"
	if _, err := GroundedResearch(ctx, client, googleRequest().Messages, nil); !errors.Is(err, ErrPolicyDenied) || calls.Load() != 0 {
		t.Fatal("missing source authority searched")
	}
	if _, err := GroundedResearch(context.Background(), client, googleRequest().Messages, authorize); !errors.Is(err, ErrPolicyDenied) {
		t.Fatal("missing budget accepted")
	}
	changed := errors.New("captured source changed")
	_, err := GroundedResearch(ctx, client, googleRequest().Messages, func(ctx context.Context) error {
		if calls.Load() > 0 {
			return changed
		}
		return context.Cause(ctx)
	})
	if !errors.Is(err, changed) || calls.Load() != 1 {
		t.Fatalf("late source change published: %v calls=%d", err, calls.Load())
	}
}

func TestGroundingMalformedMetadataAndUngroundedResponses(t *testing.T) {
	for name, alter := range map[string]func(*genai.GroundingMetadata){
		"URL":        func(m *genai.GroundingMetadata) { m.GroundingChunks[0].Web.URI = "javascript:alert(1)" },
		"index":      func(m *genai.GroundingMetadata) { m.GroundingSupports[0].GroundingChunkIndices = []int32{4} },
		"range":      func(m *genai.GroundingMetadata) { m.GroundingSupports[0].Segment.EndIndex = 400 },
		"UTF8":       func(m *genai.GroundingMetadata) { m.GroundingSupports[0].Segment.EndIndex = 4 },
		"quote":      func(m *genai.GroundingMetadata) { m.GroundingSupports[0].Segment.Text = "different" },
		"part":       func(m *genai.GroundingMetadata) { m.GroundingSupports[0].Segment.PartIndex = 2 },
		"confidence": func(m *genai.GroundingMetadata) { m.GroundingSupports[0].ConfidenceScores = []float32{2} },
		"html size":  func(m *genai.GroundingMetadata) { m.SearchEntryPoint.RenderedContent = strings.Repeat("x", (64<<10)+1) },
	} {
		t.Run(name, func(t *testing.T) {
			var response genai.GenerateContentResponse
			if err := json.Unmarshal([]byte(groundedFixtureResponse), &response); err != nil {
				t.Fatal(err)
			}
			candidate := response.Candidates[0]
			alter(candidate.GroundingMetadata)
			if _, err := groundingMetadata(&model.LLMResponse{Content: candidate.Content, GroundingMetadata: candidate.GroundingMetadata}, "Café fact."); err == nil {
				t.Fatal("invalid attribution accepted")
			}
		})
	}
	info, err := groundingMetadata(&model.LLMResponse{Content: genai.NewContentFromText("No search needed", genai.RoleModel)}, "No search needed")
	if err != nil || info.State != "ungrounded" || info.SearchUsageKnown {
		t.Fatalf("ungrounded answer mislabelled: %+v %v", info, err)
	}
	if err := validateGoogleTools(config.ModelProfile{Provider: config.GeminiProvider, Model: "gemini-2.5-flash"}, &genai.GenerateContentConfig{Tools: []*genai.Tool{{GoogleSearch: &genai.GoogleSearch{}}, {FunctionDeclarations: []*genai.FunctionDeclaration{{Name: "write"}}}}}); err == nil {
		t.Fatal("search/function mixture accepted")
	}
}

func TestGroundingMalformedNativeResponseDoesNotRetry(t *testing.T) {
	var calls atomic.Int32
	client := googleFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		io.WriteString(w, strings.Replace(groundedFixtureResponse, `"groundingChunkIndices":[0]`, `"groundingChunkIndices":[99]`, 1))
	})
	client.profile.Model = "gemini-2.5-flash"
	ctx, _ := budgetContext(t, nil)
	result, err := GroundedResearch(ctx, client, []llm.ChatMessage{{Role: "system", Content: "policy"}, {Role: "user", Content: "question"}}, func(ctx context.Context) error { return context.Cause(ctx) })
	if err == nil || result != (Result{}) || calls.Load() != 1 {
		t.Fatalf("invalid response: %+v %v calls=%d", result, err, calls.Load())
	}
}
