"use client";
import { adminAlertActions, adminDetailHref, adminReturnHref, canControlAlert, useAdminAlert, useAdminObservationScope, type AdminAlert } from "@multica/core/admin";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../../i18n";
import { AppLink, useNavigation } from "../../navigation";
import { formatAdminTime } from "../executions/list-controls";
import { ObservationHeader, ObservationState } from "../observability/common";
import { AdminControlPanel } from "../operations/control-panel";
function AlertControls({ alert, refresh }: { alert: AdminAlert; refresh(): Promise<unknown> }) {
  const { t } = useT("admin");
  const { scope, identity, enabled } = useAdminObservationScope();
  if (!enabled || identity?.role !== "super_admin") return null;
  const targets = adminAlertActions.filter(action => canControlAlert(identity.role, alert, action)).map(action => ({ target: { id: alert.id, action: "alert" as const, alertAction: action, version: alert.version, requiresResolution: alert.rule === "execution_failed" }, label: t($ => $.alerts[action]) }));
  return <AdminControlPanel key={JSON.stringify(scope)} id={alert.id} scope={scope} targets={targets} onRefresh={refresh} />;
}
export function AdminAlertDetailPage({ id }: { id: string }) {
  const { t } = useT("admin");
  const nav = useNavigation();
  const query = useAdminAlert(id);
  const alert = query.data;
  const resolution = alert?.resolutionCode;
  const timezone = nav.searchParams.get("timezone") ?? "UTC";
  const subjectList = alert?.subjectKind === "installation" ? "/admin/installations" : "/admin/tasks";
  const subjectParams = new URLSearchParams({ timezone });
  const instant = (value: string | null) => value ? <time dateTime={value}>{formatAdminTime(value, timezone)}</time> : t($ => $.observability.unknown);
  return <section className="space-y-6"><AppLink href={adminReturnHref(nav.searchParams, "/admin/alerts")} className="inline-flex min-h-11 items-center text-body underline underline-offset-4">{t($ => $.alerts.back)}</AppLink><ObservationHeader title={t($ => $.alerts.detail)} description={t($ => $.alerts.description)} />
    <ObservationState pending={query.isPending} error={query.isError} retry={() => void query.refetch()}>{alert && <>
      <div className="flex flex-wrap items-center justify-between gap-3"><div><h2 className="text-body-lg font-semibold">{t($ => $.alerts.rules[alert.rule])}</h2><p className="break-all text-caption text-muted-foreground">{alert.id}</p></div><Button variant="outline" onClick={() => void query.refetch()}>{t($ => $.observability.refresh)}</Button></div>
      <AlertControls alert={alert} refresh={() => query.refetch({ throwOnError: true })} />
      <dl className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">{[
        [t($ => $.alerts.status), t($ => $.alerts.statuses[alert.status])], [t($ => $.alerts.severity), t($ => $.alerts.severities[alert.severity])], [t($ => $.alerts.condition), alert.rule === "execution_failed" ? t($ => $.alerts.failureRecord) : alert.conditionActive === null ? t($ => $.observability.unknown) : alert.conditionActive ? t($ => $.alerts.active) : t($ => $.alerts.recovered)],
        [t($ => $.alerts.assignee), alert.assigneeId ?? t($ => $.alerts.unassigned)], [t($ => $.alerts.occurrences), alert.occurrenceCount], [t($ => $.alerts.version), alert.version],
        [t($ => $.alerts.firstSeen), instant(alert.firstSeenAt)], [t($ => $.alerts.lastSeen), instant(alert.lastSeenAt)], [t($ => $.alerts.acknowledgedAt), instant(alert.acknowledgedAt)], [t($ => $.alerts.resolvedAt), instant(alert.resolvedAt)], [t($ => $.alerts.closedAt), instant(alert.closedAt)], [t($ => $.alerts.resolution), resolution ? t($ => $.alerts.resolutions[resolution]) : t($ => $.observability.unknown)],
      ].map(([label, value]) => <div key={String(label)} className="min-w-0 space-y-1"><dt className="text-caption text-muted-foreground">{label}</dt><dd className="break-all text-body">{value}</dd></div>)}</dl>
      {alert.rule === "execution_failed" && <p className="max-w-prose text-caption text-muted-foreground">{t($ => $.alerts.failureRecordHint)}</p>}
      <div className="space-y-2"><p className="text-caption text-muted-foreground">{t($ => $.alerts.subject)}</p>{alert.subjectKind === "unknown" ? <p className="break-all text-body">{alert.subjectId}</p> : <AppLink href={adminDetailHref(`${subjectList}/${alert.subjectId}`, subjectList, subjectParams)} className="inline-flex min-h-11 items-center break-all text-body underline underline-offset-4">{alert.subjectId}</AppLink>}{alert.relatedTaskId && <p><AppLink href={adminDetailHref(`/admin/tasks/${alert.relatedTaskId}`, "/admin/tasks", subjectParams)} className="inline-flex min-h-11 items-center break-all text-body underline underline-offset-4">{t($ => $.alerts.relatedTask)}: {alert.relatedTaskId}</AppLink></p>}</div>
    </>}</ObservationState></section>;
}
