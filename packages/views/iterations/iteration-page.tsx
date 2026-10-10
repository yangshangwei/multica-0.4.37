"use client";

import { useId, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { CalendarDays, ChevronLeft, Copy, Info, LockKeyhole, MoreHorizontal, Play, Plus, UserRound } from "lucide-react";
import { isIterationAccessDenied } from "@multica/core/api";
import { toast } from "sonner";
import { copyText } from "@multica/ui/lib/clipboard";
import { memberListOptions } from "@multica/core/workspace/queries";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { useModalStore } from "@multica/core/modals";
import { iterationCapabilitiesOptions, iterationSettingsOptions, iterationDetailOptions, iterationCatalogueOptions, iterationIsOverdue, iterationScopePhase } from "@multica/core/iterations";
import { Button } from "@multica/ui/components/ui/button";
import { Badge } from "@multica/ui/components/ui/badge";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "@multica/ui/components/ui/dialog";
import { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem } from "@multica/ui/components/ui/dropdown-menu";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@multica/ui/components/ui/tabs";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Popover, PopoverContent, PopoverTitle, PopoverTrigger } from "@multica/ui/components/ui/popover";
import { AppLink, useNavigation } from "../navigation";
import { useT, useLocale } from "../i18n";
import { formatInTimeZone } from "../common/format-in-time-zone";
import { useIterationLabels } from "./labels";
import { IterationError, isDefinitiveReadError } from "./iteration-error";
import { IterationOperation } from "./iteration-operation";
import { IterationForm } from "./iteration-form";
import { IterationAddExisting } from "./iteration-add-existing";
import { IterationRecovery } from "./iteration-recovery";
import { IterationEventsPanel } from "./iteration-events";
import { IterationIssueList } from "./iteration-issue-list";
import { IterationHistory } from "./iteration-history";
import { IterationOverview } from "./iteration-overview";

export function IterationsPage({ mainLandmark = false }: { mainLandmark?: boolean }) {
  const wsId = useWorkspaceId();
  // Web supplies its main landmark in the shell; Desktop opts in at the route.
  const Container = mainLandmark ? "main" : "div";
  return <Container className="flex h-full min-h-0 flex-col"><IterationRecovery key={wsId} wsId={wsId} /><div className="min-h-0 flex-1"><IterationPageContent key={wsId} wsId={wsId} /></div></Container>;
}
function IterationPageContent({ wsId }: { wsId: string }) {
  const { t } = useT("projects");
  const capability = useQuery(iterationCapabilitiesOptions(wsId));
  if (capability.isPending && capability.errorUpdatedAt === 0) return <IterationPageSkeleton />;
  if (capability.isPending || (capability.error && (!capability.data || isDefinitiveReadError(capability.error)))) return <IterationReadFailure error={capability.error} isFetching={capability.isFetching} onRetry={() => void capability.refetch()} />;
  if (capability.data?.supported !== true || capability.data.manual !== true) return <section className="p-6"><h1 className="text-title font-semibold">{t(($) => $.iterations.title)}</h1><p className="mt-4 text-muted-foreground">{t(($) => $.iterations.unsupported)}</p></section>;
  return <>{capability.error && <IterationRefreshError error={capability.error} isFetching={capability.isFetching} onRetry={() => void capability.refetch()} />}<IterationWorkspace key={wsId} wsId={wsId} atomicHandoff={capability.data.atomic_handoff === true} /></>;
}
function IterationWorkspace({ wsId, atomicHandoff }: { wsId: string; atomicHandoff: boolean }) {
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const id = navigation.pathname.startsWith(paths.iterations() + "/") ? decodeURIComponent(navigation.pathname.slice(paths.iterations().length + 1)) : null;
  const settings = useQuery(iterationSettingsOptions(wsId));
  if (settings.isPending && settings.errorUpdatedAt === 0) return <IterationPageSkeleton />;
  if (!settings.data || isDefinitiveReadError(settings.error)) return <IterationReadFailure error={settings.error} isFetching={settings.isFetching} onRetry={() => void settings.refetch()} />;
  return <>{settings.error && <IterationRefreshError error={settings.error} isFetching={settings.isFetching} onRetry={() => void settings.refetch()} />}{id ?
    <IterationDetail key={`${wsId}:${id}`} wsId={wsId} id={id} settingsRevision={settings.data.revision} enabled={settings.data.enabled} atomicHandoff={atomicHandoff} /> :
    <IterationOverview wsId={wsId} enabled={settings.data.enabled} planningTimezone={settings.data.effective_timezone} />}</>;
}
function IterationDetail({ wsId, id, settingsRevision, enabled, atomicHandoff }: { wsId: string; id: string; settingsRevision: number; enabled: boolean; atomicHandoff: boolean }) {
  const { t } = useT("projects");
  const locale = useLocale();
  const navigation = useNavigation();
  const paths = useWorkspacePaths();
  const labels = useIterationLabels();
  const startReasonId = useId();
  const detail = useQuery(iterationDetailOptions(wsId, id));
  const members = useQuery(memberListOptions(wsId));
  const catalogue = useQuery(iterationCatalogueOptions(wsId));
  const [tab, setTab] = useState("tasks");
  const [progressVisited, setProgressVisited] = useState(false);
  const [eventsVisited, setEventsVisited] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [editVisited, setEditVisited] = useState(false);
  const [assignmentOpen, setAssignmentOpen] = useState(false);
  const [assignmentVisited, setAssignmentVisited] = useState(false);
  const [operation, setOperation] = useState<string | null>(null);
  const openModal = useModalStore((state) => state.open);
  if (detail.isPending && detail.errorUpdatedAt === 0) return <IterationPageSkeleton />;
  if (!detail.data || isDefinitiveReadError(detail.error)) return <IterationReadFailure error={detail.error} isFetching={detail.isFetching} onRetry={() => void detail.refetch()} returnToIterations />;
  const { iteration, snapshot } = detail.data;
  const statistics = snapshot?.statistics ?? detail.data.statistics;
  const phase = iterationScopePhase(iteration, snapshot);
  const planning = phase === "planned" || phase === "cancelledBeforeStart";
  const displayStatus = phase === "completed" ? "completed" : phase === "cancelledAfterStart" ? "cancelled" : iteration.status;
  const closedAt = snapshot?.logical_ended_at ?? iteration.logical_ended_at;
  const known = ["planned", "active", "completed", "cancelled"].includes(iteration.status) && iteration.mode === "manual" && phase !== "unknown";
  const assignable = enabled && known && snapshot === null && ["planned", "active"].includes(iteration.status);
  const otherCurrent = catalogue.data?.find((item) => item.mode === "manual" && item.status === "active" && item.id !== id);
  const isEmptyPlan = phase === "planned" && statistics.current === 0;
  const taskCounters = phase === "planned" ? [[t(($) => $.iterations.activityPanel.plannedTasks), statistics.current]]
    : phase === "cancelledBeforeStart" || phase === "unknown" ? []
      : [[t(($) => $.iterations.original), statistics.original], [t(($) => snapshot ? $.iterations.progressPanel.scopeAtClosure : $.iterations.currentCount), statistics.current], [t(($) => $.iterations.effective), statistics.effective], [t(($) => $.iterations.done), statistics.completed], [t(($) => $.iterations.remaining), statistics.remaining]];
  const date = (value: string) => new Intl.DateTimeFormat(locale, { month: "short", day: "numeric", timeZone: "UTC" }).format(new Date(`${value}T00:00:00Z`));
  const coordinator = members.data?.find((member) => member.user_id === iteration.coordinator_user_id)?.name ?? (iteration.coordinator_user_id ? (members.isSuccess ? t(($) => $.iterations.coordinatorMissing) : iteration.coordinator_user_id) : t(($) => $.iterations.none));
  const createTask = () => openModal("create-issue", { workspace_id: wsId, current_iteration_id: id, expected_iteration_revision: iteration.revision });
  const operationAvailable = (value: string) => enabled && known && snapshot === null && (value === "start" ? iteration.status === "planned" && !otherCurrent && !!catalogue.data && !catalogue.error : value === "delete" ? iteration.status === "planned" && statistics.current === 0 : value === "cancel" ? ["planned", "active"].includes(iteration.status) : value === "handoff" ? iteration.status === "active" && atomicHandoff : iteration.status === "active");
  const startBlocked = assignable && iteration.status === "planned" && !operationAvailable("start");
  const emptyState = <div className="flex min-h-[28rem] items-center px-5 py-12 sm:px-10"><div className="max-w-xl"><div className="mb-7 flex size-12 items-center justify-center rounded-full border-2 border-dotted border-faint-foreground text-faint-foreground"><Play className="size-5" /></div><h2 className="text-title font-semibold [overflow-wrap:anywhere]">{t(($) => $.iterations.pages.emptyLead)}</h2><p className="mt-3 text-body leading-7 text-muted-foreground">{t(($) => $.iterations.pages.emptyDescription)}</p>{assignable && <div className="mt-6 flex flex-wrap gap-2"><Button className="pointer-coarse:min-h-11" onClick={() => { setAssignmentVisited(true); setAssignmentOpen(true); }}><Plus />{t(($) => $.iterations.pages.addExisting)}</Button><Button variant="outline" className="pointer-coarse:min-h-11" onClick={createTask}>{t(($) => $.iterations.pages.newTask)}</Button></div>}{!startBlocked && <p className="mt-5 text-caption text-muted-foreground">{t(($) => $.iterations.pages.manualHint)}</p>}</div></div>;
  return <div className="@container h-full overflow-auto">
    {detail.error && <IterationRefreshError error={detail.error} isFetching={detail.isFetching} onRetry={() => void detail.refetch()} />}
    <div className="flex flex-wrap items-center justify-between gap-3 border-b px-5 py-3 sm:px-7"><AppLink href={paths.iterations()} className="inline-flex items-center gap-2 text-caption text-muted-foreground hover:text-foreground pointer-coarse:min-h-11"><ChevronLeft className="size-4" />{t(($) => $.iterations.back)}</AppLink><div className="flex flex-wrap items-center gap-2">
      {assignable && !isEmptyPlan && <Button size="sm" variant="outline" className="pointer-coarse:min-h-11" onClick={() => { setAssignmentVisited(true); setAssignmentOpen(true); }}><Plus />{t(($) => $.iterations.pages.addExisting)}</Button>}
      {assignable && <Button size="sm" variant={iteration.status === "active" ? "outline" : "default"} className="pointer-coarse:min-h-11" disabled={!operationAvailable(iteration.status === "active" ? "end" : "start")} aria-describedby={startBlocked ? startReasonId : undefined} onClick={() => setOperation(iteration.status === "active" ? "end" : "start")}>{t(($) => iteration.status === "active" ? $.iterations.end : $.iterations.start)}</Button>}
      <Button size="icon-sm" variant="ghost" className="pointer-coarse:min-h-11 pointer-coarse:min-w-11" aria-label={t(($) => $.iterations.copyLink)} onClick={() => { void copyText(navigation.getShareableUrl(paths.iterationDetail(iteration.id))).then((ok) => ok ? toast.success(t(($) => $.iterations.copiedLink)) : toast.error(t(($) => $.iterations.copyFailed))); }}><Copy /></Button>
      {enabled && known && <DropdownMenu><DropdownMenuTrigger render={<Button size="icon-sm" variant="ghost" className="pointer-coarse:min-h-11 pointer-coarse:min-w-11" aria-label={t(($) => $.iterations.pages.more)} />}><MoreHorizontal /></DropdownMenuTrigger><DropdownMenuContent align="end"><DropdownMenuItem className="pointer-coarse:min-h-11" onClick={() => { setEditVisited(true); setEditOpen(true); }}>{t(($) => $.iterations.edit)}</DropdownMenuItem>{assignable && <DropdownMenuItem className="pointer-coarse:min-h-11" onClick={createTask}>{t(($) => $.iterations.pages.newTask)}</DropdownMenuItem>}{operationAvailable("handoff") && <DropdownMenuItem className="pointer-coarse:min-h-11" onClick={() => setOperation("handoff")}>{t(($) => $.iterations.handoff)}</DropdownMenuItem>}{operationAvailable("cancel") && <DropdownMenuItem variant="destructive" className="pointer-coarse:min-h-11" onClick={() => setOperation("cancel")}>{t(($) => $.iterations.cancel)}</DropdownMenuItem>}{operationAvailable("delete") && <DropdownMenuItem variant="destructive" className="pointer-coarse:min-h-11" onClick={() => setOperation("delete")}>{t(($) => $.iterations.delete)}</DropdownMenuItem>}</DropdownMenuContent></DropdownMenu>}
    </div></div>
    {startBlocked && <div className="flex flex-wrap items-center gap-x-3 gap-y-1 px-5 pt-4 sm:px-8"><p id={startReasonId} className="text-caption text-muted-foreground">{t(($) => otherCurrent ? $.iterations.pages.startBlocked : catalogue.error ? $.iterations.audit.startUnavailable : $.iterations.audit.startChecking)}</p>{otherCurrent && <AppLink href={paths.iterationDetail(otherCurrent.id)} className="inline-flex text-caption underline underline-offset-4 [overflow-wrap:anywhere] pointer-coarse:min-h-11">{otherCurrent.name}</AppLink>}{catalogue.error && !isDefinitiveReadError(catalogue.error) && <Button variant="outline" size="sm" className="pointer-coarse:min-h-11" disabled={catalogue.isFetching} onClick={() => void catalogue.refetch()}>{t(($) => $.iterations.retry)}</Button>}</div>}
    <div className="px-5 pb-4 pt-5 sm:px-8">
      <header className="space-y-2">
        <div className="flex flex-wrap items-center gap-3"><h1 className="min-w-0 text-title-lg font-semibold [overflow-wrap:anywhere]">{iteration.name}</h1><Badge variant="secondary" className={displayStatus === "active" ? "bg-chart-1/10 text-info-foreground" : ""}>{labels.status(displayStatus)}</Badge></div>
        {iteration.description && <p className="max-w-prose whitespace-pre-wrap text-body text-muted-foreground [overflow-wrap:anywhere]">{iteration.description}</p>}
        <div className="flex flex-wrap gap-x-5 gap-y-2 text-caption text-muted-foreground">
          <span className="inline-flex items-center gap-1.5"><CalendarDays aria-hidden className="size-3.5 shrink-0" />{date(iteration.start_date)} – {date(iteration.end_date)} · {iteration.timezone}</span>
          <span className="inline-flex items-center gap-1.5 [overflow-wrap:anywhere]"><UserRound aria-hidden className="size-3.5 shrink-0" />{t(($) => $.iterations.coordinator)}: {coordinator} · {iteration.mode === "manual" ? t(($) => $.iterations.manualMode) : iteration.mode}</span>
        </div>
        {(iteration.started_at || closedAt) && <div className="flex flex-wrap gap-x-5 gap-y-2 text-caption text-muted-foreground">
          {iteration.started_at && <p>{t(($) => $.iterations.actualStartedAt)}: <time dateTime={iteration.started_at} title={iteration.timezone}>{formatInTimeZone(iteration.started_at, iteration.timezone, locale, { year: "numeric" })}</time></p>}
          {closedAt && <p className="inline-flex items-start gap-1.5">{snapshot && <LockKeyhole aria-hidden className="mt-0.5 size-3.5 shrink-0" />}<span>{t(($) => snapshot ? $.iterations.readonly : $.iterations.progressPanel.closedAt)} <time dateTime={closedAt} title={iteration.timezone}>{formatInTimeZone(closedAt, iteration.timezone, locale, { year: "numeric" })}</time></span></p>}
        </div>}
        {iterationIsOverdue(displayStatus, iteration.end_date, iteration.timezone) && <p role="status" className="text-caption font-medium text-warning-foreground">{t(($) => $.iterations.overdue)}</p>}
      </header>
      {!known && <p className="mt-4 text-body text-muted-foreground">{t(($) => iteration.mode !== "manual" ? $.iterations.unknownMode : $.iterations.unknown)}</p>}
    </div>
    <Tabs value={tab} onValueChange={(value) => { const next = String(value); setTab(next); if (next === "progress") setProgressVisited(true); if (next === "events") setEventsVisited(true); }} className="gap-0 px-5 pb-8 sm:px-8">
      <div className="border-b"><TabsList variant="line" className="data-[orientation=horizontal]:h-12 gap-5" aria-label={t(($) => $.iterations.pages.detailTabs)}><TabsTrigger value="tasks" className="pointer-coarse:min-h-11">{t(($) => snapshot ? $.iterations.pages.frozenTasks : $.iterations.issues)} <span className="text-caption text-muted-foreground">{statistics.current}</span></TabsTrigger><TabsTrigger value="progress" className="pointer-coarse:min-h-11">{t(($) => $.iterations.pages.progress)}</TabsTrigger><TabsTrigger value="events" className="pointer-coarse:min-h-11">{t(($) => planning ? $.iterations.activityPanel.planningAdjustments : $.iterations.pages.scopeChanges)}</TabsTrigger></TabsList></div>
      <TabsContent value="tasks" keepMounted className="pt-5 data-hidden:hidden">
        <div className="mb-4 flex flex-wrap items-center gap-x-4 gap-y-2">
          {taskCounters.length > 0 && <dl className="flex flex-wrap gap-x-5 gap-y-2">{taskCounters.map(([label, value]) => <div key={label} className="flex items-baseline gap-2"><dt className="text-caption text-muted-foreground">{label}</dt><dd className="text-body font-semibold tabular-nums">{value}</dd></div>)}</dl>}
          {(planning || phase === "unknown") && <p className="text-caption text-muted-foreground">{t(($) => phase === "planned" ? $.iterations.taskList.planHint : phase === "cancelledBeforeStart" ? $.iterations.activityPanel.cancelledPlanHint : $.iterations.taskList.unknownPhase)}</p>}
          {taskCounters.length > 0 && <Popover><PopoverTrigger render={<Button variant="ghost" size="icon-sm" className="text-muted-foreground pointer-coarse:min-h-11 pointer-coarse:min-w-11" aria-label={t(($) => $.iterations.taskList.countsHelp)} />}><Info aria-hidden /></PopoverTrigger><PopoverContent align="start" className="max-w-[calc(100vw-2rem)]"><PopoverTitle className="sr-only">{t(($) => $.iterations.taskList.countsHelp)}</PopoverTitle><p className="text-caption">{t(($) => $.iterations.overallScope)}</p></PopoverContent></Popover>}
        </div>
        <IterationIssueList wsId={wsId} id={id} historical={snapshot !== null} phase={phase} emptyState={isEmptyPlan ? emptyState : undefined} />
      </TabsContent>
      <TabsContent value="progress" keepMounted className="pt-5 data-hidden:hidden">{progressVisited && (planning || phase === "unknown" ? <p className="py-12 text-body text-muted-foreground">{t(($) => phase === "planned" ? $.iterations.pages.noChart : phase === "cancelledBeforeStart" ? $.iterations.activityPanel.cancelledPlanHint : $.iterations.activityPanel.unknownPhase)}</p> : <IterationHistory wsId={wsId} statistics={statistics} snapshot={snapshot} timezone={iteration.timezone} showSummary={false} showEvents={false} />)}</TabsContent>
      <TabsContent value="events" keepMounted className="pt-5 data-hidden:hidden">{eventsVisited && <IterationEventsPanel wsId={wsId} id={id} iteration={iteration} timezone={iteration.timezone} statistics={statistics} snapshot={snapshot} />}</TabsContent>
    </Tabs>
    <Dialog open={editOpen} onOpenChange={setEditOpen}>{editVisited && <DialogContent keepMounted className="max-h-[85vh] overflow-auto sm:max-w-2xl"><DialogHeader><DialogTitle>{t(($) => $.iterations.edit)}</DialogTitle></DialogHeader><IterationForm wsId={wsId} timezone={iteration.timezone} iteration={iteration} expanded available={enabled && known} /></DialogContent>}</Dialog>
    <Dialog open={assignmentOpen} onOpenChange={setAssignmentOpen}>{assignmentVisited && <DialogContent keepMounted className="max-h-[85vh] overflow-auto sm:max-w-xl"><DialogHeader><DialogTitle>{t(($) => $.iterations.pages.addExisting)}</DialogTitle><DialogDescription className="[overflow-wrap:anywhere]">{iteration.name}</DialogDescription></DialogHeader><IterationAddExisting wsId={wsId} iteration={iteration} open={assignmentOpen} available={assignable} onAdded={() => setAssignmentOpen(false)} /></DialogContent>}</Dialog>
    {(["start", "cancel", "delete", "end", "handoff"] as const).map((value) => <IterationOperation key={value} wsId={wsId} iteration={iteration} settingsRevision={settingsRevision} operation={value} available={operationAvailable(value)} open={operation === value} onOpenChange={(open) => setOperation(open ? value : null)} hideTrigger />)}
  </div>;
}
function IterationPageSkeleton() {
  const { t } = useT("projects");
  return <div role="status" className="@container h-full overflow-auto">
    <span className="sr-only">{t(($) => $.iterations.loading)}</span>
    <div aria-hidden>
      <div className="flex items-center justify-between gap-4 border-b px-5 py-3 sm:px-7"><Skeleton className="h-7 w-32" /><Skeleton className="h-7 w-28" /></div>
      <div className="space-y-6 px-5 py-5 sm:px-8">
        <Skeleton className="h-7 w-1/3" /><Skeleton className="h-4 w-1/2" />
        <div className="flex gap-5 border-b pb-4"><Skeleton className="h-6 w-20" /><Skeleton className="h-6 w-20" /><Skeleton className="h-6 w-20" /></div>
        <div className="flex flex-wrap gap-8">{[0, 1, 2, 3].map((item) => <div key={item} className="space-y-2"><Skeleton className="h-4 w-20" /><Skeleton className="h-7 w-10" /></div>)}</div>
        <div className="space-y-3">{[0, 1, 2].map((row) => <Skeleton key={row} className="h-12 w-full" />)}</div>
      </div>
    </div>
  </div>;
}
function IterationReadFailure({ error, onRetry, isFetching, returnToIterations = false }: { error: unknown; onRetry: () => void; isFetching: boolean; returnToIterations?: boolean }) {
  const { t } = useT("projects");
  const paths = useWorkspacePaths();
  const backToIterations = returnToIterations && !isIterationAccessDenied(error);
  return <section className="space-y-4 p-6">
    {isFetching ? <p role="status">{t(($) => $.iterations.loading)}</p> : <IterationError error={error} context="read" />}
    <div className="flex flex-wrap items-center gap-4">
      {!isDefinitiveReadError(error) && <Button variant="outline" className="pointer-coarse:min-h-11" disabled={isFetching} onClick={onRetry}>{t(($) => $.iterations.retry)}</Button>}
      <AppLink href={backToIterations ? paths.iterations() : paths.root()} className="inline-flex min-h-8 items-center text-caption underline underline-offset-4 pointer-coarse:min-h-11">{t(($) => backToIterations ? $.iterations.back : $.iterations.pages.backToWorkspace)}</AppLink>
    </div>
  </section>;
}
function IterationRefreshError({ error, onRetry, isFetching }: { error: unknown; onRetry: () => void; isFetching: boolean }) {
  const { t } = useT("projects");
  return <div className="space-y-2 p-5"><IterationError error={error} context="read" /><Button variant="outline" className="pointer-coarse:min-h-11" disabled={isFetching} onClick={onRetry}>{t(($) => $.iterations.retryRefresh)}</Button></div>;
}
