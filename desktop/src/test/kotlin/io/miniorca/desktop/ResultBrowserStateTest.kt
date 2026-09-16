package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

class ResultBrowserStateTest {
  @Test
  fun facetsUseLoadedRowsAndKeepCriticalAndUnknownDistinct() {
    val rows = listOf(row("critical", "Critical"), row("high", "High"), row("", "Unknown"))

    assertEquals(
        listOf("Critical" to 1, "High" to 1, "Unknown" to 1),
        resultBrowserFacets(rows).map { it.label to it.count },
    )
    assertEquals(
        listOf("Critical"),
        filteredResultRows(rows, ResultBrowserFilter.Value("critical"), "").map { it.title },
    )
    assertEquals(
        listOf("Unknown"),
        filteredResultRows(rows, ResultBrowserFilter.Value("unknown"), "").map { it.title },
    )
    assertEquals(
        listOf("High"),
        filteredResultRows(rows, ResultBrowserFilter.All, "handler.go").map { it.title },
    )
  }

  @Test
  fun selectionUsesStableKeysAndOnlyWidePagesAutoSelect() {
    val rows = listOf(row("high", "First"), row("critical", "Second"))

    assertEquals("First", resultBrowserSelection(null, rows, wide = true))
    assertNull(resultBrowserSelection(null, rows, wide = false))
    assertEquals("Second", resultBrowserSelection("Second", rows.reversed(), wide = true))
    assertEquals("First", resultBrowserSelection("Second", rows.dropLast(1), wide = true))
    assertNull(resultBrowserSelection("Second", rows.dropLast(1), wide = false))
    assertEquals("First", nextResultBrowserKey(rows, "Second", 1))
  }

  @Test
  fun storeClearsStateWhenProjectOrRunScopeIsReplaced() {
    val page = resultPageFixture("bugs")
    val project = requireNotNull(page.project)
    val run = requireNotNull(page.run)
    val store = ResultBrowserStore()
    val first = store.stateFor(page)
    first.filter = ResultBrowserFilter.Value("high")
    first.query = "retained"
    first.selectedKey = "semantic:first"

    assertEquals("", store.stateFor(page.copy(type = AnalysisResultType.Performance)).query)
    assertEquals(first, store.stateFor(page))

    val changedProject = page.copy(project = project.copy(projectId = "other"))
    val afterProjectChange = store.stateFor(changedProject)
    assertEquals("", afterProjectChange.query)
    assertNull(afterProjectChange.selectedKey)
    assertEquals("", store.stateFor(page).query)

    val changedRun = page.copy(run = run.copy(identity = run.identity.copy(generation = "next")))
    val afterRunChange = store.stateFor(changedRun)
    afterRunChange.query = "new run"
    assertEquals("", store.stateFor(page).query)

    val changedRevision =
        page.copy(
            project = project.copy(projectRevision = "next-revision"),
            run = run.copy(identity = run.identity.copy(projectRevision = "next-revision")),
        )
    val afterRevisionChange = store.stateFor(changedRevision)
    assertEquals(ResultBrowserFilter.All, afterRevisionChange.filter)
    assertNull(afterRevisionChange.selectedKey)

    afterRevisionChange.query = "discard when results are absent"
    store.resetFor(null, null)
    val afterAbsentPage = store.stateFor(page)
    assertEquals(ResultBrowserFilter.All, afterAbsentPage.filter)
    assertEquals("", afterAbsentPage.query)
    assertNull(afterAbsentPage.selectedKey)
    assertEquals(0, afterAbsentPage.listState.firstVisibleItemIndex)
    assertEquals(0, afterAbsentPage.listState.firstVisibleItemScrollOffset)
  }

  private fun row(severity: String, title: String) =
      ResultRowPresentation(
          key = title,
          title = title,
          location = if (title == "High") "internal/handler.go:8" else "internal/other.go:4",
          summary = "",
          severity = severity,
          source = "",
          state = "",
      )
}
