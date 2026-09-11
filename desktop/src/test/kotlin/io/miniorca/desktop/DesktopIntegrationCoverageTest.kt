package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

/** Deterministic cross-boundary smoke coverage; no daemon, model, or UI window is required. */
class DesktopIntegrationCoverageTest {
  @Test
  fun resultNavigationPreservesFileSelectionFiltersAndAnalysisOwnership() {
    val controller = loadedController()
    val request = controller.beginFileLoad("main.go")!!
    assertTrue(controller.fileLoaded(request, file(), listOf(symbol())))
    val run =
        ProjectAnalysisRunState(
            run = analysisRunFixture(),
            resultPaths = mapOf("bugs" to "main.go", "security" to "internal/"))
    controller.dispatch(DesktopEvent.AnalysisRunUpdated(run))
    controller.dispatch(DesktopEvent.DraftLoaded(draft()))
    val review = controller.state.review
    val selection = controller.state.selection
    listOf("open_analysis", "open_bugs", "open_performance", "open_security").forEach { action ->
      controller.dispatch(
          DesktopEvent.WorkspaceSelected(requireNotNull(commandActionWorkspace(action))))
      assertEquals(run, controller.state.analysisRun)
      assertEquals(selection, controller.state.selection)
      assertEquals(review, controller.state.review)
      assertTrue(controller.state.preparedRequest.isBlank())
    }
    controller.dispatch(DesktopEvent.WorkspaceSelected(Workspace.Editor))
    assertEquals(selection, controller.state.selection)
    assertEquals(run, controller.state.analysisRun)
  }

  @Test
  fun partialResultsAndAnOpenDraftBecomeStaleTogetherAfterObservedSourceChanges() {
    val controller = loadedController()
    val load = controller.beginFileLoad("main.go")!!
    assertTrue(controller.fileLoaded(load, file(), listOf(symbol())))
    controller.dispatch(DesktopEvent.DraftLoaded(draft()))
    val run = acceptanceRun("partial")!!
    val sections =
        listOf("bugs", "performance", "security").associate { category ->
          AnalysisResultKey(category) to
              AnalysisSectionState(results = analysisResultsFixture(run, category))
        }
    controller.dispatch(
        DesktopEvent.AnalysisRunUpdated(ProjectAnalysisRunState(run = run, sections = sections)))
    val review = controller.state.review
    listOf("bugs", "performance", "security").forEach { category ->
      val page = controller.state.analysisResultPage(category)
      assertEquals("Partial", page.statusLabel)
      assertEquals(run.identity, page.results?.identity)
      assertEquals(category, page.results?.progress?.category)
    }
    controller.dispatch(DesktopEvent.SelectedFileRefreshed(file(), listOf(symbol())))
    assertEquals(review, controller.state.review)
    controller.dispatch(
        DesktopEvent.SelectedFileRefreshed(file(hash = "shell-change"), listOf(symbol())))
    assertEquals(DraftEditorStatus.Stale, controller.state.review.editor?.status)
    assertEquals(review.draft, controller.state.review.draft)
    listOf("bugs", "performance", "security").forEach { category ->
      val page = controller.state.analysisResultPage(category)
      assertEquals("Stale", page.statusLabel)
      assertNull(page.reportedCount)
      assertEquals(run.identity, page.results?.identity)
    }
    assertFalse(
        draftReviewEligibility(
                controller.state.review.editor,
                controller.state.review.draft,
                controller.state.review.checks,
                controller.state.selectedFile,
                controller.state.project)
            .eligible)
  }

  @Test
  fun projectOpenCancelFailureAndSuccessKeepTheLandingTransitionExplicit() {
    val controller = DesktopWorkflowController()
    val beforeChooserCancel = controller.state

    assertEquals(beforeChooserCancel, controller.state)
    assertEquals(DesktopShellMode.ProjectLanding, desktopShellMode(controller.state))

    val failedRequest = controller.beginProjectLoad()
    assertTrue(controller.state.loading)
    assertEquals(DesktopShellMode.ProjectLanding, desktopShellMode(controller.state))
    assertTrue(controller.isCurrentProjectRequest(failedRequest))
    controller.dispatch(DesktopEvent.Failed("Import failed"))
    assertTrue(!controller.state.loading)
    assertEquals("Import failed", controller.state.error)
    assertEquals(DesktopShellMode.ProjectLanding, desktopShellMode(controller.state))

    val successfulRequest = controller.beginProjectLoad()
    assertTrue(
        controller.projectLoaded(successfulRequest, project(), ProjectIndex("project", "revision")))
    assertTrue(!controller.state.loading)
    assertEquals(DesktopShellMode.ProjectWorkspace, desktopShellMode(controller.state))
  }

