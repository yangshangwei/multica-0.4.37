"use client";
import { useEffect, useId, useRef, useState, type FormEvent } from "react";
import { ApiError } from "@multica/core/api";
import {
  AdminControlUncertainError, AdminControlDraftError, useAdminControlMutation, useAdminControlLookup,
  adminAlertChangeSchema, type AdminAlertAction, type AdminControlInput, type AdminExecutionFence, type AdminOperation, type AdminScope,
} from "@multica/core/admin";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useT } from "../../i18n";
import { useNavigation } from "../../navigation";
import { formatAdminTime } from "../executions/list-controls";
import { AdminOperationReceipt } from "./operation-receipt";
import { AdminAlertFields } from "../alerts/alert-fields";

export type AdminControlTarget = { id: string } & (
  { action: "cancel"; fence: AdminExecutionFence } |
  { action: "admission"; admission: "accepting" | "stopped"; version: string } |
  { action: "alert"; alertAction: AdminAlertAction; version: string; requiresResolution?: boolean }
);
export function AdminControlForm({ scope, target, restoredInput, onClose, onRefresh }: {
  scope: AdminScope; target: AdminControlTarget; restoredInput?: AdminControlInput; onClose(): void; onRefresh(): Promise<unknown> | void;
}) {
  const { t } = useT("admin");
  const nav = useNavigation();
  const [key] = useState(() => restoredInput?.key ?? crypto.randomUUID());
  const snapshot = useRef<AdminControlInput | null>(restoredInput ?? null);
  const restoreAttempted = useRef(false);
  const [restoring, setRestoring] = useState(!!restoredInput);
  const [refreshing, setRefreshing] = useState(false);
  const mutation = useAdminControlMutation(scope);
  const lookup = useAdminControlLookup(scope);
  const [uncertain, setUncertain] = useState(!!restoredInput);
  const [conflict, setConflict] = useState(false);
  const [feedback, setFeedback] = useState("");
  const [reasonInvalid, setReasonInvalid] = useState(false);
  const reasonRef = useRef<HTMLTextAreaElement>(null);
  const reasonErrorId = useId();
  const [receipt, setReceipt] = useState<AdminOperation | null>(null);
  const working = restoring || refreshing || mutation.isPending || lookup.isPending;
  const action = target.action === "cancel" ? "cancel" : target.action === "admission" && target.admission === "stopped" ? "stop" : "resume";
  const notice = target.action === "cancel" ? "cancelNotice" : target.action === "admission" && target.admission === "stopped" ? "stopNotice" : "resumeNotice";
  const alertNotice = target.action === "alert" ? target.alertAction === "acknowledge" ? "acknowledgeNotice" : target.alertAction === "assign" ? "assignNotice" : target.requiresResolution ? "closeNotice" : "closeRecoveredNotice" : null;

  const restoreLookup = lookup.mutateAsync;
  useEffect(() => {
    if (!restoredInput || restoreAttempted.current) return;
    restoreAttempted.current = true;
    void restoreLookup(restoredInput).then(result => {
      if (result) { setReceipt(result); setUncertain(false); void Promise.resolve().then(onRefresh).catch(() => {}); }
      else setFeedback(t($ => $.operations.notRecorded));
    }).catch(() => setFeedback(t($ => $.operations.uncertain))).finally(() => setRestoring(false));
  }, [restoredInput, restoreLookup, onRefresh, t]);

  function accept(result: AdminOperation) {
    setReceipt(result); setUncertain(false);
    // A failed metadata refresh does not invalidate the durable receipt.
    void Promise.resolve().then(onRefresh).catch(() => {});
  }
  async function refreshTarget() {
    setRefreshing(true);
    try { await onRefresh(); onClose(); }
    catch { setFeedback(t($ => $.operations.failed)); }
    finally { setRefreshing(false); }
  }
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (working || conflict) return;
    const values = new FormData(event.currentTarget);
    const reason = String(values.get("reason") ?? "").trim();
    if (!reason) {
      setReasonInvalid(true);
      reasonRef.current?.focus();
      return;
    }
    setReasonInvalid(false);
    let alertInput: AdminControlInput | null = null;
    if (!snapshot.current && target.action === "alert") {
      const parsed = adminAlertChangeSchema.safeParse({ action: target.alertAction, expectedVersion: target.version, reason,
        assigneeId: String(values.get("assignee") ?? "") || null,
        resolutionCode: values.get("resolution") ?? undefined, relatedTaskId: String(values.get("relatedTask") ?? "") || null });
      if (!parsed.success) { setFeedback(t($ => $.operations.failed)); return; }
      alertInput = { id: target.id, key, action: "alert", body: parsed.data };
    }
    const input: AdminControlInput | null = snapshot.current ?? alertInput ?? (target.action === "cancel"
      ? { id: target.id, key, action: "cancel", body: { expectedExecutionFence: target.fence, reason } }
      : target.action === "admission" ? { id: target.id, key, action: "admission", body: { admission: target.admission, expectedAdmissionVersion: target.version, reason } } : null);
    if (!input) return;
    snapshot.current = input;
    setFeedback("");
    try { accept(await mutation.mutateAsync(input)); }
    catch (error) {
      if (error instanceof AdminControlUncertainError) { setUncertain(true); setFeedback(t($ => $.operations.uncertain)); }
      else if (error instanceof ApiError && error.status === 409) { setConflict(true); setUncertain(false); setFeedback(t($ => $.operations.conflict)); }
      else if (error instanceof AdminControlDraftError) { setFeedback(t($ => $.operations.storageFailed)); }
      else { setFeedback(t($ => $.operations.failed)); }
    } finally { mutation.reset(); }
  }
  async function check() {
    if (!snapshot.current) return;
    try {
      const result = await lookup.mutateAsync(snapshot.current);
      if (result) accept(result);
      else setFeedback(t($ => $.operations.notRecorded));
    } catch { setFeedback(t($ => $.operations.uncertain)); }
    finally { lookup.reset(); }
  }
  if (receipt) return <div className="space-y-3"><AdminOperationReceipt key={receipt.id} scope={scope} id={receipt.id} initialData={receipt} requestKey={key} /><Button variant="ghost" onClick={onClose}>{t($ => $.operations.dismiss)}</Button></div>;
  return <section aria-labelledby="control-action-title" className="space-y-4 rounded-md border border-surface-border p-5">
    <h2 id="control-action-title" className="text-body-lg font-semibold">{target.action === "alert" ? t($ => $.alerts[target.alertAction]) : t($ => $.operations[action])}</h2>
    <p className="max-w-prose text-body text-muted-foreground">{alertNotice ? t($ => $.alerts[alertNotice]) : t($ => $.operations[notice])}</p>
    <dl className="grid gap-3 text-caption sm:grid-cols-2">
      <div className="min-w-0"><dt className="text-muted-foreground">{t($ => $.operations.target)}</dt><dd className="break-all">{target.id}</dd></div>
      <div><dt className="text-muted-foreground">{t($ => $.operations.version)}</dt><dd>{target.action === "cancel" ? target.fence.targetVersion : target.version}</dd></div>
      {target.action === "cancel" && <><div className="min-w-0"><dt className="text-muted-foreground">{t($ => $.operations.runtime)}</dt><dd className="break-all">{target.fence.runtimeId ?? t($ => $.operations.notDispatched)}</dd></div><div><dt className="text-muted-foreground">{t($ => $.operations.dispatch)}</dt><dd>{target.fence.dispatchedAt ? formatAdminTime(target.fence.dispatchedAt, nav.searchParams.get("timezone") ?? "UTC") : t($ => $.operations.notDispatched)}</dd></div></>}
    </dl>
    <form className="grid max-w-xl gap-4" onSubmit={submit}>
      {target.action === "alert" && <AdminAlertFields scope={scope} action={target.alertAction} requiresResolution={target.requiresResolution === true} locked={working || !!snapshot.current} restored={snapshot.current?.action === "alert" ? snapshot.current.body : undefined} />}
      <div className="space-y-2">
        <label className="space-y-1 text-caption">{t($ => $.operations.reason)}<Textarea ref={reasonRef} autoFocus name="reason" required maxLength={1000} defaultValue={restoredInput?.body.reason ?? ""} readOnly={working || !!snapshot.current} aria-invalid={reasonInvalid || undefined} aria-describedby={reasonInvalid ? reasonErrorId : undefined} onChange={event => {
          if (event.target.value.trim()) setReasonInvalid(false);
        }} /></label>
        {reasonInvalid && <p id={reasonErrorId} role="alert" className="text-caption text-destructive">{t($ => $.operations.reasonRequired)}</p>}
      </div>
      {feedback && <p role="alert" className="text-body">{feedback}</p>}
      {uncertain && <p className="break-all text-caption">{t($ => $.operations.key)}: {key}</p>}
      <div className="flex flex-wrap gap-3">
        <Button type="submit" disabled={working || conflict}>{working ? t($ => $.operations.working) : uncertain ? t($ => $.operations.retryOriginal) : t($ => $.operations.confirm)}</Button>
        {uncertain && <Button type="button" variant="outline" disabled={working} onClick={() => void check()}>{t($ => $.operations.check)}</Button>}
        {conflict && <Button type="button" variant="outline" disabled={working} onClick={() => void refreshTarget()}>{t($ => $.operations.refreshTarget)}</Button>}
        <Button type="button" variant="ghost" disabled={working || (uncertain && !conflict)} onClick={onClose}>{t($ => $.operations.dismiss)}</Button>
      </div>
    </form>
  </section>;
}
