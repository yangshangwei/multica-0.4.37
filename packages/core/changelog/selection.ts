import type { ChangelogRelease } from "./types";

type StableVersion = [bigint, bigint, bigint];

// Release tags are stricter than the CLI capability parser, which also accepts
// development descriptions. Match the publisher's numeric vX.Y.Z contract.
function parseStableVersion(version: string): StableVersion | null {
  const match = /^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/.exec(version);
  return match ? [BigInt(match[1]!), BigInt(match[2]!), BigInt(match[3]!)] : null;
}

function newerStableVersion(left: StableVersion, right: StableVersion): boolean {
  if (left[0] !== right[0]) return left[0] > right[0];
  if (left[1] !== right[1]) return left[1] > right[1];
  return left[2] > right[2];
}

export function sortReleases(releases: readonly ChangelogRelease[]): ChangelogRelease[] {
  return [...releases].sort((a, b) => {
    const preview = Number(b.status === "unreleased") - Number(a.status === "unreleased");
    if (preview) return preview;
    const date = (b.publishedAt ? Date.parse(b.publishedAt) : 0) - (a.publishedAt ? Date.parse(a.publishedAt) : 0);
    return date || a.id.localeCompare(b.id);
  });
}

export function latestStableRelease(releases: readonly ChangelogRelease[]): ChangelogRelease | null {
  const forkReleases = releases.filter((release) => release.source === "fork");
  const repositories = new Set(forkReleases.map((release) => release.id.split(":")[1]));
  if (repositories.size !== 1) return null;
  let latest: ChangelogRelease | null = null;
  let latestVersion: StableVersion | null = null;
  for (const release of sortReleases(forkReleases)) {
    if (release.status !== "published") continue;
    const version = parseStableVersion(release.version);
    if (version && (!latestVersion || newerStableVersion(version, latestVersion))) {
      latest = release;
      latestVersion = version;
    }
  }
  return latest;
}

export interface ReleaseGroup {
  key: string;
  month: string | null;
  releases: ChangelogRelease[];
}

export function releaseGroups(releases: readonly ChangelogRelease[]): ReleaseGroup[] {
  const groups = new Map<string, ReleaseGroup>();
  for (const release of sortReleases(releases)) {
    const month = release.publishedAt ? new Date(release.publishedAt).toISOString().slice(0, 7) : null;
    const key = release.status === "unreleased" ? "unreleased" : month ?? "undated";
    const group = groups.get(key) ?? { key, month, releases: [] };
    group.releases.push(release);
    groups.set(key, group);
  }
  return [...groups.values()];
}

/** Exact ids disambiguate upstream/fork versions; updater versions address this fork. */
export function selectRelease(
  releases: readonly ChangelogRelease[],
  hash: string,
  version: string | null,
): ChangelogRelease | null {
  if (hash) {
    try {
      const id = decodeURIComponent(hash.replace(/^#/, ""));
      return releases.find((release) => release.id === id) ?? null;
    } catch {
      return null;
    }
  }
  if (!version) return null;
  const normalized = version.replace(/^v/, "");
  return sortReleases(releases).find((release) =>
    release.source === "fork" &&
    (release.status === "published" || release.status === "prerelease") &&
    release.version.replace(/^v/, "") === normalized,
  ) ?? null;
}
