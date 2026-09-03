# 99 — Make Desktop model state scope-aware

## Status

 Complete

## Goal

Show and confirm the correct effective model destination for Analyze, Bugs, and
function-edit actions while retaining the current Desktop workflow and layout.

## Depends on

Task 98.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 99` and name Desktop
  API models, scoped confirmation state, project-import confirmation, responsive UI,
  and focused tests.
- After verification, post a separate update beginning with `Task 99 complete` and
  state the behavior, commands/results, changed files, and that Task 100 is next.

## Implementation

- Extend the Kotlin API model for `/api/models/current` with defaulted scope entries so
  the client remains tolerant of the legacy response.
- Replace the single `remoteProvider` interpretation with explicit `analyze`, `bug`,
  and `function` presentation state. Keep this local to existing Desktop state and
  composables; do not add a settings store or model picker.
- Display concise model/destination metadata where its action already lives:
  - project-open/analysis confirmation for `analyze`;
  - Analysis/Analyze-all controls for `bug`;
  - Editor composer and context inspector for `function`.
- Add remote confirmation to project import and send the existing API request field.
  Restore remains model-free and must not show or require confirmation.
- Keep separate confirmation values per scope. Reset them when the effective model
  catalog changes or the daemon reconnects to a different profile set.
- Determine confirmation from the model provider's `remote_provider` value. Remove the
  chat guard that infers LLM locality from the daemon API endpoint.
- Keep server rejection handling clear if Desktop state is stale.
- Preserve keyboard navigation, responsive drawers, read-only source/diff, and all
  existing operation/identity guards.

## Acceptance criteria

- Mixed cloud/cloud/local configuration shows the right model and destination in each
  workspace.
- Import can explicitly confirm a remote `analyze` provider and sends no source before
  confirmation.
- Remote `bug` confirmation gates Analyze and Analyze-all but not verified scans.
- Local `function` generation is not blocked because `analyze` or `bug` is remote.
- Online `function` generation requires its own confirmation and uses provider
  locality, not daemon endpoint locality.
- Restore, browsing, filtering, navigation, validation, checks, review, Apply, and Undo
  remain model-free and ungated.
- Legacy model responses degrade to the current single-profile presentation.

## Verification

- Extend `ApiClientContractTest`, Desktop state/controller tests, Analysis and Editor
  presentation tests, no-project landing tests, and accessibility tests.
- Cover all-local, mixed, all-remote, catalog-change, legacy-response, import, restore,
  and stale-server-rejection cases.
- Run:

  ```text
  ./desktop/gradlew -p desktop test
  git diff --check
  ```

## Completion

Only after all criteria pass, mark this task Complete, move it to `tasks/completed/`,
and update `tasks/INDEX.md`. Do not stage or commit unless the user separately asks.
