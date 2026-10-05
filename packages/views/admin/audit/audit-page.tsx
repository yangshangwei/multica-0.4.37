"use client";

import { useState } from "react";
import { adminDetailHref, useAdminAudit, useAdminObservationScope, type AdminAudit } from "@multica/core/admin";
import { Button } from "@multica/ui/components/ui/button";
import { Badge } from "@multica/ui/components/ui/badge";
import { copyText } from "@multica/ui/lib/clipboard";
import { useT } from "../../i18n";
import { AppLink } from "../../navigation";
import { AdminListPagination, adminTouchLinkClass, formatAdminTime } from "../executions/list-controls";
import { ObservationFilters, ObservationHeader, ObservationState, useObservationParams } from "../observability/common";
import styles from "../admin-visual.module.css";
import { auditActionCodes, auditCodeEntry, auditPhaseCodes, auditResultCodes, auditTargetKinds } from "./audit-code-labels";

export function AdminAuditPage() {
  const { t } = useT("admin");
  const { scope } = useAdminObservationScope();
  const params = useObservationParams();
  const query = useAdminAudit(params);
  const data = query.data;
  return <section className="space-y-6">
    <ObservationHeader title={t($ => $.audit.title)} description={t($ => $.audit.description)} />
    <ObservationFilters params={params} advanced maxWindowDays={90} fields={[
      { name: "action", label: t($ => $.audit.action) },
      { name: "actor_user_id", label: t($ => $.audit.actor) },
      { name: "target_kind", label: t($ => $.audit.targetKind) },
      { name: "target_id", label: t($ => $.audit.target) },
      { name: "phase", label: t($ => $.audit.phase) },
    ]} />
    <ObservationState pending={query.isPending} error={query.isError} quality={data?.dataQuality} asOf={data?.asOf} timezone={params.get("timezone") ?? "UTC"} empty={data?.items.length === 0} retry={() => void query.refetch()}>
      {data && <ol className="divide-y divide-surface-border">
        {data.items.map(event => <AuditEvent key={event.id} event={event} ownOperation={event.actorUserId === scope.userId} params={params} />)}
      </ol>}
    </ObservationState>
    {data && !query.isError && <AdminListPagination cursor={data.nextCursor} asOf={data.asOf} params={params} refresh={() => void query.refetch()} />}
  </section>;
}

function AuditEvent({ event, ownOperation, params }: { event: AdminAudit["items"][number]; ownOperation: boolean; params: URLSearchParams }) {
  const { t } = useT("admin");
  const [copyState, setCopyState] = useState<"copied" | "failed" | null>(null);
  const actionKey = auditCodeEntry(auditActionCodes, event.action);
  const result = auditCodeEntry(auditResultCodes, event.resultCode);
  const resultLabel = result ? t($ => $.audit.resultCodes[result.label]) : event.resultCode;
  const phaseKey = auditCodeEntry(auditPhaseCodes, event.phase);
  const targetKey = auditCodeEntry(auditTargetKinds, event.targetKind);
  const targetLabel = targetKey ? t($ => $.audit.targetKinds[targetKey]) : event.targetKind;
  const targetId = event.targetId.length > 18 ? `${event.targetId.slice(0, 8)}…${event.targetId.slice(-4)}` : event.targetId;
  return <li className={`space-y-2 py-4 first:pt-0 ${styles.compactRow}`}>
    <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
      <div className="flex min-w-0 flex-wrap items-center gap-2">
        <h2 className="min-w-0 text-body font-semibold [overflow-wrap:anywhere]">{actionKey ? t($ => $.audit.actionCodes[actionKey]) : event.action}</h2>
        <Badge variant="outline" className={`${styles.statusBadge} ${styles[result?.tone ?? "statusNeutral"]}`} aria-label={`${t($ => $.audit.resultLabel)}: ${resultLabel}`}>{resultLabel}</Badge>
      </div>
      <time className="text-caption text-muted-foreground" dateTime={event.createdAt}>{formatAdminTime(event.createdAt, params.get("timezone") ?? "UTC")}</time>
    </div>
    <div className="flex flex-wrap items-baseline gap-x-4 gap-y-1 text-caption">
      <p className="break-all"><span className="text-muted-foreground">{t($ => $.audit.actorName)}: </span>{event.actorDisplayName ?? event.actorUserId ?? event.actorKind}</p>
      <p className="break-all"><span className="text-muted-foreground">{t($ => $.audit.targetSummary)}: </span><span>{targetLabel}</span>{" "}<span title={event.targetId}>{targetId}</span></p>
    </div>
    {event.actorSnapshotQuality === "unknown" && <p className="text-caption text-muted-foreground">{t($ => $.audit.snapshotUnknown)}</p>}
    <details>
      <summary className="cursor-pointer text-caption text-muted-foreground">{t($ => $.audit.details)}</summary>
      <div className={`space-y-4 pt-3 ${styles.detailSection}`}>
        <dl className={styles.metadataGrid}>
          {event.actorUserId && <div className="min-w-0 space-y-1">
            <dt className="text-caption text-muted-foreground">{t($ => $.audit.actor)}</dt>
            <dd className="space-y-1"><p className={`select-all ${styles.codeValue}`}>{event.actorUserId}</p>
              <Button type="button" variant="ghost" size="sm" onClick={async () => setCopyState(await copyText(event.actorUserId!) ? "copied" : "failed")}>{t($ => $.audit.copyActorId)}</Button>
              {copyState && <p role={copyState === "failed" ? "alert" : "status"} className="text-caption text-muted-foreground">{copyState === "copied" ? t($ => $.audit.copiedActorId) : t($ => $.audit.copyFailed)}</p>}
            </dd>
          </div>}
          {[
            [t($ => $.audit.action), event.action],
            [t($ => $.audit.result), event.resultCode],
            [t($ => $.audit.targetKind), event.targetKind],
            [t($ => $.audit.target), event.targetId],
            [t($ => $.audit.request), event.requestId],
          ].map(([label, value]) => <div key={label} className="min-w-0 space-y-1"><dt className="text-caption text-muted-foreground">{label}</dt><dd className={`select-all ${styles.codeValue}`}>{value}</dd></div>)}
          <div className="min-w-0 space-y-1"><dt className="text-caption text-muted-foreground">{t($ => $.audit.phase)}</dt><dd className="space-y-1 text-body">{phaseKey && <p>{t($ => $.audit.phaseCodes[phaseKey])}</p>}<p className={`select-all ${styles.codeValue}`}>{event.phase}</p></dd></div>
          {event.operationId && <div className="min-w-0 space-y-1"><dt className="text-caption text-muted-foreground">{t($ => $.audit.operation)}</dt><dd className="break-all text-body">{ownOperation ? <AppLink href={adminDetailHref(`/admin/operations/${event.operationId}`, "/admin/audit", params)} className={`inline-block underline underline-offset-4 ${adminTouchLinkClass}`}>{t($ => $.audit.openReceipt)}</AppLink> : event.operationId}</dd></div>}
        </dl>
        <p className="whitespace-pre-wrap break-words text-body">{t($ => $.audit.reason)}: {event.reason}</p>
        <div className="grid gap-4 sm:grid-cols-2">
          {[[t($ => $.audit.before), event.beforeState], [t($ => $.audit.after), event.afterState]].map(([label, snapshot]) => <div key={String(label)} className="min-w-0"><h3 className="text-caption text-muted-foreground">{String(label)}</h3><pre className="overflow-x-auto whitespace-pre-wrap break-all text-caption">{JSON.stringify(snapshot, null, 2)}</pre></div>)}
        </div>
      </div>
    </details>
  </li>;
}
