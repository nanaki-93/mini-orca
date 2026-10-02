package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
internal fun DraftValidationStage(editor: EditableDraftState, actions: DraftEditorActions) {
  val running = editor.status == DraftEditorStatus.Validating
  val canValidate =
      editor.status in
          setOf(DraftEditorStatus.Generated, DraftEditorStatus.Dirty, DraftEditorStatus.Invalid)
  Column(Modifier.fillMaxWidth().padding(top = 8.dp)) {
    IdePaneHeader(
        title = "Validation",
        stateLabel =
            "Revision ${editor.serverDraft.revision}" +
                if (editor.unvalidatedLocalEdits) " + local edits" else "",
        stateTint = draftEditorStatusColor(editor.status))
    SelectionContainer {
      Text(
          draftEditorStatusMessage(editor.status),
          color = draftEditorStatusColor(editor.status),
          style = IdeTypography.body)
    }
    DraftValidationDiagnostics(editor.diagnostics, draftDiagnosticsAreEarlierEvidence(editor))
    editor.validationAttempt
        ?.takeIf { it.status != ValidationAttemptStatus.Running }
        ?.let { DiagnosticText("Validation ${it.status.name.lowercase()}: ${it.message}") }
    if (editor.status == DraftEditorStatus.Valid) {
      Text(
          "Validation composes this candidate in memory. It does not run tests or write source.",
          color = SecondaryText,
          style = IdeTypography.body)
      actions.review?.let { review ->
        MiniOrcaButton(
            onClick = review,
            tone = ActionTone.Primary,
            modifier = Modifier.fillMaxWidth().padding(top = 7.dp)) {
              Text("Review focused checks")
            }
      }
    }
    MiniOrcaButton(
        onClick = actions.validate,
        enabled = canValidate,
        tone = ActionTone.Primary,
        modifier = Modifier.fillMaxWidth().padding(top = 7.dp)) {
          Text(
              if (running) "Validating declaration…"
              else "Validate draft for ${editor.serverDraft.targetSymbol}")
        }
    if (running)
        actions.cancelValidation?.let { cancel ->
          MiniOrcaButton(onClick = cancel, modifier = Modifier.fillMaxWidth().padding(top = 5.dp)) {
            Text("Cancel validation")
          }
        }
  }
}
