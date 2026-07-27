# Task 1.4.4 — Test Full Flow

## Milestone
Milestone 1: Model Abstraction & LM Studio Provider

## Description
End-to-end testing of the model router integration.

## Checklist
- [x] Run daemon with config
- [x] Verify Router is initialized correctly
- [x] Verify agent calls go through router
- [x] Test with actual LM Studio instance

## Dependencies
- Task 1.4.3

## Deliverables
- Verified end-to-end flow working

## Bugs Fixed During Testing
- `Router.GetPhaseConfig()` — top-level `Temperature`/`MaxTokens` fields on `PhaseConfig` were not populated from embedded `ModelConfig`. Fixed by copying values explicitly.
- `AgentClient.ListModels()` — passed `nil` context to `router.RouteListModels()`. Fixed to use `context.Background()`.

## Status
- [x] Done
