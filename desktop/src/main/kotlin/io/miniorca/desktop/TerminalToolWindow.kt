package io.miniorca.desktop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.ui.Modifier
import androidx.compose.ui.awt.SwingPanel
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.jediterm.core.util.TermSize
import com.jediterm.terminal.TtyConnector
import com.jediterm.terminal.model.StyleState
import com.jediterm.terminal.model.TerminalTextBuffer
import com.jediterm.terminal.ui.JediTermWidget
import com.jediterm.terminal.ui.TerminalPanel
import com.jediterm.terminal.ui.settings.SettingsProvider
import java.awt.BorderLayout
import java.awt.Dimension
import java.awt.KeyEventDispatcher
import java.awt.KeyboardFocusManager
import java.awt.event.KeyEvent
import java.beans.PropertyChangeListener
import java.util.concurrent.CompletableFuture
import javax.swing.JButton
import javax.swing.JPanel
import javax.swing.JScrollBar
import javax.swing.SwingUtilities
import javax.swing.plaf.basic.BasicScrollBarUI
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.swing.Swing

internal data class TerminalWorkspaceState(
    val projectPath: String? = null,
    val session: TerminalSessionState = TerminalSessionState(),
    val widget: DesktopTerminalWidget? = null,
) {
  val requiresClose: Boolean
    get() =
        session.cleanupPending ||
            session.phase in setOf(TerminalSessionPhase.Starting, TerminalSessionPhase.Running)

  val canOpen: Boolean
    get() = !requiresClose
}

