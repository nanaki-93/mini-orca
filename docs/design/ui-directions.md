# Web workspace design directions

These are proposed alternatives to the current browser workspace, not shipped
product changes. Open [the interactive comparison](ui-directions.html) in a
browser. It is self-contained and works offline.

All four examples show the same fictional task, **Validate incoming requests**,
so the layout is the variable. The preview has no API connection, provider calls,
project execution, or source writes. Review interactions only change local demo
state. Check results are explicitly labeled as samples.

| Direction | Structure | Visual style | Best fit | Tradeoff |
| --- | --- | --- | --- | --- |
| **Orbit** | Activity rail, contextual file explorer, central editor | Deep navy, lavender, precise borders | Frequent code review and quick file switching | More persistent navigation |
| **Paper** | Top navigation, centered document, inline file selection | Warm light surfaces, ink text, muted green | Reading, planning, and slower review | Less code visible at once |
| **Pulse** | Task inbox beside the selected task | Graphite green, mint accents, compact rows | Reviewing several proposals in sequence | Less width for long code lines |
| **Focus** | Guided task stages, a single canvas, disclosed navigation | Midnight blue, soft blue accents, generous spacing | The clearest next action with minimal visual noise | More navigation to switch tasks |

For a professional IDE direction, start with **Orbit**. For the simplest
experience, start with **Focus**. The chosen structure can use the current
Midnight palette; choosing a layout does not require choosing its example colors.

Each example supports task-section navigation, file selection, local review
progress, and an explanation disclosure. Controls that would approve or apply
real changes instead open an explicit preview notice. The additional task entries
in Pulse illustrate the queue structure and are display-only examples.

The `style` query parameter selects `orbit`, `paper`, `pulse`, or `focus`.
`capture=1` hides the comparison controls for inspecting a single concept.
The production UI remains in `internal/api/handlers/webui.html`.
