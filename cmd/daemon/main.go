package main

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/api/handlers"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/logging"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"github.com/nanaki-93/mini-orca/v2/internal/version"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		logging.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// Initialize logger
	logging.Init(logging.Config{
		Level:         cfg.Logging.Level,
		Format:        cfg.Logging.Format,
		Filename:      cfg.Logging.Filename,
		SensitiveKeys: cfg.Logging.SensitiveKeys,
	})

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		logging.Error("Invalid configuration", "error", err)
		os.Exit(1)
	}

	projectManager, err := project.NewManager(cfg.ProjectPath)
	if err != nil {
		logging.Error("Invalid initial project path", "error", err)
		os.Exit(1)
	}

	application, err := app.New(cfg, projectManager)
	if err != nil {
		logging.Error("Failed to initialize application service", "error", err)
		os.Exit(1)
	}

	// Log successful startup
	logging.Info("Mini-Orca daemon started successfully")
	for _, model := range application.EffectiveModels() {
		logging.Info("Model scope initialized", "scope", model.Scope, "model", model.Model, "reasoning_effort", model.ReasoningEffort, "provider_origin", model.ProviderOrigin, "remote_provider", model.RemoteProvider)
	}
	logging.Info("Workflow: single-coder preview with explicit review")

	// Start HTTP server with all API endpoints
	server := startHTTPServer(application, projectManager)

	// Wait for shutdown signal
	quit := waitForShutdown()
	logging.Info("Received shutdown signal", "signal", quit)
	application.CancelChangeWorkflows()

	// Shutdown HTTP server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logging.Error("HTTP server shutdown error", "error", err)
	}
	if err := application.ShutdownChangeWorkflows(shutdownCtx); err != nil {
		logging.Error("Change workflow shutdown error", "error", err)
	}

	logging.Info("Shutdown complete")
}

// startHTTPServer creates and starts the HTTP server with all API endpoints.
func startHTTPServer(
	application *app.Service,
	projectManager *project.Manager,
) *http.Server {
	mux := newHTTPMux(application, projectManager)

	server := &http.Server{
		Addr:         daemonAddress(),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 6 * time.Minute,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		logging.Info("HTTP server starting", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logging.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	return server
}

// daemonAddress is loopback-only unless a local deployment explicitly opts in
// to another bind address (for example, a container port mapping).
func daemonAddress() string {
	if address := os.Getenv("MINI_ORCA_BIND_ADDRESS"); address != "" {
		return address
	}
	return "127.0.0.1:9090"
}

// newHTTPMux registers only the local daemon API routes.
func newHTTPMux(
	application *app.Service,
	projectManager *project.Manager,
) http.Handler {
	current := newProjectHTTPMux(application, projectManager)
	worktrees := &worktreeRouter{}
	return api.LocalOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			api.WriteAppError(w, api.BadRequest("invalid request query", "Provide valid URL-encoded query parameters.", err))
			return
		}
		paths, scoped := query["worktree_path"]
		if !scoped {
			current.ServeHTTP(w, r)
			return
		}
		if len(paths) != 1 || paths[0] == "" {
			api.WriteAppError(w, api.BadRequest("invalid worktree scope", "Provide worktree_path once with an absolute registered worktree path.", nil))
			return
		}
		if r.URL.Path != "/api/projects/current" && !strings.HasPrefix(r.URL.Path, "/api/projects/current/") {
			api.WriteError(w, http.StatusBadRequest, "worktree scope requires a project endpoint")
			return
		}
		service, manager, err := application.WorktreeService(r.Context(), paths[0])
		if err != nil {
			api.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		// Worktree scope belongs to routing, not the endpoint's identity guards or filters.
		query.Del("worktree_path")
		scopedRequest := r.Clone(r.Context())
		scopedRequest.URL.RawQuery = query.Encode()
		worktrees.handler(service, manager).ServeHTTP(w, scopedRequest)
	}), daemonAllowsExternalHosts())
}

