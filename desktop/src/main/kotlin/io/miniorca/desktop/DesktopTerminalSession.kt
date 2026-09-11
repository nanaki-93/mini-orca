package io.miniorca.desktop

import com.jediterm.core.util.TermSize
import com.jediterm.terminal.TtyConnector
import com.jediterm.terminal.ui.settings.DefaultSettingsProvider
import com.pty4j.PtyProcess
import com.pty4j.PtyProcessBuilder
import com.pty4j.WinSize
import java.io.IOException
import java.io.InputStream
import java.io.InputStreamReader
import java.io.OutputStream
import java.nio.file.Files
import java.nio.file.Path
import java.util.concurrent.CompletableFuture
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

internal const val TERMINAL_SCROLLBACK_LINES = 5_000
private const val TERMINAL_STOP_MILLIS = 750L

internal class DesktopTerminalSettings(var fontScale: Float = 1f) : DefaultSettingsProvider() {
  override fun getTerminalFontSize(): Float = 13f * fontScale

  override fun getDefaultForeground() = terminalColor(PrimaryText)

  override fun getDefaultBackground() = terminalColor(EditorCanvas)

  override fun getSelectionColor() =
      com.jediterm.terminal.TextStyle(terminalColor(SelectionText), terminalColor(SelectionSurface))

  override fun useInverseSelectionColor(): Boolean = false

  override fun audibleBell(): Boolean = false

  private fun terminalColor(color: androidx.compose.ui.graphics.Color) =
      com.jediterm.terminal.TerminalColor.rgb(
          (color.red * 255).toInt(), (color.green * 255).toInt(), (color.blue * 255).toInt())

  override fun getBufferMaxLinesCount(): Int = TERMINAL_SCROLLBACK_LINES
}

internal enum class TerminalSessionPhase {
  Idle,
  Starting,
  Running,
  Exited,
  Failed,
  Closed
}

internal data class TerminalSessionState(
    val phase: TerminalSessionPhase = TerminalSessionPhase.Idle,
    val directory: String? = null,
    val exitCode: Int? = null,
    val error: String? = null,
    val cleanupPending: Boolean = false,
) {
  val canStart: Boolean
    get() =
        !cleanupPending &&
            phase in
                setOf(
                    TerminalSessionPhase.Idle,
                    TerminalSessionPhase.Exited,
                    TerminalSessionPhase.Failed)
}

internal data class TerminalSize(val columns: Int = 80, val rows: Int = 24) {
  init {
    require(columns in 1..1_000 && rows in 1..1_000) {
      "Terminal size must be between 1 and 1000 cells."
    }
  }
}

internal data class TerminalLaunchSpec(
    val directory: Path,
    val command: List<String>,
    val environment: Map<String, String>,
    val size: TerminalSize,
)

internal fun terminalLaunchSpec(
    projectPath: String,
    shell: String,
    environment: Map<String, String>,
    size: TerminalSize,
    osName: String = System.getProperty("os.name"),
    architecture: String = System.getProperty("os.arch"),
): TerminalLaunchSpec {
  require(osName == "Mac OS X" && architecture in setOf("aarch64", "arm64")) {
    "The local terminal currently supports macOS arm64 only."
  }
  val requested = Path.of(projectPath)
  require(requested.isAbsolute) { "Select an existing absolute local project directory." }
  val directory =
      try {
        requested.toRealPath()
      } catch (error: IOException) {
        throw IOException(
            "The project directory is unavailable locally. Open its local checkout and retry: $requested",
            error)
      }
  require(
      Files.isDirectory(directory) &&
          Files.isReadable(directory) &&
          Files.isExecutable(directory)) {
        "The project directory is unavailable locally. Open its local checkout and retry."
      }
  val executable = Path.of(shell)
  require(
      executable.isAbsolute && Files.isRegularFile(executable) && Files.isExecutable(executable)) {
        "The configured shell is unavailable. Set SHELL to an executable absolute path and retry."
      }
  require(executable.fileName.toString() in setOf("zsh", "bash", "sh", "fish")) {
    "Choose a supported shell: zsh, bash, sh or fish."
  }
  // The directory is a process attribute, never part of a shell command. Keep the user's
  // environment
  // in memory only; terminal input and output are never stored or sent to the daemon.
  val childEnvironment =
      environment.toMutableMap().apply {
        this["TERM"] = "xterm-256color"
        this["COLORTERM"] = "truecolor"
        this["LC_CTYPE"] = "UTF-8"
      }
  return TerminalLaunchSpec(
      directory, listOf(executable.toString(), "-l", "-i"), childEnvironment, size)
}

