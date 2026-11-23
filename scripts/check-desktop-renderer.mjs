#!/usr/bin/env node
// Serve a prebuilt Desktop renderer without starting Electron or a daemon.
import { createHash } from "node:crypto";
import { createRequire } from "node:module";
import { readFile, realpath, writeFile } from "node:fs/promises";
import { resolve } from "node:path";

const [directory, rawPort, provenance] = process.argv.slice(2);
const port = Number(rawPort);
if (!directory || !provenance || !Number.isInteger(port) || port < 0 || port > 65535) {
  throw new Error("Expected renderer directory, TCP port and provenance path");
}
const renderer = await realpath(directory);
const index = await readFile(resolve(renderer, "index.html"));
// Vite is owned by the existing electron-vite dependency. Ignore ambient Vite
// configs: this server serves build output and must never compile development UI.
const fromDesktop = createRequire(new URL("../apps/desktop/package.json", import.meta.url));
const fromElectronVite = createRequire(fromDesktop.resolve("electron-vite"));
const { preview } = await import(fromElectronVite.resolve("vite"));
const server = await preview({
  configFile: false,
  root: renderer,
  build: { outDir: renderer },
  preview: { host: "127.0.0.1", port, strictPort: true, open: false },
});
const address = server.httpServer.address();
if (!address || typeof address === "string") throw new Error("Renderer did not bind TCP");
await writeFile(provenance, JSON.stringify({
  pid: process.pid, renderer, origin: `http://127.0.0.1:${address.port}`,
  index_sha256: createHash("sha256").update(index).digest("hex"),
}, null, 2) + "\n");
for (const signal of ["SIGTERM", "SIGINT"]) {
  process.once(signal, () => server.httpServer.close(() => process.exit(0)));
}
