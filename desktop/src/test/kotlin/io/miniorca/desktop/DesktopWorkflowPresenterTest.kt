package io.miniorca.desktop

import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob

class DesktopWorkflowPresenterTest {
  @Test
  fun lateFileResponseCannotReplaceTheLatestSelection() {
    val firstStarted = CountDownLatch(1)
    val releaseFirst = CountDownLatch(1)
    val presenter = presenter { _, path, _ ->
      when {
        path.contains("files/info?path=first.go") -> {
          firstStarted.countDown()
          releaseFirst.await(2, TimeUnit.SECONDS)
          response(fileJson("first.go", "first"))
        }
        path.contains("files/info?path=second.go") -> response(fileJson("second.go", "second"))
        path.contains("files/symbols?path=second.go") -> response(symbolsJson("second.go"))
        path.contains("files/analysis") ->
            response("{\"path\":\"second.go\",\"status\":\"missing\"}")
        path.contains("/impact") -> response("{\"target_path\":\"second.go\"}")
        path.contains("/git") -> response("{\"available\":false}")
        else -> error("unexpected request $path")
      }
    }
    try {
      loadProject(presenter)
      presenter.selectFile("first.go")
      assertTrue(firstStarted.await(1, TimeUnit.SECONDS))
      presenter.selectFile("second.go")
      eventually { presenter.snapshot.value.state.selectedFile?.path == "second.go" }
      releaseFirst.countDown()

      assertEquals("second.go", presenter.snapshot.value.state.selectedFile?.path)
    } finally {
      releaseFirst.countDown()
      presenter.close()
    }
  }

  @Test
  fun cancelingAnalysisRetainsTheCurrentFileAndClearsOperationState() {
    val analysisStarted = CountDownLatch(1)
    val releaseAnalysis = CountDownLatch(1)
    val presenter = presenter { _, path, _ ->
      when {
        path.contains("files/info?path=main.go") -> response(fileJson("main.go", "base"))
        path.contains("files/symbols?path=main.go") -> response(symbolsJson("main.go", "Run"))
        path == "/api/projects/current/files/analysis" -> {
          analysisStarted.countDown()
          releaseAnalysis.await(2, TimeUnit.SECONDS)
          response("{\"path\":\"main.go\",\"status\":\"fresh\"}")
        }
        path.contains("files/analysis") -> response("{\"path\":\"main.go\",\"status\":\"missing\"}")
        path.contains("/impact") -> response("{\"target_path\":\"main.go\"}")
        path.contains("/git") -> response("{\"available\":false}")
        else -> error("unexpected request $path")
      }
    }
    try {
      loadProject(presenter)
      presenter.selectFile("main.go")
      eventually { presenter.snapshot.value.state.selectedFile?.path == "main.go" }
      presenter.analyzeSelected(refresh = false)
      assertTrue(analysisStarted.await(1, TimeUnit.SECONDS))
      presenter.cancelAnalysis()
      eventually { !presenter.snapshot.value.analysisInProgress }

      assertEquals("main.go", presenter.snapshot.value.state.selectedFile?.path)
      assertFalse(presenter.snapshot.value.analysisInProgress)
    } finally {
      releaseAnalysis.countDown()
      presenter.close()
    }
  }

  @Test
  fun reconnectPublishesDaemonFailuresAndThenTheRecoveredConnection() {
    val attempts = AtomicInteger()
    val presenter = presenter { _, path, _ ->
      when (path) {
        "/status" ->
            if (attempts.incrementAndGet() == 1) error("daemon down")
            else response("{\"status\":\"ok\",\"version\":\"v1\"}")
        "/api/models/current" ->
            response(
                "{\"scopes\":{\"function\":{\"scope\":\"function\",\"profile\":\"local\",\"model\":\"fixture\"}}}")
        else -> error("unexpected request $path")
      }
    }
    try {
      presenter.refreshConnection()
      eventually { attempts.get() == 1 && !presenter.snapshot.value.state.connection.connected }
      presenter.refreshConnection()
      eventually { presenter.snapshot.value.state.connection.connected }

      assertEquals("Daemon connected", presenter.snapshot.value.state.connection.label)
      assertEquals("fixture", presenter.snapshot.value.model(ModelScope.Function).model)
    } finally {
      presenter.close()
    }
  }

