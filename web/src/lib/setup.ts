// Project setup: the providers a model scope can use, the fields each needs,
// and how analysis exclusions group by folder.

import type { SetupModel, SetupScope, SetupTimeouts } from '@/api/types';

export const SCOPES: { scope: SetupScope; label: string; use: string }[] = [
  { scope: 'analyze', label: 'Analyze', use: 'Project overview, performance and security review, and the specialist reviewers.' },
  { scope: 'bug', label: 'Bug', use: 'Bug analysis of each file, and the agent that writes a change.' },
  { scope: 'code', label: 'Code', use: 'The agent that writes tests, and code generation.' },
];

export interface ProviderInfo {
  id: string;
  label: string;
  hint: string;
  address?: boolean;
  key?: 'required' | 'optional';
  vertex?: boolean;
  cli?: boolean;
  /** Reasoning efforts the provider accepts; none means it has no such setting. */
  efforts?: string[];
}

const EFFORTS = ['', 'none', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max'];

export const PROVIDERS: ProviderInfo[] = [
  { id: 'openai', label: 'OpenAI-compatible API', hint: 'LM Studio, Ollama, vLLM, OpenAI or any /v1/chat/completions endpoint.', address: true, key: 'optional', efforts: EFFORTS },
  { id: 'gemini', label: 'Gemini API', hint: 'Google AI Studio with an API key.', key: 'required', efforts: EFFORTS },
  { id: 'vertex', label: 'Vertex AI', hint: 'Google Cloud Vertex AI: an Express API key, or a project and location.', key: 'optional', vertex: true, efforts: EFFORTS },
  { id: 'agy', label: 'Agy CLI', hint: 'Runs the installed agy command with your signed-in account.', cli: true, efforts: ['', 'low', 'medium', 'high', 'xhigh', 'max'] },
  { id: 'pi', label: 'Pi CLI', hint: 'Runs the installed pi command; models are “provider/model”.', cli: true, efforts: EFFORTS },
  { id: 'claude', label: 'Claude Code', hint: 'Runs the installed claude command on your Claude plan (sign in with /login); never with an API key. Models are aliases such as sonnet or opus.', cli: true, efforts: ['', 'low', 'medium', 'high', 'xhigh', 'max'] },
];

export const DEFAULT_ADDRESS = 'http://localhost:1234/v1';

export function providerOf(model: SetupModel): ProviderInfo {
  return PROVIDERS.find(p => p.id === (model.provider || 'openai')) ?? PROVIDERS[0];
}

/** The model after a provider change: fields the new provider does not use are dropped. */
export function withProvider(model: SetupModel, provider: string): SetupModel {
  const info = PROVIDERS.find(p => p.id === provider) ?? PROVIDERS[0];
  const same = (model.provider || 'openai') === provider;
  return {
    provider: provider === 'openai' ? '' : provider,
    model: same ? model.model : '',
    api_base_url: info.address ? (same && model.api_base_url) || DEFAULT_ADDRESS : undefined,
    api_key_set: same ? model.api_key_set : false,
    vertex_project: info.vertex && same ? model.vertex_project : undefined,
    vertex_location: info.vertex && same ? model.vertex_location : undefined,
    cli_path: info.cli && same ? model.cli_path : undefined,
    reasoning_effort: info.efforts && same ? model.reasoning_effort : undefined,
    context_max_tokens: same ? model.context_max_tokens : undefined,
  };
}

/** What a scope still needs before it can be saved; the daemon checks the rest. */
export function modelProblem(model: SetupModel): string {
  const info = providerOf(model);
  if (!model.model.trim()) return 'Choose a model.';
  if (info.address && !model.api_base_url?.trim()) return 'Enter the API address.';
  if (info.key === 'required' && !model.api_key && !model.api_key_set) return 'Enter an API key.';
  if (info.vertex && !model.api_key && !model.api_key_set && (!model.vertex_project?.trim() || !model.vertex_location?.trim())) return 'Enter an API key, or a project and location.';
  return '';
}

/** The configuration's own limits for agent budgets (config.AgentLimits.Validate). */
export const LIMIT_MAXIMA: Record<string, number> = {
  model_calls: 4096,
  tool_calls: 16384,
  input_tokens: 2 ** 40,
  output_tokens: 2 ** 40,
  elapsed_seconds: 604800,
  concurrent_calls: 32,
  repairs: 2,
};

/** Time limits the app can set, in seconds (config.TimeoutConfig). */
export const TIMEOUTS: { key: keyof SetupTimeouts; label: string; hint: string }[] = [
  { key: 'analysis_seconds', label: 'Analysis request', hint: 'Each request while analyzing a file for bugs, performance and security.' },
  { key: 'import_seconds', label: 'Project overview', hint: 'Reading the project when it opens or is reindexed, and other Analyze model requests.' },
  { key: 'generation_seconds', label: 'Code model request', hint: 'Each request to the Code model.' },
  { key: 'workflow_model_seconds', label: 'Agent model call', hint: 'Each model call agents make on a task, retries included.' },
  { key: 'focused_check_seconds', label: 'Check command', hint: 'Each test or lint command agents run.' },
];

export const MAX_TIMEOUT_SECONDS = 86400;

/** Why each time limit cannot be saved, by key: whole seconds up to a day. */
export function timeoutErrors(timeouts: SetupTimeouts): Partial<Record<keyof SetupTimeouts, string>> {
  const errors: Partial<Record<keyof SetupTimeouts, string>> = {};
  for (const { key } of TIMEOUTS) {
    const value = timeouts[key];
    if (!Number.isInteger(value) || value < 1) errors[key] = 'At least 1 second.';
    else if (value > MAX_TIMEOUT_SECONDS) errors[key] = 'At most one day (86,400 seconds).';
  }
  return errors;
}

export interface FolderGroup {
  folder: string;
  files: string[];
}

/** Files grouped by their top-level folder; root files group under "". */
export function folderGroups(paths: string[]): FolderGroup[] {
  const groups = new Map<string, string[]>();
  for (const path of paths) {
    const folder = path.includes('/') ? path.slice(0, path.indexOf('/')) : '';
    groups.set(folder, [...(groups.get(folder) ?? []), path]);
  }
  return [...groups.entries()].map(([folder, files]) => ({ folder, files })).sort((a, b) => (a.folder === '' ? 1 : b.folder === '' ? -1 : a.folder.localeCompare(b.folder)));
}

/** Folders whose analysis people usually skip: dependencies, builds and fixtures. */
export const SKIPPED_FOLDERS = ['vendor', 'node_modules', 'dist', 'build', 'third_party', 'testdata', 'fixtures', '.github'];
