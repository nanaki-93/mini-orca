package io.miniorca.desktop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
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
            else "Analyze whole project")
      },
      content = {
        DesktopAnalysisAdmissionContent(
            state,
            presenter::confirmAnalysisProvider,
            presenter::confirmAnalysisSecurity,
            Modifier.fillMaxWidth().heightIn(max = 360.dp).verticalScroll(rememberScrollState()))
      },
      actions = {
        MiniOrcaButton(onClick = presenter::dismissAnalysisAdmission, tone = ActionTone.Neutral) {
          Text("Close")
        }
        if (state.admission != null) {
          MiniOrcaButton(
              onClick = presenter::startAnalysis,
              enabled = state.admission.isConfirmed(),
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
      Text(
          "This window: ${preview.limits.batchFiles} files, ${preview.limits.budgetSeconds} seconds, up to ${preview.limits.maxAttemptsPerStage} attempts per stage. Remaining work requires an explicit continuation.")
      if (preview.compatibilityStage.isNotBlank())
          Text(
              "This saved run covers only ${analysisStageLabel(preview.compatibilityStage)}.",
              color = Warning)
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
          AnalysisConsentToggle("Confirm ${provider.model.scope} destination", checked) {
            confirmProvider(provider.id, !checked)
          }
        }
      }
      if (preview.securityReviewIntentRequired) {
        IdeHorizontalSeparator()
        AnalysisConsentToggle("Include AI Security review", admission.securityReview) {
          confirmSecurity(!admission.securityReview)
        }
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

@Composable
private fun AnalysisConsentToggle(label: String, checked: Boolean, onClick: () -> Unit) {
  ChromeButton(
      onClick = onClick,
      selected = checked,
      role = Role.Checkbox,
      accessibleName = label,
      modifier =
          Modifier.semantics { stateDescription = if (checked) "Confirmed" else "Not confirmed" }) {
        Text(if (checked) "✓" else "□")
        Spacer(Modifier.width(4.dp))
        Text(label)
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
