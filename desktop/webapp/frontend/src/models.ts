export interface ProjectIdentity {
  project_id: string;
  project_revision: string;
}
export interface Project extends ProjectIdentity {
  name: string;
  path: string;
  type: string;
  build_file: string;
  file_count: number;
  source_file_count: number;
  total_lines: number;
  languages: Record<string, number>;
  files: string[];
  summary: string;
  ai_status: string;
  analyzed_at: string;
}
export interface SymbolInfo {
  name: string;
  kind: string;
  signature: string;
  start_line: number;
  end_line: number;
  confidence: string;
  atomic_target: boolean;
}
export interface IndexedFile {
  path: string;
  content_hash: string;
  language: string;
  binary: boolean;
  size_bytes: number;
  line_count: number;
  imports: string[];
  symbols: SymbolInfo[];
  analysis_status: string;
}
export interface ProjectIndex extends ProjectIdentity {
  files: IndexedFile[];
}
export interface FileInfo {
  path: string;
  content_hash: string;
  name: string;
  language: string;
  size_bytes: number;
  line_count: number;
  binary: boolean;
  content: string;
}
export interface Insight {
  mechanism?: string;
  why_it_matters_here?: string;
  tradeoff_or_failure_mode?: string;
  transferable_lesson?: string;
}
export interface Overview extends ProjectIdentity {
  metrics: {
    type: string;
    file_count: number;
    source_file_count: number;
    total_lines: number;
    languages: Record<string, number>;
  };
  analysis: {
    purpose: string;
    architecture: string;
    components: string[];
    entry_points: string[];
    flows: string[];
    risks: { severity: string; summary: string; engineering_insight?: Insight }[];
    next_steps: string[];
    engineering_insight?: Insight;
    status: string;
    failure?: string;
  };
  analysis_coverage: Record<string, number>;
  finding_counts: { verified: number; ai_suggestions: number };
  analysis_run?: AnalysisRun;
}
export interface Model {
  scope: string;
  profile: string;
  model: string;
  provider_origin: string;
  remote_provider: boolean;
  reasoning_effort: string;
  timeout: string;
}
export interface ModelCatalog {
  scopes: Record<string, Model>;
}
export interface AnalysisModels {
  code: string;
  performance?: string;
  security?: string;
  review?: string;
  features: string;
}
export interface ModelChoice {
  id: string;
  name: string;
  provider: string;
  source: 'configured' | 'pi';
  location: 'local' | 'remote' | 'unknown';
  model: Model;
}
export interface AvailableModels {
  models: ModelChoice[];
  defaults: Record<string, string>;
  pi: { status: 'ready' | 'unavailable'; message?: string };
}
export interface TaskSpec {
  schema_version: string;
  target_path: string;
  target_symbol: string;
  target_signature: string;
  acceptance_criteria: string[];
  non_goals: string[];
  go_test_candidate?: { name: string; content: string };
}
export interface Finding extends ProjectIdentity {
  id: string;
  source: string;
  confidence: string;
  severity: string;
  title: string;
  message: string;
  rule: string;
  file_hash: string;
  location: { path: string; symbol: string; start_line: number; end_line: number };
  evidence: string;
  status: string;
  freshness: string;
  category: string;
  task_spec?: TaskSpec;
  engineering_insight?: Insight;
}
export interface FindingsResponse extends ProjectIdentity {
  findings: Finding[];
}
export interface Limits {
  batch_files: number;
  budget_seconds: number;
  max_attempts_per_stage: number;
}
export interface QueueIdentity extends ProjectIdentity {
  policy_fingerprint: string;
  provider_fingerprint: string;
  queue_id: string;
}
export interface RunIdentity extends QueueIdentity {
  id: string;
  generation: string;
}
export interface PlannedFile {
  path: string;
  content_hash: string;
  language: string;
  size_bytes: number;
  stages: {
    stage: string;
    eligible: boolean;
    cached: boolean;
    reason?: string;
    max_model_requests: number;
    provider_id?: string;
  }[];
}
export interface AnalysisPreview {
  models?: AnalysisModels;
  features?: {
    expected_hash: string;
    goals_hash: string;
    workspace_hash: string;
    excluded_paths: string[];
    provider_id: string;
    max_model_requests: number;
    reason?: string;
  };
  preview_id: string;
  identity: QueueIdentity;
  scope: string;
  refresh: boolean;
  limits: Limits;
  files: PlannedFile[];
  excluded: { path: string; reason: string }[];
  providers: {
    id: string;
    stages: string[];
    model: Model;
    remote_confirmation_required: boolean;
  }[];
  expected_model_requests: number;
  max_model_requests: number;
  security_review_intent_required: boolean;
  retry_stale_failed?: boolean;
  stale_only?: boolean;
  stale_path?: string;
  recover_incomplete?: boolean;
}
export interface Coverage {
  total: number;
  pending: number;
  running: number;
  succeeded: number;
  partial: number;
  failed: number;
  skipped: number;
  unavailable: number;
}
export interface Section {
  category: string;
  status: string;
  coverage: Coverage;
  finding_count: number | null;
}
export interface RunFile {
  path: string;
  content_hash: string;
  language: string;
  stages: {
    stage: string;
    status: string;
    attempts: number;
    cached: boolean;
    finding_count: number | null;
    reason?: string;
  }[];
}
export interface AnalysisRun {
  features?: {
    status: string;
    attempts: number;
    suggestion_count: number | null;
    report_hash?: string;
    reason?: string;
  };
  identity: RunIdentity;
  plan: AnalysisPreview;
  status: string;
  files: RunFile[];
  sections: Section[];
  elapsed_seconds: number;
  window_files_completed: number;
  reason?: string;
}
export interface Selection extends ProjectIdentity {
  selection_id: string;
  excluded_paths: string[];
  editable: boolean;
  files: {
    path: string;
    reason: string;
    stages: { stage: string; status: string; reason?: string }[];
  }[];
  recovery?: {
    state: string;
    file_count: number;
    stage_count: number;
    reason?: string;
  };
}
export interface PerformanceFinding {
  id: string;
  category: string;
  potential_impact: string;
  confidence: string;
  title: string;
  observed_pattern: string;
  workload_conditions: string;
  recommendation: string;
  tradeoff: string;
  verification_plan: string;
  start_line: number;
  end_line: number;
  symbol: string;
  engineering_insight?: Insight;
}
export interface PerformanceReport extends ProjectIdentity {
  instructions_fingerprint?: string;
  path: string;
  content_hash: string;
  status: string;
  findings: PerformanceFinding[];
  warning?: string;
  model?: string;
  configured_model?: string;
}
export interface SecurityFinding {
  id: string;
  rule: string;
  title: string;
  source_anchor: { path: string; symbol: string; start_line: number; end_line: number };
  severity: string;
  confidence: string;
  evidence_kind: string;
  observed_condition: string;
  preconditions_or_unknowns: string;
  remediation: string;
  verification_idea: string;
  cwe?: string;
  reference?: string;
  triage: string;
  verification_state: string;
  engineering_insight?: Insight;
}
export interface SecurityReport extends ProjectIdentity {
  instructions_fingerprint?: string;
  path: string;
  content_hash: string;
  status: string;
  source: string;
  findings: SecurityFinding[];
  reason?: string;
  model?: string;
  scope?: string;
}
export interface SectionResults {
  identity: RunIdentity;
  progress: Section;
  saved_finding_count: number | null;
  path?: string;
  semantic: Finding[];
  performance: PerformanceReport[];
  security: SecurityReport[];
  unclassified: Finding[];
  retained_files?: { path: string; content_hash: string; language: string }[];
}
export interface ContextManifest {
  included: {
    path: string;
    size_bytes: number;
    hash: string;
    estimated_tokens?: number;
    truncated?: boolean;
  }[];
  excluded: { path: string; include: boolean; reason: string }[];
  estimated_tokens?: number;
  byte_limit?: number;
  token_limit?: number;
  truncated?: boolean;
  scope?: string;
  model?: string;
  provider_origin?: string;
  remote_provider?: boolean;
}
export interface FileAnalysis extends ProjectIdentity {
  instructions_fingerprint?: string;
  content_hash: string;
  path: string;
  status: string;
  purpose: string;
  responsibilities: string[];
  dependencies: string[];
  side_effects: string[];
  risks: {
    severity: string;
    title?: string;
    summary: string;
    task_spec?: TaskSpec;
    engineering_insight?: Insight;
  }[];
  suggestions: { title: string; summary: string; target_symbol: string; action: string }[];
  symbol_explanations: Record<string, string>;
  engineering_insight?: Insight;
  failure?: string;
}
export interface Explanation extends ProjectIdentity {
  base_file_hash: string;
  anchor: { path: string; symbol: string };
  summary: string;
  behavior: string[];
  inputs: string[];
  outputs: string[];
  side_effects: string[];
  error_behavior: string[];
  engineering_insight?: Insight;
  context_manifest: ContextManifest;
}
export interface Check {
  name: string;
  required: boolean;
  state: string;
  command: string[];
  output: string;
  exit_code: number;
}
export interface ExecutionTrust extends ProjectIdentity {
  trusted: boolean;
  commands: string[][];
}
export interface Scan extends ProjectIdentity {
  status: string;
  phases: { name: string; state: string; command: string[]; output: string; exit_code: number }[];
}
export interface GitStatus {
  available: boolean;
  branch: string;
  file_state: string;
  diff_state: string;
}
export interface Impact {
  target_path: string;
  target_symbol: string;
  references: { path: string; symbol: string; confidence: string; reason: string }[];
}
export interface TerminalUpdate {
  id: string;
  state: string;
  data: string;
  cursor: number;
  reset: boolean;
  error: string;
}

