# 58 — Retire the legacy candidate Desktop UI

## Status

Complete

## Goal

Remove the unreachable whole-file candidate presentation so the supported
file-scoped declaration draft is the Desktop client's only editing workflow.

## Depends on

Task 57.

## Implementation

- Reconfirm the current production send path uses file-scoped chat sessions and
  declaration drafts before deleting anything.
- Delete the legacy candidate review pane and candidate branch from the Desktop
  content host.
- Remove candidate generation, alternate comparison, candidate export/check/apply,
  activity-only review wiring, duplicate applied state, and obsolete local UI fields.
- Remove candidate-only Desktop state/events/controller helpers, eligibility logic,
  API-client methods, models, and tests when no supported Desktop path consumes them.
- Remove the prompt-template implementation/tests if its preparation function is
  still unreferenced; preserve supported prepared Bug/AI suggestion requests.
- Keep draft validation/check/apply models and API calls. Do not change daemon routes
  or external API contracts in this task.
- Delete superseded files and call sites in the same change; do not leave a hidden
  compatibility branch.

## Acceptance criteria

- The Desktop client has one supported chat → draft → validation → checks → Apply path.
- No production Desktop request calls the retired `/api/chat/message` endpoint.
- File/symbol selection, prepared finding fixes, draft editing, Apply, and Undo still work.
- All remaining state and tests describe the declaration-draft workflow truthfully.

## Verification

- Search for candidate-only Desktop call sites and prove remaining uses are required.
- Run `./desktop/gradlew -p desktop test`, `make check`, and `git diff --check`.
