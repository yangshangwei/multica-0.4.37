import { useId, useState } from "react";
import { ApiError } from "@multica/core/api";
import { CollaborationUnavailableError, isKnowledgeProtocolUnsupported, prepareProjectKnowledgePolicy, useCollaborationScope, useProjectKnowledgePolicy } from "@multica/core/collaboration";
import { Button } from "@multica/ui/components/ui/button";
import { Switch } from "@multica/ui/components/ui/switch";
import { useT } from "../../i18n";
import { KnowledgeSelect } from "../../collaboration/knowledge-controls";
import { useKnowledgeAutomationControl } from "../../collaboration/knowledge-automation-recovery";

export function ProjectKnowledgePolicy({ workspaceId, projectId }: { workspaceId: string; projectId: string }) {
  const scope = useCollaborationScope();
  return <PolicyEditor key={`${scope.apiScope}:${scope.userId}:${projectId}`} workspaceId={workspaceId} projectId={projectId} />;
}

function PolicyEditor({ workspaceId, projectId }: { workspaceId: string; projectId: string }) {
  const scope = useCollaborationScope(), { t } = useT("collaboration"), id = useId();
  const query = useProjectKnowledgePolicy(scope, workspaceId, projectId);
  const policy = query.data?.value;
  const [draft, setDraft] = useState<{ revision: string; isEnabled: boolean; maxCandidates: number } | null>(null);
  const [invalid, setInvalid] = useState(false);
  const control = useKnowledgeAutomationControl(scope, { kind: "policy", projectId }, () => setDraft(null));
  const denied = query.error instanceof ApiError && [401, 403, 404, 410].includes(query.error.status);
  if (denied && control.pendingCount === 0) return null;
  const changed = draft !== null && policy !== undefined && draft.revision !== policy.revision;
  const isEnabled = draft?.isEnabled ?? policy?.is_enabled ?? false;
  const maxCandidates = draft?.maxCandidates ?? policy?.max_candidates ?? 3;
  const edit = (change: { isEnabled?: boolean; maxCandidates?: number }) => {
    if (!policy || control.locked) return;
    setInvalid(false);
    setDraft((current) => ({ revision: current?.revision ?? policy.revision, isEnabled: current?.isEnabled ?? policy.is_enabled,
      maxCandidates: current?.maxCandidates ?? policy.max_candidates, ...change }));
  };
  const save = () => {
    if (!policy || !draft || changed || control.locked || policy.can_manage !== true) return;
    try { setInvalid(false); void control.submit(prepareProjectKnowledgePolicy(scope, policy, draft)); }
    catch { setInvalid(true); }
  };
  return <section aria-labelledby={`${id}-title`} className="min-w-0 space-y-4 px-2 py-3 pointer-coarse:[&_button]:min-h-11">
    <div className="space-y-1"><h3 id={`${id}-title`} className="text-body font-semibold">{t(($) => $.knowledge.automation.policyTitle)}</h3>
      <p className="text-caption text-muted-foreground">{t(($) => $.knowledge.automation.policyHint)}</p></div>
    {control.notice}
    {!policy && !denied && <div className="space-y-2"><p role={query.isFetching ? "status" : "alert"} className="text-caption text-muted-foreground">
      {isKnowledgeProtocolUnsupported(query.error) || query.error instanceof CollaborationUnavailableError ? t(($) => $.knowledge.protocolUnsupported)
        : query.isFetching ? t(($) => $.knowledge.loading) : t(($) => $.knowledge.automation.policyLoadError)}</p>
      {!query.isFetching && <Button type="button" variant="outline" size="sm" onClick={() => void query.refetch()}>{t(($) => $.knowledge.refresh)}</Button>}</div>}
    {policy && <>
      <p className="text-caption text-muted-foreground">{policy.origin === "system" ? t(($) => $.knowledge.automation.defaultPolicy) : t(($) => $.knowledge.automation.projectPolicy)}</p>
      <div className="flex items-start justify-between gap-4"><label htmlFor={`${id}-enabled`} className="text-body">{t(($) => $.knowledge.automation.enabledLabel)}</label>
        <Switch id={`${id}-enabled`} checked={isEnabled} disabled={control.locked || policy.can_manage !== true} onCheckedChange={(checked) => edit({ isEnabled: checked })} /></div>
      <div className="space-y-2"><p className="text-caption font-medium">{t(($) => $.knowledge.automation.maxLabel)}</p><KnowledgeSelect label={t(($) => $.knowledge.automation.maxLabel)} value={String(maxCandidates)}
        items={[1, 2, 3].map((value) => ({ value: String(value), label: String(value) }))} disabled={control.locked || !isEnabled || policy.can_manage !== true}
        onChange={(value) => edit({ maxCandidates: Number(value) })} /></div>
      {invalid && <p role="alert" className="text-caption text-muted-foreground">{t(($) => $.knowledge.automation.policyChanged)}</p>}
      {changed && <div className="space-y-2"><p role="alert" className="text-caption text-muted-foreground">{t(($) => $.knowledge.automation.policyChanged)}</p>
        <Button type="button" variant="outline" size="sm" disabled={control.locked} onClick={() => { setDraft(null); void query.refetch(); }}>{t(($) => $.knowledge.automation.reloadPolicy)}</Button></div>}
      <Button type="button" variant="outline" disabled={!draft || changed || control.locked || policy.can_manage !== true || draft.isEnabled === policy.is_enabled && draft.maxCandidates === policy.max_candidates} onClick={save}>{control.working ? t(($) => $.knowledge.automation.saving) : t(($) => $.knowledge.automation.save)}</Button>
    </>}
  </section>;
}
