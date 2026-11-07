"use client";

import { useId, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ArrowRight, CalendarDays, ChevronRight } from "lucide-react";
import {
  groupIterationActivity,
  iterationActivityDays,
  type IterationEvent,
  type IterationActivityChange,
  type IterationActivityEntry,
  type IterationActivityGroup,
  type IterationActivityIdentity,
  type IterationActivityKind,
  type IterationActivityStatus,
  type IterationScopeImpact,
} from "@multica/core/iterations";
import { useWorkspacePaths } from "@multica/core/paths";
import { isIssueStatusCategory } from "@multica/core/issue-statuses";
import { memberListOptions, agentListOptions, squadListOptions } from "@multica/core/workspace/queries";
import { Button } from "@multica/ui/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@multica/ui/components/ui/table";
import { cn } from "@multica/ui/lib/utils";
import { useT, useLocale } from "../i18n";
import { formatInTimeZone } from "../common/format-in-time-zone";
import { useStatusCategoryLabel } from "../issues/utils/status-label";
import { IterationReference, useIterationCatalogue } from "./iteration-catalogue";
import { iterationDisclosureClass } from "./iteration-presentation";
import { AppLink } from "../navigation";

function useEventPresentation(wsId: string, groups: readonly IterationActivityGroup[]) {
  const { t } = useT("projects");
  const locale = useLocale();
  const categoryLabel = useStatusCategoryLabel(t(($) => $.iterations.unknownTaskStatus));
  const identities = groups.flatMap((group) => group.entries.flatMap((entry) => [
    entry.actor, entry.before.assignee, entry.after.assignee, entry.before.coordinator, entry.after.coordinator,
  ]));
  const needs = (type: string) => identities.some((identity) => identity?.type === type && identity.id && !identity.name);
  const members = useQuery({ ...memberListOptions(wsId), enabled: needs("member") });
  const agents = useQuery({ ...agentListOptions(wsId), enabled: needs("agent") });
  const squads = useQuery({ ...squadListOptions(wsId), enabled: needs("squad") });
  const names = useMemo(() => ({
    member: new Map(members.data?.map((member) => [member.user_id, member.name])),
    agent: new Map(agents.data?.map((agent) => [agent.id, agent.name])),
    squad: new Map(squads.data?.map((squad) => [squad.id, squad.name])),
  }), [members.data, agents.data, squads.data]);
  const unknown = t(($) => $.iterations.unknownHistory);
  const identityLabel = (identity: IterationActivityIdentity | undefined, actor = false): string => {
    if (!identity) return unknown;
    if (identity.type === "system") return t(($) => $.iterations.activityPanel.system);
    if (!actor && identity.id === null) return t(($) => $.iterations.activityPanel.unassigned);
    if (identity.name) return identity.name;
    const type = identity.type;
    if (type === "member" || type === "agent" || type === "squad")
      return (identity.id && names[type].get(identity.id)) || t(($) => $.iterations.activityPanel[type]);
    if (type === "plugin") return t(($) => $.iterations.activityPanel.plugin);
    if (type === "project") return t(($) => $.iterations.project);
    return actor ? t(($) => $.iterations.activityPanel.unknownActor) : unknown;
  };
  const statusLabel = (status: IterationActivityStatus | undefined): string => {
    if (!status) return unknown;
    const category = status.category;
    const key = status.key;
    if (category) {
      const label = isIssueStatusCategory(category) ? categoryLabel(category) : category;
      return key && key !== category ? `${key} (${label})` : label;
    }
    return key ? (isIssueStatusCategory(key) ? categoryLabel(key) : key) : unknown;
  };
  const eventName = (kind: IterationActivityKind) => {
    switch (kind) {
      case "join": return t(($) => $.iterations.eventJoin);
      case "leave": return t(($) => $.iterations.eventLeave);
      case "reenter": return t(($) => $.iterations.eventReenter);
      case "baseline": return t(($) => $.iterations.eventBaseline);
      case "status": return t(($) => $.iterations.eventStatus);
      case "delete": return t(($) => $.iterations.eventDelete);
      case "cancelIssue": return t(($) => $.iterations.eventCancel);
      case "reopen": return t(($) => $.iterations.eventReopen);
      case "issueChanged": return t(($) => $.iterations.eventChanged);
      case "executionStarted": return t(($) => $.iterations.eventExecution);
      case "create": return t(($) => $.iterations.eventCreate);
      case "edit": return t(($) => $.iterations.eventEdit);
      case "dateEdit": return t(($) => $.iterations.eventDates);
      case "start": return t(($) => $.iterations.eventStart);
      case "end": return t(($) => $.iterations.eventEnd);
      case "cancelIteration": return t(($) => $.iterations.eventCancelled);
      case "rollover": return t(($) => $.iterations.eventRollover);
      case "plan": return t(($) => $.iterations.eventPlan);
      default: return t(($) => $.iterations.eventUnknown);
    }
  };
  const fieldLabel = (field: IterationActivityChange["field"]) => {
    switch (field) {
      case "title": return t(($) => $.iterations.activityPanel.title);
      case "timezone": return t(($) => $.iterations.activityPanel.timezone);
      default: return t(($) => $.iterations[field]);
    }
  };
  const changeValue = (change: IterationActivityChange, side: "before" | "after") => {
    switch (change.field) {
      case "status": return statusLabel(change[side]);
      case "assignee": case "coordinator": case "project": return identityLabel(change[side]);
      default: {
        const value = change[side];
        if (value === undefined) return unknown;
        if (value === null || value === "") return t(($) => $.iterations.activityPanel.cleared);
        if (change.field === "startDate" || change.field === "endDate")
          return formatInTimeZone(value, "UTC", locale, { year: "numeric", hour: undefined, minute: undefined });
        return value;
      }
    }
  };
  return { identityLabel, eventName, fieldLabel, changeValue };
}
type Presentation = ReturnType<typeof useEventPresentation>;
type Catalogue = { id: string; name: string }[];

