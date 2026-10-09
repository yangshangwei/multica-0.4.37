"use client";

import { useId, useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, isIterationAccessDenied } from "@multica/core/api";
import { isIssueStatusCategory } from "@multica/core/issue-statuses";
import {
  groupIterationActivity, hasIterationReadAccess, iterationActivityOptions, iterationDetailOptions,
  iterationGroupedIssuesOptions, projectIterationScopeActivity, selectIterationScopeIssues, selectIterationScopeMetric,
  type IterationActivityFilter, type IterationScopeMetric, type IterationScopeProjection,
} from "@multica/core/iterations";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { useLocale, useT } from "../i18n";
import { IterationError } from "./iteration-error";
import { IterationActivityTimeline } from "./iteration-events-view";
import { iterationDisclosureClass } from "./iteration-presentation";
import { IterationScopeDetails, IterationScopeSelect } from "./iteration-scope-details";

type Detail = Awaited<ReturnType<typeof api.getIteration>>;
type Props = {
  wsId: string;
  id: string;
  iteration: Detail["iteration"];
  timezone: string;
  statistics: Detail["statistics"];
  snapshot: Detail["snapshot"];
};
type Category = "all" | "joined" | "removed" | "cancelled" | "restored" | "reentered" | "commitment" | "lifecycle" | "other";
const GROUP_BATCH_SIZE = 10;

// Categories include known planning transitions; post-start metric membership stays in core.
function activityCategory(item: IterationScopeProjection["events"][number]): Category {
  const before = item.entry.before.status?.category;
  const after = item.entry.after.status?.category;
  if (typeof before === "string" && typeof after === "string" && isIssueStatusCategory(before) && isIssueStatusCategory(after)) {
    if (before !== "cancelled" && after === "cancelled") return "cancelled";
    if (after !== "cancelled" && (before === "cancelled" || (before === "done" && after !== "done"))) return "restored";
  }
  switch (item.entry.kind) {
    case "join": return "joined";
    case "leave": case "delete": return "removed";
    case "cancelIssue": return "cancelled";
    case "reopen": return "restored";
    case "reenter": return "reentered";
    case "baseline": return "commitment";
    case "create": case "start": case "end": case "cancelIteration": return "lifecycle";
    default: return "other";
  }
}

function ScopeMetric({ metric, label, value, selected, detailId, onSelect, primary = false }: {
  metric: IterationScopeMetric;
  label: string;
  value: string;
  selected: boolean;
  detailId: string;
  onSelect: (metric: IterationScopeMetric) => void;
  primary?: boolean;
}) {
  const { t } = useT("projects");
  return <div className="min-w-0 space-y-1">
    <dt className="text-caption text-muted-foreground">{label}</dt>
    <dd><Button variant="ghost" aria-label={t(($) => $.iterations.activityPanel.viewMetric, { metric: label, value })} aria-pressed={selected} aria-controls={detailId} onClick={() => onSelect(metric)} className={`-ml-2 h-auto min-h-8 whitespace-normal px-2 py-1 text-left tabular-nums aria-pressed:bg-muted aria-pressed:font-semibold aria-pressed:underline aria-pressed:underline-offset-4 pointer-coarse:min-h-11 ${primary ? "text-title font-semibold" : "text-body"}`}>{value}</Button></dd>
  </div>;
}

export function IterationEventsPanel(props: Props) {
  return <IterationEventsContent key={`${props.wsId}:${props.id}`} {...props} />;
}

