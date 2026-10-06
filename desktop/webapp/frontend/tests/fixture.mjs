// Browser-only daemon fixture. It is never bundled into the desktop application.
export function installFixture(options = {}) {
  const identity = { project_id: 'project-1', project_revision: 'revision-1' };
  const source =
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
  const files = [
    'internal/worker/process.go',
    'internal/worker/config.go',
    'cmd/server/main.go',
    'internal/queue/queue.go',
  ].map((path) => ({
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
  const model = (scope) => ({
    scope,
    profile: scope,
    model: options.modelNames?.[scope] || 'test-model',
    provider_origin: options.remote ? 'https://provider.invalid' : 'http://127.0.0.1:11434',
    remote_provider: !!options.remote,
    reasoning_effort: 'medium',
    timeout: '2m',
  });
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
    freshness: 'fresh',
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
    title: 'Retry failed work',
    benefit: 'Let users recover failed jobs without submitting them again.',
    evidence: 'The worker queue already records failed jobs.',
    paths: [files[0].path],
    effort: 'medium',
    acceptance_criteria: ['Retry only eligible failed jobs.', 'Show the new job status.'],
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
    plan: preview,
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
    status: 'success',
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
    status: 'success',
    source: 'rules',
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
  const state = {
    project,
    files,
    finding,
    run: options.empty ? null : run,
    preview,
    context,
    performance,
    security,
    trusted: false,
    changes: {},
    changeReceipt: null,
    features: {
      ...identity,
      hash: 'features-empty',
      goals: '',
      status: options.featuresReady ? (options.featuresFail ? 'failed' : 'ready') : 'not_generated',
      freshness: options.featuresStale ? 'stale' : 'current',
      suggestions:
        options.featuresReady && !options.featuresEmpty
          ? [
              {
                ...idea,
                generation_id: options.legacyShape ? undefined : 'gen-1',
                freshness: options.legacyShape
                  ? undefined
                  : options.featuresStale
                    ? 'stale'
                    : 'current',
              },
            ]
          : [],
      generations: options.legacyShape
        ? undefined
        : options.featuresReady
          ? [
              {
                id: 'gen-1',
                timestamp: '2026-10-04T01:00:00Z',
                goals: '',
                provider_id: 'provider-1',
                model_summary: {
                  profile: 'analyze',
                  model: model('analyze').model,
                  provider_origin: model('analyze').provider_origin,
                  remote_provider: model('analyze').remote_provider,
                },
              },
            ]
          : [],
      last_generation: options.legacyShape ? undefined : options.featuresReady ? 'gen-1' : '',
      failure:
        options.featuresReady && options.featuresFail ? 'Feature search failed. Try again.' : '',
      context_manifest: context,
    },
    instructions: {
      'AGENTS.md': '# Project rules\n\nPreserve public APIs.\n',
      'internal/AGENTS.md': '# Internal rules\n\nPropagate cancellation.\n',
    },
    draft: undefined,
    session: undefined,
    applied: false,
    excluded: [],
    selectionId: 'selection-1',
    source,
    changed: false,
  };
  const requests = [],
    terminals = [],
    failures = {};
  let held = null;
  window.fixture = {
    state,
    options,
    requests,
    terminals,
    failures,
    release: () => held?.(),
    hold: '',
  };
  localStorage.setItem('mini-orca:last-project', project.path);
  const sameDraft = (d) => ({
    ...identity,
    draft_id: d.id,
    draft_revision: d.revision,
    draft_hash: d.hash,
    base_file_hash: d.base_file_hash,
    target_path: d.target_path,
  });
  const validation = () => ({
    applicable: true,
    scope_mode: 'single_declaration',
    diagnostics: [],
    diff: {
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
    },
  });
  window.go = {
    main: {
      Desktop: {
        async Request(id, method, target, payload) {
          const url = new URL(target, 'http://fixture');
          const path = url.pathname;
          const body = payload ? JSON.parse(payload) : undefined;
          requests.push({ id, method, path, query: Object.fromEntries(url.searchParams), body });
          if (window.fixture.hold === path)
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
          const rev = { ...identity, project_revision: state.project.project_revision };
          const pathFile =
            state.files.find((f) => f.path === (url.searchParams.get('path') || body?.path)) ||
            state.files[0];
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
                    timestamp: '2026-10-04T01:05:00Z',
                    goals: body.goals,
                    provider_id: 'provider-1',
                    model_summary: {
                      profile: body.profile || 'analyze',
                      model: model(body.profile || 'analyze').model,
                      provider_origin: model(body.profile || 'analyze').provider_origin,
                      remote_provider: model(body.profile || 'analyze').remote_provider,
                    },
                  },
                ];
                state.features.last_generation = newGenId;
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
              ([file]) => file === 'AGENTS.md' || file === target,
            );
            return response({
              ...rev,
              path: target,
              exists: target in state.instructions,
              existing_content: state.instructions[target] || '',
              effective: {
                files: inherited.map(([path, content]) => ({
                  path,
                  content,
                  hash: 'guide',
                  scope: path === 'AGENTS.md' ? '.' : 'internal',
                })),
                excluded: [],
                fingerprint: 'guide-1',
              },
              presets: [
                {
                  id: 'focused',
                  label: 'Keep changes focused',
                  content: 'Preserve unrelated work.',
                },
                {
                  id: 'tests',
                  label: 'Test meaningful behavior',
                  content: 'Test behavior and error paths.',
                },
              ],
            });
          }
          if (path === '/api/projects/current/instructions/proposal') {
            const change = {
              ...rev,
              id: `change-${Object.keys(state.changes).length + 1}`,
              kind: 'instructions',
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
            state.changes[change.id] = change;
            return response(change);
          }
          if (path === '/api/projects/current/changes') {
            if (method === 'GET') return response(Object.values(state.changes));
            const change = {
              ...rev,
              id: `change-${Object.keys(state.changes).length + 1}`,
              kind: body.kind,
              title: body.title,
              acceptance_criteria: body.acceptance_criteria || [],
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
                  content: 'The proposal handles cancellation and preserves its scope.',
                },
              );
              change.changes = change.targets.map((target) => ({
                path: target.path,
                content: source.replace('return nil', 'return ctx.Err()'),
                hash: `candidate-${change.revision}`,
                diff: validation().diff,
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
                  output: options.changeChecksFail ? 'Fixture test failure' : '',
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
              return response(state.changeReceipt.verification);
            }
            if (action === 'apply' || action === 'undo') {
              state.trusted = false;
              change.state = action === 'apply' ? 'applied' : 'undone';
              state.project.project_revision = action === 'apply' ? 'revision-2' : 'revision-3';
              state.source = action === 'apply' ? change.changes[0].content : source;
              if (change.kind === 'instructions')
                for (const edit of change.changes)
                  state.instructions[edit.path] =
                    action === 'apply'
                      ? edit.content
                      : change.targets.find((target) => target.path === edit.path).content;
              state.changeReceipt = {
                project_id: identity.project_id,
                project_revision: state.project.project_revision,
                session_id: change.id,
                state: change.state,
                hash: change.hash,
                undo_available: action === 'apply',
                index: {
                  ...identity,
                  project_revision: state.project.project_revision,
                  files: state.files,
                },
                warnings:
                  action === 'apply' && options.mutationWarning ? [options.mutationWarning] : [],
              };
              return response(state.changeReceipt);
            }
          }
          if (path === '/status')
            return response({
              status: 'running',
              version: 'fixture',
              workflow: 'single_coder_preview',
            });
          if (path === '/api/models/current')
            return response({
              scopes: { analyze: model('analyze'), bug: model('bug'), function: model('function') },
            });
          if (path === '/api/projects/restore' || path === '/api/projects/import')
            return response(state.project);
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
            return response({ ...rev, files: state.files });
          if (path.endsWith('/findings'))
            return response({ ...rev, findings: options.empty ? [] : [state.finding] });
          if (path.includes('/findings/')) {
            state.finding.status = body.status;
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
              editable: !['running', 'paused'].includes(state.run?.status),
              files: files.map((f) => ({
                path: f.path,
                reason: '',
                stages: stages.map((stage) => ({ stage, status: 'fresh', reason: '' })),
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
            const choices = body.resume_run ? state.run.plan.models : body.models;
            state.preview.models = choices;
            if (body.recover_incomplete) {
              state.preview.recover_incomplete = true;
            }
            if (choices) {
              const code = `code-${choices.code}`;
              const review = `review-${choices.review}`;
              const features =
                choices.features === choices.review ? review : `features-${choices.features}`;
              state.preview.preview_id = `preview-${choices.code}-${choices.review}-${choices.features}`;
              state.preview.providers = [
                {
                  id: code,
                  stages: ['semantic'],
                  model: { ...model(choices.code), scope: 'bug' },
                  remote_confirmation_required: !!options.remote,
                },
                {
                  id: review,
                  stages: [
                    'performance',
                    'security_ai',
                    ...(features === review ? ['feature_suggestions'] : []),
                  ],
                  model: { ...model(choices.review), scope: 'analyze' },
                  remote_confirmation_required: !!options.remote,
                },
                ...(features === review
                  ? []
                  : [
                      {
                        id: features,
                        stages: ['feature_suggestions'],
                        model: { ...model(choices.features), scope: 'analyze' },
                        remote_confirmation_required: !!options.remote,
                      },
                    ]),
              ];
              state.preview.files.forEach((file) =>
                file.stages.forEach((stage) => {
                  stage.provider_id =
                    stage.stage === 'semantic'
                      ? code
                      : stage.stage === 'security_rules'
                        ? undefined
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
            return response(state.preview);
          }
          if (path.endsWith('/analysis/run/control')) {
            state.run.status = { pause: 'paused', resume: 'running', cancel: 'canceled' }[
              body.action
            ];
            return response(state.run);
          }
          if (path.endsWith('/analysis/run')) {
            if (method === 'POST') {
              state.run = {
                ...run,
                plan: { ...state.preview },
                status: options.analysisCompletes ? 'completed' : 'running',
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
            return response({ ...rev, path: pathFile.path, symbols: [symbol] });
          if (path.endsWith('/git'))
            return response({
              available: true,
              branch: 'main',
              file_state: 'clean',
              diff_state: 'clean',
            });
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
          if (path.endsWith('/context')) return response(context);
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
            });
          if (path.endsWith('/security-review') || path.endsWith('/files/security-scan'))
            return response(security);
          if (path.endsWith('/chat/sessions')) {
            state.session = {
              ...rev,
              id: 'session-1',
              base_file_hash: body.base_file_hash,
              open_path: body.open_path,
              mode: body.mode,
              target_symbol: body.target_symbol,
              state: 'open',
              latest_draft_id: '',
            };
            return response(state.session);
          }
          if (path.endsWith('/messages')) {
            state.draft = {
              ...rev,
              id: 'draft-1',
              base_file_hash: files[0].content_hash,
              target_path: files[0].path,
              target_symbol: state.session.target_symbol,
              mode: state.session.mode,
              declaration: `func ${state.session.target_symbol}(ctx context.Context) error {\n\treturn ctx.Err()\n}`,
              imports: ['context'],
              revision: 1,
              hash: 'draft-hash-1',
              candidate_hash: 'candidate-1',
              state: 'generated',
              engineering_insight: insight,
            };
            return response({
              session_id: 'session-1',
              draft: state.draft,
              assistant_message: {
                role: 'assistant',
                content: options.hostile
                  ? '<img src="https://evil.invalid/tracker" onerror="alert(1)"> [link](https://evil.invalid)'
                  : 'The draft returns the context error.',
              },
              context_manifest: context,
            });
          }
          if (path.endsWith('/drafts/draft-1') && method === 'PATCH') {
            state.draft = {
              ...state.draft,
              revision: state.draft.revision + 1,
              hash: 'draft-hash-' + (state.draft.revision + 1),
              declaration: body.declaration,
              imports: body.imports,
              validation: undefined,
            };
            return response(state.draft);
          }
          if (path.endsWith('/validate')) {
            state.draft.validation = validation();
            state.draft.state = 'validated';
            return response(state.draft);
          }
          if (path.endsWith('/checks'))
            return response({
              ...sameDraft(state.draft),
              candidate_hash: 'candidate-1',
              applicable: !options.checkFail,
              checks: [
                { name: 'parse', required: true, state: 'passed', command: [], output: '' },
                {
                  name: 'format',
                  required: true,
                  state: options.checkFail ? 'failed' : 'passed',
                  command: ['gofmt'],
                  output: options.checkFail ? 'Formatting failed' : '',
                },
                ...(body.run_tests
                  ? [
                      {
                        name: 'tests',
                        required: false,
                        state: 'passed',
                        command: ['go', 'test', './...'],
                        output: 'ok worker',
                      },
                    ]
                  : []),
              ],
            });
          if (path.endsWith('/execution-trust')) {
            if (method === 'POST') state.trusted = true;
            return response({
              ...rev,
              trusted: state.trusted,
              commands: [
                ['go', 'test', './...'],
                ['go', 'vet', './...'],
              ],
            });
          }
          if (path.endsWith('/benchmarks'))
            return response(
              method === 'GET'
                ? {
                    ...sameDraft(state.draft),
                    available: true,
                    trusted: state.trusted,
                    benchmarks: [
                      {
                        name: 'BenchmarkProcess',
                        scope: 'worker',
                        command: ['go', 'test', '-bench=BenchmarkProcess'],
                      },
                    ],
                  }
                : {
                    ...sameDraft(state.draft),
                    benchmark: body.benchmark,
                    scope: body.expected_scope,
                    status: 'completed',
                    command: ['go', 'test', '-bench=BenchmarkProcess'],
                    base: {
                      samples: [
                        { iterations: 1000, ns_per_op: 250, bytes_per_op: 24, allocs_per_op: 1 },
                      ],
                    },
                    candidate: {
                      samples: [
                        { iterations: 1000, ns_per_op: 200, bytes_per_op: 0, allocs_per_op: 0 },
                      ],
                    },
                  },
            );
          if (path.endsWith('/apply') || path.endsWith('/undo')) {
            const undo = path.endsWith('/undo');
            state.applied = !undo;
            state.project.project_revision = undo ? 'revision-3' : 'revision-2';
            state.files[0].content_hash = undo ? 'restored-hash' : 'applied-hash';
            state.source = undo ? source : source.replace('return nil', 'return ctx.Err()');
            return response({
              project_revision: state.project.project_revision,
              post_apply_hash: state.files[0].content_hash,
              undo_available: !undo,
              audit: {
                id: 'receipt-1',
                action: undo ? 'undo' : 'apply',
                target_path: files[0].path,
                outcome: 'success',
                timestamp: '2026-10-04T05:00:00Z',
                before_hash: 'base',
                after_hash: state.files[0].content_hash,
              },
              index: {
                ...rev,
                project_revision: state.project.project_revision,
                files: state.files,
              },
              warnings: [],
            });
          }
          if (path.endsWith('/scan'))
            return response(
              method === 'GET'
                ? null
                : {
                    ...rev,
                    status: method === 'DELETE' ? 'canceled' : 'completed',
                    phases: [
                      {
                        name: 'vet',
                        state: 'passed',
                        command: ['go', 'vet', './...'],
                        output: 'No findings',
                        exit_code: 0,
                      },
                    ],
                  },
            );
          throw new Error(`Unimplemented fixture route: ${method} ${target}`);
        },
        async Cancel(id) {
          requests.push({ method: 'CANCEL', id });
        },
        async ChooseDirectory() {
          return '/fixture/harbor';
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
