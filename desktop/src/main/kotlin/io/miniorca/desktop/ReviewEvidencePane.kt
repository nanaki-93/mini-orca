package io.miniorca.desktop

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.TooltipArea
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
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
  if (evidence.validation.status == ReviewEvidenceStatus.Running)
      return ReviewNextActionUiState(
          ReviewNextActionKind.Waiting,
          "Validation is running",
          scope,
          "Wait for validation before focused checks or Apply.",
          false,
      )
  if (checksRunning || evidence.checks.status == ReviewEvidenceStatus.Running)
      return ReviewNextActionUiState(
          ReviewNextActionKind.Waiting,
          "Focused checks are running",
          scope,
          "Wait for current focused check evidence before reviewing Apply.",
          false,
      )
  if (decision.eligible)
      return ReviewNextActionUiState(
          ReviewNextActionKind.Apply,
          decision.actionLabel,
          scope,
          "Apply changes only this declaration in this file.",
          true,
      )
  if (evidence.validation.status != ReviewEvidenceStatus.Passed)
      return ReviewNextActionUiState(
          ReviewNextActionKind.EditDraft,
          "Edit draft",
          scope,
          decision.reason,
          true,
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
          if (evidence.validation.status == ReviewEvidenceStatus.Running ||
              evidence.checks.status == ReviewEvidenceStatus.Running)
              "Wait for current evidence before reviewing Apply."
          else if (decision.eligible) "Guarded Apply is available for this candidate."
          else decision.reason,
          when {
            evidence.validation.status == ReviewEvidenceStatus.Running ||
                evidence.checks.status == ReviewEvidenceStatus.Running ->
                ReviewEvidenceStatus.Running
            decision.eligible -> ReviewEvidenceStatus.Passed
            evidence.validation.status == ReviewEvidenceStatus.Stale ||
                evidence.checks.status == ReviewEvidenceStatus.Stale -> ReviewEvidenceStatus.Stale
            evidence.validation.status == ReviewEvidenceStatus.Failed ||
                evidence.checks.status == ReviewEvidenceStatus.Failed -> ReviewEvidenceStatus.Failed
            else -> ReviewEvidenceStatus.Missing
          },
      )
  return listOf(request, draftRow, evidence.validation, evidence.checks, review)
}

