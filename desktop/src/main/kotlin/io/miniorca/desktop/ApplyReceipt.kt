package io.miniorca.desktop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

/** Captured from the explicitly submitted operation, never from a later selection. */
data class AppliedDeclarationScope(
    val projectId: String,
    val path: String,
    val symbol: String,
    val operation: DraftMutationOperation,
)

internal fun undoEligibility(
    project: ProjectAnalysis?,
    file: ProjectFileInfo?,
    result: ApplyResult?,
    scope: AppliedDeclarationScope?,
    mutation: DraftMutationAttempt? = null,
    refreshError: String? = null,
): ApplyEligibility =
    when {
      result == null -> ApplyEligibility(false, "No Apply receipt is available.")
      scope?.operation == DraftMutationOperation.Undo ||
          result.audit?.action.equals("undo", true) ->
          ApplyEligibility(false, "This change was already undone. There is no older Undo chain.")
      !result.undoAvailable ->
          ApplyEligibility(false, "The daemon reports Undo is no longer available.")
      mutation?.operation == DraftMutationOperation.Undo &&
          mutation.status == DraftMutationStatus.Conflict ->
          ApplyEligibility(false, mutation.message)
      refreshError != null -> ApplyEligibility(false, "Refresh source before Undo: $refreshError")
      scope == null -> ApplyEligibility(false, "The applied declaration scope is unavailable.")
      project?.projectId != scope.projectId || file?.path != scope.path ->
          ApplyEligibility(false, "Refresh the applied file before Undo.")
      project.projectRevision != result.projectRevision ||
          file.contentHash != result.postApplyHash ->
          ApplyEligibility(
              false,
              "Source or project identity differs from this receipt. Refresh source; external changes block Undo.")
      else -> ApplyEligibility(true, "Undo only this immediately preceding unchanged Apply.")
    }

internal fun validateMutationReceipt(
    result: ApplyResult,
    scope: AppliedDeclarationScope,
) {
  require(result.projectRevision.isNotBlank() && result.postApplyHash.isNotBlank()) {
    "The daemon returned an incomplete operation receipt. Refresh source before continuing."
  }
  result.audit?.let { audit ->
    require(
        audit.targetPath == scope.path &&
            audit.action.equals(scope.operation.name, true) &&
            (audit.projectId.isBlank() || audit.projectId == scope.projectId) &&
            (audit.projectRevision.isBlank() || audit.projectRevision == result.projectRevision) &&
            (audit.afterHash.isBlank() || audit.afterHash == result.postApplyHash)) {
          "The returned audit does not match the submitted operation. Refresh source before continuing."
        }
    require(
        audit.outcome ==
            if (scope.operation == DraftMutationOperation.Apply) "applied" else "undone") {
          "The returned audit does not confirm a successful operation: ${audit.outcome}"
        }
  }
}

@Composable
internal fun ApplyReceiptDetails(result: ApplyResult, scope: AppliedDeclarationScope?) {
  SelectionContainer {
    Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
      scope?.let {
        Text(
            "${it.symbol} · ${it.path}",
            color = PrimaryText,
            style = IdeTypography.workspaceMetadata)
      }
      Text(
          "Project revision: ${result.projectRevision}",
          color = SecondaryText,
          style = IdeTypography.resultCode)
      if (result.postApplyHash.isNotBlank())
          Text(
              "Resulting source hash: ${result.postApplyHash}",
              color = SecondaryText,
              style = IdeTypography.resultCode)
      result.audit?.let { audit ->
        Text(
            "Audit: ${audit.action} · ${audit.outcome}",
            color = PrimaryText,
            style = IdeTypography.workspaceMetadata)
        listOf(
                "Audit ID" to audit.id,
                "Recorded at" to audit.timestamp,
                "Draft ID" to audit.generationId,
                "Before hash" to audit.beforeHash,
                "After hash" to audit.afterHash)
            .filter { it.second.isNotBlank() }
            .forEach { (label, value) ->
              Text("$label: $value", color = SecondaryText, style = IdeTypography.resultCode)
            }
      }
    }
  }
}
