# 158 — Polish Editor, workflow surfaces, and global UI copy

## Status

Pending

## Depends on

Task 157, completed and committed.

## Goal

Finish app-wide consistency and reduce narration in the editing/review workflow
without hiding safety evidence or changing source-mutation authority.

## Execution contract

Follow `desktop/UI_REFINEMENT_PLAN.md`, `desktop/UI_DESIGN_GUIDELINES.md`, and
`tasks/PROMPT_EXECUTE_UI_REFINEMENT.md`. Paths in this paragraph are repository-root
relative. This task runs only after explicit execution is requested. Use one
implementation agent, finish and commit this task before starting the next, preserve
user-owned changes, and keep the preview-first workflow intact.

## Likely files

Production/test filenames without a directory are under the corresponding
`desktop/src/main/kotlin/io/miniorca/desktop/` or
`desktop/src/test/kotlin/io/miniorca/desktop/` directory.

`EditorWorkspace.kt`, `SourceEditorPane.kt`, `ContextToolWindow.kt`,
`AssistantToolWindow.kt`, `WorkflowToolWindows.kt`, `ReviewEvidencePane.kt`,
`BottomEvidenceToolWindows.kt`, `DesktopStatusBar.kt`, `DesktopShell.kt` landing/
dialogs, `CommandPalette.kt`, and workflow tests.

## Implementation

- Complete the baseline screen inventory: Editor, AI Context, Assistant, Review,
  Checks, Output, status/details, landing, command palette, confirmations, and
  remaining empty/error surfaces. Reuse the common visual system and components.
- Remove duplicated instructional paragraphs and state narration. Keep concise
  verb-led actions, contextual helper text, aligned fields, quiet separators, and
  one ordinary empty-state message.
- Preserve user chat, candidate source, composed diff, tool/check output, error
  details, and analysis content. Compact previews must expose exact full content
  through a keyboard-accessible route.
- Keep provider destination/consent, target file/symbol, draft freshness, validation
  and focused-check evidence, Apply scope, receipt, Undo eligibility, and discard
  consequences visible where the user makes the corresponding decision.
- Preserve read-only/selectable source and diff, gutter/line navigation, editor
  context, monospaced readability, syntax and +/- diff cues.
- Keep asynchronous status useful without duplicating it across every panel;
  distinguish daemon state from actual configured provider/destination evidence.
- Refine remaining live/Preview dialogs but never remove the Preview limitation
  label or wire an unsupported control to a real action.
- Do not create a custom native titlebar or broaden dependency migration here.

## Required legacy removal

Remove superseded copy, layout wrappers, local style overrides, stale comments,
and unreachable UI branches in each touched surface. Preserve safety-policy code;
never replace it with a second UI-only eligibility implementation.

## Acceptance criteria

- Every inventoried surface uses consistent chrome, fields, badges, typography,
  and concise copy; no forgotten generic-looking workflow dialog remains.
- Critical consent/warning/evidence stays visible, not solely in a tooltip.
- Exact source/draft/messages/logs remain available; the only editable code surface
  is still the isolated draft.
- Navigation and disclosure do not trigger model work or writes; Apply/Undo and
  draft invalidation/confirmation paths retain their existing guards.

## Verification

Run existing `DraftReviewWorkflowTest`, `DesktopWorkflowPresenterTest`,
`ReviewToolWindowTest`, `ReviewEvidencePaneTest`, `WorkflowToolWindowsTest`,
`ModelScopeStateTest`, `DiffViewerTest`, and relevant integration tests through the
full desktop suite. Add actual component interactions for abbreviated/full details,
consent visibility, stale drafts, failed checks, and discard/Undo confirmation.

For every task, run `./desktop/gradlew -p desktop spotlessCheck detekt test` and
`git diff --check` before committing; checks already included above need not run
twice. Add behavior-focused tests where coverage is missing. Record actual results
below, including visual evidence and native checks deferred to Task 159/160.
Required automated failures block the task subject only to the explicitly stated
repository-baseline exception in Tasks 150 and 160.

## Commit

After acceptance passes, prepare Complete status, move this file to
`tasks/completed/`, and update its index link/status in the same isolated commit.
Follow the prompt's staging/review protocol. Use exactly this subject:

```text
style(desktop): polish editor and review surfaces (task 158)
```

No partial, checkpoint, fixup, combined-task, or extra metadata commit. Report the
full hash only after the commit succeeds; do not place its own hash in this file.

## Verification evidence

Not run — task is Pending. Replace this paragraph during execution with actual
checks, outcomes, legacy code removed, evidence paths, and remaining limitations.
