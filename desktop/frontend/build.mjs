import { build } from 'esbuild';
import { copyFile, mkdir, readFile, readdir, rm, writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const root = path.dirname(fileURLToPath(import.meta.url));
const output = path.resolve(root, '../internal/assets/files');
const branding = path.resolve(root, '../../docs/icons');
await mkdir(output, { recursive: true });
// Remove stale generated chunks while retaining the tracked asset placeholder.
for (const name of await readdir(output))
  if (name.endsWith('.js') || name.endsWith('.css')) await rm(path.join(output, name));
await build({
  entryPoints: { app: path.join(root, 'src/main.tsx') },
  outdir: output,
  format: 'esm',
  splitting: true,
  bundle: true,
  minify: true,
  sourcemap: false,
  target: 'es2022',
  legalComments: 'eof',
  define: { 'process.env.NODE_ENV': '"production"' },
});
await copyFile(path.join(root, 'index.html'), path.join(output, 'index.html'));
for (const theme of ['graphite', 'porcelain', 'midnight', 'signal', 'crimson-violet']) {
  await copyFile(
    path.join(branding, theme, 'ui/mo-mark.svg'),
    path.join(output, `mo-mark-${theme}.svg`),
  );
}
const midnightSvg = await readFile(path.join(branding, 'midnight/ui/mo-mark.svg'), 'utf8');
const nordicSvg = midnightSvg
  .replaceAll('midnight transparent mark', 'nordic transparent mark')
  .replaceAll('#EAF4FF', '#EAF2FD')
  .replaceAll('#78CDCF', '#60A5FA');
await writeFile(path.join(output, 'mo-mark-nordic.svg'), nordicSvg);

const crimsonSvg = await readFile(path.join(branding, 'crimson-violet/ui/mo-mark.svg'), 'utf8');
const amethystSvg = crimsonSvg
  .replaceAll('Crimson Violet transparent mark', 'amethyst transparent mark')
  .replaceAll('#CCA6FF', '#F3E8FF')
  .replaceAll('#85E89D', '#C084FC');
await writeFile(path.join(output, 'mo-mark-amethyst.svg'), amethystSvg);
await copyFile(path.join(branding, 'favicon.png'), path.join(output, 'favicon.png'));
await copyFile(
  path.join(branding, 'graphite/app-icon/app-icon-1024.png'),
  path.join(output, 'app-icon.png'),
);
const packages = [
  'react',
  'react-dom',
  'scheduler',
  '@xterm/xterm',
  '@xterm/addon-fit',
  'beautiful-mermaid',
  'elkjs',
  'entities',
];
const licenses = [];
for (const name of packages) {
  const location = path.join(root, 'node_modules', name);
  const { version, license } = JSON.parse(
    await readFile(path.join(location, 'package.json'), 'utf8'),
  );
  const text = await readFile(
    path.join(location, name === 'elkjs' ? 'LICENSE.md' : 'LICENSE'),
    'utf8',
  );
  licenses.push(`${name} ${version} (${license})\n${text}`);
}
await writeFile(path.join(output, 'licenses.txt'), licenses.join('\n\n'));
