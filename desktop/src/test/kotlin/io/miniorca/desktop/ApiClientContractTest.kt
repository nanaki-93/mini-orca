package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertContains
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json

class ApiClientContractTest {
    @Test fun workspaceResponsesDecodeToTypedModelsAndIgnoreUnknownFields() {
        val client = ApiClient(transport = DaemonTransport { method, path, _ ->
            when (method to path) {
                "GET" to "/api/projects/current/overview?project_revision=revision" -> TransportResponse(
                    200,
                    """{"project_id":"project","project_revision":"revision","metrics":{"type":"go","file_count":3,"unknown_metric":"ignored"},"analysis":{"purpose":"Builds the daemon","components":null,"future_field":"ignored"},"analysis_coverage":{"total":3,"fresh":2},"finding_counts":{"verified":1,"ai_suggestions":2},"source":"must not become a model field"}""",
                )
                "GET" to "/api/projects/current/findings?project_revision=revision" -> TransportResponse(
                    200,
                    """{"project_id":"project","project_revision":"revision","findings":[{"id":"finding-1","source":"vet","confidence":"tool_reported","severity":"warning","title":"Vet issue","message":"check this","location":{"path":"main.go","start_line":7},"unknown":"ignored"}]}""",
                )
                else -> error("unexpected request: $method $path")
            }
        })

        val overview = client.overview("revision")
        val findings = client.findings("revision")

        assertEquals("go", overview.metrics.type)
        assertEquals(emptyList(), overview.analysis.components)
        assertEquals(2, overview.findingCounts.aiSuggestions)
        assertEquals("main.go", findings.findings.single().location.path)
        assertEquals(7, findings.findings.single().location.startLine)
        assertFalse(Json.encodeToString(overview).contains("must not become a model field"))
    }

    @Test fun noContentProgressAndTriageResponsesDoNotDecodeEmptyBodies() {
        val client = ApiClient(transport = DaemonTransport { method, path, body ->
            when (method to path) {
                "GET" to "/api/projects/current/scan?project_revision=revision" -> TransportResponse(204, "")
                "GET" to "/api/projects/current/analysis-job?project_revision=revision" -> TransportResponse(204, "")
                "PATCH" to "/api/projects/current/findings/finding%201" -> {
                    assertContains(body.orEmpty(), "\"project_revision\":\"revision\"")
                    assertContains(body.orEmpty(), "\"status\":\"dismissed\"")
                    TransportResponse(204, "")
                }
                else -> error("unexpected request: $method $path")
            }
        })

        assertNull(client.goScan("revision"))
        assertNull(client.analyzeAllJob("revision"))
        client.updateFindingStatus("finding 1", "revision", "dismissed")
    }

    @Test fun scanAndAnalyzeAllActionsUseTypedProgressContracts() {
        val client = ApiClient(transport = DaemonTransport { method, path, body ->
            when (method to path) {
                "POST" to "/api/projects/current/scan" -> {
                    assertContains(body.orEmpty(), "\"project_revision\":\"revision\"")
                    TransportResponse(200, """{"project_id":"project","project_revision":"revision","status":"running","phases":null}""")
                }
                "DELETE" to "/api/projects/current/scan?project_revision=revision" ->
                    TransportResponse(200, """{"project_id":"project","project_revision":"revision","status":"canceled","phases":[]}""")
                "POST" to "/api/projects/current/analysis-job" -> {
                    assertContains(body.orEmpty(), "\"max_files\":10")
                    assertContains(body.orEmpty(), "\"max_retries\":2")
                    assertContains(body.orEmpty(), "\"confirm_remote_provider\":true")
                    TransportResponse(200, jobResponse("running"))
                }
                "POST" to "/api/projects/current/analysis-job/pause?project_revision=revision" ->
                    TransportResponse(200, jobResponse("paused"))
                "POST" to "/api/projects/current/analysis-job/resume" -> {
                    assertContains(body.orEmpty(), "\"confirm_remote_provider\":true")
                    TransportResponse(200, jobResponse("running"))
                }
                "POST" to "/api/projects/current/analysis-job/cancel?project_revision=revision" ->
                    TransportResponse(200, jobResponse("canceled"))
                else -> error("unexpected request: $method $path")
            }
        })

        assertEquals("running", client.startGoScan("revision").status)
        assertEquals("canceled", client.cancelGoScan("revision").status)
        assertEquals("running", client.startAnalyzeAll("revision", maxFiles = 10, maxRetries = 2, confirmRemoteProvider = true).status)
        assertEquals("paused", client.pauseAnalyzeAll("revision").status)
        assertEquals("running", client.resumeAnalyzeAll("revision", confirmRemoteProvider = true).status)
        assertEquals("canceled", client.cancelAnalyzeAll("revision").status)
    }

