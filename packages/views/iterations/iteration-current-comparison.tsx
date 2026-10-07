"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api, ApiError, isIterationAccessDenied } from "@multica/core/api";
import { protectIterationRead } from "@multica/core/iterations";
import { projectDetailOptions } from "@multica/core/projects";
import { memberListOptions, agentListOptions, squadListOptions } from "@multica/core/workspace/queries";
import { useWorkspacePaths } from "@multica/core/paths";
import { AppLink } from "../navigation";
import { priorityLabel } from "../issues/utils/priority-label";
import { Button } from "@multica/ui/components/ui/button";
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
  return (
    <div className="mt-2 space-y-2">
      <Button variant="ghost" size="sm" onClick={() => setOpen((value) => !value)} aria-expanded={open}>
        {trigger ?? t(($) => $.iterations.compareCurrent)}
      </Button>
      {open && current.isPending && <p role="status">{t(($) => $.iterations.loading)}</p>}
      {open && current.error && (current.error instanceof ApiError && current.error.status === 404 && !isIterationAccessDenied(current.error)
        ? <p>{t(($) => $.iterations.deletedOrUnavailable)}</p>
        : <IterationError error={current.error} />)}
      {open && !current.error && current.data && <><dl className="grid gap-2 sm:grid-cols-2">
        <div><dt className="font-medium">{t(($) => $.iterations.historicalValue)}</dt><dd>{issue.title} · {labels.category(issue.status_category)} · {issue.project_name ?? t(($) => $.iterations.none)} · {issue.assignee_name ?? t(($) => $.iterations.none)} · {typeof issue.priority === "string" ? priorityLabel(issue.priority, issueT) : unknown} · {showLabels(issue.labels)}</dd></div>
        <div><dt className="font-medium">{t(($) => $.iterations.currentValue)}</dt><dd>{current.data.title} · {labels.category(current.data.status_category ?? current.data.status)} · {current.data.project_id ? project.data?.title ?? unknown : t(($) => $.iterations.none)} · {current.data.assignee_id ? currentActor ?? unknown : t(($) => $.iterations.none)} · {typeof current.data.priority === "string" ? priorityLabel(current.data.priority, issueT) : unknown} · {showLabels(current.data.labels)}</dd></div>
      </dl><AppLink href={paths.issueDetail(issue.issue_id)}>{t(($) => $.iterations.openCurrent)}</AppLink></>}
    </div>
  );
}
