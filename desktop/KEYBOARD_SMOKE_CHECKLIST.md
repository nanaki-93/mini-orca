# Desktop keyboard smoke checklist

Start Mini-Orca with no project open, then use a Go project for the remaining steps.

## Task 118 baseline viewport matrix

The following is the pre-IDE-shell baseline recorded from the current Compose
implementation. The interactive Desktop application is not available in this
execution environment (the connected computer inventory contains no running
Mini-Orca application), so no screenshots were captured and none are claimed as
passing evidence. A release operator must repeat this matrix with the maintained
fixture before the redesigned shell is released.

| Viewport | Expected baseline | Evidence |
| --- | --- | --- |
| Approximately `1440x900` | The project workspace shows the `176dp` workspace rail. In Editor, the `270dp` Explorer and `390dp` Context panes are docked beside the source/review canvas; the widths clamp to `180–520dp` and `280–560dp` respectively. | Static implementation inventory only; interactive screenshot unavailable. |
| Approximately `1100x760` | The same wide Editor arrangement remains docked. Long source/diff lines scroll horizontally and source/review remain selectable and read-only. | Static implementation inventory only; interactive screenshot unavailable. |
| Exactly `1000dp` wide | This is still the wide layout: the Explorer and Context panes remain docked in Editor. | Covered by `useNarrowLayout(1000f) == false`; interactive screenshot unavailable. |
| Below `1000dp` wide | Only Editor exposes labeled `Files` and `Context` modal drawers; leaving Editor closes an open drawer while retaining the selected file. | Covered by `editorDrawerActionsVisible`; interactive screenshot unavailable. |

The current footer is `30dp` and appears only for loading or actionable errors.
Summary, Analysis, and Bugs currently occupy the full content canvas; source/review
and the draft/Apply/Undo workflow live in Editor. The subsequent tasks intentionally
replace these layout details while retaining the safety and keyboard behavior below.

1. Confirm the landing state contains only product identity, **Open project**, and contextual progress or retry feedback. Press `Cmd/Ctrl+O`, cancel the chooser, and confirm the landing state is unchanged. Repeat with a failed open if available; confirm **Open project** remains available for retry. Before a project opens, verify `Cmd/Ctrl+1`–`4`, `Cmd/Ctrl+P`, `Cmd/Ctrl+Shift+O`, `Cmd/Ctrl+K`, and `Cmd/Ctrl+Tab` have no project action.
2. Press `Cmd/Ctrl+O` and import the fixture. Confirm the full workspace replaces the landing state with no remaining landing content.
3. Press `Cmd/Ctrl+1` through `Cmd/Ctrl+4`; Summary, Analysis, Bugs, and Editor each become active. `Cmd/Ctrl+Tab` cycles through the same four destinations.
4. From Summary, Analysis, or Bugs, press `Cmd/Ctrl+P` and choose an indexed file. Confirm Editor becomes active with that exact path and its header shows the basename plus project-relative path, then use `Cmd/Ctrl+Shift+O` to choose a symbol. Repeat with duplicate basenames when available and confirm the relative path disambiguates them. Hover and click within a highlighted declaration; confirm the pointer becomes a hand and Context shows only that declaration's explanation. Click a nested declaration to confirm the most specific declaration wins; click between declarations to confirm Context clears stale symbol context while retaining the focused line. Drag source text and confirm it selects text without changing the declaration or draft.
5. On a wide window, confirm Summary, Analysis, and Bugs have no Explorer or Context pane while Editor retains both. At exactly 1000dp, confirm Editor's wide panes remain stable. Below 1000dp, confirm labeled Files and Context drawers appear only in Editor; leave Editor with a drawer open and confirm it closes without losing the selected file.
6. In Analysis, confirm project coverage and current/last-run totals are visible. Confirm only failures with path, attempts, and sanitized error text appear under Analysis Errors, with a textual no-errors state when applicable. Check that the file/retry limits share one compact row and Pause/Cancel or Resume/Cancel wrap without clipping on a narrow window.
7. In Bugs, use Tab to reach the search field, Filters disclosure, advanced filters, and finding actions. Confirm only search is initially visible, active filters are named in text, and `Cmd/Ctrl+Shift+F` returns to Bugs. Confirm filtered findings appear in nonempty `HIGH PRIORITY`, `MEDIUM PRIORITY`, `LOW PRIORITY`, then `OTHER PRIORITY` sections; each card still names its verified/tool or AI provenance.
8. In Editor, select an exact atomic Go function, method, or type and confirm Context shows one **Edit `<symbol>`** action. Activate it and confirm the Replace composer opens and focuses its message field without sending a request or changing source. Open Commands and choose **Create declaration** to confirm the new-name field appears only on that route. If another draft is active, confirm changing target requires an explicit discard decision.
9. Tab to the chat composer, enter a request, and press `Cmd/Ctrl+Enter` to send. Tab to the declaration field. For a draft without imports, confirm `Required imports` is absent and diagnostics/status follow the declaration immediately; for a draft with imports, confirm the field is editable. Edit only the draft, then press `Cmd/Ctrl+Shift+V` to validate and `Cmd/Ctrl+Shift+C` to run focused checks when each contextual action is available.
10. Confirm no Editor progress explanation or source selection subtitle is shown. Successful validation opens the read-only diff and Review beneath the same active-file header; Review combines scope, validation, checks, read-only impact/Git context, the exact Apply action, receipt, and Undo. Confirm Apply appears only after current focused checks and names the selected symbol and file; Apply has no generic confirmation dialog. Use **Edit draft** to return to the current draft, then modify it and confirm previous validation/checks are invalidated.
11. During an open dialog or active request, press Escape and confirm only that dialog/request closes or cancels. With no active dialog/request, Escape leaves the current workspace and source unchanged.
12. After Apply and Undo, confirm the selected source refreshes, the receipt names the guarded result, and old draft/check evidence is gone.

