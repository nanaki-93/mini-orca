package io.miniorca.desktop

import androidx.compose.runtime.getValue
import androidx.compose.runtime.setValue
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class ProjectSummaryPaneTest {
  @Test
  fun deterministicFactsRemainAvailableWhenModelInterpretationIsMissing() {
    val project =
        ProjectAnalysis(
            "project",
            "revision",
            "Mini",
            "/tmp/project",
            "go",
            buildFile = "go.mod",
            fileCount = 4,
            sourceFileCount = 3,
            totalLines = 120,
            languages = mapOf("Go" to 3),
            summary = "",
            aiStatus = "",
            analyzedAt = "")

    val summary = projectSummaryPresentation(null, project)

    assertTrue(summary.hasProject)
    assertEquals("go · go.mod", "${summary.projectType} · ${summary.buildMetadata}")
    assertEquals("Go", summary.languages)
    assertEquals(listOf(4, 120), summary.projectMetrics.map { it.value })
    assertTrue(summary.findingMetrics.all { it.value == null })
    assertTrue(summary.coverageMetrics.all { it.value == null })
    assertFalse(summary.toString().contains("revision"))
    assertEquals("missing", summary.interpretationStatus)
    assertEquals("Project description: unavailable", summary.interpretationMessage)
    assertEquals("Project description: unavailable", summary.analysisMessage)
  }

  @Test
  fun interpretationDetailsAndFindingSourcesStayExplicit() {
    val overview =
        ProjectOverview(
            projectId = "project",
            projectRevision = "revision",
            metrics =
                ProjectMetrics(type = "go", fileCount = 2, sourceFileCount = 2, totalLines = 20),
            analysis =
                StructuredProjectAnalysis(
                    status = "stale",
                    purpose = "Coordinate requests through one handler.",
                    architecture = "Handlers call services.",
                    components = listOf("Handlers"),
                    entryPoints = listOf("cmd/main.go"),
                    nextSteps = listOf("Add validation."),
                    risks = listOf(ProjectAnalysisRisk("medium", "Validate inputs."))),
            analysisCoverage = AnalysisCoverage(fresh = 1, stale = 1, failed = 1),
            findingCounts = FindingCounts(verified = 2, aiSuggestions = 3),
        )

    val summary = projectSummaryPresentation(overview, null)

    assertEquals("stale", summary.analysisStatus)
    assertEquals("Coordinate requests through one handler.", summary.purpose)
    assertEquals("stale", summary.interpretationStatus)
    assertTrue(summary.interpretationMessage.contains("source may have changed"))
    assertTrue(summary.analysisMessage.contains("source may have changed"))
    assertEquals(listOf(2, 20), summary.projectMetrics.map { it.value })
    assertEquals(listOf(2, 3), summary.findingMetrics.map { it.value })
    assertEquals("Tool-reported issues", summary.findingMetrics.first().label)
    assertEquals(
        listOf("Up to date", "Outdated", "Failed"),
        summary.coverageMetrics.map { it.label },
    )
    assertEquals(listOf(1, 1, 1), summary.coverageMetrics.map { it.value })
    assertEquals(listOf("Architecture", "Packages / modules"), summary.details.map { it.title })
  }

  @Test
  fun unavailableCountsStayDistinctFromGenuineZeros() {
    val unavailable = projectSummaryPresentation(null, null)
    val zeroes =
        projectSummaryPresentation(
            ProjectOverview(
                metrics = ProjectMetrics(fileCount = 0, totalLines = 0),
                analysisCoverage = AnalysisCoverage(),
                findingCounts = FindingCounts()),
            null)

    assertFalse(unavailable.hasProject)
    assertTrue(unavailable.projectMetrics.all { it.value == null })
    assertTrue(unavailable.findingMetrics.all { it.value == null })
    assertTrue(unavailable.coverageMetrics.all { it.value == null })
    assertTrue(zeroes.hasProject)
    assertEquals(listOf(0, 0), zeroes.projectMetrics.map { it.value })
    assertEquals(listOf(0, 0), zeroes.findingMetrics.map { it.value })
    assertTrue(zeroes.coverageMetrics.isEmpty())
  }

  @Test
  fun missingRunningAndFailedInterpretationStatesKeepFailureMeaning() {
    val running =
        projectSummaryPresentation(
            ProjectOverview(analysis = StructuredProjectAnalysis(status = "running")), null)
    val failed =
        projectSummaryPresentation(
            ProjectOverview(
                analysis = StructuredProjectAnalysis(status = "failed", failure = "Timed out.")),
            null)

    assertEquals("Project description: running", running.interpretationMessage)
    assertTrue(running.analysisMessage.contains(": running"))
    assertEquals("Project description: failed · Timed out.", failed.interpretationMessage)
    assertEquals("Project description: failed · Timed out.", failed.analysisMessage)
    assertEquals(null, failed.purpose)
  }

  @Test
  fun issueCountsUseCurrentAnalysisSectionsAndKeepPartialAndUnavailableStates() {
    val project = resultProjectFixture()
    val run =
        analysisRunFixture()
            .copy(
                sections =
                    listOf(
                        AnalysisSectionProgress(
                            "bugs", "completed_empty", AnalysisRunCoverage(succeeded = 1), 0),
                        AnalysisSectionProgress(
                            "performance", "partial", AnalysisRunCoverage(partial = 1), 3),
                        AnalysisSectionProgress(
                            "security", "failed", AnalysisRunCoverage(failed = 1))))
    val summary = projectSummaryPresentation(null, project, run)
    assertEquals(listOf("Bugs", "Performance", "Security"), summary.issueMetrics.map { it.label })
    assertEquals(listOf(0, 3, null), summary.issueMetrics.map { it.value })
    assertEquals(
        listOf("Completed · no findings", "Partial", "Failed"),
        summary.issueMetrics.map { it.status })
    assertEquals(
        listOf("completed_empty", "partial", "failed"), summary.issueMetrics.map { it.statusCode })
    assertTrue(projectSummaryPresentation(null, project).issueMetrics.all { it.value == null })
    listOf(
            run.copy(status = "stale"),
            run.copy(identity = run.identity.copy(projectRevision = "old")),
            run.copy(identity = run.identity.copy(projectId = "other")))
        .forEach { stale ->
          assertTrue(
              projectSummaryPresentation(null, project, stale).issueMetrics.all {
                it.value == null
              })
        }
  }

  @Test
  fun diagramInputsPreserveMermaidAndLegacyProse() {
    val mermaid = "flowchart TD\n A[API] --> B[Service]"
    assertEquals(mermaid, summaryDiagramInput(mermaid).source)
    assertEquals(
        SummaryDiagramInput(mermaid, "Overview"),
        summaryDiagramInput("Overview\n```mermaid\n$mermaid\n```"))
    assertEquals(
        SummaryDiagramInput(mermaid, "Overview"),
        summaryDiagramInput("Overview\r\n``` Mermaid\r\n$mermaid\r\n```"))
    assertTrue(
        summaryDiagramInput("API → Service").source!!.contains("n0[\"API\"] --> n1[\"Service\"]"))
    listOf("Handlers call services.", "API ->", "`API -> Service`", "API -> Service\nwith details")
        .forEach { assertEquals(SummaryDiagramInput(null, it), summaryDiagramInput(it)) }
  }

  @Test
  fun summaryDisclosuresResetForAnotherProjectWithIdenticalContent() {
    val source = "flowchart TD\n A[Client] --> B[Service]"
    val insight =
        EngineeringInsight(
            mechanism = "Validate request identity.",
            whyItMattersHere = "The current revision owns the interpretation.",
            tradeoffOrFailureMode = "Validation requires consistent rules.")
    var currentProject by
        androidx.compose.runtime.mutableStateOf(
            resultProjectFixture().copy(projectId = "first", projectRevision = "one"))
    var currentOverview by
        androidx.compose.runtime.mutableStateOf(
            ProjectOverview(
                projectId = "first",
                projectRevision = "one",
                analysis =
                    StructuredProjectAnalysis(
                        status = "fresh", architecture = source, engineeringInsight = insight)))

    ComposeVisualFixture(1000, 760) { ProjectSummaryPane(currentOverview, currentProject, {}) }
        .use { fixture ->
          fixture.awaitDescription("Show Architecture diagram", "Collapsed")
          fixture.clickDescription("Show Architecture diagram")
          fixture.awaitDescription("Architecture diagram\n$source")
          fixture.clickDescription("Zoom in Architecture")
          fixture.render()
          assertTrue(fixture.hasText("125%"))
          fixture.clickText("Mermaid source")
          assertTrue(fixture.tryClick("Expand More insight"))
          fixture.render()
          assertTrue(fixture.hasText("Trade-off or failure mode"))

          currentProject = currentProject.copy(projectId = "second", projectRevision = "two")
          currentOverview = currentOverview.copy(projectId = "second", projectRevision = "two")
          fixture.awaitDescription("Show Architecture diagram", "Collapsed")
          assertFalse(fixture.hasText("Mermaid source"))
          fixture.clickDescription("Show Architecture diagram")
          fixture.awaitDescription("Architecture diagram\n$source")
          assertTrue(fixture.hasText("100%"))
          assertEquals("Collapsed", fixture.stateDescription("More insight"))
          assertFalse(fixture.hasText("Trade-off or failure mode"))
        }
  }

  @Test
  fun moduleNamesComeBeforeTheirPathsWithoutLosingDescriptions() {
    assertEquals(
        SummaryModule("Workflow orchestration", "internal/app", "Coordinates changes."),
        summaryModule("`internal/app` (Workflow orchestration): Coordinates changes."))
    assertEquals(SummaryModule("API", "internal/api", ""), summaryModule("internal/api (API)"))
    assertEquals(SummaryModule("Legacy component", null, ""), summaryModule("Legacy component"))
    assertEquals(SummaryModule("path (unfinished", null, ""), summaryModule("path (unfinished"))
  }

  @Test
  fun analysisFailureTakesPrecedenceOverOutdatedAndUpdatedColors() {
    val project = resultProjectFixture()
    val overview = ProjectOverview(analysis = StructuredProjectAnalysis(status = "fresh"))
    val updated = projectSummaryPresentation(overview, project)
    assertEquals("unknown", updated.summaryStatus)
    assertEquals(SecondaryText, summaryAnalysisTint(updated.summaryStatus))
    val outdated =
        projectSummaryPresentation(
            overview.copy(analysisCoverage = AnalysisCoverage(stale = 1)), project)
    assertEquals(Warning, summaryAnalysisTint(outdated.summaryStatus))
    val failed =
        projectSummaryPresentation(
            overview.copy(analysisCoverage = AnalysisCoverage(stale = 1, failed = 1)),
            project,
            analysisRunFixture()
                .copy(status = "failed", reason = "Run interrupted by a storage failure."))
    assertEquals(Error, summaryAnalysisTint(failed.summaryStatus))
    assertTrue(failed.analysisMessage.contains("Run interrupted by a storage failure."))
    assertEquals(
        Error,
        summaryAnalysisTint(
            projectSummaryPresentation(
                    overview.copy(analysis = StructuredProjectAnalysis(status = "failed")), project)
                .summaryStatus))
  }

  @Test
  fun activeCurrentRunTakesPrecedenceOverCachedOverviewCoverage() {
    val project = resultProjectFixture()
    val overview =
        ProjectOverview(
            projectId = project.projectId,
            projectRevision = project.projectRevision,
            analysis = StructuredProjectAnalysis(status = "fresh"),
            analysisCoverage = AnalysisCoverage(total = 3, fresh = 3))
    val running = analysisRunFixture().copy(status = "running")

    val summary = projectSummaryPresentation(overview, project, running)

    assertEquals("running", summary.summaryStatus)
    assertEquals(Information, summaryAnalysisTint(summary.summaryStatus))
  }

  @Test
  fun selectedFileEvidenceOverridesOldOverviewAndDescriptionWithoutHidingChanges() {
    val project = analysisProjectFixture()
    val overview =
        ProjectOverview(
            projectId = "project",
            projectRevision = "revision",
            analysis = StructuredProjectAnalysis(status = "stale", purpose = "Saved description"),
            analysisCoverage = AnalysisCoverage(total = 3, fresh = 1, stale = 2))
    val selection = selectionFixture().copy(excludedPaths = listOf("main.go"))
    ComposeVisualFixture(800, 650, 1.5f) {
          ProjectSummaryPane(overview, project, {}, fileSelection = selection)
        }
        .use { fixture ->
          fixture.render("summary-current-selection-old-description-800-150")
          fixture.assertSummaryStatusPlacement("Updated")
          assertTrue(fixture.hasText("Project description: stale · source may have changed"))
          assertFalse(fixture.hasText("Outdated"))
        }
    val current = projectSummaryPresentation(overview, project, fileSelection = selection)
    assertEquals("fresh", current.summaryStatus)
    assertFalse(current.outdated)
    assertEquals("stale", current.analysisStatus)
    assertEquals("stale", current.interpretationStatus)
    assertEquals("Saved description", current.purpose)
    assertTrue(current.analysisMessage.contains("source may have changed"))
    assertEquals(listOf("Up to date"), current.coverageMetrics.map { it.label })
    val included =
        projectSummaryPresentation(
            overview, project, fileSelection = selection.copy(excludedPaths = emptyList()))
    assertEquals("partial", included.summaryStatus)
    val staleFile =
        selection.files.last().copy(stages = selectionStageFixture("stale", "Source changed."))
    val stale =
        projectSummaryPresentation(
            overview,
            project,
            fileSelection = selection.copy(excludedPaths = emptyList(), files = listOf(staleFile)))
    assertTrue(stale.outdated)
    assertEquals("stale", stale.summaryStatus)
    val allExcluded =
        projectSummaryPresentation(
            overview,
            project,
            fileSelection = selection.copy(excludedPaths = listOf("helper.go", "main.go")))
    assertEquals("excluded", allExcluded.summaryStatus)
    assertTrue(allExcluded.coverageMetrics.isEmpty())
    val otherProject =
        projectSummaryPresentation(
            overview, project, fileSelection = selection.copy(projectId = "other"))
    assertEquals("stale", otherProject.summaryStatus)
  }

  @Test
  fun coverageSegmentsPreserveUnknownEmptyAndUnaccountedFiles() {
    val unknown = projectSummaryPresentation(null, resultProjectFixture())
    assertEquals("unknown", unknown.summaryStatus)
    assertTrue(summaryCoverageFractions(unknown.coverageMetrics).isEmpty())
    val empty =
        projectSummaryPresentation(
            null,
            analysisProjectFixture(),
            fileSelection = selectionFixture().copy(excludedPaths = listOf("helper.go", "main.go")))
    assertEquals("excluded", empty.summaryStatus)
    assertTrue(summaryCoverageFractions(empty.coverageMetrics).isEmpty())
    val partial =
        projectSummaryPresentation(
            ProjectOverview(analysisCoverage = AnalysisCoverage(total = 4, fresh = 1, failed = 1)),
            null)
    val segments = summaryCoverageFractions(partial.coverageMetrics)
    assertEquals(listOf("Up to date", "Failed", "Unavailable"), segments.map { it.first.label })
    assertEquals(listOf(1, 1, 2), segments.map { it.first.value })
    assertEquals(listOf(.25f, .25f, .5f), segments.map { it.second })
    val large =
        summaryCoverageFractions(
            listOf(
                ProjectSummaryMetric("Up to date", Int.MAX_VALUE, SummaryMetricTone.Ready),
                ProjectSummaryMetric("Outdated", Int.MAX_VALUE, SummaryMetricTone.Stale)))
    assertEquals(listOf(.5f, .5f), large.map { it.second })
  }

  @Test
  fun currentRunLifecycleRemainsVisibleAndForeignRunsCannotOverrideCoverage() {
    val project = resultProjectFixture()
    val overview = ProjectOverview(analysisCoverage = AnalysisCoverage(total = 1, fresh = 1))
    listOf("paused", "interrupted", "canceled", "partial", "failed").forEach { status ->
      assertEquals(
          status,
          projectSummaryPresentation(overview, project, analysisRunFixture().copy(status = status))
              .summaryStatus)
    }
    val foreign =
        analysisRunFixture().let {
          it.copy(status = "running", identity = it.identity.copy(projectId = "other"))
        }
    assertEquals("fresh", projectSummaryPresentation(overview, project, foreign).summaryStatus)
    assertFalse(
        projectSummaryPresentation(
                overview,
                project,
                foreign.copy(status = "failed", reason = "Other project's failure"))
            .analysisMessage
            .contains("Other project's failure"))
  }

  @Test
  fun coverageNavigationIsLocalAndLiveSelectionUpdatesItsSegments() {
    val navigations = mutableListOf<Workspace>()
    val project = analysisProjectFixture()
    val initial = selectionFixture().copy(excludedPaths = listOf("main.go"))
    var selection by androidx.compose.runtime.mutableStateOf(initial)
    ComposeVisualFixture(1000, 760) {
          ProjectSummaryPane(null, project, navigations::add, fileSelection = selection)
        }
        .use { fixture ->
          fixture.render("summary-selected-coverage")
          fixture.assertTextFits("1 up to date")
          fixture.clickText("View analysis")
          assertEquals(listOf(Workspace.Analysis), navigations)
          assertEquals(initial, selection)
          assertTrue(fixture.requestFocus("View analysis"))
          fixture.pressKey(androidx.compose.ui.input.key.Key.Enter)
          fixture.render()
          assertEquals(listOf(Workspace.Analysis, Workspace.Analysis), navigations)
          selection = selection.copy(excludedPaths = emptyList())
          fixture.render("summary-selection-updated")
          fixture.assertTextFits("1 up to date")
          fixture.assertTextFits("1 not analyzed")
          assertEquals(
              "partial",
              projectSummaryPresentation(null, project, fileSelection = selection).summaryStatus)
        }
  }

  @Test
  fun outdatedSummaryIncludesStaleFilesAndMismatchedRuns() {
    val project = resultProjectFixture()
    val overview = ProjectOverview(analysis = StructuredProjectAnalysis(status = "fresh"))
    assertFalse(projectSummaryPresentation(overview, project).outdated)
    assertTrue(
        projectSummaryPresentation(
                overview.copy(analysis = StructuredProjectAnalysis(status = "stale")), project)
            .outdated)
    assertTrue(
        projectSummaryPresentation(
                overview.copy(analysisCoverage = AnalysisCoverage(stale = 1)), project)
            .outdated)
    assertTrue(
        projectSummaryPresentation(overview, project, analysisRunFixture().copy(status = "stale"))
            .outdated)
    assertTrue(
        projectSummaryPresentation(
                overview,
                project,
                analysisRunFixture().let {
                  it.copy(identity = it.identity.copy(projectRevision = "old"))
                })
            .outdated)
    assertFalse(projectSummaryPresentation(overview, project, analysisRunFixture()).outdated)
  }
}
