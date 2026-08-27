package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
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

internal enum class VerifyEvidenceStatus(val label: String) {
    Missing("Missing"),
    Running("Running"),
    Failed("Failed"),
    Stale("Stale"),
    Skipped("Skipped"),
    Passed("Passed"),
}

internal data class VerifyEvidenceRow(
    val label: String,
    val detail: String,
    val status: VerifyEvidenceStatus,
)

internal data class VerifyEvidenceUiState(
    val validation: VerifyEvidenceRow,
    val checks: VerifyEvidenceRow,
    val identity: VerifyEvidenceRow,
    val canRunChecks: Boolean,
    val runChecksLabel: String,
    val canContinueToApply: Boolean,
    val continueReason: String,
)

/** Presentation-only verification state; the daemon-owned draft and check guards remain authoritative. */
internal fun verifyEvidenceUiState(
    project: ProjectAnalysis?,
    selected: ProjectFileInfo?,
    editor: EditableDraftState?,
    draft: DeclarationDraft?,
    checks: CandidateCheckReport?,
    checksRunning: Boolean = false,
): VerifyEvidenceUiState {
    val validationCurrent = editor?.status == DraftEditorStatus.Valid && draft?.validation?.applicable == true
    val identityCurrent = draftEditorMatchesOpenFile(editor, selected, project)
    val checksRow = focusedChecksEvidence(checks, draft, checksRunning)
    val eligibility = draftReviewEligibility(editor, draft, checks, selected, project)

    return VerifyEvidenceUiState(
        validation = VerifyEvidenceRow(
            label = "Validation",
            detail = when {
                validationCurrent -> "Validation is current for draft revision ${draft?.revision}."
                editor == null -> "No editable draft is loaded."
                else -> validationSummary(editor, validationCurrent)
            },
            status = if (validationCurrent) VerifyEvidenceStatus.Passed else validationStatus(editor),
        ),
        checks = checksRow,
        identity = VerifyEvidenceRow(
            label = "Scope identity",
            detail = when {
                draft == null -> "No draft identity is available."
                identityCurrent -> "${draft.targetPath} · ${draft.targetSymbol} · revision ${draft.revision} · hash ${draft.hash.take(12)}."
                else -> "The draft project, file, revision, or base hash no longer matches the open file."
            },
            status = if (identityCurrent) VerifyEvidenceStatus.Passed else VerifyEvidenceStatus.Stale,
        ),
        canRunChecks = validationCurrent && identityCurrent && !checksRunning,
        runChecksLabel = if (checksRunning) "Focused checks are running" else "Run focused checks",
        canContinueToApply = eligibility.eligible,
        continueReason = if (eligibility.eligible) "Current validation and focused checks match this exact draft." else eligibility.reason,
    )
}

private fun validationStatus(editor: EditableDraftState?): VerifyEvidenceStatus = when (editor?.status) {
    DraftEditorStatus.Validating -> VerifyEvidenceStatus.Running
    DraftEditorStatus.Invalid -> VerifyEvidenceStatus.Failed
    DraftEditorStatus.Stale -> VerifyEvidenceStatus.Stale
    else -> VerifyEvidenceStatus.Missing
}

