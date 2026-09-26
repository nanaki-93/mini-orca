package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json

class DesktopStateTest {
  @Test
  fun checkAttemptsIgnoreUnrelatedLoadingAndLateOutcomes() {
    val draft =
        DeclarationDraft(
            id = "draft",
            projectId = "project",
            projectRevision = "rev",
            targetPath = "main.go",
            baseFileHash = "base",
            revision = 1,
            hash = "hash")
    val report =
        DraftCheckReport(
            draftId = "draft",
            draftRevision = 1,
            draftHash = "hash",
            targetPath = "main.go",
            applicable = true)
    val original =
        DesktopState(
            review = DraftReviewState(draft = draft, checks = report),
            jobs = JobState(loading = true))
    assertNull(original.review.checkAttempt)
    assertFalse(reviewToolWindowState(original).checksRunning)
    val running = original.reduce(DesktopEvent.ChecksStarted(1, CheckCandidate(draft)))
    assertEquals(ValidationAttemptStatus.Running, running.review.checkAttempt?.status)
    assertEquals(report, running.review.checks)
    val replaced = running.reduce(DesktopEvent.ChecksStarted(2, CheckCandidate(draft)))
    assertEquals(
        replaced,
        replaced.reduce(
            DesktopEvent.ChecksStopped(1, ValidationAttemptStatus.Failed, "old failure")))
    assertEquals(replaced, replaced.reduce(DesktopEvent.ChecksCompleted(1, report)))
    val canceled =
        replaced.reduce(DesktopEvent.ChecksStopped(2, ValidationAttemptStatus.Canceled, "canceled"))
    assertEquals(ValidationAttemptStatus.Canceled, canceled.review.checkAttempt?.status)
    assertEquals(report, canceled.review.checks)
    assertEquals(canceled, canceled.reduce(DesktopEvent.ChecksCompleted(2, report)))
    val edited = canceled.reduce(DesktopEvent.DraftLoaded(draft.copy(hash = "new")))
    assertNull(edited.review.checkAttempt)
    assertEquals(
        edited,
        edited.reduce(DesktopEvent.ChecksStopped(2, ValidationAttemptStatus.Failed, "late")))
  }

  @Test
  fun projectLoadClearsPriorSelectionAndDraft() {
    val project =
        ProjectAnalysis(
            "id",
            "revision",
            "fixture",
            "/tmp/fixture",
            "go",
            fileCount = 1,
            sourceFileCount = 1,
            totalLines = 2,
            summary = "",
            aiStatus = "fresh",
            analyzedAt = "")
    val index = ProjectIndex("id", "revision")
    val state =
        DesktopState(
                selection =
                    FileSelectionState(
                        selectedFile =
                            ProjectFileInfo(
                                "main.go",
                                "hash",
                                "main.go",
                                language = "Go",
                                sizeBytes = 1,
                                lineCount = 1,
                                modifiedAt = "",
                                binary = false)),
                review =
                    DraftReviewState(
                        draft = DeclarationDraft(id = "draft"),
                        benchmark = BenchmarkEvidenceState(running = true)),
                jobs = JobState(loading = true),
            )
            .reduce(DesktopEvent.ProjectLoaded(project, index))
    assertNull(state.selectedFile)
    assertNull(state.review.draft)
    assertFalse(state.review.benchmark.running)
    assertFalse(state.loading)
    assertEquals(project, state.project)
  }

  @Test
  fun apiClientUsesTypedTransportAndErrorMessages() {
    val client =
        ApiClient(
            transport =
                DaemonTransport { method, path, _ ->
                  assertEquals("GET", method)
                  assertEquals("/api/projects/current/index", path)
                  TransportResponse(
                      200, "{\"project_id\":\"p\",\"project_revision\":\"r\",\"files\":[]}")
                })
    assertEquals("p", client.index().projectId)
    val failed =
        ApiClient(
            transport =
                DaemonTransport { _, _, _ ->
                  TransportResponse(
                      409, "{\"message\":\"stale\",\"user_message\":\"Reload first\"}")
                })
    val error = runCatching { failed.index() }.exceptionOrNull()
    assertTrue(error is ApiException && error.message == "Reload first")
  }

  @Test
  fun apiClientReadsDaemonCanonicalVersion() {
    val client =
        ApiClient(
            transport =
                DaemonTransport { method, path, _ ->
                  assertEquals("GET", method)
                  assertEquals("/status", path)
                  TransportResponse(
                      200,
                      "{\"status\":\"running\",\"version\":\"4.4.0\",\"workflow\":\"single_coder_preview\"}")
                })

    assertEquals("4.4.0", client.status().version)
  }

  @Test
  fun indexAcceptsLegacyNullCollectionFields() {
    val client =
        ApiClient(
            transport =
                DaemonTransport { method, path, _ ->
                  assertEquals("GET", method)
                  assertEquals("/api/projects/current/index", path)
                  TransportResponse(
                      200,
                      """{"project_id":"p","project_revision":"r","files":[{"path":"main.go","content_hash":"hash","language":"Go","binary":false,"symbols":null}]}""")
                })

    val index = client.index()

    assertEquals(emptyList(), index.files.single().symbols)
  }

