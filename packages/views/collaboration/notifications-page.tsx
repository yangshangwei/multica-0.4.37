"use client";

import { useEffect, useId, useRef, type RefObject } from "react";
import { ArrowLeft, Bell, RefreshCw } from "lucide-react";
import { useInfiniteQuery, useQueryClient, type InfiniteData, type UseInfiniteQueryResult } from "@tanstack/react-query";
import type { ApiClient } from "@multica/core/api";
import { COLLABORATION_NOTIFICATION_VIEWS, collaborationNotificationsOptions, isCollaborationDenied, useCollaborationScope, useUpdateCollaborationNotification, type CollaborationNotification, type CollaborationScope } from "@multica/core/collaboration";
import { useCollaborationRealtime } from "@multica/core/realtime";
import { accountPath, parseAccountPath, type AccountNotificationsDestination } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { useIsCompact } from "@multica/ui/hooks/use-mobile";
import { AppLink, currentPath, rowLinkInteractiveProps, useNavigation, useRowLink } from "../navigation";
import { useLocale, useT, useTimeAgo } from "../i18n";
import { PAGE_GUTTER, PageHeader } from "../layout/page-header";
import { useScrollRestorationAdapter } from "../platform/scroll-restoration";
import { CollaborationDetail } from "./collaboration-detail";
import { recoverFocus } from "./focus";
import { useCollaborationNotificationLabel } from "./notifications";
import { CollaborationQueryState } from "./query-state";

type NotificationPage = NonNullable<Awaited<ReturnType<ApiClient["getCollaborationNotifications"]>>>;
type NotificationsQuery = UseInfiniteQueryResult<InfiniteData<NotificationPage>>;

