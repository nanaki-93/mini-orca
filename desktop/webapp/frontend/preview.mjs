import http from 'node:http';
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
const assets = new URL('../internal/assets/files/', import.meta.url);
const types = {
  '.js': 'text/javascript',
  '.css': 'text/css',
  '.html': 'text/html',
  '.txt': 'text/plain',
};
export function serve(port = 4392) {
  return new Promise((resolve) => {
    const server = http.createServer(async (req, res) => {
      const name = new URL(req.url, 'http://localhost').pathname.slice(1) || 'index.html';
      if (!/^[a-zA-Z0-9.-]+$/.test(name)) {
        res.writeHead(404).end();
        return;
      }
      try {
        const data = await readFile(new URL(name, assets));
        res.setHeader(
          'Content-Type',
          types[name.slice(name.lastIndexOf('.'))] || 'application/octet-stream',
        );
        res.end(data);
      } catch {
        res.writeHead(404).end();
      }
    });
    server.listen(port, '127.0.0.1', () => resolve(server));
  });
}
if (process.argv[1] === fileURLToPath(import.meta.url)) {
  await serve();
  console.log('UI preview: http://127.0.0.1:4392 (native bridge required for projects)');
}
