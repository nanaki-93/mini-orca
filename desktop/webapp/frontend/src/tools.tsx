import { useEffect, useRef, useState } from 'react';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';
import { errorMessage, native } from './api';
import { currentDraft, workspace as w, type State } from './workspace';
import type { BenchmarkSample, TerminalUpdate } from './models';
import {
  Badge,
  Button,
  Disclosure,
  Empty,
  Go,
  Heading,
  Icon,
  KeyValues,
  Notice,
  Panel,
  Prose,
  StatusDot,
  human,
} from './ui';

export function Receipt({ s }: { s: State }) {
  const r = s.receipt;
  return (
    <div className="workspace-page receipt-workspace">
      <Heading
        variant="intro"
        title={
          !r ? 'Change receipt' : r.audit?.action === 'undo' ? 'Change undone' : 'Change applied'
        }
        detail={r?.audit?.target_path}
      >
        {r && <Go page="editor">Open source</Go>}
        {r?.undo_available && (
          <Button icon="undo" disabled={!!s.busy || s.uncertain} onClick={() => void w.undo()}>
            Undo change
          </Button>
        )}
      </Heading>
      {!r ? (
        <Empty title="No change receipt" />
      ) : (
        <>
          <div className="grid two-columns">
            <Panel title="Receipt">
              <div className="receipt-mark">
                <Icon name="circleCheck" />
              </div>
              <KeyValues
                values={[
                  ['Action', r.audit?.action],
                  ['Outcome', r.audit?.outcome],
                  ['File', r.audit?.target_path],
                  [
                    'Time',
                    r.audit?.timestamp ? new Date(r.audit.timestamp).toLocaleString() : undefined,
                  ],
                  ['Undo', r.undo_available ? 'Available' : 'Unavailable'],
                ]}
              />
            </Panel>
            <Panel title="Audit">
              <KeyValues
                values={[
                  ['Receipt', r.audit?.id],
                  ['Before', r.audit?.before_hash],
                  ['After', r.audit?.after_hash || r.post_apply_hash],
                  ['Project revision', r.project_revision],
                ]}
              />
            </Panel>
          </div>
          {(r.warnings || []).map((warning, i) => (
            <Notice key={i}>{warning}</Notice>
          ))}
        </>
      )}
    </div>
  );
}
function Samples({ samples, label }: { samples: BenchmarkSample[]; label: string }) {
  if (!samples.length) return <p className="muted">No samples returned</p>;
  return (
    <div
      className="table-wrap benchmark-samples"
      role="region"
      aria-label={`${label} measurements`}
      tabIndex={0}
    >
      <table>
        <thead>
          <tr>
            <th>Iterations</th>
            <th>ns/op</th>
            <th>B/op</th>
            <th>allocs/op</th>
          </tr>
        </thead>
        <tbody>
          {samples.map((sample, i) => (
            <tr key={i}>
              <td>{sample.iterations.toLocaleString()}</td>
              <td>{sample.ns_per_op.toLocaleString()}</td>
              <td>{sample.bytes_per_op?.toLocaleString() ?? '—'}</td>
              <td>{sample.allocs_per_op?.toLocaleString() ?? '—'}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
export function Benchmark({ s }: { s: State }) {
  const c = s.benchmarkCatalog;
  const r = s.benchmark;
  return (
    <div className="workspace-page benchmark-workspace">
      <Heading variant="intro" title="Benchmark comparison" detail={s.draft?.target_symbol}>
        <Go page="checks">Back to checks</Go>
        <Button
          disabled={!!s.busy || !currentDraft(s)}
          onClick={() => void w.benchmarks()}
          icon="refresh"
        >
          Find benchmarks
        </Button>
      </Heading>
      <Panel
        title="Existing benchmarks"
        actions={c && !c.available ? <Badge value="unavailable" /> : undefined}
      >
        {c?.reason && <Prose text={c.reason} />}
        {c?.benchmarks?.map((choice) => (
          <div className="list-row" key={`${choice.name}:${choice.scope}`}>
            <span className="list-copy">
              <strong>{choice.name}</strong>
              <small className="mono">{choice.command.join(' ')}</small>
            </span>
            <Button
              tone="primary small"
              disabled={!!s.busy || !currentDraft(s) || !c.available}
              onClick={() => void w.compare(choice)}
            >
              Compare
            </Button>
          </div>
        ))}
        {c && !c.benchmarks?.length && (
          <Empty title={c.available ? 'No existing benchmarks' : 'Benchmark catalog unavailable'} />
        )}
        {!c && (
          <Empty title={currentDraft(s) ? 'No benchmark catalog' : 'Draft validation required'} />
        )}
      </Panel>
      {r && (
        <section className="benchmark-comparison" aria-label="Measured comparison">
          <div className="row between wrap benchmark-comparison-heading">
            <h2>{r.benchmark}</h2>
            <Badge value={r.status} />
          </div>
          {r.reason && <Notice>{r.reason}</Notice>}
          <div className="grid equal-columns">
            <Panel title="Before">
              {r.base ? (
                <Samples samples={r.base.samples} label="Before" />
              ) : (
                <p className="muted">No measurement</p>
              )}
            </Panel>
            <Panel title="Candidate">
              {r.candidate ? (
                <Samples samples={r.candidate.samples} label="Candidate" />
              ) : (
                <p className="muted">No measurement</p>
              )}
            </Panel>
          </div>
          <Disclosure title="Command">
            <pre>{r.command?.join(' ')}</pre>
          </Disclosure>
        </section>
      )}
    </div>
  );
}
export function Scan({ s }: { s: State }) {
  return (
    <div className="workspace-page scan-workspace">
      <Heading
        variant="intro"
        title="Verified scan"
        detail={
          <>
            {s.scan && (
              <span className="scan-status">
                <StatusDot value={s.scan.status} />
                <span>Status: {human(s.scan.status)}</span>
              </span>
            )}
            Local phase evidence. Completion does not mean every phase passed or the project is
            safe.
          </>
        }
      >
        <Go page="bugs">Findings</Go>
        {s.scan?.status === 'running' ? (
          <Button disabled={!!s.busy} onClick={() => void w.scanProject(true)}>
            Cancel scan
          </Button>
        ) : (
          <Button
            disabled={!!s.busy}
            tone="primary"
            icon="play"
            onClick={() => void w.scanProject()}
          >
            Run scan
          </Button>
        )}
      </Heading>
      {s.scan ? (
        <div className="stack scan-phases">
          {s.scan.phases?.map((phase, i) => (
            <Panel
              title={phase.name}
              actions={
                <span className="scan-status">
                  <StatusDot value={phase.state} />
                  <span>{human(phase.state)}</span>
                </span>
              }
              key={i}
            >
              {phase.command?.length ? (
                <pre className="command">{phase.command.join(' ')}</pre>
              ) : (
                <p className="small muted">No command reported</p>
              )}
              {phase.exit_code !== 0 && phase.exit_code !== undefined && (
                <p className="small muted">Exit {phase.exit_code}</p>
              )}
              {phase.output ? (
                <Disclosure title="Output">
                  <pre>{phase.output}</pre>
                </Disclosure>
              ) : (
                <p className="small muted">No output returned</p>
              )}
            </Panel>
          ))}
          {!s.scan.phases?.length && <Empty title="No phase evidence returned" />}
        </div>
      ) : (
        <Empty title="No verified scan yet" icon="shield" />
      )}
    </div>
  );
}
function TerminalSession({ session, visible }: { session: TerminalUpdate; visible: boolean }) {
  const container = useRef<HTMLDivElement>(null);
  const fit = useRef<FitAddon>(null);
  const terminal = useRef<Terminal>(null);
  const [status, setStatus] = useState(session.state);
  const [error, setError] = useState('');
  useEffect(() => {
    const term = new Terminal({
      fontFamily: 'SFMono-Regular, Menlo, Consolas, monospace',
      fontSize: 13,
      scrollback: 5000,
      cursorBlink: true,
      allowProposedApi: false,
      theme: {
        background: '#060c09',
        foreground: '#f1fff4',
        cursor: '#5cfa8b',
        selectionBackground: '#204b30',
      },
    });
    const addon = new FitAddon();
    fit.current = addon;
    terminal.current = term;
    term.loadAddon(addon);
    term.open(container.current!);
    let stopped = false,
      cursor = 0,
      timer: ReturnType<typeof setTimeout>;
    let writes = Promise.resolve();
    const input = term.onData((data) => {
      // Only xterm's user input stream reaches the PTY; assistant text has no path here.
      writes = writes
        .then(() => native().WriteTerminal(session.id, data))
        .catch((err) => {
          if (!stopped) setError(errorMessage(err));
        });
    });
    const resize = term.onResize((size) => {
      void native()
        .ResizeTerminal(
          session.id,
          Math.min(1000, Math.max(1, size.cols)),
          Math.min(1000, Math.max(1, size.rows)),
        )
        .catch((err) => {
          if (!stopped) setError(errorMessage(err));
        });
    });
    const observer = new ResizeObserver(() => {
      if (container.current?.offsetParent) addon.fit();
    });
    observer.observe(container.current!);
    const poll = async () => {
      try {
        const update = await native().ReadTerminal(session.id, cursor);
        if (stopped) return;
        if (update.reset) {
          term.reset();
          term.writeln('[Earlier output was truncated]');
        }
        if (update.data)
          await new Promise<void>((resolve) =>
            term.write(
              Uint8Array.from(atob(update.data), (c) => c.charCodeAt(0)),
              resolve,
            ),
          );
        cursor = update.cursor;
        setStatus(update.state);
        if (update.error) setError(update.error);
      } catch (err) {
        if (!stopped) setError(errorMessage(err));
        return;
      }
      if (!stopped) timer = setTimeout(poll, 120);
    };
    void poll();
    return () => {
      stopped = true;
      clearTimeout(timer);
      input.dispose();
      resize.dispose();
      observer.disconnect();
      term.dispose();
      terminal.current = null;
      fit.current = null;
    };
  }, [session.id]);
  useEffect(() => {
    if (visible) {
      fit.current?.fit();
      terminal.current?.focus();
    }
  }, [visible]);
  return (
    <div hidden={!visible} className="terminal-session">
      {error && <Notice error>{error}</Notice>}
      <div className="terminal-meta">
        <Badge value={status} />
      </div>
      <div
        className="terminal-surface"
        ref={container}
        aria-label={`Terminal session ${session.id}`}
      />
    </div>
  );
}
export function TerminalWorkspace({ s, visible }: { s: State; visible: boolean }) {
  const [selected, setSelected] = useState('');
  const active = s.terminals.find((t) => t.id === selected)?.id || s.terminals.at(-1)?.id;
  return (
    <div className="terminal-workspace">
      <Heading title="Terminal" detail={s.project?.path}>
        <Button
          icon="plus"
          disabled={!!s.busy || !s.project}
          tone="primary"
          onClick={() => void w.openTerminal()}
        >
          New terminal
        </Button>
      </Heading>
      {s.terminals.length ? (
        <>
          <div className="terminal-tabs" role="tablist" aria-label="Terminal sessions">
            {s.terminals.map((t) => (
              <div className={`terminal-tab ${t.id === active ? 'active' : ''}`} key={t.id}>
                <button
                  role="tab"
                  aria-selected={t.id === active}
                  onClick={() => setSelected(t.id)}
                >
                  <Icon name="terminal" />
                  Shell {t.id}
                </button>
                <button
                  disabled={!!s.busy}
                  aria-label={`Close terminal ${t.id}`}
                  onClick={() => void w.closeTerminal(t.id)}
                >
                  <Icon name="close" />
                </button>
              </div>
            ))}
          </div>
          {s.terminals.map((t) => (
            <TerminalSession key={t.id} session={t} visible={visible && active === t.id} />
          ))}
        </>
      ) : (
        <Empty title="No terminal sessions" icon="terminal">
          <Button disabled={!!s.busy || !s.project} onClick={() => void w.openTerminal()}>
            Start terminal
          </Button>
        </Empty>
      )}
    </div>
  );
}
