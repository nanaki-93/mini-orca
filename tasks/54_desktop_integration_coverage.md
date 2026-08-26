# 54 — Add desktop integration coverage

## Status

Pending

## Goal

Prove the new desktop workspaces and file-bound draft workflow across API, controller, and presentation boundaries.

## Depends on

Tasks 46, 47, 48, 49, 50, 51, 52, and 53.

## Implementation

- Complete fake-transport serialization tests for every new API method and error/no-content response.
- Add controller tests for stale responses, file isolation, Analyze-all polling, finding navigation, chat binding, draft invalidation, and Apply eligibility.
- Add presentation tests for deterministic/model separation, verified/AI classification, brief states, disabled reasons, and responsive navigation.
- Add a Compose semantics smoke flow for Summary → Bugs → Editor → chat → edited draft → validation/check review.
- Update the desktop manual smoke and accessibility checklists.

## Acceptance criteria

- Tests prove chat and Apply cannot target a file other than the open file.
- Tests prove every manual draft edit invalidates validation and checks.
- Tests prove verified/tool and AI findings cannot be conflated.
- Desktop tests are deterministic and do not require a live daemon or model.

## Verification

- Run `./desktop/gradlew -p desktop test` repeatedly and `git diff --check`.
