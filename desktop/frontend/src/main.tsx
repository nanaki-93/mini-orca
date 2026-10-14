import { GitWorkspace } from './git-workspace';
import { WorktreeImplementation } from './worktree-implementation';
import {
  Component,
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
  type ReactNode,
} from 'react';
import { createRoot } from 'react-dom/client';
import { workspace as w, canCancelOperation, findingCategory, type Page } from './workspace';
import { Button, Heading, Icon, Modal, Notice, Overlay, Spinner } from './ui';
import { Summary, ProjectPage, Models, Diagrams, SearchPage } from './overview';
import { DocsWorkspace } from './docs';
import { Analysis, AnalysisFiles, AnalysisPreview, AnalysisRun } from './analysis';
import { Results, selectedResult } from './results';
import { Editor, Context } from './editor';
import { ChangeHistory } from './change-shared';
import { Instructions } from './instructions';
import { TerminalWorkspace } from './tools';
import './style.css';
import './studio.css';

const mainNav: [Page, string, string][] = [
  ['summary', 'Overview', 'grid'],
  ['docs', 'Docs', 'book'],
  ['editor', 'Code & Diff', 'code'],
  ['bugs', 'Issues & Ideas', 'bug'],
  ['git', 'Git & Worktrees', 'branch'],
  ['instructions', 'Settings & Rules', 'shield'],
];
const contextPages: [Page, string][] = [
  ['analysis-files', 'Files to skip / Scope'],
  ['analysis-run', 'Last Run'],
  ['context', 'Provider context'],
];
const findingPages: [Page, string][] = [
  ['bugs', 'Bugs'],
  ['security', 'Security'],
  ['performance', 'Performance'],
];
const themes = [
  {
    id: 'amethyst',
    name: 'Amethyst',
    letter: 'A',
    mark: '/mo-mark-amethyst.svg',
  },
  {
    id: 'tokyo-night',
    name: 'Tokyo Night',
    letter: 'T',
    mark: '/mo-mark-tokyo-night.svg',
  },
  {
    id: 'linear-carbon',
    name: 'Linear Carbon',
    letter: 'L',
    mark: '/mo-mark-linear-carbon.svg',
  },
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
    return themes.find((choice) => choice.id === saved)?.id || 'amethyst';
  });
  const appearance = themes.find((choice) => choice.id === theme) || themes[0];
  const [large, setLarge] = useState(localStorage.getItem('mini-orca:large') === 'true');
  const [sidebarCollapsed, setSidebarCollapsed] = useState(
    () => localStorage.getItem('mini-orca:sidebar-collapsed') === 'true',
  );
  const main = useRef<HTMLElement>(null);
  const category = findingCategory(s);
  const finding = s.page === 'finding' ? selectedResult(s) : undefined;
  const categoryTitle = findingPages.find(([page]) => page === category)?.[1];
  const activePage: Page =
    category ||
    s.page === 'finding' ||
    s.page === 'features' ||
    ['analysis', 'analysis-preview', 'analysis-run'].includes(s.page)
      ? 'bugs'
      : ['docs', 'diagrams'].includes(s.page)
        ? 'docs'
        : ['editor', 'chat', 'changes', 'history'].includes(s.page)
          ? 'editor'
          : ['git', 'worktree'].includes(s.page)
            ? 'git'
            : ['instructions', 'models', 'analysis-files', 'context'].includes(s.page)
              ? 'instructions'
              : s.page;
  useEffect(() => {
    void w.start();
    const focus = () => {
      void w.refreshFile();
      void w.refreshRepository();
    };
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
  }, [s.page, finding?.key]);
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
  const issueCount =
    (s.findings?.filter((f) => f.status !== 'fixed').length ||
      s.run?.sections.reduce((sum, sec) => sum + (sec.finding_count || 0), 0) ||
      0) + (s.features?.suggestions.length || 0);
  const worktreeCount = s.gitWorkspace?.worktrees.length || 0;
  const changeCount = s.change?.changes?.length || 0;
  const nav = (items: typeof mainNav) =>
    items.map(([page, label, icon]) => {
      const count =
        page === 'editor' && changeCount > 0
          ? `${changeCount} diff`
          : page === 'bugs' && issueCount > 0
            ? issueCount
            : page === 'git' && worktreeCount > 0
              ? worktreeCount
              : undefined;
      return (
        <button
          key={page}
          className="nav-link"
          data-accent={page}
          aria-label={label}
          title={sidebarCollapsed ? label : undefined}
          disabled={!s.project && !['project', 'models', 'instructions'].includes(page)}
          aria-current={activePage === page ? 'page' : undefined}
          onClick={() => void w.navigate(page)}
        >
          <Icon name={icon} />
          <span>{label}</span>
          {count !== undefined && <span className="nav-count badge-count">{count}</span>}
        </button>
      );
    });
  const title =
    mainNav.find(([page]) => page === activePage)?.[1] ||
    (contextPages.find(([page]) => page === s.page)?.[1] ??
      s.page
        .split('-')
        .map((word) => word[0].toUpperCase() + word.slice(1))
        .join(' '));
  let content: ReactNode;
  if (s.page === 'models') content = <Models s={s} />;
  else if (s.page === 'project' || s.page === 'welcome' || !s.project)
    content = <ProjectPage s={s} />;
  else if (s.page === 'git') content = <GitWorkspace s={s} />;
  else if (s.page === 'worktree' && s.implementation)
    content = <WorktreeImplementation child={s.implementation} />;
  else if (s.page === 'summary') content = <Summary s={s} />;
  else if (s.page === 'docs') content = <DocsWorkspace s={s} />;
  else if (s.page === 'analysis') content = <Analysis s={s} />;
  else if (s.page === 'analysis-files') content = <AnalysisFiles s={s} />;
  else if (s.page === 'analysis-preview') content = <AnalysisPreview s={s} />;
  else if (s.page === 'analysis-run') content = <AnalysisRun s={s} />;
  else if (category || s.page === 'finding' || s.page === 'features')
    content = <Results s={s} key={`${s.project.project_id}:${category || s.page}`} />;
  else if (s.page === 'editor' || s.page === 'chat' || s.page === 'changes')
    content = <Editor s={s} />;
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
  else if (s.page === 'instructions') content = <Instructions s={s} />;
  else if (s.page === 'context') content = <Context s={s} />;
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
            <img className="brand-symbol" src={appearance.mark} alt="" width="41" height="28" />
            <span className="brand-name">
              mini-orca<span className="accent">.</span>
            </span>
          </button>
          <nav className="breadcrumbs" aria-label="Breadcrumb">
            <span className="muted">{s.project?.name || 'Mini-Orca'}</span>
            <span className="muted" aria-hidden="true">
              /
            </span>
            {category && s.page === 'finding' ? (
              <>
                <button onClick={() => void w.navigate('bugs')}>Issues & Ideas</button>
                <span className="muted" aria-hidden="true">
                  /
                </span>
                <button onClick={() => void w.navigate(category)}>{categoryTitle}</button>
                <span className="muted" aria-hidden="true">
                  /
                </span>
                <span aria-current="page">{finding?.name || 'Finding unavailable'}</span>
              </>
            ) : s.page === 'analysis-run' ? (
              <>
                <button onClick={() => void w.navigate('bugs')}>Issues & Ideas</button>
                <span className="muted" aria-hidden="true">
                  /
                </span>
                <span aria-current="page">Last Run</span>
              </>
            ) : (
              <span aria-current="page">{title}</span>
            )}
          </nav>
          <div className="topbar-actions">
            <div className="theme-pill-group" role="group" aria-label="Theme">
              {themes.map((choice) => (
                <button
                  key={choice.id}
                  className="theme-pill-btn"
                  aria-label={`${choice.name} theme`}
                  aria-pressed={theme === choice.id}
                  title={`${choice.name} (${choice.letter})`}
                  onClick={() => setTheme(choice.id)}
                >
                  {choice.name}
                </button>
              ))}
            </div>
            <button
              className="search-trigger"
              onClick={() => void w.navigate('search')}
              aria-label="Search files and commands"
            >
              <Icon name="search" />
              <span>Command Palette</span>
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
              </span>
              <Icon name="chevrons" />
            </button>
            {s.project && (
              <button
                className="project-branch"
                title={s.repository?.head || 'Git status'}
                aria-label={`Current branch: ${s.repository?.branch || 'unavailable'}. Open Git & Worktrees`}
                onClick={() => void w.navigate('git')}
              >
                <Icon name="branch" />
                <span>
                  {s.resourceErrors.Git
                    ? 'Git unavailable'
                    : !s.repository
                      ? 'Loading branch…'
                      : !s.repository.available
                        ? 'No Git repository'
                        : s.repository.branch || 'Detached HEAD'}
                </span>
              </button>
            )}
            <div className="sidebar-navigation">
              <div className="nav-label">Workspace</div>
              <nav aria-label="Workspaces">{nav(mainNav)}</nav>
            </div>
            <p className="sidebar-note">
              {s.workspaceTrust !== false
                ? 'Trusted workspace · In-place + Undo'
                : 'Untrusted workspace · Review before execution'}
            </p>
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
                  <Spinner />
                  {s.busy}…
                  {canCancelOperation(s.busy) && (
                    <Button tone="ghost small" onClick={() => void w.cancel()}>
                      Cancel
                    </Button>
                  )}
                </div>
              )}
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
                <Icon name="shield" />
                {s.workspaceTrust !== false ? 'Workspace trusted' : 'Execution untrusted'}
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
          {live.error && <Notice error>{live.error}</Notice>}
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
          {live.error && <Notice error>{live.error}</Notice>}
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
