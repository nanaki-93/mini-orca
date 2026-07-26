# Task 1.1.3 — Define Provider Interface

## Milestone
Milestone 1: Model Abstraction & LM Studio Provider

## Description
Define the Provider interface in `internal/model/provider.go`.

## Checklist
- [x] Define `Provider` interface with methods:
  - [x] `ListModels(ctx context.Context) ([]Model, error)`
  - [x] `Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)`
  - [x] `Name() string`
  - [x] `IsStreamingSupported() bool` (optional, for future)

## Dependencies
- Task 1.1.2

## Deliverables
- `internal/model/provider.go` with Provider interface

## Status
- [x] Done
