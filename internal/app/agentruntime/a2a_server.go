package agentruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"github.com/a2aproject/a2a-go/v2/a2asrv/limiter"
	"github.com/a2aproject/a2a-go/v2/a2asrv/taskstore"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/runner"
	adka2a "google.golang.org/adk/v2/server/adka2a/v2"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"
)

// ServerAdmission is supplied by the host after credential and project-grant
// checks. Owner is an opaque grant identity, not a client-selected session key.
type ServerAdmission struct {
	Owner     string
	ExpiresAt time.Time
	Validate  func(context.Context) error
	Execute   func(context.Context, string) (string, error)
}

type AgentServerConfig struct {
	Endpoint      string
	Version       string
	Timeout       time.Duration
	MaxConcurrent int
	InputBytes    int
	AnalysisOnly  bool
}

type AgentServer struct {
	mu       sync.Mutex
	closed   bool
	config   AgentServerConfig
	admit    func(context.Context, string) (ServerAdmission, error)
	rpc      http.Handler
	card     []byte
	executor *adka2a.Executor
	store    taskstore.Store
	messages map[string]bool
	active   map[a2a.TaskID]*serverExecution
	requests map[*serverRequest]bool
	permits  chan struct{}
}

type serverScopeKey struct{}
type serverScope struct {
	admission ServerAdmission
	request   context.Context
}

func NewAgentServer(cfg AgentServerConfig, admit func(context.Context, string) (ServerAdmission, error)) (*AgentServer, error) {
	if admit == nil || cfg.Timeout <= 0 || cfg.Timeout > 120*time.Second || cfg.MaxConcurrent < 1 || cfg.MaxConcurrent > 4 {
		return nil, fmt.Errorf("invalid agent server configuration")
	}
	if cfg.InputBytes == 0 {
		cfg.InputBytes = 8 << 10
	}
	if cfg.InputBytes < 8<<10 || cfg.InputBytes > 128<<10 {
		return nil, fmt.Errorf("agent input limit must be 8–128 KiB")
	}
	s := &AgentServer{config: cfg, admit: admit, messages: map[string]bool{}, active: map[a2a.TaskID]*serverExecution{}, requests: map[*serverRequest]bool{}, permits: make(chan struct{}, 16)}
	worker, err := agent.New(agent.Config{Name: "mini_orca", Description: "Host-admitted project analysis and proposal preparation", Run: runServerAgent})
	if err != nil {
		return nil, fmt.Errorf("create local A2A agent: %w", err)
	}
	s.executor = adka2a.NewExecutor(adka2a.ExecutorConfig{RunnerConfig: runner.Config{AppName: "mini_orca_a2a", Agent: worker, SessionService: session.InMemoryService()}, OutputMode: adka2a.OutputArtifactPerEvent})
	s.store = taskstore.NewInMemory(&taskstore.InMemoryStoreConfig{Authenticator: a2asrv.NewTaskStoreAuthenticator()})
	handler := a2asrv.NewHandler(s, a2asrv.WithTaskStore(s.store), a2asrv.WithCallInterceptors(serverInterceptor{s: s}),
		a2asrv.WithCapabilityChecks(&a2a.AgentCapabilities{Streaming: true}),
		a2asrv.WithConcurrencyConfig(limiter.ConcurrencyConfig{MaxExecutions: cfg.MaxConcurrent}),
		a2asrv.WithAgentInactivityTimeout(cfg.Timeout+time.Second),
		a2asrv.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		a2asrv.WithExecutionPanicHandler(func(any) error { return errors.New("local agent execution failed") }))
	s.rpc = a2asrv.NewJSONRPCHandler(handler)
	s.card, err = json.Marshal(serverCard(cfg))
	if err != nil {
		return nil, fmt.Errorf("encode local agent card: %w", err)
	}
	return s, nil
}

