"use client";

import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useAuthStore } from "@multica/core/auth";
import { ApiError } from "@multica/core/api";
import {
  authorizeIssueCompletion, completionCandidatesOptions, completionDraftRevision, completionDraftScope,
  completionPreviewOptions, isCompletionScopeCurrent, issueStatusCategory, readCompletionDrafts,
  reconcileCompletionResult, recoverIssueCompletion, retireCompletionDraft, revokeIssueCompletion,
  submitIssueCompletion, subscribeCompletionDrafts,
  type CompletionDraft, type CompletionPreview, type CompletionScope,
} from "@multica/core/issues";
import type { Issue } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@multica/ui/components/ui/dialog";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { Spinner } from "@multica/ui/components/ui/spinner";
import { useStatusLabel } from "../issues/utils/status-label";
import { useT } from "../i18n";

export interface IssueCompletionAuthorizationModalData {
  issue: Issue;
  scope: CompletionScope;
  targetStatus: string;
  ownerSignal: AbortSignal;
  onCompleted?: (issue: Issue) => void;
  onRefresh?: () => void | Promise<void>;
  onCancel?: () => void;
}

type CompletionAction =
  | { kind: "authorize"; preview: CompletionPreview; reason: string; expiresAt: string }
  | { kind: "submit" | "recover"; draft: CompletionDraft }
  | { kind: "revoke"; draft: CompletionDraft; reason: string };

export function IssueCompletionAuthorizationModal({ onClose, data }: {
  onClose: () => void; data: Record<string, unknown> | null;
}) {
  const d = (data ?? {}) as Partial<IssueCompletionAuthorizationModalData>;
  const { t } = useT("issues");
  if (!d.issue || !d.scope || !d.targetStatus || !d.ownerSignal) {
    return <Dialog open onOpenChange={(open) => { if (!open) onClose(); }}><DialogContent>
      <DialogHeader><DialogTitle>{t($ => $.detail.completion_authorization.title)}</DialogTitle>
        <DialogDescription>{t($ => $.detail.completion_authorization.unavailable)}</DialogDescription></DialogHeader>
      <DialogFooter><Button onClick={onClose}>{t($ => $.detail.completion_authorization.close)}</Button></DialogFooter>
    </DialogContent></Dialog>;
  }
  return <CompletionAuthorization key={`${d.scope.apiScope}:${d.issue.id}:${d.issue.revision}:${d.targetStatus}`}
    data={{ ...d, issue: d.issue, scope: d.scope, targetStatus: d.targetStatus, ownerSignal: d.ownerSignal }} onClose={onClose} />;
}

