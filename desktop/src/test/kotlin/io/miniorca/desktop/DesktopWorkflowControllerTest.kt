package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class DesktopWorkflowControllerTest {
  @Test
  fun overviewEnrichmentKeepsDeterministicProjectStateAvailable() {
    val controller = loadedController()
    controller.dispatch(
        DesktopEvent.OverviewLoaded(
            ProjectOverview(
                projectId = "project",
                projectRevision = "revision",
                metrics = ProjectMetrics(type = "go", fileCount = 1))))

    assertEquals("project", controller.state.project?.projectId)
    assertEquals("go", controller.state.overview?.metrics?.type)
  }

  @Test
  fun lateFileResponsesCannotReplaceTheNewSelection() {
    val controller = loadedController()
    val first = controller.beginFileLoad("first.go")!!
    assertTrue(controller.fileLoaded(first, file("first.go", "first-hash"), emptyList()))
    val loadedFirst = controller.currentFileRequest()!!
    val second = controller.beginFileLoad("second.go")!!

    assertTrue(controller.analysisLoaded(loadedFirst, FileAnalysis("first.go", "fresh")))
    assertFalse(controller.fileLoaded(first, file("first.go", "first-hash"), emptyList()))
    assertTrue(controller.fileLoaded(second, file("second.go", "second-hash"), emptyList()))

    assertEquals("second.go", controller.state.selectedFile?.path)
    assertFalse(controller.analysisLoaded(loadedFirst, FileAnalysis("first.go", "late")))
    assertNull(controller.state.analysis)
  }

  @Test
  fun impactEnrichmentKeepsSuppliedScopeAndRejectsOtherFileOrSource() {
    val controller = loadedController()
    val first = controller.beginFileLoad("main.go")!!
    assertTrue(controller.fileLoaded(first, file("main.go", "hash-a"), emptyList()))
    val loaded = controller.currentFileRequest()!!
    val preview =
        ImpactPreview(
            "main.go",
            "Run",
            listOf(ImpactReference("caller.go", "CallRun", "approximate", "Indexed mention")))
    assertFalse(controller.impactLoaded(loaded, preview.copy(targetPath = "other.go")))
    assertNull(controller.state.impact)
    assertTrue(controller.impactLoaded(loaded, preview))
    assertEquals(preview, controller.state.impact)
    val replacement = controller.beginFileLoad("main.go")!!
    assertTrue(controller.fileLoaded(replacement, file("main.go", "hash-b"), emptyList()))
    assertNull(controller.state.impact)
    assertFalse(controller.impactLoaded(loaded, preview))
    assertNull(controller.state.impact)
    assertTrue(controller.optionalLoadFailed(controller.currentFileRequest()!!))
    assertNull(controller.state.impact)
  }

  @Test
  fun cancellationAndOptionalFailuresLeaveTheLoadedSourceAvailable() {
    val controller = loadedController()
    val canceled = controller.beginFileLoad("canceled.go")!!
    assertTrue(controller.cancelFileLoad(canceled))
    assertFalse(controller.state.jobs.loading)
    assertNull(controller.currentFileRequest())
    assertFalse(controller.fileLoaded(canceled, file("canceled.go", "canceled-hash"), emptyList()))

    val request = controller.beginFileLoad("main.go")!!
    assertTrue(
        controller.fileLoaded(
            request,
            file("main.go", "main-hash"),
            listOf(SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true))))
    val loadedRequest = controller.currentFileRequest()!!
    assertTrue(controller.optionalLoadFailed(loadedRequest))

    assertEquals("main.go", controller.state.selectedFile?.path)
    assertEquals("Run", controller.state.symbols.single().name)
    assertNull(controller.state.analysis)
    assertNull(controller.state.impact)
    assertNull(controller.state.gitStatus)
  }

  @Test
  fun failedAndCanceledReplacementsRetainLoadedAuthorityAndWork() {
    val controller = loadedController()
    val initial = controller.beginFileLoad("main.go")!!
    assertNull(controller.currentFileRequest())
    assertNull(controller.state.selectedFile)
    assertEquals("main.go", controller.state.selection.pendingFilePath)
    assertTrue(controller.fileLoaded(initial, file("main.go", "main-hash"), emptyList()))
    val loaded = controller.currentFileRequest()!!
    val (chatRequest, chatFile) = controller.beginChatLoad()!!
    assertTrue(
        controller.chatLoaded(
            chatRequest,
            chatFile,
            ChatSession("session", "project", "revision", "main-hash", "main.go")))
    controller.dispatch(DesktopEvent.DraftLoaded(draft()))
    controller.dispatch(DesktopEvent.DraftEdited(declaration = "func Run() { changed() }"))
    assertTrue(controller.analysisLoaded(loaded, FileAnalysis("main.go", "fresh")))
    val previous = controller.state

    val failed = controller.beginFileLoad("failed.go")!!
    assertTrue(controller.fileFailed(failed, "Read denied"))
    assertEquals(previous.selectedFile, controller.state.selectedFile)
    assertEquals(previous.chat, controller.state.chat)
    assertEquals(previous.review, controller.state.review)
    assertEquals(previous.analysis, controller.state.analysis)
    assertEquals(loaded, controller.currentFileRequest())
    assertEquals("failed.go", controller.state.selection.failedFilePath)
    assertEquals("Read denied", controller.state.selection.fileReadError)
    assertNull(controller.state.selection.pendingFilePath)
    assertFalse(controller.state.jobs.loading)
    assertFalse(controller.fileFailed(failed, "late failure"))
    assertFalse(controller.fileLoaded(failed, file("failed.go", "failed-hash"), emptyList()))

    val canceled = controller.beginFileLoad("canceled.go")!!
    assertTrue(controller.cancelFileLoad(canceled))
    assertEquals(previous.selection, controller.state.selection)
    assertEquals(previous.chat, controller.state.chat)
    assertEquals(previous.review, controller.state.review)
    assertEquals(loaded, controller.currentFileRequest())
    assertFalse(controller.state.jobs.loading)
    assertFalse(controller.cancelFileLoad(canceled))
    assertFalse(controller.fileLoaded(canceled, file("canceled.go", "hash"), emptyList()))
    assertTrue(controller.analysisLoaded(loaded, FileAnalysis("main.go", "still current")))
  }

  @Test
  fun ordinaryReplacementPublicationRequiresUnchangedCapturedWorkAndIndex() {
    for (change in listOf("none", "session", "editor", "index", "source")) {
      val controller = loadedController()
      controller.dispatch(
          DesktopEvent.IndexRefreshed(
              index("project", "revision")
                  .copy(files = listOf(IndexedFile("other.go", "other", "Go", false)))))
      val initial = controller.beginFileLoad("main.go")!!
      assertTrue(controller.fileLoaded(initial, file("main.go", "main-hash"), emptyList()))
      controller.dispatch(DesktopEvent.DraftLoaded(draft()))
      val identity = controller.state.fileNavigationIdentity("other.go")!!
      val request = controller.beginFileLoad("other.go")!!
      when (change) {
        "session" ->
            controller.dispatch(
                DesktopEvent.ChatLoaded(
                    ChatSession("new", "project", "revision", "main-hash", "main.go")))
        "editor" -> controller.dispatch(DesktopEvent.DraftEdited("new declaration"))
        "index" -> controller.dispatch(DesktopEvent.IndexRefreshed(controller.state.index!!.copy()))
        "source" ->
            controller.dispatch(
                DesktopEvent.SelectedFileRefreshed(file("main.go", "new-hash"), emptyList()))
      }
      val newer = controller.state
      val published =
          controller.fileNavigationLoaded(request, file("other.go", "other"), emptyList(), identity)
      assertEquals(change == "none", published, change)
      if (published) {
        assertEquals("other.go", controller.state.selectedFile?.path)
        assertNull(controller.state.review.editor)
        assertFalse(
            controller.fileNavigationLoaded(
                request, file("other.go", "other"), emptyList(), identity))
      } else {
        assertEquals(newer, controller.state, change)
        assertTrue(controller.cancelFileLoad(request))
        assertEquals(newer.selectedFile, controller.state.selectedFile, change)
        assertEquals(newer.review, controller.state.review, change)
        assertEquals(newer.chat, controller.state.chat, change)
      }
    }
  }

  @Test
  fun supersededFailureAndCancellationCannotClearTheNewPendingRead() {
    val controller = loadedController()
    val first = controller.beginFileLoad("first.go")!!
    val second = controller.beginFileLoad("second.go")!!
    assertFalse(controller.fileFailed(first, "late failure"))
    assertFalse(controller.cancelFileLoad(first))
    assertFalse(controller.fileLoaded(first, file("first.go", "hash"), emptyList()))
    assertEquals("second.go", controller.state.selection.pendingFilePath)
    assertTrue(controller.state.jobs.loading)
    assertTrue(controller.fileFailed(second, "current failure"))
    assertFalse(controller.cancelFileLoad(first))
    assertEquals("second.go", controller.state.selection.failedFilePath)
    assertEquals("current failure", controller.state.selection.fileReadError)
  }

  @Test
  fun wrongPathCannotPublishAndLateResultsCannotClearANewerError() {
    val controller = loadedController()
    val initial = controller.beginFileLoad("main.go")!!
    assertTrue(controller.fileLoaded(initial, file("main.go", "base"), emptyList()))
    val wrong = controller.beginFileLoad("other.go")!!
    assertFalse(controller.fileLoaded(wrong, file("foreign.go", "foreign"), emptyList()))
    assertEquals("main.go", controller.state.selectedFile?.path)
    assertEquals(wrong, controller.currentPendingFileRequest())

    val newer = controller.beginFileLoad("second.go")!!
    assertTrue(controller.fileFailed(newer, "Current read denied"))
    val failed = controller.state
    assertFalse(controller.fileLoaded(wrong, file("other.go", "late"), emptyList()))
    assertFalse(controller.fileFailed(wrong, "Old read denied"))
    assertFalse(controller.cancelFileLoad(wrong))
    assertEquals(failed, controller.state)
  }

  @Test
  fun fileRequestsCannotReviveAfterProjectOrRevisionRoundTrips() {
    for (change in listOf("project", "revision", "reload")) {
      val controller = loadedController()
      val old = controller.beginFileLoad("main.go")!!
      when (change) {
        "project" -> {
          controller.dispatch(
              DesktopEvent.ProjectLoaded(project("other", "revision"), index("other", "revision")))
          controller.dispatch(
              DesktopEvent.ProjectLoaded(
                  project("project", "revision"), index("project", "revision")))
        }
        "revision" -> {
          controller.dispatch(DesktopEvent.IndexRefreshed(index("project", "next")))
          controller.dispatch(DesktopEvent.IndexRefreshed(index("project", "revision")))
        }
        else ->
            controller.dispatch(
                DesktopEvent.ProjectLoaded(
                    project("project", "revision"), index("project", "revision")))
      }
      assertNull(controller.currentPendingFileRequest(), change)
      assertNull(controller.state.selection.pendingFilePath, change)
      val current = controller.beginFileLoad("other.go")!!
      assertFalse(controller.fileLoaded(old, file("main.go", "late"), emptyList()), change)
      assertFalse(controller.fileFailed(old, "Late failure"), change)
      assertFalse(controller.cancelFileLoad(old), change)
      assertEquals(current, controller.currentPendingFileRequest(), change)
      assertEquals("other.go", controller.state.selection.pendingFilePath, change)
      assertTrue(controller.fileLoaded(current, file("other.go", "current"), emptyList()), change)
    }
  }

  @Test
  fun projectSwitchClearsFileBoundChatAndDraft() {
    val controller = loadedController()
    val initialRequest = controller.beginFileLoad("main.go")!!
    assertTrue(controller.fileLoaded(initialRequest, file("main.go", "main-hash"), emptyList()))
    val fileRequest = controller.currentFileRequest()!!
    val (chatRequest, chatFile) = controller.beginChatLoad()!!
    assertTrue(
        controller.chatLoaded(
            chatRequest,
            chatFile,
            ChatSession("session", "project", "revision", "main-hash", "main.go")))
    val draft = draft()
    val (draftRequest, draftFile) = controller.beginDraftLoad()!!
    assertTrue(controller.draftLoaded(draftRequest, draftFile, draft))

    val projectRequest = controller.beginProjectLoad()
    assertTrue(
        controller.projectLoaded(
            projectRequest, project("next", "next-revision"), index("next", "next-revision")))

    assertNull(controller.state.selectedFile)
    assertNull(controller.state.chat.session)
    assertNull(controller.state.review.draft)
    assertFalse(controller.analysisLoaded(fileRequest, FileAnalysis("main.go", "fresh")))
  }

  @Test
  fun indexingIsSingleFlightAndCapturesProjectIdentity() {
    val controller = loadedController()
    val first = controller.beginProjectIndexing()!!
    assertEquals("project", first.projectId)
    assertEquals("revision", first.projectRevision)
    assertEquals("/tmp/project", first.path)
    assertNull(controller.beginProjectIndexing())
    assertTrue(controller.projectIndexingCompleted(first, index("project", "new-revision")))
    assertEquals(
        ProjectIndexingOutcome.Succeeded("new-revision"),
        controller.state.projectState.indexingAttempt?.outcome)
    assertEquals("new-revision", controller.state.index?.projectRevision)
    assertFalse(controller.projectIndexingFailed(first, "late"))
    assertFalse(controller.projectIndexingCompleted(first, index("project", "older")))
    val second = controller.beginProjectIndexing()!!
    assertEquals("new-revision", second.projectRevision)
    assertFalse(controller.projectIndexingFailed(first, "late"))
    assertTrue(controller.cancelProjectIndexing(second))
    assertEquals(
        ProjectIndexingOutcome.Canceled, controller.state.projectState.indexingAttempt?.outcome)
    assertFalse(controller.projectIndexingCompleted(second, index("project", "late")))
  }

  @Test
  fun openingInvalidatesRunningIndexEvenIfItsJobCompletes() {
    val controller = loadedController()
    val first = controller.beginProjectIndexing()!!
    val opening = controller.beginProjectLoad("/tmp/next")
    assertEquals(
        ProjectIndexingOutcome.Canceled, controller.state.projectState.indexingAttempt?.outcome)
    assertNull(controller.beginProjectIndexing())
    assertFalse(controller.projectIndexingCompleted(first, index("project", "late")))
    assertFalse(controller.projectIndexingFailed(first, "late failure"))
    assertTrue(
        controller.projectLoaded(opening, project("next", "revision"), index("next", "revision")))
    assertNull(controller.state.projectState.indexingAttempt)
    assertEquals("next", controller.state.index?.projectId)
  }

  @Test
  fun oldResultsCannotPublishAfterReturningToSameProject() {
    val controller = loadedController()
    val old = controller.beginProjectIndexing()!!
    val toB = controller.beginProjectLoad()
    assertTrue(controller.projectLoaded(toB, project("b", "revision"), index("b", "revision")))
    val toA = controller.beginProjectLoad()
    assertTrue(
        controller.projectLoaded(toA, project("project", "revision"), index("project", "revision")))
    val current = controller.beginProjectIndexing()!!
    assertFalse(controller.projectIndexingCompleted(old, index("project", "old")))
    assertFalse(controller.projectIndexingFailed(old, "old failure"))
    assertEquals(
        ProjectIndexingOutcome.Running, controller.state.projectState.indexingAttempt?.outcome)
    assertTrue(controller.projectIndexingFailed(current, "cannot index"))
    assertEquals(
        ProjectIndexingOutcome.Failed("cannot index"),
        controller.state.projectState.indexingAttempt?.outcome)
    assertEquals("revision", controller.state.index?.projectRevision)
  }

  @Test
  fun mismatchedIndexEndsCurrentAttemptWithoutReplacingInventory() {
    val controller = loadedController()
    val wrong = controller.beginProjectIndexing()!!
    assertFalse(controller.projectIndexingCompleted(wrong, index("other", "new")))
    assertTrue(
        controller.state.projectState.indexingAttempt?.outcome is ProjectIndexingOutcome.Failed)
    assertEquals("revision", controller.state.index?.projectRevision)
    val blank = controller.beginProjectIndexing()!!
    assertFalse(controller.projectIndexingCompleted(blank, index("project", "  ")))
    assertTrue(
        controller.state.projectState.indexingAttempt?.outcome is ProjectIndexingOutcome.Failed)
    assertEquals("revision", controller.state.index?.projectRevision)
    assertTrue(controller.projectIndexingFailed(controller.beginProjectIndexing()!!, ""))
    assertTrue(
        (controller.state.projectState.indexingAttempt?.outcome as ProjectIndexingOutcome.Failed)
            .message
            .isNotBlank())
  }

  @Test
  fun indexingRejectsMissingProject() {
    assertNull(DesktopWorkflowController().beginProjectIndexing())
  }

  @Test
  fun applyEligibilityRequiresTheLatestValidatedAndCheckedDraft() {
    val selected = file("main.go", "main-hash")
    val draft = draft()
    val matchingChecks =
        DraftCheckReport(
            targetPath = "main.go",
            applicable = true,
            draftId = draft.id,
            draftRevision = draft.revision,
            draftHash = draft.hash,
            projectId = draft.projectId,
            projectRevision = draft.projectRevision,
            baseFileHash = draft.baseFileHash,
        )

    assertTrue(draftApplyEligibility(draft, matchingChecks, selected).eligible)
    assertFalse(
        draftApplyEligibility(draft, matchingChecks.copy(draftRevision = 1), selected).eligible)
    assertFalse(
        draftApplyEligibility(draft.copy(validation = null), matchingChecks, selected).eligible)
    assertFalse(
        draftApplyEligibility(draft, matchingChecks, file("other.go", "main-hash")).eligible)
  }

  private fun loadedController(): DesktopWorkflowController =
      DesktopWorkflowController().also { controller ->
        val request = controller.beginProjectLoad()
        assertTrue(
            controller.projectLoaded(
                request, project("project", "revision"), index("project", "revision")))
      }

  private fun project(id: String, revision: String) =
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

  private fun index(id: String, revision: String) = ProjectIndex(id, revision)

  private fun file(path: String, hash: String) =
      ProjectFileInfo(
          path,
          hash,
          path,
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
          baseFileHash = "main-hash",
          targetPath = "main.go",
          revision = 2,
          hash = "draft-hash",
          validation =
              DeclarationValidation(
                  true, "strict_symbol", diff = UnifiedDiff("main.go", "main.go")))
}
