// Client-only UI state shared across views: overlays and per-viewer
// preferences. Preferences persist in localStorage when it is available.

import { useSyncExternalStore } from 'react';
import type { DraftTask } from './actions';

export type Theme = 'system' | 'dark' | 'light';
export type DiffLayout = 'unified' | 'split';

export interface UIState {
  paletteOpen: boolean;
  paletteMode: 'all' | 'files' | 'commands';
  newTask: Partial<DraftTask> | null;
  shortcutsOpen: boolean;
  sidebarCollapsed: boolean;
  theme: Theme;
  diffLayout: DiffLayout;
  wrapLines: boolean;
}

const PREFS_KEY = 'mini-orca.prefs.v2';

function loadPrefs(): Partial<UIState> {
  try {
    return JSON.parse(localStorage.getItem(PREFS_KEY) ?? '{}') as Partial<UIState>;
  } catch {
    return {};
  }
}

let state: UIState = {
  paletteOpen: false,
  paletteMode: 'all',
  newTask: null,
  shortcutsOpen: false,
  sidebarCollapsed: false,
  theme: 'system',
  diffLayout: 'unified',
  wrapLines: false,
  ...pick(loadPrefs()),
};

function pick(prefs: Partial<UIState>): Partial<UIState> {
  const out: Partial<UIState> = {};
  if (prefs.theme === 'dark' || prefs.theme === 'light' || prefs.theme === 'system') out.theme = prefs.theme;
  if (prefs.diffLayout === 'split' || prefs.diffLayout === 'unified') out.diffLayout = prefs.diffLayout;
  if (typeof prefs.wrapLines === 'boolean') out.wrapLines = prefs.wrapLines;
  if (typeof prefs.sidebarCollapsed === 'boolean') out.sidebarCollapsed = prefs.sidebarCollapsed;
  return out;
}

const listeners = new Set<() => void>();

export function getUI(): UIState {
  return state;
}

export function setUI(patch: Partial<UIState>): void {
  state = { ...state, ...patch };
  try {
    localStorage.setItem(PREFS_KEY, JSON.stringify({ theme: state.theme, diffLayout: state.diffLayout, wrapLines: state.wrapLines, sidebarCollapsed: state.sidebarCollapsed }));
  } catch {
    // Preferences are a convenience; the UI works without storage.
  }
  listeners.forEach(fn => fn());
}

export function useUI<T>(select: (state: UIState) => T): T {
  return useSyncExternalStore(
    fn => {
      listeners.add(fn);
      return () => listeners.delete(fn);
    },
    () => select(state),
  );
}

export function openNewTask(prefill: Partial<DraftTask> = {}): void {
  setUI({ newTask: prefill, paletteOpen: false });
}

export function openPalette(mode: UIState['paletteMode'] = 'all'): void {
  setUI({ paletteOpen: true, paletteMode: mode });
}
