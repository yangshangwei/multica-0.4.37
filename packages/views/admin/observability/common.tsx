"use client";
import { useState, type FormEvent, type ReactNode } from "react";
import { observationParams } from "@multica/core/admin";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { useT } from "../../i18n";
import { AppLink, useNavigation } from "../../navigation";
import { formatAdminTime } from "../executions/list-controls";

export function useObservationParams() {
  const nav = useNavigation();
  const [now] = useState(() => new Date());
  return observationParams(nav.searchParams, now);
}
export function ObservationHeader({ title, description, children }: { title: string; description: string; children?: ReactNode }) {
  return <header className="space-y-2"><h1 className="text-title font-semibold">{title}</h1><p className="max-w-prose text-body text-muted-foreground">{description}</p>{children}</header>;
}
export function ObservationFilters({ params, fields = [], windowed = true }: { params: URLSearchParams; fields?: { name: string; label: string; emptyLabel?: string; options?: { value: string; label: string }[] }[]; windowed?: boolean }) {
  const { t } = useT("admin");
  const nav = useNavigation();
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const next = new URLSearchParams();
    if (!windowed && params.has("timezone")) next.set("timezone", params.get("timezone")!);
    for (const [key, value] of new FormData(event.currentTarget)) if (typeof value === "string" && value.trim()) next.set(key, value.trim());
    nav.push(`${nav.pathname}${next.size ? `?${next}` : ""}`);
  }
  const inputs = windowed ? [...fields, { name: "time_from", label: t($ => $.observability.from) }, { name: "time_to", label: t($ => $.observability.to) }, { name: "timezone", label: t($ => $.observability.timezone) }] : fields;
  return <form key={params.toString()} onSubmit={submit} className="space-y-3">
    <div className="grid items-end gap-3 sm:grid-cols-2 lg:grid-cols-3">{inputs.map(field => <label key={field.name} className="min-w-0 space-y-1 text-caption">{field.label}{field.options ? <select name={field.name} defaultValue={params.get(field.name) ?? ""} className="h-10 w-full rounded-md border border-input bg-background px-3 text-body"><option value="">{field.emptyLabel ?? t($ => $.observability.all)}</option>{field.options.map(option => <option key={option.value} value={option.value}>{option.label}</option>)}</select> : <Input name={field.name} defaultValue={params.get(field.name) ?? ""} maxLength={128} />}</label>)}
      <div className="flex flex-wrap gap-2"><Button type="submit" variant="outline">{t($ => $.observability.apply)}</Button><Button type="button" variant="ghost" onClick={() => nav.push(nav.pathname)}>{t($ => $.observability.reset)}</Button></div>
    </div>{windowed && <p className="max-w-prose text-caption text-muted-foreground">{t($ => $.observability.windowHint)}</p>}
  </form>;
}
export function ObservationState({ pending, error, quality, asOf, empty = false, timezone = "UTC", retry, children }: {
  pending: boolean; error: boolean; quality?: "complete" | "partial" | "stale" | "unavailable" | "unknown"; asOf?: string; empty?: boolean; timezone?: string; retry(): void; children: ReactNode;
}) {
  const { t } = useT("admin");
  const nav = useNavigation();
  if (error || quality === "unavailable") return <section role="alert" className="space-y-3 py-8"><h2 className="text-title font-semibold">{t($ => $.observability.error)}</h2><p className="max-w-prose text-body text-muted-foreground">{t($ => $.observability.errorHint)}</p><Button variant="outline" onClick={retry}>{t($ => $.observability.refresh)}</Button></section>;
  if (pending) return <p role="status" className="py-8 text-body text-muted-foreground">{t($ => $.observability.loading)}</p>;
  return <>{quality && quality !== "complete" && <p role="status" className="text-body text-muted-foreground">{t($ => $.observability.quality[quality])}</p>}{asOf && <p className="text-caption text-muted-foreground">{t($ => $.observability.updated)} <time dateTime={asOf}>{formatAdminTime(asOf, timezone)}</time></p>}{empty ? <p className="py-8 text-body text-muted-foreground">{nav.searchParams.size ? t($ => $.observability.filteredEmpty) : t($ => $.observability.empty)}</p> : children}</>;
}
export function MetricRows({ rows }: { rows: { label: string; value: ReactNode; href?: string }[] }) {
  const { t } = useT("admin");
  return <dl className="grid gap-x-8 gap-y-4 sm:grid-cols-2 xl:grid-cols-3">{rows.map(row => <div key={row.label} className="flex min-w-0 items-baseline justify-between gap-4"><dt className="text-body text-muted-foreground">{row.href ? <AppLink href={row.href} className="underline underline-offset-4 hover:text-foreground">{row.label}</AppLink> : row.label}</dt><dd className="shrink-0 text-body font-medium tabular-nums">{row.value ?? t($ => $.observability.unknown)}</dd></div>)}</dl>;
}