private fun focusedChecksEvidence(
    checks: CandidateCheckReport?,
    draft: DeclarationDraft?,
    checksRunning: Boolean,
): VerifyEvidenceRow {
    if (checksRunning) return VerifyEvidenceRow("Focused checks", "Focused checks are running for the current draft.", VerifyEvidenceStatus.Running)
    if (checks == null) return VerifyEvidenceRow("Focused checks", "Run checks after validating this exact draft revision and hash.", VerifyEvidenceStatus.Missing)
    if (!checksMatchDraft(checks, draft)) return VerifyEvidenceRow("Focused checks", "Check results do not match the latest draft revision or hash.", VerifyEvidenceStatus.Stale)
    if (!checks.applicable) return VerifyEvidenceRow("Focused checks", "Focused checks could not produce applicable evidence for this draft.", VerifyEvidenceStatus.Failed)

    val states = checks.checks.map { it.state.lowercase() }
    val status = when {
        states.any { it == "running" } -> VerifyEvidenceStatus.Running
        states.any { it in setOf("failed", "error", "canceled", "cancelled") } -> VerifyEvidenceStatus.Failed
        states.isNotEmpty() && states.all { it == "skipped" } -> VerifyEvidenceStatus.Skipped
        states.all { it in setOf("passed", "skipped") } -> VerifyEvidenceStatus.Passed
        else -> VerifyEvidenceStatus.Missing
    }
    val required = checks.checks.count { it.required }
    val detail = when (status) {
        VerifyEvidenceStatus.Passed -> "${checks.checks.size} checks (${required} required) are current for draft revision ${draft?.revision}."
        VerifyEvidenceStatus.Skipped -> "${checks.checks.size} checks were skipped for draft revision ${draft?.revision}."
        VerifyEvidenceStatus.Running -> "Focused checks are running for draft revision ${draft?.revision}."
        VerifyEvidenceStatus.Failed -> "At least one focused check failed for draft revision ${draft?.revision}."
        VerifyEvidenceStatus.Missing -> "Focused check state is unavailable for the latest draft."
        VerifyEvidenceStatus.Stale -> error("Stale evidence returns before details are derived.")
    }
    return VerifyEvidenceRow("Focused checks", detail, status)
}

internal fun advisoryImpactLabel(impact: ImpactPreview?): String = when {
    impact?.references.isNullOrEmpty() -> "No indexed dependents found. This is read-only context and cannot change project files."
    else -> "${impact!!.references.size} indexed dependents are visible as read-only context; they cannot be changed by this draft."
}

internal fun gitContextLabel(gitStatus: GitStatus?): String = if (gitStatus?.available == true) {
    "${gitStatus.branch.ifBlank { "unknown branch" }} · ${gitStatus.fileState.ifBlank { "clean" }} · ${gitStatus.diffState.ifBlank { "no file diff" }} · read-only context"
} else {
    "Git is unavailable for this project. This read-only context does not affect the one-file draft boundary."
}

internal data class ApplyDecisionUiState(
    val eligible: Boolean,
    val actionLabel: String,
    val reason: String,
    val receiptTitle: String? = null,
    val receiptDetail: String = "",
    val undoLabel: String = "Undo",
)

internal fun applyActionLabel(draft: DeclarationDraft?): String = draft?.let { "Apply ${it.targetSymbol} to ${it.targetPath}" } ?: "Apply draft"

internal fun applyReceiptTitle(result: ApplyResult): String = when (result.audit?.action?.lowercase()) {
    "undo" -> "Change undone"
    else -> "Change applied"
}

internal fun applyDecisionUiState(
    project: ProjectAnalysis?,
    selected: ProjectFileInfo?,
    editor: EditableDraftState?,
    draft: DeclarationDraft?,
    checks: CandidateCheckReport?,
    applied: ApplyResult?,
): ApplyDecisionUiState {
    if (applied != null) {
        val action = applyReceiptTitle(applied)
        return ApplyDecisionUiState(
            eligible = false,
            actionLabel = "Apply unavailable after receipt",
            reason = "The previous guarded operation must be reviewed before another draft can be applied.",
            receiptTitle = action,
            receiptDetail = "Project revision ${applied.projectRevision} · resulting file hash ${applied.postApplyHash}.",
            undoLabel = if (applied.undoAvailable) "Undo this change" else "Undo is no longer available",
        )
    }
    val eligibility = draftReviewEligibility(editor, draft, checks, selected, project)
    return ApplyDecisionUiState(
        eligible = eligibility.eligible,
        actionLabel = applyActionLabel(draft),
        reason = eligibility.reason,
    )
}

