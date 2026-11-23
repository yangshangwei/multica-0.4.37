#!/usr/bin/env node
// Generates the Chinese in-app docs bundle embedded by the Go server.
// Source stays in apps/docs; generated pages and assets are committed so CI can
// prove the binary content has not drifted from that source.

import {
  copyFileSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { dirname, relative, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";
import { jsxToDirective } from "../apps/docs/lib/docs-bundle/jsx-to-directive.mjs";
import { buildNav, extractToc, parseFrontmatter } from "../apps/docs/lib/docs-bundle/parse-page.mjs";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const defaultDocsRoot = resolve(repoRoot, "apps/docs/content/docs");
const defaultAssetsRoot = resolve(repoRoot, "apps/docs/public/images/docs");
const defaultOutputRoot = resolve(repoRoot, "server/internal/docs/content");

export function generateDocsBundle({
  docsRoot = defaultDocsRoot,
  assetsRoot = defaultAssetsRoot,
  outputRoot = defaultOutputRoot,
} = {}) {
  const pages = loadPages(docsRoot);
  const rootMeta = readJson(resolve(docsRoot, "meta.zh.json"));
  const directoryMetas = loadDirectoryMetas(docsRoot);
  const nav = buildNav(rootMeta, pages, directoryMetas);
  const assetFiles = listFiles(assetsRoot);
  const assets = assetFiles.map((path) => `images/docs/${path}`);

  rmSync(outputRoot, { recursive: true, force: true });
  const pagesRoot = resolve(outputRoot, "pages");
  for (const [slug, page] of pages) {
    writeJson(resolve(pagesRoot, `${slug}.json`), { slug, ...page });
  }

  for (const asset of assetFiles) {
    const target = resolve(outputRoot, "assets/images/docs", asset);
    mkdirSync(dirname(target), { recursive: true });
    copyFileSync(resolve(assetsRoot, asset), target);
  }

  writeJson(resolve(outputRoot, "manifest.json"), { ...nav, assets });
  return { pageCount: pages.size, assetCount: assets.length };
}

function loadPages(docsRoot) {
  const pages = new Map();
  for (const relativePath of listFiles(docsRoot).filter((path) => path.endsWith(".zh.mdx"))) {
    const slug = relativePath.slice(0, -".zh.mdx".length);
    const source = readFileSync(resolve(docsRoot, relativePath), "utf8");
    const { title, description, body: mdxBody } = parseFrontmatter(source);
    const body = jsxToDirective(mdxBody);
    pages.set(slug, {
      title,
      ...(description ? { description } : {}),
      body,
      toc: extractToc(body),
    });
  }
  if (pages.size === 0) throw new Error(`No *.zh.mdx pages found under ${docsRoot}`);
  return pages;
}

function loadDirectoryMetas(docsRoot) {
  const metas = new Map();
  for (const relativePath of listFiles(docsRoot).filter((path) => path !== "meta.zh.json" && path.endsWith("/meta.zh.json"))) {
    const directory = relativePath.slice(0, -"/meta.zh.json".length);
    metas.set(directory, readJson(resolve(docsRoot, relativePath)));
  }
  return metas;
}

function listFiles(root) {
  const files = [];
  const visit = (directory) => {
    for (const entry of readdirSync(directory, { withFileTypes: true }).sort((a, b) =>
      a.name.localeCompare(b.name, "en"),
    )) {
      const path = resolve(directory, entry.name);
      if (entry.isDirectory()) visit(path);
      else if (entry.isFile()) files.push(relative(root, path).split(sep).join("/"));
    }
  };
  visit(root);
  return files;
}

function readJson(path) {
  try {
    return JSON.parse(readFileSync(path, "utf8"));
  } catch (error) {
    throw new Error(`Cannot read docs metadata ${path}: ${error.message}`, { cause: error });
  }
}

function writeJson(path, value) {
  mkdirSync(dirname(path), { recursive: true });
  writeFileSync(path, `${JSON.stringify(value, null, 2)}\n`);
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const { pageCount, assetCount } = generateDocsBundle();
  console.log(`Generated ${pageCount} docs pages and ${assetCount} assets.`);
}
