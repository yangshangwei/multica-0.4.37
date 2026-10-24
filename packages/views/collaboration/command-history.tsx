import { useId, useState } from "react";
import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
import { collaborationCommandHistoryOptions, collaborationIssueAccessDenied, isCollaborationDenied,
  type CollaborationCommandHistoryEntry, type CollaborationScope } from "@multica/core/collaboration";
import { accountPath } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../i18n";
import { AppLink } from "../navigation";
import { RichContent } from "../rich-content";
import { CollaborationCommentAttachments } from "./attachment-panel";
import { useCollaborationLabels } from "./labels";
import { CollaborationQueryState } from "./query-state";

export function CollaborationCommandHistory({ scope, issueId }: { scope: CollaborationScope; issueId: string }) {
  const { t } = useT("collaboration");
  const [open, setOpen] = useState(false);
  const region = useId();
  return <div className="min-w-0 space-y-4">
    <h4><Button type="button" variant="outline" className="pointer-coarse:min-h-11" aria-expanded={open} aria-controls={region}
      onClick={() => setOpen(!open)}>{t(($) => $.commandHistory.title)}</Button></h4>
    {open && <section id={region} aria-label={t(($) => $.commandHistory.title)} className="min-w-0 space-y-5 border-l-2 border-border pl-3 sm:pl-4">
      <CommandHistoryRecords key={`${scope.apiScope}:${scope.userId}:${issueId}`} scope={scope} issueId={issueId} />
    </section>}
  </div>;
}

function CommandHistoryRecords({ scope, issueId }: { scope: CollaborationScope; issueId: string }) {
  const { t } = useT("collaboration");
  const client = useQueryClient();
  const query = useInfiniteQuery(collaborationCommandHistoryOptions(client, scope, issueId));
  if (collaborationIssueAccessDenied(client, scope, issueId) || isCollaborationDenied(query.error)) {
    return <CollaborationQueryState denied error={query.error} />;
  }
  const items = query.data?.pages.flatMap((page) => page.items) ?? [];
  return <>
    {query.isPending && <p role="status" className="text-body text-muted-foreground">{t(($) => $.page.loading)}</p>}
    {query.error && <CollaborationQueryState error={query.error} retry={() => void (query.isFetchNextPageError ? query.fetchNextPage() : query.refetch())} />}
    {query.data && items.length === 0 && <p className="text-body text-muted-foreground">{t(($) => $.commandHistory.empty)}</p>}
    <ol className="space-y-6">{items.map((item) => <CommandHistoryEntry key={item.id} item={item} scope={scope} issueId={issueId} />)}</ol>
    {query.hasNextPage && <Button type="button" variant="outline" className="pointer-coarse:min-h-11" disabled={query.isFetching}
      onClick={() => void query.fetchNextPage()}>{t(($) => $.commandHistory.more)}</Button>}
  </>;
}

