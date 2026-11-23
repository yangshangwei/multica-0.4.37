import { useRef, useState } from "react";
import { dismissKnowledgeAutomationOperation, KnowledgeAutomationReceiptCleanupError, KnowledgeRecoveryStorageError, KnowledgeUnusedIntentError,
  readPendingKnowledgeAutomationOperations, useKnowledgeAutomationAction, type CollaborationScope, type KnowledgeAutomationIdentity,
  type KnowledgeAutomationIntent, type KnowledgeAutomationReceipt } from "@multica/core/collaboration";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../i18n";

type Owner = { kind: "policy"; projectId: string } | { kind: "retry"; issueId: string };

/** Client-owned request state survives leased reads; no source material is copied here. */
export function useKnowledgeAutomationControl(scope: CollaborationScope, owner: Owner, onSuccess?: () => void) {
  const { t } = useT("collaboration");
  const mutation = useKnowledgeAutomationAction(scope);
  const busy = useRef(false);
  const [intent, setIntent] = useState<KnowledgeAutomationIntent | null>(null);
  const [settled, setSettled] = useState<string[]>([]);
  const [unconfirmed, setUnconfirmed] = useState<string[]>([]);
  const [, setRevision] = useState(0);
  const [feedback, setFeedback] = useState<"success" | "refused" | "uncertain" | "storage" | "cleanup" | null>(null);
  const owns = (identity: KnowledgeAutomationIdentity) => owner.kind === "policy"
    ? identity.kind === "policy" && identity.projectId === owner.projectId : identity.kind === "retry" && identity.issueId === owner.issueId;
  let stored: KnowledgeAutomationIdentity[] = [], storageFailed = false;
  try { stored = readPendingKnowledgeAutomationOperations(scope).filter(owns); } catch { storageFailed = true; }
  const pending = [...new Map([...stored, ...(intent ? [intent] : [])].filter((item) => !settled.includes(item.requestId)).map((item) => [item.requestId, item])).values()];
  const finish = (receipt: KnowledgeAutomationReceipt) => {
    const success = receipt.code === "knowledge_policy_updated" || receipt.code === "knowledge_job_retried";
    setIntent((current) => current?.requestId === receipt.request_id ? null : current);
    setSettled((current) => [...new Set([...current, receipt.request_id])]);
    setFeedback(success ? "success" : "refused");
    if (success) onSuccess?.();
  };
  async function run(identity: KnowledgeAutomationIdentity, next?: KnowledgeAutomationIntent) {
    if (busy.current) return;
    busy.current = true; setFeedback(null);
    if (next) setIntent(next);
    try { finish(await mutation.mutateAsync({ identity, intent: next })); }
    catch (error) {
      if (error instanceof KnowledgeAutomationReceiptCleanupError) { finish(error.receipt); setFeedback("cleanup"); }
      else if (error instanceof KnowledgeUnusedIntentError) {
        setIntent(null); setSettled((current) => [...new Set([...current, identity.requestId])]); setFeedback("storage");
      } else {
        setFeedback(error instanceof KnowledgeRecoveryStorageError ? "storage" : "uncertain");
        if (!next) setUnconfirmed((current) => [...new Set([...current, identity.requestId])]);
      }
    } finally { busy.current = false; setRevision((current) => current + 1); }
  }
  function dismiss(identity: KnowledgeAutomationIdentity) {
    if (busy.current || intent?.requestId === identity.requestId) return;
    try { dismissKnowledgeAutomationOperation(scope, identity); setFeedback(null); }
    catch { setFeedback("storage"); }
    setRevision((current) => current + 1);
  }
  const feedbackText = feedback === "success" ? owner.kind === "policy" ? t(($) => $.knowledge.automation.policySaved) : t(($) => $.knowledge.automation.retryAccepted)
    : feedback === "refused" ? t(($) => $.knowledge.automation.confirmedRefusal)
      : feedback === "uncertain" ? t(($) => $.knowledge.notFound)
        : feedback === "cleanup" ? t(($) => $.knowledge.recoveryCleanupFailed) : feedback === "storage" ? t(($) => $.knowledge.storageError) : null;
  return {
    locked: mutation.isPending || pending.length > 0 || storageFailed,
    working: mutation.isPending,
    pendingCount: pending.length,
    submit: (next: KnowledgeAutomationIntent) => run(next, next),
    notice: feedbackText || storageFailed || pending.length > 0 ? <div className="min-w-0 space-y-3">
      {feedbackText && <p role={feedback === "success" ? "status" : "alert"} className="text-caption text-muted-foreground">{feedbackText}</p>}
      {storageFailed && <p role="alert" className="text-caption text-destructive">{t(($) => $.knowledge.recoveryReadError)}</p>}
      {pending.length > 0 && <div className="space-y-2">
        <p role={mutation.isPending ? "status" : undefined} className="text-caption font-medium">{mutation.isPending ? t(($) => $.knowledge.working) : t(($) => $.knowledge.automation.unconfirmedTitle)}</p>
        <p className="text-caption text-muted-foreground">{t(($) => $.knowledge.automation.unconfirmedHint)}</p>
        {pending.map((identity) => <div key={identity.requestId} className="flex min-w-0 flex-wrap gap-2">
          <Button type="button" variant="outline" size="sm" disabled={mutation.isPending} onClick={() => void run(identity)}>{t(($) => $.knowledge.recover)}</Button>
          {intent?.requestId === identity.requestId && <Button type="button" variant="outline" size="sm" disabled={mutation.isPending} onClick={() => void run(intent, intent)}>{t(($) => $.knowledge.retry)}</Button>}
          {intent?.requestId !== identity.requestId && unconfirmed.includes(identity.requestId) && <Button type="button" variant="ghost" size="sm" disabled={mutation.isPending} onClick={() => dismiss(identity)}>{t(($) => $.knowledge.automation.removeLocal)}</Button>}
        </div>)}
        {pending.some((identity) => intent?.requestId !== identity.requestId && unconfirmed.includes(identity.requestId)) && <p className="text-caption text-muted-foreground">{t(($) => $.knowledge.automation.removeHint)}</p>}
      </div>}
    </div> : null,
  };
}
