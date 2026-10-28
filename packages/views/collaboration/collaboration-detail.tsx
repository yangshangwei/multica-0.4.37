import { useEffect, useRef, useState } from "react";
import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";
import { collaborationCommentsOptions, collaborationCommentOptions, collaborationIssueAccessDenied, isCollaborationDenied, useCollaborationIssue, type CollaborationComment, type CollaborationScope } from "@multica/core/collaboration";
import { accountPath, type AccountDestination } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { copyText } from "@multica/ui/lib/clipboard";
import { Link2 } from "lucide-react";
import { toast } from "sonner";
import { RichContent } from "../rich-content";
import { useT } from "../i18n";
import { useCollaborationLabels } from "./labels";
import { CollaborationQueryState } from "./query-state";
import { CollaborationWorkflow } from "./workflow";
import { CollaborationExecutionPanel } from "./execution-panel";
import { CollaborationCommentComposer } from "./comment-composer";
import { CollaborationActivity } from "./activity";
import { CollaborationCommentList } from "./comment-list";
import { CollaborationIssueReactions } from "./discussion-actions";
import { CollaborationAttachmentPanel } from "./attachment-panel";
import { CollaborationFieldSummary } from "./field-summary";
import { CollaborationChildren } from "./children";
import { CollaborationMemoryPanel } from "./memory-panel";
import { useRestoredScrollRef } from "../platform";
import { useNavigation } from "../navigation";

