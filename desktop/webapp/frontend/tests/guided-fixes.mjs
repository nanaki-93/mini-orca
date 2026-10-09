import { strict as assert } from 'node:assert';

export async function testGuidedFixes({ test, pageFor, nav, idle, layout, chooseModel }) {
  await test('Finding agent cards share the Analysis picker above the detail and stay passive across layouts', async () => {
    for (const category of ['Bugs', 'Performance', 'Security']) {
      const { page, close } = await pageFor({
        modelNames: { function: 'gpt-4.1', bug: 'gemini-2.5-pro', analyze: 'claude-sonnet-4' },
        trusted: true,
      });
      try {
        await nav(page, 'Analysis');
        const treatment = (card) =>
          card.evaluate((element) => {
            const style = getComputedStyle(element);
            return [style.padding, style.borderRadius, style.backgroundColor];
          });
        const analysisStyle = await treatment(page.locator('.analysis-model-card').first());
        await nav(page, category);
        await page.locator('.result-row').first().click();
        const models = page.locator('.fix-models');
        const cards = models.locator('.analysis-model-card');
        const before = await page.evaluate(() =>
          window.fixture.requests.filter((r) => r.method !== 'GET'),
        );
        assert.deepEqual(
          await cards.evaluateAll((elements) =>
            elements.map((element) => element.dataset.agentType),
          ),
          ['create', 'test', 'review'],
        );
        assert.deepEqual(await treatment(cards.first()), analysisStyle);
        assert.deepEqual(
          await cards
            .locator('.analysis-model-icon')
            .evaluateAll((elements) => elements.map((element) => element.dataset.family)),
          ['openai', 'gemini', 'claude'],
        );
        const creation = models.getByRole('button', { name: 'Creation model', exact: true });
        await creation.focus();
        await creation.press('Enter');
        const dialog = page.getByRole('dialog', { name: 'Choose a model', exact: true });
        const search = dialog.getByRole('combobox', { name: 'Search models', exact: true });
        assert.equal(await search.evaluate((element) => element === document.activeElement), true);
        assert.equal(await dialog.getByRole('button', { name: 'Pi', exact: true }).count(), 0);
        await search.fill('no matching model');
        await dialog.getByText('No models match these filters.', { exact: true }).waitFor();
        await search.fill('gemini');
        await search.press('Enter');
        assert.equal(await creation.getAttribute('value'), 'bug');
        assert.equal(
          await creation.evaluate((element) => element === document.activeElement),
          true,
        );
        await models.getByRole('button', { name: 'Review model', exact: true }).click();
        await page.keyboard.press('Escape');
        assert.equal(
          await models
            .getByRole('button', { name: 'Review model', exact: true })
            .getAttribute('value'),
          'analyze',
        );
        for (const width of [1440, 1000, 800]) {
          await page.setViewportSize({ width, height: 1000 });
          const modelBox = await models.boundingBox();
          const detailBox = await page.locator('.results-detail-layout').boundingBox();
          assert.ok(
            modelBox.y + modelBox.height <= detailBox.y,
            'Agent models precede the finding detail',
          );
          await page.locator('#main').evaluate((element) => {
            element.scrollTop = 0;
          });
          await layout(page, `finding-agents-${category.toLowerCase()}-${width}`);
        }
        await page.getByRole('button', { name: 'Larger text', exact: true }).click();
        await page.getByRole('button', { name: 'Porcelain theme', exact: true }).click();
        await layout(page, `finding-agents-${category.toLowerCase()}-800-light-larger`);
        assert.deepEqual(
          await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
          before,
        );
      } finally {
        await close();
      }
    }
  });

  await test('Unavailable workflow models recover through the shared picker without generating a fix', async () => {
    const { page, close } = await pageFor({ emptyModelCatalog: true, trusted: true });
    try {
      await nav(page, 'Bugs');
      await page.locator('.result-row').first().click();
      assert.equal(
        await page.getByRole('button', { name: 'Prepare fix', exact: true }).isDisabled(),
        true,
      );
      await page.getByRole('button', { name: 'Creation model', exact: true }).click();
      const dialog = page.getByRole('dialog', { name: 'Choose a model', exact: true });
      await dialog
        .getByText('No models available. Refresh the catalog to try again.', { exact: true })
        .waitFor();
      await page.evaluate(() => {
        window.fixture.options.emptyModelCatalog = false;
      });
      await dialog.getByRole('button', { name: 'Refresh models', exact: true }).click();
      await dialog.getByRole('option').first().waitFor();
      await page.keyboard.press('Escape');
      await page.getByRole('button', { name: 'Prepare fix', exact: true }).click({ trial: true });
      assert.equal(
        await page.evaluate(() =>
          window.fixture.requests.some(
            (r) => r.path.endsWith('/workflow') || r.path.endsWith('/messages'),
          ),
        ),
        false,
      );
    } finally {
      await close();
    }
  });

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
        assert.ok((await page.locator('.results-detail-meta').innerText()).includes('process.go'));
        const before = await page.evaluate(() =>
          window.fixture.requests.filter((r) => r.method !== 'GET'),
        );
        assert.equal(
          await page.getByRole('button', { name: 'Review fix plan', exact: true }).count(),
          0,
        );
        assert.equal(await page.getByRole('heading', { name: 'Finding', exact: true }).count(), 0);
        assert.equal(
          await page.getByRole('heading', { name: 'File analysis', exact: true }).count(),
          0,
        );
        await chooseModel(page.getByLabel('Creation model', { exact: true }), 'bug');
        await chooseModel(page.getByLabel('Testing model', { exact: true }), 'function');
        await chooseModel(page.getByLabel('Review model', { exact: true }), 'analyze');
        assert.equal(
          await page.getByRole('button', { name: 'Prepare fix', exact: true }).isEnabled(),
          true,
        );
        assert.deepEqual(
          await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
          before,
        );
        assert.equal(await page.locator('.fix-preparation input[type=checkbox]').count(), 0);
        await page.getByRole('button', { name: 'Prepare fix', exact: true }).click();
        await idle(page);
        await page.getByRole('heading', { name: 'Proposal diff', exact: true }).waitFor();
        assert.equal(await page.getByRole('dialog').count(), 0);
        const preparedWrites = await page.evaluate(() =>
          window.fixture.requests.filter((r) => r.method !== 'GET'),
        );
        const request = page.getByLabel('Change request', { exact: true });
        assert.equal(await request.isEditable(), false);
        const original = await request.inputValue();
        assert.match(original, /Location: .*process.go:5/);
        assert.match(original, /Cause:/);
        assert.match(original, /Proposed solution:/);
        await page.getByRole('button', { name: 'Go to file', exact: true }).click();
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
        assert.equal(await page.locator('.fix-models .analysis-model-trigger').count(), 3);
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
          preparedWrites,
        );
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
          create: 'bug',
          test: 'function',
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

  await test('Guided fix shows scoped permissions and starts once without a confirmation popup', async () => {
    for (const category of ['Bugs', 'Performance', 'Security']) {
      const { page, close } = await pageFor({ remote: true });
      try {
        await nav(page, category);
        await page.locator('.result-row').first().click();
        await page.getByLabel('Creation model', { exact: true }).waitFor();
        const preparation = page.locator('.fix-preparation');
        assert.equal(await preparation.getByRole('checkbox').count(), 0);
        await preparation.getByText('Remote destinations', { exact: true }).waitFor();
        assert.equal(
          await preparation
            .getByText('test-model · https://provider.invalid', { exact: true })
            .count(),
          1,
        );
        if (category === 'Security')
          await preparation
            .getByText('Selecting Prepare fix authorizes a Security fix', {
              exact: false,
            })
            .waitFor();
        const before = await page.evaluate(() =>
          window.fixture.requests.filter((r) => r.method !== 'GET'),
        );
        await preparation.getByText('Project checks', { exact: true }).click();
        await preparation.getByText('go test ./...', { exact: true }).waitFor();
        await chooseModel(page.getByLabel('Creation model', { exact: true }), 'bug');
        assert.deepEqual(
          await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
          before,
        );
        const prepare = page.getByRole('button', { name: 'Prepare fix', exact: true });
        await prepare.focus();
        await prepare.press('Enter');
        await idle(page);
        assert.equal(await page.getByRole('dialog').count(), 0);
        const workflows = await page.evaluate(() =>
          window.fixture.requests.filter((r) => r.path.endsWith('/workflow')),
        );
        assert.equal(workflows.length, 1);
        const [workflow] = workflows;
        assert.deepEqual(workflow.body.models, { create: 'bug', test: 'bug', review: 'analyze' });
        assert.deepEqual(workflow.body.confirmed_profiles.sort(), ['analyze', 'bug']);
        assert.equal(workflow.body.confirm_security, category === 'Security');
        assert.equal(
          await page.evaluate(
            () =>
              window.fixture.requests.filter(
                (r) => r.method === 'POST' && r.path.endsWith('/execution-trust'),
              ).length,
          ),
          1,
        );
      } finally {
        await close();
      }
    }
  });

  await test('Guided fix starts immediately and remains cancelable without applying source', async () => {
    const { page, close } = await pageFor({ remote: true, workflowRunning: true });
    try {
      await nav(page, 'Bugs');
      await page.locator('.result-row').first().click();
      await page.getByRole('button', { name: 'Prepare fix', exact: true }).click();
      await idle(page);
      assert.equal(await page.getByRole('dialog').count(), 0);
      await page.getByRole('button', { name: 'Cancel workflow', exact: true }).click();
      await idle(page);
      await page.getByText('Workflow canceled. No source was applied.', { exact: true }).waitFor();
      assert.equal(
        await page.evaluate(() => window.fixture.requests.some((r) => r.path.endsWith('/apply'))),
        false,
      );
    } finally {
      await close();
    }
  });

  await test('Markdown fixes use the selected creation model without a workflow or popup', async () => {
    const { page, close } = await pageFor({ sourcePaths: ['README.md'], remote: true });
    try {
      await nav(page, 'Bugs');
      await page.locator('.result-row').first().click();
      await chooseModel(page.getByLabel('Creation model', { exact: true }), 'bug');
      assert.equal(await page.getByLabel('Testing model', { exact: true }).count(), 0);
      await page.getByRole('button', { name: 'Prepare fix', exact: true }).click();
      await idle(page);
      assert.equal(await page.getByRole('dialog').count(), 0);
      const requests = await page.evaluate(() => window.fixture.requests);
      assert.equal(
        requests.some((r) => r.path.endsWith('/workflow')),
        false,
      );
      assert.equal(requests.find((r) => r.path.endsWith('/messages')).body.profile, 'bug');
      assert.equal(
        requests.find((r) => r.path.endsWith('/messages')).body.confirm_remote_provider,
        true,
      );
    } finally {
      await close();
    }
  });

  await test('Guided fix rejects changed destinations and unavailable preparation before dispatch', async () => {
    for (const scenario of ['destination', 'commands', 'revision', 'unavailable']) {
      const { page, close } = await pageFor({ remote: true });
      try {
        await nav(page, 'Bugs');
        await page.locator('.result-row').first().click();
        await page.getByRole('button', { name: 'Prepare fix', exact: true }).click({ trial: true });
        await page.evaluate((scenario) => {
          if (scenario === 'destination')
            window.fixture.options.modelNames = { function: 'replacement-model' };
          else if (scenario === 'commands')
            window.fixture.options.executionCommands = [['go', 'test', '-race', './...']];
          else if (scenario === 'revision')
            window.fixture.state.project.project_revision = 'replacement-revision';
          else window.fixture.failures['/api/projects/current/execution-trust'] = 503;
        }, scenario);
        await page.getByRole('button', { name: 'Prepare fix', exact: true }).click();
        await idle(page);
        assert.equal(await page.getByRole('dialog').count(), 0);
        assert.equal(
          await page.evaluate(() =>
            window.fixture.requests.some(
              (r) => r.path.endsWith('/workflow') || r.path.endsWith('/messages'),
            ),
          ),
          false,
        );
        assert.equal(
          await page.evaluate(() =>
            window.fixture.requests.some(
              (r) => r.method === 'POST' && r.path.endsWith('/execution-trust'),
            ),
          ),
          false,
        );
      } finally {
        await close();
      }
    }
  });

  await test('Missing project instructions link to root creation without dispatch or source writes', async () => {
    const { page, close } = await pageFor({ instructionsMissing: true });
    try {
      for (const name of ['Summary', 'Analysis', 'Bugs', 'Performance', 'Security']) {
        await nav(page, name);
        assert.equal(await page.getByLabel('Default agent instructions').count(), 0);
      }
      await nav(page, 'Chat');
      await page.getByRole('button', { name: 'Create AGENTS.md', exact: true }).waitFor();
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
