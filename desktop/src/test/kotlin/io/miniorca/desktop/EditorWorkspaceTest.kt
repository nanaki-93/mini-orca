package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class EditorWorkspaceTest {
    @Test fun compactProgressShowsOnlyInspectEditAndReviewWithExplicitCurrentState() {
        assertEquals(
            listOf("Inspect", "Edit", "Review"),
            editorProgressSteps(EditorProgress.Review).map { it.label },
        )
        assertEquals("Current", editorProgressSteps(EditorProgress.Edit)[1].status)
        assertEquals("Complete", editorProgressSteps(EditorProgress.Receipt).last().status)

        val semantics = editorProgressSemanticsLabel(EditorProgressUiState(EditorProgress.Edit, "Editing Run."))

        assertTrue(semantics.contains("Current: Edit"))
        assertTrue(semantics.contains("Inspect: Complete"))
        assertTrue(semantics.contains("Review: Next"))
    }
}