  @Test
  fun reindexKeepsSelectedFileAndClearsLoadingState() {
    val selected =
        ProjectFileInfo(
            "main.go",
            "hash",
            "main.go",
            language = "Go",
            sizeBytes = 1,
            lineCount = 1,
            modifiedAt = "",
            binary = false)
    val refreshed = ProjectIndex("id", "new-revision")
    val state =
        DesktopState(
                projectState =
                    ProjectWorkspaceState(
                        project = project().copy(projectId = "id"),
                        index = ProjectIndex("id", "revision")),
                selection = FileSelectionState(selectedFile = selected),
                review = DraftReviewState(benchmark = BenchmarkEvidenceState(running = true)),
                jobs = JobState(loading = true))
            .reduce(DesktopEvent.IndexRefreshed(refreshed))
    assertEquals(selected, state.selectedFile)
    assertEquals(refreshed, state.index)
    assertFalse(state.review.benchmark.running)
    assertFalse(state.loading)
    assertEquals("Project inventory refreshed", state.status)
  }

  @Test
  fun refreshedIndexRequiresTheLoadedProjectAndAUsableRevision() {
    val original = projectState()
    listOf(ProjectIndex("other", "next"), ProjectIndex("project", " ")).forEach { index ->
      assertEquals(original, original.reduce(DesktopEvent.IndexRefreshed(index)))
    }
    assertEquals(
        DesktopState(),
        DesktopState().reduce(DesktopEvent.IndexRefreshed(ProjectIndex("project", "next"))))
  }

  @Test
  fun indexingReductionRejectsLateEventsAndKeepsPreviousInventoryOnFailure() {
    val project =
        ProjectAnalysis(
            "id",
            "revision",
            "fixture",
            "/tmp/fixture",
            "go",
            fileCount = 1,
            sourceFileCount = 1,
            totalLines = 2,
            summary = "",
            aiStatus = "missing",
            analyzedAt = "")
    val previous = ProjectIndex("id", "revision")
    val attempt = ProjectIndexingAttempt(4, "id", "revision", "/tmp/fixture")
    val running =
        DesktopState(projectState = ProjectWorkspaceState(project = project, index = previous))
            .reduce(DesktopEvent.ProjectIndexingStarted(attempt))
    val failed =
        running.reduce(DesktopEvent.ProjectIndexingCompleted(attempt, ProjectIndex("wrong", "new")))
    assertEquals(previous, failed.index)
    assertTrue(failed.projectState.indexingAttempt?.outcome is ProjectIndexingOutcome.Failed)
    assertEquals(
        failed,
        failed.reduce(DesktopEvent.ProjectIndexingCompleted(attempt, ProjectIndex("id", "new"))))
    assertEquals(
        failed,
        failed.reduce(
            DesktopEvent.ProjectIndexingStopped(attempt, ProjectIndexingOutcome.Failed("late"))))

    val transportFailure =
        running.reduce(
            DesktopEvent.ProjectIndexingStopped(
                attempt, ProjectIndexingOutcome.Failed("Connection lost")))
    assertEquals(previous, transportFailure.index)
    assertEquals(project, transportFailure.project)
    assertEquals(
        ProjectIndexingOutcome.Failed("Connection lost"),
        transportFailure.projectState.indexingAttempt?.outcome)

    val opening =
        running.reduce(
            DesktopEvent.ProjectOpeningStarted(
                ProjectOpeningAttempt(5, "/tmp/other", ProjectOpeningKind.Import)))
    assertEquals(ProjectIndexingOutcome.Canceled, opening.projectState.indexingAttempt?.outcome)
    assertEquals(
        opening,
        opening.reduce(DesktopEvent.ProjectIndexingCompleted(attempt, ProjectIndex("id", "new"))))
    assertEquals(previous, opening.index)
  }

