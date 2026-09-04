# 134 — Generate and transport contextual engineering insights

## Status

Complete

## Goal

Return concise optional expert insight with existing analysis and declaration
proposals, using their existing calls and authoritative result identities.

## Depends on

Task 133.

## Required task commentary

- Before editing, post `Starting Task 134` and name the response/prompt contracts,
  conversions, persistence, freshness, and focused checks.
- After the commit, post `Task 134 complete` with results, exact commit hash, and
  Task 135 as the next task.

## Implementation

- Implement the shared `EngineeringInsight` value and bounded validator from feature
  plan section 3: required mechanism/relevance, optional trade-off/lesson, trimmed
  text, and at most 1,000 combined Unicode code points.
- Decode an optional insight independently of the strict parent response. Missing
  or null is valid; malformed optional content is omitted with a bounded sanitized
  diagnostic. Invalid required parent content still fails normally.
- Extend project/file analysis, risks, suggestions, and declaration proposal outputs
  with optional `engineering_insight`. Limit file-analysis output to three insights
  total across its report/risks/suggestions and a proposal to one insight.
- Prompt for roughly 50–90 words grounded in supplied evidence: mechanism, hidden
  assumption, engineering consequence, or important trade-off. Omit generic or
  introductory content and omit insight entirely when nothing worthwhile exists.
- Keep `analyze`, `bug`, and `function` routing unchanged for their existing parent
  operations. No additional provider call, follow-up, or new insight endpoint.
- Carry each insight through the actual owning Go report/finding/proposal, handlers,
  Kotlin serialization models, and feature state. Inspect all conversions, clones,
  cache reads/writes, and response fixtures rather than only adding wire fields.
- Keep finding identity and triage independent of insight wording. Deterministic
  parser/vet/test findings do not acquire invented AI explanations.
- Bind proposal insights server-side to exact draft ID/revision/hash. Manual draft
  edits invalidate their current-candidate applicability; canceled/late responses
  must not attach insight to a different owner. Do not add durable chat storage.
- Store only sanitized explanatory metadata with the parent. The model cannot
  assign trusted paths, hashes, actions, provider authority, or navigation targets.
- Version changed prompts/contracts deliberately; keep old insight-less metadata
  readable and refresh only on explicit actions. Replace obsolete prompt/parser
  branches instead of retaining two generation implementations.
- Update live API/OpenAPI response schemas and contract tests with implemented
  fields. No Performance routes, new UI panel, or new configuration scope yet.

## Acceptance criteria

- Useful optional insights reach the Desktop in existing parent responses with no
  extra model request; valid output without insight remains fully usable.
- Malformed optional insight cannot reject an otherwise valid proposal/analysis.
- Owner identity, finding triage, cache compatibility, source-free storage, remote
  confirmation, and current draft/Apply guards remain intact.
- Manual revisions cannot reuse a previous explanation as current Review evidence.
- Model prose is advisory and neither becomes source comments nor authorizes work.

## Verification

Add focused tests for optional-field validation including Unicode/limits, strict
parent parsing, prompt call counts and routing, all serialization/conversion paths,
old cache reads, sanitized metadata, unchanged finding IDs, and stale/late/manual
draft cases. Run:

```text
make fmt-check
go test ./...
make test-race
make vet
./desktop/gradlew -p desktop spotlessCheck detekt test
git diff --check
```

Use deterministic fake provider fixtures. Do not treat those fixtures as proof of
live model insight quality; that is part of final manual acceptance.

## Commit

After checks pass, record evidence, mark this task Complete, move it under
`tasks/completed/`, update the index, stage only Task 134 work, and inspect the
complete staged diff. Create exactly one commit:

```text
feat(analysis): add contextual engineering insights
```

Do not amend, squash, tag, or push. Verify no Task 134 work remains uncommitted.

## Verification evidence

- `make fmt-check` — passed.
- `go test ./...` — passed.
- `make test-race` — passed.
- `make vet` — passed.
- `./desktop/gradlew -p desktop spotlessCheck detekt test` — passed.
- `git diff --check` — passed.
- Added deterministic parser/owner tests for required fields, unknown fields,
  Unicode bounds, malformed optional content, finding-ID stability, and manual
  draft edits clearing the current-candidate insight. Provider fixtures verify
  transport only; live content quality remains final manual acceptance.
