import type { TerminalUpdate } from './models';

export interface NativeBridge {
  Request(
    id: string,
    method: string,
    path: string,
    body: string,
  ): Promise<{ status: number; body: string }>;
  Cancel(id: string): Promise<void>;
  ChooseDirectory(): Promise<string>;
  SetUnsavedDraft(dirty: boolean): Promise<void>;
  OpenTerminal(root: string, cols: number, rows: number): Promise<TerminalUpdate>;
  ReadTerminal(id: string, cursor: number): Promise<TerminalUpdate>;
  WriteTerminal(id: string, data: string): Promise<void>;
  ResizeTerminal(id: string, cols: number, rows: number): Promise<void>;
  CloseTerminal(id: string): Promise<void>;
}
declare global {
  interface Window {
    go?: { main: { Desktop: NativeBridge } };
  }
}
export function native(): NativeBridge {
  const bridge = window.go?.main.Desktop;
  if (!bridge)
    throw new Error(
      'Open Mini-Orca from the desktop application. This browser preview has no native connection.',
    );
  return bridge;
}
export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
    readonly detail: string,
  ) {
    super(message);
  }
}
export function query(values: object): string {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(values))
    if (value !== undefined && value !== '') params.set(key, String(value));
  return `?${params}`;
}
export class API {
  private pending = new Map<string, { bridge: NativeBridge; reject: () => void }>();
  async request<T>(method: string, path: string, body?: unknown): Promise<T> {
    const id = crypto.randomUUID();
    const bridge = native();
    const canceled = new Promise<never>((_, reject) =>
      this.pending.set(id, {
        bridge,
        reject: () => reject(new DOMException('Canceled', 'AbortError')),
      }),
    );
    try {
      const response = await Promise.race([
        bridge.Request(id, method, path, body === undefined ? '' : JSON.stringify(body)),
        canceled,
      ]);
      if (response.status === 204) return null as T;
      let data;
      try {
        data = JSON.parse(response.body);
      } catch {
        throw new Error('The daemon returned an unreadable response.');
      }
      if (response.status < 200 || response.status >= 300)
        throw new ApiError(
          response.status,
          data.user_message || data.message || data.error || `Request failed (${response.status})`,
          data.message || '',
        );
      return data as T;
    } finally {
      this.pending.delete(id);
    }
  }
  async cancelAll() {
    const pending = [...this.pending.entries()];
    for (const [, value] of pending) value.reject();
    const results = await Promise.allSettled(pending.map(([id, value]) => value.bridge.Cancel(id)));
    const failure = results.find((result) => result.status === 'rejected');
    if (failure?.status === 'rejected') throw failure.reason;
  }
  get<T>(path: string, params?: object) {
    return this.request<T>('GET', path + (params ? query(params) : ''));
  }
  post<T>(path: string, data: unknown) {
    return this.request<T>('POST', path, data);
  }
}
export const current = '/api/projects/current';
export function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}
