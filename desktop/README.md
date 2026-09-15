# Mini-Orca Desktop

The client uses `http://localhost:9090` by default; `MINI_ORCA_URL` overrides it.
Start the Go daemon using the root [README](../README.md). All provider keys stay
in the ignored daemon configuration, not desktop preferences.

## Runtime and build

The checked-in build uses Gradle 9.1.0, Kotlin/Compose compiler 2.3.20, Compose
Multiplatform 1.11.0 and Jewel standalone `0.40.0-262.10315.125`. Use JBR
25 SDK for app launch and packaging; the current native checks use JBR
`25.0.4.1+1-b583.48` on macOS arm64. The root
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
unsupported combinations and final package acceptance are recorded in
[acceptance](../docs/RELEASE_ACCEPTANCE.md).

Pinned dependency provenance from the completed migration:
[Jewel POM](https://repo1.maven.org/maven2/org/jetbrains/jewel/jewel-int-ui-standalone/0.40.0-262.10315.125/jewel-int-ui-standalone-0.40.0-262.10315.125.pom),
[JBR release](https://github.com/JetBrains/JetBrainsRuntime/releases/tag/jbr-release-25.0.4b508.27).
The migration recorded Skiko 0.144.6, JNA 5.17.0 and transitive Kotlin stdlib 2.4.0.
Recheck the resolved graph before an upgrade; no upgrade is needed for the new plan.

## Working in the app

The last successfully opened project is restored from local metadata without
contacting a model. If restore fails, the landing screen offers Open project/retry.
Summary describes the project. Analysis owns one whole-project run and progress;
Bugs, Performance and Security own separate results. The sidebar uses icons with
hover labels. Analysis status appears at the
top right, immediately before daemon connectivity. Editor owns one declaration change.
Go, Java and Kotlin projects show a type icon beside the project name in the top bar.

Summary groups facts, findings and coverage above the project interpretation.
Zero-count coverage tiles are hidden; unavailable counts retain a dash. Tool-reported
issues are counted separately from AI suggestions. A top-right light shows project
analysis status: green for current, red for failed, yellow for other states, with
status and failure details on hover or keyboard focus.
Architecture and Flows have a **Show diagram** button, disabled when a diagram is
unavailable. Diagrams start collapsed and render Mermaid flowcharts and sequence diagrams
locally, with zoom controls and selectable Mermaid source when expanded.
No browser, network request, provider call
or project-code execution is needed to display a diagram. Packages / modules use flat
rows: the parenthesized module name is the title, followed by its project-relative path
and responsibility. Entry points and next steps are omitted from Summary.

New Analysis results request Mermaid diagrams. Older prose reports remain readable and
are marked stale after the prompt update; run Analysis explicitly to replace them.
Previously saved arrow chains also render as Mermaid. Invalid or unsupported diagrams
keep their source visible with an error instead of an invented diagram.

The diagram renderer bundles beautiful-mermaid and its licenses, using the embedded
GraalJS community runtime and JSVG for SVG text and arrow rendering. Normal Gradle builds use the checked-in bundle and do not
require Node.js. To rebuild it after changing `desktop/mermaid/renderer.js` or its pinned
dependencies, run `npm ci --prefix desktop/mermaid --ignore-scripts` and
`npm run build --prefix desktop/mermaid`. Commit the lockfile and generated resource
bundle together; do not edit `src/main/resources/mermaid/renderer.js` manually.

Editor has docked Files and Context/Assistant/Review panes at widths ≥1000dp.
Below that width they become labeled drawers; Terminal uses a bounded overlay.
Resizing clamps visible widths without overwriting saved preferences. Source and
composed diffs are selectable/read-only; only the isolated draft is editable.

Context shows a short description for the selected declaration, with explicit
explanation and **Refactor** actions. Without a selected declaration, **Actions**
and **Details** provide file actions, metadata and expandable project context.
Labeled warning/error states use amber/red. Selecting a declaration or opening a
tab never sends a model request.

A current draft must be discarded explicitly before changing its target. Editing
it invalidates validation/check evidence. Review owns the guarded Apply, receipt
and Undo. Findings and insights show freshness; daemon connectivity never proves
that a provider is connected. Non-loopback scopes require their own confirmation.

## Analysis and results

Use **Start analysis** to refresh model analysis for every included project file,
or **Analyze stale & failed** to include only files with stale or failed analysis.
Both actions preview their scope before starting. Start bypasses saved model
results; selective retries can reuse fresh stages. Unchanged deterministic
Security rules can reuse their saved results. Resume continues the admitted run
with its original refresh choice and retained progress. Ignored and unanalysed
files have no status dot in the file tree. Opening a file does not change the
project analysis scope.
The preview shows exclusions, stage eligibility, cache use, expected requests and
inclusive retry bounds. Confirm each displayed
remote destination and explicit Security review intent. One admission coordinates
the existing specialized producers; it can make multiple model requests.

The default 100-file/900-second window limits dispatch, not captured inventory.
Pause/Resume/Cancel retain truthful partial coverage and cumulative attempts.
Resume requires a fresh preview; startup never silently resumes a model request.
Analysis shows progress, stage failures and links to results. Bugs, Performance and
Security use a shared results view with no search or filters: each page shows all
loaded findings for its category across the project, including when opened from a
file link. Rows retain their summary, severity/provenance, source link and details.
An unavailable or failed report is never presented as zero findings. Verified Go
scans live in Bugs; source hypotheses, advisory Security and measured benchmarks
keep distinct labels and execution requirements.

## New Go functions

Open a Go file, including one containing only a package declaration, and select
**New function** in its header or Context. Assistant focuses the new-name field.
**New Go function** and **New Go type** also remain in Commands. An invalid,
reserved or existing name is rejected before a provider request. Preparing the
composer does not generate or write code. A different active draft requires an
explicit discard choice; canceling keeps that draft intact.

After generation, edit the candidate, validate it, run trusted focused checks and
review the read-only diff. Apply and Undo retain the existing file/hash guards.
Review keeps check diagnostics; Assistant keeps its current request failure;
Analysis and Bugs retain their own operational evidence. The bottom bar shows only
counts of distinct configured local and cloud models, aligned to the right;
select the counts to view model and destination details. It has no hover tooltips.
The compact left rail shows icons with destination names on hover.

## Terminal

Selecting **Terminal** or pressing **Ctrl+Shift+T** opens a shell rooted in the
project. Shell tabs share the Terminal bar: **+** starts another shell and **×**
closes its tab and process. Selecting tabs, collapsing the dock, hiding the overlay
or changing workspaces preserves each shell's process and in-memory scrollback.
Project switching explicitly closes all active shells; application exit cleans
up every owned session.

Terminal owns ordinary shell keystrokes, including Ctrl+C. Use **Ctrl+Shift+F12**
to restore app focus. Returning to source/Review rechecks file content and marks
old evidence stale. Use **Re-index project** in the project menu after inventory
changes. Saved dimensions restore with the terminal collapsed, without starting
a shell.
See [terminal support and reproduction](TERMINAL.md).

## Keys

| Shortcut | Context/action |
| --- | --- |
| Cmd/Ctrl+O | Open project |
| Cmd/Ctrl+1–4 | Summary, Analysis, Bugs, Editor |
| Cmd/Ctrl+Tab | Cycle all workspaces, including Performance and Security |
| Cmd/Ctrl+P | Indexed file search |
| Cmd/Ctrl+Shift+O | Symbols in the current file |
| Cmd/Ctrl+K | Contextual command palette or eligible Assistant composer |
| Cmd/Ctrl+Shift+F | Bugs |
| Cmd/Ctrl+Shift+D | Current draft |
| Cmd/Ctrl+Enter | Generate/cancel when available |
| Cmd/Ctrl+Shift+V / Shift+C | Validate / focused checks when eligible |
| Ctrl+Shift+T | Open/focus Terminal from the application |
| Ctrl+Shift+F12 | Return from terminal input to Editor |
| Escape | Dismiss the top transient surface or cancel the active operation |

Use arrows and Enter/Space for tree/tab/disclosure navigation. Native keyboard and
reader observations and limits are in [acceptance](../docs/RELEASE_ACCEPTANCE.md);
the [keyboard checklist](KEYBOARD_SMOKE_CHECKLIST.md) owns the operator procedure. [Visual reproduction](../docs/RELEASE_ACCEPTANCE.md#reproduce-ui-component-checks) describes
fixture captures; these are not native-window evidence.

Security entry and selection stay local. Whole-project analysis owns
passive rules and explicitly admitted advisory review. Prepare fix opens the
existing Assistant composer only for current, exact Go declarations; it does not
send a request or change source. No file-scoped start controls or duplicate bottom
Problems/Checks/Output panels remain.

UI work follows [UI_DESIGN_GUIDELINES.md](UI_DESIGN_GUIDELINES.md) and the
[current product decisions](../PLAN.md#current-ui-direction--2026-09-15), including
the [self-explanatory UI and copy rules](UI_DESIGN_GUIDELINES.md#self-explanatory-ui-and-copy)
for new and changed features. Existing metadata and visual-preference migration
is documented in the
[root guide](../README.md#existing-projects-and-preferences); model-scope names are unchanged.

### Choose files for project analysis

In **Analysis**, use **Files** to search the project file list
and select or ignore files. **Select all** and **Exclude all** apply to all eligible
files, regardless of the search filter. Changes save automatically per project
and survive closing the project or app. Newly indexed files start selected;
ignored paths remain saved even if temporarily absent.

Excluded or unsupported files show their reason. Build, dependency and metadata
folders are omitted from the list. Use **Refresh files** to reload the checklist.
Finish or cancel an active/paused run before changing its files; start a new
analysis to use the saved selection. Existing results remain available.

The file list shows whether saved analysis is up to date and explains each
outstanding stage. Use **Needs attention** to see missing, outdated, failed,
or incomplete files. Unchecked files and files excluded by configuration appear
under **Excluded** and do not count as up to date or needing attention. **Details**
shows every stage and its explanation. Re-selecting a file restores its saved
analysis status. Bugs, Performance and Security panels use tinted surfaces like
Summary, with colors indicating analysis completion, partial coverage or failure,
independently of finding counts. **Run details** contains captured coverage and
operational failures; changing the selection does not rewrite a previous run.

Summary's overall status and coverage follow the current selected files, including
selection changes. The saved project description keeps its own freshness in the
status details; an older description does not mark current file analysis outdated.
