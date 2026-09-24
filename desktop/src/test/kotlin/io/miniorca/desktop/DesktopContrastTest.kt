package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.material.Text
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import kotlin.test.Test
import kotlin.test.assertTrue

class DesktopContrastTest {
  @Test
  fun analysisHeadersRenderDistinctLabeledLifecycleStates() {
    listOf(
            "completed" to Success,
            "running" to Information,
            "partial" to Warning,
            "failed" to Error)
        .forEach { (status, color) ->
          val run =
              analysisRunFixture().let {
                it.copy(
                    status = status,
                    sections = it.sections.map { section -> section.copy(status = status) })
              }
          val presentation = projectRunPresentation(ProjectAnalysisRunState(run = run))
          ComposeVisualFixture(800, 650, 1.5f) {
                AnalysisWorkspacePane(
                    AnalysisWorkspacePaneState(
                        resultProjectFixture(), ProjectAnalysisRunState(run = run)),
                    AnalysisWorkspaceActions({ _, _ -> }, {}, {}, {}, {}))
              }
              .use { fixture ->
                fixture.render("analysis-state-color-$status")
                fixture.assertTextFits(presentation.status)
                fixture.assertTextContrast(
                    presentation.status, labelBadgeBackground(analysisStatusTint(status)))
                val title = analysisRunTitle(run, presentation)
                fixture.assertTextFits(title, maxLines = 3)
                fixture.assertTextContrast(title, Panel)
                assertTrue(!fixture.hasText(presentation.headline))
                fixture.assertColorVisible(color)
              }
        }
  }

  @Test
  fun everyActionStateKeepsTextReadableOnEveryHostSurface() {
    val hosts = listOf(ActivityRail, Panel, EditorCanvas, OverlaySurface, SelectionSurface)
    ActionTone.entries.forEach { tone ->
      val style = actionToneStyle(tone)
      hosts.forEach { host ->
        listOf(style.background, style.pressedBackground).forEach { fill ->
          assertTrue(
              contrastRatio(style.content, blendOver(fill, host)) >= 4.5,
              "$tone label must remain readable on $fill over $host")
        }
        val selected = blendOver(style.selectedBackground, host)
        assertTrue(
            contrastRatio(style.selectedContent, selected) >= 4.5, "$tone selected label on $host")
        assertTrue(
            contrastRatio(style.disabledContent, blendOver(style.disabledBackground, host)) >= 4.5,
            "$tone disabled label on $host")
      }
    }
  }

  @Test
  fun badgesResolveTheirTintAndOutlineOnEachActionHost() {
    val hosts = listOf(ActivityRail, Panel, EditorCanvas, OverlaySurface, SelectionSurface)
    val tints = listOf(SelectionAccent, Information, Success, Warning, Error, SecondaryText)
    hosts.forEach { host ->
      tints.forEach { tint ->
        // IdeLabelBadge paints an opaque panel-resolved tint before the translucent outline.
        val fill = blendOver(labelBadgeBackground(tint), host)
        val outline = blendOver(tint.copy(alpha = 0.75f), fill)
        assertTrue(contrastRatio(tint, fill) >= 4.5, "Badge $tint on $host")
        assertTrue(contrastRatio(outline, fill) >= 3.0, "Badge outline $tint on $host")
      }
    }
  }

  @Test
  fun selectionAndFocusIndicatorsRemainVisibleOnSelectedActions() {
    val hosts = listOf(ActivityRail, Panel, EditorCanvas, OverlaySurface, SelectionSurface)
    hosts.forEach { host ->
      ActionTone.entries.forEach { tone ->
        val selected = blendOver(actionToneStyle(tone).selectedBackground, host)
        assertTrue(contrastRatio(SelectionAccent, selected) >= 3.0, "$tone selection on $host")
        assertTrue(contrastRatio(FocusAccent, selected) >= 3.0, "$tone focus on $host")
      }
      // Focus uses an adjacent dark keyline, not a replacement for the selection underline.
      assertTrue(contrastRatio(FocusAccent, ActivityRail) >= 3.0, "Focus keyline")
      assertTrue(contrastRatio(SelectionAccent, ActivityRail) >= 3.0, "Selection underline")
    }
  }

