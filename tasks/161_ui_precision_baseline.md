# 161 — Establish the baseline and prove Jewel compatibility

## Status

Pending

## Depends on

The current implementation and this approved plan. Task 149 is not a dependency.

## Goal

Make the design, dependency migration, and existing regressions measurable before production styling changes.

## Read first

[Plan.md](../Plan.md), [task workflow](README.md), [execution prompt](PROMPT_EXECUTE_UI_PRECISION.md),
and [UI design guidelines](../desktop/UI_DESIGN_GUIDELINES.md). Follow the shared
verification, safety, and cleanup rules; this file does not independently authorize execution.

## Primary files

Kotlin filenames below are under `desktop/src/main/kotlin/io/miniorca/desktop/`;
test filenames are under the matching `desktop/src/test/kotlin/io/miniorca/desktop/`.
Inspect these boundaries before editing; change only files needed for this task.

`desktop/build.gradle.kts`, `desktop/settings.gradle.kts`, wrapper properties (inspect); `DesktopTheme.kt`, `ChromeControls.kt`, shell/workspace components (inspect); existing desktop tests; `desktop/UI_COMPONENT_DECISION.md`; new `desktop/UI_PRECISION_BASELINE.md`.

## Implementation

1. Read the active plan, current design guidelines, retained acceptance records, and supplied dark mock. Inventory every pane background, rounded section, action bar, Material import, and custom shared control. Assign each replacement to Tasks 162–169.
2. Capture representative current production-component scenes for Editor, Analysis, Summary, Performance, Context/Review, and Problems. Record viewport, data fixture, text/display scale, and what cannot be captured natively. Preserve the existing rendering adapter's detached-window limitation.
3. Run the current checks before changing dependencies. Record exact toolchain/runtime versions and all baseline failures, including later quality stages skipped after an earlier failure.
4. Inspect official Jewel release documentation and published POM/metadata. Select a stable published standalone coordinate and verify Kotlin/compiler, Compose/Skiko, wrapper, and JBR compatibility. Record artifact availability, repository provenance, runtime distribution/platform coverage, and license implications. Do not use floating, unpublished, or snapshot versions.
5. Build a temporary standalone compatibility spike with the actual theme, action, tab, tree/disclosure, menu, and text-input APIs. Exercise focus/disabled behavior and a representative offscreen test under the selected runtime. Keep source/reproduction instructions in the baseline only as needed; remove the scratch app after recording results.
6. Replace the historical-only component decision with a dated adoption decision and exact migration matrix, preserving the old outcome as a short historical note. Confirm no custom window decoration or experimental popup flag is needed.
7. Review and own the initial planning-document cleanup under the execution prompt's bootstrap allowlist; preserve any later unrelated edits.

## Acceptance criteria

- [ ] The baseline identifies affected production components, density/surface measurements, matching fixtures, and current safety tests; it does not describe source inspection as native evidence.
- [ ] A real published Jewel/toolchain/runtime combination resolves and compiles the representative slice. JBR launch/package requirements and a reproducible command are documented.
- [ ] The selected path is Jewel adoption. A demonstrated blocker triggers a concise user decision; a Compose upgrade requirement alone does not justify deferral.
- [ ] Baseline failures are recorded verbatim and separated from newly introduced failures. No production UI migration, daemon change, persistent scratch app, or unrequested system-JDK install is included.

## Verification

Run `./desktop/gradlew -p desktop spotlessCheck detekt test`, `make check`, `make quality`, the spike's wrapper build/tests, and `git diff --check`. Record failed and skipped stages accurately. Known unchanged Go quality debt may be recorded without fixing it here; compatibility failures block this task.

## Commit and completion

After every criterion passes, record results below, mark this task Complete, and
update [INDEX.md](INDEX.md). Keep the task at this path. Stage only reviewed task-owned
changes and create exactly one local commit with this subject:

```text
test(desktop): establish UI precision and Jewel baseline
```

Report the actual commit hash and continue to the next ready task under the execution
prompt. Do not create a partial/completion commit while acceptance is blocked; never push.

## Execution record

Not started. Record actual commands/results, evidence paths, exceptions approved by
the user, and any runtime/configuration impact during execution. Do not prefill passing results.

