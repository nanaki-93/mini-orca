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

    assertEquals(LeftToolWindow.Project, layout.activeLeftToolWindow)
    assertEquals(RightToolWindow.Context, layout.activeRightToolWindow)
    assertEquals(BottomToolWindow.Problems, layout.activeBottomToolWindow)
    assertEquals(EditorSurface.Source, layout.editorSurface)
    assertEquals(270f, layout.explorerWidth)
    assertEquals(390f, layout.actionWidth)
    assertEquals(240f, layout.bottomHeight)
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
  fun storeReadsLegacyWidthsAndUsesDefaultsForMissingIdePreferences() {
    withPreferences { preferences ->
      preferences.putFloat("explorer-width", 320f)
      preferences.putFloat("action-width", 440f)

      val layout = DesktopLayoutStore(preferences).load()

      assertEquals(320f, layout.explorerWidth)
      assertEquals(440f, layout.actionWidth)
      assertEquals(DesktopLayoutState.DEFAULT_BOTTOM_HEIGHT, layout.bottomHeight)
      assertEquals(LeftToolWindow.Project, layout.activeLeftToolWindow)
      assertTrue(layout.bottomCollapsed)
    }
  }

  @Test
  fun storeNormalizesCorruptValuesAndRoundTripsKnownLayoutPreferences() {
    withPreferences { preferences ->
      preferences.put("explorer-width", "invalid")
      preferences.putFloat("action-width", 9_999f)
      preferences.put("ide-bottom-height", "invalid")
      preferences.put("ide-left-tool", "Unknown")
      preferences.put("ide-focus-region", "Unknown")

      val recovered = DesktopLayoutStore(preferences).load()

      assertEquals(DesktopLayoutState.DEFAULT_EXPLORER_WIDTH, recovered.explorerWidth)
      assertEquals(DesktopLayoutState.MAX_ACTION_WIDTH, recovered.actionWidth)
      assertEquals(DesktopLayoutState.DEFAULT_BOTTOM_HEIGHT, recovered.bottomHeight)
      assertEquals(LeftToolWindow.Project, recovered.activeLeftToolWindow)
      assertEquals(DesktopFocusRegion.Editor, recovered.lastFocusedRegion)

      val saved =
          recovered
              .openLeft(LeftToolWindow.Problems)
              .openRight(RightToolWindow.Review)
              .openBottom(BottomToolWindow.Output)
              .withBottomHeight(300f)
              .withFocus(DesktopFocusRegion.RightToolWindow)
      DesktopLayoutStore(preferences).save(saved)

      assertEquals(saved, DesktopLayoutStore(preferences).load())
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

  private fun withPreferences(test: (Preferences) -> Unit) {
    val preferences = Preferences.userRoot().node("/io/miniorca/desktop/tests/${UUID.randomUUID()}")
    try {
      test(preferences)
    } finally {
      preferences.removeNode()
    }
  }
}
