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
function UpdateBody({ revision }: { revision: ProjectUpdateRevision }) {
  const { t } = useT("projects");
  const availability = { available: "", changed: t(($) => $.management.changed), deleted: t(($) => $.management.deleted), inaccessible: t(($) => $.management.inaccessible), unverified: t(($) => $.management.unverified) };
  const conclusion = { passed: t(($) => $.management.passed), partial: t(($) => $.management.partial), failed: t(($) => $.management.failed) };
  const judgments = { on_track: t(($) => $.management.on_track), attention: t(($) => $.management.attention), risk: t(($) => $.management.risk) };
  return <div className="space-y-3 text-caption">
    <div className="whitespace-pre-wrap break-words">{revision.body}</div>
    {revision.health_judgment && <p>{t(($) => $.management.judgment)}: {judgments[revision.health_judgment]}</p>}
    {revision.acceptance && <div className="space-y-1"><p className="font-medium">{conclusion[revision.acceptance.conclusion]} · {t(($) => $.management.description_version, { revision: revision.acceptance.description_revision })}</p>
      <p className="whitespace-pre-wrap">{revision.acceptance.scope}</p><p className="whitespace-pre-wrap">{revision.acceptance.explanation}</p>
      <details><summary className="cursor-pointer text-muted-foreground">{t(($) => $.management.server_version)}</summary><pre className="max-h-60 overflow-auto whitespace-pre-wrap font-sans">{revision.acceptance.description_snapshot}</pre></details>
    </div>}
    {revision.evidence.length > 0 && <ul className="space-y-1">{revision.evidence.map((evidence, index) => <li key={index}>
      <EvidenceLink evidence={evidence} revision={revision} />
      <span className="ml-2 text-muted-foreground">{availability[evidence.availability]}</span>
    </li>)}</ul>}
    {revision.statistics_snapshot && <div className="text-muted-foreground">{t(($) => $.management.snapshot)} · {new Date(revision.statistics_snapshot.calculated_at).toLocaleString()}
      <p>{t(($) => $.management.total)} {revision.statistics_snapshot.counts.total ?? t(($) => $.management.na)} · {t(($) => $.management.completed)} {revision.statistics_snapshot.counts.completed ?? t(($) => $.management.na)} · {t(($) => $.management.cancelled)} {revision.statistics_snapshot.counts.cancelled ?? t(($) => $.management.na)}</p>
    </div>}
    {revision.correction_reason && <p className="text-muted-foreground">{t(($) => $.management.correction_reason)}: {revision.correction_reason}</p>}
  </div>;
}
function UpdateHistory({ project, updateId, onProtectedError }: { project: Project; updateId: string; onProtectedError: () => void }) {
  const { t } = useT("projects"); const [cursor, setCursor] = useState<string>();
  const query = useQuery(projectUpdateRevisionsOptions(project.workspace_id, project.id, updateId, cursor));
  useProjectAccessGuard(query.error, project.workspace_id, project.id, onProtectedError);
  return <div className="space-y-4 rounded-md border p-4">
    <h3 className="font-medium">{t(($) => $.management.history)}</h3>
    {query.error && <p role="alert">{query.error.message}</p>}
    {query.data?.items.map((revision) => <article key={revision.revision} className="space-y-2"><p className="text-caption text-muted-foreground">{t(($) => $.management.revision, { revision: revision.revision })} · <HistoricalName member={revision.editor} /> · {new Date(revision.created_at).toLocaleString()}</p><UpdateBody revision={revision} /></article>)}
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
  return <section className="space-y-4"><div className="flex flex-wrap items-center justify-between gap-3"><h2 className="text-heading font-medium">{t(($) => $.management.updates)}</h2><div className="flex gap-2"><Button size="sm" variant="outline" onClick={() => setComposer(emptyProjectUpdateDraft())}>{t(($) => $.management.new_update)}</Button><Button size="sm" variant="outline" onClick={() => setComposer({ ...emptyProjectUpdateDraft(), kind: "acceptance", acceptance: { conclusion: "passed", scope: "", explanation: "" }, expected_description_revision: project.description_revision ?? null })}>{t(($) => $.management.new_acceptance)}</Button></div></div>
    {targetUpdateId && <UpdateHistory project={project} updateId={targetUpdateId} onProtectedError={onProtectedError} />}
    {composer && <ProjectUpdateComposer key={composer.update_id ?? "create"} project={project} initial={composer} onClose={() => setComposer(null)} onProtectedError={onProtectedError} />}
    {query.isPending && <p role="status">{t(($) => $.management.loading)}</p>}
    {query.error && <div role="alert"><p>{t(($) => $.management.load_error)}</p><Button onClick={() => void query.refetch()}>{t(($) => $.management.retry)}</Button></div>}
    {query.data?.items.length === 0 && <p className="text-caption text-muted-foreground">{t(($) => $.management.no_updates)}</p>}
    {query.data?.items.map((update) => <article key={update.id} className="space-y-3 border-t pt-4"><div className="flex flex-wrap items-center justify-between gap-2"><p className="text-caption text-muted-foreground"><HistoricalName member={update.author} /> · {new Date(update.published_at).toLocaleString()} · {t(($) => $.management.revision, { revision: update.current_revision })}</p><div className="flex gap-1"><Button variant="ghost" size="sm" onClick={() => setComposer(correctionDraft(update))}>{t(($) => $.management.correct)}</Button><Button variant="ghost" size="sm" onClick={() => setHistory(history === update.id ? undefined : update.id)}>{t(($) => $.management.history)}</Button></div></div>
      <UpdateBody revision={update.current} />{update.current.acceptance && update.current.acceptance.description_revision !== project.description_revision && <p className="text-caption text-warning">{t(($) => $.management.stale_acceptance)}</p>}
      {history === update.id && <UpdateHistory project={project} updateId={update.id} onProtectedError={onProtectedError} />}
    </article>)}
    <div className="flex gap-2">{cursor && <Button size="sm" variant="outline" onClick={() => setCursor(undefined)}>{t(($) => $.management.previous)}</Button>}{query.data?.next_cursor && <Button size="sm" variant="outline" onClick={() => setCursor(query.data!.next_cursor!)}>{t(($) => $.management.next)}</Button>}</div>
  </section>;
}
