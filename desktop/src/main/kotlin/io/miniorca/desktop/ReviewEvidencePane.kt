package io.miniorca.desktop

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

internal enum class ReviewEvidenceStatus(val label: String) {
  Missing("Missing"),
  Running("Running"),
  Failed("Failed"),
  Stale("Stale"),
  Skipped("Skipped"),
  Passed("Passed"),
}

internal data class ReviewEvidenceRow(
    val label: String,
    val detail: String,
    val status: ReviewEvidenceStatus,
)

internal data class ReviewEvidenceUiState(
    val validation: ReviewEvidenceRow,
    val checks: ReviewEvidenceRow,
    val identity: ReviewEvidenceRow,
    val canRunChecks: Boolean,
    val runChecksLabel: String,
)

/** Presentation-only review evidence; daemon-owned draft and check guards remain authoritative. */
internal fun reviewEvidenceUiState(
    project: ProjectAnalysis?,
    selected: ProjectFileInfo?,
    editor: EditableDraftState?,
    draft: DeclarationDraft?,
    checks: DraftCheckReport?,
    checksRunning: Boolean = false,
): ReviewEvidenceUiState {
  val validationCurrent =
      editor?.status == DraftEditorStatus.Valid && draft?.validation?.applicable == true
  val identityCurrent = draftEditorMatchesOpenFile(editor, selected, project)
  val checksRow = focusedChecksEvidence(checks, draft, checksRunning)

  return ReviewEvidenceUiState(
      validation =
          ReviewEvidenceRow(
              label = "Validation",
              detail =
                  when {
                    validationCurrent -> "Validation is current."
                    editor == null -> "No editable draft is loaded."
                    else -> reviewValidationSummary(editor, validationCurrent)
                  },
              status =
                  if (validationCurrent) ReviewEvidenceStatus.Passed else validationStatus(editor),
          ),
      checks = checksRow,
      identity =
          ReviewEvidenceRow(
              label = "Scope identity",
              detail =
                  when {
                    draft == null -> "No draft identity is available."
                    identityCurrent ->
                        "${draft.targetPath} · ${draft.targetSymbol} matches the open file."
                    else -> "The draft no longer matches the open file."
                  },
              status =
                  if (identityCurrent) ReviewEvidenceStatus.Passed else ReviewEvidenceStatus.Stale,
          ),
      canRunChecks = validationCurrent && identityCurrent && !checksRunning,
      runChecksLabel = if (checksRunning) "Focused checks are running" else "Run focused checks",
  )
}

private fun validationStatus(editor: EditableDraftState?): ReviewEvidenceStatus =
    when (editor?.status) {
      DraftEditorStatus.Validating -> ReviewEvidenceStatus.Running
      DraftEditorStatus.Invalid -> ReviewEvidenceStatus.Failed
      DraftEditorStatus.Stale -> ReviewEvidenceStatus.Stale
      else -> ReviewEvidenceStatus.Missing
    }

internal fun reviewValidationSummary(
    editor: EditableDraftState?,
    validationCurrent: Boolean
): String =
    when {
      validationCurrent -> "Validated for the latest draft."
      editor == null -> "No editable draft is loaded."
      editor.status == DraftEditorStatus.Dirty ->
          "Manual edits require validation and fresh checks."
      editor.status == DraftEditorStatus.Validating -> "Validation is running."
      editor.status == DraftEditorStatus.Invalid -> "Fix validation diagnostics before continuing."
      editor.status == DraftEditorStatus.Stale ->
          "The draft is stale; start a new file-scoped conversation."
      else -> "Validate the latest declaration draft before continuing."
    }

internal fun checksMatchDraft(checks: DraftCheckReport?, draft: DeclarationDraft?): Boolean =
    checks != null &&
        draft != null &&
        checks.draftId == draft.id &&
        checks.draftRevision == draft.revision &&
        checks.draftHash == draft.hash

internal fun repairMessageForChecks(
    session: ChatSession?,
    draft: DeclarationDraft?,
    checks: DraftCheckReport?
): String? {
  if (session?.taskSpec == null ||
      draft?.taskSpec == null ||
      !sameTaskSpec(session.taskSpec, draft.taskSpec) ||
      session.repairCount >= 3 ||
      !checksMatchDraft(checks, draft))
      return null
  val failures =
      checks!!.checks.filter {
        it.state.lowercase() in setOf("failed", "error", "canceled", "cancelled")
      }
  if (failures.isEmpty() && checks.applicable) return null
  val evidence =
      failures
          .ifEmpty { checks.checks.filter { it.output.isNotBlank() } }
          .joinToString("\n\n") { check ->
            "${check.name} (${check.state}):\n${check.output.take(2048)}"
          }
          .take(4096)
  return "Revise the current declaration to address this sanitized focused check evidence. Keep the pinned task scope and do not change unrelated code.\n\n$evidence"
      .trim()
}

