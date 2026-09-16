package io.miniorca.desktop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** Presentation-only labels for workflow state; guards remain owned by existing state reducers. */
internal data class RightToolWindowBadge(val label: String)

internal fun workflowToolWindowBadges(
    editor: EditableDraftState?,
    evidence: ReviewEvidenceUiState,
    decision: ApplyDecisionUiState,
): Map<RightToolWindow, RightToolWindowBadge> = buildMap {
  assistantToolWindowBadge(editor)?.let { put(RightToolWindow.Assistant, it) }
  reviewToolWindowBadge(evidence, decision)?.let { put(RightToolWindow.Review, it) }
}

private fun assistantToolWindowBadge(editor: EditableDraftState?): RightToolWindowBadge? =
    when (editor?.status) {
      null -> null
      DraftEditorStatus.Invalid,
      DraftEditorStatus.Stale -> RightToolWindowBadge("Invalid")
      else -> RightToolWindowBadge("Draft")
    }

private fun reviewToolWindowBadge(
    evidence: ReviewEvidenceUiState,
    decision: ApplyDecisionUiState,
): RightToolWindowBadge? =
    when {
      decision.receiptTitle != null -> RightToolWindowBadge("Applied")
      decision.eligible -> RightToolWindowBadge("Ready to apply")
      evidence.checks.status == ReviewEvidenceStatus.Failed -> RightToolWindowBadge("Checks failed")
      else -> null
    }

internal data class ToolWindowScope(
    val path: String,
    val target: String,
)

internal enum class DeclarationCreationKind(val noun: String) {
  Function("function"),
  Type("type"),
}

internal fun assistantToolWindowScope(
    selected: ProjectFileInfo?,
    target: ChatTarget?,
    draft: DeclarationDraft?,
    newSymbol: String,
    mode: ChatEditMode = target?.mode ?: ChatEditMode.ReplaceSymbol,
    creationKind: DeclarationCreationKind = DeclarationCreationKind.Function,
): ToolWindowScope =
    ToolWindowScope(
        path =
            selected?.path
                ?: draft?.targetPath?.takeIf { mode != ChatEditMode.CreateSymbol }
                ?: "No file selected",
        target =
            if (mode == ChatEditMode.CreateSymbol)
                "New ${creationKind.noun} · ${target?.symbol ?: newSymbol.trim().ifBlank { "Name required" }}"
            else
                target?.let { "${it.mode.label} · ${it.symbol}" }
                    ?: draft?.let { "${it.mode} · ${it.targetSymbol}" }
                    ?: newSymbol.trim().takeIf(String::isNotBlank)?.let { "New declaration · $it" }
                    ?: "No declaration target",
    )

internal fun reviewToolWindowScope(
    selected: ProjectFileInfo?,
    selectedSymbol: SymbolInfo?,
    draft: DeclarationDraft?,
    applied: ApplyResult?,
): ToolWindowScope =
    ToolWindowScope(
        path =
            applied?.audit?.targetPath?.takeIf(String::isNotBlank)
                ?: draft?.targetPath
                ?: selected?.path
                ?: "No file selected",
        target = draft?.targetSymbol ?: selectedSymbol?.name ?: "No declaration target",
    )

@Composable
internal fun ReviewTargetHeader(scope: ToolWindowScope, onEditDraft: (() -> Unit)?) {
  Column(Modifier.fillMaxWidth().padding(12.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
    Row(
        Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
        verticalAlignment = Alignment.CenterVertically) {
          Text(
              scope.target,
              color = PrimaryText,
              style = IdeTypography.workspaceHeading,
              modifier = Modifier.weight(1f))
          if (onEditDraft != null)
              MiniOrcaButton(onClick = onEditDraft, tone = ActionTone.Neutral) {
                Text("Edit draft", style = IdeTypography.action)
              }
        }
    Text(
        scope.path,
        color = SecondaryText,
        style = IdeTypography.workspaceMetadata,
        modifier = Modifier.semantics { contentDescription = "Review target path: ${scope.path}" })
  }
}

@Composable
internal fun ToolWindowScopeHeader(
    label: String,
    scope: ToolWindowScope,
    modifier: Modifier = Modifier,
) {
  Column(modifier.fillMaxWidth()) {
    IdePaneHeader(title = label, stateLabel = scope.target)
    Text(
        scope.path,
        color = PrimaryText,
        fontFamily = FontFamily.Monospace,
        fontSize = 11.sp,
        modifier = Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp))
    IdeHorizontalSeparator()
  }
}
