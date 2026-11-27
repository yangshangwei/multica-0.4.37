"use client";

import { useId, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { canPublishInitializedMaterial, materialInitializationOptions, materialSettingsOptions, newestMaterialRevision, materialDraftKey, useMaterialDraftStore, setMaterialEditorDraft, type MaterialEditorDraft, type MaterialCommand, type MaterialDetail, type MaterialKind, type MaterialSource } from "@multica/core/materials";
import { useAuthStore } from "@multica/core/auth";
import { projectResourcesOptions } from "@multica/core/projects";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useT } from "../../i18n";
import { MaterialSelect } from "./material-controls";

export function MaterialRevisionEditor({ detail, kind, locked, onSave, onCancel }: {
  detail: MaterialDetail; kind: MaterialKind; locked: boolean; onSave: (command: MaterialCommand) => void; onCancel: () => void;
}) {
  const { t } = useT("projects");
  const id = useId();
  const resources = useQuery(projectResourcesOptions(detail.material.workspace_id, detail.material.project_id));
  const initialization = useQuery(materialInitializationOptions(detail.material.workspace_id, detail.material.project_id));
  const settings = useQuery(materialSettingsOptions(detail.material.workspace_id, detail.material.project_id));
  const canPublish = detail.capabilities.can_publish === true && !initialization.isError && !settings.isError && canPublishInitializedMaterial(initialization.data, settings.data?.settings?.version);
  const userId = useAuthStore((state) => state.user?.id);
  const draftKey = materialDraftKey(userId, detail.material.workspace_id, detail.material.project_id, `${detail.material.id}:${kind}`);
  const [initial] = useState<MaterialEditorDraft>(() => {
    const sourceRevision = newestMaterialRevision(detail, kind);
    const requirement = detail.revisions.find((revision) => revision.id === detail.baseline?.revision_id);
    return { version: detail.material.version, baselineId: detail.baseline?.id, content: sourceRevision?.content ?? "",
      planContent: sourceRevision?.plan_content ?? "", acceptance: (sourceRevision?.acceptance_criteria ?? requirement?.acceptance_criteria ?? []).join("\n"),
      blockers: (sourceRevision?.blocking_questions ?? []).join("\n"), sources: sourceRevision?.sources ?? requirement?.sources ?? [], ready: false };
  });
  const saved = useMaterialDraftStore((state) => state.draft.editors[draftKey]);
  const draft = saved ?? initial;
  const { content, planContent, acceptance, blockers, sources, ready } = draft;
  const original = { version: draft.version, baselineId: draft.baselineId };
  const edit = (change: Partial<MaterialEditorDraft>) => setMaterialEditorDraft(draftKey, { ...draft, ...change });
  const setSources = (update: (sources: MaterialSource[]) => MaterialSource[]) => edit({ sources: update(sources) });
  const changed = original.version !== detail.material.version || kind === "plan" && original.baselineId !== detail.baseline?.id;
  const lines = (value: string) => value.split("\n").map((line) => line.trim()).filter(Boolean);
  const valid = content.trim().length > 0 && (!ready || canPublish && (kind !== "plan" || planContent.trim().length > 0) && sources.length > 0 && lines(acceptance).length > 0 && lines(blockers).length === 0 && sources.every((source) => /^[a-f\d]{40}(?:[a-f\d]{24})?$/i.test(source.commit) && source.resource_id));
  const resourceOptions = [{ value: "", label: t(($) => $.materials.choose) }, ...(resources.data ?? []).map((resource) => ({ value: resource.id, label: resource.label || resource.id }))];
  const editSource = (index: number, change: Partial<MaterialSource>) => setSources((current) => current.map((source, i) => i === index ? { ...source, ...change } : source));
  return <form className="space-y-5" onSubmit={(event) => {
    event.preventDefault();
    if (!valid || changed || locked || resources.isError || resources.isPending) return;
    onSave({ action: "revisions", data: {
      request_id: crypto.randomUUID(), expected_version: original.version, kind, content,
      ...(kind === "plan" ? { plan_content: planContent, source_baseline_id: original.baselineId } : {}),
      sources, acceptance_criteria: lines(acceptance), blocking_questions: lines(blockers), ready,
    } });
  }}>
    <h3 className="text-title font-semibold">{kind === "requirement" ? t(($) => $.materials.edit) : t(($) => $.materials.editPlan)}</h3>
    {changed && <div role="alert" className="space-y-2"><p className="text-body text-destructive">{t(($) => $.materials.versionChanged)}</p><Button type="button" variant="outline" disabled={locked} onClick={() => edit({ version: detail.material.version, baselineId: detail.baseline?.id })}>{t(($) => $.materials.rebaseDraft)}</Button></div>}
    <fieldset disabled={locked} className="min-w-0 space-y-5">
      <div className="space-y-2"><label htmlFor={`${id}-content`} className="text-body font-medium">{kind === "requirement" ? t(($) => $.materials.content) : t(($) => $.materials.designContent)}</label><Textarea id={`${id}-content`} className="min-h-80 resize-y text-body" value={content} onChange={(event) => edit({ content: event.target.value })} required /></div>
      {kind === "plan" && <div className="space-y-2"><label htmlFor={`${id}-plan`} className="text-body font-medium">{t(($) => $.materials.planContent)}</label><Textarea id={`${id}-plan`} className="min-h-48 resize-y text-body" value={planContent} onChange={(event) => edit({ planContent: event.target.value })} /></div>}
      <div className="grid gap-5 lg:grid-cols-2">
        <div className="space-y-2"><label htmlFor={`${id}-acceptance`} className="text-body font-medium">{t(($) => $.materials.acceptance)}</label><Textarea id={`${id}-acceptance`} className="min-h-32" value={acceptance} onChange={(event) => edit({ acceptance: event.target.value })} aria-describedby={`${id}-acceptance-hint`} /><p id={`${id}-acceptance-hint`} className="text-caption text-muted-foreground">{t(($) => $.materials.acceptanceHint)}</p></div>
        <div className="space-y-2"><label htmlFor={`${id}-blockers`} className="text-body font-medium">{t(($) => $.materials.blockers)}</label><Textarea id={`${id}-blockers`} className="min-h-32" value={blockers} onChange={(event) => edit({ blockers: event.target.value })} aria-describedby={`${id}-blockers-hint`} /><p id={`${id}-blockers-hint`} className="text-caption text-muted-foreground">{t(($) => $.materials.blockersHint)}</p></div>
      </div>
      <fieldset className="space-y-4"><legend className="text-body font-medium">{t(($) => $.materials.sources)}</legend>
        {resources.isError && <p role="alert" className="text-caption text-destructive">{t(($) => $.materials.inputsError)}</p>}
        {sources.map((source, index) => <div key={index} className="grid min-w-0 gap-3 sm:grid-cols-2">
          <MaterialSelect id={`${id}-resource-${index}`} label={t(($) => $.materials.sourceResource)} value={source.resource_id} disabled={locked || resources.isPending} options={resourceOptions} onChange={(resource_id) => editSource(index, { resource_id })} />
          <div className="space-y-2"><label htmlFor={`${id}-commit-${index}`} className="text-caption font-medium">{t(($) => $.materials.commit)}</label><Input id={`${id}-commit-${index}`} value={source.commit} onChange={(event) => editSource(index, { commit: event.target.value })} /></div>
          <div className="space-y-2"><label htmlFor={`${id}-path-${index}`} className="text-caption font-medium">{t(($) => $.materials.sourcePath)}</label><Input id={`${id}-path-${index}`} value={source.path} onChange={(event) => editSource(index, { path: event.target.value })} /></div>
          <div className="flex items-end"><Button type="button" variant="ghost" onClick={() => setSources((current) => current.filter((_, i) => i !== index))}>{t(($) => $.materials.removeSource)}</Button></div>
        </div>)}
        <Button type="button" variant="outline" size="sm" onClick={() => setSources((current) => [...current, { resource_id: "", commit: "", path: "" }])}>{t(($) => $.materials.addSource)}</Button>
      </fieldset>
      <div className="space-y-2"><label className="flex items-start gap-3 text-body"><input type="checkbox" className="mt-1 size-4 shrink-0 accent-primary" checked={ready} disabled={!canPublish && !ready} onChange={(event) => edit({ ready: event.target.checked })} aria-describedby={`${id}-ready-hint`} /><span>{t(($) => $.materials.ready)}</span></label><p id={`${id}-ready-hint`} className="text-caption text-muted-foreground">{t(($) => $.materials.readyHint)}</p></div>
      {!canPublish && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.materials.initialization.notReady)}</p>}
    </fieldset>
    <div className="flex flex-wrap gap-2"><Button type="submit" disabled={!valid || locked || changed || resources.isError || resources.isPending}>{locked ? t(($) => $.materials.saving) : t(($) => $.materials.save)}</Button><Button type="button" variant="ghost" onClick={onCancel} disabled={locked}>{t(($) => $.materials.cancel)}</Button></div>
  </form>;
}
