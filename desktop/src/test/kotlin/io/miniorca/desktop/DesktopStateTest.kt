package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json

class DesktopStateTest {
    @Test fun projectLoadClearsPriorSelectionAndCandidate() {
        val project = ProjectAnalysis("id", "revision", "fixture", "/tmp/fixture", "go", fileCount = 1, sourceFileCount = 1, totalLines = 2, analysisFile = ".mini-orca/analysis.md", summary = "", aiStatus = "fresh", analyzedAt = "")
        val index = ProjectIndex("id", "revision")
        val state = DesktopState(
            selection = FileSelectionState(selectedFile = ProjectFileInfo("main.go", "hash", "main.go", language = "Go", sizeBytes = 1, lineCount = 1, modifiedAt = "", binary = false)),
            review = DraftReviewState(candidate = sampleCandidate()),
        ).reduce(DesktopEvent.ProjectLoaded(project, index))
        assertNull(state.selectedFile)
        assertNull(state.candidate)
        assertEquals(project, state.project)
    }

    @Test fun apiClientUsesTypedTransportAndErrorMessages() {
        val client = ApiClient(transport = DaemonTransport { method, path, _ ->
            assertEquals("GET", method)
            assertEquals("/api/projects/current/index", path)
            TransportResponse(200, "{\"project_id\":\"p\",\"project_revision\":\"r\",\"files\":[]}")
        })
        assertEquals("p", client.index().projectId)
        val failed = ApiClient(transport = DaemonTransport { _, _, _ -> TransportResponse(409, "{\"message\":\"stale\",\"user_message\":\"Reload first\"}") })
        val error = runCatching { failed.index() }.exceptionOrNull()
        assertTrue(error is ApiException && error.message == "Reload first")
    }

    @Test fun apiClientReadsDaemonCanonicalVersion() {
        val client = ApiClient(transport = DaemonTransport { method, path, _ ->
            assertEquals("GET", method)
            assertEquals("/status", path)
            TransportResponse(200, "{\"status\":\"running\",\"version\":\"4.3.0\",\"workflow\":\"single_coder_preview\"}")
        })

        assertEquals("4.3.0", client.status().version)
    }

    @Test fun indexAcceptsLegacyNullCollectionFields() {
        val client = ApiClient(transport = DaemonTransport { method, path, _ ->
            assertEquals("GET", method)
            assertEquals("/api/projects/current/index", path)
            TransportResponse(200, """{"project_id":"p","project_revision":"r","files":[{"path":"main.go","content_hash":"hash","language":"Go","binary":false,"symbols":null}]}""")
        })

        val index = client.index()

        assertEquals(emptyList(), index.files.single().symbols)
    }

    @Test fun reindexKeepsSelectedFileAndClearsLoadingState() {
        val selected = ProjectFileInfo("main.go", "hash", "main.go", language = "Go", sizeBytes = 1, lineCount = 1, modifiedAt = "", binary = false)
        val refreshed = ProjectIndex("id", "new-revision")
        val state = DesktopState(selection = FileSelectionState(selectedFile = selected), jobs = JobState(loading = true)).reduce(DesktopEvent.IndexRefreshed(refreshed))
        assertEquals(selected, state.selectedFile)
        assertEquals(refreshed, state.index)
        assertTrue(!state.loading)
    }

