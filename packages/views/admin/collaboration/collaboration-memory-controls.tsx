"use client";

import { useRef, useState } from "react";
import {
  AdminCollaborationMemoryRejectedError, AdminControlDraftError,
  captureAdminCollaborationMemoryPurge, dismissAdminCollaborationMemoryReceipt, isAdminCollaborationMemoryReasonValid,
  useAdminCollaborationMemoryDraft, useAdminCollaborationMemoryOperation,
  useAdminCollaborationMemoryPurge, useAdminCollaborationMemoryVersions,
  type AdminCollaborationDetail, type AdminCollaborationMemoryInput, type AdminCollaborationMemoryResult, type AdminScope,
} from "@multica/core/admin";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useT } from "../../i18n";

interface ControlsProps { scope: AdminScope; issueId: string; detail: AdminCollaborationDetail | null; refreshing: boolean }

export function AdminCollaborationMemoryControls(props: ControlsProps) {
  return <MemoryControls key={JSON.stringify([props.scope.apiScope, props.scope.userId, props.scope.organizationId, props.issueId])} {...props} />;
}

function MemoryControls({ scope, issueId, detail, refreshing }: ControlsProps) {
  const { t } = useT("admin");
  const saved = useAdminCollaborationMemoryDraft(scope, issueId);
  const versions = useAdminCollaborationMemoryVersions(scope, issueId, !!detail);
  const purge = useAdminCollaborationMemoryPurge(scope);
  const [selected, setSelected] = useState<string | null>(null);
  const [reason, setReason] = useState("");
  const [sent, setSent] = useState<AdminCollaborationMemoryInput | null>(null);
  const [rejected, setRejected] = useState(false);
  const [feedback, setFeedback] = useState<string | null>(null);
  const submitting = useRef(false);
  const panel = useRef<HTMLElement>(null);
  const input = sent ?? saved.draft?.input ?? null;
  const lookup = useAdminCollaborationMemoryOperation(scope, input, !sent && !!saved.draft);
  const receipt = !lookup.isError ? lookup.data ?? purge.data : undefined;
  const pages = detail && !versions.isError ? versions.data?.pages ?? [] : [];
  const items = [...new Map(pages.flatMap(page => page.items).map(item => [item.version, item])).values()];
  const currentVersion = pages[0]?.currentVersion ?? null;
  // Metadata has no independent permission flag. The current R38 detail is the
  // authority hint; the server rechecks the disabled creator on every write.
  const canPurge = detail?.creator.disabled === true && detail.issueId === issueId && detail.scope === scope.organizationId;
  const busy = purge.isPending || lookup.isFetching;
  const locked = busy || refreshing || versions.isFetching || saved.error || input !== null;
  const selectedItem = selected ? items.find(item => item.version === selected && !item.purged && item.version !== currentVersion) : null;

  function close() {
    setSent(null); setSelected(null); setReason(""); setRejected(false); setFeedback(null); purge.reset(); saved.reload();
    requestAnimationFrame(() => panel.current?.focus());
  }
  async function submit(value: AdminCollaborationMemoryInput) {
    if (busy || submitting.current) return;
    submitting.current = true; setSent(value); setFeedback(null);
    try { await purge.mutateAsync(value); }
    catch (error) {
      if (error instanceof AdminCollaborationMemoryRejectedError) { setRejected(true); setFeedback(t(($) => $.collaboration.memory.rejected)); }
      else if (error instanceof AdminControlDraftError) { setSent(null); setFeedback(t(($) => $.operations.storageFailed)); }
      else setFeedback(t(($) => $.collaboration.memory.failed));
    } finally { submitting.current = false; }
  }
  function confirm() {
    if (!canPurge || !detail || !selectedItem || locked) return;
    if (!isAdminCollaborationMemoryReasonValid(reason)) { setFeedback(t(($) => $.collaboration.memory.reasonInvalid)); return; }
    const value = captureAdminCollaborationMemoryPurge(issueId, detail.creator.userId, crypto.randomUUID(), selectedItem.version, reason);
    void submit(value);
  }

  if (saved.loading) return <p role="status" className="text-body text-muted-foreground">{t(($) => $.operations.restoring)}</p>;
  if (saved.error) return <p role="alert" className="text-body">{t(($) => $.operations.storageFailed)}</p>;
  return <section ref={panel} tabIndex={-1} className="min-w-0 space-y-4 rounded-md border border-surface-border p-4 outline-none focus-visible:ring-2 focus-visible:ring-ring sm:p-5" aria-label={t(($) => $.collaboration.memory.title)}>
    <div className="space-y-1"><h2 className="text-body-lg font-semibold">{t(($) => $.collaboration.memory.title)}</h2><p className="max-w-prose text-caption text-muted-foreground">{t(($) => $.collaboration.memory.description)}</p></div>
    {detail && versions.isPending && <p role="status" className="text-body text-muted-foreground">{t(($) => $.collaboration.memory.loading)}</p>}
    {detail && versions.error && <p role="alert" className="text-body text-muted-foreground">{t(($) => $.collaboration.memory.unavailable)}</p>}
    {detail && versions.isSuccess && items.length === 0 && <p className="text-body text-muted-foreground">{t(($) => $.collaboration.memory.empty)}</p>}
    {items.length > 0 && <ul className="space-y-2">{items.map(item => <li key={item.version} className="flex min-w-0 flex-wrap items-center justify-between gap-3 rounded-md border border-border p-3">
      <div className="min-w-0"><p className="font-medium">{t(($) => $.collaboration.memory.version, { version: item.version })}{item.version === currentVersion && <span className="ml-2 text-caption">{t(($) => $.collaboration.memory.current)}</span>}{item.purged && <span className="ml-2 text-caption text-muted-foreground">{t(($) => $.collaboration.memory.purged)}</span>}</p><p className="text-caption text-muted-foreground">{t(($) => $.collaboration.memory.bytes, { bytes: item.bytes })}</p></div>
      {canPurge && item.version !== currentVersion && !item.purged && <Button type="button" variant="outline" className="pointer-coarse:min-h-11" disabled={locked} onClick={() => { setSelected(item.version); setReason(""); setFeedback(null); purge.reset(); }}>{t(($) => $.collaboration.memory.purge, { version: item.version })}</Button>}
    </li>)}</ul>}
    {detail && versions.hasNextPage && <Button type="button" variant="outline" className="pointer-coarse:min-h-11" disabled={busy || versions.isFetching} onClick={() => void versions.fetchNextPage()}>{t(($) => $.collaboration.memory.more)}</Button>}
    {canPurge && selectedItem && !input && <form className="space-y-3 rounded-md border border-destructive/40 p-3" onSubmit={event => { event.preventDefault(); confirm(); }}>
      <h3 className="text-body font-medium">{t(($) => $.collaboration.memory.confirmTitle, { version: selectedItem.version })}</h3><p className="text-caption text-muted-foreground">{t(($) => $.collaboration.memory.confirmNotice)}</p>
      <Textarea aria-label={t(($) => $.collaboration.memory.reason)} value={reason} disabled={locked} onChange={event => { setReason(event.target.value); setFeedback(null); }} className="min-h-20" />
      <div className="flex flex-wrap gap-2"><Button type="submit" variant="destructive" className="pointer-coarse:min-h-11" disabled={locked || reason.trim() === ""}>{t(($) => $.collaboration.memory.confirm)}</Button><Button type="button" variant="ghost" className="pointer-coarse:min-h-11" disabled={busy} onClick={close}>{t(($) => $.collaboration.memory.cancel)}</Button></div>
    </form>}
    {input && <div className="space-y-3">
      <AdminMemoryReceipt input={input} receipt={receipt ?? undefined} />
      {busy && <p role="status" className="text-body text-muted-foreground">{t(($) => $.operations.restoring)}</p>}
      {!receipt && !busy && !rejected && lookup.isSuccess && lookup.data === null && <p role="status">{t(($) => $.collaboration.memory.notRecorded)}</p>}
      <div className="flex flex-wrap gap-2">{receipt ? <Button type="button" variant="ghost" className="pointer-coarse:min-h-11" onClick={() => {
        try { dismissAdminCollaborationMemoryReceipt(scope, input, receipt); close(); }
        catch { setFeedback(t(($) => $.operations.storageFailed)); }
      }}>{t(($) => $.collaboration.memory.dismiss)}</Button> : rejected ? <Button type="button" variant="ghost" onClick={close}>{t(($) => $.collaboration.close)}</Button> : <>
        <Button type="button" variant="outline" className="pointer-coarse:min-h-11" disabled={busy} onClick={() => { setFeedback(null); void lookup.refetch(); }}>{t(($) => $.collaboration.memory.check)}</Button>
        <Button type="button" variant="outline" className="pointer-coarse:min-h-11" disabled={busy} onClick={() => void submit(input)}>{t(($) => $.collaboration.memory.retryOriginal)}</Button>
      </>}</div>
    </div>}
    {feedback && <p role="alert" className="text-body">{feedback}</p>}
  </section>;
}

function AdminMemoryReceipt({ input, receipt }: { input: AdminCollaborationMemoryInput; receipt?: AdminCollaborationMemoryResult }) {
  const { t } = useT("admin");
  return <section aria-label={receipt ? t(($) => $.collaboration.memory.receiptTitle) : t(($) => $.collaboration.memory.pendingTitle)} className="min-w-0 space-y-3">
    <h3 className="text-body-lg font-semibold">{receipt ? t(($) => $.collaboration.memory.receiptTitle) : t(($) => $.collaboration.memory.pendingTitle)}</h3>
    {receipt && <p role="status" className="text-body">{t(($) => $.collaboration.memory.outcome, { version: receipt.result.version })}</p>}
    <dl className="grid min-w-0 gap-3 text-caption sm:grid-cols-2">
      <div><dt className="text-muted-foreground">{t(($) => $.collaboration.memory.version, { version: input.body.version })}</dt><dd>{t(($) => $.collaboration.memory.bytes, { bytes: receipt?.result.bytes ?? "—" })}</dd></div>
      <div><dt className="text-muted-foreground">{t(($) => $.collaboration.memory.reason)}</dt><dd className="whitespace-pre-wrap">{input.body.reason}</dd></div>
    </dl>
  </section>;
}
