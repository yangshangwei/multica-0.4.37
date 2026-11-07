"use client";
import { useEffect, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, errorCode } from "@multica/core/api";
import { projectDetailOptions } from "@multica/core/projects/queries";
import { projectUpdatesOptions, projectUpdateRevisionsOptions, projectExecutionEvidenceOptions, usePreviewProjectUpdate, usePublishProjectUpdate,
  projectProgressDraftKey, useProjectProgressDraftStore, writeProjectProgressDraft, emptyProjectUpdateDraft,
  prepareProjectUpdateIntent, clearProjectProgressDraft, canAccessProject, registerProjectLocalTextFlush, projectSessionGeneration } from "@multica/core/projects";
import type { Project, ProjectUpdate, ProjectUpdateDraft, ProjectUpdateRevision, ProjectHistoricalMember, ProjectEvidenceInput, ProjectEvidenceView } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { ChevronDown, ClipboardCheck, History, ShieldAlert, TrendingUp } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { ContentEditor, type ContentEditorRef } from "../../editor";
import { RevisionConflictCompare } from "../../issues/components/revision-conflict-compare";
import { useWorkspacePaths } from "@multica/core/paths";
import { AppLink } from "../../navigation";
import { AgentTranscriptDialog, buildTimeline } from "../../common/task-transcript";
import { useT } from "../../i18n";
import { useProjectAccessGuard } from "./use-project-access-guard";

