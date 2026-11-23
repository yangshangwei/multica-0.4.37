// @vitest-environment node

import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "../api/client";
import { InvalidChangelogError, parseChangelog } from "./schema";

const release = {
  id: "fork:example/multica:v1.0.0",
  version: "v1.0.0",
  title: "A test release",
  published_at: "2026-09-01T00:00:00Z",
  source: "fork",
  status: "published",
  commit: "1".repeat(40),
  base_commit: "0".repeat(40),
  sections: [{ category: "features", items: [{ text: "Read changes locally", commit: "2".repeat(40) }] }],
};

function feed(overrides: Record<string, unknown> = {}) {
  return {
    schema_version: 1,
    generated_at: "2026-09-01T00:00:00Z",
    releases: [release],
    server_version: "0.4.37-server",
    feed_source: "file",
    is_stale: false,
    warning: null,
    ...overrides,
  };
}

afterEach(() => vi.unstubAllGlobals());

describe("changelog boundary", () => {
  it("maps the complete saved and runtime contracts to camelCase", () => {
    expect(parseChangelog(feed())).toEqual({
      schemaVersion: 1,
      generatedAt: "2026-09-01T00:00:00Z",
      releases: [{
        id: release.id,
        version: release.version,
        title: release.title,
        publishedAt: release.published_at,
        source: "fork",
        status: "published",
        commit: release.commit,
        baseCommit: release.base_commit,
        sections: [{ category: "features", items: [{ text: "Read changes locally", commit: "2".repeat(40) }] }],
      }],
      serverVersion: "0.4.37-server",
      feedSource: "file",
      isStale: false,
      warning: null,
    });
  });

  it("preserves valid empty history and defaults optional display metadata", () => {
    const parsed = parseChangelog({ schema_version: 1, generated_at: release.published_at, releases: [] });
    expect(parsed.releases).toEqual([]);
    expect(parsed.serverVersion).toBeNull();
    expect(parsed.feedSource).toBe("unknown");
  });

  it("tolerates unknown fields and string enums, retaining text as plain text", () => {
    const parsed = parseChangelog(feed({
      future_field: { nested: true },
      feed_source: "future-cache",
      releases: [{ ...release, id: "new:future:v1", source: "future", status: "retracted", sections: [{ category: "security", items: [{ text: "<script>alert(1)</script> **text**" }] }] }],
    }));
    expect(parsed.releases[0]?.status).toBe("retracted");
    expect(parsed.releases[0]?.source).toBe("future");
    expect(parsed.releases[0]?.sections[0]?.category).toBe("security");
    expect(parsed.releases[0]?.sections[0]?.items[0]?.text).toBe("<script>alert(1)</script> **text**");
  });

  it.each([
    null,
    {},
    feed({ schema_version: 2 }),
    feed({ generated_at: "yesterday" }),
    feed({ releases: "missing" }),
    feed({ releases: [{ ...release, id: "" }] }),
    feed({ releases: [{ ...release, title: " " }] }),
    feed({ releases: [{ ...release, published_at: null }] }),
    feed({ releases: [{ ...release, status: "unreleased" }] }),
    feed({ releases: [{ ...release, published_at: "2026-02-30T00:00:00Z" }] }),
    feed({ releases: [{ ...release, sections: [{ category: "features", items: [{ text: 42 }] }] }] }),
    feed({ releases: [release, release] }),
    feed({ releases: [release, { ...release, id: "fork:other/multica:v2" }] }),
    feed({ is_stale: "false" }),
  ])("rejects invalid essential data instead of replacing history with empty data: %j", (raw) => {
    expect(() => parseChangelog(raw)).toThrow(InvalidChangelogError);
  });

  it("retains server stale metadata without inventing a current version", () => {
    const parsed = parseChangelog(feed({ is_stale: true, warning: "changelog_file_invalid", server_version: null }));
    expect(parsed.isStale).toBe(true);
    expect(parsed.warning).toBe("changelog_file_invalid");
    expect(parsed.serverVersion).toBeNull();
  });

  it("requests only the configured deployment and validates the response", async () => {
    const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify(feed())));
    vi.stubGlobal("fetch", fetch);
    const client = new ApiClient("https://intranet.example.test");
    await expect(client.getChangelog()).resolves.toMatchObject({ serverVersion: "0.4.37-server" });
    expect(fetch.mock.calls[0]?.[0]).toBe("https://intranet.example.test/api/changelog");
    fetch.mockResolvedValue(new Response('{"releases":[]}'));
    await expect(client.getChangelog()).rejects.toBeInstanceOf(InvalidChangelogError);
  });

  it("keeps a missing endpoint distinguishable from invalid or empty history", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response('{"error":"not found"}', { status: 404 })));
    await expect(new ApiClient("https://intranet.example.test").getChangelog()).rejects.toMatchObject({ name: "ApiError", status: 404 });
  });
});
