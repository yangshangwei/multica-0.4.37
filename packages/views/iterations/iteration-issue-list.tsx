"use client";

import { useId, useState, type ReactNode } from "react";
import { Circle, CircleCheck, CirclePlay, CircleX } from "lucide-react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { api } from "@multica/core/api";
import { iterationIssuesOptions, iterationGroupedIssuesOptions, hasIterationReadAccess } from "@multica/core/iterations";
import { useWorkspacePaths } from "@multica/core/paths";
import { Input } from "@multica/ui/components/ui/input";
import { Button } from "@multica/ui/components/ui/button";
import { Label } from "@multica/ui/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@multica/ui/components/ui/select";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { AppLink } from "../navigation";
import { useT } from "../i18n";
import { useStatusLabel } from "../issues/utils/status-label";
import { priorityLabel } from "../issues/utils/priority-label";
import { IterationCurrentComparison } from "./iteration-current-comparison";
import { IterationError, isDefinitiveReadError } from "./iteration-error";
import { useIterationLabels } from "./labels";
import { iterationDisclosureClass } from "./iteration-presentation";

type HistoricalIssue = Awaited<ReturnType<typeof api.getIterationIssues>>["items"][number];
type Grouping = "none" | "status" | "assignee" | "project" | "priority" | "label";
type Scope = "current" | "original";
type IssueListProps = { wsId: string; id: string; historical: boolean; emptyState?: ReactNode };

function TaskSelect<Value extends string>({ id, label, value, options, onValueChange, disabled, describedBy }: {
  id: string;
  label: string;
  value: Value;
  options: { value: Value; label: string }[];
  onValueChange: (value: Value) => void;
  disabled?: boolean;
  describedBy?: string;
}) {
  return <div className="min-w-0 space-y-2">
    <Label htmlFor={id}>{label}</Label>
    <Select items={options} value={value} onValueChange={(next) => { if (next !== null) onValueChange(next); }} disabled={disabled}>
      <SelectTrigger id={id} className="w-full min-w-0 pointer-coarse:min-h-11" aria-describedby={describedBy}><SelectValue /></SelectTrigger>
      <SelectContent alignItemWithTrigger={false}>{options.map((option) => <SelectItem key={option.value} value={option.value} className="pointer-coarse:min-h-11"><span className="min-w-0 whitespace-normal [overflow-wrap:anywhere]">{option.label}</span></SelectItem>)}</SelectContent>
    </Select>
  </div>;
}

export function IterationIssueList(props: IssueListProps) {
  return <IterationIssueListContent key={`${props.wsId}:${props.id}`} {...props} />;
}

