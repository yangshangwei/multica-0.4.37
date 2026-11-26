"use client";

import { useId, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useAuthStore } from "@multica/core/auth";
import { canAssignAgentToIssue } from "@multica/core/permissions";
import { projectResourcesOptions } from "@multica/core/projects";
import { githubInstallationsOptions } from "@multica/core/github/queries";
import { vcsConnectionsOptions } from "@multica/core/vcs/queries";
import { agentListOptions, memberListOptions } from "@multica/core/workspace/queries";
import { useSaveMaterialSettings, materialDraftKey, decodeMaterialSettingsInput, type MaterialSettingsEnvelope, type MaterialSettingsInput } from "@multica/core/materials";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Switch } from "@multica/ui/components/ui/switch";
import { useT } from "../../i18n";
import { MaterialActionNotice, MaterialSelect, useMaterialAction } from "./material-controls";

type Draft = Omit<MaterialSettingsInput, "request_id">;
function initialDraft(envelope: MaterialSettingsEnvelope): Draft {
  const settings = envelope.settings;
  return {
    expected_version: settings?.version ?? 0, enabled: settings?.enabled ?? true,
    provider: settings?.provider === "gitlab" ? "gitlab" : "github", resource_id: settings?.resource_id ?? "",
    connection_id: settings?.connection_id ?? null, repository: settings?.repository ?? "", base_branch: settings?.base_branch ?? "main",
    root_path: settings?.root_path ?? "", reviewer_ids: settings?.reviewer_ids ?? [], plan_agent_id: settings?.plan_agent_id ?? null,
    auto_start_plan: settings?.auto_start_plan ?? true,
  };
}