export function CollaborationDetail({ scope, destination }: { scope: CollaborationScope; destination: AccountDestination }) {
  const { t } = useT("collaboration");
  const { t: tIssues } = useT("issues");
  const navigation = useNavigation();
  const labels = useCollaborationLabels();
  const client = useQueryClient();
  const issueId = destination.issueId!;
  const query = useCollaborationIssue(scope, issueId);
  const comments = useInfiniteQuery({ ...collaborationCommentsOptions(client, scope, issueId), enabled: query.data?.capabilities.read === true });
  const linkedComment = useQuery({ ...collaborationCommentOptions(client, scope, issueId, destination.commentId ?? ""),
    enabled: query.data?.capabilities.read === true && destination.commentId !== null && !comments.data?.pages.some((page) => page.items.some((comment) => comment.id === destination.commentId)) });
  const root = useRef<HTMLDivElement>(null);
  const [replyTo, setReplyTo] = useState<CollaborationComment | null>(null);
  const consumedAnchor = useRef<string | null>(null);
  const restored = useRef(false);
  const restoreScroll = useRestoredScrollRef("account-detail");
  useEffect(() => {
    if (!query.data || !comments.data || restored.current) return;
    if (!navigation.isScopeResume && (destination.commentId || destination.runId || destination.anchor)) return;
    restoreScroll(root.current?.closest<HTMLElement>('[data-tab-scroll-root="account-detail"]') ?? null);
    restored.current = true;
  }, [query.data, comments.data, restoreScroll, destination.commentId, destination.runId, destination.anchor, navigation.isScopeResume]);
  useEffect(() => {
    const anchor = destination.runId ? `run-${destination.runId}` : destination.commentId ? `comment-${destination.commentId}` : destination.anchor;
    if (!anchor || navigation.isScopeResume) { consumedAnchor.current = null; return; }
    const intent = `${navigation.navigationKey ?? 0}:${anchor}`;
    if (consumedAnchor.current === intent) return;
    const element = root.current?.querySelector<HTMLElement>(`[id="${anchor}"]`);
    const container = element?.closest<HTMLElement>('[data-tab-scroll-root="account-detail"]');
    if (!element || !container) return;
    container.scrollTop += element.getBoundingClientRect().top - container.getBoundingClientRect().top;
    element.focus({ preventScroll: true });
    consumedAnchor.current = intent;
  }, [destination.commentId, destination.runId, destination.anchor, query.data, comments.data, linkedComment.data, navigation.isScopeResume, navigation.navigationKey]);
  const denied = collaborationIssueAccessDenied(client, scope, issueId);
  if (!query.data || denied || isCollaborationDenied(query.error)) return query.isPending
    ? <p role="status" className="py-10 text-body text-muted-foreground">{t(($) => $.page.loading)}</p>
    : <CollaborationQueryState error={query.error} denied={denied} retry={() => void query.refetch()} />;
  const issue = query.data;
  const rows = [...new Map([...(linkedComment.data ? [linkedComment.data] : []), ...(comments.data?.pages.flatMap((page) => page.items) ?? [])].map((comment) => [comment.id, comment])).values()]
    .sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime() || a.id.localeCompare(b.id));
  return <div ref={root} className="mx-auto w-full min-w-0 max-w-4xl space-y-8">
    {query.error && <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />}
    <header className="space-y-4">
      <div className="flex min-w-0 flex-wrap items-start justify-between gap-3">
        <h2 className="min-w-0 flex-1 text-heading font-semibold [overflow-wrap:anywhere]">{issue.title}</h2>
        <Button variant="ghost" size="sm" className="pointer-coarse:min-h-11" onClick={() => {
          void copyText(navigation.getShareableUrl(accountPath({ ...destination, view: "participating" }))).then((copied) => {
            if (copied) toast.success(tIssues(($) => $.detail.link_copied));
            else toast.error(tIssues(($) => $.detail.link_copy_failed));
          });
        }}><Link2 className="size-4" />{tIssues(($) => $.actions.copy_link)}</Button>
      </div>
      <dl className="flex flex-wrap gap-x-8 gap-y-3 text-caption [&_dt]:text-muted-foreground [&_dd]:mt-1 [&_dd]:text-body [&_dd]:[overflow-wrap:anywhere]">
        <div><dt>{t(($) => $.detail.status)}</dt><dd>{labels.status(issue.status)}</dd></div>
        <div><dt>{t(($) => $.detail.handler)}</dt><dd>{issue.handler.name}{issue.handler.external === true && <span className="ml-2 text-caption text-muted-foreground">{t(($) => $.detail.external)}</span>}</dd></div>
        <div><dt>{t(($) => $.detail.creator)}</dt><dd>{issue.creator.name}</dd></div>
        <div><dt>{t(($) => $.detail.source)}</dt><dd>{issue.source_workspace.name}</dd></div>
      </dl>
    </header>
    <CollaborationWorkflow scope={scope} issue={issue} />
    <section id="description" tabIndex={-1} className="min-w-0 space-y-3"><h3 className="text-body font-semibold">{t(($) => $.detail.description)}</h3>
      <RichContent resourcePolicy="restricted" content={issue.description} />
    </section>
    <CollaborationFieldSummary issue={issue} />
    <CollaborationChildren issue={issue} />
    <CollaborationIssueReactions scope={scope} issue={issue} />
    <CollaborationAttachmentPanel scope={scope} issue={issue} />
    <CollaborationExecutionPanel scope={scope} issue={issue} selectedRunId={destination.runId} />
    <CollaborationMemoryPanel scope={scope} issue={issue} destination={destination} isScopeResume={navigation.isScopeResume} navigationKey={navigation.navigationKey} />
    <CollaborationActivity scope={scope} destination={destination} />
    <section className="min-w-0 space-y-4"><h3 className="text-body font-semibold">{t(($) => $.detail.comments)}</h3>
      {comments.isPending && <p role="status" className="text-body text-muted-foreground">{t(($) => $.page.loading)}</p>}
      {comments.error && <CollaborationQueryState error={comments.error} retry={() => void comments.refetch()} />}
      {linkedComment.error && <CollaborationQueryState error={linkedComment.error} retry={() => void linkedComment.refetch()} />}
      {comments.isSuccess && rows.length === 0 && <p className="text-body text-muted-foreground">{t(($) => $.detail.noComments)}</p>}
      <CollaborationCommentList key={`comments:${issueId}`} scope={scope} issue={issue} comments={rows} onReply={setReplyTo} />
      {comments.hasNextPage && <Button variant="outline" className="pointer-coarse:min-h-11" disabled={comments.isFetchingNextPage} onClick={() => void comments.fetchNextPage()}>{t(($) => $.page.more)}</Button>}
      <CollaborationCommentComposer key={`composer:${issueId}`} scope={scope} issue={issue} replyTo={replyTo?.issue_id === issueId ? replyTo : null} onCancelReply={() => setReplyTo(null)} onSent={() => setReplyTo(null)} />
    </section>
  </div>;
}
