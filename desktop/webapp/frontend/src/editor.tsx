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

function FileTree({
  paths,
  selected,
  busy,
  depth = 0,
}: {
  paths: string[];
  selected?: string;
  busy: boolean;
  depth?: number;
}) {
  const folders = new Map<string, string[]>();
  const leaves: string[] = [];
  for (const path of paths) {
    const parts = path.split('/');
    if (parts.length > depth + 1) {
      const name = parts[depth];
      folders.set(name, [...(folders.get(name) || []), path]);
    } else leaves.push(path);
  }
  return (
    <>
      {[...folders]
        .sort(([a], [b]) => a.localeCompare(b))
        .map(([name, children]) => (
          <details className="explorer-folder" key={name} open={depth < 2}>
            <summary>
              <Icon name="folder" />
              {name}
            </summary>
            <FileTree paths={children} selected={selected} busy={busy} depth={depth + 1} />
          </details>
        ))}
      {leaves.map((path) => (
        <button
          key={path}
          title={path}
          disabled={busy}
          className={`file-item ${selected === path ? 'active' : ''}`}
          onClick={() => void w.openFile(path)}
        >
          <Icon name="file" />
          {path.split('/').at(-1)}
        </button>
      ))}
    </>
  );
}
export function Editor({ s }: { s: State }) {
  const [filter, setFilter] = useState('');
  const [limit, setLimit] = useState(100);
  const [opened, setOpened] = useState<string[]>(s.file ? [s.file.path] : []);
  useEffect(() => {
    if (s.file)
      setOpened((paths) => (paths.includes(s.file!.path) ? paths : [...paths, s.file!.path]));
  }, [s.file?.path]);
  const files = (s.index?.files || []).filter((file) =>
    file.path.toLowerCase().includes(filter.toLowerCase()),
  );
  return (
    <div className="workspace-page source-workspace">
      <div className="editor-workspace">
        <aside className="file-browser" aria-label="Project files">
          <div className="explorer-heading">
            <strong>Explorer</strong>
            <Go page="search" icon="search" tone="ghost">
              Find
            </Go>
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
          <div className="explorer-project">{s.project?.name}</div>
          <div className="file-list">
            {filter ? (
              files.slice(0, limit).map((file) => (
                <button
                  key={file.path}
                  title={file.path}
                  className={`file-item ${s.file?.path === file.path ? 'active' : ''}`}
                  disabled={!!s.busy}
                  onClick={() => void w.openFile(file.path)}
                >
                  <Icon name="file" />
                  <span>{file.path}</span>
                </button>
              ))
            ) : (
              <FileTree
                paths={files.slice(0, limit).map((file) => file.path)}
                selected={s.file?.path}
                busy={!!s.busy}
              />
            )}
            {!files.length && <p className="small muted">No matching files.</p>}
            {files.length > limit && (
              <Button onClick={() => setLimit(limit + 100)}>Show more</Button>
            )}
          </div>
          <div className="explorer-footer">
            <Icon name="lock" />
            Read-only source
          </div>
        </aside>
        <div className="editor-content">
          <div className="source-filetabs" aria-label="Open files">
            {opened.map((path) => (
              <button
                key={path}
                className={s.file?.path === path ? 'active' : ''}
                aria-pressed={s.file?.path === path}
                disabled={!!s.busy}
                onClick={() => void w.openFile(path)}
              >
                <Icon name="file" />
                {path.split('/').at(-1)}
              </button>
            ))}
          </div>
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
  const [prompt, setPrompt] = useState('');
  const symbol = s.symbols.find((value) => value.name === s.symbol);
  useEffect(() => {
    code.current?.querySelector('.selected-line')?.scrollIntoView({ block: 'nearest' });
  }, [s.symbol]);
  return (
    <div className="source-inspection">
      <div className="source-document">
        <div className="source-panel">
          <div className="code-header">
            <span className="row">
              <Icon name="lock" />
              {s.file!.path}
            </span>
            <Badge value="Read-only" />
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
        <div className="source-foot actions">
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
          <span className="small muted">
            {s.file!.line_count} lines · {s.file!.language}
          </span>
        </div>
      </div>
      <aside className="source-context" aria-label="File insights">
        <div className="row between">
          <h2 className="row">
            <Icon name="sparkles" />
            Assistant
          </h2>
          <Go page="chat" tone="ghost" icon="arrow">
            Chat
          </Go>
        </div>
        <form
          className="source-question"
          onSubmit={(event) => {
            event.preventDefault();
            w.seedChange({
              title: `Update ${s.file!.name}`,
              kind: 'feature',
              paths: [s.file!.path],
              acceptance_criteria: [],
              message: prompt,
            });
          }}
        >
          <textarea
            aria-label="Ask about this file"
            placeholder="Ask about this file…"
            value={prompt}
            onChange={(event) => setPrompt(event.target.value)}
          />
          <Button
            tone="primary"
            disabled={
              !prompt.trim() ||
              !!s.busy ||
              s.fileStale ||
              s.file!.binary ||
              !/\.(go|md)$/.test(s.file!.path) ||
              activeChangeWorkflow(s.change)
            }
            type="submit"
          >
            Continue in Chat
          </Button>
        </form>
        {s.change && (
          <div className="source-prepared">
            <h3>Prepared change</h3>
            <p>{s.change.title}</p>
            <Go page="changes" icon="arrow">
              Review changes
            </Go>
          </div>
        )}
        <Disclosure title="Inspect this file">
          <DeclarationPicker s={s} />
          <div className="actions">
            {' '}
            <Button
              disabled={!!s.busy || !s.symbol || s.fileStale}
              onClick={() => void w.explain()}
            >
              Explain declaration
            </Button>
            <Button disabled={!!s.busy || s.fileStale} onClick={() => void w.analyzeFile()}>
              Analyze file
            </Button>
            <Go page="security" icon="shield">
              Security
            </Go>
          </div>
        </Disclosure>
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
