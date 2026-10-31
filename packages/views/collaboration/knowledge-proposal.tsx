import { useEffect, useId, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { buildKnowledgeSources, prepareKnowledgeProposal, type CollaborationIssue, type CollaborationKnowledgeCandidates,
  type CollaborationKnowledgeProposal, type CollaborationKnowledgeTarget, type CollaborationScope, type KnowledgeIntent, type KnowledgeSourceSelection } from "@multica/core/collaboration";
import { workspaceListOptions } from "@multica/core/workspace/queries";
import { projectListOptions } from "@multica/core/projects/queries";
import { Button } from "@multica/ui/components/ui/button";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useT } from "../i18n";
import { KnowledgeSelect } from "./knowledge-controls";
import { KnowledgeSourcePicker } from "./knowledge-source-picker";

export interface KnowledgeProposalMode { target?: CollaborationKnowledgeTarget; supersedesId?: string; derivedId?: string }

function OtherTarget({ disabled, onChange }: { disabled: boolean; onChange: (target: CollaborationKnowledgeTarget | null) => void }) {
  const { t } = useT("collaboration");
  const workspaces = useQuery(workspaceListOptions());
  const [workspaceId, setWorkspaceId] = useState("");
  const visible = workspaces.data?.some((workspace) => workspace.id === workspaceId) === true && !workspaces.error;
  const projects = useQuery({ ...projectListOptions(workspaceId), enabled: visible });
  const [projectId, setProjectId] = useState("");
  useEffect(() => { if (workspaceId && !visible) onChange(null); }, [workspaceId, visible, onChange]);
  return <div className="space-y-3">
    <KnowledgeSelect label={t(($) => $.knowledge.workspace)} value={visible ? workspaceId : ""} disabled={disabled}
      items={[{ value: "", label: t(($) => $.knowledge.chooseWorkspace) }, ...(!workspaces.error ? workspaces.data ?? [] : []).map((workspace) => ({ value: workspace.id, label: workspace.name }))]}
      onChange={(value) => { setWorkspaceId(value); setProjectId(""); onChange(value ? { kind: "workspace", workspace_id: value } : null); }} />
    {visible && <KnowledgeSelect label={t(($) => $.knowledge.project)} value={projectId} disabled={disabled}
      items={[{ value: "", label: t(($) => $.knowledge.workspaceTarget) }, ...(!projects.error ? projects.data ?? [] : []).map((project) => ({ value: project.id, label: project.title }))]}
      onChange={(value) => { setProjectId(value); onChange(value ? { kind: "project", workspace_id: workspaceId, project_id: value } : { kind: "workspace", workspace_id: workspaceId }); }} />}
    {(workspaces.error || projects.error) && <p role="alert" className="text-caption">{t(($) => $.knowledge.loadError)}</p>}
  </div>;
}

export function KnowledgeProposalForm({ scope, issue, view, mode, locked, onSubmit, onCancel }: {
  scope: CollaborationScope; issue: CollaborationIssue; view: CollaborationKnowledgeCandidates | undefined;
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
  const submitting = useRef(false); const alive = useRef({ scope, view }); alive.current = { scope, view };
  const project = issue.source_structure?.project;
  const projectChanged = destination !== "workspace" && destination !== "other" && destination !== project?.id;
  const target = mode.target ?? (destination === "other" ? otherTarget : destination === "workspace"
    ? { kind: "workspace" as const, workspace_id: issue.source_workspace.id }
    : project && destination === project.id ? { kind: "project" as const, workspace_id: issue.source_workspace.id, project_id: project.id } : null);
  const disabled = locked || building;
  return <form aria-label={t(($) => $.knowledge.propose)} className="min-w-0 space-y-5" onSubmit={(event) => {
    event.preventDefault(); if (disabled || submitting.current || !view || view.capabilities.can_propose !== true) return;
    if (!target || !summary.trim() || !content.trim() || !surfaces.length || !selected.length || expiry && (!Number.isFinite(Date.parse(expiry)) || Date.parse(expiry) <= Date.now())) { setError(t(($) => $.knowledge.invalid)); return; }
    submitting.current = true; setBuilding(true); setError(null);
    void (async () => {
      const sources = await buildKnowledgeSources(client, scope, issue.id, selected);
      const current = alive.current;
      if (current.scope !== scope || !current.view || current.view.capabilities.can_propose !== true) throw new Error("Source authorization changed");
      const intent = prepareKnowledgeProposal(scope, current.view, { target, summary, content, sources,
        evidence: evidence.trim() ? [{ kind: evidenceKind, source: sources[0]!, reference: evidence, verification }] : [],
        applicability: { surfaces }, expires_at: expiry ? new Date(expiry).toISOString() : null,
        supersedes_candidate_id: mode.supersedesId ?? null, derived_from_candidate_id: mode.derivedId ?? null });
      await onSubmit(intent);
    })().catch(() => setError(t(($) => $.knowledge.sourceChanged))).finally(() => { setBuilding(false); submitting.current = false; });
  }}>
    {(mode.supersedesId || mode.derivedId) && <p className="text-caption text-muted-foreground">{t(($) => $.knowledge.blankRevision)}</p>}
    <div className="space-y-2"><label htmlFor={`${id}-summary`} className="text-body font-medium">{t(($) => $.knowledge.summary)}</label>
      <Input id={`${id}-summary`} autoFocus value={summary} onChange={(event) => setSummary(event.target.value)} disabled={disabled} required maxLength={2048} /></div>
    <div className="space-y-2"><label htmlFor={`${id}-content`} className="text-body font-medium">{t(($) => $.knowledge.content)}</label>
      <Textarea id={`${id}-content`} value={content} onChange={(event) => setContent(event.target.value)} disabled={disabled} required rows={7} /></div>
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
      <Textarea id={`${id}-evidence`} value={evidence} onChange={(event) => setEvidence(event.target.value)} disabled={disabled} rows={2} maxLength={2048} />
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
      <Input id={`${id}-expiry`} type="datetime-local" value={expiry} onChange={(event) => setExpiry(event.target.value)} disabled={disabled} /></div>
    {error && <p role="alert" className="text-body text-destructive">{error}</p>}
    <div className="flex flex-wrap gap-2"><Button type="submit" className="pointer-coarse:min-h-11" disabled={disabled || !view || view.capabilities.can_propose !== true}>{disabled ? t(($) => $.knowledge.submitting) : t(($) => $.knowledge.submit)}</Button>
      <Button type="button" variant="outline" disabled={disabled} onClick={onCancel}>{t(($) => $.knowledge.cancel)}</Button></div>
  </form>;
}
