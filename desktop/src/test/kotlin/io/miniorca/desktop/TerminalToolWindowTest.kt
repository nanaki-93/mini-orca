package io.miniorca.desktop

import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.ui.Modifier
import java.io.ByteArrayOutputStream
import java.io.PipedInputStream
import java.io.PipedOutputStream
import java.nio.file.Files
import java.util.concurrent.CompletableFuture
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import javax.swing.JPanel
import javax.swing.SwingUtilities
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertSame
import kotlin.test.assertTrue

class TerminalToolWindowTest {
  @Test
  fun terminalControlsRemainVisibleAtNarrowWidthAndLargeTextWithoutStartingAShell() {
    val workspace = edt { DesktopTerminalWorkspace() }
    try {
      listOf(800 to 1f, 800 to 1.5f, 1200 to 1f).forEach { (width, scale) ->
        ComposeVisualFixture(width, 240, fontScale = scale) {
              TerminalToolWindow(
                  workspace,
                  "/tmp/project with a long but readable name",
                  {},
                  Modifier.fillMaxSize())
            }
            .use { fixture ->
              fixture.render("terminal-idle-$width-$scale")
              assertTrue(fixture.hasText("Open shell"))
              assertTrue(fixture.hasText("Reindex project"))
              assertTrue(fixture.hasText("Back to editor · Ctrl+Shift+F12"))
              assertEquals(TerminalSessionPhase.Idle, workspace.state.value.session.phase)
            }
      }
    } finally {
      edt { workspace.close() }
    }
  }

  @Test
  fun hiddenShellKeepsOneReaderAndBufferAcrossHostChanges() =
      withWorkspace { workspace, process, starts, path ->
        assertNull(workspace.state.value.widget)
        assertEquals(0, starts.get())
        edt { workspace.activate(path) }
        eventually { workspace.state.value.widget != null }
        val widget = assertNotNull(workspace.state.value.widget)
        edt { widget.ttyConnector.resize(com.jediterm.core.util.TermSize(103, 31)) }
        assertEquals(TerminalSize(103, 31), process.lastSize)
        val first = JPanel()
        edt { first.add(widget) }
        process.emit("first host\r\n")
        eventually { widget.terminalTextBuffer.getScreenLines().contains("first host") }
        edt { first.remove(widget) }
        process.emit("while hidden\r\n")
        eventually { widget.terminalTextBuffer.getScreenLines().contains("while hidden") }
        edt {
          workspace.activate(path)
          JPanel().add(widget)
        }
        assertSame(widget, workspace.state.value.widget)
        assertEquals(1, starts.get())
        assertTrue(process.alive)
      }

  @Test
  fun projectSwitchCannotRedirectLiveShellAndExplicitCloseAllowsNewSession() =
      withWorkspace { workspace, process, starts, path ->
        edt { workspace.activate(path) }
        eventually { workspace.state.value.widget != null }
        val widget = workspace.state.value.widget
        edt { workspace.activate("/different/project") }
        assertEquals(path, workspace.state.value.projectPath)
        assertSame(widget, workspace.state.value.widget)
        assertTrue(workspace.state.value.requiresClose)
        assertEquals(1, starts.get())
        val closing = edt { workspace.closeSession() }
        assertFalse(closing.get(5, TimeUnit.SECONDS).cleanupPending)
        assertFalse(process.alive)
        assertNull(workspace.state.value.widget)
        assertTrue(workspace.state.value.canOpen)
        edt { workspace.activate(path) }
        eventually { starts.get() == 2 }
      }

  @Test
  fun naturalExitKeepsItsExitCodeAndNeverAutoRestarts() =
      withWorkspace { workspace, process, starts, path ->
        edt { workspace.activate(path) }
        eventually { workspace.state.value.widget != null }
        process.finish(7)
        eventually { workspace.state.value.session.phase == TerminalSessionPhase.Exited }
        assertEquals("Shell exited (7)", terminalSummary(workspace.state.value.session))
        assertEquals(1, starts.get())
        assertFalse(workspace.state.value.requiresClose)
      }

  @Test
  fun applicationDisposalStopsReaderAndProcess() = withWorkspace { workspace, process, _, path ->
    edt { workspace.activate(path) }
    eventually { workspace.state.value.widget != null }
    edt { workspace.close() }
    eventually { !process.alive }
    assertEquals(1, process.stops.get())
  }

  @Test
  fun launchFailureIsActionableAndCanBeRetried() {
    val workspace = edt { DesktopTerminalWorkspace() }
    try {
      edt { workspace.activate("/mini-orca-missing-terminal-project") }
      eventually { workspace.state.value.session.phase == TerminalSessionPhase.Failed }
      assertTrue(workspace.state.value.session.error.orEmpty().isNotBlank())
      assertTrue(workspace.state.value.canOpen)
      assertNull(workspace.state.value.widget)
    } finally {
      edt { workspace.close() }
    }
  }

