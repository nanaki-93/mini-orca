package io.miniorca.desktop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import org.jetbrains.jewel.ui.component.Text

@Composable
internal fun DesktopAnalysisAdmissionOverlay(
    state: ProjectAnalysisRunState,
    presenter: DesktopWorkflowPresenter
) {
  if (state.showsAdmissionOverlay()) DesktopAnalysisAdmissionDialog(state, presenter)
}

@Composable
internal fun DesktopAnalysisAdmissionDialog(
    state: ProjectAnalysisRunState,
    presenter: DesktopWorkflowPresenter
) {
  IdeDialog(
      onDismissRequest = presenter::dismissAnalysisAdmission,
      title = { DesktopAnalysisAdmissionTitle(state) },
      content = {
        DesktopAnalysisAdmissionContent(
            state,
            presenter::confirmAnalysisProvider,
            presenter::confirmAnalysisSecurity,
            Modifier.fillMaxWidth())
      },
      actions = {
        DesktopAnalysisAdmissionActions(
            state,
            presenter::dismissAnalysisAdmission,
            presenter::startAnalysis,
            presenter::retryAnalysisPreview)
      })
}

private fun ProjectAnalysisRunState.previewMode(): String? =
    when {
      admission?.resumeRun != null || admission == null && previewIntent?.resumeRun != null ->
          "continuation"
      admission?.preview?.retryStaleFailed == true ||
          admission == null && previewIntent?.retryStaleFailed == true -> "stale & failed"
      admission != null || previewIntent != null -> "full project"
      else -> null
    }

@Composable
internal fun DesktopAnalysisAdmissionTitle(state: ProjectAnalysisRunState) {
  Text(
      when (state.previewMode()) {
        "continuation" -> "Continue project analysis"
        "stale & failed" -> "Analyze stale & failed"
        "full project" -> "Analyze whole project"
        else -> "Analysis preview"
      })
}

private fun ProjectAnalysisRunState.canRetryPreview(): Boolean {
  val intent = previewIntent ?: return false
  return action.isEmpty() &&
      error != null &&
      admission == null &&
      admissionRecovery != AdmissionRecovery.Uncertain &&
      (intent.resumeRun == null ||
          run?.identity == intent.resumeRun &&
              run.plan == intent.resumePlan &&
              run.status in setOf("paused", "interrupted"))
}

@Composable
internal fun RowScope.DesktopAnalysisAdmissionActions(
    state: ProjectAnalysisRunState,
    close: () -> Unit,
    start: () -> Unit,
    retry: () -> Unit,
) {
  MiniOrcaButton(onClick = close, tone = ActionTone.Neutral) { Text("Close") }
  val admission = state.admission
  if (admission != null) {
    MiniOrcaButton(
        onClick = start,
        enabled = admission.isConfirmed() && admission.preview.files.isNotEmpty(),
        tone = ActionTone.Primary) {
          Text(if (admission.resumeRun == null) "Start analysis" else "Resume analysis")
        }
  } else if (state.canRetryPreview()) {
    MiniOrcaButton(onClick = retry, tone = ActionTone.Primary) {
      Text(
          if (state.admissionRecovery == AdmissionRecovery.Rejected) "Review fresh preview"
          else "Retry preview")
    }
  }
}

/**
 * The scrollable preview owns all destinations; navigation and toggles only change local consent.
 */
