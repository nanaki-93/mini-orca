# Task 2.4.2 — Implement Coder Execution

## Milestone
Milestone 2: Multi-Agent Architecture with Skills

## Description
Implement the coder agent's Execute method.

## Checklist
- [ ] Implement `Execute()` method:
  - Receive atomic unit description as input
  - Build system prompt with coder skills
  - Call `router.Chat()` with coding phase config
  - Parse response into code
  - Return `AgentResult` with code output
- [ ] Enforce ONE atomic unit per execution (function, struct, or class)
- [ ] Include context: existing code, plan reference

## Dependencies
- Task 2.4.1

## Deliverables
- Working coder Execute method

## Status
- [ ] Not Started
- [ ] In Progress
- [ ] Done
