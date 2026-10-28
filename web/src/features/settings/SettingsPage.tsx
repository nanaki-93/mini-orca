// Settings: project, trust, models and agents, analysis, appearance.

import { Bot, Coins, FolderOpen, Monitor, Moon, Palette, Play, RefreshCw, ShieldCheck, Sparkles, Sun } from 'lucide-react';
import { useModels, useOverview, useProject, useStatus, useTrust, useUsage } from '@/api/hooks';
import type { UsageTotal } from '@/api/types';
import { costCaveat, costLabel, formatTokens } from '@/lib/usage';
import { useActions, useBusy } from '@/app/actions';
import { useCheckout } from '@/app/checkout';
import { DARK_PALETTES, setUI, useUI, type Theme } from '@/app/store';
import { navigate } from '@/lib/router';
import { plural, timeAgo } from '@/lib/util';
import { Badge, Button, EmptyState, Segmented, Skeleton, cx } from '@/components/ui';
import { OpenProject } from './OpenProject';
import { QuarantineList } from './Quarantine';
import { AgentRoles } from './AgentRoles';
import { ConfigSource } from './ConfigSource';

const SECTIONS = [
  { id: 'project', label: 'Project', icon: FolderOpen },
  { id: 'trust', label: 'Trust', icon: ShieldCheck },
  { id: 'models', label: 'Models & agents', icon: Bot },
  { id: 'analysis', label: 'Analysis', icon: Sparkles },
  { id: 'usage', label: 'Usage', icon: Coins },
  { id: 'appearance', label: 'Appearance', icon: Palette },
];

export function SettingsPage({ section }: { section: string }) {
  const current = SECTIONS.find(s => s.id === section)?.id ?? 'project';
  return (
    <div className="docs">
      <nav className="docs-nav" aria-label="Settings sections">
        <div className="panel-title">Settings</div>
        {SECTIONS.map(s => (
          <a key={s.id} href={`#/settings/${s.id}`} className={cx('docs-link', s.id === current && 'is-active')} onClick={e => { e.preventDefault(); navigate(`/settings/${s.id}`); }}>
            <s.icon size={14} aria-hidden />
            {s.label}
          </a>
        ))}
      </nav>
      <div className="docs-body page-scroll">
        <div className="page-inner page-narrow stack-lg">
          <SettingsSection id={current} />
        </div>
      </div>
    </div>
  );
}

function SettingsSection({ id }: { id: string }) {
  switch (id) {
    case 'project':
      return <ProjectSettings />;
    case 'trust':
      return <TrustSettings />;
    case 'models':
      return <ModelSettings />;
    case 'analysis':
      return <AnalysisSettings />;
    case 'usage':
      return <UsageSettings />;
    default:
      return <Appearance />;
  }
}

/** What agents spent per task, with what is not tied to a task and the total. */
function UsageSettings() {
  const { data: report, isLoading } = useUsage();
  const caveat = costCaveat(report?.total.cost);
  return (
    <>
      <h1 className="page-title">Usage</h1>
      <p className="muted">Model calls and tokens agents spent in this checkout, from the agent journal. Costs are estimates from the price table in the daemon's configuration.</p>
      {isLoading ? (
        <Skeleton lines={4} />
      ) : !report || !report.total.usage.model_calls ? (
        <EmptyState icon={Coins} title="No agent usage yet">Agent runs, revisions and analysis record what they spend here.</EmptyState>
      ) : (
        <div className="card">
          <table className="table" aria-label="Usage by task">
            <thead>
              <tr><th>Task</th><th className="num">Runs</th><th className="num">Calls</th><th className="num">Tokens in</th><th className="num">Tokens out</th><th className="num">Cost</th></tr>
            </thead>
            <tbody>
              {report.tasks.map(task => (
                <tr key={task.session_id} className="table-row" onClick={() => navigate(`/tasks/${task.session_id}`)}>
                  <td className="truncate" title={task.title}>{task.title}</td>
                  <UsageCells total={task} />
                </tr>
              ))}
              {report.other.usage.model_calls ? (
                <tr>
                  <td className="muted">Other agent work (analysis, explanations, plans)</td>
                  <UsageCells total={report.other} />
                </tr>
              ) : null}
            </tbody>
            <tfoot>
              <tr>
                <td className="strong">Total</td>
                <UsageCells total={report.total} />
              </tr>
            </tfoot>
          </table>
          {caveat ? <div className="card-foot xs muted">{caveat}</div> : null}
        </div>
      )}
    </>
  );
}

