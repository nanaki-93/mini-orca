package io.miniorca.desktop

import androidx.compose.material.Text
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotEquals
import kotlin.test.assertTrue

class DesktopThemeTest {
  @Test
  fun stateMessageWithoutActionKeepsItsTitleAndExplanation() {
    ComposeVisualFixture(320, 260, 1.5f) {
          SystemStateMessage("No project selected", "Open a project to inspect local data.")
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("No project selected"))
          assertTrue(fixture.hasText("Open a project to inspect local data."))
          assertTrue(
              fixture.firstVisibleTextBounds("No project selected").bottom <
                  fixture.firstVisibleTextBounds("Open a project to inspect local data.").top)
          assertFalse(fixture.hasText("Retry loading results"))
        }
  }

  @Test
  fun stateMessagePlacesCallerActionAfterExplanationWithoutDispatchingOnRender() {
    val title = "Saved results unavailable"
    val explanation =
        "Reading saved results for this project failed. Previous findings remain available."
    for (scale in listOf(1f, 1.25f, 1.5f)) {
      var retries = 0
      ComposeVisualFixture(340, 300, scale) {
            SystemStateMessage(
                title,
                explanation,
                action = {
                  MiniOrcaButton(onClick = { retries++ }) { Text("Retry loading results") }
                })
          }
          .use { fixture ->
            fixture.render()
            assertEquals(0, retries)
            assertTrue(fixture.hasText(title))
            assertTrue(fixture.hasText(explanation))
            assertTrue(fixture.hasText("Retry loading results"))
            assertTrue(
                fixture.firstVisibleTextBounds(title).bottom <
                    fixture.firstVisibleTextBounds(explanation).top)
            assertTrue(
                fixture.firstVisibleTextBounds(explanation).bottom <
                    fixture.firstVisibleTextBounds("Retry loading results").top)
            fixture.assertTextFits(title, maxLines = 3)
            fixture.assertTextFits(explanation, maxLines = 5)
            fixture.assertTextFits("Retry loading results")
            val actionBounds = fixture.firstVisibleTextBounds("Retry loading results")
            assertTrue(actionBounds.right <= 340 && actionBounds.bottom <= 300)
            assertTrue(fixture.requestFocus("Retry loading results"))
            fixture.render()
            assertTrue(fixture.isFocused("Retry loading results"))
            assertTrue(fixture.pressKey(Key.Enter))
            assertEquals(1, retries)
          }
    }
  }

  @Test
  fun semanticPaletteSeparatesIdentityInformationAndLabeledStatusOnResolvedSurfaces() {
    assertNotEquals(ActivityRail, ToolWindowSurface)
    assertNotEquals(ToolWindowSurface, EditorCanvas)
    assertNotEquals(EditorCanvas, OverlaySurface)
    val statusColors =
        listOf(
            statusBadgeStyle("fresh").color,
            statusBadgeStyle("stale").color,
            statusBadgeStyle("failed").color)
    assertEquals(3, statusColors.distinct().size)
    assertNotEquals(Information, MiniOrcaPalette.identityAccent)
    assertTrue(statusColors.none { it == Information || it == MiniOrcaPalette.identityAccent })
    listOf(ActivityRail, ToolWindowSurface, EditorCanvas, OverlaySurface).forEach { surface ->
      listOf(MiniOrcaPalette.identityAccent, Information, Success, Warning, Error).forEach { tint ->
        assertTrue(contrastRatio(tint, surface) >= 4.5, "$tint label on $surface")
      }
    }
  }

  @Test
  fun calibratedSurfacesFollowCanvasRailPanelRaisedAndSelectedHierarchy() {
    assertEquals(Color(0xFF1A1B26), EditorCanvas)
    assertEquals(Color(0xFF171821), ActivityRail)
    assertEquals(Color(0xFF202130), Panel)
    assertEquals(Color(0xFF292B3C), OverlaySurface)
    assertEquals(OverlaySurface, StrongSurface)
    assertEquals(Color(0xFF242A41), SelectionSurface)
    assertNotEquals(PaneSeparator, ControlBorder)
    assertTrue(contrastRatio(OverlaySurface, Panel) > 1.0)
    assertTrue(contrastRatio(ControlHover, StrongSurface) > 1.0)
  }

  @Test
  fun paneBoundariesAndControlOutlinesRemainVisibleOnTheirActualSurfaces() {
    listOf(ActivityRail, ToolWindowSurface, EditorCanvas, OverlaySurface).forEach { surface ->
      assertTrue(contrastRatio(PaneSeparator, surface) >= 1.4, "Pane boundary on $surface")
      assertTrue(contrastRatio(ControlBorder, surface) >= 3.0, "Control outline on $surface")
    }
    assertTrue(contrastRatio(ControlBorder, StrongSurface) >= 3.0)
    assertTrue(contrastRatio(ControlBorder, ControlHover) >= 3.0)
    // Disabled buttons use the same essential outline, not the decorative pane separator.
    assertTrue(
        contrastRatio(ControlBorder, actionToneStyle(ActionTone.Primary).disabledBackground) >= 3.0)
    assertTrue(contrastRatio(HeaderSurface, EditorCanvas) >= 1.3)
    // A header needs more separation than the literal raised token provides on the canvas.
    assertTrue(contrastRatio(OverlaySurface, EditorCanvas) < 1.3)
    listOf(PrimaryText, SecondaryText, FaintText).forEach { text ->
      listOf(
              ActivityRail,
              Panel,
              EditorCanvas,
              OverlaySurface,
              HeaderSurface,
              StrongSurface,
              ControlHover,
              SelectionSurface)
          .forEach { surface ->
            assertTrue(contrastRatio(text, surface) >= 4.5, "$text on $surface")
          }
    }
    assertTrue(contrastRatio(SelectionAccent, SelectionSurface) >= 3.0)
  }

  @Test
  fun semanticTypographyRolesKeepReadingAndTechnicalTextDistinctAtCompactSizes() {
    listOf(
            IdeTypography.body,
            IdeTypography.compactBody,
            IdeTypography.workspaceBody,
            IdeTypography.toolbarIdentity,
            IdeTypography.resultHeading,
            IdeTypography.section,
            IdeTypography.action)
        .forEach { assertEquals(FontFamily.SansSerif, it.fontFamily, "Reading/action role: $it") }
    listOf(
            IdeTypography.workspaceMetadata,
            IdeTypography.workspaceHeading,
            IdeTypography.resultLabel,
            IdeTypography.resultCode)
        .forEach { assertEquals(FontFamily.Monospace, it.fontFamily, "Technical role: $it") }
    assertEquals(13.sp, IdeTypography.body.fontSize)
    assertEquals(20.sp, IdeTypography.body.lineHeight)
    assertEquals(12.sp, IdeTypography.compactBody.fontSize)
    assertEquals(18.sp, IdeTypography.compactBody.lineHeight)
    assertEquals(14.sp, IdeTypography.workspaceBody.fontSize)
    assertEquals(22.sp, IdeTypography.workspaceBody.lineHeight)
    assertEquals(13.sp, IdeTypography.workspaceMetadata.fontSize)
    assertEquals(20.sp, IdeTypography.workspaceMetadata.lineHeight)
    assertEquals(16.sp, IdeTypography.workspaceHeading.fontSize)
    assertEquals(22.sp, IdeTypography.workspaceHeading.lineHeight)
    assertEquals(12.sp, IdeTypography.section.fontSize)
    assertEquals(18.sp, IdeTypography.section.lineHeight)
    assertEquals(12.sp, IdeTypography.action.fontSize)
    assertEquals(16.sp, IdeTypography.action.lineHeight)
  }

  @Test
  fun resultRolesKeepHeadingsLabelsCodeAndBadgeTextReadable() {
    assertEquals(IdeTypography.body.fontSize, IdeTypography.resultHeading.fontSize)
    assertEquals(IdeTypography.body.lineHeight, IdeTypography.resultHeading.lineHeight)
    assertEquals(
        androidx.compose.ui.text.font.FontWeight.SemiBold, IdeTypography.resultHeading.fontWeight)
    assertEquals(
        androidx.compose.ui.text.font.FontWeight.SemiBold, IdeTypography.resultLabel.fontWeight)
    assertEquals(IdeTypography.compactBody.fontSize, IdeTypography.resultLabel.fontSize)
    assertEquals(FontFamily.Monospace, IdeTypography.resultCode.fontFamily)
    assertEquals(IdeTypography.compactBody.lineHeight, IdeTypography.resultCode.lineHeight)
    listOf(Panel, OverlaySurface, EditorCanvas, SelectionSurface).forEach { surface ->
      listOf(PrimaryText, ResultAccent, SecondaryText).forEach { tint ->
        assertTrue(contrastRatio(tint, surface) >= 4.5, "Result label $tint on $surface")
      }
      listOf(ResultAccent, Success, Warning, Error, SecondaryText).forEach { tint ->
        assertEquals(1f, labelBadgeBackground(tint).alpha)
        assertTrue(
            contrastRatio(tint, blendOver(labelBadgeBackground(tint), surface)) >= 4.5,
            "Badge label $tint on $surface")
      }
    }
    assertTrue(contrastRatio(SelectionText, SelectionSurface) >= 4.5)
  }

  @Test
  fun statusStylesKeepTheStateInTextAsWellAsColor() {
    assertEquals(StatusBadgeStyle("Fresh", Success), statusBadgeStyle("fresh"))
    assertEquals(StatusBadgeStyle("Stale", Warning), statusBadgeStyle("stale"))
    assertEquals(StatusBadgeStyle("Running", Information), statusBadgeStyle("running"))
    assertEquals(StatusBadgeStyle("Failed", Error), statusBadgeStyle("failed"))
    assertEquals(StatusBadgeStyle("Not analyzed", SecondaryText), statusBadgeStyle("missing"))
  }

  @Test
  fun actionTonesUseSharedPaletteTokensForEverySemanticRole() {
    assertEquals(ActionFill, actionToneStyle(ActionTone.Primary).background)
    assertEquals(SelectionText, actionToneStyle(ActionTone.Navigation).content)
    assertEquals(SelectionSurface, actionToneStyle(ActionTone.Navigation).selectedBackground)
    assertEquals(Success, actionToneStyle(ActionTone.Positive).content)
    assertEquals(Warning, actionToneStyle(ActionTone.Attention).content)
    assertEquals(Error, actionToneStyle(ActionTone.Destructive).content)
    assertEquals(PrimaryText, actionToneStyle(ActionTone.Neutral).content)
    ActionTone.entries.forEach { tone ->
      val style = actionToneStyle(tone)
      assertNotEquals(style.background, style.disabledBackground)
      assertNotEquals(style.content, style.disabledContent)
      assertNotEquals(style.background, style.pressedBackground)
      assertNotEquals(style.background, style.selectedBackground)
    }
  }

  @Test
  fun positivePrimaryActionUsesOpaqueFillAndReadableTextInEveryState() {
    val style = actionToneStyle(ActionTone.PositivePrimary)
    listOf(style.background, style.pressedBackground).forEach { fill ->
      assertEquals(1f, fill.alpha)
      assertTrue(contrastRatio(style.content, fill) >= 4.5)
      assertTrue(contrastRatio(ActivityRail, fill) >= 3.0)
    }
    assertEquals(1f, style.selectedBackground.alpha)
    assertTrue(contrastRatio(style.selectedContent, style.selectedBackground) >= 4.5)
    assertTrue(contrastRatio(SelectionAccent, style.selectedBackground) >= 3.0)
    assertTrue(contrastRatio(FocusAccent, style.selectedBackground) >= 3.0)
    assertTrue(contrastRatio(FocusAccent, ActivityRail) >= 3.0)
    assertEquals(Success, actionToneStyle(ActionTone.Positive).content)
  }

  @Test
  fun compactButtonDensitiesStayWithinTheApprovedControlScale() {
    assertEquals(32.dp, buttonDensityStyle(ButtonDensity.Standard).height)
    assertEquals(32.dp, buttonDensityStyle(ButtonDensity.Toolbar).height)
    assertEquals(
        10.dp,
        buttonDensityStyle(ButtonDensity.Standard)
            .contentPadding
            .calculateLeftPadding(androidx.compose.ui.unit.LayoutDirection.Ltr))
    assertEquals(
        8.dp,
        buttonDensityStyle(ButtonDensity.Toolbar)
            .contentPadding
            .calculateLeftPadding(androidx.compose.ui.unit.LayoutDirection.Ltr))
  }

  @Test
  fun fieldSupportColorsRemainReadableOnTheirSurfaces() {
    assertTrue(contrastRatio(SecondaryText, Panel) >= 4.5)
    assertTrue(contrastRatio(Error, Panel) >= 4.5)
    assertTrue(contrastRatio(Error, EditorCanvas) >= 3.0)
    assertTrue(contrastRatio(FocusAccent, EditorCanvas) >= 3.0)
  }

  @Test
  fun keyboardFocusRemainsDistinctFromSelection() {
    assertNotEquals(SelectionSurface, FocusAccent)
    assertNotEquals(SelectionAccent, FocusAccent)
  }

  @Test
  fun semanticSyntaxTokensDoNotChangeSourceText() {
    val source = "package demo\n// note\nfunc Run() string { return \"ok\" }\n"
    val highlighted = highlightedCode(source)

    assertEquals(source, highlighted.text)
    assertTrue(highlighted.spanStyles.any { it.item.color == CodeComment })
    assertTrue(highlighted.spanStyles.any { it.item.color == CodeString })
    assertTrue(highlighted.spanStyles.any { it.item.color == CodeKeyword })
  }

  @Test
  fun essentialTextActionAndFocusColorsMeetTheIdeContrastTargetsOnResolvedSurfaces() {
    listOf(Panel, OverlaySurface, SelectionSurface).forEach { surface ->
      assertTrue(contrastRatio(Information, surface) >= 4.5)
      assertTrue(contrastRatio(SecondaryText, surface) >= 4.5)
    }
    listOf(PrimaryText, SecondaryText, FaintText).forEach { text ->
      assertTrue(contrastRatio(text, AppBackground) >= 4.5, "Chrome labels on the frame")
    }
    listOf(FocusAccent, SelectionAccent, ControlBorder).forEach { indicator ->
      assertTrue(contrastRatio(indicator, AppBackground) >= 3.0, "Splitter and focus on the frame")
    }
    assertTrue(contrastRatio(SecondaryText, Panel) >= 4.5)
    assertTrue(contrastRatio(PrimaryText, EditorCanvas) >= 4.5)
    assertTrue(contrastRatio(PrimaryText, OverlaySurface) >= 4.5)
    assertTrue(contrastRatio(OnActionFill, ActionFill) >= 4.5)
    assertTrue(contrastRatio(SelectionText, SelectionSurface) >= 4.5)
    assertTrue(contrastRatio(FocusAccent, Panel) >= 3.0)
    assertTrue(contrastRatio(FocusAccent, OverlaySurface) >= 3.0)
    assertTrue(contrastRatio(FocusAccent, SelectionSurface) >= 3.0)
    assertTrue(
        contrastRatio(
            actionToneStyle(ActionTone.Primary).disabledContent,
            blendOver(actionToneStyle(ActionTone.Primary).disabledBackground, Panel)) >= 4.5)
    assertTrue(contrastRatio(Success, DiffAddedBackground) >= 4.5)
    assertTrue(contrastRatio(Error, DiffRemovedBackground) >= 4.5)
  }
}
