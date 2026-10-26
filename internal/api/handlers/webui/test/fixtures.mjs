// Shared proposal fixture: a fully reviewed, checked and approved session.
export function readySession(overrides = {}) {
  return { id: 'change-1', project_id: 'project-1', project_revision: 'revision-1', revision: 2,
    hash: 'hash-1', state: 'reviewed', freshness: 'current', changes: [{ path: 'main.go' }],
    reviewed_files: ['main.go'], file_review_hash: 'hash-1', reviewed_hash: 'hash-1',
    checks: [{ name: 'tests', required: true, state: 'passed' }], ...overrides };
}
