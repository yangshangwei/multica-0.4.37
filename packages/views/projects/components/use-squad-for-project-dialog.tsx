"use client";

import { useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Loader2, Plus } from "lucide-react";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceId } from "@multica/core/hooks";
import { useModalStore } from "@multica/core/modals";
import { useWorkspacePaths } from "@multica/core/paths";
import {
  eligibleProjectRuntimes,
  projectLocalDaemonIds,
  projectResourcesOptions,
  selectProjectRuntime,
  useConfigureProjectSquad,
} from "@multica/core/projects";
import { projectListOptions } from "@multica/core/projects/queries";
import { runtimeDisplayLabel, runtimeListOptions } from "@multica/core/runtimes";
import type { SquadTemplate } from "@multica/core/types";
import { agentListOptions } from "@multica/core/workspace/queries";
import { Button } from "@multica/ui/components/ui/button";
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { Label } from "@multica/ui/components/ui/label";
import {
  Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue,
} from "@multica/ui/components/ui/select";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { templateLanguageFor } from "../../agents/create/use-role-templates";
import { useLocale, useT } from "../../i18n";
import { useNavigation } from "../../navigation";
import { buildRuntimeMachines, runtimeRowLabel } from "../../runtimes/components/runtime-machines";
import { ProjectPicker } from "./project-picker";

