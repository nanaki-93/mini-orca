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

internal enum class TransientSurface {
  Context,
  Palette,
  StatusDetails,
}

internal fun topmostTransientSurface(
    contextVisible: Boolean,
    paletteVisible: Boolean,
    statusDetailsVisible: Boolean,
): TransientSurface? =
    when {
      contextVisible -> TransientSurface.Context
      paletteVisible -> TransientSurface.Palette
      statusDetailsVisible -> TransientSurface.StatusDetails
      else -> null
    }

/** Shell input bypasses app shortcuts; this reserved chord deliberately returns to the editor. */
internal fun terminalReturnShortcut(keyCode: Int, control: Boolean, shift: Boolean): Boolean =
    control && shift && keyCode == java.awt.event.KeyEvent.VK_F12

internal fun appShortcutAllowed(terminalFocused: Boolean): Boolean = !terminalFocused
