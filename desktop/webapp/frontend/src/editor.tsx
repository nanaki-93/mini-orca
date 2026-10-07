import { useEffect, useRef, useState } from 'react';
import { canApply, currentDraft, workspace as w, type State, type Page } from './workspace';
import {
  Badge,
  Button,
  BulletContent,
  Disclosure,
  Empty,
  Go,
  Heading,
  Icon,
  InsightCard,
  KeyValues,
  Notice,
  Panel,
  Prose,
} from './ui';

export function Editor({ s }: { s: State }) {
  const [filter, setFilter] = useState('');
  const [limit, setLimit] = useState(100);
  const files = (s.index?.files || []).filter((file) =>
    file.path.toLowerCase().includes(filter.toLowerCase()),
  );
  const tabs: [Page, string][] = [
    ['editor', 'Source'],
    ['context', 'Context'],
    ['assistant', 'Assistant'],
    ['draft', 'Draft'],
    ['checks', 'Checks'],
    ['review', 'Review'],
  ];
  return (
    <div className="workspace-page source-workspace">
      <Heading variant="intro" title={s.file?.name || 'Source'} detail={s.file?.path}>
        <Go page="chat" icon="sparkles">
          Implement in chat
        </Go>
        <Go page="search" icon="search">
          Find file
        </Go>
        {s.draft && (
          <Button disabled={!!s.busy} onClick={() => void w.discard()}>
            Discard draft
          </Button>
        )}
      </Heading>
      <div className="editor-workspace">
        <aside className="file-browser panel">
          <div className="panel-head">
            <h2>Files</h2>
            <span className="muted small">{files.length}</span>
          </div>
          <div className="browser-search">
            <input
              aria-label="Filter source files"
              placeholder="Filter files…"
              value={filter}
              onChange={(e) => {
                setFilter(e.target.value);
                setLimit(100);
              }}
            />
          </div>
          <div className="file-list">
            {files.slice(0, limit).map((file) => (
              <button
                title={file.path}
                key={file.path}
                className={`file-item ${s.file?.path === file.path ? 'active' : ''}`}
                disabled={!!s.busy}
                onClick={() => void w.openFile(file.path)}
              >
                <Icon name="file" />
                <span>
                  <strong>{file.path.split('/').at(-1)}</strong>
                  <small>
                    {file.path.includes('/')
                      ? file.path.slice(0, file.path.lastIndexOf('/'))
                      : 'Project root'}
                  </small>
                </span>
              </button>
            ))}
            {files.length > limit && (
              <Button onClick={() => setLimit(limit + 100)}>Show more</Button>
            )}
          </div>
        </aside>
        <div className="editor-content">
          {s.file ? (
            <>
              <div className="tabs" role="tablist" aria-label="Editor workflow">
                {tabs.map(([page, label]) => (
                  <button
                    role="tab"
                    aria-selected={
                      s.page === page ||
                      (page === 'context' && s.page === 'manifest') ||
                      (page === 'assistant' && s.page === 'new-declaration')
                    }
                    className={`tab ${s.page === page || (page === 'context' && s.page === 'manifest') || (page === 'assistant' && s.page === 'new-declaration') ? 'active' : ''}`}
                    disabled={['draft', 'checks', 'review'].includes(page) && !s.draft}
                    key={page}
                    onClick={() => void (page === 'review' ? w.review() : w.navigate(page))}
                  >
                    {label}
                    {page === 'draft' && s.dirty && (
                      <span className="edited-dot" aria-label="Edited" />
                    )}
                  </button>
                ))}
              </div>
              {s.fileStale && (
                <Notice>
                  File evidence is outdated.{' '}
                  <Button tone="ghost small" disabled={!!s.busy} onClick={() => void w.reindex()}>
                    Refresh project
                  </Button>
                </Notice>
              )}
              {s.page === 'editor' && <Source s={s} />}
              {['context', 'manifest'].includes(s.page) && <Context s={s} />}
              {['assistant', 'new-declaration'].includes(s.page) && (
                <Assistant key={`${s.file.path}:${s.page}`} s={s} />
              )}
              {s.page === 'draft' && <DraftEditor s={s} />}
              {s.page === 'checks' && <Checks s={s} />}
              {s.page === 'review' && <Review s={s} />}
            </>
          ) : (
            <Empty title="Choose a file" icon="code" />
          )}
        </div>
      </div>
    </div>
  );
}
function DeclarationPicker({ s }: { s: State }) {
  return (
    <div className="row wrap declaration-picker">
      <label className="sr-only" htmlFor="symbol-picker">
        Declaration
      </label>
      <select
        className="field symbol-picker"
        id="symbol-picker"
        disabled={!!s.busy || s.fileStale}
        value={s.symbol}
        onChange={(e) => void w.selectSymbol(e.target.value)}
      >
        <option value="">Choose a declaration…</option>
        {s.symbols.map((symbol, i) => (
          <option
            value={symbol.name}
            key={`${symbol.name}:${i}`}
            disabled={!symbol.atomic_target || symbol.confidence !== 'exact'}
          >
            {symbol.name} · {symbol.kind}
            {!symbol.atomic_target || symbol.confidence !== 'exact' ? ' · read-only' : ''}
          </option>
        ))}
      </select>
      <Go page="new-declaration" icon="plus">
        New declaration
      </Go>
    </div>
  );
}
function Source({ s }: { s: State }) {
  const code = useRef<HTMLPreElement>(null);
  const symbol = s.symbols.find((value) => value.name === s.symbol);
  useEffect(() => {
    code.current?.querySelector('.selected-line')?.scrollIntoView({ block: 'nearest' });
  }, [s.symbol]);
  return (
    <div className="stack source-inspection">
      <DeclarationPicker s={s} />
      <div className="source-panel panel">
        <div className="code-header">
          <span className="row">
            <Icon name="lock" />
            Read-only source
          </span>
          <span>
            {s.file!.line_count} lines · {s.file!.language}
          </span>
        </div>
        {s.file!.binary ? (
          <Empty title="Binary file" />
        ) : (
          <pre className="source-code" aria-label="Read-only source" tabIndex={0} ref={code}>
            {s.file!.content.split('\n').map((line, i) => (
              <span
                className={`source-line ${symbol && i + 1 >= symbol.start_line && i + 1 <= symbol.end_line ? 'selected-line' : ''}`}
                key={i}
              >
                <span className="line-number" aria-hidden="true">
                  {i + 1}
                </span>
                <code>{line || ' '}</code>
              </span>
            ))}
          </pre>
        )}
      </div>
      <div className="actions">
        <Go page="assistant" tone="primary" icon="sparkles">
          Prepare a change
        </Go>
        <Button disabled={!!s.busy || !s.symbol || s.fileStale} onClick={() => void w.explain()}>
          Explain declaration
        </Button>
        <Button disabled={!!s.busy || s.fileStale} onClick={() => void w.analyzeFile()}>
          Analyze file
        </Button>
        <Go page="security" icon="shield">
          Security
        </Go>
      </div>
      {s.fileAnalysis && (
        <Panel
          className="source-analysis"
          title="File analysis"
          actions={<Badge value={s.fileAnalysis.status} />}
        >
          <Prose text={s.fileAnalysis.purpose || s.fileAnalysis.failure} />
          <BulletContent title="Responsibilities" items={s.fileAnalysis.responsibilities} />
          <Disclosure title="Dependencies & side effects">
            <BulletContent title="Dependencies" items={s.fileAnalysis.dependencies} />
            <BulletContent title="Side effects" items={s.fileAnalysis.side_effects} />
          </Disclosure>
          {(s.fileAnalysis.risks || []).map((risk, i) => (
            <Disclosure
              key={i}
              title={
                <span className="row wrap">
                  <Badge value={risk.severity} />
                  {risk.summary}
                </span>
              }
            >
              <InsightCard insight={risk.engineering_insight} />
              {risk.task_spec && (
                <Button
                  onClick={() => {
                    w.set({ task: risk.task_spec, symbol: risk.task_spec!.target_symbol });
                    void w.navigate('assistant');
                  }}
                >
                  Prepare change
                </Button>
              )}
            </Disclosure>
          ))}
          {(s.fileAnalysis.suggestions || []).map((suggestion, i) => (
            <Disclosure key={i} title={suggestion.title}>
              <Prose text={suggestion.summary} />
            </Disclosure>
          ))}
        </Panel>
      )}
      <InsightCard insight={s.fileAnalysis?.engineering_insight} />
    </div>
  );
}
function Context({ s }: { s: State }) {
  const context = s.context;
  if (!context)
    return (
      <Empty title="Context unavailable">
        <Button disabled={!!s.busy} onClick={() => void w.inspectContext()}>
          Refresh context
        </Button>
      </Empty>
    );
  return (
    <div className="stack context-inspection">
      <Panel
        title="Context"
        actions={
          <Button tone="small" disabled={!!s.busy} onClick={() => void w.inspectContext()}>
            Refresh
          </Button>
        }
      >
        <div className="mini-metrics">
          <div>
            <strong>{context.included?.length ?? '—'}</strong>
            <small>Included files</small>
          </div>
          <div>
            <strong>{context.estimated_tokens?.toLocaleString() ?? '—'}</strong>
            <small>Estimated tokens</small>
          </div>
          <div>
            <strong>{context.excluded?.length ?? '—'}</strong>
            <small>Excluded</small>
          </div>
        </div>
        <KeyValues
          values={[
            ['Model', context.model],
            ['Destination', context.provider_origin],
            ['Scope', context.scope],
            ['Truncated', context.truncated === undefined ? '—' : context.truncated ? 'Yes' : 'No'],
          ]}
        />
      </Panel>
      <Panel title="Included files">
        <div className="table-wrap" tabIndex={0} aria-label="Included context files">
          <table>
            <thead>
              <tr>
                <th>Path</th>
                <th>Bytes</th>
                <th>Tokens</th>
              </tr>
            </thead>
            <tbody>
              {(context.included || []).map((file) => (
                <tr key={file.path}>
                  <td className="path">
                    {file.path}
                    {file.truncated && <Badge value="truncated" />}
                  </td>
                  <td>{file.size_bytes.toLocaleString()}</td>
                  <td>{file.estimated_tokens?.toLocaleString() ?? '—'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <Disclosure title="Content hashes">
          {context.included?.map((file) => (
            <div className="list-row" key={file.path}>
              <span className="list-copy">
                <strong>{file.path}</strong>
                <small className="mono">{file.hash}</small>
              </span>
            </div>
          ))}
        </Disclosure>
      </Panel>
      <Panel title="Excluded">
        <div className="scroll-list">
          {context.excluded?.map((file) => (
            <div className="list-row" key={file.path}>
              <span className="list-copy">
                <strong>{file.path}</strong>
                <small>{file.reason}</small>
              </span>
            </div>
          ))}
        </div>
      </Panel>
      {s.impact && (
        <Panel title="Related declarations" className="context-references">
          {s.impact.references.length ? (
            s.impact.references.map((item, i) => (
              <div className="list-row" key={i}>
                <span className="list-copy">
                  <strong>
                    {item.path} · {item.symbol}
                  </strong>
                  <small>{item.reason}</small>
                </span>
                <Badge value={item.confidence} tone="violet" />
              </div>
            ))
          ) : (
            <p className="muted small">No references found.</p>
          )}
        </Panel>
      )}
    </div>
  );
}
function Assistant({ s }: { s: State }) {
  const create = s.page === 'new-declaration';
  const [message, setMessage] = useState('');
  const [name, setName] = useState('');
  const [kind, setKind] = useState('function');
  const [constraints, setConstraints] = useState('');
  const submit = () => {
    const content = `${create ? `Create one ${kind} named ${name}.\n\n` : ''}${message.trim()}${constraints.trim() ? `\n\nConstraints:\n${constraints.trim()}` : ''}`;
    void w.generate(content, create ? 'create_symbol' : 'replace_symbol', create ? name : s.symbol);
  };
  return (
    <div className="stack assistant-composition">
      {!create && <DeclarationPicker s={s} />}
      {create && (
        <Panel title="New declaration">
          <div className="form-grid">
            <label>
              Kind
              <select className="field" value={kind} onChange={(e) => setKind(e.target.value)}>
                {['function', 'type', 'var', 'const'].map((value) => (
                  <option key={value}>{value}</option>
                ))}
              </select>
            </label>
            <label>
              Name
              <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="DeclarationName"
                spellCheck={false}
              />
            </label>
          </div>
        </Panel>
      )}
      {s.explanation && (
        <Panel title={`About ${s.explanation.anchor.symbol}`}>
          <Prose text={s.explanation.summary} />
          <BulletContent title="Behavior" items={s.explanation.behavior} />
          <Disclosure title="Inputs, outputs & side effects">
            <BulletContent title="Inputs" items={s.explanation.inputs} />
            <BulletContent title="Outputs" items={s.explanation.outputs} />
            <BulletContent title="Side effects" items={s.explanation.side_effects} />
            <BulletContent title="Errors" items={s.explanation.error_behavior} />
          </Disclosure>
          <InsightCard insight={s.explanation.engineering_insight} />
        </Panel>
      )}
      {s.messages.map((entry, i) => (
        <Panel key={i} title={entry.role === 'user' ? 'You' : 'Assistant'}>
          <Prose text={entry.content} />
        </Panel>
      ))}
      <Panel
        className="assistant-request"
        title={
          create ? 'What should it do?' : s.symbol ? `Change ${s.symbol}` : 'Describe your change'
        }
      >
        <div className="actions">
          {['Fix', 'Refactor', 'Document'].map((preset) => (
            <Button
              key={preset}
              tone="small"
              disabled={!!s.busy}
              onClick={() =>
                setMessage(
                  `${preset}${preset === 'Refactor' || preset === 'Document' ? ' without changing behavior' : ' a bug'}: `,
                )
              }
            >
              {preset}
            </Button>
          ))}
        </div>
        <label className="sr-only" htmlFor="change-intent">
          Change request
        </label>
        <textarea
          id="change-intent"
          className="composer"
          value={message}
          disabled={!!s.busy}
          onChange={(e) => setMessage(e.target.value)}
          placeholder="Describe the change…"
        />
        <Disclosure title="Constraints">
          <textarea
            aria-label="Change constraints"
            value={constraints}
            disabled={!!s.busy}
            onChange={(e) => setConstraints(e.target.value)}
            placeholder="Optional boundaries"
          />
        </Disclosure>
        <div className="row between wrap assistant-request-actions">
          <Go page="context" icon="layers" tone="ghost">
            Inspect context
          </Go>
          <Button
            tone="primary"
            icon="sparkles"
            disabled={
              !!s.busy || s.fileStale || !message.trim() || (create ? !name.trim() : !s.symbol)
            }
            onClick={submit}
          >
            Prepare draft
          </Button>
        </div>
      </Panel>
      {s.draft && <Go page="draft">Open current draft</Go>}
    </div>
  );
}
function DraftEditor({ s }: { s: State }) {
  const d = s.draft;
  if (!d)
    return (
      <Empty title="No draft yet">
        <Go page="assistant">Prepare a change</Go>
      </Empty>
    );
  return (
    <div className="stack draft-workspace">
      <Panel
        title="Draft target"
        actions={
          <Badge value={s.dirty ? 'edited' : d.validation?.applicable ? 'validated' : d.state} />
        }
      >
        <KeyValues
          values={[
            ['File', <span className="mono">{d.target_path}</span>],
            ['Declaration', <span className="mono">{d.target_symbol}</span>],
            ['Revision', `${d.revision}${s.dirty ? ' + local edits' : ''}`],
          ]}
        />
      </Panel>
      <div className="panel">
        <div className="code-header">Declaration draft</div>
        <textarea
          aria-label="Declaration draft"
          className="draft-code"
          spellCheck={false}
          disabled={!!s.busy || s.fileStale}
          value={s.declaration}
          onChange={(event) => w.editDraft(event.target.value, s.imports)}
        />
      </div>
      <Panel className="draft-imports">
        <Disclosure title="Imports">
          <label htmlFor="draft-imports" className="field-label">
            One import per line
          </label>
          <textarea
            id="draft-imports"
            className="mono"
            spellCheck={false}
            disabled={!!s.busy || s.fileStale}
            value={s.imports}
            onChange={(event) => w.editDraft(s.declaration, event.target.value)}
          />
        </Disclosure>
      </Panel>
      {!s.dirty && !!d.validation?.diagnostics?.length && (
        <Panel title="Validation" className="draft-validation">
          {d.validation.diagnostics.map((diagnostic, i) => (
            <Notice error key={i}>
              {diagnostic.message}
            </Notice>
          ))}
        </Panel>
      )}
      <div className="actions">
        <Button
          tone="primary"
          icon="check"
          disabled={!!s.busy || s.fileStale}
          onClick={() => void w.validate()}
        >
          Validate draft
        </Button>
        {currentDraft(s) && (
          <Go page="checks" icon="arrow">
            Continue to checks
          </Go>
        )}
        <Go page="assistant">Refine with assistant</Go>
      </div>
      <InsightCard insight={d.engineering_insight} />
    </div>
  );
}
function Checks({ s }: { s: State }) {
  const [lint, setLint] = useState(false);
  const [tests, setTests] = useState(false);
  return (
    <div className="stack checks-workspace">
      <Panel
        className="checks-controls"
        title="Checks"
        actions={s.checks && <Badge value={s.checks.applicable ? 'passed' : 'needs_attention'} />}
      >
        <div className="row wrap">
          <label className="checkbox-line">
            <input
              type="checkbox"
              checked={lint}
              onChange={(e) => setLint(e.target.checked)}
              disabled={!!s.busy}
            />
            Lint
          </label>
          <label className="checkbox-line">
            <input
              type="checkbox"
              checked={tests}
              onChange={(e) => setTests(e.target.checked)}
              disabled={!!s.busy}
            />
            Tests
          </label>
          <span className="small muted">Parse & format (required)</span>
        </div>
        <div className="actions">
          <Button
            tone="primary"
            disabled={!!s.busy || !currentDraft(s)}
            onClick={() => void w.runChecks(lint, tests)}
          >
            Run checks
          </Button>
          <Button
            disabled={!!s.busy || !currentDraft(s) || !s.checks?.applicable}
            onClick={() => void w.review()}
          >
            Review change
          </Button>
          <Button disabled={!!s.busy || !currentDraft(s)} onClick={() => void w.benchmarks()}>
            Benchmarks
          </Button>
        </div>
      </Panel>
      {s.checks ? (
        s.checks.checks.map((check, i) => (
          <Panel
            className="check-evidence"
            title={check.name}
            actions={<Badge value={check.state} />}
            key={i}
          >
            <div className="row between wrap">
              <span className="small muted">{check.required ? 'Required' : 'Optional'}</span>
              <span className="small muted">
                {check.exit_code ? `Exit ${check.exit_code}` : ''}
              </span>
            </div>
            {check.command?.length ? (
              <pre className="command" aria-label={`${check.name} command`} tabIndex={0}>
                {check.command.join(' ')}
              </pre>
            ) : null}
            {check.output && (
              <Disclosure title="Output">
                <pre aria-label={`${check.name} output`} tabIndex={0}>
                  {check.output}
                </pre>
              </Disclosure>
            )}
          </Panel>
        ))
      ) : (
        <Empty title={currentDraft(s) ? 'No checks yet' : 'Draft validation required'} />
      )}
      {s.checks && !s.checks.applicable && (
        <div className="actions">
          <Button
            disabled={!!s.busy}
            onClick={() =>
              void w.generate(
                'Repair this draft using the failed check diagnostics. Preserve the task scope.',
                s.draft!.mode as 'replace_symbol' | 'create_symbol',
                s.draft!.target_symbol,
                true,
              )
            }
          >
            Repair with assistant
          </Button>
        </div>
      )}
    </div>
  );
}
function Review({ s }: { s: State }) {
  const d = s.draft;
  if (!d) return <Empty title="No draft to review" />;
  const diff = d.validation?.diff;
  return (
    <div className="stack review-workspace">
      <Panel
        title="Review change"
        actions={<Badge value={canApply(s) ? 'ready_to_apply' : 'checks_required'} />}
      >
        <p className="small muted review-target">
          {d.target_path} · {d.target_symbol}
        </p>
      </Panel>
      {s.dirty && <Notice>Local edits need validation.</Notice>}
      {!s.dirty && diff ? (
        <div className="panel diff" aria-label="Read-only composed diff" tabIndex={0}>
          <div className="code-header">
            <span className="row">
              <Icon name="lock" />
              Composed diff
            </span>
            <span>1 file</span>
          </div>
          <pre tabIndex={0} aria-label="Composed diff lines">
            {diff.lines.map((line, i) => (
              <span className={`diff-line ${line.kind}`} key={i}>
                <span className="line-number" aria-hidden="true">
                  {line.old_line || ''}
                </span>
                <span className="line-number" aria-hidden="true">
                  {line.new_line || ''}
                </span>
                <span className="diff-sign" aria-hidden="true">
                  {line.kind === 'added' ? '+' : line.kind === 'removed' ? '−' : ' '}
                </span>
                <code>{line.text || ' '}</code>
              </span>
            ))}
          </pre>
        </div>
      ) : (
        <Empty title="Diff requires draft validation" />
      )}
      <Panel title="Check evidence">
        {s.checks ? (
          <div className="row wrap">
            {s.checks.checks.map((check, i) => (
              <span className="row" key={i}>
                <span className="small">{check.name}</span>
                <Badge value={check.state} />
              </span>
            ))}
          </div>
        ) : (
          <p className="small muted">No current checks.</p>
        )}
      </Panel>
      <div className="actions end review-actions">
        <Go page="draft">Edit draft</Go>
        <Go page="checks">Checks</Go>
        <Button
          tone="primary"
          icon="check"
          disabled={!!s.busy || !canApply(s)}
          onClick={() => void w.apply()}
        >
          Apply change
        </Button>
      </div>
      <Disclosure title="Change identity">
        <KeyValues
          values={[
            ['Project revision', d.project_revision],
            ['Base file', d.base_file_hash],
            ['Draft', d.id],
            ['Draft revision', d.revision],
            ['Draft hash', d.hash],
            ['Candidate', d.candidate_hash],
          ]}
        />
      </Disclosure>
      <InsightCard insight={d.engineering_insight} />
    </div>
  );
}
