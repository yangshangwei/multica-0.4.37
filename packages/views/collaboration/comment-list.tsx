import { useId, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { collaborationAttachmentsOptions, collaborationCommentOptions, collaborationCommentWithinByteLimit, collaborationKeys, executeCollaborationCommentChange, parseCollaborationCommentChangeDraft,
  parseCollaborationDiscussionDraft, prepareCollaborationCommentChange, useCollaborationPendingDrafts,
  type CollaborationComment, type CollaborationDraft, type CollaborationIssue, type CollaborationScope } from "@multica/core/collaboration";
import { accountPath } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { Avatar, AvatarFallback, AvatarImage } from "@multica/ui/components/ui/avatar";
import { useT } from "../i18n";
import { AppLink } from "../navigation";
import { RichContent } from "../rich-content";
import { CollaborationCommentHistory } from "./comment-history";
import { CollaborationCommentAttachments } from "./attachment-panel";
import { CollaborationDiscussionActions } from "./discussion-actions";
import { CollaborationQueryState } from "./query-state";
import { recoverFocus } from "./focus";

type CommentProps = { scope: CollaborationScope; issue: CollaborationIssue; comment: CollaborationComment; onReply: (comment: CollaborationComment) => void };
type Editor = { action: "edit" | "delete"; revision: string; content: string; attachmentIds: string[] };

export function CollaborationCommentList({ scope, issue, comments, onReply }: Omit<CommentProps, "comment"> & { comments: CollaborationComment[] }) {
  const drafts = useCollaborationPendingDrafts({ ...scope, issueId: issue.id });
  const pendingIds = [...new Set([...drafts.commentChanges.map((draft) => parseCollaborationCommentChangeDraft(draft).commentId),
    ...drafts.discussion.flatMap((draft) => { const id = parseCollaborationDiscussionDraft(draft).commentId; return id ? [id] : []; })])].filter((id) => !comments.some((comment) => comment.id === id));
  // A confirmed refusal retires its request, but its editor still owns the
  // user's unsent input. Keep restored comments visible beyond that receipt.
  const [restoredIds, setRestoredIds] = useState(pendingIds);
  if (pendingIds.some((id) => !restoredIds.includes(id))) setRestoredIds([...new Set([...restoredIds, ...pendingIds])]);
  const additionalIds = [...new Set([...restoredIds, ...pendingIds])].filter((id) => !comments.some((comment) => comment.id === id));
  const byId = new Map(comments.map((comment) => [comment.id, comment]));
  return <ol className="space-y-6">
    {[...additionalIds, ...byId.keys()].map((id) => <CommentItem key={id} scope={scope} issue={issue} commentId={id} comment={byId.get(id)} onReply={onReply} />)}
  </ol>;
}

// Keep one component identity when an individually restored comment joins a
// loaded page, so paging cannot discard its unsent editor or recovery state.
function CommentItem({ scope, issue, commentId, comment, onReply }: Omit<CommentProps, "comment"> & { commentId: string; comment?: CollaborationComment }) {
  const { t } = useT("collaboration");
  const client = useQueryClient();
  const query = useQuery({ ...collaborationCommentOptions(client, scope, issue.id, commentId), enabled: comment === undefined });
  const current = comment ?? query.data;
  if (current) return <CommentCard scope={scope} issue={issue} comment={current} onReply={onReply} />;
  if (query.isPending) return <li><p role="status" className="text-caption">{t(($) => $.page.loading)}</p></li>;
  return <li><CollaborationQueryState error={query.error} retry={() => void query.refetch()} /></li>;
}

function CommentCard({ scope, issue, comment, onReply }: CommentProps) {
  const { t } = useT("collaboration");
  const client = useQueryClient();
  const fieldId = useId();
  const drafts = useCollaborationPendingDrafts({ ...scope, issueId: issue.id });
  const pending = drafts.commentChanges.filter((draft) => parseCollaborationCommentChangeDraft(draft).commentId === comment.id);
  const pendingOversized = pending.some((draft) => { const value = parseCollaborationCommentChangeDraft(draft); return value.action === "edit" && !collaborationCommentWithinByteLimit(value.input.content); });
  const [editor, setEditor] = useState<Editor | null>(() => {
    if (!pending[0]) return null;
    const saved = parseCollaborationCommentChangeDraft(pending[0]);
    return { action: saved.action, revision: saved.input.expected_comment_revision, content: saved.action === "edit" ? saved.input.content : comment.content,
      attachmentIds: saved.action === "edit" ? saved.input.attachment_ids ?? comment.attachment_ids : comment.attachment_ids };
  });
  const submitting = useRef(false);
  const item = useRef<HTMLLIElement>(null);
  const editButton = useRef<HTMLButtonElement>(null);
  const editorForm = useRef<HTMLFormElement>(null);
  const returnFocus = () => recoverFocus(() => editButton.current, () => item.current);
  const send = useMutation({ mutationKey: [...collaborationKeys.issue(scope, issue.id), "comment-change"], retry: false, networkMode: "always",
    mutationFn: ({ draft, recovery }: { draft: CollaborationDraft; recovery: boolean }) => executeCollaborationCommentChange(client, scope, draft, recovery),
    onSuccess: (receipt, { draft }) => {
      if (receipt.status >= 300) return;
      setEditor(null);
      if (parseCollaborationCommentChangeDraft(draft).action === "delete") recoverFocus(() => item.current); else returnFocus();
    } });
  const canEdit = issue.capabilities.comment === true && comment.can_edit === true && comment.author_type === "member" && comment.type === "comment" && comment.author?.user_id === scope.userId && comment.deleted_at === null;
  const locked = send.isPending || pending.length > 0 || drafts.error;
  const tooLong = editor?.action === "edit" && !collaborationCommentWithinByteLimit(editor.content);
  const stale = editor !== null && (editor.revision !== comment.comment_revision || (send.data !== undefined && send.data.status >= 400 && send.data.code !== "invalid_comment_request"));
  const author = comment.author?.name ?? (comment.author_type === "agent" ? t(($) => $.execution.agent) : t(($) => $.detail.unknownAuthor));
  function begin(action: Editor["action"]) {
    setEditor({ action, revision: comment.comment_revision, content: comment.content, attachmentIds: [...comment.attachment_ids] }); send.reset();
    recoverFocus(() => editorForm.current);
  }
  function submit() {
    if (!editor || !canEdit || locked || stale || tooLong || submitting.current) return;
    const draft = prepareCollaborationCommentChange(scope, issue, { ...comment, comment_revision: editor.revision }, crypto.randomUUID(),
      editor.action === "edit" ? { action: "edit", content: editor.content, attachmentIds: editor.attachmentIds } : { action: "delete" });
    submitting.current = true;
    void send.mutateAsync({ draft, recovery: false }).catch(() => {}).finally(() => { submitting.current = false; });
  }
  return <li ref={item} id={`comment-${comment.id}`} tabIndex={-1} className="min-w-0 space-y-3 rounded-md py-2 outline-none focus-visible:ring-2 focus-visible:ring-ring target:bg-accent">
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-caption text-muted-foreground">
      <Avatar className="size-6"><AvatarImage src={comment.author?.avatar_url || undefined} alt="" /><AvatarFallback>{author.slice(0, 1)}</AvatarFallback></Avatar>
      <span className="font-medium text-foreground [overflow-wrap:anywhere]">{author}</span>
      {comment.author?.external === true && <span>{t(($) => $.detail.external)}</span>}
      <time dateTime={comment.created_at}>{new Date(comment.created_at).toLocaleString()}</time>
      {comment.resolved_at !== null && <span>{comment.resolved_by ? t(($) => $.commentActions.resolvedBy, { name: comment.resolved_by.name }) : t(($) => $.detail.resolved)}</span>}
    </div>
    {comment.parent_id && <AppLink href={accountPath({ view: "participating", issueId: issue.id, commentId: comment.parent_id })} className="inline-block py-1 text-caption text-muted-foreground underline underline-offset-2 pointer-coarse:min-h-11">{t(($) => $.commentActions.parent)}</AppLink>}
    {comment.deleted_at !== null ? <p className="text-body text-muted-foreground">{t(($) => $.detail.deleted)}</p> : <RichContent resourcePolicy="restricted" content={comment.content} />}
    {comment.deleted_at === null && <CollaborationCommentAttachments scope={scope} issueId={issue.id} attachmentIds={comment.attachment_ids} />}
    <div className="flex flex-wrap gap-2">
      {issue.capabilities.comment === true && <Button type="button" variant="ghost" size="sm" className="pointer-coarse:min-h-11" disabled={drafts.comments.length > 0 || drafts.error} onClick={() => onReply(comment)}>{t(($) => $.commentActions.reply)}</Button>}
      {canEdit && !editor && <><Button ref={editButton} type="button" variant="ghost" size="sm" className="pointer-coarse:min-h-11" disabled={locked} onClick={() => begin("edit")}>{t(($) => $.commentActions.edit)}</Button>
        <Button type="button" variant="ghost" size="sm" className="pointer-coarse:min-h-11" disabled={locked} onClick={() => begin("delete")}>{t(($) => $.commentActions.delete)}</Button></>}
    </div>
    {editor && <form ref={editorForm} tabIndex={-1} aria-label={editor.action === "edit" ? t(($) => $.commentActions.edit) : t(($) => $.commentActions.delete)} className="space-y-3 rounded-md border border-border p-4 outline-none focus-visible:ring-2 focus-visible:ring-ring" onSubmit={(event) => { event.preventDefault(); submit(); }}>
      {editor.action === "edit" ? <>
        <label htmlFor={fieldId} className="block text-body font-medium">{t(($) => $.commentActions.editText)}</label>
        <Textarea id={fieldId} autoFocus value={editor.content} disabled={locked || !canEdit} aria-invalid={tooLong} aria-describedby={tooLong ? `${fieldId}-limit` : undefined} onChange={(event) => setEditor({ ...editor, content: event.target.value })} className="min-h-28" />
        {tooLong && <p id={`${fieldId}-limit`} role="alert" className="text-body text-destructive">{t(($) => $.commentActions.tooLong)}</p>}
        {pendingOversized && <p className="text-caption text-muted-foreground">{t(($) => $.commentActions.oversizedRecovery)}</p>}
        <CommentAttachmentSelection scope={scope} issue={issue} selected={editor.attachmentIds} disabled={locked || !canEdit} onChange={(attachmentIds) => setEditor({ ...editor, attachmentIds })} />
      </> : <p className="text-body">{t(($) => $.commentActions.deleteHint)}</p>}
      {stale && <div className="space-y-2"><p role="alert" className="text-body">{t(($) => $.commentActions.conflict)}</p>
        <Button type="button" variant="outline" className="pointer-coarse:min-h-11" disabled={locked || !canEdit} onClick={() => { setEditor({ ...editor, revision: comment.comment_revision }); send.reset(); }}>{t(($) => $.commentActions.useLatest)}</Button></div>}
      <div className="flex flex-wrap gap-2"><Button type="submit" variant={editor.action === "delete" ? "destructive" : "default"} disabled={locked || stale || tooLong || !canEdit || (editor.action === "edit" && !editor.content.trim() && editor.attachmentIds.length === 0)} className="pointer-coarse:min-h-11">{editor.action === "edit" ? t(($) => $.commentActions.save) : t(($) => $.commentActions.confirmDelete)}</Button>
        <Button type="button" variant="outline" disabled={locked} className="pointer-coarse:min-h-11" onClick={() => { setEditor(null); send.reset(); returnFocus(); }}>{t(($) => $.workflow.dismiss)}</Button></div>
    </form>}
    {send.isPending && <p role="status" className="text-caption">{t(($) => $.workflow.pending)}</p>}
    {send.error && <CollaborationQueryState error={send.error} />}
    {send.data && send.data.status < 300 && <p role="status" className="text-caption">{t(($) => $.commentActions.saved)}</p>}
    {pending.map((draft) => <div key={draft.requestId} className="space-y-2"><p role="status" className="text-caption">{t(($) => $.workflow.unknown)}</p>
      <div className="flex flex-wrap gap-2"><Button type="button" variant="outline" className="pointer-coarse:min-h-11" disabled={send.isPending || drafts.error} onClick={() => send.mutate({ draft, recovery: true })}>{t(($) => $.workflow.recover)}</Button>
        <Button type="button" variant="outline" className="pointer-coarse:min-h-11" disabled={send.isPending || drafts.error} onClick={() => send.mutate({ draft, recovery: false })}>{t(($) => $.workflow.retryOriginal)}</Button></div>
    </div>)}
    <CollaborationDiscussionActions scope={scope} issue={issue} comment={comment} />
    <CollaborationCommentHistory scope={scope} issueId={issue.id} commentId={comment.id} />
  </li>;
}

function CommentAttachmentSelection({ scope, issue, selected, disabled, onChange }: { scope: CollaborationScope; issue: CollaborationIssue; selected: string[]; disabled: boolean; onChange: (ids: string[]) => void }) {
  const { t } = useT("collaboration");
  const client = useQueryClient();
  const query = useQuery(collaborationAttachmentsOptions(client, scope, issue.id));
  return <fieldset disabled={disabled} className="space-y-2"><legend className="text-caption font-medium">{t(($) => $.attachments.title)}</legend>
    <p className="text-caption text-muted-foreground">{t(($) => $.attachments.retained)}</p>
    {query.error && <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />}
    {query.data?.attachments.map((file) => <label key={file.id} className="flex min-h-9 items-center gap-3 text-body [overflow-wrap:anywhere] pointer-coarse:min-h-11">
      <Checkbox checked={selected.includes(file.id)} onCheckedChange={(checked) => onChange(checked === true ? [...selected, file.id] : selected.filter((id) => id !== file.id))} />{file.name}
    </label>)}
  </fieldset>;
}
