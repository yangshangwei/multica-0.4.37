"use client";

import { useEffect, useId, useRef, useState, type FormEvent } from "react";
import { adminCollaborationSnapshotMatches, captureAdminCollaborationInput, isAdminCollaborationReasonValid, useAdminCollaborationCandidates,
  type AdminCollaborationChange, type AdminCollaborationDetail, type AdminCollaborationInput, type AdminScope } from "@multica/core/admin";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useT } from "../../i18n";
import { adminSelectClass } from "../filter-feedback";

export function AdminCollaborationForm({ scope, snapshot, action, current, refreshing, disabled, onSubmit, onClose }: {
  scope: AdminScope; snapshot: AdminCollaborationDetail; action: AdminCollaborationChange["action"]; current: AdminCollaborationDetail | null;
  refreshing: boolean; disabled: boolean; onSubmit(input: AdminCollaborationInput): void; onClose(): void;
}) {
  const { t } = useT("admin");
  const id = useId();
  const reasonRef = useRef<HTMLTextAreaElement>(null);
  const successorRef = useRef<HTMLFieldSetElement>(null);
  const [key] = useState(() => crypto.randomUUID());
  const options = snapshot.actions.find(option => option.action === action);
  const [status, setStatus] = useState(options?.default_status ?? "");
  const [reason, setReason] = useState("");
  const [successor, setSuccessor] = useState<string | null>(null);
  const [expired, setExpired] = useState(false);
  const [invalid, setInvalid] = useState<"reason" | "reason_invalid" | "successor" | "input" | null>(null);
  const needsSuccessor = action === "reopen" && snapshot.successorRequired;
  const candidates = useAdminCollaborationCandidates(scope, snapshot.issueId, snapshot.collaborationRevision, needsSuccessor && !disabled);
  const candidateItems = candidates.isError ? [] : candidates.data?.pages.flatMap(page => page.items) ?? [];
  const stale = !adminCollaborationSnapshotMatches(snapshot, current);
  const blocked = disabled || refreshing || stale || (needsSuccessor && (candidates.isPending || candidates.isError || candidates.isFetching));
  useEffect(() => { reasonRef.current?.focus(); }, []);
  useEffect(() => {
    if (!successor || candidates.isFetching || !candidates.isSuccess) return;
    if (!candidates.data.pages.some(page => page.items.some(item => item.userId === successor))) { setSuccessor(null); setExpired(true); }
  }, [successor, candidates.isFetching, candidates.isSuccess, candidates.data]);

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (blocked) return;
    if (!reason.trim()) { setInvalid("reason"); reasonRef.current?.focus(); return; }
    if (!isAdminCollaborationReasonValid(reason)) { setInvalid("reason_invalid"); reasonRef.current?.focus(); return; }
    if (needsSuccessor && !successor) { setInvalid("successor"); successorRef.current?.focus(); return; }
    try {
      const body: AdminCollaborationChange = { action, expectedCollaborationRevision: snapshot.collaborationRevision, expectedStatus: snapshot.status,
        targetStatus: status, statusOptionsVersion: options?.status_options_version ?? "", reason, ...(needsSuccessor && successor ? { successorUserId: successor } : {}) };
      const input = captureAdminCollaborationInput(snapshot, key, body, candidates.data?.pages);
      setInvalid(null); onSubmit(input);
    } catch { setInvalid("input"); }
  }
  return <section aria-labelledby={`${id}-title`} className="min-w-0 space-y-4 rounded-md border border-surface-border p-4 sm:p-5">
    <h2 id={`${id}-title`} className="text-body-lg font-semibold">{t($ => $.collaboration.actions[action])}</h2>
    <p className="max-w-prose text-body text-muted-foreground">{t($ => $.collaboration.lifecycleNotice)}</p>
    <dl className="grid min-w-0 gap-3 text-caption sm:grid-cols-2">
      <div className="min-w-0"><dt className="text-muted-foreground">{t($ => $.collaboration.currentStatus)}</dt><dd>{snapshot.status}</dd></div>
      <div className="min-w-0"><dt className="text-muted-foreground">{t($ => $.collaboration.revision)}</dt><dd>{snapshot.collaborationRevision}</dd></div>
    </dl>
    {stale && <p role="alert" className="text-body">{t($ => $.collaboration.snapshotChanged)}</p>}
    <form noValidate onSubmit={submit} className="grid min-w-0 max-w-2xl gap-4">
      <div className="min-w-0 space-y-1.5">
        <label htmlFor={`${id}-status`} className="block text-body">{t($ => $.collaboration.targetStatus)}</label>
        <select id={`${id}-status`} value={status} onChange={event => setStatus(event.target.value)} disabled={blocked} className={`${adminSelectClass} min-h-11`}>
          {options?.options.map(option => <option key={option.key} value={option.key}>{option.name || option.key}</option>)}
        </select>
      </div>
      {needsSuccessor && <fieldset ref={successorRef} tabIndex={-1} aria-describedby={invalid === "successor" ? `${id}-successor-hint ${id}-successor-error` : `${id}-successor-hint`} aria-invalid={invalid === "successor" || undefined}
        className="min-w-0 space-y-3 rounded-md outline-none focus-visible:ring-2 focus-visible:ring-ring">
        <legend className="mb-1 text-body font-medium">{t($ => $.collaboration.successor)}</legend>
        <p id={`${id}-successor-hint`} className="max-w-prose text-caption text-muted-foreground">{t($ => $.collaboration.successorHint)}</p>
        {candidates.isError ? <p role="alert" className="text-body">{t($ => $.collaboration.candidatesUnavailable)}</p>
          : candidates.isPending ? <p role="status" className="text-body text-muted-foreground">{t($ => $.collaboration.candidatesLoading)}</p>
          : !candidateItems.length ? <p className="text-body text-muted-foreground">{t($ => $.collaboration.successorEmpty)}</p>
          : <div className="grid min-w-0 gap-2">
            {candidateItems.map(candidate => <label key={candidate.userId} className="flex min-h-11 min-w-0 cursor-pointer items-start gap-3 rounded-md border border-surface-border px-3 py-2 has-checked:bg-accent has-checked:font-medium">
              <input type="radio" name={`${id}-successor`} value={candidate.userId} checked={successor === candidate.userId} disabled={blocked}
                onChange={() => { setSuccessor(candidate.userId); setExpired(false); setInvalid(null); }} className="mt-1 size-4 shrink-0 accent-primary" />
              <span className="min-w-0 text-body">{candidate.name || candidate.userId}{candidate.external && <span className="mt-1 block text-caption text-muted-foreground">{t($ => $.collaboration.external)}</span>}</span>
            </label>)}
          </div>}
        {expired && <p role="status" className="text-body">{t($ => $.collaboration.successorExpired)}</p>}
        {invalid === "successor" && <p id={`${id}-successor-error`} role="alert" className="text-body">{t($ => $.collaboration.successorRequired)}</p>}
        <div className="flex flex-wrap gap-2">
          <Button type="button" variant="outline" className="h-auto min-h-11 whitespace-normal" disabled={disabled || candidates.isFetching} onClick={() => { void candidates.refetch(); }}>{t($ => $.collaboration.candidatesRefresh)}</Button>
          {candidates.hasNextPage && !candidates.isError && <Button type="button" variant="outline" className="h-auto min-h-11 whitespace-normal" disabled={disabled || candidates.isFetching} onClick={() => { void candidates.fetchNextPage(); }}>{t($ => $.collaboration.candidatesMore)}</Button>}
        </div>
      </fieldset>}
      <div className="min-w-0 space-y-1.5">
        <label htmlFor={`${id}-reason`} className="block text-body">{t($ => $.collaboration.reason)}</label>
        <Textarea id={`${id}-reason`} ref={reasonRef} value={reason} onChange={event => { setReason(event.target.value); setInvalid(null); }}
          maxLength={1000} required disabled={disabled} aria-invalid={invalid === "reason" || invalid === "reason_invalid" || undefined}
          aria-describedby={invalid === "reason" || invalid === "reason_invalid" ? `${id}-reason-error` : undefined} />
        {(invalid === "reason" || invalid === "reason_invalid") && <p id={`${id}-reason-error`} role="alert" className="text-body">
          {invalid === "reason" ? t($ => $.collaboration.reasonRequired) : t($ => $.collaboration.reasonInvalid)}
        </p>}
      </div>
      {invalid === "input" && <p role="alert" className="text-body">{t($ => $.collaboration.invalid)}</p>}
      <div className="flex flex-wrap gap-2">
        <Button type="submit" className="h-auto min-h-11 whitespace-normal" disabled={blocked}>{t($ => $.collaboration.confirm)}</Button>
        <Button type="button" variant="ghost" className="h-auto min-h-11 whitespace-normal" disabled={disabled} onClick={onClose}>{t($ => $.collaboration.close)}</Button>
      </div>
    </form>
  </section>;
}
