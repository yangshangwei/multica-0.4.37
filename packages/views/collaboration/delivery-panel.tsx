import { useState } from "react";
import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";
import { collaborationAttachmentsOptions, collaborationDeliveryHistoryOptions,
  type CollaborationIssue, type CollaborationScope, type CollaborationSubmission } from "@multica/core/collaboration";
import { accountPath } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@multica/ui/components/ui/collapsible";
import { useLocale, useT } from "../i18n";
import { AppLink } from "../navigation";
import { RichContent } from "../rich-content";
import { CollaborationAttachmentList } from "./attachment-panel";
import { CollaborationQueryState } from "./query-state";

export function CollaborationDeliveryPanel({ scope, issue }: { scope: CollaborationScope; issue: CollaborationIssue }) {
  const { t } = useT("collaboration");
  const client = useQueryClient();
  const [historyOpen, setHistoryOpen] = useState(false);
  const history = useInfiniteQuery({ ...collaborationDeliveryHistoryOptions(client, scope, issue.id), enabled: historyOpen && issue.delivery?.has_history === true });
  const submission = issue.delivery?.current ?? issue.delivery?.latest;
  const prior = [...new Map((history.data?.pages.flatMap((page) => page.items) ?? []).map((item) => [item.id, item])).values()]
    .filter((item) => item.id !== submission?.id);
  return <section id="delivery" tabIndex={-1} aria-label={t(($) => $.delivery.title)} className="min-w-0 space-y-5 outline-none focus-visible:ring-2 focus-visible:ring-ring">
    <h3 className="text-title-sm font-semibold">{t(($) => $.delivery.title)}</h3>
    {submission ? <CollaborationSubmissionRecord scope={scope} issueId={issue.id} submission={submission} />
      : <p className="text-body text-muted-foreground">{issue.delivery === undefined ? t(($) => $.delivery.unavailable) : t(($) => $.delivery.empty)}</p>}
    {issue.status_category === "done" && submission?.decision === null && <p className="text-body text-muted-foreground">{t(($) => $.delivery.noAcceptanceRecorded)}</p>}
    {issue.delivery?.has_history === true && <Collapsible open={historyOpen} onOpenChange={setHistoryOpen} className="min-w-0 space-y-4">
      <CollapsibleTrigger render={<Button type="button" variant="ghost" className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11" />}>{t(($) => $.delivery.history)}</CollapsibleTrigger>
      <CollapsibleContent className="min-w-0 space-y-5">
        {history.isPending && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.page.loading)}</p>}
        {history.error && <CollaborationQueryState error={history.error} retry={() => void history.refetch()} />}
        {history.isSuccess && !history.hasNextPage && prior.length === 0 && <p className="text-body text-muted-foreground">{t(($) => $.delivery.emptyHistory)}</p>}
        <ol className="min-w-0 space-y-8">{prior.map((item) => <li key={item.id} className="min-w-0">
          <CollaborationSubmissionRecord scope={scope} issueId={issue.id} submission={item} />
        </li>)}</ol>
        {history.hasNextPage && <Button type="button" variant="outline" className="pointer-coarse:min-h-11" disabled={history.isFetchingNextPage} onClick={() => void history.fetchNextPage()}>{t(($) => $.page.more)}</Button>}
      </CollapsibleContent>
    </Collapsible>}
  </section>;
}

