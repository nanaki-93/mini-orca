package io.miniorca.desktop

import com.jediterm.core.util.TermSize
import com.jediterm.terminal.TtyConnector
import java.awt.KeyEventDispatcher
import java.awt.KeyboardFocusManager
import java.awt.event.KeyEvent
import java.beans.PropertyChangeListener
import java.util.concurrent.CompletableFuture
import javax.swing.SwingUtilities
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.swing.Swing

internal data class TerminalTabState(
    val id: Long,
    val title: String,
    val session: TerminalSessionState = TerminalSessionState(),
    val widget: DesktopTerminalWidget? = null,
) {
  val requiresClose: Boolean
    get() =
        session.cleanupPending ||
            session.phase in setOf(TerminalSessionPhase.Starting, TerminalSessionPhase.Running)
}

internal data class TerminalWorkspaceState(
    val projectPath: String? = null,
    val tabs: List<TerminalTabState> = emptyList(),
    val activeTabId: Long? = null,
    val closingAll: Boolean = false,
) {
  val activeTab: TerminalTabState?
    get() = tabs.find { it.id == activeTabId }

  val session: TerminalSessionState
    get() = activeTab?.session ?: TerminalSessionState()

  val widget: DesktopTerminalWidget?
    get() = activeTab?.widget

  val requiresClose: Boolean
    get() = tabs.any { it.requiresClose }

  val cleanupPending: Boolean
    get() = tabs.any { it.session.cleanupPending }

  val canCreate: Boolean
    get() = !closingAll && !cleanupPending

  val errors: List<String>
    get() = tabs.mapNotNull { tab -> tab.session.error?.let { "${tab.title}: $it" } }
}

