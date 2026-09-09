/**
 * remark plumbing for the in-app documentation renderer.
 *
 * The bundle generator rewrites the docs site's three JSX components into remark
 * directives (see apps/docs/lib/docs-bundle/jsx-to-directive.mjs), so the body
 * this renderer receives is plain markdown plus:
 *
 *   :::info / :::warning        container directives — Callout, body is markdown
 *   ::video-embed{...}          leaf directive — VideoEmbed
 *   ::community-links{...}      leaf directive — CommunityLinks
 *
 * remark-directive parses those into mdast nodes but has no opinion about HTML.
 * These plugins give each node an `hName`/`hProperties` so react-markdown's
 * component map can pick it up, and give every heading the same id the bundle's
 * table of contents computed — product deep links point at those ids, so the two
 * must be generated the same way or an anchor lands nowhere.
 *
 * Kept free of React so the mapping is unit-testable on its own.
 */

import GithubSlugger from "github-slugger";
import type { Root } from "mdast";

/**
 * Directive names this renderer knows, mapped to the element the component map
 * keys off. A directive that is not listed renders as its own body (container)
 * or is dropped (leaf) — see `unknownDirectiveFallback` below.
 */
const CONTAINER_DIRECTIVES = new Map([
  ["info", { tone: "info" }],
  ["warning", { tone: "warning" }],
]);

const LEAF_DIRECTIVES = new Set(["video-embed", "community-links"]);

/** Element names the component map overrides. Prefixed to avoid colliding with
 *  a real HTML tag that rehype-sanitize would treat differently. */
export const CALLOUT_ELEMENT = "docs-callout";
export const VIDEO_EMBED_ELEMENT = "docs-video-embed";
export const COMMUNITY_LINKS_ELEMENT = "docs-community-links";

const LEAF_ELEMENTS: Record<string, string> = {
  "video-embed": VIDEO_EMBED_ELEMENT,
  "community-links": COMMUNITY_LINKS_ELEMENT,
};

interface DirectiveNode {
  type: string;
  name?: string;
  attributes?: Record<string, string | null | undefined>;
  data?: {
    hName?: string;
    hProperties?: Record<string, unknown>;
  };
  children?: unknown[];
}

/**
 * Map directive nodes onto elements the component map renders.
 *
 * An unregistered directive is deliberately NOT an error here. The generator is
 * the gate that fails a build on an unknown JSX tag; by the time content reaches
 * a reader, refusing to render is strictly worse than rendering the body without
 * its wrapper. A container keeps its markdown children (so the prose survives);
 * an unknown leaf directive has no body to keep and is left as-is, which
 * react-markdown drops.
 */
export function remarkDocsDirectives() {
  return (tree: Root) => {
    visit(tree as unknown as DirectiveNode, (node) => {
      if (node.type === "containerDirective") {
        const known = CONTAINER_DIRECTIVES.get(node.name ?? "");
        if (!known) return;
        node.data = {
          ...node.data,
          hName: CALLOUT_ELEMENT,
          hProperties: { dataTone: known.tone },
        };
        return;
      }
      if (node.type === "leafDirective") {
        const name = node.name ?? "";
        if (!LEAF_DIRECTIVES.has(name)) return;
        // Directive attributes arrive as a flat string map. They are passed
        // through as data-* properties so rehype-sanitize's attribute
        // allowlist can vet them by name rather than by shape.
        const properties: Record<string, unknown> = {};
        for (const [key, value] of Object.entries(node.attributes ?? {})) {
          if (typeof value === "string") properties[dataAttrName(key)] = value;
        }
        node.data = {
          ...node.data,
          hName: LEAF_ELEMENTS[name],
          hProperties: properties,
        };
      }
    });
  };
}

/**
 * Give every h2/h3 the github-slugger id its table-of-contents entry carries.
 *
 * The bundle's `toc[].id` comes from `extractToc`, which slugs the heading's
 * plain text with a fresh GithubSlugger per page. This does the same thing on
 * the same text in the same document order, which is what makes a
 * `#custom-runtime-profiles` deep link from product code resolve. The parity
 * test in docs-anchor-parity.test.ts is what proves the two stay in step.
 */
export function remarkDocsHeadingIds() {
  return (tree: Root) => {
    const slugger = new GithubSlugger();
    visit(tree as unknown as DirectiveNode, (node) => {
      if (node.type !== "heading") return;
      const heading = node as DirectiveNode & { depth?: number };
      const text = mdastText(node);
      if (!text) return;
      const id = slugger.slug(text);
      heading.data = {
        ...heading.data,
        hProperties: { ...heading.data?.hProperties, id },
      };
    });
  };
}

/**
 * Plain text of an mdast subtree, matching what `extractToc`'s
 * `stripInlineMarkdown` produces: inline code, emphasis and link syntax
 * contribute their text, nothing contributes its markup.
 */
export function mdastText(node: unknown): string {
  const current = node as { type?: string; value?: string; children?: unknown[] };
  if (typeof current.value === "string" && current.type !== "html") {
    return current.value;
  }
  if (!Array.isArray(current.children)) return "";
  return current.children.map(mdastText).join("");
}

function dataAttrName(key: string): string {
  // hast property names are camelCase; `provider` stays `dataProvider`, which
  // serializes to data-provider.
  return `data${key.charAt(0).toUpperCase()}${key.slice(1)}`;
}

function visit(node: DirectiveNode, fn: (node: DirectiveNode) => void): void {
  fn(node);
  for (const child of node.children ?? []) {
    if (child && typeof child === "object") visit(child as DirectiveNode, fn);
  }
}
