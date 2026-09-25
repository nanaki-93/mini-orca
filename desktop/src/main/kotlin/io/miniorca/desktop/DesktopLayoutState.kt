package io.miniorca.desktop

import java.util.prefs.Preferences

internal enum class LeftToolWindow {
  Summary,
  Analysis,
  Performance,
  Problems,
  Security,
  Editor,
}

// Presentation order is independent of the persisted LeftToolWindow enum names and shortcuts.
internal val workspaceRailOrder =
    listOf(
        LeftToolWindow.Summary,
        LeftToolWindow.Analysis,
        LeftToolWindow.Problems,
        LeftToolWindow.Performance,
        LeftToolWindow.Security,
        LeftToolWindow.Editor)

internal enum class RightToolWindow {
  Context,
  Assistant,
  Review,
}

internal enum class EditorSurface {
  Source,
  Review,
}

internal enum class DesktopFocusRegion {
  Toolbar,
  LeftToolWindow,
  Editor,
  RightToolWindow,
  BottomToolWindow,
  StatusBar,
}

internal fun leftToolWindowLabel(toolWindow: LeftToolWindow): String =
    when (toolWindow) {
      LeftToolWindow.Summary -> "Summary"
      LeftToolWindow.Analysis -> "Analysis"
      LeftToolWindow.Performance -> "Performance"
      LeftToolWindow.Problems -> "Bugs"
      LeftToolWindow.Security -> "Security"
      LeftToolWindow.Editor -> "Editor"
    }

internal fun rightToolWindowLabel(toolWindow: RightToolWindow): String =
    when (toolWindow) {
      RightToolWindow.Context -> "Context"
      RightToolWindow.Assistant -> "Assistant"
      RightToolWindow.Review -> "Review"
    }

internal fun leftToolWindowForWorkspace(workspace: Workspace): LeftToolWindow =
    when (workspace) {
      Workspace.Summary -> LeftToolWindow.Summary
      Workspace.Analysis -> LeftToolWindow.Analysis
      Workspace.Performance -> LeftToolWindow.Performance
      Workspace.Bugs -> LeftToolWindow.Problems
      Workspace.Security -> LeftToolWindow.Security
      Workspace.Editor -> LeftToolWindow.Editor
    }

internal fun workspaceForLeftToolWindow(toolWindow: LeftToolWindow): Workspace =
    when (toolWindow) {
      LeftToolWindow.Summary -> Workspace.Summary
      LeftToolWindow.Analysis -> Workspace.Analysis
      LeftToolWindow.Performance -> Workspace.Performance
      LeftToolWindow.Problems -> Workspace.Bugs
      LeftToolWindow.Security -> Workspace.Security
      LeftToolWindow.Editor -> Workspace.Editor
    }

/** Presentation-only preferences for the known IDE shell regions. */
internal data class DesktopLayoutState(
    val activeLeftToolWindow: LeftToolWindow = LeftToolWindow.Editor,
    val activeRightToolWindow: RightToolWindow = RightToolWindow.Context,
    val editorSurface: EditorSurface = EditorSurface.Source,
    val leftToolWindowVisible: Boolean = true,
    val rightToolWindowVisible: Boolean = true,
    val bottomCollapsed: Boolean = true,
    val explorerWidth: Float = DEFAULT_EXPLORER_WIDTH,
    val actionWidth: Float = DEFAULT_ACTION_WIDTH,
    val bottomHeight: Float = DEFAULT_BOTTOM_HEIGHT,
    val lastFocusedRegion: DesktopFocusRegion = DesktopFocusRegion.Editor,
) {
  fun openLeft(toolWindow: LeftToolWindow) =
      copy(activeLeftToolWindow = toolWindow, leftToolWindowVisible = true)

  fun openRight(toolWindow: RightToolWindow) =
      copy(activeRightToolWindow = toolWindow, rightToolWindowVisible = true)

  fun openTerminal() = withBottomCollapsed(false)

  fun withBottomCollapsed(collapsed: Boolean) = copy(bottomCollapsed = collapsed)

  fun withEditorSurface(surface: EditorSurface) = copy(editorSurface = surface)

  fun withExplorerWidth(value: Float) = copy(explorerWidth = clampExplorerWidth(value))

  fun withActionWidth(value: Float) = copy(actionWidth = clampActionWidth(value))

  fun withBottomHeight(value: Float) = copy(bottomHeight = clampBottomHeight(value))

  fun withFocus(region: DesktopFocusRegion) = copy(lastFocusedRegion = region)

  fun normalized() =
      copy(
          explorerWidth = clampExplorerWidth(explorerWidth),
          actionWidth = clampActionWidth(actionWidth),
          bottomHeight = clampBottomHeight(bottomHeight),
      )

  companion object {
    const val DEFAULT_EXPLORER_WIDTH = 220f
    const val DEFAULT_ACTION_WIDTH = 300f
    const val DEFAULT_BOTTOM_HEIGHT = 220f

    const val MIN_EXPLORER_WIDTH = 180f
    const val MAX_EXPLORER_WIDTH = 520f
    const val MIN_ACTION_WIDTH = 280f
    const val MAX_ACTION_WIDTH = 560f
    const val MIN_BOTTOM_HEIGHT = 140f
    const val MAX_BOTTOM_HEIGHT = 520f

    fun clampExplorerWidth(value: Float) =
        (if (value.isFinite()) value else DEFAULT_EXPLORER_WIDTH).coerceIn(
            MIN_EXPLORER_WIDTH, MAX_EXPLORER_WIDTH)

    fun clampActionWidth(value: Float) =
        (if (value.isFinite()) value else DEFAULT_ACTION_WIDTH).coerceIn(
            MIN_ACTION_WIDTH, MAX_ACTION_WIDTH)

    fun clampBottomHeight(value: Float) =
        (if (value.isFinite()) value else DEFAULT_BOTTOM_HEIGHT).coerceIn(
            MIN_BOTTOM_HEIGHT, MAX_BOTTOM_HEIGHT)
  }
}