internal fun editorProgressionRows(state: ReviewToolWindowState): List<ReviewEvidenceRow> =
    reviewProgressionRows(
        state.session,
        state.draft,
        reviewEvidenceUiState(
            state.project,
            state.selected,
            state.editor,
            state.draft,
            state.checks,
            state.checksRunning),
        applyDecisionUiState(
            state.project, state.selected, state.editor, state.draft, state.checks, state.applied),
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
                  when {
                    draft == null || editor == null -> ReviewEvidenceStatus.Missing
                    identityCurrent -> ReviewEvidenceStatus.Passed
                    else -> ReviewEvidenceStatus.Stale
                  },
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
      !repairTaskSpecMatches(session.taskSpec, draft.taskSpec) ||
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
        repairTaskSpecMatches(session.taskSpec, draft.taskSpec) &&
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
  DiffViewer(draft?.validation?.diff, modifier.fillMaxSize().padding(8.dp))
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
  BoxWithConstraints(modifier.fillMaxSize().background(ToolWindowSurface)) {
    val maximumActionHeight = maxHeight * 0.5f
    val scroll = rememberScrollState()
    Column(Modifier.fillMaxSize()) {
      Column(Modifier.weight(1f).verticalScroll(scroll).testTag("review-scroll")) {
        if (state.applied == null)
            ReviewTargetHeader(
                reviewToolWindowScope(
                    state.selected, state.selectedSymbol, state.draft, state.applied),
                if (state.editor != null &&
                    state.draft != null &&
                    nextAction.kind != ReviewNextActionKind.EditDraft)
                    evidenceActions.editDraft
                else null)
        Column(
            Modifier.fillMaxWidth().padding(horizontal = 12.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)) {
              if (decision.receiptTitle != null) {
                ReviewReadiness(
                    decision.receiptTitle, decision.receiptDetail, ReviewEvidenceStatus.Passed)
                Text(
                    if (state.applied?.undoAvailable == true) "Undo available"
                    else "Undo unavailable",
                    color = if (state.applied?.undoAvailable == true) Success else Warning,
                    style = IdeTypography.workspaceMetadata)
              } else {
                ReviewReadiness(
                    if (state.draft == null) "No candidate"
                    else reviewReadinessTitle(evidence, decision),
                    when {
                      evidence.validation.status == ReviewEvidenceStatus.Running ->
                          evidence.validation.detail
                      nextAction.kind == ReviewNextActionKind.Waiting -> evidence.checks.detail
                      decision.eligible -> "Validation and check evidence match this candidate."
                      else -> decision.reason
                    },
                    if (decision.eligible && nextAction.kind != ReviewNextActionKind.Waiting)
                        ReviewEvidenceStatus.Passed
                    else reviewReadinessStatus(evidence))
                Column {
                  listOf(
                          evidence.validation,
                          evidence.checks,
                          evidence.identity.copy(label = "Source unchanged"))
                      .forEach { row ->
                        EvidenceRow(row)
                        IdeHorizontalSeparator()
                      }
                }
                requiredChecksSummary(state.checks, state.draft, state.checksRunning)?.let {
                  Text(it, color = SecondaryText, style = IdeTypography.workspaceMetadata)
                }
                checkFailurePreview(state.checks)?.let {
                  Text(it, color = Error, style = IdeTypography.workspaceBody)
                }
                DraftValidationDiagnostics(state.editor?.diagnostics.orEmpty())
                ReviewDetails(state, evidence, nextAction, evidenceActions)
                ReadOnlyImpactPane(state.impact, state.gitStatus)
                state.draft?.engineeringInsight?.let { insight ->
                  EngineeringInsightPanel(
                      insight,
                      stale = state.draft.state.equals("stale", ignoreCase = true),
                      scopeLabel = "Current candidate")
                }
              }
            }
        Spacer(Modifier.height(12.dp))
      }
      Box(
          Modifier.heightIn(max = maximumActionHeight)
              .verticalScroll(rememberScrollState())
              .testTag("review-action-scroll")) {
            ReviewActionRegion(state, nextAction, evidenceActions, applicationActions)
          }
    }
  }
}

internal fun reviewReadinessTitle(
    evidence: ReviewEvidenceUiState,
    decision: ApplyDecisionUiState
): String =
    when {
      evidence.validation.status == ReviewEvidenceStatus.Running -> "Validating draft"
      evidence.checks.status == ReviewEvidenceStatus.Running -> "Checks running"
      decision.eligible -> "Ready to apply"
      evidence.validation.status == ReviewEvidenceStatus.Failed -> "Validation failed"
      evidence.identity.status == ReviewEvidenceStatus.Stale -> "Candidate needs attention"
      evidence.validation.status != ReviewEvidenceStatus.Passed -> "Validation needed"
      evidence.checks.status == ReviewEvidenceStatus.Failed -> "Checks failed"
      evidence.checks.status == ReviewEvidenceStatus.Stale -> "Checks are stale"
      evidence.checks.status == ReviewEvidenceStatus.Skipped -> "Checks skipped"
      else -> "Checks needed"
    }

private fun reviewReadinessStatus(evidence: ReviewEvidenceUiState): ReviewEvidenceStatus =
    listOf(evidence.validation, evidence.checks, evidence.identity)
        .firstOrNull { it.status != ReviewEvidenceStatus.Passed }
        ?.status ?: ReviewEvidenceStatus.Missing

