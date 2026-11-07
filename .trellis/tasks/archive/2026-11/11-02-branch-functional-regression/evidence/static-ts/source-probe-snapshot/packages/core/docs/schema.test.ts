// @vitest-environment node
//
// Canonical layer for the docs response contract: what survives a drifted
// backend, and what an in-app asset URL looks like. The views suite renders the
// happy path and points here rather than re-running this matrix through a DOM
// mount (CLAUDE.md, Testing).
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "../api/client";
import { EMPTY_DOCS_MANIFEST, EMPTY_DOCS_PAGE } from "./schema";

const API_ORIGIN = "https://api.example.test";

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function stubFetch(body: unknown, status = 200) {
  const mock = vi.fn().mockResolvedValue(jsonResponse(body, status));
  vi.stubGlobal("fetch", mock);
  return mock;
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("getDocsManifest", () => {
  it("parses a well-formed manifest", async () => {
    stubFetch({
      title: "Multica 文档",
      groups: [
        {
          label: "开始使用",
          items: [{ slug: "index", title: "欢迎", description: "起点" }],
        },
      ],
      assets: ["images/docs/agents.webp"],
      serverVersion: "0.4.42",
    });

    const manifest = await new ApiClient(API_ORIGIN).getDocsManifest();
    expect(manifest.title).toBe("Multica 文档");
    expect(manifest.serverVersion).toBe("0.4.42");
    expect(manifest.groups[0]?.items[0]?.slug).toBe("index");
  });

  // A leading group carries no heading. null must survive parsing rather than
  // becoming "", which the sidebar would render as an empty section title.
  it("keeps a null group label", async () => {
    stubFetch({ title: "文档", groups: [{ label: null, items: [] }] });
    const manifest = await new ApiClient(API_ORIGIN).getDocsManifest();
    expect(manifest.groups[0]?.label).toBeNull();
  });

  // An older backend predates serverVersion; a missing one must not fail the
  // whole manifest, because the nav tree is still usable without it.
  it("defaults a missing serverVersion and assets to empty", async () => {
    stubFetch({ title: "文档", groups: [{ label: "组", items: [] }] });
    const manifest = await new ApiClient(API_ORIGIN).getDocsManifest();
    expect(manifest.serverVersion).toBe("");
    expect(manifest.assets).toEqual([]);
  });

  it("keeps unknown fields instead of failing on them", async () => {
    stubFetch({ title: "文档", groups: [], future_field: { nested: true } });
    await expect(new ApiClient(API_ORIGIN).getDocsManifest()).resolves.toMatchObject({
      title: "文档",
    });
  });

  it("falls back to an empty manifest when groups is the wrong type", async () => {
    stubFetch({ title: "文档", groups: "nope" });
    await expect(new ApiClient(API_ORIGIN).getDocsManifest()).resolves.toEqual(
      EMPTY_DOCS_MANIFEST,
    );
  });

  // The reader distinguishes "no docs on this server" from "request failed" by
  // the thrown ApiError, so a 404 must not be smoothed into an empty manifest.
  it("throws when the deployment has no docs route", async () => {
    stubFetch({ error: "not found" }, 404);
    await expect(new ApiClient(API_ORIGIN).getDocsManifest()).rejects.toThrow();
  });
});

describe("getDocsPage", () => {
  it("parses a page with its heading outline", async () => {
    stubFetch({
      slug: "agents",
      title: "智能体",
      description: "怎么用",
      body: ":::warning\n见 [运行时](/daemon-runtimes)\n:::",
      toc: [{ depth: 2, title: "概览", id: "概览" }],
    });

    const page = await new ApiClient(API_ORIGIN).getDocsPage("agents");
    expect(page.slug).toBe("agents");
    expect(page.body).toContain(":::warning");
    expect(page.toc).toEqual([{ depth: 2, title: "概览", id: "概览" }]);
  });

  it("percent-encodes the slug it asks for", async () => {
    const mock = stubFetch({ slug: "developers/contributing", title: "参与贡献" });
    await new ApiClient(API_ORIGIN).getDocsPage("developers/contributing");
    expect(mock.mock.calls[0]?.[0]).toBe(
      `${API_ORIGIN}/api/docs/page?slug=developers%2Fcontributing`,
    );
  });

  // Body is the one field whose absence is visible as a blank page rather than a
  // degraded one, so it defaults instead of failing the parse.
  it("degrades a missing body to empty rather than failing", async () => {
    stubFetch({ slug: "agents", title: "智能体" });
    const page = await new ApiClient(API_ORIGIN).getDocsPage("agents");
    expect(page.title).toBe("智能体");
    expect(page.body).toBe("");
  });

  it("falls back to an empty page when body is the wrong type", async () => {
    stubFetch({ slug: "agents", title: "智能体", body: { markdown: "nope" } });
    await expect(new ApiClient(API_ORIGIN).getDocsPage("agents")).resolves.toEqual(
      EMPTY_DOCS_PAGE,
    );
  });

  // A backend that starts collecting h4 should widen the outline, not blank the
  // page — depth is a number, not a 2|3 union.
  it("accepts a heading depth this client did not expect", async () => {
    stubFetch({
      slug: "agents",
      title: "智能体",
      body: "正文",
      toc: [{ depth: 4, title: "细节", id: "细节" }],
    });
    const page = await new ApiClient(API_ORIGIN).getDocsPage("agents");
    expect(page.toc[0]?.depth).toBe(4);
  });
});

describe("docsAssetUrl", () => {
  // Absolute on purpose: the desktop renderer is served from file://, where a
  // root-relative /images/docs/x.webp cannot resolve.
  it("builds an absolute URL against the API origin", () => {
    expect(new ApiClient(API_ORIGIN).docsAssetUrl("images/docs/agents.webp")).toBe(
      `${API_ORIGIN}/api/docs/assets/images/docs/agents.webp`,
    );
  });

  it("encodes each segment but keeps the separators", () => {
    expect(new ApiClient(API_ORIGIN).docsAssetUrl("images/docs/运行时 1.webp")).toBe(
      `${API_ORIGIN}/api/docs/assets/images/docs/%E8%BF%90%E8%A1%8C%E6%97%B6%201.webp`,
    );
  });
});
