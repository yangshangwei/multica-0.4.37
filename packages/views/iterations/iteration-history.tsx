"use client";
import {
  Line,
  LineChart,
  ResponsiveContainer,
  XAxis,
  YAxis,
  Tooltip,
} from "recharts";
import { IterationEventsView } from "./iteration-events-view";
import { IterationReference, useIterationCatalogue } from "./iteration-catalogue";
import { formatInTimeZone } from "../common/format-in-time-zone";
import type { api } from "@multica/core/api";
import { useT, useLocale } from "../i18n";
type Detail = Awaited<ReturnType<typeof api.getIteration>>;
export function IterationHistory({
  statistics: currentStatistics,
  snapshot,
  timezone,
  wsId,
}: Pick<Detail, "statistics" | "snapshot"> & { timezone: string; wsId?: string }) {
  const { t } = useT("projects");
  const locale = useLocale();
  const catalogue = useIterationCatalogue(wsId ?? snapshot?.workspace_id ?? "", !!snapshot);
  const statistics = snapshot?.statistics ?? currentStatistics;
  const frozenIssues = new Map(snapshot?.scope.map((issue) => [issue.issue_id, issue]));
  const percent = (value: number | null) =>
    value === null
      ? t(($) => $.iterations.notApplicable)
      : new Intl.NumberFormat(locale, {
          style: "percent",
          maximumFractionDigits: 1,
        }).format(value);
  return (
    <section className="space-y-4">
      <h2 className="text-subtitle font-semibold">
        {t(($) => (snapshot ? $.iterations.history : $.iterations.scope))}
      </h2>
      {snapshot && (
        <p className="text-muted-foreground">
          {t(($) => $.iterations.readonly)}{" "}
          <time dateTime={snapshot.logical_ended_at} title={timezone}>
            {formatInTimeZone(snapshot.logical_ended_at, timezone, locale, {
              year: "numeric",
            })}
          </time>
        </p>
      )}
      <dl className="flex flex-wrap gap-6">
        {[
          [t(($) => $.iterations.currentCount), statistics.current],
          [t(($) => $.iterations.original), statistics.original],
          [t(($) => $.iterations.effective), statistics.effective],
          [t(($) => $.iterations.done), statistics.completed],
          [t(($) => $.iterations.remaining), statistics.remaining],
        ].map(([label, value]) => (
          <div key={label}>
            <dt className="text-caption text-muted-foreground">{label}</dt>
            <dd className="font-semibold tabular-nums">{value}</dd>
          </div>
        ))}
      </dl>
      <p className="text-muted-foreground">
        {t(($) => $.iterations.countHint)}
      </p>
      <dl className="flex flex-wrap gap-6">
        <div>
          <dt>{t(($) => $.iterations.effectiveRate)}</dt>
          <dd>{percent(statistics.effective_ratio)}</dd>
        </div>
        <div>
          <dt>{t(($) => $.iterations.originalRate)}</dt>
          <dd>{percent(statistics.original_ratio)}</dd>
        </div>
        <div>
          <dt>{t(($) => $.iterations.cancelledCount)}</dt>
          <dd>{statistics.cancelled}</dd>
        </div>
      </dl>
      <dl className="flex flex-wrap gap-6">
        {[
          [t(($) => $.iterations.addedCount), statistics.added_unique],
          [t(($) => $.iterations.removedCount), statistics.removed_events],
          [t(($) => $.iterations.reentryCount), statistics.reentry_events],
          [t(($) => $.iterations.cancelEvents), statistics.cancel_events],
          [t(($) => $.iterations.reopenEvents), statistics.reopen_events],
          [t(($) => $.iterations.startedCount), statistics.started],
          [t(($) => $.iterations.netChange), statistics.net_effective_change],
          [t(($) => $.iterations.netChangeRatio), percent(statistics.net_effective_change_ratio)],
        ].map(([label, value]) => <div key={label}><dt>{label}</dt><dd className="tabular-nums">{value}</dd></div>)}
      </dl>
      {snapshot && <div className="space-y-3">
        <dl className="space-y-2">
          <div><dt>{t(($) => $.iterations.endType)}</dt><dd>{snapshot.end_type === "completed" ? t(($) => $.iterations.completed) : snapshot.end_type === "cancelled" ? t(($) => $.iterations.cancelled) : snapshot.end_type}</dd></div>
          <div><dt>{t(($) => $.iterations.endReason)}</dt><dd className="whitespace-pre-wrap break-words">{snapshot.reason}</dd></div>
          <div><dt>{t(($) => $.iterations.processedAt)}</dt><dd><time dateTime={snapshot.processed_at} title={timezone}>{formatInTimeZone(snapshot.processed_at, timezone, locale, { year: "numeric" })}</time></dd></div>
        </dl>
        <details>
          <summary>{t(($) => $.iterations.destinations)}</summary>
          <p className="text-caption text-muted-foreground">{t(($) => $.iterations.currentNames)}</p>
          <ul className="mt-3 space-y-3">
            {snapshot.destinations.map((destination) => {
              const issue = frozenIssues.get(destination.issue_id);
              return <li key={destination.issue_id} className="break-words">
                <p>{issue ? `${issue.identifier} · ${issue.title}` : destination.issue_id}</p>
                <p>{t(($) => $.iterations.source)}: <IterationReference id={snapshot.iteration_id} catalogue={catalogue.data ?? []} /> · {t(($) => $.iterations.destination)}: <IterationReference id={destination.target_iteration_id} catalogue={catalogue.data ?? []} /></p>
                <p>{t(($) => $.iterations.rollover)}: {destination.rollover_count_before} → {destination.rollover_count_after}</p>
                {destination.rollover_count_after >= 3 && <p>{t(($) => $.iterations.rolloverReview)}</p>}
              </li>;
            })}
          </ul>
        </details>
      </div>}
      <div
        role="img"
        aria-label={t(($) => $.iterations.chart)}
        className="h-64 w-full"
      >
        <ResponsiveContainer>
          <LineChart data={statistics.chart}>
            <XAxis dataKey="date" />
            <YAxis />
            <Tooltip />
            <Line
              dataKey="effective"
              name={t(($) => $.iterations.effective)}
              stroke="var(--foreground)"
              dot={false}
            />
            <Line
              dataKey="completed"
              name={t(($) => $.iterations.done)}
              stroke="var(--primary)"
              dot={false}
            />
            <Line
              dataKey="original"
              name={t(($) => $.iterations.original)}
              stroke="var(--muted-foreground)"
              strokeDasharray="4 4"
              dot={false}
            />
          </LineChart>
        </ResponsiveContainer>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-body">
          <caption className="text-left font-medium mb-2">
            {t(($) => $.iterations.table)}
          </caption>
          <thead>
            <tr>
              {[
                t(($) => $.iterations.date),
                t(($) => $.iterations.effective),
                t(($) => $.iterations.done),
                t(($) => $.iterations.original),
              ].map((label) => (
                <th scope="col" className="p-2 text-left" key={label}>
                  {label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {statistics.chart.map((point) => (
              <tr key={point.date}>
                <th scope="row" className="p-2 text-left font-normal">
                  {point.date}
                </th>
                <td className="p-2">{point.effective}</td>
                <td className="p-2">{point.completed}</td>
                <td className="p-2">{point.original}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {snapshot && (
        <details>
          <summary>{t(($) => $.iterations.events)}</summary>
          <IterationEventsView wsId={snapshot.workspace_id} events={snapshot.events} timezone={timezone} />
        </details>
      )}
    </section>
  );
}
