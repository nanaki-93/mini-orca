package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"mini-orca/internal/agent"
	"mini-orca/internal/config"
	"mini-orca/internal/model"
	"mini-orca/internal/state"
	"mini-orca/internal/tools"
)

// Orchestrator manages the 5-phase workflow.
type Orchestrator struct {
	session    *state.Session
	plan       *state.Plan
	router     *model.Router
	agents     *agent.Registry
	tools      *tools.Executor
	fileOps    *tools.FileOps
	gitOps     *tools.GitOps
	formatter  *tools.Formatter
	stateStore *state.Store
	config     *config.Config

	mu           sync.RWMutex
	isRunning    bool
	currentUnit  string
	cancelFunc   context.CancelFunc
	ctx          context.Context

	// Retry configuration
	maxRetries   int
	retryDelay   time.Duration
}

// NewOrchestrator creates a new orchestrator.
func NewOrchestrator(
	session *state.Session,
	router *model.Router,
	agents *agent.Registry,
	tools *tools.Executor,
	fileOps *tools.FileOps,
	gitOps *tools.GitOps,
	formatter *tools.Formatter,
	stateStore *state.Store,
	config *config.Config,
) *Orchestrator {
	return &Orchestrator{
		session:    session,
		router:     router,
		agents:     agents,
		tools:      tools,
		fileOps:    fileOps,
		gitOps:     gitOps,
		formatter:  formatter,
		stateStore: stateStore,
		config:     config,
		maxRetries: 3,
		retryDelay: 2 * time.Second,
	}
}

// Start begins the orchestration workflow.
func (o *Orchestrator) Start(ctx context.Context) error {
	o.mu.Lock()
	if o.isRunning {
		o.mu.Unlock()
		return fmt.Errorf("orchestrator is already running")
	}
	o.isRunning = true

	o.ctx, o.cancelFunc = context.WithCancel(ctx)
	o.mu.Unlock()

	o.session.AddEvent(state.NewEvent("", "start", o.session.Phase, "Orchestrator started"))

	// Phase 1: Planning
	if err := o.runPlanningPhase(o.ctx); err != nil {
		o.session.AddEvent(state.NewEvent("", "error", o.session.Phase, fmt.Sprintf("Planning failed: %v", err)))
		return fmt.Errorf("planning phase failed: %w", err)
	}

	// Wait for human approval (blocking)
	if err := o.waitForHumanApproval(o.ctx, state.PhasePlanningReview); err != nil {
		return fmt.Errorf("human approval failed: %w", err)
	}

	// Phase 2-4: Coding, Testing, Review loop
	for {
		select {
		case <-o.ctx.Done():
			return nil
		default:
			if err := o.runCodingPhase(o.ctx); err != nil {
				o.session.AddEvent(state.NewEvent("", "error", o.session.Phase, fmt.Sprintf("Coding failed: %v", err)))
				return fmt.Errorf("coding phase failed: %w", err)
			}

			if err := o.runTestingPhase(o.ctx); err != nil {
				o.session.AddEvent(state.NewEvent("", "error", o.session.Phase, fmt.Sprintf("Testing failed: %v", err)))
				return fmt.Errorf("testing phase failed: %w", err)
			}

			if err := o.runReviewPhase(o.ctx); err != nil {
				o.session.AddEvent(state.NewEvent("", "error", o.session.Phase, fmt.Sprintf("Review failed: %v", err)))
				return fmt.Errorf("review phase failed: %w", err)
			}
		}
	}
}

// Stop stops the orchestrator.
func (o *Orchestrator) Stop() {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.cancelFunc != nil {
		o.cancelFunc()
	}
	o.isRunning = false
	o.session.AddEvent(state.NewEvent("", "stop", o.session.Phase, "Orchestrator stopped"))
}

// GetSession returns the current session.
func (o *Orchestrator) GetSession() *state.Session {
	return o.session
}

// GetPlan returns the current plan.
func (o *Orchestrator) GetPlan() *state.Plan {
	return o.plan
}

// RunPhase runs a specific phase manually.
func (o *Orchestrator) RunPhase(ctx context.Context, phase state.Phase) error {
	switch phase {
	case state.PhasePlanning:
		return o.runPlanningPhase(ctx)
	case state.PhaseCoding:
		return o.runCodingPhase(ctx)
	case state.PhaseTesting:
		return o.runTestingPhase(ctx)
	case state.PhaseReview:
		return o.runReviewPhase(ctx)
	default:
		return fmt.Errorf("manual phase execution not supported for: %s", phase)
	}
}

// ExecuteAgent runs an agent with retry logic.
func (o *Orchestrator) ExecuteAgent(ctx context.Context, agentName string, input string) (*agent.ExecutionResult, error) {
	var lastErr error

	for attempt := 0; attempt <= o.maxRetries; attempt++ {
		if attempt > 0 {
			o.session.AddEvent(state.NewEvent("", "retry", o.session.Phase,
				fmt.Sprintf("Retry %d/%d for agent %s", attempt, o.maxRetries, agentName)))
			time.Sleep(o.retryDelay * time.Duration(attempt)) // Exponential backoff
		}

		agentInst, err := o.agents.Get(agentName)
		if err != nil {
			return nil, fmt.Errorf("agent not found: %w", err)
		}

		result, err := agentInst.Execute(ctx, input)
		if err != nil {
			lastErr = err
			continue
		}

		return result, nil
	}

	return nil, fmt.Errorf("agent %s failed after %d retries: %w", agentName, o.maxRetries, lastErr)
}