  @Test
  fun secondaryTextSyntaxAndStatusLabelsRemainReadableOnSelectedAndRaisedSurfaces() {
    val text = listOf(PrimaryText, SecondaryText, FaintText, Information, Success, Warning, Error)
    val syntax = listOf(CodeKeyword, CodeFunction, CodeString, CodeComment, CodeType)
    listOf(ActivityRail, Panel, EditorCanvas, HeaderSurface, StrongSurface, SelectionSurface)
        .forEach { background ->
          (text + syntax).forEach { foreground ->
            assertTrue(contrastRatio(foreground, background) >= 4.5, "$foreground on $background")
          }
        }
  }

  @Test
  fun productionActionsAndBadgesRenderReadableLabelsAtLargeText() {
    listOf(1f, 1.5f).forEach { scale ->
      ComposeVisualFixture(1000, 650, scale) {
            Column(
                Modifier.fillMaxSize().background(Panel).padding(12.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp)) {
                  IdePaneHeader("Actions and states", stateLabel = "Local visual fixture")
                  ActionTone.entries.forEach { tone ->
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                      Text(tone.name, modifier = Modifier.width(140.dp))
                      MiniOrcaButton(onClick = {}, tone = tone) { Text("${tone.name} action") }
                      MiniOrcaButton(onClick = {}, tone = tone, selected = true) {
                        Text("${tone.name} selected")
                      }
                      MiniOrcaButton(onClick = {}, tone = tone, enabled = false) {
                        Text("${tone.name} disabled")
                      }
                    }
                  }
                  Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    listOf("fresh", "running", "stale", "failed", "missing").forEach {
                      StatusBadge(it)
                    }
                  }
                  CompactSingleLineField("", {}, "Search files")
                }
          }
          .use { fixture ->
            fixture.render("semantic-actions-$scale")
            ActionTone.entries.forEach { tone ->
              val style = actionToneStyle(tone)
              listOf(
                      "action" to style.background,
                      "selected" to style.selectedBackground,
                      "disabled" to style.disabledBackground)
                  .forEach { (state, background) ->
                    fixture.assertTextFits("${tone.name} $state")
                    fixture.assertTextContrast("${tone.name} $state", background)
                  }
            }
            listOf("fresh", "running", "stale", "failed", "missing").forEach { status ->
              val style = statusBadgeStyle(status)
              fixture.assertTextFits(style.label)
              fixture.assertTextContrast(style.label, labelBadgeBackground(style.color))
            }
            fixture.assertTextFits("Search files")
          }
    }
  }

  @Test
  fun sharedChromeRendersSelectionFocusTooltipMenuAndDialogSurfaces() {
    var activated = false
    ComposeVisualFixture(680, 420, 1.5f) {
          Column(Modifier.fillMaxSize().background(Panel).padding(12.dp)) {
            ChromeTab(
                onClick = {},
                selected = true,
                focusHighlight = true,
                accessibleName = "Selected tab") {
                  Text("Selected tab")
                }
            IdeControlTooltip("Keyboard shortcut")
            IdePopupMenuSurface(
                content = {
                  IdeDropdownMenuItem(label = "Open project", onClick = { activated = true })
                })
            IdeDialog(
                onDismissRequest = {},
                title = { Text("Confirm action", color = PrimaryText) },
                content = { Text("Local operation", color = SecondaryText) },
                actions = { MiniOrcaButton(onClick = { activated = true }) { Text("Continue") } })
          }
        }
        .use { fixture ->
          fixture.render("shared-chrome-overlays")
          listOf(
                  "Selected tab",
                  "Keyboard shortcut",
                  "Open project",
                  "Confirm action",
                  "Local operation",
                  "Continue")
              .forEach(fixture::assertTextFits)
          fixture.assertTextContrast("Selected tab", SelectionSurface)
          fixture.assertTextContrast("Keyboard shortcut", OverlaySurface)
          fixture.assertTextContrast("Open project", Card)
          fixture.assertTextContrast("Confirm action", OverlaySurface)
          fixture.assertTextContrast("Local operation", OverlaySurface)
          assertTrue(!activated)
        }
  }

  @Test
  fun selectedChromeEdgeAndFocusedSelectedChromeRenderIndependentIndicators() {
    listOf(false to SelectionAccent, true to FocusAccent).forEach { (focused, indicator) ->
      ComposeVisualFixture(300, 100) {
            Column(Modifier.fillMaxSize().background(Panel).padding(12.dp)) {
              ChromeButton(onClick = {}, selected = true, focusHighlight = focused) {
                Text("Selected")
              }
            }
          }
          .use { fixture ->
            fixture.render("selected-chrome-focus-$focused")
            fixture.assertColorVisible(indicator)
            fixture.assertTextContrast("Selected", SelectionSurface)
          }
    }
    ComposeVisualFixture(300, 100) {
          Column(Modifier.fillMaxSize().background(Panel).padding(12.dp)) {
            ChromeTab(onClick = {}, selected = true, focusHighlight = true) { Text("Focused tab") }
          }
        }
        .use { fixture ->
          fixture.render("selected-tab-focus")
          fixture.assertColorVisible(FocusAccent)
          fixture.assertColorVisible(SelectionAccent)
          fixture.assertTextContrast("Focused tab", SelectionSurface)
        }
  }

  @Test
  fun selectedBrightActionsRenderReadableTextAndSeparateSelectionAndFocusIndicators() {
    listOf(ActionTone.Primary, ActionTone.PositivePrimary).forEach { tone ->
      listOf(false, true).forEach { focused ->
        ComposeVisualFixture(300, 120) {
              Column(Modifier.fillMaxSize().background(Panel).padding(12.dp)) {
                MiniOrcaButton(
                    onClick = {}, tone = tone, selected = true, focusHighlight = focused) {
                      Text("Selected $tone")
                    }
              }
            }
            .use { fixture ->
              fixture.render("selected-$tone-focus-$focused")
              fixture.assertTextFits("Selected $tone")
              fixture.assertTextContrast("Selected $tone", actionToneStyle(tone).selectedBackground)
              fixture.assertColorVisible(if (focused) FocusAccent else SelectionAccent)
              if (focused) fixture.assertColorVisible(ActivityRail)
            }
      }
    }
  }

  @Test
  fun focusedPrimaryActionRetainsBothFocusKeylinesInTheRenderedPixels() {
    ComposeVisualFixture(300, 120) {
          Column(Modifier.fillMaxSize().background(Panel).padding(12.dp)) {
            MiniOrcaButton(onClick = {}, tone = ActionTone.Primary, focusHighlight = true) {
              Text("Apply")
            }
          }
        }
        .use { fixture ->
          fixture.render("primary-action-focus")
          fixture.assertColorVisible(FocusAccent)
          fixture.assertColorVisible(ActivityRail)
          fixture.assertTextContrast("Apply", ActionFill)
        }
  }

  @Test
  fun toolbarStatusesStayReadableOnTheActivityRail() {
    listOf(Information, Warning, SecondaryText, Success, Error).forEach { color ->
      assertTrue(
          contrastRatio(color, ActivityRail) >= 4.5,
          "$color toolbar status label must remain readable on flat chrome")
    }
    ComposeVisualFixture(1_440, 120) {
          MainToolbar(
              ToolbarState(
                  project = visualFixtureProject,
                  busy = false,
                  operationStatus = "",
                  connection = ConnectionState(connected = true),
                  gitStatus = GitStatus(available = true, branch = "main"),
                  analysisStatus =
                      ToolbarAnalysisStatus(
                          "Analysis · Running", "Whole-project analysis · Running", true, false)),
              ToolbarActions({}, {}, {}, {}))
        }
        .use { fixture ->
          fixture.render("toolbar-flat-statuses")
          fixture.assertTextContrast("Analysis · Running", ActivityRail)
          fixture.assertTextContrast("Daemon connected", ActivityRail)
        }
  }
}
