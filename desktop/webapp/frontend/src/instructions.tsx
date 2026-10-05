import { useEffect, useState } from 'react';
import { workspace as w, type State } from './workspace';
import { Badge, Button, Disclosure, Heading, Notice, Panel, Prose } from './ui';

export function Instructions({ s }: { s: State }) {
  const preview = s.instructionPreview;
  const [path, setPath] = useState(preview?.path || 'AGENTS.md');
  const [content, setContent] = useState(preview?.existing_content || '');
  const [selected, setSelected] = useState<string[]>([]);
  const [step, setStep] = useState(1);
  useEffect(() => {
    if (!preview) return;
    setPath(preview.path);
    setContent(preview.existing_content);
    setSelected([]);
  }, [preview]);
  const ready =
    !!preview && preview.path === path && preview.project_revision === s.project?.project_revision;
  const additions =
    preview?.presets
      .filter((preset) => selected.includes(preset.id))
      .map((preset) => `- ${preset.content}`)
      .join('\n') || '';
  const proposed = content;
  const excluded = preview?.effective.excluded.some((file) => file.path === preview.path);
  return (
    <>
      <Heading title="Project instructions" />
      <ol className="wizard-steps" aria-label="Instruction wizard steps">
        {['Choose scope', 'Edit guidance', 'Preview'].map((label, i) => (
          <li key={label} aria-current={step === i + 1 ? 'step' : undefined}>
            {i + 1}. {label}
          </li>
        ))}
      </ol>
      <div className="instruction-grid">
        <section className="stack">
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
              {!ready ? (
                <Notice error>Load this scope again before editing.</Notice>
              ) : (
                <>
                  <label className="block">
                    Guidance
                    <textarea
                      className="instruction-text"
                      aria-label="Custom instructions"
                      value={content}
                      disabled={!!s.busy || excluded}
                      onChange={(e) => setContent(e.target.value)}
                      placeholder="# Project instructions\n\nDescribe the rules agents should follow…"
                    />
                  </label>
                  <fieldset className="instruction-presets">
                    <legend>Add suggested instructions</legend>
                    {preview.presets.map((preset) => (
                      <label className="checkbox-line" key={preset.id}>
                        <input
                          type="checkbox"
                          disabled={!!s.busy || excluded}
                          checked={selected.includes(preset.id)}
                          onChange={(e) =>
                            setSelected(
                              e.target.checked
                                ? [...selected, preset.id]
                                : selected.filter((id) => id !== preset.id),
                            )
                          }
                        />
                        {preset.label}
                      </label>
                    ))}
                  </fieldset>
                  <Button
                    disabled={!!s.busy || excluded || !additions}
                    onClick={() => {
                      setContent(
                        `${content}${content ? '\n\n' : ''}## Additional guidance\n\n${additions}\n`,
                      );
                      setSelected([]);
                    }}
                  >
                    Add selected guidance
                  </Button>
                  <div className="actions">
                    <Button disabled={!!s.busy} onClick={() => setStep(1)}>
                      Back to scope
                    </Button>
                    <Button
                      tone="primary"
                      disabled={!!s.busy || excluded || !proposed.trim()}
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
              <div className="actions section-gap">
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
    </>
  );
}
