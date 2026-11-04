"use client";

import { useEffect, useId, useRef, useState } from "react";
import { useMutation, useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import { CollaborationUnavailableError, KnowledgeRecoveryStorageError, KnowledgeUnusedIntentError, KnowledgeReceiptCleanupError, collaborationKnowledgeCandidateOptions, dismissKnowledgeOperation, executeKnowledgeIntent, knowledgeTargetsEqual,
  readPendingKnowledgeOperations, recoverKnowledgeOperation,
  useCollaborationKnowledgeCandidates, useCollaborationKnowledgeTarget, useCollaborationScope,
  type AuthorizedKnowledgeProjection, type CollaborationIssue, type CollaborationKnowledgeCandidates, type CollaborationKnowledgeTarget, type CollaborationKnowledgeTargetList,
  type CollaborationKnowledgeReceipt, type CollaborationScope, type KnowledgeIntent, type KnowledgeOperationIdentity, type KnowledgePageOptions } from "@multica/core/collaboration";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../i18n";
import { KnowledgeState } from "./knowledge-controls";
import { KnowledgeProposalForm, type KnowledgeProposalMode } from "./knowledge-proposal";
import { KnowledgeReview } from "./knowledge-review";

export function CollaborationKnowledgePanel({ scope, issue }: { scope: CollaborationScope; issue: CollaborationIssue }) {
  return <SourceKnowledge key={`${scope.apiScope}:${scope.userId}:${issue.id}`} scope={scope} issue={issue} />;
}
function SourceKnowledge({ scope, issue }: { scope: CollaborationScope; issue: CollaborationIssue }) {
  const [page, setPage] = useState<KnowledgePageOptions>({});
  const query = useCollaborationKnowledgeCandidates(scope, issue.id, page);
  // cancelRefetch:false joins an in-flight refresh instead of restarting it.
  const refreshView = async () => (await query.refetch({ cancelRefetch: false })).data?.value;
  return <KnowledgeCollection scope={scope} issue={issue} query={query} page={page} setPage={setPage} sourceView={query.data?.value} refreshView={refreshView} />;
}
export function ManagedKnowledgePanel({ workspaceId, projectId }: { workspaceId: string; projectId?: string }) {
  const scope = useCollaborationScope();
  return <TargetKnowledge key={`${scope.apiScope}:${scope.userId}:${workspaceId}:${projectId ?? ""}`} scope={scope} workspaceId={workspaceId} projectId={projectId} />;
}
function TargetKnowledge({ scope, workspaceId, projectId }: { scope: CollaborationScope; workspaceId: string; projectId?: string }) {
  const [page, setPage] = useState<KnowledgePageOptions>({});
  const target: CollaborationKnowledgeTarget = projectId ? { kind: "project", workspace_id: workspaceId, project_id: projectId } : { kind: "workspace", workspace_id: workspaceId };
  const query = useCollaborationKnowledgeTarget(scope, target, page);
  return <KnowledgeCollection scope={scope} target={target} query={query} page={page} setPage={setPage} />;
}

type KnowledgePage = CollaborationKnowledgeCandidates | CollaborationKnowledgeTargetList;
function KnowledgeCollection({ scope, issue, target, query, page, setPage, sourceView, refreshView }: {
  scope: CollaborationScope; issue?: CollaborationIssue; target?: CollaborationKnowledgeTarget; query: UseQueryResult<AuthorizedKnowledgeProjection<KnowledgePage>, Error>;
  page: KnowledgePageOptions; setPage: (value: KnowledgePageOptions) => void; sourceView?: CollaborationKnowledgeCandidates;
  refreshView?: () => Promise<CollaborationKnowledgeCandidates | undefined>;
}) {
  const { t } = useT("collaboration"); const client = useQueryClient(); const id = useId(); const busy = useRef(false);
  const root = useRef<HTMLElement>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null), [proposal, setProposal] = useState<KnowledgeProposalMode | null>(null);
  const [intent, setIntent] = useState<KnowledgeIntent | null>(null), [, setRevision] = useState(0);
  const [feedback, setFeedback] = useState<{ text: string; failed: boolean } | null>(null);
  const [settledRequests, setSettledRequests] = useState<string[]>([]), [cleanupFailed, setCleanupFailed] = useState(false);
  const [openedElsewhere, setOpenedElsewhere] = useState(false);
  // Stored requests whose check in this mount settled nothing; only these may be removed locally.
  const [unconfirmed, setUnconfirmed] = useState<string[]>([]);
  // These are client-generated identities only, never a second server projection.
  let pending: KnowledgeOperationIdentity[] = [], storageError = false;
  try { pending = readPendingKnowledgeOperations(scope); } catch { storageError = true; }
  pending = pending.filter((identity) => !settledRequests.includes(identity.requestId));
  if (intent && !pending.some((identity) => identity.requestId === intent.requestId)) pending = [...pending, intent];
  const mutation = useMutation({ mutationKey: ["collaboration", scope.apiScope, scope.userId, "knowledge-ui-action"], retry: false, gcTime: 0,
    mutationFn: (operation: { identity: KnowledgeOperationIdentity; intent?: KnowledgeIntent }) => operation.intent
      ? executeKnowledgeIntent(client, scope, operation.intent) : recoverKnowledgeOperation(client, scope, operation.identity),
  });
  const locked = mutation.isPending || intent !== null;
  const current = query.data;
  const recoveryAttention = pending.length > 0 || storageError || cleanupFailed || feedback?.failed === true;
  useEffect(() => {
    if (!recoveryAttention) return;
    for (let ancestor = root.current?.parentElement; ancestor; ancestor = ancestor.parentElement) {
      if (ancestor instanceof HTMLDetailsElement) ancestor.open = true;
    }
  }, [recoveryAttention]);
  // Later pages cannot renew their first-page lease. Recheck from the root
  // after Core erases that boundary; human-authored drafts stay mounted.
  useEffect(() => {
    if (page.cursor && query.error?.name === "AbortError") setPage({});
  }, [page.cursor, query.error, setPage]);
  /**
   * The caller states ownership; a render-time intent closure is null while a submit is starting.
   * "intent": this panel's in-memory request. "list": a recovered identity whose candidate is in this panel's list.
   */
  function settled(receipt: CollaborationKnowledgeReceipt, owner: "intent" | "list" | null) {
    const success = ["knowledge_proposed", "knowledge_approved", "knowledge_rejected", "knowledge_withdrawn"].includes(receipt.code);
    setFeedback({ text: success ? receipt.action === "propose" ? t(($) => $.knowledge.proposalSaved) : t(($) => $.knowledge.decisionSaved) : t(($) => $.knowledge.refused), failed: !success });
    setIntent((currentIntent) => currentIntent?.requestId === receipt.request_id ? null : currentIntent);
    setSettledRequests((currentRequests) => [...new Set([...currentRequests, receipt.request_id])]);
    // Only the locked form that sent this proposal may close; a recovered or decision receipt never discards a draft.
    if (success && owner === "intent" && receipt.action === "propose") setProposal(null);
    if (success && owner !== null) setSelectedId(receipt.candidate_id);
    setOpenedElsewhere(success && owner === null);
    setPage({}); setRevision((value) => value + 1);
  }
  async function submit(next: KnowledgeIntent) {
    if (busy.current) return;
    busy.current = true; setIntent(next); setFeedback(null); setCleanupFailed(false); setOpenedElsewhere(false);
    const own = (receipt: CollaborationKnowledgeReceipt) => receipt.request_id === next.requestId ? "intent" as const : null;
    try { const receipt = await mutation.mutateAsync({ identity: next, intent: next }); settled(receipt, own(receipt)); }
    catch (error) {
      if (error instanceof KnowledgeReceiptCleanupError) { settled(error.receipt, own(error.receipt)); setCleanupFailed(true); }
      else if (error instanceof KnowledgeUnusedIntentError) {
        setIntent((currentIntent) => currentIntent?.requestId === next.requestId ? null : currentIntent);
        setSettledRequests((currentRequests) => [...new Set([...currentRequests, next.requestId])]);
        setCleanupFailed(error.cleanupFailed);
        setFeedback({ text: error.reason === "storage" ? t(($) => $.knowledge.storageError)
          : error.reason === "rejected" ? t(($) => $.knowledge.unusedRejected) : t(($) => $.knowledge.invalid), failed: true });
      } else setFeedback({ text: error instanceof KnowledgeRecoveryStorageError ? t(($) => $.knowledge.storageError) : t(($) => $.knowledge.pendingHint), failed: true });
    }
    finally { busy.current = false; setRevision((value) => value + 1); }
  }
  async function recover(identity: KnowledgeOperationIdentity) {
    if (busy.current) return; busy.current = true; setOpenedElsewhere(false);
    // Decided before the await: every panel lists every stored request, but only these are this panel's own.
    const owner = intent?.requestId === identity.requestId ? "intent" as const
      : current?.value.items.some((item) => item.candidate_id === identity.candidateId) === true ? "list" as const : null;
    // A refreshed or later list page cannot prove non-membership. Recheck the
    // authorized detail through Core, without retaining another server cache.
    const settleRecovered = async (receipt: CollaborationKnowledgeReceipt) => {
      let resolved = owner;
      if (resolved === null) {
        try {
          const { value } = await client.fetchQuery(collaborationKnowledgeCandidateOptions(client, scope, receipt.candidate_id));
          const candidate = value.candidate;
          if (candidate.availability !== "restricted" && (issue ? candidate.source_issue_id === issue.id : target && knowledgeTargetsEqual(candidate.target, target))) resolved = "list";
        } catch { /* A receipt remains known even if its current detail is unavailable. */ }
      }
      settled(receipt, resolved);
    };
    try { await settleRecovered(await mutation.mutateAsync({ identity })); }
    catch (error) {
      if (error instanceof KnowledgeReceiptCleanupError) { await settleRecovered(error.receipt); setCleanupFailed(true); }
      else {
        setFeedback({ text: t(($) => $.knowledge.notFound), failed: true });
        setUnconfirmed((currentRequests) => [...new Set([...currentRequests, identity.requestId])]);
      }
    }
    finally { busy.current = false; setRevision((value) => value + 1); }
  }
  /** Removes only the local pointer; no request, and never a claim that the server did not apply it. */
  function dismiss(identity: KnowledgeOperationIdentity) {
    if (busy.current || mutation.isPending || intent?.requestId === identity.requestId) return;
    try {
      dismissKnowledgeOperation(scope, identity);
      setUnconfirmed((currentRequests) => currentRequests.filter((requestId) => requestId !== identity.requestId));
      setFeedback(null);
    } catch { setFeedback({ text: t(($) => $.knowledge.dismissFailed), failed: true }); }
    finally { setRevision((value) => value + 1); }
  }
  // The panel's own in-memory intent is never dismissable; it still has Retry and Check.
  const dismissable = (identity: KnowledgeOperationIdentity) => unconfirmed.includes(identity.requestId) && intent?.requestId !== identity.requestId;
  return <section ref={root} id={issue ? "knowledge" : undefined} aria-labelledby={`${id}-title`} className="min-w-0 space-y-5 rounded-md border border-border p-4 sm:p-5 pointer-coarse:[&_button]:min-h-11">
    <div className="flex flex-wrap items-start justify-between gap-3"><div className="min-w-0 space-y-2"><h3 id={`${id}-title`} className="text-body font-semibold">{issue ? t(($) => $.knowledge.title) : t(($) => $.knowledge.managed)}</h3>
      <p className="max-w-prose text-caption text-muted-foreground">{issue ? t(($) => $.knowledge.intro) : t(($) => $.knowledge.managedIntro)}</p></div>
      {issue && sourceView?.capabilities.can_propose === true && !proposal && <Button type="button" variant="outline" disabled={locked} onClick={() => { setProposal({}); setFeedback(null); }}>{t(($) => $.knowledge.propose)}</Button>}
    </div>
    {feedback && <p role={feedback.failed ? "alert" : "status"} className={feedback.failed ? "text-body text-destructive" : "text-body"}>{feedback.text}</p>}
    {openedElsewhere && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.knowledge.openedElsewhere)}</p>}
    {cleanupFailed && <p role="alert" className="text-caption text-muted-foreground">{t(($) => $.knowledge.recoveryCleanupFailed)}</p>}
    {storageError && <p role="alert" className="text-body text-destructive">{t(($) => $.knowledge.recoveryReadError)}</p>}
    {pending.length > 0 && <div className="space-y-3"><div className="space-y-1"><h4 className="text-body font-medium">{t(($) => $.knowledge.pendingTitle)}</h4>
      <p className="max-w-prose text-caption text-muted-foreground">{t(($) => $.knowledge.pendingScope)}</p></div>
      {pending.map((identity) => <div key={identity.requestId} className="flex min-w-0 flex-wrap items-center gap-2"><span className="text-caption text-muted-foreground [overflow-wrap:anywhere]">{t(($) => $.knowledge.request, { id: identity.requestId })}</span>
        <Button type="button" variant="outline" size="sm" disabled={mutation.isPending} onClick={() => void recover(identity)}>{t(($) => $.knowledge.recover)}</Button>
        {intent?.requestId === identity.requestId && <Button type="button" variant="outline" size="sm" disabled={mutation.isPending} onClick={() => void submit(intent)}>{t(($) => $.knowledge.retry)}</Button>}
        {dismissable(identity) && <Button type="button" variant="outline" size="sm" disabled={mutation.isPending} aria-describedby={`${id}-dismiss-hint`} onClick={() => dismiss(identity)}>{t(($) => $.knowledge.dismiss)}</Button>}
      </div>)}
      {pending.some(dismissable) && <p id={`${id}-dismiss-hint`} className="max-w-prose text-caption text-muted-foreground">{t(($) => $.knowledge.dismissHint)}</p>}
    </div>}
    {!current && <div className="space-y-3"><p role={query.error ? "alert" : "status"} className="text-body text-muted-foreground">{query.error instanceof CollaborationUnavailableError ? t(($) => $.knowledge.unavailable) : query.error ? t(($) => $.knowledge.loadError) : t(($) => $.knowledge.loading)}</p>
      {query.error && <Button type="button" variant="outline" onClick={() => { setPage({}); void query.refetch(); }}>{t(($) => $.knowledge.refresh)}</Button>}</div>}
    {current && <>
      {current.value.items.length === 0 && <p className="text-body text-muted-foreground">{issue ? t(($) => $.knowledge.empty) : t(($) => $.knowledge.targetEmpty)}</p>}
      <ul className="space-y-2">{current.value.items.map((candidate) => <li key={candidate.candidate_id}>
        <button type="button" aria-pressed={selectedId === candidate.candidate_id} onClick={() => setSelectedId(candidate.candidate_id)}
          className={`flex w-full min-w-0 flex-wrap items-start justify-between gap-2 rounded-md px-2 py-2 text-left text-body hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring pointer-coarse:min-h-11 ${selectedId === candidate.candidate_id ? "bg-accent font-medium" : ""}`}>
          <span className="min-w-0 flex-1 [overflow-wrap:anywhere]">{candidate.availability === "available" ? candidate.summary : candidate.availability === "restricted" ? t(($) => $.knowledge.restricted) : t(($) => $.knowledge.sourceUnavailable)}
            {candidate.availability !== "restricted" && <span className="mt-1 block text-caption text-muted-foreground">{t(($) => $.knowledge.version, { version: candidate.version })}</span>}</span><KnowledgeState candidate={candidate} />
        </button></li>)}</ul>
      <div className="flex flex-wrap gap-2">{page.cursor && <Button type="button" variant="outline" size="sm" onClick={() => setPage({})}>{t(($) => $.knowledge.latest)}</Button>}
        {current.value.next_cursor && <Button type="button" variant="outline" size="sm" disabled={query.isFetching} onClick={() => setPage({ cursor: current.value.next_cursor!, boundary: current.boundary })}>{t(($) => $.knowledge.more)}</Button>}</div>
    </>}
    {selectedId && <KnowledgeReview scope={scope} candidateId={selectedId} issue={issue} canPropose={sourceView?.capabilities.can_propose === true} locked={locked}
      onSubmit={submit} onProposal={(mode) => { setProposal(mode); setFeedback(null); }} onSelect={setSelectedId} onClose={() => setSelectedId(null)} />}
    {proposal && issue && refreshView && <KnowledgeProposalForm key={`${proposal.supersedesId ?? ""}:${proposal.derivedId ?? ""}`} scope={scope} issue={issue} view={sourceView}
      refreshView={refreshView} mode={proposal} locked={locked} onSubmit={submit} onCancel={() => setProposal(null)} />}
  </section>;
}
