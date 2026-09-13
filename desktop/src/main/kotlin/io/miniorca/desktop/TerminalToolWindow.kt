package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.ui.Modifier
import androidx.compose.ui.awt.SwingPanel
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.jediterm.terminal.model.StyleState
import com.jediterm.terminal.model.TerminalTextBuffer
import com.jediterm.terminal.ui.JediTermWidget
import com.jediterm.terminal.ui.TerminalPanel
import com.jediterm.terminal.ui.settings.SettingsProvider
import java.awt.BorderLayout
import java.awt.Dimension
import javax.swing.JButton
import javax.swing.JPanel
import javax.swing.JScrollBar
import javax.swing.SwingUtilities
import javax.swing.plaf.basic.BasicScrollBarUI

internal fun terminalSummary(state: TerminalSessionState): String =
    when {
      state.cleanupPending -> "Stopping shell…"
      state.error != null -> "Terminal needs attention"
      else ->
          when (state.phase) {
            TerminalSessionPhase.Idle -> "Ready to open a local shell"
            TerminalSessionPhase.Starting -> "Starting shell…"
            TerminalSessionPhase.Running -> "Shell running"
            TerminalSessionPhase.Exited -> "Shell exited (${state.exitCode ?: "unknown"})"
            TerminalSessionPhase.Failed -> "Shell failed"
            TerminalSessionPhase.Closed -> "Shell closed"
          }
    }

@Composable
internal fun TerminalToolWindow(
    terminal: DesktopTerminalWorkspace,
    modifier: Modifier = Modifier,
) {
  val state by terminal.state.collectAsState()
  val fontScale = LocalDensity.current.fontScale
  Column(modifier) {
    state.session.error?.let {
      Text(it, color = Error, fontSize = 12.sp, modifier = Modifier.padding(8.dp))
    }
    if (state.activeTab != null && state.session.phase != TerminalSessionPhase.Running) {
      Text(
          terminalSummary(state.session),
          color = SecondaryText,
          fontSize = 12.sp,
          modifier = Modifier.padding(8.dp))
    }
    if (state.tabs.isEmpty()) {
      Text(
          "Use + to open a shell.",
          color = SecondaryText,
          fontSize = 12.sp,
          modifier = Modifier.padding(8.dp))
    }
    val widget = state.widget
    if (widget != null) {
      // Detaching a Swing host must never stop its reader or create a second emulator.
      key(widget) {
        val host = remember(widget) { JPanel(BorderLayout()) }
        DisposableEffect(host) { onDispose { host.remove(widget) } }
        SwingPanel(
            background = EditorCanvas,
            modifier = Modifier.fillMaxSize(),
            factory = {
              host.apply {
                add(widget, BorderLayout.CENTER)
                SwingUtilities.invokeLater { widget.requestFocusInWindow() }
              }
            },
            update = { panel ->
              widget.updateFontScale(fontScale)
              if (widget.parent !== panel) {
                panel.removeAll()
                panel.add(widget, BorderLayout.CENTER)
                panel.revalidate()
              }
            },
        )
      }
    }
  }
}

@Composable
internal fun TerminalProjectSwitchDialog(
    pendingProjectPath: String?,
    terminal: DesktopTerminalWorkspace,
    onCancel: () -> Unit,
    onSwitch: (String) -> Unit,
) {
  val path = pendingProjectPath ?: return
  val state by terminal.state.collectAsState()
  IdeDialog(
      onDismissRequest = onCancel,
      title = { Text("Close the project shells?") },
      content = {
        Column {
          Text(
              "The shells belong to ${state.projectPath}. Close them and their child processes before switching projects.")
          state.errors.forEach { Text(it, color = Error) }
          if (state.cleanupPending) Text("Waiting for the shells to stop…", color = Warning)
        }
      },
      actions = {
        MiniOrcaButton(onClick = onCancel, tone = ActionTone.Neutral) { Text("Cancel switch") }
        MiniOrcaButton(
            onClick = {
              terminal.closeAllSessions().thenAccept { if (!it.cleanupPending) onSwitch(path) }
            },
            enabled = !state.cleanupPending,
            tone = ActionTone.Destructive) {
              Text("Close shells and switch")
            }
      },
  )
}

@Composable
internal fun TerminalFocusReturnEffect(terminal: DesktopTerminalWorkspace?, onReturn: () -> Unit) {
  val currentReturn by rememberUpdatedState(onReturn)
  DisposableEffect(terminal) {
    terminal?.onReturnToEditor = { currentReturn() }
    onDispose { terminal?.onReturnToEditor = {} }
  }
}

@Composable
internal fun TerminalSourceRefreshEffect(
    state: DesktopState,
    layout: DesktopLayoutState,
    terminal: DesktopTerminalWorkspace,
    presenter: DesktopWorkflowPresenter,
) {
  LaunchedEffect(state.workspace, layout.activeRightToolWindow, layout.editorSurface) {
    if (state.workspace == Workspace.Editor &&
        terminal.state.value.projectPath == state.project?.path) {
      presenter.refreshSelectedFile()
    }
  }
}

/** A library terminal with the same source palette, text scale and flat scrollbar as the IDE. */
internal class DesktopTerminalWidget(
    private val settings: DesktopTerminalSettings = DesktopTerminalSettings(),
) : JediTermWidget(80, 24, settings) {
  override fun createTerminalPanel(
      settings: SettingsProvider,
      style: StyleState,
      buffer: TerminalTextBuffer
  ): TerminalPanel = ScaledTerminalPanel(settings, style, buffer)

  fun updateFontScale(scale: Float) {
    if (settings.fontScale == scale) return
    settings.fontScale = scale
    (terminalPanel as? ScaledTerminalPanel)?.refreshFont()
  }

  override fun createScrollBar(): JScrollBar =
      super.createScrollBar().apply {
        preferredSize = Dimension(12, 0)
        setUI(
            object : BasicScrollBarUI() {
              override fun configureScrollBarColors() {
                thumbColor = java.awt.Color(SecondaryText.toArgb(), true)
                trackColor = java.awt.Color(EditorCanvas.toArgb(), true)
              }

              override fun createDecreaseButton(orientation: Int) = emptyButton()

              override fun createIncreaseButton(orientation: Int) = emptyButton()

              private fun emptyButton() =
                  JButton().apply {
                    preferredSize = Dimension(0, 0)
                    minimumSize = Dimension(0, 0)
                    maximumSize = Dimension(0, 0)
                    isFocusable = false
                  }
            })
      }
}

private class ScaledTerminalPanel(
    settings: SettingsProvider,
    style: StyleState,
    buffer: TerminalTextBuffer
) : TerminalPanel(settings, buffer, style) {
  fun refreshFont() = reinitFontAndResize()
}
