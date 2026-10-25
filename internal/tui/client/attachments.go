package client

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

func (c *Client) ReadAttachments(ctx context.Context, id string, scope AgentSettingsScope) (*AttachmentOverview, error) {
	var result AttachmentOverview
	if err := c.externalJSON(ctx, id, scope, "attachments", "", "", nil, &result); err != nil {
		return nil, err
	}
	if result.ProjectIdentity != scope.ProjectIdentity || !libraryHash.MatchString(result.Configuration) || result.SettingsVersion < 0 || len(result.Attachments) > 128 || !slices.Contains([]string{"available", "unsupported_profile", "stale_settings"}, result.State) {
		return nil, fmt.Errorf("attachment availability belongs to another checkout or is invalid")
	}
	seen := map[string]bool{}
	if result.State == "available" && (!result.Model.Capabilities.Attachments || result.Model.Model == "" || result.Model.ProviderOrigin == "") {
		return nil, fmt.Errorf("attachment destination is unavailable")
	}
	for i := range result.Attachments {
		item := &result.Attachments[i]
		if err := validateImportedAttachment(scope, item); err != nil {
			return nil, err
		}
		if seen[item.ID] {
			return nil, fmt.Errorf("attachment catalog repeats a version")
		}
		seen[item.ID] = true
	}
	return &result, nil
}

func validateImportedAttachment(scope AgentSettingsScope, item *AgentArtifact) error {
	if err := validateLibraryArtifact(scope, item); err != nil {
		return err
	}
	if item.Kind != "inline" || item.Size < 1 || item.Size > 512<<10 || !slices.Contains([]string{"image/png", "image/jpeg", "application/pdf", "audio/wav", "audio/pcm;rate=16000"}, item.MIME) {
		return fmt.Errorf("imported attachment metadata is invalid")
	}
	return nil
}

func (c *Client) ImportAttachment(ctx context.Context, id string, scope AgentSettingsScope, request AttachmentImport) (*AgentArtifact, error) {
	if request.ProjectIdentity != scope.ProjectIdentity || !request.ConfirmImport || len(request.Data) == 0 || len(request.Data) > 512<<10 {
		return nil, fmt.Errorf("attachment import requires current scope, explicit confirmation and bounded bytes")
	}
	hash := sha256.Sum256(request.Data)
	var result AgentArtifact
	if err := c.externalJSON(ctx, id, scope, "attachments", "", "", request, &result); err != nil {
		return nil, err
	}
	if err := validateImportedAttachment(scope, &result); err != nil {
		return nil, err
	}
	if result.Hash != hex.EncodeToString(hash[:]) || result.Size != len(request.Data) || result.MIME != request.MIME || result.DisplayName != request.Name || result.ProjectIdentity != scope.ProjectIdentity {
		return nil, fmt.Errorf("import receipt differs from the reviewed bytes")
	}
	return &result, nil
}

func validAttachmentReference(ref AttachmentReference) bool {
	return libraryHash.MatchString(ref.ID) && libraryHash.MatchString(ref.Hash) && ref.Version > 0 && ref.Version <= 1<<53
}

func validateAttachmentAdvice(scope AgentSettingsScope, request AttachmentAdviceRequest) error {
	if request.ProjectIdentity != scope.ProjectIdentity || !libraryHash.MatchString(request.Configuration) || request.SettingsVersion < 0 || request.Model == "" || request.ProviderOrigin == "" || !request.ConfirmSourceSharing {
		return fmt.Errorf("attachment advice requires exact settings, destination and sharing confirmation")
	}
	if strings.TrimSpace(request.Question) == "" || len(request.Question) > 2048 || !utf8.ValidString(request.Question) || strings.ContainsRune(request.Question, 0) || len(request.Attachments) < 1 || len(request.Attachments) > 4 {
		return fmt.Errorf("provide a bounded question and one to four selected versions")
	}
	return validateAttachmentSelection(request.Attachments)
}

func validateAttachmentSelection(refs []AttachmentReference) error {
	seen := map[string]bool{}
	for _, ref := range refs {
		if !validAttachmentReference(ref) || seen[ref.ID] {
			return fmt.Errorf("select distinct immutable attachment versions")
		}
		seen[ref.ID] = true
	}
	return nil
}

func (c *Client) AttachmentAdvice(ctx context.Context, id string, scope AgentSettingsScope, request AttachmentAdviceRequest) (*AttachmentAdviceReceipt, error) {
	if err := validateAttachmentAdvice(scope, request); err != nil {
		return nil, err
	}
	var result AttachmentAdviceReceipt
	if err := c.externalJSON(ctx, id, scope, "attachments", "", "advice", request, &result); err != nil {
		return nil, err
	}
	if err := validateLibraryArtifact(scope, &result.Artifact); err != nil {
		return nil, err
	}
	if result.State != "advisory" || result.Model != request.Model || result.Artifact.ProjectIdentity != scope.ProjectIdentity || result.Artifact.Kind != "text" {
		return nil, fmt.Errorf("advice receipt differs from the admitted request")
	}
	if err := validateAttachmentUsage(result.Usage, result.UsageKnown); err != nil {
		return nil, err
	}
	return &result, nil
}

func validateAttachmentUsage(u AttachmentUsage, known bool) error {
	if u.PromptDetails != nil && (u.PromptDetails.CachedTokens < 0 || u.PromptDetails.CachedTokens > u.PromptTokens) {
		return fmt.Errorf("advice cache usage is invalid")
	}
	if u.PromptTokens < 0 || u.CompletionTokens < 0 || u.TotalTokens < 0 || known && u.TotalTokens < u.PromptTokens+u.CompletionTokens {
		return fmt.Errorf("advice usage receipt is invalid")
	}
	return nil
}
