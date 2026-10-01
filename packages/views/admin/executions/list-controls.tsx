"use client";
import type { ReactNode } from "react";
import { executionSources, executionStatuses } from "@multica/core/admin";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { useT } from "../../i18n";
import { AppLink, useNavigation } from "../../navigation";
export function AdminExecutionFilters({ issues = false }: {
  issues?: boolean;
}) {
  const { t } = useT("admin");
  const nav = useNavigation();
  const params = nav.searchParams;
  const textFields: [string, string][] = [
    ["workspace_id", t($ => $.executions.workspace)],
    ["issue_id", t($ => $.executions.issue)],
    ["time_from", t($ => $.executions.from)],
    ["time_to", t($ => $.executions.to)],
    ["timezone", t($ => $.executions.timezone)],
  ];
  if (!issues) textFields.push(
    ["runtime_id", t($ => $.executions.runtime)],
    ["user_id", t($ => $.executions.user)],
    ["installation_id", t($ => $.executions.installation)],
  );
  function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const next = new URLSearchParams();
    for (const [key, value] of new FormData(event.currentTarget)) {
      if (typeof value === "string" && value.trim())
        next.set(key, value.trim());
    }
    if (!issues) {
      const basis = params.get("time_basis");
      if (basis === "created" || basis === "finished") next.set("time_basis", basis);
      if (params.get("state_scope") === "current" && basis !== "finished" &&
          ["queued", "deferred", "dispatched", "running", "waiting_local_directory"].includes(next.get("status") ?? "") &&
          !next.has("time_from") && !next.has("time_to")) next.set("state_scope", "current");
    }
    nav.push(`${nav.pathname}${next.size ? `?${next}` : ""}`);
  }
  return <form key={params.toString()} onSubmit={submit} className="space-y-4">
    {!issues && (params.get("time_basis") === "finished" || params.get("state_scope") === "current") && <p className="text-caption text-muted-foreground">{params.get("time_basis") === "finished" ? t($ => $.observability.finished) : t($ => $.observability.live)}</p>}
    <div className="flex flex-wrap items-end gap-3">
      <div className="min-w-48 flex-1 space-y-1.5">
        <Label htmlFor="admin-search">{t($ => $.executions.search)}</Label>
        <Input id="admin-search" name="q" defaultValue={params.get("q") ?? ""} />
      </div>
      <div className="min-w-36 space-y-1.5">
        <Label htmlFor="admin-status">{t($ => $.executions.status)}</Label>{issues ? <Input id="admin-status" name="status" defaultValue={params.get("status") ?? ""} /> : <select id="admin-status" name="status" defaultValue={params.get("status") ?? ""} className="h-9 w-full rounded-md border border-input bg-background px-3 text-body">
          <option value="">{t($ => $.executions.all)}</option>{executionStatuses.filter(s => s !== "unknown").map(s => <option key={s} value={s}>{t($ => $.executions.statuses[s])}</option>)}</select>}</div>
      {!issues && <div className="min-w-40 space-y-1.5">
        <Label htmlFor="admin-source">{t($ => $.executions.source)}</Label>
        <select id="admin-source" name="source" defaultValue={params.get("source") ?? ""} className="h-9 w-full rounded-md border border-input bg-background px-3 text-body">
          <option value="">{t($ => $.executions.all)}</option>{executionSources.map(s => <option key={s} value={s}>{t($ => $.executions.sources[s])}</option>)}</select>
      </div>}
      <Button type="submit">{t($ => $.executions.apply)}</Button>
      <Button type="button" variant="ghost" onClick={() => nav.push(nav.pathname)}>{t($ => $.executions.reset)}</Button>
    </div>
    <details open={textFields.some(([name]) => params.has(name ?? "")) || undefined} className="text-body">
      <summary className="cursor-pointer text-muted-foreground">{t($ => $.executions.filters)}</summary>
      <div className="mt-3 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">{textFields.map(([name, label]) => <div key={name} className="space-y-1.5">
        <Label htmlFor={`filter-${name}`}>{label}</Label>
        <Input id={`filter-${name}`} name={name} defaultValue={params.get(name ?? "") ?? ""} placeholder={name?.startsWith("time_") ? "2026-10-01T00:00:00Z" : name === "timezone" ? "UTC" : undefined} />
      </div>)}</div>
    </details>
  </form>;
}
export function AdminExecutionTabs() {
  const { t } = useT("admin");
  const nav = useNavigation();
  return <nav aria-label={t($ => $.executions.title)} className="flex gap-5 border-b border-surface-border pb-3">
    <AppLink href="/admin/tasks" aria-current={nav.pathname.startsWith("/admin/tasks") ? "page" : undefined} className="text-body aria-[current=page]:font-semibold">{t($ => $.executions.executions)}</AppLink>
    <AppLink href="/admin/issues" aria-current={nav.pathname === "/admin/issues" ? "page" : undefined} className="text-body aria-[current=page]:font-semibold">{t($ => $.executions.issues)}</AppLink>
  </nav>;
}
export function AdminListState({ loading, error, empty, issues = false, retry, children }: {
  loading: boolean;
  error: boolean;
  empty: boolean;
  issues?: boolean;
  retry: () => void;
  children: ReactNode;
}) {
  const { t } = useT("admin");
  const nav = useNavigation();
  if (error)
    return <section role="alert" className="space-y-3 py-8">
      <h2 className="text-title font-semibold">{t($ => $.executions.unavailable)}</h2>
      <p className="text-body text-muted-foreground">{t($ => $.executions.unavailable_hint)}</p>
      <Button variant="outline" onClick={retry}>{t($ => $.executions.retry)}</Button>
    </section>;
  if (loading)
    return <p role="status" className="py-8 text-body text-muted-foreground">{t($ => $.executions.loading)}</p>;
  if (empty)
    return <div className="space-y-2 py-10">
      <p className="text-body font-medium">{nav.searchParams.size ? t($ => $.executions.filtered_empty) : issues ? t($ => $.executions.issue_empty) : t($ => $.executions.empty)}</p>
      <p className="text-body text-muted-foreground">{t($ => $.executions.empty_hint)}</p>
    </div>;
  return children;
}
export function AdminListPagination({ cursor, asOf, refresh, params: suppliedParams }: {
  cursor: string | null;
  asOf: string;
  refresh: () => void;
  params?: URLSearchParams;
}) {
  const { t } = useT("admin");
  const nav = useNavigation();
  const activeParams = suppliedParams ?? nav.searchParams;
  function move(next: string | null) {
    const params = new URLSearchParams(activeParams);
    params.delete("cursor");
    if (next)
      params.set("cursor", next);
    nav.push(`${nav.pathname}${params.size ? `?${params}` : ""}`);
  }
  return <div className="space-y-3">
    <div className="flex flex-wrap items-center justify-between gap-3">
      <p className="text-caption text-muted-foreground">{t($ => $.executions.as_of)} <time dateTime={asOf}>{formatAdminTime(asOf, activeParams.get("timezone") ?? "UTC")}</time>
      </p>
      <div className="flex gap-2">
        <Button variant="ghost" onClick={() => {
          if (activeParams.has("cursor"))
            move(null);
          else
            refresh();
        }}>{t($ => $.executions.refresh)}</Button>
        <Button variant="outline" disabled={!activeParams.has("cursor")} onClick={() => move(null)}>{t($ => $.executions.first)}</Button>
        <Button variant="outline" disabled={!cursor} onClick={() => move(cursor)}>{t($ => $.executions.next)}</Button>
      </div>
    </div>
    <p className="max-w-3xl text-caption text-muted-foreground">{t($ => $.executions.live_notice)}</p>
  </div>;
}
export function formatAdminTime(value: string, timezone = "UTC") {
  try {
    return new Intl.DateTimeFormat(undefined, {
      dateStyle: "medium", timeStyle: "short", timeZone: timezone
    }).format(new Date(value));
  }
  catch {
    return value;
  }
}
