# Engineering insights and Performance acceptance

**Status:** Complete with recorded manual follow-ups and the user-authorized
quality-gate deferral below.

## Automated evidence

Run on 2026-09-04 after Tasks 133–138:

- `make fmt-check` — passed.
- `go test ./...` — passed.
- `make test-race` — passed.
- `make vet` — passed.
- `./desktop/gradlew -p desktop spotlessCheck detekt test` — passed.
- `make check` — passed.
- `git diff --check` — passed.

`make quality` still exits nonzero because its repository-wide complexity command
reports existing high-complexity production functions, including the bounded
Performance job controller. The user explicitly authorized deferring this rule
after Task 136. Staticcheck and dead-code reachability passed; obsolete public
Performance wrappers were removed in Task 138.

## Contract and safety review

- Optional Engineering insights are bounded, strict-decoded, omitted when malformed,
  carried independently of finding identity, and shown only in the reusable collapsed
  in-page panel.
- Manual draft edits clear the current proposal insight. Stale owners are labelled and
  cannot navigate as current evidence.
- Performance remains an explicit source-based review. It does not execute project
  code, benchmark, profile, score, apply, or claim measured improvement.
- The Performance queue binds project/revision, policy fingerprint, file hashes,
  provider identity, and a bounded execution budget. Start/resume reconfirm remote
  provider use; pause/cancel/restart retain only compatible partial cached reports.
- Job progress and Performance reports are source-free. The report exposes only a
  finding-ID-to-project-relative-path mapping, not source text, prompts, or secrets.
- The Performance workspace starts no provider work on entry. It preserves the four
  numbered workspace shortcuts and uses the existing Editor/composer for an exact Go
  declaration handoff.

## Manual acceptance follow-ups

Not run in this headless environment; a release operator should complete these using
a configured local or remote provider and representative non-secret fixtures:

- Wide, exactly `1000dp`, narrow, and text-scaling layouts; keyboard focus and long
  path/content smoke checks listed in `desktop/KEYBOARD_SMOKE_CHECKLIST.md`.
- Remote-provider confirmation for a previewed Performance queue, plus pause/resume,
  cancellation, restart, empty, partial, stale, and failed review presentation.
- Content-quality review of real model output for concise grounded insights and
  conditions/trade-offs, including appropriate omission for trivial changes.

No configuration migration is required. Existing insight-less cache metadata remains
usable; Performance cache and job state are refreshed only by an explicit new review.