export interface ChangeSession extends ProjectIdentity {
  id: string;
  kind: string;
  title: string;
  acceptance_criteria: string[];
  revision: number;
  hash: string;
  state: string;
  freshness: string;
  targets: { path: string; hash: string; exists: boolean; content: string }[];
  changes: {
    path: string;
    content: string;
    hash: string;
    diff: {
      old_path: string;
      new_path: string;
      lines: { kind: string; old_line: number; new_line: number; text: string }[];
    };
  }[];
  messages: { role: string; content: string }[];
  checks: Check[];
  check_options?: { run_tests?: boolean; run_lint?: boolean };
  reviewed_hash?: string;
  repair_attempts: number;
  workflow?: ChangeWorkflow;
  context_manifest: ContextManifest;
  updated_at: string;
}
export type ChangeHistoryEntry = Pick<
  ChangeSession,
  | 'id'
  | 'project_id'
  | 'project_revision'
  | 'kind'
  | 'title'
  | 'revision'
  | 'hash'
  | 'state'
  | 'freshness'
  | 'updated_at'
> & { workflow_status?: string };
export interface ChangeWorkflowModels {
  create: string;
  test: string;
  review: string;
}
export interface ChangeWorkflow {
  id: string;
  status: string;
  reason?: string;
  models: ChangeWorkflowModels;
  stages: { name: string; status: string; model?: Model }[];
  review?: { proposal_hash: string; verdict: string; summary: string; findings: string[] };
  started_at: string;
  updated_at: string;
}
export interface ChangeMutation extends ProjectIdentity {
  session_id: string;
  state: string;
  hash: string;
  undo_available: boolean;
  index?: ProjectIndex;
  warnings: string[];
  verification?: ChangeVerification;
}
export interface ChangeVerification extends ProjectIdentity {
  session_id: string;
  proposal_hash: string;
  workspace_hash: string;
  status: string;
  reason?: string;
  checks: Check[];
}
export interface ChangeSeed {
  title: string;
  message: string;
  kind: string;
  paths: string[];
  acceptance_criteria: string[];
  finding?: {
    path: string;
    symbol: string;
    line: number;
    cause: string;
    solution: string;
    confidence: string;
  };
}