private fun repairLimitReached(
    session: ChatSession?,
    draft: DeclarationDraft?,
    checks: DraftCheckReport?
): Boolean =
    session?.taskSpec != null &&
        draft?.taskSpec != null &&
        sameTaskSpec(session.taskSpec, draft.taskSpec) &&
        session.repairCount >= 3 &&
        checksMatchDraft(checks, draft) &&
        !checks!!.applicable

private fun focusedChecksEvidence(
    checks: DraftCheckReport?,
    draft: DeclarationDraft?,
    checksRunning: Boolean,
): ReviewEvidenceRow {
  if (checksRunning)
      return ReviewEvidenceRow(
          "Focused checks",
          "Focused checks are running for the current draft.",
          ReviewEvidenceStatus.Running)
  if (checks == null)
      return ReviewEvidenceRow(
          "Focused checks", "Run checks after validating this draft.", ReviewEvidenceStatus.Missing)
  if (!checksMatchDraft(checks, draft))
      return ReviewEvidenceRow(
          "Focused checks",
          "Check results no longer match the latest draft.",
          ReviewEvidenceStatus.Stale)
  if (!checks.applicable)
      return ReviewEvidenceRow(
          "Focused checks",
          "Focused checks could not produce applicable evidence for this draft.",
          ReviewEvidenceStatus.Failed)

  val states = checks.checks.map { it.state.lowercase() }
  val status =
      when {
        states.any { it == "running" } -> ReviewEvidenceStatus.Running
        states.any { it in setOf("failed", "error", "canceled", "cancelled") } ->
            ReviewEvidenceStatus.Failed
        states.isNotEmpty() && states.all { it == "skipped" } -> ReviewEvidenceStatus.Skipped
        states.all { it in setOf("passed", "skipped") } -> ReviewEvidenceStatus.Passed
        else -> ReviewEvidenceStatus.Missing
      }
  val required = checks.checks.count { it.required }
  val detail =
      when (status) {
        ReviewEvidenceStatus.Passed ->
            "${checks.checks.size} checks (${required} required) are current."
        ReviewEvidenceStatus.Skipped -> "${checks.checks.size} checks were skipped."
        ReviewEvidenceStatus.Running -> "Focused checks are running."
        ReviewEvidenceStatus.Failed -> "At least one focused check failed."
        ReviewEvidenceStatus.Missing -> "Focused check state is unavailable for the latest draft."
        ReviewEvidenceStatus.Stale -> error("Stale evidence returns before details are derived.")
      }
  return ReviewEvidenceRow("Focused checks", detail, status)
}

internal fun advisoryImpactLabel(impact: ImpactPreview?): String =
    when {
      impact?.references.isNullOrEmpty() ->
          "No indexed dependents found. This is read-only context and cannot change project files."
      else ->
          "${impact.references.size} indexed dependents are visible as read-only context; they cannot be changed by this draft."
    }

