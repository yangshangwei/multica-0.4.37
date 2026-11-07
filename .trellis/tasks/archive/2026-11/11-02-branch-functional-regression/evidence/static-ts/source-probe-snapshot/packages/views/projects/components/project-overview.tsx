"use client";
import { useEffect } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { projectOverviewOptions, projectP1Keys } from "@multica/core/projects";
import type { Project, ProjectOverview as Overview, ProjectRiskSignal } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Activity, Ban, CheckCircle2, ChevronDown, CircleAlert, CircleDashed, CircleHelp, ClipboardCheck, Clock3, Info, TriangleAlert, UserRoundX } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { cn } from "@multica/ui/lib/utils";
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
export function ProjectOverviewPanel({ project, updatesSupported, onRisk, onProtectedError, targetUpdateId }: {
  project: Project; updatesSupported: boolean; targetUpdateId?: string;
  onRisk: (signal: ProjectRiskSignal, version: string) => void; onProtectedError: () => void;
}) {
  const { t } = useT("projects"); const qc = useQueryClient(); const query = useQuery(projectOverviewOptions(project.workspace_id, project.id));
  useProjectAccessGuard(query.error, project.workspace_id, project.id, onProtectedError);
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
    project_overdue: t(($) => $.management.project_overdue),
    unassigned_issues: t(($) => $.management.reasons.unassigned_issues),
    in_review_issues: t(($) => $.management.reasons.in_review_issues),
    invalid_project_lead: t(($) => $.management.lead_invalid),
    stale_progress: t(($) => $.management.reasons.stale_progress),
    ended_project_open_issues: t(($) => $.management.reasons.ended_project_open_issues),
    execution_environment_unavailable: t(($) => $.management.environment),
    unknown_status: t(($) => $.management.reasons.unknown_status),
  };
  const riskLabels: Record<ProjectRiskSignal, string> = { blocked: t(($) => $.management.blocked), overdue: t(($) => $.management.overdue), unassigned: t(($) => $.management.unassigned), in_review: t(($) => $.management.in_review) };
  const riskIcons: Record<ProjectRiskSignal, LucideIcon> = { blocked: Ban, overdue: Clock3, unassigned: UserRoundX, in_review: ClipboardCheck };
  const healthState = query.error || !statistics.complete ? "unavailable" : statistics.health;
  const healthPresentation: Record<Overview["statistics"]["health"], { Icon: LucideIcon; className: string }> = {
    risk: { Icon: TriangleAlert, className: "bg-destructive/10 text-destructive-foreground" },
    attention: { Icon: CircleAlert, className: "bg-warning/10 text-warning-foreground" },
    clear: { Icon: CheckCircle2, className: "bg-muted text-foreground" },
    empty: { Icon: CircleDashed, className: "bg-muted text-muted-foreground" },
    unavailable: { Icon: CircleHelp, className: "bg-muted text-muted-foreground" },
  };
  const { Icon: HealthIcon, className: healthClass } = healthPresentation[healthState];
  const riskLinksDisabled = healthState === "unavailable";
  const taskReasons: Partial<Record<string, ProjectRiskSignal>> = {
    blocked_issues: "blocked", overdue_issues: "overdue", unassigned_issues: "unassigned", in_review_issues: "in_review",
  };
  const reasonMessages = new Map(statistics.reasons.map((reason) => [reason, reasons[reason] ?? t(($) => $.management.attention)]));
  if (statistics.project_overdue) reasonMessages.set("project_overdue", reasons.project_overdue!);
  if (statistics.lead_valid === false) reasonMessages.set("invalid_project_lead", reasons.invalid_project_lead!);
  if (statistics.counts.execution_environment_unavailable) reasonMessages.set("execution_environment_unavailable", reasons.execution_environment_unavailable!);
  if (project.start_date && project.due_date && project.start_date > project.due_date) reasonMessages.set("date_invalid", t(($) => $.management.date_invalid));
  if (project.in_progress_since_source === "migration") reasonMessages.set("migration_since", t(($) => $.management.migration_since));
  const detailReasons: [string, string][] = [];
  const visibleReasons: [string, string][] = [];
  for (const [reason, message] of reasonMessages) {
    const signal = taskReasons[reason];
    // Only fold explanations already represented by a visible task count.
    (signal && (statistics.counts[signal] ?? 0) > 0 ? detailReasons : visibleReasons).push([reason, message]);
  }
  return <div className="flex-1 min-h-0 overflow-y-auto"><div className="mx-auto max-w-5xl space-y-8 px-6 pt-6 pb-28 md:px-8 md:pt-8">
    {query.error && <div role="alert" className="space-y-2 text-caption text-warning"><p>{t(($) => $.management.refresh_failed)}</p><Button size="sm" variant="outline" onClick={() => void query.refetch()}>{t(($) => $.management.retry)}</Button></div>}
    <section className="space-y-4"><div className="flex items-center justify-between gap-4"><h2 className="text-heading font-medium">{t(($) => $.management.overview)}</h2><Button size="sm" variant="outline" onClick={() => void query.refetch()} disabled={query.isFetching}>{t(($) => $.management.refresh)}</Button></div>
      <ProjectStatistics overview={overview} />
      <p className="text-caption text-muted-foreground">{t(($) => $.management.calculated, { time: new Date(statistics.calculated_at).toLocaleString(), date: statistics.reference_date })}</p>
    </section>
    <section className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="flex items-center gap-2 text-title-sm font-medium">
          <Activity className="size-4 text-muted-foreground" aria-hidden="true" />
          {t(($) => $.management.health)}
        </h2>
        <span className={cn("inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-caption font-medium", healthClass)}>
          <HealthIcon className="size-3.5 shrink-0" aria-hidden="true" />
          {healthLabels[healthState]}
        </span>
      </div>
      <div className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,9rem),1fr))] gap-2">
        {(Object.keys(riskLabels) as ProjectRiskSignal[]).map((signal) => {
          const Icon = riskIcons[signal];
          const count = statistics.counts[signal];
          const countLabel = count ?? t(($) => $.management.na);
          return <Button key={signal} variant="outline" disabled={riskLinksDisabled}
            onClick={() => onRisk(signal, statistics.snapshot_version)}
            aria-label={`${riskLabels[signal]} ${countLabel}`}
            className="h-auto min-h-12 justify-between gap-3 whitespace-normal px-3 py-3 text-left">
            <span className="flex min-w-0 items-center gap-2 text-caption text-muted-foreground">
              <Icon className="size-4 shrink-0" aria-hidden="true" />
              {riskLabels[signal]}
            </span>
            <span className={cn("tabular-nums", count === null ? "text-caption" : "text-title-sm")}>{countLabel}</span>
          </Button>;
        })}
      </div>
      {visibleReasons.length > 0 && <ul className="space-y-2 text-caption">
        {visibleReasons.map(([reason, message]) => {
          const informational = query.error || reason === "migration_since";
          const Icon = informational ? Info : TriangleAlert;
          return <li key={reason} className={cn("flex items-start gap-2", informational ? "text-muted-foreground" : "text-warning-foreground")}>
            <Icon className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
            <span>{message}</span>
          </li>;
        })}
      </ul>}
      {detailReasons.length > 0 && <details className="group/health">
        <summary className="flex min-h-8 w-fit cursor-pointer list-none items-center gap-1 rounded-sm text-caption text-muted-foreground outline-none focus-visible:ring-2 focus-visible:ring-ring pointer-coarse:min-h-11 [&::-webkit-details-marker]:hidden">
          {t(($) => $.management.health_details)}
          <ChevronDown className="size-3.5 group-open/health:rotate-180" aria-hidden="true" />
        </summary>
        <ul className="mt-1 list-disc space-y-1 pl-5 text-caption text-muted-foreground">
          {detailReasons.map(([reason, message]) => <li key={reason}>{message}</li>)}
        </ul>
      </details>}
      {!statistics.complete && <p role="status" className="text-caption text-warning-foreground">
        {statistics.incomplete_reasons.map((reason) => reasons[reason] ?? t(($) => $.management.unavailable)).join(" · ")}
      </p>}
    </section>
    <section className="space-y-3"><h2 className="text-heading font-medium">{t(($) => $.management.acceptance)}</h2><ProjectAcceptance overview={overview} /></section>
    {updatesSupported && <ProjectProgress project={project} onProtectedError={onProtectedError} targetUpdateId={targetUpdateId} />}
  </div></div>;
}
