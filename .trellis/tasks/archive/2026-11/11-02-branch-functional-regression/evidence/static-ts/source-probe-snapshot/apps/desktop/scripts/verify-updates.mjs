#!/usr/bin/env node
import { createHash } from "node:crypto";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { parseArgs } from "node:util";
import { parseUpdateInfo } from "electron-updater/out/providers/Provider.js";
import { metadataName, updateReferences } from "./update-artifacts.mjs";

function feedUrl(value) {
  const url = new URL(value);
  if (!["http:", "https:"].includes(url.protocol) || url.username || url.password || url.search || url.hash) {
    throw new Error("Feed URL must use HTTP/HTTPS without credentials, query or fragment");
  }
  return url.href.replace(/\/+$/, "") + "/";
}

async function readBytes(response, limit, collect = false) {
  let bytes = 0;
  let prefix = Buffer.alloc(0);
  const chunks = [];
  const hash = createHash("sha512");
  for await (const chunk of response.body) {
    bytes += chunk.length;
    if (bytes > limit) throw new Error(`Response exceeds byte limit ${limit}`);
    hash.update(chunk);
    if (prefix.length < 1024) prefix = Buffer.concat([prefix, chunk.subarray(0, 1024 - prefix.length)]);
    if (collect) chunks.push(chunk);
  }
  return { bytes, sha512: hash.digest("base64"), prefix, content: collect ? Buffer.concat(chunks) : undefined };
}

export async function verifyUpdates({ url, metadata = "latest.yml", expectedVersion, timeoutMs = 30 * 60 * 1000, maxArtifactBytes = 8 * 1024 ** 3 } = {}) {
  const base = feedUrl(url);
  if (!metadataName.test(metadata)) throw new Error(`Invalid metadata filename: ${metadata}`);

  async function request(name, method = "GET", headers = {}, optional = false) {
    const response = await fetch(new URL(name, base), { method, headers: { "Accept-Encoding": "identity", ...headers }, redirect: "manual", signal: AbortSignal.timeout(timeoutMs) });
    const allowed = response.status === 200 || (headers.Range && response.status === 206) || (optional && response.status === 404);
    if (!allowed) {
      await response.body?.cancel();
      throw new Error(`${method} ${name}: HTTP ${response.status}; redirects are not followed`);
    }
    const encoding = response.headers.get("content-encoding");
    if (encoding && encoding !== "identity") {
      await response.body?.cancel();
      throw new Error(`${name}: unexpected content encoding ${encoding}`);
    }
    return response;
  }

  async function artifact(reference, suppliedHead, limit = maxArtifactBytes) {
    const { name } = reference;
    if (reference.size !== undefined && reference.size > limit) throw new Error(`${name}: artifact size exceeds limit ${limit}`);
    const head = suppliedHead ?? await request(name, "HEAD");
    const length = head.headers.get("content-length");
    const size = Number(length);
    if (!length || !Number.isSafeInteger(size) || size <= 0 || size > limit) throw new Error(`${name}: invalid Content-Length or byte limit exceeded`);
    if (reference.size !== undefined && reference.size !== size) throw new Error(`${name}: HEAD size mismatch`);
    const rangeSize = Math.min(size, 1024);
    const range = await request(name, "GET", { Range: `bytes=0-${rangeSize - 1}` });
    if (range.status !== 206 || range.headers.get("content-range") !== `bytes 0-${rangeSize - 1}/${size}`) {
      await range.body?.cancel();
      throw new Error(`${name}: invalid HTTP Range response`);
    }
    const rangeBody = await readBytes(range, rangeSize);
    if (rangeBody.bytes !== rangeSize) throw new Error(`${name}: incomplete Range response`);
    const full = await readBytes(await request(name), size);
    if (full.bytes !== size) throw new Error(`${name}: incomplete full download`);
    if (!full.prefix.equals(rangeBody.prefix)) throw new Error(`${name}: Range bytes differ from full download`);
    if (reference.sha512 !== undefined && full.sha512 !== reference.sha512) throw new Error(`${name}: SHA-512 mismatch`);
    return { name, bytes: full.bytes, sha512: full.sha512, checksumVerified: reference.sha512 !== undefined, headStatus: head.status, rangeStatus: range.status, rangeBytes: rangeBody.bytes };
  }

  const response = await request(metadata);
  const cacheControl = response.headers.get("cache-control") ?? "";
  if (!/(?:^|,)\s*(?:no-cache|no-store)(?:\s*(?:,|$)|=)/i.test(cacheControl)) {
    await response.body?.cancel();
    throw new Error("Metadata requires Cache-Control: no-cache or no-store");
  }
  const body = await readBytes(response, 1024 * 1024, true);
  const info = parseUpdateInfo(body.content.toString("utf8"), metadata, new URL(metadata, base));
  const references = updateReferences(info, true);
  if (expectedVersion !== undefined && info.version !== expectedVersion) throw new Error(`Unexpected version ${info.version}; expected version ${expectedVersion}`);
  const artifacts = [];
  const blockmaps = [];
  for (const reference of references) artifacts.push(await artifact(reference));
  for (const reference of references) {
    if (reference.name.endsWith(".blockmap")) continue;
    const name = `${reference.name}.blockmap`;
    if (references.some((item) => item.name === name)) continue;
    const head = await request(name, "HEAD", {}, true);
    if (head.status === 404) {
      blockmaps.push({ name, available: false, status: 404 });
    } else {
      blockmaps.push({ ...await artifact({ name }, head, 64 * 1024 ** 2), available: true });
    }
  }
  return { url: base.replace(/\/$/, ""), version: info.version, metadata: { name: metadata, cacheControl, bytes: body.bytes }, artifacts, blockmaps, verifiedAt: new Date().toISOString() };
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  try {
    const { values } = parseArgs({ options: { url: { type: "string" }, metadata: { type: "string", default: "latest.yml" }, "expected-version": { type: "string" }, help: { type: "boolean", short: "h" } } });
    if (values.help) {
      console.log("Usage: node apps/desktop/scripts/verify-updates.mjs --url HTTP_DIRECTORY [--metadata latest.yml] [--expected-version VERSION]\nStreams full downloads and verifies HEAD, Range, size and SHA-512. Does not install updates. Requires repository dependencies; redirects are rejected.");
    } else {
      if (!values.url) throw new Error("--url is required");
      console.log(JSON.stringify(await verifyUpdates({ url: values.url, metadata: values.metadata, expectedVersion: values["expected-version"] }), null, 2));
    }
  } catch (error) {
    console.error(`Desktop update verification: ${error.message}`);
    process.exitCode = 1;
  }
}