  @Test
  fun lateAnalyzeAllPollingCannotOverwriteAReplacementProject() {
    val pollStarted = CountDownLatch(1)
    val releasePoll = CountDownLatch(1)
    val presenter = presenter { _, path, _ ->
      when (path) {
        "/api/projects/current/analysis-job" ->
            response(
                "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"running\",\"files\":[]}")
        "/api/projects/current/analysis-job?project_revision=revision" -> {
          pollStarted.countDown()
          releasePoll.await(2, TimeUnit.SECONDS)
          response(
              "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"completed\",\"files\":[]}")
        }
        else -> error("unexpected request $path")
      }
    }
    try {
      loadProject(presenter)
      presenter.startAnalyzeAll(AnalyzeAllRunOptions())
      assertTrue(pollStarted.await(1, TimeUnit.SECONDS))
      presenter.dispatch(
          DesktopEvent.ProjectLoaded(
              project("next", "next-revision"), ProjectIndex("next", "next-revision")))
      releasePoll.countDown()
      eventually { presenter.snapshot.value.state.project?.projectId == "next" }

      assertEquals("next", presenter.snapshot.value.state.project?.projectId)
      assertEquals(null, presenter.snapshot.value.state.findings.analyzeAll)
    } finally {
      releasePoll.countDown()
      presenter.close()
    }
  }