function UsageCells({ total }: { total: UsageTotal }) {
  return (
    <>
      <td className="num">{total.runs}</td>
      <td className="num">{total.usage.model_calls}</td>
      <td className="num">{formatTokens(total.usage.input_tokens)}</td>
      <td className="num">{formatTokens(total.usage.output_tokens)}</td>
      <td className="num">{costLabel(total.cost) || '—'}</td>
    </>
  );
}

function ProjectSettings() {
  const project = useProject().data;
  const status = useStatus().data;
  const { worktreePath } = useCheckout();
  const actions = useActions();
  const busy = useBusy();
  return (
    <>
      <h1 className="page-title">Project</h1>
      {project ? (
        <div className="card">
          <div className="card-body">
            <dl className="details details-wide">
              <dt>Name</dt><dd>{project.name}</dd>
              <dt>Path</dt><dd className="mono small">{project.path}</dd>
              <dt>Type</dt><dd>{project.type}{project.build_file ? ` · ${project.build_file}` : ''}</dd>
              <dt>Files</dt><dd>{plural(project.file_count, 'file')}{project.total_lines ? ` · ${project.total_lines.toLocaleString()} lines` : ''}</dd>
              <dt>Daemon</dt><dd>v{status?.version}</dd>
            </dl>
          </div>
          <div className="card-foot">
            <Button icon={RefreshCw} onClick={() => void actions.reindex(project, worktreePath)} disabled={Boolean(busy)}>Reindex files</Button>
          </div>
        </div>
      ) : null}
      <QuarantineList />
      <OpenProject />
    </>
  );
}

function TrustSettings() {
  const project = useProject().data;
  const trust = useTrust().data;
  const { worktreePath } = useCheckout();
  const actions = useActions();
  const busy = useBusy();
  return (
    <>
      <h1 className="page-title">Trust</h1>
      <p className="muted">Agents verify their work by running this project's tests and lint. That executes project code, so it needs your permission. Commands always run in an isolated copy of the project, never in your checkout.</p>
      <div className="card">
        <div className="card-head">
          <ShieldCheck size={15} className={trust?.trusted ? 'tone-success' : 'faint'} aria-hidden />
          <span className="strong">{trust?.trusted ? 'Checks are allowed' : 'Checks are not allowed'}</span>
          <span className="spacer" />
          {project ? (
            <Button variant={trust?.trusted ? 'ghost' : 'primary'} onClick={() => void actions.setTrust(project, worktreePath, trust, !trust?.trusted)} disabled={Boolean(busy)}>
              {trust?.trusted ? 'Revoke' : 'Allow checks'}
            </Button>
          ) : null}
        </div>
        <div className="card-body">
          <div className="label-row">Commands that will run</div>
          <ul className="commands">{(trust?.commands ?? []).map((argv, i) => <li key={i} className="mono small">{argv.join(' ')}</li>)}</ul>
          <p className="xs muted">Trust is remembered for this project path until you revoke it; worktrees ask separately.</p>
        </div>
      </div>
    </>
  );
}

