"use client";

import { useId, type ReactNode } from "react";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";

interface CatalogCopy {
  title: string;
  description: string;
  loading: string;
  error: string;
  empty: string;
  retry: string;
}

/** A bounded catalog above the workspace's independently filtered instances. */
export function BuiltinTemplateCatalog({
  copy,
  loading,
  failed,
  empty,
  onRetry,
  children,
}: {
  copy: CatalogCopy;
  loading: boolean;
  failed: boolean;
  empty: boolean;
  onRetry: () => void;
  children: ReactNode;
}) {
  const headingId = useId();
  return (
    <section aria-labelledby={headingId} aria-busy={loading} className="shrink-0 border-b px-4 py-4 @container">
      <h2 id={headingId} className="text-body font-semibold">{copy.title}</h2>
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
        <ul className="mt-3 grid max-h-[min(18rem,38vh)] gap-x-8 gap-y-3 overflow-y-auto overscroll-contain pr-1 @3xl:grid-cols-2">
          {children}
        </ul>
      )}
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
