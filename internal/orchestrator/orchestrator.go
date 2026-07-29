// Package orchestrator provides the main orchestration logic for the Mini-Orca pipeline.
package orchestrator

import (
	"context"

	"github.com/nanaki-93/mini-orca/internal/agent"
	"github.com/nanaki-93/mini-orca/internal/config"
	"github.com/nanaki-93/mini-orca/internal/model"
	"github.com/nanaki-93/mini-orca/internal/state"
	"github.com/nanaki-93/mini-orca/internal/tools"
)

// Session represents a single execution session within the pipeline.
type Session struct {
	// ID is the unique identifier for this session.
	ID string
	// Phase is the current phase of the session.
	Phase Phase
	// CreatedAt is when the session was created.
	CreatedAt string
}

// Orchestrator manages the execution flow of the multi-phase development pipeline.
type Orchestrator struct {
	agents         map[string]agent.Agent
	router         *model.Router
	executor       tools.ToolExecutor
	stateStore     *state.Store
	config         *config.Config
	currentPhase   Phase
	currentSession *Session
	ctx            context.Context
	cancel         context.CancelFunc
	humanGate      *HumanGate
}

// New creates a new Orchestrator instance.
func New() *Orchestrator {
	// TODO: implement orchestrator initialization
	return &Orchestrator{}
}

// Run executes the pipeline from start to finish.
func (o *Orchestrator) Run() error {
	// TODO: implement pipeline execution
	return nil
}
