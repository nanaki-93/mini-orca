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
        val state = DesktopState(selectedFile = ProjectFileInfo("main.go", "hash", "main.go", language = "Go", sizeBytes = 1, lineCount = 1, modifiedAt = "", binary = false), candidate = sampleCandidate()).reduce(DesktopEvent.ProjectLoaded(project, index))
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
            TransportResponse(200, "{\"status\":\"running\",\"version\":\"4.1.0\",\"workflow\":\"single_coder_preview\"}")
        })

        assertEquals("4.1.0", client.status().version)
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
        val state = DesktopState(selectedFile = selected, loading = true).reduce(DesktopEvent.IndexRefreshed(refreshed))
        assertEquals(selected, state.selectedFile)
        assertEquals(refreshed, state.index)
        assertTrue(!state.loading)
    }

    @Test fun suggestionPreparesButDoesNotGenerateARequest() {
        val symbol = SymbolInfo("Run", "function", confidence = "exact", atomicTarget = true)
        val state = DesktopState(candidate = sampleCandidate()).reduce(DesktopEvent.SuggestionPrepared("fix", "Handle empty input", symbol))
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
        val state = DesktopState(candidate = sampleCandidate(), checks = CandidateCheckReport("main.go", true)).reduce(DesktopEvent.CandidateDiscarded)

        assertNull(state.candidate)
        assertNull(state.checks)
        assertEquals("Discarded preview", state.status)
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
}
