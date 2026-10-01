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
  @Test
  fun unifiedAnalysisUsesExplicitScopeLimitsAndFullRunGuardsWithoutFlatteningEvidence() {
    val requests = mutableListOf<Triple<String, String, String?>>()
    val run = analysisRunFixture()
    val api =
        ApiClient(
            transport =
                DaemonTransport { method, path, body ->
                  requests.add(Triple(method, path, body))
                  when {
                    path.endsWith("/preview") ->
                        TransportResponse(200, Json.encodeToString(analysisPreviewFixture()))
                    path.contains("/results?") ->
                        TransportResponse(
                            200,
                            Json.encodeToString(
                                analysisResultsFixture(run, "security", "dir/a b.go")))
                    else -> TransportResponse(202, Json.encodeToString(run))
                  }
                })
    val preview =
        api.previewAnalysis(
            AnalysisPreviewRequest(
                "project", "revision", "project", false, AnalysisRunLimits(100, 900, 2)))
    val request = requireNotNull(requests.last().third)
    assertContains(request, "\"scope\":\"project\"")
    assertContains(request, "\"batch_files\":100")
    assertContains(request, "\"budget_seconds\":900")
    assertContains(request, "\"max_attempts_per_stage\":2")
    assertContains(request, "\"refresh\":false")
    val confirmations = AnalysisRunConfirmations(listOf("bug-provider", "analyze-provider"), true)
    assertEquals(
        run,
        api.startAnalysis(
            AnalysisRunStartRequest(
                preview.identity,
                preview.previewId,
                preview.limits,
                preview.refresh,
                confirmations)))
    api.controlAnalysis(AnalysisRunControlRequest(run.identity, "pause"))
    val pause = requireNotNull(requests.last().third)
    assertContains(pause, "\"generation\":\"generation\"")
    assertFalse(pause.contains("confirmations"))
    api.controlAnalysis(
        AnalysisRunControlRequest(run.identity, "resume", preview.previewId, confirmations))
    assertContains(requireNotNull(requests.last().third), "\"security_review\":true")
    val results = api.analysisResults(run.identity, "security", "dir/a b.go")
    assertEquals("unverified", results.security.single().findings.single().verificationState)
    val path = requests.last().second
    for (part in
        listOf(
            "project_id=project",
            "project_revision=revision",
            "policy_fingerprint=policy",
            "provider_fingerprint=providers",
            "queue_id=queue",
            "id=run",
            "generation=generation",
            "category=security",
            "path=dir%2Fa+b.go")) assertContains(path, part)
  }

  @Test
  fun analysisPreviewAndCurrentRunDecodeProviderDurationStrings() {
    // Keep daemon JSON independent of the client serializer to detect wire-type regressions.
    val previewResponse =
        """{
          "schema_version":"1","preview_id":"preview",
          "identity":{"project_id":"project","project_revision":"revision",
            "policy_fingerprint":"policy","provider_fingerprint":"providers","queue_id":"queue"},
          "scope":"project","refresh":false,
          "limits":{"batch_files":100,"budget_seconds":900,"max_attempts_per_stage":2},
          "files":[],"excluded":[],
          "providers":[
            {"id":"bug-provider","stages":["semantic"],
              "model":{"scope":"bug","profile":"bug","model":"bug-model",
                "provider_origin":"https://bug.example","remote_provider":true,
                "temperature":0.2,"max_tokens":4096,"context_max_tokens":32000,
                "timeout":"2m0s","max_retries":3},
              "remote_confirmation_required":true},
            {"id":"analyze-provider","stages":["performance","security_ai"],
              "model":{"scope":"analyze","profile":"analyze","model":"review-model",
                "provider_origin":"http://127.0.0.1:1234","remote_provider":false,
                "temperature":0.2,"max_tokens":4096,"context_max_tokens":32000,
                "timeout":"1m30.5s","max_retries":3},
              "remote_confirmation_required":false}
          ],
          "expected_model_requests":0,"max_model_requests":0,"security_review_intent_required":false
        }"""
    val runResponse =
        """{
          "schema_version":"1",
          "identity":{"project_id":"project","project_revision":"revision",
            "policy_fingerprint":"policy","provider_fingerprint":"providers",
            "queue_id":"queue","id":"run","generation":"generation"},
          "plan":$previewResponse,"status":"paused","files":[],"sections":[]
        }"""
    val api =
        ApiClient(
            transport =
                DaemonTransport { method, path, _ ->
                  when (method to path) {
                    "POST" to "/api/projects/current/analysis/preview" ->
                        TransportResponse(200, previewResponse)
                    "GET" to
                        "/api/projects/current/analysis/run?project_id=project&project_revision=revision" ->
                        TransportResponse(200, runResponse)
                    else -> error("unexpected $method $path")
                  }
                })

    val preview =
        api.previewAnalysis(
            AnalysisPreviewRequest(
                "project", "revision", "project", false, AnalysisRunLimits(100, 900, 2)))
    assertEquals(listOf("2m0s", "1m30.5s"), preview.providers.map { it.model.timeout })
    assertEquals(listOf(true, false), preview.providers.map { it.remoteConfirmationRequired })
    assertEquals(preview, api.analysisRun("project", "revision")?.plan)
  }

  @Test
  fun unifiedCurrentReadHandlesAbsentAndInterruptedRuns() {
    val api =
        ApiClient(
            transport =
                DaemonTransport { _, path, _ ->
                  if (path.contains("project_id=empty")) TransportResponse(204, "")
                  else
                      TransportResponse(
                          200,
                          Json.encodeToString(analysisRunFixture().copy(status = "interrupted")))
                })
    assertNull(api.analysisRun("empty", "revision"))
    assertEquals("interrupted", api.analysisRun("project", "revision")?.status)
    val section =
        Json.decodeFromString<AnalysisSectionProgress>(
            """{"category":"bugs","status":"running","coverage":{"total":1,"running":1},"finding_count":null}""")
    assertNull(section.findingCount)
  }

  @Test
  fun benchmarkCatalogIsReadOnlyAndComparisonSendsTheExactCatalogChoice() {
    val client =
        ApiClient(
            transport =
                DaemonTransport { method, path, body ->
                  when (method to path) {
                    "GET" to
                        "/api/projects/current/drafts/draft-1/benchmarks?project_revision=revision&expected_revision=2&expected_hash=candidate-hash" ->
                        TransportResponse(
                            200,
                            """{"draft_id":"draft-1","draft_revision":2,"draft_hash":"candidate-hash","available":true,"trusted":false,"benchmarks":[{"name":"BenchmarkRun","command":["go","test","-run","^$","-bench","^BenchmarkRun$","-benchmem"],"scope":"scope-1"}]}""",
                        )
                    "POST" to "/api/projects/current/drafts/draft-1/benchmarks" -> {
                      assertContains(body.orEmpty(), "\"project_revision\":\"revision\"")
                      assertContains(body.orEmpty(), "\"expected_revision\":2")
                      assertContains(body.orEmpty(), "\"expected_hash\":\"candidate-hash\"")
                      assertContains(body.orEmpty(), "\"benchmark\":\"BenchmarkRun\"")
                      assertContains(body.orEmpty(), "\"expected_scope\":\"scope-1\"")
                      TransportResponse(
                          200,
                          """{"draft_id":"draft-1","draft_revision":2,"draft_hash":"candidate-hash","status":"unavailable","reason":"execution trust is required"}""",
                      )
                    }
                    else -> error("unexpected request: $method $path")
                  }
                })

    val catalog = client.goBenchmarkCatalog("draft-1", "revision", 2, "candidate-hash")
    assertTrue(catalog.available)
    assertFalse(catalog.trusted)
    val comparison =
        client.compareGoBenchmark(
            "draft-1", "revision", 2, "candidate-hash", catalog.benchmarks.single())
    assertEquals("unavailable", comparison.status)
  }

  @Test
  fun benchmarkComparisonDecodingPreservesOmittedNullAndZeroMemoryMetrics() {
    val response =
        """{
      "status":"completed","benchmark":"BenchmarkRun",
      "base":{"samples":[
        {"iterations":1,"ns_per_op":100},
        {"iterations":2,"ns_per_op":101,"bytes_per_op":null,"allocs_per_op":null},
        {"iterations":3,"ns_per_op":102,"bytes_per_op":0,"allocs_per_op":0},
        {"iterations":4,"ns_per_op":103,"bytes_per_op":4,"allocs_per_op":2}]},
      "candidate":{"samples":[
        {"iterations":5,"ns_per_op":90,"bytes_per_op":null,"allocs_per_op":0},
        {"iterations":6,"ns_per_op":91,"bytes_per_op":0,"allocs_per_op":null},
        {"iterations":7,"ns_per_op":92,"allocs_per_op":1},
        {"iterations":8,"ns_per_op":93,"bytes_per_op":2}]}
    }"""
    val client =
        ApiClient(
            transport =
                DaemonTransport { method, path, _ ->
                  assertEquals("POST", method)
                  assertEquals("/api/projects/current/drafts/draft-1/benchmarks", path)
                  TransportResponse(200, response)
                })
    val comparison =
        client.compareGoBenchmark(
            "draft-1",
            "revision",
            2,
            "candidate-hash",
            GoBenchmarkChoice("BenchmarkRun", scope = "scope"))
    val base = requireNotNull(comparison.base).samples
    val candidate = requireNotNull(comparison.candidate).samples
    assertEquals(listOf(null, null, 0L, 4L), base.map { it.bytesPerOperation })
    assertEquals(listOf(null, null, 0L, 2L), base.map { it.allocationsPerOperation })
    assertEquals(listOf(null, 0L, null, 2L), candidate.map { it.bytesPerOperation })
    assertEquals(listOf(0L, null, 1L, null), candidate.map { it.allocationsPerOperation })
    assertEquals(listOf(1L, 2L, 3L, 4L), base.map { it.iterations })
    assertEquals(listOf(90.0, 91.0, 92.0, 93.0), candidate.map { it.nanosecondsPerOperation })
  }

  @Test
  fun benchmarkComparisonDecodingKeepsMissingMeasurementsSeparateFromEmptySamples() {
    for (response in
        listOf(
            """{"status":"completed","candidate":{"samples":[]}}""",
            """{"status":"completed","base":null,"candidate":{}}""")) {
      val client =
          ApiClient(transport = DaemonTransport { _, _, _ -> TransportResponse(200, response) })
      val comparison =
          client.compareGoBenchmark(
              "draft-1",
              "revision",
              2,
              "candidate-hash",
              GoBenchmarkChoice("BenchmarkRun", scope = "scope"))
      assertNull(comparison.base)
      assertEquals(GoBenchmarkMeasurement(emptyList()), comparison.candidate)
    }
  }

  @Test
  fun discoveryDecodesSparseUnavailabilityAndAvailableEmptyWithoutOtherRequests() {
    val requests = mutableListOf<Triple<String, String, String?>>()
    var response = """{"available":false,"reason":"Candidate is invalid"}"""
    val client =
        ApiClient(
            transport =
                DaemonTransport { method, path, body ->
                  requests.add(Triple(method, path, body))
                  TransportResponse(200, response)
                })
    val unavailable = client.goBenchmarkCatalog("draft-1", "revision", 2, "candidate-hash")
    assertEquals(GoBenchmarkCatalog(reason = "Candidate is invalid"), unavailable)
    response =
        """{"draft_id":"draft-1","draft_revision":2,"draft_hash":"candidate-hash","available":true,"benchmarks":[]}"""
    val empty = client.goBenchmarkCatalog("draft-1", "revision", 2, "candidate-hash")
    assertTrue(empty.available)
    assertTrue(empty.benchmarks.isEmpty())
    assertEquals(
        List(2) {
          Triple<String, String, String?>(
              "GET",
              "/api/projects/current/drafts/draft-1/benchmarks?project_revision=revision&expected_revision=2&expected_hash=candidate-hash",
              null)
        },
        requests)
  }

  @Test
  fun executionTrustUsesASeparateCurrentSessionConsentContract() {
    val client =
        ApiClient(
            transport =
                DaemonTransport { method, path, body ->
                  when (method to path) {
                    "GET" to "/api/projects/current/execution-trust?project_revision=revision" ->
                        TransportResponse(
                            200,
                            """{"project_id":"project","project_revision":"revision","trusted":false,"commands":[["go","test","./..."]]}""")
                    "POST" to "/api/projects/current/execution-trust" -> {
                      assertContains(body.orEmpty(), "\"project_revision\":\"revision\"")
                      assertContains(body.orEmpty(), "\"confirm\":true")
                      TransportResponse(
                          200,
                          """{"project_id":"project","project_revision":"revision","trusted":true,"commands":[["go","test","./..."]]}""")
                    }
                    else -> error("unexpected request: $method $path")
                  }
                })

    assertEquals(
        ExecutionTrust("project", "revision", false, listOf(listOf("go", "test", "./..."))),
        client.executionTrust("revision"))
    assertEquals(
        ExecutionTrust("project", "revision", true, listOf(listOf("go", "test", "./..."))),
        client.trustProjectExecution("revision"))
  }

  @Test
  fun trustPostPreservesIncompleteOrRejectedAcknowledgmentsForAdmissionValidation() {
    listOf(
            """{}""" to ExecutionTrust(),
            """{"project_id":"foreign","project_revision":"other","commands":[["go","test","."]]}""" to
                ExecutionTrust("foreign", "other", false, listOf(listOf("go", "test", "."))))
        .forEach { (response, expected) ->
          val requests = mutableListOf<Triple<String, String, String?>>()
          val client =
              ApiClient(
                  transport =
                      DaemonTransport { method, path, body ->
                        requests.add(Triple(method, path, body))
                        TransportResponse(200, response)
                      })
          assertEquals(expected, client.trustProjectExecution("revision"))
          assertEquals(
              listOf(
                  Triple<String, String, String?>(
                      "POST",
                      "/api/projects/current/execution-trust",
                      """{"project_revision":"revision","confirm":true}""")),
              requests)
        }
  }

  @Test
  fun projectRestoreUsesTheNonAnalyzingRestoreEndpoint() {
    val client =
        ApiClient(
            transport =
                DaemonTransport { method, path, body ->
                  assertEquals("POST", method)
                  assertEquals("/api/projects/restore", path)
                  assertContains(body.orEmpty(), "\"project_path\":\"/tmp/fixture\"")
                  assertFalse(body.orEmpty().contains("confirm_remote_provider"))
                  TransportResponse(200, projectResponse())
                })

    assertEquals("/tmp/fixture", client.restoreProject("/tmp/fixture").path)
  }

  @Test
  fun importSendsAnalyzeConfirmationWhileRestoreRemainsModelFree() {
    val client =
        ApiClient(
            transport =
                DaemonTransport { method, path, body ->
                  assertEquals("POST", method)
                  assertEquals("/api/projects/import", path)
                  assertContains(body.orEmpty(), "\"project_path\":\"/tmp/fixture\"")
                  assertContains(body.orEmpty(), "\"confirm_remote_provider\":true")
                  TransportResponse(200, projectResponse())
                })

    assertEquals("fixture", client.importProject("/tmp/fixture", confirmRemoteProvider = true).name)
  }

  @Test
  fun modelCatalogDecodesScopedResponses() {
    val scopedClient =
        ApiClient(
            transport =
                DaemonTransport { method, path, _ ->
                  assertEquals("GET", method)
                  assertEquals("/api/models/current", path)
                  TransportResponse(
                      200,
                      """{"scopes":{"analyze":{"scope":"analyze","profile":"analyze","model":"cloud-analyze","remote_provider":true},"bug":{"scope":"bug","profile":"bug","model":"cloud-bug","remote_provider":true},"function":{"scope":"function","profile":"function","model":"local-code","remote_provider":false}}}""")
                })

    val scoped = scopedClient.modelCatalog()
    assertEquals("cloud-analyze", scoped.forScope(ModelScope.Analyze).model)
    assertTrue(scoped.forScope(ModelScope.Bug).remoteProvider)
    assertFalse(scoped.forScope(ModelScope.Function).remoteProvider)
  }

  @Test
  fun fileAnalysisSendsRemoteProviderConfirmation() {
    val client =
        ApiClient(
            transport =
                DaemonTransport { method, path, body ->
                  assertEquals("POST", method)
                  assertEquals("/api/projects/current/files/analysis", path)
                  assertContains(body.orEmpty(), "\"path\":\"main.go\"")
                  assertContains(body.orEmpty(), "\"project_revision\":\"revision\"")
                  assertContains(body.orEmpty(), "\"confirm_remote_provider\":true")
                  TransportResponse(200, """{"path":"main.go","status":"fresh"}""")
                })

    assertEquals(
        "fresh", client.analyze("main.go", "revision", confirmRemoteProvider = true).status)
  }

  @Test
  fun declarationExplanationSendsExactIdentityAndFunctionConsent() {
    val client =
        ApiClient(
            transport =
                DaemonTransport { method, path, body ->
                  assertEquals("POST", method)
                  assertEquals("/api/projects/current/files/explanation", path)
                  assertContains(body.orEmpty(), "\"project_id\":\"project\"")
                  assertContains(body.orEmpty(), "\"project_revision\":\"revision\"")
                  assertContains(body.orEmpty(), "\"base_file_hash\":\"hash\"")
                  assertContains(body.orEmpty(), "\"target_path\":\"main.go\"")
                  assertContains(body.orEmpty(), "\"target_symbol\":\"Run\"")
                  assertContains(body.orEmpty(), "\"confirm_remote_provider\":true")
                  TransportResponse(
                      200,
                      """{"version":"v1","project_id":"project","project_revision":"revision","base_file_hash":"hash","anchor":{"path":"main.go","symbol":"Run","signature":"func Run()","start_line":2,"end_line":3},"summary":"Runs.","behavior":[],"inputs":[],"outputs":[],"side_effects":[],"error_behavior":[],"context_manifest":{}}""")
                })

    assertEquals(
        "Run",
        client
            .explainDeclaration("project", "revision", "hash", "main.go", "Run", true)
            .anchor
            .symbol)
  }

  @Test
  fun declarationExplanationRequiresTheDaemonSourceSignature() {
    val client =
        ApiClient(
            transport =
                DaemonTransport { _, _, _ ->
                  TransportResponse(
                      200,
                      """{"version":"v1","project_id":"project","project_revision":"revision","base_file_hash":"hash","anchor":{"path":"main.go","symbol":"Run","start_line":2,"end_line":3},"summary":"Runs.","behavior":[],"inputs":[],"outputs":[],"side_effects":[],"error_behavior":[],"context_manifest":{}}""")
                })

    val failure =
        runCatching {
              client.explainDeclaration("project", "revision", "hash", "main.go", "Run", false)
            }
            .exceptionOrNull()

    assertTrue(failure != null)
  }

  @Test
  fun securityScanAndReviewUseSeparateExplicitContracts() {
    val client =
        ApiClient(
            transport =
                DaemonTransport { method, path, body ->
                  when (method to path) {
                    "POST" to "/api/projects/current/files/security-scan" -> {
                      assertContains(body.orEmpty(), "\"path\":\"main.go\"")
                      assertContains(body.orEmpty(), "\"project_revision\":\"revision\"")
                      assertFalse(body.orEmpty().contains("confirm_remote_provider"))
                      TransportResponse(200, securityReportJson("deterministic"))
                    }
                    "POST" to "/api/projects/current/security-review" -> {
                      assertContains(body.orEmpty(), "\"project_id\":\"project\"")
                      assertContains(body.orEmpty(), "\"base_file_hash\":\"hash\"")
                      assertContains(body.orEmpty(), "\"symbol\":\"Run\"")
                      assertContains(body.orEmpty(), "\"confirm_remote_provider\":true")
                      TransportResponse(200, securityReportJson("ai"))
                    }
                    else -> error("unexpected request: $method $path")
                  }
                })

    assertEquals("deterministic", client.securityScan("main.go", "revision").source)
    assertEquals(
        "ai", client.securityReview("project", "revision", "hash", "main.go", "Run", true).source)
  }

  @Test
  fun workspaceResponsesDecodeToTypedModelsAndIgnoreUnknownFields() {
    val client =
        ApiClient(
            transport =
                DaemonTransport { method, path, _ ->
                  when (method to path) {
                    "GET" to "/api/projects/current/overview?project_revision=revision" ->
                        TransportResponse(
                            200,
                            """{"project_id":"project","project_revision":"revision","metrics":{"type":"go","file_count":3,"unknown_metric":"ignored"},"analysis":{"purpose":"Builds the daemon","components":null,"future_field":"ignored"},"analysis_coverage":{"total":4,"fresh":2,"partial":1,"unavailable":1},"finding_counts":{"verified":1,"ai_suggestions":2},"source":"must not become a model field"}""",
                        )
                    "GET" to "/api/projects/current/findings?project_revision=revision" ->
                        TransportResponse(
                            200,
                            """{"project_id":"project","project_revision":"revision","findings":[{"id":"finding-1","source":"ai","confidence":"suggested","severity":"warning","title":"Task issue","message":"check this","location":{"path":"main.go","start_line":7,"symbol":"Run"},"task_spec":{"schema_version":"1","target_path":"main.go","target_symbol":"Run","target_signature":"func Run()","acceptance_criteria":["Return errors."],"non_goals":[]},"unknown":"ignored"}]}""",
                        )
                    else -> error("unexpected request: $method $path")
                  }
                })

    val overview = client.overview("revision")
    val findings = client.findings("revision")

    assertEquals("go", overview.metrics.type)
    assertEquals(emptyList(), overview.analysis.components)
    assertEquals(1, overview.analysisCoverage.partial)
    assertEquals(1, overview.analysisCoverage.unavailable)
    assertEquals(2, overview.findingCounts.aiSuggestions)
    assertEquals("main.go", findings.findings.single().location.path)
    assertEquals(7, findings.findings.single().location.startLine)
    assertEquals("Run", findings.findings.single().taskSpec?.targetSymbol)
    assertFalse(Json.encodeToString(overview).contains("must not become a model field"))
  }

  @Test
  fun noContentProgressAndTriageResponsesDoNotDecodeEmptyBodies() {
    val client =
        ApiClient(
            transport =
                DaemonTransport { method, path, body ->
                  when (method to path) {
                    "GET" to "/api/projects/current/scan?project_revision=revision" ->
                        TransportResponse(204, "")
                    "GET" to "/api/projects/current/analysis-job?project_revision=revision" ->
                        TransportResponse(204, "")
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

  @Test
  fun scanAndAnalyzeAllActionsUseTypedProgressContracts() {
    val client =
        ApiClient(
            transport =
                DaemonTransport { method, path, body ->
                  when (method to path) {
                    "POST" to "/api/projects/current/scan" -> {
                      assertContains(body.orEmpty(), "\"project_revision\":\"revision\"")
                      TransportResponse(
                          200,
                          """{"project_id":"project","project_revision":"revision","status":"running","phases":null}""")
                    }
                    "DELETE" to "/api/projects/current/scan?project_revision=revision" ->
                        TransportResponse(
                            200,
                            """{"project_id":"project","project_revision":"revision","status":"canceled","phases":[]}""")
                    "POST" to "/api/projects/current/analysis-job" -> {
                      assertContains(body.orEmpty(), "\"max_files\":10")
                      assertContains(body.orEmpty(), "\"max_retries\":2")
                      assertContains(body.orEmpty(), "\"confirm_remote_provider\":true")
                      TransportResponse(200, jobResponse("running"))
                    }
                    "POST" to
                        "/api/projects/current/analysis-job/pause?project_revision=revision" -> {
                      assertNull(body)
                      TransportResponse(200, jobResponse("paused"))
                    }
                    "POST" to "/api/projects/current/analysis-job/resume" -> {
                      assertContains(body.orEmpty(), "\"confirm_remote_provider\":true")
                      TransportResponse(200, jobResponse("running"))
                    }
                    "POST" to
                        "/api/projects/current/analysis-job/cancel?project_revision=revision" -> {
                      assertNull(body)
                      TransportResponse(200, jobResponse("canceled"))
                    }
                    else -> error("unexpected request: $method $path")
                  }
                })

    assertEquals("running", client.startGoScan("revision").status)
    assertEquals("canceled", client.cancelGoScan("revision").status)
    assertEquals(
        "running",
        client
            .startAnalyzeAll(
                "revision", maxFiles = 10, maxRetries = 2, confirmRemoteProvider = true)
            .status)
    assertEquals("paused", client.pauseAnalyzeAll("revision").status)
    assertEquals(
        "running", client.resumeAnalyzeAll("revision", confirmRemoteProvider = true).status)
    assertEquals("canceled", client.cancelAnalyzeAll("revision").status)
  }

  @Test
  fun chatAndDraftActionsUseBoundTypedContracts() {
    val client =
        ApiClient(
            transport =
                DaemonTransport { method, path, body ->
                  when (method to path) {
                    "POST" to "/api/projects/current/chat/sessions" -> {
                      assertContains(body.orEmpty(), "\"mode\":\"replace_symbol\"")
                      assertContains(body.orEmpty(), "\"target_symbol\":\"Run\"")
                      TransportResponse(
                          201,
                          """{"id":"session","project_id":"project","project_revision":"revision","base_file_hash":"base","open_path":"main.go","mode":"replace_symbol","target_symbol":"Run","state":"active","messages":null}""")
                    }
                    "POST" to "/api/projects/current/chat/sessions/session/messages" -> {
                      assertContains(body.orEmpty(), "\"parent_draft_id\":\"older\"")
                      assertContains(body.orEmpty(), "\"confirm_remote_provider\":true")
                      TransportResponse(
                          200,
                          """{"session_id":"session","draft":{"id":"draft","project_id":"project","project_revision":"revision","base_file_hash":"base","target_path":"main.go","mode":"replace_symbol","target_symbol":"Run","declaration":"func Run() {}","revision":1,"hash":"hash","state":"generated"},"assistant_message":{"role":"assistant","content":"Proposal ready"},"context_manifest":{"included":null}}""")
                    }
                    "PATCH" to "/api/projects/current/drafts/draft" -> {
                      assertContains(body.orEmpty(), "\"expected_revision\":1")
                      assertContains(body.orEmpty(), "\"imports\":[\"fmt\"]")
                      TransportResponse(200, draftResponse())
                    }
                    "POST" to "/api/projects/current/drafts/draft/validate" ->
                        TransportResponse(200, draftResponse())
                    "POST" to "/api/projects/current/drafts/draft/checks" -> {
                      assertContains(body.orEmpty(), "\"expected_hash\":\"hash\"")
                      TransportResponse(
                          200,
                          """{"target_path":"main.go","applicable":true,"checks":null,"draft_id":"draft","draft_revision":1,"draft_hash":"hash"}""")
                    }
                    "POST" to "/api/projects/current/apply" -> {
                      assertContains(body.orEmpty(), "\"draft_id\":\"draft\"")
                      assertContains(body.orEmpty(), "\"confirm\":true")
                      TransportResponse(
                          200,
                          """{"project_revision":"next","post_apply_hash":"after","undo_available":true}""")
                    }
                    else -> error("unexpected request: $method $path")
                  }
                })

    val session =
        client.openChatSession("project", "revision", "base", "main.go", "replace_symbol", "Run")
    val proposal =
        client.sendChatMessage(session.id, "Improve Run", "older", confirmRemoteProvider = true)
    val updated = client.updateDraft("draft", "revision", 1, "func Run() {}", listOf("fmt"))
    val validated = client.validateDraft("draft", "revision", updated.revision)
    val checks = client.checkDraft("draft", "revision", validated.revision, validated.hash)
    val applied = client.applyDraft(validated)

    assertEquals(emptyList(), session.messages)
    assertEquals("draft", proposal.draft.id)
    assertEquals(emptyList(), proposal.contextManifest.included)
    assertEquals("draft", checks.draftId)
    assertEquals("next", applied.projectRevision)
  }

  @Test
  fun taskBoundChatAndExplicitRepairUseTypedContracts() {
    val task =
        BugTaskSpec(
            "1",
            "main.go",
            "Run",
            "func Run()",
            listOf("Return an error."),
            listOf("Do not edit callers."))
    val client =
        ApiClient(
            transport =
                DaemonTransport { method, path, body ->
                  when (method to path) {
                    "POST" to "/api/projects/current/chat/sessions" -> {
                      assertContains(body.orEmpty(), "\"task_spec\":{\"schema_version\":\"1\"")
                      assertContains(
                          body.orEmpty(), "\"acceptance_criteria\":[\"Return an error.\"]")
                      TransportResponse(
                          201,
                          """{"id":"session","project_id":"project","project_revision":"revision","base_file_hash":"base","open_path":"main.go","mode":"replace_symbol","target_symbol":"Run","state":"active","task_spec":{"schema_version":"1","target_path":"main.go","target_symbol":"Run","target_signature":"func Run()","acceptance_criteria":["Return an error."],"non_goals":["Do not edit callers."]}}""")
                    }
                    "POST" to "/api/projects/current/chat/sessions/session/messages" -> {
                      assertContains(body.orEmpty(), "\"repair\":true")
                      TransportResponse(
                          200,
                          """{"session_id":"session","draft":{"id":"draft","project_id":"project","project_revision":"revision","base_file_hash":"base","target_path":"main.go","mode":"replace_symbol","target_symbol":"Run","declaration":"func Run() {}","revision":1,"hash":"hash","state":"generated","task_spec":{"schema_version":"1","target_path":"main.go","target_symbol":"Run","target_signature":"func Run()","acceptance_criteria":["Return an error."],"non_goals":["Do not edit callers."]}},"assistant_message":{"role":"assistant","content":"Proposal ready"}}""")
                    }
                    else -> error("unexpected request: $method $path")
                  }
                })

    val session =
        client.openChatSession(
            "project", "revision", "base", "main.go", "replace_symbol", "Run", task)
    val proposal =
        client.sendChatMessage(
            session.id, "Use check evidence", parentDraftId = "prior", repair = true)

    assertEquals(task, session.taskSpec)
    assertEquals(task, proposal.draft.taskSpec)
  }

  @Test
  fun contextPreviewKeepsReportedMetadataAndUsesOnlyTheLocalRoute() {
    val requests = mutableListOf<String>()
    val responses =
        listOf(
            """{"included":[{"path":"dir/a b.go","size_bytes":12,"hash":"sha256:a","estimated_tokens":0,"truncated":false},{"path":"dir/b.go","size_bytes":90,"hash":"sha256:b","estimated_tokens":8,"truncated":true}],"excluded":[{"path":".env","include":false,"reason":"secret"}],"estimated_tokens":0,"byte_limit":2048,"token_limit":256,"truncated":false,"scope":"function","model":"m","provider_origin":"https://example.test","remote_provider":true}""",
            """{"included":[{"path":"dir/a b.go","size_bytes":12,"hash":"sha256:a"}],"excluded":[]}""",
            """{"included":[],"excluded":[],"estimated_tokens":0,"byte_limit":0,"token_limit":0,"truncated":false,"remote_provider":false}""")
    var next = 0
    val client =
        ApiClient(
            transport =
                DaemonTransport { method, path, body ->
                  assertEquals("GET", method)
                  assertNull(body)
                  requests.add(path)
                  TransportResponse(200, responses[next++])
                })

    val complete = client.context("dir/a b.go")
    assertEquals(0, complete.estimatedTokens)
    assertEquals(2048, complete.byteLimit)
    assertEquals(256, complete.tokenLimit)
    assertEquals(false, complete.truncated)
    assertEquals(true, complete.remoteProvider)
    assertEquals(listOf(false, true), complete.included.map { it.truncated })
    assertEquals(listOf(0, 8), complete.included.map { it.estimatedTokens })
    assertEquals("sha256:a", complete.included.first().hash)
    assertEquals("secret", complete.excluded.single().reason)

    val omitted = client.context("dir/a b.go", "analyze_file")
    assertNull(omitted.estimatedTokens)
    assertNull(omitted.byteLimit)
    assertNull(omitted.tokenLimit)
    assertNull(omitted.truncated)
    assertNull(omitted.remoteProvider)
    assertNull(omitted.included.single().estimatedTokens)
    assertNull(omitted.included.single().truncated)

    val zero = client.context("dir/a b.go")
    assertEquals(0, zero.estimatedTokens)
    assertEquals(0, zero.byteLimit)
    assertEquals(0, zero.tokenLimit)
    assertEquals(false, zero.truncated)
    assertEquals(false, zero.remoteProvider)
    assertEquals(
        listOf(
            "/api/projects/current/context?path=dir%2Fa+b.go&action=fix",
            "/api/projects/current/context?path=dir%2Fa+b.go&action=analyze_file",
            "/api/projects/current/context?path=dir%2Fa+b.go&action=fix"),
        requests)
  }

  @Test
  fun contextPreviewPreservesStructuredFailures() {
    val client =
        ApiClient(
            transport =
                DaemonTransport { method, path, body ->
                  assertEquals("GET", method)
                  assertEquals("/api/projects/current/context?path=main.go&action=fix", path)
                  assertNull(body)
                  TransportResponse(
                      409,
                      """{"type":"conflict","message":"stale revision","user_message":"Reload the project first."}""")
                })
    val failure = runCatching { client.context("main.go") }.exceptionOrNull()
    assertTrue(failure is ApiException)
    assertEquals(409, failure.status)
    assertEquals("conflict", failure.error?.type)
    assertEquals("Reload the project first.", failure.message)
  }

  @Test
  fun structuredFailuresExposeTheDaemonError() {
    val client =
        ApiClient(
            transport =
                DaemonTransport { _, _, _ ->
                  TransportResponse(
                      409,
                      """{"type":"conflict","message":"stale revision","user_message":"Reload the project first."}""")
                })

    val failure = runCatching { client.startGoScan("old-revision") }.exceptionOrNull()

    assertTrue(failure is ApiException)
    assertEquals(409, failure.status)
    assertEquals("conflict", failure.error?.type)
    assertEquals("Reload the project first.", failure.message)
  }

  private fun draftResponse() =
      """{"id":"draft","project_id":"project","project_revision":"revision","base_file_hash":"base","target_path":"main.go","mode":"replace_symbol","target_symbol":"Run","declaration":"func Run() {}","imports":null,"revision":1,"hash":"hash","state":"valid","validation":{"applicable":true,"scope_mode":"replace_symbol","diagnostics":null,"diff":{"old_path":"main.go","new_path":"main.go","lines":null}}}"""

  private fun securityReportJson(source: String) =
      """{"schema_version":"1","project_id":"project","project_revision":"revision","path":"main.go","content_hash":"hash","status":"completed_empty","source":"$source","findings":[],"context_policy_version":"policy","generated_at":"2026-09-08T00:00:00Z"}"""

  private fun jobResponse(status: String) =
      """{"project_id":"project","project_revision":"revision","status":"$status","max_files":10,"max_retries":2,"files":null}"""

  private fun projectResponse() =
      """{"project_id":"project","project_revision":"revision","name":"fixture","path":"/tmp/fixture","type":"go","file_count":1,"source_file_count":1,"total_lines":1,"summary":"Fixture","ai_status":"fresh","analyzed_at":"2026-09-02T00:00:00Z"}"""
}
