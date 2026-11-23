"use client";
import type { ReactNode } from "react";
import {
  Check,
  CircleHelp,
  Clock3,
  Copy,
  Inbox,
  Play,
  RotateCcw,
  UserRoundCheck,
  X,
} from "lucide-react";
import { ApiError } from "@multica/core/api";
import { issueDetailOptions } from "@multica/core/issues/queries";
import { useWorkspacePaths } from "@multica/core/paths";
import { AppLink } from "../navigation";
import { useQuery } from "@tanstack/react-query";
import {
  triageItemHistoryOptions,
  useRetryTriageExecution,
  type TriageAction,
  type TriageHistoryEntry,
} from "@multica/core/triage";
import { projectListOptions } from "@multica/core/projects/queries";
import { useActorName } from "@multica/core/workspace/hooks";
import { useStatusLabel } from "../issues/utils/status-label";
import { memberListOptions } from "@multica/core/workspace/queries";
import { useCurrentMember } from "@multica/core/permissions";
import { Button } from "@multica/ui/components/ui/button";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../i18n";
import {
  TRIAGE_CONTROL,
  TRIAGE_HISTORY_ACTIONS,
  triageSnapshotChanges,
  type TriageHistoryField,
} from "./triage-ui";

function historyAppearance(action: string) {
  switch (action) {
    case "accept":
      return { Icon: Check, tone: "text-success-foreground" };
    case "accept_and_execute":
      return { Icon: Play, tone: "text-success-foreground" };
    case "reject":
      return { Icon: X, tone: "text-destructive-foreground" };
    case "reopen":
      return { Icon: RotateCcw, tone: "text-info-foreground" };
    case "snooze":
      return { Icon: Clock3, tone: "text-warning-foreground" };
    case "duplicate":
      return { Icon: Copy, tone: "text-muted-foreground" };
    case "unsnooze":
      return { Icon: Inbox, tone: "text-muted-foreground" };
    case "assign_reviewer":
      return { Icon: UserRoundCheck, tone: "text-muted-foreground" };
    default:
      return { Icon: CircleHelp, tone: "text-muted-foreground" };
  }
}

export function TriageHistoryRecord({
  wsId,
  entry,
  children,
}: {
  wsId: string;
  entry: TriageAction | TriageHistoryEntry;
  children?: ReactNode;
}) {
  const { t } = useT("triage");
  const knownAction = TRIAGE_HISTORY_ACTIONS.find(
    (name) => name === entry.action,
  );
  return (
    <article className="min-w-0 space-y-2 text-body [overflow-wrap:anywhere]">
      <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
        <span
          className={cn("font-semibold", historyAppearance(entry.action).tone)}
        >
          {knownAction
            ? t(($) => $.history_timeline.actions[knownAction])
            : entry.action === "import"
              ? t(($) => $.import_csv)
              : t(($) => $.unknown)}
        </span>
        {"round" in entry && (
          <span className="rounded bg-muted px-1.5 py-0.5 text-caption font-medium text-muted-foreground">
            {t(($) => $.round, { round: entry.round })}
          </span>
        )}
      </div>
      <TriageHistoryContent wsId={wsId} entry={entry} variant="item" />
      {children}
    </article>
  );
}