function eventImpactLabel(impact: IterationScopeImpact | undefined, t: ReturnType<typeof useT<"projects">>["t"]) {
  return impact?.kind === "effective" ? impact.delta === 0 ? t(($) => $.iterations.activityPanel.scopeUnchanged) : t(($) => $.iterations.activityPanel.effectiveImpact, { delta: `${impact.delta > 0 ? "+" : ""}${impact.delta}` })
    : impact?.kind === "planning" ? t(($) => $.iterations.activityPanel.planningImpact)
      : impact?.kind === "baseline" ? t(($) => $.iterations.activityPanel.baselineImpact)
        : impact?.kind === "unknown" ? t(($) => $.iterations.activityPanel.unknownImpact) : null;
}

function EventMovement({ entry, catalogue }: { entry: IterationActivityEntry; catalogue: Catalogue }) {
  const { t } = useT("projects");
  if (!entry.movement) return null;
  return <p className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
    <span><span className="sr-only">{t(($) => $.iterations.source)}: </span><IterationReference id={entry.movement.source} catalogue={catalogue} /></span>
    <ArrowRight className="size-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
    <span><span className="sr-only">{t(($) => $.iterations.destination)}: </span><IterationReference id={entry.movement.target} catalogue={catalogue} /></span>
  </p>;
}

function EventChanges({ changes, presentation }: { changes: readonly IterationActivityChange[]; presentation: Presentation }) {
  const { t } = useT("projects");
  if (!changes.length) return null;
  return <dl className="space-y-1">
    {changes.map((change) => <div key={change.field} className="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-1">
      <dt className="text-muted-foreground">{presentation.fieldLabel(change.field)}</dt>
      <dd className="flex min-w-0 max-w-full flex-wrap items-center gap-x-2 gap-y-1">
        <span className="min-w-0 whitespace-pre-wrap"><span className="sr-only">{t(($) => $.iterations.activityPanel.before)}: </span>{presentation.changeValue(change, "before")}</span>
        <ArrowRight className="size-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
        <span className="min-w-0 whitespace-pre-wrap font-medium"><span className="sr-only">{t(($) => $.iterations.activityPanel.after)}: </span>{presentation.changeValue(change, "after")}</span>
      </dd>
    </div>)}
  </dl>;
}

