# Documentation and plans

Read [../AGENTS.md](../AGENTS.md) first. Use these rules for root Markdown and
`tasks/` documentation too.

- Document current behavior, a real contract or an unresolved decision. Verify
  claims against code/tests. Distinguish intended behavior from delivered behavior
  and measured evidence from an assumption.
- Update the existing owner of a fact: root README for setup, `CONFIG.md` and
  `config.example.yaml` for configuration, `api-contract.md`/`openapi.yaml` for the
  wire contract, desktop guides for UI/runtime, `PLAN.md` for product decisions and
  `docs/tasks.md` for its specific ordered queue. Link instead of duplicating prose.
- Keep agent instructions short and actionable. Shared rules belong in root
  `AGENTS.md`; area rules belong in the relevant area file and must be linked from
  the root. Update affected instructions when ownership or commands change.
- UI specifications and acceptance criteria should describe the interaction,
  necessary labels/states and accessibility expectations. Treat visual references
  as optional unless the current request chooses one; see the
  [UI guidelines](../desktop/UI_DESIGN_GUIDELINES.md). Distinguish proposed copy
  from text already in the product.
- Do not turn an ordinary edit into a new plan, completion report or historical
  transcript. Update a task ledger only when the request belongs to that ledger.
  Preserve historical receipts, budgets, verdicts and sealed evaluation material.
- Task cards and execution guides apply to the work they describe. Do not infer
  fresh commit, scheduler or model-call authorization from historical instructions.
- Examples must use real routes/options and placeholder secrets/portable paths.
  Keep API examples consistent with guard fields, error states and Kotlin/Go types.
  `internal/version/version.go` owns the release version; check it before changing
  release references.
- Record only checks actually performed. Separate unit/fixture evidence from
  native, packaging or live-provider evidence. Include a limitation where the
  corresponding claim would otherwise be misleading.

## Verification

For prose-only changes, review the diff, run `git diff --check` and verify local
links, file paths and documented commands. No Go/desktop suite is needed solely
for wording changes.

For API route or release-contract documentation changes, run
`go test ./cmd/daemon -run 'Test(ReleaseDocumentationUsesCanonicalVersion|DocumentedRoutesAreHandledByDaemon)$'`
and any additional tests for the affected contract. These route tests do not prove
payload semantics; cover changed fields/errors in handler and desktop contract tests.
Build-procedure changes require validating the affected procedure when available.
