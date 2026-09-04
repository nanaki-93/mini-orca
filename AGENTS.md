# AGENTS.md

## Project overview

Mini-Orca is a local-first coding assistant with:

- a Go 1.22 daemon in `cmd/` and `internal/`;
- a Kotlin/Compose Desktop client in `desktop/`;
- a loopback-only HTTP API used by the desktop client.

The product is deliberately focused: a user selects one project, one file, one
symbol, and explicitly reviews a generated candidate before applying it.

## Working conventions

- Work to a senior-engineer standard: understand the surrounding design before
  editing it, make deliberate trade-offs, anticipate failure modes and edge
  cases, and leave the codebase easier to understand and maintain.
- Keep changes narrowly scoped to the requested behavior. Preserve the
  preview-first workflow: do not introduce automatic writes, commits, or
  multi-file mutations.
- Write production-quality code. Use precise domain-oriented names, small
  cohesive functions, single-purpose types, explicit boundaries, and
  straightforward control flow. Comments should explain intent or constraints,
  not restate what the code already says.
- Follow KISS: implement the simplest complete solution that satisfies the
  current requirement. Avoid speculative abstractions, premature optimization,
  clever shortcuts, and unnecessary indirection.
- Be pragmatic: fit changes into the existing architecture and conventions,
  balance correctness with maintainability, and introduce abstractions only
  when they solve a concrete problem. Prefer clear, boring code over novelty.
- Enforce DRY without over-abstracting. Do not duplicate business rules,
  validation, transformations, or error handling. Extract shared behavior at
  the narrowest sensible boundary when duplication is real and likely to be
  maintained together; do not unify code that only happens to look similar.
- Make invalid states and failures explicit. Validate inputs at boundaries,
  preserve useful error context, avoid hidden side effects, and never silently
  ignore an error unless that behavior is intentional and documented.
- Keep APIs and modules cohesive. Preserve encapsulation, minimize mutable
  state, and avoid widening public interfaces unless the requirement demands it.
- Treat tests as part of the implementation. Cover the changed behavior and
  meaningful edge cases, keep tests deterministic and behavior-focused, and
  avoid tests that merely mirror implementation details.
- Before handoff, review the diff as a senior engineer would: remove dead code,
  debug output, stale comments, accidental duplication, and unrelated changes;
  confirm naming, error paths, and tests are appropriate for production.
- Do not preserve legacy implementations alongside replacements. When changing
  a function, replace the old implementation and remove obsolete helpers,
  branches, call sites, tests, and documentation made redundant by the change.
  Preserve required public behavior unless the requested change explicitly
  includes a compatibility break or migration.
- Prefer existing package boundaries and error/logging patterns over adding new
  abstractions.
- Do not edit generated build output, including `build/`, `desktop/build/`,
  `desktop/.gradle/`, or `desktop/.kotlin/`.
- Do not add credentials, provider tokens, or local `config.yaml` files to the
  repository. Use `config.example.yaml` for configuration examples.
- Keep the daemon loopback-only by default. Any externally reachable binding
  must be explicit configuration.

## Go daemon

- Target Go 1.22 and keep production code under `cmd/` and `internal/`.
- Format Go files with `go fmt ./...`; use idiomatic errors and retain useful
  context when wrapping failures.
- Add or update package tests when changing daemon behavior.
- Run focused tests while iterating; before handoff, run the relevant checks
  from the validation section when the environment permits.

## Desktop client

- Before UI work, read and follow `desktop/UI_DESIGN_GUIDELINES.md`. Treat the
  supplied mock as the visual reference and use the shared IDE design system,
  not unstyled Material/Swing defaults or arbitrary stacked cards.
- Keep desktop changes within `desktop/` and use the Gradle wrapper; do not
  require a globally installed Gradle.
- Preserve keyboard navigation, text labels for state, responsive drawer
  behavior below 1000dp, and read-only source/diff views.
- Run `./desktop/gradlew -p desktop test` for desktop-only changes.

## Validation

- Go formatting check: `make fmt-check`
- Go tests: `go test ./...`
- Race tests: `make test-race`
- Static analysis: `make vet`
- Desktop tests: `./desktop/gradlew -p desktop test`
- Full supported validation: `make check`

Do not run Docker cleanup or destructive make targets unless the user asks for
them explicitly.

## Handoff

Report the behavior changed, tests run (and any not run), and any configuration
or migration steps needed. Do not create commits unless explicitly requested.
