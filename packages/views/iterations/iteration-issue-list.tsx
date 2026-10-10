"use client";

import { useId, useState, type ReactNode } from "react";
import { Circle, CircleAlert, CircleCheck, CirclePlay, CircleX, Filter, UserRound, X } from "lucide-react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { api } from "@multica/core/api";
import { iterationIssuesOptions, iterationGroupedIssuesOptions, hasIterationReadAccess, type IterationScopePhase } from "@multica/core/iterations";
import { useWorkspacePaths } from "@multica/core/paths";
import { Input } from "@multica/ui/components/ui/input";
import { Button } from "@multica/ui/components/ui/button";
import { Label } from "@multica/ui/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@multica/ui/components/ui/select";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Popover, PopoverContent, PopoverTitle, PopoverTrigger } from "@multica/ui/components/ui/popover";
import { AppLink } from "../navigation";
import { useT } from "../i18n";
import { useStatusLabel } from "../issues/utils/status-label";
import { priorityLabel } from "../issues/utils/priority-label";
import { IterationCurrentComparison } from "./iteration-current-comparison";
import { IterationError, isDefinitiveReadError } from "./iteration-error";
import { useIterationLabels } from "./labels";

type HistoricalIssue = Awaited<ReturnType<typeof api.getIterationIssues>>["items"][number];
type Grouping = "none" | "status" | "assignee" | "project" | "priority" | "label";
type Scope = "current" | "original";
type IssueListProps = { wsId: string; id: string; historical: boolean; phase: IterationScopePhase; emptyState?: ReactNode };

function TaskSelect<Value extends string>({ id, label, value, options, onValueChange, disabled, describedBy }: {
  id: string;
  label: string;
  value: Value;
  options: { value: Value; label: string }[];
  onValueChange: (value: Value) => void;
  disabled?: boolean;
  describedBy?: string;
}) {
  return <div className="min-w-0 space-y-1.5">
    <Label htmlFor={id} className="text-caption">{label}</Label>
    <Select items={options} value={value} onValueChange={(next) => { if (next !== null) onValueChange(next); }} disabled={disabled}>
      <SelectTrigger id={id} className="w-full min-w-0 pointer-coarse:min-h-11" aria-describedby={describedBy}><SelectValue /></SelectTrigger>
      <SelectContent alignItemWithTrigger={false}>{options.map((option) => <SelectItem key={option.value} value={option.value} className="pointer-coarse:min-h-11"><span className="min-w-0 whitespace-normal [overflow-wrap:anywhere]">{option.label}</span></SelectItem>)}</SelectContent>
    </Select>
  </div>;
}

export function IterationIssueList(props: IssueListProps) {
  return <IterationIssueListContent key={`${props.wsId}:${props.id}`} {...props} />;
}

