/**
 * Shapes of the in-app documentation bundle the API server embeds.
 *
 * The bundle is generated from `apps/docs/content/docs/*.zh.mdx` by
 * `scripts/generate-docs-bundle.mjs` and served by `server/internal/handler/docs.go`.
 * These types describe what those endpoints return, not the MDX source.
 */

/** One page as it appears in the navigation tree. */
export interface DocsNavItem {
  slug: string;
  title: string;
  description?: string;
}

/**
 * A labelled section of the navigation tree. `label` is null for pages listed
 * before the first separator in the source meta file — a leading group with no
 * heading, which the sidebar renders without a section title.
 */
export interface DocsGroup {
  label: string | null;
  items: DocsNavItem[];
}

/**
 * The navigation manifest. It is also the allowlist the server validates
 * against: a slug or asset path absent from it does not resolve.
 */
export interface DocsManifest {
  title: string;
  groups: DocsGroup[];
  /** Asset paths this build published, e.g. `images/docs/agents.webp`. */
  assets: string[];
  /**
   * Build version of the server that served this manifest, so the reader can
   * say which deployment's documentation is on screen. Empty on unstamped dev
   * builds and on the managed cloud.
   */
  serverVersion: string;
}

/** One heading in a page's table of contents. */
export interface DocsTocItem {
  /** 2 or 3 — h1 is the page title and is not collected. */
  depth: number;
  title: string;
  /**
   * github-slugger id, matching what the public docs site generates so a
   * product deep link lands on the same heading in both places. Raw Unicode:
   * percent-encode before putting it in a URL.
   */
  id: string;
}

/** One documentation page: markdown body plus its heading outline. */
export interface DocsPage {
  slug: string;
  title: string;
  description: string;
  /**
   * Markdown with remark container/leaf directives where the MDX source had
   * JSX components (`:::warning`, `::video-embed{...}`). Not HTML, and not MDX.
   */
  body: string;
  toc: DocsTocItem[];
}
