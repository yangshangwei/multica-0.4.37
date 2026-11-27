"use client";

import { useId, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { FileText, Plus, Settings2 } from "lucide-react";
import { ApiError } from "@multica/core/api";
import { useAuthStore } from "@multica/core/auth";
import { useProjectAccessStore } from "@multica/core/projects";
import { materialListOptions, materialSettingsOptions, useCreateMaterial, materialDraftKey, decodeMaterialCreate } from "@multica/core/materials";
import { useWorkspacePaths } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { useNavigation } from "../../navigation";
import { useT } from "../../i18n";
import { getSettingsQueryState } from "../../settings/components/settings-query-feedback";
import { MaterialActionNotice, MaterialLoadError, useMaterialAction, useMaterialLabels } from "./material-controls";
import { MaterialDetailView } from "./material-detail";
import { ProjectMaterialInitialization } from "./project-material-initialization";
import { ProjectMaterialSettings } from "./project-material-settings";

export function ProjectMaterials({ workspaceId, projectId }: { workspaceId: string; projectId: string }) {
  const userId = useAuthStore((state) => state.user?.id);
  const denied = useProjectAccessStore((state) => !!(state.denied[JSON.stringify([workspaceId, "*"])] || state.denied[JSON.stringify([workspaceId, projectId])] || state.deleted[JSON.stringify([workspaceId, projectId])]));
  if (denied) return null;
  // Local text and in-flight callbacks belong to one viewer and project.
  return <MaterialsContent key={`${userId}:${workspaceId}:${projectId}`} workspaceId={workspaceId} projectId={projectId} />;
}

function MaterialsContent({ workspaceId, projectId }: { workspaceId: string; projectId: string }) {
  const { t } = useT("projects");
  const id = useId();
  const labels = useMaterialLabels();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const settings = useQuery(materialSettingsOptions(workspaceId, projectId));
  const list = useQuery({ ...materialListOptions(workspaceId, projectId), enabled: !!settings.data });
  const create = useCreateMaterial(workspaceId, projectId);
  const userId = useAuthStore((state) => state.user?.id);
  const action = useMaterialAction(create.mutateAsync, materialDraftKey(userId, workspaceId, projectId, "create"), decodeMaterialCreate, (result) => open(result.material.id));
  const [configuring, setConfiguring] = useState(false);
  const [creating, setCreating] = useState(false);
  const [title, setTitle] = useState("");
  const [search, setSearch] = useState("");
  const selected = navigation.searchParams?.get("material");
  const open = (materialId?: string) => navigation.push(materialId ? paths.projectMaterial(projectId, materialId) : paths.projectDetail(projectId, "materials"));
  const error = settings.error;
  const unsupported = error instanceof ApiError && [404, 405, 501].includes(error.status);
  const { hasData, failed } = getSettingsQueryState(settings);
  const refresh = () => { if (!settings.isFetching) void settings.refetch({ cancelRefetch: false }); if (!list.isFetching) void list.refetch({ cancelRefetch: false }); };
  if (!hasData || !settings.data) return failed
    ? <div className="p-6"><MaterialLoadError unsupported={unsupported} onRetry={refresh} busy={settings.isFetching} /></div>
    : <p role="status" className="p-6 text-body text-muted-foreground">{t(($) => $.materials.loading)}</p>;
  const envelope = settings.data;
  const materials = (list.data?.materials ?? []).filter((material) => `${material.title} ${material.path}`.toLocaleLowerCase().includes(search.toLocaleLowerCase()));
  return <div className="min-h-0 min-w-0 flex-1 overflow-y-auto p-4 sm:p-6">
    <div className="mx-auto w-full max-w-6xl space-y-6">
      {failed && <MaterialLoadError unsupported={unsupported} onRetry={refresh} busy={settings.isFetching} />}
      <header className="flex flex-wrap items-start justify-between gap-4"><div className="min-w-0 max-w-prose space-y-2"><h2 className="text-title font-semibold">{t(($) => $.materials.title)}</h2><p className="text-body text-muted-foreground">{t(($) => $.materials.intro)}</p></div>
        {envelope.can_configure && <Button type="button" size="sm" variant="outline" disabled={action.locked} onClick={() => setConfiguring(!configuring)}><Settings2 />{t(($) => $.materials.settings)}</Button>}
      </header>
      {!configuring && <ProjectMaterialInitialization workspaceId={workspaceId} projectId={projectId} settingsVersion={envelope.settings?.version} />}
      {configuring ? <ProjectMaterialSettings workspaceId={workspaceId} projectId={projectId} envelope={envelope} onClose={() => setConfiguring(false)} />
        : selected ? <MaterialDetailView key={selected} workspaceId={workspaceId} projectId={projectId} materialId={selected} onBack={() => open()} />
        : <>
          {!envelope.settings && <div className="space-y-3 py-5"><p className="text-body text-muted-foreground">{t(($) => $.materials.notConfigured)}</p>{envelope.can_configure && <Button type="button" onClick={() => setConfiguring(true)}>{t(($) => $.materials.configure)}</Button>}</div>}
          <MaterialActionNotice error={action.error} uncertain={action.pending !== null} busy={action.busy} onRetry={action.retry} />
          <div className="flex flex-wrap items-center gap-3"><div className="min-w-0 flex-1"><label className="sr-only" htmlFor={`${id}-search`}>{t(($) => $.materials.search)}</label><Input id={`${id}-search`} type="search" value={search} onChange={(event) => setSearch(event.target.value)} placeholder={t(($) => $.materials.search)} /></div>
            <Button type="button" disabled={!envelope.settings?.enabled || action.locked} onClick={() => setCreating(true)}><Plus />{t(($) => $.materials.new)}</Button>
          </div>
          {creating && <form className="max-w-2xl space-y-3" onSubmit={(event) => { event.preventDefault(); if (!title.trim() || action.locked) return; action.submit({ request_id: crypto.randomUUID(), title: title.trim() }, (result) => { setCreating(false); setTitle(""); open(result.material.id); }); }}>
            <label htmlFor={`${id}-title`} className="text-body font-medium">{t(($) => $.materials.titleLabel)}</label><Input id={`${id}-title`} value={title} disabled={action.locked} onChange={(event) => setTitle(event.target.value)} required />
            <div className="flex gap-2"><Button type="submit" size="sm" disabled={!title.trim() || action.locked}>{action.busy ? t(($) => $.materials.saving) : t(($) => $.materials.create)}</Button><Button type="button" size="sm" variant="ghost" disabled={action.locked} onClick={() => setCreating(false)}>{t(($) => $.materials.cancel)}</Button></div>
          </form>}
          {list.isError ? <MaterialLoadError onRetry={() => { void list.refetch(); }} /> : list.isPending ? <p role="status" className="text-body text-muted-foreground">{t(($) => $.materials.loading)}</p>
            : materials.length === 0 ? <div className="space-y-2 py-8"><h3 className="text-body font-medium">{search ? t(($) => $.materials.noMatches) : t(($) => $.materials.empty)}</h3><p className="text-body text-muted-foreground">{t(($) => $.materials.emptyHint)}</p></div>
            : <ul className="divide-y">{materials.map((material) => <li key={material.id}><button type="button" className="flex w-full min-w-0 items-start gap-3 rounded-md py-4 text-left hover:bg-accent/50 focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring" onClick={() => open(material.id)}>
              <FileText className="mt-1 size-4 shrink-0 text-muted-foreground" /><span className="min-w-0 flex-1 space-y-1"><span className="block break-words text-body font-medium">{material.title}</span><span className="block break-all text-caption text-muted-foreground">{material.path}</span></span><span className="max-w-40 shrink-0 text-right text-caption text-muted-foreground">{labels.state(material.status)}</span>
            </button></li>)}</ul>}
        </>}
    </div>
  </div>;
}
