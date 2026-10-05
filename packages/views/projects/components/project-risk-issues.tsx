"use client";
import { useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { projectRiskOptions } from "@multica/core/projects";
import { useWorkspacePaths } from "@multica/core/paths";
import type { Project, ProjectRiskSignal } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { AppLink } from "../../navigation";
import { useT } from "../../i18n";
import { useProjectAccessGuard } from "./use-project-access-guard";

/** Exact, temporary health scope. It deliberately never mounts the saved-view
 * controller: actor filters, hidden categories, sub-issue and date preferences
 * cannot alter membership in the server-issued risk set. */
export function ProjectRiskIssues({ project, signal, version, onBack, onProtectedError }: {
  project: Project; signal: ProjectRiskSignal; version?: string; onBack: () => void; onProtectedError: () => void;
}) {
  const { t } = useT("projects"); const paths = useWorkspacePaths(); const [page, setPage] = useState<{ cursor?: string; version?: string }>({ version }); const cursor = page.cursor; const [mode, setMode] = useState<"list" | "table">("list");
  const qc = useQueryClient();
  const [continuationChanged, setContinuationChanged] = useState(false);
  const [restarting, setRestarting] = useState(false);
  const [restartError, setRestartError] = useState<Error | null>(null);
  const query = useQuery(projectRiskOptions(project.workspace_id, project.id, signal, cursor, page.version));
  useProjectAccessGuard(query.error ?? restartError, project.workspace_id, project.id, onProtectedError);
  const changedContinuation = !!cursor && query.data?.refreshed === true;
  useEffect(() => { if (changedContinuation) setContinuationChanged(true); }, [changedContinuation]);
  const showContinuationWarning = continuationChanged || changedContinuation;
  const restart = async () => {
    setRestarting(true); setRestartError(null);
    const firstPage = projectRiskOptions(project.workspace_id, project.id, signal);
    try {
      // The explicit restart establishes a new baseline, even if this first
      // page is already cached or currently being fetched in the background.
      await qc.cancelQueries({ queryKey: firstPage.queryKey, exact: true });
      await qc.fetchQuery({ ...firstPage, staleTime: 0, retry: false });
      setPage({}); setContinuationChanged(false);
    } catch (error) { setRestartError(error instanceof Error ? error : new Error(String(error))); }
    finally { setRestarting(false); }
  };
  const labels = { blocked: t(($) => $.management.blocked), overdue: t(($) => $.management.overdue), unassigned: t(($) => $.management.unassigned), in_review: t(($) => $.management.in_review) };
  return <section className="flex-1 min-h-0 overflow-y-auto p-6"><div className="mx-auto max-w-5xl space-y-4">
    <div className="flex flex-wrap items-center gap-2"><Button variant="outline" size="sm" onClick={onBack}>{t(($) => $.management.overview)}</Button><h2 className="flex-1 text-heading font-medium">{labels[signal]}{query.data && ` · ${query.data.total}`}</h2>
      <Button size="sm" variant={mode === "list" ? "secondary" : "ghost"} onClick={() => setMode("list")}>{t(($) => $.management.list)}</Button><Button size="sm" variant={mode === "table" ? "secondary" : "ghost"} onClick={() => setMode("table")}>{t(($) => $.management.table)}</Button>
    </div><p className="text-caption text-muted-foreground">{t(($) => $.management.risk_scope)}</p>
    {query.data && <p className="text-caption text-muted-foreground">{t(($) => $.management.calculated, { time: new Date(query.data.overview.statistics.calculated_at).toLocaleString(), timezone: query.data.overview.statistics.timezone, date: query.data.overview.statistics.reference_date })}</p>}
    {showContinuationWarning ? <p role="status" className="text-caption text-warning">{t(($) => $.management.continuation_changed)}</p>
      : !cursor && query.data?.refreshed && <p role="status" className="text-caption text-warning">{t(($) => $.management.refreshed)}</p>}
    {(query.error || restartError) && <div role="alert"><p>{t(($) => $.management.load_error)}</p><Button disabled={restarting} onClick={() => restartError ? void restart() : void query.refetch()}>{t(($) => $.management.retry)}</Button></div>}
    {query.isPending && <p role="status">{t(($) => $.management.loading)}</p>}
    {query.data?.items.length === 0 && <p className="text-caption text-muted-foreground">{cursor && query.data.total > 0 ? t(($) => $.management.empty_risk_suffix) : t(($) => $.management.no_risks)}</p>}
    {mode === "table" ? <div className="overflow-x-auto"><table className="w-full text-left text-caption"><thead className="text-muted-foreground"><tr><th className="p-2">{t(($) => $.table.name)}</th><th className="p-2">{t(($) => $.table.status)}</th><th className="p-2">{t(($) => $.detail.prop_due_date)}</th></tr></thead><tbody>{query.data?.items.map((issue) => <tr key={issue.id} className="border-t"><td className="p-2"><AppLink className="block rounded-sm py-1 hover:underline" href={paths.issueDetail(issue.identifier ?? issue.id)}>{issue.identifier} {issue.title}</AppLink></td><td className="p-2">{issue.status}</td><td className="p-2">{issue.due_date ?? t(($) => $.detail.no_due_date)}</td></tr>)}</tbody></table></div>
      : <ul className="divide-y">{query.data?.items.map((issue) => <li key={issue.id}><AppLink href={paths.issueDetail(issue.identifier ?? issue.id)} className="flex items-start gap-3 rounded-sm px-2 py-3 text-caption hover:bg-accent"><span className="shrink-0 text-muted-foreground">{issue.identifier}</span><span className="min-w-0 flex-1 break-words">{issue.title}</span><span className="text-muted-foreground">{issue.status}</span></AppLink></li>)}</ul>}
    <div className="flex flex-wrap gap-2"><Button variant="outline" size="sm" disabled={restarting || query.isFetching} onClick={() => void restart()}>{t(($) => $.management.restart_risks)}</Button>{query.data?.next_cursor && <Button variant="outline" size="sm" disabled={restarting || query.isFetching} onClick={() => setPage({ cursor: query.data!.next_cursor!, version: query.data!.snapshot_version })}>{t(($) => $.management.next)}</Button>}</div>
  </div></section>;
}