@Composable
internal fun DesktopAnalysisAdmissionContent(
    state: ProjectAnalysisRunState,
    confirmProvider: (String, Boolean) -> Unit,
    confirmSecurity: (Boolean) -> Unit,
    modifier: Modifier = Modifier,
) {
  Column(modifier, verticalArrangement = Arrangement.spacedBy(8.dp)) {
    if (state.action == "preview") {
      Text(
          when (state.previewMode()) {
            "continuation" -> "Preparing continuation…"
            "stale & failed" -> "Preparing stale & failed preview…"
            else -> "Preparing project preview…"
          })
    } else if (state.admission == null) {
      state.error?.let { error ->
        Text(
            when (state.admissionRecovery) {
              AdmissionRecovery.Rejected -> "Preview expired"
              AdmissionRecovery.Uncertain -> "Start unconfirmed"
              null ->
                  when (state.previewMode()) {
                    "continuation" -> "Continuation preview failed."
                    "stale & failed" -> "Stale & failed preview failed."
                    "full project" -> "Full project preview failed."
                    else -> "Analysis preview unavailable."
                  }
            },
            color = Error)
        SelectionContainer { Text(error, color = Error) }
        Text(
            when {
              state.admissionRecovery == AdmissionRecovery.Rejected && state.canRetryPreview() ->
                  "Review a new ${state.previewMode()} preview · confirmations required"
              state.admissionRecovery == AdmissionRecovery.Rejected ->
                  "Scope unavailable · reopen Analysis for a new preview"
              state.admissionRecovery == AdmissionRecovery.Uncertain ->
                  "May have started · check Analysis status before retrying"
              state.canRetryPreview() -> "Retry preview"
              else -> "Open Analysis for a new preview"
            },
            color = SecondaryText)
      }
    } else {
      state.error?.let { error -> SelectionContainer { Text(error, color = Error) } }
    }
    val admission = state.admission
    if (admission != null) {
      val preview = admission.preview
      Text(
          "Bugs · Performance · Security", color = PrimaryText, style = IdeTypography.resultHeading)
      val included =
          "${preview.files.size} included ${if (preview.files.size == 1) "file" else "files"}"
      val exclusions = "${preview.excluded.size} excluded"
      Text(
          when {
            admission.resumeRun != null ->
                "Continuation (${admission.resumeRun.id}): ${preview.files.size} ${if (preview.files.size == 1) "file" else "files"} in the admitted file set · $exclusions"
            preview.retryStaleFailed -> "Stale & failed scope: $included · $exclusions"
            else -> "Full project scope: $included · $exclusions"
          },
          color = SecondaryText)
      if (preview.files.isEmpty())
          Text(
              if (preview.retryStaleFailed) "No stale or failed files to analyze."
              else "No eligible files to analyze.")
      Text("Your confirmation", color = PrimaryText, style = IdeTypography.resultHeading)
      Text(
          "May send source + project context to listed models · multiple requests possible",
          color = SecondaryText)
      val outstanding =
          preview.providers.filter {
            it.remoteConfirmationRequired && it.id !in admission.providerIds
          }
      val confirmationsNeeded =
          outstanding.size +
              if (preview.securityReviewIntentRequired && !admission.securityReview) 1 else 0
      IdeLabelBadge(
          label =
              if (confirmationsNeeded > 0)
                  "$confirmationsNeeded ${if (confirmationsNeeded == 1) "confirmation" else "confirmations"} needed"
              else if (preview.files.isEmpty()) "Confirmed · no eligible files" else "Confirmed",
          tint = if (confirmationsNeeded > 0) Warning else Success,
          icon = if (confirmationsNeeded > 0) DesktopIcon.Warning else DesktopIcon.Check)
      for (provider in preview.providers.filter { it.remoteConfirmationRequired }) {
        val checked = provider.id in admission.providerIds
        IdeCheckbox(
            checked = checked,
            onCheckedChange = { confirmProvider(provider.id, it) },
            accessibleName = provider.confirmationLabel(),
            stateLabel = if (checked) "Confirmed" else "Not confirmed",
            label = provider.confirmationLabel())
      }
      if (preview.securityReviewIntentRequired) {
        IdeHorizontalSeparator()
        IdeCheckbox(
            checked = admission.securityReview,
            onCheckedChange = confirmSecurity,
            accessibleName = "Include AI Security review",
            stateLabel = if (admission.securityReview) "Confirmed" else "Not confirmed",
            label = "Include AI Security review")
        Text("AI Security findings · unverified", color = SecondaryText)
      }
      IdeHorizontalSeparator()
      Text("Preview details", color = PrimaryText, style = IdeTypography.resultHeading)
      Text("Requests before retries: ${preview.expectedModelRequests}")
      Text("Request limit with retries: ${preview.maxModelRequests}")
      Text("Dispatch limit: ${preview.limits.batchFiles} files · ${preview.limits.budgetSeconds} s")
      Text("${preview.limits.maxAttemptsPerStage} attempts per stage")
      if (preview.compatibilityStage.isNotBlank())
          Text("Limited run · ${analysisStageLabel(preview.compatibilityStage)}", color = Warning)
      Text(if (preview.refresh) "Fresh evidence" else "Reuse eligible evidence")
      AnalysisStageSummary(preview)
      if (preview.files.isNotEmpty()) {
        IdeHorizontalSeparator()
        Text("Included files", color = PrimaryText, style = IdeTypography.resultHeading)
        // The disclosure belongs to this returned preview, not to the path in a later preview.
        key(preview.previewId, preview.identity) {
          preview.files.forEachIndexed { index, file ->
            key(index, file.path) { AnalysisIncludedFile(file, index + 1) }
          }
        }
      }
      if (preview.excluded.isNotEmpty()) {
        IdeHorizontalSeparator()
        Text("Excluded files", color = PrimaryText, style = IdeTypography.resultHeading)
        for (excluded in preview.excluded) {
          SelectionContainer {
            Column {
              Text(excluded.path)
              Text(excluded.reason, color = SecondaryText)
            }
          }
        }
      }
      for (provider in preview.providers) {
        IdeHorizontalSeparator()
        Text(
            provider.stages.joinToString(" · ", transform = ::analysisStageLabel),
            color = PrimaryText,
            style = IdeTypography.resultHeading)
        SelectionContainer {
          Column {
            Text("Scope: ${provider.model.scope.availableMetadata()}")
            Text("Profile: ${provider.model.profile.availableMetadata()}")
            Text("Model: ${provider.model.model.availableMetadata()}")
            Text(
                "${if (provider.model.remoteProvider) "Remote" else "Local"} destination: ${provider.model.providerOrigin.availableMetadata()}")
          }
        }
      }
    }
  }
}

