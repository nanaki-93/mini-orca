import { strict as assert } from 'node:assert';

const themes = [
  ['dark', 'Graphite', 'G'],
  ['light', 'Porcelain', 'P'],
  ['midnight', 'Midnight', 'M'],
  ['signal', 'Signal', 'S'],
  ['crimson-violet', 'Crimson Violet', 'C'],
  ['nordic', 'Nordic', 'N'],
  ['amethyst', 'Amethyst', 'A'],
];

export async function testThemes({ test, pageFor, nav, idle, layout, contrast }) {
  await test('Collapsible sidebar keeps every destination accessible and remembers its width', async () => {
    const { page, close } = await pageFor();
    try {
      await idle(page);
      const sidebar = page.getByRole('complementary', { name: 'Application' });
      const destinations = await sidebar
        .locator('.nav-link')
        .evaluateAll((links) => links.map((link) => link.getAttribute('aria-label')));
      assert.deepEqual(destinations, [
        'Overview',
        'Source',
        'Chat',
        'Analysis',
        'Findings',
        'Features',
        'Changes',
        'Git / Worktrees',
        'Architecture',
        'Instructions',
        'Context',
        'Models',
        'History',
      ]);
      const writes = await page.evaluate(() =>
        window.fixture.requests.filter((request) => request.method !== 'GET'),
      );
      const expanded = await sidebar.boundingBox();
      await sidebar.getByRole('button', { name: 'Collapse sidebar' }).focus();
      await page.keyboard.press('Enter');
      const toggle = sidebar.getByRole('button', { name: 'Expand sidebar' });
      assert.equal(await toggle.getAttribute('aria-expanded'), 'false');
      assert.equal(await toggle.evaluate((element) => element === document.activeElement), true);
      assert.ok((await sidebar.boundingBox()).width < expanded.width / 2);
      for (const destination of destinations) {
        const link = sidebar.getByRole('button', { name: destination, exact: true });
        assert.equal(await link.getAttribute('title'), destination);
        assert.equal(await link.locator('span').first().isVisible(), false);
        await link.scrollIntoViewIfNeeded();
        assert.equal(await link.isVisible(), true);
      }
      await nav(page, 'Bugs');
      await idle(page);
      assert.equal(
        await sidebar
          .getByRole('button', { name: 'Findings', exact: true })
          .getAttribute('aria-current'),
        'page',
      );
      await nav(page, 'Security');
      assert.equal(
        await sidebar
          .getByRole('button', { name: 'Findings', exact: true })
          .getAttribute('aria-current'),
        'page',
      );
      await nav(page, 'Last run');
      assert.equal(
        await sidebar
          .getByRole('button', { name: 'Analysis', exact: true })
          .getAttribute('aria-current'),
        'page',
      );
      await nav(page, 'Terminal');
      assert.equal(
        await page
          .locator('.statusbar')
          .getByRole('button', { name: 'Terminal', exact: true })
          .getAttribute('aria-expanded'),
        'true',
      );
      await nav(page, 'Bugs');
      for (const [, theme] of themes) {
        await page.getByRole('button', { name: `${theme} theme`, exact: true }).click();
        await page.setViewportSize({ width: 800, height: 640 });
        await toggle.focus();
        await contrast(page, `${theme} collapsed sidebar`);
        await layout(page, `sidebar-collapsed-${theme.toLowerCase()}`);
      }
      assert.deepEqual(
        await page.evaluate(() =>
          window.fixture.requests.filter((request) => request.method !== 'GET'),
        ),
        writes,
        'Sidebar changes and navigation perform no project work',
      );
      await page.reload();
      await page.getByRole('heading', { name: 'harbor', exact: true }).waitFor();
      await idle(page);
      assert.equal(await toggle.isVisible(), true);
      await toggle.press('Space');
      assert.equal(
        await sidebar
          .getByRole('button', { name: 'Collapse sidebar' })
          .getAttribute('aria-expanded'),
        'true',
      );
      assert.equal(
        await sidebar
          .getByRole('button', { name: 'Overview', exact: true })
          .locator('span')
          .first()
          .isVisible(),
        true,
      );
    } finally {
      await close();
    }
  });

  await test('Theme selectors support keyboard selection and remember appearance', async () => {
    const { page, close } = await pageFor();
    try {
      await idle(page);
      const group = page.getByRole('group', { name: 'Theme', exact: true });
      assert.deepEqual(
        await group.getByRole('button').allTextContents(),
        themes.map(([, , letter]) => letter),
      );
      assert.equal(await group.getByRole('button', { pressed: true }).innerText(), 'G');
      const surfaces = [];
      for (const [id, name, letter] of [...themes.slice(1), themes[0]]) {
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
        const mark = page.locator('.brand-symbol');
        await mark.evaluate((element) => element.decode());
        assert.equal(
          await mark.getAttribute('src'),
          `/mo-mark-${name.toLowerCase().replaceAll(' ', '-')}.svg`,
        );
        assert.ok(await mark.evaluate((element) => element.naturalWidth > 0));
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
          assert.deepEqual(
            await page
              .locator('.statusbar-end > *')
              .evaluateAll((controls) =>
                controls
                  .filter((control) => control.scrollWidth > control.clientWidth + 1)
                  .map((control) => control.textContent.trim()),
              ),
            [],
            'Footer action labels fit inside their controls',
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
        const action = page.getByRole('button', { name: 'Start a task', exact: true });
        await action.hover();
        await contrast(page, `${name} primary action hover`);
        await action.focus();
        await contrast(page, `${name} primary action focus`);
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
      assert.equal(new Set(surfaces).size, themes.length, 'Each theme renders a distinct palette');
      await page.emulateMedia({ reducedMotion: 'reduce' });
      const task = page.getByRole('button', { name: 'Start a task', exact: true });
      await task.hover();
      assert.equal(
        await task.evaluate((element) => getComputedStyle(element).transitionDuration),
        '0s',
        'Reduced motion disables hover transitions',
      );
      await page.evaluate(() => localStorage.setItem('mini-orca:theme', 'unknown-theme'));
      await page.reload();
      await page.getByRole('heading', { name: 'harbor', exact: true }).waitFor();
      assert.equal(await group.getByRole('button', { pressed: true }).innerText(), 'G');
      assert.equal(await page.locator('html').getAttribute('data-theme'), 'dark');
    } finally {
      await close();
    }
  });

  for (const theme of ['Midnight', 'Signal', 'Crimson Violet']) {
    await test(`${theme} keeps analysis, files, results and model selection readable`, async () => {
      const { page, close } = await pageFor();
      try {
        await idle(page);
        const writes = await page.evaluate(() =>
          window.fixture.requests.filter((request) => request.method !== 'GET'),
        );
        await page.getByRole('button', { name: `${theme} theme`, exact: true }).click();
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
          await contrast(page, `${name} in ${theme}`);
          await layout(page, `${theme.toLowerCase()}-${name.toLowerCase().replaceAll(' ', '-')}`);
        }
        await nav(page, 'Models');
        await page.getByRole('button', { name: 'Bug analysis model', exact: true }).click();
        await page.getByRole('dialog', { name: 'Choose a model' }).waitFor();
        await contrast(page, `${theme} model picker`);
        await layout(page, `${theme.toLowerCase()}-model-picker`);
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
}
