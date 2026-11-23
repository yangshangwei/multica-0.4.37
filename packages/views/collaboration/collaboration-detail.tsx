import { useEffect, useMemo, useRef, useState } from "react";
import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";
import { collaborationCommentsOptions, collaborationCommentOptions, collaborationIssueAccessDenied, deriveCollaborationCommentThreads, deriveCollaborationDetailState, isCollaborationDenied, useCollaborationIssue, type CollaborationComment, type CollaborationScope } from "@multica/core/collaboration";
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
import { CollaborationCommentList } from "./comment-list";
import { CollaborationIssueReactions } from "./discussion-actions";
import { CollaborationDetailAuxiliary, type CollaborationDetailAuxiliaryHandle } from "./detail-auxiliary";
import { CollaborationDeliveryPanel } from "./delivery-panel";
import { CollaborationStagePanel, CollaborationStageRecovery } from "./stage-panel";
import { useRestoredScrollRef } from "../platform";
import { useScrollRestorationAdapter } from "../platform/scroll-restoration";
import { useNavigation } from "../navigation";

export function CollaborationDetail({ scope, destination, returnRestorationKey }: {
  scope: CollaborationScope; destination: AccountDestination;
  /** Notifications only: undefined opens normally, null waits for the target, a key restores its memento. */
  returnRestorationKey?: string | null;
}) {
  const { t } = useT("collaboration");
  const { t: tIssues } = useT("issues");
  const navigation = useNavigation();
  // Scope resumes and Back/Forward restore position instead of replaying anchors.
  const restoringPosition = navigation.isScopeResume || navigation.isHistoryTraversal;
  const labels = useCollaborationLabels();
  const client = useQueryClient();
  const issueId = destination.issueId!;
  const identity = `${scope.apiScope}:${scope.userId}:${issueId}`;
  const query = useCollaborationIssue(scope, issueId);
  const comments = useInfiniteQuery({ ...collaborationCommentsOptions(client, scope, issueId), enabled: query.data?.capabilities.read === true });
  const linkedComment = useQuery({ ...collaborationCommentOptions(client, scope, issueId, destination.commentId ?? ""),
    enabled: query.data?.capabilities.read === true && destination.commentId !== null && !comments.data?.pages.some((page) => page.items.some((comment) => comment.id === destination.commentId)) });
  const root = useRef<HTMLDivElement>(null);
  const auxiliary = useRef<CollaborationDetailAuxiliaryHandle>(null);
  const [replyTo, setReplyTo] = useState<{ identity: string; comment: CollaborationComment } | null>(null);
  const [resultComment, setResultComment] = useState<{ identity: string; comment: CollaborationComment } | null>(null);
  const consumedAnchor = useRef<string | null>(null);
  const restored = useRef(false);
  const restoreScroll = useRestoredScrollRef("account-detail");
  const restoration = useScrollRestorationAdapter();
  const consumedReturn = useRef<string | null>(null);
  const latestReturn = useRef(returnRestorationKey);
  latestReturn.current = returnRestorationKey;
  const returning = returnRestorationKey !== undefined;
  useEffect(() => {
    if (!query.data || !comments.data) return;
    if (returning) {
      if (returnRestorationKey === null || consumedReturn.current === returnRestorationKey) return;
      if (destination.commentId && !comments.data.pages.some((page) => page.items.some((comment) => comment.id === destination.commentId)) && !linkedComment.data && !linkedComment.error) return;
      const container = root.current?.closest<HTMLElement>('[data-tab-scroll-root="account-detail"]');
      if (!container) return;
      const restore = () => {
        if (!container.isConnected || latestReturn.current !== returnRestorationKey) return;
        container.scrollTop = restoration?.get("account-detail")?.top ?? 0;
        consumedReturn.current = returnRestorationKey;
        restored.current = true;
      };
      if (auxiliary.current) auxiliary.current.closeForNavigation(restore);
      else restore();
      return;
    }
    if (restored.current) return;
    if (!restoringPosition && (destination.commentId || destination.runId || destination.submissionId || destination.anchor)) {
      // An initial notification anchor intentionally skips its offset. It
      // must not apply another route's offset on a later ordinary row click.
      if (destination.view === "notifications") restored.current = true;
      return;
    }
    restoreScroll(root.current?.closest<HTMLElement>('[data-tab-scroll-root="account-detail"]') ?? null);
    restored.current = true;
  }, [query.data, comments.data, linkedComment.data, linkedComment.error, restoreScroll, restoration, returning, returnRestorationKey, destination.view, destination.commentId, destination.runId, destination.submissionId, destination.anchor, restoringPosition]);
  useEffect(() => {
    const anchor = destination.runId ? `run-${destination.runId}` : destination.commentId ? `comment-${destination.commentId}` : destination.anchor;
    if (!anchor || restoringPosition || returning) { consumedAnchor.current = null; return; }
    const intent = `${navigation.navigationKey ?? 0}:${anchor}`;
    if (consumedAnchor.current === intent) return;
    const element = root.current?.querySelector<HTMLElement>(`[id="${anchor}"]`);
    const container = element?.closest<HTMLElement>('[data-tab-scroll-root="account-detail"]');
    if (!element || !container) return;
    for (let ancestor = element.parentElement; ancestor && ancestor !== root.current; ancestor = ancestor.parentElement) {
      if (ancestor instanceof HTMLDetailsElement) ancestor.open = true;
    }
    consumedAnchor.current = intent;
    const focusTarget = () => {
      if (!element.isConnected || latestReturn.current !== undefined) return;
      container.scrollTop += element.getBoundingClientRect().top - container.getBoundingClientRect().top;
      element.focus({ preventScroll: true });
    };
    if (auxiliary.current) auxiliary.current.closeForNavigation(focusTarget);
    else focusTarget();
  }, [destination.commentId, destination.runId, destination.anchor, query.data, comments.data, linkedComment.data, returning, restoringPosition, navigation.navigationKey]);
  const pages = comments.data?.pages;
  const linked = linkedComment.data;
  const rows = useMemo(() => [...new Map([...(linked ? [linked] : []), ...(pages?.flatMap((page) => page.items) ?? [])].map((comment) => [comment.id, comment])).values()]
    .sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime() || a.id.localeCompare(b.id)), [pages, linked]);
  // One derivation feeds both the header count and the list's thread layout.
  const threads = useMemo(() => deriveCollaborationCommentThreads(rows), [rows]);
  const denied = collaborationIssueAccessDenied(client, scope, issueId);
  if (!query.data || denied || isCollaborationDenied(query.error)) return <>
    {query.isPending ? <p role="status" className="py-10 text-body text-muted-foreground">{t(($) => $.page.loading)}</p>
      : <CollaborationQueryState error={query.error} denied={denied} retry={() => void query.refetch()} />}
    {(denied || isCollaborationDenied(query.error)) && <CollaborationStageRecovery key={`stage-recovery:${identity}`} scope={scope} issueId={issueId} />}
  </>;
  const issue = query.data;
  const state = deriveCollaborationDetailState(issue, scope.userId);
  const stage = state.stage;
  const canPrepareResult = state.canSelfComplete || state.primary === "submit_acceptance" || state.secondary.includes("submit_acceptance");
  const total = pages?.[0]?.total;
  // Without every page, an older comment may still carry a thread's resolution.
  const allLoaded = comments.isSuccess && comments.hasNextPage !== true;
  const pendingThreads = threads.filter((thread) => thread.pending);
  const firstPending = pendingThreads[0]?.items[0]?.comment.id;
  function focusFirstPending() {
    const element = firstPending ? root.current?.querySelector<HTMLElement>(`[id="comment-${firstPending}"]`) : null;
    if (!element) return;
    const container = element.closest<HTMLElement>('[data-tab-scroll-root="account-detail"]');
    if (container) container.scrollTop += element.getBoundingClientRect().top - container.getBoundingClientRect().top;
    else element.scrollIntoView?.({ block: "start" });
    element.focus({ preventScroll: true });
  }
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
      <CollaborationDetailAuxiliary key={`auxiliary:${identity}`} ref={auxiliary} scope={scope} issue={issue}
        destination={returning && (destination.anchor === "memory" || destination.memoryVersion) ? { ...destination, anchor: undefined, memoryVersion: null } : destination}
        isScopeResume={restoringPosition} navigationKey={navigation.navigationKey} />
    </header>
    <div className="min-w-0 space-y-8">
    <section id="description" tabIndex={-1} className="min-w-0 space-y-3"><h3 className="text-title-sm font-semibold">{t(($) => $.detail.description)}</h3>
      <div className="max-w-[75ch]"><RichContent resourcePolicy="restricted" content={issue.description} /></div>
      <Button variant="link" size="sm" className="h-auto max-w-full whitespace-normal px-0 py-1 text-caption text-muted-foreground underline-offset-4 pointer-coarse:min-h-11" onClick={(event) => auxiliary.current?.open("files", event.currentTarget)}>{t(($) => $.detail.referenceFiles)}</Button>
    </section>
    <CollaborationStagePanel key={`stage:${identity}`} scope={scope} issue={issue} readUnavailable={query.isError} />
    <CollaborationDeliveryPanel key={`delivery:${identity}`} scope={scope} issue={issue} submissionId={returning ? null : destination.submissionId}
      navigationKey={navigation.navigationKey} isScopeResume={restoringPosition}
      onOpenKnowledge={(opener) => auxiliary.current?.open("knowledge", opener)} onSubmissionReady={(element) => {
        if (restoringPosition || latestReturn.current !== undefined) return;
        const focus = () => {
          const container = element.closest<HTMLElement>('[data-tab-scroll-root="account-detail"]');
          if (container) container.scrollTop += element.getBoundingClientRect().top - container.getBoundingClientRect().top;
          else element.scrollIntoView?.({ block: "start" });
          element.focus({ preventScroll: true });
        };
        if (auxiliary.current) auxiliary.current.closeForNavigation(focus); else focus();
      }} />
    <CollaborationIssueReactions scope={scope} issue={issue} />
    <section id="discussion" tabIndex={-1} className="min-w-0"><div className="min-w-0 max-w-[75ch] space-y-4">
      <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1">
        <h3 className="text-title-sm font-semibold">{t(($) => $.detail.comments)}</h3>
        {comments.isSuccess && total !== undefined && <><span data-comment-total aria-hidden className="text-body tabular-nums text-muted-foreground">{total}</span>
          <span className="sr-only">{t(($) => $.detail.threadComments, { count: total })}</span></>}
        {comments.isSuccess && pendingThreads.length > 0 && <Button type="button" variant="link" size="sm" className="h-auto max-w-full whitespace-normal px-0 py-1 text-left text-caption text-foreground underline underline-offset-4 pointer-coarse:min-h-11" onClick={focusFirstPending}>
          {allLoaded ? t(($) => $.detail.pendingThreads, { count: pendingThreads.length }) : t(($) => $.detail.pendingThreadsLoaded, { count: pendingThreads.length })}
        </Button>}
      </div>
      {comments.isPending && <p role="status" className="text-body text-muted-foreground">{t(($) => $.page.loading)}</p>}
      {comments.error && <CollaborationQueryState error={comments.error} retry={() => void comments.refetch()} />}
      {linkedComment.error && <CollaborationQueryState error={linkedComment.error} retry={() => void linkedComment.refetch()} />}
      {comments.isSuccess && rows.length === 0 && <p className="text-body text-muted-foreground">{t(($) => $.detail.noComments)}</p>}
      {comments.isSuccess && total === undefined && <p className="text-caption text-muted-foreground">{t(($) => $.recall.coverageUnknown)}</p>}
      <CollaborationCommentList key={`comments:${identity}`} scope={scope} issue={issue} comments={rows} threads={threads} allLoaded={allLoaded}
        revealCommentId={restoringPosition || returning ? null : destination.commentId} revealKey={navigation.navigationKey} onReply={(comment) => setReplyTo({ identity, comment })}
        onPrepareResult={canPrepareResult ? (comment) => setResultComment({ identity, comment }) : undefined} />
      {comments.hasNextPage && <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-2">
        <Button variant="outline" className="pointer-coarse:min-h-11" disabled={comments.isFetchingNextPage} onClick={() => void comments.fetchNextPage()}>{t(($) => $.page.more)}</Button>
        {total !== undefined && <p className="text-caption text-muted-foreground">{t(($) => $.recall.loaded, { loaded: new Set((pages ?? []).flatMap((page) => page.items.map((item) => item.id))).size, total })}</p>}
      </div>}
      <CollaborationCommentComposer key={`composer:${identity}`} scope={scope} issue={issue} replyTo={replyTo?.identity === identity ? replyTo.comment : null} onCancelReply={() => setReplyTo(null)} onSent={() => setReplyTo(null)} />
    </div></section>
    <CollaborationExecutionPanel scope={scope} issue={issue} selectedRunId={destination.runId} />
    </div>

  </div>;
}
