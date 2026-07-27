# Task 1.3.3 — Implement Config Loader

## Milestone
Milestone 1: Model Abstraction & LM Studio Provider

## Description
Implement YAML config loading in `internal/config/config.go`.

## Checklist
- [x] Implement `LoadConfig(path string) (*Config, error)`
- [x] Use `gopkg.in/yaml.v3` for YAML parsing
- [x] Handle file not found error
- [x] Handle YAML parse errors
- [x] Validate required fields (base_url, phases)
- [x] Return sensible defaults for optional fields

## Dependencies
- Task 1.3.2

## Deliverables
- Working LoadConfig function

## Status
- [x] Done
