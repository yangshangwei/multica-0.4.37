"use client";

import { useId, type ReactNode } from "react";
import { useAdminOverview, observationDrilldown } from "@multica/core/admin";
import { Button } from "@multica/ui/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@multica/ui/components/ui/table";
import { Bell, Clock3, MonitorCheck, Play, RefreshCw, type LucideIcon } from "lucide-react";
import { useT } from "../../i18n";
import { AppLink } from "../../navigation";
import { ObservationFilters, ObservationHeader, ObservationState, MetricRows, useObservationParams } from "../observability/common";
import styles from "../admin-visual.module.css";

function OverviewPanel({ title, description, children }: { title: string; description?: string; children: ReactNode }) {
  const id = useId();
  return <section aria-labelledby={id} className={styles.panel}>
    <div className={`${styles.sectionHeading} space-y-1`}>
      <h2 id={id} className="text-title-sm font-semibold">{title}</h2>
      {description && <p className="max-w-prose text-caption text-muted-foreground">{description}</p>}
    </div>
    {children}
  </section>;
}

function OverviewMetric({ label, value, href, icon: Icon, tone }: {
  label: string; value: number | null | undefined; href?: string; icon: LucideIcon;
  tone?: "danger" | "warning" | "success";
}) {
  const { t } = useT("admin");
  const id = useId();
  const known = value !== null && value !== undefined;
  const color = !known || value === 0 ? styles.neutral : tone ? styles[tone] : "";
  return <div className={`${styles.metric} ${color}`}>
    <dt className={`${styles.metricLabel} text-body font-medium`}>
      {href ? <AppLink href={href} aria-describedby={id} className={styles.metricLink}>{label}</AppLink> : <span className="inline-flex min-h-11 items-center">{label}</span>}
      <Icon className={`${styles.metricIcon} size-5`} aria-hidden="true" />
    </dt>
    <dd id={id} className={`${styles.metricValue} ${known ? "text-display" : "text-title-lg"} font-semibold`}>
      {known ? new Intl.NumberFormat().format(value) : t($ => $.observability.unknown)}
    </dd>
  </div>;
}

