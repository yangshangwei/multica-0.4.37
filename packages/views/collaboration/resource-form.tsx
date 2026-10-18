import { useId, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { collaborationResourceIdentityOptions, collaborationResourceOptionsOptions, collaborationResourceWorkspacesOptions,
  type CollaborationDraft, type CollaborationResourceIdentityView, type CollaborationResourceOption, type CollaborationResourceView, type CollaborationScope } from "@multica/core/collaboration";
import { runtimeDisplayLabel } from "@multica/core/runtimes";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../i18n";
import { useCollaborationLabels } from "./labels";
import { CollaborationQueryState } from "./query-state";

export type CollaborationExecutionSubmit = (draft: CollaborationDraft, recover?: boolean) => Promise<{ status: number; code: string } | undefined>;

export function CollaborationResourceSummary({ resource }: { resource: CollaborationResourceView }) {
  const { t } = useT("collaboration");
  return <dl className="space-y-1 text-caption text-muted-foreground [overflow-wrap:anywhere]">
    <div><dt className="inline">{t(($) => $.resources.owner)}: </dt><dd className="inline">{resource.owner?.name ?? t(($) => $.resources.unknownOwner)}</dd></div>
    <div><dt className="inline">{t(($) => $.resources.runtime)}: </dt><dd className="inline">{resource.runtime ? runtimeDisplayLabel(resource.runtime) : t(($) => $.resources.unknownRuntime)}</dd></div>
  </dl>;
}

export function CollaborationResourceCandidateForm({ scope, issueId, title, confirmLabel, target, locked, submit, close }: {
  scope: CollaborationScope; issueId: string; title: string; confirmLabel: string; target?: { kind: string; resource_id: string };
  locked: boolean; submit: (candidate: CollaborationResourceOption) => Promise<void>; close: () => void;
}) {
  const { t } = useT("collaboration");
  const labels = useCollaborationLabels();
  const client = useQueryClient();
  const fieldId = useId();
  const [workspaceId, setWorkspaceId] = useState("");
  const [selection, setSelection] = useState<CollaborationResourceOption | null>(null);
  const workspaces = useQuery(collaborationResourceWorkspacesOptions(client, scope));
  const candidates = useQuery(collaborationResourceOptionsOptions(client, scope, issueId, workspaceId));
  const options = candidates.data?.items.filter((item) => !target || item.kind === target.kind && item.resource_id === target.resource_id) ?? [];
  const ready = !locked && selection?.available === true && candidates.isSuccess && !candidates.isFetching;
  return <form className="min-w-0 space-y-4 py-3" onSubmit={(event) => { event.preventDefault(); if (ready && selection) void submit(selection); }}>
    <h4 className="text-body font-semibold">{title}</h4>
    {target && <p className="text-caption text-muted-foreground">{t(($) => $.resources.proposeHint)}</p>}
    <fieldset disabled={locked} className="min-w-0 space-y-4">
      <div className="space-y-2"><label htmlFor={`${fieldId}-workspace`} className="text-body">{t(($) => $.resources.workspace)}</label>
        <select id={`${fieldId}-workspace`} value={workspaceId} onChange={(event) => { setWorkspaceId(event.target.value); setSelection(null); }} className="block min-h-11 w-full rounded-md border border-input bg-background px-3 text-body focus-visible:ring-2 focus-visible:ring-ring">
          <option value="">{t(($) => $.resources.chooseWorkspace)}</option>
          {workspaces.data?.map((workspace) => <option key={workspace.id} value={workspace.id}>{workspace.name}</option>)}
        </select>
        {workspaces.isPending && <p role="status">{t(($) => $.page.loading)}</p>}
        {workspaces.error && <CollaborationQueryState error={workspaces.error} retry={() => void workspaces.refetch()} />}
        {workspaces.isSuccess && workspaces.data.length === 0 && <p className="text-caption text-muted-foreground">{t(($) => $.resources.noWorkspaces)}</p>}
      </div>
      {workspaceId && <div className="space-y-2"><label htmlFor={`${fieldId}-target`} className="text-body">{t(($) => $.resources.target)}</label>
        <select id={`${fieldId}-target`} value={selection ? `${selection.kind}:${selection.resource_id}` : ""} onChange={(event) => setSelection(options.find((item) => `${item.kind}:${item.resource_id}` === event.target.value) ?? null)} disabled={candidates.isFetching || !candidates.isSuccess} className="block min-h-11 w-full rounded-md border border-input bg-background px-3 text-body focus-visible:ring-2 focus-visible:ring-ring">
          <option value="">{t(($) => $.resources.chooseTarget)}</option>
          {options.map((option) => <option key={`${option.kind}:${option.resource_id}`} value={`${option.kind}:${option.resource_id}`}>{option.name} · {option.kind === "agent" ? t(($) => $.execution.agent) : t(($) => $.execution.squad)}</option>)}
        </select>
        {candidates.isFetching && <p role="status">{t(($) => $.page.loading)}</p>}
        {candidates.error && <CollaborationQueryState error={candidates.error} retry={() => void candidates.refetch()} />}
        {candidates.isSuccess && options.length === 0 && <p className="text-caption text-muted-foreground">{t(($) => $.resources.noCandidates)}</p>}
      </div>}
      {selection && <div className="space-y-3">
        {selection.available !== true && <p role="status" className="text-body">{labels.reason(selection.reason)}</p>}
        <ul className="space-y-3">{selection.resources.map((resource) => <li key={resource.agent_id} className="min-w-0 space-y-1">
          <p className="text-body [overflow-wrap:anywhere]">{resource.agent_name || resource.agent_id} · {t(($) => $.execution[resource.role])}</p>
          <CollaborationResourceSummary resource={resource} />
        </li>)}</ul>
      </div>}
    </fieldset>
    <div className="flex flex-wrap gap-2"><Button type="submit" disabled={!ready} className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11 [overflow-wrap:anywhere]">{confirmLabel}</Button>
      <Button type="button" variant="ghost" disabled={locked} onClick={close} className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11 [overflow-wrap:anywhere]">{t(($) => $.workflow.dismiss)}</Button></div>
  </form>;
}

export function CollaborationResourceConsentForm({ scope, issueId, targetId, agentId, proposalId = null, name, locked, prepare, submit, close }: {
  scope: CollaborationScope; issueId: string; targetId: string; agentId: string; proposalId?: string | null; name: string; locked: boolean;
  prepare: (identity: CollaborationResourceIdentityView) => CollaborationDraft; submit: CollaborationExecutionSubmit; close: () => void;
}) {
  const { t } = useT("collaboration");
  const labels = useCollaborationLabels();
  const client = useQueryClient();
  const [error, setError] = useState<unknown>(null);
  const identity = useQuery(collaborationResourceIdentityOptions(client, scope, issueId, targetId, agentId, proposalId));
  const ready = !locked && identity.isSuccess && !identity.isFetching && identity.data.identity !== null;
  return <form className="min-w-0 space-y-3 py-3" onSubmit={(event) => {
    event.preventDefault();
    if (!ready || !identity.data) return;
    try { const draft = prepare(identity.data); void submit(draft).then(close); } catch (failure) { setError(failure); }
  }}>
    <h5 className="text-body font-semibold [overflow-wrap:anywhere]">{t(($) => $.resources.authorize, { name })}</h5>
    <p className="text-caption text-muted-foreground">{t(($) => $.resources.consentPurpose)}</p>
    {identity.isFetching && <p role="status">{t(($) => $.page.loading)}</p>}
    {identity.error && <CollaborationQueryState error={identity.error} retry={() => void identity.refetch()} />}
    {error != null && <CollaborationQueryState error={error} />}
    {identity.data?.identity === null && <p role="status" className="text-body">{labels.reason(identity.data.reason)}</p>}
    <div className="flex flex-wrap gap-2"><Button type="submit" disabled={!ready} className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11 [overflow-wrap:anywhere]">{t(($) => $.resources.confirmAuthorization)}</Button>
      <Button type="button" variant="ghost" disabled={locked} onClick={close} className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11 [overflow-wrap:anywhere]">{t(($) => $.workflow.dismiss)}</Button></div>
  </form>;
}

export function CollaborationResourceConfirmation({ title, effect, confirmLabel, draft, locked, submit, close }: {
  title: string; effect: string; confirmLabel: string; draft: CollaborationDraft; locked: boolean; submit: CollaborationExecutionSubmit; close: () => void;
}) {
  const { t } = useT("collaboration");
  return <form className="min-w-0 space-y-3 py-3" onSubmit={(event) => { event.preventDefault(); if (!locked) void submit(draft).then(close); }}>
    <h5 className="text-body font-semibold">{title}</h5><p className="text-caption text-muted-foreground">{effect}</p>
    <div className="flex flex-wrap gap-2"><Button type="submit" disabled={locked} className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11 [overflow-wrap:anywhere]">{confirmLabel}</Button><Button type="button" variant="ghost" disabled={locked} onClick={close} className="h-auto min-h-8 max-w-full whitespace-normal py-1.5 pointer-coarse:min-h-11 [overflow-wrap:anywhere]">{t(($) => $.workflow.dismiss)}</Button></div>
  </form>;
}
