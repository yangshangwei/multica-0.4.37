"use client";

import { Fragment, useId, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ArrowRight, CalendarRange, Check, ListFilter, PauseCircle, Play, Plus, Settings2 } from "lucide-react";
import { iterationCatalogueOptions, iterationDetailOptions, iterationTimeline, type Iteration } from "@multica/core/iterations";
import { useWorkspacePaths } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Badge } from "@multica/ui/components/ui/badge";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@multica/ui/components/ui/select";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@multica/ui/components/ui/dialog";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@multica/ui/components/ui/tabs";
import { AppLink } from "../navigation";
import { useT, useLocale } from "../i18n";
import { IterationError, isDefinitiveReadError } from "./iteration-error";
import { IterationForm } from "./iteration-form";
import { IterationChartTable, IterationProgressChart } from "./iteration-progress";
import { useIterationLabels } from "./labels";
import { iterationDisclosureClass } from "./iteration-presentation";

export function IterationOverview({ wsId, enabled, planningTimezone }: { wsId: string; enabled: boolean; planningTimezone: string }) {
  const { t } = useT("projects");
  const paths = useWorkspacePaths();
  const labels = useIterationLabels();
  const statusId = useId();
  const catalogue = useQuery(iterationCatalogueOptions(wsId));
  const [tab, setTab] = useState("open");
  const [search, setSearch] = useState("");
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [status, setStatus] = useState("");
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [createVisited, setCreateVisited] = useState(false);
  const [visible, setVisible] = useState(5);
  const timeline = iterationTimeline(catalogue.data ?? [], { search, from, to, status });
  const activeTab = !enabled && tab === "open" ? "history" : tab;
  const rows = activeTab === "open" ? [...timeline.planned, ...timeline.active] : activeTab === "history" ? timeline.history : timeline.unknown;
  const displayedRows = activeTab === "open" ? [...timeline.planned.slice(0, visible), ...timeline.active] : rows.slice(0, visible);
  const hasMore = (activeTab === "open" ? timeline.planned.length : rows.length) > visible;
  const filtered = !!(search || from || to || status);
  const hasCurrent = catalogue.data?.some((item) => item.status === "active" && item.mode === "manual");
  const settingsHref = `${paths.settings()}?tab=iterations`;
  const statusItems = [{ value: "", label: t(($) => $.iterations.all) }, ...["planned", "active", "completed", "cancelled"].map((value) => ({ value, label: labels.status(value) }))];
  const reset = () => { setSearch(""); setFrom(""); setTo(""); setStatus(""); setVisible(50); };
  return <div className="@container flex h-full min-h-0 flex-col">
    <header className="flex flex-wrap items-center justify-between gap-3 border-b px-5 py-3 sm:px-7">
      <h1 className="flex items-center gap-2 text-title-sm font-semibold"><CalendarRange className="size-4 text-muted-foreground" />{t(($) => $.iterations.title)}</h1>
      <div className="flex flex-wrap items-center gap-2">
        <AppLink href={settingsHref} className="inline-flex min-h-8 items-center gap-1.5 rounded-md px-2 text-caption text-muted-foreground hover:text-foreground pointer-coarse:min-h-11"><Settings2 className="size-3.5" />{t(($) => $.iterations.settings)}</AppLink>
        <Button variant="ghost" size="sm" className="pointer-coarse:min-h-11" aria-expanded={filtersOpen} onClick={() => setFiltersOpen(!filtersOpen)}><ListFilter />{t(($) => $.iterations.pages.filters)}</Button>
        {enabled && <Button size="sm" className="pointer-coarse:min-h-11" onClick={() => { setCreateVisited(true); setCreateOpen(true); }}><Plus />{t(($) => $.iterations.create)}</Button>}
      </div>
    </header>
    <div className="min-h-0 flex-1 overflow-auto">
      <Tabs value={activeTab} onValueChange={(value) => { setTab(String(value)); setVisible(value === "open" ? 5 : 50); }} className="gap-0">
        <div className="border-b px-5 sm:px-7"><TabsList variant="line" className="data-[orientation=horizontal]:h-12 gap-5" aria-label={t(($) => $.iterations.title)}>
          <TabsTrigger value="open" disabled={!enabled} className="pointer-coarse:min-h-11">{t(($) => $.iterations.pages.openTab)}</TabsTrigger>
          <TabsTrigger value="history" className="pointer-coarse:min-h-11">{t(($) => $.iterations.pages.historyTab)}</TabsTrigger>
          {timeline.unknown.length > 0 && <TabsTrigger value="unknown" className="pointer-coarse:min-h-11">{t(($) => $.iterations.unknown)}</TabsTrigger>}
        </TabsList></div>
      {filtersOpen && <div className="flex flex-wrap items-end gap-3 border-b px-5 py-4 sm:px-7">
        <label className="flex w-full flex-col gap-1.5 text-caption text-muted-foreground sm:w-56">{t(($) => $.iterations.search)}<Input placeholder={t(($) => $.iterations.search)} value={search} onChange={(event) => { setSearch(event.target.value); setVisible(50); }} className="pointer-coarse:min-h-11" /></label>
        <label className="flex flex-col gap-1.5 text-caption text-muted-foreground">{t(($) => $.iterations.startDate)}<Input type="date" value={from} onChange={(event) => { setFrom(event.target.value); setVisible(50); }} className="pointer-coarse:min-h-11" /></label>
        <label className="flex flex-col gap-1.5 text-caption text-muted-foreground">{t(($) => $.iterations.endDate)}<Input type="date" value={to} onChange={(event) => { setTo(event.target.value); setVisible(50); }} className="pointer-coarse:min-h-11" /></label>
        <div className="flex flex-col gap-1.5">
          <label htmlFor={statusId} className="text-caption text-muted-foreground">{t(($) => $.iterations.status)}</label>
          <Select items={statusItems} value={status} onValueChange={(value) => { setStatus(value ?? ""); setVisible(50); }}>
            <SelectTrigger id={statusId} className="min-w-36 pointer-coarse:min-h-11"><SelectValue /></SelectTrigger>
            <SelectContent>{statusItems.map((item) => <SelectItem key={item.value} value={item.value} className="pointer-coarse:min-h-11">{item.label}</SelectItem>)}</SelectContent>
          </Select>
        </div>
        {filtered && <Button variant="ghost" size="sm" className="pointer-coarse:min-h-11" onClick={reset}>{t(($) => $.iterations.pages.clearFilters)}</Button>}
      </div>}
      {["open", "history", "unknown"].map((value) => <TabsContent key={value} value={value} aria-label={t(($) => value === "open" ? $.iterations.pages.openTab : value === "history" ? $.iterations.pages.historyTab : $.iterations.unknown)}>
        {activeTab === value && <section className="px-4 py-5 sm:px-7" aria-label={t(($) => $.iterations.pages.timeline)}>
        {catalogue.isPending && <div role="status" className="space-y-7 py-4"><span className="sr-only">{t(($) => $.iterations.loading)}</span><div aria-hidden className="space-y-7">{[0, 1, 2].map((row) => <div key={row} className="grid grid-cols-[3rem_minmax(0,1fr)] gap-5 sm:grid-cols-[4rem_minmax(0,1fr)]"><Skeleton className="h-8 w-8" /><div className="space-y-3"><Skeleton className="h-5 w-2/5" /><Skeleton className="h-4 w-1/4" /><Skeleton className="h-14 w-full" /></div></div>)}</div></div>}
        {catalogue.error && <div className="mb-5 space-y-2"><IterationError error={catalogue.error} /><Button variant="outline" className="pointer-coarse:min-h-11" disabled={catalogue.isFetching} onClick={() => void catalogue.refetch()}>{t(($) => $.iterations.retry)}</Button></div>}
        {!enabled && <p className="mb-5 text-body text-muted-foreground">{t(($) => $.iterations.pages.disabledHistory)}</p>}
        {catalogue.data && !catalogue.error && !hasCurrent && activeTab === "open" && <p className="mb-4 text-caption text-muted-foreground">{t(($) => $.iterations.noActive)}</p>}
        {catalogue.data && rows.length === 0 && <div className="flex min-h-64 flex-col items-center justify-center gap-4 text-body text-muted-foreground"><p>{t(($) => filtered ? $.iterations.pages.noMatches : $.iterations.empty)}</p>{filtered && <Button variant="outline" className="pointer-coarse:min-h-11" onClick={reset}>{t(($) => $.iterations.pages.clearFilters)}</Button>}</div>}
        {displayedRows.map((iteration, index) => <Fragment key={iteration.id}>
          <IterationTimelineRow wsId={wsId} iteration={iteration} planningTimezone={planningTimezone} upcoming={iteration.id === timeline.upcomingId} />
          {!catalogue.isFetching && !catalogue.error && timeline.gaps.has(iteration.id) && activeTab === "open" && rows[rows.indexOf(iteration) + 1]?.id === displayedRows[index + 1]?.id && <div className="ml-12 flex items-center gap-2 border-l py-4 pl-5 text-caption text-muted-foreground sm:ml-16 sm:pl-7"><PauseCircle className="size-3.5" />{t(($) => $.iterations.pages.unallocated, { count: timeline.gaps.get(iteration.id) })}</div>}
        </Fragment>)}
        {hasMore && <Button variant="outline" className="mt-5 pointer-coarse:min-h-11" onClick={() => setVisible(visible + 50)}>{t(($) => $.iterations.audit.loadMore)}</Button>}
        </section>}
      </TabsContent>)}
      </Tabs>
    </div>
    <Dialog open={createOpen} onOpenChange={setCreateOpen}>
      {createVisited && <DialogContent keepMounted className="max-h-[85vh] overflow-auto sm:max-w-2xl"><DialogHeader><DialogTitle>{t(($) => $.iterations.create)}</DialogTitle></DialogHeader><IterationForm wsId={wsId} timezone={planningTimezone} expanded available={enabled} /></DialogContent>}
    </Dialog>
  </div>;
}

