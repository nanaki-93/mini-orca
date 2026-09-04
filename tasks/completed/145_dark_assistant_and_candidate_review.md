# 145 — Restyle the Assistant and candidate review

## Status

Complete

## Depends on

Task 144.

## Goal

Add the reference's compact inline suggestion presentation and scoped composer while
keeping the existing current-evidence review and Apply/Undo flow authoritative.

## Implementation

- Render a bounded candidate summary below the source from the real current draft:
  target, draft stage, concise changed-line preview when available, and Review candidate.
  Reuse existing diff models/renderers; do not create another candidate store.
- Make Review candidate open the real read-only diff and Review evidence. When the
  draft is edited/invalid/unvalidated, show that stage and the existing next action;
  never display a stale validated diff as current.
- Restyle the bound Assistant request, isolated declaration/import editor, provider
  destination and confirmation, progress, errors, and cancellation.
- Keep one declaration-scoped composer. Quick actions may focus/prefill it, but do
  not create a second conversation or send on focus/navigation.
- Keep Apply in the existing guarded Review with exact target wording. Reuse current
  validation/check eligibility, Discard behavior, receipt, and Undo callbacks.
- Handle summary close/reopen as layout state; Discard is a separate explicit action.
  Reserve feedback thumbs for Task 147's local preview behavior.

## Likely files

`AssistantToolWindow.kt`, `WorkflowToolWindows.kt`, `EditorWorkspace.kt`,
`DesktopApp.kt`, `DesktopShell.kt`, `DiffViewer.kt`, and review/presenter tests.

## Acceptance criteria

- The candidate summary resembles the dark reference and remains subordinate to source.
- It reflects the current draft identity and stage; changing target/draft cannot
  leave stale content, evidence, or an enabled Apply action visible.
- Both source and composed diffs stay selectable/read-only; only the isolated draft
  is editable. Collapse/hide never discards or applies a draft.
- Validation, required checks, source identity, explicit Apply, receipt, and Undo
  remain unchanged; the summary introduces no second eligibility rule.
- Short/narrow windows keep the draft controls and full review evidence reachable.

## Verification and completion

Run `./desktop/gradlew -p desktop spotlessCheck detekt test` and `git diff --check`.
Use `DraftReviewWorkflowTest`, `ReviewToolWindowTest`, `ReviewEvidencePaneTest`,
`FileChatStateTest`, `DesktopWorkflowPresenterTest`, and `DiffViewerTest`. Cover
edited/stale candidate summaries, open/close, discard, and safe Review navigation.
Complete/move the task and update the index; no unrequested commit.
