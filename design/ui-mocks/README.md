# Mini-Orca UI directions

## Selected direction

The selected implementation direction is the hybrid: Direction A's persistent
desktop shell, Direction C's Target → Draft → Verify → Apply flow and color
system, and Direction B's plain-language final Apply decision. See
`IMPLEMENTATION_PLAN.md` for the concrete Compose migration.

These high-fidelity desktop mocks compare three visual and interaction directions
using the same state: a file-scoped `RegisterRoutes` draft has been generated,
validated, checked, and is waiting for explicit review.

They preserve the product's core boundaries:

- one active project, file, and declaration;
- source and composed diff remain read-only;
- draft identity, validation, and focused checks stay visible;
- advisory impact never implies multi-file mutation;
- Apply remains explicit and names the only writable target.

## A — Workbench

A dense, dark developer workspace with permanent project navigation, inline diff,
and a review gate beside the code.

Best for frequent users who value scan speed and persistent context. It has the
highest information density and the smallest conceptual jump from an IDE.

Files: `direction-a-workbench.html` and `direction-a-workbench.png`.

## B — Calm Review

A light, editorial review surface with a quiet working-context column, a large
composed diff, and a dedicated decision panel.

Best for clarity, trust, and approachability. It gives the safety model the most
prominent visual treatment, at the cost of showing slightly less project context.

Files: `direction-b-calm-review.html` and `direction-b-calm-review.png`.

## C — Focus Flow

A dark, guided four-stage flow: Target, Draft, Verify, Apply. The verification
stage uses a side-by-side declaration comparison and reveals evidence only when
it is relevant.

Best for progressive disclosure and deliberate sequencing. It is the strongest
guardrail against skipped steps, but is less free-form than the Workbench.

Files: `direction-c-focus-flow.html` and `direction-c-focus-flow.png`.

## Suggested product direction

Use A as the desktop shell, borrow C's explicit workflow stages for Editor, and
borrow B's plain-language safety summary for the final Apply decision. This keeps
Mini-Orca efficient without losing the calm, preview-first promise.

The HTML files are self-contained except for the shared `mock-base.css`; no
external fonts, images, scripts, or network requests are required.
