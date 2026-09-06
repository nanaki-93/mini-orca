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

/** Non-secret draft/check identity values shown only inside the technical disclosure. */
internal data class ReviewIdentityHashDetail(
    val label: String,
    val hash: String,
)

internal fun reviewIdentityHashDetails(
    candidateHash: String?,
    checkHash: String?,
): List<ReviewIdentityHashDetail> = buildList {
  candidateHash?.takeIf(String::isNotBlank)?.let {
    add(ReviewIdentityHashDetail("Candidate hash", it))
  }
  checkHash?.takeIf(String::isNotBlank)?.let {
    add(ReviewIdentityHashDetail("Check identity hash", it))
  }
}

internal data class ReviewEvidenceUiState(
    val validation: ReviewEvidenceRow,
    val checks: ReviewEvidenceRow,
    val identity: ReviewEvidenceRow,
    val canRunChecks: Boolean,
    val runChecksLabel: String,
)

internal enum class ReviewNextActionKind {
  EditDraft,
  RunChecks,
  ReviseWithCheckOutput,
  Apply,
  Undo,
  Waiting,
}

/**
 * One visible next step for Review. This only arranges the existing daemon-backed evidence and
 * eligibility result; it does not calculate an alternate Apply or check policy.
 */
internal data class ReviewNextActionUiState(
    val kind: ReviewNextActionKind,
    val label: String,
    val scope: String,
    val detail: String,
    val enabled: Boolean,
)

internal fun reviewNextActionUiState(
    evidence: ReviewEvidenceUiState,
    decision: ApplyDecisionUiState,
    draft: DeclarationDraft?,
    checks: DraftCheckReport?,
    session: ChatSession?,
    checksRunning: Boolean,
): ReviewNextActionUiState {
  val scope = draft?.let { "${it.targetSymbol} in ${it.targetPath}" } ?: "the selected declaration"
  if (decision.receiptTitle != null) {
    val undoAvailable = decision.undoLabel == "Undo this change"
    return ReviewNextActionUiState(
        ReviewNextActionKind.Undo,
        decision.undoLabel,
        scope,
        decision.receiptDetail,
        undoAvailable,
    )
  }
  if (decision.eligible)
      return ReviewNextActionUiState(
          ReviewNextActionKind.Apply,
          decision.actionLabel,
          scope,
          "Apply changes only this declaration in this file.",
          true,
      )
  if (evidence.validation.status == ReviewEvidenceStatus.Running)
      return ReviewNextActionUiState(
          ReviewNextActionKind.Waiting,
          "Validation is running",
          scope,
          "Wait for validation before focused checks or Apply.",
          false,
      )
  if (evidence.validation.status != ReviewEvidenceStatus.Passed)
      return ReviewNextActionUiState(
          ReviewNextActionKind.EditDraft,
          "Edit draft",
          scope,
          decision.reason,
          true,
      )
  if (checksRunning || evidence.checks.status == ReviewEvidenceStatus.Running)
      return ReviewNextActionUiState(
          ReviewNextActionKind.Waiting,
          "Focused checks are running",
          scope,
          "Wait for current focused check evidence before reviewing Apply.",
          false,
      )
  if (evidence.canRunChecks && evidence.checks.status != ReviewEvidenceStatus.Failed)
      return ReviewNextActionUiState(
          ReviewNextActionKind.RunChecks,
          evidence.runChecksLabel,
          scope,
          "Run focused checks for the validated declaration.",
          true,
      )
  if (evidence.checks.status == ReviewEvidenceStatus.Failed) {
    val repair = repairMessageForChecks(session, draft, checks)
    return ReviewNextActionUiState(
        if (repair != null) ReviewNextActionKind.ReviseWithCheckOutput
        else ReviewNextActionKind.EditDraft,
        if (repair != null) "Revise with check output" else "Edit draft",
        scope,
        if (repair != null) "Use the failed focused check evidence to revise this declaration."
        else "Edit this declaration before validating and checking it again.",
        true,
    )
  }
  return ReviewNextActionUiState(
      ReviewNextActionKind.EditDraft,
      "Edit draft",
      scope,
      decision.reason,
      true,
  )
}

