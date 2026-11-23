import { queryOptions } from "@tanstack/react-query";
import { api, ApiError, errorCode } from "../api";
import { protectIterationRead } from "./access";

export type IterationEvent = Awaited<ReturnType<typeof api.getIterationEvents>>["items"][number];

/** The API is ascending. Only a complete traversal can support latest-first filtering. */
export function iterationActivityOptions(wsId: string, id: string) {
  const queryKey = ["iterations", wsId, "activity", id] as const;
  return queryOptions({
    queryKey,
    retry: false,
    queryFn: async ({ client, signal }) => {
      try {
        return await protectIterationRead(client, wsId, async () => {
          for (let attempt = 0; ; attempt++) {
            try {
              const items: IterationEvent[] = [];
              const cursors = new Set<string>();
              const ids = new Set<string>();
              const sequences = new Set<number>();
              let cursor: string | null = null;
              do {
                signal.throwIfAborted();
                const page = await api.getIterationEvents(wsId, id, {
                  limit: "100",
                  ...(cursor ? { cursor } : {}),
                }, { signal });
                signal.throwIfAborted();
                if (page.workspace_id !== wsId || page.iteration_id !== id)
                  throw new Error("Iteration activity identity mismatch");
                for (const item of page.items) {
                  if (item.iteration_id !== id)
                    throw new Error("Iteration activity identity mismatch");
                  if (ids.has(item.id)) throw new Error("Repeated iteration activity identity");
                  if (sequences.has(item.sequence)) throw new Error("Repeated iteration activity sequence");
                  ids.add(item.id);
                  sequences.add(item.sequence);
                  items.push(item);
                }
                cursor = page.next_cursor;
                if (cursor && cursors.has(cursor)) throw new Error("Repeated iteration activity cursor");
                if (cursor) cursors.add(cursor);
              } while (cursor);
              return items;
            } catch (error) {
              signal.throwIfAborted();
              // A cursor belongs to one collection revision. Discard that entire
              // attempt, and never let Query add more retries around this limit.
              if (attempt > 0 || errorCode(error) !== "cursor_stale") throw error;
            }
          }
        });
      } catch (error) {
        // The access guard already erases revoked workspaces and fences old
        // sessions. A missing iteration erases only this entity's cached history.
        if (error instanceof ApiError && error.status === 404) {
          client.getQueryCache().find({ queryKey, exact: true })?.setState({
            data: undefined,
            dataUpdatedAt: 0,
          });
        }
        throw error;
      }
    },
  });
}

type StoredText = string | null | undefined;
export type IterationActivityFilter = "scope" | "all";
export type IterationActivityKind =
  | "join" | "leave" | "reenter" | "baseline" | "status" | "delete"
  | "cancelIssue" | "reopen" | "issueChanged" | "executionStarted"
  | "create" | "edit" | "dateEdit" | "start" | "end" | "cancelIteration"
  | "rollover" | "plan" | "unknown";
export type IterationActivityIdentity = { type: StoredText; id: StoredText; name: StoredText };
export type IterationActivityActor = IterationActivityIdentity & { userId: StoredText; source: StoredText };
export type IterationActivityStatus = { key: StoredText; category: StoredText };
export type IterationActivityFacts = {
  title: StoredText;
  identifier: StoredText;
  name: StoredText;
  description: StoredText;
  status: IterationActivityStatus | undefined;
  assignee: IterationActivityIdentity | undefined;
  coordinator: IterationActivityIdentity | undefined;
  project: IterationActivityIdentity | undefined;
  startDate: StoredText;
  endDate: StoredText;
  timezone: StoredText;
  sourceIterationId: StoredText;
  targetIterationId: StoredText;
};
export type IterationActivityChange =
  | { field: "title" | "name" | "description" | "startDate" | "endDate" | "timezone"; before: StoredText; after: StoredText }
  | { field: "status"; before: IterationActivityStatus | undefined; after: IterationActivityStatus | undefined }
  | { field: "assignee" | "coordinator" | "project"; before: IterationActivityIdentity | undefined; after: IterationActivityIdentity | undefined };
