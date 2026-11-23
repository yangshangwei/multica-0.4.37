"use client";

import { useId, type ReactNode } from "react";
import { ChevronRight } from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@multica/ui/components/ui/collapsible";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { cn } from "@multica/ui/lib/utils";

interface CatalogCopy {
  title: string;
  description: string;
  loading: string;
  error: string;
  empty: string;
  retry: string;
}

/** An opt-in catalog above the workspace's independently filtered instances. */
export function BuiltinTemplateCatalog({
  copy,
  count,
  loading,
  failed,
  empty,
  onRetry,
  listClassName,
  className,
  defaultOpen = false,
  children,
}: {
  copy: CatalogCopy;
  count: number;
  loading: boolean;
  failed: boolean;
  empty: boolean;
  onRetry: () => void;
  listClassName?: string;
  /** Overrides the default list-header shell. Callers outside a bordered list
   *  (e.g. the settings page) pass their own spacing here. */
  className?: string;
  /** Opens the catalog expanded. Defaults closed, matching the list-page uses. */
  defaultOpen?: boolean;
  children: ReactNode;
}) {
  const headingId = useId();
  const countId = useId();
  return (
    <section
      aria-labelledby={headingId}
      aria-busy={loading}
      className={cn("shrink-0 border-b px-4 py-2 @container", className)}
    >
      <Collapsible defaultOpen={defaultOpen}>
        <h2>
          <CollapsibleTrigger aria-labelledby={loading || failed ? headingId : `${headingId} ${countId}`} className="group/catalog flex min-h-9 w-full items-center gap-2 rounded-md text-left text-body font-semibold transition-colors hover:bg-accent/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
            <ChevronRight className="size-4 shrink-0 text-muted-foreground transition-transform duration-150 group-data-[panel-open]/catalog:rotate-90 motion-reduce:transition-none" aria-hidden="true" />
            <span id={headingId}>{copy.title}</span>
            {!loading && !failed && <span id={countId} className="text-caption font-normal tabular-nums text-muted-foreground">{count}</span>}
          </CollapsibleTrigger>
        </h2>
        <CollapsibleContent className="pb-2">
          <p className="mt-1 max-w-[70ch] text-caption text-muted-foreground">{copy.description}</p>
          {failed ? (
            <div role="alert" className="mt-3 flex flex-wrap items-center gap-3 text-caption text-muted-foreground">
              <p>{copy.error}</p>
              <Button type="button" size="sm" variant="outline" onClick={onRetry}>{copy.retry}</Button>
            </div>
          ) : loading ? (
            <div role="status" className="mt-4 space-y-3">
              <span className="sr-only">{copy.loading}</span>
              <Skeleton className="h-4 w-40" />
              <Skeleton className="h-4 w-full max-w-md" />
            </div>
          ) : empty ? (
            <p className="mt-3 text-caption text-muted-foreground">{copy.empty}</p>
          ) : (
            <ul className={cn("mt-3 grid max-h-[min(18rem,38vh)] gap-x-8 gap-y-3 overflow-y-auto overscroll-contain pr-1 @3xl:grid-cols-2", listClassName)}>
              {children}
            </ul>
          )}
        </CollapsibleContent>
      </Collapsible>
    </section>
  );
}

export function BuiltinTemplateRow({
  title,
  description,
  actions,
}: {
  title: string;
  description: string;
  actions: ReactNode;
}) {
  return (
    <li aria-label={title} className="min-w-0 space-y-2 py-1">
      <div className="min-w-0">
        <h3 className="break-words text-body font-medium">{title}</h3>
        <p className="mt-1 line-clamp-2 text-caption leading-5 text-muted-foreground" title={description}>{description}</p>
      </div>
      <div className="flex flex-wrap items-center gap-2">{actions}</div>
    </li>
  );
}