internal fun requiredChecksSummary(
    checks: DraftCheckReport?,
    draft: DeclarationDraft?,
    running: Boolean
): String? {
  if (running) return "Checks are running."
  if (checks == null) return null
  if (!checksMatchDraft(checks, draft)) return "Results belong to an earlier candidate."
  if (!checks.applicable) return "Required check evidence is unavailable."
  val required = checks.checks.filter { it.required }
  if (required.isEmpty()) return "No required checks reported."
  val passed = required.count { checkStatus(it.state) == ReviewEvidenceStatus.Passed }
  return "$passed of ${required.size} required ${if (required.size == 1) "check" else "checks"} passed"
}

@Composable
private fun ReviewReadiness(title: String, detail: String, status: ReviewEvidenceStatus) {
  val tint = evidenceColor(status)
  Column(
      Modifier.fillMaxWidth()
          .clip(MiniOrcaShapes.interactiveCard)
          .background(labelBadgeBackground(tint))
          .border(1.dp, tint, MiniOrcaShapes.interactiveCard)
          .padding(14.dp),
      verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(10.dp)) {
              ReviewEvidenceMarker(status)
              Text(title, color = tint, style = IdeTypography.workspaceHeading)
            }
        Text(detail, color = PrimaryText, style = IdeTypography.workspaceMetadata)
      }
}

@Composable
private fun ReviewDetails(
    state: ReviewToolWindowState,
    evidence: ReviewEvidenceUiState,
    next: ReviewNextActionUiState,
    actions: ReviewToolWindowActions
) {
  var expanded by
      rememberSaveable(
          state.draft?.id, state.draft?.revision, state.draft?.hash, state.checks?.draftHash) {
            mutableStateOf(false)
          }
  if (state.checks?.checks.orEmpty().isNotEmpty() ||
      !state.draft?.hash.isNullOrBlank() ||
      !state.checks?.draftHash.isNullOrBlank()) {
    ReviewEvidenceDetails(
        title =
            if (evidence.checks.status == ReviewEvidenceStatus.Failed) "Failed check details"
            else "Check details",
        checks = state.checks?.checks.orEmpty(),
        candidateHash = state.draft?.hash,
        checkHash = state.checks?.draftHash,
        expanded = expanded,
        onToggle = { expanded = !expanded })
    if (expanded &&
        evidence.canRunChecks &&
        next.kind !in setOf(ReviewNextActionKind.RunChecks, ReviewNextActionKind.Waiting)) {
      if (state.draft?.taskSpec?.goTestCandidate != null) ReviewExecutionScope(state.draft)
      MiniOrcaButton(
          onClick = actions.runChecks,
          modifier = Modifier.fillMaxWidth(),
          tone = ActionTone.Neutral) {
            Text(
                if (state.draft?.taskSpec?.goTestCandidate != null)
                    "Trust local execution & rerun checks"
                else "Rerun focused checks",
                style = IdeTypography.action)
          }
    }
  }
}

@Composable
private fun ReviewExecutionScope(draft: DeclarationDraft?) {
  Text(
      "Trust local execution for this project revision:",
      color = PrimaryText,
      style = IdeTypography.workspaceMetadata)
  Text(draftProjectCodeCommand(draft), color = PrimaryText, style = IdeTypography.resultCode)
  Text(
      "gofmt and go vet are source-only.", color = SecondaryText, style = IdeTypography.compactBody)
}

