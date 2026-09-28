package io.miniorca.desktop

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.unit.dp
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertTrue
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withContext

class ProjectSummaryPaneTest {
  @Test
  fun localGridBreakpointsRespectTextScaleAndExactBoundary() {
    listOf(1f, 1.25f, 1.5f).forEach { scale ->
      listOf(8.dp, 12.dp).forEach { gap ->
        val boundary = 200.dp * 3 * scale + gap * 2
        assertTrue(categoryPanelsStacked(boundary - 1.dp, scale, gap))
        assertFalse(categoryPanelsStacked(boundary, scale, gap))
        assertFalse(categoryPanelsStacked(boundary + 1.dp, scale, gap))
      }
    }
  }

  @Test
  fun pairedRegionsRequireReadableLocalWidthAtEachTextScale() {
    listOf(1f, 1.25f, 1.5f).forEach { scale ->
      val resultsBoundary = 1080.dp * scale
      assertTrue(coverageResultsStacked(resultsBoundary - 1.dp, scale))
      assertFalse(coverageResultsStacked(resultsBoundary, scale))
      val narrativeBoundary = 840.dp * scale
      assertTrue(narrativePanelsStacked(narrativeBoundary - 1.dp, scale))
      assertFalse(narrativePanelsStacked(narrativeBoundary, scale))
      val evidenceBoundary = 960.dp * scale
      assertTrue(summaryEvidencePanelsStacked(evidenceBoundary - 1.dp, scale))
      assertFalse(summaryEvidencePanelsStacked(evidenceBoundary, scale))
    }
  }

  @Test
  fun coverageControlsStackAtLocalWidthAndTextScale() {
    listOf(1f, 1.25f, 1.5f).forEach { scale ->
      val heading = 300.dp * scale
      val dial = 320.dp * scale
      assertTrue(coverageHeadingStacked(heading - 1.dp, scale))
      assertFalse(coverageHeadingStacked(heading, scale))
      assertTrue(coverageDialStacked(dial - 1.dp, scale))
      assertFalse(coverageDialStacked(dial, scale))
    }
  }

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
    assertEquals("Go project", projectTypePresentation(summary.projectType).description)
    assertEquals("go.mod", summary.buildMetadata)
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
  fun typeLabelUsesCurrentMetricsAndFallsBackToProjectMetadata() {
    val project = resultProjectFixture().copy(type = "Go", buildFile = "go.mod")
    listOf("Go" to "Go project", "Java" to "Java project", "Kotlin" to "Kotlin project").forEach {
        (type, label) ->
      val overview =
          ProjectOverview(
              projectId = project.projectId,
              projectRevision = project.projectRevision,
              metrics = ProjectMetrics(type = type, buildFile = "build.gradle.kts"))
      val summary = projectSummaryPresentation(overview, project)
      assertEquals(label, projectTypePresentation(summary.projectType).description)
      assertEquals("build.gradle.kts", summary.buildMetadata)
    }
    val unknown = projectSummaryPresentation(null, project.copy(type = "  "))
    assertEquals("Project", projectTypePresentation(unknown.projectType).description)
    assertEquals("go.mod", unknown.buildMetadata)
    val custom = projectSummaryPresentation(null, project.copy(type = "Scala / JVM"))
    assertEquals(
        "Project type: Scala / JVM", projectTypePresentation(custom.projectType).description)
    assertEquals(
        "Project",
        projectTypePresentation(projectSummaryPresentation(null, null).projectType).description)
  }

  @Test
  fun obsoleteOverviewCannotSupplyFactsOrDescriptionAfterReindex() {
    val project =
        resultProjectFixture()
            .copy(
                projectRevision = "new",
                type = "Go",
                buildFile = "go.mod",
                fileCount = 7,
                totalLines = 150,
                languages = mapOf("Go" to 7),
                aiStatus = "missing")
    val old =
        ProjectOverview(
            projectId = project.projectId,
            projectRevision = "old",
            metrics =
                ProjectMetrics(
                    type = "Java", buildFile = "pom.xml", fileCount = 99, totalLines = 900),
            analysis =
                StructuredProjectAnalysis(
                    status = "fresh", purpose = "Old purpose", components = listOf("Old module")),
            analysisCoverage = AnalysisCoverage(total = 1, fresh = 1),
            findingCounts = FindingCounts(verified = 8, aiSuggestions = 4))
    val summary = projectSummaryPresentation(old, project)
    assertEquals("Go project", projectTypePresentation(summary.projectType).description)
    assertEquals("go.mod", summary.buildMetadata)
    assertEquals("Go", summary.languages)
    assertEquals(listOf(7, 150), summary.projectMetrics.map { it.value })
    assertTrue(summary.findingMetrics.all { it.value == null })
    assertTrue(summary.coverageMetrics.all { it.value == null })
    assertEquals("unknown", summary.summaryStatus)
    assertEquals("missing", summary.interpretationStatus)
    assertEquals(null, summary.purpose)
    assertTrue(summary.details.isEmpty())
    assertTrue(summary.issueMetrics.all { it.value == null })
    assertEquals("Project description: unavailable", summary.interpretationMessage)
    val otherProject =
        projectSummaryPresentation(old.copy(projectId = "other", projectRevision = "new"), project)
    assertEquals(summary.projectMetrics, otherProject.projectMetrics)
    assertEquals(summary.findingMetrics, otherProject.findingMetrics)
    assertEquals(summary.interpretationMessage, otherProject.interpretationMessage)
    // A matching selected-file snapshot can be current even when the saved overview is not.
    val currentSelection = selectionFixture().copy(projectRevision = "new")
    val selected = projectSummaryPresentation(old, project, fileSelection = currentSelection)
    assertTrue(selected.analysisMessage.contains("Selected files:"))
    assertTrue(selected.analysisMessage.contains("Project description: unavailable"))
    assertTrue(selected.findingMetrics.all { it.value == null })
  }

