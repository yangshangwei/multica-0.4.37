"use client";

import { useRef, useState } from "react";
import { AdminCollaborationRejectedError, AdminControlDraftError, dismissAdminCollaborationReceipt,
  useAdminCollaborationDraft, useAdminCollaborationMutation, useAdminCollaborationOperation,
  type AdminCollaborationChange, type AdminCollaborationDetail, type AdminCollaborationInput, type AdminScope } from "@multica/core/admin";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../../i18n";
import { AdminCollaborationForm } from "./collaboration-form";
import { AdminCollaborationReceipt } from "./collaboration-receipt";
import { recoverFocus } from "../../collaboration/focus";

export function AdminCollaborationControls({ scope, issueId, detail, refreshing }: {
  scope: AdminScope; issueId: string; detail: AdminCollaborationDetail | null; refreshing: boolean;
}) {
  const { t } = useT("admin");
  const saved = useAdminCollaborationDraft(scope, issueId);
  const [snapshot, setSnapshot] = useState<{ detail: AdminCollaborationDetail; action: AdminCollaborationChange["action"] } | null>(null);
  const [sent, setSent] = useState<AdminCollaborationInput | null>(null);
  const [rejected, setRejected] = useState(false);
  const [feedback, setFeedback] = useState("");
  const trigger = useRef<HTMLButtonElement | null>(null);
  const triggerAction = useRef<AdminCollaborationChange["action"] | null>(null);
  const panel = useRef<HTMLDivElement | null>(null);
  const submitting = useRef(false);
  const mutation = useAdminCollaborationMutation(scope);
  const input = sent ?? saved.draft?.input ?? null;
  const lookup = useAdminCollaborationOperation(scope, input, !sent && !!saved.draft);
  const receipt = !lookup.isError ? lookup.data ?? mutation.data : undefined;
  const busy = mutation.isPending || lookup.isFetching;
  const buttonClass = "h-auto min-h-11 whitespace-normal text-left";

  function close() {
    setSnapshot(null); setSent(null); setRejected(false); setFeedback(""); mutation.reset(); saved.reload();
    requestAnimationFrame(() => (trigger.current ?? panel.current)?.focus());
  }
  async function submit(value: AdminCollaborationInput) {
    if (busy || submitting.current) return;
    submitting.current = true;
    setSent(value); setFeedback("");
    // Sending hides the form that held focus; the receipt replaces it in this panel.
    recoverFocus(() => panel.current);
    try { await mutation.mutateAsync(value); }
    catch (error) {
      if (error instanceof AdminCollaborationRejectedError) { setRejected(true); setFeedback(t($ => $.collaboration.rejected)); }
      else if (error instanceof AdminControlDraftError) { setSent(null); setFeedback(t($ => $.operations.storageFailed)); }
      else setFeedback(t($ => $.collaboration.failed));
    } finally { submitting.current = false; }
  }
  if (saved.loading) return <p role="status" className="text-body text-muted-foreground">{t($ => $.operations.restoring)}</p>;
  if (saved.error) return <p role="alert" className="text-body">{t($ => $.operations.storageFailed)}</p>;
  return <div ref={panel} tabIndex={-1} role="group" aria-label={t($ => $.collaboration.title)} className="min-w-0 space-y-4 rounded-md outline-none focus-visible:ring-2 focus-visible:ring-ring">
    {!input && !snapshot && detail && <div className="flex flex-wrap gap-3">
      {detail.actions.map(action => <Button key={action.action} variant="outline" className={buttonClass} disabled={refreshing} ref={node => {
        if (triggerAction.current === action.action) trigger.current = node;
      }} onClick={event => { triggerAction.current = action.action; trigger.current = event.currentTarget; setSnapshot({ detail, action: action.action }); }}>{t($ => $.collaboration.actions[action.action])}</Button>)}
    </div>}
    {snapshot && <div hidden={!!input}>
      <AdminCollaborationForm scope={scope} snapshot={snapshot.detail} action={snapshot.action} current={detail} refreshing={refreshing} disabled={!!input}
        onSubmit={value => { void submit(value); }} onClose={close} />
    </div>}
    {input && <div className="min-w-0 space-y-4 rounded-md border border-surface-border p-4 sm:p-5">
      <AdminCollaborationReceipt input={input} receipt={receipt ?? undefined} />
      {busy && <p role="status" className="text-body text-muted-foreground">{t($ => $.operations.restoring)}</p>}
      {!receipt && !busy && !rejected && lookup.isSuccess && lookup.data === null && <p role="status" className="text-body">{t($ => $.collaboration.notRecorded)}</p>}
      {!receipt && lookup.isError && <p role="alert" className="text-body">{t($ => $.collaboration.failed)}</p>}
      <div className="flex flex-wrap gap-2">
        {receipt ? <Button variant="ghost" className={buttonClass} onClick={() => {
          try { dismissAdminCollaborationReceipt(scope, input, receipt); close(); }
          catch { setFeedback(t($ => $.operations.storageFailed)); }
        }}>{t($ => $.collaboration.receiptDismiss)}</Button>
          : rejected ? <Button variant="ghost" className={buttonClass} onClick={close}>{t($ => $.collaboration.close)}</Button>
          : <>
            <Button variant="outline" className={buttonClass} disabled={busy} onClick={() => { setFeedback(""); void lookup.refetch(); }}>{t($ => $.collaboration.check)}</Button>
            <Button variant="ghost" className={buttonClass} disabled={busy} onClick={() => { void submit(input); }}>{t($ => $.collaboration.retryOriginal)}</Button>
          </>}
      </div>
    </div>}
    {feedback && <p role="alert" className="text-body">{feedback}</p>}
  </div>;
}
