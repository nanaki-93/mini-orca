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
          ComposeVisualFixture(800, 650, 1.5f) {
                AnalysisWorkspacePane(
                    AnalysisWorkspacePaneState(
                        resultProjectFixture(),
                        ProjectAnalysisRunState(run = analysisRunFixture().copy(status = status))),
                    AnalysisWorkspaceActions({ _, _ -> }, {}, {}, {}, {}))
              }
              .use { fixture ->
                fixture.render("analysis-state-color-$status")
                fixture.assertTextFits(analysisStatusLabel(status))
                fixture.assertTextContrast(analysisStatusLabel(status), HeaderSurface)
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
        listOf(style.background, style.pressedBackground, style.selectedBackground).forEach { fill
          ->
          assertTrue(
              contrastRatio(style.content, blendOver(fill, host)) >= 4.5,
              "$tone label must remain readable on $fill over $host")
        }
        assertTrue(
            contrastRatio(style.disabledContent, blendOver(style.disabledBackground, host)) >= 4.5)
      }
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
}
