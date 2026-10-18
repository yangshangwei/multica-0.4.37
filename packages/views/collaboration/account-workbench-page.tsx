"use client";
import { useEffect, useRef, useState } from "react";
import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
import { collaborationWorkbenchOptions, useCollaborationScope } from "@multica/core/collaboration";
import { accountPath, accountRouteKey, type AccountDestination } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { AppLink } from "../navigation";
import { useT } from "../i18n";
import { useCollaborationLabels } from "./labels";
import { CollaborationQueryState } from "./query-state";
import { CollaborationDetail } from "./collaboration-detail";
import { CollaborationNotifications } from "./notifications";
import { useRestoredScrollRef, useRestoredViewState, useViewStateWriter } from "../platform";

/** Account identity owns every query; selected detail is independent of list membership. */
export function AccountWorkbenchPage({ destination }: { destination: AccountDestination }) {
  const { t } = useT("collaboration");
  const scope = useCollaborationScope();
  return <main className="flex h-full min-h-0 min-w-0 flex-col bg-background text-foreground">
    <header className="flex shrink-0 flex-wrap items-center gap-x-8 gap-y-3 px-4 py-4 md:px-6">
      <h1 className="text-title font-semibold">{t(($) => $.page.title)}</h1>
      <nav aria-label={t(($) => $.page.title)} className="flex min-w-0 flex-wrap gap-2 text-body">
        {(["current", "participating"] as const).map((view) => <AppLink key={view} href={accountPath({ ...destination, view })}
          aria-current={destination.view === view ? "page" : undefined}
          className="rounded-md px-3 py-2 text-muted-foreground hover:bg-accent hover:text-foreground aria-[current=page]:bg-accent aria-[current=page]:font-semibold aria-[current=page]:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring pointer-coarse:min-h-11">
          {view === "current" ? t(($) => $.page.current) : t(($) => $.page.participating)}
        </AppLink>)}
      </nav>
      <CollaborationNotifications key={`${scope.apiScope}:${scope.userId}`} scope={scope} view={destination.view} />
    </header>
    <WorkbenchBody key={`${scope.apiScope}:${scope.userId}:${accountRouteKey(accountPath(destination))}`} scope={scope} destination={destination} />
  </main>;
}