function IterationIssueListContent({ wsId, id, historical, phase, emptyState }: IssueListProps) {
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
  const canCompareScopes = phase === "active" || phase === "completed" || phase === "cancelledAfterStart";
  if (!canCompareScopes && scope !== "current") {
    setScope("current");
    resetPagination();
  }
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
  const unassigned = issueT(($) => $.table.unassigned);
  function resetFilters() {
    setSearch("");
    setFilters({});
    setScope("current");
    resetPagination();
  }
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
  const appliedFilters = fields.flatMap((field) => {
    const value = filterValue(field.key);
    if (!value) return [];
    const valueLabel = field.options.find((option) => option.value === value)?.label ?? t(($) => $.iterations.missingSelection);
    return [{ key: field.key, label: `${field.label}: ${valueLabel}` }];
  });
  const hasRefinements = !!search || appliedFilters.length > 0 || scope === "original";
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
      case "assignee": return [[`${issue.assignee_type}:${issue.assignee_id}`, issue.assignee_id === null ? unassigned : issue.assignee_name ?? unknown]];
      case "project": return [[issue.project_id ?? "none", issue.project_id === null ? none : issue.project_name ?? unknown]];
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
  const showEmptyPlan = emptyState && !result.error && data?.total === 0 && !hasRefinements;
  const showReset = hasRefinements || (!showEmptyPlan && !result.error && data?.total === 0);
  return (
    <section className="@container space-y-3">
      <div className="flex flex-wrap items-end gap-3">
        <div className="w-72 max-w-full min-w-0 space-y-1.5">
          <Label htmlFor={`${controlId}-search`} className="text-caption">{t(($) => $.iterations.audit.searchTasks)}</Label>
          <Input id={`${controlId}-search`} placeholder={t(($) => $.iterations.audit.searchTasks)} className="pointer-coarse:min-h-11" value={search} onChange={(event) => { setSearch(event.target.value); resetPagination(); }} />
        </div>
        <Popover>
          <PopoverTrigger render={<Button variant="ghost" className="pointer-coarse:min-h-11" aria-label={t(($) => $.iterations.filterTasks)} />}>
            <Filter aria-hidden />{t(($) => $.iterations.filterTasks)}{appliedFilters.length > 0 && <span aria-hidden className="rounded bg-muted px-1.5 tabular-nums">{appliedFilters.length}</span>}
          </PopoverTrigger>
          <PopoverContent align="start" className="max-h-[min(32rem,var(--available-height))] w-80 max-w-[calc(100vw-2rem)] overflow-y-auto p-4">
            <PopoverTitle className="sr-only">{t(($) => $.iterations.filterTasks)}</PopoverTitle>
            <div className="space-y-3">
              {fields.map((field) => <TaskSelect key={field.key} id={`${controlId}-${field.key}`} label={field.label} value={filterValue(field.key)} disabled={field.key !== "priority" && !choices} describedBy={field.key !== "priority" && !choices ? `${controlId}-unavailable` : undefined} onValueChange={(value) => changeFilter(field.key, value)} options={[
                { value: "", label: t(($) => $.iterations.allValues) },
                ...(filterValue(field.key) && !field.options.some(({ value }) => value === filterValue(field.key)) ? [{ value: filterValue(field.key), label: t(($) => $.iterations.missingSelection) }] : []),
                ...field.options,
              ]} />)}
            </div>
            {data && !choices && <p id={`${controlId}-unavailable`} className="text-caption text-muted-foreground">{t(($) => $.iterations.audit.advancedFiltersUnavailable)}</p>}
          </PopoverContent>
        </Popover>
        <div className="w-44 max-w-full"><TaskSelect id={`${controlId}-group`} label={t(($) => $.iterations.groupTasks)} value={groupBy} options={groupOptions} onValueChange={(value) => { setGroupBy(value); resetPagination(); }} /></div>
        {canCompareScopes && <div className="w-48 max-w-full"><TaskSelect id={`${controlId}-scope`} label={t(($) => $.iterations.scope)} value={scope} options={scopeOptions} onValueChange={(value) => { setScope(value); resetPagination(); }} /></div>}
      </div>
      {showReset && <div className="flex flex-wrap items-center gap-2">
        {appliedFilters.length > 0 && <span className="text-caption text-muted-foreground">{issueT(($) => $.filters.active_count, { count: appliedFilters.length })}</span>}
        {appliedFilters.map((filter) => <Button key={filter.key} variant="secondary" size="sm" className="h-auto min-h-8 max-w-full whitespace-normal text-left pointer-coarse:min-h-11" aria-label={issueT(($) => $.filters.chip_remove, { name: filter.label })} onClick={() => changeFilter(filter.key, "")}><span className="min-w-0 [overflow-wrap:anywhere]">{filter.label}</span><X aria-hidden className="shrink-0" /></Button>)}
        <Button variant="ghost" size="sm" className="pointer-coarse:min-h-11" aria-describedby={`${controlId}-reset-description`} title={t(($) => $.iterations.taskList.resetHint)} onClick={resetFilters}>{issueT(($) => $.filters.reset)}</Button>
        <span id={`${controlId}-reset-description`} className="sr-only">{t(($) => $.iterations.taskList.resetHint)}</span>
      </div>}
      {data && hasRefinements && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.iterations.filteredCount)}: <span className="tabular-nums">{data.total}</span></p>}
      {result.isPending && <div role="status" className="space-y-3"><span className="sr-only">{t(($) => $.iterations.loading)}</span><div aria-hidden>{Array.from({ length: 4 }, (_, index) => <div key={index} className="flex items-start gap-3 py-3.5"><Skeleton className="size-4 shrink-0" /><div className="w-full space-y-2"><Skeleton className="h-4 w-2/3" /><Skeleton className="h-3 w-1/2" /></div></div>)}</div></div>}
      {result.error && <div className="space-y-3"><IterationError error={result.error} context="read" /><Button variant="outline" className="pointer-coarse:min-h-11" disabled={result.isFetching} onClick={() => { if (cursor) resetPagination(); else void result.refetch(); }}>{t(($) => $.iterations.retry)}</Button></div>}
      {Array.from(groups, ([key, { label, items }]) => <section key={key}>
        {groupBy !== "none" && <h3 className="font-semibold">{label} ({items.length})</h3>}
        <ul className="divide-y">{items.map((issue) => <li key={issue.issue_id} className="grid items-start gap-x-5 gap-y-2 py-3 @2xl:grid-cols-[minmax(0,1fr)_9rem_10rem]">
          <div className="min-w-0 space-y-1">{historical ? <IterationCurrentComparison wsId={wsId} issue={issue} trigger={issue.title} /> : <AppLink href={paths.issueDetail(issue.issue_id)} className="rounded text-body font-medium [overflow-wrap:anywhere] hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-foreground pointer-coarse:flex pointer-coarse:min-h-11 pointer-coarse:items-center">{issue.title}</AppLink>}
            <div className="flex flex-wrap gap-x-3 gap-y-1 text-caption text-muted-foreground [overflow-wrap:anywhere]">
              <span>{issue.identifier}</span>
              {issue.project_id !== null && <span>{issue.project_name ?? unknown}</span>}
              {issue.rollover_count > 0 && <span>{t(($) => $.iterations.rollover)}: {issue.rollover_count}</span>}
            </div>
            {issue.rollover_count >= 3 && <p className="text-caption text-warning-foreground">{t(($) => $.iterations.rolloverReview)}</p>}
          </div>
          <dl className="flex min-w-0 flex-wrap gap-x-5 gap-y-2 @2xl:contents">
            <div className={`min-w-0 @2xl:pt-0.5 ${issue.status_category === "blocked" ? "font-medium text-warning-foreground" : "text-muted-foreground"}`}>
              <dt className="sr-only">{t(($) => $.iterations.status)}</dt>
              <dd className="flex items-start gap-1.5 text-caption [overflow-wrap:anywhere]"><span aria-hidden className="shrink-0">{issue.status_category === "blocked" ? <CircleAlert className="size-4" /> : issue.status_category === "done" ? <CircleCheck className="size-4 text-info-foreground" /> : issue.status_category === "cancelled" ? <CircleX className="size-4" /> : ["in_progress", "in_review"].includes(issue.status_category) ? <CirclePlay className="size-4" /> : <Circle className="size-4" />}</span>{labels.category(issue.status_category)}</dd>
            </div>
            <div className="min-w-0 text-muted-foreground @2xl:pt-0.5"><dt className="sr-only">{t(($) => $.iterations.assignee)}</dt><dd className="flex items-start gap-1.5 text-caption [overflow-wrap:anywhere]"><UserRound aria-hidden className="size-4 shrink-0" /><span className="min-w-0">{issue.assignee_id === null ? unassigned : issue.assignee_name ?? unknown}</span></dd></div>
          </dl>
        </li>)}</ul>
      </section>)}
      {showEmptyPlan ? emptyState : !result.error && data?.total === 0 && <p className="py-8 text-center text-muted-foreground">{t(($) => $.iterations.pages.noMatches)}</p>}
      {groupBy === "none" && (pageCursors.length > 1 || data && page.data?.next_cursor) && <nav aria-label={t(($) => $.iterations.issues)} className="flex flex-wrap gap-2">
        <Button variant="outline" className="pointer-coarse:min-h-11" disabled={pageCursors.length === 1 || result.isFetching} onClick={() => setPageCursors((previous) => previous.slice(0, -1))}>{t(($) => $.iterations.audit.previousPage)}</Button>
        <Button variant="outline" className="pointer-coarse:min-h-11" disabled={!data || !page.data?.next_cursor || result.isFetching || !!result.error} onClick={() => { const next = page.data?.next_cursor; if (next) setPageCursors((previous) => [...previous, next]); }}>{t(($) => $.iterations.next)}</Button>
      </nav>}
    </section>
  );
}
