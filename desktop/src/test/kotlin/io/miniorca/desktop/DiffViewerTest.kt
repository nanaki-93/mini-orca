package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals

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
}
