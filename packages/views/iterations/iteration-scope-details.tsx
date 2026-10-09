"use client";

import { useId, useState } from "react";
import type { api } from "@multica/core/api";
import { useWorkspacePaths } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@multica/ui/components/ui/select";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { useT } from "../i18n";
import { AppLink } from "../navigation";
import { IterationCurrentComparison } from "./iteration-current-comparison";
import { useIterationLabels } from "./labels";

type HistoricalIssue = Awaited<ReturnType<typeof api.getIterationIssues>>["items"][number];

export function IterationScopeSelect<Value extends string>({ id, label, value, options, onValueChange }: {
  id: string;
  label: string;
  value: Value;
  options: { value: Value; label: string }[];
  onValueChange: (value: Value) => void;
}) {
  return <div className="min-w-0 space-y-2">
    <Label htmlFor={id}>{label}</Label>
    <Select items={options} value={value} onValueChange={(next) => { if (next !== null) onValueChange(next); }}>
      <SelectTrigger id={id} className="w-full min-w-0 pointer-coarse:min-h-11"><SelectValue /></SelectTrigger>
      <SelectContent alignItemWithTrigger={false}>{options.map((option) => <SelectItem key={option.value} value={option.value} className="pointer-coarse:min-h-11"><span className="min-w-0 whitespace-normal [overflow-wrap:anywhere]">{option.label}</span></SelectItem>)}</SelectContent>
    </Select>
  </div>;
}

