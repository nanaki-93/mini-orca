package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class DesktopWorkflowControllerTest {
    @Test fun overviewEnrichmentKeepsDeterministicProjectStateAvailable() {
        val controller = loadedController()
        controller.dispatch(DesktopEvent.OverviewLoaded(ProjectOverview(projectId = "project", projectRevision = "revision", metrics = ProjectMetrics(type = "go", fileCount = 1))))

        assertEquals("project", controller.state.project?.projectId)
        assertEquals("go", controller.state.overview?.metrics?.type)
    }
    @Test fun lateFileResponsesCannotReplaceTheNewSelection() {
        val controller = loadedController()
        val first = controller.beginFileLoad("first.go")!!
        assertTrue(controller.fileLoaded(first, file("first.go", "first-hash"), emptyList()))
        val loadedFirst = controller.currentFileRequest()!!
        val second = controller.beginFileLoad("second.go")!!

        assertFalse(controller.analysisLoaded(loadedFirst, FileAnalysis("first.go", "fresh")))
        assertFalse(controller.fileLoaded(first, file("first.go", "first-hash"), emptyList()))
        assertTrue(controller.fileLoaded(second, file("second.go", "second-hash"), emptyList()))

        assertEquals("second.go", controller.state.selectedFile?.path)
        assertNull(controller.state.analysis)
    }

    @Test fun cancellationAndOptionalFailuresLeaveTheLoadedSourceAvailable() {
        val controller = loadedController()
        val canceled = controller.beginFileLoad("canceled.go")!!
        assertTrue(controller.cancelFileLoad(canceled))
        assertFalse(controller.fileLoaded(canceled, file("canceled.go", "canceled-hash"), emptyList()))

        val request = controller.beginFileLoad("main.go")!!
        assertTrue(controller.fileLoaded(request, file("main.go", "main-hash"), listOf(SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true))))
        val loadedRequest = controller.currentFileRequest()!!
        assertTrue(controller.optionalLoadFailed(loadedRequest))

        assertEquals("main.go", controller.state.selectedFile?.path)
        assertEquals("Run", controller.state.symbols.single().name)
        assertNull(controller.state.analysis)
        assertNull(controller.state.impact)
        assertNull(controller.state.gitStatus)
    }

    @Test fun projectSwitchClearsFileBoundChatAndDraft() {
        val controller = loadedController()
        val initialRequest = controller.beginFileLoad("main.go")!!
        assertTrue(controller.fileLoaded(initialRequest, file("main.go", "main-hash"), emptyList()))
        val fileRequest = controller.currentFileRequest()!!
        val (chatRequest, chatFile) = controller.beginChatLoad()!!
        assertTrue(controller.chatLoaded(chatRequest, chatFile, ChatSession("session", "project", "revision", "main-hash", "main.go")))
        val draft = draft()
        val (draftRequest, draftFile) = controller.beginDraftLoad()!!
        assertTrue(controller.draftLoaded(draftRequest, draftFile, draft))

        val projectRequest = controller.beginProjectLoad()
        assertTrue(controller.projectLoaded(projectRequest, project("next", "next-revision"), index("next", "next-revision")))

        assertNull(controller.state.selectedFile)
        assertNull(controller.state.chat.session)
        assertNull(controller.state.review.draft)
        assertFalse(controller.analysisLoaded(fileRequest, FileAnalysis("main.go", "fresh")))
    }

    @Test fun applyEligibilityRequiresTheLatestValidatedAndCheckedDraft() {
        val selected = file("main.go", "main-hash")
        val draft = draft()
        val matchingChecks = CandidateCheckReport(
            targetPath = "main.go", applicable = true, draftId = draft.id, draftRevision = draft.revision,
            draftHash = draft.hash, projectId = draft.projectId, projectRevision = draft.projectRevision, baseFileHash = draft.baseFileHash,
        )

        assertTrue(draftApplyEligibility(draft, matchingChecks, selected).eligible)
        assertFalse(draftApplyEligibility(draft, matchingChecks.copy(draftRevision = 1), selected).eligible)
        assertFalse(draftApplyEligibility(draft.copy(validation = null), matchingChecks, selected).eligible)
        assertFalse(draftApplyEligibility(draft, matchingChecks, file("other.go", "main-hash")).eligible)
    }

    private fun loadedController(): DesktopWorkflowController = DesktopWorkflowController().also { controller ->
        val request = controller.beginProjectLoad()
        assertTrue(controller.projectLoaded(request, project("project", "revision"), index("project", "revision")))
    }

    private fun project(id: String, revision: String) = ProjectAnalysis(id, revision, id, "/tmp/$id", "go", fileCount = 1, sourceFileCount = 1, totalLines = 1, analysisFile = "", summary = "", aiStatus = "missing", analyzedAt = "")
    private fun index(id: String, revision: String) = ProjectIndex(id, revision)
    private fun file(path: String, hash: String) = ProjectFileInfo(path, hash, path, language = "Go", sizeBytes = 1, lineCount = 1, modifiedAt = "", binary = false, content = "package main")
    private fun draft() = DeclarationDraft(id = "draft", projectId = "project", projectRevision = "revision", baseFileHash = "main-hash", targetPath = "main.go", revision = 2, hash = "draft-hash", validation = GenerationValidation(true, "strict_symbol", diff = UnifiedDiff("main.go", "main.go")))
}