function EventSummary({ entry, presentation, catalogue, impact }: {
  entry: IterationActivityEntry;
  presentation: Presentation;
  catalogue: Catalogue;
  impact?: IterationScopeImpact;
}) {
  const { t } = useT("projects");
  const paths = useWorkspacePaths();
  const impactLabel = eventImpactLabel(impact, t);
  const restored = impact?.kind === "effective" && impact.delta === 1 && entry.before.status?.category === "cancelled";
  const action = restored ? t(($) => $.iterations.activityPanel.restoredTask) : presentation.eventName(entry.kind);
  return <div className="min-w-0 space-y-2 [overflow-wrap:anywhere]">
    <div className="flex min-w-0 flex-wrap items-baseline gap-x-4 gap-y-1">
      {entry.issue && <p className="min-w-0 font-medium">
        {entry.issue.identifier && <span className="mr-2 text-caption text-muted-foreground">{entry.issue.identifier}</span>}
        {entry.issue.title ?? (entry.issue.identifier ? null : t(($) => $.iterations.activityPanel.unnamedTask))}
      </p>}
      {impactLabel && <span className="text-caption text-muted-foreground tabular-nums">{impactLabel}</span>}
    </div>
    <div className="flex flex-wrap items-center gap-x-4 gap-y-1">
      <p className={entry.issue ? "text-caption text-muted-foreground" : "font-medium"}>{action}</p>
      {entry.issue && <AppLink href={paths.issueDetail(entry.issue.id)} aria-label={t(($) => $.iterations.activityPanel.currentTaskLink, { task: entry.issue.identifier || entry.issue.title || t(($) => $.iterations.activityPanel.unnamedTask) })} className="inline-flex min-h-7 items-center rounded text-caption text-muted-foreground underline underline-offset-4 hover:text-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-foreground pointer-coarse:min-h-11">{t(($) => $.iterations.openCurrent)}</AppLink>}
    </div>
    <div className="space-y-1.5 text-caption">
      <EventMovement entry={entry} catalogue={catalogue} />
      <EventChanges changes={entry.changes} presentation={presentation} />
      {entry.event.reason && <p className="whitespace-pre-wrap">{entry.event.reason}</p>}
    </div>
  </div>;
}

function EventMeta({ at, actor, timezone }: { at: string; actor: string; timezone: string }) {
  const locale = useLocale();
  return <p className="flex flex-wrap items-baseline gap-x-2 gap-y-1 text-caption text-muted-foreground [overflow-wrap:anywhere]">
    <span>{actor}</span><span aria-hidden="true">·</span>
    <time dateTime={at} title={timezone} className="tabular-nums">{formatInTimeZone(at, timezone, locale, { month: undefined, day: undefined })}</time>
  </p>;
}

function EventAudit({ entry, timezone }: { entry: IterationActivityEntry; timezone: string }) {
  const { t } = useT("projects");
  const locale = useLocale();
  const event = entry.event;
  const raw = (value: unknown) => value === undefined ? t(($) => $.iterations.unknownHistory) : JSON.stringify(value, null, 2);
  return <details>
    <summary className={iterationDisclosureClass}>{t(($) => $.iterations.activityPanel.eventDetails)}</summary>
    <div className="mt-2 space-y-4">
      <dl className="grid min-w-0 gap-x-6 gap-y-3 text-caption sm:grid-cols-2">
        {[
          [t(($) => $.iterations.activityPanel.eventId), event.id],
          [t(($) => $.iterations.activityPanel.operationId), event.operation_id],
          [t(($) => $.iterations.activityPanel.sequence), event.sequence],
          [t(($) => $.iterations.activityPanel.kind), event.kind],
          ...(event.issue_id ? [[t(($) => $.iterations.issueId), event.issue_id]] : []),
        ].map(([label, value]) => <div key={label} className="min-w-0 space-y-1">
          <dt className="text-muted-foreground">{label}</dt><dd className="select-text [overflow-wrap:anywhere]">{value}</dd>
        </div>)}
        {[
          [t(($) => $.iterations.activityPanel.occurredAt), event.occurred_at],
          [t(($) => $.iterations.activityPanel.sampledAt), event.sampled_at],
        ].map(([label, value]) => <div key={label} className="space-y-1">
          <dt className="text-muted-foreground">{label}</dt><dd><time dateTime={value} title={timezone}>{formatInTimeZone(value!, timezone, locale, { year: "numeric" })}</time></dd>
        </div>)}
      </dl>
      <p className="text-caption text-muted-foreground">{t(($) => $.iterations.currentNames)}</p>
      <details>
        <summary className={iterationDisclosureClass}>{t(($) => $.iterations.audit.rawFacts)}</summary>
        <dl className="mt-3 min-w-0 space-y-4 text-caption">
        {[
          [t(($) => $.iterations.actorLabel), raw(event.actor)],
          [t(($) => $.iterations.activityPanel.beforeFacts), raw(event.before_facts)],
          [t(($) => $.iterations.activityPanel.afterFacts), raw(event.after_facts)],
        ].map(([label, value]) => <div key={label} className="min-w-0 space-y-1">
          <dt className="text-muted-foreground">{label}</dt><dd><pre className="min-w-0 select-text whitespace-pre-wrap rounded-md bg-muted p-3 font-mono [overflow-wrap:anywhere]">{value}</pre></dd>
        </div>)}
        </dl>
      </details>
    </div>
  </details>;
}

