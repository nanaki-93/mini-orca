package io.miniorca.desktop

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.state.ToggleableState
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
  fun directSaveRejectsRunLocksWithoutSendingOrInvalidatingAdmission() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      val admission = AnalysisAdmission(analysisPreviewFixture())
      val confirmed = h.current.selection!!
      assertTrue(confirmed.editable)
      for (status in listOf("queued", "running", "pausing", "canceling", "paused", "interrupted")) {
        h.state =
            h.state.reduce(
                DesktopEvent.AnalysisRunUpdated(
                    h.state.analysisRun.copy(
                        run = analysisRunFixture().copy(status = status), admission = admission)))
        h.workflow.save(listOf("main.go"))
        h.drain()
        assertEquals(admission, h.state.analysisRun.admission, status)
        assertEquals(confirmed, h.current.selection, status)
        assertFalse(h.current.saving, status)
        assertEquals(listOf("GET"), h.methods, status)
      }
      h.state =
          h.state.reduce(
              DesktopEvent.AnalysisRunUpdated(
                  h.state.analysisRun.copy(
                      run = null,
                      fileSelection =
                          h.current.copy(selection = confirmed.copy(editable = false)))))
      h.workflow.save(listOf("main.go"))
      h.drain()
      assertEquals(admission, h.state.analysisRun.admission)
      assertEquals(listOf("GET"), h.methods)
    }
  }

  @Test
  fun directSaveRespectsPendingActionsAndSelectionIoThenAcceptsOneUnlockedWrite() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      val admission = AnalysisAdmission(analysisPreviewFixture())
      for (blocked in
          listOf(
              h.state.analysisRun.copy(action = "preview", admission = admission),
              h.state.analysisRun.copy(
                  fileSelection = h.current.copy(loading = true), admission = admission),
              h.state.analysisRun.copy(
                  fileSelection = h.current.copy(saving = true), admission = admission))) {
        h.state = h.state.reduce(DesktopEvent.AnalysisRunUpdated(blocked))
        h.workflow.save(listOf("main.go"))
        h.drain()
        assertEquals(admission, h.state.analysisRun.admission)
        assertEquals(listOf("GET"), h.methods)
      }
      h.state =
          h.state.reduce(
              DesktopEvent.AnalysisRunUpdated(
                  h.state.analysisRun.copy(
                      action = "",
                      run = analysisRunFixture().copy(status = "completed"),
                      fileSelection = AnalysisSelectionState(h.saved),
                      admission = admission)))
      h.workflow.save(listOf("main.go"))
      h.workflow.save(listOf("helper.go")) // No second write while the first is outstanding.
      assertNull(h.state.analysisRun.admission)
      assertTrue(h.current.saving)
      assertEquals(emptyList(), h.current.selection!!.excludedPaths)
      h.drain()
      assertEquals(listOf("GET", "POST"), h.methods)
      assertEquals(listOf("main.go"), h.current.selection!!.excludedPaths)
      assertEquals(h.saved, h.current.selection)
    }
  }

  @Test
  fun failedSaveRefreshReadsConfirmedSelectionWithoutReplayingWrite() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      h.failSave = true
      h.workflow.save(listOf("main.go"))
      h.drain()
      assertEquals(AnalysisSelectionFailure.Save, h.current.failure)
      assertEquals(emptyList(), h.current.selection!!.excludedPaths)
      assertEquals("Daemon returned 500", h.current.error)
      h.workflow.refresh()
      h.workflow.refresh() // Duplicate activation while loading is ignored.
      h.drain()
      assertEquals(listOf("GET", "POST", "GET"), h.methods)
      assertEquals(emptyList(), h.current.selection!!.excludedPaths)
      assertNull(h.current.error)
      assertNull(h.current.failure)
      assertEquals(emptyList(), h.saved.excludedPaths)
    }
  }

  @Test
  fun failedReadRetainsConfirmedSelectionAndRefreshCanRecover() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      h.failRead = true
      h.workflow.refresh()
      h.drain()
      assertEquals(AnalysisSelectionFailure.Read, h.current.failure)
      assertEquals("Daemon returned 500", h.current.error)
      assertEquals(emptyList(), h.current.selection!!.excludedPaths)
      h.failRead = false
      h.workflow.refresh()
      h.drain()
      assertEquals(listOf("GET", "GET", "GET"), h.methods)
      assertNull(h.current.error)
      assertNull(h.current.failure)
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
  fun projectBoundReloadPreservesAbsentExclusionsAndDaemonDefaultsWithoutWriting() {
    Harness().use { h ->
      h.saved =
          selectionFixture()
              .copy(
                  excludedPaths = listOf("temporarily-absent.go"),
                  files = listOf(AnalysisSelectableFile("new.go", "")))
      h.workflow.refresh()
      h.drain()
      assertEquals(listOf("temporarily-absent.go"), h.current.selection!!.excludedPaths)
      assertEquals("new.go", h.current.selection!!.files.single().path)
      assertFalse("new.go" in h.current.selection!!.excludedPaths)

      h.workflow.detach()
      h.state =
          h.state.reduce(
              DesktopEvent.ProjectLoaded(
                  analysisProjectFixture("other"), ProjectIndex("other", "revision")))
      h.saved = selectionFixture().copy(projectId = "other", excludedPaths = listOf("other.go"))
      h.workflow.refresh()
      h.drain()
      assertEquals("other", h.current.selection!!.projectId)
      assertEquals(listOf("other.go"), h.current.selection!!.excludedPaths)

      h.workflow.detach()
      h.state =
          h.state.reduce(
              DesktopEvent.ProjectLoaded(
                  analysisProjectFixture(), ProjectIndex("project", "revision")))
      h.saved =
          selectionFixture()
              .copy(
                  excludedPaths = listOf("temporarily-absent.go"),
                  files = listOf(AnalysisSelectableFile("new.go", "")))
      h.failRead = true
      h.workflow.refresh()
      h.drain()
      assertNull(h.current.selection)
      assertEquals(AnalysisSelectionFailure.Read, h.current.failure)
      h.failRead = false
      h.workflow.refresh()
      h.drain()
      assertEquals(listOf("temporarily-absent.go"), h.current.selection!!.excludedPaths)
      assertEquals("project", h.current.selection!!.projectId)
      assertEquals(listOf("GET", "GET", "GET", "GET"), h.methods)
    }
  }

  @Test
  fun selectionFailureAndReadOnlyRecoveryRemainReachableWithFilesCollapsed() {
    val selection = selectionFixture().copy(excludedPaths = listOf("main.go"))
    val state = mutableStateOf(AnalysisSelectionState(selection))
    var refreshes = 0
    var saves = 0
    var privileged = 0
    ComposeVisualFixture(800, 650, 1.5f) {
          AnalysisFileSelector(
              ProjectAnalysisRunState(fileSelection = state.value),
              AnalysisWorkspaceActions(
                  { _, _ -> privileged++ },
                  { privileged++ },
                  { privileged++ },
                  { privileged++ },
                  { privileged++ },
                  { refreshes++ },
                  { saves++ }))
        }
        .use { fixture ->
          fixture.render()
          fixture.clickDescription("Collapse Files")
          for (failure in AnalysisSelectionFailure.entries) {
            state.value =
                AnalysisSelectionState(selection, error = "selection failed", failure = failure)
            fixture.render("f12-files-${failure.name.lowercase()}-failure-collapsed")
            assertTrue(
                fixture.hasText(
                    "1 selected · 2 excluded · ${if (failure == AnalysisSelectionFailure.Save) "Save" else "Refresh"} failed"))
            assertTrue(fixture.hasText("selection failed"))
            assertTrue(
                fixture.hasText(
                    if (failure == AnalysisSelectionFailure.Save) "Could not save file selection"
                    else "Could not refresh file selection"))
            fixture.assertTextFits("Refresh files")
            assertTrue(
                fixture.hasText(
                    "The last confirmed selection is still shown. Refresh files reads the saved selection; it does not retry a failed change or start analysis."))
            assertTrue(fixture.requestFocus("Refresh files"))
            assertTrue(fixture.pressKey(Key.Enter))
            assertEquals(0, saves)
            assertEquals(0, privileged)
          }
          assertEquals(2, refreshes)
          assertEquals(listOf("main.go"), state.value.selection!!.excludedPaths)
        }
  }

  @Test
  fun queryAwareFilterCountsAgreeWithRowsAndRefreshIsOnlyExplicitRead() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      var privileged = 0
      ComposeVisualFixture(800, 650, 1.5f) {
            AnalysisFileSelector(
                h.state.analysisRun,
                AnalysisWorkspaceActions(
                    { _, _ -> privileged++ },
                    { privileged++ },
                    { privileged++ },
                    { privileged++ },
                    { privileged++ },
                    { h.workflow.refresh() },
                    { h.workflow.save(it) }))
          }
          .use { fixture ->
            fixture.render()
            fixture.setText("main")
            fixture.render()
            assertTrue(fixture.hasText("1 of 3 files match"))
            for (choice in AnalysisFileFilter.entries) {
              fixture.clickDescription(choice.label)
              fixture.render()
              val matching =
                  if (choice == AnalysisFileFilter.All || choice == AnalysisFileFilter.Attention) 1
                  else 0
              assertTrue(fixture.hasText("$matching of 3 files match"), choice.label)
              assertEquals(matching == 1, fixture.hasText("main.go"), choice.label)
            }
            assertEquals(listOf("GET"), h.methods)
            assertEquals(0, privileged)
            fixture.clickDescription("Collapse Files")
            fixture.render()
            fixture.clickText("Refresh files")
            h.drain()
            assertEquals(listOf("GET", "GET"), h.methods)
            assertTrue(h.requests.isEmpty())
            assertEquals(0, privileged)
          }
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
  fun bulkActionsIgnoreFiltersAndNoMatchesButPreserveOutOfInventoryExclusions() {
    Harness().use { h ->
      h.saved =
          selectionFixture()
              .copy(
                  excludedPaths = listOf("absent.go", ".env", "main.go"),
                  files =
                      selectionFixture().files +
                          AnalysisSelectableFile("policy.go", "excluded by policy"))
      h.workflow.refresh()
      h.drain()
      var privileged = 0
      ComposeVisualFixture(1_440, 900) {
            AnalysisFileSelector(
                h.state.analysisRun,
                AnalysisWorkspaceActions(
                    { _, _ -> privileged++ },
                    { privileged++ },
                    { privileged++ },
                    { privileged++ },
                    { privileged++ },
                    { h.workflow.refresh() },
                    { h.workflow.save(it) }))
          }
          .use { fixture ->
            fixture.render()
            fixture.clickDescription("Up to date")
            fixture.setText("helper")
            fixture.render()
            assertTrue(fixture.hasText("1 of 4 files match"))
            fixture.clickText("Exclude all")
            // The workflow has accepted the write, but the only available snapshot is still
            // confirmed.
            fixture.render()
            h.main.runPending()
            h.io.runPending() // Leave the response queued while inspecting the confirmed UI.
            assertEquals(
                listOf(".env", "absent.go", "helper.go", "main.go"),
                h.requests.single().excludedPaths)
            assertEquals(
                listOf("absent.go", ".env", "main.go"), h.current.selection!!.excludedPaths)
            fixture.render()
            assertTrue(fixture.hasText("1 selected · 3 excluded · Saving selection…"))
            assertEquals(
                ToggleableState.On, fixture.descriptionToggleableState("Analyze helper.go"))
            assertTrue(fixture.isDescriptionDisabled("Analyze helper.go"))
            assertTrue(fixture.isDisabled("Exclude all"))
            assertFalse(fixture.tryClick("Exclude all")) // Duplicate activation is disabled.
            h.drain()
            fixture.render()
            assertEquals(listOf("GET", "POST"), h.methods)
            assertEquals(
                listOf(".env", "absent.go", "helper.go", "main.go"),
                h.current.selection!!.excludedPaths)
            assertTrue(fixture.hasText("0 selected · 4 excluded"))
            assertTrue(fixture.hasText("No matching files."))
            fixture.clickDescription("All")
            fixture.render()
            assertEquals(
                ToggleableState.Off, fixture.descriptionToggleableState("Analyze helper.go"))

            fixture.setText("not-a-project-file")
            fixture.render()
            assertTrue(fixture.hasText("No matching files."))
            assertTrue(fixture.hasText("0 of 4 files match"))
            fixture.clickText("Select all")
            h.drain()
            assertEquals(listOf(".env", "absent.go"), h.requests.last().excludedPaths)
            fixture.render()
            assertEquals(listOf("GET", "POST", "POST"), h.methods)
            assertEquals(listOf(".env", "absent.go"), h.current.selection!!.excludedPaths)
            assertTrue(fixture.hasText("2 selected · 2 excluded"))
            // Even with zero matching rows, bulk actions target the full eligible inventory.
            fixture.clickText("Exclude all")
            h.drain()
            assertEquals(
                listOf(".env", "absent.go", "helper.go", "main.go"),
                h.requests.last().excludedPaths)
            assertEquals(3, h.requests.size)
            assertEquals(0, privileged)
          }
    }
  }

  @Test
  fun saveConflictAndUncertainTransportResultRequireReadBackNotWriteReplay() {
    for (uncertain in listOf(false, true)) {
      Harness().use { h ->
        h.workflow.refresh()
        h.drain()
        val confirmed = h.current.selection!!
        if (uncertain) h.uncertainSave = true else h.saveStatus = 409
        h.workflow.save(listOf("main.go"))
        assertEquals(confirmed, h.current.selection)
        assertTrue(h.current.saving)
        h.drain()
        assertEquals(AnalysisSelectionFailure.Save, h.current.failure)
        assertEquals(confirmed, h.current.selection)
        assertFalse(h.current.saving)
        assertEquals(
            if (uncertain) "Transport result unknown" else "Daemon returned 409", h.current.error)
        assertEquals(listOf("GET", "POST"), h.methods)
        assertEquals(listOf("main.go"), h.requests.single().excludedPaths)
        // A transport error can occur after persistence; only an explicit GET can resolve it.
        assertEquals(if (uncertain) listOf("main.go") else emptyList(), h.saved.excludedPaths)
        h.workflow.refresh()
        h.workflow.refresh()
        assertEquals(confirmed, h.current.selection)
        h.drain()
        assertEquals(listOf("GET", "POST", "GET"), h.methods)
        assertEquals(h.saved, h.current.selection)
        assertNull(h.current.failure)
        assertNull(h.current.error)
        assertEquals(1, h.requests.size)
      }
    }
  }

  @Test
  fun lateReadAndSaveCompletionsCannotReplaceAnotherProjectsSelection() {
    for (saving in listOf(false, true)) {
      Harness().use { h ->
        h.workflow.refresh()
        h.drain()
        val previous = h.current.selection!!
        if (saving) h.workflow.save(listOf("main.go")) else h.workflow.refresh()
        h.main.runPending()
        h.io.runPending() // Response is now queued on main, after transport has returned.
        assertEquals(previous, h.current.selection)
        assertEquals(if (saving) listOf("GET", "POST") else listOf("GET", "GET"), h.methods)
        h.workflow.detach()
        h.state =
            h.state.reduce(
                DesktopEvent.ProjectLoaded(
                    analysisProjectFixture("other"), ProjectIndex("other", "revision")))
        h.saved = selectionFixture().copy(projectId = "other", excludedPaths = listOf("helper.go"))
        h.workflow.refresh()
        h.drain()
        assertEquals("other", h.current.selection!!.projectId)
        assertEquals(listOf("helper.go"), h.current.selection!!.excludedPaths)
        assertNull(h.current.error)
        assertNull(h.current.failure)
        assertEquals(
            if (saving) listOf("GET", "POST", "GET") else listOf("GET", "GET", "GET"), h.methods)
        assertEquals(if (saving) 1 else 0, h.requests.size)
      }
    }
  }

  @Test
  fun fileCheckboxPointerAndKeyboardToggleOnlySelectionWhileInspectionStaysLocal() {
    val selection = mutableStateOf(selectionFixture())
    val saves = mutableListOf<List<String>>()
    var privilegedCalls = 0
    ComposeVisualFixture(1_440, 900) {
          AnalysisFileSelector(
              ProjectAnalysisRunState(fileSelection = AnalysisSelectionState(selection.value)),
              AnalysisWorkspaceActions(
                  { _, _ -> privilegedCalls++ },
                  { privilegedCalls++ },
                  { privilegedCalls++ },
                  { privilegedCalls++ },
                  { privilegedCalls++ },
                  { privilegedCalls++ },
                  { paths ->
                    saves += paths
                    selection.value = selection.value.copy(excludedPaths = paths)
                  }))
        }
        .use { fixture ->
          fixture.render()
          assertEquals(1, fixture.clickableDescriptionCount("Analyze main.go"))
          assertEquals(ToggleableState.On, fixture.descriptionToggleableState("Analyze main.go"))
          assertEquals(
              "Selected for analysis", fixture.descriptionStateDescription("Analyze main.go"))
          assertEquals(ToggleableState.Off, fixture.descriptionToggleableState("Analyze .env"))
          assertTrue(fixture.isDescriptionDisabled("Analyze .env"))
          fixture.setText("main")
          fixture.render()
          assertEquals(0, privilegedCalls)
          assertTrue(fixture.hasDescription("Analyze main.go"))
          fixture.clickDescription("Analysis details for main.go")
          fixture.render()
          assertTrue(saves.isEmpty())
          fixture.clickVisibleDescription("Analyze main.go")
          fixture.render()
          assertEquals(listOf(listOf("main.go")), saves)
          assertEquals(ToggleableState.Off, fixture.descriptionToggleableState("Analyze main.go"))
          assertEquals(
              "Excluded from analysis", fixture.descriptionStateDescription("Analyze main.go"))
          assertTrue(fixture.requestDescriptionFocus("Analyze main.go"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertEquals(listOf(listOf("main.go"), emptyList()), saves)
          assertEquals(ToggleableState.On, fixture.descriptionToggleableState("Analyze main.go"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertEquals(listOf(listOf("main.go"), emptyList(), listOf("main.go")), saves)
          assertEquals(0, privilegedCalls)
        }
  }

  @Test
  fun lockedFileCheckboxesRetainStateButCannotActivate() {
    val selection = selectionFixture().copy(excludedPaths = listOf("main.go"))
    val state =
        mutableStateOf(ProjectAnalysisRunState(fileSelection = AnalysisSelectionState(selection)))
    var saves = 0
    ComposeVisualFixture(1_440, 900) {
          AnalysisFileSelector(
              state.value, AnalysisWorkspaceActions({ _, _ -> }, {}, {}, {}, {}, {}, { saves++ }))
        }
        .use { fixture ->
          listOf(
                  state.value.copy(
                      fileSelection =
                          state.value.fileSelection.copy(
                              selection = selection.copy(editable = false))),
                  state.value.copy(fileSelection = state.value.fileSelection.copy(saving = true)),
                  state.value.copy(run = analysisRunFixture().copy(status = "running")),
                  state.value.copy(run = analysisRunFixture().copy(status = "paused")),
                  state.value.copy(run = analysisRunFixture().copy(status = "interrupted")))
              .forEach { locked ->
                state.value = locked
                fixture.render()
                assertTrue(fixture.isDescriptionDisabled("Analyze main.go"))
                assertEquals(
                    ToggleableState.Off, fixture.descriptionToggleableState("Analyze main.go"))
                assertEquals(
                    "Excluded from analysis",
                    fixture.descriptionStateDescription("Analyze main.go"))
                assertFalse(fixture.tryClick("Analyze main.go"))
                assertFalse(fixture.requestDescriptionFocus("Analyze main.go"))
                fixture.pressKey(Key.Enter)
                fixture.pressKey(Key.Spacebar)
                assertEquals(0, saves)
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
          assertTrue(
              fullSize.hasText(
                  "All eligible files are excluded; no files are selected for analysis."))
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
  fun filesChromeWrapsAndKeepsScopeAndControlsAtLargeText() {
    for (width in listOf(800, 1_600)) {
      ComposeVisualFixture(width, 1_000, 1.5f) {
            AnalysisFileSelector(
                ProjectAnalysisRunState(fileSelection = AnalysisSelectionState(selectionFixture())),
                AnalysisWorkspaceActions({ _, _ -> }, {}, {}, {}, {}, {}, {}))
          }
          .use { fixture ->
            fixture.render()
            val panel = fixture.taggedBounds("analysis-file-panel")
            for (label in listOf("Files", "All", "Needs attention", "Up to date", "Excluded")) {
              val bounds = fixture.firstVisibleTextBounds(label)
              assertTrue(bounds.right <= panel.right, "$label must fit inside Files at $width")
            }
            fixture.assertTextFits("Refresh files")
            assertTrue(fixture.hasText("3 of 3 files match"))
            assertTrue(
                fixture.hasText(
                    "Select all and Exclude all affect every eligible file, regardless of search or filter matches."))
            assertTrue(
                fixture.hasText(
                    "File selection is independent of the open Editor file and does not start analysis."))
          }
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
          val project = state.value.fileSelection.selection
          val view =
              remember(project?.projectId, project?.projectRevision) { AnalysisFilesViewState() }
          AnalysisFileSelector(
              state.value,
              AnalysisWorkspaceActions(
                  { _, _ -> calls++ },
                  { calls++ },
                  { calls++ },
                  { calls++ },
                  { calls++ },
                  { calls++ },
                  { calls++ }),
              320.dp,
              view = view)
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
          fixture.render("f12-files-loading")
          assertFalse(fixture.hasText("No matching files."))
          assertFalse(fixture.hasText("No files are available for analysis."))
          assertTrue(fixture.hasText("Loading file selection…"))
          assertFalse(fixture.hasText("0 of 0 files match"))

          selectionState.value = AnalysisSelectionState(error = "Unable to load files")
          fixture.render("f12-files-initial-read-failure")
          assertTrue(fixture.hasText("Unable to load files"))
          assertFalse(fixture.hasText("0 of 0 files match"))
          assertTrue(fixture.hasText("File status is not loaded. Refresh files to try again."))

          selectionState.value =
              AnalysisSelectionState(selection = selectionFixture().copy(files = emptyList()))
          fixture.render("f12-files-empty")
          assertTrue(fixture.hasText("No files are available for analysis."))
          assertTrue(fixture.hasText("0 of 0 files match"))
          assertFalse(fixture.hasText("No matching files."))

          selectionState.value = AnalysisSelectionState(selection = selectionFixture())
          fixture.render()
          fixture.setText("not-a-project-file")
          fixture.render("f12-files-no-match")
          assertTrue(fixture.hasText("No matching files."))
          assertTrue(fixture.hasText("0 of 3 files match"))

          selectionState.value =
              AnalysisSelectionState(selection = selectionFixture().copy(editable = false))
          fixture.render("f12-files-daemon-locked")
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

  @Test
  fun rowDetailsBelongToPathsAcrossCollapseDisposalFilteringAndInventoryRefresh() {
    val initial = selectionFixture()
    val current = mutableStateOf(initial)
    val visible = mutableStateOf(true)
    val view = AnalysisFilesViewState()
    var requests = 0
    ComposeVisualFixture(1440, 900) {
          if (visible.value)
              AnalysisFileSelector(
                  ProjectAnalysisRunState(fileSelection = AnalysisSelectionState(current.value)),
                  AnalysisWorkspaceActions(
                      { _, _ -> requests++ },
                      { requests++ },
                      { requests++ },
                      { requests++ },
                      { requests++ },
                      { requests++ },
                      { requests++ }),
                  320.dp,
                  view = view)
        }
        .use { fixture ->
          fixture.render()
          fixture.clickDescription("Analysis details for main.go")
          fixture.render()
          assertEquals(
              "Expanded", fixture.descriptionStateDescription("Analysis details for main.go"))
          fixture.clickDescription("Collapse Files")
          fixture.render()
          fixture.clickDescription("Expand Files")
          fixture.render()
          assertEquals(
              "Expanded", fixture.descriptionStateDescription("Analysis details for main.go"))
          visible.value = false // Workspace navigation or outer lazy disposal.
          fixture.render()
          visible.value = true
          fixture.render()
          assertEquals(
              "Expanded", fixture.descriptionStateDescription("Analysis details for main.go"))
          fixture.setText("helper")
          fixture.render()
          assertFalse(fixture.hasDescription("Analysis details for main.go"))
          fixture.setText("")
          fixture.render()
          assertEquals(
              "Expanded", fixture.descriptionStateDescription("Analysis details for main.go"))
          assertTrue(fixture.requestDescriptionFocus("Analysis details for main.go"))
          current.value =
              current.value.copy(files = current.value.files.filterNot { it.path == "main.go" })
          fixture.render()
          assertFalse(view.expandedDetails.containsKey("main.go"))
          current.value = initial
          fixture.render()
          assertEquals(
              "Collapsed", fixture.descriptionStateDescription("Analysis details for main.go"))
          assertFalse(fixture.isDescriptionFocused("Analysis details for main.go"))
          assertEquals(0, requests)
        }
  }

  private class Harness : AutoCloseable {
    val main = AnalysisQueuedDispatcher()
    val io = AnalysisQueuedDispatcher()
    private val scope = CoroutineScope(SupervisorJob() + main)
    var state by
        mutableStateOf(
            DesktopState()
                .reduce(
                    DesktopEvent.ProjectLoaded(
                        analysisProjectFixture(), ProjectIndex("project", "revision"))))
    var saved = selectionFixture()
    var failSave = false
    var failRead = false
    var saveStatus = 200
    var uncertainSave = false
    val methods = mutableListOf<String>()
    val requests = mutableListOf<AnalysisSelectionRequest>()
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
                        if (method == "GET" && failRead)
                            return TransportResponse(500, "read failed")
                        if (method == "POST") {
                          val request = Json.decodeFromString<AnalysisSelectionRequest>(body!!)
                          requests += request
                          assertEquals(saved.selectionId, request.selectionId)
                          if (failSave) return TransportResponse(500, "save failed")
                          if (saveStatus != 200) return TransportResponse(saveStatus, "conflict")
                          saved =
                              saved.copy(
                                  selectionId = "saved", excludedPaths = request.excludedPaths)
                          if (uncertainSave) throw IllegalStateException("Transport result unknown")
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
