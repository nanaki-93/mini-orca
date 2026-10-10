import { strict as assert } from 'node:assert';

export async function testGuidedFixes({
  test,
  pageFor,
  nav,
  idle,
  layout,
  contrast,
  chooseModel: selectModel,
}) {
  const openModels = async (page) => {
    await page.getByRole('heading', { name: 'Models for fixes', exact: true }).waitFor();
  };
  const chooseModel = async (trigger, profile) => {
    await openModels(trigger.page());
    await selectModel(trigger, profile);
  };
  await test('Double-clicking the next file never turns review into Apply', async () => {
    const { page, close } = await pageFor({ trusted: true });
    try {
      await nav(page, 'Bugs');
      await page.locator('.result-row').first().click();
      await page.getByRole('button', { name: 'Prepare fix', exact: true }).click();
      await idle(page);
      await page.getByRole('button', { name: 'Review next file', exact: true }).dblclick();
      await idle(page);
      assert.equal(
        await page.evaluate(() =>
          window.fixture.requests.some(
            (r) => r.path.endsWith('/review') || r.path.endsWith('/apply'),
          ),
        ),
        false,
      );
      await page.getByRole('button', { name: 'Apply 2 files', exact: true }).click();
      await idle(page);
      await page.getByRole('heading', { name: 'Change applied', exact: true }).waitFor();
    } finally {
      await close();
    }
  });

  await test('Dedicated fix review advances file by file, resets for regeneration and preserves Apply and Undo', async () => {
    const { page, close } = await pageFor({ trusted: true });
    try {
      await nav(page, 'Bugs');
      await page.locator('.result-row').first().click();
      assert.equal(await page.locator('.result-row').count(), 0);
      assert.equal(
        await page.getByRole('heading', { name: 'Cause', exact: true }).isVisible(),
        false,
      );
      await page.getByRole('button', { name: 'Collapse sidebar' }).click();
      await layout(page, 'dedicated-fix-prepare');
      await page.getByRole('button', { name: 'Prepare fix', exact: true }).click();
      await idle(page);
      const next = page.getByRole('button', { name: 'Review next file', exact: true });
      await next.waitFor();
      assert.equal(
        await page.getByRole('button', { name: 'Apply 2 files', exact: true }).count(),
        0,
      );
      assert.equal(await page.locator('.result-row').count(), 0);
      assert.equal(
        await page
          .getByRole('complementary', { name: 'Application' })
          .getByRole('button', { name: 'Bugs', exact: true })
          .getAttribute('aria-current'),
        'page',
      );
      const writes = await page.evaluate(() =>
        window.fixture.requests.filter((r) => r.method !== 'GET'),
      );
      const tabs = page.getByRole('tablist', { name: 'Fix review', exact: true });
      await tabs.getByRole('tab', { name: 'Changes (2)' }).focus();
      await page.keyboard.press('ArrowRight');
      await page.getByRole('heading', { name: 'Check evidence', exact: true }).waitFor();
      await page.keyboard.press('ArrowRight');
      await page.getByRole('heading', { name: 'Cause', exact: true }).waitFor();
      await page.keyboard.press('Home');
      await next.focus();
      await page.keyboard.press('Enter');
      await page.getByText('2 of 2 files viewed', { exact: true }).waitFor();
      assert.equal(
        await page.getByRole('button', { name: 'Apply 2 files', exact: true }).isEnabled(),
        true,
      );
      assert.deepEqual(
        await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
        writes,
        'Review tabs and next-file action never run checks or write source',
      );
      await tabs.getByRole('tab', { name: 'Details', exact: true }).click();
      await page.locator('summary').getByText('Regenerate fix', { exact: true }).click();
      await page.getByRole('button', { name: 'Regenerate fix', exact: true }).click();
      await idle(page);
      await page.getByText('1 of 2 files viewed', { exact: true }).waitFor();
      await next.waitFor();
      for (const [theme, width, height, large] of [
        ['Graphite', 1440, 900, false],
        ['Porcelain', 1024, 768, false],
        ['Midnight', 800, 640, true],
      ]) {
        await page.setViewportSize({ width, height });
        await page.getByRole('button', { name: `${theme} theme`, exact: true }).click();
        if (large) await page.getByRole('button', { name: 'Larger text', exact: true }).click();
        const review = await page.locator('.fix-review').boundingBox();
        const diff = await page.locator('.fix-review .diff').boundingBox();
        assert.ok(Math.abs(review.width - diff.width) < 2, 'The diff fills the fix workspace');
        await next.scrollIntoViewIfNeeded();
        const action = await next.boundingBox();
        const footer = await page.locator('.statusbar').boundingBox();
        assert.ok(
          action.y >= 0 && action.y + action.height <= footer.y,
          'Primary action stays above the status bar',
        );
        await layout(page, `dedicated-fix-review-${theme.toLowerCase()}`);
        await contrast(page, `Dedicated fix in ${theme}`);
      }
      await next.click();
      const proposal = await page.evaluate(() => window.fixture.state.changes['change-1']);
      await page.getByRole('button', { name: 'Apply 2 files', exact: true }).click();
      await idle(page);
      await page.getByRole('heading', { name: 'Change applied', exact: true }).waitFor();
      const mutations = await page.evaluate(() =>
        window.fixture.requests.filter(
          (r) => r.path.endsWith('/review') || r.path.endsWith('/apply'),
        ),
      );
      assert.equal(mutations.length, 2);
      for (const request of mutations) {
        assert.equal(request.body.hash, proposal.hash);
        assert.equal(request.body.revision, proposal.revision);
      }
      assert.equal(
        await page.evaluate(() => window.fixture.requests.some((r) => r.method === 'PATCH')),
        false,
        'Applying does not mark the finding fixed',
      );
      await page.getByRole('button', { name: 'Verify applied change', exact: true }).click();
      await page
        .getByRole('dialog')
        .getByRole('button', { name: 'Trust this project', exact: true })
        .click();
      await idle(page);
      await page.getByLabel('Post-Apply verification', { exact: true }).waitFor();
      await page.getByRole('button', { name: 'Undo proposal', exact: true }).click();
      await page
        .getByRole('dialog')
        .getByRole('button', { name: 'Undo proposal', exact: true })
        .click();
      await idle(page);
      await page.getByRole('heading', { name: 'Change undone', exact: true }).waitFor();
    } finally {
      await close();
    }
  });

  await test('Finding models share a passive picker above the detail across layouts', async () => {
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
        await openModels(page);
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
          const headingBox = await page.locator('.guided-fix .page-heading h1').boundingBox();
          assert.ok(
            modelBox.y + modelBox.height <= headingBox.y,
            'Shared model settings sit above the finding',
          );
          assert.equal(await models.locator('details').count(), 0);
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
      await openModels(page);
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

  await test('Shared fix models retain choices between categories and disable during generation', async () => {
    const { page, close } = await pageFor({ workflowRunning: true, trusted: true });
    try {
      const before = await page.evaluate(() =>
        window.fixture.requests.filter((request) => request.method !== 'GET'),
      );
      for (const category of ['Bugs', 'Performance', 'Security']) {
        await nav(page, category);
        await page.locator('.result-row').first().click();
        const models = page.locator('.fix-models');
        await models.getByRole('heading', { name: 'Models for fixes', exact: true }).waitFor();
        const creation = models.getByRole('button', { name: 'Creation model', exact: true });
        if (category === 'Bugs') await chooseModel(creation, 'bug');
        assert.equal(await creation.getAttribute('value'), 'bug');
        assert.equal(await creation.isEnabled(), true);
        assert.equal(await page.locator('.fix-models').count(), 1);
      }
      assert.deepEqual(
        await page.evaluate(() =>
          window.fixture.requests.filter((request) => request.method !== 'GET'),
        ),
        before,
      );
      await page.getByRole('button', { name: 'Prepare fix', exact: true }).click();
      await idle(page);
      const creation = page.getByRole('button', { name: 'Creation model', exact: true });
      assert.equal(await creation.getAttribute('value'), 'bug');
      assert.equal(await creation.isDisabled(), true);
      const models = await page.locator('.fix-models').boundingBox();
      const heading = await page.locator('.guided-fix .page-heading h1').boundingBox();
      assert.ok(models.y + models.height <= heading.y);
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
        if (kind === 'fix')
          await page.evaluate(() => {
            window.fixture.state.finding.title = 'File analysis suggestion';
          });
        await nav(page, category);
        await page.locator('.result-row').first().click();
        assert.equal(
          await page.getByRole('heading', { name: 'Cause', exact: true }).isVisible(),
          false,
        );
        await page.locator('summary').getByText('Details', { exact: true }).click();
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
        assert.equal(await page.getByLabel('Change request', { exact: true }).count(), 0);
        assert.equal(
          await page.getByRole('heading', { name: 'Guided fix plan', exact: true }).count(),
          0,
        );
        assert.equal(await page.getByRole('region', { name: 'Model review report' }).count(), 0);
        assert.equal(await page.getByText('Model verdict', { exact: true }).count(), 0);
        const original = preparedWrites.find((r) => r.path.endsWith('/workflow')).body.message;
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
        assert.equal(await page.getByLabel('Change request', { exact: true }).count(), 0);
        assert.equal(
          await page.getByRole('combobox', { name: 'Task type', exact: true }).count(),
          0,
        );
        await page.getByRole('tab', { name: 'Details', exact: true }).click();
        await openModels(page);
        assert.equal(await page.locator('.fix-models .analysis-model-trigger').count(), 3);
        await page.getByRole('tab', { name: 'Changes (2)', exact: true }).click();
        assert.equal(await page.getByRole('checkbox').count(), 0);
        const files = page.getByRole('tablist', { name: 'Files to change', exact: true });
        const paths = await files
          .getByRole('tab')
          .evaluateAll((tabs) => tabs.map((tab) => tab.getAttribute('aria-label')));
        assert.equal(paths.length, 2);
        assert.ok(paths[1].endsWith('_test.go'));
        await files.getByRole('tab', { name: paths[1], exact: true }).click();
        assert.deepEqual(
          await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
          preparedWrites,
        );
        const solution = page.locator('.fix-solution');
        await solution
          .getByText('Return the context error and cover cancellation in a regression test.', {
            exact: true,
          })
          .waitFor();
        assert.equal(await solution.getByText('Cause:', { exact: false }).count(), 0);
        assert.equal(
          await page
            .getByText('The change handles cancellation and its tests pass.', {
              exact: true,
            })
            .count(),
          0,
        );
        if (kind === 'performance')
          await page
            .getByText('Performance unmeasured; tests do not establish a speedup.', { exact: true })
            .waitFor();
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
          await files.getByRole('tab', { name: path, exact: true }).click();
          assert.equal(await page.locator('.chat-review .diff').count(), 1);
          await page.getByLabel(`Read-only diff for ${path}`, { exact: true }).waitFor();
        }
        await files.getByRole('tab', { selected: true }).focus();
        await page.keyboard.press('ArrowLeft');
        for (const width of [1440, 1000, 800]) {
          await page.setViewportSize({ width, height: 1000 });
          await solution.scrollIntoViewIfNeeded();
          await layout(page, `guided-${kind}-${width}`);
          await contrast(page, `guided-${kind}-${width}`);
        }
        await page.getByRole('button', { name: 'Larger text', exact: true }).click();
        await page.getByRole('button', { name: 'Porcelain theme', exact: true }).click();
        await layout(page, `guided-${kind}-800-light-larger`);
        await contrast(page, `guided-${kind}-800-light-larger`);
        assert.deepEqual(
          await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
          requests.filter((r) => r.method !== 'GET'),
        );
        assert.equal(
          requests.some((r) => r.path.endsWith('/apply')),
          false,
        );
        assert.equal(
          await page.getByRole('button', { name: 'Apply 2 files', exact: true }).isEnabled(),
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
        await openModels(page);
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

  await test('Guided solutions omit labeled causes in saved explanations and retain limitations', async () => {
    for (const explanation of [
      'Cause: cancellation was ignored. Solution: Return the context error.\n\nFailure remains when the caller ignores errors.',
      'Cause: cancellation was ignored.\n\nProposed solution: Return the context error.\n\nFailure remains when the caller ignores errors.',
      '## Root cause\n\nCancellation was ignored.\n\n## Proposed solution\n\nReturn the context error.\n\nFailure remains when the caller ignores errors.',
      '**Cause:** cancellation was ignored.\n\n**Solution:** Return the context error.\n\nFailure remains when the caller ignores errors.',
      'Return the context error.\n\nFailure remains when the caller ignores errors.',
    ]) {
      const { page, close } = await pageFor({ changeAssistantMessage: explanation, trusted: true });
      try {
        await nav(page, 'Bugs');
        await page.locator('.result-row').first().click();
        await page.getByRole('button', { name: 'Prepare fix', exact: true }).click();
        await idle(page);
        const solution = page.locator('.fix-solution');
        await solution.getByText('Return the context error.', { exact: true }).first().waitFor();
        await solution.getByRole('button', { name: 'Full explanation', exact: true }).click();
        await solution
          .getByText('Failure remains when the caller ignores errors.', { exact: true })
          .waitFor();
        assert.doesNotMatch(await solution.innerText(), /cause|cancellation was ignored/i);
        await page.getByRole('tab', { name: 'Details', exact: true }).click();
        await page
          .getByText('The worker continues after its request context is canceled.', {
            exact: false,
          })
          .waitFor();
      } finally {
        await close();
      }
    }
  });

  await test('Solution previews expand in place without repeating text or making requests', async () => {
    for (const category of ['Bugs', 'Performance', 'Security']) {
      const opening = 'Keep the original cancellation error.';
      const ending = 'Preserve the caller response and verify the boundary case.';
      const text = `${opening} ${'Check the worker before processing. '.repeat(10)}\n\n${ending}`;
      const { page, close } = await pageFor({ changeAssistantMessage: text, trusted: true });
      try {
        await page.evaluate(
          ({ category, text }) => {
            const state = window.fixture.state;
            if (category === 'Bugs') state.finding.task_spec = { acceptance_criteria: [text] };
            else if (category === 'Performance')
              state.performance.findings[0].recommendation = text;
            else state.security.findings[0].remediation = text;
          },
          { category, text },
        );
        await nav(page, category);
        await page.locator('.result-row').first().click();
        for (const stage of ['finding', 'proposal']) {
          if (stage === 'proposal') {
            await page.getByRole('button', { name: 'Prepare fix', exact: true }).click();
            await idle(page);
          }
          const solution = page.locator('.fix-solution');
          const before = await page.evaluate(() => window.fixture.requests.length);
          const expand = solution.getByRole('button', { name: 'Full explanation', exact: true });
          assert.equal(await expand.getAttribute('aria-expanded'), 'false');
          const content = page.locator(`[id="${await expand.getAttribute('aria-controls')}"]`);
          const preview = await content.innerText();
          assert.ok(preview.startsWith(opening));
          assert.ok(preview.endsWith('…'));
          assert.equal(preview.includes(ending), false);
          await expand.focus();
          await page.keyboard.press('Enter');
          const collapse = solution.getByRole('button', { name: 'Show less', exact: true });
          assert.equal(await collapse.getAttribute('aria-expanded'), 'true');
          assert.equal(await content.innerText(), text);
          assert.equal((await solution.innerText()).split(opening).length - 1, 1);
          assert.equal(await collapse.evaluate((el) => el === document.activeElement), true);
          await page.keyboard.press('Space');
          assert.equal(await content.innerText(), preview);
          assert.equal(await page.evaluate(() => window.fixture.requests.length), before);
        }
      } finally {
        await close();
      }
    }
  });

  await test('Guided fix failures and requested changes appear in the solution and block acceptance', async () => {
    for (const category of ['Bugs', 'Performance', 'Security']) {
      for (const status of ['failed', 'changes_requested']) {
        const summary =
          'Handle the invalid-input case. <script>window.reviewExecuted = true</script>';
        const { page, close } = await pageFor({ workflowStatus: status, workflowSummary: summary });
        try {
          await nav(page, category);
          await page.locator('.result-row').first().click();
          await page.getByRole('button', { name: 'Prepare fix', exact: true }).click();
          await idle(page);
          assert.equal(await page.getByLabel('Change request', { exact: true }).count(), 0);
          assert.equal(await page.getByText('Model verdict', { exact: true }).count(), 0);
          assert.equal(
            await page.getByRole('button', { name: 'Apply 2 files', exact: true }).isDisabled(),
            true,
          );
          const solution = page.locator('.fix-solution');
          if (status === 'failed') {
            await solution
              .getByText(
                'Tests or source checks failed. Inspect the check evidence before running again.',
                { exact: true },
              )
              .waitFor();
            await solution.getByText('tests: failed', { exact: true }).waitFor();
            await page.getByRole('tab', { name: 'Checks (1 need attention)', exact: true }).click();
            await page.getByText('Diagnostics', { exact: true }).click();
            await page.getByText('Boundary test failed.', { exact: true }).waitFor();
          } else {
            await solution
              .getByRole('heading', { name: 'Changes needed before acceptance' })
              .waitFor();
            await solution.getByText(summary, { exact: true }).waitFor();
            await solution
              .getByText('Add a regression test for invalid input.', { exact: true })
              .waitFor();
            assert.equal(
              await page.locator('.workflow-progress').getByText(summary, { exact: true }).count(),
              0,
            );
            assert.equal(await page.evaluate(() => window.reviewExecuted === true), false);
          }
          for (const width of [1440, 800]) {
            await page.setViewportSize({ width, height: 1000 });
            await solution.scrollIntoViewIfNeeded();
            await layout(page, `guided-${category.toLowerCase()}-${status}-${width}`);
            await contrast(page, `guided-${category.toLowerCase()}-${status}-${width}`);
          }
          await page.getByRole('button', { name: 'Larger text', exact: true }).click();
          await page.getByRole('button', { name: 'Porcelain theme', exact: true }).click();
          await solution.scrollIntoViewIfNeeded();
          await layout(page, `guided-${category.toLowerCase()}-${status}-800-light-larger`);
          await contrast(page, `guided-${category.toLowerCase()}-${status}-800-light-larger`);
          assert.equal(
            await page.evaluate(() =>
              window.fixture.requests.some((r) => r.path.endsWith('/apply')),
            ),
            false,
          );
        } finally {
          await close();
        }
      }
    }
  });
}
