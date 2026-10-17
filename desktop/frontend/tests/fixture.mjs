// Browser-only daemon fixture. It is never bundled into the desktop application.
export function installFixture(options = {}) {
  const identity = { project_id: 'project-1', project_revision: 'revision-1' };
  const source =
    options.sourceContent ??
    'package worker\n\nimport "context"\n\nfunc Process(ctx context.Context) error {\n\treturn nil\n}\n';
  const symbol = {
    name: 'Process',
    kind: 'func',
    signature: 'func Process(ctx context.Context) error',
    start_line: 5,
    end_line: 7,
    confidence: 'exact',
    atomic_target: true,
  };
  const files = (
    options.sourcePaths || [
      'internal/worker/process.go',
      'internal/worker/config.go',
      'cmd/server/main.go',
      'internal/queue/queue.go',
    ]
  ).map((path) => ({
    path,
    content_hash: `hash:${path}`,
    language: 'go',
    binary: false,
    size_bytes: 2048,
    line_count: 64,
    imports: ['context'],
    symbols: [symbol],
    analysis_status: 'fresh',
  }));
  if (options.sourceFile) Object.assign(files[0], options.sourceFile);
  const model = (scope) =>
    (window.fixture?.state?.availableModels || options.availableModels)?.models.find(
      (choice) => choice.id === scope,
    )?.model || {
      scope,
      profile: scope,
      model: options.modelNames?.[scope] || 'test-model',
      provider_origin: options.remote ? 'https://provider.invalid' : 'http://127.0.0.1:11434',
      remote_provider: !!options.remote,
      reasoning_effort: 'medium',
      timeout: '2m',
      ...options.modelMetadata?.[scope],
    };
  const insight = {
    mechanism: 'Cancellation needs to reach each unit of work.',
    why_it_matters_here: 'Workers otherwise keep consuming resources after their caller has left.',
    tradeoff_or_failure_mode:
      'Checking too often adds overhead; checking too late delays shutdown.',
    transferable_lesson: 'Make cancellation part of the work boundary.',
  };
  const project = {
    ...identity,
    name: options.projectName || 'harbor',
    path: options.projectPath || '/fixture/harbor',
    type: 'go',
    build_file: 'go.mod',
    file_count: 24,
    source_file_count: 18,
    total_lines: 2450,
    languages: { Go: 18, Markdown: 4, YAML: 2 },
    files: files.map((f) => f.path),
    summary: options.projectSummary || 'A bounded worker service with a local queue.',
    ai_status: 'success',
    analyzed_at: '2026-10-04T00:00:00Z',
  };
  const finding = {
    ...identity,
    id: 'finding-1',
    source: 'semantic',
    confidence: 'ai_suggestion',
    severity: 'high',
    title: 'Cancellation stops at the queue boundary',
    message: 'The worker continues after its request context is canceled.',
    rule: '',
    file_hash: files[0].content_hash,
    location: { path: files[0].path, symbol: 'Process', start_line: 5, end_line: 7 },
    evidence: 'The loop never checks ctx.Err().',
    status: 'open',
    freshness: options.stalePaths?.includes(files[0].path) ? 'stale' : 'fresh',
    category: 'bugs',
    engineering_insight: insight,
  };
  const queue = {
    ...identity,
    policy_fingerprint: 'policy-1',
    provider_fingerprint: 'provider-1',
    queue_id: 'queue-1',
  };
  const limits = { batch_files: 20, budget_seconds: 600, max_attempts_per_stage: 2 };
  const stages = ['semantic', 'performance', 'security_rules', 'security_ai'];
  const idea = {
    id: 'idea-1',
    generation_id: 'gen-1',
    freshness: options.featuresStale ? 'stale' : 'current',
    title: options.featuresLongContent
      ? `Recover failed jobs ${'LongFeatureTitle'.repeat(16)}`
      : 'Retry failed work',
    benefit: 'Let users recover failed jobs without submitting them again.',
    evidence: options.featuresLongContent
      ? `The worker queue already records failed jobs. ${'CompleteEvidence'.repeat(20)}`
      : 'The worker queue already records failed jobs.',
    paths: options.featuresLongContent
      ? [`internal/${'long-directory-'.repeat(16)}/worker.go`]
      : [files[0].path],
    effort: 'medium',
    acceptance_criteria: [
      'Retry only eligible failed jobs.',
      options.featuresLongContent
        ? `Show the new job status. ${'CompleteAcceptanceCriteria'.repeat(20)}`
        : 'Show the new job status.',
    ],
    status: 'open',
  };
  const preview = {
    preview_id: 'preview-1',
    identity: queue,
    scope: 'project',
    refresh: false,
    limits,
    files: files.map((f) => ({
      ...f,
      stages: stages.map((stage) => ({
        stage,
        eligible: true,
        cached: false,
        reason: '',
        max_model_requests: 2,
      })),
    })),
    excluded: [{ path: '.env', reason: 'Excluded by context policy' }],
    providers: [
      {
        id: 'provider-1',
        stages: ['semantic', 'performance', 'security_ai', 'feature_suggestions'],
        model: model('analyze'),
        remote_confirmation_required: !!options.remote,
      },
    ],
    expected_model_requests: 13,
    max_model_requests: 26,
    security_review_intent_required: true,
    features: {
      expected_hash: 'features-empty',
      goals_hash: 'goals-hash',
      workspace_hash: 'workspace-hash',
      excluded_paths: [],
      provider_id: 'provider-1',
      max_model_requests: 2,
    },
  };
  const run = {
    identity: { ...queue, id: 'run-1', generation: 'generation-1' },
    plan: options.capturedModels
      ? {
          ...preview,
          models: options.capturedModels,
          providers: options.capturedProviders ?? preview.providers,
          features: {
            ...preview.features,
            provider_id:
              options.capturedProviders?.find((provider) =>
                provider.stages.includes('feature_suggestions'),
              )?.id ?? preview.features.provider_id,
          },
        }
      : preview,
    status: options.runStatus || 'completed',
    files: files.map((f) => ({
      ...f,
      stages: stages.map((stage) => ({
        stage,
        status: 'completed',
        attempts: 1,
        cached: false,
        finding_count: 1,
      })),
    })),
    sections: ['bugs', 'performance', 'security'].map((category) => ({
      category,
      status: options.runStatus || 'completed',
      coverage: {
        total: 4,
        succeeded: 4,
        partial: 0,
        failed: 0,
        pending: 0,
        running: 0,
        skipped: 0,
        unavailable: 0,
      },
      finding_count:
        category === 'performance' && options.unknown
          ? null
          : options.runStatus === 'completed_empty'
            ? 0
            : 1,
    })),
    elapsed_seconds: 24,
    window_files_completed: 4,
    reason: '',
  };
  const context = {
    included: files.slice(0, 2).map((f) => ({
      path: f.path,
      size_bytes: f.size_bytes,
      hash: f.content_hash,
      estimated_tokens: 350,
      truncated: false,
    })),
    excluded: [{ path: '.env', include: false, reason: 'Secrets excluded' }],
    estimated_tokens: 700,
    token_limit: 8000,
    byte_limit: 64000,
    truncated: false,
    scope: 'function',
    model: 'test-model',
    provider_origin: model('function').provider_origin,
    remote_provider: !!options.remote,
  };
  const performance = {
    ...identity,
    path: files[0].path,
    content_hash: files[0].content_hash,
    status: options.stalePaths?.includes(files[0].path) ? 'stale' : 'success',
    findings: [
      {
        id: 'perf-1',
        category: 'allocation',
        potential_impact: 'medium',
        confidence: 'ai_suggestion',
        title: 'Batch allocations in the worker loop',
        observed_pattern: 'A new buffer is allocated per item.',
        workload_conditions: 'High throughput queues.',
        recommendation: 'Reuse a bounded buffer.',
        tradeoff: 'Retained memory can increase.',
        verification_plan: 'Compare an existing worker benchmark.',
        start_line: 5,
        end_line: 7,
        symbol: 'Process',
        engineering_insight: insight,
      },
    ],
  };
  const security = {
    ...identity,
    path: files[0].path,
    content_hash: files[0].content_hash,
    status: options.stalePaths?.includes(files[0].path) ? 'stale' : 'success',
    source: 'deterministic',
    findings: [
      {
        id: 'sec-1',
        rule: 'path-containment',
        title: 'Unbounded input path',
        source_anchor: { path: files[0].path, symbol: 'Process', start_line: 5, end_line: 7 },
        severity: 'medium',
        confidence: 'candidate',
        evidence_kind: 'source_pattern',
        observed_condition: 'An input path reaches a filesystem operation.',
        preconditions_or_unknowns: 'Caller validation has not been established.',
        remediation: 'Check containment before opening the path.',
        verification_idea: 'Exercise parent path traversal.',
        triage: 'open',
        verification_state: 'unverified',
        cwe: 'CWE-22',
      },
    ],
  };
  // Complete detail evidence is opt-in so ordinary workflow fixtures stay small.
  if (options.findingDetail) {
    const category = options.findingDetail.split('-')[0];
    const path = `internal/${'long-source-anchor-'.repeat(14)}/process.go`;
    const prose = (label) =>
      `${label}: complete first paragraph.\n\n${'Full explanation with workload, uncertainty and recovery. '.repeat(12)}${'unbroken-evidence-'.repeat(16)}\n\n${label}: final paragraph. <script>window.detailExecuted = true</script> ![remote](https://evidence.invalid/image.png)`;
    files[0].path = path;
    project.files[0] = path;
    finding.category = category;
    finding.title = `Complete ${category} evidence ${'long-finding-title-'.repeat(12)}`;
    finding.location = { path, symbol: 'LongDeclaration'.repeat(16), start_line: 5, end_line: 7 };
    finding.message = prose('Finding');
    finding.evidence = prose('Evidence');
    finding.task_spec = {
      schema_version: '1',
      target_path: path,
      target_symbol: finding.location.symbol,
      target_signature: symbol.signature,
      acceptance_criteria: [prose('Acceptance criterion')],
      non_goals: [prose('Non-goal')],
      go_test_candidate: {
        name: 'TestProcessCancellation',
        content: `func TestProcessCancellation(t *testing.T) {\n\t// ${'long-test-guidance-'.repeat(30)}\n}`,
      },
    };
    Object.keys(insight).forEach((key) => {
      insight[key] = prose(key);
    });
    performance.path = path;
    const perf = performance.findings[0];
    perf.title = finding.title;
    perf.symbol = finding.location.symbol;
    for (const key of [
      'observed_pattern',
      'workload_conditions',
      'recommendation',
      'tradeoff',
      'verification_plan',
    ])
      perf[key] = prose(key);
    security.path = path;
    const sec = security.findings[0];
    sec.title = finding.title;
    sec.source_anchor = { ...finding.location };
    sec.engineering_insight = insight;
    for (const key of [
      'observed_condition',
      'preconditions_or_unknowns',
      'remediation',
      'verification_idea',
      'rule',
      'cwe',
      'reference',
    ])
      sec[key] = prose(key);
  }
  const state = {
    availableModels: options.availableModels,
    project,
    files,
    finding,
    completedFindings: [],
    run: options.empty ? null : { ...run, ...structuredClone(options.runOverride || {}) },
    preview,
    scan: options.scanReport ? { ...identity, ...structuredClone(options.scanReport) } : null,
    context,
    performance,
    security,
    results: options.findingDetail?.endsWith('-semantic')
      ? {
          [options.findingDetail.split('-')[0]]: {
            semantic: [finding],
            performance: [],
            security: [],
          },
        }
      : {},
    trusted: !!options.trusted,
    changes: {},
    changeReceipt: null,
    features: {
      ...identity,
      schema_version: options.legacyShape ? 1 : 2,
      hash: 'features-empty',
      workspace_hash: 'workspace-1',
      updated_at: '2026-10-04T01:00:00Z',
      goals: options.featuresLongContent ? `Recovery goals ${'LongGoal'.repeat(40)}` : '',
      status: options.featuresReady ? (options.featuresFail ? 'failed' : 'ready') : 'not_generated',
      freshness: options.featuresStale ? 'stale' : 'current',
      suggestions:
        options.featuresReady && !options.featuresEmpty
          ? [
              {
                ...idea,
                generation_id: options.legacyShape ? 'legacy' : 'gen-1',
                freshness: options.featuresStale ? 'stale' : 'current',
              },
              ...(options.featuresMixedTriage
                ? [
                    { ...idea, id: 'idea-saved', title: 'Saved recovery idea', status: 'saved' },
                    {
                      ...idea,
                      id: 'idea-dismissed',
                      title: 'Dismissed recovery idea',
                      status: 'dismissed',
                    },
                  ]
                : []),
            ]
          : [],
      generations: options.featuresReady
        ? [
            {
              id: options.legacyShape ? 'legacy' : 'gen-1',
              generated_at: '2026-10-04T01:00:00Z',
              project_revision: identity.project_revision,
              workspace_hash: 'workspace-1',
              goals_hash: 'goals-1',
              model: options.legacyShape
                ? null
                : {
                    profile: 'analyze',
                    model: model('analyze').model,
                    provider_origin: model('analyze').provider_origin,
                    remote_provider: model('analyze').remote_provider,
                  },
            },
          ]
        : [],
      last_generation:
        options.legacyShape || !options.featuresReady
          ? undefined
          : {
              generation_id: 'gen-1',
              added_count: options.featuresEmpty ? 0 : options.featuresMixedTriage ? 3 : 1,
              duplicate_count: 0,
            },
      failure:
        options.featuresReady && options.featuresFail ? 'Feature search failed. Try again.' : '',
      context_manifest: options.featuresLongContent
        ? { ...context, included: idea.paths.map((path) => ({ ...context.included[0], path })) }
        : context,
    },
    instructions: options.instructionsMissing
      ? {}
      : {
          'AGENTS.md': options.instructionsLongContent
            ? `# Project rules\n\nPreserve public APIs.\n\nInherited boundary: ${'RootGuidance'.repeat(40)}\n`
            : '# Project rules\n\nPreserve public APIs.\n',
          'internal/AGENTS.md': '# Internal rules\n\nPropagate cancellation.\n',
          ...(options.instructionPath
            ? {
                [options.instructionPath]: `# Directory rules\n\nPropagate cancellation.\n\n${'DirectoryGuidance'.repeat(40)}\n`,
              }
            : {}),
        },
    excluded: [],
    selectionId: 'selection-1',
    source,
    changed: false,
  };
  // Existing workflow cases use a non-Git project. Git cases opt into repository state.
  state.git = {
    available: !!options.git,
    at_root: true,
    root: project.path,
    head: 'head-main',
    branch: options.gitBranch || 'main',
    dirty: !!options.gitDirty,
    conflicts: [],
  };
  state.gitWorktrees = [
    {
      path: project.path,
      head: state.git.head,
      branch: state.git.branch,
      current: true,
      mergeable: false,
      dirty: state.git.dirty,
      completed: [],
    },
    {
      path: `${project.path}-fix`,
      head: 'head-fix',
      branch: 'codex/fix',
      current: false,
      mergeable: true,
      dirty: false,
      completed: [{ id: 'completed-fix', title: 'Handle cancellation', state: 'applied' }],
    },
  ];
  const requests = [],
    terminals = [],
    failures = {
      ...(options.startupOffline ? { '/status': 'transport' } : {}),
      ...(options.modelCatalogUnavailable ? { '/api/models/current': 503 } : {}),
      ...(options.selectionUnavailable ? { '/api/projects/current/analysis/selection': 503 } : {}),
      ...(options.restoreFailure ? { '/api/projects/restore': 503 } : {}),
      ...(options.importFailure ? { '/api/projects/import': 503 } : {}),
    };
  const rootState = state;
  const worktreeStates = {};
  function scopedState(path) {
    if (!path) return rootState;
    if (!worktreeStates[path]) {
      const tree = rootState.gitWorktrees.find((tree) => tree.path === path);
      if (!tree) throw new Error('Unknown fixture worktree');
      const child = structuredClone(rootState);
      child.project = {
        ...child.project,
        project_id: `project-worktree-${tree.branch}`,
        path,
        name: path.split('/').at(-1),
      };
      child.git = { ...child.git, root: path, branch: tree.branch, head: tree.head, dirty: false };
      child.trusted = false;
      child.changes = {};
      child.changeReceipt = null;
      worktreeStates[path] = child;
    }
    return worktreeStates[path];
  }
  let held = null;
  window.fixture = {
    state,
    worktreeStates,
    options,
    requests,
    terminals,
    failures,
    release: () => held?.(),
    hold: options.holdOpening ? '/api/projects/restore' : '',
  };
  if (options.noRememberedProject) localStorage.removeItem('mini-orca:last-project');
  else localStorage.setItem('mini-orca:last-project', project.path);
  const proposalDiff = () => ({
    old_path: files[0].path,
    new_path: files[0].path,
    lines: [
      {
        kind: 'context',
        old_line: 5,
        new_line: 5,
        text: 'func Process(ctx context.Context) error {',
      },
      { kind: 'removed', old_line: 6, text: '\treturn nil' },
      { kind: 'added', new_line: 6, text: '\treturn ctx.Err()' },
      { kind: 'context', old_line: 7, new_line: 7, text: '}' },
    ],
  });
  window.go = {
    main: {
      Desktop: {
        async Request(id, method, target, payload) {
          const url = new URL(target, 'http://fixture');
          const path = url.pathname;
          const state = scopedState(url.searchParams.get('worktree_path'));
          const identity = {
            project_id: state.project.project_id,
            project_revision: state.project.project_revision,
          };
          const body = payload ? JSON.parse(payload) : undefined;
          requests.push({ id, method, path, query: Object.fromEntries(url.searchParams), body });
          if (window.fixture.hold === path || window.fixture.hold === `${method} ${path}`)
            await new Promise((resolve) => {
              held = resolve;
            });
          if (failures[path]) {
            const failure = failures[path];
            delete failures[path];
            if (failure === 'transport') throw new Error('Connection lost');
            return {
              status: failure,
              body: JSON.stringify({ type: 'conflict', user_message: 'Fixture rejection' }),
            };
          }
          const response = (value) => ({
            status: value === null ? 204 : 200,
            body: value === null ? '' : JSON.stringify(value),
          });
          if (path === '/api/projects/current') return response(state.project);
          const rev = { ...identity, project_revision: state.project.project_revision };
          const pathFile =
            state.files.find((f) => f.path === (url.searchParams.get('path') || body?.path)) ||
            state.files[0];
          const repository = () => ({ ...rev, ...state.git });
          if (path === '/api/projects/current/git/repository') return response(repository());
          if (path === '/api/projects/current/git/workspace')
            return response({
              ...repository(),
              branches: [
                { name: state.git.branch, head: state.git.head, worktree_path: state.git.root },
                { name: 'feature/other', head: 'head-other' },
                {
                  name: 'codex/fix',
                  head: 'head-fix',
                  worktree_path: state.gitWorktrees.find((tree) => tree.branch === 'codex/fix')
                    ?.path,
                },
              ],
              worktrees: state.gitWorktrees.map((tree) =>
                tree.path === state.project.path
                  ? { ...tree, ...state.git, path: tree.path, current: true, mergeable: false }
                  : {
                      ...tree,
                      current: false,
                      sessions: worktreeStates[tree.path]
                        ? Object.values(worktreeStates[tree.path].changes)
                        : tree.completed || [],
                      mergeable:
                        !!tree.mergeable &&
                        tree.head !== state.git.head &&
                        !tree.dirty &&
                        !tree.locked &&
                        !tree.prunable &&
                        !tree.error &&
                        !!tree.branch &&
                        !state.git.dirty &&
                        !state.git.merge_head &&
                        !state.git.operation,
                    },
              ),
            });
          if (path === '/api/projects/current/git/worktrees') {
            if (options.gitCreateFailure)
              return response({ state: 'failed', error: 'Branch already exists.', warnings: [] });
            state.gitWorktrees.push({
              path: body.path,
              branch: body.target_branch,
              head: body.head,
              current: false,
              mergeable: false,
              dirty: false,
              completed: [],
            });
            return response({
              state: 'created',
              path: body.path,
              repository: repository(),
              warnings: [],
            });
          }
          if (path === '/api/projects/current/git/merge-preview') {
            const tree = state.gitWorktrees.find((item) => item.path === body.path);
            return response({
              target: { head: body.head, branch: body.branch },
              source: { head: body.source_head, branch: tree?.branch || 'codex/fix' },
              path: body.path,
              commits: 'a1b2c3d Handle cancellation',
              diff: 'diff --git a/internal/worker/process.go b/internal/worker/process.go\n- return nil\n+ return ctx.Err()\n',
            });
          }
          if (path === '/api/projects/current/git/discard-worktree') {
            const tree = state.gitWorktrees.find((tree) => tree.path === body.path);
            if (
              !tree ||
              tree.head !== body.source_head ||
              tree.branch !== body.source_branch ||
              tree.dirty
            )
              return {
                status: 409,
                body: JSON.stringify({
                  type: 'conflict',
                  user_message: 'Worktree changed; refresh Git status.',
                }),
              };
            if (options.gitDiscardFailure)
              return response({ state: 'failed', error: 'The checkout is locked.', warnings: [] });
            state.gitWorktrees = state.gitWorktrees.filter((tree) => tree.path !== body.path);
            return response({
              state: 'discarded',
              removed_worktree: body.path,
              repository: repository(),
              warnings: [],
            });
          }
          if (
            [
              '/api/projects/current/git/switch',
              '/api/projects/current/git/merge',
              '/api/projects/current/git/abort-merge',
            ].includes(path)
          ) {
            let outcome = 'switched';
            let removedWorktree;
            const warnings = [];
            if (path.endsWith('/switch')) {
              state.git.branch = body.target_branch;
              state.git.head = 'head-other';
            }
            if (path.endsWith('/merge')) {
              outcome = options.gitMergeConflict ? 'conflicts' : 'merged';
              if (options.gitMergeConflict) {
                state.git.merge_head = body.source_head;
                state.git.conflicts = ['internal/worker/process.go'];
                state.git.dirty = true;
              } else {
                state.git.head = body.source_head;
                const childState = worktreeStates[body.path];
                const childTargets = childState
                  ? Object.values(childState.changes)
                      .filter((c) => c.state === 'applied')
                      .flatMap((c) => c.finding_targets || [])
                      .map((t) => ({
                        ...t,
                        project_id: state.project.project_id,
                        commit_oid: body.source_head,
                      }))
                  : [];
                state.completedFindings = [
                  ...(state.completedFindings || []),
                  ...(options.gitFindingCompletions || []),
                  ...childTargets,
                ];
                if (options.gitCleanupFailure)
                  warnings.push(
                    `Merge completed, but worktree ${body.path} was not removed: checkout is locked.`,
                  );
                else {
                  removedWorktree = body.path;
                  state.gitWorktrees = state.gitWorktrees.filter((tree) => tree.path !== body.path);
                }
              }
            }
            if (path.endsWith('/abort-merge')) {
              outcome = 'aborted';
              state.git.merge_head = '';
              state.git.conflicts = [];
              state.git.dirty = false;
            }
            state.project.project_revision = `revision-${outcome}`;
            state.trusted = false;
            return response({
              state: outcome,
              removed_worktree: removedWorktree,
              repository: { ...repository(), project_revision: state.project.project_revision },
              index: {
                ...identity,
                project_revision: state.project.project_revision,
                files: state.files,
              },
              warnings,
              ...(outcome === 'conflicts'
                ? {
                    error: 'Merge conflicts occurred. Resolve the listed files or abort the merge.',
                  }
                : {}),
            });
          }
          if (path === '/api/projects/current/changes/recovery')
            return response(
              options.recoveryDropsWarnings && state.changeReceipt
                ? { ...state.changeReceipt, warnings: [] }
                : state.changeReceipt,
            );
          if (path === '/api/projects/current/features' && options.featuresReadFail)
            return {
              status: 503,
              body: JSON.stringify({
                type: 'unavailable',
                user_message: 'Saved suggestions could not be read.',
              }),
            };
          if (path === '/api/projects/current/features')
            return response({
              ...state.features,
              freshness:
                options.featuresStale || state.changed ? 'stale' : state.features.freshness,
            });
          if (path.includes('/features/')) {
            if (path.endsWith('/goals')) {
              if (state.features.goals !== body.goals && state.features.suggestions.length)
                state.features.freshness = 'stale';
              state.features.goals = body.goals;
            } else if (path.endsWith('/generate')) {
              state.features.goals = body.goals;
              state.features.status = options.featuresFail ? 'failed' : 'ready';
              state.features.failure = options.featuresFail
                ? 'Feature search failed. Try again.'
                : '';
              state.features.freshness = options.featuresStale ? 'stale' : 'current';
              if (!options.featuresFail) {
                const newGenId = 'gen-' + ((state.features.generations || []).length + 1);
                state.features.generations = [
                  ...(state.features.generations || []),
                  {
                    id: newGenId,
                    generated_at: '2026-10-04T01:05:00Z',
                    project_revision: rev.project_revision,
                    workspace_hash: 'workspace-1',
                    goals_hash: `goals:${body.goals}`,
                    model: {
                      profile: body.profile || 'analyze',
                      model: model(body.profile || 'analyze').model,
                      provider_origin: model(body.profile || 'analyze').provider_origin,
                      remote_provider: model(body.profile || 'analyze').remote_provider,
                    },
                  },
                ];
                state.features.schema_version = 2;
                state.features.last_generation = {
                  generation_id: newGenId,
                  added_count: options.featuresEmpty || options.duplicateOnly ? 0 : 1,
                  duplicate_count: options.duplicateOnly ? 1 : 0,
                };
                if (options.featuresEmpty) {
                  // do not add ideas
                } else if (options.duplicateOnly) {
                  // do not add new ideas
                } else {
                  state.features.suggestions = [
                    ...(state.features.suggestions || []),
                    {
                      ...idea,
                      id: 'idea-' + ((state.features.suggestions || []).length + 1),
                      generation_id: newGenId,
                      freshness: 'current',
                    },
                  ];
                }
              }
            } else {
              state.features.suggestions.find((idea) => idea.id === path.split('/').at(-1)).status =
                body.status;
            }
            state.features.hash += '-next';
            if (path.endsWith('/generate') && options.featuresFail)
              return {
                status: 500,
                body: JSON.stringify({ user_message: 'Feature generation request failed.' }),
              };
            return response(state.features);
          }
          if (path === '/api/projects/current/instructions') {
            const target = url.searchParams.get('path');
            const inherited = Object.entries(state.instructions).filter(
              ([file]) =>
                (file === 'AGENTS.md' ||
                  file === target ||
                  target.startsWith(file.slice(0, -'AGENTS.md'.length))) &&
                !(options.instructionsExcluded && file === target),
            );
            return response({
              ...rev,
              project_revision: options.instructionsStale ? 'older-revision' : rev.project_revision,
              path: target,
              exists: target in state.instructions,
              existing_content: state.instructions[target] || '',
              effective: {
                files: inherited.map(([path, content]) => ({
                  path,
                  content,
                  hash: 'guide',
                  scope: path === 'AGENTS.md' ? '.' : path.slice(0, -'/AGENTS.md'.length),
                })),
                excluded: options.instructionsExcluded
                  ? [
                      {
                        path: target,
                        reason: `Excluded by context policy ${'PolicyReason'.repeat(30)}`,
                      },
                    ]
                  : [],
                fingerprint: 'guide-1',
              },
              presets: options.instructionPresets || [
                {
                  id: 'go',
                  label: 'Follow Go conventions',
                  category: 'Code style and conventions',
                  content: 'Format changed Go code with gofmt.',
                  reason: 'Go source in this scope',
                  evidence: ['internal/worker.go'],
                },
                {
                  id: 'focused',
                  label: 'Keep changes focused',
                  category: 'Review and handoff',
                  content: 'Preserve unrelated work.',
                },
                {
                  id: 'tests',
                  label: 'Test meaningful behavior',
                  category: 'Testing and validation',
                  content: 'Test behavior and error paths.',
                },
              ],
            });
          }
          if (path === '/api/projects/current/instructions/proposal') {
            const change = {
              ...rev,
              id: `change-${Object.keys(state.changes).length + 1}`,
              kind: body.kind || 'instructions',
              title: body.title,
              acceptance_criteria: body.acceptance_criteria,
              revision: 1,
              hash: 'instructions-1',
              state: 'draft',
              freshness: 'current',
              targets: [
                {
                  path: body.paths[0],
                  content: state.instructions[body.paths[0]] || '',
                  exists: body.paths[0] in state.instructions,
                  hash: 'guide',
                },
              ],
              changes: [
                {
                  path: body.paths[0],
                  content: body.content,
                  hash: 'guide-next',
                  diff: {
                    lines: body.content
                      .split('\n')
                      .map((text, i) => ({ kind: 'add', text, new_line: i + 1 })),
                  },
                },
              ],
              messages: [],
              checks: [],
              repair_attempts: 0,
              context_manifest: context,
            };
            if (options.git) change.git = { head: state.git.head, branch: state.git.branch };
            state.changes[change.id] = change;
            return response(change);
          }
          if (path === '/api/projects/current/changes') {
            if (method === 'GET')
              return options.changeHistoryReadFail
                ? {
                    status: 503,
                    body: JSON.stringify({
                      user_message: 'Saved conversations could not be read.',
                    }),
                  }
                : response(Object.values(state.changes));
            const change = {
              ...rev,
              id: `change-${Object.keys(state.changes).length + 1}`,
              kind: body.kind,
              title: body.title,
              acceptance_criteria: body.acceptance_criteria || [],
              finding_targets: body.finding_targets,
              revision: 0,
              hash: 'empty',
              state: 'draft',
              freshness: 'current',
              targets: body.paths.map((path) => ({
                path,
                exists: files.some((file) => file.path === path),
                hash: 'base',
                content: source,
              })),
              changes: [],
              messages: [],
              checks: [],
              reviewed_hash: '',
              repair_attempts: 0,
              context_manifest: context,
            };
            if (options.git) change.git = { head: state.git.head, branch: state.git.branch };
            state.changes[change.id] = change;
            return response(change);
          }
          if (path.includes('/changes/')) {
            const parts = path.split('/');
            const change = state.changes[parts[5]];
            const action = parts[6];
            if (!change)
              return {
                status: 400,
                body: JSON.stringify({ user_message: 'Conversation unavailable' }),
              };
            if (!action)
              return response({ ...change, freshness: state.changed ? 'stale' : 'current' });
            if (action === 'workflow') {
              if (parts[7] === 'cancel') {
                change.workflow.status = 'canceled';
                change.workflow.reason = 'Workflow canceled. No source was applied.';
                change.workflow.stages[0].status = 'canceled';
                return response(change);
              }
              change.revision++;
              change.workflow = {
                id: `workflow-${change.revision}`,
                status: 'running',
                models: body.models,
                stages: ['create', 'test', 'review', 'human_review'].map((name, index) => ({
                  name,
                  status: index === 0 ? 'running' : index === 3 ? 'blocked' : 'pending',
                  ...(index < 3 ? { model: model(body.models[name]) } : {}),
                })),
                started_at: '2026-10-08T00:00:00Z',
                updated_at: '2026-10-08T00:00:00Z',
              };
              change.reviewed_hash = '';
              change.messages.push({ role: 'user', content: body.message });
              state.completeWorkflow = window.fixture.completeWorkflow = (
                status = 'awaiting_human_review',
              ) => {
                change.revision++;
                change.hash = `workflow-proposal-${change.revision}`;
                change.changes = change.targets.map((target) => ({
                  path: target.path,
                  content: source.replace('return nil', 'return ctx.Err()'),
                  hash: `candidate-${change.revision}`,
                  diff: proposalDiff(),
                }));
                change.messages.push({
                  role: 'assistant',
                  content:
                    options.changeAssistantMessage ||
                    'Return the context error and cover cancellation in a regression test.',
                });
                change.check_options = { run_tests: true };
                change.checks = [
                  {
                    name: 'tests',
                    required: true,
                    state: status === 'failed' ? 'failed' : 'passed',
                    output:
                      status === 'failed'
                        ? window.fixture.state.failedTestOutput || 'Boundary test failed.'
                        : '',
                  },
                ];
                change.workflow.status = status;
                change.workflow.updated_at = '2026-10-08T00:01:00Z';
                change.workflow.stages.forEach((stage, index) => {
                  stage.status =
                    index === 3
                      ? status === 'awaiting_human_review'
                        ? 'waiting'
                        : 'blocked'
                      : 'completed';
                });
                if (status === 'failed') {
                  change.workflow.reason =
                    'Tests or source checks failed. Inspect the check evidence before running again.';
                  change.workflow.stages[1].status = 'failed';
                  change.workflow.stages[2].status = 'skipped';
                } else {
                  change.workflow.review = {
                    proposal_hash: change.hash,
                    verdict: status === 'changes_requested' ? 'changes_requested' : 'approve',
                    summary:
                      options.workflowSummary ||
                      'The change handles cancellation and its tests pass.',
                    findings:
                      status === 'changes_requested'
                        ? ['Add a regression test for invalid input.']
                        : [],
                  };
                }
              };
              const nextStatus = window.fixture.state.nextWorkflowStatus || options.workflowStatus;
              window.fixture.state.nextWorkflowStatus = undefined;
              if (!options.workflowRunning) window.fixture.completeWorkflow(nextStatus);
              return response(change);
            }
            if (action === 'resume') {
              change.checks = [];
              change.reviewed_hash = '';
              change.freshness = state.changed ? 'stale' : 'current';
              return response(change);
            }
            if (action === 'messages') {
              change.revision++;
              change.hash = `proposal-${change.revision}`;
              if (body.repair) change.repair_attempts++;
              change.messages.push(
                { role: 'user', content: body.message },
                {
                  role: 'assistant',
                  content:
                    options.changeAssistantMessage ||
                    'The proposal handles cancellation and preserves its scope.',
                },
              );
              if (options.changeProposalKind) change.kind = options.changeProposalKind;
              change.changes = options.changeMissingDiffs
                ? []
                : change.targets.map((target) => ({
                    path: target.path,
                    content: source.replace('return nil', 'return ctx.Err()'),
                    hash: `candidate-${change.revision}`,
                    diff: options.changeDiff || proposalDiff(),
                  }));
              change.checks = [];
              change.reviewed_hash = '';
              return response(change);
            }
            if (action === 'checks') {
              change.checks = [
                { name: 'parse', required: true, state: 'passed' },
                {
                  name: 'tests',
                  required: true,
                  state: options.changeChecksFail ? 'failed' : 'passed',
                  output:
                    options.changeCheckOutput ||
                    (options.changeChecksFail ? 'Fixture test failure' : ''),
                },
              ];
              change.check_options = { run_tests: body.run_tests };
              change.reviewed_hash = '';
              return response(change);
            }
            if (action === 'review') {
              change.reviewed_hash = change.hash;
              return response(change);
            }
            if (action === 'verify') {
              state.changeReceipt.verification = {
                ...rev,
                session_id: change.id,
                proposal_hash: change.hash,
                workspace_hash: 'applied-workspace',
                status: options.verificationFail ? 'failed' : 'verified',
                reason: options.verificationFail ? 'An applied verification check failed.' : '',
                checks: [
                  {
                    name: 'post-Apply tests',
                    required: true,
                    state: options.verificationFail ? 'failed' : 'passed',
                    output: options.verificationFail ? 'Regression failure' : '',
                  },
                ],
              };
              Object.assign(
                state.changeReceipt.verification,
                structuredClone(options.changeVerification || {}),
              );
              return response(state.changeReceipt.verification);
            }
            if (action === 'commit') {
              state.changeReceipt.commit = {
                ...state.changeReceipt.commit,
                state: 'committed',
                message: body.commit_message,
                oid: 'commit-retry',
                error: '',
              };
              state.git.head = 'commit-retry';
              return response(state.changeReceipt);
            }
            if (action === 'apply' || action === 'undo') {
              state.trusted = false;
              change.state =
                action === 'apply' ? options.changeMutationState || 'applied' : 'undone';
              state.project.project_revision = action === 'apply' ? 'revision-2' : 'revision-3';
              state.source = action === 'apply' ? change.changes[0].content : source;
              if (change.kind === 'instructions')
                for (const edit of change.changes)
                  state.instructions[edit.path] =
                    action === 'apply'
                      ? edit.content
                      : change.targets.find((target) => target.path === edit.path).content;
              if (action === 'apply') {
                for (const edit of change.changes) {
                  if (
                    [
                      'justfile',
                      'taskfile.yml',
                      '.mise.toml',
                      'devbox.json',
                      'lefthook.yml',
                    ].includes(edit.path.toLowerCase())
                  ) {
                    const lineCount = edit.content.split('\n').length;
                    const existing = state.files.find((f) => f.path === edit.path);
                    if (existing) {
                      existing.line_count = lineCount;
                    } else {
                      state.files.push({
                        path: edit.path,
                        content_hash: `hash-${edit.path}`,
                        language: 'Text',
                        size_bytes: edit.content.length,
                        line_count: lineCount,
                        modified_at: '2026-03-01T00:00:00Z',
                        binary: false,
                        imports: [],
                        symbols: [],
                        analysis_status: 'missing',
                      });
                    }
                  }
                }
              }
              state.changeReceipt = {
                project_id: identity.project_id,
                project_revision: state.project.project_revision,
                session_id: change.id,
                state: change.state,
                hash: change.hash,
                undo_available: action === 'apply',
                index:
                  action === 'apply' && options.changeMutationMissingIndex
                    ? undefined
                    : {
                        ...identity,
                        project_revision: state.project.project_revision,
                        files: state.files,
                      },
                warnings:
                  action === 'apply' && options.mutationWarning ? [options.mutationWarning] : [],
              };
              if (action === 'apply' && body.commit_message) {
                state.changeReceipt.commit = {
                  head: body.head,
                  branch: body.branch,
                  message: body.commit_message,
                  state: options.gitCommitFailure ? 'failed' : 'committed',
                  ...(options.gitCommitFailure
                    ? { error: 'Configure a Git author before retrying.' }
                    : { oid: 'commit-applied' }),
                };
                if (!options.gitCommitFailure) {
                  state.git.head = 'commit-applied';
                  const wtPath = url.searchParams.get('worktree_path');
                  if (wtPath) {
                    const tree = rootState.gitWorktrees.find((item) => item.path === wtPath);
                    if (tree) {
                      tree.head = 'commit-applied';
                      tree.mergeable = true;
                      tree.completed = [
                        {
                          id: change.id,
                          title: change.title,
                          state: 'applied',
                          commit: state.changeReceipt.commit,
                          paths: change.targets.map((t) => t.path),
                          finding_targets: change.finding_targets,
                        },
                      ];
                    }
                  }
                }
              }
              if (action === 'apply' && options.changeMutationResponseLost)
                throw new Error('Apply response lost after source mutation');
              return response(state.changeReceipt);
            }
          }
          if (path === '/status')
            return response({
              status: 'running',
              version: 'fixture',
              workflow: 'single_coder_preview',
            });
          if (path === '/api/models/available')
            return response(
              state.availableModels || {
                defaults: { analyze: 'analyze', bug: 'bug', function: 'function' },
                models: options.emptyModelCatalog
                  ? []
                  : ['analyze', 'bug', 'function'].map((scope) => ({
                      id: scope,
                      name: model(scope).model,
                      provider: 'openai',
                      source: 'configured',
                      location: model(scope).remote_provider ? 'remote' : 'local',
                      model: { ...model(scope), scope: '' },
                    })),
                pi: { status: 'ready' },
                agy: { status: 'ready' },
              },
            );
          if (path === '/api/models/current')
            return response({
              scopes: options.emptyModelCatalog
                ? {}
                : { analyze: model('analyze'), bug: model('bug'), function: model('function') },
            });
          if (path === '/api/projects/restore' || path === '/api/projects/import') {
            if (options.git && body.project_path !== state.project.path) {
              const tree = state.gitWorktrees.find((tree) => tree.path === body.project_path);
              if (tree) {
                identity.project_id = 'project-worktree';
                state.project = {
                  ...state.project,
                  ...identity,
                  path: tree.path,
                  name: tree.path.split('/').at(-1),
                };
                state.git = {
                  ...state.git,
                  root: tree.path,
                  branch: tree.branch,
                  head: tree.head,
                  dirty: false,
                };
                state.trusted = false;
                state.changes = {};
                state.changeReceipt = null;
              }
            }
            return response(state.project);
          }
          if (path.endsWith('/overview') && options.overviewReadFail)
            return {
              status: 503,
              body: JSON.stringify({
                type: 'unavailable',
                user_message: 'Saved overview could not be read.',
              }),
            };
          if (path.endsWith('/overview'))
            return response({
              ...rev,
              metrics: project,
              analysis: {
                purpose: project.summary,
                architecture:
                  options.architecture ??
                  'flowchart TD\n  API --> Queue\n  Queue --> Workers\n  Workers --> Store',
                components: ['HTTP API', 'Bounded queue', 'Worker pool', 'Local result store'],
                entry_points: ['cmd/server/main.go'],
                flows: options.flows ?? [
                  'flowchart TD\n  Request --> Validate --> Process --> Result',
                  'sequenceDiagram\n  Client->>API: Request\n  API->>Worker: Process\n  Worker-->>Client: Result',
                ],
                risks: [],
                next_steps: ['Propagate cancellation through the worker loop.'],
                engineering_insight: insight,
                status: 'success',
              },
              analysis_coverage: options.coverage || {
                total: 18,
                fresh: 14,
                stale: 2,
                missing: 2,
                failed: 0,
              },
              finding_counts: { verified: 0, ai_suggestions: 3 },
              analysis_run: state.run,
            });
          if (path.endsWith('/index') || path.endsWith('/reindex'))
            return response({ ...rev, files: options.emptyIndex ? [] : state.files });
          if (path.endsWith('/findings'))
            return response({
              ...rev,
              findings:
                options.empty || state.clearedResults ? [] : state.findingsList || [state.finding],
              completed: state.completedFindings,
            });
          if (path.includes('/findings/')) {
            const target = body.target || {
              project_id: state.project.project_id,
              kind: 'semantic',
              id: path.split('/')[5] || state.finding.id,
              path: state.finding.location.path,
            };
            if (target.id === state.finding.id) state.finding.status = body.status;
            for (const cat of ['bugs', 'security', 'performance']) {
              for (const item of state.results[cat]?.semantic || []) {
                if (item.id === target.id) item.status = body.status;
              }
            }
            if (body.status === 'fixed') {
              state.completedFindings = [
                ...(state.completedFindings || []).filter(
                  (c) => !(c.kind === target.kind && c.id === target.id && c.path === target.path),
                ),
                { ...target, project_id: state.project.project_id },
              ];
            } else {
              state.completedFindings = (state.completedFindings || []).filter(
                (c) => !(c.kind === target.kind && c.id === target.id && c.path === target.path),
              );
            }
            return response(null);
          }
          if (path.endsWith('/analysis/selection')) {
            if (method === 'POST') {
              state.excluded = body.excluded_paths;
              state.selectionId = 'selection-2';
            }
            return response({
              ...rev,
              selection_id: state.selectionId,
              excluded_paths: state.excluded,
              editable: ![
                'queued',
                'running',
                'pausing',
                'paused',
                'interrupted',
                'canceling',
              ].includes(state.run?.status),
              files: files.map((f) => ({
                path: f.path,
                reason: '',
                stages: stages.map((stage) => ({
                  stage,
                  status: options.stalePaths?.includes(f.path)
                    ? 'stale'
                    : options.failedPaths?.includes(f.path)
                      ? 'failed'
                      : 'fresh',
                  reason: '',
                })),
              })),
              recovery: options.hasRecovery
                ? {
                    state: 'available',
                    file_count: 1,
                    stage_count: 1,
                  }
                : undefined,
            });
          }
          if (path.endsWith('/analysis/preview')) {
            state.preview = structuredClone(body.resume_run ? state.run.plan : preview);
            if (!body.resume_run) state.preview.limits = structuredClone(body.limits);
            state.preview.refresh = body.resume_run ? !!state.run.plan.refresh : !!body.refresh;
            state.preview.clear_results = body.resume_run
              ? !!state.run.plan.clear_results
              : !!body.clear_results;
            const choices = body.resume_run ? state.run.plan.models : body.models;
            state.preview.models = choices;
            if (body.recover_incomplete) {
              state.preview.recover_incomplete = true;
            }
            if (body.stale_only) {
              state.preview.stale_only = true;
              state.preview.stale_path = body.stale_path;
              state.preview.files = state.preview.files.filter(
                (file) =>
                  options.stalePaths?.includes(file.path) &&
                  (!body.stale_path || body.stale_path === file.path),
              );
            }
            // A continuation keeps the saved destinations, not today's configured providers.
            if (choices && !body.resume_run) {
              const code = `code-${choices.code}`;
              const performanceChoice = choices.performance || choices.review;
              const securityChoice = choices.security || choices.review;
              const review = `review-${performanceChoice}`;
              const security =
                securityChoice === performanceChoice ? review : `security-${securityChoice}`;
              const features =
                choices.features === performanceChoice
                  ? review
                  : choices.features === securityChoice
                    ? security
                    : `features-${choices.features}`;
              state.preview.preview_id = `preview-${choices.code}-${performanceChoice}-${securityChoice}-${choices.features}`;
              state.preview.providers = [];
              for (const [id, choice, stage, scope] of [
                [code, choices.code, 'semantic', 'bug'],
                [review, performanceChoice, 'performance', 'analyze'],
                [security, securityChoice, 'security_ai', 'analyze'],
                [features, choices.features, 'feature_suggestions', 'analyze'],
              ]) {
                const provider = state.preview.providers.find((item) => item.id === id);
                if (provider) provider.stages.push(stage);
                else
                  state.preview.providers.push({
                    id,
                    stages: [stage],
                    model: { ...model(choice), scope },
                    remote_confirmation_required: model(choice).remote_provider,
                  });
              }
              state.preview.files.forEach((file) =>
                file.stages.forEach((stage) => {
                  stage.provider_id =
                    stage.stage === 'semantic'
                      ? code
                      : stage.stage === 'security_rules'
                        ? undefined
                        : stage.stage === 'security_ai'
                          ? security
                          : review;
                  if (stage.stage === 'security_rules') stage.max_model_requests = 0;
                }),
              );
              state.preview.features.provider_id = features;
            }
            state.preview.features = body.include_features
              ? {
                  ...state.preview.features,
                  expected_hash: state.features.hash,
                  excluded_paths: state.excluded,
                }
              : undefined;
            // Preview-only boundary cases do not change request/admission behavior.
            state.preview =
              options.previewOverride === null
                ? null
                : { ...state.preview, ...structuredClone(options.previewOverride || {}) };
            return response(state.preview);
          }
          if (path.endsWith('/analysis/run/control')) {
            const nextStatus = {
              pause: 'paused',
              resume: options.resumeCompletes ? 'completed' : 'running',
              cancel: 'canceled',
            }[body.action];
            state.run = {
              ...state.run,
              identity: {
                ...state.run.identity,
                generation:
                  body.action === 'resume'
                    ? `${state.run.identity.generation}-resumed`
                    : state.run.identity.generation,
              },
              status: nextStatus,
              sections: state.run.sections.map((section) => ({
                ...section,
                status: nextStatus,
              })),
            };
            return response(state.run);
          }
          if (path.endsWith('/analysis/run')) {
            if (method === 'POST') {
              if (
                ['queued', 'running', 'pausing', 'canceling'].includes(state.run?.status) ||
                (['paused', 'interrupted'].includes(state.run?.status) && !body.clear_results)
              )
                return {
                  status: 409,
                  body: JSON.stringify({
                    type: 'conflict',
                    user_message:
                      'An analysis run is already active or paused. Continue or cancel it before starting a new analysis.',
                  }),
                };
              if (body.clear_results) {
                state.clearedResults = true;
                const activeIds = new Set();
                for (const c of Object.values(state.changes || {})) {
                  if (c.state === 'draft') {
                    for (const t of c.finding_targets || []) activeIds.add(t.id);
                  }
                }
                for (const tree of state.gitWorktrees || []) {
                  if (tree.path === state.project.path) continue;
                  const childSessions = worktreeStates[tree.path]
                    ? Object.values(worktreeStates[tree.path].changes)
                    : tree.completed || [];
                  for (const c of childSessions) {
                    if (c.state === 'draft' || c.state === 'applied') {
                      for (const t of c.finding_targets || []) activeIds.add(t.id);
                    }
                  }
                }
                state.findingsList = (state.findingsList || [])
                  .filter((f) => f.status === 'in_progress' || activeIds.has(f.id))
                  .map((f) => (f.status === 'open' ? { ...f, status: 'in_progress' } : f));
                state.features = {
                  ...state.features,
                  hash: 'features-cleared',
                  status: 'not_generated',
                  failure: '',
                  suggestions: [],
                  generations: [],
                  last_generation: undefined,
                };
                const filterSec = (category, sec) => {
                  const baseSemantic =
                    sec?.semantic ?? (category === 'bugs' ? [state.finding] : []);
                  const basePerf =
                    sec?.performance ?? (category === 'performance' ? [state.performance] : []);
                  const baseSec =
                    sec?.security ?? (category === 'security' ? [state.security] : []);
                  const semantic = baseSemantic
                    .filter((f) => f.status === 'in_progress' || activeIds.has(f.id))
                    .map((f) => (f.status === 'open' ? { ...f, status: 'in_progress' } : f));
                  const performance = basePerf
                    .map((r) => ({
                      ...r,
                      findings: (r.findings || []).filter(
                        (f) => f.triage === 'in_progress' || activeIds.has(f.id),
                      ),
                    }))
                    .filter((r) => r.findings.length > 0);
                  const security = baseSec
                    .map((r) => ({
                      ...r,
                      findings: (r.findings || []).filter(
                        (f) => f.triage === 'in_progress' || activeIds.has(f.id),
                      ),
                    }))
                    .filter((r) => r.findings.length > 0);
                  return {
                    saved_finding_count: semantic.length + performance.length + security.length,
                    semantic,
                    performance,
                    security,
                    unclassified: [],
                  };
                };
                state.results = {
                  bugs: filterSec('bugs', state.results?.bugs),
                  performance: filterSec('performance', state.results?.performance),
                  security: filterSec('security', state.results?.security),
                };
              }
              const nextStatus = options.analysisCompletes ? 'completed' : 'running';
              state.run = {
                ...run,
                identity: {
                  ...queue,
                  id: body.clear_results ? 'run-cleared' : 'run-1',
                  generation: body.clear_results ? 'generation-cleared' : 'generation-1',
                },
                plan: { ...state.preview },
                status: nextStatus,
                sections: run.sections.map((sec) => ({
                  ...sec,
                  status: nextStatus,
                  finding_count:
                    body.clear_results && !options.analysisCompletes ? 0 : sec.finding_count,
                })),
                window_files_completed:
                  body.clear_results && !options.analysisCompletes ? 0 : run.window_files_completed,
              };
              if (body.include_features) {
                state.run.features = {
                  status: options.analysisCompletes
                    ? options.featuresFail
                      ? 'failed'
                      : 'completed'
                    : 'pending',
                  attempts: options.analysisCompletes ? 1 : 0,
                  suggestion_count: options.analysisCompletes && !options.featuresFail ? 1 : null,
                };
                if (options.analysisCompletes)
                  state.features = {
                    ...state.features,
                    hash: 'features-analysis',
                    status: options.featuresFail ? 'failed' : 'ready',
                    failure: options.featuresFail ? 'Feature search failed. Try again.' : '',
                    freshness: options.featuresFail ? state.features.freshness : 'current',
                    suggestions: options.featuresFail ? state.features.suggestions : [{ ...idea }],
                  };
              }
            }
            return response(state.run);
          }
          if (path.endsWith('/analysis/results')) {
            const category = url.searchParams.get('category');
            return response({
              identity: state.run.identity,
              progress: state.run.sections.find((section) => section.category === category),
              saved_finding_count: 1,
              semantic: category === 'bugs' ? [state.finding] : [],
              performance: category === 'performance' ? [state.performance] : [],
              security: category === 'security' ? [state.security] : [],
              unclassified: [],
              ...state.results[category],
            });
          }
          if (path.endsWith('/files/info'))
            return response({
              ...pathFile,
              name: pathFile.path.split('/').at(-1),
              content_hash: state.changed ? 'externally-changed' : pathFile.content_hash,
              content: state.source,
            });
          if (path.endsWith('/files/symbols'))
            return response({ ...rev, path: pathFile.path, symbols: pathFile.symbols });
          if (path.endsWith('/git'))
            return response({
              available: true,
              branch: 'main',
              file_state: 'clean',
              diff_state: 'clean',
            });
          if (path.endsWith('/files/analysis') && options.fileAnalysis !== undefined)
            return response(
              options.fileAnalysis === null ? null : { ...rev, ...options.fileAnalysis },
            );
          if (path.endsWith('/files/analysis'))
            return response({
              ...rev,
              content_hash:
                state.applied || state.changeReceipt?.state === 'applied'
                  ? Object.values(state.changes)
                      .flatMap((change) => change.changes)
                      .find((edit) => edit.path === pathFile.path)?.hash || pathFile.content_hash
                  : pathFile.content_hash,
              path: pathFile.path,
              status: 'success',
              purpose: 'Processes one queued item.',
              responsibilities: ['Receive queued work', 'Return completion status'],
              dependencies: ['context'],
              side_effects: [],
              risks: [],
              suggestions: [],
              symbol_explanations: {},
              engineering_insight: insight,
            });
          if (path.endsWith('/context'))
            return response(options.context === undefined ? context : options.context);
          if (path.endsWith('/impact') && options.impact !== undefined)
            return response(options.impact);
          if (path.endsWith('/impact'))
            return response({
              target_path: pathFile.path,
              target_symbol: 'Process',
              references: [
                {
                  path: files[2].path,
                  symbol: 'main',
                  confidence: 'lexical',
                  reason: 'Calls Process',
                },
              ],
            });
          if (path.endsWith('/files/explanation'))
            return response({
              ...rev,
              base_file_hash: files[0].content_hash,
              anchor: { path: files[0].path, symbol: 'Process' },
              summary: 'Returns after processing the work item.',
              behavior: ['Processes one item'],
              inputs: ['Context'],
              outputs: ['Error'],
              side_effects: [],
              error_behavior: [],
              context_manifest: context,
              engineering_insight: insight,
              ...options.explanation,
            });
          if (path.endsWith('/security-review') || path.endsWith('/files/security-scan'))
            return response(security);
          if (path.endsWith('/execution-trust')) {
            if (method === 'POST') state.trusted = true;
            return response({
              ...rev,
              trusted: state.trusted,
              commands: options.executionCommands || [
                ['go', 'test', './...'],
                ['go', 'vet', './...'],
              ],
            });
          }
          if (path.endsWith('/bootstrap/exec')) {
            state.trusted = true;
            const steps = [];
            if (state.files.some((f) => f.path.toLowerCase() === '.mise.toml')) {
              steps.push({
                command: 'mise trust && mise install',
                state: 'passed',
                output: 'mise ok',
              });
            }
            if (state.files.some((f) => f.path.toLowerCase() === 'lefthook.yml')) {
              steps.push({ command: 'lefthook install', state: 'passed', output: 'lefthook ok' });
            }
            return response({
              ...rev,
              steps,
            });
          }
          if (path.endsWith('/scan')) {
            if (method === 'POST')
              state.scan = {
                ...rev,
                status: 'completed',
                phases: [
                  {
                    name: 'vet',
                    state: 'passed',
                    command: ['go', 'vet', './...'],
                    output: 'No findings',
                    exit_code: 0,
                  },
                ],
                ...structuredClone(options.scanResult || {}),
              };
            if (method === 'DELETE') state.scan = { ...state.scan, ...rev, status: 'canceled' };
            return response(state.scan);
          }
          throw new Error(`Unimplemented fixture route: ${method} ${target}`);
        },
        async Cancel(id) {
          requests.push({ method: 'CANCEL', id });
        },
        async ChooseDirectory() {
          window.fixture.browsed = (window.fixture.browsed || 0) + 1;
          return options.chosenPath ?? '/fixture/harbor';
        },
        async SetUnsavedDraft(dirty) {
          window.fixture.dirty = dirty;
        },
        async OpenTerminal(root) {
          terminals.push({ action: 'open', root });
          if (window.fixture.hold === 'OpenTerminal')
            await new Promise((resolve) => {
              held = resolve;
            });
          return {
            id: String(terminals.length),
            state: 'running',
            data: '',
            cursor: 0,
            reset: false,
            error: '',
          };
        },
        async ReadTerminal(id, cursor) {
          return {
            id,
            state: 'running',
            cursor: 1,
            data: cursor ? '' : btoa('fixture shell ready\r\n$ '),
            reset: false,
            error: '',
          };
        },
        async WriteTerminal(id, data) {
          terminals.push({ action: 'input', id, data });
        },
        async ResizeTerminal() {},
        async CloseTerminal(id) {
          terminals.push({ action: 'close', id });
        },
      },
    },
  };
}
