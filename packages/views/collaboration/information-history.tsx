import { useCallback, useEffect, useId, useState } from "react";
import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";
import { collaborationInformationAffectedOptions, collaborationInformationHandoffOptions, collaborationInformationHistoryHasDetails,
  collaborationInformationHistoryOptions, type CollaborationInformationHistoryItem, type CollaborationIssue, type CollaborationScope } from "@multica/core/collaboration";
import { Button } from "@multica/ui/components/ui/button";
import { useLocale, useT } from "../i18n";
import { RichContent } from "../rich-content";
import { CollaborationInformationBlock } from "./information-block";
import { useCollaborationLabels } from "./labels";
import { CollaborationQueryState } from "./query-state";

type AttentionChange = (key: string, attention: boolean) => void;

export function CollaborationInformationHistory({ scope, issueId, issue, active = true, onAttentionChange }: {
  scope: CollaborationScope; issueId: string; issue?: CollaborationIssue; active?: boolean; onAttentionChange?: (attention: boolean) => void;
}) {
  const { t } = useT("collaboration");
  const client = useQueryClient();
  const [open, setOpen] = useState(false);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [affectedAttention, setAffectedAttention] = useState<Record<string, boolean>>({});
  const affectedStatus = useCallback<AttentionChange>((key, attention) => {
    setAffectedAttention((current) => current[key] === attention ? current : { ...current, [key]: attention });
  }, []);
  const query = useInfiniteQuery({ ...collaborationInformationHistoryOptions(client, scope, issueId, issue), enabled: active && open });
  const detail = useQuery({ ...collaborationInformationHandoffOptions(client, scope, issueId, selectedId ?? issueId), enabled: active && open && selectedId !== null });
  const attention = query.isError || detail.isError || Object.values(affectedAttention).some(Boolean);
  useEffect(() => { onAttentionChange?.(attention); }, [attention, onAttentionChange]);
  const items = query.data?.pages.flatMap((page) => page.items) ?? [];
  return <details open={open} onToggle={(event) => setOpen(event.currentTarget.open)} className="min-w-0 space-y-4">
    <summary className="cursor-pointer py-2 text-body font-medium pointer-coarse:min-h-11">{t(($) => $.information.history.title)}</summary>
    <section aria-label={t(($) => $.information.history.title)} className="min-w-0 space-y-5">
      {query.isFetching && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.page.loading)}</p>}
      {query.error && <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />}
      {query.isSuccess && !query.hasNextPage && items.length === 0 && <p className="text-body text-muted-foreground">{t(($) => $.information.history.empty)}</p>}
      {selectedId && <section aria-label={t(($) => $.information.history.openRecord)} className="min-w-0 space-y-4 rounded-md bg-muted/30 p-4">
        <Button type="button" variant="ghost" className="pointer-coarse:min-h-11" onClick={() => setSelectedId(null)}>{t(($) => $.information.history.closeRecord)}</Button>
        {detail.isFetching && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.page.loading)}</p>}
        {detail.error && <CollaborationQueryState error={detail.error} retry={() => void detail.refetch()} />}
        {detail.data && <CollaborationInformationRecord scope={scope} issueId={issueId} item={detail.data.item} issue={issue} active={active && open}
          onOpen={setSelectedId} onAffectedAttention={affectedStatus} />}
      </section>}
      <ol className="space-y-8">{items.map((item) => <li key={item.handoff_id} data-information-handoff-id={item.handoff_id}>
        <CollaborationInformationRecord scope={scope} issueId={issueId} item={item} issue={issue} active={active && open} onOpen={setSelectedId} onAffectedAttention={affectedStatus} />
      </li>)}</ol>
      {query.hasNextPage && <Button type="button" variant="outline" className="pointer-coarse:min-h-11" disabled={query.isFetching} onClick={() => void query.fetchNextPage()}>{t(($) => $.information.history.more)}</Button>}
    </section>
  </details>;
}

function InformationReference({ id, label, onOpen }: { id: string | null; label: string; onOpen?: (id: string) => void }) {
  const { t } = useT("collaboration");
  if (id === null) return null;
  return <div className="space-y-1 [overflow-wrap:anywhere]"><dt className="text-caption text-muted-foreground">{label}</dt><dd className="space-y-1 text-body">
    <span className="block select-text">{id}</span>{onOpen && <Button type="button" variant="link" className="h-auto min-h-8 max-w-full whitespace-normal px-0 py-1 text-caption pointer-coarse:min-h-11" onClick={() => onOpen(id)}>{t(($) => $.information.history.openRecord)}</Button>}
  </dd></div>;
}

