package io.miniorca.desktop

import com.jediterm.core.util.TermSize
import com.jediterm.terminal.ui.JediTermWidget
import java.io.ByteArrayInputStream
import java.io.ByteArrayOutputStream
import java.io.IOException
import java.io.InputStream
import java.nio.file.Files
import java.nio.file.Path
import java.util.concurrent.CompletableFuture
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import javax.swing.SwingUtilities
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class DesktopTerminalSessionTest {
  @Test
  fun creationIsIdleAndStartingIsExplicitAndIdempotent() = withTerminalDirectory { directory ->
    val gate = CountDownLatch(1)
    val entered = CountDownLatch(1)
    val calls = AtomicInteger()
    val process = FakeTerminalProcess()
    val session =
        testSession(directory) {
          calls.incrementAndGet()
          entered.countDown()
          gate.await()
          process
        }
    try {
      assertEquals(TerminalSessionPhase.Idle, session.state.value.phase)
      assertEquals(0, calls.get())
      assertEquals(null, session.connector())
      assertTrue(session.start())
      assertTrue(entered.await(5, TimeUnit.SECONDS))
      assertEquals(TerminalSessionPhase.Starting, session.state.value.phase)
      assertFalse(session.start())
      gate.countDown()
      waitForTerminal { session.state.value.phase == TerminalSessionPhase.Running }
      assertFalse(session.start())
      assertEquals(1, calls.get())
    } finally {
      gate.countDown()
      session.close().get(5, TimeUnit.SECONDS)
    }
    assertTrue(process.streamsClosed)
    assertEquals(1, process.terminations.get())
  }

  @Test
  fun utf8ResizeAndInterruptReachOnlyTheOwnedProcess() = withTerminalDirectory { directory ->
    val process = FakeTerminalProcess("héllo 世界")
    val session = testSession(directory) { process }
    try {
      session.start()
      waitForTerminal { session.connector() != null }
      val connector = requireNotNull(session.connector())
      val chars = CharArray(100)
      val count = connector.read(chars, 0, chars.size)
      assertEquals("héllo 世界", String(chars, 0, count))
      connector.write("café 世界\r")
      connector.write(byteArrayOf(3))
      assertEquals("café 世界\r\u0003", process.written.toString(Charsets.UTF_8))
      connector.resize(TermSize(121, 42))
      assertEquals(TerminalSize(121, 42), process.size)
      assertTrue(connector.isConnected)
    } finally {
      session.close().get(5, TimeUnit.SECONDS)
    }
  }

  @Test
  fun exitedSessionRetainsExitStatusAndAllowsAnExplicitNewStart() =
      withTerminalDirectory { directory ->
        val first = FakeTerminalProcess()
        val second = FakeTerminalProcess()
        val calls = AtomicInteger()
        val session = testSession(directory) { if (calls.incrementAndGet() == 1) first else second }
        try {
          session.start()
          waitForTerminal { session.state.value.phase == TerminalSessionPhase.Running }
          first.exit(7)
          waitForTerminal { session.state.value.phase == TerminalSessionPhase.Exited }
          assertEquals(7, session.state.value.exitCode)
          assertTrue(session.state.value.canStart)
          assertTrue(first.streamsClosed)
          assertTrue(session.start())
          waitForTerminal { session.state.value.phase == TerminalSessionPhase.Running }
          assertEquals(null, session.state.value.exitCode)
          assertEquals(2, calls.get())
        } finally {
          session.close().get(5, TimeUnit.SECONDS)
        }
      }

  @Test
  fun launchAndNativeLoadFailuresRemainVisibleAndRetryable() = withTerminalDirectory { directory ->
    listOf(IOException("Launch denied"), UnsatisfiedLinkError("Native helper unavailable"))
        .forEach { failure ->
          val calls = AtomicInteger()
          val session =
              testSession(directory) {
                if (calls.incrementAndGet() == 1) throw failure else FakeTerminalProcess()
              }
          try {
            session.start()
            waitForTerminal { session.state.value.phase == TerminalSessionPhase.Failed }
            assertTrue(session.state.value.error!!.contains(failure.message!!))
            assertTrue(session.state.value.canStart)
            assertTrue(session.start())
            waitForTerminal { session.state.value.phase == TerminalSessionPhase.Running }
          } finally {
            session.close().get(5, TimeUnit.SECONDS)
          }
        }
  }

  @Test
  fun closeDuringLaunchDisposesTheLateProcessWithoutResurrectingState() =
      withTerminalDirectory { directory ->
        val gate = CountDownLatch(1)
        val entered = CountDownLatch(1)
        val process = FakeTerminalProcess()
        val session =
            testSession(directory) {
              entered.countDown()
              gate.await()
              process
            }
        try {
          session.start()
          assertTrue(entered.await(5, TimeUnit.SECONDS))
          val close = session.close()
          assertEquals(TerminalSessionPhase.Closed, session.state.value.phase)
          assertTrue(session.state.value.cleanupPending)
          assertFalse(session.start())
          gate.countDown()
          val closed = close.get(5, TimeUnit.SECONDS)
          assertFalse(closed.cleanupPending)
          assertEquals(TerminalSessionPhase.Closed, closed.phase)
          assertEquals(null, session.connector())
          assertTrue(process.streamsClosed)
          assertEquals(1, process.terminations.get())
          assertTrue(session.close() === close)
        } finally {
          gate.countDown()
          session.close().get(5, TimeUnit.SECONDS)
        }
      }

  @Test
  fun streamAttachmentFailureDisposesTheStartedProcess() = withTerminalDirectory { directory ->
    val process = FakeTerminalProcess()
    val broken =
        object : TerminalProcess by process {
          override val input: InputStream
            get() = throw IOException("Stream unavailable")
        }
    val session = testSession(directory) { broken }
    try {
      session.start()
      waitForTerminal { session.state.value.phase == TerminalSessionPhase.Failed }
      assertTrue(session.state.value.error!!.contains("Could not attach terminal streams"))
      assertTrue(process.streamsClosed)
      assertEquals(1, process.terminations.get())
    } finally {
      session.close().get(5, TimeUnit.SECONDS)
    }
  }

  @Test
  fun readFailuresStopTheProcessAndKeepTheirCause() = withTerminalDirectory { directory ->
    val process = FakeTerminalProcess(readFailure = true)
    val session = testSession(directory) { process }
    try {
      session.start()
      waitForTerminal { session.connector() != null }
      assertFailsWith<IOException> { session.connector()!!.read(CharArray(5), 0, 5) }
      waitForTerminal {
        session.state.value.phase == TerminalSessionPhase.Failed &&
            !session.state.value.cleanupPending
      }
      assertTrue(session.state.value.error!!.contains("PTY read failed"))
      assertEquals(null, session.connector())
      assertTrue(process.streamsClosed)
      assertEquals(1, process.terminations.get())
    } finally {
      session.close().get(5, TimeUnit.SECONDS)
    }
  }

  @Test
  fun cleanupFailuresAreVisibleEvenWhenTheSessionIsClosed() = withTerminalDirectory { directory ->
    val process = FakeTerminalProcess(cleanupError = "Shell cleanup deadline exceeded")
    val session = testSession(directory) { process }
    session.start()
    waitForTerminal { session.connector() != null }
    val closed = session.close().get(5, TimeUnit.SECONDS)
    assertEquals(TerminalSessionPhase.Closed, closed.phase)
    assertTrue(closed.error!!.contains("cleanup deadline"))
    assertFalse(closed.canStart)
  }

  @Test
  fun failedTeardownKeepsOwnershipUntilTheProcessActuallyExits() =
      withTerminalDirectory { directory ->
        val process = FakeTerminalProcess()
        val stubborn =
            object : TerminalProcess by process {
              override fun terminate(): String = "Shell did not exit within the cleanup deadline."
            }
        val session = testSession(directory) { stubborn }
        try {
          session.start()
          waitForTerminal { session.connector() != null }
          val closed = session.close().get(5, TimeUnit.SECONDS)
          assertTrue(closed.cleanupPending)
          assertTrue(closed.error!!.contains("cleanup deadline"))
          assertFalse(session.start())
          assertEquals(null, session.connector())
          process.exit(137)
          waitForTerminal { !session.state.value.cleanupPending }
          assertEquals(TerminalSessionPhase.Closed, session.state.value.phase)
        } finally {
          process.terminate()
          session.close().get(5, TimeUnit.SECONDS)
        }
      }

  @Test
  fun projectPathsAreCanonicalProcessAttributesAndNeverShellSource() =
      withTerminalDirectory { root ->
        val directory = Files.createDirectory(root.resolve("space;$(touch unwanted)"))
        val link = Files.createSymbolicLink(root.resolve("project-link"), directory)
        val spec = terminalLaunchSpec(link.toString(), "/bin/sh", emptyMap(), TerminalSize())
        assertEquals(directory.toRealPath(), spec.directory)
        assertEquals(listOf("/bin/sh", "-l", "-i"), spec.command)
        assertEquals("xterm-256color", spec.environment["TERM"])
        assertEquals("UTF-8", spec.environment["LC_CTYPE"])
        assertFalse(Files.exists(root.resolve("unwanted")))
        assertFailsWith<IllegalArgumentException> {
          terminalLaunchSpec("relative", "/bin/sh", emptyMap(), TerminalSize())
        }
        assertFailsWith<IOException> {
          terminalLaunchSpec(
              root.resolve("missing").toString(), "/bin/sh", emptyMap(), TerminalSize())
        }
        assertFailsWith<IllegalArgumentException> {
          terminalLaunchSpec(root.toString(), "/missing/shell", emptyMap(), TerminalSize())
        }
        assertFailsWith<IllegalArgumentException> {
          terminalLaunchSpec(
              root.toString(), "/bin/sh", emptyMap(), TerminalSize(), "Windows", "amd64")
        }
      }

  @Test
  fun unavailableProjectDoesNotLaunchInAFallbackDirectory() = withTerminalDirectory { root ->
    val calls = AtomicInteger()
    val session =
        testSession(root.resolve("missing")) {
          calls.incrementAndGet()
          FakeTerminalProcess()
        }
    try {
      session.start()
      waitForTerminal { session.state.value.phase == TerminalSessionPhase.Failed }
      assertEquals(0, calls.get())
      assertTrue(session.state.value.canStart)
      assertTrue(session.state.value.error!!.isNotBlank())
    } finally {
      session.close().get(5, TimeUnit.SECONDS)
    }
  }

  @Test
  fun terminalDimensionsAndEmulatorScrollbackAreBounded() {
    assertFailsWith<IllegalArgumentException> { TerminalSize(0, 24) }
    assertFailsWith<IllegalArgumentException> { TerminalSize(80, 1_001) }
    SwingUtilities.invokeAndWait {
      val widget = JediTermWidget(80, 24, DesktopTerminalSettings())
      try {
        repeat(TERMINAL_SCROLLBACK_LINES + 100) {
          widget.terminal.writeCharacters("line")
          widget.terminal.newLine()
        }
        assertEquals(TERMINAL_SCROLLBACK_LINES, widget.terminalTextBuffer.historyLinesCount)
      } finally {
        widget.close()
      }
    }
  }

  @Test
  fun realSupportedHostPtyPassesTheInteractiveContract() {
    if (System.getProperty("miniOrca.terminalNativeSmoke") != "true") return
    nativeTerminalSmoke()
  }
}

