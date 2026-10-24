package agentruntime

import (
	"context"
	"fmt"
	"iter"
	"math"
	"net/url"
	"slices"
	"strings"
	"unicode/utf8"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/tool/geminitool"
	"google.golang.org/genai"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

type GroundSource struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type GroundSupport struct {
	Start      int       `json:"start"`
	End        int       `json:"end"`
	Sources    []int     `json:"sources"`
	Confidence []float32 `json:"confidence,omitempty"`
}

// GroundingInfo preserves provider attribution, not independently verified
// repository facts. Reported queries do not establish billable search usage.
type GroundingInfo struct {
	State            string          `json:"state"`
	Sources          []GroundSource  `json:"sources"`
	Supports         []GroundSupport `json:"supports"`
	Queries          []string        `json:"queries"`
	SuggestionsHTML  string          `json:"suggestions_html,omitempty"`
	SearchUsageKnown bool            `json:"search_usage_known"`
}

// GroundedResearch runs one search-only native ADK invocation. No repository
// tools, ambient memory/skills, compaction, cache creation or retries are added.
func GroundedResearch(ctx context.Context, client *ProviderClient, messages []llm.ChatMessage, authorize func(context.Context) error) (Result, error) {
	if client == nil || !llm.SupportsGoogleSearch(client.profile) || authorize == nil || BudgetFromContext(ctx) == nil {
		return Result{}, fmt.Errorf("%w: admitted Google Search profile and budget are required", ErrPolicyDenied)
	}
	if len(messages) != 2 || messages[0].Role != "system" || messages[1].Role != "user" || len(messages[1].Content) > 64<<10 || strings.TrimSpace(messages[1].Content) == "" {
		return Result{}, fmt.Errorf("invalid bounded research input")
	}
	ctx = WithContextAuthority(ctx, authorize)
	if err := client.checkPolicy(ctx); err != nil {
		return Result{}, err
	}
	if err := BudgetFromContext(ctx).AdmitTool(ctx); err != nil {
		return Result{}, err
	}
	request := snapshotRequest(Request{Messages: messages, grounding: true})
	worker, err := agent.New(agent.Config{Name: "grounded_research", Description: "Explicit external research with retained search attribution", Run: groundingRun(client, request)})
	if err != nil {
		return Result{}, err
	}
	result, err := runCompletion(ctx, worker, request, client)
	if err != nil {
		return Result{}, err
	}
	if err := authorize(ctx); err != nil {
		return Result{}, err
	}
	return result, nil
}

func groundingRun(client *ProviderClient, request Request) func(agent.InvocationContext) iter.Seq2[*session.Event, error] {
	return func(ctx agent.InvocationContext) iter.Seq2[*session.Event, error] {
		return func(yield func(*session.Event, error) bool) {
			input, err := googleCompletionRequest(request)
			if err != nil {
				yield(nil, err)
				return
			}
			if err := (geminitool.GoogleSearch{}).ProcessRequest(agent.NewContext(ctx), input); err != nil {
				yield(nil, err)
				return
			}
			for response, err := range (&googleModel{client: client, grounding: true}).GenerateContent(ctx, input, false) {
				if err != nil {
					yield(nil, err)
					return
				}
				event, err := groundingEvent(ctx, response)
				if err != nil {
					yield(nil, err)
					return
				}
				if !yield(event, nil) {
					return
				}
			}
		}
	}
}

func groundingEvent(ctx agent.InvocationContext, response *model.LLMResponse) (*session.Event, error) {
	if response == nil || response.Partial {
		return nil, fmt.Errorf("research requires one complete response")
	}
	event, err := googleCompletionEvent(ctx, response, false)
	if err != nil {
		return nil, err
	}
	result := event.Output.(Result)
	result.Grounding, err = groundingMetadata(response, result.Content)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid search attribution", llm.ErrUnusableResponse)
	}
	event.Output = result
	return event, nil
}

func googleSearchConfigured(configuration *genai.GenerateContentConfig) bool {
	if configuration == nil {
		return false
	}
	for _, tool := range configuration.Tools {
		if tool != nil && tool.GoogleSearch != nil {
			return true
		}
	}
	return false
}

func validateGoogleTools(profile config.ModelProfile, configuration *genai.GenerateContentConfig) error {
	if !googleSearchConfigured(configuration) {
		_, err := generationTools(configuration.Tools)
		return err
	}
	if !llm.SupportsGoogleSearch(profile) || len(configuration.Tools) != 1 || configuration.ResponseJsonSchema != nil || configuration.ResponseMIMEType != "" {
		return fmt.Errorf("%w: search requires a supported profile and a search-only response", llm.ErrRequestRejected)
	}
	tool := configuration.Tools[0]
	if onlyModelFields(tool, "googleSearch") != nil || onlyModelFields(tool.GoogleSearch) != nil {
		return fmt.Errorf("%w: unsupported Google Search settings", llm.ErrRequestRejected)
	}
	return nil
}