function CollaborationInformationRecord({ scope, issueId, item, issue, active, onOpen, onAffectedAttention }: {
  scope: CollaborationScope; issueId: string; item: CollaborationInformationHistoryItem; issue?: CollaborationIssue; active: boolean;
  onOpen?: (id: string) => void; onAffectedAttention?: AttentionChange;
}) {
  const { t } = useT("collaboration");
  const locale = useLocale();
  const labels = useCollaborationLabels();
  const readable = collaborationInformationHistoryHasDetails(item);
  return <article className="min-w-0 space-y-4 [overflow-wrap:anywhere]">
    <div className="flex min-w-0 flex-wrap items-baseline gap-x-4 gap-y-1">
      <h4 className="text-body font-medium">{t(($) => $.information.history.kinds[item.kind])}</h4>
      <time dateTime={item.created_at} className="text-caption text-muted-foreground">{new Date(item.created_at).toLocaleString(locale)}</time>
      {readable && item.kind !== "information_repair" && <span className="text-caption text-muted-foreground">{t(($) => $.information.history.resolutions[item.resolution])}</span>}
    </div>
    {!readable ? <p className="text-body text-muted-foreground">{t(($) => $.information.history.unavailable)}</p> : <>
      <dl className="grid min-w-0 gap-x-6 gap-y-3 text-body sm:grid-cols-2 [&_dt]:mb-1 [&_dt]:text-caption [&_dt]:text-muted-foreground">
        <div><dt>{t(($) => $.information.history.actor)}</dt><dd className="select-text">{item.actor_user_id}</dd></div>
        <div><dt>{t(($) => $.information.history.previousStatus)}</dt><dd>{labels.status(item.previous_status)}</dd></div>
        {item.from_user_id && <div><dt>{t(($) => $.information.history.from)}</dt><dd className="select-text">{item.from_user_id}</dd></div>}
        {item.to_user_id && <div><dt>{t(($) => $.information.history.to)}</dt><dd className="select-text">{item.to_user_id}</dd></div>}
      </dl>
      <div className="min-w-0 max-w-[75ch] space-y-2"><h5 className="text-caption text-muted-foreground">{t(($) => $.information.history.note)}</h5>
        {item.note.trim() ? <RichContent content={item.note} resourcePolicy="restricted" /> : <p className="text-body text-muted-foreground">{t(($) => $.information.history.noNote)}</p>}
      </div>
      {item.kind === "information_request" && <>
        <CollaborationInformationBlock block={item.block} />
        <dl className="space-y-3">
          {item.requester_user_id && <div className="space-y-1"><dt className="text-caption text-muted-foreground">{t(($) => $.information.history.requester)}</dt><dd className="select-text text-body">{item.requester_user_id}</dd></div>}
          <InformationReference id={item.parent_request_id} label={t(($) => $.information.history.parentRequest)} onOpen={onOpen} />
          <InformationReference id={item.superseded_by_repair_handoff_id} label={t(($) => $.information.history.supersededBy)} onOpen={onOpen} />
        </dl>
      </>}
      {item.kind === "information_return" && <>
        {item.restored_status !== null ? <dl><div className="space-y-1"><dt className="text-caption text-muted-foreground">{t(($) => $.information.history.restoredStatus)}</dt><dd className="text-body">{labels.status(item.restored_status)}</dd></div></dl>
          : <p className="text-body text-muted-foreground">{t(($) => $.information.history.restorationUnavailable)}</p>}
        <p className="text-caption text-muted-foreground">{item.return_status_fallback === true ? t(($) => $.information.history.fallbackUsed)
          : item.return_status_fallback === false ? t(($) => $.information.history.fallbackNotUsed) : t(($) => $.information.history.fallbackUnavailable)}</p>
        <dl><InformationReference id={item.consumed_request_id} label={t(($) => $.information.history.consumedRequest)} onOpen={onOpen} /></dl>
      </>}
      {item.kind === "information_repair" && <>
        <dl className="space-y-3"><div><dt className="text-caption text-muted-foreground">{t(($) => $.information.history.restoredStatus)}</dt><dd className="text-body">{labels.status(item.restored_status)}</dd></div>
          <div><dt className="text-caption text-muted-foreground">{t(($) => $.information.repair.basis)}</dt><dd className="text-body">{item.basis.kind === "human_selected" ? t(($) => $.information.repair.humanBasis)
            : item.basis.kind === "historical_information" ? t(($) => $.information.repair.historicalBasis, { status: labels.status(item.restored_status) }) : t(($) => $.information.repair.pendingBasis)}</dd></div>
          <InformationReference id={item.prior_active_request_id} label={t(($) => $.information.history.priorRequest)} onOpen={onOpen} />
          {item.basis.kind === "historical_information" && <InformationReference id={item.basis.origin_request_id} label={t(($) => $.information.history.basisRequest)} onOpen={onOpen} />}
          {item.basis.kind === "pending_submission" && <div className="space-y-1"><dt className="text-caption text-muted-foreground">{t(($) => $.information.history.basisSubmission)}</dt><dd className="select-text text-body">{item.basis.submission_id}</dd></div>}
        </dl>
        <InformationAffected scope={scope} issueId={issueId} repairId={item.handoff_id} count={item.superseded_request_count} issue={issue} active={active} onOpen={onOpen} onAttentionChange={onAffectedAttention} />
      </>}
    </>}
    <details className="min-w-0 space-y-2"><summary className="cursor-pointer py-2 text-caption text-muted-foreground pointer-coarse:min-h-11">{t(($) => $.information.history.identity)}</summary>
      <dl className="space-y-3 text-caption [&_dt]:mb-1 [&_dt]:text-muted-foreground [&_dd]:select-text">
        <div><dt>{t(($) => $.information.history.recordId)}</dt><dd>{item.handoff_id}</dd></div>
        <div><dt>{t(($) => $.information.history.time)}</dt><dd><time dateTime={item.created_at}>{item.created_at}</time></dd></div>
        {item.provenance.state === "linked" && <><div><dt>{t(($) => $.information.history.eventId)}</dt><dd>{item.provenance.event_id}</dd></div><div><dt>{t(($) => $.information.history.eventSequence)}</dt><dd>{item.provenance.event_sequence}</dd></div></>}
        {readable && item.kind === "information_repair" && item.basis.kind !== "human_selected" && <div><dt>{t(($) => $.information.history.basisEvent)}</dt><dd>{item.basis.kind === "historical_information" ? item.basis.origin_event_id : item.basis.submission_event_id}</dd></div>}
      </dl>
      {item.provenance.state === "legacy_unlinked" && <p className="text-caption text-muted-foreground">{t(($) => $.information.history.unlinked)}</p>}
    </details>
  </article>;
}

