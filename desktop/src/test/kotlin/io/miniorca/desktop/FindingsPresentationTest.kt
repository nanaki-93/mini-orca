package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class FindingsPresentationTest {
  private val highOpen =
      UnifiedFinding(
          id = "high-open",
          source = "vet",
          confidence = "tool_reported",
          severity = "high",
          title = "Unchecked error",
          message = "Handle the returned error.",
          status = "open",
          freshness = "fresh",
          location = FindingLocation("internal/main.go", startLine = 12, symbol = "Run"),
          taskSpec =
              BugTaskSpec(
                  schemaVersion = "1",
                  targetPath = "internal/main.go",
                  targetSymbol = "Run",
                  targetSignature = "func Run() error",
                  acceptanceCriteria = listOf("Return the error.")),
      )

  @Test
  fun resultHeaderQualifiesReportedCountsUntilMatchingDetailsAreConfirmed() {
    val original = resultPageFixture("bugs")
    val progress = original.progress!!.copy(status = "completed_empty", findingCount = 0)
    val run =
        original.run!!.copy(
            status = "completed_empty",
            sections = original.run.sections.map { if (it.category == "bugs") progress else it })
    val page =
        original.copy(
            run = run,
            section =
                AnalysisSectionState(
                    results = original.results!!.copy(progress = progress, semantic = emptyList())))
    val cases =
        listOf(
            Triple(page, "0 findings", "No results"),
            Triple(
                page.copy(section = AnalysisSectionState()),
                "0 reported",
                "Completed · details unconfirmed"),
            Triple(
                page.copy(section = AnalysisSectionState(error = "")),
                "0 reported",
                "Completed · details unconfirmed"),
            Triple(
                page.copy(
                    run =
                        run.copy(
                            sections =
                                run.sections.map {
                                  if (it.category == "bugs") it.copy(findingCount = null) else it
                                })),
                "— reported (count unavailable)",
                "Completed · details unconfirmed"),
            Triple(
                page.copy(run = run.copy(status = "partial")),
                "0 reported",
                "Completed · details unconfirmed"))
    cases.forEach { (state, count, status) ->
      var navigations = 0
      ComposeVisualFixture(800, 650, 1.5f) { ResultSectionHeader(state, 0) { navigations++ } }
          .use { fixture ->
            fixture.render()
            fixture.assertTextFits(count)
            fixture.assertTextFits(status)
            assertEquals(0, navigations)
          }
    }
  }

  @Test
  fun retainedRowsDoNotConvertUnknownOrFailedCountsToUnqualifiedFindings() {
    val page = resultPageFixture("bugs")
    val unknown =
        page.copy(
            run =
                page.run!!.copy(sections = page.run.sections.map { it.copy(findingCount = null) }))
    val failed = page.copy(section = page.section.copy(error = "refresh failed"))
    val blankError = page.copy(section = page.section.copy(error = ""))
    listOf(
            unknown to "1 loaded · — reported (count unavailable)",
            failed to "1 loaded · 1 reported",
            blankError to "1 loaded · 1 reported",
            page to "1 loaded · 1 reported")
        .forEach { (state, label) ->
          ComposeVisualFixture(800, 650) { ResultSectionHeader(state, 1) {} }
              .use { fixture ->
                fixture.render()
                fixture.assertTextFits(label)
                if (state.section.error != null)
                    fixture.assertTextFits(
                        "Results could not be refreshed: ${state.section.error.ifBlank { "The saved result read failed without a diagnostic." }}")
              }
        }
    val completed =
        page.run.copy(
            status = "completed",
            sections = page.run.sections.map { it.copy(status = "completed") })
    val confirmed =
        page.copy(
            run = completed,
            section =
                AnalysisSectionState(
                    results =
                        page.results!!.copy(
                            progress = completed.sections.first { it.category == "bugs" })))
    ComposeVisualFixture(800, 650) { ResultSectionHeader(confirmed, 1) {} }
        .use { fixture ->
          fixture.render()
          fixture.assertTextFits("1 finding")
          assertFalse(fixture.hasText("1 loaded · 1 reported"))
        }
  }

  @Test
  fun fixPreparationUsesTheSelectedFindingAndItsExactTarget() {
    var prepared: UnifiedFinding? = null
    val actions =
        FindingActions(
            prepareFinding = { prepared = it },
            triageFinding = { _, _ -> },
        )
    val index =
        ProjectIndex(
            "project",
            "revision",
            files = listOf(IndexedFile("internal/main.go", "hash", "Go", false)))

    actions.prepareFix(highOpen)

    assertEquals(highOpen, prepared)
    assertEquals(
        EditorNavigationTarget("internal/main.go", "Run", 12),
        findingNavigationTarget(highOpen, index))
  }

  @Test
  fun routineProvenanceAndOpenFreshLabelsStayOutOfRows() {
    val current = semanticResultRow(highOpen)
    val stale = semanticResultRow(highOpen.copy(freshness = "stale", status = "partial"))

    assertEquals("", current.source)
    assertEquals("", current.state)
    assertEquals("Handle the returned error.", current.summary)
    assertEquals("Partial · Stale", stale.state)
    assertTrue(current.location.contains("internal/main.go:12"))
  }

  @Test
  fun detailIdentifiesEvidenceBeforeGuardedPrepareAndSecondaryTriageActions() {
    var prepared: UnifiedFinding? = null
    var triaged: FindingLifecycleAction? = null
    ComposeVisualFixture(900, 500) {
          FindingDetailsRegion(
              highOpen,
              FindingActions(
                  prepareFinding = { prepared = it },
                  triageFinding = { _, action -> triaged = action }))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Tool report · vet"))
          fixture.clickText("Prepare fix")
          fixture.clickText("Dismiss")
          assertEquals(highOpen, prepared)
          assertEquals(FindingLifecycleAction("Dismiss", "dismissed"), triaged)
        }

    ComposeVisualFixture(900, 500) {
          FindingDetailsRegion(highOpen.copy(freshness = "stale"), FindingActions({}, { _, _ -> }))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Tool report · vet"))
          assertTrue(fixture.isDisabled("Prepare fix"))
          assertFalse(fixture.hasText("Apply fix"))
        }
  }
}
