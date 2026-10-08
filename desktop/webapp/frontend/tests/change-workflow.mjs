import { strict as assert } from 'node:assert';

export async function testChangeWorkflows({ test, pageFor, nav, idle, layout }) {
  async function prepare(page) {
    await nav(page, 'Chat');
    await page
      .getByLabel('Files to change', { exact: true })
      .fill('internal/worker.go\ninternal/worker_test.go');
    await page
      .getByLabel('Change request', { exact: true })
      .fill('Handle cancellation and cover its boundary cases.');
  }
  async function trust(page) {
    await page
      .getByRole('dialog')
      .getByRole('button', { name: 'Trust this project', exact: true })
      .click();
    await idle(page);
  }

  await test('Agent workflow captures models, shows progress, and requires human diff approval', async () => {
    const { page, close } = await pageFor({ remote: true, workflowRunning: true });
    try {
      await prepare(page);
      await page.getByText('Agent models', { exact: true }).click();
      await page.getByLabel('Creation model', { exact: true }).selectOption('bug');
      await page.getByLabel('Testing model', { exact: true }).selectOption('function');
      await page.getByLabel('Review model', { exact: true }).selectOption('analyze');
      await page.getByRole('button', { name: 'Generate changes', exact: true }).click();
      const consent = page.getByRole('dialog');
      await consent.getByText('internal/worker_test.go', { exact: true }).waitFor();
      await consent.getByRole('button', { name: 'Start workflow', exact: true }).click();
      await trust(page);
      await page.getByRole('heading', { name: 'Agent workflow', exact: true }).waitFor();
      const request = await page.evaluate(() =>
        window.fixture.requests.find((r) => r.path.endsWith('/workflow')),
      );
      assert.deepEqual(request.body.models, { create: 'bug', test: 'function', review: 'analyze' });
      assert.deepEqual(request.body.confirmed_profiles.sort(), ['analyze', 'bug', 'function']);
      assert.equal(
        await page.getByRole('button', { name: 'New conversation', exact: true }).isDisabled(),
        true,
      );
      assert.equal(await page.getByLabel('Creation model', { exact: true }).isDisabled(), true);
      await nav(page, 'Summary');
      await page.getByRole('button', { name: 'View workflow', exact: true }).click();
      await page.evaluate(() => window.fixture.completeWorkflow());
      await page
        .getByText('Review the file differences, then select Accept changes.', {
          exact: true,
        })
        .waitFor();
      assert.equal(
        await page.getByRole('button', { name: 'Accept changes', exact: true }).isDisabled(),
        false,
      );
      assert.equal(
        await page.evaluate(() => window.fixture.requests.some((r) => r.path.endsWith('/apply'))),
        false,
      );
      await layout(page, 'agent-workflow-human-review');
      await page.getByRole('button', { name: 'Accept changes', exact: true }).click();
      await idle(page);
      assert.equal(
        await page.evaluate(
          () => window.fixture.requests.filter((r) => r.path.endsWith('/apply')).length,
        ),
        1,
      );
    } finally {
      await close();
    }
  });

  await test('Workflow entry points seed scope and task intent without dispatch', async () => {
    for (const [pageName, kind] of [
      ['Features', 'feature'],
      ['Bugs', 'fix'],
      ['Performance', 'performance'],
      ['Security', 'security'],
    ]) {
      const { page, close } = await pageFor({ featuresReady: true });
      try {
        await nav(page, pageName);
        if (pageName !== 'Features') await page.locator('.result-row').first().click();
        await page
          .getByRole('button', {
            name: kind === 'feature' ? 'Configure workflow' : 'Review fix plan',
            exact: true,
          })
          .first()
          .click();
        if (kind === 'feature')
          assert.equal(await page.getByLabel('Task type', { exact: true }).inputValue(), kind);
        else {
          assert.equal(
            await page.getByRole('combobox', { name: 'Task type', exact: true }).count(),
            0,
          );
          assert.equal(
            await page.getByLabel('Change request', { exact: true }).getAttribute('readonly'),
            '',
          );
        }
        assert.match(
          kind === 'feature'
            ? await page.getByLabel('Files to change', { exact: true }).inputValue()
            : await page.getByLabel('Files to change', { exact: true }).textContent(),
          /_test\.go/,
        );
        assert.equal(
          await page.evaluate(() =>
            window.fixture.requests.some(
              (r) => r.path.endsWith('/workflow') || r.path.endsWith('/messages'),
            ),
          ),
          false,
        );
        if (kind === 'security') {
          await page.getByRole('button', { name: 'Run fix', exact: true }).click();
          await page
            .getByRole('dialog')
            .getByRole('heading', { name: 'Start security workflow?', exact: true })
            .waitFor();
          await page
            .getByRole('dialog')
            .getByRole('button', { name: 'Cancel', exact: true })
            .click();
          await idle(page);
          assert.equal(
            await page.evaluate(() =>
              window.fixture.requests.some((r) => r.path.endsWith('/workflow')),
            ),
            false,
          );
        }
      } finally {
        await close();
      }
    }
  });

  await test('Failed tests and requested changes retain evidence and block human approval across layouts', async () => {
    for (const status of ['failed', 'changes_requested']) {
      const { page, close } = await pageFor({
        workflowStatus: status,
        workflowSummary: `Review ${'Long explanatory text. '.repeat(60)}<script>window.workflowExecuted = true</script>`,
      });
      try {
        await prepare(page);
        await page.getByRole('button', { name: 'Generate changes', exact: true }).click();
        await trust(page);
        assert.equal(
          await page.getByRole('button', { name: 'Accept changes', exact: true }).isDisabled(),
          true,
        );
        assert.equal(await page.evaluate(() => window.workflowExecuted === true), false);
        await page.setViewportSize({ width: 1000, height: 850 });
        await page.getByRole('button', { name: 'Larger text', exact: true }).click();
        await layout(page, `agent-workflow-${status}-larger`);
      } finally {
        await close();
      }
    }
  });

  await test('Cancel workflow sends the captured run identity and never applies source', async () => {
    const { page, close } = await pageFor({ workflowRunning: true });
    try {
      await prepare(page);
      await page.getByRole('button', { name: 'Generate changes', exact: true }).click();
      await trust(page);
      await page.getByRole('button', { name: 'Cancel workflow', exact: true }).click();
      await idle(page);
      await page.getByText('Workflow canceled. No source was applied.', { exact: true }).waitFor();
      const requests = await page.evaluate(() => window.fixture.requests);
      assert.equal(
        requests.find((r) => r.path.endsWith('/workflow/cancel')).body.workflow_id,
        'workflow-1',
      );
      assert.equal(
        requests.some((r) => r.path.endsWith('/apply')),
        false,
      );
      await page
        .getByLabel('Change request', { exact: true })
        .fill('Retry the change with the same scope.');
      assert.equal(
        await page.getByRole('button', { name: 'Generate changes', exact: true }).isEnabled(),
        true,
      );
      await layout(page, 'agent-workflow-canceled');
    } finally {
      await close();
    }
  });
  await test('Acceptance rejects changed or failed review and cannot apply after navigation', async () => {
    for (const scenario of ['changed-revision', 'review-failed', 'navigation']) {
      const { page, close } = await pageFor();
      try {
        await nav(page, 'Chat');
        await page
          .getByLabel('Files to change', { exact: true })
          .fill('internal/worker/process.go');
        await page.getByLabel('Change request', { exact: true }).fill('Handle cancellation.');
        await page.getByLabel('Run project tests after generation').uncheck();
        await page.getByRole('button', { name: 'Generate changes', exact: true }).click();
        await idle(page);
        await page.evaluate((scenario) => {
          window.fixture.hold = '/api/projects/current/changes/change-1/review';
          if (scenario === 'review-failed') window.fixture.failures[window.fixture.hold] = 409;
        }, scenario);
        await page.getByRole('button', { name: 'Accept changes', exact: true }).click();
        await page.waitForFunction(() =>
          window.fixture.requests.some((r) => r.path.endsWith('/review')),
        );
        assert.equal(
          await page.getByRole('button', { name: 'Accept changes', exact: true }).isDisabled(),
          true,
        );
        if (scenario === 'navigation') {
          await nav(page, 'Summary');
          await nav(page, 'Chat');
        }
        await page.evaluate((scenario) => {
          if (scenario === 'changed-revision') {
            window.fixture.state.changes['change-1'].hash = 'replacement';
            window.fixture.state.changes['change-1'].revision++;
          }
          window.fixture.hold = '';
          window.fixture.release();
        }, scenario);
        await idle(page);
        assert.equal(
          await page.evaluate(() => window.fixture.requests.some((r) => r.path.endsWith('/apply'))),
          false,
          scenario,
        );
        if (scenario !== 'navigation') assert.equal(await page.getByRole('alert').count(), 1);
      } finally {
        await page.evaluate(() => {
          window.fixture.hold = '';
          window.fixture.release();
        });
        await close();
      }
    }
  });
}
