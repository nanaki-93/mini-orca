// Which files are tests, decided as the daemon decides it: the patterns come
// from GET /rules (common conventions plus the project's AGENTS.md
// test_files) and match with the daemon's glob semantics
// (internal/project/rules/glob.go). Until they load, the conventions apply.

import type { BudgetFinding, CodeBudgets } from '@/api/types';

/** Mirrors conventionalTestFiles in internal/project/rules/glob.go. */
export const CONVENTIONAL_TEST_FILES = [
  '*_test.go',
  '*_test.py', 'test_*.py', 'conftest.py',
  '*.test.ts', '*.spec.ts', '*.test.tsx', '*.spec.tsx', '*_test.ts',
  '*.test.js', '*.spec.js', '*.test.jsx', '*.spec.jsx', '*_test.js',
  '*.test.mjs', '*.spec.mjs', '*.test.cjs', '*.spec.cjs',
  '*test.kt', '*tests.kt', '*spec.kt', '*test.java', '*tests.java', '*test.scala', '*spec.scala',
  '*_test.rs', '*_test.rb', '*_spec.rb', '*test.php', '*test.cs', '*tests.cs',
  '*test.swift', '*tests.swift', '*_test.c', '*_test.cpp', '*_test.cc', '*_test.dart', '*_test.exs',
  '**/test/**', '**/tests/**', '**/__tests__/**', '**/spec/**', '**/specs/**',
];

/**
 * A case-insensitive matcher for a slash-separated glob: * stays within one
 * path segment, ** spans any number of segments, ? matches one character, and
 * a glob without a slash matches the file name in any directory.
 */
export function globToRegExp(glob: string): RegExp | null {
  let source = glob.trim().replace(/^\.\//, '');
  if (!source || source.startsWith('/') || source.includes('\\')) return null;
  if (!source.includes('/')) source = '**/' + source;
  let pattern = '^';
  for (let i = 0; i < source.length; i++) {
    if (source.startsWith('**/', i)) {
      pattern += '(?:.*/)?';
      i += 2;
    } else if (source.startsWith('**', i)) {
      pattern += '.*';
      i += 1;
    } else if (source[i] === '*') pattern += '[^/]*';
    else if (source[i] === '?') pattern += '[^/]';
    else pattern += source[i].replace(/[.+^${}()|[\]\\]/g, '\\$&');
  }
  return new RegExp(pattern + '$', 'i');
}

function compile(patterns: string[]): RegExp[] {
  return patterns.map(globToRegExp).filter((re): re is RegExp => re !== null);
}

let testMatchers = compile(CONVENTIONAL_TEST_FILES);

/** Adopts the daemon's test file patterns; an empty list restores the conventions. */
export function setTestPatterns(patterns: string[] | undefined): void {
  testMatchers = compile(patterns?.length ? patterns : CONVENTIONAL_TEST_FILES);
}

export function isTestPath(path: string): boolean {
  const clean = path.replace(/^\.\//, '');
  return testMatchers.some(re => re.test(clean));
}

/** Where a new test for a source file conventionally goes, or '' when no single place applies. */
export function companionTestPath(path: string): string {
  if (isTestPath(path)) return '';
  const slash = path.lastIndexOf('/');
  const dir = path.slice(0, slash + 1);
  const name = path.slice(slash + 1);
  const dot = name.lastIndexOf('.');
  if (dot <= 0) return '';
  const stem = name.slice(0, dot);
  const ext = name.slice(dot + 1);
  if (ext === 'go') return `${dir}${stem}_test.go`;
  if (ext === 'py') return `${dir}test_${stem}.py`;
  if (['ts', 'tsx', 'js', 'jsx', 'mjs', 'cjs'].includes(ext)) return `${dir}${stem}.test.${ext}`;
  return '';
}

/** "800 lines per file, complexity 15 per function", or '' with no budget. */
export function describeBudgets(budgets: CodeBudgets): string {
  const parts: string[] = [];
  if (budgets.file_lines) parts.push(`${budgets.file_lines} lines per file`);
  if (budgets.function_lines) parts.push(`${budgets.function_lines} lines per function`);
  if (budgets.function_complexity) parts.push(`complexity ${budgets.function_complexity} per function`);
  return parts.join(', ');
}

/** One budget finding in words, as the complexity gate states it. */
export function describeBudgetFinding(finding: BudgetFinding): string {
  const before = finding.base && finding.base !== finding.value ? `, ${finding.base} before this task` : '';
  if (finding.kind === 'file_lines') return `${finding.value} lines (max ${finding.limit}${before})`;
  const what = finding.kind === 'function_lines' ? `${finding.value} lines` : `complexity ${finding.value}`;
  return `${finding.function ?? 'function'}${finding.line ? ` (line ${finding.line})` : ''}: ${what} (max ${finding.limit}${before})`;
}