@Composable
internal fun VerifyDiffCanvas(draft: DeclarationDraft?, modifier: Modifier = Modifier) {
    Column(modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(18.dp)) {
        Text("Verify the candidate", color = PrimaryText, fontWeight = FontWeight.SemiBold, fontSize = 18.sp)
        Text("Compare the composed declaration before moving to the final Apply step.", color = SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 4.dp))
        Spacer(Modifier.height(12.dp))
        DiffViewer(draft?.validation?.diff, Modifier.fillMaxWidth())
        Text("The composed diff is selectable and read-only. Only the isolated declaration draft was editable.", color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 10.dp))
    }
}

@Composable
internal fun ApplyDiffCanvas(draft: DeclarationDraft?, applied: ApplyResult?, modifier: Modifier = Modifier) {
    Column(modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(18.dp)) {
        Text("Review this change", color = PrimaryText, fontWeight = FontWeight.SemiBold, fontSize = 18.sp)
        Text("Read the exact composed result before the one explicit write action.", color = SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 4.dp))
        Spacer(Modifier.height(12.dp))
        if (draft != null) {
            Text("${draft.targetPath} · ${draft.targetSymbol} · ${draft.mode}", color = SecondaryText, fontSize = 11.sp)
            Spacer(Modifier.height(8.dp))
            DiffViewer(draft.validation?.diff, Modifier.fillMaxWidth())
            Text("Nothing has changed yet. The composed diff is selectable and read-only.", color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 10.dp))
        } else if (applied != null) {
            SystemStateMessage(
                applyReceiptTitle(applied),
                "The selected file has been refreshed at project revision ${applied.projectRevision}. Review the guarded receipt in Context.",
                accent = Success,
            )
        } else {
            SystemStateMessage("Apply unavailable", "Complete the current draft, validation, and focused checks before entering Apply.")
        }
    }
}

