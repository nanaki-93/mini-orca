import { describe, expect, it } from 'vitest';
import { DEFAULT_ADDRESS, folderGroups, modelHint, modelProblem, modelSuggestions, withProvider } from './setup';

describe('setup', () => {
  it('drops the fields a new provider does not use and keeps the same provider’s', () => {
    const local = { provider: '', api_base_url: 'http://127.0.0.1:1234/v1', api_key_set: true, model: 'qwen', reasoning_effort: 'low' };
    expect(withProvider(local, 'agy')).toEqual({ provider: 'agy', model: '', api_base_url: undefined, api_key_set: false, vertex_project: undefined, vertex_location: undefined, cli_path: undefined, reasoning_effort: undefined, context_max_tokens: undefined });
    expect(withProvider(local, 'openai')).toMatchObject({ provider: '', model: 'qwen', api_base_url: 'http://127.0.0.1:1234/v1', api_key_set: true, reasoning_effort: 'low' });
    expect(withProvider({ provider: 'agy', model: 'x' }, 'openai').api_base_url).toBe(DEFAULT_ADDRESS);
  });

  it('says what a scope still needs', () => {
    expect(modelProblem({ provider: '', model: '' })).toBe('Choose a model.');
    expect(modelProblem({ provider: '', model: 'm', api_base_url: '' })).toBe('Enter the API address.');
    expect(modelProblem({ provider: 'gemini', model: 'm' })).toBe('Enter an API key.');
    expect(modelProblem({ provider: 'gemini', model: 'm', api_key_set: true })).toBe('');
    expect(modelProblem({ provider: 'vertex', model: 'm', vertex_project: 'p' })).toBe('Enter an API key, or a project and location.');
    expect(modelProblem({ provider: 'agy', model: 'gemini-pro' })).toBe('');
  });

  it('groups files by top-level folder with root files last', () => {
    expect(folderGroups(['README.md', 'vendor/a.go', 'cmd/x/main.go', 'vendor/b/c.go'])).toEqual([
      { folder: 'cmd', files: ['cmd/x/main.go'] },
      { folder: 'vendor', files: ['vendor/a.go', 'vendor/b/c.go'] },
      { folder: '', files: ['README.md'] },
    ]);
  });
  it('suggests model names without the catalog prefix', () => {
    const choice = (id: string, source: string) => ({ id, name: id, provider: source, source, location: 'remote', model: { scope: '', model: id } });
    const catalog = { models: [choice('claude:sonnet', 'claude'), choice('claude:opus', 'claude'), choice('agy:gemini-3.8-pro-high', 'agy'), choice('pi:openai-codex/gpt-5.5', 'pi'), choice('configured:' + 'a'.repeat(64), 'configured')], defaults: {}, pi: { status: 'ready' }, agy: { status: 'ready' }, claude: { status: 'ready' } };
    expect(modelSuggestions(catalog, { provider: 'claude', model: '' })).toEqual(['sonnet', 'opus']);
    expect(modelSuggestions(catalog, { provider: 'agy', model: '' })).toEqual(['gemini-3.8-pro-high']);
    expect(modelSuggestions(catalog, { provider: 'pi', model: '' })).toEqual(['openai-codex/gpt-5.5']);
    expect(modelSuggestions(undefined, { provider: 'claude', model: '' })).toEqual([]);
    expect(modelHint({ provider: 'claude', model: '' }, 4)).toMatch(/alias .* or a full model name such as claude-sonnet-5/);
    expect(modelHint({ provider: 'agy', model: '' }, 3)).toBe('3 listed by the CLI');
  });
});
