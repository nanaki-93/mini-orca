# Task 1.1.3 — Define Provider Interface

## Milestone
Milestone 1: Model Abstraction & LM Studio Provider

## Description
Define the Provider interface in `internal/model/provider.go`.

## Checklist
- [ ] Define `Provider` interface with methods:
  - `ListModels(ctx context.Context) ([]Model, error)`
  - `Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)`
  - `Name() string`
  - `IsStreamingSupported() bool` (optional, for future)

## Dependencies
- Task 1.1.2

## Deliverables
- `internal/model/provider.go` with Provider interface

## Status
- [ ] Not Started
- [ ] In Progress
- [ ] Done
