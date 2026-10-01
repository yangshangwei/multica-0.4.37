"use client";

import type { FormEvent } from "react";
import { adminApiScope, useAdminAccess, useAdminUsers, type AdminUserFilters } from "@multica/core/admin";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@multica/ui/components/ui/table";
import { AppLink, useNavigation } from "../../navigation";
import { useT } from "../../i18n";

export function AdminUsersPage({ administrators = false }: { administrators?: boolean }) {
  const { t } = useT("admin");
  const navigation = useNavigation();
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
    const values = new FormData(event.currentTarget);
    const next = new URLSearchParams();
    for (const name of ["q", "status", "role"]) {
      const value = String(values.get(name) ?? "").trim();
      if (value) next.set(name, value);
    }
    for (const name of ["time_from", "time_to"]) {
      const value = String(values.get(name) ?? "");
      if (value) next.set(name, `${value}T00:00:00Z`);
    }
    navigation.replace(`${navigation.pathname}?${next}`);
  }
  function page(cursor?: string) {
    const next = new URLSearchParams(params);
    if (cursor) next.set("cursor", cursor); else next.delete("cursor");
    navigation.push(`${navigation.pathname}?${next}`);
  }
  const selectClass = "h-10 w-full rounded-md border border-input bg-background px-3 text-body focus-visible:outline-ring";
  return (
    <section className="space-y-6" aria-labelledby="admin-users-title">
      <div className="space-y-2">
        <h1 id="admin-users-title" className="text-title font-semibold">{administrators ? t(($) => $.users.adminTitle) : t(($) => $.users.title)}</h1>
        <p className="max-w-prose text-body text-muted-foreground">{t(($) => $.users.description)}</p>
      </div>
      <form key={params.toString()} onSubmit={filter} className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <label className="space-y-1 text-caption">{t(($) => $.users.search)}<Input name="q" defaultValue={filters.q} maxLength={128} /></label>
        <label className="space-y-1 text-caption">{t(($) => $.users.status)}<select name="status" defaultValue={filters.status ?? ""} className={selectClass}><option value="">{t(($) => $.users.allStates)}</option>{Object.entries(statuses).filter(([value]) => value !== "unknown").map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
        <label className="space-y-1 text-caption">{t(($) => $.users.role)}<select name="role" defaultValue={filters.role ?? ""} className={selectClass}><option value={administrators ? "administrators" : ""}>{t(($) => $.users.allRoles)}</option><option value="super_admin">{t(($) => $.users.superAdmin)}</option><option value="platform_observer">{t(($) => $.users.observer)}</option></select></label>
        <label className="space-y-1 text-caption">{t(($) => $.users.timeFrom)}<Input type="date" name="time_from" defaultValue={filters.timeFrom?.slice(0, 10)} /></label>
        <label className="space-y-1 text-caption">{t(($) => $.users.timeTo)}<Input type="date" name="time_to" defaultValue={filters.timeTo?.slice(0, 10)} /></label>
        <Button type="submit" variant="outline" className="self-end">{t(($) => $.users.filter)}</Button>
      </form>
      {query.isPending ? <p role="status">{t(($) => $.state.loading)}</p> : query.isError ? (
        <div role="alert" className="space-y-3"><p>{t(($) => $.users.loadError)}</p><Button variant="outline" onClick={() => void query.refetch()}>{t(($) => $.users.retry)}</Button></div>
      ) : query.data ? <>
        <p className="text-caption text-muted-foreground">{t(($) => $.users.updated)}: <time dateTime={query.data.asOf}>{new Date(query.data.asOf).toLocaleString()}</time></p>
        {query.data.items.length === 0 ? <p className="py-8 text-body text-muted-foreground">{query.data.dataQuality === "partial" ? t(($) => $.users.partial) : filters.q || filters.status || filters.role ? t(($) => $.users.filteredEmpty) : t(($) => $.users.empty)}</p> : (
          <div className="overflow-x-auto rounded-md border border-surface-border">
            <Table className="w-full text-left text-body">
              <TableHeader className="bg-muted/40 text-caption text-muted-foreground"><TableRow>{[t(($) => $.users.name), t(($) => $.users.username), t(($) => $.users.status), t(($) => $.users.role), t(($) => $.users.workspaces), t(($) => $.users.created)].map((label) => <TableHead key={label} scope="col" className="px-4 py-3 font-medium">{label}</TableHead>)}</TableRow></TableHeader>
              <TableBody>{query.data.items.map((user) => <TableRow key={user.id} className="border-t border-surface-border hover:bg-muted/30"><TableCell className="max-w-64 whitespace-normal break-words px-4 py-3"><AppLink href={`/admin/users/${user.id}`} className="font-medium underline-offset-4 hover:underline focus-visible:outline-ring">{user.name}</AppLink></TableCell><TableCell className="px-4 py-3">{user.username ?? "—"}</TableCell><TableCell className="px-4 py-3">{statuses[user.status]}</TableCell><TableCell className="px-4 py-3">{roleLabel(user.platformRole)}</TableCell><TableCell className="px-4 py-3 tabular-nums">{user.workspaceCount}</TableCell><TableCell className="whitespace-nowrap px-4 py-3"><time dateTime={user.createdAt}>{new Date(user.createdAt).toLocaleDateString()}</time></TableCell></TableRow>)}</TableBody>
            </Table>
          </div>
        )}
        {query.data.dataQuality === "partial" && query.data.items.length > 0 && <p role="status" className="text-body text-muted-foreground">{t(($) => $.users.partial)}</p>}
        <div className="flex flex-wrap gap-3"><Button variant="outline" disabled={!filters.cursor} onClick={() => page()}>{t(($) => $.users.first)}</Button><Button variant="outline" disabled={!query.data.nextCursor} onClick={() => page(query.data.nextCursor ?? undefined)}>{t(($) => $.users.next)}</Button></div>
        <p className="text-caption text-muted-foreground">{query.data.registration.enabled ? t(($) => $.users.registrationOpen) : t(($) => $.users.registrationClosed)} {t(($) => $.users.noApproval)}</p>
      </> : null}
    </section>
  );
}
