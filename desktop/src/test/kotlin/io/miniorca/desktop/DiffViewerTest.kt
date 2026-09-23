package io.miniorca.desktop

import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.key.Key
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
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
  fun unequalReplacementBlocksPairInOrderWithoutLosingBlankOrHunkLines() {
    val lines =
        listOf(
            DiffLine("removed", 4, 0, "first old"),
            DiffLine("removed", 5, 0, "second old"),
            DiffLine("removed", 6, 0, ""),
            DiffLine("added", 0, 4, "first new"),
            DiffLine("added", 0, 5, "second new"),
            DiffLine("hunk", text = "@@ -20 +19 @@"),
            DiffLine("added", 0, 19, "inserted"),
            DiffLine("context", 20, 20, "last"))
    val rows = sideBySideDiffRows(UnifiedDiff("main.go", "main.go", lines))
    assertEquals(
        listOf("first old", "second old", "", "@@ -20 +19 @@", null, "last"),
        rows.map { it.before?.text })
    assertEquals(
        listOf("first new", "second new", null, "@@ -20 +19 @@", "inserted", "last"),
        rows.map { it.proposed?.text })
    assertEquals(6, rows[2].before?.lineNumber)
    assertNull(rows[2].proposed)
    assertNull(rows[3].before?.lineNumber)
    assertEquals(19, rows[4].proposed?.lineNumber)
  }

  @Test
  fun longAndMultilineChangesKeepPairedRowsWithIndependentHorizontalScrolling() {
    val longLine = "return " + "veryLongIdentifier".repeat(30)
    val diff =
        UnifiedDiff(
            "main.go",
            "main.go",
            listOf(
                DiffLine("removed", 99, 0, "$longLine\nsecond current line"),
                DiffLine("removed", 100, 0, "removed without replacement"),
                DiffLine("added", 0, 99, "return newValue")) +
                (101..150).map { DiffLine("context", it, it - 1, "// line $it") })
    ComposeVisualFixture(1000, 500, 1.5f) { DiffViewer(diff) }
        .use { fixture ->
          fixture.render("diff-long-multiline-1000-1.5")
          assertFalse(fixture.hasEditableText())
          assertTrue(
              fixture.copyTextByDragging("$longLine\nsecond current line").startsWith("return"))
          assertEquals(60f, fixture.taggedBounds("diff-Current-row-0").height, 1f)
          assertEquals(60f, fixture.taggedBounds("diff-Candidate-row-0").height, 1f)
          assertEquals(
              fixture.taggedBounds("diff-Current-row-1").top,
              fixture.taggedBounds("diff-Candidate-row-1").top,
              1f)
          assertTrue(fixture.hasDescription("Candidate has no corresponding line"))
          assertFalse(fixture.hasText("Candidate unchanged"))
          fixture.horizontalScrollBy("diff-Current-horizontal", 160f)
          fixture.render()
          assertTrue(fixture.horizontalScrollValue("diff-Current-horizontal") > 0f)
          assertEquals(0f, fixture.horizontalScrollValue("diff-Candidate-horizontal"))
          fixture.scrollBy(200f, "diff-Current-vertical")
          fixture.render("diff-long-scrolled")
          assertTrue(fixture.verticalScrollValue("diff-Current-vertical") > 0f)
          assertEquals(
              fixture.verticalScrollValue("diff-Current-vertical"),
              fixture.verticalScrollValue("diff-Candidate-vertical"))
          assertTrue(fixture.requestFocus("Unified"))
          fixture.render()
          fixture.pressKey(Key.Enter)
          fixture.render("diff-unified-1000-1.5")
          assertTrue(fixture.hasText("Current → Candidate"))
          assertTrue(fixture.hasText("return newValue"))
          assertTrue(fixture.hasText("removed without replacement"))
          assertFalse(fixture.hasEditableText())
          fixture.clickText("Side-by-side")
          fixture.render()
          assertTrue(fixture.hasText("Current"))
          assertTrue(fixture.hasText("Candidate"))
        }
  }

  @Test
  fun diffDefaultsToSideBySideAndUnifiedChoiceSurvivesResize() {
    val diff =
        UnifiedDiff(
            "main.go", "main.go", listOf(DiffLine("added", 0, 12000, "func NewFunction() {}")))
    ComposeVisualFixture(1000, 500, 1.5f) { DiffViewer(diff) }
        .use { fixture ->
          fixture.render("diff-default-1000-1.5")
          assertTrue(fixture.isDescriptionSelected("Side-by-side diff"))
          fixture.resize(400, 400)
          fixture.render("diff-compact-400-1.5")
          assertTrue(fixture.isDescriptionSelected("Side-by-side diff"))
          fixture.assertTextFits("Current")
          fixture.assertTextFits("Candidate")
          fixture.assertTextFits("12000")
          fixture.assertTextLineCount("func NewFunction() {}", 1)
          fixture.clickText("Unified")
          fixture.render("diff-unified-400-1.5")
          assertTrue(fixture.hasText("Current → Candidate"))
          fixture.resize(1000, 500)
          fixture.render("diff-unified-resized-1000-500")
          assertTrue(fixture.hasText("Current → Candidate"))
          assertTrue(fixture.isDescriptionSelected("Unified diff"))
          fixture.clickText("Side-by-side")
          fixture.render("diff-split-resized-1000-500")
          fixture.assertTextFits("Current")
          fixture.assertTextFits("Candidate")
          assertTrue(fixture.hasDescription("Current has no corresponding line"))
          assertEquals("func NewFunction() {}", diff.lines.single().text)
        }
  }
}