13. At supported text scaling, verify long relative paths, symbol names, diagnostics, command output, compact buttons, and fields remain readable through scrolling or ellipsis without horizontal clipping. Confirm primary, navigation, positive, attention, destructive, and neutral actions retain text labels and readable state in addition to their color.

## Task 130 responsive and accessibility verification

The interactive Desktop application remains unavailable in this execution environment, so the
operator-only checks below are intentionally not marked as visually verified. The automated
contracts listed here passed with `./desktop/gradlew -p desktop spotlessCheck detekt test`.

| Area | Automated evidence | Interactive follow-up |
| --- | --- | --- |
| Responsive layout | `DesktopKeyboardNavigationTest` verifies that exactly `1000dp` keeps left/right/bottom regions docked, while `999dp` uses Project and right drawers plus a bounded bottom-tools overlay. | At `1440dp`, `1100dp`, `1000dp`, and `999dp`, confirm the editor remains the primary surface and no essential label clips. |
| Keyboard and focus | `DesktopKeyboardNavigationTest` verifies wrapping arrow navigation moves tab-group focus without activation, Escape selects one topmost transient surface, and leaving Editor closes only its incompatible drawer. Explorer tree arrow behavior remains covered by its existing tests. | Tab through toolbar, tool-window bar, Project tree, editor tabs, source, right tabs, bottom tabs, and status. Verify the cyan focus indication, Enter/Space activation, and return focus after each drawer or dialog closes. |
| Text scaling | The toolbar policy switches project operations to its labeled menu below the expanded width, and editor/project labels use bounded ellipsis; the long-breadcrumb policy is covered by `DesktopKeyboardNavigationTest`. | Repeat the viewport matrix at the supported platform text scale. Verify paths, findings, bottom summaries, and status labels remain readable through wrapping, scrolling, or ellipsis without horizontal clipping. |
| Semantics and non-color states | The new test verifies focused left/right/bottom tab descriptions retain text labels and selected state. Explorer freshness now always renders a textual badge, and source/review labels remain explicitly read-only. | Use the supported screen reader to confirm names for tool windows, active tabs, selected files and symbols, status details, source, and review surfaces. |

## Task 131 workspace presentation comparison

The interactive Desktop application is still unavailable in this execution environment, so no
live screenshots or focus observations are claimed. The Summary, Analysis, Bugs, and shared
Problems presentation contracts were validated with the focused desktop state tests and the
unchanged Focus Flow palette-token test.

Before release, compare the supported viewport matrix against the Task 118 baseline. Confirm that
Summary facts, Analysis progress/errors, Bugs filters/scan state, compact finding rows, and the
selected finding details region retain readable labels and scroll rather than clipping; confirm
the shared palette, focus treatment, disabled controls, and verified/advisory labels remain
recognizable.

## Task 132 final acceptance record

Automated acceptance completed in this environment. The desktop unit/presentation suite covers
landing and restore state, workspace navigation, long-path and duplicate-basename identity,
tree and tab keyboard behavior, stale analysis and responses, filtered findings, remote-provider
confirmation gates, validation/check identity, cancellation, daemon status, and Apply/Undo
receipt refresh. The repository quality gate also covers the daemon and workflow contracts.

Commit-history audit: each required Task 118–132 implementation commit appears
once and in numeric order. The only additional commit is the explicitly
user-authorized metadata-only Task 119 record (`133bf0a`); it did not replace,
combine, amend, tag, or push any required implementation commit.

The following live checks remain release-operator work because no Mini-Orca desktop window or
provider-backed fixture is available here:

- Capture the final `1440x900`, `1100x760`, `1000dp`, and `<1000dp` screenshot matrix for
  landing, Source, Review, Analysis, and Bugs, including long paths and compact bottom details.
- Perform the numbered keyboard checklist with mouse-only, keyboard-only, and mixed input;
  confirm dialog/drawer focus restoration and the cyan focus treatment.
- Check supported platform text scaling and screen-reader output for paths, selected symbols,
  provider destination, workflow states, source, and diff read-only labels.
- Exercise local and remote provider flows, disconnect/reconnect behavior, and a real
  provider-backed draft/validation/check/Apply/Undo fixture.
