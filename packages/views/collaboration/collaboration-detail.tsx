import { useEffect, useRef } from "react";
import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";
import { collaborationCommentsOptions, collaborationCommentOptions, collaborationIssueAccessDenied, isCollaborationDenied, useCollaborationIssue, type CollaborationScope } from "@multica/core/collaboration";
import type { AccountDestination } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { RichContent } from "../rich-content";
import { useT } from "../i18n";
import { useCollaborationLabels } from "./labels";
import { CollaborationQueryState } from "./query-state";
import { CollaborationWorkflow } from "./workflow";
import { CollaborationExecutionPanel } from "./execution-panel";
import { CollaborationCommentComposer } from "./comment-composer";
import { CollaborationActivity } from "./activity";
import { CollaborationCommentHistory } from "./comment-history";
import { useRestoredScrollRef } from "../platform";

export function CollaborationDetail({ scope, destination }: { scope: CollaborationScope; destination: AccountDestination }) {
  const { t } = useT("collaboration");
  const labels = useCollaborationLabels();
  const client = useQueryClient();
  const issueId = destination.issueId!;
  const query = useCollaborationIssue(scope, issueId);
  const comments = useInfiniteQuery({ ...collaborationCommentsOptions(client, scope, issueId), enabled: query.data?.capabilities.read === true });
  const linkedComment = useQuery({ ...collaborationCommentOptions(client, scope, issueId, destination.commentId ?? ""),
    enabled: query.data?.capabilities.read === true && destination.commentId !== null && !comments.data?.pages.some((page) => page.items.some((comment) => comment.id === destination.commentId)) });
  const root = useRef<HTMLDivElement>(null);
  const consumedAnchor = useRef<string | null>(null);
  const restored = useRef(false);
  const restoreScroll = useRestoredScrollRef("account-detail");
  useEffect(() => {
    if (!query.data || !comments.data || restored.current || destination.commentId || destination.anchor) return;
    restoreScroll(root.current?.closest<HTMLElement>('[data-tab-scroll-root="account-detail"]') ?? null);
    restored.current = true;
  }, [query.data, comments.data, restoreScroll, destination.commentId, destination.anchor]);
  useEffect(() => {
    const anchor = destination.commentId ? `comment-${destination.commentId}` : destination.anchor;
    if (!anchor) { consumedAnchor.current = null; return; }
    if (consumedAnchor.current === anchor) return;
    const element = root.current?.querySelector<HTMLElement>(`[id="${anchor}"]`);
    const container = element?.closest<HTMLElement>('[data-tab-scroll-root="account-detail"]');
    if (!element || !container) return;
    container.scrollTop += element.getBoundingClientRect().top - container.getBoundingClientRect().top;
    element.focus({ preventScroll: true });
    consumedAnchor.current = anchor;
  }, [destination.commentId, destination.anchor, query.data, comments.data, linkedComment.data]);
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
      <h2 className="text-heading font-semibold [overflow-wrap:anywhere]">{issue.title}</h2>
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
    <CollaborationExecutionPanel scope={scope} issue={issue} />
    <CollaborationActivity scope={scope} destination={destination} />
    <section className="min-w-0 space-y-4"><h3 className="text-body font-semibold">{t(($) => $.detail.comments)}</h3>
      {comments.isPending && <p role="status" className="text-body text-muted-foreground">{t(($) => $.page.loading)}</p>}
      {comments.error && <CollaborationQueryState error={comments.error} retry={() => void comments.refetch()} />}
      {linkedComment.error && <CollaborationQueryState error={linkedComment.error} retry={() => void linkedComment.refetch()} />}
      {comments.isSuccess && rows.length === 0 && <p className="text-body text-muted-foreground">{t(($) => $.detail.noComments)}</p>}
      <ol className="space-y-6">{rows.map((comment) => <li id={`comment-${comment.id}`} tabIndex={-1} key={comment.id} className="min-w-0 space-y-3 rounded-md py-2 target:bg-accent">
        <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1 text-caption text-muted-foreground"><span className="font-medium text-foreground">{comment.author?.name ?? t(($) => $.detail.unknownAuthor)}</span><time dateTime={comment.created_at}>{new Date(comment.created_at).toLocaleString()}</time>{comment.resolved_at !== null && <span>{t(($) => $.detail.resolved)}</span>}</div>
        {comment.deleted_at !== null ? <p className="text-body text-muted-foreground">{t(($) => $.detail.deleted)}</p> : <RichContent resourcePolicy="restricted" content={comment.content} />}
        <CollaborationCommentHistory scope={scope} issueId={issueId} commentId={comment.id} />
      </li>)}</ol>
      {comments.hasNextPage && <Button variant="outline" className="pointer-coarse:min-h-11" disabled={comments.isFetchingNextPage} onClick={() => void comments.fetchNextPage()}>{t(($) => $.page.more)}</Button>}
      <CollaborationCommentComposer scope={scope} issue={issue} />
    </section>
  </div>;
}
