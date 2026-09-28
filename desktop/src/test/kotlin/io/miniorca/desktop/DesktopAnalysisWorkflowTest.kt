package io.miniorca.desktop

import kotlin.coroutines.CoroutineContext
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue
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
      h.run = h.run.copy(status = "running")
      h.workflow.refresh()
      h.drain()
      h.workflow.preview(resume = true)
      h.drain()
      h.confirm()
      assertTrue(h.state.analysisRun.admission!!.isConfirmed())
      assertNotNull(h.state.analysisRun.previewIntent)

      h.run = h.run.copy(plan = h.run.plan.copy(limits = AnalysisRunLimits(3, 60, 1)))
      h.await { h.state.analysisRun.run?.plan == h.run.plan }
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
    var emptyPreview = false
    var admissionConflict = false
    var previewId = "preview"
    val workflow =
        DesktopAnalysisWorkflow(
            ApiClient(
                transport =
                    DaemonTransport { method, path, body ->
                      calls.add(method to path)
                      if (body != null) bodies.add(body)
                      if (failure.isNotEmpty() && path.contains(failure))
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
                              run =
                                  when (request.action) {
                                    "resume" ->
                                        run.copy(
                                            identity =
                                                run.identity.copy(generation = "new-generation"))
                                    "cancel" -> run.copy(status = "canceled")
                                    else -> run.copy(status = "paused")
                                  }
                              TransportResponse(200, Json.encodeToString(run))
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
                              if (method == "POST") {
                                val request = Json.decodeFromString<AnalysisRunStartRequest>(body!!)
                                run =
                                    run.copy(
                                        plan =
                                            run.plan.copy(
                                                refresh = request.refresh,
                                                retryStaleFailed = request.retryStaleFailed,
                                                limits = request.limits))
                              }
                              TransportResponse(200, Json.encodeToString(run))
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
