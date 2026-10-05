"use client";
import { useId, useState, type FormEvent, type ReactNode } from "react";
import { normalizeAdminFilters, observationParams } from "@multica/core/admin";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Info } from "lucide-react";
import { AdminFilterSummary } from "../filter-summary";
import { useT } from "../../i18n";
import { AppLink, useNavigation } from "../../navigation";
import { adminTouchLinkClass, formatAdminTime } from "../executions/list-controls";
import { AdminTimezoneOptions, adminFilterInputProps, adminSelectClass, useAdminFilterFeedback } from "../filter-feedback";

export function useObservationParams() {
  const nav = useNavigation();
  const [now] = useState(() => new Date());
  return observationParams(nav.searchParams, now);
}
export function ObservationHeader({ title, description, children }: { title: string; description: string; children?: ReactNode }) {
  return <header className="space-y-2"><h1 className="text-title-lg font-semibold">{title}</h1><p className="max-w-prose text-body text-muted-foreground">{description}</p>{children}</header>;
}
type ObservationField = { name: string; label: string; emptyLabel?: string; options?: { value: string; label: string }[] };
const quickRanges = [["last24Hours", 1], ["last7Days", 7], ["last30Days", 30]] as const;
const windowFields = new Set(["time_from", "time_to", "timezone"]);
/** `advanced` keeps choice filters and quick ranges visible and moves typed identifiers and the exact window into a disclosure. */
export function ObservationFilters({ params, fields = [], windowed = true, advanced = false, maxWindowDays = 31 }: {
  params: URLSearchParams; fields?: ObservationField[]; windowed?: boolean; advanced?: boolean; maxWindowDays?: number;
}) {
  const { t } = useT("admin");
  const nav = useNavigation();
  const feedback = useAdminFilterFeedback(params);
  const id = useId();
  const zones = `${id}-zones`;
  function push(next: URLSearchParams) {
    nav.push(`${nav.pathname}${next.size ? `?${next}` : ""}`);
  }
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const next = normalizeAdminFilters(new FormData(event.currentTarget));
    if (!feedback.validate(next, event.currentTarget, { maxWindowDays })) return;
    if (!windowed && params.has("timezone")) next.set("timezone", params.get("timezone")!);
    push(next);
  }
  function quickRange(days: number) {
    const next = new URLSearchParams(nav.searchParams);
    const now = Date.now();
    next.delete("cursor");
    next.set("time_from", new Date(now - days * 86_400_000).toISOString());
    next.set("time_to", new Date(now).toISOString());
    push(next);
  }
  const inputs: ObservationField[] = windowed ? [...fields, { name: "time_from", label: t($ => $.observability.from) }, { name: "time_to", label: t($ => $.observability.to) }, { name: "timezone", label: t($ => $.observability.timezone) }] : fields;
  const collapsed = advanced ? inputs.filter(field => !field.options) : [];
  const visible = advanced ? inputs.filter(field => field.options) : inputs;
  const renderField = (field: ObservationField) => <div key={field.name} className="min-w-0 space-y-1.5">
    <label htmlFor={`${id}-${field.name}`} className="block text-caption">{field.label}</label>
    {field.options
      ? <select id={`${id}-${field.name}`} name={field.name} defaultValue={params.get(field.name) ?? ""} className={adminSelectClass}><option value="">{field.emptyLabel ?? t($ => $.observability.all)}</option>{field.options.map(option => <option key={option.value} value={option.value}>{option.label}</option>)}</select>
      : <Input id={`${id}-${field.name}`} name={field.name} maxLength={128} {...adminFilterInputProps(field.name, params, zones)} {...feedback.fieldProps(field.name)} />}
    {feedback.message(field.name)}
  </div>;
  const actions = <div className="flex flex-wrap gap-2"><Button type="submit">{t($ => $.observability.apply)}</Button><Button type="reset" variant="ghost">{t($ => $.observability.reset)}</Button></div>;
  const presets = windowed && <div role="group" aria-label={t($ => $.observability.window)} className="flex flex-wrap gap-2">
    {quickRanges.map(([key, days]) => <Button key={key} type="button" variant="ghost" onClick={() => quickRange(days)}>{t($ => $.filters[key])}</Button>)}
  </div>;
  const hint = windowed && <p className="max-w-prose text-caption text-muted-foreground">{t($ => $.observability.windowHint, { days: maxWindowDays })}</p>;
  return <form key={params.toString()} noValidate onSubmit={submit} onReset={() => { feedback.reset(); nav.push(nav.pathname); }} className="space-y-3">
    {(visible.length > 0 || !advanced) && <div className="grid items-end gap-3 sm:grid-cols-2 lg:grid-cols-3">{visible.map(renderField)}{!advanced && actions}</div>}
    {advanced ? <>
      <div className="flex flex-wrap items-center justify-between gap-3">{presets}{actions}</div>
      {collapsed.length > 0 && <details open={collapsed.some(field => nav.searchParams.has(field.name)) || undefined}>
        <summary className="w-fit cursor-pointer text-body text-muted-foreground">{collapsed.every(field => windowFields.has(field.name)) ? t($ => $.observability.window) : t($ => $.filters.more)}</summary>
        <div className="mt-3 space-y-3"><div className="grid items-end gap-3 sm:grid-cols-2 lg:grid-cols-3">{collapsed.map(renderField)}</div>{hint}</div>
      </details>}
    </> : <>{presets}{hint}</>}
    <AdminFilterSummary params={nav.searchParams} fields={inputs} />
    {windowed && <AdminTimezoneOptions id={zones} />}
  </form>;
}
export function ObservationState({ pending, error, quality, asOf, empty = false, timezone = "UTC", retry, children }: {
  pending: boolean; error: boolean; quality?: "complete" | "partial" | "stale" | "unavailable" | "unknown"; asOf?: string; empty?: boolean; timezone?: string; retry(): void; children: ReactNode;
}) {
  const { t } = useT("admin");
  const nav = useNavigation();
  if (error || quality === "unavailable") return <section role="alert" className="space-y-3 py-8"><h2 className="text-title font-semibold">{t($ => $.observability.error)}</h2><p className="max-w-prose text-body text-muted-foreground">{t($ => $.observability.errorHint)}</p><Button variant="outline" onClick={retry}>{t($ => $.observability.refresh)}</Button></section>;
  if (pending) return <p role="status" className="py-8 text-body text-muted-foreground">{t($ => $.observability.loading)}</p>;
  return <>
    {(quality && quality !== "complete" || asOf) && (
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-caption text-muted-foreground">
        {quality && quality !== "complete" && <p role="status" className="inline-flex items-center gap-1.5 rounded-md border border-surface-border px-2 py-1">
          <Info className="size-3.5 shrink-0" aria-hidden="true" />
          {t($ => $.observability.quality[quality])}
        </p>}
        {asOf && <p>{t($ => $.observability.updated)} <time dateTime={asOf}>{formatAdminTime(asOf, timezone)}</time></p>}
      </div>
    )}
    {empty ? <p className="py-8 text-body text-muted-foreground">{nav.searchParams.size ? t($ => $.observability.filteredEmpty) : t($ => $.observability.empty)}</p> : children}
  </>;
}
export function MetricRows({ rows, layout = "metrics" }: { rows: { label: string; value: ReactNode; href?: string }[]; layout?: "metrics" | "settings" }) {
  const { t } = useT("admin");
  return <dl className={layout === "settings" ? "grid gap-x-10 gap-y-5 sm:grid-cols-2" : "grid gap-x-8 gap-y-4 sm:grid-cols-2 xl:grid-cols-3"}>
    {rows.map(row => <div key={row.label} className={layout === "settings" ? "flex min-w-0 flex-col gap-1.5" : "flex min-w-0 items-baseline justify-between gap-4"}>
      <dt className="text-body text-muted-foreground">{row.href ? <AppLink href={row.href} className={`inline-block underline underline-offset-4 hover:text-foreground ${adminTouchLinkClass}`}>{row.label}</AppLink> : row.label}</dt>
      <dd className={layout === "settings" ? "text-body font-medium tabular-nums [overflow-wrap:anywhere]" : "shrink-0 text-body font-medium tabular-nums"}>{row.value ?? t($ => $.observability.unknown)}</dd>
    </div>)}
  </dl>;
}
