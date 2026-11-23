import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import type { ChangelogFeed } from "./types";

const text = z.string().refine((value) => value.trim().length > 0);
const timestamp = z.string().datetime({ offset: true });
const commit = z.string().regex(/^[a-f\d]{40}(?:[a-f\d]{24})?$/i);
const nullableCommit = commit.nullish().transform((value) => value ?? null);

const ReleaseSchema = z.object({
  id: text,
  version: text,
  title: text,
  published_at: timestamp.nullable(),
  status: text,
  source: text,
  commit: nullableCommit,
  base_commit: nullableCommit,
  sections: z.array(z.object({
    category: text,
    items: z.array(z.object({ text, commit: commit.optional() })),
  })),
}).superRefine((release, ctx) => {
  if ((release.status === "published" || release.status === "prerelease") && release.published_at === null) {
    ctx.addIssue({ code: "custom", message: "Published entries require a date" });
  }
  if (release.status === "unreleased" && release.published_at !== null) {
    ctx.addIssue({ code: "custom", message: "Unreleased entries cannot have a publication date" });
  }
  if (release.source === "fork" && (!/^fork:[^:/]+\/[^:]+:.+$/.test(release.id) || !release.commit || !release.base_commit)) {
    ctx.addIssue({ code: "custom", message: "Fork entries require repository identity and an immutable range" });
  }
});

/** Validate the complete history before replacing any successful query data. */
export const ChangelogFeedSchema = z.object({
  schema_version: z.literal(1),
  generated_at: timestamp,
  releases: z.array(ReleaseSchema),
  server_version: z.string().nullish().transform((value) => value || null),
  feed_source: text.default("unknown"),
  is_stale: z.boolean().default(false),
  warning: z.string().nullish().transform((value) => value ?? null),
}).superRefine((feed, ctx) => {
  if (new Set(feed.releases.map((release) => release.id)).size !== feed.releases.length) {
    ctx.addIssue({ code: "custom", message: "Release identities must be unique" });
  }
  const forks = new Set(feed.releases.filter((release) => release.source === "fork").map((release) => release.id.split(":")[1]));
  if (forks.size > 1) {
    ctx.addIssue({ code: "custom", message: "History must belong to one fork" });
  }
}).transform((feed): ChangelogFeed => ({
  schemaVersion: feed.schema_version,
  generatedAt: feed.generated_at,
  releases: feed.releases.map((release) => ({
    id: release.id,
    version: release.version,
    title: release.title,
    publishedAt: release.published_at,
    status: release.status,
    source: release.source,
    commit: release.commit,
    baseCommit: release.base_commit,
    sections: release.sections,
  })),
  serverVersion: feed.server_version,
  feedSource: feed.feed_source,
  isStale: feed.is_stale,
  warning: feed.warning,
}));

export class InvalidChangelogError extends Error {
  constructor() {
    super("The changelog response is invalid");
    this.name = "InvalidChangelogError";
  }
}

export function parseChangelog(raw: unknown): ChangelogFeed {
  const parsed = parseWithFallback<ChangelogFeed | null>(raw, ChangelogFeedSchema, null, {
    endpoint: "GET /api/changelog",
  });
  if (parsed === null) throw new InvalidChangelogError();
  return parsed;
}
