"use client";
import { adminAlertRules, adminAlertStatuses, useAdminAlerts } from "@multica/core/admin";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@multica/ui/components/ui/table";
import { useT } from "../../i18n";
import { AppLink, useNavigation } from "../../navigation";
import { AdminListPagination, formatAdminTime } from "../executions/list-controls";
import { ObservationFilters, ObservationHeader, ObservationState } from "../observability/common";
export function AdminAlertsPage() {
  const { t } = useT("admin");
  const nav = useNavigation();
  const query = useAdminAlerts(nav.searchParams);
  const data = query.data;
  const timezone = nav.searchParams.get("timezone") ?? "UTC";
  return <section className="space-y-6"><ObservationHeader title={t($ => $.alerts.title)} description={t($ => $.alerts.description)}><AppLink href="/admin/health" className="inline-flex min-h-11 items-center text-body underline underline-offset-4">{t($ => $.health.title)}</AppLink></ObservationHeader>
    <p className="max-w-prose text-caption text-muted-foreground">{t($ => $.alerts.activeNotice)}</p>
    <ObservationFilters params={nav.searchParams} fields={[{ name: "rule", label: t($ => $.alerts.rule), options: adminAlertRules.filter(rule => rule !== "unknown").map(rule => ({ value: rule, label: t($ => $.alerts.rules[rule]) })) }, { name: "status", label: t($ => $.alerts.status), emptyLabel: t($ => $.alerts.active), options: [{ value: "all", label: t($ => $.observability.all) }, ...adminAlertStatuses.filter(status => status !== "unknown").map(status => ({ value: status, label: t($ => $.alerts.statuses[status]) }))] }, { name: "assignee_id", label: t($ => $.alerts.assignee) }]} />
    <ObservationState pending={query.isPending} error={query.isError} quality={data?.dataQuality} empty={data?.items.length === 0} asOf={data?.asOf} timezone={timezone} retry={() => void query.refetch()}>{data && <div className="overflow-x-auto rounded-md border border-surface-border"><Table><TableHeader><TableRow>{[t($ => $.alerts.rule), t($ => $.alerts.status), t($ => $.alerts.severity), t($ => $.alerts.subject), t($ => $.alerts.occurrences), t($ => $.alerts.lastSeen)].map(label => <TableHead key={label}>{label}</TableHead>)}</TableRow></TableHeader><TableBody>{data.items.map(alert => <TableRow key={alert.id}><TableCell className="min-w-48 whitespace-normal"><AppLink href={`/admin/alerts/${alert.id}`} className="font-medium underline underline-offset-4">{t($ => $.alerts.rules[alert.rule])}</AppLink></TableCell><TableCell>{t($ => $.alerts.statuses[alert.status])}</TableCell><TableCell>{t($ => $.alerts.severities[alert.severity])}</TableCell><TableCell className="max-w-52 whitespace-normal break-all">{alert.subjectId}</TableCell><TableCell>{alert.occurrenceCount}</TableCell><TableCell><time dateTime={alert.lastSeenAt}>{formatAdminTime(alert.lastSeenAt, timezone)}</time></TableCell></TableRow>)}</TableBody></Table></div>}</ObservationState>
    {data && !query.isError && <AdminListPagination cursor={data.nextCursor} asOf={data.asOf} refresh={() => void query.refetch()} />}
  </section>;
}
