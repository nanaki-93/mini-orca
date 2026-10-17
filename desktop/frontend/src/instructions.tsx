import { useWorkspace } from './workspace-context';
import { useEffect, useState } from 'react';
import { evaluateBootstrapChecklist, type State } from './workspace';
import { Badge, Button, Disclosure, Heading, Notice, Panel, Prose } from './ui';
import type { InstructionPreview } from './models';

function containsGuideline(text: string, guideline: string) {
  const normalize = (value: string) => value.trim().replace(/\s+/g, ' ');
  return normalize(text).includes(normalize(guideline));
}

function addGuidelines(content: string, presets: InstructionPreview['presets']) {
  const groups = new Map<string, string[]>();
  for (const preset of presets) {
    const category = preset.category || '';
    groups.set(category, [...(groups.get(category) || []), `- ${preset.content}`]);
  }
  let draft = content || '# Project instructions';
  for (const [category, lines] of groups) {
    draft = insertGuidelineSection(draft, category, lines.join('\n'));
  }
  return draft;
}

function insertGuidelineSection(content: string, category: string, additions: string) {
  // Only merge with a real level-two heading, never a heading shown in a code example.
  let fence = '',
    offset = 0;
  for (const line of content.split('\n')) {
    const marker = line.match(/^ {0,3}(`{3,}|~{3,})(.*)$/);
    if (marker) {
      if (!fence) fence = marker[1];
      else if (marker[1][0] === fence[0] && marker[1].length >= fence.length && !marker[2].trim())
        fence = '';
    } else if (!fence && category) {
      const heading = line.match(/^ {0,3}##[ \t]+(.+?)\s*$/)?.[1].replace(/[ \t]+#+$/, '');
      if (heading?.toLowerCase() === category.toLowerCase()) {
        const end = offset + line.length;
        return `${content.slice(0, end)}\n\n${additions}\n${content.slice(end)}`;
      }
    }
    offset += line.length + 1;
  }
  return `${content}\n\n${category ? `## ${category}\n\n` : ''}${additions}\n`;
}

export function Instructions({ s }: { s: State }) {
  const w = useWorkspace();
  const preview = s.instructionPreview;
  const [tab, setTab] = useState<'rules' | 'guidelines' | 'effective' | 'bootstrap'>(
    s.instructionsTab || 'rules',
  );
  const [path, setPath] = useState(preview?.path || 'AGENTS.md');
  const [content, setContent] = useState(preview?.existing_content || '');
  const [selected, setSelected] = useState<string[]>([]);
  const [editing, setEditing] = useState(false);
  const [query, setQuery] = useState('');
  const [category, setCategory] = useState('');
  useEffect(() => {
    if (s.instructionsTab) {
      setTab(s.instructionsTab);
      w.set({ instructionsTab: undefined });
    }
  }, [s.instructionsTab]);
  useEffect(() => {
    if (!preview) return;
    setPath(preview.path);
    setContent(preview.existing_content);
    setSelected([]);
    setQuery('');
    setCategory('');
  }, [preview]);
  const ready =
    !!preview &&
    preview.path === path &&
    preview.project_id === s.project?.project_id &&
    preview.project_revision === s.project?.project_revision;
  const presets = (preview?.presets || []).map((preset) => ({
    ...preset,
    present: containsGuideline(content, preset.content)
      ? path
      : preview?.effective.files.find(
          (file) => file.path !== path && containsGuideline(file.content, preset.content),
        )?.path,
  }));
  const pending = presets.filter((preset) => !preset.present && selected.includes(preset.id));
  const categories = [...new Set(presets.map((p) => p.category || 'General guidelines'))];
  const search = query.trim().toLowerCase();
  const visible = presets.filter(
    (p) =>
      (!category || (p.category || 'General guidelines') === category) &&
      [p.label, p.content, p.category, p.reason, ...(p.evidence || [])]
        .join(' ')
        .toLowerCase()
        .includes(search),
  );
  const groups = categories
    .map((label) => ({
      label,
      items: visible.filter((p) => (p.category || 'General guidelines') === label),
    }))
    .filter((group) => group.items.length > 0);
  const matched = presets.filter((p) => p.evidence?.length).length;
  const proposed = content;
  const excluded = preview?.effective.excluded.some((file) => file.path === preview.path);
  const bootstrapPillars = evaluateBootstrapChecklist(s);
  const readyPillarsCount = bootstrapPillars.filter((p) => p.ready).length;
  const missingPillarsCount = bootstrapPillars.length - readyPillarsCount;

  const guidelinePicker = (switchToEditorOnAdd = false) => (
    <div className="stack">
      {!matched && (
        <Notice>No matching indexed files in this scope. Use general or custom guidance.</Notice>
      )}
      <div className="instruction-filters">
        <label className="block">
          Find guidelines
          <input
            className="field"
            type="search"
            value={query}
            disabled={!!s.busy}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search rules, stacks or file paths"
          />
        </label>
        <label className="block">
          AGENTS.md section
          <select
            className="field"
            aria-label="AGENTS.md section"
            value={category}
            disabled={!!s.busy}
            onChange={(e) => setCategory(e.target.value)}
          >
            <option value="">All sections</option>
            {categories.map((name) => (
              <option key={name} value={name}>
                {name}
              </option>
            ))}
          </select>
        </label>
      </div>
      <div className="instruction-selection">
        <p role="status" className="muted">
          {pending.length} {pending.length === 1 ? 'guideline' : 'guidelines'} selected ·{' '}
          {visible.length} shown
        </p>
        <div className="actions">
          <Button
            tone="primary"
            disabled={!!s.busy || excluded || !pending.length}
            onClick={() => {
              setContent(addGuidelines(content, pending));
              setSelected([]);
              if (switchToEditorOnAdd) {
                setEditing(true);
                setTab('rules');
              }
            }}
          >
            Add selected guidance
          </Button>
          <Button disabled={!!s.busy || !pending.length} onClick={() => setSelected([])}>
            Clear selection
          </Button>
        </div>
        {pending.length > 0 && (
          <p className="muted">Selected: {pending.map((p) => p.label).join(' · ')}</p>
        )}
      </div>
      {groups.length === 0 && (
        <Notice>No guidelines match these filters. Your selection is retained.</Notice>
      )}
      {groups.map((group) => (
        <fieldset className="instruction-presets" key={group.label}>
          <legend>{group.label}</legend>
          {group.items.map((preset) => (
            <label className="instruction-option" key={preset.id}>
              <input
                type="checkbox"
                aria-label={preset.label}
                aria-describedby={`guideline-${preset.id}`}
                disabled={!!s.busy || excluded || !!preset.present}
                checked={!preset.present && selected.includes(preset.id)}
                onChange={(e) =>
                  setSelected(
                    e.target.checked
                      ? [...selected, preset.id]
                      : selected.filter((id) => id !== preset.id),
                  )
                }
              />
              <span className="instruction-option-copy" id={`guideline-${preset.id}`}>
                <strong>{preset.label}</strong>
                {preset.reason && <span className="muted">{preset.reason}</span>}
                {!!preset.evidence?.length && (
                  <span className="path muted">{preset.evidence.join(' · ')}</span>
                )}
                <span>{preset.content}</span>
                {preset.present && (
                  <span className="muted">
                    {preset.present === path
                      ? 'Already in this draft'
                      : `Inherited from ${preset.present}`}
                  </span>
                )}
              </span>
            </label>
          ))}
        </fieldset>
      ))}
    </div>
  );

  return (
    <div className="workspace-page instructions-page">
      <Heading
        variant="intro"
        title="Settings & Project Rules (AGENTS.md)"
        detail="Consolidated workspace trust and project-wide agent rules."
      >
        <Button
          tone="ghost"
          icon="folder"
          disabled={!!s.busy}
          onClick={() => void w.navigate('analysis-files')}
        >
          File Scope
        </Button>
        <Button
          disabled={!!s.busy || !ready}
          onClick={() => {
            setTab('rules');
            setEditing(!editing);
          }}
        >
          {editing ? 'Read guidance' : 'Edit draft'}
        </Button>
      </Heading>

      <div className="toolbar">
        <div className="mode-switch-group" role="tablist" aria-label="Settings and rules views">
          <button
            type="button"
            role="tab"
            aria-selected={tab === 'rules'}
            className={`mode-switch-btn ${tab === 'rules' ? 'active' : ''}`}
            onClick={() => setTab('rules')}
          >
            1. Workspace &amp; Rules
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={tab === 'guidelines'}
            className={`mode-switch-btn ${tab === 'guidelines' ? 'active' : ''}`}
            onClick={() => setTab('guidelines')}
          >
            2. Guideline Library ({presets.length})
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={tab === 'effective'}
            className={`mode-switch-btn ${tab === 'effective' ? 'active' : ''}`}
            onClick={() => setTab('effective')}
          >
            3. Active Scope ({preview?.effective.files.length || 0})
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={tab === 'bootstrap'}
            className={`mode-switch-btn ${tab === 'bootstrap' ? 'active' : ''}`}
            onClick={() => setTab('bootstrap')}
          >
            4. Repo Bootstrap ({readyPillarsCount}/4)
          </button>
        </div>
      </div>

      {tab === 'rules' && (
        <>
          <Panel
            title="Consolidated Workspace Trust"
            actions={
              <Badge
                value={s.workspaceTrust ? 'Trusted Workspace' : 'Confirmations Active'}
                tone={s.workspaceTrust ? 'green' : 'violet'}
              />
            }
          >
            <div className="row between wrap" style={{ gap: '16px' }}>
              <div>
                <p className="small muted">
                  Unifies remote AI provider consent, AI security scan consent, and local test
                  execution trust into a single workspace-level setting so running analyses and
                  prompts is frictionless.
                </p>
              </div>
              <label
                className="row"
                style={{ gap: '8px', cursor: 'pointer', fontWeight: 600, whiteSpace: 'nowrap' }}
              >
                <input
                  type="checkbox"
                  aria-label="Consolidated Workspace Trust"
                  checked={!!s.workspaceTrust}
                  onChange={(e) => void w.setWorkspaceTrust(e.target.checked)}
                />
                <span>Trust this workspace</span>
              </label>
            </div>
          </Panel>

          <section className="stack" aria-label="Instruction draft">
            <Panel title="Choose instruction scope">
              <label className="block">
                Project-relative AGENTS.md path
                <input
                  className="field"
                  aria-label="Instruction path"
                  value={path}
                  disabled={!!s.busy}
                  onChange={(e) => {
                    setPath(e.target.value);
                    w.clearInstructionScope();
                  }}
                  placeholder="AGENTS.md or internal/AGENTS.md"
                />
              </label>
              <Button
                tone="primary"
                disabled={!!s.busy || !path.trim()}
                onClick={async () => {
                  if (await w.loadInstructions(path)) setEditing(true);
                }}
              >
                Load scope
              </Button>
            </Panel>
            {!editing && ready && (
              <section className="instruction-reading">
                <div className="row between">
                  <strong>{path}</strong>
                  <Badge value={preview?.exists ? 'Included' : 'New file'} />
                </div>
                <Prose text={content || 'No instructions saved in this scope.'} />
              </section>
            )}
            {editing && (
              <Panel
                title="Edit guidance"
                actions={<Badge value={preview?.exists ? 'existing file' : 'new file'} />}
              >
                {preview && <p className="path instruction-scope">{path}</p>}
                {!ready ? (
                  <Notice error>Load this scope again before editing.</Notice>
                ) : (
                  <>
                    <label className="block">
                      Guidance draft
                      <textarea
                        className="instruction-text"
                        aria-label="Custom instructions"
                        value={content}
                        disabled={!!s.busy || excluded}
                        onChange={(e) => setContent(e.target.value)}
                        placeholder="# Project instructions\n\nDescribe the rules agents should follow…"
                      />
                    </label>
                    <Disclosure title="Add project guidelines">{guidelinePicker(false)}</Disclosure>
                    {pending.length > 0 && (
                      <Notice>Add selected guidance to the draft before continuing.</Notice>
                    )}
                    <div className="actions">
                      <Button
                        tone="primary"
                        disabled={
                          !!s.busy ||
                          !ready ||
                          excluded ||
                          !proposed.trim() ||
                          pending.length > 0 ||
                          proposed === preview?.existing_content
                        }
                        onClick={() => void w.proposeInstructions(proposed)}
                      >
                        Preview instruction diff
                      </Button>
                      <Button
                        tone="ghost"
                        disabled={
                          !!s.busy ||
                          !ready ||
                          excluded ||
                          !proposed.trim() ||
                          pending.length > 0 ||
                          proposed === preview?.existing_content
                        }
                        onClick={() => void w.saveDirectFile(path, proposed)}
                      >
                        Save Rules Directly
                      </Button>
                    </div>
                  </>
                )}
              </Panel>
            )}
          </section>
        </>
      )}

      {tab === 'guidelines' && (
        <Panel
          title={`Guideline Library for ${path}`}
          actions={<Badge value={`${presets.length} presets · ${matched} matched`} tone="violet" />}
        >
          {!ready ? (
            <Notice error>Load an instruction scope in Workspace &amp; Rules first.</Notice>
          ) : (
            guidelinePicker(true)
          )}
        </Panel>
      )}

      {tab === 'effective' && (
        <section className="stack" aria-label="Effective project guidance">
          <Panel
            title={`Registered instructions (${preview?.effective.files.length || 0} active files)`}
          >
            {!preview && <p>Load a scope to inspect its effective project guidance.</p>}
            {preview?.effective.files.length === 0 && <p>No applicable instruction files yet.</p>}
            {preview?.effective.files.map((file) => (
              <Disclosure key={file.path} title={`${file.path} · scope ${file.scope}`}>
                <Prose text={file.content} />
              </Disclosure>
            ))}
            {preview?.effective.excluded.map((file) => (
              <Notice key={file.path}>
                {file.path}: {file.reason || 'Excluded by context policy'}
              </Notice>
            ))}
            {excluded && (
              <Notice error>This AGENTS.md is excluded by project context policy.</Notice>
            )}
          </Panel>
        </section>
      )}

      {tab === 'bootstrap' && (
        <Panel
          title="Environment Bootstrap Checklist"
          actions={
            <Badge
              value={`${readyPillarsCount}/4 Pillars Ready`}
              tone={readyPillarsCount === 4 ? 'green' : 'amber'}
            />
          }
        >
          <div className="stack">
            <div className="row between wrap" style={{ gap: '12px' }}>
              <p className="small muted">
                Standardizes repository entrypoints (<code>justfile</code>), pinned toolchains (
                <code>.mise.toml</code>), concise agent contracts (<code>AGENTS.md</code> &lt;150
                lines), and fast parallel pre-commit gates (<code>lefthook.yml</code> +{' '}
                <code>gitleaks</code>).
              </p>
              <div className="actions">
                {missingPillarsCount > 0 && (
                  <Button
                    tone="primary"
                    disabled={!!s.busy}
                    onClick={() => void w.bootstrapMissingPillars()}
                  >
                    Bootstrap Missing Pillars ({missingPillarsCount})
                  </Button>
                )}
                <Button
                  tone="ghost"
                  disabled={!!s.busy}
                  onClick={() => void w.runBootstrapToolchain()}
                >
                  Run mise trust &amp;&amp; mise install
                </Button>
              </div>
            </div>
            {s.bootstrapExec && s.bootstrapExec.steps.length > 0 && (
              <div
                className="row wrap"
                style={{ gap: '8px' }}
                aria-label="Bootstrap toolchain status"
              >
                {s.bootstrapExec.steps.map((step) => (
                  <Badge
                    key={step.command}
                    value={`${step.command}: ${step.state}`}
                    tone={
                      step.state === 'passed' ? 'green' : step.state === 'failed' ? 'red' : 'amber'
                    }
                  />
                ))}
              </div>
            )}
            <div className="stack" role="list" aria-label="Environment bootstrap checklist">
              {bootstrapPillars.map((pillar) => (
                <div
                  key={pillar.id}
                  role="listitem"
                  className="row between wrap"
                  style={{
                    gap: '12px',
                    padding: '10px 12px',
                    border: '1px solid var(--line)',
                    borderRadius: '8px',
                    background: 'var(--bg-soft)',
                  }}
                >
                  <div className="list-copy" style={{ flex: '1 1 280px' }}>
                    <div className="row wrap" style={{ gap: '8px' }}>
                      <strong>{pillar.title}</strong>
                      <Badge value={pillar.statusLabel} tone={pillar.ready ? 'green' : 'amber'} />
                    </div>
                    <p className="small muted">{pillar.summary}</p>
                  </div>
                  <div className="actions">
                    {!pillar.ready && (
                      <Button
                        tone="primary"
                        disabled={!!s.busy}
                        onClick={() => void w.bootstrapPillar(pillar.id)}
                      >
                        Bootstrap {pillar.targetPath}
                      </Button>
                    )}
                    {pillar.id === 'contract' && (
                      <Button
                        tone="ghost"
                        disabled={!!s.busy}
                        onClick={async () => {
                          setPath('AGENTS.md');
                          if (!ready || path !== 'AGENTS.md') {
                            await w.loadInstructions('AGENTS.md');
                          }
                          setContent(pillar.templateContent);
                          setEditing(true);
                          setTab('rules');
                        }}
                      >
                        Load Concise Template
                      </Button>
                    )}
                    {pillar.ready && pillar.detectedPath && (
                      <Button
                        tone="ghost"
                        disabled={!!s.busy}
                        onClick={() => void w.openFile(pillar.detectedPath!)}
                      >
                        Inspect {pillar.detectedPath}
                      </Button>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>
        </Panel>
      )}
    </div>
  );
}
