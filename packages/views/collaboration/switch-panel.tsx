import { useId, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { canOrganizeCollaborationResources, collaborationResourceRoster, collaborationSwitchProposalsOptions, prepareCollaborationSwitch,
  type CollaborationDraft, type CollaborationIssue, type CollaborationScope, type CollaborationSwitchProposal } from "@multica/core/collaboration";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../i18n";
import { useCollaborationLabels } from "./labels";
import { CollaborationQueryState } from "./query-state";
import { CollaborationResourceCandidateForm, CollaborationResourceConfirmation, CollaborationResourceConsentForm, CollaborationResourceSummary, type CollaborationExecutionSubmit } from "./resource-form";

type Selection = { issue: CollaborationIssue } & (
  | { action: "propose" }
  | { action: "consent"; proposal: CollaborationSwitchProposal; agentId: string; name: string }
  | { action: "revoke" | "commit"; draft: CollaborationDraft }
);

export function CollaborationSwitchPanel({ scope, issue, targetId, locked, submit }: {
  scope: CollaborationScope; issue: CollaborationIssue; targetId: string; locked: boolean; submit: CollaborationExecutionSubmit;
}) {
  const { t } = useT("collaboration");
  const labels = useCollaborationLabels();
  const client = useQueryClient();
  const contentId = useId();
  const [open, setOpen] = useState(false);
  const [selection, setSelection] = useState<Selection | null>(null);
  const [error, setError] = useState<unknown>(null);
  const query = useQuery({ ...collaborationSwitchProposalsOptions(client, scope, issue.id, targetId), enabled: open });
  const target = issue.execution?.targets.find((item) => item.id === targetId);
  const canOrganize = canOrganizeCollaborationResources(scope, issue);
  const close = () => setSelection(null);
  if (!target) return null;
  return <div className="min-w-0 space-y-3">
    <Button variant="ghost" size="sm" className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11 [overflow-wrap:anywhere]" aria-expanded={open} aria-controls={contentId} onClick={() => setOpen(!open)}>{t(($) => $.execution.switch)}</Button>
    {open && <div id={contentId} className="min-w-0 space-y-4">
      {query.isPending && <p role="status">{t(($) => $.page.loading)}</p>}
      {query.error && <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />}
      {error != null && <CollaborationQueryState error={error} />}
      {canOrganize && <Button variant="outline" disabled={locked || selection !== null} className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11 [overflow-wrap:anywhere]" onClick={() => setSelection({ action: "propose", issue })}>{t(($) => $.resources.propose)}</Button>}
      {query.isSuccess && query.data.length === 0 && <p className="text-caption text-muted-foreground">{t(($) => $.execution.noProposals)}</p>}
      <ul className="space-y-5">{query.data?.map((proposal) => <li key={proposal.proposal_id} className="min-w-0 space-y-3">
        <h5 className="text-body font-medium">{t(($) => $.resources.proposalVersion, { version: proposal.proposed_config_version })} · {labels.execution(proposal.status)}</h5>
        <ul className="space-y-3">{proposal.resources.map((resource) => <li key={resource.agent_id} className="min-w-0 space-y-2">
          <p className="text-body [overflow-wrap:anywhere]">{t(($) => $.execution[resource.role])} · {resource.agent_name || resource.agent_id}</p>
          <CollaborationResourceSummary resource={resource} />
          <p className="text-caption">{resource.revoked ? t(($) => $.execution.consentRevoked) : resource.consented ? t(($) => $.execution.consented) : t(($) => $.execution.consentMissing)}</p>
          {resource.owner_user_id === scope.userId && proposal.status !== "committed" && <div className="flex flex-wrap gap-2">
            {!resource.consented && !resource.revoked && proposal.status !== "stale" && <Button variant="outline" size="sm" disabled={locked || query.isFetching || selection !== null} className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11 [overflow-wrap:anywhere]" onClick={() => setSelection({ action: "consent", issue, proposal, agentId: resource.agent_id, name: resource.agent_name || resource.agent_id })}>{t(($) => $.resources.authorize, { name: resource.agent_name || resource.agent_id })}</Button>}
            {resource.consented && !resource.revoked && <Button variant="outline" size="sm" disabled={locked || query.isFetching || selection !== null} className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11 [overflow-wrap:anywhere]" onClick={() => {
              try { setSelection({ action: "revoke", issue, draft: prepareCollaborationSwitch(scope, issue, crypto.randomUUID(), { action: "revoke_target_switch_consent", targetId, proposalId: proposal.proposal_id, agentId: resource.agent_id }, proposal) }); }
              catch (failure) { setError(failure); }
            }}>{t(($) => $.resources.switchWithdraw)}</Button>}
          </div>}
        </li>)}</ul>
        {canOrganize && proposal.status === "consent_ready" && <div className="space-y-2">
          <Button variant="outline" disabled={locked || query.isFetching || selection !== null || target.hold_reason === null && target.control.queue_state !== "paused"} className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11 [overflow-wrap:anywhere]" onClick={() => {
            try { setSelection({ action: "commit", issue, draft: prepareCollaborationSwitch(scope, issue, crypto.randomUUID(), { action: "commit_target_switch", targetId, proposalId: proposal.proposal_id }, proposal) }); }
            catch (failure) { setError(failure); }
          }}>{t(($) => $.resources.commit)}</Button>
          {target.hold_reason === null && target.control.queue_state !== "paused" && <p className="text-caption text-muted-foreground">{t(($) => $.resources.pauseRequired)}</p>}
        </div>}
      </li>)}</ul>
      {selection?.action === "propose" && canOrganize && <CollaborationResourceCandidateForm scope={scope} issueId={issue.id} title={t(($) => $.resources.propose)} confirmLabel={t(($) => $.resources.createProposal)} target={target} locked={locked} close={close} submit={async (candidate) => {
        try { await submit(prepareCollaborationSwitch(scope, selection.issue, crypto.randomUUID(), { action: "propose_target_switch", targetId, roster: collaborationResourceRoster(candidate) })); close(); }
        catch (failure) { setError(failure); }
      }} />}
      {selection?.action === "consent" && <CollaborationResourceConsentForm scope={scope} issueId={issue.id} targetId={targetId} agentId={selection.agentId} proposalId={selection.proposal.proposal_id} name={selection.name} locked={locked} close={close} submit={submit}
        executionViewVersion={issue.execution?.protocol_version}
        resource={selection.proposal.resources.find((resource) => resource.agent_id === selection.agentId)}
        prepare={(identity) => prepareCollaborationSwitch(scope, selection.issue, crypto.randomUUID(), { action: "consent_target_switch", targetId, proposalId: selection.proposal.proposal_id, agentId: selection.agentId, identity: identity.identity! }, selection.proposal)} />}
      {selection?.action === "revoke" && <CollaborationResourceConfirmation title={t(($) => $.resources.switchWithdraw)} effect={t(($) => $.resources.switchWithdrawEffect)} confirmLabel={t(($) => $.resources.switchWithdraw)} draft={selection.draft} locked={locked} close={close} submit={submit} />}
      {selection?.action === "commit" && <CollaborationResourceConfirmation title={t(($) => $.resources.commitTitle)} effect={t(($) => $.resources.commitEffect)} confirmLabel={t(($) => $.resources.confirmCommit)} draft={selection.draft} locked={locked} close={close} submit={submit} />}
    </div>}
  </div>;
}