/** Each shell owns one PTY and emulator reader, independent of the selected tab or view. */
internal class DesktopTerminalWorkspace(
    private val createSession: (String) -> DesktopTerminalSession = { DesktopTerminalSession(it) },
) : AutoCloseable {
  private class Shell(val id: Long, val owner: DesktopTerminalSession) {
    var widget: DesktopTerminalWidget? = null
    var observation: Job? = null
    var renderingError: String? = null
    var closing = false

    fun snapshot() =
        TerminalTabState(
            id,
            "Shell $id",
            owner.state.value.copy(error = owner.state.value.error ?: renderingError),
            widget)
  }

  private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Swing)
  private val mutableState = MutableStateFlow(TerminalWorkspaceState())
  val state = mutableState.asStateFlow()
  private val shells = mutableListOf<Shell>()
  private var nextId = 0L
  private var disposed = false
  private var closeAllCompletion: CompletableFuture<TerminalWorkspaceState>? = null
  var onReturnToEditor: () -> Unit = {}
  var onFocusLeft: () -> Unit = {}

  private val focusManager = KeyboardFocusManager.getCurrentKeyboardFocusManager()

  private fun isTerminalComponent(component: java.awt.Component?): Boolean =
      component != null &&
          shells.any { shell ->
            shell.widget?.let { SwingUtilities.isDescendingFrom(component, it) } == true
          }

  private val focusListener = PropertyChangeListener { event ->
    if (isTerminalComponent(event.oldValue as? java.awt.Component) &&
        !isTerminalComponent(event.newValue as? java.awt.Component))
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

  fun ownsFocus(): Boolean = isTerminalComponent(focusManager.focusOwner)

  fun focus() {
    state.value.widget?.requestFocusInWindow()
  }

  fun activate(projectPath: String) {
    check(SwingUtilities.isEventDispatchThread())
    if (disposed) return
    if (shells.isEmpty() || state.value.projectPath != projectPath) createShell(projectPath)
    else focus()
  }

  fun createShell(projectPath: String) {
    check(SwingUtilities.isEventDispatchThread())
    if (disposed || !state.value.canCreate) return
    if (state.value.projectPath != projectPath) {
      if (state.value.requiresClose) return
      shells.toList().forEach { release(it) }
      mutableState.value = TerminalWorkspaceState(projectPath)
    }
    val shell = Shell(++nextId, createSession(projectPath))
    shells.add(shell)
    mutableState.value = state.value.copy(activeTabId = shell.id)
    shell.owner.start()
    publish()
    shell.observation =
        scope.launch {
          shell.owner.state.collect {
            if (shell !in shells) return@collect
            if (shell.closing && !it.cleanupPending) {
              removeClosed(shell)
            } else {
              if (it.phase == TerminalSessionPhase.Running &&
                  shell.widget == null &&
                  !shell.closing) {
                attachReader(shell)
              }
              publish()
            }
          }
        }
  }

  fun selectShell(id: Long) {
    check(SwingUtilities.isEventDispatchThread())
    if (disposed || shells.none { it.id == id }) return
    mutableState.value = state.value.copy(activeTabId = id)
    focus()
  }

  private fun attachReader(shell: Shell) {
    val connector = shell.owner.connector() ?: return
    try {
      val widget = DesktopTerminalWidget()
      shell.widget = widget
      // EOF belongs to the owner; the emulator must not close a naturally exited session.
      widget.setTtyConnector(
          object : TtyConnector by connector {
            override fun close() = Unit

            override fun resize(size: TermSize) = connector.resize(size)
          })
      widget.start()
    } catch (error: Exception) {
      shell.renderingError =
          "Could not open the terminal view: ${error.message.orEmpty().take(300)}"
      shell.owner.close()
    } catch (error: LinkageError) {
      shell.renderingError =
          "Terminal renderer is unavailable: ${error.message.orEmpty().take(300)}"
      shell.owner.close()
    }
  }

  fun closeSession(id: Long): CompletableFuture<TerminalSessionState> {
    check(SwingUtilities.isEventDispatchThread())
    val shell =
        shells.find { it.id == id }
            ?: return CompletableFuture.completedFuture(
                TerminalSessionState(phase = TerminalSessionPhase.Closed))
    shell.closing = true
    val cleanup = shell.owner.close()
    publish()
    val result = CompletableFuture<TerminalSessionState>()
    cleanup.whenComplete { _, failure ->
      SwingUtilities.invokeLater {
        if (failure != null) result.completeExceptionally(failure)
        else {
          val observed = shell.owner.state.value
          if (!disposed && shell in shells) {
            if (!observed.cleanupPending) removeClosed(shell) else publish()
          }
          result.complete(observed)
        }
      }
    }
    return result
  }

  /** All sessions participate, including hidden tabs and a launch still in progress. */
  fun closeAllSessions(): CompletableFuture<TerminalWorkspaceState> {
    check(SwingUtilities.isEventDispatchThread())
    closeAllCompletion
        ?.takeIf { !it.isDone }
        ?.let {
          return it
        }
    val result = CompletableFuture<TerminalWorkspaceState>()
    closeAllCompletion = result
    mutableState.value = state.value.copy(closingAll = true)
    val closing = shells.toList().map { closeSession(it.id) }
    CompletableFuture.allOf(*closing.toTypedArray()).whenComplete { _, failure ->
      SwingUtilities.invokeLater {
        if (failure != null) result.completeExceptionally(failure)
        else {
          closeAllCompletion = null
          publish()
          result.complete(state.value)
        }
      }
    }
    return result
  }

  private fun removeClosed(shell: Shell) {
    val index = shells.indexOf(shell)
    val selected = state.value.activeTabId == shell.id
    release(shell)
    if (selected)
        mutableState.value =
            state.value.copy(
                activeTabId = shells.getOrNull(index.coerceAtMost(shells.lastIndex))?.id)
    val restoreFocus = selected && !state.value.closingAll
    publish()
    if (restoreFocus) focus()
  }

  private fun release(shell: Shell) {
    shell.observation?.cancel()
    shell.owner.close()
    shell.widget?.close()
    shells.remove(shell)
  }

  private fun publish() {
    mutableState.value =
        state.value.copy(
            tabs = shells.map { it.snapshot() },
            closingAll =
                closeAllCompletion != null || (state.value.closingAll && shells.isNotEmpty()))
  }

  override fun close() {
    check(SwingUtilities.isEventDispatchThread())
    if (disposed) return
    disposed = true
    focusManager.removePropertyChangeListener("focusOwner", focusListener)
    focusManager.removeKeyEventDispatcher(keyDispatcher)
    shells.toList().forEach { release(it) }
    scope.cancel()
  }
}
