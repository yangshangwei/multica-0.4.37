"use client";
import { useAdminOperation, type AdminOperation, type AdminScope } from "@multica/core/admin";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../../i18n";
import { AppLink } from "../../navigation";
import { formatAdminTime } from "../executions/list-controls";

const knownResults = ["cancelled_before_dispatch", "awaiting_daemon_confirmation", "confirmation_unavailable", "already_terminal", "already_cancelled_unverified", "daemon_stopped", "admission_stopped", "admission_accepting", "execution_fence_conflict", "unknown"] as const;
const alertResults = ["alert_acknowledged", "alert_assigned", "alert_closed", "alert_version_conflict", "alert_state_conflict", "alert_assignee_unavailable", "alert_resolution_invalid"] as const;
function resultKey(code: string): typeof knownResults[number] {
  return knownResults.find(value => value === code) ?? "unknown";
}
export function AdminOperationReceipt({ scope, id, initialData, requestKey, detail = false }: {
  scope: AdminScope; id: string; initialData?: AdminOperation; requestKey?: string; detail?: boolean;
}) {
  const { t } = useT("admin");
  const query = useAdminOperation(scope, id, initialData);
  const receipt = query.data;
  const alertResult = alertResults.find(code => code === receipt?.resultCode);
  const alertOperation = receipt && ["alert.acknowledge", "alert.assign", "alert.close"].includes(receipt.kind);
  return <section aria-labelledby="operation-receipt-title" className="space-y-4 rounded-md border border-surface-border p-5">
    <div className="flex flex-wrap items-center justify-between gap-3">
      <h2 id="operation-receipt-title" className="text-body-lg font-semibold">{t($ => $.operations.receipt)}</h2>
      <Button variant="outline" disabled={query.isFetching} onClick={() => void query.refetch()}>{t($ => $.operations.refresh)}</Button>
    </div>
    {query.isError && <p role="alert" className="text-body">{t($ => $.operations.readFailed)}</p>}
    {receipt && <>
      <div role="status" aria-live="polite" className="space-y-3">
        <dl className="grid gap-4 md:grid-cols-3">
          <div><dt className="text-caption text-muted-foreground">{t($ => $.operations.state)}</dt><dd className="text-body font-medium">{t($ => $.operations.states[receipt.state])}</dd></div>
          <div><dt className="text-caption text-muted-foreground">{t($ => $.operations.confirmation)}</dt><dd className="text-body">{t($ => $.operations.confirmations[receipt.confirmation])}</dd></div>
          <div><dt className="text-caption text-muted-foreground">{t($ => $.operations.reconciliation)}</dt><dd className="text-body">{t($ => $.operations.reconciliations[receipt.reconciliationState])}</dd></div>
        </dl>
        <p className="max-w-prose text-body">{alertResult ? t($ => $.alerts.results[alertResult]) : t($ => $.operations.results[resultKey(receipt.resultCode)])}</p>
      </div>
      <dl className="grid gap-3 text-caption sm:grid-cols-2">
        {[[t($ => $.operations.operation), receipt.id], [t($ => $.operations.target), receipt.targetId], [t($ => $.operations.root), receipt.rootOperationId], [t($ => $.operations.key), requestKey]].filter(([, value]) => !!value).map(([label, value]) => <div key={label} className="min-w-0"><dt className="text-muted-foreground">{label}</dt><dd className="break-all">{value}</dd></div>)}
        {[[t($ => $.operations.accepted), receipt.acceptedAt], [t($ => $.operations.appliedAt), receipt.appliedAt], [t($ => $.operations.confirmedAt), receipt.confirmedAt], [t($ => $.operations.deadline), receipt.ackDeadline], [t($ => $.operations.updated), receipt.updatedAt]].map(([label, value]) => <div key={label}><dt className="text-muted-foreground">{label}</dt><dd>{value ? <time dateTime={value}>{formatAdminTime(value)}</time> : t($ => $.operations.unknown)}</dd></div>)}
      </dl>
      {!detail && <AppLink className="inline-flex min-h-11 items-center text-body underline underline-offset-4" href={`/admin/operations/${receipt.id}`}>{t($ => $.operations.openReceipt)}</AppLink>}
      {detail && (alertOperation || receipt.kind === "task.cancel" || receipt.kind === "installation.admission") && <AppLink className="inline-flex min-h-11 items-center text-body underline underline-offset-4" href={`/admin/${alertOperation ? "alerts" : receipt.kind === "task.cancel" ? "tasks" : "installations"}/${receipt.targetId}`}>{t($ => $.operations.back)}</AppLink>}
    </>}
    <p className="max-w-prose text-caption text-muted-foreground">{t($ => $.operations.manual)}</p>
  </section>;
}
