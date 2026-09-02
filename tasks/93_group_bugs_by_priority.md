# 93 — Group Bugs by priority

## Status

Pending

## Goal

Make Bugs open in a deterministic high-to-low priority presentation while retaining
clear provenance for every finding.

## Depends on

Task 92.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 93` and name priority
  grouping, filtering order, provenance preservation, fallback handling, and focused
  checks.
- After the task commit succeeds, post a separate update beginning with
  `Task 93 complete` and state the behavior, checks, commit hash, and that Task 94 is
  next.

## Implementation

- Add a small pure priority model/helper in `BugsWorkspaceState.kt` for normalized
  `high`, `medium`, `low`, and fallback severities.
- Produce nonempty groups in this order: `HIGH PRIORITY`, `MEDIUM PRIORITY`,
  `LOW PRIORITY`, then `OTHER PRIORITY` for blank or unknown legacy client data.
- Apply `filterFindings` before grouping and preserve the backend order within each
  priority group. Do not add server sorting, a persisted preference, or a grouping
  selector.
- Update `BugsWorkspacePane` to render the priority groups instead of top-level
  verified/suggested/unclassified groups.
- Keep source, confidence, lifecycle, freshness, evidence, location, and severity on
  finding cards. In particular, preserve the visible distinction between verified/tool
  findings and AI suggestions through the existing provenance label.
- Render no empty priority section and keep the current empty filtered-result behavior
  concise.
- Preserve search, advanced filters, scan controls, triage, Open in Editor, and Prepare
  fix guards.

## Acceptance criteria

- The default Bugs list is grouped high, medium, then low, independent of incoming
  mixed order.
- Blank/unknown severity findings remain visible in one final fallback group.
- Filtering happens before grouping, and empty priority headings do not appear.
- Original order is stable within each group.
- Verified and AI-suggested findings may share a priority section but remain explicitly
  distinguishable on every card.
- Existing scan, navigation, fix preparation, filtering, and triage behavior is
  unchanged.

## Verification

- Extend `BugsWorkspaceStateTest` for case/whitespace normalization, deterministic group
  order, stable within-group order, fallback retention, and filtering-before-grouping.
- Update affected Bugs presentation/integration tests for priority headings and retained
  provenance text.
- Run:

  ```text
  ./desktop/gradlew -p desktop test
  git diff --check
  ```

## Commit

After all criteria pass, set this task to Complete, move it to `tasks/completed/`, update
its `tasks/INDEX.md` row, stage only Task 93 changes, inspect the staged diff, and create
exactly this commit:

```text
feat(desktop): group bugs by priority
```

Do not amend, combine, or push the commit.
