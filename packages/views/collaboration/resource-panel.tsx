import { useId, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { canOrganizeCollaborationResources, collaborationResourcesOptions, prepareCollaborationResource,
  type CollaborationDraft, type CollaborationIssue, type CollaborationResources, type CollaborationScope } from "@multica/core/collaboration";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../i18n";
import { useCollaborationLabels } from "./labels";
import { CollaborationQueryState } from "./query-state";
import { CollaborationResourceCandidateForm, CollaborationResourceConfirmation, CollaborationResourceConsentForm, CollaborationResourceSummary, type CollaborationExecutionSubmit } from "./resource-form";

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
  const close = () => setSelection(null);
  return <div className="min-w-0 space-y-4">
    <Button variant="outline" className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11 [overflow-wrap:anywhere]" aria-expanded={open} aria-controls={contentId} onClick={() => setOpen(!open)}>{t(($) => $.resources.manage)}</Button>
    {open && <div id={contentId} className="min-w-0 space-y-4">
      <p className="text-caption text-muted-foreground">{t(($) => $.resources.purpose)}</p>
      {query.isPending && <p role="status">{t(($) => $.page.loading)}</p>}
      {query.error && <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />}
      {error != null && <CollaborationQueryState error={error} />}
      {query.data && <div className="flex flex-wrap gap-2">
        {canOrganize && <Button variant="outline" disabled={locked || query.isFetching || selection !== null} className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11 [overflow-wrap:anywhere]" onClick={() => { setError(null); setSelection({ action: "register", issue, resources: query.data }); }}>{t(($) => $.resources.add)}</Button>}
        <Button variant="ghost" disabled={query.isFetching || locked} className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11 [overflow-wrap:anywhere]" onClick={() => void query.refetch()}>{t(($) => $.resources.refresh)}</Button>
      </div>}
      {query.isSuccess && query.data.targets.length === 0 && <p className="text-body text-muted-foreground">{t(($) => $.resources.empty)}</p>}
      <ul className="space-y-5">{query.data?.targets.map((target) => <li key={target.target_id} className="min-w-0 space-y-3">
        <div className="space-y-1"><h4 className="text-body font-semibold [overflow-wrap:anywhere]">{target.name || target.resource_id}</h4>
          {target.reason !== "" && <p className="text-caption text-muted-foreground">{labels.reason(target.reason)}</p>}</div>
        <ul className="space-y-3">{target.resources.map((resource) => <li key={resource.agent_id} className="min-w-0 space-y-2">
          <p className="text-body [overflow-wrap:anywhere]">{resource.agent_name || resource.agent_id} · {t(($) => $.execution[resource.role])}</p>
          <CollaborationResourceSummary resource={resource} />
          <p className="text-caption">{resource.consented ? t(($) => $.execution.consented) : t(($) => $.execution.consentMissing)}</p>
          {resource.owner_user_id === scope.userId && <div className="flex flex-wrap gap-2">
            {resource.can_consent === true && <Button variant="outline" size="sm" className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11 [overflow-wrap:anywhere]" disabled={locked || query.isFetching || selection !== null} onClick={() => setSelection({ action: "consent", issue, resources: query.data!, targetId: target.target_id, agentId: resource.agent_id, name: resource.agent_name || resource.agent_id })}>{t(($) => $.resources.authorize, { name: resource.agent_name || resource.agent_id })}</Button>}
            {resource.can_revoke === true && <Button variant="outline" size="sm" className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11 [overflow-wrap:anywhere]" disabled={locked || query.isFetching || selection !== null} onClick={() => {
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
      {selection?.action === "consent" && <CollaborationResourceConsentForm scope={scope} issueId={issue.id} targetId={selection.targetId} agentId={selection.agentId} name={selection.name} locked={locked} close={close} submit={submit}
        prepare={(identity) => prepareCollaborationResource(scope, selection.issue, selection.resources, crypto.randomUUID(), { action: "consent_resource", targetId: selection.targetId, agentId: selection.agentId, identity })} />}
      {selection?.action === "revoke" && <CollaborationResourceConfirmation title={t(($) => $.resources.withdrawTitle)} effect={t(($) => $.resources.withdrawEffect)} confirmLabel={t(($) => $.resources.confirmWithdraw)} draft={selection.draft} locked={locked} close={close} submit={submit} />}
    </div>}
  </div>;
}
