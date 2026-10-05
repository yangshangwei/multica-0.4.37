"use client";
import { adminDetailHref, adminReturnHref, useAdminInstallation, type AdminInstallationAxis } from "@multica/core/admin";
import { Button } from "@multica/ui/components/ui/button";
import { Table, TableHeader, TableBody, TableHead, TableCell, TableRow } from "@multica/ui/components/ui/table";
import { AppLink, useNavigation } from "../../navigation";
import { useT } from "../../i18n";
import { adminTouchLinkClass, formatAdminTime } from "../executions/list-controls";
import { InstallationAxis, InstallationReadState, InstallationRuntimeStatus } from "./installation-common";
import { AdminInstallationControls } from "../operations/control-panel";
import { InstallationIdentifier } from "./installation-identity";
export function AdminInstallationDetailPage({ id }: {
  id: string;
}) {
  const { t } = useT("admin");
  const nav = useNavigation();
  const timezone = nav.searchParams.get("timezone") ?? "UTC";
  const query = useAdminInstallation(id);
  const data = query.data;
  const item = data?.installation;
  const axes: [
    string,
    AdminInstallationAxis
  ][] = item ? [[t($ => $.installations.client), item.clientActivity], [t($ => $.installations.daemon), item.daemonReachability], [t($ => $.installations.readiness), item.executionReadiness]] : [];
  return <section className="space-y-7">
    <AppLink href={adminReturnHref(nav.searchParams, "/admin/installations")} className="inline-flex min-h-11 items-center text-body text-muted-foreground underline underline-offset-4">{t($ => $.installations.back)}</AppLink>
    <InstallationReadState pending={query.isPending} error={query.isError} empty={false} retry={() => void query.refetch()}>{data && item && <>
      <header className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h1 className="break-words text-title font-semibold">{item.displayName || t($ => $.installations.details)}</h1>
          <Button variant="outline" onClick={() => void query.refetch()}>{t($ => $.installations.refresh)}</Button>
        </div>
        <InstallationIdentifier id={item.id} />
      </header>
      <AdminInstallationControls key={item.id} installation={item} onRefresh={() => query.refetch({ throwOnError: true })} />
      <dl className="grid gap-5 md:grid-cols-3">{axes.map(([label, axis]) => <div key={String(label)} className="space-y-2">
        <dt className="text-caption text-muted-foreground">{String(label)}</dt>
        <dd>
          <InstallationAxis axis={axis} />
        </dd>
      </div>)}</dl>
      <p className="max-w-3xl text-body text-muted-foreground">{t($ => $.installations.readiness_notice)}</p>
      <dl className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">{[[t($ => $.installations.deployment), item.deploymentId], [t($ => $.installations.owner), item.responsibleUserId], [t($ => $.installations.version), item.desktopVersion], [t($ => $.installations.os), item.os], [t($ => $.installations.group), item.groups.join(", ")], [t($ => $.installations.lifecycle), item.lifecycle === "active" ? t($ => $.installations.active) : item.lifecycle === "retired" ? t($ => $.installations.retired) : t($ => $.installations.unknown_lifecycle)]].map(([label, value]) => <div key={label} className="min-w-0 space-y-1">
        <dt className="text-caption text-muted-foreground">{label}</dt>
        <dd className="break-all text-body">{value || t($ => $.installations.unknown)}</dd>
      </div>)}</dl>
      <AppLink href={`/admin/tasks?${new URLSearchParams({ installation_id: item.id, ...(nav.searchParams.has("timezone") ? { timezone } : {}) })}`} className="inline-flex min-h-11 items-center text-body underline underline-offset-4">{t($ => $.installations.view_executions)}</AppLink>
      {data.detailsTruncated && <p role="status" className="text-body text-muted-foreground">{t($ => $.installations.truncated)}</p>}
      <section className="space-y-3">
        <h2 className="text-body-lg font-semibold">{t($ => $.installations.runtimes)} ({item.runtimeCount})</h2>{data.runtimes.length ? <div className="overflow-x-auto">
          <Table role="table" className="block w-full lg:table">
            <TableHeader className="sr-only lg:not-sr-only lg:table-header-group">
              <TableRow>
                <TableHead>ID</TableHead>
                <TableHead>{t($ => $.installations.provider)}</TableHead>
                <TableHead>{t($ => $.installations.runtime_status)}</TableHead>
                <TableHead>{t($ => $.installations.running)}</TableHead>
                <TableHead>{t($ => $.installations.last_seen)}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody role="rowgroup" className="block lg:table-row-group">{data.runtimes.map(runtime => <TableRow role="row" key={runtime.id} className="grid grid-cols-2 gap-x-4 gap-y-3 py-4 lg:table-row lg:py-0">
              <TableCell className="col-span-2 row-start-2 block break-all whitespace-normal text-caption text-muted-foreground lg:table-cell">{runtime.id}</TableCell>
              <TableCell className="row-start-1 block min-w-0 break-words whitespace-normal font-medium lg:table-cell">{runtime.provider}</TableCell>
              <TableCell className="row-start-1 block min-w-0 whitespace-normal lg:table-cell"><span className="mb-1 block text-caption text-muted-foreground lg:hidden" aria-hidden="true">{t($ => $.installations.runtime_status)}</span><InstallationRuntimeStatus status={runtime.status} /></TableCell>
              <TableCell className="block min-w-0 whitespace-normal lg:table-cell"><span className="mb-1 block text-caption text-muted-foreground lg:hidden" aria-hidden="true">{t($ => $.installations.running)}</span>{runtime.runningTasks ?? t($ => $.installations.unknown)}</TableCell>
              <TableCell className="block min-w-0 whitespace-normal lg:table-cell"><span className="mb-1 block text-caption text-muted-foreground lg:hidden" aria-hidden="true">{t($ => $.installations.last_seen)}</span>{runtime.lastSeenAt ? <time dateTime={runtime.lastSeenAt}>{formatAdminTime(runtime.lastSeenAt, timezone)}</time> : t($ => $.installations.unknown)}</TableCell>
            </TableRow>)}</TableBody>
          </Table>
        </div> : <p className="text-body text-muted-foreground">{t($ => $.installations.unknown)}</p>}</section>
      <section className="space-y-3">
        <h2 className="text-body-lg font-semibold">{t($ => $.installations.bindings)}</h2>{data.bindings.length ? <ul className="divide-y divide-surface-border">{data.bindings.map(binding => <li key={binding.id} className="grid gap-2 py-3 text-body sm:grid-cols-2">
          <div className="break-all">{binding.id}<p className="text-caption text-muted-foreground">{t($ => $.installations.workspace)}: {binding.workspaceId}</p>
          </div>
          <div>{binding.state === "active" ? t($ => $.installations.active) : binding.state === "revoked" ? t($ => $.installations.revoked) : t($ => $.installations.unknown)}<p className="break-all text-caption text-muted-foreground">{t($ => $.installations.principal)}: {binding.principalUserId}</p>
          </div>
        </li>)}</ul> : <p className="text-body text-muted-foreground">{t($ => $.installations.no_bindings)}</p>}</section>
      <section className="space-y-3">
        <h2 className="text-body-lg font-semibold">{t($ => $.installations.users)}</h2>{data.users.length ? <ul className="space-y-3">{data.users.map(user => <li key={user.userId} className="flex flex-wrap justify-between gap-3 text-body">
          <AppLink href={adminDetailHref(`/admin/users/${user.userId}`, "/admin/users", new URLSearchParams({ timezone }))} className={`inline-block break-all underline underline-offset-4 ${adminTouchLinkClass}`}>{user.userId}</AppLink>
          <time className="text-muted-foreground" dateTime={user.lastSeenAt}>{formatAdminTime(user.lastSeenAt, timezone)}</time>
        </li>)}</ul> : <p className="text-body text-muted-foreground">{t($ => $.installations.no_users)}</p>}</section>
    </>}</InstallationReadState>{query.isError && data && <p className="text-caption text-muted-foreground">{t($ => $.installations.last_seen)}: <time dateTime={data.asOf}>{formatAdminTime(data.asOf, timezone)}</time>
    </p>}</section>;
}
