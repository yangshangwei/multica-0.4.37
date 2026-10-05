"use client";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
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
  const query = useQuery(projectRiskOptions(project.workspace_id, project.id, signal, cursor, page.version));
  useProjectAccessGuard(query.error, project.workspace_id, project.id, onProtectedError);
  const labels = { blocked: t(($) => $.management.blocked), overdue: t(($) => $.management.overdue), unassigned: t(($) => $.management.unassigned), in_review: t(($) => $.management.in_review) };
  return <section className="flex-1 min-h-0 overflow-y-auto p-6"><div className="mx-auto max-w-5xl space-y-4">
    <div className="flex flex-wrap items-center gap-2"><Button variant="outline" size="sm" onClick={onBack}>{t(($) => $.management.overview)}</Button><h2 className="flex-1 text-heading font-medium">{labels[signal]}{query.data && ` · ${query.data.total}`}</h2>
      <Button size="sm" variant={mode === "list" ? "secondary" : "ghost"} onClick={() => setMode("list")}>{t(($) => $.management.list)}</Button><Button size="sm" variant={mode === "table" ? "secondary" : "ghost"} onClick={() => setMode("table")}>{t(($) => $.management.table)}</Button>
    </div><p className="text-caption text-muted-foreground">{t(($) => $.management.risk_scope)}</p>
    {query.data && <p className="text-caption text-muted-foreground">{t(($) => $.management.calculated, { time: new Date(query.data.overview.statistics.calculated_at).toLocaleString(), timezone: query.data.overview.statistics.timezone, date: query.data.overview.statistics.reference_date })}</p>}
    {query.data?.refreshed && <p role="status" className="text-caption text-warning">{t(($) => $.management.refreshed)}</p>}
    {query.error && <div role="alert"><p>{t(($) => $.management.load_error)}</p><Button onClick={() => void query.refetch()}>{t(($) => $.management.retry)}</Button></div>}
    {query.isPending && <p role="status">{t(($) => $.management.loading)}</p>}
    {query.data?.items.length === 0 && <p className="text-caption text-muted-foreground">{t(($) => $.management.no_risks)}</p>}
    {mode === "table" ? <div className="overflow-x-auto"><table className="w-full text-left text-caption"><thead className="text-muted-foreground"><tr><th className="p-2">{t(($) => $.table.name)}</th><th className="p-2">{t(($) => $.table.status)}</th><th className="p-2">{t(($) => $.detail.prop_due_date)}</th></tr></thead><tbody>{query.data?.items.map((issue) => <tr key={issue.id} className="border-t"><td className="p-2"><AppLink className="block rounded-sm py-1 hover:underline" href={paths.issueDetail(issue.identifier ?? issue.id)}>{issue.identifier} {issue.title}</AppLink></td><td className="p-2">{issue.status}</td><td className="p-2">{issue.due_date ?? t(($) => $.detail.no_due_date)}</td></tr>)}</tbody></table></div>
      : <ul className="divide-y">{query.data?.items.map((issue) => <li key={issue.id}><AppLink href={paths.issueDetail(issue.identifier ?? issue.id)} className="flex items-start gap-3 rounded-sm px-2 py-3 text-caption hover:bg-accent"><span className="shrink-0 text-muted-foreground">{issue.identifier}</span><span className="min-w-0 flex-1 break-words">{issue.title}</span><span className="text-muted-foreground">{issue.status}</span></AppLink></li>)}</ul>}
    <div className="flex gap-2">{cursor && <Button variant="outline" size="sm" onClick={() => setPage({ version: query.data?.snapshot_version ?? version })}>{t(($) => $.management.previous)}</Button>}{query.data?.next_cursor && <Button variant="outline" size="sm" onClick={() => setPage({ cursor: query.data!.next_cursor!, version: query.data!.snapshot_version })}>{t(($) => $.management.next)}</Button>}</div>
  </div></section>;
}