  @Test
  fun draftEditsRetainBenchmarkEvidenceButInvalidateItsCatalogAndCurrentIdentity() {
    val draft =
        DeclarationDraft(
            id = "draft",
            projectId = "project",
            projectRevision = "revision",
            baseFileHash = "base",
            targetPath = "main.go",
            revision = 1,
            hash = "candidate",
            validation =
                DeclarationValidation(
                    applicable = true,
                    scopeMode = "strict_symbol",
                    diff = UnifiedDiff("main.go", "main.go")),
        )
    val comparison =
        GoBenchmarkComparison(
            draftId = draft.id,
            draftRevision = draft.revision,
            draftHash = draft.hash,
            projectId = draft.projectId,
            projectRevision = draft.projectRevision,
            baseFileHash = draft.baseFileHash,
            targetPath = draft.targetPath,
            benchmark = "BenchmarkRun",
            status = "completed",
        )
    val updated =
        DesktopState(
                review =
                    DraftReviewState(
                        draft = draft,
                        editor = editableDraft(draft),
                        benchmark =
                            BenchmarkEvidenceState(
                                catalog =
                                    GoBenchmarkCatalog(
                                        draftId = draft.id,
                                        draftRevision = draft.revision,
                                        draftHash = draft.hash),
                                comparison = comparison,
                                running = true),
                    ),
                jobs = JobState(loading = true))
            .reduce(DesktopEvent.DraftEdited(declaration = "func Run() int { return 1 }"))

    assertEquals(comparison, updated.review.benchmark.comparison)
    assertNull(updated.review.benchmark.catalog)
    assertFalse(updated.review.benchmark.running)
    assertFalse(updated.loading)
    assertEquals(DraftEditorStatus.Dirty, updated.review.editor?.status)
  }

  @Test
  fun loadingBenchmarkCatalogRequiresAnExplicitSelectionBeforeItCanRun() {
    val catalog =
        GoBenchmarkCatalog(
            draftId = "draft",
            draftRevision = 1,
            draftHash = "candidate",
            available = true,
            benchmarks = listOf(GoBenchmarkChoice("BenchmarkRun", listOf("go", "test"), "scope")))

    val running =
        DesktopState(
            review =
                DraftReviewState(
                    benchmark = BenchmarkEvidenceState(catalog = catalog, running = true)),
            jobs = JobState(loading = true),
        )
    val loaded = running.reduce(DesktopEvent.GoBenchmarkCatalogLoaded(catalog))

    assertEquals(catalog, loaded.review.benchmark.catalog)
    assertNull(loaded.review.benchmark.selected)
    assertNull(loaded.review.benchmark.comparison)
    assertFalse(loaded.review.benchmark.running)
    assertFalse(loaded.loading)

    val selected = loaded.reduce(DesktopEvent.GoBenchmarkSelected(catalog.benchmarks.single()))
    assertFalse(selected.review.benchmark.running)
    assertFalse(selected.loading)
  }

  @Test
  fun replacingOrDiscardingADraftStopsActiveBenchmarkLoading() {
    val draft =
        DeclarationDraft(
            id = "draft",
            validation =
                DeclarationValidation(
                    applicable = true,
                    scopeMode = "strict_symbol",
                    diff = UnifiedDiff("main.go", "main.go")))
    val catalog = GoBenchmarkCatalog(available = true)
    val active =
        DesktopState(
            review =
                DraftReviewState(
                    draft = draft,
                    editor = editableDraft(draft),
                    benchmark = BenchmarkEvidenceState(catalog = catalog, running = true)),
            jobs = JobState(loading = true),
        )

    val validating = active.reduce(DesktopEvent.DraftValidationStarted(1))
    val replaced = active.reduce(DesktopEvent.DraftLoaded(DeclarationDraft(id = "replacement")))
    val discarded = active.reduce(DesktopEvent.DraftDiscarded)

    listOf(validating, replaced, discarded).forEach {
      assertFalse(it.review.benchmark.running)
      assertNull(it.review.benchmark.catalog)
      assertFalse(it.loading)
    }
  }

  @Test
  fun validationFailureAndCancellationKeepTheBufferButNotPreviousApproval() {
    val approved =
        DeclarationDraft(
            id = "draft",
            declaration = "func Run() {}",
            validation =
                DeclarationValidation(
                    true,
                    "strict_symbol",
                    diagnostics = listOf(DeclarationFinding("warning", "Previous diagnostic")),
                    diff = UnifiedDiff("main.go", "main.go")))
    val initial =
        DesktopState(review = DraftReviewState(draft = approved, editor = editableDraft(approved)))
    for (outcome in listOf(ValidationAttemptStatus.Failed, ValidationAttemptStatus.Canceled)) {
      val running = initial.reduce(DesktopEvent.DraftValidationStarted(7))
      assertEquals(DraftEditorStatus.Validating, running.review.editor?.status)
      assertNull(running.review.draft?.validation)
      assertNull(running.review.editor?.serverDraft?.validation)
      assertTrue(running.review.editor?.diagnosticsAreRetained == true)
      assertEquals("Previous diagnostic", running.review.editor.diagnostics.single().message)
      val updated =
          running.reduce(
              DesktopEvent.DraftValidationUpdated(
                  7, approved.copy(revision = 2, validation = null)))
      assertEquals("Previous diagnostic", updated.review.editor!!.diagnostics.single().message)
      val stopped =
          updated.reduce(DesktopEvent.DraftValidationStopped(7, outcome, "Connection lost"))
      assertEquals(DraftEditorStatus.Generated, stopped.review.editor?.status)
      assertEquals("func Run() {}", stopped.review.editor?.declaration)
      assertEquals("Connection lost", stopped.review.editor?.validationAttempt?.message)
      assertEquals(outcome, stopped.review.editor?.validationAttempt?.status)
      assertTrue(stopped.review.editor?.diagnosticsAreRetained == true)
      assertEquals("Previous diagnostic", stopped.review.editor.diagnostics.single().message)
      val previousChecks =
          DraftCheckReport(
              "main.go",
              true,
              draftId = approved.id,
              draftRevision = approved.revision,
              draftHash = approved.hash)
      assertFalse(
          draftApplyEligibility(
                  stopped.review.draft, previousChecks, file("main.go", approved.baseFileHash))
              .eligible)
      assertEquals(stopped, stopped.reduce(DesktopEvent.DraftValidationStopped(7, outcome, "late")))
    }
  }

