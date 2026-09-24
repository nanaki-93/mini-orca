package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material.Text
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.state.ToggleableState
import androidx.compose.ui.unit.dp
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class ChromeControlsTest {
  @Test
  fun sharedActionSurfaceUsesOnePredictableStatePriority() {
    val colors =
        IdeActionColors(
            background = Color.Black,
            hoveredBackground = Color.DarkGray,
            pressedBackground = Color.Gray,
            selectedBackground = Color.Blue,
            disabledBackground = Color.LightGray,
            content = Color.White,
            selectedContent = Color.White,
            disabledContent = Color.Gray,
            border = Color.Transparent,
        )

    assertEquals(Color.Black, ideActionBackground(colors, true, false, IdeActionInteraction()))
    assertEquals(
        Color.DarkGray,
        ideActionBackground(colors, true, false, IdeActionInteraction(hovered = true)))
    assertEquals(
        Color.Gray,
        ideActionBackground(
            colors, true, false, IdeActionInteraction(hovered = true, pressed = true)))
    assertEquals(
        Color.Blue, ideActionBackground(colors, true, true, IdeActionInteraction(pressed = true)))
    assertEquals(
        Color.LightGray,
        ideActionBackground(colors, false, true, IdeActionInteraction(pressed = true)))
  }

  @Test
  fun selectedTabKeepsItsSelectionWhenKeyboardFocusMovesAndActivatesWithEnter() {
    var activations = 0
    ComposeVisualFixture(360, 140, 1.5f) {
          Column(Modifier.fillMaxSize().background(Panel).padding(12.dp)) {
            ChromeTab(
                onClick = { activations++ }, selected = true, accessibleName = "Results tab") {
                  Text("Results tab")
                }
          }
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasDescription("Results tab"))
          assertTrue(fixture.isDescriptionSelected("Results tab"))
          fixture.assertColorVisible(SelectionAccent)
          assertTrue(fixture.requestDescriptionFocus("Results tab"))
          fixture.render("focused-selected-tab")
          assertTrue(fixture.isDescriptionFocused("Results tab"))
          assertTrue(fixture.isDescriptionSelected("Results tab"))
          fixture.assertColorVisible(FocusAccent)
          fixture.assertColorVisible(SelectionAccent)
          fixture.assertTextFits("Results tab")
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertEquals(1, activations)
          assertTrue(fixture.isDescriptionSelected("Results tab"))
        }
  }

  @Test
  fun focusedSelectedTabTooltipDoesNotCoverItsSelectionAtDoubleDensity() {
    ComposeVisualFixture(720, 280, 1.5f, 2f) {
          Column(Modifier.fillMaxSize().background(Panel).padding(12.dp)) {
            ChromeTab(onClick = {}, selected = true, accessibleName = "Results tab") {
              Text("Results tab")
            }
          }
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestDescriptionFocus("Results tab"))
          fixture.render()
          assertTrue(fixture.isDescriptionSelected("Results tab"))
          fixture.assertColorVisible(FocusAccent)
          fixture.assertColorVisible(SelectionAccent)
        }
  }

  @Test
  fun sharedActionsDispatchOncePerPointerEnterAndSpaceGesture() {
    var chromeClicks = 0
    var primaryClicks = 0
    ComposeVisualFixture(440, 180, 1.5f) {
          Column(Modifier.fillMaxSize().background(Panel).padding(12.dp)) {
            ChromeButton(
                onClick = { chromeClicks++ }, accessibleName = "Open actions", tooltip = null) {
                  Text("Open actions")
                }
            MiniOrcaButton(onClick = { primaryClicks++ }, tone = ActionTone.Primary) {
              Text("Review candidate")
            }
          }
        }
        .use { fixture ->
          fixture.render()
          fixture.clickVisibleDescription("Open actions")
          assertEquals(1, chromeClicks)
          assertTrue(fixture.requestDescriptionFocus("Open actions"))
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(2, chromeClicks)
          assertTrue(fixture.pressKey(Key.Spacebar))
          assertEquals(3, chromeClicks)
          assertTrue(fixture.tryClick("Review candidate"))
          assertEquals(1, primaryClicks)
          assertTrue(fixture.requestFocus("Review candidate"))
          assertTrue(fixture.pressKey(Key.Enter))
          assertEquals(2, primaryClicks)
          assertTrue(fixture.pressKey(Key.Spacebar))
          assertEquals(3, primaryClicks)
        }
  }

  @Test
  fun disabledButtonsAndMenuItemsRetainNamesAndCannotActivate() {
    var activations = 0
    ComposeVisualFixture(440, 220, 1.5f) {
          Column(Modifier.fillMaxSize().background(Panel).padding(12.dp)) {
            ChromeButton(
                onClick = { activations++ }, enabled = false, accessibleName = "Unavailable") {
                  Text("Unavailable")
                }
            MiniOrcaButton(onClick = { activations++ }, enabled = false) { Text("Blocked action") }
            IdePopupMenuSurface(
                content = {
                  IdeDropdownMenuItem("Disabled menu action", { activations++ }, enabled = false)
                })
          }
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasDescription("Unavailable"))
          assertTrue(fixture.isDescriptionDisabled("Unavailable"))
          listOf("Unavailable", "Blocked action", "Disabled menu action").forEach { label ->
            fixture.assertTextFits(label)
            assertTrue(fixture.isDisabled(label), "$label must expose disabled semantics")
            assertTrue(!fixture.tryClick(label), "$label must not expose an enabled click")
            assertTrue(!fixture.requestFocus(label), "$label must not be keyboard focusable")
          }
          fixture.pressKey(Key.Enter)
          fixture.pressKey(Key.Spacebar)
          assertEquals(0, activations)
        }
  }

  @Test
  fun disclosureRetainsStateLabelAndTogglesWithoutInvokingTrailingAction() {
    var expanded by mutableStateOf(false)
    var trailingClicks = 0
    ComposeVisualFixture(480, 140, 1.5f) {
          IdeDisclosureHeader(
              title = "Evidence",
              expanded = expanded,
              onToggle = { expanded = !expanded },
              stateLabel = "Partial results",
              actions = {
                ChromeButton(onClick = { trailingClicks++ }, accessibleName = "Other action") {
                  Text("Other action")
                }
              })
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasDescription("Expand Evidence"))
          assertEquals("Collapsed", fixture.descriptionStateDescription("Expand Evidence"))
          fixture.assertTextFits("Partial results")
          assertTrue(fixture.requestDescriptionFocus("Expand Evidence"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertTrue(fixture.hasDescription("Collapse Evidence"))
          assertEquals("Expanded", fixture.descriptionStateDescription("Collapse Evidence"))
          assertEquals(0, trailingClicks)
        }
  }

  @Test
  fun checkboxHasOneLabeledTargetAndTogglesOncePerPointerAndKeyboardGesture() {
    var checked by mutableStateOf(false)
    val changes = mutableListOf<Boolean>()
    ComposeVisualFixture(400, 140, 1.5f) {
          Column(Modifier.fillMaxSize().background(Panel).padding(12.dp)) {
            IdeCheckbox(
                checked = checked,
                accessibleName = "Include security review",
                label = "Include security review",
                stateLabel = if (checked) "Confirmed" else "Not confirmed",
                onCheckedChange = {
                  checked = it
                  changes += it
                })
          }
        }
        .use { fixture ->
          fixture.render()
          assertEquals(1, fixture.clickableDescriptionCount("Include security review"))
          assertEquals(
              ToggleableState.Off, fixture.descriptionToggleableState("Include security review"))
          assertEquals(
              "Not confirmed", fixture.descriptionStateDescription("Include security review"))
          fixture.clickVisibleDescription("Include security review")
          assertEquals(listOf(true), changes)
          assertEquals(
              ToggleableState.On, fixture.descriptionToggleableState("Include security review"))
          assertEquals("Confirmed", fixture.descriptionStateDescription("Include security review"))
          assertTrue(fixture.tryClick("Include security review"))
          fixture.render()
          assertEquals(listOf(true, false), changes)
          assertTrue(fixture.requestDescriptionFocus("Include security review"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertEquals(listOf(true, false, true), changes)
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertEquals(listOf(true, false, true, false), changes)
          assertEquals(
              ToggleableState.Off, fixture.descriptionToggleableState("Include security review"))
        }
  }

  @Test
  fun disabledCheckboxRetainsStateAndNameWithoutAnActivationTarget() {
    var changes = 0
    ComposeVisualFixture(400, 140, 1.5f) {
          Column(Modifier.fillMaxSize().background(Panel).padding(12.dp)) {
            IdeCheckbox(
                checked = true,
                accessibleName = "Locked selection",
                label = "Locked selection",
                stateLabel = "Selected for analysis",
                enabled = false,
                onCheckedChange = { changes++ })
          }
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasDescription("Locked selection"))
          assertTrue(fixture.isDescriptionDisabled("Locked selection"))
          assertEquals(ToggleableState.On, fixture.descriptionToggleableState("Locked selection"))
          assertEquals(
              "Selected for analysis", fixture.descriptionStateDescription("Locked selection"))
          assertTrue(!fixture.tryClick("Locked selection"))
          assertTrue(!fixture.requestDescriptionFocus("Locked selection"))
          fixture.pressKey(Key.Enter)
          fixture.pressKey(Key.Spacebar)
          assertEquals(0, changes)
        }
  }

  @Test
  fun evidenceRowDisplaysCallerSuppliedStatusAndDetailWithoutAnAction() {
    ComposeVisualFixture(360, 200, 1.5f) {
          Column(Modifier.fillMaxSize().background(Panel).padding(12.dp)) {
            IdeEvidenceRow(
                label = "Source identity",
                statusText = "Unavailable · —",
                statusTint = Warning,
                detail = "The candidate does not match the open file. Reopen the draft.") {
                  Text("!", color = Warning)
                }
          }
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Unavailable · —"))
          assertTrue(
              fixture.hasText("The candidate does not match the open file. Reopen the draft."))
          val description =
              "Source identity: The candidate does not match the open file. Reopen the draft."
          assertTrue(fixture.hasDescription(description))
          assertEquals("Unavailable · —", fixture.descriptionStateDescription(description))
          assertTrue(!fixture.tryClick("Source identity"))
          assertTrue(!fixture.requestDescriptionFocus(description))
          fixture.assertTextFits("Unavailable · —")
        }
  }

  @Test
  fun sharedChromeUsesSelectionEdgeWithoutReplacingFocusAndRetainsOverlayContrast() {
    assertTrue(SelectionAccent != FocusAccent)
    assertEquals(OverlaySurface, Card)
    assertEquals(PaneSeparator, Border)
    assertTrue(contrastRatio(PrimaryText, OverlaySurface) >= 4.5)
    assertTrue(contrastRatio(SecondaryText, OverlaySurface) >= 4.5)
    assertTrue(contrastRatio(SelectionAccent, ToolWindowSurface) >= 3.0)
  }
}
