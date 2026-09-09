"use client";

/**
 * DocsPage — the in-app documentation reader.
 *
 * One component serves both routes: `/{slug}/docs` (no page selected, shows the
 * index) and `/{slug}/docs/{...page}`. They share the nav tree, the loading and
 * failure states, and the chrome, so splitting them would mean maintaining two
 * copies of all three.
 *
 * The manifest is what makes the reader legible when things go wrong. It is a
 * property of the server build, so a deployment older than this feature answers
 * 404 on every docs route: that is "this server has no in-app docs" (an
 * explanation), not "loading failed" (a retry prompt), and the two are
 * distinguished on the error's status rather than lumped together.
 */

import { useQuery } from "@tanstack/react-query";
import { BookOpen, CircleAlert, FileQuestion, ServerOff } from "lucide-react";
import { ApiError } from "@multica/core/api";
import { docsManifestOptions, docsPageOptions } from "@multica/core/docs";
import type { DocsManifest } from "@multica/core/docs";
import { useWorkspacePaths } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { cn } from "@multica/ui/lib/utils";
import { CollectionPageState } from "../layout/collection-page";
import { PageHeader, PAGE_GUTTER } from "../layout/page-header";
import { AppLink } from "../navigation";
import { useT } from "../i18n";
import { DocsContent } from "./docs-content";

export function DocsPage({ slug }: { slug?: string }) {
  const { t } = useT("docs");
  const paths = useWorkspacePaths();
  const manifest = useQuery(docsManifestOptions());
  const page = useQuery(docsPageOptions(slug ?? ""));

  // A 404 on the manifest means the route does not exist on this build; any
  // other failure is a request that could succeed on a retry.
  const unavailable =
    manifest.error instanceof ApiError && manifest.error.status === 404;

  const title = slug ? page.data?.title : manifest.data?.title;

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <PageHeader>
        <div className="flex min-w-0 flex-1 items-center gap-2">
          <BookOpen
            aria-hidden="true"
            className="size-4 shrink-0 text-muted-foreground"
          />
          <h1 className="truncate text-body font-medium">
            {title || t(($) => $.title)}
          </h1>
        </div>
        {manifest.data?.serverVersion ? (
          <span className="hidden shrink-0 font-mono text-caption text-muted-foreground md:inline">
            {t(($) => $.server_version, { version: manifest.data.serverVersion })}
          </span>
        ) : null}
      </PageHeader>

      {unavailable ? (
        <CollectionPageState
          icon={ServerOff}
          title={t(($) => $.unavailable.title)}
          description={t(($) => $.unavailable.description)}
          role="status"
        />
      ) : manifest.isError ? (
        <CollectionPageState
          icon={CircleAlert}
          title={t(($) => $.error.title)}
          description={t(($) => $.error.description)}
          tone="destructive"
          role="alert"
          actions={
            <Button variant="outline" size="sm" onClick={() => void manifest.refetch()}>
              {t(($) => $.error.retry)}
            </Button>
          }
        />
      ) : (
        <div className="flex min-h-0 flex-1 overflow-hidden">
          <DocsNav manifest={manifest.data} activeSlug={slug} loading={manifest.isPending} />
          <main className="min-w-0 flex-1 overflow-y-auto">
            <div className={cn("mx-auto w-full max-w-3xl pb-16 pt-2", PAGE_GUTTER)}>
              {slug ? (
                page.isPending ? (
                  <DocsBodySkeleton />
                ) : page.isError ? (
                  <CollectionPageState
                    icon={FileQuestion}
                    title={t(($) => $.not_found.title)}
                    description={t(($) => $.not_found.description)}
                    role="status"
                    actions={
                      <Button variant="outline" size="sm" render={<AppLink href={paths.docs()} />}>
                        {t(($) => $.not_found.back)}
                      </Button>
                    }
                  />
                ) : (
                  <DocsContent body={page.data?.body ?? ""} />
                )
              ) : (
                <DocsIndex manifest={manifest.data} loading={manifest.isPending} />
              )}
            </div>
          </main>
        </div>
      )}
    </div>
  );
}