  @Test
  fun predecessorCannotStopAReplacementOrAnEditedDraft() {
    val draft = DeclarationDraft(id = "draft", declaration = "func Run() {}")
    val initial =
        DesktopState(review = DraftReviewState(draft = draft, editor = editableDraft(draft)))
    val first = initial.reduce(DesktopEvent.DraftValidationStarted(1))
    val replacement = first.reduce(DesktopEvent.DraftValidationStarted(2))
    assertEquals(
        replacement,
        replacement.reduce(
            DesktopEvent.DraftValidationStopped(1, ValidationAttemptStatus.Failed, "old")))
    val edited =
        replacement.reduce(DesktopEvent.DraftEdited(declaration = "func Run() int { return 1 }"))
    assertNull(edited.review.editor?.validationAttempt)
    assertEquals(
        edited,
        edited.reduce(
            DesktopEvent.DraftValidationStopped(2, ValidationAttemptStatus.Canceled, "old")))
  }

  @Test
  fun validationResponsesRequireCurrentRequestFileAndUneditedDraft() {
    val controller = DesktopWorkflowController(projectState())
    val selected = controller.beginFileLoad("main.go")!!
    assertTrue(controller.fileLoaded(selected, file("main.go", "base"), emptyList()))
    val draft =
        DeclarationDraft(
            id = "draft",
            projectId = "project",
            projectRevision = "revision",
            targetPath = "main.go",
            baseFileHash = "base",
            revision = 1,
            declaration = "func Run() {}")
    controller.dispatch(DesktopEvent.DraftLoaded(draft))
    val (first, fileRequest) = controller.beginDraftValidation()!!
    assertNull(controller.beginDraftValidation())
    assertTrue(
        controller.draftValidationStopped(
            first, fileRequest, ValidationAttemptStatus.Canceled, "canceled"))
    val (second, _) = controller.beginDraftValidation()!!
    assertFalse(
        controller.draftValidationStopped(
            first, fileRequest, ValidationAttemptStatus.Canceled, "old"))
    assertFalse(controller.draftValidated(first, fileRequest, draft))
    assertEquals(
        ValidationAttemptStatus.Running, controller.state.review.editor?.validationAttempt?.status)
    val updated = draft.copy(revision = 2)
    assertTrue(controller.draftValidationUpdated(second, fileRequest, updated))
    assertEquals(2, controller.state.review.editor?.serverDraft?.revision)
    assertEquals("func Run() {}", controller.state.review.editor?.declaration)
    controller.dispatch(DesktopEvent.DraftEdited(declaration = "func Run() int { return 1 }"))
    assertFalse(controller.draftValidated(second, fileRequest, updated))
    assertFalse(
        controller.draftValidationStopped(
            second, fileRequest, ValidationAttemptStatus.Failed, "late"))
    val (third, _) = controller.beginDraftValidation()!!
    val otherFile = controller.beginFileLoad("other.go")!!
    assertFalse(controller.draftValidated(third, fileRequest, updated))
    assertTrue(controller.fileLoaded(otherFile, file("other.go", "other"), emptyList()))
    assertNull(controller.state.review.editor)
  }

