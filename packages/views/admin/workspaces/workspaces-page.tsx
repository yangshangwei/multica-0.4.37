"use client";
import { useAdminWorkspaces, observationDrilldown } from "@multica/core/admin";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@multica/ui/components/ui/table";
import { useT } from "../../i18n";
import { AppLink } from "../../navigation";
import { AdminListPagination, formatAdminTime } from "../executions/list-controls";
import { ObservationFilters, ObservationHeader, ObservationState, useObservationParams } from "../observability/common";
export function AdminWorkspacesPage() {
  const { t } = useT("admin");
  const params = useObservationParams();
  const query = useAdminWorkspaces(params);
  const data = query.data;
  return <section className="space-y-6"><ObservationHeader title={t($ => $.workspaces.title)} description={t($ => $.workspaces.description)}><AppLink href="/admin/users" className="inline-flex min-h-11 items-center text-body underline underline-offset-4">{t($ => $.workspaces.accounts)}</AppLink></ObservationHeader>
    <ObservationFilters params={params} fields={[{ name: "q", label: t($ => $.observability.search) }]} />
    <ObservationState pending={query.isPending} error={query.isError} quality={data?.dataQuality} asOf={data?.asOf} timezone={data?.window.timezone} empty={data?.items.length === 0} retry={() => void query.refetch()}>{data && <div className="overflow-x-auto rounded-md border border-surface-border"><Table><TableHeader><TableRow>{[t($ => $.workspaces.name), t($ => $.workspaces.members), t($ => $.workspaces.executions), t($ => $.workspaces.created)].map(label => <TableHead key={label}>{label}</TableHead>)}</TableRow></TableHeader><TableBody>{data.items.map(workspace => <TableRow key={workspace.id}><TableCell className="max-w-72 whitespace-normal"><p className="break-words font-medium">{workspace.name}</p><p className="break-all text-caption text-muted-foreground">{workspace.id}</p></TableCell><TableCell>{workspace.memberCount}</TableCell><TableCell><p>{workspace.executionCount}</p><AppLink href={observationDrilldown("/admin/tasks", data.window, { workspace_id: workspace.id, time_basis: "created" })} className="inline-flex min-h-11 items-center text-caption underline underline-offset-4">{t($ => $.workspaces.openExecutions)}</AppLink></TableCell><TableCell><time dateTime={workspace.createdAt}>{formatAdminTime(workspace.createdAt, data.window.timezone)}</time></TableCell></TableRow>)}</TableBody></Table></div>}</ObservationState>
    {data && !query.isError && <AdminListPagination cursor={data.nextCursor} asOf={data.asOf} params={params} refresh={() => void query.refetch()} />}
  </section>;
}
