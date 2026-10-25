package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"maps"
	"net"
	"net/http"
	"net/url"
	"slices"
	"sync/atomic"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/version"
)

type AgentWorkerOptions struct {
	Scope                                       AgentWorkerScope
	Endpoint                                    string
	Lifetime                                    time.Duration
	MaxJobs                                     int
	ConfirmSourceSharing, ConfirmRemoteProvider bool
}

type workerPrincipal struct {
	config     config.AgentPrincipal
	credential [32]byte
	owner      string
}

// AgentWorker owns only admitted in-memory source, model calls and A2A tasks.
// It intentionally has no Service, project manager, store or checkout handle.
type AgentWorker struct {
	native     *agentruntime.AgentServer
	ctx        context.Context
	cancel     context.CancelFunc
	profile    config.ModelProfile
	limits     config.AgentLimits
	client     *agentruntime.ProviderClient
	scope      AgentWorkerScope
	scopeHash  string
	principals []workerPrincipal
	expires    time.Time
	maxJobs    int32
	jobs       atomic.Int32
	closed     atomic.Bool
}

type AgentWorkerResult struct {
	Advisory        bool                   `json:"advisory"`
	ProjectID       string                 `json:"project_id"`
	ProjectRevision string                 `json:"project_revision"`
	ScopeHash       string                 `json:"scope_hash"`
	Checks          string                 `json:"checks"`
	Answer          AgentWorkerAnswer      `json:"answer"`
	Usage           agentstate.BudgetUsage `json:"usage"`
}

func NewAgentWorker(ctx context.Context, cfg *config.Config, options AgentWorkerOptions) (*AgentWorker, error) {
	if err := validateWorkerOptions(ctx, options); err != nil {
		return nil, err
	}
	profiles, err := config.ResolveModelProfiles(cfg)
	if err != nil {
		return nil, err
	}
	integrations, err := config.ResolveIntegrations(cfg.Integrations)
	if err != nil {
		return nil, err
	}
	runtime, err := config.ResolveAgentRuntime(cfg.AgentRuntime)
	if err != nil {
		return nil, err
	}
	profile := profiles.Code
	if profile.IsCLI() || !integrations.A2AServer.Enabled {
		return nil, fmt.Errorf("worker requires an enabled A2A service and an HTTP Code model")
	}
	if !isLoopbackURL(profile.APIBaseURL) && !options.ConfirmRemoteProvider {
		return nil, fmt.Errorf("worker requires explicit remote provider confirmation")
	}
	principals, err := workerPrincipals(integrations.A2AServer.Principals, options.Scope.ProjectID)
	if err != nil {
		return nil, err
	}
	profile.MaxTokens = min(profile.MaxTokens, 4096)
	owned, cancel := context.WithTimeout(ctx, options.Lifetime)
	w := &AgentWorker{ctx: owned, cancel: cancel, profile: profile, client: agentruntime.NewProviderClient(profile), scope: options.Scope, principals: principals, expires: time.Now().Add(options.Lifetime), maxJobs: int32(options.MaxJobs)}
	w.scope.Files = slices.Clone(options.Scope.Files)
	data, err := json.Marshal(w.scope)
	if err != nil {
		cancel()
		return nil, err
	}
	w.scopeHash = agentruntime.TraceIdentity(string(data))
	w.limits = workerLimits(runtime.Limits, integrations.A2AServer.TimeoutSeconds)
	w.native, err = agentruntime.NewAgentServer(agentruntime.AgentServerConfig{Endpoint: options.Endpoint, Version: version.Version, Timeout: time.Duration(w.limits.ElapsedSeconds) * time.Second, MaxConcurrent: integrations.A2AServer.MaxConcurrent, InputBytes: 128 << 10, AnalysisOnly: true}, w.authenticate)
	if err != nil {
		cancel()
		return nil, err
	}
	return w, nil
}

func validateWorkerOptions(ctx context.Context, o AgentWorkerOptions) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	if err := validateWorkerScope(o.Scope); err != nil {
		return err
	}
	if !o.ConfirmSourceSharing || o.Lifetime < time.Second || o.Lifetime > 15*time.Minute || o.MaxJobs < 1 || o.MaxJobs > 128 {
		return fmt.Errorf("worker requires source-sharing consent, a 1–900 second lifetime and 1–128 jobs")
	}
	return validateWorkerEndpoint(o.Endpoint)
}

func validateWorkerEndpoint(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil || u.User != nil || u.Host == "" || u.Path != "/a2a" || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("worker public URL must be a credential-free /a2a endpoint")
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme != "https" && (u.Scheme != "http" || ip == nil || !ip.IsLoopback()) {
		return fmt.Errorf("worker public URL requires HTTPS or literal loopback HTTP")
	}
	return nil
}

