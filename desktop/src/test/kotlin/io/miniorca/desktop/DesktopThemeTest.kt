package io.miniorca.desktop

import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotEquals
import kotlin.test.assertTrue

class DesktopThemeTest {
  @Test
  fun paneBoundariesAndControlOutlinesRemainVisibleOnTheirActualSurfaces() {
    listOf(ActivityRail, ToolWindowSurface, EditorCanvas, OverlaySurface).forEach { surface ->
      assertTrue(contrastRatio(PaneSeparator, surface) >= 1.4, "Pane boundary on $surface")
      assertTrue(contrastRatio(ControlBorder, surface) >= 3.0, "Control outline on $surface")
    }
    assertTrue(contrastRatio(ControlBorder, StrongSurface) >= 3.0)
    assertTrue(contrastRatio(ControlBorder, ControlHover) >= 3.0)
    assertTrue(contrastRatio(HeaderSurface, EditorCanvas) >= 1.3)
    assertTrue(contrastRatio(SelectionAccent, SelectionSurface) >= 3.0)
  }

  @Test
  fun semanticTypographyRolesUseDenseBodyAndSectionMetrics() {
    assertEquals(13.sp, IdeTypography.body.fontSize)
    assertEquals(20.sp, IdeTypography.body.lineHeight)
    assertEquals(12.sp, IdeTypography.compactBody.fontSize)
    assertEquals(18.sp, IdeTypography.compactBody.lineHeight)
    assertEquals(12.sp, IdeTypography.section.fontSize)
    assertEquals(18.sp, IdeTypography.section.lineHeight)
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
    assertEquals(
        androidx.compose.ui.text.font.FontFamily.Monospace, IdeTypography.resultCode.fontFamily)
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
    listOf(style.background, style.pressedBackground, style.selectedBackground).forEach { fill ->
      assertEquals(1f, fill.alpha)
      assertTrue(contrastRatio(style.content, fill) >= 4.5)
      assertTrue(contrastRatio(ActivityRail, fill) >= 3.0)
    }
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
  fun keyboardFocusRemainsDistinctFromSelection() {
    assertNotEquals(SelectionSurface, FocusAccent)
    assertNotEquals(SelectionAccent, FocusAccent)
  }

  @Test
  fun actionGroupsSwitchToVerticalBeforeTheyBecomeCrowded() {
    assertEquals(ActionGroupLayout.Vertical, actionGroupLayout(459f))
    assertEquals(ActionGroupLayout.Horizontal, actionGroupLayout(460f))
    assertEquals(
        ActionGroupLayout.Vertical, actionGroupLayout(419f, minimumHorizontalWidthDp = 420f))
    assertEquals(
        ActionGroupLayout.Horizontal, actionGroupLayout(420f, minimumHorizontalWidthDp = 420f))
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
