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
import { Button, Heading, Icon, Modal, Notice, Overlay } from './ui';
import { Summary, ProjectPage, Models, Diagrams, SearchPage } from './overview';
import { Analysis, AnalysisFiles, AnalysisPreview, AnalysisRun } from './analysis';
import { Results } from './results';
import { Editor, Context } from './editor';
import { ChangeWorkspace } from './change-workspace';
import { ChangeHistory } from './change-shared';
import { Features } from './features';
import { Instructions } from './instructions';
import { Scan, TerminalWorkspace } from './tools';
import './style.css';
import './studio.css';

const mainNav: [Page, string, string][] = [
  ['summary', 'Overview', 'grid'],
  ['editor', 'Source', 'code'],
  ['chat', 'Chat', 'chat'],
];
const improveNav: typeof mainNav = [
  ['analysis', 'Analysis', 'activity'],
  ['bugs', 'Findings', 'bug'],
  ['features', 'Features', 'sparkles'],
  ['changes', 'Changes', 'branch'],
];
const utilityNav: [Page, string, string][] = [
  ['diagrams', 'Architecture', 'branch'],
  ['instructions', 'Instructions', 'file'],
  ['analysis-files', 'Context', 'layers'],
  ['models', 'Models', 'layers'],
  ['history', 'History', 'clock'],
];
const analysisPages: [Page, string][] = [
  ['analysis', 'Setup'],
  ['analysis-run', 'Last run'],
];
const contextPages: [Page, string][] = [
  ['analysis-files', 'Files & scope'],
  ['context', 'Provider context'],
];
const findingPages: [Page, string][] = [
  ['bugs', 'Bugs'],
  ['security', 'Security'],
  ['performance', 'Performance'],
];
const themes = [
  { id: 'dark', name: 'Graphite', letter: 'G' },
  { id: 'light', name: 'Porcelain', letter: 'P' },
  { id: 'midnight', name: 'Midnight', letter: 'M' },
] as const;
function App() {
  const live = useSyncExternalStore(w.subscribe, w.snapshot);
  const lastPage = useRef<Page>(live.project ? 'summary' : 'welcome');
  const overlay =
    ['search', 'project', 'terminal', 'analysis-preview'].includes(live.page) && !!live.project;
  if (!overlay) lastPage.current = live.page;
  const s = overlay ? { ...live, page: lastPage.current } : live;
  const closeOverlay = () => void w.navigate(lastPage.current);
  const terminalOpen = live.page === 'terminal';
  const [theme, setTheme] = useState(() => {
    const saved = localStorage.getItem('mini-orca:theme');
    return themes.find((choice) => choice.id === saved)?.id || 'dark';
  });
  const [large, setLarge] = useState(localStorage.getItem('mini-orca:large') === 'true');
  const [sidebarCollapsed, setSidebarCollapsed] = useState(
    () => localStorage.getItem('mini-orca:sidebar-collapsed') === 'true',
  );
  const main = useRef<HTMLElement>(null);
  const activePage = findingPages.some(([page]) => page === s.page)
    ? 'bugs'
    : ['analysis', 'analysis-preview', 'analysis-run'].includes(s.page)
      ? 'analysis'
      : contextPages.some(([page]) => page === s.page)
        ? 'analysis-files'
        : s.page;
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
    const root = document.documentElement;
    // Switch text and surfaces together; hover fades must not cross theme palettes.
    root.classList.add('theme-changing');
    root.dataset.theme = theme;
    localStorage.setItem('mini-orca:theme', theme);
    let frame = requestAnimationFrame(() => {
      frame = requestAnimationFrame(() => root.classList.remove('theme-changing'));
    });
    return () => {
      cancelAnimationFrame(frame);
      root.classList.remove('theme-changing');
    };
  }, [theme]);
  useEffect(() => {
    document.documentElement.classList.toggle('large-text', large);
    localStorage.setItem('mini-orca:large', String(large));
  }, [large]);
  useEffect(() => {
    localStorage.setItem('mini-orca:sidebar-collapsed', String(sidebarCollapsed));
  }, [sidebarCollapsed]);
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
        data-accent={page}
        aria-label={label}
        title={sidebarCollapsed ? label : undefined}
        disabled={!s.project && !['project', 'models'].includes(page)}
        aria-current={
          activePage === page ||
          (page === 'analysis' && s.page === 'analysis-preview') ||
          (page === 'editor' && s.page === 'editor')
            ? 'page'
            : undefined
        }
        onClick={() => void w.navigate(page)}
      >
        <Icon name={icon} />
        <span>{label}</span>
      </button>
    ));
  const title =
    [...mainNav, ...improveNav, ...utilityNav].find(([page]) => page === activePage)?.[1] ||
    s.page
      .split('-')
      .map((word) => word[0].toUpperCase() + word.slice(1))
      .join(' ');
  const sectionLinks = (items: [Page, string][], label: string) => (
    <nav className="tabs studio-sections" aria-label={label}>
      {items.map(([page, name]) => (
        <button
          key={page}
          className={`tab ${s.page === page ? 'active' : ''}`}
          aria-current={s.page === page ? 'page' : undefined}
          onClick={() => void w.navigate(page)}
        >
          {name}
        </button>
      ))}
    </nav>
  );
  let content: ReactNode;
  if (s.page === 'models') content = <Models s={s} />;
  else if (s.page === 'project' || s.page === 'welcome' || !s.project)
    content = <ProjectPage s={s} />;
  else if (s.page === 'summary') content = <Summary s={s} />;
  else if (s.page === 'analysis') content = <Analysis s={s} />;
  else if (s.page === 'analysis-files') content = <AnalysisFiles s={s} />;
  else if (s.page === 'analysis-preview') content = <AnalysisPreview s={s} />;
  else if (s.page === 'analysis-run') content = <AnalysisRun s={s} />;
  else if (['bugs', 'performance', 'security'].includes(s.page))
    content = <Results s={s} key={s.page} />;
  else if (s.page === 'chat' || s.page === 'changes') content = <ChangeWorkspace s={s} />;
  else if (s.page === 'history')
    content = (
      <div className="workspace-page">
        <Heading
          title="History"
          detail="Restore saved work without generating or applying changes."
          variant="intro"
        />
        <ChangeHistory s={s} />
      </div>
    );
  else if (s.page === 'features') content = <Features s={s} />;
  else if (s.page === 'instructions') content = <Instructions s={s} />;
  else if (s.page === 'editor') content = <Editor s={s} />;
  else if (s.page === 'context') content = <Context s={s} />;
  else if (s.page === 'scan') content = <Scan s={s} />;
  else if (s.page === 'search') content = <SearchPage s={s} />;
  else if (s.page === 'diagrams') content = <Diagrams s={s} />;
  return (
    <>
      <a className="skip" href="#main">
        Skip to content
      </a>
      <div className="app-window">
        <header className="topbar">
          <button
            className="brand"
            aria-label="Mini-Orca home"
            title={sidebarCollapsed ? 'Mini-Orca home' : undefined}
            onClick={() => void w.navigate(s.project ? 'summary' : 'welcome')}
          >
            <span className="brand-symbol">◒</span>
            <span className="brand-name">
              mini-orca<span className="accent">.</span>
            </span>
          </button>
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
              <span>Search files and commands</span>
              <kbd>⌘ K</kbd>
            </button>
          </div>
        </header>
        <div className="app-layout">
          <aside
            className={`sidebar${sidebarCollapsed ? ' sidebar-collapsed' : ''}`}
            aria-label="Application"
          >
            <button
              className="project-switcher"
              aria-label={`Switch project · ${s.project?.name || 'Open a project'}`}
              title={sidebarCollapsed ? s.project?.name || 'Open a project' : undefined}
              onClick={() => void w.navigate('project')}
            >
              <span className="project-avatar">{s.project?.name?.[0]?.toUpperCase() || '+'}</span>
              <span className="project-label">
                <strong>{s.project?.name || 'Open a project'}</strong>
                {s.project && <small>{s.project.type.toUpperCase()}</small>}
              </span>
              <Icon name="chevrons" />
            </button>
            <div className="sidebar-navigation">
              <nav aria-label="Workspaces">{nav(mainNav)}</nav>
              <div className="nav-label">Improve</div>
              <nav aria-label="Workspaces: Improve">{nav(improveNav)}</nav>
              <div className="sidebar-bottom">
                <div className="nav-label">Project</div>
                <nav aria-label="Tools">{nav(utilityNav.slice(0, 3))}</nav>
              </div>
            </div>
            <nav className="sidebar-settings" aria-label="Settings">
              {nav(utilityNav.slice(3))}
            </nav>
            <p className="sidebar-note">Local by default. You apply changes.</p>
            <button
              className="sidebar-toggle"
              aria-label={sidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
              aria-expanded={!sidebarCollapsed}
              title={sidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
              onClick={() => setSidebarCollapsed((collapsed) => !collapsed)}
            >
              <Icon name="sidebar" />
              <span>Collapse sidebar</span>
            </button>
          </aside>
          <div className="app-body">
            <main id="main" ref={main} tabIndex={-1} className={`studio-main studio-${s.page}`}>
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
              {contextPages.some(([page]) => page === s.page) &&
                sectionLinks(contextPages, 'Context sections')}
              {analysisPages.some(([page]) => page === s.page) &&
                sectionLinks(analysisPages, 'Analysis sections')}
              {findingPages.some(([page]) => page === s.page) &&
                sectionLinks(findingPages, 'Finding categories')}
              <div className="page" data-accent={s.page}>
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
            <div
              hidden={!terminalOpen}
              className="terminal-container"
              role="region"
              aria-label="Terminal drawer"
            >
              <Button
                className="terminal-close"
                tone="ghost"
                icon="close"
                aria-label="Close terminal drawer"
                onClick={closeOverlay}
              />
              <TerminalWorkspace s={live} visible={terminalOpen} />
            </div>
            <footer className="statusbar">
              {' '}
              <div className={`connection ${s.connected ? '' : 'offline'}`}>
                <span className={`status-dot ${s.connected ? '' : 'offline'}`} />
                {s.connected ? 'Daemon connected' : 'Daemon offline'}
              </div>
              <span className="row">
                <Icon name={s.git?.available ? 'branch' : 'laptop'} />
                {s.git?.available ? s.git.branch : 'Local workspace'}
              </span>
              <span>
                {s.project
                  ? `${s.project.source_file_count ?? '—'} source files`
                  : 'No project open'}
              </span>
              <div className="statusbar-end">
                <button
                  className="row"
                  disabled={!s.project}
                  aria-expanded={terminalOpen}
                  onClick={() => (terminalOpen ? closeOverlay() : void w.navigate('terminal'))}
                >
                  <Icon name="terminal" />
                  Terminal
                </button>
                <span>{s.version ? `Daemon ${s.version}` : ''}</span>
                <div className="theme-switcher" role="group" aria-label="Theme">
                  {themes.map((choice) => (
                    <button
                      key={choice.id}
                      aria-label={`${choice.name} theme`}
                      aria-pressed={theme === choice.id}
                      title={`${choice.name} (${choice.letter})`}
                      onClick={() => setTheme(choice.id)}
                    >
                      {choice.letter}
                    </button>
                  ))}
                </div>
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
      </div>
      {live.page === 'analysis-preview' && (
        <Overlay title="Analysis preview" onClose={closeOverlay}>
          <AnalysisPreview s={live} />
        </Overlay>
      )}
      {live.page === 'search' && live.project && (
        <Overlay title="Search files and commands" onClose={closeOverlay}>
          <SearchPage s={live} />
        </Overlay>
      )}
      {live.page === 'project' && live.project && (
        <Overlay title="Switch project" onClose={closeOverlay}>
          <ProjectPage s={live} compact />
        </Overlay>
      )}
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
        <p>Reopen Mini-Orca to reconnect.</p>
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
