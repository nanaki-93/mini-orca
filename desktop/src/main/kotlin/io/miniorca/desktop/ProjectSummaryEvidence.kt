package io.miniorca.desktop

/** A local destination identity, never a title, list position or synthesized finding ID. */
internal data class SummaryFindingTarget(
    val projectId: String,
    val projectRevision: String,
    val run: AnalysisRunIdentity?,
    val category: AnalysisResultType,
    val producer: SummaryFindingProducer,
    val rowKey: String,
)

internal sealed interface SummaryFindingProducer {
  data class Semantic(val path: String, val id: String, val source: String) :
      SummaryFindingProducer

  data class Verified(val path: String, val id: String, val source: String) :
      SummaryFindingProducer

  data class Performance(
      val path: String,
      val contentHash: String,
      val id: String,
      val model: String,
      val profile: String,
      val generatedAt: String,
  ) : SummaryFindingProducer

  data class Security(
      val source: String,
      val path: String,
      val contentHash: String,
      val id: String,
      val ruleSetVersion: String,
      val model: String,
      val profile: String,
      val generatedAt: String,
  ) : SummaryFindingProducer
}

internal data class SummaryFindingPreviewRow(
    val target: SummaryFindingTarget,
    val title: String,
    val location: String,
    val severity: String,
    val origin: String,
    val materialState: String,
)

internal data class SummaryFindingCategoryState(
    val category: AnalysisResultType,
    val status: String,
    val detail: String?,
    val loadedCount: Int,
)

internal data class SummaryFindingPreview(
    val rows: List<SummaryFindingPreviewRow>,
    val loadedCount: Int,
    val categories: List<SummaryFindingCategoryState>,
)

internal fun summaryFindingEmptyMessage(
    state: DesktopState,
    preview: SummaryFindingPreview
): String {
  if (preview.loadedCount > 0) return ""
  val pages = AnalysisResultType.entries.map { state.analysisResultPage(it.category) }
  if (pages.all {
    it.emptyPresentation(0).availability == AnalysisResultAvailability.CompletedEmpty
  })
      return "No findings in the completed analyzed scopes."
  return "No findings loaded for this selection. Reported counts are separate from loaded evidence."
}

/** Uses the same accepted rows as each destination; reported totals are not loaded row counts. */
internal fun summaryFindingPreview(state: DesktopState): SummaryFindingPreview {
  val pages = AnalysisResultType.entries.associateWith { state.analysisResultPage(it.category) }
  val sorted = summaryLoadedFindingRows(state, pages)
  return SummaryFindingPreview(
      sorted.take(5),
      sorted.size,
      AnalysisResultType.entries.map { type ->
        val page = pages.getValue(type)
        SummaryFindingCategoryState(
            type,
            page.statusLabel,
            when {
              page.section.error != null -> "Saved details unavailable · ${page.section.error}"
              page.section.loading -> "Loading saved details"
              page.results == null -> "Saved details not loaded"
              else -> null
            },
            sorted.count { it.target.category == type })
      })
}

/** Unbounded accepted rows, shared by the preview and activation against the latest snapshot. */
internal fun summaryLoadedFindingRows(state: DesktopState): List<SummaryFindingPreviewRow> =
    summaryLoadedFindingRows(
        state, AnalysisResultType.entries.associateWith { state.analysisResultPage(it.category) })

