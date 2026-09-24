package io.miniorca.desktop

import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.state.ToggleableState
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class DesktopAnalysisAdmissionTest {
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