@Composable
private fun ReviewActionRegion(
    state: ReviewToolWindowState,
    action: ReviewNextActionUiState,
    evidenceActions: ReviewToolWindowActions,
    applicationActions: DraftApplicationActions,
) {
  if (state.draft == null && state.applied == null) return
  Column(
      Modifier.fillMaxWidth()
          .background(ToolWindowSurface)
          .padding(12.dp)
          .testTag("review-action-region"),
      verticalArrangement = Arrangement.spacedBy(8.dp)) {
        IdeHorizontalSeparator(Modifier.padding(bottom = 8.dp))
        if (action.kind == ReviewNextActionKind.Apply) {
          Text("Apply this change", color = PrimaryText, style = IdeTypography.workspaceHeading)
          Text(
              "1 declaration · 1 file",
              color = SecondaryText,
              style = IdeTypography.workspaceMetadata)
        }
        if (action.kind == ReviewNextActionKind.RunChecks &&
            state.draft?.taskSpec?.goTestCandidate != null)
            ReviewExecutionScope(state.draft)
        if (action.kind == ReviewNextActionKind.EditDraft &&
            repairLimitReached(state.session, state.draft, state.checks)) {
          Text(
              "The repair limit is reached. Edit the draft manually.",
              color = Warning,
              style = IdeTypography.workspaceMetadata)
        }
        if (action.kind == ReviewNextActionKind.Undo)
            Text(action.detail, color = PrimaryText, style = IdeTypography.workspaceMetadata)
        ReviewNextAction(action, state.draft, evidenceActions, applicationActions)
        if (action.kind == ReviewNextActionKind.Apply)
            Text(
                "Updates ${action.scope}.",
                color = SecondaryText,
                style = IdeTypography.workspaceMetadata)
      }
}

internal fun draftProjectCodeCommand(draft: DeclarationDraft?): String {
  val taskName = draft?.taskSpec?.goTestCandidate?.name?.takeIf { it.isNotBlank() }
  return if (taskName == null) "go test ./..." else "go test ./... -run ^$taskName$"
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
    draft: DeclarationDraft?,
    evidenceActions: ReviewToolWindowActions,
    applicationActions: DraftApplicationActions,
) {
  if (action.kind == ReviewNextActionKind.Waiting) {
    Text(action.detail, color = Information, style = IdeTypography.workspaceMetadata)
    return
  }
  val onClick =
      when (action.kind) {
        ReviewNextActionKind.EditDraft -> evidenceActions.editDraft
        ReviewNextActionKind.RunChecks -> evidenceActions.runChecks
        ReviewNextActionKind.ReviseWithCheckOutput -> evidenceActions.reviseWithCheckOutput
        ReviewNextActionKind.Apply -> applicationActions.apply
        ReviewNextActionKind.Undo -> applicationActions.undo
        ReviewNextActionKind.Waiting -> return
      }
  val label =
      when {
        action.kind == ReviewNextActionKind.Apply -> "Apply change"
        action.kind == ReviewNextActionKind.RunChecks && draft?.taskSpec?.goTestCandidate != null ->
            "Trust local execution & run checks"
        else -> action.label
      }
  MiniOrcaButton(
      onClick = onClick,
      enabled = action.enabled,
      tone =
          if (action.kind == ReviewNextActionKind.Apply) ActionTone.PositivePrimary
          else ActionTone.Primary,
      modifier =
          Modifier.fillMaxWidth().heightIn(min = 48.dp).semantics {
            contentDescription =
                if (action.kind == ReviewNextActionKind.Apply) action.label else label
          }) {
        if (action.kind == ReviewNextActionKind.Apply) {
          DesktopLineIcon(
              DesktopIcon.Branch, "One declaration change", tint = OnActionFill, iconSize = 18.dp)
          Spacer(Modifier.width(8.dp))
        }
        Text(label, style = IdeTypography.workspaceBody.copy(fontWeight = FontWeight.SemiBold))
      }
}

internal fun checkFailurePreview(checks: DraftCheckReport?, limit: Int = 240): String? {
  val failed =
      checks?.checks?.firstOrNull {
        it.state.lowercase() in setOf("failed", "error", "canceled", "cancelled")
      } ?: return null
  val output = sanitizedOutputText(failed.output).replace(Regex("\\s+"), " ").trim()
  val summary = if (output.isBlank()) failed.name else "${failed.name}: $output"
  return summary.take(limit).let { if (summary.length > limit) "$it…" else it }
}

@Composable
private fun ReviewDisclosureSection(
    title: String,
    icon: DesktopIcon,
    expanded: Boolean,
    onToggle: () -> Unit,
    content: @Composable () -> Unit,
) {
  Column(Modifier.fillMaxWidth()) {
    IdePaneHeader(title = title, icon = icon, expanded = expanded, onToggle = onToggle)
    if (expanded)
        Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp)) { content() }
    IdeHorizontalSeparator()
  }
}

