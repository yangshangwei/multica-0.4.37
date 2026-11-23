import { useId, useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { collaborationAttachmentsOptions, collaborationCommentOptions, collaborationCommentWithinByteLimit, collaborationKeys, deriveCollaborationCommentThreads, executeCollaborationCommentChange, parseCollaborationCommentChangeDraft,
  parseCollaborationDiscussionDraft, prepareCollaborationCommentChange, useCollaborationPendingDrafts,
  type CollaborationComment, type CollaborationCommentThread, type CollaborationDraft, type CollaborationIssue, type CollaborationScope } from "@multica/core/collaboration";
import { accountPath } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { Avatar, AvatarFallback, AvatarImage } from "@multica/ui/components/ui/avatar";
import { DropdownMenu, DropdownMenuContent, DropdownMenuGroup, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from "@multica/ui/components/ui/dropdown-menu";
import { CheckCircle2, ChevronDown, ChevronRight, MoreHorizontal } from "lucide-react";
import { useLocale, useT, useTimeAgo } from "../i18n";
import { AppLink } from "../navigation";
import { RichContent } from "../rich-content";
import { CollaborationCommentHistory } from "./comment-history";
import { CollaborationCommentAttachments } from "./attachment-panel";
import { CollaborationReactionButtons, CollaborationResolveButton, useCollaborationDiscussion } from "./discussion-actions";
import { CollaborationQueryState } from "./query-state";
import { recoverFocus } from "./focus";
import { useCollaborationLabels } from "./labels";
import { collaborationCommentAuthor } from "./comment-author";

type CommentProps = { scope: CollaborationScope; issue: CollaborationIssue; comment: CollaborationComment; onReply: (comment: CollaborationComment) => void;
  onPrepareResult?: (comment: CollaborationComment) => void;
  onLoadContent?: () => void; contentLoading?: boolean; contentError?: unknown; contentStale?: boolean };
type Editor = { action: "edit" | "delete"; revision: string; content: string; attachmentIds: string[] };
// Thread position of one row. The list stays flat, so indentation, the rail
// and folding are row attributes rather than nested lists.
type Placement = { depth: number; pending: boolean; start: boolean; hidden: boolean; parentLoaded: boolean; resolveHost: boolean };
const standalone: Placement = { depth: 0, pending: false, start: true, hidden: false, parentLoaded: false, resolveHost: true };
const indent = ["pl-3", "pl-8", "pl-12", "pl-16"];
const rowClass = (placement: Placement) => `min-w-0 border-l-2 ${placement.pending ? "border-muted-foreground" : "border-border"} ${indent[Math.min(placement.depth, 3)]} ${placement.start ? "mt-6 first:mt-0" : ""}`;
const week = 7 * 24 * 60 * 60 * 1000;

export function CollaborationCommentList({ scope, issue, comments, threads: suppliedThreads, allLoaded = false, revealCommentId = null, revealKey, onReply, onPrepareResult }: Omit<CommentProps, "comment"> & {
  comments: CollaborationComment[];
  /** Threads derived from `comments` by the owner that also needs them; derived here when omitted. */
  threads?: CollaborationCommentThread[];
  /** True only when every comment page is loaded; thread sizes are stated only then. */
  allLoaded?: boolean;
  /** Explicit navigation target; its folded thread opens before the owner focuses it. */
  revealCommentId?: string | null; revealKey?: number | string;
}) {
  const { t } = useT("collaboration");
  const drafts = useCollaborationPendingDrafts({ ...scope, issueId: issue.id });
  const draftIds = [...new Set([...drafts.commentChanges.map((draft) => parseCollaborationCommentChangeDraft(draft).commentId),
    ...drafts.discussion.flatMap((draft) => { const id = parseCollaborationDiscussionDraft(draft).commentId; return id ? [id] : []; })])];
  const pendingIds = draftIds.filter((id) => !comments.some((comment) => comment.id === id));
  // A confirmed refusal retires its request, but its editor still owns the
  // user's unsent input. Keep restored comments visible beyond that receipt.
  const [restoredIds, setRestoredIds] = useState(pendingIds);
  if (pendingIds.some((id) => !restoredIds.includes(id))) setRestoredIds([...new Set([...restoredIds, ...pendingIds])]);
  const additionalIds = [...new Set([...restoredIds, ...pendingIds])].filter((id) => !comments.some((comment) => comment.id === id));
  const derived = useMemo(() => suppliedThreads ?? deriveCollaborationCommentThreads(comments), [suppliedThreads, comments]);
  // Expansion is remembered per thread. A thread starts folded only when it is
  // already resolved on first sight; one resolved while the page is open stays
  // open so its feedback and Reopen control do not vanish. Unresolved threads
  // and threads with an unconfirmed request are always open, and that opening
  // is recorded so a settled recovery keeps its result and editor in view.
  // Loading an older page can move visible replies under a newly loaded root;
  // they keep the expansion they were shown with instead of folding away.
  const [fold, setFold] = useState<{ open: Map<string, boolean>; threadOf: Map<string, string>; revealed: string | null }>({ open: new Map(), threadOf: new Map(), revealed: null });
  let open = fold.open;
  let threadOf = fold.threadOf;
  let revealed = fold.revealed;
  const expand = (id: string, value: boolean) => {
    if (open.get(id) === value) return;
    if (open === fold.open) open = new Map(open);
    open.set(id, value);
  };
  const needsAttention = (thread: CollaborationCommentThread) => thread.items.some((item) => draftIds.includes(item.comment.id));
  for (const thread of derived) {
    let shownOpen = false;
    for (const { comment } of thread.items) {
      const previous = threadOf.get(comment.id);
      if (previous === thread.id) continue;
      if (previous !== undefined && open.get(previous) === true || restoredIds.includes(comment.id)) shownOpen = true;
      if (threadOf === fold.threadOf) threadOf = new Map(threadOf);
      threadOf.set(comment.id, thread.id);
    }
    const mustOpen = thread.resolvedBy === null || needsAttention(thread) || shownOpen;
    if (open.get(thread.id) === undefined) expand(thread.id, mustOpen);
    else if (mustOpen) expand(thread.id, true);
  }
  const intent = revealCommentId ? `${revealKey ?? 0}:${revealCommentId}` : null;
  if (intent !== revealed) {
    const target = derived.find((thread) => thread.items.some((item) => item.comment.id === revealCommentId));
    if (target) expand(target.id, true);
    // An unloaded target keeps its intent open until its comment arrives.
    if (intent === null || target) revealed = intent;
  }
  if (open !== fold.open || threadOf !== fold.threadOf || revealed !== fold.revealed) setFold({ open, threadOf, revealed });
  const current = { open, threadOf, revealed };
  const loaded = new Set(comments.map((comment) => comment.id));
  const card = { scope, issue, onReply, onPrepareResult };
  return <ol className="min-w-0">
    {/* One keyed array: a restored row that later joins a loaded thread must keep its identity. */}
    {[...additionalIds.map((id) => <CommentItem key={id} {...card} commentId={id} placement={standalone} />), ...derived.flatMap((thread) => {
      const resolved = thread.resolvedBy !== null;
      const expanded = current.open.get(thread.id) !== false;
      const bar = resolved ? [<li key={`thread:${thread.id}`} data-thread-fold={thread.id} className={rowClass({ ...standalone, start: true })}>
        <button type="button" aria-expanded={expanded} disabled={needsAttention(thread)} onClick={() => setFold({ ...current, open: new Map(current.open).set(thread.id, !expanded) })}
          className="flex min-h-8 w-full min-w-0 flex-wrap items-center gap-x-2 gap-y-1 rounded-md py-1 text-left text-caption text-muted-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none pointer-coarse:min-h-11">
          {expanded ? <ChevronDown className="size-4 shrink-0" aria-hidden /> : <ChevronRight className="size-4 shrink-0" aria-hidden />}
          <CheckCircle2 className="size-4 shrink-0" aria-hidden />
          <span className="min-w-0 [overflow-wrap:anywhere]">{thread.resolvedBy?.resolved_by ? t(($) => $.commentActions.resolvedBy, { name: thread.resolvedBy.resolved_by.name }) : t(($) => $.detail.resolved)}</span>
          {allLoaded && <span className="shrink-0">{t(($) => $.detail.threadComments, { count: thread.items.length })}</span>}
          <span className="ml-auto shrink-0 underline underline-offset-2">{expanded ? t(($) => $.detail.collapseThread) : t(($) => $.detail.expandThread)}</span>
        </button>
      </li>] : [];
      return [...bar, ...thread.items.map(({ comment, depth }, index) => <CommentItem key={comment.id} {...card} commentId={comment.id} comment={comment}
        placement={{ depth, pending: thread.pending, start: index === 0 && !resolved, hidden: !expanded,
          parentLoaded: comment.parent_id !== null && loaded.has(comment.parent_id),
          resolveHost: thread.resolveTarget?.id === comment.id || comment.resolved_at !== null }} />)];
    })]}
  </ol>;
}

// Keep one component identity when an individually restored comment joins a
// loaded page, so paging cannot discard its unsent editor or recovery state.
function CommentItem({ scope, issue, commentId, comment, placement, onReply, onPrepareResult }: Omit<CommentProps, "comment"> & { commentId: string; comment?: CollaborationComment; placement: Placement }) {
  const { t } = useT("collaboration");
  const client = useQueryClient();
  const [requestedRevision, setRequestedRevision] = useState<string | null>(null);
  const omitted = comment?.content_state === "omitted";
  const query = useQuery({ ...collaborationCommentOptions(client, scope, issue.id, commentId),
    enabled: comment === undefined || omitted && requestedRevision === comment.comment_revision });
  const restored = omitted && requestedRevision === comment.comment_revision && query.data?.comment_revision === comment.comment_revision && query.data.content_state !== "omitted";
  const current = restored ? query.data : comment ?? query.data;
  if (current) return <CommentCard scope={scope} issue={issue} comment={current} placement={placement} onReply={onReply} onPrepareResult={onPrepareResult}
    contentLoading={query.isFetching} contentError={requestedRevision ? query.error : undefined}
    contentStale={omitted && requestedRevision === comment.comment_revision && !!query.data && query.data.comment_revision !== comment.comment_revision}
    onLoadContent={() => {
      setRequestedRevision(comment?.comment_revision ?? null);
      void query.refetch();
      if (query.data && query.data.comment_revision !== comment?.comment_revision) void client.invalidateQueries({ queryKey: [...collaborationKeys.issue(scope, issue.id), "comments"] });
    }} />;
  if (query.isPending) return <li hidden={placement.hidden} className={`${rowClass(placement)} py-2`}><p role="status" className="text-caption">{t(($) => $.page.loading)}</p></li>;
  return <li hidden={placement.hidden} className={`${rowClass(placement)} py-2`}><CollaborationQueryState error={query.error} retry={() => void query.refetch()} /></li>;
}

function CommentCard({ scope, issue, comment, placement, onReply, onPrepareResult, onLoadContent, contentLoading, contentError, contentStale }: CommentProps & { placement: Placement }) {
  const { t } = useT("collaboration");
  const locale = useLocale();
  const timeAgo = useTimeAgo();
  const labels = useCollaborationLabels();
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
  const [historyOpen, setHistoryOpen] = useState(false);
  const item = useRef<HTMLLIElement>(null);
  const menuTrigger = useRef<HTMLButtonElement>(null);
  const editorForm = useRef<HTMLFormElement>(null);
  // What the closing menu owes its chosen item: focus for the inline form, or
  // a step that must wait until the trigger holds focus again.
  const afterMenu = useRef<"form" | (() => void) | null>(null);
  const discussion = useCollaborationDiscussion(scope, issue, comment);
  const returnFocus = () => recoverFocus(() => menuTrigger.current, () => item.current);
  const send = useMutation({ mutationKey: [...collaborationKeys.issue(scope, issue.id), "comment-change"], retry: false, networkMode: "always",
    mutationFn: ({ draft, recovery }: { draft: CollaborationDraft; recovery: boolean }) => executeCollaborationCommentChange(client, scope, draft, recovery),
    onSuccess: (receipt, { draft }) => {
      if (receipt.status >= 300) return;
      setEditor(null);
      if (parseCollaborationCommentChangeDraft(draft).action === "delete") recoverFocus(() => item.current); else returnFocus();
    } });
  const omitted = comment.content_state === "omitted";
  const canEdit = !omitted && issue.capabilities.comment === true && comment.can_edit === true && comment.author_type === "member" && comment.type === "comment" && comment.author?.user_id === scope.userId && comment.deleted_at === null;
  const locked = send.isPending || pending.length > 0 || drafts.error;
  const tooLong = editor?.action === "edit" && !collaborationCommentWithinByteLimit(editor.content);
  const stale = editor !== null && (editor.revision !== comment.comment_revision || (send.data !== undefined && send.data.status >= 400 && send.data.code !== "invalid_comment_request"));
  const author = collaborationCommentAuthor(comment, { agent: t(($) => $.execution.agent), unknownAuthor: t(($) => $.detail.unknownAuthor) });
  function begin(action: Editor["action"]) {
    afterMenu.current = "form";
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
  const deleted = comment.deleted_at !== null;
  const created = new Date(comment.created_at);
  // Only an edit moves updated_at on a comment that is not deleted.
  const edited = !deleted && new Date(comment.updated_at).getTime() > created.getTime();
  const canReply = issue.capabilities.comment === true;
  const replyDisabled = drafts.comments.length > 0 || drafts.error;
  const resolveVisible = discussion.canResolve && placement.resolveHost;
  const itemClass = "max-w-80 whitespace-normal pointer-coarse:min-h-11";
  const menu = <DropdownMenu>
    <DropdownMenuTrigger ref={menuTrigger} render={<Button type="button" variant="ghost" size="icon-sm" aria-label={t(($) => $.commentActions.more)} className="shrink-0 text-muted-foreground pointer-coarse:min-h-11 pointer-coarse:min-w-11" />}><MoreHorizontal aria-hidden /></DropdownMenuTrigger>
    <DropdownMenuContent align="end" finalFocus={() => {
      const next = afterMenu.current;
      afterMenu.current = null;
      if (next === "form") {
        // Chromium reports the chosen item's removal as focus loss, so the
        // menu holds focus until its exit animation ends. Hand it to the form
        // only now; a field that already took focus is left alone.
        recoverFocus(() => editorForm.current);
        return false;
      }
      // Runs after the menu has returned focus to the trigger below.
      if (next) setTimeout(next);
      return menuTrigger.current;
    }}>
      {deleted && canReply && <DropdownMenuItem className={itemClass} disabled={replyDisabled} onClick={() => onReply(comment)}>{t(($) => $.commentActions.reply)}</DropdownMenuItem>}
      {canEdit && !editor && <><DropdownMenuItem className={itemClass} disabled={locked} onClick={() => begin("edit")}>{t(($) => $.commentActions.edit)}</DropdownMenuItem>
        <DropdownMenuItem className={itemClass} variant="destructive" disabled={locked} onClick={() => begin("delete")}>{t(($) => $.commentActions.delete)}</DropdownMenuItem></>}
      <DropdownMenuItem className={itemClass} onClick={() => setHistoryOpen(true)}>{t(($) => $.detail.commentHistory)}</DropdownMenuItem>
      {onPrepareResult && !omitted && comment.deleted_at === null && comment.content.trim() && <DropdownMenuItem className={itemClass} disabled={drafts.actions.length > 0 || drafts.error} onClick={() => {
        // The result dialog returns focus to whatever is focused when it
        // opens. This item unmounts with the menu and the menu re-focuses it
        // on click, so prepare the result once the trigger has focus again.
        afterMenu.current = () => onPrepareResult(comment);
      }}>{t(($) => $.delivery.useComment)}</DropdownMenuItem>}
      {discussion.canResolve && !placement.resolveHost && <DropdownMenuItem className={itemClass} disabled={discussion.locked}
        onClick={() => discussion.submit({ action: comment.resolved_at === null ? "resolve" : "unresolve", commentId: comment.id })}>
        {comment.resolved_at === null ? t(($) => $.commentActions.resolve) : t(($) => $.commentActions.unresolve)}
      </DropdownMenuItem>}
      {comment.author_authority && <><DropdownMenuSeparator />
        <DropdownMenuGroup><DropdownMenuLabel className="max-w-80 space-y-1 whitespace-normal font-normal">
          <span className="block font-medium">{t(($) => $.participants.permissionContext)}</span>
          <span className="block">{labels.commentAuthorRole(comment.author_authority.role)}</span>
          <span className="block">{comment.author_authority.can_submit_command === true ? t(($) => $.recall.authorCanSubmit) : t(($) => $.recall.authorCannotSubmit)}</span>
        </DropdownMenuLabel></DropdownMenuGroup></>}
    </DropdownMenuContent>
  </DropdownMenu>;
  return <li ref={item} id={`comment-${comment.id}`} tabIndex={-1} hidden={placement.hidden} data-depth={placement.depth} className={`${rowClass(placement)} space-y-2 rounded-r-md ${deleted ? "py-1.5" : "py-3"} outline-none focus-visible:ring-2 focus-visible:ring-ring target:bg-accent`}>
    <div className="flex min-w-0 items-start gap-2">
      <div className="flex min-w-0 flex-1 flex-wrap items-center gap-x-2 gap-y-1 text-caption text-muted-foreground">
        <Avatar className={deleted ? "size-5" : "size-6"}><AvatarImage src={comment.author?.avatar_url || undefined} alt="" /><AvatarFallback>{author.name.slice(0, 1)}</AvatarFallback></Avatar>
        <span className={`min-w-0 font-medium [overflow-wrap:anywhere] ${deleted ? "" : "text-foreground"}`}>{author.name}</span>
        {author.agentLabel && <span className="shrink-0 rounded bg-muted px-1.5 py-0.5">{author.agentLabel}</span>}
        {comment.author?.external === true && <span>{t(($) => $.detail.external)}</span>}
        {deleted && <span>{t(($) => $.detail.deleted)}</span>}
        <time dateTime={comment.created_at} title={created.toLocaleString(locale)}>{Date.now() - created.getTime() < week ? timeAgo(comment.created_at) : created.toLocaleDateString(locale)}</time>
        {edited && <button type="button" aria-expanded={historyOpen} onClick={() => setHistoryOpen(true)}
          className="inline-flex items-center rounded underline underline-offset-2 outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring pointer-coarse:min-h-11">{t(($) => $.commentActions.edited)}</button>}
        {comment.resolved_at !== null && <span className="[overflow-wrap:anywhere]">{comment.resolved_by ? t(($) => $.commentActions.resolvedBy, { name: comment.resolved_by.name }) : t(($) => $.detail.resolved)}</span>}
        {deleted && (discussion.reactions.length > 0 || resolveVisible) && <span role="group" aria-label={t(($) => $.commentActions.discussion)} className="inline-flex min-w-0 flex-wrap items-center gap-1">
          <CollaborationReactionButtons discussion={discussion} comment={comment} />
          {resolveVisible && <CollaborationResolveButton discussion={discussion} comment={comment} />}
        </span>}
      </div>
      {menu}
    </div>
    {comment.parent_id && !placement.parentLoaded && <AppLink href={accountPath({ view: "participating", issueId: issue.id, commentId: comment.parent_id })} className="inline-block py-1 text-caption text-muted-foreground underline underline-offset-2 pointer-coarse:min-h-11">{t(($) => $.commentActions.parent)}</AppLink>}
    {comment.deleted_at !== null ? null : omitted
      ? <div className="space-y-2"><p className="text-body text-muted-foreground">{t(($) => $.recall.omitted)}</p>
        {contentStale && <p role="status" className="text-caption">{t(($) => $.recall.revisionChanged)}</p>}
        {!!contentError && <CollaborationQueryState error={contentError} />}
        <Button type="button" variant="outline" className="pointer-coarse:min-h-11" disabled={contentLoading} onClick={onLoadContent}>{contentLoading ? t(($) => $.page.loading) : contentStale ? t(($) => $.recall.refresh) : t(($) => $.recall.loadContent)}</Button>
      </div> : <RichContent resourcePolicy="restricted" content={comment.content} />}
    {comment.deleted_at === null && !omitted && <CollaborationCommentAttachments scope={scope} issueId={issue.id} attachmentIds={comment.attachment_ids} />}
    {!deleted && <div role="group" aria-label={t(($) => $.commentActions.discussion)} className="flex min-w-0 flex-wrap items-center gap-1">
      {canReply && <Button type="button" variant="ghost" size="sm" className="font-normal text-muted-foreground pointer-coarse:min-h-11" disabled={replyDisabled} onClick={() => onReply(comment)}>{t(($) => $.commentActions.reply)}</Button>}
      <CollaborationReactionButtons discussion={discussion} comment={comment} />
      {resolveVisible && <span className="ml-auto min-w-0"><CollaborationResolveButton discussion={discussion} comment={comment} /></span>}
    </div>}
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
    {discussion.status}
    <CollaborationCommentHistory scope={scope} issueId={issue.id} commentId={comment.id} open={historyOpen} onClose={() => { setHistoryOpen(false); returnFocus(); }} />
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
