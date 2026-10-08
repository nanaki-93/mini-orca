import { useEffect, useState } from 'react';
import { workspace as w, type State } from './workspace';
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
  const preview = s.instructionPreview;
  const [path, setPath] = useState(preview?.path || 'AGENTS.md');
  const [content, setContent] = useState(preview?.existing_content || '');
  const [selected, setSelected] = useState<string[]>([]);
  const [step, setStep] = useState(1);
  const [query, setQuery] = useState('');
  const [category, setCategory] = useState('');
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
  return (
    <div className="workspace-page instructions-page">
      <Heading
        variant="intro"
        title="Project instructions"
        detail="Load root or directory guidance, then preview changes for explicit review in Chat."
      />
      <ol className="wizard-steps" aria-label="Instruction wizard steps">
        {['Choose scope', 'Edit guidance', 'Preview'].map((label, i) => (
          <li key={label} aria-current={step === i + 1 ? 'step' : undefined}>
            {i + 1}. {label}
          </li>
        ))}
      </ol>
      <div className="instruction-grid">
        <section className="stack" aria-label="Instruction wizard">
          {step === 1 && (
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
                  if (await w.loadInstructions(path)) setStep(2);
                }}
              >
                Load scope
              </Button>
            </Panel>
          )}
          {step === 2 && (
            <Panel
              title="Edit guidance"
              actions={<Badge value={preview?.exists ? 'existing file' : 'new file'} />}
            >
              {preview && <p className="path instruction-scope">{path}</p>}
              {!ready ? (
                <Notice error>Load this scope again before editing.</Notice>
              ) : (
                <>
                  <p>
                    Choose individual rules for your AGENTS.md. Sections cover architecture, setup,
                    code style, testing, security and handoff. Project-specific choices use indexed
                    source and build files in this scope; reindex after changing the project.
                  </p>
                  <p className="muted">
                    {presets.length} choices across {categories.length} sections · {matched} matched
                    to this scope. Common project rules are also available.
                  </p>
                  {!matched && (
                    <Notice>
                      No matching indexed files in this scope. Use general or custom guidance.
                    </Notice>
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
                      {pending.length} {pending.length === 1 ? 'guideline' : 'guidelines'} selected
                      · {visible.length} shown
                    </p>
                    <div className="actions">
                      <Button
                        disabled={!!s.busy || excluded || !pending.length}
                        onClick={() => {
                          setContent(addGuidelines(content, pending));
                          setSelected([]);
                        }}
                      >
                        Add selected guidance
                      </Button>
                      <Button
                        disabled={!!s.busy || !pending.length}
                        onClick={() => setSelected([])}
                      >
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
                  {pending.length > 0 && (
                    <Notice>Add selected guidance to the draft before continuing.</Notice>
                  )}
                  <div className="actions">
                    <Button disabled={!!s.busy} onClick={() => setStep(1)}>
                      Back to scope
                    </Button>
                    <Button
                      tone="primary"
                      disabled={!!s.busy || excluded || !proposed.trim() || pending.length > 0}
                      onClick={() => setStep(3)}
                    >
                      Continue to preview
                    </Button>
                  </div>
                </>
              )}
            </Panel>
          )}
          {step === 3 && (
            <Panel title="Instruction preview">
              <strong className="path">{path}</strong>
              <Prose text={proposed} />
              {!ready && <Notice error>Load this scope again before editing.</Notice>}
              <div className="actions">
                <Button disabled={!!s.busy} onClick={() => setStep(2)}>
                  Edit guidance
                </Button>
                <Button
                  tone="primary"
                  disabled={
                    !!s.busy || !ready || excluded || proposed === preview?.existing_content
                  }
                  onClick={() => void w.proposeInstructions(proposed)}
                >
                  Preview instruction diff
                </Button>
              </div>
            </Panel>
          )}
        </section>
        <section className="stack" aria-label="Effective project guidance">
          <Panel title="Registered instructions">
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
      </div>
    </div>
  );
}