internal fun gitContextLabel(gitStatus: GitStatus?): String =
    if (gitStatus?.available == true) {
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

internal fun applyActionLabel(draft: DeclarationDraft?): String =
    draft?.let { "Apply ${it.targetSymbol} to ${it.targetPath}" } ?: "Apply draft"

internal fun applyReceiptTitle(result: ApplyResult): String =
    when (result.audit?.action?.lowercase()) {
      "undo" -> "Change undone"
      else -> "Change applied"
    }

internal fun applyDecisionUiState(
    project: ProjectAnalysis?,
    selected: ProjectFileInfo?,
    editor: EditableDraftState?,
    draft: DeclarationDraft?,
    checks: DraftCheckReport?,
    applied: ApplyResult?,
): ApplyDecisionUiState {
  if (applied != null) {
    val action = applyReceiptTitle(applied)
    return ApplyDecisionUiState(
        eligible = false,
        actionLabel = "Apply unavailable after receipt",
        reason =
            "The previous guarded operation must be reviewed before another draft can be applied.",
        receiptTitle = action,
        receiptDetail =
            "${applied.audit?.targetPath?.takeIf { it.isNotBlank() } ?: "Selected file"} ${if (action == "Change undone") "was restored" else "was updated"}.",
        undoLabel =
            if (applied.undoAvailable) "Undo this change" else "Undo is no longer available",
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
internal fun ReviewDiffCanvas(draft: DeclarationDraft?, modifier: Modifier = Modifier) {
  Column(modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(18.dp)) {
    Text("Candidate diff", color = PrimaryText, fontWeight = FontWeight.SemiBold, fontSize = 18.sp)
    Spacer(Modifier.height(12.dp))
    DiffViewer(draft?.validation?.diff, Modifier.fillMaxWidth())
  }
}

@Composable
internal fun ReviewToolWindow(
    state: ReviewToolWindowState,
    evidenceActions: ReviewToolWindowActions,
    applicationActions: DraftApplicationActions,
    modifier: Modifier = Modifier,
) {
  val evidence =
      reviewEvidenceUiState(
          state.project,
          state.selected,
          state.editor,
          state.draft,
          state.checks,
          state.checksRunning)
  val decision =
      applyDecisionUiState(
          state.project, state.selected, state.editor, state.draft, state.checks, state.applied)
  var diagnosticsExpanded by
      rememberSaveable(state.draft?.id, state.draft?.revision, state.draft?.hash) {
        mutableStateOf(false)
      }
  var checksEvidenceExpanded by
      rememberSaveable(
          state.checks?.draftId, state.checks?.draftRevision, state.checks?.draftHash) {
            mutableStateOf(false)
          }

  Column(modifier.fillMaxSize()) {
    ToolWindowScopeHeader(
        "REVIEW",
        reviewToolWindowScope(state.selected, state.selectedSymbol, state.draft, state.applied),
        Modifier)
    Column(
        Modifier.weight(1f)
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 8.dp)
            .padding(bottom = 8.dp)) {
          if (decision.receiptTitle != null && state.applied != null) {
            ReviewSection(
                title = "Receipt",
                icon = DesktopIcon.Check,
                stateLabel =
                    if (state.applied.undoAvailable) "Undo available" else "Undo unavailable",
                stateTint = if (state.applied.undoAvailable) Success else Warning) {
                  Text(
                      decision.receiptTitle,
                      color = PrimaryText,
                      fontWeight = FontWeight.SemiBold,
                      fontSize = 17.sp,
                      modifier = Modifier.padding(top = 4.dp))
                  Text(
                      decision.receiptDetail,
                      color = SecondaryText,
                      fontFamily = FontFamily.Monospace,
                      fontSize = 11.sp,
                      modifier = Modifier.padding(top = 6.dp))
                  Text(
                      if (state.applied.undoAvailable) "Undo available." else "Undo unavailable.",
                      color = SecondaryText,
                      fontSize = 12.sp,
                      modifier = Modifier.padding(top = 6.dp))
                  MiniOrcaButton(
                      onClick = applicationActions.undo,
                      enabled = state.applied.undoAvailable,
                      tone = ActionTone.Attention,
                      modifier = Modifier.padding(top = 10.dp)) {
                        Text(decision.undoLabel)
                      }
                }
            return@Column
          }
          ReviewSection(
              title = "Review evidence",
              icon = DesktopIcon.Check,
              stateLabel = evidence.validation.status.label,
              stateTint = evidenceColor(evidence.validation.status),
              actions = {
                if (state.editor != null && state.draft != null)
                    ChromeButton(
                        onClick = evidenceActions.editDraft, accessibleName = "Edit draft") {
                          Text("Edit draft", fontSize = 11.sp)
                        }
              }) {
                EvidenceRow(evidence.identity)
                IdeHorizontalSeparator(Modifier.padding(vertical = 6.dp))
                EvidenceRow(evidence.validation)
                val diagnostics = state.editor?.diagnostics.orEmpty().take(8)
                if (diagnostics.isNotEmpty())
                    ReviewEvidenceDetails(
                        title = "Validation diagnostics",
                        diagnostics = diagnostics,
                        checks = emptyList(),
                        expanded = diagnosticsExpanded,
                        onToggle = { diagnosticsExpanded = !diagnosticsExpanded })
              }
          ReviewSection(
              title = "Focused checks",
              icon = DesktopIcon.Run,
              stateLabel = evidence.checks.status.label,
              stateTint = evidenceColor(evidence.checks.status)) {
                EvidenceRow(evidence.checks)
                if (evidence.canRunChecks)
                    MiniOrcaButton(
                        onClick = evidenceActions.runChecks,
                        tone = ActionTone.Primary,
                        modifier = Modifier.padding(top = 8.dp)) {
                          Text(evidence.runChecksLabel)
                        }
                val repairMessage = repairMessageForChecks(state.session, state.draft, state.checks)
                if (repairMessage != null ||
                    repairLimitReached(state.session, state.draft, state.checks)) {
                  MiniOrcaButton(
                      onClick = evidenceActions.reviseWithCheckOutput,
                      enabled = repairMessage != null && !state.checksRunning,
                      tone = ActionTone.Attention,
                      modifier = Modifier.padding(top = 8.dp)) {
                        Text(
                            if (repairMessage != null) "Revise with check output"
                            else "Repair limit reached")
                      }
                }
                val checksWithOutput =
                    state.checks?.checks.orEmpty().filter {
                      it.command.isNotEmpty() || it.output.isNotBlank()
                    }
                if (state.checks?.checks.orEmpty().isNotEmpty() || checksWithOutput.isNotEmpty())
                    ReviewEvidenceDetails(
                        title = "Detailed evidence",
                        diagnostics = emptyList(),
                        checks = state.checks?.checks.orEmpty(),
                        expanded = checksEvidenceExpanded,
                        onToggle = { checksEvidenceExpanded = !checksEvidenceExpanded })
              }
          ReadOnlyImpactPane(state.impact, state.gitStatus)
          state.draft?.engineeringInsight?.let { insight ->
            EngineeringInsightPanel(
                insight,
                stale = state.draft.state.equals("stale", ignoreCase = true),
                scopeLabel = "Current candidate")
            IdeHorizontalSeparator()
          }
          ReviewSection(
              title = "Apply",
              icon = DesktopIcon.Check,
              stateLabel = if (decision.eligible) "Ready to apply" else "Unavailable",
              stateTint = if (decision.eligible) Success else Warning) {
                if (decision.eligible) {
                  Text("Ready to apply", color = PrimaryText, fontWeight = FontWeight.SemiBold)
                  Text(
                      "Only the named declaration in the named file will change.",
                      color = SecondaryText,
                      fontSize = 12.sp,
                      modifier = Modifier.padding(top = 4.dp))
                  MiniOrcaButton(
                      onClick = applicationActions.apply,
                      tone = ActionTone.Positive,
                      modifier = Modifier.fillMaxWidth().padding(top = 8.dp)) {
                        Text(decision.actionLabel)
                      }
                } else {
                  Text("Apply unavailable", color = PrimaryText, fontWeight = FontWeight.SemiBold)
                  Text(
                      decision.reason,
                      color = Warning,
                      fontSize = 11.sp,
                      modifier = Modifier.padding(top = 5.dp))
                }
              }
        }
  }
}

/** Immutable daemon-derived review evidence for one file-scoped candidate. */
internal data class ReviewToolWindowState(
    val project: ProjectAnalysis?,
    val selected: ProjectFileInfo?,
    val selectedSymbol: SymbolInfo?,
    val session: ChatSession?,
    val editor: EditableDraftState?,
    val draft: DeclarationDraft?,
    val checks: DraftCheckReport?,
    val impact: ImpactPreview?,
    val gitStatus: GitStatus?,
    val applied: ApplyResult?,
    val checksRunning: Boolean,
)

/** Review and repair intents that leave guarded Apply and Undo separate. */
internal data class ReviewToolWindowActions(
    val runChecks: () -> Unit,
    val reviseWithCheckOutput: () -> Unit,
    val editDraft: () -> Unit,
)

/** The only source-mutating intents exposed by the review pane. */
internal data class DraftApplicationActions(
    val apply: () -> Unit,
    val undo: () -> Unit,
)

@Composable
private fun ReviewSection(
    title: String,
    icon: DesktopIcon,
    stateLabel: String,
    stateTint: Color,
    actions: @Composable RowScope.() -> Unit = {},
    content: @Composable () -> Unit,
) {
  Column(Modifier.fillMaxWidth()) {
    IdePaneHeader(
        title = title,
        icon = icon,
        stateLabel = stateLabel,
        stateTint = stateTint,
        actions = actions)
    Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp)) { content() }
    IdeHorizontalSeparator()
  }
}

@Composable
private fun ReviewDisclosureSection(
    title: String,
    icon: DesktopIcon,
    stateLabel: String,
    stateTint: Color,
    expanded: Boolean,
    onToggle: () -> Unit,
    content: @Composable () -> Unit,
) {
  Column(Modifier.fillMaxWidth()) {
    IdePaneHeader(
        title = title,
        icon = icon,
        stateLabel = stateLabel,
        stateTint = stateTint,
        expanded = expanded,
        onToggle = onToggle)
    if (expanded)
        Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp)) { content() }
    IdeHorizontalSeparator()
  }
}

