package io.miniorca.desktop

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.material.Text
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.state.ToggleableState
import androidx.compose.ui.unit.dp
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class DesktopAnalysisAdmissionTest {
  @Test
  fun shortAdmissionDialogScrollsLongDestinationsWithoutImplicitConsentOrStart() {
    val origin = "https://example.test/" + "destination/".repeat(14) + "end"
    val preview = analysisPreviewFixture()
    val longPreview =
        preview.copy(
            providers =
                preview.providers.mapIndexed { index, provider ->
                  if (index == 0)
                      provider.copy(model = provider.model.copy(providerOrigin = origin))
                  else provider
                })
    var state by mutableStateOf(ProjectAnalysisRunState(admission = AnalysisAdmission(longPreview)))
    var starts = 0
    var closes = 0
    ComposeVisualFixture(360, 400, 1.5f) {
          Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            IdeDialogSurface(
                maxHeight = 340.dp,
                title = { Text("Analyze whole project") },
                content = {
                  DesktopAnalysisAdmissionContent(
                      state,
                      { id, checked ->
                        state =
                            state.copy(
                                admission =
                                    state.admission!!.copy(
                                        providerIds =
                                            if (checked) state.admission!!.providerIds + id
                                            else state.admission!!.providerIds - id))
                      },
                      { checked ->
                        state =
                            state.copy(admission = state.admission!!.copy(securityReview = checked))
                      })
                },
                actions = {
                  MiniOrcaButton(onClick = { closes++ }, tone = ActionTone.Neutral) {
                    Text("Close")
                  }
                  MiniOrcaButton(
                      onClick = { starts++ },
                      enabled = state.admission!!.isConfirmed(),
                      tone = ActionTone.Primary) {
                        Text("Start analysis")
                      }
                },
                focusSafeActionOnOpen = true)
          }
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.isFocusedControl("Close"))
          assertTrue(fixture.isDisabled("Start analysis"))
          assertEquals(0, starts + closes)
          fixture.resize(360, 360)
          fixture.render()
          assertEquals(0, starts + closes)
          assertFalse(state.admission!!.isConfirmed())
          assertTrue(fixture.isFocusedControl("Close"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertEquals(1, closes)
          assertEquals(0, starts)
          assertFalse(state.admission!!.isConfirmed())
          val body = fixture.taggedBounds("ide-dialog-body")
          assertTrue(body.height > 0 && body.bottom <= fixture.firstVisibleTextBounds("Close").top)
          val destination = "Remote destination: $origin"
          fixture.assertTextWrapsWithoutClipping(destination)
          fixture.scrollBy(180f, "ide-dialog-body")
          fixture.render()
          assertTrue(fixture.verticalScrollValue("ide-dialog-body") > 0f)
          for (label in
              listOf(
                  "Confirm bug destination",
                  "Confirm analyze destination",
                  "Include AI Security review")) {
            fixture.revealText(label, "ide-dialog-body")
            val bounds = fixture.descriptionBounds(label)
            assertTrue(bounds.top >= body.top && bounds.bottom <= body.bottom, "$label: $bounds")
            assertTrue(fixture.requestDescriptionFocus(label), label)
            fixture.render()
            assertTrue(fixture.isDescriptionFocused(label), label)
            assertTrue(fixture.pressKey(Key.Spacebar), label)
            fixture.render()
          }
          assertTrue(state.admission!!.isConfirmed())
          assertEquals(0, starts)
          assertEquals(1, closes)
          assertTrue(fixture.requestFocus("Start analysis"))
          fixture.render()
          assertTrue(fixture.isFocusedControl("Start analysis"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertEquals(1, starts)
          assertEquals(1, closes)
        }
  }

  @Test
  fun emptyRetryPreviewExplainsThatNoFilesNeedAnalysis() {
    val preview =
        analysisPreviewFixture()
            .copy(
                retryStaleFailed = true,
                files = emptyList(),
                expectedModelRequests = 0,
                maxModelRequests = 0)
    ComposeVisualFixture(360, 1600, 1.5f) {
          DesktopAnalysisAdmissionContent(
              ProjectAnalysisRunState(admission = AnalysisAdmission(preview)),
              { _, _ -> },
              {},
              Modifier.fillMaxWidth())
        }
        .use { fixture ->
          fixture.render("analysis-empty-retry-preview")
          fixture.assertTextFits("No stale or failed files to analyze.")
          assertFalse(fixture.hasDescription("Include AI Security review"))
          assertFalse(fixture.hasDescription("Confirm bug destination"))
        }
  }

  @Test
  fun productionPreviewNamesAllScopesAndHasSeparateKeyboardConsent() {
    for ((width, scale) in listOf(360 to 1.5f, 640 to 1f)) {
      var state by
          mutableStateOf(
              ProjectAnalysisRunState(admission = AnalysisAdmission(analysisPreviewFixture())))
      val providerChanges = mutableListOf<Pair<String, Boolean>>()
      val securityChanges = mutableListOf<Boolean>()
      ComposeVisualFixture(width, 1600, scale) {
            DesktopAnalysisAdmissionContent(
                state,
                { id, checked ->
                  providerChanges += id to checked
                  state =
                      state.copy(
                          admission =
                              state.admission!!.copy(
                                  providerIds =
                                      if (checked) state.admission!!.providerIds + id
                                      else state.admission!!.providerIds - id))
                },
                { checked ->
                  securityChanges += checked
                  state = state.copy(admission = state.admission!!.copy(securityReview = checked))
                },
                Modifier.fillMaxWidth())
          }
          .use { fixture ->
            fixture.render("analysis-admission-$width-$scale")
            for (label in
                listOf(
                    "Code analysis",
                    "Performance review · AI Security review",
                    "Remote destination: https://bug.example",
                    "Remote destination: https://analyze.example")) assertTrue(
                fixture.hasText(label))
            assertFalse(state.admission!!.isConfirmed())
            for (label in
                listOf(
                    "Confirm bug destination",
                    "Confirm analyze destination",
                    "Include AI Security review")) {
              assertEquals(1, fixture.clickableDescriptionCount(label))
              assertEquals(ToggleableState.Off, fixture.descriptionToggleableState(label))
              assertEquals("Not confirmed", fixture.descriptionStateDescription(label))
            }
            assertTrue(fixture.requestDescriptionFocus("Confirm bug destination"))
            assertTrue(fixture.pressKey(Key.Enter))
            fixture.render()
            assertEquals(listOf("bug-provider" to true), providerChanges)
            assertEquals(setOf("bug-provider"), state.admission!!.providerIds)
            assertFalse(state.admission!!.securityReview)
            assertEquals(
                ToggleableState.On, fixture.descriptionToggleableState("Confirm bug destination"))
            assertEquals(
                "Confirmed", fixture.descriptionStateDescription("Confirm bug destination"))
            assertEquals(
                ToggleableState.Off,
                fixture.descriptionToggleableState("Confirm analyze destination"))
            fixture.clickVisibleDescription("Confirm analyze destination")
            fixture.render()
            assertEquals(
                listOf("bug-provider" to true, "analyze-provider" to true), providerChanges)
            assertFalse(state.admission!!.isConfirmed(), "Security consent still blocks Start")
            assertTrue(securityChanges.isEmpty())
            fixture.clickVisibleDescription("Include AI Security review")
            fixture.render()
            assertEquals(listOf(true), securityChanges)
            assertEquals(setOf("bug-provider", "analyze-provider"), state.admission!!.providerIds)
            assertTrue(state.admission!!.isConfirmed())
            assertEquals(
                ToggleableState.On,
                fixture.descriptionToggleableState("Include AI Security review"))
            assertTrue(fixture.requestDescriptionFocus("Include AI Security review"))
            assertTrue(fixture.pressKey(Key.Spacebar))
            fixture.render()
            assertEquals(listOf(true, false), securityChanges)
            assertFalse(state.admission!!.isConfirmed())
            assertEquals(
                listOf("bug-provider" to true, "analyze-provider" to true), providerChanges)
            fixture.assertTextFits("Include AI Security review")
          }
    }
  }

  @Test
  fun loadingAndFailureRemainExplicitWithoutAnAdmission() {
    ComposeVisualFixture(360, 400, 1.5f) {
          DesktopAnalysisAdmissionContent(
              ProjectAnalysisRunState(
                  action = "preview", error = "Provider scope changed. Request a fresh preview."),
              { _, _ -> },
              {})
        }
        .use { fixture ->
          fixture.render("analysis-admission-loading-error")
          assertTrue(fixture.hasText("Preparing project scope and provider estimates…"))
          assertTrue(fixture.hasText("Provider scope changed. Request a fresh preview."))
          assertFalse(fixture.hasDescription("Include AI Security review"))
        }
  }
}
