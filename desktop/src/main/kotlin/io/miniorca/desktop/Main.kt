package io.miniorca.desktop

import androidx.compose.ui.window.Window
import androidx.compose.ui.window.application

fun main() = application {
    Window(onCloseRequest = ::exitApplication, title = "Mini-Orca", resizable = true) {
        MiniOrcaTheme {
            MiniOrcaApp()
        }
    }
}
