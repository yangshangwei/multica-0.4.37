"use client";
import { useAdminIssues } from "@multica/core/admin";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@multica/ui/components/ui/table";
import { useT } from "../../i18n";
import { AppLink, useNavigation } from "../../navigation";
import { AdminExecutionFilters, AdminExecutionTabs, AdminListPagination, AdminListState, adminTouchLinkClass, formatAdminTime } from "../executions/list-controls";
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
        <Table className="table-fixed md:table-auto">
          <TableHeader>
            <TableRow>
              <TableHead className="w-1/2 md:w-auto">{t($ => $.executions.title_column)}</TableHead>
              <TableHead className="whitespace-normal">{t($ => $.executions.status)}</TableHead>
              <TableHead className="whitespace-normal">{t($ => $.executions.count)}</TableHead>
              <TableHead className="hidden md:table-cell">{t($ => $.executions.created)}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>{data?.items.map(item => <TableRow key={item.id}>
            <TableCell className="align-top whitespace-normal">
              <div className="space-y-2">
                <p className="break-words text-body font-medium">{item.contentAccess && item.contentUrl ? <AppLink href={item.contentUrl} className={`inline-block underline decoration-foreground/30 underline-offset-4 ${adminTouchLinkClass}`}>{item.title ?? item.identifier}</AppLink> : t($ => $.executions.restricted)}</p>
                <p className="break-all text-caption text-muted-foreground">{item.identifier}</p>
                <details className="md:hidden">
                  <summary className="cursor-pointer text-caption text-muted-foreground">{t($ => $.executions.issue_details)}</summary>
                  <dl className="mt-2 text-caption"><dt className="text-muted-foreground">{t($ => $.executions.created)}</dt><dd><time dateTime={item.createdAt}>{formatAdminTime(item.createdAt, nav.searchParams.get("timezone") ?? "UTC")}</time></dd></dl>
                </details>
              </div>
            </TableCell>
            <TableCell className="break-words align-top whitespace-normal">{item.status}</TableCell>
            <TableCell className="align-top">
              <AppLink className={`inline-block underline underline-offset-4 ${adminTouchLinkClass}`} href={executionLink(item.id, item.workspaceId, data.timeFrom, data.timeTo, nav.searchParams.get("timezone"))}>{item.executionCount}</AppLink>
            </TableCell>
            <TableCell className="hidden md:table-cell">
              <time dateTime={item.createdAt}>{formatAdminTime(item.createdAt, nav.searchParams.get("timezone") ?? "UTC")}</time>
            </TableCell>
          </TableRow>)}</TableBody>
        </Table>
    </AdminListState>
    {data && !query.isError && <AdminListPagination cursor={data.nextCursor} asOf={data.asOf} refresh={() => void query.refetch()} />}</section>;
}
