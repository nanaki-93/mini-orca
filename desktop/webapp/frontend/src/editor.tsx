import { useEffect, useRef, useState } from 'react';
import { activeChangeWorkflow, workspace as w, type State } from './workspace';
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
  return (
    <div className="workspace-page source-workspace">
      <Heading variant="intro" title={s.file?.name || 'Source'} detail={s.file?.path}>
        <Go page="search" icon="search">
          Find file
        </Go>
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
              {s.fileStale && (
                <Notice>
                  File evidence is outdated.{' '}
                  <Button tone="ghost small" disabled={!!s.busy} onClick={() => void w.reindex()}>
                    Refresh project
                  </Button>
                </Notice>
              )}
              <Source s={s} />
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
    <div className="source-inspection">
      <div className="source-document">
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
          <Button
            tone="primary"
            icon="sparkles"
            disabled={
              !!s.busy ||
              s.fileStale ||
              s.file!.binary ||
              !/\.(go|md)$/.test(s.file!.path) ||
              activeChangeWorkflow(s.change)
            }
            onClick={() =>
              w.seedChange({
                title: `Update ${s.symbol || s.file!.name}`,
                kind: 'feature',
                acceptance_criteria: [],
                paths: [s.file!.path],
                message: s.symbol ? `Update ${s.symbol}: ` : '',
              })
            }
          >
            Draft change in Chat
          </Button>
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
      </div>
      <aside className="source-context" aria-label="File insights">
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
                    disabled={!!s.busy || s.fileStale || activeChangeWorkflow(s.change)}
                    onClick={() => {
                      w.seedWorkflow({
                        title: risk.summary,
                        kind: 'fix',
                        paths: [s.file!.path],
                        message: risk.summary,
                        acceptance_criteria: risk.task_spec!.acceptance_criteria,
                      });
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
      </aside>
    </div>
  );
}
export function Context({ s }: { s: State }) {
  const context = s.context;
  if (!context)
    return (
      <Empty title={s.file ? 'Context unavailable' : 'Choose a source file'}>
        {s.file ? (
          <Button disabled={!!s.busy} onClick={() => void w.inspectContext()}>
            Refresh context
          </Button>
        ) : (
          <Go page="editor">Browse source</Go>
        )}
      </Empty>
    );
  return (
    <div className="workspace-page context-inspection">
      <Heading variant="intro" title="Provider context" detail={s.file?.path} />
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
