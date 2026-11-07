import { useId, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { canOfferCollaborationResources, canOrganizeCollaborationResources, collaborationKeys, collaborationResourcesOptions, prepareCollaborationResource,
  type CollaborationDraft, type CollaborationIssue, type CollaborationResources, type CollaborationScope } from "@multica/core/collaboration";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../i18n";
import { useCollaborationLabels } from "./labels";
import { CollaborationQueryState } from "./query-state";
import { CollaborationResourceCandidateForm, CollaborationResourceConfirmation, CollaborationResourceConsentForm, CollaborationResourceSummary, type CollaborationExecutionSubmit } from "./resource-form";
import { CollaborationResourceOffers } from "./resource-offers";

type Selection = { issue: CollaborationIssue; resources: CollaborationResources } & (
  | { action: "register" }
  | { action: "consent"; targetId: string; agentId: string; name: string }
  | { action: "revoke"; draft: CollaborationDraft }
);

export function CollaborationResourcePanel({ scope, issue, locked, submit }: { scope: CollaborationScope; issue: CollaborationIssue; locked: boolean; submit: CollaborationExecutionSubmit }) {
  const { t } = useT("collaboration");
  const labels = useCollaborationLabels();
  const client = useQueryClient();
  const contentId = useId();
  const [open, setOpen] = useState(false);
  const [selection, setSelection] = useState<Selection | null>(null);
  const [error, setError] = useState<unknown>(null);
  const query = useQuery({ ...collaborationResourcesOptions(client, scope, issue.id), enabled: open });
  const canOrganize = canOrganizeCollaborationResources(scope, issue) && query.data?.can_organize === true;
  const coherent = query.isSuccess && query.data.issue_id === issue.id.toLowerCase() && query.data.lifecycle_epoch === issue.lifecycle_epoch &&
    query.data.target_set_version === issue.execution?.target_set_version;
  const currentTarget = (target: CollaborationResources["targets"][number]) => coherent && issue.execution?.targets.some((item) =>
    item.id === target.target_id && item.kind === target.kind && item.resource_id === target.resource_id && item.target_epoch === target.target_epoch && item.config_version === target.config_version);
  const selectedTarget = selection?.action === "consent" ? selection.resources.targets.find((target) => target.target_id === selection.targetId) : undefined;
  const selectedResource = selection?.action === "consent" ? query.data?.targets.find((target) => target.target_id === selection.targetId)?.resources.find((resource) => resource.agent_id === selection.agentId) : undefined;
  const consentCurrent = selection?.action === "consent" && selectedTarget !== undefined && currentTarget(selectedTarget) && canOfferCollaborationResources(scope, issue) &&
    selection.issue.lifecycle_epoch === issue.lifecycle_epoch && selection.issue.write_context?.account_epoch === issue.write_context?.account_epoch &&
    selection.issue.write_context?.participant_epoch === issue.write_context?.participant_epoch && selectedResource?.can_consent === true && selectedResource.owner_user_id === scope.userId;
  // This is unsubmitted UI state; pending operations live in the core draft store.
  if (selection?.action === "register" && !canOrganizeCollaborationResources(scope, issue)) setSelection(null);
  const close = () => setSelection(null);
  const refresh = () => {
    if (locked) return;
    close();
    setError(null);
    void client.invalidateQueries({ queryKey: collaborationKeys.issue(scope, issue.id) });
  };
  return <div className="min-w-0 space-y-4">
    <Button variant="outline" className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11 [overflow-wrap:anywhere]" aria-expanded={open} aria-controls={open ? contentId : undefined} onClick={() => {
      setOpen(!open);
      if (!open) void client.invalidateQueries({ queryKey: collaborationKeys.issue(scope, issue.id), exact: true });
    }}>{t(($) => $.resources.manage)}</Button>
    {open && <div id={contentId} className="min-w-0 space-y-4">
      <p className="text-caption text-muted-foreground">{t(($) => $.resources.purpose)}</p>
      {query.isPending && <p role="status">{t(($) => $.page.loading)}</p>}
      {query.error && <CollaborationQueryState error={query.error} retry={refresh} />}
      {error != null && <CollaborationQueryState error={error} />}
      {query.data && <div className="flex flex-wrap gap-2">
        {canOrganize && <Button variant="outline" disabled={locked || query.isFetching || !coherent || selection !== null} className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11 [overflow-wrap:anywhere]" onClick={() => { setError(null); setSelection({ action: "register", issue, resources: query.data }); }}>{t(($) => $.resources.add)}</Button>}
        <Button variant="ghost" disabled={query.isFetching || locked} className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11 [overflow-wrap:anywhere]" onClick={refresh}>{t(($) => $.resources.refresh)}</Button>
      </div>}
      {query.isSuccess && query.data.targets.length === 0 && <p className="text-body text-muted-foreground">{t(($) => $.resources.empty)}</p>}
      <ul className="space-y-5">{query.data?.targets.map((target) => <li key={target.target_id} className="min-w-0 space-y-3">
        <div className="space-y-1"><h4 className="text-body font-semibold [overflow-wrap:anywhere]">{target.name || target.resource_id}</h4>
          {target.reason !== "" && <p className="text-caption text-muted-foreground">{labels.reason(target.reason, target.resources.some((resource) => resource.execution_mode === "host_process_supervision_v1") ? "host_process_supervision_v1" : undefined)}</p>}</div>
        <ul className="space-y-3">{target.resources.map((resource) => <li key={resource.agent_id} className="min-w-0 space-y-2">
          <p className="text-body [overflow-wrap:anywhere]">{resource.agent_name || resource.agent_id} · {t(($) => $.execution[resource.role])}</p>
          <CollaborationResourceSummary resource={resource} />
          <p className="text-caption">{resource.consented ? t(($) => $.execution.consented) : t(($) => $.execution.consentMissing)}</p>
          {resource.owner_user_id === scope.userId && <div className="flex flex-wrap gap-2">
            {resource.can_consent === true && <Button variant="outline" size="sm" className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11 [overflow-wrap:anywhere]" disabled={locked || query.isFetching || !currentTarget(target) || selection !== null} onClick={() => setSelection({ action: "consent", issue, resources: query.data!, targetId: target.target_id, agentId: resource.agent_id, name: resource.agent_name || resource.agent_id })}>{t(($) => $.resources.authorize, { name: resource.agent_name || resource.agent_id })}</Button>}
            {resource.can_revoke === true && <Button variant="outline" size="sm" className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11 [overflow-wrap:anywhere]" disabled={locked || query.isFetching || !currentTarget(target) || selection !== null} onClick={() => {
              try { setSelection({ action: "revoke", issue, resources: query.data!, draft: prepareCollaborationResource(scope, issue, query.data!, crypto.randomUUID(), { action: "revoke_grant", targetId: target.target_id, agentId: resource.agent_id }) }); }
              catch (failure) { setError(failure); }
            }}>{t(($) => $.resources.withdraw, { name: resource.agent_name || resource.agent_id })}</Button>}
          </div>}
        </li>)}</ul>
      </li>)}</ul>
      {selection?.action === "register" && canOrganize && <CollaborationResourceCandidateForm scope={scope} issueId={issue.id} title={t(($) => $.resources.add)} confirmLabel={t(($) => $.resources.register)} locked={locked} close={close} submit={async (candidate) => {
        try { await submit(prepareCollaborationResource(scope, selection.issue, selection.resources, crypto.randomUUID(), { action: "register_target", candidate })); close(); }
        catch (failure) { setError(failure); }
      }} />}
      {selection?.action === "consent" && <CollaborationResourceConsentForm scope={scope} issueId={issue.id} targetId={selection.targetId} agentId={selection.agentId} name={selection.name} locked={locked} disabled={!consentCurrent} close={close} submit={submit}
        executionViewVersion={issue.execution?.protocol_version}
        resource={selectedTarget?.resources.find((resource) => resource.agent_id === selection.agentId)}
        prepare={(identity) => prepareCollaborationResource(scope, selection.issue, selection.resources, crypto.randomUUID(), { action: "consent_resource", targetId: selection.targetId, agentId: selection.agentId, identity })} />}
      {selection?.action === "revoke" && <CollaborationResourceConfirmation title={t(($) => $.resources.withdrawTitle)} effect={t(($) => $.resources.withdrawEffect)} confirmLabel={t(($) => $.resources.confirmWithdraw)} draft={selection.draft} locked={locked} close={close} submit={submit} />}
      {query.isSuccess && <CollaborationResourceOffers scope={scope} issue={issue} resources={query.data} locked={locked || selection !== null} submit={submit} />}
    </div>}
  </div>;
}
