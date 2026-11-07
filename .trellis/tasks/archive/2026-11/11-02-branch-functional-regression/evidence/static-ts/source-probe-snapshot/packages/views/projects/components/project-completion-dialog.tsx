"use client";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { projectOverviewOptions } from "@multica/core/projects";
import { useUpdateProject } from "@multica/core/projects/mutations";
import { useWorkspacePaths } from "@multica/core/paths";
import type { Project } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { AlertDialog, AlertDialogContent, AlertDialogHeader, AlertDialogTitle, AlertDialogDescription, AlertDialogFooter, AlertDialogCancel } from "@multica/ui/components/ui/alert-dialog";
import { AppLink } from "../../navigation";
import { useT } from "../../i18n";
import { ProjectAcceptance } from "./project-overview";

export function ProjectCompletionDialog({ project, onClose }: { project: Project; onClose: () => void }) {
  const { t } = useT("projects"); const paths = useWorkspacePaths(); const [reason, setReason] = useState("");
  const overview = useQuery(projectOverviewOptions(project.workspace_id, project.id)); const update = useUpdateProject();
  return <AlertDialog open onOpenChange={(open) => { if (!open) onClose(); }}><AlertDialogContent><AlertDialogHeader>
    <AlertDialogTitle>{t(($) => $.management.complete_title)}</AlertDialogTitle>
    <AlertDialogDescription>{t(($) => $.management.complete_hint, { open: overview.data?.statistics.counts.open ?? project.open_issue_count ?? t(($) => $.management.na), cancelled: overview.data?.statistics.counts.cancelled ?? project.cancelled_issue_count ?? t(($) => $.management.na) })}</AlertDialogDescription>
  </AlertDialogHeader>
    {overview.data && <ProjectAcceptance overview={overview.data} />}
    {overview.error && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.management.unavailable)}</p>}
    <label className="space-y-1 text-caption">{t(($) => $.management.complete_reason)}<Textarea value={reason} onChange={(event) => setReason(event.target.value)} /></label>
    {update.error && <p role="alert" className="text-caption text-destructive">{update.error.message}</p>}
    <AlertDialogFooter><AlertDialogCancel>{t(($) => $.management.cancel)}</AlertDialogCancel><Button variant="outline" render={<AppLink href={paths.projectDetail(project.id, "issues")} />}>{t(($) => $.management.view_issues)}</Button>
      <Button disabled={update.isPending} onClick={() => update.mutate({ id: project.id, status: "completed", expected_revision: project.revision, status_reason: reason || null }, { onSuccess: onClose })}>{t(($) => $.management.continue_complete)}</Button>
    </AlertDialogFooter>
  </AlertDialogContent></AlertDialog>;
}
