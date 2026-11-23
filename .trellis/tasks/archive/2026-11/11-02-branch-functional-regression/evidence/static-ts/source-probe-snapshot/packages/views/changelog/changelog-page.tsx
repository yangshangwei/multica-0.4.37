"use client";

import { useCallback, useEffect, useMemo, useRef } from "react";
import { useQuery } from "@tanstack/react-query";
import { CircleAlert, FileText, RefreshCw, ServerOff } from "lucide-react";
import { ApiError } from "@multica/core/api";
import {
  changelogOptions,
  latestStableRelease,
  releaseGroups,
  selectRelease,
  type ChangelogRelease,
} from "@multica/core/changelog";
import { useWorkspacePaths } from "@multica/core/paths";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { cn } from "@multica/ui/lib/utils";
import { useLocale, useT } from "../i18n";
import { useAppForeground } from "../common/use-app-foreground";
import { CollectionPageState } from "../layout/collection-page";
import { PAGE_GUTTER, PageHeader } from "../layout/page-header";
import { AppLink, resolveClickIntent, useNavigation } from "../navigation";
import { useRestoredScrollRef, useRestoredViewState, useViewStateWriter } from "../platform/scroll-restoration";

const RELEASE_SELECTION_STATE = "changelog-selection";

export function ChangelogPage({ desktopVersion }: { desktopVersion?: string | null }) {
  const { t } = useT("changelog");
  const locale = useLocale();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const active = navigation.pathname.replace(/\/$/, "") === paths.changelog();
  const foreground = useAppForeground();
  const query = useQuery(changelogOptions(active));
  const { refetch } = query;
  const wasForeground = useRef(foreground);
  const feed = query.data;
  const groups = useMemo(() => releaseGroups(feed?.releases ?? []), [feed?.releases]);
  const latest = latestStableRelease(feed?.releases ?? []);
  const version = navigation.searchParams.get("version");
  const selected = selectRelease(feed?.releases ?? [], navigation.hash, version);
  const selectedId = selected?.id;
  const requested = Boolean(navigation.hash || version);
  const selectionKey = navigation.hash || (version ? `version:${version}` : undefined);
  const hasFeed = feed !== undefined;
  const consumedSelection = useRef(useRestoredViewState(RELEASE_SELECTION_STATE));
  const writeViewState = useViewStateWriter();
  const body = useRef<HTMLElement>(null);
  const restoreScroll = useRestoredScrollRef("changelog");
  const bodyRef = useCallback((element: HTMLElement | null) => {
    body.current = element;
    restoreScroll(element);
  }, [restoreScroll]);
  const labels = useReleaseLabels();
  const jumpToRelease = useCallback((id: string, requestKey: string) => {
    const container = body.current;
    const article = Array.from(container?.querySelectorAll("article") ?? []).find((entry) => entry.id === id);
    const heading = article?.querySelector<HTMLElement>("h2");
    if (!container || !heading) return;
    // Native scrollIntoView can move overflow-hidden desktop shell ancestors.
    // Honor the heading's existing scroll margin within this reader only.
    const margin = Number.parseFloat(getComputedStyle(heading).scrollMarginTop) || 0;
    container.scrollTop = Math.max(0, container.scrollTop + heading.getBoundingClientRect().top - container.getBoundingClientRect().top - margin);
    heading.focus({ preventScroll: true });
    consumedSelection.current = requestKey;
    writeViewState(RELEASE_SELECTION_STATE, requestKey);
  }, [writeViewState]);

  // Query already handles document visibility. OS focus is a separate signal:
  // returning to a visible window refreshes immediately, while a visible but
  // unfocused window still receives the promised periodic updates.
  useEffect(() => {
    const returned = foreground && !wasForeground.current;
    wasForeground.current = foreground;
    if (returned && active) void refetch({ cancelRefetch: false });
  }, [foreground, active, refetch]);

  // A retained deep link has already landed before a tab remount. Its saved
  // reading position wins; a new request or explicit link activation jumps.
  useEffect(() => {
    if (!active) return;
    if (!selectionKey || (hasFeed && !selectedId)) {
      if (consumedSelection.current !== undefined) {
        consumedSelection.current = undefined;
        writeViewState(RELEASE_SELECTION_STATE, undefined);
      }
      return;
    }
    if (!selectedId || consumedSelection.current === selectionKey) return;
    jumpToRelease(selectedId, selectionKey);
  }, [selectedId, selectionKey, hasFeed, active, jumpToRelease, writeViewState]);

  const unavailable = query.error instanceof ApiError && query.error.status === 404;
  const retry = (
    <Button variant="outline" size="sm" disabled={query.isFetching} onClick={() => void query.refetch()}>
      {t(($) => $.error.retry)}
    </Button>
  );

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <PageHeader>
        <div className="flex min-w-0 flex-1 items-center gap-2">
          <FileText aria-hidden="true" className="size-4 shrink-0 text-muted-foreground" />
          <h1 className="truncate text-body font-medium">{t(($) => $.title)}</h1>
        </div>
        <Button variant="ghost" size="sm" disabled={query.isFetching} onClick={() => void query.refetch()}>
          <RefreshCw aria-hidden="true" className={cn("size-3.5", query.isFetching && "motion-safe:animate-spin")} />
          {query.isFetching ? t(($) => $.refreshing) : t(($) => $.refresh)}
        </Button>
      </PageHeader>

      {!feed && query.isPending ? (
        <div role="status" aria-label={t(($) => $.loading)} className={cn("mx-auto w-full max-w-4xl space-y-5 py-10", PAGE_GUTTER)}>
          <Skeleton className="h-7 w-2/3" />
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-5 w-1/3" />
          <Skeleton className="h-48 w-full" />
        </div>
      ) : !feed ? (
        <CollectionPageState
          icon={unavailable ? ServerOff : CircleAlert}
          title={unavailable ? t(($) => $.unavailable.title) : t(($) => $.error.title)}
          description={unavailable ? t(($) => $.unavailable.description) : t(($) => $.error.description)}
          role={unavailable ? "status" : "alert"}
          actions={retry}
        />
      ) : (
        <main
          ref={bodyRef}
          data-tab-scroll-root="changelog"
          className="min-h-0 min-w-0 flex-1 overflow-y-auto"
        >
          <div className={cn("mx-auto w-full max-w-5xl pb-20 pt-7 md:pt-10", PAGE_GUTTER)}>
            <div className="mb-10 space-y-5 md:ml-52">
              <p className="text-title font-medium leading-snug text-balance">{t(($) => $.description)}</p>
              <dl className="flex flex-wrap gap-x-8 gap-y-3 text-caption">
                {desktopVersion !== undefined ? (
                  <div className="min-w-0">
                    <dt className="text-muted-foreground">{t(($) => $.versions.desktop)}</dt>
                    <dd className="mt-1 break-words font-medium">{desktopVersion || t(($) => $.versions.unknown)}</dd>
                  </div>
                ) : null}
                <div className="min-w-0">
                  <dt className="text-muted-foreground">{t(($) => $.versions.server)}</dt>
                  <dd className="mt-1 break-words font-medium">{feed.serverVersion || t(($) => $.versions.unknown)}</dd>
                </div>
                <div className="min-w-0">
                  <dt className="text-muted-foreground">{t(($) => $.versions.latest)}</dt>
                  <dd className="mt-1 break-words font-medium">{latest?.version ?? t(($) => $.versions.none)}</dd>
                </div>
              </dl>
              {query.isError ? <Notice>{t(($) => $.refresh_error)}</Notice> : null}
              {feed.isStale === true ? <Notice>{t(($) => $.stale_warning)}</Notice> : null}
              {requested && !selected ? (
                <div role="status" className="space-y-1 rounded-lg bg-muted p-3 text-body">
                  <p className="font-medium">{t(($) => $.missing.title)}</p>
                  <p className="text-muted-foreground">{t(($) => $.missing.description)}</p>
                  <AppLink href={paths.changelog()} className="inline-block pt-1 font-medium underline underline-offset-4">
                    {t(($) => $.all_releases)}
                  </AppLink>
                </div>
              ) : null}
            </div>

            {feed.releases.length === 0 ? (
              <CollectionPageState icon={FileText} title={t(($) => $.empty.title)} description={t(($) => $.empty.description)} role="status" />
            ) : (
              <div className="min-w-0 md:flex md:items-start md:gap-12">
                <nav
                  aria-label={t(($) => $.navigation)}
                  className="mb-8 flex gap-6 overflow-x-auto pb-3 md:sticky md:top-6 md:mb-0 md:block md:max-h-[calc(100vh-12rem)] md:w-40 md:shrink-0 md:space-y-7 md:overflow-y-auto md:pb-0"
                >
                  {groups.map((group) => (
                    <div key={group.key} className="min-w-32 shrink-0">
                      <p className="mb-2 text-caption font-medium text-muted-foreground">
                        {group.key === "unreleased" ? t(($) => $.status.unreleased) : group.month ? new Intl.DateTimeFormat(locale, { year: "numeric", month: "long", timeZone: "UTC" }).format(new Date(`${group.month}-01T00:00:00Z`)) : t(($) => $.undated)}
                      </p>
                      <ul className="space-y-1">
                        {group.releases.map((release) => (
                          <li key={release.id}>
                            <AppLink
                              href={paths.changelog({ releaseId: release.id })}
                              onClick={(event) => {
                                if (event.defaultPrevented || event.button !== 0 || resolveClickIntent(event) !== "push" || (event.shiftKey && !navigation.openInNewTab)) return;
                                jumpToRelease(release.id, `#${encodeURIComponent(release.id)}`);
                              }}
                              aria-current={selectedId === release.id ? "location" : undefined}
                              className={cn(
                                "block rounded-md px-2 py-1.5 text-body transition-colors hover:bg-accent focus-visible:outline-2 focus-visible:outline-ring",
                                selectedId === release.id ? "bg-accent font-semibold text-accent-foreground" : "text-muted-foreground hover:text-foreground",
                              )}
                            >
                              <span className="block break-words">{release.status === "unreleased" ? t(($) => $.status.unreleased) : release.version}</span>
                              <span className="block text-caption">{labels.source(release.source)}</span>
                            </AppLink>
                          </li>
                        ))}
                      </ul>
                    </div>
                  ))}
                </nav>

                <div className="min-w-0 flex-1 space-y-16">
                  {groups.flatMap((group) => group.releases).map((release) => (
                    <ReleaseEntry key={release.id} release={release} latest={latest?.id === release.id} selected={selectedId === release.id} />
                  ))}
                </div>
              </div>
            )}
          </div>
        </main>
      )}
    </div>
  );
}

