# AGENTS.md

## Project overview

Mini-Orca is a local-first coding assistant with:

- a Go 1.22 daemon in `cmd/` and `internal/`;
- a Kotlin/Compose Desktop client in `desktop/`;
- a loopback-only HTTP API used by the desktop client.

The product is deliberately focused: a user selects one project, one file, one
symbol, and explicitly reviews a generated candidate before applying it.

## Working conventions

- Keep changes narrowly scoped to the requested behavior. Preserve the
  preview-first workflow: do not introduce automatic writes, commits, or
  multi-file mutations.
- Apply Clean Code principles consistently: use clear names, small cohesive
  functions, single-purpose types, explicit boundaries, and straightforward
  control flow.
- Follow KISS: choose the smallest understandable design that meets the
  requirement; avoid speculative abstractions and unnecessary indirection.
- Do not repeat logic. Extract and reuse a well-named local helper or shared
  component when the same behavior would otherwise be maintained in more than
  one place.
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
