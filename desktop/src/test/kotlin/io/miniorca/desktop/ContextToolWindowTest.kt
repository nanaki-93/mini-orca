package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.input.key.Key
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class ContextToolWindowTest {
  @Test
  fun contextCreationUsesPackageOnlyFileAndExplainsBusyOrUnsupportedFiles() {
    val file = file().copy(path = "internal/empty.go", name = "empty.go", lineCount = 1)
    var creations = 0
    val actions = ContextToolWindowActions({}, {}, {}, {}, {}, createDeclaration = { creations++ })
    for ((source, busy) in
        listOf(
            file to false,
            file to true,
            file.copy(binary = true) to false,
            file.copy(language = "Markdown") to false)) {
      val inspector =
          requireNotNull(
              symbolInspectorUiState(
                  source,
                  emptyList(),
                  null,
                  null,
                  false,
                  InspectorProviderState(false, false),
                  null))
      ComposeVisualFixture(320, 400) {
            ContextCreationAction(
                ContextToolWindowState(
                    inspector, ScopedModel(), false, null, null, creationInProgress = busy),
                actions)
          }
          .use { fixture ->
            fixture.render()
            assertTrue(fixture.hasText("New function"))
            if (source == file && !busy) {
              assertFalse(fixture.isDisabled("New function"))
              fixture.clickText("New function")
            } else {
              assertTrue(fixture.isDisabled("New function"))
              assertTrue(
                  fixture.hasText(requireNotNull(declarationCreationBlockedReason(source, busy))))
            }
          }
    }
    assertEquals(1, creations)
  }

  @Test
  fun noSelectionAndFailedLocalFileReadKeepFileNavigationAdjacent() {
    var selections = 0
    var analyses = 0
    val actions =
        ContextToolWindowActions({}, { analyses++ }, {}, {}, {}, openFile = { selections++ })
    for (error in listOf(null, "Permission denied", "")) {
      ComposeVisualFixture(320, 300, 1.5f) {
            ContextToolWindow(
                ContextToolWindowState(
                    null, ScopedModel(), false, null, null, fileReadError = error),
                actions)
          }
          .use { fixture ->
            fixture.render()
            assertTrue(
                fixture.hasText(if (error == null) "No file selected" else "Could not open file"))
            if (error != null) {
              assertTrue(
                  fixture.hasText(
                      "Reading local file data failed. ${error.ifBlank { "No details available." }} Select a file in Files to try again."))
            }
            fixture.clickText("Select a file")
          }
    }
    assertEquals(3, selections)
    assertEquals(0, analyses)
  }

  @Test
  fun projectAnalysisActionAndFileResultsHaveSeparateExplicitIntents() {
    var admissions = 0
    var navigations = 0
    val actions =
        ContextToolWindowActions({}, { admissions++ }, {}, {}, {}, viewResults = { navigations++ })
    ComposeVisualFixture(320, 300, 1.5f) {
          ContextProjectAnalysisActions(
              ContextToolWindowState(null, ScopedModel(), false, null, null), actions)
        }
        .use { fixture ->
          fixture.render("context-project-actions-320-1.5")
          fixture.assertTextFits("Analyze project")
          fixture.clickText("View this file’s results")
          assertEquals(0, admissions)
          assertEquals(1, navigations)
          fixture.clickText("Analyze project")
          assertEquals(1, admissions)
        }
  }

  @Test
  fun declarationExplanationActionsDistinguishLifecycle() {
    assertEquals("Explain declaration", explanationActionLabel(DeclarationExplanationState()))
    assertEquals(
        "Cancel explanation",
        explanationActionLabel(
            DeclarationExplanationState(status = DeclarationExplanationStatus.Loading)))
    assertEquals(
        "Refresh explanation",
        explanationActionLabel(
            DeclarationExplanationState(status = DeclarationExplanationStatus.Current)))
  }

  @Test
  fun declarationActionsKeepExplanationPrimaryUntilTheCurrentExplanationCanSupportRefactor() {
    assertEquals(
        DeclarationActionPresentation(ActionTone.Primary, ActionTone.Neutral),
        declarationActionPresentation(DeclarationExplanationState()))
    assertEquals(
        DeclarationActionPresentation(ActionTone.Destructive, ActionTone.Neutral),
        declarationActionPresentation(
            DeclarationExplanationState(status = DeclarationExplanationStatus.Loading)))
    assertEquals(
        DeclarationActionPresentation(ActionTone.Neutral, ActionTone.Primary),
        declarationActionPresentation(
            DeclarationExplanationState(status = DeclarationExplanationStatus.Current)))
    assertEquals(
        DeclarationActionPresentation(ActionTone.Primary, ActionTone.Neutral),
        declarationActionPresentation(
            DeclarationExplanationState(status = DeclarationExplanationStatus.Stale)))
  }

  @Test
  fun declarationExplanationShowsOnlyItsSummaryAndUpdatesWithTheResponse() {
    val result =
        DeclarationExplanation(
            version = "v1",
            projectId = "project",
            projectRevision = "revision",
            baseFileHash = "hash",
            anchor = DeclarationSourceAnchor("internal/main.go", "Run", "func Run() error", 4, 9),
            summary = "**Validate** the request before dispatching work.",
            behavior = listOf("Read `request.ID`.", "Dispatch valid requests."),
            inputs = listOf("The incoming request."),
            errorBehavior = listOf("Missing input returns an error."),
            contextManifest = ContextManifest(model = "model-a", providerOrigin = "local-provider"))
    var state by
        mutableStateOf(
            DeclarationExplanationState(
                status = DeclarationExplanationStatus.Current, result = result))
    ComposeVisualFixture(360, 400, 1.5f) {
          Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState())) {
            DeclarationExplanationDetails(state)
          }
        }
        .use { fixture ->
          fixture.render("context-structured-explanation-360-1.5")
          listOf(
                  "Summary",
                  "Behavior",
                  "Inputs",
                  "Error behavior",
                  "Read request.ID.",
                  "Dispatch valid requests.",
                  "Missing input returns an error.")
              .forEach { assertFalse(fixture.hasText(it), it) }
          fixture.assertTextWrapsWithoutClipping("Validate the request before dispatching work.")
          assertFalse(fixture.hasText("Outputs"))
          assertFalse(fixture.hasText("model-a"))
          assertEquals(1, fixture.scrollableContentCount())
          assertFalse(fixture.hasText("Explanation source"))
          assertTrue(fixture.hasText("Current explanation"))
          assertFalse(
              fixture.hasText(
                  "On-demand · matches the selected loaded source and declaration; not a disk check."))
          state = state.copy(result = result.copy(summary = "Replacement explanation."))
          fixture.render()
          assertFalse(fixture.hasText("model-a"))
          assertTrue(fixture.hasText("Replacement explanation."))
        }
  }

  @Test
  fun explanationRequiresMatchingRequestAndResponseIdentityBeforeShowingProse() {
    val source = file()
    val selected = symbol()
    val project =
        ProjectAnalysis(
            "project",
            "revision",
            "fixture",
            "/tmp/fixture",
            "go",
            fileCount = 1,
            sourceFileCount = 1,
            totalLines = 18,
            summary = "",
            aiStatus = "missing",
            analyzedAt = "")
    val target =
        DeclarationExplanationTarget(
            WorkflowFileIdentity(
                WorkflowProjectIdentity("project", "revision"), source.path, source.contentHash),
            selected.name,
            selected.signature,
            selected.startLine,
            selected.endLine)
    val result =
        DeclarationExplanation(
            "v1",
            "project",
            "revision",
            source.contentHash,
            DeclarationSourceAnchor(
                source.path,
                selected.name,
                selected.signature,
                selected.startLine,
                selected.endLine),
            "**Safe summary**",
            contextManifest = ContextManifest())
    val analysis =
        FileAnalysis(source.path, "fresh", symbolExplanations = mapOf("Run" to "Saved prose"))
    val inspector =
        symbolInspectorUiState(
            source,
            listOf(selected),
            selected,
            analysis,
            false,
            InspectorProviderState(false, false),
            null,
            project)!!
    var state by
        mutableStateOf(
            ContextToolWindowState(
                inspector,
                ScopedModel(),
                false,
                null,
                null,
                fileAnalysis = analysis,
                project = project,
                declarationExplanation =
                    DeclarationExplanationState(
                        DeclarationExplanationStatus.Current, target, result)))
    ComposeVisualFixture(320, 460, 1.5f) {
          ContextToolWindow(state, ContextToolWindowActions({}, {}, {}, {}, {}))
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Current explanation"))
          assertTrue(fixture.hasText("Safe summary"))
          assertFalse(fixture.hasText("Saved prose"))
          val foreignResults =
              listOf(
                  result.copy(projectId = "other"),
                  result.copy(projectRevision = "other"),
                  result.copy(baseFileHash = "other"),
                  result.copy(anchor = result.anchor.copy(path = "other")),
                  result.copy(anchor = result.anchor.copy(symbol = "other")),
                  result.copy(anchor = result.anchor.copy(signature = "other")),
                  result.copy(anchor = result.anchor.copy(startLine = 3)),
                  result.copy(anchor = result.anchor.copy(endLine = 10)))
          foreignResults.forEach { foreign ->
            state =
                state.copy(
                    declarationExplanation = state.declarationExplanation.copy(result = foreign))
            fixture.render()
            assertFalse(fixture.hasText("Current explanation"))
            assertFalse(fixture.hasText("Safe summary"))
            assertTrue(fixture.hasText("Explanation needs refresh"))
            assertTrue(fixture.hasText("Saved file analysis · Fresh"))
          }
          state =
              state.copy(
                  declarationExplanation = state.declarationExplanation.copy(result = result))
          listOf(
                  target.copy(
                      file =
                          target.file.copy(project = WorkflowProjectIdentity("other", "revision"))),
                  target.copy(
                      file =
                          target.file.copy(project = WorkflowProjectIdentity("project", "other"))),
                  target.copy(file = target.file.copy(contentHash = "other")),
                  target.copy(file = target.file.copy(path = "other")),
                  target.copy(symbol = "other"),
                  target.copy(signature = "other"),
                  target.copy(startLine = 3),
                  target.copy(endLine = 10))
              .forEach { foreign ->
                state =
                    state.copy(
                        declarationExplanation =
                            state.declarationExplanation.copy(target = foreign))
                fixture.render()
                assertFalse(fixture.hasText("Current explanation"))
                assertFalse(fixture.hasText("Safe summary"))
              }
        }
  }

  @Test
  fun lifecycleKeepsAttemptVisibleAndLabelsSavedProseWithoutTreatingItAsCurrent() {
    val source = file()
    val selected = symbol()
    val analysis =
        FileAnalysis(source.path, "stale", symbolExplanations = mapOf("Run" to "Saved prose"))
    val project =
        ProjectAnalysis(
            "project",
            "revision",
            "fixture",
            "/tmp/fixture",
            "go",
            fileCount = 1,
            sourceFileCount = 1,
            totalLines = 18,
            summary = "",
            aiStatus = "missing",
            analyzedAt = "")
    val inspector =
        symbolInspectorUiState(
            source,
            listOf(selected),
            selected,
            analysis,
            false,
            InspectorProviderState(false, false),
            null,
            project)!!
    val target =
        DeclarationExplanationTarget(
            WorkflowFileIdentity(
                WorkflowProjectIdentity("project", "revision"), source.path, source.contentHash),
            selected.name,
            selected.signature,
            selected.startLine,
            selected.endLine)
    var state by
        mutableStateOf(
            ContextToolWindowState(
                inspector,
                ScopedModel(),
                false,
                null,
                null,
                fileAnalysis = analysis,
                project = project))
    val actions = ContextToolWindowActions({}, {}, {}, {}, {})
    ComposeVisualFixture(320, 420, 1.5f) { ContextToolWindow(state, actions) }
        .use { fixture ->
          listOf(
                  Triple(
                      DeclarationExplanationStatus.Unavailable,
                      "No explanation yet",
                      "No on-demand explanation has been requested."),
                  Triple(DeclarationExplanationStatus.Loading, "Explaining…", "Explaining Run…"),
                  Triple(
                      DeclarationExplanationStatus.Stale,
                      "Explanation needs refresh",
                      "The project changed. Request a fresh explanation."),
                  Triple(
                      DeclarationExplanationStatus.Canceled,
                      "Explanation canceled",
                      "Explanation canceled."),
                  Triple(
                      DeclarationExplanationStatus.Failed,
                      "Explanation failed",
                      "Provider unavailable. Retry."))
              .forEach { (status, label, message) ->
                state =
                    state.copy(
                        declarationExplanation =
                            DeclarationExplanationState(status, target, message = message))
                fixture.render()
                assertTrue(fixture.hasText(label))
                assertTrue(fixture.hasText(message))
                assertTrue(fixture.hasText("Saved file analysis · Stale"))
                assertTrue(
                    fixture.hasText(
                        "Source/index comparison unavailable · not an on-demand explanation"))
                assertTrue(fixture.hasText("Saved prose"))
                assertTrue(
                    fixture.hasText(
                        if (status == DeclarationExplanationStatus.Loading) "Cancel explanation"
                        else "Explain declaration"))
              }
          state =
              state.copy(
                  inspector =
                      inspector.copy(
                          analysisStatus = InspectorAnalysisStatus.Fresh,
                          sourceIndexCorrespondence = SourceIndexCorrespondence.Changed),
                  fileAnalysis = analysis.copy(status = "fresh"),
                  declarationExplanation = DeclarationExplanationState())
          fixture.render()
          assertTrue(fixture.hasText("Saved file analysis · Fresh"))
          assertTrue(
              fixture.hasText(
                  "Loaded source differs from indexed source · not an on-demand explanation"))
          state = state.copy(fileAnalysis = analysis.copy(path = "other"))
          fixture.render()
          assertFalse(fixture.hasText("Saved prose"))
        }
  }

  @Test
  fun functionConsentAndExplanationActionsRemainExplicitAndRefactorIndependent() {
    val project =
        ProjectAnalysis(
            "project",
            "revision",
            "fixture",
            "/tmp/fixture",
            "go",
            fileCount = 1,
            sourceFileCount = 1,
            totalLines = 18,
            summary = "",
            aiStatus = "missing",
            analyzedAt = "")
    val inspector = inspector(file(), selectedSymbol = symbol())
    val model =
        ScopedModel(
            scope = "function",
            profile = "review",
            model = "provider/long-model",
            remoteProvider = true)
    val target =
        DeclarationExplanationTarget(
            WorkflowFileIdentity(
                WorkflowProjectIdentity("project", "revision"), file().path, file().contentHash),
            symbol().name,
            symbol().signature,
            symbol().startLine,
            symbol().endLine)
    var state by
        mutableStateOf(
            ContextToolWindowState(
                inspector,
                ScopedModel(),
                false,
                null,
                null,
                project = project,
                functionModel = model))
    val confirmations = mutableListOf<Boolean>()
    var requests = 0
    var cancellations = 0
    var refactors = 0
    val actions =
        ContextToolWindowActions(
            {},
            {},
            {},
            {},
            { refactors++ },
            confirmFunctionRemoteProvider = {
              confirmations += it
              state = state.copy(functionRemoteProviderConfirmed = it)
            },
            explainSelected = { requests++ },
            cancelExplanation = { cancellations++ })
    ComposeVisualFixture(320, 420, 1.5f) { ContextToolWindow(state, actions) }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText(modelDestinationLabel(ModelScope.Function, model)))
          assertTrue(fixture.hasText("Confirm remote destination"))
          assertTrue(fixture.isDisabled("Explain declaration"))
          assertFalse(fixture.tryClick("Explain declaration"))
          assertEquals(0, requests)
          fixture.clickText("Refactor")
          assertEquals(1, refactors)
          fixture.clickDescription("Confirm remote destination")
          fixture.render()
          assertEquals(listOf(true), confirmations)
          assertEquals(0, requests)
          assertFalse(fixture.isDisabled("Explain declaration"))
          fixture.clickText("Explain declaration")
          assertEquals(1, requests)
          state =
              state.copy(
                  functionRemoteProviderConfirmed = false,
                  declarationExplanation =
                      DeclarationExplanationState(DeclarationExplanationStatus.Loading, target))
          fixture.render()
          assertTrue(fixture.hasText(modelDestinationLabel(ModelScope.Function, model)))
          assertTrue(fixture.isDescriptionDisabled("Confirm remote destination"))
          assertFalse(fixture.isDisabled("Cancel explanation"))
          fixture.clickText("Cancel explanation")
          assertEquals(1, cancellations)
          assertEquals(1, requests)
          fixture.clickText("Refactor")
          assertEquals(2, refactors)
          state =
              state.copy(
                  declarationExplanation =
                      DeclarationExplanationState(
                          DeclarationExplanationStatus.Failed, target, message = "Provider failed"))
          fixture.render()
          assertTrue(fixture.isDisabled("Explain declaration"))
          fixture.clickText("Refactor")
          assertEquals(3, refactors)
          assertEquals(listOf(true), confirmations)
        }
  }

  @Test
  fun declarationIdentityIsBoundedSelectableAndBlockedTargetsRemainInspectable() {
    val longPath = "internal/" + "nested/".repeat(18) + "handler.go"
    val signature = "func Run(" + "requestID string, ".repeat(12) + ") error"
    val loaded = file().copy(path = longPath, content = (1..6).joinToString("\n") { "line $it" })
    val exact = symbol().copy(signature = signature, startLine = 4, endLine = 900)
    val approximate = exact.copy(confidence = "approximate", atomicTarget = false)
    val grouped = exact.copy(atomicTarget = false)
    val cases =
        listOf(
            Triple(loaded, exact, "Exact atomic target · editable"),
            Triple(loaded, approximate, "Approximate indexed declaration · read-only"),
            Triple(loaded, grouped, "Exact indexed declaration · read-only"),
            Triple(
                loaded.copy(language = "Python"), exact, "Exact indexed declaration · read-only"))
    var calls = 0
    val actions =
        ContextToolWindowActions(
            {}, { calls++ }, {}, {}, { calls++ }, explainSelected = { calls++ })
    cases.forEach { (source, target, confidence) ->
      val inspector =
          symbolInspectorUiState(
              source,
              listOf(target),
              target,
              null,
              false,
              InspectorProviderState(false, false),
              null)!!
      ComposeVisualFixture(280, 400, 1.5f) {
            ContextToolWindow(
                ContextToolWindowState(inspector, ScopedModel(), false, null, null), actions)
          }
          .use { fixture ->
            fixture.render()
            fixture.assertTextWrapsWithoutClipping(longPath)
            fixture.assertTextFits("Lines 4–6 · ${source.language}")
            assertTrue(fixture.hasText(confidence))
            assertTrue(fixture.hasText("Source/index comparison unavailable"))
            if (!inspector.selectedSymbol!!.editEligibility.eligible) {
              assertTrue(fixture.hasText(inspector.selectedSymbol.editEligibility.blockedReason))
              assertFalse(fixture.hasText("Refactor"))
            }
            fixture.clickText("Declaration details")
            fixture.render()
            assertTrue(fixture.hasText(signature))
          }
    }
    val missing =
        symbolInspectorUiState(
            loaded, emptyList(), exact, null, false, InspectorProviderState(false, false), null)!!
    ComposeVisualFixture(320, 400, 1f) {
          ContextToolWindow(
              ContextToolWindowState(missing, ScopedModel(), false, null, null), actions)
        }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText(exact.name))
          assertTrue(fixture.hasText(missing.selectedSymbol!!.editEligibility.blockedReason))
          assertFalse(fixture.hasText("Refactor"))
          fixture.clickText("Declaration details")
          fixture.render()
          assertTrue(fixture.hasText(signature))
        }
    assertEquals(0, calls)
  }

  @Test
  fun declarationDetailsResetOnProjectChangeEvenWithTheSameFileAndSymbol() {
    val source = file()
    val target = symbol()
    val inspector = inspector(source, selectedSymbol = target)
    val project =
        ProjectAnalysis(
            "first",
            "revision",
            "fixture",
            "/tmp/fixture",
            "go",
            fileCount = 1,
            sourceFileCount = 1,
            totalLines = 18,
            summary = "",
            aiStatus = "missing",
            analyzedAt = "")
    var state by
        mutableStateOf(
            ContextToolWindowState(inspector, ScopedModel(), false, null, null, project = project))
    var calls = 0
    val actions =
        ContextToolWindowActions(
            {}, { calls++ }, {}, {}, { calls++ }, explainSelected = { calls++ })
    ComposeVisualFixture(320, 400, 1f) { ContextToolWindow(state, actions) }
        .use { fixture ->
          fixture.render()
          assertFalse(fixture.hasText(target.signature))
          fixture.clickText("Declaration details")
          fixture.render()
          assertTrue(fixture.hasText(target.signature))
          fixture.render() // Recomposition with the same identity keeps the disclosure open.
          assertTrue(fixture.hasText(target.signature))
          state = state.copy(project = project.copy(projectId = "second"))
          fixture.render()
          assertFalse(fixture.hasText(target.signature))
          fixture.clickText("Declaration details")
          fixture.render()
          assertTrue(fixture.hasText(target.signature))
          state = state.copy(project = project.copy(projectRevision = "new-revision"))
          fixture.render()
          assertFalse(fixture.hasText(target.signature))
          fixture.clickText("Declaration details")
          fixture.render()
          state =
              state.copy(inspector = inspector.copy(file = source.copy(contentHash = "changed")))
          fixture.render()
          assertFalse(fixture.hasText(target.signature))
        }
    assertEquals(0, calls)
  }

  @Test
  fun selectedDeclarationDisclosuresExposeLocalFileAndProjectEvidenceWithoutActions() {
    val source = file()
    val target = symbol()
    val analysis =
        FileAnalysis(
            source.path,
            "stale",
            purpose = "Coordinates requests.",
            engineeringInsight = EngineeringInsight(mechanism = "Validates inputs locally."))
    val project =
        ProjectAnalysis(
            "first",
            "revision",
            "fixture",
            "/tmp/fixture",
            "go",
            fileCount = 1,
            sourceFileCount = 1,
            totalLines = 18,
            summary = "",
            aiStatus = "missing",
            analyzedAt = "")
    val overview =
        ProjectOverview(
            projectId = "first",
            projectRevision = "revision",
            analysis =
                StructuredProjectAnalysis(status = "stale", purpose = "Serve local requests."))
    var state by
        mutableStateOf(
            ContextToolWindowState(
                inspector(source, selectedSymbol = target),
                ScopedModel(),
                false,
                null,
                GitStatus(true, "main", "modified", "unstaged"),
                fileAnalysis = analysis,
                project = project,
                overview = overview))
    var privilegedActions = 0
    val actions =
        ContextToolWindowActions(
            { privilegedActions++ },
            { privilegedActions++ },
            { privilegedActions++ },
            { privilegedActions++ },
            { privilegedActions++ },
            explainSelected = { privilegedActions++ })
    ComposeVisualFixture(320, 420, 1f) { ContextToolWindow(state, actions) }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("File details"))
          assertTrue(fixture.hasText("Project context"))
          assertTrue(fixture.hasText("File context"))
          assertFalse(fixture.hasText("Coordinates requests."))
          assertFalse(fixture.hasText("Serve local requests."))
          assertFalse(fixture.hasText("main · modified"))
          fixture.clickText("File details")
          fixture.render()
          assertTrue(fixture.hasText("Coordinates requests."))
          assertTrue(fixture.hasText("AI interpretation · File · stale"))
          assertTrue(fixture.hasText("Validates inputs locally."))
          fixture.clickText("Project context")
          fixture.render()
          assertTrue(fixture.hasText("Serve local requests."))
          assertTrue(fixture.hasText("Stale"))
          fixture.clickText("File context")
          fixture.render()
          assertTrue(fixture.hasText("main · modified · unstaged · read-only context"))
          fixture.render()
          assertTrue(fixture.hasText("Validates inputs locally."))
          assertTrue(fixture.hasText("Serve local requests."))
          assertTrue(fixture.requestDescriptionFocus("Collapse Project context"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertFalse(fixture.hasText("Serve local requests."))
          assertTrue(fixture.pressKey(Key.Enter))
          fixture.render()
          assertTrue(fixture.hasText("Serve local requests."))
          state = state.copy(project = project.copy(projectId = "second"))
          fixture.render()
          assertFalse(fixture.hasText("Validates inputs locally."))
          assertFalse(fixture.hasText("Serve local requests."))
          assertFalse(fixture.hasText("main · modified"))
          fixture.clickText("File details")
          fixture.render()
          assertTrue(fixture.hasText("Coordinates requests."))
          state =
              state.copy(inspector = inspector(source, selectedSymbol = target.copy(name = "Stop")))
          fixture.render()
          assertFalse(fixture.hasText("Coordinates requests."))
          fixture.clickText("File details")
          fixture.render()
          assertTrue(fixture.hasText("Coordinates requests."))
          state =
              state.copy(
                  inspector =
                      inspector(
                          source.copy(path = "other.go"),
                          selectedSymbol = target.copy(name = "Stop")))
          fixture.render()
          assertFalse(fixture.hasText("Coordinates requests."))
          assertFalse(fixture.hasText("Validates inputs locally."))
        }
    assertEquals(0, privilegedActions)
  }

  @Test
  fun referencesShowOnlyMatchingLoadedEvidenceAndNeverDispatchOnDisclosure() {
    val source = file()
    val longPath = "internal/" + "nested/".repeat(16) + "caller.go"
    val longReason = "Potential indexed relationship: " + "through an adapter ".repeat(5)
    val rows =
        listOf(
            ImpactReference(longPath, "CallRun", "approximate", longReason),
            ImpactReference("other/consumer.go", "", "exact", "Mentions the source file."))
    var state by
        mutableStateOf(
            ContextToolWindowState(
                inspector(source, selectedSymbol = symbol()), ScopedModel(), false, null, null))
    var privilegedActions = 0
    val actions =
        ContextToolWindowActions(
            { privilegedActions++ },
            { privilegedActions++ },
            { privilegedActions++ },
            { privilegedActions++ },
            { privilegedActions++ },
            explainSelected = { privilegedActions++ },
            cancelExplanation = { privilegedActions++ })
    ComposeVisualFixture(320, 420, 1.5f) { ContextToolWindow(state, actions) }
        .use { fixture ->
          fixture.render()
          assertTrue(fixture.hasText("Reference preview unavailable."))
          assertFalse(fixture.hasText(longPath))
          state = state.copy(impact = ImpactPreview(source.path, references = rows))
          fixture.render()
          assertTrue(
              fixture.hasText(
                  "File-scoped reference preview · advisory indexed relationships, not runtime callers."))
          assertFalse(fixture.hasText(longPath))
          fixture.clickText("References")
          fixture.render()
          fixture.revealTextFullyWithin(longReason, "context-content")
          fixture.assertTextWrapsWithoutClipping(longReason)
          assertTrue(fixture.hasText(longPath))
          assertTrue(fixture.hasText("CallRun"))
          assertTrue(fixture.hasText("Confidence · approximate"))
          assertTrue(fixture.hasText("other/consumer.go"))
          assertTrue(fixture.hasText("Confidence · exact"))
          assertTrue(fixture.hasText("Mentions the source file."))
          fixture.render()
          assertTrue(fixture.hasText(longReason))
          assertTrue(fixture.requestDescriptionFocus("Collapse References"))
          assertTrue(fixture.pressKey(Key.Spacebar))
          fixture.render()
          assertFalse(fixture.hasText(longReason))
          state = state.copy(impact = ImpactPreview(source.path))
          fixture.render()
          assertTrue(
              fixture.hasText(
                  "File-scoped reference preview has no indexed references; runtime callers are unknown."))
          state = state.copy(impact = ImpactPreview("other.go", references = rows))
          fixture.render()
          assertTrue(
              fixture.hasText(
                  "Reference preview does not match the selected file and declaration."))
          assertFalse(fixture.hasText(longPath))
          state = state.copy(impact = ImpactPreview(source.path, "Other", rows))
          fixture.render()
          assertTrue(
              fixture.hasText(
                  "Reference preview does not match the selected file and declaration."))
          assertFalse(fixture.hasText(longPath))
          state = state.copy(impact = ImpactPreview(source.path, "Run", rows))
          fixture.render()
          assertTrue(
              fixture.hasText(
                  "File-scoped reference preview · advisory indexed relationships, not runtime callers."))
          fixture.clickText("References")
          fixture.render()
          assertTrue(fixture.hasText(longPath))
          state =
              state.copy(
                  inspector = inspector(source, selectedSymbol = symbol().copy(name = "Stop")))
          fixture.render()
          assertFalse(fixture.hasText(longPath))
          assertTrue(
              fixture.hasText(
                  "Reference preview does not match the selected file and declaration."))
        }
    assertEquals(0, privilegedActions)
  }

  @Test
  fun fileFallbackKeepsActionsAndDetailsWithoutMisreportingUnselectedSymbols() {
    var calls = 0
    val actions =
        ContextToolWindowActions(
            {}, { calls++ }, {}, {}, { calls++ }, explainSelected = { calls++ })
    val noSymbols = "No indexed declarations in this file. File details remain available."
    val unselected = "Select a declaration in the editor to inspect it."
    listOf(emptyList(), listOf(symbol())).forEach { symbols ->
      val inspector =
          symbolInspectorUiState(
              file(), symbols, null, null, false, InspectorProviderState(false, false), null)!!
      ComposeVisualFixture(320, 400, 1f) {
            ContextToolWindow(
                ContextToolWindowState(inspector, ScopedModel(), false, null, null), actions)
          }
          .use { fixture ->
            fixture.render()
            assertTrue(fixture.hasText("Actions"))
            assertTrue(fixture.hasText("Analyze project"))
            assertTrue(fixture.hasText(if (symbols.isEmpty()) noSymbols else unselected))
            assertFalse(fixture.hasText(if (symbols.isEmpty()) unselected else noSymbols))
            fixture.clickText("Details")
            fixture.render()
            assertTrue(fixture.hasText(file().path))
            assertTrue(fixture.hasText("Source/index comparison unavailable"))
            assertFalse(fixture.hasText("Explain declaration"))
          }
    }
    assertEquals(0, calls)
  }

  @Test
  fun contextSynchronizesBetweenFileAndExactDeclarationWithoutChangingSelectionState() {
    val file = file()
    val symbol = symbol()
    val fileContext = inspector(file, selectedSymbol = null)
    val declarationContext = inspector(file, selectedSymbol = symbol)

    assertEquals(SymbolInspectorMode.FileFallback, fileContext.mode)
    assertEquals("File context", contextHeaderLabel(fileContext))
    assertNull(contextStateBadge(fileContext))
    assertEquals(SymbolInspectorMode.SelectedSymbol, declarationContext.mode)
    assertEquals("Declaration · Run", contextHeaderLabel(declarationContext))
    assertTrue(contextToolWindowDescription(declarationContext).contains(file.path))
    assertTrue(contextToolWindowDescription(declarationContext).contains("Fresh"))
  }

  @Test
  fun contextKeepsRemoteConfirmationAdjacentToAnExplicitAnalysisAction() {
    val pending =
        symbolInspectorUiState(
            selectedFile = file(),
            symbols = listOf(symbol()),
            selectedSymbol = symbol(),
            analysis = FileAnalysis("internal/main.go", "stale"),
            analysisInProgress = false,
            provider =
                InspectorProviderState(remoteProvider = true, remoteProviderConfirmed = false),
            currentEditIdentity = null,
        )!!

    assertEquals(InspectorAnalysisAction.RefreshAnalysis, pending.analysisAction)
    assertTrue(pending.remoteProviderConfirmationRequired)
    assertTrue(contextToolWindowDescription(pending).contains("Stale"))
  }

  @Test
  fun rightToolWindowTabsAreTextualAndOnlyExplicitSelectionChangesTheirLayoutState() {
    val contextLayout = DesktopLayoutState(rightToolWindowVisible = false)

    assertEquals("Context", rightToolWindowLabel(RightToolWindow.Context))
    assertEquals(
        "Assistant tool window tab, not selected",
        rightToolWindowTabDescription(RightToolWindow.Assistant, selected = false))
    assertEquals(RightToolWindow.Context, contextLayout.activeRightToolWindow)
    assertFalse(contextLayout.rightToolWindowVisible)

    val reviewLayout =
        contextLayout
            .openRight(RightToolWindow.Review)
            .withFocus(DesktopFocusRegion.RightToolWindow)

    assertEquals(RightToolWindow.Review, reviewLayout.activeRightToolWindow)
    assertTrue(reviewLayout.rightToolWindowVisible)
    assertEquals(DesktopFocusRegion.RightToolWindow, reviewLayout.lastFocusedRegion)
  }

  @Test
  fun contextBadgesExposeBoundDraftStateWithoutAuthorizingAnyAction() {
    val draftIdentity =
        CurrentEditIdentity(ChatEditMode.ReplaceSymbol, "internal/main.go", "Run", true)
    val sessionIdentity = draftIdentity.copy(hasDraft = false)

    assertEquals(
        "CURRENT DRAFT · Run", contextStateBadge(inspector(file(), current = draftIdentity)))
    assertEquals(
        "BOUND CONVERSATION · Run", contextStateBadge(inspector(file(), current = sessionIdentity)))
    assertFalse(
        contextStateBadge(inspector(file(), current = draftIdentity)).orEmpty().contains("Apply"))
  }

  @Test
  fun contextProjectSummaryUsesTheSharedPurposeAndFreshnessPresentation() {
    val overview =
        ProjectOverview(
            analysis =
                StructuredProjectAnalysis(
                    status = "stale", purpose = "Keep request boundaries explicit."))

    val summary = projectSummaryPresentation(overview, null)

    assertEquals("Keep request boundaries explicit.", summary.purpose)
    assertEquals("stale", summary.analysisStatus)
    assertTrue(summary.analysisMessage.contains("stale"))
  }

  private fun inspector(
      file: ProjectFileInfo,
      selectedSymbol: SymbolInfo? = null,
      current: CurrentEditIdentity? = null,
  ) =
      symbolInspectorUiState(
          selectedFile = file,
          symbols = listOf(symbol()),
          selectedSymbol = selectedSymbol,
          analysis = FileAnalysis(file.path, "fresh", purpose = "Coordinates requests."),
          analysisInProgress = false,
          provider =
              InspectorProviderState(remoteProvider = false, remoteProviderConfirmed = false),
          currentEditIdentity = current,
      )!!

  private fun file() =
      ProjectFileInfo(
          path = "internal/main.go",
          contentHash = "hash",
          name = "main.go",
          language = "Go",
          sizeBytes = 256,
          lineCount = 18,
          modifiedAt = "",
          binary = false,
      )

  private fun symbol() =
      SymbolInfo(
          name = "Run",
          kind = "function",
          signature = "func Run() error",
          startLine = 4,
          endLine = 9,
          confidence = "exact",
          atomicTarget = true,
      )
}