internal interface TerminalProcess {
  val input: InputStream
  val output: OutputStream
  val alive: Boolean

  fun resize(size: TerminalSize)

  fun waitFor(): Int

  /** Release native resources and terminate owned processes within the bounded grace periods. */
  fun terminate(): String?
}

internal fun interface TerminalProcessFactory {
  fun start(spec: TerminalLaunchSpec): TerminalProcess
}

internal object Pty4jTerminalProcessFactory : TerminalProcessFactory {
  override fun start(spec: TerminalLaunchSpec): TerminalProcess =
      NativeTerminalProcess(
          PtyProcessBuilder(spec.command.toTypedArray())
              .setDirectory(spec.directory.toString())
              .setEnvironment(spec.environment)
              .setInitialColumns(spec.size.columns)
              .setInitialRows(spec.size.rows)
              .setRedirectErrorStream(true)
              .setUnixOpenTtyToPreserveOutputAfterTermination(true)
              .start())
}

private class NativeTerminalProcess(private val process: PtyProcess) : TerminalProcess {
  override val input: InputStream
    get() = process.inputStream

  override val output: OutputStream
    get() = process.outputStream

  override val alive: Boolean
    get() = process.isAlive

  override fun resize(size: TerminalSize) = process.setWinSize(WinSize(size.columns, size.rows))

  override fun waitFor(): Int = process.waitFor()

  override fun terminate(): String? {
    val errors = mutableListOf<String>()
    fun attempt(label: String, action: () -> Unit) {
      try {
        action()
      } catch (error: Exception) {
        errors += "$label: ${terminalFailureMessage(error)}"
      }
    }
    // Capture descendants before ending the shell, which can reparent a foreground/background job.
    val descendants = mutableListOf<ProcessHandle>()
    attempt("Inspect terminal jobs") {
      if (process.isAlive)
          ProcessHandle.of(process.pid()).ifPresent { handle ->
            handle.descendants().use { descendants += it.toList() }
          }
    }
    attempt("Stop terminal jobs") { descendants.filter { it.isAlive }.forEach { it.destroy() } }
    attempt("Stop shell") { if (process.isAlive) process.destroy() }
    attempt("Wait for shell") { process.waitFor(TERMINAL_STOP_MILLIS, TimeUnit.MILLISECONDS) }
    attempt("Force terminal jobs") {
      descendants.filter { it.isAlive }.forEach { it.destroyForcibly() }
    }
    attempt("Force shell") {
      if (process.isAlive) process.destroyForcibly()
      if (!process.waitFor(TERMINAL_STOP_MILLIS, TimeUnit.MILLISECONDS))
          errors += "Shell did not exit within the cleanup deadline."
    }
    // Close the raw streams, not InputStreamReader: its monitor may be held by a blocking read.
    listOf({ process.inputStream }, { process.outputStream }, { process.errorStream }).forEach {
        stream ->
      attempt("Close terminal stream") { stream().close() }
    }
    return errors.takeIf { it.isNotEmpty() }?.joinToString("; ")
  }
}

/**
 * One explicitly started local session. It owns no Compose state and performs no work on creation.
 */