function CommandHistoryEntry({ item, scope, issueId }: { item: CollaborationCommandHistoryEntry; scope: CollaborationScope; issueId: string }) {
  const { t } = useT("collaboration");
  const labels = useCollaborationLabels();
  const cancellation = item.cancellation;
  const identifiers = [
    [t(($) => $.records.inputId), item.id],
    [t(($) => $.execution.author), item.author_user_id],
    [t(($) => $.commandHistory.target), item.target_id],
    [t(($) => $.commandHistory.sequence), item.seq],
    [t(($) => $.commandHistory.inputEvent), item.input_event_id],
    [t(($) => $.commandHistory.eventSequence), item.input_event_seq],
  ];
  return <li className="min-w-0 space-y-3" data-command-id={item.id}>
    <div className="flex min-w-0 flex-wrap items-baseline gap-x-3 gap-y-1 [overflow-wrap:anywhere]">
      <h5 className="text-body font-medium">{t(($) => $.execution.input, { sequence: item.seq })}</h5>
      <span className="text-caption text-muted-foreground">{labels.execution(item.state)}</span>
    </div>
    <div className="flex min-w-0 flex-wrap items-baseline gap-x-3 gap-y-1 text-caption text-muted-foreground [overflow-wrap:anywhere]">
      <span>{t(($) => $.execution.author)}: <span className="font-medium text-foreground">{item.author?.name ?? item.author_user_id}</span></span>
      <time dateTime={item.created_at} title={item.created_at}>{new Date(item.created_at).toLocaleString()}</time>
    </div>
    <p className="text-caption text-muted-foreground [overflow-wrap:anywhere]">{t(($) => $.commandHistory.target)}: {item.target.kind === "squad" ? t(($) => $.execution.squad) : t(($) => $.execution.agent)} {item.target.resource_id}</p>
    <RichContent resourcePolicy="restricted" content={item.content} />
    <CollaborationCommentAttachments scope={scope} issueId={issueId} attachmentIds={item.attachment_ids} />
    {cancellation ? <div className="min-w-0 space-y-2">
      {cancellation.kind === null && <p className="text-caption text-muted-foreground">{t(($) => $.commandHistory.missingCancellation)}</p>}
      {cancellation.kind === "automatic" && <p className="text-caption text-muted-foreground">{t(($) => $.commandHistory.automaticCancellation)}</p>}
      <dl className="grid min-w-0 gap-x-6 gap-y-3 text-body sm:grid-cols-2 [&_dt]:text-caption [&_dt]:text-muted-foreground [&_dd]:mt-1 [&_dd]:[overflow-wrap:anywhere]">
        <div><dt>{t(($) => $.commandHistory.reason)}</dt><dd><CommandReason code={item.reason} /></dd></div>
        <div><dt>{t(($) => $.commandHistory.cancelledBy)}</dt><dd>{cancellation.actor?.name ?? cancellation.actor_user_id ?? t(($) => $.records.notRecorded)}</dd></div>
        <div><dt>{t(($) => $.commandHistory.cancelledAt)}</dt><dd>{cancellation.created_at === null ? t(($) => $.records.notRecorded)
          : <time dateTime={cancellation.created_at} title={cancellation.created_at}>{new Date(cancellation.created_at).toLocaleString()}</time>}</dd></div>
      </dl>
    </div> : item.reason !== null && <p className="text-caption text-muted-foreground [overflow-wrap:anywhere]"><CommandReason code={item.reason} /></p>}
    {item.comment_id !== null && <AppLink href={accountPath({ view: "participating", issueId, commentId: item.comment_id })}
      className="inline-flex min-h-11 items-center text-caption underline underline-offset-4">{t(($) => $.commandHistory.originalComment)}</AppLink>}
    <details className="min-w-0 space-y-3">
      <summary className="cursor-pointer py-2 text-caption font-medium pointer-coarse:min-h-11">{t(($) => $.commandHistory.details)}</summary>
      <dl className="grid min-w-0 gap-x-6 gap-y-3 text-caption sm:grid-cols-2 [&_dt]:text-muted-foreground [&_dd]:mt-1 [&_dd]:[overflow-wrap:anywhere]">
        {identifiers.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value ?? t(($) => $.records.notRecorded)}</dd></div>)}
        {item.group_id !== null && <div><dt>{t(($) => $.records.groupId)}</dt><dd>{item.group_id}</dd></div>}
        {item.reason !== null && <div><dt>{t(($) => $.commandHistory.reason)}</dt><dd>{item.reason}</dd></div>}
      </dl>
      {cancellation?.event_id != null && <div className="min-w-0 space-y-2">
        <p className="text-caption font-medium">{t(($) => $.commandHistory.cancellation)}</p>
        <dl className="grid min-w-0 gap-x-6 gap-y-3 text-caption sm:grid-cols-2 [&_dt]:text-muted-foreground [&_dd]:mt-1 [&_dd]:[overflow-wrap:anywhere]">
          <div><dt>{t(($) => $.commandHistory.eventId)}</dt><dd>{cancellation.event_id}</dd></div>
          <div><dt>{t(($) => $.commandHistory.eventSequence)}</dt><dd>{cancellation.event_seq}</dd></div>
          {cancellation.actor_user_id !== null && <div><dt>{t(($) => $.commandHistory.cancelledBy)}</dt><dd>{cancellation.actor_user_id}</dd></div>}
        </dl>
      </div>}
      {item.attachment_ids.length > 0 && <div className="space-y-2">
        <p className="text-caption text-muted-foreground">{t(($) => $.records.attachments)}</p>
        <ul className="space-y-1 text-caption [overflow-wrap:anywhere]">{item.attachment_ids.map((id) => <li key={id}>{id}</li>)}</ul>
      </div>}
    </details>
  </li>;
}

function CommandReason({ code }: { code: string | null }) {
  const { t } = useT("collaboration");
  switch (code) {
    case "cancelled_by_participant": return t(($) => $.commandHistory.reasons.cancelled_by_participant);
    case "participant_removed": return t(($) => $.commandHistory.reasons.participant_removed);
    case "participant_left": return t(($) => $.commandHistory.reasons.participant_left);
    case "issue_completed": return t(($) => $.commandHistory.reasons.issue_completed);
    case "issue_cancelled": return t(($) => $.commandHistory.reasons.issue_cancelled);
    case "credentials_revoked": return t(($) => $.commandHistory.reasons.credentials_revoked);
    case "stale_authority": return t(($) => $.commandHistory.reasons.stale_authority);
    case "never_launched": return t(($) => $.execution.neverLaunched);
    default: return code ?? t(($) => $.records.notRecorded);
  }
}
