// URL <-> view state. Deep links from the TUI (`:web`) use tab, session_id,
// file, worktree_path and section; older tab names keep working.

import { REVIEW_SECTIONS } from './readiness.js';

export const PAGES = {
  home: 'Home',
  files: 'Explorer',
  tasks: 'Tasks',
  'new-task': 'New task',
  diff: 'Review',
  issues: 'Issues',
  docs: 'Docs',
  checks: 'Checks',
  git: 'Git',
  settings: 'Settings'
};

export const DOC_SECTIONS = ['overview', 'architecture', 'components', 'scenarios', 'decisions', 'dependencies', 'runbook', 'glossary'];

const ALIASES = { dashboard: 'home', overview: 'home', review: 'diff', changes: 'diff', understand: 'docs', build: 'new-task', explorer: 'files', branches: 'git', worktrees: 'git' };

export function normalizePage(page) {
  const value = String(page || '');
  if (Object.prototype.hasOwnProperty.call(ALIASES, value)) return ALIASES[value];
  return Object.prototype.hasOwnProperty.call(PAGES, value) ? value : 'home';
}

export function parseRoute(search) {
  const params = new URLSearchParams(search || '');
  const sessionId = params.get('session_id') || '';
  const file = params.get('file') || '';
  const rawTab = params.get('tab') || '';
  const tab = normalizePage(rawTab || (sessionId ? 'diff' : file ? 'files' : 'home'));
  const section = params.get('section') || '';
  return {
    tab,
    sessionId,
    file,
    worktreePath: params.get('worktree_path') || '',
    docSection: DOC_SECTIONS.includes(section) ? section : 'overview',
    reviewSection: REVIEW_SECTIONS.includes(section) ? section : 'changes'
  };
}

export function buildSearch(view) {
  const q = new URLSearchParams();
  q.set('tab', view.tab);
  if (view.tab === 'docs' && view.docSection && view.docSection !== 'overview') q.set('section', view.docSection);
  if (view.tab === 'diff' && view.reviewSection && view.reviewSection !== 'changes') q.set('section', view.reviewSection);
  if (view.worktreePath) q.set('worktree_path', view.worktreePath);
  if (view.sessionId) q.set('session_id', view.sessionId);
  if (view.file) q.set('file', view.file);
  return '?' + q.toString();
}
