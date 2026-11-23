"use client";
import { adminDetailHref, useAdminExecutions } from "@multica/core/admin";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@multica/ui/components/ui/table";
import { Badge } from "@multica/ui/components/ui/badge";
import { useT } from "../../i18n";
import { AppLink, useNavigation } from "../../navigation";
import { AdminExecutionFilters, AdminExecutionTabs, AdminListPagination, AdminListState, adminTouchLinkClass, formatAdminTime } from "./list-controls";
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
        <Table className="table-fixed sm:table-auto">
          <TableHeader>
            <TableRow>
              <TableHead className="w-2/3 sm:w-auto">{t($ => $.executions.title_column)}</TableHead>
              <TableHead className="whitespace-normal">{t($ => $.executions.status)}</TableHead>
              <TableHead className="hidden sm:table-cell">{t($ => $.executions.source)}</TableHead>
              <TableHead className="hidden sm:table-cell">{t($ => $.executions.attempt)}</TableHead>
              <TableHead className="hidden lg:table-cell">{t($ => $.executions.created)}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>{data?.items.map(item => <TableRow key={item.id}>
            <TableCell className="align-top whitespace-normal">
              <div className="space-y-2">
                <AppLink className={`block break-words text-body font-medium underline decoration-foreground/30 underline-offset-4 ${adminTouchLinkClass}`} href={adminDetailHref(`/admin/tasks/${item.id}`, nav.pathname, nav.searchParams)}>{item.contentAccess ? item.title ?? t($ => $.executions.restricted) : t($ => $.executions.restricted)}</AppLink>
                <p className="break-all text-caption text-muted-foreground">{item.id}</p>
                <details className="lg:hidden">
                  <summary className="cursor-pointer text-caption text-muted-foreground">{t($ => $.executions.row_details)}</summary>
                  <dl className="mt-2 space-y-2 text-caption">
                    <div className="sm:hidden"><dt className="text-muted-foreground">{t($ => $.executions.source)}</dt><dd>{t($ => $.executions.sources[item.source])}</dd></div>
                    <div className="sm:hidden"><dt className="text-muted-foreground">{t($ => $.executions.attempt)}</dt><dd>{item.attempt}</dd></div>
                    <div><dt className="text-muted-foreground">{t($ => $.executions.created)}</dt><dd><time dateTime={item.createdAt}>{formatAdminTime(item.createdAt, nav.searchParams.get("timezone") ?? "UTC")}</time></dd></div>
                  </dl>
                </details>
              </div>
            </TableCell>
            <TableCell className="align-top whitespace-normal">
              <Badge variant="outline" className="max-w-full whitespace-normal">{t($ => $.executions.statuses[item.status])}</Badge>
            </TableCell>
            <TableCell className="hidden sm:table-cell">{t($ => $.executions.sources[item.source])}</TableCell>
            <TableCell className="hidden sm:table-cell">{item.attempt}</TableCell>
            <TableCell className="hidden whitespace-nowrap lg:table-cell">
              <time dateTime={item.createdAt}>{formatAdminTime(item.createdAt, nav.searchParams.get("timezone") ?? "UTC")}</time>
            </TableCell>
          </TableRow>)}</TableBody>
        </Table>
    </AdminListState>{data && !query.isError && <AdminListPagination cursor={data.nextCursor} asOf={data.asOf} refresh={() => void query.refetch()} notice={t($ => $.executions.live_notice)} />}</section>;
}
