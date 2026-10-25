package agentruntime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

// MultimodalAdvice is one explicit, bounded completion. It adds no tools,
// memory, skills, retries, compaction or provider cache operations.
func MultimodalAdvice(ctx context.Context, client *ProviderClient, messages []llm.ChatMessage, attachments []llm.Attachment, authorize func(context.Context) error) (Result, error) {
	if err := validateMultimodalAdmission(ctx, client, messages, attachments, authorize); err != nil {
		return Result{}, err
	}
	ctx = WithContextAuthority(ctx, authorize)
	if err := client.checkPolicy(ctx); err != nil {
		return Result{}, err
	}
	configuration := contextConfig(ctx)
	configuration.Compaction, configuration.LocalCache, configuration.ProviderCache = false, false, false
	ctx = context.WithValue(ctx, contextConfigKey{}, configuration)
	request := snapshotRequest(Request{Messages: messages, attachments: attachments})
	worker, err := newGeminiCompletionAgent(client, request)
	if err != nil {
		return Result{}, err
	}
	result, err := runCompletion(ctx, worker, request, client)
	if err != nil {
		return Result{}, err
	}
	if len(result.Content) > 64<<10 || !utf8.ValidString(result.Content) {
		return Result{}, fmt.Errorf("%w: multimodal answer exceeds the text bound", llm.ErrUnusableResponse)
	}
	if err := authorize(ctx); err != nil {
		return Result{}, err
	}
	return result, nil
}

func validateMultimodalAdmission(ctx context.Context, client *ProviderClient, messages []llm.ChatMessage, attachments []llm.Attachment, authorize func(context.Context) error) error {
	if client == nil || !llm.SupportsAttachments(client.profile) || authorize == nil || BudgetFromContext(ctx) == nil {
		return fmt.Errorf("%w: multimodal advice requires an admitted native profile, authority and budget", ErrPolicyDenied)
	}
	if len(messages) != 2 || messages[0].Role != "system" || messages[1].Role != "user" || len(messages[0].Content)+len(messages[1].Content) > 8<<10 || strings.TrimSpace(messages[1].Content) == "" {
		return fmt.Errorf("invalid bounded multimodal question")
	}
	if err := llm.ValidateAdviceAttachments(attachments); err != nil {
		return fmt.Errorf("%w: %w", llm.ErrRequestRejected, err)
	}
	return nil
}

func cloneAttachments(items []llm.Attachment) []llm.Attachment {
	result := slices.Clone(items)
	for i := range result {
		result[i].Data = slices.Clone(result[i].Data)
	}
	return result
}

type mediaManifestEntry struct {
	Name string `json:"name"`
	MIME string `json:"mime"`
	Hash string `json:"hash"`
	Size int    `json:"size"`
}

func attachmentManifest(items []llm.Attachment) []mediaManifestEntry {
	manifest := make([]mediaManifestEntry, 0, len(items))
	for _, item := range items {
		hash := sha256.Sum256(item.Data)
		manifest = append(manifest, mediaManifestEntry{Name: item.Name, MIME: item.MIME, Hash: hex.EncodeToString(hash[:]), Size: len(item.Data)})
	}
	return manifest
}

func addNativeAttachments(input *model.LLMRequest, items []llm.Attachment) error {
	if len(items) == 0 {
		return nil
	}
	if len(input.Contents) != 1 || input.Contents[0].Role != genai.RoleUser {
		return fmt.Errorf("%w: attachments require a single explicit user question", llm.ErrRequestRejected)
	}
	for _, item := range items {
		input.Contents[0].Parts = append(input.Contents[0].Parts, &genai.Part{InlineData: &genai.Blob{Data: slices.Clone(item.Data), MIMEType: item.MIME, DisplayName: item.Name}})
	}
	return nil
}

func validateNativeAttachment(role string, part *genai.Part) error {
	if role != genai.RoleUser || onlyModelFields(part, "inlineData") != nil || onlyModelFields(part.InlineData, "data", "mimeType", "displayName") != nil {
		return fmt.Errorf("%w: unsupported native attachment part", llm.ErrRequestRejected)
	}
	item := part.InlineData
	if err := (llm.Attachment{Name: item.DisplayName, MIME: item.MIMEType, Data: item.Data}).Validate(); err != nil {
		return fmt.Errorf("%w: %w", llm.ErrRequestRejected, err)
	}
	return nil
}
