package io.miniorca.desktop

import androidx.compose.runtime.mutableStateOf
import androidx.compose.ui.unit.dp
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
  fun measuredTableHeightUsesRemainingViewportAndStaysBounded() {
    assertEquals(160.dp, analysisFileTableHeight(1000.dp, null))
    assertEquals(400.dp, analysisFileTableHeight(1000.dp, 440.dp))
    assertEquals(240.dp, analysisFileTableHeight(1000.dp, 760.dp))
    assertEquals(160.dp, analysisFileTableHeight(600.dp, 560.dp))
  }

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
  fun selectorShowsSavedChecksSearchAndBulkActionsAtFullSize() {
    listOf(1_440 to 900).forEach { (width, height) ->
      val saves = mutableListOf<List<String>>()
      val selected = selectionFixture().copy(excludedPaths = listOf("main.go"))
      ComposeVisualFixture(width, height, 1.5f) {
            AnalysisFileSelector(
                ProjectAnalysisRunState(fileSelection = AnalysisSelectionState(selected)),
                AnalysisWorkspaceActions({ _, _ -> }, {}, {}, {}, {}, {}, { saves.add(it) }))
          }
          .use { fixture ->
            fixture.render()
            assertTrue(fixture.hasDescription("Collapse Files"))
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
  fun reducedWindowRenderDoesNotChangeBulkSelectionOrAnalysisScope() {
    val selection = mutableStateOf(selectionFixture())
    val saves = mutableListOf<List<String>>()
    val actions =
        AnalysisWorkspaceActions(
            { _, _ -> },
            {},
            {},
            {},
            {},
            {},
            { paths ->
              saves += paths
              selection.value = selection.value.copy(excludedPaths = paths)
            })
    ComposeVisualFixture(1_440, 900) {
          AnalysisFileSelector(
              ProjectAnalysisRunState(fileSelection = AnalysisSelectionState(selection.value)),
              actions)
        }
        .use { fullSize ->
          fullSize.render()
          fullSize.clickText("Exclude all")
          fullSize.render()
          assertEquals(listOf("helper.go", "main.go"), saves.single())
          assertTrue(fullSize.hasText("0 selected · 3 excluded"))

          ComposeVisualFixture(800, 650) {
                AnalysisFileSelector(
                    ProjectAnalysisRunState(
                        fileSelection = AnalysisSelectionState(selection.value)),
                    actions)
              }
              .use { reduced ->
                reduced.render()
                assertTrue(reduced.taggedBounds("analysis-file-table").height > 0f)
                assertTrue(reduced.hasText("0 selected · 3 excluded"))
                assertEquals(listOf("helper.go", "main.go"), selection.value.excludedPaths)
                assertEquals(1, saves.size, "Resizing must not dispatch another selection save")
              }

          fullSize.render()
          assertTrue(fullSize.hasText("0 selected · 3 excluded"))
          fullSize.clickText("Select all")
          assertEquals(
              emptyList(), saves.last(), "Bulk scope must still include both eligible files")
          assertEquals(2, saves.size)
          fullSize.render()
          assertTrue(fullSize.hasText("2 selected · 1 excluded"))
        }
  }

  @Test
  fun togglingAStaleFileUpdatesSelectionWithoutVerboseStageDetails() {
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
          assertTrue(fixture.hasText("1 selected · 0 excluded"))
          fixture.render()
          assertTrue(fixture.hasText("Source changed."))
          assertFalse(
              fixture.hasText("Performance review · Unavailable — Model is not configured."))
          assertTrue(fixture.hasDescription("Analysis details for main.go"))
          fixture.clickDescription("Analysis details for main.go")
          fixture.render()
          assertTrue(
              fixture.hasText(
                  "Code analysis: Source changed.\nPerformance review: Model is not configured."))
          fixture.clickDescription("Analysis details for main.go")
          fixture.render()
          assertEquals(0, saves)
          fixture.clickDescription("Analyze main.go")
          fixture.render("analysis-files-excluded-800-150")
          assertTrue(fixture.hasText("0 selected · 1 excluded"))
          assertTrue(fixture.hasText("Excluded by you."))
          assertFalse(fixture.hasText("Outdated"))
          assertFalse(fixture.hasText("Source changed."))
          fixture.clickDescription("Analyze main.go")
          fixture.render()
          assertTrue(fixture.hasText("1 selected · 0 excluded"))
          assertTrue(fixture.hasText("Outdated"))
          assertEquals(2, saves)
        }
  }

  @Test
  fun filtersRemainInlineAtFullSizeWithIncreasedTextScale() {
    ComposeVisualFixture(1_600, 1_000, 1.5f) {
          AnalysisFileSelector(
              ProjectAnalysisRunState(fileSelection = AnalysisSelectionState(selectionFixture())),
              AnalysisWorkspaceActions({ _, _ -> }, {}, {}, {}, {}, {}, {}))
        }
        .use { fixture ->
          fixture.render("analysis-files-inline-filters-1600-1000-150")
          val panel = fixture.taggedBounds("analysis-file-panel")
          val files = fixture.firstVisibleTextBounds("Files")
          val all = fixture.firstVisibleTextBounds("All")
          val attention = fixture.firstVisibleTextBounds("Needs attention")
          val excluded = fixture.firstVisibleTextBounds("Excluded")
          assertTrue(files.right < all.left, "Filters must remain beside the Files disclosure")
          assertTrue(all.right < attention.left && attention.right < excluded.left)
          assertTrue(
              kotlin.math.abs(all.center.y - excluded.center.y) < 2f,
              "Filters must stay on one line at full size and 150% text scale")
          assertTrue(excluded.right <= panel.right, "Filters must fit inside the panel")
        }
  }

  @Test
  fun longPathsAndSaveErrorsRemainReadable() {
    val path = "internal/" + "long_project_directory/".repeat(5) + "analysis.go"
    val selection = selectionFixture().copy(files = listOf(AnalysisSelectableFile(path, "")))
    ComposeVisualFixture(3_200, 2_000, 1.5f) {
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
          assertTrue(fixture.hasText("Selection could not be saved. Refresh files and retry."))
          fixture.render("analysis-files-long-path-error-800-150")
          fixture.assertTextFits(path, maxLines = 5)
          assertTrue(fixture.hasText("Selection could not be saved. Refresh files and retry."))
        }
  }

  @Test
  fun disclosureIsLocalPreservesSelectionAndResetsForAnotherProject() {
    val state =
        mutableStateOf(
            ProjectAnalysisRunState(
                fileSelection =
                    AnalysisSelectionState(
                        selectionFixture().copy(excludedPaths = listOf("main.go")))))
    var calls = 0
    ComposeVisualFixture(1_440, 900, 1.5f) {
          AnalysisFileSelector(
              state.value,
              AnalysisWorkspaceActions(
                  { _, _ -> calls++ },
                  { calls++ },
                  { calls++ },
                  { calls++ },
                  { calls++ },
                  { calls++ },
                  { calls++ }))
        }
        .use { fixture ->
          fixture.render("analysis-files-default-expanded")
          assertTrue(fixture.hasDescription("Analyze main.go"))
          fixture.clickDescription("Collapse Files")
          fixture.render("analysis-files-collapsed")
          assertFalse(fixture.hasDescription("Analyze main.go"))
          repeat(2) {
            fixture.clickDescription("Expand Files")
            fixture.render()
            assertTrue(fixture.hasText("Excluded by you."))
            fixture.clickDescription("Collapse Files")
            fixture.render()
          }
          assertEquals(0, calls)
          assertEquals(listOf("main.go"), state.value.fileSelection.selection!!.excludedPaths)
          state.value =
              state.value.copy(
                  run = analysisRunFixture().copy(status = "running"),
                  fileSelection = state.value.fileSelection.copy(error = "Save failed"))
          fixture.render("analysis-files-locked-collapsed")
          assertTrue(fixture.hasText("Save failed"))
          assertTrue(
              fixture.hasText(
                  "Selection locked. Finish or cancel the current run to change files."))
          fixture.clickDescription("Expand Files")
          fixture.render()
          assertFalse(fixture.hasText("Select all"))
          assertFalse(fixture.hasText("Exclude all"))
          assertFalse(fixture.tryClick("Analyze main.go"))
          state.value =
              state.value.copy(
                  run = null,
                  fileSelection =
                      AnalysisSelectionState(selectionFixture().copy(projectId = "other")))
          fixture.render()
          assertTrue(fixture.hasDescription("Collapse Files"))
          assertTrue(fixture.hasDescription("Analyze main.go"))
          assertEquals(0, calls)
        }
  }

  @Test
  fun fileInventoryStatesAndUnexplainedLockRemainTruthful() {
    val selectionState = mutableStateOf(AnalysisSelectionState(loading = true))
    val actions = AnalysisWorkspaceActions({ _, _ -> }, {}, {}, {}, {})
    ComposeVisualFixture(1_440, 900) {
          AnalysisFileSelector(
              ProjectAnalysisRunState(fileSelection = selectionState.value), actions)
        }
        .use { fixture ->
          fixture.render()
          assertFalse(fixture.hasText("No matching files."))
          assertFalse(fixture.hasText("No files are available for analysis."))

          selectionState.value = AnalysisSelectionState(error = "Unable to load files")
          fixture.render()
          assertTrue(fixture.hasText("Unable to load files"))
          assertTrue(fixture.hasText("File status is not loaded. Refresh files to try again."))

          selectionState.value =
              AnalysisSelectionState(selection = selectionFixture().copy(files = emptyList()))
          fixture.render()
          assertTrue(fixture.hasText("No files are available for analysis."))
          assertFalse(fixture.hasText("No matching files."))

          selectionState.value = AnalysisSelectionState(selection = selectionFixture())
          fixture.render()
          fixture.setText("not-a-project-file")
          fixture.render()
          assertTrue(fixture.hasText("No matching files."))

          selectionState.value =
              AnalysisSelectionState(selection = selectionFixture().copy(editable = false))
          fixture.render()
          assertTrue(fixture.hasText("Selection changes unavailable."))
          assertFalse(
              fixture.hasText(
                  "Selection locked. Finish or cancel the current run to change files."))
        }
  }

  @Test
  fun runningFiltersAndDisclosureStayLocalWhilePausedAndInterruptedSelectionRemainLocked() {
    val initial =
        roundedAnalysisStateFixture()
            .copy(
                fileSelection =
                    roundedAnalysisStateFixture().fileSelection.let {
                      it.copy(selection = it.selection!!.copy(editable = true))
                    })
    val state = mutableStateOf(initial)
    var actions = 0
    ComposeVisualFixture(1440, 900) {
          AnalysisFileSelector(
              state.value,
              AnalysisWorkspaceActions(
                  { _, _ -> actions++ },
                  { actions++ },
                  { actions++ },
                  { actions++ },
                  { actions++ },
                  { actions++ },
                  { actions++ }))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasDescription("Collapse Files"))
          fixture.scrollBy(100_000f, "analysis-file-table")
          fixture.render("analysis-last-file")
          fixture.assertTextFits(".env")
          fixture.scrollBy(-100_000f, "analysis-file-table")
          fixture.render()
          fixture.clickDescription("Needs attention")
          fixture.render()
          assertTrue(fixture.hasText("4 of 15 files match"))
          fixture.setText("internal/api/user")
          fixture.render("analysis-running-filtered")
          fixture.assertTextFits("internal/api/user.go")
          assertTrue(fixture.hasText("1 of 15 files match"))
          assertFalse(fixture.tryClick("Analyze internal/api/user.go"))
          fixture.clickDescription("Collapse Files")
          fixture.render()
          fixture.clickDescription("Expand Files")
          fixture.render()
          assertTrue(fixture.hasText("1 of 15 files match"))
          state.value = state.value.copy(run = state.value.run!!.copy(status = "paused"))
          fixture.render()
          assertFalse(fixture.hasText("Select all"))
          assertFalse(fixture.hasText("Exclude all"))
          assertFalse(fixture.tryClick("Analyze internal/api/user.go"))
          assertEquals(0, actions)
          assertEquals(initial.fileSelection, state.value.fileSelection)
          state.value = state.value.copy(run = state.value.run!!.copy(status = "interrupted"))
          fixture.render()
          assertFalse(fixture.hasText("Select all"))
          assertFalse(fixture.hasText("Exclude all"))
          assertFalse(fixture.tryClick("Analyze internal/api/user.go"))
          assertEquals(0, actions)
          assertEquals(initial.fileSelection, state.value.fileSelection)
          state.value = state.value.copy(run = null)
          fixture.render()
          fixture.clickDescription("Analyze internal/api/user.go")
          assertEquals(1, actions)
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
