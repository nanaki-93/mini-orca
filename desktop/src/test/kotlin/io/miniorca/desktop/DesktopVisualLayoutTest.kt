package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.ExperimentalComposeUiApi
import androidx.compose.ui.InternalComposeUiApi
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.asComposeCanvas
import androidx.compose.ui.platform.PlatformContext
import androidx.compose.ui.scene.CanvasLayersComposeScene
import androidx.compose.ui.scene.ComposeSceneContext
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.semantics.SemanticsNode
import androidx.compose.ui.semantics.SemanticsOwner
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.TextLayoutResult
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.IntSize
import androidx.compose.ui.unit.dp
import java.io.File
import kotlin.test.Test
import kotlin.test.assertFalse
import kotlin.test.assertTrue
import kotlinx.coroutines.Dispatchers
import org.jetbrains.skia.Surface

/** Renders production components with explicit test data, without a daemon or provider. */
class DesktopVisualLayoutTest {
  @Test
  fun compactFieldKeepsEditingAndItsAccessibleNameAfterInput() {
    var query by mutableStateOf("")
    ComposeVisualFixture(360, 100) {
          CompactSingleLineField(query, { query = it }, "Search findings", showLabel = false)
        }
        .use { fixture ->
          fixture.render()
          fixture.assertTextFits("Search findings")
          fixture.setText("repository")
          fixture.render()
          kotlin.test.assertEquals("repository", query)
          assertTrue(fixture.hasText("repository"))
          assertTrue(fixture.hasDescription("Search findings"))
        }
  }

  @Test
  fun analysisChromeFitsWideNarrowAndEnlargedTextViewports() {
    listOf(
            Triple(1440, 900, 1f),
            Triple(1000, 760, 1f),
            Triple(999, 760, 1f),
            Triple(800, 650, 1f),
            Triple(1000, 800, 1.3f))
        .forEach { (width, height, scale) ->
          ComposeVisualFixture(width, height, scale) { AnalysisVisualFixture(width.toFloat()) }
              .use { fixture ->
                fixture.render("analysis-$width-$scale")
                listOf(if (scale > 1.15f) "Perf." else "Performance", "Preview").forEach { label ->
                  fixture.assertTextFits(label)
                }
              }
        }
  }

  @Test
  fun previewMenuAndDialogDoNotInvokeAnalysisActions() {
    var requests = 0
    val actions =
        AnalysisWorkspaceActions(
            { requests++ }, { requests++ }, { requests++ }, { requests++ }, { requests++ })
    ComposeVisualFixture(1440, 900) { AnalysisVisualFixture(1440f, actions) }
        .use { fixture ->
          fixture.render()
          fixture.clickText("Pause")
          kotlin.test.assertEquals(1, requests)
          fixture.clickText("Preview")
          fixture.render()
          fixture.clickText("New file")
          fixture.render()
          assertTrue(fixture.hasText("New file · Preview"))
          fixture.clickText("Close")
          fixture.render()
          assertFalse(fixture.hasText("New file · Preview"))
          kotlin.test.assertEquals(1, requests)
        }
  }

  @Test
  fun editorComponentsRenderAtDockedAndDrawerWidths() {
    listOf(1440 to 900, 1000 to 760, 999 to 760).forEach { (width, height) ->
      ComposeVisualFixture(width, height) { EditorVisualFixture(width.toFloat()) }
          .use { fixture ->
            fixture.render("editor-$width")
            fixture.assertTextFits("Preview")
            fixture.assertTextFits("Performance")
            if (width >= 1_000) fixture.assertTextFits("Files")
          }
    }
  }

  @Test
  fun sharedControlsRenderReadableStatesAtEnlargedTextScale() {
    ComposeVisualFixture(720, 180, 1.3f) { SharedControlsVisualFixture() }
        .use { fixture ->
          fixture.render("shared-controls-130")
          listOf("Apply", "Selected", "Disabled", "Focused", "Search files").forEach {
            fixture.assertTextFits(it)
          }
        }
  }

