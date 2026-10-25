package client

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

func validLiveBinding(binding LiveBinding) bool {
	return binding.ProjectID != "" && binding.ProjectRevision != "" && libraryHash.MatchString(binding.Configuration) && binding.SettingsVersion >= 0 && binding.SettingsVersion <= 1<<53 && len(binding.ProviderOrigin) > 0 && len(binding.ProviderOrigin) <= 512
}

func (c *Client) ReadLiveSessions(ctx context.Context, id string, scope AgentSettingsScope) (*LiveOverview, error) {
	var result LiveOverview
	if err := c.externalJSON(ctx, id, scope, "live-sessions", "", "", nil, &result); err != nil {
		return nil, err
	}
	if result.ProjectIdentity != scope.ProjectIdentity || !validLiveBinding(result.LiveBinding) || !slices.Contains([]string{"available", "disabled", "unsupported_profile", "stale_settings"}, result.State) || len(result.Sessions) > 128 {
		return nil, fmt.Errorf("live availability belongs to another checkout or is invalid")
	}
	if err := validateLiveOverview(result); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for i := range result.Sessions {
		item := &result.Sessions[i]
		if err := validateLiveSession(scope, item); err != nil {
			return nil, err
		}
		if seen[item.ID] {
			return nil, fmt.Errorf("live history repeats a session")
		}
		seen[item.ID] = true
	}
	return &result, nil
}

func validateLiveOverview(value LiveOverview) error {
	if value.Destination.Model != value.Model || value.Destination.ProviderOrigin != value.ProviderOrigin || value.LifetimeSeconds != 300 || value.MaxConnections != 2 || value.MaxTurnsPerConnection != 8 || value.MaxInputBytes != 128<<10 || value.MaxOutputBytes != 512<<10 {
		return fmt.Errorf("live destination or bounds are inconsistent")
	}
	if value.State == "available" && (value.Model != "gemini-3.8-live" || !value.Destination.Capabilities.LiveSessions) {
		return fmt.Errorf("live protocol is unavailable")
	}
	return nil
}

func validateLiveSession(scope AgentSettingsScope, item *LiveSession) error {
	if err := validateLiveIdentity(scope, item); err != nil {
		return err
	}
	for _, check := range []func(*LiveSession) error{validateLiveCounters, validateLiveReferences, validateLiveRecovery, validateLiveTimes} {
		if err := check(item); err != nil {
			return err
		}
	}
	for i := range item.OutputArtifacts {
		if err := validateLibraryArtifact(scope, &item.OutputArtifacts[i]); err != nil {
			return err
		}
		if item.OutputArtifacts[i].ProjectRevision != item.ProjectRevision {
			return fmt.Errorf("live output belongs to another revision")
		}
	}
	return nil
}

func validateLiveIdentity(scope AgentSettingsScope, item *LiveSession) error {
	if !validLiveBinding(item.LiveBinding) || item.ProjectID != scope.ProjectID || !externalRecordID.MatchString(item.ID) || item.Policy == "" || len(item.Policy) > 128 || item.Model != "gemini-3.8-live" {
		return fmt.Errorf("live receipt has invalid ownership or binding")
	}
	if item.Version < 1 || item.ControlRevision < 1 || item.ControlRevision > item.Version || item.Version > 1<<53 || !slices.Contains([]string{"connecting", "connected", "disconnected", "go_away", "closed", "interrupted", "limit_reached", "failed"}, item.State) {
		return fmt.Errorf("live receipt state or revision is invalid")
	}
	if !item.Stale && item.ProjectRevision != scope.ProjectRevision {
		return fmt.Errorf("historical live session claims current scope")
	}
	return nil
}

func validateLiveCounters(item *LiveSession) error {
	if item.Connections < 1 || item.Connections > 2 || item.Turns < 0 || item.Turns > 16 || item.CompletedTurns < 0 || item.CompletedTurns > 512 {
		return fmt.Errorf("live progress exceeds its bound")
	}
	for _, n := range []int64{item.SentBytes, item.ReceivedBytes, item.ReportedInput, item.ReportedOutput, item.Usage.InputTokens, item.Usage.OutputTokens, item.Usage.ReservedInputTokens, item.Usage.ReservedOutputTokens, item.Usage.ElapsedMillis, int64(item.Usage.ModelCalls), int64(item.Usage.UnknownUsageCalls), int64(item.Usage.ActiveCalls)} {
		if n < 0 || n > 1<<40 {
			return fmt.Errorf("live usage is invalid")
		}
	}
	if !item.Reported && (item.ReportedInput != 0 || item.ReportedOutput != 0) {
		return fmt.Errorf("unreported live usage has measured counts")
	}
	return nil
}