  @Test
  fun summaryToBugsToBoundChatToEditedDraftReviewNeverEscapesTheOpenFile() {
    val controller = loadedController()
    controller.dispatch(DesktopEvent.WorkspaceSelected(Workspace.Summary))
    controller.dispatch(DesktopEvent.WorkspaceSelected(Workspace.Bugs))
    assertEquals(Workspace.Bugs, controller.state.workspace)
    val verified =
        UnifiedFinding(
            confidence = "tool_reported", severity = "high", location = FindingLocation("main.go"))
    val suggested =
        UnifiedFinding(
            confidence = "suggested", severity = "high", location = FindingLocation("main.go"))
    assertEquals(FindingClassification.Verified, classifyFinding(verified))
    assertEquals(FindingClassification.Suggested, classifyFinding(suggested))
    assertEquals(
        FindingPriority.High,
        groupFindingsByPriority(listOf(verified, suggested)).single().priority)
    assertTrue(findingProvenanceLabel(verified).contains("VERIFIED / TOOL-REPORTED"))
    assertTrue(findingProvenanceLabel(suggested).contains("AI SUGGESTIONS"))

    controller.dispatch(DesktopEvent.WorkspaceSelected(Workspace.Editor))
    val load = controller.beginFileLoad("main.go")!!
    assertTrue(controller.fileLoaded(load, file(), listOf(symbol())))
    val fileRequest = controller.currentFileRequest()!!
    val target =
        validateChatTarget(
            controller.state.selectedFile,
            controller.state.symbols,
            symbol(),
            ChatEditMode.ReplaceSymbol,
            "")
    assertTrue(target.valid)
    val session = session()
    val (chatRequest, chatFile) = controller.beginChatLoad()!!
    assertTrue(controller.chatLoaded(chatRequest, chatFile, session))
    val (proposalRequest, proposalFile) = controller.beginChatLoad()!!
    val proposal =
        ChatDraftProposal("session", draft(), ChatSessionMessage("assistant", "Proposal", "draft"))
    assertTrue(
        controller.chatProposalLoaded(
            proposalRequest, proposalFile, session, "Improve Run", proposal))
    assertEquals("main.go", controller.state.review.draft?.targetPath)

    val edited =
        controller.dispatch(
            DesktopEvent.DraftEdited(declaration = "func Run() error { return nil }"))
    assertEquals(DraftEditorStatus.Dirty, edited.review.editor?.status)
    assertFalse(
        draftReviewEligibility(
                edited.review.editor,
                edited.review.draft,
                edited.review.checks,
                edited.selectedFile,
                edited.project)
            .eligible)

    val other = controller.beginFileLoad("other.go")!!
    assertTrue(controller.fileLoaded(other, file("other.go", "other"), emptyList()))
    assertNull(controller.state.chat.session)
    assertNull(controller.state.review.draft)
    assertFalse(
        controller.chatProposalLoaded(proposalRequest, proposalFile, session, "late", proposal))
    assertEquals(fileRequest.path, "main.go")
  }

  @Test
  fun findingNavigationRemainsRevisionAndFileBound() {
    val index =
        ProjectIndex(
            "project", "revision", files = listOf(IndexedFile("main.go", "base", "Go", false)))
    assertEquals(
        EditorNavigationTarget("main.go", "Run", 3),
        findingNavigationTarget(
            UnifiedFinding(location = FindingLocation("main.go", startLine = 3, symbol = "Run")),
            index))
    assertNull(
        findingNavigationTarget(UnifiedFinding(location = FindingLocation("other.go")), index))
  }

  @Test
  fun fileInspectionReturnsToEditorWithoutDiscardingTheOpenFile() {
    val controller = loadedController()
    controller.dispatch(DesktopEvent.WorkspaceSelected(fileInspectionWorkspace()))
    val request = controller.beginFileLoad("main.go")!!
    assertTrue(controller.fileLoaded(request, file(), listOf(symbol())))

    controller.dispatch(DesktopEvent.WorkspaceSelected(Workspace.Analysis))
    assertEquals("main.go", controller.state.selectedFile?.path)
    controller.dispatch(DesktopEvent.WorkspaceSelected(fileInspectionWorkspace()))

    assertEquals(Workspace.Editor, controller.state.workspace)
    assertEquals("main.go", controller.state.selectedFile?.path)
  }

  @Test
  fun directEditSelectionRemainsPreviewOnlyUntilTheExplicitSendAndDraftSteps() {
    val run = symbol()
    val other = run.copy(name = "Other", signature = "func Other()")
    val request = directEditRequest(file(), listOf(run, other), other, currentEditIdentity = null)

    assertEquals(ChatEditMode.ReplaceSymbol, request?.target?.mode)
    assertEquals("Other", request?.target?.symbol)
    assertFalse(request!!.requiresDraftDiscard)
  }

  private fun loadedController() =
      DesktopWorkflowController().also { controller ->
        val projectRequest = controller.beginProjectLoad()
        assertTrue(
            controller.projectLoaded(
                projectRequest, project(), ProjectIndex("project", "revision")))
      }

  private fun project() =
      ProjectAnalysis(
          "project",
          "revision",
          "project",
          "/tmp/project",
          "go",
          fileCount = 2,
          sourceFileCount = 2,
          totalLines = 2,
          summary = "",
          aiStatus = "fresh",
          analyzedAt = "")

  private fun file(path: String = "main.go", hash: String = "base") =
      ProjectFileInfo(
          path,
          hash,
          path,
          language = "Go",
          sizeBytes = 1,
          lineCount = 1,
          modifiedAt = "",
          binary = false)

  private fun symbol() = SymbolInfo("Run", "function", "func Run()", 3, 5, "exact", true)

  private fun session() =
      ChatSession(
          "session", "project", "revision", "base", "main.go", "replace_symbol", "Run", "active")

  private fun draft() =
      DeclarationDraft(
          "draft",
          "project",
          "revision",
          "base",
          "main.go",
          "replace_symbol",
          "Run",
          "func Run() {}",
          revision = 1,
          hash = "draft")
}
