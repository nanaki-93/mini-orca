package io.miniorca.desktop

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
            fixture.clickText("Files to analyze")
            fixture.render("analysis-files-$width-$height-150")
            fixture.assertTextFits("Select all")
            fixture.assertTextFits("Ignore all")
            fixture.assertTextFits("main.go")
            assertTrue(fixture.hasText("secret or local configuration"))
            assertTrue(fixture.isDisabled(".env"))
            fixture.clickDescription("Analyze main.go")
            assertEquals(emptyList(), saves.last())
            fixture.setText("main")
            fixture.render()
            assertFalse(fixture.hasText("helper.go"))
            fixture.clickText("Ignore all")
            assertEquals(listOf("helper.go", "main.go"), saves.last())
          }
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
          fixture.clickText("Files to analyze")
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
            AnalysisSelectableFile("helper.go", ""),
            AnalysisSelectableFile("main.go", "")),
        true)