export function ProjectMaterialSettings({ workspaceId, projectId, envelope, onClose }: {
  workspaceId: string; projectId: string; envelope: MaterialSettingsEnvelope; onClose: () => void;
}) {
  const { t } = useT("projects");
  const id = useId();
  const userId = useAuthStore((state) => state.user?.id);
  const [draft, setDraft] = useState(() => initialDraft(envelope));
  const resources = useQuery(projectResourcesOptions(workspaceId, projectId));
  const members = useQuery(memberListOptions(workspaceId));
  const agents = useQuery(agentListOptions(workspaceId));
  const github = useQuery({ ...githubInstallationsOptions(workspaceId), enabled: draft.provider === "github" });
  const gitlab = useQuery({ ...vcsConnectionsOptions(workspaceId), enabled: draft.provider === "gitlab" });
  const mutation = useSaveMaterialSettings(workspaceId, projectId);
  const action = useMaterialAction(mutation.mutateAsync, materialDraftKey(userId, workspaceId, projectId, "settings"), decodeMaterialSettingsInput, onClose);
  const supported = !envelope.settings || ["github", "gitlab"].includes(envelope.settings.provider);
  const changed = draft.expected_version !== (envelope.settings?.version ?? 0);
  const connectionQuery = draft.provider === "github" ? github : gitlab;
  const prerequisites = [resources, members, agents, connectionQuery];
  const inputsFailed = prerequisites.some((query) => query.isError);
  const inputsPending = prerequisites.some((query) => query.isPending);
  const disabled = action.locked || !envelope.can_configure || !supported;
  const role = members.data?.find((member) => member.user_id === userId)?.role ?? null;
  const connections = draft.provider === "github"
    ? (github.data?.installations ?? []).map((item) => ({ value: item.id, label: item.account_login }))
    : (gitlab.data?.connections ?? []).filter((item) => item.provider === "gitlab").map((item) => ({ value: item.id, label: `${item.account_login} · ${item.instance_url}` }));
  const resourceOptions = (resources.data ?? []).filter((resource) => resource.resource_type === "github_repo").map((resource) => ({
    value: resource.id, label: resource.label || ("url" in resource.resource_ref && typeof resource.resource_ref.url === "string" ? resource.resource_ref.url : resource.id),
  }));
  const agentOptions = (agents.data ?? []).filter((agent) => !agent.archived_at && canAssignAgentToIssue(agent, { userId: userId ?? null, role }).allowed)
    .map((agent) => ({ value: agent.id, label: agent.name }));
  const set = (change: Partial<Draft>) => setDraft((current) => ({ ...current, ...change }));
  const choose = { value: "", label: t(($) => $.materials.choose) };
  const valid = !!draft.resource_id && !!draft.connection_id && !!draft.repository.trim() && !!draft.base_branch.trim() && draft.reviewer_ids.length > 0;
  const resourceChanged = (resourceId: string) => {
    const resource = resources.data?.find((item) => item.id === resourceId);
    const url = resource && "url" in resource.resource_ref && typeof resource.resource_ref.url === "string" ? resource.resource_ref.url : "";
    // A path suggestion is convenience only; the server resolves and verifies the binding.
    const repository = url.replace(/^[\w+.-]+:\/\/[^/]+\//, "").replace(/^[^@]+@[^:]+:/, "").replace(/\.git\/?$/, "").replace(/\/$/, "");
    set({ resource_id: resourceId, repository: repository !== url ? repository : draft.repository });
  };
  return <form className="mx-auto w-full max-w-3xl space-y-6" onSubmit={(event) => {
    event.preventDefault();
    if (!valid || disabled || inputsFailed || inputsPending || changed) return;
    action.submit({ ...draft, request_id: crypto.randomUUID() }, onClose);
  }}>
    <div className="space-y-2"><h3 className="text-title font-semibold">{t(($) => $.materials.settings)}</h3><p className="text-body text-muted-foreground">{t(($) => $.materials.repositoryHint)}</p></div>
    <MaterialActionNotice error={action.error} uncertain={action.pending !== null} busy={action.busy} onRetry={action.retry} />
    {!supported && <p role="alert" className="text-body text-muted-foreground">{t(($) => $.materials.unsupported)}</p>}
    {inputsFailed && <div role="alert" className="space-y-2 text-caption text-destructive"><p>{t(($) => $.materials.inputsError)}</p><Button type="button" variant="outline" onClick={() => { prerequisites.forEach((query) => { void query.refetch(); }); }}>{t(($) => $.materials.refresh)}</Button></div>}
    <div className="flex items-center justify-between gap-4"><label htmlFor={`${id}-enabled`} className="text-body font-medium">{t(($) => $.materials.enabled)}</label><Switch id={`${id}-enabled`} checked={draft.enabled} disabled={disabled} onCheckedChange={(enabled) => set({ enabled })} /></div>
    <fieldset disabled={disabled} className="grid min-w-0 gap-5 sm:grid-cols-2">
      <MaterialSelect id={`${id}-provider`} label={t(($) => $.materials.provider)} value={draft.provider} disabled={disabled} options={[{ value: "github", label: "GitHub" }, { value: "gitlab", label: "GitLab" }]} onChange={(provider) => { if (provider === "github" || provider === "gitlab") set({ provider, connection_id: null }); }} />
      <MaterialSelect id={`${id}-resource`} label={t(($) => $.materials.resource)} value={draft.resource_id} disabled={disabled || resources.isPending} options={[choose, ...resourceOptions]} onChange={resourceChanged} />
      <div className="space-y-2 sm:col-span-2"><MaterialSelect id={`${id}-connection`} label={t(($) => $.materials.connection)} value={draft.connection_id ?? ""} disabled={disabled || connectionQuery.isPending} options={[choose, ...connections]} onChange={(connection_id) => set({ connection_id: connection_id || null })} />
        {!connectionQuery.isPending && !connectionQuery.isError && connections.length === 0 && <p className="text-caption text-muted-foreground">{t(($) => $.materials.noConnections)}</p>}</div>
      <div className="space-y-2"><label htmlFor={`${id}-repository`} className="text-caption font-medium">{t(($) => $.materials.repository)}</label><Input id={`${id}-repository`} value={draft.repository} onChange={(event) => set({ repository: event.target.value })} required /></div>
      <div className="space-y-2"><label htmlFor={`${id}-branch`} className="text-caption font-medium">{t(($) => $.materials.baseBranch)}</label><Input id={`${id}-branch`} value={draft.base_branch} onChange={(event) => set({ base_branch: event.target.value })} required /></div>
      <div className="space-y-2 sm:col-span-2"><label htmlFor={`${id}-root`} className="text-caption font-medium">{t(($) => $.materials.rootPath)}</label><Input id={`${id}-root`} aria-describedby={`${id}-root-hint`} value={draft.root_path} onChange={(event) => set({ root_path: event.target.value })} /><p id={`${id}-root-hint`} className="text-caption text-muted-foreground">{t(($) => $.materials.rootHint)}</p></div>
    </fieldset>
    <fieldset disabled={disabled} className="space-y-3"><legend className="text-body font-medium">{t(($) => $.materials.reviewers)}</legend><p className="text-caption text-muted-foreground">{t(($) => $.materials.reviewersHint)}</p>
      <div className="grid gap-2 sm:grid-cols-2">{members.data?.map((member) => <label key={member.user_id} className="flex min-h-9 min-w-0 items-center gap-3 text-body"><input type="checkbox" className="size-4 accent-primary" checked={draft.reviewer_ids.includes(member.user_id)} onChange={(event) => set({ reviewer_ids: event.target.checked ? [...draft.reviewer_ids, member.user_id] : draft.reviewer_ids.filter((value) => value !== member.user_id) })} /><span className="truncate">{member.name}</span></label>)}</div>
    </fieldset>
    <div className="space-y-2"><MaterialSelect id={`${id}-agent`} label={t(($) => $.materials.planAgent)} value={draft.plan_agent_id ?? ""} disabled={disabled || agents.isPending} options={[{ value: "", label: t(($) => $.materials.noAgent) }, ...agentOptions]} onChange={(plan_agent_id) => set({ plan_agent_id: plan_agent_id || null })} /><p className="text-caption text-muted-foreground">{t(($) => $.materials.planAgentHint)}</p></div>
    <div className="space-y-2"><div className="flex items-center justify-between gap-4"><label htmlFor={`${id}-auto`} className="text-body font-medium">{t(($) => $.materials.autoStart)}</label><Switch id={`${id}-auto`} checked={draft.auto_start_plan} disabled={disabled} onCheckedChange={(auto_start_plan) => set({ auto_start_plan })} aria-describedby={`${id}-auto-hint`} /></div><p id={`${id}-auto-hint`} className="max-w-prose text-caption text-muted-foreground">{t(($) => $.materials.autoStartHint)}</p></div>
    {changed && <div role="alert" className="space-y-2 text-caption text-destructive"><p>{t(($) => $.materials.configChanged)}</p><Button type="button" variant="outline" disabled={action.locked} onClick={() => setDraft(initialDraft(envelope))}>{t(($) => $.materials.reload)}</Button></div>}
    <div className="flex flex-wrap gap-2"><Button type="submit" disabled={!valid || disabled || changed || inputsFailed || inputsPending}>{action.busy ? t(($) => $.materials.saving) : t(($) => $.materials.save)}</Button><Button type="button" variant="ghost" disabled={action.locked} onClick={onClose}>{t(($) => $.materials.cancel)}</Button></div>
  </form>;
}