func serverCard(cfg AgentServerConfig) *a2a.AgentCard {
	card := &a2a.AgentCard{Name: "Mini-Orca", Version: cfg.Version,
		Description:         "Explicitly granted project work. Send one text part containing the documented Mini-Orca request JSON. Local human review is required for source changes.",
		SupportedInterfaces: []*a2a.AgentInterface{{URL: cfg.Endpoint, ProtocolBinding: "JSONRPC", ProtocolVersion: "1.0"}},
		Capabilities:        a2a.AgentCapabilities{Streaming: true}, DefaultInputModes: []string{"text/plain"}, DefaultOutputModes: []string{"text/plain"},
		SecuritySchemes:      a2a.NamedSecuritySchemes{"bearer": a2a.HTTPAuthSecurityScheme{Scheme: "Bearer"}},
		SecurityRequirements: a2a.SecurityRequirementsOptions{{"bearer": {}}},
		Skills: []a2a.AgentSkill{
			{ID: "analysis", Name: "Analyze admitted source", Description: "Evidence-backed advisory investigation of one to eight captured paths", Tags: []string{"analysis"}},
			{ID: "proposal", Name: "Prepare a proposal", Description: "Create a private local draft for human review; never apply source", Tags: []string{"proposal"}},
		}}
	if cfg.AnalysisOnly {
		card.Description = "Authenticated advisory analysis of request-supplied source matching an operator-approved project, revision and path/hash manifest. No checkout access or source changes."
		card.Skills = card.Skills[:1]
	}
	return card
}

func runServerAgent(ctx agent.InvocationContext) iter.Seq2[*session.Event, error] {
	return func(yield func(*session.Event, error) bool) {
		scope, ok := ctx.Value(serverScopeKey{}).(serverScope)
		if !ok || scope.admission.Validate(ctx) != nil {
			yield(nil, errors.New("project access unavailable"))
			return
		}
		content := ctx.UserContent()
		if content == nil || len(content.Parts) != 1 || content.Parts[0] == nil {
			yield(nil, errors.New("invalid admitted request"))
			return
		}
		text, err := scope.admission.Execute(ctx, content.Parts[0].Text)
		if err != nil || scope.admission.Validate(ctx) != nil {
			yield(nil, errors.New("project operation failed or access changed; inspect local run history"))
			return
		}
		if len(text) > 16<<10 || strings.TrimSpace(text) == "" || !utf8.ValidString(text) {
			yield(nil, errors.New("project result exceeds the remote response contract"))
			return
		}
		event := session.NewEvent(ctx, ctx.InvocationID())
		event.Author = "mini_orca"
		event.LLMResponse = model.LLMResponse{Content: genai.NewContentFromText(text, genai.RoleModel), TurnComplete: true}
		yield(event, nil)
	}
}

func (s *AgentServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	ctx, finish, err := s.beginRequest(r.Context())
	if err != nil {
		http.Error(w, "agent service unavailable", http.StatusServiceUnavailable)
		return
	}
	defer finish()
	if r.URL.Path == "/.well-known/agent-card.json" && r.Method == http.MethodGet && r.URL.RawQuery == "" {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(s.card)
		return
	}
	if r.URL.Path != "/a2a" || r.Method != http.MethodPost || r.URL.RawQuery != "" {
		http.NotFound(w, r)
		return
	}
	admission, err := s.authenticate(ctx, r)
	if err != nil {
		w.Header().Set("WWW-Authenticate", "Bearer")
		http.Error(w, "authenticated project grant required", http.StatusUnauthorized)
		return
	}
	if err := guardServerRequest(w, r, s.config.InputBytes); err != nil {
		http.Error(w, "invalid or unsupported A2A request", http.StatusBadRequest)
		return
	}
	ctx = context.WithValue(ctx, serverScopeKey{}, serverScope{admission: admission, request: ctx})
	request := r.Clone(ctx)
	request.Header.Del("Authorization")
	s.rpc.ServeHTTP(w, request)
}

func (s *AgentServer) authenticate(ctx context.Context, r *http.Request) (ServerAdmission, error) {
	values := r.Header.Values("Authorization")
	if len(values) != 1 || !strings.HasPrefix(values[0], "Bearer ") || len(values[0]) > 4103 {
		return ServerAdmission{}, errors.New("credential required")
	}
	admission, err := s.admit(ctx, strings.TrimPrefix(values[0], "Bearer "))
	if err != nil {
		return admission, err
	}
	if admission.Owner == "" || admission.Validate == nil || admission.Execute == nil || !admission.ExpiresAt.After(time.Now()) {
		return admission, errors.New("project grant required")
	}
	return admission, admission.Validate(ctx)
}
