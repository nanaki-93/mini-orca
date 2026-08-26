package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

class EditorBriefTest {
    @Test fun deterministicBriefRemainsUsefulWithoutOptionalAnalysis() {
        val brief = editorBriefState(file(), null, null)

        assertNotNull(brief)
        assertEquals("internal/main.go", brief.path)
        assertEquals("Go", brief.language)
        assertEquals(42, brief.lineCount)
        assertEquals("sha256:base", brief.contentHash)
        assertEquals("not analyzed", brief.freshness)
        assertTrue(brief.purpose.isBlank())
    }

    @Test fun symbolSelectionUpdatesEnrichedBriefAndAdvisoryImpact() {
        val symbol = SymbolInfo("Run", "function", "func Run() error", 5, 12, "exact", true)
        val analysis = FileAnalysis(
            path = "internal/main.go",
            status = "fresh",
            purpose = "Runs the daemon.",
            responsibilities = listOf("Start the server"),
            dependencies = listOf("net/http"),
            sideEffects = listOf("Binds loopback port"),
            risks = listOf(Finding("medium", "Request handling needs review")),
            symbolExplanations = mapOf("Run" to "Coordinates startup and shutdown."),
        )

        val brief = editorBriefState(file(), analysis, symbol)

        assertNotNull(brief)
        assertEquals(symbol, brief.selectedSymbol)
        assertEquals("Coordinates startup and shutdown.", brief.symbolExplanation)
        assertEquals(listOf("MEDIUM · Request handling needs review"), brief.advisoryImpact)
        assertEquals(listOf("Start the server"), brief.responsibilities)
    }

    @Test fun failedOptionalAnalysisKeepsTheBriefAndOffersRetryState() {
        val brief = editorBriefState(file(), FileAnalysis("internal/main.go", "failed", failure = "provider unavailable"), null)

        assertNotNull(brief)
        assertEquals("failed", brief.freshness)
        assertEquals("provider unavailable", brief.analysisFailure)

        val controller = DesktopWorkflowController(projectState())
        val request = controller.beginFileLoad("internal/main.go")!!
        assertTrue(controller.fileLoaded(request, file(), emptyList()))
        assertTrue(controller.optionalLoadFailed(controller.currentFileRequest()!!))
        assertEquals("internal/main.go", controller.state.selectedFile?.path)
        assertFalse(controller.state.loading)
    }

    @Test fun responsivePlacementKeepsBriefInTheAvailableEditorPane() {
        assertEquals(EditorBriefPlacement.CompactSourcePane, editorBriefPlacement(999f))
        assertEquals(EditorBriefPlacement.WideActionPane, editorBriefPlacement(1_000f))
    }

    @Test fun sourceLinesEmphasizeTheSelectedSymbolOrFindingLocation() {
        val symbol = SymbolInfo("Run", "function", startLine = 5, endLine = 7, confidence = "exact", atomicTarget = true)

        assertEquals(SourceLineEmphasis.SelectedSymbol, sourceLineEmphasis(6, symbol, 6))
        assertEquals(SourceLineEmphasis.FocusedLocation, sourceLineEmphasis(12, symbol, 12))
        assertEquals(SourceLineEmphasis.None, sourceLineEmphasis(1, symbol, 12))
    }

    private fun file() = ProjectFileInfo(
        path = "internal/main.go",
        contentHash = "sha256:base",
        name = "main.go",
        language = "Go",
        sizeBytes = 512,
        lineCount = 42,
        modifiedAt = "",
        binary = false,
    )

    private fun projectState() = DesktopState(
        projectState = ProjectWorkspaceState(
            ProjectAnalysis("project", "revision", "fixture", "/tmp/fixture", "go", fileCount = 1, sourceFileCount = 1, totalLines = 42, analysisFile = ".mini-orca/analysis.md", summary = "", aiStatus = "fresh", analyzedAt = ""),
            ProjectIndex("project", "revision"),
        ),
    )
}