@Composable
private fun ReviewEvidenceDetails(
    title: String,
    diagnostics: List<DeclarationFinding>,
    checks: List<DraftCheck>,
    expanded: Boolean,
    onToggle: () -> Unit,
) {
  val detailCount = diagnostics.size + checks.size
  IdeDisclosureHeader(
      title = title,
      expanded = expanded,
      onToggle = onToggle,
      stateLabel = "$detailCount ${if (detailCount == 1) "item" else "items"}")
  if (expanded)
      SelectionContainer {
        Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp)) {
          diagnostics.forEach { diagnostic ->
            Text(
                "${diagnostic.code}: ${diagnostic.message}",
                color = Error,
                fontSize = 11.sp,
                modifier = Modifier.padding(top = 3.dp))
          }
          checks.forEach { check ->
            Text(
                "${check.name} · ${check.state} · ${if (check.required) "required" else "optional"}",
                color = evidenceColor(checkStatus(check.state)),
                fontSize = 11.sp,
                modifier = Modifier.padding(top = 4.dp))
            if (check.command.isNotEmpty())
                Text(
                    "\$ ${check.command.joinToString(" ")}",
                    color = SecondaryText,
                    fontFamily = FontFamily.Monospace,
                    fontSize = 10.sp)
            if (check.output.isNotBlank())
                Text(
                    check.output,
                    color = SecondaryText,
                    fontFamily = FontFamily.Monospace,
                    fontSize = 10.sp,
                    modifier = Modifier.padding(top = 2.dp))
          }
        }
      }
}

