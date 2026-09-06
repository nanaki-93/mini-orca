# Mini-Orca Desktop

The client uses `http://localhost:9090` by default; `MINI_ORCA_URL` overrides it.
Start the Go daemon using the root [README](../README.md). All provider keys stay
in the ignored daemon configuration, not desktop preferences.

## Runtime and build

The checked-in build uses Gradle 9.1.0, Kotlin/Compose compiler 2.3.20, Compose
Multiplatform 1.11.0 and Jewel standalone `0.40.0-262.10315.125`. Use JBR
`25.0.4+1-b508.27` SDK for app launch and packaging. Supply your local path; do not
commit it or replace a system JDK as part of a task.

```sh
JAVA_HOME=/path/to/jbr-sdk-home ./desktop/gradlew -p desktop run
JAVA_HOME=/path/to/jbr-sdk-home ./desktop/gradlew -p desktop test createDistributable
```

Detekt 1.23.8 needs a JDK 21 Gradle launcher; the app still uses JBR 25. JVM 22
bytecode is the current analysis compatibility boundary, not a second runtime:

```sh
JAVA_HOME=/path/to/jdk-21 ./desktop/gradlew -p desktop spotlessCheck detekt test \
  -Porg.gradle.java.installations.paths=/path/to/jbr-sdk-home
```

Packaged images include `java.net.http` for the daemon client and `jdk.unsupported`
for Jewel's native bridge. Package with the JBR launcher, not the Detekt launcher.
The earlier macOS arm64 startup smoke passed; native UI/accessibility and final
package acceptance remain separate checks in [acceptance](../docs/RELEASE_ACCEPTANCE.md).

Pinned dependency provenance from the completed migration:
[Jewel POM](https://repo1.maven.org/maven2/org/jetbrains/jewel/jewel-int-ui-standalone/0.40.0-262.10315.125/jewel-int-ui-standalone-0.40.0-262.10315.125.pom),
[JBR release](https://github.com/JetBrains/JetBrainsRuntime/releases/tag/jbr-release-25.0.4b508.27).
The migration recorded Skiko 0.144.6, JNA 5.17.0 and transitive Kotlin stdlib 2.4.0.
Recheck the resolved graph before an upgrade; no upgrade is needed for the new plan.

## Working in the app

The last successfully opened project is restored from local metadata without
contacting a model. If restore fails, the landing screen offers Open project/retry.
Summary describes the project, Analysis owns explicit runs, Bugs owns triage,
Performance shows source hypotheses, and Editor owns one declaration change.

Editor has docked Files and Context/Assistant/Review panes at widths ≥1000dp.
Below that width they become labeled drawers; bottom tools use a bounded overlay.
Resizing clamps visible widths without overwriting saved preferences. Source and
composed diffs are selectable/read-only; only the isolated draft is editable.

A current draft must be discarded explicitly before changing its target. Editing
it invalidates validation/check evidence. Review owns the guarded Apply, receipt
and Undo. Findings and insights show freshness; daemon connectivity never proves
that a provider is connected. Non-loopback scopes require their own confirmation.

Some current utility controls are labeled Preview and do no backend work, including
Terminal, Run/Debug and additional editor tabs. UI-01 will remove these distractions;
they have not been removed by the documentation cleanup.

## Keys

| Shortcut | Context/action |
| --- | --- |
| Cmd/Ctrl+O | Open project |
| Cmd/Ctrl+1–4 | Summary, Analysis, Bugs, Editor |
| Cmd/Ctrl+Tab | Cycle workspaces, including Performance |
| Cmd/Ctrl+P | Indexed file search |
| Cmd/Ctrl+Shift+O | Symbols in the current file |
| Cmd/Ctrl+K | Contextual command palette or eligible Assistant composer |
| Cmd/Ctrl+Shift+F | Bugs |
| Cmd/Ctrl+Shift+D | Current draft |
| Cmd/Ctrl+Enter | Generate/cancel when available |
| Cmd/Ctrl+Shift+V / Shift+C | Validate / focused checks when eligible |
| Escape | Dismiss the top transient surface or cancel the active operation |

Use arrows and Enter/Space for tree/tab/disclosure navigation. The supported native
keyboard and reader checks remain in the retained
[keyboard checklist](KEYBOARD_SMOKE_CHECKLIST.md); its dated baseline sections are
history, not current dimensions. [Visual reproduction](VISUAL_REVIEW.md) describes
fixture captures; these are not native-window evidence.

UI work follows [UI_DESIGN_GUIDELINES.md](UI_DESIGN_GUIDELINES.md) and the
[single plan](../PLAN.md). No API/config migration is caused by this doc cleanup.
