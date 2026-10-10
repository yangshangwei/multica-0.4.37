"use client";
import { useEffect, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { ProjectSchema } from "@multica/core/api/schemas";
import { api, ApiError } from "@multica/core/api";
import { ProjectDescriptionSave, projectGoalTemplateAppend, type ProjectGoalSection, projectProgressDraftKey, useProjectDescriptionDraftStore, writeProjectDescriptionDraft, acknowledgeProjectDescriptionDraft, clearProjectDescriptionDraft, canAccessProject, registerProjectLocalTextFlush, projectSessionGeneration, useProjectAccessStore } from "@multica/core/projects";
import { projectDetailOptions, projectKeys } from "@multica/core/projects/queries";
import { useUpdateProject } from "@multica/core/projects/mutations";
import type { Project } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { ContentEditor, type ContentEditorRef } from "../../editor";
import { RevisionConflictCompare } from "../../issues/components/revision-conflict-compare";
import { useT } from "../../i18n";
import "./project-description.css";

export function ProjectDescription({ project, supported }: { project: Project; supported: boolean }) {
  const generation = useRef(projectSessionGeneration()).current;
  const unavailable = useProjectAccessStore((state) => generation !== projectSessionGeneration() || !!state.denied[JSON.stringify([project.workspace_id, "*"])] || !!state.denied[JSON.stringify([project.workspace_id, project.id])] || !!state.deleted[JSON.stringify([project.workspace_id, project.id])]);
  const { t } = useT("projects"); const qc = useQueryClient(); const update = useUpdateProject();
  const draftKey = projectProgressDraftKey(api.getBaseUrl?.() ?? "", project.workspace_id, project.id, "description");
  const draft = useProjectDescriptionDraftStore((state) => state.draft.entries[draftKey]);
  const initialDraft = useRef(draft);
  const editor = useRef<ContentEditorRef>(null); const local = useRef(draft?.body ?? project.description ?? "");
  const [error, setError] = useState<unknown>(); const [conflict, setConflict] = useState<{ description: string; revision: number }>();
  const [adoptedServer, setAdoptedServer] = useState<{ description: string; revision: number } | null>(null);
  const controlledDescription = adoptedServer && (project.description_revision ?? 0) < adoptedServer.revision ? adoptedServer.description : project.description ?? "";
  const controlledRevision = adoptedServer && (project.description_revision ?? 0) < adoptedServer.revision ? adoptedServer.revision : project.description_revision ?? 0;
  const [template, setTemplate] = useState(false); const [selected, setSelected] = useState<string[]>([]);
  const [preview, setPreview] = useState<string | null>(null);
  const save = useRef<ProjectDescriptionSave | null>(null);
  useEffect(() => {
    const existing = initialDraft.current;
    const controller = new ProjectDescriptionSave(existing ? { description: existing.baseBody, description_revision: existing.revision } : project,
      async (description, revision) => {
        const result = await update.mutateAsync({ id: project.id, description, expected_description_revision: revision });
        acknowledgeProjectDescriptionDraft(draftKey, result.description ?? "", result.description_revision ?? revision);
        return result;
      }, (failure) => {
        setError(failure);
        if (failure instanceof ApiError && failure.status === 409 && failure.body && typeof failure.body === "object") {
          const parsed = ProjectSchema.safeParse("current" in failure.body ? failure.body.current : undefined);
          const current = parsed.success ? parsed.data : undefined;
          if (current?.id === project.id && current.workspace_id === project.workspace_id && current.description_revision) setConflict({ description: current.description ?? "", revision: current.description_revision });
        }
      });
    save.current = controller;
    return () => { controller.dispose(); };
    // One controller owns one mounted editor identity. Remote revisions are
    // adopted only through the editor baseline or explicit conflict resolution.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [draftKey]);
  const sections: ProjectGoalSection[] = [
    ["background", t(($) => $.management.background)], ["goal", t(($) => $.management.goal)],
    ["in_scope", t(($) => $.management.in_scope)], ["out_scope", t(($) => $.management.out_scope)],
    ["criteria", t(($) => $.management.criteria)], ["constraints", t(($) => $.management.constraints)],
    ["references", t(($) => $.management.references)],
  ].map(([id, title]) => ({ id: id!, title: title!, body: "" }));
  const pendingText = () => editor.current?.getMarkdown() ?? local.current;
  useEffect(() => registerProjectLocalTextFlush(project.workspace_id, project.id, () => {
    if (editor.current && editor.current.getMarkdown() !== save.current?.body && canAccessProject(project.workspace_id, project.id, generation)) writeProjectDescriptionDraft(draftKey, { body: editor.current.getMarkdown(), baseBody: save.current?.body ?? "", revision: save.current?.revision ?? 0 });
  }), [project.workspace_id, project.id, draftKey, generation]);
  const acceptServer = () => {
    if (!conflict || !canAccessProject(project.workspace_id, project.id, generation)) return;
    setAdoptedServer(conflict); save.current?.adopt(conflict.description, conflict.revision);
    void qc.cancelQueries({ queryKey: projectKeys.detail(project.workspace_id, project.id), exact: true });
    qc.setQueryData<Project>(projectKeys.detail(project.workspace_id, project.id), (current) => current ? { ...current, description: conflict.description, description_revision: conflict.revision } : current);
    local.current = conflict.description; editor.current?.adoptContent(conflict.description); clearProjectDescriptionDraft(draftKey); editor.current?.focus(); setConflict(undefined); setError(undefined);
  };
  if (unavailable) return null;
  return <div className="min-w-0 max-w-full space-y-3">
    {supported ? <ContentEditor ref={editor} className="project-description-editor" value={draft?.body ?? controlledDescription} debounceMs={0} flushPendingOnUnmount
      onUpdate={(markdown, baseline) => {
        // A clean editor may adopt a newer query version. A dirty editor's
        // baseline stays tied to the previous revision until acknowledgement.
        if (save.current && baseline === controlledDescription && baseline !== save.current.body && !error) save.current.adopt(baseline, controlledRevision);
        if (!canAccessProject(project.workspace_id, project.id, generation)) return;
        local.current = markdown;
        writeProjectDescriptionDraft(draftKey, { body: markdown, baseBody: save.current?.body ?? project.description ?? "", revision: save.current?.revision ?? project.description_revision ?? 0 });
        save.current?.enqueue(markdown, 1000);
      }} placeholder={t(($) => $.detail.description_placeholder)} />
      : <><div className="whitespace-pre-wrap break-words text-label leading-relaxed">{project.description}</div><p className="text-caption text-muted-foreground">{t(($) => $.management.description_unsupported)}</p></>}
    {conflict && <RevisionConflictCompare title={t(($) => $.management.conflict)} serverLabel={t(($) => $.management.server_version)} localLabel={t(($) => $.management.local_version)}
      serverValue={conflict.description} localValue={pendingText()} serverAction={<Button size="sm" variant="outline" className="h-auto min-h-7 w-full whitespace-normal break-words py-1.5 text-center leading-snug" onClick={acceptServer}>{t(($) => $.management.use_server)}</Button>}
      localAction={<Button size="sm" className="h-auto min-h-7 w-full whitespace-normal break-words py-1.5 text-center leading-snug" onClick={() => { save.current?.adopt(conflict.description, conflict.revision); setConflict(undefined); setError(undefined); save.current?.enqueue(pendingText()); editor.current?.focus(); }}>{t(($) => $.management.save_merge)}</Button>} />}
    {(error || (initialDraft.current && draft)) && !conflict ? <div role="alert" className="text-caption text-destructive"><p>{error instanceof Error ? error.message : t(($) => $.management.not_published)}</p>
      <Button size="sm" variant="outline" onClick={async () => { const current = await qc.fetchQuery({ ...projectDetailOptions(project.workspace_id, project.id), staleTime: 0 }); if ((current.description ?? "") !== save.current?.body) {
        setConflict({ description: current.description ?? "", revision: current.description_revision ?? 0 });
      } else { save.current?.adopt(current.description ?? "", current.description_revision ?? 0); setError(undefined); save.current?.enqueue(pendingText()); } }}>{t(($) => $.management.retry)}</Button></div> : null}
    {supported && <Button variant="outline" size="sm" onClick={() => { setTemplate(!template); setPreview(null); }}>{t(($) => $.management.template)}</Button>}
    {template && <div className="space-y-3 rounded-md border p-3 text-caption">
      <p>{t(($) => $.management.template_intro)}</p>
      <div className="grid grid-cols-2 gap-2">{sections.map((section) => <label key={section.id} className="flex items-center gap-2">
        <Checkbox checked={selected.includes(section.id)} onCheckedChange={(checked) => { setSelected((old) => checked ? [...old, section.id] : old.filter((id) => id !== section.id)); setPreview(null); }} />{section.title}
      </label>)}</div>
      {preview === null ? <Button size="sm" variant="outline" disabled={!selected.length} onClick={() => setPreview(projectGoalTemplateAppend(pendingText(), sections, selected))}>{t(($) => $.management.template_preview)}</Button> : <>
        <pre className="max-h-52 overflow-auto whitespace-pre-wrap text-caption">{preview || t(($) => $.management.template_empty)}</pre>
        <Button size="sm" disabled={!preview} onClick={() => {
          const append = projectGoalTemplateAppend(pendingText(), sections, selected);
          if (!append || !editor.current?.insertMarkdownAtEnd(append)) return;
          setTemplate(false); setPreview(null);
        }}>{t(($) => $.management.template_append)}</Button>
      </>}
    </div>}
  </div>;
}
