"use client";
import { adminAlertRules, adminAlertStatuses, adminDetailHref, useAdminAlerts } from "@multica/core/admin";
import { Badge } from "@multica/ui/components/ui/badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@multica/ui/components/ui/table";
import { useT } from "../../i18n";
import { AppLink, useNavigation } from "../../navigation";
import { AdminListPagination, adminTouchLinkClass, formatAdminTime } from "../executions/list-controls";
import { ObservationFilters, ObservationHeader, ObservationState } from "../observability/common";
export function AdminAlertsPage() {
  const { t } = useT("admin");
  const nav = useNavigation();
  const query = useAdminAlerts(nav.searchParams);
  const data = query.data;
  const timezone = nav.searchParams.get("timezone") ?? "UTC";
  return <section className="space-y-6"><ObservationHeader title={t($ => $.alerts.title)} description={t($ => $.alerts.description)}><AppLink href={`/admin/health?${new URLSearchParams({ timezone })}`} className="inline-flex min-h-11 items-center text-body underline underline-offset-4">{t($ => $.health.title)}</AppLink></ObservationHeader>
    <p className="max-w-prose text-caption text-muted-foreground">{t($ => $.alerts.activeNotice)}</p>
    <ObservationFilters advanced params={nav.searchParams} fields={[{ name: "rule", label: t($ => $.alerts.rule), options: adminAlertRules.filter(rule => rule !== "unknown").map(rule => ({ value: rule, label: t($ => $.alerts.rules[rule]) })) }, { name: "status", label: t($ => $.alerts.status), emptyLabel: t($ => $.alerts.active), options: [{ value: "all", label: t($ => $.observability.all) }, ...adminAlertStatuses.filter(status => status !== "unknown").map(status => ({ value: status, label: t($ => $.alerts.statuses[status]) }))] }, { name: "assignee_id", label: t($ => $.alerts.assignee) }]} />
    <ObservationState pending={query.isPending} error={query.isError} quality={data?.dataQuality} empty={data?.items.length === 0} asOf={data?.asOf} timezone={timezone} retry={() => void query.refetch()}>{data && <div className="overflow-x-auto rounded-md border border-surface-border">
      <Table role="table" className="block w-full lg:table">
        <TableHeader className="sr-only lg:not-sr-only lg:table-header-group"><TableRow>{[t($ => $.alerts.rule), t($ => $.alerts.status), t($ => $.alerts.severity), t($ => $.alerts.subject), t($ => $.alerts.occurrences), t($ => $.alerts.lastSeen)].map(label => <TableHead key={label}>{label}</TableHead>)}</TableRow></TableHeader>
        <TableBody role="rowgroup" className="block lg:table-row-group">{data.items.map(alert => <TableRow role="row" key={alert.id} className="grid grid-cols-2 gap-x-4 gap-y-3 py-4 lg:table-row lg:py-0">
          <TableCell className="col-span-2 block min-w-0 whitespace-normal lg:table-cell"><AppLink href={adminDetailHref(`/admin/alerts/${alert.id}`, nav.pathname, nav.searchParams)} className={`block font-medium underline underline-offset-4 ${adminTouchLinkClass}`}>{t($ => $.alerts.rules[alert.rule])}</AppLink></TableCell>
          <TableCell className="block min-w-0 whitespace-normal lg:table-cell"><span className="mb-1 block text-caption text-muted-foreground lg:hidden" aria-hidden="true">{t($ => $.alerts.status)}</span>{t($ => $.alerts.statuses[alert.status])}</TableCell>
          <TableCell className="block min-w-0 whitespace-normal lg:table-cell"><span className="mb-1 block text-caption text-muted-foreground lg:hidden" aria-hidden="true">{t($ => $.alerts.severity)}</span><Badge variant="outline">{t($ => $.alerts.severities[alert.severity])}</Badge></TableCell>
          <TableCell className="col-span-2 block min-w-0 break-all whitespace-normal text-caption text-muted-foreground lg:table-cell lg:max-w-52"><span className="mb-1 block lg:hidden" aria-hidden="true">{t($ => $.alerts.subject)}</span>{alert.subjectId}</TableCell>
          <TableCell className="block min-w-0 whitespace-normal lg:table-cell"><span className="mb-1 block text-caption text-muted-foreground lg:hidden" aria-hidden="true">{t($ => $.alerts.occurrences)}</span>{alert.occurrenceCount}</TableCell>
          <TableCell className="block min-w-0 whitespace-normal lg:table-cell"><span className="mb-1 block text-caption text-muted-foreground lg:hidden" aria-hidden="true">{t($ => $.alerts.lastSeen)}</span><time dateTime={alert.lastSeenAt}>{formatAdminTime(alert.lastSeenAt, timezone)}</time></TableCell>
        </TableRow>)}</TableBody>
      </Table>
    </div>}</ObservationState>
    {data && !query.isError && <AdminListPagination cursor={data.nextCursor} asOf={data.asOf} refresh={() => void query.refetch()} />}
  </section>;
}