/** Owns both the PTY and its single reader, independently of a docked or overlay view. */
internal class DesktopTerminalWorkspace(
    private val createSession: (String) -> DesktopTerminalSession = { DesktopTerminalSession(it) },
) : AutoCloseable {
  private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Swing)
  private val mutableState = MutableStateFlow(TerminalWorkspaceState())
  val state = mutableState.asStateFlow()
  private var session: DesktopTerminalSession? = null
  private var observation: Job? = null
  private var disposed = false
  private var renderingError: String? = null
  var onReturnToEditor: () -> Unit = {}
  var onFocusLeft: () -> Unit = {}

  private val focusManager = KeyboardFocusManager.getCurrentKeyboardFocusManager()
  private val focusListener = PropertyChangeListener { event ->
    val widget = state.value.widget ?: return@PropertyChangeListener
    val previous = event.oldValue as? java.awt.Component
    val next = event.newValue as? java.awt.Component
    if (previous != null &&
        SwingUtilities.isDescendingFrom(previous, widget) &&
        (next == null || !SwingUtilities.isDescendingFrom(next, widget)))
        onFocusLeft()
  }
  private val keyDispatcher = KeyEventDispatcher { event ->
    if (ownsFocus() &&
        terminalReturnShortcut(event.keyCode, event.isControlDown, event.isShiftDown) &&
        event.id == KeyEvent.KEY_PRESSED) {
      onReturnToEditor()
      event.consume()
      true
    } else false
  }

  init {
    focusManager.addPropertyChangeListener("focusOwner", focusListener)
    focusManager.addKeyEventDispatcher(keyDispatcher)
  }

  fun ownsFocus(): Boolean =
      state.value.widget?.let { widget ->
        focusManager.focusOwner?.let { SwingUtilities.isDescendingFrom(it, widget) } == true
      } == true

  fun focus() {
    state.value.widget?.requestFocusInWindow()
  }

  fun activate(projectPath: String) {
    check(SwingUtilities.isEventDispatchThread())
    if (disposed) return
    if (state.value.requiresClose) {
      if (state.value.projectPath == projectPath) focus()
      return
    }
    observation?.cancel()
    session?.close()
    state.value.widget?.close()
    val next = createSession(projectPath)
    session = next
    renderingError = null
    mutableState.value = TerminalWorkspaceState(projectPath)
    observation =
        scope.launch {
          next.state.collect { observed ->
            if (session !== next) return@collect
            var widget = state.value.widget
            if (observed.phase == TerminalSessionPhase.Running && widget == null) {
              val connector = next.connector() ?: return@collect
              try {
                widget = DesktopTerminalWidget()
                // EOF belongs to the owner; the emulator also closes its connector on natural exit.
                widget.setTtyConnector(
                    object : TtyConnector by connector {
                      override fun close() = Unit

                      // Kotlin interface delegation does not forward Java default methods.
                      override fun resize(size: TermSize) = connector.resize(size)
                    })
                widget.start()
              } catch (error: Exception) {
                renderingError =
                    "Could not open the terminal view: ${error.message.orEmpty().take(300)}"
                next.close()
              } catch (error: LinkageError) {
                renderingError =
                    "Terminal renderer is unavailable: ${error.message.orEmpty().take(300)}"
                next.close()
              }
            }
            mutableState.value =
                TerminalWorkspaceState(
                    projectPath, observed.copy(error = observed.error ?: renderingError), widget)
          }
        }
    next.start()
  }

  /** The result includes incomplete cleanup; callers must not switch projects in that state. */
  fun closeSession(): CompletableFuture<TerminalSessionState> {
    check(SwingUtilities.isEventDispatchThread())
    val owner =
        session
            ?: return CompletableFuture.completedFuture(
                TerminalSessionState(phase = TerminalSessionPhase.Closed))
    if (disposed) return owner.close()
    val result = CompletableFuture<TerminalSessionState>()
    owner.close().whenComplete { _, failure ->
      scope.launch {
        if (failure != null) result.completeExceptionally(failure)
        else {
          val observed = owner.state.value
          if (session === owner) {
            state.value.widget?.close()
            mutableState.value = state.value.copy(session = observed, widget = null)
          }
          result.complete(observed)
        }
      }
    }
    return result
  }

  override fun close() {
    check(SwingUtilities.isEventDispatchThread())
    if (disposed) return
    disposed = true
    focusManager.removePropertyChangeListener("focusOwner", focusListener)
    focusManager.removeKeyEventDispatcher(keyDispatcher)
    session?.close()
    state.value.widget?.close()
    scope.cancel()
  }
}

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
    projectPath: String,
    onReindex: () -> Unit,
    modifier: Modifier = Modifier,
) {
  val state by terminal.state.collectAsState()
  val fontScale = LocalDensity.current.fontScale
  Column(modifier) {
    FlowRow(
        modifier = Modifier.padding(horizontal = 8.dp, vertical = 4.dp),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
      Text(terminalSummary(state.session), color = PrimaryText, fontSize = 12.sp)
      ChromeButton(onClick = { terminal.activate(projectPath) }) {
        Text(if (state.requiresClose) "Focus terminal" else "Open shell", fontSize = 11.sp)
      }
      ChromeButton(
          onClick = { terminal.closeSession() },
          enabled = state.requiresClose && !state.session.cleanupPending) {
            Text("Close shell", fontSize = 11.sp)
          }
      ChromeButton(onClick = { terminal.onReturnToEditor() }) {
        Text("Back to editor · Ctrl+Shift+F12", fontSize = 11.sp)
      }
      ChromeButton(onClick = onReindex) { Text("Reindex project", fontSize = 11.sp) }
    }
    Text(
        state.session.directory ?: projectPath,
        color = SecondaryText,
        fontSize = 11.sp,
        maxLines = 1,
        modifier = Modifier.padding(horizontal = 8.dp))
    state.session.error?.let {
      Text(it, color = Error, fontSize = 12.sp, modifier = Modifier.padding(8.dp))
    }
    val widget = state.widget
    if (widget == null) {
      Text(
          "Local interactive shell. Reindex after adding, deleting or renaming files.",
          color = SecondaryText,
          fontSize = 12.sp,
          modifier = Modifier.padding(8.dp))
    } else {
      // Detaching a Swing host must never stop its reader or create a second emulator.
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
      title = { Text("Close the project shell?") },
      content = {
        Column {
          Text(
              "The shell belongs to ${state.projectPath}. Close it and its child processes before switching projects.")
          state.session.error?.let { Text(it, color = Error) }
          if (state.session.cleanupPending) Text("Waiting for the shell to stop…", color = Warning)
        }
      },
      actions = {
        MiniOrcaButton(onClick = onCancel, tone = ActionTone.Neutral) { Text("Cancel switch") }
        MiniOrcaButton(
            onClick = {
              terminal.closeSession().thenAccept { if (!it.cleanupPending) onSwitch(path) }
            },
            enabled = !state.session.cleanupPending,
            tone = ActionTone.Destructive) {
              Text("Close shell and switch")
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
