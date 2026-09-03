package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class BottomEvidenceToolWindowsTest {
  private val project =
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
  private val file =
      ProjectFileInfo(
          "main.go",
          "base",
          "main.go",
          language = "Go",
          sizeBytes = 1,
          lineCount = 1,
          modifiedAt = "",
          binary = false)
  private val draft =
      DeclarationDraft(
          id = "draft",
          projectId = "project",
          projectRevision = "revision",
          baseFileHash = "base",
          targetPath = "main.go",
          mode = "replace_symbol",
          targetSymbol = "Run",
          declaration = "func Run() {}",
          revision = 2,
          hash = "draft-hash",
          validation =
              DeclarationValidation(
                  true, "replace_symbol", diff = UnifiedDiff("main.go", "main.go")),
      )

  @Test
  fun checksPresentationReusesCurrentEvidenceAndMakesStaleEvidenceVisible() {
    val stale =
        DraftCheckReport(
            "main.go",
            true,
            checks =
                listOf(
                    DraftCheck(
                        "go test",
                        required = true,
                        state = "passed",
                        command = listOf("go", "test"))),
            draftId = draft.id,
            draftRevision = draft.revision,
            draftHash = "old-hash")

    val presentation =
        checksToolWindowPresentation(
            ChecksToolWindowState(project, file, editableDraft(draft), draft, stale, false))

    assertEquals(ReviewEvidenceStatus.Stale, presentation.evidence.checks.status)
    assertEquals(BottomToolWindowSummary("Checks stale", attention = true), presentation.summary)
    assertEquals(ReviewEvidenceStatus.Passed, presentation.checks.single().status)
    assertEquals("go test", presentation.checks.single().command)
  }

  @Test
  fun checksPresentationDescribesRequiredOptionalRunningAndFailedRowsWithoutApplyAuthority() {
    val checks =
        DraftCheckReport(
            "main.go",
            true,
            checks =
                listOf(
                    DraftCheck("go test", required = true, state = "failed", output = "assertion"),
                    DraftCheck("lint", required = false, state = "skipped"),
                    DraftCheck("format", required = true, state = "running")),
            draftId = draft.id,
            draftRevision = draft.revision,
            draftHash = draft.hash)

    val presentation =
        checksToolWindowPresentation(
            ChecksToolWindowState(project, file, editableDraft(draft), draft, checks, false))

    assertEquals(ReviewEvidenceStatus.Running, presentation.evidence.checks.status)
    assertEquals(BottomToolWindowSummary("Checks running"), presentation.summary)
    assertEquals(
        listOf(
            ReviewEvidenceStatus.Failed,
            ReviewEvidenceStatus.Skipped,
            ReviewEvidenceStatus.Running),
        presentation.checks.map { it.status })
    assertEquals(listOf(true, false, true), presentation.checks.map { it.required })
    assertEquals(
        BottomToolWindowSummary("Checks failed", attention = true),
        checksToolWindowPresentation(
                ChecksToolWindowState(
                    project,
                    file,
                    editableDraft(draft),
                    draft,
                    checks.copy(checks = checks.checks.filterNot { it.state == "running" }),
                    false))
            .summary)
  }

  @Test
  fun outputPresentationUsesPresenterStateAndBoundsSanitizedText() {
    val longFailure = "x".repeat(5_000) + "\u0000"
    val presentation =
        outputToolWindowPresentation(
            OutputToolWindowState(
                status = "Analysis canceled\u0001",
                error = "daemon\u0000 unavailable",
                loading = false,
                fileAnalysis = FileAnalysis("main.go", "failed", failure = longFailure),
                analyzeAll =
                    AnalyzeAllJob(
                        status = "canceled",
                        files = listOf(AnalyzeAllFileJob("main.go", "failed", 1, "bad output"))),
                coverage = null,
                scan =
                    GoScanReport(
                        status = "canceled",
                        phases = listOf(GoScanPhase("go vet", "failed", output = "vet output"))),
                analysisInProgress = false,
                generating = false,
                validating = false,
            ))

    assertTrue(presentation.entries.any { it.title == "Daemon failure" && it.failed })
    assertTrue(presentation.entries.any { it.title == "Analyze-all" && it.status == "Canceled" })
    assertTrue(presentation.entries.any { it.title == "Verified scan" && it.status == "Canceled" })
    assertEquals(BottomToolWindowSummary("Output has failures", true), presentation.summary)
    assertFalse(sanitizedOutputText(longFailure).contains('\u0000'))
    assertTrue(sanitizedOutputText(longFailure).endsWith("… output truncated"))
    assertTrue(sanitizedOutputText(longFailure).length <= 4_096 + "\n… output truncated".length)
  }

  @Test
  fun failedChecksTakeCollapsedSummaryAttentionWithoutChangingTheActiveTab() {
    val summary =
        bottomToolWindowSummary(
            BottomToolWindow.Problems,
            mapOf(
                BottomToolWindow.Problems to BottomToolWindowSummary("2 problems"),
                BottomToolWindow.Checks to BottomToolWindowSummary("Checks failed", true),
                BottomToolWindow.Output to BottomToolWindowSummary("Output ready"),
            ))

    assertEquals(BottomToolWindowSummary("Checks failed", true), summary)
  }
}