/** Notification identity and exact content stay route-owned across confirmed writes. */
export function CollaborationNotificationsPage({ destination }: { destination: AccountNotificationsDestination }) {
  const { t } = useT("collaboration");
  const scope = useCollaborationScope();
  const client = useQueryClient();
  const navigation = useNavigation();
  const navigationDestination = parseAccountPath(currentPath(navigation));
  const renderedPath = accountPath(destination);
  const returnRestorationKey = navigation.isHistoryTraversal || navigation.isScopeResume
    ? navigationDestination?.view === "notifications" && accountPath(navigationDestination) === renderedPath
      ? `${navigation.navigationKey ?? 0}:${renderedPath}` : null
    : undefined;
  const query = useInfiniteQuery({ ...collaborationNotificationsOptions(client, scope, destination.notificationView), enabled: scope.userId !== "" });
  const items = [...new Map((query.data?.pages.flatMap((page) => page.items) ?? []).map((item) => [item.id, item])).values()];
  const selected = items.find((item) => item.id === destination.notificationId);
  const hasSelection = destination.notificationId !== null;
  // Wait for the list's protected read. A known denied/mismatched notification
  // never mounts or subscribes to its route's private issue. A target beyond
  // loaded pages can still use the canonical issue-access boundary.
  const readableIssueId = hasSelection && query.data && !isCollaborationDenied(query.error) && destination.issueId &&
    (!selected || selected.can_open === true && selected.kind !== "removed" && selected.issue_id === destination.issueId) ? destination.issueId : null;
  useCollaborationRealtime(scope, readableIssueId);
  const compact = useIsCompact();
  const identity = `${scope.apiScope}:${scope.userId}`;
  const listRoot = useRef<HTMLElement>(null);
  const detailRoot = useRef<HTMLElement>(null);
  const backLink = useRef<HTMLAnchorElement>(null);
  const previousSelection = useRef(destination.notificationId);
  useEffect(() => {
    if (previousSelection.current === destination.notificationId) return;
    previousSelection.current = destination.notificationId;
    if (compact && hasSelection && (document.activeElement === document.body || listRoot.current?.contains(document.activeElement))) backLink.current?.focus({ preventScroll: true });
    if (!hasSelection) recoverFocus(() => listRoot.current);
  }, [compact, hasSelection, destination.notificationId]);
  const unread = query.data?.pages[0]?.unread_count;
  const selectedOutside = query.isSuccess && !query.hasNextPage && !selected && readableIssueId !== null;
  const notificationLabel = useCollaborationNotificationLabel();
  const backHref = accountPath({ view: "notifications", notificationView: destination.notificationView });
  function restoreActionFocus(row: HTMLElement | null) {
    recoverFocus(() => row?.querySelector<HTMLElement>("a, button:not(:disabled)"), () => listRoot.current, () => backLink.current, () => detailRoot.current);
  }
  return <main className="flex h-full min-h-0 min-w-0 flex-col bg-background text-foreground">
    <PageHeader>
      <Bell aria-hidden="true" className="size-4 shrink-0 text-muted-foreground" />
      <h1 className="min-w-0 flex-1 truncate text-body font-medium">{t(($) => $.notifications.title)}</h1>
      {unread !== undefined && unread > 0 && <span className="shrink-0 text-caption tabular-nums text-muted-foreground">{t(($) => $.page.unread, { count: unread })}</span>}
      <Button type="button" size="icon" variant="ghost" aria-label={t(($) => $.notifications.refresh)} title={t(($) => $.notifications.refresh)}
        className="pointer-coarse:min-h-11 pointer-coarse:min-w-11" disabled={query.isFetching} onClick={() => void query.refetch()}><RefreshCw aria-hidden="true" className="size-4" /></Button>
    </PageHeader>
    <div className={`${PAGE_GUTTER} shrink-0 space-y-3 py-3`}>
      <p className="text-caption text-muted-foreground">{t(($) => $.notifications.description)}</p>
      <nav aria-label={t(($) => $.notifications.views)} className="flex min-w-0 flex-wrap gap-1">
        {COLLABORATION_NOTIFICATION_VIEWS.map((view) => <AppLink key={view} href={accountPath({ view: "notifications", notificationView: view })}
          aria-current={destination.notificationView === view ? "page" : undefined}
          className="inline-flex min-h-8 items-center rounded-md px-3 py-2 text-body text-muted-foreground hover:bg-accent hover:text-foreground aria-[current=page]:bg-accent aria-[current=page]:font-semibold aria-[current=page]:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring pointer-coarse:min-h-11">
          {t(($) => $.notifications.filters[view])}
        </AppLink>)}
      </nav>
    </div>
    <div className="grid min-h-0 flex-1 grid-cols-1 grid-rows-1 lg:grid-cols-[minmax(18rem,22rem)_minmax(0,1fr)]">
      <NotificationsList key={`${identity}:${destination.notificationView}`} scope={scope} query={query} items={items} destination={destination}
        root={listRoot} hidden={compact && hasSelection} returnRestorationKey={returnRestorationKey} onActionSettled={restoreActionFocus} />
      {/* Overlapping compact cells retain real list geometry and its scroll offset. */}
      <section key={`${identity}:${destination.issueId ?? "notification"}`} ref={detailRoot}
        data-tab-scroll-root="account-detail" tabIndex={-1} aria-label={t(($) => $.detail.title)} aria-hidden={compact && !hasSelection || undefined} inert={compact && !hasSelection}
        className={`${hasSelection ? "visible" : "invisible lg:visible"} col-start-1 row-start-1 min-h-0 min-w-0 overflow-y-auto [overflow-anchor:none] px-4 pb-10 pt-4 outline-none lg:col-start-2 lg:px-8`}>
        {hasSelection ? <>
          <AppLink ref={backLink} href={backHref} className="mb-4 inline-flex min-h-11 items-center gap-2 rounded-md text-body text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring lg:hidden"><ArrowLeft aria-hidden="true" className="size-4" />{t(($) => $.notifications.back)}</AppLink>
          {selectedOutside && <p role="status" className="mb-6 text-caption text-muted-foreground">{t(($) => $.notifications.filteredOut)}</p>}
          {readableIssueId ? <CollaborationDetail key={`${identity}:${readableIssueId}`} scope={scope} destination={destination} returnRestorationKey={returnRestorationKey} />
            : query.isPending ? <p role="status" className="py-8 text-body text-muted-foreground">{t(($) => $.notifications.loading)}</p>
              : query.error && !query.data ? <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />
                : <p className="py-8 text-body text-muted-foreground">{selected?.kind === "removed" ? notificationLabel("removed") : t(($) => $.page.notificationUnavailable)}</p>}
        </> : <p className="mx-auto max-w-prose py-16 text-body text-muted-foreground">{t(($) => $.notifications.select)}</p>}
      </section>
    </div>
  </main>;
}

