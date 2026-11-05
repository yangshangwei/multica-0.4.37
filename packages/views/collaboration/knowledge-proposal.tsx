import { useEffect, useId, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { buildKnowledgeSources, collaborationTextByteLength, CollaborationUnavailableError, KNOWLEDGE_CONTENT_MAX_BYTES, KNOWLEDGE_REFERENCE_MAX_BYTES, KNOWLEDGE_REFRESH_MS,
  KNOWLEDGE_SUMMARY_MAX_BYTES, prepareKnowledgeProposal, type CollaborationIssue, type CollaborationKnowledgeCandidates,
  type CollaborationKnowledgeProposal, type CollaborationKnowledgeSource, type CollaborationKnowledgeTarget, type CollaborationScope, type KnowledgeIntent,
  type KnowledgeSourceSelection } from "@multica/core/collaboration";
import { workspaceListOptions } from "@multica/core/workspace/queries";
import { projectListOptions } from "@multica/core/projects/queries";
import { Button } from "@multica/ui/components/ui/button";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useT } from "../i18n";
import { KnowledgeByteLimit, KnowledgeSelect } from "./knowledge-controls";
import { KnowledgeSourcePicker } from "./knowledge-source-picker";

export interface KnowledgeProposalMode { target?: CollaborationKnowledgeTarget; supersedesId?: string; derivedId?: string }

/** Join the panel's in-flight list read, bounded by one refresh interval so a paused offline query cannot hang submit. */
async function awaitRefreshedView(refreshView: () => Promise<CollaborationKnowledgeCandidates | undefined>) {
  let timer: ReturnType<typeof setTimeout> | undefined;
  try {
    return await Promise.race([refreshView().catch(() => undefined),
      new Promise<undefined>((resolve) => { timer = setTimeout(() => resolve(undefined), KNOWLEDGE_REFRESH_MS); })]);
  } finally { clearTimeout(timer); }
}

function OtherTarget({ disabled, onChange }: { disabled: boolean; onChange: (target: CollaborationKnowledgeTarget | null) => void }) {
  const { t } = useT("collaboration");
  const workspaces = useQuery(workspaceListOptions());
  const [workspaceId, setWorkspaceId] = useState("");
  const visible = workspaces.isSuccess && workspaces.data.some((workspace) => workspace.id === workspaceId);
  const projects = useQuery({ ...projectListOptions(workspaceId), enabled: visible });
  const [projectId, setProjectId] = useState("");
  const projectVisible = visible && projects.isSuccess && projects.data.some((project) => project.id === projectId);
  const unavailable = workspaceId !== "" && workspaces.isSuccess && !visible || visible && projectId !== "" && projects.isSuccess && !projectVisible;
  useEffect(() => {
    if (!visible || projectId && !projectVisible) onChange(null);
    else onChange(projectId ? { kind: "project", workspace_id: workspaceId, project_id: projectId } : { kind: "workspace", workspace_id: workspaceId });
  }, [workspaceId, visible, projectId, projectVisible, onChange]);
  return <div className="space-y-3">
    <KnowledgeSelect label={t(($) => $.knowledge.workspace)} value={visible ? workspaceId : ""} disabled={disabled || !workspaces.isSuccess}
      items={[{ value: "", label: t(($) => $.knowledge.chooseWorkspace) }, ...(workspaces.isSuccess ? workspaces.data : []).map((workspace) => ({ value: workspace.id, label: workspace.name }))]}
      onChange={(value) => { setWorkspaceId(value); setProjectId(""); }} />
    {workspaces.isPending && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.knowledge.workspacesLoading)}</p>}
    {workspaces.isError && <div role="alert" className="space-y-2 text-caption"><p>{t(($) => $.knowledge.workspacesLoadError)}</p>
      <Button type="button" variant="outline" className="pointer-coarse:min-h-11" disabled={workspaces.isFetching} onClick={() => void workspaces.refetch()}>{t(($) => $.knowledge.refresh)}</Button></div>}
    {workspaces.isSuccess && workspaces.data.length === 0 && <p className="text-caption text-muted-foreground">{t(($) => $.knowledge.workspacesEmpty)}</p>}
    {visible && <>
      <KnowledgeSelect label={t(($) => $.knowledge.project)} value={projectId && !projectVisible ? "unavailable" : projectId} disabled={disabled || !projects.isSuccess}
        items={[...(projectId && !projectVisible ? [{ value: "unavailable", label: t(($) => $.knowledge.chooseProject) }] : []),
          { value: "", label: t(($) => $.knowledge.workspaceTarget) }, ...(projects.isSuccess ? projects.data : []).map((project) => ({ value: project.id, label: project.title }))]}
        onChange={(value) => { if (value !== "unavailable") setProjectId(value); }} />
      {projects.isPending && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.knowledge.projectsLoading)}</p>}
      {projects.isError && <div role="alert" className="space-y-2 text-caption"><p>{t(($) => $.knowledge.projectsLoadError)}</p>
        <Button type="button" variant="outline" className="pointer-coarse:min-h-11" disabled={projects.isFetching} onClick={() => void projects.refetch()}>{t(($) => $.knowledge.refresh)}</Button></div>}
      {projects.isSuccess && projects.data.length === 0 && <p className="text-caption text-muted-foreground">{t(($) => $.knowledge.projectsEmpty)}</p>}
    </>}
    {unavailable && <p role="alert" className="text-caption">{t(($) => $.knowledge.targetUnavailable)}</p>}
  </div>;
}