internal class DesktopTerminalSession(
    private val projectPath: String,
    private val shell: String = System.getenv("SHELL") ?: "/bin/zsh",
    private val environment: Map<String, String> = System.getenv(),
    private val factory: TerminalProcessFactory = Pty4jTerminalProcessFactory,
    private val workers: ExecutorService =
        Executors.newCachedThreadPool { task ->
          Thread(task, "mini-orca-terminal").apply { isDaemon = true }
        },
) {
  private val lock = Any()
  private val mutableState = MutableStateFlow(TerminalSessionState())
  val state: StateFlow<TerminalSessionState> = mutableState.asStateFlow()
  private var generation = 0L
  private var connection: TerminalConnection? = null
  private var startup = CompletableFuture.completedFuture(Unit)
  private val closed = CompletableFuture<TerminalSessionState>()

  fun connector(): TtyConnector? =
      synchronized(lock) {
        connection.takeIf { mutableState.value.phase == TerminalSessionPhase.Running }
      }

  fun start(size: TerminalSize = TerminalSize()): Boolean =
      synchronized(lock) {
        if (!mutableState.value.canStart || connection != null) return false
        val attempt = ++generation
        val started = CompletableFuture<Unit>()
        startup = started
        mutableState.value = TerminalSessionState(TerminalSessionPhase.Starting)
        workers.execute { launch(attempt, size, started) }
        true
      }

  private fun launch(attempt: Long, size: TerminalSize, started: CompletableFuture<Unit>) {
    try {
      val spec = terminalLaunchSpec(projectPath, shell, environment, size)
      val process = factory.start(spec)
      val opened =
          try {
            TerminalConnection(process, shell, this)
          } catch (error: Exception) {
            val cleanupError = process.terminate()
            throw IOException(
                "Could not attach terminal streams: ${terminalFailureMessage(error)}${cleanupError?.let { "; $it" }.orEmpty()}",
                error)
          }
      val accepted =
          synchronized(lock) {
            if (generation != attempt || mutableState.value.phase == TerminalSessionPhase.Closed)
                false
            else {
              connection = opened
              mutableState.value =
                  TerminalSessionState(TerminalSessionPhase.Running, spec.directory.toString())
              true
            }
          }
      if (!accepted) {
        val error = opened.dispose()
        synchronized(lock) {
          if (opened.hasLiveProcess) connection = opened
          mutableState.value =
              mutableState.value.copy(
                  error =
                      error
                          ?: if (opened.hasLiveProcess) "Shell is still running after cleanup."
                          else mutableState.value.error,
                  cleanupPending = opened.hasLiveProcess)
        }
        started.complete(Unit)
        if (opened.hasLiveProcess) finish(opened, opened.waitFor())
        return
      }
      started.complete(Unit)
      val exit = opened.waitFor()
      finish(opened, exit)
    } catch (error: Exception) {
      launchFailed(attempt, terminalFailureMessage(error))
    } catch (error: LinkageError) {
      launchFailed(
          attempt, "Terminal native library could not load: ${terminalFailureMessage(error)}")
    } finally {
      started.complete(Unit)
    }
  }

  private fun launchFailed(attempt: Long, message: String) {
    val opened =
        synchronized(lock) {
          if (generation != attempt || mutableState.value.phase == TerminalSessionPhase.Closed)
              return
          mutableState.value =
              mutableState.value.copy(
                  phase = TerminalSessionPhase.Failed,
                  error = message,
                  cleanupPending = connection != null)
          connection
        }
    if (opened != null) finish(opened, null)
  }

  private fun finish(opened: TerminalConnection, exit: Int?) {
    val cleanupError = opened.dispose()
    synchronized(lock) {
      if (connection !== opened) return
      val stillAlive = opened.hasLiveProcess
      if (!stillAlive) connection = null
      val prior = mutableState.value
      val error =
          listOfNotNull(prior.error, cleanupError).takeIf { it.isNotEmpty() }?.joinToString("; ")
      mutableState.value =
          prior.copy(
              phase =
                  when {
                    prior.phase == TerminalSessionPhase.Closed -> TerminalSessionPhase.Closed
                    error != null || stillAlive -> TerminalSessionPhase.Failed
                    else -> TerminalSessionPhase.Exited
                  },
              exitCode = exit,
              error =
                  error
                      ?: if (stillAlive)
                          "Shell is still running after cleanup; a new session cannot start."
                      else null,
              cleanupPending = stillAlive)
    }
  }

  internal fun ioFailed(opened: TerminalConnection, error: IOException): Unit =
      synchronized(lock) {
        if (connection !== opened || mutableState.value.phase == TerminalSessionPhase.Closed) return
        mutableState.value =
            mutableState.value.copy(
                phase = TerminalSessionPhase.Failed,
                error = terminalFailureMessage(error),
                cleanupPending = true)
        workers.execute { finish(opened, null) }
      }

  /** Nonblocking; callers can await the returned completion with their own UI/shutdown deadline. */
  fun close(): CompletableFuture<TerminalSessionState> =
      synchronized(lock) {
        if (mutableState.value.phase == TerminalSessionPhase.Closed) return closed
        ++generation
        val opened = connection
        mutableState.value =
            mutableState.value.copy(phase = TerminalSessionPhase.Closed, cleanupPending = true)
        val cleanup = CompletableFuture.supplyAsync({ opened?.dispose() }, workers)
        CompletableFuture.allOf(startup, cleanup).whenComplete { _, error ->
          synchronized(lock) {
            val stillAlive = connection?.hasLiveProcess == true
            if (!stillAlive) connection = null
            mutableState.value =
                mutableState.value.copy(
                    cleanupPending = stillAlive,
                    error =
                        (if (error == null) cleanup.getNow(null) else terminalFailureMessage(error))
                            ?: mutableState.value.error
                            ?: if (stillAlive) "Shell is still running after cleanup." else null)
            closed.complete(mutableState.value)
          }
        }
        workers.shutdown()
        closed
      }
}

