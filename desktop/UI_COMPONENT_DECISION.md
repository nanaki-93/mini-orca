# Desktop component decision

## Jewel evaluation — 2026-09-04

Mini-Orca is a standalone Compose Desktop application pinned to Kotlin 2.0.21,
Compose Multiplatform 1.7.0, and Gradle 8.6. It does not run inside the IntelliJ
Platform.

JetBrains documents standalone Jewel use through
`org.jetbrains.jewel:jewel-int-ui-standalone:<version>` in the
[Jewel README](https://github.com/JetBrains/jewel). The current official
[Jewel release notes](https://github.com/JetBrains/intellij-community/blob/master/platform/jewel/RELEASE%20NOTES.md)
list Jewel 0.40 against Compose Multiplatform 1.11.0 and Jewel 0.29 against 1.8.2;
both are newer than Mini-Orca's pinned Compose 1.7.0. The same notes record that
the standalone artifacts use their own dependency setup and that experimental native
popups can require JetBrains Runtime or extra configuration, fall back to Compose,
and affect UI tests. They also contain accessibility fixes, but do not establish
end-to-end accessibility coverage for this application's popup workflow.

## Decision

Do not add Jewel for Task 154. No supported current Jewel release is compatible
with the pinned Compose 1.7.0 toolchain, and adopting it would require a dependency
and theme migration larger than popup styling. Mini-Orca keeps Compose Material's
mature Desktop `DropdownMenu` placement, keyboard traversal, Escape/outside dismissal,
and scrolling behavior, then applies the existing token system through a narrow shared
menu-surface and menu-row layer. No Kotlin, Compose, Gradle, repository, or runtime
configuration changes are made by this task.

## Verification boundary

The pinned Compose raster scene exposes real detached popup semantics but does not
composite their window layer into its PNG. The desktop visual test adapter therefore
opens the actual Project and Preview menus for callbacks, disabled state, scrolling,
dismissal, and focus restoration, and renders the same production `IdePopupMenuSurface`
inline for pixel review. Native popup layering and OS keyboard traversal remain release
operator checks for Tasks 159 and 160.
