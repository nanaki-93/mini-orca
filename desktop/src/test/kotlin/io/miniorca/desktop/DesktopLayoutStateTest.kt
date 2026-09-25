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
  fun everyDimensionNormalizesDirectUpdatesAndRawStateIdempotently() {
    val dimensions =
        listOf(
            Triple(
                DesktopLayoutState.DEFAULT_EXPLORER_WIDTH,
                DesktopLayoutState.DEFAULT_ACTION_WIDTH,
                DesktopLayoutState.DEFAULT_BOTTOM_HEIGHT),
            Triple(
                DesktopLayoutState.MIN_EXPLORER_WIDTH,
                DesktopLayoutState.MIN_ACTION_WIDTH,
                DesktopLayoutState.MIN_BOTTOM_HEIGHT),
            Triple(
                DesktopLayoutState.MAX_EXPLORER_WIDTH,
                DesktopLayoutState.MAX_ACTION_WIDTH,
                DesktopLayoutState.MAX_BOTTOM_HEIGHT))
    val inputs =
        listOf(
            Triple(Float.NaN, Float.NaN, Float.NaN) to dimensions[0],
            Triple(Float.POSITIVE_INFINITY, Float.POSITIVE_INFINITY, Float.POSITIVE_INFINITY) to
                dimensions[0],
            Triple(Float.NEGATIVE_INFINITY, Float.NEGATIVE_INFINITY, Float.NEGATIVE_INFINITY) to
                dimensions[0],
            Triple(-1f, -1f, -1f) to dimensions[1],
            Triple(10_000f, 10_000f, 10_000f) to dimensions[2],
            Triple(320f, 440f, 340f) to Triple(320f, 440f, 340f))

    inputs.forEach { (values, expected) ->
      val (explorer, action, bottom) = values
      val updated =
          DesktopLayoutState()
              .withExplorerWidth(explorer)
              .withActionWidth(action)
              .withBottomHeight(bottom)
      val raw =
          DesktopLayoutState(explorerWidth = explorer, actionWidth = action, bottomHeight = bottom)
      assertEquals(
          expected, Triple(updated.explorerWidth, updated.actionWidth, updated.bottomHeight))
      assertEquals(updated, raw.normalized())
      assertEquals(updated, raw.normalized().normalized())
      assertTrue(updated.explorerWidth.isFinite())
      assertTrue(updated.actionWidth.isFinite())
      assertTrue(updated.bottomHeight.isFinite())
    }
  }

  @Test
  fun legacyDimensionsRoundTripWithoutChangingNavigationVisibilityOrFocus() {
    val cases =
        listOf(
            Triple("320", "440", "340") to Triple(320f, 440f, 340f),
            Triple("-1", "10000", "-1") to
                Triple(
                    DesktopLayoutState.MIN_EXPLORER_WIDTH,
                    DesktopLayoutState.MAX_ACTION_WIDTH,
                    DesktopLayoutState.MIN_BOTTOM_HEIGHT),
            Triple("10000", "-1", "10000") to
                Triple(
                    DesktopLayoutState.MAX_EXPLORER_WIDTH,
                    DesktopLayoutState.MIN_ACTION_WIDTH,
                    DesktopLayoutState.MAX_BOTTOM_HEIGHT),
            Triple("NaN", "Infinity", "-Infinity") to
                Triple(
                    DesktopLayoutState.DEFAULT_EXPLORER_WIDTH,
                    DesktopLayoutState.DEFAULT_ACTION_WIDTH,
                    DesktopLayoutState.DEFAULT_BOTTOM_HEIGHT),
            Triple("Infinity", "-Infinity", "NaN") to
                Triple(
                    DesktopLayoutState.DEFAULT_EXPLORER_WIDTH,
                    DesktopLayoutState.DEFAULT_ACTION_WIDTH,
                    DesktopLayoutState.DEFAULT_BOTTOM_HEIGHT),
            Triple("bad", null, "bad") to
                Triple(
                    DesktopLayoutState.DEFAULT_EXPLORER_WIDTH,
                    DesktopLayoutState.DEFAULT_ACTION_WIDTH,
                    DesktopLayoutState.DEFAULT_BOTTOM_HEIGHT))
    cases.forEach { (stored, expected) ->
      withPreferences { preferences ->
        listOf("explorer-width", "action-width", "ide-bottom-height")
            .zip(listOf(stored.first, stored.second, stored.third))
            .forEach { (key, value) -> if (value != null) preferences.put(key, value) }
        preferences.put("ide-left-tool", "Problems")
        preferences.put("ide-right-tool", "Review")
        preferences.put("ide-editor-surface", "Review")
        preferences.put("ide-focus-region", "BottomToolWindow")
        preferences.putBoolean("ide-left-visible", false)
        preferences.putBoolean("ide-right-visible", false)
        preferences.put("ide-bottom-tool", "Checks")
        preferences.putBoolean("ide-bottom-visible", true)
        preferences.putBoolean("ide-bottom-collapsed", false)
        val store = DesktopLayoutStore(preferences)
        val loaded = store.load()
        assertEquals(
            expected, Triple(loaded.explorerWidth, loaded.actionWidth, loaded.bottomHeight))
        assertEquals(LeftToolWindow.Problems, loaded.activeLeftToolWindow)
        assertEquals(RightToolWindow.Review, loaded.activeRightToolWindow)
        assertEquals(EditorSurface.Review, loaded.editorSurface)
        assertEquals(DesktopFocusRegion.BottomToolWindow, loaded.lastFocusedRegion)
        assertFalse(loaded.leftToolWindowVisible)
        assertFalse(loaded.rightToolWindowVisible)
        assertTrue(loaded.bottomCollapsed)

        store.save(loaded.openTerminal())
        assertEquals(loaded, store.load())
        store.save(store.load())
        assertEquals(loaded, store.load())
        assertEquals(expected.first, preferences.getFloat("explorer-width", -1f))
        assertEquals(expected.second, preferences.getFloat("action-width", -1f))
        assertEquals(expected.third, preferences.getFloat("ide-bottom-height", -1f))
        listOf("ide-bottom-tool", "ide-bottom-visible", "ide-bottom-collapsed").forEach {
          assertEquals(null, preferences.get(it, null))
        }
      }
    }
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
      preferences.put("ide-right-tool", "Retired")
      preferences.put("ide-editor-surface", "Retired")
      preferences.put("ide-focus-region", "Unknown")

      val recovered = DesktopLayoutStore(preferences).load()

      assertEquals(DesktopLayoutState.DEFAULT_EXPLORER_WIDTH, recovered.explorerWidth)
      assertEquals(DesktopLayoutState.MAX_ACTION_WIDTH, recovered.actionWidth)
      assertEquals(DesktopLayoutState.DEFAULT_BOTTOM_HEIGHT, recovered.bottomHeight)
      assertEquals(LeftToolWindow.Editor, recovered.activeLeftToolWindow)
      assertEquals(RightToolWindow.Context, recovered.activeRightToolWindow)
      assertEquals(EditorSurface.Source, recovered.editorSurface)
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

  @Test
  fun wideBoundaryAccountsForVisiblePanesRailInsetsDividersAndTextScale() {
    for (scale in listOf(1f, 1.25f, 1.5f)) {
      for (left in listOf(false, true)) {
        for (right in listOf(false, true)) {
          val preferred =
              DesktopLayoutState(
                  leftToolWindowVisible = left,
                  rightToolWindowVisible = right,
                  explorerWidth = DesktopLayoutState.MAX_EXPLORER_WIDTH,
                  actionWidth = DesktopLayoutState.MAX_ACTION_WIDTH)
          val boundary =
              TOOL_WINDOW_BAR_WIDTH * scale +
                  2 * WORKSPACE_FRAME_INSET +
                  MIN_EDITOR_CANVAS_WIDTH * scale +
                  (if (left) DesktopLayoutState.MIN_EXPLORER_WIDTH * scale + RESIZE_DIVIDER_WIDTH
                  else 0f) +
                  (if (right) DesktopLayoutState.MIN_ACTION_WIDTH * scale + RESIZE_DIVIDER_WIDTH
                  else 0f)
          val below = resolveDesktopLayout(preferred, Math.nextDown(boundary), scale)
          val at = resolveDesktopLayout(preferred, boundary, scale)
          val above = resolveDesktopLayout(preferred, Math.nextUp(boundary), scale)

          assertEquals(
              DesktopLayoutMode.Compact, below.mode, "below $boundary at $scale, $left/$right")
          assertEquals(DesktopLayoutMode.Wide, at.mode, "at $boundary at $scale, $left/$right")
          assertEquals(
              DesktopLayoutMode.Wide, above.mode, "above $boundary at $scale, $left/$right")
          assertTrue(at.canvasWidth >= MIN_EDITOR_CANVAS_WIDTH * scale)
          assertEquals(
              if (left) DesktopLayoutState.MIN_EXPLORER_WIDTH * scale else 0f, at.explorerWidth)
          assertEquals(
              if (right) DesktopLayoutState.MIN_ACTION_WIDTH * scale else 0f, at.actionWidth)
          if (!left) assertEquals(0f, below.explorerWidth)
          if (!right) assertEquals(0f, below.actionWidth)
          assertEquals(
              boundary - TOOL_WINDOW_BAR_WIDTH * scale - 2 * WORKSPACE_FRAME_INSET,
              below.canvasWidth,
              0.001f)
        }
      }
    }
  }

  @Test
  fun wideLayoutPreservesFittingPreferencesAndConstrainedLayoutDoesNotPersistThem() {
    withPreferences { preferences ->
      val store = DesktopLayoutStore(preferences)
      val preferred =
          DesktopLayoutState(
              explorerWidth = DesktopLayoutState.MAX_EXPLORER_WIDTH,
              actionWidth = DesktopLayoutState.MAX_ACTION_WIDTH)
      store.save(preferred)
      val restored = store.load()
      val narrow = resolveDesktopLayout(restored, 900f, 1f)
      val constrained = resolveDesktopLayout(restored, 950f, 1f)
      val large = resolveDesktopLayout(restored, 1800f, 1f)

      assertEquals(DesktopLayoutMode.Compact, narrow.mode)
      assertEquals(DesktopLayoutMode.Wide, constrained.mode)
      assertTrue(constrained.canvasWidth >= MIN_EDITOR_CANVAS_WIDTH)
      assertTrue(constrained.explorerWidth < restored.explorerWidth)
      assertTrue(constrained.actionWidth < restored.actionWidth)
      assertEquals(DesktopLayoutMode.Wide, large.mode)
      assertEquals(restored.explorerWidth, large.explorerWidth)
      assertEquals(restored.actionWidth, large.actionWidth)
      assertEquals(restored, store.load())
      assertEquals(large, resolveDesktopLayout(restored, 1800f, 1f))
    }

    for (scale in listOf(1f, 1.25f, 1.5f)) {
      val preferred =
          DesktopLayoutState(
              explorerWidth = DesktopLayoutState.MIN_EXPLORER_WIDTH,
              actionWidth = DesktopLayoutState.MIN_ACTION_WIDTH)
      val wide = resolveDesktopLayout(preferred, 1800f, scale)
      assertEquals(DesktopLayoutMode.Wide, wide.mode)
      assertEquals(DesktopLayoutState.MIN_EXPLORER_WIDTH * scale, wide.explorerWidth)
      assertEquals(DesktopLayoutState.MIN_ACTION_WIDTH * scale, wide.actionWidth)
      assertEquals(DesktopLayoutState.MIN_EXPLORER_WIDTH, preferred.explorerWidth)
      assertEquals(DesktopLayoutState.MIN_ACTION_WIDTH, preferred.actionWidth)

      val maximum =
          DesktopLayoutState(
              explorerWidth = DesktopLayoutState.MAX_EXPLORER_WIDTH,
              actionWidth = DesktopLayoutState.MAX_ACTION_WIDTH)
      val fitting = resolveDesktopLayout(maximum, 1800f, scale)
      assertEquals(DesktopLayoutMode.Wide, fitting.mode)
      assertEquals(maximum.explorerWidth, fitting.explorerWidth)
      assertEquals(maximum.actionWidth, fitting.actionWidth)
      assertTrue(fitting.canvasWidth >= MIN_EDITOR_CANVAS_WIDTH * scale)
    }
  }

  @Test
  fun invalidAndInsufficientWorkspaceInputsHaveFiniteNonnegativeAllocations() {
    val preferred =
        DesktopLayoutState(explorerWidth = Float.NaN, actionWidth = Float.POSITIVE_INFINITY)
    for (width in
        listOf(Float.NaN, Float.NEGATIVE_INFINITY, Float.POSITIVE_INFINITY, -20f, 0f, 1f, 120f)) {
      for (scale in listOf(Float.NaN, Float.NEGATIVE_INFINITY, 0f, -1f, 1.5f, Float.MAX_VALUE)) {
        val resolved = resolveDesktopLayout(preferred, width, scale)
        assertEquals(DesktopLayoutMode.Compact, resolved.mode)
        listOf(resolved.explorerWidth, resolved.canvasWidth, resolved.actionWidth).forEach {
          assertTrue(it.isFinite() && it >= 0f && it <= resolved.canvasWidth)
        }
      }
    }
    assertTrue(preferred.explorerWidth.isNaN())
    assertEquals(Float.POSITIVE_INFINITY, preferred.actionWidth)
  }

  private fun withPreferences(test: (InMemoryPreferences) -> Unit) = test(InMemoryPreferences())
}
