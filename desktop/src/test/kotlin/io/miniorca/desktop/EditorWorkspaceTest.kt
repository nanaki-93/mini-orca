package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class EditorWorkspaceTest {
  @Test
  fun fileHeaderShowsTheBasenameAndProjectRelativePathWithoutProgressCopy() {
    val header = editorFileHeaderUiState(testFile(path = "internal/runner/run.go", name = "run.go"))

    assertEquals("run.go", header.title)
    assertEquals("internal/runner/run.go", header.detail)
    assertTrue(editorFileHeaderDescription(header).contains("run.go"))
    assertFalse(editorFileHeaderDescription(header).contains("progress", ignoreCase = true))
  }

  @Test
  fun fileHeaderUsesANeutralNoFileState() {
    val header = editorFileHeaderUiState(null)

    assertEquals("No file open", header.title)
    assertEquals("No file selected", header.detail)
    assertEquals("No file open. No file selected", editorFileHeaderDescription(header))
  }

  private fun testFile(path: String, name: String) =
      ProjectFileInfo(
          path = path,
          contentHash = "hash",
          name = name,
          language = "Go",
          sizeBytes = 0,
          lineCount = 0,
          modifiedAt = "",
          binary = false,
      )
}
