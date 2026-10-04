import {
  Component,
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
  type ReactNode,
} from 'react';
import { createRoot } from 'react-dom/client';
import { workspace as w, canCancelOperation, type Page } from './workspace';
import { Button, Icon, Modal, Notice } from './ui';
import { Summary, ProjectPage, Models, Diagrams, SearchPage } from './overview';
import { Analysis, AnalysisPreview, AnalysisRun } from './analysis';
import { Results } from './results';
import { Editor } from './editor';
import { Benchmark, Scan, Receipt, TerminalWorkspace } from './tools';
import './style.css';

const mainNav: [Page, string, string][] = [
  ['summary', 'Summary', 'grid'],
  ['analysis', 'Analysis', 'activity'],
  ['bugs', 'Bugs', 'bug'],
  ['performance', 'Performance', 'gauge'],
  ['security', 'Security', 'shield'],
  ['editor', 'Editor', 'code'],
];
const utilityNav: [Page, string, string][] = [
  ['terminal', 'Terminal', 'terminal'],
  ['models', 'Models', 'sparkles'],
  ['project', 'Project', 'folder'],
];
const editorPages: Page[] = [
  'editor',
  'context',
  'manifest',
  'assistant',
  'new-declaration',
  'draft',
  'checks',
  'review',
];
function App() {
  const s = useSyncExternalStore(w.subscribe, w.snapshot);
  const [theme, setTheme] = useState(localStorage.getItem('mini-orca:theme') || 'dark');
  const [large, setLarge] = useState(localStorage.getItem('mini-orca:large') === 'true');
  const main = useRef<HTMLElement>(null);
  useEffect(() => {
    void w.start();
    const focus = () => void w.refreshFile();
    window.addEventListener('focus', focus);
    return () => {
      w.stop();
      window.removeEventListener('focus', focus);
    };
  }, []);
  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    localStorage.setItem('mini-orca:theme', theme);
  }, [theme]);
  useEffect(() => {
    document.documentElement.classList.toggle('large-text', large);
    localStorage.setItem('mini-orca:large', String(large));
  }, [large]);
  useEffect(() => {
    main.current?.focus();
    main.current?.scrollTo(0, 0);
  }, [s.page]);
  useEffect(() => {
    const key = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k' && !s.confirmation) {
        event.preventDefault();
        void w.navigate('search');
      }
    };
    window.addEventListener('keydown', key);
    return () => window.removeEventListener('keydown', key);
  }, [s.confirmation]);
  const nav = (items: typeof mainNav) =>
    items.map(([page, label, icon]) => (
      <button
        key={page}
        className="nav-link"
        aria-label={label}
        disabled={!s.project && !['project', 'models'].includes(page)}
        aria-current={
          s.page === page ||
          (page === 'analysis' && s.page.startsWith('analysis')) ||
          (page === 'editor' && editorPages.includes(s.page))
            ? 'page'
            : undefined
        }
        onClick={() => void w.navigate(page)}
      >
        <Icon name={icon} />
        <span>{label}</span>
        {['bugs', 'performance', 'security'].includes(page) && (
          <span className="nav-count">
            {s.run?.sections.find((section) => section.category === page)?.finding_count ?? '—'}
          </span>
        )}
      </button>
    ));
  const title =
    [...mainNav, ...utilityNav].find(([page]) => page === s.page)?.[1] ||
    s.page
      .split('-')
      .map((word) => word[0].toUpperCase() + word.slice(1))
      .join(' ');
  let content: ReactNode;
  if (s.page === 'models') content = <Models s={s} />;
  else if (s.page === 'project' || s.page === 'welcome' || !s.project)
    content = <ProjectPage s={s} />;
  else if (s.page === 'summary') content = <Summary s={s} />;
  else if (s.page === 'analysis') content = <Analysis s={s} />;
  else if (s.page === 'analysis-preview') content = <AnalysisPreview s={s} />;
  else if (s.page === 'analysis-run') content = <AnalysisRun s={s} />;
  else if (['bugs', 'performance', 'security'].includes(s.page))
    content = <Results s={s} key={s.page} />;
  else if (editorPages.includes(s.page)) content = <Editor s={s} />;
  else if (s.page === 'benchmark') content = <Benchmark s={s} />;
  else if (s.page === 'scan') content = <Scan s={s} />;
  else if (s.page === 'receipt') content = <Receipt s={s} />;
  else if (s.page === 'search') content = <SearchPage s={s} />;
  else if (s.page === 'diagrams') content = <Diagrams s={s} />;
  return (
    <>
      <a className="skip" href="#main">
        Skip to content
      </a>
      <div className="app-window">
        <aside className="sidebar" aria-label="Application">
          <button
            className="brand"
            onClick={() => void w.navigate(s.project ? 'summary' : 'welcome')}
          >
            <span className="brand-symbol">◒</span>
            <span>
              mini-orca<span className="accent">.</span>
            </span>
          </button>
          <button className="project-switcher" onClick={() => void w.navigate('project')}>
            <span className="project-avatar">{s.project?.name?.[0]?.toUpperCase() || '+'}</span>
            <span className="project-label">
              <strong>{s.project?.name || 'Open a project'}</strong>
              <small>{s.project ? `${s.project.type} workspace` : 'Local workspace'}</small>
            </span>
            <Icon name="chevrons" />
          </button>
          <div className="nav-label">Workspace</div>
          <nav aria-label="Workspaces">{nav(mainNav)}</nav>
          <div className="sidebar-bottom">
            <nav aria-label="Tools">{nav(utilityNav)}</nav>
            <div className="local-label">
              <Icon name="laptop" />
              On your computer<span className="shortcut">⌘ K</span>
            </div>
          </div>
        </aside>
        <div className="app-body">
          <header className="topbar">
            <div className="breadcrumbs">
              <span className="muted">{s.project?.name || 'Mini-Orca'}</span>
              <span className="muted">/</span>
              <span>{title}</span>
            </div>
            <div className="topbar-actions">
              <button
                className="search-trigger"
                onClick={() => void w.navigate('search')}
                aria-label="Search files and commands"
              >
                <Icon name="search" />
                <span>Search</span>
                <kbd>⌘ K</kbd>
              </button>
              <div className="connection">
                <span className={`status-dot ${s.connected ? '' : 'offline'}`} />
                {s.connected ? 'Daemon connected' : 'Daemon offline'}
              </div>
            </div>
          </header>
          <main
            id="main"
            ref={main}
            tabIndex={-1}
            className={s.page === 'terminal' ? 'terminal-notices' : ''}
          >
            {s.busy && (
              <div className="busy-strip" role="status">
                <span className="spinner" />
                {s.busy}…
                {canCancelOperation(s.busy) && (
                  <Button tone="ghost small" onClick={() => void w.cancel()}>
                    Cancel
                  </Button>
                )}
              </div>
            )}
            <div className="page">
              {s.error && <Notice error>{s.error}</Notice>}
              {s.notice && <Notice>{s.notice}</Notice>}
              {Object.entries(s.resourceErrors).map(([key, error]) => (
                <Notice error key={key}>
                  <strong>{key}: </strong>
                  {error}
                </Notice>
              ))}
              {content}
            </div>
          </main>
          <div hidden={s.page !== 'terminal'} className="terminal-container">
            <TerminalWorkspace s={s} visible={s.page === 'terminal'} />
          </div>
          <footer className="statusbar">
            <span>{s.git?.available ? s.git.branch : 'Local workspace'}</span>
            <span>
              {s.project ? `${s.project.source_file_count ?? '—'} source files` : 'No project open'}
            </span>
            <div className="statusbar-end">
              <span>
                {s.draft
                  ? `Draft · ${s.dirty ? 'edited' : s.draft.state}`
                  : s.version
                    ? `Daemon ${s.version}`
                    : ''}
              </span>
              <button
                aria-label={`Switch to ${theme === 'dark' ? 'light' : 'dark'} appearance`}
                onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
              >
                <Icon name={theme === 'dark' ? 'sun' : 'moon'} />
              </button>
              <button
                aria-label="Larger text"
                aria-pressed={large}
                onClick={() => setLarge(!large)}
              >
                Aa
              </button>
            </div>
          </footer>
        </div>
      </div>
      {s.confirmation && <Modal key={s.confirmation.title} />}
    </>
  );
}
class Boundary extends Component<{ children: ReactNode }, { error: string }> {
  state = { error: '' };
  static getDerivedStateFromError(error: Error) {
    return { error: error.message };
  }
  render() {
    return this.state.error ? (
      <div className="page">
        <h1>Unable to display this screen</h1>
        <Notice error>{this.state.error}</Notice>
        <p>Close and reopen Mini-Orca to reconnect.</p>
      </div>
    ) : (
      this.props.children
    );
  }
}
createRoot(document.getElementById('root')!).render(
  <Boundary>
    <App />
  </Boundary>,
);