  @Test
  fun baselineCapturesSummaryAndExercisesOpenProjectAndPreviewMenus() {
    ComposeVisualFixture(1440, 900) {
          ProjectSummaryPane(visualFixtureOverview, visualFixtureProject, {})
        }
        .use { fixture ->
          fixture.render("summary-1440")
          assertTrue(fixture.hasText("Project facts"))
          assertTrue(fixture.hasText("Workspace coverage"))
        }

    ComposeVisualFixture(1440, 900) { ToolbarVisualFixture(1440f) }
        .use { fixture ->
          fixture.render()
          fixture.clickText("go-shop · fixture")
          fixture.render("project-menu-open")
          assertTrue(fixture.hasText("Open project"))
          assertTrue(fixture.hasText("Re-index project"))
        }

    ComposeVisualFixture(1440, 900) { ToolbarVisualFixture(1440f) }
        .use { fixture ->
          fixture.render()
          fixture.clickText("Preview")
          fixture.render("preview-menu-open")
          assertTrue(fixture.hasText("New file"))
          assertTrue(fixture.hasText("Branch actions"))
        }
  }

  @Test
  fun baselineCapturesTheOpenEngineeringInsightDisclosure() {
    ComposeVisualFixture(720, 420) {
          EngineeringInsightPanel(
              EngineeringInsight(
                  mechanism = "The handler validates its identifier before the repository call.",
                  whyItMattersHere = "The returned error remains distinguishable for the caller.",
                  tradeoffOrFailureMode =
                      "Malformed input otherwise reaches the persistence layer.",
                  transferableLesson = "Keep boundary validation close to request handling."),
              scopeLabel = "Visual fixture")
        }
        .use { fixture ->
          val wasCollapsed = fixture.hasText("> Engineering insight")
          if (wasCollapsed) fixture.clickText("> Engineering insight")
          fixture.render("engineering-insight-open")
          assertTrue(fixture.hasText("Close insight"))
          if (wasCollapsed) fixture.clickText("⌄ Engineering insight")
        }
  }

  @Test
  fun problemRowsRevealDetailsBeforeAnyWorkflowAction() {
    var sourceRequests = 0
    var mutations = 0
    ComposeVisualFixture(900, 500) {
          ProblemsToolWindow(
              ProblemsToolWindowState(visualFixtureFindings, false),
              FindingActions({ sourceRequests++ }, { mutations++ }, { _, _ -> mutations++ }))
        }
        .use { fixture ->
          fixture.render()
          fixture.clickText("Validate the user identifier")
          fixture.render()
          kotlin.test.assertEquals(0, sourceRequests)
          kotlin.test.assertEquals(0, mutations)
          fixture.clickText("Open source")
          kotlin.test.assertEquals(1, sourceRequests)
          kotlin.test.assertEquals(0, mutations)
        }
  }
}

