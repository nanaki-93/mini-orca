package io.miniorca.desktop

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class DesktopGenerationWorkflowTest {
    @Test fun importToPreviewChecksApplyAndUndoUsesOneFileAndOneSymbol() {
        val requests = mutableListOf<String>()
        val client = ApiClient(transport = DaemonTransport { method, path, body ->
            requests += "$method $path ${body.orEmpty()}"
            when (path.substringBefore('?')) {
                "/api/projects/import" -> TransportResponse(200, projectJson)
                "/api/projects/current/index" -> TransportResponse(200, """{"project_id":"project","project_revision":"revision","files":[{"path":"main.go","content_hash":"base","language":"Go","binary":false,"symbols":[{"name":"Run","kind":"function","confidence":"exact","atomic_target":true}]}]}""")
                "/api/projects/current/files/info" -> TransportResponse(200, """{"path":"main.go","content_hash":"base","name":"main.go","language":"Go","size_bytes":24,"line_count":2,"modified_at":"","binary":false,"content":"package main\\nfunc Run() {}"}""")
                "/api/projects/current/files/symbols" -> TransportResponse(200, """{"project_id":"project","project_revision":"revision","path":"main.go","symbols":[{"name":"Run","kind":"function","confidence":"exact","atomic_target":true}]}""")
                "/api/projects/current/files/analysis" -> TransportResponse(200, """{"path":"main.go","status":"fresh","purpose":"Runs work."}""")
                "/api/chat/message" -> TransportResponse(200, candidateJson)
                "/api/projects/current/candidates/checks" -> TransportResponse(200, """{"target_path":"main.go","applicable":true,"checks":[{"name":"parse","required":true,"state":"passed"},{"name":"format","required":true,"state":"passed"}]}""")
                "/api/projects/current/apply" -> TransportResponse(200, """{"project_revision":"revision-after-apply","post_apply_hash":"after","undo_available":true}""")
                "/api/projects/current/undo" -> TransportResponse(200, """{"project_revision":"revision-after-undo","post_apply_hash":"base","undo_available":false}""")
                else -> error("unexpected request: $method $path")
            }
        })

        val project = client.importProject("/fixture")
        val index = client.index()
        val file = client.fileInfo("main.go")
        val symbol = client.symbols("main.go").symbols.single()
        val summary = client.analysis("main.go", project.projectRevision)
        var state = DesktopState().reduce(DesktopEvent.ProjectLoaded(project, index)).reduce(DesktopEvent.FileLoaded(file, listOf(symbol))).reduce(DesktopEvent.AnalysisLoaded(summary))
        val preview = client.generate("fix: change Run", file.path, symbol.name, project.projectId, project.projectRevision, file.contentHash)
        state = state.reduce(DesktopEvent.CandidateLoaded(preview))
        val checks = client.checks(preview.generationId, preview.projectRevision)
        state = state.reduce(DesktopEvent.ChecksLoaded(checks))
        val applied = client.apply(preview.generationId, preview.projectId, preview.projectRevision, preview.baseFileHash)
        val undone = client.undo(preview.projectId, applied.projectRevision, applied.postApplyHash)

        assertEquals("fresh", state.analysis?.status)
        assertEquals("Run", state.candidate?.targetSymbol)
        assertTrue(state.candidate?.validation?.applicable == true)
        assertTrue(state.checks?.applicable == true)
        assertTrue(applied.undoAvailable)
        assertTrue(!undone.undoAvailable)
        assertTrue(requests.any { it.startsWith("POST /api/chat/message") && it.contains("file_path") && it.contains("main.go") && it.contains("target_symbol") && it.contains("Run") })
        assertTrue(requests.any { it.startsWith("POST /api/projects/current/apply") })
        assertTrue(requests.any { it.startsWith("POST /api/projects/current/undo") })
    }

    private companion object {
        const val projectJson = """{"project_id":"project","project_revision":"revision","name":"fixture","path":"/fixture","type":"go","file_count":1,"source_file_count":1,"total_lines":2,"analysis_file":".mini-orca/analysis.md","summary":"Fixture","ai_status":"fresh","analyzed_at":""}"""
        const val candidateJson = """{"generation_id":"generation","project_id":"project","project_revision":"revision","base_file_hash":"base","target_path":"main.go","target_symbol":"Run","scope_mode":"strict_symbol","candidate_content":"package main\\nfunc Run() { println(\\\"changed\\\") }","candidate_hash":"candidate","validation":{"applicable":true,"scope_mode":"strict_symbol","diff":{"old_path":"main.go","new_path":"main.go","lines":[{"kind":"removed","old_line":2,"text":"func Run() {}"},{"kind":"added","new_line":2,"text":"func Run() { println(\\\"changed\\\") }"}]}},"context_manifest":{"included":[{"path":"main.go","size_bytes":24,"hash":"base","estimated_tokens":6}]}}"""
    }
}
