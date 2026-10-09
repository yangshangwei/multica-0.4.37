"use client";

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { ArrowRight } from "lucide-react";
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

function EventSummary({ entry, presentation, catalogue, impact }: { entry: IterationActivityEntry; presentation: Presentation; catalogue: Catalogue; impact?: IterationScopeImpact }) {
  const { t } = useT("projects");
  const paths = useWorkspacePaths();
  const impactLabel = impact?.kind === "effective" ? impact.delta === 0 ? t(($) => $.iterations.activityPanel.scopeUnchanged) : t(($) => $.iterations.activityPanel.effectiveImpact, { delta: `${impact.delta > 0 ? "+" : ""}${impact.delta}` })
    : impact?.kind === "planning" ? t(($) => $.iterations.activityPanel.planningImpact)
      : impact?.kind === "baseline" ? t(($) => $.iterations.activityPanel.baselineImpact)
        : impact?.kind === "unknown" ? t(($) => $.iterations.activityPanel.unknownImpact) : null;
  const restored = impact?.kind === "effective" && impact.delta === 1 && entry.before.status?.category === "cancelled";
  return <div className="min-w-0 space-y-2 [overflow-wrap:anywhere]">
    <div className="flex min-w-0 flex-wrap items-baseline gap-x-4 gap-y-1">
      {entry.issue && <p className="min-w-0 font-medium">
        {entry.issue.identifier && <span className="mr-2 text-caption text-muted-foreground">{entry.issue.identifier}</span>}
        {entry.issue.title ?? (entry.issue.identifier ? null : t(($) => $.iterations.activityPanel.unnamedTask))}
      </p>}
      {impactLabel && <span className="text-caption text-muted-foreground tabular-nums">{impactLabel}</span>}
    </div>
    <div className="flex flex-wrap items-center gap-x-4 gap-y-1">
      <p className={entry.issue ? "text-caption text-muted-foreground" : "font-medium"}>{restored ? t(($) => $.iterations.activityPanel.restoredTask) : presentation.eventName(entry.kind)}</p>
      {impact && entry.issue && <AppLink href={paths.issueDetail(entry.issue.id)} aria-label={t(($) => $.iterations.activityPanel.currentTaskLink, { task: entry.issue.identifier || entry.issue.title || t(($) => $.iterations.activityPanel.unnamedTask) })} className="inline-flex min-h-7 items-center rounded text-caption text-muted-foreground underline underline-offset-4 hover:text-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-foreground pointer-coarse:min-h-11">{t(($) => $.iterations.openCurrent)}</AppLink>}
    </div>
    {entry.movement && <p className="flex flex-wrap items-center gap-x-2 gap-y-1 text-caption">
      <span><span className="sr-only">{t(($) => $.iterations.source)}: </span><IterationReference id={entry.movement.source} catalogue={catalogue} /></span>
      <ArrowRight className="size-3.5 text-muted-foreground" aria-hidden="true" />
      <span><span className="sr-only">{t(($) => $.iterations.destination)}: </span><IterationReference id={entry.movement.target} catalogue={catalogue} /></span>
    </p>}
    {entry.changes.length > 0 && <dl className="space-y-1.5 text-caption">
      {entry.changes.map((change) => <div key={change.field} className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
        <dt className="text-muted-foreground">{presentation.fieldLabel(change.field)}</dt>
        <dd className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
          <span className="min-w-0 whitespace-pre-wrap"><span className="sr-only">{t(($) => $.iterations.activityPanel.before)}: </span>{presentation.changeValue(change, "before")}</span>
          <ArrowRight className="size-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
          <span className="min-w-0 whitespace-pre-wrap"><span className="sr-only">{t(($) => $.iterations.activityPanel.after)}: </span>{presentation.changeValue(change, "after")}</span>
        </dd>
      </div>)}
    </dl>}
    {entry.event.reason && <p className="whitespace-pre-wrap text-caption">{entry.event.reason}</p>}
  </div>;
}