func newProjectHTTPMux(application *app.Service, projectManager *project.Manager) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/projects/current", func(w http.ResponseWriter, r *http.Request) {
		analysis, err := projectManager.Analysis()
		if err != nil {
			api.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		api.WriteJSON(w, http.StatusOK, analysis)
	})

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","version":"` + version.Version + `"}`))
	})

	// Status endpoint
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"running","version":"` + version.Version + `","workflow":"shared_proposals"}`))
	})

	changeHandler := handlers.NewChangeHandler(application, projectManager)
	agentHandler := handlers.NewAgentRunHandler(application)
	memoryHandler := handlers.NewAgentMemoryHandler(application)
	integrationHandler := handlers.NewAgentIntegrationsHandler(application)
	mux.HandleFunc("GET /api/projects/current/integrations", integrationHandler.List)
	mux.HandleFunc("POST /api/projects/current/integrations/{integrationID}/control", integrationHandler.Control)
	apiHandler := handlers.NewAgentAPIsHandler(application)
	mux.HandleFunc("GET /api/projects/current/apis", apiHandler.List)
	mux.HandleFunc("POST /api/projects/current/apis/{integrationID}/control", apiHandler.Control)
	mux.HandleFunc("GET /api/projects/current/api-actions", apiHandler.Actions)
	mux.HandleFunc("POST /api/projects/current/api-actions/{actionID}/control", apiHandler.ControlAction)
	researchHandler := handlers.NewAgentResearchHandler(application)
	delegationHandler := handlers.NewAgentDelegationHandler(application)
	mux.HandleFunc("GET /api/projects/current/remote-agents", delegationHandler.Remotes)
	mux.HandleFunc("POST /api/projects/current/remote-agents/{integrationID}/control", delegationHandler.ControlRemote)
	mux.HandleFunc("GET /api/projects/current/delegations", delegationHandler.List)
	mux.HandleFunc("POST /api/projects/current/delegations", delegationHandler.Start)
	mux.HandleFunc("GET /api/projects/current/delegations/{delegationID}", delegationHandler.Detail)
	mux.HandleFunc("POST /api/projects/current/delegations/{delegationID}/control", delegationHandler.Control)
	mux.HandleFunc("GET /api/projects/current/research", researchHandler.Overview)
	mux.HandleFunc("POST /api/projects/current/research", researchHandler.Run)
	mux.HandleFunc("GET /api/projects/current/research/{reportID}/report", researchHandler.Report)
	mux.HandleFunc("GET /api/projects/current/memory", memoryHandler.List)
	mux.HandleFunc("POST /api/projects/current/memory", memoryHandler.Mutate)
	mux.HandleFunc("GET /api/projects/current/memory/{memoryID}", memoryHandler.Detail)
	mux.HandleFunc("POST /api/projects/current/memory/{memoryID}", memoryHandler.Mutate)
	mux.HandleFunc("GET /api/projects/current/agent-runs", agentHandler.List)
	mux.HandleFunc("GET /api/projects/current/agent-runs/events", agentHandler.Events)
	mux.HandleFunc("GET /api/projects/current/agent-runs/{runID}", agentHandler.Detail)
	mux.HandleFunc("POST /api/projects/current/agent-runs/{runID}/control", agentHandler.Control)
	mux.HandleFunc("POST /api/projects/current/agent-runs/{runID}/input", agentHandler.Input)
	mux.HandleFunc("POST /api/projects/current/plans/investigate", changeHandler.InvestigatePlan)
	gitHandler := handlers.NewGitHandler(application)
	mux.HandleFunc("GET /api/projects/current/git/repository", gitHandler.Repository)
	mux.HandleFunc("GET /api/projects/current/git/workspace", gitHandler.Workspace)
	mux.HandleFunc("POST /api/projects/current/git/worktrees", gitHandler.CreateWorktree)
	mux.HandleFunc("POST /api/projects/current/git/switch", gitHandler.Switch)
	mux.HandleFunc("POST /api/projects/current/git/merge-preview", gitHandler.PreviewMerge)
	mux.HandleFunc("POST /api/projects/current/git/merge", gitHandler.Merge)
	mux.HandleFunc("POST /api/projects/current/git/cleanup-merged", gitHandler.CleanupMergedWorktree)
	mux.HandleFunc("POST /api/projects/current/git/abort-merge", gitHandler.AbortMerge)
	mux.HandleFunc("POST /api/projects/current/git/discard-worktree", gitHandler.DiscardWorktree)
	mux.HandleFunc("GET /api/projects/current/changes", changeHandler.List)
	mux.HandleFunc("POST /api/projects/current/changes", changeHandler.Open)
	mux.HandleFunc("GET /api/projects/current/changes/recovery", changeHandler.Recovery)
	mux.HandleFunc("GET /api/projects/current/changes/{sessionID}", changeHandler.Get)
	mux.HandleFunc("POST /api/projects/current/changes/{sessionID}/resume", changeHandler.Resume)
	mux.HandleFunc("POST /api/projects/current/changes/{sessionID}/messages", changeHandler.Message)
	mux.HandleFunc("POST /api/projects/current/changes/{sessionID}/checks", changeHandler.Checks)
	mux.HandleFunc("POST /api/projects/current/changes/{sessionID}/workflow", changeHandler.StartWorkflow)
	mux.HandleFunc("POST /api/projects/current/changes/{sessionID}/workflow/cancel", changeHandler.CancelWorkflow)
	mux.HandleFunc("POST /api/projects/current/changes/{sessionID}/workflow/input", changeHandler.RespondInput)
	mux.HandleFunc("POST /api/projects/current/changes/{sessionID}/review", changeHandler.Review)
	mux.HandleFunc("POST /api/projects/current/changes/{sessionID}/review-file", changeHandler.ReviewFile)
	mux.HandleFunc("POST /api/projects/current/changes/{sessionID}/plan", changeHandler.Plan)
	mux.HandleFunc("POST /api/projects/current/changes/{sessionID}/apply", changeHandler.Apply)
	mux.HandleFunc("POST /api/projects/current/changes/{sessionID}/commit", changeHandler.Commit)
	mux.HandleFunc("POST /api/projects/current/changes/{sessionID}/undo", changeHandler.Undo)
	mux.HandleFunc("POST /api/projects/current/changes/{sessionID}/verify", changeHandler.Verify)
	mux.HandleFunc("GET /api/projects/current/instructions", changeHandler.Instructions)
	mux.HandleFunc("POST /api/projects/current/instructions/proposal", changeHandler.ProposeInstructions)
	featureHandler := handlers.NewFeatureHandler(application, projectManager)
	mux.HandleFunc("GET /api/projects/current/features", featureHandler.Get)
	mux.HandleFunc("POST /api/projects/current/features/goals", featureHandler.Goals)
	mux.HandleFunc("POST /api/projects/current/features/generate", featureHandler.Generate)
	mux.HandleFunc("PATCH /api/projects/current/features/{featureID}", featureHandler.Status)

	modelHandler := handlers.NewModelHandler(application)
	mux.HandleFunc("GET /api/models/current", modelHandler.Current)
	mux.HandleFunc("GET /api/models/available", modelHandler.Available)
	contextHandler := handlers.NewContextHandler(application)
	mux.HandleFunc("GET /api/projects/current/context", contextHandler.Preview)

	analysisHandler := handlers.NewAnalysisHandler(application)
	mux.HandleFunc("GET /api/projects/current/analysis/selection", analysisHandler.Selection)
	mux.HandleFunc("POST /api/projects/current/analysis/selection", analysisHandler.SaveSelection)
	mux.HandleFunc("POST /api/projects/current/analysis/preview", analysisHandler.Preview)
	mux.HandleFunc("POST /api/projects/current/analysis/run", analysisHandler.Start)
	mux.HandleFunc("GET /api/projects/current/analysis/run", analysisHandler.Current)
	mux.HandleFunc("POST /api/projects/current/analysis/run/control", analysisHandler.Control)
	mux.HandleFunc("GET /api/projects/current/analysis/results", analysisHandler.Results)

	projectHandler := handlers.NewProjectHandler(projectManager, application)
	mux.HandleFunc("POST /api/projects/import", projectHandler.Import)
	mux.HandleFunc("POST /api/projects/restore", projectHandler.Restore)
	mux.HandleFunc("GET /api/projects/current/overview", projectHandler.Overview)
	mux.HandleFunc("GET /api/projects/current/findings", projectHandler.Findings)
	mux.HandleFunc("PATCH /api/projects/current/findings/{findingID}", projectHandler.UpdateFindingStatus)
	mux.HandleFunc("GET /api/projects/current/scan", projectHandler.GoScanProgress)
	mux.HandleFunc("POST /api/projects/current/scan", projectHandler.StartGoScan)
	mux.HandleFunc("DELETE /api/projects/current/scan", projectHandler.CancelGoScan)
	mux.HandleFunc("GET /api/projects/current/execution-trust", projectHandler.ExecutionTrust)
	mux.HandleFunc("POST /api/projects/current/execution-trust", projectHandler.TrustProjectExecution)
	mux.HandleFunc("POST /api/projects/current/bootstrap/exec", projectHandler.BootstrapExec)
	mux.HandleFunc("GET /api/projects/current/index", projectHandler.Index)
	mux.HandleFunc("GET /api/projects/current/files/info", projectHandler.FileInfo)
	mux.HandleFunc("GET /api/projects/current/files/symbols", projectHandler.Symbols)
	mux.HandleFunc("GET /api/projects/current/impact", projectHandler.Impact)
	mux.HandleFunc("GET /api/projects/current/git", projectHandler.GitStatus)
	mux.HandleFunc("GET /api/projects/current/files/analysis", projectHandler.FileAnalysis)
	mux.HandleFunc("POST /api/projects/current/files/analysis", projectHandler.AnalyzeFile)
	mux.HandleFunc("POST /api/projects/current/files/security-scan", projectHandler.ScanSecurityFile)
	mux.HandleFunc("POST /api/projects/current/files/explanation", projectHandler.ExplainDeclaration)
	mux.HandleFunc("POST /api/projects/current/security-review", projectHandler.ReviewSecurityFile)
	mux.HandleFunc("POST /api/projects/current/reindex", projectHandler.Reindex)
	mux.HandleFunc("GET /api/projects/current/events", projectHandler.Events)

	return mux
}

func daemonAllowsExternalHosts() bool {
	address := os.Getenv("MINI_ORCA_BIND_ADDRESS")
	if address == "" {
		return false
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	if host == "" {
		return true
	}
	if strings.EqualFold(host, "localhost") {
		return false
	}
	ip := net.ParseIP(host)
	return ip == nil || !ip.IsLoopback()
}

// waitForShutdown blocks until a SIGINT or SIGTERM signal is received.
func waitForShutdown() os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	signal.Stop(sigChan)
	return sig
}