@Composable
internal fun VerifyEvidencePane(
    project: ProjectAnalysis?,
    selected: ProjectFileInfo?,
    editor: EditableDraftState?,
    draft: DeclarationDraft?,
    checks: CandidateCheckReport?,
    impact: ImpactPreview?,
    gitStatus: GitStatus?,
    checksRunning: Boolean,
    onRunChecks: () -> Unit,
    onContinueToApply: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val evidence = verifyEvidenceUiState(project, selected, editor, draft, checks, checksRunning)
    var showCommandOutput by remember(checks?.draftId, checks?.draftRevision, checks?.draftHash) { mutableStateOf(false) }

    Column(modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(12.dp)) {
        FocusFlowPanel(Modifier.fillMaxWidth(), raised = true) {
            SectionLabel("VERIFY · GATE 3 OF 4")
            Text(if (evidence.canContinueToApply) "Evidence is current" else "Evidence needs attention", color = PrimaryText, fontWeight = FontWeight.SemiBold, fontSize = 17.sp, modifier = Modifier.padding(top = 4.dp))
            Text("Review the proof for this exact draft before enabling the final Apply confirmation.", color = SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 4.dp))
            Spacer(Modifier.height(10.dp))
            EvidenceRow(evidence.identity)
            Spacer(Modifier.height(8.dp))
            EvidenceRow(evidence.validation)
            editor?.diagnostics.orEmpty().take(8).forEach { diagnostic ->
                Text("${diagnostic.code}: ${diagnostic.message}", color = Error, fontSize = 11.sp, modifier = Modifier.padding(top = 4.dp))
            }
        }
        Spacer(Modifier.height(10.dp))
        FocusFlowPanel(Modifier.fillMaxWidth()) {
            EvidenceRow(evidence.checks)
            Text("Timing: the daemon does not report focused-check duration.", color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 4.dp))
            Button(onClick = onRunChecks, enabled = evidence.canRunChecks, modifier = Modifier.padding(top = 8.dp)) { Text(evidence.runChecksLabel) }
            checks?.checks.orEmpty().forEach { check ->
                Text("${check.name} · ${check.state} · ${if (check.required) "required" else "optional"}", color = evidenceColor(checkStatus(check.state)), fontSize = 11.sp, modifier = Modifier.padding(top = 6.dp))
            }
            val checksWithOutput = checks?.checks.orEmpty().filter { it.command.isNotEmpty() || it.output.isNotBlank() }
            if (checksWithOutput.isNotEmpty()) {
                Button(onClick = { showCommandOutput = !showCommandOutput }, modifier = Modifier.padding(top = 8.dp)) {
                    Text(if (showCommandOutput) "Hide command output" else "Show command output (${checksWithOutput.size})")
                }
                if (showCommandOutput) SelectionContainer {
                    Column(Modifier.padding(top = 6.dp)) {
                        checksWithOutput.forEach { check ->
                            Text(check.name, color = PrimaryText, fontWeight = FontWeight.SemiBold, fontSize = 11.sp)
                            if (check.command.isNotEmpty()) Text("\$ ${check.command.joinToString(" ")}", color = SecondaryText, fontFamily = FontFamily.Monospace, fontSize = 10.sp)
                            if (check.output.isNotBlank()) Text(check.output, color = SecondaryText, fontFamily = FontFamily.Monospace, fontSize = 10.sp, modifier = Modifier.padding(top = 2.dp))
                        }
                    }
                }
            }
        }
        Spacer(Modifier.height(10.dp))
        ReadOnlyImpactPane(impact, gitStatus)
        Spacer(Modifier.height(12.dp))
        FocusFlowPanel(Modifier.fillMaxWidth(), raised = true) {
            Text("Next: explicit Apply", color = PrimaryText, fontWeight = FontWeight.SemiBold)
            Text("The next step names the exact target again and requires your confirmation. Nothing has changed yet.", color = SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 4.dp))
            Button(
                onClick = onContinueToApply,
                enabled = evidence.canContinueToApply,
                colors = ButtonDefaults.buttonColors(backgroundColor = Accent, contentColor = OnAccent),
                modifier = Modifier.padding(top = 8.dp),
            ) { Text("Continue to Apply") }
            if (!evidence.canContinueToApply) Text(evidence.continueReason, color = Warning, fontSize = 11.sp, modifier = Modifier.padding(top = 5.dp))
        }
    }
}

