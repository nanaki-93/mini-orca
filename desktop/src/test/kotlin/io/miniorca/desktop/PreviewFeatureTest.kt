package io.miniorca.desktop

import androidx.compose.ui.unit.dp
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class PreviewFeatureTest {
  @Test
  fun descriptionMakesTheLocalOnlyBoundaryExplicit() {
    val feature = PreviewFeature("Run", "Process execution is not available.")

    val description = previewFeatureDescription(feature)

    assertTrue(description.contains("Run preview"))
    assertTrue(description.contains("Process execution is not available."))
    assertTrue(description.contains("Local only"))
    assertTrue(description.contains("no project, provider, or workflow state changes"))
  }

  @Test
  fun previewFeatureIsOnlyPresentationMetadata() {
    val feature = PreviewFeature("Terminal", "Command execution is not available.")

    assertEquals("Terminal", feature.label)
    assertFalse(feature.description.contains("http", ignoreCase = true))
  }

  @Test
  fun terminalPreviewIsExplicitlyInert() {
    val feature = PreviewFeature("Terminal", "Command execution is not available.")

    assertTrue(previewFeatureDescription(feature).contains("Local only"))
    assertTrue(
        previewFeatureDescription(feature)
            .contains("no project, provider, or workflow state changes"))
  }

  @Test
  fun previewMenuUsesSharedBoundedRowsAndFeatureIcons() {
    assertEquals(196.dp, IdePopupMenuDefaults.minWidth)
    assertEquals(360.dp, IdePopupMenuDefaults.maxWidth)
    assertEquals(360.dp, IdePopupMenuDefaults.maxHeight)
    assertEquals(32.dp, IdePopupMenuDefaults.rowMinimumHeight)
    assertEquals(DesktopIcon.Search, previewFeatureIcon(PreviewFeature("Content search", "")))
    assertEquals(DesktopIcon.Run, previewFeatureIcon(PreviewFeature("Run / Debug", "")))
  }
}
