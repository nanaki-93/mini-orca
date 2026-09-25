package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material.Text
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
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
  fun measuredDockStaysBetweenPaneAndFooterWithoutChangingSessionOnResize() =
      withWorkspace { workspace, process, starts, path ->
        edt { workspace.activate(path) }
        eventually { workspace.state.value.widget != null }
        val widget = workspace.state.value.widget
        val preferred = DesktopLayoutState(bottomCollapsed = false, bottomHeight = 520f)
        for (scale in listOf(1f, 1.25f, 1.5f)) {
          ComposeVisualFixture(800, 650, scale) {
                Column(Modifier.fillMaxSize()) {
                  Box(Modifier.testTag("toolbar")) { Text("Toolbar") }
                  WorkspaceFrame(
                      rail = { Box {} },
                      panes = { Box(Modifier.weight(1f).fillMaxSize().testTag("workspace-pane")) },
                      terminal = { height ->
                        TerminalDock(
                            layout = preferred,
                            effectiveHeight = resolveTerminalDockHeight(preferred, height, scale),
                            state = workspace.state.value,
                            onOpen = {},
                            onCollapse = {},
                            tabActions = TerminalTabActions({}, {}, {}),
                            onHeightDelta = {},
                            onHeightCommit = {},
                            content = { modifier -> Box(modifier.background(EditorCanvas)) },
                            controlModifier = Modifier.testTag("dock-toggle"),
                            modifier = Modifier.testTag("dock"))
                      },
                      modifier = Modifier.weight(1f))
                  Box(Modifier.testTag("footer")) { Text("Footer") }
                }
              }
              .use { fixture ->
                for (height in listOf(650, 480, 900)) {
                  fixture.resize(800, height)
                  fixture.render()
                  val pane = fixture.taggedBounds("workspace-pane")
                  val dock = fixture.taggedBounds("dock")
                  val footer = fixture.taggedBounds("footer")
                  assertTrue(pane.bottom <= dock.top, "$height/$scale: pane overlaps dock")
                  assertTrue(dock.bottom <= footer.top, "$height/$scale: dock overlaps footer")
                  assertTrue(dock.height >= MIN_EXPANDED_DOCK_HEIGHT * scale)
                  if (height == 900) assertEquals(preferred.bottomHeight, dock.height, 1f)
                  val toggle = fixture.taggedBounds("dock-toggle")
                  assertTrue(toggle.top >= dock.top && toggle.bottom <= dock.bottom)
                  assertTrue(fixture.hasDescription("New shell"))
                  assertSame(widget, workspace.state.value.widget)
                  assertEquals(1, starts.get())
                  assertTrue(process.alive)
                }
              }
          ComposeVisualFixture(800, 480, scale) {
                Column(Modifier.fillMaxSize()) {
                  WorkspaceFrame(
                      rail = { Box {} },
                      panes = { Box(Modifier.weight(1f).testTag("workspace-pane")) },
                      terminal = { height ->
                        TerminalDock(
                            layout = preferred.withBottomCollapsed(true),
                            effectiveHeight =
                                resolveTerminalDockHeight(
                                    preferred.withBottomCollapsed(true), height, scale),
                            state = workspace.state.value,
                            onOpen = {},
                            onCollapse = {},
                            tabActions = TerminalTabActions({}, {}, {}),
                            onHeightDelta = {},
                            onHeightCommit = {},
                            content = { modifier -> Box(modifier) },
                            controlModifier = Modifier.testTag("dock-toggle"),
                            modifier = Modifier.testTag("dock"))
                      },
                      modifier = Modifier.weight(1f))
                  Box(Modifier.testTag("footer")) { Text("Footer") }
                }
              }
              .use { fixture ->
                fixture.render()
                val dock = fixture.taggedBounds("dock")
                assertTrue(dock.height > 0f)
                assertTrue(dock.bottom <= fixture.taggedBounds("footer").top)
                assertTrue(fixture.taggedBounds("dock-toggle").bottom <= dock.bottom)
                assertSame(widget, workspace.state.value.widget)
                assertEquals(1, starts.get())
              }
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
        val closing = edt { workspace.closeAllSessions() }
        assertFalse(closing.get(5, TimeUnit.SECONDS).cleanupPending)
        assertFalse(process.alive)
        assertNull(workspace.state.value.widget)
        assertTrue(workspace.state.value.canCreate)
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
        edt { workspace.activate(path) }
        assertEquals(1, starts.get())
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
      assertTrue(workspace.state.value.canCreate)
      assertNull(workspace.state.value.widget)
    } finally {
      edt { workspace.close() }
    }
  }

  @Test
  fun independentTabsPreserveReadersAndCloseOnlyTheRequestedShell() =
      withWorkspace { workspace, first, starts, path ->
        edt { workspace.activate(path) }
        eventually { workspace.state.value.widget != null }
        val firstTab = workspace.state.value.activeTab!!
        edt { workspace.createShell(path) }
        eventually { workspace.state.value.tabs.size == 2 && workspace.state.value.widget != null }
        val secondTab = workspace.state.value.activeTab!!
        assertEquals(2, starts.get())
        assertTrue(firstTab.id != secondTab.id)
        first.emit("background tab output\r\n")
        eventually {
          firstTab.widget!!.terminalTextBuffer.getScreenLines().contains("background tab output")
        }
        edt { workspace.selectShell(firstTab.id) }
        assertSame(firstTab.widget, workspace.state.value.widget)
        edt { workspace.selectShell(secondTab.id) }
        edt { workspace.closeSession(firstTab.id) }.get(5, TimeUnit.SECONDS)
        assertFalse(first.alive)
        assertSame(secondTab.widget, workspace.state.value.widget)
        assertTrue(workspace.state.value.requiresClose)
        assertEquals(listOf(secondTab.id), workspace.state.value.tabs.map { it.id })
        edt { workspace.createShell(path) }
        eventually { workspace.state.value.tabs.size == 2 && workspace.state.value.widget != null }
        val thirdId = workspace.state.value.activeTabId!!
        edt { workspace.closeSession(thirdId) }.get(5, TimeUnit.SECONDS)
        assertEquals(secondTab.id, workspace.state.value.activeTabId)
        assertEquals(3, starts.get())
        edt { workspace.closeAllSessions() }.get(5, TimeUnit.SECONDS)
        assertTrue(workspace.state.value.tabs.isEmpty())
        assertNull(workspace.state.value.activeTabId)
        assertEquals(3, starts.get())
        edt { workspace.activate(path) }
        eventually { starts.get() == 4 }
      }

  @Test
  fun exitedSelectedTabCannotHideALiveShellFromProjectSwitchOrShutdown() =
      withWorkspace { workspace, first, _, path ->
        edt { workspace.activate(path) }
        eventually { workspace.state.value.widget != null }
        val firstId = workspace.state.value.activeTabId!!
        edt { workspace.createShell(path) }
        eventually { workspace.state.value.tabs.size == 2 && workspace.state.value.widget != null }
        val secondWidget = workspace.state.value.widget!!
        first.finish(9)
        eventually {
          workspace.state.value.tabs.first().session.phase == TerminalSessionPhase.Exited
        }
        edt { workspace.selectShell(firstId) }
        assertTrue(workspace.state.value.requiresClose)
        edt { workspace.createShell("/another-project") }
        assertEquals(path, workspace.state.value.projectPath)
        assertEquals(2, workspace.state.value.tabs.size)
        val closed = edt { workspace.closeAllSessions() }.get(5, TimeUnit.SECONDS)
        assertTrue(closed.tabs.isEmpty())
        assertFalse(secondWidget.ttyConnector.isConnected)
      }

  @Test
  fun closeAllWaitsForLateStartupAndPreventsAReplacement() {
    val directory = Files.createTempDirectory("mini-orca-tabs-start-")
    val entered = CountDownLatch(1)
    val release = CountDownLatch(1)
    val process = PaneProcess()
    val workspace = edt {
      DesktopTerminalWorkspace { path ->
        DesktopTerminalSession(
            path,
            shell = "/bin/sh",
            environment = emptyMap(),
            factory =
                TerminalProcessFactory {
                  entered.countDown()
                  release.await()
                  process
                })
      }
    }
    try {
      edt { workspace.activate(directory.toString()) }
      assertTrue(entered.await(5, TimeUnit.SECONDS))
      val closing = edt { workspace.closeAllSessions() }
      assertFalse(closing.isDone)
      assertTrue(workspace.state.value.cleanupPending)
      edt { workspace.createShell(directory.toString()) }
      assertEquals(1, workspace.state.value.tabs.size)
      release.countDown()
      assertTrue(closing.get(5, TimeUnit.SECONDS).tabs.isEmpty())
      assertFalse(process.alive)
      assertFalse(workspace.state.value.closingAll)
    } finally {
      release.countDown()
      edt { workspace.closeAllSessions() }.get(5, TimeUnit.SECONDS)
      edt { workspace.close() }
      Files.deleteIfExists(directory)
    }
  }

  @Test
  fun failedCleanupRetainsTheTabAndBlocksNewShellsUntilTheProcessExits() {
    val directory = Files.createTempDirectory("mini-orca-tabs-cleanup-")
    val process = PaneProcess()
    val stubborn =
        object : TerminalProcess by process {
          override fun terminate(): String = "Shell cleanup deadline exceeded"
        }
    val workspace = edt {
      DesktopTerminalWorkspace { path ->
        DesktopTerminalSession(
            path,
            shell = "/bin/sh",
            environment = emptyMap(),
            factory = TerminalProcessFactory { stubborn })
      }
    }
    try {
      edt { workspace.activate(directory.toString()) }
      eventually { workspace.state.value.widget != null }
      val closed = edt { workspace.closeAllSessions() }.get(5, TimeUnit.SECONDS)
      assertTrue(closed.cleanupPending)
      assertTrue(closed.requiresClose)
      assertTrue(closed.errors.single().contains("cleanup deadline"))
      edt { workspace.createShell(directory.toString()) }
      assertEquals(1, workspace.state.value.tabs.size)
      process.finish(137)
      eventually { workspace.state.value.tabs.isEmpty() }
      assertTrue(workspace.state.value.canCreate)
    } finally {
      process.terminate()
      edt { workspace.close() }
      Files.deleteIfExists(directory)
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
      edt { workspace.closeAllSessions() }.get(5, TimeUnit.SECONDS)
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