function NotificationsList({ scope, query, items, destination, root, hidden, returnRestorationKey, onActionSettled }: {
  scope: CollaborationScope; query: NotificationsQuery; items: CollaborationNotification[]; destination: AccountNotificationsDestination;
  root: RefObject<HTMLElement | null>; hidden: boolean; returnRestorationKey?: string | null; onActionSettled: (row: HTMLElement | null) => void;
}) {
  const { t } = useT("collaboration");
  const restoration = useScrollRestorationAdapter();
  const restored = useRef(false);
  const consumedReturn = useRef<string | null>(null);
  useEffect(() => {
    if (!query.data || !root.current || returnRestorationKey === null) return;
    if (returnRestorationKey === undefined ? restored.current : consumedReturn.current === returnRestorationKey) return;
    const saved = restoration?.get("account-notifications-list");
    if (returnRestorationKey !== undefined || saved) root.current.scrollTop = saved?.top ?? 0;
    if (returnRestorationKey !== undefined) consumedReturn.current = returnRestorationKey;
    restored.current = true;
  }, [query.data, restoration, root, returnRestorationKey]);
  return <section ref={root} data-tab-scroll-root="account-notifications-list" tabIndex={-1} aria-label={t(($) => $.notifications.list)}
    aria-hidden={hidden || undefined} inert={hidden}
    className={`${destination.notificationId ? "invisible lg:visible" : "visible"} col-start-1 row-start-1 min-h-0 min-w-0 overflow-y-auto px-2 pb-6 outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring lg:border-r lg:border-border`}>
    {query.isPending && <div role="status" aria-label={t(($) => $.notifications.loading)} className="space-y-4 px-3 py-4">
      <span className="sr-only">{t(($) => $.notifications.loading)}</span>
      {[0, 1, 2].map((key) => <div key={key} aria-hidden="true" className="space-y-2"><Skeleton className="h-5 w-3/4" /><Skeleton className="h-4 w-1/2" /><Skeleton className="h-4 w-2/3" /></div>)}
    </div>}
    {query.error && <div className="px-3"><CollaborationQueryState error={query.error} retry={() => void (query.isFetchNextPageError ? query.fetchNextPage() : query.refetch())} /></div>}
    {query.isSuccess && !query.hasNextPage && items.length === 0 && <p className="px-3 py-8 text-body text-muted-foreground">{t(($) => $.notifications.empty[destination.notificationView])}</p>}
    <ul className="space-y-1">{items.map((item) => <NotificationRow key={item.id} scope={scope} item={item} query={query} destination={destination} onActionSettled={onActionSettled} />)}</ul>
    {query.hasNextPage && <Button type="button" variant="ghost" className="mt-4 h-auto min-h-8 w-full whitespace-normal pointer-coarse:min-h-11" disabled={query.isFetchingNextPage}
      onClick={() => void query.fetchNextPage()}>{query.isFetchingNextPage ? t(($) => $.notifications.loading) : t(($) => $.page.more)}</Button>}
  </section>;
}

