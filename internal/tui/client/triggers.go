package client

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode"
)

func (o TriggerOptions) Validate() error {
	if o.Mode == "disabled" {
		if len(o.Paths) != 0 || o.IntervalSeconds != 0 {
			return fmt.Errorf("disabled watches have no selected paths or interval")
		}
		return nil
	}
	if !slices.Contains([]string{"changes", "interval"}, o.Mode) || len(o.Paths) < 1 || len(o.Paths) > 8 {
		return fmt.Errorf("choose changes or interval and one to eight source paths")
	}
	if o.Mode == "changes" && o.IntervalSeconds != 0 || o.Mode == "interval" && (o.IntervalSeconds < 60 || o.IntervalSeconds > 86400) {
		return fmt.Errorf("interval must be 60–86400 seconds; changes uses zero")
	}
	return validateWatchPaths(o.Paths)
}

func validateWatchPaths(paths []string) error {
	for i, path := range paths {
		if !fs.ValidPath(path) || path == "." || len(path) > 512 || strings.ContainsAny(path, "\\:") || strings.IndexFunc(path, unicode.IsControl) >= 0 || slices.Contains(paths[:i], path) {
			return fmt.Errorf("watch paths must be distinct canonical project-relative files")
		}
	}
	return nil
}

func (a TriggerAllowance) Within(limit TriggerAllowance) bool {
	return a.Runs >= 0 && a.Runs <= limit.Runs && a.ModelCalls >= 0 && a.ModelCalls <= limit.ModelCalls && a.InputTokens >= 0 && a.InputTokens <= limit.InputTokens && a.OutputTokens >= 0 && a.OutputTokens <= limit.OutputTokens && a.Seconds >= 0 && a.Seconds <= limit.Seconds
}

func mainTriggerScope(scope AgentSettingsScope) error {
	if scope.WorktreePath != "" || scope.ProjectID == "" || scope.ProjectRevision == "" {
		return fmt.Errorf("ambient analysis requires the current main checkout")
	}
	return nil
}

