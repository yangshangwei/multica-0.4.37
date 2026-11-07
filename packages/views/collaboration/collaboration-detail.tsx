import { useEffect, useRef, useState, type ReactNode } from "react";
import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";
import { collaborationCommentsOptions, collaborationCommentOptions, collaborationIssueAccessDenied, deriveCollaborationDetailState, isCollaborationDenied, useCollaborationIssue, useCollaborationPendingDrafts, type CollaborationComment, type CollaborationScope } from "@multica/core/collaboration";
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
import { CollaborationKnowledgePanel } from "./knowledge-panel";
import { CollaborationDeliveryPanel } from "./delivery-panel";
import { useRestoredScrollRef } from "../platform";
import { useNavigation } from "../navigation";

export function CollaborationDetail({ scope, destination }: { scope: CollaborationScope; destination: AccountDestination }) {
  const { t } = useT("collaboration");
  const { t: tIssues } = useT("issues");
  const navigation = useNavigation();
  const labels = useCollaborationLabels();
  const client = useQueryClient();
  const issueId = destination.issueId!;
  const identity = `${scope.apiScope}:${scope.userId}:${issueId}`;
  const query = useCollaborationIssue(scope, issueId);
  const pending = useCollaborationPendingDrafts({ ...scope, issueId });
  const comments = useInfiniteQuery({ ...collaborationCommentsOptions(client, scope, issueId), enabled: query.data?.capabilities.read === true });
  const linkedComment = useQuery({ ...collaborationCommentOptions(client, scope, issueId, destination.commentId ?? ""),
    enabled: query.data?.capabilities.read === true && destination.commentId !== null && !comments.data?.pages.some((page) => page.items.some((comment) => comment.id === destination.commentId)) });
  const root = useRef<HTMLDivElement>(null);
  const [replyTo, setReplyTo] = useState<{ identity: string; comment: CollaborationComment } | null>(null);
  const [resultComment, setResultComment] = useState<{ identity: string; comment: CollaborationComment } | null>(null);
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
    for (let ancestor = element.parentElement; ancestor && ancestor !== root.current; ancestor = ancestor.parentElement) {
      if (ancestor instanceof HTMLDetailsElement) ancestor.open = true;
    }
    container.scrollTop += element.getBoundingClientRect().top - container.getBoundingClientRect().top;
    element.focus({ preventScroll: true });
    consumedAnchor.current = intent;
  }, [destination.commentId, destination.runId, destination.anchor, query.data, comments.data, linkedComment.data, navigation.isScopeResume, navigation.navigationKey]);
  const denied = collaborationIssueAccessDenied(client, scope, issueId);
  if (!query.data || denied || isCollaborationDenied(query.error)) return query.isPending
    ? <p role="status" className="py-10 text-body text-muted-foreground">{t(($) => $.page.loading)}</p>
    : <CollaborationQueryState error={query.error} denied={denied} retry={() => void query.refetch()} />;
  const issue = query.data;
  const state = deriveCollaborationDetailState(issue, scope.userId);
  const stage = state.stage;
  const canPrepareResult = state.canSelfComplete || state.primary === "submit_acceptance" || state.secondary.includes("submit_acceptance");
  const rows = [...new Map([...(linkedComment.data ? [linkedComment.data] : []), ...(comments.data?.pages.flatMap((page) => page.items) ?? [])].map((comment) => [comment.id, comment])).values()]
    .sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime() || a.id.localeCompare(b.id));
  return <div ref={root} className="@container/collaboration-detail mx-auto w-full min-w-0 max-w-4xl space-y-8">
    {query.error && <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />}
    <header className="space-y-4">
      <div className="flex min-w-0 flex-wrap items-start justify-between gap-3">
        <h2 className="min-w-0 flex-1 text-display-sm font-semibold [overflow-wrap:anywhere]">{issue.title}</h2>
        <Button variant="ghost" size="sm" className="pointer-coarse:min-h-11" onClick={() => {
          void copyText(navigation.getShareableUrl(accountPath({ ...destination, view: "participating" }))).then((copied) => {
            if (copied) toast.success(tIssues(($) => $.detail.link_copied));
            else toast.error(tIssues(($) => $.detail.link_copy_failed));
          });
        }}><Link2 className="size-4" />{tIssues(($) => $.actions.copy_link)}</Button>
      </div>
      <dl className="flex flex-wrap gap-x-8 gap-y-3 text-caption [&_dt]:text-muted-foreground [&_dd]:mt-1 [&_dd]:text-body [&_dd]:[overflow-wrap:anywhere]">
        <div><dt>{t(($) => $.detail.status)}</dt><dd>{stage === "working" ? labels.status(issue.status) : t(($) => $.delivery.stages[stage])}</dd></div>
        <div><dt>{t(($) => $.detail.handler)}</dt><dd>{issue.handler.name}{issue.handler.external === true && <span className="ml-2 text-caption text-muted-foreground">{t(($) => $.detail.external)}</span>}</dd></div>
        {issue.due_date && <div><dt>{t(($) => $.workflow.dueDate)}</dt><dd>{issue.due_date}</dd></div>}
      </dl>
      <CollaborationWorkflow key={`workflow:${identity}`} scope={scope} issue={issue} resultFromComment={resultComment?.identity === identity ? resultComment.comment : null}
        onResultPrepared={() => setResultComment(null)} onDiscuss={() => {
          const textarea = root.current?.querySelector<HTMLTextAreaElement>("#discussion textarea");
          textarea?.focus(); textarea?.scrollIntoView?.({ block: "nearest" });
        }} />
    </header>
    <div className="min-w-0 space-y-8">
    <section id="description" tabIndex={-1} className="min-w-0 space-y-3"><h3 className="text-title-sm font-semibold">{t(($) => $.detail.description)}</h3>
      <div className="max-w-[75ch]"><RichContent resourcePolicy="restricted" content={issue.description} /></div>
      <Button variant="link" size="sm" className="h-auto max-w-full whitespace-normal px-0 py-1 text-caption text-muted-foreground underline-offset-4 pointer-coarse:min-h-11" onClick={() => {
        const files = root.current?.querySelector<HTMLDetailsElement>("#collaboration-files");
        if (files) { files.open = true; files.scrollIntoView?.({ block: "nearest" }); files.querySelector("summary")?.focus(); }
      }}>{t(($) => $.detail.referenceFiles)}</Button>
    </section>
    <CollaborationDeliveryPanel key={`delivery:${identity}`} scope={scope} issue={issue} />
    <CollaborationIssueReactions scope={scope} issue={issue} />
    <section id="discussion" tabIndex={-1} className="min-w-0 space-y-4"><h3 className="text-title-sm font-semibold">{t(($) => $.detail.comments)}</h3>
      {comments.isPending && <p role="status" className="text-body text-muted-foreground">{t(($) => $.page.loading)}</p>}
      {comments.error && <CollaborationQueryState error={comments.error} retry={() => void comments.refetch()} />}
      {linkedComment.error && <CollaborationQueryState error={linkedComment.error} retry={() => void linkedComment.refetch()} />}
      {comments.isSuccess && rows.length === 0 && <p className="text-body text-muted-foreground">{t(($) => $.detail.noComments)}</p>}
      {comments.isSuccess && <p className="text-caption text-muted-foreground">{comments.data?.pages[0]?.total !== undefined
        ? t(($) => $.recall.loaded, { loaded: new Set(comments.data.pages.flatMap((page) => page.items.map((item) => item.id))).size, total: comments.data.pages[0].total })
        : t(($) => $.recall.coverageUnknown)}</p>}
      <CollaborationCommentList key={`comments:${identity}`} scope={scope} issue={issue} comments={rows} onReply={(comment) => setReplyTo({ identity, comment })}
        onPrepareResult={canPrepareResult ? (comment) => setResultComment({ identity, comment }) : undefined} />
      {comments.hasNextPage && <Button variant="outline" className="pointer-coarse:min-h-11" disabled={comments.isFetchingNextPage} onClick={() => void comments.fetchNextPage()}>{t(($) => $.page.more)}</Button>}
      <CollaborationCommentComposer key={`composer:${identity}`} scope={scope} issue={issue} replyTo={replyTo?.identity === identity ? replyTo.comment : null} onCancelReply={() => setReplyTo(null)} onSent={() => setReplyTo(null)} />
    </section>
    <CollaborationExecutionPanel scope={scope} issue={issue} selectedRunId={destination.runId} />
    </div>
    <aside aria-label={t(($) => $.detail.context)} className="min-w-0 space-y-2 border-t border-border pt-5">
      <CollaborationDisclosure title={t(($) => $.detail.context)}>
        <dl className="grid min-w-0 gap-x-8 gap-y-3 text-body @xl/collaboration-detail:grid-cols-2 [&_dt]:mb-1 [&_dt]:text-caption [&_dt]:text-muted-foreground [&_dd]:[overflow-wrap:anywhere]">
          <div><dt>{t(($) => $.detail.creator)}</dt><dd>{issue.creator.name}</dd></div>
          <div><dt>{t(($) => $.detail.source)}</dt><dd>{issue.source_workspace.name}</dd></div>
          {state.stage !== "working" && <div><dt>{t(($) => $.detail.status)}</dt><dd>{labels.status(issue.status)}</dd></div>}
          <div><dt>{t(($) => $.policy.title)}</dt><dd>{issue.ai_policy === "manual" ? t(($) => $.policy.manual) : issue.ai_policy === "assisted" ? t(($) => $.policy.assisted) : t(($) => $.policy.unknown)}</dd></div>
        </dl>
        <CollaborationFieldSummary issue={issue} />
      </CollaborationDisclosure>
      <CollaborationDisclosure id="collaboration-files" title={t(($) => $.attachments.title)} attention={pending.error}>
        <CollaborationAttachmentPanel scope={scope} issue={issue} />
      </CollaborationDisclosure>
      <CollaborationDisclosure title={t(($) => $.memory.title)} attention={pending.memory.length > 0 || pending.error || !!destination.memoryVersion || destination.anchor === "memory"}>
        <CollaborationMemoryPanel scope={scope} issue={issue} destination={destination} isScopeResume={navigation.isScopeResume} navigationKey={navigation.navigationKey} />
      </CollaborationDisclosure>
      <CollaborationDisclosure title={t(($) => $.knowledge.title)} attention={pending.error}>
        <CollaborationKnowledgePanel scope={scope} issue={issue} />
      </CollaborationDisclosure>
      <CollaborationDisclosure title={t(($) => $.activity.title)}>
        <CollaborationActivity scope={scope} destination={destination} />
      </CollaborationDisclosure>
      <CollaborationDisclosure title={t(($) => $.children.title)}>
        <CollaborationChildren issue={issue} />
      </CollaborationDisclosure>
    </aside>
  </div>;
}

function CollaborationDisclosure({ id, title, attention = false, children }: { id?: string; title: string; attention?: boolean; children: ReactNode }) {
  const disclosure = useRef<HTMLDetailsElement>(null);
  useEffect(() => { if (attention && disclosure.current) disclosure.current.open = true; }, [attention]);
  return <details ref={disclosure} id={id} className="min-w-0 space-y-4">
    <summary className="cursor-pointer rounded-sm py-2 text-body font-medium text-muted-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring pointer-coarse:min-h-11">{title}</summary>
    {children}
  </details>;
}
