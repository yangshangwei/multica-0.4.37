"use client";

/**
 * DocsContent — the renderer for one in-app documentation page.
 *
 * Deliberately NOT `RichContent`. That component's docstring states it is the one
 * product-content renderer and that its API must stay narrow — no `surface` prop,
 * no per-surface branches. Documentation is a second content type with different
 * needs (directives, deep-linkable headings, asset URL rewriting) and no need for
 * the things RichContent exists to provide (mentions, attachments, issue
 * identifiers, streaming fences). Adding those needs to RichContent would be
 * exactly the fork its docstring forbids, so this shares the layer below instead:
 * react-markdown plus the canonical sanitize schema from `@multica/ui/markdown`.
 *
 * Three things this does that a generic markdown render would get wrong:
 *
 *  - Image sources. The markdown says `/images/docs/x.webp`, which cannot resolve
 *    in the desktop renderer (a `file://` origin). Each one is rewritten to an
 *    absolute API URL.
 *  - Internal links. `/daemon-runtimes` in the source means "the daemon-runtimes
 *    docs page", not the app route of that name, so links are resolved against
 *    the docs route and navigated in-app rather than as a page load.
 *  - Heading ids. Generated with the same slugger the bundle's table of contents
 *    used, so a product deep link such as `#custom-runtime-profiles` lands on its
 *    heading.
 */

import { useMemo } from "react";
import type { ComponentPropsWithoutRef, ReactNode } from "react";
import ReactMarkdown, { type Components } from "react-markdown";
import rehypeSanitize from "rehype-sanitize";
import remarkDirective from "remark-directive";
import remarkGfm from "remark-gfm";
import { api } from "@multica/core/api";
import { useWorkspacePaths } from "@multica/core/paths";
import { CodeBlock, InlineCode } from "@multica/ui/markdown";
import { cn } from "@multica/ui/lib/utils";
import { AppLink } from "../navigation";
import { DocsCallout } from "./docs-callout";
import { DocsCommunityLinks } from "./docs-community-links";
import { DocsVideoEmbed } from "./docs-video-embed";
import {
  CALLOUT_ELEMENT,
  COMMUNITY_LINKS_ELEMENT,
  VIDEO_EMBED_ELEMENT,
  remarkDocsDirectives,
  remarkDocsHeadingIds,
} from "./directives";
import { docsSanitizeSchema } from "./sanitize";

const REMARK_PLUGINS = [
  [remarkGfm, { singleTilde: false }] as const,
  remarkDirective,
  remarkDocsDirectives,
  remarkDocsHeadingIds,
];

const REHYPE_PLUGINS = [[rehypeSanitize, docsSanitizeSchema] as const];

/**
 * Absolute API URL for an image the markdown references by root-relative path.
 *
 * Anything that is not a root-relative path (an absolute URL, a data URI) is
 * returned untouched — the bundle contains only the former today, but a future
 * page embedding an external diagram should not be silently rewritten into a
 * 404 against our own asset endpoint.
 */
export function docsImageSrc(src: string): string {
  if (!src.startsWith("/")) return src;
  return api.docsAssetUrl(src.slice(1));
}

/**
 * Where a link in documentation markdown actually goes.
 *
 * The source is written for the public docs site, where `/agents` is a docs page
 * and the site root is the docs root. In-app the docs live under a workspace, so
 * a root-relative href is a docs slug that has to be re-pointed; an anchor stays
 * an anchor; anything with a scheme is external.
 */
export function resolveDocsHref(
  href: string,
  docsPagePath: (slug: string, anchor?: string) => string,
): { kind: "internal"; href: string } | { kind: "anchor"; href: string } | { kind: "external"; href: string } {
  if (!href) return { kind: "anchor", href: "#" };
  if (href.startsWith("#")) return { kind: "anchor", href };
  if (/^[a-z][a-z0-9+.-]*:/i.test(href)) return { kind: "external", href };
  if (!href.startsWith("/")) return { kind: "external", href };

  const [path = "", anchor] = href.split("#");
  const slug = path.replace(/^\/+/, "").replace(/\/+$/, "");
  if (!slug) return { kind: "internal", href: docsPagePath("index") };
  return { kind: "internal", href: docsPagePath(slug, anchor) };
}

function elementText(children: ReactNode): string {
  if (children === null || children === undefined || children === false) return "";
  if (typeof children === "string" || typeof children === "number") return String(children);
  if (Array.isArray(children)) return children.map(elementText).join("");
  if (typeof children === "object" && "props" in (children as { props?: unknown })) {
    const props = (children as { props?: { children?: ReactNode } }).props;
    return elementText(props?.children);
  }
  return "";
}

