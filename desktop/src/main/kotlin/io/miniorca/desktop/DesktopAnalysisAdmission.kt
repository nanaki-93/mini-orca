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
  if (state.admission != null || state.action == "preview" || state.error != null)
      DesktopAnalysisAdmissionDialog(state, presenter)
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
            "continuation" -> "Preparing continuation preview for this analysis run…"
            "stale & failed" -> "Preparing stale & failed analysis preview…"
            else -> "Preparing full project analysis preview…"
          })
    } else if (state.admission == null) {
      state.error?.let { error ->
        Text(
            when (state.admissionRecovery) {
              AdmissionRecovery.Rejected -> "This preview can no longer be admitted."
              AdmissionRecovery.Uncertain -> "The admission response is uncertain."
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
                  "Review a fresh ${state.previewMode()} preview before starting or resuming. All destinations and Security intent must be confirmed again."
              state.admissionRecovery == AdmissionRecovery.Rejected ->
                  "This scope is no longer available for review. Close and choose a current Analysis action (Resume analysis if available for the current run) to request a new preview."
              state.admissionRecovery == AdmissionRecovery.Uncertain ->
                  "Analysis may already have started. Close and check the current Analysis run before choosing a new Analysis action; do not retry this admission."
              state.canRetryPreview() -> "Retry this preview with the same scope, or Close."
              else -> "Close and request a new preview from Analysis."
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
      Text("Reviewing this preview sends nothing to a model.", color = SecondaryText)
      if (preview.files.isEmpty())
          Text(
              if (preview.retryStaleFailed) "No stale or failed files to analyze."
              else "No eligible files to analyze.")
      Text("Your confirmation", color = PrimaryText, style = IdeTypography.resultHeading)
      Text(
          "Start or Resume may send eligible source and project context for this previewed scope to the listed models under the existing context policy. This preview describes the plan, not the exact content sent to a model. One Start or Resume may initiate multiple model requests; analysis does not execute project code or modify source files. This consent does not grant function-edit permission or execution trust.",
          color = SecondaryText)
      val outstanding =
          preview.providers.filter {
            it.remoteConfirmationRequired && it.id !in admission.providerIds
          }
      when {
        outstanding.isEmpty() &&
            (!preview.securityReviewIntentRequired || admission.securityReview) ->
            Text(
                if (preview.files.isEmpty()) "Confirmations complete; no included files to analyze."
                else "Confirmations complete for this preview.",
                color = SecondaryText)
        else -> {
          Text("Still needed before Start or Resume:", color = Warning)
          for (provider in outstanding) Text(
              "Destination still needed: ${provider.confirmationLabel()}", color = Warning)
          if (preview.securityReviewIntentRequired && !admission.securityReview)
              Text("Security intent still needed: Include AI Security review.", color = Warning)
        }
      }
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
        Text(
            "AI Security review of eligible source is advisory. Model findings are unverified, not a verified scan or safety assurance. This acknowledgment does not change the returned stage plan.",
            color = SecondaryText)
      }
      IdeHorizontalSeparator()
      Text("Preview details", color = PrimaryText, style = IdeTypography.resultHeading)
      Text("Expected model requests without retries: ${preview.expectedModelRequests}")
      Text("Inclusive maximum model requests with retries: ${preview.maxModelRequests}")
      Text(
          "Dispatch window: up to ${preview.limits.batchFiles} files and ${preview.limits.budgetSeconds} seconds. These limits bound this dispatch, not the project inventory or an ETA.")
      Text(
          "Up to ${preview.limits.maxAttemptsPerStage} attempts per stage, including the initial attempt. Further work requires explicit continuation.")
      if (preview.compatibilityStage.isNotBlank())
          Text(
              "This saved run covers only ${analysisStageLabel(preview.compatibilityStage)}.",
              color = Warning)
      Text(
          if (preview.refresh) "Refresh policy: request fresh evidence for eligible stages."
          else "Reuse policy: refresh is off; eligible existing evidence may be reused.")
      if (preview.retryStaleFailed)
          Text("Selective retry: the daemon returned the stale & failed scope.")
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
    if (file.stages.isEmpty()) Text("No stages returned for this file.", color = SecondaryText)
    file.stages.forEach { stage ->
      SelectionContainer {
        Column {
          Text(analysisStageLabel(stage.stage), color = PrimaryText)
          Text(if (stage.eligible) "Eligible stage" else "Ineligible stage on included file")
          Text(if (stage.cached) "Cached/reused" else "Not cached")
          Text("Maximum model requests for this stage: ${stage.maxModelRequests}")
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
