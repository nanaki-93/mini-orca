# Insights and performance implementation baseline

**Recorded:** 2026-09-04

**Starting implementation HEAD:** `a6fc373c5f4b3a62673775a81c43447883e6bd2f`

**Dependency evidence:** Task 132 is Complete in `tasks/completed/`, and its
implementation commit is `a10fbeb0b6c4ee67556503cea803ce4d8bb45c0e`.
The completed IDE plan retains its truthful release-operator limitation: live
viewport, screen-reader, text-scaling, and provider-flow checks need an
interactive Mini-Orca window and provider-backed fixture.

## Current ownership boundaries

- The Go daemon owns indexed source traversal, context policy, model routing,
  report persistence, finding conversion, draft identity, validation, checks,
  Apply, and Undo. It listens only on loopback through the existing API.
- The Desktop presenter owns asynchronous API calls and rejects stale project or
  file results. Composables render immutable state and emit intents; workspace
  selection itself does not initiate analysis or generation.
- Source and diff views are selectable and read-only. Apply and Undo are the
  only source mutations, each bound to project revision, file hash, draft
  identity, checks, and confirmation.
- Normal analysis owns bug findings and its cache. Performance must have a
  separate cache and job namespace and must not reconcile findings through the
  generic `ai` source.
- The existing `analyze` model scope is the only model scope Performance may
  use. Remote destination confirmation remains request-specific and cannot be
  borrowed from another operation.

## Characterization and fixture matrix

| Concern | Existing deterministic evidence | Future task coverage |
| --- | --- | --- |
| Navigation is presentation-only | Desktop presenter, shell, integration, and IDE contract tests | 135, 138 add insight/Performance intents with zero provider calls on display or selection. |
| Read-only preview-first workflow | Go Apply-stage/draft-review tests and Desktop review-state tests | 134–139 preserve draft identity; 138 only pre-fills the existing request. |
| Shortcut and 1000dp boundary | Desktop accessibility, shell, keyboard-navigation, and IDE contract tests | 135 and 138 retain explicit Cmd/Ctrl+1–4 mappings and the exact boundary. |
| Finding identity and triage | Project findings and file-analysis tests | 134 proves insight wording cannot change IDs or triage; 136 keeps Performance outside bug findings. |
| Optional insight payload | No current payload exists | 134 fixtures: absent/null, valid concise, malformed optional, Unicode limit, stale owner, and trivial omission. |
| File review | No current Performance service exists | 136 fixtures: valid-empty, partial invalid output, failed, stale source, canceled, policy-excluded, and oversized files. |
| Project job | No current Performance job exists | 137 fixtures: queue identity, confirmation, pause/resume/restart budgets, late results, coverage, and paging. |
| Desktop Performance | No current Performance workspace exists | 138 fixtures: no-work-on-entry, status/coverage/filter/page states, narrow layout, and exact-target handoff. |

## Contract decisions

- Optional `engineering_insight` parsing fails closed for the insight alone:
  malformed or over-limit optional content is omitted with a bounded sanitized
  diagnostic; valid required parent content remains usable.
- Per-file Performance input and response ceilings are 64 KiB. A response has
  at most five findings. Project pages use bounded server pagination rather
  than an unbounded report payload.
- Project review defaults to 100 selected files (maximum 500) and 15 minutes
  of active execution (maximum 60 minutes). Paused time does not consume the
  budget; restart and resume cannot reset consumed time or attempts.
- Lifecycle states are not-started, running, paused/interrupted, canceled,
  budget-limited, stale, failed, partial, and completed/successful-empty.
  Coverage always distinguishes the selected queue from the eligible project.

## Verification availability

Automated Go and Desktop checks run locally with deterministic fake providers
and temporary project roots. This environment has no interactive Mini-Orca
window or configured provider fixture, so live viewport, screen-reader,
text-scaling, long-content, and provider confirmation flows remain release-
operator checks. They are not automated passes and must remain explicit in the
final acceptance record.
