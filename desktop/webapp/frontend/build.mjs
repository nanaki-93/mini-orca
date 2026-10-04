import { build } from 'esbuild';
import { copyFile, mkdir, readFile, writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const root = path.dirname(fileURLToPath(import.meta.url));
const output = path.resolve(root, '../internal/assets/files');
await mkdir(output, { recursive: true });
await build({
  entryPoints: [path.join(root, 'src/main.tsx')],
  outfile: path.join(output, 'app.js'),
  bundle: true,
  minify: true,
  sourcemap: false,
  target: 'es2022',
  legalComments: 'eof',
  define: { 'process.env.NODE_ENV': '"production"' },
});
await copyFile(path.join(root, 'index.html'), path.join(output, 'index.html'));
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
