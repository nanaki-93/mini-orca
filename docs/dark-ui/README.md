# Dark UI direction

Use the [dark Mini-Orca mock](../../design/ui-mocks/ChatGPT%20Image%20Sep%204,%202026,%2002_37_04%20PM.png)
as the visual anchor: charcoal chrome, clear pane ownership, source as the primary
surface, a restrained blue selection edge, compact context and dense findings.
Use the [Aurora mock](../../design/ui-mocks/Gemini_Generated_Image_54u1al54u1al54u1.jpeg)
only for tab hierarchy and readable context grouping.

Current rules and tokens live in
[desktop/UI_DESIGN_GUIDELINES.md](../../desktop/UI_DESIGN_GUIDELINES.md).
For new or changed flows, follow its
[self-explanatory UI and copy rules](../../desktop/UI_DESIGN_GUIDELINES.md#self-explanatory-ui-and-copy).
Use the mocks for visual hierarchy; their titles, subtitles and explanatory text
are not required content for every pane.
The dark theme, navigation and Jewel migration are already implemented. Do not
repeat those migrations or restore the older navy/purple palette.

The current implementation in [PLAN.md](../../PLAN.md) removes inert previews, makes
the selected-function workflow explicit and records supported-host native checks.
Adapt the mock's sample tabs, terminal,
profile, scores and multi-file extraction to Mini-Orca's actual capabilities.
Native window controls and explicit Review before Apply remain intentional.

The old dark plan/baseline/contrast/acceptance documents were consolidated here and
in [release acceptance](../RELEASE_ACCEPTANCE.md). UI-04 closes the attainable Task
149 native checks; REL-01 retains the explicit release limitations.
Current measured token pairs remain in [UI_CONTRAST.md](../../desktop/UI_CONTRAST.md).
Full historical instructions and old measurements are recoverable from Git.