  private fun withWorkspace(
      block: (DesktopTerminalWorkspace, PaneProcess, AtomicInteger, String) -> Unit
  ) {
    val path = Files.createTempDirectory("mini-orca-pane-")
    val starts = AtomicInteger()
    val process = PaneProcess()
    val workspace = edt {
      DesktopTerminalWorkspace { directory ->
        DesktopTerminalSession(
            directory,
            shell = "/bin/sh",
            environment = emptyMap(),
            factory =
                TerminalProcessFactory {
                  starts.incrementAndGet()
                  if (starts.get() == 1) process else PaneProcess()
                })
      }
    }
    try {
      block(workspace, process, starts, path.toString())
    } finally {
      edt { workspace.closeSession() }.get(5, TimeUnit.SECONDS)
      edt { workspace.close() }
      Files.deleteIfExists(path)
    }
  }

  private class PaneProcess : TerminalProcess {
    override val input = PipedInputStream()
    private val stdout = PipedOutputStream(input)
    override val output = ByteArrayOutputStream()
    private val done = CountDownLatch(1)
    @Volatile private var code = 0
    override val alive: Boolean
      get() = done.count > 0

    val stops = AtomicInteger()

    @Volatile var lastSize = TerminalSize()

    override fun resize(size: TerminalSize) {
      lastSize = size
    }

    override fun waitFor(): Int {
      done.await()
      return code
    }

    fun emit(text: String) {
      stdout.write(text.toByteArray())
      stdout.flush()
    }

    fun finish(code: Int) {
      this.code = code
      stdout.close()
      done.countDown()
    }

    override fun terminate(): String? {
      stops.incrementAndGet()
      finish(code)
      input.close()
      return null
    }
  }
}

private fun <T> edt(block: () -> T): T {
  val result = CompletableFuture<T>()
  SwingUtilities.invokeAndWait {
    try {
      result.complete(block())
    } catch (error: Exception) {
      result.completeExceptionally(error)
    }
  }
  return result.get(5, TimeUnit.SECONDS)
}

private fun eventually(condition: () -> Boolean) {
  val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
  while (!condition() && System.nanoTime() < deadline) Thread.sleep(5)
  assertTrue(condition(), "Terminal condition timed out")
}

/**
 * Native smoke fixture: real application/window lifecycle and PTY, temporary source, no providers.
 */
fun main() {
  val directory = Files.createTempDirectory("mini-orca-terminal-ui-")
  val source = directory.resolve("main.go")
  Files.writeString(source, "package main\n\nfunc Run() {}\n")
  val store = LastProjectStore(InMemoryPreferences())
  store.save(directory.toString())
  val layoutStore = DesktopLayoutStore(InMemoryPreferences())
  val json = kotlinx.serialization.json.Json
  val project =
      ProjectAnalysis(
          "terminal-fixture",
          "revision",
          "Terminal verification",
          directory.toString(),
          "go",
          fileCount = 1,
          sourceFileCount = 1,
          totalLines = 3,
          summary = "Temporary native terminal verification project",
          aiStatus = "missing",
          analyzedAt = "")
  fun index() =
      json.encodeToString(
          ProjectIndex.serializer(),
          ProjectIndex(
              project.projectId,
              project.projectRevision,
              files = listOf(IndexedFile("main.go", "base", "Go", false))))
  val api =
      ApiClient(
          transport =
              DaemonTransport { method, path, _ ->
                val body =
                    when {
                      path == "/api/projects/restore" || path == "/api/projects/import" ->
                          json.encodeToString(ProjectAnalysis.serializer(), project)
                      path.startsWith("/api/projects/current/index") || path.endsWith("/reindex") ->
                          index()
                      path.contains("/files/info?") -> {
                        val content = Files.readString(source)
                        val hash =
                            java.security.MessageDigest.getInstance("SHA-256")
                                .digest(content.toByteArray())
                                .joinToString("") { "%02x".format(it) }
                        json.encodeToString(
                            ProjectFileInfo.serializer(),
                            ProjectFileInfo(
                                "main.go",
                                hash,
                                "main.go",
                                language = "Go",
                                sizeBytes = content.length.toLong(),
                                lineCount = 3,
                                modifiedAt = "",
                                binary = false,
                                content = content))
                      }
                      path.contains("/files/symbols?") ->
                          """{"project_id":"terminal-fixture","project_revision":"revision","path":"main.go","symbols":[{"name":"Run","kind":"function","start_line":3,"end_line":3,"confidence":"exact","atomic_target":true}]}"""
                      path.contains("/files/analysis?") ->
                          """{"path":"main.go","status":"missing"}"""
                      path.contains("/analysis/run?") -> "null"
                      path.contains("/impact") -> """{"target_path":"main.go"}"""
                      path.contains("/findings") -> "[]"
                      else -> "{}"
                    }
                check(
                    method == "GET" ||
                        path in
                            setOf(
                                "/api/projects/restore",
                                "/api/projects/import",
                                "/api/projects/current/reindex")) {
                      "Native fixture cannot call a provider: $method $path"
                    }
                TransportResponse(200, body)
              })
  println("Native terminal fixture: $directory")
  miniOrcaApplication(
      createTerminal = {
        DesktopTerminalWorkspace { path ->
          DesktopTerminalSession(
              path,
              shell = "/bin/zsh",
              environment =
                  System.getenv() +
                      mapOf("ZDOTDIR" to directory.toString(), "HISTFILE" to "/dev/null"))
        }
      }) { terminal ->
        MiniOrcaApp(terminal, api, store, layoutStore)
      }
}
