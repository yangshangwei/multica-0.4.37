"use client";
import { useAdminOverview, observationDrilldown } from "@multica/core/admin";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@multica/ui/components/ui/table";
import { useT } from "../../i18n";
import { ObservationFilters, ObservationHeader, ObservationState, MetricRows, useObservationParams } from "../observability/common";

export function AdminOverviewPage() {
  const { t } = useT("admin");
  const params = useObservationParams();
  const query = useAdminOverview(params);
  const data = query.data;
  const finished = (status: string) => data ? observationDrilldown("/admin/tasks", data.window, { status, time_basis: "finished" }) : undefined;
  const live = (status: string) => `/admin/tasks?${new URLSearchParams({ state_scope: "current", status, time_basis: "created", timezone: data?.window.timezone ?? "UTC" })}`;
  const activeAlerts = (status: string) => `/admin/alerts?${new URLSearchParams({ status, timezone: data?.window.timezone ?? "UTC" })}`;
  return <section className="space-y-6"><ObservationHeader title={t($ => $.observability.title)} description={t($ => $.observability.description)} /><ObservationFilters params={params} />
    <ObservationState pending={query.isPending} error={query.isError} quality={data?.dataQuality} asOf={data?.asOf} timezone={data?.window.timezone} retry={() => void query.refetch()}>{data && <>
      <section className="space-y-4"><div className="space-y-1"><h2 className="text-body-lg font-semibold">{t($ => $.observability.live)}</h2><p className="max-w-prose text-caption text-muted-foreground">{t($ => $.observability.liveHint)}</p></div>
        <MetricRows rows={[
          { label: t($ => $.observability.installed), value: data.installations.total, href: "/admin/installations" }, { label: t($ => $.observability.retired), value: data.installations.retired },
          { label: t($ => $.observability.clientActive), value: data.installations.clientActive }, { label: t($ => $.observability.reachable), value: data.installations.daemonReachable }, { label: t($ => $.observability.ready), value: data.installations.ready },
          { label: t($ => $.observability.unassociated), value: data.installations.unassociated, href: "/admin/installations/unassociated" },
          { label: t($ => $.observability.queued), value: data.executions.queued, href: live("queued") }, { label: t($ => $.observability.dispatched), value: data.executions.dispatched, href: live("dispatched") }, { label: t($ => $.observability.running), value: data.executions.running, href: live("running") },
          { label: t($ => $.observability.waiting), value: data.executions.waitingLocalDirectory, href: live("waiting_local_directory") }, { label: t($ => $.observability.deferred), value: data.executions.deferred, href: live("deferred") }, { label: t($ => $.observability.unfinished), value: data.executions.unfinished },
          { label: t($ => $.observability.openAlerts), value: data.alerts.open, href: activeAlerts("open") }, { label: t($ => $.observability.acknowledgedAlerts), value: data.alerts.acknowledged, href: activeAlerts("acknowledged") },
        ]} /></section>
      <section className="space-y-4 border-t border-surface-border pt-6"><div className="space-y-1"><h2 className="text-body-lg font-semibold">{t($ => $.observability.alertHistoryTitle)}</h2><p className="max-w-prose text-caption text-muted-foreground">{t($ => $.observability.alertHistoryHint)}</p></div><MetricRows rows={[
        { label: t($ => $.observability.resolvedAlerts), value: data.alerts.resolved, href: observationDrilldown("/admin/alerts", data.window, { status: "resolved" }) },
        { label: t($ => $.observability.closedAlerts), value: data.alerts.closed, href: observationDrilldown("/admin/alerts", data.window, { status: "closed" }) },
      ]} /></section>
      <section className="space-y-4 border-t border-surface-border pt-6"><div className="space-y-1"><h2 className="text-body-lg font-semibold">{t($ => $.observability.finished)}</h2><p className="max-w-prose text-caption text-muted-foreground">{t($ => $.observability.finishedHint)}</p></div>
        <MetricRows rows={[{ label: t($ => $.observability.completed), value: data.executions.completed, href: finished("completed") }, { label: t($ => $.observability.failed), value: data.executions.failed, href: finished("failed") }, { label: t($ => $.observability.cancelled), value: data.executions.cancelled, href: finished("cancelled") }, { label: t($ => $.observability.successRate), value: data.executions.successRate === null ? t($ => $.observability.noSamples) : new Intl.NumberFormat(undefined, { style: "percent", maximumFractionDigits: 1 }).format(data.executions.successRate) }]} />
      </section>
      <section className="space-y-4 border-t border-surface-border pt-6"><h2 className="text-body-lg font-semibold">{t($ => $.observability.latency)}</h2><div className="overflow-x-auto"><Table><TableHeader><TableRow><TableHead>{t($ => $.observability.latency)}</TableHead><TableHead>{t($ => $.observability.p50)}</TableHead><TableHead>{t($ => $.observability.p95)}</TableHead><TableHead>{t($ => $.observability.samples)}</TableHead></TableRow></TableHeader><TableBody>{[{ label: t($ => $.observability.queueTime), ...data.executions.queueSeconds }, { label: t($ => $.observability.runTime), ...data.executions.runSeconds }].map(row => <TableRow key={row.label}><TableCell>{row.label}</TableCell><TableCell>{row.p50 ?? t($ => $.observability.unknown)}</TableCell><TableCell>{row.p95 ?? t($ => $.observability.unknown)}</TableCell><TableCell>{row.samples}</TableCell></TableRow>)}</TableBody></Table></div><MetricRows rows={[{ label: t($ => $.observability.lowerBound), value: data.executions.queueSeconds.lowerBoundSamples }, { label: t($ => $.observability.unknownSamples), value: data.executions.queueSeconds.unknownSamples }]} /></section>
      <section className="space-y-4 border-t border-surface-border pt-6"><div className="space-y-1"><h2 className="text-body-lg font-semibold">{t($ => $.observability.usage)}</h2><p className="max-w-prose text-caption text-muted-foreground">{t($ => $.observability.tokensHint)}</p></div><p className="text-caption text-muted-foreground">{t($ => $.observability.quality[data.usage.quality])}</p><MetricRows rows={[
        { label: t($ => $.observability.totalTokens), value: data.usage.totalTokens }, { label: t($ => $.observability.inputTokens), value: data.usage.inputTokens }, { label: t($ => $.observability.outputTokens), value: data.usage.outputTokens }, { label: t($ => $.observability.cacheReadTokens), value: data.usage.cacheReadTokens }, { label: t($ => $.observability.cacheWriteTokens), value: data.usage.cacheWriteTokens }, { label: t($ => $.observability.missingUsage), value: data.usage.missingTasks }, { label: t($ => $.observability.unpriced), value: data.usage.unpricedTasks },
      ]} /></section><p className="text-caption text-muted-foreground">{t($ => $.observability.rules)}: {data.ruleVersion}</p>
    </>}</ObservationState>
  </section>;
}
