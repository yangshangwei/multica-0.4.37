import { z } from "zod";
import type { DocsManifest, DocsPage } from "./types";

/**
 * Zod schemas for the in-app documentation endpoints.
 *
 * Lenient in the way CLAUDE.md's API compatibility rules ask for: every field
 * has a default so an installed desktop client talking to a newer or older
 * backend degrades a field at a time instead of white-screening. `.loose()`
 * keeps unknown fields rather than stripping them, so a field this client does
 * not know about is not a parse failure.
 *
 * `depth` is a number rather than a 2|3 union on purpose — a backend that starts
 * collecting h4 should widen the outline, not fail the page.
 */
export const DocsNavItemSchema = z
  .object({
    slug: z.string().default(""),
    title: z.string().default(""),
    description: z.string().optional(),
  })
  .loose();

export const DocsGroupSchema = z
  .object({
    label: z
      .string()
      .nullable()
      .optional()
      .transform((v) => v ?? null),
    items: z.array(DocsNavItemSchema).default([]),
  })
  .loose();

export const DocsManifestSchema = z
  .object({
    title: z.string().default(""),
    groups: z.array(DocsGroupSchema).default([]),
    assets: z.array(z.string()).default([]),
    serverVersion: z.string().default(""),
  })
  .loose();

export const DocsTocItemSchema = z
  .object({
    depth: z.number().default(2),
    title: z.string().default(""),
    id: z.string().default(""),
  })
  .loose();

export const DocsPageSchema = z
  .object({
    slug: z.string().default(""),
    title: z.string().default(""),
    description: z.string().default(""),
    body: z.string().default(""),
    toc: z.array(DocsTocItemSchema).default([]),
  })
  .loose();

/**
 * Fallbacks for `parseWithFallback`. An empty manifest renders the reader's
 * error state rather than an empty sidebar that looks like "no documentation
 * exists" — `groups.length === 0` is what the UI checks.
 */
export const EMPTY_DOCS_MANIFEST: DocsManifest = {
  title: "",
  groups: [],
  assets: [],
  serverVersion: "",
};

export const EMPTY_DOCS_PAGE: DocsPage = {
  slug: "",
  title: "",
  description: "",
  body: "",
  toc: [],
};