func workerPrincipals(configured []config.AgentPrincipal, projectID string) ([]workerPrincipal, error) {
	var result []workerPrincipal
	seen := map[[32]byte]bool{}
	for _, p := range configured {
		if p.ProjectID != projectID || !slices.Equal(p.Capabilities, []string{"analysis"}) {
			return nil, fmt.Errorf("worker principals must select this exact project and only analysis")
		}
		credential, err := remoteServiceCredential(p)
		if err != nil || seen[credential] {
			return nil, fmt.Errorf("worker principals require distinct available bearer credentials")
		}
		seen[credential] = true
		result = append(result, workerPrincipal{config: p, credential: credential, owner: rand.Text()})
	}
	return result, nil
}

func workerLimits(l config.AgentLimits, seconds int) config.AgentLimits {
	l.ModelCalls, l.ToolCalls = min(l.ModelCalls, 4), min(l.ToolCalls, 16)
	l.InputTokens, l.OutputTokens = min(l.InputTokens, 131072), min(l.OutputTokens, 16384)
	l.ElapsedSeconds, l.ConcurrentCalls, l.Repairs = min(l.ElapsedSeconds, seconds), 1, 0
	return l
}

func (w *AgentWorker) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	w.native.ServeHTTP(response, request)
}
func (w *AgentWorker) Done() <-chan struct{} { return w.ctx.Done() }
func (w *AgentWorker) Close(ctx context.Context) error {
	w.closed.Store(true)
	w.cancel()
	return w.native.Close(ctx)
}

func (w *AgentWorker) authenticate(ctx context.Context, token string) (agentruntime.ServerAdmission, error) {
	digest := sha256.Sum256([]byte(token))
	var match *workerPrincipal
	for i := range w.principals {
		if subtle.ConstantTimeCompare(digest[:], w.principals[i].credential[:]) == 1 {
			match = &w.principals[i]
		}
	}
	if match == nil {
		return agentruntime.ServerAdmission{}, ErrIntegrationUnavailable
	}
	p := *match
	check := func(ctx context.Context) error { return w.validate(ctx, p) }
	if err := check(ctx); err != nil {
		return agentruntime.ServerAdmission{}, err
	}
	return agentruntime.ServerAdmission{Owner: p.owner, ExpiresAt: w.expires, Validate: check, Execute: func(ctx context.Context, text string) (string, error) { return w.execute(ctx, p, text) }}, nil
}

func (w *AgentWorker) validate(ctx context.Context, p workerPrincipal) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	if w.closed.Load() || context.Cause(w.ctx) != nil || !w.expires.After(time.Now()) {
		return ErrIntegrationUnavailable
	}
	credential, err := remoteServiceCredential(p.config)
	if err != nil || credential != p.credential {
		return ErrIntegrationUnavailable
	}
	return nil
}

func (w *AgentWorker) execute(ctx context.Context, p workerPrincipal, text string) (string, error) {
	request, err := decodeWorkerRequest(text, w.scope)
	if err != nil {
		return "", err
	}
	if !isLoopbackURL(w.profile.APIBaseURL) && !request.ConfirmRemoteProvider {
		return "", fmt.Errorf("worker request requires explicit remote provider consent")
	}
	if err = w.validate(ctx, p); err != nil {
		return "", err
	}
	if w.jobs.Add(1) > w.maxJobs {
		return "", fmt.Errorf("worker job allowance exhausted")
	}
	owned, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(w.ctx, cancel)
	defer func() { stop(); cancel() }()
	budget, done, err := agentruntime.WithBudget(owned, w.limits)
	if err != nil {
		return "", err
	}
	defer done()
	reader, err := workerReader(request.Files)
	if err != nil {
		return "", err
	}
	investigation, err := agentruntime.NewInvestigation(w.scopeHash, workerTools(reader), func(ctx context.Context) error { return w.validate(ctx, p) })
	if err != nil {
		return "", err
	}
	input, err := json.Marshal(struct {
		Question string
		Paths    []string
	}{request.Question, slices.Sorted(maps.Keys(request.Files))})
	if err != nil {
		return "", err
	}
	result, err := agentruntime.Complete(budget, w.client, agentruntime.Request{Messages: []llm.ChatMessage{{Role: "user", Content: workerPrompt + "\n" + string(input)}}, Schema: &llm.JSONSchema{Name: "worker_analysis", Schema: json.RawMessage(workerAnswerSchema)}, Investigation: investigation, ContextMaxTokens: w.profile.ContextMaxTokens})
	if err != nil {
		return "", err
	}
	answer, err := decodeWorkerAnswer(result.Content, request.Files)
	if err != nil {
		return "", err
	}
	if err = w.validate(budget, p); err != nil {
		return "", err
	}
	out, err := json.Marshal(AgentWorkerResult{Advisory: true, ProjectID: w.scope.ProjectID, ProjectRevision: w.scope.ProjectRevision, ScopeHash: w.scopeHash, Checks: "not_run", Answer: answer, Usage: agentruntime.BudgetFromContext(budget).Snapshot()})
	if err != nil {
		return "", err
	}
	return string(out), nil
}