private fun testSession(directory: Path, start: (TerminalLaunchSpec) -> TerminalProcess) =
    DesktopTerminalSession(
        directory.toString(), "/bin/sh", emptyMap(), TerminalProcessFactory(start))

private class FakeTerminalProcess(
    text: String = "",
    readFailure: Boolean = false,
    private val cleanupError: String? = null,
) : TerminalProcess {
  val written = ByteArrayOutputStream()
  val terminations = AtomicInteger()
  private val exited = CountDownLatch(1)
  @Volatile private var code = 0
  @Volatile var streamsClosed = false
  @Volatile var size = TerminalSize()
  override val input: InputStream =
      if (readFailure)
          object : InputStream() {
            override fun read(): Int = throw IOException("PTY read failed")
          }
      else ByteArrayInputStream(text.toByteArray(Charsets.UTF_8))
  override val output = written
  override val alive: Boolean
    get() = exited.count > 0

  override fun resize(size: TerminalSize) {
    this.size = size
  }

  override fun waitFor(): Int {
    exited.await()
    return code
  }

  fun exit(code: Int) {
    this.code = code
    exited.countDown()
  }

  override fun terminate(): String? {
    terminations.incrementAndGet()
    exit(code)
    input.close()
    output.close()
    streamsClosed = true
    return cleanupError
  }
}