/** This test-only adapter is tied to the Compose version pinned in build.gradle.kts. */
@OptIn(ExperimentalComposeUiApi::class, InternalComposeUiApi::class)
private class ComposeVisualFixture(
    private val width: Int,
    private val height: Int,
    fontScale: Float = 1f,
    content: @Composable () -> Unit,
) : AutoCloseable {
  private val owners = mutableListOf<SemanticsOwner>()
  private val platform =
      object : PlatformContext by PlatformContext.Empty {
        override val semanticsOwnerListener =
            object : PlatformContext.SemanticsOwnerListener {
              override fun onSemanticsOwnerAppended(semanticsOwner: SemanticsOwner) {
                owners += semanticsOwner
              }

              override fun onSemanticsOwnerRemoved(semanticsOwner: SemanticsOwner) {
                owners -= semanticsOwner
              }

              override fun onSemanticsChange(semanticsOwner: SemanticsOwner) = Unit

              override fun onLayoutChange(semanticsOwner: SemanticsOwner, semanticsNodeId: Int) =
                  Unit
            }
      }
  private val scene =
      CanvasLayersComposeScene(
          density = Density(1f, fontScale),
          size = IntSize(width, height),
          coroutineContext = Dispatchers.Unconfined,
          composeSceneContext =
              object : ComposeSceneContext {
                override val platformContext = platform
              })
  private val surface = Surface.makeRasterN32Premul(width, height)
  private var frameTime = 0L

  init {
    scene.setContent { MiniOrcaTheme { content() } }
  }

  fun render(name: String? = null) {
    repeat(3) {
      scene.render(surface.canvas.asComposeCanvas(), frameTime)
      frameTime += 80_000_000
    }
    if (name != null)
        System.getProperty("miniOrca.visualOutput")?.let { output ->
          val directory = File(output).apply { mkdirs() }
          surface.makeImageSnapshot().use { rendered ->
            requireNotNull(rendered.encodeToData()).use { data ->
              File(directory, "$name.png").writeBytes(data.bytes)
            }
          }
        }
  }

  fun hasText(label: String): Boolean =
      textNodes(label).isNotEmpty() ||
          nodes().any { it.config.getOrNull(SemanticsProperties.EditableText)?.text == label }

  fun hasDescription(label: String): Boolean =
      nodes().any {
        it.config.getOrNull(SemanticsProperties.ContentDescription)?.contains(label) == true
      }

  fun setText(value: String) {
    val editor = nodes().single { it.config.getOrNull(SemanticsActions.SetText) != null }
    assertTrue(
        requireNotNull(editor.config.getOrNull(SemanticsActions.SetText)?.action)
            .invoke(AnnotatedString(value)))
  }

  fun clickText(label: String) {
    var node: SemanticsNode? = textNodes(label).firstOrNull()
    while (node != null) {
      val click = node.config.getOrNull(SemanticsActions.OnClick)?.action
      if (click != null) {
        assertTrue(click())
        return
      }
      node = node.parent
    }
    error("No clickable control for $label")
  }

  fun assertTextFits(label: String) {
    val matches = textNodes(label)
    assertTrue(matches.isNotEmpty(), "$label must be visible at $width")
    matches.forEach { node ->
      val layouts = mutableListOf<TextLayoutResult>()
      node.config.getOrNull(SemanticsActions.GetTextLayoutResult)?.action?.invoke(layouts)
      assertTrue(layouts.isNotEmpty())
      layouts.forEach { layout ->
        assertFalse(
            (0 until layout.lineCount).any(layout::isLineEllipsized),
            "$label is truncated at $width")
        // Glyph measurements can round down by a subpixel.
        assertTrue(
            layout.multiParagraph.height <= layout.size.height + 1f,
            "$label clips vertically at $width")
        assertTrue(layout.lineCount == 1, "$label must fit on one line at $width")
      }
      assertTrue(node.boundsInRoot.right <= width && node.boundsInRoot.bottom <= height)
    }
  }

  private fun nodes() = owners.flatMap { descendants(it.unmergedRootSemanticsNode) }

  private fun textNodes(label: String) =
      nodes().filter { node ->
        node.config.getOrNull(SemanticsProperties.Text)?.any { it.text == label } == true
      }

  private fun descendants(node: SemanticsNode): List<SemanticsNode> =
      listOf(node) + node.children.flatMap(::descendants)

  override fun close() {
    scene.close()
    surface.close()
  }
}

@Composable
private fun AnalysisVisualFixture(
    width: Float,
    actions: AnalysisWorkspaceActions = AnalysisWorkspaceActions({}, {}, {}, {}, {}),
) {
  val summaries =
      mapOf(BottomToolWindow.Problems to BottomToolWindowSummary("No actionable problems"))
  val layout = DesktopLayoutState(bottomCollapsed = true)
  Column(Modifier.fillMaxSize().background(AppBackground)) {
    MainToolbar(
        ToolbarState(
            width,
            visualFixtureProject,
            false,
            "",
            ConnectionState(connected = true),
            GitStatus(available = true, branch = "main"),
            false),
        ToolbarActions({}, {}, {}, {}, {}, {}))
    Row(Modifier.fillMaxWidth().weight(1f)) {
      ToolWindowBar(LeftToolWindow.Analysis, {})
      Column(Modifier.weight(1f)) {
        Box(Modifier.weight(1f)) {
          AnalysisWorkspacePane(
              AnalysisWorkspacePaneState(
                  visualFixtureJob, AnalysisCoverage(total = 23, stale = 23), ScopedModel(), false),
              actions)
        }
        if (useNarrowLayout(width)) {
          NarrowBottomToolWindowSummary(layout, BottomToolWindow.entries, summaries, {})
        } else {
          BottomToolWindowRegion(
              layout, BottomToolWindow.entries, summaries, {}, {}, {}, {}, { _, _ -> })
        }
      }
    }
    PersistentStatusBar(
        DesktopStatusBarPresentation(
            listOf(
                DesktopStatusSegment(
                    DesktopStatusSegmentType.Operation,
                    "Visual fixture · no backend",
                    "Rendered Compose layout fixture; all data is test data",
                    0),
                DesktopStatusSegment(
                    DesktopStatusSegmentType.Index, "23 indexed files", "Fixture files", 1),
            )),
        width,
        {})
  }
}

