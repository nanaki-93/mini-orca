package io.miniorca.desktop

import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class DiffViewerTest {
  @Test
  fun pairsReplacementAndRetainsContextAndInsertions() {
    val diff =
        UnifiedDiff(
            "main.go",
            "main.go",
            listOf(
                DiffLine("context", 1, 1, "package main"),
                DiffLine("removed", 2, 0, "func Run() {}"),
                DiffLine("added", 0, 2, "func Run() error { return nil }"),
                DiffLine("added", 0, 3, "")))
    val rows = sideBySideDiffRows(diff)
    assertEquals(3, rows.size)
    assertEquals("func Run() {}", rows[1].before?.text)
    assertEquals("func Run() error { return nil }", rows[1].proposed?.text)
    assertEquals("added", rows[2].proposed?.change)
  }

  @Test
  fun diffBackgroundsKeepAddedAndRemovedLinesDistinct() {
    assertEquals(DiffAddedBackground, diffLineBackground("added"))
    assertEquals(DiffRemovedBackground, diffLineBackground("removed"))
    assertEquals(Color.Transparent, diffLineBackground("context"))
  }

  @Test
  fun diffMarkersAndStatusTextRemainReadableWithoutChangingSourceRows() {
    val diff =
        UnifiedDiff(
            "main.go",
            "main.go",
            listOf(
                DiffLine("removed", 4, 0, "return oldValue"),
                DiffLine("added", 0, 4, "return newValue")))

    val row = sideBySideDiffRows(diff).single()

    assertEquals("removed", row.before?.change)
    assertEquals("added", row.proposed?.change)
    assertEquals("return oldValue", row.before?.text)
    assertEquals("return newValue", row.proposed?.text)
    assertTrue(contrastRatio(Error, diffLineBackground(row.before?.change)) >= 4.5)
    assertTrue(contrastRatio(Success, diffLineBackground(row.proposed?.change)) >= 4.5)
  }

  @Test
  fun diffRowsUseTheSameMinimumLineHeightAsTheReadOnlySourceGutter() {
    assertEquals(20.dp, readOnlyCodeRowMinimumHeight)
  }
}
