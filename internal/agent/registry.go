package agent

import (
	"fmt"
	"sync"

	"github.com/nanaki-93/mini-orca/v2/internal/logging"
	"github.com/nanaki-93/mini-orca/v2/internal/model"
)

// Registry manages the lifecycle and lookup of agents in the multi-agent system.
// It provides thread-safe registration and retrieval of agents by name.
type Registry struct {
	mu     sync.RWMutex
	agents map[string]Agent // keyed by agent name
}

// NewRegistry creates a new empty agent registry.
func NewRegistry() *Registry {
	return &Registry{
		agents: make(map[string]Agent),
	}
}

// Register adds an agent to the registry under the given name.
// It returns an error if an agent with the same name is already registered.
func (r *Registry) Register(name string, agent Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if agent.Name() != name {
		return fmt.Errorf("agent: agent name %q does not match registration name %q", agent.Name(), name)
	}

	if _, exists := r.agents[name]; exists {
		return fmt.Errorf("agent: agent with name %q already registered", name)
	}

	r.agents[name] = agent

	return nil
}

// Get retrieves an agent by name.
// It returns an error if the agent is not found.
func (r *Registry) Get(name string) (Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agent, exists := r.agents[name]
	if !exists {
		return nil, fmt.Errorf("agent: agent %q not found", name)
	}

	return agent, nil
}

// List returns the names of all registered agents.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.agents))
	for name := range r.agents {
		names = append(names, name)
	}

	return names
}

// FromConfig creates and registers an agent from the given AgentConfig.
// The agent is instantiated using the provided model router.
func (r *Registry) FromConfig(cfg model.AgentConfig, router *model.Router) error {
	a := NewClient(router)
	a.name = cfg.Name
	a.description = cfg.Description
	a.phase = cfg.Phase

	return r.Register(cfg.Name, a)
}

// DefaultAgentNames returns the names of the four pre-registered default agents.
func DefaultAgentNames() []string {
	return []string{"planner", "coder", "tester", "reviewer"}
}

// InitAgentRegistry creates an agent registry and registers all agents that implement the Agent interface.
func InitAgentRegistry(router *model.Router) *Registry {
	registry := NewRegistry()

	// Create and register coder agent
	coder := NewCoderAgent(router, nil)
	if err := registry.Register(coder.Name(), coder); err != nil {
		logging.Warn("Failed to register coder agent", "error", err)
	}

	logging.Info("Agent registry initialized", "count", len(registry.List()))
	return registry
}