function IterationEventsContent({ wsId, id, iteration, timezone, statistics, snapshot }: Props) {
  const { t } = useT("projects");
  const locale = useLocale();
  const client = useQueryClient();
  const controlId = useId();
  const detailId = `${controlId}-details`;
  const [filter, setFilter] = useState<IterationActivityFilter>("scope");
  const [metric, setMetric] = useState<IterationScopeMetric | null>(null);
  const [detailResetKey, setDetailResetKey] = useState(0);
  const [category, setCategory] = useState<Category>("all");
  const [search, setSearch] = useState("");
  const [visibleCount, setVisibleCount] = useState(GROUP_BATCH_SIZE);
  const activity = useQuery({ ...iterationActivityOptions(wsId, id), enabled: snapshot === null });
  const events = snapshot?.events ?? activity.data;
  const projection = useMemo(() => projectIterationScopeActivity({ iteration, statistics, snapshot, events }), [iteration, statistics, snapshot, events]);
  const selection = metric ? selectIterationScopeMetric(metric, projection) : null;
  const scopeSelection = selection?.kind === "scope" ? selection : null;
  const taskRead = useQuery({
    ...iterationGroupedIssuesOptions(wsId, id, { scope: scopeSelection?.scope ?? "current" }),
    enabled: snapshot === null && scopeSelection !== null,
  });
  const scopeError = snapshot === null && scopeSelection ? taskRead.error : null;
  const invalidSnapshot = snapshot && (snapshot.workspace_id !== wsId || snapshot.iteration_id !== id);
  // Revocation cancels active reads; the final query error can be CancelledError.
  const denied = !hasIterationReadAccess(client, wsId) || isIterationAccessDenied(activity.error) || isIterationAccessDenied(scopeError);
  const missingError = !snapshot && [activity.error, scopeError].find((error) => error instanceof ApiError && error.status === 404);
  const allGroups = useMemo(() => groupIterationActivity(events ?? [], metric ? "all" : filter), [events, metric, filter]);
  if (invalidSnapshot || denied || missingError) {
    return <div className="space-y-3 text-body">
      {denied ? <p role="alert">{t(($) => $.iterations.pages.accessDenied)}</p>
        : invalidSnapshot ? <p role="alert">{t(($) => $.iterations.activityPanel.readError)}</p>
          : <IterationError error={missingError} context="read" />}
    </div>;
  }

  const { phase, statistics: counters } = projection;
  const planning = phase === "planned" || phase === "cancelledBeforeStart";
  const started = phase === "active" || phase === "completed" || phase === "cancelledAfterStart";
  const metricLabels: Record<IterationScopeMetric, string> = {
    original: t(($) => $.iterations.original),
    current: t(($) => phase === "planned" ? $.iterations.activityPanel.plannedTasks : snapshot ? $.iterations.activityPanel.finalScope : $.iterations.currentCount),
    initial_effective: t(($) => $.iterations.activityPanel.initialEffective),
    effective: t(($) => snapshot ? $.iterations.activityPanel.finalEffective : $.iterations.activityPanel.currentEffective),
    cancelled: t(($) => $.iterations.cancelledCount),
    added_unique: t(($) => $.iterations.activityPanel.addedUnique),
    removed_events: t(($) => $.iterations.activityPanel.removedDuring),
    reentry_events: t(($) => $.iterations.activityPanel.reentries),
    cancel_events: t(($) => $.iterations.activityPanel.cancellations),
    reopen_events: t(($) => $.iterations.activityPanel.reopens),
    net_effective_change: t(($) => $.iterations.activityPanel.netChange),
  };
  const taskCount = (count: number) => t(($) => $.iterations.activityPanel.taskCount, { count });
  const metricValue = (value: IterationScopeMetric) => value.endsWith("_events")
    ? t(($) => $.iterations.activityPanel.eventCount, { count: counters[value] })
    : `${value === "net_effective_change" && counters[value] > 0 ? "+" : ""}${taskCount(counters[value])}`;
  const clearRefinements = () => { setCategory("all"); setSearch(""); setVisibleCount(GROUP_BATCH_SIZE); };
  const selectMetric = (next: IterationScopeMetric) => { setMetric(next); setDetailResetKey((value) => value + 1); clearRefinements(); };
  const renderMetric = (value: IterationScopeMetric, primary = false) => <ScopeMetric key={value} metric={value} label={metricLabels[value]} value={metricValue(value)} selected={metric === value} detailId={detailId} onSelect={selectMetric} primary={primary} />;
  const categoryOptions: { value: Category; label: string }[] = [
    { value: "all", label: t(($) => $.iterations.activityPanel.categoryAll) },
    { value: "joined", label: t(($) => $.iterations.activityPanel.categoryJoined) },
    { value: "removed", label: t(($) => $.iterations.activityPanel.categoryRemoved) },
    { value: "cancelled", label: t(($) => $.iterations.activityPanel.categoryCancelled) },
    { value: "restored", label: t(($) => $.iterations.activityPanel.categoryRestored) },
    { value: "reentered", label: t(($) => $.iterations.activityPanel.categoryReentered) },
    { value: "commitment", label: t(($) => $.iterations.activityPanel.categoryCommitment) },
    { value: "lifecycle", label: t(($) => $.iterations.activityPanel.categoryLifecycle) },
    { value: "other", label: t(($) => $.iterations.activityPanel.categoryOther) },
  ];
  const eventSelection = selection?.kind === "tasks" || selection?.kind === "events" ? selection : null;
  const selectedIds = eventSelection ? new Set(eventSelection.eventIds) : null;
  const refined = metric !== null || category !== "all" || search.trim() !== "";
  const term = search.trim().toLocaleLowerCase();
  const groupIds = new Set(allGroups.flatMap((group) => group.entries.map((entry) => entry.event.id)));
  const matchingIds = new Set(projection.events.filter((item) => {
    const { entry } = item;
    return groupIds.has(entry.event.id) && (!selectedIds || selectedIds.has(entry.event.id)) &&
      (category === "all" || activityCategory(item) === category) &&
      (!term || [entry.issue?.title, entry.issue?.identifier].some((value) => value?.toLocaleLowerCase().includes(term)));
  }).map((item) => item.entry.event.id));
  const groups = refined ? allGroups.filter((group) => group.entries.some((entry) => matchingIds.has(entry.event.id))) : allGroups;
  const visible = groups.slice(0, visibleCount);
  const count = refined ? matchingIds.size : groups.reduce((total, group) => total + group.entries.length, 0);
  const impacts = new Map(projection.events.map((item) => [item.entry.event.id, item.impact]));
  const retry = <Button variant="outline" className="pointer-coarse:min-h-11" disabled={activity.isFetching} onClick={() => void activity.refetch()}>{t(($) => $.iterations.retry)}</Button>;

  const activityContent = events === undefined ? (
    activity.error || activity.errorUpdatedAt > 0 ? <div className="space-y-3"><p role="alert">{t(($) => $.iterations.activityPanel.readError)}</p>{retry}</div>
      : <div role="status" className="space-y-5"><span className="sr-only">{t(($) => $.iterations.activityPanel.loading)}</span>{Array.from({ length: 3 }, (_, index) => <div key={index} className="space-y-2"><Skeleton className="h-5 w-2/3" /><Skeleton className="h-4 w-1/3" /><Skeleton className="h-8 w-full" /></div>)}</div>
  ) : <div className="min-w-0 space-y-5">
    {!snapshot && activity.error && <div className="flex flex-wrap items-center gap-3"><p role="alert" className="text-caption">{t(($) => $.iterations.activityPanel.refreshError)}</p>{retry}</div>}
    {eventSelection && !eventSelection.complete && <p role="alert" className="max-w-prose text-caption">{t(($) => $.iterations.activityPanel.incomplete)}</p>}
    {!metric && <div role="group" aria-label={t(($) => $.iterations.activityPanel.filterLabel)} className="flex flex-wrap gap-1">
      {(["scope", "all"] as const).map((value) => <Button key={value} variant="ghost" aria-pressed={filter === value} className="aria-pressed:bg-muted aria-pressed:font-semibold pointer-coarse:min-h-11" onClick={() => setFilter(value)}>{t(($) => value === "all" ? $.iterations.activityPanel.allFilter : planning ? $.iterations.activityPanel.planningAdjustments : $.iterations.activityPanel.scopeFilter)}</Button>)}
    </div>}
    <div className="grid items-end gap-3 sm:grid-cols-[minmax(8rem,1fr)_minmax(12rem,2fr)_auto]">
      <IterationScopeSelect id={`${controlId}-category`} label={t(($) => $.iterations.activityPanel.categoryLabel)} value={category} options={categoryOptions} onValueChange={(value) => { setCategory(value); setVisibleCount(GROUP_BATCH_SIZE); }} />
      <div className="min-w-0 space-y-2"><Label htmlFor={`${controlId}-search`}>{t(($) => $.iterations.activityPanel.searchTasks)}</Label><Input id={`${controlId}-search`} value={search} placeholder={t(($) => $.iterations.activityPanel.searchPlaceholder)} className="pointer-coarse:min-h-11" onChange={(event) => { setSearch(event.target.value); setVisibleCount(GROUP_BATCH_SIZE); }} /></div>
      <Button variant="ghost" className="justify-self-start pointer-coarse:min-h-11" disabled={!search && category === "all"} onClick={clearRefinements}>{t(($) => $.iterations.pages.clearFilters)}</Button>
    </div>
    <p role="status" className="text-caption text-muted-foreground tabular-nums">{t(($) => eventSelection?.kind === "tasks"
      ? eventSelection.complete ? $.iterations.activityPanel.matchingTasks : $.iterations.activityPanel.availableTasks
      : eventSelection && !eventSelection.complete ? $.iterations.activityPanel.availableRecords
        : refined ? $.iterations.activityPanel.matchingRecords : $.iterations.activityPanel.recordCount, { count })}</p>
    {events.length === 0 && !metric ? <p className="py-5 text-muted-foreground">{t(($) => $.iterations.activityPanel.empty)}</p>
      : groups.length === 0 ? <p className="py-5 text-muted-foreground">{t(($) => eventSelection && !eventSelection.complete && !search && category === "all" ? $.iterations.activityPanel.unavailable : refined ? $.iterations.activityPanel.noMatchingActivity : $.iterations.activityPanel.noMatches)}</p>
        : <IterationActivityTimeline wsId={wsId} groups={visible} timezone={timezone} impacts={impacts} matchingEventIds={refined ? matchingIds : undefined} />}
    {groups.length > 0 && <div className="flex flex-wrap items-center gap-3">
      {visible.length < groups.length && <Button variant="outline" className="pointer-coarse:min-h-11" onClick={() => setVisibleCount((value) => value + GROUP_BATCH_SIZE)}>{t(($) => $.iterations.activityPanel.showMore)}</Button>}
      <p className="text-caption text-muted-foreground tabular-nums">{t(($) => $.iterations.activityPanel.showing, { shown: visible.length, total: groups.length })}</p>
    </div>}
  </div>;

  const scopeSource = scopeSelection ? snapshot ? scopeSelection.scope === "original" ? snapshot.original : snapshot.scope : taskRead.data?.items : undefined;
  const staleScope = !!scopeSelection && !snapshot && !!taskRead.data && taskRead.data.scope_revision !== iteration.scope_revision;
  const scopeData = scopeSelection && scopeSource && metric && !staleScope ? selectIterationScopeIssues(scopeSource, scopeSelection.filter, counters[metric]) : undefined;
  const refreshScope = () => { if (snapshot === null) void Promise.all([client.invalidateQueries({ queryKey: iterationDetailOptions(wsId, id).queryKey }), taskRead.refetch()]); };

  return <div className="min-w-0 space-y-6 text-body">
    <section aria-label={t(($) => $.iterations.activityPanel.summary)} className="space-y-4">
      {phase === "planned" && <><dl>{renderMetric("current", true)}</dl><p className="max-w-prose text-caption text-muted-foreground">{t(($) => $.iterations.activityPanel.planHint)}</p></>}
      {phase === "cancelledBeforeStart" && <><p className="font-medium">{t(($) => $.iterations.activityPanel.cancelledBeforeStart)}</p><p className="max-w-prose text-caption text-muted-foreground">{t(($) => $.iterations.activityPanel.cancelledPlanHint)}</p></>}
      {phase === "unknown" && <p className="max-w-prose text-caption text-muted-foreground">{t(($) => $.iterations.activityPanel.unknownPhase)}</p>}
      {started && <>
        <dl className="flex flex-wrap gap-x-8 gap-y-4">{(["initial_effective", "effective", "net_effective_change"] as const).map((value) => renderMetric(value, true))}</dl>
        <p className="max-w-prose text-caption text-muted-foreground">{t(($) => phase === "cancelledAfterStart" ? $.iterations.activityPanel.cancelledAfterStart : snapshot ? $.iterations.activityPanel.closedWindowHint : $.iterations.activityPanel.windowHint)}</p>
        <dl className="flex flex-wrap gap-x-8 gap-y-2">{renderMetric("added_unique")}{renderMetric("cancel_events")}</dl>
        <details>
          <summary className={iterationDisclosureClass}>{t(($) => $.iterations.activityPanel.countDetails)}</summary>
          <dl className="mt-3 grid grid-cols-1 gap-x-8 gap-y-3 sm:grid-cols-2 lg:grid-cols-3">
            {(["original", "current", "cancelled", "removed_events", "reentry_events", "reopen_events"] as const).map((value) => renderMetric(value))}
            <div className="space-y-1"><dt className="text-caption text-muted-foreground">{t(($) => $.iterations.startedCount)}</dt><dd className="tabular-nums">{taskCount(counters.started)}</dd></div>
            <div className="space-y-1"><dt className="text-caption text-muted-foreground">{t(($) => $.iterations.netChangeRatio)}</dt><dd className="tabular-nums">{counters.net_effective_change_ratio === null ? t(($) => $.iterations.notApplicable) : new Intl.NumberFormat(locale, { style: "percent", maximumFractionDigits: 1 }).format(counters.net_effective_change_ratio)}</dd></div>
          </dl>
        </details>
      </>}
    </section>

    {metric ? <section id={detailId} aria-label={t(($) => $.iterations.activityPanel.detailsRegion)} className="min-w-0 space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3"><h2 className="text-body font-semibold">{metricLabels[metric]}</h2><Button variant="ghost" className="pointer-coarse:min-h-11" onClick={() => { setMetric(null); clearRefinements(); }}>{t(($) => $.iterations.activityPanel.backToActivity)}</Button></div>
      {scopeSelection ? <IterationScopeDetails key={`${metric}:${detailResetKey}`} wsId={wsId} data={scopeData} historical={snapshot !== null || scopeSelection.scope === "original"} stale={staleScope} error={scopeError} loading={snapshot === null && taskRead.isPending} fetching={snapshot === null && taskRead.isFetching} canRetry={snapshot === null} onRetry={refreshScope} />
        : selection?.kind === "unavailable" && events !== undefined ? <p role="alert">{t(($) => $.iterations.activityPanel.unavailable)}</p>
          : <><p className="max-w-prose text-caption text-muted-foreground">{t(($) => metric === "added_unique" ? $.iterations.activityPanel.uniqueHint : metric === "net_effective_change" ? $.iterations.activityPanel.netHint : $.iterations.activityPanel.eventHint)}</p>{activityContent}</>}
    </section> : activityContent}
  </div>;
}
