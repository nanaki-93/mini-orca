import { strict as assert } from 'node:assert';

const themes = [
  ['amethyst', 'Amethyst', 'A'],
  ['tokyo-night', 'Tokyo Night', 'T'],
  ['linear-carbon', 'Linear Carbon', 'L'],
  ['porcelain', 'Porcelain', 'P'],
];

export async function testThemes({ test, pageFor, nav, idle }) {
  await test('Collapsible 10-view sidebar keeps every destination accessible and remembers its width', async () => {
    const { page, close } = await pageFor();
    try {
      await idle(page);
      const sidebar = page.getByRole('complementary', { name: 'Application' });
      const destinations = await sidebar
        .locator('.nav-link')
        .evaluateAll((links) => links.map((link) => link.getAttribute('aria-label')));
      assert.deepEqual(destinations, [
        'Overview',
        'Architecture',
        'Docs',
        'Code & Diff',
        'Issues & Ideas',
        'Features',
        'Agents',
        'Last Run',
        'Git & Worktrees',
        'Settings & Rules',
        'Model Routing',
      ]);
      const expanded = await sidebar.boundingBox();
      await sidebar.getByRole('button', { name: 'Collapse sidebar' }).focus();
      await page.keyboard.press('Enter');
      const toggle = sidebar.getByRole('button', { name: 'Expand sidebar' });
      assert.equal(await toggle.getAttribute('aria-expanded'), 'false');
      assert.ok((await sidebar.boundingBox()).width < expanded.width / 2);
      for (const destination of destinations) {
        const link = sidebar.getByRole('button', { name: destination, exact: true });
        assert.equal(await link.getAttribute('title'), destination);
        assert.equal(await link.isVisible(), true);
      }
      await nav(page, 'Issues & Ideas');
      await idle(page);
      assert.equal(
        await sidebar
          .getByRole('button', { name: 'Issues & Ideas', exact: true })
          .getAttribute('aria-current'),
        'page',
      );
      await toggle.press('Space');
      assert.equal(
        await sidebar
          .getByRole('button', { name: 'Collapse sidebar' })
          .getAttribute('aria-expanded'),
        'true',
      );
    } finally {
      await close();
    }
  });

  await test('Curated theme pills switch between Amethyst, Tokyo Night, Linear Carbon, and Porcelain', async () => {
    const { page, close } = await pageFor();
    try {
      await idle(page);
      const group = page.getByRole('group', { name: 'Theme', exact: true });
      assert.deepEqual(
        await group.getByRole('button').allTextContents(),
        themes.map(([, name]) => name),
      );
      assert.equal(await group.getByRole('button', { pressed: true }).innerText(), 'Amethyst');
      for (const [id, name, letter] of [...themes.slice(1), themes[0]]) {
        const button = group.getByRole('button', { name: `${name} theme`, exact: true });
        assert.equal(await button.getAttribute('title'), `${name} (${letter})`);
        await button.click();
        assert.equal(await group.getByRole('button', { pressed: true }).innerText(), name);
        assert.equal(await page.locator('html').getAttribute('data-theme'), id);
        const mark = page.locator('.brand-symbol');
        assert.equal(await mark.getAttribute('src'), `/mo-mark-${id}.svg`);
      }
    } finally {
      await close();
    }
  });
}