export type IterationActivityEntry = {
  event: IterationEvent;
  kind: IterationActivityKind;
  scopeRelated: boolean;
  actor: IterationActivityActor;
  issue: { id: string; title: string | undefined; identifier: string | undefined } | null;
  before: IterationActivityFacts;
  after: IterationActivityFacts;
  changes: IterationActivityChange[];
  movement: { source: StoredText; target: StoredText } | undefined;
};
export type IterationActivityGroup = {
  id: string;
  kind: IterationActivityKind;
  sequence: number;
  occurredAt: string;
  issueCount: number;
  entries: IterationActivityEntry[];
};

function record(value: unknown): Record<string, unknown> | undefined {
  return value !== null && typeof value === "object" && !Array.isArray(value)
    ? value as Record<string, unknown> : undefined;
}
function text(facts: Record<string, unknown> | undefined, key: string): StoredText {
  if (!facts || !Object.hasOwn(facts, key)) return undefined;
  const value = facts[key];
  return value === null || typeof value === "string" ? value : undefined;
}
function reference(facts: Record<string, unknown> | undefined, key: string): StoredText {
  const value = text(facts, key);
  return value === null || (typeof value === "string" && /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(value))
    ? value : undefined;
}
function has(facts: Record<string, unknown> | undefined, ...keys: string[]) {
  return !!facts && keys.some((key) => Object.hasOwn(facts, key));
}
function facts(value: unknown): IterationActivityFacts {
  const raw = record(value);
  const statusKey = text(raw, "status_key");
  return {
    title: text(raw, "title"),
    identifier: text(raw, "identifier"),
    name: text(raw, "name"),
    description: text(raw, "description"),
    status: has(raw, "status_key", "status_category", "status") ? {
      key: statusKey === undefined ? text(raw, "status") : statusKey,
      category: text(raw, "status_category"),
    } : undefined,
    assignee: has(raw, "assignee_type", "assignee_id", "assignee_name") ? {
      type: text(raw, "assignee_type"), id: reference(raw, "assignee_id"), name: text(raw, "assignee_name"),
    } : undefined,
    coordinator: has(raw, "coordinator_user_id", "coordinator_name") ? {
      type: "member", id: reference(raw, "coordinator_user_id"), name: text(raw, "coordinator_name"),
    } : undefined,
    project: has(raw, "project_id", "project_name") ? {
      type: "project", id: reference(raw, "project_id"), name: text(raw, "project_name"),
    } : undefined,
    startDate: text(raw, "start_date"),
    endDate: text(raw, "end_date"),
    timezone: text(raw, "timezone"),
    sourceIterationId: reference(raw, "source_iteration_id"),
    targetIterationId: reference(raw, "target_iteration_id"),
  };
}
function activityKind(event: IterationEvent): IterationActivityKind {
  switch (event.kind) {
    case "join": case "leave": case "reenter": case "baseline": case "reopen":
    case "create": case "edit": case "start": case "rollover": return event.kind;
    case "status": case "status_changed": case "status_change": return "status";
    case "delete": case "deleted": return "delete";
    case "cancel": return event.issue_id === null ? "cancelIteration" : "cancelIssue";
    case "cancelled": case "cancel_planned": return "cancelIteration";
    case "issue_changed": return "issueChanged";
    case "execution_started": return "executionStarted";
    case "date_edit": return "dateEdit";
    case "end": case "completed": return "end";
    case "planned_activity":
      if (event.issue_id && event.before_facts === null && record(event.after_facts)) return "join";
      if (event.issue_id && event.after_facts === null && record(event.before_facts)) return "leave";
      return "plan";
    default: return "unknown";
  }
}
function changes(before: IterationActivityFacts, after: IterationActivityFacts): IterationActivityChange[] {
  const result: IterationActivityChange[] = [];
  for (const field of ["title", "name", "description"] as const) {
    if (before[field] !== after[field]) result.push({ field, before: before[field], after: after[field] });
  }
  if (before.status?.key !== after.status?.key || before.status?.category !== after.status?.category)
    result.push({ field: "status", before: before.status, after: after.status });
  for (const field of ["assignee", "coordinator", "project"] as const) {
    const old = before[field];
    const next = after[field];
    if (old?.id !== next?.id || old?.type !== next?.type || old?.name !== next?.name)
      result.push({ field, before: old, after: next });
  }
  for (const field of ["startDate", "endDate", "timezone"] as const) {
    if (before[field] !== after[field]) result.push({ field, before: before[field], after: after[field] });
  }
  return result;
}
const readable = (value: StoredText): value is string => typeof value === "string" && value.trim().length > 0;