  @Test
  fun freshPurposeAndBlankPurposeHaveDifferentVisibleQualifications() {
    val overview =
        ProjectOverview(
            analysis =
                StructuredProjectAnalysis(status = "fresh", purpose = "Model interpretation"))
    val described = projectSummaryPresentation(overview, null)
    assertEquals("Model interpretation", described.purpose)
    assertEquals("Project description: current · AI-generated", described.interpretationMessage)
    val blank =
        projectSummaryPresentation(
            overview.copy(analysis = overview.analysis.copy(purpose = "  ")), null)
    assertEquals(null, blank.purpose)
    assertEquals(
        "Project description: unavailable · no purpose provided", blank.interpretationMessage)
    assertTrue(blank.analysisMessage.contains(blank.interpretationMessage))
    ComposeVisualFixture(800, 650) { ProjectSummaryPane(overview, null, {}) }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText(described.interpretationMessage))
        }
    ComposeVisualFixture(800, 650) {
          ProjectSummaryPane(
              overview.copy(analysis = overview.analysis.copy(purpose = "")), null, {})
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText(blank.interpretationMessage))
          assertFalse(fixture.hasText("Model interpretation"))
        }
  }

  @Test
  fun failedDescriptionKeepsDiagnosticBesideFreshSelectedFiles() {
    val project = analysisProjectFixture()
    val overview =
        ProjectOverview(
            projectId = project.projectId,
            projectRevision = project.projectRevision,
            analysis =
                StructuredProjectAnalysis(
                    status = "failed", failure = "Model timed out.", purpose = "Old text"),
            analysisCoverage = AnalysisCoverage(stale = 2))
    val selection = selectionFixture().copy(excludedPaths = listOf("main.go"))
    val summary = projectSummaryPresentation(overview, project, fileSelection = selection)
    assertEquals("fresh", summary.summaryStatus)
    assertEquals("Project description: failed · Model timed out.", summary.interpretationMessage)
    assertEquals(null, summary.purpose)
    assertTrue(summary.analysisMessage.contains(summary.interpretationMessage))
    ComposeVisualFixture(800, 650) {
          ProjectSummaryPane(overview, project, {}, fileSelection = selection)
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText(summary.interpretationMessage))
        }
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
            analysisCoverage = AnalysisCoverage(total = 3, fresh = 1, stale = 1, failed = 1),
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
    assertEquals(SummaryCoverageProjection.Unavailable, zeroes.coverage)
    assertTrue(zeroes.coverageMetrics.all { it.value == null })
  }

  @Test
  fun groupedResultsKeepUnknownAndZeroProvenanceDistinct() {
    val project = resultProjectFixture()
    ComposeVisualFixture(800, 1100) { ProjectSummaryPane(null, project, {}) }
        .use { fixture ->
          fixture.render()
          assertEquals(1, fixture.tagCount("summary-coverage-results"))
          assertTrue(fixture.hasText("File counts unavailable"))
          assertTrue(
              fixture.hasText("Overall findings · — tool-reported issues · — AI suggestions"))
          listOf("Bugs", "Performance", "Security").forEach {
            assertTrue(fixture.hasDescription("View $it results"))
          }
        }
    val zeroOverview =
        ProjectOverview(
            projectId = project.projectId,
            projectRevision = project.projectRevision,
            findingCounts = FindingCounts(verified = 0, aiSuggestions = 0))
    ComposeVisualFixture(800, 1100) { ProjectSummaryPane(zeroOverview, project, {}) }
        .use { fixture ->
          fixture.render()
          assertTrue(
              fixture.hasText("Overall findings · 0 tool-reported issues · 0 AI suggestions"))
          assertFalse(fixture.hasText("Safe"))
        }
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
        listOf("Completed · details not confirmed", "Partial", "Failed"),
        summary.issueMetrics.map { it.status })
    assertEquals(
        listOf("completed", "partial", "failed"), summary.issueMetrics.map { it.statusCode })
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
  fun summaryCardKeepsReportedZeroAndSavedReadFailureSeparateFromRunLifecycle() {
    val project = resultProjectFixture()
    val (completedRun, details) = summaryBugFixture(emptyList())
    val pausedRun = completedRun.copy(status = "paused", reason = "Paused by user")
    val sections = bugSections(details.copy(error = "Saved results could not be read"))
    val summary = projectSummaryPresentation(null, project, pausedRun, sections)
    assertEquals("paused", summary.summaryStatus)
    assertEquals("paused", summary.issueMetrics.first().statusCode)
    assertEquals(
        "Saved details unavailable · 0 reported", summary.issueMetrics.first().detailStatus)
    listOf("interrupted", "canceled").forEach { lifecycle ->
      val other =
          projectSummaryPresentation(
              null,
              project,
              pausedRun.copy(
                  status = lifecycle,
                  sections = pausedRun.sections.map { it.copy(status = lifecycle) }),
              sections)
      assertEquals(lifecycle, other.summaryStatus)
      assertEquals(lifecycle, other.issueMetrics.first().statusCode)
      assertEquals(
          "Saved details unavailable · 0 reported", other.issueMetrics.first().detailStatus)
    }
    ComposeVisualFixture(800, 650) {
          ProjectSummaryPane(null, project, {}, run = pausedRun, sections = sections)
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Saved details unavailable · 0 reported"))
          assertTrue(fixture.hasText("Paused by user"))
          assertFalse(fixture.hasText("No results"))
        }
  }

  @Test
  fun diagramInputsPreserveMermaidAndLegacyProse() {
    val mermaid = "flowchart TD\n A[API] --> B[Service]"
    listOf(mermaid, "graph LR\r\n A --> B", "sequenceDiagram\n A->>B: Request").forEach {
      assertEquals(SummaryDiagramInput(it.trim(), "", it), summaryDiagramInput(it))
    }
    listOf("Overview\n```mermaid\n$mermaid\n```", "Overview\r\n``` Mermaid\r\n$mermaid\r\n```")
        .forEach { saved ->
          assertEquals(SummaryDiagramInput(mermaid, "Overview", saved), summaryDiagramInput(saved))
        }
    val saved =
        "Before\r\n```MERMAID\r\n$mermaid\r\n```\r\nBetween\n```mermaid\n" +
            "sequenceDiagram\n A->>B: Request\n```\nAfter"
    val input = summaryDiagramInput(saved)
    assertEquals(mermaid, input.source)
    assertEquals(
        "Before\r\n\r\nBetween\n```mermaid\nsequenceDiagram\n A->>B: Request\n```\nAfter",
        input.prose)
    assertEquals(saved, input.original)
    assertFalse(input.generatedFromArrowChain)
    val chain = "API → Service"
    val generated = summaryDiagramInput(chain)
    assertTrue(generated.source!!.contains("n0[\"API\"] --> n1[\"Service\"]"))
    assertEquals(chain, generated.original)
    assertEquals("", generated.prose)
    assertTrue(generated.generatedFromArrowChain)
    listOf("Handlers call services.", "API ->", "`API -> Service`", "API -> Service\nwith details")
        .forEach { assertEquals(SummaryDiagramInput(null, it, it), summaryDiagramInput(it)) }
  }

  @Test
  fun unsupportedDeclarationsRemainAttemptedDiagramsWithOriginalContent() = runBlocking {
    val unsupported =
        listOf(
            "classDiagram\n A <|-- B",
            "stateDiagram-v2\n A --> B",
            "customDiagram\n A --> B",
            "kanban\n Todo",
            "pie showData\n  \"Dogs\" : 12",
            "pie title Pets\n  \"Cats\" : 8",
            "pie showData title Pets\n  \"Birds\" : 5")
    for (source in unsupported) {
      val saved = "Introduction\n$source\nConclusion"
      val input = summaryDiagramInput(saved)
      assertEquals(source + "\nConclusion", input.source)
      assertEquals("Introduction", input.prose)
      assertEquals(saved, input.original)
      assertFalse(input.generatedFromArrowChain)
      val failure =
          assertFailsWith<IllegalArgumentException> {
            MermaidRenderer.render(requireNotNull(input.source))
          }
      assertTrue(failure.message!!.contains("Use a Mermaid flowchart or sequence diagram"))
    }
    val fenced = "Notes\n```mermaid\nclassDiagram\n A <|-- B\n```\nMore notes"
    assertEquals("classDiagram\n A <|-- B", summaryDiagramInput(fenced).source)
    assertEquals(fenced, summaryDiagramInput(fenced).original)
    assertEquals("Notes\n\nMore notes", summaryDiagramInput(fenced).prose)
    val plain = summaryDiagramInput("Handlers call services.")
    assertEquals(null, plain.source)
    assertEquals("Handlers call services.", plain.original)
    listOf(
            "Journey through the project",
            "Timeline of changes",
            "ClassDiagram of the codebase",
            "Pie shows data for each team")
        .forEach { prose ->
          val report = "Overview\r\n$prose\r\nOther observations."
          assertEquals(SummaryDiagramInput(null, report, report), summaryDiagramInput(report))
        }
    listOf(
            "journey",
            "timeline",
            "classDiagram",
            "stateDiagram-v2",
            "customDiagram",
            "pie showData")
        .forEach { declaration ->
          val report = "Overview\r\n  $declaration  \r\n  A --> B"
          assertEquals("$declaration  \r\n  A --> B", summaryDiagramInput(report).source)
          assertEquals(report, summaryDiagramInput(report).original)
        }
  }

  @Test
  fun unsupportedDiagramFailureIsVisibleButProseOnlyIsUnavailable() {
    listOf("classDiagram\n A <|-- B", "pie showData\n  \"Dogs\" : 12").forEach { unsupported ->
      ComposeVisualFixture(800, 650) { MermaidDiagram(unsupported, "Architecture") }
          .use { fixture ->
            fixture.awaitDescription("Expand Architecture diagram", "Diagram failed")
            fixture.render()
            assertTrue(
                fixture.hasText("Diagram unavailable: Use a Mermaid flowchart or sequence diagram"))
            assertTrue(fixture.hasText(unsupported))
          }
    }
    listOf(
            "Handlers call services.",
            "Journey through the project",
            "Timeline of changes",
            "Pie shows data for each team")
        .forEach { report ->
          ComposeVisualFixture(800, 650) { MermaidDiagram(report, "Architecture") }
              .use { fixture ->
                fixture.render()
                assertTrue(fixture.hasText(report))
                fixture.awaitDescription("Expand Architecture diagram", "Diagram unavailable")
                assertFalse(
                    fixture.hasText(
                        "Diagram unavailable: Use a Mermaid flowchart or sequence diagram"))
              }
        }
  }

  @Test
  fun previewLoadingAcceptsFirstExpandWithoutDispatchingWorkflowActions() {
    val source = "flowchart LR\n A --> B"
    val gate = CompletableDeferred<MermaidImage>()
    var renders = 0
    ComposeVisualFixture(800, 650) {
          MermaidDiagram(
              source,
              "Architecture",
              render = {
                renders++
                gate.await()
              })
        }
        .use { fixture ->
          fixture.awaitDescription("Expand Architecture diagram", "Rendering diagram")
          assertTrue(fixture.hasText("Rendering diagram…"))
          assertEquals(1, fixture.tagCount("diagram-preview"))
          fixture.clickDescription("Expand Architecture diagram")
          fixture.awaitDescription("Close Architecture diagram")
          assertEquals(1, renders)
          gate.complete(MermaidImage(ImageBitmap(16, 16), 16f, 16f))
          fixture.awaitDescription("Architecture diagram preview\n$source")
          fixture.awaitDescription("Architecture diagram\n$source")
          assertEquals(1, renders)
        }
  }

  @Test
  fun legacyArrowChainCopiesOriginalInsteadOfGeneratedMermaid() {
    val saved = "API → Service"
    var renders = 0
    ComposeVisualFixture(800, 650) {
          MermaidDiagram(
              saved,
              "Flow 1",
              render = {
                renders++
                MermaidImage(ImageBitmap(16, 16), 16f, 16f)
              })
        }
        .use { fixture ->
          fixture.awaitDescription("Expand Flow 1 diagram", "Preview")
          fixture.clickDescription("Expand Flow 1 diagram")
          fixture.awaitDescription("Close Flow 1 diagram")
          fixture.clickText("Mermaid source")
          fixture.render()
          assertTrue(fixture.hasText("Original saved arrow chain"))
          assertTrue(fixture.hasText(saved))
          assertTrue(fixture.hasText("Generated Mermaid for rendering (not saved)"))
          assertTrue(fixture.hasText(requireNotNull(summaryDiagramInput(saved).source)))
          fixture.clickDescription("Copy saved content for Flow 1")
          fixture.render()
          assertEquals(saved, fixture.clipboardText())
          assertEquals(1, renders)
        }
  }

  @Test
  fun failedAndOverLimitDiagramsKeepCompleteLiteralSource() {
    val invalid = "classDiagram\n A <|-- B\n<script>inert</script>"
    val overLimit = "flowchart LR\n" + " A --> B\n".repeat(260)
    for (saved in listOf(invalid, overLimit)) {
      ComposeVisualFixture(800, 650) { MermaidDiagram(saved, "Architecture") }
          .use { fixture ->
            fixture.awaitDescription("Expand Architecture diagram", "Diagram failed")
            fixture.render()
            assertTrue(fixture.hasText(saved))
            fixture.clickDescription("Expand Architecture diagram")
            fixture.awaitDescription("Close Architecture diagram")
            fixture.clickText("Mermaid source")
            fixture.render()
            assertTrue(fixture.hasText(saved))
            assertFalse(fixture.hasEditableText(withinTag = "diagram-source-scroll"))
            fixture.clickDescription("Copy saved content for Architecture")
            fixture.render()
            assertEquals(saved, fixture.clipboardText())
          }
    }
  }

  @Test
  fun blankDiagramDoesNotCreatePreview() {
    ComposeVisualFixture(800, 650) { MermaidDiagram("  ", "Architecture") }
        .use { fixture ->
          fixture.render()
          assertEquals(0, fixture.tagCount("diagram-preview"))
          assertTrue(fixture.isDisabled("Expand diagram"))
        }
  }

  @Test
  fun ownedRenderRejectsLateCompletionAndDoesNotReportCancellationAsFailure() = runBlocking {
    val scope = CoroutineScope(SupervisorJob() + Dispatchers.Unconfined)
    val oldGate = CompletableDeferred<MermaidImage>()
    val started = CompletableDeferred<Unit>()
    val cancelled = CompletableDeferred<Unit>()
    val image = MermaidImage(ImageBitmap(8, 8), 8f, 8f)
    val old = DiagramViewState()
    try {
      old.start(scope, "old") {
        started.complete(Unit)
        try {
          oldGate.await()
        } catch (exception: CancellationException) {
          cancelled.complete(Unit)
          withContext(NonCancellable) { oldGate.await() }
        }
      }
      started.await()
      assertEquals(DiagramState.Loading, old.renderState)
      old.cancel()
      cancelled.await()
      val replacement = DiagramViewState()
      replacement.start(scope, "new") { image }
      assertEquals(DiagramState.Ready(image), replacement.renderState)
      oldGate.complete(image)
      // The canceled job may finish despite cancellation; it must not publish the old image.
      scope.coroutineContext[kotlinx.coroutines.Job]!!.children.forEach { it.join() }
      assertEquals(DiagramState.Loading, old.renderState)
      assertEquals(DiagramState.Ready(image), replacement.renderState)
    } finally {
      oldGate.complete(image)
      scope.cancel()
    }
  }

  @Test
  fun previewAndExpandKeepAllSavedFlowsAndNarrativeWithoutDispatch() {
    val project = resultProjectFixture()
    val architecture = "flowchart LR\n A --> B"
    val flow = "sequenceDiagram\n A->>B: Request"
    val overview =
        ProjectOverview(
            projectId = project.projectId,
            projectRevision = project.projectRevision,
            analysis =
                StructuredProjectAnalysis(
                    status = "fresh",
                    purpose = "Project purpose",
                    architecture = architecture,
                    flows = listOf(flow, "Legacy prose flow"),
                    components = listOf("Component description")))
    val navigations = mutableListOf<Workspace>()
    val actions =
        AnalysisWorkspaceActions(
            { _, _ -> error("Diagram preview started analysis") },
            { error("Diagram preview started analysis") },
            { error("Diagram preview started analysis") },
            { error("Diagram preview started analysis") },
            { error("Diagram preview started analysis") })
    var renders = 0
    ComposeVisualFixture(1000, 2400) {
          ProjectSummaryPane(
              overview,
              project,
              navigations::add,
              analysisActions = actions,
              diagramRender = { MermaidImage(ImageBitmap(12, 12), 12f, 12f).also { renders++ } })
        }
        .use { fixture ->
          fixture.awaitDescription("Architecture diagram preview\n$architecture")
          fixture.awaitDescription("Flow 1 diagram preview\n$flow")
          fixture.render()
          assertTrue(fixture.hasText("Project purpose"))
          assertTrue(fixture.hasText("Component description"))
          assertTrue(fixture.hasText("Legacy prose flow"))
          assertTrue(fixture.hasText("Change lifecycle"))
          fixture.clickDescription("Expand Architecture diagram")
          fixture.awaitDescription("Architecture diagram\n$architecture")
          fixture.clickDescription("Close Architecture diagram")
          fixture.clickDescription("Expand Flow 1 diagram")
          fixture.awaitDescription("Flow 1 diagram\n$flow")
          assertEquals(2, renders)
          assertTrue(navigations.isEmpty())
        }
  }

  @Test
  fun expandedOwnerClosesOnReplacementAndReopensWithoutLosingLocalSettings() {
    val project = resultProjectFixture()
    val original = "flowchart LR\n A --> B"
    val replacement = "flowchart LR\n C --> D"
    var overview by
        androidx.compose.runtime.mutableStateOf(
            ProjectOverview(
                projectId = project.projectId,
                projectRevision = project.projectRevision,
                analysis = StructuredProjectAnalysis(status = "fresh", architecture = original)))
    val renders = mutableListOf<String>()
    ComposeVisualFixture(1000, 1400) {
          ProjectSummaryPane(
              overview,
              project,
              {},
              diagramRender = { source ->
                renders += source
                MermaidImage(ImageBitmap(1200, 1200), 1200f, 1200f)
              })
        }
        .use { fixture ->
          fixture.awaitDescription("Architecture diagram preview\n$original")
          fixture.clickDescription("Expand Architecture diagram")
          fixture.awaitDescription("Architecture diagram\n$original")
          fixture.clickDescription("Zoom in Architecture")
          fixture.render()
          assertEquals("Zoom 125%", fixture.descriptionState("Reset zoom Architecture"))
          fixture.scrollTagged("diagram-horizontal-scroll", horizontal = true, pixels = 150f)
          fixture.scrollTagged("diagram-vertical-scroll", horizontal = false, pixels = 120f)
          val x = fixture.scrollPosition("diagram-horizontal-scroll", horizontal = true)
          val y = fixture.scrollPosition("diagram-vertical-scroll", horizontal = false)
          assertTrue(x > 0f && y > 0f)
          fixture.clickDescription("Mermaid source for Architecture")
          fixture.clickDescription("Close Architecture diagram")
          fixture.awaitDescription("Expand Architecture diagram", "Preview")
          fixture.clickDescription("Expand Architecture diagram")
          fixture.render()
          assertTrue(fixture.hasText("125%"))
          assertTrue(fixture.scrollPosition("diagram-horizontal-scroll", horizontal = true) >= x)
          assertTrue(fixture.scrollPosition("diagram-vertical-scroll", horizontal = false) >= y)
          assertTrue(fixture.hasText("Hide Mermaid"))
          repeat(3) {
            fixture.clickDescription("Zoom in Architecture")
            fixture.render()
          }
          assertTrue(fixture.hasText("200%"))
          assertTrue(fixture.isDisabled("+"))
          assertEquals("Zoom 200%", fixture.descriptionState("Reset zoom Architecture"))
          repeat(5) {
            fixture.clickDescription("Zoom out Architecture")
            fixture.render()
          }
          assertTrue(fixture.hasText("75%"))
          assertTrue(fixture.isDisabled("−"))
          fixture.clickDescription("Reset zoom Architecture")
          fixture.render()
          assertTrue(fixture.hasText("100%"))
          assertEquals("Zoom 100%", fixture.descriptionState("Reset zoom Architecture"))
          assertEquals(listOf(original), renders)
          overview = overview.copy(analysis = overview.analysis.copy(architecture = replacement))
          fixture.awaitDescription("Expand Architecture diagram", "Preview")
          assertFalse(fixture.hasDescription("Close Architecture diagram"))
          assertFalse(fixture.hasDescription("Architecture diagram\n$original"))
          fixture.clickDescription("Expand Architecture diagram")
          fixture.awaitDescription("Architecture diagram\n$replacement")
          assertTrue(fixture.hasText("100%"))
          assertFalse(fixture.hasText("Hide Mermaid"))
          assertEquals(listOf(original, replacement), renders)
        }
  }

  @Test
  fun summaryKeepsOneDecodedResultAcrossLazyDisposalAndReflow() {
    val source = "flowchart TD\n A[Client] --> B[Service]"
    val project = resultProjectFixture()
    var overview by
        androidx.compose.runtime.mutableStateOf(
            ProjectOverview(
                projectId = project.projectId,
                projectRevision = project.projectRevision,
                analysis =
                    StructuredProjectAnalysis(
                        status = "fresh",
                        purpose = "Project context. ".repeat(400),
                        architecture = source,
                        engineeringInsight =
                            EngineeringInsight(
                                mechanism = "Mechanism",
                                whyItMattersHere = "Reason",
                                tradeoffOrFailureMode = "Trade-off"))))
    var renders = 0
    val image = MermaidImage(ImageBitmap(8, 8), 8f, 8f)
    ComposeVisualFixture(1440, 650) {
          ProjectSummaryPane(
              overview,
              project,
              {},
              diagramRender = {
                renders++
                image
              })
        }
        .use { fixture ->
          fixture.render()
          fixture.revealText("Expand diagram", "summary-scroll")
          fixture.awaitDescription("Expand Architecture diagram", "Preview")
          fixture.clickDescription("Expand Architecture diagram")
          fixture.awaitDescription("Architecture diagram\n$source")
          fixture.clickDescription("Zoom in Architecture")
          assertEquals(1, renders)
          fixture.scrollBy(-100_000f, "summary-scroll")
          fixture.render()
          assertEquals(0, fixture.tagCount("summary-lower-composition"))
          overview = overview.copy(findingCounts = FindingCounts(verified = 2))
          fixture.resize(800, 650)
          fixture.awaitDescription("Close Architecture diagram")
          fixture.awaitDescription("Architecture diagram\n$source")
          assertTrue(fixture.hasText("125%"))
          assertEquals(1, renders)
          fixture.resize(1440, 650)
          fixture.render()
          fixture.awaitDescription("Close Architecture diagram")
          fixture.awaitDescription("Architecture diagram\n$source")
          assertEquals(1, renders)
        }
  }

  @Test
  fun replacingSummaryOwnerCancelsOldRenderAndNeverDisplaysItsLateImage() {
    val project = resultProjectFixture()
    val old = "flowchart LR\n A --> B"
    val next = "flowchart LR\n C --> D"
    var overview by
        androidx.compose.runtime.mutableStateOf(
            ProjectOverview(
                projectId = project.projectId,
                projectRevision = project.projectRevision,
                analysis = StructuredProjectAnalysis(status = "fresh", architecture = old)))
    val oldGate = CompletableDeferred<MermaidImage>()
    val canceled = CompletableDeferred<Unit>()
    val oldImage = MermaidImage(ImageBitmap(8, 8), 8f, 8f)
    val newImage = MermaidImage(ImageBitmap(12, 12), 12f, 12f)
    val rendered = mutableListOf<String>()
    try {
      ComposeVisualFixture(1000, 1400) {
            ProjectSummaryPane(
                overview,
                project,
                {},
                diagramRender = { source ->
                  rendered += source
                  if (source == old) {
                    try {
                      oldGate.await()
                    } catch (exception: CancellationException) {
                      canceled.complete(Unit)
                      withContext(NonCancellable) { oldGate.await() }
                    }
                  } else newImage
                })
          }
          .use { fixture ->
            fixture.awaitDescription("Expand Architecture diagram", "Rendering diagram")
            overview = overview.copy(analysis = overview.analysis.copy(architecture = next))
            fixture.awaitDescription("Expand Architecture diagram", "Preview")
            runBlocking { canceled.await() }
            fixture.clickDescription("Expand Architecture diagram")
            fixture.awaitDescription("Architecture diagram\n$next")
            oldGate.complete(oldImage)
            fixture.render()
            assertFalse(fixture.hasDescription("Architecture diagram\n$old"))
            assertFalse(fixture.hasText("Diagram unavailable:"))
            assertEquals(listOf(old, next), rendered)
          }
    } finally {
      oldGate.complete(oldImage)
    }
  }

  @Test
  fun flowSlotsRetainIndependentResultsAndResetWhenReplacedOrReordered() {
    val project = resultProjectFixture()
    val a = "flowchart LR\n A --> B"
    val b = "flowchart LR\n C --> D"
    val c = "flowchart LR\n E --> F"
    var overview by
        androidx.compose.runtime.mutableStateOf(
            ProjectOverview(
                projectId = project.projectId,
                projectRevision = project.projectRevision,
                analysis = StructuredProjectAnalysis(status = "fresh", flows = listOf(a, b))))
    val counts = mutableMapOf<String, Int>()
    val image = MermaidImage(ImageBitmap(8, 8), 8f, 8f)
    ComposeVisualFixture(1000, 1800) {
          ProjectSummaryPane(
              overview,
              project,
              {},
              diagramRender = { source ->
                counts[source] = (counts[source] ?: 0) + 1
                image
              })
        }
        .use { fixture ->
          fixture.awaitDescription("Expand Flow 1 diagram", "Preview")
          fixture.awaitDescription("Expand Flow 2 diagram", "Preview")
          fixture.clickDescription("Expand Flow 1 diagram")
          fixture.awaitDescription("Flow 1 diagram\n$a")
          fixture.clickDescription("Zoom in Flow 1")
          assertEquals(mapOf(a to 1, b to 1), counts)
          overview = overview.copy(analysis = overview.analysis.copy(flows = listOf(a, c)))
          fixture.awaitDescription("Expand Flow 2 diagram", "Preview")
          fixture.render()
          assertTrue(fixture.hasText("125%"))
          assertEquals(mapOf(a to 1, b to 1, c to 1), counts)
          overview = overview.copy(analysis = overview.analysis.copy(flows = listOf(c, a)))
          fixture.awaitDescription("Expand Flow 1 diagram", "Preview")
          fixture.awaitDescription("Expand Flow 2 diagram", "Preview")
          fixture.render()
          assertEquals(mapOf(a to 2, b to 1, c to 2), counts)
          assertFalse(fixture.hasText("125%"))
        }
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

    var renders = 0
    val image = MermaidImage(ImageBitmap(8, 8), 8f, 8f)
    ComposeVisualFixture(1000, 760) {
          ProjectSummaryPane(
              currentOverview,
              currentProject,
              {},
              diagramRender = {
                renders++
                image
              })
        }
        .use { fixture ->
          fixture.awaitDescription("Expand Architecture diagram", "Preview")
          fixture.clickDescription("Expand Architecture diagram")
          fixture.awaitDescription("Architecture diagram\n$source")
          fixture.clickDescription("Zoom in Architecture")
          fixture.render()
          assertTrue(fixture.hasText("125%"))
          assertEquals(1, renders)
          fixture.clickText("Mermaid source")
          assertTrue(fixture.tryClick("Expand More insight"))
          fixture.render()
          assertTrue(fixture.hasText("Trade-off or failure mode"))

          currentProject = currentProject.copy(projectId = "second", projectRevision = "two")
          currentOverview = currentOverview.copy(projectId = "second", projectRevision = "two")
          fixture.awaitDescription("Expand Architecture diagram", "Preview")
          assertFalse(fixture.hasText("Mermaid source"))
          fixture.clickDescription("Expand Architecture diagram")
          fixture.awaitDescription("Architecture diagram\n$source")
          assertTrue(fixture.hasText("100%"))
          assertEquals("Collapsed", fixture.stateDescription("More insight"))
          assertFalse(fixture.hasText("Trade-off or failure mode"))
          assertEquals(2, renders)
          currentProject = currentProject.copy(projectRevision = "three")
          currentOverview = currentOverview.copy(projectRevision = "three")
          fixture.awaitDescription("Expand Architecture diagram", "Preview")
          fixture.clickDescription("Expand Architecture diagram")
          fixture.awaitDescription("Architecture diagram\n$source")
          assertEquals(3, renders)
          assertTrue(fixture.hasText("100%"))
        }
  }

  @Test
  fun summaryNarrativeDisclosuresSurviveLazyScrollAndResetOnlyForTheirOwners() {
    val source = "flowchart TD\n A[Client] --> B[Service]"
    val insight =
        EngineeringInsight(
            mechanism = "Mechanism one.",
            whyItMattersHere = "Local evidence.",
            tradeoffOrFailureMode = "Trade-off one.",
            transferableLesson = "Lesson one.")
    var project by
        androidx.compose.runtime.mutableStateOf(
            resultProjectFixture().copy(projectId = "first", projectRevision = "one"))
    var overview by
        androidx.compose.runtime.mutableStateOf(
            ProjectOverview(
                projectId = "first",
                projectRevision = "one",
                analysis =
                    StructuredProjectAnalysis(
                        status = "fresh",
                        purpose = "Project context. ".repeat(400),
                        architecture = source,
                        engineeringInsight = insight)))
    var unrelated by androidx.compose.runtime.mutableStateOf(0)
    val navigations = mutableListOf<Workspace>()
    val actions =
        AnalysisWorkspaceActions(
            { _, _ -> error("Inspection requested analysis preview") },
            { error("Inspection requested run control") },
            { error("Inspection requested run control") },
            { error("Inspection requested run control") },
            { error("Inspection requested run control") })
    ComposeVisualFixture(1000, 650) {
          unrelated // Force recomposition without changing the narrative owner.
          ProjectSummaryPane(overview, project, navigations::add, analysisActions = actions)
        }
        .use { fixture ->
          fun reveal() {
            fixture.revealText("More insight")
            fixture.render()
          }
          fun expand() {
            reveal()
            fixture.clickDescription("Expand Architecture diagram")
            fixture.awaitDescription("Architecture diagram\n$source")
            fixture.clickDescription("Zoom in Architecture")
            fixture.clickText("Mermaid source")
            assertTrue(fixture.tryClick("Expand More insight"))
            fixture.render()
            assertTrue(fixture.hasText("Trade-off one."))
          }
          fixture.render()
          expand()
          unrelated++
          fixture.render()
          assertEquals("Expanded", fixture.stateDescription("More insight"))
          fixture.scrollBy(-100_000f, "summary-scroll")
          fixture.render()
          assertEquals(0, fixture.tagCount("summary-lower-composition"))
          reveal()
          assertEquals("Expanded", fixture.stateDescription("More insight"))
          assertTrue(fixture.hasText("Trade-off one."))
          assertTrue(fixture.hasText("Lesson one."))
          assertTrue(fixture.hasText("Hide Mermaid"))
          assertTrue(fixture.hasText("125%"))
          assertTrue(navigations.isEmpty())

          overview =
              overview.copy(
                  analysis =
                      overview.analysis.copy(
                          engineeringInsight =
                              insight.copy(tradeoffOrFailureMode = "Trade-off two.")))
          fixture.render()
          assertEquals("Collapsed", fixture.stateDescription("More insight"))
          assertTrue(fixture.hasText("Mechanism one."))
          assertTrue(fixture.hasText("Local evidence."))
          assertTrue(fixture.hasText("125%")) // Changing insight does not reset architecture.
          assertTrue(fixture.tryClick("Expand More insight"))
          fixture.render()
          assertTrue(fixture.hasText("Trade-off two."))
          assertTrue(fixture.hasText("Lesson one."))

          overview =
              overview.copy(
                  analysis =
                      overview.analysis.copy(architecture = source.replace("Service", "Store")))
          fixture.render()
          fixture.awaitDescription("Expand Architecture diagram", "Preview")
          assertEquals("Expanded", fixture.stateDescription("More insight"))
          fixture.clickDescription("Expand Architecture diagram")
          fixture.awaitDescription("Architecture diagram\n${source.replace("Service", "Store")}")
          project = project.copy(projectRevision = "two")
          overview = overview.copy(projectRevision = "two")
          fixture.render()
          reveal()
          fixture.awaitDescription("Expand Architecture diagram", "Preview")
          assertEquals("Collapsed", fixture.stateDescription("More insight"))
          fixture.clickDescription("Expand Architecture diagram")
          fixture.awaitDescription("Architecture diagram\n${source.replace("Service", "Store")}")
          assertTrue(fixture.tryClick("Expand More insight"))
          fixture.render()
          project = project.copy(projectId = "second")
          overview = overview.copy(projectId = "second")
          fixture.render()
          reveal()
          fixture.awaitDescription("Expand Architecture diagram", "Preview")
          assertEquals("Collapsed", fixture.stateDescription("More insight"))
          assertTrue(navigations.isEmpty())
        }
  }

  @Test
  fun resizingAcrossNarrativeBreakpointRetainsDisclosureAndLocalNavigation() {
    val project = resultProjectFixture()
    val overview =
        ProjectOverview(
            projectId = project.projectId,
            projectRevision = project.projectRevision,
            analysis =
                StructuredProjectAnalysis(
                    status = "fresh",
                    architecture = "flowchart TD\n A[Client] --> B[Service]",
                    engineeringInsight =
                        EngineeringInsight(
                            mechanism = "Mechanism",
                            whyItMattersHere = "Local reason",
                            tradeoffOrFailureMode = "Trade-off")))
    val navigations = mutableListOf<Workspace>()
    ComposeVisualFixture(1440, 900, 1.5f) {
          ProjectSummaryPane(overview, project, navigations::add)
        }
        .use { fixture ->
          fixture.render()
          fixture.clickDescription("Expand Architecture diagram")
          assertTrue(fixture.tryClick("Expand More insight"))
          fixture.render()
          assertEquals("Expanded", fixture.stateDescription("More insight"))
          fixture.resize(800, 650)
          fixture.render()
          fixture.revealText("More insight")
          assertEquals("Expanded", fixture.stateDescription("More insight"))
          assertTrue(fixture.hasText("Trade-off"))
          fixture.revealText("Open Editor")
          fixture.clickText("Open Editor")
          assertEquals(listOf(Workspace.Editor), navigations)
          fixture.resize(1440, 900)
          fixture.render()
          fixture.revealText("More insight")
          assertEquals("Expanded", fixture.stateDescription("More insight"))
        }
  }

  @Test
  fun narrativeSectionsOmitMissingArchitectureWithoutLosingSavedContent() {
    val project = resultProjectFixture()
    val overview =
        ProjectOverview(
            projectId = project.projectId,
            projectRevision = project.projectRevision,
            analysis =
                StructuredProjectAnalysis(
                    status = "fresh",
                    components =
                        listOf(
                            "internal/api (API): Handles requests.",
                            "internal/app (Workflow): Coordinates changes."),
                    flows = listOf("First flow", "Second flow"),
                    engineeringInsight =
                        EngineeringInsight(
                            mechanism = "Keep the boundary explicit.",
                            whyItMattersHere = "Project changes need review.",
                            tradeoffOrFailureMode = "Review takes time.")))
    val presentation = projectSummaryPresentation(overview, project)
    assertEquals(listOf("Packages / modules", "Flows"), presentation.details.map { it.title })
    ComposeVisualFixture(1000, 2400) { ProjectSummaryPane(overview, project, {}) }
        .use { fixture ->
          fixture.render()
          assertEquals(0, fixture.tagCount("summary-architecture"))
          assertEquals(1, fixture.tagCount("summary-insight"))
          assertEquals(1, fixture.tagCount("summary-modules"))
          assertEquals(1, fixture.tagCount("summary-flows"))
          listOf(
                  "Keep the boundary explicit.",
                  "Project changes need review.",
                  "API",
                  "internal/api",
                  "Handles requests.",
                  "Workflow",
                  "internal/app",
                  "Coordinates changes.",
                  "First flow",
                  "Second flow")
              .forEach { assertTrue(fixture.hasText(it), it) }
          assertTrue(fixture.tryClick("Expand More insight"))
          fixture.render()
          assertTrue(fixture.hasText("Review takes time."))
        }
    ComposeVisualFixture(1000, 1800) {
          ProjectSummaryPane(
              overview.copy(
                  analysis =
                      overview.analysis.copy(engineeringInsight = null, components = emptyList())),
              project,
              {})
        }
        .use { fixture ->
          fixture.render()
          assertEquals(0, fixture.tagCount("summary-insight"))
          assertEquals(0, fixture.tagCount("summary-modules"))
          assertEquals(1, fixture.tagCount("summary-flows"))
        }
  }

  @Test
  fun changeLifecycleFollowsSavedProjectFlowsWithSharedSectionGap() {
    val project = resultProjectFixture()
    val overview =
        ProjectOverview(
            projectId = project.projectId,
            projectRevision = project.projectRevision,
            analysis =
                StructuredProjectAnalysis(status = "fresh", flows = listOf("Saved project flow")))
    val navigations = mutableListOf<Workspace>()
    ComposeVisualFixture(1000, 1800) { ProjectSummaryPane(overview, project, navigations::add) }
        .use { fixture ->
          fixture.render()
          assertEquals(1, fixture.tagCount("summary-flows"))
          assertEquals(1, fixture.tagCount("summary-change-lifecycle"))
          assertTrue(fixture.hasText("Saved project flow"))
          assertTrue(fixture.hasText("Change lifecycle"))
          assertTrue(fixture.hasText("Mini-Orca editing workflow (not a project Flow)"))
          assertTrue(fixture.hasText("Request → Draft → Validate → Checks → Review → Apply"))
          assertTrue(
              fixture.hasText(
                  "Edit only an isolated declaration/import draft. Apply is an explicit, guarded one-file source change; Undo is guarded and available only when the change is still eligible."))
          val flows = fixture.taggedBounds("summary-flows")
          val lifecycle = fixture.taggedBounds("summary-change-lifecycle")
          assertEquals(MiniOrcaSpacing.section.value, lifecycle.top - flows.bottom, 1f)
          assertFalse(fixture.requestFocus("Request → Draft → Validate → Checks → Review → Apply"))
          fixture.clickText("Open Editor")
          assertEquals(listOf(Workspace.Editor), navigations)
        }
  }

  @Test
  fun changeLifecycleRemainsReachableWithoutOptionalProjectNarrative() {
    val destinations = mutableListOf<Workspace>()
    val project = resultProjectFixture()
    ComposeVisualFixture(1000, 1800) { ProjectSummaryPane(null, project, destinations::add) }
        .use { fixture ->
          fixture.render()
          val coverage = fixture.taggedBounds("summary-coverage-results")
          val findings = fixture.taggedBounds("summary-selected-findings")
          val lifecycle = fixture.taggedBounds("summary-change-lifecycle")
          assertEquals(MiniOrcaSpacing.section.value, findings.top - coverage.bottom, 1f)
          assertEquals(MiniOrcaSpacing.section.value, lifecycle.top - findings.bottom, 1f)
          assertEquals(0, fixture.tagCount("summary-lower-composition"))
        }
    ComposeVisualFixture(800, 650, 1.5f) { ProjectSummaryPane(null, project, destinations::add) }
        .use { fixture ->
          fixture.render()
          listOf("summary-architecture", "summary-insight", "summary-modules", "summary-flows")
              .forEach { assertEquals(0, fixture.tagCount(it)) }
          fixture.revealText("Open Editor")
          fixture.render()
          assertEquals(1, fixture.tagCount("summary-change-lifecycle"))
          assertTrue(fixture.hasText("Change lifecycle"))
          fixture.clickText("Open Editor")
          assertEquals(listOf(Workspace.Editor), destinations)
        }
  }

  @Test
  fun selectedFindingsRendersWithoutNarrativeAndKeepsLoadedScopeAndLocalRoutes() {
    val page = resultPageFixture("bugs")
    val app =
        DesktopState(
            projectState = ProjectWorkspaceState(page.project),
            analysisRun =
                ProjectAnalysisRunState(
                    run = page.run, sections = mapOf(AnalysisResultKey("bugs") to page.section)))
    val preview = summaryFindingPreview(app)
    val navigations = mutableListOf<Workspace>()
    val activations = mutableListOf<SummaryFindingTarget>()
    ComposeVisualFixture(1000, 1600) {
          ProjectSummaryPane(
              null,
              page.project,
              navigations::add,
              findingState = app,
              onFindingSelected = activations::add)
        }
        .use { fixture ->
          fixture.render()
          assertEquals(0, fixture.tagCount("summary-lower-composition"))
          assertEquals(1, fixture.tagCount("summary-selected-findings"))
          assertTrue(
              fixture.hasText(
                  "Showing ${preview.rows.size} of ${preview.loadedCount} loaded findings · not category totals"))
          val row = preview.rows.first()
          assertTrue(fixture.hasText(row.title))
          assertTrue(
              fixture.hasText(
                  row.origin + " · " + row.materialState.ifBlank { "Material state unavailable" }))
          val label = "Inspect ${row.title} in Bugs results at ${row.location}"
          fixture.clickDescription(label)
          assertEquals(listOf(row.target), activations)
          listOf(Workspace.Bugs, Workspace.Performance, Workspace.Security).forEach { type ->
            fixture.clickText("All ${type.name} results")
          }
          assertEquals(
              listOf(Workspace.Bugs, Workspace.Performance, Workspace.Security), navigations)
        }
    val empty =
        summaryFindingPreview(DesktopState(projectState = ProjectWorkspaceState(page.project)))
    assertEquals(0, empty.loadedCount)
    assertTrue(
        summaryFindingEmptyMessage(
                DesktopState(projectState = ProjectWorkspaceState(page.project)), empty)
            .contains("No findings loaded"))
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
    val overview =
        ProjectOverview(
            projectId = project.projectId,
            projectRevision = project.projectRevision,
            analysis = StructuredProjectAnalysis(status = "fresh", purpose = "Saved purpose"))
    val updated = projectSummaryPresentation(overview, project)
    assertEquals("unknown", updated.summaryStatus)
    assertEquals(SecondaryText, summaryAnalysisTint(updated.summaryStatus))
    val outdated =
        projectSummaryPresentation(
            overview.copy(analysisCoverage = AnalysisCoverage(total = 1, stale = 1)), project)
    assertEquals(Warning, summaryAnalysisTint(outdated.summaryStatus))
    val failed =
        projectSummaryPresentation(
            overview.copy(analysisCoverage = AnalysisCoverage(total = 2, stale = 1, failed = 1)),
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
  fun coverageProjectionDistinguishesDefaultOverviewEmptySelectionAndZeroCurrent() {
    val project = analysisProjectFixture()
    val default = projectSummaryPresentation(ProjectOverview(), null)
    assertEquals(SummaryCoverageProjection.Unavailable, default.coverage)
    assertEquals("unknown", default.summaryStatus)
    val empty =
        projectSummaryPresentation(
            null,
            project,
            fileSelection = selectionFixture().copy(excludedPaths = listOf("helper.go", "main.go")))
    assertEquals(
        SummaryCoverageProjection.Empty(SummaryCoverageOwner("project", "revision", "selection")),
        empty.coverage)
    assertEquals("excluded", empty.summaryStatus)
    val missing =
        projectSummaryPresentation(
            null,
            project,
            fileSelection =
                selectionFixture().copy(files = listOf(selectionFixture().files.last())))
    val known = missing.coverage as SummaryCoverageProjection.Known
    assertEquals(1, known.total)
    assertEquals(0, known.saved.fresh)
    assertEquals(AnalysisCoverageBucket.NotAnalyzed, known.buckets.single().id)
    assertEquals("missing", missing.summaryStatus)
  }

  @Test
  fun aggregateCoveragePreservesDenominatorAndNeverBorrowsSelectionPaths() {
    val project = analysisProjectFixture()
    val overview =
        ProjectOverview(
            projectId = project.projectId,
            projectRevision = project.projectRevision,
            analysisCoverage = AnalysisCoverage(total = 7, fresh = 2, failed = 1))
    val foreign = selectionFixture().copy(projectRevision = "old")
    val summary = projectSummaryPresentation(overview, project, fileSelection = foreign)
    val known = summary.coverage as SummaryCoverageProjection.Known
    assertEquals(SummaryCoverageOwner("project", "revision"), known.owner)
    assertEquals(7, known.total)
    assertEquals(listOf(2, 1, 4), known.buckets.map { it.count })
    assertEquals(
        listOf("Up to date", "Failed", "Unavailable"), summary.coverageMetrics.map { it.label })
    assertTrue(known.buckets.all { it.paths == SummaryCoveragePaths.Unavailable })
    assertEquals(4, known.saved.unavailable)
    assertFalse(summary.analysisMessage.contains("Selected files: Up to date"))
    val matching = projectSummaryPresentation(overview, project, fileSelection = selectionFixture())
    assertEquals(2, (matching.coverage as SummaryCoverageProjection.Known).total)
    assertTrue(matching.coverage.buckets.all { it.paths is SummaryCoveragePaths.Selected })
  }

  @Test
  fun invalidAggregatesDoNotInventDenominatorsOrRows() {
    val project = analysisProjectFixture()
    val invalid =
        listOf(
            AnalysisCoverage(total = 0, fresh = 1),
            AnalysisCoverage(total = -1),
            AnalysisCoverage(total = 2, fresh = -1, stale = 2),
            AnalysisCoverage(total = 1, failed = 2),
            AnalysisCoverage(total = Int.MAX_VALUE, fresh = Int.MAX_VALUE, stale = Int.MAX_VALUE))
    invalid.forEach { counts ->
      val summary =
          projectSummaryPresentation(
              ProjectOverview("project", "revision", analysisCoverage = counts),
              project,
              fileSelection = selectionFixture().copy(projectId = "other"))
      assertEquals(SummaryCoverageProjection.Unavailable, summary.coverage, "$counts")
      assertEquals("unknown", summary.summaryStatus, "$counts")
      assertTrue(summary.coverageMetrics.all { it.value == null }, "$counts")
      assertFalse(summary.analysisMessage.contains("Selected files:"), "$counts")
    }
    val large =
        summaryCoverageProjection(
            ProjectOverview(
                "project",
                "revision",
                analysisCoverage =
                    AnalysisCoverage(total = Int.MAX_VALUE, fresh = Int.MAX_VALUE - 1)),
            project,
            null)
            as SummaryCoverageProjection.Known
    assertEquals(Int.MAX_VALUE, large.total)
    assertEquals(1, large.saved.unavailable)
    assertEquals(listOf(Int.MAX_VALUE - 1, 1), large.buckets.map { it.count })
  }

  @Test
  fun selectionProjectionKeepsSavedRowsAndReinclusionStatusTogether() {
    val project = analysisProjectFixture()
    val stale =
        selectionFixture()
            .files
            .last()
            .copy(stages = selectionStageFixture("stale", "Source changed."))
    val selection =
        selectionFixture()
            .copy(
                files = listOf(selectionFixture().files[1], stale),
                excludedPaths = listOf("main.go", "absent.go"))
    val before =
        summaryCoverageProjection(null, project, selection) as SummaryCoverageProjection.Known
    assertEquals(1, before.total)
    val included =
        summaryCoverageProjection(null, project, selection.copy(excludedPaths = emptyList()))
            as SummaryCoverageProjection.Known
    assertEquals(2, included.total)
    assertEquals(
        listOf(AnalysisCoverageBucket.UpToDate, AnalysisCoverageBucket.Outdated),
        included.buckets.map { it.id })
    included.buckets.forEach { bucket ->
      val rows = (bucket.paths as SummaryCoveragePaths.Selected).rows
      assertEquals(bucket.count, rows.size)
    }
    val outdated = (included.buckets.last().paths as SummaryCoveragePaths.Selected).rows.single()
    assertEquals("main.go", outdated.file.path)
    assertEquals(AnalysisFileSyncStatus.Stale, outdated.status)
    assertTrue(outdated.explanation.contains("Source changed."))
  }

  @Test
  fun fileLedgerSortsSavedCoverageAcrossBucketsBeforeBounding() {
    val project = analysisProjectFixture()
    val files =
        listOf(
            AnalysisSelectableFile("z/last.go", "", selectionStageFixture("fresh", "Current")),
            AnalysisSelectableFile("B/second.go", "", selectionStageFixture("stale", "Changed")),
            AnalysisSelectableFile("a/third.go", "", selectionStageFixture("failed", "Failed")),
            AnalysisSelectableFile("A/first.go", "", selectionStageFixture("missing", "Missing")),
            AnalysisSelectableFile("c/fourth.go", "", selectionStageFixture("fresh", "Current")),
            AnalysisSelectableFile("0-excluded.go", "", selectionStageFixture("fresh", "Current")),
            AnalysisSelectableFile(".env", "Excluded by policy"))
    val selection =
        selectionFixture().copy(files = files, excludedPaths = listOf("0-excluded.go", "absent.go"))
    for (ordered in listOf(files, files.reversed())) {
      val confirmed = selection.copy(files = ordered)
      val summary = projectSummaryPresentation(null, project, fileSelection = confirmed)
      val ledger = summary.fileLedger as SummaryFileLedger.Selected
      val coverage = summary.coverage as SummaryCoverageProjection.Known
      assertEquals(coverage.owner, ledger.owner)
      assertEquals(coverage.total, ledger.totalSelected)
      assertEquals(5, ledger.totalSelected)
      assertEquals(
          listOf("A/first.go", "B/second.go", "a/third.go"), ledger.rows.map { it.file.path })
      assertEquals(
          listOf(
              AnalysisFileSyncStatus.Missing,
              AnalysisFileSyncStatus.Stale,
              AnalysisFileSyncStatus.Failed),
          ledger.rows.map { it.status })
      ledger.rows.forEach { row ->
        assertTrue(
            coverage.buckets.any { bucket ->
              (bucket.paths as SummaryCoveragePaths.Selected).rows.contains(row)
            })
      }
      assertTrue(ledger.rows[1].explanation.contains("Changed"))
      assertEquals(null, ledger.selectionNotice)
    }
  }

  @Test
  fun fileLedgerSeparatesAggregateEmptyAndUnavailableSelection() {
    val project = analysisProjectFixture()
    val overview =
        ProjectOverview(
            "project", "revision", analysisCoverage = AnalysisCoverage(total = 6, fresh = 2))
    val noSelection = projectSummaryPresentation(overview, project)
    assertEquals(SummaryFileLedger.AggregateOnly(6, null), noSelection.fileLedger)
    val readFailure =
        AnalysisSelectionState(error = "Timed out.", failure = AnalysisSelectionFailure.Read)
    val aggregateFailure =
        projectSummaryPresentation(overview, project, selectionState = readFailure)
    assertEquals(
        SummaryFileLedger.AggregateOnly(6, aggregateFailure.selectionNotice),
        aggregateFailure.fileLedger)
    assertTrue(aggregateFailure.fileLedger.selectionNotice!!.contains("No confirmed selection"))
    val unavailable = projectSummaryPresentation(null, project, selectionState = readFailure)
    assertEquals(SummaryFileLedger.Unavailable(unavailable.selectionNotice), unavailable.fileLedger)
    assertEquals(
        SummaryFileLedger.Unavailable(null), projectSummaryPresentation(null, project).fileLedger)
    val excluded =
        selectionFixture().copy(excludedPaths = listOf("main.go", "helper.go", "absent.go"))
    val empty = projectSummaryPresentation(overview, project, fileSelection = excluded)
    assertEquals(
        SummaryFileLedger.Empty(SummaryCoverageOwner("project", "revision", "selection"), null),
        empty.fileLedger)
    assertTrue(empty.coverage is SummaryCoverageProjection.Empty)
    val retainedEmpty =
        projectSummaryPresentation(
            overview, project, selectionState = AnalysisSelectionState(excluded, saving = true))
    assertEquals(
        SummaryFileLedger.Empty(
            SummaryCoverageOwner("project", "revision", "selection"),
            retainedEmpty.selectionNotice),
        retainedEmpty.fileLedger)
    assertTrue(retainedEmpty.fileLedger.selectionNotice!!.contains("last confirmed selection"))
    val foreign =
        projectSummaryPresentation(
            overview, project, fileSelection = excluded.copy(projectRevision = "old"))
    assertEquals(SummaryFileLedger.AggregateOnly(6, null), foreign.fileLedger)
  }

  @Test
  fun fileLedgerRetainsConfirmedRowsAndNoticesWithoutBorrowingPendingEditsOrRunProgress() {
    val project = analysisProjectFixture()
    val confirmed = selectionFixture()
    val pending =
        confirmed.copy(
            files =
                listOf(
                    AnalysisSelectableFile(
                        "pending.go", "", selectionStageFixture("fresh", "Current"))))
    val run =
        analysisRunFixture()
            .copy(
                status = "running",
                files =
                    listOf(
                        AnalysisRunFile(
                            "main.go",
                            "base",
                            "Go",
                            listOf(AnalysisStageProgress("semantic", "pending", 0, false)))))
    assertEquals(AnalysisFileSyncStatus.Pending, analysisFileStatuses(confirmed, run).last().status)
    val finished =
        run.copy(
            files =
                run.files.map {
                  it.copy(stages = listOf(AnalysisStageProgress("semantic", "completed", 1, false)))
                })
    assertEquals(
        AnalysisFileSyncStatus.Finished, analysisFileStatuses(confirmed, finished).last().status)
    val states =
        listOf(
            AnalysisSelectionState(confirmed, loading = true),
            AnalysisSelectionState(confirmed, saving = true),
            AnalysisSelectionState(
                confirmed, error = "Read failed.", failure = AnalysisSelectionFailure.Read),
            AnalysisSelectionState(
                confirmed, error = "Save failed.", failure = AnalysisSelectionFailure.Save))
    states.forEach { state ->
      listOf(run, finished).forEach { overlay ->
        val summary =
            projectSummaryPresentation(
                null, project, overlay, fileSelection = pending, selectionState = state)
        val ledger = summary.fileLedger as SummaryFileLedger.Selected
        assertEquals(listOf("helper.go", "main.go"), ledger.rows.map { it.file.path })
        assertEquals(
            listOf(AnalysisFileSyncStatus.Updated, AnalysisFileSyncStatus.Missing),
            ledger.rows.map { it.status })
        assertEquals(2, ledger.totalSelected)
        assertEquals(summary.selectionNotice, ledger.selectionNotice)
        assertTrue(ledger.selectionNotice!!.contains("last confirmed selection"))
        assertTrue(ledger.rows.none { it.file.path == "pending.go" })
      }
    }
  }

  @Test
  fun fileEvidenceRendersBoundedSavedRowsAndAllFilesOnlyNavigates() {
    val project = analysisProjectFixture()
    val longPath = "src/" + "日本語/long-directory/".repeat(8) + "last.go"
    val files =
        listOf(
            AnalysisSelectableFile("z.go", "", selectionStageFixture("fresh", "Current")),
            AnalysisSelectableFile("b.go", "", selectionStageFixture("stale", "Changed")),
            AnalysisSelectableFile(longPath, "", selectionStageFixture("failed", "Failed read")),
            AnalysisSelectableFile("a.go", "", selectionStageFixture("missing", "Not started")),
            AnalysisSelectableFile("excluded.go", "", selectionStageFixture("fresh", "Current")))
    val selection = selectionFixture().copy(files = files, excludedPaths = listOf("excluded.go"))
    val state =
        ProjectAnalysisRunState(
            fileSelection =
                AnalysisSelectionState(
                    selection, error = "Disk unavailable", failure = AnalysisSelectionFailure.Read))
    val destinations = mutableListOf<Workspace>()
    var requests = 0
    val actions =
        AnalysisWorkspaceActions(
            { _, _ -> requests++ },
            { requests++ },
            { requests++ },
            { requests++ },
            { requests++ },
            saveSelection = { requests++ })
    ComposeVisualFixture(1440, 1100) {
          ProjectSummaryPane(
              null, project, destinations::add, analysisState = state, analysisActions = actions)
        }
        .use { fixture ->
          fixture.render()
          fixture.revealText("File evidence")
          assertTrue(fixture.hasText("Showing 3 of 4 selected files · saved status"))
          assertTrue(
              fixture.hasText(
                  "File selection load failed · Showing last confirmed selection. Disk unavailable"))
          for (path in listOf("a.go", "b.go", longPath)) assertTrue(fixture.hasText(path))
          assertFalse(fixture.hasText("z.go"))
          assertFalse(fixture.hasText("excluded.go"))
          for (row in
              (projectSummaryPresentation(null, project, selectionState = state.fileSelection)
                      .fileLedger as SummaryFileLedger.Selected)
                  .rows) {
            assertTrue(fixture.hasText("${row.status.label} · ${row.explanation}"))
          }
          assertFalse(fixture.hasEditableText(withinTag = "summary-file-paths"))
          fixture.revealText("All files")
          fixture.clickText("All files")
          assertEquals(listOf(Workspace.Analysis), destinations)
          assertEquals(0, requests)
          assertEquals(selection, state.fileSelection.selection)
          assertTrue(fixture.requestFocus("All files"))
          fixture.pressKey(androidx.compose.ui.input.key.Key.Enter)
          assertEquals(listOf(Workspace.Analysis, Workspace.Analysis), destinations)
        }
  }

  @Test
  fun fileEvidenceSeparatesAggregateEmptyUnavailableAndFailedRead() {
    val project = analysisProjectFixture()
    val aggregate =
        ProjectOverview(
            "project", "revision", analysisCoverage = AnalysisCoverage(total = 6, fresh = 2))
    val empty = selectionFixture().copy(excludedPaths = listOf("helper.go", "main.go"))
    val scenarios =
        listOf(
            Triple(
                aggregate,
                AnalysisSelectionState(),
                "File paths unavailable · 6 files in saved aggregate coverage. Load a confirmed selection to inspect file evidence."),
            Triple(
                null,
                AnalysisSelectionState(empty),
                "No files selected in the confirmed selection."),
            Triple(
                null,
                AnalysisSelectionState(),
                "File evidence unavailable · no confirmed file selection or saved file paths."),
            Triple(
                null,
                AnalysisSelectionState(
                    error = "Read denied", failure = AnalysisSelectionFailure.Read),
                "File selection load failed · No confirmed selection available. Read denied"))
    scenarios.forEach { (overview, selection, expected) ->
      ComposeVisualFixture(800, 650) {
            ProjectSummaryPane(
                overview,
                project,
                {},
                analysisState = ProjectAnalysisRunState(fileSelection = selection))
          }
          .use { fixture ->
            fixture.render()
            fixture.revealText("File evidence")
            assertTrue(fixture.hasText(expected), expected)
            if (selection.error != null)
                assertTrue(
                    fixture.hasText(
                        "File evidence unavailable · no confirmed file selection or saved file paths."))
            assertTrue(fixture.hasText("All files"))
          }
    }
  }

  @Test
  fun coverageArcsUseValidatedDenominatorWithoutGapsOrInflatedSmallBuckets() {
    val project = analysisProjectFixture()
    val unknown = projectSummaryPresentation(null, project)
    assertTrue(summaryCoverageArcs(unknown.coverage).isEmpty())
    val empty =
        projectSummaryPresentation(
            null,
            project,
            fileSelection = selectionFixture().copy(excludedPaths = listOf("helper.go", "main.go")))
    assertTrue(summaryCoverageArcs(empty.coverage).isEmpty())
    val partial =
        projectSummaryPresentation(
            ProjectOverview(
                "project",
                "revision",
                analysisCoverage = AnalysisCoverage(total = 4, fresh = 1, failed = 1)),
            project)
    val arcs = summaryCoverageArcs(partial.coverage)
    assertEquals(
        listOf(
            AnalysisCoverageBucket.UpToDate,
            AnalysisCoverageBucket.Failed,
            AnalysisCoverageBucket.Unavailable),
        arcs.map { it.bucket })
    assertEquals(listOf(0.0, 90.0, 180.0), arcs.map { it.start })
    assertEquals(listOf(90.0, 90.0, 180.0), arcs.map { it.sweep })
    val allCurrent =
        summaryCoverageProjection(
            ProjectOverview(
                "project", "revision", analysisCoverage = AnalysisCoverage(total = 23, fresh = 23)),
            project,
            null)
    assertEquals(
        listOf(SummaryCoverageArc(AnalysisCoverageBucket.UpToDate, 0.0, 360.0)),
        summaryCoverageArcs(allCurrent))
    val tiny =
        summaryCoverageProjection(
            ProjectOverview(
                "project",
                "revision",
                analysisCoverage =
                    AnalysisCoverage(total = Int.MAX_VALUE, fresh = Int.MAX_VALUE - 1, failed = 1)),
            project,
            null)
    val tinyArcs = summaryCoverageArcs(tiny)
    assertEquals(2, tinyArcs.size)
    assertTrue(tinyArcs.last().sweep > 0.0)
    assertEquals("99%", summaryCoveragePercent(Int.MAX_VALUE - 1, Int.MAX_VALUE))
    assertEquals("100%", summaryCoveragePercent(23, 23))
    assertEquals("0%", summaryCoveragePercent(0, 23))
    assertEquals("25%", summaryCoveragePercent(1, 4))
    listOf(partial.coverage, allCurrent, tiny).forEach { coverage ->
      val boundaries = summaryCoverageArcs(coverage)
      boundaries.forEach { arc ->
        assertTrue(arc.start.isFinite() && arc.sweep.isFinite())
        assertTrue(arc.start >= 0.0 && arc.sweep >= 0.0)
        assertTrue(arc.start + arc.sweep <= 360.0)
      }
      boundaries.zipWithNext().forEach { (a, b) ->
        assertTrue(a.start + a.sweep <= b.start + 1e-12)
      }
    }
    assertTrue(
        summaryCoverageArcs(
                summaryCoverageProjection(
                    ProjectOverview(
                        "project",
                        "revision",
                        analysisCoverage = AnalysisCoverage(total = 0, fresh = 1)),
                    project,
                    null))
            .isEmpty())
  }

  @Test
  fun currentRunLifecycleRemainsVisibleAndForeignRunsCannotOverrideCoverage() {
    val project = resultProjectFixture()
    val overview =
        ProjectOverview(
            projectId = project.projectId,
            projectRevision = project.projectRevision,
            analysisCoverage = AnalysisCoverage(total = 1, fresh = 1))
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
  fun retainedSelectionIsQualifiedThroughLoadSaveAndFailureWithoutOptimisticCoverage() {
    val project = analysisProjectFixture()
    val selection = selectionFixture()
    val destinations = mutableListOf<Workspace>()
    var state by
        mutableStateOf(ProjectAnalysisRunState(fileSelection = AnalysisSelectionState(selection)))
    ComposeVisualFixture(1000, 760) {
          ProjectSummaryPane(null, project, destinations::add, analysisState = state)
        }
        .use { fixture ->
          fixture.render()
          fixture.clickVisibleDescription("Up to date, 1 file")
          fixture.render()
          assertTrue(fixture.hasText("helper.go"))
          for (pending in
              listOf(
                  AnalysisSelectionState(selection, loading = true),
                  AnalysisSelectionState(selection, saving = true),
                  AnalysisSelectionState(
                      selection,
                      error = "Refresh timed out.",
                      failure = AnalysisSelectionFailure.Read),
                  AnalysisSelectionState(
                      selection,
                      error = "Save rejected.",
                      failure = AnalysisSelectionFailure.Save))) {
            state = state.copy(fileSelection = pending)
            fixture.render()
            assertTrue(fixture.hasText("1 of 2 selected files are up to date"))
            assertTrue(fixture.hasText("50%"))
            assertTrue(fixture.hasText("helper.go"))
            assertTrue(fixture.hasText("Up to date · 1 of 2 selected files"))
            val notice =
                when {
                  pending.loading -> "Loading file selection · showing last confirmed selection."
                  pending.saving ->
                      "Saving file selection · showing last confirmed selection until the save succeeds."
                  pending.failure == AnalysisSelectionFailure.Read ->
                      "File selection load failed · Showing last confirmed selection. Refresh timed out."
                  else ->
                      "File selection save failed · Showing last confirmed selection. Save rejected."
                }
            assertTrue(fixture.hasText(notice))
          }
          assertEquals(emptyList(), destinations)
          state = state.copy(fileSelection = AnalysisSelectionState(selection))
          fixture.render()
          assertEquals(0, fixture.tagCount("summary-selection-notice"))
        }
  }

  @Test
  fun selectionFailureWithoutDataIsNotEmptySuccessAndKeepsRecoveryRoute() {
    val project = analysisProjectFixture()
    val destinations = mutableListOf<Workspace>()
    var selection by mutableStateOf(AnalysisSelectionState(loading = true))
    ComposeVisualFixture(800, 650) {
          ProjectSummaryPane(
              null,
              project,
              destinations::add,
              fileSelection = selectionFixture(),
              analysisState = ProjectAnalysisRunState(fileSelection = selection))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("File counts unavailable"))
          assertTrue(fixture.hasText("Loading file selection · no confirmed selection available."))
          selection =
              AnalysisSelectionState(
                  error = "Selection endpoint unavailable.",
                  failure = AnalysisSelectionFailure.Read)
          fixture.render()
          assertTrue(fixture.hasText("File counts unavailable"))
          assertTrue(
              fixture.hasText(
                  "File selection load failed · No confirmed selection available. Selection endpoint unavailable."))
          assertFalse(fixture.hasText("0 selected files"))
          assertFalse(fixture.hasText("helper.go"))
          fixture.clickText("View analysis")
          assertEquals(listOf(Workspace.Analysis), destinations)
        }
    val aggregate =
        ProjectOverview(
            project.projectId,
            project.projectRevision,
            analysisCoverage = AnalysisCoverage(total = 2, stale = 2))
    ComposeVisualFixture(800, 650) {
          ProjectSummaryPane(
              aggregate,
              project,
              {},
              analysisState = ProjectAnalysisRunState(fileSelection = selection))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("0 of 2 selected files are up to date"))
          assertTrue(
              fixture.hasText(
                  "File selection load failed · No confirmed selection available. Selection endpoint unavailable."))
          fixture.clickVisibleDescription("Outdated, 2 files")
          fixture.render()
          assertTrue(
              fixture.hasText(
                  "File paths are unavailable for aggregate coverage. View analysis for file scope."))
          assertFalse(fixture.hasText("helper.go"))
        }
  }

  @Test
  fun runLifecycleIsSeparateFromSavedCoverageAndForeignEvidenceCannotLeak() {
    val project = analysisProjectFixture()
    val allCurrent = selectionFixture().copy(excludedPaths = listOf("main.go"))
    var state by
        mutableStateOf(ProjectAnalysisRunState(fileSelection = AnalysisSelectionState(allCurrent)))
    ComposeVisualFixture(1000, 760) {
          ProjectSummaryPane(null, project, {}, run = state.run, analysisState = state)
        }
        .use { fixture ->
          fixture.render()
          fixture.clickVisibleDescription("Up to date, 1 file")
          for (status in listOf("queued", "running", "completed")) {
            val run = analysisRunFixture().copy(status = status)
            state = state.copy(run = run)
            fixture.render()
            assertTrue(fixture.hasText("100%"))
            assertTrue(fixture.hasText("1 of 1 selected files are up to date"))
            assertTrue(fixture.hasText("helper.go"))
            assertTrue(
                fixture.hasText(
                    "${if (status == "completed") "Last" else "Current"} analysis run: ${analysisStatusLabel(status)} · separate from saved coverage."))
            assertEquals(1, fixture.tagCount("summary-analysis-run-strip"))
          }
          state = state.copy(fileSelection = AnalysisSelectionState(selectionFixture()))
          for (status in listOf("queued", "running", "completed")) {
            state = state.copy(run = analysisRunFixture().copy(status = status))
            fixture.render()
            assertTrue(fixture.hasText("1 of 2 selected files are up to date"))
            assertTrue(fixture.hasText("50%"))
            assertTrue(fixture.hasText("helper.go"))
          }
          state =
              state.copy(
                  run =
                      analysisRunFixture()
                          .copy(
                              identity =
                                  analysisRunFixture().identity.copy(projectRevision = "other")),
                  fileSelection =
                      AnalysisSelectionState(
                          selectionFixture().copy(projectId = "other"),
                          error = "Other selection failed.",
                          failure = AnalysisSelectionFailure.Read))
          fixture.render()
          assertTrue(fixture.hasText("File counts unavailable"))
          assertFalse(fixture.hasText("helper.go"))
          assertEquals(0, fixture.tagCount("summary-coverage-inspection"))
          assertEquals(0, fixture.tagCount("summary-coverage-run-status"))
          assertEquals(0, fixture.tagCount("summary-analysis-run-strip"))
          assertFalse(fixture.hasText("Other selection failed."))
        }
  }

  @Test
  fun compactRunUsesCapturedProgressWithoutChangingSavedCoverageOrDispatchingInspection() {
    val project = analysisProjectFixture()
    val path = "src/" + "long-directory/".repeat(12) + "file.go"
    val base = analysisRunFixture()
    val planFile =
        AnalysisPlannedFile(
            path,
            "hash",
            "Go",
            20,
            listOf(AnalysisStagePlan("semantic", true, false, maxModelRequests = 0)))
    val planned = base.copy(plan = base.plan.copy(files = listOf(planFile)))
    val reported =
        AnalysisRunFile(
            path, "hash", "Go", listOf(AnalysisStageProgress("semantic", "partial", 1, false)))
    var state by
        mutableStateOf(
            ProjectAnalysisRunState(
                run = planned.copy(status = "running", files = listOf(reported))))
    val coverage =
        ProjectOverview(
            project.projectId,
            project.projectRevision,
            analysisCoverage = AnalysisCoverage(total = 2, fresh = 1, stale = 1))
    var dispatches = 0
    val actions =
        AnalysisWorkspaceActions(
            { _, _ -> dispatches++ },
            { dispatches++ },
            { dispatches++ },
            { dispatches++ },
            { dispatches++ })
    ComposeVisualFixture(800, 650, 1.5f) {
          ProjectSummaryPane(
              coverage,
              project,
              {},
              run = state.run,
              analysisState = state,
              analysisActions = actions)
        }
        .use { fixture ->
          for (status in listOf("running", "paused", "interrupted", "partial", "failed")) {
            state = state.copy(run = planned.copy(status = status, files = listOf(reported)))
            fixture.render()
            fixture.assertTextFits(analysisStatusLabel(status))
            fixture.assertTextFits("1 of 1 files finished")
            fixture.revealText("1 of 2 selected files are up to date", "summary-scroll")
            assertTrue(fixture.hasText("1 of 2 selected files are up to date"))
            assertTrue(
                fixture.hasText(
                    "Finished includes partial and failed outcomes; it does not mean successful."))
            assertEquals(1, fixture.tagCount("summary-analysis-run-strip"))
          }
          state =
              state.copy(
                  run =
                      planned.copy(
                          status = "running",
                          files =
                              listOf(
                                  reported.copy(
                                      stages =
                                          listOf(
                                              AnalysisStageProgress(
                                                  "semantic", "running", 1, false))))))
          fixture.render()
          fixture.revealText("Show full path", "summary-scroll")
          assertTrue(fixture.requestDescriptionFocus("Show active files"))
          fixture.pressKey(androidx.compose.ui.input.key.Key.Enter)
          fixture.render()
          fixture.revealText("Current: $path", "summary-scroll")
          assertTrue(fixture.hasText("Current: $path"))
          state = state.copy(run = requireNotNull(state.run).copy(files = emptyList()))
          fixture.render()
          fixture.assertTextFits(
              "File progress incomplete · captured records missing or inconsistent", maxLines = 3)
          assertFalse(fixture.hasText("Current: $path"))
          state =
              state.copy(
                  run =
                      planned.copy(
                          status = "interrupted",
                          plan =
                              planned.plan.copy(
                                  identity = planned.plan.identity.copy(queueId = "foreign")),
                          files = listOf(reported)))
          fixture.render()
          fixture.assertTextFits("File progress unavailable")
          assertFalse(fixture.hasText("1 of 1 files finished"))
          assertFalse(fixture.hasText("Current: $path"))
          assertEquals(0, dispatches)
        }
  }

  @Test
  fun summaryStartRequestsDefaultFullRunPreviewAndKeepsFirstFailureRetryable() {
    val project = analysisProjectFixture()
    var state by androidx.compose.runtime.mutableStateOf(ProjectAnalysisRunState())
    val previews = mutableListOf<Pair<AnalysisRunLimits, Boolean>>()
    val destinations = mutableListOf<Workspace>()
    val actions =
        AnalysisWorkspaceActions(
            { limits, retry ->
              previews += limits to retry
              state = state.copy(action = "previewing", error = null)
            },
            {},
            {},
            {},
            {})
    ComposeVisualFixture(800, 650) {
          ProjectSummaryPane(
              null, project, destinations::add, analysisState = state, analysisActions = actions)
        }
        .use { fixture ->
          fixture.render()
          fixture.clickText("Start analysis")
          fixture.render()
          assertEquals(listOf(defaultAnalysisRunLimits to false), previews)
          assertTrue(fixture.isDisabled("Start analysis"))
          assertTrue(fixture.hasText("Analysis: Previewing…"))
          assertEquals(0, fixture.tagCount("summary-analysis-run-strip"))
          fixture.clickText("View analysis")
          assertEquals(listOf(Workspace.Analysis), destinations)
          assertEquals(1, previews.size)

          state = state.copy(action = "", error = "Preview timed out. Try again.")
          fixture.render()
          assertTrue(fixture.hasText("Analysis action needs attention"))
          assertTrue(fixture.hasText("Preview timed out. Try again."))
          assertFalse(fixture.isDisabled("Start analysis"))
          assertEquals(1, previews.size)
          fixture.clickText("Start analysis")
          assertEquals(
              listOf(defaultAnalysisRunLimits to false, defaultAnalysisRunLimits to false),
              previews)
          fixture.render()
          assertFalse(fixture.hasText("Preview timed out. Try again."))

          state = state.copy(action = "", fileSelection = AnalysisSelectionState(saving = true))
          fixture.render()
          assertTrue(fixture.isDisabled("Start analysis"))
          assertEquals(2, previews.size)
        }
  }

  @Test
  fun startOnlyAppearsForCurrentProjectWithAvailableStartCommandAndAction() {
    val project = analysisProjectFixture()
    val actions = AnalysisWorkspaceActions({ _, _ -> error("Unexpected preview") }, {}, {}, {}, {})
    ComposeVisualFixture(800, 650) { ProjectSummaryPane(null, null, {}, analysisActions = actions) }
        .use { fixture ->
          fixture.render()
          assertFalse(fixture.hasText("Start analysis"))
          assertTrue(fixture.hasText("No project selected"))
        }
    ComposeVisualFixture(800, 650) {
          ProjectSummaryPane(ProjectOverview(), null, {}, analysisActions = actions)
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.isDisabled("Start analysis"))
        }
    ComposeVisualFixture(800, 650) { ProjectSummaryPane(null, project, {}) }
        .use { fixture ->
          fixture.render()
          assertFalse(fixture.hasText("Start analysis"))
        }
    listOf("queued", "running", "pausing", "paused", "interrupted", "canceling").forEach { status ->
      val run = analysisRunFixture().copy(status = status)
      ComposeVisualFixture(800, 650) {
            ProjectSummaryPane(null, project, {}, run = run, analysisActions = actions)
          }
          .use { fixture ->
            fixture.render()
            assertFalse(fixture.hasText("Start analysis"), "$status must not offer a new preview")
            assertTrue(fixture.hasText("View analysis"))
          }
    }
    ComposeVisualFixture(800, 650) {
          ProjectSummaryPane(
              null,
              project,
              {},
              run =
                  analysisRunFixture()
                      .copy(
                          status = "running",
                          identity = analysisRunFixture().identity.copy(projectId = "other")),
              analysisActions = actions)
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Start analysis"))
        }
  }

  @Test
  fun legendInspectionTogglesSavedRowsLocallyAndAggregateHasNoPaths() {
    val destinations = mutableListOf<Workspace>()
    var previews = 0
    var saves = 0
    val actions =
        AnalysisWorkspaceActions(
            { _, _ -> previews++ }, {}, {}, {}, {}, saveSelection = { saves++ })
    val selection = selectionFixture()
    ComposeVisualFixture(1000, 760) {
          ProjectSummaryPane(
              null,
              analysisProjectFixture(),
              destinations::add,
              fileSelection = selection,
              analysisActions = actions)
        }
        .use { fixture ->
          fixture.render()
          assertEquals(0, fixture.tagCount("summary-coverage-inspection"))
          fixture.clickVisibleDescription("Up to date, 1 file")
          fixture.render()
          assertEquals("Inspecting", fixture.descriptionStateDescription("Up to date, 1 file"))
          assertTrue(fixture.isDescriptionSelected("Up to date, 1 file"))
          assertTrue(fixture.hasText("Up to date · 1 of 2 selected files"))
          assertTrue(fixture.hasText("helper.go"))
          assertTrue(fixture.hasText("Up to date · All applicable stages have current analysis."))
          assertFalse(fixture.hasEditableText(withinTag = "summary-inspection-paths"))
          fixture.clickVisibleDescription("Not analyzed, 1 file")
          fixture.render()
          assertEquals("Not inspecting", fixture.descriptionStateDescription("Up to date, 1 file"))
          assertFalse(fixture.isDescriptionSelected("Up to date, 1 file"))
          assertTrue(fixture.hasText("main.go"))
          assertTrue(
              fixture.hasText(
                  "Not analyzed · ${analysisFileStatus(selection.files.last()).explanation}"))
          fixture.clickVisibleDescription("Not analyzed, 1 file")
          fixture.render()
          assertEquals(0, fixture.tagCount("summary-coverage-inspection"))
          assertEquals(emptyList(), destinations)
          assertEquals(0, previews)
          assertEquals(0, saves)
        }
    val aggregate =
        ProjectOverview(
            "project", "revision", analysisCoverage = AnalysisCoverage(total = 2, failed = 1))
    ComposeVisualFixture(1000, 760) {
          ProjectSummaryPane(aggregate, analysisProjectFixture(), destinations::add)
        }
        .use { fixture ->
          fixture.render()
          fixture.clickVisibleDescription("Failed, 1 file")
          fixture.render()
          assertTrue(
              fixture.hasText(
                  "File paths are unavailable for aggregate coverage. View analysis for file scope."))
          assertTrue(fixture.hasText("View analysis"))
          assertFalse(fixture.hasText("helper.go"))
          assertEquals(emptyList(), destinations)
        }
  }

  @Test
  fun inspectedRowsFollowSameSelectionRefreshAndDisappearWithTheirBucket() {
    val project = analysisProjectFixture()
    var selection by mutableStateOf(selectionFixture())
    val destinations = mutableListOf<Workspace>()
    var previews = 0
    var saves = 0
    val actions =
        AnalysisWorkspaceActions(
            { _, _ -> previews++ }, {}, {}, {}, {}, saveSelection = { saves++ })
    ComposeVisualFixture(1000, 760) {
          ProjectSummaryPane(
              null,
              project,
              destinations::add,
              fileSelection = selection,
              analysisActions = actions)
        }
        .use { fixture ->
          fixture.render()
          fixture.clickVisibleDescription("Not analyzed, 1 file")
          fixture.render()
          selection =
              selection.copy(
                  files =
                      selection.files +
                          AnalysisSelectableFile(
                              "src/new.go", "", selectionStageFixture("missing", "New file.")))
          fixture.render()
          assertTrue(fixture.hasText("Not analyzed · 2 of 3 selected files"))
          assertTrue(fixture.hasText("src/new.go"))
          assertTrue(fixture.hasText("main.go"))
          fixture.clickVisibleDescription("Up to date, 1 file")
          fixture.render()
          // A status refresh removes the focused bucket, not its owner.
          assertTrue(fixture.requestDescriptionFocus("Up to date, 1 file"))
          selection =
              selection.copy(
                  files =
                      selection.files.map { file ->
                        if (file.path == "helper.go")
                            file.copy(stages = selectionStageFixture("missing", "Needs analysis."))
                        else file
                      })
          fixture.render()
          assertEquals(0, fixture.tagCount("summary-coverage-inspection"))
          assertTrue(fixture.isFocusedControl("Not analyzed, 3 files"))
          fixture.clickVisibleDescription("Not analyzed, 3 files")
          fixture.render()
          assertTrue(fixture.hasText("Not analyzed · 3 of 3 selected files"))
          assertTrue(fixture.hasText("helper.go"))
          assertEquals(emptyList(), destinations)
          assertEquals(0, previews)
          assertEquals(0, saves)
        }
  }

  @Test
  fun inspectionIsOwnedByProjectRevisionAndConfirmedSelectionId() {
    var project by mutableStateOf(analysisProjectFixture())
    var selection by mutableStateOf(selectionFixture())
    ComposeVisualFixture(1000, 760) {
          ProjectSummaryPane(null, project, {}, fileSelection = selection)
        }
        .use { fixture ->
          fixture.render()
          fixture.clickVisibleDescription("Up to date, 1 file")
          fixture.render()
          assertTrue(fixture.hasText("helper.go"))
          selection =
              selection.copy(
                  selectionId = "replacement",
                  files =
                      listOf(
                          AnalysisSelectableFile(
                              "replacement.go", "", selectionStageFixture("fresh", "New."))))
          fixture.render()
          assertEquals(0, fixture.tagCount("summary-coverage-inspection"))
          assertFalse(fixture.hasText("helper.go"))
          fixture.clickVisibleDescription("Up to date, 1 file")
          fixture.render()
          assertTrue(fixture.hasText("replacement.go"))
          project = project.copy(projectRevision = "next")
          fixture.render()
          assertEquals(0, fixture.tagCount("summary-coverage-inspection"))
          assertFalse(fixture.hasText("replacement.go"))
          selection = selection.copy(projectRevision = "next")
          fixture.render()
          assertEquals(0, fixture.tagCount("summary-coverage-inspection"))
          fixture.clickVisibleDescription("Up to date, 1 file")
          fixture.render()
          project = project.copy(projectId = "other")
          fixture.render()
          assertEquals(0, fixture.tagCount("summary-coverage-inspection"))
          assertFalse(fixture.hasText("replacement.go"))
        }
  }

  @Test
  fun inspectionSurvivesResizeAndLazyDisposalWithoutDispatch() {
    val destinations = mutableListOf<Workspace>()
    var selection by mutableStateOf(selectionFixture())
    val overview =
        ProjectOverview(
            "project",
            "revision",
            analysis =
                StructuredProjectAnalysis(
                    status = "fresh", components = (0 until 60).map { "Module $it" }))
    ComposeVisualFixture(1100, 780) {
          ProjectSummaryPane(
              overview, analysisProjectFixture(), destinations::add, fileSelection = selection)
        }
        .use { fixture ->
          fixture.render()
          fixture.clickVisibleDescription("Up to date, 1 file")
          fixture.render()
          fixture.resize(530, 650)
          fixture.render()
          assertTrue(fixture.hasText("helper.go"))
          fixture.revealText("Open Editor")
          assertTrue(fixture.requestFocus("Open Editor"))
          fixture.scrollBy(100_000f, "summary-scroll")
          fixture.render()
          assertEquals(0, fixture.tagCount("summary-coverage-inspection"))
          selection =
              selection.copy(
                  files =
                      selection.files +
                          AnalysisSelectableFile(
                              "new.go", "", selectionStageFixture("fresh", "Saved.")))
          fixture.render()
          fixture.revealText("Analysis coverage")
          fixture.render()
          assertTrue(fixture.hasText("Up to date · 2 of 3 selected files"))
          assertTrue(fixture.hasText("helper.go"))
          assertTrue(fixture.hasText("new.go"))
          assertEquals("Inspecting", fixture.descriptionStateDescription("Up to date, 2 files"))
          assertEquals(emptyList(), destinations)
        }
  }

  @Test
  fun narrowInspectionKeepsTextAndViewAnalysisReachable() {
    val destinations = mutableListOf<Workspace>()
    for ((width, scale) in listOf(1440 to 1f, 800 to 1.25f, 430 to 1.5f)) {
      ComposeVisualFixture(width, 650, scale) {
            ProjectSummaryPane(
                null,
                analysisProjectFixture(),
                destinations::add,
                fileSelection = selectionFixture())
          }
          .use { fixture ->
            fixture.render()
            fixture.revealText("Analysis coverage")
            fixture.clickVisibleDescription("Up to date, 1 file")
            fixture.render()
            assertTrue(fixture.hasText("helper.go"))
            fixture.revealText("View analysis")
            fixture.assertTextFits("View analysis")
            val action = fixture.taggedBounds("summary-view-analysis")
            val panel = fixture.taggedBounds("analysis-summary")
            assertTrue(action.left >= panel.left && action.right <= panel.right)
            assertTrue(fixture.requestFocus("View analysis"))
            assertEquals(emptyList(), destinations)
          }
    }
  }

  @Test
  fun longSavedExplanationIsSelectableAndFullyReachableWithoutStartingWork() {
    val reason = "Changed: " + "detail ".repeat(700) + "end of saved explanation"
    val file = AnalysisSelectableFile("src/long.go", "", selectionStageFixture("stale", reason))
    val selection = selectionFixture().copy(files = listOf(file))
    val explanation = "Outdated · ${analysisFileStatus(file).explanation}"
    var previews = 0
    var saves = 0
    val destinations = mutableListOf<Workspace>()
    val actions =
        AnalysisWorkspaceActions(
            { _, _ -> previews++ }, {}, {}, {}, {}, saveSelection = { saves++ })
    ComposeVisualFixture(1000, 760) {
          ProjectSummaryPane(
              null,
              analysisProjectFixture(),
              destinations::add,
              fileSelection = selection,
              analysisActions = actions)
        }
        .use { fixture ->
          fixture.render()
          fixture.clickVisibleDescription("Outdated, 1 file")
          fixture.render()
          assertTrue(fixture.hasText("src/long.go"))
          assertFalse(fixture.hasText(explanation))
          assertTrue(fixture.hasText("Show full available output"))
          fixture.clickDescription("Expand available diagnostic output")
          fixture.render()
          assertTrue(fixture.hasText(explanation))
          assertFalse(fixture.hasEditableText(withinTag = "summary-inspection-paths"))
          fixture.scrollBy(100_000f, "diagnostic-output-scroll")
          fixture.render()
          assertTrue(fixture.verticalScrollValue("diagnostic-output-scroll") > 0f)
          assertEquals(emptyList(), destinations)
          assertEquals(0, previews)
          assertEquals(0, saves)
        }
  }

  @Test
  fun inspectionScrollKeepsEveryPathReachableWithoutStartingWork() {
    val files =
        (0 until 40).map { index ->
          AnalysisSelectableFile(
              "src/file-%02d.go".format(index), "", selectionStageFixture("stale", "Changed."))
        }
    val selection = selectionFixture().copy(files = files.reversed())
    val destinations = mutableListOf<Workspace>()
    ComposeVisualFixture(1000, 760) {
          ProjectSummaryPane(
              null, analysisProjectFixture(), destinations::add, fileSelection = selection)
        }
        .use { fixture ->
          fixture.render()
          fixture.clickVisibleDescription("Outdated, 40 files")
          fixture.render()
          assertEquals(
              40,
              (summaryCoverageProjection(null, analysisProjectFixture(), selection)
                      as SummaryCoverageProjection.Known)
                  .buckets
                  .single()
                  .count)
          files.forEach { assertTrue(fixture.hasText(it.path), "Missing ${it.path}") }
          fixture.scrollBy(10_000f, "summary-inspection-paths")
          fixture.render()
          assertEquals(emptyList(), destinations)
          assertTrue(fixture.hasText("View analysis"))
        }
  }

  @Test
  fun coverageNavigationIsLocalAndLiveSelectionUpdatesItsDial() {
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
    val overview =
        ProjectOverview(
            projectId = project.projectId,
            projectRevision = project.projectRevision,
            analysis = StructuredProjectAnalysis(status = "fresh", purpose = "Saved purpose"))
    assertFalse(projectSummaryPresentation(overview, project).outdated)
    assertTrue(
        projectSummaryPresentation(
                overview.copy(analysis = StructuredProjectAnalysis(status = "stale")), project)
            .outdated)
    assertTrue(
        projectSummaryPresentation(
                overview.copy(analysisCoverage = AnalysisCoverage(total = 1, stale = 1)), project)
            .outdated)
    assertTrue(
        projectSummaryPresentation(overview, project, analysisRunFixture().copy(status = "stale"))
            .outdated)
    assertFalse(
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
