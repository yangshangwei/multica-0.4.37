"use client";

import type { FormEvent } from "react";
import { adminApiScope, adminDetailHref, normalizeAdminFilters, useAdminAccess, useAdminUsers, type AdminUserFilters } from "@multica/core/admin";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@multica/ui/components/ui/table";
import { AppLink, useNavigation } from "../../navigation";
import { useT } from "../../i18n";
import { adminTouchLinkClass, formatAdminTime } from "../executions/list-controls";
import { AdminFilterSummary } from "../filter-summary";
import styles from "../admin-visual.module.css";
import { adminSelectClass, useAdminFilterFeedback } from "../filter-feedback";

export function AdminUsersPage({ administrators = false }: { administrators?: boolean }) {
  const { t } = useT("admin");
  const navigation = useNavigation();
  const feedback = useAdminFilterFeedback(navigation.searchParams);
  const { identity } = useAdminAccess();
  const params = navigation.searchParams;
  const filters: AdminUserFilters = {
    q: params.get("q") || undefined,
    status: params.get("status") || undefined,
    role: params.get("role") || (administrators ? "administrators" : undefined),
    timeFrom: params.get("time_from") || undefined,
    timeTo: params.get("time_to") || undefined,
    cursor: params.get("cursor") || undefined,
    limit: 50,
    timezone: params.get("timezone") || undefined,
  };
  const query = useAdminUsers({ apiScope: adminApiScope(), userId: identity?.userId ?? "", organizationId: identity?.organizationId ?? null }, filters);
  const statuses = {
    active: t(($) => $.users.statusActive), disabled: t(($) => $.users.statusDisabled),
    setup_required: t(($) => $.users.statusSetup), password_change_required: t(($) => $.users.statusChange),
    unknown: t(($) => $.users.statusUnknown),
  };
  const roleLabel = (role: string | null) => role === "super_admin" ? t(($) => $.users.superAdmin) : role === "platform_observer" ? t(($) => $.users.observer) : t(($) => $.users.noRole);
  function filter(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const next = normalizeAdminFilters(new FormData(event.currentTarget), { dateOnly: true });
    if (!feedback.validate(next, event.currentTarget, { maxWindowDays: null })) return;
    navigation.replace(`${navigation.pathname}${next.size ? `?${next}` : ""}`);
  }
  function page(cursor?: string) {
    const next = new URLSearchParams(params);
    if (cursor) next.set("cursor", cursor); else next.delete("cursor");
    navigation.push(`${navigation.pathname}?${next}`);
  }
  const timezone = filters.timezone || "UTC";
  const endDate = filters.timeTo ? new Date(new Date(filters.timeTo).getTime() - 1) : null;
  const inclusiveEnd = endDate && Number.isFinite(endDate.getTime()) ? endDate.toISOString().slice(0, 10) : "";
  const detailHref = (id: string) => adminDetailHref(`/admin/users/${id}`, navigation.pathname, params);
  const title = administrators ? t(($) => $.users.adminTitle) : t(($) => $.users.title);
  return (
    <section className="space-y-6" aria-labelledby="admin-users-title">
      <div className="space-y-2">
        <h1 id="admin-users-title" className="text-title font-semibold">{title}</h1>
        <p className="max-w-prose text-body text-muted-foreground">{t(($) => $.users.description)}</p>
      </div>
      <form key={params.toString()} onSubmit={filter} onReset={() => { feedback.reset(); navigation.replace(navigation.pathname); }} noValidate className="space-y-3">
        <div className="grid items-end gap-3 sm:grid-cols-2 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)_auto]">
          <label className="min-w-0 space-y-1 text-caption">{t(($) => $.users.search)}<Input name="q" defaultValue={filters.q} maxLength={128} {...feedback.fieldProps("q")} />{feedback.message("q")}</label>
          <label className="min-w-0 space-y-1 text-caption">{t(($) => $.users.status)}<select name="status" defaultValue={filters.status ?? ""} className={adminSelectClass}><option value="">{t(($) => $.users.allStates)}</option>{Object.entries(statuses).filter(([value]) => value !== "unknown").map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
          <div className="flex gap-2 self-end"><Button type="submit">{t(($) => $.users.filter)}</Button><Button type="reset" variant="ghost">{t(($) => $.executions.reset)}</Button></div>
        </div>
        <details open={["role", "time_from", "time_to", "timezone"].some(name => params.has(name)) || undefined}>
          <summary className="cursor-pointer text-body text-muted-foreground">{t(($) => $.executions.filters)}</summary>
          <div className="mt-3 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <label className="min-w-0 space-y-1 text-caption">{t(($) => $.users.role)}<select name="role" defaultValue={filters.role ?? ""} className={adminSelectClass}><option value={administrators ? "administrators" : ""}>{t(($) => $.users.allRoles)}</option><option value="super_admin">{t(($) => $.users.superAdmin)}</option><option value="platform_observer">{t(($) => $.users.observer)}</option></select></label>
            <label className="min-w-0 space-y-1 text-caption">{t(($) => $.users.timeFrom)}<Input type="date" name="time_from" defaultValue={filters.timeFrom?.slice(0, 10)} {...feedback.fieldProps("time_from")} />{feedback.message("time_from")}</label>
            <label className="min-w-0 space-y-1 text-caption">{t(($) => $.users.timeTo)}<Input type="date" name="time_to" defaultValue={inclusiveEnd} {...feedback.fieldProps("time_to")} />{feedback.message("time_to")}</label>
            <label className="min-w-0 space-y-1 text-caption">{t(($) => $.executions.timezone)}<Input name="timezone" defaultValue={timezone} {...feedback.fieldProps("timezone")} />{feedback.message("timezone")}</label>
          </div>
          <p className="mt-2 text-caption text-muted-foreground">{t(($) => $.users.dateRangeHint)}</p>
        </details>
        <AdminFilterSummary params={params} fields={[
          { name: "q", label: t($ => $.users.search) },
          { name: "status", label: t($ => $.users.status), options: Object.entries(statuses).map(([value, label]) => ({ value, label })) },
          { name: "role", label: t($ => $.users.role), options: [{ value: "super_admin", label: t($ => $.users.superAdmin) }, { value: "platform_observer", label: t($ => $.users.observer) }, { value: "administrators", label: t($ => $.users.adminTitle) }] },
          { name: "time_from", label: t($ => $.users.timeFrom), displayValue: filters.timeFrom?.slice(0, 10) },
          { name: "time_to", label: t($ => $.users.timeTo), displayValue: inclusiveEnd },
          { name: "timezone", label: t($ => $.executions.timezone) },
        ]} />
      </form>
      {query.isPending ? <p role="status">{t(($) => $.state.loading)}</p> : query.isError ? (
        <div role="alert" className="space-y-3"><p>{t(($) => $.users.loadError)}</p><Button variant="outline" onClick={() => void query.refetch()}>{t(($) => $.users.retry)}</Button></div>
      ) : query.data ? <>
        <p className="text-caption text-muted-foreground">{t(($) => $.users.updated)}: <time dateTime={query.data.asOf}>{formatAdminTime(query.data.asOf, timezone)}</time></p>
        {query.data.items.length === 0 ? <p className="py-8 text-body text-muted-foreground">{query.data.dataQuality === "partial" ? t(($) => $.users.partial) : filters.q || filters.status || filters.role || filters.timeFrom || filters.timeTo ? t(($) => $.users.filteredEmpty) : t(($) => $.users.empty)}</p> : <>
          <ul aria-label={title} className="divide-y divide-surface-border md:hidden">
            {query.data.items.map(user => <li key={user.id} className={`${styles.compactRow} space-y-1 first:pt-0`}>
              <div className="flex items-start justify-between gap-3">
                <AppLink href={detailHref(user.id)} aria-label={user.name} aria-describedby={`compact-account-${user.id}`} className="flex min-h-11 min-w-11 flex-col justify-center text-body font-medium [overflow-wrap:anywhere] underline-offset-4 hover:underline focus-visible:outline-ring">
                  <span>{user.name}</span><span id={`compact-account-${user.id}`} className="text-caption font-normal text-muted-foreground">{user.username ?? "—"}</span>
                </AppLink>
                <span className={`${styles.statusBadge} ${styles.statusNeutral} max-w-[50%]`}>{statuses[user.status]}</span>
              </div>
              <details><summary className="cursor-pointer text-caption text-muted-foreground">{t(($) => $.users.moreDetails)}</summary>
                <dl className="mt-2 space-y-3 text-body">
                  <div><dt className="text-caption text-muted-foreground">{t(($) => $.users.id)}</dt><dd className="break-all select-all">{user.id}</dd></div>
                  <div><dt className="text-caption text-muted-foreground">{t(($) => $.users.role)}</dt><dd>{roleLabel(user.platformRole)}</dd></div>
                  <div><dt className="text-caption text-muted-foreground">{t(($) => $.users.workspaces)}</dt><dd className="tabular-nums">{user.workspaceCount}</dd></div>
                  <div><dt className="text-caption text-muted-foreground">{t(($) => $.users.created)}</dt><dd><time dateTime={user.createdAt}>{formatAdminTime(user.createdAt, timezone)}</time></dd></div>
                </dl>
              </details>
            </li>)}
          </ul>
          <div className="hidden overflow-x-auto rounded-md border border-surface-border md:block">
            <Table className="w-full text-left text-body">
              <TableHeader className="bg-muted/40 text-caption text-muted-foreground"><TableRow>{[t(($) => $.users.name), t(($) => $.users.username), t(($) => $.users.status), t(($) => $.users.role), t(($) => $.users.workspaces), t(($) => $.users.created)].map((label) => <TableHead key={label} scope="col" className="px-4 py-3 font-medium">{label}</TableHead>)}</TableRow></TableHeader>
              <TableBody>{query.data.items.map((user) => <TableRow key={user.id} className="border-t border-surface-border hover:bg-muted/30"><TableCell className="max-w-64 whitespace-normal break-words px-4 py-3"><AppLink href={detailHref(user.id)} className={`inline-block font-medium underline-offset-4 hover:underline focus-visible:outline-ring ${adminTouchLinkClass}`}>{user.name}</AppLink></TableCell><TableCell className="px-4 py-3">{user.username ?? "—"}</TableCell><TableCell className="px-4 py-3">{statuses[user.status]}</TableCell><TableCell className="px-4 py-3">{roleLabel(user.platformRole)}</TableCell><TableCell className="px-4 py-3 tabular-nums">{user.workspaceCount}</TableCell><TableCell className="whitespace-nowrap px-4 py-3"><time dateTime={user.createdAt}>{formatAdminTime(user.createdAt, timezone)}</time></TableCell></TableRow>)}</TableBody>
            </Table>
          </div>
        </>}
        {query.data.dataQuality === "partial" && query.data.items.length > 0 && <p role="status" className="text-body text-muted-foreground">{t(($) => $.users.partial)}</p>}
        <div className="flex flex-wrap justify-end gap-2"><Button variant="ghost" disabled={query.isFetching} onClick={() => filters.cursor ? page() : void query.refetch()}>{t(($) => $.executions.refresh)}</Button><Button variant="outline" disabled={!filters.cursor} onClick={() => page()}>{t(($) => $.users.first)}</Button><Button variant="outline" disabled={!query.data.nextCursor} onClick={() => page(query.data.nextCursor ?? undefined)}>{t(($) => $.users.next)}</Button></div>
        <p className="text-caption text-muted-foreground">{query.data.registration.enabled ? t(($) => $.users.registrationOpen) : t(($) => $.users.registrationClosed)} {t(($) => $.users.noApproval)}</p>
      </> : null}
    </section>
  );
}
