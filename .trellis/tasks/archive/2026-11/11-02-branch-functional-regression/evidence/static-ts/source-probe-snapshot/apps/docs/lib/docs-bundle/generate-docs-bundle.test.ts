// @vitest-environment node
import {
  existsSync,
  mkdtempSync,
  mkdirSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { resolve } from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { generateDocsBundle } from "../../../../scripts/generate-docs-bundle.mjs";

const cleanups: string[] = [];

afterEach(() => {
  for (const directory of cleanups.splice(0)) {
    rmSync(directory, { recursive: true, force: true });
  }
});

describe("generateDocsBundle", () => {
  it("writes transformed pages, nested navigation, and copied assets", () => {
    const root = mkdtempSync(resolve(tmpdir(), "multica-docs-bundle-"));
    cleanups.push(root);
    const docsRoot = resolve(root, "docs");
    const assetsRoot = resolve(root, "assets");
    const outputRoot = resolve(root, "output");
    mkdirSync(resolve(docsRoot, "developers"), { recursive: true });
    mkdirSync(assetsRoot, { recursive: true });

    writeFileSync(
      resolve(docsRoot, "meta.zh.json"),
      JSON.stringify({ title: "文档", pages: ["---开始---", "index", "developers"] }),
    );
    writeFileSync(
      resolve(docsRoot, "developers/meta.zh.json"),
      JSON.stringify({ title: "开发", pages: ["contributing"] }),
    );
    writeFileSync(
      resolve(docsRoot, "index.zh.mdx"),
      ["---", "title: 欢迎", "description: 起点", "---", "", "## 概览", "", '<Callout type="warn">**当心**</Callout>'].join("\n"),
    );
    writeFileSync(
      resolve(docsRoot, "developers/contributing.zh.mdx"),
      ["---", "title: 参与", "---", "", "## 准备"].join("\n"),
    );
    writeFileSync(resolve(assetsRoot, "diagram.webp"), "image-bytes");
    mkdirSync(outputRoot, { recursive: true });
    writeFileSync(resolve(outputRoot, "stale.json"), "stale");

    expect(generateDocsBundle({ docsRoot, assetsRoot, outputRoot })).toEqual({
      pageCount: 2,
      assetCount: 1,
    });

    const page = readJson(resolve(outputRoot, "pages/index.json"));
    expect(page).toMatchObject({
      slug: "index",
      title: "欢迎",
      description: "起点",
      toc: [{ depth: 2, title: "概览", id: "概览" }],
    });
    expect(page.body).toContain([":::warning", "**当心**", ":::"].join("\n"));

    const manifest = readJson(resolve(outputRoot, "manifest.json"));
    expect(manifest.groups[1]).toEqual({
      label: "开发",
      items: [{ slug: "developers/contributing", title: "参与" }],
    });
    expect(manifest.assets).toEqual(["images/docs/diagram.webp"]);
    expect(readFileSync(resolve(outputRoot, "assets/images/docs/diagram.webp"), "utf8")).toBe(
      "image-bytes",
    );
    expect(existsSync(resolve(outputRoot, "stale.json"))).toBe(false);
  });
});

function readJson(path: string): any {
  return JSON.parse(readFileSync(path, "utf8"));
}
