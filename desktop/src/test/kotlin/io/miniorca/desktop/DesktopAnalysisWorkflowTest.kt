package io.miniorca.desktop

import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import kotlin.coroutines.CoroutineContext
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json

class DesktopAnalysisWorkflowTest {
  @Test
  fun startRefreshesIncludedFilesAndResumePreservesTheRefreshChoice() {
    Harness().use { h ->
      h.workflow.preview()
      h.drain()
      val preview = Json.decodeFromString<AnalysisPreviewRequest>(h.bodies.single())
      assertTrue(preview.refresh)
      assertFalse(preview.retryStaleFailed)
      assertFalse(h.calls.any { it.first == "POST" && it.second.endsWith("/run") })
      h.confirm()
      h.workflow.admit()
      h.drain()
      val start = Json.decodeFromString<AnalysisRunStartRequest>(h.bodies.last())
      assertTrue(start.refresh)
      assertFalse(start.retryStaleFailed)
      h.workflow.preview(resume = true)
      h.drain()
      val resume = Json.decodeFromString<AnalysisPreviewRequest>(h.bodies.last())
      assertTrue(resume.refresh)
      assertEquals(h.run.identity, resume.resumeRun)
    }
  }

  @Test
  fun summaryPreviewDismissalAndEmptyScopeNeverAdmitWork() {
    for (emptyScope in listOf(false, true)) {
      Harness().use { h ->
        h.emptyPreview = emptyScope
        // Summary's Start callback delegates to this preview with the default full-run policy.
        h.workflow.preview(defaultAnalysisRunLimits, resume = false)
        h.drain()
        val request = Json.decodeFromString<AnalysisPreviewRequest>(h.bodies.single())
        assertEquals(defaultAnalysisRunLimits, request.limits)
        assertTrue(request.refresh)
        assertFalse(request.retryStaleFailed)
        assertNull(request.resumeRun)
        val admission = assertNotNull(h.state.analysisRun.admission)
        assertFalse(admission.isConfirmed())
        assertEquals(emptyScope, admission.preview.files.isEmpty())
        h.workflow.admit()
        h.drain()
        h.assertPreviewOnly()
        if (emptyScope) {
          h.confirm()
          assertTrue(h.state.analysisRun.admission!!.isConfirmed())
          h.workflow.admit()
          h.drain()
          assertTrue(h.state.analysisRun.error!!.contains("No eligible files"))
          h.assertPreviewOnly()
        }
        h.workflow.dismissAdmission()
        h.workflow.admit()
        h.drain()
        assertNull(h.state.analysisRun.admission)
        h.assertPreviewOnly()
      }
    }
  }

