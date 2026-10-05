"use client";
import { useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { projectOverviewOptions, projectP1Keys, useProjectPlanningTimezone } from "@multica/core/projects";
import type { Project, ProjectOverview as Overview, ProjectRiskSignal } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { useT } from "../../i18n";
import { ProjectProgress } from "./project-progress";
import { useProjectAccessGuard } from "./use-project-access-guard";

export function ProjectStatistics({ overview }: { overview: Overview }) {
  const { t } = useT("projects"); const stats = overview.statistics;
  return <dl className="grid grid-cols-2 gap-x-6 gap-y-4 sm:grid-cols-5">
    {([["total", t(($) => $.management.total)], ["completed", t(($) => $.management.completed)],
      ["cancelled", t(($) => $.management.cancelled)], ["open", t(($) => $.management.open)]] as const).map(([key, title]) =>
      <div key={key}><dt className="text-caption text-muted-foreground">{title}</dt><dd className="mt-1 text-title tabular-nums">{stats.counts[key] ?? t(($) => $.management.na)}</dd></div>)}
    <div><dt className="text-caption text-muted-foreground">{t(($) => $.management.closure)}</dt><dd className="mt-1 text-title tabular-nums">{stats.closure_ratio === null ? t(($) => $.management.na) : `${Math.round(stats.closure_ratio * 100)}%`}</dd></div>
  </dl>;
}
export function ProjectAcceptance({ overview }: { overview: Overview }) {
  const { t } = useT("projects");
  const labels = { passed: t(($) => $.management.passed), partial: t(($) => $.management.partial), failed: t(($) => $.management.failed) };
  return <div className="space-y-2 text-caption">
    {[{ label: t(($) => $.management.current_acceptance), value: overview.current_description_acceptance }, { label: t(($) => $.management.latest_acceptance), value: overview.latest_acceptance }].map(({ label, value }) => <div key={label}>
      <span className="text-muted-foreground">{label}: </span>{value ? <>
        <span className="font-medium">{labels[value.conclusion]}</span><span className="ml-2">{t(($) => $.management.description_version, { revision: value.description_revision })}</span>
        {!value.applicable_to_current_description && <span className="ml-2 text-warning">{t(($) => $.management.stale_acceptance)}</span>}
      </> : t(($) => $.management.no_acceptance)}
    </div>)}
  </div>;
}
export function ProjectOverviewPanel({ project, canEditTimezone, updatesSupported, onRisk, onProtectedError }: {
  project: Project; canEditTimezone: boolean; updatesSupported: boolean;
  onRisk: (signal: ProjectRiskSignal, version: string) => void; onProtectedError: () => void;
}) {
  const { t } = useT("projects"); const qc = useQueryClient(); const query = useQuery(projectOverviewOptions(project.workspace_id, project.id));
  useProjectAccessGuard(query.error, project.workspace_id, project.id, onProtectedError);
  const [timezone, setTimezone] = useState(""); const changeTimezone = useProjectPlanningTimezone(project.workspace_id);
  const stats = query.data?.statistics;
  // A minute poll only checks the planning calendar day; it does not fetch on
  // every tick. It catches DST and suspended/background-tab wakeups.
  useEffect(() => {
    if (!stats) return;
    const check = () => {
      const date = new Intl.DateTimeFormat("en-CA", { timeZone: stats.timezone, year: "numeric", month: "2-digit", day: "2-digit" }).format(new Date());
      if (date !== stats.reference_date) void qc.invalidateQueries({ queryKey: projectP1Keys.all(project.workspace_id, project.id) });
    };
    const timer = setInterval(check, 60_000); window.addEventListener("focus", check);
    return () => { clearInterval(timer); window.removeEventListener("focus", check); };
  }, [stats, project.workspace_id, project.id, qc]);
  if (query.isPending) return <p role="status" className="p-6 text-caption text-muted-foreground">{t(($) => $.management.loading)}</p>;
  if (!query.data) return <div role="alert" className="p-6"><p>{t(($) => $.management.load_error)}</p><Button onClick={() => void query.refetch()}>{t(($) => $.management.retry)}</Button></div>;
  const overview = query.data; const statistics = overview.statistics;
  const healthLabels = { unavailable: t(($) => $.management.unavailable), empty: t(($) => $.management.empty), risk: t(($) => $.management.risk), attention: t(($) => $.management.attention), clear: t(($) => $.management.clear) };
  const reasons: Record<string, string> = {
    blocked_issues: t(($) => $.management.reasons.blocked_issues),
    overdue_issues: t(($) => $.management.reasons.overdue_issues),
    project_overdue: t(($) => $.management.reasons.project_overdue),
    unassigned_issues: t(($) => $.management.reasons.unassigned_issues),
    in_review_issues: t(($) => $.management.reasons.in_review_issues),
    invalid_project_lead: t(($) => $.management.reasons.invalid_project_lead),
    stale_progress: t(($) => $.management.reasons.stale_progress),
    ended_project_open_issues: t(($) => $.management.reasons.ended_project_open_issues),
    execution_environment_unavailable: t(($) => $.management.reasons.execution_environment_unavailable),
    unknown_status: t(($) => $.management.reasons.unknown_status),
  };
  const riskLabels: Record<ProjectRiskSignal, string> = { blocked: t(($) => $.management.blocked), overdue: t(($) => $.management.overdue), unassigned: t(($) => $.management.unassigned), in_review: t(($) => $.management.in_review) };
  return <div className="flex-1 min-h-0 overflow-y-auto"><div className="mx-auto max-w-5xl space-y-8 p-6 md:p-8">
    <section className="space-y-4"><div className="flex items-center justify-between gap-4"><h2 className="text-heading font-medium">{t(($) => $.management.overview)}</h2><Button size="sm" variant="outline" onClick={() => void query.refetch()} disabled={query.isFetching}>{t(($) => $.management.refresh)}</Button></div>
      <ProjectStatistics overview={overview} />
      <p className="text-caption text-muted-foreground">{t(($) => $.management.calculated, { time: new Date(statistics.calculated_at).toLocaleString(), timezone: statistics.timezone, date: statistics.reference_date })}</p>
    </section>
    <section className="space-y-4"><h2 className="text-heading font-medium">{t(($) => $.management.health)}</h2>
      <p className={statistics.health === "risk" ? "text-destructive" : statistics.health === "attention" ? "text-warning" : "text-muted-foreground"}>{healthLabels[statistics.health]}</p>
      <div className="flex flex-wrap gap-2">{(Object.keys(riskLabels) as ProjectRiskSignal[]).map((signal) => <Button key={signal} variant="outline" disabled={!statistics.complete} onClick={() => onRisk(signal, statistics.snapshot_version)}>{riskLabels[signal]} <span className="tabular-nums">{statistics.counts[signal] ?? t(($) => $.management.na)}</span></Button>)}</div>
      {statistics.reasons.length > 0 && <ul className="list-disc space-y-1 pl-5 text-caption text-muted-foreground">{statistics.reasons.map((reason) => <li key={reason}>{reasons[reason] ?? t(($) => $.management.attention)}</li>)}</ul>}
      {!statistics.complete && <p role="status" className="text-caption text-warning">{statistics.incomplete_reasons.map((reason) => reasons[reason] ?? t(($) => $.management.unavailable)).join(" · ")}</p>}
      {statistics.project_overdue && <p className="text-caption text-warning">{t(($) => $.management.project_overdue)}</p>}
      {statistics.lead_valid === false && <p className="text-caption text-warning">{t(($) => $.management.lead_invalid)}</p>}
      {!!statistics.counts.execution_environment_unavailable && <p className="text-caption text-warning">{t(($) => $.management.environment)}</p>}
      {project.start_date && project.due_date && project.start_date > project.due_date && <p className="text-caption text-warning">{t(($) => $.management.date_invalid)}</p>}
      {project.in_progress_since_source === "migration" && <p className="text-caption text-muted-foreground">{t(($) => $.management.migration_since)}</p>}
    </section>
    <section className="space-y-3"><h2 className="text-heading font-medium">{t(($) => $.management.acceptance)}</h2><ProjectAcceptance overview={overview} /></section>
    {updatesSupported && <ProjectProgress project={project} onProtectedError={onProtectedError} />}
    <section className="space-y-3"><h2 className="text-heading font-medium">{t(($) => $.management.timezone)}</h2>
      <p className="text-caption text-muted-foreground">{statistics.timezone_configured ? statistics.timezone : t(($) => $.management.utc_default)}</p>
      {canEditTimezone && <form className="max-w-lg space-y-2" onSubmit={(event) => { event.preventDefault(); changeTimezone.mutate(timezone.trim() || null); }}>
        <label className="text-caption" htmlFor="project-timezone">{t(($) => $.management.timezone_hint)}</label><div className="flex gap-2"><Input id="project-timezone" value={timezone} onChange={(event) => setTimezone(event.target.value)} /><Button type="submit" disabled={changeTimezone.isPending}>{t(($) => $.management.save)}</Button></div>
        {changeTimezone.error && <p role="alert" className="text-caption text-destructive">{changeTimezone.error.message}</p>}
      </form>}
    </section>
  </div></div>;
}
