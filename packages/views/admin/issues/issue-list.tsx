"use client";
import { useAdminIssues } from "@multica/core/admin";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@multica/ui/components/ui/table";
import { useT } from "../../i18n";
import { AppLink, useNavigation } from "../../navigation";
import { AdminExecutionFilters, AdminExecutionTabs, AdminListPagination, AdminListState, formatAdminTime } from "../executions/list-controls";
function executionLink(issueId: string, workspaceId: string, from: string | null | undefined, to: string | null | undefined, timezone: string | null) {
  const params = new URLSearchParams({ issue_id: issueId, workspace_id: workspaceId });
  if (from)
    params.set("time_from", from);
  if (to)
    params.set("time_to", to);
  if (timezone)
    params.set("timezone", timezone);
  return `/admin/tasks?${params}`;
}
export function AdminIssueListPage() {
  const { t } = useT("admin");
  const nav = useNavigation();
  const query = useAdminIssues(nav.searchParams);
  const data = query.data;
  return <section className="space-y-6">
    <header className="space-y-2">
      <h1 className="text-title font-semibold">{t($ => $.executions.issues)}</h1>
      <p className="text-body text-muted-foreground">{t($ => $.executions.business_description)}</p>
    </header>
    <AdminExecutionTabs />
    <AdminExecutionFilters issues />
    <AdminListState loading={query.isPending} error={query.isError} empty={!data?.items.length} issues retry={() => void query.refetch()}>
      <div className="overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t($ => $.executions.title_column)}</TableHead>
              <TableHead>{t($ => $.executions.status)}</TableHead>
              <TableHead>{t($ => $.executions.count)}</TableHead>
              <TableHead>{t($ => $.executions.created)}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>{data?.items.map(item => <TableRow key={item.id}>
            <TableCell>
              <div className="space-y-1">
                <span className="text-caption text-muted-foreground">{item.identifier}</span>
                <p className="max-w-md truncate text-body">{item.contentAccess && item.contentUrl ? <AppLink href={item.contentUrl} className="underline decoration-foreground/30 underline-offset-4">{item.title}</AppLink> : t($ => $.executions.restricted)}</p>
              </div>
            </TableCell>
            <TableCell>{item.status}</TableCell>
            <TableCell>
              <AppLink className="underline underline-offset-4" href={executionLink(item.id, item.workspaceId, data.timeFrom, data.timeTo, nav.searchParams.get("timezone"))}>{item.executionCount}</AppLink>
            </TableCell>
            <TableCell>
              <time dateTime={item.createdAt}>{formatAdminTime(item.createdAt, nav.searchParams.get("timezone") ?? "UTC")}</time>
            </TableCell>
          </TableRow>)}</TableBody>
        </Table>
      </div>
    </AdminListState>
    {data && !query.isError && <AdminListPagination cursor={data.nextCursor} asOf={data.asOf} refresh={() => void query.refetch()} />}</section>;
}