  @Test
  fun failedSummaryPreviewRetainsProjectEvidenceAndRequiresExplicitRetry() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      val project = h.state.project
      val sections = h.state.analysisRun.sections
      val run = h.state.analysisRun.run
      val summary = projectSummaryPresentation(null, project, run, sections)
      h.calls.clear()
      h.bodies.clear()
      h.failure = "/analysis/preview"
      h.workflow.preview(defaultAnalysisRunLimits, resume = false)
      h.drain()
      assertNull(h.state.analysisRun.admission)
      assertTrue(h.state.analysisRun.error!!.contains("unavailable"))
      assertEquals(AnalysisRunErrorKind.Preview, h.state.analysisRun.errorKind)
      assertTrue(h.state.analysisRun.showsAdmissionOverlay())
      assertEquals(project, h.state.project)
      assertEquals(sections, h.state.analysisRun.sections)
      assertEquals(
          summary,
          projectSummaryPresentation(
              null, h.state.project, h.state.analysisRun.run, h.state.analysisRun.sections))
      h.assertPreviewOnly()
      h.drain()
      assertEquals(1, h.calls.count { it.first == "POST" })
      h.failure = ""
      h.workflow.retryPreview()
      h.drain()
      assertNotNull(h.state.analysisRun.admission)
      assertNull(h.state.analysisRun.error)
      assertEquals(2, h.calls.count { it.first == "POST" })
      h.assertPreviewOnly()
    }
  }

  @Test
  fun failedPreviewsRetryTheCapturedRequestWithoutAdmittingWork() {
    for (mode in listOf("full", "selective", "resume")) {
      Harness().use { h ->
        val limits = AnalysisRunLimits(7, 120, 3)
        if (mode == "resume") {
          h.run =
              h.run.copy(
                  plan = h.run.plan.copy(limits = limits, refresh = true, retryStaleFailed = true))
          h.workflow.refresh()
          h.drain()
          h.calls.clear()
          h.bodies.clear()
        }
        h.failure = "/analysis/preview"
        when (mode) {
          "full" -> h.workflow.preview(limits = limits, refresh = false)
          "selective" -> h.workflow.preview(limits = limits, retryStaleFailed = true)
          else -> h.workflow.preview(resume = true)
        }
        h.drain()
        val requested = Json.decodeFromString<AnalysisPreviewRequest>(h.bodies.single())
        assertEquals(limits, requested.limits, mode)
        assertEquals(mode == "selective" || mode == "resume", requested.retryStaleFailed, mode)
        assertEquals(mode == "resume", requested.resumeRun != null, mode)
        assertEquals(mode == "resume", requested.refresh, mode)
        assertEquals(requested, h.state.analysisRun.previewIntent?.request(), mode)
        assertNull(h.state.analysisRun.admission, mode)
        h.assertPreviewOnly()
        h.failure = ""
        h.workflow.retryPreview()
        h.drain()
        assertEquals(2, h.bodies.size, mode)
        assertEquals(
            requested, Json.decodeFromString<AnalysisPreviewRequest>(h.bodies.last()), mode)
        assertNotNull(h.state.analysisRun.admission, mode)
        h.assertPreviewOnly()
        h.confirm()
        h.workflow.admit()
        h.drain()
        val expectedEndpoint = if (mode == "resume") "/control" else "/run"
        assertEquals(
            1, h.calls.count { it.first == "POST" && it.second.endsWith(expectedEndpoint) }, mode)
        h.workflow.retryPreview()
        h.drain()
        assertEquals(
            2, h.calls.count { it.first == "POST" && it.second.endsWith("/preview") }, mode)
      }
    }
  }

  @Test
  fun obsoleteResumeCannotRetryOrRestoreReview() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      h.failure = "/analysis/preview"
      h.workflow.preview(resume = true)
      h.drain()
      assertNotNull(h.state.analysisRun.previewIntent)
      val before = h.calls.count { it.second.endsWith("/preview") }
      h.dispatch(
          DesktopEvent.AnalysisRunUpdated(
              h.state.analysisRun.copy(
                  run = h.run.copy(identity = h.run.identity.copy(generation = "replacement")))))
      h.failure = ""
      h.workflow.retryPreview()
      h.drain()
      assertEquals(before, h.calls.count { it.second.endsWith("/preview") })
      assertNull(h.state.analysisRun.admission)
      assertNull(h.state.analysisRun.previewIntent)
      h.assertPreviewOnly()
    }
  }

  @Test
  fun lateRetryAndProviderChangeCannotRestoreReview() {
    for (change in listOf("dismiss", "provider", "replacement")) {
      Harness().use { h ->
        h.failure = "/analysis/preview"
        h.workflow.preview(retryStaleFailed = true)
        h.drain()
        h.failure = ""
        h.workflow.retryPreview()
        h.main.runPending()
        h.io.runPending()
        when (change) {
          "dismiss" -> h.workflow.dismissAdmission()
          "provider" -> h.workflow.providerChanged()
          else -> {
            h.failure = "/analysis/preview"
            h.workflow.preview(limits = AnalysisRunLimits(5, 100, 2))
          }
        }
        h.drain()
        assertNull(h.state.analysisRun.admission, change)
        assertEquals(change == "replacement", h.state.analysisRun.previewIntent != null, change)
        assertFalse(h.calls.any { it.first == "POST" && it.second.endsWith("/run") }, change)
      }
    }
  }

  @Test
  fun failedIntentIsDiscardedOnProjectProviderAndDetachChanges() {
    for (change in listOf("project", "revision", "provider", "detach")) {
      Harness().use { h ->
        h.failure = "/analysis/preview"
        h.workflow.preview(limits = AnalysisRunLimits(6, 90, 2))
        h.drain()
        assertNotNull(h.state.analysisRun.previewIntent, change)
        when (change) {
          "project" ->
              h.dispatch(
                  DesktopEvent.ProjectLoaded(
                      analysisProjectFixture("other"), ProjectIndex("other", "revision")))
          "revision" -> h.dispatch(DesktopEvent.IndexRefreshed(ProjectIndex("project", "new")))
          "provider" -> h.workflow.providerChanged()
          else -> h.workflow.detach()
        }
        h.drain()
        assertNull(h.state.analysisRun.previewIntent, change)
        h.failure = ""
        h.workflow.retryPreview()
        h.drain()
        assertEquals(1, h.calls.count { it.second.endsWith("/preview") }, change)
        h.assertPreviewOnly()
      }
    }
  }

  @Test
  fun changedResumePolicyAndLateResponseCannotRestoreConsent() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      h.workflow.preview(resume = true)
      h.main.runPending()
      h.io.runPending()
      h.run = h.run.copy(plan = h.run.plan.copy(limits = AnalysisRunLimits(3, 60, 1)))
      h.dispatch(DesktopEvent.AnalysisRunUpdated(h.state.analysisRun.copy(run = h.run)))
      h.drain()
      assertNull(h.state.analysisRun.previewIntent)
      assertNull(h.state.analysisRun.admission)
      h.workflow.retryPreview()
      h.drain()
      assertEquals(1, h.calls.count { it.second.endsWith("/preview") })
      h.assertPreviewOnly()
    }
  }

  @Test
  fun successfulResumePreviewLosesConsentWhenCapturedRunChanges() {
    for (change in listOf("identity", "plan")) {
      Harness().use { h ->
        h.workflow.refresh()
        h.drain()
        h.workflow.preview(resume = true)
        h.drain()
        h.confirm()
        assertTrue(h.state.analysisRun.admission!!.isConfirmed(), change)
        assertNotNull(h.state.analysisRun.previewIntent, change)
        val changedRun =
            when (change) {
              "identity" -> h.run.copy(identity = h.run.identity.copy(generation = "replacement"))
              else -> h.run.copy(plan = h.run.plan.copy(limits = AnalysisRunLimits(3, 60, 1)))
            }
        h.dispatch(DesktopEvent.AnalysisRunUpdated(h.state.analysisRun.copy(run = changedRun)))
        assertNull(h.state.analysisRun.previewIntent, change)
        assertNull(h.state.analysisRun.admission, change)
        h.workflow.admit()
        h.workflow.retryPreview()
        h.drain()
        assertEquals(1, h.calls.count { it.second.endsWith("/preview") }, change)
        h.assertPreviewOnly()
      }
    }
  }

  @Test
  fun polledResumePlanChangeInvalidatesConfirmedAdmission() {
    Harness(pollMillis = 15).use { h ->
      h.run = h.run.copy(status = "paused")
      h.workflow.refresh()
      h.drain()
      h.workflow.preview(resume = true)
      h.drain()
      h.confirm()
      assertTrue(h.state.analysisRun.admission!!.isConfirmed())
      assertNotNull(h.state.analysisRun.previewIntent)

      h.run = h.run.copy(plan = h.run.plan.copy(limits = AnalysisRunLimits(3, 60, 1)))
      h.workflow.refresh()
      h.drain()
      assertEquals(h.run.plan, h.state.analysisRun.run?.plan)
      val statusUpdate = h.runUpdates.first { it.run?.plan == h.run.plan }
      assertNull(statusUpdate.previewIntent)
      assertNull(statusUpdate.admission)
      assertEquals(h.run.identity, h.state.analysisRun.run?.identity)
      assertNull(h.state.analysisRun.previewIntent)
      assertNull(h.state.analysisRun.admission)
      h.workflow.admit()
      h.workflow.retryPreview()
      h.drain()
      assertEquals(1, h.calls.count { it.second.endsWith("/preview") })
      h.assertPreviewOnly()
    }
  }

  @Test
  fun dismissalReplacementAndSelectionSaveSuppressLatePreviewResponses() {
    for (change in listOf("dismiss", "replacement", "selection")) {
      Harness().use { h ->
        if (change == "selection") h.run = h.run.copy(status = "completed")
        h.workflow.refresh()
        h.drain()
        h.calls.clear()
        h.bodies.clear()
        if (change == "selection") h.failure = "/analysis/preview"
        h.workflow.preview(retryStaleFailed = true)
        if (change == "selection") h.drain()
        else {
          h.main.runPending()
          h.io.runPending()
        }
        when (change) {
          "dismiss" -> h.workflow.dismissAdmission()
          "replacement" -> h.workflow.preview(limits = AnalysisRunLimits(4, 80, 1))
          else -> h.workflow.fileSelection.save(listOf("main.go"))
        }
        h.drain()
        assertTrue(h.state.analysisRun.admission == null || change == "replacement", change)
        assertEquals(change == "replacement", h.state.analysisRun.previewIntent != null, change)
        if (change == "replacement")
            assertEquals(4, h.state.analysisRun.admission?.preview?.limits?.batchFiles)
        assertFalse(h.calls.any { it.first == "POST" && it.second.endsWith("/run") }, change)
        h.workflow.retryPreview()
        h.drain()
        assertEquals(
            if (change == "replacement") 2 else 1,
            h.calls.count { it.second.endsWith("/preview") },
            change)
      }
    }
  }

  @Test
  fun pendingSummaryPreviewCannotAttachAfterProjectOrRevisionChange() {
    for (change in listOf("project", "revision")) {
      Harness().use { h ->
        h.workflow.preview(defaultAnalysisRunLimits, resume = false)
        h.main.runPending()
        h.io.runPending() // The old response is queued for delivery on the main dispatcher.
        when (change) {
          "project" ->
              h.dispatch(
                  DesktopEvent.ProjectLoaded(
                      analysisProjectFixture("other"), ProjectIndex("other", "revision")))
          else -> h.dispatch(DesktopEvent.IndexRefreshed(ProjectIndex("project", "new")))
        }
        h.drain()
        assertNull(h.state.analysisRun.admission, change)
        assertEquals("", h.state.analysisRun.action, change)
        assertEquals(if (change == "project") "other" else "project", h.state.project?.projectId)
        assertEquals(
            if (change == "project") "revision" else "new", h.state.project?.projectRevision)
        h.workflow.admit()
        h.drain()
        h.assertPreviewOnly()
      }
    }
  }

  @Test
  fun staleFailedSelectionTravelsThroughPreviewStartAndResume() {
    Harness().use { h ->
      h.run = h.run.copy(plan = h.run.plan.copy(retryStaleFailed = true))
      h.workflow.preview(retryStaleFailed = true)
      h.drain()
      val preview = Json.decodeFromString<AnalysisPreviewRequest>(h.bodies.single())
      assertTrue(preview.retryStaleFailed)
      assertFalse(preview.refresh)
      assertTrue(h.state.analysisRun.admission!!.preview.retryStaleFailed)
      h.confirm()
      h.workflow.admit()
      h.drain()
      val start = Json.decodeFromString<AnalysisRunStartRequest>(h.bodies.last())
      assertTrue(start.retryStaleFailed)
      h.workflow.preview(resume = true)
      h.drain()
      val resume = Json.decodeFromString<AnalysisPreviewRequest>(h.bodies.last())
      assertTrue(resume.retryStaleFailed)
      assertEquals(h.run.identity, resume.resumeRun)
    }
  }

  @Test
  fun mismatchedRetryPreviewIsRejectedBeforeAdmission() {
    Harness().use { h ->
      h.wrongRetryPreview = true
      h.workflow.preview(retryStaleFailed = true)
      h.drain()
      assertNull(h.state.analysisRun.admission)
      assertTrue(h.state.analysisRun.error!!.contains("preview no longer matches"))
      assertFalse(h.calls.any { it.first == "POST" && it.second.endsWith("/run") })
    }
  }

  @Test
  fun admissionRequiresEveryDestinationAndSecurityIntentAndConsumesThemBeforeDispatch() {
    Harness().use { h ->
      h.workflow.preview()
      h.drain()
      assertEquals(listOf("POST" to "/api/projects/current/analysis/preview"), h.calls)
      h.workflow.admit()
      h.drain()
      assertEquals(1, h.calls.size)
      h.workflow.confirmProvider("bug-provider", true)
      h.workflow.admit()
      h.drain()
      assertEquals(1, h.calls.size)
      h.workflow.confirmProvider("analyze-provider", true)
      h.workflow.confirmSecurity(true)
      assertTrue(h.state.analysisRun.admission!!.isConfirmed())
      h.workflow.admit()
      assertNull(h.state.analysisRun.admission)
      h.drain()
      assertEquals(1, h.calls.count { it.first == "POST" && it.second.endsWith("/run") })
      assertEquals(h.run, h.state.analysisRun.run)
      val start =
          Json.decodeFromString<AnalysisRunStartRequest>(
              h.bodies.first { it.contains("confirmations") })
      assertEquals(
          setOf("bug-provider", "analyze-provider"), start.confirmations.providerIds.toSet())
      assertTrue(start.confirmations.securityReview)
      assertEquals(3, h.state.analysisRun.sections.size)
      h.workflow.admit()
      h.drain()
      assertEquals(1, h.calls.count { it.second.endsWith("/run") && it.first == "POST" })
    }
  }

  @Test
  fun rejectedAdmissionRetainsOnlySameScopeIntentForExplicitUncheckedReview() {
    for (mode in listOf("full", "selective", "resume")) {
      Harness().use { h ->
        val limits = AnalysisRunLimits(7, 120, 3)
        if (mode == "resume") {
          h.run =
              h.run.copy(
                  plan = h.run.plan.copy(limits = limits, refresh = true, retryStaleFailed = true))
          h.workflow.refresh()
          h.drain()
          h.calls.clear()
          h.bodies.clear()
        }
        when (mode) {
          "full" -> h.workflow.preview(limits = limits, refresh = false)
          "selective" -> h.workflow.preview(limits = limits, retryStaleFailed = true)
          else -> h.workflow.preview(resume = true)
        }
        h.drain()
        val original = Json.decodeFromString<AnalysisPreviewRequest>(h.bodies.single())
        val previewId = h.state.analysisRun.admission!!.preview.previewId
        h.confirm()
        h.admissionConflict = true
        h.workflow.admit()
        h.workflow.admit() // The first activation consumes the admission before transport.
        h.drain()
        assertNull(h.state.analysisRun.admission, mode)
        assertEquals(original, h.state.analysisRun.previewIntent?.request(), mode)
        assertEquals(AdmissionRecovery.Rejected, h.state.analysisRun.admissionRecovery, mode)
        assertEquals(AnalysisRunErrorKind.Admission, h.state.analysisRun.errorKind, mode)
        assertTrue(h.state.analysisRun.showsAdmissionOverlay(), mode)
        assertTrue(h.state.analysisRun.error!!.contains("rejected"), mode)
        assertEquals(h.run, h.state.analysisRun.run, mode)
        val endpoint = if (mode == "resume") "/control" else "/run"
        assertEquals(1, h.calls.count { it.first == "POST" && it.second.endsWith(endpoint) }, mode)
        assertEquals(1, h.calls.count { it.second.endsWith("/preview") }, mode)
        assertTrue(h.calls.any { it.first == "GET" && it.second.contains("/analysis/run?") })
        if (mode == "resume") {
          val request = Json.decodeFromString<AnalysisRunControlRequest>(h.bodies.last())
          assertEquals(h.run.identity, request.identity)
          assertEquals(previewId, request.previewId)
          assertEquals(
              setOf("bug-provider", "analyze-provider"),
              request.confirmations!!.providerIds.toSet())
          assertTrue(request.confirmations.securityReview)
        } else {
          val request = Json.decodeFromString<AnalysisRunStartRequest>(h.bodies.last())
          assertEquals(previewId, request.previewId)
          assertEquals(h.state.analysisRun.run!!.identity.queue(), request.identity)
          assertEquals(limits, request.limits)
          assertEquals(original.refresh, request.refresh)
          assertEquals(original.retryStaleFailed, request.retryStaleFailed)
          assertEquals(
              setOf("bug-provider", "analyze-provider"), request.confirmations.providerIds.toSet())
        }
        h.admissionConflict = false
        h.previewId = "replacement-preview"
        h.workflow.retryPreview()
        h.drain()
        assertEquals(2, h.calls.count { it.second.endsWith("/preview") }, mode)
        assertEquals(original, Json.decodeFromString<AnalysisPreviewRequest>(h.bodies.last()), mode)
        val replacement = assertNotNull(h.state.analysisRun.admission, mode)
        assertEquals("replacement-preview", replacement.preview.previewId)
        assertFalse(replacement.isConfirmed(), mode)
        assertTrue(replacement.providerIds.isEmpty())
        assertFalse(replacement.securityReview)
        assertNull(h.state.analysisRun.admissionRecovery)
        assertNull(h.state.analysisRun.error)
        assertEquals(1, h.calls.count { it.first == "POST" && it.second.endsWith(endpoint) }, mode)
        h.workflow.admit() // No fresh confirmation, even with the same providers.
        h.drain()
        assertEquals(1, h.calls.count { it.first == "POST" && it.second.endsWith(endpoint) }, mode)
      }
    }
  }

  @Test
  fun rejectedAdmissionDoesNotAllowUnknownOrPartialConsentOrObsoleteResumeRecovery() {
    Harness().use { h ->
      h.workflow.preview()
      h.drain()
      h.workflow.confirmProvider("unknown", true)
      h.workflow.confirmProvider("bug-provider", true)
      h.workflow.confirmSecurity(true)
      h.workflow.admit()
      h.drain()
      h.assertPreviewOnly()
      h.workflow.confirmProvider("analyze-provider", true)
      h.workflow.confirmProvider("bug-provider", false)
      h.workflow.admit()
      h.drain()
      h.assertPreviewOnly()
      h.workflow.confirmProvider("bug-provider", true)
      h.admissionConflict = true
      h.workflow.admit()
      h.drain()
      assertEquals(1, h.calls.count { it.first == "POST" && it.second.endsWith("/run") })
      assertNull(h.state.analysisRun.admission)
    }
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      h.workflow.preview(resume = true)
      h.drain()
      h.confirm()
      h.admissionConflict = true
      h.workflow.admit()
      h.drain()
      val before = h.calls.count { it.second.endsWith("/preview") }
      assertNotNull(h.state.analysisRun.previewIntent)
      h.dispatch(
          DesktopEvent.AnalysisRunUpdated(
              h.state.analysisRun.copy(
                  run = h.run.copy(plan = h.run.plan.copy(refresh = !h.run.plan.refresh)))))
      h.workflow.retryPreview()
      h.drain()
      assertNull(h.state.analysisRun.previewIntent)
      assertEquals(before, h.calls.count { it.second.endsWith("/preview") })
      assertEquals(1, h.calls.count { it.first == "POST" && it.second.endsWith("/control") })
    }
  }

  @Test
  fun rejectedStartCannotRetryAfterSelectionProjectOrProviderChanges() {
    for (change in listOf("selection", "project", "revision", "provider")) {
      Harness().use { h ->
        if (change == "selection") h.run = h.run.copy(status = "completed")
        h.workflow.refresh()
        h.drain()
        h.workflow.preview()
        h.drain()
        h.confirm()
        h.admissionConflict = true
        h.workflow.admit()
        h.drain()
        assertNotNull(h.state.analysisRun.previewIntent, change)
        val before = h.calls.count { it.second.endsWith("/preview") }
        when (change) {
          "selection" -> h.workflow.fileSelection.save(listOf("main.go"))
          "project" ->
              h.dispatch(
                  DesktopEvent.ProjectLoaded(
                      analysisProjectFixture("other"), ProjectIndex("other", "revision")))
          "revision" -> h.dispatch(DesktopEvent.IndexRefreshed(ProjectIndex("project", "new")))
          else -> h.workflow.providerChanged()
        }
        h.drain()
        if (change == "selection") assertEquals(listOf("main.go"), h.selection.excludedPaths)
        h.admissionConflict = false
        h.workflow.retryPreview()
        h.drain()
        assertNull(h.state.analysisRun.previewIntent, change)
        assertNull(h.state.analysisRun.admission, change)
        assertEquals(before, h.calls.count { it.second.endsWith("/preview") }, change)
        assertEquals(1, h.calls.count { it.first == "POST" && it.second.endsWith("/run") }, change)
      }
    }
  }

  @Test
  fun uncertainAdmissionReadsDurableStateWithoutRetryingOrReusingConsent() {
    Harness().use { h ->
      h.workflow.preview()
      h.drain()
      h.confirm()
      h.failure = "/analysis/run"
      h.workflow.admit()
      h.drain()
      assertNull(h.state.analysisRun.admission)
      assertNull(h.state.analysisRun.previewIntent)
      assertEquals(AdmissionRecovery.Uncertain, h.state.analysisRun.admissionRecovery)
      assertEquals(AnalysisRunErrorKind.Admission, h.state.analysisRun.errorKind)
      assertTrue(h.state.analysisRun.showsAdmissionOverlay())
      assertEquals(h.run, h.state.analysisRun.run)
      assertTrue(h.state.analysisRun.error!!.contains("unavailable"))
      h.workflow.retryPreview()
      h.workflow.admit()
      h.drain()
      assertEquals(1, h.calls.count { it == "POST" to "/api/projects/current/analysis/run" })
      assertTrue(h.calls.any { it.first == "GET" && it.second.contains("/analysis/run?") })
    }
  }

  @Test
  fun dismissingAdmissionErrorsDoesNotGrantConsentOrRetry() {
    for (failure in listOf("preview", "rejected", "uncertain")) Harness().use { h ->
      h.workflow.preview()
      if (failure == "preview") h.failure = "/analysis/preview"
      h.drain()
      if (failure != "preview") {
        h.confirm()
        if (failure == "rejected") h.admissionConflict = true else h.failure = "/analysis/run"
        h.workflow.admit()
        h.drain()
      }
      assertTrue(h.state.analysisRun.showsAdmissionOverlay(), failure)
      val posts = h.calls.count { it.first == "POST" }
      h.workflow.dismissAdmission()
      h.drain()
      assertFalse(h.state.analysisRun.showsAdmissionOverlay(), failure)
      assertNull(h.state.analysisRun.admission, failure)
      assertNull(h.state.analysisRun.previewIntent, failure)
      assertNull(h.state.analysisRun.admissionRecovery, failure)
      h.workflow.admit()
      h.workflow.retryPreview()
      h.drain()
      assertEquals(posts, h.calls.count { it.first == "POST" }, failure)
      assertTrue(h.calls.none { it.first == "POST" && it.second.endsWith("/control") }, failure)
    }
  }

  @Test
  fun resumeStatusChangeAfterActivationProducesDismissibleAdmissionErrorWithoutDispatch() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      h.workflow.preview(resume = true)
      h.drain()
      h.confirm()
      val evidence = h.state.analysisRun.sections
      val previewPosts = h.calls.count { it.first == "POST" }

      h.workflow.admit() // Consumes preview and queues dispatch on the main dispatcher.
      assertNull(h.state.analysisRun.previewIntent)
      assertNull(h.state.analysisRun.admission)
      h.run = h.run.copy(status = "canceled")
      h.dispatch(DesktopEvent.AnalysisRunUpdated(h.state.analysisRun.copy(run = h.run)))
      h.drain()

      assertEquals("canceled", h.state.analysisRun.run?.status)
      assertEquals(evidence.keys, h.state.analysisRun.sections.keys)
      assertTrue(h.state.analysisRun.sections.values.all { it.results?.identity == h.run.identity })
      assertNull(h.state.analysisRun.admissionRecovery)
      assertEquals(AnalysisRunErrorKind.Admission, h.state.analysisRun.errorKind)
      assertTrue(h.state.analysisRun.error!!.contains("scope changed"))
      assertTrue(h.state.analysisRun.showsAdmissionOverlay())
      assertEquals(previewPosts, h.calls.count { it.first == "POST" })

      h.workflow.dismissAdmission()
      h.drain()
      assertFalse(h.state.analysisRun.showsAdmissionOverlay())
      assertNull(h.state.analysisRun.error)
      assertNull(h.state.analysisRun.errorKind)
      assertNull(h.state.analysisRun.previewIntent)
      h.workflow.admit()
      h.workflow.retryPreview()
      h.drain()
      assertEquals(previewPosts, h.calls.count { it.first == "POST" })
      assertEquals("canceled", h.state.analysisRun.run?.status)
    }
  }

  @Test
  fun admissionRecoverySurvivesFailedStatusReconciliationWithoutBecomingARunDialog() {
    Harness().use { h ->
      h.workflow.preview()
      h.drain()
      h.confirm()
      h.admissionConflict = true
      h.failStatus = true
      h.workflow.admit()
      h.drain()
      assertNull(h.state.analysisRun.admission)
      assertEquals(AdmissionRecovery.Rejected, h.state.analysisRun.admissionRecovery)
      assertEquals(AnalysisRunErrorKind.Admission, h.state.analysisRun.errorKind)
      assertTrue(h.state.analysisRun.showsAdmissionOverlay())
      assertTrue(h.state.analysisRun.error!!.contains("status could not be read"))
      assertEquals(1, h.calls.count { it.first == "POST" && it.second.endsWith("/run") })
    }
  }

  @Test
  fun dismissalReconcilesAdmissionThatArrivesAfterTheFirstStatusRead() {
    for (resume in listOf(false, true)) {
      Harness(pollMillis = 15).use { h ->
        h.workflow.refresh()
        h.drain()
        val evidence = h.state.analysisRun.sections
        h.workflow.preview(resume = resume)
        h.drain()
        h.confirm()
        val entered = CountDownLatch(1)
        val release = CountDownLatch(1)
        h.lateAdmissionGate = entered to release
        h.workflow.admit()
        h.main.runPending()
        val transport = Thread { h.io.runPending() }
        transport.start()
        try {
          assertTrue(entered.await(5, TimeUnit.SECONDS), "Admission did not reach transport")
          h.workflow.dismissAdmission()
          h.main.runPending()
          h.io.runPending() // Status still reports the old, paused run.
          h.main.runPending()
          assertEquals("paused", h.state.analysisRun.run?.status)
          assertTrue(h.calls.any { it.first == "GET" && it.second.contains("/analysis/run?") })
          assertNull(h.state.analysisRun.admission)
          assertNull(h.state.analysisRun.previewIntent)
          release.countDown()
          transport.join(5_000)
          assertFalse(transport.isAlive, "Admission transport did not finish")
          h.drain()
          assertEquals("running", h.state.analysisRun.run?.status)
          assertEquals("admitted", h.state.analysisRun.run?.updatedAt)
          assertEquals(evidence.keys, h.state.analysisRun.sections.keys)
          assertNull(h.state.analysisRun.admission)
          assertNull(h.state.analysisRun.previewIntent)
          h.run = h.run.copy(updatedAt = "later progress")
          h.await { h.state.analysisRun.run?.updatedAt == "later progress" }
          h.workflow.admit()
          h.drain()
          assertEquals(
              1,
              h.calls.count {
                it.first == "POST" && it.second.endsWith(if (resume) "/control" else "/run")
              })
        } finally {
          release.countDown()
          transport.join(5_000)
        }
      }
    }
  }

  @Test
  fun delayedUncertainAdmissionReconcilesNewIdentityAfterOldStatusAndPreservesError() {
    for (resume in listOf(false, true)) {
      for (dismiss in listOf(false, true)) {
        Harness(pollMillis = 15).use { h ->
          h.run = h.run.copy(status = if (resume) "paused" else "running")
          h.workflow.refresh()
          h.drain()
          val oldRun = h.run
          val evidence = h.state.analysisRun.sections
          h.workflow.preview(resume = resume)
          h.drain()
          h.confirm()
          val entered = CountDownLatch(1)
          val release = CountDownLatch(1)
          h.lateAdmissionGate = entered to release
          h.lateAdmissionNewIdentity = true
          h.admissionTransportFailure = true
          h.workflow.admit()
          h.main.runPending()
          val transport = Thread { h.io.runPending() }
          transport.start()
          try {
            assertTrue(entered.await(5, TimeUnit.SECONDS), "Admission did not reach transport")
            if (dismiss) {
              h.workflow.dismissAdmission()
              h.main.runPending()
              h.io.runPending() // The dismissed dialog reads the old active run.
              h.main.runPending()
            }
            h.staleStatus = oldRun.copy(status = "running")
            h.staleStatusReads = 1
            release.countDown()
            transport.join(5_000)
            assertFalse(transport.isAlive, "Admission transport did not finish")
            h.main.runPending() // Failure schedules a read-only status request.
            h.io.runPending() // This response still sees the old active run.
            h.main.runPending()
            assertTrue(
                h.runUpdates.any {
                  it.run?.identity == oldRun.identity && it.error?.contains("unavailable") == true
                })
            assertTrue(h.state.analysisRun.error!!.contains("unavailable"))
            assertNull(h.state.analysisRun.admission)
            assertNull(h.state.analysisRun.previewIntent)
            h.await { h.state.analysisRun.run?.identity == h.run.identity }
            assertEquals("admitted", h.state.analysisRun.run?.updatedAt)
            assertTrue(h.state.analysisRun.error!!.contains("unavailable"))
            assertFalse(h.state.analysisRun.error!!.contains("replaced"))
            assertEquals(evidence.keys, h.state.analysisRun.sections.keys)
            h.workflow.retryPreview()
            h.workflow.admit()
            h.drain()
            assertEquals(
                1,
                h.calls.count {
                  it.first == "POST" && it.second.endsWith(if (resume) "/control" else "/run")
                })
          } finally {
            release.countDown()
            transport.join(5_000)
          }
        }
      }
    }
  }

  @Test
  fun lateDismissalStatusCannotReplaceUncertainRecoveryPollOrClearTransportError() {
    for (resume in listOf(false, true)) {
      Harness(pollMillis = 15).use { h ->
        h.run = h.run.copy(status = if (resume) "paused" else "running")
        h.workflow.refresh()
        h.drain()
        val oldRun = h.run
        h.workflow.preview(resume = resume)
        h.drain()
        h.confirm()
        val admissionEntered = CountDownLatch(1)
        val releaseAdmission = CountDownLatch(1)
        val statusEntered = CountDownLatch(1)
        val releaseStatus = CountDownLatch(1)
        h.lateAdmissionGate = admissionEntered to releaseAdmission
        h.lateAdmissionNewIdentity = true
        h.admissionTransportFailure = true
        h.lateStatusGate = statusEntered to releaseStatus
        h.workflow.admit()
        h.main.runPending()
        val admissionThread = Thread { h.io.runPending() }
        admissionThread.start()
        val statusThread = Thread { h.io.runPending() }
        try {
          assertTrue(admissionEntered.await(5, TimeUnit.SECONDS))
          h.workflow.dismissAdmission()
          h.main.runPending()
          statusThread.start()
          assertTrue(statusEntered.await(5, TimeUnit.SECONDS))
          h.staleStatus = oldRun.copy(status = "running")
          h.staleStatusReads = 1
          releaseAdmission.countDown()
          admissionThread.join(5_000)
          assertFalse(admissionThread.isAlive)
          h.main.runPending() // The failed transport schedules recovery status.
          h.io.runPending() // Recovery sees the old run and starts polling.
          h.main.runPending()
          assertEquals(oldRun.identity, h.state.analysisRun.run?.identity)
          assertTrue(h.state.analysisRun.error!!.contains("unavailable"))
          releaseStatus.countDown()
          statusThread.join(5_000)
          assertFalse(statusThread.isAlive)
          h.main.runPending() // The older dismissal read must not replace recovery.
          h.await { h.state.analysisRun.run?.identity == h.run.identity }
          assertTrue(h.state.analysisRun.error!!.contains("unavailable"))
          assertFalse(h.state.analysisRun.error!!.contains("replaced"))
          assertNull(h.state.analysisRun.admission)
          assertNull(h.state.analysisRun.previewIntent)
          assertEquals(
              1,
              h.calls.count {
                it.first == "POST" && it.second.endsWith(if (resume) "/control" else "/run")
              })
        } finally {
          releaseAdmission.countDown()
          releaseStatus.countDown()
          admissionThread.join(5_000)
          if (statusThread.state != Thread.State.NEW) statusThread.join(5_000)
        }
      }
    }
  }

  @Test
  fun uncertainAdmissionDoesNotFollowAnUnrelatedPolledRun() {
    for (resume in listOf(false, true)) {
      Harness(pollMillis = 15).use { h ->
        h.run = h.run.copy(status = if (resume) "paused" else "running")
        h.workflow.refresh()
        h.drain()
        val oldRun = h.run
        h.workflow.preview(resume = resume)
        h.drain()
        h.confirm()
        h.admissionTransportFailure = true
        h.workflow.admit()
        h.run = h.run.copy(status = "running") // The old run remains active during recovery.
        h.drain()
        assertTrue(h.state.analysisRun.error!!.contains("unavailable"))
        h.run =
            h.run.copy(
                identity =
                    h.run.identity.copy(
                        queueId = if (resume) h.run.identity.queueId else "other-queue",
                        id = if (resume) "other-run" else h.run.identity.id,
                        generation = "other-generation"))
        h.await { h.state.analysisRun.error?.contains("replaced") == true }
        assertEquals(oldRun.identity, h.state.analysisRun.run?.identity)
        assertTrue(h.state.analysisRun.error!!.contains("unavailable"))
        assertNull(h.state.analysisRun.previewIntent)
        h.workflow.admit()
        h.drain()
        assertEquals(
            1,
            h.calls.count {
              it.first == "POST" && it.second.endsWith(if (resume) "/control" else "/run")
            })
      }
    }
  }

  @Test
  fun delayedAdmissionCannotReviveDismissedOrReplacedReview() {
    for (change in
        listOf(
            "dismiss", "detach", "project", "revision", "provider", "selection", "replacement")) {
      Harness().use { h ->
        h.workflow.refresh()
        h.drain()
        h.workflow.preview()
        h.drain()
        h.confirm()
        h.admissionConflict = true
        h.workflow.admit()
        h.main.runPending()
        h.io.runPending() // Conflict is queued for delivery.
        when (change) {
          "dismiss" -> h.workflow.dismissAdmission()
          "detach" -> h.workflow.detach()
          "project" ->
              h.dispatch(
                  DesktopEvent.ProjectLoaded(
                      analysisProjectFixture("other"), ProjectIndex("other", "revision")))
          "revision" -> h.dispatch(DesktopEvent.IndexRefreshed(ProjectIndex("project", "new")))
          "provider" -> h.workflow.providerChanged()
          "selection" -> {
            h.dispatch(
                DesktopEvent.AnalysisRunUpdated(
                    h.state.analysisRun.copy(
                        fileSelection = h.state.analysisRun.fileSelection.copy(saving = true))))
          }
          else -> h.workflow.preview(limits = AnalysisRunLimits(4, 80, 1))
        }
        h.drain()
        assertNull(h.state.analysisRun.admission, change)
        assertEquals(change == "replacement", h.state.analysisRun.previewIntent != null, change)
        assertEquals(1, h.calls.count { it.first == "POST" && it.second.endsWith("/run") }, change)
        h.workflow.retryPreview()
        h.drain()
        assertEquals(
            if (change == "replacement") 2 else 1,
            h.calls.count { it.second.endsWith("/preview") },
            change)
      }
    }
  }

  @Test
  fun delayedReconciliationKeepsCurrentReviewAndEvidenceInsteadOfPublishingOldStatus() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      val evidence = h.state.analysisRun.sections
      h.workflow.preview(resume = true)
      h.drain()
      h.confirm()
      h.admissionConflict = true
      h.workflow.admit()
      h.main.runPending()
      h.io.runPending()
      h.main.runPending() // Conflict schedules a read-only status request.
      h.io.runPending()
      val newer = h.run.copy(identity = h.run.identity.copy(generation = "replacement"))
      h.dispatch(DesktopEvent.AnalysisRunUpdated(h.state.analysisRun.copy(run = newer)))
      h.drain()
      assertEquals(newer, h.state.analysisRun.run)
      assertNull(h.state.analysisRun.previewIntent)
      assertNull(h.state.analysisRun.admission)
      assertEquals(evidence, h.state.analysisRun.sections)
      assertEquals(1, h.calls.count { it.first == "POST" && it.second.endsWith("/control") })
    }
  }

  @Test
  fun rejectedResumeReconcilesMatchingPlanButNotReplacementAndPreservesEvidence() {
    for (change in listOf("same", "plan", "run")) {
      Harness().use { h ->
        h.workflow.refresh()
        h.drain()
        val evidence = h.state.analysisRun.sections
        h.workflow.preview(resume = true)
        h.drain()
        h.confirm()
        h.admissionConflict = true
        h.workflow.admit()
        h.main.runPending()
        h.io.runPending()
        h.run =
            when (change) {
              "plan" -> h.run.copy(plan = h.run.plan.copy(refresh = !h.run.plan.refresh))
              "run" -> h.run.copy(identity = h.run.identity.copy(generation = "replacement"))
              else -> h.run
            }
        h.drain()
        assertEquals(h.run, h.state.analysisRun.run, change)
        assertEquals(change == "same", h.state.analysisRun.previewIntent != null, change)
        assertNull(h.state.analysisRun.admission, change)
        if (change != "run") assertEquals(evidence, h.state.analysisRun.sections, change)
        assertEquals(
            1, h.calls.count { it.first == "POST" && it.second.endsWith("/control") }, change)
      }
    }
  }

  @Test
  fun polledReplacementInvalidatesRejectedResumeButMatchingPollKeepsIt() {
    for (change in listOf("plan", "run")) {
      Harness(pollMillis = 15).use { h ->
        h.run = h.run.copy(status = "paused")
        h.workflow.refresh()
        h.drain()
        val evidence = h.state.analysisRun.sections
        h.workflow.preview(resume = true)
        h.drain()
        h.confirm()
        h.admissionConflict = true
        h.workflow.admit()
        h.drain()
        assertNotNull(h.state.analysisRun.previewIntent)
        h.run = h.run.copy(updatedAt = "progress")
        h.dispatch(DesktopEvent.AnalysisRunUpdated(h.state.analysisRun.copy(run = h.run)))
        assertEquals("progress", h.state.analysisRun.run?.updatedAt)
        assertNotNull(h.state.analysisRun.previewIntent)
        assertEquals(evidence.keys, h.state.analysisRun.sections.keys)
        h.run =
            if (change == "plan") h.run.copy(plan = h.run.plan.copy(retryStaleFailed = true))
            else h.run.copy(identity = h.run.identity.copy(generation = "replacement"))
        h.dispatch(DesktopEvent.AnalysisRunUpdated(h.state.analysisRun.copy(run = h.run)))
        assertEquals(h.run, h.state.analysisRun.run)
        assertNull(h.state.analysisRun.previewIntent, change)
        assertNull(h.state.analysisRun.admission, change)
        h.workflow.retryPreview()
        h.drain()
        assertEquals(1, h.calls.count { it.second.endsWith("/preview") }, change)
        assertEquals(
            1, h.calls.count { it.first == "POST" && it.second.endsWith("/control") }, change)
      }
    }
  }

  @Test
  fun lateStatusAfterDismissalOrBlockedPreviewCannotRestoreRejectedIntent() {
    for (change in listOf("dismiss", "new preview")) {
      Harness().use { h ->
        h.workflow.refresh()
        h.drain()
        h.workflow.preview(resume = true)
        h.drain()
        h.confirm()
        h.admissionConflict = true
        h.workflow.admit()
        h.main.runPending()
        h.io.runPending()
        h.main.runPending()
        h.io.runPending() // Old status is queued for delivery.
        if (change == "dismiss") h.workflow.dismissAdmission()
        else h.workflow.preview(limits = AnalysisRunLimits(3, 60, 1))
        h.drain()
        assertEquals(
            change == "new preview", h.state.analysisRun.previewIntent?.resumeRun != null, change)
        assertNull(h.state.analysisRun.admission, change)
        assertEquals(1, h.calls.count { it.second.endsWith("/preview") })
        assertEquals(1, h.calls.count { it.first == "POST" && it.second.endsWith("/control") })
      }
    }
  }

  @Test
  fun delayedStatusDoesNotOverwriteNewerProgressForTheSameRun() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      h.workflow.preview(resume = true)
      h.drain()
      h.confirm()
      h.admissionConflict = true
      h.workflow.admit()
      h.main.runPending()
      h.io.runPending()
      h.main.runPending()
      h.io.runPending()
      val updated = h.run.copy(status = "interrupted", updatedAt = "newer")
      h.dispatch(DesktopEvent.AnalysisRunUpdated(h.state.analysisRun.copy(run = updated)))
      h.drain()
      assertEquals(updated, h.state.analysisRun.run)
      assertNull(h.state.analysisRun.previewIntent) // Paused → interrupted invalidates review.
      assertEquals(1, h.calls.count { it.first == "POST" && it.second.endsWith("/control") })
    }
  }

  @Test
  fun uncertainStartAndResumeShowAdmittedDurableProgressWithoutRetryOrLostError() {
    for (resume in listOf(false, true)) {
      Harness().use { h ->
        h.workflow.refresh()
        h.drain()
        val evidence = h.state.analysisRun.sections
        h.workflow.preview(resume = resume)
        h.drain()
        h.confirm()
        h.failure = if (resume) "/control" else "/analysis/run"
        h.workflow.admit()
        h.workflow.admit()
        h.main.runPending()
        h.io.runPending()
        h.run = h.run.copy(status = "running", updatedAt = "admitted")
        h.drain()
        assertEquals("running", h.state.analysisRun.run?.status)
        assertEquals("admitted", h.state.analysisRun.run?.updatedAt)
        assertNull(h.state.analysisRun.admission)
        assertNull(h.state.analysisRun.previewIntent)
        assertTrue(h.state.analysisRun.error!!.contains("unavailable"))
        assertEquals(evidence.keys, h.state.analysisRun.sections.keys)
        h.workflow.retryPreview()
        h.workflow.admit()
        h.drain()
        assertEquals(
            1,
            h.calls.count {
              it.first == "POST" && it.second.endsWith(if (resume) "/control" else "/run")
            })
      }
    }
  }

  @Test
  fun delayedOverviewFallbackCannotReplaceNewerRunOrItsEvidence() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      h.workflow.preview()
      h.drain()
      h.confirm()
      h.failure = "/analysis/run"
      h.workflow.admit()
      h.main.runPending()
      h.io.runPending() // Uncertain admission.
      h.main.runPending()
      h.io.runPending() // Failed status read.
      h.main.runPending()
      h.io.runPending() // Overview fallback is in flight.
      val newer = h.run.copy(identity = h.run.identity.copy(generation = "replacement"))
      h.dispatch(DesktopEvent.AnalysisRunUpdated(h.state.analysisRun.copy(run = newer)))
      val evidence = h.state.analysisRun.sections
      h.drain()
      assertEquals(newer, h.state.analysisRun.run)
      assertEquals(evidence, h.state.analysisRun.sections)
      assertNull(h.state.analysisRun.previewIntent)
      assertEquals(1, h.calls.count { it.first == "POST" && it.second.endsWith("/run") })
    }
  }

  @Test
  fun canceledAdmissionDoesNotBecomeARejectedOrRetryableRequest() {
    Harness().use { h ->
      h.workflow.preview()
      h.drain()
      h.confirm()
      h.cancelAdmission = true
      h.workflow.admit()
      h.drain()
      assertNull(h.state.analysisRun.previewIntent)
      assertNull(h.state.analysisRun.admission)
      assertNull(h.state.analysisRun.error)
      assertEquals(1, h.calls.count { it.first == "POST" && it.second.endsWith("/run") })
      assertFalse(h.calls.any { it.first == "GET" && it.second.contains("/analysis/run?") })
      h.workflow.retryPreview()
      h.workflow.admit()
      h.drain()
      assertEquals(1, h.calls.count { it.first == "POST" && it.second.endsWith("/run") })
    }
  }

  @Test
  fun statusFailureAfterUncertainAdmissionRetainsTransportErrorAndOverviewProgress() {
    Harness().use { h ->
      h.workflow.preview()
      h.drain()
      h.confirm()
      h.failure = "/analysis/run"
      h.workflow.admit()
      h.main.runPending()
      h.io.runPending()
      h.main.runPending()
      h.run = h.run.copy(status = "running")
      h.drain()
      assertEquals("running", h.state.analysisRun.run?.status)
      assertTrue(h.state.analysisRun.error!!.contains("unavailable"))
      assertTrue(h.state.analysisRun.error!!.contains("status could not be read"))
      assertNull(h.state.analysisRun.previewIntent)
    }
  }

  @Test
  fun resumeUsesFreshPreviewAndExactPriorGenerationAndDoesNotResetRetainedEvidence() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      val evidence = h.state.analysisRun.sections
      h.workflow.preview(resume = true)
      h.drain()
      assertFalse(h.state.analysisRun.admission!!.isConfirmed())
      assertEquals(evidence, h.state.analysisRun.sections)
      val preview = Json.decodeFromString<AnalysisPreviewRequest>(h.bodies.last())
      assertEquals(h.run.identity, preview.resumeRun)
      h.confirm()
      h.workflow.admit()
      h.drain()
      val control = Json.decodeFromString<AnalysisRunControlRequest>(h.bodies.last())
      assertEquals("resume", control.action)
      assertEquals(analysisRunFixture().identity, control.identity)
      assertEquals("new-generation", h.state.analysisRun.run?.identity?.generation)
      assertNull(h.state.analysisRun.admission)
    }
  }

  @Test
  fun previewReplacementProviderChangeDismissalAndRevisionChangeInvalidateConsent() {
    for (change in listOf("preview", "provider", "dismiss", "revision", "project", "close")) {
      Harness().use { h ->
        h.workflow.preview()
        h.drain()
        h.confirm()
        when (change) {
          "preview" -> h.workflow.preview(refresh = true)
          "provider" -> h.workflow.providerChanged()
          "dismiss" -> h.workflow.dismissAdmission()
          "revision" -> h.dispatch(DesktopEvent.IndexRefreshed(ProjectIndex("project", "new")))
          "project" ->
              h.dispatch(
                  DesktopEvent.ProjectLoaded(
                      analysisProjectFixture("other"), ProjectIndex("other", "revision")))
          "close" -> h.workflow.detach()
        }
        h.drain()
        assertTrue(h.state.analysisRun.admission?.isConfirmed() != true, change)
        assertFalse(h.calls.any { it.first == "POST" && it.second.endsWith("/run") }, change)
      }
    }
  }

  @Test
  fun latePreviewAndActionCannotPublishAfterProjectReplacementOrDetachment() {
    for (admit in listOf(false, true)) {
      for (close in listOf(false, true)) {
        Harness().use { h ->
          h.workflow.preview()
          if (admit) {
            h.drain()
            h.confirm()
            h.workflow.admit()
          }
          h.main.runPending()
          h.io.runPending()
          if (close) h.workflow.detach()
          else
              h.dispatch(
                  DesktopEvent.ProjectLoaded(
                      analysisProjectFixture("other"), ProjectIndex("other", "revision")))
          h.drain()
          assertNull(h.state.analysisRun.admission)
          assertNull(h.state.analysisRun.run)
        }
      }
    }
  }

  @Test
  fun explicitStatusRefreshReadsOnlyTheRunWithoutSelectionOrControlRequests() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      h.failStatus = true
      h.workflow.refreshStatus()
      h.drain()
      assertTrue(h.state.analysisRun.statusUnavailable)
      val before = h.calls.size
      h.failStatus = false
      h.workflow.refreshStatus()
      h.drain()
      val refreshCalls = h.calls.drop(before)
      assertEquals(
          1, refreshCalls.count { it.first == "GET" && it.second.contains("/analysis/run?") })
      assertTrue(refreshCalls.all { it.first == "GET" && !it.second.contains("/selection?") })
      assertFalse(h.state.analysisRun.statusUnavailable)
      assertNull(h.state.analysisRun.error)
    }
  }

  @Test
  fun selectionRefreshAndSaveUseConfirmedEvidenceWithoutAdmittingAnalysis() {
    Harness().use { h ->
      h.run = h.run.copy(status = "completed")
      h.selection = selectionFixture()
      h.workflow.refresh()
      h.drain()
      assertEquals(h.selection, h.state.analysisRun.fileSelection.selection)
      val coverage = analysisSelectionCoverage(h.selection)
      assertEquals(AnalysisCoverage(total = 2, fresh = 1, missing = 1), coverage)
      val readCount =
          h.calls.count { it.first == "GET" && it.second.contains("/analysis/selection?") }
      h.workflow.fileSelection.refresh()
      h.drain()
      assertEquals(
          readCount + 1,
          h.calls.count { it.first == "GET" && it.second.contains("/analysis/selection?") })
      assertEquals(
          coverage,
          analysisSelectionCoverage(requireNotNull(h.state.analysisRun.fileSelection.selection)))
      h.workflow.preview()
      h.drain()
      assertNotNull(h.state.analysisRun.admission)
      val beforeSave = h.calls.size
      h.workflow.fileSelection.save(listOf("main.go"))
      h.drain()
      assertNull(h.state.analysisRun.admission)
      assertEquals(
          listOf("POST" to "/api/projects/current/analysis/selection"), h.calls.drop(beforeSave))
      assertEquals(listOf("main.go"), h.selection.excludedPaths)
      assertEquals(
          AnalysisCoverage(total = 1, fresh = 1),
          analysisSelectionCoverage(requireNotNull(h.state.analysisRun.fileSelection.selection)))
      assertEquals(0, h.calls.count { it.first == "POST" && it.second.endsWith("/run") })
      assertEquals(1, h.calls.count { it.first == "POST" && it.second.endsWith("/preview") })
    }
  }

  @Test
  fun fileAndPageSelectionDoNotCancelOrChangeProjectAdmission() {
    Harness().use { h ->
      h.workflow.preview()
      h.drain()
      h.confirm()
      val admission = h.state.analysisRun.admission
      h.dispatch(DesktopEvent.FileLoaded(analysisFileFixture("another.go"), emptyList()))
      Workspace.entries.forEach { h.dispatch(DesktopEvent.WorkspaceSelected(it)) }
      h.drain()
      assertEquals(admission, h.state.analysisRun.admission)
      assertEquals(1, h.calls.size)
      h.workflow.admit()
      h.drain()
      assertNotNull(h.state.analysisRun.run)
    }
  }

  @Test
  fun failedSectionAndFilteredReadsRetainOtherTypedEvidenceAndNeverDispatchModels() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      val security = h.state.analysisRun.sections.getValue(AnalysisResultKey("security")).results
      assertEquals("ai", security!!.security.single().source)
      assertEquals("unverified", security.security.single().findings.single().verificationState)
      val performance =
          h.state.analysisRun.sections.getValue(AnalysisResultKey("performance")).results
      h.failure = "category=security"
      h.workflow.loadResults("security")
      h.drain()
      val failed = h.state.analysisRun.sections.getValue(AnalysisResultKey("security"))
      assertEquals(security, failed.results)
      assertTrue(failed.error!!.contains("unavailable"))
      assertEquals(
          performance,
          h.state.analysisRun.sections.getValue(AnalysisResultKey("performance")).results)
      h.workflow.loadResults("security", "main.go")
      h.drain()
      assertNotNull(
          h.state.analysisRun.sections.getValue(AnalysisResultKey("security", "main.go")).error)
      assertEquals(
          security, h.state.analysisRun.sections.getValue(AnalysisResultKey("security")).results)
      assertTrue(h.calls.all { it.first == "GET" })
    }
  }

  @Test
  fun explicitResultRetryReadsOnlyTheFailedCategoryAndRejectsDuplicateActivation() {
    AnalysisResultType.entries.forEach { type ->
      Harness().use { h ->
        h.workflow.refresh()
        h.drain()
        val key = AnalysisResultKey(type.category)
        val retained = h.state.analysisRun.sections.getValue(key).results
        h.failure = "category=${type.category}"
        h.workflow.retryResults(type.category)
        h.drain()
        assertEquals(retained, h.state.analysisRun.sections.getValue(key).results)
        assertNotNull(h.state.analysisRun.sections.getValue(key).error)
        h.failure = ""
        val before = h.calls.size
        h.workflow.retryResults(type.category)
        assertTrue(h.state.analysisRun.sections.getValue(key).loading)
        h.workflow.retryResults(type.category)
        h.drain()
        assertEquals(1, h.calls.drop(before).size)
        assertEquals("GET", h.calls.last().first)
        assertTrue(h.calls.last().second.endsWith("&category=${type.category}"))
        assertEquals(null, h.state.analysisRun.sections.getValue(key).error)
        assertEquals(retained, h.state.analysisRun.sections.getValue(key).results)
        assertTrue(h.calls.all { it.first == "GET" })
      }
    }
  }

  @Test
  fun fileScopedRetryAndLatePredecessorCannotReplaceNewRun() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      val path = "main.go"
      h.failure = "category=security"
      h.workflow.retryResults("security", path)
      h.drain()
      val key = AnalysisResultKey("security", path)
      assertNotNull(h.state.analysisRun.sections.getValue(key).error)
      h.failure = ""
      val before = h.calls.size
      h.workflow.retryResults("security", path)
      h.main.runPending()
      h.io.runPending()
      h.run = h.run.copy(identity = h.run.identity.copy(generation = "replacement"))
      h.workflow.refresh()
      h.drain()
      assertEquals(
          1,
          h.calls.drop(before).count {
            it.first == "GET" &&
                it.second.contains("/analysis/results?") &&
                it.second.endsWith("&category=security&path=main.go")
          })
      assertTrue(h.state.analysisRun.sections.values.all { it.results?.identity == h.run.identity })
      assertTrue(h.calls.all { it.first == "GET" })
    }
  }

  @Test
  fun resultScopeMismatchCannotEraseValidEvidenceAndOldGenerationCannotPublish() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      val original = h.state.analysisRun.sections.getValue(AnalysisResultKey("security")).results
      h.wrongResult = true
      h.workflow.loadResults("security")
      h.drain()
      assertEquals(
          original, h.state.analysisRun.sections.getValue(AnalysisResultKey("security")).results)
      assertNotNull(h.state.analysisRun.sections.getValue(AnalysisResultKey("security")).error)
      h.wrongResult = false
      h.workflow.loadResults("security")
      h.main.runPending()
      h.io.runPending()
      h.run = h.run.copy(identity = h.run.identity.copy(generation = "replacement"))
      h.workflow.refresh()
      h.drain()
      assertTrue(h.state.analysisRun.sections.values.all { it.results?.identity == h.run.identity })
    }
  }

  @Test
  fun controlsRequireCurrentEligibleRunAndDoNotDuplicatePendingRequests() {
    for (status in
        listOf(
            "queued",
            "running",
            "pausing",
            "paused",
            "interrupted",
            "canceling",
            "canceled",
            "completed",
            "stale",
            "unknown")) {
      Harness().use { h ->
        h.run = h.run.copy(status = status)
        h.workflow.refresh()
        h.drain()
        h.calls.clear()
        h.bodies.clear()
        for (action in listOf("pause", "cancel")) {
          h.workflow.control(action)
          h.workflow.control(action)
          assertEquals(
              action in
                  if (status in setOf("queued", "running")) setOf("pause", "cancel")
                  else if (status in setOf("pausing", "paused", "interrupted")) setOf("cancel")
                  else emptySet(),
              h.state.analysisRun.action == action,
              "$status $action")
          h.drain()
          val requests =
              h.bodies.mapNotNull {
                runCatching { Json.decodeFromString<AnalysisRunControlRequest>(it) }.getOrNull()
              }
          assertEquals(
              if (action == "pause" && status in setOf("queued", "running") ||
                  action == "cancel" &&
                      status in setOf("queued", "running", "pausing", "paused", "interrupted"))
                  1
              else 0,
              requests.size,
              "$status $action")
          if (requests.isNotEmpty()) {
            assertEquals(h.run.identity, requests.single().identity)
            break
          }
        }
      }
    }
    Harness().use { h ->
      h.workflow.control("cancel") // No saved run.
      h.run = h.run.copy(status = "running")
      h.workflow.refresh()
      h.drain()
      h.calls.clear()
      h.dispatch(
          DesktopEvent.AnalysisRunUpdated(
              h.state.analysisRun.copy(
                  run = h.run.copy(identity = h.run.identity.copy(projectId = "other")))))
      h.workflow.control("pause")
      h.workflow.control("cancel")
      h.drain()
      assertTrue(h.calls.none { it.first == "POST" })
    }
  }

  @Test
  fun queuedControlRechecksStatusAndFullRunIdentityBeforeTransport() {
    for (change in listOf("status", "run", "revision", "project")) {
      Harness(pollMillis = 1).use { h ->
        h.run = h.run.copy(status = "running")
        h.workflow.refresh()
        h.drain()
        h.calls.clear()
        h.workflow.control("pause")
        when (change) {
          "status" ->
              h.dispatch(
                  DesktopEvent.AnalysisRunUpdated(
                      h.state.analysisRun.copy(run = h.run.copy(status = "canceling"))))
          "run" -> {
            h.run = h.run.copy(identity = h.run.identity.copy(generation = "other"))
            h.dispatch(DesktopEvent.AnalysisRunUpdated(h.state.analysisRun.copy(run = h.run)))
          }
          "revision" -> h.dispatch(DesktopEvent.IndexRefreshed(ProjectIndex("project", "new")))
          else ->
              h.dispatch(
                  DesktopEvent.ProjectLoaded(
                      analysisProjectFixture("other"), ProjectIndex("other", "revision")))
        }
        h.drain()
        assertTrue(h.calls.none { it.first == "POST" }, change)
        if (change == "status" || change == "run") {
          assertEquals("", h.state.analysisRun.action)
          assertEquals(
              if (change == "status") "canceling" else "running", h.state.analysisRun.run?.status)
          h.run = h.run.copy(status = "canceled")
          val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
          while (h.state.analysisRun.run?.status != "canceled" && System.nanoTime() < deadline) {
            h.main.runPending()
            h.io.runPending()
            Thread.yield()
          }
          assertEquals("canceled", h.state.analysisRun.run?.status)
          assertEquals(h.run.identity, h.state.analysisRun.run?.identity)
          assertTrue(h.calls.none { it.first == "POST" })
        }
      }
    }
  }

  @Test
  fun queuedControlClearsPendingStateWhenRunDisappearsOrBelongsToAnotherProject() {
    for (action in listOf("pause", "cancel")) {
      for (replacement in listOf("absent", "foreign")) {
        Harness().use { h ->
          h.run = h.run.copy(status = "running")
          h.workflow.refresh()
          h.drain()
          h.calls.clear()
          h.workflow.control(action)
          assertEquals(action, h.state.analysisRun.action)
          val run =
              if (replacement == "absent") null
              else h.run.copy(identity = h.run.identity.copy(projectId = "other"))
          h.dispatch(DesktopEvent.AnalysisRunUpdated(h.state.analysisRun.copy(run = run)))
          h.drain()
          assertEquals(run, h.state.analysisRun.run, "$action $replacement")
          assertEquals("", h.state.analysisRun.action, "$action $replacement")
          assertTrue(h.calls.none { it.first == "POST" }, "$action $replacement")
        }
      }
    }
  }

  @Test
  fun pendingAdmissionCannotBeReplacedByPreviewOrControlEvenAfterDismissal() {
    for (resume in listOf(false, true)) {
      Harness().use { h ->
        h.run = h.run.copy(status = if (resume) "paused" else "running")
        h.workflow.refresh()
        h.drain()
        h.workflow.preview(resume = resume)
        h.drain()
        h.confirm()
        h.workflow.admit()
        h.main.runPending()
        h.io.runPending() // Admission response is waiting for the UI dispatcher.
        h.workflow.preview()
        h.workflow.control("cancel")
        h.workflow.admit()
        assertEquals(if (resume) "resume" else "start", h.state.analysisRun.action)
        h.workflow.dismissAdmission()
        h.workflow.preview()
        h.workflow.control("cancel")
        h.drain()
        assertEquals(
            1,
            h.calls.count {
              it.first == "POST" && it.second.endsWith(if (resume) "/control" else "/run")
            })
        assertEquals(1, h.calls.count { it.second.endsWith("/preview") })
        assertEquals(
            if (resume) "new-generation" else "generation",
            h.state.analysisRun.run?.identity?.generation)
      }
    }
  }

  @Test
  fun lateDispatchedPauseCannotReplaceCancellationOrItsFailure() {
    for (failure in listOf(false, true)) {
      Harness().use { h ->
        h.run = h.run.copy(status = "running")
        h.workflow.refresh()
        h.drain()
        val entered = CountDownLatch(1)
        val release = CountDownLatch(1)
        h.lateControlGate = entered to release
        h.lateControlFailure = failure
        h.workflow.control("pause")
        h.main.runPending()
        val transport = Thread { h.io.runPending() }
        transport.start()
        try {
          assertTrue(entered.await(5, TimeUnit.SECONDS))
          h.workflow.control("cancel")
          h.workflow.control("pause") // Cancellation has priority.
          h.workflow.control("cancel")
          h.drain()
          assertEquals("canceled", h.state.analysisRun.run?.status)
          release.countDown()
          transport.join(5_000)
          assertFalse(transport.isAlive)
          h.drain()
          assertEquals("canceled", h.state.analysisRun.run?.status)
          assertNull(h.state.analysisRun.error)
          assertEquals(
              listOf("pause", "cancel"),
              h.bodies.mapNotNull {
                runCatching { Json.decodeFromString<AnalysisRunControlRequest>(it).action }
                    .getOrNull()
              })
        } finally {
          release.countDown()
          transport.join(5_000)
        }
      }
    }
  }

  @Test
  fun canceledPollingSuccessOrFailureCannotUndoAcceptedControl() {
    for (failure in listOf(false, true)) {
      Harness(pollMillis = 1).use { h ->
        h.run = h.run.copy(status = "running")
        h.workflow.refresh()
        h.drain()
        val entered = CountDownLatch(1)
        val release = CountDownLatch(1)
        h.lateStatusGate = entered to release
        h.lateStatusFailure = failure
        val transport = Thread {
          while (entered.count > 0) {
            h.io.runPending()
            Thread.yield()
          }
          h.io.runPending()
        }
        transport.start()
        try {
          val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
          while (entered.count > 0 && System.nanoTime() < deadline) {
            h.main.runPending()
            Thread.yield()
          }
          assertEquals(0L, entered.count, "Polling did not reach the gated read")
          h.workflow.control("cancel")
          h.drain()
          assertEquals("canceled", h.state.analysisRun.run?.status)
          release.countDown()
          transport.join(5_000)
          assertFalse(transport.isAlive)
          h.drain()
          assertEquals("canceled", h.state.analysisRun.run?.status)
          assertNull(h.state.analysisRun.error)
          assertEquals(1, h.calls.count { it.first == "POST" && it.second.endsWith("/control") })
        } finally {
          release.countDown()
          transport.join(5_000)
        }
      }
    }
  }

  @Test
  fun acceptedControlReportsActualSettlementIncludingNaturalCompletion() {
    for (status in listOf("pausing", "paused", "completed", "canceling", "canceled")) {
      Harness().use { h ->
        h.run = h.run.copy(status = "running")
        h.workflow.refresh()
        h.drain()
        h.controlSettledStatus = status
        h.workflow.control(if (status in setOf("canceling", "canceled")) "cancel" else "pause")
        h.drain()
        assertEquals(status, h.state.analysisRun.run?.status)
        assertEquals("", h.state.analysisRun.action)
      }
    }
  }

  @Test
  fun queuedOlderControlCannotReachTheDaemonAfterNewerControl() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      h.workflow.control("pause")
      h.main.runPending()
      h.workflow.control("cancel")
      h.drain()
      val controls =
          h.bodies.mapNotNull {
            runCatching { Json.decodeFromString<AnalysisRunControlRequest>(it) }.getOrNull()
          }
      assertEquals(listOf("cancel"), controls.map { it.action })
      assertEquals("canceled", h.state.analysisRun.run?.status)
    }
  }

  @Test
  fun pausingAndCancelingKeepOnePollUntilTerminalAndSectionFailuresDoNotStopIt() {
    for (status in listOf("queued", "running", "pausing", "canceling")) {
      Harness(pollMillis = 1).use { h ->
        h.run = h.run.copy(status = status)
        h.failure = "category=performance"
        h.workflow.refresh()
        h.drain()
        assertEquals(status, h.state.analysisRun.run?.status)
        h.run = h.run.copy(status = "paused")
        h.await { h.state.analysisRun.run?.status == "paused" }
        val reads = h.calls.count { it.second.contains("/analysis/run?") }
        Thread.sleep(10)
        h.drain()
        assertEquals(reads, h.calls.count { it.second.contains("/analysis/run?") })
        assertNotNull(h.state.analysisRun.sections.getValue(AnalysisResultKey("performance")).error)
      }
    }
  }

  @Test
  fun rejectedAndUnconfirmedControlsReconcileWithoutRepeatingThePost() {
    for (rejected in listOf(true, false)) {
      Harness().use { h ->
        h.run = h.run.copy(status = "running")
        h.workflow.refresh()
        h.drain()
        val evidence = h.state.analysisRun.sections
        if (rejected) h.controlConflict = true else h.failure = "/control"
        h.workflow.control("pause")
        assertEquals(
            AnalysisControlRequest("pause", AnalysisControlOutcome.Requesting),
            h.state.analysisRun.controlRequest)
        h.main.runPending()
        h.io.runPending()
        h.main.runPending() // The rejection or transport failure schedules reconciliation.
        assertEquals(
            AnalysisControlRequest(
                "pause",
                if (rejected) AnalysisControlOutcome.Reconciling
                else AnalysisControlOutcome.Unconfirmed),
            h.state.analysisRun.controlRequest)
        assertTrue(projectRunPresentation(h.state.project, h.state.analysisRun).commands.isEmpty())
        h.workflow.preview(resume = true)
        h.workflow.control("cancel")
        h.drain()
        assertEquals("running", h.state.analysisRun.run?.status)
        assertEquals(evidence, h.state.analysisRun.sections)
        assertEquals(null, h.state.analysisRun.controlRequest)
        assertFalse(h.state.analysisRun.statusUnavailable)
        assertTrue(
            h.state.analysisRun.error!!.contains(
                if (rejected) "pause rejected" else "pause outcome unconfirmed"))
        assertEquals(AnalysisRunErrorKind.Control, h.state.analysisRun.errorKind)
        assertFalse(h.state.analysisRun.showsAdmissionOverlay())
        h.workflow.dismissAdmission() // Dismissing a run error is not an admission action.
        h.drain()
        assertNotNull(h.state.analysisRun.error)
        assertEquals(1, h.calls.count { it.first == "POST" && it.second.endsWith("/control") })
        assertTrue(h.calls.any { it.first == "GET" && it.second.contains("/analysis/run?") })
      }
    }
  }

  @Test
  fun cancellationTransportFailureStaysOnRunSurfaceWithoutAdmissionRetry() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      h.failure = "/control"
      h.workflow.control("cancel")
      h.drain()
      assertEquals(AnalysisRunErrorKind.Control, h.state.analysisRun.errorKind)
      assertTrue(h.state.analysisRun.error!!.contains("cancel outcome unconfirmed"))
      assertFalse(h.state.analysisRun.showsAdmissionOverlay())
      assertNull(h.state.analysisRun.admission)
      h.workflow.dismissAdmission()
      h.drain()
      assertEquals(1, h.calls.count { it.first == "POST" && it.second.endsWith("/control") })
      assertTrue(h.calls.none { it.second.endsWith("/preview") })
    }
  }

  @Test
  fun failedControlReconciliationRetainsEvidenceAndBlocksOldAuthorityUntilExplicitRead() {
    for (rejected in listOf(true, false)) {
      Harness().use { h ->
        h.workflow.refresh()
        h.drain()
        val accepted = h.state.analysisRun.run
        val evidence = h.state.analysisRun.sections
        if (rejected) h.controlConflict = true else h.failure = "/control"
        h.failure = if (rejected) "" else "/control"
        h.failStatus = true
        h.run = h.run.copy(status = "interrupted", reason = "Save failed after stage 1")
        h.workflow.control("cancel")
        h.drain()
        assertEquals(h.run, h.state.analysisRun.run) // Overview retains daemon progress.
        assertEquals(evidence.keys, h.state.analysisRun.sections.keys)
        assertTrue(h.state.analysisRun.sections.values.all { it.results != null })
        assertTrue(h.state.analysisRun.statusUnavailable)
        assertTrue(projectRunPresentation(h.state.project, h.state.analysisRun).commands.isEmpty())
        assertEquals(
            if (rejected) null
            else AnalysisControlRequest("cancel", AnalysisControlOutcome.Unconfirmed),
            h.state.analysisRun.controlRequest)
        assertTrue(h.state.analysisRun.error!!.contains("status could not be read"))
        assertEquals(AnalysisRunErrorKind.StatusRead, h.state.analysisRun.errorKind)
        assertFalse(h.state.analysisRun.showsAdmissionOverlay())
        h.workflow.preview(resume = true)
        h.workflow.preview()
        h.workflow.control("cancel")
        h.drain()
        assertEquals(0, h.calls.count { it.second.endsWith("/preview") })
        assertEquals(1, h.calls.count { it.first == "POST" && it.second.endsWith("/control") })
        h.failStatus = false
        h.failure = ""
        h.workflow.refresh()
        h.drain()
        assertFalse(h.state.analysisRun.statusUnavailable)
        assertTrue(
            projectRunPresentation(h.state.project, h.state.analysisRun).commands.isNotEmpty())
        assertNull(h.state.analysisRun.controlRequest)
        assertNull(h.state.analysisRun.error)
        assertEquals(h.run, h.state.analysisRun.run)
        assertTrue(h.calls.none { it.first == "POST" && it.second.endsWith("/run") })
        assertTrue(accepted != h.run)
      }
    }
  }

  @Test
  fun failedStatusAndOverviewKeepTheOldPausedSnapshotButNotItsResumeAuthority() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      val accepted = h.state.analysisRun.run
      val evidence = h.state.analysisRun.sections
      h.failure = "/overview?"
      h.failStatus = true
      h.controlConflict = true
      h.workflow.control("cancel")
      h.drain()
      assertEquals(accepted, h.state.analysisRun.run)
      assertEquals(evidence, h.state.analysisRun.sections)
      assertTrue(h.state.analysisRun.statusUnavailable)
      assertTrue(projectRunPresentation(h.state.project, h.state.analysisRun).commands.isEmpty())
      h.workflow.preview(resume = true)
      h.workflow.preview()
      h.drain()
      assertEquals(0, h.calls.count { it.second.endsWith("/preview") })
      assertEquals(1, h.calls.count { it.first == "POST" && it.second.endsWith("/control") })
      h.failure = ""
      h.failStatus = false
      h.run = h.run.copy(status = "canceled")
      h.workflow.refresh()
      h.drain()
      assertEquals("canceled", h.state.analysisRun.run?.status)
      assertFalse(h.state.analysisRun.statusUnavailable)
      assertNull(h.state.analysisRun.error)
      assertFalse(
          projectRunPresentation(h.state.project, h.state.analysisRun)
              .commands
              .contains(AnalysisRunCommand.Resume))
    }
  }

  @Test
  fun lateControlFallbackCannotReplaceNewerAcceptedProgress() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      h.failStatus = true
      h.failure = "/control"
      h.workflow.control("cancel")
      h.main.runPending()
      h.io.runPending() // Control failure.
      h.main.runPending() // Status read begins.
      h.io.runPending() // Failed status response awaits the UI dispatcher.
      h.main.runPending() // Overview fallback begins.
      h.io.runPending() // Retained overview response awaits the UI dispatcher.
      val newer = h.run.copy(status = "interrupted", reason = "Save failed", updatedAt = "newer")
      h.dispatch(DesktopEvent.AnalysisRunUpdated(h.state.analysisRun.copy(run = newer)))
      val evidence = h.state.analysisRun.sections
      h.drain()
      assertEquals(newer, h.state.analysisRun.run)
      assertEquals(evidence, h.state.analysisRun.sections)
      assertTrue(h.state.analysisRun.statusUnavailable)
      assertEquals(1, h.calls.count { it.first == "POST" && it.second.endsWith("/control") })
    }
  }

  @Test
  fun lateFailedReconciliationCannotOverwriteExplicitRefresh() {
    Harness().use { h ->
      h.run = h.run.copy(status = "running")
      h.workflow.refresh()
      h.drain()
      h.failure = "/control"
      h.failStatus = true
      h.workflow.control("pause")
      h.main.runPending()
      h.io.runPending()
      h.main.runPending() // Reconciliation read queued.
      h.io.runPending() // Its failure is queued for UI delivery.
      // A newer project revision invalidates the old operation and its fallback.
      h.dispatch(DesktopEvent.IndexRefreshed(ProjectIndex("project", "new")))
      h.drain()
      assertEquals("new", h.state.project?.projectRevision)
      assertNull(h.state.analysisRun.controlRequest)
      assertFalse(h.state.analysisRun.statusUnavailable)
      assertEquals(1, h.calls.count { it.first == "POST" && it.second.endsWith("/control") })
    }
  }

  @Test
  fun statusReadErrorKindSurvivesAnOrdinaryExceptionMessageAndClearsOnNextRead() {
    Harness().use { h ->
      h.run = h.run.copy(status = "completed")
      h.workflow.refresh()
      h.drain()
      val accepted = h.state.analysisRun.run
      h.failure = "/analysis/run?"
      h.workflow.refresh()
      h.drain()
      assertEquals(accepted, h.state.analysisRun.run)
      assertEquals("unavailable", h.state.analysisRun.error)
      assertEquals(AnalysisRunErrorKind.StatusRead, h.state.analysisRun.errorKind)
      assertFalse(h.state.analysisRun.showsAdmissionOverlay())
      h.workflow.dismissAdmission()
      assertEquals("unavailable", h.state.analysisRun.error)
      val header = toolbarAnalysisStatus(h.state)!!
      assertEquals("Analysis · Status unavailable", header.label)
      assertTrue(header.attention)
      assertFalse(header.running)
      assertTrue(h.calls.all { it.first == "GET" })
      h.failure = ""
      h.workflow.refresh()
      h.drain()
      assertNull(h.state.analysisRun.error)
      assertNull(h.state.analysisRun.errorKind)
      assertEquals("Analysis · Completed", toolbarAnalysisStatus(h.state)?.label)
    }
  }

  @Test
  fun readFailureCanRecoverRetainedOverviewAndReconnectNeverAdmitsWork() {
    Harness().use { h ->
      h.failure = "/analysis/run?"
      h.workflow.refresh()
      h.drain()
      assertEquals(h.run, h.state.analysisRun.run)
      assertNotNull(h.state.analysisRun.error)
      assertNull(h.state.analysisRun.admission)
      assertTrue(h.calls.all { it.first == "GET" })
      h.failure = ""
      h.run = h.run.copy(status = "interrupted")
      h.workflow.refresh()
      h.drain()
      assertEquals("interrupted", h.state.analysisRun.run?.status)
      assertNull(h.state.analysisRun.admission)
    }
  }

  @Test
  fun acceptedTriageUpdatesUnifiedEvidenceAndRejectsAReadStartedBeforeTheWrite() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      h.workflow.loadResults("bugs")
      h.main.runPending()
      h.io.runPending()
      h.dispatch(DesktopEvent.FindingStatusUpdated("bug", "dismissed"))
      h.drain()
      val section = h.state.analysisRun.sections.getValue(AnalysisResultKey("bugs"))
      assertEquals("dismissed", section.results!!.semantic.single().status)
      assertFalse(section.loading)
    }
  }

  @Test
  fun finalProgressReloadsAReportReadThatWasStillInFlightAtCompletion() {
    Harness().use { h ->
      h.run = h.run.copy(status = "running")
      h.workflow.refresh()
      h.drain()
      h.workflow.loadResults("bugs")
      h.main.runPending()
      h.io.runPending()
      h.run = h.run.copy(status = "completed", updatedAt = "finished")
      h.dispatch(DesktopEvent.AnalysisRunUpdated(h.state.analysisRun.copy(run = h.run)))
      h.drain()
      val section = h.state.analysisRun.sections.getValue(AnalysisResultKey("bugs"))
      assertEquals("completed", section.results!!.semantic.single().message)
      assertFalse(section.loading)
    }
  }

  @Test
  fun refreshedProgressPublishesAllCategoryCountsForSummaryAndAnalysisTogether() {
    Harness().use { h ->
      fun withCounts(status: String, first: Int) =
          h.run.copy(
              status = status,
              sections =
                  AnalysisResultType.entries.mapIndexed { index, type ->
                    AnalysisSectionProgress(
                        type.category,
                        status,
                        AnalysisRunCoverage(running = if (status == "running") 1 else 0),
                        first + index)
                  })

      h.run = withCounts("running", 11)
      h.workflow.refresh()
      h.drain()
      assertEquals(
          listOf(11, 12, 13),
          summaryIssueMetrics(
                  h.state.project, h.state.analysisRun.run, h.state.analysisRun.sections)
              .map { it.value })
      assertEquals(
          AnalysisResultType.entries.map { it.category }.toSet(),
          h.state.analysisRun.sections.keys
              .filter { it.path.isEmpty() }
              .map { it.category }
              .toSet())

      h.run = withCounts("completed", 21)
      h.workflow.refresh()
      h.drain()
      assertEquals(
          listOf(21, 22, 23),
          summaryIssueMetrics(
                  h.state.project, h.state.analysisRun.run, h.state.analysisRun.sections)
              .map { it.value })
    }
  }

  @Test
  fun historyRetainsOnlyObservedTerminalReplacementsAndNotPollingOrContinuation() {
    Harness().use { h ->
      h.run = h.run.copy(status = "running")
      h.workflow.refresh()
      h.drain()
      assertNull(h.state.analysisRun.previousRun)
      h.run = h.run.copy(identity = h.run.identity.copy(id = "unobserved-replacement"))
      h.workflow.refresh()
      h.drain()
      assertNull(h.state.analysisRun.previousRun)

      h.run = h.run.copy(status = "paused")
      h.workflow.refresh()
      h.drain()
      h.workflow.preview(resume = true)
      h.drain()
      h.confirm()
      h.workflow.admit()
      h.drain()
      assertEquals("new-generation", h.state.analysisRun.run?.identity?.generation)
      assertNull(h.state.analysisRun.previousRun)

      h.run = h.run.copy(status = "partial", reason = "First observed failure")
      h.workflow.refresh()
      h.drain()
      h.run = h.run.copy(updatedAt = "final observation", elapsedSeconds = 42)
      h.workflow.refresh() // Same identity, new snapshot, no history.
      h.drain()
      val first = h.state.analysisRun.run!!
      assertNull(h.state.analysisRun.previousRun)
      h.run = h.run.copy(identity = h.run.identity.copy(generation = "second"), status = "failed")
      h.workflow.refresh()
      h.drain()
      assertEquals(first, h.state.analysisRun.previousRun)
      assertTrue(h.state.analysisRun.sections.values.all { it.results?.identity == h.run.identity })
      val second = h.state.analysisRun.run!!
      h.run = h.run.copy(identity = h.run.identity.copy(id = "third"), status = "canceled")
      h.workflow.refresh()
      h.drain()
      assertEquals(second, h.state.analysisRun.previousRun)
      assertEquals(h.run, h.state.analysisRun.run)
      h.dispatch(DesktopEvent.WorkspaceSelected(Workspace.Bugs))
      h.dispatch(DesktopEvent.WorkspaceSelected(Workspace.Analysis))
      assertEquals(second, h.state.analysisRun.previousRun)
    }
  }

  @Test
  fun onlyObservedTerminalStatusesBecomeHistory() {
    for (status in
        listOf(
            "queued",
            "running",
            "pausing",
            "canceling",
            "paused",
            "interrupted",
            "stale",
            "completed",
            "completed_empty",
            "partial",
            "failed",
            "unavailable",
            "canceled")) {
      Harness().use { h ->
        h.run = h.run.copy(status = status)
        h.workflow.refresh()
        h.drain()
        val observed = h.state.analysisRun.run!!
        h.run = h.run.copy(identity = h.run.identity.copy(id = "next"), status = "running")
        h.workflow.refresh()
        h.drain()
        assertEquals(
            if (status in
                setOf(
                    "completed", "completed_empty", "partial", "failed", "unavailable", "canceled"))
                observed
            else null,
            h.state.analysisRun.previousRun,
            status)
      }
    }
  }

  @Test
  fun queuedOldStatusCannotReplaceCurrentRunOrHistory() {
    Harness().use { h ->
      h.run = h.run.copy(status = "completed")
      h.workflow.refresh()
      h.drain()
      val first = h.state.analysisRun.run!!
      h.run =
          h.run.copy(identity = h.run.identity.copy(generation = "replacement"), status = "partial")
      h.workflow.refresh()
      h.drain()
      val accepted = h.state.analysisRun.run!!
      assertEquals(first, h.state.analysisRun.previousRun)
      h.staleStatus = first
      h.staleStatusReads = 1
      h.workflow.refresh()
      h.main.runPending()
      h.io.runPending() // Old terminal status is queued on the UI dispatcher.
      val newer = accepted.copy(updatedAt = "newer progress")
      h.dispatch(DesktopEvent.AnalysisRunUpdated(h.state.analysisRun.copy(run = newer)))
      h.drain()
      assertEquals(newer, h.state.analysisRun.run)
      assertEquals(first, h.state.analysisRun.previousRun)
      assertTrue(
          h.state.analysisRun.sections.values.all { it.results?.identity == accepted.identity })
    }
  }

  @Test
  fun budgetLimitedContinuationKeepsCapturedPolicyAccountingAndOneHistorySnapshot() {
    Harness().use { h ->
      val prior = h.run.copy(status = "completed", reason = "Prior run")
      h.run = prior
      h.workflow.refresh()
      h.drain()
      h.run =
          h.run.copy(
              identity = h.run.identity.copy(id = "budget-run", generation = "budget-generation"),
              status = "paused",
              reason = "Dispatch window exhausted",
              elapsedSeconds = 92,
              plan =
                  h.run.plan.copy(
                      identity = h.run.identity.queue(),
                      limits = AnalysisRunLimits(4, 70, 3),
                      refresh = false,
                      retryStaleFailed = true,
                      files =
                          listOf(
                              AnalysisPlannedFile(
                                  "main.go",
                                  "base",
                                  "Go",
                                  20,
                                  listOf(
                                      AnalysisStagePlan(
                                          "semantic", true, false, maxModelRequests = 0))))),
              files =
                  listOf(
                      AnalysisRunFile(
                          "main.go",
                          "base",
                          "Go",
                          listOf(
                              AnalysisStageProgress(
                                  "semantic", "completed", attempts = 2, cached = false)))))
      h.workflow.refresh()
      h.drain()
      assertEquals(prior, h.state.analysisRun.previousRun)
      val paused = h.state.analysisRun.run!!
      h.calls.clear()
      h.bodies.clear()
      h.workflow.preview(resume = true)
      h.drain()
      val preview = Json.decodeFromString<AnalysisPreviewRequest>(h.bodies.single())
      assertEquals(paused.identity, preview.resumeRun)
      assertEquals(paused.plan.limits, preview.limits)
      assertEquals(paused.plan.refresh, preview.refresh)
      assertEquals(paused.plan.retryStaleFailed, preview.retryStaleFailed)
      h.confirm()
      h.workflow.admit()
      h.drain()
      val control = Json.decodeFromString<AnalysisRunControlRequest>(h.bodies.last())
      assertEquals(paused.identity, control.identity)
      assertTrue(control.confirmations!!.securityReview)
      assertEquals(
          setOf("bug-provider", "analyze-provider"), control.confirmations.providerIds.toSet())
      assertEquals(paused.plan, h.state.analysisRun.run?.plan)
      assertEquals(paused.files, h.state.analysisRun.run?.files)
      assertEquals(92, h.state.analysisRun.run?.elapsedSeconds)
      assertEquals(prior, h.state.analysisRun.previousRun)
      assertEquals("new-generation", h.state.analysisRun.run?.identity?.generation)
      assertTrue(
          h.state.analysisRun.sections.values.all {
            it.results?.identity == h.state.analysisRun.run?.identity
          })
    }
  }

  @Test
  fun oldGenerationResultReadCannotBePublishedAsResumedEvidence() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      val oldIdentity = h.state.analysisRun.run!!.identity
      h.workflow.preview(resume = true)
      h.drain()
      h.confirm()
      h.workflow.loadResults("bugs")
      h.main.runPending()
      h.io.runPending() // An old-generation result is waiting for delivery.
      h.workflow.admit()
      h.drain()
      val newIdentity = h.state.analysisRun.run!!.identity
      assertEquals(oldIdentity.queue(), newIdentity.queue())
      assertEquals(oldIdentity.id, newIdentity.id)
      assertTrue(oldIdentity.generation != newIdentity.generation)
      assertEquals(
          newIdentity,
          h.state.analysisRun.sections.getValue(AnalysisResultKey("bugs")).results?.identity)
      assertTrue(h.state.analysisRun.sections.values.all { it.results?.identity == newIdentity })
    }
  }

  @Test
  fun resumeCannotAcceptThePriorGenerationOrChangedPlanAsAContinuation() {
    for (mismatch in listOf("generation", "plan")) Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      val original = h.state.analysisRun.run
      h.workflow.preview(resume = true)
      h.drain()
      h.confirm()
      h.wrongResumeGeneration = mismatch == "generation"
      h.wrongResumePlan = mismatch == "plan"
      h.workflow.admit()
      h.drain()
      assertEquals(original, h.state.analysisRun.run)
      assertEquals(AdmissionRecovery.Uncertain, h.state.analysisRun.admissionRecovery)
      assertNull(h.state.analysisRun.admission)
      h.workflow.admit()
      h.drain()
      assertEquals(
          1, h.calls.count { it.first == "POST" && it.second.endsWith("/control") }, mismatch)
    }
  }

  @Test
  fun repeatedResumeWhilePreviewPendingKeepsTheFirstRequestAndItsReview() {
    Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      h.calls.clear()
      h.bodies.clear()

      h.workflow.preview(resume = true)
      h.workflow.preview(resume = true) // First preview is still queued on the UI dispatcher.
      h.main.runPending()
      h.workflow.preview(resume = true) // The request is queued on the IO dispatcher.
      h.io.runPending()
      h.workflow.preview(resume = true) // The response is queued for delivery.
      h.main.runPending()

      assertEquals(1, h.calls.count { it.first == "POST" && it.second.endsWith("/preview") })
      assertEquals(1, h.bodies.size)
      assertNotNull(h.state.analysisRun.admission)
      assertNull(h.state.analysisRun.error)
      h.confirm()
      h.workflow.admit()
      h.drain()
      assertEquals(1, h.calls.count { it.first == "POST" && it.second.endsWith("/control") })
    }
  }

  @Test
  fun continuationPreviewRequiresCurrentPausedOrInterruptedRun() {
    for (status in
        listOf(
            "queued",
            "running",
            "pausing",
            "canceling",
            "canceled",
            "completed",
            "failed",
            "stale",
            "unknown",
            "paused",
            "interrupted")) {
      Harness().use { h ->
        h.run = h.run.copy(status = status)
        h.workflow.refresh()
        h.drain()
        h.calls.clear()
        h.bodies.clear()
        h.workflow.preview(resume = true)
        h.drain()
        assertEquals(
            status in setOf("paused", "interrupted"),
            h.calls.any { it.second.endsWith("/preview") },
            status)
        assertEquals(
            status in setOf("paused", "interrupted"), h.state.analysisRun.admission != null, status)
        assertTrue(h.calls.none { it.first == "POST" && it.second.endsWith("/control") }, status)
      }
    }
  }

  @Test
  fun statusChangeInvalidatesConfirmedContinuationEvenWithSameIdentityAndPlan() {
    for (status in listOf("interrupted", "canceling", "canceled", "completed")) {
      for (read in listOf(false, true)) Harness().use { h ->
        h.workflow.refresh()
        h.drain()
        h.workflow.preview(resume = true)
        h.drain()
        h.confirm()
        assertTrue(h.state.analysisRun.admission!!.isConfirmed())
        val previewCount = h.calls.count { it.second.endsWith("/preview") }
        h.run = h.run.copy(status = status)
        if (read) h.workflow.refresh()
        else h.dispatch(DesktopEvent.AnalysisRunUpdated(h.state.analysisRun.copy(run = h.run)))
        h.drain()
        assertEquals(status, h.state.analysisRun.run?.status)
        assertNull(h.state.analysisRun.previewIntent)
        assertNull(h.state.analysisRun.admission)
        h.workflow.admit()
        h.workflow.retryPreview()
        h.drain()
        assertEquals(previewCount, h.calls.count { it.second.endsWith("/preview") })
        assertTrue(h.calls.none { it.first == "POST" && it.second.endsWith("/control") })
      }
    }
  }

  @Test
  fun continuationConsentIsInvalidAfterProjectRevisionOrProviderReplacement() {
    for (change in listOf("project", "revision", "provider")) Harness().use { h ->
      h.workflow.refresh()
      h.drain()
      h.workflow.preview(resume = true)
      h.drain()
      h.confirm()
      when (change) {
        "project" ->
            h.dispatch(
                DesktopEvent.ProjectLoaded(
                    analysisProjectFixture("other"), ProjectIndex("other", "revision")))
        "revision" -> h.dispatch(DesktopEvent.IndexRefreshed(ProjectIndex("project", "new")))
        else -> h.workflow.providerChanged()
      }
      h.workflow.admit()
      h.workflow.retryPreview()
      h.drain()
      assertNull(h.state.analysisRun.admission, change)
      assertNull(h.state.analysisRun.previewIntent, change)
      assertEquals(1, h.calls.count { it.second.endsWith("/preview") }, change)
      assertTrue(h.calls.none { it.first == "POST" && it.second.endsWith("/control") }, change)
    }
  }

  @Test
  fun statusChangeDuringPreviewOrFailedRetryCannotRestoreContinuation() {
    for (failed in listOf(false, true)) {
      Harness().use { h ->
        h.workflow.refresh()
        h.drain()
        if (failed) h.failure = "/analysis/preview"
        h.workflow.preview(resume = true)
        if (failed) h.drain()
        else {
          h.main.runPending()
          h.io.runPending() // Preview response queued on UI dispatcher.
        }
        h.run = h.run.copy(status = "canceled")
        h.dispatch(DesktopEvent.AnalysisRunUpdated(h.state.analysisRun.copy(run = h.run)))
        h.failure = ""
        h.workflow.retryPreview()
        h.drain()
        assertNull(h.state.analysisRun.previewIntent)
        assertNull(h.state.analysisRun.admission)
        assertEquals(1, h.calls.count { it.second.endsWith("/preview") })
        assertTrue(h.calls.none { it.first == "POST" && it.second.endsWith("/control") })
      }
    }
  }

  @Test
  fun admissionRechecksStatusBeforeConsumingConfirmationAndQueuedDispatch() {
    for (queued in listOf(false, true)) {
      Harness().use { h ->
        h.workflow.refresh()
        h.drain()
        h.workflow.preview(resume = true)
        h.drain()
        h.confirm()
        h.run = h.run.copy(status = "canceled")
        if (queued) {
          h.workflow.admit()
          h.dispatch(DesktopEvent.AnalysisRunUpdated(h.state.analysisRun.copy(run = h.run)))
        } else {
          h.dispatch(DesktopEvent.AnalysisRunUpdated(h.state.analysisRun.copy(run = h.run)))
          h.workflow.admit()
        }
        h.drain()
        assertNull(h.state.analysisRun.previewIntent)
        assertNull(h.state.analysisRun.admission)
        assertTrue(h.calls.none { it.first == "POST" && it.second.endsWith("/control") })
      }
    }
  }

  private class Harness(pollMillis: Long = 100_000) : AutoCloseable {
    val main = AnalysisQueuedDispatcher()
    val io = AnalysisQueuedDispatcher()
    private val scope = CoroutineScope(SupervisorJob() + main)
    var state =
        DesktopState()
            .reduce(
                DesktopEvent.ProjectLoaded(
                    analysisProjectFixture(), ProjectIndex("project", "revision")))
    val coordinator = DesktopJobCoordinator(scope, pollMillis)
    val calls = mutableListOf<Pair<String, String>>()
    val bodies = mutableListOf<String>()
    val runUpdates = mutableListOf<ProjectAnalysisRunState>()
    var run = analysisRunFixture()
    var selection = selectionFixture()
    var failure = ""
    var wrongResult = false
    var wrongRetryPreview = false
    var wrongResumeGeneration = false
    var wrongResumePlan = false
    var emptyPreview = false
    var admissionConflict = false
    var controlConflict = false
    var failStatus = false
    var admissionTransportFailure = false
    var cancelAdmission = false
    var lateAdmissionGate: Pair<CountDownLatch, CountDownLatch>? = null
    var lateControlGate: Pair<CountDownLatch, CountDownLatch>? = null
    var controlSettledStatus: String? = null
    var lateControlFailure = false
    var lateAdmissionNewIdentity = false
    var lateStatusGate: Pair<CountDownLatch, CountDownLatch>? = null
    var lateStatusFailure = false
    var staleStatus: AnalysisRun? = null
    var staleStatusReads = 0
    var previewId = "preview"
    val workflow =
        DesktopAnalysisWorkflow(
            ApiClient(
                transport =
                    DaemonTransport { method, path, body ->
                      calls.add(method to path)
                      if (body != null) bodies.add(body)
                      if (cancelAdmission && method == "POST" && path.endsWith("/analysis/run"))
                          throw CancellationException("Canceled admission")
                      if (method == "POST" &&
                          (path.endsWith("/analysis/run") || path.endsWith("/control"))) {
                        lateAdmissionGate?.let { (entered, release) ->
                          entered.countDown()
                          check(release.await(5, TimeUnit.SECONDS)) { "Admission gate timed out" }
                          run =
                              run.copy(
                                  identity =
                                      if (lateAdmissionNewIdentity)
                                          run.identity.copy(
                                              id =
                                                  if (path.endsWith("/control")) run.identity.id
                                                  else "admitted-run",
                                              generation = "admitted-generation")
                                      else run.identity,
                                  status = "running",
                                  updatedAt = "admitted")
                        }
                      }
                      if (failStatus && method == "GET" && path.contains("/analysis/run?"))
                          TransportResponse(500, """{"message":"status unavailable"}""")
                      else if (controlConflict && method == "POST" && path.endsWith("/control"))
                          TransportResponse(409, """{"message":"Control rejected"}""")
                      else if ((failure.isNotEmpty() && path.contains(failure)) ||
                          (admissionTransportFailure &&
                              method == "POST" &&
                              (path.endsWith("/analysis/run") || path.endsWith("/control"))))
                          TransportResponse(500, """{"message":"unavailable"}""")
                      else if (admissionConflict &&
                          method == "POST" &&
                          (path.endsWith("/analysis/run") || path.endsWith("/control")))
                          TransportResponse(409, """{"message":"Admission rejected"}""")
                      else
                          when {
                            path.endsWith("/preview") -> {
                              val request = Json.decodeFromString<AnalysisPreviewRequest>(body!!)
                              TransportResponse(
                                  200,
                                  Json.encodeToString(
                                      analysisPreviewFixture()
                                          .copy(
                                              previewId = previewId,
                                              refresh = request.refresh,
                                              limits = request.limits,
                                              retryStaleFailed =
                                                  request.retryStaleFailed && !wrongRetryPreview,
                                              files =
                                                  if (emptyPreview) emptyList()
                                                  else analysisPreviewFixture().files)))
                            }
                            path.endsWith("/control") -> {
                              val request = Json.decodeFromString<AnalysisRunControlRequest>(body!!)
                              val response =
                                  when (request.action) {
                                    "resume" ->
                                        run.copy(
                                            identity =
                                                run.identity.copy(
                                                    generation =
                                                        if (wrongResumeGeneration)
                                                            run.identity.generation
                                                        else "new-generation"),
                                            plan =
                                                if (wrongResumePlan)
                                                    run.plan.copy(refresh = !run.plan.refresh)
                                                else run.plan)
                                    "cancel" ->
                                        run.copy(status = controlSettledStatus ?: "canceled")
                                    else -> run.copy(status = controlSettledStatus ?: "paused")
                                  }
                              val gate = if (request.action == "pause") lateControlGate else null
                              if (gate != null) {
                                lateControlGate = null
                                gate.first.countDown()
                                check(gate.second.await(5, TimeUnit.SECONDS)) {
                                  "Control gate timed out"
                                }
                              } else run = response
                              if (gate != null && lateControlFailure)
                                  TransportResponse(500, """{"message":"late pause failure"}""")
                              else TransportResponse(200, Json.encodeToString(response))
                            }
                            path.contains("/analysis/results?") -> {
                              val category = path.substringAfter("category=").substringBefore('&')
                              val filter = path.substringAfter("&path=", "")
                              var result = analysisResultsFixture(run, category, filter)
                              if (wrongResult) result = result.copy(path = "wrong.go")
                              TransportResponse(200, Json.encodeToString(result))
                            }
                            path.contains("/analysis/selection?") ->
                                TransportResponse(200, Json.encodeToString(selection))
                            path.endsWith("/analysis/selection") && method == "POST" -> {
                              val request = Json.decodeFromString<AnalysisSelectionRequest>(body!!)
                              assertEquals(selection.selectionId, request.selectionId)
                              selection =
                                  selection.copy(
                                      selectionId = "saved", excludedPaths = request.excludedPaths)
                              TransportResponse(200, Json.encodeToString(selection))
                            }
                            path.contains("/overview?") ->
                                TransportResponse(
                                    200, Json.encodeToString(ProjectOverview(analysisRun = run)))
                            path.contains("/analysis/run") -> {
                              if (method == "GET" && lateStatusGate != null) {
                                val (entered, release) = lateStatusGate!!
                                lateStatusGate = null
                                val captured = run
                                entered.countDown()
                                check(release.await(5, TimeUnit.SECONDS)) {
                                  "Status gate timed out"
                                }
                                if (lateStatusFailure)
                                    TransportResponse(500, """{"message":"late poll failure"}""")
                                else TransportResponse(200, Json.encodeToString(captured))
                              } else if (method == "GET" && staleStatusReads > 0) {
                                staleStatusReads--
                                TransportResponse(200, Json.encodeToString(staleStatus!!))
                              } else if (method == "POST") {
                                val request = Json.decodeFromString<AnalysisRunStartRequest>(body!!)
                                run =
                                    run.copy(
                                        plan =
                                            run.plan.copy(
                                                refresh = request.refresh,
                                                retryStaleFailed = request.retryStaleFailed,
                                                limits = request.limits))
                                TransportResponse(200, Json.encodeToString(run))
                              } else TransportResponse(200, Json.encodeToString(run))
                            }
                            else -> error("unexpected $method $path")
                          }
                    }),
            scope,
            io,
            coordinator,
            { state },
            ::dispatch)

    init {
      coordinator.projectOpened(WorkflowProjectIdentity("project", "revision"))
    }

    fun dispatch(event: DesktopEvent) {
      workflow.beforeEvent(event)
      if (event is DesktopEvent.AnalysisRunUpdated) runUpdates.add(event.state)
      state = state.reduce(event)
      if (event is DesktopEvent.ProjectLoaded)
          coordinator.projectOpened(
              WorkflowProjectIdentity(event.project.projectId, event.project.projectRevision))
    }

    fun assertPreviewOnly() {
      assertEquals(
          listOf("POST" to "/api/projects/current/analysis/preview"),
          calls.filter { it.first != "GET" }.distinct())
    }

    fun confirm() {
      workflow.confirmProvider("bug-provider", true)
      workflow.confirmProvider("analyze-provider", true)
      workflow.confirmSecurity(true)
    }

    fun drain() {
      repeat(8) {
        main.runPending()
        io.runPending()
      }
      main.runPending()
    }

    fun await(predicate: () -> Boolean) {
      repeat(100) {
        drain()
        if (predicate()) return
        Thread.sleep(5)
      }
      assertTrue(predicate(), "Timed out waiting for analysis state")
    }

    override fun close() {
      workflow.detach()
      coordinator.close()
      scope.cancel()
      drain()
    }
  }
}