/** Renders the workflow in order without inventing a second set of mutation guards. */
internal fun reviewProgressionRows(
    session: ChatSession?,
    draft: DeclarationDraft?,
    evidence: ReviewEvidenceUiState,
    decision: ApplyDecisionUiState,
): List<ReviewEvidenceRow> {
  val request =
      when {
        session == null ->
            ReviewEvidenceRow(
                "Request", "No bound request is loaded.", ReviewEvidenceStatus.Missing)
        draft != null && chatDraftMatchesSession(draft, session) ->
            ReviewEvidenceRow(
                "Request", "The request is bound to this candidate.", ReviewEvidenceStatus.Passed)
        else ->
            ReviewEvidenceRow(
                "Request",
                "The bound request no longer matches this candidate.",
                ReviewEvidenceStatus.Stale)
      }
  val draftRow =
      ReviewEvidenceRow(
          "Draft",
          evidence.identity.detail,
          evidence.identity.status,
      )
  val review =
      ReviewEvidenceRow(
          "Review",
          if (decision.eligible) "Guarded Apply is available for this candidate."
          else decision.reason,
          when {
            decision.eligible -> ReviewEvidenceStatus.Passed
            evidence.validation.status == ReviewEvidenceStatus.Stale ||
                evidence.checks.status == ReviewEvidenceStatus.Stale -> ReviewEvidenceStatus.Stale
            evidence.validation.status == ReviewEvidenceStatus.Failed ||
                evidence.checks.status == ReviewEvidenceStatus.Failed -> ReviewEvidenceStatus.Failed
            evidence.validation.status == ReviewEvidenceStatus.Running ||
                evidence.checks.status == ReviewEvidenceStatus.Running ->
                ReviewEvidenceStatus.Running
            else -> ReviewEvidenceStatus.Missing
          },
      )
  return listOf(request, draftRow, evidence.validation, evidence.checks, review)
}

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
  val nextAction =
      reviewNextActionUiState(
          evidence, decision, state.draft, state.checks, state.session, state.checksRunning)
  val progression = reviewProgressionRows(state.session, state.draft, evidence, decision)
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
                  ReviewNextAction(nextAction, evidenceActions, applicationActions)
                }
            return@Column
          }
          ReviewSection(
              title = "Progress",
              icon = DesktopIcon.Check,
              stateLabel = "Request → Draft → Validate → Checks → Review",
              stateTint = SecondaryText,
              actions = {
                if (state.editor != null && state.draft != null)
                    ChromeButton(
                        onClick = evidenceActions.editDraft, accessibleName = "Edit draft") {
                          Text("Edit draft", fontSize = 11.sp)
                        }
              }) {
                progression.forEachIndexed { index, row ->
                  if (index > 0) IdeHorizontalSeparator(Modifier.padding(vertical = 6.dp))
                  EvidenceRow(row)
                }
                checkFailurePreview(state.checks)?.let { preview ->
                  Text(
                      preview,
                      color = Error,
                      fontFamily = FontFamily.Monospace,
                      fontSize = 10.sp,
                      modifier = Modifier.padding(top = 5.dp))
                }
                val diagnostics = state.editor?.diagnostics.orEmpty()
                if (diagnostics.isNotEmpty())
                    ReviewEvidenceDetails(
                        title = "Validation diagnostics",
                        diagnostics = diagnostics,
                        checks = emptyList(),
                        candidateHash = state.draft?.hash,
                        expanded = diagnosticsExpanded,
                        onToggle = { diagnosticsExpanded = !diagnosticsExpanded })
              }
          val checksWithDetails =
              state.checks?.checks.orEmpty().isNotEmpty() ||
                  !state.draft?.hash.isNullOrBlank() ||
                  !state.checks?.draftHash.isNullOrBlank()
          if (checksWithDetails)
              ReviewEvidenceDetails(
                  title =
                      if (evidence.checks.status == ReviewEvidenceStatus.Failed)
                          "Failed check details"
                      else "Focused check details",
                  diagnostics = emptyList(),
                  checks = state.checks?.checks.orEmpty(),
                  candidateHash = state.draft?.hash,
                  checkHash = state.checks?.draftHash,
                  expanded = checksEvidenceExpanded,
                  onToggle = { checksEvidenceExpanded = !checksEvidenceExpanded })
          ReviewSection(
              title = "Next action",
              icon = DesktopIcon.Run,
              stateLabel = nextAction.scope,
              stateTint = if (nextAction.enabled) SelectionText else Warning) {
                ReviewNextAction(nextAction, evidenceActions, applicationActions)
                if (nextAction.kind == ReviewNextActionKind.Apply && evidence.canRunChecks)
                    ChromeButton(
                        onClick = evidenceActions.runChecks,
                        accessibleName = "Rerun focused checks") {
                          Text("Rerun focused checks", fontSize = 11.sp)
                        }
                if (nextAction.kind == ReviewNextActionKind.EditDraft &&
                    repairLimitReached(state.session, state.draft, state.checks))
                    Text(
                        "The repair limit is reached. Edit the draft manually.",
                        color = Warning,
                        fontSize = 11.sp,
                        modifier = Modifier.padding(top = 5.dp))
              }
          ReadOnlyImpactPane(state.impact, state.gitStatus)
          state.draft?.engineeringInsight?.let { insight ->
            EngineeringInsightPanel(
                insight,
                stale = state.draft.state.equals("stale", ignoreCase = true),
                scopeLabel = "Current candidate")
            IdeHorizontalSeparator()
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
private fun ReviewNextAction(
    action: ReviewNextActionUiState,
    evidenceActions: ReviewToolWindowActions,
    applicationActions: DraftApplicationActions,
) {
  Text(
      action.detail,
      color = SecondaryText,
      fontSize = 12.sp,
      modifier = Modifier.padding(top = 4.dp))
  if (action.kind == ReviewNextActionKind.Waiting) return
  val onClick =
      when (action.kind) {
        ReviewNextActionKind.EditDraft -> evidenceActions.editDraft
        ReviewNextActionKind.RunChecks -> evidenceActions.runChecks
        ReviewNextActionKind.ReviseWithCheckOutput -> evidenceActions.reviseWithCheckOutput
        ReviewNextActionKind.Apply -> applicationActions.apply
        ReviewNextActionKind.Undo -> applicationActions.undo
        ReviewNextActionKind.Waiting -> return
      }
  MiniOrcaButton(
      onClick = onClick,
      enabled = action.enabled,
      tone =
          if (action.kind == ReviewNextActionKind.Apply) ActionTone.Positive
          else ActionTone.Primary,
      modifier = Modifier.fillMaxWidth().padding(top = 8.dp)) {
        Text(action.label)
      }
}

internal fun checkFailurePreview(checks: DraftCheckReport?, limit: Int = 240): String? {
  val failed =
      checks?.checks?.firstOrNull {
        it.state.lowercase() in setOf("failed", "error", "canceled", "cancelled")
      } ?: return null
  val output = failed.output.replace(Regex("\\s+"), " ").trim()
  val summary = if (output.isBlank()) failed.name else "${failed.name}: $output"
  return summary.take(limit).let { if (summary.length > limit) "$it…" else it }
}

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
    candidateHash: String? = null,
    checkHash: String? = null,
    expanded: Boolean,
    onToggle: () -> Unit,
) {
  val identityHashes = reviewIdentityHashDetails(candidateHash, checkHash)
  val detailCount = diagnostics.size + checks.size + identityHashes.size
  IdeDisclosureHeader(
      title = title,
      expanded = expanded,
      onToggle = onToggle,
      stateLabel = "$detailCount ${if (detailCount == 1) "item" else "items"}")
  if (expanded)
      SelectionContainer {
        Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp)) {
          identityHashes.forEachIndexed { index, identity ->
            Text(
                "${identity.label}: ${identity.hash}",
                color = SecondaryText,
                fontFamily = FontFamily.Monospace,
                fontSize = 10.sp,
                modifier = Modifier.padding(top = if (index == 0) 0.dp else 3.dp))
          }
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