function Notice({ children }: { children: React.ReactNode }) {
  return (
    <div role="status" className="flex items-start gap-2 rounded-lg bg-muted p-3 text-body">
      <CircleAlert aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
      <p>{children}</p>
    </div>
  );
}

function useReleaseLabels() {
  const { t } = useT("changelog");
  return {
    source(source: string) {
      switch (source) {
        case "fork": return t(($) => $.source.fork);
        case "upstream": return t(($) => $.source.upstream);
        default: return t(($) => $.source.unknown);
      }
    },
    status(status: string) {
      switch (status) {
        case "published": return t(($) => $.status.published);
        case "prerelease": return t(($) => $.status.prerelease);
        case "unreleased": return t(($) => $.status.unreleased);
        default: return t(($) => $.status.unknown);
      }
    },
    category(category: string) {
      switch (category) {
        case "features": return t(($) => $.category.features);
        case "improvements": return t(($) => $.category.improvements);
        case "fixes": return t(($) => $.category.fixes);
        default: return t(($) => $.category.other);
      }
    },
  };
}

function ReleaseEntry({ release, latest, selected }: { release: ChangelogRelease; latest: boolean; selected: boolean }) {
  const { t } = useT("changelog");
  const locale = useLocale();
  const labels = useReleaseLabels();
  return (
    <article id={release.id} data-selected-release={selected} className="max-w-[72ch] break-words">
      <div className="mb-3 flex flex-wrap items-center gap-x-3 gap-y-2 text-caption">
        {release.status !== "unreleased" ? <span className="font-semibold">{release.version}</span> : null}
        {release.publishedAt ? (
          <time dateTime={release.publishedAt} className="text-muted-foreground">
            {new Intl.DateTimeFormat(locale, { dateStyle: "long", timeZone: "UTC" }).format(new Date(release.publishedAt))}
          </time>
        ) : null}
        <Badge variant="secondary">{labels.status(release.status)}</Badge>
        {latest ? <Badge variant="default">{t(($) => $.status.latest)}</Badge> : null}
        <span className="text-muted-foreground">{labels.source(release.source)}</span>
      </div>
      <h2 tabIndex={-1} className="scroll-mt-7 text-title font-semibold leading-snug text-balance focus-visible:rounded-sm focus-visible:outline-2 focus-visible:outline-ring">
        {release.title}
      </h2>
      {release.status === "unreleased" || release.source === "upstream" ? (
        <p className="mt-3 text-caption leading-relaxed text-muted-foreground">
          {release.status === "unreleased" ? t(($) => $.preview_notice) : t(($) => $.upstream_notice)}
        </p>
      ) : null}
      {release.sections.length ? release.sections.map((section, index) => (
        <section key={`${section.category}-${index}`} className="mt-7">
          <h3 className="text-body font-semibold">{labels.category(section.category)}</h3>
          <ul className="mt-3 list-disc space-y-2.5 pl-5 text-body leading-relaxed marker:text-muted-foreground">
            {section.items.map((item, itemIndex) => <li key={itemIndex} className="pl-1 [overflow-wrap:anywhere]">{item.text}</li>)}
          </ul>
        </section>
      )) : <p className="mt-5 text-body text-muted-foreground">{t(($) => $.no_details)}</p>}
    </article>
  );
}
