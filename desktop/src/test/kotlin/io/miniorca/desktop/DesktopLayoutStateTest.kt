package io.miniorca.desktop

import java.util.UUID
import java.util.prefs.Preferences
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class DesktopLayoutStateTest {
  @Test
  fun defaultsKeepDockedPaneDimensionsAndACollapsedBottomSummary() {
    val layout = DesktopLayoutState()

    assertEquals(LeftToolWindow.Editor, layout.activeLeftToolWindow)
    assertEquals(RightToolWindow.Context, layout.activeRightToolWindow)
    assertEquals(BottomToolWindow.Problems, layout.activeBottomToolWindow)
    assertEquals(EditorSurface.Source, layout.editorSurface)
    assertEquals(256f, layout.explorerWidth)
    assertEquals(344f, layout.actionWidth)
    assertEquals(220f, layout.bottomHeight)
    assertTrue(layout.bottomToolWindowVisible)
    assertTrue(layout.bottomCollapsed)
  }

  @Test
  fun transitionsKeepIndependentRegionsAndOnlyChangePresentationState() {
    val initial = DesktopLayoutState()
    val updated =
        initial
            .openLeft(LeftToolWindow.Analysis)
            .openRight(RightToolWindow.Assistant)
            .openBottom(BottomToolWindow.Checks)
            .withEditorSurface(EditorSurface.Review)
            .withFocus(DesktopFocusRegion.BottomToolWindow)

    assertEquals(LeftToolWindow.Analysis, updated.activeLeftToolWindow)
    assertEquals(RightToolWindow.Assistant, updated.activeRightToolWindow)
    assertEquals(BottomToolWindow.Checks, updated.activeBottomToolWindow)
    assertEquals(EditorSurface.Review, updated.editorSurface)
    assertTrue(updated.leftToolWindowVisible)
    assertTrue(updated.rightToolWindowVisible)
    assertTrue(updated.bottomToolWindowVisible)
    assertFalse(updated.bottomCollapsed)
    assertEquals(DesktopFocusRegion.BottomToolWindow, updated.lastFocusedRegion)
    assertEquals(initial.explorerWidth, updated.explorerWidth)
    assertEquals(initial.actionWidth, updated.actionWidth)
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
              .openBottom(BottomToolWindow.Output)
              .withBottomHeight(300f)
              .withFocus(DesktopFocusRegion.RightToolWindow)
      DesktopLayoutStore(preferences).save(saved)

      assertEquals(saved, DesktopLayoutStore(preferences).load())

      preferences.put("ide-left-tool", "Unknown")
      assertEquals(
          LeftToolWindow.Editor, DesktopLayoutStore(preferences).load().activeLeftToolWindow)
    }
  }

  @Test
  fun storePersistsTheCollapsedSelectedBottomTabWithoutWorkflowState() {
    withPreferences { preferences ->
      val layout =
          DesktopLayoutState()
              .openBottom(BottomToolWindow.Checks)
              .withBottomHeight(360f)
              .withBottomCollapsed(true)
              .withFocus(DesktopFocusRegion.BottomToolWindow)

      DesktopLayoutStore(preferences).save(layout)

      assertEquals(layout, DesktopLayoutStore(preferences).load())
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
    assertEquals(356f, constrained.action)
    assertEquals(MIN_EDITOR_WIDTH, constrained.editor)
    assertEquals(520f, DesktopLayoutState().withExplorerWidth(520f).explorerWidth)
    assertEquals(560f, DesktopLayoutState().withActionWidth(560f).actionWidth)
  }

  @Test
  fun dockedPaneWidthsRestorePreferredDimensionsWhenTheViewportGrows() {
    val preferred =
        dockedPaneWidths(1_440f, preferredExplorerWidth = 256f, preferredActionWidth = 344f)

    assertEquals(256f, preferred.explorer)
    assertEquals(344f, preferred.action)
    assertEquals(736f, preferred.editor)
  }

  @Test
  fun newTerminalPreviewTabDoesNotInvalidateOlderBottomPreferences() {
    withPreferences { preferences ->
      preferences.put("ide-bottom-tool", "Output")
      assertEquals(
          BottomToolWindow.Output, DesktopLayoutStore(preferences).load().activeBottomToolWindow)
      preferences.put("ide-bottom-tool", "Terminal")
      assertEquals(
          BottomToolWindow.Terminal, DesktopLayoutStore(preferences).load().activeBottomToolWindow)
    }
  }

  private fun withPreferences(test: (Preferences) -> Unit) {
    val preferences = Preferences.userRoot().node("/io/miniorca/desktop/tests/${UUID.randomUUID()}")
    try {
      test(preferences)
    } finally {
      preferences.removeNode()
    }
  }
}
