package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

/** Deterministic cross-boundary smoke coverage; no daemon, model, or UI window is required. */
class DesktopIntegrationCoverageTest {
    @Test fun summaryToBugsToBoundChatToEditedDraftReviewNeverEscapesTheOpenFile() {
        val controller = loadedController()
        controller.dispatch(DesktopEvent.WorkspaceSelected(Workspace.Summary))
        controller.dispatch(DesktopEvent.WorkspaceSelected(Workspace.Bugs))
        assertEquals(Workspace.Bugs, controller.state.workspace)
        val verified = UnifiedFinding(confidence = "tool_reported", location = FindingLocation("main.go"))
        val suggested = UnifiedFinding(confidence = "suggested", location = FindingLocation("main.go"))
        assertEquals(FindingClassification.Verified, classifyFinding(verified))
        assertEquals(FindingClassification.Suggested, classifyFinding(suggested))

        controller.dispatch(DesktopEvent.WorkspaceSelected(Workspace.Editor))
        val load = controller.beginFileLoad("main.go")!!
        assertTrue(controller.fileLoaded(load, file(), listOf(symbol())))
        val fileRequest = controller.currentFileRequest()!!
        val target = validateChatTarget(controller.state.selectedFile, controller.state.symbols, symbol(), ChatEditMode.ReplaceSymbol, "")
        assertTrue(target.valid)
        val session = session()
        val (chatRequest, chatFile) = controller.beginChatLoad()!!
        assertTrue(controller.chatLoaded(chatRequest, chatFile, session))
        val (proposalRequest, proposalFile) = controller.beginChatLoad()!!
        val proposal = ChatDraftProposal("session", draft(), ChatSessionMessage("assistant", "Proposal", "draft"))
        assertTrue(controller.chatProposalLoaded(proposalRequest, proposalFile, session, "Improve Run", proposal))
        assertEquals("main.go", controller.state.review.draft?.targetPath)

        val edited = controller.dispatch(DesktopEvent.DraftEdited(declaration = "func Run() error { return nil }"))
        assertEquals(DraftEditorStatus.Dirty, edited.review.editor?.status)
        assertFalse(draftReviewEligibility(edited.review.editor, edited.review.draft, edited.review.checks, edited.selectedFile, edited.project).eligible)

        val other = controller.beginFileLoad("other.go")!!
        assertTrue(controller.fileLoaded(other, file("other.go", "other"), emptyList()))
        assertNull(controller.state.chat.session)
        assertNull(controller.state.review.draft)
        assertFalse(controller.chatProposalLoaded(proposalRequest, proposalFile, session, "late", proposal))
        assertEquals(fileRequest.path, "main.go")
    }

    @Test fun stalePollingAndFindingNavigationRemainRevisionAndFileBound() {
        val polling = AnalyzeAllPollingController()
        polling.activate("revision")
        assertTrue(polling.receive("revision", AnalyzeAllJob(projectRevision = "revision", status = "running")) != null)
        assertTrue(polling.shouldPoll("revision"))
        assertNull(polling.receive("next", AnalyzeAllJob(projectRevision = "next", status = "running")))
        assertFalse(polling.shouldPoll("revision"))

        val index = ProjectIndex("project", "revision", files = listOf(IndexedFile("main.go", "base", "Go", false)))
        assertEquals(EditorNavigationTarget("main.go", "Run", 3), findingNavigationTarget(UnifiedFinding(location = FindingLocation("main.go", startLine = 3, symbol = "Run")), index))
        assertNull(findingNavigationTarget(UnifiedFinding(location = FindingLocation("other.go")), index))
    }

    @Test fun fileInspectionReturnsToEditorWithoutDiscardingTheOpenFile() {
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

    private fun loadedController() = DesktopWorkflowController().also { controller ->
        val projectRequest = controller.beginProjectLoad()
        assertTrue(controller.projectLoaded(projectRequest, project(), ProjectIndex("project", "revision")))
    }

    private fun project() = ProjectAnalysis("project", "revision", "project", "/tmp/project", "go", fileCount = 2, sourceFileCount = 2, totalLines = 2, analysisFile = "", summary = "", aiStatus = "fresh", analyzedAt = "")
    private fun file(path: String = "main.go", hash: String = "base") = ProjectFileInfo(path, hash, path, language = "Go", sizeBytes = 1, lineCount = 1, modifiedAt = "", binary = false)
    private fun symbol() = SymbolInfo("Run", "function", "func Run()", 3, 5, "exact", true)
    private fun session() = ChatSession("session", "project", "revision", "base", "main.go", "replace_symbol", "Run", "active")
    private fun draft() = DeclarationDraft("draft", "project", "revision", "base", "main.go", "replace_symbol", "Run", "func Run() {}", revision = 1, hash = "draft")
}
