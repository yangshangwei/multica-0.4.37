import { useEffect, useState } from "react";
import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
import { collaborationActivityOptions, type CollaborationIssue, type CollaborationScope } from "@multica/core/collaboration";
import { accountPath, type AccountDestination } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { AppLink } from "../navigation";
import { RichContent } from "../rich-content";
import { useLocale, useT } from "../i18n";
import { CollaborationQueryState } from "./query-state";
import type { CollaborationPanelStatusChange } from "./panel-status";
import { useCollaborationStageLabels } from "./stage-labels";
import { CollaborationInformationHistory } from "./information-history";
import { useCollaborationLabels } from "./labels";

export function CollaborationActivity({ scope, destination, issue, active = true, onStatusChange }: { scope: CollaborationScope; destination: AccountDestination; issue?: CollaborationIssue; active?: boolean; onStatusChange?: CollaborationPanelStatusChange }) {
  const { t } = useT("collaboration");
  const locale = useLocale();
  const stageLabels = useCollaborationStageLabels();
  const labels = useCollaborationLabels();
  const [historyAttention, setHistoryAttention] = useState(false);
  const client = useQueryClient();
  const query = useInfiniteQuery(collaborationActivityOptions(client, scope, destination.issueId!));
  useEffect(() => { onStatusChange?.({ attention: query.isError || historyAttention }); }, [onStatusChange, query.isError, historyAttention]);
  const items = [...new Map((query.data?.pages.flatMap((page) => page.items) ?? []).map((item) => [item.id, item])).values()];
  return <section aria-label={t(($) => $.activity.title)} className="min-w-0 space-y-4">
    <h3 className="text-body font-semibold">{t(($) => $.activity.title)}</h3>
    {query.isPending && <p role="status" className="text-body text-muted-foreground">{t(($) => $.page.loading)}</p>}
    {query.error && <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />}
    {query.isSuccess && items.length === 0 && <p className="text-body text-muted-foreground">{t(($) => $.activity.empty)}</p>}
    <ol className="space-y-6">{items.map((item) => <li key={item.id} className="min-w-0 space-y-3">
      <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1 text-caption text-muted-foreground">
        <span className="font-medium text-foreground">{t(($) => $.activity.kinds[item.kind])}</span>
        <span className="min-w-0 [overflow-wrap:anywhere]">{item.actor?.name ?? t(($) => $.detail.unknownAuthor)}</span>
        <time dateTime={item.created_at}>{new Date(item.created_at).toLocaleString(locale)}</time>
      </div>
      {!item.details_available && <p className="text-caption text-muted-foreground">{t(($) => $.activity.detailsUnavailable)}</p>}
      {item.details_available && item.stage && <dl className="space-y-2 text-caption [overflow-wrap:anywhere]">{Object.entries(item.stage).map(([field, value]) => value !== null && <div key={field}>
        <dt className="text-muted-foreground">{t(($) => $.activity.stageFields[field as keyof typeof $.activity.stageFields])}</dt>
        <dd>{field === "current" ? stageLabels.stage(value) : field === "step" ? stageLabels.codeStep(value) : value}</dd>
      </div>)}</dl>}
      {item.details_available && item.kind === "iteration_changed" && item.note && <p className="text-body whitespace-pre-wrap [overflow-wrap:anywhere]">{item.note}</p>}
      {item.details_available && item.information_repair && <dl className="space-y-2 text-body [overflow-wrap:anywhere]">
        <div><dt className="text-caption text-muted-foreground">{t(($) => $.information.history.previousStatus)}</dt><dd>{labels.status(item.information_repair.previous_status)}</dd></div>
        <div><dt className="text-caption text-muted-foreground">{t(($) => $.information.history.restoredStatus)}</dt><dd>{labels.status(item.information_repair.restored_status)}</dd></div>
        <div><dt className="text-caption text-muted-foreground">{t(($) => $.information.history.to)}</dt><dd className="select-text">{item.information_repair.target_user_id}</dd></div>
        <div><dt className="text-caption text-muted-foreground">{t(($) => $.information.history.recordId)}</dt><dd className="select-text">{item.information_repair.handoff_id}</dd></div>
        <div><dt className="text-caption text-muted-foreground">{t(($) => $.information.repair.basis)}</dt><dd>{item.information_repair.basis.kind === "human_selected" ? t(($) => $.information.repair.humanBasis)
          : item.information_repair.basis.kind === "historical_information" ? t(($) => $.information.repair.historicalBasis, { status: labels.status(item.information_repair.restored_status) }) : t(($) => $.information.repair.pendingBasis)}</dd></div>
      </dl>}
      {item.handoff ? <div className="space-y-2 text-body">
        <p>{t(($) => $.activity.handoff, { from: item.handoff.from?.name ?? t(($) => $.detail.unknownAuthor), to: item.handoff.to?.name ?? t(($) => $.detail.unknownAuthor) })}</p>
        {item.handoff.note && <RichContent resourcePolicy="restricted" content={item.handoff.note} />}
      </div> : item.target && <p className="text-body [overflow-wrap:anywhere]">{item.target.name}</p>}
      {item.comment_id && <AppLink className="inline-flex min-h-11 items-center text-body underline underline-offset-4" href={accountPath({ view: destination.view, issueId: destination.issueId, commentId: item.comment_id })}>{t(($) => $.activity.openComment)}</AppLink>}
      <dl className="space-y-4">{item.changes.map((change) => <div key={change.field} className="min-w-0 space-y-2">
        <dt className="text-caption font-medium">{t(($) => $.activity.fields[change.field])}</dt>
        <dd className="grid min-w-0 gap-4 text-body sm:grid-cols-2">
          {(["before", "after"] as const).map((side) => <div key={side} className="min-w-0 space-y-1 [overflow-wrap:anywhere]">
            <p className="text-caption text-muted-foreground">{t(($) => $.activity[side])}</p>
            {change[side] === null || change[side] === "" ? <p className="text-muted-foreground">{t(($) => $.activity.cleared)}</p>
              : change.field === "description" ? <RichContent resourcePolicy="restricted" content={change[side]} /> : <p className="whitespace-pre-wrap">{change[side]}</p>}
          </div>)}
        </dd>
      </div>)}</dl>
    </li>)}</ol>
    {query.hasNextPage && <Button variant="outline" className="pointer-coarse:min-h-11" disabled={query.isFetchingNextPage} onClick={() => void query.fetchNextPage()}>{t(($) => $.page.more)}</Button>}
    <CollaborationInformationHistory scope={scope} issueId={destination.issueId!} issue={issue} active={active} onAttentionChange={setHistoryAttention} />
  </section>;
}
