import { useWorkspace } from './workspace-context';
import {
  useEffect,
  useLayoutEffect,
  useState,
  useRef,
  type ReactNode,
  type ButtonHTMLAttributes,
} from 'react';
import { icons } from './icons';
import type { Insight } from './models';
import { type Page } from './workspace';

export function Icon({ name }: { name: string }) {
  return (
    <svg
      className="icon"
      viewBox="0 0 24 24"
      aria-hidden="true"
      dangerouslySetInnerHTML={{ __html: icons[name] || icons.file }}
    />
  );
}
export function Button({
  children,
  icon,
  loading = false,
  tone = '',
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { icon?: string; tone?: string; loading?: boolean }) {
  return (
    <button
      {...props}
      disabled={props.disabled || loading}
      aria-busy={loading || undefined}
      className={`button ${tone} ${props.className || ''}`}
    >
      {loading ? <Spinner /> : icon && <Icon name={icon} />}
      {children}
    </button>
  );
}
export function Spinner() {
  return <span className="spinner" aria-hidden="true" />;
}
export function Loading({ label }: { label: string }) {
  return (
    <div className="loading" role="status">
      <Spinner />
      <span>{label}</span>
    </div>
  );
}
export function Go({
  page,
  children,
  icon,
  tone = '',
}: {
  page: Page;
  children: ReactNode;
  icon?: string;
  tone?: string;
}) {
  const workspace = useWorkspace();
  return (
    <Button tone={tone} icon={icon} onClick={() => void workspace.navigate(page)}>
      {children}
    </Button>
  );
}
export const human = (text?: string) =>
  (text || 'Unavailable').replaceAll('_', ' ').replaceAll('-', ' ');
export type NoticeTone = 'info' | 'warning' | 'error' | 'success';

export function classifyNoticeTone(text?: string): NoticeTone {
  const value = (text || '').trim();
  if (!value) return 'info';
  if (
    /\b(failed|failure|error|conflict|conflicts|rejected|blocked|invalid|could not|cannot|unknown)\b/i.test(
      value,
    ) &&
    !/all hunks rejected/i.test(value)
  ) {
    return 'error';
  }
  if (
    /\b(stale|outdated|warning|attention|unsaved|paused|interrupted|stopped|changed\.\s*refresh|refresh status|before editing|before preparing|start a new)\b/i.test(
      value,
    )
  ) {
    return 'warning';
  }
  if (
    /\b(saved|applied|committed|merged|created|restored|completed|passed|ready for your review|trusted workspace:|added \d+ new)\b/i.test(
      value,
    )
  ) {
    return 'success';
  }
  return 'info';
}

export function StatusDot({ value, label }: { value?: string; label?: string }) {
  const status = label ? `${label}: ${human(value)}` : human(value);
  const tone = [
    'fresh',
    'current',
    'ready',
    'passed',
    'success',
    'completed',
    'completed_empty',
    'applied',
    'committed',
    'approve',
    'ready_to_apply',
    'verified',
    'fixed',
  ].includes(value || '')
    ? 'green'
    : ['failed', 'error', 'conflicts', 'request_changes'].includes(value || '')
      ? 'red'
      : [
            'info',
            'draft',
            'open',
            'saved',
            'pending',
            'awaiting_human_review',
            'not_generated',
            'skipped',
          ].includes(value || '')
        ? 'blue'
        : 'yellow';
  const active = ['running', 'queued', 'pausing', 'canceling'].includes(value || '');
  return (
    <span
      className={`status-dot ${tone}`}
      data-active={active || undefined}
      role="img"
      aria-label={status}
      title={status}
    >
      {active && <Spinner />}
    </span>
  );
}
export function Badge({ value, tone }: { value?: string; tone?: string }) {
  if (['suggested', 'ai_suggestion'].includes(value || '')) return null;
  if (['fresh', 'stale', 'error', 'completed', 'completed_empty'].includes(value || ''))
    return <StatusDot value={value} />;
  const style =
    tone ??
    ([
      'fresh',
      'passed',
      'success',
      'completed',
      'completed_empty',
      'verified',
      'applied',
      'committed',
      'approve',
      'validated',
      'ready_to_apply',
      'fixed',
      'ready',
      'current',
      'done',
    ].includes(value || '')
      ? 'green'
      : ['failed', 'high', 'critical', 'error', 'conflicts', 'request_changes'].includes(
            value || '',
          )
        ? 'red'
        : [
              'stale',
              'partial',
              'paused',
              'medium',
              'interrupted',
              'edited',
              'needs_attention',
              'checks_required',
              'unverified',
              'outdated',
              'dirty',
              'warning',
            ].includes(value || '')
          ? 'amber'
          : [
                'running',
                'queued',
                'pausing',
                'canceling',
                'info',
                'low',
                'open',
                'saved',
                'draft',
                'pending',
                'awaiting_human_review',
                'in_progress',
                'included',
                'existing file',
                'new file',
              ].includes(value || '')
            ? 'blue'
            : ['ai_suggestion', 'candidate', 'hypothesis', 'estimated'].includes(value || '')
              ? 'violet'
              : '');
  return (
    <span className={`badge ${style}`}>
      {['running', 'queued', 'pausing', 'canceling'].includes(value || '') && <Spinner />}
      {human(value)}
    </span>
  );
}
export function PriorityIcon({ value }: { value: string }) {
  const icon = {
    critical: 'priorityHighest',
    high: 'priorityHigh',
    medium: 'priorityMedium',
    low: 'priorityLow',
    info: 'priorityLowest',
  }[value];
  if (!icon) return <Badge value={value} />;
  const label = `Priority: ${human(value)}`;
  return (
    <span className={`priority-icon priority-${value}`} role="img" aria-label={label} title={label}>
      <Icon name={icon} />
    </span>
  );
}
export function Heading({
  title,
  children,
  detail,
  variant,
}: {
  title: string;
  children?: ReactNode;
  detail?: ReactNode;
  variant?: 'intro';
}) {
  return (
    <header className={`page-heading${variant === 'intro' ? ' page-heading--intro' : ''}`}>
      <div>
        <h1>{title}</h1>
        {detail && <p>{detail}</p>}
      </div>
      {children && <div className="actions">{children}</div>}
    </header>
  );
}
export function Panel({
  title,
  actions,
  children,
  className = '',
}: {
  title?: string;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <section className={`panel ${className}`}>
      {title && (
        <div className="panel-head">
          <h2>{title}</h2>
          {actions}
        </div>
      )}
      <div className={title ? 'panel-body' : 'panel-pad'}>{children}</div>
    </section>
  );
}
export function Empty({
  title,
  detail,
  children,
  icon = 'inbox',
}: {
  title: string;
  detail?: string;
  children?: ReactNode;
  icon?: string;
}) {
  return (
    <div className="empty">
      <div className="empty-icon">
        <Icon name={icon} />
      </div>
      <h2>{title}</h2>
      {detail && <p>{detail}</p>}
      {children && <div className="actions">{children}</div>}
    </div>
  );
}
export function Notice({
  children,
  tone,
  error = false,
  warning = false,
  success = false,
  info = false,
}: {
  children: ReactNode;
  tone?: NoticeTone;
  error?: boolean;
  warning?: boolean;
  success?: boolean;
  info?: boolean;
}) {
  const resolvedTone: NoticeTone =
    tone ||
    (error
      ? 'error'
      : warning
        ? 'warning'
        : success
          ? 'success'
          : info
            ? 'info'
            : typeof children === 'string'
              ? classifyNoticeTone(children)
              : 'info');
  const iconName =
    resolvedTone === 'error'
      ? 'error'
      : resolvedTone === 'warning'
        ? 'warning'
        : resolvedTone === 'success'
          ? 'circleCheck'
          : 'info';
  return (
    <div
      className={`notice ${resolvedTone}`}
      data-notice-tone={resolvedTone}
      role={resolvedTone === 'error' ? 'alert' : 'status'}
    >
      <Icon name={iconName} />
      <div>{children}</div>
    </div>
  );
}
export function KeyValues({ values }: { values: [string, ReactNode][] }) {
  return (
    <dl className="key-values">
      {values.map(([key, value]) => (
        <div key={key}>
          <dt>{key}</dt>
          <dd>{value ?? '—'}</dd>
        </div>
      ))}
    </dl>
  );
}
export function Disclosure({
  title,
  children,
  open = false,
}: {
  title: ReactNode;
  children: ReactNode;
  open?: boolean;
}) {
  return (
    <details open={open || undefined}>
      <summary>{title}</summary>
      <div className="disclosure-body">{children}</div>
    </details>
  );
}
export function InsightCard({ insight }: { insight?: Insight }) {
  if (!insight || !Object.values(insight).some(Boolean)) return null;
  return (
    <Panel title="Engineering insight" className="insight">
      <Prose text={insight.mechanism} />
      <Disclosure title="Why & tradeoffs">
        <Prose text={insight.why_it_matters_here} />
        <Prose text={insight.tradeoff_or_failure_mode} />
        <Prose text={insight.transferable_lesson} />
      </Disclosure>
    </Panel>
  );
}
function Diagram({ source, label }: { source: string; label: string }) {
  const [rendered, setRendered] = useState<{ source: string; url?: string; error?: string }>();
  const result = rendered?.source === source ? rendered : undefined;
  useEffect(() => {
    let active = true;
    void import('./diagram-renderer')
      .then(({ renderDiagram }) => {
        if (active) setRendered({ source, ...renderDiagram(source) });
      })
      .catch(() => {
        if (active) setRendered({ source, error: 'Diagram preview unavailable' });
      });
    return () => {
      active = false;
    };
  }, [source]);
  return (
    <div className="diagram">
      {result?.url && <img src={result.url} alt={label} />}
      {!result && <Loading label="Rendering diagram…" />}
      {result?.error && <p>{result.error}</p>}
      <Disclosure title="Diagram source" open={!!result?.error}>
        <pre tabIndex={0}>{source}</pre>
      </Disclosure>
    </div>
  );
}
export function Prose({
  text,
  diagramLabel = 'Mermaid diagram',
}: {
  text?: string;
  diagramLabel?: string;
}) {
  if (!text) return null;
  const source = text.trim();
  // Project reports store Mermaid directly, without Markdown fences.
  if (/^(flowchart|graph|sequenceDiagram)\b/.test(source))
    return (
      <div className="prose">
        <Diagram source={source} label={diagramLabel} />
      </div>
    );
  const pieces = text.split(/(```[\s\S]*?```)/g);
  return (
    <div className="prose">
      {pieces.map((piece, i) => {
        const mermaid = piece.match(/^```[ \t]*mermaid[ \t]*\r?\n([\s\S]*?)```$/i);
        if (mermaid) return <Diagram source={mermaid[1].trim()} label={diagramLabel} key={i} />;
        if (piece.startsWith('```'))
          return <pre key={i}>{piece.replace(/^```[^\n]*\n?/, '').replace(/```$/, '')}</pre>;
        return piece
          .split(/\n\s*\n/)
          .filter(Boolean)
          .map((paragraph, j) => <p key={`${i}-${j}`}>{paragraph}</p>);
      })}
    </div>
  );
}
export function BulletContent({ title, items }: { title: string; items?: string[] }) {
  return items?.length ? (
    <div className="content-section">
      <h3>{title}</h3>
      <ul>
        {items.map((item, i) => (
          <li key={i}>
            <Prose text={item} />
          </li>
        ))}
      </ul>
    </div>
  ) : null;
}
export function Modal() {
  const workspace = useWorkspace();
  const confirmation = workspace.state.confirmation;
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null;
    ref.current?.showModal();
    return () => previous?.focus();
  }, []);
  if (!confirmation) return null;
  return (
    <dialog
      ref={ref}
      aria-labelledby="confirm-title"
      onCancel={(event) => {
        event.preventDefault();
        workspace.respond(false);
      }}
    >
      <h2 id="confirm-title">{confirmation.title}</h2>
      <p>{confirmation.message}</p>
      {confirmation.details?.length ? (
        <div className="confirmation-details">
          {confirmation.details.map((line, i) => (
            <div key={i}>{line}</div>
          ))}
        </div>
      ) : null}
      <div className="actions end">
        <Button autoFocus onClick={() => workspace.respond(false)}>
          Cancel
        </Button>
        <Button
          tone={confirmation.destructive ? 'danger' : 'primary'}
          onClick={() => workspace.respond(true)}
        >
          {confirmation.accept}
        </Button>
      </div>
    </dialog>
  );
}

export function Overlay({
  title,
  children,
  onClose,
}: {
  title: string;
  children: ReactNode;
  onClose: () => void;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  useLayoutEffect(() => {
    const previous = document.activeElement as HTMLElement | null;
    ref.current?.showModal();
    ref.current?.querySelector<HTMLElement>('[data-initial-focus]')?.focus();
    return () => {
      if (previous?.isConnected) previous.focus();
    };
  }, []);
  return (
    <dialog
      ref={ref}
      className="studio-overlay"
      aria-label={title}
      onCancel={(event) => {
        event.preventDefault();
        onClose();
      }}
    >
      <header className="overlay-heading">
        <h2>{title}</h2>
        <Button
          aria-label={`Close ${title.toLowerCase()}`}
          tone="ghost"
          icon="close"
          onClick={onClose}
        />
      </header>
      {children}
    </dialog>
  );
}
