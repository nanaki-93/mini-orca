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
} from './ui';

export function Receipt({ s }: { s: State }) {
  const r = s.receipt;
  if (!r) return <Empty title="No change receipt" />;
  return (
    <>
      <Heading
        title={r.audit?.action === 'undo' ? 'Change undone' : 'Change applied'}
        eyebrow="Source updated"
        detail={r.audit?.target_path}
      >
        <Go page="editor">Open source</Go>
        {r.undo_available && (
          <Button icon="undo" disabled={!!s.busy || s.uncertain} onClick={() => void w.undo()}>
            Undo change
          </Button>
        )}
      </Heading>
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
  );
}
function Samples({ samples }: { samples: BenchmarkSample[] }) {
  return (
    <div className="table-wrap">
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
    <>
      <Heading
        title="Benchmark comparison"
        eyebrow="Measured evidence"
        detail={s.draft?.target_symbol}
      >
        <Go page="checks">Back to checks</Go>
        <Button
          disabled={!!s.busy || !currentDraft(s)}
          onClick={() => void w.benchmarks()}
          icon="refresh"
        >
          Find benchmarks
        </Button>
      </Heading>
      <Panel title="Existing benchmarks">
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
        {!c && <Empty title="Validate a draft to find compatible benchmarks" />}
      </Panel>
      {r && (
        <>
          <div className="row between section-gap">
            <h2>{r.benchmark}</h2>
            <Badge value={r.status} />
          </div>
          {r.reason && <Notice>{r.reason}</Notice>}
          <div className="grid equal-columns section-gap">
            <Panel title="Before">
              {r.base ? (
                <Samples samples={r.base.samples} />
              ) : (
                <p className="muted">No measurement</p>
              )}
            </Panel>
            <Panel title="Candidate">
              {r.candidate ? (
                <Samples samples={r.candidate.samples} />
              ) : (
                <p className="muted">No measurement</p>
              )}
            </Panel>
          </div>
          <Disclosure title="Command">
            <pre>{r.command?.join(' ')}</pre>
          </Disclosure>
        </>
      )}
    </>
  );
}
export function Scan({ s }: { s: State }) {
  return (
    <>
      <Heading
        title="Verified scan"
        eyebrow="Local tools"
        detail={s.scan && <Badge value={s.scan.status} />}
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
        <div className="stack">
          {s.scan.phases?.map((phase, i) => (
            <Panel title={phase.name} actions={<Badge value={phase.state} />} key={i}>
              <pre className="command">{phase.command?.join(' ')}</pre>
              {phase.output && (
                <Disclosure title="Output">
                  <pre>{phase.output}</pre>
                </Disclosure>
              )}
              {phase.exit_code !== 0 && phase.exit_code !== undefined && (
                <p className="small muted">Exit {phase.exit_code}</p>
              )}
            </Panel>
          ))}
        </div>
      ) : (
        <Empty
          title="No verified scan yet"
          detail="Run the local checks for this project."
          icon="shield"
        />
      )}
    </>
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
        background: '#131619',
        foreground: '#e9eeee',
        cursor: '#b4e5cb',
        selectionBackground: '#355344',
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
        <span className="small muted">User-operated shell</span>
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
      <Heading title="Terminal" eyebrow={s.project?.name || 'Workspace'} detail={s.project?.path}>
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
        <Empty
          title="Your shell, in this project"
          detail="Start a terminal when you need it."
          icon="terminal"
        >
          <Button disabled={!!s.busy || !s.project} onClick={() => void w.openTerminal()}>
            Start terminal
          </Button>
        </Empty>
      )}
    </div>
  );
}
