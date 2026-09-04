# 149 — Complete dark UI visual and regression acceptance

## Status

Pending

## Depends on

Tasks 140–148, completed in order.

## Goal

Deliver a visually coherent dark Mini-Orca that follows both supplied references,
retains real workflows, and exposes missing functionality only as UI previews.

## Implementation

- Compare native after-captures with Task 140's baseline and both supplied images.
  Check palette, icon rail, toolbar, tree, editor chrome, AI Context sections,
  candidate summary, issue table, status, and every retained workspace.
- Record captures under `design/ui-mocks/dark-ui/after/` and conclusions in
  `docs/dark-ui/ACCEPTANCE.md`. Describe deliberate adaptations such as one real file,
  Review-before-Apply, retained Performance, native window controls, and Preview badges.
- Verify every Live/Preview matrix row, including zero backend actions from previews
  and honest missing/stale/failed state. Close any omitted visual surface or behavior.
- Run the reference viewport/keyboard matrix and available full workflow fixture:
  open, choose symbol, prepare, validate, check, review, Apply, receipt, Undo.
- Review the final diff for dead styling, duplicate policies, obsolete helpers,
  misleading sample content, stale documentation, and accidental backend changes.
- Update `desktop/README.md` with the delivered theme, real UI behavior, and preview
  limitations. Update the active plan/task index with actual status and evidence.

## Acceptance criteria

- Visual similarity is supported by native evidence, not inferred solely from tokens
  or an HTML design concept. Outstanding native checks remain explicitly incomplete.
- All existing workspaces, text labels, keyboard paths, source/diff read-only behavior,
  provider confirmation, and guarded Apply/Undo behavior are retained.
- Unsupported controls are visibly Preview, local-only, and cannot produce backend effects.
- Implementation is confined to desktop presentation and needed tests/documentation;
  no configuration/API migration or new source-mutation capability is introduced.
- Required checks pass. A blocked required check or missing material visual evidence
  is reported accurately; do not mark full visual acceptance complete without it.

## Verification and completion

Run:

```text
./desktop/gradlew -p desktop spotlessCheck detekt test
make check
make quality
git diff --check
```

Do not run destructive targets. Report task statuses, changed behavior, verification
results, any unavailable manual checks, and migration/configuration needs (expected:
none; existing pane preferences retained). Complete/move this task and update the
index only when the required acceptance is satisfied. No unrequested commit or push.
