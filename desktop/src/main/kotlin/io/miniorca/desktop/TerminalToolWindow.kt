package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
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
import androidx.compose.ui.platform.LocalWindowInfo
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
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
  TerminalSessionContent(state, modifier) { canvasModifier ->
    val widget = state.widget
    if (widget != null) {
      // Detaching a Swing host must never stop its reader or create a second emulator.
      key(widget) {
        val host = remember(widget) { JPanel(BorderLayout()) }
        DisposableEffect(host) { onDispose { host.remove(widget) } }
        SwingPanel(
            background = EditorCanvas,
            modifier = canvasModifier,
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
internal fun TerminalSessionContent(
    state: TerminalWorkspaceState,
    modifier: Modifier = Modifier,
    canvas: @Composable (Modifier) -> Unit,
) {
  BoxWithConstraints(modifier.background(EditorCanvas)) {
    val feedbackHeight = maxHeight * 0.35f
    Column(Modifier.fillMaxSize()) {
      Column(
          Modifier.fillMaxWidth()
              .heightIn(max = feedbackHeight)
              .verticalScroll(rememberScrollState())
              .testTag("terminal-feedback")
              .padding(8.dp)) {
            SelectionContainer {
              Column {
                if (state.tabs.isEmpty()) {
                  Text(
                      "New shell opens a local shell in the current project.",
                      color = SecondaryText,
                      style = IdeTypography.workspaceMetadata)
                } else {
                  state.projectPath?.let {
                    Text("Opened in: $it", color = SecondaryText, style = IdeTypography.resultCode)
                  }
                  Text(
                      "Ctrl+Shift+F12 returns to Editor",
                      color = SecondaryText,
                      style = IdeTypography.workspaceMetadata)
                  if (state.session.phase != TerminalSessionPhase.Running ||
                      state.session.cleanupPending) {
                    Text(
                        terminalSummary(state.session),
                        color = Warning,
                        style = IdeTypography.workspaceMetadata)
                  }
                  state.session.error?.let {
                    Text(it, color = Error, style = IdeTypography.workspaceMetadata)
                  }
                }
              }
            }
          }
      Box(Modifier.weight(1f).fillMaxWidth().testTag("terminal-canvas")) {
        canvas(Modifier.fillMaxSize())
      }
    }
  }
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
  val windowFocused = LocalWindowInfo.current.isWindowFocused
  val terminalState by terminal.state.collectAsState()
  LaunchedEffect(
      windowFocused,
      state.workspace,
      state.project?.projectId,
      terminalState.projectPath,
      layout.activeRightToolWindow,
      layout.editorSurface) {
        if (windowFocused &&
            state.workspace == Workspace.Editor &&
            state.project != null &&
            terminalState.projectPath == state.project?.path) {
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
