# Desktop component decision

## Active decision — standalone Jewel adoption (2026-09-04)

Mini-Orca will adopt standalone Jewel as its desktop component foundation. The
validated coordinate is
`org.jetbrains.jewel:jewel-int-ui-standalone:0.40.0-262.10315.125`; it will enter
production in Task 162, not before. The temporary Task 161 spike compiled and
rendered the theme, action, tab, tree/disclosure, popup menu, and text-input APIs
under JBR 25. The complete evidence, resolved graph, runtime details, and test
boundary are in [UI_PRECISION_BASELINE.md](UI_PRECISION_BASELINE.md).

| Concern | Adopted decision |
| --- | --- |
| Component library | Jewel standalone, not IntelliJ Platform integration and not a second custom control framework. |
| Pinned platform | Jewel `0.40.0-262.10315.125`; Kotlin JVM/serialization/Compose compiler `2.3.20`; Compose Multiplatform `1.11.0`; Gradle wrapper `9.1.0`; JBR `25.0.4+1-b508.27`. |
| Bytecode and analysis | JBR 25 runs the toolchain/application; source targets JVM 22 solely because stable Detekt 1.23.8 cannot run on JDK 25 or analyze target 25. Remove this workaround when stable Detekt supports it. |
| Application policy | Use Jewel for standard controls and theme roles. Keep Mini-Orca-owned source/diff rendering, state, accessibility labels, callbacks, and preview-first safety behavior. |
| Windows and popups | Retain normal desktop window decoration. No custom titlebar and no experimental popup flag are required. Offscreen popup evidence does not replace Task 170 native keyboard/window checks. |
| Packaging | Package with the matching JBR 25 distribution for each supported target; validate the actual artifact in Task 171. No local JBR path is committed. |

## Migration ownership

| Task | Jewel/migration ownership |
| --- | --- |
| 162 | Add the production dependency and semantic IDE theme. |
| 163 | Shared dense headers, toolbars, tabs, disclosures, dividers, and control states. |
| 164 | Shell surface ownership and continuous pane dividers. |
| 165 | Analysis header controls and flat rows. |
| 166 | Summary and Performance dense flat sections. |
| 167 | Explorer, editor tabs, breadcrumbs, source, and diff chrome. |
| 168 | Context, Assistant, Review, and workflow evidence panes. |
| 169 | Bottom panes, menus/dialogs, and Material bridge removal. |
| 170 | Native/responsive/accessibility validation, including Escape and focus routing. |
| 171 | Package/acceptance ledger and final runtime documentation. |

## Historical note

Task 154 correctly deferred Jewel within its narrower Compose 1.7.0 scope. That
decision is superseded by this approved, tested toolchain migration; the old
Material-only path is not retained as a competing implementation.