function EventMeta({ at, actor, timezone }: { at: string; actor: string; timezone: string }) {
  const locale = useLocale();
  return <p className="flex flex-wrap items-baseline gap-x-2 gap-y-1 text-caption text-muted-foreground [overflow-wrap:anywhere]">
    <span>{actor}</span><span aria-hidden="true">·</span>
    <time dateTime={at} title={timezone} className="tabular-nums">{formatInTimeZone(at, timezone, locale, { month: undefined, day: undefined })}</time>
  </p>;
}

function EventDetails({ entry, timezone }: { entry: IterationActivityEntry; timezone: string }) {
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

export function IterationActivityTimeline({ wsId, groups, timezone, impacts, matchingEventIds }: {
  wsId: string;
  groups: IterationActivityGroup[];
  timezone: string;
  impacts?: ReadonlyMap<string, IterationScopeImpact>;
  matchingEventIds?: ReadonlySet<string>;
}) {
  const { t } = useT("projects");
  const locale = useLocale();
  const presentation = useEventPresentation(wsId, groups);
  const hasReferences = groups.some((group) => group.entries.some((entry) => entry.movement && (entry.movement.source || entry.movement.target)));
  const catalogue = useIterationCatalogue(wsId, hasReferences);
  return <div className="min-w-0 space-y-7">
    {iterationActivityDays(groups, timezone).map((day) => <section key={day.id} className="space-y-4">
      <h2 className="text-caption font-semibold">{formatInTimeZone(day.groups[0]!.occurredAt, timezone, locale, { year: "numeric", hour: undefined, minute: undefined })}</h2>
      <ol className="space-y-5">
        {day.groups.map((group) => {
          const first = group.entries[0]!;
          const multiple = group.entries.length > 1;
          const actors = [...new Set(group.entries.map((entry) => presentation.identityLabel(entry.actor, true)))].join(", ");
          const matched = matchingEventIds ? group.entries.filter((entry) => matchingEventIds.has(entry.event.id)) : null;
          const record = (entry: IterationActivityEntry) => <li key={entry.event.id} className="min-w-0 space-y-1.5">
            <EventSummary entry={entry} presentation={presentation} catalogue={catalogue.data ?? []} impact={impacts?.get(entry.event.id)} />
            <EventMeta at={entry.event.occurred_at} actor={presentation.identityLabel(entry.actor, true)} timezone={timezone} />
            <EventDetails entry={entry} timezone={timezone} />
          </li>;
          return <li key={group.id} className="min-w-0 space-y-1.5">
            {multiple ? <p className="font-medium [overflow-wrap:anywhere]">
              {presentation.eventName(group.kind)}
              {matched ? <span className="ml-2 text-caption font-normal text-muted-foreground">{t(($) => $.iterations.activityPanel.matchingRecords, { count: matched.length })}</span>
                : group.issueCount > 0 && <span className="ml-2 text-caption font-normal text-muted-foreground">{t(($) => $.iterations.activityPanel.taskCount, { count: group.issueCount })}</span>}
            </p> : <EventSummary entry={first} presentation={presentation} catalogue={catalogue.data ?? []} impact={impacts?.get(first.event.id)} />}
            <EventMeta at={group.occurredAt} actor={actors} timezone={timezone} />
            {multiple && matched && <ol className="space-y-5 py-3 pl-3 sm:pl-4">{matched.map(record)}</ol>}
            {multiple ? <details>
              <summary className={iterationDisclosureClass}>{t(($) => matched ? $.iterations.activityPanel.fullOperation : $.iterations.activityPanel.recordDetails, { count: group.entries.length })}</summary>
              <ol className="mt-3 space-y-5 pl-3 sm:pl-4">
                {group.entries.map(record)}
              </ol>
            </details> : <EventDetails entry={first} timezone={timezone} />}
          </li>;
        })}
      </ol>
    </section>)}
  </div>;
}

/** Embedded consumers keep access to all events; page-only filtering lives in the panel. */
export function IterationEventsView({ wsId, events, timezone }: { wsId: string; events: IterationEvent[]; timezone: string }) {
  return <IterationActivityTimeline wsId={wsId} groups={groupIterationActivity(events)} timezone={timezone} />;
}