func validateLiveReferences(item *LiveSession) error {
	if len(item.RunIDs) > 2 || len(item.Artifacts) > 4 || len(item.OutputArtifacts) > 4 || item.MissingArtifacts < 0 || item.MissingArtifacts > 4 {
		return fmt.Errorf("live references exceed their bound")
	}
	for _, id := range item.RunIDs {
		if id == "" || len(id) > 128 {
			return fmt.Errorf("live run reference is invalid")
		}
	}
	for _, ref := range item.Artifacts {
		if ref.Version < 1 || ref.Name == "" || ref.Session.ID == "" {
			return fmt.Errorf("live artifact reference is invalid")
		}
	}
	return nil
}

func validateLiveRecovery(item *LiveSession) error {
	if item.CanReconnect && (!item.Owned || item.Stale || item.RetentionFailed || item.Connections >= 2 || !slices.Contains([]string{"failed", "go_away", "disconnected"}, item.State)) {
		return fmt.Errorf("live recovery metadata is inconsistent")
	}
	if !item.Owned && slices.Contains([]string{"connected", "connecting", "go_away", "disconnected"}, item.State) {
		return fmt.Errorf("live state has no current owner")
	}
	return nil
}

func validateLiveTimes(item *LiveSession) error {
	if item.CreatedAt.IsZero() || item.UpdatedAt.Before(item.CreatedAt) || !item.ExpiresAt.After(item.CreatedAt) || item.ExpiresAt.Sub(item.CreatedAt) > 5*time.Minute {
		return fmt.Errorf("live lifetime is invalid")
	}
	return nil
}

func (c *Client) LiveSessionDetail(ctx context.Context, id string, scope AgentSettingsScope, sessionID string) (*LiveSession, error) {
	var result LiveSession
	if err := c.externalJSON(ctx, id, scope, "live-sessions", sessionID, "", nil, &result); err != nil {
		return nil, err
	}
	if err := validateLiveSession(scope, &result); err != nil {
		return nil, err
	}
	if result.ID != sessionID {
		return nil, fmt.Errorf("live detail belongs to another session")
	}
	return &result, nil
}

func (c *Client) OpenLiveSession(ctx context.Context, id string, scope AgentSettingsScope, request LiveOpenRequest) (*LiveSession, error) {
	if request.ProjectIdentity != scope.ProjectIdentity || !validLiveBinding(request.LiveBinding) || !request.ConfirmConnect || request.Model != "gemini-3.8-live" {
		return nil, fmt.Errorf("live connection requires exact scope, configuration and confirmation")
	}
	var result LiveSession
	if err := c.externalJSON(ctx, id, scope, "live-sessions", "", "", request, &result); err != nil {
		return nil, err
	}
	if err := validateLiveSession(scope, &result); err != nil {
		return nil, err
	}
	if result.LiveBinding != request.LiveBinding {
		return nil, fmt.Errorf("live connection receipt differs from the admitted destination")
	}
	return &result, nil
}

func (c *Client) ControlLiveSession(ctx context.Context, id string, scope AgentSettingsScope, sessionID string, request LiveControl) (*LiveSession, error) {
	if request.ProjectIdentity != scope.ProjectIdentity || !validLiveBinding(request.LiveBinding) || request.ControlRevision < 1 {
		return nil, fmt.Errorf("live control requires the current binding and revision")
	}
	if err := validateLiveControl(request); err != nil {
		return nil, err
	}
	var result LiveSession
	if err := c.externalJSON(ctx, id, scope, "live-sessions", sessionID, "control", request, &result); err != nil {
		return nil, err
	}
	if err := validateLiveSession(scope, &result); err != nil {
		return nil, err
	}
	if result.ID != sessionID || result.LiveBinding != request.LiveBinding || result.ControlRevision <= request.ControlRevision {
		return nil, fmt.Errorf("live receipt differs from the accepted control")
	}
	return &result, nil
}

func validateLiveControl(request LiveControl) error {
	if request.Action != "send" && (request.Text != "" || request.Attachment != nil) {
		return fmt.Errorf("only send accepts live input")
	}
	switch request.Action {
	case "reconnect":
		if !request.ConfirmConnect {
			return fmt.Errorf("reconnect requires fresh confirmation")
		}
	case "close", "interrupt":
	case "send":
		return validateLiveSend(request)
	default:
		return fmt.Errorf("unsupported live control")
	}
	return nil
}

func validateLiveSend(request LiveControl) error {
	if !request.ConfirmSourceSharing {
		return fmt.Errorf("live input requires sharing confirmation")
	}
	if request.Attachment != nil {
		if request.Text != "" || !validAttachmentReference(*request.Attachment) {
			return fmt.Errorf("send exactly one selected media version")
		}
		return nil
	}
	if strings.TrimSpace(request.Text) == "" || len(request.Text) > 2048 || !utf8.ValidString(request.Text) || strings.ContainsRune(request.Text, 0) {
		return fmt.Errorf("live text must contain 1–2048 UTF-8 bytes")
	}
	return nil
}