function EventDetails({ entry, presentation, catalogue, impact, timezone }: {
  entry: IterationActivityEntry;
  presentation: Presentation;
  catalogue: Catalogue;
  impact?: IterationScopeImpact;
  timezone: string;
}) {
  const { t } = useT("projects");
  const locale = useLocale();
  const impactLabel = eventImpactLabel(impact, t);
  return <div className="min-w-0 max-w-3xl space-y-3 text-label [overflow-wrap:anywhere]">
    <dl className="grid min-w-0 gap-x-6 gap-y-2 sm:grid-cols-2">
      <div className="min-w-0 space-y-1">
        <dt className="text-muted-foreground">{t(($) => $.iterations.activityPanel.ledgerChange)}</dt>
        <dd>{presentation.eventName(entry.kind)}</dd>
      </div>
      {impactLabel && <div className="min-w-0 space-y-1">
        <dt className="text-muted-foreground">{t(($) => $.iterations.activityPanel.ledgerImpact)}</dt>
        <dd>{impactLabel}</dd>
      </div>}
      <div className="min-w-0 space-y-1 sm:col-span-2">
        <dt className="text-muted-foreground">{t(($) => $.iterations.activityPanel.occurredAt)}</dt>
        <dd><time dateTime={entry.event.occurred_at}>{formatInTimeZone(entry.event.occurred_at, timezone, locale, { year: "numeric" })}</time><span className="ml-2 text-muted-foreground">{timezone}</span></dd>
      </div>
    </dl>
    <EventMovement entry={entry} catalogue={catalogue} />
    <EventChanges changes={entry.changes} presentation={presentation} />
    {entry.event.reason && <p className="max-w-prose whitespace-pre-wrap">{entry.event.reason}</p>}
    <EventAudit entry={entry} timezone={timezone} />
  </div>;
}

function scopeChangeAction(entry: IterationActivityEntry, impact: IterationScopeImpact | undefined, presentation: Presentation, t: ReturnType<typeof useT<"projects">>["t"]): string {
  const before = entry.before.status?.category;
  const after = entry.after.status?.category;
  if (typeof before === "string" && typeof after === "string" && isIssueStatusCategory(before) && isIssueStatusCategory(after)) {
    if (before !== "cancelled" && after === "cancelled") return t(($) => $.iterations.activityPanel.scopeCancelled);
    if (before === "cancelled" && after !== "cancelled") return t(($) => $.iterations.activityPanel.scopeRestored);
  }
  const planning = impact?.kind === "planning";
  switch (entry.kind) {
    case "join": return t(($) => planning ? $.iterations.activityPanel.planningAdded : $.iterations.activityPanel.scopeAdded);
    case "leave": case "delete": return t(($) => planning ? $.iterations.activityPanel.planningRemoved : $.iterations.activityPanel.scopeRemoved);
    case "reenter": return t(($) => $.iterations.activityPanel.scopeReentered);
    case "cancelIssue": return t(($) => $.iterations.activityPanel.scopeCancelled);
    case "reopen": return t(($) => $.iterations.activityPanel.scopeRestored);
    case "baseline": return t(($) => $.iterations.activityPanel.scopeBaseline);
    case "status": case "issueChanged": case "executionStarted": return t(($) => $.iterations.activityPanel.scopeChanged);
    case "plan": return t(($) => $.iterations.eventPlan);
    case "create": case "edit": case "dateEdit": case "start": case "end": case "cancelIteration": case "rollover": case "unknown":
      return presentation.eventName(entry.kind);
  }
}

function scopeChangeImpact(impacts: (IterationScopeImpact | undefined)[], t: ReturnType<typeof useT<"projects">>["t"]): string[] {
  // A partial numeric total must not masquerade as the impact of the operation.
  if (impacts.some((impact) => !impact || impact.kind === "unknown")) return [t(($) => $.iterations.activityPanel.unknownImpact)];
  const effective = impacts.filter((impact) => impact?.kind === "effective");
  const delta = effective.reduce((sum, impact) => sum + impact.delta, 0);
  const labels: string[] = [];
  if (effective.length) labels.push(delta === 0 ? t(($) => $.iterations.activityPanel.scopeUnchanged)
    : t(($) => $.iterations.activityPanel.effectiveImpact, { delta: `${delta > 0 ? "+" : ""}${delta}` }));
  if (impacts.some((impact) => impact?.kind === "planning")) labels.push(t(($) => $.iterations.activityPanel.planningImpact));
  if (impacts.some((impact) => impact?.kind === "baseline")) labels.push(t(($) => $.iterations.activityPanel.baselineImpact));
  return labels.length ? labels : [t(($) => $.iterations.activityPanel.lifecycleImpact)];
}

