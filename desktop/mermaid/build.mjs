import { build } from 'esbuild';
import { mkdir, copyFile } from 'node:fs/promises';
const output = new URL('../src/main/resources/mermaid/', import.meta.url);
await mkdir(output, { recursive: true });
await build({
  absWorkingDir: new URL('.', import.meta.url).pathname,
  entryPoints: ['renderer.js'],
  bundle: true,
  format: 'iife',
  globalName: 'MiniOrcaMermaid',
  platform: 'browser',
  target: 'es2022',
  minify: true,
  legalComments: 'eof',
  outfile: new URL('renderer.js', output).pathname,
});
for (const [source, target] of [
  ['beautiful-mermaid/LICENSE', 'beautiful-mermaid-LICENSE.txt'],
  ['elkjs/LICENSE.md', 'elkjs-LICENSE.md'],
  ['entities/LICENSE', 'entities-LICENSE.txt'],
]) await copyFile(new URL(`node_modules/${source}`, import.meta.url), new URL(target, output));
