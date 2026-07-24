// Package agent provides the multi-agent architecture for Mini-Orca.
//
// Mini-Orca uses four specialized agents that work together to plan,
// implement, test, and review code changes:
//
//   - Planner: Analyzes requirements and breaks them into atomic units
//   - Coder: Implements exactly one atomic unit per execution
//   - Tester: Creates and runs tests for the implemented code
//   - Reviewer: Reviews code quality, style, and correctness
//
// # Architecture
//
// Each agent is defined by the Agent interface and registered in a
// Registry. Agents are configured with a set of skills (knowledge or tool)
// that influence their behavior through prompt engineering.
//
// # Skills System
//
// Skills are reusable capabilities that can be attached to any agent.
// There are two types:
//   - knowledge: Principles and guidelines (e.g., SOLID, Clean Code)
//   - tool: Concrete capabilities (e.g., function_generation, unit_testing)
//
// Skills are configured per-agent in config.yaml and are injected into
// the agent's prompt context when executing.
//
// # Example Usage
//
//	registry := agent.NewRegistry()
//	registry.Register(planner.NewPlanner(router))
//	registry.Register(coder.NewCoder(router))
//
//	agentInst, _ := registry.Get("planner")
//	result, err := agentInst.Execute(ctx, "Create a user authentication service")
package agent