export function IterationScopeChangeLedger({ wsId, groups, timezone, impacts, matchingEventIds }: {
  wsId: string;
  groups: IterationActivityGroup[];
  timezone: string;
  impacts?: ReadonlyMap<string, IterationScopeImpact>;
  matchingEventIds?: ReadonlySet<string>;
}) {
  const { t } = useT("projects");
  const locale = useLocale();
  const paths = useWorkspacePaths();
  const presentation = useEventPresentation(wsId, groups);
  const hasReferences = groups.some((group) => group.entries.some((entry) => entry.movement && (entry.movement.source || entry.movement.target)));
  const catalogue = useIterationCatalogue(wsId, hasReferences);
  const formatDateTime = (value: string) => formatInTimeZone(value, timezone, locale, { year: "numeric", month: "numeric", day: "numeric", hour: "numeric", minute: "2-digit" });
  const cellClass = "block min-w-0 whitespace-normal p-0 align-top [overflow-wrap:anywhere] @3xl:table-cell @3xl:px-4 @3xl:pt-3";
  return <div className="@container min-w-0 rounded-md border border-border/70">
    <Table role="table" aria-label={t(($) => $.iterations.activityPanel.scopeFilter)} className="block table-fixed @3xl:table">
      <TableHeader role="rowgroup" className="sr-only @3xl:not-sr-only @3xl:table-header-group">
        <TableRow role="row" className="bg-muted/30 hover:bg-muted/30">
          <TableHead scope="col" className="w-[18%] px-4 text-caption text-muted-foreground">{t(($) => $.iterations.activityPanel.ledgerChange)}</TableHead>
          <TableHead scope="col" className="w-[34%] px-4 text-caption text-muted-foreground">{t(($) => $.iterations.activityPanel.ledgerTask)}</TableHead>
          <TableHead scope="col" className="w-[17%] px-4 text-caption text-muted-foreground">{t(($) => $.iterations.activityPanel.ledgerImpact)}</TableHead>
          <TableHead scope="col" className="w-[13%] px-4 text-caption text-muted-foreground">{t(($) => $.iterations.activityPanel.ledgerActor)}</TableHead>
          <TableHead scope="col" className="w-[18%] px-4 text-caption text-muted-foreground">{t(($) => $.iterations.activityPanel.ledgerTime)}</TableHead>
        </TableRow>
      </TableHeader>
        {groups.map((group) => {
          const entries = matchingEventIds ? group.entries.filter((entry) => matchingEventIds.has(entry.event.id)) : group.entries;
          const issueEntries = [...new Map([...entries].reverse().filter((entry) => entry.issue).map((entry) => [entry.issue!.id, entry])).values()];
          const actor = new Intl.ListFormat(locale).format([...new Set(entries.map((entry) => presentation.identityLabel(entry.actor, true)))]);
          const actions = ["start", "end", "cancelIteration"].includes(group.kind) ? [presentation.eventName(group.kind)]
            : [...new Set(entries.map((entry) => scopeChangeAction(entry, impacts?.get(entry.event.id), presentation, t)))];
          const impactLabels = scopeChangeImpact(entries.map((entry) => impacts?.get(entry.event.id)), t);
          return <TableBody key={group.id} role="rowgroup" className="block border-b border-border/70 last:border-0 @3xl:table-row-group">
            <TableRow role="row" className="grid grid-cols-2 gap-x-4 gap-y-2 border-0 px-4 pt-3 hover:bg-transparent @3xl:table-row">
            <TableCell role="cell" className={`${cellClass} order-1 space-y-1`}><p className="font-medium">{new Intl.ListFormat(locale).format(actions)}</p>{group.entries.length > 1 && <p className="text-caption text-muted-foreground">{t(($) => matchingEventIds ? $.iterations.activityPanel.matchingRecords : $.iterations.activityPanel.ledgerRecords, { count: entries.length })}</p>}</TableCell>
            <TableCell role="cell" className={`${cellClass} order-3 col-span-2 space-y-1`}>
              {issueEntries.length === 1 && issueEntries[0]!.issue ? <AppLink href={paths.issueDetail(issueEntries[0]!.issue.id)} className="inline-flex max-w-full flex-wrap items-baseline gap-x-2 rounded font-medium hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-foreground pointer-coarse:min-h-11"><span className="text-caption text-muted-foreground">{issueEntries[0]!.issue.identifier}</span><span>{issueEntries[0]!.issue.title ?? t(($) => $.iterations.activityPanel.unnamedTask)}</span></AppLink>
                : issueEntries.length > 1 ? <p className="font-medium">{t(($) => $.iterations.activityPanel.taskCount, { count: issueEntries.length })}</p>
                  : <p className="text-muted-foreground">{t(($) => $.iterations.activityPanel.noTask)}</p>}
            </TableCell>
            <TableCell role="cell" className={`${cellClass} order-2 space-y-1 text-right text-caption font-medium tabular-nums @3xl:text-left`}>{impactLabels.map((label) => <p key={label}>{label}</p>)}</TableCell>
            <TableCell role="cell" className={`${cellClass} order-4 text-caption text-muted-foreground`}>{actor}</TableCell>
            <TableCell role="cell" className={`${cellClass} order-5 text-right text-caption tabular-nums text-muted-foreground @3xl:text-left`}><time dateTime={group.occurredAt} title={timezone}>{formatDateTime(group.occurredAt)}</time></TableCell>
            </TableRow>
            <TableRow role="row" className="block border-0 hover:bg-transparent @3xl:table-row"><TableCell role="cell" colSpan={5} className="block whitespace-normal px-4 pb-2 pt-0 @3xl:table-cell">
              <details>
                <summary className={iterationDisclosureClass}>{t(($) => matchingEventIds && entries.length < group.entries.length ? $.iterations.activityPanel.fullOperation : $.iterations.activityPanel.ledgerDetails, { count: group.entries.length })}</summary>
                <ol className="mt-3 space-y-4 border-l border-border/70 pl-4">
                  {group.entries.map((entry) => <li key={entry.event.id} className="min-w-0 space-y-1.5"><EventSummary entry={entry} presentation={presentation} catalogue={catalogue.data ?? []} impact={impacts?.get(entry.event.id)} /><EventMeta at={entry.event.occurred_at} actor={presentation.identityLabel(entry.actor, true)} timezone={timezone} /><EventAudit entry={entry} timezone={timezone} /></li>)}
                </ol>
              </details>
            </TableCell></TableRow>
          </TableBody>;
        })}
    </Table>
  </div>;
}