export function KnowledgeProposalForm({ scope, issue, view, refreshView, mode, locked, onSubmit, onCancel }: {
  scope: CollaborationScope; issue: CollaborationIssue; view: CollaborationKnowledgeCandidates | undefined;
  refreshView: () => Promise<CollaborationKnowledgeCandidates | undefined>;
  mode: KnowledgeProposalMode; locked: boolean; onSubmit: (intent: KnowledgeIntent) => Promise<void>; onCancel: () => void;
}) {
  const { t } = useT("collaboration"); const client = useQueryClient(); const id = useId();
  const [summary, setSummary] = useState(""), [content, setContent] = useState(""), [expiry, setExpiry] = useState("");
  const [evidence, setEvidence] = useState(""), [verification, setVerification] = useState<"unverified" | "observed" | "verified">("unverified");
  const [evidenceKind, setEvidenceKind] = useState<"test" | "commit">("test");
  const [selected, setSelected] = useState<KnowledgeSourceSelection[]>([]);
  const [surfaces, setSurfaces] = useState<CollaborationKnowledgeProposal["applicability"]["surfaces"]>(["issue", "collaboration"]);
  const [destination, setDestination] = useState("workspace"), [otherTarget, setOtherTarget] = useState<CollaborationKnowledgeTarget | null>(null);
  const [building, setBuilding] = useState(false), [error, setError] = useState<string | null>(null);
  const submitting = useRef(false);
  const project = issue.source_structure?.project;
  const projectChanged = destination !== "workspace" && destination !== "other" && destination !== project?.id;
  const target = mode.target ?? (destination === "other" ? otherTarget : destination === "workspace"
    ? { kind: "workspace" as const, workspace_id: issue.source_workspace.id }
    : project && destination === project.id ? { kind: "project" as const, workspace_id: issue.source_workspace.id, project_id: project.id } : null);
  const targetIdentity = JSON.stringify(target);
  const alive = useRef({ scope, view, targetIdentity }); alive.current = { scope, view, targetIdentity };
  const disabled = locked || building;
  // Limits count UTF-8 bytes like the schema and server; DOM maxLength counts UTF-16 units and truncates paste silently.
  const summaryOver = collaborationTextByteLength(summary) > KNOWLEDGE_SUMMARY_MAX_BYTES;
  const contentOver = collaborationTextByteLength(content) > KNOWLEDGE_CONTENT_MAX_BYTES;
  const evidenceOver = collaborationTextByteLength(evidence) > KNOWLEDGE_REFERENCE_MAX_BYTES;
  const overLimit = summaryOver || contentOver || evidenceOver;
  // A refreshing (erased) list does not block submit; only a current, explicit non-grant or a field over its limit does.
  const blocked = disabled || overLimit || !target || view !== undefined && view.capabilities.can_propose !== true;
  return <form aria-label={t(($) => $.knowledge.propose)} className="min-w-0 space-y-5" onSubmit={(event) => {
    event.preventDefault(); if (blocked || submitting.current) return;
    if (!target || !summary.trim() || !content.trim() || !surfaces.length || !selected.length || expiry && (!Number.isFinite(Date.parse(expiry)) || Date.parse(expiry) <= Date.now())) { setError(t(($) => $.knowledge.invalid)); return; }
    submitting.current = true; setBuilding(true); setError(null);
    void (async () => {
      let sources: CollaborationKnowledgeSource[];
      try { sources = await buildKnowledgeSources(client, scope, issue.id, selected); }
      catch { setError(t(($) => $.knowledge.sourceChanged)); return; }
      // Prepare only from a current authorized list; never reuse a grant a refresh erased.
      const current = alive.current.view ?? await awaitRefreshedView(refreshView);
      if (alive.current.scope !== scope || current?.capabilities.can_propose !== true) { setError(t(($) => $.knowledge.proposalUnavailable)); return; }
      if (alive.current.targetIdentity !== targetIdentity) { setError(t(($) => $.knowledge.targetUnavailable)); return; }
      let intent: KnowledgeIntent;
      try {
        intent = prepareKnowledgeProposal(scope, current, { target, summary, content, sources,
          evidence: evidence.trim() ? [{ kind: evidenceKind, source: sources[0]!, reference: evidence, verification }] : [],
          applicability: { surfaces }, expires_at: expiry ? new Date(expiry).toISOString() : null,
          supersedes_candidate_id: mode.supersedesId ?? null, derived_from_candidate_id: mode.derivedId ?? null });
      } catch (error) { setError(error instanceof CollaborationUnavailableError ? t(($) => $.knowledge.proposalUnavailable) : t(($) => $.knowledge.invalid)); return; }
      await onSubmit(intent);
    })().catch(() => setError(t(($) => $.knowledge.proposalUnavailable))).finally(() => { setBuilding(false); submitting.current = false; });
  }}>
    {(mode.supersedesId || mode.derivedId) && <p className="text-caption text-muted-foreground">{t(($) => $.knowledge.blankRevision)}</p>}
    <div className="space-y-2"><label htmlFor={`${id}-summary`} className="text-body font-medium">{t(($) => $.knowledge.summary)}</label>
      <Input id={`${id}-summary`} autoFocus value={summary} onChange={(event) => setSummary(event.target.value)} disabled={disabled} required className="pointer-coarse:min-h-11"
        aria-invalid={summaryOver || undefined} aria-describedby={summaryOver ? `${id}-summary-limit` : undefined} />
      <KnowledgeByteLimit id={`${id}-summary-limit`} value={summary} max={KNOWLEDGE_SUMMARY_MAX_BYTES} /></div>
    <div className="space-y-2"><label htmlFor={`${id}-content`} className="text-body font-medium">{t(($) => $.knowledge.content)}</label>
      <Textarea id={`${id}-content`} value={content} onChange={(event) => setContent(event.target.value)} disabled={disabled} required rows={7}
        aria-invalid={contentOver || undefined} aria-describedby={contentOver ? `${id}-content-limit` : undefined} />
      <KnowledgeByteLimit id={`${id}-content-limit`} value={content} max={KNOWLEDGE_CONTENT_MAX_BYTES} /></div>
    <div className="space-y-2"><p className="text-body font-medium">{t(($) => $.knowledge.target)}</p>
      {mode.target ? <p className="text-body">{t(($) => $.knowledge[mode.target!.kind])}</p> : <>
        <KnowledgeSelect label={t(($) => $.knowledge.target)} value={projectChanged ? "" : destination} onChange={(value) => { setDestination(value); setOtherTarget(null); }} disabled={disabled}
          items={[{ value: "workspace", label: t(($) => $.knowledge.sourceWorkspace, { name: issue.source_workspace.name }) },
            ...(project ? [{ value: project.id, label: t(($) => $.knowledge.sourceProject, { name: project.name || t(($) => $.knowledge.currentProject) }) }] : []),
            { value: "other", label: t(($) => $.knowledge.otherTarget) }]} />
        {projectChanged && <p role="alert" className="text-caption text-destructive">{t(($) => $.knowledge.targetChanged)}</p>}
        {destination === "other" && <OtherTarget disabled={disabled} onChange={setOtherTarget} />}
      </>}
    </div>
    <KnowledgeSourcePicker scope={scope} issue={issue} selected={selected} onChange={setSelected} disabled={disabled} />
    <div className="space-y-2"><label htmlFor={`${id}-evidence`} className="text-body font-medium">{t(($) => $.knowledge.evidence)}</label>
      <Textarea id={`${id}-evidence`} value={evidence} onChange={(event) => setEvidence(event.target.value)} disabled={disabled} rows={2}
        aria-invalid={evidenceOver || undefined} aria-describedby={evidenceOver ? `${id}-evidence-limit` : undefined} />
      <KnowledgeByteLimit id={`${id}-evidence-limit`} value={evidence} max={KNOWLEDGE_REFERENCE_MAX_BYTES} />
      <p className="text-caption text-muted-foreground">{t(($) => $.knowledge.evidenceHint)}</p>
      {evidence.trim() && <><KnowledgeSelect label={t(($) => $.knowledge.evidenceType)} value={evidenceKind} disabled={disabled}
        items={(["test", "commit"] as const).map((value) => ({ value, label: t(($) => $.knowledge.evidenceKinds[value]) }))}
        onChange={(value) => { if (value === "test" || value === "commit") setEvidenceKind(value); }} />
        <KnowledgeSelect label={t(($) => $.knowledge.verification)} value={verification} disabled={disabled}
        items={(["unverified", "observed", "verified"] as const).map((value) => ({ value, label: t(($) => $.knowledge[value]) }))}
        onChange={(value) => { if (value === "unverified" || value === "observed" || value === "verified") setVerification(value); }} /></>}
    </div>
    <fieldset disabled={disabled} className="space-y-2"><legend className="mb-2 text-body font-medium">{t(($) => $.knowledge.applies)}</legend>
      <div className="flex flex-wrap gap-x-6 gap-y-3">{(["issue", "chat", "quick_create", "autopilot", "collaboration"] as const).map((surface) => <label key={surface} className="flex cursor-pointer items-center gap-2 text-body pointer-coarse:min-h-11">
        <Checkbox checked={surfaces.includes(surface)} onCheckedChange={(checked) => setSurfaces((current) => checked ? [...current, surface] : current.filter((value) => value !== surface))} />{t(($) => $.knowledge.surfaces[surface])}
      </label>)}</div>
    </fieldset>
    <div className="space-y-2"><label htmlFor={`${id}-expiry`} className="text-body font-medium">{t(($) => $.knowledge.expires)}</label>
      <Input id={`${id}-expiry`} type="datetime-local" value={expiry} onChange={(event) => setExpiry(event.target.value)} disabled={disabled} className="pointer-coarse:min-h-11" /></div>
    {error && <p role="alert" className="text-body text-destructive">{error}</p>}
    <div className="flex flex-wrap gap-2"><Button type="submit" className="pointer-coarse:min-h-11" disabled={blocked}>{disabled ? t(($) => $.knowledge.submitting) : t(($) => $.knowledge.submit)}</Button>
      <Button type="button" variant="outline" className="pointer-coarse:min-h-11" disabled={disabled} onClick={onCancel}>{t(($) => $.knowledge.cancel)}</Button></div>
  </form>;
}