function CompletionAuthorization({ data: d, onClose }: { data: IssueCompletionAuthorizationModalData; onClose: () => void }) {
  const { t } = useT("issues");
  const statusLabel = useStatusLabel(d.scope.workspaceId);
  const userId = useAuthStore(state => state.user?.id);
  const client = useQueryClient();
  const [controller] = useState(() => new AbortController());
  const [cursorHistory, setCursorHistory] = useState<Array<string | undefined>>([undefined]);
  const [approvalId, setApprovalId] = useState("");
  const [reason, setReason] = useState("");
  const [expiryInput, setExpiryInput] = useState("");
  const [revocationReason, setRevocationReason] = useState("");
  const [selectedRequestKey, setSelectedRequestKey] = useState<string | null>(null);
  const [localError, setLocalError] = useState<string | null>(null);
  const completed = useRef(false);
  const closed = useRef(false);
  const inFlight = useRef(false);
  useSyncExternalStore(subscribeCompletionDrafts, completionDraftRevision, completionDraftRevision);
  const retained = (() => {
    try { return { drafts: readCompletionDrafts(completionDraftScope(d.scope)), failed: false }; }
    catch { return { drafts: [], failed: true }; }
  })();
  const current = userId === d.scope.userId && !controller.signal.aborted && !d.ownerSignal.aborted && isCompletionScopeCurrent(d.scope);

  const mutation = useMutation({
    mutationFn: async (action: CompletionAction) => {
      if (!current || controller.signal.aborted || !isCompletionScopeCurrent(d.scope)) throw new Error(t($ => $.detail.completion_authorization.session_changed));
      const options = { signal: controller.signal };
      switch (action.kind) {
        case "authorize": return authorizeIssueCompletion(d.scope, action.preview, { reason: action.reason, expiresAt: action.expiresAt }, options);
        case "submit": return submitIssueCompletion(d.scope, action.draft, options);
        case "recover": return recoverIssueCompletion(d.scope, action.draft, options);
        case "revoke": return revokeIssueCompletion(d.scope, action.draft, action.reason, options);
      }
    },
    onSuccess: (result, action) => {
      if (controller.signal.aborted || d.ownerSignal.aborted || !isCompletionScopeCurrent(d.scope)) return;
      reconcileCompletionResult(client, d.scope, result);
      setSelectedRequestKey(result.draft.requestKey);
      if (action.kind === "submit" && result.submittedNow && result.outcome === "consumed" &&
        result.currentIssueRead === "ok" && result.issue && issueStatusCategory(result.issue) === "done" &&
        result.issue.status === result.draft.request.target.target_status && !completed.current) {
        completed.current = true;
        d.onCompleted?.(result.issue);
      }
    },
  });
  const result = mutation.data;
  const draft = result?.draft ?? retained.drafts.find(item => item.requestKey === selectedRequestKey) ?? retained.drafts[0];
  const candidates = useQuery({ ...completionCandidatesOptions(d.scope, { limit: 20, cursor: cursorHistory.at(-1) }, client),
    enabled: current && !draft && !retained.failed, retry: false });
  const previewQuery = useQuery({ ...completionPreviewOptions(d.scope, {
    expected_issue_revision: d.issue.revision ?? -1, target_status: d.targetStatus, approval_id: approvalId,
  }, client), enabled: current && !draft && !!approvalId && !retained.failed, retry: false });
  const preview = previewQuery.isError ? undefined : previewQuery.data;
  const candidatePage = candidates.isError ? undefined : candidates.data;
  const defaultExpiry = preview?.max_expires_at ? new Date(Math.min(Date.parse(preview.max_expires_at), Date.parse(preview.checked_at) + 15 * 60_000)).toISOString() : "";
  const expiresAt = expiryInput || defaultExpiry;
  const reasonValid = reason.trim().length > 0 && [...reason.trim()].length <= 1000;
  const expiryValid = !!preview?.max_expires_at && expiresAt.endsWith("Z") && Number.isFinite(Date.parse(expiresAt)) &&
    Date.parse(expiresAt) > Date.now() && Date.parse(expiresAt) <= Date.parse(preview.max_expires_at);
  const canAuthorize = current && !retained.failed && !draft && !previewQuery.isFetching && preview?.authorization_binding != null &&
    preview.observation?.verdict === "pass" && preview.target_status === d.targetStatus && reasonValid && expiryValid;
  const canSubmit = current && result?.outcome === "authorized" && draft && !draft.submissionDispatched && !draft.revocation?.dispatched;
  const canRevoke = current && !!draft?.authorizationId && !draft.revocation?.dispatched &&
    result?.outcome !== "consumed" && result?.outcome !== "revoked";
  const canRetire = result && (result.outcome === "consumed" || result.outcome === "revoked" || (result.outcome === "refused" && !draft?.authorizationId));
  const pending = mutation.isPending;
  const evidenceError = candidates.error ?? previewQuery.error;
  const evidenceErrorMessage = evidenceError instanceof ApiError && [401, 403, 404].includes(evidenceError.status)
    ? t($ => $.detail.completion_authorization.access_denied)
    : evidenceError ? t($ => $.detail.completion_authorization.read_failed) : null;

  useEffect(() => {
    const abort = () => { controller.abort(); if (!closed.current) { closed.current = true; onClose(); } };
    if (d.ownerSignal.aborted) abort();
    else d.ownerSignal.addEventListener("abort", abort, { once: true });
    return () => { d.ownerSignal.removeEventListener("abort", abort); controller.abort(); };
  }, [controller, d.ownerSignal, onClose]);

  const close = () => {
    if (closed.current) return;
    closed.current = true;
    controller.abort(); d.onCancel?.(); onClose();
  };
  const dispatch = async (action: CompletionAction) => {
    if (inFlight.current || !current) return;
    inFlight.current = true;
    setLocalError(null);
    try { await mutation.mutateAsync(action); }
    catch { /* The mutation owns the readable error; a sent intent remains retained. */ }
    finally { inFlight.current = false; }
  };
  const refresh = async () => {
    if (pending || !current) return;
    try {
      if (canRetire && result) retireCompletionDraft(d.scope, result);
      onClose();
      await d.onRefresh?.();
    } catch (error) { setLocalError(error instanceof Error ? error.message : t($ => $.detail.completion_authorization.unavailable)); }
  };

  if (!current) return <Dialog open onOpenChange={(open) => { if (!open) close(); }}><DialogContent>
    <DialogHeader><DialogTitle>{t($ => $.detail.completion_authorization.title)}</DialogTitle>
      <DialogDescription>{t($ => $.detail.completion_authorization.session_changed)}</DialogDescription></DialogHeader>
    <DialogFooter><Button onClick={close}>{t($ => $.detail.completion_authorization.close)}</Button></DialogFooter>
  </DialogContent></Dialog>;

  const failure = retained.failed ? t($ => $.detail.completion_authorization.storage_failed) : localError ??
    (mutation.error instanceof Error ? mutation.error.message : null) ??
    evidenceErrorMessage;
  return <Dialog open onOpenChange={(open) => { if (!open) close(); }}>
    <DialogContent className="flex max-h-[85dvh] flex-col sm:max-w-2xl">
      <DialogHeader>
        <DialogTitle>{t($ => $.detail.completion_authorization.title)}</DialogTitle>
        <DialogDescription>{t($ => $.detail.completion_authorization.description)}</DialogDescription>
      </DialogHeader>
      <div className="min-h-0 space-y-4 overflow-y-auto text-body">
        <p className="font-medium [overflow-wrap:anywhere]">{d.issue.identifier} · {d.issue.title}</p>
        <p className="text-muted-foreground">{statusLabel(draft?.request.target.expected_status ?? d.issue.status)} → {statusLabel(draft?.request.target.target_status ?? d.targetStatus)} · {t($ => $.detail.completion_authorization.revision)} {draft?.request.target.expected_issue_revision ?? d.issue.revision}</p>
        {failure ? <p role="alert" className="text-destructive [overflow-wrap:anywhere]">{failure}</p> : null}
        {!draft ? <>
          <fieldset disabled={pending} className="space-y-2">
            <legend className="mb-2 font-medium">{t($ => $.detail.completion_authorization.evidence)}</legend>
            {candidates.isLoading ? <p role="status"><Spinner className="mr-2 inline size-4" />{t($ => $.detail.completion_authorization.loading)}</p> : null}
            {candidatePage?.items.length === 0 ? <p>{t($ => $.detail.completion_authorization.no_evidence)}</p> : null}
            {candidatePage?.items.map(item => <label key={item.approval_id} className="flex cursor-pointer items-start gap-3 rounded-md border p-3 has-checked:border-ring">
              <input type="radio" name="completion-evidence" value={item.approval_id} checked={approvalId === item.approval_id} disabled={!item.selectable}
                onChange={() => { setApprovalId(item.approval_id); setExpiryInput(""); setReason(""); }} className="mt-1" />
              <span className="min-w-0 space-y-1 [overflow-wrap:anywhere]"><span className="block font-medium">{item.summary || item.approval_id}</span>
                <span className="block text-caption text-muted-foreground">{item.source_revision}</span>
                <span className="block text-caption text-muted-foreground">{item.status}{item.result_code ? ` · ${item.result_code}` : ""}</span>
                <span className="block text-caption text-muted-foreground">{item.selectable ? item.expires_at : Date.parse(item.expires_at) <= Date.now()
                  ? item.expires_at
                  : t($ => $.detail.completion_authorization.evidence_unavailable)}</span>
                {!item.selectable && Date.parse(item.expires_at) <= Date.now()
                  ? <span className="block text-caption text-muted-foreground">{t($ => $.detail.completion_authorization.expired)}</span>
                  : null}
              </span>
            </label>)}
          </fieldset>
          {(cursorHistory.length > 1 || candidatePage?.has_more) ? <div className="flex gap-2">
            <Button variant="outline" disabled={pending || cursorHistory.length === 1 || candidates.isFetching} onClick={() => { setCursorHistory(value => value.slice(0, -1)); setApprovalId(""); }}>{t($ => $.detail.completion_authorization.previous)}</Button>
            <Button variant="outline" disabled={pending || !candidatePage?.next_cursor || candidates.isFetching} onClick={() => { const next = candidatePage?.next_cursor; if (next) { setCursorHistory(value => [...value, next]); setApprovalId(""); } }}>{t($ => $.detail.completion_authorization.next)}</Button>
          </div> : null}
          {previewQuery.isFetching ? <p role="status">{t($ => $.detail.completion_authorization.checking)}</p> : null}
          {preview ? <>
            <p>{t($ => $.detail.completion_authorization.observation)}: {preview.observation?.verdict ?? preview.reason}</p>
            <p className="text-caption text-muted-foreground">{t($ => $.detail.completion_authorization.observation_limit)}</p>
            <details className="rounded-md border p-3"><summary className="cursor-pointer">{t($ => $.detail.completion_authorization.evidence_details)}</summary>
              <dl className="mt-3 grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-2 text-caption [overflow-wrap:anywhere]">
                <dt>{t($ => $.detail.completion_authorization.project_revision)}</dt><dd>{preview.project_id} / {preview.project_revision} / {preview.policy_revision}</dd>
                <dt>{t($ => $.detail.completion_authorization.source)}</dt><dd>{preview.observation?.source_revision ?? "—"}</dd>
                <dt>{t($ => $.detail.completion_authorization.artifact)}</dt><dd>{preview.observation?.artifact_digest ?? "—"}</dd>
                <dt>{t($ => $.detail.completion_authorization.spec)}</dt><dd>{preview.observation?.spec_digest ?? "—"}</dd>
                <dt>{t($ => $.detail.completion_authorization.checked_at)}</dt><dd>{preview.checked_at}</dd>
                <dt>{t($ => $.detail.completion_authorization.evidence_expiry)}</dt><dd>{preview.observation?.valid_until ?? "—"}</dd>
                <dt>{t($ => $.detail.completion_authorization.authorization_limit)}</dt><dd>{preview.max_expires_at ?? "—"}</dd>
              </dl>
            </details>
            {preview.authorization_binding ? <div className="space-y-3">
              <div className="space-y-1"><Label htmlFor="completion-reason">{t($ => $.detail.completion_authorization.reason)}</Label>
                <Textarea id="completion-reason" value={reason} disabled={pending} onChange={event => setReason(event.target.value)} /></div>
              <div className="space-y-1"><Label htmlFor="completion-expiry">{t($ => $.detail.completion_authorization.expires_at)}</Label>
                <Input id="completion-expiry" value={expiresAt} disabled={pending} onChange={event => setExpiryInput(event.target.value)} aria-describedby="completion-expiry-format" />
                <p id="completion-expiry-format" className="text-caption text-muted-foreground">{t($ => $.detail.completion_authorization.expiry_format)}</p></div>
            </div> : <p>{t($ => $.detail.completion_authorization.cannot_authorize)}</p>}
          </> : null}
        </> : <>
          {retained.drafts.length > 1 ? <div className="space-y-1"><Label htmlFor="completion-original">{t($ => $.detail.completion_authorization.original_operation)}</Label>
            <select id="completion-original" value={draft.requestKey} disabled={pending} className="w-full rounded-md border bg-background p-2" onChange={event => { setSelectedRequestKey(event.target.value); mutation.reset(); }}>
              {retained.drafts.map(item => <option key={item.requestKey} value={item.requestKey}>{item.request.operation_id}</option>)}
            </select></div> : null}
          <p role="status">{result?.outcome === "consumed" ? t($ => $.detail.completion_authorization.consumed)
            : result?.outcome === "revoked" ? t($ => $.detail.completion_authorization.revoked)
            : result?.outcome === "authorized" ? t($ => $.detail.completion_authorization.authorized)
            : result?.outcome === "refused" ? t($ => $.detail.completion_authorization.refused)
            : t($ => $.detail.completion_authorization.unknown)}</p>
          {result?.issue ? <p>{t($ => $.detail.completion_authorization.current_status)}: <span>{statusLabel(result.issue.status)}</span> · {t($ => $.detail.completion_authorization.revision)} {result.issue.revision}</p> : null}
          {result && (result.currentIssueRead === "denied" || result.currentIssueRead === "unknown") ? <p>{t($ => $.detail.completion_authorization.current_unavailable)}</p> : null}
          <details className="rounded-md border p-3"><summary className="cursor-pointer">{t($ => $.detail.completion_authorization.recovery_details)}</summary>
            <dl className="mt-3 grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-2 text-caption [overflow-wrap:anywhere]">
              <dt>{t($ => $.detail.completion_authorization.original_operation)}</dt><dd>{draft.request.operation_id}</dd>
              <dt>{t($ => $.detail.completion_authorization.request_key)}</dt><dd>{draft.requestKey}</dd>
              <dt>{t($ => $.detail.completion_authorization.workspace)}</dt><dd>{d.scope.workspaceId}</dd>
              <dt>{t($ => $.detail.completion_authorization.authorization)}</dt><dd>{draft.authorizationId ?? "—"}</dd>
              <dt>{t($ => $.detail.completion_authorization.expires_at)}</dt><dd>{draft.request.expires_at}</dd>
              <dt>{t($ => $.detail.completion_authorization.reason)}</dt><dd>{draft.request.reason}</dd>
              {result?.reason ? <><dt>{t($ => $.detail.completion_authorization.result_reason)}</dt><dd>{result.reason}</dd></> : null}
            </dl>
          </details>
          {canRevoke ? <div className="space-y-1"><Label htmlFor="completion-revoke-reason">{t($ => $.detail.completion_authorization.revocation_reason)}</Label>
            <Textarea id="completion-revoke-reason" value={revocationReason} disabled={pending} onChange={event => setRevocationReason(event.target.value)} /></div> : null}
        </>}
      </div>
      <DialogFooter className="flex-wrap gap-2">
        <Button type="button" variant="outline" onClick={close}>{t($ => $.detail.completion_authorization.close)}</Button>
        {!draft && d.onRefresh ? <Button type="button" variant="outline" disabled={pending} onClick={() => void refresh()}>{t($ => $.detail.completion_authorization.refresh)}</Button> : null}
        {draft ? <Button type="button" variant="outline" disabled={pending || retained.failed} onClick={() => void dispatch({ kind: "recover", draft })}>{t($ => $.detail.completion_authorization.query)}</Button> : null}
        {canRevoke && draft ? <Button type="button" variant="outline" disabled={pending || !revocationReason.trim() || [...revocationReason.trim()].length > 1000} onClick={() => void dispatch({ kind: "revoke", draft, reason: revocationReason.trim() })}>{t($ => $.detail.completion_authorization.revoke)}</Button> : null}
        {canRetire && d.onRefresh ? <Button type="button" variant="outline" disabled={pending} onClick={() => void refresh()}>{t($ => $.detail.completion_authorization.new_review)}</Button> : null}
        {canSubmit && draft ? <Button type="button" disabled={pending} onClick={() => void dispatch({ kind: "submit", draft })}>{pending ? <Spinner className="size-4" /> : null}{t($ => $.detail.completion_authorization.submit)}</Button> : null}
        {!draft && preview?.authorization_binding ? <Button type="button" disabled={pending || !canAuthorize} onClick={() => { if (canAuthorize) void dispatch({ kind: "authorize", preview, reason: reason.trim(), expiresAt }); }}>{pending ? <Spinner className="size-4" /> : null}{t($ => $.detail.completion_authorization.authorize)}</Button> : null}
      </DialogFooter>
    </DialogContent>
  </Dialog>;
}
