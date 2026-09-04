package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

class DesktopKeyboardNavigationTest {
  @Test
  fun responsiveShellKeepsTheExactBreakpointDocked() {
    assertEquals(
        ResponsiveShellPresentation(
            left = ResponsiveShellRegion.Docked,
            right = ResponsiveShellRegion.Docked,
            bottom = ResponsiveShellRegion.Docked,
        ),
        responsiveShellPresentation(1_000f),
    )
    assertEquals(
        ResponsiveShellPresentation(
            left = ResponsiveShellRegion.Drawer,
            right = ResponsiveShellRegion.Drawer,
            bottom = ResponsiveShellRegion.Overlay,
        ),
        responsiveShellPresentation(999f),
    )
  }

  @Test
  fun tabGroupArrowsMoveFocusWithoutActivatingTheDestination() {
    val entries = listOf("Source", "Review", "History")

    assertEquals(
        TabGroupInteraction("History"),
        tabGroupInteraction(entries, "Source", TabGroupKey.Previous),
    )
    assertEquals(
        TabGroupInteraction("Review"),
        tabGroupInteraction(entries, "Source", TabGroupKey.Next),
    )
    assertEquals(
        TabGroupInteraction("Source", "Source"),
        tabGroupInteraction(entries, "Source", TabGroupKey.Activate),
    )
  }

  @Test
  fun activityRailCyclesOnlyTheFiveRetainedWorkspaces() {
    val entries = LeftToolWindow.entries.toList()

    assertEquals(
        LeftToolWindow.Analysis,
        tabGroupInteraction(entries, LeftToolWindow.Summary, TabGroupKey.Next)?.focused)
    assertEquals(
        LeftToolWindow.Editor,
        tabGroupInteraction(entries, LeftToolWindow.Summary, TabGroupKey.Previous)?.focused)
    assertEquals(
        TabGroupInteraction(LeftToolWindow.Editor, LeftToolWindow.Editor),
        tabGroupInteraction(entries, LeftToolWindow.Editor, TabGroupKey.Activate),
    )
  }

  @Test
  fun escapeSelectsOnlyTheTopmostTransientSurface() {
    assertEquals(
        TransientSurface.Context,
        topmostTransientSurface(
            contextVisible = true,
            paletteVisible = true,
            statusDetailsVisible = true,
            bottomToolsVisible = true,
            drawerVisible = true,
        ),
    )
    assertEquals(
        TransientSurface.BottomTools,
        topmostTransientSurface(
            contextVisible = false,
            paletteVisible = false,
            statusDetailsVisible = false,
            bottomToolsVisible = true,
            drawerVisible = true,
        ),
    )
    assertNull(
        topmostTransientSurface(
            contextVisible = false,
            paletteVisible = false,
            statusDetailsVisible = false,
            bottomToolsVisible = false,
            drawerVisible = false,
        ),
    )
  }

  @Test
  fun leavingEditorClosesOnlyAnEditorDrawer() {
    assertEquals(true, closesEditorDrawerOnWorkspaceChange(Workspace.Editor, Workspace.Bugs))
    assertEquals(false, closesEditorDrawerOnWorkspaceChange(Workspace.Editor, Workspace.Editor))
    assertEquals(false, closesEditorDrawerOnWorkspaceChange(Workspace.Analysis, Workspace.Bugs))
  }

  @Test
  fun focusedTabsKeepTextualAccessibilityStateAtCompactWidths() {
    assertEquals(
        "Editor tool window, selected, focused",
        toolWindowSemanticsLabel(LeftToolWindow.Editor, selected = true, focused = true),
    )
    assertEquals(
        "AI Context tool window tab, not selected, focused",
        rightToolWindowTabDescription(
            RightToolWindow.Context,
            selected = false,
            focused = true,
        ),
    )
    assertEquals(
        "Output tool window tab, 2 lines, selected, focused",
        bottomToolWindowTabDescription(
            BottomToolWindow.Output,
            selected = true,
            summary = "2 lines",
            focused = true,
        ),
    )
  }

  @Test
  fun compactToolbarAndBreadcrumbPoliciesKeepLongTextBounded() {
    assertEquals(ToolbarPresentation(false, false, false), toolbarPresentation(999f))
    assertEquals(ToolbarPresentation(true, false, true), toolbarPresentation(1_000f))
    assertEquals(
        "very / … / main.go / Run",
        editorBreadcrumbLabel("very/long/project/path/main.go", "Run"),
    )
  }
}
