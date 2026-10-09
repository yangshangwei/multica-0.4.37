"use client";

import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, isIterationAccessDenied } from "@multica/core/api";
import { groupIterationActivity, hasIterationReadAccess, iterationActivityOptions, type IterationActivityFilter } from "@multica/core/iterations";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { useLocale, useT } from "../i18n";
import { IterationError } from "./iteration-error";
import { IterationActivityTimeline } from "./iteration-events-view";
import { iterationDisclosureClass } from "./iteration-presentation";

type Detail = Awaited<ReturnType<typeof api.getIteration>>;
type Props = {
  wsId: string;
  id: string;
  timezone: string;
  statistics: Detail["statistics"];
  snapshot: Detail["snapshot"];
};
const GROUP_BATCH_SIZE = 10;

export function IterationEventsPanel(props: Props) {
  return <IterationEventsContent key={`${props.wsId}:${props.id}`} {...props} />;
}

function IterationEventsContent({ wsId, id, timezone, statistics, snapshot }: Props) {
  const { t } = useT("projects");
  const locale = useLocale();
  const client = useQueryClient();
  const [filter, setFilter] = useState<IterationActivityFilter>("scope");
  const [visibleCount, setVisibleCount] = useState(GROUP_BATCH_SIZE);
  const activity = useQuery({ ...iterationActivityOptions(wsId, id), enabled: snapshot === null });
  const invalidSnapshot = snapshot && (snapshot.workspace_id !== wsId || snapshot.iteration_id !== id);
  // Revocation cancels active reads; the final query error can be CancelledError.
  const denied = !hasIterationReadAccess(client, wsId) || isIterationAccessDenied(activity.error);
  const missing = !snapshot && activity.error instanceof ApiError && activity.error.status === 404;
  if (invalidSnapshot || denied || missing) {
    return <div className="space-y-3 text-body">
      {denied ? <p role="alert">{t(($) => $.iterations.pages.accessDenied)}</p>
        : invalidSnapshot ? <p role="alert">{t(($) => $.iterations.activityPanel.readError)}</p>
          : <IterationError error={activity.error} context="read" />}
    </div>;
  }

  const events = snapshot?.events ?? activity.data;
  const groups = groupIterationActivity(events ?? [], filter);
  const visible = groups.slice(0, visibleCount);
  const counters = snapshot?.statistics ?? statistics;
  const taskCount = (count: number) => t(($) => $.iterations.activityPanel.taskCount, { count });
  const eventCount = (count: number) => t(($) => $.iterations.activityPanel.eventCount, { count });
  const retry = <Button variant="outline" className="pointer-coarse:min-h-11" disabled={activity.isFetching} onClick={() => void activity.refetch()}>{t(($) => $.iterations.retry)}</Button>;

  return <div className="min-w-0 space-y-6 text-body">
    <section aria-label={t(($) => $.iterations.activityPanel.summary)} className="space-y-4">
      <dl className="flex flex-wrap gap-x-8 gap-y-4">
        {[
          [t(($) => $.iterations.activityPanel.initialEffective), taskCount(counters.initial_effective)],
          [t(($) => snapshot ? $.iterations.activityPanel.finalEffective : $.iterations.activityPanel.currentEffective), taskCount(counters.effective)],
          [t(($) => $.iterations.activityPanel.netChange), `${counters.net_effective_change > 0 ? "+" : ""}${taskCount(counters.net_effective_change)}`],
        ].map(([label, value]) => <div key={label} className="space-y-1">
          <dt className="text-caption text-muted-foreground">{label}</dt>
          <dd className="text-title font-semibold tabular-nums">{value}</dd>
        </div>)}
      </dl>
      <dl className="flex flex-wrap gap-x-6 gap-y-2 text-caption">
        <div className="flex flex-wrap gap-x-2"><dt className="text-muted-foreground">{t(($) => $.iterations.activityPanel.addedUnique)}</dt><dd className="tabular-nums">{taskCount(counters.added_unique)}</dd></div>
        <div className="flex flex-wrap gap-x-2"><dt className="text-muted-foreground">{t(($) => $.iterations.activityPanel.cancellations)}</dt><dd className="tabular-nums">{eventCount(counters.cancel_events)}</dd></div>
      </dl>
      <details>
        <summary className={iterationDisclosureClass}>{t(($) => $.iterations.activityPanel.countDetails)}</summary>
        <dl className="mt-3 grid grid-cols-1 gap-x-8 gap-y-3 sm:grid-cols-2 lg:grid-cols-3">
          {[
            [t(($) => snapshot ? $.iterations.activityPanel.finalScope : $.iterations.currentCount), taskCount(counters.current)],
            [t(($) => $.iterations.cancelledCount), taskCount(counters.cancelled)],
            [t(($) => $.iterations.activityPanel.removedDuring), eventCount(counters.removed_events)],
            [t(($) => $.iterations.activityPanel.reentries), eventCount(counters.reentry_events)],
            [t(($) => $.iterations.activityPanel.reopens), eventCount(counters.reopen_events)],
            [t(($) => $.iterations.startedCount), taskCount(counters.started)],
            [t(($) => $.iterations.netChangeRatio), counters.net_effective_change_ratio === null ? t(($) => $.iterations.notApplicable) : new Intl.NumberFormat(locale, { style: "percent", maximumFractionDigits: 1 }).format(counters.net_effective_change_ratio)],
          ].map(([label, value]) => <div key={label} className="space-y-1">
            <dt className="text-caption text-muted-foreground">{label}</dt>
            <dd className="tabular-nums">{value}</dd>
          </div>)}
        </dl>
      </details>
    </section>

    {events === undefined ? (
      activity.error || activity.errorUpdatedAt > 0 ? <div className="space-y-3">
        <p role="alert">{t(($) => $.iterations.activityPanel.readError)}</p>{retry}
      </div> : <div role="status" className="space-y-5"><span className="sr-only">{t(($) => $.iterations.activityPanel.loading)}</span>{Array.from({ length: 3 }, (_, index) => <div key={index} className="space-y-2"><Skeleton className="h-5 w-2/3" /><Skeleton className="h-4 w-1/3" /><Skeleton className="h-8 w-full" /></div>)}</div>
    ) : <div className="space-y-5">
      {!snapshot && activity.error && <div className="flex flex-wrap items-center gap-3">
        <p role="alert" className="text-caption">{t(($) => $.iterations.activityPanel.refreshError)}</p>{retry}
      </div>}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div role="group" aria-label={t(($) => $.iterations.activityPanel.filterLabel)} className="flex flex-wrap gap-1">
          {(["scope", "all"] as const).map((value) => <Button
            key={value}
            variant="ghost"
            aria-pressed={filter === value}
            className="aria-pressed:bg-muted aria-pressed:font-semibold pointer-coarse:min-h-11"
            onClick={() => setFilter(value)}
          >{t(($) => value === "scope" ? $.iterations.activityPanel.scopeFilter : $.iterations.activityPanel.allFilter)}</Button>)}
        </div>
        <p role="status" className="text-caption text-muted-foreground tabular-nums">
          {t(($) => $.iterations.activityPanel.recordCount, { count: groups.reduce((count, group) => count + group.entries.length, 0) })}
        </p>
      </div>
      {events.length === 0 ? <p className="py-5 text-muted-foreground">{t(($) => $.iterations.activityPanel.empty)}</p>
        : groups.length === 0 ? <p className="py-5 text-muted-foreground">{t(($) => $.iterations.activityPanel.noMatches)}</p>
          : <IterationActivityTimeline wsId={wsId} groups={visible} timezone={timezone} />}
      {groups.length > 0 && <div className="flex flex-wrap items-center gap-3">
        {visible.length < groups.length && <Button variant="outline" className="pointer-coarse:min-h-11" onClick={() => setVisibleCount((count) => count + GROUP_BATCH_SIZE)}>{t(($) => $.iterations.activityPanel.showMore)}</Button>}
        <p className="text-caption text-muted-foreground tabular-nums">{t(($) => $.iterations.activityPanel.showing, { shown: visible.length, total: groups.length })}</p>
      </div>}
    </div>}
  </div>;
}