private val visualFixtureProject =
    ProjectAnalysis(
        "visual-fixture",
        "fixture-revision",
        "go-shop · fixture",
        "",
        "Go",
        fileCount = 23,
        sourceFileCount = 23,
        totalLines = 1800,
        summary = "Test fixture",
        aiStatus = "fresh",
        analyzedAt = "")

private val visualFixtureOverview =
    ProjectOverview(
        projectId = "visual-fixture",
        projectRevision = "fixture-revision",
        metrics =
            ProjectMetrics(
                type = "Go",
                buildFile = "go.mod",
                fileCount = 23,
                sourceFileCount = 23,
                totalLines = 1800,
                languages = mapOf("Go" to 21, "Markdown" to 2)),
        analysis =
            StructuredProjectAnalysis(
                status = "fresh",
                purpose = "Go service with a small HTTP API and a repository layer.",
                architecture = "HTTP handlers delegate through services to repository adapters.",
                components = listOf("API handlers", "Repository adapters"),
                entryPoints = listOf("cmd/server/main.go"),
                flows = listOf("HTTP request to handler to service to repository"),
                risks = listOf(ProjectAnalysisRisk("medium", "Input validation is incomplete.")),
                nextSteps = listOf("Review boundary validation.")),
        analysisCoverage = AnalysisCoverage(total = 23, fresh = 16, stale = 4, missing = 3),
        findingCounts = FindingCounts(verified = 2, aiSuggestions = 4))

@Composable
private fun ToolbarVisualFixture(width: Float) {
  Column(Modifier.fillMaxSize().background(AppBackground)) {
    MainToolbar(
        ToolbarState(
            width,
            visualFixtureProject,
            false,
            "",
            ConnectionState(connected = true),
            GitStatus(available = true, branch = "main"),
            false),
        ToolbarActions({}, {}, {}, {}, {}, {}))
  }
}

@Composable
private fun SharedControlsVisualFixture() {
  Column(Modifier.fillMaxSize().background(AppBackground).padding(12.dp)) {
    Row(horizontalArrangement = androidx.compose.foundation.layout.Arrangement.spacedBy(8.dp)) {
      MiniOrcaButton(onClick = {}, tone = ActionTone.Primary) { Text("Apply") }
      MiniOrcaButton(onClick = {}, tone = ActionTone.Navigation, selected = true) {
        Text("Selected")
      }
      MiniOrcaButton(onClick = {}, enabled = false) { Text("Disabled") }
      ChromeButton(onClick = {}, focusHighlight = true) { Text("Focused") }
    }
    Spacer(Modifier.height(8.dp))
    CompactSingleLineField(
        value = "",
        onValueChange = {},
        label = "Search files",
        showLabel = false,
        modifier = Modifier.fillMaxWidth())
  }
}

private val visualFixtureJob =
    AnalyzeAllJob(
        status = "running",
        maxFiles = 100,
        maxRetries = 1,
        files =
            List(19) { index ->
              AnalyzeAllFileJob(
                  path = "internal/api/handler_$index.go",
                  status =
                      when {
                        index < 8 -> "completed"
                        index == 8 -> "running"
                        else -> "pending"
                      })
            })

