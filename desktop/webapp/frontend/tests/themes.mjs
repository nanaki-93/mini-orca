import { strict as assert } from 'node:assert';

export async function testThemes({ test, pageFor, nav, idle, layout, contrast }) {
  await test('G, P and M themes support keyboard selection and remember appearance', async () => {
    const { page, close } = await pageFor();
    try {
      await idle(page);
      const group = page.getByRole('group', { name: 'Theme', exact: true });
      assert.deepEqual(await group.getByRole('button').allTextContents(), ['G', 'P', 'M']);
      assert.equal(await group.getByRole('button', { pressed: true }).innerText(), 'G');
      const surfaces = [];
      for (const [id, name, letter] of [
        ['light', 'Porcelain', 'P'],
        ['midnight', 'Midnight', 'M'],
        ['dark', 'Graphite', 'G'],
      ]) {
        const writes = await page.evaluate(() =>
          window.fixture.requests.filter((request) => request.method !== 'GET'),
        );
        const button = group.getByRole('button', { name: `${name} theme`, exact: true });
        assert.equal(await button.getAttribute('title'), `${name} (${letter})`);
        await button.focus();
        await button.press('Space');
        assert.equal(await button.evaluate((element) => element === document.activeElement), true);
        assert.equal(await group.getByRole('button', { pressed: true }).innerText(), letter);
        assert.equal(await page.locator('html').getAttribute('data-theme'), id);
        surfaces.push(
          await page
            .locator('html')
            .evaluate((element) => getComputedStyle(element).backgroundColor),
        );
        for (const [width, height, large] of [
          [1440, 1000, false],
          [800, 1000, false],
          [900, 640, true],
        ]) {
          await page.setViewportSize({ width, height });
          if (large) await page.getByRole('button', { name: 'Larger text', exact: true }).click();
          await button.focus();
          await button.press('Enter');
          const footer = await page.locator('.statusbar').boundingBox();
          assert.equal(footer.y + footer.height, height, 'Footer stays at the window edge');
          assert.equal(
            await page
              .locator('.statusbar')
              .evaluate((element) => element.scrollWidth > element.clientWidth),
            false,
            'All footer controls fit without clipping',
          );
          for (const control of await group.getByRole('button').all()) {
            const bounds = await control.boundingBox();
            assert.ok(bounds.width >= 24 && bounds.height >= 24);
            assert.ok(bounds.x >= footer.x && bounds.x + bounds.width <= footer.x + footer.width);
            assert.ok(bounds.y >= footer.y && bounds.y + bounds.height <= height);
          }
          await contrast(page, `${name} footer, focus and content at ${width}`);
          await layout(page, `theme-${id}-${width}`);
          if (large) await page.getByRole('button', { name: 'Larger text', exact: true }).click();
        }
        assert.deepEqual(
          await page.evaluate(() =>
            window.fixture.requests.filter((request) => request.method !== 'GET'),
          ),
          writes,
          'Changing theme and text size performs no work',
        );
        await page.reload();
        await page.getByRole('heading', { name: 'harbor', exact: true }).waitFor();
        await idle(page);
        assert.equal(await group.getByRole('button', { pressed: true }).innerText(), letter);
        assert.equal(await page.locator('html').getAttribute('data-theme'), id);
        assert.equal(await page.evaluate(() => localStorage.getItem('mini-orca:theme')), id);
      }
      assert.equal(new Set(surfaces).size, 3, 'Each theme renders a distinct palette');
      await page.evaluate(() => localStorage.setItem('mini-orca:theme', 'unknown-theme'));
      await page.reload();
      await page.getByRole('heading', { name: 'harbor', exact: true }).waitFor();
      assert.equal(await group.getByRole('button', { pressed: true }).innerText(), 'G');
      assert.equal(await page.locator('html').getAttribute('data-theme'), 'dark');
    } finally {
      await close();
    }
  });

  await test('Midnight keeps analysis, files, results and model selection readable', async () => {
    const { page, close } = await pageFor();
    try {
      await idle(page);
      const writes = await page.evaluate(() =>
        window.fixture.requests.filter((request) => request.method !== 'GET'),
      );
      await page.getByRole('button', { name: 'Midnight theme', exact: true }).click();
      await page.getByRole('button', { name: 'Larger text', exact: true }).click();
      await page.setViewportSize({ width: 800, height: 1000 });
      for (const name of [
        'Analysis',
        'Files',
        'Last run',
        'Bugs',
        'Performance',
        'Security',
        'Chat',
        'Source',
        'Models',
      ]) {
        await nav(page, name);
        await idle(page);
        await contrast(page, `${name} in Midnight`);
        await layout(page, `midnight-${name.toLowerCase().replaceAll(' ', '-')}`);
      }
      await nav(page, 'Analysis');
      await page.getByRole('button', { name: 'Bug analysis model', exact: true }).click();
      await page.getByRole('dialog', { name: 'Choose a model' }).waitFor();
      await contrast(page, 'Midnight model picker');
      await layout(page, 'midnight-model-picker');
      await page.keyboard.press('Escape');
      assert.equal(await page.getByRole('dialog').count(), 0);
      assert.deepEqual(
        await page.evaluate(() =>
          window.fixture.requests.filter((request) => request.method !== 'GET'),
        ),
        writes,
        'Appearance, navigation and model inspection remain passive',
      );
    } finally {
      await close();
    }
  });
}