function ActivityTaskLink({ entry }: { entry: IterationActivityEntry }) {
  const { t } = useT("projects");
  const paths = useWorkspacePaths();
  if (!entry.issue) return null;
  const label = [entry.issue.identifier, entry.issue.title ?? t(($) => $.iterations.activityPanel.unnamedTask)].filter(Boolean).join(" ");
  return <AppLink href={paths.issueDetail(entry.issue.id)} aria-label={t(($) => $.iterations.activityPanel.currentTaskLink, { task: label })} className="inline-flex max-w-full flex-wrap items-baseline gap-x-2 rounded font-medium underline decoration-transparent underline-offset-4 hover:decoration-current focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-foreground pointer-coarse:min-h-11 pointer-coarse:min-w-11 pointer-coarse:items-center">
    {entry.issue.identifier && <span className="text-label font-normal text-muted-foreground">{entry.issue.identifier}</span>}
    <span className="min-w-0">{entry.issue.title ?? t(($) => $.iterations.activityPanel.unnamedTask)}</span>
  </AppLink>;
}

function ActivityChange({ entry, presentation, catalogue, impact }: {
  entry: IterationActivityEntry;
  presentation: Presentation;
  catalogue: Catalogue;
  impact?: IterationScopeImpact;
}) {
  const { t } = useT("projects");
  const changesDescribeAction = entry.changes.length > 0 && !entry.scopeRelated && ["plan", "status", "issueChanged"].includes(entry.kind);
  const action = impact?.kind === "planning" && entry.scopeRelated ? scopeChangeAction(entry, impact, presentation, t)
    : impact?.kind === "effective" && impact.delta === 1 && entry.before.status?.category === "cancelled" ? t(($) => $.iterations.activityPanel.restoredTask)
      : presentation.eventName(entry.kind);
  const showImpact = impact?.kind === "unknown" || (impact?.kind === "effective" && (impact.delta !== 0 || !changesDescribeAction)) || impact?.kind === "baseline";
  const changes = [...entry.changes].sort((a, b) => Number(!["status", "assignee"].includes(a.field)) - Number(!["status", "assignee"].includes(b.field))).slice(0, 2);
  return <div className="space-y-1">
    {((!changesDescribeAction && entry.issue) || showImpact) && <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
      {!changesDescribeAction && entry.issue && <p>{action}</p>}
      {showImpact && <span className="text-label font-medium tabular-nums">{eventImpactLabel(impact, t)}</span>}
    </div>}
    <EventChanges changes={changes} presentation={presentation} />
    {entry.changes.length > changes.length && <p className="text-caption text-muted-foreground">{t(($) => $.iterations.activityPanel.moreChanges, { count: entry.changes.length - changes.length })}</p>}
    <div className="text-label text-muted-foreground"><EventMovement entry={entry} catalogue={catalogue} /></div>
  </div>;
}

