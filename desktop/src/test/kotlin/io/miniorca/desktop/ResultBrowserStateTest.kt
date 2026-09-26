package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertIs
import kotlin.test.assertNull
import kotlin.test.assertSame
import kotlin.test.assertTrue

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
  fun selectionRetainsVisibleKeyAndFallsBackToFirstVisibleRow() {
    val rows = listOf(row("high", "First"), row("critical", "Second"))

    assertEquals("First", resultBrowserSelection(null, rows))
    assertEquals("Second", resultBrowserSelection("Second", rows.reversed()))
    assertEquals("First", resultBrowserSelection("Second", rows.dropLast(1)))
    assertNull(resultBrowserSelection("Second", emptyList()))
    assertEquals("First", nextResultBrowserKey(rows, "Second", 1))
  }

  @Test
  fun savedResultReadChangesDoNotReplaceLocalBrowserState() {
    val page = resultPageFixture("bugs")
    val store = ResultBrowserStore()
    val browser = store.stateFor(page)
    browser.query = "handler"
    browser.filter = ResultBrowserFilter.Value("high")
    browser.selectedKey = "semantic:first"
    val position = browser.listState

    listOf(
            page.copy(section = page.section.copy(loading = true)),
            page.copy(section = page.section.copy(error = "read failed")),
            page.copy(section = page.section.copy(error = "")),
            page.copy(section = page.section.copy(loading = false, error = null)))
        .forEach { updated ->
          assertSame(browser, store.stateFor(updated))
          assertSame(position, store.stateFor(updated).listState)
          assertEquals("handler", browser.query)
          assertEquals(ResultBrowserFilter.Value("high"), browser.filter)
          assertEquals("semantic:first", browser.selectedKey)
        }
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

  private fun snapshot(
      page: AnalysisResultPageState,
      verified: List<UnifiedFinding> = emptyList(),
  ): DesktopState =
      DesktopState(
          projectState = ProjectWorkspaceState(project = page.project),
          analysisRun =
              ProjectAnalysisRunState(
                  run = page.run,
                  sections = mapOf(AnalysisResultKey(page.category) to page.section)),
          findings = FindingsState(findings = verified))

  @Test
  fun explicitActivationRevealsOnlyDestinationAndPreservesOtherBrowser() {
    val page = resultPageFixture("bugs")
    val state = snapshot(page)
    val target = summaryFindingPreview(state).rows.single().target
    val store = ResultBrowserStore()
    val browser = store.stateFor(page)
    val other = store.stateFor(page.copy(type = AnalysisResultType.Security))
    other.query = "keep"
    other.selectedKey = "security:keep"
    browser.query = "no matching title"
    browser.filter = ResultBrowserFilter.Value("critical")
    assertIs<ExplicitResultTarget.Resolved>(store.activate(target, state))
    assertEquals("", browser.query)
    assertEquals(ResultBrowserFilter.All, browser.filter)
    assertEquals(target.rowKey, browser.selectedKey)
    assertEquals("keep", other.query)
    assertEquals("security:keep", other.selectedKey)
    assertSame(other, store.stateFor(page.copy(type = AnalysisResultType.Security)))

    val replaced =
        page.copy(run = page.run!!.copy(identity = page.run.identity.copy(generation = "new")))
    store.resetFor(replaced.project, replaced.run)
    assertNull(store.activate(target, snapshot(replaced)))
    assertNull(store.stateFor(replaced).explicitTarget)
  }

  @Test
  fun replacedRunRejectsQueuedVerifiedClickAndLateReconciliation() {
    val page = resultPageFixture("bugs")
    val verified =
        UnifiedFinding(
            id = "tool-bug",
            projectId = "project",
            projectRevision = "revision",
            category = "bugs",
            confidence = "tool_reported",
            source = "vet",
            location = FindingLocation("tool.go"))
    val previous = snapshot(page, listOf(verified))
    val target =
        summaryFindingPreview(previous)
            .rows
            .single { it.target.producer is SummaryFindingProducer.Verified }
            .target
    val store = ResultBrowserStore()
    store.stateFor(page)
    assertIs<ExplicitResultTarget.Resolved>(store.activate(target, previous))

    val replaced =
        page.copy(run = page.run!!.copy(identity = page.run.identity.copy(generation = "new")))
    val current = snapshot(replaced, listOf(verified))
    store.resetFor(replaced.project, replaced.run)
    val browser = store.stateFor(replaced)
    val currentTarget =
        summaryFindingPreview(current)
            .rows
            .single { it.target.producer is SummaryFindingProducer.Verified }
            .target
    assertIs<ExplicitResultTarget.Resolved>(store.activate(currentTarget, current))
    browser.query = "keep"
    val other = store.stateFor(replaced.copy(type = AnalysisResultType.Security))
    other.filter = ResultBrowserFilter.Value("high")

    assertNull(store.activate(target, current))
    store.reconcileTarget(page, previous)
    store.reconcileTarget(page, current)
    assertSame(browser, store.stateFor(replaced))
    assertSame(other, store.stateFor(replaced.copy(type = AnalysisResultType.Security)))
    assertEquals("keep", browser.query)
    assertEquals(currentTarget.rowKey, browser.selectedKey)
    assertEquals(ExplicitResultTarget.Resolved(currentTarget), browser.explicitTarget)
    assertEquals(ResultBrowserFilter.Value("high"), other.filter)
  }

  @Test
  fun vanishedTargetsRemainUnavailableUntilDismissedOrChosen() {
    val page = resultPageFixture("bugs")
    val state = snapshot(page)
    val target = summaryFindingPreview(state).rows.single().target
    val store = ResultBrowserStore()
    val browser = store.stateFor(page)
    store.activate(target, state)
    val missing = page.copy(section = page.section.copy(results = null))
    val changed = snapshot(missing)
    store.reconcileTarget(missing, changed)
    val unavailable = assertIs<ExplicitResultTarget.Unavailable>(browser.explicitTarget)
    assertTrue("no longer" in unavailable.reason)
    assertNull(browser.selectedKey)
    assertSame(unavailable, store.activate(target, state))
    browser.choose("another")
    assertNull(browser.explicitTarget)
    assertEquals("another", browser.selectedKey)
    store.activate(target, changed)
    browser.dismissTarget()
    assertNull(browser.explicitTarget)
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
