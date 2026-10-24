import { useId, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { canOfferCollaborationResources, canOrganizeCollaborationResources, collaborationResourceOfferOptions, collaborationResourceOffersOptions,
  collaborationResourceRoster, prepareCollaborationResource, type CollaborationDraft, type CollaborationIssue, type CollaborationResourceOffer,
  type CollaborationResources, type CollaborationResourceOffersPage, type CollaborationScope } from "@multica/core/collaboration";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../i18n";
import { useCollaborationLabels } from "./labels";
import { CollaborationQueryState } from "./query-state";
import { CollaborationResourceCandidateForm, CollaborationResourceConfirmation, CollaborationResourceSummary, type CollaborationExecutionSubmit } from "./resource-form";

type Selection = { action: "offer"; issue: CollaborationIssue; resources: CollaborationResources; page: CollaborationResourceOffersPage }
  | { action: "adopt_resource_offer" | "withdraw_resource_offer"; offer: CollaborationResourceOffer; draft: CollaborationDraft };

function OfferConfirmation({ scope, selection, locked, submit, close }: {
  scope: CollaborationScope; selection: Extract<Selection, { offer: CollaborationResourceOffer }>; locked: boolean; submit: CollaborationExecutionSubmit; close: () => void;
}) {
  const { t } = useT("collaboration");
  const labels = useCollaborationLabels();
  const client = useQueryClient();
  const [refused, setRefused] = useState(false);
  const query = useQuery(collaborationResourceOfferOptions(client, scope, selection.offer.issue_id, selection.offer.offer_id));
  const current = query.isSuccess ? query.data : null;
  const original = selection.offer;
  const unchanged = current !== null && current.version === original.version && current.state === "offered" && current.kind === original.kind &&
    current.resource_id === original.resource_id && current.execution_workspace_id === original.execution_workspace_id &&
    current.offered_by?.user_id === original.offered_by?.user_id && JSON.stringify(collaborationResourceRoster(current)) === JSON.stringify(collaborationResourceRoster(original));
  const adopt = selection.action === "adopt_resource_offer";
  const allowed = adopt ? current?.can_adopt === true : current?.can_withdraw === true && current.offered_by?.user_id.toLowerCase() === scope.userId;
  const title = adopt ? t(($) => $.resources.adoptOffer) : t(($) => $.resources.withdrawOffer);
  return <CollaborationResourceConfirmation title={title} effect={adopt ? t(($) => $.resources.offerConfirmation) : t(($) => $.resources.withdrawOfferEffect)}
    confirmLabel={title} draft={selection.draft} locked={locked} disabled={refused || query.isFetching || !unchanged || !allowed} preserveOnRefusal close={close}
    submit={async (draft) => { const result = await submit(draft); if (result && result.status >= 400) setRefused(true); return result; }}>
    {query.isFetching && <p role="status">{t(($) => $.page.loading)}</p>}
    {query.error && <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />}
    {current && <p className="text-body [overflow-wrap:anywhere]">{original.name || original.resource_id}</p>}
    {current && !refused && (!unchanged || !allowed) && <p role="status" className="text-body">{current.reason ? labels.reason(current.reason) : t(($) => $.resources.staleOffer)}</p>}
    {current && <ul className="space-y-3">{original.resources.map((resource) => <li key={resource.agent_id} className="min-w-0 space-y-1">
      <p className="text-body [overflow-wrap:anywhere]">{resource.agent_name || resource.agent_id} · {t(($) => $.execution[resource.role])}</p>
      <CollaborationResourceSummary resource={resource} />
    </li>)}</ul>}
  </CollaborationResourceConfirmation>;
}

export function CollaborationResourceOffers({ scope, issue, resources, locked, submit }: {
  scope: CollaborationScope; issue: CollaborationIssue; resources: CollaborationResources; locked: boolean; submit: CollaborationExecutionSubmit;
}) {
  const { t } = useT("collaboration");
  const labels = useCollaborationLabels();
  const client = useQueryClient();
  const headingId = useId();
  const [cursors, setCursors] = useState<string[]>([]);
  const [selection, setSelection] = useState<Selection | null>(null);
  const [error, setError] = useState<unknown>(null);
  const query = useQuery(collaborationResourceOffersOptions(client, scope, issue.id, cursors.at(-1) ?? null));
  const page = query.isSuccess ? query.data : null;
  const organizer = issue.capabilities.invite === true && [issue.creator.user_id, issue.handler.user_id].includes(scope.userId);
  const canOrganize = canOrganizeCollaborationResources(scope, issue) && resources.can_organize === true;
  const canSupply = canOfferCollaborationResources(scope, issue);
  const canOffer = canSupply && page?.can_offer === true;
  const coherent = resources.issue_id === issue.id.toLowerCase() && resources.lifecycle_epoch === issue.lifecycle_epoch && resources.target_set_version === issue.execution?.target_set_version;
  // Drop only local selection when its action is no longer permitted. The
  // independently persisted operation keeps its original recovery identity.
  if (selection && (selection.action === "offer" && !canSupply || selection.action === "adopt_resource_offer" && !canOrganize ||
    selection.action === "withdraw_resource_offer" && selection.offer.offered_by?.user_id.toLowerCase() !== scope.userId)) setSelection(null);
  const items = page?.items.filter((offer) => organizer || offer.offered_by?.user_id.toLowerCase() === scope.userId) ?? [];
  const close = () => setSelection(null);
  const select = (action: "adopt_resource_offer" | "withdraw_resource_offer", offer: CollaborationResourceOffer) => {
    try {
      setError(null);
      setSelection({ action, offer, draft: prepareCollaborationResource(scope, issue, resources, crypto.randomUUID(), { action, offer }) });
    } catch (failure) { setError(failure); }
  };
  return <section aria-labelledby={headingId} className="min-w-0 space-y-4">
    <h4 id={headingId} className="text-body font-semibold">{t(($) => $.resources.offersTitle)}</h4>
    <p className="text-caption text-muted-foreground">{t(($) => $.resources.offerDescription)}</p>
    <div className="flex flex-wrap gap-2">
      {canOffer && <Button variant="outline" disabled={locked || query.isFetching || selection !== null} className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11 [overflow-wrap:anywhere]"
        onClick={() => { setError(null); setSelection({ action: "offer", issue, resources, page: page! }); }}>{t(($) => $.resources.offer)}</Button>}
      <Button variant="ghost" disabled={locked || query.isFetching} className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11 [overflow-wrap:anywhere]"
        onClick={() => { setCursors([]); void client.invalidateQueries({ queryKey: collaborationResourceOffersOptions(client, scope, issue.id).queryKey.slice(0, -1) }); }}>{t(($) => $.page.refresh)}</Button>
    </div>
    {query.isPending && <p role="status">{t(($) => $.page.loading)}</p>}
    {query.error && <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />}
    {error != null && <CollaborationQueryState error={error} />}
    {page && items.length === 0 && <p className="text-body text-muted-foreground">{t(($) => $.resources.noOffers)}</p>}
    <ul className="space-y-5">{items.map((offer) => <li key={offer.offer_id} className="min-w-0 space-y-2">
      <p className="text-body font-medium [overflow-wrap:anywhere]">{offer.name || offer.resource_id}</p>
      <p className="text-caption text-muted-foreground [overflow-wrap:anywhere]">{t(($) => $.resources.offeredBy, { name: offer.offered_by?.name ?? t(($) => $.resources.unknownOwner) })}</p>
      <p className="text-caption">{t(($) => $.resources.offerStates[offer.state])}</p>
      {offer.state === "offered" && offer.reason !== "" && <p className="text-caption text-muted-foreground">{labels.reason(offer.reason)}</p>}
      <div className="flex flex-wrap gap-2">
        {canOrganize && offer.can_adopt === true && <Button variant="outline" disabled={locked || query.isFetching || !coherent || selection !== null} className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11 [overflow-wrap:anywhere]"
          onClick={() => select("adopt_resource_offer", offer)}>{t(($) => $.resources.adoptOffer)}</Button>}
        {offer.offered_by?.user_id.toLowerCase() === scope.userId && offer.can_withdraw === true && <Button variant="outline" disabled={locked || query.isFetching || selection !== null} className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11 [overflow-wrap:anywhere]"
          onClick={() => select("withdraw_resource_offer", offer)}>{t(($) => $.resources.withdrawOffer)}</Button>}
      </div>
    </li>)}</ul>
    {(cursors.length > 0 || page?.next_cursor) && <div className="flex flex-wrap gap-2">
      {cursors.length > 0 && <Button variant="outline" disabled={locked || query.isFetching || selection !== null} className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11 [overflow-wrap:anywhere]"
        onClick={() => setCursors(cursors.slice(0, -1))}>{t(($) => $.resources.previousOffers)}</Button>}
      {page?.next_cursor && !cursors.includes(page.next_cursor) && <Button variant="outline" disabled={locked || query.isFetching || selection !== null} className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11 [overflow-wrap:anywhere]"
        onClick={() => setCursors([...cursors, page.next_cursor!])}>{t(($) => $.resources.moreOffers)}</Button>}
    </div>}
    {selection?.action === "offer" && canOffer && <CollaborationResourceCandidateForm scope={scope} issueId={issue.id} purpose="offer" title={t(($) => $.resources.offer)} confirmLabel={t(($) => $.resources.offer)} locked={locked} close={close}
      refreshBaseline={() => { if (page) setSelection({ action: "offer", issue, resources, page }); }}
      submit={async (candidate, executionWorkspaceId) => {
        try {
          const draft = prepareCollaborationResource(scope, selection.issue, selection.resources, crypto.randomUUID(), { action: "offer_resource", candidate, executionWorkspaceId, offers: selection.page });
          const result = await submit(draft);
          if (!result || result.status < 400) close();
          return result;
        } catch (failure) { setError(failure); return undefined; }
      }} />}
    {selection && selection.action !== "offer" && (selection.action === "adopt_resource_offer" ? canOrganize : selection.offer.offered_by?.user_id.toLowerCase() === scope.userId) &&
      <OfferConfirmation key={selection.draft.requestId} scope={scope} selection={selection} locked={locked} submit={submit} close={close} />}
  </section>;
}
