# Mini-Orca

A local-first, Go-first coding assistant for analysis and deliberate AI changes:
describe a task in Chat, inspect checked file diffs, then explicitly Apply.
The Wails/React desktop client talks to a Go daemon through a native bridge.

## Use it

1. Open a local project and inspect its summary, files and symbols.
2. Select **Prepare analysis** in Analysis. Review the whole-project inventory,
   request bounds and model destinations, then confirm the displayed run.
   Analysis also checks for new feature suggestions using saved project goals.
   Summary links category counts to Bugs, Performance, Security and Features.
3. Use **Prepare fix** on a finding, or describe a feature in **Chat** with up to
   eight explicit Go/Markdown paths, including new files. Preparation generates
   a proposal, checks it and attempts at most three repairs of failed checks.
   Project tests require execution trust; preparation stops at diff review.
4. Read all proposed diffs, select **Review this diff**, then **Approve and apply**.
   Confirm the displayed scope. Further chat revisions require new checks/review.
   Guarded **Undo proposal** restores the latest unchanged grouped change.
5. Use **Features** for goal-aware advisory ideas, with Save/Dismiss and a passive
   handoff to Chat. Use **Instructions** to load root/directory AGENTS.md guides,
   add editable presets or custom text, and review the resulting instruction diff.
6. After Apply, explicitly **Verify applied change** or **Reanalyze changed files**.
   Applied and verified results stay separate; original findings are not marked
   fixed automatically. **Source** retains read-only inspection and the precise
   declaration/import draft tools, including selected benchmark comparisons.
7. Open **Terminal** and select **New terminal** for a local shell in the project.
   Hiding it preserves the process; closing its tab ends it. **⌘K / Ctrl+K** opens
   file and command search.

Bugs, Performance and Security offer search and impact filters over saved results.
Pause waits for an active stage; Cancel stops further work. Resume uses a
fresh preview and intent; completed or canceled runs need a new Start. The run coordinates specialized
semantic, Performance, Security-rule, advisory Security and project-wide feature
suggestion stages and can make
multiple model requests. Security intent is explicit even with a local provider.
Verified Go scans, focused checks and selected benchmarks remain separate trusted
execution actions. Source hypotheses are not runtime measurements.

Source changes always require approval. Mini-Orca does not automatically run
scans, commit or push. Shared proposals support Go and Markdown without deletions;
other languages retain conservative analysis and symbol information. Optimization
claims remain unmeasured unless a benchmark comparison provides measurements.

## Run

Use Go 1.25+, Node.js 22+ and Xcode command-line tools for the macOS arm64 desktop
application. The daemon itself still targets Go 1.22. Read
[desktop setup](desktop/webapp/README.md#build-and-run) before the first launch.
Configure all three model scopes using [CONFIG.md](CONFIG.md). Each scope supports
an OpenAI-compatible HTTP provider, Antigravity CLI (`agy`), or Pi (`pi`).

```sh
cp config.example.yaml config.yaml
go run ./cmd/daemon
# In a second terminal:
make desktop-run
```

The daemon defaults to `127.0.0.1:9090`; the desktop uses
`http://127.0.0.1:9090` unless `MINI_ORCA_URL` is set. Keep `config.yaml` local.
Each prompt-bearing request to a non-loopback HTTP or CLI provider needs scope-specific
confirmation. Provider keys are not part of API metadata.

## Project intelligence

The daemon owns indexing, context exclusions, model requests and guarded source
mutation. The desktop renders state and rejects stale asynchronous results.
Three configured scopes serve project/Performance/Security analysis (`analyze`), file/bug
analysis (`bug`) and change/declaration proposals/repairs (`function`).

Project-local `.mini-orca/` stores `index.json`, `project-analysis.json`,
`file-analysis/`, `findings.json`, `performance/files/`, unified progress in
`analysis/run.json`, legacy job history under `sessions/`,
Apply receipts/audit under `sessions/`, and declaration Undo data under `backups/`.
Private `changes/` files retain conversations, captured source and proposals,
feature goals/ideas, the latest grouped recovery journal and verification evidence.
AGENTS.md files stay in the project as the instruction source of truth. Restore
uses local persisted analysis without a model call.
See the [API guide](docs/api-contract.md) for current contracts and boundaries.

## Existing projects and preferences

No model-scope configuration rename is required. Fresh semantic reports use prompt
`file-analysis-v14` with explicit Bugs/Performance/Security risk categories. The
persisted semantic report schema remains `1`; uncategorized historical findings
remain visible as historical/unclassified evidence and never enter new category
counts. Changed prompt/provider/source identities require fresh analysis; reading
or restoring history makes no model request.

Unified runs use schema `1` in `.mini-orca/analysis/run.json`. Interrupted runs
retain their attempt ledger and need a fresh preview before resuming. Older
Analyze-all/Performance jobs remain readable but cannot inherit fresh dispatch
authority; start a new run when they cannot be resumed. See the
[API migration details](docs/api-contract.md#legacy-job-migration-ana-05).

The web client remembers the last project path, theme and text size. Compose pane
preferences remain separate. Terminal starts without launching a shell.
Mini-Orca does not save terminal transcripts or send them to a model. Returning to
Source/Review rechecks the selected file and invalidates stale evidence; reindex
explicitly after adding, removing or renaming files in the terminal.

## Development and documentation

Follow [AGENTS.md](AGENTS.md). UI contributions follow the
[UI interaction and accessibility guidelines](desktop/UI_DESIGN_GUIDELINES.md).
Run focused tests while editing. For a clean checkout,
install Go 1.25+ and Node.js 22+. The retained Compose client also uses the
checked-in Gradle wrapper. The full local gate is:

```sh
MINI_ORCA_JDK21_HOME=/path/to/jdk-21 \
MINI_ORCA_JBR25_HOME=/path/to/jbr-25 \
./scripts/validate.sh
```

The JDK 21 launcher is required by Detekt; the Java 25 location supplies the desktop
toolchain and may be a JBR or JDK. Both explicit homes must contain `java` and `javac`
at those exact major versions. `MINI_ORCA_JAVA25_HOME` is an equivalent explicit name
for a non-JBR toolchain. If `MINI_ORCA_JDK21_HOME` is omitted, the script accepts only
a verified Java 21 JDK from `JAVA_HOME` or `PATH`; it does not defer launcher selection
to Gradle. The Java 25 toolchain may be omitted only when Gradle can discover it. The
default checks use fakes and temporary directories, do not start the daemon, and do not
require a provider, key, or project configuration. The wrapper pins Gradle 9.1.0 and
verifies its published SHA-256 before use.

`make check` remains the supported Make gate for formatting, tests, race detection,
vet and both desktop clients' tests. `make quality` runs the pinned static/reachability/complexity/
clone and Desktop static stages and reports every failed stage.

- [Web desktop setup and usage](desktop/webapp/README.md) · [Legacy Compose client](desktop/README.md) · [UI guidelines](desktop/UI_DESIGN_GUIDELINES.md)
- [API guide](docs/api-contract.md) · [OpenAPI](docs/openapi.yaml) · [configuration](CONFIG.md)
- [Docker](DOCKER.md) · [release notes](RELEASE_NOTES.md)

Documentation describes current use and supported contracts, with one owner per fact.
MIT licensed; see [LICENSE](LICENSE).