export function DocsContent({
  body,
  className,
}: {
  body: string;
  className?: string;
}) {
  const paths = useWorkspacePaths();

  const components = useMemo<Components>(() => {
    const docsPagePath = (slug: string, anchor?: string) =>
      paths.docsPage(slug, anchor);

    return {
      // Directive elements. The names come from ./directives and are allowed
      // through by ./sanitize; without both halves these render as nothing.
      [CALLOUT_ELEMENT]: ({ children, ...props }: ComponentPropsWithoutRef<"aside"> & { "data-tone"?: string }) => (
        <DocsCallout tone={props["data-tone"] ?? "info"}>{children}</DocsCallout>
      ),
      [VIDEO_EMBED_ELEMENT]: (props: {
        "data-provider"?: string;
        "data-id"?: string;
        "data-title"?: string;
      }) => (
        <DocsVideoEmbed
          provider={props["data-provider"]}
          id={props["data-id"] ?? ""}
          title={props["data-title"]}
        />
      ),
      [COMMUNITY_LINKS_ELEMENT]: (props: {
        "data-discord-description"?: string;
        "data-github-description"?: string;
        "data-x-description"?: string;
      }) => (
        <DocsCommunityLinks
          discordDescription={props["data-discord-description"]}
          githubDescription={props["data-github-description"]}
          xDescription={props["data-x-description"]}
        />
      ),

      a: ({ href, children, ...props }) => {
        const resolved = resolveDocsHref(typeof href === "string" ? href : "", docsPagePath);
        const linkClass =
          "font-medium text-foreground underline decoration-muted-foreground/40 underline-offset-4 transition-colors hover:decoration-foreground";
        if (resolved.kind === "internal") {
          // AppLink so a modifier-click opens a desktop tab and a plain click
          // is an in-app navigation rather than a document load.
          return (
            <AppLink href={resolved.href} className={linkClass}>
              {children}
            </AppLink>
          );
        }
        if (resolved.kind === "anchor") {
          return (
            <a href={resolved.href} className={linkClass} {...props}>
              {children}
            </a>
          );
        }
        return (
          <a
            href={resolved.href}
            target="_blank"
            rel="noopener noreferrer"
            className={linkClass}
            {...props}
          >
            {children}
          </a>
        );
      },

      img: ({ src, alt }) => (
        // No width/height: the bundle does not carry intrinsic dimensions, and
        // guessing them would be worse than the reflow. A broken asset shows its
        // alt text, which is why alt is never dropped.
        <img
          src={docsImageSrc(typeof src === "string" ? src : "")}
          alt={alt ?? ""}
          loading="lazy"
          className="my-4 max-w-full rounded-lg border border-border"
        />
      ),

      // Fenced code arrives as pre > code; the fence is rendered by CodeBlock
      // (shiki + copy button) and `pre` only unwraps it. Inline code keeps the
      // primitive's own treatment.
      pre: ({ children }) => {
        const child = Array.isArray(children) ? children[0] : children;
        const props =
          child && typeof child === "object" && "props" in child
            ? (child as { props: { className?: string; children?: ReactNode } }).props
            : null;
        if (!props) return <>{children}</>;
        const language = /language-([\w-]+)/.exec(props.className ?? "")?.[1] ?? "text";
        return (
          <div className="my-4">
            <CodeBlock code={elementText(props.children).replace(/\n$/, "")} language={language} mode="full" />
          </div>
        );
      },
      code: ({ children, className: codeClass }) => {
        if (codeClass?.includes("language-")) {
          // Reached only when a fence is not wrapped in <pre>; render as text
          // rather than losing the content.
          return <>{children}</>;
        }
        return <InlineCode>{elementText(children)}</InlineCode>;
      },

      h1: ({ children, ...props }) => (
        <h1 className="mt-8 mb-3 text-title font-semibold tracking-tight" {...props}>
          {children}
        </h1>
      ),
      h2: ({ children, ...props }) => (
        // scroll-mt so a heading targeted by an anchor clears the sticky header
        // instead of hiding beneath it.
        <h2 className="mt-8 mb-3 scroll-mt-20 text-body font-semibold tracking-tight" {...props}>
          {children}
        </h2>
      ),
      h3: ({ children, ...props }) => (
        <h3 className="mt-6 mb-2 scroll-mt-20 text-body font-medium" {...props}>
          {children}
        </h3>
      ),
      p: ({ children }) => <p className="my-3 text-body leading-7">{children}</p>,
      ul: ({ children }) => <ul className="my-3 list-disc space-y-1.5 pl-5 text-body leading-7">{children}</ul>,
      ol: ({ children }) => <ol className="my-3 list-decimal space-y-1.5 pl-5 text-body leading-7">{children}</ol>,
      li: ({ children }) => <li className="pl-0.5">{children}</li>,
      blockquote: ({ children }) => (
        <blockquote className="my-4 border-l-2 border-border pl-4 text-body text-muted-foreground">
          {children}
        </blockquote>
      ),
      hr: () => <hr className="my-8 border-border" />,
      strong: ({ children }) => <strong className="font-semibold">{children}</strong>,
      // Tables are 33 of the bundle's blocks; the wrapper is what keeps a wide
      // one from widening the whole page on a narrow window.
      table: ({ children }) => (
        <div className="my-4 w-full overflow-x-auto rounded-lg border border-border">
          <table className="w-full border-collapse text-body">{children}</table>
        </div>
      ),
      thead: ({ children }) => <thead className="bg-muted/40">{children}</thead>,
      th: ({ children }) => (
        <th className="border-b border-border px-3 py-2 text-left font-medium">{children}</th>
      ),
      td: ({ children }) => (
        <td className="border-b border-border px-3 py-2 align-top last:border-b-0">{children}</td>
      ),
    } as Components;
  }, [paths]);

  return (
    <div className={cn("min-w-0", className)}>
      <ReactMarkdown
        remarkPlugins={REMARK_PLUGINS as never}
        rehypePlugins={REHYPE_PLUGINS as never}
        components={components}
      >
        {body}
      </ReactMarkdown>
    </div>
  );
}