func (c *Client) ReadAgentTriggers(ctx context.Context, id string, scope AgentSettingsScope) (*AgentTriggers, error) {
	if err := mainTriggerScope(scope); err != nil {
		return nil, err
	}
	var result AgentTriggers
	if err := c.externalJSON(ctx, id, scope, "agent-triggers", "", "", nil, &result); err != nil {
		return nil, err
	}
	if err := validateAgentTriggers(scope, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Refresh re-reads the index because an admitted ambient attempt may have updated
// the main revision. It can advance that checkout, never adopt another project.
func (c *Client) RefreshAgentTriggers(ctx context.Context, id string, scope AgentSettingsScope) (*AgentTriggers, *ProjectIndex, error) {
	if err := mainTriggerScope(scope); err != nil {
		return nil, nil, err
	}
	var index ProjectIndex
	if err := c.DoJSON(ctx, id+"-index", http.MethodGet, "/api/projects/current/index", nil, &index); err != nil {
		return nil, nil, err
	}
	if index.ProjectID != scope.ProjectID || index.ProjectRevision == "" {
		return nil, nil, fmt.Errorf("ambient index belongs to another project")
	}
	scope.ProjectIdentity = index.ProjectIdentity
	value, err := c.ReadAgentTriggers(ctx, id+"-state", scope)
	if err != nil {
		return nil, nil, err
	}
	return value, &index, nil
}

func (c *Client) ControlAgentTriggers(ctx context.Context, id string, scope AgentSettingsScope, request TriggerControl) (*AgentTriggers, error) {
	if err := validateTriggerRequest(scope, request); err != nil {
		return nil, err
	}
	var result AgentTriggers
	if err := c.externalJSON(ctx, id, scope, "agent-triggers", "", "control", request, &result); err != nil {
		return nil, err
	}
	if err := validateAgentTriggers(scope, &result); err != nil {
		return nil, err
	}
	if result.Record.ControlRevision != request.ControlRevision+1 {
		return nil, fmt.Errorf("ambient control receipt has an unexpected revision; refresh before retrying")
	}
	if err := validateTriggerOutcome(request, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func validateTriggerRequest(scope AgentSettingsScope, r TriggerControl) error {
	if err := mainTriggerScope(scope); err != nil {
		return err
	}
	if r.ProjectIdentity != scope.ProjectIdentity || !r.Confirm || r.ControlRevision < 0 || r.ControlRevision >= 1<<53 || !slices.Contains([]string{"watch", "grant", "revoke", "disable"}, r.Action) {
		return fmt.Errorf("ambient control requires the current checkout, revision and confirmation")
	}
	return validateTriggerControlOptions(r)
}

func validateTriggerControlOptions(r TriggerControl) error {
	if r.Action == "watch" {
		if r.Options == nil || r.Options.Mode == "disabled" {
			return fmt.Errorf("watch requires explicit source paths and mode")
		}
		return r.Options.Validate()
	}
	if r.Options != nil {
		return fmt.Errorf("only watch can replace selected paths")
	}
	if r.Action == "grant" && (!r.ConfirmSourceSharing || !libraryHash.MatchString(r.ScopeHash) || r.GrantSeconds < 60 || r.GrantSeconds > 86400) {
		return fmt.Errorf("grant requires exact scope, source-sharing confirmation and bounded expiry")
	}
	return nil
}

func validateTriggerOutcome(r TriggerControl, v *AgentTriggers) error {
	if r.Action == "grant" {
		g := v.Record.Grant
		if v.TriggerBinding != r.TriggerBinding || g == nil || g.ScopeHash != r.ScopeHash || g.ExpiresAt.Sub(g.CreatedAt) != time.Duration(r.GrantSeconds)*time.Second {
			return fmt.Errorf("grant receipt does not match the confirmed scope or expiry")
		}
		return nil
	}
	if v.Record.Grant != nil {
		return fmt.Errorf("control unexpectedly retained automatic permission")
	}
	if r.Action == "disable" && (v.Owned || v.Record.Options.Mode != "disabled") {
		return fmt.Errorf("disable receipt did not stop the watch")
	}
	if r.Action == "watch" {
		paths := slices.Clone(r.Options.Paths)
		slices.Sort(paths)
		o := v.Record.Options
		if !v.Owned || o.Mode != r.Options.Mode || o.IntervalSeconds != r.Options.IntervalSeconds || !slices.Equal(o.Paths, paths) {
			return fmt.Errorf("watch receipt differs from the confirmed selection")
		}
	}
	return nil
}

func validateAgentTriggers(scope AgentSettingsScope, v *AgentTriggers) error {
	if v.ProjectIdentity != scope.ProjectIdentity || v.Record.ProjectID != scope.ProjectID || !validTriggerBinding(v.TriggerBinding) || !libraryHash.MatchString(v.ScopeHash) {
		return fmt.Errorf("ambient metadata belongs to another checkout or binding")
	}
	if v.Destination.Model != v.Model || v.Destination.ProviderOrigin != v.ProviderOrigin || !slices.Contains([]string{"disabled", "watch_only", "granted", "grant_expired", "stale_settings", "stale_grant", "interrupted", "failed"}, v.State) {
		return fmt.Errorf("ambient permission state or destination is invalid")
	}
	for _, check := range []func(*AgentTriggers) error{validateTriggerRecord, validateTriggerWindow, validateTriggerGrant, validateTriggerBounds} {
		if err := check(v); err != nil {
			return err
		}
	}
	return nil
}

func validTriggerBinding(b TriggerBinding) bool {
	return libraryHash.MatchString(b.Configuration) && b.SettingsVersion >= 0 && b.SettingsVersion <= 1<<53 && b.Policy != "" && len(b.Policy) <= 128 && b.Model != "" && len(b.Model) <= 128 && b.ProviderOrigin != "" && len(b.ProviderOrigin) <= 512
}

func validateTriggerRecord(v *AgentTriggers) error {
	r := v.Record
	if r.Version < 0 || r.Version > 1<<53 || r.ControlRevision < 0 || r.ControlRevision > r.Version || r.Options.Validate() != nil || len(r.Reason) > 256 {
		return fmt.Errorf("ambient record or selection is invalid")
	}
	if !slices.Contains([]string{"disabled", "watching", "suggested", "waiting", "unchanged", "running", "completed", "failed", "interrupted", "canceled", "budget_exhausted", "stale", "unavailable"}, r.State) {
		return fmt.Errorf("ambient attempt state is invalid")
	}
	for _, values := range []map[string]string{r.Observed, r.Handled} {
		if len(values) > 8 {
			return fmt.Errorf("ambient snapshot list exceeds its bound")
		}
		for path, hash := range values {
			if !slices.Contains(r.Options.Paths, path) || !libraryHash.MatchString(hash) {
				return fmt.Errorf("ambient snapshot has invalid source identity")
			}
		}
	}
	return validateTriggerAttempt(r.LastAttempt)
}

func validateTriggerAttempt(a *TriggerAttempt) error {
	if a == nil {
		return nil
	}
	maximum := TriggerAllowance{Runs: 1, ModelCalls: 64, InputTokens: 64 << 20, OutputTokens: 16 << 20, Seconds: 300}
	if !externalRecordID.MatchString(a.ID) || a.ProjectRevision == "" || validateWatchPaths([]string{a.Path}) != nil || !libraryHash.MatchString(a.SourceHash) || !a.Reserved.Within(maximum) || a.Reserved.Runs != 1 {
		return fmt.Errorf("ambient attempt receipt is invalid")
	}
	if !slices.Contains([]string{"running", "completed", "failed", "interrupted", "canceled"}, a.State) || a.StartedAt.IsZero() {
		return fmt.Errorf("ambient attempt state or start time is invalid")
	}
	if a.State == "running" && !a.FinishedAt.IsZero() || a.State != "running" && a.FinishedAt.Before(a.StartedAt) {
		return fmt.Errorf("ambient attempt timestamps are inconsistent")
	}
	return validateTriggerUsage(a.Usage)
}

func validateTriggerUsage(u AgentUsage) error {
	for _, n := range []int64{int64(u.ModelCalls), int64(u.ToolCalls), int64(u.UnknownUsageCalls), int64(u.ActiveCalls), u.InputTokens, u.OutputTokens, u.ReservedInputTokens, u.ReservedOutputTokens, u.ElapsedMillis} {
		if n < 0 {
			return fmt.Errorf("ambient attempt usage is invalid")
		}
	}
	return nil
}

func validateTriggerWindow(v *AgentTriggers) error {
	w := v.Record.Window
	if w.StartedAt.IsZero() {
		if w != (TriggerWindow{}) {
			return fmt.Errorf("unstarted ambient window contains charges")
		}
		return nil
	}
	maximum := TriggerAllowance{Runs: 16, ModelCalls: 64, InputTokens: 64 << 20, OutputTokens: 16 << 20, Seconds: 86400}
	if !w.EndsAt.After(w.StartedAt) || w.EndsAt.Sub(w.StartedAt) > 24*time.Hour || !w.Limits.Within(maximum) || !w.Charged.Within(w.Limits) {
		return fmt.Errorf("ambient window allowance or consumption is invalid")
	}
	return nil
}

func validateTriggerGrant(v *AgentTriggers) error {
	g := v.Record.Grant
	if g == nil {
		if v.State == "granted" || v.State == "grant_expired" {
			return fmt.Errorf("automatic permission has no grant")
		}
		return nil
	}
	if v.Record.Options.Mode == "disabled" || !externalRecordID.MatchString(g.ID) || g.ProjectID != v.ProjectID || g.CreatedAt.IsZero() || !g.ExpiresAt.After(g.CreatedAt) || g.ExpiresAt.Sub(g.CreatedAt) > 24*time.Hour {
		return fmt.Errorf("ambient grant identity or lifetime is invalid")
	}
	if v.State == "granted" && (!v.Owned || !triggerGrantIsCurrent(v, g)) {
		return fmt.Errorf("automatic permission does not match its current binding")
	}
	return nil
}

func triggerGrantIsCurrent(v *AgentTriggers, g *TriggerGrant) bool {
	return g.Configuration == v.Configuration && g.SettingsVersion == v.SettingsVersion && g.Policy == v.Policy && g.ScopeHash == v.ScopeHash && g.Model == v.Model && g.ProviderOrigin == v.ProviderOrigin
}

func validateTriggerBounds(v *AgentTriggers) error {
	c, l := v.Ceilings, v.RunLimits
	for _, rangeValue := range [][3]int64{{int64(c.PollSeconds), 1, 60}, {int64(c.DebounceSeconds), 1, 300}, {int64(c.QuietSeconds), 5, 3600}, {int64(c.WindowSeconds), 60, 86400}, {int64(c.MaxRuns), 1, 16}, {int64(c.MaxModelCalls), 1, 64}, {c.MaxInputTokens, 1024, 64 << 20}, {c.MaxOutputTokens, 1024, 16 << 20}, {int64(c.RunSeconds), 1, 300}, {int64(c.GrantSeconds), 60, 86400}} {
		if rangeValue[0] < rangeValue[1] || rangeValue[0] > rangeValue[2] {
			return fmt.Errorf("ambient configured ceiling is invalid")
		}
	}
	if l.ModelCalls < 1 || l.ModelCalls > c.MaxModelCalls/c.MaxRuns || l.InputTokens < 1 || l.InputTokens > c.MaxInputTokens/int64(c.MaxRuns) || l.OutputTokens < 1 || l.OutputTokens > c.MaxOutputTokens/int64(c.MaxRuns) || l.ElapsedSeconds < 1 || l.ElapsedSeconds > c.RunSeconds {
		return fmt.Errorf("ambient run allowance exceeds its window ceilings")
	}
	if l.ConcurrentCalls != 1 || l.ToolCalls != 1 || l.Repairs != 0 {
		return fmt.Errorf("ambient run must preserve serial advisory execution")
	}
	return nil
}
