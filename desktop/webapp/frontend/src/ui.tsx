import { useEffect, useRef, useState, type ReactNode, type ButtonHTMLAttributes } from 'react';
import { renderMermaidSVG } from 'beautiful-mermaid';
import { icons } from './icons';
import type { Insight } from './models';
import { workspace, type Page } from './workspace';

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
  tone = '',
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { icon?: string; tone?: string }) {
  return (
    <button {...props} className={`button ${tone} ${props.className || ''}`}>
      {icon && <Icon name={icon} />}
      {children}
    </button>
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
  return (
    <Button tone={tone} icon={icon} onClick={() => void workspace.navigate(page)}>
      {children}
    </Button>
  );
}
export const human = (text?: string) =>
  (text || 'Unavailable').replaceAll('_', ' ').replaceAll('-', ' ');
export function Badge({ value, tone }: { value?: string; tone?: string }) {
  const style =
    tone ??
    (['fresh', 'passed', 'completed', 'completed_empty', 'verified', 'applied'].includes(
      value || '',
    )
      ? 'green'
      : ['failed', 'high', 'critical', 'error'].includes(value || '')
        ? 'red'
        : ['stale', 'partial', 'paused', 'medium', 'interrupted'].includes(value || '')
          ? 'amber'
          : '');
  return <span className={`badge ${style}`}>{human(value)}</span>;
}
export function Heading({
  title,
  eyebrow,
  children,
  detail,
}: {
  title: string;
  eyebrow?: string;
  children?: ReactNode;
  detail?: ReactNode;
}) {
  return (
    <header className="page-heading">
      <div>
        {eyebrow && <div className="eyebrow">{eyebrow}</div>}
        <h1>{title}</h1>
        {detail && <p>{detail}</p>}
      </div>
      <div className="actions">{children}</div>
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
      <div className="actions">{children}</div>
    </div>
  );
}
export function Notice({ children, error = false }: { children: ReactNode; error?: boolean }) {
  return (
    <div className={`notice ${error ? 'error' : ''}`} role={error ? 'alert' : 'status'}>
      <Icon name="warning" />
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
      {insight.why_it_matters_here && <Prose text={insight.why_it_matters_here} />}
      <Disclosure title="Tradeoff & lesson">
        <Prose text={insight.tradeoff_or_failure_mode} />
        <Prose text={insight.transferable_lesson} />
      </Disclosure>
    </Panel>
  );
}
function Diagram({ source }: { source: string }) {
  const [result, setResult] = useState<{ url?: string; error?: string }>({});
  useEffect(() => {
    try {
      if (source.length > 16000 || source.split('\n').length > 160)
        throw new Error('Diagram is too large to render.');
      const svg = renderMermaidSVG(source, {
        bg: '#1a1e22',
        fg: '#e9eeee',
        accent: '#b4e5cb',
        font: 'sans-serif',
        padding: 24,
      });
      // SVG images cannot run scripts or activate model-authored links.
      setResult({ url: `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}` });
    } catch {
      setResult({ error: 'Diagram preview unavailable' });
    }
  }, [source]);
  return (
    <div className="diagram">
      {result.url && <img src={result.url} alt="Architecture diagram" />}
      {result.error && <p>{result.error}</p>}
      <Disclosure title="Diagram source">
        <pre>{source}</pre>
      </Disclosure>
    </div>
  );
}
export function Prose({ text }: { text?: string }) {
  if (!text) return null;
  const pieces = text.split(/(```[\s\S]*?```)/g);
  return (
    <div className="prose">
      {pieces.map((piece, i) => {
        if (piece.startsWith('```mermaid\n'))
          return <Diagram source={piece.slice(11, -3).trim()} key={i} />;
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