function IterationIssueListContent({ wsId, id, historical, emptyState }: IssueListProps) {
  const { t } = useT("projects");
  const { t: issueT } = useT("issues");
  const client = useQueryClient();
  const controlId = useId();
  const labels = useIterationLabels();
  const statusLabel = useStatusLabel(wsId);
  const paths = useWorkspacePaths();
  const [pageCursors, setPageCursors] = useState<string[]>([""]);
  const cursor = pageCursors[pageCursors.length - 1]!;
  const resetPagination = () => setPageCursors([""]);
  const [scope, setScope] = useState<Scope>("current");
  const [search, setSearch] = useState("");
  const [filters, setFilters] = useState<Record<string, string>>({});
  const [groupBy, setGroupBy] = useState<Grouping>("none");
  const params = {
    ...(scope === "original" ? { scope } : {}),
    ...(search ? { search } : {}),
    ...filters,
  };
  const page = useQuery({
    ...iterationIssuesOptions(wsId, id, { ...params, ...(cursor ? { cursor } : {}) }),
    enabled: groupBy === "none",
  });
  const grouped = useQuery({
    ...iterationGroupedIssuesOptions(wsId, id, params),
    enabled: groupBy !== "none",
  });
  const result = groupBy === "none" ? page : grouped;
  const data = hasIterationReadAccess(client, wsId) && !isDefinitiveReadError(result.error) ? result.data : undefined;
  const choices = data?.filter_options;
  const none = t(($) => $.iterations.none);
  const unknown = t(($) => $.iterations.unknownHistory);
  function changeFilter(key: string, value: string) {
    setFilters((previous) => {
      const next = { ...previous };
      if (key === "assignee") {
        delete next.assignee_type;
        delete next.assignee_id;
        if (value === "null") next.assignee_id = "null";
        else if (value) {
          const [type, id] = value.split(":");
          if (type && id) { next.assignee_type = type; next.assignee_id = id; }
        }
      } else if (value) next[key] = value;
      else delete next[key];
      return next;
    });
    resetPagination();
  }
  const filterValue = (key: string) => key === "assignee" ? (filters.assignee_id === "null" ? "null" : filters.assignee_id ? `${filters.assignee_type}:${filters.assignee_id}` : "") : filters[key] ?? "";
  const fields = [
    { key: "status", label: t(($) => $.iterations.status), options: (choices?.statuses ?? []).map((value) => ({ value, label: statusLabel(value) })) },
    { key: "project_id", label: t(($) => $.iterations.projectFilter), options: (choices?.projects ?? []).map((project) => ({ value: project.id ?? "null", label: project.id === null ? none : project.name ?? unknown })) },
    { key: "assignee", label: t(($) => $.iterations.assigneeFilter), options: (choices?.assignees ?? []).map((assignee) => ({ value: assignee.id ? `${assignee.type}:${assignee.id}` : "null", label: assignee.id === null ? none : assignee.name ?? unknown })) },
    { key: "priority", label: t(($) => $.iterations.priorityFilter), options: ["urgent", "high", "medium", "low", "none"].map((value) => ({ value, label: priorityLabel(value, issueT) })) },
    { key: "label_id", label: t(($) => $.iterations.labelFilter), options: (choices?.labels ?? []).map((label) => ({ value: label.id, label: label.name })) },
  ];
  const scopeOptions = [{ value: "current" as const, label: t(($) => $.iterations.scope) }, { value: "original" as const, label: t(($) => $.iterations.original) }];
  const groupOptions: { value: Grouping; label: string }[] = [
    { value: "none", label: t(($) => $.iterations.noGrouping) },
    { value: "status", label: t(($) => $.iterations.status) },
    { value: "assignee", label: t(($) => $.iterations.assigneeFilter) },
    { value: "project", label: t(($) => $.iterations.projectFilter) },
    { value: "priority", label: t(($) => $.iterations.priorityFilter) },
    { value: "label", label: t(($) => $.iterations.labelFilter) },
  ];
  function groupEntries(issue: HistoricalIssue): [string, string][] {
    switch (groupBy) {
      case "status": return [[issue.status_key, statusLabel(issue.status_key)]];
      case "assignee": return [[`${issue.assignee_type}:${issue.assignee_id}`, issue.assignee_name ?? none]];
      case "project": return [[issue.project_id ?? "none", issue.project_name ?? none]];
      case "priority": return [[issue.priority ?? "unknown", issue.priority == null ? unknown : priorityLabel(issue.priority, issueT)]];
      case "label": return issue.labels == null ? [["unknown", unknown]] : issue.labels.length ? issue.labels.map((label) => [label.id, label.name]) : [["none", none]];
      default: return [["", ""]];
    }
  }
  const groups = new Map<string, { label: string; items: HistoricalIssue[] }>();
  for (const issue of data?.items ?? []) {
    for (const [key, label] of new Map(groupEntries(issue))) {
      const group = groups.get(key) ?? { label, items: [] };
      group.items.push(issue);
      groups.set(key, group);
    }
  }
  if (emptyState && !result.error && data?.total === 0 && scope === "current" && !search && Object.keys(filters).length === 0) return <>{emptyState}</>;
  return (
    <section className="space-y-4">
      <h2 className="text-subtitle font-semibold">{t(($) => $.iterations.issues)}</h2>
      <p className="text-caption text-muted-foreground">{t(($) => $.iterations.overallScope)}</p>
      <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(10rem,auto)]">
        <div className="space-y-2">
          <Label htmlFor={`${controlId}-search`}>{t(($) => $.iterations.audit.searchTasks)}</Label>
          <Input id={`${controlId}-search`} placeholder={t(($) => $.iterations.audit.searchTasks)} className="pointer-coarse:min-h-11" value={search} onChange={(event) => { setSearch(event.target.value); resetPagination(); }} />
        </div>
        <TaskSelect id={`${controlId}-scope`} label={t(($) => $.iterations.scope)} value={scope} options={scopeOptions} onValueChange={(value) => { setScope(value); resetPagination(); }} />
      </div>
      <details className="rounded-lg border border-border p-3">
        <summary className={iterationDisclosureClass}>{t(($) => $.iterations.filterTasks)}</summary>
        <div className="mt-3 grid gap-3 sm:grid-cols-3">
          {fields.map((field) => <TaskSelect key={field.key} id={`${controlId}-${field.key}`} label={field.label} value={filterValue(field.key)} disabled={field.key !== "priority" && !choices} describedBy={field.key !== "priority" && !choices ? `${controlId}-unavailable` : undefined} onValueChange={(value) => changeFilter(field.key, value)} options={[
            { value: "", label: t(($) => $.iterations.allValues) },
            ...(filterValue(field.key) && !field.options.some(({ value }) => value === filterValue(field.key)) ? [{ value: filterValue(field.key), label: t(($) => $.iterations.missingSelection) }] : []),
            ...field.options,
          ]} />)}
          <TaskSelect id={`${controlId}-group`} label={t(($) => $.iterations.groupTasks)} value={groupBy} options={groupOptions} onValueChange={(value) => { setGroupBy(value); resetPagination(); }} />
        </div>
        {data && !choices && <p id={`${controlId}-unavailable`} className="mt-3 text-caption text-muted-foreground">{t(($) => $.iterations.audit.advancedFiltersUnavailable)}</p>}
      </details>
      {result.isPending && <div role="status" className="space-y-3"><span className="sr-only">{t(($) => $.iterations.loading)}</span><div aria-hidden>{Array.from({ length: 4 }, (_, index) => <div key={index} className="flex items-start gap-3 py-3.5"><Skeleton className="size-4 shrink-0" /><div className="w-full space-y-2"><Skeleton className="h-4 w-2/3" /><Skeleton className="h-3 w-1/2" /></div></div>)}</div></div>}
      {result.error && <div className="space-y-3"><IterationError error={result.error} context="read" /><Button variant="outline" className="pointer-coarse:min-h-11" disabled={result.isFetching} onClick={() => { if (cursor) resetPagination(); else void result.refetch(); }}>{t(($) => $.iterations.retry)}</Button></div>}
      {data && <p className="text-caption text-muted-foreground">{t(($) => $.iterations.filteredCount)}: {data.total}</p>}
      {Array.from(groups, ([key, { label, items }]) => <section key={key}>
        {groupBy !== "none" && <h3 className="font-semibold">{label} ({items.length})</h3>}
        <ul className="divide-y">{items.map((issue) => <li key={issue.issue_id} className="grid grid-cols-[1rem_minmax(0,1fr)] items-start gap-3 py-3.5">
          <span className="pt-0.5 text-muted-foreground" aria-hidden>{issue.status_category === "done" ? <CircleCheck className="size-4 text-chart-1" /> : issue.status_category === "cancelled" ? <CircleX className="size-4" /> : ["in_progress", "in_review"].includes(issue.status_category) ? <CirclePlay className="size-4" /> : <Circle className="size-4" />}</span>
          <div className="min-w-0">{historical ? <IterationCurrentComparison wsId={wsId} issue={issue} trigger={`${issue.identifier} · ${issue.title}`} /> : <AppLink href={paths.issueDetail(issue.issue_id)} className="[overflow-wrap:anywhere] hover:underline pointer-coarse:flex pointer-coarse:min-h-11 pointer-coarse:items-center">{issue.identifier} · {issue.title}</AppLink>}
          <p className="text-caption text-muted-foreground">{issue.project_name} · {issue.assignee_name} · {labels.category(issue.status_category)} · {t(($) => $.iterations.rollover)}: {issue.rollover_count}</p>
          {issue.rollover_count >= 3 && <p className="text-caption">{t(($) => $.iterations.rolloverReview)}</p>}
          </div>
        </li>)}</ul>
      </section>)}
      {!result.error && data?.total === 0 && <div className="space-y-3 py-8 text-center text-muted-foreground"><p>{t(($) => $.iterations.pages.noMatches)}</p><Button variant="outline" className="pointer-coarse:min-h-11" onClick={() => { setSearch(""); setFilters({}); setScope("current"); resetPagination(); }}>{t(($) => $.iterations.pages.clearFilters)}</Button></div>}
      {groupBy === "none" && (pageCursors.length > 1 || data && page.data?.next_cursor) && <nav aria-label={t(($) => $.iterations.issues)} className="flex flex-wrap gap-2">
        <Button variant="outline" className="pointer-coarse:min-h-11" disabled={pageCursors.length === 1 || result.isFetching} onClick={() => setPageCursors((previous) => previous.slice(0, -1))}>{t(($) => $.iterations.audit.previousPage)}</Button>
        <Button variant="outline" className="pointer-coarse:min-h-11" disabled={!data || !page.data?.next_cursor || result.isFetching || !!result.error} onClick={() => { const next = page.data?.next_cursor; if (next) setPageCursors((previous) => [...previous, next]); }}>{t(($) => $.iterations.next)}</Button>
      </nav>}
    </section>
  );
}
