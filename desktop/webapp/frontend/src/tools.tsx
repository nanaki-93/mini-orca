import { useEffect, useRef, useState } from 'react';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';
import { errorMessage, native } from './api';
import { workspace as w, type State } from './workspace';
import type { TerminalUpdate } from './models';
import {
  Badge,
  Button,
  Disclosure,
  Empty,
  Heading,
  Icon,
  Notice,
  Panel,
  StatusDot,
  human,
} from './ui';

export function Tests({ s }: { s: State }) {
  return (
    <section className="workspace-page tests-workspace" aria-labelledby="project-tests-title">
      <header className="page-heading">
        <div>
          <h2 id="project-tests-title">Tests</h2>
          <p>
            {s.scan && (
              <span className="test-status">
                <StatusDot value={s.scan.status} />
                <span>Status: {human(s.scan.status)}</span>
              </span>
            )}
            Local check results. Review each phase for failures and diagnostics.
          </p>
        </div>
      </header>
      {s.scan ? (
        <div className="stack test-phases">
          {s.scan.phases?.map((phase, i) => (
            <Panel
              title={phase.name}
              actions={
                <span className="test-status">
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
        <Empty
          title="No test run yet"
          detail="Use Run Tests in the top bar to run the project checks."
          icon="circleCheck"
        />
      )}
    </section>
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
