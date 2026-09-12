"use client";

import { useId, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ChevronRight, Plus, Zap } from "lucide-react";
import { autopilotListOptions } from "@multica/core/autopilots/queries";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { autopilotTemplateHref } from "../../autopilots/template-create-defaults";
import { formatInTimeZone } from "../../common/format-in-time-zone";
import { useT } from "../../i18n";
import { AppLink } from "../../navigation";

export function ProjectAutomationsSection({
  projectId,
  defaultSquadId,
}: {
  projectId: string;
  defaultSquadId?: string | null;
}) {
  const { t, i18n } = useT("autopilots");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const contentId = useId();
  const [open, setOpen] = useState(true);
  const { data = [], isLoading, isError, refetch } = useQuery(autopilotListOptions(wsId));
  const automations = data.filter((automation) =>
    automation.workspace_id === wsId &&
    automation.project_id === projectId &&
    automation.status !== "archived",
  );
  const templateHref = autopilotTemplateHref(paths.newAutopilotTemplate(), {
    initialProjectId: projectId,
    initialAssigneeType: defaultSquadId ? "squad" : null,
    initialAssigneeId: defaultSquadId,
  });

  return (
    <section>
      <button
        type="button"
        aria-expanded={open}
        aria-controls={contentId}
        className="mb-2 flex w-full items-center gap-1 rounded-md px-2 py-1 text-caption font-medium transition-colors hover:bg-accent/70 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        onClick={() => setOpen((value) => !value)}
      >
        {t(($) => $.project_section.title)}
        <ChevronRight aria-hidden="true" className={`size-3 shrink-0 text-muted-foreground transition-transform ${open ? "rotate-90" : ""}`} />
      </button>
      {open && (
        <div id={contentId} className="space-y-3 pl-2">
          {isLoading ? (
            <Skeleton className="h-10 w-full" />
          ) : isError ? (
            <div className="space-y-1">
              <p role="alert" className="text-caption text-destructive">{t(($) => $.project_section.load_failed)}</p>
              <Button variant="ghost" size="sm" onClick={() => void refetch()}>{t(($) => $.page.retry)}</Button>
            </div>
          ) : automations.length === 0 ? (
            <p className="text-caption text-muted-foreground">{t(($) => $.project_section.empty)}</p>
          ) : (
            <ul className="max-h-64 space-y-1 overflow-y-auto">
              {automations.map((automation) => (
                <li key={automation.id}>
                  <AppLink href={paths.autopilotDetail(automation.id)} className="-mx-1 flex min-w-0 items-start gap-2 rounded-md px-1 py-1.5 hover:bg-accent/70 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
                    <Zap aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-muted-foreground" />
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-caption font-medium">{automation.title}</p>
                      <p className="mt-0.5 text-micro text-muted-foreground">
                        {automation.status === "active"
                          ? t(($) => $.status.active)
                          : automation.status === "paused"
                            ? automation.pause_reason === "agent_runtime_required"
                              ? t(($) => $.status.paused_runtime_required)
                              : t(($) => $.status.paused)
                            : t(($) => $.project_section.status_unknown)}
                      </p>
                      {automation.status === "active" && automation.next_run_at && (
                        <p className="mt-0.5 text-micro tabular-nums text-muted-foreground">
                          {t(($) => $.project_section.next_run, { time: formatInTimeZone(automation.next_run_at, undefined, i18n.language) })}
                        </p>
                      )}
                    </div>
                  </AppLink>
                </li>
              ))}
            </ul>
          )}
          <AppLink href={templateHref} className="inline-flex items-center gap-1.5 rounded-md text-caption text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
            <Plus className="size-3.5" aria-hidden="true" />
            {t(($) => $.project_section.add)}
          </AppLink>
        </div>
      )}
    </section>
  );
}
