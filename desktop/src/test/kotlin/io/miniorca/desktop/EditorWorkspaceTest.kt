package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class EditorWorkspaceTest {
    @Test fun stageLabelsAndSemanticsExposeCurrentReadyAndLockedStates() {
        val current = EditorStageUiState(EditorStage.Draft, unlocked = true, reason = "Draft is ready for the first message bound to this target.")
        val locked = EditorStageUiState(EditorStage.Apply, unlocked = false, reason = "Run focused checks for the latest draft before applying it.")

        assertEquals("Draft · Current", editorStageLabel(current, current = true))
        assertEquals("Draft · Ready", editorStageLabel(current, current = false))
        assertEquals("Apply · Locked", editorStageLabel(locked, current = false))
        assertTrue(editorStageSemanticsLabel(locked, current = false).contains("Run focused checks"))
    }
}
