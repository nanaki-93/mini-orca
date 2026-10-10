import { strict as assert } from 'node:assert';

export async function testStudio({ test, pageFor, nav, idle, layout }) {
  await test('Studio pages fill the available workspace width', async () => {
    const { page, close } = await pageFor();
    try {
      for (const [width, height, larger] of [
        [1920, 1080, false],
        [900, 640, true],
      ]) {
        await page.setViewportSize({ width, height });
        if (larger) await page.getByRole('button', { name: 'Larger text', exact: true }).click();
        for (const collapsed of [false, true]) {
          if (collapsed)
            await page.getByRole('button', { name: 'Collapse sidebar', exact: true }).click();
          for (const [route, selector] of [
            ['Overview', '.summary-dashboard'],
            ['Analysis', '.analysis-setup'],
            ['Chat', '.chat-composer'],
            ['Source', '.source-workspace'],
          ]) {
            await nav(page, route);
            await idle(page);
            const workspace = await page.locator('#main').boundingBox();
            const content = await page.locator('#main > .page').boundingBox();
            const surface = await page.locator(selector).boundingBox();
            const padding = await page.locator('#main > .page').evaluate((element) => {
              const style = getComputedStyle(element);
              return { left: parseFloat(style.paddingLeft), right: parseFloat(style.paddingRight) };
            });
            assert.ok(Math.abs(content.x - workspace.x) < 2, `${route}: no unused left band`);
            assert.ok(
              Math.abs(content.width - workspace.width) < 2,
              `${route}: page fills the workspace`,
            );
            assert.ok(
              Math.abs(surface.x - content.x - padding.left) < 2 &&
                Math.abs(surface.width - content.width + padding.left + padding.right) < 2,
              `${route}: working content fills the page inside its padding`,
            );
            await layout(page, `studio-full-width-${route.toLowerCase()}-${width}-${collapsed}`);
          }
        }
        await page.getByRole('button', { name: 'Expand sidebar', exact: true }).click();
      }
    } finally {
      await close();
    }
  });
  await test('Studio overlays and terminal preserve the source workspace', async () => {
    const { page, close } = await pageFor();
    try {
      await nav(page, 'Source');
      await page.locator('.file-item').first().click();
      await idle(page);
      const source = await page.locator('.source-code').innerText();
      const mutations = await page.evaluate(
        () => window.fixture.requests.filter((r) => r.method !== 'GET').length,
      );
      await page.getByRole('button', { name: 'Search files and commands', exact: true }).click();
      const search = page.getByRole('dialog', { name: 'Search files and commands' });
      await search.waitFor();
      await page.waitForFunction(() => document.activeElement?.id === 'search');
      await search.getByLabel('Search files and commands', { exact: true }).fill('no-such-file');
      await search.getByText('No matching files.', { exact: true }).waitFor();
      await page.keyboard.press('Escape');
      assert.equal(await page.locator('.source-code').innerText(), source);
      await page.getByRole('button', { name: 'Terminal', exact: true }).click();
      await page.getByRole('region', { name: 'Terminal drawer' }).waitFor();
      assert.equal(await page.locator('.source-code').isVisible(), true);
      assert.equal(
        await page.evaluate(() => window.fixture.requests.filter((r) => r.method !== 'GET').length),
        mutations,
      );
      await page.getByRole('button', { name: 'Close terminal drawer' }).click();
      await page.getByRole('button', { name: /^Switch project/ }).click();
      await page.getByRole('dialog', { name: 'Switch project', exact: true }).waitFor();
      assert.equal(await page.getByLabel('Project folder', { exact: true }).isVisible(), true);
      await page.keyboard.press('Escape');
      assert.equal(await page.locator('.source-code').innerText(), source);
      await layout(page, 'studio-source-canvas');
    } finally {
      await close();
    }
  });
  await test('Studio project pages use a hub, readable guidance and role settings', async () => {
    const { page, close } = await pageFor();
    try {
      await layout(page, 'studio-overview-hub');
      await nav(page, 'Instructions');
      await page.locator('.instruction-reading').waitFor();
      assert.equal(await page.locator('.wizard-steps').count(), 0);
      assert.equal(await page.getByLabel('Custom instructions', { exact: true }).count(), 0);
      await page.getByRole('button', { name: 'Edit draft', exact: true }).click();
      await page
        .getByLabel('Custom instructions', { exact: true })
        .fill('# Project guidance\n\nPreserve cancellation.');
      await layout(page, 'studio-instructions-editor');
      await page.getByRole('button', { name: 'Preview instruction diff', exact: true }).click();
      await idle(page);
      await page.locator('.diff').first().waitFor();
      assert.equal(
        await page.evaluate(() => window.fixture.requests.some((r) => r.path.endsWith('/apply'))),
        false,
      );
      await nav(page, 'Models');
      assert.equal(await page.locator('.analysis-model-trigger').count(), 7);
      await layout(page, 'studio-model-roles');
      await nav(page, 'Architecture');
      await page.locator('.architecture-canvas').waitFor();
      await layout(page, 'studio-architecture');
      await nav(page, 'History');
      await layout(page, 'studio-history');
    } finally {
      await close();
    }
  });
  await test('Studio findings, feature dialogs and analysis preview keep context', async () => {
    const { page, close } = await pageFor({ featuresReady: true });
    try {
      await nav(page, 'Bugs');
      await page.locator('.result-row').first().click();
      await page.locator('.results-detail').waitFor();
      assert.equal(await page.locator('.result-list').isVisible(), true);
      await layout(page, 'studio-inline-finding');
      await nav(page, 'Features');
      await page.locator('.feature-row').first().click();
      await page.getByRole('dialog').waitFor();
      await layout(page, 'studio-feature-dialog');
      await page.keyboard.press('Escape');
      await nav(page, 'Analysis');
      await page.getByRole('button', { name: 'Prepare analysis', exact: true }).click();
      const preview = page.getByRole('dialog', { name: 'Analysis preview', exact: true });
      await preview.waitFor();
      assert.equal(await page.locator('.analysis-setup').isVisible(), true);
      await layout(page, 'studio-analysis-preview');
      await page.keyboard.press('Escape');
      assert.equal(
        await page.evaluate(() =>
          window.fixture.requests.some(
            (r) => r.path.endsWith('/analysis/runs') && r.method === 'POST',
          ),
        ),
        false,
      );
    } finally {
      await close();
    }
  });
  await test('Studio chat and changes share file-by-file review and explicit Apply', async () => {
    const { page, close } = await pageFor({ trusted: true });
    try {
      await nav(page, 'Chat');
      await page
        .getByLabel('Files to change', { exact: true })
        .fill('internal/worker.go\ninternal/worker_test.go');
      await page
        .getByLabel('Change request', { exact: true })
        .fill('Handle cancellation with regression coverage.');
      assert.equal(await page.getByLabel('Creation model', { exact: true }).count(), 0);
      await page.getByRole('button', { name: 'Generate changes', exact: true }).click();
      await page.getByRole('heading', { name: 'Ready for review', exact: true }).waitFor();
      await layout(page, 'studio-task-conversation');
      await page.getByRole('button', { name: 'Open proposal', exact: true }).click();
      await page.getByRole('complementary', { name: 'Review evidence' }).waitFor();
      assert.equal(
        await page.getByRole('tablist', { name: 'Files to change' }).getByRole('tab').count(),
        2,
      );
      assert.equal(await page.getByRole('button', { name: /^Apply \d/ }).count(), 0);
      const workspace = await page.locator('#main').boundingBox();
      const review = await page.locator('.fix-review').boundingBox();
      const footer = await page.locator('.fix-action-bar').boundingBox();
      const status = await page.locator('.statusbar').boundingBox();
      assert.ok(Math.abs(review.width - workspace.width) < 2, 'Review fills the workspace');
      assert.ok(Math.abs(footer.y + footer.height - status.y) < 2, 'Apply stays at the bottom');
      await layout(page, 'studio-review-evidence');
      await page.getByRole('button', { name: 'Review next file', exact: true }).click();
      const apply = page.getByRole('button', { name: 'Apply 2 files', exact: true });
      assert.equal(await apply.isEnabled(), true);
      assert.equal(
        await page.evaluate(() => window.fixture.requests.some((r) => r.path.endsWith('/apply'))),
        false,
      );
      await apply.click();
      await idle(page);
      await page.getByRole('heading', { name: 'Change applied', exact: true }).waitFor();
      assert.equal(
        await page.evaluate(
          () => window.fixture.requests.filter((r) => r.path.endsWith('/apply')).length,
        ),
        1,
      );
      for (const theme of ['Porcelain', 'Midnight']) {
        await page.getByRole('button', { name: `${theme} theme`, exact: true }).click();
        await layout(page, `studio-review-${theme.toLowerCase()}`);
      }
    } finally {
      await close();
    }
  });
}