function ActivityGroupRows({ group, presentation, catalogue, timezone, impacts, matchingEventIds }: {
  group: IterationActivityGroup;
  presentation: Presentation;
  catalogue: Catalogue;
  timezone: string;
  impacts?: ReadonlyMap<string, IterationScopeImpact>;
  matchingEventIds?: ReadonlySet<string>;
}) {
  const { t } = useT("projects");
  const locale = useLocale();
  const [expanded, setExpanded] = useState(false);
  const detailsId = useId();
  const first = group.entries[0]!;
  const multiple = group.entries.length > 1;
  const matched = matchingEventIds ? group.entries.filter((entry) => matchingEventIds.has(entry.event.id)) : null;
  const entries = matched ?? group.entries;
  const issueCount = matched ? new Set(entries.flatMap((entry) => entry.issue ? [entry.issue.id] : [])).size : group.issueCount;
  const actor = new Intl.ListFormat(locale).format([...new Set(entries.map((entry) => presentation.identityLabel(entry.actor, true)))]);
  const subject = first.issue ? [first.issue.identifier, first.issue.title].filter(Boolean).join(" ") || t(($) => $.iterations.activityPanel.unnamedTask) : presentation.eventName(first.kind);
  const disclosureLabel = multiple
    ? t(($) => matched ? $.iterations.activityPanel.fullOperation : $.iterations.activityPanel.recordDetails, { count: group.entries.length })
    : t(($) => $.iterations.activityPanel.detailsFor, { subject, time: formatInTimeZone(first.event.occurred_at, timezone, locale, { year: "numeric", second: "2-digit" }), timezone });
  // Activity entries are descending by sequence. Reverse before deduping so
  // the later write keeps the newest saved title while preserving first-key order.
  const matchingTasks = matched ? [...new Map([...matched].reverse().filter((entry) => entry.issue).map((entry) => [entry.issue!.id, entry])).values()] : [];
  const cellClass = "block min-w-0 whitespace-normal p-0 [overflow-wrap:anywhere] @3xl:table-cell @3xl:px-3 @3xl:py-3";
  return <>
    <TableRow role="row" className="grid grid-cols-[minmax(0,1fr)_auto_2.75rem] items-center gap-x-3 gap-y-2 border-border/60 px-3 py-3 @3xl:table-row">
      <TableCell role="cell" className={`${cellClass} col-span-2 col-start-1 row-start-1`}>
        {multiple ? <div className="space-y-1"><p className="font-medium">{presentation.eventName(group.kind)}</p>{matchingTasks.map((entry) => <div key={entry.issue!.id}><ActivityTaskLink entry={entry} /></div>)}</div>
          : first.issue ? <ActivityTaskLink entry={first} /> : <p className="font-medium">{presentation.eventName(first.kind)}</p>}
      </TableCell>
      <TableCell role="cell" className={`${cellClass} col-span-3 col-start-1 row-start-2`}>
        {multiple ? <div className="space-y-1">
          <p className="flex flex-wrap gap-x-3 gap-y-1 text-label text-muted-foreground">
            <span>{t(($) => matched ? $.iterations.activityPanel.matchingRecords : $.iterations.activityPanel.recordCount, { count: entries.length })}</span>
            {issueCount > 0 && <span>{t(($) => matched ? $.iterations.activityPanel.matchingTasks : $.iterations.activityPanel.taskCount, { count: issueCount })}</span>}
          </p>
          {impacts && <p className="text-label font-medium tabular-nums">{new Intl.ListFormat(locale).format(scopeChangeImpact(entries.map((entry) => impacts.get(entry.event.id)), t))}</p>}
        </div> : <ActivityChange entry={first} presentation={presentation} catalogue={catalogue} impact={impacts?.get(first.event.id)} />}
      </TableCell>
      <TableCell role="cell" className={`${cellClass} col-start-1 row-start-3 text-label text-muted-foreground`}>{actor}</TableCell>
      <TableCell role="cell" className={`${cellClass} col-start-2 row-start-3 text-label text-muted-foreground tabular-nums @3xl:text-right`}><time dateTime={group.occurredAt} title={timezone} className="whitespace-nowrap">{formatInTimeZone(group.occurredAt, timezone, locale, { month: undefined, day: undefined })}</time></TableCell>
      <TableCell role="cell" className={`${cellClass} col-start-3 row-start-1 justify-self-end @3xl:pl-0 @3xl:pr-2 @3xl:text-right`}>
        <Button variant="ghost" size="icon" aria-label={disclosureLabel} title={disclosureLabel} aria-expanded={expanded} aria-controls={detailsId} onClick={() => setExpanded((open) => !open)} className="text-muted-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-foreground pointer-coarse:min-h-11 pointer-coarse:min-w-11">
          <ChevronRight className={cn("size-4", expanded && "rotate-90")} aria-hidden="true" />
        </Button>
      </TableCell>
    </TableRow>
    <TableRow id={detailsId} role="row" hidden={!expanded} className={cn("border-border/60 bg-muted/20 hover:bg-muted/20", expanded ? "block @3xl:table-row" : "hidden")}>
      <TableCell role="cell" colSpan={5} className="block min-w-0 whitespace-normal px-4 py-4 @3xl:table-cell @3xl:px-6">
        {multiple ? <ol className="max-w-3xl space-y-4">
          {group.entries.map((entry) => <li key={entry.event.id} className="min-w-0 space-y-2 border-b border-border/60 pb-4 last:border-0 last:pb-0">
            <EventSummary entry={entry} presentation={presentation} catalogue={catalogue} impact={impacts?.get(entry.event.id)} />
            <EventMeta at={entry.event.occurred_at} actor={presentation.identityLabel(entry.actor, true)} timezone={timezone} />
            <EventAudit entry={entry} timezone={timezone} />
          </li>)}
        </ol> : <EventDetails entry={first} presentation={presentation} catalogue={catalogue} impact={impacts?.get(first.event.id)} timezone={timezone} />}
      </TableCell>
    </TableRow>
  </>;
}