function ModelSettings() {
  const models = useModels().data;
  return (
    <>
      <h1 className="page-title">Models & agents</h1>
      <p className="muted">Model scopes are configured in the daemon's <span className="mono">config.yaml</span>. Choose which model each agent role uses for this project below; remote models receive bounded project context and always ask before each run.</p>
      <ConfigSource models={models} />
      <div className="card">
        <div className="card-head"><Sparkles size={15} aria-hidden /><span className="strong">Model scopes</span></div>
        <table className="table">
          <thead><tr><th>Scope</th><th>Model</th><th>Where</th><th>Timeout</th></tr></thead>
          <tbody>
            {Object.values(models?.scopes ?? {}).map(m => (
              <tr key={m.scope}>
                <td className="strong-row">{m.scope}</td>
                <td className="mono small">{m.model}</td>
                <td>{m.remote_provider ? <Badge tone="warning">remote · {m.provider_origin}</Badge> : <Badge tone="success">local</Badge>}</td>
                <td className="small muted">{m.timeout}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <AgentRoles />
    </>
  );
}

function AnalysisSettings() {
  const overview = useOverview().data;
  const project = useProject().data;
  const { worktreePath } = useCheckout();
  const actions = useActions();
  const busy = useBusy();
  const coverage = overview?.analysis_coverage ?? {};
  return (
    <>
      <h1 className="page-title">Analysis</h1>
      <p className="muted">Analysis reads the project to find bugs, performance and security issues, and writes the handbook in Docs.</p>
      <div className="card">
        <div className="card-head">
          <Sparkles size={15} aria-hidden />
          <span className="strong">Coverage</span>
          <span className="spacer" />
          <Button onClick={() => navigate('/analysis')}>Open dashboard</Button>
          {project ? <Button variant="primary" icon={Play} onClick={() => void actions.runAnalysis(project, worktreePath)} disabled={Boolean(busy)}>Analyze project</Button> : null}
        </div>
        <div className="card-body">
          <div className="stat-row">
            {['fresh', 'stale', 'missing', 'failed', 'running'].map(key => (
              <div key={key} className="stat"><div className="stat-value">{coverage[key] ?? 0}</div><div className="stat-label">{key}</div></div>
            ))}
          </div>
          {overview?.analysis_run ? <p className="small muted">Last run {overview.analysis_run.status}{overview.analysis_run.updated_at ? ` · ${timeAgo(overview.analysis_run.updated_at)}` : ''}{overview.analysis_run.reason ? ` — ${overview.analysis_run.reason}` : ''}</p> : null}
          {overview?.analysis?.failure ? <p className="small tone-danger">{overview.analysis.failure}</p> : null}
        </div>
      </div>
    </>
  );
}

function Appearance() {
  const theme = useUI(s => s.theme);
  const palette = useUI(s => s.darkPalette);
  const layout = useUI(s => s.diffLayout);
  return (
    <>
      <h1 className="page-title">Appearance</h1>
      <div className="card">
        <div className="card-body stack-lg">
          <div className="row"><span className="grow">Theme</span><Segmented<Theme> label="Theme" value={theme} onChange={value => setUI({ theme: value })} options={[{ value: 'system', label: 'System', icon: Monitor }, { value: 'dark', label: 'Dark', icon: Moon }, { value: 'light', label: 'Light', icon: Sun }]} /></div>
          <div className="stack">
            <div className="row">
              <span className="grow">Dark palette</span>
              {theme === 'light' ? <span className="xs muted">Used when the theme is dark</span> : null}
            </div>
            <div className="palette-grid" role="radiogroup" aria-label="Dark palette">
              {DARK_PALETTES.map(option => (
                <button
                  key={option.id}
                  type="button"
                  role="radio"
                  aria-checked={palette === option.id}
                  className={cx('palette-swatch', palette === option.id && 'is-on')}
                  onClick={() => setUI({ darkPalette: option.id, theme: theme === 'light' ? 'dark' : theme })}
                >
                  <span className="swatch-preview" style={{ background: option.chrome }} aria-hidden>
                    <span className="swatch-panel" style={{ background: option.surface }}>
                      <span className="swatch-accent" style={{ background: option.accent }} />
                    </span>
                  </span>
                  <span className="small">{option.label}</span>
                </button>
              ))}
            </div>
          </div>
          <div className="row"><span className="grow">Diff layout</span><Segmented label="Diff layout" value={layout} onChange={value => setUI({ diffLayout: value })} options={[{ value: 'unified', label: 'Unified' }, { value: 'split', label: 'Split' }]} /></div>
        </div>
      </div>
    </>
  );
}