function WorkbenchBody({ scope, destination }: { scope: ReturnType<typeof useCollaborationScope>; destination: AccountDestination }) {
  const { t } = useT("collaboration");
  const labels = useCollaborationLabels();
  const client = useQueryClient();
  const list = useInfiniteQuery({ ...collaborationWorkbenchOptions(client, scope, destination.view), enabled: scope.userId !== "" });
  const savedSearch = useRestoredViewState("account-search");
  const savedSort = useRestoredViewState("account-sort");
  const [search, setSearch] = useState(savedSearch ?? "");
  const [sort, setSort] = useState(savedSort === "title" ? "title" : "updated");
  const writeView = useViewStateWriter();
  const restoreList = useRestoredScrollRef("account-list");
  const restoreDetail = useRestoredScrollRef("account-detail");
  const listRoot = useRef<HTMLDivElement>(null);
  const listRestored = useRef(false);
  useEffect(() => {
    if (!list.data || listRestored.current) return;
    restoreList(listRoot.current);
    listRestored.current = true;
  }, [list.data, restoreList]);
  const rows = [...new Map((list.data?.pages.flatMap((page) => page.items) ?? []).map((item) => [item.id, item])).values()];
  const visible = rows.filter((item) => `${item.title} ${item.handler.name} ${item.source_workspace.name}`.toLocaleLowerCase().includes(search.toLocaleLowerCase()))
    .sort((a, b) => sort === "title" ? a.title.localeCompare(b.title) : b.updated_at.localeCompare(a.updated_at));
  const selectedOutside = list.isSuccess && !list.hasNextPage && destination.issueId !== null && !rows.some((item) => item.id === destination.issueId);
  return <div className="grid min-h-0 flex-1 grid-cols-1 md:grid-cols-[minmax(16rem,22rem)_minmax(0,1fr)]">
    <section aria-label={t(($) => $.page.list)} className={`${destination.issueId ? "hidden md:flex" : "flex"} min-h-0 min-w-0 flex-col border-r border-border`}>
      <div className="space-y-3 px-4 pb-4">
        <Input aria-label={t(($) => $.page.search)} placeholder={t(($) => $.page.search)} value={search} onChange={(event) => { setSearch(event.target.value); writeView("account-search", event.target.value); }} className="pointer-coarse:min-h-11" />
        <div className="flex flex-wrap items-center justify-between gap-2 text-caption text-muted-foreground">
          <span className="tabular-nums">{t(($) => $.page.count, { count: list.data?.pages[0]?.total ?? 0 })}</span>
          <select aria-label={t(($) => $.page.sort)} value={sort} onChange={(event) => { setSort(event.target.value); writeView("account-sort", event.target.value); }} className="min-h-8 max-w-full rounded-md bg-background px-2 text-foreground focus-visible:ring-2 focus-visible:ring-ring pointer-coarse:min-h-11">
            <option value="updated">{t(($) => $.page.updated)}</option><option value="title">{t(($) => $.page.titleSort)}</option>
          </select>
        </div>
      </div>
      <div ref={listRoot} data-tab-scroll-root="account-list" className="min-h-0 overflow-y-auto px-2 pb-6">
        {list.isPending && <p role="status" className="px-2 py-6 text-body text-muted-foreground">{t(($) => $.page.loading)}</p>}
        {list.error && <CollaborationQueryState error={list.error} retry={() => void list.refetch()} />}
        {list.isSuccess && rows.length === 0 && <p className="px-2 py-8 text-body text-muted-foreground">{destination.view === "current" ? t(($) => $.page.emptyCurrent) : t(($) => $.page.emptyParticipating)}</p>}
        {list.isSuccess && rows.length > 0 && visible.length === 0 && <p className="px-2 py-6 text-body text-muted-foreground">{t(($) => $.page.noMatches)}</p>}
        <ul className="space-y-1">{visible.map((item) => <li key={item.id}>
          <AppLink href={accountPath({ view: destination.view, issueId: item.id })} aria-current={item.id === destination.issueId ? "true" : undefined}
            className="block min-w-0 space-y-2 rounded-md px-3 py-3 hover:bg-accent aria-[current=true]:bg-accent aria-[current=true]:font-semibold focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
            <span className="block text-body [overflow-wrap:anywhere]">{item.title}</span>
            <span className="flex flex-wrap gap-x-3 gap-y-1 text-caption text-muted-foreground"><span>{item.source_workspace.name}</span><span>{labels.status(item.status)}</span></span>
            <span className="block text-caption text-muted-foreground [overflow-wrap:anywhere]">{item.handler.name}{item.handler.external === true && <span> · {t(($) => $.detail.external)}</span>}</span>
          </AppLink>
        </li>)}</ul>
        {list.hasNextPage && <Button variant="ghost" className="mt-4 w-full pointer-coarse:min-h-11" disabled={list.isFetchingNextPage} onClick={() => void list.fetchNextPage()}>{t(($) => $.page.more)}</Button>}
      </div>
    </section>
    <section ref={restoreDetail} data-tab-scroll-root="account-detail" aria-label={t(($) => $.detail.title)} className={`${destination.issueId ? "flex" : "hidden md:flex"} min-h-0 min-w-0 flex-col overflow-y-auto px-4 pb-10 md:px-8`}>
      {destination.issueId ? <>
        <AppLink className="mb-4 inline-flex min-h-11 items-center self-start text-body text-muted-foreground hover:text-foreground md:hidden" href={accountPath({ view: destination.view })}>{t(($) => $.page.back)}</AppLink>
        {selectedOutside && <div role="status" className="mb-6 space-y-2 text-caption text-muted-foreground"><p>{t(($) => $.page.filteredOut)}</p>{destination.view === "current" && <AppLink className="underline underline-offset-4" href={accountPath({ ...destination, view: "participating" })}>{t(($) => $.page.openParticipating)}</AppLink>}</div>}
        <CollaborationDetail key={`${scope.apiScope}:${scope.userId}:${destination.issueId}`} scope={scope} destination={destination} />
      </> : <p className="m-auto max-w-prose py-16 text-body text-muted-foreground">{t(($) => $.page.select)}</p>}
    </section>
  </div>;
}
