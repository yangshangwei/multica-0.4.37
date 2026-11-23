// @vitest-environment node
import { createHash } from "node:crypto";
import { execFile } from "node:child_process";
import { createServer } from "node:http";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";
import { afterEach, describe, expect, it } from "vitest";
import { verifyUpdates } from "./verify-updates.mjs";

const servers = [];
const run = promisify(execFile);
const cli = fileURLToPath(new URL("./verify-updates.mjs", import.meta.url));
const sha512 = (bytes) => createHash("sha512").update(bytes).digest("base64");

async function fixture({ version = "1.2.3", override } = {}) {
  const name = `multica-desktop-${version}-windows-x64.exe`;
  const bytes = Buffer.from("installer content ".repeat(256));
  const metadata = {
    version,
    files: [{ url: name, size: bytes.length, sha512: sha512(bytes) }],
    path: name,
    sha512: sha512(bytes),
  };
  const files = new Map([[name, bytes], [`${name}.blockmap`, Buffer.from("blockmap content")]]);
  const requests = [];
  let rawMetadata;
  const server = createServer((req, res) => {
    requests.push({ url: req.url, method: req.method, range: req.headers.range });
    if (override?.(req, res)) return;
    const file = req.url?.slice("/desktop/".length);
    if (file === "latest.yml") {
      res.writeHead(200, { "Cache-Control": "no-cache", "Content-Type": "text/yaml" });
      res.end(rawMetadata ?? JSON.stringify(metadata));
      return;
    }
    const content = files.get(file);
    if (!content) { res.writeHead(404); res.end(); return; }
    if (req.method === "HEAD") {
      res.writeHead(200, { "Content-Length": content.length });
      res.end();
    } else if (req.headers.range) {
      const end = Math.min(1023, content.length - 1);
      res.writeHead(206, { "Content-Length": end + 1, "Content-Range": `bytes 0-${end}/${content.length}` });
      res.end(content.subarray(0, end + 1));
    } else {
      res.writeHead(200, { "Content-Length": content.length });
      res.end(content);
    }
  });
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  servers.push(server);
  return { url: `http://127.0.0.1:${server.address().port}/desktop`, metadata, files, requests, name, bytes, setMetadata: (value) => { rawMetadata = value; } };
}

afterEach(async () => {
  for (const server of servers.splice(0)) {
    server.closeAllConnections();
    await new Promise((resolve) => server.close(resolve));
  }
});