function HistoricalName({ member }: { member: ProjectHistoricalMember }) {
  const { t } = useT("projects");
  return <span>{member.availability === "deleted" ? t(($) => $.management.deleted_member) : member.name ?? member.id}
    {member.availability === "departed" && <span className="ml-1 text-muted-foreground">({t(($) => $.management.departed)})</span>}</span>;
}
function ExecutionEvidence({ evidence, revision }: { evidence: ProjectEvidenceView; revision: ProjectUpdateRevision }) {
  const { t } = useT("projects"); const qc = useQueryClient(); const [open, setOpen] = useState(false); const [sourceDenied, setSourceDenied] = useState(false);
  const options = projectExecutionEvidenceOptions(revision.workspace_id, revision.project_id, revision.update_id, revision.revision, evidence.input.id!);
  const query = useQuery({ ...projectExecutionEvidenceOptions(revision.workspace_id, revision.project_id, revision.update_id, revision.revision, evidence.input.id!), enabled: open });
  useEffect(() => {
    if (errorCode(query.error) === "project_evidence_forbidden") {
      qc.removeQueries({ queryKey: options.queryKey }); setOpen(false); setSourceDenied(true);
    }
  }, [query.error, qc, options.queryKey]);
  return <><Button size="sm" variant="link" onClick={() => { setSourceDenied(false); setOpen(true); }}>{evidence.label ?? t(($) => $.management.open_execution)}</Button>
    {sourceDenied && <span role="alert" className="text-caption text-destructive">{t(($) => $.management.source_forbidden)}</span>}
    {open && query.error && <p role="alert" className="text-destructive">{query.error.message}</p>}
    {open && query.isPending && <span role="status">{t(($) => $.management.loading)}</span>}
    {open && !query.error && query.data && <AgentTranscriptDialog open onOpenChange={setOpen} task={query.data.task} items={buildTimeline(query.data.messages)} agentName="" />}
  </>;
}
function EvidenceLink({ evidence, revision }: { evidence: ProjectEvidenceView; revision: ProjectUpdateRevision }) {
  const paths = useWorkspacePaths();
  if (["deleted", "inaccessible"].includes(evidence.availability)) return <span>{evidence.input.id ?? evidence.input.url}</span>;
  if (evidence.input.kind === "issue" && evidence.input.id) return <AppLink className="underline underline-offset-2" href={paths.issueDetail(evidence.input.id)}>{evidence.label ?? evidence.input.id}</AppLink>;
  if (evidence.input.kind === "execution" && evidence.input.id) return <ExecutionEvidence evidence={evidence} revision={revision} />;
  return evidence.href ? <a className="underline underline-offset-2" href={evidence.href} target="_blank" rel="noopener noreferrer">{evidence.label ?? evidence.input.url}</a> : <span>{evidence.input.url}</span>;
}
function UpdateOutcome({ revision, descriptionRevision }: { revision: ProjectUpdateRevision; descriptionRevision: Project["description_revision"] }) {
  const { t } = useT("projects");
  const conclusions = { passed: t(($) => $.management.passed), partial: t(($) => $.management.partial), failed: t(($) => $.management.failed) };
  const judgments = { on_track: t(($) => $.management.on_track), attention: t(($) => $.management.attention), risk: t(($) => $.management.risk) };
  const outcomeClass = { passed: "text-success", partial: "text-warning-foreground", failed: "text-destructive", on_track: "text-success", attention: "text-warning-foreground", risk: "text-destructive" };
  return <>
    {revision.health_judgment && <span className={outcomeClass[revision.health_judgment]}><span className="text-muted-foreground">{t(($) => $.management.judgment)}: </span><span className="font-medium">{judgments[revision.health_judgment]}</span></span>}
    {revision.acceptance && <>
      <span className={`font-medium ${outcomeClass[revision.acceptance.conclusion]}`}>{conclusions[revision.acceptance.conclusion]}</span>
      <span className="text-muted-foreground">{t(($) => $.management.description_version, { revision: revision.acceptance.description_revision })}</span>
      {revision.acceptance.description_revision !== descriptionRevision && <span className="text-warning-foreground">{t(($) => $.management.stale_acceptance)}</span>}
    </>}
  </>;
}
function UpdateBody({ revision }: { revision: ProjectUpdateRevision }) {
  const { t } = useT("projects");
  const availability = { available: "", changed: t(($) => $.management.changed), deleted: t(($) => $.management.deleted), inaccessible: t(($) => $.management.inaccessible), unverified: t(($) => $.management.unverified) };
  const body = revision.body.trim();
  const bodySummary = body.split(/\r?\n/, 1)[0]?.slice(0, 240) ?? "";
  const hasBodyDetails = body !== bodySummary;
  const hasDetails = hasBodyDetails || Boolean(revision.acceptance || revision.evidence.length > 0 || revision.statistics_snapshot || revision.correction_reason);
  return <div className="min-w-0 space-y-2 text-caption [overflow-wrap:anywhere]">
    <p className="max-w-prose whitespace-pre-wrap leading-relaxed">{bodySummary || t(($) => $.management.none)}{hasBodyDetails && "…"}</p>
    {hasDetails && <details className="group/update">
      <summary className="flex min-h-8 w-fit cursor-pointer list-none items-center gap-1.5 rounded-md text-muted-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 [@media(pointer:coarse)]:min-h-11 [&::-webkit-details-marker]:hidden"><span>{t(($) => $.management.details)}</span><ChevronDown className="size-3.5 shrink-0 group-open/update:rotate-180" aria-hidden="true" /></summary>
      <div className="mt-3 space-y-4">
        {hasBodyDetails && <div className="max-w-prose whitespace-pre-wrap leading-relaxed">{revision.body}</div>}
        {revision.acceptance && <div className="space-y-3">
          <dl className="space-y-3">
            <div><dt className="text-muted-foreground">{t(($) => $.management.scope)}</dt><dd className="mt-1 max-w-prose whitespace-pre-wrap">{revision.acceptance.scope}</dd></div>
            {revision.acceptance.explanation && <div><dt className="text-muted-foreground">{t(($) => $.management.explanation)}</dt><dd className="mt-1 max-w-prose whitespace-pre-wrap">{revision.acceptance.explanation}</dd></div>}
          </dl>
          <details className="group/snapshot">
            <summary className="flex min-h-8 w-fit cursor-pointer list-none items-center gap-1.5 rounded-md text-muted-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 [@media(pointer:coarse)]:min-h-11 [&::-webkit-details-marker]:hidden"><span>{t(($) => $.management.server_version)}</span><ChevronDown className="size-3.5 shrink-0 group-open/snapshot:rotate-180" aria-hidden="true" /></summary>
            <pre className="mt-2 max-h-60 overflow-auto whitespace-pre-wrap font-sans">{revision.acceptance.description_snapshot}</pre>
          </details>
        </div>}
        {revision.evidence.length > 0 && <div className="space-y-1"><p className="text-muted-foreground">{t(($) => $.management.evidence)}</p><ul className="space-y-1">{revision.evidence.map((evidence, index) => <li key={index}>
          <EvidenceLink evidence={evidence} revision={revision} />
          <span className="ml-2 text-muted-foreground">{availability[evidence.availability]}</span>
        </li>)}</ul></div>}
        {revision.statistics_snapshot && <div className="space-y-1 tabular-nums"><p className="text-muted-foreground">{t(($) => $.management.snapshot)} · <time dateTime={revision.statistics_snapshot.calculated_at}>{new Date(revision.statistics_snapshot.calculated_at).toLocaleString()}</time></p>
          <p>{t(($) => $.management.total)} {revision.statistics_snapshot.counts.total ?? t(($) => $.management.na)} · {t(($) => $.management.completed)} {revision.statistics_snapshot.counts.completed ?? t(($) => $.management.na)} · {t(($) => $.management.cancelled)} {revision.statistics_snapshot.counts.cancelled ?? t(($) => $.management.na)}</p>
        </div>}
        {revision.correction_reason && <div className="space-y-1"><p className="text-muted-foreground">{t(($) => $.management.correction_reason)}</p><p className="max-w-prose whitespace-pre-wrap">{revision.correction_reason}</p></div>}
      </div>
    </details>}
  </div>;
}
function UpdateHistory({ project, updateId, onProtectedError }: { project: Project; updateId: string; onProtectedError: () => void }) {
  const { t } = useT("projects"); const [cursor, setCursor] = useState<string>();
  const query = useQuery(projectUpdateRevisionsOptions(project.workspace_id, project.id, updateId, cursor));
  useProjectAccessGuard(query.error, project.workspace_id, project.id, onProtectedError);
  return <div className="min-w-0 space-y-4 border-t pt-4">
    <h3 className="flex items-center gap-2 text-caption font-medium"><History className="size-3.5 text-muted-foreground" aria-hidden="true" />{t(($) => $.management.history)}</h3>
    {query.error && <p role="alert">{query.error.message}</p>}
    {query.data?.items.map((revision) => <article key={revision.revision} className="min-w-0 space-y-2"><p className="text-caption text-muted-foreground [overflow-wrap:anywhere]">{t(($) => $.management.revision, { revision: revision.revision })} · <HistoricalName member={revision.editor} /> · <time dateTime={revision.created_at}>{new Date(revision.created_at).toLocaleString()}</time></p><div className="flex flex-wrap items-baseline gap-x-2 gap-y-1 text-caption"><UpdateOutcome revision={revision} descriptionRevision={project.description_revision} /></div><UpdateBody revision={revision} /></article>)}
    <div className="flex gap-2">{cursor && <Button size="sm" variant="outline" onClick={() => setCursor(undefined)}>{t(($) => $.management.previous)}</Button>}{query.data?.next_cursor && <Button size="sm" variant="outline" onClick={() => setCursor(query.data!.next_cursor!)}>{t(($) => $.management.next)}</Button>}</div>
  </div>;
}
function correctionDraft(update: ProjectUpdate): ProjectUpdateDraft {
  const current = update.current;
  return { operation: "correct", update_id: update.id, expected_revision: update.current_revision,
    kind: current.kind, body: current.body, health_judgment: current.health_judgment,
    evidence: current.evidence.map((item) => item.input), acceptance: current.acceptance ? {
      conclusion: current.acceptance.conclusion, scope: current.acceptance.scope, explanation: current.acceptance.explanation,
    } : null, expected_description_revision: current.acceptance?.description_revision ?? null,
    include_statistics: false, correction_reason: "" };
}
function ProjectUpdateComposer({ project, initial, onClose, onProtectedError }: {
  project: Project; initial: ProjectUpdateDraft; onClose: () => void; onProtectedError: () => void;
}) {
  const generation = useRef(projectSessionGeneration()).current;
  const { t } = useT("projects"); const qc = useQueryClient(); const key = projectProgressDraftKey(api.getBaseUrl?.() ?? "", project.workspace_id, project.id, initial.update_id ?? undefined);
  const entry = useProjectProgressDraftStore((state) => state.draft.entries[key]); const draft = entry?.draft ?? initial;
  const preview = usePreviewProjectUpdate(project.workspace_id, project.id); const publish = usePublishProjectUpdate(project.workspace_id, project.id);
  const editor = useRef<ContentEditorRef>(null); const [evidenceKind, setEvidenceKind] = useState<ProjectEvidenceInput["kind"]>("url");
  const [evidenceValue, setEvidenceValue] = useState(""); const [conflict, setConflict] = useState<ProjectUpdateRevision | null>(null);
  const [descriptionConflict, setDescriptionConflict] = useState<Project | null>(null);
  const originalDescription = useRef(project.description ?? "");
  const errorFocus = useRef<HTMLDivElement>(null);
  const [reviewing, setReviewing] = useState(false);
  useProjectAccessGuard(publish.error ?? preview.error, project.workspace_id, project.id, onProtectedError);
  const patch = (change: Partial<ProjectUpdateDraft>) => {
    if (!canAccessProject(project.workspace_id, project.id, generation)) return;
    const current = useProjectProgressDraftStore.getState().draft.entries[key]?.draft ?? draft;
    const next = { ...current, ...change };
    // An unmount flush can acknowledge the exact body already sent to preview.
    // Only a genuine edit invalidates that preview and its explicit consent.
    if (JSON.stringify(next) === JSON.stringify(current)) return;
    writeProjectProgressDraft(key, next); preview.reset(); setReviewing(false);
  };
  useEffect(() => registerProjectLocalTextFlush(project.workspace_id, project.id, () => {
    if (editor.current && (entry || editor.current.getMarkdown().trim() !== initial.body) && canAccessProject(project.workspace_id, project.id, generation)) writeProjectProgressDraft(key, { ...draft, body: editor.current.getMarkdown().trim() });
  }), [project.workspace_id, project.id, key, draft, entry, initial.body, generation]);
  const busy = preview.isPending || publish.isPending;
  const error = publish.error ?? preview.error;
  useEffect(() => { if (error) errorFocus.current?.focus(); }, [error]);
  const handleConflict = async (failure: unknown) => {
    if (errorCode(failure) === "project_update_revision_conflict" && draft.update_id) {
      const result = await qc.fetchQuery({ ...projectUpdateRevisionsOptions(project.workspace_id, project.id, draft.update_id), staleTime: 0 }).catch(() => undefined);
      setConflict(result?.items[0] ?? null);
    }
    if (errorCode(failure) === "project_description_conflict" && draft.operation === "create" && draft.kind === "acceptance") {
      const current = await qc.fetchQuery({ ...projectDetailOptions(project.workspace_id, project.id), staleTime: 0 }).catch(() => undefined);
      if (current) setDescriptionConflict(current);
    }
  };
  const previewPublish = () => {
    // The imperative reader is raw; match ContentEditor autosave/unmount trim.
    const next = { ...draft, body: (editor.current?.getMarkdown() ?? draft.body).trim() };
    writeProjectProgressDraft(key, next);
    preview.mutate(next, { onSuccess: () => setReviewing(true), onError: handleConflict });
  };
  const confirm = () => {
    const input = preview.data ? prepareProjectUpdateIntent(key, preview.data) : entry?.intent;
    if (!input) return;
    publish.mutate(input, { onSuccess: () => {
      const unchanged = useProjectProgressDraftStore.getState().draft.entries[key]?.intent?.request_id === input.request_id;
      clearProjectProgressDraft(key, input.request_id); if (unchanged) onClose();
    }, onError: handleConflict });
  };
  const typeLabels = { progress: t(($) => $.management.progress), risk: t(($) => $.management.manual_risk), acceptance: t(($) => $.management.acceptance) };
  const stale = errorCode(error) === "project_update_preview_stale";
  return <div className="space-y-4 rounded-md border p-4" aria-label={t(($) => $.management.new_update)}>
    {entry?.intent && !reviewing && <div role="status" className="flex items-center justify-between gap-3 text-caption"><p>{t(($) => $.management.not_published)}</p><Button size="sm" disabled={busy} onClick={confirm}>{t(($) => $.management.retry)}</Button></div>}
    {!reviewing ? <>
      <label className="flex flex-col gap-1 text-caption">{t(($) => $.management.updates)}<select className="h-9 rounded-md border bg-background px-2" value={draft.kind} disabled={draft.operation === "correct" || busy} onChange={(event) => {
        const kind = event.target.value as ProjectUpdateDraft["kind"];
        patch({ kind, acceptance: kind === "acceptance" ? { conclusion: "passed", scope: "", explanation: "" } : null, expected_description_revision: kind === "acceptance" ? project.description_revision ?? null : null });
      }}>{Object.entries(typeLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
      <label className="block text-caption">{t(($) => $.management.body)}</label>
      <ContentEditor ref={editor} defaultValue={draft.body} flushPendingOnUnmount onUpdate={(body) => patch({ body })} placeholder={t(($) => $.management.body_hint)} debounceMs={300} />
      <label className="flex flex-col gap-1 text-caption">{t(($) => $.management.judgment)}<select className="h-9 rounded-md border bg-background px-2" value={draft.health_judgment ?? ""} onChange={(event) => patch({ health_judgment: event.target.value ? event.target.value as NonNullable<ProjectUpdateDraft["health_judgment"]> : null })}>
        <option value="">{t(($) => $.management.none)}</option><option value="on_track">{t(($) => $.management.on_track)}</option><option value="attention">{t(($) => $.management.attention)}</option><option value="risk">{t(($) => $.management.risk)}</option>
      </select></label>
      {draft.acceptance && <div className="space-y-3"><p className="text-caption">{t(($) => $.management.description_version, { revision: draft.expected_description_revision })}{draft.expected_description_revision !== project.description_revision && <span className="ml-2 text-warning">{t(($) => $.management.stale_acceptance)}</span>}</p>
        <label className="flex flex-col gap-1 text-caption">{t(($) => $.management.acceptance)}<select className="h-9 rounded-md border bg-background px-2" value={draft.acceptance.conclusion} onChange={(event) => patch({ acceptance: { ...draft.acceptance!, conclusion: event.target.value as "passed" | "partial" | "failed" } })}>
          <option value="passed">{t(($) => $.management.passed)}</option><option value="partial">{t(($) => $.management.partial)}</option><option value="failed">{t(($) => $.management.failed)}</option>
        </select></label>
        <label className="block space-y-1 text-caption">{t(($) => $.management.scope)}<Textarea value={draft.acceptance.scope} onChange={(event) => patch({ acceptance: { ...draft.acceptance!, scope: event.target.value } })} /></label>
        <label className="block space-y-1 text-caption">{t(($) => $.management.explanation)}<Textarea value={draft.acceptance.explanation ?? ""} onChange={(event) => patch({ acceptance: { ...draft.acceptance!, explanation: event.target.value } })} /></label>
      </div>}
      <div className="space-y-2"><p className="text-caption">{t(($) => $.management.evidence)}</p><div className="flex flex-wrap gap-2">
        <select aria-label={t(($) => $.management.evidence)} className="h-9 rounded-md border bg-background px-2 text-caption" value={evidenceKind} onChange={(event) => setEvidenceKind(event.target.value as ProjectEvidenceInput["kind"])}><option value="url">{t(($) => $.management.evidence_url)}</option><option value="issue">{t(($) => $.management.evidence_issue)}</option><option value="execution">{t(($) => $.management.evidence_execution)}</option></select>
        <Input className="min-w-40 flex-1" aria-label={t(($) => $.management.evidence_hint)} value={evidenceValue} onChange={(event) => setEvidenceValue(event.target.value)} />
        <Button variant="outline" size="sm" disabled={!evidenceValue.trim()} onClick={() => { patch({ evidence: [...draft.evidence, { kind: evidenceKind, id: evidenceKind === "url" ? null : evidenceValue.trim(), url: evidenceKind === "url" ? evidenceValue.trim() : null }] }); setEvidenceValue(""); }}>{t(($) => $.management.add_evidence)}</Button>
      </div>{draft.evidence.map((item, index) => <div key={index} className="flex items-center justify-between gap-2 text-caption"><span className="break-all">{item.url ?? item.id}</span><Button variant="ghost" size="sm" onClick={() => patch({ evidence: draft.evidence.filter((_, i) => i !== index) })}>{t(($) => $.management.remove)}</Button></div>)}</div>
      <label className="flex items-center gap-2 text-caption"><Checkbox checked={draft.include_statistics} onCheckedChange={(checked) => patch({ include_statistics: checked === true })} />{t(($) => $.management.include_statistics)}</label>
      {draft.operation === "correct" && <label className="block space-y-1 text-caption">{t(($) => $.management.correction_reason)}<Textarea value={draft.correction_reason ?? ""} onChange={(event) => patch({ correction_reason: event.target.value })} /></label>}
    </> : preview.data && <div className="space-y-3 text-caption"><h3 className="font-medium">{t(($) => $.management.preview)}</h3><div className="whitespace-pre-wrap break-words">{preview.data.draft.body}</div>
      {preview.data.draft.acceptance && <p>{preview.data.draft.acceptance.scope} · {t(($) => $.management.description_version, { revision: preview.data.description_revision })}</p>}
      <p className="font-medium">{t(($) => $.management.recipients)}</p>{preview.data.recipients.length ? <ul>{preview.data.recipients.map((member) => <li key={member.id}><HistoricalName member={member} /></li>)}</ul> : <p>{t(($) => $.management.no_recipients)}</p>}
      <p className="text-muted-foreground">{t(($) => $.management.agent_reference)}</p>
      {preview.data.statistics_snapshot && <p>{t(($) => $.management.snapshot)} · {new Date(preview.data.statistics_snapshot.calculated_at).toLocaleString()} · {t(($) => $.management.total)} {preview.data.statistics_snapshot.counts.total}</p>}
      <ul>{preview.data.evidence_versions.map((evidence, index) => <li key={index} className="break-all">{evidence.kind === "url" ? `${evidence.url} · ${t(($) => $.management.unverified)}` : evidence.id}</li>)}</ul>
    </div>}
    {error && <div ref={errorFocus} tabIndex={-1} role="alert" className="space-y-2 text-caption text-destructive"><p>{t(($) => $.management.not_published)}</p><p>{stale ? t(($) => $.management.preview_stale) : errorCode(error) === "project_evidence_forbidden" ? t(($) => $.management.source_forbidden) : error.message}</p>
      {error instanceof ApiError && error.body && typeof error.body === "object" && "field_errors" in error.body && Array.isArray(error.body.field_errors) ? <ul>{error.body.field_errors.map((item: { field?: string; message?: string }, i: number) => <li key={i}>{item.field}: {item.message}</li>)}</ul> : null}
    </div>}
    {conflict && <RevisionConflictCompare title={t(($) => $.management.conflict)} serverLabel={t(($) => $.management.server_version)} localLabel={t(($) => $.management.local_version)} serverValue={conflict.body} localValue={draft.body}
      localAction={<Button size="sm" onClick={() => { patch({ expected_revision: conflict.revision }); setConflict(null); publish.reset(); }}>{t(($) => $.management.save_merge)}</Button>} />}
    {descriptionConflict && <RevisionConflictCompare title={t(($) => $.management.conflict)} serverLabel={t(($) => $.management.server_version)} localLabel={t(($) => $.management.acceptance_description)}
      serverValue={descriptionConflict.description ?? ""} localValue={originalDescription.current}
      serverAction={<Button size="sm" onClick={() => {
        patch({ expected_description_revision: descriptionConflict.description_revision ?? null });
        originalDescription.current = descriptionConflict.description ?? ""; setDescriptionConflict(null); publish.reset();
      }}>{t(($) => $.management.adopt_description)}</Button>} />}
    <div className="flex flex-wrap gap-2">
      <Button variant="outline" disabled={busy} onClick={() => { if (canAccessProject(project.workspace_id, project.id, generation)) writeProjectProgressDraft(key, { ...draft, body: (editor.current?.getMarkdown() ?? draft.body).trim() }); onClose(); }}>{t(($) => $.management.cancel)}</Button>
      {reviewing ? <><Button variant="outline" disabled={busy} onClick={() => { setReviewing(false); preview.reset(); publish.reset(); }}>{t(($) => $.management.back_edit)}</Button>
        <Button disabled={busy || stale || !!conflict || !!descriptionConflict} onClick={confirm}>{t(($) => $.management.publish)}</Button></> : <Button disabled={busy} onClick={previewPublish}>{t(($) => $.management.preview)}</Button>}
    </div>
  </div>;
}
export function ProjectProgress({ project, onProtectedError, targetUpdateId }: { project: Project; onProtectedError: () => void; targetUpdateId?: string }) {
  const { t } = useT("projects"); const [cursor, setCursor] = useState<string>(); const [composer, setComposer] = useState<ProjectUpdateDraft | null>(null); const [history, setHistory] = useState<string>();
  const query = useQuery(projectUpdatesOptions(project.workspace_id, project.id, cursor));
  useProjectAccessGuard(query.error, project.workspace_id, project.id, onProtectedError);
  const typePresentations: Record<string, { label: string; Icon: LucideIcon }> = {
    progress: { label: t(($) => $.management.progress), Icon: TrendingUp },
    risk: { label: t(($) => $.management.manual_risk), Icon: ShieldAlert },
    acceptance: { label: t(($) => $.management.acceptance), Icon: ClipboardCheck },
  };
  return <section className="space-y-4"><div className="flex flex-wrap items-center justify-between gap-3"><h2 className="text-heading font-medium">{t(($) => $.management.updates)}</h2><div className="flex flex-wrap gap-2"><Button size="sm" variant="outline" onClick={() => setComposer(emptyProjectUpdateDraft())}>{t(($) => $.management.new_update)}</Button><Button size="sm" variant="outline" onClick={() => setComposer({ ...emptyProjectUpdateDraft(), kind: "acceptance", acceptance: { conclusion: "passed", scope: "", explanation: "" }, expected_description_revision: project.description_revision ?? null })}>{t(($) => $.management.new_acceptance)}</Button></div></div>
    {targetUpdateId && <UpdateHistory project={project} updateId={targetUpdateId} onProtectedError={onProtectedError} />}
    {composer && <ProjectUpdateComposer key={composer.update_id ?? "create"} project={project} initial={composer} onClose={() => setComposer(null)} onProtectedError={onProtectedError} />}
    {query.isPending && <p role="status">{t(($) => $.management.loading)}</p>}
    {query.error && <div role="alert"><p>{t(($) => $.management.load_error)}</p><Button onClick={() => void query.refetch()}>{t(($) => $.management.retry)}</Button></div>}
    {query.data?.items.length === 0 && <p className="text-caption text-muted-foreground">{t(($) => $.management.no_updates)}</p>}
    {query.data?.items.length ? <ol>
      {query.data.items.map((update) => {
        const presentation = typePresentations[String(update.current.kind)] ?? { label: t(($) => $.management.updates), Icon: History };
        const TypeIcon = presentation.Icon;
        return <li key={update.id} className="relative min-w-0 pb-6 pl-9 before:absolute before:bottom-0 before:left-3 before:top-6 before:w-px before:bg-border last:pb-0 last:before:hidden">
          <span className="absolute left-0 top-0 flex size-6 items-center justify-center rounded-full border border-border bg-background text-muted-foreground" aria-hidden="true"><TypeIcon className="size-3.5" /></span>
          <article className="min-w-0 space-y-3">
            <header className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
              <div className="min-w-0 space-y-1">
                <div className="flex min-h-6 flex-wrap items-baseline gap-x-2 gap-y-1 text-caption [overflow-wrap:anywhere]">
                  <span className="text-caption font-medium">{presentation.label}</span>
                  <UpdateOutcome revision={update.current} descriptionRevision={project.description_revision} />
                </div>
                <p className="text-caption text-muted-foreground [overflow-wrap:anywhere]"><HistoricalName member={update.author} /> · <time dateTime={update.published_at}>{new Date(update.published_at).toLocaleString()}</time> · {t(($) => $.management.revision, { revision: update.current_revision })}</p>
              </div>
              <div className="flex w-full shrink-0 flex-wrap gap-1 sm:w-auto sm:justify-end">
                <Button className="[@media(pointer:coarse)]:min-h-11" variant="ghost" size="sm" onClick={() => setComposer(correctionDraft(update))}>{t(($) => $.management.correct)}</Button>
                <Button className="[@media(pointer:coarse)]:min-h-11" variant="ghost" size="sm" aria-expanded={history === update.id} onClick={() => setHistory(history === update.id ? undefined : update.id)}><History className="size-3.5" aria-hidden="true" />{t(($) => $.management.history)}</Button>
              </div>
            </header>
            <UpdateBody revision={update.current} />
            {history === update.id && <UpdateHistory project={project} updateId={update.id} onProtectedError={onProtectedError} />}
          </article>
        </li>;
      })}
    </ol> : null}
    <div className="flex gap-2">{cursor && <Button size="sm" variant="outline" onClick={() => setCursor(undefined)}>{t(($) => $.management.previous)}</Button>}{query.data?.next_cursor && <Button size="sm" variant="outline" onClick={() => setCursor(query.data!.next_cursor!)}>{t(($) => $.management.next)}</Button>}</div>
  </section>;
}