/**
 * The section tree. Groups come from the source meta file's `---label---`
 * separators, so their order is editorial and must be preserved as given.
 */
function DocsNav({
  manifest,
  activeSlug,
  loading,
}: {
  manifest?: DocsManifest;
  activeSlug?: string;
  loading: boolean;
}) {
  const { t } = useT("docs");
  const paths = useWorkspacePaths();

  return (
    <nav
      aria-label={t(($) => $.title)}
      className="hidden w-60 shrink-0 overflow-y-auto border-r py-3 lg:block"
    >
      {loading ? (
        <div className="space-y-2 px-3">
          {Array.from({ length: 12 }, (_, i) => (
            <Skeleton key={i} className="h-6 w-full" />
          ))}
        </div>
      ) : (
        <ul className="space-y-4 px-2">
          {manifest?.groups.map((group, groupIndex) => (
            <li key={group.label ?? `group-${groupIndex}`}>
              {group.label ? (
                <h2 className="px-2 pb-1 text-caption font-medium text-muted-foreground">
                  {group.label}
                </h2>
              ) : null}
              <ul>
                {group.items.map((item) => {
                  const active = item.slug === activeSlug;
                  return (
                    <li key={item.slug}>
                      <AppLink
                        href={paths.docsPage(item.slug)}
                        aria-current={active ? "page" : undefined}
                        // Active is carried by weight and text colour, not by
                        // background alone: hover paints a background too, so a
                        // background-only active state visually downgrades the
                        // selected row the moment the pointer crosses it.
                        className={cn(
                          "block truncate rounded-md px-2 py-1.5 text-body transition-colors hover:bg-accent",
                          active
                            ? "bg-accent font-medium text-accent-foreground"
                            : "text-muted-foreground hover:text-foreground",
                        )}
                      >
                        {item.title}
                      </AppLink>
                    </li>
                  );
                })}
              </ul>
            </li>
          ))}
        </ul>
      )}
    </nav>
  );
}

/** The landing view: every section and page, so the tree is reachable without
 *  the sidebar the narrow layout hides. */
function DocsIndex({
  manifest,
  loading,
}: {
  manifest?: DocsManifest;
  loading: boolean;
}) {
  const { t } = useT("docs");
  const paths = useWorkspacePaths();
  const isEmpty = !loading && (manifest?.groups.length ?? 0) === 0;

  if (loading) return <DocsBodySkeleton />;
  if (isEmpty) {
    return (
      <CollectionPageState
        icon={FileQuestion}
        title={t(($) => $.index.empty)}
        role="status"
      />
    );
  }

  return (
    <div className="py-4">
      <p className="text-body text-muted-foreground">{t(($) => $.index.description)}</p>
      <p className="mt-1 text-caption text-muted-foreground">
        {t(($) => $.content_language_notice)}
      </p>
      <div className="mt-8 space-y-8">
        {manifest?.groups.map((group, groupIndex) => (
          <section key={group.label ?? `group-${groupIndex}`}>
            {group.label ? (
              <h2 className="text-body font-semibold">{group.label}</h2>
            ) : null}
            <ul className="mt-3 space-y-1">
              {group.items.map((item) => (
                <li key={item.slug}>
                  <AppLink
                    href={paths.docsPage(item.slug)}
                    className="group flex flex-col gap-0.5 rounded-lg px-3 py-2 transition-colors hover:bg-accent"
                  >
                    <span className="text-body font-medium">{item.title}</span>
                    {item.description ? (
                      <span className="text-caption text-muted-foreground">
                        {item.description}
                      </span>
                    ) : null}
                  </AppLink>
                </li>
              ))}
            </ul>
          </section>
        ))}
      </div>
    </div>
  );
}

function DocsBodySkeleton() {
  return (
    <div className="space-y-3 py-6">
      <Skeleton className="h-7 w-1/2" />
      <Skeleton className="h-4 w-full" />
      <Skeleton className="h-4 w-11/12" />
      <Skeleton className="h-4 w-4/5" />
      <Skeleton className="mt-6 h-40 w-full" />
    </div>
  );
}
