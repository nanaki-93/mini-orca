package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class EditorWorkspaceTest {
    @Test fun stageLabelsAndSemanticsExposeCurrentReadyAndLockedStates() {
        val current = EditorStageUiState(EditorStage.Draft, unlocked = true, reason = "Draft is ready for the first message bound to this target.")
        val locked = EditorStageUiState(EditorStage.Apply, unlocked = false, reason = "Run focused checks for the latest draft before applying it.")

        assertEquals("Draft", editorStageLabel(current, current = true))
        assertEquals("Draft", editorStageLabel(current, current = false))
        assertEquals("Apply", editorStageLabel(locked, current = false))
        assertEquals("Current", editorStageStateLabel(current, current = true))
        assertEquals("Ready", editorStageStateLabel(current, current = false))
        assertEquals("Locked", editorStageStateLabel(locked, current = false))
        assertTrue(editorStageSemanticsLabel(locked, current = false).contains("Apply, Locked"))
        assertTrue(editorStageSemanticsLabel(locked, current = false).contains("Run focused checks"))
    }

    @Test fun editorSurfacesExposeTheSourceAndFileAnalysisViews() {
        assertEquals("Source / diff · Current", editorSurfaceLabel(EditorSurface.Source, current = true))
        assertEquals("File analysis", editorSurfaceLabel(EditorSurface.FileAnalysis, current = false))
        assertTrue(editorSurfaceSemanticsLabel(EditorSurface.FileAnalysis, current = true).contains("Current"))
    }
}