function IterationTimelineRow({ wsId, iteration, planningTimezone, upcoming }: { wsId: string; iteration: Iteration; planningTimezone: string; upcoming: boolean }) {
  const { t } = useT("projects");
  const locale = useLocale();
  const labels = useIterationLabels();
  const paths = useWorkspacePaths();
  const active = iteration.status === "active" && iteration.mode === "manual";
  const detail = useQuery({ ...iterationDetailOptions(wsId, iteration.id), enabled: active });
  const statistics = isDefinitiveReadError(detail.error) ? undefined : detail.data?.snapshot?.statistics ?? detail.data?.statistics;
  const formatter = new Intl.DateTimeFormat(locale, { month: "short", day: "numeric", timeZone: "UTC" });
  const date = (value: string) => formatter.format(new Date(`${value}T00:00:00Z`));
  const startDateParts = formatter.formatToParts(new Date(`${iteration.start_date}T00:00:00Z`)).reduce<string[]>((parts, part) => {
    if (part.type === "literal" && parts.length > 0) parts[parts.length - 1] += part.value;
    else parts.push(part.value);
    return parts;
  }, []);
  return <article className="grid grid-cols-[3rem_minmax(0,1fr)] sm:grid-cols-[4rem_minmax(0,1fr)]">
    <div className="relative pr-4 pt-6 text-right text-caption text-muted-foreground sm:pr-5"><time dateTime={iteration.start_date} className="flex flex-wrap justify-end gap-x-1">{startDateParts.map((part, index) => <span key={index} className="whitespace-nowrap">{part}</span>)}</time><span aria-hidden className={`absolute -right-1 top-7 size-2 rounded-full border-2 ${active ? "border-chart-1 bg-chart-1" : "border-faint-foreground bg-background"}`} /></div>
    <div className={`min-w-0 border-l pl-5 sm:pl-7 ${active ? "border-chart-1" : "border-border"}`}>
      <div className={`flex min-h-24 flex-wrap items-center justify-between gap-3 py-5 ${active ? "" : "border-b"}`}>
        <div className="flex min-w-0 items-center gap-3"><span aria-hidden className={`flex size-6 shrink-0 items-center justify-center rounded-full border ${active ? "border-chart-1 text-chart-1" : iteration.status === "planned" ? "border-dashed border-faint-foreground text-muted-foreground" : "text-muted-foreground"}`}>{iteration.status === "completed" ? <Check className="size-3" /> : <Play className="size-3" />}</span><div className="min-w-0"><AppLink href={paths.iterationDetail(iteration.id)} className="inline-flex min-w-0 items-center text-body font-semibold [overflow-wrap:anywhere] hover:underline pointer-coarse:min-h-11">{iteration.name}</AppLink><p className="mt-1 text-caption text-muted-foreground">{date(iteration.start_date)} – {date(iteration.end_date)}{iteration.timezone !== planningTimezone && <> · {iteration.timezone}</>}</p></div></div>
        <div className="flex flex-wrap items-center gap-4"><Badge variant="secondary" className={active ? "bg-chart-1/10 text-info-foreground" : ""}>{active ? t(($) => $.iterations.currentGroup) : upcoming ? t(($) => $.iterations.upcoming) : labels.status(iteration.status)}</Badge>{active && statistics && <span className="text-caption text-muted-foreground">{t(($) => $.iterations.currentCount)} {statistics.current}</span>}</div>
      </div>
      {active && <div className="pb-4">
        {detail.isPending && <div role="status" className="space-y-4"><span className="sr-only">{t(($) => $.iterations.loading)}</span><div aria-hidden className="space-y-4"><Skeleton className="h-16 w-1/2" /><Skeleton className="h-56 w-full" /></div></div>}
        {detail.error && <div className="space-y-2"><IterationError error={detail.error} context="read" />{!isDefinitiveReadError(detail.error) && <Button size="sm" variant="outline" className="pointer-coarse:min-h-11" disabled={detail.isFetching} onClick={() => void detail.refetch()}>{t(($) => $.iterations.retry)}</Button>}</div>}
        {statistics && <><IterationProgressChart statistics={statistics} /><details className="mb-3"><summary className={iterationDisclosureClass}>{t(($) => $.iterations.pages.viewChartData)}</summary><IterationChartTable statistics={statistics} /></details><div className="flex flex-wrap items-center justify-between gap-3 py-2 text-caption text-muted-foreground"><p>{t(($) => $.iterations.cancelledCount)} {statistics.cancelled} · {t(($) => $.iterations.pages.countUnit)}</p><AppLink href={paths.iterationDetail(iteration.id)} className="inline-flex min-h-8 items-center gap-2 hover:text-foreground pointer-coarse:min-h-11">{t(($) => $.iterations.pages.viewIteration)}<ArrowRight className="size-3.5" /></AppLink></div></>}
      </div>}
    </div>
  </article>;
}
