// @vitest-environment node
/**
 * Parity between the deep links product code holds and the documentation that
 * actually shipped.
 *
 * A docs link is only correct if its page exists in the bundle and its anchor
 * matches a heading on that page. Nothing checked that before, and two links had
 * already rotted in the tree without anyone noticing — one of them in all four
 * locales, broken on the public docs site too:
 *
 *   - `#事件过滤` pointed at a heading that reads `过滤事件` (same words, other
 *     order). It has never resolved anywhere.
 *   - the Korean daemon-runtimes link pointed at `사용자 지정 런타임 프로필` where
 *     the heading says `사용자 지정 런타임 설정`.
 *
 * This test is the reason those cannot come back. It reads the generated bundle
 * rather than the MDX source, because the bundle is what the app serves and its
 * `toc[].id` values are the ids the renderer puts in the DOM.
 *
 * Lives here rather than beside the slugs module because it needs the filesystem:
 * `packages/core` is platform-neutral by contract, and the rest of the bundle's
 * tests are already in this directory.
 */
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import {
  DOCS_ANCHORS,
  DOCS_SLUGS,
} from "../../../../packages/core/docs/slugs";

const REPO_ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "../../../..");
const BUNDLE_ROOT = resolve(REPO_ROOT, "server/internal/docs/content");

interface BundleManifest {
  groups: Array<{ items: Array<{ slug: string }> }>;
}

interface BundlePage {
  slug: string;
  toc: Array<{ id: string; title: string }>;
}

function readManifest(): BundleManifest {
  return JSON.parse(readFileSync(resolve(BUNDLE_ROOT, "manifest.json"), "utf8"));
}

function readPage(slug: string): BundlePage {
  return JSON.parse(
    readFileSync(resolve(BUNDLE_ROOT, "pages", `${slug}.json`), "utf8"),
  );
}

/** Which page each anchor is expected to live on. */
const ANCHOR_PAGES: Record<keyof typeof DOCS_ANCHORS, string> = {
  webhookEventFilters: DOCS_SLUGS.autopilots,
  customRuntimeProfiles: DOCS_SLUGS.daemonRuntimes,
};

describe("docs deep-link parity", () => {
  const manifest = readManifest();
  const publishedSlugs = new Set(
    manifest.groups.flatMap((group) => group.items.map((item) => item.slug)),
  );

  it("every slug product code links to is published in the bundle", () => {
    const missing = Object.entries(DOCS_SLUGS)
      .filter(([, slug]) => !publishedSlugs.has(slug))
      .map(([name, slug]) => `${name} -> ${slug}`);

    expect(
      missing,
      `product code links to docs pages that this build does not publish: ${missing.join(", ")}`,
    ).toEqual([]);
  });

  it.each(Object.entries(ANCHOR_PAGES))(
    "anchor %s resolves to a real heading",
    (name, slug) => {
      const anchor = DOCS_ANCHORS[name as keyof typeof DOCS_ANCHORS];
      const page = readPage(slug);
      const ids = page.toc.map((heading) => heading.id);

      expect(
        ids,
        `${name} points at "#${anchor}" on ${slug}, whose headings are: ${page.toc
          .map((h) => `${h.title} (#${h.id})`)
          .join(", ")}`,
      ).toContain(anchor);
    },
  );

  // The bug this whole test exists for: an anchor that is a real heading's words
  // in the wrong order looks right in a code review and resolves nowhere.
  it("rejects the reversed wording that shipped before", () => {
    const autopilots = readPage(DOCS_SLUGS.autopilots);
    const ids = autopilots.toc.map((heading) => heading.id);
    expect(ids).not.toContain("事件过滤");
    expect(ids).toContain(DOCS_ANCHORS.webhookEventFilters);
  });
});