// Native control padding leaves too little room for Performance in the 78dp mock rail.
internal const val TOOL_WINDOW_BAR_WIDTH = 96f
internal const val WORKSPACE_FRAME_INSET = 8f
internal const val RESIZE_DIVIDER_WIDTH = 8f

// Wide Editor reserves this much readable canvas at 100% text. Pane minima are the existing
// preference minima, scaled with text; the rail scales as ToolWindowBar does. WorkspaceFrame also
// consumes an end inset and a rail-to-pane gap (both WORKSPACE_FRAME_INSET), plus one divider per
// visible side pane. The input is WorkspaceFrame's allocation, not the outer window width.
internal const val MIN_EDITOR_CANVAS_WIDTH = 360f

internal enum class DesktopLayoutMode {
  Wide,
  Compact,
}

/** Effective Editor geometry; never saved in DesktopLayoutStore. Hidden panes have zero width. */
internal data class ResolvedDesktopLayout(
    val mode: DesktopLayoutMode,
    val explorerWidth: Float,
    val canvasWidth: Float,
    val actionWidth: Float,
)

/** Resolve the Editor against its allocated workspace width, leaving preferred sizes unchanged. */
internal fun resolveDesktopLayout(
    preferred: DesktopLayoutState,
    workspaceWidthDp: Float,
    fontScale: Float,
): ResolvedDesktopLayout {
  val width = workspaceWidthDp.takeIf { it.isFinite() && it > 0f }?.toDouble() ?: 0.0
  val scale = fontScale.takeIf { it.isFinite() && it > 0f }?.coerceAtLeast(1f)?.toDouble() ?: 1.0
  val railAndInsets = TOOL_WINDOW_BAR_WIDTH * scale + 2 * WORKSPACE_FRAME_INSET
  val paneWidth = (width - railAndInsets).coerceAtLeast(0.0)
  val leftVisible = preferred.leftToolWindowVisible
  val rightVisible = preferred.rightToolWindowVisible
  val leftMin = if (leftVisible) DesktopLayoutState.MIN_EXPLORER_WIDTH * scale else 0.0
  val rightMin = if (rightVisible) DesktopLayoutState.MIN_ACTION_WIDTH * scale else 0.0
  val dividers = (listOf(leftVisible, rightVisible).count { it } * RESIZE_DIVIDER_WIDTH).toDouble()
  val canvasMin = MIN_EDITOR_CANVAS_WIDTH * scale
  val leftDesired =
      if (leftVisible)
          maxOf(leftMin, DesktopLayoutState.clampExplorerWidth(preferred.explorerWidth).toDouble())
      else 0.0
  val rightDesired =
      if (rightVisible)
          maxOf(rightMin, DesktopLayoutState.clampActionWidth(preferred.actionWidth).toDouble())
      else 0.0

  // At the breakpoint all visible minima and dividers fit. Below it, panes stack without
  // consuming canvas width; even invalid or tiny allocations yield finite, nonnegative sizes.
  if (paneWidth < canvasMin + leftMin + rightMin + dividers) {
    return ResolvedDesktopLayout(
        DesktopLayoutMode.Compact,
        minOf(leftDesired, paneWidth).toFloat(),
        paneWidth.toFloat(),
        minOf(rightDesired, paneWidth).toFloat())
  }

  val spare = (paneWidth - canvasMin - leftMin - rightMin - dividers).coerceAtLeast(0.0)
  val leftExtra = leftDesired - leftMin
  val rightExtra = rightDesired - rightMin
  val desiredExtra = leftExtra + rightExtra
  val fraction = if (desiredExtra > 0.0) minOf(1.0, spare / desiredExtra) else 1.0
  val explorer = leftMin + leftExtra * fraction
  val action = rightMin + rightExtra * fraction
  return ResolvedDesktopLayout(
      DesktopLayoutMode.Wide,
      explorer.toFloat(),
      (paneWidth - dividers - explorer - action).toFloat(),
      action.toFloat())
}

