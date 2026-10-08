import { strict as assert } from 'node:assert';

export async function testGuidedFixes({ test, pageFor, nav, idle, layout }) {
  await test('Guided fixes preserve cause, solution and scope with a passive file diff selector', async () => {
    for (const [category, kind] of [
      ['Bugs', 'fix'],
      ['Performance', 'performance'],
      ['Security', 'security'],
    ]) {
      const { page, close } = await pageFor();
      try {
        await nav(page, category);
        await page.locator('.result-row').first().click();
        await page.getByRole('heading', { name: 'Cause', exact: true }).waitFor();
        await page.getByRole('heading', { name: 'Proposed solution', exact: true }).waitFor();
        assert.ok(
          (await page.locator('.results-detail-source').innerText()).includes('process.go'),
        );
        const before = await page.evaluate(() =>
          window.fixture.requests.filter((r) => r.method !== 'GET'),
        );
        await page.getByRole('button', { name: 'Review fix plan', exact: true }).click();
        const request = page.getByLabel('Change request', { exact: true });
        assert.equal(await request.isEditable(), false);
        const original = await request.inputValue();
        assert.match(original, /Location: .*process.go:5/);
        assert.match(original, /Cause:/);
        assert.match(original, /Proposed solution:/);
        await page.getByRole('button', { name: 'Inspect source', exact: true }).click();
        await page.getByLabel('Read-only source', { exact: true }).waitFor();
        assert.equal(
          await page.evaluate(
            () =>
              window.fixture.requests.filter((r) => r.path.endsWith('/files/info')).at(-1).query
                .path,
          ),
          'internal/worker/process.go',
        );
        await nav(page, 'Chat');
        assert.equal(await request.inputValue(), original);
        assert.equal(
          await page.getByRole('combobox', { name: 'Task type', exact: true }).count(),
          0,
        );
        assert.equal(await page.getByRole('combobox', { name: /model/i }).count(), 0);
        assert.equal(await page.getByRole('checkbox').count(), 0);
        const files = page.getByRole('combobox', { name: 'Files to change', exact: true });
        const paths = await files
          .locator('option')
          .evaluateAll((options) => options.map((option) => option.value));
        assert.equal(paths.length, 2);
        assert.ok(paths[1].endsWith('_test.go'));
        await files.selectOption(paths[1]);
        assert.deepEqual(
          await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
          before,
        );
        await page.getByRole('button', { name: 'Run fix', exact: true }).click();
        if (kind === 'security')
          await page
            .getByRole('dialog')
            .getByRole('button', { name: 'Start workflow', exact: true })
            .click();
        await page
          .getByRole('dialog')
          .getByRole('button', { name: 'Trust this project', exact: true })
          .click();
        await idle(page);
        await page.getByRole('heading', { name: 'Proposal diff', exact: true }).waitFor();
        await page
          .getByText(
            'Cause: cancellation was ignored. Solution: return the context error and cover cancellation in a regression test.',
            { exact: true },
          )
          .waitFor();
        if (kind === 'performance')
          await page
            .getByText('Performance unmeasured; tests do not establish a speedup.', { exact: true })
            .waitFor();
        assert.equal(await request.isEditable(), false);
        assert.equal(await request.inputValue(), original);
        const requests = await page.evaluate(() => window.fixture.requests);
        const capture = requests.find(
          (r) => r.method === 'POST' && r.path === '/api/projects/current/changes',
        );
        assert.equal(capture.body.kind, kind);
        assert.deepEqual(capture.body.paths, paths);
        const workflow = requests.find((r) => r.path.endsWith('/workflow'));
        assert.equal(workflow.body.message, original);
        assert.deepEqual(workflow.body.models, {
          create: 'function',
          test: 'bug',
          review: 'analyze',
        });
        assert.equal(workflow.body.confirm_security, kind === 'security');
        for (const path of paths) {
          await files.selectOption(path);
          assert.equal(await page.locator('.chat-review .diff').count(), 1);
          await page.getByLabel(`Read-only diff for ${path}`, { exact: true }).waitFor();
        }
        await files.focus();
        await files.press('ArrowUp');
        for (const width of [1440, 1000, 800]) {
          await page.setViewportSize({ width, height: 1000 });
          await layout(page, `guided-${kind}-${width}`);
        }
        await page.getByRole('button', { name: 'Larger text', exact: true }).click();
        await page.getByRole('button', { name: 'Porcelain theme', exact: true }).click();
        await layout(page, `guided-${kind}-800-light-larger`);
        assert.deepEqual(
          await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
          requests.filter((r) => r.method !== 'GET'),
        );
        assert.equal(
          requests.some((r) => r.path.endsWith('/apply')),
          false,
        );
        assert.equal(
          await page.getByRole('button', { name: 'Accept changes', exact: true }).isEnabled(),
          true,
        );
      } finally {
        await close();
      }
    }
  });

  await test('Missing project instructions link to root creation without dispatch or source writes', async () => {
    const { page, close } = await pageFor({ instructionsMissing: true });
    try {
      await page.getByRole('button', { name: 'Create AGENTS.md', exact: true }).waitFor();
      await nav(page, 'Bugs');
      const before = await page.evaluate(() =>
        window.fixture.requests.filter((r) => r.method !== 'GET'),
      );
      await page.getByRole('button', { name: 'Create AGENTS.md', exact: true }).click();
      await idle(page);
      await page.getByRole('heading', { name: 'Project instructions', exact: true }).waitFor();
      assert.equal(await page.getByLabel('Instruction path').inputValue(), 'AGENTS.md');
      assert.deepEqual(
        await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
        before,
      );
      assert.deepEqual(await page.evaluate(() => window.fixture.state.instructions), {});
    } finally {
      await close();
    }
  });

  await test('Guided fix failures retain readonly intent and cannot be accepted', async () => {
    const { page, close } = await pageFor({ workflowStatus: 'failed' });
    try {
      await nav(page, 'Bugs');
      await page.locator('.result-row').first().click();
      await page.getByRole('button', { name: 'Prepare fix', exact: true }).click();
      await page
        .getByRole('dialog')
        .getByRole('button', { name: 'Trust this project', exact: true })
        .click();
      await idle(page);
      assert.equal(await page.getByLabel('Change request', { exact: true }).isEditable(), false);
      assert.equal(
        await page.getByRole('button', { name: 'Accept changes', exact: true }).isDisabled(),
        true,
      );
      await page.getByText('Diagnostics', { exact: true }).click();
      await page.getByText('Boundary test failed.', { exact: true }).waitFor();
      assert.equal(
        await page.evaluate(() => window.fixture.requests.some((r) => r.path.endsWith('/apply'))),
        false,
      );
    } finally {
      await close();
    }
  });
}