internal class TerminalConnection(
    private val process: TerminalProcess,
    private val shell: String,
    private val owner: DesktopTerminalSession,
) : TtyConnector {
  private val reader = InputStreamReader(process.input, Charsets.UTF_8)
  private val disposing = AtomicBoolean(false)
  private val disposed = CompletableFuture<String?>()
  private val inputLock = Any()

  private fun <T> io(action: () -> T): T {
    try {
      return action()
    } catch (error: IOException) {
      if (!disposing.get()) owner.ioFailed(this, error)
      throw error
    }
  }

  override fun read(buffer: CharArray, offset: Int, length: Int): Int = io {
    reader.read(buffer, offset, length)
  }

  override fun ready(): Boolean = io { reader.ready() }

  override fun write(bytes: ByteArray) = io {
    synchronized(inputLock) {
      check(isConnected()) { "The terminal session is no longer running." }
      process.output.write(bytes)
      process.output.flush()
    }
  }

  override fun write(text: String) = write(text.toByteArray(Charsets.UTF_8))

  internal val hasLiveProcess: Boolean
    get() = process.alive

  override fun isConnected(): Boolean = !disposing.get() && process.alive

  override fun getName(): String = Path.of(shell).fileName.toString()

  override fun resize(size: TermSize) {
    if (isConnected()) io { process.resize(TerminalSize(size.columns, size.rows)) }
  }

  override fun waitFor(): Int = process.waitFor()

  override fun close() {
    owner.close()
  }

  internal fun dispose(): String? {
    if (disposing.compareAndSet(false, true)) {
      try {
        disposed.complete(process.terminate())
      } catch (error: Exception) {
        disposed.complete("Terminal cleanup failed: ${terminalFailureMessage(error)}")
      } catch (error: LinkageError) {
        disposed.complete("Terminal native cleanup failed: ${terminalFailureMessage(error)}")
      }
    }
    return disposed.join()
  }
}

private fun terminalFailureMessage(error: Throwable): String =
    (error.message?.takeIf { it.isNotBlank() } ?: error.javaClass.simpleName).take(500)
