"use client";
import { IterationChartTable, IterationProgressChart } from "./iteration-progress";
import { IterationEventsView } from "./iteration-events-view";
import { IterationReference, useIterationCatalogue } from "./iteration-catalogue";
import { formatInTimeZone } from "../common/format-in-time-zone";
import type { api } from "@multica/core/api";
import { isIssueStatusCategory } from "@multica/core/issue-statuses";
import { useT, useLocale } from "../i18n";
import { iterationDisclosureClass } from "./iteration-presentation";
type Detail = Awaited<ReturnType<typeof api.getIteration>>;
export function IterationHistory({
  statistics: currentStatistics,
  snapshot,
  timezone,
  wsId,
  showSummary = true,
  showEvents = true,
}: Pick<Detail, "statistics" | "snapshot"> & { timezone: string; wsId?: string; showSummary?: boolean; showEvents?: boolean }) {
  const { t } = useT("projects");
  const locale = useLocale();
  const catalogue = useIterationCatalogue(wsId ?? snapshot?.workspace_id ?? "", !!snapshot);
  const statistics = snapshot?.statistics ?? currentStatistics;
  const frozenIssues = new Map(snapshot?.scope.map((issue) => [issue.issue_id, issue]));
  const destinations = new Map(snapshot?.destinations.map((destination) => [destination.issue_id, destination]));
  const unfinished = snapshot?.scope.filter((issue) =>
    isIssueStatusCategory(issue.status_category) && issue.status_category !== "done" && issue.status_category !== "cancelled",
  ) ?? [];
  const completeDestinations = snapshot?.scope.every((issue) => isIssueStatusCategory(issue.status_category)) &&
    unfinished.length === statistics.remaining && unfinished.every((issue) => destinations.has(issue.issue_id));
  const carried = unfinished.filter((issue) => destinations.get(issue.issue_id)?.target_iteration_id != null).length;
  const removed = unfinished.filter((issue) => destinations.get(issue.issue_id)?.target_iteration_id === null).length;
  const percent = (value: number | null, denominator: number) =>
    value === null || denominator === 0
      ? t(($) => $.iterations.notApplicable)
      : new Intl.NumberFormat(locale, {
          style: "percent",
          maximumFractionDigits: 1,
        }).format(value);
  return (
    <section className="min-w-0 space-y-7">
      {showSummary && <h2 className="text-title-sm font-semibold">
        {t(($) => (snapshot ? $.iterations.history : $.iterations.scope))}
      </h2>}
      {snapshot && showSummary && (
        <p className="text-caption text-muted-foreground">
          {t(($) => $.iterations.readonly)}{" "}
          <time dateTime={snapshot.logical_ended_at} title={timezone}>
            {formatInTimeZone(snapshot.logical_ended_at, timezone, locale, {
              year: "numeric",
            })}
          </time>
        </p>
      )}
      <section aria-label={t(($) => $.iterations.progressPanel.deliverySummary)}>
        <dl className="flex flex-wrap gap-x-10 gap-y-5">
          <div>
            <dt className="text-caption text-muted-foreground">{t(($) => $.iterations.effectiveRate)}</dt>
            <dd className="mt-1 flex flex-wrap items-baseline gap-x-3 gap-y-1 tabular-nums"><span className="text-title-lg font-semibold">{statistics.completed} / {statistics.effective}</span><span className="text-body">{percent(statistics.effective_ratio, statistics.effective)}</span></dd>
          </div>
          <div>
            <dt className="text-caption text-muted-foreground">{t(($) => $.iterations.originalRate)}</dt>
            <dd className="mt-1 flex flex-wrap items-baseline gap-x-3 gap-y-1 tabular-nums"><span className="text-title-lg font-semibold">{statistics.original_completed} / {statistics.original}</span><span className="text-body">{percent(statistics.original_ratio, statistics.original)}</span></dd>
          </div>
          <div>
            <dt className="text-caption text-muted-foreground">{t(($) => $.iterations.remaining)}</dt>
            <dd className="mt-1 text-title-lg font-semibold tabular-nums">{statistics.remaining}</dd>
          </div>
        </dl>
        <details className="mt-2">
          <summary className={iterationDisclosureClass}>{t(($) => $.iterations.progressPanel.countDetails)}</summary>
          <p className="mt-2 text-caption text-muted-foreground">{t(($) => $.iterations.countHint)} {t(($) => $.iterations.progressPanel.effectiveScopeHint)}</p>
          <dl className="mt-4 grid grid-cols-2 gap-x-8 gap-y-3 text-body @2xl:grid-cols-4">
            {[
              [t(($) => snapshot ? $.iterations.progressPanel.scopeAtClosure : $.iterations.currentCount), statistics.current],
              [t(($) => $.iterations.original), statistics.original],
              [t(($) => $.iterations.effective), statistics.effective],
              [t(($) => $.iterations.done), statistics.completed],
              [t(($) => $.iterations.progressPanel.originalCompleted), statistics.original_completed],
              [t(($) => $.iterations.remaining), statistics.remaining],
              [t(($) => $.iterations.cancelledCount), statistics.cancelled],
              [t(($) => $.iterations.startedCount), statistics.started],
            ].map(([label, value]) => <div key={label}><dt className="text-caption text-muted-foreground">{label}</dt><dd className="mt-1 tabular-nums">{value}</dd></div>)}
          </dl>
        </details>
      </section>
      {snapshot && <section aria-label={t(($) => $.iterations.progressPanel.unfinishedDestinations)} className="space-y-3">
        <h2 className="text-title-sm font-medium">{t(($) => $.iterations.progressPanel.unfinishedDestinations)}</h2>
        {statistics.remaining === 0 ? <p className="text-body text-muted-foreground">{t(($) => $.iterations.progressPanel.noUnfinished)}</p> : completeDestinations ? <dl className="flex flex-wrap gap-x-6 gap-y-2 text-caption">
          <div className="flex items-baseline gap-2"><dt className="text-muted-foreground">{t(($) => $.iterations.progressPanel.carriedAtClosure)}</dt><dd className="font-medium tabular-nums">{carried}</dd></div>
          <div className="flex items-baseline gap-2"><dt className="text-muted-foreground">{t(($) => $.iterations.progressPanel.removedAtClosure)}</dt><dd className="font-medium tabular-nums">{removed}</dd></div>
        </dl> : <p className="text-caption text-muted-foreground">{t(($) => $.iterations.progressPanel.missingDestinations)}</p>}
        {unfinished.length > 0 && <ul aria-label={t(($) => $.iterations.progressPanel.unfinishedDestinations)} className="space-y-2 text-body">
          {unfinished.map((issue) => {
            const destination = destinations.get(issue.issue_id);
            return <li key={issue.issue_id} className="flex flex-wrap items-baseline gap-x-4 gap-y-1 [overflow-wrap:anywhere]">
              <span>{issue.identifier} · {issue.title}</span>
              <span className="text-caption text-muted-foreground [&_a]:underline [&_a]:underline-offset-4">{!destination ? t(($) => $.iterations.unknownHistory) : destination.target_iteration_id === null ? t(($) => $.iterations.progressPanel.removed) : <>{t(($) => $.iterations.progressPanel.carriedTo)} <IterationReference id={destination.target_iteration_id} catalogue={catalogue.data ?? []} /></>}</span>
            </li>;
          })}
        </ul>}
        <details>
          <summary className={iterationDisclosureClass}>{t(($) => $.iterations.progressPanel.closeoutDetails)}</summary>
          <dl className="mt-3 space-y-3 text-body">
            <div><dt className="text-caption text-muted-foreground">{t(($) => $.iterations.endType)}</dt><dd>{snapshot.end_type === "completed" ? t(($) => $.iterations.completed) : snapshot.end_type === "cancelled" ? t(($) => $.iterations.cancelled) : snapshot.end_type}</dd></div>
            <div><dt className="text-caption text-muted-foreground">{t(($) => $.iterations.endReason)}</dt><dd className="whitespace-pre-wrap [overflow-wrap:anywhere]">{snapshot.reason}</dd></div>
            <div><dt className="text-caption text-muted-foreground">{t(($) => $.iterations.progressPanel.closedAt)}</dt><dd><time dateTime={snapshot.logical_ended_at} title={timezone}>{formatInTimeZone(snapshot.logical_ended_at, timezone, locale, { year: "numeric" })}</time></dd></div>
            <div><dt className="text-caption text-muted-foreground">{t(($) => $.iterations.processedAt)}</dt><dd><time dateTime={snapshot.processed_at} title={timezone}>{formatInTimeZone(snapshot.processed_at, timezone, locale, { year: "numeric" })}</time></dd></div>
          </dl>
          <h3 className="mb-1 mt-5 text-body font-medium">{t(($) => $.iterations.destinations)}</h3>
          <p className="text-caption text-muted-foreground">{t(($) => $.iterations.currentNames)}</p>
          <ul className="mt-3 space-y-3 text-body [&_a]:underline [&_a]:underline-offset-4">
            {snapshot.destinations.map((destination) => {
              const issue = frozenIssues.get(destination.issue_id);
              return <li key={destination.issue_id} className="[overflow-wrap:anywhere]">
                <p>{issue ? `${issue.identifier} · ${issue.title}` : destination.issue_id}</p>
                <p>{t(($) => $.iterations.source)}: <IterationReference id={snapshot.iteration_id} catalogue={catalogue.data ?? []} /> · {t(($) => $.iterations.destination)}: <IterationReference id={destination.target_iteration_id} catalogue={catalogue.data ?? []} /></p>
                <p>{t(($) => $.iterations.rollover)}: {destination.rollover_count_before} → {destination.rollover_count_after}</p>
                {destination.rollover_count_after >= 3 && <p>{t(($) => $.iterations.rolloverReview)}</p>}
              </li>;
            })}
          </ul>
        </details>
      </section>}
      <div>
        <h2 className="text-title-sm font-medium">{t(($) => $.iterations.chart)}</h2>
        <IterationProgressChart statistics={statistics} showCompletionRate={false} />
        <details>
          <summary className={iterationDisclosureClass}>{t(($) => $.iterations.pages.viewChartData)}</summary>
          <IterationChartTable statistics={statistics} />
        </details>
      </div>
      {snapshot && showEvents && (
        <details>
          <summary className={iterationDisclosureClass}>{t(($) => $.iterations.events)}</summary>
          <IterationEventsView wsId={snapshot.workspace_id} events={snapshot.events} timezone={timezone} />
        </details>
      )}
    </section>
  );
}
