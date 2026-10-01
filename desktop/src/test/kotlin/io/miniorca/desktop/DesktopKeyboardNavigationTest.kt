package io.miniorca.desktop

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshots.Snapshot
import androidx.compose.ui.ExperimentalComposeUiApi
import androidx.compose.ui.InternalComposeUiApi
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusManager
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEvent
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.platform.ClipEntry
import androidx.compose.ui.platform.Clipboard
import androidx.compose.ui.platform.LocalClipboard
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.platform.asAwtTransferable
import androidx.compose.ui.unit.dp
import java.nio.file.Files
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import javax.swing.SwingUtilities
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue
import kotlinx.coroutines.CompletableDeferred

class DesktopKeyboardNavigationTest {
  @Test
  fun explorerFilterOwnsSpacesArrowsAndEditingWithoutActivatingTheTree() {
    val files = listOf(IndexedFile("src/a b.go", "hash", "Go", false))
    var state by
        mutableStateOf(
            ExplorerPaneState(
                ProjectIndex("project", "revision", files = files),
                "src/a b.go",
                "",
                emptySet(),
                false))
    val calls = mutableListOf<String>()
    ComposeVisualFixture(360, 400) {
          ExplorerPane(
              state,
              ExplorerPaneActions(
                  { state = state.copy(filter = it) },
                  { calls += "toggle:$it" },
                  { calls += "collapse" },
                  { calls += "reveal" },
                  { calls += "open:$it" }),
              Modifier.fillMaxSize())
        }
        .use { fixture ->
          fixture.render()
          fixture.focusDescribedEditor("Filter indexed files")
          assertTrue(fixture.typeCharacter(Key.A, 'a'))
          assertTrue(fixture.typeCharacter(Key.Spacebar, ' '))
          assertTrue(fixture.typeCharacter(Key.B, 'b'))
          assertEquals("a b", state.filter)
          for (key in listOf(Key.DirectionUp, Key.DirectionDown, Key.Enter)) {
            fixture.pressKey(key)
            fixture.render()
            assertEquals("a b", state.filter)
          }
          fixture.pressKey(Key.DirectionLeft)
          fixture.render()
          fixture.pressKey(Key.Backspace)
          fixture.render()
          assertEquals("ab", state.filter, "Left/Backspace must edit the filter, not the tree")
          fixture.pressKey(Key.DirectionRight)
          fixture.render()
          fixture.pressKey(Key.Backspace)
          fixture.render()
          assertEquals("a", state.filter)
          fixture.pressKey(Key.MoveHome)
          fixture.render()
          fixture.pressKey(Key.Delete)
          fixture.render()
          assertEquals("", state.filter)
          assertTrue(fixture.typeCharacter(Key.Spacebar, ' '))
          assertEquals(" ", state.filter)
          assertTrue(fixture.isFocusedControl("Filter indexed relative file paths"))
          assertEquals(emptyList(), calls, "Filter keys must never emit tree actions")
        }
  }

  @Test
  fun explorerTreeArrowsRevealOffscreenFocusAndOnlyExplicitActivationOpensFiles() {
    val files =
        (1..60).map {
          IndexedFile("file-${it.toString().padStart(2, '0')}.go", "hash", "Go", false)
        }
    val calls = mutableListOf<String>()
    ComposeVisualFixture(360, 260, 1.5f) {
          ExplorerPane(
              ExplorerPaneState(
                  ProjectIndex("project", "revision", files = files),
                  files.first().path,
                  "",
                  emptySet(),
                  false),
              ExplorerPaneActions({}, { calls += "toggle:$it" }, {}, {}, { calls += "open:$it" }),
              Modifier.fillMaxSize())
        }
        .use { fixture ->
          fixture.render()
          assertFalse(fixture.hasText(files.last().path))
          assertTrue(fixture.requestDescriptionFocus("Indexed file tree"))
          fixture.render()
          fun assertFocused(index: Int) {
            val description = explorerRowDescription(explorerRows(files)[index], index == 0, true)
            fixture.awaitVisibleDescription(description)
            assertTrue(fixture.descriptionState(description)!!.contains("Keyboard focused"))
            val bounds = fixture.descriptionBounds(description)
            val viewport = fixture.descriptionBounds("Indexed file tree")
            assertTrue(bounds.top >= viewport.top && bounds.bottom <= viewport.bottom)
            fixture.assertTextFits(files[index].path)
          }
          assertFocused(0)
          assertTrue(fixture.pressKey(Key.DirectionUp))
          fixture.render()
          assertFocused(0)
          for (index in 1..files.lastIndex) {
            assertTrue(fixture.pressKey(Key.DirectionDown))
            fixture.render()
            assertFocused(index)
          }
          assertTrue(fixture.pressKey(Key.DirectionDown))
          fixture.render("f23-explorer-last-row-keyboard-focus")
          assertFocused(files.lastIndex)
          fixture.assertColorVisible(FocusAccent)
          assertEquals(emptyList(), calls, "Focus movement must not read or open files")
          for (key in listOf(Key.Enter, Key.Spacebar)) {
            assertTrue(fixture.pressKey(key))
            fixture.render()
          }
          assertEquals(List(2) { "open:${files.last().path}" }, calls)
          repeat(files.lastIndex) {
            assertTrue(fixture.pressKey(Key.DirectionUp))
            fixture.render()
          }
          assertFocused(0)
          fixture.render("f23-explorer-first-row-keyboard-focus")
          assertEquals(List(2) { "open:${files.last().path}" }, calls)
        }
  }

  @Test
  fun contextDisclosuresKeepKeyboardFocusAndDoNotDispatch() {
    val base = contextVisualState()
    val state =
        base.copy(
            impact =
                ImpactPreview(
                    base.inspector!!.file.path,
                    references =
                        listOf(
                            ImpactReference("internal/caller.go", "Run", "exact", "Indexed use."))))
    var privileged = 0
    ComposeVisualFixture(320, 400, 1.5f) {
          ContextToolWindow(
              state,
              ContextToolWindowActions(
                  { privileged++ },
                  { privileged++ },
                  { privileged++ },
                  { privileged++ },
                  { privileged++ },
                  explainSelected = { privileged++ }),
              androidx.compose.ui.Modifier.fillMaxSize())
        }
        .use { fixture ->
          fixture.render()
          for ((title, key) in
              listOf(
                  "Declaration details" to Key.Enter,
                  "References" to Key.Spacebar,
                  "File details" to Key.Enter,
                  "Project context" to Key.Spacebar)) {
            fixture.revealText(title, "context-content")
            assertTrue(fixture.requestDescriptionFocus("Expand $title"))
            fixture.render("f24-keyboard-$title-focused")
            assertTrue(fixture.isFocusedControl("Expand $title"))
            fixture.assertColorVisible(FocusAccent)
            assertTrue(fixture.pressKey(key))
            fixture.render()
            assertEquals("Expanded", fixture.descriptionState("Collapse $title"))
            assertTrue(fixture.isFocusedControl("Collapse $title"))
          }
          fixture.revealText("internal/caller.go", "context-content")
          assertEquals(0, privileged)
          fixture.revealText("Refactor", "context-content")
          assertTrue(fixture.requestFocus("Refactor"))
          fixture.render()
          assertTrue(fixture.isFocusedControl("Refactor"))
          assertEquals(0, privileged, "Focus alone cannot prepare an edit")
        }
  }