internal class AnalysisQueuedDispatcher : CoroutineDispatcher() {
  private val pending = java.util.concurrent.ConcurrentLinkedQueue<Runnable>()

  override fun dispatch(context: CoroutineContext, block: Runnable) {
    pending.add(block)
  }

  fun runPending() {
    while (true) (pending.poll() ?: return).run()
  }
}

internal fun analysisPreviewFixture() =
    AnalysisRunPreview(
        "1",
        "preview",
        AnalysisQueueIdentity("project", "revision", "policy", "providers", "queue"),
        "project",
        false,
        AnalysisRunLimits(100, 900, 2),
        files = listOf(AnalysisPlannedFile("main.go", "base", "Go", 20, emptyList())),
        providers =
            listOf(
                AnalysisProviderRequirement(
                    "bug-provider",
                    listOf("semantic"),
                    AnalysisEffectiveModel(
                        "bug",
                        "remote",
                        "bug-model",
                        providerOrigin = "https://bug.example",
                        remoteProvider = true,
                        timeout = "2m0s"),
                    true),
                AnalysisProviderRequirement(
                    "analyze-provider",
                    listOf("performance", "security_ai"),
                    AnalysisEffectiveModel(
                        "analyze",
                        "remote",
                        "review-model",
                        providerOrigin = "https://analyze.example",
                        remoteProvider = true,
                        timeout = "5m0s"),
                    true)),
        expectedModelRequests = 3,
        maxModelRequests = 6,
        securityReviewIntentRequired = true)

