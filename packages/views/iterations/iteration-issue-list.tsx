"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import type { api } from "@multica/core/api";
import { iterationIssuesOptions, iterationGroupedIssuesOptions } from "@multica/core/iterations";
import { useWorkspacePaths } from "@multica/core/paths";
import { Input } from "@multica/ui/components/ui/input";
import { Button } from "@multica/ui/components/ui/button";
import { AppLink } from "../navigation";
import { useT } from "../i18n";
import { useStatusLabel } from "../issues/utils/status-label";
import { priorityLabel } from "../issues/utils/priority-label";
import { IterationCurrentComparison } from "./iteration-current-comparison";
import { IterationError } from "./iteration-error";
import { useIterationLabels } from "./labels";

type HistoricalIssue = Awaited<ReturnType<typeof api.getIterationIssues>>["items"][number];
type Grouping = "none" | "status" | "assignee" | "project" | "priority" | "label";

export function IterationIssueList({ wsId, id, historical }: { wsId: string; id: string; historical: boolean }) {
  const { t } = useT("projects");
  const { t: issueT } = useT("issues");
  const labels = useIterationLabels();
  const statusLabel = useStatusLabel(wsId);
  const paths = useWorkspacePaths();
  const [cursor, setCursor] = useState("");
  const [scope, setScope] = useState("current");
  const [search, setSearch] = useState("");
  const [filters, setFilters] = useState<Record<string, string>>({});
  const [filtersOpen, setFiltersOpen] = useState(false);
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
  const choices = useQuery({
    ...iterationGroupedIssuesOptions(wsId, id, scope === "original" ? { scope } : {}),
    enabled: filtersOpen,
  });
  const result = groupBy === "none" ? page : grouped;
  const all = choices.data?.items ?? [];
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
    setCursor("");
  }
  const filterValue = (key: string) => key === "assignee" ? (filters.assignee_id === "null" ? "null" : filters.assignee_id ? `${filters.assignee_type}:${filters.assignee_id}` : "") : filters[key] ?? "";
  const fields = [
    { key: "status", label: t(($) => $.iterations.status), options: Array.from(new Map(all.map((issue) => [issue.status_key, statusLabel(issue.status_key)]))) },
    { key: "project_id", label: t(($) => $.iterations.projectFilter), options: Array.from(new Map(all.map((issue) => [issue.project_id ?? "null", issue.project_name ?? none]))), },
    { key: "assignee", label: t(($) => $.iterations.assigneeFilter), options: Array.from(new Map(all.map((issue) => [issue.assignee_id ? `${issue.assignee_type}:${issue.assignee_id}` : "null", issue.assignee_name ?? none]))), },
    { key: "priority", label: t(($) => $.iterations.priorityFilter), options: ["urgent", "high", "medium", "low", "none"].map((value) => [value, priorityLabel(value, issueT)]) },
    { key: "label_id", label: t(($) => $.iterations.labelFilter), options: Array.from(new Map(all.flatMap((issue) => (issue.labels ?? []).map((label) => [label.id, label.name] as const)))), },
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
  for (const issue of result.data?.items ?? []) {
    for (const [key, label] of new Map(groupEntries(issue))) {
      const group = groups.get(key) ?? { label, items: [] };
      group.items.push(issue);
      groups.set(key, group);
    }
  }
  return (
    <section className="space-y-3">
      <h2 className="text-subtitle font-semibold">{t(($) => $.iterations.issues)}</h2>
      <p className="text-caption text-muted-foreground">{t(($) => $.iterations.overallScope)}</p>
      <div className="flex flex-wrap gap-3">
        <Input aria-label={t(($) => $.iterations.selectTasks)} value={search} onChange={(event) => { setSearch(event.target.value); setCursor(""); }} />
        <select aria-label={t(($) => $.iterations.scope)} value={scope} onChange={(event) => { setScope(event.target.value); setCursor(""); }} className="rounded-md border bg-background p-2">
          <option value="current">{t(($) => $.iterations.scope)}</option>
          <option value="original">{t(($) => $.iterations.original)}</option>
        </select>
      </div>
      <details onToggle={(event) => setFiltersOpen(event.currentTarget.open)}>
        <summary>{t(($) => $.iterations.filterTasks)}</summary>
        <div className="mt-3 grid gap-3 sm:grid-cols-3">
          {fields.map((field) => <label key={field.key}>{field.label}
            <select className="block w-full rounded-md border bg-background p-2" value={filterValue(field.key)} onChange={(event) => changeFilter(field.key, event.target.value)}>
              <option value="">{t(($) => $.iterations.allValues)}</option>
              {filterValue(field.key) && !field.options.some(([value]) => value === filterValue(field.key)) && <option value={filterValue(field.key)}>{t(($) => $.iterations.missingSelection)}</option>}
              {field.options.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
            </select>
          </label>)}
          <label>{t(($) => $.iterations.groupTasks)}
            <select className="block w-full rounded-md border bg-background p-2" value={groupBy} onChange={(event) => { setGroupBy(event.target.value as Grouping); setCursor(""); }}>
              <option value="none">{t(($) => $.iterations.noGrouping)}</option>
              {[["status", t(($) => $.iterations.status)], ["assignee", t(($) => $.iterations.assigneeFilter)], ["project", t(($) => $.iterations.projectFilter)], ["priority", t(($) => $.iterations.priorityFilter)], ["label", t(($) => $.iterations.labelFilter)]].map(([value, label]) => <option key={value} value={value}>{label}</option>)}
            </select>
          </label>
        </div>
        {choices.error && <IterationError error={choices.error} />}
      </details>
      {result.isPending && <p role="status">{t(($) => $.iterations.loading)}</p>}
      {result.error && <div><IterationError error={result.error} /><Button onClick={() => { if (cursor) setCursor(""); else void result.refetch(); }}>{t(($) => $.iterations.retry)}</Button></div>}
      {!result.error && result.data && <p>{t(($) => $.iterations.filteredCount)}: {result.data.total}</p>}
      {!result.error && Array.from(groups, ([key, { label, items }]) => <section key={key}>
        {groupBy !== "none" && <h3 className="font-semibold">{label} ({items.length})</h3>}
        <ul className="divide-y">{items.map((issue) => <li key={issue.issue_id} className="py-3">
          {historical ? <IterationCurrentComparison wsId={wsId} issue={issue} trigger={`${issue.identifier} · ${issue.title}`} /> : <AppLink href={paths.issueDetail(issue.issue_id)} className="break-words hover:underline">{issue.identifier} · {issue.title}</AppLink>}
          <p className="text-caption text-muted-foreground">{issue.project_name} · {issue.assignee_name} · {labels.category(issue.status_category)} · {t(($) => $.iterations.rollover)}: {issue.rollover_count}</p>
          {issue.rollover_count >= 3 && <p className="text-caption">{t(($) => $.iterations.rolloverReview)}</p>}
        </li>)}</ul>
      </section>)}
      {groupBy === "none" && page.data?.next_cursor && <Button onClick={() => setCursor(page.data!.next_cursor!)}>{t(($) => $.iterations.next)}</Button>}
    </section>
  );
}
