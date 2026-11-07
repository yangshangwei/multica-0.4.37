import { useId, useRef } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { collaborationIssueReactionsOptions, collaborationKeys, executeCollaborationDiscussion, parseCollaborationDiscussionDraft, prepareCollaborationDiscussion,
  useCollaborationPendingDrafts, type CollaborationComment, type CollaborationDiscussionSelection, type CollaborationDraft, type CollaborationIssue, type CollaborationScope } from "@multica/core/collaboration";
import { Button } from "@multica/ui/components/ui/button";
import { QuickEmojiPicker } from "@multica/ui/components/common/quick-emoji-picker";
import { useT } from "../i18n";
import { CollaborationQueryState } from "./query-state";

export function CollaborationDiscussionActions({ scope, issue, comment, reactions = [] }: {
  scope: CollaborationScope; issue: CollaborationIssue; comment?: CollaborationComment; reactions?: CollaborationComment["reactions"];
}) {
  const { t } = useT("collaboration");
  const countId = useId();
  const client = useQueryClient();
  const drafts = useCollaborationPendingDrafts({ ...scope, issueId: issue.id });
  const pending = drafts.discussion.filter((draft) => parseCollaborationDiscussionDraft(draft).commentId === (comment?.id ?? null));
  const submitting = useRef(false);
  const send = useMutation({ mutationKey: [...collaborationKeys.issue(scope, issue.id), "discussion-send"], retry: false, networkMode: "always",
    mutationFn: ({ draft, recovery }: { draft: CollaborationDraft; recovery: boolean }) => executeCollaborationDiscussion(client, scope, draft, recovery) });
  const locked = send.isPending || pending.length > 0 || drafts.error;
  const canReact = issue.capabilities.comment === true && issue.capabilities.react === true;
  const canResolve = issue.capabilities.comment === true && issue.capabilities.resolve_thread === true && comment?.can_resolve === true;
  const currentReactions = comment?.reactions ?? reactions;
  function submit(selection: CollaborationDiscussionSelection) {
    if (locked || submitting.current) return;
    const draft = prepareCollaborationDiscussion(scope, issue, crypto.randomUUID(), selection);
    submitting.current = true;
    void send.mutateAsync({ draft, recovery: false }).catch(() => {}).finally(() => { submitting.current = false; });
  }
  function toggle(emoji: string) {
    if (!canReact) return;
    const own = currentReactions.some((reaction) => reaction.emoji === emoji && reaction.viewer_reacted);
    if (comment?.deleted_at && !own) return;
    submit({ action: own ? "remove_reaction" : "add_reaction", commentId: comment?.id ?? null, emoji });
  }
  return <div className="min-w-0 space-y-2">
    <div role="group" aria-label={t(($) => $.commentActions.discussion)} className="flex flex-wrap items-center gap-2">
      {canResolve && comment && <Button type="button" variant="ghost" size="sm" disabled={locked} className="h-auto min-h-8 whitespace-normal pointer-coarse:min-h-11"
        onClick={() => submit({ action: comment.resolved_at === null ? "resolve" : "unresolve", commentId: comment.id })}>
        {comment.resolved_at === null ? t(($) => $.commentActions.resolve) : t(($) => $.commentActions.unresolve)}
      </Button>}
      {currentReactions.map((reaction, index) => <Button key={reaction.emoji} type="button" variant={reaction.viewer_reacted ? "secondary" : "ghost"} size="sm"
        disabled={locked || !canReact || Boolean(comment?.deleted_at && !reaction.viewer_reacted)} aria-pressed={reaction.viewer_reacted} aria-describedby={`${countId}-${index}`}
        aria-label={reaction.viewer_reacted ? t(($) => $.commentActions.removeReaction, { emoji: reaction.emoji }) : t(($) => $.commentActions.addReaction, { emoji: reaction.emoji })}
        className={`h-auto min-h-8 gap-2 rounded-full pointer-coarse:min-h-11 ${reaction.viewer_reacted ? "font-semibold" : ""}`} onClick={() => toggle(reaction.emoji)}>
        <span>{reaction.emoji}</span><span id={`${countId}-${index}`} className="tabular-nums">{reaction.count}</span>
      </Button>)}
      {canReact && !comment?.deleted_at && <QuickEmojiPicker onSelect={toggle} disabled={locked} label={t(($) => $.commentActions.chooseReaction)}
        moreLabel={t(($) => $.commentActions.moreEmojis)} loadingLabel={t(($) => $.page.loading)} className="min-h-8 min-w-8 pointer-coarse:min-h-11 pointer-coarse:min-w-11" />}
    </div>
    {send.isPending && <p role="status" className="text-caption">{t(($) => $.workflow.pending)}</p>}
    {send.error && <CollaborationQueryState error={send.error} />}
    {send.data && <p role="status" className="text-caption">{send.data.status === 200 ? t(($) => $.commentActions.updated) : t(($) => $.workflow.refused)}</p>}
    {pending.map((draft) => <div key={draft.requestId} className="space-y-2"><p role="status" className="text-caption">{t(($) => $.workflow.unknown)}</p>
      <div className="flex flex-wrap gap-2"><Button type="button" variant="outline" className="pointer-coarse:min-h-11" disabled={send.isPending || drafts.error} onClick={() => send.mutate({ draft, recovery: true })}>{t(($) => $.workflow.recover)}</Button>
        <Button type="button" variant="outline" className="pointer-coarse:min-h-11" disabled={send.isPending || drafts.error} onClick={() => send.mutate({ draft, recovery: false })}>{t(($) => $.workflow.retryOriginal)}</Button></div>
    </div>)}
  </div>;
}

export function CollaborationIssueReactions({ scope, issue }: { scope: CollaborationScope; issue: CollaborationIssue }) {
  const { t } = useT("collaboration");
  const client = useQueryClient();
  const query = useQuery(collaborationIssueReactionsOptions(client, scope, issue.id));
  return <section aria-label={t(($) => $.commentActions.issueReactions)} className="space-y-2">
    {query.isPending && <p role="status" className="text-caption">{t(($) => $.page.loading)}</p>}
    {query.error && <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />}
    {query.data && <CollaborationDiscussionActions scope={scope} issue={issue} reactions={query.data.items} />}
  </section>;
}