/** The parent owns protected reads and revision checks; this view only refines a verified source. */
export function IterationScopeDetails({ wsId, data, historical, stale, error, loading, fetching, canRetry, onRetry }: {
  wsId: string;
  data: { issues: readonly HistoricalIssue[]; complete: boolean } | undefined;
  historical: boolean;
  stale: boolean;
  error: unknown;
  loading: boolean;
  fetching: boolean;
  canRetry: boolean;
  onRetry: () => void;
}) {
  const { t } = useT("projects");
  const labels = useIterationLabels();
  const paths = useWorkspacePaths();
  const controlId = useId();
  const [search, setSearch] = useState("");
  const [project, setProject] = useState("");
  const [assignee, setAssignee] = useState("");
  const unknown = t(($) => $.iterations.unknownHistory);
  const none = t(($) => $.iterations.none);
  const projectKey = (issue: HistoricalIssue) => issue.project_id ?? "none";
  const assigneeKey = (issue: HistoricalIssue) => issue.assignee_id ? `${issue.assignee_type}:${issue.assignee_id}` : "none";
  const projects = new Map(data?.issues.map((issue) => [projectKey(issue), issue.project_id === null ? none : issue.project_name ?? unknown]));
  const assignees = new Map(data?.issues.map((issue) => [assigneeKey(issue), issue.assignee_id === null ? none : issue.assignee_name ?? unknown]));
  const options = (choices: Map<string, string>, selected: string) => [
    { value: "", label: t(($) => $.iterations.allValues) },
    ...(selected && !choices.has(selected) ? [{ value: selected, label: t(($) => $.iterations.missingSelection) }] : []),
    ...Array.from(choices, ([value, label]) => ({ value, label })).sort((a, b) => a.label.localeCompare(b.label)),
  ];
  const term = search.trim().toLocaleLowerCase();
  const matches = data?.issues.filter((issue) =>
    (!term || `${issue.identifier} ${issue.title}`.toLocaleLowerCase().includes(term)) &&
    (!project || projectKey(issue) === project) && (!assignee || assigneeKey(issue) === assignee));
  const retry = canRetry ? <Button variant="outline" className="pointer-coarse:min-h-11" disabled={fetching} onClick={onRetry}>{t(($) => $.iterations.retry)}</Button> : null;

  if (stale) return <div className="space-y-3"><p role="alert">{t(($) => $.iterations.activityPanel.scopeStale)}</p>{retry}</div>;
  if (!data) return error ? <div className="space-y-3"><p role="alert">{t(($) => $.iterations.activityPanel.detailReadError)}</p>{retry}</div>
    : loading ? <div role="status" className="space-y-3"><span className="sr-only">{t(($) => $.iterations.activityPanel.detailLoading)}</span><Skeleton className="h-5 w-2/3" /><Skeleton className="h-12 w-full" /></div>
      : <p role="alert">{t(($) => $.iterations.activityPanel.unavailable)}</p>;

  return <div className="min-w-0 space-y-5">
    {!!error && <div className="flex flex-wrap items-center gap-3"><p role="alert" className="text-caption">{t(($) => $.iterations.activityPanel.detailRefreshError)}</p>{retry}</div>}
    {!data.complete && <div className="flex flex-wrap items-center gap-3"><p role="alert" className="max-w-prose text-caption">{t(($) => $.iterations.activityPanel.incomplete)}</p>{retry}</div>}
    {data.complete && <p className="max-w-prose text-caption text-muted-foreground">{t(($) => historical ? $.iterations.activityPanel.historicalTaskHint : $.iterations.activityPanel.currentTaskHint)}</p>}
    <div className="grid items-end gap-3 sm:grid-cols-2 lg:grid-cols-[minmax(12rem,2fr)_minmax(8rem,1fr)_minmax(8rem,1fr)_auto]">
      <div className="min-w-0 space-y-2"><Label htmlFor={`${controlId}-search`}>{t(($) => $.iterations.activityPanel.searchTasks)}</Label><Input id={`${controlId}-search`} value={search} placeholder={t(($) => $.iterations.activityPanel.searchPlaceholder)} onChange={(event) => setSearch(event.target.value)} className="pointer-coarse:min-h-11" /></div>
      <IterationScopeSelect id={`${controlId}-project`} label={t(($) => $.iterations.project)} value={project} options={options(projects, project)} onValueChange={setProject} />
      <IterationScopeSelect id={`${controlId}-assignee`} label={t(($) => $.iterations.assignee)} value={assignee} options={options(assignees, assignee)} onValueChange={setAssignee} />
      <Button variant="ghost" className="justify-self-start pointer-coarse:min-h-11" disabled={!search && !project && !assignee} onClick={() => { setSearch(""); setProject(""); setAssignee(""); }}>{t(($) => $.iterations.pages.clearFilters)}</Button>
    </div>
    <p role="status" className="text-caption text-muted-foreground tabular-nums">{t(($) => data.complete ? $.iterations.activityPanel.matchingTasks : $.iterations.activityPanel.availableTasks, { count: matches?.length ?? 0 })}</p>
    {matches?.length === 0 ? <p className="py-5 text-muted-foreground">{t(($) => !data.complete && !search && !project && !assignee ? $.iterations.activityPanel.unavailable : $.iterations.activityPanel.noMatchingTasks)}</p> : <ul className="space-y-5">{matches?.map((issue) => <li key={issue.issue_id} className="min-w-0 space-y-2 [overflow-wrap:anywhere]">
      {historical ? <p className="font-medium"><span className="mr-2 text-caption text-muted-foreground">{issue.identifier}</span><span>{issue.title}</span></p>
        : <AppLink href={paths.issueDetail(issue.issue_id)} aria-label={t(($) => $.iterations.activityPanel.currentTaskLink, { task: `${issue.identifier} ${issue.title}` })} className="inline-flex min-h-8 max-w-full flex-wrap items-baseline gap-x-2 rounded font-medium hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-foreground pointer-coarse:min-h-11"><span className="text-caption text-muted-foreground">{issue.identifier}</span><span>{issue.title}</span></AppLink>}
      <p className="text-caption text-muted-foreground">{labels.category(issue.status_category)} · {issue.project_id === null ? none : issue.project_name ?? unknown} · {issue.assignee_id === null ? none : issue.assignee_name ?? unknown}</p>
      {historical && <IterationCurrentComparison wsId={wsId} issue={issue} />}
    </li>)}</ul>}
  </div>;
}
