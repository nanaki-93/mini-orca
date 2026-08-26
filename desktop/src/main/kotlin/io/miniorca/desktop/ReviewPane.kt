package io.miniorca.desktop

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
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
import androidx.compose.material.Button
import androidx.compose.material.ButtonDefaults
import androidx.compose.material.OutlinedTextField
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
internal fun ReviewPane(
    candidate: GenerationResult?, comparisonBase: GenerationResult?, comparison: CandidateComparison?, checks: CandidateCheckReport?, applied: ApplyResult?, selected: ProjectFileInfo?, onDiscard: () -> Unit,
    onAskForRevision: () -> Unit, onRunChecks: () -> Unit, onGenerateAlternate: () -> Unit, onCompare: () -> Unit, onExport: () -> Unit,
    comparisonBaseNote: String, comparisonCandidateNote: String, onComparisonBaseNote: (String) -> Unit, onComparisonCandidateNote: (String) -> Unit, onApply: () -> Unit, onUndo: () -> Unit, activity: List<ActivityEntry>,
    showActivity: Boolean, onToggleActivity: () -> Unit, impact: ImpactPreview?, gitStatus: GitStatus?,
) {
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(18.dp)) {
        if (candidate == null) {
            if (applied?.undoAvailable == true) {
                Text("LAST APPLIED CHANGE", fontWeight = FontWeight.SemiBold)
                Text("Post-apply hash: ${applied.postApplyHash}", color = SecondaryText, fontFamily = FontFamily.Monospace, fontSize = 11.sp)
                Spacer(Modifier.height(10.dp))
                Button(onClick = onUndo) { Text("Undo") }
            } else EmptyPane("Changes", "Generate a scoped preview to inspect its validation, diff, and checks.")
        } else {
            Text("CHANGE PREVIEW · ${candidate.targetSymbol}", fontWeight = FontWeight.SemiBold)
            Text("${candidate.targetPath} → ${candidate.targetSymbol} · ${if (candidate.scopeMode == "strict_symbol") "Strict symbol" else "Symbol + required imports"}", color = SecondaryText, fontSize = 12.sp)
            Text("Base ${candidate.baseFileHash.takeLast(12)} · Candidate ${candidate.candidateHash.takeLast(12)}", color = SecondaryText, fontFamily = FontFamily.Monospace, fontSize = 11.sp, modifier = Modifier.padding(top = 4.dp))
            Spacer(Modifier.height(14.dp))
            Text(if (candidate.validation.applicable) "✓ Scope validation passed" else "! Scope validation failed", color = if (candidate.validation.applicable) Success else Error, fontSize = 12.sp)
            candidate.validation.diagnostics.forEach { finding -> Text("${finding.code}: ${finding.message}", color = Error, fontSize = 11.sp, modifier = Modifier.padding(top = 4.dp)) }
            Spacer(Modifier.height(12.dp))
            SelectionContainer {
                Column(Modifier.fillMaxWidth().background(Card, RoundedCornerShape(6.dp)).padding(10.dp)) {
                    candidate.validation.diff.lines.forEach { line ->
                        val prefix = when (line.kind) { "added" -> "+"; "removed" -> "-"; else -> " " }
                        Row {
                            Text("$prefix ${line.newLine.takeIf { it > 0 } ?: line.oldLine}  ", fontFamily = FontFamily.Monospace, fontSize = 11.sp)
                            Text(highlightedCode(line.text), color = when (line.kind) { "added" -> Success; "removed" -> Error; else -> PrimaryText }, fontFamily = FontFamily.Monospace, fontSize = 11.sp)
                        }
                    }
                }
            }
            Spacer(Modifier.height(12.dp))
            Text("FOCUSED CHECKS", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
            if (checks == null) Text("Required checks have not run.", color = Warning, fontSize = 12.sp)
            else checks.checks.forEach { check -> Text("${if (check.state == "passed") "✓" else "!"} ${check.name} · ${check.state}", color = if (check.state == "passed" || check.state == "skipped") Success else Warning, fontSize = 12.sp) }
            Spacer(Modifier.height(12.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Button(onClick = onDiscard) { Text("Discard") }
                Button(onClick = onAskForRevision) { Text("Ask for revision") }
                Button(onClick = onRunChecks, enabled = candidate.validation.applicable) { Text("Run focused checks") }
            }
            Spacer(Modifier.height(8.dp))
            Button(onClick = onGenerateAlternate, enabled = candidate.validation.applicable, colors = ButtonDefaults.buttonColors(backgroundColor = Card, contentColor = PrimaryText)) { Text("Generate alternate") }
            Text("An alternate is another preview only. It cannot apply either candidate.", color = SecondaryText, fontSize = 10.sp, modifier = Modifier.padding(top = 4.dp))
            if (comparisonBase != null) {
                Spacer(Modifier.height(12.dp))
                Text("CANDIDATE COMPARISON", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
                OutlinedTextField(value = comparisonBaseNote, onValueChange = onComparisonBaseNote, label = { Text("First candidate note") }, modifier = Modifier.fillMaxWidth(), minLines = 1)
                OutlinedTextField(value = comparisonCandidateNote, onValueChange = onComparisonCandidateNote, label = { Text("Alternate candidate note") }, modifier = Modifier.fillMaxWidth(), minLines = 1)
                Button(onClick = onCompare, colors = ButtonDefaults.buttonColors(backgroundColor = Card, contentColor = PrimaryText), modifier = Modifier.padding(top = 6.dp)) { Text("Compare candidates") }
                comparison?.let { result ->
                    Spacer(Modifier.height(6.dp))
                    Text("First · ${result.left.diffLines} diff lines · ${result.left.scopeMode} · checks ${result.left.checks} · ${result.left.model}", color = SecondaryText, fontSize = 11.sp)
                    Text("Alternate · ${result.right.diffLines} diff lines · ${result.right.scopeMode} · checks ${result.right.checks} · ${result.right.model}", color = SecondaryText, fontSize = 11.sp)
                }
            }
            Spacer(Modifier.height(8.dp))
            Button(onClick = onExport, colors = ButtonDefaults.buttonColors(backgroundColor = Card, contentColor = PrimaryText)) { Text("Export review Markdown") }
            Text("Exports source-free summary, findings, candidate metadata, checks, and audit reference only.", color = SecondaryText, fontSize = 10.sp, modifier = Modifier.padding(top = 4.dp))
            Spacer(Modifier.height(8.dp))
            val eligibility = candidateApplyEligibility(candidate, checks, selected)
            Button(onClick = onApply, enabled = eligibility.eligible, colors = ButtonDefaults.buttonColors(backgroundColor = Accent, contentColor = Color.White)) { Text("Apply") }
            if (!eligibility.eligible) Text(eligibility.reason, color = SecondaryText, fontSize = 10.sp, modifier = Modifier.padding(top = 4.dp))
        }
        Spacer(Modifier.height(18.dp))
        Text("ADVISORY IMPACT", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
        if (impact?.references.isNullOrEmpty()) Text("No indexed dependents found. This never expands model context.", color = SecondaryText, fontSize = 11.sp)
        else impact!!.references.forEach { reference -> Text("${reference.confidence} · ${reference.path} · ${reference.reason}", color = SecondaryText, fontSize = 11.sp) }
        Spacer(Modifier.height(10.dp))
        Text("GIT (READ-ONLY)", color = SecondaryText, fontSize = 10.sp, fontWeight = FontWeight.Bold)
        if (gitStatus?.available == true) Text("${gitStatus.branch} · ${gitStatus.fileState.ifBlank { "clean" }} · ${gitStatus.diffState}", color = SecondaryText, fontSize = 11.sp) else Text("Git is unavailable for this project.", color = SecondaryText, fontSize = 11.sp)
        Spacer(Modifier.height(18.dp))
        Button(onClick = onToggleActivity, colors = ButtonDefaults.buttonColors(backgroundColor = Card, contentColor = PrimaryText)) { Text(if (showActivity) "Hide activity" else "Show activity") }
        if (showActivity) {
            if (activity.isEmpty()) Text("No project activity recorded yet.", color = SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 8.dp))
            activity.takeLast(12).reversed().forEach { entry -> Text("${entry.phase} · ${entry.content} · ${entry.targetFile} ${entry.targetSymbol}".trim(), color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 5.dp)) }
        }
    }
}
