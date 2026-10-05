"use client";
import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { ApiError } from "@multica/core/api";
import { useProjectAccessStore } from "@multica/core/projects";
import { useUpdateProject } from "@multica/core/projects/mutations";
import { projectDetailOptions } from "@multica/core/projects/queries";
import type { Project, UpdateProjectRequest } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { AlertDialog, AlertDialogContent, AlertDialogHeader, AlertDialogTitle, AlertDialogDescription, AlertDialogFooter, AlertDialogCancel } from "@multica/ui/components/ui/alert-dialog";
import { useT } from "../../i18n";

/** Retains the user's attempted property patch separately from server cache. */
export function useProjectPropertyEditor(project: Project | undefined) {
  const { t } = useT("projects"); const qc = useQueryClient(); const mutation = useUpdateProject();
  const [attempt, setAttempt] = useState<UpdateProjectRequest | null>(null); const [error, setError] = useState<Error | null>(null);
  const send = (data: UpdateProjectRequest, revision = project?.revision) => {
    if (!project) return;
    setAttempt(data); setError(null);
    mutation.mutate({ id: project.id, ...data, ...(revision ? { expected_revision: revision } : {}) }, {
      onError: (failure) => setError(failure), onSuccess: () => { setAttempt(null); setError(null); },
    });
  };
  const unavailable = useProjectAccessStore((state) => !!state.denied[JSON.stringify([project?.workspace_id, "*"])] || !!state.deleted[JSON.stringify([project?.workspace_id, project?.id])]);
  useEffect(() => { if (unavailable) { setAttempt(null); setError(null); } }, [unavailable]);
  const retry = async () => {
    if (!project || !attempt) return;
    try { const latest = await qc.fetchQuery({ ...projectDetailOptions(project.workspace_id, project.id), staleTime: 0 }); send(attempt, latest.revision); }
    catch (failure) { setError(failure instanceof Error ? failure : new Error(String(failure))); }
  };
  const fieldLabels: Record<string, string> = { title: t(($) => $.table.name), status: t(($) => $.table.status), priority: t(($) => $.table.priority), start_date: t(($) => $.detail.prop_start_date), due_date: t(($) => $.detail.prop_due_date), lead_id: t(($) => $.table.lead), lead_type: t(($) => $.table.lead) };
  const recovery = <AlertDialog open={!unavailable && !!attempt && !!error} onOpenChange={(open) => { if (!open) { setAttempt(null); setError(null); } }}><AlertDialogContent><AlertDialogHeader>
    <AlertDialogTitle>{t(($) => $.management.property_failed)}</AlertDialogTitle><AlertDialogDescription>{t(($) => $.management.property_retry_hint)}</AlertDialogDescription>
  </AlertDialogHeader>
    <p role="alert" className="text-caption text-destructive">{error?.message}</p>
    {error instanceof ApiError && !!error.body && typeof error.body === "object" && "field_errors" in error.body && Array.isArray(error.body.field_errors) && <ul className="text-caption text-destructive">{error.body.field_errors.map((item: { message?: string }, index: number) => <li key={index}>{item.message}</li>)}</ul>}
    <dl className="space-y-1 text-caption">{attempt && Object.entries(attempt).filter(([key]) => !key.startsWith("expected_")).map(([key, value]) => <div key={key} className="flex gap-3"><dt className="text-muted-foreground">{fieldLabels[key] ?? key}</dt><dd className="min-w-0 break-words">{value === null ? t(($) => $.management.none) : String(value)}</dd></div>)}</dl>
    <AlertDialogFooter><AlertDialogCancel>{t(($) => $.management.discard_change)}</AlertDialogCancel><Button disabled={mutation.isPending} onClick={() => void retry()}>{t(($) => $.management.retry)}</Button></AlertDialogFooter>
  </AlertDialogContent></AlertDialog>;
  return { send, recovery };
}
