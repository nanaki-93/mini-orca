package io.miniorca.desktop

import java.util.prefs.Preferences

internal enum class LeftToolWindow {
  Project,
  Summary,
  Analysis,
  Problems,
  Editor,
}

internal enum class RightToolWindow {
  Context,
  Assistant,
  Review,
}

internal enum class BottomToolWindow {
  Problems,
  Checks,
  Output,
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
      LeftToolWindow.Project -> "Project"
      LeftToolWindow.Summary -> "Summary"
      LeftToolWindow.Analysis -> "Analysis"
      LeftToolWindow.Problems -> "Problems"
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
      Workspace.Bugs -> LeftToolWindow.Problems
      Workspace.Editor -> LeftToolWindow.Editor
    }

internal fun workspaceForLeftToolWindow(toolWindow: LeftToolWindow): Workspace =
    when (toolWindow) {
      LeftToolWindow.Project -> Workspace.Editor
      LeftToolWindow.Summary -> Workspace.Summary
      LeftToolWindow.Analysis -> Workspace.Analysis
      LeftToolWindow.Problems -> Workspace.Bugs
      LeftToolWindow.Editor -> Workspace.Editor
    }

/** Presentation-only preferences for the known IDE shell regions. */
internal data class DesktopLayoutState(
    val activeLeftToolWindow: LeftToolWindow = LeftToolWindow.Project,
    val activeRightToolWindow: RightToolWindow = RightToolWindow.Context,
    val activeBottomToolWindow: BottomToolWindow = BottomToolWindow.Problems,
    val editorSurface: EditorSurface = EditorSurface.Source,
    val leftToolWindowVisible: Boolean = true,
    val rightToolWindowVisible: Boolean = true,
    val bottomToolWindowVisible: Boolean = true,
    val bottomCollapsed: Boolean = true,
    val explorerWidth: Float = DEFAULT_EXPLORER_WIDTH,
    val actionWidth: Float = DEFAULT_ACTION_WIDTH,
    val bottomHeight: Float = DEFAULT_BOTTOM_HEIGHT,
    val lastFocusedRegion: DesktopFocusRegion = DesktopFocusRegion.Editor,
) {
  fun openLeft(toolWindow: LeftToolWindow) =
      copy(activeLeftToolWindow = toolWindow, leftToolWindowVisible = true)

  fun closeLeft() = copy(leftToolWindowVisible = false)

  fun openRight(toolWindow: RightToolWindow) =
      copy(activeRightToolWindow = toolWindow, rightToolWindowVisible = true)

  fun closeRight() = copy(rightToolWindowVisible = false)

  fun openBottom(toolWindow: BottomToolWindow) =
      copy(
          activeBottomToolWindow = toolWindow,
          bottomToolWindowVisible = true,
          bottomCollapsed = false)

  fun closeBottom() = copy(bottomToolWindowVisible = true, bottomCollapsed = true)

  fun withBottomCollapsed(collapsed: Boolean) =
      copy(bottomToolWindowVisible = true, bottomCollapsed = collapsed)

  fun withEditorSurface(surface: EditorSurface) = copy(editorSurface = surface)

  fun withExplorerWidth(value: Float) = copy(explorerWidth = clampExplorerWidth(value))

  fun withActionWidth(value: Float) = copy(actionWidth = clampActionWidth(value))

  fun withBottomHeight(value: Float) = copy(bottomHeight = clampBottomHeight(value))

  fun withFocus(region: DesktopFocusRegion) = copy(lastFocusedRegion = region)

  fun normalized() =
      copy(
          // A collapsed pane remains visible as its text-only summary and explicit reopen control.
          bottomToolWindowVisible = true,
          explorerWidth = clampExplorerWidth(explorerWidth),
          actionWidth = clampActionWidth(actionWidth),
          bottomHeight = clampBottomHeight(bottomHeight),
      )

  companion object {
    const val DEFAULT_EXPLORER_WIDTH = 270f
    const val DEFAULT_ACTION_WIDTH = 390f
    const val DEFAULT_BOTTOM_HEIGHT = 240f

    const val MIN_EXPLORER_WIDTH = 180f
    const val MAX_EXPLORER_WIDTH = 520f
    const val MIN_ACTION_WIDTH = 280f
    const val MAX_ACTION_WIDTH = 560f
    const val MIN_BOTTOM_HEIGHT = 140f
    const val MAX_BOTTOM_HEIGHT = 520f

    fun clampExplorerWidth(value: Float) = value.coerceIn(MIN_EXPLORER_WIDTH, MAX_EXPLORER_WIDTH)

    fun clampActionWidth(value: Float) = value.coerceIn(MIN_ACTION_WIDTH, MAX_ACTION_WIDTH)

    fun clampBottomHeight(value: Float) = value.coerceIn(MIN_BOTTOM_HEIGHT, MAX_BOTTOM_HEIGHT)
  }
}

/** Persists visual preferences only; it never stores workflow or authorization state. */
internal class DesktopLayoutStore(
    private val preferences: Preferences =
        Preferences.userNodeForPackage(DesktopLayoutStore::class.java)
) {
  fun load(): DesktopLayoutState =
      DesktopLayoutState(
              activeLeftToolWindow = enumPreference(LEFT_TOOL_KEY, LeftToolWindow.Project),
              activeRightToolWindow = enumPreference(RIGHT_TOOL_KEY, RightToolWindow.Context),
              activeBottomToolWindow = enumPreference(BOTTOM_TOOL_KEY, BottomToolWindow.Problems),
              editorSurface = enumPreference(EDITOR_SURFACE_KEY, EditorSurface.Source),
              leftToolWindowVisible = preferences.getBoolean(LEFT_VISIBLE_KEY, true),
              rightToolWindowVisible = preferences.getBoolean(RIGHT_VISIBLE_KEY, true),
              bottomToolWindowVisible = preferences.getBoolean(BOTTOM_VISIBLE_KEY, false),
              bottomCollapsed = preferences.getBoolean(BOTTOM_COLLAPSED_KEY, true),
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
    preferences.put(BOTTOM_TOOL_KEY, normalized.activeBottomToolWindow.name)
    preferences.put(EDITOR_SURFACE_KEY, normalized.editorSurface.name)
    preferences.putBoolean(LEFT_VISIBLE_KEY, normalized.leftToolWindowVisible)
    preferences.putBoolean(RIGHT_VISIBLE_KEY, normalized.rightToolWindowVisible)
    preferences.putBoolean(BOTTOM_VISIBLE_KEY, normalized.bottomToolWindowVisible)
    preferences.putBoolean(BOTTOM_COLLAPSED_KEY, normalized.bottomCollapsed)
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