function NotificationRow({ scope, item, query, destination, onActionSettled }: {
  scope: CollaborationScope; item: CollaborationNotification; query: NotificationsQuery; destination: AccountNotificationsDestination;
  onActionSettled: (row: HTMLElement | null) => void;
}) {
  const { t } = useT("collaboration");
  const locale = useLocale();
  const timeAgo = useTimeAgo();
  const notificationLabel = useCollaborationNotificationLabel();
  const rowLink = useRowLink();
  const update = useUpdateCollaborationNotification(scope);
  const root = useRef<HTMLLIElement>(null);
  const metadataId = useId();
  const confirmed = update.data?.id === item.id ? update.data : undefined;
  const unread = (confirmed ? confirmed.read_at : item.read_at) === null;
  const archived = (confirmed ? confirmed.archived_at : item.archived_at) !== null;
  const canOpen = item.can_open === true && item.kind !== "removed";
  const selected = destination.notificationId === item.id;
  const href = accountPath({ view: "notifications", notificationView: destination.notificationView, notificationId: item.id,
    issueId: canOpen ? item.issue_id : null, runId: canOpen ? item.run_id : null,
    commentId: canOpen && item.source_kind === "comment" ? item.source_id : null, anchor: canOpen && item.source_kind === "body" ? "description" : undefined });
  const date = new Date(item.created_at);
  async function change(action: "read" | "archive") {
    if (update.isPending) return;
    let rowStaysInView = true;
    try {
      const receipt = await update.mutateAsync({ id: item.id, action });
      rowStaysInView = destination.notificationView === "archived" ? receipt.archived_at !== null
        : receipt.archived_at === null && (destination.notificationView === "all" || receipt.read_at === null);
      // Prefix invalidation starts every view's refresh; join this list's read.
      await query.refetch({ cancelRefetch: false });
    } catch { /* The mutation keeps its own visible failure and explicit retry. */ }
    // A confirmed row can still be mounted while its filtered Query projection commits.
    onActionSettled(rowStaysInView ? root.current : null);
  }
  return <li ref={root} {...rowLink(href)} data-notification-id={item.id} data-unread={unread || undefined} data-selected={selected || undefined}
    className={`${selected ? "bg-accent" : "hover:bg-accent"} min-w-0 cursor-pointer rounded-md px-3 py-3 [overflow-wrap:anywhere]`}>
    <div className="flex min-w-0 items-start gap-2">
      {unread && <><span aria-hidden="true" className="mt-1.5 size-1.5 shrink-0 rounded-full bg-brand" /><span className="sr-only">{t(($) => $.notifications.unread)}</span></>}
      <AppLink href={href} {...rowLinkInteractiveProps} aria-current={selected ? "true" : undefined} aria-describedby={metadataId}
        className={`${selected ? "font-semibold text-foreground" : unread ? "font-medium text-foreground" : "text-muted-foreground"} min-w-0 flex-1 rounded-sm text-body focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring pointer-coarse:min-h-11`}>
        {canOpen ? item.issue_title : t(($) => $.page.notificationUnavailable)}
      </AppLink>
    </div>
    <p id={metadataId} className="mt-1 text-caption text-muted-foreground">{item.is_mention ? t(($) => $.notifications.mention) : notificationLabel(item.kind)}</p>
    <div className="mt-2 flex min-w-0 flex-wrap items-center justify-between gap-x-2 gap-y-1">
      <time dateTime={item.created_at} title={date.toLocaleString(locale)} className="text-caption tabular-nums text-muted-foreground">{Date.now() - date.getTime() < 7 * 86400_000 ? timeAgo(item.created_at) : date.toLocaleDateString(locale)}</time>
      <div {...rowLinkInteractiveProps} className="flex min-w-0 flex-wrap gap-1">
        {unread && <Button type="button" variant="ghost" size="sm" className="h-auto min-h-8 whitespace-normal pointer-coarse:min-h-11" disabled={update.isPending} onClick={() => void change("read")}>{t(($) => $.page.read)}</Button>}
        {!archived && <Button type="button" variant="ghost" size="sm" className="h-auto min-h-8 whitespace-normal pointer-coarse:min-h-11" disabled={update.isPending} onClick={() => void change("archive")}>{t(($) => $.page.archive)}</Button>}
      </div>
    </div>
    {update.isPending && <p role="status" className="mt-2 text-caption text-muted-foreground">{update.variables.action === "read" ? t(($) => $.notifications.reading) : t(($) => $.notifications.archiving)}</p>}
    {update.error && <div role="alert" className="mt-2 space-y-2 text-caption">
      <p>{update.variables?.action === "archive" ? t(($) => $.notifications.archiveError) : t(($) => $.notifications.readError)}</p>
      <Button {...rowLinkInteractiveProps} type="button" variant="outline" size="sm" className="h-auto min-h-8 whitespace-normal pointer-coarse:min-h-11"
        onClick={(event) => { event.stopPropagation(); void change(update.variables?.action ?? "read"); }}>{update.variables?.action === "archive" ? t(($) => $.notifications.retryArchive) : t(($) => $.notifications.retryRead)}</Button>
    </div>}
  </li>;
}