  @Test
  fun suggestionPreparesARequestWithoutChangingTheCurrentDraft() {
    val symbol = SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true)
    val draft = DeclarationDraft(id = "draft")
    val state =
        DesktopState(review = DraftReviewState(draft = draft))
            .reduce(DesktopEvent.SuggestionPrepared("fix", "Handle empty input", symbol))
    assertEquals(symbol, state.selectedSymbol)
    assertEquals("fix", state.preparedAction)
    assertEquals("Handle empty input", state.preparedRequest)
    assertEquals(draft, state.review.draft)
  }

  @Test
  fun consumedPreparedRequestDoesNotReplayWhenTheTargetChanges() {
    val first = SymbolInfo("First", "function", confidence = "exact", atomicTarget = true)
    val second = SymbolInfo("Second", "function", confidence = "exact", atomicTarget = true)
    val task = BugTaskSpec("1", "main.go", "First", "func First()", listOf("Handle empty input."))
    val oldGeneration = DesktopState().preparedRequestGeneration
    val prepared =
        DesktopState()
            .reduce(DesktopEvent.SuggestionPrepared("fix", "Handle empty input", first, task))

    val changedTargets =
        listOf(
            prepared.reduce(DesktopEvent.SymbolSelected(second)),
            prepared.reduce(DesktopEvent.EditorContextSelected(second, second.startLine)),
            prepared.reduce(
                DesktopEvent.SourceLineSelected(SourceLineSelection(second.startLine, second))))
    changedTargets.forEach { changedTarget ->
      assertEquals("", changedTarget.preparedAction)
      assertEquals("", changedTarget.preparedRequest)
      assertNull(changedTarget.preparedTaskSpec)
      assertTrue(changedTarget.preparedRequestGeneration > prepared.preparedRequestGeneration)
      assertNull(unconsumedPreparedRequest(changedTarget, oldGeneration))
    }

    val sameTarget =
        prepared.reduce(
            DesktopEvent.SourceLineSelected(
                SourceLineSelection(9, first.copy(signature = "func First() error"))))
    assertEquals(task, sameTarget.preparedTaskSpec)
    assertEquals(prepared.preparedRequestGeneration, sameTarget.preparedRequestGeneration)

    val cleared = prepared.reduce(DesktopEvent.SuggestionCleared)
    assertEquals("", cleared.preparedRequest)
    assertNull(cleared.preparedTaskSpec)
    assertNull(unconsumedPreparedRequest(cleared, oldGeneration))

    val changedTarget = changedTargets.first()
    val preparedAgain =
        changedTarget.reduce(DesktopEvent.SuggestionPrepared("fix", "Handle empty input", second))
    assertEquals(
        "Handle empty input",
        unconsumedPreparedRequest(preparedAgain, changedTarget.preparedRequestGeneration))
  }

  @Test
  fun explainSymbolRemainsAReadOnlyPreparedAction() {
    val symbol = SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true)

    val state =
        DesktopState()
            .reduce(
                DesktopEvent.SuggestionPrepared(
                    "explain_symbol", "Show the cached explanation for Run.", symbol),
            )

    assertEquals(symbol, state.selectedSymbol)
    assertEquals("explain_symbol", state.preparedAction)
    assertEquals("Show the cached explanation for Run.", state.preparedRequest)
    assertNull(state.review.draft)
  }

  @Test
  fun fileSwitchRejectsLateEnrichmentAndClearsFileBoundState() {
    val controller = DesktopWorkflowController(projectState())
    val first = controller.beginFileLoad("first.go")!!
    assertTrue(controller.fileLoaded(first, file("first.go", "first"), emptyList()))
    val loadedFirst = controller.currentFileRequest()!!
    assertTrue(
        controller.chatLoaded(
            controller.beginChatLoad()!!.first, loadedFirst, session("first.go", "first")))

    val second = controller.beginFileLoad("second.go")!!
    assertNull(controller.state.selectedFile)
    assertNull(controller.state.chat.session)
    assertNull(controller.state.review.draft)
    assertTrue(!controller.analysisLoaded(loadedFirst, FileAnalysis("first.go", "fresh")))
    assertTrue(controller.fileLoaded(second, file("second.go", "second"), emptyList()))
    assertEquals("second.go", controller.state.selectedFile?.path)
  }

  @Test
  fun projectAndCanceledFileRequestsRejectStaleResponses() {
    val controller = DesktopWorkflowController()
    val firstProjectRequest =
        controller.beginProjectLoad("/tmp/fixture", ProjectOpeningKind.Restore)
    val secondProjectRequest =
        controller.beginProjectLoad("/tmp/fixture", ProjectOpeningKind.Import)
    assertTrue(secondProjectRequest > firstProjectRequest)
    assertEquals(
        ProjectOpeningAttempt(secondProjectRequest, "/tmp/fixture", ProjectOpeningKind.Import),
        controller.state.projectState.openingAttempt)
    val project = project()
    val index = ProjectIndex("project", "revision")

    assertFalse(controller.projectLoaded(firstProjectRequest, project, index))
    assertFalse(controller.projectFailed(firstProjectRequest, "Old restore failed"))
    assertTrue(controller.projectLoaded(secondProjectRequest, project, index))
    assertNull(controller.state.projectState.openingAttempt)
    assertFalse(controller.projectFailed(secondProjectRequest, "Late import failure"))
    val fileRequest = controller.beginFileLoad("main.go")!!
    assertTrue(controller.cancelFileLoad(fileRequest))
    assertTrue(!controller.fileLoaded(fileRequest, file("main.go", "hash"), emptyList()))
  }

  @Test
  fun fileReadErrorBelongsToCurrentFileAttemptNotTheGlobalJob() {
    val controller = DesktopWorkflowController(projectState())
    val first = controller.beginFileLoad("first.go")!!
    assertTrue(controller.fileFailed(first, "Read denied"))
    controller.dispatch(DesktopEvent.Failed("Unrelated provider failure"))
    assertEquals("Read denied", controller.state.selection.fileReadError)
    assertEquals("Unrelated provider failure", controller.state.error)

    val second = controller.beginFileLoad("second.go")!!
    assertNull(controller.state.selection.fileReadError)
    assertTrue(!controller.fileFailed(first, "Late failure"))
    assertTrue(controller.fileFailed(second, "Second file denied"))
    assertEquals("Second file denied", controller.state.selection.fileReadError)
    assertTrue(controller.fileLoaded(second, file("second.go", "hash"), emptyList()))
    assertNull(controller.state.selection.fileReadError)
    assertEquals("second.go", controller.state.selectedFile?.path)

    controller.dispatch(DesktopEvent.SelectedFileUnavailable("Source changed"))
    assertEquals("Source changed", controller.state.selection.fileReadError)
    controller.dispatch(DesktopEvent.Failed("Another operation failed"))
    assertEquals("Source changed", controller.state.selection.fileReadError)
  }

  @Test
  fun projectOpeningAttemptKeepsRequestedAndLoadedProjectsDistinct() {
    val controller = DesktopWorkflowController(projectState())
    val original = controller.state.project
    val first = controller.beginProjectLoad("/tmp/other", ProjectOpeningKind.Restore)
    assertEquals(original, controller.state.project)
    assertEquals("/tmp/other", controller.state.projectState.openingAttempt?.path)
    assertEquals(
        ProjectOpeningOutcome.Opening, controller.state.projectState.openingAttempt?.outcome)
    assertTrue(controller.projectFailed(first, "Restore denied"))
    controller.dispatch(DesktopEvent.Failed("Unrelated failure"))
    assertEquals(
        ProjectOpeningOutcome.Failed("Restore denied"),
        controller.state.projectState.openingAttempt?.outcome)
    assertEquals(original, controller.state.project)
    val second = controller.beginProjectLoad("/tmp/other", ProjectOpeningKind.Restore)
    assertEquals(
        ProjectOpeningOutcome.Opening, controller.state.projectState.openingAttempt?.outcome)
    assertFalse(controller.projectFailed(first, "Late failure"))
    assertFalse(controller.projectLoaded(first, project(), ProjectIndex("project", "revision")))
    assertEquals(second, controller.state.projectState.openingAttempt?.requestId)
    assertTrue(controller.projectLoaded(second, project(), ProjectIndex("project", "revision")))
    assertNull(controller.state.projectState.openingAttempt)
    assertEquals(original, controller.state.project)
  }

  @Test
  fun currentProjectIndexMismatchFailsWithoutReplacingTheLoadedProject() {
    for (index in listOf(ProjectIndex("other", "revision"), ProjectIndex("project", "old"))) {
      val controller = DesktopWorkflowController(projectState())
      val original = controller.state.project
      val request = controller.beginProjectLoad("/tmp/other", ProjectOpeningKind.Import)
      assertFalse(controller.projectLoaded(request, project(), index))
      assertEquals(original, controller.state.project)
      assertEquals(ProjectIndex("project", "revision"), controller.state.index)
      val failed = controller.state.projectState.openingAttempt
      assertEquals(request, failed?.requestId)
      assertTrue(failed?.outcome is ProjectOpeningOutcome.Failed)
      assertFalse(controller.isCurrentProjectRequest(request))
      assertFalse(controller.projectLoaded(request, project(), ProjectIndex("project", "revision")))
      assertEquals(failed, controller.state.projectState.openingAttempt)
    }
  }

  @Test
  fun cancellationTerminatesOnlyTheCurrentOpeningAttempt() {
    val controller = DesktopWorkflowController(projectState())
    val first = controller.beginProjectLoad("/tmp/fixture", ProjectOpeningKind.Restore)
    val second = controller.beginProjectLoad("/tmp/fixture", ProjectOpeningKind.Restore)
    assertFalse(controller.cancelProjectLoad(first))
    assertTrue(controller.cancelProjectLoad(second))
    assertEquals(
        ProjectOpeningOutcome.Canceled, controller.state.projectState.openingAttempt?.outcome)
    assertFalse(controller.state.loading)
    assertFalse(controller.projectFailed(second, "Late failure"))
    assertFalse(controller.projectLoaded(second, project(), ProjectIndex("project", "revision")))
    val third = controller.beginProjectLoad("/tmp/new", ProjectOpeningKind.Import)
    assertFalse(controller.cancelProjectLoad(second))
    assertEquals(third, controller.state.projectState.openingAttempt?.requestId)
    assertEquals(
        ProjectOpeningOutcome.Opening, controller.state.projectState.openingAttempt?.outcome)
  }

  @Test
  fun draftEligibilityRequiresMatchingLatestValidationAndChecks() {
    val selected = file("main.go", "base")
    val draft =
        DeclarationDraft(
            id = "draft",
            baseFileHash = "base",
            targetPath = "main.go",
            revision = 2,
            hash = "latest",
            validation =
                DeclarationValidation(
                    true, "replace_symbol", diff = UnifiedDiff("main.go", "main.go")))
    val staleChecks =
        DraftCheckReport("main.go", true, draftId = "draft", draftRevision = 1, draftHash = "old")
    val currentChecks =
        DraftCheckReport(
            "main.go", true, draftId = "draft", draftRevision = 2, draftHash = "latest")

    assertTrue(!draftApplyEligibility(draft, staleChecks, selected).eligible)
    assertTrue(draftApplyEligibility(draft, currentChecks, selected).eligible)
  }

  @Test
  fun workspaceSwitchKeepsTheOpenFileAndDirtyDraft() {
    val selected = file("main.go", "base")
    val draft =
        DeclarationDraft(
            id = "draft",
            targetPath = "main.go",
            baseFileHash = "base",
            declaration = "func Run() {}",
            revision = 3)
    val initial =
        DesktopState(
            workspace = Workspace.Editor,
            selection = FileSelectionState(selectedFile = selected),
            review = DraftReviewState(draft = draft),
        )

    val switched = initial.reduce(DesktopEvent.WorkspaceSelected(Workspace.Bugs))

    assertEquals(Workspace.Bugs, switched.workspace)
    assertEquals(selected, switched.selectedFile)
    assertEquals(draft, switched.review.draft)
  }

  @Test
  fun findingNavigationOnlyUsesTheActiveProjectIndexAndKeepsItsContext() {
    val index =
        ProjectIndex(
            "project",
            "revision",
            files = listOf(IndexedFile("internal/main.go", "hash", "Go", false)))
    val finding =
        UnifiedFinding(
            location = FindingLocation("internal/main.go", startLine = 7, symbol = "Run"))

    assertEquals(
        EditorNavigationTarget("internal/main.go", "Run", 7),
        findingNavigationTarget(finding, index))
    assertNull(
        findingNavigationTarget(
            finding.copy(location = FindingLocation("../outside.go", startLine = 7)), index))
  }

  @Test
  fun findingNavigationPrefersExactSymbolThenLineRange() {
    val symbols =
        listOf(
            SymbolInfo(
                "Other",
                "function",
                startLine = 1,
                endLine = 3,
                confidence = "exact",
                atomicTarget = true),
            SymbolInfo(
                "Run",
                "function",
                startLine = 10,
                endLine = 15,
                confidence = "exact",
                atomicTarget = true),
        )

    assertEquals(
        symbols[1], symbolForNavigation(symbols, EditorNavigationTarget("main.go", "Run", 2)))
    assertEquals(
        symbols[1], symbolForNavigation(symbols, EditorNavigationTarget("main.go", line = 12)))
    assertEquals(12, navigationFocusLine(EditorNavigationTarget("main.go", "Run", 12), symbols[1]))
    assertEquals(10, navigationFocusLine(EditorNavigationTarget("main.go", "Run"), symbols[1]))
    assertEquals(0, navigationFocusLine(EditorNavigationTarget("main.go"), null))
  }

  @Test
  fun findingNavigationResolvesAnUnambiguousMentionedDeclaration() {
    val command =
        SymbolInfo(
            "diffCmd", "var", startLine = 3, endLine = 9, confidence = "exact", atomicTarget = true)
    val index =
        ProjectIndex(
            "project",
            "revision",
            files =
                listOf(IndexedFile("command.go", "hash", "Go", false, symbols = listOf(command))))
    val finding =
        UnifiedFinding(
            title = "Cobra command behavior",
            message = "diffCmd should validate its input before running.",
            location = FindingLocation("command.go"),
        )

    val target = findingNavigationTarget(finding, index)

    assertEquals(EditorNavigationTarget("command.go", "diffCmd", 3), target)
    assertEquals(
        EditorNavigationSelection(command, 3),
        resolveEditorNavigation(listOf(command), requireNotNull(target)))
  }

  @Test
  fun sourceLineSelectionUsesTheMostSpecificValidDeclaration() {
    val symbols =
        listOf(
            SymbolInfo(
                "Invalid",
                "function",
                startLine = 0,
                endLine = 4,
                confidence = "exact",
                atomicTarget = true),
            SymbolInfo(
                "Container",
                "type",
                startLine = 1,
                endLine = 20,
                confidence = "exact",
                atomicTarget = true),
            SymbolInfo(
                "Nested",
                "function",
                startLine = 5,
                endLine = 8,
                confidence = "approximate",
                atomicTarget = false),
            SymbolInfo(
                "ApproximateTie",
                "function",
                startLine = 10,
                endLine = 12,
                confidence = "approximate",
                atomicTarget = false),
            SymbolInfo(
                "AtomicTie",
                "function",
                startLine = 10,
                endLine = 12,
                confidence = "exact",
                atomicTarget = true),
            SymbolInfo(
                "FirstAtomicTie",
                "function",
                startLine = 14,
                endLine = 16,
                confidence = "exact",
                atomicTarget = true),
            SymbolInfo(
                "SecondAtomicTie",
                "function",
                startLine = 14,
                endLine = 16,
                confidence = "exact",
                atomicTarget = true),
            SymbolInfo(
                "Reversed",
                "function",
                startLine = 22,
                endLine = 21,
                confidence = "exact",
                atomicTarget = true),
            SymbolInfo(
                "Empty",
                "function",
                startLine = 0,
                endLine = 0,
                confidence = "exact",
                atomicTarget = true),
            SymbolInfo(
                "Disjoint",
                "function",
                startLine = 30,
                endLine = 31,
                confidence = "exact",
                atomicTarget = true),
        )

    assertEquals("Nested", symbolAtLine(symbols, 6)?.name)
    assertEquals("AtomicTie", symbolAtLine(symbols, 11)?.name)
    assertEquals("FirstAtomicTie", symbolAtLine(symbols, 15)?.name)
    assertEquals("Disjoint", symbolAtLine(symbols, 30)?.name)
    assertNull(symbolAtLine(symbols, 21))

    val draft = DeclarationDraft(id = "draft")
    val state =
        DesktopState(
                selection = FileSelectionState(selectedSymbol = symbols[1], focusedLine = 2),
                review = DraftReviewState(draft = draft),
            )
            .reduce(DesktopEvent.SourceLineSelected(SourceLineSelection(6, symbols[2])))

    assertEquals(symbols[2], state.selectedSymbol)
    assertEquals(6, state.selection.focusedLine)
    assertEquals(draft, state.review.draft)

    val outsideDeclaration =
        state.reduce(DesktopEvent.SourceLineSelected(sourceLineSelection(symbols, 21)))

    assertNull(outsideDeclaration.selectedSymbol)
    assertEquals(21, outsideDeclaration.selection.focusedLine)
    assertEquals(draft, outsideDeclaration.review.draft)
  }

  @Test
  fun discardingForANewEditClearsOnlyTheInMemoryConversationDraftAndChecks() {
    val selected =
        ProjectFileInfo(
            "main.go",
            "hash",
            "main.go",
            language = "Go",
            sizeBytes = 1,
            lineCount = 1,
            modifiedAt = "",
            binary = false)
    val draft = DeclarationDraft(id = "draft")
    val receipt = ApplyResult("revision", "post-apply", true)
    val initial =
        DesktopState(
            selection = FileSelectionState(selectedFile = selected),
            chat = ChatState(ChatSession(id = "session")),
            review =
                DraftReviewState(
                    draft = draft,
                    editor = editableDraft(draft),
                    checks = DraftCheckReport("main.go", true),
                    applied = receipt),
        )

    val discarded = initial.reduce(DesktopEvent.DraftDiscarded)

    assertEquals(selected, discarded.selectedFile)
    assertNull(discarded.chat.session)
    assertNull(discarded.review.draft)
    assertNull(discarded.review.editor)
    assertNull(discarded.review.checks)
    assertEquals(receipt, discarded.review.applied)
  }

  @Test
  fun contextInspectorClientKeepsOnlySourceFreeManifestMetadata() {
    val client =
        ApiClient(
            "https://provider.example",
            DaemonTransport { _, _, _ ->
              TransportResponse(
                  200,
                  """{"included":[{"path":"main.go","size_bytes":20,"hash":"sha256:base","estimated_tokens":5}],"excluded":[{"path":".env","include":false,"reason":"secret"}],"estimated_tokens":5,"byte_limit":1024,"token_limit":256,"scope":"function","model":"local-code","provider_origin":"http://localhost:11434","content":"private source must not reach the UI model"}""")
            })

    val manifest = client.context("main.go")

    assertEquals("Remote endpoint", client.endpointLocality())
    assertEquals("main.go", manifest.included.single().path)
    assertEquals("secret", manifest.excluded.single().reason)
    assertEquals("function", manifest.scope)
    assertEquals("local-code", manifest.model)
    assertEquals("http://localhost:11434", manifest.providerOrigin)
    assertTrue(!Json.encodeToString(manifest).contains("private source"))
  }

  private fun project() =
      ProjectAnalysis(
          "project",
          "revision",
          "fixture",
          "/tmp/fixture",
          "go",
          fileCount = 1,
          sourceFileCount = 1,
          totalLines = 2,
          summary = "",
          aiStatus = "fresh",
          analyzedAt = "")

  private fun projectState() =
      DesktopState(
          projectState = ProjectWorkspaceState(project(), ProjectIndex("project", "revision")))

  private fun file(path: String, hash: String) =
      ProjectFileInfo(
          path,
          hash,
          path,
          language = "Go",
          sizeBytes = 1,
          lineCount = 1,
          modifiedAt = "",
          binary = false)

  private fun session(path: String, hash: String) =
      ChatSession(
          id = "session",
          projectId = "project",
          projectRevision = "revision",
          baseFileHash = hash,
          openPath = path)
}