@Composable
private fun EditorVisualFixture(width: Float) {
  val layout = DesktopLayoutState(bottomCollapsed = false)
  val panes = dockedPaneWidths(width, layout.explorerWidth, layout.actionWidth)
  val symbol =
      SymbolInfo(
          "GetUser", "function", "func GetUser(id string) (User, error)", 5, 12, "exact", true)
  val file =
      ProjectFileInfo(
          "internal/api/user.go",
          "fixture-hash",
          "user.go",
          language = "Go",
          sizeBytes = 480,
          lineCount = 18,
          modifiedAt = "",
          binary = false,
          content =
              """
        package api

        import "errors"

        func GetUser(id string) (User, error) {
            if id == "" {
                return User{}, errors.New("missing user id")
            }

            user, err := repository.Find(id)
            return user, err
        }

        type User struct {
            ID   string
            Name string
        }
      """
                  .trimIndent())
  val index =
      ProjectIndex(
          "visual-fixture",
          "fixture-revision",
          files =
              listOf(
                      "cmd/server/main.go",
                      "internal/api/routes.go",
                      file.path,
                      "internal/db/store.go",
                      "internal/models/user.go",
                      "go.mod",
                      "README.md")
                  .map { IndexedFile(it, "fixture-hash", "Go", false, analysisStatus = "fresh") })
  val analysis =
      FileAnalysis(
          file.path,
          "fresh",
          purpose = "Resolves user requests and delegates persistence to the repository.",
          symbolExplanations =
              mapOf(
                  "GetUser" to
                      "Validates the identifier before looking up a user. Returns the repository result and preserves its error."))
  val inspector =
      symbolInspectorUiState(
          file, listOf(symbol), symbol, analysis, false, InspectorProviderState(false, false), null)
  Column(Modifier.fillMaxSize().background(AppBackground)) {
    MainToolbar(
        ToolbarState(
            width,
            visualFixtureProject,
            false,
            "",
            ConnectionState(connected = true),
            GitStatus(available = true, branch = "main"),
            useNarrowLayout(width)),
        ToolbarActions({}, {}, {}, {}, {}, {}))
    Row(Modifier.fillMaxWidth().weight(1f)) {
      ToolWindowBar(LeftToolWindow.Editor, {})
      Column(Modifier.weight(1f)) {
        Row(Modifier.fillMaxWidth().weight(1f)) {
          if (!useNarrowLayout(width)) {
            ExplorerPane(
                ExplorerPaneState(index, file.path, "", emptySet(), false),
                ExplorerPaneActions({}, {}, {}, {}, {}),
                Modifier.width(panes.explorer.dp))
            ResizableDivider({}, {})
          }
          EditorWorkspace(
              EditorChromeUiState(
                  file.name,
                  file.path,
                  editorBreadcrumbLabel(file.path, symbol.name),
                  "Read-only source fixture",
                  EditorSurface.Source,
                  false,
                  "SOURCE"),
              null,
              {},
              canvas = {
                SourceEditorPane(
                    visualFixtureProject, file, listOf(symbol), symbol, 7, emptyList(), {})
              },
              modifier = Modifier.weight(1f))
        }
        if (useNarrowLayout(width)) {
          NarrowBottomToolWindowSummary(layout, BottomToolWindow.entries, emptyMap(), {})
        } else {
          BottomToolWindowRegion(
              layout,
              BottomToolWindow.entries,
              emptyMap(),
              {},
              {},
              {},
              {},
              { _, modifier ->
                ProblemsToolWindow(
                    ProblemsToolWindowState(visualFixtureFindings, false),
                    FindingActions({}, {}, { _, _ -> }),
                    modifier)
              })
        }
      }
      if (!useNarrowLayout(width)) {
        ResizableDivider({}, {})
        RightToolWindowContainer(
            RightToolWindow.Context,
            {},
            content = { _, modifier ->
              ContextToolWindow(
                  ContextToolWindowState(
                      inspector,
                      ScopedModel(),
                      false,
                      null,
                      null,
                      analysis,
                      visualFixtureProject,
                      ProjectOverview(
                          analysis =
                              StructuredProjectAnalysis(
                                  status = "fresh",
                                  purpose =
                                      "Go service with a small HTTP API and a repository layer."),
                          metrics =
                              ProjectMetrics(
                                  type = "Go",
                                  buildFile = "go.mod",
                                  languages = mapOf("Go" to 7)))),
                  ContextToolWindowActions({}, {}, {}, {}, {}),
                  modifier)
            },
            modifier = Modifier.width(panes.action.dp))
      }
    }
    PersistentStatusBar(
        DesktopStatusBarPresentation(
            listOf(
                DesktopStatusSegment(
                    DesktopStatusSegmentType.Operation,
                    "Visual fixture · no backend",
                    "Rendered Compose layout fixture; all data is test data",
                    0))),
        width,
        {})
  }
}

private val visualFixtureFindings =
    listOf(
        UnifiedFinding(
            id = "fixture-1",
            severity = "high",
            source = "file_analysis",
            confidence = "suggested",
            title = "Validate the user identifier",
            message = "Check malformed identifiers before querying the repository.",
            location = FindingLocation("internal/api/user.go", 6),
            status = "open",
            freshness = "fresh"),
        UnifiedFinding(
            id = "fixture-2",
            severity = "medium",
            source = "file_analysis",
            confidence = "suggested",
            title = "Add context to repository errors",
            message = "Include the operation name when returning repository failures.",
            location = FindingLocation("internal/api/user.go", 11),
            status = "open",
            freshness = "fresh"),
    )