@Composable
private fun AnalysisIncludedFile(file: AnalysisPlannedFile, number: Int) {
  var expanded by remember { mutableStateOf(false) }
  IdeDisclosureHeader(
      title = "Included file $number · ${file.path}",
      expanded = expanded,
      onToggle = { expanded = !expanded })
  SelectionContainer { Text(file.path, color = SecondaryText) }
  if (expanded) {
    if (file.stages.isEmpty()) Text("Stages unavailable", color = SecondaryText)
    file.stages.forEach { stage ->
      SelectionContainer {
        Column {
          Text(analysisStageLabel(stage.stage), color = PrimaryText)
          Text(if (stage.eligible) "Eligible" else "Not applicable")
          Text(if (stage.cached) "Cached/reused" else "Not cached")
          Text("Request limit: ${stage.maxModelRequests}")
          Text("Reason: ${stage.reason.availableMetadata()}", color = SecondaryText)
        }
      }
    }
  }
}

@Composable
private fun AnalysisStageSummary(preview: AnalysisRunPreview) {
  val stages = preview.files.flatMap { it.stages }.groupBy { it.stage }
  if (stages.isEmpty()) return
  IdeHorizontalSeparator()
  Text("Returned stage plan", color = PrimaryText, style = IdeTypography.resultHeading)
  for ((stageId, plans) in stages) {
    val applicable = plans.count { it.eligible }
    val cached = plans.count { it.cached }
    val requesting = plans.count { it.maxModelRequests > 0 }
    val nonRequesting = plans.count { it.maxModelRequests == 0 }
    Text(analysisStageLabel(stageId), color = PrimaryText)
    Text(
        "${plans.size} returned · $applicable applicable · $cached cached/reused · $requesting with model requests planned · $nonRequesting with no model requests planned",
        color = SecondaryText)
    if (stageId == "security_rules") {
      Text("Deterministic Security rules · no model destination", color = SecondaryText)
    } else {
      val providers = preview.providers.associateBy { it.id }
      for (providerId in plans.map { it.providerId }.distinct()) {
        val model = providers[providerId]?.model
        SelectionContainer {
          Text(
              if (model == null)
                  "Model destination: Unavailable (provider reference ${providerId.availableMetadata()})"
              else
                  "Model destination: ${model.model.availableMetadata()} · ${if (model.remoteProvider) "Remote" else "Local"} ${model.providerOrigin.availableMetadata()} (provider $providerId)",
              color = SecondaryText)
        }
      }
    }
  }
}

private fun AnalysisProviderRequirement.confirmationLabel(): String =
    "Confirm ${model.scope.availableMetadata()} destination · ${model.model.availableMetadata()} (provider ${id.availableMetadata()})"

private fun String.availableMetadata(): String = if (isBlank()) "Unavailable" else this

internal fun analysisStageLabel(stage: String): String =
    when (stage) {
      "semantic" -> "Code analysis"
      "performance" -> "Performance review"
      "security_rules" -> "Security rules"
      "security_ai" -> "AI Security review"
      else -> stage
    }
