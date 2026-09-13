package io.miniorca.desktop

import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.window.Window
import androidx.compose.ui.window.application

fun main() = miniOrcaApplication()

internal fun miniOrcaApplication(
    createTerminal: () -> DesktopTerminalWorkspace = { DesktopTerminalWorkspace() },
    content: @Composable (DesktopTerminalWorkspace) -> Unit = { MiniOrcaApp(terminal = it) },
) {
  // AWT reads the native macOS appearance before the first window is created.
  if (System.getProperty("os.name").startsWith("Mac", ignoreCase = true)) {
    System.setProperty("apple.awt.application.appearance", "NSAppearanceNameDarkAqua")
  }
  application {
    val terminal = remember { createTerminal() }
    var exitRequested by remember { mutableStateOf(false) }
    val terminalState by terminal.state.collectAsState()
    DisposableEffect(terminal) { onDispose { terminal.close() } }
    LaunchedEffect(exitRequested) {
      if (exitRequested)
          terminal.closeAllSessions().thenAccept {
            if (exitRequested && !it.cleanupPending) exitApplication()
          }
    }
    LaunchedEffect(exitRequested, terminalState.cleanupPending) {
      if (exitRequested && terminalState.tabs.isEmpty() && !terminalState.cleanupPending)
          exitApplication()
    }
    Window(onCloseRequest = { exitRequested = true }, title = "Mini-Orca", resizable = true) {
      MiniOrcaTheme {
        content(terminal)
        if (exitRequested && terminalState.cleanupPending) {
          IdeDialog(
              onDismissRequest = { exitRequested = false },
              title = { Text("Closing the terminal") },
              content = {
                Text(
                    terminalState.errors.takeIf { it.isNotEmpty() }?.joinToString("\n")
                        ?: "Waiting for the shells and their child processes to stop…")
              },
              actions = {
                MiniOrcaButton(onClick = { exitRequested = false }, tone = ActionTone.Neutral) {
                  Text("Keep app open")
                }
              },
          )
        }
      }
    }
  }
}
