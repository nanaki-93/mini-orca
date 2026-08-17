# 09 — Extract exact Go symbols

## Status

Complete

## Goal

Provide reliable atomic targets and facts for Go files.

## Depends on

Task 08.

## Implementation

- Use `go/parser` and `go/ast`, not regex, to collect functions, methods, types, interfaces, imports, signatures, visibility, and line ranges.
- Define canonical symbol identifiers such as `Type.Method` and package-level function names.
- Mark supported edit targets and distinguish types from constants/variables.
- Capture parse errors as index diagnostics without failing the whole project index.

## Acceptance criteria

- Symbol picker locations match Go source lines.
- Methods, generic declarations, interfaces, and malformed files are handled predictably.

## Verification

- Add parser fixtures for functions, methods, structs, interfaces, generics, comments, and syntax errors.