export function IterationActivityTimeline({ wsId, groups, timezone, impacts, matchingEventIds }: {
  wsId: string;
  groups: IterationActivityGroup[];
  timezone: string;
  impacts?: ReadonlyMap<string, IterationScopeImpact>;
  matchingEventIds?: ReadonlySet<string>;
}) {
  const { t } = useT("projects");
  const locale = useLocale();
  const headingPrefix = useId();
  const presentation = useEventPresentation(wsId, groups);
  const hasReferences = groups.some((group) => group.entries.some((entry) => entry.movement && (entry.movement.source || entry.movement.target)));
  const catalogue = useIterationCatalogue(wsId, hasReferences);
  return <div role="group" aria-label={t(($) => $.iterations.activityPanel.allFilter)} className="@container min-w-0 space-y-8">
    {iterationActivityDays(groups, timezone).map((day) => {
      const headingId = `${headingPrefix}-${day.id}`;
      return <section key={day.id} aria-labelledby={headingId} className="space-y-3">
        <h2 id={headingId} className="flex items-center gap-2 text-body font-semibold">
          <CalendarDays className="size-4 text-muted-foreground" aria-hidden="true" />
          <time dateTime={day.date}>{formatInTimeZone(day.groups[0]!.occurredAt, timezone, locale, { year: "numeric", hour: undefined, minute: undefined })}</time>
        </h2>
        <div className="min-w-0 rounded-md border border-border/70">
          <Table role="table" aria-labelledby={headingId} className="block table-fixed @3xl:table">
            <TableHeader role="rowgroup" className="sr-only @3xl:not-sr-only @3xl:table-header-group">
              <TableRow role="row" className="border-border/60 hover:bg-transparent">
                <TableHead scope="col" className="w-[30%] px-3 text-caption text-muted-foreground">{t(($) => $.iterations.activityPanel.ledgerTask)}</TableHead>
                <TableHead scope="col" className="px-3 text-caption text-muted-foreground">{t(($) => $.iterations.activityPanel.ledgerChange)}</TableHead>
                <TableHead scope="col" className="w-32 px-3 text-caption text-muted-foreground">{t(($) => $.iterations.activityPanel.ledgerActor)}</TableHead>
                <TableHead scope="col" className="w-24 px-3 text-right text-caption text-muted-foreground">{t(($) => $.iterations.activityPanel.ledgerTime)}</TableHead>
                <TableHead scope="col" className="w-14 px-2"><span className="sr-only">{t(($) => $.iterations.activityPanel.details)}</span></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody role="rowgroup" className="block @3xl:table-row-group">
              {day.groups.map((group) => <ActivityGroupRows key={group.id} group={group} presentation={presentation} catalogue={catalogue.data ?? []} timezone={timezone} impacts={impacts} matchingEventIds={matchingEventIds} />)}
            </TableBody>
          </Table>
        </div>
      </section>;
    })}
  </div>;
}

/** Embedded consumers keep access to all events; page-only filtering lives in the panel. */
export function IterationEventsView({ wsId, events, timezone }: { wsId: string; events: IterationEvent[]; timezone: string }) {
  return <IterationActivityTimeline wsId={wsId} groups={groupIterationActivity(events)} timezone={timezone} />;
}