@Composable
internal fun ApplyDecisionPane(
    project: ProjectAnalysis?,
    selected: ProjectFileInfo?,
    editor: EditableDraftState?,
    draft: DeclarationDraft?,
    checks: CandidateCheckReport?,
    impact: ImpactPreview?,
    gitStatus: GitStatus?,
    applied: ApplyResult?,
    onApply: () -> Unit,
    onUndo: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val decision = applyDecisionUiState(project, selected, editor, draft, checks, applied)
    Column(modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(12.dp)) {
        if (decision.receiptTitle != null && applied != null) {
            FocusFlowPanel(Modifier.fillMaxWidth(), raised = true) {
                SectionLabel("APPLIED RECEIPT")
                Text(decision.receiptTitle, color = PrimaryText, fontWeight = FontWeight.SemiBold, fontSize = 17.sp, modifier = Modifier.padding(top = 4.dp))
                Text(decision.receiptDetail, color = SecondaryText, fontFamily = FontFamily.Monospace, fontSize = 11.sp, modifier = Modifier.padding(top = 6.dp))
                Text(if (applied.undoAvailable) "The guarded Undo action is available for this returned identity." else "The returned identity no longer has an available Undo action.", color = SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 6.dp))
                Button(onClick = onUndo, enabled = applied.undoAvailable, modifier = Modifier.padding(top = 10.dp)) { Text(decision.undoLabel) }
            }
            return@Column
        }

        if (draft == null || editor == null) {
            SystemStateMessage("Apply is locked", decision.reason, modifier = Modifier.fillMaxWidth())
            return@Column
        }
        val evidence = verifyEvidenceUiState(project, selected, editor, draft, checks)
        FocusFlowPanel(Modifier.fillMaxWidth(), raised = true) {
            SectionLabel("APPLY · GATE 4 OF 4")
            Text("Safe to apply", color = PrimaryText, fontWeight = FontWeight.SemiBold, fontSize = 17.sp, modifier = Modifier.padding(top = 4.dp))
            Text("Nothing has changed yet. Review the exact scope and current proof before confirming the write.", color = SecondaryText, fontSize = 12.sp, modifier = Modifier.padding(top = 4.dp))
            Spacer(Modifier.height(10.dp))
            EvidenceRow(evidence.identity)
            Spacer(Modifier.height(7.dp))
            EvidenceRow(evidence.validation)
            Spacer(Modifier.height(7.dp))
            EvidenceRow(evidence.checks)
            Text("Target: ${draft.targetPath} · ${draft.targetSymbol} · ${draft.mode}", color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 8.dp))
            Text("Only this named file and isolated declaration can change. Apply remains revision/hash guarded.", color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 4.dp))
        }
        Spacer(Modifier.height(10.dp))
        ReadOnlyImpactPane(impact, gitStatus)
        Spacer(Modifier.height(12.dp))
        FocusFlowPanel(Modifier.fillMaxWidth(), raised = true) {
            Button(
                onClick = onApply,
                enabled = decision.eligible,
                colors = ButtonDefaults.buttonColors(backgroundColor = Accent, contentColor = OnAccent),
                modifier = Modifier.fillMaxWidth(),
            ) { Text(decision.actionLabel) }
            if (!decision.eligible) Text(decision.reason, color = Warning, fontSize = 11.sp, modifier = Modifier.padding(top = 6.dp))
        }
    }
}

@Composable
private fun EvidenceRow(row: VerifyEvidenceRow) {
    Text(row.label, color = PrimaryText, fontWeight = FontWeight.SemiBold, fontSize = 12.sp)
    Text("${row.status.label} · ${row.detail}", color = evidenceColor(row.status), fontSize = 11.sp, modifier = Modifier.padding(top = 2.dp))
}

@Composable
private fun ReadOnlyImpactPane(impact: ImpactPreview?, gitStatus: GitStatus?) {
    FocusFlowPanel(Modifier.fillMaxWidth()) {
        SectionLabel("ADVISORY IMPACT · READ-ONLY")
        if (impact?.references.isNullOrEmpty()) {
            Text(advisoryImpactLabel(impact), color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 4.dp))
        } else {
            Text(advisoryImpactLabel(impact), color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 4.dp))
            impact!!.references.forEach { reference ->
                Text("${reference.confidence} · ${reference.path} · ${reference.reason}", color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 4.dp))
            }
        }
        Spacer(Modifier.height(10.dp))
        SectionLabel("GIT CONTEXT · READ-ONLY")
        Text(gitContextLabel(gitStatus), color = SecondaryText, fontSize = 11.sp, modifier = Modifier.padding(top = 4.dp))
    }
}

private fun checkStatus(state: String): VerifyEvidenceStatus = when (state.lowercase()) {
    "passed" -> VerifyEvidenceStatus.Passed
    "skipped" -> VerifyEvidenceStatus.Skipped
    "running" -> VerifyEvidenceStatus.Running
    "failed", "error", "canceled", "cancelled" -> VerifyEvidenceStatus.Failed
    else -> VerifyEvidenceStatus.Missing
}

private fun evidenceColor(status: VerifyEvidenceStatus): Color = when (status) {
    VerifyEvidenceStatus.Passed -> Success
    VerifyEvidenceStatus.Running, VerifyEvidenceStatus.Skipped, VerifyEvidenceStatus.Missing, VerifyEvidenceStatus.Stale -> Warning
    VerifyEvidenceStatus.Failed -> Error
}