/** Shared snapshot presentation for global and single-item review history. */
export function TriageHistoryContent({
  wsId,
  entry,
  compact = false,
  variant = "default",
}: {
  wsId: string;
  entry: TriageAction | TriageHistoryEntry;
  compact?: boolean;
  variant?: "default" | "item";
}) {
  const { t, i18n } = useT("triage");
  const { data: members = [] } = useQuery(memberListOptions(wsId));
  const actor = members.find((m) => m.user_id === entry.actor_id);
  const { data: projects = [] } = useQuery(projectListOptions(wsId));
  const { getActorName } = useActorName();
  const statusLabel = useStatusLabel(wsId);
  const changes = triageSnapshotChanges(entry.before, entry.after);
  const fieldLabel = (field: TriageHistoryField) => {
    switch (field) {
      case "title":
        return t(($) => $.title_field);
      case "status":
        return t(($) => $.acceptance_status);
      case "project_id":
        return t(($) => $.project);
      case "assignee_id":
        return t(($) => $.assignee);
      case "admission_status":
        return t(($) => $.result);
      case "reviewer_id":
        return t(($) => $.reviewer);
      case "snoozed_until":
        return t(($) => $.snooze);
      case "duplicate_identifier":
        return t(($) => $.duplicate_target);
      case "candidate_project_id":
        return t(($) => $.candidate_project);
      case "candidate_assignee_id":
        return t(($) => $.candidate_assignee);
      default:
        return t(($) => $[field]);
    }
  };
  const formatValue = (
    field: TriageHistoryField,
    value: unknown,
    snapshot: Record<string, unknown>,
  ): string => {
    if (value === null || value === undefined || value === "")
      return t(($) => $.none);
    if (Array.isArray(value))
      return (
        value
          .map((v) =>
            typeof v === "string"
              ? v
              : v && typeof v === "object" && typeof v.name === "string"
                ? v.name
                : "",
          )
          .filter(Boolean)
          .join(", ") || t(($) => $.none)
      );
    if (typeof value !== "string")
      return typeof value === "number" ? String(value) : t(($) => $.unknown);
    if (field === "priority") {
      const known = (["none", "low", "medium", "high", "urgent"] as const).find(
        (p) => p === value,
      );
      return known ? t(($) => $.priorities[known]) : value;
    }
    if (field === "status") return statusLabel(value);
    if (field === "admission_status") {
      const known = (
        [
          "pending",
          "accepted",
          "rejected",
          "duplicate",
          "not_required",
        ] as const
      ).find((s) => s === value);
      return known ? t(($) => $[known]) : value;
    }
    if (field === "reviewer_id")
      return (
        members.find((m) => m.user_id === value)?.name ??
        t(($) => $.removed_member)
      );
    if (field === "project_id" || field === "candidate_project_id")
      return projects.find((p) => p.id === value)?.title ?? value;
    if (field === "assignee_id" || field === "candidate_assignee_id") {
      const issue = snapshot.issue;
      const nested =
        issue && typeof issue === "object" && !Array.isArray(issue)
          ? (issue as Record<string, unknown>)
          : {};
      const type =
        field === "candidate_assignee_id"
          ? snapshot.candidate_assignee_type
          : (nested.assignee_type ?? snapshot.assignee_type);
      return type === "member" || type === "agent" || type === "squad"
        ? getActorName(type, value)
        : value;
    }
    if (field === "snoozed_until")
      return new Date(value).toLocaleString(i18n.language, {
        timeZoneName: "short",
      });
    return value;
  };

  const source = entry.after.source ?? entry.before.source;
  const itemDate = variant === "item" ? new Date(entry.created_at) : null;
  return (
    <div className="min-w-0 space-y-2 [overflow-wrap:anywhere]">
      <div
        className={
          variant === "item"
            ? "flex flex-wrap items-baseline gap-x-3 gap-y-1"
            : compact
              ? "flex flex-wrap gap-x-3 gap-y-1"
              : "space-y-2"
        }
      >
        <p className="text-caption text-muted-foreground">
          {actor?.name ?? actor?.email ?? t(($) => $.unknown)}
          {variant !== "item" &&
            "round" in entry &&
            ` · ${t(($) => $.round, { round: entry.round })}`}
        </p>
        {itemDate &&
          (Number.isNaN(itemDate.getTime()) ? (
            <span className="text-caption text-muted-foreground">
              {t(($) => $.history_timeline.unknown_time)}
            </span>
          ) : (
            <time
              className="text-caption tabular-nums text-muted-foreground"
              dateTime={entry.created_at}
            >
              {itemDate.toLocaleString(i18n.language, { timeZoneName: "short" })}
            </time>
          ))}
        {(source === "manual" || source === "csv") && (
          <p className="text-caption text-muted-foreground">
            {t(($) => $[source])}
          </p>
        )}
      </div>
      {entry.reason && (
        <p className="max-w-prose whitespace-pre-wrap">{entry.reason}</p>
      )}
      {changes.length > 0 && (
        <details>
          <summary
            className={cn(
              TRIAGE_CONTROL,
              "w-fit max-w-full cursor-pointer rounded-sm text-caption text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
            )}
          >
            {t(($) => $.changes)}
          </summary>
          <p className="mt-2 text-caption text-muted-foreground">
            {t(($) => $.snapshot_names_hint)}
          </p>
          <dl className="mt-3 space-y-3 text-caption">
            {changes.map((change) => (
              <div key={change.field} className="space-y-1">
                <dt className="font-medium">{fieldLabel(change.field)}</dt>
                <dd className="grid min-w-0 gap-2 sm:grid-cols-2">
                  <div className="min-w-0 whitespace-pre-wrap">
                    <span className="text-muted-foreground">
                      {t(($) => $.before)}:{" "}
                    </span>
                    {formatValue(change.field, change.before, entry.before)}
                  </div>
                  <div className="min-w-0 whitespace-pre-wrap">
                    <span className="text-muted-foreground">
                      {t(($) => $.after)}:{" "}
                    </span>
                    {formatValue(change.field, change.after, entry.after)}
                  </div>
                </dd>
              </div>
            ))}
          </dl>
        </details>
      )}
    </div>
  );
}

