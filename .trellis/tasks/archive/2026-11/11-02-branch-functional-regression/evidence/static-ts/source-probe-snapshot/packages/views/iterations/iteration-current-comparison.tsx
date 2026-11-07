"use client";

import { useId, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api, ApiError, isIterationAccessDenied } from "@multica/core/api";
import { protectIterationRead } from "@multica/core/iterations";
import { projectDetailOptions } from "@multica/core/projects";
import { memberListOptions, agentListOptions, squadListOptions } from "@multica/core/workspace/queries";
import { useWorkspacePaths } from "@multica/core/paths";
import { AppLink } from "../navigation";
import { priorityLabel } from "../issues/utils/priority-label";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { useT } from "../i18n";
import { IterationError } from "./iteration-error";
import { useIterationLabels } from "./labels";

type HistoricalIssue = Awaited<ReturnType<typeof api.getIterationIssues>>["items"][number];

export function IterationCurrentComparison({ wsId, issue, trigger }: { wsId: string; issue: HistoricalIssue; trigger?: string }) {
  const { t } = useT("projects");
  const labels = useIterationLabels();
  const paths = useWorkspacePaths();
  const { t: issueT } = useT("issues");
  const [open, setOpen] = useState(false);
  const comparisonId = useId();
  const current = useQuery({
    queryKey: ["iterations", wsId, "current-comparison", issue.issue_id],
    queryFn: ({ client, signal }) => protectIterationRead(client, wsId, () => api.getIssue(issue.issue_id, { signal })),
    enabled: open,
    retry: false,
  });
  const project = useQuery({ ...projectDetailOptions(wsId, current.data?.project_id ?? ""), enabled: open && !!current.data?.project_id });
  const members = useQuery({ ...memberListOptions(wsId), enabled: open && current.data?.assignee_type === "member" });
  const agents = useQuery({ ...agentListOptions(wsId), enabled: open && current.data?.assignee_type === "agent" });
  const squads = useQuery({ ...squadListOptions(wsId), enabled: open && current.data?.assignee_type === "squad" });
  const currentActor = current.data?.assignee_type === "member" ? members.data?.find((item) => item.user_id === current.data?.assignee_id)?.name : current.data?.assignee_type === "agent" ? agents.data?.find((item) => item.id === current.data?.assignee_id)?.name : squads.data?.find((item) => item.id === current.data?.assignee_id)?.name;
  const unknown = t(($) => $.iterations.unknownHistory);
  const showLabels = (value: unknown) => !Array.isArray(value) ? unknown : value.every((item) => item && typeof item === "object" && typeof item.name === "string") ? value.map((item) => item.name).join(", ") || t(($) => $.iterations.none) : unknown;
  const none = t(($) => $.iterations.none);
  const fields = [
    [t(($) => $.iterations.activityPanel.title), issue.title, current.data?.title],
    [t(($) => $.iterations.status), labels.category(issue.status_category), current.data ? labels.category(current.data.status_category ?? current.data.status) : unknown],
    [t(($) => $.iterations.project), issue.project_id ? issue.project_name ?? unknown : none, current.data?.project_id ? project.data?.title ?? unknown : none],
    [t(($) => $.iterations.assignee), issue.assignee_id ? issue.assignee_name ?? unknown : none, current.data?.assignee_id ? currentActor ?? unknown : none],
    [t(($) => $.iterations.priorityFilter), typeof issue.priority === "string" ? priorityLabel(issue.priority, issueT) : unknown, typeof current.data?.priority === "string" ? priorityLabel(current.data.priority, issueT) : unknown],
    [t(($) => $.iterations.labelFilter), showLabels(issue.labels), showLabels(current.data?.labels)],
  ];
  return (
    <div className="min-w-0 space-y-3">
      <Button variant="ghost" size="sm" className="h-auto min-h-7 max-w-full justify-start whitespace-normal px-0 text-left leading-5 [overflow-wrap:anywhere] pointer-coarse:min-h-11" onClick={() => setOpen((value) => !value)} aria-expanded={open} aria-controls={comparisonId}>
        {trigger ?? t(($) => $.iterations.compareCurrent)}
      </Button>
      {open && <div id={comparisonId} className="min-w-0 space-y-4 rounded-lg bg-muted p-4">
        {current.isPending && <div role="status" className="space-y-3"><span className="sr-only">{t(($) => $.iterations.loading)}</span><Skeleton className="h-5 w-2/3" /><Skeleton className="h-24 w-full" /></div>}
        {current.error && (current.error instanceof ApiError && current.error.status === 404 && !isIterationAccessDenied(current.error)
          ? <p className="text-body text-muted-foreground">{t(($) => $.iterations.deletedOrUnavailable)}</p>
          : <IterationError error={current.error} />)}
        {!current.error && current.data && <>
          <div className="grid gap-6 sm:grid-cols-2">
            {[t(($) => $.iterations.historicalValue), t(($) => $.iterations.currentValue)].map((title, index) => <section key={title} className="min-w-0 space-y-3">
              <h3 className="text-body font-semibold">{title}</h3>
              <dl className="space-y-3">{fields.map(([label, historical, latest]) => <div key={label} className="min-w-0 space-y-1">
                <dt className="text-caption text-muted-foreground">{label}</dt>
                <dd className="whitespace-pre-wrap text-body [overflow-wrap:anywhere]">{index === 0 ? historical : latest}</dd>
              </div>)}</dl>
            </section>)}
          </div>
          <AppLink href={paths.issueDetail(issue.issue_id)} className="inline-flex min-h-8 items-center rounded text-caption font-medium underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-foreground pointer-coarse:min-h-11">{t(($) => $.iterations.openCurrent)}</AppLink>
        </>}
      </div>}
    </div>
  );
}
