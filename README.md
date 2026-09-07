# Mini-Orca

A local-first, Go-first coding assistant for one deliberate change: one project,
one file, one declaration, an editable AI draft, checks and explicit Apply.
The Kotlin/Compose Desktop client talks to a Go daemon on loopback.

## Use it

1. Open a local project and inspect its summary, files and symbols.
2. Review a bug suggestion or select a declaration to change. New declarations
   must belong to the current file and have an absent valid name.
3. Send a short request in the file-bound Assistant. Edit only the returned
   declaration/import draft; source and composed diffs stay selectable/read-only.
4. Validate, run focused checks and inspect Review. Apply names the exact file
   and declaration. Undo restores only the immediately preceding unchanged Apply.

Analysis, deterministic scans, advisory Security review and selected benchmark
runs are explicit actions. Findings retain provenance and freshness. Security
keeps local rule matches separate from AI suggestions, and remote review consumes
a fresh Security-specific Analyze confirmation.

Mini-Orca does not automatically edit source, run scans, write tests to your
project, commit or push. Exact declaration editing is currently Go-first; other
languages have conservative analysis and symbol information. Remaining delivery
and release work is tracked in [PLAN.md](PLAN.md).

## Run

Use Go 1.22+ and the desktop's pinned JBR 25 toolchain. Read
[desktop setup](desktop/README.md#runtime-and-build) before the first launch.
Configure all three model scopes using [CONFIG.md](CONFIG.md).

```sh
cp config.example.yaml config.yaml
go run ./cmd/daemon
# In a second terminal, with the documented desktop runtime:
MINI_ORCA_JDK21_HOME=/path/to/jdk-21 \
MINI_ORCA_JBR25_HOME=/path/to/jbr-25 \
  ./scripts/desktop-gradle.sh run
```

The daemon defaults to `127.0.0.1:9090`; the desktop uses
`http://localhost:9090` unless `MINI_ORCA_URL` is set. Keep `config.yaml` local.
Each prompt-bearing request to a non-loopback model needs scope-specific
confirmation. Provider keys are not part of API metadata.

## Project intelligence

The daemon owns indexing, context exclusions, model requests and guarded source
mutation. The desktop renders state and rejects stale asynchronous results.
Three configured scopes serve project/Performance analysis (`analyze`), file/bug
analysis (`bug`) and declaration proposals/repairs (`function`).

Project-local `.mini-orca/` stores `index.json`, `project-analysis.json`,
`file-analysis/`, `findings.json`, `performance/files/`, job state under `sessions/`,
Apply receipts/audit under `sessions/`, and Undo data under `backups/`. These are
application metadata. Restore uses local persisted analysis without a model call.
See the [API guide](docs/api-contract.md) for current contracts and boundaries.

## Development and documentation

Follow [AGENTS.md](AGENTS.md). Run focused tests while editing. For a clean checkout,
install Go 1.22 and use the checked-in Gradle wrapper. The full local gate is:

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
vet and desktop tests. `make quality` runs the pinned static/reachability/complexity/
clone and Desktop static stages and reports every failed stage.

- [Plan and task ledger](PLAN.md) · [agent execution workflow](tasks/README.md)
- [Desktop runtime, usage and keys](desktop/README.md) · [UI guidelines](desktop/UI_DESIGN_GUIDELINES.md)
- [API guide](docs/api-contract.md) · [OpenAPI](docs/openapi.yaml) · [configuration](CONFIG.md)
- [Docker](DOCKER.md) · [release notes](RELEASE_NOTES.md) · [acceptance evidence](docs/RELEASE_ACCEPTANCE.md)

Completed plans and task history live in Git. New documentation should describe
current use, a real contract or an unresolved decision, with one owner per fact.
MIT licensed; see [LICENSE](LICENSE).
