"use client";
import { useAdminExecutions } from "@multica/core/admin";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@multica/ui/components/ui/table";
import { Badge } from "@multica/ui/components/ui/badge";
import { useT } from "../../i18n";
import { AppLink, useNavigation } from "../../navigation";
import { AdminExecutionFilters, AdminExecutionTabs, AdminListPagination, AdminListState, formatAdminTime } from "./list-controls";
export function AdminExecutionListPage() {
  const { t } = useT("admin");
  const nav = useNavigation();
  const query = useAdminExecutions(nav.searchParams);
  const data = query.data;
  return <section className="space-y-6">
    <header className="space-y-2">
      <h1 className="text-title font-semibold">{t($ => $.executions.title)}</h1>
      <p className="max-w-3xl text-body text-muted-foreground">{t($ => $.executions.description)}</p>
    </header>
    <AdminExecutionTabs />
    <AdminExecutionFilters />
    <AdminListState loading={query.isPending} error={query.isError} empty={!data?.items.length} retry={() => void query.refetch()}>
      <div className="overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t($ => $.executions.id)}</TableHead>
              <TableHead>{t($ => $.executions.title_column)}</TableHead>
              <TableHead>{t($ => $.executions.status)}</TableHead>
              <TableHead>{t($ => $.executions.source)}</TableHead>
              <TableHead>{t($ => $.executions.attempt)}</TableHead>
              <TableHead>{t($ => $.executions.created)}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>{data?.items.map(item => <TableRow key={item.id}>
            <TableCell>
              <AppLink className="text-body underline decoration-foreground/30 underline-offset-4" href={`/admin/tasks/${item.id}`}>{item.id}</AppLink>
            </TableCell>
            <TableCell className="max-w-xs truncate">{item.title ?? t($ => $.executions.restricted)}</TableCell>
            <TableCell>
              <Badge variant="outline">{t($ => $.executions.statuses[item.status])}</Badge>
            </TableCell>
            <TableCell>{t($ => $.executions.sources[item.source])}</TableCell>
            <TableCell>{item.attempt}</TableCell>
            <TableCell className="whitespace-nowrap">
              <time dateTime={item.createdAt}>{formatAdminTime(item.createdAt, nav.searchParams.get("timezone") ?? "UTC")}</time>
            </TableCell>
          </TableRow>)}</TableBody>
        </Table>
      </div>
    </AdminListState>{data && !query.isError && <AdminListPagination cursor={data.nextCursor} asOf={data.asOf} refresh={() => void query.refetch()} />}</section>;
}
