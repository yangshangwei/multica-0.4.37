"use client";

import { useId } from "react";
import type { TriageHistoryEntry } from "@multica/core/triage";
import { Button } from "@multica/ui/components/ui/button";
import { cn } from "@multica/ui/lib/utils";
import {
  Check,
  CircleHelp,
  Clock3,
  Copy,
  FileSpreadsheet,
  Inbox,
  Play,
  RotateCcw,
  UserRoundCheck,
  X,
} from "lucide-react";
import { useT } from "../i18n";
import { TriageHistoryContent } from "./triage-history";
import { groupTriageHistoryEntries } from "./triage-history-dates";
import { TRIAGE_CONTROL, TRIAGE_HISTORY_ACTIONS } from "./triage-ui";

const ACTION_ICONS = {
  accept: Check,
  accept_and_execute: Play,
  reject: X,
  duplicate: Copy,
  snooze: Clock3,
  unsnooze: Inbox,
  reopen: RotateCcw,
  assign_reviewer: UserRoundCheck,
};

export function TriageHistoryTimeline({
  wsId,
  entries,
  onOpenIssue,
  onOpenImport,
}: {
  wsId: string;
  entries: readonly TriageHistoryEntry[];
  onOpenIssue: (id: string) => void;
  onOpenImport: (batchId: string) => void;
}) {
  const { t, i18n } = useT("triage");
  const id = useId();
  const groups = groupTriageHistoryEntries(entries, i18n.language);

  return (
    <div className="mx-auto max-w-4xl space-y-6 py-5">
      {groups.map((group) => {
        const headingId = `${id}-${group.key}`;
        return (
          <section key={group.key} aria-labelledby={headingId}>
            <h2 id={headingId} className="mb-3 text-label font-semibold">
              {group.dateLabel ?? t(($) => $.history_timeline.unknown_date)}
            </h2>
            <ol aria-labelledby={headingId}>
              {group.events.map(({ entry, time, fullTime }) => {
                const isImport = entry.kind === "import";
                const knownAction = TRIAGE_HISTORY_ACTIONS.find(
                  (name) => name === entry.action,
                );
                const Icon = isImport
                  ? FileSpreadsheet
                  : knownAction
                    ? ACTION_ICONS[knownAction]
                    : CircleHelp;
                const title = isImport
                  ? t(($) => $.import_history, {
                      filename: entry.filename || t(($) => $.unknown),
                    })
                  : `${entry.identifier ?? ""} ${entry.title}`.trim() ||
                    t(($) => $.unknown);
                const canOpen = (isImport && entry.batch_id) || entry.issue_id;
                return (
                  <li
                    key={entry.id}
                    className="group/event relative pb-5 pl-9 last:pb-0 sm:pl-32"
                  >
                    <div
                      aria-hidden="true"
                      className="absolute inset-y-0 left-0 w-6 sm:left-24"
                    >
                      <span className="absolute bottom-0 left-3 top-3 border-l border-surface-border group-last/event:hidden" />
                      <span className="relative flex size-6 items-center justify-center rounded-full bg-background text-muted-foreground">
                        <Icon className="size-4" />
                      </span>
                    </div>
                    <article className="min-w-0 space-y-1 text-body [overflow-wrap:anywhere]">
                      {time && fullTime ? (
                        <time
                          dateTime={entry.created_at}
                          title={fullTime}
                          className="block text-caption tabular-nums text-muted-foreground sm:absolute sm:left-0 sm:top-1 sm:w-20 sm:text-right"
                        >
                          <span aria-hidden="true">{time}</span>
                          <span className="sr-only">{fullTime}</span>
                        </time>
                      ) : (
                        <span className="block text-caption text-muted-foreground sm:absolute sm:left-0 sm:top-1 sm:w-20 sm:text-right">
                          {t(($) => $.history_timeline.unknown_time)}
                        </span>
                      )}
                      <div className="flex min-w-0 flex-wrap items-baseline gap-x-2">
                        {!isImport && (
                          <span className="font-medium">
                            {knownAction
                              ? t(($) => $.history_timeline.actions[knownAction])
                              : entry.action || t(($) => $.unknown)}
                          </span>
                        )}
                        {canOpen ? (
                          <Button
                            variant="link"
                            className={cn(
                              TRIAGE_CONTROL,
                              "h-auto min-w-0 max-w-full shrink justify-start whitespace-normal p-0 text-left [overflow-wrap:anywhere]",
                            )}
                            onClick={() => {
                              if (isImport && entry.batch_id)
                                onOpenImport(entry.batch_id);
                              else if (entry.issue_id)
                                onOpenIssue(entry.issue_id);
                            }}
                          >
                            <span className="min-w-0">{title}</span>
                          </Button>
                        ) : (
                          <span className="font-medium">{title}</span>
                        )}
                      </div>
                      {entry.counts && (
                        <p className="text-caption text-muted-foreground">
                          {t(($) => $.csv_results, { ...entry.counts })}
                        </p>
                      )}
                      <TriageHistoryContent wsId={wsId} entry={entry} compact />
                    </article>
                  </li>
                );
              })}
            </ol>
          </section>
        );
      })}
    </div>
  );
}
