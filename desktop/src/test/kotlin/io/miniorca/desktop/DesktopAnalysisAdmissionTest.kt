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
  private val bugConsent = "Confirm bug destination · bug-model (provider bug-provider)"
  private val analyzeConsent =
      "Confirm analyze destination · review-model (provider analyze-provider)"
  private val contextExplanation =
      "Start or Resume may send eligible source and project context for this previewed scope to the listed models under the existing context policy. This preview describes the plan, not the exact content sent to a model. One Start or Resume may initiate multiple model requests; analysis does not execute project code or modify source files. This consent does not grant function-edit permission or execution trust."
  private val securityExplanation =
      "AI Security review of eligible source is advisory. Model findings are unverified, not a verified scan or safety assurance. This acknowledgment does not change the returned stage plan."

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
    ComposeVisualFixture(480, 600, 1.5f) {
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
          fixture.resize(480, 560)
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
          for (label in listOf(bugConsent, analyzeConsent, "Include AI Security review")) {
            assertTrue(
                fixture.descriptionBounds(label).height <= body.height,
                "$label cannot fit in scroll body: ${fixture.descriptionBounds(label)} vs $body")
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
  fun outstandingDecisionTracksPartialConsentAndUncheckingWithoutInspectingDetails() {
    val preview = analysisPreviewFixture()
    var state by mutableStateOf(ProjectAnalysisRunState(admission = AnalysisAdmission(preview)))
    var starts = 0
    var changes = 0
    val bugMissing = "Destination still needed: $bugConsent"
    val analyzeMissing = "Destination still needed: $analyzeConsent"
    val securityMissing = "Security intent still needed: Include AI Security review."
    ComposeVisualFixture(640, 1300) {
          Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            IdeDialogSurface(
                1200.dp,
                title = { DesktopAnalysisAdmissionTitle(state) },
                content = {
                  DesktopAnalysisAdmissionContent(
                      state,
                      { id, checked ->
                        changes++
                        state =
                            state.copy(
                                admission =
                                    state.admission!!.copy(
                                        providerIds =
                                            if (checked) state.admission!!.providerIds + id
                                            else state.admission!!.providerIds - id))
                      },
                      { checked ->
                        changes++
                        state =
                            state.copy(admission = state.admission!!.copy(securityReview = checked))
                      })
                },
                actions = { DesktopAnalysisAdmissionActions(state, {}, { starts++ }, {}) },
                focusSafeActionOnOpen = true)
          }
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Your confirmation"))
          assertTrue(fixture.hasText("Still needed before Start or Resume:"))
          assertTrue(fixture.hasText(bugMissing))
          assertTrue(fixture.hasText(analyzeMissing))
          assertTrue(fixture.hasText(securityMissing))
          assertTrue(fixture.isDisabled("Start analysis"))
          assertTrue(fixture.isFocusedControl("Close"))
          assertTrue(
              fixture.firstVisibleTextBounds("Your confirmation").top <
                  fixture.firstVisibleTextBounds("Preview details").top)
          assertFalse(fixture.hasText("Eligible stage"))
          assertEquals(0, changes + starts)
          assertTrue(fixture.requestDescriptionFocus(bugConsent))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertFalse(fixture.hasText(bugMissing))
          assertTrue(fixture.hasText(analyzeMissing))
          assertTrue(fixture.hasText(securityMissing))
          assertTrue(fixture.isDisabled("Start analysis"))
          assertTrue(fixture.requestDescriptionFocus("Include AI Security review"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertFalse(fixture.hasText(securityMissing))
          assertTrue(fixture.hasText(analyzeMissing))
          assertTrue(fixture.isDisabled("Start analysis"))
          assertTrue(fixture.requestDescriptionFocus(analyzeConsent))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertFalse(fixture.hasText("Still needed before Start or Resume:"))
          assertTrue(fixture.hasText("Confirmations complete for this preview."))
          assertFalse(fixture.isDisabled("Start analysis"))
          assertTrue(fixture.requestDescriptionFocus(bugConsent))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertTrue(fixture.hasText(bugMissing))
          assertFalse(fixture.hasText(analyzeMissing))
          assertFalse(fixture.hasText(securityMissing))
          assertTrue(fixture.isDisabled("Start analysis"))
          assertEquals(4, changes)
          assertEquals(0, starts)
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
            assertTrue(fixture.hasText(contextExplanation))
            assertTrue(fixture.hasText(securityExplanation))
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
              assertTrue(fixture.hasText("Expected model requests without retries: 0"))
              assertTrue(fixture.hasText("Inclusive maximum model requests with retries: 0"))
              assertFalse(fixture.hasText("Expected model requests without retries: 3"))
            }
          }
    }
  }

  @Test
  fun localOnlySecurityIntentStaysInThePlanUntilSeparatelyAcknowledged() {
    val base = analysisPreviewFixture()
    val preview =
        base.copy(
            files =
                listOf(
                    base.files
                        .single()
                        .copy(
                            stages =
                                listOf(
                                    AnalysisStagePlan(
                                        "security_ai",
                                        true,
                                        false,
                                        providerId = "local-model",
                                        maxModelRequests = 1),
                                    AnalysisStagePlan(
                                        "security_rules", true, false, maxModelRequests = 0)))),
            providers =
                listOf(
                    base.providers
                        .last()
                        .copy(
                            id = "local-model",
                            remoteConfirmationRequired = false,
                            model =
                                base.providers
                                    .last()
                                    .model
                                    .copy(model = "local-review", remoteProvider = false))),
            securityReviewIntentRequired = true)
    var state by mutableStateOf(ProjectAnalysisRunState(admission = AnalysisAdmission(preview)))
    var securityChanges = 0
    var starts = 0
    ComposeVisualFixture(480, 800) {
          Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            IdeDialogSurface(
                700.dp,
                title = { DesktopAnalysisAdmissionTitle(state) },
                content = {
                  DesktopAnalysisAdmissionContent(
                      state,
                      { _, _ -> error("Local destination must not require remote consent") },
                      { checked ->
                        securityChanges++
                        state =
                            state.copy(admission = state.admission!!.copy(securityReview = checked))
                      })
                },
                actions = { DesktopAnalysisAdmissionActions(state, {}, { starts++ }, {}) })
          }
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Local destination: https://analyze.example"))
          assertTrue(fixture.hasText("AI Security review"))
          assertTrue(fixture.hasText("Security rules"))
          assertTrue(fixture.hasText(contextExplanation))
          assertTrue(fixture.hasText(securityExplanation))
          assertFalse(fixture.hasDescription(analyzeConsent))
          assertTrue(fixture.hasText("Security intent still needed: Include AI Security review."))
          assertEquals(
              ToggleableState.Off, fixture.descriptionToggleableState("Include AI Security review"))
          assertTrue(fixture.isDisabled("Start analysis"))
          fixture.resize(440, 750)
          fixture.render()
          assertEquals(0, securityChanges + starts)
          assertEquals(preview, state.admission!!.preview)
          assertTrue(fixture.requestDescriptionFocus("Include AI Security review"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertEquals(1, securityChanges)
          assertEquals(0, starts)
          assertEquals(
              ToggleableState.On, fixture.descriptionToggleableState("Include AI Security review"))
          assertTrue(state.admission!!.isConfirmed())
          assertFalse(fixture.hasText("Security intent still needed: Include AI Security review."))
          assertTrue(fixture.hasText("Confirmations complete for this preview."))
          assertEquals(preview, state.admission!!.preview)
          assertTrue(fixture.hasText("AI Security review"))
        }
  }

  @Test
  fun noConfirmationPreviewExplainsContextWithoutDispatchingOnInspection() {
    val base = analysisPreviewFixture()
    val preview =
        base.copy(
            providers =
                listOf(
                    base.providers
                        .first()
                        .copy(
                            remoteConfirmationRequired = false,
                            model = base.providers.first().model.copy(remoteProvider = false))),
            securityReviewIntentRequired = false)
    val state = ProjectAnalysisRunState(admission = AnalysisAdmission(preview))
    var changes = 0
    var starts = 0
    ComposeVisualFixture(480, 760) {
          Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            IdeDialogSurface(
                680.dp,
                title = { DesktopAnalysisAdmissionTitle(state) },
                content = {
                  DesktopAnalysisAdmissionContent(state, { _, _ -> changes++ }, { changes++ })
                },
                actions = { DesktopAnalysisAdmissionActions(state, {}, { starts++ }, {}) })
          }
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText(contextExplanation))
          assertFalse(fixture.hasText(securityExplanation))
          assertFalse(fixture.hasDescription("Include AI Security review"))
          assertFalse(fixture.hasDescription(bugConsent))
          assertTrue(fixture.hasText("Local destination: https://bug.example"))
          assertTrue(state.admission!!.isConfirmed())
          assertFalse(fixture.hasText("Still needed before Start or Resume:"))
          assertTrue(fixture.hasText("Confirmations complete for this preview."))
          assertFalse(fixture.isDisabled("Start analysis"))
          fixture.revealText(contextExplanation, "ide-dialog-body")
          fixture.resize(440, 720)
          fixture.render()
          assertEquals(0, changes + starts)
          assertTrue(fixture.hasText(contextExplanation))
          assertTrue(fixture.hasText("Close"))
        }
  }

  @Test
  fun completedConfirmationsCannotStartAnEmptyScopeIncludingWhenNoneAreRequired() {
    val base = analysisPreviewFixture()
    for (preview in
        listOf(
            base.copy(files = emptyList()),
            base.copy(
                files = emptyList(),
                providers = emptyList(),
                securityReviewIntentRequired = false))) {
      val admission =
          AnalysisAdmission(
              preview,
              providerIds =
                  preview.providers.filter { it.remoteConfirmationRequired }.map { it.id }.toSet(),
              securityReview = preview.securityReviewIntentRequired)
      val state = ProjectAnalysisRunState(admission = admission)
      var starts = 0
      ComposeVisualFixture(480, 640) {
            Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
              IdeDialogSurface(
                  560.dp,
                  title = { DesktopAnalysisAdmissionTitle(state) },
                  content = { DesktopAnalysisAdmissionContent(state, { _, _ -> }, {}) },
                  actions = { DesktopAnalysisAdmissionActions(state, {}, { starts++ }, {}) })
            }
          }
          .use { fixture ->
            fixture.render()
            assertTrue(admission.isConfirmed())
            assertTrue(fixture.hasText("No eligible files to analyze."))
            assertTrue(fixture.hasText("Confirmations complete; no included files to analyze."))
            assertFalse(fixture.hasText("Still needed before Start or Resume:"))
            assertTrue(fixture.isDisabled("Start analysis"))
            assertFalse(fixture.tryClick("Start analysis"))
            assertEquals(0, starts)
          }
    }
  }

  @Test
  fun daemonTotalsAndDispatchWindowAreNotInferredFromFileCount() {
    val preview =
        analysisPreviewFixture()
            .copy(
                files = listOf(AnalysisPlannedFile("one.go", "a", "Go", 1, emptyList())),
                expectedModelRequests = 47,
                maxModelRequests = 83,
                limits = AnalysisRunLimits(7, 120, 3))
    ComposeVisualFixture(560, 1200) {
          DesktopAnalysisAdmissionContent(
              ProjectAnalysisRunState(admission = AnalysisAdmission(preview)), { _, _ -> }, {})
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Full project scope: 1 included file · 0 excluded"))
          assertTrue(fixture.hasText("Expected model requests without retries: 47"))
          assertTrue(fixture.hasText("Inclusive maximum model requests with retries: 83"))
          assertTrue(
              fixture.hasText(
                  "Dispatch window: up to 7 files and 120 seconds. These limits bound this dispatch, not the project inventory or an ETA."))
          assertTrue(
              fixture.hasText(
                  "Up to 3 attempts per stage, including the initial attempt. Further work requires explicit continuation."))
          assertFalse(fixture.hasText("Run limits"))
        }
  }

  @Test
  fun everyProviderDisplaysItsOwnMetadataAndMissingFieldsStayUnavailable() {
    val origin = "https://provider.example/" + "nested/".repeat(25) + "end"
    val base = analysisPreviewFixture()
    val preview =
        base.copy(
            providers =
                listOf(
                    base.providers
                        .first()
                        .copy(model = base.providers.first().model.copy(providerOrigin = origin)),
                    base.providers
                        .last()
                        .copy(
                            model =
                                base.providers
                                    .last()
                                    .model
                                    .copy(
                                        scope = "",
                                        profile = "  ",
                                        model = "",
                                        providerOrigin = "",
                                        remoteProvider = false))))
    ComposeVisualFixture(420, 2600, 1.5f) {
          DesktopAnalysisAdmissionContent(
              ProjectAnalysisRunState(admission = AnalysisAdmission(preview)), { _, _ -> }, {})
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Scope: bug"))
          assertTrue(fixture.hasText("Profile: remote"))
          assertTrue(fixture.hasText("Model: bug-model"))
          assertTrue(fixture.hasText("Remote destination: $origin"))
          assertTrue(fixture.hasText("Scope: Unavailable"))
          assertTrue(fixture.hasText("Profile: Unavailable"))
          assertTrue(fixture.hasText("Model: Unavailable"))
          assertTrue(fixture.hasText("Local destination: Unavailable"))
          assertTrue(fixture.hasDescription(bugConsent))
          assertFalse(fixture.hasDescription("Confirm Unavailable destination"))
          assertTrue(fixture.copyTextByDragging("Remote destination: $origin").isNotBlank())
        }
  }

  @Test
  fun repeatedScopesModelsAndMissingMetadataStillHaveIndependentKeyboardConsent() {
    val base = analysisPreviewFixture()
    val remote = base.providers.first()
    val local =
        base.providers
            .last()
            .copy(
                id = "local-provider",
                remoteConfirmationRequired = false,
                model = base.providers.last().model.copy(remoteProvider = false))
    val providers =
        listOf(
            remote.copy(
                id = "remote-one",
                model = remote.model.copy(scope = "shared", model = "same", providerOrigin = "")),
            remote.copy(
                id = "remote-two",
                model = remote.model.copy(scope = "shared", model = "same", providerOrigin = "")),
            remote.copy(
                id = "remote-three",
                model =
                    remote.model.copy(scope = "", profile = "", model = "", providerOrigin = "")),
            remote.copy(
                id = "remote-four",
                model =
                    remote.model.copy(scope = "", profile = "", model = "", providerOrigin = "")),
            local)
    val preview = base.copy(providers = providers)
    var state by mutableStateOf(ProjectAnalysisRunState(admission = AnalysisAdmission(preview)))
    val changes = mutableListOf<Pair<String, Boolean>>()
    val labels =
        providers.take(4).map { provider ->
          "Confirm ${provider.model.scope.ifBlank { "Unavailable" }} destination · " +
              "${provider.model.model.ifBlank { "Unavailable" }} (provider ${provider.id})"
        }
    ComposeVisualFixture(600, 1800) {
          DesktopAnalysisAdmissionContent(
              state,
              { id, checked ->
                changes += id to checked
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
              })
        }
        .use { fixture ->
          fixture.render()
          assertEquals(4, labels.distinct().size)
          assertTrue(fixture.hasText("Local destination: https://analyze.example"))
          assertTrue(fixture.hasText("Scope: Unavailable"))
          assertTrue(fixture.hasText("Profile: Unavailable"))
          assertTrue(fixture.hasText("Model: Unavailable"))
          for ((index, label) in labels.withIndex()) {
            assertEquals(1, fixture.clickableDescriptionCount(label))
            assertEquals(ToggleableState.Off, fixture.descriptionToggleableState(label))
            assertTrue(fixture.requestDescriptionFocus(label))
            assertTrue(fixture.pressKey(if (index % 2 == 0) Key.Enter else Key.Spacebar))
            fixture.render()
            assertEquals(
                providers.take(index + 1).map { it.id }.toSet(), state.admission!!.providerIds)
            assertEquals(ToggleableState.On, fixture.descriptionToggleableState(label))
          }
          assertEquals(providers.take(4).map { it.id to true }, changes)
          assertFalse(state.admission!!.securityReview)
          assertFalse(state.admission!!.isConfirmed())
          assertTrue(fixture.requestDescriptionFocus(labels[1]))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertEquals(
              providers.take(4).map { it.id }.toSet() - "remote-two", state.admission!!.providerIds)
          assertEquals("remote-two" to false, changes.last())
          assertEquals(ToggleableState.On, fixture.descriptionToggleableState(labels[0]))
          assertEquals(ToggleableState.Off, fixture.descriptionToggleableState(labels[1]))
        }
  }

  @Test
  fun returnedStageSummaryCountsDispositionsAndUsesStageProviderReferences() {
    val base = analysisPreviewFixture()
    val preview =
        base.copy(
            files =
                listOf(
                    AnalysisPlannedFile(
                        "first.go",
                        "a",
                        "Go",
                        1,
                        listOf(
                            AnalysisStagePlan(
                                "semantic",
                                true,
                                true,
                                providerId = "bug-provider",
                                maxModelRequests = 0),
                            AnalysisStagePlan(
                                "performance",
                                true,
                                false,
                                providerId = "analyze-provider",
                                maxModelRequests = 2),
                            AnalysisStagePlan("security_rules", true, false, maxModelRequests = 0),
                            AnalysisStagePlan(
                                "security_ai",
                                true,
                                false,
                                "The model for this stage is not configured.",
                                "analyze-provider",
                                0),
                            AnalysisStagePlan(
                                "new_stage",
                                true,
                                false,
                                providerId = "missing-provider",
                                maxModelRequests = 1))),
                    AnalysisPlannedFile(
                        "second.go",
                        "b",
                        "Go",
                        1,
                        listOf(
                            AnalysisStagePlan(
                                "semantic",
                                false,
                                false,
                                "Not eligible for semantic source analysis.",
                                "bug-provider",
                                0),
                            AnalysisStagePlan(
                                "performance",
                                true,
                                false,
                                providerId = "bug-provider",
                                maxModelRequests = 1),
                            AnalysisStagePlan(
                                "security_rules",
                                false,
                                false,
                                "Passive rules require Go.",
                                maxModelRequests = 0)))),
            // Headline bounds belong to the daemon, not the stage-summary arithmetic.
            expectedModelRequests = 41,
            maxModelRequests = 79,
            providers = base.providers.reversed())
    val state = ProjectAnalysisRunState(admission = AnalysisAdmission(preview))
    ComposeVisualFixture(680, 1800) { DesktopAnalysisAdmissionContent(state, { _, _ -> }, {}) }
        .use { fixture ->
          fixture.render()
          for (label in
              listOf(
                  "Returned stage plan",
                  "Code analysis",
                  "Performance review",
                  "Security rules",
                  "AI Security review",
                  "new_stage",
                  "2 returned · 1 applicable · 1 cached/reused · 0 with model requests planned · 2 with no model requests planned",
                  "2 returned · 2 applicable · 0 cached/reused · 2 with model requests planned · 0 with no model requests planned",
                  "2 returned · 1 applicable · 0 cached/reused · 0 with model requests planned · 2 with no model requests planned",
                  "1 returned · 1 applicable · 0 cached/reused · 0 with model requests planned · 1 with no model requests planned",
                  "1 returned · 1 applicable · 0 cached/reused · 1 with model requests planned · 0 with no model requests planned",
                  "Deterministic Security rules · no model destination",
                  "Model destination: bug-model · Remote https://bug.example (provider bug-provider)",
                  "Model destination: review-model · Remote https://analyze.example (provider analyze-provider)",
                  "Model destination: Unavailable (provider reference missing-provider)",
                  "Expected model requests without retries: 41",
                  "Inclusive maximum model requests with retries: 79")) assertTrue(
              fixture.hasText(label), label)
          assertFalse(fixture.hasText("failed analysis"))
          assertFalse(state.admission!!.isConfirmed())
        }
  }

  @Test
  fun stageDestinationFollowsProviderIdEvenWhenCategoryAndProviderOrderDisagree() {
    val base = analysisPreviewFixture()
    val preview =
        base.copy(
            files =
                listOf(
                    base.files
                        .single()
                        .copy(
                            stages =
                                listOf(
                                    AnalysisStagePlan(
                                        "semantic",
                                        true,
                                        false,
                                        providerId = "analyze-provider",
                                        maxModelRequests = 1)))),
            providers = base.providers.reversed())
    ComposeVisualFixture(580, 1000) {
          DesktopAnalysisAdmissionContent(
              ProjectAnalysisRunState(admission = AnalysisAdmission(preview)), { _, _ -> }, {})
        }
        .use { fixture ->
          fixture.render()
          assertTrue(
              fixture.hasText(
                  "Model destination: review-model · Remote https://analyze.example (provider analyze-provider)"))
          assertFalse(
              fixture.hasText(
                  "Model destination: bug-model · Remote https://bug.example (provider bug-provider)"))
          assertFalse(fixture.hasText("Security rules"))
        }
  }

  @Test
  fun resumedUncachedZeroAllowanceDoesNotClaimReuseOrCompletion() {
    val base = analysisPreviewFixture()
    val preview =
        base.copy(
            files =
                listOf(
                    AnalysisPlannedFile(
                        "retained.go",
                        "hash",
                        "Go",
                        1,
                        listOf(
                            AnalysisStagePlan(
                                "semantic",
                                true,
                                false,
                                providerId = "bug-provider",
                                maxModelRequests = 0)))),
            expectedModelRequests = 0,
            maxModelRequests = 0)
    val state =
        ProjectAnalysisRunState(
            admission = AnalysisAdmission(preview, analysisRunFixture().identity))
    ComposeVisualFixture(560, 1000) { DesktopAnalysisAdmissionContent(state, { _, _ -> }, {}) }
        .use { fixture ->
          fixture.render()
          assertTrue(
              fixture.hasText(
                  "1 returned · 1 applicable · 0 cached/reused · 0 with model requests planned · 1 with no model requests planned"))
          assertTrue(fixture.hasText("Expected model requests without retries: 0"))
          assertTrue(fixture.hasText("Inclusive maximum model requests with retries: 0"))
          assertFalse(fixture.hasText("completed"))
          assertFalse(fixture.hasText("successful"))
          assertFalse(state.admission!!.isConfirmed())
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
          assertTrue(fixture.hasText("Still needed before Start or Resume:"))
          assertTrue(fixture.hasText("excluded.go"))
          assertTrue(fixture.hasText(preview.excluded.single().reason))
          assertTrue(fixture.hasDescription("Include AI Security review"))
          assertTrue(fixture.hasDescription(bugConsent))
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
          assertTrue(fixture.hasText("Still needed before Start or Resume:"))
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
                    "Remote destination: https://analyze.example",
                    "Scope: bug",
                    "Profile: remote",
                    "Model: bug-model")) assertTrue(fixture.hasText(label))
            assertFalse(state.admission!!.isConfirmed())
            for (label in listOf(bugConsent, analyzeConsent, "Include AI Security review")) {
              assertEquals(1, fixture.clickableDescriptionCount(label))
              assertEquals(ToggleableState.Off, fixture.descriptionToggleableState(label))
              assertEquals("Not confirmed", fixture.descriptionStateDescription(label))
            }
            assertTrue(fixture.requestDescriptionFocus(bugConsent))
            assertTrue(fixture.pressKey(Key.Enter))
            fixture.render()
            assertEquals(listOf("bug-provider" to true), providerChanges)
            assertEquals(setOf("bug-provider"), state.admission!!.providerIds)
            assertFalse(state.admission!!.securityReview)
            assertEquals(ToggleableState.On, fixture.descriptionToggleableState(bugConsent))
            assertEquals("Confirmed", fixture.descriptionStateDescription(bugConsent))
            assertEquals(ToggleableState.Off, fixture.descriptionToggleableState(analyzeConsent))
            fixture.clickVisibleDescription(analyzeConsent)
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
  fun includedFilesExposeCompleteSelectableStageEvidenceWithoutChangingAdmission() {
    val path = "internal/" + "deeply-nested/".repeat(12) + "handler.go"
    val reason = "No model configured: " + "configuration detail ".repeat(6) + "end"
    val base = analysisPreviewFixture()
    val preview =
        base.copy(
            files =
                listOf(
                    AnalysisPlannedFile(
                        path,
                        "hash",
                        "Go",
                        200,
                        listOf(
                            AnalysisStagePlan(
                                "semantic", true, true, "Reused saved evidence", "bug-provider", 0),
                            AnalysisStagePlan(
                                "security_ai", false, false, reason, "analyze-provider", 0),
                            AnalysisStagePlan(
                                "new_stage", true, false, "", "analyze-provider", 2))),
                    AnalysisPlannedFile(
                        "other.go",
                        "second",
                        "Go",
                        10,
                        listOf(
                            AnalysisStagePlan(
                                "security_rules",
                                true,
                                false,
                                "Local rules",
                                maxModelRequests = 0)))),
            excluded = listOf(AnalysisExcludedFile("skipped.go", "Whole-file policy exclusion")))
    val state = ProjectAnalysisRunState(admission = AnalysisAdmission(preview))
    var providerChanges = 0
    var securityChanges = 0
    var starts = 0
    ComposeVisualFixture(440, 560, 1.5f) {
          Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            IdeDialogSurface(
                440.dp,
                title = { DesktopAnalysisAdmissionTitle(state) },
                content = {
                  DesktopAnalysisAdmissionContent(
                      state, { _, _ -> providerChanges++ }, { securityChanges++ })
                },
                actions = { DesktopAnalysisAdmissionActions(state, {}, { starts++ }, {}) })
          }
        }
        .use { fixture ->
          fixture.render()
          val label = "Included file 1 · $path"
          assertTrue(fixture.hasDescription("Expand $label"))
          assertEquals("Collapsed", fixture.descriptionStateDescription("Expand $label"))
          assertFalse(fixture.hasText("Ineligible stage on included file"))
          fixture.revealText(path, "ide-dialog-body")
          assertTrue(fixture.copyTextByDragging(path).isNotBlank())
          assertTrue(fixture.requestDescriptionFocus("Expand $label"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertEquals("Expanded", fixture.descriptionStateDescription("Collapse $label"))
          for (text in
              listOf(
                  "Code analysis",
                  "Eligible stage",
                  "Cached/reused",
                  "Maximum model requests for this stage: 0",
                  "Reason: Reused saved evidence",
                  "AI Security review",
                  "Ineligible stage on included file",
                  "Not cached",
                  "Reason: $reason",
                  "new_stage",
                  "Maximum model requests for this stage: 2",
                  "Reason: Unavailable")) assertTrue(fixture.hasText(text), text)
          fixture.revealText("Reason: $reason", "ide-dialog-body")
          assertTrue(fixture.copyTextByDragging("Reason: $reason").isNotBlank())
          assertTrue(fixture.hasText("skipped.go"))
          assertTrue(fixture.hasText("Whole-file policy exclusion"))
          assertTrue(fixture.requestDescriptionFocus("Collapse $label"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertEquals("Collapsed", fixture.descriptionStateDescription("Expand $label"))
          assertEquals(0, providerChanges + securityChanges + starts)
          assertTrue(state.admission!!.providerIds.isEmpty())
          assertFalse(state.admission.securityReview)
          assertTrue(fixture.isDisabled("Start analysis"))
        }
  }

  @Test
  fun longBackendReasonWrapsAndItsTailRemainsReachableInTheBoundedDialog() {
    val reason = "Missing model: " + "full backend explanation ".repeat(12) + "final detail"
    val base = analysisPreviewFixture()
    val file =
        base.files
            .single()
            .copy(
                stages =
                    listOf(
                        AnalysisStagePlan(
                            "security_ai", false, false, reason, maxModelRequests = 0)))
    val state =
        ProjectAnalysisRunState(admission = AnalysisAdmission(base.copy(files = listOf(file))))
    ComposeVisualFixture(440, 900, 1.5f) {
          Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            IdeDialogSurface(
                750.dp,
                title = { DesktopAnalysisAdmissionTitle(state) },
                content = { DesktopAnalysisAdmissionContent(state, { _, _ -> }, {}) },
                actions = { DesktopAnalysisAdmissionActions(state, {}, {}, {}) })
          }
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.requestDescriptionFocus("Expand Included file 1 · ${file.path}"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          fixture.revealText("Reason: $reason", "ide-dialog-body")
          fixture.assertTextWrapsWithoutClipping("Reason: $reason")
          assertTrue(fixture.copyTextByDragging("Reason: $reason").isNotBlank())
          assertTrue(fixture.hasText("Close"))
        }
  }

  @Test
  fun fileDisclosuresResetForReplacementPreviewButNotRecompositionAndReachFinalRecord() {
    val base = analysisPreviewFixture()
    val files =
        (1..28).map { index ->
          AnalysisPlannedFile(
              "src/package/file-$index.go",
              "hash-$index",
              "Go",
              10,
              listOf(
                  AnalysisStagePlan(
                      "semantic", true, false, "Reason for file $index", "bug-provider", 1)))
        }
    var state by
        mutableStateOf(
            ProjectAnalysisRunState(admission = AnalysisAdmission(base.copy(files = files))))
    ComposeVisualFixture(400, 510) {
          Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            IdeDialogSurface(
                410.dp,
                title = { DesktopAnalysisAdmissionTitle(state) },
                content = { DesktopAnalysisAdmissionContent(state, { _, _ -> }, {}) },
                actions = { DesktopAnalysisAdmissionActions(state, {}, {}, {}) })
          }
        }
        .use { fixture ->
          fixture.render()
          val first = "Included file 1 · ${files.first().path}"
          val last = "Included file 28 · ${files.last().path}"
          assertEquals(
              28,
              (1..28).count {
                fixture.hasDescription("Expand Included file $it · ${files[it - 1].path}")
              })
          assertTrue(fixture.requestDescriptionFocus("Expand $first"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertTrue(fixture.hasText("Reason: Reason for file 1"))
          fixture.render()
          assertEquals("Expanded", fixture.descriptionStateDescription("Collapse $first"))
          fixture.revealTextFullyWithin(files.last().path, "ide-dialog-body")
          fixture.copyTextByDragging(files.last().path, files.last().path)
          assertTrue(fixture.requestDescriptionFocus("Expand $last"))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          fixture.revealText("Reason: Reason for file 28", "ide-dialog-body")
          fixture.copyTextByDragging("Reason: Reason for file 28", "Reason: Reason for file 28")
          assertEquals("Expanded", fixture.descriptionStateDescription("Collapse $last"))
          state =
              state.copy(
                  admission =
                      AnalysisAdmission(base.copy(previewId = "replacement", files = files)))
          fixture.render()
          assertEquals("Collapsed", fixture.descriptionStateDescription("Expand $first"))
          assertEquals("Collapsed", fixture.descriptionStateDescription("Expand $last"))
          assertFalse(fixture.hasText("Reason: Reason for file 28"))
          assertTrue(fixture.hasText("Close"))
          assertTrue(fixture.isDisabled("Start analysis"))
        }
  }

  @Test
  fun everyWholeFileExclusionRetainsItsOwnPathAndCompleteReasonWithoutChangingConsent() {
    val exclusions =
        listOf(
            AnalysisExcludedFile(
                "src/policy/internal/secret.go",
                "Policy exclusion: " + "private source restriction ".repeat(14) + "policy end"),
            AnalysisExcludedFile(
                "vendor/日本語/unsupported.asset",
                "Source eligibility: " + "unsupported content ".repeat(14) + "source end"),
            AnalysisExcludedFile(
                "src/retry/already-current.go",
                "Selective retry: " + "not stale or failed ".repeat(14) + "retry end"))
    val base = analysisPreviewFixture()
    val preview =
        base.copy(
            retryStaleFailed = true,
            refresh = false,
            files = emptyList(),
            excluded = exclusions,
            expectedModelRequests = 0,
            maxModelRequests = 0)
    val state = ProjectAnalysisRunState(admission = AnalysisAdmission(preview))
    var operations = 0
    ComposeVisualFixture(420, 960, 1.5f) {
          Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            IdeDialogSurface(
                860.dp,
                title = { DesktopAnalysisAdmissionTitle(state) },
                content = {
                  DesktopAnalysisAdmissionContent(state, { _, _ -> operations++ }, { operations++ })
                },
                actions = {
                  DesktopAnalysisAdmissionActions(state, { operations++ }, { operations++ }, {})
                },
                focusSafeActionOnOpen = true)
          }
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("No stale or failed files to analyze."))
          assertTrue(fixture.isDisabled("Start analysis"))
          assertFalse(fixture.hasText("User-selected exclusions"))
          for (excluded in exclusions) {
            assertEquals(1, fixture.textCount(excluded.path), excluded.path)
            assertEquals(1, fixture.textCount(excluded.reason), excluded.path)
            fixture.revealText(excluded.reason, "ide-dialog-body")
            fixture.assertTextWrapsWithoutClipping(excluded.reason)
            assertTrue(fixture.copyTextByDragging(excluded.reason).isNotBlank(), excluded.path)
            assertTrue(fixture.hasText(excluded.reason), excluded.path)
          }
          assertTrue(fixture.verticalScrollValue("ide-dialog-body") > 0f)
          assertEquals(0, operations)
          assertTrue(state.admission!!.providerIds.isEmpty())
          assertFalse(state.admission.securityReview)
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
            assertFalse(fixture.hasText("Expected model requests without retries: 3"), mode)
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
            assertFalse(fixture.hasText("Expected model requests without retries: 3"), mode)
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
