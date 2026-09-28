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
  fun returnedScopeAndPolicyFollowPreviewRatherThanOriginalRunPlan() {
    val base = analysisPreviewFixture()
    val cases =
        listOf(
            Triple(
                base.copy(
                    refresh = true,
                    excluded = listOf(AnalysisExcludedFile("skip.go", "Policy excluded"))),
                null,
                "Full project scope: 1 included file · 1 excluded"),
            Triple(
                base.copy(retryStaleFailed = true, refresh = false),
                null,
                "Stale & failed scope: 1 included file · 0 excluded"),
            Triple(
                base.copy(
                    files =
                        listOf(
                            AnalysisPlannedFile("one.go", "a", "Go", 1, emptyList()),
                            AnalysisPlannedFile("two.go", "b", "Go", 1, emptyList())),
                    expectedModelRequests = 0,
                    maxModelRequests = 0,
                    compatibilityStage = "security_rules"),
                analysisRunFixture().identity,
                "Continuation (run): 2 files in the admitted file set · 0 excluded"))
    for ((preview, resumeRun, scope) in cases) {
      val state = ProjectAnalysisRunState(admission = AnalysisAdmission(preview, resumeRun))
      ComposeVisualFixture(600, 1600) { DesktopAnalysisAdmissionContent(state, { _, _ -> }, {}) }
          .use { fixture ->
            fixture.render()
            assertTrue(fixture.hasText(scope), scope)
            assertTrue(fixture.hasText("Reviewing this preview sends nothing to a model."))
            assertTrue(
                fixture.hasText(
                    if (preview.refresh)
                        "Refresh policy: request fresh evidence for eligible stages."
                    else "Reuse policy: refresh is off; eligible existing evidence may be reused."))
            if (preview.retryStaleFailed)
                assertTrue(
                    fixture.hasText(
                        "Selective retry: the daemon returned the stale & failed scope."))
            if (resumeRun != null) {
              assertTrue(fixture.hasText("This saved run covers only Security rules."))
              assertTrue(fixture.hasText("Expected model requests: 0 · Maximum: 0"))
              assertFalse(fixture.hasText("Expected model requests: 3 · Maximum: 6"))
            }
          }
    }
  }

  @Test
  fun emptyRetryPreviewExplainsThatNoFilesNeedAnalysis() {
    val preview =
        analysisPreviewFixture()
            .copy(
                retryStaleFailed = true,
                files = emptyList(),
                excluded =
                    listOf(
                        AnalysisExcludedFile(
                            "excluded.go", "Policy: " + "detail ".repeat(70) + "end")),
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
          assertTrue(fixture.hasText("excluded.go"))
          assertTrue(fixture.hasText(preview.excluded.single().reason))
          assertTrue(fixture.hasDescription("Include AI Security review"))
          assertTrue(fixture.hasDescription("Confirm bug destination"))
        }
  }

  @Test
  fun emptyFullScopeKeepsExclusionsAccessibleButCannotStart() {
    val reason = "Source not eligible: " + "long explanation ".repeat(30) + "end"
    val preview =
        analysisPreviewFixture()
            .copy(
                files = emptyList(),
                excluded = listOf(AnalysisExcludedFile("src/private/skip.go", reason)),
                expectedModelRequests = 0,
                maxModelRequests = 0)
    val state = ProjectAnalysisRunState(admission = AnalysisAdmission(preview))
    ComposeVisualFixture(400, 500, 1.5f) {
          Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            IdeDialogSurface(
                400.dp,
                title = { DesktopAnalysisAdmissionTitle(state) },
                content = { DesktopAnalysisAdmissionContent(state, { _, _ -> }, {}) },
                actions = { DesktopAnalysisAdmissionActions(state, {}, {}, {}) })
          }
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("No eligible files to analyze."))
          assertTrue(fixture.isDisabled("Start analysis"))
          fixture.revealText("src/private/skip.go", "ide-dialog-body")
          assertTrue(fixture.hasText(reason))
          assertTrue(fixture.copyTextByDragging(reason).isNotBlank())
          assertTrue(fixture.hasText("Close"))
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
  fun loadingAndFailureNameTheCapturedModeWithoutReusingPreviewTotals() {
    val run = analysisRunFixture()
    val cases =
        listOf(
            Triple(
                "full project",
                "Analyze whole project",
                "Preparing full project analysis preview…"),
            Triple(
                "stale & failed",
                "Analyze stale & failed",
                "Preparing stale & failed analysis preview…"),
            Triple(
                "continuation",
                "Continue project analysis",
                "Preparing continuation preview for this analysis run…"))
    for ((mode, title, preparing) in cases) {
      val intent =
          AnalysisPreviewIntent(
              "project",
              "revision",
              AnalysisRunLimits(7, 120, 3),
              mode != "stale & failed",
              mode == "stale & failed",
              if (mode == "continuation") run.identity else null,
              if (mode == "continuation") run.plan else null)
      val diagnostic =
          "Transport failure: " + "connection detail ".repeat(300) + "end of diagnostic"
      var state by
          mutableStateOf(
              ProjectAnalysisRunState(run = run, previewIntent = intent, action = "preview"))
      var retries = 0
      var closes = 0
      var starts = 0
      ComposeVisualFixture(420, 450, 1.5f) {
            Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
              IdeDialogSurface(
                  360.dp,
                  title = { DesktopAnalysisAdmissionTitle(state) },
                  content = { DesktopAnalysisAdmissionContent(state, { _, _ -> }, {}) },
                  actions = {
                    DesktopAnalysisAdmissionActions(
                        state,
                        close = {
                          closes++
                          state = state.copy(previewIntent = null, error = null)
                        },
                        start = { starts++ },
                        retry = {
                          retries++
                          state = state.copy(action = "preview", error = null)
                        })
                  },
                  focusSafeActionOnOpen = true)
            }
          }
          .use { fixture ->
            fixture.render()
            assertTrue(fixture.hasText(title), mode)
            assertTrue(fixture.hasText(preparing), mode)
            assertFalse(fixture.hasText("Expected model requests: 3 · Maximum: 6"), mode)
            assertFalse(fixture.tryClick("Retry preview"), mode)
            assertEquals(0, retries + starts + closes, mode)
            state = state.copy(action = "", error = diagnostic)
            fixture.render()
            val failureTitle =
                when (mode) {
                  "full project" -> "Full project preview failed."
                  "stale & failed" -> "Stale & failed preview failed."
                  else -> "Continuation preview failed."
                }
            assertTrue(fixture.hasText(failureTitle), mode)
            assertTrue(fixture.hasText(diagnostic), "Complete diagnostic for $mode")
            assertTrue(fixture.hasText("Retry this preview with the same scope, or Close."), mode)
            assertFalse(fixture.hasText("Expected model requests: 3 · Maximum: 6"), mode)
            assertFalse(fixture.tryClick("New preview"), mode)
            assertTrue(fixture.isFocusedControl("Close"), mode)
            assertTrue(fixture.requestFocus("Retry preview"), mode)
            fixture.render()
            assertTrue(fixture.pressKey(Key.Enter), mode)
            fixture.render()
            assertEquals(1, retries, mode)
            assertEquals(0, starts + closes, mode)
            assertTrue(fixture.hasText(preparing), mode)
            assertFalse(fixture.hasText(diagnostic), mode)
            assertFalse(fixture.tryClick("Retry preview"), mode)
            assertTrue(fixture.tryClick("Close"), mode)
            fixture.render()
            assertEquals(1, closes, mode)
            assertEquals(0, starts, mode)
          }
    }
  }

  @Test
  fun obsoleteOrMissingIntentOffersCloseInsteadOfDefaultingToFullPreview() {
    val run = analysisRunFixture()
    val intent =
        AnalysisPreviewIntent(
            "project",
            "revision",
            run.plan.limits,
            run.plan.refresh,
            run.plan.retryStaleFailed,
            run.identity,
            run.plan)
    for (obsolete in listOf(true, false)) {
      var retries = 0
      var closes = 0
      val state =
          ProjectAnalysisRunState(
              run = run.copy(identity = run.identity.copy(generation = "replacement")),
              previewIntent = if (obsolete) intent else null,
              error = "Captured run changed")
      ComposeVisualFixture(420, 360) {
            Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
              IdeDialogSurface(
                  320.dp,
                  title = { DesktopAnalysisAdmissionTitle(state) },
                  content = { DesktopAnalysisAdmissionContent(state, { _, _ -> }, {}) },
                  actions = {
                    DesktopAnalysisAdmissionActions(state, { closes++ }, {}, { retries++ })
                  })
            }
          }
          .use { fixture ->
            fixture.render()
            assertTrue(fixture.hasText("Close and request a new preview from Analysis."))
            assertTrue(fixture.copyTextByDragging("Captured run changed").isNotBlank())
            assertFalse(fixture.tryClick("Retry preview"))
            assertFalse(fixture.tryClick("New preview"))
            assertEquals(0, retries)
            assertTrue(fixture.tryClick("Close"))
            assertEquals(1, closes)
          }
    }
  }
}
