package io.miniorca.desktop

import androidx.compose.ui.text.input.TextFieldValue
import java.net.http.HttpTimeoutException
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
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.asCoroutineDispatcher
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch

class DesktopWorkflowPresenterTest {

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
      assertEquals(DraftEditorStatus.Generated, failed.status)
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
      assertEquals(
          DraftEditorStatus.Generated, presenter.snapshot.value.state.review.editor?.status)
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
  fun generationFailureSurvivesOtherStatusAndClearsBeforeRetryOrFileChange() {
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
                TransportResponse(503, """{"message":"provider request failed"}""")
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
      val failure = presenter.snapshot.value.state.chat.failure
      assertEquals("Request failed for Run.", presenter.snapshot.value.state.status)
      assertEquals(ChatTarget(ChatEditMode.ReplaceSymbol, "Run"), failure?.target)
      assertTrue(failure?.message.orEmpty().contains("provider request failed"))
      presenter.dispatch(DesktopEvent.Status("Another operation finished"))
      assertEquals(failure, presenter.snapshot.value.state.chat.failure)
      presenter.sendChatMessage(
          ChatEditMode.ReplaceSymbol, "", "Try preserving the public signature again")
      assertNull(presenter.snapshot.value.state.chat.failure)
      main.runPending()
      presenter.selectFile("other.go")
      drain()
      assertEquals("other.go", presenter.snapshot.value.state.selectedFile?.path)
      assertNull(presenter.snapshot.value.state.chat.failure)
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
  fun benchmarkCatalogRequestIsCanceledImmediatelyWhenAFileIsSelected() {
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

      presenter.selectFile("other.go")

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
        val presenter = presenter { method, path, _ ->
          check(path.contains("/benchmarks"))
          if (comparing && method == "GET") {
            response(benchmarkCatalogJson(trusted = true))
          } else {
            started.countDown()
            try {
              release.await(2, TimeUnit.SECONDS)
            } catch (error: InterruptedException) {
              interrupted.countDown()
              throw error
            }
            response(if (comparing) benchmarkComparisonJson() else benchmarkCatalogJson())
          }
        }
        try {
          loadFile(presenter)
          presenter.dispatch(DesktopEvent.DraftLoaded(draft()))
          presenter.loadGoBenchmarks()
          if (comparing) {
            eventually { presenter.snapshot.value.state.review.benchmark.catalog != null }
            presenter.selectGoBenchmark(
                presenter.snapshot.value.state.review.benchmark.catalog!!.benchmarks.single())
            presenter.compareSelectedGoBenchmark()
          }
          assertTrue(started.await(1, TimeUnit.SECONDS))
          if (closing) presenter.close()
          else
              presenter.dispatch(
                  DesktopEvent.ProjectLoaded(project("other", "new"), ProjectIndex("other", "new")))
          assertTrue(interrupted.await(1, TimeUnit.SECONDS))
          assertFalse(presenter.snapshot.value.state.review.benchmark.running)
          assertNull(presenter.snapshot.value.state.review.benchmark.comparison)
        } finally {
          release.countDown()
          presenter.close()
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
  fun unavailableBenchmarkResponseWithARecomputedScopeStopsAndPublishesStaleEvidence() {
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "GET" to
            "/api/projects/current/drafts/draft/benchmarks?project_revision=revision&expected_revision=1&expected_hash=draft-hash" ->
            response(benchmarkCatalogJson(trusted = true))
        "POST" to "/api/projects/current/drafts/draft/benchmarks" ->
            response(unavailableBenchmarkComparisonJson(scope = "recomputed-scope"))
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

      eventually {
        !presenter.snapshot.value.state.review.benchmark.running &&
            presenter.snapshot.value.state.review.benchmark.comparison != null
      }

      val comparison = presenter.snapshot.value.state.review.benchmark.comparison!!
      assertEquals("unavailable", comparison.status)
      assertEquals("recomputed-scope", comparison.scope)
      assertEquals(
          "Stale · selected benchmark changed",
          performanceBenchmarkPresentation(
                  comparison,
                  goBenchmarkComparisonIdentity(presenter.snapshot.value.state.review.draft),
                  presenter.snapshot.value.state.review.benchmark.selected)
              .stateLabel)
    } finally {
      presenter.close()
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
      conflictPresenter.explainSelectedDeclaration()
      eventually {
        conflictPresenter.snapshot.value.declarationExplanation.status ==
            DeclarationExplanationStatus.Stale
      }
      assertEquals(null, conflictPresenter.snapshot.value.declarationExplanation.result)

      loadFile(failedPresenter)
      failedPresenter.explainSelectedDeclaration()
      eventually {
        failedPresenter.snapshot.value.declarationExplanation.status ==
            DeclarationExplanationStatus.Failed
      }
      assertEquals(null, failedPresenter.snapshot.value.declarationExplanation.result)
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
          if (route == "POST") presenter.runVerifiedScan() else presenter.cancelVerifiedScan()
          eventually {
            presenter.snapshot.value.state.verifiedScan.operation is VerifiedScanOperation.Failed
          }
          assertEquals(retained, presenter.snapshot.value.state.findings.scan)
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
          presenter.snapshot.value.state.verifiedScan.read is VerifiedScanRead.Unavailable
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
  fun delayedVerifiedScanStartCannotReplaceACancelResponse() {
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
      eventually { presenter.snapshot.value.state.findings.scan?.status == "canceled" }
      eventually { findingsRefreshes.get() == 1 }
      releaseStart.countDown()
      Thread.sleep(25)

      assertEquals("canceled", presenter.snapshot.value.state.findings.scan?.status)
      assertEquals(1, findingsRefreshes.get())
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
      eventually { findingsRefreshes.get() == 1 }

      assertEquals(1, polls.get())
      assertEquals(1, findingsRefreshes.get())
    } finally {
      presenter.close()
    }
  }

  @Test
  fun terminalPollThatOverlapsDelayedCancelCannotPublishOrRefreshTwice() {
    val pollStarted = CountDownLatch(1)
    val releasePoll = CountDownLatch(1)
    val cancelStarted = CountDownLatch(1)
    val releaseCancel = CountDownLatch(1)
    val findingsRefreshes = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "POST" to "/api/projects/current/scan" ->
            response(
                "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"running\"}")
        "GET" to "/api/projects/current/scan?project_revision=revision" -> {
          pollStarted.countDown()
          releasePoll.await(2, TimeUnit.SECONDS)
          response(
              "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"canceled\"}")
        }
        "DELETE" to "/api/projects/current/scan?project_revision=revision" -> {
          cancelStarted.countDown()
          releaseCancel.await(2, TimeUnit.SECONDS)
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
      presenter.dispatch(DesktopEvent.VerifiedScanReadUpdated(VerifiedScanRead.Absent))
      presenter.runVerifiedScan()
      assertTrue(pollStarted.await(1, TimeUnit.SECONDS))
      presenter.cancelVerifiedScan()
      assertTrue(cancelStarted.await(1, TimeUnit.SECONDS))
      releasePoll.countDown()
      Thread.sleep(25)

      assertEquals("running", presenter.snapshot.value.state.findings.scan?.status)
      assertEquals(0, findingsRefreshes.get())

      releaseCancel.countDown()
      eventually { presenter.snapshot.value.state.findings.scan?.status == "canceled" }
      eventually { findingsRefreshes.get() == 1 }

      assertEquals(1, findingsRefreshes.get())
    } finally {
      releasePoll.countDown()
      releaseCancel.countDown()
      presenter.close()
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
  fun failedVerifiedScanCancelRestoresOnePollForACancelingScan() {
    val initialPollStarted = CountDownLatch(1)
    val releaseInitialPoll = CountDownLatch(1)
    val polls = AtomicInteger()
    val presenter = presenter { method, path, _ ->
      when (method to path) {
        "POST" to "/api/projects/current/scan" ->
            response(
                "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"canceling\"}")
        "DELETE" to "/api/projects/current/scan?project_revision=revision" ->
            TransportResponse(500, "cancel failed")
        "GET" to "/api/projects/current/scan?project_revision=revision" ->
            if (polls.incrementAndGet() == 1) {
              initialPollStarted.countDown()
              releaseInitialPoll.await(2, TimeUnit.SECONDS)
              response(
                  "{\"project_id\":\"project\",\"project_revision\":\"revision\",\"status\":\"canceling\"}")
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
      presenter.cancelVerifiedScan()
      eventually { presenter.snapshot.value.state.findings.scan?.status == "canceled" }
      Thread.sleep(25)

      assertEquals(2, polls.get())
      assertTrue(presenter.snapshot.value.state.jobs.error != null)
    } finally {
      releaseInitialPoll.countDown()
      presenter.close()
    }
  }

  @Test
  fun queuedOlderVerifiedScanStartCannotReachTheDaemonAfterCancel() {
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

      eventually { cancels.get() == 1 }
      eventually { presenter.snapshot.value.state.findings.scan?.status == "canceled" }
      assertEquals(0, starts.get())
      assertEquals("canceled", presenter.snapshot.value.state.findings.scan?.status)
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
      presenter.sendChatMessage(ChatEditMode.CreateSymbol, "func", "Return one.")
      dispatcher.runPending()
      assertTrue(requests.isEmpty())
      assertEquals(
          "func is a Go keyword. Choose a different name.", presenter.snapshot.value.state.error)
      presenter.sendChatMessage(ChatEditMode.CreateSymbol, "新規", " ")
      dispatcher.runPending()
      assertTrue(requests.isEmpty())
      presenter.sendChatMessage(ChatEditMode.CreateSymbol, "新規", "Return one.")
      dispatcher.runPending()
      val state = presenter.snapshot.value.state
      assertEquals(2, requests.size)
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
          ChatEditMode.ReplaceSymbol, "", FunctionChangePreset.BugFix.preparedMessage())
      dispatcher.runPending()
      assertEquals(0, sessionRequests.get())
      assertEquals(0, messageRequests.get())
      assertEquals(
          "Add a concise intent after the selected preset before sending.",
          presenter.snapshot.value.state.error)

      presenter.sendChatMessage(
          ChatEditMode.ReplaceSymbol, "", "Fix a bug: return a typed error for a missing user")
      dispatcher.runPending()
      assertEquals(0, sessionRequests.get())
      assertEquals(0, messageRequests.get())
      assertEquals(
          "Confirm the Function edits model destination before sending context.",
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
          response(
              "{\"id\":\"session\",\"project_id\":\"project\",\"project_revision\":\"revision\",\"base_file_hash\":\"base\",\"open_path\":\"main.go\",\"mode\":\"replace_symbol\",\"target_symbol\":\"Run\",\"state\":\"active\",\"messages\":[]}")
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
      for (replacement in listOf("project", "revision", "action")) {
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
                  else -> {
                    presenter.dispatch(
                        DesktopEvent.GoScanLoaded(
                            GoScanReport("project", "revision", status = "running")))
                    presenter.cancelVerifiedScan()
                  }
                }
              }
              when {
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
  fun selectingAFileCancelsSecurityBeforeTheQueuedRequestCanRun() {
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
        val index = resultIndexFixture()
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
            "fileRequest" -> presenter.openFileInEditor("main.go")
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

  private fun benchmarkCatalogJson(trusted: Boolean = false) =
      """{"draft_id":"draft","draft_revision":1,"draft_hash":"draft-hash","project_id":"project","project_revision":"revision","base_file_hash":"base","target_path":"main.go","available":true,"trusted":$trusted,"benchmarks":[{"name":"BenchmarkRun","command":["go","test","-run","^$","-bench","^BenchmarkRun$","-benchtime","100ms","-benchmem"],"scope":"scope"}]}"""

  private fun benchmarkComparisonJson() =
      """{"draft_id":"draft","draft_revision":1,"draft_hash":"draft-hash","project_id":"project","project_revision":"revision","base_file_hash":"base","target_path":"main.go","benchmark":"BenchmarkRun","scope":"scope","status":"completed","command":["go","test","-benchtime","100ms","-benchmem"],"base":{"samples":[{"iterations":1,"ns_per_op":100,"bytes_per_op":10,"allocs_per_op":1},{"iterations":1,"ns_per_op":100,"bytes_per_op":10,"allocs_per_op":1},{"iterations":1,"ns_per_op":100,"bytes_per_op":10,"allocs_per_op":1},{"iterations":1,"ns_per_op":100,"bytes_per_op":10,"allocs_per_op":1},{"iterations":1,"ns_per_op":100,"bytes_per_op":10,"allocs_per_op":1}]},"candidate":{"samples":[{"iterations":1,"ns_per_op":90,"bytes_per_op":10,"allocs_per_op":1},{"iterations":1,"ns_per_op":90,"bytes_per_op":10,"allocs_per_op":1},{"iterations":1,"ns_per_op":90,"bytes_per_op":10,"allocs_per_op":1},{"iterations":1,"ns_per_op":90,"bytes_per_op":10,"allocs_per_op":1},{"iterations":1,"ns_per_op":90,"bytes_per_op":10,"allocs_per_op":1}]}}"""

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

  private fun explanationJson(symbol: String) =
      """{"version":"v1","project_id":"project","project_revision":"revision","base_file_hash":"base","anchor":{"path":"main.go","symbol":"$symbol","signature":"","start_line":0,"end_line":0},"summary":"Explains $symbol.","behavior":[],"inputs":[],"outputs":[],"side_effects":[],"error_behavior":[],"context_manifest":{"scope":"function"}}"""

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
