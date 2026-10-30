import { afterEach, describe, expect, it } from 'vitest';
import { companionTestPath, describeBudgetFinding, describeBudgets, globToRegExp, isTestPath, setTestPatterns } from './rules';

describe('test file patterns', () => {
  afterEach(() => setTestPatterns(undefined));

  it('match globs as the daemon does', () => {
    const cases: [string, string, boolean][] = [
      ['*_test.go', 'a/b/c_test.go', true],
      ['*_test.go', 'c_test.go', true],
      ['src/*.ts', 'src/a.ts', true],
      ['src/*.ts', 'src/x/a.ts', false],
      ['src/**/*.ts', 'src/a.ts', true],
      ['src/**/*.ts', 'src/x/y/a.ts', true],
      ['**/test/**', 'test/a.go', true],
      ['a?.py', 'ab.py', true],
      ['*Test.java', 'FooTEST.java', true],
    ];
    for (const [glob, file, want] of cases) expect(globToRegExp(glob)?.test(file), `${glob} vs ${file}`).toBe(want);
    expect(globToRegExp('/abs/*.go')).toBeNull();
  });

  it('follow the conventions until the daemon sends the project patterns', () => {
    expect(isTestPath('pkg/tests/helpers.py')).toBe(true);
    expect(isTestPath('src/test/java/a/FooTest.java')).toBe(true);
    expect(isTestPath('svc/orders.it.ts')).toBe(false);
    setTestPatterns(['*_test.go', 'svc/**/*.it.ts']);
    expect(isTestPath('svc/deep/orders.it.ts')).toBe(true);
    expect(isTestPath('pkg/tests/helpers.py')).toBe(false);
    setTestPatterns([]);
    expect(isTestPath('pkg/tests/helpers.py')).toBe(true);
  });

  it('name a companion test where one place is conventional', () => {
    expect(companionTestPath('a/b.go')).toBe('a/b_test.go');
    expect(companionTestPath('src/view.tsx')).toBe('src/view.test.tsx');
    expect(companionTestPath('app/models.py')).toBe('app/test_models.py');
    expect(companionTestPath('src/Foo.java')).toBe('');
    expect(companionTestPath('a/b_test.go')).toBe('');
  });
});

describe('budget wording', () => {
  it('reads like the complexity gate', () => {
    expect(describeBudgets({ file_lines: 800, function_complexity: 15 })).toBe('800 lines per file, complexity 15 per function');
    expect(describeBudgets({})).toBe('');
    expect(describeBudgetFinding({ kind: 'file_lines', value: 845, base: 790, limit: 800 })).toBe('845 lines (max 800, 790 before this task)');
    expect(describeBudgetFinding({ kind: 'function_complexity', function: 'Store.add', line: 12, value: 17, limit: 15 })).toBe('Store.add (line 12): complexity 17 (max 15)');
  });
});