export function UseSquadForProjectDialog({ template, onClose }: {
  template: SquadTemplate;
  onClose: () => void;
}) {
  const { t } = useT("squads");
  const locale = useLocale();
  const wsId = useWorkspaceId();
  const userId = useAuthStore((state) => state.user?.id ?? null);
  const navigation = useNavigation();
  const paths = useWorkspacePaths();
  const projects = useQuery(projectListOptions(wsId));
  const runtimes = useQuery(runtimeListOptions(wsId));
  const agents = useQuery(agentListOptions(wsId));
  const configure = useConfigureProjectSquad(wsId);
  const [projectId, setProjectId] = useState<string | null>(null);
  const [runtimeId, setRuntimeId] = useState<string | null>(null);
  const [error, setError] = useState(false);
  const pending = useRef(false);
  const mounted = useRef(true);
  const currentWorkspace = useRef(wsId);
  currentWorkspace.current = wsId;

  useEffect(() => {
    mounted.current = true;
    return () => { mounted.current = false; };
  }, []);

  const project = (projects.data ?? []).find((entry) => entry.id === projectId && entry.workspace_id === wsId);
  const resources = useQuery({
    ...projectResourcesOptions(wsId, project?.id ?? ""),
    enabled: !!project,
  });
  const runtimeOptions = {
    workspaceId: wsId,
    userId,
    daemonIds: projectLocalDaemonIds(resources.data ?? []),
  };
  const eligibleRuntimes = eligibleProjectRuntimes(runtimes.isError ? [] : runtimes.data ?? [], runtimeOptions);
  const mika = agents.isError ? undefined : agents.data?.find((agent) =>
    agent.system_key === "mika" && !agent.archived_at && agent.workspace_id === wsId,
  );
  const suggestedRuntime = selectProjectRuntime(eligibleRuntimes, { ...runtimeOptions, preferredRuntimeId: mika?.runtime_id });
  const selectedRuntimeId = runtimeId ?? suggestedRuntime?.id ?? "";
  const runtimeUnavailable = !!selectedRuntimeId && !eligibleRuntimes.some((runtime) => runtime.id === selectedRuntimeId);
  const machines = buildRuntimeMachines(eligibleRuntimes, { now: Date.now(), currentUserId: userId });
  const runtimeItems = [
    { value: "", label: t(($) => $.use_for_project.connect_later) },
    ...eligibleRuntimes.map((runtime) => ({ value: runtime.id, label: runtimeDisplayLabel(runtime) })),
  ];
  const canApply = !!project && !!userId && !projects.isError && !resources.isPending && !resources.isError
    && !runtimeUnavailable && !configure.isPending && (!runtimes.isPending || runtimeId === "");

  const apply = async () => {
    if (!canApply || !project || pending.current) return;
    const originWorkspace = wsId;
    const targetProjectId = project.id;
    pending.current = true;
    setError(false);
    try {
      const updated = await configure.mutateAsync({
        id: targetProjectId,
        template_key: template.key,
        ...(selectedRuntimeId ? { runtime_id: selectedRuntimeId } : {}),
        language: templateLanguageFor(locale),
      });
      if (!mounted.current || currentWorkspace.current !== originWorkspace) return;
      if (updated.id !== targetProjectId || updated.workspace_id !== originWorkspace) {
        setError(true);
        return;
      }
      // Both needs_runtime and failed have recovery on the project. Neither
      // response means the machine is ready, and neither starts an agent run.
      onClose();
      navigation.push(paths.projectDetail(updated.id));
    } catch {
      if (mounted.current && currentWorkspace.current === originWorkspace) setError(true);
    } finally {
      pending.current = false;
    }
  };

  return (
    <Dialog open onOpenChange={(open) => { if (!open && !pending.current) onClose(); }}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t(($) => $.use_for_project.title, { name: template.title || template.name })}</DialogTitle>
          <DialogDescription>{template.description}</DialogDescription>
        </DialogHeader>
        <div className="max-h-[60vh] space-y-4 overflow-y-auto">
          <p className="text-caption text-muted-foreground">{t(($) => $.use_for_project.description)}</p>
          {projects.isError ? (
            <QueryRecovery message={t(($) => $.use_for_project.projects_error)} onRetry={() => void projects.refetch()} />
          ) : projects.isPending ? (
            <div role="status">
              <span className="sr-only">{t(($) => $.use_for_project.projects_loading)}</span>
              <Skeleton className="h-9 w-full" />
            </div>
          ) : (projects.data ?? []).length === 0 ? (
            <p className="text-body text-muted-foreground">{t(($) => $.use_for_project.no_projects)}</p>
          ) : (
            <div className="space-y-2">
              <Label htmlFor="use-squad-project">{t(($) => $.use_for_project.project_label)}</Label>
              <ProjectPicker
                projectId={project?.id ?? null}
                disabled={configure.isPending}
                onUpdate={({ project_id }) => { setProjectId(project_id ?? null); setRuntimeId(null); setError(false); }}
                triggerRender={<Button id="use-squad-project" type="button" variant="outline" className="max-w-full justify-start" aria-label={t(($) => $.use_for_project.project_label)} />}
              />
            </div>
          )}

          {project && !projects.isError && (
            resources.isError ? (
              <QueryRecovery message={t(($) => $.use_for_project.resources_error)} onRetry={() => void resources.refetch()} />
            ) : resources.isPending ? (
              <div role="status">
                <span className="sr-only">{t(($) => $.use_for_project.resources_loading)}</span>
                <Skeleton className="h-9 w-full" />
              </div>
            ) : (
              <div className="space-y-2">
                <Label htmlFor="use-squad-runtime">{t(($) => $.use_for_project.runtime_label)}</Label>
                {runtimes.isPending ? (
                  <div className="flex flex-wrap items-center gap-2">
                    <Skeleton className="h-9 w-40" />
                    <Button type="button" size="sm" variant="ghost" onClick={() => setRuntimeId("")}>{t(($) => $.use_for_project.connect_later)}</Button>
                  </div>
                ) : (
                  <Select items={runtimeItems} value={selectedRuntimeId} disabled={configure.isPending} onValueChange={(value) => { setRuntimeId(value ?? ""); setError(false); }}>
                    <SelectTrigger id="use-squad-runtime" className="w-full" aria-label={t(($) => $.use_for_project.runtime_label)} aria-invalid={runtimeUnavailable}>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectItem value="">{t(($) => $.use_for_project.connect_later)}</SelectItem>
                      {machines.map((machine) => (
                        <SelectGroup key={machine.id}>
                          <SelectLabel>{machine.title}</SelectLabel>
                          {machine.runtimes.map((runtime) => (
                            <SelectItem key={runtime.id} value={runtime.id}>
                              <span className="truncate">{runtimeRowLabel(runtime, machine.title)}</span>
                              {runtime.status !== "online" && <span className="text-caption text-muted-foreground">{t(($) => $.use_for_project.offline)}</span>}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      ))}
                    </SelectContent>
                  </Select>
                )}
                <p className="text-caption text-muted-foreground">{t(($) => $.use_for_project.runtime_hint)}</p>
                {runtimeUnavailable && <p role="alert" className="text-caption text-destructive">{t(($) => $.use_for_project.runtime_unavailable)}</p>}
                {runtimes.isError && <QueryRecovery message={t(($) => $.use_for_project.runtimes_error)} onRetry={() => void runtimes.refetch()} />}
                {agents.isError && <QueryRecovery message={t(($) => $.use_for_project.agents_error)} onRetry={() => void agents.refetch()} />}
              </div>
            )
          )}

          {error && <p role="alert" className="text-caption text-destructive">{t(($) => $.use_for_project.failed)}</p>}
          <Button type="button" variant="outline" size="sm" disabled={configure.isPending} onClick={() => {
            if (pending.current) return;
            onClose();
            useModalStore.getState().open("create-project", { squad_template_key: template.key });
          }}>
            <Plus className="size-3.5" aria-hidden="true" />{t(($) => $.use_for_project.new_project)}
          </Button>
        </div>
        <DialogFooter>
          <Button type="button" variant="ghost" disabled={configure.isPending} onClick={() => { if (!pending.current) onClose(); }}>{t(($) => $.use_for_project.cancel)}</Button>
          <Button type="button" disabled={!canApply} onClick={() => void apply()}>
            {configure.isPending && <Loader2 className="size-3.5 animate-spin" aria-hidden="true" />}
            {configure.isPending ? t(($) => $.use_for_project.applying) : t(($) => $.use_for_project.apply)}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function QueryRecovery({ message, onRetry }: { message: string; onRetry: () => void }) {
  const { t } = useT("squads");
  return (
    <div role="alert" className="flex flex-wrap items-center gap-3 text-caption text-muted-foreground">
      <p>{message}</p>
      <Button type="button" size="sm" variant="outline" onClick={onRetry}>{t(($) => $.catalog.retry)}</Button>
    </div>
  );
}
