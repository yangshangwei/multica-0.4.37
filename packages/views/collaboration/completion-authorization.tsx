import { useEffect, useId, useLayoutEffect, useRef, useState, useSyncExternalStore } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { getApi } from "@multica/core/api";
import { authorizeCollaborationCompletion, collaborationCompletionCandidatesOptions, collaborationCompletionPreviewOptions,
  type CollaborationCompletionCandidates, type CollaborationCompletionScope } from "@multica/core/collaboration";
import { assertCompletionScope, completionDraftRevision, completionReason, completionUUID, readCompletionDrafts, recoverIssueCompletion, retireCompletionDraft,
  revokeIssueCompletion, subscribeCompletionDrafts, type CollaborationCompletionTarget, type CompletionDraft, type CompletionResult } from "@multica/core/issues";
import type { DeliveryCompletionBinding } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useT } from "../i18n";
import { StageBlockers } from "./stage-blockers";
import { useStageClock } from "./stage-code-panel";

export interface CompletionAuthorizationSelection { active: boolean; binding: DeliveryCompletionBinding | null }
export function CollaborationCompletionAuthorization({ scope, sourceChildren, makeTarget, targetStatus, disabled, onChange, onBusyChange }: {
  targetStatus: string;
  scope: CollaborationCompletionScope; sourceChildren?: { id: string; title: string; can_open: boolean }[];
  makeTarget(revision: number, context?: CollaborationCompletionCandidates["target_context"]): CollaborationCompletionTarget | null;
  disabled: boolean; onChange(value: CompletionAuthorizationSelection): void; onBusyChange?(busy: boolean): void;
}) {
  const { t } = useT("collaboration");
  const id = useId(), client = useQueryClient();
  const now = useStageClock();
  const [active, setActive] = useState(false), [sourceId, setSource] = useState(""), [approvalId, setApproval] = useState("");
  const [cursors, setCursors] = useState<(string | undefined)[]>([undefined]);
  const [reason, setReason] = useState(""), [expires, setExpires] = useState(""), [revocationReason, setRevocationReason] = useState("");
  const [operationId, setOperationId] = useState(() => crypto.randomUUID());
  const [mode, setMode] = useState<"review" | "existing">("review"), [authorizationInput, setAuthorizationInput] = useState(""), [lookupId, setLookup] = useState("");
  useSyncExternalStore(subscribeCompletionDrafts, completionDraftRevision, completionDraftRevision);
  let draft: CompletionDraft | undefined, storageFailed = false;
  try { draft = readCompletionDrafts(scope)[0]; } catch { storageFailed = true; }
  const effectiveActive = active || draft !== undefined;
  const restored = useRef("");
  useEffect(() => {
    if (!draft || restored.current === draft.requestKey) return;
    restored.current = draft.requestKey;
    setActive(true); setMode("review"); setSource(draft.request.evidence.issue_id); setApproval(draft.request.evidence.approval_id);
    setReason(draft.request.reason); setExpires(draft.request.expires_at);
  }, [draft]);
  const candidates = useQuery({ ...collaborationCompletionCandidatesOptions(client, scope, sourceId, cursors.at(-1)), enabled: active && mode === "review" && completionUUID.safeParse(sourceId).success });
  const page = !candidates.error && candidates.data?.state === "available" ? candidates.data.value : null;
  const target = page ? makeTarget(page.issue_revision, page.target_context) : null;
  const preview = useQuery(collaborationCompletionPreviewOptions(client, scope, active && mode === "review" && target && page?.items.some((item) => item.approval_id === approvalId && item.selectable) ? { source_issue_id: sourceId, approval_id: approvalId, target } : null));
  const current = !preview.error && preview.data?.state === "available" ? preview.data.value : null;
  const mutation = useMutation({ mutationKey: ["delivery-completion", scope.server, scope.userId, scope.issueId, "authorization"], retry: false,
    mutationFn: async (action: "authorize" | "recover" | "revoke") => {
      if (action === "authorize" && current) return authorizeCollaborationCompletion(scope, current, { reason, expiresAt: expires, operationId });
      if (!draft) throw new Error("Original completion authorization unavailable");
      return action === "revoke" ? revokeIssueCompletion(scope, draft, revocationReason) : recoverIssueCompletion(scope, draft);
    } });
  const result: CompletionResult | undefined = mutation.data;
  const imported = useQuery({ queryKey: ["delivery-completion-reference", scope.server, scope.apiScope, scope.userId, scope.workspaceId, scope.issueId, lookupId],
    enabled: active && mode === "existing" && !draft && completionUUID.safeParse(lookupId).success, retry: false, staleTime: 0, gcTime: 0,
    queryFn: async ({ signal }) => { assertCompletionScope(scope, signal); const value = await getApi().getDeliveryCompletionAuthorization(lookupId, { workspaceId: scope.workspaceId, signal }); assertCompletionScope(scope, signal); return value; } });
  const grant = mode === "existing" && !imported.error ? imported.data : result?.authorization;
  const currentTarget = target ? JSON.stringify(target) : null;
  const originalTarget = draft ? JSON.stringify(draft.request.target) : null;
  const exact = mode === "existing" ? targetStatus !== "" && authorizationInput === lookupId && grant?.authorization_id === lookupId : currentTarget !== null && currentTarget === originalTarget && current?.authorization_binding !== null && current?.authorization_binding !== undefined && !preview.isFetching;
  const usable = effectiveActive && !disabled && !storageFailed && !mutation.isPending && !imported.isFetching && exact && grant?.state === "issued" && grant.applicability === "not_checked" &&
    grant.issue_id === scope.issueId && grant.target_status === targetStatus && Date.parse(grant.expires_at) > now &&
    (mode === "existing" || result?.outcome === "authorized" && !draft?.submissionDispatched && !draft?.revocation);
  const authorizationId = usable ? grant.authorization_id : null, originalOperationId = usable ? grant.operation_id : null, expectedRevision = usable ? grant.issue_revision : null;
  useLayoutEffect(() => { onChange({ active: effectiveActive, binding: authorizationId && originalOperationId && expectedRevision !== null ? {
    authorization_id: authorizationId, operation_id: originalOperationId, expected_issue_revision: expectedRevision } : null }); }, [effectiveActive, authorizationId, originalOperationId, expectedRevision, onChange]);
  useEffect(() => { onBusyChange?.(mutation.isPending); return () => onBusyChange?.(false); }, [mutation.isPending, onBusyChange]);
  const canAuthorize = !disabled && !draft && !storageFailed && !mutation.isPending && !preview.isFetching && current?.authorization_binding !== null && current?.authorization_binding !== undefined &&
    current.max_expires_at !== null && completionReason.safeParse(reason).success && Number.isFinite(Date.parse(expires)) && Date.parse(expires) > now && Date.parse(expires) <= Date.parse(current.max_expires_at);
  const canRetire = result && (result.outcome === "consumed" || result.outcome === "revoked" || result.outcome === "refused" && result.draft.authorizationId === null);
  return <section aria-labelledby={`${id}-title`} className="min-w-0 space-y-3 rounded-md border border-input p-3">
    <p id={`${id}-title`} className="text-body font-medium">{t(($) => $.completion.title)}</p>
    <p className="text-caption text-muted-foreground">{t(($) => $.completion.hint)}</p>
    <label className="flex items-center gap-2 text-body"><input type="checkbox" checked={effectiveActive} disabled={disabled || mutation.isPending || !!draft} onChange={(event) => setActive(event.target.checked)} />{t(($) => $.completion.enable)}</label>
    {(active || draft) && <div className="min-w-0 space-y-3">
      <label htmlFor={`${id}-mode`} className="block text-caption">{t(($) => $.completion.method)}</label>
      <select id={`${id}-mode`} value={mode} disabled={disabled || !!draft || mutation.isPending} onChange={(event) => setMode(event.target.value === "existing" ? "existing" : "review")} className="min-h-9 w-full rounded-md border border-input bg-background px-3 text-body">
        <option value="review">{t(($) => $.completion.review)}</option><option value="existing">{t(($) => $.completion.existing)}</option>
      </select>
      {mode === "existing" && !draft ? <>
        <label htmlFor={`${id}-authorization`} className="block text-caption">{t(($) => $.completion.authorizationId)}</label>
        <Input id={`${id}-authorization`} value={authorizationInput} disabled={disabled} onChange={(event) => setAuthorizationInput(event.target.value)} />
        <Button type="button" variant="outline" disabled={disabled || !completionUUID.safeParse(authorizationInput).success || imported.isFetching} onClick={() => { setLookup(authorizationInput); if (lookupId === authorizationInput) void imported.refetch(); }}>{t(($) => $.completion.lookup)}</Button>
        <p className="text-caption text-muted-foreground">{t(($) => $.completion.referenceHint)}</p>
      </> : <>
        <label htmlFor={`${id}-source`} className="block text-caption">{t(($) => $.completion.source)}</label>
        {sourceChildren ? <select id={`${id}-source`} value={sourceId} disabled={disabled || !!draft} onChange={(event) => { setSource(event.target.value); setApproval(""); setCursors([undefined]); }} className="min-h-9 w-full rounded-md border border-input bg-background px-3 text-body">
          <option value="">{t(($) => $.completion.chooseSource)}</option>{sourceChildren.filter((child) => child.can_open).map((child) => <option key={child.id} value={child.id}>{child.title} · {child.id}</option>)}</select>
          : <Input id={`${id}-source`} value={sourceId} disabled={disabled || !!draft} onChange={(event) => { setSource(event.target.value); setApproval(""); setCursors([undefined]); }} />}
        {page && <>
          <label htmlFor={`${id}-artifact`} className="block text-caption">{t(($) => $.completion.artifact)}</label>
          <select id={`${id}-artifact`} value={approvalId} disabled={disabled || !!draft} onChange={(event) => setApproval(event.target.value)} className="min-h-9 w-full rounded-md border border-input bg-background px-3 text-body">
            <option value="">{t(($) => $.completion.chooseArtifact)}</option>{page.items.map((item) => <option key={item.approval_id} value={item.approval_id} disabled={!item.selectable || Date.parse(item.expires_at) <= now}>{item.summary || item.approval_id} · {item.approval_id}</option>)}</select>
          {page.items.length === 0 && !page.has_more && <p className="text-caption">{t(($) => $.completion.empty)}</p>}
          <div className="flex gap-2"><Button type="button" variant="outline" disabled={disabled || !!draft || cursors.length < 2 || candidates.isFetching} onClick={() => { setCursors((all) => all.slice(0, -1)); setApproval(""); }}>{t(($) => $.completion.previous)}</Button>
            <Button type="button" variant="outline" disabled={disabled || !!draft || !page.next_cursor || candidates.isFetching} onClick={() => { if (page.next_cursor && !cursors.includes(page.next_cursor)) { setCursors((all) => [...all, page.next_cursor!]); setApproval(""); } }}>{t(($) => $.completion.next)}</Button></div>
        </>}
        {current && <><p className="text-caption">{t(($) => $.completion.observation, { verdict: current.observation?.verdict ?? current.reason })}</p>
          {current.stage_blockers.length > 0 && <StageBlockers blockers={current.stage_blockers} truncated={current.stage_blockers_truncated} />}
          {current.observation && <dl className="space-y-1 text-caption [overflow-wrap:anywhere]"><dt>{t(($) => $.completion.artifactDigest)}</dt><dd>{current.observation.artifact_digest}</dd><dt>{t(($) => $.completion.sourceRevision)}</dt><dd>{current.observation.source_revision}</dd></dl>}
          {current.max_expires_at && <p className="text-caption">{t(($) => $.completion.horizon, { time: current.max_expires_at })}</p>}</>}
        {!draft && <><label htmlFor={`${id}-reason`} className="block text-caption">{t(($) => $.completion.reason)}</label><Textarea id={`${id}-reason`} value={reason} disabled={disabled || mutation.isPending} onChange={(event) => setReason(event.target.value)} />
          <label htmlFor={`${id}-expires`} className="block text-caption">{t(($) => $.completion.expires)}</label><Input id={`${id}-expires`} value={expires} placeholder={t(($) => $.completion.expiryExample)} disabled={disabled || mutation.isPending} onChange={(event) => setExpires(event.target.value)} />
          {current?.max_expires_at && <Button type="button" variant="outline" disabled={disabled || mutation.isPending} onClick={() => setExpires(current.max_expires_at!)}>{t(($) => $.completion.useExpiry)}</Button>}
          <Button type="button" disabled={!canAuthorize} onClick={() => mutation.mutate("authorize")}>{t(($) => $.completion.authorize)}</Button></>}
      </>}
      {storageFailed && <p role="alert">{t(($) => $.workflow.storageError)}</p>}
      {(candidates.error || preview.error || imported.error || mutation.error || candidates.data?.state === "denied" || candidates.data?.state === "unavailable" || preview.data?.state === "denied" || preview.data?.state === "unavailable") && <p role="status" className="text-caption">{t(($) => $.completion.unavailable)}</p>}
      {result && <p role="status" className="text-caption">{result.outcome === "authorized" ? t(($) => $.completion.authorized) : result.outcome === "consumed" ? t(($) => $.completion.consumed) : result.outcome === "revoked" ? t(($) => $.completion.revoked) : t(($) => $.completion.unknown)}</p>}
      {usable && <p role="status" className="text-caption">{t(($) => $.completion.ready)}</p>}
      {draft && <>
        <p className="text-caption [overflow-wrap:anywhere]">{t(($) => $.completion.original, { id: draft.request.operation_id })}</p>
        {currentTarget !== null && currentTarget !== originalTarget && <p role="status" className="text-caption">{t(($) => $.completion.changed)}</p>}
        <Button type="button" variant="outline" disabled={mutation.isPending} onClick={() => mutation.mutate("recover")}>{t(($) => $.completion.recover)}</Button>
        {draft.authorizationId && !draft.revocation && result?.outcome !== "consumed" && <><label htmlFor={`${id}-revoke`} className="block text-caption">{t(($) => $.completion.revocationReason)}</label><Input id={`${id}-revoke`} value={revocationReason} onChange={(event) => setRevocationReason(event.target.value)} />
          <Button type="button" variant="outline" disabled={mutation.isPending || !completionReason.safeParse(revocationReason).success} onClick={() => mutation.mutate("revoke")}>{t(($) => $.completion.revoke)}</Button></>}
        {canRetire && <Button type="button" variant="outline" disabled={mutation.isPending} onClick={() => { if (result) retireCompletionDraft(scope, result); mutation.reset(); setOperationId(crypto.randomUUID()); setActive(false); }}>{t(($) => $.completion.newReview)}</Button>}
      </>}
    </div>}
  </section>;
}
