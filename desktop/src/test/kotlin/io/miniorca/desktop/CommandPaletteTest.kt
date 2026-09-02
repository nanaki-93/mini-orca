package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class CommandPaletteTest {
    @Test fun freshFileAnalysisCanBeRefreshedFromCommandsWithoutAddingASecondInspectorAction() {
        val freshActions = availableCommandActions(FileAnalysis("main.go", "fresh"))

        assertTrue("refresh_file_analysis" in freshActions)
        assertTrue("create_declaration" in freshActions)
        assertFalse("refresh_file_analysis" in availableCommandActions(FileAnalysis("main.go", "stale")))
        assertEquals("Refresh file analysis", commandActionLabel("refresh_file_analysis"))
        assertEquals("Create declaration", commandActionLabel("create_declaration"))
    }
}
