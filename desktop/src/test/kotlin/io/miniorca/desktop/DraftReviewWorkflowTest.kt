package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertContains
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class DraftReviewWorkflowTest {
  @Test
  fun editedDraftRequiresFreshValidationAndChecksBeforeTheFakeTransportCanApplyAndUndo() {
    listOf("replace_symbol", "create_symbol").forEach { mode ->
      val requests = mutableListOf<String>()
      val client =
          ApiClient(
              transport =
                  DaemonTransport { method, path, body ->
                    requests += "$method $path ${body.orEmpty()}"
                    when (method to path) {
                      "PATCH" to "/api/projects/current/drafts/draft" -> {
                        assertContains(body.orEmpty(), "\"expected_revision\":2")
                        TransportResponse(
                            200, draftJson(3, "edited", validation = "null", mode = mode))
                      }
                      "POST" to "/api/projects/current/drafts/draft/validate" ->
                          TransportResponse(
                              200,
                              draftJson(
                                  3,
                                  "validated",
                                  validation =
                                      "{\"applicable\":true,\"scope_mode\":\"symbol_plus_imports\",\"diff\":{\"old_path\":\"main.go\",\"new_path\":\"main.go\",\"lines\":[]}}",
                                  mode = mode))
                      "POST" to "/api/projects/current/drafts/draft/checks" ->
                          TransportResponse(
                              200,
                              "{\"target_path\":\"main.go\",\"applicable\":true,\"draft_id\":\"draft\",\"draft_revision\":3,\"draft_hash\":\"validated\",\"checks\":[]}")
                      "POST" to "/api/projects/current/apply" ->
                          TransportResponse(
                              200,
                              "{\"project_revision\":\"next\",\"post_apply_hash\":\"after\",\"undo_available\":true}")
                      "POST" to "/api/projects/current/undo" ->
                          TransportResponse(
                              200,
                              "{\"project_revision\":\"restored\",\"post_apply_hash\":\"base\",\"undo_available\":false}")
                      else -> error("Unexpected request: $method $path")
                    }
                  })
      val original =
          draft(
              2,
              "base",
              validation =
                  DeclarationValidation(
                      true, "symbol_plus_imports", diff = UnifiedDiff("main.go", "main.go")),
              mode = mode)
      val editedState =
          DesktopState(
                  review = DraftReviewState(draft = original, editor = editableDraft(original)))
              .reduce(DesktopEvent.DraftEdited(declaration = "func Run() error { return nil }"))

      assertFalse(
          draftReviewEligibility(
                  editedState.review.editor,
                  editedState.review.draft,
                  editedState.review.checks,
                  file(),
                  project())
              .eligible)
      val updated =
          client.updateDraft(
              original.id,
              "revision",
              original.revision,
              editedState.review.editor!!.declaration,
              editedState.review.editor.imports)
      val validated = client.validateDraft(updated.id, "revision", updated.revision)
      val validatedState = editedState.reduce(DesktopEvent.DraftLoaded(validated))
      assertFalse(
          draftReviewEligibility(validatedState.review.editor, validated, null, file(), project())
              .eligible)
      assertFalse(requests.any { it.startsWith("POST /api/projects/current/apply") })
      val checks = client.checkDraft(validated.id, "revision", validated.revision, validated.hash)
      val checkedState = validatedState.reduce(DesktopEvent.ChecksLoaded(checks))
      assertTrue(checks.checks.isEmpty())
      val rerunning = checkedState.reduce(DesktopEvent.ChecksStarted(1, CheckCandidate(validated)))
      assertFalse(
          draftReviewEligibility(
                  rerunning.review.editor,
                  rerunning.review.draft,
                  rerunning.review.checks,
                  file(),
                  project(),
                  rerunning.review.checkAttempt)
              .eligible)
      val canceled =
          rerunning.reduce(
              DesktopEvent.ChecksStopped(1, ValidationAttemptStatus.Canceled, "Canceled by user"))
      assertFalse(
          draftReviewEligibility(
                  canceled.review.editor,
                  canceled.review.draft,
                  canceled.review.checks,
                  file(),
                  project(),
                  canceled.review.checkAttempt)
              .eligible)
      assertTrue(
          canceled
              .reduce(DesktopEvent.ChecksStarted(2, CheckCandidate(validated)))
              .reduce(DesktopEvent.ChecksCompleted(2, checks))
              .review
              .checkAttempt == null)

      assertTrue(
          draftReviewEligibility(
                  checkedState.review.editor,
                  checkedState.review.draft,
                  checkedState.review.checks,
                  file(),
                  project())
              .eligible)
      val applied = client.applyDraft(validated)
      val undone = client.undo("project", applied.projectRevision, applied.postApplyHash)

      assertTrue(applied.undoAvailable)
      assertFalse(undone.undoAvailable)
      assertTrue(requests.any { it.startsWith("POST /api/projects/current/apply") })
      assertTrue(requests.any { it.startsWith("POST /api/projects/current/undo") })
    }
  }

  @Test
  fun revisionChangingIndexRetainsEditableBufferAndFindingsButRevokesDraftAuthority() {
    val validated =
        draft(
            2,
            "validated",
            DeclarationValidation(
                true,
                "symbol_plus_imports",
                diagnostics = listOf(DeclarationFinding("warning", "Prior diagnostic")),
                diff = UnifiedDiff("main.go", "main.go")),
            "replace_symbol")
    val checks =
        DraftCheckReport(
            "main.go",
            true,
            draftId = validated.id,
            draftRevision = validated.revision,
            draftHash = validated.hash)
    val finding =
        UnifiedFinding(projectId = "project", projectRevision = "revision", freshness = "fresh")
    val overview = ProjectOverview(projectId = "project", projectRevision = "revision")
    val index = ProjectIndex("project", "revision")
    val attempt = ProjectIndexingAttempt(1, "project", "revision", "/tmp/project")
    val original =
        DesktopState(
            projectState =
                ProjectWorkspaceState(
                    project = project(),
                    index = index,
                    overview = overview,
                    sourceChangeObserved = true),
            selection = FileSelectionState(selectedFile = file()),
            findings = FindingsState(findings = listOf(finding)),
            review =
                DraftReviewState(
                    draft = validated,
                    editor =
                        editableDraft(validated)
                            .copy(declaration = "func Run() error { return nil }"),
                    checks = checks),
            jobs = JobState(status = "Previous work"))
    assertTrue(
        draftReviewEligibility(original.review.editor, validated, checks, file(), project())
            .eligible)

    val running = original.reduce(DesktopEvent.ProjectIndexingStarted(attempt))
    val failed =
        running.reduce(
            DesktopEvent.ProjectIndexingStopped(
                attempt, ProjectIndexingOutcome.Failed("Index unavailable")))
    assertEquals(original.review, failed.review)
    assertEquals(index, failed.index)
    assertEquals(overview, failed.overview)
    assertEquals(listOf(finding), failed.findings.findings)

    val unchanged = running.reduce(DesktopEvent.ProjectIndexingCompleted(attempt, index))
    assertEquals(original.review, unchanged.review)
    assertEquals(overview, unchanged.overview)
    assertEquals(listOf(finding), unchanged.findings.findings)
    assertEquals("Project inventory refreshed", unchanged.status)
    assertTrue(
        draftReviewEligibility(
                unchanged.review.editor,
                unchanged.review.draft,
                unchanged.review.checks,
                file(),
                unchanged.project)
            .eligible)

    val changed =
        running.reduce(
            DesktopEvent.ProjectIndexingCompleted(attempt, ProjectIndex("project", "next")))
    assertEquals("next", changed.project?.projectRevision)
    assertEquals(DraftEditorStatus.Stale, changed.review.editor?.status)
    assertEquals("func Run() error { return nil }", changed.review.editor?.declaration)
    assertEquals(validated.id, changed.review.draft?.id)
    assertNull(changed.review.draft?.validation)
    assertNull(changed.review.editor?.serverDraft?.validation)
    assertNull(changed.review.editor?.validationAttempt)
    assertTrue(changed.review.editor?.diagnosticsAreRetained == true)
    assertEquals(
        "Prior diagnostic", requireNotNull(changed.review.editor).diagnostics.single().message)
    assertNull(changed.review.checks)
    assertNull(changed.review.checkAttempt)
    assertFalse(
        draftReviewEligibility(
                changed.review.editor, changed.review.draft, checks, file(), changed.project)
            .eligible)
    assertFalse(draftApplyEligibility(changed.review.draft, changed.review.checks, file()).eligible)
    assertEquals(overview, changed.overview)
    assertEquals(listOf(finding), changed.findings.findings)
    assertFalse(changed.projectState.sourceChangeObserved)
  }

  private fun draft(
      revision: Long,
      hash: String,
      validation: DeclarationValidation?,
      mode: String
  ) =
      DeclarationDraft(
          "draft",
          "project",
          "revision",
          "base",
          "main.go",
          mode,
          "Run",
          "func Run() {}",
          revision = revision,
          hash = hash,
          validation = validation)

  private fun file() =
      ProjectFileInfo(
          "main.go",
          "base",
          "main.go",
          language = "Go",
          sizeBytes = 1,
          lineCount = 1,
          modifiedAt = "",
          binary = false)

  private fun project() =
      ProjectAnalysis(
          "project",
          "revision",
          "project",
          "/tmp/project",
          "go",
          fileCount = 1,
          sourceFileCount = 1,
          totalLines = 1,
          summary = "",
          aiStatus = "fresh",
          analyzedAt = "")

  private fun draftJson(revision: Long, hash: String, validation: String, mode: String) =
      "{\"id\":\"draft\",\"project_id\":\"project\",\"project_revision\":\"revision\",\"base_file_hash\":\"base\",\"target_path\":\"main.go\",\"mode\":\"$mode\",\"target_symbol\":\"Run\",\"declaration\":\"func Run() error { return nil }\",\"imports\":[],\"revision\":$revision,\"hash\":\"$hash\",\"state\":\"valid\",\"validation\":$validation}"
}
