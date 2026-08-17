package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

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

    private fun sampleCandidate() = GenerationResult("g", "p", "r", "base", "main.go", "Run", "strict_symbol", "", "hash", GenerationValidation(true, "strict_symbol", diff = UnifiedDiff("main.go", "main.go")), ContextManifest())
}
