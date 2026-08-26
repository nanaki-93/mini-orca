package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.AlertDialog
import androidx.compose.material.Button
import androidx.compose.material.ButtonDefaults
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
internal fun DraftReviewPane(
    project: ProjectAnalysis?,
    selected: ProjectFileInfo?,
    editor: EditableDraftState?,
    draft: DeclarationDraft?,
    checks: CandidateCheckReport?,
    impact: ImpactPreview?,
    gitStatus: GitStatus?,
    applied: ApplyResult?,
    onRunChecks: () -> Unit,
    onApply: () -> Unit,
    onUndo: () -> Unit,
) {
    var confirmApply by remember(draft?.id, draft?.revision, draft?.hash) { mutableStateOf(false) }
    val eligibility = draftReviewEligibility(editor, draft, checks, selected, project)
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(18.dp)) {
        if (draft == null || editor == null) {
            if (applied?.undoAvailable == true) {
                Text("LAST APPLIED DRAFT", fontWeight = FontWeight.SemiBold)
                Text("Post-apply hash: ${applied.postApplyHash}", color = SecondaryText, fontFamily = FontFamily.Monospace, fontSize = 11.sp)
                Button(onClick = onUndo, modifier = Modifier.padding(top = 8.dp)) { Text("Undo") }
            } else EmptyPane("Draft review", "Send a file-scoped message to receive a declaration draft.")
            return@Column
        }
        Text("DRAFT REVIEW · ${draft.targetSymbol}", fontWeight = FontWeight.SemiBold)
        Text("${draft.targetPath} · ${draft.mode} · revision ${draft.revision}", color = SecondaryText, fontSize = 12.sp)
        Text("Project ${draft.projectRevision.take(12)} · Base ${draft.baseFileHash.take(12)} · Draft ${draft.hash.take(12)}", color = SecondaryText, fontFamily = FontFamily.Monospace, fontSize = 11.sp)
        Spacer(Modifier.height(12.dp))
        Text("VALIDATION", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
        Text(editor.status.name.lowercase(), color = if (draft.validation?.applicable == true) Success else Warning, fontSize = 12.sp)
        draft.validation?.diagnostics.orEmpty().take(8).forEach { diagnostic -> Text("${diagnostic.code}: ${diagnostic.message}", color = Error, fontSize = 11.sp) }
        ReadOnlyDiff(draft.validation?.diff)
        Spacer(Modifier.height(12.dp))
        Text("FOCUSED CHECKS", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
        if (checks == null) Text("Run checks after validating this exact draft revision and hash.", color = Warning, fontSize = 12.sp)
        else checks.checks.forEach { check -> Text("${if (check.state == "passed") "✓" else "!"} ${check.name} · ${check.state}", color = if (check.state == "passed" || check.state == "skipped") Success else Warning, fontSize = 12.sp) }
        Button(onClick = onRunChecks, enabled = editor.status == DraftEditorStatus.Valid, modifier = Modifier.padding(top = 7.dp)) { Text("Run focused checks") }
        Spacer(Modifier.height(12.dp))
        Text("ADVISORY IMPACT", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
        if (impact?.references.isNullOrEmpty()) Text("No indexed dependents found.", color = SecondaryText, fontSize = 11.sp)
        else impact!!.references.forEach { reference -> Text("${reference.confidence} · ${reference.path} · ${reference.reason}", color = SecondaryText, fontSize = 11.sp) }
        Spacer(Modifier.height(8.dp))
        Text("GIT (READ-ONLY)", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
        Text(if (gitStatus?.available == true) "${gitStatus.branch} · ${gitStatus.fileState.ifBlank { "clean" }} · ${gitStatus.diffState}" else "Git is unavailable for this project.", color = SecondaryText, fontSize = 11.sp)
        Spacer(Modifier.height(14.dp))
        Button(onClick = { confirmApply = true }, enabled = eligibility.eligible, colors = ButtonDefaults.buttonColors(backgroundColor = Accent, contentColor = Color.White)) { Text("Apply draft") }
        if (!eligibility.eligible) Text(eligibility.reason, color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 4.dp))
    }
    if (confirmApply && draft != null) {
        AlertDialog(
            onDismissRequest = { confirmApply = false },
            title = { Text("Apply declaration draft?") },
            text = { Text("Apply the validated draft for ${draft.targetSymbol} to ${draft.targetPath}? This is the only project file that can change.") },
            confirmButton = { Button(onClick = { confirmApply = false; onApply() }) { Text("Apply ${draft.targetSymbol}") } },
            dismissButton = { Button(onClick = { confirmApply = false }) { Text("Cancel") } },
        )
    }
}

@Composable
private fun ReadOnlyDiff(diff: UnifiedDiff?) {
    Spacer(Modifier.height(8.dp))
    Text("COMPOSED DIFF (READ-ONLY)", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
    if (diff == null) Text("Validate the draft to view the composed diff.", color = SecondaryText, fontSize = 11.sp)
    else SelectionContainer {
        Column(Modifier.fillMaxWidth().background(Card, RoundedCornerShape(6.dp)).padding(8.dp)) {
            diff.lines.forEach { line ->
                Row {
                    Text("${when (line.kind) { "added" -> "+"; "removed" -> "-"; else -> " " }} ${line.newLine.takeIf { it > 0 } ?: line.oldLine}  ", fontFamily = FontFamily.Monospace, fontSize = 10.sp)
                    Text(highlightedCode(line.text), color = when (line.kind) { "added" -> Success; "removed" -> Error; else -> PrimaryText }, fontFamily = FontFamily.Monospace, fontSize = 10.sp)
                }
            }
        }
    }
}