    @Test fun suggestionPreparesButDoesNotGenerateARequest() {
        val symbol = SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true)
        val state = DesktopState(review = DraftReviewState(candidate = sampleCandidate())).reduce(DesktopEvent.SuggestionPrepared("fix", "Handle empty input", symbol))
        assertEquals(symbol, state.selectedSymbol)
        assertEquals("fix", state.preparedAction)
        assertEquals("Handle empty input", state.preparedRequest)
        assertEquals("g", state.candidate?.generationId)
    }

    @Test fun explainSymbolRemainsAReadOnlyPreparedAction() {
        val symbol = SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true)

        val state = DesktopState().reduce(
            DesktopEvent.SuggestionPrepared("explain_symbol", "Show the cached explanation for Run.", symbol),
        )

        assertEquals(symbol, state.selectedSymbol)
        assertEquals("explain_symbol", state.preparedAction)
        assertEquals("Show the cached explanation for Run.", state.preparedRequest)
        assertNull(state.candidate)
    }

    @Test fun discardingPreviewAlsoClearsItsChecks() {
        val state = DesktopState(review = DraftReviewState(candidate = sampleCandidate(), checks = CandidateCheckReport("main.go", true))).reduce(DesktopEvent.CandidateDiscarded)

        assertNull(state.candidate)
        assertNull(state.checks)
        assertEquals("Discarded preview", state.status)
    }

    @Test fun fileSwitchRejectsLateEnrichmentAndClearsFileBoundState() {
        val controller = DesktopWorkflowController(projectState())
        val first = controller.beginFileLoad("first.go")!!
        assertTrue(controller.fileLoaded(first, file("first.go", "first"), emptyList()))
        val loadedFirst = controller.currentFileRequest()!!
        assertTrue(controller.chatLoaded(controller.beginChatLoad()!!.first, loadedFirst, session("first.go", "first")))

        val second = controller.beginFileLoad("second.go")!!
        assertNull(controller.state.selectedFile)
        assertNull(controller.state.chat.session)
        assertNull(controller.state.review.draft)
        assertTrue(!controller.analysisLoaded(loadedFirst, FileAnalysis("first.go", "fresh")))
        assertTrue(controller.fileLoaded(second, file("second.go", "second"), emptyList()))
        assertEquals("second.go", controller.state.selectedFile?.path)
    }

    @Test fun projectAndCanceledFileRequestsRejectStaleResponses() {
        val controller = DesktopWorkflowController()
        val firstProjectRequest = controller.beginProjectLoad()
        val secondProjectRequest = controller.beginProjectLoad()
        val project = project()
        val index = ProjectIndex("project", "revision")

        assertTrue(!controller.projectLoaded(firstProjectRequest, project, index))
        assertTrue(controller.projectLoaded(secondProjectRequest, project, index))
        val fileRequest = controller.beginFileLoad("main.go")!!
        assertTrue(controller.cancelFileLoad(fileRequest))
        assertTrue(!controller.fileLoaded(fileRequest, file("main.go", "hash"), emptyList()))
    }

    @Test fun draftEligibilityRequiresMatchingLatestValidationAndChecks() {
        val selected = file("main.go", "base")
        val draft = DeclarationDraft(id = "draft", baseFileHash = "base", targetPath = "main.go", revision = 2, hash = "latest", validation = GenerationValidation(true, "replace_symbol", diff = UnifiedDiff("main.go", "main.go")))
        val staleChecks = CandidateCheckReport("main.go", true, draftId = "draft", draftRevision = 1, draftHash = "old")
        val currentChecks = CandidateCheckReport("main.go", true, draftId = "draft", draftRevision = 2, draftHash = "latest")

        assertTrue(!draftApplyEligibility(draft, staleChecks, selected).eligible)
        assertTrue(draftApplyEligibility(draft, currentChecks, selected).eligible)
    }

    @Test fun workspaceSwitchKeepsTheOpenFileAndDirtyDraft() {
        val selected = file("main.go", "base")
        val draft = DeclarationDraft(id = "draft", targetPath = "main.go", baseFileHash = "base", declaration = "func Run() {}", revision = 3)
        val initial = DesktopState(
            workspace = Workspace.Editor,
            selection = FileSelectionState(selectedFile = selected),
            review = DraftReviewState(draft = draft),
        )

        val switched = initial.reduce(DesktopEvent.WorkspaceSelected(Workspace.Bugs))

        assertEquals(Workspace.Bugs, switched.workspace)
        assertEquals(selected, switched.selectedFile)
        assertEquals(draft, switched.review.draft)
    }

    @Test fun findingNavigationOnlyUsesTheActiveProjectIndexAndKeepsItsContext() {
        val index = ProjectIndex("project", "revision", files = listOf(IndexedFile("internal/main.go", "hash", "Go", false)))
        val finding = UnifiedFinding(location = FindingLocation("internal/main.go", startLine = 7, symbol = "Run"))

        assertEquals(EditorNavigationTarget("internal/main.go", "Run", 7), findingNavigationTarget(finding, index))
        assertNull(findingNavigationTarget(finding.copy(location = FindingLocation("../outside.go", startLine = 7)), index))
    }

    @Test fun findingNavigationPrefersExactSymbolThenLineRange() {
        val symbols = listOf(
            SymbolInfo("Other", "function", startLine = 1, endLine = 3, confidence = "exact", atomicTarget = true),
            SymbolInfo("Run", "function", startLine = 10, endLine = 15, confidence = "exact", atomicTarget = true),
        )

        assertEquals(symbols[1], symbolForNavigation(symbols, EditorNavigationTarget("main.go", "Run", 2)))
        assertEquals(symbols[1], symbolForNavigation(symbols, EditorNavigationTarget("main.go", line = 12)))
    }

    @Test fun contextInspectorClientKeepsOnlySourceFreeManifestMetadata() {
        val client = ApiClient("https://provider.example", DaemonTransport { _, _, _ ->
            TransportResponse(200, """{"included":[{"path":"main.go","size_bytes":20,"hash":"sha256:base","estimated_tokens":5}],"excluded":[{"path":".env","include":false,"reason":"secret"}],"estimated_tokens":5,"byte_limit":1024,"token_limit":256,"content":"private source must not reach the UI model"}""")
        })

        val manifest = client.context("main.go")

        assertEquals("Remote endpoint", client.endpointLocality())
        assertEquals("main.go", manifest.included.single().path)
        assertEquals("secret", manifest.excluded.single().reason)
        assertTrue(!Json.encodeToString(manifest).contains("private source"))
    }

    private fun sampleCandidate() = GenerationResult("g", "p", "r", "base", "main.go", "Run", "strict_symbol", "", "hash", GenerationValidation(true, "strict_symbol", diff = UnifiedDiff("main.go", "main.go")), ContextManifest())

    private fun project() = ProjectAnalysis("project", "revision", "fixture", "/tmp/fixture", "go", fileCount = 1, sourceFileCount = 1, totalLines = 2, analysisFile = ".mini-orca/analysis.md", summary = "", aiStatus = "fresh", analyzedAt = "")
    private fun projectState() = DesktopState(projectState = ProjectWorkspaceState(project(), ProjectIndex("project", "revision")))
    private fun file(path: String, hash: String) = ProjectFileInfo(path, hash, path, language = "Go", sizeBytes = 1, lineCount = 1, modifiedAt = "", binary = false)
    private fun session(path: String, hash: String) = ChatSession(id = "session", projectId = "project", projectRevision = "revision", baseFileHash = hash, openPath = path)
}
