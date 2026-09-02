package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class EditorInspectionStateTest {

    @Test fun sourceLinesKeepDirectSelectableSemanticsWithoutAnInstructionalSubtitle() {
        val symbol = SymbolInfo("Run", "function", startLine = 5, endLine = 7, confidence = "exact", atomicTarget = true)

        assertEquals(SourceLineEmphasis.FocusedSelectedSymbol, sourceLineEmphasis(6, symbol, 6))
        assertEquals(SourceLineEmphasis.FocusedLocation, sourceLineEmphasis(12, symbol, 12))
        assertEquals(SourceLineEmphasis.None, sourceLineEmphasis(1, symbol, 12))
        assertEquals("Line 6, focused location in selected declaration", sourceLineDescription(6, SourceLineEmphasis.FocusedSelectedSymbol))
        assertEquals(
            "Line 5, selected declaration, selectable declaration Run",
            sourceLineContentDescription(5, SourceLineEmphasis.SelectedSymbol, symbol),
        )
        assertFalse(sourceLineContentDescription(5, SourceLineEmphasis.SelectedSymbol, symbol).contains("Click"))
        assertTrue(symbolAtLine(listOf(symbol), 6) != null)
        assertFalse(symbolAtLine(listOf(symbol), 12) != null)
        assertEquals(SourceLineSelection(6, symbol), sourceLineSelection(listOf(symbol), 6))
        assertEquals(SourceLineSelection(12, null), sourceLineSelection(listOf(symbol), 12))
        assertTrue(sourceTapSelectsContext(dragged = false))
        assertFalse(sourceTapSelectsContext(dragged = true))
    }

    @Test fun inspectorShowsOneSelectedSymbolAndOneAnalysisActionAtATime() {
        val selected = SymbolInfo("Run", "function", "func Run() error", 5, 12, "exact", true)
        val remoteUnconfirmed = InspectorProviderState(remoteProvider = true, remoteProviderConfirmed = false)
        val expectedActions = listOf(
            null to InspectorAnalysisAction.AnalyzeFile,
            FileAnalysis("internal/main.go", "failed") to InspectorAnalysisAction.AnalyzeFile,
            FileAnalysis("internal/main.go", "stale") to InspectorAnalysisAction.RefreshAnalysis,
            FileAnalysis("internal/main.go", "running") to InspectorAnalysisAction.CancelAnalysis,
            FileAnalysis("internal/main.go", "fresh") to InspectorAnalysisAction.None,
        )

        expectedActions.forEach { (analysis, action) ->
            val inspector = symbolInspectorUiState(
                selectedFile = file(),
                symbols = listOf(selected),
                selectedSymbol = selected,
                analysis = analysis?.copy(symbolExplanations = mapOf("Run" to "Runs the daemon.")),
                analysisInProgress = false,
                provider = remoteUnconfirmed,
                currentEditIdentity = null,
            )!!

            assertEquals(SymbolInspectorMode.SelectedSymbol, inspector.mode)
            assertEquals(action, inspector.analysisAction)
            assertEquals(if (analysis == null) null else "Runs the daemon.", inspector.selectedSymbol?.explanation)
            assertEquals(action != InspectorAnalysisAction.CancelAnalysis, inspector.remoteProviderConfirmationRequired)
        }

        val fallback = symbolInspectorUiState(file(), listOf(selected), null, FileAnalysis("internal/main.go", "fresh"), false, remoteUnconfirmed, null)!!
        assertEquals(SymbolInspectorMode.FileFallback, fallback.mode)
        assertEquals(InspectorAnalysisAction.None, fallback.analysisAction)
    }

    @Test fun inspectorEditEligibilityAndCurrentDraftIdentityStaySeparate() {
        val selected = SymbolInfo("Run", "function", "func Run()", 5, 12, "exact", true)
        val draft = DeclarationDraft(
            id = "draft",
            projectId = "project",
            projectRevision = "revision",
            baseFileHash = "sha256:base",
            targetPath = "internal/main.go",
            mode = ChatEditMode.ReplaceSymbol.wireValue,
            targetSymbol = "Other",
        )
        val identity = currentEditIdentity(null, draft, file(), project())
        val inspector = symbolInspectorUiState(
            file(),
            listOf(selected),
            selected,
            FileAnalysis("internal/main.go", "fresh"),
            analysisInProgress = false,
            provider = InspectorProviderState(remoteProvider = false, remoteProviderConfirmed = false),
            currentEditIdentity = identity,
        )!!

        assertTrue(inspector.selectedSymbol!!.editEligibility.eligible)
        assertTrue(inspector.selectedSymbol.currentDraftBoundToAnotherTarget)
        assertEquals("Other", inspector.currentEditIdentity?.targetSymbol)
        assertFalse(symbolEditEligibility(file().copy(language = "Kotlin"), listOf(selected), selected).eligible)
        val nonAtomic = selected.copy(atomicTarget = false)
        assertFalse(symbolEditEligibility(file(), listOf(nonAtomic), nonAtomic).eligible)
        val unsupportedKind = selected.copy(kind = "variable")
        assertFalse(symbolEditEligibility(file(), listOf(unsupportedKind), unsupportedKind).eligible)
        assertFalse(symbolEditEligibility(file(), listOf(selected.copy(kind = "var")), selected.copy(kind = "var")).eligible)
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

    private fun project() = ProjectAnalysis(
        "project",
        "revision",
        "fixture",
        "/tmp/fixture",
        "go",
        fileCount = 1,
        sourceFileCount = 1,
        totalLines = 42,
        analysisFile = ".mini-orca/analysis.md",
        summary = "",
        aiStatus = "fresh",
        analyzedAt = "",
    )
}
