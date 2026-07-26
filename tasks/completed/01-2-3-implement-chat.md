# Task 1.2.3 — Implement Chat

## Milestone
Milestone 1: Model Abstraction & LM Studio Provider

## Description
Implement Chat method that calls LM Studio's /v1/chat/completions endpoint.

## Checklist
- [x] Implement `Chat()` that calls `POST /v1/chat/completions`
- [x] Serialize request to JSON
- [x] Parse response JSON into `ChatResponse`
- [x] Handle non-200 status codes
- [x] Handle streaming flag (return error for now, support later)
- [x] Add context timeout (e.g., 5 minutes)

## Dependencies
- Task 1.2.1, Task 1.1.2

## Deliverables
- Working Chat implementation

## Status
- [x] Done