  @Test
  fun scanKeyboardDisclosureAndOutputExpansionKeepVisibleFocusWithoutDispatchingWork() {
    val state = verifiedScanLayoutCases().first { it.first == "long-output" }.second
    val calls = mutableListOf<String>()
    ComposeVisualFixture(800, 400, 1.5f) {
          BugsWorkspacePane(
              state,
              BugsWorkspaceActions(
                  FindingActions(
                      { calls += "prepare" }, { _, _ -> calls += "write" }, { calls += "source" }),
                  { calls += "trust/start" },
                  { calls += "cancel" },
                  { calls += "analysis" },
                  { calls += "results/read" },
                  { calls += "status/read" }))
        }
        .use { fixture ->
          fixture.render()
          fixture.revealTextFullyWithin("Command and output", "result-overview")
          assertTrue(fixture.requestDescriptionFocus("Expand Command and output"))
          fixture.render("f20-keyboard-disclosure-focused")
          assertTrue(fixture.isFocusedControl("Expand Command and output"))
          fixture.assertColorVisible(FocusAccent)
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertTrue(fixture.isFocusedControl("Collapse Command and output"))
          assertEquals("Expanded", fixture.descriptionState("Collapse Command and output"))
          fixture.scrollBy(100_000f, "result-overview")
          fixture.render()
          fixture.revealTextFullyWithin("Show full available output", "scan-diagnostics")
          assertTrue(fixture.requestDescriptionFocus("Expand available diagnostic output"))
          fixture.render("f20-keyboard-output-focused")
          fixture.assertColorVisible(FocusAccent)
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertEquals("Expanded", fixture.descriptionState("Collapse available diagnostic output"))
          assertTrue(fixture.hasText(state.scan!!.phases.single().output))
          fixture.revealTextFullyWithin("Show preview", "scan-diagnostics")
          assertTrue(fixture.requestDescriptionFocus("Collapse available diagnostic output"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertFalse(fixture.hasText(state.scan.phases.single().output))
          fixture.revealTextFullyWithin("Command and output", "result-overview")
          assertTrue(fixture.requestDescriptionFocus("Collapse Command and output"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertEquals("Collapsed", fixture.descriptionState("Expand Command and output"))
          assertEquals(0, fixture.tagCount("scan-diagnostics"))
          fixture.revealTextFullyWithin("Refresh scan status", "result-overview")
          assertTrue(fixture.requestFocus("Refresh scan status"))
          fixture.render("f20-keyboard-recovery-focused")
          fixture.assertColorVisible(FocusAccent)
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(listOf("status/read"), calls, "Only explicit recovery emits a read intent")
        }
  }

  @Test
  fun securityKeyboardBrowseSearchAndDisclosureDoNotDispatchWork() {
    val original = securityPageFixture()
    val report = original.results!!.security.first()
    val page =
        original.copy(
            section =
                original.section.copy(
                    results =
                        original.results!!.copy(
                            security =
                                listOf(
                                    report.copy(
                                        findings =
                                            (1..60).map { number ->
                                              report.findings
                                                  .single()
                                                  .copy(
                                                      id =
                                                          "rule-${number.toString().padStart(2, '0')}",
                                                      title = "Rule finding $number")
                                            })))))
    val browser = newResultBrowserState(page)
    val calls = mutableListOf<String>()
    ComposeVisualFixture(800, 650, 1.5f) {
          SecurityWorkspacePane(
              SecurityWorkspacePaneState(page, resultIndexFixture(), browser),
              SecurityWorkspaceActions(
                  { _, _ -> calls += "prepare/provider" },
                  { _, _ -> calls += "source/read" },
                  { calls += "analysis/preview" },
                  FindingActions(
                      { calls += "semantic/provider" },
                      { _, _ -> calls += "triage/write" },
                      { calls += "semantic/read" }),
                  retryResults = { calls += "saved/read" },
                  reviewSecurityIntent = { calls += "analysis/navigation" }))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestDescriptionFocus("Inspect Rule finding 1"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertTrue(fixture.isDescriptionSelected("Inspect Rule finding 1"))
          assertTrue(fixture.requestDescriptionFocus("Inspect Rule finding 1"))
          assertTrue(fixture.pressKey(Key.DirectionDown))
          fixture.render()
          assertTrue(fixture.isFocusedControl("Inspect Rule finding 2"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertTrue(fixture.isDescriptionSelected("Inspect Rule finding 2"))
          fixture.revealText("Report metadata", "result-detail")
          assertTrue(fixture.requestDescriptionFocus("Expand Report metadata"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertEquals("Expanded", fixture.descriptionState("Collapse Report metadata"))
          assertTrue(fixture.hasText("Content hash"))
          assertTrue(fixture.requestDescriptionFocus("Collapse Report metadata"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertFalse(fixture.hasText("Content hash"))
          fixture.scrollBy(500f, "result-detail")
          fixture.render()
          val detailOffset = fixture.verticalScrollValue("result-detail")
          assertTrue(detailOffset > 0f)
          fixture.scrollBy(700f, "result-list")
          fixture.render()
          assertTrue(browser.listState.firstVisibleItemIndex > 0)
          assertEquals(detailOffset, fixture.verticalScrollValue("result-detail"), 5f)
          fixture.focusDescribedEditor("Filter results")
          fixture.setFocusedText("Rule finding 42")
          fixture.render()
          assertEquals("Rule finding 42", browser.query)
          assertTrue(fixture.hasText("Rule finding 42"))
          fixture.clickDescription("Clear filters")
          fixture.render()
          browser.listState.requestScrollToItem(59)
          fixture.render()
          fixture.awaitVisibleDescription("Inspect Rule finding 60")
          assertTrue(fixture.requestDescriptionFocus("Inspect Rule finding 60"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertTrue(fixture.isDescriptionSelected("Inspect Rule finding 60"))
          assertTrue(
              calls.isEmpty(),
              "Search, selection, scrolling and disclosure must stay local: $calls")
        }
  }

  @Test
  fun securityReviewEntryUsesKeyboardNavigationWithoutPreviewOrDispatch() {
    val page = securityPageFixture()
    val runState =
        ProjectAnalysisRunState(
            run = page.run, sections = mapOf(AnalysisResultKey("security") to page.section))
    var state by
        mutableStateOf(
            shellFocusState(resultProjectFixture()).let {
              it.copy(app = it.app.copy(workspace = Workspace.Security, analysisRun = runState))
            })
    var privilegedCalls = 0
    ComposeVisualFixture(800, 650, 1.5f) {
          FocusTestShell(state, onState = { state = it }, onOperation = { privilegedCalls++ })
        }
        .use { fixture ->
          fixture.render()
          fixture.revealText("Review Security intent", "result-overview")
          assertTrue(fixture.requestFocus("Review Security intent"))
          fixture.render()
          assertTrue(fixture.isFocusedControl("Review Security intent"))
          assertEquals(0, privilegedCalls)
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertEquals(Workspace.Analysis, state.app.workspace)
          assertEquals(runState, state.app.analysisRun)
          assertEquals(0, privilegedCalls)
          assertFalse(fixture.hasText("Include AI Security review"))

          state = state.copy(app = state.app.copy(workspace = Workspace.Security))
          fixture.render()
          fixture.revealText("Review Security intent", "result-overview")
          assertTrue(fixture.requestFocus("Review Security intent"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertEquals(Workspace.Analysis, state.app.workspace)
          assertEquals(runState, state.app.analysisRun)
          assertEquals(0, privilegedCalls)
        }
  }

  @Test
  fun performanceEvidenceEntryIsKeyboardReachableAndScopedToResultAndProject() {
    val original = performancePageFixture()
    val report = original.results!!.performance.single()
    val second = report.findings.single().copy(id = "another", title = "Another opportunity")
    var page by
        mutableStateOf(
            original.copy(
                section =
                    original.section.copy(
                        results =
                            original.results!!.copy(
                                performance =
                                    listOf(report.copy(findings = report.findings + second))))))
    var browser by mutableStateOf(newResultBrowserState(page))
    browser.choose("performance:main.go:perf")
    var actions = 0
    ComposeVisualFixture(800, 650, 1.5f) {
          PerformanceWorkspacePane(
              PerformanceWorkspacePaneState(page, null, browser = browser),
              PerformanceWorkspaceActions(
                  { actions++ },
                  { actions++ },
                  FindingActions({ actions++ }, { _, _ -> actions++ }, { actions++ }),
                  { actions++ },
                  { actions++ },
                  { actions++ },
                  { actions++ }))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestDescriptionFocus("Expand Explore benchmark evidence"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertTrue(fixture.isFocusedControl("Collapse Explore benchmark evidence"))
          assertEquals("Expanded", fixture.descriptionState("Collapse Explore benchmark evidence"))
          assertTrue(fixture.hasText("Benchmark comparison"))
          assertEquals(0, actions)

          fixture.clickDescription("Inspect Another opportunity")
          fixture.render()
          assertEquals("performance:main.go:another", browser.selectedKey)
          assertTrue(fixture.hasDescription("Expand Explore benchmark evidence"))
          assertFalse(fixture.hasText("Benchmark comparison"))
          assertTrue(fixture.requestDescriptionFocus("Expand Explore benchmark evidence"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertTrue(fixture.hasText("Benchmark comparison"))

          page = page.copy(project = page.project!!.copy(projectId = "new-project"))
          browser = newResultBrowserState(page)
          fixture.render()
          assertTrue(fixture.hasDescription("Expand Explore benchmark evidence"))
          assertFalse(fixture.hasText("Benchmark comparison"))
          assertEquals(0, actions)
        }
  }

  @Test
  fun benchmarkKeyboardTraversalRevealsChoicesCommandAndAdmissionWithoutImplicitActions() {
    for (activation in listOf(Key.Enter, Key.Spacebar)) {
      val page = performancePageFixture()
      val project = page.project!!
      val draft =
          DeclarationDraft(
              id = "draft",
              revision = 1,
              hash = "candidate",
              projectId = project.projectId,
              projectRevision = project.projectRevision,
              targetPath = "main.go",
              baseFileHash = "base",
              declaration = "func Run() {}",
              validation =
                  DeclarationValidation(
                      true, "strict_symbol", diff = UnifiedDiff("main.go", "main.go")))
      val choices =
          (1..24).map {
            GoBenchmarkChoice(
                "BenchmarkWork$it",
                listOf("go", "test", ".", "-bench", "^BenchmarkWork$it$"),
                "scope-$it")
          }
      val catalog =
          GoBenchmarkCatalog(
              draftId = draft.id,
              draftRevision = draft.revision,
              draftHash = draft.hash,
              projectId = project.projectId,
              projectRevision = project.projectRevision,
              baseFileHash = draft.baseFileHash,
              targetPath = draft.targetPath,
              available = true,
              trusted = activation == Key.Enter,
              benchmarks = choices)
      var snapshot by
          mutableStateOf(
              DesktopState(
                  projectState = ProjectWorkspaceState(project),
                  selection =
                      FileSelectionState(
                          selectedFile =
                              ProjectFileInfo(
                                  draft.targetPath,
                                  draft.baseFileHash,
                                  "main.go",
                                  language = "Go",
                                  sizeBytes = 1,
                                  lineCount = 20,
                                  modifiedAt = "",
                                  binary = false)),
                  review = DraftReviewState(draft = draft, editor = editableDraft(draft))))
      val calls = mutableListOf<String>()
      val browser = newResultBrowserState(page)
      browser.choose(performanceResults(page).single().row().key)
      // Keyboard qualification pumps real-sized frames through Compose's focus-scroll spring.
      ComposeVisualFixture(800, 650, 1.5f, frameDurationNanos = 16_000_000) {
            val evidence = snapshot.review.benchmark
            PerformanceWorkspacePane(
                PerformanceWorkspacePaneState(
                    page,
                    null,
                    browser = browser,
                    benchmarkCatalog = evidence.catalog,
                    selectedBenchmark = evidence.selected,
                    benchmarkDiscovery = evidence.discovery,
                    benchmarkEligibility = benchmarkEligibility(snapshot)),
                PerformanceWorkspaceActions(
                    { calls += "provider" },
                    { calls += "analysis" },
                    FindingActions(
                        { calls += "provider" },
                        { _, _ -> calls += "write" },
                        { calls += "source" }),
                    { calls += "source" },
                    loadBenchmarks = {
                      calls += "discovery/read"
                      snapshot =
                          snapshot.copy(
                              review =
                                  snapshot.review.copy(
                                      benchmark =
                                          BenchmarkEvidenceState(
                                              catalog = catalog,
                                              discovery = BenchmarkDiscoveryOutcome.Loaded)))
                    },
                    selectBenchmark = {
                      calls += "selection/local"
                      snapshot =
                          snapshot.copy(
                              review =
                                  snapshot.review.copy(
                                      benchmark = snapshot.review.benchmark.copy(selected = it)))
                    },
                    runBenchmark = { calls += "admission" }))
          }
          .use { fixture ->
            fixture.render()
            assertTrue(fixture.requestDescriptionFocus("Expand Explore benchmark evidence"))
            assertTrue(fixture.pressKey(activation))
            fixture.render()
            tabToBenchmarkControl(fixture, "benchmark-discovery")
            fixture.assertColorVisible(FocusAccent)
            assertTrue(calls.isEmpty())
            assertTrue(fixture.pressKey(activation))
            fixture.render()
            assertEquals(listOf("discovery/read"), calls)
            assertNull(snapshot.review.benchmark.selected)
            choices.forEachIndexed { index, choice ->
              val label = "Select benchmark ${choice.name}"
              tabToBenchmarkControl(fixture, "benchmark-choice-$index")
              fixture.awaitVisibleDescription(label)
              assertFalse(fixture.isDescriptionSelected(label))
              assertEquals("Not selected", fixture.descriptionState(label))
              assertNull(snapshot.review.benchmark.selected, "Focus is not selection")
            }
            fixture.render("f21-keyboard-choice-focus-$activation")
            fixture.assertColorVisible(FocusAccent)
            assertEquals(listOf("discovery/read"), calls)
            assertTrue(fixture.pressKey(activation))
            fixture.render()
            val selected = choices.last()
            assertEquals(selected, snapshot.review.benchmark.selected)
            assertEquals(listOf("discovery/read", "selection/local"), calls)
            assertTrue(fixture.isDescriptionSelected("Select benchmark ${selected.name}"))
            assertEquals("Selected", fixture.descriptionState("Select benchmark ${selected.name}"))
            tabToBenchmarkControl(fixture, "benchmark-copy-argv")
            awaitBenchmarkReveal(fixture, "benchmark-copy-argv", minimumHeight = 32f)
            fixture.render("f21-keyboard-copy-focus-$activation")
            fixture.awaitVisibleDescription("Copy selected benchmark argv")
            assertTrue(fixture.pressKey(activation))
            fixture.render()
            assertEquals(performanceBenchmarkArgv(selected.command), fixture.clipboardText())
            assertEquals(
                listOf("discovery/read", "selection/local"), calls, "Copying is not admission")
            tabToBenchmarkControl(fixture, "benchmark-argv")
            fixture.render("f21-keyboard-command-focus-$activation")
            awaitBenchmarkReveal(fixture, "benchmark-argv", minimumHeight = 140f)
            val commandBounds = fixture.descriptionBounds("Selected benchmark argv")
            val overviewBounds = fixture.taggedBounds("result-overview")
            assertTrue(
                commandBounds.top >= overviewBounds.top &&
                    commandBounds.bottom <= overviewBounds.bottom,
                "The focused command must be revealed within the overview: $commandBounds / $overviewBounds")
            for (text in
                listOf("Daemon-returned argv (read-only)") +
                    performanceBenchmarkArgv(selected.command).lines()) {
              val bounds = fixture.firstVisibleTextBounds(text)
              assertTrue(
                  bounds.height > 0f &&
                      bounds.top >= overviewBounds.top &&
                      bounds.bottom <= overviewBounds.bottom,
                  "Focused command text must be fully available: $text / $bounds")
            }
            assertEquals("Read-only", fixture.descriptionState("Selected benchmark argv"))
            fixture.assertColorVisible(FocusAccent)
            val run =
                if (catalog.trusted) "Run selected benchmark"
                else "Trust and run selected benchmark"
            tabToBenchmarkControl(fixture, "benchmark-run")
            awaitBenchmarkReveal(fixture, "benchmark-run", minimumHeight = 32f)
            fixture.render("f21-keyboard-run-focus-$activation")
            val runBounds = fixture.taggedBounds("benchmark-run")
            assertTrue(
                runBounds.height > 0f &&
                    runBounds.top >= overviewBounds.top &&
                    runBounds.bottom <= overviewBounds.bottom)
            assertTrue(fixture.isFocusedControl(run))
            assertFalse(fixture.hasText("Finding ID"), "Optional report metadata stays collapsed")
            assertFalse(
                fixture.hasText("Benchmark evidence"), "Optional measurements stay collapsed")
            for (required in
                listOf(
                    "Running benchmarks executes imported project code.",
                    "Baseline and candidate use copied workspaces; these are not a security sandbox.",
                    "Trust contract (separate from selected argv): go test ./...",
                    "This is broader than benchmark-only permission.")) {
              assertTrue(fixture.hasText(required), required)
            }
            // Revisit the disclosure from admission as well as from the selected choice.
            // Pending copy feedback must not move the next focused target out of view.
            repeat(3) {
              tabToBenchmarkControl(fixture, "benchmark-argv", reverse = true)
              awaitBenchmarkReveal(fixture, "benchmark-argv", minimumHeight = 140f)
              fixture.assertColorVisible(FocusAccent)
              tabToBenchmarkControl(fixture, "benchmark-copy-argv", reverse = true)
              awaitBenchmarkReveal(fixture, "benchmark-copy-argv", minimumHeight = 32f)
              fixture.awaitVisibleDescription("Copy selected benchmark argv")
              tabToBenchmarkControl(fixture, "benchmark-run")
              awaitBenchmarkReveal(fixture, "benchmark-run", minimumHeight = 32f)
              assertEquals(listOf("discovery/read", "selection/local"), calls)
            }
            fixture.resize(1280, 600)
            fixture.render()
            assertTrue(fixture.isFocusedControl(run))
            assertEquals(listOf("discovery/read", "selection/local"), calls)
            assertTrue(fixture.pressKey(activation))
            assertEquals(listOf("discovery/read", "selection/local", "admission"), calls)
          }
    }
  }

  @OptIn(ExperimentalComposeUiApi::class)
  @Test
  fun recordedBenchmarkKeyboardInspectionAndCopyFailureStayLocal() {
    for (activation in listOf(Key.Enter, Key.Spacebar)) {
      val page = performancePageFixture()
      val comparison =
          GoBenchmarkComparison(
              benchmark = "BenchmarkRun",
              status = "completed",
              command = listOf("go", "test", ".", "-count=5", "-benchtime=100ms", "-benchmem"),
              base = GoBenchmarkMeasurement(List(5) { GoBenchmarkSample(1000, 100.0, 10, 1) }),
              candidate =
                  GoBenchmarkMeasurement(List(5) { GoBenchmarkSample(1000, 80.0, 0, null) }))
      val clipboard =
          object : Clipboard {
            override val nativeClipboard = java.awt.datatransfer.Clipboard("benchmark-keyboard")
            var fail = true

            override suspend fun getClipEntry(): ClipEntry? =
                nativeClipboard.getContents(null)?.let(::ClipEntry)

            override suspend fun setClipEntry(clipEntry: ClipEntry?) {
              if (fail) error("Clipboard unavailable")
              nativeClipboard.setContents(clipEntry?.asAwtTransferable, null)
            }
          }
      var actions = 0
      ComposeVisualFixture(800, 650, 1.5f, frameDurationNanos = 16_000_000) {
            CompositionLocalProvider(LocalClipboard provides clipboard) {
              PerformanceWorkspacePane(
                  PerformanceWorkspacePaneState(page, null, benchmarkComparison = comparison),
                  PerformanceWorkspaceActions(
                      { actions++ },
                      { actions++ },
                      FindingActions({ actions++ }, { _, _ -> actions++ }, { actions++ }),
                      { actions++ },
                      loadBenchmarks = { actions++ },
                      selectBenchmark = { actions++ },
                      runBenchmark = { actions++ }))
            }
          }
          .use { fixture ->
            fixture.render()
            for (label in
                listOf(
                    "Explore benchmark evidence",
                    "Prior measurement details",
                    "Recorded conditions & identity",
                    "Returned sample details")) {
              fixture.revealTextFullyWithin(label, "result-overview")
              assertTrue(fixture.requestDescriptionFocus("Expand $label"))
              fixture.render()
              assertTrue(fixture.isFocusedControl("Expand $label"))
              fixture.assertColorVisible(FocusAccent)
              assertEquals("Collapsed", fixture.descriptionState("Expand $label"))
              assertEquals(0, actions, "Focus is not execution")
              assertTrue(fixture.pressKey(activation))
              fixture.render()
              assertTrue(fixture.isFocusedControl("Collapse $label"))
              assertEquals("Expanded", fixture.descriptionState("Collapse $label"))
            }
            val assessment =
                performanceBenchmarkPresentation(comparison, null, null, priorEvidence = true)
            for (metric in assessment.metrics) {
              for ((side, values) in
                  listOf("Baseline" to metric.base, "Candidate" to metric.candidate)) {
                assertTrue(
                    fixture.hasDescription("${metric.label}, $side median: ${values.display()}"))
              }
            }
            assertTrue(
                fixture.hasText(
                    "Candidate sample 1: iterations=1000; ns/op=80.0; B/op=0; allocs/op=unavailable"))
            assertFalse(fixture.hasEditableText(withinTag = "benchmark-measurement-evidence"))
            tabToBenchmarkControl(fixture, "benchmark-copy-evidence")
            awaitBenchmarkReveal(fixture, "benchmark-copy-evidence", 32f)
            fixture.assertColorVisible(FocusAccent)
            assertTrue(fixture.isFocusedControl("Copy displayed benchmark evidence"))
            assertEquals(0, actions)
            assertTrue(fixture.pressKey(activation))
            fixture.render()
            assertTrue(fixture.hasText("Could not copy benchmark evidence: Clipboard unavailable"))
            assertEquals(0, actions, "Clipboard failure is not a workflow failure")
            clipboard.fail = false
            assertTrue(fixture.pressKey(activation))
            fixture.render()
            assertEquals(
                performanceBenchmarkCopyText(comparison, assessment, true, true, true),
                clipboard.nativeClipboard.getData(java.awt.datatransfer.DataFlavor.stringFlavor))
            assertFalse(fixture.hasText("Could not copy benchmark evidence: Clipboard unavailable"))
            fixture.resize(1280, 600)
            fixture.render()
            assertTrue(fixture.isFocusedControl("Copy displayed benchmark evidence"))
            assertTrue(fixture.hasDescription("Collapse Returned sample details"))
            for (label in
                listOf(
                    "Returned sample details",
                    "Recorded conditions & identity",
                    "Prior measurement details")) {
              fixture.revealTextFullyWithin(label, "result-overview")
              assertTrue(fixture.requestDescriptionFocus("Collapse $label"))
              assertTrue(fixture.pressKey(if (activation == Key.Enter) Key.Spacebar else Key.Enter))
              fixture.render()
              assertEquals("Collapsed", fixture.descriptionState("Expand $label"))
            }
            assertEquals(0, actions, "Inspection, selection/copy and resize remain passive")
          }
    }
  }

  @Test
  fun latestResponseKeyboardInspectionAndCopyStaySeparateFromPriorMeasurements() {
    val response =
        GoBenchmarkComparison(
            status = "failed",
            reason = "Execution failed before measurements",
            benchmark = "BenchmarkResponse",
            command = listOf("go", "test", "response"),
            base = GoBenchmarkMeasurement(),
            candidate = null)
    val prior =
        GoBenchmarkComparison(
            status = "completed",
            benchmark = "BenchmarkPrior",
            base = GoBenchmarkMeasurement(List(5) { GoBenchmarkSample(1, 100.0, 0, 0) }),
            candidate = GoBenchmarkMeasurement(List(5) { GoBenchmarkSample(1, 80.0, 0, 0) }))
    for (activation in listOf(Key.Enter, Key.Spacebar)) {
      for (retainPrior in listOf(false, true)) {
        var requests = 0
        ComposeVisualFixture(800, 650, 1.5f, frameDurationNanos = 16_000_000) {
              PerformanceWorkspacePane(
                  PerformanceWorkspacePaneState(
                      performancePageFixture(),
                      null,
                      benchmarkComparison = prior.takeIf { retainPrior },
                      benchmarkLatestOutcome = BenchmarkComparisonOutcome(response)),
                  PerformanceWorkspaceActions(
                      { requests++ },
                      { requests++ },
                      FindingActions({ requests++ }, { _, _ -> requests++ }, { requests++ }),
                      { requests++ },
                      loadBenchmarks = { requests++ },
                      selectBenchmark = { requests++ },
                      runBenchmark = { requests++ }))
            }
            .use { fixture ->
              fixture.render()
              for (label in
                  listOf(
                      "Explore benchmark evidence",
                      "Latest response details",
                      "Recorded conditions & identity",
                      "Returned sample details")) {
                fixture.revealTextFullyWithin(label, "result-overview")
                assertTrue(fixture.requestDescriptionFocus("Expand $label"))
                fixture.render()
                assertTrue(fixture.isFocusedControl("Expand $label"))
                fixture.assertColorVisible(FocusAccent)
                assertEquals("Collapsed", fixture.descriptionState("Expand $label"))
                assertEquals(0, requests, "Focus alone is passive")
                assertTrue(fixture.pressKey(activation))
                fixture.render()
                assertEquals("Expanded", fixture.descriptionState("Collapse $label"))
              }
              assertTrue(fixture.hasText("Baseline returned samples: 0"))
              assertTrue(
                  fixture.hasText(
                      "Candidate returned samples: unavailable · measurement not returned"))
              assertFalse(fixture.hasText("Baseline median"))
              assertFalse(fixture.hasEditableText(withinTag = "benchmark-latest-response"))
              tabToBenchmarkControl(fixture, "benchmark-copy-response")
              awaitBenchmarkReveal(fixture, "benchmark-copy-response", 32f)
              fixture.assertColorVisible(FocusAccent)
              assertTrue(fixture.isFocusedControl("Copy displayed response details"))
              fixture.failClipboardWrites = true
              assertTrue(fixture.pressKey(activation))
              fixture.render()
              assertTrue(
                  fixture.hasText("Could not copy benchmark evidence: Clipboard unavailable"))
              fixture.failClipboardWrites = false
              assertTrue(fixture.pressKey(activation))
              fixture.render()
              assertEquals(
                  performanceBenchmarkResponseCopyText(response, true, true),
                  fixture.clipboardText())
              assertFalse(fixture.clipboardText().contains("BenchmarkPrior"))
              fixture.resize(1280, 600)
              fixture.render()
              assertTrue(fixture.isFocusedControl("Copy displayed response details"))
              fixture.render("f22-keyboard-latest-response-$activation-$retainPrior")
              assertEquals(
                  0, requests, "Disclosure, copy failure/success and resizing remain local")
            }
      }
    }
  }

  @Test
  fun benchmarkKeyboardBlockedReasonsRemainAccessibleWithoutActivatingDisabledRecovery() {
    for (discovery in
        listOf(
            BenchmarkDiscoveryOutcome.Failed("Lookup timed out. Retry explicitly."),
            BenchmarkDiscoveryOutcome.Unavailable("No compatible benchmark."),
            BenchmarkDiscoveryOutcome.Invalidated)) {
      val page = performancePageFixture()
      var reads = 0
      var privileged = 0
      val blocked = benchmarkEligibility(DesktopState())
      ComposeVisualFixture(800, 650, 1.5f, frameDurationNanos = 16_000_000) {
            PerformanceWorkspacePane(
                PerformanceWorkspacePaneState(
                    page, null, benchmarkDiscovery = discovery, benchmarkEligibility = blocked),
                PerformanceWorkspaceActions(
                    { privileged++ },
                    {},
                    FindingActions({ privileged++ }, { _, _ -> privileged++ }, {}),
                    {},
                    loadBenchmarks = { reads++ },
                    runBenchmark = { privileged++ }))
          }
          .use { fixture ->
            fixture.render()
            assertTrue(fixture.requestDescriptionFocus("Expand Explore benchmark evidence"))
            assertTrue(fixture.pressKey(Key.Spacebar))
            fixture.render()
            assertTrue(fixture.hasText(blocked.discoveryBlockedReason!!))
            assertTrue(fixture.isDisabled("Refresh compatible benchmarks"))
            assertEquals(
                blocked.discoveryBlockedReason,
                fixture.stateDescription("Refresh compatible benchmarks"))
            assertFalse(fixture.requestFocus("Refresh compatible benchmarks"))
            fixture.pressKey(Key.Tab)
            fixture.render()
            assertEquals(0, reads + privileged)
          }
    }
  }

  @Test
  fun bugsKeyboardInspectionKeepsTextEntryAndBothScrollRegionsIndependent() {
    val base = resultPageFixture("bugs")
    val findings =
        (1..240).map { number ->
          base.semantic
              .first()
              .copy(
                  id = "f17-$number",
                  title = "Finding $number",
                  message = "Long evidence line $number ".repeat(80).trim(),
                  location = FindingLocation("internal/handler$number.go", startLine = number))
        }
    val page =
        base.copy(section = base.section.copy(results = base.results!!.copy(semantic = findings)))
    val browser = newResultBrowserState(page)
    var external = 0
    ComposeVisualFixture(800, 650, 1.5f) {
          BugsWorkspacePane(
              BugsWorkspacePaneState(findings, null, false, page, browser),
              BugsWorkspaceActions(
                  FindingActions({ external++ }, { _, _ -> external++ }, { external++ }),
                  { external++ },
                  { external++ }))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestDescriptionFocus("Inspect Finding 1"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertTrue(fixture.isDescriptionSelected("Inspect Finding 1"))
          assertTrue(fixture.requestDescriptionFocus("Inspect Finding 1"))
          assertTrue(fixture.pressKey(Key.DirectionDown))
          fixture.render()
          assertTrue(fixture.isFocusedControl("Inspect Finding 2"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertTrue(fixture.isDescriptionSelected("Inspect Finding 2"))
          assertEquals("semantic:${findingDisplayKey(findings[1])}", browser.selectedKey)
          fixture.scrollBy(700f, "result-detail")
          fixture.render()
          val detail = fixture.verticalScrollValue("result-detail")
          assertTrue(detail > 0f)
          fixture.scrollBy(1300f, "result-list")
          fixture.render()
          assertTrue(browser.listState.firstVisibleItemIndex > 0)
          assertTrue(fixture.verticalScrollValue("result-detail") >= detail - 5f)
          fixture.revealText("Filter results", "result-overview")
          fixture.focusDescribedEditor("Filter results")
          fixture.setFocusedText("Finding 2")
          fixture.render()
          fixture.pressKey(Key.DirectionDown)
          fixture.render()
          assertEquals("Finding 2", browser.query)
          assertEquals("semantic:${findingDisplayKey(findings[1])}", browser.selectedKey)
          assertEquals(0, external)
        }
  }

  @Test
  fun diagramViewerTrapsFocusAndRestoresTheOpenerAfterCloseAndEscape() {
    val source = "flowchart LR\n A --> B"
    for (attempt in 0..1) {
      val view = DiagramViewState()
      var renders = 0
      ComposeVisualFixture(800, 650, 1.5f) {
            MermaidDiagram(
                source,
                "Flow 1",
                viewState = view,
                render = {
                  renders++
                  MermaidImage(ImageBitmap(1200, 900), 1200f, 900f)
                })
            RestoreDiagramFocus(view)
            if (view.showDiagram)
                MermaidDiagramViewer(summaryDiagramInput(source), "Flow 1", "Flow 1", view)
          }
          .use { fixture ->
            fixture.awaitDescription("Expand Flow 1 diagram", "Preview")
            assertTrue(fixture.requestDescriptionFocus("Expand Flow 1 diagram"))
            assertTrue(fixture.pressKey(if (attempt == 0) Key.Enter else Key.Spacebar))
            fixture.awaitDescription("Close Flow 1 diagram")
            assertTrue(fixture.isFocusedControl("Close Flow 1 diagram"))
            fixture.pressKey(Key.Tab)
            fixture.render()
            assertTrue(
                fixture.isFocusedControl("Zoom out Flow 1"), "Tab from Close should wrap to zoom")
            fixture.pressKey(Key.Tab)
            fixture.render()
            assertTrue(fixture.isFocusedControl("Reset zoom Flow 1"))
            fixture.pressKey(Key.Tab, shift = true)
            fixture.render()
            assertTrue(fixture.isFocusedControl("Zoom out Flow 1"))
            assertTrue(fixture.requestDescriptionFocus("Close Flow 1 diagram"))
            if (attempt == 0) assertTrue(fixture.pressKey(Key.Escape))
            else assertTrue(fixture.pressKey(Key.Enter))
            fixture.render()
            assertFalse(fixture.hasDescription("Close Flow 1 diagram"))
            assertTrue(fixture.isFocusedControl("Expand Flow 1 diagram"))
            assertEquals(1, renders)
          }
    }
  }

  @Test
  fun summaryArchitectureViewerReturnsFocusToItsSurvivingExpandControl() {
    val project = resultProjectFixture()
    val overview =
        ProjectOverview(
            projectId = project.projectId,
            projectRevision = project.projectRevision,
            analysis =
                StructuredProjectAnalysis(
                    status = "fresh", architecture = "flowchart LR\n A --> B"))
    ComposeVisualFixture(800, 650, 1.5f) {
          ProjectSummaryPane(
              overview,
              project,
              {},
              diagramRender = { MermaidImage(ImageBitmap(32, 32), 32f, 32f) })
        }
        .use { fixture ->
          fixture.render()
          fixture.revealText("Expand diagram", "summary-scroll")
          assertTrue(fixture.requestDescriptionFocus("Expand Architecture diagram"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.awaitDescription("Close Architecture diagram")
          assertTrue(fixture.pressKey(Key.Escape))
          fixture.render()
          assertTrue(fixture.isFocusedControl("Expand Architecture diagram"))
        }
  }

  @Test
  fun diagramCanvasAndLongSourceCanBeScrolledFromTheKeyboardWithoutLosingControls() {
    val source =
        "flowchart LR\n A --> B\n" +
            (1..120).joinToString("\n") { "note $it " + "long ".repeat(80) }
    val view = DiagramViewState()
    ComposeVisualFixture(800, 650, 1.5f) {
          MermaidDiagram(
              source,
              "Architecture",
              viewState = view,
              render = { MermaidImage(ImageBitmap(1200, 900), 1200f, 900f) })
          RestoreDiagramFocus(view)
          if (view.showDiagram)
              MermaidDiagramViewer(
                  summaryDiagramInput(source), "Architecture", "Architecture", view)
        }
        .use { fixture ->
          fixture.awaitDescription("Expand Architecture diagram", "Preview")
          assertTrue(fixture.requestDescriptionFocus("Expand Architecture diagram"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.awaitDescription("Architecture diagram horizontal scroll")
          assertTrue(fixture.requestDescriptionFocus("Architecture diagram vertical scroll"))
          assertTrue(fixture.pressKey(Key.PageDown))
          fixture.render()
          assertTrue(fixture.scrollPosition("diagram-vertical-scroll", horizontal = false) > 0f)
          assertTrue(fixture.requestDescriptionFocus("Architecture diagram horizontal scroll"))
          assertTrue(fixture.pressKey(Key.DirectionRight))
          fixture.render()
          assertTrue(fixture.scrollPosition("diagram-horizontal-scroll", horizontal = true) > 0f)
          fixture.clickText("Mermaid source")
          fixture.render()
          assertTrue(fixture.requestDescriptionFocus("Saved diagram source vertical scroll"))
          assertTrue(fixture.pressKey(Key.PageDown))
          fixture.render()
          assertTrue(fixture.scrollPosition("diagram-source-scroll", horizontal = false) > 0f)
          assertTrue(fixture.requestDescriptionFocus("Saved diagram source horizontal scroll"))
          assertTrue(fixture.pressKey(Key.DirectionRight))
          fixture.render()
          assertTrue(
              fixture.scrollPosition("diagram-source-horizontal-scroll", horizontal = true) > 0f)
          assertTrue(fixture.requestDescriptionFocus("Copy saved content for Architecture"))
          assertTrue(fixture.requestDescriptionFocus("Close Architecture diagram"))
          assertTrue(fixture.pressKey(Key.Escape))
          fixture.render()
          assertTrue(fixture.isFocusedControl("Expand Architecture diagram"))
        }
  }

  @Test
  fun summaryViewerFallsBackWithoutMovingScrollWhenLazyOpenerIsDisposed() {
    lateinit var focusManager: FocusManager
    val project = resultProjectFixture()
    val overview =
        ProjectOverview(
            projectId = project.projectId,
            projectRevision = project.projectRevision,
            analysis =
                StructuredProjectAnalysis(
                    status = "fresh",
                    architecture = "flowchart LR\n A --> B",
                    flows = (1..12).map { "Flow $it: " + "saved context ".repeat(24) }))
    ComposeVisualFixture(800, 650, 1.5f) {
          focusManager = LocalFocusManager.current
          ProjectSummaryPane(
              overview,
              project,
              {},
              diagramRender = { MermaidImage(ImageBitmap(32, 32), 32f, 32f) })
        }
        .use { fixture ->
          fixture.render()
          fixture.revealText("Expand diagram", "summary-scroll")
          fixture.clickDescription("Expand Architecture diagram")
          fixture.awaitDescription("Close Architecture diagram")
          // An offscreen lazy item remains pinned while it owns focus; release it before scrolling.
          focusManager.clearFocus(force = true)
          fixture.scrollBy(100_000f, "summary-scroll")
          fixture.render()
          fixture.scrollBy(-4_000f, "summary-scroll")
          fixture.render()
          val position = fixture.scrollPosition("summary-scroll", horizontal = false)
          val maximum = fixture.scrollMaximum("summary-scroll", false)
          assertTrue(position > 0f)
          assertFalse(fixture.hasDescription("Expand Architecture diagram"))
          assertTrue(fixture.requestDescriptionFocus("Close Architecture diagram"))
          assertTrue(fixture.pressKey(Key.Escape))
          fixture.render()
          assertFalse(fixture.hasDescription("Close Architecture diagram"))
          assertTrue(fixture.isTaggedNodeFocused("summary-scroll"))
          // Lazy item measurements can change when the viewer closes; preserve the visible
          // context relative to the end of the list rather than assuming a fixed pixel height.
          assertEquals(
              maximum - position,
              fixture.scrollMaximum("summary-scroll", false) -
                  fixture.scrollPosition("summary-scroll", horizontal = false))
        }
  }

  @Test
  fun diagramLoadingFailureAndZoomBoundsRemainNamedAndReachableAtLargeText() {
    val source = "flowchart LR\n A --> B"
    val release = CompletableDeferred<Unit>()
    ComposeVisualFixture(800, 600, 1.5f) {
          MermaidDiagram(
              source,
              "Architecture",
              render = {
                release.await()
                throw IllegalArgumentException("Invalid saved diagram")
              })
        }
        .use { fixture ->
          fixture.render()
          assertEquals("Rendering diagram", fixture.stateDescription("Expand diagram"))
          assertTrue(fixture.requestDescriptionFocus("Expand Architecture diagram"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.awaitDescription("Close Architecture diagram")
          assertTrue(fixture.hasText("Rendering diagram…"))
          release.complete(Unit)
          fixture.awaitDescription("Expand Architecture diagram", "Diagram failed")
          fixture.revealText("Diagram unavailable: Invalid saved diagram", "ide-dialog-body")
          assertTrue(fixture.requestDescriptionFocus("Mermaid source for Architecture"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          fixture.revealText("Copy source", "ide-dialog-body")
          assertTrue(fixture.requestDescriptionFocus("Copy saved content for Architecture"))
          assertTrue(fixture.requestDescriptionFocus("Zoom out Architecture"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertEquals("Minimum zoom 75%", fixture.descriptionState("Zoom out Architecture"))
          assertTrue(fixture.isDisabled("−"))
          assertTrue(fixture.requestDescriptionFocus("Close Architecture diagram"))
          fixture.pressKey(Key.Tab)
          fixture.render()
          assertTrue(fixture.isFocusedControl("Reset zoom Architecture"))
          fixture.pressKey(Key.Tab, shift = true)
          fixture.render()
          assertTrue(fixture.isFocusedControl("Close Architecture diagram"))
          repeat(5) {
            assertTrue(fixture.requestDescriptionFocus("Zoom in Architecture"))
            assertTrue(fixture.pressKey(Key.Enter))
            fixture.render()
          }
          assertEquals("Maximum zoom 200%", fixture.descriptionState("Zoom in Architecture"))
          assertTrue(fixture.isDisabled("+"))
          assertTrue(fixture.requestDescriptionFocus("Reset zoom Architecture"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertEquals("Zoom 100%", fixture.descriptionState("Reset zoom Architecture"))
          assertTrue(fixture.requestDescriptionFocus("Close Architecture diagram"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertTrue(fixture.isFocusedControl("Expand Architecture diagram"))
        }
  }

  @Test
  fun canceledDirectoryChooserLeavesTheExistingFailureAndSideEffectsUntouched() {
    val failure =
        ProjectOpeningAttempt(
            8,
            "/remembered",
            ProjectOpeningKind.Restore,
            ProjectOpeningOutcome.Failed("Missing saved analysis"))
    var state = DesktopState(projectState = ProjectWorkspaceState(openingAttempt = failure))
    var selections = 0
    var saves = 0
    var terminalCloses = 0
    var draftDiscards = 0
    chooseProjectDirectory({ null }) {
      selections++
      state = DesktopState()
      saves++
      terminalCloses++
      draftDiscards++
    }
    assertEquals(0, selections + saves + terminalCloses + draftDiscards)
    assertEquals(failure, state.projectState.openingAttempt)
    chooseProjectDirectory({ java.io.File("/chosen") }) {
      assertEquals("/chosen", it)
      selections++
    }
    assertEquals(1, selections)
  }

  @OptIn(InternalComposeUiApi::class)
  @Test
  fun openShortcutUsesTheSameAttemptAvailabilityAsThePointer() {
    var opens = 0
    val actions = DesktopShellProjectActions({ opens++ }, {}, {})
    val editor =
        DesktopShellEditorState(
            EditorProgressUiState(EditorProgress.Inspect, ""),
            EditorContextualActions(false, false, false, false, false),
            false,
            false)
    fun shortcut(state: DesktopState, switchPending: Boolean = false): Boolean =
        handleDesktopShortcut(
            KeyEvent(Key.O, KeyEventType.KeyDown, isCtrlPressed = true),
            desktopShellMode(state),
            state,
            editor,
            actions,
            switchPending,
            DesktopShellEditorActions({}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}),
            DesktopShellPaletteActions({}, {}, {}, {}, {}, {}, {}),
            onDismissTransient = { false },
            onWorkspaceSelected = {},
            terminal = null,
            onTerminalSelected = {})
    val unrelatedJob = DesktopState(jobs = JobState(loading = true))
    assertTrue(shortcut(unrelatedJob))
    assertEquals(1, opens)
    val opening =
        unrelatedJob.copy(
            projectState =
                ProjectWorkspaceState(
                    openingAttempt =
                        ProjectOpeningAttempt(1, "/remembered", ProjectOpeningKind.Restore)))
    assertFalse(projectOpenAvailable(opening.projectState.openingAttempt))
    assertFalse(shortcut(opening))
    assertEquals(1, opens)
    val failed =
        opening.copy(
            projectState =
                opening.projectState.copy(
                    openingAttempt =
                        ProjectOpeningAttempt(
                            1,
                            "/remembered",
                            ProjectOpeningKind.Restore,
                            ProjectOpeningOutcome.Failed("Missing"))))
    assertTrue(shortcut(failed))
    assertEquals(2, opens)
    assertTrue(
        shortcut(
            failed.copy(projectState = failed.projectState.copy(project = resultProjectFixture()))))
    assertEquals(3, opens)
    assertFalse(shortcut(failed, switchPending = true))
    assertEquals(3, opens)
    val project = resultProjectFixture()
    val indexing =
        failed.copy(
            projectState =
                failed.projectState.copy(
                    project = project,
                    indexingAttempt =
                        ProjectIndexingAttempt(
                            1, project.projectId, project.projectRevision, project.path)))
    assertTrue(shortcut(indexing), "Indexing does not start a switch; opening may invalidate it")
    assertEquals(4, opens)
    assertFalse(
        shortcut(
            opening.copy(
                projectState =
                    opening.projectState.copy(
                        openingAttempt =
                            opening.projectState.openingAttempt!!.copy(
                                kind = ProjectOpeningKind.Import)))))
    assertEquals(4, opens)
  }

  @Test
  fun disablingFocusedOpenMovesFocusToOpeningStatusWithoutDispatchingWork() {
    var opens = 0
    var retries = 0
    var state by mutableStateOf(DesktopState())
    ComposeVisualFixture(800, 650) {
          ProjectLanding(
              state,
              DesktopShellProjectActions({ opens++ }, {}, {}, { retries++ }),
              FocusRequester())
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestFocus("Open project…"))
          fixture.render()
          assertTrue(fixture.isFocusedControl("Open project…"))
          state =
              state.copy(
                  projectState =
                      ProjectWorkspaceState(
                          openingAttempt =
                              ProjectOpeningAttempt(1, "/remembered", ProjectOpeningKind.Restore)))
          fixture.render()
          assertTrue(fixture.isDisabled("Open project…"))
          assertTrue(fixture.isTaggedNodeFocused("project-opening-focus"))
          assertEquals(0, opens + retries)
          state =
              state.copy(
                  projectState =
                      state.projectState.copy(
                          openingAttempt =
                              state.projectState.openingAttempt!!.copy(
                                  outcome = ProjectOpeningOutcome.Failed("Missing"))))
          fixture.render()
          assertTrue(fixture.isFocusedControl("Open project…"))
          assertEquals(0, opens + retries)
        }
  }

  @Test
  fun cancelingChooserOrSwitchReviewRestoresLandingOpenFocusWithoutOpeningProject() {
    var pending by mutableStateOf(false)
    var reviewing by mutableStateOf(false)
    var requests = 0
    var imports = 0
    val review =
        PendingProjectSwitch(
            1,
            "/next",
            ProjectSwitchContext(
                null,
                SwitchDraftIdentity(null, null, null),
                SwitchAnalyzeDestination(ScopedModel(scope = ModelScope.Analyze.wireValue)),
                false),
            SwitchReviewStage.Final)
    ComposeVisualFixture(800, 650) {
          Box {
            ProjectLanding(
                DesktopState(),
                DesktopShellProjectActions(
                    importProject = {
                      requests++
                      pending = true
                    },
                    reindexProject = {},
                    reconnect = {}),
                FocusRequester(),
                switchPending = pending)
            if (reviewing) {
              ProjectSwitchReviewDialog(
                  review,
                  ScopedModel(scope = ModelScope.Analyze.wireValue),
                  false,
                  TerminalWorkspaceState(),
                  SwitchCleanupFeedback(),
                  onCancel = {
                    reviewing = false
                    pending = false
                  },
                  onDraftApproved = {},
                  onProviderConfirmed = {},
                  onProviderApproved = {},
                  onReviewApproved = {},
                  onCommit = { imports++ })
            }
          }
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestFocus("Open project…"))
          fixture.render()
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertTrue(fixture.isDisabled("Open project…"))
          pending = false // Native chooser canceled without creating an opening attempt.
          fixture.render()
          assertTrue(fixture.isFocusedControl("Open project…"))
          assertEquals(1, requests)
          assertEquals(0, imports)

          assertTrue(fixture.pressKey(Key.Enter))
          reviewing = true
          fixture.render()
          assertTrue(fixture.isDisabled("Open project…"))
          assertTrue(fixture.isFocusedControl("Cancel switch"))
          assertTrue(fixture.pressKey(Key.Escape))
          fixture.render()
          assertTrue(fixture.isFocusedControl("Open project…"))
          assertEquals(2, requests)
          assertEquals(0, imports)
        }
  }

  @Test
  fun disappearingRetryMovesFocusWithoutDispatchingAnotherOperation() {
    var retries = 0
    var opens = 0
    val failed =
        ProjectOpeningAttempt(
            1, "/remembered", ProjectOpeningKind.Restore, ProjectOpeningOutcome.Failed("Missing"))
    var state by
        mutableStateOf(DesktopState(projectState = ProjectWorkspaceState(openingAttempt = failed)))
    ComposeVisualFixture(800, 650) {
          ProjectLanding(
              state,
              DesktopShellProjectActions({ opens++ }, {}, {}, { retries++ }),
              FocusRequester())
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestFocus("Retry restore"))
          fixture.render()
          assertEquals(0, retries + opens)
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(1, retries)
          state =
              state.copy(
                  projectState =
                      state.projectState.copy(
                          openingAttempt =
                              failed.copy(requestId = 2, outcome = ProjectOpeningOutcome.Opening)))
          fixture.render()
          assertTrue(fixture.isTaggedNodeFocused("project-opening-focus"))
          assertEquals(1, retries + opens)
          state =
              state.copy(
                  projectState =
                      state.projectState.copy(
                          openingAttempt =
                              failed.copy(
                                  requestId = 2,
                                  outcome = ProjectOpeningOutcome.Failed("Still missing"))))
          fixture.render()
          assertTrue(fixture.isFocusedControl("Open project…"))
          assertEquals(1, retries + opens)
        }
  }

  @Test
  fun explorerFileActivationSharesConfirmationAndCancelEscapePreserveAllWork() {
    for (work in listOf("draft", "session", "composer")) {
      for (action in listOf("cancel", "escape", "confirm")) {
        FileNavigationUiFixture(work).use { navigation ->
          val previous = navigation.presenter.snapshot.value.state
          val originalInput = navigation.input()
          ComposeVisualFixture(800, 650) {
                ExplorerPane(
                    ExplorerPaneState(previous.index, "main.go", "", emptySet(), false),
                    ExplorerPaneActions({}, {}, {}, {}, navigation::route),
                    Modifier.fillMaxSize())
                navigation.DiscardDialog()
              }
              .use { fixture ->
                fixture.render()
                fixture.clickText("main.go")
                fixture.render()
                assertNull(navigation.pending)
                navigation.runPending()
                assertTrue(navigation.calls.isEmpty())
                assertEquals(originalInput, navigation.input())
                fixture.clickText("other.go")
                fixture.render()
                assertTrue(navigation.pending != null)
                assertTrue(
                    fixture.isFocusedControl(if (work == "draft") "Keep draft" else "Keep work"))
                navigation.runPending()
                assertTrue(navigation.calls.isEmpty())
                assertEquals(
                    previous.selection, navigation.presenter.snapshot.value.state.selection)
                assertEquals(previous.chat, navigation.presenter.snapshot.value.state.chat)
                assertEquals(previous.review, navigation.presenter.snapshot.value.state.review)
                assertEquals(originalInput, navigation.input())
                when (action) {
                  "cancel" -> fixture.clickText(if (work == "draft") "Keep draft" else "Keep work")
                  "escape" -> assertTrue(fixture.pressKey(Key.Escape))
                  "confirm" -> {
                    assertTrue(
                        fixture.requestFocus(
                            if (work == "draft") "Discard draft" else "Discard work"))
                    assertTrue(fixture.pressKey(Key.Enter))
                  }
                }
                fixture.render()
                navigation.runPending()
                assertNull(navigation.pending)
                if (action == "confirm") {
                  assertEquals(
                      "other.go", navigation.presenter.snapshot.value.state.selectedFile?.path)
                  assertEquals(1, navigation.clears)
                  assertEquals(1, navigation.calls.count { it.contains("files/info?") })
                } else {
                  assertEquals(
                      previous.selection, navigation.presenter.snapshot.value.state.selection)
                  assertEquals(previous.chat, navigation.presenter.snapshot.value.state.chat)
                  assertEquals(previous.review, navigation.presenter.snapshot.value.state.review)
                  assertEquals(originalInput, navigation.input())
                  assertEquals(0, navigation.clears)
                  assertTrue(navigation.calls.isEmpty())
                }
              }
        }
      }
    }
  }

  @Test
  fun discardPromptFocusesKeepDraftAndEscapeCannotDiscard() {
    var visible by mutableStateOf(true)
    var cancels = 0
    var discards = 0
    ComposeVisualFixture(800, 650) {
          if (visible)
              DraftDiscardDialog(
                  CurrentEditIdentity(ChatEditMode.ReplaceSymbol, "main.go", "Run", true),
                  "edit Run",
                  onDiscard = { discards++ },
                  onCancel = {
                    cancels++
                    visible = false
                  })
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.isFocusedControl("Keep draft"))
          assertFalse(fixture.isFocusedControl("Discard draft"))
          assertTrue(fixture.pressKey(Key.Escape))
          fixture.render()
          assertEquals(1, cancels)
          assertEquals(0, discards)
        }
  }

  @Test
  fun importConsentPromptDismissalCannotConfirmOrImport() {
    var visible by mutableStateOf(true)
    var confirmations = 0
    var imports = 0
    var cancels = 0
    ComposeVisualFixture(800, 650) {
          if (visible)
              ProjectImportConfirmationDialog(
                  model = ScopedModel(scope = ModelScope.Analyze.wireValue, remoteProvider = true),
                  confirmed = false,
                  onConfirmed = { confirmations++ },
                  onImport = { imports++ },
                  onCancel = {
                    cancels++
                    visible = false
                  })
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.isFocusedControl("Cancel"))
          assertFalse(fixture.isFocusedControl("Import project"))
          assertTrue(fixture.pressKey(Key.Escape))
          fixture.render()
          assertEquals(1, cancels)
          assertEquals(0, confirmations)
          assertEquals(0, imports)
        }
  }

  @Test
  fun wrappedToolbarKeepsProjectMenuAndSearchKeyboardReachable() {
    val name = "Long project identity for keyboard traversal at scaled widths"
    for ((width, height) in listOf(800 to 650, 1280 to 600)) {
      for (scale in listOf(1.25f, 1.5f)) {
        var imports = 0
        var reindexes = 0
        var reconnects = 0
        var searches = 0
        ComposeVisualFixture(width, height, scale) {
              MainToolbar(
                  ToolbarState(
                      resultProjectFixture().copy(name = name),
                      false,
                      "",
                      ConnectionState(label = "Disconnected"),
                      GitStatus(available = true, branch = "feature/a-long-branch-name")),
                  ToolbarActions({ imports++ }, { reindexes++ }, { reconnects++ }, { searches++ }))
            }
            .use { fixture ->
              fixture.render()
              fixture.awaitVisibleDescription(name)
              fixture.awaitVisibleDescription("Search files, symbols, commands")
              assertTrue(fixture.requestDescriptionFocus(name))
              fixture.render()
              assertTrue(fixture.isFocusedControl(name))
              assertTrue(fixture.pressKey(Key.Tab))
              fixture.render("header-search-focus-$width-$height-$scale")
              assertTrue(fixture.isFocusedControl("Search files, symbols, commands"))
              fixture.assertColorVisible(FocusAccent)
              assertTrue(fixture.pressKey(Key.Enter))
              assertEquals(1, searches)
              assertTrue(fixture.requestDescriptionFocus(name))
              fixture.render()
              assertTrue(fixture.pressKey(Key.Enter))
              fixture.render()
              assertTrue(fixture.hasText("Switch project…"))
              assertTrue(fixture.hasText("Re-index project"))
              assertTrue(fixture.hasText("Reconnect"))
              fixture.clickText("Re-index project")
              fixture.render()
              assertEquals(1, reindexes)
              assertEquals(0, imports)
              assertEquals(0, reconnects)
            }
      }
    }
  }

  @Test
  fun resizingWithPaletteOpenDoesNotStealFocusOrActivateItsOpener() {
    var operations = 0
    val initial = shellFocusState(resultProjectFixture())
    var state by mutableStateOf(initial.copy(app = initial.app.copy(workspace = Workspace.Editor)))
    ComposeVisualFixture(1600, 800) {
          FocusTestShell(state, onState = { state = it }, onOperation = { operations++ })
        }
        .use { fixture ->
          fixture.render()
          fixture.clickText("Search files, symbols, commands")
          fixture.render()
          assertTrue(fixture.isFocusedControl("Filter files"))
          fixture.resize(800, 650)
          fixture.render()
          assertTrue(fixture.isFocusedControl("Filter files"))
          assertTrue(fixture.pressKey(Key.Escape))
          fixture.render()
          assertTrue(fixture.isFocusedControl("Search files, symbols, commands"))
          assertEquals(0, operations)
        }
  }

  @Test
  fun successfulOpenMovesLandingFocusToSurvivingToolbarActionWithoutWork() {
    var operations = 0
    var state by mutableStateOf(shellFocusState(resultProjectFixture()).copy(app = DesktopState()))
    ComposeVisualFixture(1280, 800) {
          FocusTestShell(state, onState = { state = it }, onOperation = { operations++ })
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestFocus("Open project…"))
          fixture.render()
          assertEquals(0, operations)
          state =
              state.copy(
                  app = DesktopState(projectState = ProjectWorkspaceState(resultProjectFixture())))
          fixture.render()
          fixture.render()
          assertTrue(
              fixture.isFocusedControl("Search files, symbols, commands"),
              "Toolbar search should own focus after landing is removed")
          assertEquals(0, operations)
        }
  }

  @Test
  fun shellRestoresStatusOpenerOnCloseAndEscapeWithoutDispatchingWork() {
    var operations = 0
    val project = resultProjectFixture()
    var state by mutableStateOf(shellFocusState(project))
    ComposeVisualFixture(1_280, 800) {
          FocusTestShell(state, onState = { state = it }, onOperation = { operations++ })
        }
        .use { fixture ->
          fixture.render()
          repeat(2) { escape ->
            assertTrue(fixture.requestDescriptionFocus("Configured model details"))
            fixture.render()
            assertTrue(fixture.isFocusedControl("Configured model details"))
            assertTrue(fixture.pressKey(Key.Enter))
            fixture.render()
            assertTrue(fixture.hasText("Provider details"))
            assertTrue(fixture.isFocusedControl("Close"))
            if (escape == 0) {
              fixture.clickText("Close")
            } else {
              assertTrue(fixture.pressKey(Key.Escape))
            }
            fixture.render()
            assertTrue(fixture.isFocusedControl("Configured model details"))
          }
          assertEquals(0, operations)
        }
  }

  @Test
  fun railAndFooterModelsShareUnavailableDetailsAndRestoreTheirOwnFocus() {
    var operations = 0
    var state by
        mutableStateOf(
            shellFocusState(resultProjectFixture()).let {
              it.copy(editor = it.editor.copy(analysisInProgress = true, generating = true))
            })
    ComposeVisualFixture(1_280, 800) {
          FocusTestShell(state, onState = { state = it }, onOperation = { operations++ })
        }
        .use { fixture ->
          fixture.render()
          listOf("Models · Configured model details", "Configured model details").forEach { opener
            ->
            repeat(2) { attempt ->
              assertTrue(fixture.requestDescriptionFocus(opener))
              fixture.render()
              assertTrue(fixture.pressKey(if (attempt == 0) Key.Enter else Key.Spacebar))
              fixture.render()
              assertTrue(fixture.hasText("Provider details"))
              assertTrue(fixture.hasText("Models: unavailable"))
              assertTrue(
                  fixture.hasText(
                      "Model counts are unavailable until all configured model scopes have been loaded."))
              assertEquals(Workspace.Summary, state.app.workspace)
              assertTrue(fixture.isFocusedControl("Close"))
              if (attempt == 0) fixture.clickText("Close")
              else assertTrue(fixture.pressKey(Key.Escape))
              fixture.render()
              assertFalse(fixture.hasText("Provider details"))
              assertTrue(fixture.isFocusedControl(opener))
            }
          }
          assertEquals(0, operations)
        }
  }

  @Test
  fun railAndFooterModelsShowTheSameConfiguredDetailsWithoutChangingWorkspace() {
    var operations = 0
    val local = ScopedModel(profile = "local", model = "shared", providerOrigin = "loopback")
    val cloud =
        ScopedModel(
            profile = "cloud",
            model = "cloud-model",
            providerOrigin = "remote",
            remoteProvider = true)
    var state by
        mutableStateOf(
            shellFocusState(resultProjectFixture())
                .copy(statusProviders = DesktopShellStatusProviders(local, cloud, local)))
    ComposeVisualFixture(1_280, 800) {
          FocusTestShell(state, onState = { state = it }, onOperation = { operations++ })
        }
        .use { fixture ->
          fixture.render()
          listOf("Models · Configured model details", "Configured model details").forEach { opener
            ->
            fixture.clickDescription(opener)
            fixture.render()
            assertTrue(fixture.hasText("Provider details"))
            assertTrue(fixture.hasText("Models: 1 local · 1 cloud"))
            assertTrue(
                fixture.hasText(
                    "Distinct configured models by destination; shared models are counted once." +
                        "\nAnalyze: local · shared · local provider · project context stays on this machine" +
                        "\nBugs: cloud · cloud-model · remote provider · confirmation required before sending project context" +
                        "\nFunction edits: local · shared · local provider · project context stays on this machine"))
            assertEquals(Workspace.Summary, state.app.workspace)
            fixture.clickDescription(opener)
            fixture.render()
            assertTrue(fixture.hasText("Provider details"))
            fixture.clickText("Close")
            fixture.render()
            assertFalse(fixture.hasText("Provider details"))
            assertTrue(fixture.isFocusedControl(opener))
          }
          assertEquals(0, operations)
        }
  }

  @Test
  fun railModelsProjectReplacementFallsBackToTheNewProjectOrLanding() {
    var operations = 0
    var state by mutableStateOf(shellFocusState(resultProjectFixture()))
    ComposeVisualFixture(1_280, 800) {
          FocusTestShell(state, onState = { state = it }, onOperation = { operations++ })
        }
        .use { fixture ->
          fixture.render()
          fixture.clickDescription("Models · Configured model details")
          fixture.render()
          assertTrue(fixture.hasText("Provider details"))
          state =
              state.copy(
                  app =
                      state.app.copy(
                          projectState =
                              ProjectWorkspaceState(
                                  resultProjectFixture().copy(projectId = "replacement"))))
          fixture.render()
          assertFalse(fixture.hasText("Provider details"))
          assertTrue(fixture.isFocusedControl("Search files, symbols, commands"))
          assertFalse(fixture.isFocusedControl("Models · Configured model details"))
          fixture.clickDescription("Models · Configured model details")
          fixture.render()
          state = state.copy(app = DesktopState())
          fixture.render()
          assertFalse(fixture.hasText("Provider details"))
          assertTrue(fixture.isFocusedControl("Open project…"))
          assertEquals(0, operations)
        }
  }

  @Test
  fun shellUsesLiveOwnerAfterProjectChangesAndDoesNotRefocusOldOpener() {
    var operations = 0
    var state by mutableStateOf(shellFocusState(resultProjectFixture()))
    ComposeVisualFixture(1_280, 800) {
          FocusTestShell(state, onState = { state = it }, onOperation = { operations++ })
        }
        .use { fixture ->
          fixture.render()
          fixture.clickText("Models: unavailable")
          fixture.render()
          assertTrue(fixture.hasText("Provider details"))
          state =
              state.copy(
                  app =
                      state.app.copy(
                          projectState =
                              ProjectWorkspaceState(
                                  resultProjectFixture().copy(projectId = "new"))))
          fixture.render()
          assertFalse(fixture.hasText("Provider details"))
          assertTrue(fixture.isFocusedControl("Search files, symbols, commands"))
          fixture.clickText("Models: unavailable")
          fixture.render()
          state = state.copy(app = DesktopState())
          fixture.render()
          assertFalse(fixture.hasText("Provider details"))
          assertTrue(fixture.isFocusedControl("Open project…"))
          assertEquals(0, operations)
        }
  }

  @Test
  fun paletteClosesOnProjectSwitchAndRestoresTheNewOwnerRegion() {
    var operations = 0
    var state by mutableStateOf(shellFocusState(resultProjectFixture()))
    ComposeVisualFixture(1_280, 800) {
          FocusTestShell(state, onState = { state = it }, onOperation = { operations++ })
        }
        .use { fixture ->
          fixture.render()
          fixture.clickText("Search files, symbols, commands")
          fixture.render()
          assertTrue(fixture.isFocusedControl("Filter files"))
          state =
              state.copy(
                  app =
                      state.app.copy(
                          projectState =
                              ProjectWorkspaceState(
                                  resultProjectFixture().copy(projectId = "other"))))
          fixture.render()
          assertFalse(fixture.hasText("Filter files"))
          assertTrue(fixture.isFocusedControl("Search files, symbols, commands"))
          assertEquals(0, operations)
        }
  }

  @Test
  fun contextDismissalFallsBackToEditorWhenWorkspaceHidesItsOwner() {
    var operations = 0
    var state by
        mutableStateOf(
            shellFocusState(resultProjectFixture()).let {
              it.copy(
                  app = it.app.copy(workspace = Workspace.Editor),
                  layout = it.layout.copy(lastFocusedRegion = DesktopFocusRegion.RightToolWindow),
                  context = it.context.copy(manifest = ContextManifest()))
            })
    ComposeVisualFixture(1_280, 800) {
          FocusTestShell(state, onState = { state = it }, onOperation = { operations++ })
        }
        .use { fixture ->
          fixture.render()
          repeat(2) { escape ->
            // Focus a live non-canvas control so fallback restoration cannot pass vacuously.
            assertTrue(fixture.requestDescriptionFocus("Search files, symbols, commands"))
            fixture.render()
            assertFalse(fixture.isTaggedNodeFocused("desktop-canvas-focus"))
            state = state.copy(context = state.context.copy(visible = true))
            fixture.render()
            assertTrue(fixture.hasText("Context inspector · read-only"))
            state = state.copy(app = state.app.copy(workspace = Workspace.Summary))
            fixture.render()
            assertFalse(fixture.isTaggedNodeFocused("desktop-canvas-focus"))
            if (escape == 0) fixture.clickText("Close")
            else assertTrue(fixture.pressKey(Key.Escape))
            fixture.render()
            assertFalse(fixture.hasText("Context inspector · read-only"))
            assertTrue(
                fixture.isTaggedNodeFocused("desktop-canvas-focus"),
                "Summary canvas must own focus after dismissal")
            assertEquals(0, operations)
            if (escape == 0) {
              state = state.copy(app = state.app.copy(workspace = Workspace.Editor))
              fixture.render()
            }
          }
        }
  }

  @Test
  fun shellPaletteCloseAndEscapeReturnToToolbarTriggerWithoutActivatingIt() {
    var operations = 0
    var state by mutableStateOf(shellFocusState(resultProjectFixture()))
    ComposeVisualFixture(1_280, 800) {
          FocusTestShell(state, onState = { state = it }, onOperation = { operations++ })
        }
        .use { fixture ->
          fixture.render()
          repeat(2) { escape ->
            fixture.clickText("Search files, symbols, commands")
            fixture.render()
            assertTrue(fixture.isFocusedControl("Filter files"))
            if (escape == 0) fixture.clickText("Close")
            else assertTrue(fixture.pressKey(Key.Escape))
            fixture.render()
            assertTrue(fixture.isFocusedControl("Search files, symbols, commands"))
          }
          assertEquals(0, operations)
        }
  }

  @Test
  fun commandsRailOpensActionsAndRestoresItsOwnFocusOnCloseAndEscape() {
    var operations = 0
    var opens = 0
    var state by
        mutableStateOf(
            shellFocusState(resultProjectFixture()).let {
              it.copy(editor = it.editor.copy(analysisInProgress = true, generating = true))
            })
    ComposeVisualFixture(1_280, 800) {
          FocusTestShell(
              state,
              onState = { state = it },
              onOperation = { operations++ },
              onPaletteOpen = { opens++ })
        }
        .use { fixture ->
          fixture.render()
          repeat(3) { gesture ->
            assertTrue(fixture.requestDescriptionFocus("Commands · Open actions"))
            fixture.render()
            when (gesture) {
              0 -> fixture.clickDescription("Commands · Open actions")
              1 -> assertTrue(fixture.pressKey(Key.Enter))
              else -> assertTrue(fixture.pressKey(Key.Spacebar))
            }
            fixture.render()
            assertEquals(gesture + 1, opens)
            assertEquals(PaletteMode.Actions, state.palette.mode)
            assertTrue(fixture.isFocusedControl("Filter commands"))
            assertEquals(Workspace.Summary, state.app.workspace)
            if (gesture == 1) assertTrue(fixture.pressKey(Key.Escape))
            else fixture.clickText("Close")
            fixture.render()
            assertTrue(fixture.isFocusedControl("Commands · Open actions"))
          }
          assertEquals(0, operations)
        }
  }

  @Test
  fun commandsPaletteProjectReplacementFallsBackWithoutFocusingDetachedOpener() {
    var operations = 0
    var state by mutableStateOf(shellFocusState(resultProjectFixture()))
    ComposeVisualFixture(1_280, 800) {
          FocusTestShell(state, onState = { state = it }, onOperation = { operations++ })
        }
        .use { fixture ->
          fixture.render()
          fixture.clickDescription("Commands · Open actions")
          fixture.render()
          assertTrue(fixture.isFocusedControl("Filter commands"))
          state =
              state.copy(
                  app =
                      state.app.copy(
                          projectState =
                              ProjectWorkspaceState(
                                  resultProjectFixture().copy(projectId = "other"))))
          fixture.render()
          assertFalse(fixture.hasText("Filter commands"))
          assertTrue(fixture.isFocusedControl("Search files, symbols, commands"))
          assertFalse(fixture.isFocusedControl("Commands · Open actions"))
          fixture.clickDescription("Commands · Open actions")
          fixture.render()
          state = state.copy(app = DesktopState())
          fixture.render()
          assertFalse(fixture.hasText("Filter commands"))
          assertTrue(fixture.isFocusedControl("Open project…"))
          assertEquals(0, operations)
        }
  }

  @Test
  fun shellNavigationAndReadOnlyUtilitiesDoNotInvokeWorkflowCallbacks() {
    var state by mutableStateOf(shellFocusState(resultProjectFixture()))
    var operations = 0
    var paletteOpens = 0
    ComposeVisualFixture(1_280, 800) {
          FocusTestShell(
              state,
              onState = { state = it },
              onOperation = { operations++ },
              onPaletteOpen = { paletteOpens++ })
        }
        .use { fixture ->
          fixture.render()
          listOf(
                  Workspace.Summary,
                  Workspace.Analysis,
                  Workspace.Bugs,
                  Workspace.Performance,
                  Workspace.Security,
                  Workspace.Editor)
              .forEach { workspace ->
                val label = leftToolWindowLabel(leftToolWindowForWorkspace(workspace))
                fixture.clickDescription(
                    "$label tool window, ${if (state.app.workspace == workspace) "selected" else "not selected"}")
                fixture.render()
                assertEquals(workspace, state.app.workspace)
                assertEquals(0, operations)
              }
          fixture.clickDescription("Models · Configured model details")
          fixture.render()
          assertTrue(fixture.hasText("Provider details"))
          fixture.clickText("Close")
          fixture.render()
          fixture.clickDescription("Commands · Open actions")
          fixture.render()
          assertEquals(PaletteMode.Actions, state.palette.mode)
          assertEquals(1, paletteOpens)
          fixture.clickText("Close")
          fixture.render()
          assertEquals(Workspace.Editor, state.app.workspace)
          assertEquals(
              0,
              operations,
              "Passive navigation/inspection cannot scan, check, Apply/Undo or write source")
        }
  }

  @Test
  fun summaryPreviewOpensExactLoadedDetailsByPointerEnterAndSpaceWithoutPrivilegedWork() {
    val pages = listOf(resultPageFixture("bugs"), performancePageFixture(), securityPageFixture())
    val run = pages.first().run
    val project = resultProjectFixture()
    val draft =
        DeclarationDraft(
            id = "retained-draft",
            projectId = project.projectId,
            projectRevision = project.projectRevision,
            targetPath = "main.go",
            targetSymbol = "Run",
            declaration = "func Run() {}")
    val initial =
        shellFocusState(project).let { shell ->
          shell.copy(
              app =
                  shell.app.copy(
                      selection = FileSelectionState(selectedFile = analysisFileFixture()),
                      review = DraftReviewState(draft = draft),
                      analysisRun =
                          ProjectAnalysisRunState(
                              run = run,
                              sections =
                                  pages.associate {
                                    AnalysisResultKey(it.category) to it.section
                                  })))
        }
    val original = initial.app
    val rows = summaryFindingPreview(original).rows
    assertTrue(rows.any { it.target.category == AnalysisResultType.Bugs })
    assertTrue(rows.any { it.target.category == AnalysisResultType.Performance })
    assertTrue(rows.any { it.target.category == AnalysisResultType.Security })
    rows.forEachIndexed { index, row ->
      var state by mutableStateOf(initial)
      var privileged = 0
      ComposeVisualFixture(1280, 800) {
            FocusTestShell(state, onState = { state = it }, onOperation = { privileged++ })
          }
          .use { fixture ->
            fixture.render()
            val label =
                "Inspect ${row.title} in ${row.target.category.workspace.name} results at ${row.location}"
            fixture.revealText("Selected findings", "summary-scroll")
            fixture.render()
            when (index % 3) {
              0 -> fixture.clickDescription(label)
              1 -> {
                assertTrue(fixture.requestDescriptionFocus(label))
                assertTrue(fixture.pressKey(Key.Enter))
              }
              else -> {
                assertTrue(fixture.requestDescriptionFocus(label))
                assertTrue(fixture.pressKey(Key.Spacebar))
              }
            }
            fixture.render()
            assertEquals(row.target.category.workspace, state.app.workspace)
            assertTrue(fixture.hasText(row.title), "Expected exact destination detail for $label")
            assertEquals(original.selection, state.app.selection)
            assertEquals(original.review, state.app.review)
            assertEquals(0, privileged)
          }
    }
  }

  @Test
  fun summaryEvidenceFocusAndRoutesRemainInOrderAtLargeText() {
    val page = resultPageFixture("bugs")
    val project = requireNotNull(page.project)
    val state =
        DesktopState(
            projectState = ProjectWorkspaceState(project),
            analysisRun =
                ProjectAnalysisRunState(
                    run = page.run, sections = mapOf(AnalysisResultKey("bugs") to page.section)))
    val row = summaryFindingPreview(state).rows.single()
    val destinations = mutableListOf<Workspace>()
    var selected: SummaryFindingTarget? = null
    for (density in listOf(1f, 2f)) {
      ComposeVisualFixture((800 * density).toInt(), (650 * density).toInt(), 1.5f, density) {
            ProjectSummaryPane(
                null,
                project,
                destinations::add,
                findingState = state,
                onFindingSelected = { selected = it })
          }
          .use { fixture ->
            fixture.render()
            fixture.revealText("All files", "summary-scroll")
            assertTrue(fixture.requestFocus("All files"))
            fixture.render("f10-keyboard-ledger-${density}x")
            assertTrue(fixture.isFocusedControl("All files"))
            fixture.assertColorVisible(FocusAccent)
            fixture.revealText("Selected findings", "summary-scroll")
            val inspect = "Inspect ${row.title} in Bugs results at ${row.location}"
            assertTrue(fixture.requestDescriptionFocus(inspect))
            fixture.render("f10-keyboard-finding-${density}x")
            assertTrue(fixture.isFocusedControl(inspect))
            fixture.assertColorVisible(FocusAccent)
            repeat(3) {
              if (!fixture.isFocusedControl("All Bugs results")) {
                assertTrue(fixture.pressKey(Key.Tab))
                fixture.render()
              }
            }
            assertTrue(
                fixture.isFocusedControl("All Bugs results"),
                "Finding must lead to category routes")
            assertTrue(fixture.pressKey(Key.Tab))
            fixture.render()
            assertTrue(fixture.isFocusedControl("All Performance results"))
            assertTrue(fixture.pressKey(Key.Tab))
            fixture.render()
            assertTrue(fixture.isFocusedControl("All Security results"))
            assertTrue(fixture.pressKey(Key.Tab))
            fixture.render()
            assertTrue(fixture.isFocusedControl("Open Editor"))
            assertEquals(null, selected)
            assertTrue(fixture.pressKey(Key.Enter))
            assertEquals(Workspace.Editor, destinations.last())
          }
    }
  }

  @Test
  fun summaryUnavailableTargetKeepsExplanationAndRecoveryKeyboardReachable() {
    val page = resultPageFixture("bugs")
    val row =
        summaryFindingPreview(
                DesktopState(
                    projectState = ProjectWorkspaceState(page.project),
                    analysisRun =
                        ProjectAnalysisRunState(
                            run = page.run,
                            sections = mapOf(AnalysisResultKey("bugs") to page.section))))
            .rows
            .single()
    val browser = newResultBrowserState(page)
    browser.target(
        ExplicitResultTarget.Unavailable(
            row.target, "This finding is no longer in the loaded results."))
    for ((width, height, scale) in listOf(Triple(800, 650, 1.5f), Triple(1440, 900, 1f))) {
      var navigations = 0
      ComposeVisualFixture(width, height, scale) {
            AnalysisResultsPane(
                page,
                listOf(semanticResultRow(page.semantic.single())),
                browser,
                openAnalysis = { navigations++ }) {
                  androidx.compose.material.Text("Unrelated detail")
                }
          }
          .use { fixture ->
            fixture.render("f10-target-unavailable-$width-$scale")
            fixture.revealText(
                "Finding unavailable · This finding is no longer in the loaded results.",
                "result-target-unavailable")
            assertTrue(
                fixture.hasText(
                    "Finding unavailable · This finding is no longer in the loaded results."))
            assertFalse(fixture.hasText("Unrelated detail"))
            assertTrue(fixture.requestFocus("View current Bugs results"))
            fixture.render("f10-target-unavailable-focused-$width-$scale")
            fixture.assertColorVisible(FocusAccent)
            assertTrue(fixture.pressKey(Key.Enter))
            fixture.render()
            assertEquals(null, browser.explicitTarget)
            assertEquals(0, navigations)
          }
      browser.target(
          ExplicitResultTarget.Unavailable(
              row.target, "This finding is no longer in the loaded results."))
    }
  }

  @Test
  fun summaryEditorEntryNavigatesByPointerAndKeyboardWithoutWorkflowActions() {
    val project = resultProjectFixture()
    val draft =
        DeclarationDraft(
            id = "retained-draft",
            projectId = project.projectId,
            projectRevision = project.projectRevision,
            targetPath = "main.go",
            targetSymbol = "Run",
            declaration = "func Run() {}")
    var state by
        mutableStateOf(
            shellFocusState(project).let {
              it.copy(
                  app =
                      it.app.copy(
                          review =
                              DraftReviewState(
                                  draft = draft,
                                  editor =
                                      EditableDraftState(
                                          draft, declaration = "func Run() { /* local edit */ }"))))
            })
    val initial = state
    var operations = 0
    ComposeVisualFixture(1_280, 800) {
          FocusTestShell(state, onState = { state = it }, onOperation = { operations++ })
        }
        .use { fixture ->
          fixture.render()
          repeat(3) { gesture ->
            fixture.revealText("Open Editor", "summary-scroll")
            fixture.render()
            assertEquals(0, operations)
            when (gesture) {
              0 -> fixture.clickText("Open Editor")
              1 -> {
                assertTrue(fixture.requestFocus("Open Editor"))
                fixture.render()
                assertTrue(fixture.pressKey(Key.Enter))
              }
              else -> {
                assertTrue(fixture.requestFocus("Open Editor"))
                fixture.render()
                assertTrue(fixture.pressKey(Key.Spacebar))
              }
            }
            fixture.render()
            assertEquals(Workspace.Editor, state.app.workspace)
            assertEquals(initial.app.projectState, state.app.projectState)
            assertEquals(initial.app.review, state.app.review)
            assertEquals(initial.editor, state.editor)
            assertEquals(0, operations, "Editor entry cannot generate, validate, check or Apply")
            if (gesture < 2) {
              fixture.clickDescription("Summary tool window, not selected")
              fixture.render()
              assertEquals(Workspace.Summary, state.app.workspace)
            }
          }
        }
  }

  @Test
  fun terminalRailOpensExistingDockWithoutStartingOnRenderFocusOrWorkspaceSwitch() {
    val directory = Files.createTempDirectory("mini-orca-rail-terminal-")
    val starts = AtomicInteger()
    val terminal = DesktopTerminalWorkspace { path ->
      DesktopTerminalSession(
          path,
          shell = "/bin/sh",
          environment = emptyMap(),
          factory =
              TerminalProcessFactory {
                starts.incrementAndGet()
                throw IllegalStateException("Synthetic launch failure")
              })
    }
    var state by
        mutableStateOf(shellFocusState(resultProjectFixture().copy(path = directory.toString())))
    var selections = 0
    try {
      ComposeVisualFixture(1_280, 800) {
            FocusTestShell(
                state,
                onState = { state = it },
                onOperation = { selections++ },
                terminal = terminal)
          }
          .use { fixture ->
            fixture.render()
            assertEquals(0, starts.get())
            assertTrue(
                fixture.requestDescriptionFocus(
                    "Terminal · Open or focus; may start a local shell"))
            fixture.render()
            assertEquals(0, starts.get())
            fixture.clickDescription("Analysis tool window, not selected")
            fixture.render()
            assertEquals(0, starts.get())
            assertEquals(Workspace.Analysis, state.app.workspace)
            assertTrue(state.layout.bottomCollapsed)
            SwingUtilities.invokeAndWait {
              fixture.clickDescription("Terminal · Open or focus; may start a local shell")
            }
            fixture.render()
            assertFalse(state.layout.bottomCollapsed)
            assertEquals(Workspace.Analysis, state.app.workspace)
            assertEquals(1, terminal.state.value.tabs.size)
            val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
            while (starts.get() == 0 && System.nanoTime() < deadline) Thread.sleep(5)
            assertEquals(1, starts.get())
            val tab = terminal.state.value.activeTabId
            SwingUtilities.invokeAndWait {
              fixture.clickDescription("Terminal · Open or focus; may start a local shell")
            }
            fixture.render()
            assertEquals(tab, terminal.state.value.activeTabId)
            assertEquals(1, terminal.state.value.tabs.size)
            assertEquals(1, starts.get())
            assertEquals(Workspace.Analysis, state.app.workspace)
            assertEquals(0, selections)
          }
    } finally {
      SwingUtilities.invokeAndWait { terminal.close() }
      Files.deleteIfExists(directory)
    }
  }

  private fun shellFocusState(project: ProjectAnalysis): DesktopShellState =
      DesktopShellState(
          app = DesktopState(projectState = ProjectWorkspaceState(project)),
          layout = DesktopLayoutState(),
          editor =
              DesktopShellEditorState(
                  EditorProgressUiState(EditorProgress.Inspect, ""),
                  EditorContextualActions(false, false, false, false, false),
                  analysisInProgress = false,
                  generating = false),
          context =
              DesktopShellContextState(
                  false, null, ScopedModel(), false, ScopedModel(), false, false),
          palette = DesktopShellPaletteState(PaletteMode.Files, "", false),
          statusProviders =
              DesktopShellStatusProviders(ScopedModel(), ScopedModel(), ScopedModel()))

  @Composable
  private fun FocusTestShell(
      state: DesktopShellState,
      onState: (DesktopShellState) -> Unit,
      onOperation: () -> Unit,
      terminal: DesktopTerminalWorkspace? = null,
      onPaletteOpen: () -> Unit = {},
  ) {
    DesktopShell(
        state = state,
        layoutActions = DesktopShellLayoutActions({ onState(state.copy(layout = it)) }, {}),
        projectActions = DesktopShellProjectActions(onOperation, onOperation, onOperation),
        editorActions =
            DesktopShellEditorActions(
                selectWorkspace = { onState(state.copy(app = state.app.copy(workspace = it))) },
                selectEditorSurface = {},
                focusChat = {},
                focusDraft = {},
                cancelAnalysis = onOperation,
                sourceLineSelected = {},
                validateDraft = onOperation,
                runDraftChecks = onOperation,
                generate = onOperation,
                cancelGeneration = onOperation,
                dismissContext = {
                  onState(state.copy(context = state.context.copy(visible = false)))
                },
                createDeclaration = onOperation),
        analysisActions =
            DesktopShellAnalysisActions(
                refreshStatus = onOperation,
                refreshAnalysisSelection = onOperation,
                saveAnalysisSelection = {},
                retryResults = { _, _ -> onOperation() },
                startAnalysis = { _, _ -> onOperation() },
                pauseAnalysis = onOperation,
                resumeAnalysis = onOperation,
                cancelAnalysis = onOperation,
                startScan = onOperation,
                cancelScan = onOperation,
                refreshScanStatus = onOperation,
                preparePerformanceFinding = { _, _ -> onOperation() },
                openPerformanceSource = { _, _ -> onOperation() },
                loadGoBenchmarks = onOperation,
                selectGoBenchmark = {},
                compareSelectedGoBenchmark = onOperation,
                prepareSecurityFinding = { _, _ -> onOperation() },
                openSecuritySource = { _, _ -> onOperation() }),
        findingActions = FindingActions({ onOperation() }, { _, _ -> onOperation() }, {}),
        paletteActions =
            DesktopShellPaletteActions(
                updateQuery = { onState(state.copy(palette = state.palette.copy(query = it))) },
                dismiss = { onState(state.copy(palette = state.palette.copy(visible = false))) },
                open = {
                  onPaletteOpen()
                  onState(state.copy(palette = state.palette.copy(visible = true, mode = it)))
                },
                switchMode = { onState(state.copy(palette = state.palette.copy(mode = it))) },
                selectFile = {},
                selectSymbol = {},
                selectAction = {}),
        panes =
            DesktopShellPanes(
                explorer = { _, _ -> },
                rightToolWindows = { _, _ -> },
                rightToolWindowBadges = emptyMap(),
                terminalContent = {},
                terminalState = terminal?.state?.value ?: TerminalWorkspaceState(),
                terminalTabActions = TerminalTabActions({}, {}, {})),
        terminal = terminal)
  }

  @Test
  fun paletteShortcutsKeepFilesAndSymbolsDirectWhileCommandKFocusesChat() {
    assertEquals(DesktopShortcut.OpenFile, desktopShortcut("P", primaryModifier = true))
    assertEquals(
        DesktopShortcut.OpenSymbol, desktopShortcut("O", primaryModifier = true, shift = true))
    assertEquals(DesktopShortcut.FocusChat, desktopShortcut("K", primaryModifier = true))
  }

  @Test
  fun openingAndReconnectRecoveryRequireExplicitKeyboardActivation() {
    var opens = 0
    var reconnects = 0
    ComposeVisualFixture(800, 650, 1.5f) {
          ProjectLanding(
              DesktopState(
                  projectState = ProjectWorkspaceState(openingError = "Local read failed"),
                  connection = ConnectionState(label = "Disconnected")),
              DesktopShellProjectActions({ opens++ }, {}, { reconnects++ }),
              androidx.compose.ui.focus.FocusRequester())
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestFocus("Reconnect daemon"))
          fixture.render()
          assertEquals(0, opens + reconnects)
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(1, reconnects)
          assertTrue(fixture.requestFocus("Open project…"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          assertEquals(1, opens)
        }
  }

  @Test
  fun savedResultRecoveryRequiresKeyboardActivationAndDoesNotStartAnalysis() {
    AnalysisResultType.entries.forEach { type ->
      val page =
          resultPageFixture(type.category).let {
            it.copy(section = it.section.copy(error = "saved read failed"))
          }
      var retries = 0
      var navigations = 0
      val browser = newResultBrowserState(page)
      browser.query = "no matching result"
      ComposeVisualFixture(800, 650, 1.5f) {
            AnalysisResultsPane(
                page,
                listOf(
                    ResultRowPresentation(
                        "retained", "Retained result", "main.go:1", "Evidence", "high", "", "")),
                browser,
                openAnalysis = { navigations++ },
                retryResults = { retries++ }) {
                  androidx.compose.material.Text("Retained detail")
                }
          }
          .use { fixture ->
            fixture.render()
            fixture.revealText("Retry loading results", "result-read-feedback")
            assertTrue(fixture.requestFocus("Retry loading results"))
            fixture.render()
            assertEquals(0, retries + navigations)
            assertTrue(fixture.pressKey(Key.Enter))
            assertEquals(1, retries)
            fixture.revealText("Clear filters", "result-overview")
            assertTrue(fixture.requestDescriptionFocus("Clear filters"))
            assertTrue(fixture.pressKey(Key.Spacebar))
            fixture.render()
            assertEquals("", browser.query)
            assertEquals(1, retries)
            assertEquals(0, navigations)
          }
    }
  }

  @Test
  fun resultNavigationAndFixPreparationStaySeparateFromKeyboardInspection() {
    var destination: Workspace? = null
    var preparations = 0
    val inspected = mutableListOf<PerformanceResult>()
    val page = performancePageFixture()
    ComposeVisualFixture(1_000, 760, 1.25f) {
          PerformanceWorkspacePane(
              PerformanceWorkspacePaneState(page, resultIndexFixture()),
              PerformanceWorkspaceActions(
                  prepareOptimization = { preparations++ },
                  openSource = { inspected += it },
                  openAnalysis = { destination = Workspace.Analysis },
                  semanticActions = FindingActions({}, { _, _ -> }, {})))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestFocus("View analysis"))
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(Workspace.Analysis, destination)
          assertEquals(0, preparations)

          assertTrue(fixture.requestDescriptionFocus("Inspect Avoid repeated allocation"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertTrue(fixture.hasText("Prepare fix"))
          assertEquals(0, preparations)

          assertTrue(fixture.requestFocus("Open source"))
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(performanceResults(page).single().finding, inspected.single().finding)
          assertEquals(0, preparations)

          assertTrue(fixture.requestFocus("Prepare fix"))
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(1, preparations)
        }
  }

  @Test
  fun semanticPerformanceSourceIsKeyboardReachableWithoutPreparingAndUnavailableWithoutIndex() {
    val original = performancePageFixture()
    val finding =
        UnifiedFinding(
            id = "semantic-source",
            category = "performance",
            projectId = "project",
            projectRevision = "revision",
            title = "Inspect repeated work",
            location = FindingLocation("main.go", startLine = 7, symbol = "Run"))
    val page =
        original.copy(
            section =
                original.section.copy(
                    results = original.results!!.copy(semantic = listOf(finding))))
    var index by mutableStateOf<ProjectIndex?>(resultIndexFixture())
    val browser = newResultBrowserState(page)
    browser.choose(semanticResultRow(finding).key)
    val opened = mutableListOf<UnifiedFinding>()
    var prepared = 0
    ComposeVisualFixture(800, 650, 1.5f) {
          PerformanceWorkspacePane(
              PerformanceWorkspacePaneState(page, index, browser = browser),
              PerformanceWorkspaceActions(
                  prepareOptimization = { prepared++ },
                  openAnalysis = {},
                  semanticActions = FindingActions({ prepared++ }, { _, _ -> }, { opened += it }),
                  openSource = {}))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("main.go:7 · Run"))
          assertTrue(fixture.requestFocus("Open source"))
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(listOf(finding), opened)
          assertEquals(0, prepared)

          index = resultIndexFixture().copy(files = emptyList())
          fixture.render()
          assertTrue(fixture.isDisabled("Open source"))
          assertTrue(fixture.hasText("Open source requires a path in the current project index."))
          assertEquals(listOf(finding), opened)
        }
  }

  @Test
  fun analysisKeyboardControlsNavigateOrUpdateOnlyLocalPresentationState() {
    val navigations = mutableListOf<Workspace>()
    var starts = 0
    var pauses = 0
    var resumes = 0
    var cancels = 0
    var refreshes = 0
    var saves = 0
    val run =
        analysisRunFixture()
            .copy(
                status = "running",
                plan =
                    analysisPreviewFixture()
                        .copy(
                            files =
                                listOf("helper.go" to "helper", "main.go" to "main").map {
                                    (path, hash) ->
                                  AnalysisPlannedFile(
                                      path,
                                      hash,
                                      "Go",
                                      20,
                                      listOf(
                                          AnalysisStagePlan(
                                              "semantic", true, false, maxModelRequests = 0)))
                                }),
                files =
                    listOf(
                        AnalysisRunFile(
                            "helper.go",
                            "helper",
                            "Go",
                            listOf(AnalysisStageProgress("semantic", "running", 1, false))),
                        AnalysisRunFile(
                            "main.go",
                            "main",
                            "Go",
                            listOf(AnalysisStageProgress("semantic", "running", 1, false))),
                    ),
                sections = analysisRunFixture().sections.map { it.copy(status = "running") },
            )
    val actions =
        AnalysisWorkspaceActions(
            start = { _, _ -> starts++ },
            pause = { pauses++ },
            resume = { resumes++ },
            cancel = { cancels++ },
            openResults = { navigations += it },
            refreshSelection = { refreshes++ },
            saveSelection = { saves++ },
            refreshStatus = { error("Unexpected status refresh") })
    ComposeVisualFixture(1_600, 1_000) {
          AnalysisWorkspacePane(
              AnalysisWorkspacePaneState(
                  resultProjectFixture(),
                  ProjectAnalysisRunState(
                      run = run, fileSelection = AnalysisSelectionState(selectionFixture()))),
              actions,
          )
        }
        .use { fixture ->
          fixture.render()
          AnalysisResultType.entries.forEach { type ->
            assertTrue(fixture.requestDescriptionFocus("View ${type.workspace.name} results"))
            assertTrue(fixture.pressKey(Key.Enter))
            assertTrue(fixture.pressKey(Key.Spacebar))
            fixture.render()
          }
          assertEquals(
              AnalysisResultType.entries.flatMap { listOf(it.workspace, it.workspace) },
              navigations)

          assertTrue(fixture.requestDescriptionFocus("Show active files"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertTrue(fixture.hasDescription("Hide active files"))

          assertTrue(fixture.requestDescriptionFocus("Collapse Files"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertEquals("Collapsed", fixture.stateDescription("Files"))
          assertTrue(fixture.requestDescriptionFocus("Expand Files"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertEquals("Expanded", fixture.stateDescription("Files"))

          assertTrue(fixture.requestDescriptionFocus("Filter files"))
          fixture.setFocusedText("helper.go")
          fixture.render()
          assertTrue(fixture.hasText("helper.go"))
          assertTrue(fixture.requestDescriptionFocus("Needs attention"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertTrue(fixture.isDescriptionSelected("Needs attention"))

          assertEquals(0, starts)
          assertEquals(0, pauses)
          assertEquals(0, resumes)
          assertEquals(0, cancels)
          assertEquals(0, refreshes)
          assertEquals(0, saves)
        }
  }

  @Test
  fun analysisFilesControlsSurviveOuterScrollNavigationAndSelectionUpdates() {
    val project = mutableStateOf(resultProjectFixture())
    val selection = mutableStateOf(AnalysisSelectionState(loading = true))
    val workspace = mutableStateOf(Workspace.Analysis)
    val filesItemComposed = mutableStateOf(true)
    var requests = 0
    val actions =
        AnalysisWorkspaceActions(
            { _, _ -> requests++ },
            { requests++ },
            { requests++ },
            { requests++ },
            { requests++ },
            { requests++ },
            { requests++ },
            refreshStatus = { error("Unexpected status refresh") })
    ComposeVisualFixture(1_024, 768) {
          // Like DesktopShell, the owner stays above the workspace switch and the page's lazy
          // items.
          val view =
              remember(project.value.projectId, project.value.projectRevision) {
                AnalysisFilesViewState()
              }
          if (workspace.value == Workspace.Analysis && filesItemComposed.value)
              AnalysisWorkspacePane(
                  AnalysisWorkspacePaneState(
                      project.value, ProjectAnalysisRunState(fileSelection = selection.value)),
                  actions,
                  view)
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasDescription("Collapse Files"))
          selection.value = AnalysisSelectionState(selectionFixture())
          fixture.render()
          fixture.clickDescription("Needs attention")
          fixture.setText("main")
          fixture.render()
          assertTrue(fixture.hasText("1 of 3 files match"))
          fixture.clickDescription("Collapse Files")
          fixture.render()
          fixture.clickDescription("Expand Files")
          fixture.render()
          assertTrue(fixture.isDescriptionSelected("Needs attention"))
          assertTrue(fixture.hasText("1 of 3 files match"))
          fixture.scrollBy(100_000f, "analysis-page")
          fixture.render()
          fixture.scrollBy(-100_000f, "analysis-page")
          fixture.render()
          // Force the same disposal/re-entry that the outer lazy item can perform.
          filesItemComposed.value = false
          fixture.render()
          filesItemComposed.value = true
          fixture.render()
          assertTrue(fixture.hasText("1 of 3 files match"))
          selection.value = AnalysisSelectionState(selectionFixture().copy(editable = false))
          fixture.render()
          assertTrue(fixture.hasText("1 of 3 files match"))
          workspace.value = Workspace.Bugs
          fixture.render()
          workspace.value = Workspace.Analysis
          fixture.render()
          assertTrue(fixture.isDescriptionSelected("Needs attention"))
          assertTrue(fixture.hasText("1 of 3 files match"))
          project.value = resultProjectFixture().copy(projectId = "other")
          selection.value = AnalysisSelectionState(loading = true)
          fixture.render()
          assertTrue(fixture.hasDescription("Collapse Files"))
          assertTrue(fixture.isDescriptionSelected("All"))
          selection.value = AnalysisSelectionState(selectionFixture().copy(projectId = "other"))
          fixture.render()
          assertTrue(fixture.hasText("3 of 3 files match"))
          assertEquals(0, requests)
        }
  }

  @Test
  fun filesKeyboardTraversalAndSingleActivationWorkInBothRowArrangements() {
    val selection = selectionFixture()
    for (width in listOf(1440, 800)) {
      var saves = 0
      var reads = 0
      var starts = 0
      val view = AnalysisFilesViewState()
      ComposeVisualFixture(width, 650, if (width == 800) 1.5f else 1f) {
            AnalysisFileSelector(
                ProjectAnalysisRunState(fileSelection = AnalysisSelectionState(selection)),
                AnalysisWorkspaceActions(
                    { _, _ -> starts++ },
                    {},
                    {},
                    {},
                    {},
                    { reads++ },
                    { saves++ },
                    refreshStatus = { error("Unexpected status refresh") }),
                320.dp,
                view = view)
          }
          .use { fixture ->
            fixture.render()
            assertTrue(fixture.requestDescriptionFocus("Filter files"))
            fixture.setFocusedText("main")
            fixture.render()
            assertTrue(fixture.isFocusedControl("Filter files"), "Typing must retain search focus")
            assertEquals("main", view.query)
            // The offscreen scene has no native IME; an unconsumed Space reaches text input.
            assertFalse(fixture.pressKey(Key.Spacebar))
            fixture.setFocusedText("main ")
            fixture.render()
            assertEquals("main ", view.query, "$width: Space must remain valid search text")
            assertTrue(fixture.isFocusedControl("Filter files"))
            fixture.resize(if (width == 1440) 800 else 1440, 650)
            fixture.render()
            assertTrue(fixture.isFocusedControl("Filter files"))
            fixture.resize(width, 650)
            fixture.render()
            assertTrue(fixture.isFocusedControl("Filter files"))
            assertTrue(fixture.pressKey(Key.Tab, shift = true))
            fixture.render()
            assertTrue(fixture.isFocusedControl("Refresh files"))
            assertTrue(fixture.pressKey(Key.Enter))
            fixture.render()
            assertEquals(1, reads)
            assertTrue(fixture.pressKey(Key.Tab))
            fixture.render()
            assertTrue(fixture.isFocusedControl("Filter files"))
            fixture.setFocusedText("main")
            fixture.render()
            assertTrue(fixture.pressKey(Key.Tab))
            fixture.render()
            assertTrue(fixture.isFocusedControl("All"))
            for (name in listOf("Needs attention", "Up to date", "Excluded")) {
              assertTrue(fixture.pressKey(Key.Tab))
              fixture.render()
              assertTrue(fixture.isFocusedControl(name), "$width: $name must follow search")
            }
            assertTrue(fixture.pressKey(Key.Enter))
            fixture.render()
            assertEquals(AnalysisFileFilter.Excluded, view.filter)
            assertEquals(1, reads)
            assertEquals(0, saves + starts)
            assertTrue(fixture.pressKey(Key.Tab))
            fixture.render()
            assertTrue(
                fixture.isFocusedControl("Select all"), "$width: bulk actions follow filters")
            assertTrue(fixture.pressKey(Key.Enter))
            fixture.render()
            assertEquals(1, saves)
            assertTrue(fixture.pressKey(Key.Tab))
            fixture.render()
            assertTrue(fixture.isFocusedControl("Exclude all"))
            assertTrue(fixture.pressKey(Key.Spacebar))
            fixture.render()
            assertEquals(2, saves)
            assertTrue(fixture.requestDescriptionFocus("All"))
            assertTrue(fixture.pressKey(Key.Spacebar))
            fixture.render()
            assertTrue(fixture.requestFocus("Exclude all"))
            assertTrue(fixture.pressKey(Key.Tab))
            fixture.render()
            assertTrue(fixture.isFocusedControl("Analyze main.go"))
            assertTrue(fixture.pressKey(Key.Spacebar))
            assertEquals(3, saves)
            var reachedDetails = false
            repeat(4) {
              if (!reachedDetails) {
                assertTrue(fixture.pressKey(Key.Tab))
                fixture.render()
                reachedDetails = fixture.isFocusedControl("Analysis details for main.go")
              }
            }
            assertTrue(reachedDetails, "Details must be in keyboard traversal at $width")
            assertTrue(fixture.pressKey(Key.Enter))
            fixture.render()
            assertEquals("Expanded", fixture.descriptionState("Analysis details for main.go"))
            assertTrue(fixture.pressKey(Key.Spacebar))
            fixture.render()
            assertEquals("Collapsed", fixture.descriptionState("Analysis details for main.go"))
            assertEquals(3, saves)
            assertEquals(1, reads)
            view.query = ".env"
            fixture.render()
            assertTrue(fixture.isDescriptionDisabled("Analyze .env"))
            assertFalse(fixture.requestDescriptionFocus("Analyze .env"))
            fixture.tryClick("Analyze .env")
            fixture.render()
            assertEquals(3, saves, "Policy-disabled file cannot save")
            assertEquals(0, starts)
          }
    }
  }

  @Test
  fun filesDoNotReclaimFocusAfterLeavingThePanel() {
    val view = AnalysisFilesViewState()
    var calls = 0
    ComposeVisualFixture(1440, 650) {
          androidx.compose.foundation.layout.Column {
            AnalysisFileSelector(
                ProjectAnalysisRunState(fileSelection = AnalysisSelectionState(selectionFixture())),
                AnalysisWorkspaceActions(
                    { _, _ -> calls++ },
                    {},
                    {},
                    {},
                    {},
                    { calls++ },
                    { calls++ },
                    refreshStatus = { error("Unexpected status refresh") }),
                320.dp,
                view = view)
            MiniOrcaButton(onClick = { calls++ }) {
              androidx.compose.material.Text("Outside Files")
            }
          }
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestDescriptionFocus("Analysis details for main.go"))
          assertTrue(fixture.requestFocus("Outside Files"))
          fixture.resize(800, 650)
          fixture.render()
          assertTrue(fixture.isFocusedControl("Outside Files"))
          assertEquals(0, calls)
        }
  }

  @Test
  fun filesFocusRecoversFromFilteringAndRemovalAndSurvivesReflowWithoutActions() {
    val view = AnalysisFilesViewState()
    var selection by mutableStateOf(selectionFixture())
    var reads = 0
    var saves = 0
    var starts = 0
    ComposeVisualFixture(1440, 650) {
          AnalysisFileSelector(
              ProjectAnalysisRunState(fileSelection = AnalysisSelectionState(selection)),
              AnalysisWorkspaceActions(
                  { _, _ -> starts++ },
                  {},
                  {},
                  {},
                  {},
                  { reads++ },
                  { saves++ },
                  refreshStatus = { error("Unexpected status refresh") }),
              320.dp,
              view = view)
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestDescriptionFocus("Analysis details for main.go"))
          fixture.resize(800, 650)
          fixture.render()
          assertTrue(
              fixture.isFocusedControl("Analysis details for main.go"),
              "compact: ${view.focusedRow}")
          fixture.resize(1440, 650)
          fixture.render()
          assertTrue(fixture.isFocusedControl("Analysis details for main.go"))
          assertTrue(fixture.requestDescriptionFocus("Needs attention"))
          fixture.resize(800, 650)
          fixture.render()
          assertTrue(fixture.isFocusedControl("Needs attention"))
          fixture.resize(1440, 650)
          fixture.render()
          assertTrue(fixture.requestDescriptionFocus("Analysis details for main.go"))
          assertTrue(fixture.requestDescriptionFocus("Filter files"))
          fixture.render()
          assertEquals(null, view.focusedRow, "Search must release row focus ownership")
          fixture.resize(800, 650)
          fixture.render()
          assertTrue(fixture.isFocusedControl("Filter files"))
          fixture.resize(1440, 650)
          fixture.render()
          assertTrue(fixture.isFocusedControl("Filter files"))
          assertTrue(fixture.requestDescriptionFocus("Analysis details for main.go"))
          view.query = "helper"
          fixture.render()
          assertTrue(fixture.isFocusedControl("Filter files"))
          assertEquals(0, reads + saves + starts)
          view.query = ""
          fixture.render()
          assertTrue(fixture.requestDescriptionFocus("Analyze main.go"))
          selection = selection.copy(files = selection.files.filterNot { it.path == "main.go" })
          fixture.render()
          assertTrue(fixture.isFocusedControl("Filter files"))
          assertEquals(0, reads + saves + starts)
        }
  }

  @Test
  fun analysisKeyboardRunSelectionAndDetailsControlsRemainIndependent() {
    var pauses = 0
    var resumes = 0
    var cancels = 0
    val runActions =
        AnalysisWorkspaceActions(
            start = { _, _ -> },
            pause = { pauses++ },
            resume = { resumes++ },
            cancel = { cancels++ },
            openResults = {},
            refreshStatus = { error("Unexpected status refresh") })
    ComposeVisualFixture(1_600, 1_000) {
          AnalysisWorkspacePane(
              AnalysisWorkspacePaneState(
                  resultProjectFixture(),
                  ProjectAnalysisRunState(run = analysisRunFixture().copy(status = "running"))),
              runActions,
          )
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestFocus("Pause"))
          assertTrue(fixture.pressKey(Key.Enter))
          assertTrue(fixture.requestFocus("Cancel"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          assertEquals(1, pauses)
          assertEquals(1, cancels)
        }

    ComposeVisualFixture(1_600, 1_000) {
          AnalysisWorkspacePane(
              AnalysisWorkspacePaneState(
                  resultProjectFixture(),
                  ProjectAnalysisRunState(run = analysisRunFixture().copy(status = "paused"))),
              runActions,
          )
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestFocus("Resume → fresh preview"))
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(1, resumes)
        }

    val selectionWithDetails =
        selectionFixture()
            .copy(
                files =
                    selectionFixture().files.map { file ->
                      if (file.path == "main.go")
                          file.copy(
                              stages =
                                  listOf(
                                      AnalysisFileStageStatus(
                                          "semantic", "missing", "Semantic results are missing."),
                                      AnalysisFileStageStatus(
                                          "performance",
                                          "fresh",
                                          "Performance results are current."),
                                  ))
                      else file
                    })
    var saves = 0
    ComposeVisualFixture(1_440, 900) {
          AnalysisFileSelector(
              ProjectAnalysisRunState(fileSelection = AnalysisSelectionState(selectionWithDetails)),
              AnalysisWorkspaceActions(
                  start = { _, _ -> },
                  pause = {},
                  resume = {},
                  cancel = {},
                  openResults = {},
                  saveSelection = { saves++ },
                  refreshStatus = { error("Unexpected status refresh") }),
          )
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestDescriptionFocus("Analyze main.go"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          assertEquals(1, saves)
          assertTrue(fixture.requestDescriptionFocus("Analysis details for main.go"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertEquals(
              "Expanded", fixture.descriptionStateDescription("Analysis details for main.go"))
          assertEquals(1, saves, "Opening details must not update selection")
        }

    listOf(
            ProjectAnalysisRunState(
                run = analysisRunFixture().copy(status = "running"),
                fileSelection = AnalysisSelectionState(selectionWithDetails)),
            ProjectAnalysisRunState(
                run = analysisRunFixture().copy(status = "paused"),
                fileSelection = AnalysisSelectionState(selectionWithDetails)),
            ProjectAnalysisRunState(
                run = analysisRunFixture().copy(status = "interrupted"),
                fileSelection = AnalysisSelectionState(selectionWithDetails)),
            ProjectAnalysisRunState(
                fileSelection = AnalysisSelectionState(selectionWithDetails, saving = true)),
            ProjectAnalysisRunState(
                action = "pausing", fileSelection = AnalysisSelectionState(selectionWithDetails)),
        )
        .forEach { analysis ->
          ComposeVisualFixture(1_440, 900) {
                AnalysisFileSelector(
                    analysis,
                    AnalysisWorkspaceActions(
                        start = { _, _ -> },
                        pause = {},
                        resume = {},
                        cancel = {},
                        openResults = {},
                        saveSelection = { saves++ },
                        refreshStatus = { error("Unexpected status refresh") }),
                )
              }
              .use { fixture ->
                fixture.render()
                assertTrue(fixture.isDescriptionDisabled("Analyze main.go"))
              }
        }
  }

  @Test
  fun summaryStartRespondsOnceToEnterAndSpaceWithTheSharedPendingGuard() {
    val project = analysisProjectFixture()
    var state by mutableStateOf(ProjectAnalysisRunState())
    val previews = mutableListOf<Pair<AnalysisRunLimits, Boolean>>()
    val actions =
        AnalysisWorkspaceActions(
            { limits, retry ->
              previews += limits to retry
              state = state.copy(action = "previewing")
            },
            {},
            {},
            {},
            {},
            refreshStatus = { error("Unexpected status refresh") })
    ComposeVisualFixture(800, 650) {
          ProjectSummaryPane(null, project, {}, analysisState = state, analysisActions = actions)
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestFocus("Start analysis"))
          assertTrue(fixture.isFocusedControl("Start analysis"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertEquals(listOf(defaultAnalysisRunLimits to false), previews)
          assertTrue(fixture.isDisabled("Start analysis"))
          fixture.pressKey(Key.Spacebar)
          assertEquals(1, previews.size)
          state = state.copy(action = "")
          fixture.render()
          assertTrue(fixture.requestFocus("Start analysis"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          assertEquals(
              listOf(defaultAnalysisRunLimits to false, defaultAnalysisRunLimits to false),
              previews)
          state = state.copy(action = "")
          fixture.render()
          assertTrue(fixture.requestFocus("Start analysis"))
          repeat(2) {
            assertTrue(fixture.pressKey(Key.Tab))
            fixture.render()
          }
          assertTrue(fixture.isFocusedControl("View analysis"))
        }
  }

  @Test
  fun summaryKeyboardControlsKeepDisclosuresLocalAndNavigateOnlyToTheirWorkspace() {
    val destinations = mutableListOf<Workspace>()
    val overview =
        visualFixtureOverview.copy(
            analysis =
                visualFixtureOverview.analysis.copy(
                    engineeringInsight =
                        EngineeringInsight(
                            mechanism = "Validate requests before persistence.",
                            whyItMattersHere = "Invalid input stays outside the repository.",
                            tradeoffOrFailureMode = "Rules need one owner.")))
    ComposeVisualFixture(1_600, 1_000) {
          ProjectSummaryPane(overview, visualFixtureProject, destinations::add)
        }
        .use { fixture ->
          fixture.render()
          assertEquals(1, fixture.tagCount("summary-coverage-results"))
          assertTrue(
              fixture.taggedBounds("analysis-summary").right <=
                  fixture.taggedBounds("summary-results").left)
          repeat(2) {
            assertTrue(fixture.pressKey(Key.Tab), "Tab must advance through the coverage status")
            fixture.render()
          }
          val traversal =
              listOf(
                  "Up to date, 16 files",
                  "Outdated, 4 files",
                  "Not analyzed, 3 files",
                  "View analysis",
                  "View Bugs results",
                  "View Performance results",
                  "View Security results")
          traversal.forEachIndexed { index, control ->
            assertTrue(fixture.pressKey(Key.Tab), "Tab must reach $control")
            fixture.render()
            assertTrue(
                fixture.isFocusedControl(control),
                "Tab position $index must focus $control in Summary visual order")
            when (control) {
              "Up to date, 16 files",
              "Outdated, 4 files",
              "Not analyzed, 3 files" -> {
                assertTrue(fixture.pressKey(Key.Enter))
                fixture.render()
                assertEquals("Inspecting", fixture.descriptionStateDescription(control))
                assertTrue(fixture.isFocusedControl(control))
                assertTrue(fixture.pressKey(Key.Spacebar))
                fixture.render()
                assertEquals("Not inspecting", fixture.descriptionStateDescription(control))
              }
              "View analysis" -> assertTrue(fixture.pressKey(Key.Enter))
              "View Bugs results",
              "View Performance results",
              "View Security results" -> assertTrue(fixture.pressKey(Key.Spacebar))
            }
            fixture.render()
          }
          assertEquals(
              listOf(Workspace.Analysis, Workspace.Bugs, Workspace.Performance, Workspace.Security),
              destinations)
          fixture.scrollBy(100_000f)
          fixture.render()
          assertTrue(fixture.tryClick("Expand More insight"))
          fixture.render()
          assertEquals("Expanded", fixture.descriptionStateDescription("Collapse More insight"))
          fixture.revealText("Architecture")
          fixture.awaitVisibleDescription("Expand Architecture diagram")
          fixture.awaitDescription("Expand Architecture diagram", "Preview")
          assertTrue(fixture.tryClick("Expand Architecture diagram"))
          fixture.awaitDescription("Close Architecture diagram")
          assertEquals(4, destinations.size, "Summary disclosures must not navigate or start work")
        }
  }

  @Test
  fun removedFocusedCoverageEntryMovesFocusWithoutActivatingNavigation() {
    var selection by
        mutableStateOf(
            selectionFixture()
                .copy(files = selectionFixture().files.filter { it.path == "helper.go" }))
    val destinations = mutableListOf<Workspace>()
    ComposeVisualFixture(1000, 760) {
          ProjectSummaryPane(
              null, analysisProjectFixture(), destinations::add, fileSelection = selection)
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestDescriptionFocus("Up to date, 1 file"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertEquals("Inspecting", fixture.descriptionStateDescription("Up to date, 1 file"))
          selection = selection.copy(files = emptyList())
          fixture.render()
          assertEquals(0, fixture.tagCount("summary-coverage-inspection"))
          assertTrue(fixture.isFocusedControl("View analysis"))
          assertEquals(emptyList(), destinations)
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(listOf(Workspace.Analysis), destinations)
        }
  }

  @Test
  fun terminalInputOwnsInterruptAndOrdinaryAppChordsUntilExplicitFocusReturn() {
    assertFalse(appShortcutAllowed(terminalFocused = true))
    assertTrue(appShortcutAllowed(terminalFocused = false))
    assertFalse(terminalReturnShortcut(java.awt.event.KeyEvent.VK_C, control = true, shift = false))
    assertFalse(
        terminalReturnShortcut(java.awt.event.KeyEvent.VK_F12, control = false, shift = true))
    assertFalse(
        terminalReturnShortcut(java.awt.event.KeyEvent.VK_F12, control = true, shift = false))
    assertTrue(terminalReturnShortcut(java.awt.event.KeyEvent.VK_F12, control = true, shift = true))
  }

  @Test
  fun labeledRailKeepsVisibleNamesAndKeyboardReachabilityAtEverySupportedSize() {
    listOf(
            Triple(1440, 900, 1f),
            Triple(1000, 760, 1f),
            Triple(999, 760, 1f),
            Triple(800, 650, 1f),
            Triple(1280, 600, 1.25f),
            Triple(1280, 600, 1.5f))
        .forEach { (width, height, scale) ->
          var active by mutableStateOf(LeftToolWindow.Summary)
          var selections = 0
          val focus = FocusRequester()
          ComposeVisualFixture(width, height, scale) {
                Row(Modifier.fillMaxSize()) {
                  ToolWindowBar(
                      active,
                      {
                        active = it
                        selections++
                      },
                      Modifier.focusRequester(focus),
                      onOpenTerminal = {})
                  Box(Modifier.weight(1f))
                }
              }
              .use { fixture ->
                fixture.render("navigation-labels-$width-$scale")
                assertFalse(fixture.hasText("Perf."))
                workspaceRailOrder.forEach {
                  assertTrue(fixture.hasText(leftToolWindowLabel(it)))
                  assertTrue(fixture.hasDescription(toolWindowSemanticsLabel(it, it == active)))
                }
                workspaceRailOrder.forEach { fixture.assertRailLabelFits(leftToolWindowLabel(it)) }
                listOf(
                        "Project",
                        "Results",
                        "Editing",
                        "Run & progress",
                        "findings",
                        "Running",
                        "Completed")
                    .forEach { assertFalse(fixture.hasText(it)) }
                assertEquals(null, fixture.stateDescription("Performance"))
                focus.requestFocus()
                fixture.render()
                repeat(5) {
                  fixture.pressKey(Key.DirectionDown)
                  fixture.render()
                }
                fixture.render("navigation-editor-focused-$width-$scale")
                assertEquals(0, selections)
                assertEquals(LeftToolWindow.Summary, active)
                assertTrue(fixture.hasDescription("Editor tool window, not selected, focused"))
                assertTrue(fixture.hasText("Editor"))
                assertTrue(fixture.pressKey(Key.Enter))
                fixture.render()
                assertEquals(LeftToolWindow.Editor, active)
                assertEquals(1, selections)
                assertTrue(fixture.hasDescription("Editor tool window, selected, focused"))
                fixture.assertColorVisible(FocusAccent)
                fixture.assertColorVisible(SelectionAccent)
                assertTrue(fixture.pressKey(Key.DirectionUp))
                fixture.render()
                assertEquals(1, selections)
                assertTrue(fixture.hasDescription("Security tool window, not selected, focused"))
                assertTrue(fixture.hasDescription("Editor tool window, selected"))
                assertTrue(fixture.pressKey(Key.Spacebar))
                fixture.render()
                assertEquals(LeftToolWindow.Security, active)
                assertEquals(2, selections)
                assertTrue(fixture.hasDescription("Security tool window, selected, focused"))
              }
        }
  }

  @Test
  fun railScrollRevealsFocusedDestinationWithoutSelectingOrStartingWork() {
    var active by mutableStateOf(LeftToolWindow.Summary)
    val selected = mutableListOf<LeftToolWindow>()
    val focus = FocusRequester()
    ComposeVisualFixture(120, 220, 1.5f) {
          ToolWindowBar(
              active,
              {
                active = it
                selected += it
              },
              Modifier.focusRequester(focus),
              onOpenTerminal = {})
        }
        .use { fixture ->
          fixture.render()
          focus.requestFocus()
          fixture.render()
          repeat(5) {
            assertTrue(fixture.pressKey(Key.DirectionDown))
            fixture.render()
          }
          fixture.awaitVisibleDescription("Editor tool window, not selected, focused")
          val editor = fixture.firstVisibleTextBounds("Editor")
          assertTrue(editor.top >= 0 && editor.bottom <= 220, "Focus must reveal the last label")
          assertEquals(LeftToolWindow.Summary, active)
          assertTrue(selected.isEmpty())
          fixture.clickDescription("Editor tool window, not selected, focused")
          fixture.render()
          assertEquals(listOf(LeftToolWindow.Editor), selected)
        }
  }

  @Test
  fun allNineRailActionsCanBeRevealedByKeyboardWithoutDispatchingWork() {
    var selected by mutableStateOf(LeftToolWindow.Summary)
    var selections = 0
    var utilityActions = 0
    val railFocus = FocusRequester()
    ComposeVisualFixture(180, 220, 1.5f) {
          ToolWindowBar(
              selected,
              {
                selected = it
                selections++
              },
              Modifier.focusRequester(railFocus),
              onOpenTerminal = { utilityActions++ },
              onOpenCommands = { utilityActions++ },
              onOpenModels = { utilityActions++ })
        }
        .use { fixture ->
          fixture.render()
          railFocus.requestFocus()
          fixture.render()
          workspaceRailOrder.forEachIndexed { index, entry ->
            if (index > 0) {
              assertTrue(fixture.pressKey(Key.DirectionDown))
              fixture.render()
            }
            val label = leftToolWindowLabel(entry)
            fixture.awaitVisibleDescription(
                "$label tool window, ${if (index == 0) "selected" else "not selected"}, focused")
          }
          listOf(
                  "Terminal" to "Terminal · Open or focus; may start a local shell",
                  "Commands" to "Commands · Open actions",
                  "Models" to "Models · Configured model details")
              .forEach { (label, description) ->
                assertTrue(fixture.requestDescriptionFocus(description))
                fixture.awaitVisibleDescription(description)
                assertTrue(fixture.isFocusedControl(description), "$label must retain focus")
              }
          assertEquals(0, selections)
          assertEquals(0, utilityActions)
          assertEquals(LeftToolWindow.Summary, selected)
        }
  }

  @Test
  fun terminalRailActionIsSeparateFromWorkspaceSelectionAndKeyPreview() {
    var active by mutableStateOf(LeftToolWindow.Summary)
    var selections = 0
    var opens = 0
    ComposeVisualFixture(180, 340) {
          ToolWindowBar(
              active,
              {
                active = it
                selections++
              },
              onOpenTerminal = { opens++ })
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Terminal"))
          assertTrue(fixture.hasDescription("Terminal · Open or focus; may start a local shell"))
          assertEquals(0, opens)
          assertTrue(
              fixture.requestDescriptionFocus("Terminal · Open or focus; may start a local shell"))
          fixture.render()
          val terminalLabel = fixture.firstVisibleTextBounds("Terminal")
          assertTrue(terminalLabel.top >= 0 && terminalLabel.bottom <= 340)
          assertEquals(0, opens)
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertEquals(1, opens)
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertEquals(2, opens)
          fixture.clickDescription("Terminal · Open or focus; may start a local shell")
          fixture.render()
          assertEquals(3, opens)
          assertEquals(0, selections)
          assertEquals(LeftToolWindow.Summary, active)
        }
  }

  @Test
  fun commandsRailActionIsIndependentOfWorkspaceKeyPreview() {
    var active by mutableStateOf(LeftToolWindow.Summary)
    var selections = 0
    var opens = 0
    ComposeVisualFixture(180, 340) {
          ToolWindowBar(
              active,
              {
                active = it
                selections++
              },
              onOpenTerminal = {},
              onOpenCommands = { opens++ })
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Commands"))
          assertEquals(0, opens)
          assertTrue(fixture.requestDescriptionFocus("Commands · Open actions"))
          fixture.render()
          val label = fixture.firstVisibleTextBounds("Commands")
          assertTrue(label.top >= 0 && label.bottom <= 340)
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertEquals(1, opens)
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertEquals(2, opens)
          fixture.clickDescription("Commands · Open actions")
          fixture.render()
          assertEquals(3, opens)
          assertEquals(0, selections)
          assertEquals(LeftToolWindow.Summary, active)
        }
  }

  @Test
  fun modelsRailActionIsIndependentOfWorkspaceKeyPreview() {
    var active by mutableStateOf(LeftToolWindow.Summary)
    var selections = 0
    var opens = 0
    // Small frame steps exercise the intermediate, partially revealed control.
    ComposeVisualFixture(180, 340, frameDurationNanos = 1_000_000) {
          ToolWindowBar(
              active,
              {
                active = it
                selections++
              },
              onOpenTerminal = {},
              onOpenModels = { opens++ })
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Models"))
          assertTrue(fixture.requestDescriptionFocus("Models · Configured model details"))
          fixture.awaitVisibleDescription("Models · Configured model details")
          val label = fixture.firstVisibleTextBounds("Models")
          assertTrue(label.top >= 0 && label.bottom <= 340)
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertEquals(1, opens)
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertEquals(2, opens)
          fixture.clickDescription("Models · Configured model details")
          fixture.render()
          assertEquals(3, opens)
          assertEquals(0, selections)
          assertEquals(LeftToolWindow.Summary, active)
        }
  }

  @Test
  fun tabGroupArrowsMoveFocusWithoutActivatingTheDestination() {
    val entries = listOf("Source", "Review", "History")

    assertEquals(
        TabGroupInteraction("History"),
        tabGroupInteraction(entries, "Source", TabGroupKey.Previous),
    )
    assertEquals(
        TabGroupInteraction("Review"),
        tabGroupInteraction(entries, "Source", TabGroupKey.Next),
    )
    assertEquals(
        TabGroupInteraction("Source", "Source"),
        tabGroupInteraction(entries, "Source", TabGroupKey.Activate),
    )
  }

  @Test
  fun activityRailCyclesTheSixRetainedWorkspaces() {
    val entries = workspaceRailOrder

    assertEquals(
        LeftToolWindow.Analysis,
        tabGroupInteraction(entries, LeftToolWindow.Summary, TabGroupKey.Next)?.focused)
    assertEquals(
        LeftToolWindow.Editor,
        tabGroupInteraction(entries, LeftToolWindow.Summary, TabGroupKey.Previous)?.focused)
    assertEquals(
        LeftToolWindow.Problems,
        tabGroupInteraction(entries, LeftToolWindow.Analysis, TabGroupKey.Next)?.focused)
    assertEquals(
        LeftToolWindow.Performance,
        tabGroupInteraction(entries, LeftToolWindow.Problems, TabGroupKey.Next)?.focused)
    assertEquals(
        TabGroupInteraction(LeftToolWindow.Editor, LeftToolWindow.Editor),
        tabGroupInteraction(entries, LeftToolWindow.Editor, TabGroupKey.Activate),
    )
  }

  @Test
  fun escapeSelectsOnlyTheTopmostTransientSurface() {
    assertEquals(
        TransientSurface.Context,
        topmostTransientSurface(
            contextVisible = true,
            paletteVisible = true,
            statusDetailsVisible = true,
        ),
    )
    assertEquals(
        TransientSurface.StatusDetails,
        topmostTransientSurface(
            contextVisible = false,
            paletteVisible = false,
            statusDetailsVisible = true,
        ),
    )
    assertNull(
        topmostTransientSurface(
            contextVisible = false,
            paletteVisible = false,
            statusDetailsVisible = false,
        ),
    )
  }

  @Test
  fun focusedTabsKeepTextualAccessibilityStateAtCompactWidths() {
    assertEquals(
        "Editor tool window, selected, focused",
        toolWindowSemanticsLabel(LeftToolWindow.Editor, selected = true, focused = true),
    )
    assertEquals(
        "Context tool window tab, not selected, focused",
        rightToolWindowTabDescription(
            RightToolWindow.Context,
            selected = false,
            focused = true,
        ),
    )
  }

  @Test
  fun editorBreadcrumbsKeepLongTextBounded() {
    assertEquals(
        listOf("very", "…", "main.go", "Run"),
        editorBreadcrumbSegments("very/long/project/path/main.go", "Run").map { it.label },
    )
  }
}

/** Selectable evidence has passive tab stops between actions; traverse rather than skipping it. */
internal fun tabToBenchmarkControl(
    fixture: ComposeVisualFixture,
    tag: String,
    reverse: Boolean = false,
) {
  repeat(12) {
    assertTrue(fixture.pressKey(Key.Tab, shift = reverse))
    awaitBenchmarkScrollSettled(fixture)
    if (fixture.isTaggedNodeFocused(tag)) return
  }
  error("Keyboard traversal did not reach $tag")
}

// The offscreen scene uses Unconfined, while global snapshot notifications can arrive on
// another dispatcher. Flush them on the fixture thread before pumping reveal animations;
// a stable scroll value alone does not mean pending focus/layout state has been delivered.
private fun awaitBenchmarkScrollSettled(fixture: ComposeVisualFixture) {
  val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(2)
  var previous = Float.NaN
  var stableSince = System.nanoTime()
  do {
    Thread.sleep(20)
    Snapshot.sendApplyNotifications()
    fixture.render()
    val scroll = fixture.verticalScrollValue("result-overview")
    if (scroll != previous) stableSince = System.nanoTime()
    previous = scroll
    if (System.nanoTime() - stableSince >= TimeUnit.MILLISECONDS.toNanos(100)) return
  } while (System.nanoTime() < deadline)
  error("Benchmark overview reveal did not settle: $previous")
}

private fun awaitBenchmarkReveal(fixture: ComposeVisualFixture, tag: String, minimumHeight: Float) {
  val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(2)
  do {
    Snapshot.sendApplyNotifications()
    fixture.render()
    val bounds = fixture.taggedBounds(tag)
    val overview = fixture.taggedBounds("result-overview")
    if (bounds.height >= minimumHeight &&
        bounds.top >= overview.top &&
        bounds.bottom <= overview.bottom &&
        bounds.width > 0f)
        return
    Thread.sleep(20)
  } while (System.nanoTime() < deadline)
  fixture.render("f21-reveal-failure-$tag")
  error(
      "Keyboard focus did not reveal $tag: ${fixture.taggedBounds(tag)} / ${fixture.taggedBounds("result-overview")}; focused=${fixture.isTaggedNodeFocused(tag)}; header=${runCatching { fixture.firstVisibleTextBounds("Daemon-returned argv (read-only)") }}; last=${runCatching { fixture.firstVisibleTextBounds("argv[4] = \"^BenchmarkWork24$\"") }}")
}