describe("desktop HTTP update verification", () => {
  it("streams a real feed, verifies all reference forms and reports sidecar evidence", async () => {
    const f = await fixture({ version: "1.2.3-dirty" });
    const packageName = "multica-desktop-1.2.3-dirty-windows-x64.7z";
    const packageBytes = Buffer.from("package content");
    f.files.set(packageName, packageBytes);
    f.metadata.packages = { x64: { path: packageName, size: packageBytes.length, sha512: sha512(packageBytes) } };
    const result = await verifyUpdates({ url: f.url, expectedVersion: "1.2.3-dirty" });
    expect(result.version).toBe("1.2.3-dirty");
    expect(result.metadata).toMatchObject({ name: "latest.yml", cacheControl: "no-cache" });
    expect(result.artifacts).toHaveLength(2);
    expect(result.artifacts[0]).toMatchObject({ name: f.name, bytes: f.bytes.length, sha512: sha512(f.bytes), checksumVerified: true, headStatus: 200, rangeStatus: 206, rangeBytes: 1024 });
    expect(result.blockmaps).toContainEqual(expect.objectContaining({ name: `${f.name}.blockmap`, available: true, checksumVerified: false, rangeStatus: 206 }));
    expect(f.requests.filter((r) => r.url.endsWith(f.name) && r.method === "GET" && !r.range)).toHaveLength(1);
  });

  it("reports optional missing blockmaps without failing the verified installer", async () => {
    const f = await fixture();
    f.files.delete(`${f.name}.blockmap`);
    expect((await verifyUpdates({ url: f.url })).blockmaps).toContainEqual({ name: `${f.name}.blockmap`, available: false, status: 404 });
  });

  it("rejects an unexpected version before downloading artifacts", async () => {
    const f = await fixture();
    await expect(verifyUpdates({ url: f.url, expectedVersion: "2.0.0" })).rejects.toThrow(/version.*2\.0\.0/i);
    expect(f.requests).toHaveLength(1);
  });

  it.each(["../outside.exe", "https://example.test/file.exe", "//example.test/file.exe", "%2e%2e%2ffile.exe", "/file.exe", "..\\file.exe", "file.exe?token=1"])("rejects unsafe references before fetching: %s", async (name) => {
    const f = await fixture();
    f.metadata.files[0].url = name;
    await expect(verifyUpdates({ url: f.url })).rejects.toThrow(/unsafe.*filename/i);
    expect(f.requests).toHaveLength(1);
  });

  it.each(["sha512", "size", "legacy", "package", "files", "record", "version", "yaml"])("rejects invalid %s metadata", async (kind) => {
    const f = await fixture();
    if (kind === "sha512") f.metadata.files[0].sha512 = sha512("different bytes");
    if (kind === "size") f.metadata.files[0].size += 1;
    if (kind === "legacy") f.metadata.sha512 = sha512("different legacy bytes");
    if (kind === "package") f.metadata.packages = { x64: { path: "../package.7z", size: 1, sha512: sha512("a") } };
    if (kind === "files") f.metadata.files = [];
    if (kind === "record") f.metadata.files = [null];
    if (kind === "version") f.metadata.version = "not-a-version";
    if (kind === "yaml") f.setMetadata("version: [");
    await expect(verifyUpdates({ url: f.url })).rejects.toThrow();
  });

  it.each(["metadata", "HEAD", "range", "download", "blockmap"])("rejects redirects from %s without following them", async (stage) => {
    const f = await fixture({ override: (req, res) => {
      const match = stage === "metadata" ? req.url.endsWith(".yml")
        : stage === "blockmap" ? req.url.endsWith(".blockmap")
          : req.url.endsWith(".exe") && (stage === "HEAD" ? req.method === "HEAD" : req.method === "GET" && Boolean(req.headers.range) === (stage === "range"));
      if (!match) return false;
      res.writeHead(302, { Location: "/should-not-be-fetched" }); res.end(); return true;
    } });
    await expect(verifyUpdates({ url: f.url })).rejects.toThrow(/redirect|HTTP 302/i);
    expect(f.requests.some((r) => r.url === "/should-not-be-fetched")).toBe(false);
  });

  it.each(["missing", "head-length", "range-status", "range-header", "range-short", "range-content", "full-corruption", "full-short", "cache", "encoding", "blockmap-status"])("rejects broken HTTP behavior: %s", async (kind) => {
    const f = await fixture({ override: (req, res) => {
      if (kind === "cache" && req.url.endsWith(".yml")) { res.writeHead(200); res.end("{}"); return true; }
      if (kind === "blockmap-status" && req.url.endsWith(".blockmap")) { res.writeHead(503); res.end(); return true; }
      if (!req.url.endsWith(".exe")) return false;
      if (kind === "missing") { res.writeHead(404); res.end(); return true; }
      if (kind === "encoding") { res.writeHead(200, { "Content-Encoding": "gzip" }); res.end(); return true; }
      if (kind === "head-length" && req.method === "HEAD") { res.writeHead(200, { "Content-Length": f.bytes.length + 1 }); res.end(); return true; }
      if (kind.startsWith("range-") && req.headers.range) {
        const bytes = kind === "range-short" ? f.bytes.subarray(0, 8) : kind === "range-content" ? Buffer.alloc(1024) : f.bytes.subarray(0, 1024);
        res.writeHead(kind === "range-status" ? 200 : 206, { "Content-Length": bytes.length, "Content-Range": kind === "range-header" ? `bytes 1-1024/${f.bytes.length}` : `bytes 0-1023/${f.bytes.length}` });
        res.end(bytes); return true;
      }
      if (kind.startsWith("full-") && req.method === "GET" && !req.headers.range) {
        res.writeHead(200);
        res.end(kind === "full-short" ? f.bytes.subarray(0, 20) : Buffer.alloc(f.bytes.length)); return true;
      }
      return false;
    } });
    await expect(verifyUpdates({ url: f.url })).rejects.toThrow();
  });

  it("bounds artifact size before issuing downloads", async () => {
    const f = await fixture();
    await expect(verifyUpdates({ url: f.url, maxArtifactBytes: 2048 })).rejects.toThrow(/limit|large/i);
    expect(f.requests.some((r) => r.url.endsWith(".exe") && r.method === "GET")).toBe(false);
  });

  it("bounds metadata and aborts stalled responses", async () => {
    const f = await fixture();
    f.setMetadata("x".repeat(1024 * 1024 + 1));
    await expect(verifyUpdates({ url: f.url })).rejects.toThrow(/limit|large/i);
    const stalled = await fixture({ override: () => true });
    await expect(verifyUpdates({ url: stalled.url, timeoutMs: 50 })).rejects.toThrow(/timeout|timed out/i);
  });

  it.each(["file:///desktop", "http://user:secret@localhost/desktop", "http://localhost/desktop?token=1", "http://localhost/desktop#fragment"])("rejects invalid feed URL %s", async (url) => {
    await expect(verifyUpdates({ url })).rejects.toThrow(/URL/i);
  });

  it("offers CLI help, JSON evidence, and nonzero failure status", async () => {
    const f = await fixture();
    expect((await run(process.execPath, [cli, "--help"])).stdout).toMatch(/--expected-version/);
    const success = await run(process.execPath, [cli, "--url", f.url, "--expected-version", "1.2.3"]);
    expect(JSON.parse(success.stdout).artifacts[0].checksumVerified).toBe(true);
    await expect(run(process.execPath, [cli, "--url", f.url, "--expected-version", "9.9.9"])).rejects.toMatchObject({ code: 1 });
    await expect(run(process.execPath, [cli, "--unknown"])).rejects.toMatchObject({ code: 1 });
    await expect(run(process.execPath, [cli, "--url", f.url, "--metadata", "../latest.yml"])).rejects.toMatchObject({ code: 1 });
  });
});