export function AdminOverviewPage() {
  const { t } = useT("admin");
  const params = useObservationParams();
  const query = useAdminOverview(params);
  const data = query.data;
  const finished = (status: string) => data ? observationDrilldown("/admin/tasks", data.window, { status, time_basis: "finished" }) : undefined;
  const live = (status: string) => `/admin/tasks?${new URLSearchParams({ state_scope: "current", status, time_basis: "created", timezone: data?.window.timezone ?? "UTC" })}`;
  const activeAlerts = (status: string) => `/admin/alerts?${new URLSearchParams({ status, timezone: data?.window.timezone ?? "UTC" })}`;

  return <section className={styles.overview}>
    <div className={styles.pageHeading}>
      <ObservationHeader title={t($ => $.observability.title)} description={t($ => $.observability.description)} />
      <Button variant="outline" disabled={query.isFetching} onClick={() => void query.refetch()}>
        <RefreshCw className={`size-4 ${query.isFetching ? "animate-spin motion-reduce:animate-none" : ""}`} aria-hidden="true" />
        {t($ => $.observability.refresh)}
      </Button>
    </div>
    {(query.isPending || query.isError || !data || data.dataQuality === "unavailable") && <div className={styles.panel}><ObservationFilters params={params} advanced /></div>}
    <ObservationState pending={query.isPending} error={query.isError} quality={data?.dataQuality} asOf={data?.asOf} timezone={data?.window.timezone} retry={() => void query.refetch()}>
      {data && <>
        <section className="space-y-4">
          <div className="space-y-1">
            <h2 className="text-title-sm font-semibold">{t($ => $.observability.attention)}</h2>
            <p className="max-w-prose text-caption text-muted-foreground">{t($ => $.observability.attentionHint)}</p>
          </div>
          <dl className={styles.metricGrid}>
            <OverviewMetric label={t($ => $.observability.openAlerts)} value={data.alerts.open} href={activeAlerts("open")} icon={Bell} tone="danger" />
            <OverviewMetric label={t($ => $.observability.running)} value={data.executions.running} href={live("running")} icon={Play} />
            <OverviewMetric label={t($ => $.observability.queued)} value={data.executions.queued} href={live("queued")} icon={Clock3} tone="warning" />
            <OverviewMetric label={t($ => $.observability.ready)} value={data.installations.ready} icon={MonitorCheck} tone="success" />
          </dl>
          <div className={`${styles.panel} ${styles.attentionDetails}`}>
            <MetricRows rows={[
              { label: t($ => $.observability.acknowledgedAlerts), value: data.alerts.acknowledged, href: activeAlerts("acknowledged") },
              { label: t($ => $.observability.waiting), value: data.executions.waitingLocalDirectory, href: live("waiting_local_directory") },
              { label: t($ => $.observability.deferred), value: data.executions.deferred, href: live("deferred") },
              { label: t($ => $.observability.unassociated), value: data.installations.unassociated, href: "/admin/installations/unassociated" },
            ]} />
          </div>
        </section>
        <OverviewPanel title={t($ => $.observability.live)} description={t($ => $.observability.liveHint)}>
          <MetricRows rows={[
            { label: t($ => $.observability.installed), value: data.installations.total, href: "/admin/installations" },
            { label: t($ => $.observability.retired), value: data.installations.retired },
            { label: t($ => $.observability.clientActive), value: data.installations.clientActive },
            { label: t($ => $.observability.reachable), value: data.installations.daemonReachable },
            { label: t($ => $.observability.dispatched), value: data.executions.dispatched, href: live("dispatched") },
            { label: t($ => $.observability.unfinished), value: data.executions.unfinished },
          ]} />
        </OverviewPanel>
        <OverviewPanel title={t($ => $.observability.historyWindow)}>
          <ObservationFilters params={params} advanced />
        </OverviewPanel>
        <div className={styles.twoColumns}>
          <OverviewPanel title={t($ => $.observability.finished)} description={t($ => $.observability.finishedHint)}>
            <MetricRows rows={[
              { label: t($ => $.observability.completed), value: data.executions.completed, href: finished("completed") },
              { label: t($ => $.observability.failed), value: data.executions.failed, href: finished("failed") },
              { label: t($ => $.observability.cancelled), value: data.executions.cancelled, href: finished("cancelled") },
              { label: t($ => $.observability.successRate), value: data.executions.successRate === null ? t($ => $.observability.noSamples) : new Intl.NumberFormat(undefined, { style: "percent", maximumFractionDigits: 1 }).format(data.executions.successRate) },
            ]} />
          </OverviewPanel>
          <OverviewPanel title={t($ => $.observability.alertHistoryTitle)} description={t($ => $.observability.alertHistoryHint)}>
            <MetricRows rows={[
              { label: t($ => $.observability.resolvedAlerts), value: data.alerts.resolved, href: observationDrilldown("/admin/alerts", data.window, { status: "resolved" }) },
              { label: t($ => $.observability.closedAlerts), value: data.alerts.closed, href: observationDrilldown("/admin/alerts", data.window, { status: "closed" }) },
            ]} />
          </OverviewPanel>
        </div>
        <div className={styles.twoColumns}>
          <OverviewPanel title={t($ => $.observability.usage)} description={t($ => $.observability.tokensHint)}>
            <p className="mb-4 text-caption text-muted-foreground">{t($ => $.observability.quality[data.usage.quality])}</p>
            <MetricRows rows={[
              { label: t($ => $.observability.totalTokens), value: data.usage.totalTokens },
              { label: t($ => $.observability.inputTokens), value: data.usage.inputTokens },
              { label: t($ => $.observability.outputTokens), value: data.usage.outputTokens },
              { label: t($ => $.observability.cacheReadTokens), value: data.usage.cacheReadTokens },
              { label: t($ => $.observability.cacheWriteTokens), value: data.usage.cacheWriteTokens },
              { label: t($ => $.observability.missingUsage), value: data.usage.missingTasks },
              { label: t($ => $.observability.unpriced), value: data.usage.unpricedTasks },
            ]} />
          </OverviewPanel>
          <OverviewPanel title={t($ => $.observability.latency)}>
            <div className="mb-4 overflow-x-auto">
              <Table><TableHeader><TableRow>
                <TableHead>{t($ => $.observability.latency)}</TableHead>
                <TableHead>{t($ => $.observability.p50)}</TableHead>
                <TableHead>{t($ => $.observability.p95)}</TableHead>
                <TableHead>{t($ => $.observability.samples)}</TableHead>
              </TableRow></TableHeader><TableBody>
                {[{ label: t($ => $.observability.queueTime), ...data.executions.queueSeconds }, { label: t($ => $.observability.runTime), ...data.executions.runSeconds }].map(row => <TableRow key={row.label}>
                  <TableCell>{row.label}</TableCell><TableCell>{row.p50 ?? t($ => $.observability.unknown)}</TableCell>
                  <TableCell>{row.p95 ?? t($ => $.observability.unknown)}</TableCell><TableCell>{row.samples}</TableCell>
                </TableRow>)}
              </TableBody></Table>
            </div>
            <MetricRows rows={[
              { label: t($ => $.observability.lowerBound), value: data.executions.queueSeconds.lowerBoundSamples },
              { label: t($ => $.observability.unknownSamples), value: data.executions.queueSeconds.unknownSamples },
            ]} />
          </OverviewPanel>
        </div>
        <p className="text-caption text-muted-foreground">{t($ => $.observability.rules)}: {data.ruleVersion}</p>
      </>}
    </ObservationState>
  </section>;
}
