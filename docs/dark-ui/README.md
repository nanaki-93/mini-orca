# Dark UI direction

Use the [dark Mini-Orca mock](../../design/ui-mocks/ChatGPT%20Image%20Sep%204,%202026,%2002_37_04%20PM.png)
as the visual anchor: charcoal chrome, clear pane ownership, source as the primary
surface, a restrained blue selection edge, compact context and dense findings.
Use the [Aurora mock](../../design/ui-mocks/Gemini_Generated_Image_54u1al54u1al54u1.jpeg)
only for tab hierarchy and readable context grouping.

Current rules and tokens live in
[desktop/UI_DESIGN_GUIDELINES.md](../../desktop/UI_DESIGN_GUIDELINES.md).
The dark theme, navigation and Jewel migration are already implemented. Do not
repeat those migrations or restore the older navy/purple palette.

The next improvements in [PLAN.md](../../PLAN.md) remove inert previews, make the
selected-function workflow obvious and verify the real desktop at narrow widths,
large text and with a screen reader. Adapt the mock's sample tabs, terminal,
profile, scores and multi-file extraction to Mini-Orca's actual capabilities.
Native window controls and explicit Review before Apply remain intentional.

The old dark plan/baseline/contrast/acceptance documents were consolidated here and
in [release acceptance](../RELEASE_ACCEPTANCE.md). Their Task 149 native checks
remain outstanding under UI-04 and REL-01; document consolidation is not a pass.
Current measured token pairs remain in [UI_CONTRAST.md](../../desktop/UI_CONTRAST.md).
Full historical instructions and old measurements are recoverable from Git.