@Composable
private fun ReviewEvidenceDetails(
    title: String,
    checks: List<DraftCheck>,
    candidateHash: String? = null,
    checkHash: String? = null,
    expanded: Boolean,
    onToggle: () -> Unit,
) {
  val identityHashes = reviewIdentityHashDetails(candidateHash, checkHash)
  IdeDisclosureHeader(
      title = title,
      expanded = expanded,
      onToggle = onToggle,
      modifier = Modifier.heightIn(min = 40.dp))
  if (expanded)
      SelectionContainer {
        Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp)) {
          identityHashes.forEachIndexed { index, identity ->
            Text(
                "${identity.label}: ${identity.hash}",
                color = SecondaryText,
                style = IdeTypography.resultCode,
                modifier = Modifier.padding(top = if (index == 0) 0.dp else 3.dp))
          }
          checks.forEach { check ->
            Text(
                sanitizedOutputText(check.name, 256),
                color = PrimaryText,
                style = IdeTypography.resultHeading,
                modifier = Modifier.padding(top = 12.dp))
            IdeLabelBadge(
                "${checkStatus(check.state).label} · ${if (check.required) "Required" else "Optional"}",
                evidenceColor(checkStatus(check.state)),
                Modifier.padding(vertical = 4.dp))
            if (check.command.isNotEmpty())
                DiagnosticText("\$ ${check.command.joinToString(" ")}", color = SecondaryText)
            if (check.output.isNotBlank())
                DiagnosticText(check.output, Modifier.padding(top = 2.dp))
          }
        }
      }
}

@Composable
@OptIn(ExperimentalFoundationApi::class)
private fun EvidenceRow(row: ReviewEvidenceRow) {
  TooltipArea(tooltip = { IdeControlTooltip(row.detail) }) {
    Row(
        Modifier.fillMaxWidth().heightIn(min = 44.dp).semantics(mergeDescendants = true) {
          contentDescription = "${row.label}: ${row.detail}"
          stateDescription = row.status.label
        },
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(8.dp)) {
          ReviewEvidenceMarker(row.status)
          Text(
              row.label,
              color = PrimaryText,
              style = IdeTypography.workspaceBody,
              modifier = Modifier.weight(1f))
          IdeLabelBadge(row.status.label, evidenceColor(row.status))
        }
  }
}

@Composable
private fun ReviewEvidenceMarker(status: ReviewEvidenceStatus) {
  val tint = evidenceColor(status)
  val passed = status == ReviewEvidenceStatus.Passed
  Box(
      Modifier.size(22.dp * LocalDensity.current.fontScale)
          .background(if (passed) tint else ToolWindowSurface, MiniOrcaShapes.pill)
          .border(1.dp, tint, MiniOrcaShapes.pill),
      contentAlignment = Alignment.Center) {
        if (passed)
            DesktopLineIcon(
                DesktopIcon.Check,
                status.label,
                tint = EditorCanvas,
                iconSize = 14.dp * LocalDensity.current.fontScale)
        else
            Text(
                when (status) {
                  ReviewEvidenceStatus.Running -> "…"
                  ReviewEvidenceStatus.Failed -> "×"
                  ReviewEvidenceStatus.Skipped -> "–"
                  else -> "!"
                },
                color = tint,
                style = IdeTypography.compactBody)
      }
}

@Composable
private fun ReadOnlyImpactPane(impact: ImpactPreview?, gitStatus: GitStatus?) {
  var expanded by rememberSaveable { mutableStateOf(false) }
  ReviewDisclosureSection(
      title = "Project context",
      icon = DesktopIcon.Branch,
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
      ReviewEvidenceStatus.Running -> Information
      ReviewEvidenceStatus.Skipped,
      ReviewEvidenceStatus.Missing,
      ReviewEvidenceStatus.Stale -> Warning
      ReviewEvidenceStatus.Failed -> Error
    }
