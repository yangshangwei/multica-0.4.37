import { useEffect, useId, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { collaborationCommentOptions, collaborationKeys, collaborationMentionMarkdown, collaborationMentionTargetsOptions, executeCollaborationComment,
  prepareCollaborationComment, useCollaborationCommentPreview, useCollaborationPendingDrafts,
  useCollaborationDeliveryForm,
  type CollaborationComment, type CollaborationDraft, type CollaborationIssue, type CollaborationScope } from "@multica/core/collaboration";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { useT } from "../i18n";
import { useCollaborationLabels } from "./labels";
import { CollaborationQueryState } from "./query-state";
import { CollaborationAttachmentPicker } from "./attachment-picker";

export function CollaborationCommentComposer({ scope, issue, replyTo = null, onCancelReply, onSent }: {
  scope: CollaborationScope; issue: CollaborationIssue; replyTo?: CollaborationComment | null; onCancelReply?: () => void; onSent?: () => void;
}) {
  const { t } = useT("collaboration");
  const labels = useCollaborationLabels();
  const client = useQueryClient();
  const fieldId = useId();
  const textarea = useRef<HTMLTextAreaElement>(null);
  const selectedParentId = replyTo?.issue_id === issue.id ? replyTo.id : null;
  const selectedParent = useRef<string | null>(null);
  const delivery = useCollaborationDeliveryForm({ server: scope.server, userId: scope.userId, context: { kind: "issue", issueId: issue.id, sourceWorkspaceId: issue.source_workspace.id } }, "comment");
  const parentId = selectedParentId ?? delivery.form.parentCommentId ?? null;
  const parent = useQuery({ ...collaborationCommentOptions(client, scope, issue.id, parentId ?? ""), enabled: parentId !== null && selectedParentId === null && issue.capabilities.comment === true });
  const parentComment = selectedParentId ? replyTo : parent.data;
  useEffect(() => {
    if (selectedParentId && selectedParent.current !== selectedParentId) delivery.update({ parentCommentId: selectedParentId });
    selectedParent.current = selectedParentId;
  }, [selectedParentId, delivery]);
  useEffect(() => {
    if (parentId) { textarea.current?.focus(); textarea.current?.scrollIntoView?.({ block: "nearest" }); }
  }, [parentId]);
  const content = delivery.form.content;
  const [uploading, setUploading] = useState(false);
  const [suppressed, setSuppressed] = useState<string[]>([]);
  const submitting = useRef(false);
  const mentions = useQuery({ ...collaborationMentionTargetsOptions(client, scope, issue.id), enabled: issue.capabilities.comment === true });
  const drafts = useCollaborationPendingDrafts({ ...scope, issueId: issue.id });
  const inspect = useCollaborationCommentPreview(scope, issue.id);
  const preview = inspect.variables?.parent_id === parentId ? inspect.data : undefined;
  const send = useMutation({ mutationKey: [...collaborationKeys.issue(scope, issue.id), "comment-send"], retry: false, networkMode: "always", mutationFn: ({ draft, recovery }: { draft: CollaborationDraft; recovery: boolean }) => executeCollaborationComment(client, scope, draft, recovery),
    onSuccess: (receipt) => { if (receipt.status >= 200 && receipt.status < 300) { delivery.reset(); inspect.reset(); setSuppressed([]); onSent?.(); } } });
  const locked = inspect.isPending || send.isPending || drafts.comments.length > 0 || drafts.error || uploading || delivery.uploads.length > 0 || Boolean(delivery.error);
  function change(next: string) { delivery.update({ content: next }); inspect.reset(); send.reset(); }
  const targetName = (target: { kind: "agent" | "squad"; resource_id: string }) => mentions.data?.targets.find((item) => item.kind === target.kind && item.resource_id === target.resource_id)?.name
    ?? `${target.kind === "squad" ? t(($) => $.execution.squad) : t(($) => $.execution.agent)} ${target.resource_id}`;
  if (issue.capabilities.comment !== true) return null;
  return <form aria-label={t(($) => $.detail.comments)} className="min-w-0 space-y-4" onSubmit={(event) => {
    event.preventDefault();
    if (!preview || locked || submitting.current) return;
    const draft = prepareCollaborationComment(scope, issue.id, crypto.randomUUID(), content, suppressed, preview, { draftId: delivery.form.draftId, attachmentIds: delivery.form.selectedAttachmentIds }, parentId);
    submitting.current = true;
    void send.mutateAsync({ draft, recovery: false }).catch(() => {}).finally(() => { submitting.current = false; });
  }}>
    <label htmlFor={fieldId} className="block text-body font-medium">{t(($) => $.composer.label)}</label>
    {parentId && <div className="flex flex-wrap items-center gap-3 text-caption text-muted-foreground"><span>{t(($) => $.commentActions.replyingTo, { name: parentComment?.author?.name ?? (parentComment?.author_type === "agent" ? t(($) => $.execution.agent) : t(($) => $.detail.unknownAuthor)) })}</span>
      <Button type="button" variant="ghost" size="sm" disabled={locked} className="pointer-coarse:min-h-11" onClick={() => { delivery.update({ parentCommentId: null }); inspect.reset(); onCancelReply?.(); }}>{t(($) => $.commentActions.cancelReply)}</Button></div>}
    {parent.error && <CollaborationQueryState error={parent.error} retry={() => void parent.refetch()} />}
    <Textarea ref={textarea} id={fieldId} placeholder={t(($) => $.composer.placeholder)} value={content} disabled={locked} onChange={(event) => change(event.target.value)} className="min-h-28" />
    {mentions.data && <div className="flex min-w-0 flex-wrap gap-2" role="group" aria-label={t(($) => $.composer.mentions)}>
      {mentions.data.participants.map((person) => <Button key={person.user_id} type="button" size="sm" variant="ghost" disabled={locked} className="h-auto min-h-8 whitespace-normal [overflow-wrap:anywhere] pointer-coarse:min-h-11"
        onClick={() => change(`${content}${content ? " " : ""}${collaborationMentionMarkdown("member", person.user_id, person.name, mentions.data!)}`)}>{person.name}</Button>)}
      {mentions.data.targets.map((target) => <Button key={target.target_id} type="button" size="sm" variant="ghost" disabled={locked} className="h-auto min-h-8 whitespace-normal [overflow-wrap:anywhere] pointer-coarse:min-h-11"
        onClick={() => change(`${content}${content ? " " : ""}${collaborationMentionMarkdown(target.kind, target.resource_id, target.name, mentions.data!)}`)}>{target.name}</Button>)}
    </div>}
    {mentions.error && <CollaborationQueryState error={mentions.error} retry={() => void mentions.refetch()} />}
    <CollaborationAttachmentPicker scope={scope} form={delivery.form} update={delivery.update} uploads={delivery.uploads} onBusyChange={setUploading}
      disabled={inspect.isPending || send.isPending || drafts.comments.length > 0 || drafts.error || Boolean(delivery.error)} onChange={() => inspect.reset()} />
    {Boolean(delivery.error) && <p role="alert">{t(($) => $.workflow.storageError)}</p>}
    <div className="flex flex-wrap gap-2"><Button variant="outline" type="button" disabled={locked || !content.trim()} onClick={() => inspect.mutate({ content, parent_id: parentId, suppressed_target_ids: suppressed,
      ...(delivery.form.selectedAttachmentIds.length ? { draft_id: delivery.form.draftId, attachment_ids: delivery.form.selectedAttachmentIds } : {}) })} className="pointer-coarse:min-h-11">{t(($) => $.composer.preview)}</Button>
      <Button type="submit" disabled={!preview || locked || !content.trim()} className="pointer-coarse:min-h-11">{t(($) => $.composer.send)}</Button>
    </div>
    {preview && <div className="space-y-3 text-body"><p className="text-caption text-muted-foreground">{t(($) => $.composer.recipients, { count: preview.recipients.length })}</p>
      <ul className="flex flex-wrap gap-x-4 gap-y-1 text-caption">{preview.recipients.map((person) => <li key={person.recipient.user_id}>{person.recipient.name}</li>)}</ul>
      <ul className="space-y-3">{preview.targets.map((target) => <li key={`${target.kind}:${target.resource_id}`} className="space-y-1 [overflow-wrap:anywhere]">
        <label className="flex min-h-11 items-start gap-3"><Checkbox checked={target.target_id !== null && !suppressed.includes(target.target_id)} disabled={locked || target.target_id === null} onCheckedChange={(checked) => {
          if (!target.target_id) return;
          setSuppressed(checked === true ? suppressed.filter((id) => id !== target.target_id) : [...suppressed, target.target_id]); inspect.reset();
        }} /><span>{targetName(target)}
          <span className="block text-caption text-muted-foreground">{target.state === "suppressed" ? t(($) => $.composer.suppressed) : target.state === "blocked" ? labels.reason(target.code) : labels.reason(target.waiting_reason ?? target.hold_reason ?? "waiting")}</span></span></label>
      </li>)}</ul>
    </div>}
    {(inspect.error || send.error) && <CollaborationQueryState error={inspect.error ?? send.error} />}
    {drafts.error && <p role="alert">{t(($) => $.workflow.storageError)}</p>}
    {(inspect.isPending || send.isPending) && <p role="status">{t(($) => $.workflow.pending)}</p>}
    {send.data && <div role="status" className="space-y-2"><p>{send.data.status < 300 ? t(($) => $.composer.sent) : t(($) => $.workflow.refused)}</p>{send.data.targets.map((target) => <p key={`${target.kind}:${target.resource_id}`} className="text-caption [overflow-wrap:anywhere]">{targetName(target)} · {target.state === "waiting" ? target.code === "command_joined_squad" ? t(($) => $.composer.joined) : labels.reason(target.waiting_reason ?? target.hold_reason ?? "waiting") : target.state === "suppressed" ? t(($) => $.composer.suppressed) : labels.reason(target.code)}</p>)}</div>}
    {!send.isPending && drafts.comments.map((draft) => <div key={draft.requestId} className="space-y-2"><p role="status">{t(($) => $.workflow.unknown)}</p><div className="flex flex-wrap gap-2"><Button type="button" variant="outline" disabled={drafts.error} onClick={() => send.mutate({ draft, recovery: true })}>{t(($) => $.workflow.recover)}</Button><Button type="button" variant="outline" disabled={drafts.error} onClick={() => send.mutate({ draft, recovery: false })}>{t(($) => $.workflow.retryOriginal)}</Button></div></div>)}
  </form>;
}
