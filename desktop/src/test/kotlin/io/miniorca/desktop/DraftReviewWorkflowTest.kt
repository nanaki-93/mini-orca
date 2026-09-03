package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertContains
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class DraftReviewWorkflowTest {
    @Test fun editedDraftRequiresFreshValidationAndChecksBeforeTheFakeTransportCanApplyAndUndo() {
        val requests = mutableListOf<String>()
        val client = ApiClient(transport = DaemonTransport { method, path, body ->
            requests += "$method $path ${body.orEmpty()}"
            when (method to path) {
                "PATCH" to "/api/projects/current/drafts/draft" -> {
                    assertContains(body.orEmpty(), "\"expected_revision\":2")
                    TransportResponse(200, draftJson(3, "edited", validation = "null"))
                }
                "POST" to "/api/projects/current/drafts/draft/validate" -> TransportResponse(200, draftJson(3, "validated", validation = "{\"applicable\":true,\"scope_mode\":\"replace_symbol\",\"diff\":{\"old_path\":\"main.go\",\"new_path\":\"main.go\",\"lines\":[]}}"))
                "POST" to "/api/projects/current/drafts/draft/checks" -> TransportResponse(200, "{\"target_path\":\"main.go\",\"applicable\":true,\"draft_id\":\"draft\",\"draft_revision\":3,\"draft_hash\":\"validated\",\"checks\":[]}")
                "POST" to "/api/projects/current/apply" -> TransportResponse(200, "{\"project_revision\":\"next\",\"post_apply_hash\":\"after\",\"undo_available\":true}")
                "POST" to "/api/projects/current/undo" -> TransportResponse(200, "{\"project_revision\":\"restored\",\"post_apply_hash\":\"base\",\"undo_available\":false}")
                else -> error("Unexpected request: $method $path")
            }
        })
        val original = draft(2, "base", validation = DeclarationValidation(true, "replace_symbol", diff = UnifiedDiff("main.go", "main.go")))
        val editedState = DesktopState(review = DraftReviewState(draft = original, editor = editableDraft(original)))
            .reduce(DesktopEvent.DraftEdited(declaration = "func Run() error { return nil }"))

        assertFalse(draftReviewEligibility(editedState.review.editor, editedState.review.draft, editedState.review.checks, file(), project()).eligible)
        val updated = client.updateDraft(original.id, "revision", original.revision, editedState.review.editor!!.declaration, editedState.review.editor.imports)
        val validated = client.validateDraft(updated.id, "revision", updated.revision)
        val validatedState = editedState.reduce(DesktopEvent.DraftLoaded(validated))
        val checks = client.checkDraft(validated.id, "revision", validated.revision, validated.hash)
        val checkedState = validatedState.reduce(DesktopEvent.ChecksLoaded(checks))

        assertTrue(draftReviewEligibility(checkedState.review.editor, checkedState.review.draft, checkedState.review.checks, file(), project()).eligible)
        val applied = client.applyDraft(validated)
        val undone = client.undo("project", applied.projectRevision, applied.postApplyHash)

        assertTrue(applied.undoAvailable)
        assertFalse(undone.undoAvailable)
        assertTrue(requests.any { it.startsWith("POST /api/projects/current/apply") })
        assertTrue(requests.any { it.startsWith("POST /api/projects/current/undo") })
    }

    private fun draft(revision: Long, hash: String, validation: DeclarationValidation?) = DeclarationDraft("draft", "project", "revision", "base", "main.go", "replace_symbol", "Run", "func Run() {}", revision = revision, hash = hash, validation = validation)
    private fun file() = ProjectFileInfo("main.go", "base", "main.go", language = "Go", sizeBytes = 1, lineCount = 1, modifiedAt = "", binary = false)
    private fun project() = ProjectAnalysis("project", "revision", "project", "/tmp/project", "go", fileCount = 1, sourceFileCount = 1, totalLines = 1, summary = "", aiStatus = "fresh", analyzedAt = "")
    private fun draftJson(revision: Long, hash: String, validation: String) = "{\"id\":\"draft\",\"project_id\":\"project\",\"project_revision\":\"revision\",\"base_file_hash\":\"base\",\"target_path\":\"main.go\",\"mode\":\"replace_symbol\",\"target_symbol\":\"Run\",\"declaration\":\"func Run() error { return nil }\",\"imports\":[],\"revision\":$revision,\"hash\":\"$hash\",\"state\":\"valid\",\"validation\":$validation}"
}
