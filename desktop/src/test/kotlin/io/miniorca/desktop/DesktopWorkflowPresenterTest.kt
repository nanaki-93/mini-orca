package io.miniorca.desktop

import androidx.compose.ui.input.key.Key
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.input.TextFieldValue
import java.net.http.HttpTimeoutException
import java.nio.file.Files
import java.util.Collections
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import java.util.prefs.AbstractPreferences
import java.util.prefs.BackingStoreException
import kotlin.coroutines.CoroutineContext
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.asCoroutineDispatcher
import kotlinx.coroutines.cancel
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withTimeout
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive

class DesktopWorkflowPresenterTest {

  @Test
  fun guardedApplyDeduplicatesActivationAndRejectsEditedCandidatesBeforeDispatch() {
    for (editBeforeDispatch in listOf(false, true)) {
      val main = QueuedDispatcher()
      val io = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + main)
      var applies = 0
      val owner =
          presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
            check(path.endsWith("/apply"))
            applies++
            TransportResponse(409, """{"message":"Source changed on disk"}""")
          }
      try {
        loadFile(owner)
        val current = draft()
        owner.dispatch(DesktopEvent.DraftLoaded(current))
        owner.dispatch(
            DesktopEvent.ChecksLoaded(
                DraftCheckReport(
                    "main.go",
                    true,
                    listOf(DraftCheck("gofmt", true, "passed")),
                    current.id,
                    current.revision,
                    current.hash)))
        owner.applyEditableDraft()
        owner.applyEditableDraft()
        assertEquals(
            DraftMutationStatus.Running, owner.snapshot.value.state.review.mutation?.status)
        assertNull(owner.snapshot.value.state.review.applied)
        if (editBeforeDispatch) owner.dispatch(DesktopEvent.DraftEdited(imports = listOf("fmt")))
        repeat(3) {
          main.runPending()
          io.runPending()
        }
        assertEquals(if (editBeforeDispatch) 0 else 1, applies)
        assertNull(owner.snapshot.value.state.review.applied)
        assertEquals(
            if (editBeforeDispatch) DraftMutationStatus.Failed else DraftMutationStatus.Conflict,
            owner.snapshot.value.state.review.mutation?.status)
        if (!editBeforeDispatch)
            assertEquals(DraftEditorStatus.Stale, owner.snapshot.value.state.review.editor?.status)
      } finally {
        owner.close()
        scope.cancel()
      }
    }
  }

  @Test
  fun repairHandoffCannotUseDirtyStaleForeignOrExhaustedEvidence() {
    for (condition in listOf("dirty", "stale", "target", "limit", "checking")) {
      val main = QueuedDispatcher()
      val io = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + main)
      val calls = mutableListOf<String>()
      val owner =
          presenter(parentScope = scope, ioDispatcher = io) { method, path, _ ->
            calls += "$method $path"
            creationFileResponse(path) ?: error("Repair must not send: $path")
          }
      try {
        loadQueuedChatFile(owner, main, io)
        val symbol = SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true)
        owner.dispatch(DesktopEvent.FileLoaded(file(), listOf(symbol)))
        owner.dispatch(DesktopEvent.SymbolSelected(symbol))
        val task = BugTaskSpec(targetPath = "main.go", targetSymbol = "Run")
        val current = draft().copy(taskSpec = task)
        owner.dispatch(DesktopEvent.DraftLoaded(current))
        owner.dispatch(
            DesktopEvent.ChatLoaded(
                ChatSession(
                    id = "session",
                    projectId = current.projectId,
                    projectRevision = current.projectRevision,
                    baseFileHash = current.baseFileHash,
                    openPath = current.targetPath,
                    mode = current.mode,
                    targetSymbol = current.targetSymbol,
                    latestDraftId = current.id,
                    state = "active",
                    taskSpec = task,
                    repairCount = if (condition == "limit") 3 else 0)))
        owner.dispatch(
            DesktopEvent.ChecksLoaded(
                DraftCheckReport(
                    "main.go",
                    false,
                    listOf(DraftCheck("test", true, "failed", output = "failure")),
                    current.id,
                    current.revision,
                    current.hash)))
        when (condition) {
          "dirty" ->
              owner.dispatch(DesktopEvent.DraftEdited(declaration = "func Run() int { return 1 }"))
          "stale" -> owner.dispatch(DesktopEvent.DraftMarkedStale)
          "target" ->
              owner.dispatch(
                  DesktopEvent.SymbolSelected(
                      SymbolInfo("Other", "function", confidence = "exact", atomicTarget = true)))
          "checking" -> owner.runDraftChecks()
        }
        calls.clear()
        owner.reviseWithCheckOutput(ChatEditMode.ReplaceSymbol, "")
        assertTrue(
            owner.snapshot.value.state.error?.contains(
                if (condition == "limit") "limit" else "blocked") == true,
            "$condition: ${owner.snapshot.value.state.error}")
        assertTrue(calls.isEmpty())
        assertEquals(current.declaration, owner.snapshot.value.state.review.draft?.declaration)
      } finally {
        owner.close()
        scope.cancel()
      }
    }
  }

  @Test
  fun focusedChecksRecheckCandidateAndTrustIdentityBeforeEveryPrivilegedRequest() {
    for (scenario in
        listOf("valid", "foreign-preview", "foreign-ack", "denied", "edit-preview", "edit-ack")) {
      val main = QueuedDispatcher()
      val io = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + main)
      val calls = mutableListOf<String>()
      lateinit var owner: DesktopWorkflowPresenter
      val current =
          draft()
              .copy(
                  taskSpec =
                      BugTaskSpec(
                          targetPath = "main.go",
                          targetSymbol = "Run",
                          goTestCandidate =
                              GoTestCandidateSpec("TestRun", "func TestRun(t *testing.T) {}")))
      owner =
          presenter(parentScope = scope, ioDispatcher = io, interceptTrust = false) {
              method,
              path,
              _ ->
            if (path.contains("execution-trust")) {
              calls += method
              val preview = method == "GET"
              if (scenario == if (preview) "edit-preview" else "edit-ack")
                  owner.dispatch(
                      DesktopEvent.DraftEdited(declaration = "func Run() int { return 2 }"))
              val foreign = scenario == if (preview) "foreign-preview" else "foreign-ack"
              response(
                  Json.encodeToString(
                      ExecutionTrust(
                          if (foreign) "other" else "project",
                          "revision",
                          trusted = scenario != "denied",
                          commands =
                              listOf(
                                  if (preview) listOf("go", "test", "./...", "-run", "^TestRun$")
                                  else listOf("go", "test", "./...")))))
            } else if (path.endsWith("/drafts/draft/checks")) {
              calls += "checks"
              response(
                  Json.encodeToString(
                      DraftCheckReport(
                          "main.go",
                          true,
                          listOf(DraftCheck("focused test", true, "passed")),
                          current.id,
                          current.revision,
                          current.hash)))
            } else creationFileResponse(path) ?: error("Unexpected $path")
          }
      try {
        loadQueuedChatFile(owner, main, io)
        owner.dispatch(DesktopEvent.DraftLoaded(current))
        owner.runDraftChecks()
        repeat(4) {
          main.runPending()
          io.runPending()
        }
        assertEquals(
            when (scenario) {
              "valid" -> listOf("GET", "POST", "checks")
              "foreign-preview",
              "edit-preview" -> listOf("GET")
              else -> listOf("GET", "POST")
            },
            calls,
            scenario)
        if (scenario.startsWith("edit")) {
          assertEquals(DraftEditorStatus.Dirty, owner.snapshot.value.state.review.editor?.status)
          assertNull(owner.snapshot.value.state.checks)
        } else if (scenario != "valid") {
          assertEquals(
              ValidationAttemptStatus.Failed,
              owner.snapshot.value.state.review.checkAttempt?.status)
        }
      } finally {
        owner.close()
        scope.cancel()
      }
    }
  }

  @Test
  fun contextInspectionPublishesLoadingAndOnlyAResponseCanProduceReady() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val calls = mutableListOf<String>()
    var fail = false
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { method, path, _ ->
          require(method == "GET" && path.startsWith("/api/projects/current/context?"))
          calls += path
          if (fail) TransportResponse(503, "") else response("{}")
        }
    try {
      loadFile(presenter)
      presenter.inspectContext("fix")
      val loading = presenter.snapshot.value.contextInspection
      assertEquals(ContextInspectionStatus.Loading, loading.status)
      assertNull(loading.manifest)
      assertEquals("main.go", loading.identity?.file?.path)
      assertEquals("base", loading.identity?.file?.contentHash)
      assertEquals("revision", loading.identity?.file?.project?.revision)
      assertEquals("Run", loading.identity?.symbol?.name)
      presenter.inspectContext("fix")
      main.runPending()
      io.runPending()
      main.runPending()
      assertEquals(1, calls.size)
      assertEquals(ContextInspectionStatus.Ready, presenter.snapshot.value.contextInspection.status)
      assertEquals(emptyList(), presenter.snapshot.value.contextInspection.manifest?.included)

      fail = true
      presenter.retryContextInspection()
      assertEquals(
          ContextInspectionStatus.Loading, presenter.snapshot.value.contextInspection.status)
      assertNull(presenter.snapshot.value.contextInspection.manifest)
      main.runPending()
      io.runPending()
      main.runPending()
      val failed = presenter.snapshot.value.contextInspection
      assertEquals(ContextInspectionStatus.Failed, failed.status)
      assertTrue(failed.message.isNotBlank())
      assertNull(failed.manifest)
      assertEquals(2, calls.size)
      assertFalse(presenter.snapshot.value.state.status.contains("503"))
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun returnedKnownDestinationMismatchIsStaleNotCurrent() {
    val main = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    var responseScope = "bug"
    val presenter =
        presenter(parentScope = scope, ioDispatcher = main) { method, path, _ ->
          require(method == "GET" && path.startsWith("/api/projects/current/context?"))
          response("""{"scope":"$responseScope","model":"returned-model","remote_provider":true}""")
        }
    try {
      loadFile(presenter)
      presenter.inspectContext("fix")
      main.runPending()
      val stale = presenter.snapshot.value.contextInspection
      assertEquals(ContextInspectionStatus.Stale, stale.status)
      assertNull(stale.manifest)
      assertTrue(stale.message.contains("destination"))
      assertEquals("main.go", stale.identity?.file?.path)
      presenter.retryContextInspection()
      main.runPending()
      assertEquals(ContextInspectionStatus.Stale, presenter.snapshot.value.contextInspection.status)
      responseScope = ""
      presenter.inspectContext("fix")
      main.runPending()
      assertEquals(ContextInspectionStatus.Ready, presenter.snapshot.value.contextInspection.status)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun contextInspectionCapturesCreationTargetAndUsesCanonicalPreviewAction() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val calls = mutableListOf<String>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
          calls += path
          response("{}")
        }
    try {
      loadFile(presenter)
      presenter.inspectContext("document", ChatEditMode.CreateSymbol, "NewType", "type")
      val identity = presenter.snapshot.value.contextInspection.identity!!
      assertEquals("fix", identity.action)
      assertNull(identity.symbol)
      assertEquals("NewType", identity.creationName)
      assertEquals("type", identity.creationKind)
      main.runPending()
      io.runPending()
      main.runPending()
      assertEquals(listOf("/api/projects/current/context?path=main.go&action=fix"), calls)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun assistantIntentsUseOnlySupportedPreviewActionsWithoutChangingConsentOrEditEvidence() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val calls = mutableListOf<Triple<String, String, String?>>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { method, path, body ->
          calls += Triple(method, path, body)
          response("{}")
        }
    try {
      loadFile(presenter)
      val before = presenter.snapshot.value
      listOf("fix" to "Fix", "refactor" to "Refactor", "document" to "Document").forEach {
          (operation, label) ->
        presenter.inspectContext(operation)
        assertEquals(label, presenter.snapshot.value.contextInspection.identity?.intent)
        assertEquals("fix", presenter.snapshot.value.contextInspection.identity?.action)
        main.runPending()
        io.runPending()
        main.runPending()
      }
      presenter.inspectContext("create", ChatEditMode.CreateSymbol, "Build", "function")
      assertEquals("Create function", presenter.snapshot.value.contextInspection.identity?.intent)
      assertEquals("Build", presenter.snapshot.value.contextInspection.identity?.creationName)
      main.runPending()
      io.runPending()
      main.runPending()
      presenter.inspectContext("unsupported-preset")
      assertEquals("fix", presenter.snapshot.value.contextInspection.identity?.action)
      main.runPending()
      io.runPending()
      main.runPending()
      assertEquals(
          List(5) {
            Triple<String, String, String?>(
                "GET", "/api/projects/current/context?path=main.go&action=fix", null)
          },
          calls)
      presenter.inspectContext("analyze_file")
      assertEquals("analyze_file", presenter.snapshot.value.contextInspection.identity?.action)
      main.runPending()
      io.runPending()
      main.runPending()
      assertEquals(
          Triple("GET", "/api/projects/current/context?path=main.go&action=analyze_file", null),
          calls.last())
      assertEquals(before.providerConfirmations, presenter.snapshot.value.providerConfirmations)
      assertEquals(before.state.review, presenter.snapshot.value.state.review)
      assertEquals(before.state.chat, presenter.snapshot.value.state.chat)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun contextInspectionWithoutProjectOrFileExplainsUnavailableWithoutRequest() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    var requests = 0
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { _, _, _ ->
          requests++
          response("{}")
        }
    try {
      presenter.inspectContext("refactor")
      assertEquals(
          ContextInspectionStatus.Failed, presenter.snapshot.value.contextInspection.status)
      assertTrue(presenter.snapshot.value.contextInspection.message.contains("project"))
      loadProject(presenter)
      presenter.inspectContext("document")
      assertEquals(
          ContextInspectionStatus.Failed, presenter.snapshot.value.contextInspection.status)
      assertTrue(presenter.snapshot.value.contextInspection.message.contains("file"))
      main.runPending()
      io.runPending()
      main.runPending()
      assertEquals(0, requests)
      assertNull(presenter.snapshot.value.contextInspection.manifest)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun contextInspectionReplacementAndDismissalRejectLateSuccessAndFailure() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    var fail = false
    var calls = 0
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { _, _, _ ->
          calls++
          if (fail) TransportResponse(503, "") else response("{}")
        }
    try {
      loadFile(presenter)
      presenter.inspectContext("fix")
      main.runPending()
      io.runPending() // The first result is waiting to publish on main.
      presenter.retryContextInspection()
      main.runPending()
      assertEquals(
          ContextInspectionStatus.Loading, presenter.snapshot.value.contextInspection.status)
      io.runPending()
      main.runPending()
      assertEquals(ContextInspectionStatus.Ready, presenter.snapshot.value.contextInspection.status)
      assertEquals(2, calls)

      presenter.retryContextInspection()
      main.runPending()
      io.runPending() // A successful response is waiting when the dialog closes.
      presenter.closeContextInspection()
      main.runPending()
      assertEquals(
          ContextInspectionStatus.Closed, presenter.snapshot.value.contextInspection.status)
      assertEquals(3, calls)

      fail = true
      presenter.inspectContext("fix")
      main.runPending()
      io.runPending() // The error is waiting to publish.
      fail = false
      presenter.retryContextInspection()
      main.runPending()
      assertEquals(
          ContextInspectionStatus.Loading, presenter.snapshot.value.contextInspection.status)
      io.runPending()
      main.runPending()
      assertEquals(ContextInspectionStatus.Ready, presenter.snapshot.value.contextInspection.status)
      assertEquals(5, calls)
      assertFalse(presenter.snapshot.value.state.status.contains("503"))

      fail = true
      presenter.inspectContext("fix")
      main.runPending()
      io.runPending() // The error is waiting to publish after dismissal.
      presenter.closeContextInspection()
      main.runPending()
      assertEquals(
          ContextInspectionStatus.Closed, presenter.snapshot.value.contextInspection.status)
      assertNull(presenter.snapshot.value.contextInspection.manifest)
      assertEquals(6, calls)
      assertFalse(presenter.snapshot.value.state.status.contains("503"))

      presenter.inspectContext("fix")
      presenter.cancelContextInspection()
      main.runPending()
      io.runPending()
      main.runPending()
      assertEquals(
          ContextInspectionStatus.Canceled, presenter.snapshot.value.contextInspection.status)
      assertNull(presenter.snapshot.value.contextInspection.manifest)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun inspectionInvalidatesFileHashRevisionAndDeclarationWithoutChangingEditEvidence() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val calls = AtomicInteger()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { _, _, _ ->
          calls.incrementAndGet()
          response("{}")
        }
    try {
      loadFile(presenter)
      val other = SymbolInfo("Other", "function", confidence = "exact", atomicTarget = true)
      val changes: List<(DesktopWorkflowPresenter) -> Unit> =
          listOf(
              { it.dispatch(DesktopEvent.SymbolSelected(other)) },
              {
                it.dispatch(
                    DesktopEvent.SelectedFileRefreshed(
                        file().copy(contentHash = "new"), listOf(other)))
              },
              { it.dispatch(DesktopEvent.IndexRefreshed(ProjectIndex("project", "next"))) },
              {
                it.dispatch(DesktopEvent.FileLoaded(file().copy(path = "other.go"), listOf(other)))
              },
          )
      for (change in changes) {
        loadFile(presenter)
        presenter.inspectContext("fix")
        val captured = presenter.snapshot.value.contextInspection.identity
        main.runPending()
        io.runPending() // Completion waits on the presenter dispatcher.
        change(presenter)
        val stale = presenter.snapshot.value.contextInspection
        assertEquals(ContextInspectionStatus.Stale, stale.status)
        assertEquals(captured, stale.identity)
        val evidence =
            presenter.snapshot.value.let {
              Triple(it.state.chat, it.state.review, it.providerConfirmations)
            }
        val beforeRetry = calls.get()
        presenter.retryContextInspection()
        main.runPending()
        assertEquals(
            ContextInspectionStatus.Stale, presenter.snapshot.value.contextInspection.status)
        assertEquals(beforeRetry, calls.get())
        assertEquals(
            evidence,
            presenter.snapshot.value.let {
              Triple(it.state.chat, it.state.review, it.providerConfirmations)
            })
        presenter.inspectContext("fix") // A new explicit activation captures the changed target.
        assertEquals(
            ContextInspectionStatus.Loading, presenter.snapshot.value.contextInspection.status)
        presenter.closeContextInspection()
      }
      main.runPending()
      io.runPending()
      main.runPending()
      assertEquals(
          ContextInspectionStatus.Closed, presenter.snapshot.value.contextInspection.status)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun samePathFileLoadWithNewHashStalesReadyInspectionAndRejectsRetry() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val contextCalls = AtomicInteger()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { method, path, _ ->
          require(method == "GET")
          when {
            path.startsWith("/api/projects/current/context?") -> {
              contextCalls.incrementAndGet()
              response("{}")
            }
            path.contains("files/info?path=main.go") -> response(fileJson("main.go", "new"))
            path.contains("files/symbols?path=main.go") ->
                response(symbolsJson("main.go", "Run", "func Run()", 2, 10))
            path.contains("files/analysis") -> response("""{"path":"main.go","status":"missing"}""")
            path.contains("/impact") -> response("""{"target_path":"main.go"}""")
            path.contains("/git") -> response("""{"available":false}""")
            else -> error("Unexpected $path")
          }
        }
    try {
      loadFile(presenter)
      presenter.dispatch(
          DesktopEvent.SymbolSelected(
              SymbolInfo("Run", "function", "func Run()", 2, 10, "exact", true)))
      val index =
          resultIndexFixture()
              .copy(files = resultIndexFixture().files.map { it.copy(contentHash = "new") })
      presenter.dispatch(DesktopEvent.IndexRefreshed(index))
      val run = analysisRunFixture()
      val finding =
          UnifiedFinding(
              id = "updated",
              category = "bugs",
              projectId = "project",
              projectRevision = "revision",
              fileHash = "new",
              freshness = "fresh",
              location = FindingLocation("main.go", symbol = "Run"),
              taskSpec = BugTaskSpec("1", "main.go", "Run", "func Run()", listOf("Fix Run.")))
      presenter.dispatch(
          DesktopEvent.AnalysisRunUpdated(
              ProjectAnalysisRunState(
                  run = run,
                  sections =
                      mapOf(
                          AnalysisResultKey("bugs") to
                              AnalysisSectionState(
                                  results =
                                      analysisResultsFixture(run, "bugs")
                                          .copy(semantic = listOf(finding)))))))
      presenter.inspectContext("fix", ChatEditMode.CreateSymbol, "Fresh", "type")
      main.runPending()
      io.runPending()
      main.runPending()
      val captured = presenter.snapshot.value.contextInspection.identity
      assertEquals(ContextInspectionStatus.Ready, presenter.snapshot.value.contextInspection.status)
      assertEquals("base", captured?.file?.contentHash)

      presenter.prepareFinding(finding) // Reloads main.go rather than navigating to another path.
      assertEquals(ContextInspectionStatus.Ready, presenter.snapshot.value.contextInspection.status)
      main.runPending()
      assertEquals(ContextInspectionStatus.Ready, presenter.snapshot.value.contextInspection.status)
      io.runPending()
      assertEquals(ContextInspectionStatus.Ready, presenter.snapshot.value.contextInspection.status)
      main.runPending()
      val inspection = presenter.snapshot.value.contextInspection
      assertEquals("new", presenter.snapshot.value.state.selectedFile?.contentHash)
      assertEquals(ContextInspectionStatus.Stale, inspection.status)
      assertEquals(captured, inspection.identity)
      assertEquals(1, contextCalls.get())
      presenter.retryContextInspection()
      main.runPending()
      io.runPending()
      main.runPending()
      assertEquals(ContextInspectionStatus.Stale, presenter.snapshot.value.contextInspection.status)
      assertEquals(1, contextCalls.get())
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun changedHashIsStaleAtFilePublicationBeforeNavigationSelection() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val calls = AtomicInteger()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { method, path, _ ->
          require(method == "GET")
          when {
            path.startsWith("/api/projects/current/context?") -> {
              calls.incrementAndGet()
              response("{}")
            }
            path.contains("files/info?path=main.go") -> response(fileJson("main.go", "new"))
            path.contains("files/symbols?path=main.go") -> response(symbolsJson("main.go"))
            path.contains("files/analysis") -> response("""{"path":"main.go","status":"missing"}""")
            path.contains("/impact") -> response("""{"target_path":"main.go"}""")
            path.contains("/git") -> response("""{"available":false}""")
            else -> error("Unexpected $path")
          }
        }
    try {
      val page = performancePageFixture()
      presenter.dispatch(DesktopEvent.ProjectLoaded(resultProjectFixture(), resultIndexFixture()))
      presenter.dispatch(
          DesktopEvent.AnalysisRunUpdated(
              ProjectAnalysisRunState(
                  run = page.run,
                  sections = mapOf(AnalysisResultKey("performance") to page.section))))
      val result =
          performanceResults(presenter.snapshot.value.state.analysisResultPage("performance"))
              .single()
      var statusAtPublication: ContextInspectionStatus? = null
      presenter.openPerformanceFinding(
          result,
          onLoaded = {
            assertEquals("new", presenter.snapshot.value.state.selectedFile?.contentHash)
            statusAtPublication = presenter.snapshot.value.contextInspection.status
          })
      main.runPending() // Start the file read before the inspection is admitted.
      presenter.dispatch(DesktopEvent.FileLoaded(file(), emptyList()))
      presenter.inspectContext("fix", ChatEditMode.CreateSymbol, "Fresh", "type")
      val captured = presenter.snapshot.value.contextInspection.identity
      main.runPending()
      io.runLast() // Complete the inspection before the pending same-path file read.
      main.runPending()
      assertEquals(ContextInspectionStatus.Ready, presenter.snapshot.value.contextInspection.status)
      io.runPending()
      main.runPending()
      assertEquals(ContextInspectionStatus.Stale, statusAtPublication)
      assertEquals(captured, presenter.snapshot.value.contextInspection.identity)
      presenter.retryContextInspection()
      main.runPending()
      io.runPending()
      assertEquals(1, calls.get())
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun inspectionCreationTargetChangeRetainsCapturedIdentityAndRejectsRetry() {
    val main = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val calls = AtomicInteger()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = main) { _, _, _ ->
          calls.incrementAndGet()
          response("{}")
        }
    try {
      loadFile(presenter)
      presenter.inspectContext("fix", ChatEditMode.CreateSymbol, "NewType", "type")
      val captured = presenter.snapshot.value.contextInspection.identity
      presenter.contextCreationTargetChanged("OtherType", "type")
      assertEquals(ContextInspectionStatus.Stale, presenter.snapshot.value.contextInspection.status)
      assertEquals(captured, presenter.snapshot.value.contextInspection.identity)
      presenter.retryContextInspection()
      main.runPending()
      assertEquals(0, calls.get())
      presenter.inspectContext("fix", ChatEditMode.CreateSymbol, "OtherType", "type")
      main.runPending()
      assertEquals(ContextInspectionStatus.Ready, presenter.snapshot.value.contextInspection.status)
      assertEquals(1, calls.get())
      presenter.contextCreationTargetChanged("", "") // Switch back to declaration editing.
      assertEquals(ContextInspectionStatus.Stale, presenter.snapshot.value.contextInspection.status)
      presenter.retryContextInspection()
      main.runPending()
      assertEquals(1, calls.get())
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun changedDeclarationRejectsLateFailureEvenAfterFreshSameFileInspection() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    var fail = true
    val calls = AtomicInteger()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { _, _, _ ->
          calls.incrementAndGet()
          if (fail) TransportResponse(503, "") else response("{}")
        }
    try {
      loadFile(presenter)
      presenter.inspectContext("fix")
      main.runPending()
      io.runPending() // The error is waiting to publish.
      presenter.dispatch(
          DesktopEvent.SymbolSelected(
              SymbolInfo("Other", "function", confidence = "exact", atomicTarget = true)))
      assertEquals(ContextInspectionStatus.Stale, presenter.snapshot.value.contextInspection.status)
      fail = false
      presenter.inspectContext("fix")
      main.runPending()
      io.runPending()
      main.runPending()
      assertEquals(2, calls.get())
      assertEquals(ContextInspectionStatus.Ready, presenter.snapshot.value.contextInspection.status)
      assertEquals("Other", presenter.snapshot.value.contextInspection.identity?.symbol?.name)
      assertFalse(presenter.snapshot.value.state.status.contains("503"))
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun pendingFileNavigationInvalidatesInspectionBeforeTheReadCompletes() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val contextCalls = AtomicInteger()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
          if (path.startsWith("/api/projects/current/context?")) {
            contextCalls.incrementAndGet()
            response("{}")
          } else error("Unexpected $path")
        }
    try {
      loadFile(presenter)
      presenter.inspectContext("fix")
      val captured = presenter.snapshot.value.contextInspection.identity
      presenter.selectFile("other.go")
      assertEquals(ContextInspectionStatus.Stale, presenter.snapshot.value.contextInspection.status)
      assertEquals(captured, presenter.snapshot.value.contextInspection.identity)
      presenter.retryContextInspection()
      main.runPending()
      io.runPending()
      main.runPending()
      assertEquals(0, contextCalls.get())
      assertEquals(ContextInspectionStatus.Stale, presenter.snapshot.value.contextInspection.status)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun inspectionClosesOnProjectSwitchAndRejectsLateCompletion() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val presenter = presenter(parentScope = scope, ioDispatcher = io) { _, _, _ -> response("{}") }
    try {
      loadFile(presenter)
      presenter.inspectContext("fix")
      main.runPending()
      io.runPending()
      presenter.dispatch(
          DesktopEvent.ProjectLoaded(project("other"), ProjectIndex("other", "revision")))
      assertEquals(
          ContextInspectionStatus.Closed, presenter.snapshot.value.contextInspection.status)
      main.runPending()
      assertEquals(
          ContextInspectionStatus.Closed, presenter.snapshot.value.contextInspection.status)
      presenter.retryContextInspection()
      assertEquals(
          ContextInspectionStatus.Closed, presenter.snapshot.value.contextInspection.status)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun staleInspectionClosesOnProjectSwitchInsteadOfRetainingTheOldOwner() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val calls = AtomicInteger()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { _, _, _ ->
          calls.incrementAndGet()
          response("{}")
        }
    try {
      loadFile(presenter)
      presenter.inspectContext("fix")
      val captured = presenter.snapshot.value.contextInspection.identity
      main.runPending()
      io.runPending() // Completion is still queued on the presenter dispatcher.
      presenter.dispatch(
          DesktopEvent.SymbolSelected(
              SymbolInfo("Other", "function", confidence = "exact", atomicTarget = true)))
      assertEquals(ContextInspectionStatus.Stale, presenter.snapshot.value.contextInspection.status)
      assertEquals(captured, presenter.snapshot.value.contextInspection.identity)

      presenter.dispatch(
          DesktopEvent.ProjectLoaded(project("other"), ProjectIndex("other", "revision")))
      assertEquals(
          ContextInspectionStatus.Closed, presenter.snapshot.value.contextInspection.status)
      assertNull(presenter.snapshot.value.contextInspection.identity)
      main.runPending()
      presenter.retryContextInspection()
      main.runPending()
      io.runPending()
      assertEquals(
          ContextInspectionStatus.Closed, presenter.snapshot.value.contextInspection.status)
      assertEquals(1, calls.get())
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun reindexClosesInspectionBeforeCapturedRevisionChanges() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val calls = AtomicInteger()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { method, path, _ ->
          when {
            method == "POST" && path.endsWith("/reindex") ->
                response("""{"project_id":"project","project_revision":"next"}""")
            method == "GET" && path.startsWith("/api/projects/current/context?") -> {
              calls.incrementAndGet()
              response("{}")
            }
            method == "GET" && (path.contains("/analysis/run?") || path.contains("/scan?")) ->
                TransportResponse(204, "")
            method == "GET" &&
                (path.contains("/overview?") ||
                    path.contains("/findings?") ||
                    path.contains("/analysis/selection?")) -> response("{}")
            else -> error("Unexpected $method $path")
          }
        }
    try {
      loadFile(presenter)
      presenter.inspectContext("fix")
      main.runPending()
      io.runPending()
      main.runPending()
      assertEquals(ContextInspectionStatus.Ready, presenter.snapshot.value.contextInspection.status)
      presenter.reindexProject()
      assertEquals(
          ContextInspectionStatus.Closed, presenter.snapshot.value.contextInspection.status)
      main.runPending()
      io.runPending()
      main.runPending()
      assertEquals("next", presenter.snapshot.value.state.project?.projectRevision)
      assertEquals(
          ContextInspectionStatus.Closed, presenter.snapshot.value.contextInspection.status)
      assertNull(presenter.snapshot.value.contextInspection.identity)
      presenter.retryContextInspection()
      main.runPending()
      io.runPending()
      assertEquals(1, calls.get())
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun inspectionInvalidatesOnlyWhenItsRelevantModelChanges() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    var function = "function-a"
    var bug = "bug-a"
    val calls = AtomicInteger()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { method, path, _ ->
          when (method to path) {
            "GET" to "/status" -> response("""{"status":"ok","version":"v1"}""")
            "GET" to "/api/models/current" ->
                response(
                    """{"scopes":{"function":{"scope":"function","model":"$function"},"bug":{"scope":"bug","model":"$bug"}}}""")
            "GET" to "/api/projects/current/context?path=main.go&action=fix" -> {
              calls.incrementAndGet()
              response("{}")
            }
            else -> error("Unexpected $method $path")
          }
        }
    fun drain() {
      main.runPending()
      io.runPending()
      main.runPending()
    }
    try {
      presenter.refreshConnection()
      drain()
      loadFile(presenter)
      presenter.inspectContext("fix")
      drain()
      val captured = presenter.snapshot.value.contextInspection.identity
      bug = "bug-b"
      presenter.refreshConnection()
      drain()
      assertEquals(ContextInspectionStatus.Ready, presenter.snapshot.value.contextInspection.status)
      function = "function-b"
      presenter.refreshConnection()
      drain()
      assertEquals(ContextInspectionStatus.Stale, presenter.snapshot.value.contextInspection.status)
      assertEquals(captured, presenter.snapshot.value.contextInspection.identity)
      presenter.retryContextInspection()
      drain()
      assertEquals(1, calls.get())
      presenter.inspectContext("fix")
      drain()
      assertEquals(ContextInspectionStatus.Ready, presenter.snapshot.value.contextInspection.status)
      assertEquals(2, calls.get())
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun checkRerunUsesItsOwnAttemptAndNeverAppliesAnOlderPass() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    var fail = false
    val calls = mutableListOf<String>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
          calls += path
          when {
            path.contains("/drafts/draft/checks") ->
                if (fail) TransportResponse(503, "")
                else
                    response(
                        """{"target_path":"main.go","applicable":true,"draft_id":"draft","draft_revision":1,"draft_hash":"draft-hash","checks":[]}""")
            else -> creationFileResponse(path) ?: error("Unexpected $path")
          }
        }
    try {
      loadProject(presenter)
      presenter.selectFile("main.go")
      main.runPending()
      io.runPending()
      main.runPending()
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      presenter.runDraftChecks()
      main.runPending()
      io.runPending()
      main.runPending()
      assertNull(presenter.snapshot.value.state.review.checkAttempt)
      assertTrue(presenter.snapshot.value.state.checks?.applicable == true)

      fail = true
      presenter.runDraftChecks()
      presenter.runDraftChecks()
      var state = presenter.snapshot.value.state
      assertEquals(ValidationAttemptStatus.Running, state.review.checkAttempt?.status)
      assertTrue(reviewToolWindowState(state).checksRunning)
      assertFalse(
          draftReviewEligibility(
                  state.review.editor,
                  state.review.draft,
                  state.checks,
                  state.selectedFile,
                  state.project,
                  state.review.checkAttempt)
              .eligible)
      presenter.applyEditableDraft()
      assertTrue(calls.none { it.contains("/apply") })
      main.runPending()
      io.runPending()
      main.runPending()
      state = presenter.snapshot.value.state
      assertEquals(ValidationAttemptStatus.Failed, state.review.checkAttempt?.status)
      assertTrue(state.review.checkAttempt?.message?.isNotBlank() == true)
      assertTrue(state.checks?.applicable == true)
      assertFalse(reviewToolWindowState(state).checksRunning)
      assertFalse(
          draftReviewEligibility(
                  state.review.editor,
                  state.review.draft,
                  state.checks,
                  state.selectedFile,
                  state.project,
                  state.review.checkAttempt)
              .eligible)
      presenter.applyEditableDraft()
      assertTrue(calls.none { it.contains("/apply") })
      assertEquals(2, calls.count { it.contains("/drafts/draft/checks") })

      presenter.runDraftChecks()
      presenter.cancelDraftValidation()
      state = presenter.snapshot.value.state
      assertEquals(ValidationAttemptStatus.Canceled, state.review.checkAttempt?.status)
      assertTrue(state.checks?.applicable == true)
      presenter.applyEditableDraft()
      assertTrue(calls.none { it.contains("/apply") })
      presenter.runDraftChecks()
      val replacement = presenter.snapshot.value.state.review.checkAttempt!!
      presenter.dispatch(
          DesktopEvent.ChecksStopped(
              replacement.requestId - 1, ValidationAttemptStatus.Canceled, "late cancellation"))
      assertEquals(replacement, presenter.snapshot.value.state.review.checkAttempt)
      presenter.dispatch(DesktopEvent.DraftEdited(declaration = "func Run() int { return 1 }"))
      main.runPending()
      io.runPending()
      main.runPending()
      assertNull(presenter.snapshot.value.state.review.checkAttempt)
      assertNull(presenter.snapshot.value.state.review.checks)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun validationTransportFailureAndCancelResolveOnlyTheCurrentDraftAttempt() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    var fail = true
    val calls = mutableListOf<String>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
          calls += path
          when {
            path.contains("/drafts/draft/validate") ->
                response(
                    kotlinx.serialization.json.Json.encodeToString(
                        DeclarationDraft.serializer(), draft().copy(revision = 2)))
            path.contains("/drafts/draft") ->
                if (fail) TransportResponse(503, "")
                else
                    response(
                        kotlinx.serialization.json.Json.encodeToString(
                            DeclarationDraft.serializer(),
                            draft().copy(revision = 2, validation = null)))
            else -> creationFileResponse(path) ?: error("Unexpected $path")
          }
        }
    try {
      loadProject(presenter)
      presenter.selectFile("main.go")
      main.runPending()
      io.runPending()
      main.runPending()
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      presenter.dispatch(DesktopEvent.DraftEdited(declaration = "func Run() { println(1) }"))
      presenter.validateEditableDraft()
      assertEquals(
          DraftEditorStatus.Validating, presenter.snapshot.value.state.review.editor?.status)
      main.runPending()
      io.runPending()
      main.runPending()
      val failed = presenter.snapshot.value.state.review.editor!!
      assertEquals(DraftEditorStatus.Dirty, failed.status)
      assertEquals(ValidationAttemptStatus.Failed, failed.validationAttempt?.status)
      assertTrue(failed.validationAttempt?.message?.isNotBlank() == true)
      assertEquals("func Run() { println(1) }", failed.declaration)
      assertNull(presenter.snapshot.value.state.review.draft?.validation)
      assertFalse(presenter.snapshot.value.draftValidationInProgress)

      fail = false
      presenter.validateEditableDraft()
      main.runPending()
      presenter.cancelDraftValidation()
      assertEquals(
          ValidationAttemptStatus.Canceled,
          presenter.snapshot.value.state.review.editor?.validationAttempt?.status)
      assertEquals(DraftEditorStatus.Dirty, presenter.snapshot.value.state.review.editor?.status)
      presenter.validateEditableDraft()
      val replacement = presenter.snapshot.value.state.review.editor?.validationAttempt
      assertEquals(ValidationAttemptStatus.Running, replacement?.status)
      main.runPending()
      io.runPending()
      main.runPending()
      io.runPending()
      main.runPending()
      assertEquals(DraftEditorStatus.Valid, presenter.snapshot.value.state.review.editor?.status)
      assertEquals(1, calls.count { it.contains("/drafts/draft/validate") })

      presenter.validateEditableDraft()
      main.runPending()
      io.runPending()
      main.runPending()
      presenter.dispatch(DesktopEvent.DraftEdited(declaration = "func Run() int { return 3 }"))
      io.runPending()
      main.runPending()
      assertEquals(DraftEditorStatus.Dirty, presenter.snapshot.value.state.review.editor?.status)
      assertEquals(
          "func Run() int { return 3 }", presenter.snapshot.value.state.review.editor?.declaration)
      assertNull(presenter.snapshot.value.state.review.draft?.validation)
      assertNull(presenter.snapshot.value.state.review.editor?.validationAttempt)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun validationFailureAfterUpdateRetainsRevisionForExplicitRetry() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    var failValidation = true
    val updates = mutableListOf<String>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { method, path, body ->
          when {
            method == "PATCH" && path.contains("/drafts/draft") -> {
              updates += body.orEmpty()
              val revision = if (updates.size == 1) 2L else 3L
              response(
                  kotlinx.serialization.json.Json.encodeToString(
                      DeclarationDraft.serializer(),
                      draft().copy(revision = revision, validation = null)))
            }
            path.contains("/drafts/draft/validate") ->
                if (failValidation) TransportResponse(503, "")
                else
                    response(
                        kotlinx.serialization.json.Json.encodeToString(
                            DeclarationDraft.serializer(), draft().copy(revision = 3)))
            else -> creationFileResponse(path) ?: error("Unexpected $method $path")
          }
        }
    try {
      loadProject(presenter)
      presenter.selectFile("main.go")
      main.runPending()
      io.runPending()
      main.runPending()
      val prior =
          draft()
              .copy(
                  validation =
                      DeclarationValidation(
                          true,
                          "strict_symbol",
                          diagnostics =
                              listOf(DeclarationFinding("old", "Prior validation warning")),
                          diff = UnifiedDiff("main.go", "main.go")))
      presenter.dispatch(DesktopEvent.DraftLoaded(prior))
      presenter.validateEditableDraft()
      presenter.validateEditableDraft()
      assertTrue(presenter.snapshot.value.state.review.editor?.diagnosticsAreRetained == true)
      assertNull(presenter.snapshot.value.state.review.draft?.validation)
      main.runPending()
      presenter.validateEditableDraft()
      io.runPending()
      main.runPending()
      presenter.validateEditableDraft()
      io.runPending()
      main.runPending()
      val failed = presenter.snapshot.value.state.review.editor!!
      assertEquals(1, updates.size)
      assertEquals("Prior validation warning", failed.diagnostics.single().message)
      assertTrue(failed.diagnosticsAreRetained)
      assertEquals(ValidationAttemptStatus.Failed, failed.validationAttempt?.status)
      assertEquals(2L, failed.serverDraft.revision)
      assertEquals("func Run() {}", failed.declaration)
      assertFalse(presenter.snapshot.value.draftValidationInProgress)
      failValidation = false
      presenter.validateEditableDraft()
      main.runPending()
      presenter.cancelDraftValidation()
      val canceled = presenter.snapshot.value.state.review.editor!!
      assertEquals(ValidationAttemptStatus.Canceled, canceled.validationAttempt?.status)
      assertEquals("Prior validation warning", canceled.diagnostics.single().message)
      assertNull(presenter.snapshot.value.state.review.draft?.validation)
      presenter.validateEditableDraft()
      main.runPending()
      io.runPending()
      main.runPending()
      io.runPending()
      main.runPending()
      assertTrue(updates.last().contains("\"expected_revision\":2"))
      assertEquals(2, updates.size)
      assertEquals(DraftEditorStatus.Valid, presenter.snapshot.value.state.review.editor?.status)
      assertFalse(presenter.snapshot.value.state.review.editor!!.diagnosticsAreRetained)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun startupWithoutARememberedProjectOnlyChecksTheConnection() {
    val dispatcher = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    val calls = mutableListOf<Pair<String, String>>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = dispatcher) { method, path, _ ->
          calls.add(method to path)
          projectStartupResponse(method, path)
        }
    try {
      presenter.start()
      dispatcher.runPending()

      assertNull(presenter.snapshot.value.state.project)
      assertNull(presenter.snapshot.value.state.error)
      assertEquals(listOf("GET" to "/status", "GET" to "/api/models/current"), calls)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun startupRestoresTheLastSuccessfulProjectAcrossPresenterAndStoreInstances() {
    val preferences = InMemoryPreferences()
    val dispatcher = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    val first =
        presenter(
            parentScope = scope,
            ioDispatcher = dispatcher,
            lastProjectStore = LastProjectStore(preferences)) { method, path, _ ->
              projectStartupResponse(method, path)
            }
    val calls = mutableListOf<Pair<String, String>>()
    val reopened =
        presenter(
            parentScope = scope,
            ioDispatcher = dispatcher,
            lastProjectStore = LastProjectStore(preferences)) { method, path, body ->
              calls.add(method to path)
              if (path == "/api/projects/restore")
                  assertEquals("""{"project_path":"/tmp/project"}""", body)
              projectStartupResponse(method, path)
            }
    try {
      first.loadProject("/tmp/project", restore = false)
      dispatcher.runPending()
      assertEquals("/tmp/project", first.snapshot.value.state.project?.path)
      first.close()

      reopened.start()
      dispatcher.runPending()

      val state = reopened.snapshot.value.state
      assertEquals("/tmp/project", state.project?.path)
      assertEquals("/tmp/project", state.projectState.rememberedPath)
      assertEquals("project", state.index?.projectId)
      assertEquals("Reopened project", state.status)
      assertFalse(state.loading)
      assertNull(state.error)
      assertEquals(listOf("POST" to "/api/projects/restore"), calls.filter { it.first != "GET" })
    } finally {
      first.close()
      reopened.close()
      scope.cancel()
    }
  }

  @Test
  fun failedRestorePreservesTheRememberedProjectWithoutImportingOrCallingAModel() {
    val preferences = InMemoryPreferences()
    val store = LastProjectStore(preferences)
    store.save("/tmp/missing-project")
    val dispatcher = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    val calls = mutableListOf<Pair<String, String>>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = dispatcher, lastProjectStore = store) {
            method,
            path,
            _ ->
          calls.add(method to path)
          if (path == "/api/projects/restore")
              TransportResponse(400, """{"message":"project directory does not exist"}""")
          else projectStartupResponse(method, path)
        }
    try {
      presenter.start()
      dispatcher.runPending()

      val state = presenter.snapshot.value.state
      assertNull(state.project)
      assertFalse(state.loading)
      assertTrue(state.error.orEmpty().contains("project directory does not exist"))
      assertEquals(state.error, state.projectState.openingError)
      assertEquals("/tmp/missing-project", state.projectState.rememberedPath)
      presenter.dispatch(DesktopEvent.Failed("Unrelated failure"))
      assertEquals(
          state.projectState.openingError, presenter.snapshot.value.state.projectState.openingError)
      assertEquals("/tmp/missing-project", LastProjectStore(preferences).load())
      assertEquals(1, preferences.flushCount)
      assertEquals(listOf("POST" to "/api/projects/restore"), calls.filter { it.first != "GET" })
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun retryRestoresOnlyTheFailedPathOnceWithoutStartingOtherWork() {
    val dispatcher = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    val store = LastProjectStore(InMemoryPreferences()).also { it.save("/tmp/remembered") }
    val calls = mutableListOf<Triple<String, String, String?>>()
    var restoreFails = true
    val presenter =
        presenter(parentScope = scope, ioDispatcher = dispatcher, lastProjectStore = store) {
            method,
            path,
            body ->
          calls += Triple(method, path, body)
          if (path == "/api/projects/restore" && restoreFails)
              TransportResponse(404, """{"message":"folder unavailable"}""")
          else projectStartupResponse(method, path)
        }
    try {
      presenter.retryProjectRestore() // No failed restore exists.
      presenter.start()
      dispatcher.runPending()
      val failed = presenter.snapshot.value.state.projectState.openingAttempt!!
      assertEquals(ProjectOpeningKind.Restore, failed.kind)
      assertEquals("/tmp/remembered", failed.path)
      assertEquals(ProjectOpeningOutcome.Failed("folder unavailable"), failed.outcome)
      assertEquals("/tmp/remembered", store.load())

      restoreFails = false
      presenter.retryProjectRestore()
      val pending = presenter.snapshot.value.state.projectState.openingAttempt!!
      assertTrue(pending.requestId > failed.requestId)
      assertEquals(ProjectOpeningOutcome.Opening, pending.outcome)
      presenter.retryProjectRestore() // The first retry is still pending.
      dispatcher.runPending()
      assertEquals("/tmp/project", presenter.snapshot.value.state.project?.path)
      assertNull(presenter.snapshot.value.state.projectState.openingAttempt)
      presenter.retryProjectRestore() // Success is not retryable.
      dispatcher.runPending()
      assertEquals(2, calls.count { it.first == "POST" && it.second == "/api/projects/restore" })
      assertTrue(
          calls.all { (method, path, _) ->
            method == "GET" || (method == "POST" && path == "/api/projects/restore")
          },
          "Restore must not import, generate, start/resume analysis, execute, mutate or start a terminal: $calls")
      assertEquals(
          listOf(
              """{"project_path":"/tmp/remembered"}""", """{"project_path":"/tmp/remembered"}"""),
          calls.filter { it.second == "/api/projects/restore" }.map { it.third })
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun failedImportAndSupersededRestoreCannotBeRetried() {
    val dispatcher = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    val calls = mutableListOf<Pair<String, String>>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = dispatcher) { method, path, _ ->
          calls += method to path
          when (path) {
            "/api/projects/restore" -> TransportResponse(404, """{"message":"missing"}""")
            "/api/projects/import" -> throw IllegalStateException(" ")
            else -> projectStartupResponse(method, path)
          }
        }
    try {
      presenter.loadProject("/tmp/old", restore = true)
      dispatcher.runPending()
      assertEquals(
          ProjectOpeningOutcome.Failed("missing"),
          presenter.snapshot.value.state.projectState.openingAttempt?.outcome)
      presenter.loadProject("/tmp/new", restore = false)
      assertEquals(
          ProjectOpeningOutcome.Opening,
          presenter.snapshot.value.state.projectState.openingAttempt?.outcome)
      // Cannot replay the superseded failure while import is pending.
      presenter.retryProjectRestore()
      dispatcher.runPending()
      val failed = presenter.snapshot.value.state.projectState.openingAttempt!!
      assertEquals(ProjectOpeningKind.Import, failed.kind)
      assertEquals(ProjectOpeningOutcome.Failed("Import failed"), failed.outcome)
      presenter.retryProjectRestore()
      presenter.refreshConnection() // Reconnect is not an implicit retry.
      dispatcher.runPending()
      assertEquals(1, calls.count { it == "POST" to "/api/projects/restore" })
      assertEquals(1, calls.count { it == "POST" to "/api/projects/import" })
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun indexReadFailureAndCurrentMismatchRemainRetryableButLateSamePathWorkIsIgnored() {
    for (failure in listOf("read", "mismatch")) {
      val main = QueuedDispatcher()
      val io = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + main)
      var indexFails = true
      val calls = mutableListOf<Pair<String, String>>()
      val presenter =
          presenter(parentScope = scope, ioDispatcher = io) { method, path, _ ->
            calls += method to path
            when (path) {
              "/api/projects/current/index" ->
                  if (indexFails && failure == "read") throw IllegalStateException("index offline")
                  else if (indexFails) response(indexJson().replace(":\"revision\"", ":\"other\""))
                  else response(indexJson())
              else -> projectStartupResponse(method, path)
            }
          }
      try {
        presenter.loadProject("/tmp/project", restore = true)
        main.runPending()
        io.runPending()
        main.runPending()
        val failed = presenter.snapshot.value.state.projectState.openingAttempt!!
        assertEquals(ProjectOpeningKind.Restore, failed.kind)
        val outcome = failed.outcome
        assertTrue(outcome is ProjectOpeningOutcome.Failed)
        assertTrue(
            outcome.message.contains(if (failure == "read") "index offline" else "do not match"),
            "Unexpected $failure outcome: $outcome")
        assertNull(presenter.snapshot.value.state.project)

        indexFails = false
        presenter.retryProjectRestore()
        val retry = presenter.snapshot.value.state.projectState.openingAttempt!!
        assertTrue(retry.requestId > failed.requestId)
        main.runPending()
        io.runPending() // Retry has returned from I/O; its completion is queued on main.
        presenter.loadProject("/tmp/project", restore = true)
        val newest = presenter.snapshot.value.state.projectState.openingAttempt!!
        main.runPending()
        assertEquals(newest, presenter.snapshot.value.state.projectState.openingAttempt)
        io.runPending()
        main.runPending()
        assertNull(presenter.snapshot.value.state.projectState.openingAttempt)
        assertEquals("/tmp/project", presenter.snapshot.value.state.project?.path)
        assertTrue(newest.requestId > retry.requestId)
        assertEquals(3, calls.count { it == "POST" to "/api/projects/restore" })
      } finally {
        presenter.close()
        scope.cancel()
      }
    }
  }

  @Test
  fun blankRestoreFailureUsesRestoreFallbackAndCancellationDoesNotBecomeFailure() {
    val dispatcher = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    var cancel = false
    val calls = mutableListOf<String>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = dispatcher) { _, path, _ ->
          calls += path
          if (path == "/api/projects/restore") {
            if (cancel) throw kotlinx.coroutines.CancellationException("request canceled")
            throw IllegalStateException(" ")
          }
          projectStartupResponse("GET", path)
        }
    try {
      presenter.loadProject("/tmp/remembered", restore = true)
      dispatcher.runPending()
      assertEquals(
          ProjectOpeningOutcome.Failed("Could not restore project"),
          presenter.snapshot.value.state.projectState.openingAttempt?.outcome)
      cancel = true
      presenter.retryProjectRestore()
      dispatcher.runPending()
      assertEquals(
          ProjectOpeningOutcome.Canceled,
          presenter.snapshot.value.state.projectState.openingAttempt?.outcome)
      presenter.retryProjectRestore()
      dispatcher.runPending()
      assertEquals(2, calls.count { it == "/api/projects/restore" })
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun startupReadsPreferencesOffThePresentationDispatcherAndRestoresOnlyOnce() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    var reads = 0
    val preferences =
        ControlledProjectPreferences(
            read = {
              reads++
              "/tmp/project"
            })
    val calls = mutableListOf<Pair<String, String>>()
    val presenter =
        presenter(
            parentScope = scope,
            ioDispatcher = io,
            lastProjectStore = LastProjectStore(preferences)) { method, path, _ ->
              calls += method to path
              projectStartupResponse(method, path)
            }
    try {
      presenter.start()
      presenter.start()
      assertEquals(0, reads)
      main.runPending()
      assertEquals(0, reads)
      assertNull(presenter.snapshot.value.state.projectState.rememberedPath)
      io.runPending()
      assertEquals(1, reads)
      main.runPending()
      assertEquals("/tmp/project", presenter.snapshot.value.state.projectState.rememberedPath)
      assertNull(presenter.snapshot.value.state.projectState.preferenceReadWarning)
      repeat(5) {
        io.runPending()
        main.runPending()
      }
      assertEquals("/tmp/project", presenter.snapshot.value.state.project?.path)
      assertEquals(Workspace.Summary, presenter.snapshot.value.state.workspace)
      assertEquals(1, calls.count { it == "POST" to "/api/projects/restore" })
      presenter.start()
      main.runPending()
      io.runPending()
      assertEquals(1, reads)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun blankStartupPreferenceNeverRequestsRestore() {
    for (value in listOf(null, "  ")) {
      val main = QueuedDispatcher()
      val io = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + main)
      val calls = mutableListOf<Pair<String, String>>()
      val presenter =
          presenter(
              parentScope = scope,
              ioDispatcher = io,
              lastProjectStore =
                  LastProjectStore(ControlledProjectPreferences(read = { value }))) {
                  method,
                  path,
                  _ ->
                calls += method to path
                projectStartupResponse(method, path)
              }
      try {
        presenter.start()
        repeat(3) {
          main.runPending()
          io.runPending()
        }
        main.runPending()
        assertNull(presenter.snapshot.value.state.projectState.rememberedPath)
        assertNull(presenter.snapshot.value.state.projectState.preferenceReadWarning)
        assertNull(presenter.snapshot.value.state.project)
        assertTrue(calls.none { it.first == "POST" })
      } finally {
        presenter.close()
        scope.cancel()
      }
    }
  }

  @Test
  fun preferenceReadFailureIsLocalAndDoesNotPreventExplicitOpen() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val calls = mutableListOf<Pair<String, String>>()
    val presenter =
        presenter(
            parentScope = scope,
            ioDispatcher = io,
            lastProjectStore =
                LastProjectStore(
                    ControlledProjectPreferences({ null }, syncFailure = "storage denied"))) {
                method,
                path,
                _ ->
              calls += method to path
              projectStartupResponse(method, path)
            }
    try {
      presenter.start()
      main.runPending()
      io.runPending()
      main.runPending()
      val state = presenter.snapshot.value.state
      assertNull(state.projectState.rememberedPath)
      assertTrue(state.projectState.preferenceReadWarning.orEmpty().contains("storage denied"))
      assertNull(state.error)
      assertTrue(calls.none { it.first == "POST" })
      presenter.loadProject("/tmp/project", restore = false)
      repeat(6) {
        main.runPending()
        io.runPending()
      }
      assertEquals("/tmp/project", presenter.snapshot.value.state.project?.path)
      assertEquals(Workspace.Summary, presenter.snapshot.value.state.workspace)
      assertEquals(
          state.projectState.preferenceReadWarning,
          presenter.snapshot.value.state.projectState.preferenceReadWarning)
      assertEquals(listOf("POST" to "/api/projects/import"), calls.filter { it.first == "POST" })
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun delayedStartupReadCannotPublishOrRestoreOverAnExplicitOpen() {
    for (readFailure in listOf(false, true)) {
      val main = QueuedDispatcher()
      val io = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + main)
      val calls = mutableListOf<Pair<String, String>>()
      val presenter =
          presenter(
              parentScope = scope,
              ioDispatcher = io,
              lastProjectStore =
                  LastProjectStore(
                      ControlledProjectPreferences(
                          { "/tmp/obsolete" },
                          syncFailure = if (readFailure) "late storage failure" else null))) {
                  method,
                  path,
                  _ ->
                calls += method to path
                projectStartupResponse(method, path)
              }
      try {
        presenter.start()
        main.runPending() // Queue the preference read on I/O.
        presenter.loadProject("/tmp/project", restore = false)
        main.runPending()
        io.runPending() // Complete the preference read after the chooser-based open began.
        main.runPending()
        repeat(5) {
          io.runPending()
          main.runPending()
        }
        val state = presenter.snapshot.value.state
        assertEquals("/tmp/project", state.project?.path)
        assertEquals("/tmp/project", state.projectState.rememberedPath)
        assertNull(state.projectState.preferenceReadWarning)
        assertEquals(listOf("POST" to "/api/projects/import"), calls.filter { it.first == "POST" })
      } finally {
        presenter.close()
        scope.cancel()
      }
    }
  }

  @Test
  fun canceledPreferenceReadDoesNotBecomeAStorageWarningOrRestore() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val calls = mutableListOf<Pair<String, String>>()
    val presenter =
        presenter(
            parentScope = scope,
            ioDispatcher = io,
            lastProjectStore =
                LastProjectStore(
                    ControlledProjectPreferences(
                        { null },
                        onSync = { throw kotlinx.coroutines.CancellationException() }))) {
                method,
                path,
                _ ->
              calls += method to path
              projectStartupResponse(method, path)
            }
    try {
      presenter.start()
      repeat(3) {
        main.runPending()
        io.runPending()
      }
      assertNull(presenter.snapshot.value.state.projectState.preferenceReadWarning)
      assertNull(presenter.snapshot.value.state.projectState.rememberedPath)
      assertTrue(calls.none { it.first == "POST" })
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun acceptedProjectsAreSavedOffThePresentationDispatcherInAcceptanceOrder() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val preferences = InMemoryPreferences()
    val store = LastProjectStore(preferences)
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io, lastProjectStore = store) {
            method,
            path,
            body ->
          if (method == "POST") {
            val returnedPath = if (body.orEmpty().contains("alias")) "/tmp/first" else "/tmp/second"
            response(projectJson().replace("/tmp/project", returnedPath))
          } else projectStartupResponse(method, path)
        }
    try {
      presenter.loadProject("/tmp/alias", restore = false)
      main.runPending()
      io.runPending()
      main.runPending()
      assertEquals("/tmp/first", presenter.snapshot.value.state.project?.path)
      assertNull(store.load()) // The accepted first save is still queued on I/O.

      presenter.loadProject("/tmp/second", restore = false)
      main.runPending()
      io.runLast() // Complete the newer API response while the older save remains queued.
      main.runPending()
      assertEquals("/tmp/second", presenter.snapshot.value.state.project?.path)
      assertEquals(0, preferences.flushCount)
      io.runNext() // Complete the older save after the newer project was accepted.
      main.runPending()
      assertEquals(1, preferences.flushCount)
      assertNull(presenter.snapshot.value.state.projectState.rememberedPath)
      repeat(6) {
        io.runPending()
        main.runPending()
      }
      assertEquals("/tmp/second", store.load())
      assertEquals("/tmp/second", presenter.snapshot.value.state.projectState.rememberedPath)
      assertEquals(2, preferences.flushCount)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun rejectedAndSupersededResponsesNeverWritePreferences() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val preferences = InMemoryPreferences()
    var mismatch = true
    val presenter =
        presenter(
            parentScope = scope,
            ioDispatcher = io,
            lastProjectStore = LastProjectStore(preferences)) { method, path, _ ->
              when (method to path) {
                "POST" to "/api/projects/import" -> response(projectJson())
                "GET" to "/api/projects/current/index" ->
                    if (mismatch) response(indexJson().replace(":\"revision\"", ":\"wrong\""))
                    else response(indexJson())
                else -> projectStartupResponse(method, path)
              }
            }
    try {
      presenter.loadProject("/tmp/obsolete", restore = false)
      main.runPending() // Old request is queued on I/O.
      presenter.loadProject("/tmp/project", restore = false)
      main.runPending()
      io.runPending()
      main.runPending()
      io.runPending()
      main.runPending()
      assertNull(presenter.snapshot.value.state.project)
      assertTrue(
          presenter.snapshot.value.state.projectState.openingError
              .orEmpty()
              .contains("do not match"),
          "Opening state: ${presenter.snapshot.value.state.projectState.openingAttempt}")
      assertEquals(0, preferences.flushCount)
      mismatch = false
      presenter.loadProject("/tmp/project", restore = false)
      repeat(5) {
        main.runPending()
        io.runPending()
      }
      assertEquals("/tmp/project", presenter.snapshot.value.state.project?.path)
      assertEquals(1, preferences.flushCount)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun failedSaveDoesNotUndoAnOpenOrSkipWorkspaceRefresh() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val calls = mutableListOf<String>()
    val preferences =
        object : AbstractPreferences(null, "") {
          override fun putSpi(key: String, value: String) = Unit

          override fun getSpi(key: String): String? = null

          override fun removeSpi(key: String) = Unit

          override fun removeNodeSpi() = Unit

          override fun keysSpi(): Array<String> = emptyArray()

          override fun childrenNamesSpi(): Array<String> = emptyArray()

          override fun childSpi(name: String): AbstractPreferences = error("Unexpected child")

          override fun syncSpi() = Unit

          override fun flushSpi(): Unit = throw BackingStoreException("disk denied")
        }
    val presenter =
        presenter(
            parentScope = scope,
            ioDispatcher = io,
            lastProjectStore = LastProjectStore(preferences)) { method, path, _ ->
              calls += path
              projectStartupResponse(method, path)
            }
    try {
      presenter.loadProject("/tmp/project", restore = false)
      main.runPending()
      io.runPending()
      main.runPending()
      assertEquals("/tmp/project", presenter.snapshot.value.state.project?.path)
      assertNull(presenter.snapshot.value.state.projectState.preferenceSaveWarning)
      repeat(6) {
        io.runPending()
        main.runPending()
      }
      val state = presenter.snapshot.value.state
      assertEquals(Workspace.Summary, state.workspace)
      assertEquals("/tmp/project", state.project?.path)
      assertNull(state.projectState.openingError)
      assertNull(state.error)
      assertNull(state.projectState.rememberedPath)
      assertTrue(state.projectState.preferenceSaveWarning.orEmpty().contains("next launch"))
      assertTrue(state.projectState.preferenceSaveWarning.orEmpty().contains("disk denied"))
      assertTrue(calls.any { it.contains("/overview?") })
      assertTrue(calls.any { it.contains("/findings?") })
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun failedFileReadRetainsItsOwnErrorAfterAnUnrelatedFailure() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val calls = mutableListOf<String>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
          calls.add(path)
          TransportResponse(403, """{"message":"File read denied"}""")
        }
    try {
      loadProject(presenter)
      presenter.selectFile("main.go")
      repeat(3) {
        main.runPending()
        io.runPending()
      }
      assertEquals("File read denied", presenter.snapshot.value.state.selection.fileReadError)
      presenter.dispatch(DesktopEvent.Failed("Unrelated analysis failed"))
      val state = presenter.snapshot.value.state
      assertEquals("File read denied", state.selection.fileReadError)
      assertEquals("Unrelated analysis failed", state.error)
      assertEquals(1, calls.size)
      assertTrue(calls.single().contains("files/info?"))
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun passiveFileReadsAndInspectionDoNotSaveSelectionSendWorkOrWriteProjectSource() {
    val directory = Files.createTempDirectory("mini-orca-passive-navigation-")
    val source = directory.resolve("empty.go")
    val original = "package example\n"
    Files.writeString(source, original)
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val calls = mutableListOf<Pair<String, String>>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { method, path, _ ->
          calls += method to path
          when {
            path.contains("files/info?path=empty.go") ->
                response(
                    Json.encodeToString(
                        file()
                            .copy(
                                path = "empty.go",
                                name = "empty.go",
                                content = Files.readString(source))))
            path.contains("files/symbols?path=empty.go") -> response(symbolsJson("empty.go"))
            else -> response("{}")
          }
        }
    try {
      presenter.dispatch(
          DesktopEvent.ProjectLoaded(
              project().copy(path = directory.toString()),
              ProjectIndex(
                  "project",
                  "revision",
                  files = listOf(IndexedFile("empty.go", "base", "Go", false)))))
      presenter.openFileInEditor("missing.go")
      assertTrue(calls.isEmpty())
      presenter.openFileInEditor("empty.go")
      repeat(6) {
        main.runPending()
        io.runPending()
      }
      val loaded = presenter.snapshot.value.state
      assertEquals("empty.go", loaded.selectedFile?.path)
      assertEquals(original, loaded.selectedFile?.content)
      assertTrue(loaded.symbols.isEmpty())
      assertEquals("package example\n", Files.readString(source))
      assertEquals(1, calls.count { it.second.contains("files/info?path=empty.go") })
      assertEquals(1, calls.count { it.second.contains("files/symbols?path=empty.go") })
      val readCount = calls.size
      presenter.dispatch(DesktopEvent.SourceLineSelected(SourceLineSelection(1, null)))
      presenter.openFileInEditor("empty.go", EditorNavigationTarget("empty.go", line = 1))
      main.runPending()
      io.runPending()
      assertEquals(readCount, calls.size)
      assertNull(presenter.snapshot.value.state.selectedSymbol)
      assertEquals(1, presenter.snapshot.value.state.selection.focusedLine)
      assertTrue(calls.all { (method, _) -> method == "GET" })
      assertTrue(
          calls.all { (_, path) ->
            listOf("/files/info?", "/files/symbols?", "/files/analysis?", "/impact?", "/git?")
                .any(path::contains)
          },
          "Only file inspection and read-only enrichment are permitted: $calls")
      assertEquals(original, Files.readString(source))
    } finally {
      presenter.close()
      scope.cancel()
      Files.deleteIfExists(source)
      Files.deleteIfExists(directory)
    }
  }

  @Test
  fun ordinaryFileOpeningWithoutWorkLoadsAndAllowsOnlyAnIndexedMatchingTarget() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val calls = mutableListOf<String>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
          calls += path
          when {
            path.contains("files/info?") -> response(fileJson("other.go", "other"))
            path.contains("files/symbols?") ->
                response(symbolsJson("other.go", "Other", start = 2, end = 5))
            else -> response("{}")
          }
        }
    try {
      loadFile(presenter)
      presenter.openFileInEditor("missing.go")
      presenter.openFileInEditor("other.go", EditorNavigationTarget("main.go", line = 3))
      main.runPending()
      io.runPending()
      assertTrue(calls.isEmpty())
      presenter.openFileInEditor("other.go", EditorNavigationTarget("other.go", line = 3))
      assertEquals("main.go", presenter.snapshot.value.state.selectedFile?.path)
      repeat(5) {
        main.runPending()
        io.runPending()
      }
      main.runPending()
      assertEquals("other.go", presenter.snapshot.value.state.selectedFile?.path)
      assertEquals("Other", presenter.snapshot.value.state.selectedSymbol?.name)
      assertEquals(3, presenter.snapshot.value.state.selection.focusedLine)
      assertEquals(1, calls.count { it.contains("files/info?") })
      assertTrue(
          calls.all {
            it.contains("files/info?") ||
                it.contains("files/symbols?") ||
                it.contains("files/analysis?") ||
                it.contains("/impact?") ||
                it.contains("/git?")
          })
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun fileRoutingConfirmsProtectedWorkAndClearsComposerOnlyAfterReplacement() {
    for (work in listOf("none", "session", "draft", "message", "constraints")) {
      val main = QueuedDispatcher()
      val io = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + main)
      val calls = mutableListOf<String>()
      val presenter =
          presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
            calls += path
            when {
              path.contains("files/info?") -> response(fileJson("other.go", "other"))
              path.contains("files/symbols?") -> response(symbolsJson("other.go"))
              else -> response("{}")
            }
          }
      try {
        loadFile(presenter)
        if (work == "session")
            presenter.dispatch(
                DesktopEvent.ChatLoaded(
                    ChatSession("session", "project", "revision", "base", "main.go")))
        if (work == "draft") presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
        var message = TextFieldValue(if (work == "message") "keep message" else "")
        var constraints = TextFieldValue(if (work == "constraints") "keep constraints" else "")
        var pending: PendingDraftDiscard.FileNavigation? = null
        var clears = 0
        val input = { message to constraints }
        val clear = {
          clears++
          message = TextFieldValue()
          constraints = TextFieldValue()
        }
        val previous = presenter.snapshot.value.state
        routeFileNavigationRequest(presenter, "main.go", message, constraints, input, clear) {
          pending = it
        }
        assertNull(pending, work)
        assertEquals(previous.selection, presenter.snapshot.value.state.selection, work)
        assertEquals(0, clears, work)
        routeFileNavigationRequest(presenter, "other.go", message, constraints, input, clear) {
          pending = it
        }
        if (work != "none") {
          main.runPending()
          io.runPending()
          main.runPending()
          assertTrue(calls.isEmpty(), work)
          assertEquals(previous.selection, presenter.snapshot.value.state.selection, work)
          assertEquals(previous.review, presenter.snapshot.value.state.review, work)
          assertEquals(previous.chat, presenter.snapshot.value.state.chat, work)
          confirmFileNavigationDiscard(presenter, requireNotNull(pending), input, clear)
          // A repeated dialog callback cannot admit a second replacement.
          confirmFileNavigationDiscard(presenter, requireNotNull(pending), input, clear)
        }
        assertEquals(previous.selectedFile, presenter.snapshot.value.state.selectedFile, work)
        assertEquals(previous.review, presenter.snapshot.value.state.review, work)
        assertEquals(previous.chat, presenter.snapshot.value.state.chat, work)
        assertEquals(0, clears, work)
        repeat(5) {
          main.runPending()
          io.runPending()
        }
        main.runPending()
        assertEquals("other.go", presenter.snapshot.value.state.selectedFile?.path, work)
        assertNull(presenter.snapshot.value.state.review.editor, work)
        assertNull(presenter.snapshot.value.state.chat.session, work)
        assertEquals(TextFieldValue() to TextFieldValue(), input(), work)
        assertEquals(1, clears, work)
        assertEquals(1, calls.count { it.contains("files/info?") }, work)
      } finally {
        presenter.close()
        scope.cancel()
      }
    }
  }

  @Test
  fun fileRoutingRejectsChangedTextFieldValuesBeforeConfirmationAndPublication() {
    for (timing in listOf("dialog", "read")) {
      for (change in
          listOf("message", "constraints", "message selection", "constraints composition")) {
        val main = QueuedDispatcher()
        val io = QueuedDispatcher()
        val scope = CoroutineScope(SupervisorJob() + main)
        val calls = mutableListOf<String>()
        val presenter =
            presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
              calls += path
              when {
                path.contains("files/info?") -> response(fileJson("other.go", "other"))
                path.contains("files/symbols?") -> response(symbolsJson("other.go"))
                else -> response("{}")
              }
            }
        try {
          loadFile(presenter)
          presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
          presenter.dispatch(
              DesktopEvent.ChatLoaded(
                  ChatSession("session", "project", "revision", "base", "main.go")))
          val previous = presenter.snapshot.value.state
          var message = TextFieldValue("keep message")
          var constraints = TextFieldValue("keep constraints")
          var pending: PendingDraftDiscard.FileNavigation? = null
          var clears = 0
          val input = { message to constraints }
          routeFileNavigationRequest(
              presenter, "other.go", message, constraints, input, { clears++ }) {
                pending = it
              }
          val approval = requireNotNull(pending)
          if (timing == "read") {
            confirmFileNavigationDiscard(presenter, approval, input) { clears++ }
            main.runPending()
            io.runPending() // Hold matching publication until the input changes.
          }
          when (change) {
            "message" -> message = TextFieldValue("new message")
            "constraints" -> constraints = TextFieldValue("new constraints")
            "message selection" -> message = message.copy(selection = TextRange(1, 3))
            "constraints composition" ->
                constraints = constraints.copy(composition = TextRange(1, 3))
          }
          val newerInput = input()
          if (timing == "dialog")
              confirmFileNavigationDiscard(presenter, approval, input) { clears++ }
          repeat(5) {
            main.runPending()
            io.runPending()
          }
          main.runPending()
          assertEquals(previous.selectedFile, presenter.snapshot.value.state.selectedFile, change)
          assertEquals(previous.symbols, presenter.snapshot.value.state.symbols, change)
          assertEquals(
              previous.selection.selectedSymbol,
              presenter.snapshot.value.state.selectedSymbol,
              change)
          assertEquals(previous.chat, presenter.snapshot.value.state.chat, change)
          assertEquals(previous.review, presenter.snapshot.value.state.review, change)
          assertEquals(newerInput, input(), change)
          assertEquals(0, clears, change)
          assertNull(presenter.snapshot.value.state.selection.pendingFilePath, change)
          if (timing == "dialog") assertTrue(calls.isEmpty(), change)
          confirmFileNavigationDiscard(presenter, approval, input) { clears++ }
          assertEquals(0, clears, change)
        } finally {
          presenter.close()
          scope.cancel()
        }
      }
    }
  }

  @Test
  fun failedFileRoutingPreservesComposerAndLoadedWork() {
    val dispatcher = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    val presenter =
        presenter(parentScope = scope, ioDispatcher = dispatcher) { _, _, _ ->
          TransportResponse(503, "")
        }
    try {
      loadFile(presenter)
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      val previous = presenter.snapshot.value.state
      val message = TextFieldValue("keep message", TextRange(1, 4))
      val constraints = TextFieldValue("keep constraints", composition = TextRange(0, 4))
      var clears = 0
      var pending: PendingDraftDiscard.FileNavigation? = null
      val input = { message to constraints }
      routeFileNavigationRequest(presenter, "other.go", message, constraints, input, { clears++ }) {
        pending = it
      }
      confirmFileNavigationDiscard(presenter, requireNotNull(pending), input) { clears++ }
      dispatcher.runPending()
      assertEquals(previous.selectedFile, presenter.snapshot.value.state.selectedFile)
      assertEquals(previous.review, presenter.snapshot.value.state.review)
      assertEquals(previous.chat, presenter.snapshot.value.state.chat)
      assertEquals(previous.selection.selectedSymbol, presenter.snapshot.value.state.selectedSymbol)
      assertEquals("other.go", presenter.snapshot.value.state.selection.failedFilePath)
      assertEquals(0, clears)
      assertEquals(message to constraints, input())
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun failedReplacementRetryRequiresFreshAdmissionAndLeavesWorkUntilSuccess() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val calls = mutableListOf<String>()
    var denyRead = true
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
          calls += path
          when {
            path.contains("files/info?path=other.go") && denyRead -> TransportResponse(503, "")
            path.contains("files/info?path=other.go") -> response(fileJson("other.go", "other"))
            path.contains("files/symbols?path=other.go") -> response(symbolsJson("other.go"))
            else -> response("{}")
          }
        }
    fun drain() {
      repeat(5) {
        main.runPending()
        io.runPending()
      }
      main.runPending()
    }
    try {
      loadFile(presenter)
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      val previous = presenter.snapshot.value.state
      var message = TextFieldValue("keep input")
      val input = { message to TextFieldValue() }
      var clears = 0
      var pending: PendingDraftDiscard.FileNavigation? = null
      fun route() =
          routeFileNavigationRequest(
              presenter,
              "other.go",
              message,
              TextFieldValue(),
              input,
              {
                clears++
                message = TextFieldValue()
              }) {
                pending = it
              }
      route()
      val failedApproval = requireNotNull(pending)
      confirmFileNavigationDiscard(presenter, failedApproval, input) { clears++ }
      drain()
      assertEquals("other.go", presenter.snapshot.value.state.selection.failedFilePath)
      assertEquals(previous.selectedFile, presenter.snapshot.value.state.selectedFile)
      assertEquals(previous.review, presenter.snapshot.value.state.review)
      assertEquals("keep input", message.text)
      assertEquals(0, clears)
      assertEquals(1, calls.count { it.contains("files/info?") })

      denyRead = false
      confirmFileNavigationDiscard(presenter, failedApproval, input) { clears++ }
      presenter.openFileInEditor("other.go") // Direct entry cannot bypass the draft guard.
      drain()
      assertEquals(1, calls.count { it.contains("files/info?") })
      assertEquals(previous.review, presenter.snapshot.value.state.review)
      assertEquals("keep input", message.text)

      pending = null
      route() // Dismissing the fresh dialog does not start a read.
      assertTrue(pending != null)
      drain()
      assertEquals(1, calls.count { it.contains("files/info?") })
      route()
      confirmFileNavigationDiscard(presenter, requireNotNull(pending), input) {
        clears++
        message = TextFieldValue()
      }
      assertEquals(previous.review, presenter.snapshot.value.state.review)
      drain()
      assertEquals("other.go", presenter.snapshot.value.state.selectedFile?.path)
      assertNull(presenter.snapshot.value.state.selection.failedFilePath)
      assertNull(presenter.snapshot.value.state.review.editor)
      assertEquals("", message.text)
      assertEquals(1, clears)
      assertEquals(2, calls.count { it.contains("files/info?") })
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun ordinaryFileOpeningRejectsProtectedWorkAtBothPresenterEntryPoints() {
    for (work in listOf("session", "draft", "edited buffer")) {
      val main = QueuedDispatcher()
      val io = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + main)
      val calls = mutableListOf<String>()
      val presenter =
          presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
            calls += path
            error("No read is admitted")
          }
      try {
        loadFile(presenter)
        if (work == "session") {
          presenter.dispatch(
              DesktopEvent.ChatLoaded(
                  ChatSession("session", "project", "revision", "base", "main.go")))
        } else {
          presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
          if (work == "edited buffer") {
            presenter.dispatch(DesktopEvent.DraftEdited(declaration = "func Run() { changed() }"))
          }
        }
        val previous = presenter.snapshot.value.state
        presenter.openFileInEditor("other.go")
        presenter.selectFile("other.go")
        main.runPending()
        io.runPending()
        main.runPending()
        assertEquals(previous.selectedFile, presenter.snapshot.value.state.selectedFile, work)
        assertEquals(previous.chat, presenter.snapshot.value.state.chat, work)
        assertEquals(previous.review, presenter.snapshot.value.state.review, work)
        assertNull(presenter.snapshot.value.state.selection.pendingFilePath, work)
        assertTrue(calls.isEmpty(), work)
      } finally {
        presenter.close()
        scope.cancel()
      }
    }
  }

  @Test
  fun sameFileActivationIsInspectionOnlyEvenWithStaleDraftEvidence() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val calls = mutableListOf<String>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
          calls += path
          error("Same-file activation must not read")
        }
    try {
      loadFile(presenter)
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      presenter.dispatch(
          DesktopEvent.SelectedFileRefreshed(file().copy(contentHash = "changed"), emptyList()))
      val previous = presenter.snapshot.value
      presenter.openFileInEditor("main.go")
      assertEquals(previous.state.selection, presenter.snapshot.value.state.selection)
      assertEquals(previous.state.review, presenter.snapshot.value.state.review)
      val intent =
          presenter.fileNavigationIntent(
              "main.go", EditorNavigationTarget("main.go", line = 1), composerHasWork = true)!!
      var loaded = false
      assertTrue(presenter.confirmFileNavigationIntent(intent) { loaded = true })
      assertFalse(presenter.confirmFileNavigationIntent(intent))
      assertEquals(1, presenter.snapshot.value.state.selection.focusedLine)
      assertEquals(previous.state.review, presenter.snapshot.value.state.review)
      assertEquals(previous.state.chat, presenter.snapshot.value.state.chat)
      assertEquals(previous.declarationExplanation, presenter.snapshot.value.declarationExplanation)
      main.runPending()
      io.runPending()
      assertFalse(loaded)
      assertTrue(calls.isEmpty())
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun fileApprovalIsSingleUseAndDoesNotDiscardUntilSuccessfulPublication() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val calls = mutableListOf<String>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
          calls += path
          when {
            path.contains("files/info?") -> response(fileJson("other.go", "other"))
            path.contains("files/symbols?") -> response(symbolsJson("other.go"))
            else -> response("{}")
          }
        }
    try {
      loadFile(presenter)
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      val previous = presenter.snapshot.value.state
      var successes = 0
      val intent = presenter.fileNavigationIntent("other.go")!!
      assertTrue(intent.hasWork)
      assertTrue(presenter.confirmFileNavigationIntent(intent) { successes++ })
      assertEquals(previous.review, presenter.snapshot.value.state.review)
      assertEquals(previous.selectedFile, presenter.snapshot.value.state.selectedFile)
      assertFalse(presenter.confirmFileNavigationIntent(intent) { successes++ })
      repeat(5) {
        main.runPending()
        io.runPending()
      }
      main.runPending()
      assertEquals("other.go", presenter.snapshot.value.state.selectedFile?.path)
      assertNull(presenter.snapshot.value.state.review.editor)
      assertEquals(1, successes)
      assertFalse(presenter.confirmFileNavigationIntent(intent) { successes++ })
      assertEquals(1, calls.count { it.contains("files/info?") })
      assertEquals(1, successes)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun changedWorkOrDestinationRevokesFileApprovalBeforeStartAndPublication() {
    for (timing in listOf("before start", "before transport", "during read")) {
      for (change in
          listOf("editor", "session", "draft", "index", "project", "selected file", "composer")) {
        val main = QueuedDispatcher()
        val io = QueuedDispatcher()
        val scope = CoroutineScope(SupervisorJob() + main)
        val calls = mutableListOf<String>()
        val presenter =
            presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
              calls += path
              when {
                path.contains("files/info?") -> response(fileJson("other.go", "other"))
                path.contains("files/symbols?") -> response(symbolsJson("other.go"))
                else -> response("{}")
              }
            }
        try {
          loadFile(presenter)
          presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
          var composerCurrent = true
          var successes = 0
          val intent =
              presenter.fileNavigationIntent("other.go", composerCurrent = { composerCurrent })!!
          if (timing != "before start") {
            assertTrue(presenter.confirmFileNavigationIntent(intent) { successes++ })
            if (timing == "during read") {
              main.runPending()
              io.runPending() // Transport completed; publication is still queued.
            }
          }
          when (change) {
            "editor" ->
                presenter.dispatch(DesktopEvent.DraftEdited(declaration = "func Run() { newer() }"))
            "session" ->
                presenter.dispatch(
                    DesktopEvent.ChatLoaded(
                        ChatSession("new", "project", "revision", "base", "main.go")))
            "draft" -> presenter.dispatch(DesktopEvent.DraftLoaded(draft().copy(revision = 2)))
            "index" ->
                presenter.dispatch(
                    DesktopEvent.IndexRefreshed(presenter.snapshot.value.state.index!!.copy()))
            "project" -> loadProject(presenter)
            "selected file" ->
                presenter.dispatch(
                    DesktopEvent.SelectedFileRefreshed(
                        file().copy(contentHash = "new"), emptyList()))
            "composer" -> composerCurrent = false
          }
          val newer = presenter.snapshot.value.state
          if (timing == "before start")
              assertFalse(presenter.confirmFileNavigationIntent(intent) { successes++ })
          repeat(5) {
            main.runPending()
            io.runPending()
          }
          main.runPending()
          val finished = presenter.snapshot.value.state
          assertEquals(newer.selectedFile, finished.selectedFile, "$timing/$change")
          assertEquals(newer.chat, finished.chat, "$timing/$change")
          assertEquals(newer.review, finished.review, "$timing/$change")
          assertNull(finished.selection.pendingFilePath, "$timing/$change")
          assertEquals(0, successes, "$timing/$change")
          if (timing != "during read") assertTrue(calls.isEmpty(), change)
          assertFalse(presenter.confirmFileNavigationIntent(intent))
        } finally {
          presenter.close()
          scope.cancel()
        }
      }
    }
  }

  @Test
  fun replacementReadsRetainSourceWorkAndEvidenceUntilSuccessfulPublication() {
    for (outcome in listOf("failed", "canceled", "loaded")) {
      val main = QueuedDispatcher()
      val io = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + main)
      val presenter =
          presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
            when {
              path.contains("files/info?path=other.go") -> response(fileJson("other.go", "other"))
              path.contains("files/symbols?path=other.go") ->
                  when (outcome) {
                    "failed" -> TransportResponse(403, """{"message":"Replacement denied"}""")
                    "canceled" -> throw kotlinx.coroutines.CancellationException("read canceled")
                    else -> response(symbolsJson("other.go", "Other"))
                  }
              path.endsWith("/explanation") -> response(explanationJson("Run"))
              path.contains("files/symbols?path=main.go") -> response(symbolsJson("main.go", "Run"))
              else -> creationFileResponse(path) ?: response("{}")
            }
          }
      fun drain() {
        repeat(5) {
          main.runPending()
          io.runPending()
        }
        main.runPending()
      }
      try {
        loadProject(presenter)
        presenter.selectFile("main.go")
        drain()
        val run = SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true)
        presenter.dispatch(DesktopEvent.EditorContextSelected(run, 3))
        presenter.dispatch(
            DesktopEvent.ChatLoaded(
                ChatSession("session", "project", "revision", "base", "main.go")))
        presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
        presenter.dispatch(DesktopEvent.DraftEdited(declaration = "func Run() { changed() }"))
        presenter.dispatch(
            DesktopEvent.ChecksLoaded(DraftCheckReport("main.go", applicable = true)))
        presenter.explainSelectedDeclaration()
        drain()
        val previous = presenter.snapshot.value
        assertEquals(DeclarationExplanationStatus.Current, previous.declarationExplanation.status)

        assertTrue(
            presenter.confirmFileNavigationIntent(presenter.fileNavigationIntent("other.go")!!))
        main.runPending()
        val pending = presenter.snapshot.value
        assertEquals("other.go", pending.state.selection.pendingFilePath)
        assertEquals(previous.state.selectedFile, pending.state.selectedFile)
        assertEquals(previous.state.symbols, pending.state.symbols)
        assertEquals(run, pending.state.selectedSymbol)
        assertEquals(3, pending.state.selection.focusedLine)
        assertEquals(previous.state.chat, pending.state.chat)
        assertEquals(previous.state.review, pending.state.review)
        assertEquals(previous.state.analysis, pending.state.analysis)
        assertEquals(previous.declarationExplanation, pending.declarationExplanation)
        drain()
        val finished = presenter.snapshot.value
        assertNull(finished.state.selection.pendingFilePath)
        assertFalse(finished.state.jobs.loading)
        if (outcome == "loaded") {
          assertEquals("other.go", finished.state.selectedFile?.path)
          assertEquals("Other", finished.state.symbols.single().name)
          assertNull(finished.state.chat.session)
          assertNull(finished.state.review.editor)
          assertNull(finished.state.checks)
          assertEquals(DeclarationExplanationStatus.Stale, finished.declarationExplanation.status)
        } else {
          assertEquals(previous.state.selectedFile, finished.state.selectedFile)
          assertEquals(previous.state.symbols, finished.state.symbols)
          assertEquals(previous.state.selectedSymbol, finished.state.selectedSymbol)
          assertEquals(previous.state.selection.focusedLine, finished.state.selection.focusedLine)
          assertEquals(previous.state.chat, finished.state.chat)
          assertEquals(previous.state.review, finished.state.review)
          assertEquals(previous.declarationExplanation, finished.declarationExplanation)
          assertEquals(
              if (outcome == "failed") "other.go" else null,
              finished.state.selection.failedFilePath)
          assertEquals(
              if (outcome == "failed") "Replacement denied" else null,
              finished.state.selection.fileReadError)
        }
      } finally {
        presenter.close()
        scope.cancel()
      }
    }
  }

  @Test
  fun ordinaryNavigationValidatesMetadataAndRetainsWorkOnReadFailure() {
    for (outcome in
        listOf(
            "file path",
            "symbols path",
            "symbols project",
            "symbols revision",
            "symbols failure",
            "empty")) {
      val main = QueuedDispatcher()
      val io = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + main)
      val presenter =
          presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
            when {
              path.contains("files/info?path=other.go") ->
                  response(
                      fileJson(if (outcome == "file path") "foreign.go" else "other.go", "other"))
              path.contains("files/symbols?path=other.go") -> {
                val symbols =
                    symbolsJson(
                        if (outcome == "symbols path") "foreign.go" else "other.go",
                        if (outcome == "empty") "" else "Foreign")
                when (outcome) {
                  "symbols project" ->
                      response(
                          symbols.replace(
                              "\"project_id\":\"project\"", "\"project_id\":\"foreign\""))
                  "symbols revision" ->
                      response(
                          symbols.replace(
                              "\"project_revision\":\"revision\"",
                              "\"project_revision\":\"foreign\""))
                  "symbols failure" ->
                      TransportResponse(403, """{"message":"Declarations read denied"}""")
                  else -> response(symbols)
                }
              }
              else -> creationFileResponse(path) ?: response("{}")
            }
          }
      fun drain() {
        repeat(5) {
          main.runPending()
          io.runPending()
        }
        main.runPending()
      }
      try {
        loadProject(presenter)
        presenter.selectFile("main.go")
        drain()
        presenter.dispatch(
            DesktopEvent.ChatLoaded(
                ChatSession("session", "project", "revision", "base", "main.go")))
        presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
        presenter.dispatch(DesktopEvent.DraftEdited(declaration = "func Run() { changed() }"))
        val previous = presenter.snapshot.value.state
        assertTrue(
            presenter.confirmFileNavigationIntent(presenter.fileNavigationIntent("other.go")!!))
        drain()
        val state = presenter.snapshot.value.state
        assertNull(state.selection.pendingFilePath, outcome)
        assertFalse(state.jobs.loading, outcome)
        if (outcome == "empty") {
          assertEquals("other.go", state.selectedFile?.path)
          assertEquals("package main", state.selectedFile?.content)
          assertTrue(state.symbols.isEmpty())
          assertNull(state.selectedSymbol)
          assertNull(state.selection.fileReadError)
        } else {
          assertEquals(previous.selectedFile, state.selectedFile, outcome)
          assertEquals(previous.symbols, state.symbols, outcome)
          assertEquals(previous.chat, state.chat, outcome)
          assertEquals(previous.review, state.review, outcome)
          assertEquals("other.go", state.selection.failedFilePath, outcome)
          assertTrue(
              state.selection.fileReadError?.contains(
                  if (outcome == "symbols failure") "Declarations read denied"
                  else "Could not open other.go:") == true,
              outcome)
        }
      } finally {
        presenter.close()
        scope.cancel()
      }
    }
  }

  @Test
  fun completedOrdinaryReadCannotPublishAfterProjectRevisionOrInventoryChanges() {
    for (change in listOf("project", "revision", "removed destination", "foreign index")) {
      val main = QueuedDispatcher()
      val io = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + main)
      val presenter =
          presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
            when {
              path.contains("files/info?") -> response(fileJson("other.go", "other"))
              path.contains("files/symbols?") -> response(symbolsJson("other.go", "Other"))
              else -> response("{}")
            }
          }
      try {
        loadFile(presenter)
        presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
        if (change == "foreign index") {
          val index = presenter.snapshot.value.state.index!!
          presenter.dispatch(
              DesktopEvent.ProjectLoaded(project(), index.copy(projectId = "foreign")))
        }
        if (change == "foreign index") {
          assertNull(presenter.fileNavigationIntent("other.go"))
          presenter.openFileInEditor("other.go")
        } else {
          assertTrue(
              presenter.confirmFileNavigationIntent(presenter.fileNavigationIntent("other.go")!!))
        }
        main.runPending()
        io.runPending() // Read completed; its UI publication is still deferred.
        val previous = presenter.snapshot.value.state
        val index = previous.index!!
        when (change) {
          "project" ->
              presenter.dispatch(
                  DesktopEvent.ProjectLoaded(project("new"), index.copy(projectId = "new")))
          "revision" ->
              presenter.dispatch(DesktopEvent.IndexRefreshed(index.copy(projectRevision = "next")))
          "removed destination" ->
              presenter.dispatch(
                  DesktopEvent.IndexRefreshed(
                      index.copy(files = index.files.filterNot { it.path == "other.go" })))
        }
        main.runPending()
        assertTrue(presenter.snapshot.value.state.selectedFile?.path != "other.go", change)
        assertTrue(presenter.snapshot.value.state.symbols.none { it.name == "Other" }, change)
        if (change == "removed destination" || change == "foreign index") {
          val state = presenter.snapshot.value.state
          assertEquals(previous.selectedFile, state.selectedFile)
          assertEquals(previous.review, state.review)
          assertNull(state.selection.failedFilePath)
          assertNull(state.selection.fileReadError)
          if (change == "foreign index")
              assertTrue(state.error?.contains("no longer points") == true)
        }
        assertNull(presenter.snapshot.value.state.selection.pendingFilePath, change)
      } finally {
        presenter.close()
        scope.cancel()
      }
    }
  }

  @Test
  fun nonCooperativeSupersededTransportCannotReplaceNewSourceOrClearNewFailure() {
    for (lateFailure in listOf(false, true)) {
      for (newFailure in listOf(false, true)) {
        val entered = CountDownLatch(1)
        val ignoredCancellation = CountDownLatch(1)
        val release = CountDownLatch(1)
        val main = Executors.newSingleThreadExecutor().asCoroutineDispatcher()
        val io = Executors.newFixedThreadPool(2).asCoroutineDispatcher()
        val scope = CoroutineScope(SupervisorJob() + main)
        val presenter =
            presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
              when {
                path.contains("files/info?path=other.go") -> response(fileJson("other.go", "late"))
                path.contains("files/symbols?path=other.go") -> {
                  entered.countDown()
                  while (true) {
                    try {
                      release.await()
                      break
                    } catch (_: InterruptedException) {
                      ignoredCancellation.countDown()
                    }
                  }
                  if (lateFailure) TransportResponse(403, """{"message":"Late read failure"}""")
                  else response(symbolsJson("other.go", "Late"))
                }
                path.contains("files/info?path=second.go") -> response(fileJson("second.go", "new"))
                path.contains("files/symbols?path=second.go") ->
                    if (newFailure) TransportResponse(403, """{"message":"Current read failure"}""")
                    else response(symbolsJson("second.go", "Current"))
                else -> creationFileResponse(path) ?: response("{}")
              }
            }
        try {
          runBlocking(main) {
            loadFile(presenter)
            presenter.dispatch(
                DesktopEvent.IndexRefreshed(
                    presenter.snapshot.value.state.index!!.let {
                      it.copy(files = it.files + IndexedFile("second.go", "new", "Go", false))
                    }))
            presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
            assertTrue(
                presenter.confirmFileNavigationIntent(presenter.fileNavigationIntent("other.go")!!))
          }
          assertTrue(entered.await(2, TimeUnit.SECONDS))
          val newer =
              runBlocking(main) {
                assertTrue(
                    presenter.confirmFileNavigationIntent(
                        presenter.fileNavigationIntent("second.go")!!))
                withTimeout(2_000) {
                      presenter.snapshot.first {
                        it.state.selection.fileReadError == "Current read failure" ||
                            it.state.symbols.any { symbol -> symbol.name == "Current" }
                      }
                    }
                    .state
              }
          assertTrue(ignoredCancellation.await(2, TimeUnit.SECONDS))
          release.countDown()
          runBlocking(main) {
            val lifetime = scope.coroutineContext[Job]!!.children.single()
            presenter.close()
            withTimeout(2_000) { lifetime.join() }
            val state = presenter.snapshot.value.state
            assertEquals(newer.selectedFile, state.selectedFile)
            assertEquals(newer.symbols, state.symbols)
            assertEquals(newer.review, state.review)
            assertEquals(newer.selection.fileReadError, state.selection.fileReadError)
            assertEquals(newer.selection.failedFilePath, state.selection.failedFilePath)
            assertNull(state.selection.pendingFilePath)
            assertTrue(state.symbols.none { it.name == "Late" })
          }
        } finally {
          release.countDown()
          presenter.close()
          scope.cancel()
          io.close()
          main.close()
        }
      }
    }
  }

  @Test
  fun closingCancelsPendingReplacementEvenBeforeItsCoroutineStarts() {
    for (started in listOf(false, true)) {
      val main = QueuedDispatcher()
      val io = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + main)
      val calls = mutableListOf<String>()
      val presenter =
          presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
            calls += path
            creationFileResponse(path) ?: error("Unexpected $path")
          }
      try {
        loadProject(presenter)
        presenter.selectFile("main.go")
        repeat(5) {
          main.runPending()
          io.runPending()
        }
        main.runPending()
        presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
        val previous = presenter.snapshot.value.state
        calls.clear()
        assertTrue(
            presenter.confirmFileNavigationIntent(presenter.fileNavigationIntent("other.go")!!))
        if (started) main.runPending()
        assertEquals("other.go", presenter.snapshot.value.state.selection.pendingFilePath)
        presenter.close()
        assertNull(presenter.snapshot.value.state.selection.pendingFilePath)
        assertEquals(previous.selectedFile, presenter.snapshot.value.state.selectedFile)
        assertEquals(previous.review.editor, presenter.snapshot.value.state.review.editor)
        main.runPending()
        io.runPending()
        main.runPending()
        assertTrue(calls.isEmpty())
        assertNull(presenter.snapshot.value.state.selection.pendingFilePath)
      } finally {
        presenter.close()
        scope.cancel()
      }
    }
  }

  @Test
  fun pendingReplacementDoesNotCancelChecksAndSuccessfulPublicationClearsRunningWork() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
          when {
            path.contains("/drafts/draft/checks") ->
                response(
                    """{"target_path":"main.go","applicable":true,"draft_id":"draft","draft_revision":1,"draft_hash":"draft-hash","checks":[]}""")
            path.contains("files/info?path=other.go") -> response(fileJson("other.go", "other"))
            path.contains("files/symbols?path=other.go") -> response(symbolsJson("other.go"))
            else -> creationFileResponse(path) ?: response("{}")
          }
        }
    try {
      loadProject(presenter)
      presenter.selectFile("main.go")
      repeat(5) {
        main.runPending()
        io.runPending()
      }
      main.runPending()
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      presenter.runDraftChecks()
      main.runPending()
      val running = presenter.snapshot.value.state.review.checkAttempt
      assertEquals(ValidationAttemptStatus.Running, running?.status)
      assertTrue(
          presenter.confirmFileNavigationIntent(presenter.fileNavigationIntent("other.go")!!))
      main.runPending()
      assertEquals(running, presenter.snapshot.value.state.review.checkAttempt)
      io.runNext() // The old-file checks may finish while the replacement read is pending.
      main.runPending()
      assertTrue(presenter.snapshot.value.state.checks?.applicable == true)
      assertEquals("main.go", presenter.snapshot.value.state.selectedFile?.path)
      io.runPending()
      main.runPending()
      assertEquals("other.go", presenter.snapshot.value.state.selectedFile?.path)
      assertNull(presenter.snapshot.value.state.review.checkAttempt)
      assertNull(presenter.snapshot.value.state.checks)
      assertFalse(presenter.snapshot.value.generating)
      assertFalse(presenter.snapshot.value.draftValidationInProgress)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun generationFailureSurvivesOtherStatusAndRetryUntilFileChange() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { method, path, _ ->
          when {
            path.contains("files/info?") ->
                response(fileJson(if (path.contains("other.go")) "other.go" else "main.go", "base"))
            path.contains("files/symbols?") ->
                response(
                    symbolsJson(if (path.contains("other.go")) "other.go" else "main.go", "Run"))
            method == "POST" && path.endsWith("chat/sessions") ->
                TransportResponse(
                    503,
                    """{"type":"provider_unavailable","message":"provider request failed","user_message":"provider request failed"}""")
            else -> response("{}")
          }
        }
    fun drain() {
      repeat(5) {
        main.runPending()
        io.runPending()
      }
      main.runPending()
    }
    try {
      loadProject(presenter)
      presenter.selectFile("main.go")
      drain()
      presenter.dispatch(
          DesktopEvent.SymbolSelected(
              SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true)))
      presenter.sendChatMessage(ChatEditMode.ReplaceSymbol, "", "Preserve the public signature")
      drain()
      val failure = presenter.snapshot.value.state.chat.attempts.single()
      assertEquals("Request failed for Run.", presenter.snapshot.value.state.status)
      assertEquals(ChatTarget(ChatEditMode.ReplaceSymbol, "Run"), failure.scope.target)
      assertTrue(
          (failure.outcome as ChatRequestOutcome.Failed)
              .message
              .contains("provider request failed"))
      presenter.dispatch(DesktopEvent.Status("Another operation finished"))
      assertEquals(failure, presenter.snapshot.value.state.chat.attempts.single())
      presenter.sendChatMessage(
          ChatEditMode.ReplaceSymbol, "", "Try preserving the public signature again")
      assertEquals(failure, presenter.snapshot.value.state.chat.attempts.first())
      drain()
      assertEquals(2, presenter.snapshot.value.state.chat.attempts.size)
      assertTrue(
          presenter.confirmFileNavigationIntent(presenter.fileNavigationIntent("other.go")!!))
      drain()
      assertEquals("other.go", presenter.snapshot.value.state.selectedFile?.path)
      assertTrue(presenter.snapshot.value.state.chat.attempts.isEmpty())
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun returningFromTerminalRetainsUnchangedDraftAndStalesChangedEvidenceUsingOnlyReads() {
    listOf(false, true).forEach { changed ->
      val calls = mutableListOf<Pair<String, String>>()
      val main = QueuedDispatcher()
      val io = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + main)
      val presenter =
          presenter(parentScope = scope, ioDispatcher = io) { method, path, _ ->
            calls.add(method to path)
            when {
              path.contains("files/info?") ->
                  response(fileJson("main.go", if (changed) "shell-edit" else "base"))
              path.contains("files/symbols?") -> response(symbolsJson("main.go", "Run"))
              else -> error("Unexpected $method $path")
            }
          }
      try {
        loadFile(presenter)
        presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
        presenter.dispatch(
            DesktopEvent.AnalysisRunUpdated(ProjectAnalysisRunState(run = analysisRunFixture())))
        val before = presenter.snapshot.value.state.review
        presenter.refreshSelectedFile()
        repeat(4) {
          main.runPending()
          io.runPending()
        }
        main.runPending()
        if (changed) {
          val state = presenter.snapshot.value.state
          assertEquals(DraftEditorStatus.Stale, state.review.editor?.status)
          assertNull(state.review.checks)
          assertNull(state.analysis)
          assertEquals("stale", state.analysisRun.run?.status)
          assertFalse(
              draftReviewEligibility(
                      state.review.editor,
                      state.review.draft,
                      state.review.checks,
                      state.selectedFile,
                      state.project)
                  .eligible)
          // Polling the same daemon run cannot make locally observed stale evidence fresh again.
          presenter.dispatch(
              DesktopEvent.AnalysisRunUpdated(ProjectAnalysisRunState(run = analysisRunFixture())))
          assertEquals("stale", presenter.snapshot.value.state.analysisRun.run?.status)
        } else assertEquals(before, presenter.snapshot.value.state.review)
        assertTrue(calls.all { it.first == "GET" })
        assertEquals(if (changed) 2 else 1, calls.size)
      } finally {
        presenter.close()
        scope.cancel()
      }
    }
  }

  @Test
  fun failedReturnRefreshBlocksOldDraftAndLateRefreshCannotReplaceAnotherSelection() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { _, _, _ ->
          TransportResponse(404, """{"message":"file removed"}""")
        }
    try {
      loadFile(presenter)
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      presenter.refreshSelectedFile()
      main.runPending()
      io.runPending()
      main.runPending()
      assertNull(presenter.snapshot.value.state.selectedFile)
      assertEquals(DraftEditorStatus.Stale, presenter.snapshot.value.state.review.editor?.status)
      assertTrue(presenter.snapshot.value.state.error.orEmpty().contains("file removed"))
      loadFile(presenter)
      presenter.refreshSelectedFile()
      main.runPending()
      presenter.dispatch(DesktopEvent.FileLoaded(file().copy(path = "other.go"), emptyList()))
      io.runPending()
      main.runPending()
      assertEquals("other.go", presenter.snapshot.value.state.selectedFile?.path)
      assertNull(presenter.snapshot.value.state.error)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun resultNavigationShowsAllFilesWithoutRequestingAnalysisOrChangingTheSelectedFile() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val calls = mutableListOf<String>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
          calls.add(path)
          response("{}")
        }
    try {
      presenter.dispatch(DesktopEvent.ProjectLoaded(resultProjectFixture(), resultIndexFixture()))
      val run = analysisRunFixture()
      val sections =
          listOf("bugs", "performance", "security").associate { category ->
            val result = analysisResultsFixture(run, category)
            val findings =
                listOf("main.go", "other.go").map { path ->
                  UnifiedFinding(
                      id = path,
                      category = category,
                      projectId = run.identity.projectId,
                      projectRevision = run.identity.projectRevision,
                      location = FindingLocation(path))
                }
            AnalysisResultKey(category) to
                AnalysisSectionState(results = result.copy(semantic = findings))
          }
      val analysis = ProjectAnalysisRunState(run = run, sections = sections)
      presenter.dispatch(DesktopEvent.AnalysisRunUpdated(analysis))
      for (category in listOf("bugs", "performance", "security")) {
        presenter.viewAnalysisResults(category, "main.go")
        val state = presenter.snapshot.value.state
        assertEquals(analysisCategoryWorkspace(category), state.workspace)
        assertEquals(
            listOf("main.go", "other.go"),
            state.analysisResultPage(category).semantic.map { it.location.path })
        assertEquals(analysis, state.analysisRun)
      }
      main.runPending()
      io.runPending()
      main.runPending()
      assertEquals(Workspace.Security, presenter.snapshot.value.state.workspace)
      assertNull(presenter.snapshot.value.state.selectedFile)
      assertTrue(calls.isEmpty())
      presenter.viewAnalysisResults("bugs", "missing.go")
      presenter.viewAnalysisResults("unknown", "main.go")
      assertEquals(Workspace.Security, presenter.snapshot.value.state.workspace)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun projectWideResultsPrepareOnlyAnExplicitCurrentDeclarationWithoutGenerating() {
    listOf("security", "performance").forEach { category ->
      val writes = AtomicInteger()
      val presenter = presenter { method, path, _ ->
        if (method != "GET") writes.incrementAndGet()
        when {
          path.contains("files/info?path=main.go") ->
              response(fileJson("main.go", "base").replace("\"line_count\":1", "\"line_count\":20"))
          path.contains("files/symbols?path=main.go") ->
              response(symbolsJson("main.go", "Run", "func Run()", 2, 10))
          path.contains("files/analysis") -> response("""{"path":"main.go","status":"missing"}""")
          path.contains("/impact") -> response("""{"target_path":"main.go"}""")
          path.contains("/git") -> response("""{"available":false}""")
          else -> response("{}")
        }
      }
      try {
        presenter.dispatch(DesktopEvent.ProjectLoaded(resultProjectFixture(), resultIndexFixture()))
        val page = if (category == "security") securityPageFixture() else performancePageFixture()
        presenter.dispatch(
            DesktopEvent.AnalysisRunUpdated(
                ProjectAnalysisRunState(
                    run = page.run, sections = mapOf(AnalysisResultKey(category) to page.section))))
        assertNull(presenter.snapshot.value.state.selectedFile)
        if (category == "security")
            presenter.prepareSecurityFinding(
                securityResults(presenter.snapshot.value.state.analysisResultPage("security"))
                    .first())
        else
            presenter.preparePerformanceFinding(
                performanceResults(presenter.snapshot.value.state.analysisResultPage("performance"))
                    .single())
        eventually { presenter.snapshot.value.state.preparedRequest.isNotBlank() }
        assertEquals("main.go", presenter.snapshot.value.state.selectedFile?.path)
        assertEquals(0, writes.get())
      } finally {
        presenter.close()
      }
    }
  }

  @Test
  fun browsingAndSourceInspectionStayLocalUntilAnExactFixIsExplicitlyPrepared() {
    val dispatcher = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    val calls = mutableListOf<String>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = dispatcher) { method, path, _ ->
          calls += "$method $path"
          when {
            method == "GET" && path.contains("files/info?path=main.go") ->
                response(fileJson("main.go", "base"))
            method == "GET" && path.contains("files/symbols?path=main.go") ->
                response(symbolsJson("main.go", "Run", "func Run()"))
            method == "GET" && path.contains("files/analysis") ->
                response("""{"path":"main.go","status":"missing"}""")
            method == "GET" && path.contains("/impact") -> response("""{"target_path":"main.go"}""")
            method == "GET" && path.contains("/git") -> response("""{"available":false}""")
            else -> error("Unexpected $method $path")
          }
        }
    try {
      val run = analysisRunFixture()
      val finding =
          UnifiedFinding(
              id = "exact",
              category = "bugs",
              projectId = "project",
              projectRevision = "revision",
              fileHash = "base",
              freshness = "fresh",
              title = "Fix Run",
              location = FindingLocation("main.go", startLine = 17, symbol = "Run"),
              taskSpec = BugTaskSpec("1", "main.go", "Run", "func Run()", listOf("Fix Run.")))
      presenter.dispatch(DesktopEvent.ProjectLoaded(resultProjectFixture(), resultIndexFixture()))
      presenter.dispatch(
          DesktopEvent.AnalysisRunUpdated(
              ProjectAnalysisRunState(
                  run = run,
                  sections =
                      mapOf(
                          AnalysisResultKey("bugs") to
                              AnalysisSectionState(
                                  results =
                                      analysisResultsFixture(run, "bugs")
                                          .copy(semantic = listOf(finding)))))))
      val store = ResultBrowserStore()
      val bugs = presenter.snapshot.value.state.analysisResultPage("bugs")
      val browser = store.stateFor(bugs)
      val rows = presenter.snapshot.value.state.projectBugFindings().map(::semanticResultRow)
      assertEquals(1, rows.size)
      browser.query = "MAIN.GO"
      browser.filter = ResultBrowserFilter.Value("unknown")
      assertEquals(rows, filteredResultRows(rows, browser.filter, browser.query))
      browser.choose(rows.single().key)
      presenter.dispatch(DesktopEvent.WorkspaceSelected(Workspace.Performance))
      store.stateFor(presenter.snapshot.value.state.analysisResultPage("performance"))
      presenter.dispatch(DesktopEvent.WorkspaceSelected(Workspace.Bugs))
      assertEquals(
          browser, store.stateFor(presenter.snapshot.value.state.analysisResultPage("bugs")))
      assertEquals(rows.single().key, browser.selectedKey)
      assertTrue(calls.isEmpty(), "Browsing and selecting evidence must not issue requests")

      presenter.openFinding(finding)
      dispatcher.runPending()
      assertEquals(17, presenter.snapshot.value.state.selection.focusedLine)
      assertEquals("main.go", presenter.snapshot.value.state.selectedFile?.path)
      assertTrue(presenter.snapshot.value.state.preparedRequest.isBlank())
      val sourceCalls = calls.toList()
      assertEquals(5, sourceCalls.size)
      assertTrue(sourceCalls.all { it.startsWith("GET ") }, "$sourceCalls")

      presenter.dispatch(DesktopEvent.WorkspaceSelected(Workspace.Bugs))
      presenter.prepareFinding(finding)
      dispatcher.runPending()
      assertEquals("Run", presenter.snapshot.value.state.selectedSymbol?.name)
      assertTrue(presenter.snapshot.value.state.preparedRequest.contains("Fix Run."))
      assertEquals(sourceCalls + sourceCalls, calls)
      assertTrue(calls.none { it.startsWith("POST") || it.startsWith("DELETE") })
      assertNull(presenter.snapshot.value.state.review.draft)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun findingNavigationRequiresCurrentApprovalBeforeReplacingDraft() {
    val dispatcher = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    val calls = mutableListOf<String>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = dispatcher) { method, path, _ ->
          calls += "$method $path"
          when {
            path.contains("files/info?path=") ->
                response(fileJson(if (path.contains("other.go")) "other.go" else "main.go", "base"))
            path.contains("files/symbols?path=") ->
                response(
                    symbolsJson(
                        if (path.contains("other.go")) "other.go" else "main.go",
                        "Run",
                        "func Run()"))
            path.contains("files/analysis") ->
                response("""{"path":"other.go","status":"missing"}""")
            path.contains("/impact") -> response("""{"target_path":"other.go"}""")
            path.contains("/git") -> response("""{"available":false}""")
            else -> error("Unexpected $method $path")
          }
        }
    try {
      val run = analysisRunFixture()
      val finding =
          UnifiedFinding(
              id = "other",
              category = "bugs",
              projectId = "project",
              projectRevision = "revision",
              fileHash = "base",
              freshness = "fresh",
              location = FindingLocation("other.go", startLine = 7, symbol = "Run"),
              taskSpec = BugTaskSpec("1", "other.go", "Run", "func Run()", listOf("Fix Run.")))
      val index = resultIndexFixture()
      presenter.dispatch(
          DesktopEvent.ProjectLoaded(
              resultProjectFixture(),
              index.copy(files = index.files + index.files.single().copy(path = "other.go"))))
      presenter.dispatch(
          DesktopEvent.AnalysisRunUpdated(
              ProjectAnalysisRunState(
                  run = run,
                  sections =
                      mapOf(
                          AnalysisResultKey("bugs") to
                              AnalysisSectionState(
                                  results =
                                      analysisResultsFixture(run, "bugs")
                                          .copy(semantic = listOf(finding)))))))
      presenter.selectFile("main.go")
      dispatcher.runPending()
      assertEquals("main.go", presenter.snapshot.value.state.selectedFile?.path)
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      val original = presenter.snapshot.value.state
      val source = presenter.findingIntent(finding, false)!!
      val prepare = presenter.findingIntent(finding, true)!!
      val before = calls.size
      presenter.openFinding(finding)
      presenter.prepareFinding(finding)
      assertEquals(before, calls.size)
      assertEquals(original.review, presenter.snapshot.value.state.review)
      assertEquals("main.go", presenter.snapshot.value.state.selectedFile?.path)

      presenter.dispatch(DesktopEvent.DraftEdited(declaration = "func Run() { changed() }"))
      presenter.confirmFindingIntent(source)
      presenter.confirmFindingIntent(prepare)
      assertEquals(before, calls.size)
      assertEquals("main.go", presenter.snapshot.value.state.selectedFile?.path)
      assertTrue(presenter.snapshot.value.state.review.draft != null)

      val current = presenter.findingIntent(finding, true)!!
      presenter.dispatch(
          DesktopEvent.AnalysisRunUpdated(
              ProjectAnalysisRunState(
                  run = run,
                  sections =
                      mapOf(
                          AnalysisResultKey("bugs") to
                              AnalysisSectionState(
                                  results = analysisResultsFixture(run, "bugs"))))))
      presenter.confirmFindingIntent(current)
      assertEquals(before, calls.size)
      assertTrue(presenter.snapshot.value.state.review.draft != null)
      presenter.dispatch(
          DesktopEvent.AnalysisRunUpdated(
              ProjectAnalysisRunState(
                  run = run,
                  sections =
                      mapOf(
                          AnalysisResultKey("bugs") to
                              AnalysisSectionState(
                                  results =
                                      analysisResultsFixture(run, "bugs")
                                          .copy(semantic = listOf(finding)))))))
      presenter.confirmFindingIntent(presenter.findingIntent(finding, true)!!)
      dispatcher.runPending()
      assertTrue(presenter.snapshot.value.state.preparedRequest.contains("Fix Run."))
      assertEquals("other.go", presenter.snapshot.value.state.selectedFile?.path)
      assertNull(presenter.snapshot.value.state.review.draft)
      presenter.selectFile("main.go")
      dispatcher.runPending()
      assertEquals("main.go", presenter.snapshot.value.state.selectedFile?.path)
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      presenter.confirmFindingIntent(presenter.findingIntent(finding, false)!!)
      dispatcher.runPending()
      assertEquals("other.go", presenter.snapshot.value.state.selectedFile?.path)
      assertEquals(7, presenter.snapshot.value.state.selection.focusedLine)
      assertNull(presenter.snapshot.value.state.review.draft)
      assertTrue(calls.none { it.startsWith("POST") })
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun changedProjectRevokesCapturedFindingApproval() {
    val calls = mutableListOf<String>()
    val presenter = presenter { method, path, _ ->
      calls += "$method $path"
      error("Unexpected request $method $path")
    }
    try {
      val run = analysisRunFixture()
      val finding =
          UnifiedFinding(
              id = "bug",
              category = "bugs",
              projectId = "project",
              projectRevision = "revision",
              location = FindingLocation("main.go", startLine = 4))
      val project = resultProjectFixture()
      val index = resultIndexFixture()
      presenter.dispatch(DesktopEvent.ProjectLoaded(project, index))
      presenter.dispatch(
          DesktopEvent.AnalysisRunUpdated(
              ProjectAnalysisRunState(
                  run = run,
                  sections =
                      mapOf(
                          AnalysisResultKey("bugs") to
                              AnalysisSectionState(
                                  results =
                                      analysisResultsFixture(run, "bugs")
                                          .copy(semantic = listOf(finding)))))))
      val captured = presenter.findingIntent(finding, false)!!
      presenter.dispatch(
          DesktopEvent.ProjectLoaded(
              project.copy(projectRevision = "next"), index.copy(projectRevision = "next")))
      presenter.confirmFindingIntent(captured)
      assertTrue(calls.isEmpty())
      assertNull(presenter.snapshot.value.state.selectedFile)
      assertTrue(presenter.snapshot.value.state.jobs.error?.contains("changed") == true)
    } finally {
      presenter.close()
    }
  }

  @Test
  fun sameSourceFindingInspectionPreservesDraftAndExactLineWithoutReads() {
    val dispatcher = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    val calls = mutableListOf<String>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = dispatcher) { method, path, _ ->
          calls += "$method $path"
          when {
            path.contains("files/info?") -> response(fileJson("main.go", "base"))
            path.contains("files/symbols?") -> response(symbolsJson("main.go", "Run", "func Run()"))
            path.contains("files/analysis") -> response("""{"path":"main.go","status":"missing"}""")
            path.contains("/impact") -> response("""{"target_path":"main.go"}""")
            path.contains("/git") -> response("""{"available":false}""")
            else -> error("Unexpected $method $path")
          }
        }
    try {
      val run = analysisRunFixture()
      val finding =
          UnifiedFinding(
              id = "same",
              category = "bugs",
              projectId = "project",
              projectRevision = "revision",
              location = FindingLocation("main.go", startLine = 7, symbol = "Run"))
      presenter.dispatch(DesktopEvent.ProjectLoaded(resultProjectFixture(), resultIndexFixture()))
      presenter.dispatch(
          DesktopEvent.AnalysisRunUpdated(
              ProjectAnalysisRunState(
                  run = run,
                  sections =
                      mapOf(
                          AnalysisResultKey("bugs") to
                              AnalysisSectionState(
                                  results =
                                      analysisResultsFixture(run, "bugs")
                                          .copy(semantic = listOf(finding)))))))
      presenter.selectFile("main.go")
      dispatcher.runPending()
      assertEquals("main.go", presenter.snapshot.value.state.selectedFile?.path)
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      val sourceReads = calls.count { it.contains("files/info?") || it.contains("files/symbols?") }
      val review = presenter.snapshot.value.state.review
      presenter.openFinding(finding)
      assertEquals(
          sourceReads, calls.count { it.contains("files/info?") || it.contains("files/symbols?") })
      assertEquals(review, presenter.snapshot.value.state.review)
      assertEquals(7, presenter.snapshot.value.state.selection.focusedLine)
      assertEquals(Workspace.Editor, presenter.snapshot.value.state.workspace)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun typedPerformancePreparationRetainsWorkOnReadFailureAndRejectsChangedApproval() {
    val dispatcher = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    var fail = true
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
          when {
            path.contains("files/info?") ->
                if (fail) TransportResponse(503, "") else response(fileJson("main.go", "base"))
            path.contains("files/symbols?") ->
                response(symbolsJson("main.go", "Run", "func Run()", 2, 10))
            else -> error("Unexpected request $path")
          }
        }
    try {
      val page = performancePageFixture()
      presenter.dispatch(DesktopEvent.ProjectLoaded(resultProjectFixture(), resultIndexFixture()))
      presenter.dispatch(
          DesktopEvent.AnalysisRunUpdated(
              ProjectAnalysisRunState(
                  run = page.run,
                  sections = mapOf(AnalysisResultKey("performance") to page.section))))
      val result =
          performanceResults(presenter.snapshot.value.state.analysisResultPage("performance"))
              .single()
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      var message = TextFieldValue("Keep this text")
      var constraints = TextFieldValue("Keep constraints")
      fun clearConstraints() {
        constraints = TextFieldValue()
      }
      var pending: PendingDraftDiscard.PerformancePreparation? = null
      fun route() {
        routePerformancePreparationRequest(
            presenter,
            result,
            message,
            constraints,
            { true },
            { message to constraints },
            ::clearConstraints) {
              pending = it
            }
      }
      fun confirm() {
        confirmPerformancePreparationDiscard(
            presenter,
            requireNotNull(pending),
            message,
            constraints,
            { message to constraints },
            ::clearConstraints)
      }
      route()
      assertEquals("Keep this text", message.text) // Dismissal leaves all work intact.
      assertEquals(draft(), presenter.snapshot.value.state.review.draft)
      constraints = TextFieldValue("Changed")
      confirm()
      assertEquals("Changed", constraints.text)
      assertEquals(draft(), presenter.snapshot.value.state.review.draft)
      constraints = TextFieldValue("Keep constraints")
      pending = null
      route()
      presenter.dispatch(DesktopEvent.DraftEdited(declaration = "func Run() { return }"))
      confirm()
      assertEquals("Keep constraints", constraints.text)
      assertEquals(
          "func Run() { return }", presenter.snapshot.value.state.review.editor?.declaration)
      pending = null
      route()
      confirm()
      dispatcher.runPending()
      io.runPending()
      dispatcher.runPending()
      assertEquals("Keep this text", message.text)
      assertEquals("Keep constraints", constraints.text)
      assertEquals(
          "func Run() { return }", presenter.snapshot.value.state.review.editor?.declaration)
      assertTrue(presenter.snapshot.value.state.preparedRequest.isBlank())
      fail = false
      pending = null
      route()
      confirm()
      dispatcher.runNext()
      message = TextFieldValue("New input during read")
      io.runPending()
      dispatcher.runPending()
      assertEquals("New input during read", message.text)
      assertEquals("Keep constraints", constraints.text)
      assertTrue(presenter.snapshot.value.state.preparedRequest.isBlank())
      assertEquals(
          "func Run() { return }", presenter.snapshot.value.state.review.editor?.declaration)
      pending = null
      route()
      confirm()
      dispatcher.runPending()
      io.runPending()
      dispatcher.runPending()
      val state = presenter.snapshot.value.state
      assertEquals("", constraints.text)
      assertNull(state.review.editor)
      assertTrue(
          state.preparedRequest.contains(
              "Observed pattern: A buffer is allocated on every request."))
      assertTrue(state.preparedRequest.contains("Recommendation: Reuse a bounded buffer."))
      assertTrue(
          state.preparedRequest.contains("Trade-offs: Retained buffers increase memory use."))
      assertEquals("Run", state.selectedSymbol?.name)
      assertEquals(0, state.preparedTaskSpec?.acceptanceCriteria?.size ?: 0)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun typedPreparationCancelsPreviousFileWorkOnlyAfterLoadedSourceValidation() {
    for (readFails in listOf(false, true)) {
      val main = QueuedDispatcher()
      val io = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + main)
      var chatRequests = 0
      var fileReads = 0
      val presenter =
          presenter(parentScope = scope, ioDispatcher = io) { method, path, _ ->
            when {
              path.contains("files/info?") -> {
                fileReads++
                if (readFails && fileReads == 2) TransportResponse(503, "")
                else response(fileJson("main.go", "base"))
              }
              path.contains("files/symbols?") ->
                  response(symbolsJson("main.go", "Run", "func Run()", 2, 10))
              path.contains("files/analysis") ->
                  response("""{"path":"main.go","status":"missing"}""")
              path.contains("/impact") -> response("""{"target_path":"main.go"}""")
              path.contains("/git") -> response("""{"available":false}""")
              method == "POST" -> {
                chatRequests++
                response(creationSessionJson())
              }
              else -> error("Unexpected $method $path")
            }
          }
      try {
        val page = performancePageFixture()
        presenter.dispatch(DesktopEvent.ProjectLoaded(resultProjectFixture(), resultIndexFixture()))
        presenter.dispatch(
            DesktopEvent.AnalysisRunUpdated(
                ProjectAnalysisRunState(
                    run = page.run,
                    sections = mapOf(AnalysisResultKey("performance") to page.section))))
        presenter.selectFile("main.go")
        repeat(3) {
          main.runPending()
          io.runPending()
        }
        main.runPending()
        assertEquals("main.go", presenter.snapshot.value.state.selectedFile?.path)
        presenter.dispatch(
            DesktopEvent.SymbolSelected(resultIndexFixture().files.single().symbols.single()))
        val result =
            performanceResults(presenter.snapshot.value.state.analysisResultPage("performance"))
                .single()
        // Preparation reads first; a still-authorized chat starts while those reads are pending.
        presenter.preparePerformanceFinding(result)
        main.runNext()
        presenter.sendChatMessage(ChatEditMode.ReplaceSymbol, "", "Improve Run")
        assertTrue(presenter.snapshot.value.generating)
        main.runNext()
        io.runNext() // Complete only the preparation read, leaving chat I/O queued.
        main.runPending()
        if (readFails) {
          assertEquals("main.go", presenter.snapshot.value.state.selectedFile?.path)
          assertTrue(presenter.snapshot.value.generating)
          assertTrue(presenter.snapshot.value.state.preparedRequest.isBlank())
        } else {
          assertTrue(
              presenter.snapshot.value.state.preparedRequest.startsWith("Optimize Run"),
              "error=${presenter.snapshot.value.state.error}, reads=$fileReads, chat=$chatRequests, generating=${presenter.snapshot.value.generating}")
          io.runPending()
          main.runPending()
          assertFalse(presenter.snapshot.value.generating)
          assertEquals(0, chatRequests)
          assertNull(presenter.snapshot.value.state.chat.session)
        }
      } finally {
        presenter.close()
        scope.cancel()
      }
    }
  }

  @Test
  fun typedPreparationRejectsBrowserSelectionChangesDuringReadAndBeforeApproval() {
    for (failure in listOf(false, true)) {
      val main = QueuedDispatcher()
      val io = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + main)
      val presenter =
          presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
            when {
              path.contains("files/info?") ->
                  if (failure) TransportResponse(503, "") else response(fileJson("main.go", "base"))
              path.contains("files/symbols?") ->
                  response(symbolsJson("main.go", "Run", "func Run()", 2, 10))
              else -> error("Unexpected $path")
            }
          }
      try {
        val base = performancePageFixture()
        val report = base.results!!.performance.single()
        val updated =
            report.copy(
                findings =
                    report.findings +
                        report.findings.single().copy(id = "other", title = "Other opportunity"))
        val page =
            base.copy(
                section =
                    base.section.copy(results = base.results!!.copy(performance = listOf(updated))))
        presenter.dispatch(DesktopEvent.ProjectLoaded(resultProjectFixture(), resultIndexFixture()))
        presenter.dispatch(
            DesktopEvent.AnalysisRunUpdated(
                ProjectAnalysisRunState(
                    run = page.run,
                    sections = mapOf(AnalysisResultKey("performance") to page.section))))
        val loaded = presenter.snapshot.value.state.analysisResultPage("performance")
        val results = performanceResults(loaded)
        val selected = results.single { it.finding.id == "perf" }
        val other = results.single { it.finding.id == "other" }
        val browser = newResultBrowserState(loaded)
        browser.choose(selected.row().key)
        val guard = performanceSelectionGuard(loaded, browser, selected)
        presenter.preparePerformanceFinding(selected, guard)
        main.runNext() // File and symbol reads are suspended on I/O.
        io.runPending()
        browser.choose(other.row().key)
        browser.choose(selected.row().key) // Returning does not reinstate the old action.
        main.runPending()
        assertNull(presenter.snapshot.value.state.selectedFile)
        assertTrue(presenter.snapshot.value.state.preparedRequest.isBlank())
        assertNull(presenter.snapshot.value.state.error)

        presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
        val input = TextFieldValue("Keep my request")
        val constraints = TextFieldValue("Keep my constraints")
        var pending: PendingDraftDiscard.PerformancePreparation? = null
        routePerformancePreparationRequest(
            presenter,
            selected,
            input,
            constraints,
            performanceSelectionGuard(loaded, browser, selected),
            { input to constraints },
            { error("Selection change must not clear constraints") }) {
              pending = it
            }
        browser.choose(other.row().key)
        confirmPerformancePreparationDiscard(
            presenter,
            requireNotNull(pending),
            input,
            constraints,
            { input to constraints },
            { error("Selection change must not clear constraints") })
        assertEquals(draft(), presenter.snapshot.value.state.review.draft)
        assertTrue(presenter.snapshot.value.state.preparedRequest.isBlank())
      } finally {
        presenter.close()
        scope.cancel()
      }
    }
  }

  @Test
  fun typedPreparationIgnoresLateReadsAfterEvidenceOrSelectionChanges() {
    for (change in
        listOf("results", "index", "selection", "project", "file request", "late failure")) {
      val dispatcher = QueuedDispatcher()
      val io = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + dispatcher)
      val presenter =
          presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
            when {
              path.contains("files/info?") ->
                  if (change == "late failure") TransportResponse(503, "")
                  else response(fileJson("main.go", "base"))
              path.contains("files/symbols?") ->
                  response(symbolsJson("main.go", "Run", "func Run()", 2, 10))
              path.contains("files/analysis") ->
                  response("""{"path":"main.go","status":"missing"}""")
              path.contains("/impact") -> response("""{"target_path":"main.go"}""")
              path.contains("/git") -> response("""{"available":false}""")
              else -> error("Unexpected $path")
            }
          }
      try {
        val page = performancePageFixture()
        presenter.dispatch(DesktopEvent.ProjectLoaded(resultProjectFixture(), resultIndexFixture()))
        presenter.dispatch(
            DesktopEvent.AnalysisRunUpdated(
                ProjectAnalysisRunState(
                    run = page.run,
                    sections = mapOf(AnalysisResultKey("performance") to page.section))))
        val result =
            performanceResults(presenter.snapshot.value.state.analysisResultPage("performance"))
                .single()
        presenter.preparePerformanceFinding(result)
        dispatcher.runNext() // Suspend before the file read completes.
        when (change) {
          "late failure",
          "results" ->
              presenter.dispatch(
                  DesktopEvent.AnalysisRunUpdated(
                      ProjectAnalysisRunState(
                          run = page.run,
                          sections =
                              mapOf(
                                  AnalysisResultKey("performance") to
                                      page.section.copy(
                                          results =
                                              page.results!!.copy(performance = emptyList()))))))
          "index" ->
              presenter.dispatch(
                  DesktopEvent.IndexRefreshed(
                      resultIndexFixture()
                          .copy(
                              files =
                                  resultIndexFixture().files.map { it.copy(contentHash = "new") })))
          "selection" -> presenter.dispatch(DesktopEvent.WorkspaceSelected(Workspace.Bugs))
          "project" ->
              presenter.dispatch(
                  DesktopEvent.ProjectLoaded(
                      resultProjectFixture().copy(projectRevision = "next"),
                      resultIndexFixture().copy(projectRevision = "next")))
          "file request" -> presenter.selectFile("main.go")
        }
        dispatcher.runPending()
        io.runPending()
        dispatcher.runPending()
        assertTrue(presenter.snapshot.value.state.preparedRequest.isBlank(), change)
        assertNull(presenter.snapshot.value.state.error, change)
      } finally {
        presenter.close()
        scope.cancel()
      }
    }
  }

  @Test
  fun typedPreparationRejectsObsoleteAndAmbiguousResultsBeforeReading() {
    val dispatcher = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    var reads = 0
    val presenter =
        presenter(parentScope = scope, ioDispatcher = dispatcher) { _, _, _ ->
          reads++
          error("Unexpected read")
        }
    try {
      val page = performancePageFixture()
      presenter.dispatch(DesktopEvent.ProjectLoaded(resultProjectFixture(), resultIndexFixture()))
      presenter.dispatch(
          DesktopEvent.AnalysisRunUpdated(
              ProjectAnalysisRunState(
                  run = page.run,
                  sections = mapOf(AnalysisResultKey("performance") to page.section))))
      val result =
          performanceResults(presenter.snapshot.value.state.analysisResultPage("performance"))
              .single()
      val intent = requireNotNull(presenter.performancePreparationIntent(result))
      val duplicate =
          page.section.copy(
              results = page.results!!.copy(performance = listOf(result.report, result.report)))
      presenter.dispatch(
          DesktopEvent.AnalysisRunUpdated(
              ProjectAnalysisRunState(
                  run = page.run, sections = mapOf(AnalysisResultKey("performance") to duplicate))))
      assertFalse(presenter.confirmPerformancePreparationIntent(intent))
      presenter.preparePerformanceFinding(result)
      assertTrue(presenter.snapshot.value.state.error?.contains("ambiguous") == true)
      assertEquals(0, reads)
      assertNull(presenter.snapshot.value.state.selectedFile)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun typedPreparationRejectsMismatchedReadIdentityWithoutPublishing() {
    for (read in
        listOf(
            "file hash",
            "file path",
            "symbols project",
            "symbols revision",
            "symbols path",
            "signature")) {
      val dispatcher = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + dispatcher)
      val presenter =
          presenter(parentScope = scope, ioDispatcher = dispatcher) { _, path, _ ->
            when {
              path.contains("files/info?") ->
                  response(
                      fileJson(
                          if (read == "file path") "other.go" else "main.go",
                          if (read == "file hash") "new" else "base"))
              path.contains("files/symbols?") -> {
                val symbols =
                    symbolsJson(
                        if (read == "symbols path") "other.go" else "main.go",
                        "Run",
                        if (read == "signature") "func Run(int)" else "func Run()",
                        2,
                        10)
                response(
                    when (read) {
                      "symbols project" ->
                          symbols.replace("\"project_id\":\"project\"", "\"project_id\":\"other\"")
                      "symbols revision" ->
                          symbols.replace(
                              "\"project_revision\":\"revision\"", "\"project_revision\":\"next\"")
                      else -> symbols
                    })
              }
              else -> error("Unexpected $path")
            }
          }
      try {
        val page = performancePageFixture()
        presenter.dispatch(DesktopEvent.ProjectLoaded(resultProjectFixture(), resultIndexFixture()))
        presenter.dispatch(
            DesktopEvent.AnalysisRunUpdated(
                ProjectAnalysisRunState(
                    run = page.run,
                    sections = mapOf(AnalysisResultKey("performance") to page.section))))
        val result =
            performanceResults(presenter.snapshot.value.state.analysisResultPage("performance"))
                .single()
        presenter.preparePerformanceFinding(result)
        dispatcher.runPending()
        assertNull(presenter.snapshot.value.state.selectedFile, read)
        assertTrue(presenter.snapshot.value.state.preparedRequest.isBlank(), read)
        assertTrue(presenter.snapshot.value.state.error?.isNotBlank() == true, read)
      } finally {
        presenter.close()
        scope.cancel()
      }
    }
  }

  @Test
  fun typedPerformanceSourceRejectsIdenticalEvidenceFromAnotherRunAtApprovalAndDuringRead() {
    val dispatcher = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    val presenter =
        presenter(parentScope = scope, ioDispatcher = dispatcher) { _, path, _ ->
          when {
            path.contains("files/info?") -> response(fileJson("main.go", "base"))
            path.contains("files/symbols?") -> response(symbolsJson("main.go", "Run"))
            else -> error("Unexpected request $path")
          }
        }
    try {
      val page = performancePageFixture()
      val original =
          ProjectAnalysisRunState(
              run = page.run, sections = mapOf(AnalysisResultKey("performance") to page.section))
      val newIdentity = page.run!!.identity.copy(id = "replacement-run", generation = "replacement")
      val replacement =
          ProjectAnalysisRunState(
              run = page.run.copy(identity = newIdentity),
              sections =
                  mapOf(
                      AnalysisResultKey("performance") to
                          page.section.copy(results = page.results!!.copy(identity = newIdentity))))
      presenter.dispatch(DesktopEvent.ProjectLoaded(resultProjectFixture(), resultIndexFixture()))
      presenter.dispatch(DesktopEvent.AnalysisRunUpdated(original))
      val result =
          performanceResults(presenter.snapshot.value.state.analysisResultPage("performance"))
              .single()
      val intent = requireNotNull(presenter.performanceSourceIntent(result))
      presenter.dispatch(DesktopEvent.AnalysisRunUpdated(replacement))
      assertFalse(presenter.confirmPerformanceSourceIntent(intent))
      assertNull(presenter.performanceSourceIntent(result))
      assertNull(presenter.snapshot.value.state.selectedFile)

      presenter.dispatch(DesktopEvent.AnalysisRunUpdated(original))
      presenter.openPerformanceFinding(result)
      presenter.dispatch(DesktopEvent.AnalysisRunUpdated(replacement))
      dispatcher.runPending()
      assertNull(presenter.snapshot.value.state.selectedFile)
      assertTrue(presenter.snapshot.value.state.preparedRequest.isBlank())
      assertNull(presenter.snapshot.value.state.error)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun typedPerformanceSourceFailedReadRetainsComposerUntilSuccessfulConfirmedNavigation() {
    val dispatcher = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    var fail = true
    val presenter =
        presenter(parentScope = scope, ioDispatcher = dispatcher) { _, path, _ ->
          when {
            path.contains("files/info?") ->
                if (fail) TransportResponse(503, "") else response(fileJson("main.go", "base"))
            path.contains("files/symbols?") -> response(symbolsJson("main.go", "Run"))
            else -> error("Unexpected request $path")
          }
        }
    try {
      val page = performancePageFixture()
      presenter.dispatch(DesktopEvent.ProjectLoaded(resultProjectFixture(), resultIndexFixture()))
      presenter.dispatch(
          DesktopEvent.AnalysisRunUpdated(
              ProjectAnalysisRunState(
                  run = page.run,
                  sections = mapOf(AnalysisResultKey("performance") to page.section))))
      val result =
          performanceResults(presenter.snapshot.value.state.analysisResultPage("performance"))
              .single()
      var message = TextFieldValue("Keep my request")
      var constraints = TextFieldValue("Keep my constraints")
      var pending: PendingDraftDiscard.PerformanceSource? = null
      var clears = 0
      fun clear() {
        clears++
        message = TextFieldValue()
        constraints = TextFieldValue()
      }
      fun route() {
        routePerformanceSourceRequest(
            presenter,
            result,
            message,
            constraints,
            { true },
            { message to constraints },
            ::clear) {
              pending = it
            }
        val approval = requireNotNull(pending)
        pending = null
        confirmPerformanceSourceDiscard(
            presenter, approval, message, constraints, { message to constraints }, ::clear)
      }
      route()
      assertEquals("Keep my request", message.text)
      dispatcher.runPending()
      assertEquals("Keep my request", message.text)
      assertEquals("Keep my constraints", constraints.text)
      assertEquals(0, clears)
      assertNull(presenter.snapshot.value.state.selectedFile)
      assertTrue(presenter.snapshot.value.state.error != null)

      fail = false
      route()
      assertEquals(0, clears)
      dispatcher.runPending()
      assertEquals("main.go", presenter.snapshot.value.state.selectedFile?.path)
      assertEquals(1, clears)
      assertTrue(message.text.isEmpty())
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun typedPerformanceSourceInspectsRetainedEvidenceWithoutPreparingOrReplacingSameFileDraft() {
    val dispatcher = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    val calls = mutableListOf<String>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = dispatcher) { method, path, _ ->
          calls += "$method $path"
          when {
            path.contains("files/info?path=main.go") -> response(fileJson("main.go", "base"))
            path.contains("files/symbols?path=main.go") ->
                response(symbolsJson("main.go", "Run", "func Run()"))
            path.contains("files/analysis") -> response("""{"path":"main.go","status":"missing"}""")
            path.contains("/impact") -> response("""{"target_path":"main.go"}""")
            path.contains("/git") -> response("""{"available":false}""")
            else -> error("Unexpected $method $path")
          }
        }
    try {
      val page = performancePageFixture()
      presenter.dispatch(DesktopEvent.ProjectLoaded(resultProjectFixture(), resultIndexFixture()))
      presenter.dispatch(
          DesktopEvent.AnalysisRunUpdated(
              ProjectAnalysisRunState(
                  run = page.run,
                  sections = mapOf(AnalysisResultKey("performance") to page.section))))
      val result =
          performanceResults(presenter.snapshot.value.state.analysisResultPage("performance"))
              .single()
      presenter.openPerformanceFinding(result)
      dispatcher.runPending()
      assertEquals("main.go", presenter.snapshot.value.state.selectedFile?.path)
      assertEquals(4, presenter.snapshot.value.state.selection.focusedLine)
      assertNull(presenter.snapshot.value.state.selection.selectedSymbol)
      assertTrue(presenter.snapshot.value.state.preparedRequest.isBlank())
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      val review = presenter.snapshot.value.state.review
      val reads = calls.size
      presenter.openPerformanceFinding(result)
      dispatcher.runPending()
      assertEquals(reads, calls.size)
      assertEquals(review, presenter.snapshot.value.state.review)
      assertTrue(calls.none { it.startsWith("POST") })
      presenter.dispatch(
          DesktopEvent.AnalysisRunUpdated(
              ProjectAnalysisRunState(
                  run = page.run!!.copy(status = "stale"),
                  sections = mapOf(AnalysisResultKey("performance") to page.section))))
      val stale =
          performanceResults(presenter.snapshot.value.state.analysisResultPage("performance"))
              .single()
      assertTrue(stale.stale)
      presenter.openPerformanceFinding(stale)
      assertEquals(review, presenter.snapshot.value.state.review)
      assertEquals(4, presenter.snapshot.value.state.selection.focusedLine)
      assertEquals(reads, calls.size)
      val withoutAnchor = stale.report.copy(findings = listOf(stale.finding.copy(startLine = 0)))
      presenter.dispatch(
          DesktopEvent.AnalysisRunUpdated(
              ProjectAnalysisRunState(
                  run = page.run.copy(status = "stale"),
                  sections =
                      mapOf(
                          AnalysisResultKey("performance") to
                              page.section.copy(
                                  results =
                                      page.results!!.copy(performance = listOf(withoutAnchor)))))))
      val anchorless =
          performanceResults(presenter.snapshot.value.state.analysisResultPage("performance"))
              .single()
      presenter.openPerformanceFinding(anchorless)
      assertEquals(0, presenter.snapshot.value.state.selection.focusedLine)
      assertNull(presenter.snapshot.value.state.selection.selectedSymbol)
      assertEquals(reads, calls.size)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun typedPerformanceSourceRejectsMissingAmbiguousAndChangedApproval() {
    val calls = mutableListOf<String>()
    val presenter = presenter { method, path, _ ->
      calls += "$method $path"
      error("Unexpected request $method $path")
    }
    try {
      val page = performancePageFixture()
      presenter.dispatch(DesktopEvent.ProjectLoaded(resultProjectFixture(), resultIndexFixture()))
      presenter.dispatch(
          DesktopEvent.AnalysisRunUpdated(
              ProjectAnalysisRunState(
                  run = page.run,
                  sections = mapOf(AnalysisResultKey("performance") to page.section))))
      val result =
          performanceResults(presenter.snapshot.value.state.analysisResultPage("performance"))
              .single()
      val intent = requireNotNull(presenter.performanceSourceIntent(result))
      val duplicate =
          page.section.copy(
              results = page.results!!.copy(performance = listOf(result.report, result.report)))
      presenter.dispatch(
          DesktopEvent.AnalysisRunUpdated(
              ProjectAnalysisRunState(
                  run = page.run, sections = mapOf(AnalysisResultKey("performance") to duplicate))))
      assertFalse(presenter.confirmPerformanceSourceIntent(intent))
      assertNull(presenter.snapshot.value.state.selectedFile)
      presenter.openPerformanceFinding(result)
      assertTrue(presenter.snapshot.value.state.error?.contains("indexed source") == true)
      assertTrue(calls.isEmpty())
    } finally {
      presenter.close()
    }
  }

  @Test
  fun typedPerformanceSourceDropsLateSuccessAndFailureAfterSelectionOrIndexChanges() {
    for (change in listOf("selection", "selection-failure", "index", "index-failure")) {
      val main = QueuedDispatcher()
      val io = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + main)
      var loaded = 0
      val presenter =
          presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
            when {
              path.contains("files/info?") ->
                  if (change.endsWith("failure")) TransportResponse(503, "")
                  else response(fileJson("main.go", "base"))
              path.contains("files/symbols?") -> response(symbolsJson("main.go", "Run"))
              else -> error("Unexpected request $path")
            }
          }
      try {
        val page = performancePageFixture()
        val index = resultIndexFixture()
        presenter.dispatch(DesktopEvent.ProjectLoaded(resultProjectFixture(), index))
        presenter.dispatch(
            DesktopEvent.AnalysisRunUpdated(
                ProjectAnalysisRunState(
                    run = page.run,
                    sections = mapOf(AnalysisResultKey("performance") to page.section))))
        val currentPage = presenter.snapshot.value.state.analysisResultPage("performance")
        val result = performanceResults(currentPage).single()
        val browser = newResultBrowserState(currentPage)
        browser.selectedKey = result.row().key
        val guard = performanceSelectionGuard(currentPage, browser, result)
        presenter.openPerformanceFinding(result, guard) { loaded++ }
        main.runPending()
        if (change.startsWith("selection")) browser.selectedKey = "different-row"
        else
            presenter.dispatch(
                DesktopEvent.IndexRefreshed(
                    index.copy(files = index.files.map { it.copy(contentHash = "changed") })))
        io.runPending()
        main.runPending()
        val state = presenter.snapshot.value.state
        assertNull(state.selectedFile, change)
        assertNull(state.selection.fileReadError, change)
        assertNull(state.jobs.error, change)
        assertTrue(state.preparedRequest.isBlank(), change)
        assertEquals(0, loaded, change)
      } finally {
        presenter.close()
        scope.cancel()
      }
    }
  }

  @Test
  fun typedPerformanceSourceDropsLateReadsAfterResultReplacement() {
    val dispatcher = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    val presenter =
        presenter(parentScope = scope, ioDispatcher = dispatcher) { _, path, _ ->
          when {
            path.contains("files/info?") -> response(fileJson("main.go", "base"))
            path.contains("files/symbols?") -> response(symbolsJson("main.go", "Run"))
            else -> error("Unexpected request $path")
          }
        }
    try {
      val page = performancePageFixture()
      presenter.dispatch(DesktopEvent.ProjectLoaded(resultProjectFixture(), resultIndexFixture()))
      presenter.dispatch(
          DesktopEvent.AnalysisRunUpdated(
              ProjectAnalysisRunState(
                  run = page.run,
                  sections = mapOf(AnalysisResultKey("performance") to page.section))))
      val result =
          performanceResults(presenter.snapshot.value.state.analysisResultPage("performance"))
              .single()
      presenter.openPerformanceFinding(result)
      presenter.dispatch(
          DesktopEvent.AnalysisRunUpdated(
              ProjectAnalysisRunState(
                  run = page.run,
                  sections =
                      mapOf(
                          AnalysisResultKey("performance") to
                              page.section.copy(
                                  results = page.results!!.copy(performance = emptyList()))))))
      dispatcher.runPending()
      assertNull(presenter.snapshot.value.state.selectedFile)
      assertTrue(presenter.snapshot.value.state.preparedRequest.isBlank())
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun typedPerformanceSourceRejectsMissingAndAmbiguousIndexedPathsWithoutReads() {
    for (files in
        listOf(
            emptyList(),
            listOf(resultIndexFixture().files.single(), resultIndexFixture().files.single()))) {
      val calls = mutableListOf<String>()
      val presenter = presenter { method, path, _ ->
        calls += "$method $path"
        error("Unexpected request $method $path")
      }
      try {
        val page = performancePageFixture()
        presenter.dispatch(
            DesktopEvent.ProjectLoaded(
                resultProjectFixture(), resultIndexFixture().copy(files = files)))
        presenter.dispatch(
            DesktopEvent.AnalysisRunUpdated(
                ProjectAnalysisRunState(
                    run = page.run,
                    sections = mapOf(AnalysisResultKey("performance") to page.section))))
        val result =
            performanceResults(presenter.snapshot.value.state.analysisResultPage("performance"))
                .single()
        assertNull(presenter.performanceSourceIntent(result))
        presenter.openPerformanceFinding(result)
        assertTrue(presenter.snapshot.value.state.error?.contains("indexed source") == true)
        assertNull(presenter.snapshot.value.state.selectedFile)
        assertTrue(calls.isEmpty())
      } finally {
        presenter.close()
      }
    }
  }

  @Test
  fun typedPerformanceSourceConfirmationPreservesDraftWhenBufferChanges() {
    val dispatcher = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    val calls = mutableListOf<String>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = dispatcher) { method, path, _ ->
          calls += "$method $path"
          when {
            path.contains("files/info?path=other.go") -> response(fileJson("other.go", "base"))
            path.contains("files/symbols?path=other.go") -> response(symbolsJson("other.go", "Run"))
            path.contains("files/info?") -> response(fileJson("main.go", "base"))
            path.contains("files/symbols?") -> response(symbolsJson("main.go", "Run"))
            path.contains("files/analysis") -> response("""{"path":"main.go","status":"missing"}""")
            path.contains("/impact") -> response("""{"target_path":"main.go"}""")
            path.contains("/git") -> response("""{"available":false}""")
            else -> error("Unexpected $method $path")
          }
        }
    try {
      val original = performancePageFixture()
      val page =
          original.copy(
              section =
                  original.section.copy(
                      results =
                          original.results!!.copy(
                              performance =
                                  listOf(
                                      original.results!!
                                          .performance
                                          .single()
                                          .copy(path = "other.go")))))
      val index = resultIndexFixture()
      presenter.dispatch(
          DesktopEvent.ProjectLoaded(
              resultProjectFixture(),
              index.copy(files = index.files + index.files.single().copy(path = "other.go"))))
      presenter.dispatch(
          DesktopEvent.AnalysisRunUpdated(
              ProjectAnalysisRunState(
                  run = page.run,
                  sections = mapOf(AnalysisResultKey("performance") to page.section))))
      val result =
          performanceResults(presenter.snapshot.value.state.analysisResultPage("performance"))
              .single()
      presenter.selectFile("main.go")
      dispatcher.runPending()
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      val intent = requireNotNull(presenter.performanceSourceIntent(result))
      presenter.dispatch(DesktopEvent.DraftEdited(declaration = "func Run() { changed() }"))
      val review = presenter.snapshot.value.state.review
      val reads = calls.size
      assertFalse(presenter.confirmPerformanceSourceIntent(intent))
      assertEquals(review, presenter.snapshot.value.state.review)
      assertEquals(reads, calls.size)
      assertEquals("main.go", presenter.snapshot.value.state.selectedFile?.path)
      presenter.openPerformanceFinding(result)
      assertEquals(reads, calls.size)
      assertEquals(review, presenter.snapshot.value.state.review)
      assertTrue(presenter.snapshot.value.state.error?.contains("discarding") == true)
      assertTrue(
          presenter.confirmPerformanceSourceIntent(
              requireNotNull(presenter.performanceSourceIntent(result))))
      dispatcher.runPending()
      assertEquals("other.go", presenter.snapshot.value.state.selectedFile?.path)
      assertNull(presenter.snapshot.value.state.review.draft)
      assertEquals(4, presenter.snapshot.value.state.selection.focusedLine)
      assertTrue(presenter.snapshot.value.state.preparedRequest.isBlank())
      assertTrue(calls.none { it.startsWith("POST") })
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun semanticSourceInspectionUsesTheLoadedCategoryWithoutPreparing() {
    for (category in listOf("bugs", "performance", "security")) {
      val dispatcher = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + dispatcher)
      val calls = mutableListOf<String>()
      val presenter =
          presenter(parentScope = scope, ioDispatcher = dispatcher) { method, path, _ ->
            calls += "$method $path"
            when {
              path.contains("files/info?path=main.go") -> response(fileJson("main.go", "base"))
              path.contains("files/symbols?path=main.go") ->
                  response(symbolsJson("main.go", "Run", "func Run()"))
              path.contains("files/analysis") ->
                  response("""{"path":"main.go","status":"missing"}""")
              path.contains("/impact") -> response("""{"target_path":"main.go"}""")
              path.contains("/git") -> response("""{"available":false}""")
              else -> error("Unexpected $method $path")
            }
          }
      try {
        val run = analysisRunFixture()
        val finding =
            UnifiedFinding(
                id = "semantic-$category",
                category = category,
                projectId = "project",
                projectRevision = "revision",
                location = FindingLocation("main.go", startLine = 17, symbol = "Run"))
        presenter.dispatch(DesktopEvent.ProjectLoaded(resultProjectFixture(), resultIndexFixture()))
        presenter.dispatch(
            DesktopEvent.AnalysisRunUpdated(
                ProjectAnalysisRunState(
                    run = run,
                    sections =
                        mapOf(
                            AnalysisResultKey(category) to
                                AnalysisSectionState(
                                    results =
                                        analysisResultsFixture(run, category)
                                            .copy(semantic = listOf(finding)))))))
        presenter.openFinding(finding)
        dispatcher.runPending()
        assertEquals("main.go", presenter.snapshot.value.state.selectedFile?.path, category)
        assertEquals(17, presenter.snapshot.value.state.selection.focusedLine, category)
        assertTrue(presenter.snapshot.value.state.preparedRequest.isBlank(), category)
        assertTrue(calls.any { it.contains("files/info?path=main.go") }, category)
        assertTrue(calls.none { it.startsWith("POST") }, category)
      } finally {
        presenter.close()
        scope.cancel()
      }
    }
  }

  @Test
  fun semanticPerformanceSourceRejectsMissingDuplicateAndWrongProjectFindings() {
    for (case in listOf("missing", "duplicate", "wrong-project")) {
      val calls = mutableListOf<String>()
      val presenter = presenter { method, path, _ ->
        calls += "$method $path"
        error("Unexpected request $method $path")
      }
      try {
        val run = analysisRunFixture()
        val finding =
            UnifiedFinding(
                id = "performance-source",
                category = "performance",
                projectId = "project",
                projectRevision = "revision",
                location = FindingLocation("main.go", startLine = 17, symbol = "Run"))
        val loaded =
            when (case) {
              "missing" -> emptyList()
              "duplicate" -> listOf(finding, finding)
              else -> listOf(finding)
            }
        val requested =
            if (case == "wrong-project") finding.copy(projectId = "another-project") else finding
        presenter.dispatch(DesktopEvent.ProjectLoaded(resultProjectFixture(), resultIndexFixture()))
        presenter.dispatch(
            DesktopEvent.AnalysisRunUpdated(
                ProjectAnalysisRunState(
                    run = run,
                    sections =
                        mapOf(
                            AnalysisResultKey("performance") to
                                AnalysisSectionState(
                                    results =
                                        analysisResultsFixture(run, "performance")
                                            .copy(semantic = loaded))))))
        assertNull(presenter.findingIntent(requested, false), case)
        presenter.openFinding(requested)
        assertNull(presenter.snapshot.value.state.selectedFile, case)
        assertTrue(presenter.snapshot.value.state.error?.contains("no longer") == true, case)
        assertTrue(calls.isEmpty(), case)
      } finally {
        presenter.close()
      }
    }
  }

  @Test
  fun semanticPreparationUsesTheDisplayedCategoryAndRejectsRemovedEvidence() {
    for (category in listOf("bugs", "performance", "security")) {
      val dispatcher = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + dispatcher)
      val calls = mutableListOf<String>()
      val presenter =
          presenter(parentScope = scope, ioDispatcher = dispatcher) { method, path, _ ->
            calls += "$method $path"
            when {
              path.contains("files/info?path=main.go") -> response(fileJson("main.go", "base"))
              path.contains("files/symbols?path=main.go") ->
                  response(symbolsJson("main.go", "Run", "func Run()"))
              path.contains("files/analysis") ->
                  response("""{"path":"main.go","status":"missing"}""")
              path.contains("/impact") -> response("""{"target_path":"main.go"}""")
              path.contains("/git") -> response("""{"available":false}""")
              else -> error("Unexpected $method $path")
            }
          }
      try {
        val project = resultProjectFixture()
        val index = resultIndexFixture()
        val run = analysisRunFixture()
        val finding =
            UnifiedFinding(
                id = "semantic-$category",
                category = category,
                projectId = project.projectId,
                projectRevision = project.projectRevision,
                fileHash = "base",
                freshness = "fresh",
                location = FindingLocation("main.go", symbol = "Run"),
                taskSpec = BugTaskSpec("1", "main.go", "Run", "func Run()", listOf("Fix Run.")))
        val key = AnalysisResultKey(category)
        val results = analysisResultsFixture(run, category).copy(semantic = listOf(finding))
        presenter.dispatch(DesktopEvent.ProjectLoaded(project, index))
        presenter.dispatch(
            DesktopEvent.AnalysisRunUpdated(
                ProjectAnalysisRunState(
                    run = run, sections = mapOf(key to AnalysisSectionState(results = results)))))
        val state = presenter.snapshot.value.state
        val displayed =
            if (category == "bugs") state.projectBugFindings()
            else state.analysisResultPage(category).semantic
        assertTrue(
            findingPreparationDecision(finding, state.project, displayed, state.index)
                is FindingPreparationDecision.Eligible,
            category)
        presenter.prepareFinding(finding)
        dispatcher.runPending()
        assertTrue(presenter.snapshot.value.state.preparedRequest.contains("Fix Run."), category)
        assertEquals("main.go", presenter.snapshot.value.state.selectedFile?.path, category)
        assertTrue(calls.any { it.contains("files/info?path=main.go") }, category)
        assertTrue(calls.none { it.startsWith("POST") }, category)
        val requestsBeforeMismatch = calls.size
        presenter.prepareFinding(finding.copy(fileHash = "different"))
        assertTrue(presenter.snapshot.value.state.error?.contains("no longer") == true, category)
        assertEquals(requestsBeforeMismatch, calls.size, category)

        presenter.dispatch(
            DesktopEvent.AnalysisRunUpdated(
                ProjectAnalysisRunState(
                    run = run,
                    sections =
                        mapOf(
                            key to
                                AnalysisSectionState(
                                    results = results.copy(semantic = emptyList()))))))
        val requestsBefore = calls.size
        presenter.prepareFinding(finding)
        assertTrue(presenter.snapshot.value.state.error?.contains("no longer") == true, category)
        assertEquals(requestsBefore, calls.size, category)
      } finally {
        presenter.close()
        scope.cancel()
      }
    }
  }

  @Test
  fun preparationRejectsLoadedSourceThatDiffersFromIndexedEvidence() {
    for (variant in
        listOf("hash", "signature", "missing", "ambiguous", "inexact", "path", "failure")) {
      val calls = Collections.synchronizedList(mutableListOf<String>())
      val presenter = presenter { method, path, _ ->
        calls += "$method $path"
        when {
          path.contains("files/info?path=main.go") ->
              if (variant == "failure") TransportResponse(503, "")
              else response(fileJson("main.go", if (variant == "hash") "changed" else "base"))
          path.contains("files/symbols?path=main.go") ->
              response(
                  when (variant) {
                    "path" -> symbolsJson("other.go", "Run", "func Run()")
                    "missing" -> symbolsJson("main.go")
                    "signature" -> symbolsJson("main.go", "Run", "func Changed()")
                    "ambiguous" ->
                        symbolsJson("main.go", "Run", "func Run()")
                            .replace(
                                "\"symbols\":[{",
                                "\"symbols\":[{\"name\":\"Run\",\"signature\":\"func Run()\",\"kind\":\"function\",\"confidence\":\"exact\",\"atomic_target\":true},{")
                    "inexact" ->
                        symbolsJson("main.go", "Run", "func Run()")
                            .replace("\"confidence\":\"exact\"", "\"confidence\":\"approximate\"")
                    else -> symbolsJson("main.go", "Run", "func Run()")
                  })
          path.contains("files/analysis") -> response("""{"path":"main.go","status":"missing"}""")
          path.contains("/impact") -> response("""{"target_path":"main.go"}""")
          path.contains("/git") -> response("""{"available":false}""")
          else -> error("Unexpected $method $path")
        }
      }
      try {
        val project = resultProjectFixture()
        val run = analysisRunFixture()
        val finding =
            UnifiedFinding(
                id = "bug",
                category = "bugs",
                projectId = project.projectId,
                projectRevision = project.projectRevision,
                fileHash = "base",
                freshness = "fresh",
                location = FindingLocation("main.go", symbol = "Run"),
                taskSpec = BugTaskSpec("1", "main.go", "Run", "func Run()", listOf("Fix Run.")))
        presenter.dispatch(DesktopEvent.ProjectLoaded(project, resultIndexFixture()))
        presenter.dispatch(
            DesktopEvent.AnalysisRunUpdated(
                ProjectAnalysisRunState(
                    run = run,
                    sections =
                        mapOf(
                            AnalysisResultKey("bugs") to
                                AnalysisSectionState(
                                    results =
                                        analysisResultsFixture(run, "bugs")
                                            .copy(semantic = listOf(finding)))))))
        presenter.prepareFinding(finding)
        eventually { presenter.snapshot.value.state.jobs.error != null }
        assertTrue(presenter.snapshot.value.state.preparedRequest.isBlank(), variant)
        assertTrue(calls.none { it.startsWith("POST") }, variant)
        if (variant == "failure" || variant == "path")
            assertTrue(presenter.snapshot.value.state.selection.fileReadError != null, variant)
      } finally {
        presenter.close()
      }
    }
  }

  @Test
  fun latePreparationCannotPublishAfterEvidenceOrSelectionChanges() {
    for (change in
        listOf(
            "evidence",
            "evidence-failure",
            "evidence-replaced",
            "selection",
            "selection-failure",
            "replacement")) {
      val main = QueuedDispatcher()
      val io = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + main)
      val presenter =
          presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
            when {
              path.contains("files/info?path=main.go") ->
                  if (change.endsWith("failure")) TransportResponse(503, "")
                  else response(fileJson("main.go", "base"))
              path.contains("files/symbols?path=main.go") ->
                  response(symbolsJson("main.go", "Run", "func Run()"))
              path.contains("files/analysis") ->
                  response("""{"path":"main.go","status":"missing"}""")
              path.contains("/impact") -> response("""{"target_path":"main.go"}""")
              path.contains("/git") -> response("""{"available":false}""")
              else -> error("Unexpected $path")
            }
          }
      try {
        val project = resultProjectFixture()
        val run = analysisRunFixture()
        val finding =
            UnifiedFinding(
                id = "bug",
                category = "bugs",
                projectId = project.projectId,
                projectRevision = project.projectRevision,
                fileHash = "base",
                freshness = "fresh",
                location = FindingLocation("main.go", symbol = "Run"),
                taskSpec = BugTaskSpec("1", "main.go", "Run", "func Run()", listOf("Fix Run.")))
        presenter.dispatch(DesktopEvent.ProjectLoaded(project, resultIndexFixture()))
        val key = AnalysisResultKey("bugs")
        val results = analysisResultsFixture(run, "bugs")
        presenter.dispatch(
            DesktopEvent.AnalysisRunUpdated(
                ProjectAnalysisRunState(
                    run = run,
                    sections =
                        mapOf(
                            key to
                                AnalysisSectionState(
                                    results = results.copy(semantic = listOf(finding)))))))
        presenter.prepareFinding(finding)
        main.runPending()
        io.runPending()
        when (change) {
          "evidence",
          "evidence-failure",
          "evidence-replaced" ->
              presenter.dispatch(
                  DesktopEvent.AnalysisRunUpdated(
                      ProjectAnalysisRunState(
                          run = run,
                          sections =
                              mapOf(
                                  key to
                                      AnalysisSectionState(
                                          results =
                                              results.copy(
                                                  semantic =
                                                      if (change == "evidence-replaced")
                                                          listOf(finding.copy(fileHash = "new"))
                                                      else emptyList()))))))
          "selection",
          "selection-failure" -> presenter.dispatch(DesktopEvent.WorkspaceSelected(Workspace.Bugs))
          else -> presenter.selectFile("main.go")
        }
        main.runPending()
        io.runPending()
        main.runPending()
        val state = presenter.snapshot.value.state
        assertTrue(state.preparedRequest.isBlank(), change)
        if (change.startsWith("selection") || change.startsWith("evidence")) {
          assertFalse(state.jobs.loading, change)
          assertNull(state.jobs.error, change)
          assertNull(state.selection.fileReadError, change)
          assertNull(state.selectedFile, change)
          assertNull(presenter.snapshot.value.state.selection.selectedSymbol, change)
          assertEquals(
              if (change.startsWith("evidence")) Workspace.Editor else Workspace.Bugs,
              state.workspace,
              change)
          if (change == "evidence-replaced")
              assertEquals("new", state.projectBugFindings().single().fileHash)
          else if (change.startsWith("evidence")) assertTrue(state.projectBugFindings().isEmpty())
        } else if (change == "replacement") {
          assertEquals("main.go", state.selectedFile?.path)
          assertFalse(state.jobs.loading)
          assertNull(state.jobs.error)
        }
      } finally {
        presenter.close()
        scope.cancel()
      }
    }
  }

  @Test
  fun failedReindexRetainsTheProjectRunWithoutStartingAnotherAnalysis() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val calls = mutableListOf<Pair<String, String>>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { method, path, _ ->
          calls.add(method to path)
          when {
            path.endsWith("/reindex") ->
                TransportResponse(500, """{"message":"index unavailable"}""")
            path.contains("/analysis/run?") ->
                response(
                    kotlinx.serialization.json.Json.encodeToString(
                        AnalysisRun.serializer(), analysisRunFixture()))
            path.contains("/analysis/results?") -> {
              val category = path.substringAfter("category=")
              response(
                  kotlinx.serialization.json.Json.encodeToString(
                      AnalysisSectionResults.serializer(),
                      analysisResultsFixture(analysisRunFixture(), category)))
            }
            else -> error("Unexpected $method $path")
          }
        }
    try {
      loadProject(presenter)
      presenter.dispatch(
          DesktopEvent.AnalysisRunUpdated(ProjectAnalysisRunState(run = analysisRunFixture())))
      presenter.reindexProject()
      repeat(8) {
        main.runPending()
        io.runPending()
      }
      main.runPending()
      assertEquals("paused", presenter.snapshot.value.state.analysisRun.run?.status)
      assertEquals(
          ProjectIndexingOutcome.Failed("index unavailable"),
          presenter.snapshot.value.state.projectState.indexingAttempt?.outcome)
      assertEquals("revision", presenter.snapshot.value.state.project?.projectRevision)
      assertEquals(
          listOf("POST" to "/api/projects/current/reindex"), calls.filter { it.first == "POST" })
      assertTrue(calls.any { it.first == "GET" && it.second.contains("/analysis/run?") })
      assertTrue(calls.none { it.first == "POST" && it.second.contains("/analysis/") })
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun unsuccessfulReindexResumesPollingTheExistingDaemonRun() {
    for (result in listOf("failed", "canceled", "mismatched")) {
      val run = analysisRunFixture().copy(status = "running")
      val polls = AtomicInteger()
      val posts = AtomicInteger()
      val presenter = presenter { method, path, _ ->
        when {
          method == "POST" && path.endsWith("/reindex") -> {
            posts.incrementAndGet()
            when (result) {
              "failed" -> TransportResponse(500, """{"message":"index unavailable"}""")
              "canceled" -> throw kotlinx.coroutines.CancellationException("request canceled")
              else -> response("""{"project_id":"other","project_revision":"revision"}""")
            }
          }
          path.contains("/analysis/run?") -> {
            val status = if (polls.incrementAndGet() == 1) "running" else "paused"
            response(
                kotlinx.serialization.json.Json.encodeToString(
                    AnalysisRun.serializer(), run.copy(status = status)))
          }
          path.contains("/analysis/results?") -> {
            val category = path.substringAfter("category=").substringBefore("&")
            response(
                kotlinx.serialization.json.Json.encodeToString(
                    AnalysisSectionResults.serializer(), analysisResultsFixture(run, category)))
          }
          path.contains("/analysis/selection?") -> response("{}")
          else -> error("Unexpected $method $path")
        }
      }
      try {
        loadProject(presenter)
        presenter.dispatch(DesktopEvent.AnalysisRunUpdated(ProjectAnalysisRunState(run = run)))
        presenter.reindexProject()
        eventually {
          polls.get() >= 2 && presenter.snapshot.value.state.analysisRun.run?.status == "paused"
        }
        assertEquals(1, posts.get(), result)
        assertEquals("revision", presenter.snapshot.value.state.project?.projectRevision, result)
        val outcome = presenter.snapshot.value.state.projectState.indexingAttempt?.outcome
        if (result == "canceled") assertEquals(ProjectIndexingOutcome.Canceled, outcome)
        else assertTrue(outcome is ProjectIndexingOutcome.Failed, result)
      } finally {
        presenter.close()
      }
    }
  }

  @Test
  fun reindexAdmitsOneRequestWithCapturedRevisionAndOnlyReadsAfterAcceptance() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val calls = mutableListOf<Triple<String, String, String?>>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { method, path, body ->
          calls += Triple(method, path, body)
          when {
            path.endsWith("/reindex") -> response(indexJson())
            path.contains("/analysis/run?") || path.contains("/scan?") -> TransportResponse(204, "")
            path.contains("/analysis/selection?") -> response("{}")
            path.contains("/overview?") || path.contains("/findings?") -> response("{}")
            else -> error("Unexpected $method $path")
          }
        }
    try {
      presenter.reindexProject()
      assertTrue(calls.isEmpty())
      loadProject(presenter)
      presenter.reindexProject()
      val attempt = presenter.snapshot.value.state.projectState.indexingAttempt!!
      presenter.reindexProject()
      assertEquals(attempt, presenter.snapshot.value.state.projectState.indexingAttempt)
      main.runPending()
      io.runPending()
      assertEquals(1, calls.size)
      assertEquals("POST", calls.single().first)
      assertEquals("/api/projects/current/reindex", calls.single().second)
      assertEquals("{\"project_revision\":\"revision\"}", calls.single().third)
      main.runPending()
      assertEquals(
          ProjectIndexingOutcome.Succeeded("revision"),
          presenter.snapshot.value.state.projectState.indexingAttempt?.outcome)
      repeat(5) {
        io.runPending()
        main.runPending()
      }
      assertTrue(calls.drop(1).all { it.first == "GET" })
      assertTrue(
          calls.none {
            it.second.contains("/preview") ||
                it.second.contains("/execute") ||
                it.second.contains("/apply")
          })
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun refreshedInventoryKeepsDetailReadFailureSeparateAndSelectionReadVisible() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val calls = mutableListOf<String>()
    var unavailable = true
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { method, path, _ ->
          calls += "$method $path"
          when {
            path.endsWith("/reindex") -> response(indexJson())
            path.contains("/analysis/run?") || path.contains("/scan?") -> TransportResponse(204, "")
            path.contains("/analysis/selection?") ->
                if (unavailable) TransportResponse(500, "selection unavailable")
                else response(kotlinx.serialization.json.Json.encodeToString(selectionFixture()))
            path.contains("/overview?") -> response("{}")
            path.contains("/findings?") ->
                if (unavailable) TransportResponse(500, "findings unavailable") else response("{}")
            else -> error("Unexpected $method $path")
          }
        }
    try {
      loadProject(presenter)
      val retained =
          listOf(
              UnifiedFinding(
                  id = "saved",
                  category = "bugs",
                  projectId = "project",
                  projectRevision = "revision",
                  location = FindingLocation("main.go")))
      presenter.dispatch(DesktopEvent.FindingsLoaded(retained))
      presenter.reindexProject()
      repeat(12) {
        main.runPending()
        io.runPending()
      }
      main.runPending()
      val state = presenter.snapshot.value.state
      assertEquals(
          ProjectIndexingOutcome.Succeeded("revision"), state.projectState.indexingAttempt?.outcome)
      assertEquals(ProjectIndex("project", "revision"), state.index)
      assertEquals(retained, state.findings.findings)
      assertTrue(state.projectState.detailsOutcome is ProjectDetailsOutcome.Unavailable)
      assertEquals(AnalysisSelectionFailure.Read, state.analysisRun.fileSelection.failure)
      assertTrue(state.analysisRun.fileSelection.error?.isNotBlank() == true)
      assertTrue(calls.any { it.contains("/analysis/selection?") })
      assertTrue(calls.any { it.contains("/overview?") })
      assertTrue(calls.any { it.contains("/findings?") })
      assertTrue(calls.none { it.startsWith("POST") && !it.endsWith("/reindex") })
      unavailable = false
      presenter.reindexProject()
      main.runPending()
      io.runPending()
      main.runPending()
      assertEquals(
          ProjectDetailsOutcome.Refreshing,
          presenter.snapshot.value.state.projectState.detailsOutcome)
      repeat(12) {
        main.runPending()
        io.runPending()
      }
      main.runPending()
      val recovered = presenter.snapshot.value.state
      assertEquals(ProjectDetailsOutcome.Available, recovered.projectState.detailsOutcome)
      assertEquals("project", recovered.analysisRun.fileSelection.selection?.projectId)
      assertEquals(emptyList(), recovered.analysisRun.fileSelection.selection?.excludedPaths)
      assertTrue(calls.none { it.startsWith("POST") && !it.endsWith("/reindex") })
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun verifiedScanActionSettlesSupersededDetailRefreshAndRejectsLateRead() {
    for (failRead in listOf(false, true)) {
      val main = QueuedDispatcher()
      val io = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + main)
      val calls = mutableListOf<String>()
      val presenter =
          presenter(parentScope = scope, ioDispatcher = io) { method, path, _ ->
            calls += "$method $path"
            when {
              path.endsWith("/reindex") -> response(indexJson())
              path.contains("/analysis/run?") || path.contains("/scan?") ->
                  TransportResponse(204, "")
              path.contains("/analysis/selection?") -> response("{}")
              path.contains("/overview?") -> response("{}")
              path.contains("/findings?") ->
                  if (failRead) TransportResponse(503, "read failed") else response("{}")
              else -> error("Unexpected $method $path")
            }
          }
      try {
        loadProject(presenter)
        val retained =
            listOf(
                UnifiedFinding(
                    id = "saved",
                    category = "bugs",
                    projectId = "project",
                    projectRevision = "revision",
                    location = FindingLocation("main.go")))
        presenter.dispatch(DesktopEvent.FindingsLoaded(retained))
        presenter.reindexProject()
        main.runPending()
        io.runPending()
        main.runPending()
        assertEquals(
            ProjectDetailsOutcome.Refreshing,
            presenter.snapshot.value.state.projectState.detailsOutcome)
        assertTrue(calls.none { it.contains("/findings?") })

        presenter.dispatch(DesktopEvent.VerifiedScanReadUpdated(VerifiedScanRead.Absent))
        presenter.runVerifiedScan()
        val interrupted = presenter.snapshot.value.state.projectState.detailsOutcome
        assertTrue(interrupted is ProjectDetailsOutcome.Unavailable)
        assertTrue(interrupted.message.contains("Verified scan"))
        io.runPending()
        main.runPending()
        assertEquals(interrupted, presenter.snapshot.value.state.projectState.detailsOutcome)
        assertEquals(retained, presenter.snapshot.value.state.findings.findings)
        assertEquals(null, presenter.snapshot.value.state.projectState.overview)
        assertEquals(
            ProjectIndexingOutcome.Succeeded("revision"),
            presenter.snapshot.value.state.projectState.indexingAttempt?.outcome)
        assertTrue(calls.any { it.contains("/findings?") })
        assertTrue(calls.none { it.startsWith("POST") && !it.endsWith("/reindex") })
      } finally {
        presenter.close()
        scope.cancel()
      }
    }
  }

  @Test
  fun reindexOpeningLateResultsAndSameRevisionReplacementCannotPublish() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val calls = mutableListOf<String>()
    var fail = false
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { method, path, _ ->
          calls += "$method $path"
          if (path.endsWith("/reindex")) {
            if (fail) throw IllegalStateException(" ")
            response(indexJson())
          } else error("Unexpected $method $path")
        }
    try {
      loadProject(presenter)
      presenter.dispatch(
          DesktopEvent.ProjectOpeningStarted(
              ProjectOpeningAttempt(1, "/tmp/other", ProjectOpeningKind.Import)))
      presenter.reindexProject()
      assertTrue(calls.isEmpty())
      presenter.dispatch(DesktopEvent.ProjectOpeningCanceled(1))
      presenter.reindexProject()
      val old = presenter.snapshot.value.state.projectState.indexingAttempt!!
      main.runPending()
      io.runPending()
      presenter.dispatch(
          DesktopEvent.ProjectOpeningStarted(
              ProjectOpeningAttempt(2, "/tmp/project", ProjectOpeningKind.Import)))
      presenter.dispatch(DesktopEvent.ProjectLoaded(project(), ProjectIndex("project", "revision")))
      presenter.reindexProject()
      val current = presenter.snapshot.value.state.projectState.indexingAttempt!!
      assertTrue(current.generation != old.generation)
      // The old response was queued first; it must not complete the new same-revision attempt.
      main.runNext()
      assertEquals(
          ProjectIndexingOutcome.Running,
          presenter.snapshot.value.state.projectState.indexingAttempt?.outcome)
      main.runPending()
      fail = true
      io.runPending()
      main.runPending()
      assertEquals(
          ProjectIndexingOutcome.Failed("Could not re-index project. Try again."),
          presenter.snapshot.value.state.projectState.indexingAttempt?.outcome)
      assertEquals("revision", presenter.snapshot.value.state.project?.projectRevision)
      assertEquals(2, calls.size)
      presenter.reindexProject()
      assertEquals(
          "revision", presenter.snapshot.value.state.projectState.indexingAttempt?.projectRevision)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun lateFailureCannotFailANewAttemptAfterReturningToTheSameProject() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    var fail = true
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { method, path, _ ->
          when {
            method == "POST" && path.endsWith("/reindex") -> {
              if (fail) throw IllegalStateException("old failure")
              response(indexJson())
            }
            path.contains("/analysis/run?") || path.contains("/scan?") -> TransportResponse(204, "")
            path.contains("/analysis/selection?") ||
                path.contains("/overview?") ||
                path.contains("/findings?") -> response("{}")
            else -> error("Unexpected $method $path")
          }
        }
    try {
      loadProject(presenter)
      presenter.reindexProject()
      main.runPending()
      io.runPending()
      presenter.dispatch(
          DesktopEvent.ProjectLoaded(project("other"), ProjectIndex("other", "revision")))
      loadProject(presenter)
      presenter.reindexProject()
      val replacement = presenter.snapshot.value.state.projectState.indexingAttempt!!
      main.runNext()
      assertEquals(replacement, presenter.snapshot.value.state.projectState.indexingAttempt)
      main.runPending()
      fail = false
      io.runPending()
      main.runPending()
      assertEquals(
          ProjectIndexingOutcome.Succeeded("revision"),
          presenter.snapshot.value.state.projectState.indexingAttempt?.outcome)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun canceledAndMismatchedReindexNeverRefreshSavedWork() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val calls = mutableListOf<String>()
    var index = indexJson()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { method, path, _ ->
          calls += "$method $path"
          when {
            path.endsWith("/reindex") -> response(index)
            path == "/api/projects/import" -> response(projectJson())
            path == "/api/projects/current/index" -> response(indexJson())
            path.contains("/analysis/run?") || path.contains("/scan?") -> TransportResponse(204, "")
            path.contains("/analysis/selection?") ||
                path.contains("/overview?") ||
                path.contains("/findings?") -> response("{}")
            else -> error("Unexpected $method $path")
          }
        }
    try {
      loadProject(presenter)
      presenter.reindexProject()
      main.runPending()
      io.runPending()
      presenter.loadProject("/tmp/project", restore = false)
      assertEquals(
          ProjectIndexingOutcome.Canceled,
          presenter.snapshot.value.state.projectState.indexingAttempt?.outcome)
      main.runPending()
      assertTrue(calls.none { it.contains("/overview?") || it.contains("/findings?") })
      repeat(6) {
        io.runPending()
        main.runPending()
      }
      index = """{"project_id":"other","project_revision":"revision"}"""
      presenter.reindexProject()
      main.runPending()
      io.runPending()
      main.runPending()
      assertTrue(
          presenter.snapshot.value.state.projectState.indexingAttempt?.outcome
              is ProjectIndexingOutcome.Failed)
      assertEquals("project", presenter.snapshot.value.state.projectState.index?.projectId)
      assertEquals("revision", presenter.snapshot.value.state.project?.projectRevision)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun sixWorkspaceCycleAndNumberedDestinationsRetainDraftFileAndResultsWithoutRequests() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val calls = mutableListOf<Pair<String, String>>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { method, path, _ ->
          calls += method to path
          response("{}")
        }
    try {
      loadFile(presenter)
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      presenter.dispatch(DesktopEvent.DraftEdited(declaration = "func Run() { println(1) }"))
      val run = analysisRunFixture()
      val results = analysisResultsFixture(run, "bugs")
      val analysis =
          ProjectAnalysisRunState(
              run = run,
              sections =
                  mapOf(AnalysisResultKey("bugs") to AnalysisSectionState(results = results)))
      presenter.dispatch(DesktopEvent.AnalysisRunUpdated(analysis))
      val retained = presenter.snapshot.value.state
      assertEquals("main.go", retained.selectedFile?.path)
      assertEquals("func Run() { println(1) }", retained.review.editor?.declaration)
      for (expected in
          listOf(
              Workspace.Analysis,
              Workspace.Performance,
              Workspace.Bugs,
              Workspace.Security,
              Workspace.Editor,
              Workspace.Summary)) {
        presenter.dispatch(
            DesktopEvent.WorkspaceSelected(nextWorkspace(presenter.snapshot.value.state.workspace)))
        assertEquals(expected, presenter.snapshot.value.state.workspace)
        assertEquals(retained.selection, presenter.snapshot.value.state.selection)
        assertEquals(retained.review, presenter.snapshot.value.state.review)
        assertEquals(analysis, presenter.snapshot.value.state.analysisRun)
      }
      assertEquals(
          listOf(
              DesktopShortcut.SummaryWorkspace,
              DesktopShortcut.AnalysisWorkspace,
              DesktopShortcut.BugsWorkspace,
              DesktopShortcut.EditorWorkspace),
          (1..4).map { desktopShortcut(it.toString(), primaryModifier = true) })
      main.runPending()
      io.runPending()
      main.runPending()
      assertTrue(
          calls.isEmpty(),
          "Navigation must not call providers, scans, checks, Apply/Undo or write source: $calls")
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun everyAnalysisEntryPreviewsTheWholeProjectAndNavigationNeverStartsIt() {
    val calls = mutableListOf<Pair<String, String>>()
    val requests = mutableListOf<AnalysisPreviewRequest>()
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { method, path, body ->
          calls.add(method to path)
          assertEquals("/api/projects/current/analysis/preview", path)
          val request =
              kotlinx.serialization.json.Json.decodeFromString<AnalysisPreviewRequest>(body!!)
          requests.add(request)
          response(
              kotlinx.serialization.json.Json.encodeToString(
                  AnalysisRunPreview.serializer(),
                  analysisPreviewFixture()
                      .copy(
                          refresh = request.refresh, retryStaleFailed = request.retryStaleFailed)))
        }
    try {
      loadProject(presenter)
      for (entry in
          listOf<() -> Unit>(
              { presenter.previewAnalysis() },
              { presenter.startAnalyzeAll(AnalyzeAllRunOptions()) },
              { presenter.previewPerformance() },
              { presenter.reviewSecurity() },
              { presenter.analyzeSelected(false) },
              { presenter.previewAnalysis(retryStaleFailed = true) })) {
        entry()
        main.runPending()
        io.runPending()
        main.runPending()
        assertEquals(
            "project", presenter.snapshot.value.state.analysisRun.admission?.preview?.scope)
        presenter.dismissAnalysisAdmission()
      }
      Workspace.entries.forEach { presenter.dispatch(DesktopEvent.WorkspaceSelected(it)) }
      main.runPending()
      io.runPending()
      main.runPending()
      assertEquals(6, calls.size)
      assertEquals(listOf(true, true, true, true, false, false), requests.map { it.refresh })
      assertEquals(
          listOf(false, false, false, false, false, true), requests.map { it.retryStaleFailed })
      assertTrue(calls.all { it.first == "POST" && it.second.endsWith("/preview") })
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun benchmarkAdmissionIsRevokedAtValidationApplyUndoAndDisposalEntryPoints() {
    for (stage in 0..2) {
      for (action in listOf("validate", "apply", "undo", "close")) {
        val main = QueuedDispatcher()
        val io = QueuedDispatcher()
        val scope = CoroutineScope(SupervisorJob() + main)
        val calls = mutableListOf<Pair<String, String>>()
        val presenter =
            presenter(parentScope = scope, ioDispatcher = io, interceptTrust = false) {
                method,
                path,
                _ ->
              calls += method to path
              when {
                path.contains("execution-trust") ->
                    response(
                        """{"project_id":"project","project_revision":"revision","trusted":true,"commands":[["go","test","./..."]]}""")
                path.endsWith("/benchmarks") -> response(benchmarkComparisonJson())
                else -> TransportResponse(503, """{"message":"Test mutation unavailable"}""")
              }
            }
        try {
          loadFile(presenter)
          presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
          presenter.dispatch(
              DesktopEvent.ChecksLoaded(
                  DraftCheckReport(
                      "main.go",
                      true,
                      draftId = "draft",
                      draftRevision = 1,
                      draftHash = "draft-hash")))
          if (action == "undo")
              presenter.dispatch(DesktopEvent.Applied(ApplyResult("next", "after", true)))
          val catalog = Json.decodeFromString<GoBenchmarkCatalog>(benchmarkCatalogJson())
          val prior = Json.decodeFromString<GoBenchmarkComparison>(benchmarkComparisonJson())
          presenter.dispatch(DesktopEvent.GoBenchmarkCatalogLoaded(catalog))
          presenter.selectGoBenchmark(catalog.benchmarks.single())
          presenter.dispatch(DesktopEvent.GoBenchmarkComparisonLoaded(prior))
          presenter.compareSelectedGoBenchmark()
          repeat(stage) {
            main.runPending()
            io.runPending()
            main.runPending()
          }
          main.runPending()
          io.runPending() // A completed stage still waiting to resume on the presenter dispatcher.
          when (action) {
            "validate" -> presenter.validateEditableDraft()
            "apply" -> presenter.applyEditableDraft()
            "undo" -> presenter.undoAppliedDraft()
            "close" -> presenter.close()
          }
          val evidence = presenter.snapshot.value.state.review.benchmark
          assertFalse(evidence.running, "$stage: $action")
          assertNull(evidence.catalog)
          assertNull(evidence.selected)
          assertEquals(prior, evidence.comparison)
          repeat(6) {
            main.runPending()
            io.runPending()
          }
          assertEquals(
              stage + 1,
              calls.count {
                it.second.contains("execution-trust") || it.second.endsWith("/benchmarks")
              },
              "$stage: $action")
          assertEquals(prior, presenter.snapshot.value.state.review.benchmark.comparison)
          assertFalse(presenter.snapshot.value.state.review.benchmark.running)
          if (action == "apply") assertTrue(calls.any { it.second.endsWith("/apply") })
          if (action == "undo") assertTrue(calls.any { it.second.endsWith("/undo") })
        } finally {
          presenter.close()
          scope.cancel()
        }
      }
    }
  }

  @Test
  fun nonCooperativeBenchmarkTerminalsCannotPublishAfterPresenterLifecycleActions() {
    for (status in listOf("completed", "failed", "canceled", "unavailable")) {
      for (action in listOf("validate", "apply", "undo", "refresh", "close")) {
        val entered = CountDownLatch(1)
        val interrupted = CountDownLatch(1)
        val release = CountDownLatch(1)
        val main = Executors.newSingleThreadExecutor().asCoroutineDispatcher()
        // Refresh and mutation requests can finish while the obsolete comparison stays blocked.
        val io = Executors.newFixedThreadPool(2).asCoroutineDispatcher()
        val scope = CoroutineScope(SupervisorJob() + main)
        val calls = Collections.synchronizedList(mutableListOf<Pair<String, String>>())
        val prior = Json.decodeFromString<GoBenchmarkComparison>(benchmarkComparisonJson())
        val terminal = prior.copy(status = status, reason = "Late $status")
        val presenter =
            presenter(parentScope = scope, ioDispatcher = io) { method, path, _ ->
              calls += method to path
              when {
                path.endsWith("/benchmarks") -> {
                  entered.countDown()
                  while (true) {
                    try {
                      release.await()
                      break
                    } catch (_: InterruptedException) {
                      interrupted.countDown()
                    }
                  }
                  response(Json.encodeToString(terminal))
                }
                path.contains("files/info?") -> response(fileJson("main.go", "shell-edit"))
                path.contains("files/symbols?") -> response(symbolsJson("main.go", "Run"))
                else -> TransportResponse(503, """{"message":"Test mutation unavailable"}""")
              }
            }
        try {
          val lifetime = scope.coroutineContext[Job]!!.children.single()
          val comparisonJob =
              runBlocking(main) {
                loadFile(presenter)
                presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
                presenter.dispatch(
                    DesktopEvent.ChecksLoaded(
                        DraftCheckReport(
                            "main.go",
                            true,
                            draftId = "draft",
                            draftRevision = 1,
                            draftHash = "draft-hash")))
                if (action == "undo")
                    presenter.dispatch(DesktopEvent.Applied(ApplyResult("next", "after", true)))
                val catalog =
                    Json.decodeFromString<GoBenchmarkCatalog>(benchmarkCatalogJson(trusted = true))
                presenter.dispatch(DesktopEvent.GoBenchmarkCatalogLoaded(catalog))
                presenter.selectGoBenchmark(catalog.benchmarks.single())
                presenter.dispatch(DesktopEvent.GoBenchmarkComparisonLoaded(prior))
                val existing = lifetime.children.toSet()
                presenter.compareSelectedGoBenchmark()
                (lifetime.children.toSet() - existing).single()
              }
          assertTrue(entered.await(5, TimeUnit.SECONDS), "$status/$action: transport must start")
          val actionJobs =
              runBlocking(main) {
                val existing = lifetime.children.toSet()
                when (action) {
                  "validate" -> presenter.validateEditableDraft()
                  "apply" -> presenter.applyEditableDraft()
                  "undo" -> presenter.undoAppliedDraft()
                  "refresh" -> presenter.refreshSelectedFile()
                  "close" -> presenter.close()
                }
                (lifetime.children.toSet() - existing)
              }
          runBlocking { withTimeout(5_000) { actionJobs.forEach { it.join() } } }
          assertTrue(
              interrupted.await(5, TimeUnit.SECONDS),
              "$status/$action: cancellation must interrupt")
          val replacement = runBlocking(main) { presenter.snapshot.value }
          assertFalse(replacement.state.review.benchmark.running)
          assertNull(replacement.state.review.benchmark.catalog)
          assertNull(replacement.state.review.benchmark.selected)
          assertEquals(prior, replacement.state.review.benchmark.comparison)
          assertNull(replacement.state.review.benchmark.latestOutcome)
          if (action == "refresh")
              assertEquals("shell-edit", replacement.state.selectedFile?.contentHash)
          release.countDown()
          runBlocking { withTimeout(5_000) { comparisonJob.join() } }
          runBlocking(main) {
            assertEquals(
                replacement, presenter.snapshot.value, "$status/$action: no late publication")
          }
          assertEquals(
              listOf("POST"), calls.filter { it.second.endsWith("/benchmarks") }.map { it.first })
          assertTrue(
              calls.none { it.second.contains("execution-trust") }, "No automatic trust grant")
          if (action == "apply") assertTrue(calls.any { it.second.endsWith("/apply") })
          if (action == "undo") assertTrue(calls.any { it.second.endsWith("/undo") })
        } finally {
          release.countDown()
          runBlocking(main) { presenter.close() }
          scope.cancel()
          io.close()
          main.close()
        }
      }
    }
  }

  @Test
  fun passivePerformanceNavigationSelectionAndDisclosureRetainUnchangedCandidateEvidence() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val calls = mutableListOf<Pair<String, String>>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { method, path, _ ->
          calls += method to path
          assertTrue(path.contains("/benchmarks"))
          response(if (method == "GET") benchmarkCatalogJson(true) else benchmarkComparisonJson())
        }
    try {
      loadFile(presenter)
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      presenter.loadGoBenchmarks()
      main.runPending()
      io.runPending()
      main.runPending()
      presenter.selectGoBenchmark(
          presenter.snapshot.value.state.review.benchmark.catalog!!.benchmarks.single())
      presenter.compareSelectedGoBenchmark()
      main.runPending()
      io.runPending()
      main.runPending()
      val retained = presenter.snapshot.value.state.review
      assertEquals("completed", retained.benchmark.comparison?.status)
      calls.clear()
      Workspace.entries.forEach { presenter.dispatch(DesktopEvent.WorkspaceSelected(it)) }
      presenter.dispatch(DesktopEvent.WorkspaceSelected(Workspace.Performance))
      val original = performancePageFixture()
      val report = original.results!!.performance.single()
      val page =
          original.copy(
              section =
                  original.section.copy(
                      results =
                          original.results!!.copy(
                              performance =
                                  listOf(
                                      report.copy(
                                          findings =
                                              report.findings +
                                                  report.findings.single().copy(id = "other"))))))
      val browser = newResultBrowserState(page)
      var sourceActions = 0
      ComposeVisualFixture(800, 650) {
            val state = presenter.snapshot.value.state
            PerformanceWorkspacePane(
                PerformanceWorkspacePaneState(
                    page,
                    state.index,
                    benchmarkComparison = state.review.benchmark.comparison,
                    expectedBenchmarkIdentity =
                        (benchmarkEligibility(state).candidate as? BenchmarkCandidateDecision.Ready)
                            ?.draft
                            ?.let(::goBenchmarkComparisonIdentity),
                    benchmarkCatalog = state.review.benchmark.catalog,
                    selectedBenchmark = state.review.benchmark.selected,
                    benchmarkEligibility = benchmarkEligibility(state),
                    browser = browser),
                PerformanceWorkspaceActions(
                    prepareOptimization = { sourceActions++ },
                    openAnalysis = { sourceActions++ },
                    semanticActions =
                        FindingActions(
                            { sourceActions++ }, { _, _ -> sourceActions++ }, { sourceActions++ }),
                    openSource = { sourceActions++ },
                    loadBenchmarks = presenter::loadGoBenchmarks,
                    selectBenchmark = presenter::selectGoBenchmark,
                    runBenchmark = presenter::compareSelectedGoBenchmark))
          }
          .use { fixture ->
            performanceResults(page).forEach { result ->
              browser.choose(result.row().key)
              fixture.render()
              fixture.clickDescription("Expand Explore benchmark evidence")
              fixture.render()
              for (label in
                  listOf(
                      "Measurement details",
                      "Recorded conditions & identity",
                      "Returned sample details")) {
                fixture.revealTextFullyWithin(label, "result-overview")
                assertTrue(fixture.requestDescriptionFocus("Expand $label"))
                assertTrue(fixture.pressKey(Key.Enter))
                fixture.render()
                assertEquals("Expanded", fixture.descriptionState("Collapse $label"))
              }
              val sampleLine =
                  performanceBenchmarkSampleRows(retained.benchmark.comparison!!)
                      .first { it.first == "Baseline sample 1" }
                      .let { "${it.first}: ${it.second}" }
              fixture.revealTextFullyWithin(sampleLine, "result-overview")
              val selectedText = fixture.copyTextByDragging(sampleLine, expectedText = sampleLine)
              assertTrue(selectedText.isNotEmpty())
              assertTrue(sampleLine.contains(selectedText))
              fixture.revealTextFullyWithin("Copy displayed benchmark evidence", "result-overview")
              assertTrue(fixture.requestDescriptionFocus("Copy displayed benchmark evidence"))
              assertTrue(fixture.pressKey(Key.Spacebar))
              fixture.render()
              val comparison = retained.benchmark.comparison
              assertEquals(
                  performanceBenchmarkCopyText(
                      comparison,
                      performanceBenchmarkPresentation(
                          comparison,
                          goBenchmarkComparisonIdentity(retained.draft!!),
                          retained.benchmark.selected),
                      false,
                      true,
                      true),
                  fixture.clipboardText())
              fixture.resize(1280, 600)
              fixture.render()
              fixture.resize(800, 650)
              fixture.render()
              fixture.revealTextFullyWithin("Explore benchmark evidence", "result-overview")
              fixture.clickDescription("Collapse Explore benchmark evidence")
              fixture.render()
              assertEquals(retained, presenter.snapshot.value.state.review)
            }
          }
      main.runPending()
      io.runPending()
      main.runPending()
      assertTrue(
          calls.isEmpty(),
          "Passive interactions must not request discovery, trust, providers or source writes: $calls")
      assertEquals(0, sourceActions)
      assertEquals(retained, presenter.snapshot.value.state.review)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun benchmarkComparisonUsesAnExplicitReadOnlyCatalogThenRetainsExactEvidence() {
    val catalogCalls = AtomicInteger()
    val comparisonCalls = AtomicInteger()
    val presenter = presenter { method, path, body ->
      when (method to path) {
        "GET" to
            "/api/projects/current/drafts/draft/benchmarks?project_revision=revision&expected_revision=1&expected_hash=draft-hash" -> {
          catalogCalls.incrementAndGet()
          response(benchmarkCatalogJson())
        }
        "POST" to "/api/projects/current/drafts/draft/benchmarks" -> {
          comparisonCalls.incrementAndGet()
          assertTrue(body.orEmpty().contains("\"benchmark\":\"BenchmarkRun\""))
          assertTrue(body.orEmpty().contains("\"expected_scope\":\"scope\""))
          response(benchmarkComparisonJson())
        }
        else -> error("unexpected request $method $path")
      }
    }
    try {
      loadFile(presenter)
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))

      assertEquals(0, catalogCalls.get())
      assertEquals(0, comparisonCalls.get())
      presenter.loadGoBenchmarks()
      eventually { presenter.snapshot.value.state.review.benchmark.catalog != null }

      assertEquals(1, catalogCalls.get())
      assertNull(presenter.snapshot.value.state.review.benchmark.selected)
      presenter.compareSelectedGoBenchmark()
      assertEquals(0, comparisonCalls.get())
      assertFalse(presenter.snapshot.value.state.review.benchmark.running)

      presenter.selectGoBenchmark(
          presenter.snapshot.value.state.review.benchmark.catalog!!.benchmarks.single())
      presenter.compareSelectedGoBenchmark()
      eventually {
        presenter.snapshot.value.state.review.benchmark.comparison?.status == "completed"
      }

      assertEquals(1, comparisonCalls.get())
      assertEquals(
          "BenchmarkRun", presenter.snapshot.value.state.review.benchmark.comparison?.benchmark)
    } finally {
      presenter.close()
    }
  }

  @Test
  fun benchmarkCatalogRequestIsCanceledWhenTheDraftChanges() {
    val started = CountDownLatch(1)
    val interrupted = CountDownLatch(1)
    val release = CountDownLatch(1)
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "GET" to
            "/api/projects/current/drafts/draft/benchmarks?project_revision=revision&expected_revision=1&expected_hash=draft-hash" -> {
          started.countDown()
          try {
            release.await(2, TimeUnit.SECONDS)
          } catch (error: InterruptedException) {
            interrupted.countDown()
            throw error
          }
          response(benchmarkCatalogJson())
        }
        else -> error("unexpected request $method $path")
      }
    }
    try {
      loadFile(presenter)
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      presenter.loadGoBenchmarks()
      assertTrue(started.await(1, TimeUnit.SECONDS))

      presenter.dispatch(DesktopEvent.DraftEdited(declaration = "func Run() int { return 2 }"))

      assertTrue(interrupted.await(1, TimeUnit.SECONDS))
      assertNull(presenter.snapshot.value.state.review.benchmark.catalog)
      assertFalse(presenter.snapshot.value.state.loading)
    } finally {
      release.countDown()
      presenter.close()
    }
  }

  @Test
  fun benchmarkCatalogRequestIsCanceledWhenAnApprovedReplacementPublishes() {
    val started = CountDownLatch(1)
    val interrupted = CountDownLatch(1)
    val release = CountDownLatch(1)
    val presenter = presenter { _, path, _ ->
      when {
        path.startsWith("/api/projects/current/drafts/draft/benchmarks") -> {
          started.countDown()
          try {
            release.await(2, TimeUnit.SECONDS)
          } catch (error: InterruptedException) {
            interrupted.countDown()
            throw error
          }
          response(benchmarkCatalogJson())
        }
        path.contains("files/info?path=other.go") -> response(fileJson("other.go", "other"))
        path.contains("files/symbols?path=other.go") -> response(symbolsJson("other.go"))
        path.contains("files/analysis?path=other.go") ->
            response("""{"path":"other.go","status":"missing"}""")
        path.contains("/impact?path=other.go") -> response("""{"target_path":"other.go"}""")
        path.contains("/git?path=other.go") -> response("""{"available":false}""")
        else -> error("unexpected request $path")
      }
    }
    try {
      loadFile(presenter)
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      presenter.loadGoBenchmarks()
      assertTrue(started.await(1, TimeUnit.SECONDS))

      assertTrue(
          presenter.confirmFileNavigationIntent(presenter.fileNavigationIntent("other.go")!!))

      assertTrue(interrupted.await(1, TimeUnit.SECONDS))
      eventually { presenter.snapshot.value.state.selectedFile?.path == "other.go" }
      assertNull(presenter.snapshot.value.state.review.benchmark.catalog)
    } finally {
      release.countDown()
      presenter.close()
    }
  }

  @Test
  fun projectSwitchAndCloseCancelBothBenchmarkRequestKinds() {
    for (comparing in listOf(false, true)) {
      for (closing in listOf(false, true)) {
        val started = CountDownLatch(1)
        val interrupted = CountDownLatch(1)
        val release = CountDownLatch(1)
        val main = Executors.newSingleThreadExecutor().asCoroutineDispatcher()
        val io = Executors.newSingleThreadExecutor().asCoroutineDispatcher()
        val scope = CoroutineScope(SupervisorJob() + main)
        val methods = Collections.synchronizedList(mutableListOf<String>())
        val presenter =
            presenter(parentScope = scope, ioDispatcher = io) { method, path, _ ->
              check(path.contains("/benchmarks"))
              methods += method
              if (comparing && method == "GET") {
                response(benchmarkCatalogJson(trusted = true))
              } else {
                started.countDown()
                try {
                  // Only cancellation or cleanup releases the request, never elapsed time.
                  release.await()
                } catch (error: InterruptedException) {
                  interrupted.countDown()
                  throw error
                }
                response(if (comparing) benchmarkComparisonJson() else benchmarkCatalogJson())
              }
            }
        try {
          // Production confines presentation to the UI dispatcher. Observing catalog publication
          // on a Default worker did not wait for its trailing Status dispatch, which could race
          // selection/admission on the test thread and overwrite the selected choice.
          runBlocking(main) {
            loadFile(presenter)
            presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
            presenter.loadGoBenchmarks()
            if (comparing) {
              val catalog =
                  withTimeout(1_000) {
                        presenter.snapshot.first { it.state.review.benchmark.catalog != null }
                      }
                      .state
                      .review
                      .benchmark
                      .catalog!!
              presenter.selectGoBenchmark(catalog.benchmarks.single())
              assertTrue(benchmarkEligibility(presenter.snapshot.value.state).canCompare)
              presenter.compareSelectedGoBenchmark()
              assertEquals(
                  BenchmarkAdmissionOutcome.Running,
                  presenter.snapshot.value.state.review.benchmark.admission)
            }
          }
          assertTrue(started.await(1, TimeUnit.SECONDS), "Benchmark request must enter transport")
          assertEquals(if (comparing) listOf("GET", "POST") else listOf("GET"), methods.toList())
          runBlocking(main) {
            if (closing) presenter.close()
            else
                presenter.dispatch(
                    DesktopEvent.ProjectLoaded(
                        project("other", "new"), ProjectIndex("other", "new")))
          }
          assertTrue(
              interrupted.await(1, TimeUnit.SECONDS), "Cancellation must interrupt transport")
          runBlocking(main) {
            assertFalse(presenter.snapshot.value.state.review.benchmark.running)
            assertNull(presenter.snapshot.value.state.review.benchmark.catalog)
            assertNull(presenter.snapshot.value.state.review.benchmark.comparison)
          }
        } finally {
          release.countDown()
          runBlocking(main) { presenter.close() }
          scope.cancel()
          io.close()
          main.close()
        }
      }
    }
  }

  @Test
  fun failedReindexDoesNotLeaveBenchmarkComparisonRunning() {
    val presenter = presenter { method, path, _ ->
      if (method == "POST" && path == "/api/projects/current/reindex")
          throw IllegalStateException("reindex failed")
      else error("unexpected request $method $path")
    }
    try {
      loadFile(presenter)
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      presenter.dispatch(DesktopEvent.GoBenchmarkComparisonStarted)
      assertTrue(presenter.snapshot.value.state.review.benchmark.running)

      presenter.reindexProject()

      eventually {
        presenter.snapshot.value.state.projectState.indexingAttempt?.outcome ==
            ProjectIndexingOutcome.Failed("reindex failed")
      }
      assertFalse(presenter.snapshot.value.state.review.benchmark.running)
      assertFalse(presenter.snapshot.value.state.loading)
    } finally {
      presenter.close()
    }
  }

  @Test
  fun benchmarkComparisonResponseCannotPublishAfterTheDraftChanges() {
    val started = CountDownLatch(1)
    val release = CountDownLatch(1)
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "GET" to
            "/api/projects/current/drafts/draft/benchmarks?project_revision=revision&expected_revision=1&expected_hash=draft-hash" ->
            response(benchmarkCatalogJson(trusted = true))
        "POST" to "/api/projects/current/drafts/draft/benchmarks" -> {
          started.countDown()
          release.await(2, TimeUnit.SECONDS)
          response(benchmarkComparisonJson())
        }
        else -> error("unexpected request $method $path")
      }
    }
    try {
      loadFile(presenter)
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      presenter.loadGoBenchmarks()
      eventually { presenter.snapshot.value.state.review.benchmark.catalog != null }
      presenter.selectGoBenchmark(
          presenter.snapshot.value.state.review.benchmark.catalog!!.benchmarks.single())
      presenter.compareSelectedGoBenchmark()
      assertTrue(started.await(1, TimeUnit.SECONDS))
      presenter.dispatch(DesktopEvent.DraftEdited(declaration = "func Run() int { return 1 }"))
      assertFalse(presenter.snapshot.value.state.review.benchmark.running)
      assertFalse(presenter.snapshot.value.state.loading)
      release.countDown()
      eventually { !presenter.snapshot.value.state.review.benchmark.running }

      assertNull(presenter.snapshot.value.state.review.benchmark.comparison)
      assertNull(presenter.snapshot.value.state.review.benchmark.catalog)
    } finally {
      release.countDown()
      presenter.close()
    }
  }

  @Test
  fun unavailableBenchmarkResponseRevokesScopeAuthorityAndRetainsPriorEvidence() {
    val prior = Json.decodeFromString<GoBenchmarkComparison>(benchmarkComparisonJson())
    val unavailable = unavailableBenchmarkComparisonJson(scope = "recomputed-scope")
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "GET" to
            "/api/projects/current/drafts/draft/benchmarks?project_revision=revision&expected_revision=1&expected_hash=draft-hash" ->
            response(benchmarkCatalogJson(trusted = true))
        "POST" to "/api/projects/current/drafts/draft/benchmarks" -> response(unavailable)
        else -> error("unexpected request $method $path")
      }
    }
    try {
      loadFile(presenter)
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      presenter.loadGoBenchmarks()
      eventually { presenter.snapshot.value.state.review.benchmark.catalog != null }
      presenter.selectGoBenchmark(
          presenter.snapshot.value.state.review.benchmark.catalog!!.benchmarks.single())
      presenter.dispatch(DesktopEvent.GoBenchmarkComparisonLoaded(prior))
      presenter.compareSelectedGoBenchmark()

      eventually {
        presenter.snapshot.value.state.review.benchmark.latestOutcome?.status ==
            BenchmarkComparisonStatus.Unavailable
      }

      val evidence = presenter.snapshot.value.state.review.benchmark
      assertFalse(evidence.running)
      assertEquals(prior, evidence.comparison)
      assertNull(evidence.catalog)
      assertNull(evidence.selected)
      assertEquals(BenchmarkDiscoveryOutcome.Invalidated, evidence.discovery)
      assertEquals(BenchmarkAdmissionOutcome.Idle, evidence.admission)
      assertEquals(
          Json.decodeFromString<GoBenchmarkComparison>(unavailable),
          evidence.latestOutcome?.response)
      assertFalse(benchmarkEligibility(presenter.snapshot.value.state).canCompare)
      assertFalse(presenter.snapshot.value.state.loading)
      assertNull(presenter.snapshot.value.state.jobs.error)
    } finally {
      presenter.close()
    }
  }

  @Test
  fun cachedContextInspectionAndEditorNavigationDoNotDispatchOrChangeEditEvidence() {
    val other = SymbolInfo("Other", "function", confidence = "exact", atomicTarget = true)
    FileNavigationUiFixture("draft", extraSymbols = listOf(other)).use { navigation ->
      val presenter = navigation.presenter
      presenter.dispatch(
          DesktopEvent.AnalysisLoaded(
              FileAnalysis(
                  "main.go", "fresh", symbolExplanations = mapOf("Run" to "Cached prose"))))
      val before = presenter.snapshot.value.state
      presenter.dispatch(DesktopEvent.WorkspaceSelected(Workspace.Editor))
      val selected = presenter.snapshot.value.state.selectedSymbol!!
      val inspector =
          symbolInspectorUiState(
              before.selectedFile,
              before.symbols,
              selected,
              before.analysis,
              false,
              InspectorProviderState(false, false),
              currentEditIdentity(before))!!
      assertEquals("Cached prose", inspector.selectedSymbol?.explanation)
      presenter.dispatch(DesktopEvent.SymbolSelected(other))
      presenter.dispatch(DesktopEvent.SymbolSelected(selected))
      navigation.runPending()
      val after = presenter.snapshot.value
      assertEquals(before.review, after.state.review)
      assertEquals(before.chat, after.state.chat)
      assertEquals(before.selectedFile, after.state.selectedFile)
      assertEquals(DeclarationExplanationStatus.Unavailable, after.declarationExplanation.status)
      assertEquals(emptyList(), navigation.calls, "Inspection must not explain, generate or write")
    }
  }

  @Test
  fun explanationPublishesOnlyForItsExactSelectionAndDoesNotAlterDraftState() {
    val calls = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      if (method == "POST" && path == "/api/projects/current/files/explanation") {
        calls.incrementAndGet()
        response(explanationJson("Run"))
      } else error("unexpected request $method $path")
    }
    try {
      loadFile(presenter)
      presenter.explainSelectedDeclaration()
      eventually {
        presenter.snapshot.value.declarationExplanation.status ==
            DeclarationExplanationStatus.Current
      }

      assertEquals(1, calls.get())
      assertEquals("Explains Run.", presenter.snapshot.value.declarationExplanation.result?.summary)
      assertEquals(null, presenter.snapshot.value.state.chat.session)
      assertEquals(null, presenter.snapshot.value.state.review.draft)
      assertEquals(null, presenter.snapshot.value.state.review.checks)
    } finally {
      presenter.close()
    }
  }

  @Test
  fun explanationRejectsMissingAndIneligibleSelectionsWithoutDispatch() {
    val requests = mutableListOf<String>()
    val presenter = presenter { method, path, _ ->
      requests += "$method $path"
      error("unexpected request $method $path")
    }
    try {
      loadFile(presenter)
      assertTrue(requests.isEmpty()) // Selection is passive.
      val run = presenter.snapshot.value.state.selectedSymbol!!
      presenter.dispatch(DesktopEvent.FileLoaded(file(), emptyList()))
      presenter.dispatch(DesktopEvent.SymbolSelected(run))
      presenter.explainSelectedDeclaration()
      assertEquals(
          DeclarationExplanationStatus.Unavailable,
          presenter.snapshot.value.declarationExplanation.status)

      for (invalid in
          listOf(run.copy(confidence = "approximate"), run.copy(atomicTarget = false))) {
        presenter.dispatch(DesktopEvent.FileLoaded(file(), listOf(invalid)))
        presenter.dispatch(DesktopEvent.SymbolSelected(invalid))
        presenter.explainSelectedDeclaration()
      }
      presenter.dispatch(DesktopEvent.FileLoaded(file().copy(language = "Python"), listOf(run)))
      presenter.dispatch(DesktopEvent.SymbolSelected(run))
      presenter.explainSelectedDeclaration()
      assertTrue(requests.isEmpty())
      assertEquals(
          DeclarationExplanationStatus.Unavailable,
          presenter.snapshot.value.declarationExplanation.status)
      assertNull(presenter.snapshot.value.state.review.draft)
    } finally {
      presenter.close()
    }
  }

  @Test
  fun explanationUsesAdmittedIdentityAndFunctionConfirmation() {
    val dispatcher = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    val bodies = mutableListOf<String>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = dispatcher) { method, path, body ->
          when (method to path) {
            "GET" to "/status" -> response("""{"status":"ok","version":"v1"}""")
            "GET" to "/api/models/current" -> response(functionCatalog("destination-a"))
            "POST" to "/api/projects/current/files/explanation" -> {
              bodies += body.orEmpty()
              response(explanationJson("Run", "func Run()", 2, 4))
            }
            else -> error("unexpected request $method $path")
          }
        }
    try {
      presenter.refreshConnection()
      dispatcher.runPending()
      loadFile(presenter)
      val symbol =
          SymbolInfo(
              "Run", "function", "func Run()", 2, 4, confidence = "exact", atomicTarget = true)
      presenter.dispatch(DesktopEvent.FileLoaded(file(), listOf(symbol)))
      presenter.dispatch(DesktopEvent.SymbolSelected(symbol))
      presenter.explainSelectedDeclaration()
      assertTrue(bodies.isEmpty())
      assertEquals(
          DeclarationExplanationStatus.Failed,
          presenter.snapshot.value.declarationExplanation.status)

      presenter.setProviderConfirmation(ModelScope.Function, true)
      presenter.explainSelectedDeclaration()
      // Revoking Function consent before transport must prevent the admitted request.
      presenter.setProviderConfirmation(ModelScope.Function, false)
      dispatcher.runPending()
      assertTrue(bodies.isEmpty())
      assertEquals(
          DeclarationExplanationStatus.Stale,
          presenter.snapshot.value.declarationExplanation.status)
      presenter.setProviderConfirmation(ModelScope.Function, true)
      presenter.explainSelectedDeclaration()
      dispatcher.runPending()
      assertEquals(1, bodies.size)
      val body = bodies.single()
      for (field in
          listOf(
              "\"project_id\":\"project\"",
              "\"project_revision\":\"revision\"",
              "\"base_file_hash\":\"base\"",
              "\"target_path\":\"main.go\"",
              "\"target_symbol\":\"Run\"",
              "\"confirm_remote_provider\":true")) {
        assertTrue(body.contains(field), "Missing $field in $body")
      }
      assertEquals(
          DeclarationExplanationStatus.Current,
          presenter.snapshot.value.declarationExplanation.status)
      assertNull(presenter.snapshot.value.state.review.draft)
      assertNull(presenter.snapshot.value.state.review.checks)
    } finally {
      presenter.close()
      scope.cancel()
      dispatcher.runPending()
    }
  }

  @Test
  fun selectionChangeCancelsAndRejectsLateDeclarationExplanation() {
    val started = CountDownLatch(1)
    val release = CountDownLatch(1)
    val presenter = presenter { method, path, _ ->
      if (method == "POST" && path == "/api/projects/current/files/explanation") {
        started.countDown()
        release.await(2, TimeUnit.SECONDS)
        response(explanationJson("Run"))
      } else error("unexpected request $method $path")
    }
    try {
      loadProject(presenter)
      val run = SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true)
      val other = SymbolInfo("Other", "function", confidence = "exact", atomicTarget = true)
      presenter.dispatch(DesktopEvent.FileLoaded(file(), listOf(run, other)))
      presenter.dispatch(DesktopEvent.SymbolSelected(run))
      presenter.explainSelectedDeclaration()
      assertTrue(started.await(1, TimeUnit.SECONDS))
      presenter.dispatch(DesktopEvent.SymbolSelected(other))
      release.countDown()
      eventually {
        presenter.snapshot.value.declarationExplanation.status == DeclarationExplanationStatus.Stale
      }

      assertEquals(null, presenter.snapshot.value.declarationExplanation.result)
      assertTrue(
          presenter.snapshot.value.declarationExplanation.message.contains("Selection changed"))
    } finally {
      release.countDown()
      presenter.close()
    }
  }

  @Test
  fun sameTargetRetryAndRapidReplacementPublishOnlyTheLastExplanation() {
    val first = DelayedExplanation()
    val second = DelayedExplanation()
    val calls = AtomicInteger()
    val scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)
    val presenter =
        presenter(parentScope = scope) { method, path, _ ->
          assertEquals("POST" to "/api/projects/current/files/explanation", method to path)
          when (calls.incrementAndGet()) {
            1 -> first.respond(explanationJson("Run").replace("Explains Run.", "Old."))
            2 -> second.respond(explanationJson("Run").replace("Explains Run.", "Middle."))
            else -> response(explanationJson("Run").replace("Explains Run.", "Latest."))
          }
        }
    try {
      loadFile(presenter)
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      val before = presenter.snapshot.value
      val firstJob = explanationRequestJob(scope) { presenter.explainSelectedDeclaration() }
      first.awaitStarted()
      val secondJob =
          explanationRequestJob(scope) {
            presenter
                .explainSelectedDeclaration() // Same target, while the first transport is blocked.
          }
      second.awaitStarted()
      presenter.explainSelectedDeclaration() // Rapid replacement of the second request.
      eventually { presenter.snapshot.value.declarationExplanation.result?.summary == "Latest." }
      second.release()
      first.release()
      awaitExplanationCompletion(secondJob)
      awaitExplanationCompletion(firstJob)
      assertEquals(3, calls.get())
      assertEquals("Latest.", presenter.snapshot.value.declarationExplanation.result?.summary)
      assertEquals(
          DeclarationExplanationStatus.Current,
          presenter.snapshot.value.declarationExplanation.status)
      assertExplanationPreservesEditState(before, presenter.snapshot.value)
    } finally {
      first.release()
      second.release()
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun explicitExplanationCancelSurvivesLateCompletionAndAllowsExplicitRetry() {
    val delayed = DelayedExplanation()
    val calls = AtomicInteger()
    val scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)
    val presenter =
        presenter(parentScope = scope) { method, path, _ ->
          assertEquals("POST" to "/api/projects/current/files/explanation", method to path)
          if (calls.incrementAndGet() == 1) delayed.respond(explanationJson("Run"))
          else response(explanationJson("Run").replace("Explains Run.", "Retried."))
        }
    try {
      loadFile(presenter)
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      val before = presenter.snapshot.value
      val requestJob = explanationRequestJob(scope) { presenter.explainSelectedDeclaration() }
      delayed.awaitStarted()
      assertEquals(
          DeclarationExplanationStatus.Loading,
          presenter.snapshot.value.declarationExplanation.status)
      presenter.cancelDeclarationExplanation()
      delayed.release()
      awaitExplanationCompletion(requestJob)
      assertEquals(
          DeclarationExplanationStatus.Canceled,
          presenter.snapshot.value.declarationExplanation.status)
      assertNull(presenter.snapshot.value.declarationExplanation.result)
      assertEquals(1, calls.get()) // No background retry.
      assertExplanationPreservesEditState(before, presenter.snapshot.value)
      presenter.explainSelectedDeclaration()
      eventually { presenter.snapshot.value.declarationExplanation.result?.summary == "Retried." }
      assertEquals(2, calls.get())
    } finally {
      delayed.release()
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun projectAndFileIdentityChangesRejectDelayedExplanation() {
    for (change in listOf("project", "hash")) {
      val delayed = DelayedExplanation()
      val calls = AtomicInteger()
      val scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)
      val presenter =
          presenter(parentScope = scope) { method, path, _ ->
            assertEquals("POST" to "/api/projects/current/files/explanation", method to path)
            calls.incrementAndGet()
            delayed.respond(explanationJson("Run"))
          }
      try {
        loadFile(presenter)
        val requestJob = explanationRequestJob(scope) { presenter.explainSelectedDeclaration() }
        delayed.awaitStarted()
        val symbol = presenter.snapshot.value.state.selectedSymbol!!
        if (change == "project") {
          presenter.dispatch(
              DesktopEvent.ProjectLoaded(
                  project("new-project"), ProjectIndex("new-project", "revision")))
          presenter.dispatch(DesktopEvent.FileLoaded(file(), listOf(symbol)))
        } else {
          presenter.dispatch(
              DesktopEvent.FileLoaded(file().copy(contentHash = "new-hash"), listOf(symbol)))
        }
        presenter.dispatch(DesktopEvent.SymbolSelected(symbol))
        assertEquals(
            DeclarationExplanationStatus.Stale,
            presenter.snapshot.value.declarationExplanation.status,
            change)
        delayed.release()
        awaitExplanationCompletion(requestJob)
        assertEquals(
            DeclarationExplanationStatus.Stale,
            presenter.snapshot.value.declarationExplanation.status,
            change)
        assertNull(presenter.snapshot.value.declarationExplanation.result, change)
        assertEquals(1, calls.get(), change)
      } finally {
        delayed.release()
        presenter.close()
        scope.cancel()
      }
    }
  }

  @Test
  fun signatureAndRangeChangesRejectDelayedExplanationEvenForTheSameSymbol() {
    for (change in listOf("signature", "range")) {
      val delayed = DelayedExplanation()
      val scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)
      val presenter =
          presenter(parentScope = scope) { method, path, _ ->
            assertEquals("POST" to "/api/projects/current/files/explanation", method to path)
            delayed.respond(explanationJson("Run", "func Run()", 2, 4))
          }
      try {
        loadFile(presenter)
        val original =
            SymbolInfo(
                "Run", "function", "func Run()", 2, 4, confidence = "exact", atomicTarget = true)
        presenter.dispatch(DesktopEvent.FileLoaded(file(), listOf(original)))
        presenter.dispatch(DesktopEvent.SymbolSelected(original))
        val requestJob = explanationRequestJob(scope) { presenter.explainSelectedDeclaration() }
        delayed.awaitStarted()
        val changed =
            if (change == "signature") original.copy(signature = "func Run(x int)")
            else original.copy(startLine = 3, endLine = 5)
        presenter.dispatch(DesktopEvent.FileLoaded(file(), listOf(changed)))
        presenter.dispatch(DesktopEvent.SymbolSelected(changed))
        assertEquals(
            DeclarationExplanationStatus.Stale,
            presenter.snapshot.value.declarationExplanation.status,
            change)
        delayed.release()
        awaitExplanationCompletion(requestJob)
        assertNull(presenter.snapshot.value.declarationExplanation.result, change)
        assertEquals(
            DeclarationExplanationStatus.Stale,
            presenter.snapshot.value.declarationExplanation.status,
            change)
      } finally {
        delayed.release()
        presenter.close()
        scope.cancel()
      }
    }
  }

  @Test
  fun rejectedReplacementCannotLeaveAnOlderExplanationRunning() {
    val delayed = DelayedExplanation()
    val calls = AtomicInteger()
    val scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)
    val presenter =
        presenter(parentScope = scope) { method, path, _ ->
          assertEquals("POST" to "/api/projects/current/files/explanation", method to path)
          calls.incrementAndGet()
          delayed.respond(explanationJson("Run"))
        }
    try {
      loadFile(presenter)
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      val requestJob = explanationRequestJob(scope) { presenter.explainSelectedDeclaration() }
      delayed.awaitStarted()
      presenter.dispatch(DesktopEvent.FileLoaded(file(), emptyList()))
      val before = presenter.snapshot.value
      presenter.explainSelectedDeclaration()
      assertEquals(
          DeclarationExplanationStatus.Unavailable,
          presenter.snapshot.value.declarationExplanation.status)
      delayed.release()
      awaitExplanationCompletion(requestJob)
      assertEquals(
          DeclarationExplanationStatus.Unavailable,
          presenter.snapshot.value.declarationExplanation.status)
      assertNull(presenter.snapshot.value.declarationExplanation.result)
      assertEquals(1, calls.get())
      assertExplanationPreservesEditState(before, presenter.snapshot.value)
    } finally {
      delayed.release()
      presenter.close()
      scope.cancel()
    }
  }

  // Capture the launched job before releasing the fake transport; a transport return alone does
  // not mean the presenter has finished decoding and publishing (or rejecting) its response.
  private fun explanationRequestJob(scope: CoroutineScope, request: () -> Unit): Job {
    val lifetime = scope.coroutineContext[Job]!!.children.single()
    val before = lifetime.children.toSet()
    request()
    return (lifetime.children.toSet() - before).single()
  }

  private fun awaitExplanationCompletion(job: Job) = runBlocking {
    withTimeout(5_000) { job.join() }
  }

  private fun assertExplanationPreservesEditState(
      before: DesktopWorkflowSnapshot,
      after: DesktopWorkflowSnapshot,
  ) {
    assertEquals(before.state.review, after.state.review)
    assertEquals(before.state.checks, after.state.checks)
    assertEquals(before.generating, after.generating)
    assertEquals(before.draftValidationInProgress, after.draftValidationInProgress)
  }

  private class DelayedExplanation {
    private val started = CountDownLatch(1)
    private val released = CountDownLatch(1)

    fun respond(body: String): TransportResponse {
      started.countDown()
      // Simulate a transport that ignores interruption until its response is available.
      while (true) {
        try {
          if (released.await(5, TimeUnit.SECONDS)) break
          error("Timed out waiting to release the explanation response")
        } catch (_: InterruptedException) {
          // An interrupted HTTP call can still return a response; generation guards must win.
        }
      }
      return TransportResponse(200, body)
    }

    fun awaitStarted() = assertTrue(started.await(2, TimeUnit.SECONDS), "Request did not start")

    fun release() = released.countDown()
  }

  @Test
  fun cachedSymbolExplanationRemainsAvailableWithoutAProviderRequest() {
    val calls = AtomicInteger()
    val presenter = presenter { _, _, _ -> calls.incrementAndGet().let { response("{}") } }
    try {
      loadFile(presenter)
      presenter.dispatch(
          DesktopEvent.AnalysisLoaded(
              FileAnalysis(
                  path = "main.go",
                  status = "fresh",
                  symbolExplanations = mapOf("Run" to "Cached explanation."))))

      val inspector =
          symbolInspectorUiState(
              presenter.snapshot.value.state.selectedFile,
              presenter.snapshot.value.state.symbols,
              presenter.snapshot.value.state.selectedSymbol,
              presenter.snapshot.value.state.analysis,
              false,
              InspectorProviderState(false, false),
              null)!!
      assertEquals("Cached explanation.", inspector.selectedSymbol?.explanation)
      assertEquals(0, calls.get())
    } finally {
      presenter.close()
    }
  }

  @Test
  fun remoteDeclarationExplanationRequiresCurrentFunctionConsentBeforeAnyCall() {
    val explanationCalls = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "GET" to "/status" -> response("{\"status\":\"ok\",\"version\":\"v1\"}")
        "GET" to "/api/models/current" ->
            response(
                """{"scopes":{"function":{"scope":"function","profile":"function","model":"remote","remote_provider":true}}}""")
        "POST" to "/api/projects/current/files/explanation" -> {
          explanationCalls.incrementAndGet()
          response(explanationJson("Run"))
        }
        else -> error("unexpected request $method $path")
      }
    }
    try {
      presenter.refreshConnection()
      eventually { presenter.snapshot.value.model(ModelScope.Function).remoteProvider }
      loadFile(presenter)
      presenter.explainSelectedDeclaration()

      assertEquals(0, explanationCalls.get())
      assertEquals(
          DeclarationExplanationStatus.Failed,
          presenter.snapshot.value.declarationExplanation.status)
      assertTrue(presenter.snapshot.value.declarationExplanation.message.contains("Confirm"))
    } finally {
      presenter.close()
    }
  }

  @Test
  fun changedFunctionDestinationCannotInheritExplanationConsent() {
    val dispatcher = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    var destination = "destination-a"
    val bodies = mutableListOf<String>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = dispatcher) { method, path, body ->
          when (method to path) {
            "GET" to "/status" -> response("""{"status":"ok","version":"v1"}""")
            "GET" to "/api/models/current" -> response(functionCatalog(destination))
            "POST" to "/api/projects/current/files/explanation" -> {
              bodies += body.orEmpty()
              response(explanationJson("Run"))
            }
            else -> error("unexpected request $method $path")
          }
        }
    try {
      presenter.refreshConnection()
      dispatcher.runPending()
      loadFile(presenter)
      presenter.explainSelectedDeclaration()
      // A newly loaded catalog must cancel even a queued request to the old destination.
      destination = "destination-b"
      presenter.refreshConnection()
      dispatcher.runLast() // Connection job before the queued explanation job.
      dispatcher.runLast() // Catalog transport.
      dispatcher.runLast() // Apply the new catalog and invalidate the pending explanation.
      dispatcher.runPending()
      assertTrue(bodies.isEmpty())
      assertFalse(presenter.snapshot.value.providerConfirmed(ModelScope.Function))
      presenter.explainSelectedDeclaration()
      dispatcher.runPending()
      assertTrue(bodies.isEmpty())
      assertEquals(
          DeclarationExplanationStatus.Failed,
          presenter.snapshot.value.declarationExplanation.status)
      presenter.setProviderConfirmation(ModelScope.Function, true)
      presenter.explainSelectedDeclaration()
      dispatcher.runPending()
      assertEquals(1, bodies.size)
      assertTrue(bodies.single().contains("\"confirm_remote_provider\":true"))
    } finally {
      presenter.close()
      scope.cancel()
      dispatcher.runPending()
    }
  }

  @Test
  fun nonFunctionCatalogChangeResetsConsentAndCancelsQueuedExplanation() {
    val dispatcher = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    var bugModel = "bug-a"
    val bodies = mutableListOf<String>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = dispatcher) { method, path, body ->
          when (method to path) {
            "GET" to "/status" -> response("""{"status":"ok","version":"v1"}""")
            "GET" to "/api/models/current" ->
                response(
                    """{"scopes":{"function":{"scope":"function","profile":"remote","model":"destination-a","remote_provider":true},"bug":{"scope":"bug","profile":"remote","model":"$bugModel","remote_provider":true}}}""")
            "POST" to "/api/projects/current/files/explanation" -> {
              bodies += body.orEmpty()
              response(explanationJson("Run"))
            }
            else -> error("unexpected request $method $path")
          }
        }
    try {
      presenter.refreshConnection()
      dispatcher.runPending()
      loadFile(presenter)
      presenter.setProviderConfirmation(ModelScope.Function, true)
      presenter.explainSelectedDeclaration()
      bugModel = "bug-b"
      presenter.refreshConnection()
      dispatcher.runLast() // Connection job before the queued explanation job.
      dispatcher.runLast() // Catalog transport.
      dispatcher.runLast() // Apply the catalog change before explanation transport.
      dispatcher.runPending()
      assertTrue(bodies.isEmpty())
      assertFalse(presenter.snapshot.value.providerConfirmed(ModelScope.Function))
      assertEquals(
          DeclarationExplanationStatus.Stale,
          presenter.snapshot.value.declarationExplanation.status)
      presenter.explainSelectedDeclaration()
      dispatcher.runPending()
      assertTrue(bodies.isEmpty())
      assertEquals(
          DeclarationExplanationStatus.Failed,
          presenter.snapshot.value.declarationExplanation.status)
    } finally {
      presenter.close()
      scope.cancel()
      dispatcher.runPending()
    }
  }

  @Test
  fun explanationConflictPublishesStaleWhileOtherFailuresRemainFailed() {
    val conflictPresenter = presenter { method, path, _ ->
      if (method == "POST" && path == "/api/projects/current/files/explanation")
          TransportResponse(
              409, """{"type":"conflict","message":"stale","user_message":"Refresh."}""")
      else error("unexpected request $method $path")
    }
    val failedPresenter = presenter { method, path, _ ->
      if (method == "POST" && path == "/api/projects/current/files/explanation")
          TransportResponse(
              502,
              """{"type":"internal","message":"provider failed","user_message":"Try again."}""")
      else error("unexpected request $method $path")
    }
    try {
      loadFile(conflictPresenter)
      conflictPresenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      val beforeConflict = conflictPresenter.snapshot.value
      conflictPresenter.explainSelectedDeclaration()
      eventually {
        conflictPresenter.snapshot.value.declarationExplanation.status ==
            DeclarationExplanationStatus.Stale
      }
      assertEquals(null, conflictPresenter.snapshot.value.declarationExplanation.result)
      assertExplanationPreservesEditState(beforeConflict, conflictPresenter.snapshot.value)

      loadFile(failedPresenter)
      failedPresenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      val beforeFailure = failedPresenter.snapshot.value
      failedPresenter.explainSelectedDeclaration()
      eventually {
        failedPresenter.snapshot.value.declarationExplanation.status ==
            DeclarationExplanationStatus.Failed
      }
      assertEquals(null, failedPresenter.snapshot.value.declarationExplanation.result)
      assertExplanationPreservesEditState(beforeFailure, failedPresenter.snapshot.value)
    } finally {
      conflictPresenter.close()
      failedPresenter.close()
    }
  }

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
      presenter.dispatch(
          DesktopEvent.IndexRefreshed(
              ProjectIndex(
                  "project",
                  "revision",
                  files =
                      listOf("first.go", "second.go").map {
                        IndexedFile(it, "base", "Go", false)
                      })))
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
  fun foreignStartAndCancelReportsRetainEvidenceWithoutPollingOrFindingsRefresh() {
    for (route in listOf("POST", "DELETE")) {
      for ((foreignId, foreignRevision) in listOf("other" to "revision", "project" to "other")) {
        val polls = AtomicInteger()
        val findings = AtomicInteger()
        val presenter = presenter { method, path, _ ->
          when {
            method == route && path.contains("/scan") ->
                response(
                    """{"project_id":"$foreignId","project_revision":"$foreignRevision","status":"completed"}""")
            method == "GET" && path.contains("/scan?") -> {
              polls.incrementAndGet()
              TransportResponse(204, "")
            }
            path.contains("/findings?") -> {
              findings.incrementAndGet()
              response("{}")
            }
            else -> error("Unexpected $method $path")
          }
        }
        try {
          loadProject(presenter)
          val retained = GoScanReport("project", "revision", status = "completed")
          presenter.dispatch(DesktopEvent.GoScanLoaded(retained))
          if (route == "POST") presenter.runVerifiedScan()
          else {
            presenter.dispatch(DesktopEvent.GoScanLoaded(retained.copy(status = "running")))
            presenter.cancelVerifiedScan()
          }
          eventually {
            presenter.snapshot.value.state.verifiedScan.operation is VerifiedScanOperation.Failed
          }
          assertEquals(
              if (route == "POST") retained else retained.copy(status = "running"),
              presenter.snapshot.value.state.findings.scan)
          assertTrue(
              (presenter.snapshot.value.state.verifiedScan.operation
                      as VerifiedScanOperation.Failed)
                  .message
                  .contains("identity"))
          assertEquals(0, polls.get())
          assertEquals(0, findings.get())
        } finally {
          presenter.close()
        }
      }
    }
  }

  @Test
  fun foreignPollStopsWithoutPublishingOrRefreshingFindings() {
    for ((foreignId, foreignRevision) in listOf("other" to "revision", "project" to "other")) {
      val polls = AtomicInteger()
      val findings = AtomicInteger()
      val presenter = presenter { method, path, _ ->
        when (method to path) {
          "POST" to "/api/projects/current/scan" ->
              response(
                  """{"project_id":"project","project_revision":"revision","status":"running"}""")
          "GET" to "/api/projects/current/scan?project_revision=revision" -> {
            polls.incrementAndGet()
            response(
                """{"project_id":"$foreignId","project_revision":"$foreignRevision","status":"completed"}""")
          }
          "GET" to "/api/projects/current/findings?project_revision=revision" -> {
            findings.incrementAndGet()
            response("{}")
          }
          else -> error("Unexpected $method $path")
        }
      }
      try {
        loadProject(presenter)
        presenter.dispatch(DesktopEvent.VerifiedScanReadUpdated(VerifiedScanRead.Absent))
        presenter.runVerifiedScan()
        eventually {
          presenter.snapshot.value.state.verifiedScan.read is VerifiedScanRead.PollUnavailable
        }
        assertEquals("running", presenter.snapshot.value.state.findings.scan?.status)
        assertEquals(1, polls.get())
        assertEquals(0, findings.get())
      } finally {
        presenter.close()
      }
    }
  }

  @Test
  fun foreignWorkspaceScanCannotReplaceRetainedEvidence() {
    for ((foreignId, foreignRevision) in listOf("other" to "revision", "project" to "other")) {
      val main = QueuedDispatcher()
      val io = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + main)
      val findings = AtomicInteger()
      val presenter =
          presenter(parentScope = scope, ioDispatcher = io) { method, path, _ ->
            when {
              path.endsWith("/reindex") -> response(indexJson())
              path.contains("/analysis/run?") -> TransportResponse(204, "")
              path.contains("/analysis/selection?") || path.contains("/overview?") -> response("{}")
              path.contains("/findings?") -> {
                findings.incrementAndGet()
                response("{}")
              }
              path.contains("/scan?") ->
                  response(
                      """{"project_id":"$foreignId","project_revision":"$foreignRevision","status":"completed"}""")
              else -> error("Unexpected $method $path")
            }
          }
      try {
        loadProject(presenter)
        val retained = GoScanReport("project", "revision", status = "completed")
        presenter.dispatch(DesktopEvent.GoScanLoaded(retained))
        val retainedFindings =
            listOf(
                UnifiedFinding(
                    id = "saved",
                    category = "bugs",
                    projectId = "project",
                    projectRevision = "revision",
                    location = FindingLocation("main.go")))
        presenter.dispatch(DesktopEvent.FindingsLoaded(retainedFindings))
        presenter.reindexProject()
        repeat(12) {
          main.runPending()
          io.runPending()
        }
        main.runPending()
        assertEquals(retained, presenter.snapshot.value.state.findings.scan)
        assertEquals(retainedFindings, presenter.snapshot.value.state.findings.findings)
        assertTrue(presenter.snapshot.value.state.verifiedScan.read is VerifiedScanRead.Unavailable)
        assertTrue(
            presenter.snapshot.value.state.projectState.detailsOutcome
                is ProjectDetailsOutcome.Unavailable)
        assertEquals(1, findings.get())
      } finally {
        presenter.close()
        scope.cancel()
      }
    }
  }

  @Test
  fun failedInitialWorkspaceScanReadIsUnavailableNotAbsent() {
    // Confine reducer updates as in the UI, independently of transport continuations.
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { method, path, _ ->
          when (method to path) {
            "POST" to "/api/projects/current/reindex" -> response(indexJson())
            "GET" to
                "/api/projects/current/analysis/run?project_id=project&project_revision=revision" ->
                TransportResponse(204, "")
            "GET" to "/api/projects/current/analysis/selection?project_revision=revision",
            "GET" to "/api/projects/current/overview?project_revision=revision",
            "GET" to "/api/projects/current/findings?project_revision=revision" -> response("{}")
            "GET" to "/api/projects/current/scan?project_revision=revision" ->
                TransportResponse(503, "status unavailable")
            else -> error("Unexpected $method $path")
          }
        }
    try {
      loadProject(presenter)
      assertEquals(VerifiedScanRead.Unread, presenter.snapshot.value.state.verifiedScan.read)
      val retained = GoScanReport("project", "revision", status = "completed")
      presenter.dispatch(DesktopEvent.GoScanLoaded(retained))
      presenter.reindexProject()
      main.runPending()
      io.runPending()
      main.runPending()
      assertEquals(VerifiedScanRead.Reading, presenter.snapshot.value.state.verifiedScan.read)
      io.runPending()
      main.runPending()
      val state = presenter.snapshot.value.state
      val unavailable = state.verifiedScan.read as VerifiedScanRead.Unavailable
      assertTrue(unavailable.message.contains("Daemon returned 503"))
      assertTrue(state.projectState.detailsOutcome is ProjectDetailsOutcome.Unavailable)
      assertEquals(retained, state.findings.scan)
      assertNull(state.jobs.error)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun statusRecoveryReadsOnlyScanAndDistinguishesAbsenceFromReadFailure() {
    val requests = Collections.synchronizedList(mutableListOf<String>())
    var fail = true
    val presenter = presenter { method, path, _ ->
      requests += "$method $path"
      when (method to path) {
        "GET" to "/api/projects/current/scan?project_revision=revision" ->
            if (fail) TransportResponse(503, "status offline") else TransportResponse(204, "")
        else -> error("Unexpected $method $path")
      }
    }
    try {
      loadProject(presenter)
      val retained = GoScanReport("project", "revision", status = "completed")
      presenter.dispatch(DesktopEvent.GoScanLoaded(retained))
      presenter.refreshVerifiedScanStatus()
      eventually {
        presenter.snapshot.value.state.verifiedScan.read is VerifiedScanRead.Unavailable
      }
      assertEquals(retained, presenter.snapshot.value.state.findings.scan)
      assertTrue(
          (presenter.snapshot.value.state.verifiedScan.read as VerifiedScanRead.Unavailable)
              .message
              .contains("Unable to refresh scan status"))
      fail = false
      presenter.refreshVerifiedScanStatus()
      eventually { presenter.snapshot.value.state.verifiedScan.read == VerifiedScanRead.Absent }
      assertEquals(retained, presenter.snapshot.value.state.findings.scan)
      assertEquals(
          listOf(
              "GET /api/projects/current/scan?project_revision=revision",
              "GET /api/projects/current/scan?project_revision=revision"),
          requests.toList())
      assertNull(presenter.snapshot.value.state.jobs.error)
    } finally {
      presenter.close()
    }
  }

  @Test
  fun statusRecoveryResumesOnePollerAndReportsPollFailureBesideDiagnostics() {
    val pollStarted = CountDownLatch(1)
    val releasePoll = CountDownLatch(1)
    val reads = AtomicInteger()
    val requests = Collections.synchronizedList(mutableListOf<String>())
    val presenter = presenter { method, path, _ ->
      requests += "$method $path"
      when (method to path) {
        "GET" to "/api/projects/current/scan?project_revision=revision" -> {
          if (reads.incrementAndGet() == 1)
              response(
                  """{"project_id":"project","project_revision":"revision","status":"running"}""")
          else {
            pollStarted.countDown()
            assertTrue(releasePoll.await(5, TimeUnit.SECONDS))
            TransportResponse(503, "poll offline")
          }
        }
        else -> error("Unexpected $method $path")
      }
    }
    try {
      loadProject(presenter)
      presenter.refreshVerifiedScanStatus()
      assertTrue(pollStarted.await(2, TimeUnit.SECONDS))
      assertEquals(VerifiedScanRead.Loaded, presenter.snapshot.value.state.verifiedScan.read)
      releasePoll.countDown()
      eventually {
        presenter.snapshot.value.state.verifiedScan.read is VerifiedScanRead.PollUnavailable
      }
      assertEquals("running", presenter.snapshot.value.state.findings.scan?.status)
      assertTrue(
          (presenter.snapshot.value.state.verifiedScan.read as VerifiedScanRead.PollUnavailable)
              .message
              .contains("Live scan status"))
      assertEquals(2, reads.get())
      assertTrue(requests.all { it == "GET /api/projects/current/scan?project_revision=revision" })
      assertNull(presenter.snapshot.value.state.jobs.error)
    } finally {
      releasePoll.countDown()
      presenter.close()
    }
  }

  @Test
  fun foreignStatusRecoveryAndLateProjectResponseCannotPublish() {
    for (foreign in listOf("project_id" to "other", "project_revision" to "other")) {
      val pollCount = AtomicInteger()
      val findingsCount = AtomicInteger()
      val presenter = presenter { method, path, _ ->
        when (method to path) {
          "GET" to "/api/projects/current/scan?project_revision=revision" ->
              response(
                  """{"project_id":"${if (foreign.first == "project_id") foreign.second else "project"}","project_revision":"${if (foreign.first == "project_revision") foreign.second else "revision"}","status":"completed"}""")
          "GET" to "/api/projects/current/findings?project_revision=revision" -> {
            findingsCount.incrementAndGet()
            response("{}")
          }
          else -> {
            pollCount.incrementAndGet()
            error("Unexpected $method $path")
          }
        }
      }
      try {
        loadProject(presenter)
        val retained = GoScanReport("project", "revision", status = "running")
        presenter.dispatch(DesktopEvent.GoScanLoaded(retained))
        presenter.refreshVerifiedScanStatus()
        eventually {
          presenter.snapshot.value.state.verifiedScan.read is VerifiedScanRead.Unavailable
        }
        assertEquals(retained, presenter.snapshot.value.state.findings.scan)
        assertEquals(0, findingsCount.get())
        assertEquals(0, pollCount.get())
      } finally {
        presenter.close()
      }
    }

    val requested = CountDownLatch(1)
    val release = CountDownLatch(1)
    val scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)
    val presenter =
        presenter(parentScope = scope) { method, path, _ ->
          when (method to path) {
            "GET" to "/api/projects/current/scan?project_revision=revision" -> {
              requested.countDown()
              assertTrue(release.await(5, TimeUnit.SECONDS))
              response(
                  """{"project_id":"project","project_revision":"revision","status":"completed"}""")
            }
            else -> error("Unexpected $method $path")
          }
        }
    try {
      loadProject(presenter)
      val lifetime = scope.coroutineContext[Job]!!.children.single()
      val before = lifetime.children.toSet()
      presenter.refreshVerifiedScanStatus()
      assertTrue(requested.await(2, TimeUnit.SECONDS))
      val readJob = (lifetime.children.toSet() - before).single()
      presenter.dispatch(
          DesktopEvent.ProjectLoaded(
              project("replacement", "new"), ProjectIndex("replacement", "new")))
      release.countDown()
      runBlocking { withTimeout(2_000) { readJob.join() } }
      assertEquals(VerifiedScanRead.Unread, presenter.snapshot.value.state.verifiedScan.read)
      assertNull(presenter.snapshot.value.state.findings.scan)
    } finally {
      release.countDown()
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun terminalFindingsFailureRetainsRowsAndLateEnrichmentCannotReplaceNewScan() {
    val scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)
    val firstFindingsStarted = CountDownLatch(1)
    val releaseFirstFindings = CountDownLatch(1)
    val requests = Collections.synchronizedList(mutableListOf<String>())
    val findingsReads = AtomicInteger()
    val presenter =
        presenter(parentScope = scope) { method, path, _ ->
          requests += "$method $path"
          when (method to path) {
            "GET" to "/api/projects/current/scan?project_revision=revision" ->
                response(
                    """{"project_id":"project","project_revision":"revision","status":"completed"}""")
            "GET" to "/api/projects/current/findings?project_revision=revision" -> {
              if (findingsReads.incrementAndGet() == 1) {
                firstFindingsStarted.countDown()
                assertTrue(releaseFirstFindings.await(5, TimeUnit.SECONDS))
                response(
                    """{"findings":[{"id":"late","category":"bugs","project_id":"project","project_revision":"revision","location":{"path":"main.go"}}]}""")
              } else TransportResponse(503, "findings offline")
            }
            else -> error("Unexpected $method $path")
          }
        }
    try {
      loadProject(presenter)
      val rows =
          listOf(
              UnifiedFinding(
                  id = "saved",
                  category = "bugs",
                  projectId = "project",
                  projectRevision = "revision",
                  location = FindingLocation("main.go")))
      presenter.dispatch(DesktopEvent.FindingsLoaded(rows))
      val lifetime = scope.coroutineContext[Job]!!.children.single()
      val before = lifetime.children.toSet()
      presenter.refreshVerifiedScanStatus()
      assertTrue(firstFindingsStarted.await(2, TimeUnit.SECONDS))
      val firstRefreshJob = (lifetime.children.toSet() - before).single()
      assertEquals(
          VerifiedScanFindingsRefresh.Refreshing,
          presenter.snapshot.value.state.verifiedScan.findingsRefresh)
      presenter.refreshVerifiedScanStatus()
      eventually {
        presenter.snapshot.value.state.verifiedScan.findingsRefresh is
            VerifiedScanFindingsRefresh.Unavailable
      }
      assertTrue(
          (presenter.snapshot.value.state.verifiedScan.findingsRefresh
                  as VerifiedScanFindingsRefresh.Unavailable)
              .message
              .contains("previous rows may be stale"))
      releaseFirstFindings.countDown()
      runBlocking { withTimeout(2_000) { firstRefreshJob.join() } }
      assertEquals(rows, presenter.snapshot.value.state.findings.findings)
      assertEquals("completed", presenter.snapshot.value.state.findings.scan?.status)
      assertEquals(2, requests.count { it.contains("/findings?") })
    } finally {
      releaseFirstFindings.countDown()
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun pendingVerifiedScanStartCannotBeCanceledBeforeAcceptance() {
    val startRequested = CountDownLatch(1)
    val releaseStart = CountDownLatch(1)
    val findingsRefreshes = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "POST" to "/api/projects/current/scan" -> {
          startRequested.countDown()
          releaseStart.await(2, TimeUnit.SECONDS)
          response(
              "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"running\"}")
        }
        "DELETE" to "/api/projects/current/scan?project_revision=revision" ->
            response(
                "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"canceled\"}")
        "GET" to "/api/projects/current/findings?project_revision=revision" -> {
          findingsRefreshes.incrementAndGet()
          response("{}")
        }
        else -> response("{}")
      }
    }
    try {
      loadProject(presenter)
      presenter.dispatch(DesktopEvent.VerifiedScanReadUpdated(VerifiedScanRead.Absent))
      presenter.runVerifiedScan()
      assertTrue(startRequested.await(1, TimeUnit.SECONDS))
      presenter.cancelVerifiedScan()
      assertEquals(
          VerifiedScanOperation.Starting, presenter.snapshot.value.state.verifiedScan.operation)
      releaseStart.countDown()
      eventually { presenter.snapshot.value.state.findings.scan?.status == "running" }

      assertEquals(0, findingsRefreshes.get())
    } finally {
      releaseStart.countDown()
      presenter.close()
    }
  }

  @Test
  fun repeatedVerifiedScanStartDuringPendingRequestIsCoalesced() {
    val firstStartRequested = CountDownLatch(1)
    val releaseFirstStart = CountDownLatch(1)
    val starts = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "POST" to "/api/projects/current/scan" ->
            if (starts.incrementAndGet() == 1) {
              firstStartRequested.countDown()
              releaseFirstStart.await(2, TimeUnit.SECONDS)
              response(
                  "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"canceled\"}")
            } else
                response(
                    "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"running\"}")
        "GET" to "/api/projects/current/scan?project_revision=revision" ->
            response(
                "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"completed\"}")
        else -> response("{}")
      }
    }
    try {
      loadProject(presenter)
      presenter.dispatch(DesktopEvent.VerifiedScanReadUpdated(VerifiedScanRead.Absent))
      presenter.runVerifiedScan()
      assertTrue(firstStartRequested.await(1, TimeUnit.SECONDS))
      presenter.runVerifiedScan()
      assertEquals(1, starts.get())
      releaseFirstStart.countDown()
      eventually { presenter.snapshot.value.state.findings.scan?.status == "canceled" }

      assertEquals(1, starts.get())
    } finally {
      releaseFirstStart.countDown()
      presenter.close()
    }
  }

  @Test
  fun cancelingVerifiedScanPollsToTerminalStateAndRefreshesFindingsOnce() {
    val polls = AtomicInteger()
    val findingsRefreshes = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "DELETE" to "/api/projects/current/scan?project_revision=revision" ->
            response(
                "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"canceling\"}")
        "GET" to "/api/projects/current/scan?project_revision=revision" -> {
          polls.incrementAndGet()
          response(
              "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"canceled\"}")
        }
        "GET" to "/api/projects/current/findings?project_revision=revision" -> {
          findingsRefreshes.incrementAndGet()
          response("{}")
        }
        else -> response("{}")
      }
    }
    try {
      loadProject(presenter)
      presenter.dispatch(
          DesktopEvent.GoScanLoaded(
              GoScanReport(
                  projectId = "project", projectRevision = "revision", status = "running")))
      presenter.cancelVerifiedScan()
      eventually { presenter.snapshot.value.state.findings.scan?.status == "canceled" }
      eventually {
        findingsRefreshes.get() == 1 &&
            presenter.snapshot.value.state.verifiedScan.findingsRefresh ==
                VerifiedScanFindingsRefresh.Current
      }

      assertEquals(1, polls.get())
      assertEquals(1, findingsRefreshes.get())
    } finally {
      presenter.close()
    }
  }

  @Test
  fun terminalPollWhileCancelIsPendingOutranksLateCancelResponseOrFailure() {
    for (cancelFails in listOf(false, true)) {
      val scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)
      val pollStarted = CountDownLatch(1)
      val releasePoll = CountDownLatch(1)
      val cancelStarted = CountDownLatch(1)
      val releaseCancel = CountDownLatch(1)
      val polls = AtomicInteger()
      val findingsRefreshes = AtomicInteger()
      val presenter =
          presenter(parentScope = scope) { method, path, _ ->
            when (method to path) {
              "POST" to "/api/projects/current/scan" ->
                  response(
                      """{"project_id":"project","project_revision":"revision","status":"running"}""")
              "GET" to "/api/projects/current/scan?project_revision=revision" -> {
                polls.incrementAndGet()
                pollStarted.countDown()
                assertTrue(releasePoll.await(5, TimeUnit.SECONDS))
                response(
                    """{"project_id":"project","project_revision":"revision","status":"canceled"}""")
              }
              "DELETE" to "/api/projects/current/scan?project_revision=revision" -> {
                cancelStarted.countDown()
                assertTrue(releaseCancel.await(5, TimeUnit.SECONDS))
                if (cancelFails) TransportResponse(504, "cancel timed out")
                else
                    response(
                        """{"project_id":"project","project_revision":"revision","status":"running"}""")
              }
              "GET" to "/api/projects/current/findings?project_revision=revision" -> {
                findingsRefreshes.incrementAndGet()
                response("{}")
              }
              else -> error("Unexpected $method $path")
            }
          }
      try {
        loadProject(presenter)
        presenter.dispatch(DesktopEvent.VerifiedScanReadUpdated(VerifiedScanRead.Absent))
        presenter.runVerifiedScan()
        assertTrue(pollStarted.await(2, TimeUnit.SECONDS))
        val presenterLifetime = scope.coroutineContext[Job]!!.children.single()
        val beforeCancel = presenterLifetime.children.toSet()
        presenter.cancelVerifiedScan()
        val cancelJob = (presenterLifetime.children.toSet() - beforeCancel).single()
        assertTrue(cancelStarted.await(2, TimeUnit.SECONDS))
        assertEquals(
            VerifiedScanOperation.CancellationRequested,
            presenter.snapshot.value.state.verifiedScan.operation)
        releasePoll.countDown()
        eventually { presenter.snapshot.value.state.findings.scan?.status == "canceled" }
        eventually { findingsRefreshes.get() == 1 }
        assertEquals(
            VerifiedScanOperation.Idle, presenter.snapshot.value.state.verifiedScan.operation)

        releaseCancel.countDown()
        runBlocking { withTimeout(2_000) { cancelJob.join() } }
        assertEquals("canceled", presenter.snapshot.value.state.findings.scan?.status)
        assertEquals(
            VerifiedScanOperation.Idle, presenter.snapshot.value.state.verifiedScan.operation)
        assertEquals(1, polls.get())
        assertEquals(1, findingsRefreshes.get())
        assertNull(presenter.snapshot.value.state.jobs.error)
      } finally {
        releasePoll.countDown()
        releaseCancel.countDown()
        presenter.close()
        scope.cancel()
      }
    }
  }

  @Test
  fun pollFailureWhileCancelIsPendingRetainsReportAndMarksLiveStatusUnavailable() {
    val scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)
    val pollStarted = CountDownLatch(1)
    val releasePoll = CountDownLatch(1)
    val cancelStarted = CountDownLatch(1)
    val releaseCancel = CountDownLatch(1)
    val polls = AtomicInteger()
    val findingsRefreshes = AtomicInteger()
    val presenter =
        presenter(parentScope = scope) { method, path, _ ->
          when (method to path) {
            "POST" to "/api/projects/current/scan" ->
                response(
                    """{"project_id":"project","project_revision":"revision","status":"running"}""")
            "GET" to "/api/projects/current/scan?project_revision=revision" -> {
              polls.incrementAndGet()
              pollStarted.countDown()
              assertTrue(releasePoll.await(5, TimeUnit.SECONDS))
              TransportResponse(503, "poll offline")
            }
            "DELETE" to "/api/projects/current/scan?project_revision=revision" -> {
              cancelStarted.countDown()
              assertTrue(releaseCancel.await(5, TimeUnit.SECONDS))
              response(
                  """{"project_id":"project","project_revision":"revision","status":"running"}""")
            }
            "GET" to "/api/projects/current/findings?project_revision=revision" -> {
              findingsRefreshes.incrementAndGet()
              response("{}")
            }
            else -> error("Unexpected $method $path")
          }
        }
    try {
      loadProject(presenter)
      presenter.dispatch(DesktopEvent.VerifiedScanReadUpdated(VerifiedScanRead.Absent))
      presenter.runVerifiedScan()
      assertTrue(pollStarted.await(2, TimeUnit.SECONDS))
      val retained = presenter.snapshot.value.state.findings.scan
      presenter.cancelVerifiedScan()
      assertTrue(cancelStarted.await(2, TimeUnit.SECONDS))
      assertEquals(
          VerifiedScanOperation.CancellationRequested,
          presenter.snapshot.value.state.verifiedScan.operation)
      releasePoll.countDown()
      eventually {
        presenter.snapshot.value.state.verifiedScan.read is VerifiedScanRead.PollUnavailable &&
            presenter.snapshot.value.state.verifiedScan.operation is
                VerifiedScanOperation.CancellationUnconfirmed
      }
      assertEquals(retained, presenter.snapshot.value.state.findings.scan)
      assertTrue(
          presenter.snapshot.value.state.verifiedScan.operation
              is VerifiedScanOperation.CancellationUnconfirmed)
      assertTrue(
          (presenter.snapshot.value.state.verifiedScan.read as VerifiedScanRead.PollUnavailable)
              .message
              .contains("Live scan status"))
      assertEquals(1, polls.get())
      assertEquals(0, findingsRefreshes.get())
      assertNull(presenter.snapshot.value.state.jobs.error)
    } finally {
      releasePoll.countDown()
      releaseCancel.countDown()
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun admittedScanWithLostCancellationObservationRecoversWithoutRepeatingMutation() {
    for (lostStatus in
        listOf(
            TransportResponse(503, "poll offline"),
            TransportResponse(204, ""),
            response(
                """{"project_id":"project","project_revision":"revision","status":"unknown"}"""),
            response(
                """{"project_id":"foreign","project_revision":"revision","status":"canceled"}"""))) {
      val initialPoll = CountDownLatch(1)
      val releaseInitialPoll = CountDownLatch(1)
      val reads = AtomicInteger()
      val requests = Collections.synchronizedList(mutableListOf<String>())
      val presenter =
          presenter(interceptTrust = false) { method, path, body ->
            requests += "$method $path"
            when (method to path) {
              "GET" to "/api/projects/current/execution-trust?project_revision=revision" ->
                  response(
                      """{"project_id":"project","project_revision":"revision","trusted":false,"commands":[["go","test","./..."]]}""")
              "POST" to "/api/projects/current/execution-trust" -> {
                assertTrue(body.orEmpty().contains("\"confirm\":true"))
                response(
                    """{"project_id":"project","project_revision":"revision","trusted":true,"commands":[["go","test","./..."]]}""")
              }
              "POST" to "/api/projects/current/scan",
              "DELETE" to "/api/projects/current/scan?project_revision=revision" ->
                  response(
                      """{"project_id":"project","project_revision":"revision","status":"running"}""")
              "GET" to "/api/projects/current/scan?project_revision=revision" ->
                  when (reads.incrementAndGet()) {
                    1 -> {
                      initialPoll.countDown()
                      assertTrue(releaseInitialPoll.await(5, TimeUnit.SECONDS))
                      response(
                          """{"project_id":"project","project_revision":"revision","status":"running"}""")
                    }
                    2 -> lostStatus
                    3 ->
                        response(
                            """{"project_id":"project","project_revision":"revision","status":"canceled","phases":[{"name":"tests","state":"canceled","command":["go","test","./..."],"output":"partial test output","exit_code":143}]}""")
                    else -> error("Unexpected duplicate poll")
                  }
              "GET" to "/api/projects/current/findings?project_revision=revision" ->
                  response(
                      """{"findings":[{"id":"tool","category":"bugs","project_id":"project","project_revision":"revision","confidence":"tool_reported","location":{"path":"main.go"}}]}""")
              else -> error("Unexpected $method $path")
            }
          }
      try {
        loadProject(presenter)
        presenter.dispatch(DesktopEvent.VerifiedScanReadUpdated(VerifiedScanRead.Absent))
        presenter.dispatch(DesktopEvent.WorkspaceSelected(Workspace.Bugs))
        assertTrue(requests.isEmpty())
        presenter.runVerifiedScan()
        assertTrue(initialPoll.await(2, TimeUnit.SECONDS))
        presenter.cancelVerifiedScan()
        eventually {
          presenter.snapshot.value.state.verifiedScan.operation is
              VerifiedScanOperation.CancellationUnconfirmed
        }
        assertEquals(2, reads.get())
        assertTrue(presenter.snapshot.value.state.findings.scan != null)
        presenter.refreshVerifiedScanStatus()
        eventually {
          presenter.snapshot.value.state.findings.scan?.status == "canceled" &&
              presenter.snapshot.value.state.verifiedScan.findingsRefresh ==
                  VerifiedScanFindingsRefresh.Current
        }
        assertEquals(
            VerifiedScanOperation.Idle, presenter.snapshot.value.state.verifiedScan.operation)
        assertEquals(
            "partial test output",
            presenter.snapshot.value.state.findings.scan!!.phases.single().output)
        assertEquals("tool", presenter.snapshot.value.state.projectBugFindings().single().id)
        assertEquals(
            listOf(
                "GET /api/projects/current/execution-trust?project_revision=revision",
                "POST /api/projects/current/execution-trust",
                "POST /api/projects/current/scan",
                "GET /api/projects/current/scan?project_revision=revision",
                "DELETE /api/projects/current/scan?project_revision=revision",
                "GET /api/projects/current/scan?project_revision=revision",
                "GET /api/projects/current/scan?project_revision=revision",
                "GET /api/projects/current/findings?project_revision=revision"),
            requests.toList())
        assertNull(presenter.snapshot.value.state.jobs.error)
      } finally {
        releaseInitialPoll.countDown()
        presenter.close()
      }
    }
  }

  @Test
  fun activeVerifiedScanCannotStartAgainWhilePolling() {
    val initialPollStarted = CountDownLatch(1)
    val releaseInitialPoll = CountDownLatch(1)
    val starts = AtomicInteger()
    val polls = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "POST" to "/api/projects/current/scan" ->
            if (starts.incrementAndGet() == 1)
                response(
                    "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"running\"}")
            else TransportResponse(500, "start failed")
        "GET" to "/api/projects/current/scan?project_revision=revision" ->
            if (polls.incrementAndGet() == 1) {
              initialPollStarted.countDown()
              releaseInitialPoll.await(2, TimeUnit.SECONDS)
              response(
                  "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"running\"}")
            } else
                response(
                    "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"canceled\"}")
        else -> response("{}")
      }
    }
    try {
      loadProject(presenter)
      presenter.dispatch(DesktopEvent.VerifiedScanReadUpdated(VerifiedScanRead.Absent))
      presenter.runVerifiedScan()
      assertTrue(initialPollStarted.await(1, TimeUnit.SECONDS))
      presenter.runVerifiedScan()
      assertEquals(1, starts.get())
      releaseInitialPoll.countDown()
      eventually { presenter.snapshot.value.state.findings.scan?.status == "canceled" }

      assertEquals(1, starts.get())
      assertEquals(2, polls.get())
      assertNull(presenter.snapshot.value.state.jobs.error)
    } finally {
      releaseInitialPoll.countDown()
      presenter.close()
    }
  }

  @Test
  fun failedVerifiedScanCancelRetainsEvidenceUntilTerminalPoll() {
    val initialPollStarted = CountDownLatch(1)
    val releaseInitialPoll = CountDownLatch(1)
    val releaseTerminalPoll = CountDownLatch(1)
    val polls = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "POST" to "/api/projects/current/scan" ->
            response(
                "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"running\"}")
        "DELETE" to "/api/projects/current/scan?project_revision=revision" ->
            TransportResponse(500, "cancel failed")
        "GET" to "/api/projects/current/scan?project_revision=revision" ->
            if (polls.incrementAndGet() == 1) {
              initialPollStarted.countDown()
              releaseInitialPoll.await(2, TimeUnit.SECONDS)
              response(
                  "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"canceling\"}")
            } else {
              releaseTerminalPoll.await(2, TimeUnit.SECONDS)
              response(
                  "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"canceled\"}")
            }
        else -> response("{}")
      }
    }
    try {
      loadProject(presenter)
      presenter.dispatch(DesktopEvent.VerifiedScanReadUpdated(VerifiedScanRead.Absent))
      presenter.runVerifiedScan()
      assertTrue(initialPollStarted.await(1, TimeUnit.SECONDS))
      presenter.cancelVerifiedScan()
      eventually {
        presenter.snapshot.value.state.verifiedScan.operation is
            VerifiedScanOperation.CancellationUnconfirmed
      }
      assertEquals("running", presenter.snapshot.value.state.findings.scan?.status)
      assertNull(presenter.snapshot.value.state.jobs.error)
      releaseInitialPoll.countDown()
      releaseTerminalPoll.countDown()
      // The terminal report and cancellation settlement are published as separate events.
      eventually {
        val state = presenter.snapshot.value.state
        state.findings.scan?.status == "canceled" &&
            state.verifiedScan.operation == VerifiedScanOperation.Idle
      }

      assertEquals(2, polls.get())
      assertNull(presenter.snapshot.value.state.jobs.error)
      assertEquals(
          VerifiedScanOperation.Idle, presenter.snapshot.value.state.verifiedScan.operation)
    } finally {
      releaseInitialPoll.countDown()
      releaseTerminalPoll.countDown()
      presenter.close()
    }
  }

  @Test
  fun queuedVerifiedScanStartDoesNotAllowCancelBeforeAcceptance() {
    val dispatcher = Executors.newSingleThreadExecutor().asCoroutineDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    val blockerStarted = CountDownLatch(1)
    val releaseBlocker = CountDownLatch(1)
    val starts = AtomicInteger()
    val cancels = AtomicInteger()
    scope.launch {
      blockerStarted.countDown()
      releaseBlocker.await(2, TimeUnit.SECONDS)
    }
    val presenter =
        presenter(
            responder = { method, path, _ ->
              when (method to path) {
                "POST" to "/api/projects/current/scan" -> {
                  starts.incrementAndGet()
                  response(
                      "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"running\"}")
                }
                "DELETE" to "/api/projects/current/scan?project_revision=revision" -> {
                  cancels.incrementAndGet()
                  response(
                      "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"canceled\"}")
                }
                "GET" to "/api/projects/current/findings?project_revision=revision" ->
                    response("{}")
                else -> error("unexpected request $method $path")
              }
            },
            parentScope = scope)
    try {
      assertTrue(blockerStarted.await(1, TimeUnit.SECONDS))
      loadProject(presenter)
      presenter.dispatch(DesktopEvent.VerifiedScanReadUpdated(VerifiedScanRead.Absent))

      presenter.runVerifiedScan()
      presenter.cancelVerifiedScan()
      releaseBlocker.countDown()

      eventually { starts.get() == 1 }
      assertEquals(0, cancels.get())
      assertEquals("running", presenter.snapshot.value.state.findings.scan?.status)
    } finally {
      releaseBlocker.countDown()
      presenter.close()
      scope.cancel()
      dispatcher.close()
    }
  }

  @Test
  fun delayedTerminalScanFindingsCannotReplaceFindingsFromANewerScan() {
    val firstRefreshStarted = CountDownLatch(1)
    val releaseFirstRefresh = CountDownLatch(1)
    val starts = AtomicInteger()
    val findingsRefreshes = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "POST" to "/api/projects/current/scan" ->
            if (starts.incrementAndGet() == 1)
                response(
                    "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"canceled\"}")
            else
                response(
                    "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"running\"}")
        "GET" to "/api/projects/current/scan?project_revision=revision" ->
            response(
                "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"completed\"}")
        "GET" to "/api/projects/current/findings?project_revision=revision" ->
            if (findingsRefreshes.incrementAndGet() == 1) {
              firstRefreshStarted.countDown()
              releaseFirstRefresh.await(2, TimeUnit.SECONDS)
              response("{\"findings\":[{\"id\":\"old\"}]}")
            } else response("{\"findings\":[{\"id\":\"new\"}]}")
        else -> error("unexpected request $method $path")
      }
    }
    try {
      loadProject(presenter)
      presenter.dispatch(DesktopEvent.VerifiedScanReadUpdated(VerifiedScanRead.Absent))
      presenter.runVerifiedScan()
      assertTrue(firstRefreshStarted.await(1, TimeUnit.SECONDS))
      presenter.runVerifiedScan()
      eventually {
        presenter.snapshot.value.state.findings.scan?.status == "completed" &&
            presenter.snapshot.value.state.findings.findings.singleOrNull()?.id == "new"
      }
      releaseFirstRefresh.countDown()
      Thread.sleep(25)

      assertEquals("new", presenter.snapshot.value.state.findings.findings.singleOrNull()?.id)
    } finally {
      releaseFirstRefresh.countDown()
      presenter.close()
    }
  }

  @Test
  fun delayedWorkspaceFindingsCannotReplaceANewerTerminalScanRefresh() {
    val workspaceFindingsStarted = CountDownLatch(1)
    val releaseWorkspaceFindings = CountDownLatch(1)
    val findingsRequests = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "POST" to "/api/projects/import" -> response(projectJson())
        "GET" to "/api/projects/current/index" -> response(indexJson())
        "GET" to "/api/projects/current/overview?project_revision=revision" -> response("{}")
        "GET" to "/api/projects/current/findings?project_revision=revision" ->
            if (findingsRequests.incrementAndGet() == 1) {
              workspaceFindingsStarted.countDown()
              releaseWorkspaceFindings.await(2, TimeUnit.SECONDS)
              response("{\"findings\":[{\"id\":\"stale\"}]}")
            } else response("{\"findings\":[{\"id\":\"fresh\"}]}")
        "POST" to "/api/projects/current/scan" ->
            response(
                "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"canceled\"}")
        "GET" to "/api/projects/current/analysis-job?project_revision=revision",
        "GET" to "/api/projects/current/scan?project_revision=revision",
        "GET" to "/api/projects/current/performance-job?project_revision=revision",
        "GET" to "/api/projects/current/performance?project_revision=revision" ->
            TransportResponse(204, "")
        else -> error("unexpected request $method $path")
      }
    }
    try {
      presenter.loadProject("/tmp/project", restore = false)
      assertTrue(workspaceFindingsStarted.await(1, TimeUnit.SECONDS))
      presenter.dispatch(DesktopEvent.VerifiedScanReadUpdated(VerifiedScanRead.Absent))
      presenter.runVerifiedScan()
      eventually { presenter.snapshot.value.state.findings.findings.singleOrNull()?.id == "fresh" }
      releaseWorkspaceFindings.countDown()
      Thread.sleep(25)

      assertEquals("fresh", presenter.snapshot.value.state.findings.findings.singleOrNull()?.id)
    } finally {
      releaseWorkspaceFindings.countDown()
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
  fun creationInAPackageOnlyFileValidatesBeforeOpeningTheExistingChatPipeline() {
    val dispatcher = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    val requests = mutableListOf<String>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = dispatcher) { method, path, body ->
          creationFileResponse(path)
              ?: when (method to path) {
                "POST" to "/api/projects/current/chat/sessions" -> {
                  requests += path
                  assertTrue(body.orEmpty().contains("\"mode\":\"create_symbol\""))
                  assertTrue(body.orEmpty().contains("\"target_symbol\":\"新規\""))
                  assertTrue(body.orEmpty().contains("\"base_file_hash\":\"base\""))
                  response(creationSessionJson())
                }
                "POST" to "/api/projects/current/chat/sessions/session/messages" -> {
                  requests += path
                  response(creationProposalJson())
                }
                else -> error("unexpected request $method $path")
              }
        }
    try {
      loadProject(presenter)
      presenter.selectFile("main.go")
      dispatcher.runPending()
      assertTrue(presenter.snapshot.value.state.symbols.isEmpty())
      assertNull(presenter.snapshot.value.state.selectedSymbol)
      assertTrue(requests.isEmpty())
      listOf("func", "_", "init", "main", "bad.name").forEach { rejected ->
        presenter.sendChatMessage(ChatEditMode.CreateSymbol, rejected, "Return one.")
        dispatcher.runPending()
        assertTrue(requests.isEmpty(), rejected)
        if (rejected == "func")
            assertEquals(
                "func is a Go keyword. Choose a different name.",
                presenter.snapshot.value.state.error)
      }
      presenter.sendChatMessage(ChatEditMode.CreateSymbol, "新規", " ")
      dispatcher.runPending()
      assertTrue(requests.isEmpty())
      presenter.sendChatMessage(
          ChatEditMode.CreateSymbol,
          " 新規 ",
          "Return one.",
          creationKind = DeclarationCreationKind.Function)
      dispatcher.runPending()
      val state = presenter.snapshot.value.state
      assertEquals(2, requests.size)
      assertTrue(state.chat.attempts.last().requestText.contains("function named 新規"))
      assertEquals("create_symbol", state.chat.session?.mode)
      assertEquals("新規", state.review.draft?.targetSymbol)
      assertNull(state.review.draft?.validation)
      assertNull(state.review.checks)
      assertEquals("package main", state.selectedFile?.content)
      assertFalse(presenter.snapshot.value.generating)
    } finally {
      presenter.close()
      scope.cancel()
      dispatcher.runPending()
    }
  }

  @Test
  fun creationCannotSendWhileTheSelectedFileSnapshotIsPendingOrFailed() {
    val dispatcher = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    val posts = mutableListOf<String>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = dispatcher) { method, path, _ ->
          if (method == "POST") posts += path
          creationFileResponse(path) ?: error("unexpected request $method $path")
        }
    try {
      loadProject(presenter)
      presenter.selectFile("main.go")
      // No file/symbol response has been loaded yet.
      presenter.sendChatMessage(ChatEditMode.CreateSymbol, "NewRun", "Return one.")
      assertTrue(posts.isEmpty())
      dispatcher.runPending()
      assertEquals("main.go", presenter.snapshot.value.state.selectedFile?.path)
      presenter.dispatch(DesktopEvent.FileLoadFailed("Read failed", "main.go"))
      presenter.sendChatMessage(ChatEditMode.CreateSymbol, "NewRun", "Return one.")
      dispatcher.runPending()
      assertTrue(posts.isEmpty())
      assertTrue(presenter.snapshot.value.state.error.orEmpty().contains("load before creating"))
      assertNull(presenter.snapshot.value.state.review.draft)
      assertNull(presenter.snapshot.value.state.chat.session)
    } finally {
      presenter.close()
      scope.cancel()
      dispatcher.runPending()
    }
  }

  @Test
  fun reportedNonAtomicNameCollisionNeverOpensACreationSession() {
    val dispatcher = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    val posts = mutableListOf<String>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = dispatcher) { method, path, _ ->
          if (method == "POST") posts += path
          creationFileResponse(path) ?: error("unexpected request $method $path")
        }
    try {
      loadProject(presenter)
      presenter.dispatch(
          DesktopEvent.FileLoaded(
              file(),
              listOf(
                  SymbolInfo(
                      "Build", "function", confidence = "approximate", atomicTarget = false))))
      presenter.sendChatMessage(ChatEditMode.CreateSymbol, " Build ", "Return one.")
      dispatcher.runPending()
      assertTrue(posts.isEmpty())
      assertEquals(
          "Build already exists in this file; select it to replace instead.",
          presenter.snapshot.value.state.error)
      assertNull(presenter.snapshot.value.state.review.draft)
    } finally {
      presenter.close()
      scope.cancel()
      dispatcher.runPending()
    }
  }

  @Test
  fun lateCreationProposalCannotSurviveCancellationOrAFileProjectOrRevisionChange() {
    listOf("cancel", "file", "project", "revision").forEach { change ->
      val dispatcher = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + dispatcher)
      var replies = 0
      lateinit var activePresenter: DesktopWorkflowPresenter
      activePresenter =
          presenter(parentScope = scope, ioDispatcher = dispatcher) { method, path, _ ->
            creationFileResponse(path)
                ?: when (method to path) {
                  "POST" to "/api/projects/current/chat/sessions" -> response(creationSessionJson())
                  "POST" to "/api/projects/current/chat/sessions/session/messages" -> {
                    // Deliver a valid old response after the selection/cancellation boundary.
                    when (change) {
                      "cancel" -> activePresenter.cancelGeneration()
                      "file" ->
                          activePresenter.dispatch(
                              DesktopEvent.FileLoaded(
                                  file().copy(path = "other.go", contentHash = "other"),
                                  emptyList()))
                      "project" ->
                          activePresenter.dispatch(
                              DesktopEvent.ProjectLoaded(
                                  project("other"), ProjectIndex("other", "revision")))
                      "revision" ->
                          activePresenter.dispatch(
                              DesktopEvent.IndexRefreshed(ProjectIndex("project", "next")))
                    }
                    replies++
                    response(creationProposalJson())
                  }
                  else -> error("unexpected request $method $path")
                }
          }
      try {
        loadProject(activePresenter)
        activePresenter.selectFile("main.go")
        dispatcher.runPending()
        activePresenter.sendChatMessage(ChatEditMode.CreateSymbol, "新規", "Return one.")
        dispatcher.runPending()
        assertEquals(1, replies, change)
        assertNull(activePresenter.snapshot.value.state.review.draft, change)
        assertNull(activePresenter.snapshot.value.state.chat.session, change)
        assertFalse(activePresenter.snapshot.value.generating, change)
      } finally {
        activePresenter.close()
        scope.cancel()
        dispatcher.runPending()
      }
    }
  }

  @Test
  fun failedCreationKeepsTheFileAndReportsTheDaemonReason() {
    val dispatcher = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    val presenter =
        presenter(parentScope = scope, ioDispatcher = dispatcher) { method, path, _ ->
          creationFileResponse(path)
              ?: when (method to path) {
                "POST" to "/api/projects/current/chat/sessions" ->
                    TransportResponse(
                        409,
                        """{"type":"conflict","user_message":"The Go file changed. Reopen it before creating a function."}""")
                else -> error("unexpected request $method $path")
              }
        }
    try {
      loadProject(presenter)
      presenter.selectFile("main.go")
      dispatcher.runPending()
      presenter.sendChatMessage(ChatEditMode.CreateSymbol, "新規", "Return one.")
      dispatcher.runPending()
      assertTrue(presenter.snapshot.value.state.error.orEmpty().contains("The Go file changed."))
      assertEquals("package main", presenter.snapshot.value.state.selectedFile?.content)
      assertNull(presenter.snapshot.value.state.review.draft)
      assertFalse(presenter.snapshot.value.generating)
    } finally {
      presenter.close()
      scope.cancel()
      dispatcher.runPending()
    }
  }

  @Test
  fun presenterRejectsRawIntentBeforeConstraintsOrCreationWording() {
    val dispatcher = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    val calls = mutableListOf<String>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = dispatcher) { method, path, _ ->
          if (method == "POST") calls += "$method $path"
          chatFileResponse(path) ?: error("Unexpected request $method $path")
        }
    try {
      loadProject(presenter)
      presenter.selectFile("main.go")
      dispatcher.runPending()
      presenter.dispatch(
          DesktopEvent.SymbolSelected(
              SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true)))
      listOf("", "  \n", FunctionChangePreset.Fix.preparedMessage()).forEach { raw ->
        presenter.sendChatMessage(ChatEditMode.ReplaceSymbol, "", raw, "Keep the public signature.")
        dispatcher.runPending()
        assertTrue(calls.isEmpty())
        assertFalse(presenter.snapshot.value.generating)
      }
      presenter.sendChatMessage(
          ChatEditMode.CreateSymbol,
          "NewName",
          FunctionChangePreset.Document.preparedMessage(),
          "Keep the public signature.",
          creationKind = DeclarationCreationKind.Function)
      dispatcher.runPending()
      assertTrue(calls.isEmpty())
      assertNull(presenter.snapshot.value.state.chat.session)
    } finally {
      presenter.close()
      scope.cancel()
      dispatcher.runPending()
    }
  }

  @Test
  fun directSendCannotBypassDraftValidationAdmission() {
    val dispatcher = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    val calls = mutableListOf<String>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = dispatcher) { method, path, _ ->
          if (method == "POST") calls += path
          chatFileResponse(path) ?: error("Unexpected request $method $path")
        }
    try {
      loadProject(presenter)
      presenter.selectFile("main.go")
      dispatcher.runPending()
      presenter.dispatch(
          DesktopEvent.SymbolSelected(
              SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true)))
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      presenter.validateEditableDraft()
      assertEquals(
          DraftEditorStatus.Validating, presenter.snapshot.value.state.review.editor?.status)
      assertTrue(presenter.snapshot.value.draftValidationInProgress)

      presenter.sendChatMessage(ChatEditMode.ReplaceSymbol, "", "Preserve the signature")
      presenter.sendChatMessage(
          ChatEditMode.CreateSymbol,
          "NewName",
          "Return one.",
          creationKind = DeclarationCreationKind.Function)
      assertTrue(calls.isEmpty())
      assertEquals(0L, presenter.snapshot.value.state.chat.pendingRequestId)
      assertTrue(presenter.snapshot.value.state.chat.attempts.isEmpty())
      assertFalse(presenter.snapshot.value.generating)
      assertEquals(
          DraftEditorStatus.Validating, presenter.snapshot.value.state.review.editor?.status)
      assertEquals("draft", presenter.snapshot.value.state.review.draft?.id)
    } finally {
      presenter.close()
      scope.cancel()
      dispatcher.runPending()
    }
  }

  @Test
  fun presenterComposesConstraintsOnlyAfterAdmittingIntent() {
    val dispatcher = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    val calls = mutableListOf<Pair<String, String>>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = dispatcher) { method, path, body ->
          if (method == "POST") calls += path to body.orEmpty()
          chatFileResponse(path)
              ?: when (path) {
                "/api/projects/current/chat/sessions" ->
                    response(
                        """{"id":"session","project_id":"project","project_revision":"revision","base_file_hash":"base","open_path":"main.go","mode":"replace_symbol","target_symbol":"Run","state":"active","messages":[]}""")
                "/api/projects/current/chat/sessions/session/messages" ->
                    response(
                        """{"session_id":"session","draft":{"id":"draft","project_id":"project","project_revision":"revision","base_file_hash":"base","target_path":"main.go","mode":"replace_symbol","target_symbol":"Run","declaration":"func Run() {}","revision":1,"hash":"draft-hash","state":"generated"},"assistant_message":{"role":"assistant","content":"Ready"}}""")
                else -> error("Unexpected request $method $path")
              }
        }
    try {
      loadProject(presenter)
      presenter.selectFile("main.go")
      dispatcher.runPending()
      presenter.dispatch(
          DesktopEvent.SymbolSelected(
              SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true)))
      presenter.sendChatMessage(
          ChatEditMode.ReplaceSymbol, "", "  Preserve order.  ", "  Keep the public signature.  ")
      dispatcher.runPending()
      assertEquals(2, calls.size, presenter.snapshot.value.state.error.orEmpty())
      assertEquals(
          "Preserve order.\n\nConstraints:\nKeep the public signature.",
          Json.parseToJsonElement(calls[1].second).jsonObject["message"]?.jsonPrimitive?.content)
      assertEquals("draft", presenter.snapshot.value.state.review.draft?.id)
    } finally {
      presenter.close()
      scope.cancel()
      dispatcher.runPending()
    }
  }

  @Test
  fun presenterKeepsCreationWordingAndRepairFlagAfterRawIntentAdmission() {
    listOf(false, true).forEach { repair ->
      val dispatcher = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + dispatcher)
      val requests = mutableListOf<Pair<String, String>>()
      val presenter =
          presenter(parentScope = scope, ioDispatcher = dispatcher) { method, path, body ->
            if (method == "POST") requests += path to body.orEmpty()
            chatFileResponse(path)
                ?: when (path) {
                  "/api/projects/current/chat/sessions" -> response(creationSessionJson())
                  "/api/projects/current/chat/sessions/session/messages" ->
                      response(creationProposalJson())
                  else -> error("Unexpected request $method $path")
                }
          }
      try {
        loadProject(presenter)
        presenter.selectFile("main.go")
        dispatcher.runPending()
        presenter.sendChatMessage(
            ChatEditMode.CreateSymbol,
            "新規",
            "  Return one.  ",
            "  Keep exported names. ",
            repair = repair,
            creationKind = DeclarationCreationKind.Function)
        dispatcher.runPending()
        assertEquals(2, requests.size, presenter.snapshot.value.state.error.orEmpty())
        assertTrue(requests[0].second.contains("\"mode\":\"create_symbol\""))
        assertTrue(requests[0].second.contains("\"target_symbol\":\"新規\""))
        val body = Json.parseToJsonElement(requests[1].second).jsonObject
        assertEquals(
            "Create a Go function named 新規.\n\nReturn one.\n\nConstraints:\nKeep exported names.",
            body["message"]?.jsonPrimitive?.content)
        assertEquals(repair.toString(), body["repair"]?.jsonPrimitive?.content)
        assertEquals("created", presenter.snapshot.value.state.review.draft?.id)
      } finally {
        presenter.close()
        scope.cancel()
        dispatcher.runPending()
      }
    }
  }

  @Test
  fun explicitSendMakesOneChatRequestAndPreservesFunctionRemoteConsent() {
    val dispatcher = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    val sessionRequests = AtomicInteger()
    val messageRequests = AtomicInteger()
    var messageBody = ""
    val presenter =
        presenter(parentScope = scope, ioDispatcher = dispatcher) { method, path, body ->
          when (method to path) {
            "GET" to "/status" -> response("{\"status\":\"ok\",\"version\":\"v1\"}")
            "GET" to "/api/models/current" ->
                response(
                    """{"scopes":{"function":{"scope":"function","profile":"remote","model":"provider/editor","remote_provider":true}}}""")
            "GET" to "/api/projects/current/files/info?path=main.go" ->
                response(fileJson("main.go", "base"))
            "GET" to "/api/projects/current/files/symbols?path=main.go" ->
                response(symbolsJson("main.go", "Run"))
            "GET" to
                "/api/projects/current/files/analysis?path=main.go&project_revision=revision" ->
                response("{\"path\":\"main.go\",\"status\":\"missing\"}")
            "GET" to "/api/projects/current/impact?path=main.go" ->
                response("{\"target_path\":\"main.go\"}")
            "GET" to "/api/projects/current/git?path=main.go" -> response("{\"available\":false}")
            "POST" to "/api/projects/current/chat/sessions" -> {
              sessionRequests.incrementAndGet()
              response(
                  "{\"id\":\"session\",\"project_id\":\"project\",\"project_revision\":\"revision\",\"base_file_hash\":\"base\",\"open_path\":\"main.go\",\"mode\":\"replace_symbol\",\"target_symbol\":\"Run\",\"state\":\"active\",\"messages\":[]}")
            }
            "POST" to "/api/projects/current/chat/sessions/session/messages" -> {
              messageRequests.incrementAndGet()
              messageBody = body.orEmpty()
              response(
                  "{\"session_id\":\"session\",\"draft\":{\"id\":\"draft\",\"project_id\":\"project\",\"project_revision\":\"revision\",\"base_file_hash\":\"base\",\"target_path\":\"main.go\",\"mode\":\"replace_symbol\",\"target_symbol\":\"Run\",\"declaration\":\"func Run() {}\",\"revision\":1,\"hash\":\"draft-hash\",\"state\":\"generated\"},\"assistant_message\":{\"role\":\"assistant\",\"content\":\"Ready\"}}")
            }
            else -> error("unexpected request $method $path")
          }
        }
    try {
      presenter.refreshConnection()
      dispatcher.runPending()
      assertTrue(presenter.snapshot.value.model(ModelScope.Function).remoteProvider)
      loadProject(presenter)
      presenter.selectFile("main.go")
      // Complete file loading and enrichment before checking synchronous validation messages.
      dispatcher.runPending()
      assertEquals("main.go", presenter.snapshot.value.state.selectedFile?.path)
      assertEquals("missing", presenter.snapshot.value.state.analysis?.status)
      presenter.dispatch(
          DesktopEvent.SymbolSelected(
              SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true)))

      presenter.sendChatMessage(
          ChatEditMode.ReplaceSymbol, "", FunctionChangePreset.Fix.preparedMessage())
      dispatcher.runPending()
      assertEquals(0, sessionRequests.get())
      assertEquals(0, messageRequests.get())
      assertEquals("Enter a specific intent before sending.", presenter.snapshot.value.state.error)

      presenter.sendChatMessage(
          ChatEditMode.ReplaceSymbol, "", "Fix a bug: return a typed error for a missing user")
      dispatcher.runPending()
      assertEquals(0, sessionRequests.get())
      assertEquals(0, messageRequests.get())
      assertEquals(
          "Confirm the Function remote destination before sending.",
          presenter.snapshot.value.state.error)

      presenter.setProviderConfirmation(ModelScope.Function, true)
      presenter.sendChatMessage(
          ChatEditMode.ReplaceSymbol, "", "Fix a bug: return a typed error for a missing user")
      dispatcher.runPending()
      assertEquals("draft", presenter.snapshot.value.state.review.draft?.id)

      assertEquals(1, sessionRequests.get())
      assertEquals(1, messageRequests.get())
      assertTrue(messageBody.contains("\"confirm_remote_provider\":true"))
    } finally {
      presenter.close()
      scope.cancel()
      dispatcher.runPending()
    }
  }

  @Test
  fun cancelBeforeLaunchIsSynchronousAndOldJobCannotClearNewRunningRequest() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    var opens = 0
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
          chatFileResponse(path)
              ?: when (path) {
                "/api/projects/current/chat/sessions" -> {
                  opens++
                  response(
                      """{"id":"session","project_id":"project","project_revision":"revision","base_file_hash":"base","open_path":"main.go","mode":"replace_symbol","target_symbol":"Run","state":"active","messages":[]}""")
                }
                else -> error("Unexpected request $path")
              }
        }
    try {
      loadQueuedChatFile(presenter, main, io)
      presenter.sendChatMessage(ChatEditMode.ReplaceSymbol, "", "First request")
      val first = presenter.snapshot.value.state.chat.pendingRequestId
      presenter.cancelGeneration()
      assertEquals(
          ChatRequestOutcome.Canceled,
          presenter.snapshot.value.state.chat.attempts.single().outcome)
      assertEquals(0L, presenter.snapshot.value.state.chat.pendingRequestId)
      presenter.sendChatMessage(ChatEditMode.ReplaceSymbol, "", "Second request")
      val second = presenter.snapshot.value.state.chat.pendingRequestId
      assertTrue(second != first)
      main.runPending()
      assertEquals(second, presenter.snapshot.value.state.chat.pendingRequestId)
      assertTrue(presenter.snapshot.value.generating)
      assertEquals(0, opens)
    } finally {
      presenter.close()
      scope.cancel()
      main.runPending()
      io.runPending()
    }
  }

  @Test
  fun lateTransportSuccessAndFailureCannotReplaceChangedTargetDraftOrClosedPresenter() {
    for (failure in listOf(false, true)) for (change in
        listOf("target", "draft", "replacement", "discard", "close", "cancel")) {
      val main = QueuedDispatcher()
      val io = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + main)
      lateinit var presenter: DesktopWorkflowPresenter
      var messages = 0
      presenter =
          presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
            chatFileResponse(path)
                ?: when (path) {
                  "/api/projects/current/chat/sessions" ->
                      response(
                          """{"id":"session","project_id":"project","project_revision":"revision","base_file_hash":"base","open_path":"main.go","mode":"replace_symbol","target_symbol":"Run","state":"active","messages":[]}""")
                  "/api/projects/current/chat/sessions/session/messages" -> {
                    messages++
                    when (change) {
                      "target" ->
                          presenter.dispatch(
                              DesktopEvent.SymbolSelected(
                                  SymbolInfo(
                                      "Other",
                                      "function",
                                      confidence = "exact",
                                      atomicTarget = true)))
                      "draft" ->
                          presenter.dispatch(
                              DesktopEvent.DraftEdited(declaration = "func Run() { println(1) }"))
                      "replacement" ->
                          presenter.dispatch(
                              DesktopEvent.DraftLoaded(draft().copy(id = "newer", revision = 2)))
                      "discard" -> presenter.discardDraft()
                      "close" -> presenter.close()
                      "cancel" -> presenter.cancelGeneration()
                    }
                    if (failure)
                        TransportResponse(503, """{"user_message":"Late provider error"}""")
                    else
                        response(
                            """{"session_id":"session","draft":{"id":"new-draft","project_id":"project","project_revision":"revision","base_file_hash":"base","target_path":"main.go","mode":"replace_symbol","target_symbol":"Run","declaration":"func Run() {}","revision":1,"hash":"new-hash","state":"generated"},"assistant_message":{"role":"assistant","content":"Late"}}""")
                  }
                  else -> error("Unexpected request $path")
                }
          }
      try {
        loadQueuedChatFile(presenter, main, io)
        presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
        presenter.sendChatMessage(ChatEditMode.ReplaceSymbol, "", "Improve Run")
        main.runPending()
        io.runPending()
        main.runPending()
        io.runPending()
        main.runPending()
        assertEquals(1, messages, "$failure/$change")
        val state = presenter.snapshot.value.state
        assertEquals(0L, state.chat.pendingRequestId, "$failure/$change")
        assertNull(state.chat.session, "$failure/$change")
        assertTrue(
            state.chat.attempts.none {
              it.outcome is ChatRequestOutcome.Failed || it.outcome is ChatRequestOutcome.Succeeded
            },
            "$failure/$change")
        assertTrue(state.review.draft?.id != "new-draft", "$failure/$change")
        if (change == "replacement") assertEquals("newer", state.review.draft?.id)
        assertFalse(presenter.snapshot.value.generating, "$failure/$change")
      } finally {
        presenter.close()
        scope.cancel()
        main.runPending()
        io.runPending()
      }
    }
  }

  @Test
  fun lateTransportResultAfterFunctionConsentRevocationCannotPublish() {
    for (failure in listOf(false, true)) {
      val main = QueuedDispatcher()
      val io = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + main)
      lateinit var presenter: DesktopWorkflowPresenter
      presenter =
          presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
            chatFileResponse(path)
                ?: when (path) {
                  "/status" -> response("""{"status":"ok","version":"v1"}""")
                  "/api/models/current" -> response(functionCatalog("provider/editor"))
                  "/api/projects/current/chat/sessions" ->
                      response(
                          """{"id":"session","project_id":"project","project_revision":"revision","base_file_hash":"base","open_path":"main.go","mode":"replace_symbol","target_symbol":"Run","state":"active","messages":[]}""")
                  "/api/projects/current/chat/sessions/session/messages" -> {
                    presenter.setProviderConfirmation(ModelScope.Function, false)
                    if (failure) TransportResponse(503, """{"user_message":"Late failure"}""")
                    else
                        response(
                            """{"session_id":"session","draft":{"id":"new-draft","project_id":"project","project_revision":"revision","base_file_hash":"base","target_path":"main.go","mode":"replace_symbol","target_symbol":"Run","declaration":"func Run() {}","revision":1,"hash":"new-hash","state":"generated"},"assistant_message":{"role":"assistant","content":"Late"}}""")
                  }
                  else -> error("Unexpected request $path")
                }
          }
      try {
        loadQueuedChatFile(presenter, main, io)
        presenter.refreshConnection()
        main.runPending()
        io.runPending()
        main.runPending()
        presenter.setProviderConfirmation(ModelScope.Function, true)
        presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
        presenter.sendChatMessage(ChatEditMode.ReplaceSymbol, "", "Improve Run")
        main.runPending()
        io.runPending()
        main.runPending()
        io.runPending()
        main.runPending()
        val state = presenter.snapshot.value.state
        assertEquals(ChatRequestOutcome.Canceled, state.chat.attempts.last().outcome)
        assertNull(state.chat.session)
        assertEquals("draft", state.review.draft?.id)
        assertFalse(presenter.snapshot.value.generating)
      } finally {
        presenter.close()
        scope.cancel()
        main.runPending()
        io.runPending()
      }
    }
  }

  @Test
  fun transportTimeoutIsFailureNotConfirmedCancellation() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
          chatFileResponse(path)
              ?: when (path) {
                "/api/projects/current/chat/sessions" ->
                    response(
                        """{"id":"session","project_id":"project","project_revision":"revision","base_file_hash":"base","open_path":"main.go","mode":"replace_symbol","target_symbol":"Run","state":"active","messages":[]}""")
                "/api/projects/current/chat/sessions/session/messages" ->
                    throw HttpTimeoutException("timed out")
                else -> error("Unexpected request $path")
              }
        }
    try {
      loadQueuedChatFile(presenter, main, io)
      presenter.sendChatMessage(ChatEditMode.ReplaceSymbol, "", "Improve Run")
      main.runPending()
      io.runPending()
      main.runPending()
      io.runPending()
      main.runPending()
      assertTrue(
          presenter.snapshot.value.state.chat.attempts.single().outcome
              is ChatRequestOutcome.Failed)
      assertEquals(0L, presenter.snapshot.value.state.chat.pendingRequestId)
      assertFalse(presenter.snapshot.value.generating)
    } finally {
      presenter.close()
      scope.cancel()
      main.runPending()
      io.runPending()
    }
  }

  @Test
  fun duplicateSendWhileAttemptRunsOpensOnlyOneSession() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    var opens = 0
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
          chatFileResponse(path)
              ?: when (path) {
                "/api/projects/current/chat/sessions" -> {
                  opens++
                  response(
                      """{"id":"session","project_id":"project","project_revision":"revision","base_file_hash":"base","open_path":"main.go","mode":"replace_symbol","target_symbol":"Run","state":"active","messages":[]}""")
                }
                "/api/projects/current/chat/sessions/session/messages" ->
                    TransportResponse(503, """{"user_message":"Provider unavailable"}""")
                else -> error("Unexpected request $path")
              }
        }
    try {
      loadQueuedChatFile(presenter, main, io)
      presenter.sendChatMessage(ChatEditMode.ReplaceSymbol, "", "Improve Run")
      val admitted = presenter.snapshot.value.state.chat.attempts.single()
      presenter.sendChatMessage(ChatEditMode.ReplaceSymbol, "", "Improve Run again")
      assertEquals(listOf(admitted), presenter.snapshot.value.state.chat.attempts)
      main.runPending()
      io.runPending()
      main.runPending()
      io.runPending()
      main.runPending()
      assertEquals(1, opens)
      val attempts = presenter.snapshot.value.state.chat.attempts
      assertEquals(1, attempts.size)
      assertEquals(ChatRequestOutcome.Failed("Provider unavailable"), attempts.single().outcome)
      assertEquals("Improve Run", attempts.single().requestText)
    } finally {
      presenter.close()
      scope.cancel()
      main.runPending()
      io.runPending()
    }
  }

  @Test
  fun sessionOpenFailureBelongsToAdmittedAttemptAndPreservesPriorEvidence() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    var opens = 0
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
          chatFileResponse(path)
              ?: if (path == "/api/projects/current/chat/sessions") {
                opens++
                TransportResponse(
                    409, """{"type":"conflict","user_message":"Reopen the changed file."}""")
              } else error("Unexpected request $path")
        }
    try {
      loadQueuedChatFile(presenter, main, io)
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      presenter.sendChatMessage(ChatEditMode.ReplaceSymbol, "", "Preserve the signature")
      main.runPending()
      io.runPending()
      main.runPending()
      val state = presenter.snapshot.value.state
      assertEquals(1, opens)
      assertEquals("Preserve the signature", state.chat.attempts.single().requestText)
      assertTrue(
          (state.chat.attempts.single().outcome as ChatRequestOutcome.Failed)
              .message
              .contains("Reopen the changed file."))
      assertEquals("draft", state.review.draft?.id)
      assertEquals("Run", state.selectedSymbol?.name)
      assertFalse(presenter.snapshot.value.generating)
    } finally {
      presenter.close()
      scope.cancel()
      main.runPending()
      io.runPending()
    }
  }

  @Test
  fun messageTransportFailureRetainsDraftAndDoesNotExposeTransportDetails() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    var messages = 0
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
          chatFileResponse(path)
              ?: when (path) {
                "/api/projects/current/chat/sessions" ->
                    response(
                        """{"id":"session","project_id":"project","project_revision":"revision","base_file_hash":"base","open_path":"main.go","mode":"replace_symbol","target_symbol":"Run","state":"active","messages":[]}""")
                "/api/projects/current/chat/sessions/session/messages" -> {
                  messages++
                  error("private transport details: token=secret")
                }
                else -> error("Unexpected request $path")
              }
        }
    try {
      loadQueuedChatFile(presenter, main, io)
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      presenter.sendChatMessage(ChatEditMode.ReplaceSymbol, "", "Improve Run")
      main.runPending()
      io.runPending()
      main.runPending()
      io.runPending()
      main.runPending()
      val state = presenter.snapshot.value.state
      assertEquals(1, messages)
      assertEquals("Improve Run", state.chat.attempts.single().requestText)
      assertEquals(
          ChatRequestOutcome.Failed("Chat request failed. Send again to retry."),
          state.chat.attempts.single().outcome)
      assertEquals("draft", state.review.draft?.id)
      assertFalse(presenter.snapshot.value.generating)
    } finally {
      presenter.close()
      scope.cancel()
      main.runPending()
      io.runPending()
    }
  }

  @Test
  fun openedSessionForAnotherTargetDoesNotReceiveTheAdmittedMessage() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    var messages = 0
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
          chatFileResponse(path)
              ?: when (path) {
                "/api/projects/current/chat/sessions" ->
                    response(
                        """{"id":"wrong","project_id":"project","project_revision":"revision","base_file_hash":"base","open_path":"other.go","mode":"replace_symbol","target_symbol":"Run","state":"active","messages":[]}""")
                "/api/projects/current/chat/sessions/wrong/messages" -> {
                  messages++
                  error("Must not send to a mismatched session")
                }
                else -> error("Unexpected request $path")
              }
        }
    try {
      loadQueuedChatFile(presenter, main, io)
      presenter.sendChatMessage(ChatEditMode.ReplaceSymbol, "", "Improve Run")
      main.runPending()
      io.runPending()
      main.runPending()
      assertEquals(0, messages)
      assertTrue(
          presenter.snapshot.value.state.chat.attempts.single().outcome
              is ChatRequestOutcome.Failed)
      assertEquals("Improve Run", presenter.snapshot.value.state.chat.attempts.single().requestText)
    } finally {
      presenter.close()
      scope.cancel()
      main.runPending()
      io.runPending()
    }
  }

  @Test
  fun revokedFunctionConsentDuringSessionOpenCannotDispatchWithOldConfirmation() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    var messages = 0
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
          chatFileResponse(path)
              ?: when (path) {
                "/status" -> response("""{"status":"ok","version":"v1"}""")
                "/api/models/current" -> response(functionCatalog("provider/editor"))
                "/api/projects/current/chat/sessions" ->
                    response(
                        """{"id":"session","project_id":"project","project_revision":"revision","base_file_hash":"base","open_path":"main.go","mode":"replace_symbol","target_symbol":"Run","state":"active","messages":[]}""")
                "/api/projects/current/chat/sessions/session/messages" -> {
                  messages++
                  error("Must not dispatch without Function authorization")
                }
                else -> error("Unexpected request $path")
              }
        }
    try {
      loadQueuedChatFile(presenter, main, io)
      presenter.refreshConnection()
      main.runPending()
      io.runPending()
      main.runPending()
      assertTrue(presenter.snapshot.value.model(ModelScope.Function).remoteProvider)
      presenter.setProviderConfirmation(ModelScope.Analyze, true)
      presenter.setProviderConfirmation(ModelScope.Bug, true)
      presenter.sendChatMessage(ChatEditMode.ReplaceSymbol, "", "Improve Run")
      assertTrue(presenter.snapshot.value.state.chat.attempts.isEmpty())
      presenter.setProviderConfirmation(ModelScope.Function, true)
      presenter.sendChatMessage(ChatEditMode.ReplaceSymbol, "", "Improve Run")
      assertTrue(presenter.snapshot.value.state.chat.attempts.single().remoteConfirmed)
      assertEquals(
          "provider/editor",
          presenter.snapshot.value.state.chat.attempts.single().destination.model)
      main.runPending()
      io.runPending() // Session creation finishes; the message has not started.
      presenter.setProviderConfirmation(ModelScope.Function, false)
      main.runPending()
      io.runPending()
      assertEquals(0, messages)
      assertEquals(
          ChatRequestOutcome.Canceled,
          presenter.snapshot.value.state.chat.attempts.single().outcome)
      assertFalse(presenter.snapshot.value.generating)
    } finally {
      presenter.close()
      scope.cancel()
      main.runPending()
      io.runPending()
    }
  }

  @Test
  fun revokedFunctionConsentBeforeQueuedMessageTransportDoesNotDispatch() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    var messages = 0
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
          chatFileResponse(path)
              ?: when (path) {
                "/status" -> response("""{"status":"ok","version":"v1"}""")
                "/api/models/current" -> response(functionCatalog("provider/editor"))
                "/api/projects/current/chat/sessions" ->
                    response(
                        """{"id":"session","project_id":"project","project_revision":"revision","base_file_hash":"base","open_path":"main.go","mode":"replace_symbol","target_symbol":"Run","state":"active","messages":[]}""")
                "/api/projects/current/chat/sessions/session/messages" -> {
                  messages++
                  error("Revoked consent must not reach message transport")
                }
                else -> error("Unexpected request $path")
              }
        }
    try {
      loadQueuedChatFile(presenter, main, io)
      presenter.refreshConnection()
      main.runPending()
      io.runPending()
      main.runPending()
      presenter.setProviderConfirmation(ModelScope.Function, true)
      presenter.sendChatMessage(ChatEditMode.ReplaceSymbol, "", "Improve Run")
      main.runPending()
      io.runPending() // Session open completes; the continuation is on main.
      main.runPending() // Authorization is checked on main; transport is queued on I/O.
      presenter.setProviderConfirmation(ModelScope.Function, false)
      io.runPending()
      main.runPending()
      assertEquals(0, messages)
      assertEquals(
          ChatRequestOutcome.Canceled,
          presenter.snapshot.value.state.chat.attempts.single().outcome)
      assertEquals(0L, presenter.snapshot.value.state.chat.pendingRequestId)
      assertFalse(presenter.snapshot.value.generating)
    } finally {
      presenter.close()
      scope.cancel()
      main.runPending()
      io.runPending()
    }
  }

  @Test
  fun changedFunctionDestinationDuringSessionOpenCannotDispatch() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    var model = "provider/first"
    var messages = 0
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
          chatFileResponse(path)
              ?: when (path) {
                "/status" -> response("""{"status":"ok","version":"v1"}""")
                "/api/models/current" -> response(functionCatalog(model))
                "/api/projects/current/chat/sessions" ->
                    response(
                        """{"id":"session","project_id":"project","project_revision":"revision","base_file_hash":"base","open_path":"main.go","mode":"replace_symbol","target_symbol":"Run","state":"active","messages":[]}""")
                "/api/projects/current/chat/sessions/session/messages" -> {
                  messages++
                  error("Message must not reach a changed destination")
                }
                else -> error("Unexpected request $path")
              }
        }
    try {
      loadQueuedChatFile(presenter, main, io)
      presenter.refreshConnection()
      main.runPending()
      io.runPending()
      main.runPending()
      presenter.setProviderConfirmation(ModelScope.Function, true)
      presenter.sendChatMessage(ChatEditMode.ReplaceSymbol, "", "Improve Run")
      main.runPending()
      io.runPending() // Open completed, but Send is still suspended.
      model = "provider/second"
      presenter.refreshConnection()
      main.runLast() // Start catalog refresh before resuming Send.
      io.runPending()
      main.runLast() // Publish the new catalog and invalidate consent.
      main.runPending()
      io.runPending()
      assertEquals(0, messages)
      assertEquals(
          "provider/first", presenter.snapshot.value.state.chat.attempts.single().destination.model)
      assertEquals(
          ChatRequestOutcome.Canceled,
          presenter.snapshot.value.state.chat.attempts.single().outcome)
      assertFalse(presenter.snapshot.value.providerConfirmed(ModelScope.Function))
    } finally {
      presenter.close()
      scope.cancel()
      main.runPending()
      io.runPending()
    }
  }

  @Test
  fun newLocalChangeDoesNotAttachAnOldPreparedBugTask() {
    val sessionBodies = Collections.synchronizedList(mutableListOf<String>())
    val messageRequests = AtomicInteger()
    val presenter = presenter { method, path, body ->
      when (method to path) {
        "GET" to "/api/projects/current/files/info?path=main.go" ->
            response(fileJson("main.go", "base"))
        "GET" to "/api/projects/current/files/symbols?path=main.go" ->
            response(symbolsJson("main.go", "Run"))
        "GET" to "/api/projects/current/files/analysis?path=main.go&refresh=false" ->
            response("{\"path\":\"main.go\",\"status\":\"missing\"}")
        "GET" to "/api/projects/current/impact?path=main.go" ->
            response("{\"target_path\":\"main.go\"}")
        "GET" to "/api/projects/current/git?path=main.go" -> response("{\"available\":false}")
        "POST" to "/api/projects/current/chat/sessions" -> {
          sessionBodies += body.orEmpty()
          val task =
              if (body.orEmpty().contains("\"task_spec\""))
                  ",\"task_spec\":{\"schema_version\":\"1\",\"target_path\":\"main.go\",\"target_symbol\":\"Run\",\"target_signature\":\"func Run()\",\"acceptance_criteria\":[\"Return an error for empty input.\"]}"
              else ""
          response(
              "{\"id\":\"session\",\"project_id\":\"project\",\"project_revision\":\"revision\",\"base_file_hash\":\"base\",\"open_path\":\"main.go\",\"mode\":\"replace_symbol\",\"target_symbol\":\"Run\",\"state\":\"active\",\"messages\":[]$task}")
        }
        "POST" to "/api/projects/current/chat/sessions/session/messages" -> {
          messageRequests.incrementAndGet()
          error("stop after observing the request")
        }
        else -> error("unexpected request $method $path")
      }
    }
    try {
      loadProject(presenter)
      presenter.selectFile("main.go")
      eventually { presenter.snapshot.value.state.selectedFile?.path == "main.go" }
      val symbol = SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true)
      presenter.dispatch(DesktopEvent.SymbolSelected(symbol))
      val task =
          BugTaskSpec(
              "1", "main.go", "Run", "func Run()", listOf("Return an error for empty input."))

      presenter.dispatch(
          DesktopEvent.SuggestionPrepared(
              "fix", "Fix a bug: return an error for empty input", symbol, task))
      presenter.sendChatMessage(
          ChatEditMode.ReplaceSymbol, "", "Fix a bug: return an error for empty input")
      eventually { messageRequests.get() == 1 && !presenter.snapshot.value.generating }
      assertTrue(sessionBodies.single().contains("\"task_spec\""))

      presenter.dispatch(
          DesktopEvent.SuggestionPrepared(
              "fix", "Fix a bug: return an error for empty input", symbol, task))
      presenter.clearPreparedSuggestion()
      presenter.sendChatMessage(
          ChatEditMode.ReplaceSymbol, "", "Change behavior: preserve insertion order")
      eventually { messageRequests.get() == 2 && !presenter.snapshot.value.generating }

      assertEquals(2, sessionBodies.size)
      assertFalse(sessionBodies.last().contains("\"task_spec\""))
    } finally {
      presenter.close()
    }
  }

  @Test
  fun benchmarkOutcomesNeitherAuthorizeApplyNorBlockAnOtherwiseCheckedCandidate() {
    for (outcome in listOf("absent", "completed", "inconclusive", "failed", "stale")) {
      val main = QueuedDispatcher()
      val io = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + main)
      val calls = mutableListOf<Pair<String, String>>()
      val presenter =
          presenter(parentScope = scope, ioDispatcher = io) { method, path, body ->
            calls += method to path
            if (path.endsWith("/apply")) {
              assertTrue(body.orEmpty().contains("\"draft_id\":\"draft\""))
              response(
                  """{"project_revision":"next","post_apply_hash":"after","undo_available":true}""")
            } else TransportResponse(503, """{"message":"Source reload unavailable"}""")
          }
      try {
        loadFile(presenter)
        presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
        if (outcome != "absent") {
          var comparison = Json.decodeFromString<GoBenchmarkComparison>(benchmarkComparisonJson())
          comparison =
              when (outcome) {
                "inconclusive" -> comparison.copy(candidate = GoBenchmarkMeasurement())
                "failed" ->
                    comparison.copy(
                        status = "failed",
                        base = null,
                        candidate = null,
                        reason = "Execution failed")
                "stale" -> comparison.copy(draftHash = "older-candidate")
                else -> comparison
              }
          presenter.dispatch(DesktopEvent.GoBenchmarkComparisonLoaded(comparison))
        }
        presenter.applyEditableDraft()
        repeat(3) {
          main.runPending()
          io.runPending()
        }
        assertTrue(calls.isEmpty(), "$outcome cannot bypass missing checks: $calls")
        presenter.dispatch(
            DesktopEvent.ChecksLoaded(
                DraftCheckReport(
                    "main.go",
                    true,
                    draftId = "draft",
                    draftRevision = 1,
                    draftHash = "draft-hash")))
        presenter.applyEditableDraft()
        repeat(4) {
          main.runPending()
          io.runPending()
        }
        assertEquals(1, calls.count { it == "POST" to "/api/projects/current/apply" }, outcome)
        assertEquals("after", presenter.snapshot.value.state.review.applied?.postApplyHash, outcome)
        assertTrue(
            calls.none {
              it.second.contains("benchmarks") || it.second.contains("execution-trust")
            },
            outcome)
      } finally {
        presenter.close()
        scope.cancel()
      }
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

  @Test
  fun startFailureAfterPostIsUncertainAndRetainsEarlierReport() {
    val requests = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "POST" to "/api/projects/current/scan" -> {
          requests.incrementAndGet()
          TransportResponse(504, "request timed out")
        }
        else -> error("Unexpected $method $path")
      }
    }
    try {
      loadProject(presenter)
      val prior = GoScanReport("project", "revision", status = "completed")
      presenter.dispatch(DesktopEvent.GoScanLoaded(prior))
      presenter.runVerifiedScan()
      assertEquals(
          VerifiedScanOperation.Starting, presenter.snapshot.value.state.verifiedScan.operation)
      assertEquals(prior, presenter.snapshot.value.state.findings.scan)
      presenter.runVerifiedScan()
      eventually {
        presenter.snapshot.value.state.verifiedScan.operation is
            VerifiedScanOperation.StartUncertain
      }
      assertEquals(prior, presenter.snapshot.value.state.findings.scan)
      assertTrue(
          (presenter.snapshot.value.state.verifiedScan.operation
                  as VerifiedScanOperation.StartUncertain)
              .message
              .contains("refresh scan status"))
      presenter.runVerifiedScan()
      assertEquals(1, requests.get())
      assertNull(presenter.snapshot.value.state.jobs.error)
    } finally {
      presenter.close()
    }
  }

  @Test
  fun trustFailureBeforeStartIsLocalAndDoesNotClaimRequestWasSent() {
    val starts = AtomicInteger()
    val presenter =
        presenter(interceptTrust = false) { method, path, _ ->
          when (method to path) {
            "GET" to "/api/projects/current/execution-trust?project_revision=revision" ->
                TransportResponse(500, "trust unavailable")
            "POST" to "/api/projects/current/scan" -> {
              starts.incrementAndGet()
              error("Start must not be sent")
            }
            else -> error("Unexpected $method $path")
          }
        }
    try {
      loadProject(presenter)
      presenter.dispatch(DesktopEvent.VerifiedScanReadUpdated(VerifiedScanRead.Absent))
      presenter.runVerifiedScan()
      eventually {
        presenter.snapshot.value.state.verifiedScan.operation is VerifiedScanOperation.Failed
      }
      assertEquals(0, starts.get())
      assertNull(presenter.snapshot.value.state.jobs.error)
    } finally {
      presenter.close()
    }
  }

  @Test
  fun cancelTimeoutLeavesCancellationUnconfirmedAndRetainsDiagnostics() {
    val pollStarted = CountDownLatch(1)
    val releasePoll = CountDownLatch(1)
    val cancels = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "DELETE" to "/api/projects/current/scan?project_revision=revision" -> {
          cancels.incrementAndGet()
          TransportResponse(504, "cancel timed out")
        }
        "GET" to "/api/projects/current/scan?project_revision=revision" -> {
          pollStarted.countDown()
          releasePoll.await(2, TimeUnit.SECONDS)
          response("""{"project_id":"project","project_revision":"revision","status":"running"}""")
        }
        else -> error("Unexpected $method $path")
      }
    }
    try {
      loadProject(presenter)
      val running = GoScanReport("project", "revision", status = "running")
      presenter.dispatch(DesktopEvent.GoScanLoaded(running))
      presenter.cancelVerifiedScan()
      assertTrue(pollStarted.await(1, TimeUnit.SECONDS))
      eventually {
        presenter.snapshot.value.state.verifiedScan.operation is
            VerifiedScanOperation.CancellationUnconfirmed
      }
      presenter.cancelVerifiedScan()
      assertEquals(1, cancels.get())
      assertEquals(running, presenter.snapshot.value.state.findings.scan)
      assertNull(presenter.snapshot.value.state.jobs.error)
    } finally {
      releasePoll.countDown()
      presenter.close()
    }
  }

  @Test
  fun cancelRunningResponseStaysPendingUntilTerminalPoll() {
    val pollStarted = CountDownLatch(1)
    val releasePoll = CountDownLatch(1)
    val cancels = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "DELETE" to "/api/projects/current/scan?project_revision=revision" -> {
          cancels.incrementAndGet()
          response("""{"project_id":"project","project_revision":"revision","status":"running"}""")
        }
        "GET" to "/api/projects/current/scan?project_revision=revision" -> {
          pollStarted.countDown()
          releasePoll.await(2, TimeUnit.SECONDS)
          response("""{"project_id":"project","project_revision":"revision","status":"canceled"}""")
        }
        "GET" to "/api/projects/current/findings?project_revision=revision" -> response("{}")
        else -> error("Unexpected $method $path")
      }
    }
    try {
      loadProject(presenter)
      val running = GoScanReport("project", "revision", status = "running")
      presenter.dispatch(DesktopEvent.GoScanLoaded(running))
      presenter.cancelVerifiedScan()
      assertEquals(
          VerifiedScanOperation.CancellationRequested,
          presenter.snapshot.value.state.verifiedScan.operation)
      assertEquals(running, presenter.snapshot.value.state.findings.scan)
      presenter.cancelVerifiedScan()
      assertTrue(pollStarted.await(1, TimeUnit.SECONDS))
      assertEquals(
          VerifiedScanOperation.CancellationRequested,
          presenter.snapshot.value.state.verifiedScan.operation)
      assertEquals("running", presenter.snapshot.value.state.findings.scan?.status)
      releasePoll.countDown()
      eventually {
        val state = presenter.snapshot.value.state
        state.findings.scan?.status == "canceled" &&
            state.verifiedScan.operation == VerifiedScanOperation.Idle
      }
      assertEquals(
          VerifiedScanOperation.Idle, presenter.snapshot.value.state.verifiedScan.operation)
      assertEquals(1, cancels.get())
    } finally {
      releasePoll.countDown()
      presenter.close()
    }
  }

  @Test
  fun verifiedScanTrustsBeforeStartingAndNavigationSendsNoTrustRequest() {
    val requests = mutableListOf<String>()
    val presenter =
        presenter(interceptTrust = false) { method, path, body ->
          requests += "$method $path"
          if (method == "POST" && path.endsWith("/execution-trust"))
              assertTrue(body.orEmpty().contains("\"confirm\":true"))
          when (method to path) {
            "GET" to "/api/projects/current/execution-trust?project_revision=revision" ->
                response(
                    """{"project_id":"project","project_revision":"revision","trusted":false,"commands":[["go","test","./..."]]}""")
            "POST" to "/api/projects/current/execution-trust" ->
                response(
                    """{"project_id":"project","project_revision":"revision","trusted":true,"commands":[["go","test","./..."]]}""")
            "POST" to "/api/projects/current/scan" ->
                response(
                    """{"project_id":"project","project_revision":"revision","status":"canceled"}""")
            "GET" to "/api/projects/current/findings?project_revision=revision" -> response("{}")
            else -> response("{}")
          }
        }
    try {
      loadProject(presenter)
      presenter.dispatch(DesktopEvent.WorkspaceSelected(Workspace.Bugs))
      assertTrue(requests.isEmpty())
      presenter.dispatch(DesktopEvent.VerifiedScanReadUpdated(VerifiedScanRead.Absent))
      presenter.runVerifiedScan()
      presenter.runVerifiedScan()
      eventually { requests.any { it == "POST /api/projects/current/scan" } }
      assertEquals(
          listOf(
              "GET /api/projects/current/execution-trust?project_revision=revision",
              "POST /api/projects/current/execution-trust",
              "POST /api/projects/current/scan"),
          requests.take(3))
      assertEquals(1, requests.count { it == "POST /api/projects/current/scan" })
    } finally {
      presenter.close()
    }
  }

  @Test
  fun verifiedScanRejectsInvalidTrustAtEachStageWithoutFurtherMutation() {
    val invalid =
        listOf(
            """{"project_id":"other","project_revision":"revision","trusted":true,"commands":[["go","test","./..."]]}""",
            """{"project_id":"","project_revision":"revision","trusted":true,"commands":[["go","test","./..."]]}""",
            """{"project_id":"project","project_revision":"","trusted":true,"commands":[["go","test","./..."]]}""",
            """{"project_id":"project","project_revision":"changed","trusted":true,"commands":[["go","test","./..."]]}""",
            """{"project_id":"project","project_revision":"revision","trusted":true,"commands":[["go","vet","./..."]]}""",
            """{"project_id":"project","project_revision":"revision","trusted":true,"commands":[]}""",
        )
    val valid =
        """{"project_id":"project","project_revision":"revision","trusted":true,"commands":[["go","test","./..."]]}"""
    for (stage in listOf("GET", "POST")) {
      for (body in
          invalid +
              if (stage == "POST") listOf(valid.replace("\"trusted\":true", "\"trusted\":false"))
              else emptyList()) {
        val main = QueuedDispatcher()
        val io = QueuedDispatcher()
        val scope = CoroutineScope(SupervisorJob() + main)
        val calls = mutableListOf<String>()
        val presenter =
            presenter(parentScope = scope, ioDispatcher = io, interceptTrust = false) {
                method,
                path,
                _ ->
              calls += "$method $path"
              when (method to path) {
                "GET" to "/api/projects/current/execution-trust?project_revision=revision" ->
                    response(if (stage == "GET") body else valid)
                "POST" to "/api/projects/current/execution-trust" -> response(body)
                else -> error("Unexpected request $method $path")
              }
            }
        try {
          loadProject(presenter)
          presenter.dispatch(DesktopEvent.VerifiedScanReadUpdated(VerifiedScanRead.Absent))
          presenter.runVerifiedScan()
          main.runPending()
          io.runPending()
          main.runPending()
          assertEquals(
              if (stage == "GET")
                  listOf("GET /api/projects/current/execution-trust?project_revision=revision")
              else
                  listOf(
                      "GET /api/projects/current/execution-trust?project_revision=revision",
                      "POST /api/projects/current/execution-trust"),
              calls)
          assertTrue(
              presenter.snapshot.value.state.verifiedScan.operation is VerifiedScanOperation.Failed)
        } finally {
          presenter.close()
          scope.cancel()
        }
      }
    }
  }

  @Test
  fun verifiedScanAdmissionStopsAfterProjectOrActionReplacementDuringTrust() {
    val valid =
        """{"project_id":"project","project_revision":"revision","trusted":true,"commands":[["go","test","./..."]]}"""
    for (stage in listOf("GET", "POST")) {
      for (replacement in listOf("project", "revision", "reindex")) {
        val main = QueuedDispatcher()
        val io = QueuedDispatcher()
        val scope = CoroutineScope(SupervisorJob() + main)
        val calls = mutableListOf<String>()
        lateinit var presenter: DesktopWorkflowPresenter
        presenter =
            presenter(parentScope = scope, ioDispatcher = io, interceptTrust = false) {
                method,
                path,
                _ ->
              calls += "$method $path"
              if ((stage == "GET" && method == "GET" && path.contains("/execution-trust?")) ||
                  (stage == "POST" && method == "POST" && path.endsWith("/execution-trust"))) {
                when (replacement) {
                  "project" ->
                      presenter.dispatch(
                          DesktopEvent.ProjectLoaded(
                              project(id = "other"), ProjectIndex("other", "revision")))
                  "revision" ->
                      presenter.dispatch(
                          DesktopEvent.ProjectLoaded(
                              project(revision = "new"), ProjectIndex("project", "new")))
                  "reindex" -> presenter.reindexProject()
                }
              }
              when {
                path.endsWith("/reindex") -> response(indexJson())
                path.contains("/execution-trust") -> response(valid)
                method == "DELETE" ->
                    response(
                        """{"project_id":"project","project_revision":"revision","status":"canceled"}""")
                path.contains("/findings?") -> response("{}")
                else -> error("Unexpected request $method $path")
              }
            }
        try {
          loadProject(presenter)
          presenter.dispatch(DesktopEvent.VerifiedScanReadUpdated(VerifiedScanRead.Absent))
          presenter.runVerifiedScan()
          main.runPending()
          io.runPending()
          main.runPending()
          io.runPending()
          main.runPending()
          assertEquals(
              if (stage == "GET") 0 else 1,
              calls.count { it == "POST /api/projects/current/execution-trust" })
          assertEquals(0, calls.count { it == "POST /api/projects/current/scan" })
        } finally {
          presenter.close()
          scope.cancel()
        }
      }
    }
  }

  @Test
  fun verifiedScanRejectsUnsupportedMissingAndUnknownLifecycleBeforeTrust() {
    val calls = mutableListOf<String>()
    val presenter = presenter { method, path, _ ->
      calls += "$method $path"
      error("Unexpected request $method $path")
    }
    try {
      presenter.runVerifiedScan()
      for (candidate in
          listOf(
              project().copy(type = "python"),
              project().copy(type = "unknown"),
              project(id = ""),
              project(revision = ""))) {
        presenter.dispatch(
            DesktopEvent.ProjectLoaded(
                candidate, ProjectIndex(candidate.projectId, candidate.projectRevision)))
        presenter.dispatch(DesktopEvent.VerifiedScanReadUpdated(VerifiedScanRead.Absent))
        presenter.runVerifiedScan()
      }
      loadProject(presenter)
      presenter.dispatch(
          DesktopEvent.GoScanLoaded(GoScanReport("project", "revision", status = "unexpected")))
      presenter.runVerifiedScan()
      assertTrue(calls.isEmpty())
    } finally {
      presenter.close()
    }
  }

  @Test
  fun securityActionsRejectReportsOwnedByTheOtherSource() {
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "POST" to "/api/projects/current/files/security-scan" -> response(securityReportJson("ai"))
        else -> response("{}")
      }
    }
    try {
      loadFile(presenter)

      presenter.scanSecurity()
      eventually {
        presenter.snapshot.value.state.security.sourceOperation.status ==
            SecuritySectionOperationStatus.Failed
      }
      assertNull(presenter.snapshot.value.state.security.sourceReport)
      assertNull(presenter.snapshot.value.state.security.aiReport)
    } finally {
      presenter.close()
    }
  }

  @Test
  fun reindexCancelsTheVisibleSecurityAction() {
    val scanStarted = CountDownLatch(1)
    val releaseScan = CountDownLatch(1)
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "POST" to "/api/projects/current/files/security-scan" -> {
          scanStarted.countDown()
          releaseScan.await(2, TimeUnit.SECONDS)
          response(securityReportJson("deterministic"))
        }
        "POST" to "/api/projects/current/reindex" -> response(indexJson())
        else -> response("{}")
      }
    }
    try {
      loadFile(presenter)
      presenter.scanSecurity()
      assertTrue(scanStarted.await(1, TimeUnit.SECONDS))

      presenter.reindexProject()

      assertTrue(presenter.snapshot.value.state.security.action.isBlank())
      assertEquals(
          SecuritySectionOperationStatus.Canceled,
          presenter.snapshot.value.state.security.sourceOperation.status)
    } finally {
      releaseScan.countDown()
      presenter.close()
    }
  }

  @Test
  fun securityResponsesCannotPublishAfterFileProjectRevisionChangesOrClose() {
    for (change in listOf("file", "project", "revision", "close")) {
      val main = QueuedDispatcher()
      val io = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + main)
      val presenter =
          DesktopWorkflowPresenter(
              ApiClient(
                  transport =
                      DaemonTransport { _, _, _ -> response(securityReportJson("deterministic")) }),
              LastProjectStore(InMemoryPreferences()),
              scope,
              io)
      try {
        loadFile(presenter)
        presenter.scanSecurity()
        main.runPending()
        io.runPending()
        when (change) {
          "file" ->
              presenter.dispatch(
                  DesktopEvent.FileLoaded(file().copy(path = "other.go"), emptyList()))
          "project" ->
              presenter.dispatch(
                  DesktopEvent.ProjectLoaded(project("other"), ProjectIndex("other", "revision")))
          "revision" ->
              presenter.dispatch(DesktopEvent.IndexRefreshed(ProjectIndex("project", "new")))
          "close" -> presenter.close()
        }
        main.runPending()
        val security = presenter.snapshot.value.state.security
        assertNull(security.sourceReport, change)
        assertNull(security.aiReport, change)
        assertTrue(security.action.isBlank(), change)
      } finally {
        presenter.close()
        scope.cancel()
        main.runPending()
      }
    }
  }

  @Test
  fun publishingAReplacementCancelsSecurityBeforeTheQueuedRequestCanRun() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val paths = mutableListOf<String>()
    val presenter =
        DesktopWorkflowPresenter(
            ApiClient(
                transport =
                    DaemonTransport { _, path, _ ->
                      paths.add(path)
                      when {
                        path.contains("/files/info") -> response(fileJson("other.go", "other"))
                        path.contains("/files/symbols") -> response(symbolsJson("other.go"))
                        else -> response("{}")
                      }
                    }),
            LastProjectStore(InMemoryPreferences()),
            scope,
            io)
    try {
      loadFile(presenter)
      presenter.scanSecurity()
      main.runPending()
      presenter.setSecurityReviewRemoteConfirmation(true)
      presenter.selectFile("other.go")
      main.runPending()
      assertEquals("main.go", presenter.snapshot.value.state.selectedFile?.path)
      assertTrue(presenter.snapshot.value.securityReviewRemoteConfirmed)
      assertEquals(
          SecuritySectionOperationStatus.Running,
          presenter.snapshot.value.state.security.sourceOperation.status)
      io.runLast() // Publish the replacement while the old-file security read is still queued.
      main.runPending()
      repeat(3) {
        main.runPending()
        io.runPending()
      }
      main.runPending()
      assertFalse(paths.any { it.contains("security") })
      assertEquals("other.go", presenter.snapshot.value.state.selectedFile?.path)
      assertFalse(presenter.snapshot.value.securityReviewRemoteConfirmed)
      assertEquals(
          SecuritySectionOperationStatus.Canceled,
          presenter.snapshot.value.state.security.sourceOperation.status)
    } finally {
      presenter.close()
      scope.cancel()
      main.runPending()
    }
  }

  @Test
  fun preparingASecurityFixOnlyOpensTheComposerPathWithoutASend() {
    val calls = mutableListOf<String>()
    val presenter = presenter { method, path, _ ->
      calls += "$method $path"
      when {
        path.contains("files/info?") ->
            response(fileJson("main.go", "base").replace("\"line_count\":1", "\"line_count\":20"))
        path.contains("files/symbols?") ->
            response(symbolsJson("main.go", "Run", "func Run()", 2, 10))
        path.contains("files/analysis") -> response("""{"path":"main.go","status":"missing"}""")
        path.contains("/impact") -> response("""{"target_path":"main.go"}""")
        path.contains("/git") -> response("""{"available":false}""")
        else -> error("Unexpected $path")
      }
    }
    try {
      val page = securityPageFixture()
      presenter.dispatch(DesktopEvent.ProjectLoaded(resultProjectFixture(), resultIndexFixture()))
      presenter.dispatch(DesktopEvent.SecurityReportLoaded(page.results!!.security.first()))
      assertTrue(presenter.snapshot.value.state.preparedRequest.isBlank())
      presenter.dispatch(
          DesktopEvent.AnalysisRunUpdated(
              ProjectAnalysisRunState(
                  run = page.run, sections = mapOf(AnalysisResultKey("security") to page.section))))
      val result =
          securityResults(presenter.snapshot.value.state.analysisResultPage("security")).first()
      presenter.prepareSecurityFinding(result)
      eventually { presenter.snapshot.value.state.preparedRequest.contains("source rule match") }
      assertTrue(
          presenter.snapshot.value.state.preparedRequest.contains("Preconditions / unknowns"))
      assertTrue(calls.none { it.startsWith("POST") })
      assertEquals("fix", presenter.snapshot.value.state.preparedAction)
    } finally {
      presenter.close()
    }
  }

  @Test
  fun staleSecurityFindingCannotPrepareAComposerRequest() {
    val sourceOpenCalls = AtomicInteger()
    val presenter = presenter { _, path, _ ->
      if (path.contains("/files/info")) sourceOpenCalls.incrementAndGet()
      response("{}")
    }
    try {
      loadProject(presenter)
      presenter.dispatch(
          DesktopEvent.FileLoaded(
              file(),
              listOf(
                  SymbolInfo(
                      "Run",
                      "function",
                      startLine = 2,
                      endLine = 4,
                      confidence = "exact",
                      atomicTarget = true))))
      val stale =
          SecurityFinding(
              id = "stale",
              anchor = SecuritySourceAnchor("main.go", 5, 5, "Run"),
              observedCondition = "Old evidence.",
              remediation = "Refresh.")
      presenter.openSecurityFinding(securityResults(securityPageFixture()).first())
      presenter.prepareSecurityFinding(
          securityResults(securityPageFixture()).first().copy(finding = stale))

      assertTrue(presenter.snapshot.value.state.jobs.error.orEmpty().isNotBlank())
      assertTrue(presenter.snapshot.value.state.preparedRequest.isBlank())
      assertEquals(0, sourceOpenCalls.get())
    } finally {
      presenter.close()
    }
  }

  @Test
  fun securitySourceOpensStaleIndexedRangeWithoutPreparingAndKeepsSameFileDraft() {
    val dispatcher = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    val calls = mutableListOf<String>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = dispatcher) { method, path, _ ->
          calls += "$method $path"
          when {
            path.contains("files/info?") ->
                response(
                    fileJson("main.go", "changed").replace("\"line_count\":1", "\"line_count\":20"))
            path.contains("files/symbols?") -> response(symbolsJson("main.go"))
            path.contains("files/analysis") -> response("""{"path":"main.go","status":"missing"}""")
            path.contains("/impact") -> response("""{"target_path":"main.go"}""")
            path.contains("/git") -> response("""{"available":false}""")
            else -> error("Unexpected request $path")
          }
        }
    try {
      val page = securityPageFixture()
      presenter.dispatch(DesktopEvent.ProjectLoaded(resultProjectFixture(), resultIndexFixture()))
      presenter.dispatch(
          DesktopEvent.AnalysisRunUpdated(
              ProjectAnalysisRunState(
                  run = page.run!!.copy(status = "stale"),
                  sections = mapOf(AnalysisResultKey("security") to page.section))))
      val result =
          securityResults(presenter.snapshot.value.state.analysisResultPage("security")).first()
      assertTrue(result.stale)
      presenter.openSecurityFinding(result)
      dispatcher.runPending()
      assertEquals("main.go", presenter.snapshot.value.state.selectedFile?.path)
      assertEquals(4, presenter.snapshot.value.state.selection.focusedLine)
      assertNull(presenter.snapshot.value.state.selection.selectedSymbol)
      assertTrue(presenter.snapshot.value.state.preparedRequest.isBlank())
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      val review = presenter.snapshot.value.state.review
      val reads = calls.size
      presenter.openSecurityFinding(result)
      dispatcher.runPending()
      assertEquals(reads, calls.size)
      assertEquals(review, presenter.snapshot.value.state.review)
      assertTrue(calls.none { it.startsWith("POST") })
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun securitySourceDraftAdmissionDoesNotDiscardOnCancelChangedBufferOrReadFailure() {
    val dispatcher = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + dispatcher)
    val calls = mutableListOf<String>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = dispatcher) { _, path, _ ->
          calls += path
          if (path.contains("files/info?")) TransportResponse(503, "")
          else error("Unexpected $path")
        }
    try {
      val page = securityPageFixture()
      presenter.dispatch(DesktopEvent.ProjectLoaded(resultProjectFixture(), resultIndexFixture()))
      presenter.dispatch(
          DesktopEvent.AnalysisRunUpdated(
              ProjectAnalysisRunState(
                  run = page.run, sections = mapOf(AnalysisResultKey("security") to page.section))))
      presenter.dispatch(DesktopEvent.FileLoaded(file().copy(path = "other.go"), emptyList()))
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      val result =
          securityResults(presenter.snapshot.value.state.analysisResultPage("security")).first()
      var pending: PendingDraftDiscard.SecuritySource? = null
      val message = TextFieldValue("keep request")
      fun route() =
          routeSecuritySourceRequest(
              presenter,
              result,
              message,
              TextFieldValue(),
              { true },
              { message to TextFieldValue() },
              {}) {
                pending = it
              }
      route()
      val canceled = requireNotNull(pending)
      val original = presenter.snapshot.value.state.review
      assertTrue(calls.isEmpty()) // Escape/cancel: no confirmation, no reads.
      presenter.dispatch(DesktopEvent.DraftEdited("new buffer"))
      confirmSecuritySourceDiscard(
          presenter, canceled, message, TextFieldValue(), { message to TextFieldValue() }, {})
      assertTrue(calls.isEmpty())
      val edited = presenter.snapshot.value.state.review
      route()
      confirmSecuritySourceDiscard(
          presenter,
          requireNotNull(pending),
          message,
          TextFieldValue(),
          { message to TextFieldValue() },
          {})
      dispatcher.runPending()
      assertTrue(calls.any { it.contains("files/info?") })
      assertEquals(edited, presenter.snapshot.value.state.review)
      assertEquals("other.go", presenter.snapshot.value.state.selectedFile?.path)
      assertEquals("keep request", message.text)
      assertTrue(original != edited)
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun securitySourceApprovalAndFailedOrObsoleteReadsPreserveWork() {
    for (change in listOf("failure", "selection", "duplicate", "composer")) {
      val main = QueuedDispatcher()
      val io = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + main)
      val presenter =
          presenter(parentScope = scope, ioDispatcher = io) { _, path, _ ->
            when {
              path.contains("files/info?") ->
                  if (change == "failure") TransportResponse(503, "")
                  else
                      response(
                          fileJson("main.go", "base")
                              .replace("\"line_count\":1", "\"line_count\":20"))
              path.contains("files/symbols?") -> response(symbolsJson("main.go"))
              else -> error("Unexpected $path")
            }
          }
      try {
        val page = securityPageFixture()
        presenter.dispatch(DesktopEvent.ProjectLoaded(resultProjectFixture(), resultIndexFixture()))
        presenter.dispatch(
            DesktopEvent.AnalysisRunUpdated(
                ProjectAnalysisRunState(
                    run = page.run,
                    sections = mapOf(AnalysisResultKey("security") to page.section))))
        val loaded = presenter.snapshot.value.state.analysisResultPage("security")
        val result = securityResults(loaded).first()
        val browser = newResultBrowserState(loaded)
        browser.selectedKey = result.rowKey
        val guard = securitySelectionGuard(loaded, browser, result)
        var message = TextFieldValue("keep input")
        var pending: PendingDraftDiscard.SecuritySource? = null
        routeSecuritySourceRequest(
            presenter,
            result,
            message,
            TextFieldValue(),
            guard,
            { message to TextFieldValue() },
            { message = TextFieldValue() }) {
              pending = it
            }
        val approval = requireNotNull(pending)
        assertNull(presenter.snapshot.value.state.selectedFile)
        if (change == "duplicate") {
          presenter.dispatch(
              DesktopEvent.AnalysisRunUpdated(
                  ProjectAnalysisRunState(
                      run = page.run,
                      sections =
                          mapOf(
                              AnalysisResultKey("security") to
                                  page.section.copy(
                                      results =
                                          page.results!!.copy(
                                              security = listOf(result.report, result.report)))))))
        }
        if (change == "composer") message = TextFieldValue("changed")
        confirmSecuritySourceDiscard(
            presenter, approval, message, TextFieldValue(), { message to TextFieldValue() }) {
              message = TextFieldValue()
            }
        if (change == "selection" || change == "failure") {
          main.runPending()
          if (change == "selection") browser.selectedKey = "another"
          io.runPending()
          main.runPending()
        }
        assertNull(presenter.snapshot.value.state.selectedFile, change)
        assertEquals(if (change == "composer") "changed" else "keep input", message.text, change)
        assertTrue(presenter.snapshot.value.state.preparedRequest.isBlank(), change)
      } finally {
        presenter.close()
        scope.cancel()
      }
    }
  }

  @Test
  fun securityPreparationWaitsForExactLoadedSourceAndPreservesWorkOnRejectionOrTimeout() {
    for (variant in
        listOf(
            "hash",
            "path",
            "revision",
            "declaration",
            "duplicate",
            "range",
            "failure",
            "sourceTimeout",
            "symbolTimeout")) {
      val main = QueuedDispatcher()
      val io = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + main)
      val calls = mutableListOf<String>()
      val presenter =
          presenter(parentScope = scope, ioDispatcher = io) { method, path, _ ->
            calls += "$method $path"
            when {
              path.contains("files/info?") ->
                  when (variant) {
                    "sourceTimeout" -> throw HttpTimeoutException("Source read timed out")
                    "failure" -> TransportResponse(503, "")
                    else ->
                        response(
                            fileJson(
                                    if (variant == "path") "other.go" else "main.go",
                                    if (variant == "hash") "changed" else "base")
                                .replace(
                                    "\"line_count\":1",
                                    "\"line_count\":${if (variant == "range") 3 else 20}"))
                  }
              path.contains("files/symbols?") -> {
                if (variant == "symbolTimeout") throw HttpTimeoutException("Symbols read timed out")
                response(
                    when (variant) {
                      "revision" ->
                          symbolsJson("main.go", "Run", "func Run()", 2, 10)
                              .replace(
                                  "\"project_revision\":\"revision\"",
                                  "\"project_revision\":\"next\"")
                      "declaration" -> symbolsJson("main.go", "Run", "func Other()", 2, 10)
                      "duplicate" ->
                          symbolsJson("main.go", "Run", "func Run()", 2, 10)
                              .replace(
                                  "}]",
                                  "},{\"name\":\"Run\",\"signature\":\"func Run()\",\"start_line\":2,\"end_line\":10,\"kind\":\"function\",\"confidence\":\"exact\",\"atomic_target\":true}]")
                      else -> symbolsJson("main.go", "Run", "func Run()", 2, 10)
                    })
              }
              else -> error("Unexpected $method $path")
            }
          }
      try {
        val page = securityPageFixture()
        presenter.dispatch(DesktopEvent.ProjectLoaded(resultProjectFixture(), resultIndexFixture()))
        presenter.dispatch(
            DesktopEvent.AnalysisRunUpdated(
                ProjectAnalysisRunState(
                    run = page.run,
                    sections = mapOf(AnalysisResultKey("security") to page.section))))
        presenter.dispatch(DesktopEvent.FileLoaded(file().copy(path = "other.go"), emptyList()))
        presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
        val result =
            securityResults(presenter.snapshot.value.state.analysisResultPage("security")).first()
        val before = presenter.snapshot.value.state.review
        var message = TextFieldValue("keep request")
        var constraints = TextFieldValue("keep constraints")
        var pending: PendingDraftDiscard.SecurityPreparation? = null
        routeSecurityPreparationRequest(
            presenter,
            result,
            message,
            constraints,
            { true },
            { message to constraints },
            {
              message = TextFieldValue()
              constraints = TextFieldValue()
            }) {
              pending = it
            }
        assertTrue(calls.isEmpty(), variant)
        assertTrue(presenter.snapshot.value.state.preparedRequest.isBlank(), variant)
        confirmSecurityPreparationDiscard(
            presenter, requireNotNull(pending), message, constraints, { message to constraints }) {
              message = TextFieldValue()
              constraints = TextFieldValue()
            }
        main.runPending()
        io.runPending()
        main.runPending()
        assertEquals(before, presenter.snapshot.value.state.review, variant)
        assertEquals("other.go", presenter.snapshot.value.state.selectedFile?.path, variant)
        assertEquals("keep request", message.text, variant)
        assertEquals("keep constraints", constraints.text, variant)
        assertTrue(presenter.snapshot.value.state.preparedRequest.isBlank(), variant)
        assertTrue(presenter.snapshot.value.state.jobs.error.orEmpty().isNotBlank(), variant)
        if (variant.endsWith("Timeout")) {
          assertTrue(
              presenter.snapshot.value.state.jobs.error.orEmpty().contains("timed out"), variant)
          assertEquals(
              variant == "symbolTimeout", calls.any { it.contains("files/symbols?") }, variant)
        }
        assertTrue(calls.none { it.startsWith("POST") }, variant)
      } finally {
        presenter.close()
        scope.cancel()
      }
    }
  }

  @Test
  fun confirmedSecurityPreparationDiscardsOnlyAfterSuccessfulReadsAndPrefillsAnUnsentRequest() {
    val main = QueuedDispatcher()
    val io = QueuedDispatcher()
    val scope = CoroutineScope(SupervisorJob() + main)
    val calls = mutableListOf<String>()
    val presenter =
        presenter(parentScope = scope, ioDispatcher = io) { method, path, _ ->
          calls += "$method $path"
          when {
            path.contains("files/info?") ->
                response(
                    fileJson("main.go", "base").replace("\"line_count\":1", "\"line_count\":20"))
            path.contains("files/symbols?") ->
                response(symbolsJson("main.go", "Run", "func Run()", 2, 10))
            path.contains("files/analysis") -> response("""{"path":"main.go","status":"missing"}""")
            path.contains("/impact") -> response("""{"target_path":"main.go"}""")
            path.contains("/git") -> response("""{"available":false}""")
            else -> error("Unexpected $method $path")
          }
        }
    try {
      val page = securityPageFixture()
      presenter.dispatch(DesktopEvent.ProjectLoaded(resultProjectFixture(), resultIndexFixture()))
      presenter.dispatch(
          DesktopEvent.AnalysisRunUpdated(
              ProjectAnalysisRunState(
                  run = page.run, sections = mapOf(AnalysisResultKey("security") to page.section))))
      presenter.dispatch(DesktopEvent.FileLoaded(file().copy(path = "other.go"), emptyList()))
      presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
      val result =
          securityResults(presenter.snapshot.value.state.analysisResultPage("security")).first()
      val original = presenter.snapshot.value.state.review
      var message = TextFieldValue("old input")
      var constraints = TextFieldValue("old constraints")
      var pending: PendingDraftDiscard.SecurityPreparation? = null
      fun route() =
          routeSecurityPreparationRequest(
              presenter,
              result,
              message,
              constraints,
              { true },
              { message to constraints },
              {
                message = TextFieldValue()
                constraints = TextFieldValue()
              }) {
                pending = it
              }
      route()
      assertTrue(pending != null)
      assertEquals(original, presenter.snapshot.value.state.review)
      assertTrue(calls.isEmpty()) // Escape or Cancel leaves everything untouched.
      pending = null
      route()
      confirmSecurityPreparationDiscard(
          presenter, requireNotNull(pending), message, constraints, { message to constraints }) {
            message = TextFieldValue()
            constraints = TextFieldValue()
          }
      assertEquals(original, presenter.snapshot.value.state.review)
      main.runPending()
      assertEquals("old input", message.text)
      assertEquals(original, presenter.snapshot.value.state.review)
      io.runPending()
      main.runPending()
      assertEquals("main.go", presenter.snapshot.value.state.selectedFile?.path)
      assertNull(presenter.snapshot.value.state.review.draft)
      assertEquals("", message.text)
      assertEquals("", constraints.text)
      assertEquals("fix", presenter.snapshot.value.state.preparedAction)
      assertTrue(presenter.snapshot.value.state.preparedRequest.contains("source rule match"))
      assertTrue(calls.none { it.startsWith("POST") })
    } finally {
      presenter.close()
      scope.cancel()
    }
  }

  @Test
  fun securityPreparationRejectsObsoleteApprovalAndLateSuccessOrFailure() {
    for (variant in
        listOf(
            "approval",
            "selection",
            "index",
            "result",
            "project",
            "draft",
            "composer",
            "fileRequest",
            "lateFailure")) {
      val main = QueuedDispatcher()
      val io = QueuedDispatcher()
      val scope = CoroutineScope(SupervisorJob() + main)
      val calls = mutableListOf<String>()
      val presenter =
          presenter(parentScope = scope, ioDispatcher = io) { method, path, _ ->
            calls += "$method $path"
            when {
              path.contains("files/info?") ->
                  if (variant == "lateFailure") TransportResponse(503, "")
                  else
                      response(
                          fileJson("main.go", "base")
                              .replace("\"line_count\":1", "\"line_count\":20"))
              path.contains("files/symbols?") ->
                  response(symbolsJson("main.go", "Run", "func Run()", 2, 10))
              else -> error("Unexpected $method $path")
            }
          }
      try {
        val page = securityPageFixture()
        val index =
            resultIndexFixture().let {
              if (variant == "fileRequest")
                  it.copy(files = it.files + IndexedFile("other.go", "base", "Go", false))
              else it
            }
        presenter.dispatch(DesktopEvent.ProjectLoaded(resultProjectFixture(), index))
        presenter.dispatch(
            DesktopEvent.AnalysisRunUpdated(
                ProjectAnalysisRunState(
                    run = page.run,
                    sections = mapOf(AnalysisResultKey("security") to page.section))))
        presenter.dispatch(DesktopEvent.FileLoaded(file().copy(path = "other.go"), emptyList()))
        presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
        val loaded = presenter.snapshot.value.state.analysisResultPage("security")
        val result = securityResults(loaded).first()
        val browser = newResultBrowserState(loaded)
        browser.selectedKey = result.rowKey
        val guard = securitySelectionGuard(loaded, browser, result)
        var message = TextFieldValue("keep input")
        var pending: PendingDraftDiscard.SecurityPreparation? = null
        routeSecurityPreparationRequest(
            presenter,
            result,
            message,
            TextFieldValue(),
            guard,
            { message to TextFieldValue() },
            { message = TextFieldValue() }) {
              pending = it
            }
        val approval = requireNotNull(pending)
        if (variant == "approval") presenter.dispatch(DesktopEvent.DraftEdited("edited buffer"))
        if (variant == "approval") {
          confirmSecurityPreparationDiscard(
              presenter, approval, message, TextFieldValue(), { message to TextFieldValue() }) {
                message = TextFieldValue()
              }
          assertTrue(calls.isEmpty())
        } else {
          confirmSecurityPreparationDiscard(
              presenter, approval, message, TextFieldValue(), { message to TextFieldValue() }) {
                message = TextFieldValue()
              }
          main.runPending()
          when (variant) {
            "selection",
            "lateFailure" -> browser.selectedKey = "another"
            "index" ->
                presenter.dispatch(
                    DesktopEvent.IndexRefreshed(
                        index.copy(files = index.files.map { it.copy(contentHash = "changed") })))
            "result" ->
                presenter.dispatch(
                    DesktopEvent.AnalysisRunUpdated(
                        ProjectAnalysisRunState(
                            run = page.run,
                            sections =
                                mapOf(
                                    AnalysisResultKey("security") to
                                        page.section.copy(
                                            results =
                                                page.results!!.copy(
                                                    security =
                                                        listOf(result.report, result.report)))))))
            "project" ->
                presenter.dispatch(
                    DesktopEvent.ProjectLoaded(
                        resultProjectFixture().copy(projectRevision = "next"),
                        index.copy(projectRevision = "next")))
            "draft" -> presenter.dispatch(DesktopEvent.DraftEdited("edited buffer"))
            "composer" -> message = TextFieldValue("changed input")
            "fileRequest" -> {
              // Reactivating retained source cancels its pending replacement without reloading.
              assertTrue(
                  presenter.confirmFileNavigationIntent(
                      presenter.fileNavigationIntent("other.go")!!))
            }
          }
          io.runPending()
          main.runPending()
        }
        assertTrue(presenter.snapshot.value.state.preparedRequest.isBlank(), variant)
        assertEquals(
            if (variant == "composer") "changed input" else "keep input", message.text, variant)
        assertTrue(calls.none { it.startsWith("POST") }, variant)
      } finally {
        presenter.close()
        scope.cancel()
      }
    }
  }

  private class QueuedDispatcher : CoroutineDispatcher() {
    private val pending = ArrayDeque<Runnable>()

    override fun dispatch(context: CoroutineContext, block: Runnable) {
      pending.addLast(block)
    }

    fun runPending() {
      while (pending.isNotEmpty()) pending.removeFirst().run()
    }

    fun runLast() {
      pending.removeLast().run()
    }

    fun runNext() {
      pending.removeFirst().run()
    }
  }

  private fun presenter(
      parentScope: CoroutineScope = CoroutineScope(SupervisorJob() + Dispatchers.Default),
      interceptTrust: Boolean = true,
      ioDispatcher: CoroutineDispatcher = Dispatchers.Default,
      lastProjectStore: LastProjectStore = LastProjectStore(InMemoryPreferences()),
      responder: (String, String, String?) -> TransportResponse,
  ): DesktopWorkflowPresenter {
    return DesktopWorkflowPresenter(
        ApiClient(
            transport =
                DaemonTransport { method, path, body ->
                  if (interceptTrust &&
                      method == "GET" &&
                      path == "/api/projects/current/execution-trust?project_revision=revision")
                      response(
                          """{"project_id":"project","project_revision":"revision","trusted":false,"commands":[["go","test","./..."]]}""")
                  else if (interceptTrust &&
                      method == "POST" &&
                      path == "/api/projects/current/execution-trust")
                      response(
                          """{"project_id":"project","project_revision":"revision","trusted":true,"commands":[["go","test","./..."]]}""")
                  else responder(method, path, body)
                }),
        lastProjectStore,
        parentScope,
        ioDispatcher,
        5)
  }

  private fun loadProject(presenter: DesktopWorkflowPresenter) {
    presenter.dispatch(
        DesktopEvent.ProjectLoaded(
            project(),
            ProjectIndex(
                "project",
                "revision",
                files =
                    listOf("main.go", "other.go").map { IndexedFile(it, "base", "Go", false) })))
  }

  private fun loadQueuedChatFile(
      presenter: DesktopWorkflowPresenter,
      main: QueuedDispatcher,
      io: QueuedDispatcher,
  ) {
    loadProject(presenter)
    presenter.selectFile("main.go")
    repeat(4) {
      main.runPending()
      io.runPending()
    }
    presenter.dispatch(
        DesktopEvent.SymbolSelected(
            SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true)))
    assertEquals("main.go", presenter.snapshot.value.state.selectedFile?.path)
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

  private fun benchmarkCatalogJson(trusted: Boolean = false) =
      """{"draft_id":"draft","draft_revision":1,"draft_hash":"draft-hash","project_id":"project","project_revision":"revision","base_file_hash":"base","target_path":"main.go","available":true,"trusted":$trusted,"benchmarks":[{"name":"BenchmarkRun","command":["go","test","-run","^$","-bench","^BenchmarkRun$","-benchtime","100ms","-benchmem"],"scope":"scope"}]}"""

  private fun benchmarkComparisonJson() =
      """{"draft_id":"draft","draft_revision":1,"draft_hash":"draft-hash","project_id":"project","project_revision":"revision","base_file_hash":"base","target_path":"main.go","benchmark":"BenchmarkRun","scope":"scope","status":"completed","command":["go","test","-run","^$","-bench","^BenchmarkRun$","-benchtime","100ms","-benchmem"],"base":{"samples":[{"iterations":1,"ns_per_op":100,"bytes_per_op":10,"allocs_per_op":1},{"iterations":1,"ns_per_op":100,"bytes_per_op":10,"allocs_per_op":1},{"iterations":1,"ns_per_op":100,"bytes_per_op":10,"allocs_per_op":1},{"iterations":1,"ns_per_op":100,"bytes_per_op":10,"allocs_per_op":1},{"iterations":1,"ns_per_op":100,"bytes_per_op":10,"allocs_per_op":1}]},"candidate":{"samples":[{"iterations":1,"ns_per_op":90,"bytes_per_op":10,"allocs_per_op":1},{"iterations":1,"ns_per_op":90,"bytes_per_op":10,"allocs_per_op":1},{"iterations":1,"ns_per_op":90,"bytes_per_op":10,"allocs_per_op":1},{"iterations":1,"ns_per_op":90,"bytes_per_op":10,"allocs_per_op":1},{"iterations":1,"ns_per_op":90,"bytes_per_op":10,"allocs_per_op":1}]}}"""

  private fun unavailableBenchmarkComparisonJson(scope: String) =
      """{"draft_id":"draft","draft_revision":1,"draft_hash":"draft-hash","project_id":"project","project_revision":"revision","base_file_hash":"base","target_path":"main.go","benchmark":"BenchmarkRun","scope":"$scope","status":"unavailable","reason":"displayed benchmark scope changed","command":["go","test","-benchtime","100ms","-benchmem"]}"""

  private fun response(body: String) = TransportResponse(200, body)

  private fun projectStartupResponse(method: String, path: String): TransportResponse =
      when (method to path) {
        "GET" to "/status" -> response("""{"status":"running","version":"test"}""")
        "POST" to "/api/projects/import",
        "POST" to "/api/projects/restore" -> response(projectJson())
        "GET" to "/api/projects/current/index" -> response(indexJson())
        "GET" to "/api/models/current",
        "GET" to "/api/projects/current/overview?project_revision=revision",
        "GET" to "/api/projects/current/findings?project_revision=revision" -> response("{}")
        "GET" to "/api/projects/current/analysis/run?project_id=project&project_revision=revision",
        "GET" to "/api/projects/current/scan?project_revision=revision" ->
            TransportResponse(204, "")
        else -> error("Unexpected request $method $path")
      }

  private fun chatFileResponse(path: String): TransportResponse? =
      when {
        path.contains("files/info?path=main.go") -> response(fileJson("main.go", "base"))
        path.contains("files/symbols?path=main.go") -> response(symbolsJson("main.go", "Run"))
        path.contains("files/analysis") -> response("""{"path":"main.go","status":"missing"}""")
        path.contains("/impact") -> response("""{"target_path":"main.go"}""")
        path.contains("/git") -> response("""{"available":false}""")
        else -> null
      }

  private fun creationFileResponse(path: String): TransportResponse? =
      when {
        path.contains("files/info?path=main.go") -> response(fileJson("main.go", "base"))
        path.contains("files/symbols?path=main.go") -> response(symbolsJson("main.go"))
        path.contains("files/analysis") -> response("""{"path":"main.go","status":"missing"}""")
        path.contains("/impact") -> response("""{"target_path":"main.go"}""")
        path.contains("/git") -> response("""{"available":false}""")
        else -> null
      }

  private fun creationSessionJson() =
      """{"id":"session","project_id":"project","project_revision":"revision","base_file_hash":"base","open_path":"main.go","mode":"create_symbol","target_symbol":"新規","state":"active","messages":[]}"""

  private fun creationProposalJson() =
      """{"session_id":"session","draft":{"id":"created","project_id":"project","project_revision":"revision","base_file_hash":"base","target_path":"main.go","mode":"create_symbol","target_symbol":"新規","declaration":"func 新規() int { return 1 }","revision":1,"hash":"draft-hash","state":"generated"},"assistant_message":{"role":"assistant","content":"Ready for review"}}"""

  private fun functionCatalog(model: String) =
      """{"scopes":{"function":{"scope":"function","profile":"remote","model":"$model","remote_provider":true}}}"""

  private fun explanationJson(
      symbol: String,
      signature: String = "",
      start: Int = 0,
      end: Int = 0
  ) =
      """{"version":"v1","project_id":"project","project_revision":"revision","base_file_hash":"base","anchor":{"path":"main.go","symbol":"$symbol","signature":"$signature","start_line":$start,"end_line":$end},"summary":"Explains $symbol.","behavior":[],"inputs":[],"outputs":[],"side_effects":[],"error_behavior":[],"context_manifest":{"scope":"function"}}"""

  private fun securityReportJson(source: String) =
      """{"schema_version":"1","project_id":"project","project_revision":"revision","path":"main.go","content_hash":"base","status":"completed_empty","source":"$source","findings":[],"context_policy_version":"policy","generated_at":"2026-09-08T00:00:00Z"}"""

  private fun projectJson() =
      "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"name\":\"project\",\"path\":\"/tmp/project\",\"type\":\"go\",\"file_count\":0,\"source_file_count\":0,\"total_lines\":0,\"summary\":\"\",\"ai_status\":\"missing\",\"analyzed_at\":\"\"}"

  private fun indexJson() = "{\"project_id\":\"project\",\"project_revision\":\"revision\"}"

  private fun fileJson(path: String, hash: String) =
      "{\"path\":\"$path\",\"content_hash\":\"$hash\",\"name\":\"$path\",\"language\":\"Go\",\"size_bytes\":1,\"line_count\":1,\"modified_at\":\"\",\"binary\":false,\"content\":\"package main\"}"

  private fun symbolsJson(
      path: String,
      symbol: String = "",
      signature: String = "",
      start: Int = 0,
      end: Int = 0
  ) =
      if (symbol.isBlank())
          """{"project_id":"project","project_revision":"revision","path":"$path","symbols":[]}"""
      else
          """{"project_id":"project","project_revision":"revision","path":"$path","symbols":[{"name":"$symbol","signature":"$signature","start_line":$start,"end_line":$end,"kind":"function","confidence":"exact","atomic_target":true}]}"""

  private fun eventually(condition: () -> Boolean) {
    val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(2)
    while (System.nanoTime() < deadline) {
      if (condition()) return
      Thread.sleep(10)
    }
    assertTrue(condition(), "condition did not become true")
  }
}
