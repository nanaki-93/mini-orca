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
}