/** Historical values are projected once here; renderers never decode the raw payload. */
export function projectIterationEvent(event: IterationEvent): IterationActivityEntry {
  const before = facts(event.before_facts);
  const after = facts(event.after_facts);
  const kind = activityKind(event);
  const isMove = event.issue_id !== null && ["join", "leave", "reenter", "rollover"].includes(kind);
  const rawActor = record(event.actor);
  const actorType = text(rawActor, "type");
  const actorId = reference(rawActor, "id");
  const userId = reference(rawActor, "user_id");
  const fromCategory = before.status?.category;
  const toCategory = after.status?.category;
  const cancelledCrossing = readable(fromCategory) && readable(toCategory)
    && (fromCategory === "cancelled") !== (toCategory === "cancelled");
  const scopeRelated = isMove || cancelledCrossing
    || ["baseline", "start", "end", "cancelIteration", "cancelIssue", "delete", "rollover"].includes(kind);
  const describeChanges = !isMove && !["baseline", "create", "delete", "start", "end", "cancelIteration"].includes(kind);
  return {
    event,
    kind,
    scopeRelated,
    actor: {
      type: actorType,
      id: actorType === "member" ? userId ?? actorId : actorId,
      userId,
      name: text(rawActor, "name"),
      source: text(rawActor, "source"),
    },
    issue: event.issue_id ? {
      id: event.issue_id,
      title: [after.title, before.title].find(readable),
      identifier: [after.identifier, before.identifier].find(readable),
    } : null,
    before,
    after,
    changes: describeChanges ? changes(before, after) : [],
    movement: isMove ? {
      source: before.sourceIterationId === undefined ? after.sourceIterationId : before.sourceIterationId,
      target: after.targetIterationId === undefined ? before.targetIterationId : after.targetIterationId,
    } : undefined,
  };
}

export function groupIterationActivity(events: readonly IterationEvent[], filter: IterationActivityFilter = "all"): IterationActivityGroup[] {
  const operations = new Map<string, IterationActivityEntry[]>();
  for (const event of [...events].sort((a, b) => b.sequence - a.sequence)) {
    const key = `${event.iteration_id}:${event.operation_id}`;
    const entry = projectIterationEvent(event);
    const group = operations.get(key);
    if (group) group.push(entry);
    else operations.set(key, [entry]);
  }
  const groups: IterationActivityGroup[] = [];
  for (const [id, entries] of operations) {
    if (filter === "scope" && !entries.some((entry) => entry.scopeRelated)) continue;
    const first = entries[0]!;
    const anchor = entries.find((entry) => ["start", "end", "cancelIteration"].includes(entry.kind));
    const relevant = entries.find((entry) => entry.scopeRelated);
    groups.push({
      id,
      kind: anchor?.kind ?? (entries.every((entry) => entry.event.kind === "planned_activity") ? "plan" : (relevant ?? first).kind),
      sequence: first.event.sequence,
      occurredAt: first.event.occurred_at,
      issueCount: new Set(entries.flatMap((entry) => entry.issue ? [entry.issue.id] : [])).size,
      entries,
    });
  }
  return groups;
}

export function iterationActivityDays(groups: readonly IterationActivityGroup[], timezone: string) {
  const formatter = new Intl.DateTimeFormat("en-US-u-ca-iso8601", {
    timeZone: timezone, year: "numeric", month: "2-digit", day: "2-digit",
  });
  const days: { id: string; date: string; groups: IterationActivityGroup[] }[] = [];
  const occurrences = new Map<string, number>();
  for (const group of groups) {
    const parts = formatter.formatToParts(new Date(group.occurredAt));
    const date = ["year", "month", "day"].map((type) => parts.find((part) => part.type === type)?.value).join("-");
    // Contiguous buckets retain sequence order even if business timestamps were
    // backdated. Coalescing all equal dates would silently reorder operations.
    const previous = days.at(-1);
    if (previous?.date === date) previous.groups.push(group);
    else {
      const occurrence = occurrences.get(date) ?? 0;
      occurrences.set(date, occurrence + 1);
      days.push({ id: `${date}:${occurrence}`, date, groups: [group] });
    }
  }
  return days;
}
