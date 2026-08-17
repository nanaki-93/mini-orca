package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class PromptTemplatesTest {
    @Test fun templatesRemainFocusedAndGenerateTestRequiresSelectedTestFile() {
        val testTemplate = templateFor("generate-test")

        assertTrue(promptTemplates.all { it.action.isNotBlank() && it.id.isNotBlank() })
        assertFalse(templateAllowed(testTemplate, ProjectFileInfo("main.go", "hash", "main.go", language = "Go", sizeBytes = 1, lineCount = 1, modifiedAt = "", binary = false)))
        assertTrue(templateAllowed(testTemplate, ProjectFileInfo("main_test.go", "hash", "main_test.go", language = "Go", sizeBytes = 1, lineCount = 1, modifiedAt = "", binary = false)))
    }
}
