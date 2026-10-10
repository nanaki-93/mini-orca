import { strict as assert } from 'node:assert';

export async function testStudio({ test, pageFor, nav, idle, layout }) {
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
}
