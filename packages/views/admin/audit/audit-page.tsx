"use client";
import { useAdminAudit, useAdminObservationScope } from "@multica/core/admin";
import { useT } from "../../i18n";
import { AppLink } from "../../navigation";
import { AdminListPagination, formatAdminTime } from "../executions/list-controls";
import { ObservationFilters, ObservationHeader, ObservationState, useObservationParams } from "../observability/common";
export function AdminAuditPage() {
  const { t } = useT("admin");
  const { scope } = useAdminObservationScope();
  const params = useObservationParams();
  const query = useAdminAudit(params);
  const data = query.data;
  return <section className="space-y-6"><ObservationHeader title={t($ => $.audit.title)} description={t($ => $.audit.description)} />
    <ObservationFilters params={params} fields={[{ name: "action", label: t($ => $.audit.action) }, { name: "actor_user_id", label: t($ => $.audit.actor) }, { name: "target_kind", label: t($ => $.audit.targetKind) }, { name: "target_id", label: t($ => $.audit.target) }, { name: "phase", label: t($ => $.audit.phase) }]} />
    <ObservationState pending={query.isPending} error={query.isError} quality={data?.dataQuality} asOf={data?.asOf} timezone={params.get("timezone") ?? "UTC"} empty={data?.items.length === 0} retry={() => void query.refetch()}>{data && <ol className="divide-y divide-surface-border">{data.items.map(event => <li key={event.id} className="space-y-4 py-5 first:pt-0"><div className="flex flex-wrap items-baseline justify-between gap-3"><h2 className="break-all text-body-lg font-medium">{event.action}</h2><time className="text-caption text-muted-foreground" dateTime={event.createdAt}>{formatAdminTime(event.createdAt, params.get("timezone") ?? "UTC")}</time></div>
      <dl className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3"><div className="min-w-0 space-y-1"><dt className="text-caption text-muted-foreground">{t($ => $.audit.actor)}</dt><dd className="break-all text-body">{event.actorDisplayName ?? event.actorUserId ?? event.actorKind}</dd>{event.actorSnapshotQuality === "unknown" && <dd className="text-caption text-muted-foreground">{t($ => $.audit.snapshotUnknown)}</dd>}</div>{[[t($ => $.audit.target), `${event.targetKind}: ${event.targetId}`], [t($ => $.audit.phase), event.phase], [t($ => $.audit.result), event.resultCode], [t($ => $.audit.request), event.requestId]].map(([label, value]) => <div key={label} className="min-w-0 space-y-1"><dt className="text-caption text-muted-foreground">{label}</dt><dd className="break-all text-body">{value}</dd></div>)}
      {event.operationId && <div className="min-w-0 space-y-1"><dt className="text-caption text-muted-foreground">{t($ => $.audit.operation)}</dt><dd className="break-all text-body">{event.actorUserId === scope.userId ? <AppLink href={`/admin/operations/${event.operationId}`} className="underline underline-offset-4">{t($ => $.audit.openReceipt)}</AppLink> : event.operationId}</dd></div>}</dl>
      <details className="space-y-3"><summary className="cursor-pointer text-body text-muted-foreground">{t($ => $.audit.details)}</summary><p className="whitespace-pre-wrap break-words text-body">{t($ => $.audit.reason)}: {event.reason}</p><div className="grid gap-4 sm:grid-cols-2">{[[t($ => $.audit.before), event.beforeState], [t($ => $.audit.after), event.afterState]].map(([label, snapshot]) => <div key={String(label)} className="min-w-0"><h3 className="text-caption text-muted-foreground">{String(label)}</h3><pre className="overflow-x-auto whitespace-pre-wrap break-all text-caption">{JSON.stringify(snapshot, null, 2)}</pre></div>)}</div></details>
    </li>)}</ol>}</ObservationState>
    {data && !query.isError && <AdminListPagination cursor={data.nextCursor} asOf={data.asOf} params={params} refresh={() => void query.refetch()} />}
  </section>;
}
