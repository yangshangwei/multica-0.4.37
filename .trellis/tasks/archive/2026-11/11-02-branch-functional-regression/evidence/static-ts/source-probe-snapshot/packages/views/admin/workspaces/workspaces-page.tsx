"use client";
import { useAdminWorkspaces, observationDrilldown } from "@multica/core/admin";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@multica/ui/components/ui/table";
import styles from "../admin-visual.module.css";
import { useT } from "../../i18n";
import { AppLink } from "../../navigation";
import { AdminListPagination, formatAdminTime } from "../executions/list-controls";
import { ObservationFilters, ObservationHeader, ObservationState, useObservationParams } from "../observability/common";
export function AdminWorkspacesPage() {
  const { t } = useT("admin");
  const params = useObservationParams();
  const query = useAdminWorkspaces(params);
  const data = query.data;
  return <section className="space-y-6"><ObservationHeader title={t($ => $.workspaces.title)} description={t($ => $.workspaces.description)}><AppLink href="/admin/users" className="inline-flex min-h-11 min-w-11 items-center text-body underline underline-offset-4">{t($ => $.workspaces.accounts)}</AppLink></ObservationHeader>
    <ObservationFilters params={params} fields={[{ name: "q", label: t($ => $.observability.search) }]} />
    <ObservationState pending={query.isPending} error={query.isError} quality={data?.dataQuality} asOf={data?.asOf} timezone={data?.window.timezone} empty={data?.items.length === 0} retry={() => void query.refetch()}>{data && <>
      <ul aria-label={t($ => $.workspaces.title)} className="divide-y divide-surface-border md:hidden">
        {data.items.map(workspace => <li key={workspace.id} className={`${styles.compactRow} space-y-1 first:pt-0`}>
          <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1">
            <h2 className="min-w-0 break-words text-body font-medium">{workspace.name}</h2>
            <AppLink href={observationDrilldown("/admin/tasks", data.window, { workspace_id: workspace.id, time_basis: "created" })} className="inline-flex min-h-11 items-center text-caption underline underline-offset-4">{t($ => $.workspaces.openExecutions)}</AppLink>
          </div>
          <dl className="flex flex-wrap gap-x-5 gap-y-1 text-body">
            <div className="flex items-baseline gap-2"><dt className="text-caption text-muted-foreground">{t($ => $.workspaces.members)}</dt><dd className="tabular-nums">{workspace.memberCount}</dd></div>
            <div className="flex items-baseline gap-2"><dt className="text-caption text-muted-foreground">{t($ => $.workspaces.executions)}</dt><dd className="tabular-nums">{workspace.executionCount}</dd></div>
          </dl>
          <details><summary className="cursor-pointer text-caption text-muted-foreground">{t($ => $.workspaces.details)}</summary>
            <dl className="mt-2 space-y-3 text-body">
              <div><dt className="text-caption text-muted-foreground">{t($ => $.workspaces.id)}</dt><dd className="break-all select-all">{workspace.id}</dd></div>
              <div><dt className="text-caption text-muted-foreground">{t($ => $.workspaces.created)}</dt><dd><time dateTime={workspace.createdAt}>{formatAdminTime(workspace.createdAt, data.window.timezone)}</time></dd></div>
            </dl>
          </details>
        </li>)}
      </ul>
      <div className="hidden overflow-x-auto rounded-md border border-surface-border md:block"><Table><TableHeader><TableRow>{[t($ => $.workspaces.name), t($ => $.workspaces.members), t($ => $.workspaces.executions), t($ => $.workspaces.created)].map(label => <TableHead key={label}>{label}</TableHead>)}</TableRow></TableHeader><TableBody>{data.items.map(workspace => <TableRow key={workspace.id}><TableCell className="max-w-72 whitespace-normal"><p className="break-words font-medium">{workspace.name}</p><p className="break-all text-caption text-muted-foreground">{workspace.id}</p></TableCell><TableCell>{workspace.memberCount}</TableCell><TableCell><p>{workspace.executionCount}</p><AppLink href={observationDrilldown("/admin/tasks", data.window, { workspace_id: workspace.id, time_basis: "created" })} className="inline-flex min-h-11 items-center text-caption underline underline-offset-4">{t($ => $.workspaces.openExecutions)}</AppLink></TableCell><TableCell><time dateTime={workspace.createdAt}>{formatAdminTime(workspace.createdAt, data.window.timezone)}</time></TableCell></TableRow>)}</TableBody></Table></div>
    </>}</ObservationState>
    {data && !query.isError && <AdminListPagination cursor={data.nextCursor} asOf={data.asOf} params={params} refresh={() => void query.refetch()} />}
  </section>;
}
