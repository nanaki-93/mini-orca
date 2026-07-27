# Task 1.4.1 — Audit Existing LLM Calls

## Milestone
Milestone 1: Model Abstraction & LM Studio Provider

## Description
Find and document all existing LLM call sites in the codebase.

## Checklist
- [x] Find all places in codebase that call LLM
- [x] Document each call site with its purpose
- [x] Identify hardcoded model names

## Dependencies
- None (can start early)

## Deliverables
- List of all LLM call sites to be migrated

## Audit Findings

The codebase contains **no existing LLM call sites** to migrate. The only LLM-related code is the abstraction layer built in Milestone 1:

### Existing LLM-Related Code (Abstraction Layer — No Migration Needed)
| File | Function/Type | Purpose |
|---|---|---|
| `internal/model/provider.go` | `Provider` interface | Abstraction for all LLM providers |
| `internal/model/lm_studio.go` | `LMStudioProvider.Chat()` | Calls `POST /v1/chat/completions` on LM Studio |
| `internal/model/lm_studio.go` | `LMStudioProvider.ListModels()` | Calls `GET /v1/models` on LM Studio |
| `internal/model/router.go` | `Router.Chat()` | Routes chat requests to the correct provider |
| `internal/model/router.go` | `Router.RouteChat()` | Routes chat with context to the correct provider |
| `internal/model/router.go` | `Router.RouteListModels()` | Routes model listing to the active provider |

### Hardcoded Model Names
- **None found.** All model names are configured via `ModelConfig.ModelID` (empty string = provider default).

### Hardcoded Provider URLs
- `DefaultProviderURL = "http://localhost:1234"` in `internal/config/default.go` — this is a **configuration default**, not a hardcoded call site.

### Conclusion
No migration is needed. The codebase is a greenfield project with only the model abstraction layer. Future agent implementations will consume the `Provider` interface and `Router` rather than calling LLM APIs directly.

## Status
- [x] Done
