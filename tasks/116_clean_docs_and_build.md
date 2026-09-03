# 116 — Clean documentation and build residue

## Status

Pending

## Goal

Make maintained documentation and build configuration describe only the cleaned
system, consolidate obsolete task/design artifacts, and remove known build residue.

## Depends on

Task 115.

## Required task commentary

- Before editing, post an update beginning with `Starting Task 116` and name the
  canonical docs, historical artifacts, Docker cleanup, Gradle compatibility check,
  and verification scope.
- After the commit, post a separate update beginning with `Task 116 complete` and
  report removed/consolidated artifacts, build changes, commands/results, exact commit
  hash, and that Task 117 is next.

## Implementation

- Audit every tracked Markdown/design file and every relative link before deletion.
- Preserve current product, architecture, configuration, migration, API, contributor,
  and release information in durable maintained documents.
- Consolidate the pre-cleanup Tasks 01–102 archive into a concise historical ledger,
  then delete their detailed completed task files and obsolete execution prompts. Git
  history remains the full archive.
- Keep Tasks 103–117 and `PROMPT_EXECUTE_LEGACY_CLEANUP.md` because they are the active
  auditable execution record until final acceptance.
- Remove obsolete UI mock HTML/PNG/CSS and roadmap files only after confirming that no
  maintained document or active design workflow needs them.
- Choose one canonical human API guide plus `docs/openapi.yaml`; remove duplicate API
  prose or replace a top-level duplicate with a short pointer.
- Update README, Desktop README, CONFIG, Docker, API, migration, release notes, and
  release acceptance to match the final packages, routes, scoped configuration,
  persistence files, and manual cleanup guidance.
- Remove copied Go source from the runtime Docker image and validate that the binary
  still starts and the health check still works.
- Check official compatibility documentation for the current Compose/Kotlin/Gradle
  stack before changing versions. Align the minimum necessary wrapper/plugin versions
  to eliminate the recorded usage-attribute warning; do not perform an unrelated
  dependency modernization.
- Remove obsolete Compose version declarations and Docker Compose syntax only when
  current tooling confirms they are redundant.
- Add ignores only for proven local/editor artifacts; do not delete untracked user files.
- Validate all maintained relative links and OpenAPI references.

## Acceptance criteria

- Documentation exposes one current architecture, config contract, API contract, and
  migration path.
- No pre-cleanup task roadmap or implemented UI mock is presented as current work.
- The current cleanup task set remains usable through Task 117.
- The runtime container contains only the binary and required runtime assets and its
  health check passes.
- Desktop tests run without the identified Gradle usage-attribute warning.
- No generated output, credential, local config, or untracked user artifact is included.

## Verification

Run:

```text
make fmt-check
go test ./...
./desktop/gradlew -p desktop test
make check
git diff --check
```

Also build/inspect the supported container target without running cleanup commands,
validate maintained documentation links, and capture the official build-tool
compatibility source used for any version change.

## Commit

After all criteria pass, mark the task Complete, move it to `tasks/completed/`, update
`tasks/INDEX.md`, stage only Task 116 changes, inspect the staged diff, and create
exactly one commit:

```text
chore: remove legacy docs and build residue
```

Do not amend, squash, tag, or push the commit.