func groundingMetadata(response *model.LLMResponse, text string) (*GroundingInfo, error) {
	info := &GroundingInfo{State: "ungrounded", Sources: []GroundSource{}, Supports: []GroundSupport{}, Queries: []string{}}
	if len(text) > 64<<10 || !utf8.ValidString(text) {
		return nil, fmt.Errorf("research answer exceeds limits")
	}
	metadata := response.GroundingMetadata
	if metadata == nil {
		return info, nil
	}
	if onlyModelFields(metadata, "groundingChunks", "groundingSupports", "searchEntryPoint", "webSearchQueries") != nil || len(metadata.GroundingChunks) > 32 || len(metadata.GroundingSupports) > 64 || len(metadata.WebSearchQueries) > 16 {
		return nil, fmt.Errorf("unsupported search metadata")
	}
	if err := collectGroundingSources(info, metadata); err != nil {
		return nil, err
	}
	if err := collectSearchPresentation(info, metadata); err != nil {
		return nil, err
	}
	for _, support := range metadata.GroundingSupports {
		value, err := groundingSupport(response.Content, support, len(info.Sources))
		if err != nil {
			return nil, err
		}
		info.Supports = append(info.Supports, value)
	}
	if len(info.Sources) > 0 {
		info.State = "uncited"
	}
	if len(info.Supports) > 0 {
		info.State = "grounded"
	}
	return info, nil
}

func collectGroundingSources(info *GroundingInfo, metadata *genai.GroundingMetadata) error {
	for _, chunk := range metadata.GroundingChunks {
		if chunk == nil || chunk.Web == nil || onlyModelFields(chunk, "web") != nil || !validGroundURL(chunk.Web.URI) || len(chunk.Web.Title) > 512 || !utf8.ValidString(chunk.Web.Title) {
			return fmt.Errorf("invalid external citation")
		}
		info.Sources = append(info.Sources, GroundSource{Title: chunk.Web.Title, URL: chunk.Web.URI})
	}
	return nil
}

func collectSearchPresentation(info *GroundingInfo, metadata *genai.GroundingMetadata) error {
	for _, query := range metadata.WebSearchQueries {
		if len(query) > 2048 || !utf8.ValidString(query) {
			return fmt.Errorf("invalid reported search query")
		}
		info.Queries = append(info.Queries, query)
	}
	if metadata.SearchEntryPoint == nil {
		return nil
	}
	entry := metadata.SearchEntryPoint
	if len(entry.RenderedContent) > 64<<10 || !utf8.ValidString(entry.RenderedContent) {
		return fmt.Errorf("search suggestions exceed limits")
	}
	if len(entry.SDKBlob) > 0 && entry.RenderedContent == "" {
		return fmt.Errorf("search suggestions require a displayable HTML fragment")
	}
	info.SuggestionsHTML = entry.RenderedContent
	return nil
}

func validGroundURL(value string) bool {
	u, err := url.Parse(value)
	return err == nil && len(value) <= 4096 && u.Hostname() != "" && u.User == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Opaque == "" && !strings.ContainsAny(value, "\x00\r\n")
}

func groundingSupport(content *genai.Content, support *genai.GroundingSupport, sourceCount int) (GroundSupport, error) {
	var result GroundSupport
	if content == nil || support == nil || support.Segment == nil || len(support.RenderedParts) > 0 || len(support.GroundingChunkIndices) == 0 || len(support.GroundingChunkIndices) > 32 {
		return result, fmt.Errorf("invalid citation support")
	}
	var err error
	result.Start, result.End, err = groundingRange(content, support.Segment)
	if err != nil {
		return result, err
	}
	for _, index := range support.GroundingChunkIndices {
		if index < 0 || int(index) >= sourceCount || slices.Contains(result.Sources, int(index)) {
			return result, fmt.Errorf("citation source does not exist or is duplicated")
		}
		result.Sources = append(result.Sources, int(index))
	}
	if err := validateGroundConfidence(support.ConfidenceScores, len(result.Sources)); err != nil {
		return result, err
	}
	result.Confidence = slices.Clone(support.ConfidenceScores)
	return result, nil
}

func groundingRange(content *genai.Content, segment *genai.Segment) (int, int, error) {
	if segment.PartIndex < 0 || int(segment.PartIndex) >= len(content.Parts) {
		return 0, 0, fmt.Errorf("citation part does not exist")
	}
	part := content.Parts[segment.PartIndex]
	if part == nil || part.Thought || segment.StartIndex < 0 || segment.EndIndex <= segment.StartIndex || int(segment.EndIndex) > len(part.Text) {
		return 0, 0, fmt.Errorf("citation range is invalid")
	}
	quoted := part.Text[segment.StartIndex:segment.EndIndex]
	if !utf8.ValidString(quoted) || segment.Text != "" && segment.Text != quoted {
		return 0, 0, fmt.Errorf("citation text differs from its range")
	}
	offset := 0
	for _, previous := range content.Parts[:segment.PartIndex] {
		if previous != nil && !previous.Thought {
			offset += len(previous.Text)
		}
	}
	return offset + int(segment.StartIndex), offset + int(segment.EndIndex), nil
}

func validateGroundConfidence(scores []float32, count int) error {
	if len(scores) != 0 && len(scores) != count {
		return fmt.Errorf("citation confidence count differs")
	}
	for _, confidence := range scores {
		if math.IsNaN(float64(confidence)) || confidence < 0 || confidence > 1 {
			return fmt.Errorf("invalid provider confidence")
		}
	}
	return nil
}
