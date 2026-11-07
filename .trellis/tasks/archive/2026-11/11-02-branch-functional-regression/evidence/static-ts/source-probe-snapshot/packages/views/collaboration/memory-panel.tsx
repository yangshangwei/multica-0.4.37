"use client";

import { useEffect, useId, useRef, useState } from "react";
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  CollaborationDraftError, CollaborationMemoryTooLargeError, CollaborationUnavailableError,
  collaborationIssueAccessDenied, collaborationKeys, collaborationMemoryOptions, collaborationMemoryVersionOptions,
  collaborationMemoryVersionsOptions, collaborationMemoryReasonValid, collaborationTextByteLength,
  executeCollaborationMemoryChange, isCollaborationDenied, parseCollaborationMemoryDraft,
  prepareCollaborationMemoryChange, useCollaborationPendingDrafts,
  type CollaborationDraft, type CollaborationIssue, type CollaborationMemory, type CollaborationMemoryChange,
  type CollaborationMemoryReceipt, type CollaborationMemoryVersionMeta, type CollaborationScope,
} from "@multica/core/collaboration";
import { accountPath, type AccountDestination } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription,
  AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@multica/ui/components/ui/alert-dialog";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@multica/ui/components/ui/collapsible";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@multica/ui/components/ui/dialog";
import { AppLink } from "../navigation";
import { RichContent } from "../rich-content";
import { useLocale, useT } from "../i18n";
import { CollaborationQueryState } from "./query-state";
import { recoverFocus } from "./focus";

interface PanelProps { scope: CollaborationScope; issue: CollaborationIssue; destination?: AccountDestination; isScopeResume?: boolean; navigationKey?: number }
interface Editor { baseVersion: string; content: string }
interface Confirmation { action: "restore" | "purge"; version: string; snapshot: CollaborationMemory }

export function CollaborationMemoryPanel(props: PanelProps) {
  const identity = JSON.stringify([props.scope.apiScope, props.scope.userId, props.issue.id]);
  return <MemoryPanel key={identity} {...props} />;
}

function MemoryAttribution({ author, createdAt, issueId }: { author: CollaborationMemory["author"]; createdAt: string | null; issueId: string }) {
  const { t } = useT("collaboration");
  const locale = useLocale();
  if (!author) return null;
  const name = author.member?.name || (author.kind === "agent"
    ? author.agent?.name ? t(($) => $.memory.agentWriter, { name: author.agent.name }) : t(($) => $.memory.unknownAgent)
    : t(($) => $.memory.unknownWriter));
  return <dl className="flex min-w-0 flex-wrap gap-x-6 gap-y-2 text-caption [&_dt]:text-muted-foreground [&_dd]:mt-1 [&_dd]:[overflow-wrap:anywhere]">
    <div><dt>{t(($) => $.memory.writer)}</dt><dd>{name}</dd></div>
    {author.resource_owner && <div><dt>{t(($) => $.memory.resourceOwner)}</dt><dd>{author.resource_owner.name}</dd></div>}
    {author.agent?.run_id && <div><dt>{t(($) => $.memory.sourceRun)}</dt><dd><AppLink className="underline underline-offset-4" href={accountPath({ view: "participating", issueId, runId: author.agent.run_id })}>{t(($) => $.memory.openRun)}</AppLink></dd></div>}
    {createdAt && <div><dt>{t(($) => $.memory.savedAt)}</dt><dd><time dateTime={createdAt}>{new Date(createdAt).toLocaleString(locale)}</time></dd></div>}
  </dl>;
}