private fun summaryLoadedFindingRows(
    state: DesktopState,
    pages: Map<AnalysisResultType, AnalysisResultPageState>,
): List<SummaryFindingPreviewRow> {
  val bugs = pages.getValue(AnalysisResultType.Bugs)
  val rows = buildList {
    if (bugs.project != null) {
      val semanticKeys = bugs.semantic.map(::findingDisplayKey).toSet()
      state.projectBugFindings().forEach { finding ->
        val row = semanticResultRow(finding)
        val producer =
            if (findingDisplayKey(finding) in semanticKeys)
                SummaryFindingProducer.Semantic(finding.location.path, finding.id, finding.source)
            else SummaryFindingProducer.Verified(finding.location.path, finding.id, finding.source)
        add(
            previewRow(
                bugs,
                producer,
                row,
                findingEvidenceIdentity(finding),
                findingMaterialStateLabel(finding)))
      }
    }
    for (type in listOf(AnalysisResultType.Performance, AnalysisResultType.Security)) {
      val page = pages.getValue(type)
      (if (type == AnalysisResultType.Performance) performanceResults(page) else emptyList())
          .forEach { result ->
            val row = result.row()
            add(
                previewRow(
                    page,
                    SummaryFindingProducer.Performance(
                        result.report.path,
                        result.report.contentHash,
                        result.finding.id,
                        result.report.model,
                        result.report.profile,
                        result.report.generatedAt),
                    row,
                    "Model suggestion · unmeasured recommendation",
                    row.state,
                    result.report.status))
          }
      (if (type == AnalysisResultType.Security) securityResults(page) else emptyList()).forEach {
          result ->
        val row = result.row()
        add(
            previewRow(
                page,
                SummaryFindingProducer.Security(
                    result.report.source,
                    result.report.path,
                    result.report.contentHash,
                    result.finding.id,
                    result.report.ruleSetVersion,
                    result.report.model,
                    result.report.profile,
                    result.report.generatedAt),
                row,
                securityEvidencePresentation(result).label,
                row.state,
                result.report.status))
      }
      page.semantic.forEach { finding ->
        val row = semanticResultRow(finding)
        add(
            previewRow(
                page,
                SummaryFindingProducer.Semantic(finding.location.path, finding.id, finding.source),
                row,
                findingEvidenceIdentity(finding),
                row.state))
      }
    }
  }
  // Bugs is already deduplicated by its destination. Typed results retain destination duplicates:
  // the next stage must reject ambiguous row keys rather than invent an arbitrary winner.
  val sorted =
      rows.sortedWith(
          compareBy<SummaryFindingPreviewRow> { it.target.category.ordinal }
              .thenBy { it.location }
              .thenBy { it.target.rowKey }
              .thenBy { it.target.producer.toString() }
              .thenBy { it.title }
              .thenBy { it.severity }
              .thenBy { it.origin }
              .thenBy { it.materialState })
  return sorted
}

private fun previewRow(
    page: AnalysisResultPageState,
    producer: SummaryFindingProducer,
    row: ResultRowPresentation,
    origin: String,
    material: String,
    reportStatus: String? = null,
): SummaryFindingPreviewRow =
    SummaryFindingPreviewRow(
        SummaryFindingTarget(
            requireNotNull(page.project).projectId,
            page.project.projectRevision,
            page.run?.identity,
            page.type,
            producer,
            row.key),
        row.title,
        row.location,
        row.severity,
        origin,
        // Verified diagnostics have their own lifecycle; an unrelated analysis run cannot stale
        // them.
        if (producer is SummaryFindingProducer.Verified) material
        else materialState(page, reportStatus, material))

private fun materialState(page: AnalysisResultPageState, reportStatus: String?, rowState: String) =
    listOfNotNull(
            "Stale".takeIf { page.stale },
            "Saved details unavailable".takeIf { page.section.error != null },
            page.run
                ?.status
                ?.takeIf { it in setOf("partial", "failed", "canceled", "cancelled") }
                ?.let(::analysisStatusLabel),
            page.progress
                ?.status
                ?.takeIf { it in setOf("partial", "failed", "canceled", "cancelled") }
                ?.let(::analysisStatusLabel),
            reportStatus
                ?.takeIf { it in setOf("stale", "partial", "failed", "canceled", "cancelled") }
                ?.let(::analysisStatusLabel),
            rowState.takeIf { it.isNotBlank() })
        .flatMap { it.split(" · ") }
        .distinct()
        .joinToString(" · ")
