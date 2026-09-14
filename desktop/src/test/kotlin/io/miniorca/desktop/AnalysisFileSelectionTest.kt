package io.miniorca.desktop

import androidx.compose.runtime.mutableStateOf
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json

class AnalysisFileSelectionTest {
  @Test
  fun saveRestoresAcrossClientsAndNeverDispatchesAnalysis() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      assertEquals(emptyList(), h.current.selection!!.excludedPaths)
      h.workflow.save(listOf("main.go"))
      h.drain()
      assertEquals(listOf("main.go"), h.saved.excludedPaths)
      assertEquals(h.saved, h.current.selection)
      h.workflow.detach()
      assertNull(h.current.selection)
      h.workflow.refresh()
      h.drain()
      assertEquals(listOf("main.go"), h.current.selection!!.excludedPaths)
      val restored = h.current.selection!!
      val row = restored.files.first { it.path == "main.go" }
      assertEquals(
          AnalysisFileSyncStatus.Excluded,
          analysisFileStatus(row, row.path in restored.excludedPaths).status)
      assertEquals(listOf("GET", "POST", "GET"), h.methods)
    }
  }

  @Test
  fun failureRetainsConfirmedSelectionAndProjectSwitchRejectsLateRead() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      h.failSave = true
      h.workflow.save(listOf("main.go"))
      h.drain()
      assertNotNull(h.current.error)
      assertEquals(emptyList(), h.current.selection!!.excludedPaths)
      assertFalse(h.current.saving)
      h.workflow.refresh()
      h.main.runPending()
      h.io.runPending()
      h.workflow.detach()
      h.state =
          h.state.reduce(
              DesktopEvent.ProjectLoaded(
                  analysisProjectFixture("other"), ProjectIndex("other", "revision")))
      h.drain()
      assertNull(h.current.selection)
    }
  }

  @Test
  fun selectorShowsSavedChecksSearchAndBulkActionsAcrossViewports() {
    listOf(1280 to 800, 1000 to 650, 999 to 650, 800 to 650, 1280 to 600).forEach { (width, height)
      ->
      val saves = mutableListOf<List<String>>()
      val selected = selectionFixture().copy(excludedPaths = listOf("main.go"))
      ComposeVisualFixture(width, height, 1.5f) {
            AnalysisFileSelector(
                ProjectAnalysisRunState(fileSelection = AnalysisSelectionState(selected)),
                AnalysisWorkspaceActions({ _, _ -> }, {}, {}, {}, {}, {}, { saves.add(it) }))
          }
          .use { fixture ->
            fixture.render()
            fixture.render("analysis-files-$width-$height-150")
            fixture.assertTextFits("Select all")
            fixture.assertTextFits("Exclude all")
            assertTrue(fixture.hasText("secret or local configuration"))
            assertTrue(fixture.hasDescription("Analyze .env"))
            fixture.clickDescription("Needs attention")
            fixture.render()
            assertFalse(fixture.hasText("helper.go"))
            assertFalse(fixture.hasText("main.go"))
            assertFalse(fixture.hasText(".env"))
            assertTrue(saves.isEmpty())
            fixture.clickDescription("Excluded")
            fixture.render()
            assertTrue(fixture.hasText("Excluded by you."))
            fixture.setText("main")
            fixture.render()
            fixture.assertTextFits("main.go")
            fixture.clickDescription("Analyze main.go")
            assertEquals(emptyList(), saves.last())
            fixture.setText("main")
            fixture.render()
            assertFalse(fixture.hasText("helper.go"))
            fixture.clickText("Exclude all")
            assertEquals(listOf("helper.go", "main.go"), saves.last())
          }
    }
  }

  @Test
  fun togglingAStaleFileUpdatesCoverageAndKeepsReasonsAvailableOnDemand() {
    val file =
        AnalysisSelectableFile(
            "main.go",
            "",
            listOf(
                AnalysisFileStageStatus("semantic", "stale", "Source changed."),
                AnalysisFileStageStatus("performance", "unavailable", "Model is not configured.")))
    val selected = mutableStateOf(selectionFixture().copy(files = listOf(file)))
    var saves = 0
    ComposeVisualFixture(800, 650, 1.5f) {
          AnalysisFileSelector(
              ProjectAnalysisRunState(fileSelection = AnalysisSelectionState(selected.value)),
              AnalysisWorkspaceActions(
                  { _, _ -> },
                  {},
                  {},
                  {},
                  {},
                  {},
                  {
                    saves++
                    selected.value = selected.value.copy(excludedPaths = it)
                  }))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("0 up to date · 1 need attention · 0 excluded"))
          assertTrue(fixture.hasText("Source changed."))
          assertFalse(
              fixture.hasText("Performance review · Unavailable — Model is not configured."))
          fixture.clickDescription("Analysis details for main.go")
          fixture.render()
          assertTrue(fixture.hasText("Performance review · Unavailable — Model is not configured."))
          assertEquals(0, saves)
          fixture.clickDescription("Analyze main.go")
          fixture.render("analysis-files-excluded-800-150")
          assertTrue(fixture.hasText("0 up to date · 0 need attention · 1 excluded"))
          assertTrue(fixture.hasText("Excluded by you."))
          assertFalse(fixture.hasText("Outdated"))
          assertFalse(fixture.hasText("Source changed."))
          fixture.clickDescription("Analyze main.go")
          fixture.render()
          assertTrue(fixture.hasText("0 up to date · 1 need attention · 0 excluded"))
          assertTrue(fixture.hasText("Outdated"))
          assertEquals(2, saves)
        }
  }

  @Test
  fun longPathsAndSaveErrorsRemainReadable() {
    val path = "internal/" + "long_project_directory/".repeat(5) + "analysis.go"
    val selection = selectionFixture().copy(files = listOf(AnalysisSelectableFile(path, "")))
    ComposeVisualFixture(800, 650, 1.5f) {
          AnalysisFileSelector(
              ProjectAnalysisRunState(
                  fileSelection =
                      AnalysisSelectionState(
                          selection,
                          error = "Selection could not be saved. Refresh files and retry.")),
              AnalysisWorkspaceActions({ _, _ -> }, {}, {}, {}, {}))
        }
        .use { fixture ->
          fixture.render()
          fixture.render("analysis-files-long-path-error-800-150")
          fixture.assertTextFits(path, maxLines = 5)
          assertTrue(fixture.hasText("Selection could not be saved. Refresh files and retry."))
        }
  }

  private class Harness : AutoCloseable {
    val main = AnalysisQueuedDispatcher()
    val io = AnalysisQueuedDispatcher()
    private val scope = CoroutineScope(SupervisorJob() + main)
    var state =
        DesktopState()
            .reduce(
                DesktopEvent.ProjectLoaded(
                    analysisProjectFixture(), ProjectIndex("project", "revision")))
    var saved = selectionFixture()
    var failSave = false
    val methods = mutableListOf<String>()
    val current
      get() = state.analysisRun.fileSelection

    val workflow =
        DesktopAnalysisSelectionWorkflow(
            ApiClient(
                transport =
                    object : DaemonTransport {
                      override fun send(
                          method: String,
                          path: String,
                          body: String?
                      ): TransportResponse {
                        assertTrue(path.startsWith("/api/projects/current/analysis/selection"))
                        methods.add(method)
                        if (method == "POST") {
                          if (failSave) return TransportResponse(500, "save failed")
                          val request = Json.decodeFromString<AnalysisSelectionRequest>(body!!)
                          assertEquals(saved.selectionId, request.selectionId)
                          saved =
                              saved.copy(
                                  selectionId = "saved", excludedPaths = request.excludedPaths)
                        }
                        return TransportResponse(200, Json.encodeToString(saved))
                      }
                    }),
            scope,
            io,
            { state },
            { state = state.reduce(it) })

    fun drain() {
      repeat(4) {
        main.runPending()
        io.runPending()
      }
      main.runPending()
    }

    override fun close() {
      workflow.detach()
      scope.cancel()
      drain()
    }
  }
}

internal fun selectionFixture() =
    AnalysisFileSelection(
        "project",
        "revision",
        "selection",
        emptyList(),
        listOf(
            AnalysisSelectableFile(".env", "secret or local configuration"),
            AnalysisSelectableFile(
                "helper.go", "", selectionStageFixture("fresh", "Saved analysis is up to date.")),
            AnalysisSelectableFile(
                "main.go",
                "",
                selectionStageFixture("missing", "No saved analysis exists for this stage."))),
        true)

internal fun selectionStageFixture(status: String, reason: String) =
    listOf("semantic", "performance", "security_rules", "security_ai").map {
      AnalysisFileStageStatus(it, status, reason)
    }
