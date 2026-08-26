package io.miniorca.desktop

import androidx.compose.material.MaterialTheme
import androidx.compose.material.darkColors
import androidx.compose.ui.window.Window
import androidx.compose.ui.window.application

fun main() = application {
    Window(onCloseRequest = ::exitApplication, title = "Mini-Orca", resizable = true) {
        MaterialTheme(colors = darkColors(primary = Accent, background = AppBackground, surface = Panel, onBackground = PrimaryText, onSurface = PrimaryText)) {
            MiniOrcaApp()
        }
    }
}
