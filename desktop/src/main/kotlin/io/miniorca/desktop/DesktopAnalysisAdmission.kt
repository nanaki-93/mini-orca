package io.miniorca.desktop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
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
      title = {
        Text(
            if (state.admission?.resumeRun != null) "Continue project analysis"
            else if (state.admission?.preview?.retryStaleFailed == true) "Analyze stale & failed"
            else "Analyze whole project")
      },
      content = {
        DesktopAnalysisAdmissionContent(
            state,
            presenter::confirmAnalysisProvider,
            presenter::confirmAnalysisSecurity,
            Modifier.fillMaxWidth())
      },
      actions = {
        MiniOrcaButton(onClick = presenter::dismissAnalysisAdmission, tone = ActionTone.Neutral) {
          Text("Close")
        }
        if (state.admission != null) {
          MiniOrcaButton(
              onClick = presenter::startAnalysis,
              enabled = state.admission.isConfirmed() && state.admission.preview.files.isNotEmpty(),
              tone = ActionTone.Primary) {
                Text(if (state.admission.resumeRun == null) "Start analysis" else "Resume analysis")
              }
        } else if (state.action != "preview") {
          MiniOrcaButton(onClick = { presenter.previewAnalysis() }, tone = ActionTone.Primary) {
            Text("New preview")
          }
        }
      })
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
    if (state.action == "preview") Text("Preparing project scope and provider estimates…")
    state.error?.let { Text(it, color = Error) }
    val admission = state.admission
    if (admission != null) {
      val preview = admission.preview
      Text(
          "Bugs · Performance · Security", color = PrimaryText, style = IdeTypography.resultHeading)
      Text(
          "${preview.files.size} project ${if (preview.files.size == 1) "file" else "files"} · ${preview.excluded.size} excluded",
          color = SecondaryText)
      Text(
          "Expected model requests: ${preview.expectedModelRequests} · Maximum: ${preview.maxModelRequests}")
      if (preview.files.isEmpty()) {
        Text(
            if (preview.retryStaleFailed) "No stale or failed files to analyze."
            else "No eligible files to analyze.")
        return@Column
      }
      Text(
          "This window: ${preview.limits.batchFiles} files, ${preview.limits.budgetSeconds} seconds, up to ${preview.limits.maxAttemptsPerStage} attempts per stage. Remaining work requires an explicit continuation.")
      if (preview.compatibilityStage.isNotBlank())
          Text(
              "This saved run covers only ${analysisStageLabel(preview.compatibilityStage)}.",
              color = Warning)
      if (preview.retryStaleFailed) {
        Text(
            "Only files with stale or failed analysis are included. Fresh stages reuse their results.")
      }
      if (preview.refresh) Text("Refresh requests fresh evidence for eligible stages.")
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
