package io.miniorca.desktop

import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.input.key.Key
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
      ComposeVisualFixture(width, 1600, scale) {
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
            assertTrue(fixture.requestFocus("Confirm bug destination"))
            fixture.pressKey(Key.Enter)
            fixture.render()
            assertEquals("Confirmed", fixture.stateDescription("Confirm bug destination"))
            fixture.clickDescription("Confirm analyze destination")
            fixture.render()
            assertFalse(state.admission!!.isConfirmed())
            fixture.clickDescription("Include AI Security review")
            fixture.render()
            assertTrue(state.admission!!.isConfirmed())
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
