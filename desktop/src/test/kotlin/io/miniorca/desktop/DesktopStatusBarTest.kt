package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class DesktopStatusBarTest {
  @Test
  fun idlePresentationUsesOnlyKnownProjectProviderAndDaemonState() {
    val presentation = desktopStatusBarPresentation(statusBarState())

    assertEquals(
        listOf(
            "Status: Daemon ready",
            "Project: Mini",
            "Index: 1 files",
            "Function edits: local provider",
            "Daemon: Connected",
        ),
        presentation.segments.map { it.label },
    )
    assertTrue(desktopStatusBarDescription(presentation.segments).contains("Local endpoint"))
  }

  @Test
  fun statusBarIsPersistentForAProjectAndAbsentFromTheProjectLanding() {
    assertTrue(desktopStatusBarVisible(project()))
    assertFalse(desktopStatusBarVisible(null))
  }

  @Test
  fun busyRemoteAndStaleFilePresentationKeepsItsTextualState() {
    val symbol =
        SymbolInfo("Run", "function", startLine = 4, confidence = "exact", atomicTarget = true)
    val presentation =
        desktopStatusBarPresentation(
            statusBarState(
                selectedFile = selectedFile(),
                symbols = listOf(symbol),
                selectedSymbol = symbol,
                focusedLine = 7,
                analysis = FileAnalysis("main.go", "stale"),
                loading = true,
                operation = "Indexing project",
                provider = remoteProvider(),
            ))

    assertTrue(
        presentation.segments.any { it.label == "Working: Indexing project" && it.actionable })
    assertTrue(presentation.segments.any { it.label == "File: main.go · Go" })
    assertTrue(presentation.segments.any { it.label == "Symbol: Run · line 7" })
    assertTrue(presentation.segments.any { it.label == "File analysis: Stale" && it.attention })
    assertTrue(
        presentation.segments.any { it.label == "Function edits: remote provider" && it.attention })
  }

  @Test
  fun failuresAndDisconnectedDaemonRemainActionableAndAccurate() {
    val presentation =
        desktopStatusBarPresentation(
            statusBarState(
                error = "Analysis request failed",
                connection =
                    ConnectionState(label = "Daemon unavailable", locality = "Loopback endpoint"),
            ))

    assertTrue(
        presentation.segments.any {
          it.type == DesktopStatusSegmentType.Error &&
              it.label == "Error: Analysis request failed" &&
              it.actionable &&
              it.attention
        })
    assertTrue(
        presentation.segments.any {
          it.type == DesktopStatusSegmentType.Daemon &&
              it.label == "Daemon: Disconnected" &&
              it.detail.contains("Loopback endpoint") &&
              it.attention
        })
  }

  @Test
  fun focusedLineIsReportedWithoutRequiringASymbolSelection() {
    val presentation =
        desktopStatusBarPresentation(statusBarState(selectedFile = selectedFile(), focusedLine = 3))

    assertTrue(presentation.segments.any { it.label == "Line: 3" })
  }

  @Test
  fun responsivePriorityKeepsOperationErrorRemoteProviderAndDaemonAtNarrowWidths() {
    val presentation =
        desktopStatusBarPresentation(
            statusBarState(
                error = "Request failed",
                provider = remoteProvider(),
                selectedFile = selectedFile(),
                analysis = FileAnalysis("main.go", "stale"),
            ))

    assertEquals(presentation.segments, visibleDesktopStatusSegments(presentation, 1_220f))
    assertFalse(
        visibleDesktopStatusSegments(presentation, 1_000f).any {
          it.type in setOf(DesktopStatusSegmentType.Project, DesktopStatusSegmentType.Index)
        })
    assertEquals(
        setOf(
            DesktopStatusSegmentType.Error,
            DesktopStatusSegmentType.Operation,
            DesktopStatusSegmentType.Provider,
            DesktopStatusSegmentType.Daemon,
        ),
        visibleDesktopStatusSegments(presentation, 500f).map { it.type }.toSet(),
    )
  }

  @Test
  fun staleProjectOrFileDataCannotLeakIntoTheCurrentStatus() {
    val symbol =
        SymbolInfo("Old", "function", startLine = 2, confidence = "exact", atomicTarget = true)
    val presentation =
        desktopStatusBarPresentation(
            statusBarState(
                index = index(revision = "old-revision"),
                selectedFile = selectedFile(),
                symbols = listOf(symbol),
                selectedSymbol = symbol,
                focusedLine = 2,
                analysis = FileAnalysis("main.go", "fresh"),
            ))

    assertTrue(presentation.segments.any { it.label == "Index: unavailable" })
    assertFalse(
        presentation.segments.any {
          it.type in
              setOf(
                  DesktopStatusSegmentType.File,
                  DesktopStatusSegmentType.Symbol,
                  DesktopStatusSegmentType.Analysis,
              )
        })
  }

  private fun statusBarState(
      project: ProjectAnalysis? = project(),
      index: ProjectIndex? = index(),
      selectedFile: ProjectFileInfo? = null,
      symbols: List<SymbolInfo> = emptyList(),
      selectedSymbol: SymbolInfo? = null,
      focusedLine: Int = 0,
      analysis: FileAnalysis? = null,
      loading: Boolean = false,
      operation: String = "Daemon ready",
      error: String? = null,
      provider: DesktopStatusProvider = localProvider(),
      connection: ConnectionState = ConnectionState(connected = true, locality = "Local endpoint"),
  ) =
      DesktopStatusBarState(
          project = project,
          index = index,
          selectedFile = selectedFile,
          symbols = symbols,
          selectedSymbol = selectedSymbol,
          focusedLine = focusedLine,
          analysis = analysis,
          loading = loading,
          operation = operation,
          error = error,
          provider = provider,
          connection = connection,
      )

  private fun project(revision: String = "revision") =
      ProjectAnalysis(
          projectId = "project",
          projectRevision = revision,
          name = "Mini",
          path = "/tmp/mini",
          type = "go",
          fileCount = 1,
          sourceFileCount = 1,
          totalLines = 10,
          summary = "",
          aiStatus = "fresh",
          analyzedAt = "",
      )

  private fun index(revision: String = "revision") =
      ProjectIndex(
          projectId = "project",
          projectRevision = revision,
          files = listOf(IndexedFile("main.go", "hash", "Go", false)),
      )

  private fun selectedFile() =
      ProjectFileInfo(
          path = "main.go",
          contentHash = "hash",
          name = "main.go",
          language = "Go",
          sizeBytes = 10,
          lineCount = 10,
          modifiedAt = "",
          binary = false,
      )

  private fun localProvider() =
      DesktopStatusProvider(
          ModelScope.Function,
          ScopedModel(scope = "function", profile = "function", model = "local-model"),
      )

  private fun remoteProvider() =
      DesktopStatusProvider(
          ModelScope.Function,
          ScopedModel(
              scope = "function",
              profile = "function",
              model = "remote-model",
              remoteProvider = true,
          ),
      )
}
