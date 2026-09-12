// @vitest-environment node

import { describe, expect, it } from "vitest";
import { latestStableRelease, releaseGroups, selectRelease, sortReleases } from "./selection";
import type { ChangelogRelease } from "./types";

function release(overrides: Partial<ChangelogRelease> = {}): ChangelogRelease {
  return {
    id: "fork:example/multica:v1.0.0", version: "v1.0.0", title: "Fixture",
    publishedAt: "2026-09-01T00:00:00Z", source: "fork", status: "published",
    commit: null, baseCommit: null, sections: [], ...overrides,
  };
}

describe("release identity, publication and selection", () => {
  const stable = release();
  const upstream = release({ id: "upstream:multica-ai/multica:v1.0.0", source: "upstream", publishedAt: "2026-09-10T00:00:00Z" });
  const preview = release({ id: "fork:example/multica:unreleased", status: "unreleased", publishedAt: null, version: "Unreleased" });
  const prerelease = release({ id: "fork:example/multica:v2.0.0-rc.1", version: "v2.0.0-rc.1", status: "prerelease", publishedAt: "2026-09-12T00:00:00Z" });

  it("only labels published stable releases from the same fork as latest", () => {
    expect(latestStableRelease([upstream, preview, prerelease, stable])).toBe(stable);
    expect(latestStableRelease([upstream, preview, prerelease])).toBeNull();
    expect(latestStableRelease([release({ status: "withdrawn" })])).toBeNull();
    expect(latestStableRelease([stable, release({ id: "fork:other/project:v2" })])).toBeNull();
  });

  it("does not regress latest stable when an older tag is published later, while keeping chronological history", () => {
    const newer = release({ id: "fork:example/multica:v1.2.0", version: "v1.2.0", publishedAt: "2026-09-02T00:00:00Z" });
    const retry = release({ id: "fork:example/multica:v1.1.0", version: "v1.1.0", publishedAt: "2026-09-12T00:00:00Z" });
    expect(latestStableRelease([newer, retry])).toBe(newer);
    expect(sortReleases([newer, retry])).toEqual([retry, newer]);
  });

  it.each([
    ["v1.9.0", "v1.10.0"],
    ["v0.99.99", "v1.0.0"],
    ["v1.2.9", "v1.2.10"],
    ["v1.2.9007199254740992", "v1.2.9007199254740993"],
  ])("compares numeric components: %s precedes %s", (olderVersion, newerVersion) => {
    const older = release({ id: `fork:example/multica:${olderVersion}`, version: olderVersion, publishedAt: "2026-09-12T00:00:00Z" });
    const newer = release({ id: `fork:example/multica:${newerVersion}`, version: newerVersion });
    expect(latestStableRelease([older, newer])).toBe(newer);
  });

  it.each(["v99.0.0-rc.1", "v99.0.0-dirty", "v99.0.0+build.1", "v99", "v99.0", "v01.2.3", "latest", "v99.0.0 extra"])(
    "ignores invalid or non-stable version %s even when status has drifted to published",
    (version) => {
      const invalid = release({ id: `fork:example/multica:${version}`, version, publishedAt: "2026-09-12T00:00:00Z" });
      expect(latestStableRelease([invalid, stable])).toBe(stable);
      expect(latestStableRelease([invalid])).toBeNull();
    },
  );

  it("orders undated previews separately then published dates with deterministic ties", () => {
    expect(sortReleases([stable, upstream, preview, prerelease])).toEqual([preview, prerelease, upstream, stable]);
    expect(releaseGroups([stable, upstream, preview, prerelease]).map((group) => [group.month, group.releases.length])).toEqual([[null, 1], ["2026-09", 3]]);
    const tied = release({ id: "fork:example/multica:v0" });
    expect(sortReleases([stable, tied])).toEqual(sortReleases([tied, stable]));
  });

  it("uses exact release ids in hashes and prefers fork versions for updater requests", () => {
    expect(selectRelease([upstream, stable], `#${encodeURIComponent(upstream.id)}`, null)).toBe(upstream);
    expect(selectRelease([upstream, stable], "", "1.0.0")).toBe(stable);
    expect(selectRelease([upstream], "", "1.0.0")).toBeNull();
    expect(selectRelease([stable], "#missing", null)).toBeNull();
    expect(selectRelease([stable], "#%E0%A4%A", null)).toBeNull();
    expect(selectRelease([stable], "", "999")).toBeNull();
    expect(selectRelease([preview, stable], "", null)).toBeNull();
  });
});
