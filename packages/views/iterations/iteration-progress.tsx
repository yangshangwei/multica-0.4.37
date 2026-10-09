"use client";

import { Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import type { api } from "@multica/core/api";
import { useT, useLocale } from "../i18n";

type Statistics = Awaited<ReturnType<typeof api.getIteration>>["statistics"];

export function IterationChartTable({ statistics }: { statistics: Statistics }) {
  const { t } = useT("projects");
  return (
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
  );
}

export function IterationProgressChart({ statistics, showCompletionRate = true }: { statistics: Statistics; showCompletionRate?: boolean }) {
  const { t } = useT("projects");
  const locale = useLocale();
  const formatDate = (date: string) => new Intl.DateTimeFormat(locale, { month: "short", day: "numeric", timeZone: "UTC" }).format(new Date(`${date}T00:00:00Z`));
  const singlePoint = statistics.chart.length === 1 ? statistics.chart[0] : undefined;
  const compact = statistics.chart.length < 2;
  const series = [
    { key: "effective", label: t(($) => $.iterations.effective), color: "var(--muted-foreground)", width: 2, dash: undefined },
    { key: "completed", label: t(($) => $.iterations.done), color: "var(--chart-1)", width: 2.5, dash: undefined },
    { key: "original", label: t(($) => $.iterations.original), color: "var(--foreground)", width: 2, dash: "4 4" },
  ] as const;
  const legend = <dl className={`flex flex-wrap gap-x-6 gap-y-3 text-caption ${compact ? "" : "@2xl:flex-col @2xl:gap-4"}`}>
    {series.map((item) => <div key={item.key} className="flex items-center gap-2">
      {!compact && <svg aria-hidden width="20" height="10" className="shrink-0"><line x1="0" y1="5" x2="20" y2="5" stroke={item.color} strokeWidth={item.width} strokeDasharray={item.dash} /></svg>}
      <dt className="text-muted-foreground">{item.label}</dt><dd className="ml-auto font-medium tabular-nums">{(singlePoint ?? statistics)[item.key]}</dd>
    </div>)}
    {showCompletionRate && <div><dt className="text-muted-foreground">{t(($) => $.iterations.effectiveRate)}</dt><dd className="font-medium tabular-nums">{statistics.effective_ratio === null || statistics.effective === 0 ? t(($) => $.iterations.notApplicable) : new Intl.NumberFormat(locale, { style: "percent", maximumFractionDigits: 1 }).format(statistics.effective_ratio)}</dd></div>}
  </dl>;

  if (statistics.chart.length === 0) return <div className="space-y-3 py-3">
    <p className="text-caption text-muted-foreground">{t(($) => $.iterations.progressPanel.noChartData)}</p>
    {showCompletionRate && legend}
  </div>;

  if (singlePoint) return <section aria-label={t(($) => $.iterations.chart)} className="space-y-3 py-3">
    <p className="flex flex-wrap gap-x-2 gap-y-1 text-caption text-muted-foreground"><time dateTime={singlePoint.date}>{formatDate(singlePoint.date)}</time><span>{t(($) => $.iterations.progressPanel.singleDay)}</span></p>
    {legend}
  </section>;

  return <div className="grid min-w-0 items-center gap-5 py-3 @2xl:grid-cols-[minmax(0,1fr)_10rem]">
    <div role="img" aria-label={t(($) => $.iterations.chart)} className="h-60 min-w-0 w-full">
      <ResponsiveContainer>
        <LineChart data={statistics.chart} margin={{ top: 12, right: 8, bottom: 8, left: -12 }}>
          <XAxis dataKey="date" tickFormatter={formatDate} tickLine={false} axisLine={false} minTickGap={45} tick={{ fontSize: 12, fill: "var(--muted-foreground)" }} />
          <YAxis allowDecimals={false} tickLine={false} axisLine={false} tick={{ fontSize: 12, fill: "var(--muted-foreground)" }} />
          <Tooltip labelFormatter={(value) => typeof value === "string" ? formatDate(value) : value} contentStyle={{ background: "var(--popover)", color: "var(--popover-foreground)", border: "1px solid var(--border)", borderRadius: 8 }} />
          {series.map((item) => <Line key={item.key} dataKey={item.key} name={item.label} stroke={item.color} strokeWidth={item.width} strokeDasharray={item.dash} dot={false} isAnimationActive={false} />)}
        </LineChart>
      </ResponsiveContainer>
    </div>
    {legend}
  </div>;
}
