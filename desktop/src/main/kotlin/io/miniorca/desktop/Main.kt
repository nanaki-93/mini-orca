package io.miniorca.desktop

import androidx.compose.ui.window.Window
import androidx.compose.ui.window.application

fun main() {
  // AWT reads the native macOS appearance before the first window is created.
  if (System.getProperty("os.name").startsWith("Mac", ignoreCase = true)) {
    System.setProperty("apple.awt.application.appearance", "NSAppearanceNameDarkAqua")
  }
  application {
    Window(onCloseRequest = ::exitApplication, title = "Mini-Orca", resizable = true) {
      MiniOrcaTheme { MiniOrcaApp() }
    }
  }
}
