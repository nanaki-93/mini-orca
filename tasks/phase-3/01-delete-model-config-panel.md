# Task 3.1: Delete model-config-panel.html

## Goal
Delete the model configuration panel component.

## Files to DELETE
- `internal/api/templates/components/model-config-panel.html`

## Implementation
```bash
rm internal/api/templates/components/model-config-panel.html
```

## Prerequisites
- Task 1.12 (HTML templates update) should have already removed references

## Verification
- File is deleted
- `go build ./...` succeeds
