# Task 1.3.3 — Implement Config Loader

## Milestone
Milestone 1: Model Abstraction & LM Studio Provider

## Description
Implement YAML config loading in `internal/config/config.go`.

## Checklist
- [ ] Implement `LoadConfig(path string) (*Config, error)`
- [ ] Use `gopkg.in/yaml.v3` for YAML parsing
- [ ] Handle file not found error
- [ ] Handle YAML parse errors
- [ ] Validate required fields (base_url, phases)
- [ ] Return sensible defaults for optional fields

## Dependencies
- Task 1.3.2

## Deliverables
- Working LoadConfig function

## Status
- [ ] Not Started
- [ ] In Progress
- [ ] Done