export function TriageItemHistory({
  wsId,
  issueId,
}: {
  wsId: string;
  issueId: string;
}) {
  const { t } = useT("triage");
  const query = useQuery(triageItemHistoryOptions(wsId, issueId));
  const retry = useRetryTriageExecution(wsId);
  const { userId } = useCurrentMember(wsId);
  return (
    <div className="space-y-2">
      {query.isPending ? (
        <p role="status">{t(($) => $.loading)}</p>
      ) : query.isError ? (
        <p role="alert">{query.error.message}</p>
      ) : !query.data?.events.length ? (
        <p className="text-caption text-muted-foreground">
          {t(($) => $.history_empty)}
        </p>
      ) : (
        <ol aria-label={t(($) => $.history)}>
          {query.data.events.map((event) => {
            const { Icon, tone } = historyAppearance(event.action);
            return (
              <li key={event.id} className="group/event relative pb-5 pl-9 last:pb-0">
                <div aria-hidden="true" className="absolute inset-y-0 left-0 w-6">
                  <span className="absolute bottom-0 left-3 top-3 border-l border-surface-border group-last/event:hidden" />
                  <span className={cn("relative flex size-6 items-center justify-center rounded-full bg-background", tone)}>
                    <Icon className="size-4" />
                  </span>
                </div>
                <TriageHistoryRecord wsId={wsId} entry={event}>
                  {(event.execution_status === "failed" ||
                    event.execution_status === "pending") && (
                    <div className="space-y-2 rounded-md bg-muted p-3" role="status">
                      <p className="font-medium">
                        {event.execution_status === "failed"
                          ? t(($) => $.execution_failed)
                          : t(($) => $.execution_pending)}
                      </p>
                      {event.execution_error && (
                        <p className="text-caption text-destructive">
                          {event.execution_error}
                        </p>
                      )}
                      {userId === event.actor_id && (
                        <Button
                          variant="outline"
                          className={TRIAGE_CONTROL}
                          disabled={retry.isPending}
                          onClick={() => retry.mutate(event.id)}
                        >
                          {t(($) => $.retry_execution)}
                        </Button>
                      )}
                    </div>
                  )}
                  {event.execution_status === "queued" && (
                    <p className="text-caption text-muted-foreground">
                      {t(($) => $.execution_queued)}
                    </p>
                  )}
                </TriageHistoryRecord>
              </li>
            );
          })}
        </ol>
      )}
      {retry.isError && (
        <p role="alert" className="text-destructive">
          {retry.error.message}
        </p>
      )}
    </div>
  );
}

export function TriageExecutionStatus({
  wsId,
  issueId,
}: {
  wsId: string;
  issueId: string;
}) {
  const { t } = useT("triage");
  const { data } = useQuery(triageItemHistoryOptions(wsId, issueId));
  const { userId } = useCurrentMember(wsId);
  const retry = useRetryTriageExecution(wsId);
  const event = data?.events.find(
    (event) =>
      event.execution_status === "failed" ||
      event.execution_status === "pending",
  );
  if (!event) return null;
  return (
    <div role="status" className="space-y-2 rounded-md bg-muted p-3 text-body">
      <p className="font-medium">
        {event.execution_status === "failed"
          ? t(($) => $.execution_failed)
          : t(($) => $.execution_pending)}
      </p>
      {event.execution_error && (
        <p className="text-caption text-destructive">{event.execution_error}</p>
      )}
      {userId === event.actor_id && (
        <Button
          variant="outline"
          className={TRIAGE_CONTROL}
          disabled={retry.isPending}
          onClick={() => retry.mutate(event.id)}
        >
          {t(($) => $.retry_execution)}
        </Button>
      )}
      {retry.isError && (
        <p role="alert" className="text-destructive">
          {retry.error.message}
        </p>
      )}
    </div>
  );
}

export function TriageDuplicateTarget({
  wsId,
  issueId,
  identifier,
}: {
  wsId: string;
  issueId: string | null;
  identifier: string;
}) {
  const { t } = useT("triage");
  const paths = useWorkspacePaths();
  const query = useQuery({
    ...issueDetailOptions(wsId, issueId ?? ""),
    enabled: !!issueId,
  });
  const deleted =
    !issueId ||
    (query.error instanceof ApiError && query.error.status === 404) ||
    (query.isSuccess && !query.data);
  return (
    <div className="text-caption">
      <p>{t(($) => $.duplicate_reference, { identifier })}</p>
      {deleted ? (
        <p className="text-muted-foreground">
          {t(($) => $.target_unavailable)}
        </p>
      ) : (
        issueId && (
          <AppLink className="underline" href={paths.issueDetail(issueId)}>
            {t(($) => $.open_task)}
          </AppLink>
        )
      )}
    </div>
  );
}
