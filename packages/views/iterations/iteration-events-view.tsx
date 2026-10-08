"use client";
import { useQuery } from "@tanstack/react-query";
import type { api } from "@multica/core/api";
import { memberListOptions, agentListOptions, squadListOptions } from "@multica/core/workspace/queries";
import { useT, useLocale } from "../i18n";
import { formatInTimeZone } from "../common/format-in-time-zone";
import { IterationReference, useIterationCatalogue } from "./iteration-catalogue";

type Event = Awaited<ReturnType<typeof api.getIterationEvents>>["items"][number];
function record(value: unknown): Record<string, unknown> | undefined {
  return value !== null && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : undefined;
}
function reference(event: Event, key: string): string | null | undefined {
  for (const value of [event.before_facts, event.after_facts]) {
    const facts = record(value);
    if (facts && Object.hasOwn(facts, key)) {
      const id = facts[key];
      return id === null ? null : typeof id === "string" && /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(id) ? id : undefined;
    }
  }
  return undefined;
}
export function IterationEventsView({ wsId, events, timezone }: { wsId: string; events: Event[]; timezone: string }) {
  const { t } = useT("projects");
  const locale = useLocale();
  const catalogue = useIterationCatalogue(wsId);
  const members = useQuery(memberListOptions(wsId));
  const agents = useQuery({ ...agentListOptions(wsId), enabled: events.some((event) => record(event.actor)?.type === "agent") });
  const squads = useQuery({ ...squadListOptions(wsId), enabled: events.some((event) => record(event.actor)?.type === "squad") });
  const eventName = (kind: string, hasIssue: boolean) => {
    switch (kind) {
      case "join": return t(($) => $.iterations.eventJoin);
      case "leave": return t(($) => $.iterations.eventLeave);
      case "reenter": return t(($) => $.iterations.eventReenter);
      case "baseline": return t(($) => $.iterations.eventBaseline);
      case "status": case "status_changed": case "status_change": return t(($) => $.iterations.eventStatus);
      case "delete": case "deleted": return t(($) => $.iterations.eventDelete);
      case "cancel": return t(($) => hasIssue ? $.iterations.eventCancel : $.iterations.eventCancelled);
      case "reopen": return t(($) => $.iterations.eventReopen);
      case "issue_changed": return t(($) => $.iterations.eventChanged);
      case "execution_started": return t(($) => $.iterations.eventExecution);
      case "create": return t(($) => $.iterations.eventCreate);
      case "edit": return t(($) => $.iterations.eventEdit);
      case "date_edit": return t(($) => $.iterations.eventDates);
      case "start": return t(($) => $.iterations.eventStart);
      case "end": case "completed": return t(($) => $.iterations.eventEnd);
      case "cancelled": case "cancel_planned": return t(($) => $.iterations.eventCancelled);
      case "rollover": return t(($) => $.iterations.eventRollover);
      case "planned_activity": return t(($) => $.iterations.eventPlan);
      default: return `${t(($) => $.iterations.eventUnknown)} (${kind})`;
    }
  };
  return <div className="space-y-3">
    <p className="text-caption text-muted-foreground">{t(($) => $.iterations.currentNames)}</p>
    <ul className="space-y-3">{events.map((event) => {
      const actor = record(event.actor);
      const type = typeof actor?.type === "string" ? actor.type : "";
      const id = typeof actor?.user_id === "string" ? actor.user_id : typeof actor?.id === "string" ? actor.id : "";
      const name = type === "member" ? members.data?.find((member) => member.user_id === id)?.name : type === "agent" ? agents.data?.find((agent) => agent.id === id)?.name : type === "squad" ? squads.data?.find((squad) => squad.id === id)?.name : undefined;
      const facts = [record(event.after_facts), record(event.before_facts)];
      const title = facts.map((value) => value?.title).find((value): value is string => typeof value === "string" && value.trim().length > 0);
      const identifier = facts.map((value) => value?.identifier).find((value): value is string => typeof value === "string" && value.trim().length > 0);
      return <li key={event.id} className="break-words">
        <p className="font-medium">{eventName(event.kind, event.issue_id !== null)}</p>
        {event.issue_id && <p>{t(($) => $.iterations.identifier)}: {identifier ?? event.issue_id}{title && ` · ${title}`}</p>}
        <p>{t(($) => $.iterations.actorLabel)}: {name ?? ([type, id].filter(Boolean).join(" · ") || t(($) => $.iterations.unknownHistory))} · <time dateTime={event.occurred_at} title={timezone}>{formatInTimeZone(event.occurred_at, timezone, locale, { year: "numeric" })}</time></p>
        <p><IterationReference id={reference(event, "source_iteration_id")} catalogue={catalogue.data ?? []} /> → <IterationReference id={reference(event, "target_iteration_id")} catalogue={catalogue.data ?? []} /></p>
        {event.reason && <p className="whitespace-pre-wrap">{event.reason}</p>}
      </li>;
    })}</ul>
  </div>;
}