/** Render only the exact immutable submission projected by the Server. */
export function CollaborationSubmissionRecord({ scope, issueId, submission }: { scope: CollaborationScope; issueId: string; submission: CollaborationSubmission }) {
  const { t } = useT("collaboration");
  const locale = useLocale();
  const decision = submission.decision;
  return <div data-submission-id={submission.id} className="min-w-0 space-y-5">
    <dl className="flex min-w-0 flex-wrap gap-x-6 gap-y-2 text-caption [&_dt]:text-muted-foreground [&_dd]:mt-1 [&_dd]:[overflow-wrap:anywhere]">
      <div><dt>{t(($) => $.delivery.author)}</dt><dd>{submission.author?.name ?? t(($) => $.detail.unknownAuthor)}</dd></div>
      <div><dt>{t(($) => $.delivery.submittedAt)}</dt><dd><time dateTime={submission.created_at}>{new Date(submission.created_at).toLocaleString(locale)}</time></dd></div>
    </dl>
    {submission.evidence_state !== "available" && <p className="text-body text-muted-foreground">{t(($) => $.delivery.legacy)}</p>}
    <div className="min-w-0 max-w-[75ch] space-y-2"><h4 className="text-body font-medium">{t(($) => $.delivery.result)}</h4>
      {submission.result !== null ? <RichContent content={submission.result} resourcePolicy="restricted" /> : <p className="text-body text-muted-foreground">{t(($) => $.delivery.resultUnavailable)}</p>}
    </div>
    <div className="min-w-0 max-w-[75ch] space-y-2"><h4 className="text-body font-medium">{t(($) => $.delivery.basis)}</h4>
      {submission.basis !== null ? <RichContent content={submission.basis} resourcePolicy="restricted" /> : <p className="text-body text-muted-foreground">{t(($) => $.delivery.basisUnavailable)}</p>}
    </div>
    <SubmissionFiles scope={scope} issueId={issueId} attachmentIds={submission.attachment_ids} />
    {submission.source_comment_id && <AppLink href={accountPath({ view: "participating", issueId, commentId: submission.source_comment_id })} className="inline-flex min-h-9 items-center text-caption text-muted-foreground underline underline-offset-4 pointer-coarse:min-h-11">{t(($) => $.delivery.sourceComment)}</AppLink>}
    {decision && <div className="min-w-0 space-y-3 rounded-md bg-muted/30 p-4">
      <p className="text-body font-medium">{t(($) => $.delivery.decisions[decision.kind])}</p>
      <dl className="flex min-w-0 flex-wrap gap-x-6 gap-y-2 text-caption [&_dt]:text-muted-foreground [&_dd]:mt-1 [&_dd]:[overflow-wrap:anywhere]">
        <div><dt>{t(($) => $.delivery.decidedBy)}</dt><dd>{decision.actor?.name ?? t(($) => $.detail.unknownAuthor)}</dd></div>
        <div><dt>{t(($) => $.delivery.decisionAt)}</dt><dd><time dateTime={decision.created_at}>{new Date(decision.created_at).toLocaleString(locale)}</time></dd></div>
      </dl>
      {decision.reason !== null && <RichContent content={decision.reason} resourcePolicy="restricted" />}
      {decision.target_user_id && <details className="text-caption text-muted-foreground"><summary className="cursor-pointer py-1 pointer-coarse:min-h-11">{t(($) => $.delivery.revisionRecipient)}</summary><p className="py-2 [overflow-wrap:anywhere]">{decision.target_user_id}</p></details>}
    </div>}
  </div>;
}

function SubmissionFiles({ scope, issueId, attachmentIds }: { scope: CollaborationScope; issueId: string; attachmentIds: string[] }) {
  const { t } = useT("collaboration");
  const client = useQueryClient();
  const query = useQuery({ ...collaborationAttachmentsOptions(client, scope, issueId), enabled: attachmentIds.length > 0 });
  if (attachmentIds.length === 0) return null;
  const files = query.data?.attachments.filter((file) => attachmentIds.includes(file.id)) ?? [];
  return <section aria-label={t(($) => $.delivery.evidence)} className="min-w-0 space-y-3">
    <h4 className="text-body font-medium">{t(($) => $.delivery.evidence)}</h4>
    {query.isPending && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.attachments.loading)}</p>}
    {query.error && <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />}
    {query.data && files.length !== attachmentIds.length && <p className="text-caption text-muted-foreground">{t(($) => $.delivery.filesUnavailable)}</p>}
    <CollaborationAttachmentList scope={scope} issueId={issueId} files={files} />
  </section>;
}