  @Test
  fun chatProposalReplacesOnlyTheActiveTaskDraft() {
    val presenter = presenter { _, path, _ ->
      when {
        path.contains("files/info?path=main.go") -> response(fileJson("main.go", "base"))
        path.contains("files/symbols?path=main.go") -> response(symbolsJson("main.go", "Run"))
        path.contains("files/analysis") -> response("{\"path\":\"main.go\",\"status\":\"missing\"}")
        path.contains("/impact") -> response("{\"target_path\":\"main.go\"}")
        path.contains("/git") -> response("{\"available\":false}")
        path == "/api/projects/current/chat/sessions" ->
            response(
                "{\"id\":\"session\",\"project_id\":\"project\",\"project_revision\":\"revision\",\"base_file_hash\":\"base\",\"open_path\":\"main.go\",\"mode\":\"replace_symbol\",\"target_symbol\":\"Run\",\"state\":\"active\",\"messages\":[]}")
        path == "/api/projects/current/chat/sessions/session/messages" ->
            response(
                "{\"session_id\":\"session\",\"draft\":{\"id\":\"draft\",\"project_id\":\"project\",\"project_revision\":\"revision\",\"base_file_hash\":\"base\",\"target_path\":\"main.go\",\"mode\":\"replace_symbol\",\"target_symbol\":\"Run\",\"declaration\":\"func Run() {}\",\"revision\":1,\"hash\":\"draft-hash\",\"state\":\"generated\"},\"assistant_message\":{\"role\":\"assistant\",\"content\":\"Ready\"}}")
        else -> error("unexpected request $path")
      }
    }
    try {
      loadProject(presenter)
      presenter.selectFile("main.go")
      eventually { presenter.snapshot.value.state.selectedFile?.path == "main.go" }
      presenter.dispatch(
          DesktopEvent.SymbolSelected(
              SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true)))
      presenter.sendChatMessage(ChatEditMode.ReplaceSymbol, "", "Improve Run")
      eventually { presenter.snapshot.value.state.review.draft?.id == "draft" }

      assertEquals("Run", presenter.snapshot.value.state.review.draft?.targetSymbol)
      assertEquals("session", presenter.snapshot.value.state.chat.session?.id)
    } finally {
      presenter.close()
    }
  }

  @Test
  fun applyAndUndoReloadOnlyTheBoundDraftFile() {
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "POST" to "/api/projects/current/apply" ->
            response(
                "{\"project_revision\":\"next\",\"post_apply_hash\":\"after\",\"undo_available\":true}")
        "POST" to "/api/projects/current/undo" ->
            response(
                "{\"project_revision\":\"restored\",\"post_apply_hash\":\"before\",\"undo_available\":true}")
        else -> response("{}")
      }
    }
    try {
      loadFile(presenter)
      val draft = draft()
      presenter.dispatch(DesktopEvent.DraftLoaded(draft))
      presenter.dispatch(
          DesktopEvent.ChecksLoaded(
              DraftCheckReport(
                  "main.go",
                  true,
                  draftId = draft.id,
                  draftRevision = draft.revision,
                  draftHash = draft.hash)))
      presenter.applyEditableDraft()
      eventually { presenter.snapshot.value.state.review.applied?.postApplyHash == "after" }
      presenter.undoAppliedDraft()
      eventually { presenter.snapshot.value.state.review.applied?.postApplyHash == "before" }

      assertEquals("before", presenter.snapshot.value.state.review.applied?.postApplyHash)
    } finally {
      presenter.close()
    }
  }

  private fun presenter(
      responder: (String, String, String?) -> TransportResponse
  ): DesktopWorkflowPresenter {
    val scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)
    return DesktopWorkflowPresenter(
        ApiClient(transport = DaemonTransport(responder)),
        LastProjectStore(),
        scope,
        Dispatchers.Default,
        5)
  }

  private fun loadProject(presenter: DesktopWorkflowPresenter) {
    presenter.dispatch(DesktopEvent.ProjectLoaded(project(), ProjectIndex("project", "revision")))
  }

  private fun loadFile(presenter: DesktopWorkflowPresenter) {
    loadProject(presenter)
    presenter.dispatch(
        DesktopEvent.FileLoaded(
            file(),
            listOf(SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true))))
    presenter.dispatch(
        DesktopEvent.SymbolSelected(
            SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true)))
  }

  private fun project(id: String = "project", revision: String = "revision") =
      ProjectAnalysis(
          id,
          revision,
          id,
          "/tmp/$id",
          "go",
          fileCount = 1,
          sourceFileCount = 1,
          totalLines = 1,
          summary = "",
          aiStatus = "missing",
          analyzedAt = "")

  private fun file() =
      ProjectFileInfo(
          "main.go",
          "base",
          "main.go",
          language = "Go",
          sizeBytes = 1,
          lineCount = 1,
          modifiedAt = "",
          binary = false,
          content = "package main")

  private fun draft() =
      DeclarationDraft(
          id = "draft",
          projectId = "project",
          projectRevision = "revision",
          baseFileHash = "base",
          targetPath = "main.go",
          mode = "replace_symbol",
          targetSymbol = "Run",
          declaration = "func Run() {}",
          revision = 1,
          hash = "draft-hash",
          validation =
              DeclarationValidation(
                  true, "strict_symbol", diff = UnifiedDiff("main.go", "main.go")),
      )

  private fun response(body: String) = TransportResponse(200, body)

  private fun fileJson(path: String, hash: String) =
      "{\"path\":\"$path\",\"content_hash\":\"$hash\",\"name\":\"$path\",\"language\":\"Go\",\"size_bytes\":1,\"line_count\":1,\"modified_at\":\"\",\"binary\":false,\"content\":\"package main\"}"

  private fun symbolsJson(path: String, symbol: String = "") =
      if (symbol.isBlank())
          """{"project_id":"project","project_revision":"revision","path":"$path","symbols":[]}"""
      else
          """{"project_id":"project","project_revision":"revision","path":"$path","symbols":[{"name":"$symbol","kind":"function","confidence":"exact","atomic_target":true}]}"""

  private fun eventually(condition: () -> Boolean) {
    val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(2)
    while (System.nanoTime() < deadline) {
      if (condition()) return
      Thread.sleep(10)
    }
    assertTrue(condition(), "condition did not become true")
  }
}