export interface FeatureGeneration {
  id: string;
  generated_at: string;
  project_revision: string;
  workspace_hash: string;
  goals_hash: string;
  model: {
    profile: string;
    model: string;
    provider_origin: string;
    remote_provider: boolean;
  } | null;
}

export interface FeatureGenerationOutcome {
  generation_id: string;
  added_count: number;
  duplicate_count: number;
}

export interface FeatureSuggestion {
  id: string;
  generation_id?: string;
  freshness?: string;
  title: string;
  benefit: string;
  evidence: string;
  paths: string[];
  effort: string;
  acceptance_criteria: string[];
  status: 'open' | 'saved' | 'dismissed';
}
export interface FeatureReport extends ProjectIdentity {
  schema_version: number;
  hash: string;
  goals: string;
  status: string;
  freshness: string;
  failure?: string;
  suggestions: FeatureSuggestion[];
  generations: FeatureGeneration[];
  last_generation?: FeatureGenerationOutcome;
  workspace_hash: string;
  context_manifest: ContextManifest;
  updated_at: string;
}
export interface InstructionPreview extends ProjectIdentity {
  path: string;
  exists: boolean;
  existing_content: string;
  presets: {
    id: string;
    label: string;
    content: string;
    category?: string;
    reason?: string;
    evidence?: string[];
  }[];
  effective: {
    files: { path: string; scope: string; content: string; hash: string }[];
    excluded: { path: string; reason: string }[];
    text: string;
    fingerprint: string;
  };
}
