package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class DesktopLayoutStateTest {
  @Test
  fun navigationPreservesStoredDestinationsAndPreferredPaneWidths() {
    withPreferences { preferences ->
      LeftToolWindow.entries.forEach { destination ->
        val saved =
            DesktopLayoutState(
                activeLeftToolWindow = destination, explorerWidth = 340f, actionWidth = 440f)
        DesktopLayoutStore(preferences).save(saved)
        assertEquals(saved.withBottomCollapsed(true), DesktopLayoutStore(preferences).load())
        assertEquals(
            destination, leftToolWindowForWorkspace(workspaceForLeftToolWindow(destination)))
      }
      preferences.put("ide-left-tool", "Problems")
      assertEquals(
          Workspace.Bugs,
          workspaceForLeftToolWindow(DesktopLayoutStore(preferences).load().activeLeftToolWindow))
    }
  }

  @Test
  fun defaultsKeepDockedPaneDimensionsAndACollapsedBottomSummary() {
    val layout = DesktopLayoutState()

    assertEquals(LeftToolWindow.Editor, layout.activeLeftToolWindow)
    assertEquals(RightToolWindow.Context, layout.activeRightToolWindow)
    assertEquals(EditorSurface.Source, layout.editorSurface)
    assertEquals(220f, layout.explorerWidth)
    assertEquals(300f, layout.actionWidth)
    assertEquals(220f, layout.bottomHeight)
    assertTrue(layout.bottomCollapsed)
  }

  @Test
  fun transitionsKeepIndependentRegionsAndOnlyChangePresentationState() {
    val initial = DesktopLayoutState()
    val updated =
        initial
            .openLeft(LeftToolWindow.Analysis)
            .openRight(RightToolWindow.Assistant)
            .openTerminal()
            .withEditorSurface(EditorSurface.Review)
            .withFocus(DesktopFocusRegion.BottomToolWindow)

    assertEquals(LeftToolWindow.Analysis, updated.activeLeftToolWindow)
    assertEquals(RightToolWindow.Assistant, updated.activeRightToolWindow)
    assertEquals(EditorSurface.Review, updated.editorSurface)
    assertTrue(updated.leftToolWindowVisible)
    assertTrue(updated.rightToolWindowVisible)
    assertFalse(updated.bottomCollapsed)
    assertEquals(DesktopFocusRegion.BottomToolWindow, updated.lastFocusedRegion)
    assertEquals(initial.explorerWidth, updated.explorerWidth)
    assertEquals(initial.actionWidth, updated.actionWidth)
  }

  @Test
  fun preparedSecurityFixAlwaysShowsAndFocusesTheAssistantComposer() {
    val layout =
        layoutForPreparedRequest(
            DesktopLayoutState(
                activeRightToolWindow = RightToolWindow.Review, rightToolWindowVisible = false))

    assertEquals(RightToolWindow.Assistant, layout.activeRightToolWindow)
    assertTrue(layout.rightToolWindowVisible)
    assertEquals(DesktopFocusRegion.RightToolWindow, layout.lastFocusedRegion)
  }

  @Test
  fun dimensionsClampAtSafeBounds() {
    val layout =
        DesktopLayoutState().withExplorerWidth(-1f).withActionWidth(10_000f).withBottomHeight(-1f)

    assertEquals(DesktopLayoutState.MIN_EXPLORER_WIDTH, layout.explorerWidth)
    assertEquals(DesktopLayoutState.MAX_ACTION_WIDTH, layout.actionWidth)
    assertEquals(DesktopLayoutState.MIN_BOTTOM_HEIGHT, layout.bottomHeight)
  }

  @Test
  fun storeKeepsExistingPanePreferencesAndDefaultsMissingNavigationToEditor() {
    withPreferences { preferences ->
      preferences.putFloat("explorer-width", 320f)
      preferences.putFloat("action-width", 440f)

      val layout = DesktopLayoutStore(preferences).load()

      assertEquals(320f, layout.explorerWidth)
      assertEquals(440f, layout.actionWidth)
      assertEquals(DesktopLayoutState.DEFAULT_BOTTOM_HEIGHT, layout.bottomHeight)
      assertEquals(LeftToolWindow.Editor, layout.activeLeftToolWindow)
      assertTrue(layout.bottomCollapsed)
    }
  }

  @Test
  fun storeRecoversRetiredAndCorruptNavigationValuesWithoutLosingPanePreferences() {
    withPreferences { preferences ->
      preferences.put("explorer-width", "invalid")
      preferences.putFloat("action-width", 9_999f)
      preferences.put("ide-bottom-height", "invalid")
      preferences.put("ide-left-tool", "Project")
      preferences.put("ide-focus-region", "Unknown")

      val recovered = DesktopLayoutStore(preferences).load()

      assertEquals(DesktopLayoutState.DEFAULT_EXPLORER_WIDTH, recovered.explorerWidth)
      assertEquals(DesktopLayoutState.MAX_ACTION_WIDTH, recovered.actionWidth)
      assertEquals(DesktopLayoutState.DEFAULT_BOTTOM_HEIGHT, recovered.bottomHeight)
      assertEquals(LeftToolWindow.Editor, recovered.activeLeftToolWindow)
      assertEquals(DesktopFocusRegion.Editor, recovered.lastFocusedRegion)

      DesktopLayoutStore(preferences).save(recovered)
      assertEquals("Editor", preferences.get("ide-left-tool", ""))

      val saved =
          recovered
              .openLeft(LeftToolWindow.Problems)
              .openRight(RightToolWindow.Review)
              .openTerminal()
              .withBottomHeight(300f)
              .withFocus(DesktopFocusRegion.RightToolWindow)
      DesktopLayoutStore(preferences).save(saved)

      assertEquals(saved.withBottomCollapsed(true), DesktopLayoutStore(preferences).load())

      preferences.put("ide-left-tool", "Unknown")
      assertEquals(
          LeftToolWindow.Editor, DesktopLayoutStore(preferences).load().activeLeftToolWindow)
    }
  }

  @Test
  fun storeKeepsTerminalDimensionsButNeverRestoresAnOpenSession() {
    withPreferences { preferences ->
      val layout =
          DesktopLayoutState()
              .openTerminal()
              .withBottomHeight(360f)
              .withFocus(DesktopFocusRegion.BottomToolWindow)

      DesktopLayoutStore(preferences).save(layout)

      assertEquals(layout.withBottomCollapsed(true), DesktopLayoutStore(preferences).load())
    }
  }

  @Test
  fun wideLayoutBoundaryStaysAtExactlyOneThousandDp() {
    assertFalse(useNarrowLayout(1_000f))
    assertTrue(useNarrowLayout(999f))
  }

  @Test
  fun dockedPaneWidthsProtectTheEditorWithoutChangingStoredPreferences() {
    val constrained =
        dockedPaneWidths(1_000f, preferredExplorerWidth = 520f, preferredActionWidth = 560f)

    assertEquals(DesktopLayoutState.MIN_EXPLORER_WIDTH, constrained.explorer)
    assertEquals(340f, constrained.action)
    assertEquals(MIN_EDITOR_WIDTH, constrained.editor)
    assertEquals(520f, DesktopLayoutState().withExplorerWidth(520f).explorerWidth)
    assertEquals(560f, DesktopLayoutState().withActionWidth(560f).actionWidth)
  }

  @Test
  fun dockedPaneWidthsRestorePreferredDimensionsWhenTheViewportGrows() {
    val preferred =
        dockedPaneWidths(1_440f, preferredExplorerWidth = 220f, preferredActionWidth = 300f)

    assertEquals(220f, preferred.explorer)
    assertEquals(300f, preferred.action)
    assertEquals(840f, preferred.editor)
  }

  @Test
  fun defaultDocksKeepSourceUsefulAtTheDockedBreakpoint() {
    val defaults = DesktopLayoutState()
    val constrained = dockedPaneWidths(1_000f, defaults.explorerWidth, defaults.actionWidth)

    assertEquals(400f, constrained.editor)
    assertEquals(defaults.explorerWidth, constrained.explorer)
    assertEquals(defaults.actionWidth, constrained.action)
  }

  @Test
  fun dockedPaneWidthsReserveTheFrameWithoutPersistingTemporaryClamps() {
    val constrained =
        dockedPaneWidths(1_000f, preferredExplorerWidth = 520f, preferredActionWidth = 560f)

    assertEquals(
        1_000f,
        TOOL_WINDOW_BAR_WIDTH +
            WORKSPACE_FRAME_INSET * 2 +
            RESIZE_DIVIDER_WIDTH * 2 +
            constrained.explorer +
            constrained.action +
            constrained.editor,
    )
    assertEquals(520f, DesktopLayoutState().withExplorerWidth(520f).explorerWidth)
    assertEquals(560f, DesktopLayoutState().withActionWidth(560f).actionWidth)
  }

  @Test
  fun everyLegacyBottomDestinationRestoresCollapsedAndRetainsPaneDimensions() {
    listOf("Problems", "Checks", "Output", "Terminal", "Unknown").forEach { destination ->
      withPreferences { preferences ->
        preferences.put("ide-bottom-tool", destination)
        preferences.putBoolean("ide-bottom-visible", true)
        preferences.putBoolean("ide-bottom-collapsed", false)
        preferences.putFloat("ide-bottom-height", 340f)
        preferences.putFloat("explorer-width", 320f)
        preferences.putFloat("action-width", 440f)
        val store = DesktopLayoutStore(preferences)
        val layout = store.load()
        assertTrue(layout.bottomCollapsed)
        assertEquals(340f, layout.bottomHeight)
        assertEquals(320f, layout.explorerWidth)
        assertEquals(440f, layout.actionWidth)
        store.save(layout.openTerminal())
        assertTrue(store.load().bottomCollapsed)
        assertEquals(340f, store.load().bottomHeight)
        listOf("ide-bottom-tool", "ide-bottom-visible", "ide-bottom-collapsed").forEach {
          assertEquals(null, preferences.get(it, null))
        }
      }
    }
  }

  private fun withPreferences(test: (InMemoryPreferences) -> Unit) = test(InMemoryPreferences())
}