    @Test fun chatAndDraftActionsUseBoundTypedContracts() {
        val client = ApiClient(transport = DaemonTransport { method, path, body ->
            when (method to path) {
                "POST" to "/api/projects/current/chat/sessions" -> {
                    assertContains(body.orEmpty(), "\"mode\":\"replace_symbol\"")
                    assertContains(body.orEmpty(), "\"target_symbol\":\"Run\"")
                    TransportResponse(201, """{"id":"session","project_id":"project","project_revision":"revision","base_file_hash":"base","open_path":"main.go","mode":"replace_symbol","target_symbol":"Run","state":"active","messages":null}""")
                }
                "GET" to "/api/projects/current/chat/sessions/session" ->
                    TransportResponse(200, """{"id":"session","project_id":"project","project_revision":"revision","base_file_hash":"base","open_path":"main.go","mode":"replace_symbol","target_symbol":"Run","state":"active","messages":[{"role":"user","content":"Improve Run"}]}""")
                "POST" to "/api/projects/current/chat/sessions/session/messages" -> {
                    assertContains(body.orEmpty(), "\"parent_draft_id\":\"older\"")
                    assertContains(body.orEmpty(), "\"confirm_remote_provider\":true")
                    TransportResponse(200, """{"session_id":"session","draft":{"id":"draft","project_id":"project","project_revision":"revision","base_file_hash":"base","target_path":"main.go","mode":"replace_symbol","target_symbol":"Run","declaration":"func Run() {}","revision":1,"hash":"hash","state":"generated"},"assistant_message":{"role":"assistant","content":"Proposal ready"},"context_manifest":{"included":null}}""")
                }
                "PATCH" to "/api/projects/current/drafts/draft" -> {
                    assertContains(body.orEmpty(), "\"expected_revision\":1")
                    assertContains(body.orEmpty(), "\"imports\":[\"fmt\"]")
                    TransportResponse(200, draftResponse())
                }
                "POST" to "/api/projects/current/drafts/draft/validate" -> TransportResponse(200, draftResponse())
                "POST" to "/api/projects/current/drafts/draft/checks" -> {
                    assertContains(body.orEmpty(), "\"expected_hash\":\"hash\"")
                    TransportResponse(200, """{"target_path":"main.go","applicable":true,"checks":null,"draft_id":"draft","draft_revision":1,"draft_hash":"hash"}""")
                }
                "GET" to "/api/projects/current/drafts/draft/review?project_revision=revision" -> TransportResponse(200, """{"draft":${draftResponse()},"checks":null,"apply_eligible":true}""")
                "POST" to "/api/projects/current/apply" -> {
                    assertContains(body.orEmpty(), "\"draft_id\":\"draft\"")
                    assertContains(body.orEmpty(), "\"confirm\":true")
                    TransportResponse(200, """{"project_revision":"next","post_apply_hash":"after","undo_available":true}""")
                }
                else -> error("unexpected request: $method $path")
            }
        })

        val session = client.openChatSession("project", "revision", "base", "main.go", "replace_symbol", "Run")
        val resumed = client.chatSession(session.id)
        val proposal = client.sendChatMessage(session.id, "Improve Run", "older", confirmRemoteProvider = true)
        val updated = client.updateDraft("draft", "revision", 1, "func Run() {}", listOf("fmt"))
        val validated = client.validateDraft("draft", "revision", updated.revision)
        val checks = client.checkDraft("draft", "revision", validated.revision, validated.hash)
        val review = client.draftReview("draft", "revision")
        val applied = client.applyDraft(validated)

        assertEquals(emptyList(), session.messages)
        assertEquals("Improve Run", resumed.messages.single().content)
        assertEquals("draft", proposal.draft.id)
        assertEquals(emptyList(), proposal.contextManifest.included)
        assertEquals("draft", checks.draftId)
        assertTrue(review.applyEligible)
        assertEquals("next", applied.projectRevision)
    }

    @Test fun structuredFailuresExposeTheDaemonError() {
        val client = ApiClient(transport = DaemonTransport { _, _, _ ->
            TransportResponse(409, """{"type":"conflict","message":"stale revision","user_message":"Reload the project first."}""")
        })

        val failure = runCatching { client.startGoScan("old-revision") }.exceptionOrNull()

        assertTrue(failure is ApiException)
        assertEquals(409, failure.status)
        assertEquals("conflict", failure.error?.type)
        assertEquals("Reload the project first.", failure.message)
    }

    private fun draftResponse() = """{"id":"draft","project_id":"project","project_revision":"revision","base_file_hash":"base","target_path":"main.go","mode":"replace_symbol","target_symbol":"Run","declaration":"func Run() {}","imports":null,"revision":1,"hash":"hash","state":"valid","validation":{"applicable":true,"scope_mode":"replace_symbol","diagnostics":null,"diff":{"old_path":"main.go","new_path":"main.go","lines":null}}}"""

    private fun jobResponse(status: String) = """{"project_id":"project","project_revision":"revision","status":"$status","max_files":10,"max_retries":2,"files":null}"""
}
