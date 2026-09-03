package io.miniorca.desktop

import androidx.compose.ui.input.key.Key

internal enum class TabGroupKey {
  Previous,
  Next,
  Activate,
}

internal data class TabGroupInteraction<T>(
    val focused: T,
    val activate: T? = null,
)

internal fun tabGroupKey(key: Key): TabGroupKey? =
    when (key) {
      Key.DirectionUp,
      Key.DirectionLeft -> TabGroupKey.Previous
      Key.DirectionDown,
      Key.DirectionRight -> TabGroupKey.Next
      Key.Enter,
      Key.Spacebar -> TabGroupKey.Activate
      else -> null
    }

internal fun <T> tabGroupInteraction(
    entries: List<T>,
    focused: T,
    key: TabGroupKey?,
): TabGroupInteraction<T>? {
  if (entries.isEmpty()) return null
  val command = key ?: return null
  val index = entries.indexOf(focused).takeIf { it >= 0 } ?: 0
  val target =
      when (command) {
        TabGroupKey.Previous -> entries[(index - 1 + entries.size) % entries.size]
        TabGroupKey.Next -> entries[(index + 1) % entries.size]
        TabGroupKey.Activate -> entries[index]
      }
  return TabGroupInteraction(target, target.takeIf { command == TabGroupKey.Activate })
}

internal enum class ResponsiveShellRegion {
  Docked,
  Drawer,
  Overlay,
}

internal data class ResponsiveShellPresentation(
    val left: ResponsiveShellRegion,
    val right: ResponsiveShellRegion,
    val bottom: ResponsiveShellRegion,
)

internal fun responsiveShellPresentation(widthDp: Float): ResponsiveShellPresentation =
    if (useNarrowLayout(widthDp)) {
      ResponsiveShellPresentation(
          left = ResponsiveShellRegion.Drawer,
          right = ResponsiveShellRegion.Drawer,
          bottom = ResponsiveShellRegion.Overlay,
      )
    } else {
      ResponsiveShellPresentation(
          left = ResponsiveShellRegion.Docked,
          right = ResponsiveShellRegion.Docked,
          bottom = ResponsiveShellRegion.Docked,
      )
    }

internal fun closesEditorDrawerOnWorkspaceChange(
    current: Workspace,
    next: Workspace,
): Boolean = editorChromeVisible(current) && !editorChromeVisible(next)

internal enum class TransientSurface {
  Context,
  Palette,
  StatusDetails,
  BottomTools,
  Drawer,
}

internal fun topmostTransientSurface(
    contextVisible: Boolean,
    paletteVisible: Boolean,
    statusDetailsVisible: Boolean,
    bottomToolsVisible: Boolean,
    drawerVisible: Boolean,
): TransientSurface? =
    when {
      contextVisible -> TransientSurface.Context
      paletteVisible -> TransientSurface.Palette
      statusDetailsVisible -> TransientSurface.StatusDetails
      bottomToolsVisible -> TransientSurface.BottomTools
      drawerVisible -> TransientSurface.Drawer
      else -> null
    }