function MemoryPanel({ scope, issue, destination, isScopeResume = false, navigationKey = 0 }: PanelProps) {
  const { t } = useT("collaboration");
  const client = useQueryClient();
  const fieldId = useId();
  const root = useRef<HTMLElement>(null);
  const editButton = useRef<HTMLButtonElement>(null);
  const editorField = useRef<HTMLTextAreaElement>(null);
  const submitting = useRef(false);
  const memoryQuery = useQuery({ ...collaborationMemoryOptions(client, scope, issue.id), enabled: issue.capabilities.read === true });
  const drafts = useCollaborationPendingDrafts({ ...scope, issueId: issue.id });
  const [editor, setEditor] = useState<Editor | null>(null);
  const [expanded, setExpanded] = useState(false);
  const [historyOpen, setHistoryOpen] = useState(false);
  const [selectedVersion, setSelectedVersion] = useState<string | null>(isScopeResume ? null : destination?.memoryVersion ?? null);
  const [confirmation, setConfirmation] = useState<Confirmation | null>(null);
  const [reason, setReason] = useState("");
  const [feedback, setFeedback] = useState<string | null>(null);
  const memory = memoryQuery.data;
  const denied = collaborationIssueAccessDenied(client, scope, issue.id) || isCollaborationDenied(memoryQuery.error);
  const historyQuery = useInfiniteQuery({ ...collaborationMemoryVersionsOptions(client, scope, issue.id), enabled: !!memory && historyOpen && !denied });
  const selectedQuery = useQuery({ ...collaborationMemoryVersionOptions(client, scope, issue.id, selectedVersion ?? "1"), enabled: !!memory && selectedVersion !== null && !denied });
  const send = useMutation({
    mutationKey: [...collaborationKeys.issue(scope, issue.id), "memory-write"], retry: false, networkMode: "always",
    mutationFn: ({ draft, recovery }: { draft: CollaborationDraft; recovery: boolean }) => executeCollaborationMemoryChange(client, scope, draft, recovery),
    onSuccess: (receipt, { draft }) => {
      if (collaborationIssueAccessDenied(client, scope, issue.id)) return;
      const payload = parseCollaborationMemoryDraft(draft);
      if (receipt.status < 400) { setEditor(null); setConfirmation(null); setReason(""); recoverFocus(() => editButton.current, () => root.current); }
      else if (payload.action === "memory_update") setEditor({ baseVersion: payload.input.expected_version, content: payload.input.content });
      setFeedback(receiptMessage(receipt));
    },
    onError: (error) => {
      setFeedback(error instanceof CollaborationDraftError ? t(($) => $.workflow.storageError) : null);
      if (!(error instanceof CollaborationDraftError)) setConfirmation(null);
    },
  });
  const locked = send.isPending || drafts.memory.length > 0 || drafts.error;
  const resetSend = send.reset;
  const memoryVersion = memory?.version;
  const stale = editor !== null && (editor.baseVersion !== memory?.version || send.data?.code === "stale_memory_version");
  const closed = issue.status_category === "done" || issue.status_category === "cancelled";
  const canEdit = memory?.can_edit === true && !closed;
  const staleRestore = confirmation?.action === "restore" && (confirmation.snapshot.version !== memoryVersion || send.data?.code === "stale_memory_version");
  const bytes = editor ? collaborationTextByteLength(editor.content) : memory?.bytes ?? 0;
  const overLimit = !!memory && bytes > memory.max_bytes;
  const feedbackRole = send.data && send.data.status < 400 ? "status" : "alert";
  const reasonInvalid = reason !== "" && !collaborationMemoryReasonValid(reason);

  function receiptMessage(receipt: CollaborationMemoryReceipt): string {
    switch (receipt.code) {
      case "memory_updated": return t(($) => $.memory.updated, { version: receipt.version ?? "" });
      case "memory_restored": return t(($) => $.memory.restored, { version: receipt.version ?? "" });
      case "memory_purged": return t(($) => $.memory.purged, { version: receipt.version ?? "" });
      case "stale_memory_version": return t(($) => $.memory.conflict);
      case "memory_too_large": return t(($) => $.memory.tooLong, { max: receipt.max_bytes });
      case "invalid_memory_content": return t(($) => $.memory.invalidContent);
      case "memory_version_purged": return t(($) => $.memory.alreadyPurged);
      case "memory_version_not_found": return t(($) => $.memory.notFound);
      case "memory_version_current": return t(($) => $.memory.currentNotPurgeable);
      case "issue_closed": return t(($) => $.memory.closed);
      default: return t(($) => $.memory.refused);
    }
  }
  function submit(change: CollaborationMemoryChange, snapshot = memory) {
    if (!snapshot || locked || submitting.current) return;
    try {
      const draft = prepareCollaborationMemoryChange(scope, snapshot, crypto.randomUUID(), change);
      setFeedback(null); submitting.current = true;
      void send.mutateAsync({ draft, recovery: false }).catch(() => {}).finally(() => { submitting.current = false; });
    } catch (error) {
      send.reset();
      setFeedback(error instanceof CollaborationMemoryTooLargeError ? t(($) => $.memory.tooLong, { max: error.maxBytes }) : t(($) => $.memory.invalidContent));
    }
  }
  function openConfirmation(action: Confirmation["action"], version: string) {
    if (!memory || locked) return;
    setSelectedVersion(null); setConfirmation({ action, version, snapshot: memory }); setReason(""); setFeedback(null); send.reset();
  }
  useEffect(() => {
    if (!denied) return;
    setEditor(null); setConfirmation(null); setSelectedVersion(null); setReason(""); setFeedback(null); resetSend();
  }, [denied, resetSend]); // The access fence also removes retained requests and Query data.
  useEffect(() => { if (!isScopeResume) setSelectedVersion(destination?.memoryVersion ?? null); }, [destination?.memoryVersion, isScopeResume, navigationKey]);
  useEffect(() => {
    if (isScopeResume || destination?.anchor !== "memory" || memoryVersion === undefined) return;
    root.current?.scrollIntoView?.({ block: "start" }); root.current?.focus({ preventScroll: true });
  }, [destination?.anchor, memoryVersion, isScopeResume, navigationKey]);

  if (denied) return <CollaborationQueryState denied error={memoryQuery.error} />;
  if (memoryQuery.isPending) return <section id="memory" tabIndex={-1} className="min-w-0 space-y-3"><h3 className="text-body font-semibold">{t(($) => $.memory.title)}</h3><p role="status" className="text-body text-muted-foreground">{t(($) => $.memory.loading)}</p></section>;
  if (!memory || memoryQuery.error instanceof CollaborationUnavailableError) return null;
  const historyItems = [...new Map((historyQuery.data?.pages.flatMap(page => page.items) ?? []).map(item => [item.version, item])).values()];
  const selected = !selectedQuery.isError ? selectedQuery.data : undefined;
  const pendingUpdate = drafts.memory.map(parseCollaborationMemoryDraft).find(payload => payload.action === "memory_update");
  return <section ref={root} id="memory" tabIndex={-1} aria-labelledby={`${fieldId}-title`} className="min-w-0 space-y-5 rounded-md border border-border p-4 outline-none focus-visible:ring-2 focus-visible:ring-ring sm:p-5">
    <div className="space-y-2"><h3 id={`${fieldId}-title`} className="text-body font-semibold">{t(($) => $.memory.title)}</h3><p className="max-w-prose text-caption text-muted-foreground">{t(($) => $.memory.description)}</p></div>
    {memoryQuery.isError && <CollaborationQueryState error={memoryQuery.error} retry={() => void memoryQuery.refetch()} />}
    <div className="flex min-w-0 flex-wrap items-center justify-between gap-3">
      <p className="text-caption text-muted-foreground">{t(($) => $.memory.versionLabel, { version: memory.version })} · {t(($) => $.memory.bytes, { used: bytes, max: memory.max_bytes })}</p>
      {canEdit && !editor && drafts.memory.length === 0 && <Button ref={editButton} type="button" variant="outline" className="pointer-coarse:min-h-11" disabled={locked} onClick={() => { send.reset(); setFeedback(null); setEditor({ baseVersion: memory.version, content: memory.content }); recoverFocus(() => editorField.current); }}>{t(($) => $.memory.edit)}</Button>}
    </div>
    {memory.bytes > memory.max_bytes && <p className="text-caption text-destructive">{t(($) => $.memory.overLimit)}</p>}
    {editor ? <form aria-label={t(($) => $.memory.edit)} className="space-y-3" onSubmit={event => {
      event.preventDefault(); if (canEdit && !stale && !overLimit) submit({ action: "update", baseVersion: editor.baseVersion, content: editor.content });
    }}>
      <Textarea ref={editorField} id={fieldId} aria-label={t(($) => $.memory.editLabel)} aria-invalid={overLimit || undefined} aria-describedby={overLimit ? `${fieldId}-hint ${fieldId}-limit` : `${fieldId}-hint`} value={editor.content} disabled={locked || !canEdit} onChange={event => setEditor({ ...editor, content: event.target.value })} className="min-h-48 font-mono text-body" />
      <p id={`${fieldId}-hint`} className="text-caption text-muted-foreground">{t(($) => $.memory.editHint)}</p>
      {overLimit && <p id={`${fieldId}-limit`} role="alert" className="text-caption text-destructive">{t(($) => $.memory.tooLong, { max: memory.max_bytes })}</p>}
      {stale && <div className="space-y-3"><p role="alert" className="text-body">{t(($) => $.memory.conflict)}</p>
        <div className="max-h-64 overflow-auto rounded-md bg-muted/30 p-3"><p className="mb-2 text-caption font-medium">{t(($) => $.memory.current)} · {t(($) => $.memory.versionLabel, { version: memory.version })}</p><RichContent content={memory.content} resourcePolicy="restricted" /></div>
        <Button type="button" variant="outline" className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11" disabled={locked || !canEdit || memoryQuery.isFetching || (send.data?.code === "stale_memory_version" && memory.version === editor.baseVersion)} onClick={() => { setEditor({ ...editor, baseVersion: memory.version }); setFeedback(null); send.reset(); }}>{t(($) => $.memory.useCurrent)}</Button>
      </div>}
      <div className="flex flex-wrap gap-2"><Button type="submit" className="pointer-coarse:min-h-11" disabled={locked || stale || overLimit || !canEdit}>{t(($) => $.memory.save)}</Button><Button type="button" variant="ghost" className="pointer-coarse:min-h-11" disabled={locked} onClick={() => { setEditor(null); setFeedback(null); send.reset(); recoverFocus(() => editButton.current, () => root.current); }}>{t(($) => $.memory.cancel)}</Button></div>
    </form> : <>
      {memory.content ? <div className="space-y-2"><div id={`${fieldId}-content`} className={`min-w-0 overflow-auto rounded-md bg-muted/30 p-3 ${expanded ? "" : "max-h-64"}`}><RichContent content={memory.content} resourcePolicy="restricted" /></div><Button type="button" variant="ghost" size="sm" className="pointer-coarse:min-h-11" aria-expanded={expanded} aria-controls={`${fieldId}-content`} onClick={() => setExpanded(!expanded)}>{expanded ? t(($) => $.memory.collapse) : t(($) => $.memory.expand)}</Button></div> : <p className="text-body text-muted-foreground">{t(($) => $.memory.empty)}</p>}
      <MemoryAttribution author={memory.author} createdAt={memory.created_at} issueId={issue.id} />
      {memory.restored_from_version !== null && <p className="text-caption text-muted-foreground">{t(($) => $.memory.restoredFrom, { version: memory.restored_from_version })}</p>}
      {pendingUpdate?.action === "memory_update" && <Textarea aria-label={t(($) => $.memory.editLabel)} value={pendingUpdate.input.content} readOnly className="min-h-32 font-mono" />}
    </>}
    {closed && <p className="text-caption text-muted-foreground">{t(($) => $.memory.closed)}</p>}
    {drafts.error && <p role="alert">{t(($) => $.workflow.storageError)}</p>}
    {send.isPending && <p role="status">{t(($) => $.workflow.pending)}</p>}
    {feedback && !(stale && feedback === t(($) => $.memory.conflict)) && <p key={feedbackRole} role={feedbackRole} className="text-body">{feedback}</p>}
    {drafts.memory.map(draft => { const payload = parseCollaborationMemoryDraft(draft); return <div key={draft.requestId} className="space-y-2">
      <p role="status">{payload.action === "memory_update" ? t(($) => $.memory.pendingUpdate) : payload.action === "memory_restore" ? t(($) => $.memory.pendingRestore, { version: payload.version }) : t(($) => $.memory.pendingPurge, { version: payload.version })}</p>
      <div className="flex flex-wrap gap-2"><Button type="button" variant="outline" className="pointer-coarse:min-h-11" disabled={send.isPending || drafts.error} onClick={() => send.mutate({ draft, recovery: true })}>{t(($) => $.workflow.recover)}</Button><Button type="button" variant="outline" className="pointer-coarse:min-h-11" disabled={send.isPending || drafts.error} onClick={() => send.mutate({ draft, recovery: false })}>{t(($) => $.workflow.retryOriginal)}</Button></div>
    </div>; })}
    <Collapsible open={historyOpen} onOpenChange={setHistoryOpen} className="space-y-3 border-t border-border pt-4">
      <CollapsibleTrigger render={<Button type="button" variant="ghost" className="pointer-coarse:min-h-11" />}>{t(($) => $.memory.history)}</CollapsibleTrigger>
      <CollapsibleContent className="space-y-3">
        {historyQuery.isPending && <p role="status" className="text-caption">{t(($) => $.memory.loading)}</p>}
        {historyQuery.isError && <CollaborationQueryState error={historyQuery.error} retry={() => void historyQuery.refetch()} />}
        {historyQuery.isSuccess && historyItems.length === 0 && <p className="text-caption text-muted-foreground">{t(($) => $.memory.historyEmpty)}</p>}
        <ul className="space-y-2">{historyItems.map(item => <MemoryHistoryRow key={item.version} item={item} issueId={issue.id} currentVersion={memory.version} canEdit={canEdit} canPurge={memory.can_purge} disabled={locked} onView={() => setSelectedVersion(item.version)} onRestore={() => openConfirmation("restore", item.version)} onPurge={() => openConfirmation("purge", item.version)} />)}</ul>
        {historyQuery.hasNextPage && <Button type="button" variant="outline" className="pointer-coarse:min-h-11" disabled={historyQuery.isFetchingNextPage} onClick={() => void historyQuery.fetchNextPage()}>{t(($) => $.memory.historyMore)}</Button>}
      </CollapsibleContent>
    </Collapsible>
    <Dialog open={selectedVersion !== null} onOpenChange={open => { if (!open) setSelectedVersion(null); }}>
      <DialogContent closeLabel={t(($) => $.memory.close)} finalFocus={() => root.current} className="max-h-[85dvh] overflow-y-auto sm:max-w-2xl">
        <DialogTitle>{t(($) => $.memory.viewTitle, { version: selectedVersion ?? "" })}</DialogTitle>
        <DialogDescription>{selected?.purged ? t(($) => $.memory.viewPurged) : t(($) => $.memory.description)}</DialogDescription>
        {selectedQuery.isPending && <p role="status">{t(($) => $.memory.loading)}</p>}
        {selectedQuery.isError && <CollaborationQueryState error={selectedQuery.error} retry={() => void selectedQuery.refetch()} />}
        {selected && <><p className="text-caption text-muted-foreground">{t(($) => $.memory.bytes, { used: selected.bytes, max: selected.max_bytes })}</p>{selected.content !== null && <RichContent content={selected.content} resourcePolicy="restricted" />}<MemoryAttribution author={selected.author} createdAt={selected.created_at} issueId={issue.id} />
          <div className="flex flex-wrap gap-2">{canEdit && selected.version !== memory.version && !selected.purged && <Button type="button" variant="outline" className="pointer-coarse:min-h-11" disabled={locked} onClick={() => openConfirmation("restore", selected.version)}>{t(($) => $.memory.restore, { version: selected.version })}</Button>}{memory.can_purge && selected.version !== memory.version && !selected.purged && <Button type="button" variant="outline" className="pointer-coarse:min-h-11" disabled={locked} onClick={() => openConfirmation("purge", selected.version)}>{t(($) => $.memory.purge, { version: selected.version })}</Button>}</div>
        </>}
      </DialogContent>
    </Dialog>
    <AlertDialog open={confirmation !== null} onOpenChange={open => { if (!open && !send.isPending) { setConfirmation(null); setFeedback(null); } }}>
      <AlertDialogContent finalFocus={() => root.current}>
        <AlertDialogHeader><AlertDialogTitle>{confirmation?.action === "restore" ? t(($) => $.memory.restoreTitle, { version: confirmation.version }) : t(($) => $.memory.purgeTitle, { version: confirmation?.version ?? "" })}</AlertDialogTitle><AlertDialogDescription>{confirmation?.action === "restore" ? t(($) => $.memory.restoreBody, { version: confirmation.version }) : t(($) => $.memory.purgeBody)}</AlertDialogDescription></AlertDialogHeader>
        {confirmation?.action === "purge" && <><Textarea aria-label={t(($) => $.memory.purgeReason)} aria-invalid={reasonInvalid || undefined} aria-describedby={reasonInvalid ? `${fieldId}-reason` : undefined} value={reason} onChange={event => setReason(event.target.value)} disabled={locked} className="min-h-20" />{reasonInvalid && <p id={`${fieldId}-reason`} role="alert" className="text-caption text-destructive">{t(($) => $.memory.purgeReasonRequired)}</p>}</>}
        {feedback && <p role="alert">{feedback}</p>}
        {staleRestore && <div className="space-y-2"><p className="text-body">{t(($) => $.memory.conflict)}</p><Button type="button" variant="outline" className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11" disabled={locked || !canEdit || memoryQuery.isFetching} onClick={() => { if (confirmation) setConfirmation({ ...confirmation, snapshot: memory }); send.reset(); setFeedback(null); }}>{t(($) => $.memory.useCurrent)}</Button></div>}
        <AlertDialogFooter><AlertDialogCancel disabled={locked}>{t(($) => $.memory.cancel)}</AlertDialogCancel><AlertDialogAction variant={confirmation?.action === "purge" ? "destructive" : "default"} disabled={locked || (confirmation?.action === "restore" ? !canEdit || staleRestore : !memory.can_purge || confirmation?.version === memory.version || !collaborationMemoryReasonValid(reason))} onClick={() => {
          if (!confirmation) return;
          submit(confirmation.action === "restore" ? { action: "restore", version: confirmation.version } : { action: "purge", version: confirmation.version, reason }, confirmation.snapshot);
        }}>{confirmation?.action === "restore" ? t(($) => $.memory.restoreConfirm) : t(($) => $.memory.purgeConfirm)}</AlertDialogAction></AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </section>;
}

function MemoryHistoryRow({ item, issueId, currentVersion, canEdit, canPurge, onView, onRestore, onPurge, disabled }: {
  item: CollaborationMemoryVersionMeta; issueId: string; currentVersion: string; canEdit: boolean; canPurge: boolean;
  onView: () => void; onRestore: () => void; onPurge: () => void; disabled: boolean;
}) {
  const { t } = useT("collaboration");
  const historical = item.version !== currentVersion;
  return <li className="flex min-w-0 flex-wrap items-start justify-between gap-3 rounded-md border border-border p-3">
    <div className="min-w-0 space-y-2"><Button type="button" variant="link" className="h-auto p-0" aria-label={t(($) => $.memory.view, { version: item.version })} onClick={onView}>{t(($) => $.memory.versionLabel, { version: item.version })}</Button>
      {!historical && <span className="ml-2 text-caption font-medium">{t(($) => $.memory.current)}</span>}{item.purged && <span className="ml-2 text-caption text-muted-foreground">{t(($) => $.memory.purgedMarker)}</span>}
      <p className="text-caption text-muted-foreground">{t(($) => $.memory.versionBytes, { used: Math.ceil(item.bytes / 1024) })}</p>
      <MemoryAttribution author={item.author} createdAt={item.created_at} issueId={issueId} />
    </div>
    <div className="flex flex-wrap gap-2">{canEdit && historical && !item.purged && <Button type="button" variant="outline" size="sm" disabled={disabled} className="pointer-coarse:min-h-11" onClick={onRestore}>{t(($) => $.memory.restore, { version: item.version })}</Button>}{canPurge && historical && !item.purged && <Button type="button" variant="outline" size="sm" disabled={disabled} className="pointer-coarse:min-h-11" onClick={onPurge}>{t(($) => $.memory.purge, { version: item.version })}</Button>}</div>
  </li>;
}