internal fun analysisRunFixture() =
    AnalysisRun(
        "1",
        AnalysisRunIdentity(
            "project", "revision", "policy", "providers", "queue", "run", "generation"),
        analysisPreviewFixture(),
        "paused",
        files = listOf(AnalysisRunFile("main.go", "base", "Go", emptyList())),
        sections =
            listOf("bugs", "performance", "security").map {
              AnalysisSectionProgress(
                  it, "paused", AnalysisRunCoverage(total = 1, succeeded = 1), 1)
            })

internal fun analysisResultsFixture(run: AnalysisRun, category: String, path: String = "") =
    AnalysisSectionResults(
        run.identity,
        run.sections.first { it.category == category },
        path,
        semantic =
            if (category == "bugs")
                listOf(
                    UnifiedFinding(
                        id = "bug",
                        category = "bugs",
                        message = run.status,
                        projectId = "project",
                        projectRevision = "revision",
                        fileHash = "base",
                        location = FindingLocation("main.go")))
            else emptyList(),
        performance =
            if (category == "performance")
                listOf(
                    PerformanceFileReport(
                        projectId = "project",
                        projectRevision = "revision",
                        path = "main.go",
                        contentHash = "base",
                        status = "completed_empty"))
            else emptyList(),
        security =
            if (category == "security")
                listOf(
                    SecurityFileReport(
                        projectId = "project",
                        projectRevision = "revision",
                        path = "main.go",
                        contentHash = "base",
                        status = "completed",
                        source = "ai",
                        findings =
                            listOf(
                                SecurityFinding(
                                    id = "security", verificationState = "unverified"))))
            else emptyList())

internal fun analysisProjectFixture(id: String = "project") =
    ProjectAnalysis(
        id,
        "revision",
        id,
        "/tmp/$id",
        "go",
        fileCount = 1,
        sourceFileCount = 1,
        totalLines = 1,
        summary = "",
        aiStatus = "missing",
        analyzedAt = "")

internal fun analysisFileFixture(path: String = "main.go") =
    ProjectFileInfo(
        path,
        "base",
        path,
        language = "Go",
        sizeBytes = 20,
        lineCount = 1,
        modifiedAt = "",
        binary = false,
        content = "package main")
