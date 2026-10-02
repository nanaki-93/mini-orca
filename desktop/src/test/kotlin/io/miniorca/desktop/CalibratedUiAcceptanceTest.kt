package io.miniorca.desktop

import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.width
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class CalibratedUiAcceptanceTest {
  @Test
  fun reviewAndReceiptActionsRemainTruthfulAcrossWindowTextAndDisplaySizes() {
    val base = editorComparisonReviewFixture()
    val scope =
        AppliedDeclarationScope(
            base.project!!.projectId,
            base.selected!!.path,
            base.draft!!.targetSymbol,
            DraftMutationOperation.Apply)
    val receipt =
        base.copy(
            project = base.project.copy(projectRevision = "next"),
            selected = base.selected.copy(contentHash = "after"),
            applied = ApplyResult("next", "after", true),
            receiptScope = scope)
    val states =
        listOf(
            "ready" to base,
            "applying" to
                base.copy(
                    mutation =
                        DraftMutationAttempt(
                            DraftMutationOperation.Apply,
                            DraftMutationStatus.Running,
                            "Waiting for the guarded Apply receipt.")),
            "missing-checks" to base.copy(checks = null),
            "receipt" to receipt,
            "undo-failed" to
                receipt.copy(
                    mutation =
                        DraftMutationAttempt(
                            DraftMutationOperation.Undo,
                            DraftMutationStatus.Failed,
                            "Undo request failed; source has not been confirmed restored.")),
            "undo-conflict" to
                receipt.copy(
                    mutation =
                        DraftMutationAttempt(
                            DraftMutationOperation.Undo,
                            DraftMutationStatus.Conflict,
                            "Source changed externally; Undo is blocked.")),
            "undone" to
                receipt.copy(receiptScope = scope.copy(operation = DraftMutationOperation.Undo)))
    for ((width, height) in
        listOf(1600 to 1000, 1440 to 900, 1024 to 768, 800 to 650, 1280 to 600)) {
      for (scale in listOf(1f, 1.25f, 1.5f)) {
        for (density in if (width == 800 || width == 1440) listOf(1f, 2f) else listOf(1f)) {
          var state by mutableStateOf(base)
          var actions = 0
          ComposeVisualFixture(
                  (width * density).toInt(), (height * density).toInt(), scale, density) {
                    Row(Modifier.fillMaxSize()) {
                      ReviewDiffCanvas(state.draft, Modifier.weight(1f))
                      ReviewToolWindow(
                          state,
                          ReviewToolWindowActions({ actions++ }, { actions++ }, { actions++ }),
                          DraftApplicationActions({ actions++ }, { actions++ }),
                          Modifier.width(340.dp))
                    }
                  }
              .use { fixture ->
                for ((name, next) in states) {
                  state = next
                  fixture.render(
                      if (width == 800 && scale == 1.5f)
                          "f38-$name-$width-$height-$scale-${density}x"
                      else null)
                  assertFalse(fixture.hasEditableText("Read-only composed diff"))
                  assertEquals(name == "ready", fixture.hasText("Apply change"), name)
                  assertEquals(
                      name in listOf("receipt", "undo-failed"),
                      fixture.hasText("Undo this change"),
                      name)
                  when (name) {
                    "ready" -> fixture.revealTextFullyWithin("Apply change", "review-action-scroll")
                    "receipt",
                    "undo-failed" -> {
                      assertTrue(fixture.hasText("Change applied"))
                      fixture.revealTextFullyWithin("Undo this change", "review-action-scroll")
                    }
                    "undo-conflict",
                    "undone" -> {
                      assertTrue(fixture.isDisabled("Undo is no longer available"))
                      fixture.revealTextFullyWithin(
                          "Undo is no longer available", "review-action-scroll")
                    }
                  }
                  assertEquals(0, actions, "Rendering or disclosing state cannot execute an action")
                }
              }
        }
      }
    }
  }
}