private fun waitForTerminal(condition: () -> Boolean) {
  val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
  while (!condition()) {
    check(System.nanoTime() < deadline) { "Terminal state deadline exceeded" }
    Thread.sleep(5)
  }
}

private fun withTerminalDirectory(block: (Path) -> Unit) {
  val root = Files.createTempDirectory("mini-orca-terminal-test-")
  try {
    block(root)
  } finally {
    Files.walk(root).use { paths -> paths.sorted(Comparator.reverseOrder()).forEach(Files::delete) }
  }
}

/** Also invoked against the distributable's classes/runtime by the packaged-host probe. */
fun main() {
  nativeTerminalSmoke()
}

private fun nativeTerminalSmoke() = withTerminalDirectory { root ->
  val factory = TerminalProcessFactory { spec ->
    // No user login/profile startup scripts in a synthetic probe; production still uses -l -i.
    Pty4jTerminalProcessFactory.start(spec.copy(command = listOf("/bin/sh", "-i")))
  }
  val session =
      DesktopTerminalSession(
          root.toString(),
          "/bin/sh",
          mapOf("PATH" to "/usr/bin:/bin", "ENV" to "/dev/null", "PS1" to ""),
          factory)
  var reader: CompletableFuture<Void>? = null
  val transcript = StringBuilder()
  try {
    check(session.start())
    waitForTerminal { session.state.value.phase != TerminalSessionPhase.Starting }
    check(session.state.value.phase == TerminalSessionPhase.Running) {
      session.state.value.toString()
    }
    val connector = requireNotNull(session.connector())
    reader =
        CompletableFuture.runAsync {
          val chars = CharArray(1024)
          while (true) {
            val count = connector.read(chars, 0, chars.size)
            if (count < 0) break
            synchronized(transcript) {
              check(transcript.length + count < 64_000) { "Probe output limit exceeded" }
              transcript.append(chars, 0, count)
            }
          }
        }
    fun awaitText(text: String) {
      try {
        waitForTerminal { synchronized(transcript) { transcript.contains(text) } }
      } catch (error: IllegalStateException) {
        throw IllegalStateException(
            "Missing probe marker $text; state=${session.state.value}; synthetic output=${synchronized(transcript) { transcript.toString() }}",
            error)
      }
    }
    connector.write("set +H; stty -echo; printf '__READY__\\n'\r")
    awaitText("__READY__")
    connector.write(
        "printf '__CWD__'; pwd; printf '__UTF8__café 世界\\n'; test -t 0 && test -t 1 && printf '__TTY__yes\\n'\r")
    awaitText("__UTF8__café 世界")
    awaitText("__CWD__${root.toRealPath()}")
    awaitText("__TTY__yes")
    connector.resize(TermSize(121, 42))
    connector.write("printf '__SIZE__'; stty size\r")
    awaitText("__SIZE__42 121")
    connector.write("sleep 30\r")
    Thread.sleep(150)
    connector.write(byteArrayOf(3))
    connector.write("printf '__INTERRUPTED__%s\\n' \"$?\"\r")
    awaitText("__INTERRUPTED__130")
    connector.write("sleep 30 & printf '__CHILD__%s\\n' \"$!\"\r")
    awaitText("__CHILD__")
    waitForTerminal {
      synchronized(transcript) { Regex("__CHILD__(\\d+)").containsMatchIn(transcript) }
    }
    val pid =
        synchronized(transcript) {
          Regex("__CHILD__(\\d+)").find(transcript)!!.groupValues[1].toLong()
        }
    val start = System.nanoTime()
    val closed = session.close().get(5, TimeUnit.SECONDS)
    check(closed.error == null) { closed.error.orEmpty() }
    check(!closed.cleanupPending)
    waitForTerminal { !ProcessHandle.of(pid).map { it.isAlive }.orElse(false) }
    check(TimeUnit.NANOSECONDS.toMillis(System.nanoTime() - start) < 3_000)
    println(
        "PTY smoke passed: canonical cwd, UTF-8, real TTY, 121x42 resize, Ctrl+C, child cleanup, bounded close.")
  } finally {
    session.close().get(5, TimeUnit.SECONDS)
    reader?.handle { _, _ -> null }?.get(5, TimeUnit.SECONDS)
  }
}
