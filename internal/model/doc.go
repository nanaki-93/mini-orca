// Package model provides the model abstraction layer for Mini-Orca.
//
// The model package defines a Provider interface that abstracts away
// the specific LLM provider implementation. This allows switching
// between providers (LM Studio, OpenAI, etc.) without changing
// the rest of the codebase.
//
// # Architecture
//
// The model package uses a Router pattern to route requests to
// different providers based on the current workflow phase:
//
//	+-------------+     +-----------+     +--------------+
//	| Orchestrator|---->|   Router  |---->|  Provider    |
//	| (Planning)  |     | (Phase    |     | (LM Studio)  |
//	+-------------+     |  routing) |     +--------------+
//	                     +-----------+
//
// # Provider Interface
//
// All providers must implement:
//   - ListModels() ([]Model, error)
//   - Chat(req ChatRequest) (*ChatResponse, error)
//
// # Phase-Based Configuration
//
// Each phase can use a different model, temperature, and provider:
//
//	phases:
//	  planning:
//	    provider: "lm-studio"
//	    model: "qwen3-coder-30b"
//	    temperature: 0.3   # Higher for creativity
//	  coding:
//	    provider: "lm-studio"
//	    model: "qwen3-coder-30b"
//	    temperature: 0.1   # Lower for precision
//
// # LM Studio Provider
//
// The LM Studio provider implements the Provider interface and connects
// to a local LM Studio instance via its OpenAI-compatible API:
//
//	POST /v1/chat/completions
//	GET  /v1/models
//
// # Example
//
//	router := model.NewRouter(config.Models)
//	router.RegisterProvider("lm-studio", model.NewLMStudioProvider(url, key))
//
//	modelCfg, _ := router.GetModelConfigForPhase(state.PhasePlanning)
//	resp, err := router.Chat(ctx, modelCfg, messages)
package model
