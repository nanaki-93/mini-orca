# 44 — Introduce desktop workspace and workflow state

## Status

Complete

## Goal

Make asynchronous project, file, findings, chat, and draft behavior testable and resistant to stale responses.

## Depends on

Tasks 42 and 43.

## Implementation

- Replace integer tabs with explicit Summary, Analysis, Bugs, and Editor workspace state.
- Split application state into cohesive project, selection/brief, jobs, findings, chat, draft/review, and connection sections with one small controller.
- Guard async results by project revision, path, content hash, and request identity; cancel or ignore results for old selections.
- Load selected-file source and deterministic symbols first, then load cached analysis, impact, and Git independently so optional failure cannot block the source.
- Centralize draft validity and Apply eligibility outside composable-local expressions.

## Acceptance criteria

- Late responses cannot replace the current project, open file, brief, session, or draft.
- Changing files clears or stales file-bound chat/draft state and never carries it into the new file.
- Workspace navigation preserves the open file when safe.
- A failed semantic-analysis, impact, or Git request does not prevent source display.

## Verification

- Add reducer/controller tests for stale responses, cancellation, partial file load, project switch, and Apply eligibility.
- Run `./desktop/gradlew -p desktop test` and `git diff --check`.
