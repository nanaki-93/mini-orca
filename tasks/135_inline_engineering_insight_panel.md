# 135 — Add the collapsible in-page engineering insight panel

## Status

Pending

## Goal

Show a small, concise, closeable and reopenable insight within existing result
pages/tool windows without a separate learning system or workflow interruption.

## Depends on

Task 134.

## Required task commentary

- Before editing, post `Starting Task 135` and describe panel placement, disclosure
  preference, owner freshness, keyboard/narrow behavior, and verification.
- After the commit, post `Task 135 complete` with results, exact commit hash, and
  Task 136 as the next task.

## Implementation

- Implement one reusable `EngineeringInsightPanel` using the existing Focus Flow
  theme, compact controls, normal wrapping, selectable text, and parent scrolling.
- Use the plan's placements: project Summary/interpretation, file Analysis/Context,
  selected bug/Problems details, current Assistant proposal, and candidate Review.
  Performance reuses this component in Task 138.
- Display only an insight belonging to the visible result. Clearly label file-level
  scope if shown alongside a selected symbol; do not misattribute it to a bug.
- Start collapsed with an obvious `Engineering insight` opener. Header activation
  and `Close insight` use the same disclosure state; closing retains the opener.
- Persist one expanded/collapsed presentation preference, not per-result records,
  content, confirmation, or authorization. New results never override a closed
  preference, switch tabs, move focus, or switch Source to Review.
- Opening and closing render already-returned data only. When insight is absent,
  omit the panel rather than creating an empty placeholder or Generate action.
- Label AI interpretation and stale owners. Hide prior-proposal insight from the
  current manually edited candidate, or retain it only as explicitly historical
  proposal content. Do not use stale line numbers as current evidence anchors.
- Keep Review's insight secondary and outside its Apply guard/action grouping. Do
  not duplicate evidence policy or let panel state influence eligibility.
- Make the opener keyboard reachable with Enter/Space and expanded semantics;
  Close restores focus to it. Preserve labels without relying on color or hover.
- Below `1000dp`, keep the panel inside its current page/right drawer. No additional
  drawer, modal, full-height pane, workspace, or Learn tab.
- Do not add quizzes, feedback ratings, challenges, tracking, badges, learning
  settings, onboarding, automatic model calls, or source annotations.
- Update Desktop usage and relevant keyboard smoke steps for this shipped panel.

## Acceptance criteria

- Every applicable existing result surface uses the same compact panel; a closed
  panel can always be reopened without another provider request.
- Missing/stale/historical insights and file/project changes are truthful and cannot
  leak the previously selected result's content.
- Existing layout, provider controls, source/diff selection, draft editing, and
  Apply/Undo flow remain unchanged.
- Long text, narrow width, and scaling do not clip the opener or required actions.
- There is no gamified, quiz, or separate learning UI.

## Verification

Add presentation/state and practical Compose interaction coverage for disclosure,
preference persistence, identity changes, no-side-effect callbacks, focus, text
semantics, and the `1000dp` boundary. Run:

```text
./desktop/gradlew -p desktop spotlessCheck detekt test
make check
git diff --check
```

Inspect wide, exactly `1000dp`, narrow, and scaled-text fixtures if interactive UI
is available. Record any unavailable manual check for Task 139; do not invent a pass.

## Commit

After checks pass, record evidence, mark this task Complete, move it under
`tasks/completed/`, update the index, inspect only Task 135 staged work, and create
exactly one commit:

```text
feat(desktop): add collapsible engineering insights
```

Do not amend, squash, tag, or push. Verify no Task 135 work remains uncommitted.
