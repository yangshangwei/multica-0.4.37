"use client";
import { useAdminInstallation, type AdminInstallationAxis } from "@multica/core/admin";
import { Button } from "@multica/ui/components/ui/button";
import { Table, TableHeader, TableBody, TableHead, TableCell, TableRow } from "@multica/ui/components/ui/table";
import { AppLink } from "../../navigation";
import { useT } from "../../i18n";
import { formatAdminTime } from "../executions/list-controls";
import { InstallationAxis, InstallationReadState } from "./installation-common";
import { AdminInstallationControls } from "../operations/control-panel";
export function AdminInstallationDetailPage({ id }: {
  id: string;
}) {
  const { t } = useT("admin");
  const query = useAdminInstallation(id);
  const data = query.data;
  const item = data?.installation;
  const axes: [
    string,
    AdminInstallationAxis
  ][] = item ? [[t($ => $.installations.client), item.clientActivity], [t($ => $.installations.daemon), item.daemonReachability], [t($ => $.installations.readiness), item.executionReadiness]] : [];
  return <section className="space-y-7">
    <AppLink href="/admin/installations" className="text-body text-muted-foreground underline underline-offset-4">{t($ => $.installations.back)}</AppLink>
    <InstallationReadState pending={query.isPending} error={query.isError} empty={false} retry={() => void query.refetch()}>{data && item && <>
      <header className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h1 className="break-words text-title font-semibold">{item.displayName || t($ => $.installations.details)}</h1>
          <Button variant="outline" onClick={() => void query.refetch()}>{t($ => $.installations.refresh)}</Button>
        </div>
        <p className="break-all text-caption text-muted-foreground">{item.id}</p>
      </header>
      <AdminInstallationControls key={item.id} installation={item} onRefresh={() => query.refetch({ throwOnError: true })} />
      <dl className="grid gap-5 md:grid-cols-3">{axes.map(([label, axis]) => <div key={String(label)} className="space-y-2">
        <dt className="text-caption text-muted-foreground">{String(label)}</dt>
        <dd>
          <InstallationAxis axis={axis} />
        </dd>
      </div>)}</dl>
      <p className="max-w-3xl text-body text-muted-foreground">{t($ => $.installations.readiness_notice)}</p>
      <dl className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">{[[t($ => $.installations.deployment), item.deploymentId], [t($ => $.installations.owner), item.responsibleUserId], [t($ => $.installations.version), item.desktopVersion], [t($ => $.installations.os), item.os], [t($ => $.installations.group), item.groups.join(", ")], [t($ => $.installations.lifecycle), item.lifecycle]].map(([label, value]) => <div key={label} className="min-w-0 space-y-1">
        <dt className="text-caption text-muted-foreground">{label}</dt>
        <dd className="break-all text-body">{value || t($ => $.installations.unknown)}</dd>
      </div>)}</dl>
      <AppLink href={`/admin/tasks?installation_id=${item.id}`} className="inline-flex min-h-11 items-center text-body underline underline-offset-4">{t($ => $.installations.view_executions)}</AppLink>
      {data.detailsTruncated && <p role="status" className="text-body text-muted-foreground">{t($ => $.installations.truncated)}</p>}
      <section className="space-y-3">
        <h2 className="text-body-lg font-semibold">{t($ => $.installations.runtimes)} ({item.runtimeCount})</h2>{data.runtimes.length ? <div className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>ID</TableHead>
                <TableHead>{t($ => $.installations.provider)}</TableHead>
                <TableHead>{t($ => $.installations.runtime_status)}</TableHead>
                <TableHead>{t($ => $.installations.running)}</TableHead>
                <TableHead>{t($ => $.installations.last_seen)}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>{data.runtimes.map(runtime => <TableRow key={runtime.id}>
              <TableCell>{runtime.id}</TableCell>
              <TableCell>{runtime.provider}</TableCell>
              <TableCell>{runtime.status}</TableCell>
              <TableCell>{runtime.runningTasks ?? t($ => $.installations.unknown)}</TableCell>
              <TableCell>{runtime.lastSeenAt ? formatAdminTime(runtime.lastSeenAt) : t($ => $.installations.unknown)}</TableCell>
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
          <AppLink href={`/admin/users/${user.userId}`} className="break-all underline underline-offset-4">{user.userId}</AppLink>
          <time className="text-muted-foreground" dateTime={user.lastSeenAt}>{formatAdminTime(user.lastSeenAt)}</time>
        </li>)}</ul> : <p className="text-body text-muted-foreground">{t($ => $.installations.no_users)}</p>}</section>
    </>}</InstallationReadState>{query.isError && data && <p className="text-caption text-muted-foreground">{t($ => $.installations.last_seen)}: <time dateTime={data.asOf}>{formatAdminTime(data.asOf)}</time>
    </p>}</section>;
}
