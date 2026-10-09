import { createRequire } from 'node:module';
import { writeFile } from 'node:fs/promises';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const dir = import.meta.dirname;
const root = path.resolve(dir, '../../..');
const require = createRequire(path.join(root, 'packages/views/package.json'));
const { createServer } = await import(pathToFileURL(require.resolve('vite')));
const { default: react } = await import(pathToFileURL(require.resolve('@vitejs/plugin-react')));
const { default: tailwind } = await import(pathToFileURL(require.resolve('@tailwindcss/vite')));
const server = await createServer({
  configFile: false,
  envFile: false,
  root: dir,
  cacheDir: path.join(dir, '.cache'),
  plugins: [react(), tailwind()],
  resolve: { dedupe: ['react', 'react-dom', '@tanstack/react-query', 'react-i18next', 'i18next'] },
  server: { host: '127.0.0.1', port: 0, fs: { allow: [root] } },
});
await server.listen();
const address = server.httpServer.address();
const runtime = { pid: process.pid, url: `http://127.0.0.1:${address.port}`, started: new Date().toISOString(), running: true };
await writeFile(path.join(dir, 'runtime.json'), JSON.stringify(runtime, null, 2));
console.log(JSON.stringify(runtime));
let stopping = false;
async function close() {
  if (stopping) return;
  stopping = true;
  await server.close();
  await writeFile(path.join(dir, 'runtime.json'), JSON.stringify({ ...runtime, running: false, stopped: new Date().toISOString() }, null, 2));
  process.exit(0);
}
process.on('SIGTERM', close);
process.on('SIGINT', close);
await new Promise(() => {});
