# 168 — Flatten Context, Assistant, and Review safely

## Status

Pending

## Depends on

Task 167, including its verified local commit.

## Goal

Remove remaining right-pane card padding while keeping every workflow decision and safety signal clear.

## Read first

[Plan.md](../Plan.md), [task workflow](README.md), [execution prompt](PROMPT_EXECUTE_UI_PRECISION.md),
and [UI design guidelines](../desktop/UI_DESIGN_GUIDELINES.md). Follow the shared
verification, safety, and cleanup rules; this file does not independently authorize execution.

## Primary files

Kotlin filenames below are under `desktop/src/main/kotlin/io/miniorca/desktop/`;
test filenames are under the matching `desktop/src/test/kotlin/io/miniorca/desktop/`.
Inspect these boundaries before editing; change only files needed for this task.

`ContextToolWindow.kt`, `AssistantToolWindow.kt`, `WorkflowToolWindows.kt`, `ReviewEvidencePane.kt`, `EditorContextualActions.kt`, related shared controls; Context/Assistant/Review/workflow tests and visual fixtures.

## Implementation

1. Replace whole-section rounded containers and generous 18–20dp padding with shared flat headers, 8dp content insets, compact rows, and 1dp section dividers.
2. Use separate disclosure controls for project context, focused analysis, insight, and detailed evidence. Keep per-section expansion state without collapsing a section when its action is used.
3. Move safe navigation/retry/copy/close operations into owning header toolbars. Avoid duplicate quick-action lists where the same live operation is already present.
4. Preserve labeled editable draft fields as contained inputs, not a whole-pane card. Preserve draft text, provider destination/consent, validation errors, and text selection when sections collapse or drawers reopen.
5. Keep Prepare/Review/Apply/Undo wording and guard states explicit. Apply/Undo cannot become unlabeled icons, be enabled by layout changes, or bypass review/evidence.
6. Use the same migrated Jewel controls for inputs, buttons, and section headers; remove replaced view helpers without changing presenter/domain ownership.

## Acceptance criteria

- [ ] The right pane reads as a continuous tool window with flat sections and compact typography, not nested cards.
- [ ] Every destructive or remote operation remains named and guarded; stale/invalid/missing evidence cannot enable Apply.
- [ ] Collapse, tab changes, narrow drawers, and reopen preserve relevant input/draft state and restore focus correctly.
- [ ] Local-only Preview controls remain incapable of backend calls or workflow-state mutation.

## Verification

Exercise Context/Assistant/Review, draft identity, provider consent, stale checks, Apply/receipt/Undo, and Preview-isolation tests. Render no-symbol, invalid-draft, ready-review, and receipt states. Run `./desktop/gradlew -p desktop spotlessCheck detekt test` and `git diff --check`.

## Commit and completion

After every criterion passes, record results below, mark this task Complete, and
update [INDEX.md](INDEX.md). Keep the task at this path. Stage only reviewed task-owned
changes and create exactly one local commit with this subject:

```text
refactor(desktop): unify flat context and review tool windows
```

Report the actual commit hash and continue to the next ready task under the execution
prompt. Do not create a partial/completion commit while acceptance is blocked; never push.

## Execution record

Not started. Record actual commands/results, evidence paths, exceptions approved by
the user, and any runtime/configuration impact during execution. Do not prefill passing results.