@Composable
private fun EvidenceRow(row: ReviewEvidenceRow) {
  Text(row.label, color = PrimaryText, fontWeight = FontWeight.SemiBold, fontSize = 12.sp)
  Text(
      "${row.status.label} · ${row.detail}",
      color = evidenceColor(row.status),
      fontSize = 11.sp,
      modifier = Modifier.padding(top = 2.dp))
}

@Composable
private fun ReadOnlyImpactPane(impact: ImpactPreview?, gitStatus: GitStatus?) {
  var expanded by rememberSaveable { mutableStateOf(false) }
  ReviewDisclosureSection(
      title = "Project context",
      icon = DesktopIcon.Branch,
      stateLabel = "Advisory · read-only",
      stateTint = SecondaryText,
      expanded = expanded,
      onToggle = { expanded = !expanded }) {
        SectionLabel("ADVISORY IMPACT · READ-ONLY")
        if (impact?.references.isNullOrEmpty()) {
          Text(
              advisoryImpactLabel(impact),
              color = SecondaryText,
              fontSize = 11.sp,
              modifier = Modifier.padding(top = 4.dp))
        } else {
          Text(
              advisoryImpactLabel(impact),
              color = SecondaryText,
              fontSize = 11.sp,
              modifier = Modifier.padding(top = 4.dp))
          impact.references.forEach { reference ->
            Text(
                "${reference.confidence} · ${reference.path} · ${reference.reason}",
                color = SecondaryText,
                fontSize = 11.sp,
                modifier = Modifier.padding(top = 4.dp))
          }
        }
        Spacer(Modifier.height(10.dp))
        SectionLabel("GIT CONTEXT · READ-ONLY")
        Text(
            gitContextLabel(gitStatus),
            color = SecondaryText,
            fontSize = 11.sp,
            modifier = Modifier.padding(top = 4.dp))
      }
}

internal fun checkStatus(state: String): ReviewEvidenceStatus =
    when (state.lowercase()) {
      "passed" -> ReviewEvidenceStatus.Passed
      "skipped" -> ReviewEvidenceStatus.Skipped
      "running" -> ReviewEvidenceStatus.Running
      "failed",
      "error",
      "canceled",
      "cancelled" -> ReviewEvidenceStatus.Failed
      else -> ReviewEvidenceStatus.Missing
    }

internal fun evidenceColor(status: ReviewEvidenceStatus): Color =
    when (status) {
      ReviewEvidenceStatus.Passed -> Success
      ReviewEvidenceStatus.Running,
      ReviewEvidenceStatus.Skipped,
      ReviewEvidenceStatus.Missing,
      ReviewEvidenceStatus.Stale -> Warning
      ReviewEvidenceStatus.Failed -> Error
    }
