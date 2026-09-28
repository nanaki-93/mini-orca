package io.miniorca.desktop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.runtime.Composable
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
      (intent.resumeRun == null ||
          run?.identity == intent.resumeRun && run.plan == intent.resumePlan)
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
    MiniOrcaButton(onClick = retry, tone = ActionTone.Primary) { Text("Retry preview") }
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
            when (state.previewMode()) {
              "continuation" -> "Continuation preview failed."
              "stale & failed" -> "Stale & failed preview failed."
              "full project" -> "Full project preview failed."
              else -> "Analysis preview unavailable."
            },
            color = Error)
        SelectionContainer { Text(error, color = Error) }
        Text(
            if (state.canRetryPreview()) "Retry this preview with the same scope, or Close."
            else "Close and request a new preview from Analysis.",
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
      Text(
          "Expected model requests: ${preview.expectedModelRequests} · Maximum: ${preview.maxModelRequests}")
      if (preview.files.isEmpty())
          Text(
              if (preview.retryStaleFailed) "No stale or failed files to analyze."
              else "No eligible files to analyze.")
      Text(
          "This window: ${preview.limits.batchFiles} files, ${preview.limits.budgetSeconds} seconds, up to ${preview.limits.maxAttemptsPerStage} attempts per stage. Remaining work requires an explicit continuation.")
      if (preview.compatibilityStage.isNotBlank())
          Text(
              "This saved run covers only ${analysisStageLabel(preview.compatibilityStage)}.",
              color = Warning)
      Text(
          if (preview.refresh) "Refresh policy: request fresh evidence for eligible stages."
          else "Reuse policy: refresh is off; eligible existing evidence may be reused.")
      if (preview.retryStaleFailed)
          Text("Selective retry: the daemon returned the stale & failed scope.")
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
        Text("${provider.model.scope} · ${provider.model.profile} · ${provider.model.model}")
        Text(
            "${if (provider.model.remoteProvider) "Remote" else "Local"} destination: ${provider.model.providerOrigin}")
        if (provider.remoteConfirmationRequired) {
          val checked = provider.id in admission.providerIds
          IdeCheckbox(
              checked = checked,
              onCheckedChange = { confirmProvider(provider.id, it) },
              accessibleName = "Confirm ${provider.model.scope} destination",
              stateLabel = if (checked) "Confirmed" else "Not confirmed",
              label = "Confirm ${provider.model.scope} destination")
        }
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
            "Review eligible project source for possible security issues. Findings remain unverified until reviewed.",
            color = SecondaryText)
      }
      Text(
          "Start sends the displayed context to the listed providers. It does not execute project code or change source files.",
          color = SecondaryText)
    }
  }
}

internal fun analysisStageLabel(stage: String): String =
    when (stage) {
      "semantic" -> "Code analysis"
      "performance" -> "Performance review"
      "security_rules" -> "Security rules"
      "security_ai" -> "AI Security review"
      else -> stage
    }