function InformationAffected({ scope, issueId, repairId, count, issue, active, onOpen, onAttentionChange }: {
  scope: CollaborationScope; issueId: string; repairId: string; count: string; issue?: CollaborationIssue; active: boolean;
  onOpen?: (id: string) => void; onAttentionChange?: AttentionChange;
}) {
  const { t } = useT("collaboration");
  const client = useQueryClient();
  const key = useId();
  const [open, setOpen] = useState(false);
  const query = useInfiniteQuery({ ...collaborationInformationAffectedOptions(client, scope, issueId, repairId, issue, count), enabled: active && open });
  useEffect(() => { onAttentionChange?.(key, query.isError); }, [key, query.isError, onAttentionChange]);
  useEffect(() => () => onAttentionChange?.(key, false), [key, onAttentionChange]);
  const items = query.data?.pages.flatMap((page) => page.items) ?? [];
  const complete = query.isSuccess && !query.isFetching && !query.hasNextPage && BigInt(items.length) === BigInt(count);
  return <details open={open} onToggle={(event) => setOpen(event.currentTarget.open)} className="min-w-0 space-y-3">
    <summary className="cursor-pointer py-2 text-body font-medium pointer-coarse:min-h-11">{t(($) => $.information.history.affected)}</summary>
    <section aria-label={t(($) => $.information.history.affected)} className="min-w-0 space-y-4">
      <p role="status" className="text-caption text-muted-foreground">{complete ? t(($) => $.information.history.affectedComplete, { total: count }) : t(($) => $.information.history.affectedLoaded, { loaded: String(items.length), total: count })}</p>
      {query.isFetching && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.page.loading)}</p>}
      {query.error && <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />}
      <ol className="space-y-4">{items.map((item) => <li key={item.request_id} data-information-request-id={item.request_id} className="min-w-0 space-y-2 [overflow-wrap:anywhere]">
        <dl><InformationReference id={item.request_id} label={t(($) => $.information.history.requestId)} onOpen={onOpen} /></dl>
        <p className="text-caption text-muted-foreground">{t(($) => $.information.history.resolutions.superseded)}</p>
        {item.provenance.state === "linked" ? <dl className="space-y-2 text-caption [&_dt]:text-muted-foreground"><div><dt>{t(($) => $.information.history.eventId)}</dt><dd className="select-text">{item.provenance.event_id}</dd></div><div><dt>{t(($) => $.information.history.eventSequence)}</dt><dd>{item.provenance.event_sequence}</dd></div></dl>
          : <p className="text-caption text-muted-foreground">{item.provenance.state === "legacy_unlinked" ? t(($) => $.information.history.unlinked) : t(($) => $.information.history.unavailable)}</p>}
      </li>)}</ol>
      {query.hasNextPage && <Button type="button" variant="outline" className="pointer-coarse:min-h-11" disabled={query.isFetching} onClick={() => void query.fetchNextPage()}>{t(($) => $.information.history.moreAffected)}</Button>}
    </section>
  </details>;
}