/** Persists visual preferences only; it never stores workflow or authorization state. */
internal class DesktopLayoutStore(
    private val preferences: Preferences =
        Preferences.userNodeForPackage(DesktopLayoutStore::class.java)
) {
  fun load(): DesktopLayoutState =
      DesktopLayoutState(
              activeLeftToolWindow = enumPreference(LEFT_TOOL_KEY, LeftToolWindow.Editor),
              activeRightToolWindow = enumPreference(RIGHT_TOOL_KEY, RightToolWindow.Context),
              editorSurface = enumPreference(EDITOR_SURFACE_KEY, EditorSurface.Source),
              leftToolWindowVisible = preferences.getBoolean(LEFT_VISIBLE_KEY, true),
              rightToolWindowVisible = preferences.getBoolean(RIGHT_VISIBLE_KEY, true),
              // Restoring a layout never opens or starts a shell, including legacy bottom tabs.
              bottomCollapsed = true,
              explorerWidth =
                  preferences.getFloat(
                      EXPLORER_WIDTH_KEY, DesktopLayoutState.DEFAULT_EXPLORER_WIDTH),
              actionWidth =
                  preferences.getFloat(ACTION_WIDTH_KEY, DesktopLayoutState.DEFAULT_ACTION_WIDTH),
              bottomHeight =
                  preferences.getFloat(BOTTOM_HEIGHT_KEY, DesktopLayoutState.DEFAULT_BOTTOM_HEIGHT),
              lastFocusedRegion = enumPreference(FOCUS_REGION_KEY, DesktopFocusRegion.Editor),
          )
          .normalized()

  fun save(layout: DesktopLayoutState) {
    val normalized = layout.normalized()
    preferences.put(LEFT_TOOL_KEY, normalized.activeLeftToolWindow.name)
    preferences.put(RIGHT_TOOL_KEY, normalized.activeRightToolWindow.name)
    preferences.remove(BOTTOM_TOOL_KEY)
    preferences.remove(BOTTOM_VISIBLE_KEY)
    preferences.remove(BOTTOM_COLLAPSED_KEY)
    preferences.put(EDITOR_SURFACE_KEY, normalized.editorSurface.name)
    preferences.putBoolean(LEFT_VISIBLE_KEY, normalized.leftToolWindowVisible)
    preferences.putBoolean(RIGHT_VISIBLE_KEY, normalized.rightToolWindowVisible)
    preferences.putFloat(EXPLORER_WIDTH_KEY, normalized.explorerWidth)
    preferences.putFloat(ACTION_WIDTH_KEY, normalized.actionWidth)
    preferences.putFloat(BOTTOM_HEIGHT_KEY, normalized.bottomHeight)
    preferences.put(FOCUS_REGION_KEY, normalized.lastFocusedRegion.name)
  }

  private inline fun <reified T : Enum<T>> enumPreference(key: String, default: T): T =
      preferences.get(key, default.name).let { stored ->
        enumValues<T>().firstOrNull { it.name == stored } ?: default
      }

  private companion object {
    // Keep these stable keys so pane-width preferences survive shell upgrades.
    const val EXPLORER_WIDTH_KEY = "explorer-width"
    const val ACTION_WIDTH_KEY = "action-width"
    const val BOTTOM_HEIGHT_KEY = "ide-bottom-height"
    const val BOTTOM_COLLAPSED_KEY = "ide-bottom-collapsed"
    const val LEFT_TOOL_KEY = "ide-left-tool"
    const val RIGHT_TOOL_KEY = "ide-right-tool"
    const val BOTTOM_TOOL_KEY = "ide-bottom-tool"
    const val EDITOR_SURFACE_KEY = "ide-editor-surface"
    const val LEFT_VISIBLE_KEY = "ide-left-visible"
    const val RIGHT_VISIBLE_KEY = "ide-right-visible"
    const val BOTTOM_VISIBLE_KEY = "ide-bottom-visible"
    const val FOCUS_REGION_KEY = "ide-focus-region"
  }
}
