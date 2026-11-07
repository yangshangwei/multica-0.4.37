"use client";

import type { ReactNode } from "react";
import { Info, TriangleAlert } from "lucide-react";
import { cn } from "@multica/ui/lib/utils";

/**
 * Callout — the in-app rendering of the docs site's `<Callout>`, which the
 * bundle generator rewrites to a `:::info` / `:::warning` container directive.
 *
 * Children are a normal markdown subtree (that is the whole reason the generator
 * emits directives instead of HTML), so links, bold runs and inline code inside
 * a callout render as markdown rather than as literal text.
 */
export function DocsCallout({
  tone,
  children,
}: {
  tone: string;
  children?: ReactNode;
}) {
  // Unknown tone degrades to info rather than throwing: the generator's tag
  // registry is the gate that fails a build, and by the time content reaches a
  // reader an unstyled-but-present callout beats a blank page.
  const warning = tone === "warning";
  const Icon = warning ? TriangleAlert : Info;
  return (
    <aside
      className={cn(
        "my-4 flex gap-2.5 rounded-lg border px-3.5 py-3",
        warning
          ? "border-warning/30 bg-warning/10"
          : "border-border bg-muted/40",
      )}
    >
      <Icon
        aria-hidden="true"
        className={cn(
          "mt-0.5 size-4 shrink-0",
          warning ? "text-warning" : "text-muted-foreground",
        )}
      />
      {/* The body owns its own block spacing; strip the outer margins the
          prose styles would otherwise add to first/last paragraphs. */}
      <div className="min-w-0 flex-1 [&>:first-child]:mt-0 [&>:last-child]:mb-0">
        {children}
      </div>
    </aside>
  );
}
