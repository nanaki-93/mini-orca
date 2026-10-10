import { strict as assert } from 'node:assert';

export async function testResults({ test, pageFor, nav, idle, layout }) {
  await test('Source security controls keep explicit review consent and open the shared list', async () => {
    for (const ai of [false, true]) {
      const { page, close } = await pageFor({ remote: ai });
      try {
        await nav(page, 'Source');
        await page.locator('.file-item[title="internal/worker/process.go"]').click();
        await page.getByText('Inspect this file', { exact: true }).click();
        const endpoint = ai ? '/security-review' : '/files/security-scan';
        const action = page.getByRole('button', {
          name: ai ? 'AI Security review' : 'Scan source',
          exact: true,
        });
        await action.click();
        if (ai) {
          const dialog = page.getByRole('dialog', { name: 'Security review', exact: true });
          await dialog.waitFor();
          assert.equal(
            await page.evaluate(
              (endpoint) => window.fixture.requests.some((r) => r.path.endsWith(endpoint)),
              endpoint,
            ),
            false,
          );
          await dialog.getByRole('button', { name: 'Continue', exact: true }).click();
        }
        await idle(page);
        await page.getByRole('heading', { name: 'Security', exact: true }).waitFor();
        assert.equal(await page.locator('.results-source, .results-reports').count(), 0);
        assert.equal(
          await page.getByRole('button', { name: 'Scan source', exact: true }).count(),
          0,
        );
        const requests = await page.evaluate(
          (endpoint) => window.fixture.requests.filter((r) => r.path.endsWith(endpoint)),
          endpoint,
        );
        assert.equal(requests.length, 1);
        assert.equal(requests[0].body.path, 'internal/worker/process.go');
        if (ai) assert.equal(requests[0].body.confirm_remote_provider, true);
        await page.locator('.result-row').click();
        await page.getByRole('heading', { name: 'Unbounded input path', exact: true }).waitFor();
      } finally {
        await close();
      }
    }
  });
  await test('Finding priorities and freshness use named compact indicators', async () => {
    const { page, close } = await pageFor();
    try {
      await page.evaluate(() => {
        const state = window.fixture.state;
        state.results.bugs = {
          semantic: ['critical', 'high', 'medium', 'low', 'info'].map((severity, index) => ({
            ...state.finding,
            id: `priority-${index}`,
            severity,
            freshness: ['fresh', 'stale', 'error', 'fresh', 'fresh'][index],
          })),
          performance: [],
          security: [],
          saved_finding_count: 5,
        };
      });
      await nav(page, 'Bugs');
      const rows = page.locator('.result-row');
      for (const [index, priority] of ['critical', 'high', 'medium', 'low', 'info'].entries()) {
        const indicator = rows
          .nth(index)
          .getByRole('img', { name: `Priority: ${priority}`, exact: true });
        assert.equal(await indicator.isVisible(), true);
        assert.equal(await indicator.getAttribute('title'), `Priority: ${priority}`);
        assert.equal(await indicator.innerText(), '');
      }
      for (const state of ['fresh', 'stale', 'error']) {
        const dot = rows.getByRole('img', { name: `Freshness: ${state}`, exact: true }).first();
        assert.equal(await dot.isVisible(), true);
        assert.equal(await dot.getAttribute('title'), `Freshness: ${state}`);
      }
      for (const width of [1440, 700]) {
        await page.setViewportSize({ width, height: 900 });
        for (const theme of ['Graphite', 'Porcelain', 'Midnight']) {
          await page.getByRole('button', { name: `${theme} theme`, exact: true }).click();
          await layout(page, `finding-indicators-${width}-${theme}`);
        }
      }
      await rows.nth(1).click();
      await page.getByRole('img', { name: 'Freshness: stale', exact: true }).waitFor();
      assert.equal(
        await page.getByRole('button', { name: 'Prepare fix', exact: true }).isDisabled(),
        true,
      );
    } finally {
      await close();
    }
  });
  await test('Finding search, priority reset and numbered pagination remain local and keyboard accessible', async () => {
    const { page, close } = await pageFor();
    try {
      await page.evaluate(() => {
        const state = window.fixture.state;
        state.results.bugs = {
          semantic: Array.from({ length: 123 }, (_, index) => ({
            ...state.finding,
            id: `issue-${index}`,
            title: `Issue ${index}`,
            severity: index % 2 ? 'low' : 'high',
          })),
          performance: [],
          security: [],
          saved_finding_count: 123,
        };
      });
      await nav(page, 'Bugs');
      const before = await page.evaluate(() =>
        window.fixture.requests.filter((r) => r.method !== 'GET'),
      );
      const pagination = page.getByRole('navigation', { name: 'Findings pagination' });
      const search = page.getByLabel('Filter findings', { exact: true });
      const priority = page.getByLabel('Finding priority', { exact: true });
      assert.deepEqual(await priority.locator('option').allTextContents(), [
        'All priorities',
        'High',
        'Low',
      ]);
      assert.equal(
        await pagination.getByRole('button', { name: 'Previous page' }).isDisabled(),
        true,
      );
      await pagination.getByRole('button', { name: 'Page 13', exact: true }).press('Enter');
      await page.getByText('121–123 of 123 findings', { exact: true }).waitFor();
      assert.equal(await pagination.getByRole('button', { name: 'Next page' }).isDisabled(), true);
      assert.equal(
        await pagination
          .getByRole('button', { name: 'Page 13', exact: true })
          .getAttribute('aria-current'),
        'page',
      );
      await pagination.getByRole('button', { name: 'Page 11', exact: true }).click();
      await page.getByText('101–110 of 123 findings', { exact: true }).waitFor();
      for (const width of [1440, 700]) {
        await page.setViewportSize({ width, height: 900 });
        if (width === 700)
          await page.getByRole('button', { name: 'Larger text', exact: true }).click();
        for (const theme of ['Graphite', 'Porcelain', 'Midnight']) {
          await page.getByRole('button', { name: `${theme} theme`, exact: true }).click();
          await pagination.scrollIntoViewIfNeeded();
          assert.equal(
            await pagination.evaluate((el) => el.scrollWidth <= el.clientWidth + 1),
            true,
          );
          const bounds = await pagination.boundingBox();
          for (const button of await pagination.getByRole('button').all()) {
            const box = await button.boundingBox();
            assert.ok(box.x >= bounds.x - 1 && box.x + box.width <= bounds.x + bounds.width + 1);
          }
          await layout(page, `finding-pagination-${width}-${theme}`);
        }
      }
      await search.fill('Issue 12');
      await page.getByText('Page 1 of 1', { exact: true }).waitFor();
      assert.equal(await page.locator('.result-row').count(), 4);
      await priority.selectOption('low');
      assert.equal(await page.locator('.result-row strong').innerText(), 'Issue 121');
      await page.getByRole('button', { name: 'Reset filters', exact: true }).press('Enter');
      assert.equal(await priority.inputValue(), '');
      assert.equal(await search.inputValue(), '');
      assert.equal(await search.evaluate((el) => el === document.activeElement), true);
      await page.getByText('Page 1 of 13', { exact: true }).waitFor();
      await search.fill('no match');
      await page.getByRole('heading', { name: 'No matching findings', exact: true }).waitFor();
      await page.getByRole('button', { name: 'Clear finding search', exact: true }).press('Enter');
      assert.equal(await search.inputValue(), '');
      assert.equal(await search.evaluate((el) => el === document.activeElement), true);
      await search.fill('Issue 12');
      await search.press('Escape');
      assert.equal(await search.inputValue(), '');
      assert.equal(await page.locator('.result-row').count(), 10);
      await priority.selectOption('low');
      const fieldBounds = await priority.boundingBox();
      const arrowBounds = await page.locator('.priority-filter > .icon').boundingBox();
      assert.ok(
        arrowBounds.x > fieldBounds.x &&
          arrowBounds.x + arrowBounds.width < fieldBounds.x + fieldBounds.width,
      );
      await layout(page, 'finding-search-active-priority');
      assert.deepEqual(
        await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
        before,
      );
    } finally {
      await close();
    }
  });
  await test('Finding pages preserve category pagination, breadcrumbs and local navigation', async () => {
    for (const name of ['Bugs', 'Security', 'Performance']) {
      const category = name.toLowerCase();
      const { page, close } = await pageFor();
      try {
        await page.evaluate((category) => {
          const state = window.fixture.state;
          state.results[category] = {
            semantic: Array.from({ length: 23 }, (_, index) => ({
              ...state.finding,
              id: `finding-${index}`,
              category,
              title: `Worker issue ${index}`,
            })),
            performance: [],
            security: [],
            saved_finding_count: 23,
          };
        }, category);
        await nav(page, name);
        const breadcrumbs = page.getByRole('navigation', { name: 'Breadcrumb', exact: true });
        await breadcrumbs
          .locator('[aria-current="page"]')
          .getByText(name, { exact: true })
          .waitFor();
        assert.equal(await page.locator('.results-source, .results-reports').count(), 0);
        assert.equal(
          await page.locator('.results-findings').evaluate((el) => el.nextElementSibling === null),
          true,
          'The findings list is the final section, without a file list underneath',
        );
        await page.getByLabel('Filter findings', { exact: true }).fill('Worker issue');
        await page.getByLabel('Finding priority', { exact: true }).selectOption('high');
        await page.getByRole('button', { name: 'Next page', exact: true }).click();
        await page.getByText('11–20 of 23 findings', { exact: true }).waitFor();
        const before = await page.evaluate(() =>
          window.fixture.requests.filter((r) => r.method !== 'GET'),
        );
        await page.locator('.result-row').first().press('Enter');
        await page.locator('.studio-finding').waitFor();
        await page.getByRole('heading', { name: 'Worker issue 10', exact: true }).waitFor();
        assert.equal(await page.locator('.result-list, .findings-pagination').count(), 0);
        assert.equal(await page.getByLabel('Filter findings', { exact: true }).count(), 0);
        assert.equal(
          await page.locator('#main').evaluate((el) => el === document.activeElement),
          true,
        );
        assert.equal(
          await breadcrumbs.locator('[aria-current="page"]').innerText(),
          'Worker issue 10',
        );
        await breadcrumbs.getByRole('button', { name, exact: true }).waitFor();
        assert.equal(
          await page
            .getByRole('complementary', { name: 'Application' })
            .getByRole('button', { name: 'Findings', exact: true })
            .getAttribute('aria-current'),
          'page',
        );
        await page.getByRole('button', { name: 'Manage models', exact: true }).click();
        await page.getByRole('button', { name: 'Back to workspace', exact: true }).click();
        await idle(page);
        await page.getByRole('heading', { name: 'Worker issue 10', exact: true }).waitFor();
        await page.getByRole('button', { name: 'Search files and commands', exact: true }).click();
        await page.getByRole('dialog').waitFor();
        await page.keyboard.press('Escape');
        await page.getByRole('heading', { name: 'Worker issue 10', exact: true }).waitFor();
        for (const [width, large] of [
          [1440, false],
          [700, true],
        ]) {
          await page.setViewportSize({ width, height: 900 });
          if (large) await page.getByRole('button', { name: 'Larger text', exact: true }).click();
          assert.equal(await breadcrumbs.isVisible(), true);
          const header = await page.locator('.topbar').boundingBox();
          const trail = await breadcrumbs.boundingBox();
          assert.ok(trail.y >= header.y && trail.y + trail.height <= header.y + header.height);
          assert.equal(
            await breadcrumbs.evaluate((el) => el.scrollWidth <= el.clientWidth + 1),
            true,
          );
          await layout(page, `${category}-dedicated-detail-${width}`);
        }
        await breadcrumbs.getByRole('button', { name, exact: true }).press('Enter');
        await page.getByText('Page 2 of 3', { exact: true }).waitFor();
        assert.equal(
          await page.getByLabel('Filter findings', { exact: true }).inputValue(),
          'Worker issue',
        );
        assert.equal(
          await page.getByLabel('Finding priority', { exact: true }).inputValue(),
          'high',
        );
        assert.equal(
          await page.locator('.result-row strong').first().innerText(),
          'Worker issue 10',
        );
        await nav(page, name === 'Bugs' ? 'Performance' : 'Bugs');
        await nav(page, name);
        await page.getByText('Page 2 of 3', { exact: true }).waitFor();
        await page.getByRole('button', { name: 'Next page', exact: true }).click();
        await page.getByText('21–23 of 23 findings', { exact: true }).waitFor();
        await page.evaluate((category) => {
          const results = window.fixture.state.results[category];
          results.semantic = results.semantic.slice(0, 1);
          results.saved_finding_count = 1;
        }, category);
        await page.getByRole('button', { name: 'Refresh', exact: true }).click();
        await idle(page);
        await page.getByText('Page 1 of 1', { exact: true }).waitFor();
        assert.equal(await page.locator('.result-row').count(), 1);
        await page.locator('.result-row').click();
        await page.getByRole('button', { name: 'Manage models', exact: true }).click();
        await page.evaluate((category) => {
          window.fixture.state.results[category].semantic = [];
          window.fixture.state.results[category].saved_finding_count = 0;
        }, category);
        await page.getByRole('button', { name: 'Back to workspace', exact: true }).click();
        await page.getByRole('heading', { name: 'Finding unavailable', exact: true }).waitFor();
        assert.equal(
          await page.getByRole('button', { name: 'Prepare fix', exact: true }).count(),
          0,
        );
        await page.getByRole('button', { name: 'All findings', exact: true }).click();
        await page.getByRole('heading', { name: 'No saved findings', exact: true }).waitFor();
        assert.deepEqual(
          await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET')),
          before,
        );
        assert.equal(await page.evaluate(() => window.fixture.terminals.length), 0);
      } finally {
        await close();
      }
    }
  });
}
