# Mini-Orca Desktop

The client uses `http://localhost:9090` by default; `MINI_ORCA_URL` overrides it.
Start the Go daemon using the root [README](../README.md). All provider keys stay
in the ignored daemon configuration, not desktop preferences.

## Runtime and build

The checked-in build uses Gradle 9.1.0, Kotlin/Compose compiler 2.3.20, Compose
Multiplatform 1.11.0 and Jewel standalone `0.40.0-262.10315.125`. Use JBR
`25.0.4+1-b508.27` SDK for app launch and packaging. The root
[development guide](../README.md#development-and-documentation) owns clean-checkout
toolchain setup and full validation, including explicit JDK/JBR locations. Do not
commit a local runtime path or replace a system JDK as part of a task.

From the repository root:

```sh
MINI_ORCA_JDK21_HOME=/path/to/jdk-21 \
MINI_ORCA_JBR25_HOME=/path/to/jbr-25 \
  ./scripts/desktop-gradle.sh run

MINI_ORCA_JDK21_HOME=/path/to/jdk-21 \
MINI_ORCA_JBR25_HOME=/path/to/jbr-25 \
  ./scripts/desktop-gradle.sh test createDistributable
```

The build runs Gradle on Java 21 and explicitly launches Desktop `JavaExec` tasks,
including `run`, with the selected Java 25 toolchain. Calling `gradlew run` with a
Java 21 `JAVA_HOME` and no discoverable Java 25 toolchain cannot start the app.

Packaged images include `java.net.http` for the daemon client and `jdk.unsupported`
for Jewel's native bridge. Package with the JBR launcher, not the Detekt launcher.
The macOS arm64 startup and attainable UI-04 native/accessibility checks passed;
unsupported combinations and final package acceptance remain in
[acceptance](../docs/RELEASE_ACCEPTANCE.md).

Pinned dependency provenance from the completed migration:
[Jewel POM](https://repo1.maven.org/maven2/org/jetbrains/jewel/jewel-int-ui-standalone/0.40.0-262.10315.125/jewel-int-ui-standalone-0.40.0-262.10315.125.pom),
[JBR release](https://github.com/JetBrains/JetBrainsRuntime/releases/tag/jbr-release-25.0.4b508.27).
The migration recorded Skiko 0.144.6, JNA 5.17.0 and transitive Kotlin stdlib 2.4.0.
Recheck the resolved graph before an upgrade; no upgrade is needed for the new plan.

## Working in the app

The last successfully opened project is restored from local metadata without
contacting a model. If restore fails, the landing screen offers Open project/retry.
Summary describes the project, Analysis owns explicit runs, Bugs owns triage,
Performance shows source hypotheses, Security separates deterministic rule matches
from advisory AI suggestions, and Editor owns one declaration change.

Editor has docked Files and Context/Assistant/Review panes at widths ≥1000dp.
Below that width they become labeled drawers; bottom tools use a bounded overlay.
Resizing clamps visible widths without overwriting saved preferences. Source and
composed diffs are selectable/read-only; only the isolated draft is editable.

A current draft must be discarded explicitly before changing its target. Editing
it invalidates validation/check evidence. Review owns the guarded Apply, receipt
and Undo. Findings and insights show freshness; daemon connectivity never proves
that a provider is connected. Non-loopback scopes require their own confirmation.

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

Use arrows and Enter/Space for tree/tab/disclosure navigation. Native keyboard and
reader results plus their explicit limitations remain in the retained
[keyboard checklist](KEYBOARD_SMOKE_CHECKLIST.md); its dated baseline sections are
history, not current dimensions. [Visual reproduction](VISUAL_REVIEW.md) describes
fixture captures; these are not native-window evidence.

Security entry, filtering and selection stay local. Scan checks the selected Go
file with source-only rules. Review covers one eligible text file through Analyze
and asks for fresh consent when that provider is remote. Prepare fix opens the
existing Assistant composer only for a current, exact Go declaration; it does not
send the request or change source.

UI work follows [UI_DESIGN_GUIDELINES.md](UI_DESIGN_GUIDELINES.md) and the
[single plan](../PLAN.md). No API/config migration is caused by this doc cleanup.
